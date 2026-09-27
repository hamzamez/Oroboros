package emit

import (
	"strings"
	"testing"
)

// THE LIMB LIBRARY CHECKS AS A THEORY (ADR 0035): every definition, at every
// width the corpus uses and at the extremes, is typed, linear and has every
// refinement obligation discharged under its precondition, on every target
// that takes the rung.
func TestTheLimbLibraryChecksAsATheory(t *testing.T) {
	for _, target := range []string{"go", "js", "java", "windows"} {
		tg, err := LoadTarget("../targets/" + target)
		if err != nil {
			t.Fatal(err)
		}
		for _, bits := range []int{54, 201, 1301, 4096} {
			n, lim := LimbWidth(bits)
			for fn := range limbSigs {
				if fn == "to-host" && !tg.HasBig() {
					continue // it names the host's bignum
				}
				if _, err := LimbFunc(tg, fn, n, lim); err != nil {
					t.Errorf("%s at %d bits: %v", target, bits, err)
				}
			}
		}
	}
}

// THE PRECONDITION IS WHAT DISCHARGES THE DIVISOR, and nothing else does: with
// it dropped, short division is refused, so the theory's check is not vacuous.
func TestALimbDivisorNeedsItsPrecondition(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	saved := limbSigs["div-small"]
	defer func() { limbSigs["div-small"] = saved }()
	bare := saved
	bare.divisor = false
	limbSigs["div-small"] = bare
	n, lim := LimbWidth(77) // a width no other test caches
	if _, err := limbFunc(tg, "div-small", n, lim); err == nil || !strings.Contains(err.Error(), "requires") {
		t.Errorf("short division without k ≠ 0: %v", err)
	}
}
