package game

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestDefaultKeyBindingsBindEveryAction(t *testing.T) {
	kb, err := ParseKeyBindings(DefaultKeyBindings)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]action{editorActions, practiceActions, matchActions} {
		for name, a := range table {
			if len(kb.keysFor(a)) == 0 {
				t.Errorf("default file leaves %q unbound", name)
			}
		}
	}
	if got := strings.Join(kb.keysFor(actLineStart), "/"); got != "ctrl+a/home" {
		t.Errorf("line-start keys = %q", got)
	}
}

func TestParseKeyBindingsErrors(t *testing.T) {
	tests := []struct {
		src, want string
	}{
		{"[editor]\nfly = [\"f\"]", `unknown action "fly"`},
		{"[practice]\nsubmit = [\"enter\"]", `unknown action "submit"`},
		{"[editor]\nsubmit = [\"hyper+enter\"]", `unknown modifier "hyper"`},
		{"[editor]\nsubmit = [\"entr\"]", `unknown key name "entr"`},
		{"[editor]\nsubmit = [\"enter\"]\nline-end = [\"Enter\"]", "already bound"},
		{"[keys]\nsubmit = [\"enter\"]", `unknown setting "keys`},
		{"[editor]\nsubmit = \"enter\"", "incompatible types"},
		{"not toml", ""},
	}
	for _, tt := range tests {
		_, err := ParseKeyBindings([]byte(tt.src))
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%q: err = %v, want containing %q", tt.src, err, tt.want)
		}
	}
}

func TestParseKeyCombo(t *testing.T) {
	c, err := parseKeyCombo("Ctrl+Shift+ArrowLeft")
	if err != nil {
		t.Fatal(err)
	}
	if c.key != ebiten.KeyArrowLeft || !c.mods.ctrl || !c.mods.shift || c.mods.alt {
		t.Errorf("got %+v", c)
	}
	if c.String() != "ctrl+shift+arrowleft" {
		t.Errorf("String() = %q", c.String())
	}
}

func TestEmptyListUnbinds(t *testing.T) {
	kb, err := ParseKeyBindings([]byte("[editor]\nsubmit = [\"enter\"]\nkill-to-start = []"))
	if err != nil {
		t.Fatal(err)
	}
	if len(kb.keysFor(actKillToStart)) != 0 || len(kb.keysFor(actLineStart)) != 0 {
		t.Error("unlisted and empty actions should be unbound")
	}
}

func TestLoadKeyBindingsWritesDefaultWhenMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spelling", "keybindings.toml")
	if _, err := LoadKeyBindings(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, DefaultKeyBindings) {
		t.Error("written file differs from the default")
	}
}

func TestLoadKeyBindingsKeepsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.toml")
	custom := []byte("[editor]\nsubmit = [\"ctrl+j\"]\n")
	if err := os.WriteFile(path, custom, 0o644); err != nil {
		t.Fatal(err)
	}
	kb, err := LoadKeyBindings(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(kb.keysFor(actSubmit), "/"); got != "ctrl+j" {
		t.Errorf("submit keys = %q, want ctrl+j", got)
	}
	if data, _ := os.ReadFile(path); !bytes.Equal(data, custom) {
		t.Error("existing file was modified")
	}
}

func TestLoadKeyBindingsInvalidFileNamesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keybindings.toml")
	if err := os.WriteFile(path, []byte("[editor]\nfly = [\"f\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadKeyBindings(path)
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("err = %v, want it to name %s", err, path)
	}
}

func TestCtrlByte(t *testing.T) {
	if ctrlByte(ebiten.KeyA) != 0x01 || ctrlByte(ebiten.KeyW) != 0x17 || ctrlByte(ebiten.KeyZ) != 0x1a {
		t.Error("wrong control characters")
	}
}

func TestRematchKey(t *testing.T) {
	kb, err := ParseKeyBindings([]byte("[match]\nrematch = [\"f5\"]"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(kb.keysFor(actRematch), "/"); got != "f5" {
		t.Errorf("rematch keys = %q, want f5", got)
	}
}

// Regression (current behavior, not a decision): a key binding file written
// before an action existed does not bind it. A file from before the rematch
// action leaves rematch unbound, so a decided match cannot be restarted
// until the player adds the key or deletes the file.
func TestOldFileLeavesNewActionsUnbound(t *testing.T) {
	old, _, _ := strings.Cut(string(DefaultKeyBindings), "[match]")
	kb, err := ParseKeyBindings([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if keys := kb.keysFor(actRematch); len(keys) != 0 {
		t.Errorf("rematch keys = %v; current behavior leaves it unbound", keys)
	}
}
