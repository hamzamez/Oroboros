package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The list is the 34 files, each source present: the 33 the harnesses call,
// under gauntlet/, 26 for Go, JavaScript and Java and 7 for windows
// (gauntlet-2026-11-16), and windows32's runtime, under lib/rt/, which every
// program of that target links (ADR 0123). The JSON programs' JavaScript and
// Java lines once named a port of Go's names; since they are the language's
// operators every line is its source as it is.
func TestTheRegenListNamesItsSources(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := readRegenList(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 34 {
		t.Errorf("gauntlet/regen.txt lists %d files; the harnesses call 33, and windows32 links its runtime", len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(root, e.source)); err != nil {
			t.Errorf("line %d: source %s: %v", e.line, e.source, err)
		}
		if !(strings.HasPrefix(e.out, "gauntlet/") || strings.HasPrefix(e.out, "lib/rt/")) || seen[e.out] {
			t.Errorf("line %d: output %s is outside the gauntlet and the runtimes, or listed twice", e.line, e.out)
		}
		seen[e.out] = true
	}
}

// THE COMPARISON MUST BE ABLE TO FAIL: a changed copy and a missing one, each
// named; the identical one not.
func TestAStaleGauntletFileIsNamed(t *testing.T) {
	root, gen := t.TempDir(), t.TempDir()
	write := func(dir, name, text string) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	entries := []regenEntry{{out: "gauntlet/a.go", source: "a.oro"}, {out: "gauntlet/b.go", source: "b.oro"}, {out: "gauntlet/c.go", source: "c.oro"}}
	write(gen, "gauntlet/a.go", "same")
	write(root, "gauntlet/a.go", "same")
	write(gen, "gauntlet/b.go", "new")
	write(root, "gauntlet/b.go", "old")
	write(gen, "gauntlet/c.go", "new")
	stale := staleFiles(root, gen, entries)
	if len(stale) != 2 || !strings.Contains(stale[0], "b.go: differs") || !strings.Contains(stale[1], "c.go: not committed") {
		t.Errorf("want b.go differing and c.go missing, got %v", stale)
	}
}
