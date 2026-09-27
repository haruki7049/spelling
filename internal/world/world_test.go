package world

import (
	"math"
	"testing"
)

func TestBodiesStartOnTheFloor(t *testing.T) {
	w := New()
	w.Step()
	for i, b := range w.Bodies {
		if b.Y != 0 || b.VY != 0 || !b.Grounded {
			t.Errorf("body %d: y %d vy %d grounded %v, want resting on the floor", i, b.Y, b.VY, b.Grounded)
		}
	}
	if w.Bodies[0].Facing != 1 || w.Bodies[1].Facing != -1 {
		t.Error("bodies should face each other")
	}
}

func TestJumpRisesAndLands(t *testing.T) {
	w := New()
	w.Bodies[0].VY = 12 * One
	peak := int32(0)
	for range 100 {
		w.Step()
		peak = max(peak, w.Bodies[0].Y)
	}
	// v^2 / 2g = 144 / 1 = 144 units, minus discretization.
	if peak < 130*One || peak > 144*One {
		t.Errorf("peak = %.2f, want about 144", float64(peak)/One)
	}
	if b := w.Bodies[0]; b.Y != 0 || !b.Grounded {
		t.Errorf("did not land: y %d grounded %v", b.Y, b.Grounded)
	}
}

func TestFrictionStopsARun(t *testing.T) {
	w := New()
	w.Bodies[0].VX = 8 * One
	x0 := w.Bodies[0].X
	for range 40 {
		w.Step()
	}
	b := w.Bodies[0]
	if b.VX != 0 {
		t.Errorf("vx = %d, want stopped by friction", b.VX)
	}
	if b.X <= x0 || b.Facing != 1 {
		t.Errorf("x %d (from %d), facing %d: want moved right", b.X, x0, b.Facing)
	}
}

func TestWallsAndCeiling(t *testing.T) {
	w := New()
	w.Bodies[0].VX = math.MinInt32
	w.Bodies[1].VX = math.MaxInt32
	w.Bodies[1].VY = math.MaxInt32
	w.Step()
	if b := w.Bodies[0]; b.X != 0 || b.VX != 0 {
		t.Errorf("left wall: x %d vx %d", b.X, b.VX)
	}
	if b := w.Bodies[1]; b.X != Width-BodyWidth || b.Y != Height-BodyHeight || b.VX != 0 || b.VY != 0 {
		t.Errorf("right wall/ceiling: x %d y %d vx %d vy %d", b.X, b.Y, b.VX, b.VY)
	}
}

func TestPositionWrittenOutsideIsClamped(t *testing.T) {
	w := New()
	w.Bodies[0].X = math.MinInt32
	w.Bodies[0].Y = math.MaxInt32
	w.Step()
	if b := w.Bodies[0]; b.X != 0 || b.Y != Height-BodyHeight {
		t.Errorf("x %d y %d, want clamped into the stage", b.X, b.Y)
	}
}

// Regression (current behavior, not a decision): with the current
// implementation, gravity wraps vy = math.MinInt32 around to a huge upward
// velocity, so a body falling as fast as possible is launched into the
// ceiling instead of landing. This test records that behavior; if it is
// changed on purpose, update this test.
func TestMinVelocityIsLaunchedToCeiling(t *testing.T) {
	w := New()
	w.Bodies[0].Y = 100 * One
	w.Bodies[0].Grounded = false
	w.Bodies[0].VY = math.MinInt32
	w.Step()
	if b := w.Bodies[0]; b.Y != Height-BodyHeight {
		t.Errorf("y %d vy %d: current behavior launches the body to the ceiling (y %d)", b.Y, b.VY, Height-BodyHeight)
	}
}

// Regression: friction does not overflow for vx = math.MinInt32; the body is
// pushed into the left wall.
func TestFrictionDoesNotOverflowVelocity(t *testing.T) {
	w := New()
	w.Bodies[0].VX = math.MinInt32
	w.Step()
	if b := w.Bodies[0]; b.VX > 0 || b.X != 0 {
		t.Errorf("x %d vx %d: want pushed into the left wall", b.X, b.VX)
	}
}
