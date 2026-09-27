package machine

import (
	"testing"
)

func TestWatchRegistrationDefaults(t *testing.T) {
	m := New(0x1000, 0)
	for i := range WatchEntries {
		base := WatchBase + uint32(i)*WatchEntrySize
		if got := m.Read(base+WatchStart, 4); got != 0 {
			t.Errorf("entry %d start = %#x, want 0", i, got)
		}
		if got := m.Read(base+WatchEnd, 4); got != 0 {
			t.Errorf("entry %d end = %#x, want 0", i, got)
		}
		if got := m.Read(base+WatchFlags, 4); got != 0 {
			t.Errorf("entry %d flags = %#x, want 0", i, got)
		}
		if got := m.Read(base+WatchHandler, 4); got != 0 {
			t.Errorf("entry %d handler = %#x, want 0", i, got)
		}
		if got := m.Read(base+WatchEnabled, 4); got != 0 {
			t.Errorf("entry %d enabled = %d, want 0", i, got)
		}
		if got := m.Read(base+WatchPolicy, 4); got != 0 {
			t.Errorf("entry %d policy = %d, want 0", i, got)
		}
	}
}

func TestWatchRegistrationReadWrite(t *testing.T) {
	m := New(0x1000, 0)
	base := uint32(WatchBase) // slot 0

	m.Write(base+WatchStart, 4, 0x1000_2000)
	m.Write(base+WatchEnd, 4, 0x1000_2030)
	m.Write(base+WatchFlags, 4, WatchWrite|WatchRead)
	m.Write(base+WatchHandler, 4, 0x500)
	m.Write(base+WatchEnabled, 4, 1)
	m.Write(base+WatchPolicy, 4, PolicyDeny)

	if got := m.Read(base+WatchStart, 4); got != 0x1000_2000 {
		t.Errorf("start = %#x, want 0x1000_2000", got)
	}
	if got := m.Read(base+WatchEnd, 4); got != 0x1000_2030 {
		t.Errorf("end = %#x, want 0x1000_2030", got)
	}
	if got := m.Read(base+WatchFlags, 4); got != WatchWrite|WatchRead {
		t.Errorf("flags = %#x, want %#x", got, WatchWrite|WatchRead)
	}
	if got := m.Read(base+WatchHandler, 4); got != 0x500 {
		t.Errorf("handler = %#x, want 0x500", got)
	}
	if got := m.Read(base+WatchEnabled, 4); got != 1 {
		t.Errorf("enabled = %d, want 1", got)
	}
	if got := m.Read(base+WatchPolicy, 4); got != PolicyDeny {
		t.Errorf("policy = %d, want %d", got, PolicyDeny)
	}

	// Slot 1 remains zeroed
	base1 := WatchBase + WatchEntrySize
	if got := m.Read(base1+WatchStart, 4); got != 0 {
		t.Errorf("slot 1 start = %#x, want 0", got)
	}
}

func TestWatchMatching(t *testing.T) {
	m := New(0x1000, 0)
	base := uint32(WatchBase)

	// Configure watch for writes to own body: [0x1000_2000, 0x1000_2030)
	m.Write(base+WatchStart, 4, 0x1000_2000)
	m.Write(base+WatchEnd, 4, 0x1000_2030)
	m.Write(base+WatchFlags, 4, WatchWrite)
	m.Write(base+WatchHandler, 4, 0x800)
	m.Write(base+WatchEnabled, 4, 0) // not enabled yet

	if match := m.MatchingWatch(0x1000_2008, true); match != nil {
		t.Fatalf("disabled watch matched, want nil")
	}

	m.Write(base+WatchEnabled, 4, 1)

	// Within range write
	match := m.MatchingWatch(0x1000_2008, true)
	if match == nil {
		t.Fatalf("in-range write did not match")
	}
	if match.Handler != 0x800 {
		t.Errorf("match handler = %#x, want 0x800", match.Handler)
	}

	// Read should not match (only WatchWrite is set)
	if m.MatchingWatch(0x1000_2008, false) != nil {
		t.Errorf("read matched when only WatchWrite configured")
	}

	// Boundary checks
	if m.MatchingWatch(0x1000_2000, true) == nil {
		t.Errorf("start address (inclusive) did not match")
	}
	if m.MatchingWatch(0x1000_2030, true) != nil {
		t.Errorf("end address (exclusive) matched, want nil")
	}
	if m.MatchingWatch(0x1000_1fff, true) != nil {
		t.Errorf("below start address matched, want nil")
	}
}

func TestWatchByteAccessAndOutOfBounds(t *testing.T) {
	m := New(0x1000, 0)
	base := WatchBase

	// Byte write to handler
	m.Write(base+WatchHandler, 4, 0x12345678)
	m.Write(base+WatchHandler+1, 1, 0x99)
	if got := m.Read(base+WatchHandler, 4); got != 0x12349978 {
		t.Errorf("handler after byte write = %#x, want 0x12349978", got)
	}

	// Reserved offsets within entry (+0x18, +0x1c) read 0 and ignore writes
	m.Write(base+0x18, 4, 0xdeadbeef)
	if got := m.Read(base+0x18, 4); got != 0 {
		t.Errorf("reserved offset +0x18 = %#x, want 0", got)
	}

	// Reading beyond WatchEntries reads 0
	oob := WatchBase + uint32(WatchEntries)*WatchEntrySize
	m.Write(oob, 4, 0xdeadbeef)
	if got := m.Read(oob, 4); got != 0 {
		t.Errorf("out of bounds offset %#x = %#x, want 0", oob, got)
	}
}

func TestWatchPriority(t *testing.T) {
	m := New(0x1000, 0)

	// Configure slot 0 for position [0x1000_2000, 0x1000_2008)
	m.Write(WatchBase+WatchStart, 4, 0x1000_2000)
	m.Write(WatchBase+WatchEnd, 4, 0x1000_2008)
	m.Write(WatchBase+WatchFlags, 4, WatchWrite)
	m.Write(WatchBase+WatchHandler, 4, 0x100)
	m.Write(WatchBase+WatchEnabled, 4, 1)

	// Configure slot 1 for entire body [0x1000_2000, 0x1000_2030)
	base1 := WatchBase + WatchEntrySize
	m.Write(base1+WatchStart, 4, 0x1000_2000)
	m.Write(base1+WatchEnd, 4, 0x1000_2030)
	m.Write(base1+WatchFlags, 4, WatchWrite)
	m.Write(base1+WatchHandler, 4, 0x200)
	m.Write(base1+WatchEnabled, 4, 1)

	// For position (0x1000_2000), slot 0 should take priority as lowest index
	match := m.MatchingWatch(0x1000_2000, true)
	if match == nil || match.Handler != 0x100 {
		t.Errorf("expected slot 0 handler 0x100, got %v", match)
	}

	// For velocity (0x1000_2008), only slot 1 matches
	match = m.MatchingWatch(0x1000_2008, true)
	if match == nil || match.Handler != 0x200 {
		t.Errorf("expected slot 1 handler 0x200, got %v", match)
	}
}
