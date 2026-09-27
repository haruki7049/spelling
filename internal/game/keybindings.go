package game

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/haruki7049/spelling/internal/machine"
)

// DefaultKeyBindings is written to the config directory when no key
// binding file exists. No binding is hard-coded anywhere else.
//
//go:embed keybindings.toml
var DefaultKeyBindings []byte

// action is something a key can trigger in the built-in line editor.
type action int

const (
	actSubmit action = iota
	actLineStart
	actLineEnd
	actCharLeft
	actCharRight
	actDeleteBackward
	actDeleteForward
	actKillToStart
	actKillWordBackward
	actHistoryPrev
	actHistoryNext
	actExample1
	actExample2
	actExample3
	actRematch
)

// editorActions, practiceActions, and matchActions name the actions of each
// table.
var (
	editorActions = map[string]action{
		"submit":             actSubmit,
		"line-start":         actLineStart,
		"line-end":           actLineEnd,
		"char-left":          actCharLeft,
		"char-right":         actCharRight,
		"delete-backward":    actDeleteBackward,
		"delete-forward":     actDeleteForward,
		"kill-to-start":      actKillToStart,
		"kill-word-backward": actKillWordBackward,
	}
	practiceActions = map[string]action{
		"history-prev": actHistoryPrev,
		"history-next": actHistoryNext,
		"example-1":    actExample1,
		"example-2":    actExample2,
		"example-3":    actExample3,
	}
	matchActions = map[string]action{
		"rematch": actRematch,
	}
)

// editActions maps editor actions to machine line editor actions.
var editActions = map[action]machine.EditAction{
	actSubmit:           machine.EditSubmit,
	actLineStart:        machine.EditLineStart,
	actLineEnd:          machine.EditLineEnd,
	actCharLeft:         machine.EditCharLeft,
	actCharRight:        machine.EditCharRight,
	actDeleteBackward:   machine.EditDeleteBackward,
	actDeleteForward:    machine.EditDeleteForward,
	actKillToStart:      machine.EditKillToStart,
	actKillWordBackward: machine.EditKillWordBackward,
}

// modifiers is a set of held modifier keys.
type modifiers struct {
	ctrl, shift, alt, meta bool
}

// keyCombo is a key with the exact modifiers that must be held.
type keyCombo struct {
	key  ebiten.Key
	mods modifiers
}

func (c keyCombo) String() string {
	var parts []string
	for _, m := range []struct {
		on   bool
		name string
	}{{c.mods.ctrl, "ctrl"}, {c.mods.shift, "shift"}, {c.mods.alt, "alt"}, {c.mods.meta, "meta"}} {
		if m.on {
			parts = append(parts, m.name)
		}
	}
	return strings.ToLower(strings.Join(append(parts, c.key.String()), "+"))
}

// binding binds a key combination to an action.
type binding struct {
	combo  keyCombo
	action action
	name   string
}

// KeyBindings are the line editor key bindings read from a config file.
type KeyBindings struct {
	bindings []binding
	missing  []MissingAction
}

// DefaultKeyBindingsPath returns where the key binding file lives by
// default, in the OS user config directory.
func DefaultKeyBindingsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "spelling", "keybindings.toml"), nil
}

// LoadKeyBindings reads the key binding file at path. If it does not exist,
// the default file is written there first. An invalid file is an error.
func LoadKeyBindings(path string) (*KeyBindings, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, DefaultKeyBindings, 0o644); err != nil {
			return nil, err
		}
		data = DefaultKeyBindings
	} else if err != nil {
		return nil, err
	}
	kb, err := ParseKeyBindings(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return kb, nil
}

// ParseKeyBindings parses a key binding file.
func ParseKeyBindings(data []byte) (*KeyBindings, error) {
	var file struct {
		Editor   map[string][]string `toml:"editor"`
		Practice map[string][]string `toml:"practice"`
		Match    map[string][]string `toml:"match"`
	}
	md, err := toml.Decode(string(data), &file)
	if err != nil {
		return nil, err
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, fmt.Errorf("unknown setting %q", undecoded[0].String())
	}

	kb := &KeyBindings{}
	seen := map[keyCombo]string{}
	for _, table := range []struct {
		name    string
		entries map[string][]string
		actions map[string]action
	}{
		{"editor", file.Editor, editorActions},
		{"practice", file.Practice, practiceActions},
		{"match", file.Match, matchActions},
	} {
		names := make([]string, 0, len(table.entries))
		for name := range table.entries {
			names = append(names, name)
		}
		slices.Sort(names) // deterministic error messages
		for _, name := range names {
			act, ok := table.actions[name]
			if !ok {
				return nil, fmt.Errorf("[%s]: unknown action %q", table.name, name)
			}
			for _, key := range table.entries[name] {
				combo, err := parseKeyCombo(key)
				if err != nil {
					return nil, fmt.Errorf("[%s] %s: %w", table.name, name, err)
				}
				if other, dup := seen[combo]; dup {
					return nil, fmt.Errorf("[%s] %s: key %q is already bound to %s", table.name, name, key, other)
				}
				seen[combo] = name
				kb.bindings = append(kb.bindings, binding{combo: combo, action: act, name: name})
			}
		}
		for _, name := range sortedKeys(table.actions) {
			if _, ok := table.entries[name]; !ok {
				kb.missing = append(kb.missing, MissingAction{Table: table.name, Action: name})
			}
		}
	}
	return kb, nil
}

// parseKeyCombo parses "ctrl+shift+a" style key names.
func parseKeyCombo(s string) (keyCombo, error) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	var c keyCombo
	for _, mod := range parts[:len(parts)-1] {
		switch mod {
		case "ctrl":
			c.mods.ctrl = true
		case "shift":
			c.mods.shift = true
		case "alt":
			c.mods.alt = true
		case "meta":
			c.mods.meta = true
		default:
			return keyCombo{}, fmt.Errorf("key %q: unknown modifier %q", s, mod)
		}
	}
	if err := c.key.UnmarshalText([]byte(parts[len(parts)-1])); err != nil {
		return keyCombo{}, fmt.Errorf("key %q: unknown key name %q", s, parts[len(parts)-1])
	}
	return c, nil
}

// keysFor returns the keys bound to an action, for help text.
func (kb *KeyBindings) keysFor(a action) []string {
	var out []string
	for _, b := range kb.bindings {
		if b.action == a {
			out = append(out, b.combo.String())
		}
	}
	return out
}

// MissingAction is an action that a key binding file does not list, usually
// because the file was written before the action existed. It is unbound.
type MissingAction struct {
	Table, Action string
	DefaultKeys   []string // from the default key binding file
}

func (m MissingAction) String() string {
	quoted := make([]string, len(m.DefaultKeys))
	for i, k := range m.DefaultKeys {
		quoted[i] = fmt.Sprintf("%q", k)
	}
	return fmt.Sprintf("key binding for %q is missing; add %s = [%s] under [%s]",
		m.Action, m.Action, strings.Join(quoted, ", "), m.Table)
}

// Missing returns the actions the file does not list, with their default
// keys. An action listed with an empty list is unbound on purpose and is
// not missing. The game never rewrites the file; it only reports these.
func (kb *KeyBindings) Missing() []MissingAction {
	var defaults map[string]map[string][]string
	if _, err := toml.Decode(string(DefaultKeyBindings), &defaults); err != nil {
		panic(fmt.Sprintf("default key bindings: %v", err)) // covered by tests
	}
	out := make([]MissingAction, len(kb.missing))
	for i, m := range kb.missing {
		m.DefaultKeys = defaults[m.Table][m.Action]
		out[i] = m
	}
	return out
}

func sortedKeys(m map[string]action) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
