package emit_test

import (
	"testing"

	"oroboros/ir"
	"oroboros/ir/golang"
)

// TWO FAULTS examples/os/du.oro found, the first program to reach either.

// A LOOP'S RESULTS ARE TYPED WHERE THE LOOP IS, before their uses (ir/typing.go,
// program order). The loop sits in a scope, and its results meet strings in a
// variadic list of boxes. Typed outermost region first, the list fixed them as
// strings before the loop's region was visited, and the loop's ints clashed in
// the verifier (W5); typed in program order they are ints, boxed at the list.
func TestALoopsResultsAreTypedBeforeTheirUses(t *testing.T) {
	compiles(t, `(use go/os as os)
(use go/fmt as fmt)
(export main)
(def walk (root)
  (local stk (table 4 (fn (k) root))
    (loop ((stk stk) (k 0) (a 0) (b 0))
      (< 3 k) (tuple a b)
      else (again (set stk k root) (+ k 1) (+ a 1) (+ b 2)))))
(def main ()
  (let av (os.Args)
    (if (< (len av) 1) (tuple)
      (let (tuple a b) (walk (av 0)) (fmt.Println a "and" b)))))
`)
}

// AN ASCRIPTION WRITTEN IN A PROGRAM LOWERS (core/desugar.go): `(the S e)` carries
// S as its canonical type spelling, the form every consumer reads. Written as a
// term, `int` was lowered as a value; the interval route, which lowers the
// residual to decide a range obligation, failed silently, and h's range was
// refused as unproven by the route that decides ranges above the word.
func TestAnAscriptionInAProgramLowers(t *testing.T) {
	compiles(t, `(export f)
(sig h ((x (int 0 100))) int)
(def h (x) (+ x 1))
(sig f ((n (int 0 5))) int)
(def f (n) (h (the (int 0 9) (+ n 1))))
`)
}

// compiles runs the drivers' pipeline on src's first export, on Go, and prints it.
func compiles(t *testing.T, src string) {
	t.Helper()
	tg := goNative(t)
	prog, nf, err := normalizeOn(t, tg, src)
	if err != nil {
		t.Fatal(err)
	}
	name := prog.Exports[0]
	f, _, err := ir.Entry(tg, name, name, prog.Sigs[name], nf, allProgSigs(prog), false, nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	// AND PRINTED, as gen prints it: the step to IR_P verifies the types (W5).
	if _, err := golang.FromFunc(tg, f); err != nil {
		t.Fatalf("not printed: %v", err)
	}
}
