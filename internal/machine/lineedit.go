package machine

// EditAction is an editing operation of the built-in line editor. Which key
// triggers which action is not decided here; the game reads it from the
// player's key binding file.
type EditAction int

// Line editor actions.
const (
	EditSubmit           EditAction = iota // assemble and run the line
	EditLineStart                          // move the cursor to the start
	EditLineEnd                            // move the cursor to the end
	EditCharLeft                           // move the cursor left
	EditCharRight                          // move the cursor right
	EditDeleteBackward                     // delete the character before the cursor
	EditDeleteForward                      // delete the character at the cursor
	EditKillToStart                        // delete from the start to the cursor
	EditKillWordBackward                   // delete the word before the cursor
)

// Edit applies an editing action to the built-in line editor. It does
// nothing while a keyboard handler (a language implementation) is
// registered, because the language then owns all input.
func (m *Machine) Edit(a EditAction) {
	if m.keyHandler != 0 {
		return
	}
	switch a {
	case EditSubmit:
		m.submitLine()
	case EditLineStart:
		m.cursor = 0
	case EditLineEnd:
		m.cursor = len(m.line)
	case EditCharLeft:
		m.cursor = max(m.cursor-1, 0)
	case EditCharRight:
		m.cursor = min(m.cursor+1, len(m.line))
	case EditDeleteBackward:
		if m.cursor > 0 {
			m.cut(m.cursor-1, m.cursor)
		}
	case EditDeleteForward:
		if m.cursor < len(m.line) {
			m.cut(m.cursor, m.cursor+1)
		}
	case EditKillToStart:
		m.cut(0, m.cursor)
	case EditKillWordBackward:
		start := m.cursor
		for start > 0 && m.line[start-1] == ' ' {
			start--
		}
		for start > 0 && m.line[start-1] != ' ' {
			start--
		}
		m.cut(start, m.cursor)
	}
}

// insert inserts a printable character at the cursor. Other characters are
// ignored: the line editor has no built-in meaning for control characters.
func (m *Machine) insert(c byte) {
	if c < 0x20 || c >= 0x7f {
		return
	}
	if len(m.line) >= KeyBufferSize {
		m.keyOverflow = true
		return
	}
	m.line = append(m.line, 0)
	copy(m.line[m.cursor+1:], m.line[m.cursor:])
	m.line[m.cursor] = c
	m.cursor++
}

// cut removes line[from:to] and leaves the cursor at from.
func (m *Machine) cut(from, to int) {
	m.line = append(m.line[:from], m.line[to:]...)
	m.cursor = from
}

// Line returns the text in the line editor and the cursor position in it.
func (m *Machine) Line() (string, int) {
	return string(m.line), m.cursor
}

// SetLine replaces the line editor text and moves the cursor to its end.
// Characters the editor would not insert are dropped, and the text is cut
// to KeyBufferSize. It is meant for practice features such as history.
func (m *Machine) SetLine(s string) {
	m.line = m.line[:0]
	for i := range len(s) {
		if c := s[i]; c >= 0x20 && c < 0x7f && len(m.line) < KeyBufferSize {
			m.line = append(m.line, c)
		}
	}
	m.cursor = len(m.line)
}

// HasLanguage reports whether a keyboard handler is registered, in which
// case input should be sent with Type in terminal encoding.
func (m *Machine) HasLanguage() bool {
	return m.keyHandler != 0
}
