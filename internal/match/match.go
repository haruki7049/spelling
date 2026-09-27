// Package match runs a match: two player machines and the shared world.
//
// Each tick, both machines get the same instruction budget and execute one
// instruction at a time, alternating. The player with more mana at the start
// of the tick goes first; a tie is decided by coin flip. After both budgets
// are used up (or both players wait with wfi), the world advances one
// physics step and mana regenerates.
//
// Mana: every executed instruction and every write to a body costs mana.
// A write the player cannot afford is ignored and costs nothing. A player
// who cannot pay for the next instruction is depleted: the CPU stops, the
// body's velocity becomes 0, and mana never regenerates again.
//
// Body regions (see issue #15). Values are 16.16 fixed-point unless noted.
// Writes cost mana; writing the opponent's body costs OpponentCostFactor
// times as much.
//
//	0x1000_2000  own body
//	0x1000_3000  opponent body
//	  +0x00  x           read-write (CostOwnPosition)
//	  +0x04  y           read-write (CostOwnPosition)
//	  +0x08  vx          read-write, per tick (squared change, see writeVelocity)
//	  +0x0c  vy          read-write, per tick (squared change, see writeVelocity)
//	  +0x10  facing      read-write, 1 = right, -1 = left (CostOwnFacing)
//	  +0x14  grounded    read-only, 0 or 1
//	  +0x18  HP          read-only (integer)
//	  +0x1c  max HP      read-only (integer)
//	  +0x20  mana        read-only (integer)
//	  +0x24  max mana    read-only (integer)
//	  +0x28  mana regen  read-only, per tick (integer)
package match

import (
	"math"

	"github.com/haruki7049/spelling/internal/machine"
	"github.com/haruki7049/spelling/internal/world"
)

// Tentative first-playable numbers (see issue #15).
const (
	TicksPerSecond      = 60
	InstructionsPerTick = 1000
	RAMSize             = 64 * 1024
)

// Tentative mana numbers (see issue #15). Tune them here.
const (
	MaxMana         = 600_000 // about 10 s of full-speed execution
	ManaRegen       = 500     // per tick, unless depleted
	InstructionCost = 1       // per executed instruction
	CostOwnFacing   = 1_000   // writing own facing
	// Velocity writes cost VelocityCostNum/VelocityCostDen mana per (speed
	// change)², where the change is accumulated per tick (see velocityCost).
	VelocityCostNum    = 25
	VelocityCostDen    = 16     // 25/16 per speed² makes a change of 64 cost 6,400
	CostOwnPosition    = 10_000 // writing own position (teleport)
	OpponentCostFactor = 10     // writing the opponent's body costs this many times more
	CostAssembler      = 2_000  // per built-in assembler window call
)

// Body region addresses and offsets.
const (
	OwnBodyBase      = 0x1000_2000
	OpponentBodyBase = 0x1000_3000
	bodySize         = 0x30

	BodyX         = 0x00
	BodyY         = 0x04
	BodyVX        = 0x08
	BodyVY        = 0x0c
	BodyFacing    = 0x10
	BodyGrounded  = 0x14
	BodyHP        = 0x18
	BodyMaxHP     = 0x1c
	BodyMana      = 0x20
	BodyMaxMana   = 0x24
	BodyManaRegen = 0x28
)

// idleProgram is loaded for a player without an ELF: "loop: wfi; j loop",
// so the CPU waits for typed lines while spending almost no mana.
var idleProgram = [...]uint32{0x10500073, 0xffdff06f}

// Match is a running match between players 0 and 1.
type Match struct {
	World    *world.World
	Machines [2]*machine.Machine
	Tick     uint32

	// CoinFlip decides who goes first on a tie; true means player 0.
	// It is the only randomness in the game.
	CoinFlip func() bool

	// InfiniteMana disables mana depletion and costs for both players,
	// intended for practice and spell testing. Expenditures are still
	// tracked so real costs remain visible.
	InfiniteMana bool

	result Result

	spentThisTick [2]int32
	spentHistory  [2][TicksPerSecond]int32
	spentSum      [2]int32
}

// New returns a match. elfs[i] is player i's program; nil means the idle
// program, which only runs typed lines.
func New(elfs [2][]byte, coinFlip func() bool) (*Match, error) {
	m := &Match{World: world.New(), CoinFlip: coinFlip}
	for i := range m.Machines {
		body := &m.World.Bodies[i]
		body.Mana = MaxMana
		mc := machine.New(RAMSize, 0)
		mc.Player = uint32(i)
		if elfs[i] != nil {
			if err := mc.LoadELF(elfs[i]); err != nil {
				return nil, err
			}
		} else {
			for j, w := range idleProgram {
				mc.RAM.Write(uint32(4*j), 4, w)
			}
		}
		mc.Map(OwnBodyBase, bodySize, &bodyDevice{match: m, body: body, payer: i, factor: 1, writer: i, target: i})
		mc.Map(OpponentBodyBase, bodySize, &bodyDevice{match: m, body: &m.World.Bodies[1-i], payer: i, factor: OpponentCostFactor, writer: i, target: 1 - i})
		mc.PayAssembler = func() bool { return m.pay(i, CostAssembler) }
		m.Machines[i] = mc
	}
	return m, nil
}

// Step runs one tick. It does nothing once the match is decided.
func (m *Match) Step() {
	if m.result != Ongoing {
		return
	}
	first := m.first()
	for _, mc := range m.Machines {
		mc.Tick = m.Tick
		mc.SetBudget(InstructionsPerTick)
	}
	for {
		ranA, ranB := m.stepPlayer(first), m.stepPlayer(1-first)
		if !ranA && !ranB {
			break
		}
	}
	m.World.Step()
	for i := range m.World.Bodies {
		if b := &m.World.Bodies[i]; !b.Depleted {
			b.Mana = min(b.Mana+ManaRegen, MaxMana)
		}
	}
	slot := m.Tick % TicksPerSecond
	for i := range m.spentSum {
		old := m.spentHistory[i][slot]
		new := m.spentThisTick[i]
		m.spentHistory[i][slot] = new
		m.spentSum[i] = m.spentSum[i] - old + new
		m.spentThisTick[i] = 0
	}
	m.Tick++
	m.result = m.judge()
}

// ManaSpentPerSecond returns the mana spent by player i (0 or 1) over the
// last second (TicksPerSecond ticks).
func (m *Match) ManaSpentPerSecond(player int) int32 {
	return m.spentSum[player]
}

// first returns the player who goes first this tick.
func (m *Match) first() int {
	a, b := m.World.Bodies[0].Mana, m.World.Bodies[1].Mana
	switch {
	case a > b:
		return 0
	case b > a:
		return 1
	case m.CoinFlip():
		return 0
	}
	return 1
}

// stepPlayer executes one instruction of player i if it has budget left and
// can pay for it, and reports whether an instruction ran.
func (m *Match) stepPlayer(i int) bool {
	mc, b := m.Machines[i], &m.World.Bodies[i]
	if b.Depleted || mc.Remaining() <= 0 {
		return false
	}
	if !m.pay(i, InstructionCost) {
		deplete(b, mc)
		return false
	}
	return mc.Step()
}

// pay spends cost from player's mana if it can afford it, and records the cost
// for that player's mana rate. With InfiniteMana, payments always succeed
// and mana is untouched, while expenditures are still recorded.
func (m *Match) pay(player int, cost int32) bool {
	if m.InfiniteMana {
		m.spentThisTick[player] += cost
		return true
	}
	b := &m.World.Bodies[player]
	if b.Depleted || b.Mana < cost {
		return false
	}
	b.Mana -= cost
	m.spentThisTick[player] += cost
	return true
}

// deplete makes all of a player's spell effects disappear for good.
func deplete(b *world.Body, mc *machine.Machine) {
	b.Mana = 0
	b.Depleted = true
	b.VX, b.VY = 0, 0
	mc.SetBudget(0)
}

// bodyDevice maps a body into a player's memory. Writes are paid by payer
// at factor times the own-body cost.
type bodyDevice struct {
	match  *Match
	body   *world.Body
	payer  int
	factor int32
	writer int
	target int

	// changed is the total |change| of vx and vy written through this
	// device during tick changedTick; velocity costs are charged on it.
	changed     [2]int64
	changedTick uint32
}

func (d *bodyDevice) notifyDefense(off, v uint32) bool {
	if d.writer == d.target {
		return true
	}
	targetAddr := OwnBodyBase + off
	watch := d.match.Machines[d.target].MatchingWatch(targetAddr, true)
	if watch == nil {
		return true
	}
	if !d.match.Machines[d.target].NotifyDefense(uint32(d.writer), targetAddr, v) && watch.Policy == machine.PolicyDeny {
		return false
	}
	return true
}

func (d *bodyDevice) ReadReg(off uint32) uint32 {
	b := d.body
	switch off {
	case BodyX:
		return uint32(b.X)
	case BodyY:
		return uint32(b.Y)
	case BodyVX:
		return uint32(b.VX)
	case BodyVY:
		return uint32(b.VY)
	case BodyFacing:
		return uint32(b.Facing)
	case BodyGrounded:
		if b.Grounded {
			return 1
		}
	case BodyHP:
		return uint32(b.HP)
	case BodyMaxHP:
		return world.MaxHP
	case BodyMana:
		if d.match.InfiniteMana {
			return MaxMana
		}
		return uint32(b.Mana)
	case BodyMaxMana:
		return MaxMana
	case BodyManaRegen:
		return ManaRegen
	}
	return 0
}

// writeCost returns the own-body cost of writing a register, or 0 if the
// register is read-only.
func writeCost(off uint32) int32 {
	switch off {
	case BodyX, BodyY:
		return CostOwnPosition
	case BodyFacing:
		return CostOwnFacing
	}
	return 0
}

func (d *bodyDevice) WriteReg(off, v uint32) {
	switch off {
	case BodyVX:
		d.writeVelocity(off, 0, &d.body.VX, v)
		return
	case BodyVY:
		d.writeVelocity(off, 1, &d.body.VY, v)
		return
	}
	cost := writeCost(off)
	if cost == 0 || !d.match.pay(d.payer, cost*d.factor) {
		return
	}
	if !d.notifyDefense(off, v) {
		return
	}
	b := d.body
	switch off {
	case BodyX:
		b.X = int32(v)
	case BodyY:
		b.Y = int32(v)
	case BodyFacing:
		if int32(v) < 0 {
			b.Facing = -1
		} else {
			b.Facing = 1
		}
		b.FacingHold = world.ManaHoldTicks
	}
}

// maxChangeForCost caps the accumulated change so squaring cannot overflow;
// a change this large is never affordable anyway.
const maxChangeForCost = 4096 * world.One

// velocityCost is the own-body cost of a total speed change of change
// (16.16) within one tick.
func velocityCost(change int64) int64 {
	change = min(change, maxChangeForCost)
	return change * change * VelocityCostNum / (VelocityCostDen * world.One * world.One)
}

// velocityWriteCost computes the incremental mana cost and new accumulated
// change for moving dst to target along axis.
func (d *bodyDevice) velocityWriteCost(axis int, current, target int32) (cost, total int64) {
	delta := int64(target) - int64(current)
	if delta < 0 {
		delta = -delta
	}
	total = d.changed[axis] + delta
	cost = (velocityCost(total) - velocityCost(d.changed[axis])) * int64(d.factor)
	return cost, total
}

// writeVelocity writes a velocity (clamped to the speed limit). The cost is
// the square of the total change written to this axis through this device
// in the current tick, minus what was already paid this tick, so splitting
// a change into several writes does not save mana.
func (d *bodyDevice) writeVelocity(off uint32, axis int, dst *int32, rawV uint32) {
	if d.changedTick != d.match.Tick {
		d.changed, d.changedTick = [2]int64{}, d.match.Tick
	}
	v := min(max(int32(rawV), -world.MaxSpeed), world.MaxSpeed)
	cost, total := d.velocityWriteCost(axis, *dst, v)
	if cost > int64(math.MaxInt32) || !d.match.pay(d.payer, int32(cost)) {
		return
	}
	d.changed[axis] = total
	if !d.notifyDefense(off, rawV) {
		return
	}
	*dst = v
}
