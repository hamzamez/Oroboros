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
  (type int (host "int")) (type bool (host "bool")) (type any (host "any"))
  (type (array A) (host "[]%s"))
  (module tgt
    (sig + ((a int) (b int)) int pure (host expr "%s + %s"))
    (sig - ((a int) (b int)) int pure (host expr "%s - %s"))
    (sig * ((a int) (b int)) int pure (host expr "%s * %s"))
    (sig < ((a int) (b int)) bool pure (host expr "%s < %s"))
    (sig <= ((a int) (b int)) bool pure (host expr "%s <= %s"))
    (sig > ((a int) (b int)) bool pure (host expr "%s > %s"))
    (sig >= ((a int) (b int)) bool pure (host expr "%s >= %s"))
    ` + prim + `
    (sig need ((k int)) int pure (where (<= 0 k)) (host expr "need(%s)"))))
`
	if err := os.WriteFile(path, []byte(withWord(src)), 0o644); err != nil {
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
		(fn (v) (let n (tgt.size v)
            (tgt.need n)))`
	// `need` requires `0 <= k`, and nothing but the postcondition says so.
	tg := tempTarget(t, `(sig size ((v any)) int pure (ensures (<= 0 result)) (host expr "size(%s)"))`)
	notes, err := refineWith(t, tg, prog)
	if err != nil {
		t.Errorf("the postcondition must discharge the obligation: %v", err)
	}
	if propagated(notes) {
		t.Errorf("the obligation must be PROVEN, not propagated: %s", notes)
	}
	// The same program with no postcondition declared must not discharge it.
	//
	// It used to be PROPAGATED, because `(size v)` was outside the fragment and an
	// opaque obligation is reported rather than refused. A pure call is an atom
	// now (theories.md §7.9), so `0 <= size(v)` is a linear goal that nothing
	// entails, and it is REFUSED — facts.md §6's predicted gate: naming an atom
	// makes an obligation readable that was opaque before. Either way it is not
	// proven, and that is the property.
	bare := tempTarget(t, `(sig size ((v any)) int pure (host expr "size(%s)"))`)
	notes, err = refineWith(t, bare, prog)
	if err == nil && !propagated(notes) {
		t.Errorf("without a postcondition there is no fact and the obligation "+
			"must not be proven, got %q", notes)
	}
}

// A PURE CALL'S CONTRACT IS A FACT, SO IT PROVES CONSEQUENCES, NOT ONLY ITSELF
// (spec/theories.md §7.9, §7.10 item 3; facts.md §6).
//
// The test above passes by SYNTACTIC MATCH: `need` requires `0 <= (size v)` and
// `size` ensures exactly that printed term, kept as an opaque atom. A consequence
// is not a match. `need1` requires `1 <= k + 1`, which follows from
// `0 <= size(v)` in one linear step — but only if the application `(size v)` is a
// VARIABLE of the linear fragment, which referential transparency licenses for a
// pure call. The control: with no `ensures` there is no fact to use.
func TestAPureContractProvesALinearConsequence(t *testing.T) {
	const prog = `(use tgt) (fn (v) (tgt.need1 (tgt.size v)))`
	const need1 = `(sig need1 ((k int)) int pure (where (<= 1 (tgt.+ k 1))) (host expr "need1(%s)"))`
	tg := tempTarget(t, `(sig size ((v any)) int pure (ensures (<= 0 result)) (host expr "size(%s)"))
    `+need1)
	notes, err := refineWith(t, tg, prog)
	if err != nil || propagated(notes) {
		t.Errorf("0 <= size(v) must prove 1 <= size(v) + 1: err=%v notes=%q", err, notes)
	}
	bare := tempTarget(t, `(sig size ((v any)) int pure (host expr "size(%s)"))
    `+need1)
	notes, err = refineWith(t, bare, prog)
	if err == nil && !propagated(notes) {
		t.Errorf("with no postcondition the obligation must not be proven, got %q", notes)
	}
}

// Σ ADMITS EXACTLY A PURE HOST CALL. An atom is keyed by its printed form, which
// is sound only by referential transparency — so an impure call must never be one
// (two occurrences denote different values), and neither may anything the
// fragment already interprets or keeps opaque on purpose. A buffer read is kept
// out by construction: it is not a declared primitive at all.
func TestTheAtomSignatureAdmitsOnlyPureHostCalls(t *testing.T) {
	tg := tempTarget(t, `(sig size ((v any)) int pure (host expr "size(%s)"))
    (sig tick ((v any)) int (host expr "tick(%s)"))
    (sig put ((v any)) int pure (host stmt "put(%s)"))`)
	sigma := pureAtoms(tg)
	for op, want := range map[string]bool{
		"tgt.size": true,  // pure, an expression: an atom
		"tgt.tick": false, // impure: two calls differ
		"tgt.put":  false, // a statement, not a value
		"tgt.+":    false, // interpreted by the fragment
		"tgt.*":    false, // a product stays opaque, as it was
		"tgt.<=":   false, // a proposition, not a term
		"tgt.need": true,  // pure and an expression, contract or not
		"b":        false, // not a declared primitive: a buffer read's head
	} {
		if got := sigma(op); got != want {
			t.Errorf("Σ(%s) = %v, want %v", op, got, want)
		}
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
	tg := tempTarget(t, `(sig ident ((x int)) int `+
		`(where (< 0 (tgt.* x x))) (ensures (< 0 result)) (host expr "%s"))`)
	const prog = `(use tgt)
		(fn (n) (let y (tgt.ident n)
            (tgt.need y)))`
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
	ok := tempTarget(t, `(sig ident ((x int)) int `+
		`(where (< 0 x)) (ensures (< 0 result)) (host expr "%s"))`)
	notes, err = refineWith(t, ok, `(use tgt)
		(fn (n) (let y (tgt.ident 7)
            (tgt.need y)))`)
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
	tg := tempTarget(t, `(sig readc ((h int)) int (ensures (<= 0 result)) (host expr "readc(%s)"))`)
	notes, err := refineWith(t, tg, `(use tgt)
		(fn (h) (let a (tgt.readc h)
            (tgt.need a)))`)
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
