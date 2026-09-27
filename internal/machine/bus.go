package machine

type region struct {
	base, size uint32
	// read returns a register. With peek set, it must not have side effects.
	read  func(m *Machine, off uint32, peek bool) uint32
	write func(m *Machine, off, v uint32)
}

var regions = []region{
	{SystemBase, systemSize, (*Machine).readSystem, (*Machine).writeSystem},
	{KeyboardBase, keyboardSize, (*Machine).readKeyboard, (*Machine).writeKeyboard},
	{AssemblerBase, assemblerSize, (*Machine).readAssembler, (*Machine).writeAssembler},
}

func regionAt(a uint32) *region {
	for i := range regions {
		if a-regions[i].base < regions[i].size {
			return &regions[i]
		}
	}
	return nil
}

// memAt returns the plain memory byte at a, if any.
func (m *Machine) memAt(a uint32) *byte {
	switch {
	case uint64(a) < uint64(len(m.RAM)):
		return &m.RAM[a]
	case a-ImmediateBase < ImmediateSize:
		return &m.Immediate[a-ImmediateBase]
	}
	return nil
}

// Read implements vm.Bus. Unmapped bytes read as 0. Registers are 32-bit
// words; each register an access touches is read once, so reading KeyNext
// pops one character per access.
func (m *Machine) Read(addr uint32, size int) uint32 {
	var v uint32
	for i := 0; i < size; {
		a := addr + uint32(i)
		if p := m.memAt(a); p != nil {
			v |= uint32(*p) << (8 * i)
			i++
			continue
		}
		r := regionAt(a)
		if r == nil {
			i++
			continue
		}
		reg := a &^ 3
		word := r.read(m, reg-r.base, false)
		for ; i < size && (addr+uint32(i))&^3 == reg; i++ {
			v |= (word >> (8 * ((addr + uint32(i)) & 3)) & 0xff) << (8 * i)
		}
	}
	return v
}

// Write implements vm.Bus. Unmapped bytes are ignored. A partial write to a
// register replaces only the written bytes of its word, and any write to a
// trigger register (return, assembler command) triggers it once.
func (m *Machine) Write(addr uint32, size int, value uint32) {
	for i := 0; i < size; {
		a := addr + uint32(i)
		if p := m.memAt(a); p != nil {
			*p = byte(value >> (8 * i))
			i++
			continue
		}
		r := regionAt(a)
		if r == nil {
			i++
			continue
		}
		reg := a &^ 3
		word := r.read(m, reg-r.base, true)
		for ; i < size && (addr+uint32(i))&^3 == reg; i++ {
			shift := 8 * ((addr + uint32(i)) & 3)
			word = word&^(0xff<<shift) | (value>>(8*i)&0xff)<<shift
		}
		r.write(m, reg-r.base, word)
	}
}

func (m *Machine) readSystem(off uint32, _ bool) uint32 {
	switch {
	case off == SysTick:
		return m.Tick
	case off == SysRemaining:
		return uint32(m.remaining)
	case off == SysPlayer:
		return m.Player
	case off == SysIntEnable:
		return boolToU32(m.intEnable)
	case off == SysSavedPC:
		return m.savedPC
	case off == SysKeyHandler:
		return m.keyHandler
	case off >= SysSavedRegs:
		return m.savedRegs[(off-SysSavedRegs)/4]
	}
	return 0
}

func (m *Machine) writeSystem(off, v uint32) {
	switch {
	case off == SysIntEnable:
		m.intEnable = v&1 != 0
	case off == SysSavedPC:
		m.savedPC = v
	case off == SysKeyHandler:
		m.keyHandler = v
	case off == SysReturn:
		m.returning = true
	case off >= SysSavedRegs:
		m.savedRegs[(off-SysSavedRegs)/4] = v
	}
}

func (m *Machine) readKeyboard(off uint32, peek bool) uint32 {
	switch off {
	case KeyCount:
		return uint32(len(m.keys))
	case KeyNext:
		if len(m.keys) == 0 {
			return 0
		}
		c := m.keys[0]
		if !peek {
			m.keys = m.keys[1:]
		}
		return uint32(c)
	case KeyOverflow:
		return boolToU32(m.keyOverflow)
	}
	return 0
}

func (m *Machine) writeKeyboard(off, v uint32) {
	if off == KeyOverflow {
		m.keyOverflow = v&1 != 0
	}
}

func (m *Machine) readAssembler(off uint32, _ bool) uint32 {
	switch off {
	case AsmSource:
		return m.asmSource
	case AsmSourceLen:
		return m.asmSourceLen
	case AsmOutput:
		return m.asmOutput
	case AsmOutputCap:
		return m.asmOutputCap
	case AsmStatus:
		return m.asmStatus
	case AsmOutputLen:
		return m.asmOutputLen
	case AsmErrorLine:
		return m.asmErrorLine
	}
	return 0
}

func (m *Machine) writeAssembler(off, v uint32) {
	switch off {
	case AsmSource:
		m.asmSource = v
	case AsmSourceLen:
		m.asmSourceLen = v
	case AsmOutput:
		m.asmOutput = v
	case AsmOutputCap:
		m.asmOutputCap = v
	case AsmCommand:
		m.assemble()
	}
}

func boolToU32(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}
