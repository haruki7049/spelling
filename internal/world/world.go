// Package world implements the deterministic world shared by both players:
// bodies, the stage, and physics. All quantities are integers; positions and
// velocities are 16.16 fixed-point world units (velocities per tick).
//
// Coordinates: x grows to the right and y grows upward. The floor is y = 0,
// the walls are x = 0 and x = Width, and the ceiling is y = Height. A body's
// position is its bottom-left corner.
//
// All numbers here are tentative first-playable values (see issue #15).
package world

// One is 1.0 in 16.16 fixed-point.
const One = 1 << 16

// Stage and body constants.
const (
	Width      = 1024 * One
	Height     = 576 * One
	BodyWidth  = 32 * One
	BodyHeight = 64 * One
	MaxHP      = 100

	Gravity  = One / 2 // subtracted from vy every tick
	Friction = One / 4 // removed from |vx| every tick on the ground

	// MaxSpeed is the largest velocity magnitude on each axis, per tick.
	// Physics clamps velocities to it, which also rules out overflow.
	MaxSpeed = 64 * One

	// DamageThreshold is the speed a body can lose in one impact without
	// damage. Above it, each 1.0 of lost speed costs 1 HP.
	DamageThreshold = 16 * One

	// ManaHoldTicks is how long a value written with mana beats physics
	// before physics re-derives it (see issue #15).
	ManaHoldTicks = 20
)

// Body is a player's body.
type Body struct {
	X, Y   int32 // bottom-left corner
	VX, VY int32
	Facing int32 // 1 = right, -1 = left
	// FacingHold counts down the steps during which physics keeps a
	// facing written with mana instead of deriving it from vx.
	FacingHold int32
	Grounded   bool
	HP         int32

	// Mana is spent by the player's CPU and writes. Depleted is set when
	// the player could not pay for an instruction; it is permanent.
	Mana     int32
	Depleted bool
}

// World holds both bodies. Index 0 starts on the left facing right, and
// index 1 on the right facing left.
type World struct {
	Bodies [2]Body
}

// New returns a world with both bodies standing on the floor.
func New() *World {
	w := &World{}
	w.Bodies[0] = Body{X: Width / 4, Facing: 1, Grounded: true, HP: MaxHP}
	w.Bodies[1] = Body{X: Width*3/4 - BodyWidth, Facing: -1, Grounded: true, HP: MaxHP}
	return w
}

// substeps splits each tick's movement so that no body moves more than 8
// units at a time. Bodies are 32 units wide, so even two bodies meeting at
// max speed cannot skip past each other.
const substeps = 8

// Step advances physics by one tick: velocities are limited, gravity and
// friction apply, bodies move in substeps while hitting the stage and each
// other, and impacts deal damage.
func (w *World) Step() {
	for i := range w.Bodies {
		w.Bodies[i].accelerate()
	}
	var start [2]Body
	copy(start[:], w.Bodies[:])
	a, b := &w.Bodies[0], &w.Bodies[1]
	a.Grounded, b.Grounded = false, false
	for k := range int64(substeps) {
		a.moveSubstep(start[0], k)
		b.moveSubstep(start[1], k)
		collide(a, b)
	}
	for i := range w.Bodies {
		w.Bodies[i].finish()
	}
}

// accelerate applies the speed limit, gravity, and ground friction.
func (b *Body) accelerate() {
	// Clamp before gravity so it cannot overflow, and again after.
	b.VX, b.VY = limit(b.VX), limit(b.VY)
	b.VY = limit(b.VY - Gravity)
	if b.Grounded {
		switch {
		case b.VX > Friction:
			b.VX -= Friction
		case b.VX < -Friction:
			b.VX += Friction
		default:
			b.VX = 0
		}
	}
}

// moveSubstep moves the body through substep k of its velocity at the start
// of the tick (start), unless an impact already stopped it on that axis.
func (b *Body) moveSubstep(start Body, k int64) {
	part := func(v int32) int32 {
		return int32(int64(v)*(k+1)/substeps - int64(v)*k/substeps)
	}
	// Always pass through stageAxis, so a position written outside the stage
	// is pulled back in even when the body is not moving.
	var dx, dy int32
	if b.VX != 0 {
		dx = part(start.VX)
	}
	if b.VY != 0 {
		dy = part(start.VY)
	}
	b.X = b.stageAxis(b.X, dx, Width-BodyWidth, &b.VX)
	b.Y = b.stageAxis(b.Y, dy, Height-BodyHeight, &b.VY)
	if b.Y == 0 {
		b.Grounded = true
	}
}

// stageAxis moves pos by d and stops it inside [0, max]. On contact the
// velocity on that axis is lost, and the impact deals damage.
func (b *Body) stageAxis(pos, d, max int32, v *int32) int32 {
	p := int64(pos) + int64(d)
	switch {
	case p <= 0:
		b.impact(v, 0)
		return 0
	case p >= int64(max):
		b.impact(v, 0)
		return max
	}
	return int32(p)
}

// impact changes the velocity *v to to and deals damage for the change in
// speed above DamageThreshold.
func (b *Body) impact(v *int32, to int32) {
	change := int64(*v) - int64(to)
	if change < 0 {
		change = -change
	}
	*v = to
	if change > DamageThreshold {
		b.HP = max(b.HP-int32((change-DamageThreshold)/One), 0)
	}
}

// collide separates overlapping bodies along the axis of least overlap.
// If they are moving toward each other on that axis, momentum is conserved
// (equal masses, perfectly inelastic): both take the average velocity, and
// each takes impact damage for its own change. A body pushed up onto the
// other stands on it.
func collide(a, b *Body) {
	ox := min(a.X, b.X) + BodyWidth - max(a.X, b.X)
	oy := min(a.Y, b.Y) + BodyHeight - max(a.Y, b.Y)
	if ox <= 0 || oy <= 0 {
		return
	}
	if ox < oy {
		left, right := a, b
		if b.X < a.X { // on a tie, body 0 counts as the left one
			left, right = b, a
		}
		separate(&left.X, &right.X, ox, Width-BodyWidth)
		share(left, right, &left.VX, &right.VX)
		return
	}
	low, high := a, b
	if b.Y < a.Y {
		low, high = b, a
	}
	separate(&low.Y, &high.Y, oy, Height-BodyHeight)
	share(low, high, &low.VY, &high.VY)
	high.Grounded = true
}

// separate pushes lo down and hi up by a total of overlap, sharing it
// evenly. If the stage stops one of them, the other takes the rest.
func separate(lo, hi *int32, overlap, maxPos int32) {
	down := overlap / 2
	*lo -= down
	*hi += overlap - down
	if *lo < 0 {
		*hi -= *lo
		*lo = 0
	}
	if *hi > maxPos {
		*lo -= *hi - maxPos
		*hi = maxPos
	}
}

// finish updates facing after movement.
func (b *Body) finish() {
	switch {
	case b.FacingHold > 0:
		b.FacingHold--
	case b.VX > 0:
		b.Facing = 1
	case b.VX < 0:
		b.Facing = -1
	}
}

// limit clamps a velocity to [-MaxSpeed, MaxSpeed].
func limit(v int32) int32 {
	return min(max(v, -MaxSpeed), MaxSpeed)
}

// share makes two colliding bodies move together along one axis if they
// are approaching: lo (left or lower) moving toward hi faster than hi moves
// away. Both end with the average velocity.
func share(lo, hi *Body, vlo, vhi *int32) {
	if *vlo <= *vhi {
		return // not approaching
	}
	avg := int32((int64(*vlo) + int64(*vhi)) / 2)
	lo.impact(vlo, avg)
	hi.impact(vhi, avg)
}
