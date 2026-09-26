package ir

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/emit"
)

// THE LOOP RULES, CHECKED AGAINST EXECUTIONS. Each rule the interval domain
// applies at a loop — the threshold widening, the arm conditions, the trip
// bounds of Theorems 1 and 2, the steps `delta` reads, and Theorem 3's
// bounded increments into a buffer — concludes a fact about every iteration.
// Local soundness tables cannot check that; an execution can. Every value a
// run defines, and every cell of every buffer when it is defined, must lie in
// its fact (interp_test.go).

// TestTreeRunIsContained: tree.oro's run, the function whose walk needed
// Theorem 3 on two buffers and the repeated refinement, run on its own input.
func TestTreeRunIsContained(t *testing.T) {
	tg, p := compile(t, "examples/json/tree.oro", "go")
	ran := 0
	for _, f := range p.Funcs {
		if len(f.Params) != 1 || f.Types[f.Params[0]] != "int" {
			continue
		}
		fs := analyse(tg, f)
		for _, n := range []int64{0, 1, 2} {
			seen, err := runFunc(f, []cval{intv(n)})
			if err != nil {
				t.Fatalf("%s(%d): %v", f.Name, n, err)
			}
			if err := contained(f, fs, seen); err != nil {
				t.Fatalf("%s(%d): %v", f.Name, n, err)
			}
			ran++
		}
	}
	if ran == 0 {
		t.Fatal("no function of tree.oro ran")
	}
	// And the facts are PRECISE enough: every operation proven, as the term
	// analysis proves them. The walk inlines the tokeniser, whose hundreds of
	// constants made the threshold widening climb one per round until the
	// budget ended and the loop took ⊤ (thresholdRounds).
	for _, f := range p.Funcs {
		if f.Name == "t-run" {
			if leg := Decide(tg, f.Clone(), false); leg.Proven != leg.Ops {
				t.Fatalf("t-run: %d of %d proven; first unproven: %s", leg.Proven, leg.Ops, leg.Unproven[0])
			}
		}
	}
}

// loopShapes are the templates the generator draws from, one per rule. Each
// takes random constants; `n` is the function's parameter.
var loopShapes = []func(r *rand.Rand) string{
	// Theorem 2, increasing, with an arm-conditioned accumulator and a
	// running maximum (the thresholds' case).
	func(r *rand.Rand) string {
		return fmt.Sprintf(`(loop ((i %d) (acc %d) (m 0))
    (>= i (+ n %d)) (+ acc m)
    else (again (+ i %d) (if (< i %d) (+ acc (* i %d)) (- acc %d)) (if (> i m) i m)))`,
			r.Intn(5)-2, r.Intn(7)-3, r.Intn(9), 1+r.Intn(3), r.Intn(12), r.Intn(7)-3, r.Intn(5))
	},
	// Theorem 2, decreasing.
	func(r *rand.Rand) string {
		return fmt.Sprintf(`(loop ((i (+ n %d)) (acc %d) (k 0))
    (< i %d) (+ acc k)
    else (again (- i %d) (+ acc (* i %d)) (+ k 1)))`,
			r.Intn(9), r.Intn(5), r.Intn(5)-2, 1+r.Intn(3), r.Intn(7)-3)
	},
	// Theorem 2, geometric.
	func(r *rand.Rand) string {
		return fmt.Sprintf(`(loop ((x (+ n %d)) (acc 0) (k 0))
    (< x 1) (+ acc k)
    else (again (/ x %d) (+ acc (* x %d)) (+ k 1)))`,
			1+r.Intn(40), 2+r.Intn(3), r.Intn(5)-2)
	},
	// Theorem 3: increments into a buffer, read from another cell, with a
	// fresh store on one arm.
	func(r *rand.Rand) string {
		l := 1 + r.Intn(5)
		return fmt.Sprintf(`(let t (build b %d
          (loop ((b b) (i 0))
            (>= i (+ n %d)) b
            (< i %d) (again (set b (%% i %d) %d) (+ i 1))
            else (again (set b (%% i %d) (+ (b (%% (+ i %d) %d)) %d)) (+ i 1))))
    (+ (t 0) (t %d)))`,
			l, r.Intn(9), r.Intn(4), l, r.Intn(11)-5, l, r.Intn(4), l, r.Intn(7)-3, l-1)
	},
	// Theorem 3 on two buffers: a copy between them, and an increment.
	func(r *rand.Rand) string {
		l := 1 + r.Intn(4)
		return fmt.Sprintf(`(let t (build a %d  c %d
          (loop ((a a) (c c) (i 0))
            (>= i (+ n %d)) a
            else (again (set a (%% i %d) (+ (c (%% i %d)) %d)) (set c (%% (+ i 1) %d) (a (%% i %d))) (+ i 1))))
    (t %d))`,
			l, l, r.Intn(9), l, l, r.Intn(5)-1, l, l, l-1)
	},
	// delta's inner-loop rule: the outer step is an inner loop's result.
	func(r *rand.Rand) string {
		m := 2 + r.Intn(3)
		return fmt.Sprintf(`(loop ((i 0) (acc 0))
    (>= i (+ n %d)) acc
    else (let j (loop ((j i))
                  (>= j (+ n %d)) j
                  (= (%% j %d) 0) j
                  else (again (+ j 1)))
           (again (+ j %d) (+ acc j))))`,
			r.Intn(9), r.Intn(9), m, 1+r.Intn(2))
	},
	// delta's inner-loop rule where it is TIGHT: q moves by the inner loop's
	// result minus a literal, so Theorem 1's lower end is reached exactly when
	// the inner loop breaks at once; and an inner loop that DESCENDS, which
	// the rule must refuse (its parameter is not non-decreasing).
	func(r *rand.Rand) string {
		up := r.Intn(2) == 0
		inner := fmt.Sprintf(`(loop ((j q))
                  (>= j (+ q %d)) j
                  (= (%% j 3) 0) j
                  else (again (+ j 1)))`, r.Intn(3))
		if !up {
			inner = fmt.Sprintf(`(loop ((j q))
                  (<= j (- q %d)) j
                  (= (%% j 3) 0) j
                  else (again (- j 1)))`, 1+r.Intn(3))
		}
		return fmt.Sprintf(`(loop ((i 0) (q %d) (acc 0))
    (>= i (+ n %d)) (+ acc q)
    else (let j %s
           (again (+ i 1) (- j %d) (+ acc j))))`,
			r.Intn(21)-10, r.Intn(5), inner, r.Intn(4))
	},
	// The arm conditions and the backward rules: a guard over a sum, a
	// difference, a square, a multiple, a conjunction, a disjunction, a
	// negation, or a second variable (the other side of a π), with each arm
	// computing on the values the guard narrowed there.
	func(r *rand.Rand) string {
		k := func(lo, hi int) int { return lo + r.Intn(hi-lo+1) }
		conds := []string{
			fmt.Sprintf("(< (+ i %d) %d)", k(-3, 3), k(-4, 8)),
			fmt.Sprintf("(> (- %d i) %d)", k(-4, 6), k(-3, 3)),
			fmt.Sprintf("(<= (* i i) %d)", k(0, 30)),
			fmt.Sprintf("(>= (* %d i) %d)", k(-3, 3), k(-6, 9)),
			fmt.Sprintf("(and (< (+ i %d) %d) (>= (- i %d) %d))", k(-2, 2), k(0, 9), k(-2, 2), k(-5, 3)),
			fmt.Sprintf("(or (< (* 2 i) %d) (> (- i %d) %d))", k(-4, 8), k(-3, 3), k(0, 6)),
			fmt.Sprintf("(not (< (- %d i) %d))", k(-3, 6), k(-3, 3)),
			fmt.Sprintf("(= (+ i %d) %d)", k(-3, 3), k(-3, 9)),
			"(< i m)",
			fmt.Sprintf("(>= (+ i %d) m)", k(-2, 2)),
		}
		c := conds[r.Intn(len(conds))]
		return fmt.Sprintf(`(loop ((i (- 0 %d)) (m (+ n %d)) (acc 0))
    (>= i (+ n %d)) acc
    else (again (+ i 1) m
           (if %s (+ acc (+ (* i %d) (* m 2))) (- acc (+ (* i %d) m)))))`,
			k(0, 6), k(-3, 3), k(0, 6), c, k(-3, 3), k(-3, 3))
	},
	// The step at ONE continue, from its own facts: a counter reset to a
	// literal below its guard (match.oro's shape) still decreases, since the
	// reset is taken only where the counter exceeds the literal.
	func(r *rand.Rand) string {
		d := 2 + r.Intn(9)
		return fmt.Sprintf(`(loop ((v (+ n %d)) (c 0) (k 0))
    (= v 0) (+ c k)
    (>= v %d) (again (- v %d) (+ c 1) (+ k 1))
    else (again 0 c (+ k 1)))`, r.Intn(30), d, d)
	},
	// B = ∞: a loop no parameter ranks (the walk advances j only on one arm),
	// carrying a buffer given only fresh values, whose cells keep their hull
	// however many times the loop goes round.
	func(r *rand.Rand) string {
		l := 1 + r.Intn(5)
		return fmt.Sprintf(`(let t (build b %d
          (loop ((b b) (i 0) (j 0) (m 0))
            (>= j (+ n %d)) (set b 0 (+ (b 0) m))
            (= (%% i 3) 0) (again (set b (%% i %d) %d) (+ i 1) j (- m 1))
            else (again (set b (%% j %d) (%% i %d)) (+ i 1) (+ j 1) m)))
    (+ (t 0) (t %d)))`, l, r.Intn(9), l, r.Intn(21)-10, l, 2+r.Intn(5), l-1)
	},
}

func TestTheLoopRulesAreSound(t *testing.T) {
	dir := t.TempDir()
	r := rand.New(rand.NewSource(7))
	const cases = 330
	compiled, ran := 0, 0
	for c := 0; c < cases; c++ {
		shape := loopShapes[c%len(loopShapes)](r)
		src := fmt.Sprintf("(export run)\n(sig run ((n (int 0 12))) int)\n(def run (n)\n  %s)\n", shape)
		path := filepath.Join(dir, fmt.Sprintf("c%d.oro", c))
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		tg, p, err := compilePath(path, "go", Options{})
		if err != nil {
			continue
		}
		compiled++
		f := p.Funcs[0]
		fs := analyse(tg, f)
		for n := int64(0); n <= 12; n += 3 {
			seen, err := runFunc(f, []cval{intv(n)})
			if err != nil {
				if strings.Contains(err.Error(), "unsupported") {
					t.Fatalf("case %d uses an operation the interpreter lacks: %v\n%s", c, err, src)
				}
				continue
			}
			ran++
			if err := contained(f, fs, seen); err != nil {
				t.Fatalf("case %d, n = %d: %v\n%s", c, n, err, src)
			}
		}
	}
	// Anti-vacuity: a generator whose programs never compile, or never run,
	// checks nothing.
	if compiled < cases*3/4 || ran < compiled*3 {
		t.Fatalf("only %d of %d cases compiled and %d runs completed", compiled, cases, ran)
	}
	t.Logf("%d of %d cases compiled, %d runs checked", compiled, cases, ran)
}

// TestTheUnsignedLoopIsContained: the unsigned word's rules at a loop, on a
// program wordsel selected (testdata/u64-digits.oro, compiled by `gen -ir`).
// digitsum's v is in U: its guard `u64=` narrows v on each arm only because
// the domain reads U's order as ℤ's, and it ranks the loop only because
// `u64/` by a literal is a geometric step. Without either, `s + digit` is
// unproven. Every value of a run must lie in its fact, the answers must be
// the case's, and every operation must be proven.
func TestTheUnsignedLoopIsContained(t *testing.T) {
	src, err := os.ReadFile("testdata/u64-digits.ir")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Read(string(src))
	if err != nil {
		t.Fatal(err)
	}
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	f := p.Funcs[0]
	fs := analyse(tg, f)
	want := map[int64]int64{0: 87, 1: 86, 7: 89, 13: 83, 99: 87, 100: 86}
	for n, w := range want {
		seen, err := runFunc(f, []cval{intv(n)})
		if err != nil {
			t.Fatalf("run(%d): %v", n, err)
		}
		if err := contained(f, fs, seen); err != nil {
			t.Fatalf("run(%d): %v", n, err)
		}
		if got := seen[f.Body.Args[0]]; got == nil || got.ints[len(got.ints)-1].Int64() != w {
			t.Fatalf("run(%d) is not %d", n, w)
		}
	}
	if leg := Decide(tg, f.Clone(), false); leg.Proven != leg.Ops {
		t.Fatalf("%d of %d proven: %v", leg.Proven, leg.Ops, leg.Unproven)
	}
}

// TestTheShiftedLoopIsContained: the shift as Theorem 2's geometric step, on
// runs.oro as SelectShifts left it (testdata/runs-shift.oro, by `gen -ir`):
// `(go./ v 2)` is `v >> 1` and `(go.% v 2)` is `v & 1`. The run counter is
// proven only because v >> 1 ranks the loop. Every value of a run must lie in
// its fact, and every operation must be proven.
func TestTheShiftedLoopIsContained(t *testing.T) {
	src, err := os.ReadFile("testdata/runs-shift.ir")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Read(string(src))
	if err != nil {
		t.Fatal(err)
	}
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	f := p.Funcs[0]
	fs := analyse(tg, f)
	for _, n := range []int64{0, 1, 2, 5, 0b1011_0111, 1<<32 - 1, 0xAAAAAAAA} {
		seen, err := runFunc(f, []cval{intv(n)})
		if err != nil {
			t.Fatalf("runs(%d): %v", n, err)
		}
		if err := contained(f, fs, seen); err != nil {
			t.Fatalf("runs(%d): %v", n, err)
		}
	}
	if leg := Decide(tg, f.Clone(), false); leg.Proven != leg.Ops {
		t.Fatalf("%d of %d proven: %v", leg.Proven, leg.Ops, leg.Unproven)
	}
}
