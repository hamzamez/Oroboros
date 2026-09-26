package ir

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// bigUnit is one exported definition lowered and decided on the host's rung,
// before the selection.
type bigUnit struct {
	f    *Func
	sig  *core.Sig
	plan emit.BigPlan
}

// lowerUnselected is compilePath up to the selection: the term keeps its
// ascriptions above the word, and the IR is decided but not selected.
func lowerUnselected(src, target string) (*emit.Target, []bigUnit, error) {
	layers, err := emit.SearchPath(src, filepath.Join("..", "targets"))
	if err != nil {
		return nil, nil, err
	}
	dirs := []string{filepath.Dir(src), filepath.Join("..", "lib")}
	tg, err := emit.LoadTargetLayers(target, layers, dirs)
	if err != nil {
		return nil, nil, err
	}
	text, err := os.ReadFile(src)
	if err != nil {
		return nil, nil, err
	}
	forms, err := core.Read(string(text))
	if err != nil {
		return nil, nil, err
	}
	prog, _, err := core.LoadWithDefs(forms, resolver(dirs), tg.Defs)
	if err != nil {
		return nil, nil, err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return nil, nil, err
	}
	reqs := emit.InstallRequires(env, prog)
	exports := append([]string(nil), prog.Exports...)
	sort.Strings(exports)
	var all []*core.Sig
	for _, s := range prog.Sigs {
		all = append(all, s)
	}
	var out []bigUnit
	for _, q := range exports {
		name := "t-" + q[strings.LastIndex(q, ".")+1:]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			return nil, nil, err
		}
		sig := prog.Sigs[q]
		if nf, err = emit.DischargeRequires(reqs, tg, name, sig, nf,
			func(x *core.Term) *core.Term { return DischargeRanges(tg, sig, x) }); err != nil {
			return nil, nil, err
		}
		plan, err := emit.PlanBig(tg, sig, nf, all...)
		if err != nil {
			return nil, nil, err
		}
		if !plan.Host {
			return nil, nil, fmt.Errorf("%s: not on the host's rung", name)
		}
		nf = emit.EraseWordAscriptions(tg.Word, nf)
		if err := emit.Check(tg, name, nf); err != nil {
			return nil, nil, err
		}
		f, err := Lower(tg, name, sig, nf, Options{Decided: true})
		if err != nil {
			return nil, nil, err
		}
		Decide(tg, f, false)
		out = append(out, bigUnit{f, sig, plan})
	}
	return tg, out, nil
}

// THE SELECTION KEEPS EVERY ANSWER (ir/big.go, ir/bigreuse.go). The
// interpreter computes the unselected function's operations in ℤ, and the
// selected one's bignum primitives in ℤ too, with a destination form writing
// INTO its receiver's object as math/big does. So the two must agree on every
// input: a value computed in the word that should not be is refused by the
// decision taken again, and a destination whose receiver is read later, or
// is held by something else, changes the answer.
//
// The shapes are the corpus's (factorial, Fibonacci's permutation, squaring,
// the pair, decimal digits by a word) and the two that make the destination
// rule's premises necessary: a continue that hands a parameter an object from
// outside the loop, and a parameter read after the operation that would write
// into it.
func TestTheBigSelectionKeepsEveryAnswer(t *testing.T) {
	dir := t.TempDir()
	r := rand.New(rand.NewSource(29))
	k := func(lo, n int) int { return lo + r.Intn(n) }
	big := func() string { return fmt.Sprintf("%d%018d%06d", k(1, 9), r.Int63n(1e18), r.Intn(1e6)) }
	shapes := []func() string{
		// factorial
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n) (loop ((acc %d) (i 1)) (> i (* n 3)) acc else (again (* acc (+ i %d)) (+ i 1))))`, k(1, 5), k(0, 9))
		},
		// Fibonacci: a permutation of two owned objects
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n) (loop ((a %d) (b %d) (i 0)) (>= i (* n 9)) a else (again b (+ a b) (+ i 1))))`, k(0, 3), k(1, 3))
		},
		// squaring: x read by the accumulator and by its own square
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (loop ((acc 1) (x (+ n %d)) (y (+ n %d)))
    (= y 0) acc
    (= (%% y 2) 1) (again (* acc x) (* x x) (/ y 2))
    else (again acc (* x x) (/ y 2))))`, k(2, 50), k(10, 60))
		},
		// the pair: each update reads both
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n) (loop ((a %d) (b %d) (i 0)) (>= i n) b else (again (* a b) (+ b a) (+ i 1))))`, k(2, 4), k(1, 4))
		},
		// a word result: the demand arrives by an ascription
		func() string {
			return fmt.Sprintf(`(sig big ((n (int 0 12))) (int 0 +inf))
(def big (n) (loop ((acc %d) (i 1)) (> i (* n 4)) acc else (again (* acc (+ i 7)) (+ i 1))))
(sig run ((n (int 0 12))) int)
(def run (n)
  (let x (big n)
    (+ (%% x %d) (if (> x %s) 1000 0))))`, k(1, 9), k(2, 100000), big())
		},
		// SUPPLY WITHOUT DEMAND: a remainder by a bignum, a join of bignums and
		// a constant past the word, each only compared, so nothing demands them
		func() string {
			return fmt.Sprintf(`(sig big ((n (int 0 12))) (int 0 +inf))
(def big (n) (loop ((acc %d) (i 1)) (> i (* n 4)) acc else (again (* acc (+ i 7)) (+ i 1))))
(sig run ((n (int 0 12))) int)
(def run (n)
  (let x (big n)
       y (big (- 12 n))
       t (if (> n %d) x (+ y 1))
    (+ (+ (if (> (%% x (+ y 1)) %d) 1 0)
          (if (> t %s) 10 0))
       (if (> (* n n) %s) 100 0))))`, k(1, 9), k(0, 12), k(0, 1000), big(), big())
		},
		// a permutation through a destination: b takes a's object, a is b·c
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (loop ((a (+ n %d)) (b 1) (i 0))
    (>= i 12) (+ a b)
    else (again (* b %d) a (+ i 1))))`, k(1, 9), k(2, 9))
		},
		// A CONTINUE HANDS acc AN OBJECT FROM OUTSIDE THE LOOP. Written into on
		// the next iteration, x itself would change.
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (let x (* (+ n 5) %s)
    (loop ((acc 1) (i 0))
      (>= i %d) (+ acc x)
      (= (%% i 3) 1) (again x (+ i 1))
      else (again (* acc %d) (+ i 1)))))`, big(), k(4, 6), k(2, 9))
		},
		// A PARAMETER READ AFTER THE OPERATION that would write into it
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (loop ((a %d) (b %d) (i 0))
    (>= i (* n 5)) (+ a b)
    else (again (* a %d) (+ a b) (+ i 1))))`, k(1, 9), k(1, 9), k(2, 9))
		},
		// ONE OBJECT BOUND TO TWO PARAMETERS: after (again b b), writing into
		// a would change b
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (loop ((a %d) (b (* (+ n %d) %s)) (i 0))
    (>= i (* n 3)) (+ a b)
    (= (%% i 3) 0) (again b b (+ i 1))
    else (again (* a %d) (+ b 1) (+ i 1))))`, k(1, 9), k(1, 9), big(), k(2, 9))
		},
		// A JOIN THAT MAY HOLD a's OBJECT, bound before the operation on a
		// (t is read twice, so it is not substituted) and read after it
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (loop ((a %d) (b %s) (i 0))
    (>= i (* n 3)) (+ a b)
    else (let t (if (> i %d) a b)
           (again (* a %d) (+ t t) (+ i 1)))))`, k(1, 9), big(), k(0, 4), k(2, 9))
		},
		// AN INITIALISER READ ELSEWHERE: the loop does not own x's object
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (let x (* (+ n 5) %s)
    (loop ((acc x) (i 0))
      (>= i %d) (+ acc x)
      else (again (* acc %d) (+ i 1)))))`, big(), k(1, 6), k(2, 9))
		},
		// conditionals and comparisons against words
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 +inf))
(def run (n)
  (loop ((acc %s) (i 0))
    (>= i (* n 4)) acc
    (> acc (* %s %d)) (again (- (/ acc %d) n) (+ i 1))
    else (again (+ (* acc %d) i) (+ i 1))))`, big(), big(), k(2, 50), k(2, 9), k(2, 9))
		},
		// a finite bound, enforced by big-fit, and division by a bignum
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int 0 (pow 2 400)))
(def run (n)
  (let a (* (+ n %d) %s) b (+ (* n %s) 7)
    (+ (/ a b) (%% a b))))`, k(1, 99), big(), big())
		},
		// a signed set
		func() string {
			return fmt.Sprintf(`(sig run ((n (int 0 12))) (int (- 0 (pow 2 300)) (pow 2 300)))
(def run (n)
  (let p %s q (* (- n 6) %s)
    (- (* p (- 0 n)) q)))`, big(), big())
		},
	}
	accepted, dests, fits := 0, 0, 0
	for c := 0; c < 220; c++ {
		src := "(export run)\n" + shapes[c%len(shapes)]() + "\n"
		path := filepath.Join(dir, fmt.Sprintf("b%d.oro", c))
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		tg, units, err := lowerUnselected(path, "go")
		if err != nil {
			t.Fatalf("%v\n%s", err, src)
		}
		u := units[len(units)-1]
		before := u.f
		after := before.Clone()
		after.facts = before.facts
		if _, err := SelectBig(tg, after, u.sig, u.plan.Bits, u.plan.Signed); err != nil {
			t.Fatalf("refused: %v\n%s", err, src)
		}
		if leg := Decide(tg, after, false); leg.Proven < leg.Ops {
			t.Fatalf("an operation left in the word is unproven: %v\n%s", leg.Unproven, src)
		}
		if err := ToP(tg, after.Clone()); err != nil {
			t.Fatalf("the selected function does not verify: %v\n%s\n%s", err, src, printF(after))
		}
		after.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if _, ok := destSource(s.Name); ok {
					dests++
				}
				if strings.HasPrefix(s.Name, "big-fit") {
					fits++
				}
			}
		})
		for x := int64(0); x <= 12; x++ {
			ra, err1 := runResult(before, []cval{intv(x)})
			rb, err2 := runResult(after, []cval{intv(x)})
			if err1 != nil || err2 != nil {
				t.Fatalf("run(%d): %v / %v\n%s\n%s", x, err1, err2, src, printF(after))
			}
			if ra.i == nil || rb.i == nil || ra.i.Cmp(rb.i) != 0 {
				t.Fatalf("run(%d) changed under the selection: %v, then %v\n%s\n%s", x, ra.i, rb.i, src, printF(after))
			}
		}
		accepted++
	}
	// ANTI-VACUITY: the destination rule and the bound must have fired.
	if dests < 100 || fits < 20 {
		t.Fatalf("%d destinations and %d fits in %d programs: the test checks too little", dests, fits, accepted)
	}
	t.Logf("%d programs, %d destinations, %d fits", accepted, dests, fits)
}

func printF(f *Func) string {
	return Print(&Program{Target: "go", Stage: StageA, Funcs: []*Func{f}})
}

// A VALUE HELD EXACTLY IS NOT A WORD: it may not reach a position declared in
// the word, and the refusal says why (unbounded-rung.md §3).
func TestABigValueIsRefusedWhereAWordIsDeclared(t *testing.T) {
	dir := t.TempDir()
	src := `(export run)
(sig run ((x (int 0 (pow 2 100)))) string)
(def run (x) (string-of (+ x 1)))
`
	path := filepath.Join(dir, "w.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tg, units, err := lowerUnselected(path, "go")
	if err != nil {
		t.Fatal(err)
	}
	u := units[0]
	if _, err := SelectBig(tg, u.f, u.sig, u.plan.Bits, u.plan.Signed); err == nil ||
		!strings.Contains(err.Error(), "WIDER than the word") {
		t.Errorf("a big value into string-of's int: %v", err)
	}
}

// AN OBJECT ALLOCATED BEFORE THE LOOP IS NOT THIS ITERATION'S, even read once.
// No source program reaches the shape: a pure binding read once is substituted
// into its use, so it is computed in the loop. A pass that hoisted it would
// make it, so the premise is witnessed on IR built by hand: x is handed to acc
// by a continue taken at i = 1 and i = 4, and written into at i = 2.
func TestAnObjectFromBeforeTheLoopIsNotOwned(t *testing.T) {
	src := `(ir 1
  (target go)
  (stage A)
  (ops const add rem eq ge call loop yield break continue branch)
  (func t (params (%0 (int 0 12))) (results big)
    (region
      (val (%1 big) (call big-of %0))
      (val (%2 int) (const 1))
      (val (%3 big) (call big-of %2))
      (val (%4 int) (const 0))
      (val (%5 big)
        (loop (init %3 %4)
          (region
            (params (%6 big) (%7 int))
            (val (%8 int) (const 6))
            (val (%9 bool) (ge %7 %8))
            (branch %9
              (region
                (break %6))
              (region
                (val (%10 int) (const 3))
                (val (%11 int) (rem exact %7 %10))
                (val (%12 int) (const 1))
                (val (%13 bool) (eq %11 %12))
                (val (%14 int) (const 1))
                (val (%15 int) (add exact %7 %14))
                (branch %13
                  (region
                    (continue %1 %15))
                  (region
                    (val (%16 int) (const 7))
                    (val (%17 big) (call big-of %16))
                    (val (%18 big) (call big* %6 %17))
                    (continue %18 %15))))))))
      (yield %5))))
`
	p, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	tg, _, err := compilePath(filepath.Join("..", "examples", "big", "fact.oro"), "go", Options{})
	if err != nil {
		t.Fatal(err)
	}
	f := p.Funcs[0]
	before, _ := runResult(f, []cval{intv(5)})
	reuseBig(tg, f)
	after, err := runResult(f, []cval{intv(5)})
	if err != nil || before.i.Cmp(after.i) != 0 {
		t.Errorf("an object from before the loop was written into: %v, then %v (%v)\n%s", before.i, after.i, err, printF(f))
	}
}

// THE ONE SET IS ENFORCED (ADR 0029): a program declaring (int 0 2^70), whose
// set is [0, 2^71), traps on the host's bignum when a sum reaches 4·10²¹ >
// 2^71 ≈ 2.36·10²¹, as it does on fixed limbs; the product, 2·10²¹, is inside.
func TestTheDeclaredBoundTrapsOnTheHostsBignum(t *testing.T) {
	dir := t.TempDir()
	src := `(export run)
(sig run ((n (int 0 12))) (int 0 (pow 2 70)))
(def run (n) (let a (* (+ n 1) 2000000000000000000000) (+ a a)))
`
	path := filepath.Join(dir, "fit.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	tg, units, err := lowerUnselected(path, "go")
	if err != nil {
		t.Fatal(err)
	}
	u := units[0]
	if _, err := SelectBig(tg, u.f, u.sig, u.plan.Bits, u.plan.Signed); err != nil {
		t.Fatal(err)
	}
	if _, err := runResult(u.f, []cval{intv(0)}); err == nil || !strings.Contains(err.Error(), "bignum overflow") {
		t.Errorf("4·10²¹ under a declared 2^70: %v\n%s", err, printF(u.f))
	}
}
