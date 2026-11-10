package emit_test

import (
	"testing"

	"oroboros/emit"
)

// K(ℕ*) IS A THEORY CHECKED ONCE (ADR 0099, ADR 0035): every definition types,
// is linear, and has its indices and divisors discharged on windows, the
// target with no bignum that takes it.
func TestTheNumeralsOfAnyLengthCheckAsATheory(t *testing.T) {
	tg := winTarget(t)
	for _, fn := range []string{"of", "add", "sub", "mul", "mul-small", "div-small", "rem-small",
		"lt", "le", "gt", "ge", "eq", "render", "div", "rem", "fit", "fit-signed"} {
		if _, err := emit.NatFunc(tg, fn); err != nil {
			t.Errorf("%s: %v", fn, err)
		}
	}
}

