// Package machine implements one player's machine: the virtual CPU, its
// memory map, keyboard input, interrupts, and the built-in assembler.
//
// Memory map (see issue #15). All registers are 32-bit little-endian words.
//
//	0x0000_0000  RAM
//	0x1000_0000  System region
//	  +0x00  tick                    read-only
//	  +0x04  remaining instructions  read-only, in this tick after the current one
//	  +0x08  player number           read-only
//	  +0x10  interrupt enable        0 or 1
//	  +0x14  saved PC                PC of the interrupted program
//	  +0x18  keyboard handler        0 = use the built-in assembler
//	  +0x1c  return                  any write returns from the interrupt
//	  +0x80  saved registers         x0-x31 of the interrupted program
//	0x1000_1000  Keyboard region
//	  +0x00  count                   characters in the buffer, read-only
//	  +0x04  next                    reading pops a character (0 if empty)
//	  +0x08  overflow                1 after input was dropped; write 0 to clear
//	0x1005_0000  Built-in assembler window
//	  +0x00  source address          text in RAM
//	  +0x04  source length           bytes, at most MaxAssemblerSource
//	  +0x08  output address          machine code destination in RAM
//	  +0x0c  output capacity         bytes available at the output address
//	  +0x10  command                 any write assembles
//	  +0x14  status                  AssemblerOK, ... read-only
//	  +0x18  output length           bytes written, read-only
//	  +0x1c  error line              1-based line of a syntax error, read-only
//	0x1006_0000  Immediate-code region (ImmediateSize bytes)
//
// Interrupts: before each instruction, if interrupts are enabled and one is
// pending, the machine saves PC and all registers to the System region,
// clears the enable flag, and jumps to the handler. A write to the return
// register restores the saved registers and PC and sets the enable flag,
// all at once, so a handler never clobbers the interrupted program.
//
// Keyboard: with a handler registered, typed characters go to the keyboard
// buffer and an interrupt is pending while it is not empty. Without one,
// characters go to a line editor; Enter assembles the line with the
// built-in assembler and runs it as an interrupt from the immediate-code
// region, followed by a return.
package machine

import (
	"github.com/haruki7049/spelling/internal/asm"
	"github.com/haruki7049/spelling/internal/vm"
)

// Region base addresses and sizes.
const (
	SystemBase    = 0x1000_0000
	systemSize    = 0x100
	KeyboardBase  = 0x1000_1000
	keyboardSize  = 0x10
	AssemblerBase = 0x1005_0000
	assemblerSize = 0x20
	ImmediateBase = 0x1006_0000
	ImmediateSize = 0x1000
)

// System region offsets.
const (
	SysTick       = 0x00
	SysRemaining  = 0x04
	SysPlayer     = 0x08
	SysIntEnable  = 0x10
	SysSavedPC    = 0x14
	SysKeyHandler = 0x18
	SysReturn     = 0x1c
	SysSavedRegs  = 0x80
)

// Keyboard region offsets.
const (
	KeyCount    = 0x00
	KeyNext     = 0x04
	KeyOverflow = 0x08
)

// Assembler window offsets.
const (
	AsmSource    = 0x00
	AsmSourceLen = 0x04
	AsmOutput    = 0x08
	AsmOutputCap = 0x0c
	AsmCommand   = 0x10
	AsmStatus    = 0x14
	AsmOutputLen = 0x18
	AsmErrorLine = 0x1c
)

// Assembler window status values.
const (
	AssemblerOK     = 0 // assembled and written to the output
	AssemblerSyntax = 1 // syntax error; see the error line
	AssemblerRange  = 2 // source or output outside RAM, or output too small
	AssemblerMana   = 3 // the call could not be paid for
)

// Limits (tentative; see the numbers section of issue #15).
const (
	KeyBufferSize      = 256  // keyboard buffer and line editor, in characters
	MaxAssemblerSource = 4096 // bytes of source per assembler window call
)

// Characters with special meaning to the line editor.
const (
	Backspace = '\b'
	Delete    = 0x7f
	Enter     = '\n'
)

// returnTrailer is appended to every typed line to return from the
// interrupt after it runs.
const returnTrailer = "\nlui t0, 0x10000; sw zero, 0x1c(t0)"

// Machine is one player's machine. It is the Bus of its CPU.
type Machine struct {
	CPU       *vm.CPU
	RAM       vm.RAM
	Immediate vm.RAM

	Tick   uint32
	Player uint32

	// PayAssembler is called once per assembler window call and reports
	// whether the fixed cost was paid. Nil means the call is free.
	// Mana does not exist yet; this is where it will be charged.
	PayAssembler func() bool

	remaining int

	intEnable  bool
	savedPC    uint32
	savedRegs  [32]uint32
	keyHandler uint32
	returning  bool

	keys        []byte
	keyOverflow bool
	line        []byte
	pendingLine []uint32 // assembled typed line waiting for an interrupt

	asmSource, asmSourceLen, asmOutput, asmOutputCap uint32
	asmStatus, asmOutputLen, asmErrorLine            uint32
}

// New returns a machine with ramSize bytes of RAM whose CPU starts at entry,
// with interrupts enabled.
func New(ramSize int, entry uint32) *Machine {
	m := &Machine{
		RAM:       vm.NewRAM(ramSize),
		Immediate: vm.NewRAM(ImmediateSize),
		intEnable: true,
	}
	m.CPU = vm.NewCPU(m, entry)
	return m
}

// LoadELF loads an executable into RAM and points the CPU at its entry.
func (m *Machine) LoadELF(data []byte) error {
	entry, err := vm.LoadELF(data, m.RAM)
	if err != nil {
		return err
	}
	m.CPU.Entry = entry
	m.CPU.Restart()
	return nil
}

// Run executes up to budget instructions, taking pending interrupts
// between instructions. Entering and returning from an interrupt cost no
// instructions.
func (m *Machine) Run(budget int) {
	for m.remaining = budget; m.remaining > 0; {
		m.remaining--
		m.takeInterrupt()
		ok := m.CPU.Step()
		switch {
		case !ok:
			// Illegal instruction: the CPU restarted. Leave any handler.
			m.intEnable = true
		case m.returning:
			m.CPU.Regs = m.savedRegs
			m.CPU.Regs[0] = 0
			m.CPU.PC = m.savedPC
			m.intEnable = true
		}
		m.returning = false
	}
}

func (m *Machine) takeInterrupt() {
	if !m.intEnable {
		return
	}
	var target uint32
	switch {
	case m.keyHandler != 0 && len(m.keys) > 0:
		target = m.keyHandler
	case m.keyHandler == 0 && m.pendingLine != nil:
		clear(m.Immediate)
		for i, w := range m.pendingLine {
			m.Immediate.Write(uint32(4*i), 4, w)
		}
		m.pendingLine = nil
		target = ImmediateBase
	default:
		return
	}
	m.savedRegs = m.CPU.Regs
	m.savedPC = m.CPU.PC
	m.intEnable = false
	m.CPU.PC = target
}

// Type delivers one typed character to the machine.
func (m *Machine) Type(c byte) {
	if m.keyHandler != 0 {
		if len(m.keys) < KeyBufferSize {
			m.keys = append(m.keys, c)
		} else {
			m.keyOverflow = true
		}
		return
	}
	switch c {
	case Backspace, Delete:
		if len(m.line) > 0 {
			m.line = m.line[:len(m.line)-1]
		}
	case Enter, '\r':
		m.submitLine()
	default:
		if len(m.line) < KeyBufferSize {
			m.line = append(m.line, c)
		} else {
			m.keyOverflow = true
		}
	}
}

// submitLine assembles the edited line. A line that fails to assemble or
// does not fit in the immediate-code region does nothing.
func (m *Machine) submitLine() {
	src := string(m.line)
	m.line = m.line[:0]
	code, err := asm.Assemble(src+returnTrailer, ImmediateBase)
	if err != nil || len(code) == 2 || 4*len(code) > ImmediateSize {
		return
	}
	if m.pendingLine != nil {
		m.keyOverflow = true
		return
	}
	m.pendingLine = code
}

// assemble runs one assembler window call.
func (m *Machine) assemble() {
	m.asmOutputLen, m.asmErrorLine = 0, 0
	if m.PayAssembler != nil && !m.PayAssembler() {
		m.asmStatus = AssemblerMana
		return
	}
	src, ok := m.ramSlice(m.asmSource, m.asmSourceLen)
	if !ok || m.asmSourceLen > MaxAssemblerSource {
		m.asmStatus = AssemblerRange
		return
	}
	code, err := asm.Assemble(string(src), m.asmOutput)
	if err != nil {
		m.asmStatus = AssemblerSyntax
		if e, ok := err.(*asm.Error); ok {
			m.asmErrorLine = uint32(e.Line)
		}
		return
	}
	n := uint32(4 * len(code))
	out, ok := m.ramSlice(m.asmOutput, m.asmOutputCap)
	if !ok || n > m.asmOutputCap {
		m.asmStatus = AssemblerRange
		return
	}
	for i, w := range code {
		vm.RAM(out).Write(uint32(4*i), 4, w)
	}
	m.asmStatus = AssemblerOK
	m.asmOutputLen = n
}

// ramSlice returns RAM[addr:addr+n] if it lies inside RAM.
func (m *Machine) ramSlice(addr, n uint32) ([]byte, bool) {
	size := uint64(len(m.RAM))
	if uint64(addr) > size || uint64(n) > size-uint64(addr) {
		return nil, false
	}
	return m.RAM[addr : addr+n], true
}
