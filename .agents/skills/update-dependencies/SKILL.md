# Update Workflow: Nix Inputs, Go Version, and Go Modules

Go module dependencies are consumed by Nix through `gomod2nix` (`buildGoApplication` in `flake.nix`), so `go.mod`, `go.sum`, and `gomod2nix.toml` must always be updated together.

## Nix inputs (`flake.lock`)

Dependabot (`.github/dependabot.yml`) checks `flake.lock` daily and opens one pull request per outdated input; review and merge those instead of updating manually. For a manual update of all inputs, create a topic branch and:

1. Run `nix flake update`.
1. Run `treefmt --fail-on-change`, `go build ./...`, `go test ./...`, and `nix build .#default` (inside `nix develop` or via direnv).
1. Commit with `build: nix flake update`.

## Go modules

1. Update dependencies with `go get <module>@<version>` (or `go get -u ./...`), then `go mod tidy`.
1. Regenerate the Nix lock with `gomod2nix` (writes `gomod2nix.toml`).
1. Run `go build ./...`, `go test ./...`, and `nix build .#default` to confirm the Nix build still resolves all modules.
1. Commit `go.mod`, `go.sum`, and `gomod2nix.toml` together, e.g. `build: update ebiten to vX.Y.Z`.

## Go version bumps

When changing the Go version, update all of the following together:

- The `go` directive in `go.mod`
- `pkgs.go` in `flake.nix` if a specific version attribute is required
- `gomod2nix.toml` (regenerate with `gomod2nix`)

`go-release.yml` reads the version via `go-version-file: 'go.mod'`, so it needs no change.
