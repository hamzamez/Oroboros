package emit

import (
	"strings"
	"testing"
)

// A TABLE'S LENGTH IS ITS SIZE, WHATEVER ITS FILL (refinements.md §5b,
// localtab-2026-10-14). `(table (+ n 1) 7)` reads as
// `(let (+ n 1) (fn (#n) (build #n …fill loop…)))`, so the size is a binder of
// the value, and the length the walk returned was `#n`, a variable no fact
// outside constrains. The zero fill has no binder and was proven.
func TestAFilledTableHasItsSizeAsLength(t *testing.T) {
	prog := func(fill, idx string) string {
		return "export fn f(n ∈ 1..100) (Int) {\n    src = { var s := array { " + fill + " ∈ 0..n+1-1 }; s }\n    src[" + idx + "]\n}"
	}
	for _, fill := range []string{"0 : _", "7 : _", "i : i"} {
		if err := refineGo(t, prog(fill, "n")); err != nil {
			t.Errorf("fill %s: len src = n + 1, so (src n) is in range: %v", fill, err)
		}
		if err := refineGo(t, prog(fill, "n + 1")); err == nil || !strings.Contains(err.Error(), "is an indexing") {
			t.Errorf("fill %s: (src (+ n 1)) is one past the end and must be refused: %v", fill, err)
		}
	}
}

// A LENGTH NAMES NO BINDER OF ITS VALUE (refinements.md §5b, the subsection of
// that name). Two filled tables are both `(let SIZE (fn (#n) (build #n …)))`, siblings that keep one spelling
// (ADR 0036 names apart only along the scope chain), so a length read as `#n`
// equated len src with len dst. The witness was accepted, and on Go it panics:
// index out of range [5] with length 2 at n = 1, k = 5.
func TestALengthNamesNoBinderOfItsValue(t *testing.T) {
	prog := func(guard string) string {
		return "export fn f(n ∈ 1..100, k ∈ 0..1000) (Int) {\n" +
			"    src = { var s := array { 7 : _ ∈ 0..n+1-1 }; s }\n" +
			"    dst = { var d := array { 7 : _ ∈ 0..n+5-1 }; d }\n" +
			"    if k < len(" + guard + ") { src[k] } else { 0 }\n}"
	}
	if err := refineGo(t, prog("dst")); err == nil || !strings.Contains(err.Error(), "src[k] is an indexing") {
		t.Errorf("k < len dst = n + 5 does not bound a read of src, n + 1 long: %v", err)
	}
	if err := refineGo(t, prog("src")); err != nil {
		t.Errorf("k < len src is the proof: %v", err)
	}
	// PLANTED: the count read as written, so the binder escapes into the length.
	// The test must see the wrong proof it makes.
	plantLenEscapeFault = true
	defer func() { plantLenEscapeFault = false }()
	if err := refineGo(t, prog("dst")); err != nil {
		t.Errorf("the planted fault (a count read as written) proved nothing, so this test cannot see it: %v", err)
	}
}
