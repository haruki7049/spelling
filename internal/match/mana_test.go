package match

import (
	"math"
	"testing"

	"github.com/haruki7049/spelling/internal/asm"
	"github.com/haruki7049/spelling/internal/machine"
	"github.com/haruki7049/spelling/internal/world"
)

// load assembles src into player i's RAM at 0 and restarts the CPU there.
func load(t *testing.T, m *Match, i int, src string) {
	t.Helper()
	code, err := asm.Assemble(src, 0)
	if err != nil {
		t.Fatal(err)
	}
	mc := m.Machines[i]
	for j, w := range code {
		mc.RAM.Write(uint32(4*j), 4, w)
	}
	mc.CPU.Entry = 0
	mc.CPU.Restart()
}

const busyLoop = "loop: j loop"

func TestManaStartsFull(t *testing.T) {
	m := newMatch(t)
	for i, b := range m.World.Bodies {
		if b.Mana != MaxMana {
			t.Errorf("player %d mana = %d, want %d", i, b.Mana, MaxMana)
		}
	}
}

func TestInstructionsCostManaAndManaRegenerates(t *testing.T) {
	m := newMatch(t)
	load(t, m, 0, busyLoop)
	m.Step()
	want := int32(MaxMana - InstructionsPerTick*InstructionCost + ManaRegen)
	if got := m.World.Bodies[0].Mana; got != want {
		t.Errorf("mana after a busy tick = %d, want %d", got, want)
	}
}

func TestIdlePlayerKeepsMana(t *testing.T) {
	// The default idle program waits with wfi, so it barely spends mana and
	// regeneration keeps it full.
	m := newMatch(t)
	for range 10 {
		m.Step()
	}
	if got := m.World.Bodies[0].Mana; got != MaxMana {
		t.Errorf("idle mana = %d, want %d", got, MaxMana)
	}
}

func TestManaIsCappedAtMax(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[0].Mana = MaxMana - 1
	m.Step()
	if got := m.World.Bodies[0].Mana; got != MaxMana {
		t.Errorf("mana = %d, want capped at %d", got, MaxMana)
	}
}

func mustAssemble(t *testing.T, src string) []uint32 {
	t.Helper()
	code, err := asm.Assemble(src, 0)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestWriteCosts(t *testing.T) {
	// ManaRegen would hide small costs at full mana, so start lower.
	tests := []struct {
		name string
		src  string
		want int32
	}{
		{"own velocity 8.0", "lui t0, 0x10002; li t1, 0x80000; sw t1, 8(t0)", 100},
		{"own facing", "lui t0, 0x10002; li t1, -1; sw t1, 16(t0)", CostOwnFacing},
		{"own position", "lui t0, 0x10002; li t1, 0x10000; sw t1, 0(t0)", CostOwnPosition},
		{"opponent velocity 8.0", "lui t0, 0x10003; li t1, 0x80000; sw t1, 8(t0)", 100 * OpponentCostFactor},
		{"opponent position", "lui t0, 0x10003; li t1, 0x10000; sw t1, 0(t0)", CostOwnPosition * OpponentCostFactor},
		{"read-only register", "lui t0, 0x10002; li t1, 5; sw t1, 0x18(t0)", 0},
		{"reading is free", "lui t0, 0x10003; lw t1, 0(t0)", 0},
	}
	for _, tt := range tests {
		got, _ := spentFrom(t, tt.src, MaxMana/2)
		if got != tt.want {
			t.Errorf("%s: spent %d on writes, want %d", tt.name, got, tt.want)
		}
	}
}

// spentFrom runs one tick of src for player 0 starting from the given mana
// and returns the mana spent on writes (total spent minus instructions,
// before regeneration).
func spentFrom(t *testing.T, src string, start int32) (int32, *Match) {
	t.Helper()
	m := newMatch(t)
	m.World.Bodies[0].Mana = start
	load(t, m, 0, src+"; stay: wfi; j stay")
	m.Step()
	instructions := int32(len(mustAssemble(t, src)) + 1)
	return start - m.World.Bodies[0].Mana + ManaRegen - instructions*InstructionCost, m
}

func TestOpponentBodyIsWritableAtCost(t *testing.T) {
	_, m := spentFrom(t, "lui t0, 0x10003; li t1, 0x50000; sw t1, 8(t0)", MaxMana/2)
	if vx := m.World.Bodies[1].VX; vx <= 0 {
		t.Errorf("opponent vx = %d, want pushed right", vx)
	}
}

func TestUnaffordableWriteIsIgnoredAndFree(t *testing.T) {
	// Enough mana for the instructions, not for the position write.
	got, m := spentFrom(t, "lui t0, 0x10002; li t1, 0x10000; sw t1, 0(t0)", CostOwnPosition-1)
	if got != 0 {
		t.Errorf("spent %d on an unaffordable write, want 0", got)
	}
	if x := m.World.Bodies[0].X; x == 0x10000 {
		t.Error("unaffordable write was applied")
	}
}

func TestDepletion(t *testing.T) {
	m := newMatch(t)
	load(t, m, 0, "lui t0, 0x10002; li t1, 0x40000; sw t1, 8(t0); loop: addi s0, s0, 1; j loop")
	m.World.Bodies[0].Mana = 200 // the vx 4.0 write costs 25; runs out during the first tick
	m.Step()

	b := m.World.Bodies[0]
	if b.Mana != 0 || !b.Depleted {
		t.Fatalf("mana %d depleted %v, want depleted", b.Mana, b.Depleted)
	}
	if b.VX != 0 || b.VY != 0 {
		t.Errorf("velocity (%d, %d), want movement effects gone", b.VX, b.VY)
	}
	s0 := m.Machines[0].CPU.Regs[8]
	for range 5 {
		m.Step()
	}
	if m.Machines[0].CPU.Regs[8] != s0 {
		t.Error("the CPU kept running after depletion")
	}
	if m.World.Bodies[0].Mana != 0 {
		t.Error("mana regenerated after depletion")
	}
}

func TestMoreManaGoesFirst(t *testing.T) {
	flips := 0
	m, err := New([2][]byte{}, func() bool { flips++; return true })
	if err != nil {
		t.Fatal(err)
	}
	m.Step() // tie: coin flip
	if flips != 1 {
		t.Fatalf("flips = %d after a tie, want 1", flips)
	}

	// Both players write the same register of player 0's body on their
	// first instruction; the one who goes first is overwritten by the other.
	for i, v := range []string{"1", "-1"} {
		base := "0x10002"
		if i == 1 {
			base = "0x10003"
		}
		load(t, m, i, "lui t0, "+base+"; li t1, "+v+"; sw t1, 16(t0); stay: wfi; j stay")
	}
	m.World.Bodies[0].Mana = MaxMana / 2 // player 1 has more mana
	m.Step()
	if flips != 1 {
		t.Errorf("coin flipped without a tie")
	}
	// Player 1 goes first, so player 0's own write lands last.
	if f := m.World.Bodies[0].Facing; f != 1 {
		t.Errorf("facing = %d, want player 0's write (1) to land last", f)
	}
}

func TestBodyRegionShowsMana(t *testing.T) {
	m := newMatch(t)
	load(t, m, 0, "lui t0, 0x10002; lw a0, 0x20(t0); lw a1, 0x24(t0); lw a2, 0x28(t0); stay: wfi; j stay")
	m.Step()
	r := m.Machines[0].CPU.Regs
	if r[11] != MaxMana || r[12] != ManaRegen || r[10] == 0 || r[10] > MaxMana {
		t.Errorf("mana %d max %d regen %d", r[10], r[11], r[12])
	}
}

func TestAssemblerWindowCostsMana(t *testing.T) {
	m := newMatch(t)
	mc := m.Machines[0]
	copy(mc.RAM[0x800:], "nop")
	m.World.Bodies[0].Mana = MaxMana / 2
	before := m.World.Bodies[0].Mana
	mc.Write(machine.AssemblerBase+machine.AsmSource, 4, 0x800)
	mc.Write(machine.AssemblerBase+machine.AsmSourceLen, 4, 3)
	mc.Write(machine.AssemblerBase+machine.AsmOutput, 4, 0x900)
	mc.Write(machine.AssemblerBase+machine.AsmOutputCap, 4, 4)
	mc.Write(machine.AssemblerBase+machine.AsmCommand, 4, 1)
	if got := before - m.World.Bodies[0].Mana; got != CostAssembler {
		t.Errorf("assembler call cost %d, want %d", got, CostAssembler)
	}
}

func TestFacingWriteHolds(t *testing.T) {
	m := newMatch(t)
	// Run right, then face left while still moving.
	load(t, m, 0, "lui t0, 0x10002; li t1, 0x100000; sw t1, 8(t0); li t1, -1; sw t1, 16(t0); stay: wfi; j stay")
	for i := range world.ManaHoldTicks {
		m.Step()
		if f := m.World.Bodies[0].Facing; f != -1 {
			t.Fatalf("tick %d: facing = %d, want the paid write (-1) held", i+1, f)
		}
	}
	m.Step()
	if f := m.World.Bodies[0].Facing; f != 1 {
		t.Errorf("facing = %d after the hold, want physics (1) again", f)
	}
}

func TestFacingRewriteRestartsHold(t *testing.T) {
	m := newMatch(t)
	b := &m.World.Bodies[0]
	b.VX = 0x100000
	face := func() {
		typeLine(m, 0, "lui t0, 0x10002; li t1, -1; sw t1, 16(t0)")
	}
	face()
	for range world.ManaHoldTicks / 2 {
		m.Step()
	}
	b.VX = 0x100000
	face() // restart the hold halfway
	for i := range world.ManaHoldTicks {
		b.VX = 0x100000
		m.Step()
		if b.Facing != -1 {
			t.Fatalf("tick %d after the rewrite: facing = %d, want held", i+1, b.Facing)
		}
	}
}

func TestManaSpentPerSecondTracksRate(t *testing.T) {
	m := newMatch(t)
	if got := m.ManaSpentPerSecond(0); got != 0 {
		t.Fatalf("initial mana spent per second = %d, want 0", got)
	}

	// Player 0 runs a busy loop for 1 tick: 1000 instructions = 1000 mana spent.
	load(t, m, 0, busyLoop)
	m.Step()
	if got := m.ManaSpentPerSecond(0); got != InstructionsPerTick*InstructionCost {
		t.Fatalf("mana spent per second after 1 tick = %d, want %d", got, InstructionsPerTick*InstructionCost)
	}
	// Player 1 is idle, so spends 1 instruction before wfi.
	if got := m.ManaSpentPerSecond(1); got != 1 {
		t.Fatalf("idle player mana spent per second = %d, want 1", got)
	}

	// Now replace player 0's program with idleProgram so it only spends 1 mana per tick.
	for j, w := range idleProgram {
		m.Machines[0].RAM.Write(uint32(4*j), 4, w)
	}
	m.Machines[0].CPU.Entry = 0
	m.Machines[0].CPU.Restart()

	// Advance 58 more ticks (total 59 ticks completed: tick 0 + 58 ticks).
	for range 58 {
		m.Step()
	}
	// The window of 59 ticks has: 1 tick of 1000, 1 tick of 1 (wfi only),
	// and 57 ticks of 2 (j and wfi) = 1115.
	if got := m.ManaSpentPerSecond(0); got != 1115 {
		t.Fatalf("mana spent after 59 ticks = %d, want 1115", got)
	}

	// Tick 59 (the 60th tick): all 60 slots (0..59) are now filled.
	m.Step()
	if got := m.ManaSpentPerSecond(0); got != 1117 {
		t.Fatalf("mana spent after 60 ticks = %d, want 1117", got)
	}

	// Tick 60 (the 61st tick): the 1000-mana tick rolls out of slot 0 and
	// is replaced by another 2-mana tick.
	m.Step()
	// Now 1 tick of 1 + 59 ticks of 2 = 119.
	if got := m.ManaSpentPerSecond(0); got != 119 {
		t.Fatalf("mana spent after tick 0 rolled out = %d, want 119", got)
	}
}

func TestManaSpentPerSecondTracksWrites(t *testing.T) {
	m := newMatch(t)
	// Player 0 writes own velocity 8.0: costs 100.
	// Plus 1 instruction for idle tick.
	velocityWrite(t, m, OwnBodyBase, 8<<16)
	m.Step()
	// 100 from velocity write + 1 from idle instruction = 101.
	if got := m.ManaSpentPerSecond(0); got != 101 {
		t.Fatalf("mana spent with velocity write = %d, want 101", got)
	}
	// Player 1 spent only 1 (idle).
	if got := m.ManaSpentPerSecond(1); got != 1 {
		t.Fatalf("player 1 mana spent = %d, want 1", got)
	}
}

func TestManaSpentPerSecondTracksOpponentWrites(t *testing.T) {
	m := newMatch(t)
	// Player 0 writes opponent's velocity 8.0: costs 100 * 10 = 1000.
	velocityWrite(t, m, OpponentBodyBase, 8<<16)
	m.Step()
	// Player 0 pays for writing the opponent, not player 1!
	if got := m.ManaSpentPerSecond(0); got != 1001 {
		t.Fatalf("player 0 mana spent writing opponent = %d, want 1001", got)
	}
	if got := m.ManaSpentPerSecond(1); got != 1 {
		t.Fatalf("player 1 mana spent = %d, want 1", got)
	}
}

func TestManaSpentPerSecondIgnoresUnaffordableWrites(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[0].Mana = 100
	// Write MaxInt32 velocity (would cost 6400, unaffordable): ignored.
	velocityWrite(t, m, OwnBodyBase, math.MaxInt32)
	m.Step()
	// Only the 1 idle instruction was paid.
	if got := m.ManaSpentPerSecond(0); got != 1 {
		t.Fatalf("mana spent after unaffordable write = %d, want 1", got)
	}
}

func TestInfiniteManaNeverDepletesAndAllowsExpensiveWrites(t *testing.T) {
	m := newMatch(t)
	m.InfiniteMana = true
	m.World.Bodies[0].Mana = 10 // Not enough for busy loop or large writes

	load(t, m, 0, busyLoop)
	m.Step()
	b := &m.World.Bodies[0]
	if b.Depleted {
		t.Fatalf("depleted with infinite mana, want not depleted")
	}
	if b.Mana != 10+ManaRegen {
		t.Fatalf("mana = %d, want preserved at %d", b.Mana, 10+ManaRegen)
	}

	// Writing velocity MaxSpeed costs 6400 (way above 10 mana).
	spent := velocityWrite(t, m, OwnBodyBase, 64<<16)
	if spent != 0 {
		t.Fatalf("spent from b.Mana = %d, want 0 (infinite mana doesn't deduct)", spent)
	}
	if b.VX != 64<<16 {
		t.Fatalf("vx = %d, want 64.0 (write succeeded)", b.VX)
	}
}

func TestInfiniteManaStillTracksManaSpentPerSecond(t *testing.T) {
	m := newMatch(t)
	m.InfiniteMana = true
	// Write velocity 8.0: costs 100 mana.
	velocityWrite(t, m, OwnBodyBase, 8<<16)
	m.Step()
	// 100 from velocity write + 1 from idle instruction = 101.
	if got := m.ManaSpentPerSecond(0); got != 101 {
		t.Fatalf("mana spent with infinite mana = %d, want 101", got)
	}
}
