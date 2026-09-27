# Spelling

A 2D versus game where you *spell* out incantations on the keyboard. The world runs on the machine code of a virtual RISC-V CPU called Idea: fireballs, defenses, and even movement are reads and writes to memory. The design is tracked in [issue #15](https://github.com/haruki7049/spelling/issues/15).

This is an early playable version: you can move your character by typing Idea, and spells cost mana. Damage and winning are not implemented yet.

## Play

```sh
nix develop   # or use direnv
go run ./cmd/spelling -practice
```

Type a line of RISC-V assembly and press Enter to run it. Your body is memory-mapped at `0x1000_2000` (x, y, vx, vy as 16.16 fixed-point at offsets 0, 4, 8, 12; mana at 0x20) and the opponent's at `0x1000_3000`:

```
lui t0, 0x10002; li t1, 0x80000; sw t1, 8(t0)     # run right (vx = 8.0)
lui t0, 0x10002; li t1, 0xc0000; sw t1, 12(t0)    # jump (vy = 12.0)
```

Change the numbers to run faster or jump higher.

Every instruction and every write costs mana (writing the opponent's body costs 10x). If you cannot pay for an instruction, your mana is depleted for good and you can no longer act. A program that has nothing to do should execute `wfi` to wait for the next tick without spending mana.

### Options

| Option | Meaning |
| --- | --- |
| `-practice` | Enable practice-only keys: input history and example spells (F1-F3 by default) |
| `-config <file>` | Key binding file to use |
| `-elf <file>` | Load your own language implementation (a RISC-V ELF) |
| `-opponent-elf <file>` | Load a CPU opponent (a RISC-V ELF) |

### Key bindings

No key binding is built into the game. On the first run, the default bindings are written to `keybindings.toml` in your config directory (for example `~/.config/spelling/keybindings.toml` on Linux). Edit that file to change the keys; delete it to restore the defaults. A language implementation loaded with `-elf` receives keys as terminal characters instead and defines its own editing.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).
