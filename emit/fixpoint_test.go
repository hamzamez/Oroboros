package emit

import (
	"testing"
)

// Every spelling of "how long is this" must key alike, or two mentions of one
// quantity are two variables and nothing composes. `len` is tables.md's
// structural name and was unrecognised: only the RETIRED portable layer's
// `alen`/`slen` were.
func TestIsLenOp(t *testing.T) {
	for _, n := range []string{"len", "go.len", "vec.alen", "alen", "str.slen"} {
		if !isLenOp(n) {
			t.Errorf("%q must be recognised as a length", n)
		}
	}
	for _, n := range []string{"length", "go.+", "table"} {
		if isLenOp(n) {
			t.Errorf("%q must not be", n)
		}
	}
}
