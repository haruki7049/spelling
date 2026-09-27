package match

import "testing"

func TestQueuePendingWriteEnforcesPerPlayerLimit(t *testing.T) {
	m := newMatch(t)
	for i := range MaxPendingWrites {
		if !m.QueuePendingWrite(0, 1, BodyX, uint32(i)) {
			t.Fatalf("write %d: want queued, got rejected", i)
		}
	}
	if m.QueuePendingWrite(0, 1, BodyX, 999) {
		t.Error("write beyond the limit: want rejected, got queued")
	}
	// The other player's queue is independent.
	if !m.QueuePendingWrite(1, 0, BodyX, 1) {
		t.Error("other writer's write: want queued, got rejected")
	}
}

func TestPendingWriteAppliesAfterGracePeriod(t *testing.T) {
	m := newMatch(t)
	want := uint32(12345)
	m.QueuePendingWrite(0, 1, BodyX, want)
	for range GracePeriodTicks - 1 {
		m.resolvePendingWrites()
		if got := uint32(m.World.Bodies[1].X); got == want {
			t.Fatalf("x = %d, want unchanged before the grace period ends", got)
		}
	}
	m.resolvePendingWrites()
	if got := uint32(m.World.Bodies[1].X); got != want {
		t.Errorf("x = %d, want %d after the grace period", got, want)
	}
}

func TestPendingWriteFreesQueueSlotAfterResolving(t *testing.T) {
	m := newMatch(t)
	for range MaxPendingWrites {
		m.QueuePendingWrite(0, 1, BodyX, 1)
	}
	for range GracePeriodTicks {
		m.resolvePendingWrites()
	}
	if !m.QueuePendingWrite(0, 1, BodyX, 2) {
		t.Error("write after the queue drained: want queued, got rejected")
	}
}

func TestPendingWritesResolveDuringStep(t *testing.T) {
	m := newMatch(t)
	want := uint32(777)
	m.QueuePendingWrite(0, 1, BodyFacing, want)
	for range GracePeriodTicks {
		m.Step()
	}
	if got := uint32(m.World.Bodies[1].Facing); got != 1 {
		t.Errorf("facing = %d, want 1 (positive value normalizes to 1)", got)
	}
}
