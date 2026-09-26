package ir

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// THE SHIFT REWRITE KEEPS EVERY ANSWER (ir/shift.go). x div 2ᵏ = x >> k and
// x rem 2ᵏ = x & (2ᵏ − 1) hold for x ≥ 0 and fail below it (−3 div 2 = −1,
// −3 >> 1 = −2; −3 rem 2 = −1, −3 & 1 = 1). Each generated program divides a
// value that is negative for some inputs and not for others, depending on its
// constants; its IR is run before and after the rewrite, and every input must
// give the same answer. The anti-vacuity guard asks that the rewrite fired.
func TestTheShiftRewriteKeepsEveryAnswer(t *testing.T) {
	dir := t.TempDir()
	r := rand.New(rand.NewSource(11))
	rewrote := 0
	for c := 0; c < 120; c++ {
		k := 1 + r.Intn(4)
		src := fmt.Sprintf(`(export run)
(sig run ((n (int 0 12))) int)
(def run (n)
  (loop ((i (- n %d)) (acc 0))
    (>= i %d) acc
    else (again (+ i 1) (+ acc (+ (* 100 (/ i %d)) (%% i %d))))))
`, r.Intn(2)*r.Intn(16), 8+r.Intn(8), 1<<k, 1<<(1+r.Intn(4)))
		path := filepath.Join(dir, fmt.Sprintf("s%d.oro", c))
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
		n := SelectShifts(tg, after)
		rewrote += n
		for x := int64(0); x <= 12; x++ {
			a, err1 := runFunc(before, []cval{intv(x)})
			b, err2 := runFunc(after, []cval{intv(x)})
			if err1 != nil || err2 != nil {
				t.Fatalf("run(%d): %v %v\n%s", x, err1, err2, src)
			}
			ra, rb := a[before.Body.Args[0]], b[after.Body.Args[0]]
			if ra == nil || rb == nil || ra.ints[len(ra.ints)-1].Cmp(rb.ints[len(rb.ints)-1]) != 0 {
				t.Fatalf("run(%d) changed under %d rewrite(s)\n%s", x, n, src)
			}
		}
	}
	if rewrote == 0 {
		t.Fatal("the rewrite never fired: the test checks nothing")
	}
	t.Logf("%d rewrites checked", rewrote)
}
