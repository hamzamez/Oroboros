package emit

import "oroboros/core"

// Connective recognises a conditional that is a boolean connective.
//
// `and`, `or` and `not` are reader sugar over `if` (booleans.md §4.2), so by the
// time a backend sees them they are conditionals with a boolean LITERAL in one
// branch:
//
//	(if p q false)   and
//	(if p true q)    or
//	(if p false true) not
//
// Every host with expressions has an operator for each, and emitting the
// conditional instead would be lowering further than the target requires —
// CLAUDE.md's most-cited failure mode, and the objection that kept `and` a
// primitive in ADR 0012. The recognition is a one-node match on a literal, not
// an analysis, which is what makes answering that objection cheap.
//
// Measured: neither form is faster on Go or V8
// (gauntlet/results/and-form-2026-08-19.md). So this is for the legibility of
// emitted code, and legibility is a requirement.
type Connective struct {
	Op   string // "and", "or", or "not"
	Args []*core.Term
}

func connective(tgt *Target, t *core.Term) (Connective, bool) {
	if t.Kind != core.KApp || t.Op().Kind != core.KName {
		return Connective{}, false
	}
	if p, ok := tgt.Prims[t.Op().Name]; !ok || p.Kind != "cond" {
		return Connective{}, false
	}
	a := t.Args()
	if len(a) != 3 {
		return Connective{}, false
	}
	lit := func(k *core.Term, want bool) bool {
		return k.Kind == core.KBool && k.IsTrue() == want
	}
	switch {
	case lit(a[1], false) && lit(a[2], true):
		return Connective{Op: "not", Args: []*core.Term{a[0]}}, true
	case lit(a[2], false):
		return Connective{Op: "and", Args: []*core.Term{a[0], a[1]}}, true
	case lit(a[1], true):
		return Connective{Op: "or", Args: []*core.Term{a[0], a[2]}}, true
	}
	return Connective{}, false
}
