package match

import (
	"math"
	"testing"
)

// velocityWrite writes v to own (0x10002) or opponent (0x10003) vx through
// the memory map and returns the mana spent on the write.
func velocityWrite(t *testing.T, m *Match, base uint32, v int32) int32 {
	t.Helper()
	b := &m.World.Bodies[0]
	before := b.Mana
	m.Machines[0].Write(base+BodyVX, 4, uint32(v))
	return before - b.Mana
}

func TestVelocityCostIsSquareOfChange(t *testing.T) {
	for _, tt := range []struct {
		name  string
		base  uint32
		speed int32
		want  int32
	}{
		{"own 8", OwnBodyBase, 8, 100},
		{"own 12", OwnBodyBase, 12, 225},
		{"own 32", OwnBodyBase, 32, 1_600},
		{"own 64", OwnBodyBase, 64, 6_400},
		{"opponent 8", OpponentBodyBase, 8, 1_000},
		{"opponent 64", OpponentBodyBase, 64, 64_000},
	} {
		m := newMatch(t)
		if got := velocityWrite(t, m, tt.base, tt.speed<<16); got != tt.want {
			t.Errorf("%s: cost %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestSplittingAChangeInOneTickDoesNotSave(t *testing.T) {
	m := newMatch(t)
	total := int32(0)
	for v := int32(8); v <= 64; v += 8 {
		total += velocityWrite(t, m, OwnBodyBase, v<<16)
	}
	if total != 6_400 {
		t.Errorf("eight writes of +8 cost %d, want the same as one write of 64 (6400)", total)
	}
}

func TestChangeAccumulatesBothWays(t *testing.T) {
	// Going to 32 and back to 0 in one tick is a total change of 64.
	m := newMatch(t)
	total := velocityWrite(t, m, OwnBodyBase, 32<<16) + velocityWrite(t, m, OwnBodyBase, 0)
	if total != 6_400 {
		t.Errorf("32 then 0 cost %d, want 6400", total)
	}
}

func TestAccumulatorResetsEachTick(t *testing.T) {
	m := newMatch(t)
	velocityWrite(t, m, OwnBodyBase, 32<<16)
	m.Step()
	b := &m.World.Bodies[0]
	// A fresh tick: the next change of 8 costs 100 again, measured from the
	// current velocity.
	target := b.VX + 8<<16
	if got := velocityWrite(t, m, OwnBodyBase, target); got != 100 {
		t.Errorf("cost %d in a new tick, want 100", got)
	}
}

func TestNoChangeIsFree(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[0].VX = 20 << 16
	if got := velocityWrite(t, m, OwnBodyBase, 20<<16); got != 0 {
		t.Errorf("writing the current velocity cost %d, want 0", got)
	}
}

func TestWrittenVelocityIsClampedBeforeCosting(t *testing.T) {
	m := newMatch(t)
	if got := velocityWrite(t, m, OwnBodyBase, math.MaxInt32); got != 6_400 {
		t.Errorf("writing MaxInt32 cost %d, want 6400 (as for 64)", got)
	}
	if vx := m.World.Bodies[0].VX; vx != 64<<16 {
		t.Errorf("vx = %d, want clamped to 64.0", vx)
	}
}

func TestUnaffordableVelocityWriteIsIgnored(t *testing.T) {
	m := newMatch(t)
	m.World.Bodies[0].Mana = 6_399
	if got := velocityWrite(t, m, OwnBodyBase, 64<<16); got != 0 || m.World.Bodies[0].VX != 0 {
		t.Errorf("cost %d, vx %d: want ignored and free", got, m.World.Bodies[0].VX)
	}
	// The ignored write does not count toward this tick's change.
	m.World.Bodies[0].Mana = MaxMana
	if got := velocityWrite(t, m, OwnBodyBase, 8<<16); got != 100 {
		t.Errorf("cost %d after an ignored write, want 100", got)
	}
}
