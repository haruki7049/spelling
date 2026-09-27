// Package vm implements the virtual CPU that runs Idea, the in-game machine
// code. Idea is the RISC-V RV32I base integer instruction set.
//
// Game-specific behavior (see issue #15):
//   - An illegal instruction halts the CPU and restarts it: PC is set to the
//     entry point and registers are cleared, while memory is kept.
//   - ECALL, EBREAK, and FENCE are no-ops.
//   - WFI (privileged spec) sets Waiting; the owner ends the tick early.
//   - Misaligned loads, stores, and instruction fetches are allowed.
//   - Unmapped accesses are handled by the Bus (RAM reads 0 and ignores writes).
package vm

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

// instWFI is WFI (wait for interrupt) from the RISC-V privileged spec, the
// only instruction supported outside RV32I.
const instWFI = 0x10500073

// CPU is a single RV32I hart.
type CPU struct {
	// Regs holds x0-x31. x0 always reads as zero.
	Regs [32]uint32
	PC   uint32
	// Entry is the address the CPU restarts from after an illegal instruction.
	Entry uint32
	Bus   Bus
	// Waiting is set by WFI. The owner of the CPU decides how long to wait
	// and clears it.
	Waiting bool
}

// NewCPU returns a CPU that starts executing at entry.
func NewCPU(bus Bus, entry uint32) *CPU {
	return &CPU{PC: entry, Entry: entry, Bus: bus}
}

// Restart sets PC to the entry point and clears all registers.
// Memory is left untouched.
func (c *CPU) Restart() {
	c.Regs = [32]uint32{}
	c.PC = c.Entry
}

// Step executes one instruction. It returns false if the instruction was
// illegal, in which case the CPU has been restarted.
func (c *CPU) Step() bool {
	inst := c.Bus.Read(c.PC, 4)
	if !c.execute(inst) {
		c.Restart()
		return false
	}
	c.Regs[0] = 0
	return true
}

func (c *CPU) execute(inst uint32) bool {
	opcode := inst & 0x7f
	rd := (inst >> 7) & 0x1f
	funct3 := (inst >> 12) & 0x7
	rs1 := (inst >> 15) & 0x1f
	rs2 := (inst >> 20) & 0x1f
	funct7 := inst >> 25
	a, b := c.Regs[rs1], c.Regs[rs2]
	next := c.PC + 4

	switch opcode {
	case opLUI:
		c.Regs[rd] = inst & 0xfffff000
	case opAUIPC:
		c.Regs[rd] = c.PC + inst&0xfffff000
	case opJAL:
		c.Regs[rd] = next
		next = c.PC + immJ(inst)
	case opJALR:
		if funct3 != 0 {
			return false
		}
		target := (a + immI(inst)) &^ 1
		c.Regs[rd] = next
		next = target
	case opBranch:
		var taken bool
		switch funct3 {
		case 0b000:
			taken = a == b
		case 0b001:
			taken = a != b
		case 0b100:
			taken = int32(a) < int32(b)
		case 0b101:
			taken = int32(a) >= int32(b)
		case 0b110:
			taken = a < b
		case 0b111:
			taken = a >= b
		default:
			return false
		}
		if taken {
			next = c.PC + immB(inst)
		}
	case opLoad:
		addr := a + immI(inst)
		switch funct3 {
		case 0b000:
			c.Regs[rd] = uint32(int8(c.Bus.Read(addr, 1)))
		case 0b001:
			c.Regs[rd] = uint32(int16(c.Bus.Read(addr, 2)))
		case 0b010:
			c.Regs[rd] = c.Bus.Read(addr, 4)
		case 0b100:
			c.Regs[rd] = c.Bus.Read(addr, 1)
		case 0b101:
			c.Regs[rd] = c.Bus.Read(addr, 2)
		default:
			return false
		}
	case opStore:
		addr := a + immS(inst)
		switch funct3 {
		case 0b000:
			c.Bus.Write(addr, 1, b)
		case 0b001:
			c.Bus.Write(addr, 2, b)
		case 0b010:
			c.Bus.Write(addr, 4, b)
		default:
			return false
		}
	case opImm:
		imm := immI(inst)
		shamt := rs2
		switch funct3 {
		case 0b000:
			c.Regs[rd] = a + imm
		case 0b010:
			c.Regs[rd] = boolToU32(int32(a) < int32(imm))
		case 0b011:
			c.Regs[rd] = boolToU32(a < imm)
		case 0b100:
			c.Regs[rd] = a ^ imm
		case 0b110:
			c.Regs[rd] = a | imm
		case 0b111:
			c.Regs[rd] = a & imm
		case 0b001:
			if funct7 != 0 {
				return false
			}
			c.Regs[rd] = a << shamt
		case 0b101:
			switch funct7 {
			case 0b0000000:
				c.Regs[rd] = a >> shamt
			case 0b0100000:
				c.Regs[rd] = uint32(int32(a) >> shamt)
			default:
				return false
			}
		}
	case opReg:
		shamt := b & 0x1f
		switch {
		case funct7 == 0 && funct3 == 0b000:
			c.Regs[rd] = a + b
		case funct7 == 0b0100000 && funct3 == 0b000:
			c.Regs[rd] = a - b
		case funct7 == 0 && funct3 == 0b001:
			c.Regs[rd] = a << shamt
		case funct7 == 0 && funct3 == 0b010:
			c.Regs[rd] = boolToU32(int32(a) < int32(b))
		case funct7 == 0 && funct3 == 0b011:
			c.Regs[rd] = boolToU32(a < b)
		case funct7 == 0 && funct3 == 0b100:
			c.Regs[rd] = a ^ b
		case funct7 == 0 && funct3 == 0b101:
			c.Regs[rd] = a >> shamt
		case funct7 == 0b0100000 && funct3 == 0b101:
			c.Regs[rd] = uint32(int32(a) >> shamt)
		case funct7 == 0 && funct3 == 0b110:
			c.Regs[rd] = a | b
		case funct7 == 0 && funct3 == 0b111:
			c.Regs[rd] = a & b
		default:
			return false
		}
	case opMiscMem:
		// FENCE is a no-op: there is a single hart and no caches.
		if funct3 != 0b000 {
			return false
		}
	case opSystem:
		if inst == instWFI {
			c.Waiting = true
			break
		}
		// ECALL (imm 0) and EBREAK (imm 1) are no-ops.
		if inst&^(1<<20) != opSystem {
			return false
		}
	default:
		return false
	}

	c.PC = next
	return true
}

func immI(inst uint32) uint32 {
	return uint32(int32(inst) >> 20)
}

func immS(inst uint32) uint32 {
	return uint32(int32(inst)>>20)&^0x1f | (inst>>7)&0x1f
}

func immB(inst uint32) uint32 {
	return uint32(int32(inst)>>19)&^0xfff | // imm[12] (sign) at bit 12
		(inst<<4)&0x800 | // imm[11] from bit 7
		(inst>>20)&0x7e0 | // imm[10:5] from bits 30:25
		(inst>>7)&0x1e // imm[4:1] from bits 11:8
}

func immJ(inst uint32) uint32 {
	return uint32(int32(inst)>>11)&^0xfffff | // imm[20] (sign) at bit 20
		inst&0xff000 | // imm[19:12]
		(inst>>9)&0x800 | // imm[11] from bit 20
		(inst>>20)&0x7fe // imm[10:1] from bits 30:21
}

func boolToU32(v bool) uint32 {
	if v {
		return 1
	}
	return 0
}
