package match

import "github.com/haruki7049/spelling/internal/world"

// Tentative pending-write numbers (see issue #15, "Grace period for writes
// to the watch registration"). Tune GracePeriodTicks by playing.
const (
	MaxPendingWrites = 4                   // per writer, at once
	GracePeriodTicks = 10 * TicksPerSecond // 10 s
)

// PendingWrite is a body write awaiting its grace period before it takes
// effect. Writer and Target are player indices (0 or 1).
type PendingWrite struct {
	Writer    int
	Target    int
	Offset    uint32
	Value     uint32
	TicksLeft uint32
}

// QueuePendingWrite queues a write by writer to target's body register at
// offset, to take effect after the grace period. It reports whether the
// write was queued; it is rejected once writer already has
// MaxPendingWrites pending.
func (m *Match) QueuePendingWrite(writer, target int, offset, value uint32) bool {
	if len(m.pendingWrites[writer]) >= MaxPendingWrites {
		return false
	}
	m.pendingWrites[writer] = append(m.pendingWrites[writer], PendingWrite{
		Writer:    writer,
		Target:    target,
		Offset:    offset,
		Value:     value,
		TicksLeft: GracePeriodTicks,
	})
	return true
}

// resolvePendingWrites advances every pending write by one tick, applying
// and removing the ones whose grace period has ended.
func (m *Match) resolvePendingWrites() {
	for writer := range m.pendingWrites {
		pending := m.pendingWrites[writer][:0]
		for _, pw := range m.pendingWrites[writer] {
			pw.TicksLeft--
			if pw.TicksLeft > 0 {
				pending = append(pending, pw)
				continue
			}
			applyBodyWrite(&m.World.Bodies[pw.Target], pw.Offset, pw.Value)
		}
		m.pendingWrites[writer] = pending
	}
}

// applyBodyWrite sets a body register directly, with the same normalization
// as bodyDevice.WriteReg, but without paying mana or notifying defenses:
// the caller is responsible for both before queuing the write.
func applyBodyWrite(b *world.Body, off, v uint32) {
	switch off {
	case BodyX:
		b.X = int32(v)
	case BodyY:
		b.Y = int32(v)
	case BodyVX:
		b.VX = min(max(int32(v), -world.MaxSpeed), world.MaxSpeed)
	case BodyVY:
		b.VY = min(max(int32(v), -world.MaxSpeed), world.MaxSpeed)
	case BodyFacing:
		if int32(v) < 0 {
			b.Facing = -1
		} else {
			b.Facing = 1
		}
		b.FacingHold = world.ManaHoldTicks
	}
}
