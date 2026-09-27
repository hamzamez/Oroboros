package ir

import (
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// limbValue is ι(o) = Σ oᵢ·Bⁱ with B = 2²⁴: the integer a table of limbs presents.
func limbValue(arr []*big.Int) *big.Int {
	out := new(big.Int)
	for i := len(arr) - 1; i >= 0; i-- {
		out.Lsh(out, 24)
		out.Add(out, arr[i])
	}
	return out
}

// THE FIXED-LIMB RUNG KEEPS EVERY ANSWER (ir/limbs.go, ADR 0035). Each program
// runs on the interpreter twice: unselected, where every operation is ℤ's, and
// on the limb rung, where every operation is an inlined instance of the limb
// library. ι of the limb result must be the integer, on every input, and the
// lowered function must be decided with every word operation proven and verify
// at IR_P. The shapes use every instance: the one-pass multiply and divide by a
// word and their full-width counterparts, a widening that stays limbs, a
// subtraction that keeps a magnitude, the comparisons, and a word result.
func TestTheLimbRungKeepsEveryAnswer(t *testing.T) {
	dir := t.TempDir()
	r := rand.New(rand.NewSource(41))
	k := func(lo, n int) int { return lo + r.Intn(n) }
	big := func() string { return fmt.Sprintf("%d%018d%06d", k(1, 9), r.Int63n(1e18), r.Intn(1e6)) }
	const set = "(int 0 (pow 2 400))"
	shapes := []func() string{
		func() string { // factorial: mul-small
			return fmt.Sprintf(`(sig run ((n (int 0 12))) %s)
(def run (n) (loop ((acc %d) (i 1)) (> i (* n 4)) acc else (again (* acc (+ i %d)) (+ i 1))))`, set, k(1, 9), k(0, 9))
		},
		func() string { // Fibonacci: add
			return fmt.Sprintf(`(sig run ((n (int 0 12))) %s)
(def run (n) (loop ((a %d) (b %d) (i 0)) (>= i (* n 9)) a else (again b (+ a b) (+ i 1))))`, set, k(0, 3), k(1, 3))
		},
		func() string { // the pair: mul of two bignums
			return fmt.Sprintf(`(sig run ((n (int 0 12))) %s)
(def run (n) (loop ((a %d) (b %d) (i 0)) (>= i (/ n 2)) b else (again (* a b) (+ b a) (+ i 1))))`, set, k(2, 4), k(1, 4))
		},
		func() string { // a product by a word past 2²⁸: mul, with the word widened into limbs
			return fmt.Sprintf(`(sig run ((n (int 0 12))) %s)
(def run (n) (loop ((acc %d) (i 0)) (>= i n) acc else (again (* acc %d) (+ i 1))))`, set, k(1, 9), 1<<29+k(0, 1000))
		},
		func() string { // subtraction keeping a magnitude, and a comparison
			return fmt.Sprintf(`(sig run ((n (int 0 12))) %s)
(def run (n)
  (loop ((a (* (+ n 3) %s)) (i 0))
    (>= i 5) a
    (> a %s) (again (- a (* n %d)) (+ i 1))
    else (again (+ a %d) (+ i 1))))`, set, big(), big(), k(1, 1000), k(1, 1000))
		},
		func() string { // digits: div-small and rem-small, a word result
			return fmt.Sprintf(`(sig big ((n (int 0 12))) %s)
(def big (n) (loop ((acc %d) (i 1)) (> i (* n 3)) acc else (again (* acc (+ i 7)) (+ i 1))))
(sig run ((n (int 0 12))) int)
(def run (n)
  (loop ((v (big n)) (s 0) (i 0))
    (or (= v 0) (>= i 120)) s
    else (again (/ v %d) (+ s (%% v %d)) (+ i 1))))`, set, k(1, 9), k(2, 1000), k(2, 1000))
		},
		func() string { // comparisons of two bignums, and a widened word kept as limbs
			return fmt.Sprintf(`(sig run ((n (int 0 12))) %s)
(def run (n)
  (let a (* (+ n 1) %s) b (* (- 13 n) %s)
    (if (< a b) (+ b n) (if (= a b) a (- a n)))))`, set, big(), big())
		},
	}
	tried, lowered := 0, 0
	for _, target := range []string{"go", "windows"} {
		for c := 0; c < 42; c++ {
			src := "(export run)\n" + shapes[c%len(shapes)]() + "\n"
			path := filepath.Join(dir, fmt.Sprintf("%s%d.oro", target, c))
			if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			tg, units, err := lowerUnselectedAs(path, target, "limbs")
			if err != nil {
				t.Fatalf("%v\n%s", err, src)
			}
			u := units[len(units)-1]
			tried++
			before := u.f
			after := before.Clone()
			after.facts = before.facts
			if _, err := SelectRung(tg, after, u.sig, u.plan); err != nil {
				t.Fatalf("%s: refused: %v\n%s", target, err, src)
			}
			if leg := Decide(tg, after, false); leg.Proven < leg.Ops {
				t.Fatalf("%s: a word operation is unproven: %v\n%s", target, leg.Unproven, src)
			}
			if err := ToP(tg, after.Clone()); err != nil {
				t.Fatalf("%s: the lowered function does not verify: %v\n%s", target, err, src)
			}
			if strings.Contains(printF(after), "big") {
				t.Fatalf("%s: a bignum operation is left on the limb rung\n%s\n%s", target, src, printF(after))
			}
			lowered++
			for x := int64(0); x <= 12; x++ {
				ra, err1 := runResult(before, []cval{intv(x)})
				rb, err2 := runResult(after, []cval{intv(x)})
				if err1 != nil || err2 != nil {
					t.Fatalf("%s run(%d): %v / %v\n%s", target, x, err1, err2, src)
				}
				got := rb.i
				if rb.arr != nil {
					got = limbValue(rb.arr)
				}
				if ra.i == nil || got == nil || ra.i.Cmp(got) != 0 {
					t.Fatalf("%s run(%d) changed on the limb rung: %v, then %v\n%s", target, x, ra.i, got, src)
				}
			}
		}
	}
	t.Logf("%d programs lowered to limbs and checked", lowered)
}

// THE ONE SET IS ENFORCED ON LIMBS: a sum past 2^71 traps, as big-fit does on
// the host's bignum (TestTheDeclaredBoundTrapsOnTheHostsBignum).
func TestTheDeclaredBoundTrapsOnLimbs(t *testing.T) {
	dir := t.TempDir()
	src := `(export run)
(sig run ((n (int 0 12))) (int 0 (pow 2 70)))
(def run (n) (let a (* (+ n 1) 2000000000000000000000) (+ a a)))
`
	path := filepath.Join(dir, "fit.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tg, units, err := lowerUnselectedAs(path, "windows", "limbs")
	if err != nil {
		t.Fatal(err)
	}
	u := units[0]
	if _, err := SelectRung(tg, u.f, u.sig, u.plan); err != nil {
		t.Fatal(err)
	}
	if _, err := runResult(u.f, []cval{intv(0)}); err == nil || !strings.Contains(err.Error(), "trap") {
		t.Errorf("4·10²¹ under a declared 2^70 on limbs: %v", err)
	}
}

// LIMBS HOLD A MAGNITUDE: a widened value that may be negative is refused by
// name, not split into limbs it has none of.
func TestANegativeWideningIsRefusedOnLimbs(t *testing.T) {
	dir := t.TempDir()
	src := `(export run)
(sig run ((n (int 0 12))) (int 0 (pow 2 300)))
(def run (n) (+ 5000000000000000000000000 (- n 6)))
`
	path := filepath.Join(dir, "neg.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tg, units, err := lowerUnselectedAs(path, "windows", "limbs")
	if err != nil {
		t.Fatal(err)
	}
	u := units[0]
	if _, err := SelectRung(tg, u.f, u.sig, u.plan); err == nil || !strings.Contains(err.Error(), "magnitude") {
		t.Errorf("a possibly negative widening on limbs: %v", err)
	}
}

// AN INSTANCE ENDING IN A BRANCH is inlined as an `if` whose arms are the
// branch's (inlineInto). No limb instance ends in one today, so the path is
// witnessed on a hand-built callee, max(a, b), whose arms carry a π.
func TestAnInlinedBranchIsAnIf(t *testing.T) {
	p, err := Read(`(ir 1
  (target go)
  (stage A)
  (ops lt yield branch)
  (func m (params (%0 int) (%1 int)) (results int)
    (region
      (val (%2 bool) (lt %0 %1))
      (branch %2
        (region
          (pi %3 int (%0 lt %1))
          (yield %1))
        (region
          (yield %0))))))
`)
	if err != nil {
		t.Fatal(err)
	}
	callee := p.Funcs[0]
	caller := &Func{Name: "t", Params: []V{0, 1}, Results: []string{"int"},
		Types: []string{"int", "int", "int"}, Body: &Region{T: TYield, Args: []V{2}}}
	ren := map[V]V{}
	caller.Body.Stmts = inlineInto(caller, nil, callee, []V{1, 0}, []V{2}, ren)
	renameAway(caller, ren)
	for _, c := range [][2]int64{{3, 9}, {9, 3}, {5, 5}, {-4, 2}} {
		got, err := runResult(caller, []cval{intv(c[0]), intv(c[1])})
		want := c[0]
		if c[1] > want {
			want = c[1]
		}
		if err != nil || got.i.Int64() != want {
			t.Errorf("max(%d, %d) inlined: %v (%v)\n%s", c[1], c[0], got.i, err, printF(caller))
		}
	}
}
