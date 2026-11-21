package llvm

import (
	"strings"
	"testing"
)

// A SIGNED QUOTIENT OF E BY A CONSTANT THAT IS NOT A POWER OF TWO IS THE
// MAGNITUDE'S UNSIGNED ONE (divMagnitude): LLVM lowers `udiv i128` and `urem
// i128` by a constant to multiplications on x86-64, and `sdiv i128` and `srem
// i128` to calls of __divti3 and __modti3, which no windows program links (no
// C runtime). plantDivTI3 prints the signed ones again.
func TestASignedQuotientOfEIsTheMagnitudes(t *testing.T) {
	srcs := []string{
		"export fn q7(y ∈ -2^63..2^63-1) (-2^63..2^63-1) { y * 5 / 7 }\n",
		"export fn m7(y ∈ -2^63..2^63-1) (0..6) { y * 3 mod 7 }\n",
		"export fn r7(y ∈ -2^63..2^63-1) (-2^63..2^63-1) { y * 3 % 7 }\n",
	}
	signedBySeven := func(ll string) bool {
		for _, l := range strings.Split(ll, "\n") {
			l = strings.TrimSpace(l)
			if (strings.Contains(l, "sdiv i128") || strings.Contains(l, "srem i128")) && strings.HasSuffix(l, ", 7") {
				return true
			}
		}
		return false
	}
	for _, src := range srcs {
		ll, err := compile(t, src)
		if err != nil {
			t.Fatal(err)
		}
		if signedBySeven(ll) || !strings.Contains(ll, "div i128") && !strings.Contains(ll, "rem i128") {
			t.Errorf("by 7: want the magnitude's unsigned division and no signed i128 one:\n%s", ll)
		}
		plantDivTI3 = true
		ll, err = compile(t, src)
		plantDivTI3 = false
		if err != nil || !signedBySeven(ll) {
			t.Errorf("with LLVM's own signed i128 division, want one by 7: %v\n%s", err, ll)
		}
	}
}
