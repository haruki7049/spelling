#!/usr/bin/env nu

def main [] {
  # go-mod-tidy
  print "Running 'go mod tidy'..."
  go mod tidy

  # gomod2nix for ./gomod2nix.toml
  let gomod2nix_ability = which gomod2nix | is-not-empty
  if ($gomod2nix_ability) {
    print "Running 'gomod2nix'..."
    gomod2nix
  }
}
