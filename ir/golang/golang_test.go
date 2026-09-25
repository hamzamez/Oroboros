package golang

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// print runs gen's front half on a source and prints every export through the
// IR (FromResidual), as `gen -printer ir` does. It skips the legality step, so
// it is for sources that pass it; checked says -checked's rewrite is wanted.
func print(t *testing.T, src string, checked bool) string {
	t.Helper()
	src = filepath.Join("..", "..", src)
	layers, err := emit.SearchPath(src, filepath.Join("..", "..", "targets"))
	if err != nil {
		t.Fatal(err)
	}
	dirs := []string{filepath.Dir(src), filepath.Join("..", "..", "lib")}
	tg, err := emit.LoadTargetLayers("go", layers, dirs)
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
		name := "t-" + q[strings.LastIndex(q, ".")+1:]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if nf, err = emit.DischargeRequires(reqs, tg, name, prog.Sigs[q], nf); err != nil {
			t.Fatal(err)
		}
		sig := prog.Sigs[q]
		if checked {
			if _, sel := emit.Intervals(tg, sig, nf, 0); sel != nil {
				nf = sel
			}
		}
		code, err := FromResidual(tg, name, sig, nf)
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

// TestTheMeasuredRules: each decision of spec §9.4 the gauntlet's programs
// exercise is visible in what is printed.
func TestTheMeasuredRules(t *testing.T) {
	dot := print(t, "examples/native/dot-go.oro", false)
	has(t, dot, "[:", "bounds-check re-slicing, from the `where` len p = len q (L13)")
	has(t, dot, "for ; ; ", "PostVars: the index moves to the post clause")
	has(t, dot, "return v", "soleExit: the accumulator is returned, with no result temporary")
	if n := strings.Count(dot, "float64\n"); n != 0 {
		t.Errorf("soleExit: %d float64 result temporaries declared, want none:\n%s", n, dot)
	}

	tok := print(t, "examples/json/tokenize.oro", false)
	has(t, tok, " || ", "connectives print as operators (L10)")
	has(t, tok, "[]byte", "the stack's element is a byte (Theorem D′)")

	tally := print(t, "examples/native/wordcount-go.oro", true)
	has(t, tally, `panic("int overflow")`, "a trap-mode add prints the target's checked form")
}

// TestNoRestrictWithoutItsPremise: the spec §9.4 witness prints no re-slice.
func TestNoRestrictWithoutItsPremise(t *testing.T) {
	code := print(t, "ir/testdata/count-zeros.oro", false)
	if strings.Contains(code, "[:") {
		t.Fatalf("a re-slice without len a ≤ len b assumed:\n%s", code)
	}
}

// TestPrintingIsAFunction: two prints of one program are one text.
func TestPrintingIsAFunction(t *testing.T) {
	a := print(t, "examples/json/tree.oro", false)
	b := print(t, "examples/json/tree.oro", false)
	if a != b {
		t.Fatal("two prints of one program differ")
	}
}
