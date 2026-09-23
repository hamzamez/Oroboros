package emit

import (
	"regexp"
	"strings"
	"testing"
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
