package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A LENGTH IS BOUNDED AT BOTH ENDS, and there are two separate claims in that.
//
//	(1) THE LANGUAGE'S OWN BOUND, needing no declaration. `(len t)` returns an
//	    `int` and ADR 0012 says `int` is exact within ±(2^53−1), so a table with
//	    more elements has a length this language cannot count. Assuming
//	    `(len t) ≤ 2^53−1` assumes nothing ADR 0012 did not already require.
//
//	(2) A TARGET'S TIGHTER BOUND. A Java array holds at most 2^31−1 elements,
//	    because `arraylength` returns an `int`.
//
// The two do different work and a test for one would pass without the other, so
// this pins both on the same program: the counter is BOUNDED on every target,
// and it FITS AN INDEX only where a target said something tighter. Before this,
// `(len a)` was `[0, +inf)` and the counter was unbounded everywhere — 32 of
// the corpus's unproven operations, every one of them this exact shape.
func TestLengthIsBounded(t *testing.T) {
	for _, c := range []struct {
		dir, op   string
		fitsIndex bool
	}{
		{"../targets/go", "go", false},    // the language's bound: 2^53−1
		{"../targets/java", "java", true}, // declared: 2^31−1
	} {
		tg, err := LoadTarget(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		src := "(fn (a) (loop ((i 0)) (" + c.op + ".>= i (len a)) i " +
			"else (again (" + c.op + ".+ i 1))))"
		terms, err := core.ReadAll(src)
		if err != nil || len(terms) != 1 {
			t.Fatalf("%s: read: %v", c.op, err)
		}
		rep, _ := Intervals(tg, nil, terms[0], 0)

		if rep.Ops == 0 {
			t.Fatalf("%s: nothing was counted, so this test proves nothing", c.op)
		}
		if rep.Proven != rep.Ops {
			t.Fatalf("%s: %d of %d operations bounded, want all of them; a "+
				"counter under a `len` guard is bounded because a LENGTH is",
				c.op, rep.Proven, rep.Ops)
		}
		if got := rep.FitsIndex(); got != c.fitsIndex {
			t.Fatalf("%s: FitsIndex is %v, want %v (MaxOp %s). The language's "+
				"own bound is 2^53−1, which does NOT fit a 32-bit index; only a "+
				"target that declared something tighter gets one",
				c.op, got, c.fitsIndex, rep.MaxOpRange())
		}
	}
}

// A DECLARED BOUND MUST BE TIGHTER THAN THE LANGUAGE'S, not looser. A target
// claiming it can hold more elements than an `int` can count is claiming a
// length the language cannot represent, which is not a length.
func TestMaxLenBeyondTheWindowIsRefused(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if got := tg.MaxLenOf(); got != portableMaxLen {
		t.Fatalf("a target that declares no max-len gets the language's own "+
			"bound; got %d, want %d", got, portableMaxLen)
	}
	jv, err := LoadTarget("../targets/java")
	if err != nil {
		t.Fatal(err)
	}
	if got := jv.MaxLenOf(); got != 2147483647 {
		t.Fatalf("java declares max-len 2147483647; got %d", got)
	}

	form, err := core.ReadAll("(target bad (max-len 9007199254740992))")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseTarget(form[0], "bad.oro"); err == nil {
		t.Fatal("a max-len past the portable window must be refused")
	} else if !strings.Contains(err.Error(), "portable window") {
		t.Fatalf("the refusal should say why: %v", err)
	}
}
