package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// heldModifiers returns the modifier keys held now.
func heldModifiers() modifiers {
	return modifiers{
		ctrl:  ebiten.IsKeyPressed(ebiten.KeyControl),
		shift: ebiten.IsKeyPressed(ebiten.KeyShift),
		alt:   ebiten.IsKeyPressed(ebiten.KeyAlt),
		meta:  ebiten.IsKeyPressed(ebiten.KeyMeta),
	}
}

// repeated reports a key press with key repeat after a short delay.
func repeated(key ebiten.Key) bool {
	d := inpututil.KeyPressDuration(key)
	return d == 1 || d >= 30 && d%3 == 0
}

// triggered returns the actions whose keys were pressed (or repeated) this
// frame with exactly their modifiers held.
func (kb *KeyBindings) triggered(mods modifiers) []action {
	var out []action
	for _, b := range kb.bindings {
		if b.combo.mods == mods && repeated(b.combo.key) {
			out = append(out, b.action)
		}
	}
	return out
}

// printableChars returns the printable ASCII characters typed this frame.
// While Ctrl, Alt, or Meta is held, typed characters are ignored so that
// shortcuts do not also insert text.
func printableChars(mods modifiers) []byte {
	chars := ebiten.AppendInputChars(nil)
	if mods.ctrl || mods.alt || mods.meta {
		return nil
	}
	var out []byte
	for _, r := range chars {
		if r >= 0x20 && r < 0x7f {
			out = append(out, byte(r))
		}
	}
	return out
}

// terminalKeys encodes special keys the way a terminal sends them. This is
// an encoding, not a key binding: a language implementation receives these
// bytes and gives them meaning.
var terminalKeys = map[ebiten.Key]string{
	ebiten.KeyEnter:       "\n",
	ebiten.KeyNumpadEnter: "\n",
	ebiten.KeyBackspace:   "\b",
	ebiten.KeyTab:         "\t",
	ebiten.KeyEscape:      "\x1b",
	ebiten.KeyArrowUp:     "\x1b[A",
	ebiten.KeyArrowDown:   "\x1b[B",
	ebiten.KeyArrowRight:  "\x1b[C",
	ebiten.KeyArrowLeft:   "\x1b[D",
	ebiten.KeyHome:        "\x1b[H",
	ebiten.KeyEnd:         "\x1b[F",
	ebiten.KeyDelete:      "\x1b[3~",
}

// terminalInput returns this frame's keyboard input in terminal encoding:
// printable characters, Ctrl+letter as 0x01-0x1a, and the special keys above.
func terminalInput() []byte {
	mods := heldModifiers()
	out := printableChars(mods)
	if mods.ctrl {
		for k := ebiten.KeyA; k <= ebiten.KeyZ; k++ {
			if repeated(k) {
				out = append(out, ctrlByte(k))
			}
		}
	}
	for k, seq := range terminalKeys {
		if repeated(k) {
			out = append(out, seq...)
		}
	}
	return out
}

// ctrlByte returns the control character for Ctrl+letter (Ctrl+A = 0x01).
func ctrlByte(letter ebiten.Key) byte {
	return byte(letter-ebiten.KeyA) + 1
}
