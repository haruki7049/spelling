package machine

import (
	"testing"

	"github.com/haruki7049/spelling/internal/asm"
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

func TestDefenseInterruptPriorityOverKeyHandler(t *testing.T) {
	m := newMachine(t, `
	main:
		addi s0, s0, 1
		j main
	`)

	// Assemble defHandler at 0x100
	defCode, err := asm.Assemble(`
		lui t0, 0x10031         # DefenseBase
		sw zero, 16(t0)         # DefEventPop
		li a0, 0xdef
		sw a0, 0x600(zero)      # mark defense handler ran
		lui t0, 0x10000
		sw zero, 0x1c(t0)       # SysReturn
	`, 0x100)
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range defCode {
		m.RAM.Write(uint32(0x100+4*i), 4, w)
	}

	// Assemble keyHandler at 0x200
	keyCode, err := asm.Assemble(`
		lui t0, 0x10001         # KeyboardBase
		lw a1, 4(t0)            # KeyNext (pop char)
		li a0, 0xcafe
		sw a0, 0x604(zero)      # mark key handler ran
		lui t0, 0x10000
		sw zero, 0x1c(t0)       # SysReturn
	`, 0x200)
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range keyCode {
		m.RAM.Write(uint32(0x200+4*i), 4, w)
	}

	// Configure keyHandler
	m.Write(SystemBase+SysKeyHandler, 4, 0x200)

	// Configure watch entry 0 pointing to defHandler at 0x100
	base := WatchBase
	m.Write(base+WatchStart, 4, 0x1000_2000)
	m.Write(base+WatchEnd, 4, 0x1000_2020)
	m.Write(base+WatchFlags, 4, WatchWrite)
	m.Write(base+WatchHandler, 4, 0x100)
	m.Write(base+WatchEnabled, 4, 1)

	// Run main program a few steps
	m.Run(10)

	// Simultaneously enqueue keyboard character and defense event
	m.Type('A')
	m.NotifyDefense(1, 0x1000_2008, 0x80000)

	// Step once with fresh budget: defense interrupt must be taken first
	m.SetBudget(1)
	if !m.Step() {
		t.Fatal("Step failed")
	}
	if m.CPU.PC != 0x104 { // stepped first instruction at 0x100
		t.Fatalf("first interrupt PC = %#x, want 0x104 (defHandler)", m.CPU.PC)
	}

	// Run until defense handler finishes
	m.Run(10)
	if got := m.RAM.Read(0x600, 4); got != 0xdef {
		t.Errorf("RAM[0x600] = %#x, want 0xdef", got)
	}
	if got := m.Read(DefenseBase+DefEventCount, 4); got != 0 {
		t.Errorf("DefEventCount = %d, want 0", got)
	}

	// Now keyboard interrupt runs on subsequent steps
	m.Run(10)
	if got := m.RAM.Read(0x604, 4); got != 0xcafe {
		t.Errorf("RAM[0x604] = %#x, want 0xcafe", got)
	}
	if got := m.Read(KeyboardBase+KeyCount, 4); got != 0 {
		t.Errorf("KeyCount = %d, want 0", got)
	}
}

func TestDefenseInterruptPriorityOverPendingLine(t *testing.T) {
	m := newMachine(t, `
	main:
		addi s0, s0, 1
		j main
	`)

	// Assemble defHandler at 0x100
	defCode, err := asm.Assemble(`
		lui t0, 0x10031         # DefenseBase
		sw zero, 16(t0)         # DefEventPop
		li a0, 0xdef
		sw a0, 0x600(zero)      # mark defense handler ran
		lui t0, 0x10000
		sw zero, 0x1c(t0)       # SysReturn
	`, 0x100)
	if err != nil {
		t.Fatal(err)
	}
	for i, w := range defCode {
		m.RAM.Write(uint32(0x100+4*i), 4, w)
	}

	base := WatchBase
	m.Write(base+WatchStart, 4, 0x1000_2000)
	m.Write(base+WatchEnd, 4, 0x1000_2020)
	m.Write(base+WatchFlags, 4, WatchWrite)
	m.Write(base+WatchHandler, 4, 0x100)
	m.Write(base+WatchEnabled, 4, 1)

	m.Run(5)

	// Simultaneously submit a typed line and enqueue a defense event
	typeString(m, "li a0, 0x123; sw a0, 0x604(zero)\n")
	m.NotifyDefense(1, 0x1000_2008, 0x80000)

	// Step once with budget: defense interrupt must be taken first
	m.SetBudget(1)
	if !m.Step() {
		t.Fatal("Step failed")
	}
	if m.CPU.PC != 0x104 {
		t.Fatalf("first interrupt PC = %#x, want 0x104 (defHandler)", m.CPU.PC)
	}

	// Run defHandler to completion (remaining 5 instructions)
	m.Run(5)
	if got := m.RAM.Read(0x600, 4); got != 0xdef {
		t.Errorf("RAM[0x600] = %#x, want 0xdef", got)
	}
	if got := m.RAM.Read(0x604, 4); got != 0 {
		t.Errorf("RAM[0x604] = %#x, want 0 before line execution", got)
	}

	// Next, line editor interrupt runs
	m.Run(10)
	if got := m.RAM.Read(0x604, 4); got != 0x123 {
		t.Errorf("RAM[0x604] = %#x, want 0x123 after line execution", got)
	}
}

// TestStaleFrontDefenseEventBlocksQueue records the current behavior where
// defenseHandler only inspects the front event (m.defEvents[0]). If that front
// event's watch is disabled or reconfigured so it no longer matches, defenseHandler
// returns 0 and blocks subsequent events whose watches are still valid and enabled
// from dispatching their interrupts until the front event is popped.
//
// NOTE: This records current behavior and is not a decided design choice.
func TestStaleFrontDefenseEventBlocksQueue(t *testing.T) {
	m := New(0x1000, 0)
	m.intEnable = true

	// Slot 0: watch 0x1000_2000..0x1000_2010 with handler at 0x100
	base0 := WatchBase + 0*WatchEntrySize
	m.Write(base0+WatchStart, 4, 0x1000_2000)
	m.Write(base0+WatchEnd, 4, 0x1000_2010)
	m.Write(base0+WatchFlags, 4, WatchWrite)
	m.Write(base0+WatchHandler, 4, 0x100)
	m.Write(base0+WatchEnabled, 4, 1)

	// Slot 1: watch 0x1000_2020..0x1000_2030 with handler at 0x200
	base1 := WatchBase + 1*WatchEntrySize
	m.Write(base1+WatchStart, 4, 0x1000_2020)
	m.Write(base1+WatchEnd, 4, 0x1000_2030)
	m.Write(base1+WatchFlags, 4, WatchWrite)
	m.Write(base1+WatchHandler, 4, 0x200)
	m.Write(base1+WatchEnabled, 4, 1)

	// Step 1: Queue event for slot 0
	if !m.NotifyDefense(1, 0x1000_2004, 0x111) {
		t.Fatal("failed to notify first defense event")
	}
	if h := m.defenseHandler(); h != 0x100 {
		t.Fatalf("defenseHandler = %#x, want 0x100", h)
	}

	// Step 2: Disable slot 0 watch
	m.Write(base0+WatchEnabled, 4, 0)

	// Step 3: Queue event for slot 1
	if !m.NotifyDefense(1, 0x1000_2024, 0x222) {
		t.Fatal("failed to notify second defense event")
	}

	// Current behavior: defenseHandler returns 0 because defEvents[0] does not match
	// an enabled watch, starving the still-valid defEvents[1].
	if h := m.defenseHandler(); h != 0 {
		t.Errorf("defenseHandler = %#x, want 0 (stale front event blocks queue)", h)
	}

	// takeInterrupt does not fire
	m.takeInterrupt()
	if m.CPU.PC != 0 {
		t.Errorf("PC = %#x, want 0 (no interrupt taken)", m.CPU.PC)
	}

	// Popping the stale front event unblocks the queue
	m.Write(DefenseBase+DefEventPop, 4, 1)
	if h := m.defenseHandler(); h != 0x200 {
		t.Errorf("defenseHandler after pop = %#x, want 0x200", h)
	}

	// Now taking the interrupt dispatches to handler 0x200
	m.takeInterrupt()
	if m.CPU.PC != 0x200 {
		t.Errorf("PC after unblock = %#x, want 0x200", m.CPU.PC)
	}
}
