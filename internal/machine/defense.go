package machine

// Defense info region constants.
const (
	DefenseBase uint32 = 0x1003_1000
	defenseSize uint32 = 0x20

	DefEventCount uint32 = 0x00 // number of events in queue (read-only)
	DefEventWho   uint32 = 0x04 // writer player ID for front event (read-only)
	DefEventAddr  uint32 = 0x08 // target address of front event (read-only)
	DefEventValue uint32 = 0x0c // written value of front event (read-only)
	DefEventPop   uint32 = 0x10 // any write pops the front event (write-only)
	DefOverflow   uint32 = 0x14 // 1 after an event was dropped; write 0 to clear

	DefQueueCap = 16
)

// DefenseEvent is a queued write access event for defense processing.
type DefenseEvent struct {
	Who   uint32
	Addr  uint32
	Value uint32
}

func (m *Machine) readDefense(off uint32, _ bool) uint32 {
	switch off {
	case DefEventCount:
		return uint32(len(m.defEvents))
	case DefEventWho:
		if len(m.defEvents) == 0 {
			return 0
		}
		return m.defEvents[0].Who
	case DefEventAddr:
		if len(m.defEvents) == 0 {
			return 0
		}
		return m.defEvents[0].Addr
	case DefEventValue:
		if len(m.defEvents) == 0 {
			return 0
		}
		return m.defEvents[0].Value
	case DefOverflow:
		return boolToU32(m.defOverflow)
	}
	return 0
}

func (m *Machine) writeDefense(off, v uint32) {
	switch off {
	case DefEventPop:
		if len(m.defEvents) > 0 {
			copy(m.defEvents, m.defEvents[1:])
			m.defEvents = m.defEvents[:len(m.defEvents)-1]
		}
	case DefOverflow:
		m.defOverflow = u32ToBool(v)
	}
}

// NotifyDefense attempts to record an opponent access event in the defense queue.
// It returns true if the event was queued, or false if the queue was full.
// When the queue is full, the overflow flag is set.
func (m *Machine) NotifyDefense(who, addr, value uint32) bool {
	if len(m.defEvents) >= DefQueueCap {
		m.defOverflow = true
		return false
	}
	m.defEvents = append(m.defEvents, DefenseEvent{
		Who:   who,
		Addr:  addr,
		Value: value,
	})
	return true
}
