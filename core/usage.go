package core

import "strconv"

// 𝒰, USAGE — docs/spec/abstract.md §8.1, on the trees.
//
// The usage of a name in a tree is the set of counts of its uses over the tree's
// paths, abstracted by its convex hull (an exact interval abstraction, Cousot and
// Cousot 1977). Sequence adds; an additive eliminator — `if`, `case`, the arms of
// `try` and `expect`, a loop's clauses — joins its branches; and ∅ is the path
// that never reaches the end of the name's scope: `abandon`, whose value is in 0,
// and the back edge of a loop the name is bound outside. ∅ absorbs a sum and is
// the unit of a join.
//
// A RELEVANT name is used on every path that ends normally: its usage is ∅, or
// its least count is at least 1 (ADR 0043). This file computes that predicate on
// trees, the specification of what the reducer's relevance check computes on
// terms (Env.usedOnEveryPath); TestRelevantUsageAgreesOnTheCorpus holds the two
// to the same answer on every definition's parameters.
//
// HANDING ON. A name passed bare to a definition, or to a λ written in place, is
// used as that parameter is used in the receiver's body: usage inference
// (errors.md §7.2). A compound argument is evaluated, so the uses inside it
// count whatever the receiver does with its value.

// Usage is an interval of counts, or ∅ (Ended).
type Usage struct {
	Lo, Hi int // Hi < 0 is unbounded
	Ended  bool
}

var (
	unused = Usage{}
	once   = Usage{Lo: 1, Hi: 1}
	ended  = Usage{Ended: true}
)

// Plus is sequence: uses add, and a path that has ended stays ended.
func (a Usage) Plus(b Usage) Usage {
	if a.Ended || b.Ended {
		return ended
	}
	hi := a.Hi + b.Hi
	if a.Hi < 0 || b.Hi < 0 {
		hi = -1
	}
	return Usage{Lo: a.Lo + b.Lo, Hi: hi}
}

// Join is a choice between paths: the hull of both, an ended path adding none.
func (a Usage) Join(b Usage) Usage {
	switch {
	case a.Ended:
		return b
	case b.Ended:
		return a
	}
	u := Usage{Lo: min(a.Lo, b.Lo), Hi: max(a.Hi, b.Hi)}
	if a.Hi < 0 || b.Hi < 0 {
		u.Hi = -1
	}
	return u
}

// Relevant reports whether a usage meets relevance: used on every path that
// ends normally.
func (a Usage) Relevant() bool { return a.Ended || a.Lo >= 1 }

// UsageEnv holds the definitions a name may be handed to, by the name a tree
// calls them by.
type UsageEnv struct {
	Defs     map[string]*Lam
	consumed map[string]Usage
}

// ParamUsage is the usage of a definition's parameter j in its body: what
// handing it a value does with the value.
func (u *UsageEnv) ParamUsage(name string, j int) Usage {
	key := name + "/" + strconv.Itoa(j)
	if v, ok := u.consumed[key]; ok {
		return v
	}
	if u.consumed == nil {
		u.consumed = map[string]Usage{}
	}
	l := u.Defs[name]
	v := u.Of(l.Params[j], l.Body, false)
	for _, b := range l.Borrows {
		if b == l.Params[j] {
			// A BORROWED PARAMETER IS HANDED BACK on every path, the λ's value
			// carrying it (ADR 0052 item 3, spec/local.md §2.1).
			v = v.Plus(once)
		}
	}
	u.consumed[key] = v
	return v
}

// Of is 𝒰ₓ(t). outer is set once the walk has entered the body of a loop that x
// is bound outside, where a back edge ends x's path (ADR 0015, abstract.md §8.1).
func (u *UsageEnv) Of(x string, t Tail, outer bool) Usage {
	switch t := t.(type) {
	case *Var:
		if t.Name == x {
			return once
		}
		return unused
	case *Lit, *EmptyList:
		return unused
	case *Lam:
		if binds(t.Params, x) {
			return unused
		}
		return u.Of(x, t.Body, outer)
	case *Apply:
		return u.apply(x, t, outer)
	case *Index:
		return u.Of(x, t.Table, outer).Plus(u.Of(x, t.At, outer))
	case *Let:
		return u.let(x, t.Binds, t.Body, outer)
	case *TLet:
		return u.let(x, t.Binds, t.Body, outer)
	case *Seq:
		return u.seq(x, t.Xs[:len(t.Xs)-1], t.Xs[len(t.Xs)-1], outer)
	case *TSeq:
		return u.seq(x, t.Xs, t.Last, outer)
	case *If:
		return u.Of(x, t.C, outer).Plus(u.Of(x, t.A, outer).Join(u.Of(x, t.B, outer)))
	case *And: // (and a b …) = (if a (and b …) false)
		return u.chain(x, t.Xs, outer, false)
	case *Or: // (or a b …) = (if a true (or b …))
		return u.chain(x, t.Xs, outer, true)
	case *Not:
		return u.Of(x, t.X, outer)
	case *Cond:
		return u.cond(x, t.Arms, outer)
	case *Case:
		v := u.Of(x, t.Scrut, outer)
		var arms Usage
		for i, a := range t.Arms {
			au := unused
			if !patBinds(a.Pat, x) {
				au = u.Of(x, a.Body, outer)
			}
			if i == 0 {
				arms = au
			} else {
				arms = arms.Join(au)
			}
		}
		return v.Plus(arms)
	case *Monad:
		return u.monad(x, t.Form, t.Items, t.Body, outer)
	case *TMonad:
		return u.monad(x, t.Form, t.Items, t.Body, outer)
	case *Loop:
		return u.loop(x, t, outer)
	case *Local:
		sum := unused
		for _, b := range t.Binds {
			sum = sum.Plus(u.Of(x, b.Init, outer))
			if b.Name == x {
				return sum
			}
		}
		return sum.Plus(u.Of(x, t.Body, outer))
	case *BuildMap:
		sum := unused
		for i, c := range t.Caps {
			sum = sum.Plus(u.Of(x, c, outer))
			if t.Names[i] == x {
				return sum
			}
		}
		return sum.Plus(u.Of(x, t.Body, outer))
	case *The:
		return u.Of(x, t.Set, outer).Plus(u.Of(x, t.E, outer))
	case *Again:
		if outer {
			return ended
		}
		return u.sum(x, t.Args, outer)
	case *Write:
		// Every value, then each place's indices: the order D evaluates them;
		// an element write uses its place's current version, p := set(p, i, v).
		s := u.sum(x, t.Values, outer)
		for _, p := range t.Places {
			s = s.Plus(u.sum(x, p.At, outer))
			if p.Name == x && len(p.At) > 0 {
				s = s.Plus(once)
			}
		}
		return s
	}
	return unused
}

func (u *UsageEnv) sum(x string, es []Expr, outer bool) Usage {
	s := unused
	for _, e := range es {
		s = s.Plus(u.Of(x, e, outer))
	}
	return s
}

// apply is an application: abandon ends the path; a definition or a λ written
// in place, with its arity, receives a bare x as its parameter is used; and any
// other application, of a host or of a computed function, is strict in all it
// contains, so passing x to it is a use.
func (u *UsageEnv) apply(x string, t *Apply, outer bool) Usage {
	if v, ok := t.Fn.(*Var); ok && v.Name == "abandon" {
		return u.sum(x, t.Args, outer).Plus(ended)
	}
	var recv func(j int) Usage
	switch f := t.Fn.(type) {
	case *Lam:
		if len(f.Params) == len(t.Args) {
			recv = func(j int) Usage { return u.Of(f.Params[j], f.Body, false) }
		}
	case *Var:
		if l, ok := u.Defs[f.Name]; ok && f.Name != x && len(l.Params) == len(t.Args) {
			recv = func(j int) Usage { return u.ParamUsage(f.Name, j) }
		}
	}
	if recv == nil {
		return u.Of(x, t.Fn, outer).Plus(u.sum(x, t.Args, outer))
	}
	s := unused
	for j, a := range t.Args {
		if v, ok := a.(*Var); ok && v.Name == x {
			s = s.Plus(recv(j))
			continue
		}
		s = s.Plus(u.Of(x, a, outer))
	}
	if l, ok := t.Fn.(*Lam); ok && !binds(l.Params, x) {
		s = s.Plus(u.Of(x, l.Body, outer)) // a λ written in place sees x itself
	}
	return s
}

// let binds in sequence: each value is evaluated, a bare x handed to its binder
// is used as the binder is, and a binder named x ends x's scope.
func (u *UsageEnv) let(x string, binds []LetBind, body Tail, outer bool) Usage {
	if len(binds) == 0 {
		return u.Of(x, body, outer)
	}
	b, rest := binds[0], binds[1:]
	var s Usage
	if v, ok := b.Value.(*Var); ok && v.Name == x {
		switch p := b.Pat.(type) {
		case *PVar:
			s = u.let(p.Name, rest, body, outer) // handed to the binder
		case *PWild:
			s = unused // handed to `_`, which binds nothing: dropped
		default:
			s = once // a tuple pattern takes it apart where it stands: a use
		}
	} else {
		s = u.Of(x, b.Value, outer)
	}
	if patNames(b.Pat, x) {
		return s
	}
	return s.Plus(u.let(x, rest, body, outer))
}

func (u *UsageEnv) seq(x string, pre []Expr, last Tail, outer bool) Usage {
	return u.sum(x, pre, outer).Plus(u.Of(x, last, outer))
}

// chain is `and` (on false) or `or` (on true): the first operand is evaluated,
// then either the rest or nothing.
func (u *UsageEnv) chain(x string, xs []Expr, outer, or bool) Usage {
	if len(xs) == 0 {
		return unused
	}
	if len(xs) == 1 {
		return u.Of(x, xs[0], outer)
	}
	return u.Of(x, xs[0], outer).Plus(u.chain(x, xs[1:], outer, or).Join(unused))
}

func (u *UsageEnv) cond(x string, arms []CondArm, outer bool) Usage {
	if len(arms) == 1 || arms[0].Guard == nil {
		return u.Of(x, arms[0].Body, outer)
	}
	return u.Of(x, arms[0].Guard, outer).Plus(u.Of(x, arms[0].Body, outer).Join(u.cond(x, arms[1:], outer)))
}

// monad is `try` and `expect`: the value is evaluated, then either the success
// arm, with its pattern's binder, or a failure arm: `try`'s returns the failure
// and uses nothing, `expect`'s crashes, a path that ends.
func (u *UsageEnv) monad(x, form string, items []Expr, body Tail, outer bool) Usage {
	if len(items) < 2 {
		return u.sum(x, items, outer).Plus(u.Of(x, body, outer))
	}
	v := u.Of(x, items[1], outer)
	ok := unused
	if !patBinds(items[0], x) {
		ok = u.Of(x, body, outer)
	}
	fail := unused
	if form == "expect" {
		fail = ended
	}
	return v.Plus(ok.Join(fail))
}

// loop is ADR 0015's: the initial values are evaluated, then the clauses, a
// guard before its body, a loop variable named x ending x's scope; a back edge
// ends the path of a name bound outside the loop.
func (u *UsageEnv) loop(x string, t *Loop, outer bool) Usage {
	s := u.sum(x, t.Inits, outer)
	if binds(t.Vars, x) {
		return s
	}
	var walk func(i int) Usage
	walk = func(i int) Usage {
		a := t.Arms[i]
		if a.Else {
			return u.Of(x, a.Body, true)
		}
		body := unused
		if !armBinds(a, x) {
			body = u.Of(x, a.Body, true)
		}
		guard := unused
		switch {
		case a.Guard != nil:
			guard = u.Of(x, a.Guard, true)
		case a.When != nil && !armBinds(a, x):
			guard = u.Of(x, a.When, true)
		}
		return guard.Plus(body.Join(walk(i + 1)))
	}
	return s.Plus(walk(0))
}

func binds(names []string, x string) bool {
	for _, n := range names {
		if n == x {
			return true
		}
	}
	return false
}

func patNames(p Pat, x string) bool {
	switch p := p.(type) {
	case *PVar:
		return p.Name == x
	case *PTuple:
		return binds(p.Names, x)
	}
	return false
}

func armBinds(a LoopArm, x string) bool {
	for _, p := range a.Pats {
		if patNames(p, x) {
			return true
		}
	}
	return false
}

// patBinds reports whether a `case` or `try` pattern, kept as written, binds x:
// `(c x)` does.
func patBinds(p Expr, x string) bool {
	a, ok := p.(*Apply)
	if !ok {
		return false
	}
	for _, k := range a.Args {
		if v, ok := k.(*Var); ok && v.Name == x {
			return true
		}
	}
	return false
}
