package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
	"oroboros/ir/java"
)

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
    else (let t (+ (* (at a i) k) c)
           (again (set o i (% t 16777216)) (+ i 1) (/ t 16777216))))))))
(def f (fn (n) (at (loop ((acc (small 1)) (i 2)) (> i n) acc
                     else (again (step acc i) (+ i 1))) 0)))
`
	for _, dir := range []string{"../targets/go", "../targets/java"} {
		tg, err := emit.LoadTarget(dir)
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
			code, err = golang.FromResidual(tg, "f", prog.Sigs["f"], nf)
		} else {
			code, err = java.FromResidual(tg, "f", prog.Sigs["f"], nf)
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
