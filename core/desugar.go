package core

import "fmt"

// D, DESUGARING — docs/spec/abstract.md §6.
//
// D maps a tree of Σ into today's core Λ⁺: the atom's seven term kinds, plus the
// forms Load still expands (`case`, `try`, `expect`, a cell's `#cell`, `build`,
// `build-map`, the variadic and out-cell rewrites). Choosing Λ⁺ makes D the
// reader's output, so every stage after the reader is unchanged.
//
// D is a homomorphism: one clause per operator, each a context applied to the
// images of its arguments, computed bottom-up. Each clause builds exactly the
// term the core has read for the form since before J (Theorem D3, held by
// cmd/check's byte-identical emission). A derived operator's clause is its
// definition's (abstract.md §2.7), written out.
//
// D is memoized per node, so the reader may ask for a subtree's image while it
// is still building the tree around it — a message quotes the term a program
// wrote, as the reader always has — and the image is computed once.
type desugarer struct {
	memo map[Tail]*Term
}

func newDesugarer() *desugarer { return &desugarer{memo: map[Tail]*Term{}} }

// term is D(a).
func (d *desugarer) term(a Tail) *Term {
	if t, ok := d.memo[a]; ok {
		return t
	}
	t := d.build(a)
	d.memo[a] = t
	return t
}

func (d *desugarer) terms(as []Expr) []*Term {
	out := make([]*Term, len(as))
	for i, a := range as {
		out[i] = d.term(a)
	}
	return out
}

func (d *desugarer) build(a Tail) *Term {
	switch a := a.(type) {
	case *Var:
		return Name(a.Name)
	case *Lit:
		return a.T
	case *EmptyList:
		return &Term{Kind: KApp}
	case *Lam:
		body := d.term(a.Body)
		if len(a.Borrows) > 0 {
			// A BORROWED PARAMETER IS A PLACE in the body (local.md §2.1).
			kids := []*Term{Name(BorrowName)}
			for _, p := range a.Borrows {
				kids = append(kids, Name(p))
			}
			body = App(kids[0], append(kids[1:], Fn(a.Borrows, body))...)
		}
		return Fn(a.Params, body)
	case *Apply:
		// THE PRODUCT FORMER, Σ₀'s `tuple`: a tuple is a function on Fin n, and
		// its Church presentation makes the eliminator β itself (data.md §1).
		// `#k` starts no identifier, so nothing a program writes is captured.
		if v, ok := a.Fn.(*Var); ok && v.Name == "tuple" {
			return Fn([]string{"#k"}, &Term{Kind: KApp, Kids: append([]*Term{Name("#k")}, d.terms(a.Args)...)})
		}
		return &Term{Kind: KApp, Kids: append([]*Term{d.term(a.Fn)}, d.terms(a.Args)...)}
	case *Index:
		return &Term{Kind: KApp, Kids: []*Term{d.term(a.Table), d.term(a.At)}}
	case *Let:
		return d.let(a.Binds, d.term(a.Body))
	case *TLet:
		return d.let(a.Binds, d.term(a.Body))
	case *Seq:
		return d.seq(a.Xs[:len(a.Xs)-1], d.term(a.Xs[len(a.Xs)-1]))
	case *TSeq:
		return d.seq(a.Xs, d.term(a.Last))
	case *If:
		return mkIf(d.term(a.C), d.term(a.A), d.term(a.B))
	case *And:
		// (and a b …) = (if a (and b …) false), and (and) = true, its unit.
		if len(a.Xs) == 0 {
			return Bool(true)
		}
		t := d.term(a.Xs[len(a.Xs)-1])
		for i := len(a.Xs) - 2; i >= 0; i-- {
			t = conj(d.term(a.Xs[i]), t)
		}
		return t
	case *Or:
		// (or a b …) = (if a true (or b …)), and (or) = false.
		if len(a.Xs) == 0 {
			return Bool(false)
		}
		t := d.term(a.Xs[len(a.Xs)-1])
		for i := len(a.Xs) - 2; i >= 0; i-- {
			t = &Term{Kind: KApp, Kids: []*Term{Name("if"), d.term(a.Xs[i]), Bool(true), t}}
		}
		return t
	case *Not:
		return &Term{Kind: KApp, Kids: []*Term{Name("if"), d.term(a.X), Bool(false), Bool(true)}}
	case *Cond:
		guards := make([]*Term, len(a.Arms))
		bodies := make([]*Term, len(a.Arms))
		for i, arm := range a.Arms {
			if arm.Guard != nil {
				guards[i] = d.term(arm.Guard)
			}
			bodies[i] = d.term(arm.Body)
		}
		return chain(guards, bodies)
	case *Loop:
		return d.loop(a)
	case *Local:
		t := d.term(a.Body)
		for i := len(a.Binds) - 1; i >= 0; i-- {
			if a.Binds[i].Place {
				t = localPlace(a.Binds[i].Name, d.term(a.Binds[i].Init), t)
				continue
			}
			t = localBinder(a.Binds[i].Name, d.term(a.Binds[i].Init), t)
		}
		return t
	case *Write:
		return d.write(a)
	case *BuildMap:
		t := d.term(a.Body)
		for i := len(a.Names) - 1; i >= 0; i-- {
			if a.Place {
				t = App(Name(MapPlaceName), Name(a.Names[i]), Fn([]string{a.Names[i]}, t))
			}
			t = &Term{Kind: KApp, Kids: []*Term{Name("build-map"), d.term(a.Caps[i]), Fn([]string{a.Names[i]}, t)}}
		}
		return t
	case *The:
		// THE CORE'S ASCRIPTION CARRIES ITS SET AS THE CANONICAL TYPE SPELLING,
		// the form AscribeWide writes and every consumer reads (the checker,
		// lowering, the rung above the word). Written as a term, a program's
		// `(the (int 0 9) e)` lowered `int` as a value and was refused
		// (examples/os/du.oro, the first program to write one).
		set := d.term(a.Set)
		ty := TypeTerm(set)
		if ty != "" {
			set = Str(ty)
		}
		e := d.term(a.E)
		// A BINDER'S CLAIM IS AN OBLIGATION (ADR 0049, 0062): `b ∈ S = e` with S a
		// range marks e as a declared parameter's range marks an argument
		// (ADR 0028), decided by a literal, the intervals or the refinement layer,
		// or the program is refused. Only once it is proven is the(S; e) a set
		// the IR may read. Trusted, `x ∈ 0..5 = 10` compiled (ownbuf-2026-10-21).
		if _, _, isRange := IntRangeBig(ty); isRange && a.Name != "" {
			e = &Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(ClaimDef), Str(a.Name), Str(ty), e}}
		} else if ty == "" && a.Name != "" {
			// A SET NAMING A CONSTANT, `0..size_buf`, is canonical only once
			// reduction has unfolded it: the reducer makes the mark then (ClaimName).
			e = &Term{Kind: KApp, Kids: []*Term{Name(ClaimName), Str(a.Name), d.term(a.Set), e}}
		}
		return &Term{Kind: KApp, Kids: []*Term{Name(AscribeName), set, e}}
	case *Case:
		kids := []*Term{Name("case"), d.term(a.Scrut)}
		for _, arm := range a.Arms {
			kids = append(kids, d.term(arm.Pat), d.term(arm.Body))
		}
		return &Term{Kind: KApp, Kids: kids}
	case *TCase:
		kids := []*Term{Name("case"), d.term(a.Scrut)}
		for _, arm := range a.Arms {
			kids = append(kids, d.term(arm.Pat), d.term(arm.Body))
		}
		return &Term{Kind: KApp, Kids: kids}
	case *Monad:
		return d.monad(a.Form, a.Items, d.term(a.Body))
	case *TMonad:
		return d.monad(a.Form, a.Items, d.term(a.Body))
	case *Again:
		return &Term{Kind: KApp, Kids: append([]*Term{Name("again")}, d.terms(a.Args)...)}
	}
	panic(fmt.Sprintf("desugar: a provisional tree reached D: %T", a))
}

// let is a binder group around a body, folded from the right: a name binds by
// the redex ((fn (x) body) e), β's own, and a tuple of names by applying the
// value to a continuation, the product's eliminator (binding.md, data.md §3.2).
// `_` is respelled unwritable, as `seq`'s binder is (discards).
func (d *desugarer) let(binds []LetBind, body *Term) *Term {
	t := body
	for i := len(binds) - 1; i >= 0; i-- {
		b := binds[i]
		v := d.term(b.Value)
		switch p := b.Pat.(type) {
		case *PVar:
			t = &Term{Kind: KApp, Kids: []*Term{Fn(discards([]string{p.Name}), t), v}}
		case *PWild:
			t = &Term{Kind: KApp, Kids: []*Term{Fn(discards([]string{"_"}), t), v}}
		case *PTuple:
			t = &Term{Kind: KApp, Kids: []*Term{v, Fn(discards(p.Names), t)}}
		}
	}
	return t
}

// write is assignment by its equations (spec/local.md §2.1): a place with no
// index takes the value, `x := e` ↦ (set x e), the cell's write; an element is
// the place's store on its current version, `b[i] := e` ↦ (set b (set b i e)),
// McCarthy's; a record's component is the curried store, `b[i][j] := e` ↦
// (set b (set (b i) j e)). Several places are written concurrently: every value
// is bound first, in the state before, then each place written,
// `x̄ := ē` ↦ let t̄ = ē in x₁ := t₁; …; xₖ := tₖ (Dijkstra 1975).
func (d *desugarer) write(w *Write) *Term {
	one := func(p Place, v *Term) *Term {
		if len(p.At) == 0 {
			return App(Name("set"), Name(p.Name), v)
		}
		var at *Term = Name(p.Name)
		for _, i := range p.At[:len(p.At)-1] {
			at = App(at, d.term(i))
		}
		return App(Name("set"), Name(p.Name), App(Name("set"), at, d.term(p.At[len(p.At)-1]), v))
	}
	if len(w.Places) == 1 {
		return one(w.Places[0], d.term(w.Values[0]))
	}
	names := make([]string, len(w.Places))
	for i := range w.Places {
		names[i] = "#w" + itoaInt(i+1)
	}
	t := one(w.Places[len(w.Places)-1], Name(names[len(names)-1]))
	for i := len(w.Places) - 2; i >= 0; i-- {
		t = App(Fn([]string{"#_"}, t), one(w.Places[i], Name(names[i])))
	}
	return App(Fn(names, t), d.terms(w.Values)...)
}

func itoaInt(n int) string { return fmt.Sprint(n) }

// seq is a binding whose name is discarded: (seq a b) = ((fn (#_) b) a).
func (d *desugarer) seq(pre []Expr, last *Term) *Term {
	t := last
	for i := len(pre) - 1; i >= 0; i-- {
		t = &Term{Kind: KApp, Kids: []*Term{Fn([]string{"#_"}, t), d.term(pre[i])}}
	}
	return t
}

func (d *desugarer) monad(form string, items []Expr, body *Term) *Term {
	return &Term{Kind: KApp, Kids: append(append([]*Term{Name(form)}, d.terms(items)...), body)}
}

// loop is ADR 0015's loop, or match.md's: the arms fold into a chain of `if`
// over the state, a match arm's patterns becoming its guard and renaming its
// body, and the loop name is dropped when nothing jumps back (loopOrLet).
func (d *desugarer) loop(a *Loop) *Term {
	guards := make([]*Term, len(a.Arms))
	bodies := make([]*Term, len(a.Arms))
	for i, arm := range a.Arms {
		switch {
		case arm.Else:
			bodies[i] = d.term(arm.Body)
		case !a.Match:
			guards[i] = d.term(arm.Guard)
			bodies[i] = d.term(arm.Body)
		default:
			guard, binds := d.matchArm(arm, a.Vars)
			guards[i] = guard
			bodies[i] = renameFree(d.term(arm.Body), binds)
		}
	}
	return loopOrLet(a.Vars, chain(guards, bodies), d.terms(a.Inits))
}

// matchArm is a match arm's guard, its patterns' tests conjoined with its
// `when`, and the renaming its name patterns make.
func (d *desugarer) matchArm(arm LoopArm, vars []string) (*Term, map[string]string) {
	guard, binds := matchGuard(arm.Pats, vars)
	if arm.When != nil {
		c := renameFree(d.term(arm.When), binds)
		if guard.Kind == KBool && guard.IsTrue() {
			// Patterns that test nothing: the `when` IS the guard.
			guard = c
		} else {
			guard = &Term{Kind: KApp, Kids: []*Term{Name("if"), guard, c, Bool(false)}}
		}
	}
	return guard, binds
}

// chain folds guarded arms, the last unguarded, into a right-nested chain of
// `if`: the first guard that holds selects its body (McCarthy 1960), and the
// last body is the chain's tail, so no test is emitted for it.
func chain(guards, bodies []*Term) *Term {
	body := bodies[len(bodies)-1]
	for i := len(bodies) - 2; i >= 0; i-- {
		body = mkIf(guards[i], bodies[i], body)
	}
	return body
}

// matchGuard turns one match arm's patterns into a guard and a renaming. A name
// pattern is a rename rather than a `let`: the pattern variable is another name
// for the state variable, and renaming needs no binder (match.md §2).
func matchGuard(pats []Pat, vars []string) (*Term, map[string]string) {
	var tests []*Term
	binds := map[string]string{}
	for i, p := range pats {
		v := Name(vars[i])
		switch p := p.(type) {
		case *PVar:
			binds[p.Name] = vars[i]
		case *PLit:
			switch {
			case p.T.Kind == KBool && p.T.IsTrue():
				tests = append(tests, v)
			case p.T.Kind == KBool:
				tests = append(tests, &Term{Kind: KApp, Kids: []*Term{Name("if"), v, Bool(false), Bool(true)}})
			default:
				tests = append(tests, &Term{Kind: KApp, Kids: []*Term{Name("="), v, p.T}})
			}
		}
	}
	if len(tests) == 0 {
		return Bool(true), binds
	}
	guard := tests[len(tests)-1]
	for i := len(tests) - 2; i >= 0; i-- {
		// Conjunction, spelled the way `and` desugars: (if a b false).
		guard = &Term{Kind: KApp, Kids: []*Term{Name("if"), tests[i], guard, Bool(false)}}
	}
	return guard, binds
}
