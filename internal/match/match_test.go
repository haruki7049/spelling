package match

import (
	"testing"

	"github.com/haruki7049/spelling/internal/machine"
	"github.com/haruki7049/spelling/internal/world"
)

func newMatch(t *testing.T) *Match {
	t.Helper()
	flip := false
	m, err := New([2][]byte{}, func() bool { flip = !flip; return flip })
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func typeLine(m *Match, player int, line string) {
	for i := range len(line) {
		m.Machines[player].Type(line[i])
	}
	m.Machines[player].Edit(machine.EditSubmit)
}

func TestTypedSpellMovesBody(t *testing.T) {
	m := newMatch(t)
	x0 := m.World.Bodies[0].X
	typeLine(m, 0, "lui t0, 0x10002; li t1, 0x80000; sw t1, 8(t0)")
	m.Step()
	if vx := m.World.Bodies[0].VX; vx <= 0 {
		t.Fatalf("vx = %d after the run spell, want positive", vx)
	}
	for range 60 {
		m.Step()
	}
	if b := m.World.Bodies[0]; b.X <= x0 || b.VX != 0 {
		t.Errorf("x %d (from %d) vx %d: want moved right and stopped", b.X, x0, b.VX)
	}
	if m.World.Bodies[1].X != world.Width*3/4-world.BodyWidth {
		t.Error("the opponent moved")
	}
}

func TestTypedJump(t *testing.T) {
	m := newMatch(t)
	typeLine(m, 0, "lui t0, 0x10002; li t1, 0xc0000; sw t1, 12(t0)")
	peak := int32(0)
	for range 60 {
		m.Step()
		peak = max(peak, m.World.Bodies[0].Y)
	}
	if peak < 100*world.One {
		t.Errorf("peak = %d, want a jump", peak/world.One)
	}
}

func TestBodyRegions(t *testing.T) {
	m := newMatch(t)
	// Player 1 reads both bodies, then tries to write the opponent's body.
	typeLine(m, 1, "lui t0, 0x10002; lw a0, 0(t0); lw a1, 0x18(t0); lui t1, 0x10003; lw a2, 0(t1); li a3, 0; sw a3, 0(t1); sw a0, 0x100(zero); sw a1, 0x104(zero); sw a2, 0x108(zero)")
	m.Step()
	mc := m.Machines[1]
	if got := int32(mc.RAM.Read(0x100, 4)); got != m.World.Bodies[1].X {
		t.Errorf("own x = %d, want %d", got, m.World.Bodies[1].X)
	}
	if got := mc.RAM.Read(0x104, 4); got != world.MaxHP {
		t.Errorf("own HP = %d, want %d", got, world.MaxHP)
	}
	if got := int32(mc.RAM.Read(0x108, 4)); got != m.World.Bodies[0].X {
		t.Errorf("opponent x = %d, want %d", got, m.World.Bodies[0].X)
	}
	if m.World.Bodies[0].X == 0 {
		t.Error("the opponent body was written")
	}
}

func TestTickRunsBothBudgets(t *testing.T) {
	m := newMatch(t)
	// Both players count instructions in s0; each gets the full budget.
	for i := range m.Machines {
		m.Machines[i].RAM.Write(0, 4, 0x00140413) // addi s0, s0, 1
		m.Machines[i].RAM.Write(4, 4, 0xffdff06f) // j 0
	}
	m.Step()
	for i, mc := range m.Machines {
		if s0 := mc.CPU.Regs[8]; s0 != InstructionsPerTick/2 {
			t.Errorf("player %d: s0 = %d, want %d", i, s0, InstructionsPerTick/2)
		}
	}
	if m.Tick != 1 || m.Machines[0].Tick != 0 {
		t.Errorf("tick = %d, machine tick = %d", m.Tick, m.Machines[0].Tick)
	}
}
