package emit

import (
	"strings"
	"testing"
)

// THE FIRST ITERATION PEELED, AND THE GUARDS IT NEEDS (refinements.md §5d,
// claimsdemand-2026-10-13): a relation that holds only once a loop has gone
// round is proven by checking the first iteration under its initial values and
// the later ones from the first iteration's back edges; a failed `or` is the
// conjunction of the failed disjuncts; a failed `and` is a clause that a later
// guard resolves.

// peelHead is the imports, and the export the definition's header continues.
const peelHead = "use go\nexport "

func TestPeelingChecksTheFirstIteration(t *testing.T) {
	// ACCEPTED: d ≥ 1 once k ≥ 1, because the first iteration takes the k = 0
	// clause. Every iteration has only k = 0 ∨ d ≥ 1.
	for _, body := range []string{
		`fn f(t ∈ Array(Int), n ∈ 0..100) (Int) where len(t) == n {
    loop k = 0, d = 0, s = 0 {
        k >= n -> s
        k == 0 -> again(k + 1, d + 1, s)
        else -> again(k + 1, d, s + t[d - 1])
    }
}`,
		// The same through a failed `or`: ¬(k = 0 ∨ d = 7) gives k ≠ 0.
		`fn f(t ∈ Array(Int), n ∈ 0..100) (Int) where len(t) == n {
    loop k = 0, d = 0, s = 0 {
        k >= n -> s
        k == 0 || d == 7 -> again(k + 1, d + 1, s)
        else -> again(k + 1, d, s + t[d - 1])
    }
}`,
	} {
		if err := refineGo(t, peelHead+body); err != nil {
			t.Errorf("refused, and every read is in range on every iteration:\n%s\n%v", body, err)
		}
	}
	// REFUSED: the first iteration reads t[−1], and every later one t[0]. The
	// later iterations alone prove it, so the first must be checked.
	bad := `fn f(t ∈ Array(Int), n ∈ 0..100) (Int) where len(t) == n {
    loop k = 0, d = 0, s = 0 { k >= n -> s; else -> again(k + 1, 1, s + t[d - 1]) }
}`
	if err := refineGo(t, peelHead+bad); err == nil {
		t.Fatal("t[−1] on the first iteration was accepted")
	}
	// PLANTED: the first iteration's check skipped.
	plantPeelFault = true
	defer func() { plantPeelFault = false }()
	if err := refineGo(t, peelHead+bad); err != nil {
		t.Errorf("with the first iteration unchecked the program is still refused, so this test cannot see the fault: %v", err)
	}
}

// A FAILED CONJUNCTION RESOLVED: ¬(b ∧ j ≥ len t) and then b give j < len t.
func TestAFailedConjunctionIsResolvedByALaterGuard(t *testing.T) {
	head := "use go\nexport "
	ok := `fn f(t ∈ Array(Int), j ∈ Int, b ∈ Bool, c ∈ Bool) (Int) where 0 <= j {
    if b && j >= len(t) { 0 } else if b { t[j] } else { 0 }
}`
	if err := refineGo(t, head+ok); err != nil {
		t.Errorf("b after ¬(b ∧ j ≥ len t) did not give j < len t: %v", err)
	}
	// c is no literal of the clause, so nothing is resolved.
	bad := `fn f(t ∈ Array(Int), j ∈ Int, b ∈ Bool, c ∈ Bool) (Int) where 0 <= j {
    if b && j >= len(t) { 0 } else if c { t[j] } else { 0 }
}`
	if err := refineGo(t, head+bad); err == nil || !strings.Contains(err.Error(), "indexing") {
		t.Errorf("c, which is not b, resolved the clause: %v", err)
	}
}
