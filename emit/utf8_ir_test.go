package emit_test

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// VALID UTF-8 IS RECOGNIZED BY ITS MONOID (ADR 0112, strings.md §11).
// `where utf8_valid(b)` is ι_V's obligation, decided by φ, the bytes run
// through Table 3-7's automaton, on an array literal, and on anything else only
// by a guard `if utf8_valid(b)` on its arm. Each refusal names the predicate;
// the literals are the ill-formed cases text-of.oro decodes (an overlong, an
// encoded surrogate, a truncated sequence, a byte past F4).
func TestValidUTF8IsRecognizedByItsMonoid(t *testing.T) {
	const g = "fn g(b ∈ Array(0..255)) (String) where utf8_valid(b) { utf8_text(b) }\n"
	accepted := map[string]string{
		"ASCII":               `export fn h() (String) { g(array { 104, 105 }) }`,
		"a three-byte scalar": `export fn h() (String) { g(array { 230, 151, 165 }) }`,
		"the largest scalar":  `export fn h() (String) { g(array { 244, 143, 191, 191 }) }`,
		"a guard":             `export fn h(b ∈ Array(0..255)) (String) { if utf8_valid(b) { g(b) } else { "" } }`,
	}
	for what, src := range accepted {
		if err := legalGo(t, g+src); err != nil {
			t.Errorf("%s: want it accepted, got %v", what, err)
		}
	}
	refused := map[string]string{
		"an overlong":           `export fn h() (String) { g(array { 192, 175 }) }`,
		"an encoded surrogate":  `export fn h() (String) { g(array { 237, 160, 128 }) }`,
		"a truncated sequence":  `export fn h() (String) { g(array { 230, 151 }) }`,
		"a byte past F4":        `export fn h() (String) { g(array { 245, 128, 128, 128 }) }`,
		"an unknown table":      `export fn h(b ∈ Array(0..255)) (String) { g(b) }`,
		"the guard's other arm": `export fn h(b ∈ Array(0..255)) (String) { if utf8_valid(b) { "" } else { g(b) } }`,
	}
	for what, src := range refused {
		err := legalGo(t, g+src)
		if err == nil || !strings.Contains(err.Error(), "utf8_valid") && !strings.Contains(err.Error(), "utf8-valid") {
			t.Errorf("%s: want it refused naming utf8_valid, got %v", what, err)
		}
	}

	// THE PLANTED FAULT: a literal read as valid without φ.
	emit.PlantUTF8Fault = true
	defer func() { emit.PlantUTF8Fault = false }()
	if err := legalGo(t, g+refused["an overlong"]); err != nil {
		t.Errorf("with every literal read as valid the overlong should be accepted, so the test reaches φ; got %v", err)
	}
}
