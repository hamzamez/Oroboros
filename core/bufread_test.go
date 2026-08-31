package core

import (
	"strings"
	"testing"
)

// A TABLE READ IS NOT SUBSTITUTED INTO AN IMPURE BODY.
//
// ADR 0010 denies exchange — no store reordered, and no read reordered across
// one — and ADR 0018 says `(buffer V)` reads are impure where `(array V)` reads
// are pure. They were not: `pureTerm` answers "value" for every bound variable,
// so `(b 0)` was judged pure and could be moved. The smallest program that
// shows it is a SWAP:
//
//	(let (b 0) (fn (vx) (let (b 1) (fn (vy) (set (set b 0 vy) 1 vx)))))
//
// Both reads happen before either store and the program is correct. Both were
// substituted into the store positions, and Go emitted
//
//	b[0] = b[1]
//	b[1] = b[0]        ← reads what it just overwrote
//
// so a swap became a copy — on every target, which is why the differential
// suite could not see it either.
//
// WHY IT WAS LATENT: it needs a read of a slot, then a store to that slot, then
// a USE of the read value. The tokeniser and the tree read buffers constantly
// and consume each read inside the same expression as the store that follows.
// The first program to need it was a sort, for a map's `keys`.
//
// THE FIX TESTS THE DESTINATION, NOT THE OPERAND. Nothing in a term says which
// bound variables are buffers — an array read has the identical shape and is
// genuinely pure — so a read may move freely into a body with no effects to be
// reordered against, and not into one that has. That is exactly the property at
// stake, and it is decidable at the β site because the body is in hand.
//
// Testing the operand instead — "any application of a bound variable is impure"
// — was tried, measured and is wrong: a rule-table's rule reads its parameter
// table, so `(table n f)` becomes impure, stops being substituted, and reaches
// the backend UNFUSED. `dot` and `smooth` on Java stop compiling.
func TestATableReadDoesNotMoveIntoAnImpureBody(t *testing.T) {
	prims := "(prim build)\n(prim !set)\n(prim if)\n"

	// The swap. Both reads must survive as bindings rather than be inlined into
	// the stores, so the property is about WHERE the reads are rather than about
	// a count.
	got := norm(t, prims+"(fn (b) (let (b 0) (fn (vx) (let (b 1) (fn (vy) "+
		"(set (set b 0 vy) 1 vx))))))", "")
	i := strings.Index(got, "(set")
	if i < 0 {
		t.Fatalf("expected the stores to survive, got %s", got)
	}
	if stores := got[i:]; strings.Contains(stores, "(b 0)") || strings.Contains(stores, "(b 1)") {
		t.Errorf("a buffer read was substituted into a store position, so a "+
			"swap becomes a copy:\n%s", got)
	}
}

// AND THE RULE IS NARROW: into a PURE body a table read still moves, which is
// what keeps fusion working. Without this control the test above would pass
// against a compiler that had simply stopped substituting anything.
func TestATableReadStillMovesIntoAPureBody(t *testing.T) {
	prims := "(prim add)\n(prim if)\n"
	// ONE occurrence of x, deliberately: with two, call-by-need binds it
	// whatever its purity, and the control would pass for the wrong reason.
	got := norm(t, prims+"(fn (a) (let (a 0) (fn (x) (add x 1))))", "")
	if strings.Contains(got, "let") {
		t.Errorf("a table read was bound rather than substituted into a PURE "+
			"body; that is what un-fuses a rule-table:\n%s", got)
	}
}

// And an ordinary argument is unaffected: the rule fires only on a term that
// READS a table through a bound variable, not on every argument to an impure
// body.
func TestAnOrdinaryArgumentIsUnaffected(t *testing.T) {
	prims := "(prim !set)\n(prim add)\n(prim if)\n"
	got := norm(t, prims+"(fn (b n) (let (add n 1) (fn (x) (set b 0 x))))", "")
	if strings.Contains(got, "let") {
		t.Errorf("a pure arithmetic argument was bound rather than substituted "+
			"into an impure body, so the rule is wider than it should be:\n%s", got)
	}
}
