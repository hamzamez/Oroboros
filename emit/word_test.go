package emit

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
)

// withWord gives a test fixture the word the Go, JVM and windows target files
// declare, when the fixture says none. ADR 0026 makes the word required of
// every target — the compiler holds no default window — and a fixture about
// something else should not have to repeat it. A fixture that declares its own
// word, or tests its absence, is left alone.
func withWord(src string) string {
	if strings.Contains(src, " word)") || strings.Contains(src, "no-word") {
		return src
	}
	loc := targetHead.FindStringIndex(src)
	if loc == nil {
		return src
	}
	return src[:loc[1]] + " (repr (int -9223372036854775808 9223372036854775807) word)" + src[loc[1]:]
}

var targetHead = regexp.MustCompile(`\(target\s+[^\s()]+`)

// A HOST PRECONDITION IS THE HOST'S OWN BOUNDARY, once `int` at a host
// boundary is the host's integer (ADR 0026). `hex.EncodedLen` is n ↦ 2n on
// Go's `int`, [−2^63, 2^63−1], so it is exact precisely on
// [−2^62, 2^62−1] — asymmetric, because two's complement is. Under ADR 0012's
// window the declaration demanded |n| ≤ 2^52−1, a precondition Go does not
// have. Each edge is pinned from both sides.
func TestEncodedLenIsExactOnGosOwnBoundary(t *testing.T) {
	call := func(lo, hi string) error {
		return refineGo(t, "(use go/encoding/hex)\n(export f)\n"+
			"(sig f ((n (int "+lo+" "+hi+"))) int)\n(def f (n) (hex.EncodedLen n))")
	}
	for _, c := range []struct {
		lo, hi string
		ok     bool
	}{
		{"0", "4611686018427387903", true},   // 2^62 − 1
		{"0", "4611686018427387904", false},  // 2^62: 2n = 2^63 wraps in Go
		{"-4611686018427387904", "0", true},  // −2^62: 2n = −2^63 is Go's int
		{"-4611686018427387905", "0", false}, // one below
		{"0", "4503599627370496", true},      // 2^52: refused under the window, fine in Go
	} {
		if err := call(c.lo, c.hi); (err == nil) != c.ok {
			t.Errorf("EncodedLen on [%s, %s]: accepted = %v, want %v (%v)", c.lo, c.hi, err == nil, c.ok, err)
		}
	}
}

// A TUPLE COMPONENT'S RANGE IS A FACT, as a single result's is (mathbits-2026-09-23):
// Add64's carry-out is declared (int 0 1), so it satisfies the next Add64's
// `carry ≤ 1`. Before, the refinement layer bound a host call's several results
// with nothing known of them, and the most basic use of Add64 — a carry chain —
// was refused.
func TestATupleComponentsRangeIsAFact(t *testing.T) {
	err := refineGo(t, `(use go/math/bits as b)
(export f)
(sig f ((x (int 0 18446744073709551615)) (y (int 0 18446744073709551615))) (int 0 1))
(def f (x y) ((b.Add64 x y 0) (fn (s c) ((b.Add64 x y c) (fn (s2 c2) c2)))))`)
	if err != nil {
		t.Errorf("a carry chain must be proven: the first carry is (int 0 1): %v", err)
	}
	// ANTI-VACUITY: a carry-in the program cannot bound is still refused.
	err = refineGo(t, `(use go/math/bits as b)
(export f)
(sig f ((x (int 0 18446744073709551615)) (c (int 0 18446744073709551615))) (int 0 1))
(def f (x c) ((b.Add64 x x c) (fn (s o) o)))`)
	if err == nil {
		t.Error("an unbounded carry-in must be refused")
	}
}

// A PRIMITIVE IS APPLIED TO EXACTLY WHAT IT DECLARES. `(fmt.Println a b)` against
// a one-argument Println emitted `fmt.Println(a)`: b was dropped, and the program
// printed less than it said (mathbits-2026-09-23).
func TestAnExtraArgumentIsRefusedNotDropped(t *testing.T) {
	tg := goNative(t)
	nf := reduce(t, `(use go/fmt as fmt) (fn () (fmt.Println 1 2))`, "go")
	if err := Check(tg, "t", nf); err == nil || !strings.Contains(err.Error(), "takes 1 argument") {
		t.Errorf("an over-applied primitive must be refused by name; got %v", err)
	}
	ok := reduce(t, `(use go/fmt as fmt) (fn () (fmt.Println2 1 2))`, "go")
	if err := Check(tg, "t", ok); err != nil {
		t.Errorf("an exactly-applied primitive must be accepted: %v", err)
	}
}

// A REMAINDER BY A POSITIVE LITERAL IS AN ATOM of the fragment, as a quotient is,
// so the language's remainder facts (lang-facts.oro, F6–F8) reach a precondition.
// They were dead in the refiner: the fact matched, its conclusion was read, found
// outside the fragment, and nothing was assumed. So `hi < y` for
// `(b.Div64 (% h k) l k)` — the high word reduced mod the divisor, the step of
// every short division — was "propagated, not proven" and emitted anyway
// (u128-2026-09-23). Both spellings: h in the signed word, and h in U, where the
// goal says `u64%` and the fact's instance `%`, and each divisor is `(u64-of k)`.
func TestARemainderBoundsAPrecondition(t *testing.T) {
	for _, hi := range []string{"1000000", "18446744073709551615"} {
		notes, err := refineWorded(t, `(use go/math/bits as b)
(export f)
(sig f ((h (int 0 `+hi+`)) (l (int 0 18446744073709551615))) any)
(def f (h l) ((b.Div64 (% h 1000000000000000000) l 1000000000000000000) (fn (q r) q)))`)
		if err != nil || propagated(notes) {
			t.Errorf("h ≤ %s: h mod k < k must be PROVEN: err %v, notes %s", hi, err, notes)
		}
	}
	// ANTI-VACUITY: a remainder by a LARGER divisor bounds nothing useful, and
	// the precondition is not proven.
	notes, err := refineWorded(t, `(use go/math/bits as b)
(export f)
(sig f ((h (int 0 18446744073709551615)) (l (int 0 18446744073709551615))) any)
(def f (h l) ((b.Div64 (% h 1000000000000000001) l 1000000000000000000) (fn (q r) q)))`)
	if err == nil && !propagated(notes) {
		t.Errorf("h mod (k+1) < k does not follow and must not be proven: %s", notes)
	}
}

// refineWorded is refineGo with the unsigned word selected first and the type
// checker run on the result, as cmd/gen does when a signature declares a range
// in U.
func refineWorded(t *testing.T, src string) (string, error) {
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
	sig := prog.Sigs[q]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if nw, k := SelectWords(tg, sig, nf); k > 0 {
		nf = nw
	}
	if err := Check(tg, "test", nf); err != nil {
		return "", err
	}
	notes, err := Refine(tg, "test", sig, nf)
	return strings.Join(notes, "; "), err
}

// A LOOP VARIABLE THAT SHADOWS A PARAMETER gives the parameter back when its
// scope ends. `(loop ((h h)) …)` keeps the spelling — a parameter is not among
// the names the pass has bound — and the loop's exit DELETED h from the
// environment, taking the parameter's premise with it (u128-2026-09-23). Two
// witnesses: after the loop the parameter still bounds `(* h h)`; and a U
// parameter walked down by a loop of the same name keeps its representation,
// where the next sweep had read it as ⊤ and left `(= h 0)` to be refused by type.
func TestAShadowingLoopGivesTheParameterBack(t *testing.T) {
	tg := goNative(t)
	src := `(export f)
(sig f ((h (int 0 100))) int)
(def f (h) (+ (loop ((h h)) (= h 0) 1 else (again (/ h 10))) (* h h)))`
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
	if rep, _ := Intervals(tg, prog.Sigs["f"], nf, 0); rep.Proven != rep.Ops {
		t.Errorf("h ∈ [0, 100] after a loop that shadowed it: %d of %d proven, unproven %v",
			rep.Proven, rep.Ops, rep.Unproven)
	}
	// Through CheckSignatures, where it was met: the claim is checked on the
	// definition after the representation is chosen, and that pass sweeps the
	// loop twice.
	forms, err = core.Read(`(export f)
(sig f ((h (int 0 18446744073709551615))) int)
(def f (h) (loop ((h h)) (= h 0) 1 else (again (/ h 10))))`)
	if err != nil {
		t.Fatal(err)
	}
	if prog, _, err = core.Load(forms); err != nil {
		t.Fatal(err)
	}
	if env, err = tg.Env(prog); err != nil {
		t.Fatal(err)
	}
	if err := CheckSignatures(tg, prog, env); err != nil {
		t.Errorf("a U parameter shadowed by its loop must stay in U: %v", err)
	}
}
