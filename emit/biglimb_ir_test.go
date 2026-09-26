package emit_test

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
)

// LOOP-CARRIED BUFFER REUSE (LoopBufferReuse), the back-edge instance of the
// gap named in four places.
//
// A `build` on a back edge writes into storage the loop already owns. The
// conditions are rule R's one level up, and the two that carry the weight are
// (3) — every occurrence of the loop variable is inside that argument — and the
// requirement that both lengths be the same CONSTANT, since two buffers of
// different sizes are not interchangeable storage.
func TestABackEdgeBuildReusesTheLoopsStorage(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// THE LIMB RUNG IS ASKED FOR: Go's own answer for a bounded big value is
	// `math/big` (targets/go/bigint.oro), where the reuse this test is about is
	// `bigreuse.go`'s and not this one's. Same program, different storage.
	tg.BigRepr = "limbs"
	src := `(export fact)
(sig fact ((n (int 0 50))) (int 0 (pow 2 300)))
(def fact (fn (n) (loop ((acc 1) (i 2)) (> i n) acc else (again (* acc i) (+ i 1)))))
`
	code := emitGo(t, tg, src, "fact")
	// One `make` for the initial buffer and one for the spare, both OUTSIDE the
	// loop; the allocating form has one inside it.
	if n := strings.Count(code, "make("); n != 2 {
		t.Errorf("%d allocations, want 2 — the spare is hoisted and the back edge "+
			"swaps into it:\n%s", n, code)
	}
	if !strings.Contains(code, "clear(") {
		t.Errorf("the spare is not cleared; `build` zero-fills (tables.md §14.3) "+
			"and a program may rely on it:\n%s", code)
	}
	// AND THE SWAP, not a plain assignment: `acc = o` alone would make the
	// accumulator and the spare the same buffer, and `clear` would wipe it
	// before every read. That is what the first version did, and it returned 0.
	// The IR's names are value numbers, so the swap is its shape: the
	// parameter and the spare exchanged in one parallel assignment,
	// `p, sp = new, p`.
	if !regexp.MustCompile(`(\w+), (sp\w+) = \w+, (\w+)`).MatchString(code) ||
		!swapped(code) {
		t.Errorf("no swap at the back edge:\n%s", code)
	}
}

// AND A LOOP WHOSE BUFFER IS READ SOMEWHERE ELSE KEEPS ITS ALLOCATION, which is
// condition (3): the old buffer has to be dead after the back edge, and another
// reader is the proof that it is not.
func TestABufferReadElsewhereIsNotReused(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export f)
(sig f ((n (int 0 20))) int)
(def at (fn (t i) (if (< i 0) 0 (if (>= i (len t)) 0 (t i)))))
(def f (fn (n)
  (loop ((b (build 8 (fn (x) x))) (s 0) (i 0))
    (>= i n)  s
    else      (again (build 8 (fn (o) (set o 0 (at b 0)))) (+ s (at b 1)) (+ i 1)))))
`
	code := emitGo(t, tg, src, "f")
	if strings.Count(code, "make(") != 2 || strings.Contains(code, "clear(") {
		t.Errorf("a buffer read outside its own back-edge argument was reused; "+
			"its old contents are still needed:\n%s", code)
	}
}

func emitGo(t *testing.T, tg *emit.Target, src, name string) string {
	t.Helper()
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
	nf, err := core.Normalize(prog.Defs[name], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	nf, _, err = emit.PromoteBig(tg, prog.Sigs[name], nf, allProgSigs(prog)...)
	if err != nil {
		t.Fatal(err)
	}
	nf, _ = emit.SelectShifts(tg, prog.Sigs[name], nf)
	code, err := golang.FromResidual(tg, name, prog.Sigs[name], nf)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func TestADividendProvedNonNegativeBecomesAShift(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export f)
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (+ (/ n 8) (% n 8))))
`
	code := emitGo(t, tg, src, "f")
	if !strings.Contains(code, ">> 3") || !strings.Contains(code, "& 7") {
		t.Errorf("a non-negative dividend did not become a shift and a mask:\n%s", code)
	}
}

// AND A DIVIDEND THAT MAY BE NEGATIVE DOES NOT, which is the rule this could
// most easily get wrong — and the differential suite could not catch it, because
// Go, the JVM and x86 would all shift and all be wrong together.
//
// An arithmetic shift is FLOOR division; our `/` truncates toward zero. They
// differ by one on every negative dividend: `-1 / 2` is 0 and `-1 >> 1` is −1.
func TestAPossiblyNegativeDividendStaysADivision(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export f)
(sig f ((n (int -1000 1000))) int)
(def f (fn (n) (/ n 8)))
`
	code := emitGo(t, tg, src, "f")
	if strings.Contains(code, ">>") {
		t.Errorf("a dividend that can be negative was rewritten to a shift, which "+
			"is floor division where ours truncates:\n%s", code)
	}
	if !strings.Contains(code, "/ 8") {
		t.Errorf("the division disappeared without becoming a shift:\n%s", code)
	}
}

// AND A TARGET THAT DECLARES NOTHING GETS NOTHING, which is the containment
// property for this pass and the reason a third-party target is safe by default.
func TestNoShiftWidthMeansNoRewrite(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	tg.ShiftWidth = 0
	src := `(export f)
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (/ n 8)))
`
	if code := emitGo(t, tg, src, "f"); strings.Contains(code, ">>") {
		t.Errorf("a target declaring no shift width was rewritten anyway:\n%s", code)
	}
}

func TestADeclaredWideningSurvivesInlining(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// `scaled` is not exported and `run` returns an ordinary `int`, so after
	// inlining nothing in the residual mentions a big type — except the
	// ascription.
	src := `(export run)
(sig scaled ((n (int 0 50))) (int 0 (pow 2 200)))
(def scaled (fn (n) (loop ((acc (+ n 7)) (i 0)) (>= i 6) acc else (again (* acc 999983) (+ i 1)))))
(sig run ((n (int 0 50))) int)
(def run (fn (n) (% (scaled n) 100000000)))
`
	code := emitGo(t, tg, src, "run")
	if !strings.Contains(code, "big.NewInt") || !strings.Contains(code, "Mul") {
		t.Errorf("the inlined accumulator was not promoted:\n%s", code)
	}
	// AND THE MARKER IS GONE. It carries a range and nothing else; the runtime
	// enforcement of the bound is `big-fit`, which is a different mechanism.
	if strings.Contains(code, core.AscribeName+"(") || strings.Contains(code, "int 0 16069") {
		t.Errorf("the ascription reached the backend:\n%s", code)
	}
}

// swapped: some parallel assignment `p, sp = x, p` gives the spare the old
// parameter, which is what keeps the two buffers apart.
func swapped(code string) bool {
	for _, m := range regexp.MustCompile(`(\w+), (sp\w+) = \w+, (\w+)`).FindAllStringSubmatch(code, -1) {
		if m[1] == m[3] {
			return true
		}
	}
	return false
}
