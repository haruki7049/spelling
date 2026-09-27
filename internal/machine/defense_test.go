package machine

import (
	"testing"
)

func TestDefenseInfoDefaults(t *testing.T) {
	m := New(0x1000, 0)
	base := DefenseBase

	if got := m.Read(base+DefEventCount, 4); got != 0 {
		t.Errorf("count = %d, want 0", got)
	}
	if got := m.Read(base+DefEventWho, 4); got != 0 {
		t.Errorf("who = %d, want 0", got)
	}
	if got := m.Read(base+DefEventAddr, 4); got != 0 {
		t.Errorf("addr = %#x, want 0", got)
	}
	if got := m.Read(base+DefEventValue, 4); got != 0 {
		t.Errorf("value = %#x, want 0", got)
	}
	if got := m.Read(base+DefOverflow, 4); got != 0 {
		t.Errorf("overflow = %d, want 0", got)
	}
}

func TestDefenseInfoQueueAndPop(t *testing.T) {
	m := New(0x1000, 0)
	base := DefenseBase

	ok := m.NotifyDefense(1, 0x1000_2008, 0x80000)
	if !ok {
		t.Fatalf("NotifyDefense failed on empty queue")
	}

	if got := m.Read(base+DefEventCount, 4); got != 1 {
		t.Errorf("count = %d, want 1", got)
	}
	if got := m.Read(base+DefEventWho, 4); got != 1 {
		t.Errorf("who = %d, want 1", got)
	}
	if got := m.Read(base+DefEventAddr, 4); got != 0x1000_2008 {
		t.Errorf("addr = %#x, want 0x1000_2008", got)
	}
	if got := m.Read(base+DefEventValue, 4); got != 0x80000 {
		t.Errorf("value = %#x, want 0x80000", got)
	}

	// Pop the event
	m.Write(base+DefEventPop, 4, 1)

	if got := m.Read(base+DefEventCount, 4); got != 0 {
		t.Errorf("count after pop = %d, want 0", got)
	}
	if got := m.Read(base+DefEventWho, 4); got != 0 {
		t.Errorf("who after pop = %d, want 0", got)
	}
	if got := m.Read(base+DefEventAddr, 4); got != 0 {
		t.Errorf("addr after pop = %#x, want 0", got)
	}
	if got := m.Read(base+DefEventValue, 4); got != 0 {
		t.Errorf("value after pop = %#x, want 0", got)
	}
}

func TestDefenseInfoFIFOOrder(t *testing.T) {
	m := New(0x1000, 0)
	base := DefenseBase

	m.NotifyDefense(0, 0x1000_2000, 10)
	m.NotifyDefense(1, 0x1000_2004, 20)

	if got := m.Read(base+DefEventAddr, 4); got != 0x1000_2000 {
		t.Errorf("first event addr = %#x, want 0x1000_2000", got)
	}
	m.Write(base+DefEventPop, 4, 1)

	if got := m.Read(base+DefEventAddr, 4); got != 0x1000_2004 {
		t.Errorf("second event addr = %#x, want 0x1000_2004", got)
	}
	m.Write(base+DefEventPop, 4, 1)

	if got := m.Read(base+DefEventCount, 4); got != 0 {
		t.Errorf("count after all pops = %d, want 0", got)
	}
}

func TestDefenseInfoQueueCapacityAndOverflow(t *testing.T) {
	m := New(0x1000, 0)
	base := DefenseBase

	for i := range DefQueueCap {
		if !m.NotifyDefense(0, 0x1000_2000+uint32(i*4), uint32(i)) {
			t.Fatalf("event %d failed to queue", i)
		}
	}

	if got := m.Read(base+DefEventCount, 4); got != DefQueueCap {
		t.Errorf("count = %d, want %d", got, DefQueueCap)
	}
	if got := m.Read(base+DefOverflow, 4); got != 0 {
		t.Errorf("overflow = %d before drop, want 0", got)
	}

	// 17th event should be dropped and set overflow
	if m.NotifyDefense(1, 0x1000_2099, 99) {
		t.Fatalf("17th event succeeded, want dropped")
	}

	if got := m.Read(base+DefEventCount, 4); got != DefQueueCap {
		t.Errorf("count after drop = %d, want %d", got, DefQueueCap)
	}
	if got := m.Read(base+DefOverflow, 4); got != 1 {
		t.Errorf("overflow after drop = %d, want 1", got)
	}

	// Write 0 to clear overflow
	m.Write(base+DefOverflow, 4, 0)
	if got := m.Read(base+DefOverflow, 4); got != 0 {
		t.Errorf("overflow after clear = %d, want 0", got)
	}
}

func TestDefenseInfoPopEmpty(t *testing.T) {
	m := New(0x1000, 0)
	base := DefenseBase
	// Popping an empty queue should do nothing and not crash.
	m.Write(base+DefEventPop, 4, 1)
	if got := m.Read(base+DefEventCount, 4); got != 0 {
		t.Errorf("count = %d, want 0", got)
	}
}
