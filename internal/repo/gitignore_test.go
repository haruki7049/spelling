// Package repo holds checks about the repository itself.
package repo

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoGoFileIsIgnored guards against .gitignore patterns that hide source
// files. `result*` once matched internal/match/result.go, so the file was
// never committed: local tests passed, but the Nix build in CI failed.
// Skipped where git or the repository is unavailable (e.g. the Nix build).
func TestNoGoFileIsIgnored(t *testing.T) {
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Skip("not in a git repository")
	}
	dir := strings.TrimSpace(string(root))
	for _, args := range [][]string{
		{"ls-files", "--others", "--ignored", "--exclude-standard", "--", "*.go"}, // untracked
		{"ls-files", "--cached", "--ignored", "--exclude-standard", "--", "*.go"}, // tracked
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %s: %v", strings.Join(args, " "), err)
		}
		if files := strings.Fields(string(out)); len(files) > 0 {
			t.Errorf(".gitignore hides Go files: %v", files)
		}
	}
}
