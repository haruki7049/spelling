package asm

import (
	"os"
	"strings"
	"testing"
)

// TestSpecExamples assembles every asm code block in docs/spec.md, so the
// examples in the specification stay valid.
func TestSpecExamples(t *testing.T) {
	data, err := os.ReadFile("../../docs/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	blocks := 0
	rest := string(data)
	for {
		_, after, ok := strings.Cut(rest, "```asm\n")
		if !ok {
			break
		}
		block, next, ok := strings.Cut(after, "```")
		if !ok {
			t.Fatal("unterminated asm block")
		}
		rest = next
		blocks++
		if _, err := Assemble(block, 0); err != nil {
			t.Errorf("block %d does not assemble: %v\n%s", blocks, err, block)
		}
	}
	if blocks == 0 {
		t.Error("found no asm blocks")
	}
}
