package emit

import (
	"testing"

	"oroboros/core"
)

// BOUNDED-BY-DEFAULT MUST NOT BE VACUOUS INSIDE A MULTI-RESULT CONTINUATION.
//
// `evalR` met an application whose operator is itself an application, did not
// recognise it, and returned ⊤ WITHOUT WALKING THE BODY — so the interval pass
// never entered any code written after a host call with several results, which
// is how every program that opens a file is written. The identical unbounded
// multiplication was REFUSED in an ordinary function and ACCEPTED silently in a
// continuation.
//
// Same failure mode as bigrep-2026-09-02's, which found bounded-by-default
// "enforced on three targets and vacuous on the fourth" — arriving at a
// CONSTRUCT this time instead of at a target. Found by writing a tool
// (jsonfmt-2026-09-07), one day after the construct shipped.
func TestBoundedByDefaultReachesIntoAContinuation(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// The same loop, once directly and once under `os.ReadFile`'s continuation.
	// `(go.* a n)` on an unbounded `n` cannot be proven in-window either way.
	loop := func(n string) string {
		return `(loop ((a 1) (k 0)) (go.>= k 40) a else (again (go.* a ` + n +
			`) (go.+ k 1)))`
	}
	for _, c := range []struct{ what, src string }{
		{"directly", `(fn (n) ` + loop("n") + `)`},
		{"in a continuation",
			`(fn () ((go/os.ReadFile "f") (fn (b e) ` + loop("(go.len b)") + `)))`},
	} {
		terms, err := core.ReadAll(c.src)
		if err != nil || len(terms) != 1 {
			t.Fatalf("%s: read: %v", c.what, err)
		}
		rep, _ := Intervals(tg, nil, terms[0], 0)
		if rep.Ops == 0 {
			t.Fatalf("%s: NOTHING WAS COUNTED. The walk never reached the "+
				"arithmetic, so ADR 0019 is vacuous here and any program "+
				"written in this shape is unchecked.", c.what)
		}
		if rep.Proven == rep.Ops {
			t.Errorf("%s: %d of %d proven; an unbounded multiply must not be",
				c.what, rep.Proven, rep.Ops)
		}
	}
}
