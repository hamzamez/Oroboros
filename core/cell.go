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
func translateCells(t *Term, decls map[string]CellDecl, borrows map[string]BorrowDecl, cbs map[string][]CallbackArg, defs map[string]*Term, product bool) (*Term, error) {
	t = writtenCells(t, borrows)
	t, err := translateScopes(t, decls, borrows, cbs, defs, product)
	if err != nil {
		return nil, err
	}
	if len(decls) == 0 {
		return t, nil
	}
	c := newCellTr(decls, borrows, cbs, defs)
	// FRESH ABOVE THE SCOPES' NAMES: the scope pass named its references and
	// versions #s1, #v2, …, and this pass may thread a call inside one of its
	// scopes (an out-call's continuation holding a `var`), so its own names
	// start past them. Both counting from 1, `#s1` bound twice captured the
	// call's other argument: GetFileSize(#s1, #s1) for GetFileSize(h, hi).
	if !PlantCellNamesClash {
		c.n = maxFreshIndex(t)
	}
	return c.calls(t)
}

// PlantCellNamesClash starts the calls pass's names at 1 again, as before.
var PlantCellNamesClash bool

// maxFreshIndex is the largest N of a name #sN or of a version the cell
// translation made in t, binders' hints included.
func maxFreshIndex(t *Term) int {
	n := 0
	see := func(s string) {
		for _, pre := range []string{"#s", VersionPrefix} {
			if strings.HasPrefix(s, pre) {
				var k int
				if _, err := fmt.Sscanf(s[len(pre):], "%d", &k); err == nil && k > n {
					n = k
				}
			}
		}
	}
	var walk func(t *Term)
	walk = func(t *Term) {
		if t == nil {
			return
		}
		if t.Kind == KName {
			see(t.Name)
		}
		for _, p := range t.Params {
			see(p)
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(t)
	return n
}

func newCellTr(decls map[string]CellDecl, borrows map[string]BorrowDecl, cbs map[string][]CallbackArg, defs map[string]*Term) *cellTr {
	return &cellTr{locals: map[string]bool{}, places: map[string]bool{}, maps: map[string]bool{}, decls: decls, borrows: borrows, cbs: cbs, defs: defs}
}

func translateScopes(t *Term, decls map[string]CellDecl, borrows map[string]BorrowDecl, cbs map[string][]CallbackArg, defs map[string]*Term, product bool) (*Term, error) {
	if t == nil || len(t.Kids) == 0 {
		return t, nil
	}
	if isCellScope(t) || isBorrowScope(t) {
		c := newCellTr(decls, borrows, cbs, defs)
		c.product = product
		return c.st(t, false) // with no cell in scope, a scope's tuple is its value alone
	}
	if t.Kind == KFn {
		body, err := translateScopes(t.Body(), decls, borrows, cbs, defs, product)
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, body), nil
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		x, err := translateScopes(k, decls, borrows, cbs, defs, false)
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
	return t.Kind == KApp && t.Kids[0].Kind == KName &&
		(len(t.Kids) == 3 && (t.Kids[0].Name == CellName || t.Kids[0].Name == PlaceName || t.Kids[0].Name == MapPlaceName) ||
			len(t.Kids) == 4 && t.Kids[0].Name == MapPlaceName) &&
		t.Kids[2].Kind == KFn && len(t.Kids[2].Params) == 1
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
					"(spec/local.md §3)", ShowName(x), ShowName(x))
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
	// places are the state binders whose value is a buffer, in program order
	// (spec/local.md §2.1), and borrows the callees that borrow one.
	places map[string]bool
	maps   map[string]bool // the places whose value is a map
	// caps are the map places' capacities, the c of their `map(c)`: a map of
	// capacity c is in {m : len(m) <= c} (maps.md §6, hamza's B′), so each
	// insert into one is marked with that claim.
	caps    map[string]*Term
	borrows map[string]BorrowDecl
	// cbs are the host operations with a callback parameter, by qualified
	// name: each callback's argument index, and whether its host promises the
	// order of its calls (ADR 0143)
	cbs map[string][]CallbackArg
	// defs are the program's definitions, by qualified name: one applied to a
	// λ that mentions a cell is expanded before the translation (expandHigher)
	defs map[string]*Term
	// product: the definition's header declares a product result, so its
	// borrow scope hands back the factors flat (ADR 0112)
	product bool
}

func (c *cellTr) fresh() string {
	c.n++
	return fmt.Sprintf("#s%d", c.n)
}

// in is the name a state binder is threaded under: a place's is marked a
// version (VersionPrefix), so that β puts the version a binder of it receives
// where it is used once, first (reduce.go, firstUse); a cell's is its own.
func (c *cellTr) in(u string) string {
	if c.places[u] {
		return VersionPrefix + u
	}
	return u
}

// userOf is the program's name of a state binder threaded as x.
func (c *cellTr) userOf(x string) string {
	if u, ok := strings.CutPrefix(x, VersionPrefix); ok && c.places[u] {
		return u
	}
	return x
}

// freshVersion names a buffer's version the translation binds, marked so that
// β puts it where it is used once, first (reduce.go, firstUse).
func (c *cellTr) freshVersion() string {
	c.n++
	return fmt.Sprintf("%s%d", VersionPrefix, c.n)
}

// state is the cells' current values, as names.
func (c *cellTr) state() []*Term {
	out := make([]*Term, len(c.xs))
	for i, x := range c.xs {
		out[i] = Name(x)
	}
	return out
}

// spliceTails rewrites each tail (#k v x₁ … xₖ p₁ … pₘ) of a translated borrow
// body, v a written tuple (v₁ … vₙ), to ((p₁ … pₘ, v₁ … vₙ), x₁ … xₖ): the
// borrow scope's value with v's factors in place (ADR 0112). The tails are
// tailArity's: an `if`'s arms, a let's body, each continuation of an
// elimination, a loop's exits; a jump or a crash is left as it is. ok is false
// at a tail of any other shape.
func spliceTails(t *Term, k, m int) (*Term, bool) {
	if t.Kind == KFn && len(t.Params) == 1 && t.Params[0] == "#k" {
		b := t.Body()
		if b.Kind != KApp || len(b.Kids) != 2+k+m || b.Kids[0].Kind != KName || b.Kids[0].Name != "#k" {
			return nil, false
		}
		return spliceValue(b.Kids[1], b.Kids[2:2+k], b.Kids[2+k:])
	}
	if t.Kind != KApp || len(t.Kids) == 0 {
		return nil, false
	}
	if headIs(t, "abandon") || isAgain(t) {
		return t, true
	}
	op, args := t.Kids[0], t.Kids[1:]
	switch {
	case headIs(t, RequireWhereName) && len(args) == 3:
		// a mark's value is its body's, a loop's invariant around its loop
		nb, ok := spliceTails(args[2], k, m)
		if !ok {
			return nil, false
		}
		return App(op, args[0], args[1], nb), true
	case headIs(t, "if") && len(args) == 3:
		a, ok1 := spliceTails(args[1], k, m)
		b, ok2 := spliceTails(args[2], k, m)
		if !ok1 || !ok2 {
			return nil, false
		}
		return App(op, args[0], a, b), true
	case headIs(t, "loop") && len(args) >= 1 && args[0].Kind == KFn:
		ch, ok := spliceTails(args[0].Body(), k, m)
		if !ok {
			return nil, false
		}
		return App(op, append([]*Term{Fn(args[0].Params, ch)}, args[1:]...)...), true
	case op.Kind == KFn && len(op.Params) == 1 && strings.HasPrefix(op.Params[0], "#k") &&
		len(args) == 1 && args[0].Kind == KFn:
		// A TUPLE APPLIED TO ITS CONTINUATION, the state translation's let:
		// the continuation's body is the tail.
		nb, ok := spliceTails(args[0].Body(), k, m)
		if !ok {
			return nil, false
		}
		return App(op, Fn(args[0].Params, nb)), true
	case op.Kind == KFn && len(op.Params) == len(args): // a let, a seq
		nb, ok := spliceTails(op.Body(), k, m)
		if !ok {
			return nil, false
		}
		return App(Fn(op.Params, nb), args...), true
	}
	// AN ELIMINATION: each λ argument is a continuation, a case's arms.
	var out []*Term
	any := false
	for _, a := range args {
		if a.Kind != KFn {
			return nil, false
		}
		nb, ok := spliceTails(a.Body(), k, m)
		if !ok {
			return nil, false
		}
		out = append(out, Fn(a.Params, nb))
		any = true
	}
	if !any {
		return nil, false
	}
	return App(op, out...), true
}

// spliceValue is the tail (v, st…, ps…) rewritten to ((ps…, v₁ … vₙ), st…)
// where v is a written tuple, or a let, a seq or an `if` whose own tails are:
// the rewrite goes down v's tails, which see st and ps unchanged (their names
// are the translation's, which a program cannot bind).
func spliceValue(v *Term, st, ps []*Term) (*Term, bool) {
	switch {
	case v.Kind == KFn && len(v.Params) == 1 && strings.HasPrefix(v.Params[0], "#k"):
		vb := v.Body()
		if vb.Kind != KApp || len(vb.Kids) < 3 || vb.Kids[0].Kind != KName || vb.Kids[0].Name != v.Params[0] {
			return nil, false
		}
		val := append(append([]*Term(nil), ps...), vb.Kids[1:]...)
		return tupOf(tupleTerm(val), st), true
	case v.Kind != KApp || len(v.Kids) == 0:
		return nil, false
	case headIs(v, "abandon"):
		return v, true
	case headIs(v, "if") && len(v.Kids) == 4:
		a, ok1 := spliceValue(v.Kids[2], st, ps)
		b, ok2 := spliceValue(v.Kids[3], st, ps)
		if !ok1 || !ok2 {
			return nil, false
		}
		return App(v.Kids[0], v.Kids[1], a, b), true
	case v.Kids[0].Kind == KFn && len(v.Kids[0].Params) == len(v.Kids)-1: // a let, a seq
		nb, ok := spliceValue(v.Kids[0].Body(), st, ps)
		if !ok {
			return nil, false
		}
		return App(Fn(v.Kids[0].Params, nb), v.Kids[1:]...), true
	}
	return nil, false
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
		x = c.userOf(x)
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
		if mentionsName(t, c.userOf(x)) {
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
	if isCellScope(t) || isBorrowScope(t) {
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
		if c.places[t.Name] {
			return c.tup(Name(c.in(t.Name))), nil // the place's state, under its threaded name
		}
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
			return nil, fmt.Errorf("cell %s holds a function: a cell holds a value (spec/local.md §3)", ShowName(y))
		}
		body := lam.Body()
		if err := noShadow(body, y); err != nil {
			return nil, err
		}
		place := headIs(t, PlaceName) || headIs(t, MapPlaceName)
		if place {
			if err := c.checkPlace(body, y, false); err != nil {
				return nil, err
			}
		}
		te, err := c.stLoop(args[0], inLoop, carried)
		if err != nil {
			return nil, err
		}
		if place {
			c.places[y] = true
		}
		if headIs(t, MapPlaceName) {
			c.maps[y] = true // its element write is insert (ADR 0058)
			if len(args) == 3 {
				if c.caps == nil {
					c.caps = map[string]*Term{}
				}
				c.caps[y] = args[2] // its capacity, which each insert's mark claims
			}
		}
		iy := c.in(y)
		outer := append([]string(nil), c.xs...)
		c.xs = append(c.xs, iy)
		tb, err := c.stLoop(body, false, nil) // an `again` cannot leave a scope
		c.xs = outer
		if err != nil {
			return nil, err
		}
		// (v, x₁ … xₖ, y) ↦ (v, x₁ … xₖ)
		r := c.fresh()
		project := App(tb, Fn(append(append([]string{r}, outer...), iy), c.tup(Name(r))))
		s := c.fresh()
		return c.bind(te, s, App(Fn([]string{iy}, project), Name(s))), nil
	}

	// A BORROW SCOPE: (#borrow p… (fn (p…) body)), a λ's body whose parameters
	// are places. Its value is their last versions, then the body's, a unit
	// body dropped (spec/local.md §2.1).
	if isBorrowScope(t) {
		lam := args[len(args)-1]
		ps := lam.Params
		body := lam.Body()
		ips := make([]string, len(ps))
		for i, p := range ps {
			if err := noShadow(body, p); err != nil {
				return nil, err
			}
			if err := c.checkPlace(body, p, true); err != nil {
				return nil, err
			}
			c.places[p] = true
			ips[i] = c.in(p)
		}
		unit := unitBody(body, c.borrows)
		outer := append([]string(nil), c.xs...)
		c.xs = append(c.xs, ips...)
		tb, err := c.stLoop(body, false, nil)
		c.xs = outer
		if err != nil {
			return nil, err
		}
		// (v, x₁ … xₖ, p…) ↦ ((p…, v), x₁ … xₖ)
		r := c.fresh()
		val := make([]*Term, 0, len(ps)+1)
		for _, p := range ips {
			val = append(val, Name(p))
		}
		// A PRODUCT VALUE v = (v₁ … vₙ) hands back its factors, (p…, v₁ … vₙ),
		// as a host call with n results does (ADR 0112). A name holds one value,
		// so v is not bound and then taken apart: each tail (v, x…, p…) of the
		// translated body is rewritten in place to ((p…, v₁ … vₙ), x…), and no
		// tuple nests in another where a join point binds it (tables.md §2.5).
		if !unit && c.product {
			if n := tailArity(body); n > 1 {
				if out, ok := spliceTails(tb, len(outer), len(ips)); ok {
					return App(Fn(ips, out), args[:len(args)-1]...), nil
				}
				return nil, fmt.Errorf("a definition that lends a buffer and answers a product of %d "+
					"values reaches a tail that is no tuple; each tail is (v₁, …, v%d) written out "+
					"(spec/local.md §2.1, ADR 0112)", n, n)
			}
		}
		if !unit {
			val = append(val, Name(r))
		}
		project := App(tb, Fn(append(append([]string{r}, outer...), ips...), c.tup(tupleTerm(val))))
		return App(Fn(ips, project), args[:len(args)-1]...), nil
	}

	// A TUPLE APPLIED TO A λ runs it once, now: ((fn (#k) (#k a b)) (fn (p q)
	// body)) is ((fn (p q) body) a b) by β, a let.
	if op.Kind == KFn && len(op.Params) == 1 && len(args) == 1 && args[0].Kind == KFn {
		if b := op.Body(); headIs(b, op.Params[0]) && len(args[0].Params) == len(b.Kids)-1 {
			return c.stLoop(App(args[0], b.Kids[1:]...), inLoop, carried)
		}
	}

	// (set x e): the cell's value becomes e's, and the write's value is ().
	if headIs(t, "set") && len(args) >= 1 && args[0].Kind == KName && c.isCell(args[0].Name) &&
		!(len(args) == 3 && c.places[args[0].Name]) {
		x := args[0].Name
		if len(args) != 2 {
			return nil, fmt.Errorf("%s is a cell, written `%s := v`; `b[i] := v` stores into a "+
				"buffer, which a table initializer makes (spec/local.md §1)", ShowName(x), ShowName(x))
		}
		// AN ELEMENT WRITE puts the store in its place's slot of the state, a
		// tuple's component, evaluated where the tuple is taken apart: the
		// binder that receives it is the place's own (VersionPrefix), and β puts
		// it where it is used once, first (reduce.go, firstUse), with no binding
		// of its own to reduce.
		if v := args[1]; c.places[x] && headIs(v, "set") && len(v.Kids) == 4 {
			store := "set"
			if c.maps[x] {
				store = "insert" // McCarthy's store over a map's index set (ADR 0058)
			}
			return c.thread(v.Kids[1:], inLoop, carried, true, func(vs []*Term) *Term {
				after := c.state()
				for i, n := range c.xs {
					if n == c.in(x) {
						after[i] = c.capacityMark(x, App(Name(store), vs...))
					}
				}
				return tupOf(Unit(), after)
			})
		}
		te, err := c.stLoop(args[1], inLoop, carried)
		if err != nil {
			return nil, err
		}
		s := c.fresh()
		if c.places[x] {
			s = c.freshVersion() // a buffer's new version (reduce.go, firstUse)
		}
		after := c.state()
		for i, n := range c.xs {
			if n == c.in(x) {
				after[i] = Name(s)
			}
		}
		return c.bind(te, s, tupOf(Unit(), after)), nil
	}

	// The jump back carries the loop's cells.
	if inLoop && isAgain(t) {
		return c.thread(args, inLoop, carried, false, func(vs []*Term) *Term {
			state := make([]*Term, 0, len(carried)+len(vs))
			for _, x := range carried {
				state = append(state, Name(x))
			}
			return App(op, append(state, vs...)...)
		})
	}

	// (if c a b): the condition, then each branch from the state after it.
	if headIs(t, "if") && len(args) == 3 {
		ta, err := c.stLoop(args[1], inLoop, carried)
		if err != nil {
			return nil, err
		}
		tb, err := c.stLoop(args[2], inLoop, carried)
		if err != nil {
			return nil, err
		}
		// A STATEMENT IF THAT WRITES ONE STATE BINDER is a φ on its value:
		// x := if c then π_x(S⟦a⟧) else π_x(S⟦b⟧). Each branch runs once, in its
		// own component, so no effect is duplicated, and the rest of the block
		// stays after the join instead of being copied into both branches, as
		// the reducer's case-of-case would copy a continuation eliminating a
		// tuple. SSA's φ (Appel 1998); today's programs write it as a value,
		// `(again (if c (nl out) out) …)` (storeorder-2026-10-15).
		if x, ok := c.phi(args[1], args[2], inLoop); ok {
			proj := func(tt *Term) *Term {
				r := c.fresh()
				return App(tt, Fn(append([]string{r}, c.xs...), Name(x)))
			}
			s := c.fresh()
			if c.places[c.userOf(x)] {
				s = c.freshVersion()
			}
			after := c.state()
			for i, n := range c.xs {
				if n == x {
					after[i] = Name(s)
				}
			}
			join := func(cond *Term) *Term {
				return App(Fn([]string{s}, tupOf(Unit(), after)), App(Name("if"), cond, proj(ta), proj(tb)))
			}
			if !c.mentions(args[0], inLoop) {
				return join(args[0]), nil
			}
			tc, err := c.stLoop(args[0], inLoop, carried)
			if err != nil {
				return nil, err
			}
			k := c.fresh()
			return c.bind(tc, k, join(Name(k))), nil
		}
		// A CONDITION THAT TOUCHES NO STATE is evaluated where it stands: it is
		// first in the if, so nothing is reordered, and binding it would leave a
		// let β keeps when it reads a table and a branch has effects.
		if !c.mentions(args[0], inLoop) {
			return App(Name("if"), args[0], ta, tb), nil
		}
		tc, err := c.stLoop(args[0], inLoop, carried)
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
		// A LOOP CARRIES WHAT IT WRITES, FIRST, in binding order (spec/local.md
		// §2.1): what it only reads is free in it, since a variable every back
		// edge passes unchanged is invariant, and a product's order is its
		// symmetry. The order is the corpus's, measured: 49 of the 53 loops that
		// thread a buffer by hand put it first.
		var touched []string
		for _, x := range c.xs {
			if c.writes(lam, c.userOf(x)) {
				touched = append(touched, x)
			}
		}
		chain, err := c.stLoop(lam.Body(), true, touched)
		if err != nil {
			return nil, err
		}
		params := append(append([]string(nil), touched...), lam.Params...)
		return c.thread(args[1:], inLoop, carried, true, func(vs []*Term) *Term {
			inits := make([]*Term, 0, len(touched)+len(vs))
			for _, x := range touched {
				inits = append(inits, Name(x))
			}
			return App(Name("loop"), append([]*Term{Fn(params, chain)}, append(inits, vs...)...)...)
		})
	}

	// A buffer's scope, (build n (fn (b) body)): its λ runs once, now. An
	// `again` cannot leave a scope, so the body is not in the loop.
	if (headIs(t, "build") || headIs(t, "build-map") || headIs(t, AdoptName)) && len(args) == 2 && args[1].Kind == KFn {
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
			// A λ BOUND TO A NAME that mentions a cell is refused as one passed
			// is: the name may be applied any number of times, or never, so the
			// cell's order would not be the program's (spec/local.md §3). A
			// thread keeps a λ argument as it is, for β, so its writes reached
			// no translation and were lost, and its reads saw the cell's first
			// value: `add = fn(x) { n := n + x }; add(1); add(2)` printed n as 0
			// (callbacks-derivation-2026-11-20).
			if a.Kind == KFn && !PlantLetLambdaCellUnchecked && c.mentionsCells(a) {
				return nil, c.closure(a)
			}
			if a.Kind != KFn {
				c.locals[op.Params[j]] = true
			}
		}
		body, err := c.stLoop(op.Body(), inLoop, carried)
		if err != nil {
			return nil, err
		}
		// ONE BINDER takes its value under its own name, the program's, not a
		// fresh one β would have to substitute back: where β keeps the binding
		// (a value used twice), the facts the analyses derive are about the
		// program's name (freq's `s = d - 1`, storeorder-2026-10-15).
		if len(op.Params) == 1 && args[0].Kind != KFn {
			ta, err := c.stLoop(args[0], inLoop, carried)
			if err != nil {
				return nil, err
			}
			return c.bind(ta, op.Params[0], body), nil
		}
		return c.thread(args, inLoop, carried, true, func(vs []*Term) *Term {
			return App(Fn(op.Params, body), vs...)
		})
	}

	// A BORROW (spec/local.md §2.1): a call that lends places, eliminated or as
	// a statement or a value.
	if len(args) == 1 && args[0].Kind == KFn {
		if d, ok := c.borrowCall(op); ok {
			return c.translateBorrow(op, d, args[0], inLoop, carried)
		}
	}
	if d, ok := c.borrowCall(t); ok {
		return c.translateBorrow(t, d, nil, inLoop, carried)
	}

	// A HOST WRITING A CELL (spec/local.md §5): a call of a declaration with
	// cell parameters, eliminated or as a value, AS WRITTEN: one the scope pass
	// already translated holds compiler names at its cells and is an ordinary
	// call now, as `calls` leaves it. Threaded again, a scope translated inside
	// an out-call's continuation (`var hi := 0` after `(sz, ok) = …(out …)`)
	// was translated twice and its cell read as no cell.
	if len(args) == 1 && args[0].Kind == KFn {
		if d, ok := c.cellCall(op); ok && (c.asWritten(op, d) || PlantCellCallTwice) {
			return c.translateCall(op, d, args[0], inLoop, carried)
		}
	}
	if d, ok := c.cellCall(t); ok && (c.asWritten(t, d) || PlantCellCallTwice) {
		// A CALL THAT WRITES A HOST'S CELL AND TAKES A λ MENTIONING ONE
		// (InitOnceExecuteOnce's block and the callback's count, ADR 0143):
		// the λ's cells are threaded first, r = #ref(x); v = h(…, λ′, …);
		// x := #deref(r), and the call inside is the host's cell call again,
		// its λ′ mentioning only references. Translated the other way round,
		// the λ was passed as it stands and its cells read as values.
		if h := t.Kids[0]; h.Kind == KName && c.cbs[h.Name] != nil && !PlantCallbackCellRefused {
			if u, ok, err := c.callbackCells(t, c.cbs[h.Name]); err != nil {
				return nil, err
			} else if ok {
				return c.stLoop(u, inLoop, carried)
			}
		}
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

	// A LOOP'S INVARIANT (j.md §2.8) is logic over the loop's state, decided and
	// never run. A cell or buffer in scope may appear in it only under `len`,
	// which no store changes (len ∘ set = len ∘ π₁, the store law), so len(b)
	// at the loop head is len of every version and the λ is kept as written;
	// only its body, the loop, is translated. An element of a buffer would need
	// the loop head's version, which is not built.
	if headIs(t, RequireWhereName) && len(t.Kids) == 4 && t.Kids[1].Kind == KStr && t.Kids[1].Str == LoopInvariantDef &&
		t.Kids[2].Kind == KFn && !PlantMarkIsAClosure {
		if bad := c.cellOffLen(t.Kids[2].Body()); bad != "" {
			return nil, fmt.Errorf("a loop's invariant mentions %s other than through its length: an invariant is a "+
				"formula over the loop's variables, and a buffer or cell in scope may appear in it only as "+
				"len(%s), which no store changes (j.md §2.8)", ShowName(bad), ShowName(bad))
		}
		body, err := c.stLoop(t.Kids[3], inLoop, carried)
		if err != nil {
			return nil, err
		}
		return &Term{Kind: KApp, Kids: []*Term{t.Kids[0], t.Kids[1], t.Kids[2], body}}, nil
	}
	// A λ THE HOST APPLIES DURING THE CALL (ADR 0143) that mentions a cell.
	if h := t.Kids[0]; h.Kind == KName && c.cbs[h.Name] != nil && !PlantCallbackCellRefused {
		if u, ok, err := c.callbackCells(t, c.cbs[h.Name]); err != nil {
			return nil, err
		} else if ok {
			return c.stLoop(u, inLoop, carried)
		}
	}
	// A DEFINITION OF THE PROGRAM'S APPLIED TO A λ THAT MENTIONS A CELL: the
	// strongest host there is, expanded before the translation (expandHigher).
	if u, ok := c.expandHigher(t); ok {
		return c.stLoop(u, inLoop, carried)
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
	// AN ARGUMENT THAT WRITES NO STATE leaves the state as it was, so its value
	// is itself with each place named by its threaded name, evaluated where it
	// stands. That keeps the order of evaluation when no argument after it
	// writes, so every argument past the last that writes stays in place
	// unbound: a binding β cannot undo (a table read into a body with effects)
	// would otherwise remain, and a φ argument of a jump become two jumps.
	last := -1
	for i, a := range ts {
		if a.Kind != KFn && !c.readsOnly(a, inLoop) {
			last = i
		}
	}
	for i, a := range ts {
		if a.Kind == KFn {
			vs[i] = a
			continue
		}
		if i > last {
			vs[i] = c.renamePlaces(a)
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

// readsOnly reports a term that writes no state: no write, borrow or host
// writing a cell of a binder in scope, no scope of its own, and, in a loop that
// carries state, no jump.
func (c *cellTr) readsOnly(t *Term, inLoop bool) bool {
	if containsCellScope(t) || c.hasCellCall(t) || (inLoop && hasOwnAgain(t)) {
		return false
	}
	// A REFERENCE HAS IDENTITY: the host is handed (#ref c) and the cell read
	// back through the same reference, so it is bound, never left in place to be
	// made twice (spec/local.md §5).
	if containsHead(t, RefName) || containsHead(t, FreshName) {
		return false
	}
	for _, x := range c.xs {
		if c.writes(t, c.userOf(x)) {
			return false
		}
	}
	return true
}

// containsHead reports an application of the name h in t.
func containsHead(t *Term, h string) bool {
	if t == nil {
		return false
	}
	if headIs(t, h) {
		return true
	}
	for _, k := range t.Kids {
		if containsHead(k, h) {
			return true
		}
	}
	return false
}

// renamePlaces is t with each place in scope named by its threaded name.
func (c *cellTr) renamePlaces(t *Term) *Term {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case KName:
		if c.places[t.Name] {
			return Name(c.in(t.Name))
		}
		return t
	case KFn:
		return Fn(t.Params, c.renamePlaces(t.Body()))
	}
	if len(t.Kids) == 0 {
		return t
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = c.renamePlaces(k)
	}
	return &Term{Kind: t.Kind, Kids: kids}
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
	case (op.Name == "let" || op.Name == "build" || op.Name == "build-map" || op.Name == AdoptName) &&
		len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return e.isData(t.Kids[2].Closed())
	case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
		return e.isData(t.Kids[1].Closed())
	}
	return true
}

// PlantLetLambdaCellUnchecked lets a λ bound by a let mention a cell, the
// wrong answer callbacks-derivation-2026-11-20 found.
var PlantLetLambdaCellUnchecked bool

// expandHigher expands a definition of the program applied to a λ that
// mentions a cell (ADR 0144). The definition is the strongest host there is:
// no recursion (ADR 0014), no escape (a closure may not survive staging), and
// its body runs in program order, every application of its parameter at a
// point β sees. So its activations of the λ are a sequence the program fixes,
// and the state translation applied after the expansion is Theorem T₁ with the
// definition's body as the traversal: S(d(λ)) = d_S(λ_S), the state monad's
// lift, which S computes itself on the expanded term. The translation ran
// before β, so it refused what the definition's meaning admits.
//
// The expansion is β on values: each λ argument substituted for its
// parameter, which copies a value and runs nothing, and every other argument
// let-bound first, in order, so its effects stay where they were written
// (ADR 0010, 0071). The definition is locally nameless (its binders are
// indices), so the substitution captures nothing.
func (c *cellTr) expandHigher(t *Term) (*Term, bool) {
	if PlantHigherNotExpanded || t.Kind != KApp || len(t.Kids) < 2 || t.Kids[0].Kind != KName {
		return nil, false
	}
	d, ok := c.defs[t.Kids[0].Name]
	args := t.Kids[1:]
	if !ok || d == nil || d.Kind != KFn || len(d.Params) != len(args) {
		return nil, false
	}
	cellLam := false
	for _, a := range args {
		if a.Kind == KFn && c.mentionsCells(a) {
			cellLam = true
		}
	}
	if !cellLam {
		return nil, false
	}
	m := map[string]*Term{}
	var names []string
	var vals []*Term
	for i, a := range args {
		if a.Kind == KFn {
			m[d.Params[i]] = a
			continue
		}
		y := c.fresh()
		m[d.Params[i]] = Name(y)
		names, vals = append(names, y), append(vals, a)
	}
	body := betaLams(subst(d.Body(), m))
	for i := len(names) - 1; i >= 0; i-- {
		body = App(Fn([]string{names[i]}, body), vals[i])
	}
	return body, true
}

// betaLams rewrites each application of a λ to as many arguments as it has
// parameters into lets, innermost first: ((fn (x y) b) a₁ a₂) is let x = a₁ in
// let y = a₂ in b, the arguments in order, β on the λ's own parameters.
func betaLams(t *Term) *Term {
	if t == nil || len(t.Kids) == 0 {
		return t
	}
	if t.Kind == KFn {
		return Fn(t.Params, betaLams(t.Body()))
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = betaLams(k)
	}
	if h := kids[0]; h.Kind == KFn && len(h.Params) == len(kids)-1 && len(h.Params) > 1 {
		body := h.Body()
		for i := len(h.Params) - 1; i >= 0; i-- {
			body = App(Fn([]string{h.Params[i]}, body), kids[1+i])
		}
		return body
	}
	return &Term{Kind: t.Kind, Kids: kids}
}

// PlantHigherNotExpanded leaves a definition applied to a λ that mentions a
// cell unexpanded, as before ADR 0144: the λ is then refused as a closure
// (TestADefinitionAppliedToALambdaThatWritesACellIsExpanded).
var PlantHigherNotExpanded bool

func (c *cellTr) closure(lam *Term) error {
	return fmt.Errorf("a function that mentions a cell (%s): %s may run any number of times, or "+
		"later, so the cell's order would not be the program's. A cell may be read and written in "+
		"the scope, a case arm, a loop and a host call's continuation; a λ at a callback's field "+
		"may capture one it passes to a step's cell parameter (ADR 0133), a λ at a host's callback parameter "+
		"may write one where the host promises the order of its calls (ADR 0143), and a λ given to a "+
		"definition of the program's may write one (ADR 0144) (spec/local.md §3)", strings.Join(ShowNames(c.xs), ", "), ShowTerm(lam))
}

// writtenCells makes a buffer of each cell its scope writes element by element:
// by a store, (set b i v), or by lending it to a callee's linear parameter,
// which writes it in place (ADR 0070). Such a binder holds a table and is
// written as one, so it is a fresh region holding its initializer's value,
// (adopt e (fn (b) (#place b …))): moved in when e is a fresh region, a linear
// result, and copied in otherwise (reduce.go, adopt; ADR 0084). Only the loader
// knows which callees borrow, so the reader cannot decide it.
func writtenCells(t *Term, borrows map[string]BorrowDecl) *Term {
	if t == nil {
		return nil
	}
	if t.Kind == KFn {
		return Fn(t.Params, writtenCells(t.Body(), borrows))
	}
	if t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName && t.Kids[0].Name == CellName &&
		t.Kids[2].Kind == KFn && len(t.Kids[2].Params) == 1 {
		b := t.Kids[2].Params[0]
		body := writtenCells(t.Kids[2].Body(), borrows)
		e := writtenCells(t.Kids[1], borrows)
		if !scalarLiteral(e) && writesElements(body, b, borrows) {
			return App(Name(AdoptName), e, Fn([]string{b}, App(Name(PlaceName), Name(b), Fn([]string{b}, body))))
		}
		return App(t.Kids[0], e, Fn([]string{b}, body))
	}
	if len(t.Kids) == 0 {
		return t
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = writtenCells(k, borrows)
	}
	return &Term{Kind: t.Kind, Kids: kids}
}

// writesElements reports a store into b, or b lent at a borrowed parameter.
func writesElements(t *Term, b string, borrows map[string]BorrowDecl) bool {
	if t == nil {
		return false
	}
	if t.Kind == KFn {
		return writesElements(t.Kids[0], b, borrows)
	}
	if t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName {
		op := t.Kids[0].Name
		if op == "set" && len(t.Kids) == 4 && t.Kids[1].Kind == KName && t.Kids[1].Name == b {
			return true
		}
		if d, ok := borrows[op]; ok {
			for i, a := range t.Kids[1:] {
				if d.at(i) && a.Kind == KName && a.Name == b {
					return true
				}
			}
		}
	}
	for _, k := range t.Kids {
		if writesElements(k, b, borrows) {
			return true
		}
	}
	return false
}

// scalarLiteral reports a literal that is no table: a binder it initializes
// is a cell whatever its scope writes, and a store into it is the cell's error.
func scalarLiteral(t *Term) bool {
	switch t.Kind {
	case KInt, KDec, KStr, KBool:
		return true
	}
	return false
}

// MapCapacityDef labels the obligation an insert into a map carries, its
// binder's claim |dom m| < c before the insert (maps.md §6).
const MapCapacityDef = "a map's capacity"

// PlantCapacityUnmarked writes no capacity mark at an insert, so a map may
// take more keys than its capacity: TestAMapsCapacityIsAClaimOnLen must catch
// it.
var PlantCapacityUnmarked bool

// capacityMark is an insert into the map place x, under its binder's claim (B′):
// a map of capacity c is in {m : len(m) <= c}, so insert(m, k, v) stays in it
// when len(m) < c, a sufficient condition for its domain k ∈ dom m ∨ |dom m| < c
// (the presence of k is a fact this layer does not have). The mark is ADR
// 0028's: decided apart, its value the insert's, an undecided one refusing the
// program; its condition is never run, so repeating c there costs nothing.
func (c *cellTr) capacityMark(x string, ins *Term) *Term {
	cap, ok := c.caps[x]
	if !ok || !headIs(ins, "insert") || len(ins.Kids) != 4 || PlantCapacityUnmarked {
		return ins
	}
	cond := App(Name("<"), App(Name("len"), ins.Kids[1]), cap)
	return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(MapCapacityDef), cond, ins}}
}

// cellOffLen is a cell or buffer in scope that t mentions other than as the
// argument of `len`, or "".
func (c *cellTr) cellOffLen(t *Term) string {
	if t == nil {
		return ""
	}
	switch {
	case t.Kind == KName:
		if c.isCell(t.Name) || c.places[t.Name] {
			return t.Name
		}
		return ""
	case t.Kind == KFn:
		return c.cellOffLen(t.Body())
	case headIs(t, "len") && len(t.Kids) == 2 && t.Kids[1].Kind == KName:
		return ""
	}
	for _, k := range t.Kids {
		if n := c.cellOffLen(k); n != "" {
			return n
		}
	}
	return ""
}

// PlantCellCallTwice translates a call the scope pass already translated
// again, as before: a cell declared after an out-call is refused as no cell.
var PlantCellCallTwice bool

// CallbackArg is a host operation's callback parameter (ADR 0143): its
// argument's index, and whether its host promises the order of its calls, by
// `ordered` or by a count of at most one.
type CallbackArg struct {
	Index   int
	Ordered bool
}

// PlantCallbackCellOrderUnchecked threads a cell written under a host that
// promises no order, Theorem D unchecked: strings.Map's count is then the
// host's (TestACellWrittenInACallbackIsThreadedInTheHostsOrder).
var PlantCallbackCellOrderUnchecked bool

// PlantCallbackCellRefused refuses every λ at a callback parameter that
// mentions a cell, as before ADR 0143: the closure refusal, which
// TestACellWrittenInACallbackIsThreadedInTheHostsOrder must see.
var PlantCallbackCellRefused bool

// callbackCells rewrites a host call whose λ at a callback parameter mentions
// a cell of the scope (ADR 0143). The λ is second-class and applied only
// during the call, so its environment is the caller's frame:
//   - A CELL IT ONLY READS is a value at the call: bound to a fresh name before
//     it, the λ capturing that value, pure under any host.
//   - A CELL IT WRITES is threaded through the host's activations, Theorem
//     T₁: the program whose cell is one variable written in place, captured
//     by reference. That is ADR 0048's form with the λ the writer, a reference
//     to a fresh copy, read and written through in the λ and read back after
//     the call: r = #ref(x); v = h(…, λ′, …); x := #deref(r); v. It has the
//     host's order only where the host promises one, so it is refused
//     elsewhere (Theorem D): the count of writes would be the host's too.
//
// ok is false where no λ at a callback parameter mentions a cell.
func (c *cellTr) callbackCells(t *Term, cbs []CallbackArg) (*Term, bool, error) {
	args := t.Kids[1:]
	type use struct {
		x       string
		written bool
	}
	var uses []use
	seen := map[string]bool{}
	ordered := map[int]bool{}
	for _, cb := range cbs {
		if cb.Index >= len(args) || args[cb.Index].Kind != KFn {
			continue
		}
		ordered[cb.Index] = cb.Ordered
		for _, x := range c.xs {
			u := c.userOf(x)
			if c.places[u] || seen[u] || !mentionsName(args[cb.Index], u) {
				continue
			}
			w := writesName(args[cb.Index], u)
			if w && !cb.Ordered && !PlantCallbackCellOrderUnchecked {
				return nil, true, fmt.Errorf("%s writes the cell %s, and %s does not promise the order or the number "+
					"of its calls (no `ordered`): the cell's last value would be the host's, which may differ "+
					"between releases (Theorem D, ADR 0143). Count with a loop of your own, or pass the λ to a host "+
					"whose declaration is `ordered`", ShowTerm(args[cb.Index]), ShowName(u), ShowName(t.Kids[0].Name))
			}
			seen[u] = true
			uses = append(uses, use{u, w})
		}
	}
	if len(uses) == 0 {
		return nil, false, nil
	}
	kids := append([]*Term(nil), t.Kids...)
	var refs []string
	for i, u := range uses {
		r := fmt.Sprintf("#cbr%d", c.n+i+1)
		refs = append(refs, r)
		for _, cb := range cbs {
			if j := cb.Index + 1; j < len(kids) && kids[j].Kind == KFn {
				if u.written {
					kids[j] = loadStore(kids[j], u.x, r)
				} else {
					kids[j] = substFree(kids[j], u.x, Name(r))
				}
			}
		}
	}
	c.n += len(uses)
	// EACH λ′ IS ITS OWN SCOPE NOW, the cells it binds its own: translated
	// here, since a λ is otherwise passed on as it stands
	for _, cb := range cbs {
		if j := cb.Index + 1; j < len(kids) && kids[j].Kind == KFn {
			lam, err := translateScopes(kids[j], c.decls, c.borrows, c.cbs, c.defs, false)
			if err != nil {
				return nil, true, err
			}
			kids[j] = lam
		}
	}
	// v = h(…); then each written cell takes the reference's last value
	call := &Term{Kind: KApp, Kids: kids}
	v := fmt.Sprintf("#cbv%d", c.n)
	body := Name(v)
	for i := len(uses) - 1; i >= 0; i-- {
		if uses[i].written {
			body = App(Fn([]string{"#_"}, body), App(Name("set"), Name(uses[i].x), App(Name(DerefName), Name(refs[i]))))
		}
	}
	out := App(Fn([]string{v}, body), call)
	for i := len(uses) - 1; i >= 0; i-- {
		init := Name(uses[i].x) // a read cell: its value at the call
		if uses[i].written {
			init = App(Name(RefName), Name(uses[i].x))
		}
		out = App(Fn([]string{refs[i]}, out), init)
	}
	return out, true, nil
}

// writesName says t writes the cell x: a `(set x e)` x's own binder reaches.
func writesName(t *Term, x string) bool {
	if t == nil {
		return false
	}
	if headIs(t, "set") && len(t.Kids) == 3 && t.Kids[1].Kind == KName && t.Kids[1].Name == x {
		return true
	}
	for _, k := range t.Kids {
		if writesName(k, x) {
			return true
		}
	}
	return false
}

// loadStore is the λ fn(p…) { e } with the cell x threaded through the
// reference r, Theorem T₁'s λ_S : S × A → S × R: the state enters an
// activation once and leaves it once,
//
//	fn(p…) { var x := #deref(r); v = e; #store-ref(r, x); v }
//
// so within one activation x is an ordinary cell of the λ's own, a write
// under a guard a φ the guard bounds, and the host's order threads the
// activations through r. A λ's own parameters are indices, so x in e is the
// cell, now the inner scope's.
func loadStore(lam *Term, x, r string) *Term {
	v := "#cbres"
	store := App(Fn([]string{"#_"}, Name(v)), App(Name(StoreRefName), Name(r), Name(x)))
	inner := App(Fn([]string{v}, store), lam.Body())
	scope := App(Name(CellName), App(Name(DerefName), Name(r)), Fn([]string{x}, inner))
	return Fn(lam.Params, scope)
}
