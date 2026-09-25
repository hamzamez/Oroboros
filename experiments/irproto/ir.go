package main

// P1 of docs/ir-research.md: the two candidate IRs, lowered from the residual.
//
// C2 is STRUCTURED SSA: a region is a list of operations and ends in a
// terminator, and an operation may own regions (if, loop, build). C1 is a
// CONTROL-FLOW GRAPH of blocks with parameters. Both name every value once, so
// neither ever reopens a binder: the residual's de Bruijn indices are resolved
// against a stack of frames, one pushed per λ crossed, and that is the whole of
// what "names" costs (research §4).

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
)

type V int32

type OpKind uint8

const (
	OpConst   OpKind = iota // a literal
	OpGlobal                // a free name: a definition, or a primitive used as a value
	OpPrim                  // a primitive call, one or several results
	OpIndex                 // (t i): a table read
	OpMapRead               // ((m k) (fn (#t #p) …)): a map read, two results
	OpIf                    // a conditional in VALUE position: two regions that yield
	OpLoop                  // a loop: one region, whose exits break and whose back edges continue
	OpBuild                 // a scope: one region with the buffer as its parameter
	OpOpaque                // a shape the prototype does not know (counted; P1 wants zero)
)

type Op struct {
	Kind OpKind
	Name string
	Lit  *core.Term
	Args []V
	Res  []V
	Sub  []*Region
}

type TermKind uint8

const (
	TYield    TermKind = iota // leave this region with values
	TBreak                    // leave the innermost loop with values
	TContinue                 // back edge of the innermost loop (`again`)
	TBranch                   // a conditional in TAIL position: two regions, each with its own terminator
)

type Region struct {
	Params     []V
	Pis        []Pi // π-parameters: values this branch renames, each narrowed by a guard (research §4b)
	Ops        []Op
	T          TermKind
	Args       []V
	Cond       V
	Then, Else *Region
}

// Pi is a π-node: V is Of renamed on one side of a branch, where `Of Rel Other`
// holds. Each name then carries one fact, so an analysis never keeps a map per
// program point (research §4b, irp1-2026-09-25 §4).
type Pi struct {
	V, Of, Other V
	Rel          string
	Len          bool // the guard is on Of's LENGTH, `(len Of) Rel Other`
}

// guard is a fact a branch establishes about a bound variable.
type guard struct {
	bound *core.Term // the KBound whose frame entry the branch renames
	of    V
	rel   string
	other V
	len   bool // a guard on the bound table's length
}

// lenOf recognises `(len X)` with X a bound variable, whose length a guard can
// narrow by renaming X.
func (l *lowerer) lenOf(t *core.Term) (*core.Term, bool) {
	if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[0].Kind == core.KName && t.Kids[1].Kind == core.KBound {
		if p, ok := l.prim(t.Kids[0].Name); ok && p.Kind == "len" {
			return t.Kids[1], true
		}
	}
	return nil, false
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

type Func struct {
	Params []V
	Body   *Region
	NV     int
}

// ---------------------------------------------------------------- lowering

type lowerer struct {
	tg     *emit.Target
	nv     int
	frames [][]V
	opaque int
	ops    int
	nodes  int
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
	f := l.frames[len(l.frames)-1-t.Depth]
	return f[t.Index]
}
func (l *lowerer) emit(r *Region, op Op) { l.ops++; r.Ops = append(r.Ops, op) }

func (l *lowerer) prim(name string) (emit.Prim, bool) {
	p, ok := l.tg.Prims[name]
	return p, ok
}

func isTupleLam(t *core.Term) bool {
	if t.Kind != core.KFn || len(t.Params) != 1 || t.Params[0] != "#k" {
		return false
	}
	b := t.Closed()
	return b.Kind == core.KApp && len(b.Kids) >= 3 && b.Kids[0].Kind == core.KBound &&
		b.Kids[0].Depth == 0 && b.Kids[0].Index == 0
}

// Lower lowers a residual definition `(fn (x̄) body)`.
func Lower(tg *emit.Target, t *core.Term) (*Func, *lowerer) {
	l := &lowerer{tg: tg}
	f := &Func{}
	body := t
	if t.Kind == core.KFn {
		f.Params = l.freshN(len(t.Params))
		l.push(f.Params)
		body = t.Closed()
	}
	f.Body = &Region{}
	l.tail(body, f.Body, false)
	f.NV = l.nv
	return f, l
}

// value lowers a term in value position, appending to r, and returns its values
// (several for a tuple).
func (l *lowerer) value(t *core.Term, r *Region) []V {
	l.nodes++
	switch t.Kind {
	case core.KInt, core.KFloat, core.KStr, core.KBool:
		v := l.fresh()
		l.emit(r, Op{Kind: OpConst, Lit: t, Res: []V{v}})
		return []V{v}
	case core.KBound:
		return []V{l.lookup(t)}
	case core.KName:
		v := l.fresh()
		l.emit(r, Op{Kind: OpGlobal, Name: t.Name, Res: []V{v}})
		return []V{v}
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
		return l.opaqueOp(t, r)
	}
	// KApp
	op := t.Kids[0]
	args := t.Kids[1:]
	switch op.Kind {
	case core.KFn: // a β-redex left in place: bind and go on
		var vs []V
		for _, a := range args {
			vs = append(vs, l.value(a, r)[0])
		}
		l.push(vs)
		out := l.value(op.Closed(), r)
		l.pop()
		return out
	case core.KBound:
		if len(args) == 1 {
			tab := l.lookup(op)
			i := l.value(args[0], r)[0]
			v := l.fresh()
			l.emit(r, Op{Kind: OpIndex, Args: []V{tab, i}, Res: []V{v}})
			return []V{v}
		}
		return l.opaqueOp(t, r)
	case core.KApp:
		if vs, ok := l.eliminator(t, r, func(body *core.Term) []V { return l.value(body, r) }); ok {
			return vs
		}
		return l.opaqueOp(t, r)
	case core.KName:
	default:
		return l.opaqueOp(t, r)
	}
	p, known := l.prim(op.Name)
	if !known {
		if len(args) == 1 { // a global table read
			g := l.value(op, r)[0]
			i := l.value(args[0], r)[0]
			v := l.fresh()
			l.emit(r, Op{Kind: OpIndex, Args: []V{g, i}, Res: []V{v}})
			return []V{v}
		}
		return l.opaqueOp(t, r)
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
		n := yieldArity(th)
		if n < 0 {
			n = yieldArity(el)
		}
		if n < 0 {
			n = 1
		}
		res := l.freshN(n)
		l.emit(r, Op{Kind: OpIf, Args: []V{c}, Res: res, Sub: []*Region{th, el}})
		return res
	case p.Kind == "iterate" && len(args) >= 2 && args[0].Kind == core.KFn:
		var inits []V
		for _, z := range args[1:] {
			inits = append(inits, l.value(z, r)[0])
		}
		body := &Region{Params: l.freshN(len(args[0].Params))}
		l.push(body.Params)
		l.tail(args[0].Closed(), body, true)
		l.pop()
		n := breakArity(body)
		if n < 0 {
			n = 1
		}
		res := l.freshN(n)
		l.emit(r, Op{Kind: OpLoop, Args: inits, Res: res, Sub: []*Region{body}})
		return res
	case (p.Kind == "table-build" || p.Kind == "map-build") && len(args) == 2 && args[1].Kind == core.KFn:
		n := l.value(args[0], r)[0]
		body := &Region{Params: l.freshN(1)}
		l.push(body.Params)
		l.tail(args[1].Closed(), body, false)
		l.pop()
		k := yieldArity(body)
		if k < 0 {
			k = 1
		}
		res := l.freshN(k)
		l.emit(r, Op{Kind: OpBuild, Name: op.Name, Args: []V{n}, Res: res, Sub: []*Region{body}})
		return res
	}
	var vs []V
	for _, a := range args {
		vs = append(vs, l.value(a, r)[0])
	}
	n := 1
	if len(p.Results) >= 2 {
		n = len(p.Results)
	}
	res := l.freshN(n)
	l.emit(r, Op{Kind: OpPrim, Name: op.Name, Args: vs, Res: res})
	return res
}

// eliminator lowers the three shapes whose operator is an application: a host
// call's continuation, a map read's, and a tuple's (a join point). `then`
// continues with the continuation's body, in value or in tail position.
func (l *lowerer) eliminator(t *core.Term, r *Region, then func(*core.Term) []V) ([]V, bool) {
	op, args := t.Kids[0], t.Kids[1:]
	if len(args) != 1 || args[0].Kind != core.KFn {
		return nil, false
	}
	k := args[0]
	if op.Kind == core.KApp && op.Kids[0].Kind == core.KName {
		if p, ok := l.prim(op.Kids[0].Name); ok && len(p.Results) >= 2 && len(k.Params) == len(p.Results) {
			var vs []V
			for _, a := range op.Kids[1:] {
				vs = append(vs, l.value(a, r)[0])
			}
			res := l.freshN(len(p.Results))
			l.emit(r, Op{Kind: OpPrim, Name: op.Kids[0].Name, Args: vs, Res: res})
			l.push(res)
			out := then(k.Closed())
			l.pop()
			return out, true
		}
	}
	// A map read under its eliminator: the read is not a primitive.
	if op.Kind == core.KApp && len(op.Kids) == 2 && len(k.Params) == 2 && op.Kids[0].Kind != core.KName {
		m := l.value(op.Kids[0], r)[0]
		key := l.value(op.Kids[1], r)[0]
		res := l.freshN(2)
		l.emit(r, Op{Kind: OpMapRead, Args: []V{m, key}, Res: res})
		l.push(res)
		out := then(k.Closed())
		l.pop()
		return out, true
	}
	// A tuple made where β cannot reach it: the producer yields m values, and the
	// continuation's names ARE those values. No projection, no copy: this is
	// research §4's point in one line.
	if len(k.Params) >= 2 {
		vs := l.value(op, r)
		if len(vs) != len(k.Params) {
			return nil, false
		}
		l.push(vs)
		out := then(k.Closed())
		l.pop()
		return out, true
	}
	return nil, false
}

// tail lowers a term in TAIL position of r. In a loop body a leaf is a back
// edge (`again`) or an exit (a break); elsewhere it yields.
func (l *lowerer) tail(t *core.Term, r *Region, loop bool) {
	l.nodes++
	leaf := func(vs []V) {
		r.Args = vs
		if loop {
			r.T = TBreak
		} else {
			r.T = TYield
		}
	}
	if t.Kind == core.KFn && isTupleLam(t) {
		leaf(l.value(t, r))
		return
	}
	if t.Kind != core.KApp {
		leaf(l.value(t, r))
		return
	}
	op, args := t.Kids[0], t.Kids[1:]
	if op.Kind == core.KName {
		if op.Name == "again" && loop {
			var vs []V
			for _, a := range args {
				vs = append(vs, l.value(a, r)[0])
			}
			r.T, r.Args = TContinue, vs
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

func (l *lowerer) opaqueOp(t *core.Term, r *Region) []V {
	l.opaque++
	v := l.fresh()
	l.emit(r, Op{Kind: OpOpaque, Lit: t, Res: []V{v}})
	return []V{v}
}

// yieldArity is the number of values a region yields, read at its first yield.
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

// breakArity is the number of values a loop body exits with, at its first break.
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

// ---------------------------------------------------------------- C1: flatten

type TermC1 uint8

const (
	JJump TermC1 = iota
	JBr
	JRet
)

type Block struct {
	Params []V
	Ops    []Op // no op here owns a region
	T      TermC1
	Cond   V
	To     [2]int
	Args   [2][]V
}

type CFG struct{ Blocks []*Block }

type flat struct {
	g *CFG
	// where a Yield goes: a block and whether it is the function's return
	yieldTo  int
	ret      bool
	contTo   int // the innermost loop's head
	breakTo  int // the innermost loop's exit
	nextVals int
	nv       *int
}

func (f *flat) newBlock(params []V) int {
	f.g.Blocks = append(f.g.Blocks, &Block{Params: params})
	return len(f.g.Blocks) - 1
}

func (f *flat) freshN(n int) []V {
	out := make([]V, n)
	for i := range out {
		out[i] = V(*f.nv)
		*f.nv++
	}
	return out
}

// Flatten turns a C2 function into a C1 graph.
func Flatten(fn *Func) *CFG {
	nv := fn.NV
	f := &flat{g: &CFG{}, ret: true, nv: &nv}
	entry := f.newBlock(fn.Params)
	f.region(fn.Body, entry)
	return f.g
}

// region flattens r's operations into block b and its successors, and ends the
// last block with r's terminator.
func (f *flat) region(r *Region, b int) {
	for _, op := range r.Ops {
		switch op.Kind {
		case OpIf:
			join := f.newBlock(op.Res)
			th, el := f.newBlock(nil), f.newBlock(nil)
			blk := f.g.Blocks[b]
			blk.T, blk.Cond, blk.To = JBr, op.Args[0], [2]int{th, el}
			save := *f
			f.yieldTo, f.ret = join, false
			f.region(op.Sub[0], th)
			f.region(op.Sub[1], el)
			f.yieldTo, f.ret = save.yieldTo, save.ret
			b = join
		case OpLoop:
			head := f.newBlock(op.Sub[0].Params)
			exit := f.newBlock(op.Res)
			blk := f.g.Blocks[b]
			blk.T, blk.To, blk.Args = JJump, [2]int{head, 0}, [2][]V{op.Args, nil}
			save := *f
			f.contTo, f.breakTo = head, exit
			f.region(op.Sub[0], head)
			f.contTo, f.breakTo = save.contTo, save.breakTo
			b = exit
		case OpBuild:
			// the allocation, then the scope inline; its yield joins after it
			f.g.Blocks[b].Ops = append(f.g.Blocks[b].Ops, Op{Kind: OpPrim, Name: "alloc", Args: op.Args, Res: op.Sub[0].Params})
			join := f.newBlock(op.Res)
			save := *f
			f.yieldTo, f.ret = join, false
			f.region(op.Sub[0], b)
			f.yieldTo, f.ret = save.yieldTo, save.ret
			b = join
		default:
			f.g.Blocks[b].Ops = append(f.g.Blocks[b].Ops, op)
		}
	}
	blk := f.g.Blocks[b]
	switch r.T {
	case TYield:
		if f.ret {
			blk.T, blk.Args = JRet, [2][]V{r.Args, nil}
		} else {
			blk.T, blk.To, blk.Args = JJump, [2]int{f.yieldTo, 0}, [2][]V{r.Args, nil}
		}
	case TBreak:
		blk.T, blk.To, blk.Args = JJump, [2]int{f.breakTo, 0}, [2][]V{r.Args, nil}
	case TContinue:
		blk.T, blk.To, blk.Args = JJump, [2]int{f.contTo, 0}, [2][]V{r.Args, nil}
	case TBranch:
		th, el := f.newBlock(nil), f.newBlock(nil)
		blk.T, blk.Cond, blk.To = JBr, r.Cond, [2]int{th, el}
		f.region(r.Then, th)
		f.region(r.Else, el)
	}
}

// ---------------------------------------------------------------- what C1 must compute to print structured

func (g *CFG) succ(b int) []int {
	blk := g.Blocks[b]
	switch blk.T {
	case JJump:
		return []int{blk.To[0]}
	case JBr:
		return []int{blk.To[0], blk.To[1]}
	}
	return nil
}

// RPOAndDominators is the preparation Ramsey's structuring needs (research §3):
// a reverse postorder, and the immediate dominators by Cooper, Harvey and
// Kennedy's iterative algorithm ("A simple, fast dominance algorithm", 2001).
func (g *CFG) RPOAndDominators() ([]int, []int) {
	n := len(g.Blocks)
	seen := make([]bool, n)
	post := make([]int, 0, n)
	var dfs func(b int)
	dfs = func(b int) {
		seen[b] = true
		for _, s := range g.succ(b) {
			if !seen[s] {
				dfs(s)
			}
		}
		post = append(post, b)
	}
	dfs(0)
	rpo := make([]int, len(post))
	num := make([]int, n)
	for i := range num {
		num[i] = -1
	}
	for i, b := range post {
		rpo[len(post)-1-i] = b
	}
	for i, b := range rpo {
		num[b] = i
	}
	preds := make([][]int, n)
	for _, b := range rpo {
		for _, s := range g.succ(b) {
			preds[s] = append(preds[s], b)
		}
	}
	idom := make([]int, n)
	for i := range idom {
		idom[i] = -1
	}
	idom[0] = 0
	intersect := func(a, b int) int {
		for a != b {
			for num[a] > num[b] {
				a = idom[a]
			}
			for num[b] > num[a] {
				b = idom[b]
			}
		}
		return a
	}
	for changed := true; changed; {
		changed = false
		for _, b := range rpo[1:] {
			nd := -1
			for _, p := range preds[b] {
				if idom[p] < 0 {
					continue
				}
				if nd < 0 {
					nd = p
				} else {
					nd = intersect(p, nd)
				}
			}
			if nd >= 0 && idom[b] != nd {
				idom[b], changed = nd, true
			}
		}
	}
	return rpo, idom
}

func (fn *Func) String() string {
	var ops, regs int
	var walk func(r *Region)
	walk = func(r *Region) {
		regs++
		ops += len(r.Ops)
		for _, op := range r.Ops {
			for _, s := range op.Sub {
				walk(s)
			}
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(fn.Body)
	return fmt.Sprintf("%d values, %d ops, %d regions", fn.NV, ops, regs)
}

// ---------------------------------------------------------------- π-parameters

// cond lowers a branch's condition and returns the guards each side establishes.
// A comparison of two operands guards each operand that is a bound variable; a
// connective, `(if a b false)` for and and `(if a true b)` for or, guards what
// it implies on the side where it is decided, with operands that are bound
// variables or literals (whose values dominate the branch).
func (l *lowerer) cond(t *core.Term, r *Region) (V, []guard, []guard) {
	if t.Kind == core.KApp && t.Kids[0].Kind == core.KName && len(t.Kids) == 3 {
		if rel := emit.CmpOp(t.Kids[0].Name); rel != "" {
			a := l.value(t.Kids[1], r)[0]
			b := l.value(t.Kids[2], r)[0]
			c := l.fresh()
			l.emit(r, Op{Kind: OpPrim, Name: t.Kids[0].Name, Args: []V{a, b}, Res: []V{c}})
			var th, el []guard
			if t.Kids[1].Kind == core.KBound {
				th = append(th, guard{t.Kids[1], a, rel, b, false})
				el = append(el, guard{t.Kids[1], a, negate(rel), b, false})
			} else if x, ok := l.lenOf(t.Kids[1]); ok {
				th = append(th, guard{x, l.lookup(x), rel, b, true})
				el = append(el, guard{x, l.lookup(x), negate(rel), b, true})
			}
			if t.Kids[2].Kind == core.KBound {
				th = append(th, guard{t.Kids[2], b, flip(rel), a, false})
				el = append(el, guard{t.Kids[2], b, negate(flip(rel)), a, false})
			} else if x, ok := l.lenOf(t.Kids[2]); ok {
				th = append(th, guard{x, l.lookup(x), flip(rel), a, true})
				el = append(el, guard{x, l.lookup(x), negate(flip(rel)), a, true})
			}
			return c, th, el
		}
	}
	th, el := l.implied(t, r)
	c := l.value(t, r)[0]
	return c, th, el
}

// implied reads the guards a connective establishes, from the term: the side a
// conjunction is true on has both, the side a disjunction is false on has both
// negations. Only operands that dominate the branch are used.
func (l *lowerer) implied(t *core.Term, r *Region) (th, el []guard) {
	if t.Kind != core.KApp || t.Kids[0].Kind != core.KName {
		return nil, nil
	}
	if rel := emit.CmpOp(t.Kids[0].Name); rel != "" && len(t.Kids) == 3 {
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
	if p, ok := l.prim(t.Kids[0].Name); ok && p.Kind == "cond" && len(t.Kids) == 4 {
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

// guarded lowers one side of a branch with its guards as π-parameters: each
// guarded binder is renamed, for this side only, by overriding its frame entry.
func (l *lowerer) guarded(r *Region, gs []guard, body func()) {
	type saved struct {
		depth int
		frame []V
	}
	var undo []saved
	current := map[*core.Term]V{}
	for _, g := range gs {
		d := len(l.frames) - 1 - g.bound.Depth
		of := g.of
		if v, ok := current[g.bound]; ok {
			of = v // a second guard on the same variable narrows the first π
		} else if cur := l.frames[d][g.bound.Index]; cur != g.of {
			of = cur
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
