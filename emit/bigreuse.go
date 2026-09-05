package emit

import "oroboros/core"

// THE MUTABLE BIGNUM: a bignum operation writes into storage nothing else can
// read, instead of allocating.
//
// bigrep-2026-09-02 measured the emitted form at parity with hand-written
// bignum code and then measured the CAREFUL hand-written form at 6.4x, 4.1x and
// 1.44x faster than both. That gap is one thing: `new(big.Int).Add(a, b)`
// allocates a fresh receiver every step, and a Go programmer who owns the
// temporaries writes `a.Add(a, b)` and allocates nothing.
//
// So on Go we were at parity with what a person FIRST writes and not with what
// a person eventually writes, which is the only host where that was true — and
// that is exactly the gauntlet's bar, so it was the one place the bar was
// missed.
//
// ═══ WHY THIS IS A GO QUESTION AND NOT A PORTABLE ONE
//
// `math/big` mutates its receiver. `java.math.BigInteger` is IMMUTABLE and the
// JDK's mutable version is package-private. JavaScript's `BigInt` is a
// primitive. So on two of the three hosts that have a bignum, the careful form
// DOES NOT EXIST — a Java programmer cannot write it, and neither can a
// JavaScript one.
//
// That makes this a capability the target declares, not an optimisation we
// port: `targets/go/bigint.oro` declares the destination forms and the other
// two declare nothing, and the rule below simply does not fire there. It also
// means our claim on those hosts is unchanged and already at the bar — we are
// at parity with the best code a person can write with the host's own bignum,
// because the allocating form IS the best code there.
//
// The rung that closes it on Java and V8 is a different one: our own fixed-limb
// representation, which needs a FINITE endpoint as a limb count. That is still
// owed and is not this.
//
// ═══ WHAT HAS TO BE PROVED
//
// An operation may write into an object `d` only if nothing that is still live
// reads `d`'s old value. In general that is a liveness analysis over aliasing
// heap references; here it is small, because reduction has already made the
// program lexically local and ADR 0015's loop makes the whole back edge one
// simultaneous assignment whose operands are exactly the loop's variables.
//
// RULE R. At an `again` with arguments a₁…aₙ and loop variables v₁…vₙ,
// argument aₖ may be computed IN PLACE into the storage of a big loop variable
// vⱼ when all four hold:
//
//	(1) aₖ is an application of a big arithmetic primitive — the thing that
//	    allocates today;
//	(2) vⱼ occurs in aₖ, so the receiver is one of its own operands, which is
//	    the shape the host's contract explicitly permits;
//	(3) EVERY occurrence of vⱼ in the whole `again` is inside aₖ — so nothing
//	    else at this back edge reads vⱼ's old value, and after the simultaneous
//	    assignment nothing can reach it at all;
//	(4) vⱼ's INITIAL value is freshly allocated in this function.
//
// (3) is the liveness argument and (4) is the one that is easy to miss and is a
// silent wrong answer: a loop variable initialised from a PARAMETER holds an
// object the caller still owns, and writing into it corrupts the caller's
// value. `(loop ((acc n)) …)` with a big parameter `n` is the shape, and it
// looks exactly like the safe one.
//
// The rule is deliberately conservative. `power`'s squaring step reads `x`
// twice at one back edge — once for the accumulator and once for itself — so no
// variable satisfies (3) for it and that argument keeps its allocation. Closing
// that needs a scratch object with a lifetime longer than the loop, which is a
// different construct, and refusing is the safe direction: it costs an
// allocation, where a wrong destination costs an answer.

// bigArithNames are the operations that allocate a result today and have a
// destination form.
var bigDestOf = map[string]string{
	"big+": "big+!", "big-": "big-!", "big*": "big*!",
	"big/": "big/!", "big%": "big%!",
}

// HasBigDest reports whether this target's bignum can be written into. Go's
// can; Java's BigInteger and JavaScript's BigInt cannot, and that is a fact
// about those hosts rather than a gap in their target files.
func (tg *Target) HasBigDest() bool {
	_, ok := tg.Prims["big+!"]
	return ok
}

// countName counts occurrences of a bare name in a term. Only bare names
// matter: the rule is about which VARIABLE's storage is read, and every read of
// a loop variable in a residual is a bare occurrence — reduction has already
// removed the calls that could have hidden one.
func countName(t *core.Term, n string) int {
	if t == nil {
		return 0
	}
	if t.Kind == core.KName && t.Name == n {
		return 1
	}
	c := 0
	for _, k := range t.Kids {
		c += countName(k, n)
	}
	return c
}

// freshlyAllocated reports whether an initialiser produces an object this
// function owns — condition (4).
//
// Every big primitive except a destination form allocates: `big-of` calls
// `big.NewInt` and each arithmetic form calls `new(big.Int)`. A bare NAME does
// not, and that is the case this exists to refuse: it may be a parameter, and
// then the object belongs to the caller.
func freshlyAllocated(t *core.Term) bool {
	if t == nil || t.Kind != core.KApp {
		return false
	}
	op := t.Op()
	if op.Kind != core.KName {
		return false
	}
	if _, isDest := destSource(op.Name); isDest {
		return false
	}
	return isBigOp(op.Name) && op.Name != "big-str"
}

func destSource(name string) (string, bool) {
	for src, dst := range bigDestOf {
		if dst == name {
			return src, true
		}
	}
	if name == "big-of!" {
		return "big-of", true
	}
	return "", false
}

// reuseAgain applies rule R to one `again`, returning the rewritten term.
//
// `raw` are the loop's variable names, `inits` their initialisers, and `big`
// says which are held in arbitrary precision.
func (p *intervalPass) reuseAgain(t *core.Term, raw []string, inits []*core.Term) *core.Term {
	args := t.Args()
	if len(args) != len(raw) {
		return t
	}
	// THE ALIASING GUARD. Two arguments that are the same bare name make two
	// variables share one object after the assignment, and then writing through
	// one of them is visible through the other. Nothing in the corpus does it;
	// refusing costs nothing and admitting it would make (3) false in a way (3)
	// cannot see.
	seen := map[string]bool{}
	for _, a := range args {
		if a.Kind == core.KName {
			if seen[a.Name] {
				return t
			}
			seen[a.Name] = true
		}
	}
	out := append([]*core.Term(nil), t.Kids...)
	changed := false
	// A destination may be handed out ONCE. Two arguments writing into the same
	// object at one back edge would be two writes to one slot, and the second
	// would see the first.
	taken := map[string]bool{}
	for k, a := range args {
		if a.Kind != core.KApp || a.Op().Kind != core.KName {
			continue
		}
		dst, ok := bigDestOf[a.Op().Name] // (1)
		if !ok {
			continue
		}
		if _, have := p.tgt.Prims[dst]; !have {
			continue
		}
		for j, v := range raw {
			if !p.big[v] || taken[v] {
				continue
			}
			if countName(a, v) == 0 { // (2)
				continue
			}
			total := 0
			for _, b := range args {
				total += countName(b, v)
			}
			if total != countName(a, v) { // (3)
				continue
			}
			if j >= len(inits) || !freshlyAllocated(inits[j]) { // (4)
				continue
			}
			kids := []*core.Term{core.Name(dst), core.Name(v)}
			kids = append(kids, a.Args()...)
			out[k+1] = &core.Term{Kind: core.KApp, Kids: kids}
			taken[v] = true
			changed = true
			if p.count {
				p.rep.BigReuse++
			}
			break
		}
	}
	if !changed {
		return t
	}
	return &core.Term{Kind: core.KApp, Kids: out}
}

// reuseInLoop walks a rebuilt clause chain and applies rule R at every `again`.
//
// It runs on the REBUILT body, after the representation fixpoint has settled
// and while the loop's fresh names are still in scope — the same place and for
// the same reason the initialisers are widened.
func (p *intervalPass) reuseInLoop(t *core.Term, raw []string, inits []*core.Term) *core.Term {
	if t == nil || t.Kind != core.KApp || t.Op().Kind != core.KName {
		return t
	}
	if t.Op().Name == "again" {
		return p.reuseAgain(t, raw, inits)
	}
	pr, known := p.tgt.Prims[t.Op().Name]
	if !known {
		return t
	}
	switch pr.Kind {
	case "cond":
		if len(t.Args()) != 3 {
			return t
		}
		a := p.reuseInLoop(t.Args()[1], raw, inits)
		b := p.reuseInLoop(t.Args()[2], raw, inits)
		return core.App(t.Op(), t.Args()[0], a, b)
	case "let":
		// ADR 0015 permits `again` under a `let`. The bound value is not a
		// clause body, so only the body is walked.
		//
		// `Body()` OPENS, so the walked body carries this binder's parameters as
		// NAMES and must be CLOSED again — `core.Fn` closes, `FnClosed` does
		// not. Getting it wrong leaves the parameter's occurrences free, which
		// PRINTS IDENTICALLY: the binder still shows its name and so does every
		// occurrence, so no comparison of printed terms can see it. What it
		// costs arrives one pass later — `p.let` freshens the binder against the
		// names in scope and does NOT rename the (now free) occurrences with it,
		// so the backend emits `_ = …` for a binding nothing uses beside a use
		// of an undefined name. Fourth time `Body()`/`openFresh` rebuilding has
		// bitten this compiler, and the first where the wrong term still printed
		// correctly.
		if len(t.Args()) != 2 || t.Args()[1].Kind != core.KFn {
			return t
		}
		lam := t.Args()[1]
		nb := p.reuseInLoop(lam.Body(), raw, inits)
		return core.App(t.Op(), t.Args()[0], core.Fn(lam.Params, nb))
	}
	return t
}
