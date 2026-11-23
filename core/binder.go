package core

import "fmt"

// NO BINDER OF A LANGUAGE NAME (ADR 0159). Σ_L, the language's signature
// (LanguageSignature), is the names J's reader writes its constructs with:
// `a + b` is app(+; a, b), `if c { a } else { b }` is if(c; a; b). In the
// abstract syntax they are operation symbols of a binding signature (ADR 0050,
// abstract.md §1.1), and a binder binds a variable: a variable is subject to α,
// renamed with its occurrences, and an operation symbol is not, for `1 + 2` is
// no occurrence of a variable. The core spells both as names (the atom: a
// target partitions names into primitive and defined), so a binder of a symbol
// of Σ_L captured the construct in its scope: `fn(`if`) { if c { 1 } else { 2 } }`
// applied to `fn(x, a, b) { 7 }` compiled on Go to `return 7`, whatever c was;
// `fn(`+`) { x + 2 }` applied to multiplication to `x * 2`; and a case arm's
// `'some(`if`)` made the arm's `if` an application of the payload.
//
// It is the rule of defects-2026-10-26 for a definition, which introduces a
// fresh symbol (a definitional extension, Shoenfield 1967 §4.6), stated for a
// binder, which introduces a fresh variable. So the condition is
// bv(t) ∩ Σ_L = ∅ for every declaration t, with bv the bound variables, the
// function the signature's arities define by structural recursion: one clause
// per binding operator, each naming the positions it binds. It is decided on
// the reader's tree, where every binder stands as written: D turns a block's
// binder and a quantifier's into a λ's, but a match loop's arm renames its
// pattern away and Load substitutes a case arm's, so neither is a λ's binder
// in any later image. A name of the target's (`h`, a host function) is no
// symbol of Σ_L and may be bound, as any variable may shadow another.

// PlantLanguageBinder skips the check: a binder of `if` captures the construct
// again (TestNoBinderOfALanguageName).
var PlantLanguageBinder bool

// noLanguageBinder refuses a declaration whose tree binds a symbol of Σ_L.
func noLanguageBinder(decls []Decl) error {
	if PlantLanguageBinder {
		return nil
	}
	for _, dc := range decls {
		var where string
		var bad string
		check := func(n string) {
			if bad == "" && LanguageSignature[n] {
				bad = n
			}
		}
		switch dc := dc.(type) {
		case *DefDecl:
			where = " in " + ShowName(dc.Name)
			bv(dc.Value, check)
		case *ClaimDecl:
			where = " in " + ShowName(dc.Name) + "'s header"
			for _, p := range dc.Claim.Params {
				check(p.Name)
			}
		case *TermDecl:
			bv(dc.E, check)
		}
		if bad != "" {
			return fmt.Errorf("a binder%s names %s, which is the language's own: J's reader writes its "+
				"constructs with these names (`a + b` is `+`, `if c { a } else { b }` is `if`), so the "+
				"binder would capture each of them in its scope. A binder introduces a fresh name; "+
				"choose another", where, ShowName(bad))
		}
	}
	return nil
}

// bv calls f on each name a tree binds, by the binding operators of
// abstract.md §1.1: a λ's parameters (a quantifier's and a rule's binder among
// them), a let's patterns, a loop's variables and its match arms' patterns, a
// case arm's pattern, a state binder's name (a cell, a buffer, a map buffer),
// a claim's name `x ∈ S = e`, and the pattern of `try` and `expect`.
func bv(t Tail, f func(string)) {
	var tail func(t Tail)
	var expr func(e Expr)
	exprs := func(es []Expr) {
		for _, e := range es {
			expr(e)
		}
	}
	pat := func(p Pat) {
		switch p := p.(type) {
		case *PVar:
			f(p.Name)
		case *PTuple:
			for _, n := range p.Names {
				f(n)
			}
		}
	}
	// a pattern written as an expression: a tag applied to its payloads'
	// patterns, or at its root a name, which binds in `try` and `expect`
	var epat func(e Expr, root bool)
	epat = func(e Expr, root bool) {
		switch e := e.(type) {
		case *Var:
			if !root {
				f(e.Name)
			}
		case *Apply:
			for _, a := range e.Args {
				epat(a, false)
			}
		}
	}
	binds := func(bs []LetBind) {
		for _, b := range bs {
			pat(b.Pat)
			expr(b.Value)
		}
	}
	monad := func(form string, items []Expr) {
		if len(items) > 0 && (form == "try" || form == "expect") {
			if v, ok := items[0].(*Var); ok {
				f(v.Name)
			} else {
				epat(items[0], true)
			}
			exprs(items[1:])
			return
		}
		exprs(items)
	}
	expr = func(e Expr) {
		switch e := e.(type) {
		case nil:
		case *Lam:
			for _, p := range e.Params {
				f(p)
			}
			expr(e.Body)
		case *Apply:
			expr(e.Fn)
			exprs(e.Args)
		case *Index:
			expr(e.Table)
			expr(e.At)
		case *Let:
			binds(e.Binds)
			expr(e.Body)
		case *Case:
			expr(e.Scrut)
			for _, a := range e.Arms {
				epat(a.Pat, true)
				expr(a.Body)
			}
		case *Loop:
			for _, v := range e.Vars {
				f(v)
			}
			exprs(e.Inits)
			expr(e.Invariant)
			for _, a := range e.Arms {
				for _, p := range a.Pats {
					pat(p)
				}
				expr(a.Guard)
				expr(a.When)
				tail(a.Body)
			}
		case *Local:
			for _, b := range e.Binds {
				f(b.Name)
				expr(b.Init)
			}
			expr(e.Body)
		case *BuildMap:
			for _, n := range e.Names {
				f(n)
			}
			exprs(e.Caps)
			expr(e.Body)
		case *The:
			if e.Name != "" {
				f(e.Name)
			}
			expr(e.Set)
			expr(e.E)
		case *If:
			expr(e.C)
			expr(e.A)
			expr(e.B)
		case *And:
			exprs(e.Xs)
		case *Or:
			exprs(e.Xs)
		case *Not:
			expr(e.X)
		case *Cond:
			for _, a := range e.Arms {
				expr(a.Guard)
				expr(a.Body)
			}
		case *Seq:
			exprs(e.Xs)
		case *Monad:
			monad(e.Form, e.Items)
			expr(e.Body)
		case *Write:
			exprs(e.Values)
			for _, pl := range e.Places {
				exprs(pl.At)
			}
		}
	}
	tail = func(t Tail) {
		switch t := t.(type) {
		case *Again:
			exprs(t.Args)
		case *TLet:
			binds(t.Binds)
			tail(t.Body)
		case *TSeq:
			exprs(t.Xs)
			tail(t.Last)
		case *TMonad:
			monad(t.Form, t.Items)
			tail(t.Body)
		case *TCase:
			expr(t.Scrut)
			for _, a := range t.Arms {
				epat(a.Pat, true)
				tail(a.Body)
			}
		case Expr:
			expr(t)
		}
	}
	tail(t)
}
