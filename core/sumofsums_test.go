package core

import (
	"strings"
	"testing"
)

// A SUM OF SUMS REDUCES TO PLAIN CONTROL FLOW (sums.md §5, sumofsums-2026-10-06).
//
// sums.md's nested test nests sums IN SEQUENCE, each payload an integer: a sum,
// eliminated, producing a sum. A sum INSIDE another sum's payload, A + (K₁ + K₂),
// is the shape an error model needs, `(err not-found)`, and it stopped
// reducing for two reasons, one per kind of arm:
//
//   - PURE ARMS: the eliminator names its payload in every arm, so β counted two
//     occurrences and let-bound the conditional payload; folding the known tag
//     then removed one arm, and the binding was left in the way. The recount
//     substitutes a pure binding whose variable now occurs once.
//   - ARMS WITH EFFECTS: a constructor is `(fn (#x) (#x tag payload))`, and its
//     `(#x …)` read as a table read through a bound variable, which may not move
//     into a body with effects. A read under a λ runs where the λ is applied, so
//     the check now stops at a λ.
//
// A + (K₁ + K₂) ≅ A + K₁ + K₂ by associativity of the coproduct, so the residual
// is the flat sum's: one test per summand and no tag.

func reduceWith(t *testing.T, src, export string, prims ...string) string {
	t.Helper()
	p, err := loadSrc(t, src)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := Normalize(p.Defs[export], testEnv(p, prims...), DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	return nf.String()
}

const sumOfSums = `
	(use go)
	(variant (result T E) (ok T) (err E))
	(variant kind small big)
	(def g (fn (n) (if (go.> 5 n) (ok n) (err (if (go.> 10 n) small big)))))`

func TestASumOfSumsReducesWithPureArms(t *testing.T) {
	got := reduceWith(t, sumOfSums+`
		(def f (fn (n) (case (g n) (ok v) v (err k) (case k small 1000 big 2000))))`,
		"f", "go.>", "if", "=")
	want := "(fn (n) (if (go.> 5 n) n (if (go.> 10 n) 1000 2000)))"
	if got != want {
		t.Errorf("a sum of sums must reduce as the flat sum does:\n got  %s\n want %s", got, want)
	}
}

func TestASumOfSumsReducesWithArmsThatHaveEffects(t *testing.T) {
	got := reduceWith(t, sumOfSums+`
		(def f (fn (n) (case (g n) (ok v) (go.print v) (err k) (case k small (go.print 1) big (go.print 2)))))`,
		"f", "go.>", "if", "=", "!go.print")
	want := "(fn (n) (if (go.> 5 n) (go.print n) (if (go.> 10 n) (go.print 1) (go.print 2))))"
	if got != want {
		t.Errorf("a sum of sums must reduce when its arms have effects:\n got  %s\n want %s", got, want)
	}
}

// THE RECOUNT MOVES ONLY A PURE VALUE. An impure payload is bound at the
// application, where the program wrote it (ADR 0010), and it stays there even
// when its variable occurs once: substituting it would move the effect into an
// arm, after whatever the arm's test runs. So this sum stays stuck, which the
// emitter reports, and the impure call stays in binding position.
func TestTheRecountDoesNotMoveAnImpureValue(t *testing.T) {
	got := reduceWith(t, `
		(use go)
		(variant (result T E) (ok T) (err E))
		(variant kind small big)
		(def g (fn (n) (if (go.> 5 n) (ok n) (err (if (go.> (go.roll) n) small big)))))
		(def f (fn (n) (case (g n) (ok v) v (err k) (case k small 1000 big 2000))))`,
		"f", "go.>", "if", "=", "!go.roll")
	if !strings.Contains(got, "(let (if (go.> (go.roll) n)") {
		t.Errorf("the impure call must stay bound where the program wrote it:\n%s", got)
	}
	if strings.Count(got, "go.roll") != 1 {
		t.Errorf("the impure call must run once:\n%s", got)
	}
}
