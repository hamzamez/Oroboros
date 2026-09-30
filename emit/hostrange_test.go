package emit_test

import (
	"strings"
	"testing"
)

// A HOST FUNCTION'S PARAMETER RANGE IS AN OBLIGATION AT ITS CALLS (ADR 0028,
// refinements.md §6b, gotarget-2026-09-30). `bits.Len32`'s argument is a
// uint32, [0, 2^32 − 1], and its template converts with uint32(%s): the range
// was read as a type only, so (bits.Len32 x) with x : int compiled to
// bits.Len32(uint32(x)) and answered 32 for −1.
func TestAHostParametersRangeIsAnObligation(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(use go/math/bits) (export f) (sig f ((x int)) int) (def f (x) (bits.Len32 x))`,
			"go/math/bits.Len32's parameter x, declared (int 0 4294967295), requires"},
		{`(use go/math/bits) (export f) (sig f () int) (def f () (bits.Len32 -1))`,
			"go/math/bits.Len32's parameter x is declared (int 0 4294967295), and a call passes -1"},
		{`(use go/os) (export f) (sig f ((d (array (int 0 255)))) bool) (def f (d) (os.err-nil (os.WriteFile "p" d -1)))`,
			"go/os.WriteFile's parameter perm is declared (int 0 4294967295), and a call passes -1"},
	} {
		_, err := dischargeGo(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s\n  want %q, got %v", c.src, c.want, err)
		}
	}
	// …and a value the program's own facts keep inside the range passes.
	for _, src := range []string{
		`(use go/math/bits) (export f) (sig f ((x (int 0 5))) int) (def f (x) (bits.Len32 x))`,
		`(use go/os) (export f) (sig f ((d (array (int 0 255)))) bool) (def f (d) (os.err-nil (os.WriteFile "p" d 420)))`,
	} {
		if _, err := dischargeGo(t, src); err != nil {
			t.Errorf("%s: %v", src, err)
		}
	}
}

// A SHIFT'S DOMAIN IS b ≥ 0 (the Go specification: a negative count panics).
// Declared as a `where`, it is decided like `/`'s b ≠ 0: by a literal, by a
// guard, or refused.
func TestAGoShiftCountMustBeNonNegative(t *testing.T) {
	head := "(use go) (export f) (sig f ((x (int 0 1000)) (n int)) int) "
	for _, c := range []struct {
		body string
		ok   bool
	}{
		{"(def f (x n) (go.<< x 3))", true},
		{"(def f (x n) (if (>= n 0) (go.>> x n) 0))", true},
		{"(def f (x n) (go.<< x -1))", false},
		{"(def f (x n) (go.>> x n))", false},
	} {
		_, err := entryGo(t, head+c.body)
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.body, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "requires")) {
			t.Errorf("%s: want the shift's domain refused, got %v", c.body, err)
		}
	}
}
