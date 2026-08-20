package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
)

// docs/spec/target-files.md §3 `length`. The declaration is a claim about the
// HOST call, so the tests that matter are the ones pinning what it does and
// does not license.

// goNative is the NATIVE Go target — targets/go/, not the portable layer.
// goTarget loads portable-go.oro, whose primitives are not the `go.` names
// these tests use, and a target that does not know a name raises no obligation
// for it: the first draft of these tests passed vacuously against it.
func goNative(t *testing.T) *Target {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatalf("load native go target: %v", err)
	}
	return tg
}

func refineGo(t *testing.T, src string) error {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, terms, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	var term *core.Term
	var sig *core.Sig
	if len(prog.Exports) > 0 {
		q := prog.Exports[0]
		term, sig = prog.Defs[q], prog.Sigs[q]
	} else {
		term = terms[0]
	}
	nf, err := core.Normalize(term, env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Refine(tg, "test", sig, nf)
	return err
}

// An `int` argument is a COUNT: `make([]bool, n)` is n long, so an index below
// n is in range. This is the sieve's proof and it has no other route.
func TestLengthFromACount(t *testing.T) {
	if err := refineGo(t, `
		(use go)
		(export f)
		(sig f ((n int)) bool (where (and (go.<= 0 n) (go.< n 1000))))
		(def f (fn (n)
		  (let (go.make-bool n) (fn (c)
		    (loop ((i 0))
		      (go.>= i n)      false
		      (go.at-bool c i) true
		      else             (again (go.+ i 1)))))))
	`); err != nil {
		t.Errorf("an array built with a declared count must be indexable below it: %v", err)
	}
}

// A CONTAINER argument passes its own length through, which is what makes an
// in-place store usable as a loop variable — the threaded sieve's shape.
func TestLengthThroughAThreadedStore(t *testing.T) {
	if err := refineGo(t, `
		(use go)
		(export f)
		(sig f ((n int)) slice-bool (where (and (go.<= 0 n) (go.< n 1000))))
		(def f (fn (n)
		  (loop ((c (go.make-bool n)) (i 0))
		    (go.>= i n)  c
		    else         (again (go.set-bool c i (go.at-bool c i)) (go.+ i 1)))))
	`); err != nil {
		t.Errorf("a threaded store must keep its length as a loop invariant: %v", err)
	}
}

// The negative that matters most. `set-map` and `set-bool` are the same three
// characters of Go and OPPOSITE facts: an array store leaves the length alone,
// a map insert can add a key. Nothing may infer one from the other, and the
// only thing separating them is that one declares `(length 0)` and one does
// not — so this test guards a declaration's ABSENCE.
func TestAMapStoreClaimsNoLength(t *testing.T) {
	tg := goNative(t)
	if p, ok := tg.Prims["go.set-map"]; !ok {
		t.Fatal("go target must declare set-map")
	} else if p.Length != 0 {
		t.Errorf("set-map must not claim a length: a map insert can add a key")
	}
	if p, ok := tg.Prims["go.set-bool"]; !ok {
		t.Fatal("go target must declare set-bool")
	} else if p.Length != 1 {
		t.Errorf("set-bool must pass its container's length through, got %d", p.Length)
	}
}

// And the fallback is silence, not a guess: an array whose length nothing
// declares proves nothing, rather than being assumed long enough.
func TestAnUndeclaredLengthProvesNothing(t *testing.T) {
	err := refineGo(t, `
		(use go)
		(export f)
		(sig f ((n int)) int (where (and (go.<= 0 n) (go.< n 1000))))
		(def f (fn (n)
		  (let (go.make-bool 4) (fn (c)
		    (loop ((i 0))
		      (go.>= i n)      0
		      (go.at-bool c i) 1
		      else             (again (go.+ i 1)))))))
	`)
	if err == nil {
		t.Fatal("indexing a 4-long array under a bound of 1000 must not be provable")
	}
	if !strings.Contains(err.Error(), "at-bool") {
		t.Errorf("the refusal must name the operation, got %v", err)
	}
}

// (length N) must name an argument the primitive has. Checked through a real
// target file, because a bare `(prim …)` is not a top-level form — it is only
// meaningful inside a `(target …)`.
func TestLengthMustNameARealArgument(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.oro")
	src := `(target bad
  (type int "int")
  (type slice-bool "[]bool")
  (prim mk (int) slice-bool expr "make([]bool, %s)" (length 3)))
`
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTarget(path)
	if err == nil || !strings.Contains(err.Error(), "which it does not have") {
		t.Errorf("(length 3) on a one-argument primitive must be rejected, got %v", err)
	}
}
