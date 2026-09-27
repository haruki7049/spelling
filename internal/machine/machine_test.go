package machine

import (
	"testing"

	"github.com/haruki7049/spelling/internal/asm"
)

// newMachine returns a machine with 64 KiB of RAM running src from 0.
func newMachine(t *testing.T, src string) *Machine {
	t.Helper()
	code, err := asm.Assemble(src, 0)
	if err != nil {
		t.Fatal(err)
	}
	m := New(0x10000, 0)
	for i, w := range code {
		m.RAM.Write(uint32(4*i), 4, w)
	}
	return m
}

func typeString(m *Machine, s string) {
	for i := range len(s) {
		m.Type(s[i])
	}
}

// counter is a main program that keeps incrementing s0 and stores it at
// 0x700, so tests can check that it keeps running with its registers intact.
const counter = "loop: addi s0, s0, 1; sw s0, 0x700(zero); j loop"

func TestTypedLineRunsAsInterrupt(t *testing.T) {
	m := newMachine(t, counter)
	m.Run(30)
	before := m.CPU.Regs[8]

	typeString(m, "li s0, 1000; li a0, 5; sw a0, 0x600(zero)\n")
	m.Run(30)

	if got := m.RAM.Read(0x600, 4); got != 5 {
		t.Errorf("typed store = %d, want 5", got)
	}
	// The typed line clobbered s0 and a0, but the return restored them,
	// so the counter continued from where it was interrupted.
	if s0 := m.CPU.Regs[8]; s0 < before || s0 > before+30 {
		t.Errorf("s0 = %d after interrupt, want between %d and %d", s0, before, before+30)
	}
	if m.CPU.Regs[10] != 0 {
		t.Errorf("a0 = %d, want 0 (restored)", m.CPU.Regs[10])
	}
	if !m.intEnable {
		t.Error("interrupts not re-enabled after return")
	}
}

func TestLineEditor(t *testing.T) {
	m := newMachine(t, counter)
	typeString(m, "li a0, 9X\b; sw a0, 0x600(zero)\r")
	m.Run(30)
	if got := m.RAM.Read(0x600, 4); got != 9 {
		t.Errorf("stored %d, want 9 (backspace removed X)", got)
	}

	// A line that fails to assemble does nothing.
	typeString(m, "fly a0\n")
	if m.pendingLine != nil {
		t.Error("invalid line is pending")
	}
	// An empty line does nothing.
	typeString(m, "\n")
	if m.pendingLine != nil {
		t.Error("empty line is pending")
	}
}

func TestTypedLinesWhileInterrupted(t *testing.T) {
	m := newMachine(t, counter)
	// The first line never returns, so interrupts stay disabled.
	typeString(m, "stay: j stay\n")
	m.Run(5)
	if m.intEnable {
		t.Fatal("interrupts enabled inside the typed line")
	}

	typeString(m, "li a0, 1\n")
	if m.pendingLine == nil || m.keyOverflow {
		t.Fatal("second line should wait without overflow")
	}
	typeString(m, "li a0, 2\n")
	if !m.keyOverflow {
		t.Error("third line should set the overflow flag")
	}
}

func TestRestartLeavesHandler(t *testing.T) {
	m := newMachine(t, counter)
	typeString(m, "j 0x10\n") // jumps into zeroed immediate code: illegal
	m.Run(10)
	if !m.intEnable {
		t.Error("interrupts not re-enabled after an illegal-instruction restart")
	}
	if m.CPU.PC >= ImmediateBase {
		t.Errorf("PC = %#x, want back in the main program", m.CPU.PC)
	}
}

func TestKeyboardHandler(t *testing.T) {
	// The main program registers a handler, then counts. The handler copies
	// every buffered character to 0x600.. and returns.
	m := newMachine(t, `
		lui t0, 0x10000
		la t1, handler
		sw t1, 0x18(t0)
	main:
		addi s0, s0, 1
		j main
	handler:
		lui t0, 0x10001
		lw t1, 0x7fc(zero)      # write position, kept in RAM
		bnez t1, next
		li t1, 0x600
	next:
		lw t2, 0(t0)            # count
		beqz t2, done
		lbu t3, 4(t0)           # pop
		sb t3, 0(t1)
		addi t1, t1, 1
		j next
	done:
		sw t1, 0x7fc(zero)
		lui t0, 0x10000
		sw zero, 0x1c(t0)       # return
	`)
	m.Run(10)
	typeString(m, "fire\n")
	m.Run(200)
	typeString(m, "!")
	m.Run(200)

	if got := string(m.RAM[0x600:0x606]); got != "fire\n!" {
		t.Errorf("handler received %q, want %q", got, "fire\n!")
	}
	if m.CPU.Regs[8] < 10 {
		t.Errorf("main program did not keep running: s0 = %d", m.CPU.Regs[8])
	}
	if len(m.keys) != 0 {
		t.Errorf("%d characters left in the buffer", len(m.keys))
	}
}

func TestKeyboardBufferOverflow(t *testing.T) {
	m := New(0x1000, 0)
	m.keyHandler = 0x100
	m.intEnable = false
	for range KeyBufferSize + 1 {
		m.Type('a')
	}
	if len(m.keys) != KeyBufferSize || !m.keyOverflow {
		t.Errorf("len = %d, overflow = %v", len(m.keys), m.keyOverflow)
	}
	// Reading pops, and the overflow flag clears with a write of 0.
	if m.Read(KeyboardBase+KeyNext, 4) != 'a' || len(m.keys) != KeyBufferSize-1 {
		t.Error("reading next did not pop")
	}
	m.Write(KeyboardBase+KeyOverflow, 4, 0)
	if m.keyOverflow {
		t.Error("overflow not cleared")
	}
}

func TestSavedRegistersAreAccessible(t *testing.T) {
	// The typed line changes the saved a0 of the interrupted program, so the
	// change survives the return (a scheduler could switch tasks this way).
	m := newMachine(t, counter)
	typeString(m, "lui t0, 0x10000; li t1, 42; sw t1, 0xa8(t0)\n") // 0x80 + 4*10
	m.Run(20)
	if m.CPU.Regs[10] != 42 {
		t.Errorf("a0 = %d, want 42", m.CPU.Regs[10])
	}
}

func TestSystemRegion(t *testing.T) {
	m := newMachine(t, "lui t0, 0x10000; lw a0, 0(t0); lw a1, 4(t0); lw a2, 8(t0); stay: j stay")
	m.Tick, m.Player = 7, 1
	m.Run(10)
	if r := m.CPU.Regs; r[10] != 7 || r[11] != 7 || r[12] != 1 {
		t.Errorf("tick, remaining, player = %d, %d, %d; want 7, 7, 1", r[10], r[11], r[12])
	}
	// Read-only registers ignore writes.
	m.Write(SystemBase+SysTick, 4, 99)
	if m.Tick != 7 {
		t.Error("tick was written")
	}
	// Byte access to a register.
	m.Write(SystemBase+SysKeyHandler+1, 1, 0x12)
	if m.keyHandler != 0x1200 || m.Read(SystemBase+SysKeyHandler+1, 1) != 0x12 {
		t.Errorf("key handler = %#x after byte write", m.keyHandler)
	}
}

func TestAssemblerWindow(t *testing.T) {
	m := New(0x1000, 0)
	call := func(src string, out, capacity uint32) (status, n, line uint32) {
		copy(m.RAM[0x100:], src)
		m.Write(AssemblerBase+AsmSource, 4, 0x100)
		m.Write(AssemblerBase+AsmSourceLen, 4, uint32(len(src)))
		m.Write(AssemblerBase+AsmOutput, 4, out)
		m.Write(AssemblerBase+AsmOutputCap, 4, capacity)
		m.Write(AssemblerBase+AsmCommand, 4, 1)
		return m.Read(AssemblerBase+AsmStatus, 4), m.Read(AssemblerBase+AsmOutputLen, 4), m.Read(AssemblerBase+AsmErrorLine, 4)
	}

	status, n, _ := call("x: addi a0, zero, 1; j x", 0x400, 16)
	if status != AssemblerOK || n != 8 {
		t.Fatalf("status %d, length %d", status, n)
	}
	if m.RAM.Read(0x400, 4) != 0x00100513 || m.RAM.Read(0x404, 4) != 0xffdff06f {
		t.Errorf("output %#08x %#08x", m.RAM.Read(0x400, 4), m.RAM.Read(0x404, 4))
	}

	if status, _, line := call("nop\nfly", 0x400, 16); status != AssemblerSyntax || line != 2 {
		t.Errorf("syntax error: status %d, line %d", status, line)
	}
	if status, _, _ := call("nop; nop", 0x400, 4); status != AssemblerRange {
		t.Errorf("small output: status %d", status)
	}
	if status, _, _ := call("nop", 0xffe, 4); status != AssemblerRange {
		t.Errorf("output past RAM: status %d", status)
	}
	m.Write(AssemblerBase+AsmSourceLen, 4, 0xffff_ffff)
	m.Write(AssemblerBase+AsmCommand, 4, 1)
	if m.Read(AssemblerBase+AsmStatus, 4) != AssemblerRange {
		t.Error("huge source length accepted")
	}

	paid := 0
	m.PayAssembler = func() bool { paid++; return paid < 2 }
	if status, _, _ := call("nop", 0x400, 4); status != AssemblerOK {
		t.Errorf("first paid call: status %d", status)
	}
	if status, _, _ := call("nop", 0x400, 4); status != AssemblerMana {
		t.Errorf("unpaid call: status %d", status)
	}
}

func TestAssemblerWindowFromProgram(t *testing.T) {
	// A language implementation emits assembly text into RAM, assembles it
	// through the window, and calls the result.
	const text = "li a0, 77; ret"
	m := newMachine(t, `
		lui t0, 0x10050
		li t1, 0x200
		sw t1, 0(t0)       # source
		li t1, 14
		sw t1, 4(t0)       # source length
		li t1, 0x400
		sw t1, 8(t0)       # output
		li t1, 64
		sw t1, 12(t0)      # capacity
		sw zero, 16(t0)    # assemble
		li t2, 0x400
		jalr ra, 0(t2)
	stay: j stay
	`)
	copy(m.RAM[0x200:], text)
	m.Run(30)
	if m.CPU.Regs[10] != 77 {
		t.Errorf("a0 = %d, want 77", m.CPU.Regs[10])
	}
}

// FuzzMachine runs arbitrary RAM contents and typed input, which the
// opponent can influence, and checks that nothing panics.
func FuzzMachine(f *testing.F) {
	f.Add([]byte{0x13, 0x05, 0x10, 0x00}, "li a0, 1\n")
	f.Add([]byte{0x37, 0x05, 0x05, 0x10, 0x23, 0x28, 0x05, 0x00}, "x")
	f.Fuzz(func(t *testing.T, ram []byte, input string) {
		m := New(0x1000, 0)
		copy(m.RAM, ram)
		for i := range len(input) {
			m.Type(input[i])
			m.Run(8)
		}
		m.Run(64)
	})
}
