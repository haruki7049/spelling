package game

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/haruki7049/spelling/internal/match"
	"github.com/haruki7049/spelling/internal/world"
)

// stubScene returns next and err from Update and counts how often it is updated.
type stubScene struct {
	next    Scene
	err     error
	updates int
}

func (s *stubScene) Update() (Scene, error) {
	s.updates++
	return s.next, s.err
}

func (s *stubScene) Draw(screen *ebiten.Image) {}

func TestUpdateStaysOnSceneWhenNextIsNil(t *testing.T) {
	scene := &stubScene{}
	g := NewGame(scene)

	if err := g.Update(); err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}
	if g.scene != scene {
		t.Errorf("scene changed, want it to stay on the initial scene")
	}
	if scene.updates != 1 {
		t.Errorf("scene updated %d times, want 1", scene.updates)
	}
}

func TestUpdateSwitchesToNextScene(t *testing.T) {
	next := &stubScene{}
	g := NewGame(&stubScene{next: next})

	if err := g.Update(); err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}
	if g.scene != next {
		t.Errorf("scene did not switch to the next scene")
	}
}

func TestUpdateReturnsSceneError(t *testing.T) {
	want := errors.New("boom")
	scene := &stubScene{next: &stubScene{}, err: want}
	g := NewGame(scene)

	if err := g.Update(); !errors.Is(err, want) {
		t.Fatalf("Update() error = %v, want %v", err, want)
	}
	if g.scene != scene {
		t.Errorf("scene switched despite an error")
	}
}

func TestLayoutReturnsFixedScreenSize(t *testing.T) {
	w, h := NewGame(&stubScene{}).Layout(1920, 1080)
	if w != WindowWidth || h != WindowHeight {
		t.Errorf("Layout() = (%d, %d), want (%d, %d)", w, h, WindowWidth, WindowHeight)
	}
}

func TestManaText(t *testing.T) {
	if got := manaText(world.Body{Mana: 1234}); got != "mana 1234/600000" {
		t.Errorf("manaText = %q", got)
	}
	if got := manaText(world.Body{Depleted: true}); got != "mana DEPLETED" {
		t.Errorf("manaText = %q", got)
	}
}

// The spec says its first examples are the ones shown in the game.
func TestExamplesAreInSpec(t *testing.T) {
	spec, err := os.ReadFile("../../docs/spec.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range examples {
		if !strings.Contains(string(spec), e.spell) {
			t.Errorf("docs/spec.md does not contain the example %q", e.spell)
		}
	}
}

func TestResultText(t *testing.T) {
	for r, want := range map[match.Result]string{
		match.Player0Wins: "YOU WIN",
		match.Player1Wins: "YOU LOSE",
		match.Draw:        "DRAW",
	} {
		if got := resultText(r); got != want {
			t.Errorf("resultText(%v) = %q, want %q", r, got, want)
		}
	}
}

func TestTimeText(t *testing.T) {
	for tick, want := range map[uint32]string{
		0:                          "3:00",
		60:                         "2:59",
		match.TimeLimitTicks - 1:   "0:01",
		match.TimeLimitTicks:       "0:00",
		match.TimeLimitTicks + 100: "0:00",
	} {
		if got := timeText(tick); got != want {
			t.Errorf("timeText(%d) = %q, want %q", tick, got, want)
		}
	}
}
