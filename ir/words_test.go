package ir

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// THE WORD SELECTION KEEPS EVERY ANSWER (ADR 0033, ir/words.go). The
// interpreter runs a language operation in ℤ and a u64 primitive by the
// residue map (interp_test.go), so the selected function must return what the
// unselected one returns on every input: a conversion that is not the identity
// on the values that occur, or an order or a division computed in a
// realization that does not hold both operands, gives a different answer.
// The programs cross 2⁶³ with sums, products, comparisons, division and
// remainder, digit loops over U, and results brought back into S.
func TestTheWordSelectionKeepsEveryAnswer(t *testing.T) {
	dir := t.TempDir()
	r := rand.New(rand.NewSource(17))
	shapes := []func() string{
		func() string {
			return fmt.Sprintf(`(let x (+ 9223372036854775000 (* n %d))
    (if (> x 9223372036854775%03d) (%% x %d) (/ x %d)))`, 50+r.Intn(200), r.Intn(807), 2+r.Intn(1000), 2+r.Intn(50))
		},
		func() string {
			return fmt.Sprintf(`(loop ((v (- 18446744073709551615 (* n %d))) (s 0))
    (= v 0) s
    else (again (/ v %d) (+ s (%% v %d))))`, 1+r.Intn(1000), 2+r.Intn(15), 2+r.Intn(15))
		},
		func() string {
			return fmt.Sprintf(`(let x (- 18446744073709551615 (* n %d))
       y (- 18446744073709551615 %d)
    (+ (if (< x y) 1 2) (/ x %d)))`, r.Intn(99999), r.Intn(999999), 1000000000+r.Intn(1000000000))
		},
		func() string {
			return fmt.Sprintf(`(let x (* (+ n %d) 1000000000000000000)
    (+ (%% x %d) (/ x 1000000000000000000)))`, 9+r.Intn(8), 2+r.Intn(100000))
		},
		// a remainder that stays in U∖S: x mod d = x for x < d, near 2⁶⁴
		func() string {
			return fmt.Sprintf(`(let x (- 18446744073709551614 (* n %d))
    (/ (%% x 18446744073709551615) %d))`, 1+r.Intn(1000), 3+r.Intn(5))
		},
	}
	selected := 0
	for c := 0; c < 150; c++ {
		src := fmt.Sprintf("(export run)\n(sig run ((n (int 0 12))) int)\n(def run (n)\n  %s)\n", shapes[c%len(shapes)]())
		path := filepath.Join(dir, fmt.Sprintf("w%d.oro", c))
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		tg, p, err := compilePath(path, "go", Options{Decided: true})
		if err != nil {
			t.Fatalf("%v\n%s", err, src)
		}
		before := p.Funcs[0]
		Decide(tg, before, false)
		after := before.Clone()
		after.facts = before.facts
		changed, err := SelectWords(tg, after)
		if err != nil {
			continue // refused: an obligation no fact discharges, which is a sound answer
		}
		if changed {
			selected++
		}
		// The representations must also FLOW: the verifier's rule (W5), on the
		// step to IR_P, judges what the interpreter cannot, a u64 operation
		// given an int.
		if err := ToP(tg, after.Clone()); err != nil {
			t.Fatalf("the selected function does not verify: %v\n%s", err, src)
		}
		for x := int64(0); x <= 12; x++ {
			ra, err1 := runResult(before, []cval{intv(x)})
			rb, err2 := runResult(after, []cval{intv(x)})
			if err1 != nil || err2 != nil {
				t.Fatalf("run(%d): %v / %v\n%s", x, err1, err2, src)
			}
			if ra.i == nil || rb.i == nil || ra.i.Cmp(rb.i) != 0 {
				t.Fatalf("run(%d) changed under the selection: %v, then %v\n%s", x, ra.i, rb.i, src)
			}
		}
	}
	if selected < 75 {
		t.Fatalf("the selection fired on %d of 150 programs: the test checks too little", selected)
	}
	t.Logf("%d programs selected and checked", selected)
}

// NO REALIZATION HOLDS BOTH: a value past 2⁶³ compared with one that may be
// negative is refused by name, not left to fail verification.
func TestAnOrderNoRealizationHoldsIsRefused(t *testing.T) {
	dir := t.TempDir()
	src := `(export run)
(sig run ((n (int 0 12))) int)
(def run (n)
  (let x (+ 9223372036854775000 (* n 100)) y (- n 6)
    (if (> x y) 1 0)))
`
	path := filepath.Join(dir, "r.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tg, p, err := compilePath(path, "go", Options{Decided: true})
	if err != nil {
		t.Fatal(err)
	}
	f := p.Funcs[0]
	Decide(tg, f, false)
	if _, err := SelectWords(tg, f); err == nil || !strings.Contains(err.Error(), "no realization holds both") {
		t.Errorf("a U value against a possibly negative one: %v", err)
	}
}
