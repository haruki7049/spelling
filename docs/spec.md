# Idea machine specification

This document specifies the machine every player programs in Spelling: the instruction set (Idea), the memory map, interrupts, keyboard input, the built-in assembler, the execution model, mana, ELF loading, and the world. It describes the behavior of the current version of the code. Design discussion and open questions live in [issue #15](https://github.com/haruki7049/spelling/issues/15).

Values marked *tentative* are placeholders to be tuned through playtesting. Each one names the Go constant that defines it, so this document and the code can be checked against each other.

Every `asm` code block in this document is assembled by a test (`internal/asm/spec_test.go`), so the examples stay valid.

## 1. Conventions

- Addresses are byte addresses, written like `0x1000_2000`.
- Memory is little-endian. Memory-mapped registers are 32-bit words.
- Values marked *16.16* are signed fixed-point numbers: `0x10000` is 1.0 and `-0x8000` is -0.5.
- "Tick" is one step of the game; see [Execution model](#6-execution-model).

## 2. Instruction set

Idea is **RISC-V RV32I** (the 32-bit base integer instruction set, 32 registers of 32 bits, `x0` hard-wired to 0), plus **`WFI`** from the RISC-V privileged specification. No other extensions (M, A, C, F, Zicsr, ...) exist.

Differences from a standard RISC-V hart:

| Situation | Behavior |
| --- | --- |
| Illegal or unsupported instruction | The CPU **restarts**: PC is set to the entry point and all registers are cleared. Memory is kept. Interrupts are re-enabled. |
| `ECALL`, `EBREAK` | No-op. |
| `FENCE` | No-op (single hart, no caches). |
| `WFI` (`0x10500073`) | Ends the player's execution for the rest of the tick. Execution resumes after `WFI` on the next tick, where pending interrupts are taken first. The unused instructions cost no mana. |
| Misaligned loads, stores, and instruction fetches | Allowed. |
| Access to an unmapped address | Reads return 0; writes are ignored. |

Because an illegal instruction restarts the CPU, writing an illegal instruction into a player's code forces a "reboot" of their program.

## 3. Memory map

Each player has their own address space:

| Address | Region | Section |
| --- | --- | --- |
| `0x0000_0000` | RAM, 64 KiB *(tentative, `match.RAMSize`)* | — |
| `0x1000_0000` | System | [3.1](#31-system-0x1000_0000) |
| `0x1000_1000` | Keyboard | [3.2](#32-keyboard-0x1000_1000) |
| `0x1000_2000` | Own body | [3.3](#33-bodies-0x1000_2000-and-0x1000_3000) |
| `0x1000_3000` | Opponent body | [3.3](#33-bodies-0x1000_2000-and-0x1000_3000) |
| `0x1005_0000` | Built-in assembler window | [3.4](#34-built-in-assembler-window-0x1005_0000) |
| `0x1006_0000` | Immediate-code region, 4 KiB | [3.5](#35-immediate-code-region-0x1006_0000) |

Everything else is unmapped. Planned regions that do not exist yet: the object table (`0x1001_0000`), laws (`0x1002_0000`), win conditions (`0x1002_1000`), watch registration (`0x1003_0000`), defense info (`0x1003_1000`), and the opponent RAM window (`0x1004_0000`).

Register access rules: a byte or halfword access to a register reads or replaces only those bytes of the word. Each register an access touches is read once, so a side effect such as popping a character happens once per access. Writes to read-only registers are ignored.

### 3.1 System (`0x1000_0000`)

| Offset | Register | Access |
| --- | --- | --- |
| `+0x00` | Current tick | read-only |
| `+0x04` | Instructions remaining in this tick, after the current one | read-only |
| `+0x08` | Player number (0 or 1) | read-only |
| `+0x10` | Interrupt enable (0 or 1) | read-write |
| `+0x14` | Saved PC of the interrupted program | read-write |
| `+0x18` | Keyboard interrupt handler address; 0 means "use the built-in line editor" | read-write |
| `+0x1C` | Return: any write returns from the interrupt | write-only |
| `+0x80`-`+0xFC` | Saved registers `x0`-`x31` of the interrupted program | read-write |

### 3.2 Keyboard (`0x1000_1000`)

Used by language implementations (a registered keyboard handler). See [Keyboard input](#5-keyboard-input).

| Offset | Register | Access |
| --- | --- | --- |
| `+0x00` | Number of characters in the buffer | read-only |
| `+0x04` | Next character; **reading pops it** (0 if empty) | read-only |
| `+0x08` | Overflow flag: 1 after input was dropped; write 0 to clear | read-write |

The buffer holds 256 characters *(tentative, `machine.KeyBufferSize`)*. When it is full, new characters are dropped and the overflow flag is set.

### 3.3 Bodies (`0x1000_2000` and `0x1000_3000`)

Both regions have the same layout: `0x1000_2000` is your body and `0x1000_3000` is the opponent's. Writes cost mana (see [Mana](#7-mana)); writing the opponent's body costs 10 times as much *(tentative, `match.OpponentCostFactor`)*.

| Offset | Register | Format | Access | Write cost (own body) |
| --- | --- | --- | --- | --- |
| `+0x00` | x (left edge) | 16.16 | read-write | 10,000 *(`match.CostOwnPosition`)* |
| `+0x04` | y (bottom edge) | 16.16 | read-write | 10,000 |
| `+0x08` | vx, per tick (clamped to ±64.0 by physics) | 16.16 | read-write | 1,000 *(`match.CostOwnMotion`)* |
| `+0x0C` | vy, per tick (clamped to ±64.0 by physics) | 16.16 | read-write | 1,000 |
| `+0x10` | Facing: 1 = right, -1 = left (a negative write means left) | integer | read-write | 1,000 |
| `+0x14` | Grounded (0 or 1) | integer | read-only | — |
| `+0x18` | HP | integer | read-only | — |
| `+0x1C` | Max HP | integer | read-only | — |
| `+0x20` | Mana | integer | read-only | — |
| `+0x24` | Max mana | integer | read-only | — |
| `+0x28` | Mana regeneration per tick | integer | read-only | — |

Physics sets facing from the sign of vx, but a facing written with mana **holds for 20 ticks** *(tentative, `world.ManaHoldTicks`)* before physics takes over again. Writing it again restarts the count. This follows a general rule: a value paid for with mana beats physics for a while.

Examples (the first three are the examples shown in the game):

```asm
lui t0, 0x10002; li t1, 0x80000; sw t1, 8(t0)     # run right (vx = 8.0)
lui t0, 0x10002; li t1, -0x80000; sw t1, 8(t0)    # run left
lui t0, 0x10002; li t1, 0xc0000; sw t1, 12(t0)    # jump (vy = 12.0)
lui t0, 0x10003; li t1, 0x100000; sw t1, 8(t0)    # push the opponent right (costs 10x)
lui t0, 0x10002; lw a0, 0x20(t0)                  # read your mana
```

### 3.4 Built-in assembler window (`0x1005_0000`)

Lets a program assemble text with the built-in assembler (see [Built-in assembler](#8-built-in-assembler)).

| Offset | Register | Access |
| --- | --- | --- |
| `+0x00` | Source address (in RAM) | read-write |
| `+0x04` | Source length in bytes, at most 4,096 *(tentative, `machine.MaxAssemblerSource`)* | read-write |
| `+0x08` | Output address (in RAM); labels resolve relative to it | read-write |
| `+0x0C` | Output capacity in bytes | read-write |
| `+0x10` | Command: any write assembles | write-only |
| `+0x14` | Status | read-only |
| `+0x18` | Output length in bytes | read-only |
| `+0x1C` | Error line (1-based) after a syntax error | read-only |

Each command costs a fixed 2,000 mana *(tentative, `match.CostAssembler`)*, paid first. Status values:

| Status | Meaning |
| --- | --- |
| 0 | OK: the code was written to the output address |
| 1 | Syntax error; see the error line |
| 2 | Out of range: the source or output is not entirely inside RAM, the source is longer than the limit, or the code does not fit the capacity. Nothing is written. Requests are **rejected, never clamped**. |
| 3 | Not enough mana; nothing was done |

Example: assemble the text at `0x200` (14 bytes) into `0x400` and call it.

```asm
lui t0, 0x10050
li t1, 0x200
sw t1, 0(t0)        # source address
li t1, 14
sw t1, 4(t0)        # source length
li t1, 0x400
sw t1, 8(t0)        # output address
li t1, 64
sw t1, 12(t0)       # output capacity
sw zero, 16(t0)     # assemble
li t2, 0x400
jalr ra, 0(t2)      # run the result
```

### 3.5 Immediate-code region (`0x1006_0000`)

4 KiB of memory where the built-in line editor places each submitted line (see [Keyboard input](#5-keyboard-input)). The game overwrites it every time a line runs. It is readable, writable, and executable by the player.

## 4. Interrupts

Interrupts let typed input run while a program keeps running.

- **Taking an interrupt**: before each instruction, if interrupts are enabled and one is pending, the machine saves the PC to System `+0x14` and all 32 registers to System `+0x80`, clears the enable flag, and jumps to the handler. Taking an interrupt costs no instruction.
- **Returning**: any write to System `+0x1C` returns from the interrupt when that instruction completes. The registers, the PC, and the enable flag (set to 1) are restored at once. Register changes made by the handler are therefore discarded; its effects persist through memory writes. A handler may change the saved registers or saved PC to affect the interrupted program (for example, to switch tasks).
- **Nesting**: interrupts stay disabled inside a handler, so new interrupts wait. A handler may set the enable flag itself. A nested interrupt then overwrites the saved PC and registers, so the handler must copy them to RAM first.
- **Restart**: an illegal-instruction restart re-enables interrupts and leaves any handler.
- **Sources** (only the keyboard exists today):

| Source | Pending while | Handler |
| --- | --- | --- |
| Keyboard, with a handler | The keyboard buffer is not empty | The address in System `+0x18` |
| Keyboard, without a handler | A submitted line is waiting | The immediate-code region |

Defense interrupts (watch registration) are planned and will be taken before keyboard interrupts.

This is the whole return sequence used by the built-in line editor:

```asm
lui t0, 0x10000; sw zero, 0x1c(t0)    # return from the interrupt
```

## 5. Keyboard input

Input is typed characters only; there is no key-held state.

### 5.1 Without a language implementation (built-in line editor)

While System `+0x18` is 0, printable ASCII characters (`0x20`-`0x7E`) are inserted into a line editor at the cursor. The line holds up to 256 characters; more input is dropped and sets the overflow flag. Other keys trigger editing actions. **No key is built into the game**: the keys are read from `keybindings.toml` in the user config directory (for example `~/.config/spelling/keybindings.toml` on Linux, or the file given with `-config`). If the file does not exist, the game writes the default file there first. An invalid file stops the game with an error naming the file.

| Action | Effect | Default keys |
| --- | --- | --- |
| `submit` | Assemble and run the line | Enter |
| `line-start` / `line-end` | Move to the start / end | Ctrl+A, Home / Ctrl+E, End |
| `char-left` / `char-right` | Move one character | Ctrl+B, Left / Ctrl+F, Right |
| `delete-backward` / `delete-forward` | Delete before / at the cursor | Backspace / Ctrl+D, Delete |
| `kill-to-start` | Delete from the start to the cursor | Ctrl+U |
| `kill-word-backward` | Delete the word before the cursor | Ctrl+W |
| `history-prev` / `history-next` | Recall earlier lines (practice only) | Up / Down |
| `example-1`...`example-3` | Insert an example spell (practice only) | F1-F3 |

Practice-only actions skip typing, so they only work with the `-practice` option.

On `submit`, the line is assembled as if it were placed at `0x1006_0000`, and the return sequence from [Interrupts](#4-interrupts) is appended. It runs as a keyboard interrupt as soon as interrupts are enabled. A line that fails to assemble, or that is empty, does nothing. Only one submitted line can wait. While one is waiting, further submitted lines are dropped and set the overflow flag.

### 5.2 With a language implementation

Once a program writes a handler address to System `+0x18`, the line editor and the key binding file are not used. Every key goes to the keyboard buffer in **terminal encoding**. This is an encoding, not a key binding: the language decides what each character means.

| Key | Bytes |
| --- | --- |
| Printable character | Its ASCII code |
| Ctrl+A ... Ctrl+Z | `0x01` ... `0x1A` |
| Enter | `0x0A` |
| Backspace | `0x08` |
| Tab | `0x09` |
| Escape | `0x1B` |
| Up / Down / Right / Left | `ESC [ A` / `ESC [ B` / `ESC [ C` / `ESC [ D` |
| Home / End | `ESC [ H` / `ESC [ F` |
| Delete | `ESC [ 3 ~` |

## 6. Execution model

- The game runs at 60 ticks per second *(tentative, `match.TicksPerSecond`)*.
- Each tick, each player's CPU gets a budget of 1,000 instructions *(tentative, `match.InstructionsPerTick`)*.
- The two CPUs execute **one instruction at a time, alternating**. The player with more mana at the start of the tick goes first; on a tie, a coin flip decides. The coin flip is the only randomness in the game.
- A player stops for the tick when their budget is used up, when they execute `WFI`, or when they are depleted.
- After both players stop, the world advances one physics step, and then mana regenerates.
- Everything is integer arithmetic, so the same inputs always give the same results.

## 7. Mana

Mana is an integer, spent by executing instructions and by writing to bodies.

| Item | Value |
| --- | --- |
| Max mana (and starting mana) | 600,000 *(tentative, `match.MaxMana`)* |
| Regeneration | 500 per tick, up to the maximum *(tentative, `match.ManaRegen`)* |
| Each executed instruction | 1 *(tentative, `match.InstructionCost`)* |
| Body writes | See [Bodies](#33-bodies-0x1000_2000-and-0x1000_3000) |
| Built-in assembler window call | 2,000 |

Reading is free.

- **Unaffordable writes** are ignored and cost nothing.
- **Depletion**: a player who cannot pay for the next instruction is depleted for good. Their CPU stops, their body's velocity becomes 0 (movement effects disappear), and their mana never regenerates.
- **Idling**: a program with nothing to do should wait with `WFI`, which spends no mana for the rest of the tick:

```asm
loop: wfi; j loop    # the program a player without an ELF runs
```

## 8. Built-in assembler

The built-in assembler turns text into Idea. It is used by the line editor and the assembler window. It accepts a subset of the RISC-V assembly syntax of GNU as and LLVM. **Everything it accepts means the same thing to LLVM's RISC-V assembler** (checked against recorded LLVM output in tests), so code can move between the game and external toolchains.

- One instruction per statement. Statements are separated by newlines or `;`. `#` starts a comment that runs to the end of the line.
- A statement may start with labels (`loop:`). Label names use letters, digits, `_`, and `.`, and do not start with a digit.
- Mnemonics are case-insensitive. Registers are lowercase: `x0`-`x31` or ABI names (`zero`, `ra`, `sp`, `gp`, `tp`, `t0`-`t6`, `s0`/`fp`, `s1`-`s11`, `a0`-`a7`).
- Integers are decimal, `0x` hex, `0b` binary, or `0`-prefixed octal, optionally negative. `_` separators and `0o` are rejected, as in LLVM.
- Instructions: all of RV32I, and `wfi`. `fence` takes no operands or `pred, succ` (subsets of `iorw`).
- Pseudo-instructions: `nop`, `li`, `la`, `mv`, `j`, `call`, `ret`, `beqz`, `bnez`.
- Loads, stores, and `jalr` use `offset(reg)` operands. The offset may be omitted.
- Branch and jump targets are a label or a byte offset from the instruction. `la` and `call` take a label only.
- Immediates out of range, and misaligned branch or jump offsets, are errors.
- Directives (`.section`, `.word`, ...) and relocations are not supported. Build whole programs with an external toolchain.

## 9. ELF loading

A language implementation is a RISC-V executable passed with `-elf` (a CPU opponent uses `-opponent-elf`). It must be:

- 32-bit (`ELFCLASS32`), little-endian, `EM_RISCV`, and `ET_EXEC`.
- Every `PT_LOAD` segment must fit entirely inside RAM, and its file size must not exceed its memory size.
- The entry point must lie inside a loaded, executable segment.

Segments are copied to RAM, and the rest of each segment's memory size (such as `.bss`) is zero-filled. Other RAM is zero. The CPU starts at the entry point, and a restart returns there. An invalid file is rejected before the match starts. For example, `ld.lld -m elf32lriscv --no-relax --image-base=0 -e 0x200 -Ttext=0x200` produces a loadable file (this is how the assembler tests link LLVM output).

Without `-elf`, the player runs the idle program from [Mana](#7-mana) at address 0 and plays by typing.

## 10. World

- The stage is 1,024 × 576 units. x grows to the right and y grows upward. The floor is y = 0, the walls are x = 0 and x = 1,024, and the ceiling is y = 576 *(`world.Width`, `world.Height`)*.
- Bodies are 32 × 64 units *(tentative)*. A body's position is its bottom-left corner. Player 0 starts at x = 256 and player 1 at x = 736, on the floor, facing each other.
- Each tick, for each body:
  1. Velocities are clamped to ±64 units per tick on each axis *(tentative, `world.MaxSpeed`)*. Gravity then subtracts 0.5 from vy *(tentative, `world.Gravity`)*, and vy is clamped again.
  1. On the ground, friction reduces |vx| by 0.25 *(tentative, `world.Friction`)*.
  1. The body moves by its velocity. The walls, floor, and ceiling stop it and zero the velocity on that axis.
  1. Grounded is updated. Facing follows the sign of vx, unless a facing written with mana is still holding.
- HP starts at 100 *(tentative, `world.MaxHP`)*. Nothing reduces HP yet; damage, winning, and losing are not implemented.

Any velocity can be written, but physics clamps it on the next step, so extreme values cannot overflow or move a body more than 64 units per tick.
