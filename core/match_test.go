package core

import (
	"strings"
	"testing"
)

// docs/spec/match.md. `match` IS `loop`: it desugars to one, adds ZERO
// reduction rules and ZERO term kinds, and joins `let`, `seq`, `and`, `or`,
// `not`, `cond` as sugar that erases in the reader.
//
// These tests pin the desugaring itself rather than any emitted code, because
// the desugaring is the whole implementation — if it is right, every backend
// gets it for free from `loop`, which four targets already emit at parity.

func readOne(t *testing.T, src string) string {
	t.Helper()
	forms, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range forms {
		if f.Kind == "def" {
			return f.Term.String()
		}
	}
	t.Fatal("no def")
	return ""
}

func mustFail(t *testing.T, src, want string) {
	t.Helper()
	_, err := Read(src)
	if err == nil {
		t.Fatalf("expected a refusal mentioning %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected %q, got: %v", want, err)
	}
}

// The desugaring, whole. A tag pattern becomes `tag=`, a name pattern becomes a
// RENAME (not a `let`), a wildcard becomes nothing at all, and the clause list
// becomes the `loop`'s clause list unchanged.
//
// Note `s` and `i` as the LOOP's variables, shadowing the function's: see
// TestScrutineeNameBecomesTheLoopVariable for why that is load-bearing.
func TestMatchIsLoop(t *testing.T) {
	got := readOne(t, `
		(use go)
		(def classify (fn (s i)
			(match (s i)
				0 0    (again 1 (go.+ i 1))
				0 _    (again 2 (go.+ i 1))
				1 k    k
				else   (go.- 0 1))))`)
	want := `(fn (s i) (loop (fn (s i) ` +
		`(if (if (tag= s 0) (tag= i 0) false) (again 1 (go.+ i 1)) ` +
		`(if (tag= s 0) (again 2 (go.+ i 1)) ` +
		`(if (tag= s 1) i (go.- 0 1))))) s i))`
	if got != want {
		t.Errorf("desugaring changed:\n got %s\nwant %s", got, want)
	}
}

// `when` is not decoration, and building `match` is what showed it. ADR 0015
// forbids `again` under an `if`, so a condition that guards a transition CANNOT
// live in the clause body — without `when`, `match` is strictly weaker than the
// `loop` it desugars to, which would make it a worse spelling of the same thing.
func TestWhenGuardsAClause(t *testing.T) {
	got := readOne(t, `
		(use go)
		(def run (fn (s n)
			(match (s 0)
				_ k (when (go.>= k n))  k
				0 k                     (again 1 (go.+ k 1))
				else                    0)))`)
	if !strings.Contains(got, "(if (go.>= #m1 n) #m1") {
		t.Errorf("a `when` on an all-wildcard clause IS the guard:\n%s", got)
	}
	if !strings.Contains(got, "(if (tag= s 0) (again 1") {
		t.Errorf("later clauses must survive the guard:\n%s", got)
	}
}

// A `when` on a clause that also tests must be the CONJUNCTION of the two, and
// spelled the way `and` desugars so nothing new reaches the reducer.
func TestWhenConjoinsWithPatterns(t *testing.T) {
	got := readOne(t, `
		(use go)
		(def f (fn (s i)
			(match (s i)
				1 k (when (go.> k 0))  k
				else                   0)))`)
	if !strings.Contains(got, "(if (tag= s 1) (go.> i 0) false)") {
		t.Errorf("pattern AND guard, spelled as `and` desugars:\n%s", got)
	}
}

// A name pattern is a RENAME, not a `let`. That is what lets a `(when …)` see
// it — a `let` wrapping only the body could not — and it is why no binder is
// introduced for something that is just another name for a loop variable.
func TestNamePatternRenamesRatherThanBinds(t *testing.T) {
	got := readOne(t, `(use go) (def f (fn (a) (match (a) x (go.+ x x) else 0)))`)
	if strings.Contains(got, "let") {
		t.Errorf("a name pattern must not introduce a binder:\n%s", got)
	}
	if !strings.Contains(got, "(go.+ a a)") {
		t.Errorf("every occurrence must be renamed:\n%s", got)
	}
}

// The rename is safe without capture analysis, and this is the reason: by the
// time readMatch runs, every inner `fn` has been through `Fn`, which closed its
// body — so an occurrence bound by a nested binder is a KBound, not a KName.
// A shadowing lambda must therefore be left completely alone.
func TestRenameDoesNotEnterAShadowingBinder(t *testing.T) {
	got := readOne(t, `(use go) (def f (fn (a) (match (a) x ((fn (x) x) x) else 0)))`)
	// The outer occurrence is renamed; the lambda's own body is untouched.
	if !strings.Contains(got, "(fn (x) x) a") {
		t.Errorf("a nested binder's occurrences must not be renamed:\n%s", got)
	}
}

// A bool pattern needs no equality at all: the scrutinee IS the test. `if` is
// the eliminator of `bool` (ADR 0017), so `true`/`false` patterns cost nothing.
func TestBoolPatternIsTheScrutinee(t *testing.T) {
	got := readOne(t, `(use go) (def f (fn (b) (match (b) true 1 else 0)))`)
	if strings.Contains(got, "tag=") {
		t.Errorf("a bool pattern needs no equality:\n%s", got)
	}
	if !strings.Contains(got, "(loop (fn (b) (if b 1 0)) b)") {
		t.Errorf("the scrutinee is the test:\n%s", got)
	}
}

// `again` in a clause body is the LOOP's `again` — this is the state machine
// working, and it is the reason `match` is shaped like `loop` rather than like
// ML's `case`: a parser, an event loop and a protocol handler are all
// (state, input) → (state', input') and want the jump, not a tail call.
func TestAgainInAClauseBodyIsAJump(t *testing.T) {
	got := readOne(t, `
		(use go)
		(def f (fn (s i) (match (s i) 0 k (again 1 k) else i)))`)
	if !strings.Contains(got, "(again 1 i)") {
		t.Errorf("again must survive into the loop:\n%s", got)
	}
}

// --- what match REFUSES -----------------------------------------------------

// ADR 0015's rule reaches through the sugar: the clause list is the loop's
// whole control flow, so `again` may not hide under an `if` in a body. This is
// the failure that produced `when`, and it must keep failing.
func TestAgainUnderAnIfIsStillRefused(t *testing.T) {
	mustFail(t, `
		(use go)
		(def f (fn (s n) (match (s 0) _ k (if (go.>= k n) k (again s k)) else 0)))`,
		"not under an `if`")
}

// Float and string patterns are absent because the language has no portable
// equality, and a float pattern would inherit IEEE's NaN — which is not the
// equivalence relation a pattern needs.
func TestFloatPatternIsRefused(t *testing.T) {
	mustFail(t, `(use go) (def f (fn (a) (match (a) 1.5 1 else 0)))`, "not a pattern")
}

func TestStringPatternIsRefused(t *testing.T) {
	mustFail(t, `(use go) (def f (fn (a) (match (a) "x" 1 else 0)))`, "not a pattern")
}

// Arity is checked: n scrutinees means n patterns per clause, so a clause that
// is short is a mistake rather than an implicit wildcard.
func TestClauseArityMustMatch(t *testing.T) {
	mustFail(t, `(use go) (def f (fn (a b) (match (a b) 0 1 else 0)))`,
		"pattern(s), an optional (when …), and a body")
}

// A repeated name in one clause would be an equality test, which patterns do
// not do — Erlang allows it, ML does not, and we do not, because it would smuggle
// an equality in that the language has no portable spelling for.
func TestRepeatedBindingIsRefused(t *testing.T) {
	mustFail(t, `(use go) (def f (fn (a b) (match (a b) x x 1 else 0)))`, "bound twice")
}

// `else` last, or the clauses after it could never run.
func TestElseMustBeLast(t *testing.T) {
	mustFail(t, `(use go) (def f (fn (a) (match (a) else 0 1 2)))`, "must be the last clause")
}

// A list of scrutinees, so that clauses need no tuple built and taken apart —
// this is the same reason `loop` has n variables and no product (ADR 0015).
func TestScrutineesAreAList(t *testing.T) {
	mustFail(t, `(use go) (def f (fn (a) (match a 0 1 else 0)))`, "LIST of scrutinees")
}

// A scrutinee that is a bare NAME becomes the loop variable under that same
// name, and this is the subtlest thing in the desugaring.
//
// `(match (s i) …)` initialises the loop from `s` and `i`. With FRESH loop
// variables, a clause body reading `i` would see the outer `i` — the value the
// loop STARTED from — while `again` advanced a hidden `#m1`. Every iteration
// after the first would read a stale value and the program would look right.
// Reusing the name shadows the outer binding, which is also what the
// state-machine reading means: `s` and `i` ARE the state.
func TestScrutineeNameBecomesTheLoopVariable(t *testing.T) {
	got := readOne(t, `
		(use go)
		(def f (fn (s i)
			(match (s i)
				0 _ (again 1 (go.+ i 1))
				else i)))`)
	if !strings.Contains(got, "(loop (fn (s i)") {
		t.Errorf("bare-name scrutinees must become the loop variables:\n%s", got)
	}
	// The `i` inside is the loop's, so `again` advances what the body reads.
	if !strings.Contains(got, "(again 1 (go.+ i 1))") {
		t.Errorf("a body reading `i` must read the LOOP's `i`:\n%s", got)
	}
}

// A scrutinee that is not a name has no name to reuse, so it gets a fresh one.
func TestComputedScrutineeGetsAFreshName(t *testing.T) {
	got := readOne(t, `(use go) (def f (fn (a b) (match ((go.+ a b)) 0 1 else 2)))`)
	if !strings.Contains(got, "(loop (fn (#m0) (if (tag= #m0 0) 1 2)) (go.+ a b))") {
		t.Errorf("a computed scrutinee needs a fresh variable:\n%s", got)
	}
}

// And a name repeated in the scrutinee list can only be one loop variable, so
// the second occurrence gets a fresh one rather than silently aliasing.
func TestRepeatedScrutineeNameIsNotAliased(t *testing.T) {
	got := readOne(t, `(use go) (def f (fn (a) (match (a a) 0 1 7 else 3)))`)
	if !strings.Contains(got, "(loop (fn (a #m1)") {
		t.Errorf("the second `a` cannot be the same loop variable:\n%s", got)
	}
}
