#!/usr/bin/env nu

def main [] {
  # Create target dir
  mkdir target/coverage

  # Run tests with coverage profiling
  go test -coverprofile=target/coverage/coverage.out ./...

  # Print a human-readable per-function coverage summary
  go tool cover -func=target/coverage/coverage.out

  # Render an HTML report for browsing coverage line-by-line
  go tool cover -html=target/coverage/coverage.out -o target/coverage/coverage.html
}
