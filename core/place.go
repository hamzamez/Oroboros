package core

import (
	"fmt"
	"strings"
)

// A BUFFER IN PROGRAM ORDER (ADR 0052, spec/local.md §2.1).
//
// A buffer is a state binder whose value is a table: a cell written element by
// element, Ref(Seq(A)) with point update. Its meaning is §3's state-passing
// translation S, whose image is the single-threaded programs (Schmidt 1985),
// which are today's threaded ones. D writes each new form by its equation, so
// S meets only cell writes, `(set b (set b i v))` being b := set(b, i, v), and
// the one new clause, the BORROW: a call that lends a buffer at a linear
// parameter, whose callee hands it back first, then its result, a unit result
// dropped (B × 1 ≅ B):
//
//	S⟦f(…, b, …)⟧ = let (b', r̄) = f(…, b, …) in (r̄, σ[b ↦ b'])
//
// RULES R0–R4 are Reynolds' syntactic control of interference (1978) and the
// freshness of a buffer's value; checkPlace decides them on each place's scope,
// so that S's image is single-threaded by construction.

// BorrowDecl is what a callee says about the buffers it borrows: the indices of
// its linear parameters, and how many components its result has after the
// buffers it hands back (0 for a unit result).
type BorrowDecl struct {
	Params  []int
	Results int
}

// BorrowDeclTerm is the form a target hands the loader, (borrows RESULTS i…).
func BorrowDeclTerm(d BorrowDecl) *Term {
	kids := []*Term{Name("borrows"), Int(int64(d.Results))}
	for _, i := range d.Params {
		kids = append(kids, Int(int64(i)))
	}
	return &Term{Kind: KApp, Kids: kids}
}

func readBorrowDecl(t *Term) (BorrowDecl, error) {
	if t == nil || t.Kind != KApp || len(t.Kids) < 3 {
		return BorrowDecl{}, fmt.Errorf("a borrows form is (borrows RESULTS i…), got %s", t)
	}
	var ints []int
	for _, k := range t.Kids[1:] {
		if k.Kind != KInt {
			return BorrowDecl{}, fmt.Errorf("a borrows form is (borrows RESULTS i…), got %s", t)
		}
		ints = append(ints, int(k.Int))
	}
	return BorrowDecl{Results: ints[0], Params: ints[1:]}, nil
}

// At reports whether argument i of a call is a borrowed parameter.
func (d BorrowDecl) At(i int) bool { return d.at(i) }

func (d BorrowDecl) at(i int) bool {
	for _, p := range d.Params {
		if p == i {
			return true
		}
	}
	return false
}

// isBorrowScope reports a λ body whose parameters are places: (#borrow p…
// (fn (p…) body)).
func isBorrowScope(t *Term) bool {
	if t.Kind != KApp || len(t.Kids) < 3 || t.Kids[0].Kind != KName || t.Kids[0].Name != BorrowName {
		return false
	}
	lam := t.Kids[len(t.Kids)-1]
	return lam.Kind == KFn && len(lam.Params) == len(t.Kids)-2
}

// BorrowsOf is what a definition's value says it borrows, when it is a λ whose
// body is a borrow scope: its places' parameter indices, and whether its body
// is a unit, by the declarations known so far.
func BorrowsOf(def *Term, known map[string]BorrowDecl) (BorrowDecl, bool) {
	if def == nil || def.Kind != KFn {
		return BorrowDecl{}, false
	}
	body := def.Body()
	if !isBorrowScope(body) {
		return BorrowDecl{}, false
	}
	lam := body.Kids[len(body.Kids)-1]
	var d BorrowDecl
	for _, p := range lam.Params {
		for i, q := range def.Params {
			if p == q {
				d.Params = append(d.Params, i)
			}
		}
	}
	if !unitBody(lam.Body(), known) {
		d.Results = 1
	}
	return d, true
}

// unitBody reports whether a body's value is the unit at every tail: `()`, a
// write, or a borrow whose result is the unit. The unit law B × 1 ≅ B then
// drops it from the callee's value (spec/local.md §2.1).
func unitBody(t *Term, borrows map[string]BorrowDecl) bool {
	switch {
	case isUnit(t):
		return true
	case t.Kind == KFn:
		return false
	case t.Kind != KApp || len(t.Kids) == 0:
		return false
	}
	op, args := t.Kids[0], t.Kids[1:]
	switch {
	case headIs(t, "set") && len(args) == 2:
		return true
	case headIs(t, "if") && len(args) == 3:
		return unitBody(args[1], borrows) && unitBody(args[2], borrows)
	case headIs(t, "loop") && len(args) >= 1 && args[0].Kind == KFn:
		return unitExits(args[0].Body(), borrows)
	case op.Kind == KFn && len(op.Params) == len(args): // a let, a seq
		return unitBody(op.Body(), borrows)
	case len(args) == 1 && args[0].Kind == KFn: // an elimination's continuation
		return unitBody(args[0].Body(), borrows)
	case op.Kind == KName:
		if d, ok := borrows[op.Name]; ok {
			return d.Results == 0
		}
	}
	return false
}

// unitExits is unitBody over a loop's chain: every exit, the jumps aside.
func unitExits(t *Term, borrows map[string]BorrowDecl) bool {
	switch {
	case isAgain(t):
		return true
	case headIs(t, "if") && len(t.Kids) == 4:
		return unitExits(t.Kids[2], borrows) && unitExits(t.Kids[3], borrows)
	case t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KFn && len(t.Kids[0].Params) == len(t.Kids)-1:
		return unitExits(t.Kids[0].Body(), borrows)
	case t.Kind == KApp && len(t.Kids) == 2 && t.Kids[1].Kind == KFn:
		return unitExits(t.Kids[1].Body(), borrows)
	}
	return unitBody(t, borrows)
}

// isUnit is the unit's term, the empty tuple (fn (#k) (#k)).
func isUnit(t *Term) bool {
	if t.Kind != KFn || len(t.Params) != 1 || !IsTupleBinder(t.Params[0]) {
		return false
	}
	b := t.Body()
	return b.Kind == KApp && len(b.Kids) == 1 && b.Kids[0].Kind == KName && b.Kids[0].Name == t.Params[0]
}

// tupleTerm is the product of ts, or the one term itself.
func tupleTerm(ts []*Term) *Term {
	if len(ts) == 1 {
		return ts[0]
	}
	return Fn([]string{"#k"}, App(Name("#k"), ts...))
}

// writes reports whether t writes the state binder x: a write, a borrow that
// lends x, or a host writing x as a cell. A loop carries what it writes
// (spec/local.md §2.1); what it only reads is free in it.
func (c *cellTr) writes(t *Term, x string) bool {
	if t == nil || len(t.Kids) == 0 {
		return false
	}
	if t.Kind == KFn {
		return c.writes(t.Kids[0], x)
	}
	if headIs(t, "set") && len(t.Kids) == 3 && t.Kids[1].Kind == KName && t.Kids[1].Name == x {
		return true
	}
	if d, ok := c.borrowCallOf(t); ok {
		for i, a := range t.Kids[1:] {
			if d.at(i) && a.Kind == KName && a.Name == x {
				return true
			}
		}
	}
	if d, ok := c.cellCall(t); ok {
		for i, a := range t.Kids[1:] {
			if d.at(i) && a.Kind == KName && a.Name == x {
				return true
			}
		}
	}
	for _, k := range t.Kids {
		if c.writes(k, x) {
			return true
		}
	}
	return false
}

// phi reports an if whose branches are statements, unit-valued and with no
// jump, that write exactly one state binder between them: its value is a φ on
// that binder (cell.go).
func (c *cellTr) phi(a, b *Term, inLoop bool) (string, bool) {
	if hasOwnAgain(a) || hasOwnAgain(b) || !unitBody(a, c.borrows) || !unitBody(b, c.borrows) {
		return "", false
	}
	x := ""
	for _, n := range c.xs {
		if c.writes(a, c.userOf(n)) || c.writes(b, c.userOf(n)) {
			if x != "" {
				return "", false
			}
			x = n
		}
	}
	return x, x != ""
}

// borrowCallOf reports a call of a callee that borrows, whatever its arguments.
func (c *cellTr) borrowCallOf(t *Term) (BorrowDecl, bool) {
	if t == nil || t.Kind != KApp || len(t.Kids) == 0 || t.Kids[0].Kind != KName {
		return BorrowDecl{}, false
	}
	d, ok := c.borrows[t.Kids[0].Name]
	return d, ok
}

// borrowCall reports a call that lends places: a callee that borrows, with a
// place in scope at every linear parameter. A call of one with something else
// there lends nothing S can rebind, and is left as written.
func (c *cellTr) borrowCall(t *Term) (BorrowDecl, bool) {
	d, ok := c.borrowCallOf(t)
	if !ok {
		return d, false
	}
	for _, i := range d.Params {
		if i+1 >= len(t.Kids) {
			return d, false
		}
		a := t.Kids[i+1]
		if a.Kind != KName || !c.places[a.Name] {
			return d, false
		}
	}
	return d, true
}

// translateBorrow is S⟦·⟧ of a call that lends places, with k its continuation
// when it is eliminated, nil when it stands as a statement or a value: the
// callee's value is the lent buffers' last versions, then its result's
// components, and each place is rebound to its version under its own name.
func (c *cellTr) translateBorrow(call *Term, d BorrowDecl, k *Term, inLoop bool, carried []string) (*Term, error) {
	var lent []string
	for _, i := range d.Params {
		lent = append(lent, call.Kids[i+1].Name)
	}
	var body *Term
	if k != nil {
		if len(k.Params) != d.Results {
			return nil, fmt.Errorf("%s hands back its buffer, and its result has %d component(s); the λ it "+
				"is applied to takes %d (spec/local.md §2.1)", call, d.Results, len(k.Params))
		}
		for _, p := range k.Params {
			c.locals[p] = true
		}
		b, err := c.stLoop(k.Body(), inLoop, carried)
		if err != nil {
			return nil, err
		}
		body = b
	}
	head := call.Kids[0]
	return c.thread(call.Kids[1:], inLoop, carried, true, func(vs []*Term) *Term {
		call := App(head, vs...)
		if k != nil {
			binders := make([]string, len(lent))
			for j, b := range lent {
				binders[j] = c.in(b) // the place's state, rebound to the handed-back version
			}
			return App(call, Fn(append(binders, k.Params...), body))
		}
		if d.Results == 0 && len(lent) == 1 {
			// THE HANDED-BACK BUFFER ALONE goes in its place's slot of the state,
			// as an element write's store does (cell.go).
			after := c.state()
			for i, x := range c.xs {
				if c.userOf(x) == lent[0] {
					after[i] = call
				}
			}
			return tupOf(Unit(), after)
		}
		rs := make([]string, d.Results)
		rv := make([]*Term, d.Results)
		for j := range rs {
			rs[j] = c.fresh()
			rv[j] = Name(rs[j])
		}
		value := Unit()
		if d.Results > 0 {
			value = tupleTerm(rv)
		}
		// THE HANDED-BACK BUFFERS are versions, bound under fresh names and put
		// in the state in their places' stead; β puts each where it is used
		// once, first (reduce.go, firstUse).
		nv := make([]string, len(lent))
		after := c.state()
		for j, b := range lent {
			nv[j] = c.freshVersion()
			for i, x := range c.xs {
				if c.userOf(x) == b {
					after[i] = Name(nv[j])
				}
			}
		}
		names := append(nv, rs...)
		if len(names) == 1 {
			return App(Fn(names, tupOf(value, after)), call)
		}
		return App(call, Fn(names, tupOf(value, after)))
	})
}

// checkPlace decides R0–R4 for the place b over its scope's body (spec/local.md
// §2.1). borrowed says b is a λ's borrowed parameter, the caller's, which the
// λ's value never names (R4).
func (c *cellTr) checkPlace(body *Term, b string, borrowed bool) error {
	pc := &placeCheck{c: c, b: b, borrowed: borrowed}
	return pc.walk(body, true)
}

type placeCheck struct {
	c        *cellTr
	b        string
	borrowed bool
}

func (pc *placeCheck) is(t *Term) bool { return t.Kind == KName && t.Name == pc.b }

func (pc *placeCheck) refuse(rule, what string, t *Term) error {
	return fmt.Errorf("%s: the buffer %s %s (%s; spec/local.md §2.1, ADR 0052): %s",
		rule, pc.b, what, map[string]string{
			"R0": "a buffer's value is fresh",
			"R1": "a buffer is named only by its binder",
			"R2": "a call borrows a buffer once",
			"R3": "no closure over a buffer",
			"R4": "the scope's value freezes the buffer, and a borrowed one is the caller's",
		}[rule], t)
}

// walk checks every occurrence of b in t; tail says t is a tail of the scope's
// body, where its value is the scope's.
func (pc *placeCheck) walk(t *Term, tail bool) error {
	switch t.Kind {
	case KName:
		if pc.is(t) {
			if !tail {
				return pc.refuse("R1", "is named as a value", t)
			}
			if pc.borrowed {
				return pc.refuse("R4", "is the value of the λ that borrows it", t)
			}
		}
		return nil
	case KFn:
		if mentionsName(t, pc.b) {
			// A TUPLE AT A TAIL is the scope's value, a product of frozen buffers
			// and values (ADR 0031); any other λ is a closure.
			if body := t.Body(); len(t.Params) == 1 && IsTupleBinder(t.Params[0]) && headIs(body, t.Params[0]) {
				for _, k := range body.Kids[1:] {
					if pc.is(k) {
						if !tail {
							return pc.refuse("R1", "is a component of a tuple", t)
						}
						if pc.borrowed {
							return pc.refuse("R4", "is the value of the λ that borrows it", t)
						}
						continue
					}
					if err := pc.walk(k, false); err != nil {
						return err
					}
				}
				return nil
			}
			return pc.refuse("R3", "is mentioned by a function", t)
		}
		return nil
	case KApp:
	default:
		return nil
	}
	op, args := t.Kids[0], t.Kids[1:]
	switch {
	case headIs(t, "set") && len(args) == 2 && pc.is(args[0]):
		// A WRITE of b: an element's, b := set(b, i, v) or set((b i), j, v); a
		// move in a concurrent assignment, b := #w; nothing else (R0).
		v := args[1]
		if headIs(v, "set") && len(v.Kids) == 4 {
			at := v.Kids[1]
			for at.Kind == KApp && len(at.Kids) == 2 {
				if err := pc.walk(at.Kids[1], false); err != nil {
					return err
				}
				at = at.Kids[0]
			}
			if pc.is(at) {
				return pc.walks(v.Kids[2:])
			}
		}
		if v.Kind == KName && strings.HasPrefix(v.Name, "#w") {
			return nil
		}
		return pc.refuse("R0", "is given a value that is not fresh", t)
	case headIs(t, "set") && len(args) == 2 && pc.is(args[1]):
		return pc.refuse("R1", "is the value written to another place", t)
	case headIs(t, "set") && len(args) == 3 && (pc.is(args[0]) || pc.indexes(args[0])):
		return pc.refuse("R1", "is stored into outside its own write, which makes a second version", t)
	case headIs(t, "again"):
		for _, a := range args {
			if pc.is(a) {
				return pc.refuse("R1", "is handed to a jump; the loop carries it", t)
			}
		}
		return pc.walks(args)
	case pc.is(op):
		// AN INDEX: b[i], and b[i][j] through the application above it.
		return pc.walks(args)
	case op.Kind == KApp && len(op.Kids) == 2 && pc.is(op.Kids[0]):
		if err := pc.walk(op.Kids[1], false); err != nil {
			return err
		}
		return pc.walks(args)
	case (headIs(t, "build") || headIs(t, "build-map")) && len(args) == 2 && args[1].Kind == KFn,
		isCellScope(t):
		// A REGION'S λ RUNS ONCE, NOW: build's, or a nested scope's.
		if err := pc.walk(args[0], false); err != nil {
			return err
		}
		return pc.walk(args[1].Body(), tail)
	case headIs(t, "if") && len(args) == 3:
		if err := pc.walk(args[0], false); err != nil {
			return err
		}
		if err := pc.walk(args[1], tail); err != nil {
			return err
		}
		return pc.walk(args[2], tail)
	case headIs(t, "loop") && len(args) >= 1 && args[0].Kind == KFn:
		if err := pc.walks(args[1:]); err != nil {
			return err
		}
		return pc.walk(args[0].Body(), tail)
	case op.Kind == KFn && len(op.Params) == len(args):
		// A LET, A SEQ, or a concurrent assignment's bindings: b bound to a #w
		// is moved, legal when the assignment writes b too (R0's permutation).
		moves := len(op.Params) > 0 && strings.HasPrefix(op.Params[0], "#w")
		for _, a := range args {
			if pc.is(a) {
				if !moves {
					return pc.refuse("R1", "is bound to another name", t)
				}
				if !pc.writesItself(op.Body()) {
					return pc.refuse("R0", "is moved to another place, and its own is not written in the same assignment", t)
				}
				continue
			}
			if err := pc.walk(a, false); err != nil {
				return err
			}
		}
		return pc.walk(op.Body(), tail)
	case len(args) == 1 && args[0].Kind == KFn && !pc.isTupleLam(args[0]):
		// AN ELIMINATION: the scrutinee, then the continuation, a tail if t is.
		if err := pc.walk(op, false); err != nil {
			return err
		}
		return pc.walk(args[0].Body(), tail)
	}
	// A CALL: b once among its arguments (R2), by reference.
	if op.Kind == KName || op.Kind == KApp {
		n := 0
		for _, a := range args {
			if pc.is(a) {
				n++
			}
		}
		if n > 1 {
			return pc.refuse("R2", "is handed to one call twice", t)
		}
		if pc.is(op) {
			return pc.refuse("R1", "is applied", t)
		}
		if err := pc.walk(op, false); err != nil {
			return err
		}
		for _, a := range args {
			if pc.is(a) {
				continue
			}
			if err := pc.walk(a, false); err != nil {
				return err
			}
		}
		return nil
	}
	return pc.walks(t.Kids)
}

func (pc *placeCheck) walks(ts []*Term) error {
	for _, t := range ts {
		if err := pc.walk(t, false); err != nil {
			return err
		}
	}
	return nil
}

// indexes reports b[i], b[i][j], … : an application chain headed by b.
func (pc *placeCheck) indexes(t *Term) bool {
	for t.Kind == KApp && len(t.Kids) == 2 {
		t = t.Kids[0]
	}
	return pc.is(t)
}

func (pc *placeCheck) isTupleLam(t *Term) bool {
	return len(t.Params) == 1 && IsTupleBinder(t.Params[0])
}

// writesItself reports whether a concurrent assignment's writes include b.
func (pc *placeCheck) writesItself(t *Term) bool {
	for {
		if headIs(t, "set") && len(t.Kids) == 3 && pc.is(t.Kids[1]) {
			return true
		}
		if t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KFn {
			if headIs(t.Kids[1], "set") && len(t.Kids[1].Kids) == 3 && pc.is(t.Kids[1].Kids[1]) {
				return true
			}
			t = t.Kids[0].Body()
			continue
		}
		return false
	}
}
