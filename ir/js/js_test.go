package js

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// print runs gen's front half on a source and prints every export through the
// IR, as `gen -printer go,js` does.
func print(t *testing.T, src string) string {
	t.Helper()
	src = filepath.Join("..", "..", src)
	layers, err := emit.SearchPath(src, filepath.Join("..", "..", "targets"))
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{filepath.Dir(src), filepath.Join("..", "..", "lib")}
	tg, err := emit.LoadTargetLayers("js", layers, dirs)
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(string(text))
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.LoadWithDefs(forms, func(path string) (string, bool, error) {
		for _, d := range dirs {
			if b, err := os.ReadFile(filepath.Join(d, filepath.FromSlash(path)+".oro")); err == nil {
				return string(b), true, nil
			}
		}
		return "", false, nil
	}, tg.Defs)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	reqs := emit.InstallRequires(env, prog)
	exports := append([]string(nil), prog.Exports...)
	sort.Strings(exports)
	var out strings.Builder
	for _, q := range exports {
		name := q[strings.LastIndex(q, ".")+1:]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if nf, err = emit.DischargeRequires(reqs, tg, name, prog.Sigs[q], nf); err != nil {
			t.Fatal(err)
		}
		code, err := FromResidual(tg, name, prog.Sigs[q], nf)
		if err != nil {
			t.Fatal(err)
		}
		out.WriteString(code)
	}
	return out.String()
}

func has(t *testing.T, code, want, why string) {
	t.Helper()
	if !strings.Contains(code, want) {
		t.Errorf("missing %q (%s) in\n%s", want, why, code)
	}
}

// TestTheMeasuredShapes: the JavaScript decisions of spec §9.4.
func TestTheMeasuredShapes(t *testing.T) {
	dot := print(t, "examples/native/dot-js.oro")
	has(t, dot, "return v", "a tail loop returns from its exit (1.31× on V8)")
	has(t, dot, "for (;; ", "PostVars: the index steps in the post clause")
	if strings.Contains(dot, "break;") {
		t.Errorf("a tail loop breaks instead of returning:\n%s", dot)
	}
	shapes := print(t, "ir/testdata/js-shapes.oro")
	has(t, shapes, ".fill(0)", "a build is packed, not sparse")
}

// TestDivisionIsTheTargetsIntegerDivision: ℤ's division prints as `idiv`'s
// truncating form, and JavaScript's own `/`, division in the reals, stays `/`.
func TestDivisionIsTheTargetsIntegerDivision(t *testing.T) {
	code := print(t, "ir/testdata/js-shapes.oro")
	i := strings.Index(code, "function intDiv")
	j := strings.Index(code, "function realDiv")
	if i < 0 || j < 0 {
		t.Fatalf("functions not found in\n%s", code)
	}
	intDiv, realDiv := code[i:], code[j:]
	if k := strings.Index(intDiv[1:], "export function"); k >= 0 {
		intDiv = intDiv[:k+1]
	}
	has(t, intDiv, "Math.trunc", "ℤ's division is idiv")
	if strings.Contains(realDiv, "Math.trunc") {
		t.Errorf("JavaScript's `/` printed as ℤ's division:\n%s", realDiv)
	}
}

// TestTheSwapIsAParallelMove: a loop that swaps two parameters needs one
// temporary, and computes what the simultaneous assignment computes. Run under
// node when it is on the path.
func TestTheSwapIsAParallelMove(t *testing.T) {
	code := print(t, "ir/testdata/js-shapes.oro")
	has(t, code, "const u", "the swap's cycle is broken by a temporary")
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on the path")
	}
	dir := t.TempDir()
	mod := filepath.Join(dir, "m.mjs")
	if err := os.WriteFile(mod, []byte(code+"\nconsole.log([0,1,2,3].map(swapSum).join(' '), intDiv(7, 2), realDiv(7, 2), zeroFill(0));\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(node, mod).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	// a, b start 1, 2 and swap n times; the result is a + 10b.
	if got, want := strings.TrimSpace(string(out)), "21 12 21 12 3 3.5 7"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
