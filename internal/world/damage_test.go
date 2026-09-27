package world

import "testing"

// airborne returns a world whose body 0 is at (x, y) in the air, with body 1
// moved out of the way.
func airborne(x, y int32) *World {
	w := New()
	w.Bodies[0].X, w.Bodies[0].Y = x, y
	w.Bodies[0].Grounded = false
	w.Bodies[1].X = Width - BodyWidth
	return w
}

func TestJumpLandingDoesNoDamage(t *testing.T) {
	w := New()
	w.Bodies[0].VY = 12 * One
	for range 100 {
		w.Step()
	}
	if hp := w.Bodies[0].HP; hp != MaxHP {
		t.Errorf("HP = %d after a normal jump, want %d", hp, MaxHP)
	}
}

func TestDamageThreshold(t *testing.T) {
	for _, tt := range []struct {
		speed int32
		want  int32
	}{
		{DamageThreshold, 0},
		{DamageThreshold + One, 1},
		{MaxSpeed, (MaxSpeed - DamageThreshold) / One},
	} {
		// Hit the left wall at the given speed.
		w := airborne(One, 200*One)
		w.Bodies[0].VX = -tt.speed
		w.Step()
		if got := MaxHP - w.Bodies[0].HP; got != tt.want {
			t.Errorf("wall hit at %d: damage %d, want %d", tt.speed/One, got, tt.want)
		}
	}
}

func TestWallSlamAtMaxSpeed(t *testing.T) {
	w := airborne(Width/2, 200*One)
	w.Bodies[0].VX = MaxSpeed
	for range 20 {
		w.Step()
	}
	if got := MaxHP - w.Bodies[0].HP; got != 48 {
		t.Errorf("damage %d from a max-speed wall slam, want 48", got)
	}
}

func TestFallFromTheCeilingHurtsALittle(t *testing.T) {
	w := airborne(Width/2, Height-BodyHeight)
	for range 200 {
		w.Step()
	}
	if got := MaxHP - w.Bodies[0].HP; got <= 0 || got >= 10 {
		t.Errorf("fall damage %d, want a little (1-9)", got)
	}
}

func TestHPDoesNotGoBelowZero(t *testing.T) {
	w := airborne(Width/2, 200*One)
	w.Bodies[0].HP = 1
	w.Bodies[0].VX = MaxSpeed
	for range 20 {
		w.Step()
	}
	if hp := w.Bodies[0].HP; hp != 0 {
		t.Errorf("HP = %d, want 0", hp)
	}
}

func overlap(a, b Body) bool {
	return a.X < b.X+BodyWidth && b.X < a.X+BodyWidth &&
		a.Y < b.Y+BodyHeight && b.Y < a.Y+BodyHeight
}

func TestBodiesDoNotPassThroughEachOther(t *testing.T) {
	w := New()
	w.Bodies[0].X = w.Bodies[1].X - BodyWidth - 10*One
	for range 30 {
		w.Bodies[0].VX = MaxSpeed
		w.Step()
		if overlap(w.Bodies[0], w.Bodies[1]) {
			t.Fatalf("bodies overlap: %+v %+v", w.Bodies[0], w.Bodies[1])
		}
	}
	if w.Bodies[0].X >= w.Bodies[1].X {
		t.Error("body 0 passed through body 1")
	}
}

func TestHeadOnCollisionHurtsBoth(t *testing.T) {
	w := New()
	w.Bodies[0].X = Width/2 - BodyWidth - One
	w.Bodies[1].X = Width/2 + One
	w.Bodies[0].VX, w.Bodies[1].VX = 40*One, -40*One
	w.Step()
	a, b := w.Bodies[0], w.Bodies[1]
	if overlap(a, b) || a.VX != 0 || b.VX != 0 {
		t.Errorf("after a head-on hit: overlap %v, vx %d %d; want separated and stopped", overlap(a, b), a.VX, b.VX)
	}
	// Each loses about 40 (friction takes a little first) -> about 24 damage.
	for i, body := range w.Bodies {
		if d := MaxHP - body.HP; d < 20 || d > 24 {
			t.Errorf("body %d damage %d, want about 24", i, d)
		}
	}
}

// Agreed rule: bodies lose their own velocity along the collision axis. So
// ramming a standing body hurts only the rammer. Recorded as current
// behavior; see issue #15.
func TestRammingHurtsOnlyTheRammer(t *testing.T) {
	w := New()
	w.Bodies[0].X = w.Bodies[1].X - BodyWidth - One
	w.Bodies[0].VX = MaxSpeed
	w.Step()
	if d := MaxHP - w.Bodies[0].HP; d == 0 {
		t.Error("the rammer took no damage")
	}
	if d := MaxHP - w.Bodies[1].HP; d != 0 {
		t.Errorf("the standing body took %d damage; the agreed rule gives 0", d)
	}
}

func TestStandingOnTheOtherBody(t *testing.T) {
	w := New()
	w.Bodies[0].X = w.Bodies[1].X
	w.Bodies[0].Y = BodyHeight + 20*One
	w.Bodies[0].Grounded = false
	for range 60 {
		w.Step()
	}
	a := w.Bodies[0]
	if a.Y != BodyHeight || !a.Grounded || overlap(a, w.Bodies[1]) {
		t.Errorf("y %d grounded %v: want resting on the other body (y %d)", a.Y, a.Grounded, BodyHeight)
	}
}

// FuzzWorld checks invariants for arbitrary positions and velocities, which
// players can write: bodies stay in the stage, never overlap after a step,
// HP stays in [0, MaxHP], and speed stays within the limit.
func FuzzWorld(f *testing.F) {
	f.Add(int32(0), int32(0), int32(MaxSpeed), int32(0), int32(Width/2), int32(0), int32(-MaxSpeed), int32(0))
	f.Add(int32(Width/2), int32(BodyHeight+One), int32(0), int32(-MaxSpeed), int32(Width/2), int32(0), int32(0), int32(MaxSpeed))
	f.Fuzz(func(t *testing.T, ax, ay, avx, avy, bx, by, bvx, bvy int32) {
		w := New()
		w.Bodies[0].X, w.Bodies[0].Y, w.Bodies[0].VX, w.Bodies[0].VY = ax, ay, avx, avy
		w.Bodies[1].X, w.Bodies[1].Y, w.Bodies[1].VX, w.Bodies[1].VY = bx, by, bvx, bvy
		for step := range 30 {
			w.Step()
			for i, b := range w.Bodies {
				if b.X < 0 || b.X > Width-BodyWidth || b.Y < 0 || b.Y > Height-BodyHeight {
					t.Fatalf("step %d: body %d outside the stage: %+v", step, i, b)
				}
				if b.HP < 0 || b.HP > MaxHP {
					t.Fatalf("step %d: body %d HP %d", step, i, b.HP)
				}
				if b.VX < -MaxSpeed || b.VX > MaxSpeed || b.VY < -MaxSpeed || b.VY > MaxSpeed {
					t.Fatalf("step %d: body %d speed (%d, %d)", step, i, b.VX, b.VY)
				}
			}
			if overlap(w.Bodies[0], w.Bodies[1]) {
				t.Fatalf("step %d: bodies overlap: %+v %+v", step, w.Bodies[0], w.Bodies[1])
			}
		}
	})
}
