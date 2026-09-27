# Agent Guidelines for `spelling`

This document defines core principles, architectural invariants, and non-negotiable safety rules for AI agents working on the `spelling` repository.

______________________________________________________________________

## 1. Project Overview & Architecture

`spelling` is a 2D game written in Go using [Ebitengine](https://ebitengine.org/) (`github.com/hajimehoshi/ebiten/v2`).

- **Development Environment**: Managed with Nix (`flake.nix`, `flake-parts`), `direnv` (`.envrc`), and `treefmt-nix` for formatting Nix, Go, GitHub Actions, Markdown, and shell scripts. `go`, `gomod2nix`, and `treefmt` are available on `PATH` inside `nix develop` (or via direnv).
- **Target Language Version**: Go as declared by the `go` directive in `go.mod`. Go module dependencies are pinned for Nix in `gomod2nix.toml`, which must be kept in sync with `go.mod`/`go.sum`.
- **Directory Structure**:
  - `cmd/spelling/spelling.go`: Application entry point (window setup and `ebiten.RunGame`). Run with `go run ./cmd/spelling`.
  - `internal/game/`: Game logic (`game.go`: `Game` type and window constants, `scene.go`: scene abstraction, `example_scene.go`: sample scene).
  - `internal/vm/`: Virtual CPU that runs Idea, the in-game RISC-V RV32I machine code (`cpu.go`: instruction execution, `bus.go`: memory interface and RAM, `elf.go`: validating ELF loader). Game-specific behavior is tracked in issue #15.
  - `internal/asm/`: Built-in assembler that turns typed text into Idea (RISC-V assembly with common pseudo-instructions and labels).
  - `internal/machine/`: One player's machine: CPU, memory map (System, Keyboard, assembler window, immediate-code region), keyboard input, and interrupts. The register layout is documented in the package comment.
  - `flake.nix`, `default.nix`, `shell.nix`: Nix package (`buildGoApplication` via `gomod2nix`), devShell, treefmt config, and flake-compat shims.
  - `scripts/push-artifacts-to-cachix.nu`: Nushell script used by the Cachix workflow.
  - `.github/workflows/`: Nix checks and build (`nix-ci.yml`), release binaries on `v*` tags (`go-release.yml`), scheduled `nix flake update` auto-commits (`cron-flake-update.yml`), Cachix pushes (`cachix-push.yml`), and stale issue/PR handling (`stale-issues-pullrequests.yml`).

______________________________________________________________________

## 2. Strict Safety & Operational Rules (Always Enforced)

- **NEVER AUTO-MERGE TO MAIN**: AI agents **MUST NEVER** merge PRs, execute `git merge`, or directly push commits to the `main` branch autonomously.
- **NEVER PROPOSE COMMITS OR PUSHES UNPROMPTED**: AI agents **MUST NEVER** prompt the user to commit or push, nor propose commit messages unprompted. When instructed by the user or when creating/updating pull requests on topic branches, agents may execute `git commit` and `git push` directly without seeking confirmation.
- **Mandatory Human Approval**: AI agents may create branches, create commits, push topic branches, propose PRs, format code, and run test suites, but the final action of merging changes into `main` rests strictly with the human maintainer.
- **Verification Before Submitting**: All changes must pass `treefmt --fail-on-change`, `go build ./...`, and `go test ./...`.
- **Conventional Commits**: Use conventional commit prefixes (`feat:`, `fix:`, `refactor:`, `docs:`, `build:`, `test:`), optionally with a scope.
- **Evidence First**: Base all answers and actions on actual file contents and command output. Never speculate or assume.
- **Non-Destructive**: Never perform irreversible actions (file deletions, hard resets, remote push) without explicit user approval.
- **Targeted Edits**: Make minimal, logical changes strictly necessary for the request. Do not modify unrelated files.
- **English-Only Documentation**: All repository documentation, agent skills, code comments, commit messages, and PR descriptions must be written strictly in English.
- **Explicit Milestone Assignment Only**: AI agents **MUST NEVER** automatically attach or set GitHub Milestones on Pull Requests or Issues unless explicitly requested or instructed by the user.

______________________________________________________________________

## 3. Status Assessment Workflow

When asked to check status, assess the situation, or understand workspace context:

1. **Local Git State**: Inspect working tree (`git status -s -b`) and recent commits (`git log -n 5 --oneline`).
1. **GitHub PRs (always display)**: List **all** open PRs (`gh pr list`) and check the current branch's PR (`gh pr status`). Never skip this step, even when the local state is clean.
1. **GitHub Issues (always display)**: List **all** open issues (`gh issue list`). Never skip this step.
1. **Environment Health**: Verify build and test status (`treefmt --fail-on-change`, `go build ./...`, `go test ./...`) inside `nix develop`.
1. **Synthesis**: Report a concise, structured status covering local state, remote GitHub state, and environment health. The report **must** include the open PR and Issue lists (number, title, and state), or explicitly state that there are none.

______________________________________________________________________

## 4. Workspace Skills

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
