package game

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/haruki7049/spelling/internal/machine"
	"github.com/haruki7049/spelling/internal/match"
	"github.com/haruki7049/spelling/internal/world"
)

// scale converts world units to screen pixels. The world fills the window.
const scale = float32(WindowWidth) / float32(world.Width/world.One)

// Debug font metrics used to place the cursor.
const (
	glyphWidth  = 6
	glyphHeight = 16
)

var (
	backgroundColor = color.RGBA{0x1b, 0x1e, 0x2b, 0xff}
	playerColors    = [2]color.RGBA{{0x6c, 0xb6, 0xff, 0xff}, {0xff, 0x7a, 0x7a, 0xff}}
	cursorColor     = color.RGBA{0xff, 0xff, 0xff, 0xc0}
)

const help = `Type Idea and press Enter. Examples:
  lui t0, 0x10002; li t1, 0x80000; sw t1, 8(t0)     # run right (vx = 8.0)
  lui t0, 0x10002; li t1, -0x80000; sw t1, 8(t0)    # run left
  lui t0, 0x10002; li t1, 0xc0000; sw t1, 12(t0)    # jump (vy = 12.0)
Editing: Ctrl+A/E start/end, Left/Right, Ctrl+U/W kill, Ctrl+D delete`

const practiceHelp = "Practice: Up/Down history"

// ctrlKeys maps Ctrl+letter to the control character sent to the machine.
var ctrlKeys = map[ebiten.Key]byte{
	ebiten.KeyA: machine.CtrlA,
	ebiten.KeyB: machine.CtrlB,
	ebiten.KeyD: machine.CtrlD,
	ebiten.KeyE: machine.CtrlE,
	ebiten.KeyF: machine.CtrlF,
	ebiten.KeyU: machine.CtrlU,
	ebiten.KeyW: machine.CtrlW,
}

// editKeys maps editing keys to control characters, with key repeat.
var editKeys = map[ebiten.Key]byte{
	ebiten.KeyBackspace:  machine.Backspace,
	ebiten.KeyDelete:     machine.CtrlD,
	ebiten.KeyArrowLeft:  machine.CtrlB,
	ebiten.KeyArrowRight: machine.CtrlF,
	ebiten.KeyHome:       machine.CtrlA,
	ebiten.KeyEnd:        machine.CtrlE,
}

// MatchScene plays a match. The keyboard drives player 0.
type MatchScene struct {
	match *match.Match
	// practice enables features that skip typing (history), which are not
	// allowed in real matches.
	practice bool
	history  history
}

// NewMatchScene returns a scene playing m. practice enables practice-only
// input features.
func NewMatchScene(m *match.Match, practice bool) *MatchScene {
	return &MatchScene{match: m, practice: practice}
}

func (s *MatchScene) Update() (Scene, error) {
	me := s.match.Machines[0]
	if s.practice {
		s.updateHistory(me)
	}
	for _, c := range typedChars() {
		if c == machine.Enter && s.practice {
			line, _ := me.Line()
			s.history.add(line)
		}
		me.Type(c)
	}
	s.match.Step()
	return nil, nil
}

func (s *MatchScene) updateHistory(me *machine.Machine) {
	line, _ := me.Line()
	var next string
	var ok bool
	switch {
	case repeated(ebiten.KeyArrowUp):
		next, ok = s.history.prev(line)
	case repeated(ebiten.KeyArrowDown):
		next, ok = s.history.next()
	}
	if ok {
		me.SetLine(next)
	}
}

// typedChars returns the characters typed since the last update, as the
// bytes the machine expects: printable ASCII plus the control characters
// of the editing keys. Non-ASCII input is ignored.
func typedChars() []byte {
	var out []byte
	if ebiten.IsKeyPressed(ebiten.KeyControl) {
		for k, c := range ctrlKeys {
			if repeated(k) {
				out = append(out, c)
			}
		}
	} else {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 0x20 && r < 0x7f {
				out = append(out, byte(r))
			}
		}
	}
	for k, c := range editKeys {
		if repeated(k) {
			out = append(out, c)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		out = append(out, machine.Enter)
	}
	return out
}

// repeated reports a key press with key repeat after a short delay.
func repeated(key ebiten.Key) bool {
	d := inpututil.KeyPressDuration(key)
	return d == 1 || d >= 30 && d%3 == 0
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
	ebitenutil.DebugPrintAt(screen, help, 8, 48)
	if s.practice {
		ebitenutil.DebugPrintAt(screen, practiceHelp, 8, 48+5*glyphHeight)
	}

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
