package core

import (
	"strings"
	"testing"
)

// THE MAP LITERAL AND ITS BETA-TAB CLAUSE (maps.md §3.1, §5).
//
// `(map (k v) …)` is a table given by its GRAPH, and because a map's index set
// is not implicit the graph carries both columns. Applying it is beta-tab with
// a SUM in the result position, which is what makes F2 — `(m k) : (option V)` —
// cost nothing at the static level: the constructor folds against the `case`
// and no map, tag, closure or allocation survives.
//
// These go through `norm`, which runs the real `Load`, because the injected
// `option` sum and `case`'s expansion both live there. A helper that built an
// Env by hand would exercise neither.
func mapNorm(t *testing.T, src string) string {
	t.Helper()
	const prims = "(prim map)\n(prim array)\n(prim len)\n(prim if)\n(prim =)\n(prim +)\n"
	return norm(t, prims+src, "")
}

func TestMapLiteralReduces(t *testing.T) {
	// The expected forms are the CHURCH ENCODING, because δ has already
	// unfolded the constructor by the time reduction stops: `some` is
	// `λp.λk. k 0 p` and `none` is `λk. k 1 0` (sums.md). Tag 0 is `some` and
	// tag 1 is `none`, in declaration order.
	//
	// Asserting on the encoded form rather than the surface spelling is the
	// more informative test: it shows there is no sum VALUE anywhere, only a
	// function that a `case` consumes and erases.
	for _, c := range []struct{ src, want string }{
		{"((map (1 10) (2 20) (5 50)) 2)", "(fn (#x) (#x 0 20))"},
		{"((map (1 10) (2 20) (5 50)) 9)", "(fn (#x) (#x 1 0))"},
		{"((map (1 10)) 1)", "(fn (#x) (#x 0 10))"},
		{"(len (map (1 10) (2 20) (5 50)))", "3"},
		{"(len (map))", "0"},
	} {
		if got := mapNorm(t, c.src); got != c.want {
			t.Errorf("%s reduced to %s, want %s", c.src, got, c.want)
		}
	}
}

// A STATIC MAP LEAVES NOTHING — the claim maps.md §5.2 makes, and the reason F2
// is affordable at all.
func TestAStaticMapLeavesNothing(t *testing.T) {
	got := mapNorm(t, "(case ((map (1 10) (2 20)) 2) (some v) (+ v 1) none 0)")
	// `(+ 20 1)` rather than `21`, because this Env has no constant folder —
	// that belongs to the target, and `cmd/oro` against a real one folds the
	// rest of the way. What matters here is what is GONE.
	if got != "(+ 20 1)" {
		t.Errorf("a static map read left %s, want (+ 20 1)", got)
	}
	for _, gone := range []string{"map", "#x", "case"} {
		if strings.Contains(got, gone) {
			t.Errorf("%q survived a static map read: %s. Free where it is used "+
				"to THINK is the whole argument for F2; if a map, a tag or a "+
				"dispatch survives, the option is a runtime cost after all", gone, got)
		}
	}
}

// ABSENCE IS A RESULT, NOT A STUCK TERM, and that is where a map differs from
// an array by design rather than by accident.
//
// An array's out-of-range index is deliberately LEFT ALONE so the refinement
// layer can report it with the bound and the call site — its domain condition
// is discharged statically, in QF-LIA. A map's is discharged by the PROGRAM,
// because `k ∈ dom m` is set membership and nothing decides it (maps.md §1.1).
// So `none` is the answer here and a diagnostic there.
func TestAbsenceIsAResultAndOutOfRangeIsNot(t *testing.T) {
	if got := mapNorm(t, "((map (1 10)) 7)"); got != "(fn (#x) (#x 1 0))" {
		t.Errorf("a missing key gave %s, want none's encoding (tag 1)", got)
	}
	// The control, and it must NOT fold. If this ever starts returning a sum,
	// the two constructs have been conflated and the refinement layer has lost
	// a diagnostic that only it can give.
	got := mapNorm(t, "((array 1 2 3) 7)")
	if strings.Contains(got, "#x") || !strings.Contains(got, "array") {
		t.Errorf("an out-of-range ARRAY read gave %s; it must stay stuck so the "+
			"refinement layer can report it with the bound and the call site", got)
	}
}

// A DYNAMIC KEY LEAVES THE MAP ALONE. Reduction cannot decide `k ∈ dom m` when
// k is unknown, so the term is stuck and the map reaches the boundary — which
// is exactly the case a target has to implement, and the reason indexing being
// application costs nothing: `(m k)` is the same text either way.
func TestADynamicKeyDoesNotReduce(t *testing.T) {
	got := mapNorm(t, "(fn (k) ((map (1 10) (2 20)) k))")
	if strings.Contains(got, "#x") {
		t.Fatalf("a dynamic key decided the domain condition: %s", got)
	}
	if !strings.Contains(got, "map (1 10) (2 20)") {
		t.Errorf("the map literal did not survive a dynamic key: %s", got)
	}
}

// A NON-LITERAL KEY IN THE GRAPH also stops it, and for the same reason: the
// clause decides membership by inspecting integers, so a key it cannot inspect
// makes the whole lookup undecidable rather than absent. Answering `none` here
// would be a silent wrong answer, because `(j 10)` might BE the row for 2.
func TestANonLiteralKeyInTheGraphDoesNotReduce(t *testing.T) {
	got := mapNorm(t, "(fn (j) ((map (j 10) (2 20)) 2))")
	if strings.Contains(got, "#x") {
		t.Errorf("a graph with an unknown key was decided anyway: %s", got)
	}
}

// The language's `option` is INJECTED into every module, so a program that
// declares no sum can still be handed one by the compiler — a map read produces
// `some`/`none` and they have to resolve, the same way `if` has to (maps.md §4).
func TestOptionIsAvailableWithoutBeingDeclared(t *testing.T) {
	if got := mapNorm(t, "(case (some 3) (some v) v none 0)"); got != "3" {
		t.Errorf("case on an injected `some` gave %s, want 3", got)
	}
	if got := mapNorm(t, "(case none (some v) v none 7)"); got != "7" {
		t.Errorf("case on an injected `none` gave %s, want 7", got)
	}
}

// A `case` IN A BARE TERM EXPANDS, which it did not until the map work needed
// it. `expandCase` ran over every module's DEFS and not over the top-level
// entry terms, so the identical `case` reduced inside a `(def …)` and stayed a
// stuck application at top level.
//
// Every real program puts its code in a def, which is why this was never seen —
// and a construct that works in one position and silently does not in another
// is the shape of bug this repository keeps finding.
func TestCaseExpandsInABareTerm(t *testing.T) {
	if got := mapNorm(t, "(case (some 5) (some v) v none 0)"); strings.Contains(got, "case") {
		t.Errorf("a top-level `case` did not expand: %s", got)
	}
}
