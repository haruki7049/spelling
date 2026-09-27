package match

import (
	"testing"

	"github.com/haruki7049/spelling/internal/asm"
	"github.com/haruki7049/spelling/internal/machine"
	"github.com/haruki7049/spelling/internal/world"
)

func configureWatch(mc *machine.Machine, slot int, start, end, flags uint32, policy uint32) {
	base := machine.WatchBase + uint32(slot)*machine.WatchEntrySize
	mc.Write(base+machine.WatchStart, 4, start)
	mc.Write(base+machine.WatchEnd, 4, end)
	mc.Write(base+machine.WatchFlags, 4, flags)
	mc.Write(base+machine.WatchPolicy, 4, policy)
	mc.Write(base+machine.WatchEnabled, 4, 1)
}

func TestOpponentWriteQueuesDefenseEvent(t *testing.T) {
	m := newMatch(t)
	// Player 1 watches writes to their own body
	configureWatch(m.Machines[1], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyDeny)

	// Player 0 writes to OpponentBodyBase + BodyVX (8.0)
	vx := uint32(8 * world.One)
	m.Machines[0].Write(OpponentBodyBase+BodyVX, 4, vx)

	// Check Player 1's defense info
	defBase := machine.DefenseBase
	if got := m.Machines[1].Read(defBase+machine.DefEventCount, 4); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
	if got := m.Machines[1].Read(defBase+machine.DefEventWho, 4); got != 0 {
		t.Errorf("who = %d, want 0", got)
	}
	if got := m.Machines[1].Read(defBase+machine.DefEventAddr, 4); got != OwnBodyBase+BodyVX {
		t.Errorf("addr = %#x, want %#x", got, OwnBodyBase+BodyVX)
	}
	if got := m.Machines[1].Read(defBase+machine.DefEventValue, 4); got != vx {
		t.Errorf("value = %#x, want %#x", got, vx)
	}

	// Verify Player 1's VX was actually set
	if m.World.Bodies[1].VX != int32(vx) {
		t.Errorf("opponent VX = %d, want %d", m.World.Bodies[1].VX, vx)
	}
}

func TestSelfWriteDoesNotQueueDefenseEvent(t *testing.T) {
	m := newMatch(t)
	// Player 0 watches writes to their own body
	configureWatch(m.Machines[0], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyDeny)

	// Player 0 writes to OwnBodyBase + BodyVX
	m.Machines[0].Write(OwnBodyBase+BodyVX, 4, uint32(8*world.One))

	// Defense queue of Player 0 should remain empty
	if got := m.Machines[0].Read(machine.DefenseBase+machine.DefEventCount, 4); got != 0 {
		t.Errorf("count = %d, want 0", got)
	}
}

func TestOpponentWriteDenyPolicyOnOverflow(t *testing.T) {
	m := newMatch(t)
	// Player 1 watches writes to their own body with PolicyDeny
	configureWatch(m.Machines[1], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyDeny)

	// Fill Player 1's defense queue with 16 events
	for i := range machine.DefQueueCap {
		if !m.Machines[1].NotifyDefense(0, OwnBodyBase, uint32(i)) {
			t.Fatalf("fill event %d failed", i)
		}
	}

	// Player 0 attempts to write to opponent's VX
	m.Machines[0].Write(OpponentBodyBase+BodyVX, 4, uint32(10*world.One))

	// Write should be denied because queue is full and policy is deny
	if got := m.World.Bodies[1].VX; got != 0 {
		t.Errorf("opponent VX = %d, want 0 (denied)", got)
	}

	// Overflow flag should be set
	if got := m.Machines[1].Read(machine.DefenseBase+machine.DefOverflow, 4); got != 1 {
		t.Errorf("overflow = %d, want 1", got)
	}
}

func TestOpponentWriteAllowPolicyOnOverflow(t *testing.T) {
	m := newMatch(t)
	// Player 1 watches writes to their own body with PolicyAllow
	configureWatch(m.Machines[1], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyAllow)

	// Fill Player 1's defense queue with 16 events
	for i := range machine.DefQueueCap {
		if !m.Machines[1].NotifyDefense(0, OwnBodyBase, uint32(i)) {
			t.Fatalf("fill event %d failed", i)
		}
	}

	// Player 0 attempts to write to opponent's VX
	targetVX := int32(10 * world.One)
	m.Machines[0].Write(OpponentBodyBase+BodyVX, 4, uint32(targetVX))

	// Write should be allowed despite queue overflow because policy is allow
	if got := m.World.Bodies[1].VX; got != targetVX {
		t.Errorf("opponent VX = %d, want %d (allowed)", got, targetVX)
	}

	// Overflow flag should still be set
	if got := m.Machines[1].Read(machine.DefenseBase+machine.DefOverflow, 4); got != 1 {
		t.Errorf("overflow = %d, want 1", got)
	}
}

func TestOpponentWritePositionRegistersDefense(t *testing.T) {
	m := newMatch(t)
	// Player 1 watches writes to their own body
	configureWatch(m.Machines[1], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyDeny)

	newX := uint32(300 * world.One)
	m.Machines[0].Write(OpponentBodyBase+BodyX, 4, newX)

	defBase := machine.DefenseBase
	if got := m.Machines[1].Read(defBase+machine.DefEventCount, 4); got != 1 {
		t.Fatalf("count = %d, want 1", got)
	}
	if got := m.Machines[1].Read(defBase+machine.DefEventAddr, 4); got != OwnBodyBase+BodyX {
		t.Errorf("addr = %#x, want %#x", got, OwnBodyBase+BodyX)
	}
	if got := m.Machines[1].Read(defBase+machine.DefEventValue, 4); got != newX {
		t.Errorf("value = %#x, want %#x", got, newX)
	}
	if got := m.World.Bodies[1].X; got != int32(newX) {
		t.Errorf("body X = %d, want %d", got, newX)
	}
}

// TestManaChargedOnDeniedDefenseWrites records the current behavior where mana
// spent on a write is not refunded when the write is denied by defense
// (e.g. when the defense event queue overflows under PolicyDeny).
//
// NOTE: This records current behavior and is not a decided design choice.
func TestManaChargedOnDeniedDefenseWrites(t *testing.T) {
	t.Run("velocity", func(t *testing.T) {
		m := newMatch(t)
		configureWatch(m.Machines[1], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyDeny)

		for i := range machine.DefQueueCap {
			if !m.Machines[1].NotifyDefense(0, OwnBodyBase, uint32(i)) {
				t.Fatalf("fill event %d failed", i)
			}
		}

		initialMana := m.World.Bodies[0].Mana
		targetVX := uint32(10 * world.One)
		m.Machines[0].Write(OpponentBodyBase+BodyVX, 4, targetVX)

		if got := m.World.Bodies[1].VX; got != 0 {
			t.Errorf("opponent VX = %d, want 0 (denied)", got)
		}
		if m.World.Bodies[0].Mana >= initialMana {
			t.Errorf("mana = %d, want < %d (mana deducted despite write denied)", m.World.Bodies[0].Mana, initialMana)
		}
	})

	t.Run("position", func(t *testing.T) {
		m := newMatch(t)
		configureWatch(m.Machines[1], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyDeny)

		for i := range machine.DefQueueCap {
			if !m.Machines[1].NotifyDefense(0, OwnBodyBase, uint32(i)) {
				t.Fatalf("fill event %d failed", i)
			}
		}

		initialMana := m.World.Bodies[0].Mana
		initialX := m.World.Bodies[1].X
		m.Machines[0].Write(OpponentBodyBase+BodyX, 4, uint32(300*world.One))

		if got := m.World.Bodies[1].X; got != initialX {
			t.Errorf("opponent X = %d, want %d (denied)", got, initialX)
		}
		if m.World.Bodies[0].Mana >= initialMana {
			t.Errorf("mana = %d, want < %d (mana deducted despite write denied)", m.World.Bodies[0].Mana, initialMana)
		}
	})
}

// TestEndToEndDefenseHandler runs a full defense cycle in an active match:
// Player 1 registers a watch over their own body with a handler that reads
// the defense event, pops it, and counters by resetting their own velocity
// to 0. Player 0 casts a spell (a typed line) that sets Player 1's VX, and
// the match must run the whole cycle within one tick without corrupting
// state.
func TestEndToEndDefenseHandler(t *testing.T) {
	m := newMatch(t)

	// Assemble Player 1's defense handler at 0x100 (idle program occupies
	// only the first 2 words).
	handlerSrc := `
		lui t0, 0x10031         # DefenseBase (0x1003_1000)
		lw a0, 8(t0)            # DefEventAddr (unused, read for realism)
		sw zero, 16(t0)         # DefEventPop
		lui t0, 0x10002         # OwnBodyBase (0x1000_2000)
		sw zero, 8(t0)          # counter: reset own VX to 0
		lui t0, 0x10000         # SystemBase
		sw zero, 0x1c(t0)       # SysReturn
	`
	handlerCode, err := asm.Assemble(handlerSrc, 0x100)
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range handlerCode {
		m.Machines[1].RAM.Write(uint32(0x100+4*i), 4, w)
	}

	// Player 1 watches writes to their own body with PolicyDeny.
	configureWatch(m.Machines[1], 0, OwnBodyBase, OwnBodyBase+bodySize, machine.WatchWrite, machine.PolicyDeny)
	m.Machines[1].Write(machine.WatchBase+machine.WatchHandler, 4, 0x100)

	// Player 0 casts a spell setting Player 1's VX to 8.0 (0x80000 in
	// 16.16 fixed point).
	typeLine(m, 0, "lui t0, 0x10003; li t1, 0x80000; sw t1, 8(t0)")

	// Player 1's idle program (wfi; j loop) waits out the rest of the tick
	// once it finds no defense event pending, so the interrupt is only
	// taken on the following tick once Player 0's write has landed.
	m.Step()
	m.Step()

	// The defense event must have been consumed.
	if got := m.Machines[1].Read(machine.DefenseBase+machine.DefEventCount, 4); got != 0 {
		t.Fatalf("DefEventCount = %d, want 0 (handler consumed it)", got)
	}

	// The handler's counter must have taken effect: VX back to 0 despite
	// the incoming write of targetVX.
	if got := m.World.Bodies[1].VX; got != 0 {
		t.Errorf("Player 1 VX = %d, want 0 (counter applied)", got)
	}

	// Player 1's interrupts must be re-enabled after SysReturn, so the
	// match keeps running normally.
	for range 60 {
		m.Step()
	}
	if m.result != Ongoing {
		t.Errorf("result = %v, want Ongoing (match continues smoothly)", m.result)
	}
}
