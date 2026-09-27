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
//	  +0x08  vx          read-write, per tick (CostOwnMotion)
//	  +0x0c  vy          read-write, per tick (CostOwnMotion)
//	  +0x10  facing      read-write, 1 = right, -1 = left (CostOwnMotion)
//	  +0x14  grounded    read-only, 0 or 1
//	  +0x18  HP          read-only (integer)
//	  +0x1c  max HP      read-only (integer)
//	  +0x20  mana        read-only (integer)
//	  +0x24  max mana    read-only (integer)
//	  +0x28  mana regen  read-only, per tick (integer)
package match

import (
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
	MaxMana            = 600_000 // about 10 s of full-speed execution
	ManaRegen          = 500     // per tick, unless depleted
	InstructionCost    = 1       // per executed instruction
	CostOwnMotion      = 1_000   // writing own velocity or facing
	CostOwnPosition    = 10_000  // writing own position (teleport)
	OpponentCostFactor = 10      // writing the opponent's body costs this many times more
	CostAssembler      = 2_000   // per built-in assembler window call
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
		mc.Map(OwnBodyBase, bodySize, &bodyDevice{body: body, payer: body, factor: 1})
		mc.Map(OpponentBodyBase, bodySize, &bodyDevice{body: &m.World.Bodies[1-i], payer: body, factor: OpponentCostFactor})
		mc.PayAssembler = func() bool { return pay(body, CostAssembler) }
		m.Machines[i] = mc
	}
	return m, nil
}

// Step runs one tick.
func (m *Match) Step() {
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
	m.Tick++
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
	if !pay(b, InstructionCost) {
		deplete(b, mc)
		return false
	}
	return mc.Step()
}

// pay spends cost from b's mana if it can afford it.
func pay(b *world.Body, cost int32) bool {
	if b.Depleted || b.Mana < cost {
		return false
	}
	b.Mana -= cost
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
	body   *world.Body
	payer  *world.Body
	factor int32
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
	case BodyVX, BodyVY, BodyFacing:
		return CostOwnMotion
	}
	return 0
}

func (d *bodyDevice) WriteReg(off, v uint32) {
	cost := writeCost(off)
	if cost == 0 || !pay(d.payer, cost*d.factor) {
		return
	}
	b := d.body
	switch off {
	case BodyX:
		b.X = int32(v)
	case BodyY:
		b.Y = int32(v)
	case BodyVX:
		b.VX = int32(v)
	case BodyVY:
		b.VY = int32(v)
	case BodyFacing:
		if int32(v) < 0 {
			b.Facing = -1
		} else {
			b.Facing = 1
		}
		b.FacingHold = world.ManaHoldTicks
	}
}
