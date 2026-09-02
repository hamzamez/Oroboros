package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE MUTABLE BIGNUM (emit/bigreuse.go). Rule R lets a bignum operation write
// into storage nothing live can read, instead of allocating a fresh receiver.
//
// Each condition is pinned separately, because they fail in different ways:
// (3) failing is a wrong answer inside the loop, (4) failing is a wrong answer
// in the CALLER, and the rule declining is only ever slow.

const bigFactSrc = `(export fact)
(sig fact ((n (int 0 30))) (int 0 (pow 2 110)))
(def fact (fn (n) (loop ((acc 1) (i 2)) (> i n) acc else (again (* acc i) (+ i 1)))))
`

// THE ACCUMULATOR IS ITS OWN DESTINATION. `acc` is read once at the back edge,
// inside the multiply that replaces it, so its old value is dead the moment the
// multiply has read it — which is exactly `acc.Mul(acc, k)`, the form a Go
// programmer writes once they care.
func TestAnAccumulatorIsWrittenInPlace(t *testing.T) {
	got := promoteOn(t, "../targets/go", bigFactSrc, "fact")
	if !strings.Contains(got, "(big*! acc acc") {
		t.Errorf("the multiply still allocates a fresh receiver:\n%s", got)
	}
}

// AND A ROTATION FALLS OUT OF THE SAME RULE. fib assigns `b` to `a` and the sum
// to `b`, so `b`'s object is live in two places and `a`'s is live in exactly
// one — the sum. So the sum is written into `a`, and the simultaneous
// assignment does the rotation.
//
// The hand-written careful form uses THREE objects and a temporary; this uses
// two, because the rule found the free one rather than being told about it.
func TestALoopRotatesThroughTheVariableThatIsFree(t *testing.T) {
	src := `(export fib)
(sig fib ((n (int 0 1000))) (int 0 (pow 2 1000)))
(def fib (fn (n) (loop ((a 0) (b 1) (i 0)) (>= i n) a else (again b (+ a b) (+ i 1)))))
`
	got := promoteOn(t, "../targets/go", src, "fib")
	if !strings.Contains(got, "(big+! a a b)") {
		t.Errorf("the sum was not written into `a`, whose object is free:\n%s", got)
	}
	if strings.Contains(got, "(big+! b") {
		t.Errorf("the sum was written into `b`, which is ALSO assigned to `a` at "+
			"this back edge — two variables would share one object:\n%s", got)
	}
}

// CONDITION (3): A VALUE READ TWICE AT ONE BACK EDGE MAY NOT BE OVERWRITTEN.
//
// `power`'s squaring clause reads `x` for the accumulator and again for itself,
// so no variable is free for that argument and it keeps its allocation. The
// accumulator in the same clause IS free and does get written in place, so this
// is a test that the rule is selective rather than off.
//
// Refusing is the safe direction: it costs an allocation, where a wrong
// destination costs an answer.
func TestAValueReadTwiceKeepsItsAllocation(t *testing.T) {
	src := `(export power)
(sig power ((b (int 0 1000)) (e (int 0 64))) (int 0 (pow 2 640)))
(def power (fn (b e)
  (loop ((acc 1) (x b) (k e))
    (= k 0)        acc
    (= (% k 2) 1)  (again (* acc x) (* x x) (/ k 2))
    else           (again acc (* x x) (/ k 2)))))
`
	got := promoteOn(t, "../targets/go", src, "power")
	if !strings.Contains(got, "(big*! acc acc x)") {
		t.Errorf("the accumulator was not written in place:\n%s", got)
	}
	// In the clause that also reads `x` for the accumulator, the square must
	// still allocate. In the OTHER clause `x` is read only by its own square,
	// so there it is written in place — both appear, which is the point.
	if !strings.Contains(got, "(big* x x)") {
		t.Errorf("every square was written in place; the clause that also reads "+
			"`x` for the accumulator must not be:\n%s", got)
	}
	if !strings.Contains(got, "(big*! x x x)") {
		t.Errorf("the clause whose only reader of `x` is its own square did not "+
			"get the in-place form:\n%s", got)
	}
}

// CONDITION (4), AND IT IS THE ONE WHOSE ABSENCE IS A SILENT WRONG ANSWER IN
// THE CALLER.
//
// A loop variable initialised from a big PARAMETER holds an object the caller
// still owns. Writing into it corrupts the caller's value — and the loop's own
// answer is correct, so nothing inside the function looks wrong. It is only
// visible to someone who uses the argument again afterwards.
//
// The shape is indistinguishable from the safe one by eye: `(loop ((acc n)) …)`
// against `(loop ((acc 1)) …)`.
func TestALoopVariableFromAParameterIsNeverWrittenInto(t *testing.T) {
	src := `(export sq)
(sig sq ((n (int 0 (pow 2 70)))) (int 0 (pow 2 200)))
(def sq (fn (n) (loop ((acc n) (i 0)) (>= i 2) acc else (again (* acc n) (+ i 1)))))
`
	got := promoteOn(t, "../targets/go", src, "sq")
	if strings.Contains(got, "big*!") {
		t.Errorf("a loop variable initialised from a PARAMETER was written into; "+
			"the object belongs to the caller:\n%s", got)
	}
}

// AND A HOST WHOSE BIGNUM IS IMMUTABLE GETS NONE OF THIS, which is not a gap in
// its target file.
//
// `java.math.BigInteger` is immutable and the JDK's mutable version is
// package-private; JavaScript's `BigInt` is a primitive. So on two of the three
// hosts that HAVE a bignum, the careful hand-written form does not exist either
// — our claim there is unchanged and already at the bar, because the allocating
// form IS the best a person can write with the host's own bignum.
func TestOnlyGoCanWriteIntoItsBignum(t *testing.T) {
	for _, dir := range []string{"../targets/js", "../targets/java", "../targets/windows"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatal(err)
		}
		if tg.HasBigDest() {
			t.Errorf("%s declares a destination form; if that host's bignum has "+
				"become mutable this test has done its job, but check that the "+
				"declaration is not simply wrong", dir)
		}
	}
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if !tg.HasBigDest() {
		t.Error("Go no longer declares a destination form; math/big writes into " +
			"its receiver and that is the whole of this optimisation")
	}
	// The destination forms must be IMPURE. An operation that writes into an
	// object it was handed is what ADR 0010's effect discipline exists to keep
	// in order; declaring one pure would let the reducer substitute it into two
	// places, and both would write the same object.
	for _, n := range []string{"big+!", "big-!", "big*!", "big/!", "big%!", "big-of!"} {
		if tg.Prims[n].Pure {
			t.Errorf("%s is declared pure; it writes into its first argument", n)
		}
	}
	// And the plain forms must stay pure, or nothing fuses.
	for _, n := range []string{"big+", "big*", "big-of"} {
		if !tg.Prims[n].Pure {
			t.Errorf("%s is not pure; it allocates a fresh result and reads nothing "+
				"it can change", n)
		}
	}
}

// THE RULE IS OFF WHEN NOTHING IS A BIGNUM, which is the containment property
// for this pass: a program with no declaration above the window must be
// untouched.
func TestNoBignumMeansNoRewrite(t *testing.T) {
	src := `(export f)
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (loop ((acc 0) (i 0)) (>= i n) acc else (again (+ acc i) (+ i 1)))))
`
	got := promoteOn(t, "../targets/go", src, "f")
	if strings.Contains(got, "!") {
		t.Errorf("a program with no bignum was rewritten:\n%s", got)
	}
}

// ONE ELEMENT TYPE PER LOOP-CARRIED BUFFER (LoopOneJoin).
//
// A buffer that is a loop variable has SEVERAL sources and one variable, and
// each `build` was narrowed on its own. Nothing in the corpus had two that
// disagreed until the interval analysis got precise enough about division —
// a bignum accumulator starts as a buffer whose stores are provably 0..1 and is
// replaced each iteration by one whose stores are 0..2^24−1.
//
// A COMPILE ERROR IS THE LUCKY CASE, and only two of the four hosts give one:
// Go and Java type their slices, JavaScript narrows nothing, and on x86 the
// element size travels BY NAME (wintables-2026-08-25), so reading a byte table
// as qwords is a wrong answer rather than a slow one.
func TestALoopCarriedBufferHasOneElementType(t *testing.T) {
	src := `(export f)
(sig f ((n (int 2 200))) int)
(def at (fn (t i) (if (< i 0) 0 (if (>= i (len t)) 0 (t i)))))
(def small (fn (v) (build 8 (fn (b)
  (loop ((b b) (i 0) (r v)) (>= i 8) b
    else (again (set b i (% r 16777216)) (+ i 1) (/ r 16777216)))))))
(def step (fn (a k) (build 8 (fn (o)
  (loop ((o o) (i 0) (c 0)) (>= i 8) o
    else (let (+ (* (at a i) k) c) (fn (t)
           (again (set o i (% t 16777216)) (+ i 1) (/ t 16777216)))))))))
(def f (fn (n) (at (loop ((acc (small 1)) (i 2)) (> i n) acc
                     else (again (step acc i) (+ i 1))) 0)))
`
	for _, dir := range []string{"../targets/go", "../targets/java"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatal(err)
		}
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, _, err := core.Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		env, err := tg.Env(prog)
		if err != nil {
			t.Fatal(err)
		}
		nf, err := core.Normalize(prog.Defs["f"], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		var code string
		if dir == "../targets/go" {
			code, err = Func(tg, "f", prog.Sigs["f"], nf)
		} else {
			code, err = JavaMethod(tg, "f", prog.Sigs["f"], nf)
		}
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		// The two allocations must name the SAME element type. Comparing the
		// emitted text is the honest check here: the failure this exists to
		// catch is precisely that two spellings reach the host.
		kinds := map[string]bool{}
		for _, line := range strings.Split(code, "\n") {
			// Matched with the closing punctuation, because `make([]int` is a
			// PREFIX of `make([]int32` and a substring test reports two kinds
			// where the emitter wrote one.
			for _, k := range []string{"byte", "int8", "int16", "int32", "uint16", "int"} {
				if strings.Contains(line, "make([]"+k+",") {
					kinds[k] = true
				}
			}
			for _, k := range []string{"byte", "short", "int", "long"} {
				if strings.Contains(line, "new "+k+"[") {
					kinds[k] = true
				}
			}
		}
		if len(kinds) > 1 {
			t.Errorf("%s: a loop-carried buffer got %v; its sources must agree\n%s",
				dir, kinds, code)
		}
	}
}
