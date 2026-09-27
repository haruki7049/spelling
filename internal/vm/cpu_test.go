package vm

import "testing"

// Instruction encoders for building test programs.

func encR(funct7, rs2, rs1, funct3, rd, opcode uint32) uint32 {
	return funct7<<25 | rs2<<20 | rs1<<15 | funct3<<12 | rd<<7 | opcode
}

func encI(imm int32, rs1, funct3, rd, opcode uint32) uint32 {
	return uint32(imm)<<20 | rs1<<15 | funct3<<12 | rd<<7 | opcode
}

func encS(imm int32, rs2, rs1, funct3, opcode uint32) uint32 {
	u := uint32(imm)
	return (u>>5&0x7f)<<25 | rs2<<20 | rs1<<15 | funct3<<12 | (u&0x1f)<<7 | opcode
}

func encB(imm int32, rs2, rs1, funct3 uint32) uint32 {
	u := uint32(imm)
	return (u>>12&1)<<31 | (u>>5&0x3f)<<25 | rs2<<20 | rs1<<15 | funct3<<12 |
		(u>>1&0xf)<<8 | (u>>11&1)<<7 | opBranch
}

func encU(imm uint32, rd, opcode uint32) uint32 {
	return imm&0xfffff000 | rd<<7 | opcode
}

func encJ(imm int32, rd uint32) uint32 {
	u := uint32(imm)
	return (u>>20&1)<<31 | (u>>1&0x3ff)<<21 | (u>>11&1)<<20 | (u>>12&0xff)<<12 | rd<<7 | opJAL
}

func addi(rd, rs1 uint32, imm int32) uint32 { return encI(imm, rs1, 0b000, rd, opImm) }

var (
	ecall  = uint32(0x00000073)
	ebreak = uint32(0x00100073)
	fence  = uint32(0x0ff0000f)
)

// newTestCPU loads program at address 0 of a 4 KiB RAM.
func newTestCPU(t *testing.T, program ...uint32) (*CPU, RAM) {
	t.Helper()
	ram := NewRAM(4096)
	for i, inst := range program {
		ram.Write(uint32(4*i), 4, inst)
	}
	return NewCPU(ram, 0), ram
}

func step(t *testing.T, c *CPU, n int) {
	t.Helper()
	for i := range n {
		if !c.Step() {
			t.Fatalf("step %d: unexpected illegal instruction", i)
		}
	}
}

func TestEncodersMatchKnownMachineCode(t *testing.T) {
	tests := []struct {
		name string
		got  uint32
		want uint32
	}{
		{"addi x1, x0, 5", addi(1, 0, 5), 0x00500093},
		{"addi x1, x1, -1", addi(1, 1, -1), 0xfff08093},
		{"lui x1, 0x12345", encU(0x12345000, 1, opLUI), 0x123450b7},
		{"sw x2, 8(x1)", encS(8, 2, 1, 0b010, opStore), 0x0020a423},
		{"beq x1, x2, -4", encB(-4, 2, 1, 0b000), 0xfe208ee3},
		{"jal x1, 2048", encJ(2048, 1), 0x001000ef},
		{"sub x3, x1, x2", encR(0b0100000, 2, 1, 0b000, 3, opReg), 0x402081b3},
		{"sw x5, -2048(x6)", encS(-2048, 5, 6, 0b010, opStore), 0x80532023},
		{"sh x5, 2047(x6)", encS(2047, 5, 6, 0b001, opStore), 0x7e531fa3},
		{"srai x9, x10, 31", encI(0x400|31, 10, 0b101, 9, opImm), 0x41f55493},
		{"beq x1, x2, -2048", encB(-2048, 2, 1, 0b000), 0x802080e3},
		{"bne x3, x4, 4090", encB(4090, 4, 3, 0b001), 0x7e419de3},
		{"blt x1, x2, -4094", encB(-4094, 2, 1, 0b100), 0x8020c163},
		{"jal x1, -6146", encJ(-6146, 1), 0xffefe0ef},
		{"jal x0, 12", encJ(12, 0), 0x00c0006f},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %#08x, want %#08x", tt.name, tt.got, tt.want)
		}
	}
}

// TestDecodesAssemblerOutput executes machine code produced by GNU as
// (riscv32-none-elf-as -march=rv32i) and checks the resulting PC, so the
// immediate decoding is verified independently of the test encoders.
func TestDecodesAssemblerOutput(t *testing.T) {
	tests := []struct {
		name   string
		inst   uint32
		pc     uint32
		x2     uint32 // x1 is 1, x3 is 3, x4 is 4
		wantPC uint32
	}{
		{"beq x1, x2, -2048", 0x802080e3, 0x800, 1, 0x0},
		{"bne x3, x4, 4090", 0x7e419de3, 0x804, 0, 0x17fe},
		{"blt x1, x2, -4094", 0x8020c163, 0x17fe, 2, 0x800},
		{"jal x1, -6146", 0xffefe0ef, 0x1802, 0, 0x0},
		{"jal x0, 12", 0x00c0006f, 0x1806, 0, 0x1812},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ram := NewRAM(0x2000)
			ram.Write(tt.pc, 4, tt.inst)
			c := NewCPU(ram, 0)
			c.PC = tt.pc
			c.Regs[1], c.Regs[2], c.Regs[3], c.Regs[4] = 1, tt.x2, 3, 4
			step(t, c, 1)
			if c.PC != tt.wantPC {
				t.Errorf("PC = %#x, want %#x", c.PC, tt.wantPC)
			}
		})
	}
}

func TestArithmeticAndLogic(t *testing.T) {
	var a, b uint32 = 0xfffffff6, 3 // a is -10
	tests := []struct {
		name string
		inst uint32
		want uint32
	}{
		{"add", encR(0, 2, 1, 0b000, 3, opReg), a + b},
		{"sub", encR(0b0100000, 2, 1, 0b000, 3, opReg), a - b},
		{"sll", encR(0, 2, 1, 0b001, 3, opReg), a << b},
		{"slt", encR(0, 2, 1, 0b010, 3, opReg), 1},
		{"sltu", encR(0, 2, 1, 0b011, 3, opReg), 0},
		{"xor", encR(0, 2, 1, 0b100, 3, opReg), a ^ b},
		{"srl", encR(0, 2, 1, 0b101, 3, opReg), a >> b},
		{"sra", encR(0b0100000, 2, 1, 0b101, 3, opReg), 0xfffffffe},
		{"or", encR(0, 2, 1, 0b110, 3, opReg), a | b},
		{"and", encR(0, 2, 1, 0b111, 3, opReg), a & b},
		{"addi negative", encI(-20, 1, 0b000, 3, opImm), 0xffffffe2},
		{"slti", encI(-9, 1, 0b010, 3, opImm), 1},
		{"sltiu", encI(-1, 1, 0b011, 3, opImm), 1},
		{"xori", encI(-1, 1, 0b100, 3, opImm), ^uint32(a)},
		{"ori", encI(0x0f, 1, 0b110, 3, opImm), a | 0x0f},
		{"andi", encI(0x0f, 1, 0b111, 3, opImm), a & 0x0f},
		{"slli", encI(4, 1, 0b001, 3, opImm), a << 4},
		{"srli", encI(4, 1, 0b101, 3, opImm), a >> 4},
		{"srai", encI(0x400|4, 1, 0b101, 3, opImm), 0xffffffff},
		{"lui", encU(0xabcde000, 3, opLUI), 0xabcde000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestCPU(t, tt.inst)
			c.Regs[1], c.Regs[2] = a, b
			step(t, c, 1)
			if c.Regs[3] != tt.want {
				t.Errorf("x3 = %#08x, want %#08x", c.Regs[3], tt.want)
			}
			if c.PC != 4 {
				t.Errorf("PC = %d, want 4", c.PC)
			}
		})
	}
}

func TestShiftUsesLowFiveBits(t *testing.T) {
	c, _ := newTestCPU(t, encR(0, 2, 1, 0b001, 3, opReg))
	c.Regs[1], c.Regs[2] = 1, 33
	step(t, c, 1)
	if c.Regs[3] != 2 {
		t.Errorf("x3 = %d, want 2", c.Regs[3])
	}
}

func TestAUIPC(t *testing.T) {
	c, _ := newTestCPU(t, addi(0, 0, 0), encU(0x1000, 3, opAUIPC))
	step(t, c, 2)
	if c.Regs[3] != 0x1004 {
		t.Errorf("x3 = %#x, want 0x1004", c.Regs[3])
	}
}

func TestX0IsAlwaysZero(t *testing.T) {
	c, _ := newTestCPU(t, addi(0, 0, 5), encU(0x1000, 0, opLUI), encJ(4, 0))
	step(t, c, 3)
	if c.Regs[0] != 0 {
		t.Errorf("x0 = %d, want 0", c.Regs[0])
	}
}

func TestLoadsAndStores(t *testing.T) {
	c, ram := newTestCPU(t,
		encS(0, 2, 1, 0b010, opStore),  // sw x2, 0(x1)
		encI(0, 1, 0b000, 3, opLoad),   // lb x3, 0(x1)
		encI(0, 1, 0b100, 4, opLoad),   // lbu x4, 0(x1)
		encI(2, 1, 0b001, 5, opLoad),   // lh x5, 2(x1)
		encI(2, 1, 0b101, 6, opLoad),   // lhu x6, 2(x1)
		encI(0, 1, 0b010, 7, opLoad),   // lw x7, 0(x1)
		encS(-4, 2, 1, 0b000, opStore), // sb x2, -4(x1)
		encS(-8, 2, 1, 0b001, opStore), // sh x2, -8(x1)
		encI(-4, 1, 0b010, 8, opLoad),  // lw x8, -4(x1)
		encI(-8, 1, 0b010, 9, opLoad),  // lw x9, -8(x1)
	)
	c.Regs[1], c.Regs[2] = 0x800, 0x8765_43f0
	step(t, c, 10)

	if got := ram.Read(0x800, 4); got != 0x876543f0 {
		t.Errorf("memory = %#08x, want 0x876543f0", got)
	}
	want := map[int]uint32{
		3: 0xfffffff0, // lb sign-extends
		4: 0xf0,       // lbu zero-extends
		5: 0xffff8765, // lh sign-extends
		6: 0x8765,     // lhu zero-extends
		7: 0x876543f0,
		8: 0xf0,   // sb stores the low byte only
		9: 0x43f0, // sh stores the low half only
	}
	for r, w := range want {
		if c.Regs[r] != w {
			t.Errorf("x%d = %#08x, want %#08x", r, c.Regs[r], w)
		}
	}
}

func TestMisalignedAccessIsAllowed(t *testing.T) {
	c, ram := newTestCPU(t,
		encS(1, 2, 1, 0b010, opStore), // sw x2, 1(x1)
		encI(1, 1, 0b010, 3, opLoad),  // lw x3, 1(x1)
	)
	c.Regs[1], c.Regs[2] = 0x800, 0x11223344
	step(t, c, 2)
	if c.Regs[3] != 0x11223344 {
		t.Errorf("x3 = %#08x, want 0x11223344", c.Regs[3])
	}
	if got := ram.Read(0x800, 1); got != 0 {
		t.Errorf("byte before the store = %#x, want 0", got)
	}
}

func TestUnmappedAccess(t *testing.T) {
	c, ram := newTestCPU(t,
		encS(0, 2, 1, 0b010, opStore), // sw x2, 0(x1)
		encI(0, 1, 0b010, 3, opLoad),  // lw x3, 0(x1)
		encS(0, 2, 4, 0b010, opStore), // sw x2, 0(x4)
		encI(0, 4, 0b010, 5, opLoad),  // lw x5, 0(x4)
	)
	c.Regs[1] = 0x1000_0000 // outside RAM
	c.Regs[2] = 0xdeadbeef
	c.Regs[4] = uint32(len(ram) - 2) // straddles the end of RAM
	c.Regs[3] = 1
	step(t, c, 4)
	if c.Regs[3] != 0 {
		t.Errorf("unmapped read = %#x, want 0", c.Regs[3])
	}
	if c.Regs[5] != 0xbeef {
		t.Errorf("read straddling the end = %#x, want 0xbeef", c.Regs[5])
	}
}

func TestBranches(t *testing.T) {
	tests := []struct {
		name   string
		funct3 uint32
		a, b   uint32
		taken  bool
	}{
		{"beq taken", 0b000, 5, 5, true},
		{"beq not taken", 0b000, 5, 6, false},
		{"bne taken", 0b001, 5, 6, true},
		{"blt signed", 0b100, 0xffffffff, 1, true},
		{"bge signed", 0b101, 0xffffffff, 1, false},
		{"bltu unsigned", 0b110, 0xffffffff, 1, false},
		{"bgeu unsigned", 0b111, 0xffffffff, 1, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := newTestCPU(t, addi(0, 0, 0), encB(-4, 2, 1, tt.funct3))
			c.PC = 4
			c.Regs[1], c.Regs[2] = tt.a, tt.b
			step(t, c, 1)
			want := uint32(8)
			if tt.taken {
				want = 0
			}
			if c.PC != want {
				t.Errorf("PC = %d, want %d", c.PC, want)
			}
		})
	}
}

func TestJumps(t *testing.T) {
	t.Run("jal", func(t *testing.T) {
		c, _ := newTestCPU(t, encJ(16, 1))
		step(t, c, 1)
		if c.PC != 16 || c.Regs[1] != 4 {
			t.Errorf("PC = %d, x1 = %d, want 16, 4", c.PC, c.Regs[1])
		}
	})
	t.Run("jalr clears bit 0", func(t *testing.T) {
		c, _ := newTestCPU(t, encI(3, 2, 0b000, 1, opJALR))
		c.Regs[2] = 0x100
		step(t, c, 1)
		if c.PC != 0x102 || c.Regs[1] != 4 {
			t.Errorf("PC = %#x, x1 = %d, want 0x102, 4", c.PC, c.Regs[1])
		}
	})
	t.Run("jalr with rd == rs1 uses the old value", func(t *testing.T) {
		c, _ := newTestCPU(t, encI(0, 1, 0b000, 1, opJALR))
		c.Regs[1] = 0x200
		step(t, c, 1)
		if c.PC != 0x200 || c.Regs[1] != 4 {
			t.Errorf("PC = %#x, x1 = %d, want 0x200, 4", c.PC, c.Regs[1])
		}
	})
}

func TestNoOps(t *testing.T) {
	for name, inst := range map[string]uint32{"ecall": ecall, "ebreak": ebreak, "fence": fence} {
		t.Run(name, func(t *testing.T) {
			c, _ := newTestCPU(t, inst)
			c.Regs[5] = 42
			step(t, c, 1)
			if c.PC != 4 || c.Regs[5] != 42 {
				t.Errorf("PC = %d, x5 = %d, want 4, 42", c.PC, c.Regs[5])
			}
		})
	}
}

func TestIllegalInstructionRestarts(t *testing.T) {
	tests := map[string]uint32{
		"all zeros":           0x00000000,
		"compressed":          0x00000001,
		"unknown opcode":      0x0000007f,
		"csrrw (Zicsr)":       0x34001073,
		"fence.i (Zifencei)":  0x0000100f,
		"mul (M extension)":   encR(0b0000001, 2, 1, 0b000, 3, opReg),
		"slli with funct7":    encI(0x400|1, 1, 0b001, 3, opImm),
		"jalr with funct3":    encI(0, 1, 0b001, 3, opJALR),
		"branch funct3 010":   encB(8, 2, 1, 0b010),
		"load funct3 011":     encI(0, 1, 0b011, 3, opLoad),
		"store funct3 011":    encS(0, 2, 1, 0b011, opStore),
		"ecall with rd set":   ecall | 1<<7,
		"srai bad funct7":     encI(0x200|1, 1, 0b101, 3, opImm),
		"add with bad funct7": encR(0b0000010, 2, 1, 0b000, 3, opReg),
	}
	for name, inst := range tests {
		t.Run(name, func(t *testing.T) {
			ram := NewRAM(4096)
			ram.Write(0x100, 4, inst)
			c := NewCPU(ram, 0x40)
			c.PC = 0x100
			for i := range c.Regs {
				c.Regs[i] = 7
			}

			if c.Step() {
				t.Fatalf("Step() = true, want false")
			}
			if c.PC != 0x40 {
				t.Errorf("PC = %#x, want entry 0x40", c.PC)
			}
			if c.Regs != [32]uint32{} {
				t.Errorf("registers not cleared: %v", c.Regs)
			}
			if got := ram.Read(0x100, 4); got != inst {
				t.Errorf("memory changed on restart: %#08x", got)
			}
		})
	}
}

// TestProgramSumsOneToTen runs a loop built from a branch and a jump:
//
//	    addi x1, x0, 10   # counter
//	    addi x2, x0, 0    # sum
//	loop:
//	    beq  x1, x0, end
//	    add  x2, x2, x1
//	    addi x1, x1, -1
//	    jal  x0, loop
//	end:
//	    jal  x0, end
func TestProgramSumsOneToTen(t *testing.T) {
	c, _ := newTestCPU(t,
		addi(1, 0, 10),
		addi(2, 0, 0),
		encB(16, 0, 1, 0b000),
		encR(0, 1, 2, 0b000, 2, opReg),
		addi(1, 1, -1),
		encJ(-12, 0),
		encJ(0, 0),
	)
	step(t, c, 100)
	if c.Regs[2] != 55 {
		t.Errorf("sum = %d, want 55", c.Regs[2])
	}
	if c.PC != 24 {
		t.Errorf("PC = %d, want 24 (spinning at end)", c.PC)
	}
}

func TestWFI(t *testing.T) {
	c, _ := newTestCPU(t, 0x10500073)
	c.Regs[5] = 42
	step(t, c, 1)
	if c.PC != 4 || c.Regs[5] != 42 || !c.Waiting {
		t.Errorf("PC = %d, x5 = %d, waiting = %v; want 4, 42, true", c.PC, c.Regs[5], c.Waiting)
	}
}
