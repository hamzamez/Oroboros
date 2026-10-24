package emit_test

import (
	"strings"
	"testing"
)

// These two pin a HOST `where`'s constant bounds, which are the parameter's
// range obligation (wherebounds.go, ADR 0062's bounds in binder sets): decided
// by a literal, the IR's intervals or the refinement layer, so through the
// pipeline that decides marks, not the refinement layer alone (refineGo erases
// them). Until wc-2026-10-24 both were refinement-layer tests.

// LEMMA 1: a guarantee holds only where its precondition is proven. With src
// unbounded, EncodedLen's own `where` (|n| ≤ 2^62 − 1) does not follow, so the
// program is refused there, and nothing is concluded from the result it would
// have had.
func TestAGuaranteeNeedsItsPrecondition(t *testing.T) {
	_, err := entryGo(t, `use go/encoding/hex
export fn f(src ∈ Array(0..255)) (Array(0..255)) {
    build(hex.EncodedLen(len(src)), fn(b) { (b2, n) = hex.Encode(b, src); b2 })
}`)
	if err == nil || !strings.Contains(err.Error(), "EncodedLen") {
		t.Errorf("len src may pass 2^62 − 1, where Go's EncodedLen wraps; got %v", err)
	}
}

// A HOST PRECONDITION IS THE HOST'S OWN BOUNDARY, once `int` at a host
// boundary is the host's integer (ADR 0026). `hex.EncodedLen` is n ↦ 2n on
// Go's `int`, [−2^63, 2^63−1], so it is exact precisely on
// [−2^62, 2^62−1] — asymmetric, because two's complement is. Under ADR 0012's
// window the declaration demanded |n| ≤ 2^52−1, a precondition Go does not
// have. Each edge is pinned from both sides.
func TestEncodedLenIsExactOnGosOwnBoundary(t *testing.T) {
	call := func(lo, hi string) error {
		_, err := entryGo(t, "use go/encoding/hex\nexport fn f(n ∈ "+lo+".."+hi+") (Int) { hex.EncodedLen(n) }")
		return err
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
