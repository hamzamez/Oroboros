package ir

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// lowerFile is a program's every entry point through the checker and Lower:
// the first refusal on the way, or nil.
func lowerFile(src, target string) error {
	_, err := lowerFuncs(src, target, false)
	return err
}

// lowerFuncs is lowerFile's functions, with numbers selected when numbers is set
// (numbers.go): the IR the decision reads.
func lowerFuncs(src, target string, numbers bool) ([]*Func, error) {
	var out []*Func
	err := eachEntry(src, target, func(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) error {
		f, err := Lower(tg, name, sig, nf, Options{Decided: true})
		if err != nil {
			return err
		}
		if numbers {
			if _, err := SelectNumbers(tg, f); err != nil {
				return err
			}
		}
		out = append(out, f)
		return nil
	})
	return out, err
}

func eachEntry(src, target string, use func(*emit.Target, string, *core.Sig, *core.Term) error) error {
	layers, err := emit.SearchPath(src, filepath.Join("..", "targets"))
	if err != nil {
		return err
	}
	dirs := []string{filepath.Dir(src), filepath.Join("..", "lib")}
	tg, err := emit.LoadTargetLayers(target, layers, dirs)
	if err != nil {
		return err
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	forms, err := core.ReadEntry(string(text), src)
	if err != nil {
		return err
	}
	prog, _, err := core.LoadWithDefs(forms, resolver(dirs), tg.Defs)
	if err != nil {
		return err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return err
	}
	reqs := emit.InstallRequires(env, prog)
	exports := append([]string(nil), prog.Exports...)
	sort.Strings(exports)
	for _, q := range exports {
		name := "t-" + q[strings.LastIndex(q, ".")+1:]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			return err
		}
		nf = emit.EtaTails(tg, nf)
		sig := prog.Sigs[q]
		if nf, err = emit.DischargeRequires(reqs, tg, name, sig, nf,
			func(x *core.Term) *core.Term { return DischargeRanges(tg, sig, x) }); err != nil {
			return err
		}
		nf = emit.EraseWordAscriptions(tg.Word, nf)
		if err := emit.Check(tg, name, nf); err != nil {
			return err
		}
		if err := use(tg, name, sig, nf); err != nil {
			return err
		}
	}
	return nil
}

// A NUMERAL IS EXACT (ADR 0086, numbers.md §1): `0.1` is one tenth, which no
// double holds, so at a double it is refused, saying why, where it used to be
// rounded silently by the reader. Its premise is computed, not assumed: 0.5 is
// a double and passes the same position, and 0.1 is refused only because its
// denominator has a factor of five.
func TestANumeralOutsideF64IsRefused(t *testing.T) {
	dir := t.TempDir()
	run := func(lit string) error {
		src := "use go\nexport fn f(x ∈ F64) (F64) { go.`f+`(x, " + lit + ") }\n"
		path := filepath.Join(dir, "n.oro")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		return lowerFile(path, "go")
	}
	if err := run("0.5"); err != nil {
		t.Fatalf("0.5 is a double, and was refused: %v", err)
	}
	for _, lit := range []string{"0.1", "1e-400", "1e400"} {
		err := run(lit)
		if err == nil || !strings.Contains(err.Error(), "is not a double") {
			t.Errorf("%s at a double: %v", lit, err)
		}
	}
}
