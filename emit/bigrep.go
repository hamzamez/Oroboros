package emit

import (
	"fmt"
	"math/big"

	"oroboros/core"
)

// ARBITRARY PRECISION, SELECTED — ADR 0019's THIRD ESCAPE.
//
// ADR 0019 says an integer operation the compiler cannot prove stays inside the
// portable window is a compile error, cleared by saying one of three things:
// narrow the range, ask for the trap, or DECLARE A RANGE ABOVE THE WINDOW,
// which promotes that value to arbitrary precision. The refusal in bounded.go
// has been offering that third escape in its own error message since the day it
// was written, and nothing implemented it. This is it.
//
// ═══ WHAT IS DECIDED HERE
//
// A representation, out of a two-point lattice `word ⊑ big` with `big`
// absorbing — precision-by-declaration.md's own proposal, and the reason it
// argued the interval domain does NOT have to go arbitrary-precision: an
// operation on a value that is already exact cannot overflow, so the analysis is
// never asked about it. `emit/interval.go`'s `int64` arithmetic is untouched.
//
// The soundness rule is one line: an operation is an error only if EVERY operand
// is word-represented and its result cannot be proven to fit. A big operation is
// therefore not an operation the window accounting sees at all, which is why
// this needed no change to `record`, to `Unbounded`, or to the report — a big
// primitive's declared result is `big`, `transfer` already returns "not
// checkable" for a result that is not `int`, and the operation simply is not
// counted.
//
// ═══ WHY IT MUST BE BIDIRECTIONAL, WITH FACTORIAL AS THE WITNESS
//
// precision-by-declaration.md named this and it is the whole difficulty:
//
//	(sig fact ((n (int 0 30))) (int 0 (pow 2 110)))
//
// every INPUT is small and the accumulator reaches 30! ≈ 2.65×10³². Nothing
// flowing forward makes `acc` big; the pressure comes from the declared RESULT.
// A forward-only solver passes the entire current corpus and fails on the first
// factorial.
//
// So a name is big if it is in the least set B closed under three rules:
//
//	(S) SUPPLY      — some expression assigned to it is big.
//	(D) DEMAND      — it is returned where a big result is declared, or it is
//	                  assigned to a loop variable that is already big.
//	(P) PRESSURE    — a big operation reads it AND it is not provably inside
//	                  the window.
//
// (S) and (D) are the two directions. (P) is the one that is easy to get wrong
// in either direction, and the gate is exactly right: in `fact`, the counter `i`
// is read by the big multiply and IS provably 1..31, so it stays a machine word
// and is widened at the call — which is what keeps a bignum loop from having a
// bignum loop counter. In `power`, `x` is read by the big multiply and its own
// `(* x x)` is unbounded, so it is promoted, which is the only way that program
// can be right.
//
// The system is monotone over a finite lattice, so the least fixed point is
// reached by iteration and the iteration terminates. Promoting too much costs
// speed; promoting too little is caught by the existing refusal. That is the
// same safety direction as the interval analysis's γ-soundness — containment,
// never tightness — and it is chosen for the same reason.
//
// ═══ WIDENING IS THE ONLY IMPLICIT CONVERSION
//
// `int ⊆ big`, so a word value may be widened where a bignum is wanted and this
// pass inserts `big-of` to do it. The other direction is REFUSED, and that
// refusal is unbounded-rung.md §3 — the promotion is a widening rather than a
// refinement, so it is where a programmer finds out a value became a bignum.
// `core.ValueType` returning `big` is what makes the type checker say so.

// langArith and langCompare are the LANGUAGE's integer operators — the ones
// integers.md promoted because all four hosts agree on them inside the window.
//
// A TARGET-NATIVE NAME IS NOT PROMOTED. `go.+` is Go's own `+` with Go's own
// semantics and no portability claim; silently turning it into a `math/big`
// call would be changing what a target's own name means, which is the parasite
// model backwards. A program that wants arbitrary precision writes the
// language's `+`, which it can, since integers.md.
func langArith(name string) bool {
	switch name {
	case "+", "-", "*", "/", "%":
		return true
	}
	return false
}

func langCompare(name string) bool {
	switch name {
	case "<", "<=", ">", ">=", "=":
		return true
	}
	return false
}

var bigOpNames = map[string]bool{
	"big+": true, "big-": true, "big*": true, "big/": true, "big%": true,
	"big<": true, "big<=": true, "big>": true, "big>=": true, "big=": true,
	"big-of": true, "big-of-small": true, "big-str": true,
}

func isBigOp(name string) bool { return bigOpNames[name] }

// bigTerm decides whether an expression is arbitrary-precision. It DECIDES
// rather than observes: an unrewritten `(* acc i)` with `acc` big is big, which
// is what lets the fixpoint see through a term the rewrite has not reached yet.
//
// A `let` body is deliberately answered NO. Seeing through one would mean
// opening its binder here, and every fresh name this pass hands out has to be
// the same one the evaluation hands out — the naming hazard that has bitten
// this compiler twice. Refusing to claim big is the safe direction: the widen
// is inserted instead, and a widen of something already big is the only cost.
func (p *intervalPass) bigTerm(t *core.Term) bool {
	if t == nil || p.big == nil {
		return false
	}
	// A `let` OR A `loop` REPORTS ITSELF, because neither can be seen through
	// from outside: the one has a binder this pass may not open a second time,
	// and the other's value is spread over its exit clauses. Both are recorded
	// by the method that REBUILDS them, keyed by the rebuilt term itself, which
	// is exact — there is no naming to reproduce and no key to guess.
	//
	// It has to be exact rather than conservative, and that is the difference
	// from the `let` note below: a term wrongly called word gets wrapped in
	// `big-of`, which is a type error rather than a missed optimisation.
	if p.bigVal[t] {
		return true
	}
	switch t.Kind {
	case core.KName:
		return p.big[t.Name]
	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return false
		}
		if isBigOp(op.Name) {
			return op.Name != "big-str"
		}
		if langArith(op.Name) {
			if constBigValue(t) {
				return true
			}
			for _, a := range t.Args() {
				if p.bigTerm(a) {
					return true
				}
			}
			return false
		}
		if pr, ok := p.tgt.Prims[op.Name]; ok {
			if pr.Result == core.BigType {
				return true
			}
			// AN `if` IS BIG WHEN EITHER BRANCH IS, which is the join in the
			// representation lattice. `cond` then makes the other branch agree,
			// because no host can type a conditional whose arms are a machine
			// word and a bignum.
			if pr.Kind == "cond" && len(t.Args()) == 3 {
				return p.bigTerm(t.Args()[1]) || p.bigTerm(t.Args()[2])
			}
		}
	}
	return false
}

// widenTo wraps a rebuilt term in `big-of` unless the original was already big.
func (p *intervalPass) widenTo(orig, rebuilt *core.Term, vals []ival, i int) *core.Term {
	if p.bigTerm(orig) {
		return rebuilt
	}
	// A WIDENED WORD THAT IS SMALL ENOUGH TO BE A MULTIPLIER gets its own NAME,
	// because the fixed-limb rung's one-pass multiply needs the fact and cannot
	// tell from the shape. `limb * k + carry` must stay inside the window: a
	// limb is under 2^24 and a carry under 2^25, so k under 2^28 leaves
	// 2^52 + 2^25.
	//
	// Getting it wrong is a WRONG ANSWER rather than a slow one — a big
	// literal's Horner spine multiplies by 10^15, and `mul-small` at that
	// multiplier overflows every column.
	//
	// A NAME AND NOT A MAP KEYED BY THE TERM, which was the first attempt and
	// the fourth time this compiler has been caught by a rebuild: `core.Fn`
	// closes a loop body by reconstructing it, so every pointer inside the map
	// was stale before the lowering ever saw it. A name survives.
	//
	// It appears only on the limb rung, so the host rung never sees a second
	// spelling of its widening.
	if p.limbs && i < len(vals) {
		v := vals[i]
		if !v.loInf && !v.hiInf && v.lo >= 0 && v.hi < 1<<28 {
			return core.App(core.Name("big-of-small"), rebuilt)
		}
	}
	return core.App(core.Name("big-of"), rebuilt)
}

// selectBig is the rewrite: a language operator with a big operand becomes the
// target's own arbitrary-precision form, and every word operand is widened.
//
// The second condition — sitting in a position that demands a bignum — is what
// covers a body that is not a loop. `(sig sq ((n int)) (int 0 (pow 2 70)))` with
// body `(* n n)` has no binder to promote and no loop to iterate; the demand
// arrives straight from the signature. The same rule reads `(big-str e)`, whose
// declared argument is `big`, which is how a whole program asks — see the note
// on inlining in `app`.
func (p *intervalPass) selectBig(t *core.Term, kids []*core.Term, vals []ival) (*core.Term, bool) {
	// `shiftOnly` runs AFTER a representation has been chosen, so re-selecting
	// here would promote what is already promoted.
	if p.big == nil || !p.bigOK() || !p.selecting || p.shiftOnly {
		return nil, false
	}
	op := t.Op()
	arith, cmp := langArith(op.Name), langCompare(op.Name)
	if !arith && !cmp {
		return nil, false
	}
	args := t.Args()
	if len(args) != 2 || len(kids) != 3 {
		return nil, false
	}
	byOperand := p.bigTerm(args[0]) || p.bigTerm(args[1]) || constBigValue(t)
	// A COMPARISON IS NEVER FORCED BY ITS POSITION, only by its operands: it
	// yields a bool, so a big DEMAND on the surrounding position says nothing
	// about it.
	if !byOperand && !(arith && p.demandBig) {
		return nil, false
	}
	bigName := BigOpName(op.Name)
	// ON THE LIMB RUNG THE NAME NEED NOT BE A PRIMITIVE. `big+` is an
	// intermediate here — `LowerLimbs` replaces every one of them with the
	// library before anything else looks — so demanding that the target declare
	// it would exclude the one host the rung exists for.
	if _, ok := p.tgt.Prims[bigName]; !ok && !p.limbs {
		return nil, false
	}
	// (P), the pressure rule's input: which names a big ARITHMETIC operation
	// reads. The promotion decision itself is made in `iterate`, where the
	// interval that gates it is in hand.
	//
	// A COMPARISON RECORDS NOTHING, and that is the rule and not an omission: a
	// word compared against a bignum is widened at the comparison and is still a
	// word everywhere else, so there is no pressure to promote it. Only a value
	// that FEEDS a big result has to be exact.
	if arith {
		for _, a := range args {
			if a.Kind == core.KName {
				if p.bigReads == nil {
					p.bigReads = map[string]bool{}
				}
				p.bigReads[a.Name] = true
			}
		}
	}
	return &core.Term{Kind: core.KApp, Kids: []*core.Term{
		core.Name(bigName),
		p.widenTo(args[0], kids[1], vals, 0),
		p.widenTo(args[1], kids[2], vals, 1),
	}}, true
}

// promote records a name as arbitrary-precision and reports whether that was
// news, which is what drives the fixpoint.
func (p *intervalPass) promote(n string) bool {
	if p.big == nil {
		p.big = map[string]bool{}
	}
	if p.big[n] {
		return false
	}
	p.big[n] = true
	p.bigChanged = true
	return true
}

// againDemand is rule (D) at a loop's back edge, in both directions.
//
//	SUPPLY  — an `again` argument that is big makes its loop variable big.
//	DEMAND  — a big loop variable makes a bare-name argument big.
//
// The demand half is what carries fib: the result declaration promotes `a`, and
// `(again b …)` then promotes `b`, without which `(+ a b)` would be a bignum
// added to a machine word that overflows on its own.
//
// Demand stops at a bare NAME on purpose. `(again (+ a b) …)` needs no promotion
// of anything — the expression is already big by supply, and forcing its
// operands big would promote the whole dependency cone, which is what rule (P)
// exists to do selectively and with a provability gate.
func (p *intervalPass) againDemand(args []*core.Term, raw []string) {
	if p.big == nil || !p.bigOK() {
		return
	}
	for i, a := range args {
		if i >= len(raw) {
			continue
		}
		if p.bigTerm(a) {
			p.promote(raw[i])
		}
		if p.big[raw[i]] && a.Kind == core.KName {
			p.promote(a.Name)
		}
	}
}

// exitDemand is rule (D) at a loop's exit: a clause returning a bare loop
// variable, in a loop whose VALUE is demanded as a bignum.
//
// This is the grip a declaration has on a loop, and it is why the pass is
// bidirectional at all. The demand is `loopTail` and not `wantBig`, which is
// what makes it work for a whole program as well as for an exported function:
// after reduction inlines `fib` into `main` there is no signature left, and the
// demand arrives instead from `(big-str …)`, whose declared argument is `big`.
func (p *intervalPass) exitDemand(t *core.Term) {
	if !p.loopTail || p.big == nil || !p.bigOK() {
		return
	}
	if t.Kind == core.KName {
		p.promote(t.Name)
	}
}

// DeclaresBig reports whether a signature mentions a range above the portable
// window, which is the only way a program can ask for arbitrary precision —
// ADR 0019's blast-radius argument, that every source of `big` is a declaration
// somebody wrote.
func DeclaresBig(sig *core.Sig) bool {
	if sig == nil {
		return false
	}
	if core.ValueType(sig.Result) == core.BigType {
		return true
	}
	for _, r := range sig.Results {
		if core.ValueType(r) == core.BigType {
			return true
		}
	}
	for _, sp := range sig.Params {
		if core.ValueType(sp.Type) == core.BigType {
			return true
		}
	}
	return false
}

// PromoteBig runs the representation pass and returns the rewritten residual.
//
// It is the SAME pass as Intervals, with the checked-arithmetic selection turned
// off: `-checked` and arbitrary precision are two different answers to the same
// unprovable operation (ADR 0019's second and third escapes), and taking both
// would emit a checked bignum, which is a contradiction — a bignum cannot
// overflow.
//
// It runs BEFORE the type checker, which is not an implementation detail: the
// checker types `(* acc i)` as `int` and would refuse it against a declared big
// result. After the rewrite it types `(big* acc (big-of i))` as `big` and
// agrees. The promotion is part of what the program MEANS, so it has to happen
// before the program is checked.
func PromoteBig(tgt *Target, sig *core.Sig, t *core.Term, all ...*core.Sig) (*core.Term, int, error) {
	// THE FIXED-LIMB RUNG COMES FIRST, because it decides how the promotion
	// itself runs: a limb value is a TABLE, so the mutable-bignum rewrite —
	// which writes into a host bignum object — has nothing to say about it.
	//
	// A target with no bignum at all can still take the limb rung, and that is
	// the point of having it: windows ships nothing to fall back to.
	limbs, w, bits := BigRepr(tgt, append([]*core.Sig{sig}, all...)...)
	if !limbs && !tgt.HasBig() {
		return t, 0, nil
	}
	if bits == 0 && !DeclaresBig(sig) && !MentionsBig(t) {
		return t, 0, nil
	}
	rep, out := intervals(tgt, sig, t, 0, nil, true, limbs, limbs)
	if !limbs {
		out, err := fitBig(tgt, out, bits)
		return out, rep.BigOps, err
	}
	// AN OPERATION WITH NO LIMB FORM TAKES THE WHOLE PROGRAM BACK to the host's
	// bignum. Mixing representations needs a conversion at every boundary
	// between them, and where that boundary sits is a question ADR 0019 opened
	// and did not close — so this is coarse on purpose, and the promotion is
	// simply redone with the destination rewrite allowed.
	if why, ok := LimbSupported(out); !ok {
		if !tgt.HasBig() {
			return nil, 0, fmt.Errorf("this program needs %s on an "+
				"arbitrary-precision value.\n"+
				"  Target %s stores one as fixed limbs (big-repr) and the built-in\n"+
				"  limb library does not implement that; it declares no bignum of\n"+
				"  its own to fall back to either. What it does have is addition,\n"+
				"  subtraction, multiplication, and division by a machine word\n"+
				"  (emit/bignum.oro).", why, tgt.Name)
		}
		rep, out = intervals(tgt, sig, t, 0, nil, true)
		out, err := fitBig(tgt, out, bits)
		return out, rep.BigOps, err
	}
	lowered, n, err := LowerLimbs(tgt, w, bits, out)
	if err != nil {
		return nil, 0, err
	}
	return lowered, n, nil
}

// MentionsBig reports whether a residual names an arbitrary-precision primitive
// itself, which is the second way a program can ask.
//
// It is not a convenience. Reduction inlines every non-exported call, so a
// helper's declared big result is gone before this pass runs — the same
// structural limit refinements.md §6b records for a `where` and
// indexnarrow-2026-08-27 records for a narrowed parameter. What survives
// inlining is the BOUNDARY, and for a bignum the boundary is `big-str`: a value
// past 2^53 cannot be printed as an `int` on any of the four hosts, so the one
// place a whole program must name arbitrary precision is exactly the one place
// the demand can be read off the residual.
func MentionsBig(t *core.Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KName && isBigOp(t.Name) {
		return true
	}
	for _, k := range t.Kids {
		if MentionsBig(k) {
			return true
		}
	}
	return false
}

// isJumpTerm reports whether a term is a back edge rather than a value.
//
// `again` is a JUMP (ADR 0015), so it produces nothing to convert — and the
// representation join at an `if` has to know that, or a loop whose one arm
// returns a bignum and whose other arm iterates emits `big-of(goto)`. A clause
// chain is a nest of conditionals, so a branch that is itself all jumps is one
// too.
func isJumpTerm(t *core.Term) bool {
	if t == nil || t.Kind != core.KApp {
		return false
	}
	op := t.Op()
	if op.Kind != core.KName {
		return false
	}
	if op.Name == "again" {
		return true
	}
	if len(t.Args()) == 3 && isCondName(op.Name) {
		return isJumpTerm(t.Args()[1]) && isJumpTerm(t.Args()[2])
	}
	return false
}

func isCondName(n string) bool { return n == "if" }

// chainBig reports whether a REBUILT clause chain produces a bignum. A jump has
// no value and contributes nothing; every other arm is a value the loop can
// return.
func (p *intervalPass) chainBig(t *core.Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName &&
		isCondName(t.Op().Name) && len(t.Args()) == 3 {
		a, b := t.Args()[1], t.Args()[2]
		return (!isJumpTerm(a) && p.chainBig(a)) || (!isJumpTerm(b) && p.chainBig(b))
	}
	return p.bigTerm(t)
}

// markBig records the representation of a rebuilt term that cannot be read off
// its shape.
func (p *intervalPass) markBig(t *core.Term, big bool) {
	if !big {
		return
	}
	if p.bigVal == nil {
		p.bigVal = map[*core.Term]bool{}
	}
	p.bigVal[t] = true
}

// bigOK reports whether arbitrary precision is available at all: either the
// target ships a bignum, or the fixed-limb rung is selected and we ship one.
// bigOK is whether this pass may touch arbitrary precision at all.
//
// `shiftOnly` says no, and that gate is not decoration: the division-to-shift
// pass runs AFTER a representation has been chosen and after the fixed-limb
// library has been spliced in, so the supply/demand solver seeing a loop whose
// variable it once promoted will promote it AGAIN — and it wrapped a limb table
// in `big.NewInt`, emitting `func GenFactLimbs(n int) *big.Int` over a body that
// builds a `[]byte`. It compiled to nothing sensible and it would have been
// caught by the first build; a subtler shape might not have been.
func (p *intervalPass) bigOK() bool { return !p.shiftOnly && (p.limbs || p.tgt.HasBig()) }

// A CONSTANT ARITHMETIC TERM WHOSE EXACT VALUE LEAVES THE WINDOW is a bignum,
// and there is nothing else it could be.
//
// This is what makes a big LITERAL work. `core/read.go` desugars one into Horner
// over base 10^15 — `(+ (* 123456789012345 1000000000000000) 678901234567890)` —
// because `KInt` holds an `int64` and the alternative was an eighth term kind.
// Every leaf is inside the window; the SPINE is not, so the operations have to
// be arbitrary precision or the program is refused.
//
// Recognising it exactly, rather than propagating a demand into every operand of
// a big operation, is what keeps `(* acc (+ i 1))` from acquiring a bignum
// counter: `(+ i 1)` is not constant, so nothing here fires. And there are no
// false positives to worry about — a constant expression that leaves the window
// can only be meant as a big value, since bounded-by-default refuses it as a
// machine word.
//
// Constant folding does NOT collapse the spine first: ADR 0009 forbids folding a
// result outside the window, because compile time is Go's int64 and run time on
// V8 is a binary64 exact only to ±(2^53−1). That refusal is what leaves the
// spine here to be found.
func constBigValue(t *core.Term) bool {
	v, ok := constValue(t, 0)
	if !ok {
		return false
	}
	w := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 53), big.NewInt(1))
	return v.CmpAbs(w) > 0
}

// constValue evaluates a closed arithmetic term at arbitrary precision, or
// reports that it is not one. The depth cap is a guard rather than a case: a
// literal's spine is one level per fifteen digits.
func constValue(t *core.Term, depth int) (*big.Int, bool) {
	if t == nil || depth > 64 {
		return nil, false
	}
	if t.Kind == core.KInt {
		return big.NewInt(t.Int), true
	}
	if t.Kind != core.KApp || t.Op().Kind != core.KName || len(t.Args()) != 2 {
		return nil, false
	}
	a, ok1 := constValue(t.Args()[0], depth+1)
	b, ok2 := constValue(t.Args()[1], depth+1)
	if !ok1 || !ok2 {
		return nil, false
	}
	switch t.Op().Name {
	case "+":
		return new(big.Int).Add(a, b), true
	case "-":
		return new(big.Int).Sub(a, b), true
	case "*":
		return new(big.Int).Mul(a, b), true
	}
	return nil, false
}

// ═══ THE DECLARED BOUND, ENFORCED ON THE HOST'S OWN BIGNUM
//
// A range is SEMANTICS: `(int 0 (pow 2 1300))` says the value is a mathematical
// integer in that interval, which is a fact about the program and true on every
// target. Which storage a target picks for it is `BigRepr`'s business.
//
// But then the two representations have to enforce the same thing. The limb
// rung traps when a value passes its bound, because a fixed width has no choice
// — it would otherwise truncate. The host's bignum is exact whatever the
// declaration says, so left alone it would silently ACCEPT what the limb rung
// refuses, and `(big-repr host)` would stop being a change of storage and
// become a change of ANSWER. That is ADR 0009's rule at the representation
// boundary, and it is the one thing this project refuses.
//
// So every operation that can GROW a value is wrapped. `big-of` is not, and
// that is a proof rather than an omission: a range reaches this rung only by
// being above the portable window, so the bound is at least 54 bits, and a
// machine word is under 2^53 — widening one can never exceed it. Comparisons
// and `big-str` produce no big value at all.
//
// The cost is one `BitLen` per operation and it is paid only by a program that
// declares a bound. A program declaring `(int 0 +inf)` has nothing to check,
// which is the honest difference between the two declarations and the reason
// they are two.
var bigGrows = map[string]bool{
	"big+": true, "big-": true, "big*": true, "big/": true, "big%": true,
	"big+!": true, "big-!": true, "big*!": true, "big/!": true, "big%!": true,
}

func fitBig(tgt *Target, t *core.Term, bits int) (*core.Term, error) {
	if bits == 0 {
		return t, nil // `(int 0 +inf)` — no bound was declared, so none is enforced
	}
	if _, ok := tgt.Prims["big-fit"]; !ok {
		return nil, fmt.Errorf("this program declares a range above the portable "+
			"window with a FINITE bound, and target %s stores that in its own "+
			"arbitrary-precision integer but declares no `big-fit` to enforce the "+
			"bound.\n"+
			"  A bound the target cannot check is a declaration that means one\n"+
			"  thing here and another on the fixed-limb rung, so it is refused\n"+
			"  rather than dropped. Declare `big-fit`, or `(big-repr limbs)`.",
			tgt.Name)
	}
	return fitWalk(t, bits), nil
}

func fitWalk(t *core.Term, bits int) *core.Term {
	if t == nil {
		return nil
	}
	if t.Kind == core.KFn {
		// Rebuilt closed: the binders are de Bruijn indices and the wrap
		// introduces no free names, so nothing can capture.
		return core.FnClosed(t.Params, fitWalk(t.Closed(), bits))
	}
	if t.Kind != core.KApp {
		return t
	}
	kids := make([]*core.Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = fitWalk(k, bits)
	}
	out := &core.Term{Kind: core.KApp, Kids: kids}
	if op := out.Op(); op.Kind == core.KName && bigGrows[op.Name] {
		return core.App(core.Name("big-fit"), out, core.Int(int64(bits)))
	}
	return out
}
