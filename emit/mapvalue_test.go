package emit

import (
	"testing"

	"oroboros/core"
)

// A MAP'S VALUE RANGE, and it is frozen-2026-08-28's buffer theorem at a
// different index set.
//
//	a value read out of `m` is either ABSENT — and then the `none` branch runs
//	and no payload exists — or it is the most recent `insert`. There is no third
//	source: `build-map` is the only allocator, `insert` the only store, and ADR
//	0018's linearity means nothing else can have written it.
//
// So the hull of the inserted values bounds every value a read can produce, and
// the absent case needs no join because absence is a SUM rather than a
// sentinel. That is arrays-revisited.md §6 again — the discipline does not care
// what the index set is.
//
// Without it a map read is ⊤ and every operation downstream of one is
// unbounded. growth.md called the map's value range FREE; it is free only once
// the analysis is told.
func TestAMapReadCarriesTheInsertedRange(t *testing.T) {
	rep := mapIntervals(t, "(insert m i 7)")
	if rep.Ops == 0 {
		t.Fatal("nothing was counted, so this test proves nothing")
	}
	if rep.Proven != rep.Ops {
		t.Errorf("%d of %d operations bounded, want all: a read of a map whose "+
			"every insert is the literal 7 is in [0,7]", rep.Proven, rep.Ops)
	}
	if !rep.FitsIndex() {
		t.Errorf("MaxOp is %s; `100 * v` for v in [0,7] is 0..700 and fits an "+
			"index", rep.MaxOpRange())
	}
}

// AND IT REFUSES A COMPUTED VALUE, which is soundness rather than laziness.
//
// The derivation is SYNTACTIC on purpose, exactly as `bufferElem` is: a literal
// is its own exact range and anything else refuses. A range too narrow would
// let a read be believed tighter than it is, and only facts exact by
// construction are used. Refusing is always the safe direction.
//
// This is also the control. Without it the test above could pass against an
// analysis that had started believing everything.
func TestAComputedInsertRefusesToNarrow(t *testing.T) {
	rep := mapIntervals(t, "(insert m i (* i 10))")
	if rep.Ops == 0 {
		t.Fatal("nothing was counted, so this test proves nothing")
	}
	if rep.Proven == rep.Ops {
		t.Errorf("all %d operations were bounded; `(* i 10)` is not a literal, "+
			"so the map's value range is not exact by construction and must "+
			"not be claimed", rep.Ops)
	}
}

func mapIntervals(t *testing.T, ins string) *IntervalReport {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := "(use go)\n(export run)\n(def run (fn (n)\n" +
		"  (let (build-map 8 (fn (m)\n" +
		"         (loop ((m m) (i 0))\n" +
		"           (>= i n)  m\n" +
		"           else      (again " + ins + " (+ i 1)))))\n" +
		"    (fn (m) (* 100 (case (m 3) (some v) v none 0))))))\n" +
		"(sig run ((n (int 0 8))) int)\n"
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["run"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	rep, _ := Intervals(tg, prog.Sigs["run"], nf, 0)
	return rep
}
