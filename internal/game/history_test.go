package game

import "testing"

func TestHistory(t *testing.T) {
	var h history
	if _, ok := h.prev("draft"); ok {
		t.Fatal("prev on empty history")
	}
	h.add("one")
	h.add("two")
	h.add("two") // consecutive duplicates are kept once
	h.add("")    // empty lines are not recorded

	steps := []struct {
		up   bool
		want string
		ok   bool
	}{
		{true, "two", true},
		{true, "one", true},
		{true, "", false},
		{false, "two", true},
		{false, "typing", true}, // back to the draft
		{false, "", false},
	}
	for i, s := range steps {
		var got string
		var ok bool
		if s.up {
			got, ok = h.prev("typing")
		} else {
			got, ok = h.next()
		}
		if got != s.want || ok != s.ok {
			t.Errorf("step %d: got %q %v, want %q %v", i, got, ok, s.want, s.ok)
		}
	}
}

func TestHistoryLimit(t *testing.T) {
	var h history
	for i := range maxHistory + 5 {
		h.add(string(rune('a'+i%26)) + string(rune('0'+i%10)) + string(rune('A'+i/26)))
	}
	if len(h.lines) != maxHistory {
		t.Errorf("kept %d lines, want %d", len(h.lines), maxHistory)
	}
}
