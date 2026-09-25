package main

// P2 of docs/ir-research.md: a Go printer for C2.
//
// The printer reads TYPES and never recomputes a fact (research §6). Types are
// inferred once, by UNIFICATION: a union-find over values, where a loop's
// parameter, its initial value and every `again` argument are one class, a
// build's buffer and every store into it are one class, and so on. An integer
// table's element width is then chosen ONCE PER CLASS from the sparse analysis's
// element ranges. That is LoopOneJoin's rule derived as unification: every
// table that flows into one variable must have one host type.
//
// Measured emission decisions kept (research §7):
//   - soleExit: a loop result every exit yields as the same parameter IS that
//     parameter, so no temporary defeats escape analysis;
//   - PostVars: a parameter every back edge updates by the same step moves to
//     the `for` post clause;
//   - parallel assignment on `again`, skipping unchanged parameters.
// Not kept yet, to be measured first: bounds-check re-slicing (`emitNarrow`) and
// back-edge buffer reuse.

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// ---------------------------------------------------------------- types by unification

type typer struct {
	parent []V
	ty     []string
}

func newTyper(n int) *typer {
	t := &typer{parent: make([]V, n), ty: make([]string, n)}
	for i := range t.parent {
		t.parent[i] = V(i)
	}
	return t
}

func (t *typer) find(v V) V {
	for t.parent[v] != v {
		t.parent[v] = t.parent[t.parent[v]]
		v = t.parent[v]
	}
	return v
}

// union merges two classes; a known type wins over an unknown one.
func (t *typer) union(a, b V) bool {
	ra, rb := t.find(a), t.find(b)
	if ra == rb {
		return false
	}
	ta, tb := t.ty[ra], t.ty[rb]
	t.parent[rb] = ra
	if ta == "" || ta == "any" {
		t.ty[ra] = tb
	}
	return true
}

// set gives a class a type it does not have yet.
func (t *typer) set(v V, ty string) bool {
	if ty == "" || ty == "any" || ty == "none" {
		return false
	}
	r := t.find(v)
	if t.ty[r] == "" || t.ty[r] == "any" {
		t.ty[r] = ty
		return true
	}
	return false
}

func (t *typer) get(v V) string { return t.ty[t.find(v)] }

// scalar normalises a language type for a scalar value: every integer range is
// the word, `int`, because the printer computes in the word and converts only at
// a table's narrowed storage.
func scalar(ty string) string {
	if _, _, ok := core.IntRange(ty); ok {
		return "int"
	}
	return ty
}

// elemOf is the element type of a table type, "" if it is not one.
func elemOf(tg *emit.Target, ty string) string {
	if strings.HasPrefix(ty, "array ") {
		return strings.TrimPrefix(ty, "array ")
	}
	if e := core.ArrayElem(ty); e != "" {
		return e
	}
	return ""
}

type goPrinter struct {
	tg    *emit.Target
	fn    *Func
	sp    *Sparse
	t     *typer
	alias []V // a π, or a value that IS another (stmt, set, a coalesced result)
	uses  []int
	b     strings.Builder
	ind   int
	imps  map[string]bool
	sig   *core.Sig
	err   error

	constOf map[V]*core.Term // a literal, inlined at every read
	yieldTo []string         // where a non-top yield assigns, nil when it returns
	rename  map[V]string     // a table re-sliced for bounds-check elimination
}

func (g *goPrinter) res(v V) V {
	for g.alias[v] >= 0 && g.alias[v] != v {
		v = g.alias[v]
	}
	return v
}

// infer runs the unification to a fixpoint.
func (g *goPrinter) infer() {
	t := g.t
	for i, p := range g.fn.Params {
		if g.sig != nil && i < len(g.sig.Params) {
			t.set(p, scalar(g.sig.Params[i].Type))
		}
	}
	for changed := true; changed; {
		changed = false
		var region func(r *Region, loop *Op)
		region = func(r *Region, loop *Op) {
			for _, pi := range r.Pis {
				if t.union(pi.Of, pi.V) {
					changed = true
				}
			}
			for i := range r.Ops {
				op := &r.Ops[i]
				if g.op(op, &changed) {
					changed = true
				}
				for k, s := range op.Sub {
					inner := loop
					if op.Kind == OpLoop {
						inner = op
					}
					region(s, inner)
					if y := yields(s); y != nil && op.Kind != OpLoop {
						for j, v := range y {
							if j < len(op.Res) && t.union(op.Res[j], v) {
								changed = true
							}
						}
					}
					_ = k
				}
				if op.Kind == OpLoop {
					for j, p := range op.Sub[0].Params {
						if j < len(op.Args) && t.union(p, op.Args[j]) {
							changed = true
						}
					}
					for _, c := range terms(op.Sub[0], TContinue) {
						for j, v := range c {
							if j < len(op.Sub[0].Params) && t.union(op.Sub[0].Params[j], v) {
								changed = true
							}
						}
					}
					for _, b := range terms(op.Sub[0], TBreak) {
						for j, v := range b {
							if j < len(op.Res) && t.union(op.Res[j], v) {
								changed = true
							}
						}
					}
				}
			}
			if r.T == TBranch {
				region(r.Then, loop)
				region(r.Else, loop)
			}
		}
		region(g.fn.Body, nil)
	}
}

// op types one operation; it reports a change.
func (g *goPrinter) op(op *Op, changed *bool) bool {
	t := g.t
	c := false
	mark := func(b bool) {
		if b {
			c = true
		}
	}
	switch op.Kind {
	case OpConst:
		switch op.Lit.Kind {
		case core.KInt:
			mark(t.set(op.Res[0], "int"))
		case core.KFloat:
			mark(t.set(op.Res[0], "f64"))
		case core.KBool:
			mark(t.set(op.Res[0], "bool"))
		case core.KStr:
			mark(t.set(op.Res[0], "string"))
		}
	case OpIndex:
		if e := elemOf(g.tg, t.get(op.Args[0])); e != "" {
			mark(t.set(op.Res[0], scalar(e)))
		}
	case OpMapRead:
		mark(t.set(op.Res[0], "int"))
		if _, v, ok := core.MapTypes(t.get(op.Args[0])); ok {
			mark(t.set(op.Res[1], scalar(v)))
		}
	case OpBuild:
		p := op.Sub[0].Params[0]
		if len(op.Res) == 1 {
			if y := yields(op.Sub[0]); len(y) == 1 {
				mark(t.union(op.Res[0], y[0]))
			}
		}
		_ = p
	case OpPrim:
		p, ok := g.tg.Prims[op.Name]
		if !ok {
			break
		}
		kind := p.Kind
		if g.tg.IsLengthName(op.Name) {
			kind = "len"
		}
		switch kind {
		case "len":
			mark(t.set(op.Res[0], "int"))
		case "table-set":
			mark(t.union(op.Res[0], op.Args[0]))
			if vt := t.get(op.Args[2]); vt != "" && len(op.Args) == 3 {
				mark(t.set(op.Args[0], "array "+scalar(vt)))
			}
		case "map-insert":
			mark(t.union(op.Res[0], op.Args[0]))
			if kt, vt := t.get(op.Args[1]), t.get(op.Args[2]); kt != "" && vt != "" {
				mark(t.set(op.Args[0], "map "+kt+" "+vt))
			}
		case "table-alloc":
			mark(t.union(op.Res[0], op.Args[0]))
		case "array":
			if len(op.Args) > 0 {
				if et := t.get(op.Args[0]); et != "" {
					mark(t.set(op.Res[0], "array "+et))
				}
			}
		case "map-keys":
			mark(t.set(op.Res[0], "array int"))
		case "stmt":
			mark(t.union(op.Res[0], op.Args[0]))
			for i, a := range op.Args {
				if i < len(p.Args) {
					mark(t.set(a, scalar(p.Args[i])))
				}
			}
		default:
			if len(p.Results) >= 2 {
				for i, r := range op.Res {
					if i < len(p.Results) {
						mark(t.set(r, scalar(p.Results[i])))
					}
				}
			} else if len(op.Res) == 1 {
				mark(t.set(op.Res[0], scalar(p.Result)))
			}
			for i, a := range op.Args {
				if i < len(p.Args) {
					mark(t.set(a, scalar(p.Args[i])))
				}
			}
		}
	}
	return c
}

// yields collects the values a region yields, at its first yield.
func yields(r *Region) []V {
	switch r.T {
	case TYield:
		return r.Args
	case TBranch:
		if y := yields(r.Then); y != nil {
			return y
		}
		return yields(r.Else)
	}
	return nil
}

// terms collects the argument lists of every terminator of one kind in a loop
// body (not inside nested loops, whose terminators are their own).
func terms(r *Region, k TermKind) [][]V {
	var out [][]V
	var walk func(r *Region)
	walk = func(r *Region) {
		if r.T == k {
			out = append(out, r.Args)
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
	return out
}

// ---------------------------------------------------------------- widths

// narrowTables chooses, per class of integer tables, the narrowest host element
// type holding every element the analysis says the class's tables can hold.
func (g *goPrinter) narrowTables() {
	join := map[V]iv{}
	seen := map[V]bool{}
	for v := 0; v < g.fn.NV; v++ {
		r := g.t.find(V(v))
		if g.t.ty[r] != "array int" {
			continue
		}
		f := g.sp.f[v]
		if !f.hasEl {
			seen[r] = true
			join[r] = ivTop
			continue
		}
		if !seen[r] {
			seen[r], join[r] = true, f.el
		} else {
			join[r] = joinIV(join[r], f.el)
		}
	}
	for r, e := range join {
		if !e.bot && !e.nlo && !e.phi {
			g.t.ty[r] = fmt.Sprintf("array int %d %d", e.lo, e.hi)
		}
	}
}

func joinIV(a, b iv) iv { return join(a, b) }

// ---------------------------------------------------------------- printing

func (g *goPrinter) line(format string, args ...any) {
	g.b.WriteString(strings.Repeat("\t", g.ind))
	fmt.Fprintf(&g.b, format, args...)
	g.b.WriteString("\n")
}

func (g *goPrinter) hostTy(v V) string {
	ty := g.t.get(v)
	if ty == "" {
		return "any"
	}
	return g.tg.HostType(ty)
}

func (g *goPrinter) name(v V) string {
	v = g.res(v)
	return fmt.Sprintf("v%d", v)
}

// ref is how a value is written where it is read: a literal inline, a variable
// by name.
func (g *goPrinter) ref(v V) string {
	v = g.res(v)
	if s, ok := g.rename[v]; ok {
		return s
	}
	if d := g.constOf[v]; d != nil {
		switch d.Kind {
		case core.KInt:
			return fmt.Sprint(d.Int)
		case core.KFloat:
			s := fmt.Sprint(d.Float)
			if !strings.ContainsAny(s, ".eE") {
				s += ".0"
			}
			return s
		case core.KBool:
			if d.IsTrue() {
				return "true"
			}
			return "false"
		case core.KStr:
			return fmt.Sprintf("%q", d.Str)
		}
	}
	return g.name(v)
}

// elemWide reports whether a table's element type is narrower than the word, so
// a read converts to `int` and a store converts to the element type.
func (g *goPrinter) elemHost(tab V) (string, bool) {
	e := elemOf(g.tg, g.t.get(tab))
	if e == "" {
		return "", false
	}
	h := g.tg.HostType(e)
	return h, strings.HasPrefix(e, "int ") && h != "int"
}

// countUses counts the reads of every value, after aliases resolve, as the
// LEAST fixpoint of liveness: an operation's arguments are read only if the
// operation is live (it has an effect, owns a region, or a result is read), and
// a yield's or a break's argument j only if its owner's result j is read. The
// function's own yield is always read. Starting from nothing read and counting
// again until the counts stop changing gives the least solution, so a value
// that only a dead value reads is dead too.
func (g *goPrinter) countUses() {
	prev := make([]int, g.fn.NV)
	for {
		g.uses = make([]int, g.fn.NV)
		use := func(vs []V) {
			for _, v := range vs {
				if v >= 0 {
					g.uses[g.res(v)]++
				}
			}
		}
		gate := func(vs, res []V) {
			for j, v := range vs {
				if v >= 0 && (res == nil || j < len(res) && prev[g.res(res[j])] > 0) {
					g.uses[g.res(v)]++
				}
			}
		}
		var region func(r *Region, yieldRes, breakRes []V)
		region = func(r *Region, yieldRes, breakRes []V) {
			for i := range r.Ops {
				op := &r.Ops[i]
				if !g.liveIn(op, prev) {
					continue
				}
				use(op.Args)
				for _, s := range op.Sub {
					if op.Kind == OpLoop {
						region(s, nil, op.Res)
					} else {
						region(s, op.Res, breakRes)
					}
				}
			}
			switch r.T {
			case TYield:
				gate(r.Args, yieldRes)
			case TBreak:
				gate(r.Args, breakRes)
			case TContinue:
				use(r.Args)
			case TBranch:
				use([]V{r.Cond})
				region(r.Then, yieldRes, breakRes)
				region(r.Else, yieldRes, breakRes)
			}
		}
		region(g.fn.Body, nil, nil)
		same := true
		for i := range prev {
			if (prev[i] > 0) != (g.uses[i] > 0) {
				same = false
			}
		}
		if same {
			return
		}
		prev = g.uses
	}
}

// liveIn reports whether an operation must be printed, given which values are
// read: it owns a region, it has an effect, or a result is read.
func (g *goPrinter) liveIn(op *Op, uses []int) bool {
	switch op.Kind {
	case OpLoop, OpIf, OpBuild, OpOpaque:
		return true
	case OpPrim:
		p, ok := g.tg.Prims[op.Name]
		if !ok || !p.Pure {
			return true
		}
		switch p.Kind {
		case "stmt", "table-set", "map-insert":
			return true
		}
	}
	for _, r := range op.Res {
		if uses[g.res(r)] > 0 {
			return true
		}
	}
	return false
}

// aliases resolves the values that ARE other values: π-parameters, a statement's
// value (its first argument), a store's and an insert's buffer, an alloc of a
// table (immutable, so the same table: η-tab), and a build's buffer yielded as
// its result.
func (g *goPrinter) aliases() {
	g.alias = make([]V, g.fn.NV)
	for i := range g.alias {
		g.alias[i] = -1
	}
	g.constOf = map[V]*core.Term{}
	var region func(r *Region)
	region = func(r *Region) {
		for _, pi := range r.Pis {
			g.alias[pi.V] = pi.Of
		}
		for i := range r.Ops {
			op := &r.Ops[i]
			switch op.Kind {
			case OpConst:
				g.constOf[op.Res[0]] = op.Lit
			case OpPrim:
				if p, ok := g.tg.Prims[op.Name]; ok {
					switch p.Kind {
					case "stmt", "table-set", "map-insert", "table-alloc":
						if len(op.Args) > 0 {
							g.alias[op.Res[0]] = op.Args[0]
						}
					}
				}
			}
			for _, s := range op.Sub {
				region(s)
			}
		}
		if r.T == TBranch {
			region(r.Then)
			region(r.Else)
		}
	}
	region(g.fn.Body)
}

// PrintGo prints one export as a Go function.
func PrintGo(tg *emit.Target, name string, sig *core.Sig, fn *Func, sp *Sparse, imps map[string]bool) (string, error) {
	g := &goPrinter{tg: tg, fn: fn, sp: sp, t: newTyper(fn.NV), sig: sig, imps: imps, rename: map[V]string{}}
	g.aliases()
	g.infer()
	g.narrowTables()
	g.countUses()
	var params []string
	for i, p := range fn.Params {
		ty := "any"
		if sig != nil && i < len(sig.Params) {
			ty = tg.HostType(sig.Params[i].Type)
		} else if t := g.t.get(p); t != "" {
			ty = tg.HostType(t)
		}
		params = append(params, fmt.Sprintf("%s %s", g.name(p), ty))
	}
	ret := ""
	if y := yields(fn.Body); len(y) == 1 {
		ret = " " + g.hostTy(y[0])
		if sig != nil && sig.Result != "" {
			if _, _, isR := core.IntRange(sig.Result); !isR {
				ret = " " + tg.HostType(sig.Result)
			}
		}
	} else if len(y) > 1 {
		var tys []string
		for _, v := range y {
			tys = append(tys, g.hostTy(v))
		}
		ret = " (" + strings.Join(tys, ", ") + ")"
	}
	g.line("func %s(%s)%s {", exportName(name), strings.Join(params, ", "), ret)
	g.ind++
	g.region(fn.Body, nil, true)
	g.ind--
	g.line("}")
	return g.b.String(), g.err
}

func exportName(s string) string {
	var b strings.Builder
	for _, part := range strings.Split(s, "-") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

// loopInfo is what the printer decided for one loop.
type loopInfo struct {
	op     *Op
	post   map[int]V // parameter -> the value every back edge gives it
	sole   map[int]int
	result []string // where each result lands
}

// region prints a region's operations and its terminator. `top` is the
// function body, whose yield returns.
func (g *goPrinter) region(r *Region, lp *loopInfo, top bool) {
	for i := range r.Ops {
		g.printOp(&r.Ops[i])
	}
	switch r.T {
	case TYield:
		if top {
			var vs []string
			for _, v := range r.Args {
				vs = append(vs, g.ref(v))
			}
			g.line("return %s", strings.Join(vs, ", "))
		}
		// a non-top yield is assigned by the owner, through yieldTo
		if g.yieldTo != nil {
			g.assign(g.yieldTo, r.Args)
		}
	case TContinue:
		g.again(lp, r.Args)
		g.line("continue")
	case TBreak:
		var lhs, rhs []string
		for j, v := range r.Args {
			if j < len(lp.result) && lp.result[j] != "" && lp.result[j] != g.ref(v) {
				lhs, rhs = append(lhs, lp.result[j]), append(rhs, g.ref(v))
			}
		}
		if len(lhs) > 0 {
			g.line("%s = %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
		}
		g.line("break")
	case TBranch:
		g.line("if %s {", g.ref(r.Cond))
		g.ind++
		g.region(r.Then, lp, top)
		g.ind--
		if terminates(r.Then, top) {
			g.line("}")
			g.region(r.Else, lp, top)
			return
		}
		g.line("} else {")
		g.ind++
		g.region(r.Else, lp, top)
		g.ind--
		g.line("}")
	}
}

// terminates reports whether control never falls out of r: every leaf jumps,
// or returns when r is in the function's tail.
func terminates(r *Region, top bool) bool {
	switch r.T {
	case TBreak, TContinue:
		return true
	case TYield:
		return top
	case TBranch:
		return terminates(r.Then, top) && terminates(r.Else, top)
	}
	return false
}

// assign writes values into destination names, skipping self-assignments.
func (g *goPrinter) assign(dst []string, vs []V) {
	var lhs, rhs []string
	for j, v := range vs {
		if j < len(dst) && dst[j] != "" && dst[j] != g.ref(v) {
			lhs, rhs = append(lhs, dst[j]), append(rhs, g.ref(v))
		}
	}
	if len(lhs) > 0 {
		g.line("%s = %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
	}
}

// again is the back edge: a parallel assignment of the changed parameters,
// except those the post clause updates.
func (g *goPrinter) again(lp *loopInfo, vs []V) {
	var lhs, rhs []string
	for j, v := range vs {
		if _, hoisted := lp.post[j]; hoisted {
			continue
		}
		p := g.name(lp.op.Sub[0].Params[j])
		if s := g.ref(v); s != p {
			lhs, rhs = append(lhs, p), append(rhs, s)
		}
	}
	if len(lhs) > 0 {
		g.line("%s = %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
	}
}

// define declares a value from an expression, unless nothing reads it.
func (g *goPrinter) define(v V, expr string, impure bool) {
	if g.uses[g.res(v)] == 0 {
		if impure {
			g.line("_ = %s", expr)
		}
		return
	}
	g.line("%s := %s", g.name(v), expr)
}

func (g *goPrinter) printOp(op *Op) {
	if !g.liveIn(op, g.uses) {
		return
	}
	switch op.Kind {
	case OpConst:
		return // inline at every read
	case OpGlobal:
		g.define(op.Res[0], exportName(op.Name), false)
	case OpOpaque:
		g.err = fmt.Errorf("an opaque term reached the printer: %s", op.Lit)
	case OpIndex:
		tab, idx := g.ref(op.Args[0]), g.ref(op.Args[1])
		read := fmt.Sprintf("%s[%s]", tab, idx)
		if _, conv := g.elemHost(op.Args[0]); conv {
			read = "int(" + read + ")"
		}
		g.define(op.Res[0], read, false)
	case OpMapRead:
		m, k := g.ref(op.Args[0]), g.ref(op.Args[1])
		tag, pay := g.name(op.Res[0]), g.name(op.Res[1])
		pn := pay
		if g.uses[g.res(op.Res[1])] == 0 {
			pn = "_"
		}
		g.line("%s, ok%d := %s[%s]", pn, op.Res[0], m, k)
		if g.uses[g.res(op.Res[0])] > 0 {
			g.line("%s := 1", tag)
			g.line("if ok%d {", op.Res[0])
			g.line("\t%s = 0", tag)
			g.line("}")
		} else {
			g.line("_ = ok%d", op.Res[0])
		}
	case OpPrim:
		g.printPrim(op)
	case OpIf:
		if e, ok := g.connective(op, g.ref); ok {
			g.define(op.Res[0], e, false)
			return
		}
		dst := g.declareResults(op.Res)
		g.line("if %s {", g.ref(op.Args[0]))
		g.ind++
		g.withYield(dst, func() { g.region(op.Sub[0], nil, false) })
		g.ind--
		g.line("} else {")
		g.ind++
		g.withYield(dst, func() { g.region(op.Sub[1], nil, false) })
		g.ind--
		g.line("}")
	case OpBuild:
		body := op.Sub[0]
		buf := body.Params[0]
		g.line("%s := make(%s, %s)", g.name(buf), g.hostTy(buf), g.ref(op.Args[0]))
		// A build whose every yield is its own buffer IS that buffer.
		if y := yields(body); len(y) == 1 && len(op.Res) == 1 && g.res(y[0]) == g.res(buf) && allYield(body, buf, g) {
			g.alias[op.Res[0]] = buf
			g.region(body, nil, false)
			return
		}
		dst := g.declareResults(op.Res)
		g.withYield(dst, func() { g.region(body, nil, false) })
	case OpLoop:
		g.printLoop(op)
	}
}

// connective prints a boolean `if` with a literal branch as the operator it is
// (booleans.md; cond-2026-09-19): (if c true E) is c || E and (if c E false) is
// c && E, when E is an expression tree, that is, its region is pure and every
// value in it is read once, by the tree.
func (g *goPrinter) connective(op *Op, ref func(V) string) (string, bool) {
	if len(op.Res) != 1 || g.t.get(op.Res[0]) != "bool" {
		return "", false
	}
	return g.branchExpr(ref(op.Args[0]), op.Sub[0], op.Sub[1], ref)
}

// branchExpr is `if c then th else el` as a connective, when one side yields a
// boolean literal and the other is an expression tree.
func (g *goPrinter) branchExpr(c string, th, el *Region, ref func(V) string) (string, bool) {
	lit := func(r *Region) (bool, bool) {
		if r.T != TYield || len(r.Args) != 1 {
			return false, false
		}
		for i := range r.Ops {
			if r.Ops[i].Kind != OpConst {
				return false, false
			}
		}
		d := g.constOf[g.res(r.Args[0])]
		if d == nil || d.Kind != core.KBool {
			return false, false
		}
		return d.IsTrue(), true
	}
	if b, ok := lit(th); ok && b {
		if e, ok := g.exprOf(el, ref); ok {
			return "(" + c + " || " + e + ")", true
		}
	}
	if b, ok := lit(el); ok && !b {
		if e, ok := g.exprOf(th, ref); ok {
			return "(" + c + " && " + e + ")", true
		}
	}
	return "", false
}

// exprOf is a region's yielded value as one expression, or false when the
// region is not an expression tree: it has an effect, a loop, a branch, or a
// value read more than once.
func (g *goPrinter) exprOf(r *Region, outer func(V) string) (string, bool) {
	if r.T == TYield && len(r.Args) != 1 || r.T != TYield && r.T != TBranch {
		return "", false
	}
	exprs := map[V]string{}
	arg := func(v V) string {
		if e, ok := exprs[g.res(v)]; ok {
			return e
		}
		return outer(v)
	}
	for i := range r.Ops {
		o := &r.Ops[i]
		switch o.Kind {
		case OpConst:
			continue
		case OpIndex:
			if g.uses[g.res(o.Res[0])] != 1 {
				return "", false
			}
			e := arg(o.Args[0]) + "[" + arg(o.Args[1]) + "]"
			if _, conv := g.elemHost(o.Args[0]); conv {
				e = "int(" + e + ")"
			}
			exprs[o.Res[0]] = e
		case OpPrim:
			p, ok := g.tg.Prims[o.Name]
			if !ok || !p.Pure || len(o.Res) != 1 || g.uses[g.res(o.Res[0])] != 1 {
				return "", false
			}
			switch p.Kind {
			case "stmt", "table-set", "map-insert", "table-alloc", "array", "map-keys", "iterate", "cond", "let":
				return "", false
			}
			args := make([]string, len(o.Args))
			for k, a := range o.Args {
				args[k] = arg(a)
			}
			if g.tg.IsLengthName(o.Name) {
				exprs[o.Res[0]] = "len(" + args[0] + ")"
			} else {
				exprs[o.Res[0]] = "(" + emit.Fill(p.Form, args...) + ")"
			}
		case OpIf:
			if g.uses[g.res(o.Res[0])] != 1 {
				return "", false
			}
			e, ok := g.connective(o, arg)
			if !ok {
				return "", false
			}
			exprs[o.Res[0]] = e
		default:
			return "", false
		}
	}
	if r.T == TBranch {
		return g.branchExpr(arg(r.Cond), r.Then, r.Else, arg)
	}
	return arg(r.Args[0]), true
}

func allYield(r *Region, v V, g *goPrinter) bool {
	switch r.T {
	case TYield:
		return len(r.Args) == 1 && g.res(r.Args[0]) == g.res(v)
	case TBranch:
		return allYield(r.Then, v, g) && allYield(r.Else, v, g)
	}
	return false
}

func (g *goPrinter) declareResults(res []V) []string {
	dst := make([]string, len(res))
	for j, v := range res {
		if g.uses[g.res(v)] == 0 {
			continue
		}
		dst[j] = g.name(v)
		g.line("var %s %s", dst[j], g.hostTy(v))
	}
	return dst
}

func (g *goPrinter) withYield(dst []string, f func()) {
	old := g.yieldTo
	g.yieldTo = dst
	f()
	g.yieldTo = old
}

func (g *goPrinter) printPrim(op *Op) {
	p, ok := g.tg.Prims[op.Name]
	if !ok {
		g.err = fmt.Errorf("unknown primitive %s", op.Name)
		return
	}
	if p.Import != "" {
		g.imps[p.Import] = true
	}
	args := make([]string, len(op.Args))
	for i, a := range op.Args {
		args[i] = g.ref(a)
	}
	kind := p.Kind
	if g.tg.IsLengthName(op.Name) {
		kind = "len"
	}
	switch kind {
	case "len":
		g.define(op.Res[0], "len("+args[0]+")", false)
		return
	case "table-set":
		v := args[2]
		if h, conv := g.elemHost(op.Args[0]); conv {
			v = h + "(" + v + ")"
		}
		g.line("%s[%s] = %s", args[0], args[1], v)
		return
	case "map-insert":
		g.line("%s[%s] = %s", args[0], args[1], args[2])
		return
	case "table-alloc":
		return // an immutable table: η-tab, the same table
	case "array":
		g.define(op.Res[0], fmt.Sprintf("%s{%s}", g.hostTy(op.Res[0]), strings.Join(args, ", ")), false)
		return
	case "stmt":
		g.line("%s", emit.Fill(p.Form, args...))
		return
	case "map-keys":
		g.err = fmt.Errorf("keys is not printed by the prototype")
		return
	}
	expr := emit.Fill(p.Form, args...)
	if len(op.Res) >= 2 {
		names := make([]string, len(op.Res))
		used := false
		for i, r := range op.Res {
			if g.uses[g.res(r)] > 0 {
				names[i], used = g.name(r), true
			} else {
				names[i] = "_"
			}
		}
		if used {
			g.line("%s := %s", strings.Join(names, ", "), expr)
		} else {
			g.line("%s", expr)
		}
		return
	}
	g.define(op.Res[0], expr, !p.Pure)
}

// printLoop decides soleExit and PostVars, declares the parameters, and prints
// the body.
func (g *goPrinter) printLoop(op *Op) {
	body := op.Sub[0]
	lp := &loopInfo{op: op, post: map[int]V{}}
	// parameters, declared with their initial values
	for j, p := range body.Params {
		init := g.ref(op.Args[j])
		if g.t.get(p) == "int" || g.constOf[g.res(op.Args[j])] != nil {
			g.line("var %s %s = %s", g.name(p), g.hostTy(p), init)
		} else {
			g.line("%s := %s", g.name(p), init)
		}
		if g.uses[g.res(p)] == 0 {
			g.line("_ = %s", g.name(p))
		}
	}
	// soleExit: result j is parameter k at every break
	breaks := terms(body, TBreak)
	lp.result = make([]string, len(op.Res))
	for j, rv := range op.Res {
		k := -1
		for _, b := range breaks {
			if j >= len(b) {
				k = -2
				break
			}
			idx := -1
			for pi, p := range body.Params {
				if g.res(b[j]) == g.res(p) {
					idx = pi
				}
			}
			if idx < 0 || (k >= 0 && idx != k) {
				k = -2
				break
			}
			k = idx
		}
		if k >= 0 && len(breaks) > 0 {
			g.alias[rv] = body.Params[k]
			lp.result[j] = "" // it is the parameter; nothing to assign
			continue
		}
		if g.uses[g.res(rv)] > 0 {
			lp.result[j] = g.name(rv)
			g.line("var %s %s", lp.result[j], g.hostTy(rv))
		}
	}
	// PostVars: parameter j updated by the same operation at every back edge,
	// `p + c` with a literal c, and nothing else reads the updated value
	conts := terms(body, TContinue)
	defs := map[V]*Op{}
	var collect func(r *Region)
	collect = func(r *Region) {
		for i := range r.Ops {
			defs[r.Ops[i].Res0()] = &r.Ops[i]
		}
		if r.T == TBranch {
			collect(r.Then)
			collect(r.Else)
		}
	}
	collect(body)
	var postL, postR []string
	for j, p := range body.Params {
		step := ""
		ok := len(conts) > 0
		for _, c := range conts {
			d := defs[c[j]]
			if d == nil || d.Kind != OpPrim || emit.ArithOp(d.Name, len(d.Args)) != "add" ||
				g.res(d.Args[0]) != g.res(p) || g.constOf[d.Args[1]] == nil || g.uses[g.res(c[j])] != 1 {
				ok = false
				break
			}
			s := g.ref(d.Args[1])
			if step != "" && step != s {
				ok = false
				break
			}
			step = s
		}
		if ok {
			lp.post[j] = conts[0][j]
			for _, c := range conts {
				g.uses[g.res(c[j])] = 0 // the post clause computes it
			}
			postL = append(postL, g.name(p))
			postR = append(postR, fmt.Sprintf("%s + %s", g.name(p), step))
		}
	}
	undo := g.narrowLoop(op)
	defer undo()
	if len(postL) > 0 {
		g.line("for ; ; %s = %s {", strings.Join(postL, ", "), strings.Join(postR, ", "))
	} else {
		g.line("for {")
	}
	g.ind++
	old := g.yieldTo
	g.yieldTo = nil
	g.region(body, lp, false)
	g.yieldTo = old
	g.ind--
	g.line("}")
}

// narrowLoop is bounds-check elimination as an IR rule (bce-2026-08-15, 1.96×
// on compute-bound loops). A loop whose body compares a parameter p against
// `len X`, X invariant, bounds p by X's length; every other invariant table Y
// the loop reads ONLY at p is re-sliced to that length before the loop, so the
// host's prover sees one bound for both. The rule is invisible: an
// out-of-range read is unspecified (primitives.md §2), so moving its panic
// earlier changes no specified answer. It returns the undo of its renaming.
func (g *goPrinter) narrowLoop(op *Op) func() {
	if g.tg.Narrow == "" {
		return func() {}
	}
	body := op.Sub[0]
	inside := map[V]bool{}
	var defs func(r *Region)
	defs = func(r *Region) {
		for _, p := range r.Params {
			inside[p] = true
		}
		for _, pi := range r.Pis {
			inside[pi.V] = true
		}
		for i := range r.Ops {
			for _, v := range r.Ops[i].Res {
				inside[v] = true
			}
			for _, s := range r.Ops[i].Sub {
				defs(s)
			}
		}
		if r.T == TBranch {
			defs(r.Then)
			defs(r.Else)
		}
	}
	defs(body)
	// the guard: a comparison at the body's top level of a parameter and `len X`
	lenOf := map[V]V{}
	param, bound := V(-1), V(-1)
	for i := range body.Ops {
		o := &body.Ops[i]
		if o.Kind != OpPrim {
			continue
		}
		if g.tg.IsLengthName(o.Name) && !inside[g.res(o.Args[0])] {
			lenOf[o.Res[0]] = g.res(o.Args[0])
			continue
		}
		if emit.CmpOp(o.Name) == "" || len(o.Args) != 2 {
			continue
		}
		a, b := g.res(o.Args[0]), o.Args[1]
		if x, ok := lenOf[b]; ok {
			for _, p := range body.Params {
				if g.res(p) == a {
					param, bound = p, x
				}
			}
		}
		if param >= 0 {
			break
		}
	}
	if param < 0 {
		return func() {}
	}
	// candidates: invariant tables read at the parameter, and read nowhere else
	good, bad := map[V]bool{}, map[V]bool{}
	var scan func(r *Region)
	scan = func(r *Region) {
		for i := range r.Ops {
			o := &r.Ops[i]
			if o.Kind == OpIndex || o.Kind == OpPrim && g.tg.Prims[o.Name].Index && len(o.Args) == 2 {
				t := g.res(o.Args[0])
				if !inside[t] && g.res(o.Args[1]) == g.res(param) {
					good[t] = true
				} else {
					bad[t] = true
				}
			} else {
				for _, a := range o.Args {
					bad[g.res(a)] = true
				}
			}
			for _, s := range o.Sub {
				scan(s)
			}
		}
		for _, a := range r.Args {
			bad[g.res(a)] = true
		}
		if r.T == TBranch {
			scan(r.Then)
			scan(r.Else)
		}
	}
	scan(body)
	var ys []V
	for y := range good {
		if !bad[y] && y != bound && g.constOf[y] == nil {
			ys = append(ys, y)
		}
	}
	if len(ys) == 0 {
		return func() {}
	}
	sort.Slice(ys, func(i, j int) bool { return ys[i] < ys[j] })
	n := fmt.Sprintf("n%d", op.Sub[0].Params[0])
	g.line("%s := len(%s)", n, g.ref(bound))
	for _, y := range ys {
		s := fmt.Sprintf("s%d", y)
		g.line("%s", fmt.Sprintf(g.tg.Narrow, s, g.ref(y), n))
		g.rename[y] = s
	}
	return func() {
		for _, y := range ys {
			delete(g.rename, y)
		}
	}
}

// Res0 is an op's first result, or -1.
func (op *Op) Res0() V {
	if len(op.Res) > 0 {
		return op.Res[0]
	}
	return -1
}

// FileGo wraps functions into a Go file of package `pkg`.
func FileGo(pkg string, funcs []string, imps map[string]bool) string {
	var b strings.Builder
	b.WriteString("// Code generated by oroboros irproto (P2). DO NOT EDIT.\n\npackage " + pkg + "\n\n")
	if len(imps) > 0 {
		var ks []string
		for k := range imps {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		b.WriteString("import (\n")
		for _, k := range ks {
			fmt.Fprintf(&b, "\t%q\n", k)
		}
		b.WriteString(")\n\n")
	}
	for _, f := range funcs {
		b.WriteString(f)
		b.WriteString("\n")
	}
	return b.String()
}
