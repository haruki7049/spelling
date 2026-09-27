package machine

// edit applies one character to the built-in line editor.
func (m *Machine) edit(c byte) {
	switch c {
	case Enter, '\r':
		m.submitLine()
	case CtrlA:
		m.cursor = 0
	case CtrlE:
		m.cursor = len(m.line)
	case CtrlB:
		m.cursor = max(m.cursor-1, 0)
	case CtrlF:
		m.cursor = min(m.cursor+1, len(m.line))
	case Backspace, Delete:
		if m.cursor > 0 {
			m.cut(m.cursor-1, m.cursor)
		}
	case CtrlD:
		if m.cursor < len(m.line) {
			m.cut(m.cursor, m.cursor+1)
		}
	case CtrlU:
		m.cut(0, m.cursor)
	case CtrlW:
		start := m.cursor
		for start > 0 && m.line[start-1] == ' ' {
			start--
		}
		for start > 0 && m.line[start-1] != ' ' {
			start--
		}
		m.cut(start, m.cursor)
	default:
		if c < 0x20 || c >= 0x7f {
			return // other control characters are ignored
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
