package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The port is regen.sh's former sed, line for line: the import of Go's root
// module, and every `go.` prefix, and nothing else.
func TestThePortRenamesGosPrefixAndImport(t *testing.T) {
	src := "use go\nuse go/os\nfn f(c) { go.byte_at(c) }\n// use go stays in a comment line? no: only a whole line\n  use go\n"
	got := port(src, "js")
	want := "use js\nuse go/os\nfn f(c) { js.byte_at(c) }\n// use go stays in a comment line? no: only a whole line\n  use go\n"
	if got != want {
		t.Errorf("port:\n got %q\nwant %q", got, want)
	}
}

// The list is the 26 files, each source present, each output under gauntlet/.
func TestTheRegenListNamesItsSources(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := readRegenList(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 26 {
		t.Errorf("gauntlet/regen.txt lists %d files; the harnesses call 26", len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if _, err := os.Stat(filepath.Join(root, e.source)); err != nil {
			t.Errorf("line %d: source %s: %v", e.line, e.source, err)
		}
		if !strings.HasPrefix(e.out, "gauntlet/") || seen[e.out] {
			t.Errorf("line %d: output %s is outside the gauntlet or listed twice", e.line, e.out)
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
