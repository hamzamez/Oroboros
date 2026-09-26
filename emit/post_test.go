package emit

import (
	"testing"

	"oroboros/core"
)

// THE LOOP SHAPE (tables-write-2026-08-25 §1, loopshape-2026-08-25).
//
// Our loops emitted `for { if guard { break }; …; i = i + 1; continue }` where a
// person writes `for i := 2; i*i < n; i++`. The increment was duplicated into
// every clause, so the loop had several back edges and Go's SSA did not see a
// counted loop — worth 1.4x on the sieve.
//
// A loop variable updated identically by every `again` moves into the `for`
// statement's post clause.

func mustRead(t *testing.T, src string) *core.Term {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	return forms[0].Term
}
