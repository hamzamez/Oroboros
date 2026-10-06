package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/golang"
)

// entryGo takes a program's first export through the drivers' order: reduce,
// η for products, the joins' check, then ir.Entry, and prints Go.
func entryGo(t *testing.T, src string) (string, error) {
	t.Helper()
	tg := goNative(t)
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	nf = emit.EtaTails(tg, nf)
	if err := emit.CheckJoins(tg, nf); err != nil {
		return "", err
	}
	f, _, err := ir.Entry(tg, q, q, prog.Sigs[q], nf, allProgSigs(prog), false, nil)
	if err != nil {
		return "", err
	}
	ir.SelectShifts(tg, f)
	return golang.FromFunc(tg, f)
}

const etaHead = "(use go/encoding/hex)\n(export f)\n(sig f ((n (int 0 100))) int)\n"

// η FOR PRODUCTS (tables.md §2.5): a host call with m declared results at a
// producer's exit is the tuple of its results, p = ⟨π₁ p, …, πₘ p⟩. The short
// spelling and the one with the tuple written out are one program, so they
// print one Go function, and the call is made once.
func TestAHostCallsResultsAreAProductByEta(t *testing.T) {
	short := etaHead + `(def f (n)
  (let src (array 104 105 33)
       (tuple dst k) (build buf (* 2 (len src)) (hex.Encode buf src))
    (if (> k 2) (len dst) 0)))`
	long := etaHead + `(def f (n)
  (let src (array 104 105 33)
       (tuple dst k) (build buf (* 2 (len src)) (let (tuple d c) (hex.Encode buf src) (tuple d c)))
    (if (> k 2) (len dst) 0)))`
	a, err := entryGo(t, short)
	if err != nil {
		t.Fatalf("the short spelling: %v", err)
	}
	b, err := entryGo(t, long)
	if err != nil {
		t.Fatalf("the tuple written out: %v", err)
	}
	if a != b {
		t.Errorf("η changed the program:\n%s\nagainst\n%s", a, b)
	}
	if n := strings.Count(a, "hex.Encode("); n != 1 {
		t.Errorf("the call is made %d times; η evaluates it once:\n%s", n, a)
	}
}

// THROUGH EVERY FORM A TAIL PASSES THROUGH: an `if`'s arms and a loop's exits,
// each giving the call's two results or a tuple.
func TestEtaReachesArmsAndLoopExits(t *testing.T) {
	for _, body := range []string{
		`(build buf (* 2 (len src)) (if (> n 50) (hex.Encode buf src) (tuple buf 0)))`,
		`(build buf (* 2 (len src)) (if (> n 50) (tuple buf 0) (hex.Encode buf src)))`,
		`(build buf (* 2 (len src)) (loop ((i 0)) (>= i 2) (hex.Encode buf src) else (again (+ i 1))))`,
	} {
		src := etaHead + `(def f (n)
  (let src (array 104 105 33)
       (tuple dst k) ` + body + `
    (if (> k 2) (len dst) 0)))`
		if _, err := entryGo(t, src); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
}

// A PATTERN OF ANOTHER SIZE, OR AN EXIT OF ONE VALUE, IS REFUSED BY NAME: η
// applies to a call declaring exactly as many results as the pattern has names.
func TestAnExitThatIsNotAnMTupleIsRefused(t *testing.T) {
	for _, c := range []struct{ pattern, body string }{
		{"(tuple dst k j)", `(build buf 6 (hex.Encode buf src))`},
		{"(tuple dst k)", `(build buf 6 buf)`},
	} {
		src := etaHead + `(def f (n)
  (let src (array 104 105 33)
       ` + c.pattern + ` ` + c.body + `
    (len dst)))`
		_, err := entryGo(t, src)
		if err == nil || !strings.Contains(err.Error(), "every exit must give") {
			t.Errorf("%s over %s: %v", c.pattern, c.body, err)
		}
	}
}
