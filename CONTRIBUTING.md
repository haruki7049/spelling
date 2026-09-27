# Contributing to `spelling`

This guide describes the day-to-day workflow for human contributors. AI agents should follow [`AGENTS.md`](AGENTS.md) and the skills under `.agents/skills/` instead.

## Development Environment

The toolchain (`go`, `gomod2nix`, `treefmt`) is provided by Nix.

- With [direnv](https://direnv.net/): run `direnv allow` once in the repository root.
- Without direnv: run `nix develop` to enter the devShell.

Run the game with:

```sh
go run ./cmd/spelling
```

## Workflow

1. Sync `main` with `origin/main`:

   ```sh
   git switch main
   git pull --ff-only origin main
   ```

1. Create a topic branch named after the change, e.g. `feat/<topic>`, `fix/<topic>`, or `docs/<topic>`:

   ```sh
   git switch -c feat/my-feature
   ```

1. Make your changes. Keep each change focused on one topic.

1. Verify locally (see below).

1. Commit using [Conventional Commits](#commit-messages), push the branch, and open a pull request against `main`.

1. Wait for CI (`nix-ci.yml`) to pass and for review. Only the maintainer merges into `main`; never push directly to `main`.

## Verification

All of the following must pass before opening a pull request:

```sh
treefmt --fail-on-change   # formatting (run `treefmt` to auto-fix)
go build ./...
go test ./...
```

Recommended additional checks:

| When | Command |
| :--- | :--- |
| Go code changed | `go vet ./...` |
| `go.mod`, `go.sum`, `gomod2nix.toml`, or Nix files changed | `nix build .#default` |
| `flake.nix` or `flake.lock` changed | `nix flake check --all-systems` |

## Commit Messages

Use Conventional Commits prefixes, optionally with a scope (e.g. `feat(vm): ...`):

- `feat:` new feature
- `fix:` bug fix
- `refactor:` restructuring without behavior change
- `docs:` documentation
- `build:` dependencies, Nix, or CI workflows
- `test:` tests

Do not put issue numbers in commit messages or PR titles.

## Pull Requests

A pull request description should include:

- **Summary** of the changes.
- **Linked issue** using a closing keyword (e.g. `Closes #15`) when applicable.
- **Verification**: the commands you ran and their results.
- **Breaking changes**, if any.

## Updating Dependencies

- **Nix inputs**: Dependabot opens a pull request for each outdated input in `flake.lock` (`.github/dependabot.yml`). For a manual update, run `nix flake update`.
- **Go modules**: after `go get` and `go mod tidy`, run `gomod2nix` to regenerate `gomod2nix.toml`, then commit `go.mod`, `go.sum`, and `gomod2nix.toml` together.
- **Go version**: update the `go` directive in `go.mod` and regenerate `gomod2nix.toml`.

## Language

Write all documentation, code comments, commit messages, and PR descriptions in English.
