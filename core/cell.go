package core

import (
	"fmt"
	"strings"
)

// CELLS ARE TRANSLATED AWAY BEFORE REDUCTION (spec/local.md §3).
//
// A cell is a local variable read and written in program order. Its meaning is
// the state-passing translation, Moggi's state monad compiled into the pure
// core, which read operationally is SSA construction at the source. The state
// is THE TUPLE OF EVERY CELL IN SCOPE, x₁ … xₖ, and each term that mentions one
// becomes a term whose value is the flat tuple (its value, x₁' … xₖ'): a write
// rebinds its cell for the rest of the path, an `if` joins its branches'
// tuples, a loop carries the cells it touches as more loop variables, and an
// inner scope pushes its cell on entry and projects it off at its end.
//
// FLAT, because products are associative only up to isomorphism, A × (B × C) ≅
// A × B × C, and the language takes apart one level of a product where a loop
// or a scope yields one (ADR 0031's join point). Translating each cell on its
// own nested the pairs, ((v, b), a), and a component that is a product is not a
// value (local-2026-10-08). So the outermost scope translates every cell
// inside it at once, as SSA construction handles all a function's variables.
//
// The reducer's η and β take the tuples apart, so nothing below the loader
// learns cells exist, and a cell costs what a loop variable costs. It runs after
// `case`, `try` and `expect` expand (expandCase), because their implicit arms
// are terms too and must carry the cells.

// translateCells translates every outermost `(#cell e (fn (x) body))` in t,
// then every call of a declaration with cell parameters the scopes did not
// reach: one outside any scope, or inside a λ a scope passes as a value
// (spec/local.md §5, §6). decls holds those declarations by qualified name.
func translateCells(t *Term, decls map[string]CellDecl) (*Term, error) {
	t, err := translateScopes(t, decls)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return t, nil
	}
	c := &cellTr{locals: map[string]bool{}, decls: decls}
	return c.calls(t)
}

func translateScopes(t *Term, decls map[string]CellDecl) (*Term, error) {
	if t == nil || len(t.Kids) == 0 {
		return t, nil
	}
	if isCellScope(t) {
		c := &cellTr{locals: map[string]bool{}, decls: decls}
		return c.st(t, false) // with no cell in scope, a scope's tuple is its value alone
	}
	if t.Kind == KFn {
		body, err := translateScopes(t.Body(), decls)
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, body), nil
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		x, err := translateScopes(k, decls)
		if err != nil {
			return nil, err
		}
		kids[i] = x
	}
	return &Term{Kind: t.Kind, Kids: kids}, nil
}

// calls translates the calls of a declaration with cell parameters that are
// still as written: outside any scope there is no cell, so each cell parameter
// must hold `(out τ)`. A call the scope pass translated holds compiler names
// (`#…`) there, which a program cannot write, and is left alone.
func (c *cellTr) calls(t *Term) (*Term, error) {
	if t == nil || len(t.Kids) == 0 {
		return t, nil
	}
	if t.Kind == KFn {
		body, err := c.calls(t.Body())
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, body), nil
	}
	if op := t.Kids[0]; len(t.Kids) == 2 && t.Kids[1].Kind == KFn {
		if d, ok := c.cellCall(op); ok && c.asWritten(op, d) {
			x, err := c.translateCall(op, d, t.Kids[1], false, nil)
			if err != nil {
				return nil, err
			}
			return c.calls(x)
		}
	}
	if d, ok := c.cellCall(t); ok && c.asWritten(t, d) {
		x, err := c.translateCall(t, d, nil, false, nil)
		if err != nil {
			return nil, err
		}
		return c.calls(x)
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		x, err := c.calls(k)
		if err != nil {
			return nil, err
		}
		kids[i] = x
	}
	return &Term{Kind: t.Kind, Kids: kids}, nil
}

func isCellScope(t *Term) bool {
	return t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName &&
		t.Kids[0].Name == CellName && t.Kids[2].Kind == KFn && len(t.Kids[2].Params) == 1
}

// noShadow refuses a binder inside a cell's scope that hides the cell.
func noShadow(t *Term, x string) error {
	if t == nil {
		return nil
	}
	if t.Kind == KFn {
		for _, p := range t.Params {
			if p == x {
				return fmt.Errorf("a binder named %s hides the cell %s in its scope; rename one "+
					"(spec/local.md §3)", x, x)
			}
		}
		return noShadow(t.Kids[0], x)
	}
	for _, k := range t.Kids {
		if err := noShadow(k, x); err != nil {
			return err
		}
	}
	return nil
}

// cellTr translates the cells of one outermost scope.
type cellTr struct {
	xs     []string        // the cells in scope, outermost first
	n      int             // fresh names
	locals map[string]bool // names bound inside the scope to a non-function value
	decls  map[string]CellDecl
}

func (c *cellTr) fresh() string {
	c.n++
	return fmt.Sprintf("#s%d", c.n)
}

// state is the cells' current values, as names.
func (c *cellTr) state() []*Term {
	out := make([]*Term, len(c.xs))
	for i, x := range c.xs {
		out[i] = Name(x)
	}
	return out
}

// tupOf is the flat tuple (v, x₁ … xₖ) over the given state. With no cell in
// scope it is v itself: the outermost scope's value.
func tupOf(v *Term, st []*Term) *Term {
	if len(st) == 0 {
		return v
	}
	return Fn([]string{"#k"}, &Term{Kind: KApp, Kids: append([]*Term{Name("#k"), v}, st...)})
}

func (c *cellTr) tup(v *Term) *Term { return tupOf(v, c.state()) }

// bind is `let (name, x₁ … xₖ) = T in k`, the elimination of a state tuple.
// With no cell in scope, T is a plain value and bind is a let.
func (c *cellTr) bind(T *Term, name string, k *Term) *Term {
	if len(c.xs) == 0 {
		return App(Fn([]string{name}, k), T)
	}
	return App(T, Fn(append([]string{name}, c.xs...), k))
}

func (c *cellTr) isCell(n string) bool {
	for _, x := range c.xs {
		if x == n {
			return true
		}
	}
	return false
}

// mentions reports whether t reads or writes a cell in scope, holds a scope of
// its own, or, inside a loop that carries cells, jumps back.
func (c *cellTr) mentions(t *Term, inLoop bool) bool {
	return c.mentionsCells(t) || containsCellScope(t) || inLoop && hasOwnAgain(t) || c.hasCellCall(t)
}

// hasCellCall reports a call of a declaration with cell parameters in t.
func (c *cellTr) hasCellCall(t *Term) bool {
	if len(c.decls) == 0 || t == nil {
		return false
	}
	if _, ok := c.cellCall(t); ok {
		return true
	}
	if t.Kind == KFn {
		return c.hasCellCall(t.Kids[0])
	}
	for _, k := range t.Kids {
		if c.hasCellCall(k) {
			return true
		}
	}
	return false
}

func (c *cellTr) mentionsCells(t *Term) bool {
	for _, x := range c.xs {
		if mentionsName(t, x) {
			return true
		}
	}
	return false
}

func mentionsName(t *Term, x string) bool {
	if t == nil {
		return false
	}
	if t.Kind == KName {
		return t.Name == x
	}
	for _, k := range t.Kids {
		if mentionsName(k, x) {
			return true
		}
	}
	return false
}

func containsCellScope(t *Term) bool {
	if t == nil {
		return false
	}
	if isCellScope(t) {
		return true
	}
	for _, k := range t.Kids {
		if containsCellScope(k) {
			return true
		}
	}
	return false
}

// hasOwnAgain reports an `again` that belongs to the enclosing loop: not one
// under a nested loop, which jumps to that loop.
func hasOwnAgain(t *Term) bool {
	if t == nil {
		return false
	}
	if isAgain(t) {
		return true
	}
	if headIs(t, "loop") {
		return false
	}
	for _, k := range t.Kids {
		if hasOwnAgain(k) {
			return true
		}
	}
	return false
}

func headIs(t *Term, name string) bool {
	return t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName && t.Kids[0].Name == name
}

// st is S⟦t⟧: a term whose value is the flat tuple (t's value, the cells after
// t). An `again` of a loop carrying cells is the one exception: a jump, it
// returns the jump with the loop's cells appended.
func (c *cellTr) st(t *Term, inLoop bool) (*Term, error) {
	return c.stLoop(t, inLoop, nil)
}

// stLoop is st with the cells the innermost carrying loop carries, which its
// `again` must pass.
func (c *cellTr) stLoop(t *Term, inLoop bool, carried []string) (*Term, error) {
	if !c.mentions(t, inLoop) {
		return c.tup(t), nil
	}
	switch t.Kind {
	case KName:
		return c.tup(t), nil
	case KFn:
		// A TUPLE IS A VALUE, (tuple a b) reading as (fn (#k) (#k a b)): its
		// components are threaded in order, as a constructor's arguments are.
		if b := t.Body(); len(t.Params) == 1 && IsTupleBinder(t.Params[0]) && headIs(b, t.Params[0]) {
			k := t.Params[0]
			return c.thread(b.Kids[1:], inLoop, carried, false, func(vs []*Term) *Term {
				return Fn([]string{k}, App(Name(k), vs...))
			})
		}
		return nil, c.closure(t)
	case KApp:
	default:
		return c.tup(t), nil
	}
	op, args := t.Kids[0], t.Kids[1:]

	// A SCOPE: (#cell e (fn (y) body)). Its initial value from the state
	// before it; its body with y pushed; y projected off at its end.
	if isCellScope(t) {
		lam := args[1]
		y := lam.Params[0]
		if initial := args[0]; initial.Kind == KFn {
			return nil, fmt.Errorf("cell %s holds a function: a cell holds a value (spec/local.md §3)", y)
		}
		body := lam.Body()
		if err := noShadow(body, y); err != nil {
			return nil, err
		}
		te, err := c.stLoop(args[0], inLoop, carried)
		if err != nil {
			return nil, err
		}
		outer := append([]string(nil), c.xs...)
		c.xs = append(c.xs, y)
		tb, err := c.stLoop(body, false, nil) // an `again` cannot leave a scope
		c.xs = outer
		if err != nil {
			return nil, err
		}
		// (v, x₁ … xₖ, y) ↦ (v, x₁ … xₖ)
		r := c.fresh()
		project := App(tb, Fn(append(append([]string{r}, outer...), y), c.tup(Name(r))))
		s := c.fresh()
		return c.bind(te, s, App(Fn([]string{y}, project), Name(s))), nil
	}

	// A TUPLE APPLIED TO A λ runs it once, now: ((fn (#k) (#k a b)) (fn (p q)
	// body)) is ((fn (p q) body) a b) by β, a let.
	if op.Kind == KFn && len(op.Params) == 1 && len(args) == 1 && args[0].Kind == KFn {
		if b := op.Body(); headIs(b, op.Params[0]) && len(args[0].Params) == len(b.Kids)-1 {
			return c.stLoop(App(args[0], b.Kids[1:]...), inLoop, carried)
		}
	}

	// (set x e): the cell's value becomes e's, and the write's value is ().
	if headIs(t, "set") && len(args) >= 1 && args[0].Kind == KName && c.isCell(args[0].Name) {
		x := args[0].Name
		if len(args) != 2 {
			return nil, fmt.Errorf("%s is a cell, written (set %s v); (set b i v) stores into a "+
				"buffer, which a table initializer makes (spec/local.md §1)", x, x)
		}
		te, err := c.stLoop(args[1], inLoop, carried)
		if err != nil {
			return nil, err
		}
		s := c.fresh()
		after := c.state()
		for i, n := range c.xs {
			if n == x {
				after[i] = Name(s)
			}
		}
		return c.bind(te, s, tupOf(Unit(), after)), nil
	}

	// The jump back carries the loop's cells.
	if inLoop && isAgain(t) {
		return c.thread(args, inLoop, carried, false, func(vs []*Term) *Term {
			for _, x := range carried {
				vs = append(vs, Name(x))
			}
			return App(op, vs...)
		})
	}

	// (if c a b): the condition, then each branch from the state after it.
	if headIs(t, "if") && len(args) == 3 {
		tc, err := c.stLoop(args[0], inLoop, carried)
		if err != nil {
			return nil, err
		}
		ta, err := c.stLoop(args[1], inLoop, carried)
		if err != nil {
			return nil, err
		}
		tb, err := c.stLoop(args[2], inLoop, carried)
		if err != nil {
			return nil, err
		}
		s := c.fresh()
		return c.bind(tc, s, App(Name("if"), Name(s), ta, tb)), nil
	}

	// (loop (fn (v…) chain) z…): the cells the loop touches are more loop
	// variables; the others keep their values across it.
	if headIs(t, "loop") && len(args) >= 1 && args[0].Kind == KFn {
		lam := args[0]
		var touched []string
		for _, x := range c.xs {
			if mentionsName(lam, x) {
				touched = append(touched, x)
			}
		}
		chain, err := c.stLoop(lam.Body(), true, touched)
		if err != nil {
			return nil, err
		}
		params := append(append([]string(nil), lam.Params...), touched...)
		return c.thread(args[1:], inLoop, carried, true, func(vs []*Term) *Term {
			for _, x := range touched {
				vs = append(vs, Name(x))
			}
			return App(Name("loop"), append([]*Term{Fn(params, chain)}, vs...)...)
		})
	}

	// A buffer's scope, (build n (fn (b) body)): its λ runs once, now. An
	// `again` cannot leave a scope, so the body is not in the loop.
	if (headIs(t, "build") || headIs(t, "build-map")) && len(args) == 2 && args[1].Kind == KFn {
		lam := args[1]
		body, err := c.stLoop(lam.Body(), false, nil)
		if err != nil {
			return nil, err
		}
		return c.thread(args[:1], inLoop, carried, true, func(vs []*Term) *Term {
			return App(op, vs[0], Fn(lam.Params, body))
		})
	}

	// A let, a seq: ((fn (y…) body) a…). The arguments, then the body.
	if op.Kind == KFn && len(op.Params) == len(args) {
		for j, a := range args {
			if a.Kind != KFn {
				c.locals[op.Params[j]] = true
			}
		}
		body, err := c.stLoop(op.Body(), inLoop, carried)
		if err != nil {
			return nil, err
		}
		return c.thread(args, inLoop, carried, true, func(vs []*Term) *Term {
			return App(Fn(op.Params, body), vs...)
		})
	}

	// A HOST WRITING A CELL (spec/local.md §5): a call of a declaration with
	// cell parameters, eliminated or as a value.
	if len(args) == 1 && args[0].Kind == KFn {
		if d, ok := c.cellCall(op); ok {
			return c.translateCall(op, d, args[0], inLoop, carried)
		}
	}
	if d, ok := c.cellCall(t); ok {
		return c.translateCall(t, d, nil, inLoop, carried)
	}

	// AN ELIMINATION, (h (fn (y…) body)): a `case` arm (its parameters begin
	// #t), a tuple pattern or a host call's continuation over a computed value.
	// Its λ runs once, now, so the state flows into it as into a let.
	if len(args) == 1 && args[0].Kind == KFn && c.eliminates(op, args[0]) {
		lam := args[0]
		for _, p := range lam.Params {
			c.locals[p] = true
		}
		body, err := c.stLoop(lam.Body(), inLoop, carried)
		if err != nil {
			return nil, err
		}
		k := Fn(lam.Params, body)
		// WHETHER IT IS AN ELIMINATION is decided where the reducer meets it:
		// (op λ) runs λ once, now, iff op's value is data, a product or a sum,
		// by η; a static function could run λ twice, or never. Here op is a
		// term whose static arguments are not substituted yet, so the
		// translation marks the claim, ((#elim op) λ), and the reducer
		// discharges it on op's normal form or refuses (elimMark). A case
		// arm's scrutinee is a constructor's tuple by construction.
		mark := func(op *Term) *Term {
			if strings.HasPrefix(lam.Params[0], "#t") {
				return op
			}
			return App(Name(ElimName), op)
		}
		// The eliminated term stays where it is when it mentions no cell: a
		// host call's continuation must remain one, ((p a…) (fn …)).
		if !c.mentions(op, false) {
			return App(mark(op), k), nil
		}
		// A call (p a…) keeps its shape, its arguments threaded. Any other
		// term (a pair's own elimination) is evaluated, its value bound, then
		// eliminated; η for products takes the binding apart.
		if op.Kind == KApp && op.Kids[0].Kind != KFn {
			return c.thread(op.Kids, inLoop, carried, true, func(vs []*Term) *Term {
				return App(mark(App(vs[0], vs[1:]...)), k)
			})
		}
		th, err := c.stLoop(op, inLoop, carried)
		if err != nil {
			return nil, err
		}
		s := c.fresh()
		return c.bind(th, s, App(mark(Name(s)), k)), nil
	}

	// Any other application: every argument in order, then the call.
	for _, a := range t.Kids {
		if a.Kind == KFn && c.mentionsCells(a) {
			return nil, c.closure(a)
		}
	}
	// A HEAD THAT IS A NAME STAYS WHERE IT IS, unless it is a cell (a table
	// held in a cell, indexed): its value does not depend on the state, and
	// the loader's variadic rewrite reads a call by its declaration's name.
	if h := t.Kids[0]; h.Kind == KName && !c.isCell(h.Name) {
		return c.thread(t.Kids[1:], inLoop, carried, false, func(vs []*Term) *Term { return App(h, vs...) })
	}
	return c.thread(t.Kids, inLoop, carried, false, func(vs []*Term) *Term { return App(vs[0], vs[1:]...) })
}

// thread binds each term's value in order, then builds the result from them.
// pair says the result is already a state tuple (a loop, a scope, a let, an
// elimination, whose bodies were translated); otherwise it is paired with the
// state, unless it is a jump. A λ is a value, passed as it stands.
func (c *cellTr) thread(ts []*Term, inLoop bool, carried []string, pair bool, k func([]*Term) *Term) (*Term, error) {
	vs := make([]*Term, len(ts))
	var steps []func(*Term) *Term
	for i, a := range ts {
		if a.Kind == KFn {
			vs[i] = a
			continue
		}
		ta, err := c.stLoop(a, inLoop, carried)
		if err != nil {
			return nil, err
		}
		s := c.fresh()
		vs[i] = Name(s)
		steps = append(steps, func(inner *Term) *Term { return c.bind(ta, s, inner) })
	}
	out := k(vs)
	if !isAgain(out) && !pair {
		out = c.tup(out)
	}
	for i := len(steps) - 1; i >= 0; i-- {
		out = steps[i](out)
	}
	return out, nil
}

// eliminates reports whether (op lam) runs lam once, now: a case arm, or an
// elimination of a computed value or a local value. A function a program
// passes a λ to may run it any number of times, or later, so it is not one.
func (c *cellTr) eliminates(op, lam *Term) bool {
	if len(lam.Params) > 0 && strings.HasPrefix(lam.Params[0], "#t") {
		return true // a case arm: the scrutinee is a constructor's tuple
	}
	switch op.Kind {
	case KApp:
		return true // a host call's continuation, or a tuple pattern over a call
	case KName:
		return c.locals[op.Name] || c.isCell(op.Name)
	}
	return false
}

// ElimName marks a claim the translation makes and the reducer decides: in
// ((#elim op) λ), that op's value is data, so the application is an
// elimination that runs λ once, now (spec/local.md §3).
const ElimName = "#elim"

func isElimMark(t *Term) bool {
	return t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == ElimName
}

// isData reports whether no tail of the normal form t is a function: every
// tail is a tuple, `(fn (#k) (#k …))`, the value of a product and of a sum
// (ADR 0041), or not a λ at all. It walks the forms a tail sits under, as
// tupleTails does. A static function is a λ in a normal form, since β has
// substituted every static argument by then and closures do not survive
// staging, so a λ that is not a tuple is the one thing that is not data.
func (e *Env) isData(t *Term) bool {
	if t.Kind == KFn {
		return len(t.Params) == 1 && IsTupleBinder(t.Params[0])
	}
	if t.Kind == KName && e.Prim[t.Name] {
		return false // a primitive is a function: applied to a λ, it is passed a callback
	}
	if t.Kind != KApp || len(t.Kids) == 0 {
		return true
	}
	op := t.Kids[0]
	if op.Kind == KApp && len(op.Kids) > 0 && op.Kids[0].Kind == KName && e.Prim[op.Kids[0].Name] &&
		len(t.Kids) == 2 && t.Kids[1].Kind == KFn {
		return e.isData(t.Kids[1].Closed()) // a host call's continuation
	}
	if op.Kind != KName {
		return true
	}
	switch {
	case op.Name == RequireWhereName && len(t.Kids) == 4:
		return e.isData(t.Kids[3])
	case !e.Prim[op.Name]:
		return true
	case op.Name == "if" && len(t.Kids) == 4:
		return e.isData(t.Kids[2]) && e.isData(t.Kids[3])
	case (op.Name == "let" || op.Name == "build" || op.Name == "build-map") &&
		len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return e.isData(t.Kids[2].Closed())
	case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
		return e.isData(t.Kids[1].Closed())
	}
	return true
}

func (c *cellTr) closure(lam *Term) error {
	return fmt.Errorf("a function that mentions a cell (%s): %s may run any number of times, or "+
		"later, so the cell's order would not be the program's. A cell may be read and written in "+
		"the scope, a case arm, a loop and a host call's continuation; callbacks over a scope's "+
		"cells are tier 1, not built (spec/local.md §3)", strings.Join(c.xs, ", "), lam)
}
