package emit_test

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
)

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
	tg, err := emit.LoadTarget("../targets/go")
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
	out, err := golang.FromResidual(tg, "gen-copy", nil, terms[0])
	if err != nil {
		t.Fatal(err)
	}
	// The parameter's printed name: the IR numbers its values.
	m := regexp.MustCompile(`func GenCopy\((v\d+) `).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no function:\n%s", out)
	}
	src0 := m[1]
	// `src = src[:…]` in any form is the bug. In SSA a restriction is a new
	// value, so the IR cannot write back; the check stays as the witness.
	if strings.Contains(out, src0+" = "+src0+"[:") {
		t.Fatalf("the narrowing writes back to its own source, so every "+
			"later len(src) sees the loop's bound:\n%s", out)
	}
	if !strings.Contains(out, "len("+src0+")") {
		t.Fatalf("this test needs the length read AFTER the loop, or it "+
			"cannot see the truncation:\n%s", out)
	}
}
