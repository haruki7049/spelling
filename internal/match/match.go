// Package match runs a match: two player machines and the shared world.
//
// Each tick, both machines get the same instruction budget and execute one
// instruction at a time, alternating. The first player of each tick is
// chosen by coin flip (mana does not exist yet, so every tick is a tie).
// After both budgets are used up, the world advances one physics step.
//
// Body regions (see issue #15). Values are 16.16 fixed-point unless noted:
//
//	0x1000_2000  own body       (read-write where noted)
//	0x1000_3000  opponent body  (read-only for now)
//	  +0x00  x          read-write
//	  +0x04  y          read-write
//	  +0x08  vx         read-write, per tick
//	  +0x0c  vy         read-write, per tick
//	  +0x10  facing     read-write, 1 = right, -1 = left (integer)
//	  +0x14  grounded   read-only, 0 or 1
//	  +0x18  HP         read-only (integer)
//	  +0x1c  max HP     read-only (integer)
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

// Body region addresses and offsets.
const (
	OwnBodyBase      = 0x1000_2000
	OpponentBodyBase = 0x1000_3000
	bodySize         = 0x20

	BodyX        = 0x00
	BodyY        = 0x04
	BodyVX       = 0x08
	BodyVY       = 0x0c
	BodyFacing   = 0x10
	BodyGrounded = 0x14
	BodyHP       = 0x18
	BodyMaxHP    = 0x1c
)

// idleProgram is loaded for a player without an ELF: "j 0", so the CPU
// waits for typed lines instead of restarting on zeroed memory.
const idleProgram = 0x0000006f

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
		mc := machine.New(RAMSize, 0)
		mc.Player = uint32(i)
		if elfs[i] != nil {
			if err := mc.LoadELF(elfs[i]); err != nil {
				return nil, err
			}
		} else {
			mc.RAM.Write(0, 4, idleProgram)
		}
		mc.Map(OwnBodyBase, bodySize, &bodyDevice{body: &m.World.Bodies[i], writable: true})
		mc.Map(OpponentBodyBase, bodySize, &bodyDevice{body: &m.World.Bodies[1-i]})
		m.Machines[i] = mc
	}
	return m, nil
}

// Step runs one tick.
func (m *Match) Step() {
	first := 0
	if !m.CoinFlip() {
		first = 1
	}
	a, b := m.Machines[first], m.Machines[1-first]
	for _, mc := range m.Machines {
		mc.Tick = m.Tick
		mc.SetBudget(InstructionsPerTick)
	}
	for {
		ranA, ranB := a.Step(), b.Step()
		if !ranA && !ranB {
			break
		}
	}
	m.World.Step()
	m.Tick++
}

type bodyDevice struct {
	body     *world.Body
	writable bool
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
	}
	return 0
}

func (d *bodyDevice) WriteReg(off, v uint32) {
	if !d.writable {
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
	}
}
