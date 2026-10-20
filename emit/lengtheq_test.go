package emit_test

import (
	"strings"
	"testing"
)

// A CONJUNCTION OF EQUATIONS IS ONE FACT, WHATEVER ITS ORDER AND ORIENTATION
// (emit/linear.go, assumeEQ; lengtheq-2026-10-03). The equations are a solved
// form, Gaussian elimination, and an equation is added to it. It was assigned:
// `len a = len out` and then `len b = len out`, both keyed under len out, and
// the second overwrote the first, so `(b i)` under `i < len a` was refused in
// one orientation of a true precondition and proven in the other.
func TestEquationsAreAddedNotOverwritten(t *testing.T) {
	prog := func(where string) string {
		return `use go
export fn add(out ∈ Array(F64) linear, a ∈ Array(F64), b ∈ Array(F64)) (buffer(F64)) where ` + where + ` {
    loop i = 0 {
        i >= len(a) -> ()
        else -> { out[i] := go.` + "`f+`" + `(a[i], b[i]); again(i + 1) }
    }
}`
	}
	for _, c := range []struct {
		where string
		ok    bool
	}{
		// len b = len a, in every orientation and order, through a third length
		{"len(a) == len(out) && len(b) == len(out)", true},
		{"len(out) == len(a) && len(b) == len(a)", true},
		{"len(a) == len(out) && len(out) == len(b)", true},
		{"len(out) == len(a) && len(out) == len(b)", true},
		{"len(b) == len(out) && len(a) == len(out)", true},
		// a redundant equation changes nothing
		{"len(a) == len(out) && len(b) == len(out) && len(out) == len(a)", true},
		// b longer than a is in range; b shorter is not
		{"len(a) == len(out) && len(b) == len(out) + 1", true},
		{"len(a) == len(out) && len(b) == len(out) - 1", false},
		// no unit coefficient: kept as two inequalities, which still decide
		{"len(a) == len(out) && 2 * len(b) == 2 * len(out)", true},
		{"len(a) == len(out) && 2 * len(b) == len(out)", false},
	} {
		_, err := entryGo(t, prog(c.where))
		if c.ok && err != nil {
			t.Errorf("%s: %v", c.where, err)
		}
		if !c.ok && (err == nil || !strings.Contains(err.Error(), "b[i] is an indexing")) {
			t.Errorf("%s: want b[i] refused, got %v", c.where, err)
		}
	}
}
