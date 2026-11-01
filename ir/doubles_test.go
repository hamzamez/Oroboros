package ir

import (
	"math"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// doublesOn lowers src on target through Entry, the pipeline every driver
// takes, and returns its functions or the first refusal.
func doublesOn(t *testing.T, target, src string) ([]*Func, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "d.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	var out []*Func
	err := eachEntry(path, target, func(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) error {
		f, _, err := Entry(tg, name, name, sig, nf, nil, false, nil)
		if err == nil {
			out = append(out, f)
		}
		return err
	})
	return out, err
}

// calls is every call's primitive in fs, in order.
func calls(fs []*Func) []string {
	var out []string
	for _, f := range fs {
		f.Walk(func(r *Region) {
			for i := range r.Stmts {
				if r.Stmts[i].Op == OCall {
					out = append(out, r.Stmts[i].Name)
				}
			}
		})
	}
	return out
}

const bounded = "set E = set { x ∈ F64 : -1e100 <= x && x <= 1e100 }\n"

// A ROUNDING IS ITS TARGET'S CALL (numbers.md §6.2): each of the four ring
// operations and ÷, and an integer's conversion, on each target, is the call
// of the declaration whose `ensure` states it, and nothing else.
func TestARoundingIsItsTargetsCall(t *testing.T) {
	src := bounded +
		"export fn add(a ∈ E, b ∈ E) (F64) { round(F64, a + b) }\n" +
		"export fn sub(a ∈ E, b ∈ E) (F64) { round(F64, a - b) }\n" +
		"export fn mul(a ∈ E, b ∈ E) (F64) { round(F64, b * a) }\n" +
		"export fn quo(a ∈ E, b ∈ set { x ∈ F64 : 1 <= x && x <= 2 }) (F64) { round(F64, a ÷ b) }\n" +
		"export fn conv(n ∈ 0..1000) (F64) { round(F64, n) }\n"
	for _, target := range []string{"go", "js", "java"} {
		fs, err := doublesOn(t, target, src)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		want := []string{".fadd", ".fint", ".fmul", ".fdiv", ".fsub"} // exports in sorted order
		got := calls(fs)
		if len(got) != len(want) {
			t.Fatalf("%s: calls %v, want %v", target, got, want)
		}
		for i := range want {
			if !strings.HasSuffix(got[i], want[i]) {
				t.Errorf("%s: call %d is %s, want %s", target, i, got[i], want[i])
			}
		}
	}
}

// fma is a realization of round(F64, a · b + c), which no target declares: a
// test's own, so the fused rounding can be told from two.
func withFMA(tg *emit.Target) func() {
	p := tg.Prims["go.fadd"]
	p.Name = "go.fma"
	p.Names = []string{"a", "b", "c"}
	p.Args = []string{core.F64Name, core.F64Name, core.F64Name}
	ts, err := core.ReadRaw("(= result (round \"f64\" (+ (* a b) c) \"nearest\"))")
	if err != nil || len(ts) != 1 {
		panic(err)
	}
	p.Ensures = ts[0]
	tg.Prims["go.fma"] = p
	realizationCache.Delete(tg)
	return func() {
		delete(tg.Prims, "go.fma")
		realizationCache.Delete(tg)
	}
}

// TWO ROUNDINGS ARE TWO OPERATIONS (numbers.md §6.2): with a fused
// multiply-add declared, round(F64, a·b + c) is it, and round(F64,
// round(F64, a·b) + c) is a product and a sum, never the fused operation, whose
// answer differs. Planted fault: the matcher looking through the inner
// rounding (plantFuseFault) makes the second fused, and this test fails.
func TestTwoRoundingsAreTwoOperations(t *testing.T) {
	run := func(body string) []string {
		path := filepath.Join(t.TempDir(), "f.oro")
		src := bounded + "export fn f(a ∈ E, b ∈ E, c ∈ E) (F64) { " + body + " }\n"
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		var got []string
		err := eachEntry(path, "go", func(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) error {
			defer withFMA(tg)()
			f, _, err := Entry(tg, name, name, sig, nf, nil, false, nil)
			if err != nil {
				return err
			}
			got = calls([]*Func{f})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := run("round(F64, a * b + c)"); len(got) != 1 || got[0] != "go.fma" {
		t.Errorf("round(F64, a * b + c) is %v, want the fused go.fma", got)
	}
	two := func() []string { return run("round(F64, round(F64, a * b) + c)") }
	if got := two(); len(got) != 2 || got[0] != "go.fmul" || got[1] != "go.fadd" {
		t.Errorf("round(F64, round(F64, a * b) + c) is %v, want go.fmul then go.fadd", got)
	}
	plantFuseFault = true
	defer func() { plantFuseFault = false }()
	for _, c := range two() {
		if c == "go.fma" {
			return // the fault is seen: the check above is what catches it
		}
	}
	t.Errorf("the planted fault (a rounding looked through) was not fused, so the check could not see it")
}

// A COMPARISON WITH A CONSTANT IS EXACT (doubles.go, the Galois connection):
// for x ∈ F64 and any rational q, x rel q holds exactly when galois's
// comparison with a double does, checked on the doubles next to q, where it
// matters. Planted fault: a strict bound kept strict against ⌊q⌋, which is
// false of x = ⌊q⌋.
func TestAComparisonWithAConstantIsExact(t *testing.T) {
	check := func() []string {
		var bad []string
		qs := []*big.Rat{
			big.NewRat(1, 10), big.NewRat(3, 10), big.NewRat(-1, 10), big.NewRat(1, 3), big.NewRat(5, 1),
			new(big.Rat).SetFloat64(0.5), new(big.Rat).SetFloat64(math.MaxFloat64),
			new(big.Rat).Mul(new(big.Rat).SetFloat64(math.MaxFloat64), big.NewRat(2, 1)),
			new(big.Rat).Neg(new(big.Rat).Mul(new(big.Rat).SetFloat64(math.MaxFloat64), big.NewRat(2, 1))),
			new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 1080)), // below every subnormal
		}
		for _, q := range qs {
			x0, _ := q.Float64()
			var xs []float64
			for _, x := range []float64{x0, math.Nextafter(x0, math.Inf(1)), math.Nextafter(x0, math.Inf(-1)), 0,
				math.MaxFloat64, -math.MaxFloat64, 5e-324} {
				if !math.IsInf(x, 0) {
					xs = append(xs, x)
				}
			}
			for _, rel := range []string{"lt", "le", "gt", "ge", "eq", "ne"} {
				nrel, c, k := galois(rel, q)
				for _, x := range xs {
					cmp := new(big.Rat).SetFloat64(x).Cmp(q)
					want := map[string]bool{"lt": cmp < 0, "le": cmp <= 0, "gt": cmp > 0, "ge": cmp >= 0,
						"eq": cmp == 0, "ne": cmp != 0}[rel]
					var got bool
					if k != nil {
						got = *k
					} else {
						got = map[string]bool{"lt": x < c, "le": x <= c, "gt": x > c, "ge": x >= c, "eq": x == c,
							"ne": x != c}[nrel]
					}
					if got != want {
						bad = append(bad, rel+" "+q.RatString())
					}
				}
			}
		}
		return bad
	}
	if bad := check(); len(bad) > 0 {
		t.Fatalf("a comparison with a constant is not exact: %v", bad)
	}
	plantGaloisFault = true
	defer func() { plantGaloisFault = false }()
	if bad := check(); len(bad) == 0 {
		t.Errorf("the planted fault (x < q as x < ⌊q⌋) went unseen")
	}
}

// A CONSTANT'S ROUNDING IS FOLDED, AND IT IS THE NEAREST DOUBLE, a tie to the
// even one (numbers.md §3, §4): 1 + 2⁻⁵³ is a tie between 1 and 1 + 2⁻⁵², and
// rounds to 1; 1 + 3·2⁻⁵³ ties between 1 + 2⁻⁵² and 1 + 2⁻⁵¹, and rounds to
// the second. A constant past F64 is refused.
func TestAConstantsRoundingIsTheNearestEven(t *testing.T) {
	cases := []struct {
		term string
		want float64
	}{
		{"(2^53 + 1) ÷ 2^53", 1},           // a tie, to the even one
		{"(2^53 + 3) ÷ 2^53", 1 + 0x1p-51}, // a tie, to the even one above
		{"1 ÷ 3", 1.0 / 3},
		{"0.1", 0.1},
		{"0.1 + 0.2", 0.3}, // exactly 3/10, rounded once: not IEEE's 0.30000000000000004
	}
	for _, c := range cases {
		fs, err := doublesOn(t, "go", "export fn f() (F64) { round(F64, "+c.term+") }\n")
		if err != nil {
			t.Errorf("%s: %v", c.term, err)
			continue
		}
		var got *core.Term
		fs[0].Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op == OConst && s.Lit.Kind == core.KDec {
					got = s.Lit
				}
			}
		})
		if got == nil || got.Dec.Double() != c.want {
			t.Errorf("round(F64, %s) is %v, want %v", c.term, got, c.want)
		}
	}
	if _, err := doublesOn(t, "go", "export fn f() (F64) { round(F64, 1e309) }\n"); err == nil ||
		!strings.Contains(err.Error(), "overflows F64") {
		t.Errorf("round(F64, 1e309) was not refused as an overflow: %v", err)
	}
}

// AN ACCUMULATOR IS BOUNDED BY ITS TRIP COUNT (doubles.go, Theorem A): a sum
// of bounded doubles is proven finite over any length the word counts; a
// doubling is not (no linear bound holds of it), and neither is a sum whose
// loop has no trip bound. Planted fault: a product by more than one taken as a
// link of the chain makes the doubling proven, which is false.
func TestAnAccumulatorIsBoundedByItsTripCount(t *testing.T) {
	sum := bounded + "export fn s(a ∈ Array(E)) (F64) { round(F64) { loop i = 0, s = 0 { i < len(a) -> again(i + 1, s + a[i]); else -> s } } }\n"
	if _, err := doublesOn(t, "go", sum); err != nil {
		t.Fatalf("a sum of bounded doubles was refused: %v", err)
	}
	double := "export fn d(n ∈ 0..2000) (F64) { loop i = 0, x = 1 { i < n -> again(i + 1, round(F64, x * 2)); else -> x } }\n"
	if _, err := doublesOn(t, "go", double); err == nil || !strings.Contains(err.Error(), "may overflow") {
		t.Errorf("x ← round(F64, x · 2) 2000 times was not refused: %v", err)
	}
	// (x < 1e300 -> again(round(F64, x + 1)) is proven, rightly: its guard bounds x.)
	untripped := "export fn u(b ∈ Bool) (F64) { loop x = 0 { b -> again(round(F64, x + 1e300)); else -> x } }\n"
	if _, err := doublesOn(t, "go", untripped); err == nil {
		t.Errorf("an accumulator with no trip bound was accepted")
	}
	plantChainFault = true
	defer func() { plantChainFault = false }()
	if _, err := doublesOn(t, "go", double); err == nil {
		return // the fault is seen
	}
	t.Errorf("the planted fault (a product by 2 as a link) still refused the doubling: the test cannot see it")
}

// A HOST'S DOUBLE IS NOT AN ELEMENT OF F64 (numbers.md §6.3): Go's own `/`,
// whose quotient by zero is ±∞ or NaN, may not stand where a rounding's
// operand or N's order would read it; it may stand where the host's own
// operations read it. Planted fault: the taint off, and 1 ÷ 0 reaches a
// rounding.
func TestAHostsDoubleIsNotF64(t *testing.T) {
	refused := []string{
		"use go\nexport fn f(a ∈ F64, b ∈ F64) (F64) { round(F64, go.`f/`(a, b) * 0.5) }\n",
		"use go\nexport fn f(a ∈ F64, b ∈ F64) (Bool) { go.`f/`(a, b) < 1 }\n",
	}
	for _, src := range refused {
		if _, err := doublesOn(t, "go", src); err == nil || !strings.Contains(err.Error(), "a host computed") {
			t.Errorf("a host's double read by the language's numbers was not refused: %v\n%s", err, src)
		}
	}
	tier2 := "use go\nexport fn f(a ∈ F64, b ∈ F64) (F64) { go.`f+`(go.`f/`(a, b), 1.0) }\n"
	if _, err := doublesOn(t, "go", tier2); err != nil {
		t.Errorf("a host's double kept to the host's operations was refused: %v", err)
	}
	plantHostFault = true
	defer func() { plantHostFault = false }()
	if _, err := doublesOn(t, "go", refused[0]); err == nil {
		return
	}
	t.Errorf("the planted fault (no taint) still refused: the test cannot see it")
}

// THE SCOPE ROUNDS N'S OPERATIONS AND NO OTHERS (numbers.md §3): in round(F64)
// { … } an index's `i + 1` is the integer it was, so it still indexes; a
// numeral not in F64 is rounded; a rounding the program wrote inside is its
// own, and with a fused multiply-add declared it is that.
func TestTheScopeRoundsNsOperationsOnly(t *testing.T) {
	src := bounded + "export fn f(a ∈ Array(E)) (F64) where len(a) > 2 { round(F64) { a[0 + 1] * 0.1 } }\n"
	fs, err := doublesOn(t, "go", src)
	if err != nil {
		t.Fatalf("the scope over an index: %v", err)
	}
	if got := calls(fs); len(got) != 1 || got[0] != "go.fmul" {
		t.Errorf("the scope's calls are %v, want one go.fmul", got)
	}
	path := filepath.Join(t.TempDir(), "s.oro")
	if err := os.WriteFile(path, []byte(bounded+"export fn g(a ∈ E, b ∈ E, c ∈ E) (F64) { round(F64) { round(F64, a * b + c) } }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err = eachEntry(path, "go", func(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) error {
		defer withFMA(tg)()
		f, _, err := Entry(tg, name, name, sig, nf, nil, false, nil)
		if err != nil {
			return err
		}
		if got := calls([]*Func{f}); len(got) != 1 || got[0] != "go.fma" {
			t.Errorf("a rounding written inside the scope is %v, want its own go.fma", got)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// F64 ∩ [lo, hi] AT A CALL IS AN OBLIGATION (ADR 0028), decided on the doubles'
// intervals: an unbounded argument is refused, and a guard that bounds it
// proves it. Its premise is computed: the same call under the guard passes.
func TestAnF64GridIsAnObligationAtACall(t *testing.T) {
	g := "set S = set { x ∈ F64 : 0 <= x && x <= 1 }\nfn g(x ∈ S) (Bool) { x < 2 }\n"
	if _, err := doublesOn(t, "go", g+"export fn f(y ∈ F64) (Bool) { g(y) }\n"); err == nil ||
		!strings.Contains(err.Error(), "not proven in it") {
		t.Errorf("an argument outside F64 ∩ [0, 1] was not refused: %v", err)
	}
	if _, err := doublesOn(t, "go", g+"export fn f(y ∈ F64) (Bool) { if y >= 0 && y <= 1 { g(y) } else { false } }\n"); err != nil {
		t.Errorf("a guarded argument was refused: %v", err)
	}
}
