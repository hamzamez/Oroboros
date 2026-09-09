package emit

import (
	"strings"
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

// THE BOUNDS-CHECK NARROWING MUST NOT TRUNCATE ITS SOURCE.
//
// `emitNarrow` implements bounds-check elimination by restricting a container to
// the loop's bound, and it emitted `q = q[:n]` — writing back to the container
// itself. A loop bounded by LESS than the whole table therefore shortened it
// permanently, and every later `len(q)` saw the narrowed length.
//
// The emitter's justification — "narrowing moves the panic earlier, and no
// program with defined meaning can tell" — is true of INDEXING, which
// primitives.md §2 leaves unspecified out of range, and was never true of `len`,
// which is specified.
//
// Invisible for three weeks because every earlier narrow was to the container's
// OWN length, where the slice expression is the identity. A formatter copies a
// TOKEN out of a document, so the bound is that token's end: after the first
// string the document was 7 bytes long and the loop exited, printing a truncated
// file with no diagnostic.
func TestNarrowingDoesNotTruncateItsSource(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// A loop that copies a PREFIX of `src` — the bound is `m`, not `(len src)` —
	// and then asks for `(len src)` afterwards. If the narrowing wrote back, the
	// length read after the loop is the prefix's.
	src := `(fn (src m)
	          (go.+ (loop ((acc 0) (k 0))
	                  (go.>= k m) acc
	                  else (again (go.+ acc (src k)) (go.+ k 1)))
	                (go.len src)))`
	terms, err := core.ReadAll(src)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	out, err := Func(tg, "gen-copy", nil, terms[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(out, "\n") {
		s := strings.TrimSpace(line)
		// `src = src[:…]` in any form is the bug. A fresh destination is fine.
		if strings.HasPrefix(s, "src = src[:") {
			t.Fatalf("the narrowing writes back to its own source, so every "+
				"later len(src) sees the loop's bound:\n%s", out)
		}
	}
	if !strings.Contains(out, "len(src)") {
		t.Fatalf("this test needs the length read AFTER the loop, or it "+
			"cannot see the truncation:\n%s", out)
	}
}
