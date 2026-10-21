#!/usr/bin/env nu

def main [] {
  # Create target dir
  mkdir target/bin

  # Build cmd/spelling/*.go files
  # The path is from project root, because this script is executed by /Makefile
  go build -o target/bin cmd/spelling/*.go
}
