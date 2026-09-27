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

func TestDefenseInterruptDispatch(t *testing.T) {
	// Main program counts s0, handler reads defense event addr and pops it.
	m := newMachine(t, `
	main:
		addi s0, s0, 1
		j main
	handler:
		lui t0, 0x10031         # DefenseBase (0x1003_1000)
		lw a0, 8(t0)            # DefEventAddr
		sw a0, 0x600(zero)      # store event addr to RAM for verification
		sw zero, 16(t0)         # DefEventPop
		lui t0, 0x10000         # SystemBase
		sw zero, 0x1c(t0)       # SysReturn
	`)

	// Register watch entry 0 on range [0x1000_2000, 0x1000_2020)
	// 'main' has 2 instructions (8 bytes), so 'handler' starts at 0x08.
	handlerPC := uint32(8)
	base := WatchBase
	m.Write(base+WatchStart, 4, 0x1000_2000)
	m.Write(base+WatchEnd, 4, 0x1000_2020)
	m.Write(base+WatchFlags, 4, WatchWrite)
	m.Write(base+WatchHandler, 4, handlerPC)
	m.Write(base+WatchEnabled, 4, 1)

	// Run main program a few steps
	m.Run(10)
	s0Before := m.CPU.Regs[8]

	// Notify defense event
	m.NotifyDefense(1, 0x1000_2008, 0x80000)

	// Run next steps to trigger defense interrupt
	m.Run(10)

	// 1. RAM[0x600] has the event addr (0x1000_2008)
	if got := m.RAM.Read(0x600, 4); got != 0x1000_2008 {
		t.Errorf("handler stored event addr = %#x, want 0x1000_2008", got)
	}
	// 2. Event was popped, DefEventCount == 0
	if got := m.Read(DefenseBase+DefEventCount, 4); got != 0 {
		t.Errorf("DefEventCount = %d, want 0", got)
	}
	// 3. SysReturn restored s0 and returned to main
	if s0After := m.CPU.Regs[8]; s0After <= s0Before {
		t.Errorf("s0 after = %d, want > %d", s0After, s0Before)
	}
	// 4. a0 was restored to 0 (since main had a0 = 0)
	if m.CPU.Regs[10] != 0 {
		t.Errorf("a0 = %d, want 0 (restored)", m.CPU.Regs[10])
	}
	// 5. Interrupts re-enabled
	if !m.intEnable {
		t.Error("intEnable = false, want true after SysReturn")
	}
}

func TestDefenseInterruptNoHandler(t *testing.T) {
	m := newMachine(t, `
	main:
		addi s0, s0, 1
		j main
	`)
	// Watch entry has Handler = 0
	base := WatchBase
	m.Write(base+WatchStart, 4, 0x1000_2000)
	m.Write(base+WatchEnd, 4, 0x1000_2020)
	m.Write(base+WatchFlags, 4, WatchWrite)
	m.Write(base+WatchHandler, 4, 0)
	m.Write(base+WatchEnabled, 4, 1)

	m.Run(5)

	m.NotifyDefense(1, 0x1000_2008, 0x80000)
	m.Run(5)

	// Interrupt should not be taken; event remains in queue
	if got := m.Read(DefenseBase+DefEventCount, 4); got != 1 {
		t.Errorf("DefEventCount = %d, want 1", got)
	}
	if !m.intEnable {
		t.Error("intEnable should remain true")
	}
}
