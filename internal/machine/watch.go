package machine

// Watch registration region constants.
const (
	WatchBase      uint32 = 0x1003_0000
	WatchEntries          = 4
	WatchEntrySize uint32 = 0x20
	watchSize             = WatchEntries * WatchEntrySize

	// Offsets within each watch entry.
	WatchStart   uint32 = 0x00 // start address of watched range (inclusive)
	WatchEnd     uint32 = 0x04 // end address of watched range (exclusive)
	WatchFlags   uint32 = 0x08 // bit 0 = write, bit 1 = read
	WatchHandler uint32 = 0x0c // defense handler PC
	WatchEnabled uint32 = 0x10 // 1 = enabled, 0 = disabled
	WatchPolicy  uint32 = 0x14 // 0 = allow, 1 = deny

	WatchWrite uint32 = 1 << 0
	WatchRead  uint32 = 1 << 1

	PolicyAllow uint32 = 0
	PolicyDeny  uint32 = 1
)

// WatchEntry is one watch entry configured in the Watch registration region.
type WatchEntry struct {
	Start   uint32
	End     uint32
	Flags   uint32
	Handler uint32
	Enabled bool
	Policy  uint32
}

func (m *Machine) readWatch(off uint32, _ bool) uint32 {
	entryIdx := off / WatchEntrySize
	if entryIdx >= WatchEntries {
		return 0
	}
	e := &m.Watches[entryIdx]
	switch off % WatchEntrySize {
	case WatchStart:
		return e.Start
	case WatchEnd:
		return e.End
	case WatchFlags:
		return e.Flags
	case WatchHandler:
		return e.Handler
	case WatchEnabled:
		return boolToU32(e.Enabled)
	case WatchPolicy:
		return e.Policy
	}
	return 0
}

func (m *Machine) writeWatch(off, v uint32) {
	entryIdx := off / WatchEntrySize
	if entryIdx >= WatchEntries {
		return
	}
	e := &m.Watches[entryIdx]
	switch off % WatchEntrySize {
	case WatchStart:
		e.Start = v
	case WatchEnd:
		e.End = v
	case WatchFlags:
		e.Flags = v & (WatchWrite | WatchRead)
	case WatchHandler:
		e.Handler = v
	case WatchEnabled:
		e.Enabled = v&1 != 0
	case WatchPolicy:
		e.Policy = v & 1
	}
}

// MatchingWatch returns the first enabled watch entry whose address range
// [Start, End) contains addr and whose flags watch this access kind
// (write vs read). It returns nil if no entry matches.
func (m *Machine) MatchingWatch(addr uint32, write bool) *WatchEntry {
	flag := uint32(WatchRead)
	if write {
		flag = WatchWrite
	}
	for i := range m.Watches {
		w := &m.Watches[i]
		if w.Enabled && addr >= w.Start && addr < w.End && w.Flags&flag != 0 {
			return w
		}
	}
	return nil
}
