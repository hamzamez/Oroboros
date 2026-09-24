package emit

import (
	"testing"

	"oroboros/core"
)

// A BUFFER KEEPS ITS LENGTH (buflen.go). Each test hangs one operation's proof
// on a length: `(+ (len b) K)` with K = 2^63 − 1 − L is inside the word exactly
// when len b ≤ L. So a length the analysis knows proves it, a length it does not
// know leaves it unproven, and a length it wrongly believes proves an overflow.

// intervalsGo reduces a program's first export on Go and reports how many of its
// integer operations are proven.
func intervalsGo(t *testing.T, src string) (proven, ops int) {
	t.Helper()
	tg := goNative(t)
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
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	rep, _ := Intervals(tg, prog.Sigs[q], nf, 0)
	return rep.Proven, rep.Ops
}

// A LOOP-CARRIED BUFFER keeps the length of the `build` that made it, through
// its stores: 48 + (2^63 − 49) is the largest int64.
func TestALoopCarriedBufferKeepsItsLength(t *testing.T) {
	src := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (let t (build 48 (fn (b)
           (loop ((b b) (i 0))
             (>= i n) (set b 0 (+ (len b) 9223372036854775759))
             else     (again (set b i 1) (+ i 1)))))
    (t 0)))`
	if p, o := intervalsGo(t, src); p != o {
		t.Errorf("%d of %d proven: the loop's buffer is the build's, 48 cells", p, o)
	}
	// AND THROUGH A NESTED LOOP that hands its own buffer variable back at every
	// exit, which is how every winmap probe is shaped.
	nested := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (let t (build 48 (fn (b)
           (loop ((b b) (i 0))
             (>= i n) (set b 0 (+ (len b) 9223372036854775759))
             else     (again (loop ((c b) (j 0))
                               (>= j 2) c
                               else     (again (set c j 1) (+ j 1)))
                             (+ i 1)))))
    (t 0)))`
	if p, o := intervalsGo(t, nested); p != o {
		t.Errorf("%d of %d proven: the inner loop hands back the buffer it was given", p, o)
	}
}

// A LOOP VARIABLE REBOUND TO ANOTHER TABLE does not keep its initialiser's
// length: 3 on entry, 8 after one back edge, so 8 + (2^63 − 4) overflows.
func TestATableReplacedOnABackEdgeHasNoLength(t *testing.T) {
	src := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (loop ((t (array 1 2 3)) (i 0))
    (>= i n) (+ (len t) 9223372036854775804)
    else     (again (array 1 2 3 4 5 6 7 8) (+ i 1))))`
	if p, o := intervalsGo(t, src); p == o {
		t.Errorf("%d of %d proven: len t may be 8, and 8 + (2^63 − 4) is past the word", p, o)
	}
}

// A NAME IS NOT ITS SPELLING. A loop variable spelled like the build's buffer,
// holding a table of unknown length, must not read the buffer's 3.
//
// The result is the build's LENGTH, not a `let`: binding the table would run the
// element analysis over the build first, which opens the loop once and takes the
// spelling `b`, and the counted walk would then see `b2` and no collision at all.
// These two tests were vacuous in that shape — they passed with the shadowing
// removed — until the plant showed it.
func TestALoopVariableDoesNotInheritAShadowedLength(t *testing.T) {
	src := `(export f)
(sig f ((a (array int)) (n (int 0 10))) int)
(def f (a n)
  (len (build 3 (fn (b)
         (set b 0 (loop ((b a) (i 0))
                    (>= i n) (+ (len b) 9223372036854775804)
                    else     (again b (+ i 1))))))))`
	if p, o := intervalsGo(t, src); p == o {
		t.Errorf("%d of %d proven: the loop's b is a, of any length, not the build's 3 cells", p, o)
	}
}

// AND A `let` BINDER: a table of unknown length bound under the buffer's
// spelling reads its own length, not the buffer's. Used twice, so it stays bound.
func TestALetBinderDoesNotInheritAShadowedLength(t *testing.T) {
	src := `(export f)
(sig f ((n (int 0 10))) int)
(def f (n)
  (len (build 3 (fn (b)
         (set b 0 (let b (alloc (table n (fn (i) i)))
                    (+ (len b) (- 9223372036854775804 (len b)))))))))`
	if p, o := intervalsGo(t, src); p == o {
		t.Errorf("%d of %d proven: the let's b has n cells, not the build's 3", p, o)
	}
}

// A HOST CALL'S RESULT is the host's table: spelled like the buffer, it still
// has a length of its own.
func TestAContinuationParameterDoesNotInheritAShadowedLength(t *testing.T) {
	src := `(use go/os as os)
(export f)
(sig f ((p string)) int)
(def f (p)
  (len (build 3 (fn (b)
         (set b 0 ((os.ReadFile p) (fn (b err) (+ (len b) 9223372036854775804))))))))`
	if p, o := intervalsGo(t, src); p == o {
		t.Errorf("%d of %d proven: the file's bytes are not the build's 3 cells", p, o)
	}
}

// THE MASK IS STRICT IN ⊥. In a branch no value reaches, n is ⊥, and so is
// n & n: the mask's two clauses are for a non-negative operand, and ⊥ is
// neither, so it answered ⊤ and the addition after it was refused. (The
// additive transfers are left as they were: making them canonical too lost
// print-int's digit bound on windows, which the differential suite caught.)
func TestAnUnreachableMaskIsUnreachable(t *testing.T) {
	src := `(use go)
(export f)
(sig f ((n (int 0 10))) int)
(def f (n) (if (< n 0) (+ (go.& n n) 1) 0))`
	if p, o := intervalsGo(t, src); p != o {
		t.Errorf("%d of %d proven: the branch is dead, so the mask and its sum are ⊥", p, o)
	}
}
