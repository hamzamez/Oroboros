package j

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"oroboros/core"
)

// UNTHREADING, U (ADR 0052, spec/local.md §2.1, storeorder-research §2.3). The
// converse of the state-passing translation S on today's corpus: a buffer
// threaded as a value becomes a place written in program order. Today's
// linearity check makes the threading single: each version is consumed once,
// in evaluation order, so every name a buffer's chain is bound to IS the place,
// and U renames each to the place's binder. Its cases:
//
//   - a `local` buffer is a state binder, `var b := array { … }`;
//   - a store chain is element writes, `(set (set b i x) j y)` ↦ b[i] := x; b[j] := y;
//   - a call that hands a buffer back is a borrow, a statement or a binding of
//     the rest of its result; a definition that returns the buffer it was given
//     borrows it, `fn f(b linear, …)`;
//   - a loop forgets its carried buffers: each back edge's carrier argument
//     becomes the writes before the jump, and a carrier handed another place's
//     buffer is a move, the moves together one concurrent assignment;
//   - a version given another name is the place, under its own.
//
// S of U's output is today's residual; the ORO_VIA_J gate checks that on every
// program, byte for byte.

type unthreader struct {
	// borrows are the callees that borrow, by the name a call writes: this
	// file's definitions, an imported module's as alias.member, a host's.
	borrows map[string]core.BorrowDecl
	origin  map[core.Tail]core.Tail
	fresh   int
}

// places maps the names of a buffer's versions in scope to the place's name.
type places map[string]string

func (s places) with(kv ...string) places {
	out := places{}
	for k, v := range s {
		out[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		out[kv[i]] = kv[i+1]
	}
	return out
}

func (s places) without(names ...string) places {
	hit := false
	for _, n := range names {
		if _, ok := s[n]; ok {
			hit = true
		}
	}
	if !hit {
		return s
	}
	out := places{}
	for k, v := range s {
		out[k] = v
	}
	for _, n := range names {
		delete(out, n)
	}
	return out
}

// uitem is a block's statement: a binding or an expression run for its effect.
type uitem struct {
	bind   *core.LetBind
	expr   core.Expr
	origin core.Tail
	unit   bool // expr's value is the unit: a write, a borrow, a loop or an if U made of writes
}

func unit() core.Expr { return &core.Apply{Fn: &core.Var{Name: "tuple"}} }

func (u *unthreader) from(n, old core.Tail) {
	if old != nil {
		if o, ok := u.origin[old]; ok {
			old = o
		}
		u.origin[n] = old
	}
}

// unitBlock is a block of statements whose value is the unit: its last
// statement where that is a write, a loop or an if run for its effect, else ().
func (u *unthreader) unitBlock(items []uitem) core.Expr {
	if n := len(items); n > 0 && items[n-1].bind == nil && items[n-1].unit {
		return u.fold(items[:n-1], items[n-1].expr)
	}
	return u.fold(items, unit())
}

// fold makes items and a final value one block's tree.
func (u *unthreader) fold(items []uitem, value core.Expr) core.Expr {
	acc := value
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		if it.bind != nil {
			acc = &core.Let{Binds: []core.LetBind{*it.bind}, Body: acc}
		} else if seq, ok := acc.(*core.Seq); ok {
			acc = &core.Seq{Xs: append([]core.Expr{it.expr}, seq.Xs...)}
		} else {
			acc = &core.Seq{Xs: []core.Expr{it.expr, acc}}
		}
		u.from(acc, it.origin)
	}
	return acc
}

func (u *unthreader) foldTail(items []uitem, t core.Tail) core.Tail {
	if e, ok := t.(core.Expr); ok {
		return u.fold(items, e)
	}
	acc := t
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		if it.bind != nil {
			acc = &core.TLet{Binds: []core.LetBind{*it.bind}, Body: acc}
		} else if ts, ok := acc.(*core.TSeq); ok {
			acc = &core.TSeq{Xs: append([]core.Expr{it.expr}, ts.Xs...), Last: ts.Last}
		} else {
			acc = &core.TSeq{Xs: []core.Expr{it.expr}, Last: acc}
		}
		u.from(acc, it.origin)
	}
	return acc
}

// callee is the name a call writes, and its borrow declaration.
func (u *unthreader) callee(e core.Expr) (string, core.BorrowDecl, bool) {
	a, ok := e.(*core.Apply)
	if !ok {
		return "", core.BorrowDecl{}, false
	}
	v, ok := a.Fn.(*core.Var)
	if !ok {
		return "", core.BorrowDecl{}, false
	}
	d, ok := u.borrows[v.Name]
	return v.Name, d, ok
}

// version reports the place whose version e's value is.
func (u *unthreader) version(e core.Expr, sc places) (string, bool) {
	switch e := e.(type) {
	case *core.Var:
		q, ok := sc[e.Name]
		return q, ok
	case *core.Apply:
		if v, ok := e.Fn.(*core.Var); ok && (v.Name == "set" || v.Name == "insert") && len(e.Args) == 3 {
			if q, ok := u.version(e.Args[0], sc); ok {
				return q, true
			}
			if rec, ok := e.Args[0].(*core.Apply); ok && len(rec.Args) == 1 {
				return u.version(rec.Fn, sc)
			}
			return "", false
		}
		if _, d, ok := u.callee(e); ok && d.Results == 0 && len(d.Params) == 1 && d.Params[0] < len(e.Args) {
			return u.version(e.Args[d.Params[0]], sc)
		}
	case *core.Let:
		inner := sc
		for _, b := range e.Binds {
			inner = u.bindScope(b, inner)
		}
		return u.version(e.Body, inner)
	case *core.Seq:
		return u.version(e.Xs[len(e.Xs)-1], sc)
	case *core.If:
		a, oka := u.version(e.A, sc)
		b, okb := u.version(e.B, sc)
		if oka && okb && a == b {
			return a, true
		}
	case *core.Loop:
		carriers := u.carriers(e, sc)
		if len(carriers) == 0 {
			return "", false
		}
		inner := u.loopScope(e, sc, carriers)
		q := ""
		for _, a := range e.Arms {
			if endsInJump(a.Body) {
				continue
			}
			x, ok := u.versionTail(a.Body, inner)
			if !ok || (q != "" && x != q) {
				return "", false
			}
			q = x
		}
		return q, q != ""
	}
	return "", false
}

// endsInJump reports a loop arm whose tail is a back edge, past its bindings.
func endsInJump(t core.Tail) bool {
	switch t := t.(type) {
	case *core.Again:
		return true
	case *core.TLet:
		return endsInJump(t.Body)
	case *core.TSeq:
		return endsInJump(t.Last)
	case *core.TMonad:
		return endsInJump(t.Body)
	}
	return false
}

func (u *unthreader) versionTail(t core.Tail, sc places) (string, bool) {
	switch t := t.(type) {
	case *core.TLet:
		inner := sc
		for _, b := range t.Binds {
			inner = u.bindScope(b, inner)
		}
		return u.versionTail(t.Body, inner)
	case *core.TSeq:
		return u.versionTail(t.Last, sc)
	case core.Expr:
		return u.version(t, sc)
	}
	return "", false
}

// bindScope is the scope after a binding: a version given a name is the place.
func (u *unthreader) bindScope(b core.LetBind, sc places) places {
	switch p := b.Pat.(type) {
	case *core.PVar:
		if q, ok := u.version(b.Value, sc); ok {
			return sc.with(p.Name, q)
		}
		return sc.without(p.Name)
	case *core.PTuple:
		if lent, ok := u.lent(b.Value, sc); ok && len(p.Names) > len(lent) {
			out := sc.without(p.Names...)
			for i, q := range lent {
				out = out.with(p.Names[i], q)
			}
			return out
		}
		return sc.without(p.Names...)
	}
	return sc
}

// lent reports a borrow whose result is a product: the places it lends, in
// order, which the result hands back first.
func (u *unthreader) lent(e core.Expr, sc places) ([]string, bool) {
	a, ok := e.(*core.Apply)
	if !ok {
		return nil, false
	}
	_, d, ok := u.callee(e)
	if !ok || d.Results == 0 {
		return nil, false
	}
	var qs []string
	for _, i := range d.Params {
		if i >= len(a.Args) {
			return nil, false
		}
		q, ok := u.version(a.Args[i], sc)
		if !ok {
			return nil, false
		}
		qs = append(qs, q)
	}
	return qs, true
}

// carriers are a loop's variables whose initializers are versions of places.
func (u *unthreader) carriers(e *core.Loop, sc places) map[int]string {
	out := map[int]string{}
	if e.Match {
		return out
	}
	for j, init := range e.Inits {
		if q, ok := u.version(init, sc); ok {
			out[j] = q
		}
	}
	return out
}

func (u *unthreader) loopScope(e *core.Loop, sc places, carriers map[int]string) places {
	inner := sc.without(e.Vars...)
	for j, q := range carriers {
		inner = inner.with(e.Vars[j], q)
	}
	return inner
}

// ---- statements: a version of a place, as writes ----

// stmts is the writes that make place q hold e's value, e a version of q.
func (u *unthreader) stmts(e core.Expr, q string, sc places) ([]uitem, error) {
	switch e := e.(type) {
	case *core.Var:
		return nil, nil
	case *core.Apply:
		if v, ok := e.Fn.(*core.Var); ok && (v.Name == "set" || v.Name == "insert") && len(e.Args) == 3 {
			// AN ELEMENT WRITE, a table's store or a map's insert (ADR 0058): the
			// chain's earlier writes, then this one. A
			// store's index and value are read before it, in the state before.
			at := []core.Expr{u.value(e.Args[1], sc)}
			base := e.Args[0]
			if _, ok := u.version(base, sc); !ok {
				rec := base.(*core.Apply)
				at = append([]core.Expr{u.value(rec.Args[0], sc)}, at...)
				base = rec.Fn
			}
			before, err := u.stmts(base, q, sc)
			if err != nil {
				return nil, err
			}
			w := &core.Write{Places: []core.Place{{Name: q, At: at}}, Values: []core.Expr{u.value(e.Args[2], sc)}}
			u.from(w, e)
			return append(before, uitem{expr: w, origin: e, unit: true}), nil
		}
		if _, d, ok := u.callee(e); ok && d.Results == 0 {
			// A BORROW: the buffer's earlier writes, then the call, which writes it.
			k := d.Params[0]
			before, err := u.stmts(e.Args[k], q, sc)
			if err != nil {
				return nil, err
			}
			call := u.call(e, d, sc)
			return append(before, uitem{expr: call, origin: e, unit: true}), nil
		}
	case *core.Let:
		var items []uitem
		inner := sc
		for _, b := range e.Binds {
			its, next, err := u.binding(b, inner)
			if err != nil {
				return nil, err
			}
			items, inner = append(items, its...), next
		}
		rest, err := u.stmts(e.Body, q, inner)
		return append(items, rest...), err
	case *core.Seq:
		var items []uitem
		for _, x := range e.Xs[:len(e.Xs)-1] {
			items = append(items, uitem{expr: u.value(x, sc), origin: x})
		}
		rest, err := u.stmts(e.Xs[len(e.Xs)-1], q, sc)
		return append(items, rest...), err
	case *core.If:
		a, err := u.stmts(e.A, q, sc)
		if err != nil {
			return nil, err
		}
		b, err := u.stmts(e.B, q, sc)
		if err != nil {
			return nil, err
		}
		x := &core.If{C: u.value(e.C, sc), A: u.unitBlock(a), B: u.unitBlock(b)}
		u.from(x, e)
		return []uitem{{expr: x, origin: e, unit: true}}, nil
	case *core.Loop:
		items, loop, err := u.loop(e, sc, true)
		if err != nil {
			return nil, err
		}
		return append(items, uitem{expr: loop, origin: e, unit: true}), nil
	}
	return nil, fmt.Errorf("U: no writes for a version of %s: %T", q, e)
}

// call is a borrow's call with each lent version spelled as its place.
func (u *unthreader) call(e *core.Apply, d core.BorrowDecl, sc places) *core.Apply {
	args := make([]core.Expr, len(e.Args))
	for i, a := range e.Args {
		if d.At(i) {
			q, _ := u.version(a, sc)
			args[i] = &core.Var{Name: q}
			continue
		}
		args[i] = u.value(a, sc)
	}
	out := &core.Apply{Fn: e.Fn, Args: args}
	u.from(out, e)
	return out
}

// binding is one binding of a let, as items, and the scope after it: a version
// renamed is its writes; a borrow with a result binds the result's rest.
func (u *unthreader) binding(b core.LetBind, sc places) ([]uitem, places, error) {
	switch p := b.Pat.(type) {
	case *core.PVar:
		if q, ok := u.version(b.Value, sc); ok {
			its, err := u.stmts(b.Value, q, sc)
			return its, sc.with(p.Name, q), err
		}
	case *core.PTuple:
		if lent, ok := u.lent(b.Value, sc); ok && len(p.Names) > len(lent) {
			a := b.Value.(*core.Apply)
			_, d, _ := u.callee(a)
			var before []uitem
			for j, k := range d.Params {
				its, err := u.stmts(a.Args[k], lent[j], sc)
				if err != nil {
					return nil, nil, err
				}
				before = append(before, its...)
			}
			call := u.call(a, d, sc)
			rest := p.Names[len(lent):]
			next := u.bindScope(b, sc)
			if len(rest) == 1 {
				return append(before, uitem{bind: &core.LetBind{Pat: &core.PVar{Name: rest[0]}, Value: call}, origin: b.Value}), next, nil
			}
			return append(before, uitem{bind: &core.LetBind{Pat: &core.PTuple{Names: rest}, Value: call}, origin: b.Value}), next, nil
		}
	}
	nb := core.LetBind{Pat: b.Pat, Value: u.value(b.Value, sc)}
	return []uitem{{bind: &nb, origin: b.Value}}, u.bindScope(b, sc), nil
}

// ---- values ----

// value is e in a value position, every version spelled as its place.
func (u *unthreader) value(e core.Expr, sc places) core.Expr {
	out, err := u.valueE(e, sc)
	if err != nil {
		panic(unthreadError{err})
	}
	return out
}

type unthreadError struct{ err error }

// valueOrVersion is e where a value stands that may be a version: its writes,
// then the place.
func (u *unthreader) valueOrVersion(e core.Expr, sc places) core.Expr {
	if _, loop := e.(*core.Loop); loop {
		// A LOOP WHERE A VALUE STANDS keeps its exits' values, as written.
		return u.value(e, sc)
	}
	if _, plain := e.(*core.Var); !plain {
		if q, ok := u.version(e, sc); ok {
			its, err := u.stmts(e, q, sc)
			if err != nil {
				panic(unthreadError{err})
			}
			v := &core.Var{Name: q}
			return u.fold(its, v)
		}
	}
	return u.value(e, sc)
}

func (u *unthreader) values(es []core.Expr, sc places) []core.Expr {
	out := make([]core.Expr, len(es))
	for i, e := range es {
		out[i] = u.value(e, sc)
	}
	return out
}

func (u *unthreader) valueE(e core.Expr, sc places) (core.Expr, error) {
	if e == nil {
		return nil, nil
	}
	var out core.Expr
	switch e := e.(type) {
	case *core.Var:
		if q, ok := sc[e.Name]; ok && q != e.Name {
			out = &core.Var{Name: q}
		} else {
			return e, nil
		}
	case *core.Lit, *core.EmptyList:
		return e, nil
	case *core.Lam:
		out = &core.Lam{Params: e.Params, Body: u.value(e.Body, sc.without(e.Params...)), Claim: e.Claim, Borrows: e.Borrows}
	case *core.Apply:
		if lam, ok := cont(e); ok {
			// A CALL'S CONTINUATION, ((f a…) (fn (q r…) body)): a borrow binds
			// the result's rest, its buffers handed back under the places' names.
			if lent, ok := u.lent(e.Fn, sc); ok && len(lam.Params) > len(lent) {
				b := core.LetBind{Pat: &core.PTuple{Names: lam.Params}, Value: e.Fn}
				its, next, err := u.binding(b, sc)
				if err != nil {
					return nil, err
				}
				out = u.fold(its, u.valueOrVersion(lam.Body, next))
				break
			}
		}
		if _, ok := u.version(e, sc); ok {
			out = u.valueOrVersion(e, sc)
			break
		}
		out = &core.Apply{Fn: u.value(e.Fn, sc), Args: u.values(e.Args, sc)}
	case *core.Index:
		out = &core.Index{Table: u.value(e.Table, sc), At: u.value(e.At, sc)}
	case *core.Let:
		var items []uitem
		inner := sc
		for _, b := range e.Binds {
			its, next, err := u.binding(b, inner)
			if err != nil {
				return nil, err
			}
			items, inner = append(items, its...), next
		}
		out = u.fold(items, u.valueOrVersion(e.Body, inner))
	case *core.Seq:
		var items []uitem
		for _, x := range e.Xs[:len(e.Xs)-1] {
			items = append(items, uitem{expr: u.value(x, sc), origin: x})
		}
		out = u.fold(items, u.valueOrVersion(e.Xs[len(e.Xs)-1], sc))
	case *core.If:
		out = &core.If{C: u.value(e.C, sc), A: u.valueOrVersion(e.A, sc), B: u.valueOrVersion(e.B, sc)}
	case *core.And:
		out = &core.And{Xs: u.values(e.Xs, sc)}
	case *core.Or:
		out = &core.Or{Xs: u.values(e.Xs, sc)}
	case *core.Not:
		out = &core.Not{X: u.value(e.X, sc)}
	case *core.Cond:
		arms := make([]core.CondArm, len(e.Arms))
		for i, a := range e.Arms {
			arms[i] = core.CondArm{Guard: u.value(a.Guard, sc), Body: u.valueOrVersion(a.Body, sc)}
			u.from(arms[i].Body, a.Body)
		}
		out = &core.Cond{Arms: arms}
	case *core.Case:
		arms := make([]core.CaseArm, len(e.Arms))
		for i, a := range e.Arms {
			arms[i] = core.CaseArm{Pat: a.Pat, Body: u.valueOrVersion(a.Body, sc.without(patNames(a.Pat)...))}
		}
		out = &core.Case{Scrut: u.value(e.Scrut, sc), Arms: arms}
	case *core.The:
		out = &core.The{Set: e.Set, E: u.value(e.E, sc)}
	case *core.Monad:
		out = &core.Monad{Form: e.Form, Items: u.values(e.Items, sc), Body: u.valueOrVersion(e.Body, sc.without(patNames(e.Items[0])...))}
	case *core.Local:
		binds := make([]core.LocalBind, len(e.Binds))
		inner := sc
		for i, b := range e.Binds {
			binds[i] = core.LocalBind{Name: b.Name, Init: u.value(b.Init, inner), Place: b.Buffer}
			if b.Buffer {
				inner = inner.with(b.Name, b.Name)
			} else {
				inner = inner.without(b.Name)
			}
		}
		out = &core.Local{Binds: binds, Body: u.valueOrVersion(e.Body, inner)}
	case *core.BuildMap:
		// A MAP BUFFER is a place too (ADR 0058).
		inner := sc
		for _, n := range e.Names {
			inner = inner.with(n, n)
		}
		out = &core.BuildMap{Names: e.Names, Caps: u.values(e.Caps, sc), Body: u.valueOrVersion(e.Body, inner), Place: true}
	case *core.Loop:
		items, loop, err := u.loop(e, sc, false)
		if err != nil {
			return nil, err
		}
		out = u.fold(items, loop)
	default:
		return e, nil
	}
	u.from(out, e)
	return out, nil
}

func patNames(e core.Expr) []string {
	var out []string
	switch e := e.(type) {
	case *core.Var:
		out = append(out, e.Name)
	case *core.Apply:
		for _, a := range e.Args {
			out = append(out, patNames(a)...)
		}
	}
	return out
}

// ---- loops ----

// loop forgets a loop's carried buffers: their initializers become writes before
// it, each back edge's carrier arguments writes before the jump. stmt says the
// loop's value is a version that a place takes, so each exit writes, then is ().
func (u *unthreader) loop(e *core.Loop, sc places, stmt bool) ([]uitem, core.Expr, error) {
	carriers := u.carriers(e, sc)
	inner := u.loopScope(e, sc, carriers)
	var items []uitem
	var vars []string
	var inits []core.Expr
	// THE INITIALIZERS, in order: a carrier's writes before the loop; an
	// ordinary one bound first if a later carrier writes a place it reads.
	for j, init := range e.Inits {
		if q, ok := carriers[j]; ok {
			its, err := u.stmts(init, q, sc)
			if err != nil {
				return nil, nil, err
			}
			items = append(items, its...)
			continue
		}
		v := u.value(init, sc)
		if u.readsLaterWrite(init, sc, carriers, j, e.Inits) {
			t := u.temp()
			items = append(items, uitem{bind: &core.LetBind{Pat: &core.PVar{Name: t}, Value: v}, origin: init})
			v = &core.Var{Name: t}
		}
		vars, inits = append(vars, e.Vars[j]), append(inits, v)
	}
	arms := make([]core.LoopArm, len(e.Arms))
	for i, a := range e.Arms {
		body, err := u.armTail(a.Body, inner, e, carriers, stmt)
		if err != nil {
			return nil, nil, err
		}
		arms[i] = core.LoopArm{Guard: u.value(a.Guard, inner), Pats: a.Pats, When: u.value(a.When, inner), Else: a.Else, Body: body}
		u.from(arms[i].Body, a.Body)
	}
	if len(vars) == 0 && len(carriers) > 0 {
		// A LOOP WITH NOTHING BUT ITS CARRIERS keeps a variable that counts
		// nothing? It cannot: every loop has a variable it tests or it jumps
		// with no arguments, which J writes as a loop with no binders.
	}
	loop := &core.Loop{Vars: vars, Inits: inits, Arms: arms, Match: e.Match}
	u.from(loop, e)
	return items, loop, nil
}

func (u *unthreader) temp() string {
	u.fresh++
	return fmt.Sprintf("t%d_", u.fresh)
}

// readsLaterWrite reports whether a loop's argument j reads a place that a
// carrier argument after it writes: threading evaluated it first, so it is
// bound before the writes.
func (u *unthreader) readsLaterWrite(arg core.Expr, sc places, carriers map[int]string, j int, args []core.Expr) bool {
	for k := j + 1; k < len(args); k++ {
		q, ok := carriers[k]
		if !ok {
			continue
		}
		if _, plain := args[k].(*core.Var); plain {
			continue
		}
		if mentionsPlace(arg, q, sc) {
			return true
		}
	}
	return false
}

func mentionsPlace(e core.Tail, q string, sc places) bool {
	hit := false
	visit(e, func(x core.Expr) {
		if v, ok := x.(*core.Var); ok && (v.Name == q || sc[v.Name] == q) {
			hit = true
		}
	})
	return hit
}

// armTail is a loop arm's body: a back edge's carrier arguments as writes, then
// the jump with the others; an exit a value, or, in a statement, its writes.
func (u *unthreader) armTail(t core.Tail, sc places, e *core.Loop, carriers map[int]string, stmt bool) (core.Tail, error) {
	switch t := t.(type) {
	case *core.Again:
		var items []uitem
		var moves []core.Place
		var sources []core.Expr
		var args []core.Expr
		for j, a := range t.Args {
			q, carrier := carriers[j]
			if !carrier {
				v := u.value(a, sc)
				if u.readsLaterWrite(a, sc, carriers, j, t.Args) {
					tmp := u.temp()
					items = append(items, uitem{bind: &core.LetBind{Pat: &core.PVar{Name: tmp}, Value: v}, origin: a})
					v = &core.Var{Name: tmp}
				}
				args = append(args, v)
				continue
			}
			src, ok := u.version(a, sc)
			if !ok {
				return nil, fmt.Errorf("U: the jump hands %s's carrier %s, which is no version of a place", q, core.Desugar(a))
			}
			its, err := u.stmts(a, src, sc)
			if err != nil {
				return nil, err
			}
			items = append(items, its...)
			if src != q {
				moves = append(moves, core.Place{Name: q})
				sources = append(sources, &core.Var{Name: src})
			}
		}
		if len(moves) > 0 {
			if !permutation(moves, sources) {
				return nil, fmt.Errorf("U: a jump moves buffers between places that are not a permutation (R0)")
			}
			w := &core.Write{Places: moves, Values: sources}
			u.from(w, t)
			items = append(items, uitem{expr: w, origin: t, unit: true})
		}
		jump := &core.Again{Args: args}
		u.from(jump, t)
		return u.foldTail(items, jump), nil
	case *core.TLet:
		var items []uitem
		inner := sc
		for _, b := range t.Binds {
			its, next, err := u.binding(b, inner)
			if err != nil {
				return nil, err
			}
			items, inner = append(items, its...), next
		}
		body, err := u.armTail(t.Body, inner, e, carriers, stmt)
		if err != nil {
			return nil, err
		}
		return u.foldTail(items, body), nil
	case *core.TSeq:
		var items []uitem
		for _, x := range t.Xs {
			items = append(items, uitem{expr: u.value(x, sc), origin: x})
		}
		body, err := u.armTail(t.Last, sc, e, carriers, stmt)
		if err != nil {
			return nil, err
		}
		return u.foldTail(items, body), nil
	case *core.TMonad:
		body, err := u.armTail(t.Body, sc.without(patNames(t.Items[0])...), e, carriers, stmt)
		if err != nil {
			return nil, err
		}
		out := &core.TMonad{Form: t.Form, Items: u.values(t.Items, sc), Body: body}
		u.from(out, t)
		return out, nil
	case core.Expr:
		if stmt {
			q, ok := u.version(t, sc)
			if !ok {
				return nil, fmt.Errorf("U: a loop whose value a place takes exits with %s, no version of it", describe(t))
			}
			its, err := u.stmts(t, q, sc)
			if err != nil {
				return nil, err
			}
			return u.fold(its, unit()), nil
		}
		return u.valueOrVersion(t, sc), nil
	}
	return t, nil
}

func describe(t core.Tail) string { return fmt.Sprintf("%T", t) }

// permutation reports moves whose destinations are their sources' places, each
// once: the map from tables to places stays injective (R0).
func permutation(dst []core.Place, src []core.Expr) bool {
	a, b := map[string]int{}, map[string]int{}
	for _, p := range dst {
		a[p.Name]++
	}
	for _, s := range src {
		b[s.(*core.Var).Name]++
	}
	if len(a) != len(b) {
		return false
	}
	for k, n := range a {
		if n != 1 || b[k] != 1 {
			return false
		}
	}
	return true
}

// ---- definitions ----

// borrowed is the parameter a definition borrows: the one its value is a
// version of, by the borrows known so far.
func (u *unthreader) borrowed(lam *core.Lam) (int, bool) {
	sc := places{}
	for _, p := range lam.Params {
		sc[p] = p
	}
	q, ok := u.version(lam.Body, sc)
	if !ok || !u.written(lam.Body, q) {
		return 0, false
	}
	for i, p := range lam.Params {
		if p == q {
			return i, true
		}
	}
	return 0, false
}

// written reports whether a body writes the buffer named q: stores into it, or
// lends it to a borrow, under its own name or a loop variable's that carries
// it. A value only handed through, an integer a loop counts with, is no buffer.
func (u *unthreader) written(body core.Expr, q string) bool {
	names := map[string]bool{q: true}
	for changed := true; changed; {
		changed = false
		visit(body, func(e core.Expr) {
			l, ok := e.(*core.Loop)
			if !ok {
				return
			}
			for j, init := range l.Inits {
				if v, ok := init.(*core.Var); ok && names[v.Name] && !names[l.Vars[j]] {
					names[l.Vars[j]], changed = true, true
				}
			}
		})
	}
	hit := false
	visit(body, func(e core.Expr) {
		a, ok := e.(*core.Apply)
		if !ok {
			return
		}
		if v, ok := a.Fn.(*core.Var); ok && v.Name == "set" && len(a.Args) == 3 {
			base := a.Args[0]
			if rec, ok := base.(*core.Apply); ok && len(rec.Args) == 1 {
				base = rec.Fn
			}
			if v, ok := base.(*core.Var); ok && names[v.Name] {
				hit = true
			}
		}
		if _, d, ok := u.callee(a); ok {
			for _, i := range d.Params {
				if i < len(a.Args) {
					if v, ok := a.Args[i].(*core.Var); ok && names[v.Name] {
						hit = true
					}
				}
			}
		}
	})
	return hit
}

// unthread rewrites a module's definitions and terms in program order.
func unthread(decls []core.Decl, borrows map[string]core.BorrowDecl) (origin map[core.Tail]core.Tail, err error) {
	u := &unthreader{borrows: borrows, origin: map[core.Tail]core.Tail{}}
	defs := map[string]*core.Lam{}
	for _, d := range decls {
		if dd, ok := d.(*core.DefDecl); ok {
			if lam, ok := dd.Value.(*core.Lam); ok {
				defs[dd.Name] = lam
			}
		}
	}
	// THE DEFINITIONS THAT BORROW, a least fixpoint: one that returns the buffer
	// it was given, possibly through a call of another.
	names := make([]string, 0, len(defs))
	for n := range defs {
		names = append(names, n)
	}
	sort.Strings(names)
	for changed := true; changed; {
		changed = false
		for _, n := range names {
			if _, ok := u.borrows[n]; ok {
				continue
			}
			if i, ok := u.borrowed(defs[n]); ok {
				u.borrows[n] = core.BorrowDecl{Params: []int{i}}
				changed = true
			}
		}
	}
	defer func() {
		if r := recover(); r != nil {
			ue, ok := r.(unthreadError)
			if !ok {
				panic(r)
			}
			err = ue.err
		}
	}()
	for _, d := range decls {
		switch d := d.(type) {
		case *core.DefDecl:
			lam, ok := d.Value.(*core.Lam)
			if !ok {
				if d.Value != nil {
					d.Value = u.value(d.Value, places{})
				}
				continue
			}
			if b, ok := u.borrows[d.Name]; ok && len(b.Params) == 1 && defs[d.Name] == lam {
				p := lam.Params[b.Params[0]]
				sc := places{p: p}
				its, err := u.stmts(lam.Body, p, sc)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", d.Name, err)
				}
				body := u.unitBlock(its)
				n := &core.Lam{Params: lam.Params, Body: body, Claim: lam.Claim, Borrows: []string{p}}
				u.from(n, lam)
				d.Value = n
				continue
			}
			n := &core.Lam{Params: lam.Params, Body: u.value(lam.Body, places{}), Claim: lam.Claim}
			u.from(n, lam)
			d.Value = n
		case *core.TermDecl:
			d.E = u.value(d.E, places{})
		}
	}
	return u.origin, nil
}

// ---- what callees borrow ----

var (
	hostMu    sync.Mutex
	hostCache = map[string]map[string]core.BorrowDecl{}
)

// hostBorrows reads every target file's declarations once: a host's
// declaration borrows when it takes a `(buffer V)` and its result hands it back
// first (emit/target.go, borrowForms, reads the same), by its module path and
// name, `go/encoding/hex.Encode`.
func hostBorrows(root string) map[string]core.BorrowDecl {
	hostMu.Lock()
	defer hostMu.Unlock()
	if m, ok := hostCache[root]; ok {
		return m
	}
	out := map[string]core.BorrowDecl{}
	filepath.Walk(filepath.Join(root, "targets"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".oro") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		forms, err := core.ReadRaw(string(b))
		if err != nil {
			return nil
		}
		var walk func(ts []*core.Term, mod string)
		walk = func(ts []*core.Term, mod string) {
			for _, f := range ts {
				switch head(f) {
				case "target":
					walk(f.Kids[2:], mod)
				case "module":
					p := f.Kids[1].Name
					if mod != "" && !strings.Contains(p, "/") {
						p = mod + "/" + p
					}
					walk(f.Kids[2:], p)
				case "sig":
					if d, ok := sigBorrows(f); ok && mod != "" {
						name := f.Kids[1]
						if name.Kind == core.KApp {
							name = name.Kids[0]
						}
						out[mod+"."+name.Name] = d
					}
				}
			}
		}
		walk(forms, "")
		return nil
	})
	hostCache[root] = out
	return out
}

func isBufferType(t *core.Term) bool { return head(t) == "buffer" }

func sigBorrows(f *core.Term) (core.BorrowDecl, bool) {
	if len(f.Kids) < 4 || f.Kids[2].Kind != core.KApp {
		return core.BorrowDecl{}, false
	}
	var d core.BorrowDecl
	for i, p := range f.Kids[2].Kids {
		if isBufferType(p) || (p.Kind == core.KApp && len(p.Kids) == 2 && p.Kids[0].Kind == core.KName && isBufferType(p.Kids[1])) {
			d.Params = append(d.Params, i)
		}
	}
	if len(d.Params) == 0 {
		return d, false
	}
	r := f.Kids[3]
	results := []*core.Term{r}
	if head(r) == "tuple" {
		results = r.Kids[1:]
	}
	if len(results) < len(d.Params) {
		return d, false
	}
	for j := range d.Params {
		if !isBufferType(results[j]) {
			return d, false
		}
	}
	d.Results = len(results) - len(d.Params)
	return d, true
}

// moduleBorrows is what an imported module's definitions borrow, read from its
// source by U's own fixpoint: an imported definition that returns the buffer it
// was given is a borrow at its call, as a host's is.
func moduleBorrows(file string, host map[string]core.BorrowDecl) map[string]core.BorrowDecl {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	ds, err := core.ReadTrees(string(b))
	if err != nil {
		return nil
	}
	u := &unthreader{borrows: map[string]core.BorrowDecl{}, origin: map[core.Tail]core.Tail{}}
	for k, v := range host {
		u.borrows[k] = v
	}
	defs := map[string]*core.Lam{}
	for _, d := range ds {
		if dd, ok := d.(*core.DefDecl); ok {
			if lam, ok := dd.Value.(*core.Lam); ok {
				defs[dd.Name] = lam
			}
		}
	}
	out := map[string]core.BorrowDecl{}
	for changed := true; changed; {
		changed = false
		for n, lam := range defs {
			if _, ok := out[n]; ok {
				continue
			}
			if i, ok := u.borrowed(lam); ok {
				out[n] = core.BorrowDecl{Params: []int{i}}
				u.borrows[n] = out[n]
				changed = true
			}
		}
	}
	return out
}
