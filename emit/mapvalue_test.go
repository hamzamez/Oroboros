package emit_test

import (
	"testing"

	"oroboros/core"
	"oroboros/emit"
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
		t.Errorf("%d of %d operations bounded (unproven %v), want all: a read of a map whose "+
			"every insert is the literal 7 is in [0,7]", rep.Proven, rep.Ops, rep.Unproven)
	}
}

// AND IT CLAIMS NOTHING OF A VALUE NO FACT BOUNDS, which is the control:
// without it the test above could pass against an analysis that had started
// believing everything. The inserted value is the parameter k, an unbounded
// `int`, so `100 * v` is not proven. (The term analysis refused any computed
// insert, `(* i 10)` included; the IR's cells carry the inserted values' facts,
// and i ∈ [0, 8] bounds that one.)
func TestAnUnboundedInsertIsNotClaimed(t *testing.T) {
	rep := mapIntervals(t, "(insert m i k)")
	if rep.Ops == 0 {
		t.Fatal("nothing was counted, so this test proves nothing")
	}
	if rep.Proven == rep.Ops {
		t.Errorf("all %d operations were bounded; k is an unbounded int, so the "+
			"map's value range is not bounded and must not be claimed", rep.Ops)
	}
}

func mapIntervals(t *testing.T, ins string) decision {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := "(use go)\n(export run)\n(def run (fn (n k)\n" +
		"  (let t (build-map 8 (fn (m)\n" +
		"         (loop ((m m) (i 0))\n" +
		"           (>= i n)  m\n" +
		"           else      (again " + ins + " (+ i 1)))))\n" +
		"    (* 100 (case (t 3) (some v) v none 0)))))\n" +
		"(sig run ((n (int 0 8)) (k int)) int)\n"
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
	return decideOn(t, tg, prog.Sigs["run"], nf)
}
