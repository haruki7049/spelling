package asm

import (
	"fmt"
	"strings"
)

// Opcodes of the RV32I base instruction set.
const (
	opLoad    = 0b0000011
	opMiscMem = 0b0001111
	opImm     = 0b0010011
	opAUIPC   = 0b0010111
	opStore   = 0b0100011
	opReg     = 0b0110011
	opLUI     = 0b0110111
	opBranch  = 0b1100011
	opJALR    = 0b1100111
	opJAL     = 0b1101111
	opSystem  = 0b1110011
)

const ra = 1

type encoder func(s statement, labels map[string]uint32) ([]uint32, error)

var encoders map[string]encoder

func init() {
	encoders = map[string]encoder{
		"lui":   upper(opLUI),
		"auipc": upper(opAUIPC),
		"jal":   encJAL,
		"jalr":  encJALR,

		"beq":  branch(0b000),
		"bne":  branch(0b001),
		"blt":  branch(0b100),
		"bge":  branch(0b101),
		"bltu": branch(0b110),
		"bgeu": branch(0b111),

		"lb":  load(0b000),
		"lh":  load(0b001),
		"lw":  load(0b010),
		"lbu": load(0b100),
		"lhu": load(0b101),
		"sb":  store(0b000),
		"sh":  store(0b001),
		"sw":  store(0b010),

		"addi":  aluImm(0b000),
		"slti":  aluImm(0b010),
		"sltiu": aluImm(0b011),
		"xori":  aluImm(0b100),
		"ori":   aluImm(0b110),
		"andi":  aluImm(0b111),
		"slli":  shiftImm(0b001, 0),
		"srli":  shiftImm(0b101, 0),
		"srai":  shiftImm(0b101, 0b0100000),

		"add":  aluReg(0b000, 0),
		"sub":  aluReg(0b000, 0b0100000),
		"sll":  aluReg(0b001, 0),
		"slt":  aluReg(0b010, 0),
		"sltu": aluReg(0b011, 0),
		"xor":  aluReg(0b100, 0),
		"srl":  aluReg(0b101, 0),
		"sra":  aluReg(0b101, 0b0100000),
		"or":   aluReg(0b110, 0),
		"and":  aluReg(0b111, 0),

		"fence":  encFence,
		"ecall":  fixed(0x00000073),
		"ebreak": fixed(0x00100073),
		"wfi":    fixed(0x10500073),

		// Pseudo-instructions.
		"nop":  fixed(encI(0, 0, 0b000, 0, opImm)),
		"ret":  fixed(encI(0, ra, 0b000, 0, opJALR)),
		"li":   encLI,
		"la":   encLA,
		"mv":   encMV,
		"j":    encJ,
		"call": encCall,
		"beqz": branchZero(0b000),
		"bnez": branchZero(0b001),
	}
}

func encode(s statement, labels map[string]uint32) ([]uint32, error) {
	return encoders[s.mnemonic](s, labels)
}

func encR(funct7, rs2, rs1, funct3, rd, opcode uint32) uint32 {
	return funct7<<25 | rs2<<20 | rs1<<15 | funct3<<12 | rd<<7 | opcode
}

func encI(imm int64, rs1, funct3, rd, opcode uint32) uint32 {
	return uint32(imm)<<20 | rs1<<15 | funct3<<12 | rd<<7 | opcode
}

func encS(imm int64, rs2, rs1, funct3 uint32) uint32 {
	u := uint32(imm)
	return (u>>5&0x7f)<<25 | rs2<<20 | rs1<<15 | funct3<<12 | (u&0x1f)<<7 | opStore
}

func encB(imm int64, rs2, rs1, funct3 uint32) uint32 {
	u := uint32(imm)
	return (u>>12&1)<<31 | (u>>5&0x3f)<<25 | rs2<<20 | rs1<<15 | funct3<<12 |
		(u>>1&0xf)<<8 | (u>>11&1)<<7 | opBranch
}

func encU(imm20, rd, opcode uint32) uint32 {
	return imm20<<12 | rd<<7 | opcode
}

func encJType(imm int64, rd uint32) uint32 {
	u := uint32(imm)
	return (u>>20&1)<<31 | (u>>1&0x3ff)<<21 | (u>>11&1)<<20 | (u>>12&0xff)<<12 | rd<<7 | opJAL
}

func wantArgs(s statement, n int) error {
	if len(s.args) != n {
		return fmt.Errorf("%s: want %d operands, got %d", s.mnemonic, n, len(s.args))
	}
	return nil
}

func regs(args ...string) ([]uint32, error) {
	out := make([]uint32, len(args))
	for i, a := range args {
		r, err := parseReg(a)
		if err != nil {
			return nil, err
		}
		out[i] = r
	}
	return out, nil
}

// pcOffset resolves a target and checks it fits a signed immediate of bits
// width and is a multiple of 2.
func pcOffset(arg string, pc uint32, labels map[string]uint32, bits uint) (int64, error) {
	off, err := target(arg, pc, labels)
	if err != nil {
		return 0, err
	}
	if off%2 != 0 {
		return 0, fmt.Errorf("target offset %d is not a multiple of 2", off)
	}
	if !fitsSigned(off, bits) {
		return 0, fmt.Errorf("target offset %d out of range", off)
	}
	return off, nil
}

func fixed(inst uint32) encoder {
	return func(s statement, _ map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 0); err != nil {
			return nil, err
		}
		return []uint32{inst}, nil
	}
}

func upper(opcode uint32) encoder {
	return func(s statement, _ map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 2); err != nil {
			return nil, err
		}
		rd, err := parseReg(s.args[0])
		if err != nil {
			return nil, err
		}
		imm, err := parseImm(s.args[1], 0, 0xfffff)
		if err != nil {
			return nil, err
		}
		return []uint32{encU(uint32(imm), rd, opcode)}, nil
	}
}

func encJAL(s statement, labels map[string]uint32) ([]uint32, error) {
	rd, arg := uint32(ra), ""
	switch len(s.args) {
	case 1:
		arg = s.args[0]
	case 2:
		r, err := parseReg(s.args[0])
		if err != nil {
			return nil, err
		}
		rd, arg = r, s.args[1]
	default:
		return nil, fmt.Errorf("jal: want 1 or 2 operands, got %d", len(s.args))
	}
	off, err := pcOffset(arg, s.addr, labels, 21)
	if err != nil {
		return nil, err
	}
	return []uint32{encJType(off, rd)}, nil
}

func encJALR(s statement, _ map[string]uint32) ([]uint32, error) {
	var rd, rs1 uint32
	var off int64
	var err error
	switch len(s.args) {
	case 1: // jalr rs1
		rd = ra
		rs1, err = parseReg(s.args[0])
	case 2: // jalr rd, offset(rs1)
		if rd, err = parseReg(s.args[0]); err == nil {
			off, rs1, err = parseMem(s.args[1])
		}
	case 3: // jalr rd, rs1, offset
		var r []uint32
		if r, err = regs(s.args[0], s.args[1]); err == nil {
			rd, rs1 = r[0], r[1]
			off, err = parseImm(s.args[2], -2048, 2047)
		}
	default:
		return nil, fmt.Errorf("jalr: want 1 to 3 operands, got %d", len(s.args))
	}
	if err != nil {
		return nil, err
	}
	return []uint32{encI(off, rs1, 0b000, rd, opJALR)}, nil
}

func branch(funct3 uint32) encoder {
	return func(s statement, labels map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 3); err != nil {
			return nil, err
		}
		r, err := regs(s.args[0], s.args[1])
		if err != nil {
			return nil, err
		}
		off, err := pcOffset(s.args[2], s.addr, labels, 13)
		if err != nil {
			return nil, err
		}
		return []uint32{encB(off, r[1], r[0], funct3)}, nil
	}
}

func branchZero(funct3 uint32) encoder {
	return func(s statement, labels map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 2); err != nil {
			return nil, err
		}
		rs1, err := parseReg(s.args[0])
		if err != nil {
			return nil, err
		}
		off, err := pcOffset(s.args[1], s.addr, labels, 13)
		if err != nil {
			return nil, err
		}
		return []uint32{encB(off, 0, rs1, funct3)}, nil
	}
}

func load(funct3 uint32) encoder {
	return func(s statement, _ map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 2); err != nil {
			return nil, err
		}
		rd, err := parseReg(s.args[0])
		if err != nil {
			return nil, err
		}
		off, rs1, err := parseMem(s.args[1])
		if err != nil {
			return nil, err
		}
		return []uint32{encI(off, rs1, funct3, rd, opLoad)}, nil
	}
}

func store(funct3 uint32) encoder {
	return func(s statement, _ map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 2); err != nil {
			return nil, err
		}
		rs2, err := parseReg(s.args[0])
		if err != nil {
			return nil, err
		}
		off, rs1, err := parseMem(s.args[1])
		if err != nil {
			return nil, err
		}
		return []uint32{encS(off, rs2, rs1, funct3)}, nil
	}
}

func aluImm(funct3 uint32) encoder {
	return func(s statement, _ map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 3); err != nil {
			return nil, err
		}
		r, err := regs(s.args[0], s.args[1])
		if err != nil {
			return nil, err
		}
		imm, err := parseImm(s.args[2], -2048, 2047)
		if err != nil {
			return nil, err
		}
		return []uint32{encI(imm, r[1], funct3, r[0], opImm)}, nil
	}
}

func shiftImm(funct3, funct7 uint32) encoder {
	return func(s statement, _ map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 3); err != nil {
			return nil, err
		}
		r, err := regs(s.args[0], s.args[1])
		if err != nil {
			return nil, err
		}
		shamt, err := parseImm(s.args[2], 0, 31)
		if err != nil {
			return nil, err
		}
		return []uint32{encR(funct7, uint32(shamt), r[1], funct3, r[0], opImm)}, nil
	}
}

func aluReg(funct3, funct7 uint32) encoder {
	return func(s statement, _ map[string]uint32) ([]uint32, error) {
		if err := wantArgs(s, 3); err != nil {
			return nil, err
		}
		r, err := regs(s.args...)
		if err != nil {
			return nil, err
		}
		return []uint32{encR(funct7, r[2], r[1], funct3, r[0], opReg)}, nil
	}
}

func encLI(s statement, _ map[string]uint32) ([]uint32, error) {
	rd, err := parseReg(s.args[0])
	if err != nil {
		return nil, err
	}
	v, _ := parseImm(s.args[1], -1<<31, 1<<32-1) // validated by size
	if fitsSigned(int64(int32(v)), 12) {
		return []uint32{encI(int64(int32(v)), 0, 0b000, rd, opImm)}, nil
	}
	hi, lo := splitHiLo(uint32(v))
	if lo == 0 {
		return []uint32{encU(hi, rd, opLUI)}, nil
	}
	return []uint32{
		encU(hi, rd, opLUI),
		encI(lo, rd, 0b000, rd, opImm),
	}, nil
}

func encLA(s statement, labels map[string]uint32) ([]uint32, error) {
	if err := wantArgs(s, 2); err != nil {
		return nil, err
	}
	rd, err := parseReg(s.args[0])
	if err != nil {
		return nil, err
	}
	if !isIdent(s.args[1]) {
		return nil, fmt.Errorf("la: want a label, got %q", s.args[1])
	}
	off, err := target(s.args[1], s.addr, labels)
	if err != nil {
		return nil, err
	}
	hi, lo := splitHiLo(uint32(off))
	return []uint32{
		encU(hi, rd, opAUIPC),
		encI(lo, rd, 0b000, rd, opImm),
	}, nil
}

func encCall(s statement, labels map[string]uint32) ([]uint32, error) {
	if err := wantArgs(s, 1); err != nil {
		return nil, err
	}
	if !isIdent(s.args[0]) {
		return nil, fmt.Errorf("call: want a label, got %q", s.args[0])
	}
	off, err := target(s.args[0], s.addr, labels)
	if err != nil {
		return nil, err
	}
	hi, lo := splitHiLo(uint32(off))
	return []uint32{
		encU(hi, ra, opAUIPC),
		encI(lo, ra, 0b000, ra, opJALR),
	}, nil
}

func encMV(s statement, _ map[string]uint32) ([]uint32, error) {
	if err := wantArgs(s, 2); err != nil {
		return nil, err
	}
	r, err := regs(s.args...)
	if err != nil {
		return nil, err
	}
	return []uint32{encI(0, r[1], 0b000, r[0], opImm)}, nil
}

func encJ(s statement, labels map[string]uint32) ([]uint32, error) {
	if err := wantArgs(s, 1); err != nil {
		return nil, err
	}
	off, err := pcOffset(s.args[0], s.addr, labels, 21)
	if err != nil {
		return nil, err
	}
	return []uint32{encJType(off, 0)}, nil
}

// encFence encodes "fence" (same as "fence iorw, iorw") or "fence pred, succ",
// where pred and succ are non-empty subsets of "iorw" in that order.
func encFence(s statement, _ map[string]uint32) ([]uint32, error) {
	switch len(s.args) {
	case 0:
		return []uint32{0x0ff0000f}, nil
	case 2:
		pred, err := fenceSet(s.args[0])
		if err != nil {
			return nil, err
		}
		succ, err := fenceSet(s.args[1])
		if err != nil {
			return nil, err
		}
		return []uint32{pred<<24 | succ<<20 | opMiscMem}, nil
	}
	return nil, fmt.Errorf("fence: want 0 or 2 operands, got %d", len(s.args))
}

func fenceSet(arg string) (uint32, error) {
	var bits uint32
	rest := arg
	for i, c := range "iorw" {
		if r, ok := strings.CutPrefix(rest, string(c)); ok {
			bits |= 1 << (3 - i)
			rest = r
		}
	}
	if bits == 0 || rest != "" {
		return 0, fmt.Errorf("fence: invalid operand %q, want a subset of iorw", arg)
	}
	return bits, nil
}
