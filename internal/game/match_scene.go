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

var (
	backgroundColor = color.RGBA{0x1b, 0x1e, 0x2b, 0xff}
	playerColors    = [2]color.RGBA{{0x6c, 0xb6, 0xff, 0xff}, {0xff, 0x7a, 0x7a, 0xff}}
)

const help = `Type Idea and press Enter. Examples:
  lui t0, 0x10002; li t1, 0x80000; sw t1, 8(t0)     # run right (vx = 8.0)
  lui t0, 0x10002; li t1, -0x80000; sw t1, 8(t0)    # run left
  lui t0, 0x10002; li t1, 0xc0000; sw t1, 12(t0)    # jump (vy = 12.0)`

// MatchScene plays a match. The keyboard drives player 0.
type MatchScene struct {
	match *match.Match
}

// NewMatchScene returns a scene playing m.
func NewMatchScene(m *match.Match) *MatchScene {
	return &MatchScene{match: m}
}

func (s *MatchScene) Update() (Scene, error) {
	for _, c := range typedChars() {
		s.match.Machines[0].Type(c)
	}
	s.match.Step()
	return nil, nil
}

// typedChars returns the characters typed since the last update, as the
// bytes the machine expects. Non-ASCII input is ignored.
func typedChars() []byte {
	var out []byte
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 0x20 && r < 0x7f {
			out = append(out, byte(r))
		}
	}
	if repeated(ebiten.KeyBackspace) {
		out = append(out, machine.Backspace)
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
	ebitenutil.DebugPrintAt(screen, "> "+s.match.Machines[0].Line()+"_", 8, WindowHeight-24)
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
