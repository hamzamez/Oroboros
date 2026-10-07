package emit_test

import (
	"strings"
	"testing"
)

// What examples/lines/lines.oro demanded of the compiler (bufio-2026-10-01).
// Each rule below has a refusal beside its acceptance, and each refusal was
// accepted with the rule's fault planted.

// A CONDITIONAL JOINS TWO COMPARABLE BRANCHES OF ONE REPRESENTATION AT THE
// LARGER (types.md §3.2). `(if c "" b)` with b a host string is a host
// string: string ≤ go.bytestring, both Go's `string`. It is not a `string`,
// since the way back is a decode; and a concrete type does not join its
// interface here, which the IR could not hold.
func TestAConditionalJoinsAtTheLarger(t *testing.T) {
	head := "(use go) (use go/os as gos) (use go/strings as strings) (use go/fmt as fmt) (export f) "
	for _, c := range []struct{ src, refused string }{
		{`(sig f ((c bool) (b go.bytestring)) go.bytestring) (def f (c b) (if c "" b))`, ""},
		{`(sig f ((c bool) (b go.bytestring)) go.bytestring) (def f (c b) (if c b ""))`, ""},
		{`(sig f ((c bool) (b go.bytestring)) string) (def f (c b) (if c "" b))`,
			"its signature declares result string, and the body yields go.bytestring"},
		{`(sig f ((c bool) (w go/io.Writer)) bool)
		  (def f (c w) (let (tuple n e) (fmt.Fprintln (if c (gos.Stderr) w) "x") (go.err-nil e)))`,
			"the branches of a conditional are go/os.File and go/io.Writer"},
		{`(sig f ((c bool)) bool)
		  (def f (c) (let (tuple n e) (fmt.Fprintln (if c (gos.Stderr) (strings.NewBuilder)) "x") (go.err-nil e)))`,
			"the branches of a conditional are"},
	} {
		_, err := entryGo(t, head+c.src)
		switch {
		case c.refused == "" && err != nil:
			t.Errorf("%s: %v", c.src, err)
		case c.refused != "" && (err == nil || !strings.Contains(err.Error(), c.refused)):
			t.Errorf("%s\n  want a refusal naming %q, got %v", c.src, c.refused, err)
		}
	}
}

// A LOOP VARIABLE DOES NOT INHERIT A PARAMETER'S FACTS (core/hygiene.go). The
// parameter n is declared in [0, 5] and the loop's own n runs to 101; the
// refinement layer's facts are keyed by name, and it gave the loop's exit the
// parameter's range, so an index of 101 into a table of 6 was accepted on
// every target. On JavaScript that is `undefined`; on x86 it is a read outside
// the table.
func TestALoopVariableDoesNotInheritAParametersFacts(t *testing.T) {
	src := `(use go) (export f)
(sig f ((n (int 0 5)) (a (array int))) int (where (= (len a) 6)))
(def f (n a)
  (let s (loop ((n 10))
           (> n 100) n
           else (again (+ n 1)))
    (a s)))`
	_, err := entryGo(t, src)
	if err == nil || !strings.Contains(err.Error(), "is not proven") {
		t.Errorf("an index of 101 into a table of 6 was accepted: %v", err)
	}
	// The control: the parameter itself is in range.
	if _, err := entryGo(t, `(use go) (export f)
(sig f ((n (int 0 5)) (a (array int))) int (where (= (len a) 6)))
(def f (n a) (a n))`); err != nil {
		t.Errorf("the parameter's own range no longer proves its index: %v", err)
	}
}

// A VALUE IS BOUNDED BY ITS TAILS, through every form of a clause chain and
// the loop it sits in (refine.go, joinConditional). os.Exit requires
// 0 ≤ code ≤ 255, and a status that is 0 or 1 at every exit of a loop, one of
// them under a host call's continuation, is in [0, 1].
func TestAStatusIsBoundedByItsExits(t *testing.T) {
	prog := func(exit0, exit1 string) string {
		return `(use go) (use go/os as gos) (use go/fmt as fmt) (export f)
(sig f ((k (int 0 100))) int)
(def f (k)
  (let s (loop ((i 0))
           (>= i k)  ` + exit0 + `
           (= i 50)  (let (tuple w e) (fmt.Fprintln (gos.Stderr) "x") (seq (ignore e) ` + exit1 + `))
           else      (again (+ i 1)))
    (gos.Exit s)))`
	}
	if _, err := entryGo(t, prog("0", "1")); err != nil {
		t.Errorf("a status of 0 or 1 at every exit: %v", err)
	}
	for _, c := range []struct{ exit0, exit1, why string }{
		// a tail under a continuation is a tail
		{"0", "300", "an exit of 300 under a host call's continuation"},
		// a loop variable is a binder: its initial value holds on the first
		// iteration only, and here it exits anywhere up to k
		{"(* i 3)", "1", "an exit of 3i, up to 300"},
	} {
		_, err := entryGo(t, prog(c.exit0, c.exit1))
		if err == nil || !strings.Contains(err.Error(), "go/os.Exit requires") {
			t.Errorf("%s was accepted as an exit status: %v", c.why, err)
		}
	}
}

// A LOOP VARIABLE HAS ITS INITIAL VALUE'S LENGTH WHEN EVERY BACK EDGE KEEPS IT
// (refine.go, threadLengths). `set` preserves length, so a variable threading a
// buffer is as long as the buffer at every iteration, by induction. Until
// bufio-2026-10-01 that was known only when the variable was spelled like the
// buffer, through the fact layer's names; here it never is.
func TestALoopVariableHasItsBuffersLength(t *testing.T) {
	head := "(use go) (export f) "
	for _, c := range []struct {
		why, src string
		ok       bool
	}{
		{"a variable threading a buffer under another name", `
(sig f ((n (int 1 100))) (array int))
(def f (n)
  (build t n
    (loop ((u t) (i 0))
      (>= i n) u
      else (again (set u i (+ (u i) i)) (+ i 1)))))`, true},
		{"two buffers that swap on the back edge, both of length n", `
(sig f ((n (int 1 100))) int)
(def f (n)
  (build a n  b n
    (loop ((x a) (y b) (i 0))
      (>= i n) (x 0)
      else (again (set y i (+ (x i) 1)) x (+ i 1)))))`, true},
		{"a back edge through a let", `
(sig f ((n (int 1 100))) (array int))
(def f (n)
  (build t n
    (loop ((u t) (i 0))
      (>= i n) u
      else (let w (set u i 7)
             (again (set w i (w i)) (+ i 1))))))`, true},
		// The back edge passes a table of length 1 where one of length n is
		// read: the equation is not inductive, and the index is not proven.
		{"a back edge that passes a shorter table", `
(sig f ((n (int 2 100)) (a (array int)) (b (array int))) int
  (where (and (= (len a) n) (= (len b) 1))))
(def f (n a b)
  (loop ((c a) (i 0) (s 0))
    (>= i n) s
    else (again b (+ i 1) (+ (c i) 0))))`, false},
		// Two buffers of different lengths that swap: neither equation holds.
		{"two buffers of different lengths that swap", `
(sig f ((n (int 2 100))) int)
(def f (n)
  (build a n  b 1
    (loop ((x a) (y b) (i 0))
      (>= i n) (x 0)
      else (again (set y 0 (+ (x i) 1)) x (+ i 1)))))`, false},
	} {
		_, err := entryGo(t, head+c.src)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.why, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "is an indexing")) {
			t.Errorf("%s: want the index refused, got %v", c.why, err)
		}
	}
}

// INSIDE A LOOP, ITS VARIABLES' INITIAL VALUES ARE NOT ASSUMED (refine.go,
// joinConditional). The tails here are constants, 0 and 1, reached under
// guards that compare the loop variable i with the parameter m. With i = 0
// assumed on those paths, (>= i m) gives m ≤ 0, and the template m ≤ s holds
// at every tail: a false fact, since m is up to 100 and s is 0 or 1. It would
// prove 0 ≤ s − m, and the index below is −m or 1 − m. (s is used twice, so
// that reduction leaves it bound: a pure value used once is substituted.)
func TestATailsPathDoesNotAssumeALoopVariablesStart(t *testing.T) {
	src := `(use go) (export f)
(sig f ((m (int 2 100)) (a (array int))) int (where (= (len a) 2)))
(def f (m a)
  (let s (loop ((i 0))
           (>= i m)  0
           (>= i 150) 1
           else      (again (+ i 1)))
    (+ (a (- s m)) s)))`
	_, err := entryGo(t, src)
	if err == nil || !strings.Contains(err.Error(), "is an indexing") {
		t.Errorf("an index of -m was accepted: %v", err)
	}
	// The control: the same value is bound and bounded, s ∈ [0, 1], so the
	// refusal above is not the rule failing to see the loop at all.
	if _, err := entryGo(t, strings.Replace(src, "(a (- s m))", "(a s)", 1)); err != nil {
		t.Errorf("s is 0 or 1 and the table has two elements: %v", err)
	}
}
