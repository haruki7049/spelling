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

func TestPositionWrittenOutsideIsClamped(t *testing.T) {
	w := New()
	w.Bodies[0].X = math.MinInt32
	w.Bodies[0].Y = math.MaxInt32
	w.Step()
	if b := w.Bodies[0]; b.X != 0 || b.Y != Height-BodyHeight {
		t.Errorf("x %d y %d, want clamped into the stage", b.X, b.Y)
	}
}

// Without a hold, physics sets facing from the sign of vx every tick, so a
// facing set while moving is overwritten on the next step. Facing only
// sticks while vx is 0.
func TestFacingFollowsVelocity(t *testing.T) {
	w := New()
	w.Bodies[0].VX = 4 * One
	w.Bodies[0].Facing = -1
	w.Step()
	if f := w.Bodies[0].Facing; f != 1 {
		t.Errorf("facing = %d, current behavior overwrites it with the direction of vx (1)", f)
	}

	w = New()
	w.Bodies[0].Facing = -1 // standing still
	w.Step()
	if f := w.Bodies[0].Facing; f != -1 {
		t.Errorf("facing = %d, want -1 kept while vx is 0", f)
	}
}

// A facing written with mana holds against physics for ManaHoldTicks steps.
func TestFacingHoldBeatsPhysicsForAWhile(t *testing.T) {
	w := New()
	w.Bodies[0].VX = 4 * One
	w.Bodies[0].Facing = -1
	w.Bodies[0].FacingHold = ManaHoldTicks
	for i := range ManaHoldTicks {
		w.Step()
		if f := w.Bodies[0].Facing; f != -1 {
			t.Fatalf("step %d: facing = %d, want -1 held", i+1, f)
		}
	}
	w.Bodies[0].VX = 4 * One // keep moving right after friction
	w.Step()
	if f := w.Bodies[0].Facing; f != 1 {
		t.Errorf("facing = %d after the hold, want physics (1) again", f)
	}
}
func TestWallsAndCeiling(t *testing.T) {
	w := New()
	w.Bodies[0].X = MaxSpeed / 2 // closer to the wall than one step at max speed
	w.Bodies[0].VX = -MaxSpeed
	w.Bodies[1].X = Width - BodyWidth - MaxSpeed/2
	w.Bodies[1].Y = Height - BodyHeight - MaxSpeed/2
	w.Bodies[1].VX = MaxSpeed
	w.Bodies[1].VY = MaxSpeed
	w.Step()
	if b := w.Bodies[0]; b.X != 0 || b.VX != 0 {
		t.Errorf("left wall: x %d vx %d", b.X, b.VX)
	}
	if b := w.Bodies[1]; b.X != Width-BodyWidth || b.Y != Height-BodyHeight || b.VX != 0 || b.VY != 0 {
		t.Errorf("right wall/ceiling: x %d y %d vx %d vy %d", b.X, b.Y, b.VX, b.VY)
	}
}

func TestSpeedLimit(t *testing.T) {
	for _, v := range []int32{math.MinInt32, -MaxSpeed - 1, MaxSpeed + 1, math.MaxInt32} {
		w := New()
		b := &w.Bodies[0]
		b.X, b.Y = Width/2, Height/2
		b.Grounded = false
		b.VX, b.VY = v, v
		x0, y0 := b.X, b.Y
		w.Step()
		if b.VX < -MaxSpeed || b.VX > MaxSpeed || b.VY < -MaxSpeed || b.VY > MaxSpeed {
			t.Errorf("v %d: velocity (%d, %d) exceeds the limit %d", v, b.VX, b.VY, MaxSpeed)
		}
		if dx, dy := abs(b.X-x0), abs(b.Y-y0); dx > MaxSpeed || dy > MaxSpeed {
			t.Errorf("v %d: moved (%d, %d) in one step, more than %d", v, dx, dy, MaxSpeed)
		}
	}
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// The most negative vy used to wrap around under gravity and launch the
// body to the ceiling. With the speed limit it falls at max speed and lands.
func TestMinVelocityFallsAndLands(t *testing.T) {
	w := New()
	w.Bodies[0].Y = 100 * One
	w.Bodies[0].Grounded = false
	w.Bodies[0].VY = math.MinInt32
	for range 10 {
		w.Step()
	}
	if b := w.Bodies[0]; b.Y != 0 || !b.Grounded {
		t.Errorf("y %d vy %d grounded %v: want landed", b.Y, b.VY, b.Grounded)
	}
}

// Friction does not overflow for vx = math.MinInt32; the body moves left at
// max speed until it reaches the wall.
func TestFrictionDoesNotOverflowVelocity(t *testing.T) {
	w := New()
	w.Bodies[0].VX = math.MinInt32
	w.Step()
	if b := w.Bodies[0]; b.VX > 0 || b.X >= Width/4 {
		t.Errorf("x %d vx %d: want moving left", b.X, b.VX)
	}
}
