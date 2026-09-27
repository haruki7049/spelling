// Package asm implements the built-in assembler that turns typed text into
// Idea (RISC-V RV32I) machine code.
//
// Syntax (see issue #15) follows GNU as for RISC-V:
//   - One instruction per statement. Statements are separated by newlines
//     or ';'. '#' starts a comment that runs to the end of the line.
//   - A statement may start with one or more labels ("loop:").
//   - Mnemonics are case-insensitive. Registers are lowercase numeric
//     (x0-x31) or ABI names (zero, ra, sp, a0, ...).
//   - Integers are decimal, 0x hex, 0b binary, or 0-prefixed octal,
//     optionally negative.
//   - Loads, stores, and jalr take "offset(reg)" memory operands.
//   - Branch and jump targets are a label or an integer byte offset from
//     the instruction. la and call take a label only.
//   - fence takes no operands or "pred, succ" (subsets of iorw).
//
// Anything accepted here is also accepted by LLVM's RISC-V assembler with
// the same meaning, so typed code can be moved to an external toolchain.
//   - wfi (from the RISC-V privileged spec) is also supported.
//   - Pseudo-instructions: nop, li, la, mv, j, call, ret, beqz, bnez.
//
// Directives (.section, .word, ...) and relocations are not supported;
// whole programs are built with an external toolchain into ELF.
package asm

import (
	"fmt"
	"strconv"
	"strings"
)

// Error reports a problem in one source line.
type Error struct {
	Line int // 1-based line number in the source
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

type statement struct {
	line     int
	mnemonic string
	args     []string
	addr     uint32
}

// Assemble assembles src into machine code that will be placed at address
// base. base is used to resolve labels for la and call.
func Assemble(src string, base uint32) ([]uint32, error) {
	var stmts []statement
	labels := map[string]uint32{}
	addr := base

	for i, line := range strings.Split(src, "\n") {
		lineNo := i + 1
		if j := strings.IndexByte(line, '#'); j >= 0 {
			line = line[:j]
		}
		for text := range strings.SplitSeq(line, ";") {
			text = strings.TrimSpace(text)
			for {
				j := strings.IndexByte(text, ':')
				if j < 0 {
					break
				}
				name := strings.TrimSpace(text[:j])
				if !isIdent(name) {
					return nil, &Error{lineNo, fmt.Sprintf("invalid label %q", name)}
				}
				if _, dup := labels[name]; dup {
					return nil, &Error{lineNo, fmt.Sprintf("duplicate label %q", name)}
				}
				labels[name] = addr
				text = strings.TrimSpace(text[j+1:])
			}
			if text == "" {
				continue
			}
			s := statement{line: lineNo, addr: addr}
			s.mnemonic, s.args = splitStatement(text)
			n, err := size(s)
			if err != nil {
				return nil, &Error{lineNo, err.Error()}
			}
			stmts = append(stmts, s)
			addr += 4 * n
		}
	}

	var code []uint32
	for _, s := range stmts {
		words, err := encode(s, labels)
		if err != nil {
			return nil, &Error{s.line, err.Error()}
		}
		code = append(code, words...)
	}
	return code, nil
}

func splitStatement(text string) (string, []string) {
	mnemonic, rest, _ := strings.Cut(text, " ")
	if m, r, ok := strings.Cut(mnemonic, "\t"); ok {
		mnemonic, rest = m, r+" "+rest
	}
	rest = strings.TrimSpace(rest)
	var args []string
	if rest != "" {
		for a := range strings.SplitSeq(rest, ",") {
			args = append(args, strings.TrimSpace(a))
		}
	}
	return strings.ToLower(mnemonic), args
}

// size returns the number of instruction words a statement expands to.
func size(s statement) (uint32, error) {
	switch s.mnemonic {
	case "li":
		if len(s.args) != 2 {
			return 0, fmt.Errorf("li: want 2 operands, got %d", len(s.args))
		}
		v, err := parseImm(s.args[1], -1<<31, 1<<32-1)
		if err != nil {
			return 0, err
		}
		if _, lo := splitHiLo(uint32(v)); fitsSigned(int64(int32(v)), 12) || lo == 0 {
			return 1, nil
		}
		return 2, nil
	case "la", "call":
		return 2, nil
	}
	if _, ok := encoders[s.mnemonic]; !ok {
		return 0, fmt.Errorf("unknown instruction %q", s.mnemonic)
	}
	return 1, nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_' || r == '.' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}

var abiNames = map[string]uint32{
	"zero": 0, "ra": 1, "sp": 2, "gp": 3, "tp": 4, "t0": 5, "t1": 6, "t2": 7,
	"s0": 8, "fp": 8, "s1": 9, "a0": 10, "a1": 11, "a2": 12, "a3": 13, "a4": 14,
	"a5": 15, "a6": 16, "a7": 17, "s2": 18, "s3": 19, "s4": 20, "s5": 21,
	"s6": 22, "s7": 23, "s8": 24, "s9": 25, "s10": 26, "s11": 27, "t3": 28,
	"t4": 29, "t5": 30, "t6": 31,
}

func parseReg(s string) (uint32, error) {
	if r, ok := abiNames[s]; ok {
		return r, nil
	}
	if n, ok := strings.CutPrefix(s, "x"); ok {
		if r, err := strconv.ParseUint(n, 10, 8); err == nil && r < 32 {
			return uint32(r), nil
		}
	}
	return 0, fmt.Errorf("invalid register %q", s)
}

// parseImm parses an integer in [lo, hi].
func parseImm(s string, lo, hi int64) (int64, error) {
	// strconv also accepts '_' separators and 0o octal, which LLVM rejects.
	digits := strings.TrimPrefix(s, "-")
	if strings.Contains(s, "_") || strings.HasPrefix(digits, "0o") || strings.HasPrefix(digits, "0O") {
		return 0, fmt.Errorf("invalid integer %q", s)
	}
	v, err := strconv.ParseInt(s, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid integer %q", s)
	}
	if v < lo || v > hi {
		return 0, fmt.Errorf("integer %s out of range [%d, %d]", s, lo, hi)
	}
	return v, nil
}

func fitsSigned(v int64, bits uint) bool {
	return v >= -1<<(bits-1) && v < 1<<(bits-1)
}

// parseMem parses a memory operand "offset(reg)" where offset is optional.
func parseMem(s string) (int64, uint32, error) {
	open := strings.IndexByte(s, '(')
	if open < 0 || !strings.HasSuffix(s, ")") {
		return 0, 0, fmt.Errorf("invalid memory operand %q, want offset(reg)", s)
	}
	rs1, err := parseReg(strings.TrimSpace(s[open+1 : len(s)-1]))
	if err != nil {
		return 0, 0, err
	}
	var off int64
	if o := strings.TrimSpace(s[:open]); o != "" {
		if off, err = parseImm(o, -2048, 2047); err != nil {
			return 0, 0, err
		}
	}
	return off, rs1, nil
}

// target resolves a branch or jump target to a byte offset from pc.
func target(s string, pc uint32, labels map[string]uint32) (int64, error) {
	if isIdent(s) {
		addr, ok := labels[s]
		if !ok {
			return 0, fmt.Errorf("undefined label %q", s)
		}
		return int64(int32(addr - pc)), nil
	}
	return parseImm(s, -1<<31, 1<<31-1)
}

// splitHiLo splits v into a 20-bit upper part and a sign-extended 12-bit
// lower part such that hi<<12 + lo == v (mod 2^32).
func splitHiLo(v uint32) (hi uint32, lo int64) {
	lo = int64(int32(v<<20) >> 20)
	hi = (v - uint32(lo)) >> 12
	return hi, lo
}
