package game

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/haruki7049/spelling/internal/machine"
	"github.com/haruki7049/spelling/internal/match"
	"github.com/haruki7049/spelling/internal/world"
)

// scale converts world units to screen pixels. The world fills the window.
const scale = float32(WindowWidth) / float32(world.Width/world.One)

// Debug font metrics used to place text and the cursor.
const (
	glyphWidth  = 6
	glyphHeight = 16
)

var (
	backgroundColor = color.RGBA{0x1b, 0x1e, 0x2b, 0xff}
	playerColors    = [2]color.RGBA{{0x6c, 0xb6, 0xff, 0xff}, {0xff, 0x7a, 0x7a, 0xff}}
	cursorColor     = color.RGBA{0xff, 0xff, 0xff, 0xc0}
)

// examples are the example spells shown in the help and inserted by the
// practice example keys.
var examples = [3]struct{ spell, note string }{
	{"lui t0, 0x10002; li t1, 0x80000; sw t1, 8(t0)", "run right (vx = 8.0)"},
	{"lui t0, 0x10002; li t1, -0x80000; sw t1, 8(t0)", "run left"},
	{"lui t0, 0x10002; li t1, 0xc0000; sw t1, 12(t0)", "jump (vy = 12.0)"},
}

// MatchScene plays a match. The keyboard drives player 0.
type MatchScene struct {
	match *match.Match
	keys  *KeyBindings
	// practice enables actions that skip typing (history, examples), which
	// are not allowed in real matches.
	practice bool
	history  history
	help     string
}

// NewMatchScene returns a scene playing m with the given key bindings.
// practice enables practice-only actions.
func NewMatchScene(m *match.Match, keys *KeyBindings, practice bool) *MatchScene {
	s := &MatchScene{match: m, keys: keys, practice: practice}
	s.help = s.helpText()
	return s
}

func (s *MatchScene) Update() (Scene, error) {
	me := s.match.Machines[0]
	if me.HasLanguage() {
		for _, c := range terminalInput() {
			me.Type(c)
		}
	} else {
		mods := heldModifiers()
		for _, c := range printableChars(mods) {
			me.Type(c)
		}
		for _, a := range s.keys.triggered(mods) {
			s.do(me, a)
		}
	}
	s.match.Step()
	return nil, nil
}

// do performs a line editor action.
func (s *MatchScene) do(me *machine.Machine, a action) {
	if e, ok := editActions[a]; ok {
		if e == machine.EditSubmit && s.practice {
			line, _ := me.Line()
			s.history.add(line)
		}
		me.Edit(e)
		return
	}
	if !s.practice {
		return
	}
	line, _ := me.Line()
	switch a {
	case actHistoryPrev:
		if prev, ok := s.history.prev(line); ok {
			me.SetLine(prev)
		}
	case actHistoryNext:
		if next, ok := s.history.next(); ok {
			me.SetLine(next)
		}
	case actExample1, actExample2, actExample3:
		me.SetLine(examples[a-actExample1].spell)
	}
}

// helpText describes the examples and the keys actually bound in the
// player's key binding file.
func (s *MatchScene) helpText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Type Idea and press %s. Examples:\n", s.keyNames(actSubmit))
	for i, e := range examples {
		key := ""
		if s.practice {
			key = "[" + s.keyNames(actExample1+action(i)) + "] "
		}
		fmt.Fprintf(&b, "  %s%-48s # %s\n", key, e.spell, e.note)
	}
	fmt.Fprintf(&b, "Keys: start %s, end %s, left %s, right %s, kill line %s, kill word %s",
		s.keyNames(actLineStart), s.keyNames(actLineEnd), s.keyNames(actCharLeft),
		s.keyNames(actCharRight), s.keyNames(actKillToStart), s.keyNames(actKillWordBackward))
	if s.practice {
		fmt.Fprintf(&b, "\nPractice: history %s / %s", s.keyNames(actHistoryPrev), s.keyNames(actHistoryNext))
	}
	return b.String()
}

func (s *MatchScene) keyNames(a action) string {
	keys := s.keys.keysFor(a)
	if len(keys) == 0 {
		return "(unbound)"
	}
	return strings.Join(keys, "/")
}

func (s *MatchScene) Draw(screen *ebiten.Image) {
	screen.Fill(backgroundColor)
	for i, b := range s.match.World.Bodies {
		x, y := toScreen(b.X, b.Y+world.BodyHeight)
		vector.FillRect(screen, x, y, toPixels(world.BodyWidth), toPixels(world.BodyHeight), playerColors[i], false)
	}

	me, opp := s.match.World.Bodies[0], s.match.World.Bodies[1]
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("You  HP %d  x %.1f y %.1f  vx %.2f vy %.2f",
		me.HP, fixed(me.X), fixed(me.Y), fixed(me.VX), fixed(me.VY)), 8, 8)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Opponent  HP %d", opp.HP), 8, 24)
	ebitenutil.DebugPrintAt(screen, s.help, 8, 48)

	line, cursor := s.match.Machines[0].Line()
	const prompt = "> "
	x, y := 8, WindowHeight-24
	ebitenutil.DebugPrintAt(screen, prompt+line, x, y)
	cx := float32(x + (len(prompt)+cursor)*glyphWidth)
	vector.FillRect(screen, cx, float32(y+2), 1, glyphHeight-2, cursorColor, false)
}

// toScreen converts a world point to screen coordinates (y flipped).
func toScreen(x, y int32) (float32, float32) {
	return toPixels(x), float32(WindowHeight) - toPixels(y)
}

func toPixels(v int32) float32 {
	return float32(v) / world.One * scale
}

func fixed(v int32) float64 {
	return float64(v) / world.One
}
