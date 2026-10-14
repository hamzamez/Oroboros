package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// RECORD FACTS AND READS AS ATOMS (refinements.md §5e, claimsdemand-2026-10-13).
//
// A table of pairs is a table of records, Π_{j<n} ℤ²; a record fact relates the
// two components of every stored pair, ∀j. sp[2j] ≤ sp[2j+1], which freq's
// spans need. It is established at a store that writes a record whole, killed
// by a store of one component it reads, and instantiated at a proven read with
// the record's other read beside it, itself proven in range. Each rule has a
// witness the integers refute and a planted fault the witness catches.

// spans is a table of n pairs built by the loop; `store` is the pair stored at
// w, and `after` what is done to the table before it is read.
func spans(store, after string) string {
	return `(use go) (export f)
(sig f ((n (int 1 100)) (src (array int))) int (where (= (len src) (+ n 1))))
(def f (n src)
  (let sp (local sp (table n 0)
            (loop ((sp sp) (w 0))
              (>= w n) sp
              else (again (set sp w ` + store + `) (+ w 1))))
       sq ` + after + `
    (src (- ((sq 0) 1) ((sq 0) 0)))))`
}

// refineFlat is refineGo through the product pass, which turns a table of
// pairs into a table read at stride 2 before refinement, as the drivers do.
func refineFlat(t *testing.T, src string) error {
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
	flat, sig, _, err := FlattenProducts(tg, prog.Sigs[q], nf)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Refine(tg, "f", sig, flat)
	return err
}

func TestRecordFacts(t *testing.T) {
	// ACCEPTED: each pair is (w, w + 1), so end − beg = 1 ≥ 0, a record fact,
	// and end − beg < n + 1 from each component's own bound.
	good := spans("(tuple w (+ w 1))", "sp")
	if err := refineFlat(t, good); err != nil {
		t.Fatalf("end − beg of a pair stored as (w, w + 1) was not proven non-negative: %v", err)
	}
	refused := func(src string) bool {
		err := refineFlat(t, src)
		return err != nil && strings.Contains(err.Error(), "indexing")
	}
	// The records are stored as (w, w + 1), and each iteration then sets
	// component 0 of record 0 to 9: end − beg = −8 there.
	overwritten := `(use go) (export f)
(sig f ((n (int 1 100)) (src (array int))) int (where (= (len src) (+ n 1))))
(def f (n src)
  (let sp (local sp (table n 0)
            (loop ((sp sp) (w 0))
              (>= w n) sp
              else (let b (set sp w (tuple w (+ w 1)))
                     (again (set (b 0) 0 9) (+ w 1)))))
    (src (- ((sp 0) 1) ((sp 0) 0)))))`
	cases := []struct {
		what, src string
		fault     *bool
	}{
		// The pair stored is (w + 1, w): end − beg = −1, and src[−1] has no value.
		// Planted: the record's values not checked at the store.
		{"a record stored as (w + 1, w)", spans("(tuple (+ w 1) w)", "sp"), &plantRecStoreFault},
		// Planted: a one-component store keeping φ though φ reads that component.
		{"component 0 overwritten in the loop", overwritten, &plantRecComponentFault},
	}
	for _, c := range cases {
		if !refused(c.src) {
			t.Errorf("%s: the read src[end − beg] was accepted", c.what)
			continue
		}
		*c.fault = true
		if refused(c.src) {
			t.Errorf("%s: with its fault planted the program is still refused, so the witness cannot see the fault", c.what)
		}
		*c.fault = false
	}

	// A READ IS KEYED BY ITS INDEX: a guard on t[0] says nothing of t[1].
	// Planted: reads keyed by the table alone.
	byIndex := `(use go) (export f)
(sig f ((t (array int)) (u (array int))) int (where (= (len u) 5)))
(def f (t u) (if (and (<= 2 (len t)) (and (<= 0 (t 0)) (< (t 0) 5))) (u (t 1)) 0))`
	if !refused(byIndex) {
		t.Error("a guard on t[0] bounded the read of t[1]")
	}
	plantReadKeyFault = true
	if refused(byIndex) {
		t.Error("with reads keyed by the table alone the program is still refused, so the witness cannot see the fault")
	}
	plantReadKeyFault = false
}

// THE SIBLING READ MUST BE DEFINED: in a table of length 3 read at stride 2,
// t[2] is component 0 of a record whose component 1, t[3], does not exist, so
// the record fact gives nothing there; t[0] has t[1] beside it.
func TestARecordInstanceNeedsItsSiblingInRange(t *testing.T) {
	tg := goNative(t)
	r := &refiner{tgt: tg, pure: pureAtoms(tg)}
	term := func(src string) *core.Term {
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		return forms[0].Term
	}
	instances := func(read string) []*core.Term {
		f := newFacts()
		f.pure = r.pure
		assume(f, term("(= (len t) 3)"))
		f.content["t"] = []*core.Term{recFact(2, core.App(core.Name("<="), core.Name(compVar(0)), core.Name(compVar(1))))}
		return r.readFacts(term(read), f)
	}
	if got := instances("(t 0)"); len(got) != 1 {
		t.Fatalf("t[0] with t[1] beside it gave %v, want the one record instance", got)
	}
	if got := instances("(t 2)"); len(got) != 0 {
		t.Fatalf("t[2], whose sibling t[3] is out of range, gave %v", got)
	}
	plantSiblingFault = true
	defer func() { plantSiblingFault = false }()
	if got := instances("(t 2)"); len(got) == 0 {
		t.Error("with the sibling's range unchecked t[2] still gives nothing, so this test cannot see the fault")
	}
}
