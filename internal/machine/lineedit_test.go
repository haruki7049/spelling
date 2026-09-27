package machine

import "testing"

// step is one input to the line editor: text to type, or an action.
type step struct {
	text   string
	action EditAction
}

func typed(s string) step   { return step{text: s, action: -1} }
func act(a EditAction) step { return step{action: a} }
func acts(a EditAction, n int) []step {
	out := make([]step, n)
	for i := range out {
		out[i] = act(a)
	}
	return out
}

func apply(m *Machine, steps ...step) {
	for _, s := range steps {
		if s.action < 0 {
			for i := range len(s.text) {
				m.Type(s.text[i])
			}
		} else {
			m.Edit(s.action)
		}
	}
}

func TestLineEditorActions(t *testing.T) {
	tests := []struct {
		name       string
		steps      []step
		wantLine   string
		wantCursor int
	}{
		{"insert", []step{typed("li a0")}, "li a0", 5},
		{"delete backward", []step{typed("li a0X"), act(EditDeleteBackward)}, "li a0", 5},
		{"line start and insert", []step{typed("sw a0"), act(EditLineStart), typed("X")}, "Xsw a0", 1},
		{"line end", []step{typed("abc"), act(EditLineStart), act(EditLineEnd), typed("d")}, "abcd", 4},
		{"left and right", []step{typed("ac"), act(EditCharLeft), typed("b"), act(EditCharRight), typed("d")}, "abcd", 4},
		{"left stops at start", append([]step{typed("a")}, acts(EditCharLeft, 3)...), "a", 0},
		{"right stops at end", append([]step{typed("a")}, acts(EditCharRight, 2)...), "a", 1},
		{"delete forward", []step{typed("abc"), act(EditLineStart), act(EditDeleteForward)}, "bc", 0},
		{"delete forward at end", []step{typed("abc"), act(EditDeleteForward)}, "abc", 3},
		{"kill to start", []step{typed("li a0, 1"), act(EditCharLeft), act(EditKillToStart)}, "1", 0},
		{"kill word", []step{typed("li a0, 1234"), act(EditKillWordBackward)}, "li a0, ", 7},
		{"kill word skips spaces", []step{typed("li a0   "), act(EditKillWordBackward)}, "li ", 3},
		{"kill word in the middle", append(append([]step{typed("aa bb cc")}, acts(EditCharLeft, 3)...), act(EditKillWordBackward)), "aa  cc", 3},
		{"control characters are not inserted", []step{typed("a\x01\x08\x1bb")}, "ab", 2},
	}
	for _, tt := range tests {
		m := New(0x100, 0)
		apply(m, tt.steps...)
		line, cursor := m.Line()
		if line != tt.wantLine || cursor != tt.wantCursor {
			t.Errorf("%s: got %q cursor %d, want %q cursor %d", tt.name, line, cursor, tt.wantLine, tt.wantCursor)
		}
	}
}

func TestEditedLineRuns(t *testing.T) {
	// Fix a typo in the middle with the cursor, then run the line.
	m := newMachine(t, counter)
	apply(m, typed("li a0, 7; sx a0, 0x600(zero)"), act(EditLineStart))
	apply(m, acts(EditCharRight, len("li a0, 7; s"))...)
	apply(m, act(EditDeleteForward), typed("w"), act(EditSubmit))
	m.Run(30)
	if got := m.RAM.Read(0x600, 4); got != 7 {
		t.Errorf("stored %d, want 7", got)
	}
}

func TestEditIgnoredWithLanguage(t *testing.T) {
	m := New(0x100, 0)
	m.keyHandler = 0x40
	m.Type('a')
	m.Edit(EditSubmit)
	if line, _ := m.Line(); line != "" || len(m.keys) != 1 {
		t.Errorf("line %q, keys %q: with a language, input goes to the buffer only", line, m.keys)
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
