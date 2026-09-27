# Agent Guidelines for `spelling`

This document defines core principles, architectural invariants, and non-negotiable safety rules for AI agents working on the `spelling` repository.

______________________________________________________________________

## 1. Project Overview & Architecture

`spelling` is a 2D game written in Go using [Ebitengine](https://ebitengine.org/) (`github.com/hajimehoshi/ebiten/v2`).

- **Development Environment**: Managed with Nix (`flake.nix`, `flake-parts`), `direnv` (`.envrc`), and `treefmt-nix` for formatting Nix, Go, GitHub Actions, Markdown, and shell scripts. `go`, `gomod2nix`, and `treefmt` are available on `PATH` inside `nix develop` (or via direnv).
- **Target Language Version**: Go as declared by the `go` directive in `go.mod`. Go module dependencies are pinned for Nix in `gomod2nix.toml`, which must be kept in sync with `go.mod`/`go.sum`.
- **Directory Structure**:
  - `cmd/spelling/spelling.go`: Application entry point (window setup and `ebiten.RunGame`). Run with `go run ./cmd/spelling`.
  - `internal/game/`: Game logic (`game.go`: `Game` type and window constants, `scene.go`: scene abstraction, `match_scene.go`: match screen, `input.go`: keyboard input and terminal encoding, `keybindings.go` + `keybindings.toml`: key binding file loading and the embedded default, `history.go`: practice input history).
  - `internal/vm/`: Virtual CPU that runs Idea, the in-game RISC-V RV32I machine code (`cpu.go`: instruction execution, `bus.go`: memory interface and RAM, `elf.go`: validating ELF loader). Game-specific behavior is tracked in issue #15.
  - `internal/asm/`: Built-in assembler that turns typed text into Idea (RISC-V assembly with common pseudo-instructions and labels).
  - `internal/machine/`: One player's machine: CPU, memory map (System, Keyboard, assembler window, immediate-code region), keyboard input, and interrupts. The register layout is documented in the package comment.
  - `internal/world/`: Deterministic world: stage, bodies, and fixed-point physics.
  - `internal/match/`: A match: two machines executing alternately each tick, the world step, and the memory-mapped body regions.
  - `docs/spec.md`: The Idea machine specification (instruction set, memory map, interrupts, input, mana, ELF loading, world). Update it in the same PR as any change to that behavior; its `asm` examples are assembled by `internal/asm/spec_test.go`.
  - `internal/repo/`: Tests about the repository itself (e.g. no Go source file is hidden by `.gitignore`).
  - `flake.nix`, `default.nix`, `shell.nix`: Nix package (`buildGoApplication` via `gomod2nix`), devShell, treefmt config, and flake-compat shims.
  - `scripts/push-artifacts-to-cachix.nu`: Nushell script used by the Cachix workflow.
  - `.github/workflows/`: Nix checks and build (`nix-ci.yml`), release binaries on `v*` tags (`go-release.yml`), Cachix pushes (`cachix-push.yml`), and stale issue/PR handling (`stale-issues-pullrequests.yml`). Dependabot (`.github/dependabot.yml`) opens pull requests for outdated `flake.lock` inputs.

______________________________________________________________________

## 2. Strict Safety & Operational Rules (Always Enforced)

- **NEVER AUTO-MERGE TO MAIN**: AI agents **MUST NEVER** merge PRs, execute `git merge`, or directly push commits to the `main` branch autonomously.
- **NEVER PROPOSE COMMITS OR PUSHES UNPROMPTED**: AI agents **MUST NEVER** prompt the user to commit or push, nor propose commit messages unprompted. When instructed by the user or when creating/updating pull requests on topic branches, agents may execute `git commit` and `git push` directly without seeking confirmation.
- **Mandatory Human Approval**: AI agents may create branches, create commits, push topic branches, propose PRs, format code, and run test suites, but the final action of merging changes into `main` rests strictly with the human maintainer.
- **Verification Before Submitting**: All changes must pass `treefmt --fail-on-change`, `go build ./...`, and `go test ./...`.
- **Conventional Commits**: Use conventional commit prefixes (`feat:`, `fix:`, `refactor:`, `docs:`, `build:`, `test:`), optionally with a scope.
- **Evidence First**: Base all answers and actions on actual file contents and command output. Never speculate or assume.
- **Non-Destructive**: Never perform irreversible actions (file deletions, hard resets, force pushes or other rewrites of pushed history) without explicit user approval. Pushing to `main` is never allowed (see above). Ordinary pushes to your own topic branches are allowed without confirmation.
- **Targeted Edits**: Make minimal, logical changes strictly necessary for the request. Do not modify unrelated files.
- **Small PRs**: Split work into small, focused pull requests on topic branches, each reviewable on its own. When asked what can be implemented, list what the decided spec allows now, then implement it as small PRs.
- **English-Only Documentation**: All repository documentation, agent skills, code comments, commit messages, and PR descriptions must be written strictly in English.
- **Explicit Milestone Assignment Only**: AI agents **MUST NEVER** automatically attach or set GitHub Milestones on Pull Requests or Issues unless explicitly requested or instructed by the user.
- **Fetch Before Branching**: Always run `git fetch origin` before creating a topic branch for a PR and before judging the state of a remote branch (what `main` contains, whether a PR is merged). Branch from the fetched `origin/main`, not a possibly stale local `main`.
- **Announce the Game Window**: Before running anything that opens the game window (`go run ./cmd/spelling`, screenshot harnesses), tell the user in one line: that a window will open (it takes focus and keyboard input), for how long, and how it will be checked (e.g. capturing the game's own frame to a PNG). Permission is not required.

______________________________________________________________________

## 3. Development Workflow

- **Test First**: For new features and decided behavior, write the test first, confirm it fails, then implement. Where practical, confirm that a deliberate break of the implementation makes the tests fail.
- **Doubts Become Characterization Tests**: When a question or doubt about existing behavior comes up (an edge case, an overflow, "what happens if ..."), immediately add a regression test that records the **current** behavior, even if it looks like a bug, with a comment saying it is current behavior and not a decision. **Do not change the implementation** unless the user decides to; report the finding and ask.
- **Design Decisions Go to Issue #15**: Game design is discussed with the user by offering a few concrete options with trade-offs and a clear recommendation. Decisions are recorded in the design memo, issue #15 (English): tick the checklist item, add "**Decided: ...**", and update the affected sections. When editing the issue body, fetch it fresh, check that the fetched body is not empty before writing it back, and diff before and after. If a body is ever lost, restore it from the issue's edit history (GraphQL `userContentEdits`). When asked to list the open questions again, list the remaining ones by priority and mark newly raised ones with ★.
- **Do Not Block on CI**: After pushing, report that CI is running and continue. Check CI when the user asks, before calling a PR mergeable, or when a failure is reported.

______________________________________________________________________

## 4. Status Assessment Workflow

When asked to check status, assess the situation, or understand workspace context:

1. **Local Git State**: Inspect working tree (`git status -s -b`) and recent commits (`git log -n 5 --oneline`).
1. **GitHub PRs (always display)**: List **all** open PRs (`gh pr list`) and check the current branch's PR (`gh pr status`). Never skip this step, even when the local state is clean.
1. **GitHub Issues (always display)**: List **all** open issues (`gh issue list`). Never skip this step.
1. **Environment Health**: Verify build and test status (`treefmt --fail-on-change`, `go build ./...`, `go test ./...`) inside `nix develop`.
1. **Synthesis**: Report a concise, structured status covering local state, remote GitHub state, and environment health. The report **must** include the open PR and Issue lists (number, title, and state), or explicitly state that there are none.

______________________________________________________________________

## 5. Workspace Skills

Detailed runbooks and procedural workflows are maintained as workspace skills under `.agents/skills/`:

| Trigger / Context | Skill to Read | Purpose |
| :--- | :--- | :--- |
| Deep investigation, complex code search | [`investigate`](.agents/skills/investigate/SKILL.md) | Non-destructive investigation guidelines |
| Commit conventions & policies | [`git-commit`](.agents/skills/git-commit/SKILL.md) | Commit conventions and prohibition of unprompted commit/push proposals |
| Deleting files, overwriting, git push/reset | [`irreversible`](.agents/skills/irreversible/SKILL.md) | Pre-checks and confirmation prompts |
| Testing, verifying builds or behavior | [`verify`](.agents/skills/verify/SKILL.md) | Minimal, high-signal verification steps |
| Bumping `flake.lock`, Go version, or Go modules | [`update-dependencies`](.agents/skills/update-dependencies/SKILL.md) | Procedures for Nix input updates, Go version bumps, and `gomod2nix.toml` regeneration |
| Preparing PRs, formatting, pre-submission checks | [`pr-workflow`](.agents/skills/pr-workflow/SKILL.md) | Verification command table, commit rules, and PR requirements |
| "Fresh eyes" sweep for issues not already tracked, sanity-checking a batch of fixes | [`fresh-eyes-audit`](.agents/skills/fresh-eyes-audit/SKILL.md) | Parallel, context-free repo audits to surface gaps a single continuously-informed reviewer would miss |
