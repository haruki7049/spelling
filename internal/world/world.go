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
)

// Body is a player's body.
type Body struct {
	X, Y     int32 // bottom-left corner
	VX, VY   int32
	Facing   int32 // 1 = right, -1 = left
	Grounded bool
	HP       int32

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

// Step advances physics by one tick.
func (w *World) Step() {
	for i := range w.Bodies {
		w.Bodies[i].step()
	}
}

func (b *Body) step() {
	b.VY -= Gravity
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

	b.X = clampAxis(b.X, b.VX, Width-BodyWidth, &b.VX)
	b.Y = clampAxis(b.Y, b.VY, Height-BodyHeight, &b.VY)
	b.Grounded = b.Y == 0
	if b.VX > 0 {
		b.Facing = 1
	} else if b.VX < 0 {
		b.Facing = -1
	}
}

// clampAxis moves pos by v and stops it inside [0, max], zeroing the
// velocity on contact. It computes in int64 so huge velocities written by a
// player cannot overflow.
func clampAxis(pos, v, max int32, vp *int32) int32 {
	p := int64(pos) + int64(v)
	switch {
	case p <= 0:
		*vp = 0
		return 0
	case p >= int64(max):
		*vp = 0
		return max
	}
	return int32(p)
}
