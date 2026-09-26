package ir

import (
	"fmt"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// LOWERING, L : residual → IR (spec §8).
//
// L carries an environment from the residual's de Bruijn binders to values: a
// stack of frames, one pushed per λ crossed, and a binder is its frame's slot.
// A λ's body is read through Closed(), which does not rebuild the term, so no
// binder is ever reopened and no term is ever copied (research §4).
//
// Every row of spec §8.1's table is a case below, and anything else is an
// error that names the term: the IR has no opaque operation.

// Options says what the pipeline has already decided when it hands L a
// residual.
type Options struct {
	// Decided: the residual has passed the legality check (ADR 0019): every
	// plain integer operation is proven inside its representation, and every
	// unproven one was rewritten to the target's checked form. Lowering then
	// writes each operation's mode, `exact` or `trap` (spec §4.4). Without it,
	// modes are left undecided, which is IR_A's normal state.
	Decided bool
}

// Lower lowers one exported definition's residual.
func Lower(tg *emit.Target, name string, sig *core.Sig, t *core.Term, opt Options) (f *Func, err error) {
	l := &lowerer{tg: tg, decl: map[V]string{}, opt: opt, sig: sig}
	defer func() {
		if x := recover(); x != nil {
			e, ok := x.(lowerError)
			if !ok {
				panic(x)
			}
			f, err = nil, fmt.Errorf("%s: lowering: %s", name, e.msg)
		}
	}()
	f = &Func{Name: name, Body: &Region{}}
	body := t
	if t.Kind == core.KFn {
		l.top = t.Params
		f.Params = l.freshN(len(t.Params))
		for i, p := range f.Params {
			if sig != nil && i < len(sig.Params) && sig.Params[i].Type != "" {
				l.decl[p] = sig.Params[i].Type
			}
		}
		l.push(f.Params)
		body = t.Closed()
		l.assumeWhere(sig, f)
	}
	l.tail(body, f.Body, false)
	f.Types = make([]string, l.nv)
	typeFunc(tg, f, l.decl)
	for promote(tg, f, opt) {
		typeFunc(tg, f, l.decl)
	}
	// A DECLARED RESULT is the signature's, not what the body's yields were
	// typed as: it is a fixed member of its value's class (Theorem D′), and the
	// host compiles the declaration.
	if sig != nil {
		// THE DECLARED ARITY IS THE BOUNDARY, as the types are: a caller is
		// compiled against the signature, so a body yielding a different
		// number of values is refused, not printed with its own. The term
		// backends refused it at emission; the IR's printers took the body's
		// arity and printed `func Three(a int) (int, int)` for a declared
		// (tuple int int int) (irstep4a-2026-09-26).
		declared := len(sig.Results)
		if declared == 0 && sig.Result != "" {
			declared = 1
		}
		if declared > 0 && declared != len(f.Results) {
			return nil, fmt.Errorf("%s: declares %d result(s) and does not produce them: the body yields %d",
				name, declared, len(f.Results))
		}
		switch {
		case len(sig.Results) > 0 && len(sig.Results) == len(f.Results):
			for j, r := range sig.Results {
				if declaresTable(tg, r) {
					f.Results[j] = r
				}
			}
		case len(f.Results) == 1 && declaresTable(tg, sig.Result):
			f.Results[0] = sig.Result
		}
	}
	Canonicalize(f)
	return f, nil
}

// promote turns a call of an overloaded host operator into the language's
// integer operation when typing has shown both operands are integers
// (JavaScript's `+` over its one number type). Promotion only adds equations
// (the operands and result are `int`), so re-typing after it is monotone and
// the loop in Lower reaches a fixpoint. It reports whether anything changed.
func promote(tg *emit.Target, f *Func, opt Options) bool {
	changed := false
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OCall || len(s.Res) != 1 {
				continue
			}
			p := tg.Prims[s.Name]
			o, m, ok := classify(tg, s.Name, p, len(s.Args), opt)
			if !ok {
				continue
			}
			for _, a := range s.Args {
				if tg.ValueType(f.Types[a]) != "int" {
					ok = false
				}
			}
			if ok {
				s.Op, s.Mode, s.Name = o, m, ""
				if o.IsCmp() {
					s.Mode = MNone
				}
				changed = true
			}
		}
	})
	return changed
}

type lowerError struct{ msg string }

type lowerer struct {
	tg     *emit.Target
	opt    Options
	nv     int
	frames [][]V
	decl   map[V]string // types a definition declares: a parameter's, a primitive's result, an ascription
	sig    *core.Sig
	top    []string // the function's parameter hints, which the interval analysis keys its signature by
}

func (l *lowerer) fail(format string, args ...any) {
	panic(lowerError{fmt.Sprintf(format, args...)})
}

func (l *lowerer) fresh() V { l.nv++; return V(l.nv - 1) }

func (l *lowerer) freshN(n int) []V {
	out := make([]V, n)
	for i := range out {
		out[i] = l.fresh()
	}
	return out
}

func (l *lowerer) push(vs []V) { l.frames = append(l.frames, vs) }
func (l *lowerer) pop()        { l.frames = l.frames[:len(l.frames)-1] }

func (l *lowerer) lookup(t *core.Term) V {
	if t.Depth >= len(l.frames) {
		l.fail("an unbound variable (depth %d)", t.Depth)
	}
	f := l.frames[len(l.frames)-1-t.Depth]
	if t.Index >= len(f) || f[t.Index] < 0 {
		l.fail("a variable with no value: %s", t)
	}
	return f[t.Index]
}

func (l *lowerer) emit(r *Region, s Stmt) { r.Stmts = append(r.Stmts, s) }

// one emits a statement with one fresh result and returns it.
func (l *lowerer) one(r *Region, s Stmt) V {
	v := l.fresh()
	s.Res = []V{v}
	l.emit(r, s)
	return v
}

func (l *lowerer) prim(name string) (emit.Prim, bool) {
	p, ok := l.tg.Prims[name]
	return p, ok
}

// isTupleLam recognises the Church encoding of a product, λk. k a₁ … aₙ with
// n ≥ 2: a tuple, which in the IR is a list of values (spec §1.1). It is
// recognised by STRUCTURE — the binder occurs as the head and nowhere else —
// and not by its name hint: the reader spells a tuple's binder `#k`, and a
// variant crossing a boundary is `(fn (#x) (#x tag payload))` (sums.md), the
// same product under another hint.
func isTupleLam(t *core.Term) bool {
	if t.Kind != core.KFn || len(t.Params) != 1 {
		return false
	}
	b := t.Closed()
	if b.Kind != core.KApp || len(b.Kids) < 3 || b.Kids[0].Kind != core.KBound ||
		b.Kids[0].Depth != 0 || b.Kids[0].Index != 0 {
		return false
	}
	for _, a := range b.Kids[1:] {
		if mentions(a, 0) {
			return false
		}
	}
	return true
}

// mentions reports whether t refers to the binder `depth` λs out.
func mentions(t *core.Term, depth int) bool {
	switch t.Kind {
	case core.KBound:
		return t.Depth == depth && t.Index == 0
	case core.KFn:
		return mentions(t.Closed(), depth+1)
	case core.KApp:
		for _, k := range t.Kids {
			if mentions(k, depth) {
				return true
			}
		}
	}
	return false
}

// value lowers a term in value position and returns its values: several for a
// tuple.
func (l *lowerer) value(t *core.Term, r *Region) []V {
	switch t.Kind {
	case core.KInt, core.KFloat, core.KStr, core.KBool:
		return []V{l.one(r, Stmt{Op: OConst, Lit: t})}
	case core.KBound:
		return []V{l.lookup(t)}
	case core.KName:
		// A primitive of no arguments is a call; a constant (a host's `nil`,
		// `math.Pi`) is one.
		if p, ok := l.prim(t.Name); ok && len(p.Args) == 0 && p.Kind != "cond" && p.Kind != "let" {
			return []V{l.call(r, t.Name, p, nil)}
		}
		l.fail("the free name %s is not a value the IR has: a global (spec §2.4) is not produced by staging, and a primitive used as a value is a closure", t.Name)
	case core.KFn:
		if isTupleLam(t) {
			l.push([]V{-1})
			comps := t.Closed().Kids[1:]
			out := make([]V, 0, len(comps))
			for _, c := range comps {
				out = append(out, l.value(c, r)[0])
			}
			l.pop()
			return out
		}
		l.fail("a λ in value position is a closure (callbacks.md): %s", t)
	}
	op, args := t.Kids[0], t.Kids[1:]
	switch op.Kind {
	case core.KFn: // a β-redex the reducer left: a binding
		var vs []V
		for _, a := range args {
			vs = append(vs, l.value(a, r)[0])
		}
		l.push(vs)
		out := l.value(op.Closed(), r)
		l.pop()
		return out
	case core.KBound: // indexing is application (tables.md §3)
		if len(args) == 1 {
			tab := l.lookup(op)
			i := l.value(args[0], r)[0]
			return []V{l.one(r, Stmt{Op: OIndex, Args: []V{tab, i}})}
		}
		l.fail("a bound variable applied to %d arguments: %s", len(args), t)
	case core.KApp:
		if vs, ok := l.eliminator(t, r, func(body *core.Term) []V { return l.value(body, r) }); ok {
			return vs
		}
		l.fail("an application whose operator is an application, and not an eliminator: %s", t)
	case core.KName:
	default:
		l.fail("an application of %s", op)
	}
	if op.Name == "again" {
		l.fail("`again` outside a loop's tail (ADR 0015)")
	}
	p, known := l.prim(op.Name)
	if !known {
		l.fail("%s is not a primitive of target %s, and a global (spec §2.4) is not produced by staging", op.Name, l.tg.Name)
	}
	switch {
	case p.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn:
		vs := l.value(args[0], r)
		l.push(vs[:1])
		out := l.value(args[1].Closed(), r)
		l.pop()
		return out
	case p.Kind == "cond" && len(args) == 3:
		c, gt, ge := l.cond(args[0], r)
		th, el := &Region{}, &Region{}
		l.guarded(th, gt, func() { l.tail(args[1], th, false) })
		l.guarded(el, ge, func() { l.tail(args[2], el, false) })
		res := l.freshN(arity(yieldArity(th), yieldArity(el)))
		l.emit(r, Stmt{Op: OIf, Args: []V{c}, Res: res, Sub: []*Region{th, el}})
		return res
	case p.Kind == "iterate" && len(args) >= 2 && args[0].Kind == core.KFn:
		var inits []V
		for _, z := range args[1:] {
			inits = append(inits, l.value(z, r)[0])
		}
		if len(inits) != len(args[0].Params) {
			l.fail("a loop with %d variables and %d initial values", len(args[0].Params), len(inits))
		}
		body := &Region{Params: l.freshN(len(args[0].Params))}
		l.push(body.Params)
		l.tail(args[0].Closed(), body, true)
		l.pop()
		res := l.freshN(arity(breakArity(body), -1))
		l.emit(r, Stmt{Op: OLoop, Args: inits, Res: res, Sub: []*Region{body}})
		return res
	case (p.Kind == "table-build" || p.Kind == "map-build") && len(args) == 2 && args[1].Kind == core.KFn:
		n := l.value(args[0], r)[0]
		body := &Region{Params: l.freshN(1)}
		// A BUILD'S ELEMENT RANGE is what the interval analysis proves of its
		// stores, joined with the zero fill, on the build's own λ: the fact
		// Theorem D′ joins over the build's class (Finalize). The analyses still
		// run on terms (ADR 0032's step 4 moves them); their conclusion is
		// written into the IR here, where the λ is in hand.
		if p.Kind == "table-build" && l.opt.Decided {
			if rng, ok := emit.BufferRange(l.tg, args[1], l.sig, l.top); ok {
				l.decl[body.Params[0]] = "buffer " + rng
			}
		}
		l.push(body.Params)
		l.tail(args[1].Closed(), body, false)
		l.pop()
		res := l.freshN(arity(yieldArity(body), -1))
		o := OBuild
		if p.Kind == "map-build" {
			o = OBuildMap
		}
		l.emit(r, Stmt{Op: o, Args: []V{n}, Res: res, Sub: []*Region{body}})
		return res
	case p.Kind == "table-alloc" && len(args) == 1:
		if rule, n, ok := l.tableRule(args[0]); ok {
			nv := l.value(n, r)[0]
			body := &Region{Params: l.freshN(1)}
			l.push(body.Params)
			l.tail(rule.Closed(), body, false)
			l.pop()
			return []V{l.one(r, Stmt{Op: OTabulate, Args: []V{nv}, Sub: []*Region{body}})}
		}
		// `alloc t` of a table that is not a rule is the table of t's contents
		// NOW: tabulate (len t) (i ↦ t i). For an immutable t that equals t
		// (η-tab, L12), and a printer may drop the copy where the IR's type
		// says t is immutable. For a live buffer the copy IS the meaning: a
		// later store must not show through (linearity.go: "alloc copies the
		// contents, so it is an ordinary read and must come first").
		t := l.value(args[0], r)[0]
		n := l.one(r, Stmt{Op: OLen, Args: []V{t}})
		body := &Region{Params: l.freshN(1)}
		e := l.fresh()
		body.Stmts = []Stmt{{Op: OIndex, Args: []V{t, body.Params[0]}, Res: []V{e}}}
		body.T, body.Args = TYield, []V{e}
		return []V{l.one(r, Stmt{Op: OTabulate, Args: []V{n}, Sub: []*Region{body}})}
	case p.Kind == "table":
		l.fail("a table rule that was never allocated (tables.md §2): %s", t)
	case p.Kind == "ascribe" && len(args) == 2 && args[0].Kind == core.KStr:
		a := l.value(args[1], r)[0]
		v := l.one(r, Stmt{Op: OThe, Type: args[0].Str, Args: []V{a}})
		l.decl[v] = args[0].Str
		return []V{v}
	case p.Kind == "array":
		return []V{l.one(r, Stmt{Op: OArray, Args: l.values(args, r)})}
	case p.Kind == "map":
		var kv []V
		for _, row := range args {
			if row.Kind != core.KApp || len(row.Kids) != 2 {
				l.fail("a map literal's row is (key value), got %s", row)
			}
			kv = append(kv, l.value(row.Kids[0], r)[0], l.value(row.Kids[1], r)[0])
		}
		return []V{l.one(r, Stmt{Op: OMap, Args: kv})}
	case p.Kind == "len" || l.tg.IsLengthName(op.Name) && len(args) == 1:
		return []V{l.one(r, Stmt{Op: OLen, Args: l.values(args, r)})}
	case p.Kind == "table-set" && len(args) == 3:
		return []V{l.one(r, Stmt{Op: OSet, Args: l.values(args, r)})}
	case p.Kind == "map-insert" && len(args) == 3:
		return []V{l.one(r, Stmt{Op: OInsert, Args: l.values(args, r)})}
	case p.Kind == "map-keys" && len(args) == 1:
		return []V{l.one(r, Stmt{Op: OKeys, Args: l.values(args, r)})}
	case p.Index && len(args) == 2 && l.isLanguageTable(p):
		return []V{l.one(r, Stmt{Op: OIndex, Args: l.values(args, r)})}
	}
	if o, mode, ok := l.integerOp(op.Name, p, len(args)); ok {
		return []V{l.one(r, Stmt{Op: o, Mode: mode, Args: l.values(args, r)})}
	}
	return []V{l.call(r, op.Name, p, l.values(args, r))}
}

func (l *lowerer) values(ts []*core.Term, r *Region) []V {
	out := make([]V, len(ts))
	for i, a := range ts {
		out[i] = l.value(a, r)[0]
	}
	return out
}

// call emits a primitive call with its declared number of results.
func (l *lowerer) call(r *Region, name string, p emit.Prim, args []V) V {
	n := 1
	if len(p.Results) >= 2 {
		n = len(p.Results)
	}
	res := l.freshN(n)
	for i, v := range res {
		switch {
		case n >= 2 && declares(p.Results[i]):
			l.decl[v] = p.Results[i]
		case n == 1 && p.Kind != "stmt" && declares(p.Result):
			l.decl[v] = p.Result
		}
	}
	l.emit(r, Stmt{Op: OCall, Name: name, Args: args, Res: res})
	return res[0]
}

// declares reports whether a declared type says more than the solver would:
// a range, or a host's own name. `int` and `any` say nothing unification does
// not.
func declares(ty string) bool {
	return ty != "" && ty != "any" && ty != "int"
}

// integerOp classifies a primitive once, here, as the language's integer
// operation it is (spec §1.2, §8.1): by the target's own classification
// (emit.ArithOp, emit.CmpOp), and only where the primitive is an integer one —
// `go.f+` is a float call, not `add`.
func (l *lowerer) integerOp(name string, p emit.Prim, n int) (Op, Mode, bool) {
	o, m, ok := classify(l.tg, name, p, n, l.opt)
	if !ok {
		return 0, 0, false
	}
	for _, a := range p.Args {
		if l.tg.ValueType(a) != "int" {
			return 0, 0, false // an overloaded host operator: decided after typing (promote)
		}
	}
	if len(p.Args) != n {
		return 0, 0, false
	}
	return o, m, true
}

// classify is the operation a primitive would be if its operands are
// integers: the target's own classification (emit.ArithOp, emit.CmpOp), where
// nothing in its declaration says otherwise — `go.f+` declares f64 and is a
// float call, and JavaScript's `+` declares `any` and is decided by its
// operands (promote).
func classify(tg *emit.Target, name string, p emit.Prim, n int, opt Options) (Op, Mode, bool) {
	open := func(ty string) bool { return ty == "" || ty == "any" || tg.ValueType(ty) == "int" }
	for _, a := range p.Args {
		if !open(a) {
			return 0, 0, false
		}
	}
	if open(p.Result) {
		ops := map[string]Op{"add": OAdd, "sub": OSub, "mul": OMul, "neg": ONeg, "div": ODiv, "rem": ORem}
		if o, ok := ops[emit.ArithOp(name, n)]; ok {
			if (o == ODiv || o == ORem) && !IntegerDivision(tg, name, p) {
				return 0, 0, false // a host's division in the reals is not ℤ's
			}
			mode := MNone
			if opt.Decided {
				mode = MExact
				if emit.IsCheckedName(name) {
					mode = MTrap
				}
			}
			return o, mode, true
		}
	}
	if n == 2 && (p.Result == "bool" || p.Result == "" || p.Result == "any") {
		ops := map[string]Op{"eq": OEq, "ne": ONe, "lt": OLt, "le": OLe, "gt": OGt, "ge": OGe}
		if o, ok := ops[emit.CmpOp(name)]; ok {
			return o, MNone, true
		}
	}
	return 0, 0, false
}

// isLanguageTable reports whether an index primitive reads a language table,
// whose element type is the table's: then it is `index` (spec §8.1).
func (l *lowerer) isLanguageTable(p emit.Prim) bool {
	return len(p.Args) == 2 && (core.ArrayElem(p.Args[0]) != "" || l.aliasElem(p.Args[0]))
}

func (l *lowerer) aliasElem(ty string) bool {
	_, ok := newUnifier(l.tg).aliasOf(ty)
	return ok
}

// tableRule recognises `(table n (fn (i) e))`.
func (l *lowerer) tableRule(t *core.Term) (*core.Term, *core.Term, bool) {
	if t.Kind != core.KApp || len(t.Kids) != 3 || t.Kids[0].Kind != core.KName {
		return nil, nil, false
	}
	if p, ok := l.prim(t.Kids[0].Name); !ok || p.Kind != "table" {
		return nil, nil, false
	}
	rule := t.Kids[2]
	if rule.Kind != core.KFn || len(rule.Params) != 1 {
		return nil, nil, false
	}
	return rule, t.Kids[1], true
}

// eliminator lowers the three shapes whose operator is an application: a host
// call's continuation (ADR 0027), a map read's, and a tuple's, which is a join
// point (ADR 0031). `then` continues with the continuation's body.
func (l *lowerer) eliminator(t *core.Term, r *Region, then func(*core.Term) []V) ([]V, bool) {
	op, args := t.Kids[0], t.Kids[1:]
	if len(args) != 1 || args[0].Kind != core.KFn {
		return nil, false
	}
	k := args[0]
	if op.Kind == core.KApp && op.Kids[0].Kind == core.KName {
		if p, ok := l.prim(op.Kids[0].Name); ok && len(p.Results) >= 2 && len(k.Params) == len(p.Results) {
			vs := l.values(op.Kids[1:], r)
			l.call(r, op.Kids[0].Name, p, vs)
			res := r.Stmts[len(r.Stmts)-1].Res
			l.push(res)
			out := then(k.Closed())
			l.pop()
			return out, true
		}
	}
	// A map read under its eliminator: `((m k) (fn (#t #p) b))`.
	if op.Kind == core.KApp && len(op.Kids) == 2 && len(k.Params) == 2 && op.Kids[0].Kind != core.KName {
		m := l.value(op.Kids[0], r)[0]
		key := l.value(op.Kids[1], r)[0]
		res := l.freshN(2)
		l.emit(r, Stmt{Op: ORead, Args: []V{m, key}, Res: res})
		l.push(res)
		out := then(k.Closed())
		l.pop()
		return out, true
	}
	// A tuple where β cannot reach it: the producer's values ARE the
	// continuation's names. No projection and no copy (L5).
	if len(k.Params) >= 2 {
		vs := l.value(op, r)
		if len(vs) != len(k.Params) {
			l.fail("a producer of %d values bound to %d names", len(vs), len(k.Params))
		}
		l.push(vs)
		out := then(k.Closed())
		l.pop()
		return out, true
	}
	return nil, false
}

// tail lowers a term in TAIL position of r. In a loop body a leaf is a `break`
// and `again` is a `continue`; elsewhere a leaf yields.
func (l *lowerer) tail(t *core.Term, r *Region, loop bool) {
	leaf := func(vs []V) {
		r.Args = vs
		if loop {
			r.T = TBreak
		} else {
			r.T = TYield
		}
	}
	if t.Kind != core.KApp {
		leaf(l.value(t, r))
		return
	}
	op, args := t.Kids[0], t.Kids[1:]
	if op.Kind == core.KName {
		if op.Name == "again" {
			if !loop {
				l.fail("`again` outside a loop's tail (ADR 0015)")
			}
			r.T, r.Args = TContinue, l.values(args, r)
			return
		}
		if p, ok := l.prim(op.Name); ok {
			switch {
			case p.Kind == "cond" && len(args) == 3:
				c, gt, ge := l.cond(args[0], r)
				th, el := &Region{}, &Region{}
				l.guarded(th, gt, func() { l.tail(args[1], th, loop) })
				l.guarded(el, ge, func() { l.tail(args[2], el, loop) })
				r.T, r.Cond, r.Then, r.Else = TBranch, c, th, el
				return
			case p.Kind == "let" && len(args) == 2 && args[1].Kind == core.KFn:
				vs := l.value(args[0], r)
				l.push(vs[:1])
				l.tail(args[1].Closed(), r, loop)
				l.pop()
				return
			}
		}
	}
	if op.Kind == core.KFn {
		var vs []V
		for _, a := range args {
			vs = append(vs, l.value(a, r)[0])
		}
		l.push(vs)
		l.tail(op.Closed(), r, loop)
		l.pop()
		return
	}
	if op.Kind == core.KApp {
		if _, ok := l.eliminator(t, r, func(body *core.Term) []V { l.tail(body, r, loop); return nil }); ok {
			return
		}
	}
	leaf(l.value(t, r))
}

// arity is a region's result count read at its first leaf, or at the second
// when the first has none; a region that never leaves (every leaf jumps) has 1
// by convention, which W4 then checks against nothing.
func arity(a, b int) int {
	if a >= 0 {
		return a
	}
	if b >= 0 {
		return b
	}
	return 1
}

func yieldArity(r *Region) int {
	switch r.T {
	case TYield:
		return len(r.Args)
	case TBranch:
		if n := yieldArity(r.Then); n >= 0 {
			return n
		}
		return yieldArity(r.Else)
	}
	return -1
}

func breakArity(r *Region) int {
	switch r.T {
	case TBreak:
		return len(r.Args)
	case TBranch:
		if n := breakArity(r.Then); n >= 0 {
			return n
		}
		return breakArity(r.Else)
	}
	return -1
}

// assumeWhere lowers an export's `where` to `assume`s at the function's entry.
// At an export the precondition is ASSUMED, since the callers are outside the
// program (ADR 0028), and an assumption is what refinements.md §3a's second
// route discharges an obligation with. The `where` names parameters; they are
// read here as the entry frame's values. A `where` lowering does not know is
// skipped: an assumption only ever adds facts, so dropping one is sound.
func (l *lowerer) assumeWhere(sig *core.Sig, f *Func) {
	if sig == nil || sig.Where == nil {
		return
	}
	names := make([]string, len(sig.Params))
	for i, p := range sig.Params {
		names[i] = p.Name
		// The `where` is the SOURCE's term, written before the representation
		// passes (PromoteBig, SelectWords) moved a parameter above the word;
		// its operations would not mean what they mean now. Not assumed.
		if vt := l.tg.ValueType(p.Type); vt == core.BigType || vt == core.U64Type {
			return
		}
	}
	term := bindNames(sig.Where, names, 0)
	r := f.Body
	mark, nv := len(r.Stmts), l.nv
	defer func() {
		if x := recover(); x != nil {
			if _, ok := x.(lowerError); !ok {
				panic(x)
			}
			r.Stmts, l.nv = r.Stmts[:mark], nv // not lowered: no assumption
		}
	}()
	c := l.value(term, r)[0]
	l.emit(r, Stmt{Op: OAssume, Args: []V{c}})
}

// bindNames reads a signature's term, which names parameters, against the
// entry frame: each name becomes that frame's slot.
func bindNames(t *core.Term, names []string, depth int) *core.Term {
	switch t.Kind {
	case core.KName:
		for i, n := range names {
			if n == t.Name {
				return &core.Term{Kind: core.KBound, Depth: depth, Index: i}
			}
		}
		return t
	case core.KFn:
		out := *t
		out.Kids = []*core.Term{bindNames(t.Closed(), names, depth+1)}
		return &out
	case core.KApp:
		out := *t
		out.Kids = make([]*core.Term, len(t.Kids))
		for i, k := range t.Kids {
			out.Kids[i] = bindNames(k, names, depth)
		}
		return &out
	}
	return t
}

// ---------------------------------------------------------------- π-parameters (spec §6)

type guard struct {
	bound *core.Term // the binder whose frame entry the arm renames
	of    V
	rel   string
	other V
	len   bool
}

func flip(rel string) string {
	switch rel {
	case "lt":
		return "gt"
	case "le":
		return "ge"
	case "gt":
		return "lt"
	case "ge":
		return "le"
	}
	return rel
}

func negate(rel string) string {
	switch rel {
	case "lt":
		return "ge"
	case "le":
		return "gt"
	case "gt":
		return "le"
	case "ge":
		return "lt"
	case "eq":
		return "ne"
	case "ne":
		return "eq"
	}
	return rel
}

var cmpRel = map[Op]string{OEq: "eq", ONe: "ne", OLt: "lt", OLe: "le", OGt: "gt", OGe: "ge"}

// lenOf recognises `(len X)` with X a bound variable.
func (l *lowerer) lenOf(t *core.Term) (*core.Term, bool) {
	if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[0].Kind == core.KName && t.Kids[1].Kind == core.KBound {
		if p, ok := l.prim(t.Kids[0].Name); ok && (p.Kind == "len" || l.tg.IsLengthName(t.Kids[0].Name)) {
			return t.Kids[1], true
		}
	}
	return nil, false
}

// cond lowers a branch's condition and returns the guards each arm has: a
// comparison guards each operand that is a bound value or the length of one; a
// connective guards what it implies on the arm where it is decided.
func (l *lowerer) cond(t *core.Term, r *Region) (V, []guard, []guard) {
	if t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName {
		if p, ok := l.prim(t.Kids[0].Name); ok {
			if o, _, ok := classify(l.tg, t.Kids[0].Name, p, 2, l.opt); ok && o.IsCmp() {
				rel := cmpRel[o]
				a := l.value(t.Kids[1], r)[0]
				b := l.value(t.Kids[2], r)[0]
				// The comparison is the language's where its operands are
				// declared integers, and a call otherwise (promote decides it
				// once they are typed). Its guards are π-parameters either
				// way: a π is the identity (L9), so one on a float is idle,
				// never wrong.
				var c V
				if _, _, definite := l.integerOp(t.Kids[0].Name, p, 2); definite {
					c = l.one(r, Stmt{Op: o, Args: []V{a, b}})
				} else {
					c = l.call(r, t.Kids[0].Name, p, []V{a, b})
				}
				var th, el []guard
				add := func(x *core.Term, of V, rel string, other V, isLen bool) {
					th = append(th, guard{x, of, rel, other, isLen})
					el = append(el, guard{x, of, negate(rel), other, isLen})
				}
				if t.Kids[1].Kind == core.KBound {
					add(t.Kids[1], a, rel, b, false)
				} else if x, ok := l.lenOf(t.Kids[1]); ok {
					add(x, l.lookup(x), rel, b, true)
				}
				if t.Kids[2].Kind == core.KBound {
					add(t.Kids[2], b, flip(rel), a, false)
				} else if x, ok := l.lenOf(t.Kids[2]); ok {
					add(x, l.lookup(x), flip(rel), a, true)
				}
				return c, th, el
			}
		}
	}
	th, el := l.implied(t, r)
	c := l.value(t, r)[0]
	return c, th, el
}

// implied reads the guards a connective establishes: a conjunction's true arm
// has both conjuncts' guards, a disjunction's false arm both negations. Only
// operands that dominate the branch are used: bound values and literals.
func (l *lowerer) implied(t *core.Term, r *Region) (th, el []guard) {
	if t.Kind != core.KApp || t.Kids[0].Kind != core.KName {
		return nil, nil
	}
	p, ok := l.prim(t.Kids[0].Name)
	if !ok {
		return nil, nil
	}
	if o, _, ok := classify(l.tg, t.Kids[0].Name, p, len(t.Kids)-1, l.opt); ok && o.IsCmp() && len(t.Kids) == 3 {
		rel := cmpRel[o]
		dom := func(x *core.Term) (V, bool) {
			switch x.Kind {
			case core.KBound:
				return l.lookup(x), true
			case core.KInt:
				return l.value(x, r)[0], true
			}
			return 0, false
		}
		a, okA := dom(t.Kids[1])
		b, okB := dom(t.Kids[2])
		if !okA || !okB {
			return nil, nil
		}
		if t.Kids[1].Kind == core.KBound {
			th = append(th, guard{t.Kids[1], a, rel, b, false})
			el = append(el, guard{t.Kids[1], a, negate(rel), b, false})
		}
		if t.Kids[2].Kind == core.KBound {
			th = append(th, guard{t.Kids[2], b, flip(rel), a, false})
			el = append(el, guard{t.Kids[2], b, negate(flip(rel)), a, false})
		}
		return th, el
	}
	if p.Kind == "cond" && len(t.Kids) == 4 {
		x, y, z := t.Kids[1], t.Kids[2], t.Kids[3]
		if z.Kind == core.KBool && !z.IsTrue() { // and
			tx, _ := l.implied(x, r)
			ty, _ := l.implied(y, r)
			return append(tx, ty...), nil
		}
		if y.Kind == core.KBool && y.IsTrue() { // or
			_, ex := l.implied(x, r)
			_, ez := l.implied(z, r)
			return nil, append(ex, ez...)
		}
	}
	return nil, nil
}

// guarded lowers one arm with its guards as π-parameters: each guarded binder
// is renamed, for this arm only, by overriding its frame entry.
func (l *lowerer) guarded(r *Region, gs []guard, body func()) {
	type saved struct {
		depth int
		frame []V
	}
	var undo []saved
	current := map[*core.Term]V{}
	for _, g := range gs {
		d := len(l.frames) - 1 - g.bound.Depth
		of := l.frames[d][g.bound.Index]
		if v, ok := current[g.bound]; ok {
			of = v // a second guard on one variable narrows the first π
		}
		pi := l.fresh()
		r.Pis = append(r.Pis, Pi{V: pi, Of: of, Other: g.other, Rel: g.rel, Len: g.len})
		undo = append(undo, saved{d, l.frames[d]})
		f := append([]V(nil), l.frames[d]...)
		f[g.bound.Index] = pi
		l.frames[d] = f
		current[g.bound] = pi
	}
	body()
	for i := len(undo) - 1; i >= 0; i-- {
		l.frames[undo[i].depth] = undo[i].frame
	}
}

// IntegerDivision reports whether a primitive the target classifies as `div` or
// `rem` is ℤ's truncating division (integers.md §3). The same spelling means
// different things on different hosts: Go's `/` on integers truncates, and
// JavaScript's `/` is division in the reals, embedded in floats (7/2 is 3.5).
// So it is ℤ's operation only where the declaration fixes integer operands, or
// the target names it integer division (`idiv`, `irem`).
func IntegerDivision(tg *emit.Target, name string, p emit.Prim) bool {
	seg := name
	if i := strings.LastIndex(seg, "."); i >= 0 {
		seg = seg[i+1:]
	}
	if seg == "idiv" || seg == "irem" {
		return true
	}
	if len(p.Args) == 0 {
		return false
	}
	for _, a := range p.Args {
		if tg.ValueType(a) != "int" {
			return false
		}
	}
	return true
}
