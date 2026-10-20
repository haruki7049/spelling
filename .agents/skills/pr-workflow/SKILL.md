# Pull Request & Commit Workflow for `spelling`

This skill defines the procedures for code verification, commit creation, and pull request submission.

## 1. Mandatory Verification Steps

Before committing or opening a PR, execute the following commands (inside `nix develop` or via direnv) and ensure all pass cleanly:

| Task | Command | Description |
| :--- | :--- | :--- |
| **Check All Formatting (treefmt)** | `treefmt --fail-on-change` | Verifies formatting across Nix, Go, GitHub Actions, Markdown, and shell files |
| **Format All Files (treefmt)** | `treefmt` | Auto-formats all files in the repository using treefmt |
| **Build** | `go build ./...` | Compiles all packages |
| **Run All Tests** | `go test ./...` | Executes the unit tests |
| **Vet** | `go vet ./...` | Reports suspicious constructs (recommended for Go code changes) |
| **Nix Build** | `nix build .#default` | Mirrors `nix-ci.yml`; needed when `go.mod`, `go.sum`, `gomod2nix.toml`, or Nix files change, and useful when files were added (the flake only sees git-tracked files). It is slow, so follow the `verify` skill: ask the user before running it |
| **Ignored Files** | `git status -s --ignored` | After `git add -A`, confirm no new source file is hidden by `.gitignore` (`internal/repo` also tests this for Go files) |
| **Nix Flake Check** | `nix flake check --all-systems` | Mirrors `nix-ci.yml`; run when `flake.nix`/`flake.lock` change |

## 2. Commit & PR Title Conventions

Use Conventional Commits style prefixes, optionally with a scope:

- `feat:` New gameplay feature, scene, or capability.
- `fix:` Bug fixes.
- `build:` Updates to `go.mod`, `go.sum`, `gomod2nix.toml`, `flake.nix`, `flake.lock`, or CI workflows.
- `refactor:` Code restructuring without changing behavior.
- `docs:` Updates to README, AGENTS.md, skills, or code documentation.
- `test:` Adding or updating unit tests.

**Do NOT include issue numbers (e.g., `(#24)` or `#24`) anywhere in commit messages (summary or body) or PR titles.** Issue linkage must be done exclusively in the PR description using explicit issue-closing keywords (e.g. `Closes #24`). Squash merges copy every commit message into `main`, so a closing keyword in a commit body can close the wrong issue. The ` (#N)` suffix GitHub itself appends to squash-merge summaries is the only exception.

**Language**: Write all commit messages, PR titles, PR descriptions, and repository documentation strictly in English.

## 3. PR Description Requirements

Ensure the PR description includes:

- **Summary**: Concise overview of changes.
- **Linked Issue / Closes Statement**: Always include an explicit issue-closing keyword (e.g. `Closes #16`, `Fixes #12`, or `Resolves #5`) when resolving an open issue.
- **Verification**: Explicitly list executed verification commands (`treefmt --fail-on-change`, `go test ./...`, etc.) and their success status.
- **Breaking Changes**: Highlight any breaking changes.

## 4. Strict Safety & Approval Rules

- **NEVER MERGE PULL REQUESTS**: AI agents **MUST NEVER** merge PRs (including enabling auto-merge with `gh pr merge --auto`), execute `git merge` into `main`, or directly push commits to the `main` branch autonomously.
- **NEVER PROPOSE COMMITS OR PUSHES UNPROMPTED**: AI agents **MUST NEVER** prompt the user to commit or push unprompted. When instructed by the user or when preparing pull requests on topic branches, agents may execute `git commit` and `git push` directly.
- **Mandatory Human Approval**: AI agents may create branches, create commits, push topic branches, propose PRs, format code, and run test suites, but the final action of merging changes into `main` rests strictly with the human maintainer.
- **Explicit Milestone Assignment Only**: AI agents **MUST NEVER** automatically attach or set GitHub Milestones on Pull Requests or Issues unless explicitly requested or instructed by the user.
