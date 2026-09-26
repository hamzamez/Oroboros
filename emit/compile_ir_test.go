package emit_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"oroboros/emit"
)

// goBuilds compiles printed Go functions as a package with Go's own compiler,
// which is the judge of what Go refuses ("declared and not used", a constant
// that overflows its type). Imports are the ones the printer recorded.
func goBuilds(t *testing.T, funcs map[string]string) {
	t.Helper()
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on the path")
	}
	dir := t.TempDir()
	src := emit.File("gen", funcs)
	if err := os.WriteFile(filepath.Join(dir, "gen.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module gen\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(gobin, "build", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Go refuses the printed file: %v\n%s\n%s", err, out, src)
	}
}
