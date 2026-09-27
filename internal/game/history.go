package game

// maxHistory is the number of submitted lines kept for practice history.
const maxHistory = 100

// history recalls submitted lines with up/down, like a shell. It is a
// practice feature and lives outside the machine, which never sees it.
type history struct {
	lines []string
	pos   int    // index into lines; len(lines) means the draft
	draft string // the line being typed before browsing started
}

// add records a submitted line and stops browsing.
func (h *history) add(line string) {
	if line != "" && (len(h.lines) == 0 || h.lines[len(h.lines)-1] != line) {
		h.lines = append(h.lines, line)
		if len(h.lines) > maxHistory {
			h.lines = h.lines[1:]
		}
	}
	h.pos = len(h.lines)
	h.draft = ""
}

// prev returns the previous line given the current editor text, and
// whether it changed.
func (h *history) prev(current string) (string, bool) {
	if h.pos == 0 {
		return "", false
	}
	if h.pos == len(h.lines) {
		h.draft = current
	}
	h.pos--
	return h.lines[h.pos], true
}

// next returns the next line, ending with the saved draft.
func (h *history) next() (string, bool) {
	if h.pos >= len(h.lines) {
		return "", false
	}
	h.pos++
	if h.pos == len(h.lines) {
		return h.draft, true
	}
	return h.lines[h.pos], true
}
