package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
)

// POSTCONDITIONS — docs/spec/postconditions.md.
//
// A contract is `∀x. P(x) ⟹ Q(x, f(x))`. Everything here tests one of the two
// lemmas that implication forces, because getting either wrong is unsound
// rather than imprecise.

// tempTarget writes a target declaring one primitive, so a contract can be
// tested without editing a shipped target file.
func tempTarget(t *testing.T, prim string) *Target {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "t.oro")
	src := `(target tgt
  (type int "int") (type bool "bool") (type any "any")
  (array-type "[]%s")
  (module tgt
    (prim + ((a int) (b int)) int expr "%s + %s" pure)
    (prim - ((a int) (b int)) int expr "%s - %s" pure)
    (prim * ((a int) (b int)) int expr "%s * %s" pure)
    (prim < ((a int) (b int)) bool expr "%s < %s" pure)
    (prim <= ((a int) (b int)) bool expr "%s <= %s" pure)
    (prim > ((a int) (b int)) bool expr "%s > %s" pure)
    (prim >= ((a int) (b int)) bool expr "%s >= %s" pure)
    ` + prim + `
    (prim need ((k int)) int expr "need(%s)" pure (where (<= 0 k)))))
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tg, err := LoadTarget(path)
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

func refineWith(t *testing.T, tg *Target, src string) (string, error) {
	t.Helper()
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
	term := terms[0]
	nf, err := core.Normalize(term, env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	notes, err := Refine(tg, "test", nil, nf)
	return strings.Join(notes, "; "), err
}

// propagated reports that an obligation was NOT proven. The refinement layer
// reports rather than assumes an atom outside its fragment, so "no error" does
// not mean "proven" and a test that checks only for an error proves nothing.
func propagated(notes string) bool {
	return strings.Contains(notes, "propagated, not proven")
}

// A postcondition is the only way a PRIMITIVE can establish anything about its
// result: it has no body, so nothing can derive it.
func TestPrimEnsuresDischargesADownstreamObligation(t *testing.T) {
	const prog = `(use tgt)
		(fn (v) (let (tgt.size v) (fn (n) (tgt.need n))))`
	// `need` requires `0 <= k`, and nothing but the postcondition says so.
	tg := tempTarget(t, `(prim size ((v any)) int expr "size(%s)" pure (ensures (<= 0 result)))`)
	notes, err := refineWith(t, tg, prog)
	if err != nil {
		t.Errorf("the postcondition must discharge the obligation: %v", err)
	}
	if propagated(notes) {
		t.Errorf("the obligation must be PROVEN, not propagated: %s", notes)
	}
	// The same program with no postcondition declared must not discharge it.
	// Note that it does not ERROR either: an atom outside the fragment is
	// reported, never assumed, so the note is the whole difference.
	bare := tempTarget(t, `(prim size ((v any)) int expr "size(%s)" pure)`)
	notes, _ = refineWith(t, bare, prog)
	if !propagated(notes) {
		t.Errorf("without a postcondition there is no fact and the obligation "+
			"must be reported unproven, got %q", notes)
	}
}

// LEMMA 1 — an assumption needs its precondition.
//
// A contract is an implication. With P unproven, Q says nothing, and assuming
// it puts a false fact into a conjunctive fragment from which everything
// follows. `f = λx.x` with P ≜ `x > 0` and Q ≜ `result > 0` satisfies the
// contract and is false at `f(-5)`.
func TestEnsuresIsNotAssumedWhenThePreconditionIsUnproven(t *testing.T) {
	// The precondition is `0 < x*x`, which is NON-LINEAR and therefore outside
	// the decidable fragment. That matters for the test: an unprovable-but-
	// refusable precondition aborts the walk on the first error and the
	// downstream obligation is never reached, so nothing is learned. The
	// PROPAGATED path continues, which is what makes the difference observable.
	//
	// Deliberately impure, so ADR 0010 let-binds the call and the guarantee has
	// a name the linear fragment could use — if it were licensed.
	tg := tempTarget(t, `(prim ident ((x int)) int expr "%s" `+
		"(where (< 0 (tgt.* x x))) (ensures (< 0 result)))")
	const prog = `(use tgt)
		(fn (n) (let (tgt.ident n) (fn (y) (tgt.need y))))`
	notes, err := refineWith(t, tg, prog)
	// `ident`'s own precondition is outside the fragment, so it is REPORTED
	// rather than refused — the walk continues, which is what makes the
	// downstream effect observable at all.
	if !strings.Contains(notes, "tgt.ident") {
		t.Errorf("the unproven precondition must be reported: %q", notes)
	}
	// And the guarantee must not have been believed. `need` requires
	// `0 <= y`, and with `0 < y` unlicensed there is no other route to it, so
	// the program is REFUSED. Believing the guarantee would accept it.
	if err == nil {
		t.Fatalf("with P unproven, Q must NOT be assumed — but the downstream "+
			"obligation was discharged (notes %q)", notes)
	}
	if !strings.Contains(err.Error(), "tgt.need") {
		t.Errorf("the refusal must be the downstream obligation: %v", err)
	}

	// THE CONTROL. The same shapes with a precondition the fragment can prove:
	// now the guarantee is licensed and the downstream obligation is proven.
	ok := tempTarget(t, `(prim ident ((x int)) int expr "%s" `+
		"(where (< 0 x)) (ensures (< 0 result)))")
	notes, err = refineWith(t, ok, `(use tgt)
		(fn (n) (let (tgt.ident 7) (fn (y) (tgt.need y))))`)
	if err != nil || propagated(notes) {
		t.Errorf("with P discharged the guarantee holds: %v / %q", err, notes)
	}
}

// LEMMA 2 — a postcondition attaches to the BINDER, not to the call.
//
// Two occurrences of an impure call denote different values and the fact layer
// is keyed by printed term. ADR 0010 guarantees the binder exists: an impure
// argument is never substituted, it is let-bound at the application site.
func TestEnsuresAttachesToTheBinder(t *testing.T) {
	tg := tempTarget(t, `(prim readc ((h int)) int expr "readc(%s)" (ensures (<= 0 result)))`)
	notes, err := refineWith(t, tg, `(use tgt)
		(fn (h) (let (tgt.readc h) (fn (a) (tgt.need a))))`)
	if err != nil || propagated(notes) {
		t.Errorf("an impure call's postcondition holds of the name it is bound to: "+
			"%v / %q", err, notes)
	}
}

// `result` names the result, so a parameter may not take the name — otherwise
// an `ensures` would mean two things and the checker would pick one silently.
func TestResultIsReservedInsideAnEnsures(t *testing.T) {
	_, err := core.Read(`(sig f ((result int)) int (ensures (<= 0 result)))`)
	if err == nil {
		t.Fatal("a parameter named `result` alongside an `ensures` must be refused")
	}
	if !strings.Contains(err.Error(), "result") {
		t.Errorf("the refusal must say which name: %v", err)
	}
	// Without an `ensures` the name is ordinary: the language has no keyword.
	if _, err := core.Read(`(sig f ((result int)) int)`); err != nil {
		t.Errorf("`result` is reserved only inside an ensures: %v", err)
	}
}
