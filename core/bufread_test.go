package core

import (
	"strings"
	"testing"
)

// A BUFFER READ IS SUBSTITUTED PAST A STORE, AND THAT IS A SILENT WRONG ANSWER.
//
// ADR 0010 exists to forbid exactly this: an impure argument is never
// substituted, which denies contraction, weakening and EXCHANGE — no reordered
// store, and no read reordered across one. ADR 0018 says in as many words that
// `(array V)` reads are pure and `(buffer V)` reads are impure.
//
// They are not. `pureTerm` answers "value" for every bound variable, so an
// application whose operator is a bound variable is judged PURE — and a buffer
// read is exactly that shape. The smallest program that shows it is a SWAP:
//
//	(let (b 0) (fn (vx) (let (b 1) (fn (vy) (set (set b 0 vy) 1 vx)))))
//
// Both reads happen before either store, and the program is correct. Both are
// substituted into the store positions, and Go emits
//
//	b[0] = b[1]
//	b[1] = b[0]        ← reads what it just overwrote
//
// so a swap becomes a copy. Every target agrees, and all of them are wrong,
// which is why the differential suite cannot see it either.
//
// WHY IT WAS LATENT: it needs a read of a slot, then a store to that slot, then
// a USE of the read value. The tokeniser and the tree read a buffer constantly
// and consume each read inside the same expression as the store that follows.
// The first program to need it was a sort.
//
// WHY THE ONE-LINE FIX IS WRONG, recorded so it is not tried again: making an
// application of a bound variable impure kills FUSION. `(table n f)`'s rule
// reads its parameter table, so the whole rule-table becomes impure, is no
// longer substituted, and reaches the backend un-fused — `dot` and `smooth` on
// Java stop compiling. Measured: 2 of 164 emitted files improved, 2 vanished.
//
// The real fix is to know WHICH bound variables are buffers, which is
// `build`'s parameter threaded through `let`, `loop` and `set` — the same
// tracking `emit/winmap.go` does for the same reason, and it has to reach the
// reducer because purity is asked about an argument in isolation.
//
// This test asserts the CURRENT, WRONG behaviour so the bug is recorded and the
// fix has something to flip. When the fix lands, invert it.
func TestKnownBugBufferReadIsJudgedPure(t *testing.T) {
	e := &Env{Defs: map[string]*Term{}, Prim: map[string]bool{}, Pure: map[string]bool{}}
	for _, n := range []string{"build", "set", "let", "if"} {
		e.Prim[n] = true
	}
	e.Pure["build"], e.Pure["set"] = false, false // ADR 0018: both allocate or store

	forms, err := Read("(fn (b) (b 0))")
	if err != nil {
		t.Fatal(err)
	}
	read := forms[0].Term.Body() // `(b 0)`, with b a KBound

	if !e.pureTerm(read, map[string]bool{}) {
		t.Log("a buffer read is now judged impure — the bug is FIXED. " +
			"Invert this test and delete the `known bug` framing.")
		t.Fail()
	}
}

// And the property the fix must establish, stated as the theorem rather than as
// a spelling: READS BEFORE A STORE MUST STAY BEFORE IT.
//
// Written against the printed residual because that is where the reordering is
// visible; it fails today, so it is skipped rather than deleted — a test that
// only exists in a commit message is not a test.
func TestKnownBugReadsMustNotMovePastAStore(t *testing.T) {
	t.Skip("known bug: a buffer read is judged pure and is substituted past a " +
		"store, so a swap emits as a copy. See TestKnownBugBufferReadIsJudgedPure.")

	const prims = "(prim build)\n(prim set)\n(prim if)\n"
	got := norm(t, prims+"(build 2 (fn (b) (let (b 0) (fn (vx) "+
		"(let (b 1) (fn (vy) (set (set b 0 vy) 1 vx)))))))", "")
	// The reads must survive as bindings rather than be inlined into the stores.
	if strings.Count(got, "(b 0)") != 1 || strings.Count(got, "(b 1)") != 1 {
		t.Errorf("a buffer read was duplicated or moved: %s", got)
	}
}
