package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
)

// A LENT BUFFER IS NOALIAS, AND ITS PREMISE IS REFUSED TWICE (φ's fourth kind;
// ir/llvm.tableAttr). A kept definition's lent buffer is printed `noalias`:
// during the call its region is reached only through that parameter. The
// premise is that no call hands one buffer to two parameters, and two
// independent checks refuse such a call:
//   - R2 (core/place.go): a buffer is handed to one call once;
//   - with R2 planted off, linearity: the second position reads b as a frozen
//     table, and the callee's store into b is a store into a frozen table.
//
// Both refusals are asserted, R2's under its own name. The attribute's place
// is read off the printed module: the lent buffer takes noalias, and the table
// it only reads does not. noalias over read-only parameters would be sound, but
// it is not claimed.
func TestALentBufferIsNoalias(t *testing.T) {
	const src = "use win/print\nuse os\n\nfn shift(b ∈ Array(0..255) linear, t ∈ Array(0..255)) (Int) {\n" +
		"    loop i = 1, s = 0 {\n        i >= len(b) || i >= len(t) -> s\n" +
		"        else -> { b[i] := t[i - 1]; again(i + 1, s + t[i] % 7 + (t[i] * 3) % 5 + (t[i] + i) % 11) }\n" +
		"    }\n}\n\nexport fn main() {\n    n = len(os.Args())\n" +
		"    var b := array { (k * 37 + n) mod 256 : k ∈ 0..255 }\n    MAIN\n" +
		"    print.print_int(b[200] + b[100] + z + w)\n}\n"
	aliased := plantOnce(t, src, "MAIN", "z = shift(b, b)\n    w = shift(b, b)")
	if _, err := buildAndRunNoExec(t, aliased, "windows"); err == nil || !strings.Contains(err.Error(), "R2") {
		t.Errorf("R2 should refuse a buffer handed to one call twice, got %v", err)
	}
	core.PlantR2Off = true
	_, err := buildAndRunNoExec(t, aliased, "windows")
	core.PlantR2Off = false
	if err == nil || !strings.Contains(err.Error(), "frozen") {
		t.Errorf("with R2 off, linearity should refuse the store into b read as frozen, got %v", err)
	}
	apart := plantOnce(t, src, "MAIN", "c = alloc(array { k mod 256 : k ∈ 0..255 })\n    z = shift(b, c) + shift(b, c)\n    w = shift(b, c) + shift(b, c) + shift(b, c)")
	code, err := emittedLL(t, apart)
	if err != nil {
		t.Fatal(err)
	}
	line := ""
	for _, l := range strings.Split(code, "\n") {
		if strings.HasPrefix(l, "define") && strings.Contains(l, "shift~lent") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("shift is not kept, so it has no parameters to mark:\n%s", code)
	}
	if strings.Count(line, "noalias") != 1 || !strings.Contains(line, "ptr noalias nonnull") {
		t.Errorf("the lent buffer, and only it, should be noalias: %s", line)
	}
}

// emittedLL is the windows module cmd/gen prints for src, its functions named
// as the program names them.
func emittedLL(t *testing.T, src string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	in, out := filepath.Join(dir, "p.oro"), filepath.Join(dir, "p.ll")
	if err := os.WriteFile(in, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/gen", "-name", "x", in, "windows", out)
	cmd.Dir = "../.."
	if b, err := cmd.CombinedOutput(); err != nil {
		return "", &genError{string(b)}
	}
	b, err := os.ReadFile(out)
	return string(b), err
}

type genError struct{ out string }

func (e *genError) Error() string { return e.out }
