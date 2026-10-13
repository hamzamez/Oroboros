package emit

import (
	"testing"

	"oroboros/core"
)

// TRANSLATION INVARIANTS (translation.go): a merge advances i or j and always k,
// so i + j − k keeps its initial value, mid, which the merge's reads need
// (claimsdemand-2026-10-13). The candidates come from a null space and are only
// candidates: a wrong vector, planted, must be dropped by Houdini's check.
func TestTranslationInvariants(t *testing.T) {
	tg := goNative(t)
	loop := findLoop(t, `(use go) (fn (lo mid hi)
  (loop ((i lo) (j mid) (k lo))
    (>= k hi)  k
    (>= i mid) (again i (+ j 1) (+ k 1))
    else       (again (+ i 1) j (+ k 1))))`)
	lam, inits := loop.Args()[0], loop.Args()[1:]
	holds := func(g *facts, src string) bool {
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		gs, ok := g.oblig(core.Rename2(forms[0].Term, map[string]*core.Term{
			"i": core.Name(lam.Params[0]), "j": core.Name(lam.Params[1]), "k": core.Name(lam.Params[2])}))
		return ok && allEntailed(g, gs)
	}
	run := func() *facts {
		r := &refiner{tgt: tg, pure: pureAtoms(tg)}
		f := newFacts()
		g := f.clone()
		r.loopInvariants(lam, inits, f, g)
		return g
	}
	if c := translationCandidates(lam, inits); len(c) != 1 {
		t.Fatalf("the merge's steps (0,1,1) and (1,0,1) span a plane, so one vector is orthogonal to both; got %d", len(c))
	}
	g := run()
	if !holds(g, "(<= (+ i j) (+ k mid))") || !holds(g, "(<= (+ k mid) (+ i j))") {
		t.Fatalf("i + j − k = mid was not derived: %s", g.fingerprint())
	}
	// PLANTED: the candidate generator gives i + j + k, which no back edge keeps.
	// Houdini must drop it: assumed, it would give i + j + k = lo + mid + lo for
	// every iteration, false after the first.
	plantTranslationFault = true
	defer func() { plantTranslationFault = false }()
	if g := run(); holds(g, "(<= (+ (+ i j) k) (+ (+ lo mid) lo))") && holds(g, "(<= (+ (+ lo mid) lo) (+ (+ i j) k))") {
		t.Error("a candidate no back edge preserves was assumed: Houdini's check did not run on it")
	}
}

// findLoop is the first loop in src's normal form.
func findLoop(t *testing.T, src string) *core.Term {
	t.Helper()
	nf := reduce(t, src, "go")
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
