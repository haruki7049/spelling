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
	overlayColor    = color.RGBA{0x00, 0x00, 0x00, 0xa0}
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
	match    *match.Match
	newMatch func() (*match.Match, error)
	keys     *KeyBindings
	// practice enables actions that skip typing (history, examples), which
	// are not allowed in real matches.
	practice bool
	history  history
	help     string
}

// NewMatchScene starts a match made by newMatch, which is called again for
// each rematch. practice enables practice-only actions.
func NewMatchScene(newMatch func() (*match.Match, error), keys *KeyBindings, practice bool) (*MatchScene, error) {
	m, err := newMatch()
	if err != nil {
		return nil, err
	}
	s := &MatchScene{match: m, newMatch: newMatch, keys: keys, practice: practice}
	s.help = s.helpText()
	return s, nil
}

func (s *MatchScene) Update() (Scene, error) {
	if s.match.Result() != match.Ongoing {
		// The match is decided: input only starts a rematch.
		for _, a := range s.keys.triggered(heldModifiers()) {
			if a == actRematch {
				m, err := s.newMatch()
				if err != nil {
					return nil, err
				}
				s.match = m
			}
		}
		return nil, nil
	}
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
	for _, m := range s.keys.Missing() {
		fmt.Fprintf(&b, "\nWarning: %s", m)
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
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("You       HP %d  %s  x %.1f y %.1f  vx %.2f vy %.2f",
		me.HP, manaText(me, s.match.ManaSpentPerSecond(0), s.match.InfiniteMana), fixed(me.X), fixed(me.Y), fixed(me.VX), fixed(me.VY)), 8, 8)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("Opponent  HP %d  %s", opp.HP, manaText(opp, s.match.ManaSpentPerSecond(1), s.match.InfiniteMana)), 8, 24)
	ebitenutil.DebugPrintAt(screen, s.help, 8, 48)

	line, cursor := s.match.Machines[0].Line()
	const prompt = "> "
	x, y := 8, WindowHeight-24
	ebitenutil.DebugPrintAt(screen, prompt+line, x, y)
	cx := float32(x + (len(prompt)+cursor)*glyphWidth)
	vector.FillRect(screen, cx, float32(y+2), 1, glyphHeight-2, cursorColor, false)

	ebitenutil.DebugPrintAt(screen, "Time "+timeText(s.match.Tick), WindowWidth-80, 8)

	if r := s.match.Result(); r != match.Ongoing {
		vector.FillRect(screen, 0, 0, WindowWidth, WindowHeight, overlayColor, false)
		msg := resultText(r) + "\n\nPress " + s.keyNames(actRematch) + " for a rematch"
		ebitenutil.DebugPrintAt(screen, msg, WindowWidth/2-80, WindowHeight/2-16)
	}
}

// resultText describes a finished match from player 0's point of view.
func resultText(r match.Result) string {
	switch r {
	case match.Player0Wins:
		return "YOU WIN"
	case match.Player1Wins:
		return "YOU LOSE"
	}
	return "DRAW"
}

// timeText shows the time left before the time limit as m:ss, rounding up.
func timeText(tick uint32) string {
	left := max(int64(match.TimeLimitTicks)-int64(tick), 0)
	secs := (left + match.TicksPerSecond - 1) / match.TicksPerSecond
	return fmt.Sprintf("%d:%02d", secs/60, secs%60)
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

// manaText shows a body's mana and spent rate, or that it is depleted or infinite.
func manaText(b world.Body, spentPerSec int32, infinite bool) string {
	if infinite {
		return fmt.Sprintf("mana INF (spent %d/s)", spentPerSec)
	}
	if b.Depleted {
		return "mana DEPLETED"
	}
	return fmt.Sprintf("mana %d/%d (spent %d/s)", b.Mana, match.MaxMana, spentPerSec)
}
