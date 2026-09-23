package emit

import (
	"testing"

	"oroboros/core"
)

// THE REFINER'S MEMOS ANSWER AS RECOMPUTATION WOULD (refineMemo,
// tokenize-compile-2026-09-23). The corpus cannot check this: a memo keyed too
// coarsely left every one of 472 compiles byte-identical, because no program
// reaches one loop twice under the same facts with different answers. So the
// witness is built here, where it can be: the same loop at several starting
// values under ONE fact set, each answer compared with a refiner that has no
// memo, and the starts chosen so the right answers differ.

func loopUnderTest(t *testing.T) *core.Term {
	t.Helper()
	nf := reduce(t, `(use go) (fn (n) (loop ((i 0)) (>= i n) i else (again (+ i 1))))`, "go")
	var found *core.Term
	var walk func(x *core.Term)
	walk = func(x *core.Term) {
		if x == nil || found != nil {
			return
		}
		if x.Kind == core.KApp && x.Op().Kind == core.KName && loopKinds[x.Op().Name] &&
			len(x.Args()) >= 2 && x.Args()[0].Kind == core.KFn {
			found = x
			return
		}
		if x.Kind == core.KFn {
			walk(x.Body())
			return
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(nf)
	if found == nil {
		t.Fatalf("no loop in %s", nf)
	}
	return found
}

func TestHoudiniMemoAnswersAsRecomputationWould(t *testing.T) {
	tg := goNative(t)
	lam := loopUnderTest(t).Args()[0]
	shared := &refiner{tgt: tg, pure: pureAtoms(tg), memo: &refineMemo{
		houdini: map[string][]*core.Term{}, summary: map[string]summaryMemo{}}}
	answers := map[int64]string{}
	// Repeats on purpose: the second 0 and −5 are the calls a memo answers.
	for _, z := range []int64{0, -5, 3, 0, -5, 3} {
		inits := []*core.Term{core.Int(z)}
		f := newFacts()
		gm, gf := f.clone(), f.clone()
		shared.loopInvariants(lam, inits, f, gm)
		fresh := &refiner{tgt: tg, pure: pureAtoms(tg)} // no memo: recomputes
		fresh.loopInvariants(lam, inits, f, gf)
		if gm.fingerprint() != gf.fingerprint() {
			t.Errorf("start %d: the memo answered\n  %s\nand recomputing gives\n  %s", z, gm.fingerprint(), gf.fingerprint())
		}
		answers[z] = gf.fingerprint()
	}
	// ANTI-VACUITY: 0 ≤ i is an invariant from 0 and not from −5, so a memo that
	// could not tell the starts apart would be caught above.
	if answers[0] == answers[-5] {
		t.Fatal("the starts 0 and −5 give the same invariants, so this test cannot see a key that ignores the start")
	}
}

// And a loop's result summary, which is stored over the name it was computed
// for: asked again for another name, it must come back about THAT name.
func TestSummaryMemoRenamesTheBoundName(t *testing.T) {
	tg := goNative(t)
	loop := loopUnderTest(t)
	shared := &refiner{tgt: tg, pure: pureAtoms(tg), memo: &refineMemo{
		houdini: map[string][]*core.Term{}, summary: map[string]summaryMemo{}}}
	var prints []string
	for _, x := range []string{"a", "b", "a"} {
		at := newFacts()
		intoM, intoF := at.clone(), at.clone()
		shared.summarizeNamed(intoM, x, loop, at)
		fresh := &refiner{tgt: tg, pure: pureAtoms(tg)}
		fresh.summarizeNamed(intoF, x, loop, at)
		if intoM.fingerprint() != intoF.fingerprint() {
			t.Errorf("result named %s: the memo answered\n  %s\nand recomputing gives\n  %s", x, intoM.fingerprint(), intoF.fingerprint())
		}
		prints = append(prints, intoF.fingerprint())
	}
	// ANTI-VACUITY: the summary says something, and says it about the name.
	if prints[0] == "" || prints[0] == prints[1] {
		t.Fatalf("the summary is empty or does not mention the bound name (%q, %q), so a missing rename would pass", prints[0], prints[1])
	}
}
