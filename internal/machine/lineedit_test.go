package machine

import "testing"

func TestLineEditorKeys(t *testing.T) {
	tests := []struct {
		name       string
		keys       string
		wantLine   string
		wantCursor int
	}{
		{"insert", "li a0", "li a0", 5},
		{"backspace", "li a0X\b", "li a0", 5},
		{"delete key acts as backspace", "abc\x7f", "ab", 2},
		{"move and insert", "sw a0\x01X", "Xsw a0", 1},
		{"end", "abc\x01\x05d", "abcd", 4},
		{"left right", "ac\x02b\x06d", "abcd", 4},
		{"left stops at start", "a\x02\x02\x02", "a", 0},
		{"right stops at end", "a\x06\x06", "a", 1},
		{"ctrl-d deletes forward", "abc\x01\x04", "bc", 0},
		{"ctrl-d at end does nothing", "abc\x04", "abc", 3},
		{"ctrl-u kills to start", "li a0, 1\x02\x15", "1", 0},
		{"ctrl-w kills word", "li a0, 1234\x17", "li a0, ", 7},
		{"ctrl-w skips trailing spaces", "li a0   \x17", "li ", 3},
		{"ctrl-w in the middle", "aa bb cc\x02\x02\x02\x17", "aa  cc", 3},
		{"other control characters ignored", "a\x07\x1bb", "ab", 2},
	}
	for _, tt := range tests {
		m := New(0x100, 0)
		typeString(m, tt.keys)
		line, cursor := m.Line()
		if line != tt.wantLine || cursor != tt.wantCursor {
			t.Errorf("%s: got %q cursor %d, want %q cursor %d", tt.name, line, cursor, tt.wantLine, tt.wantCursor)
		}
	}
}

func TestEditedLineRuns(t *testing.T) {
	// Fix a typo in the middle with the cursor, then run the line.
	m := newMachine(t, counter)
	typeString(m, "li a0, 7; sx a0, 0x600(zero)")
	typeString(m, "\x01\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06\x06") // after "li a0, 7; s"
	typeString(m, "\x04w\n")                                          // replace x with w
	m.Run(30)
	if got := m.RAM.Read(0x600, 4); got != 7 {
		t.Errorf("stored %d, want 7", got)
	}
}

func TestSetLine(t *testing.T) {
	m := New(0x100, 0)
	m.SetLine("li a0, 1\x01\x00é")
	if line, cursor := m.Line(); line != "li a0, 1" || cursor != 8 {
		t.Errorf("got %q cursor %d", line, cursor)
	}
	long := make([]byte, KeyBufferSize+10)
	for i := range long {
		long[i] = 'a'
	}
	m.SetLine(string(long))
	if line, _ := m.Line(); len(line) != KeyBufferSize {
		t.Errorf("length %d, want %d", len(line), KeyBufferSize)
	}
}
