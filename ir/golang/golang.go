// Package golang prints IR_P as Go: a model of Σ in Go (docs/spec/ir.md §9).
//
// A printer spells; it computes no fact. Its input is an IR_P function and the
// target's declarations, and every decision below is either a spelling of an
// operation or one of the measured rules of spec §9.4, stated on the IR:
//
//   - soleExit (coalescing): a loop result every break passes as the same
//     parameter IS that parameter, so no temporary defeats escape analysis;
//   - PostVars: a parameter every continue advances by one literal step moves
//     to the `for` post clause (the body is rewritten, f† is kept: L4);
//   - connectives: (if c true E) is `c || E`, (if c E false) is `c && E`, when
//     E is an expression tree (L10, and L6 for the short circuit);
//   - element widths: a table is stored in ρ_T of its IR_P type, which
//     Theorem D′ made uniform over its class; a narrow element widens on a read
//     and narrows on a store.
//
// Values print as `vN`, literals inline, π-parameters as their source (L9),
// and a pure value no one reads is not printed (L7, total in IR_P).
package golang

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// Func prints one IR_P function. Imports are registered in emit.Imports, which
// emit.File reads, exactly as the term backend does.
func Func(tg *emit.Target, f *ir.Func) (string, error) {
	p := &printer{tg: tg, f: f, imps: map[string]bool{}}
	if err := p.prepare(); err != nil {
		return "", err
	}
	out, err := p.function()
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.Name, err)
	}
	for imp := range p.imps {
		emit.Imports[imp] = true
	}
	return out, nil
}

type printer struct {
	tg   *emit.Target
	f    *ir.Func
	b    strings.Builder
	ind  int
	imps map[string]bool
	err  error

	alias   []ir.V // a π is its source, a store's or statement's value is its buffer
	uses    []int
	constOf map[ir.V]*core.Term
	yieldTo []string
	rename  map[ir.V]string
	spell   map[ir.Op][2]string // an integer operation's form, exact and trap
	tmp     int
	spare   map[*ir.Stmt]string // a back-edge build that writes into its loop's spare (reuse)
}

// ---------------------------------------------------------------- setup

func (p *printer) prepare() error {
	nv := p.f.NV()
	p.alias = make([]ir.V, nv)
	for i := range p.alias {
		p.alias[i] = -1
	}
	p.constOf = map[ir.V]*core.Term{}
	p.rename = map[ir.V]string{}
	p.f.Walk(func(r *ir.Region) {
		for _, pi := range r.Pis {
			p.alias[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch s.Op {
			case ir.OConst:
				p.constOf[s.Res[0]] = s.Lit
			case ir.OSet, ir.OInsert:
				p.alias[s.Res[0]] = s.Args[0]
			case ir.OCall:
				if p.tg.Prims[s.Name].Kind == "stmt" && len(s.Args) > 0 {
					p.alias[s.Res[0]] = s.Args[0]
				}
			}
		}
	})
	p.spell = map[ir.Op][2]string{}
	if err := p.spellings(); err != nil {
		return err
	}
	p.countUses()
	return nil
}

// spellings finds the target's form for each integer operation: a primitive
// the target classifies as that operation on integers, and its checked form.
// It is target data read once; the choice among equal spellings is by name,
// so the printer is a function of its input.
func (p *printer) spellings() error {
	names := make([]string, 0, len(p.tg.Prims))
	for n := range p.tg.Prims {
		names = append(names, n)
	}
	sort.Strings(names)
	arith := map[string]ir.Op{"add": ir.OAdd, "sub": ir.OSub, "mul": ir.OMul, "neg": ir.ONeg, "div": ir.ODiv, "rem": ir.ORem}
	cmp := map[string]ir.Op{"eq": ir.OEq, "ne": ir.ONe, "lt": ir.OLt, "le": ir.OLe, "gt": ir.OGt, "ge": ir.OGe}
	// Two passes: a primitive declared on integers first, then one overloaded
	// over `any` (Go's `!=`), which spells the integer operation too.
	for pass := 0; pass < 2; pass++ {
		for _, n := range names {
			q := p.tg.Prims[n]
			if q.Form == "" || q.Kind != "expr" || emit.IsCheckedName(n) {
				continue
			}
			if pass == 0 && !intArgs(p.tg, q) || pass == 1 && !openArgs(p.tg, q) {
				continue
			}
			var o ir.Op
			var ok bool
			if a := emit.ArithOp(n, len(q.Args)); a != "" && (p.tg.ValueType(q.Result) == "int" || pass == 1 && open(q.Result)) {
				o, ok = arith[a]
			} else if c := emit.CmpOp(n); c != "" && len(q.Args) == 2 && q.Result == "bool" {
				o, ok = cmp[c]
			}
			if !ok {
				continue
			}
			cur, have := p.spell[o]
			if have && (cur[1] != "" || q.Checked == "") {
				continue // keep the first, unless this one also names a checked form
			}
			trap := ""
			if c, ok := p.tg.Prims[q.Checked]; ok {
				trap = c.Form
			}
			p.spell[o] = [2]string{q.Form, trap}
		}
	}
	return nil
}

func open(ty string) bool { return ty == "" || ty == "any" }

// openArgs: every argument is an integer or undeclared.
func openArgs(tg *emit.Target, q emit.Prim) bool {
	if len(q.Args) == 0 {
		return false
	}
	for _, a := range q.Args {
		if tg.ValueType(a) != "int" && !open(a) {
			return false
		}
	}
	return true
}

func intArgs(tg *emit.Target, q emit.Prim) bool {
	if len(q.Args) == 0 {
		return false
	}
	for _, a := range q.Args {
		if tg.ValueType(a) != "int" {
			return false
		}
	}
	return true
}

func (p *printer) res(v ir.V) ir.V {
	for v >= 0 && int(v) < len(p.alias) && p.alias[v] >= 0 && p.alias[v] != v {
		v = p.alias[v]
	}
	return v
}

// countUses is the least fixpoint of liveness (irp2): an operation's operands
// are read only if it is live, and a yield's or break's operand j only if its
// owner's result j is read.
func (p *printer) countUses() {
	nv := p.f.NV()
	prev := make([]int, nv)
	for {
		p.uses = make([]int, nv)
		use := func(vs []ir.V) {
			for _, v := range vs {
				if v >= 0 {
					p.uses[p.res(v)]++
				}
			}
		}
		gate := func(vs, res []ir.V) {
			for j, v := range vs {
				if v >= 0 && (res == nil || j < len(res) && prev[p.res(res[j])] > 0) {
					p.uses[p.res(v)]++
				}
			}
		}
		var region func(r *ir.Region, yieldRes, breakRes []ir.V)
		region = func(r *ir.Region, yieldRes, breakRes []ir.V) {
			for _, pi := range r.Pis {
				_ = pi // a π reads nothing at run time
			}
			for i := range r.Stmts {
				s := &r.Stmts[i]
				if !p.liveIn(s, prev) {
					continue
				}
				use(s.Args)
				for _, sub := range s.Sub {
					switch s.Op {
					case ir.OLoop:
						region(sub, nil, s.Res)
					case ir.OTabulate:
						region(sub, nil, breakRes) // its yield is stored, always read
					default:
						region(sub, s.Res, breakRes)
					}
				}
			}
			switch r.T {
			case ir.TYield:
				gate(r.Args, yieldRes)
			case ir.TBreak:
				gate(r.Args, breakRes)
			case ir.TContinue:
				use(r.Args)
			case ir.TBranch:
				use([]ir.V{r.Cond})
				region(r.Then, yieldRes, breakRes)
				region(r.Else, yieldRes, breakRes)
			}
		}
		region(p.f.Body, nil, nil)
		same := true
		for i := range prev {
			if (prev[i] > 0) != (p.uses[i] > 0) {
				same = false
			}
		}
		if same {
			return
		}
		prev = p.uses
	}
}

func (p *printer) liveIn(s *ir.Stmt, uses []int) bool {
	switch s.Op {
	case ir.OIf:
		// An `if` is total when its arms are, and a total pure operation whose
		// results no one reads is dropped (L7). A LOOP is not: it may not
		// terminate, and dropping it would turn ⊥ into a value.
		if !p.pureRegion(s.Sub[0]) || !p.pureRegion(s.Sub[1]) {
			return true
		}
	case ir.OLoop, ir.OBuild, ir.OBuildMap, ir.OTabulate, ir.OSet, ir.OInsert:
		return true
	case ir.OCall:
		q := p.tg.Prims[s.Name]
		if !q.Pure || q.Kind == "stmt" {
			return true
		}
	case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg:
		if s.Mode == ir.MTrap {
			return true // a trap is an effect: dropping it would change which programs stop
		}
	}
	for _, r := range s.Res {
		if uses[p.res(r)] > 0 {
			return true
		}
	}
	return false
}

// pureRegion: every operation in r is pure and total, so r may be dropped
// when nothing reads what it yields.
func (p *printer) pureRegion(r *ir.Region) bool {
	for i := range r.Stmts {
		s := &r.Stmts[i]
		switch s.Op {
		case ir.OLoop, ir.OBuild, ir.OBuildMap, ir.OTabulate, ir.OSet, ir.OInsert:
			return false
		case ir.OCall:
			if q := p.tg.Prims[s.Name]; !q.Pure || q.Kind == "stmt" {
				return false
			}
		case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg:
			if s.Mode == ir.MTrap {
				return false
			}
		case ir.ODiv, ir.ORem:
			return false // a zero divisor stops the program: not total
		}
		for _, sub := range s.Sub {
			if !p.pureRegion(sub) {
				return false
			}
		}
	}
	if r.T == ir.TBranch {
		return p.pureRegion(r.Then) && p.pureRegion(r.Else)
	}
	return true
}

// ---------------------------------------------------------------- types

// hostTy is how Go holds a VALUE of type t: a scalar in the type that holds
// its value, not its narrowest storage (elemwidth-2026-08-27); a table in
// ρ_T of its element.
func (p *printer) hostTy(t string) string {
	if strings.HasPrefix(t, "buffer ") {
		t = "array " + t[len("buffer "):] // a buffer is the table it is (ADR 0020)
	}
	if strings.HasPrefix(t, "array ") {
		return p.tg.HostType(t)
	}
	if _, _, ok := core.IntRangeBig(t); ok {
		return p.tg.HostType(p.tg.ValueType(t))
	}
	if t == "any" || t == "" {
		return "any"
	}
	return p.tg.HostType(t)
}

func (p *printer) typeOf(v ir.V) string { return p.hostTy(p.f.Types[v]) }

// storage is a table value's element storage, and whether it is narrower than
// the word, so a read widens and a store narrows.
func (p *printer) storage(tab ir.V) (string, bool) {
	t := p.f.Types[p.res(tab)]
	if strings.HasPrefix(t, "buffer ") {
		t = "array " + t[len("buffer "):]
	}
	e := ir.ElemOf(p.tg, t)
	if e == "" {
		return "", false
	}
	if _, _, ok := core.IntRangeBig(e); !ok {
		return p.tg.HostType(e), false
	}
	h := p.tg.HostType(e)
	return h, h != p.tg.HostType("int")
}

// ---------------------------------------------------------------- names

func (p *printer) name(v ir.V) string { return fmt.Sprintf("v%d", p.res(v)) }

func (p *printer) ref(v ir.V) string {
	v = p.res(v)
	if s, ok := p.rename[v]; ok {
		return s
	}
	if d := p.constOf[v]; d != nil {
		return p.lit(d)
	}
	return p.name(v)
}

func (p *printer) refs(vs []ir.V) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = p.ref(v)
	}
	return out
}

func (p *printer) lit(d *core.Term) string {
	switch d.Kind {
	case core.KInt:
		return strconv.FormatInt(d.Int, 10)
	case core.KFloat:
		f := d.Float
		if math.IsInf(f, 1) {
			p.imps["math"] = true
			return "math.Inf(1)"
		}
		if math.IsInf(f, -1) {
			p.imps["math"] = true
			return "math.Inf(-1)"
		}
		s := strconv.FormatFloat(f, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eEn") {
			s += ".0"
		}
		return s
	case core.KBool:
		if d.IsTrue() {
			return "true"
		}
		return "false"
	case core.KStr:
		return strconv.Quote(d.Str)
	}
	return d.String()
}

func (p *printer) line(format string, args ...any) {
	p.b.WriteString(strings.Repeat("\t", p.ind))
	fmt.Fprintf(&p.b, format, args...)
	p.b.WriteString("\n")
}

func (p *printer) fail(format string, args ...any) {
	if p.err == nil {
		p.err = fmt.Errorf(format, args...)
	}
}

// ---------------------------------------------------------------- the function

func (p *printer) function() (string, error) {
	params := make([]string, len(p.f.Params))
	for i, v := range p.f.Params {
		params[i] = p.name(v) + " " + p.typeOf(v)
	}
	ret := ""
	switch len(p.f.Results) {
	case 0:
	case 1:
		ret = " " + p.hostTy(p.f.Results[0])
	default:
		rs := make([]string, len(p.f.Results))
		for i, r := range p.f.Results {
			rs[i] = p.hostTy(r)
		}
		ret = " (" + strings.Join(rs, ", ") + ")"
	}
	p.line("func %s(%s)%s {", emit.ExportName(p.f.Name), strings.Join(params, ", "), ret)
	p.ind++
	for _, v := range p.f.Params {
		if p.uses[v] == 0 {
			p.line("_ = %s", p.name(v))
		}
	}
	p.region(p.f.Body, nil, true)
	p.ind--
	p.line("}")
	return p.b.String(), p.err
}

// loopCtx is what a loop's body needs at its exits.
type loopCtx struct {
	op     *ir.Stmt
	post   map[int]bool
	result []string       // where each result is written, "" when it is coalesced or unread
	spare  map[int]string // parameter j alternates with this spare (buffer reuse)
}

// region prints r's statements and terminator. top: the function's body, whose
// yield returns.
func (p *printer) region(r *ir.Region, lp *loopCtx, top bool) {
	for i := range r.Stmts {
		p.stmt(&r.Stmts[i])
	}
	switch r.T {
	case ir.TYield:
		if top {
			p.line("return %s", strings.Join(p.refs(r.Args), ", "))
			return
		}
		p.assign(p.yieldTo, r.Args)
	case ir.TContinue:
		p.again(lp, r.Args)
		p.line("continue")
	case ir.TBreak:
		if lp != nil {
			p.assign(lp.result, r.Args)
		}
		p.line("break")
	case ir.TBranch:
		p.line("if %s {", p.ref(r.Cond))
		p.ind++
		p.region(r.Then, lp, top)
		p.ind--
		if terminates(r.Then, top) {
			p.line("}")
			p.region(r.Else, lp, top)
			return
		}
		p.line("} else {")
		p.ind++
		p.region(r.Else, lp, top)
		p.ind--
		p.line("}")
	}
}

// terminates: control never falls out of r, every leaf jumps or, in the
// function's tail, returns.
func terminates(r *ir.Region, top bool) bool {
	switch r.T {
	case ir.TBreak, ir.TContinue:
		return true
	case ir.TYield:
		return top
	case ir.TBranch:
		return terminates(r.Then, top) && terminates(r.Else, top)
	}
	return false
}

// assign is a parallel copy (spec §9.3): Go's tuple assignment, with copies of a
// value onto its own name omitted.
func (p *printer) assign(dst []string, vs []ir.V) {
	var lhs, rhs []string
	for j, v := range vs {
		if j < len(dst) && dst[j] != "" && dst[j] != p.ref(v) {
			lhs, rhs = append(lhs, dst[j]), append(rhs, p.ref(v))
		}
	}
	if len(lhs) > 0 {
		p.line("%s = %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
	}
}

func (p *printer) again(lp *loopCtx, vs []ir.V) {
	var lhs, rhs []string
	for j, v := range vs {
		if lp.post[j] {
			continue
		}
		q := p.name(lp.op.Sub[0].Params[j])
		if s := p.ref(v); s != q {
			lhs, rhs = append(lhs, q), append(rhs, s)
		}
		if sp, ok := lp.spare[j]; ok {
			// The swap: the buffer just read becomes the spare the next
			// iteration writes. A parallel assignment, so it reads the old q.
			lhs, rhs = append(lhs, sp), append(rhs, q)
		}
	}
	if len(lhs) > 0 {
		p.line("%s = %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
	}
}

// define binds a value to an expression.
func (p *printer) define(v ir.V, expr string) {
	if p.uses[p.res(v)] == 0 {
		return
	}
	p.line("%s := %s", p.name(v), expr)
}

// ---------------------------------------------------------------- statements

func (p *printer) stmt(s *ir.Stmt) {
	if !p.liveIn(s, p.uses) {
		return
	}
	switch s.Op {
	case ir.OConst:
		// inline at every read
	case ir.OGlobal:
		p.define(s.Res[0], emit.ExportName(s.Name))
	case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg, ir.ODiv, ir.ORem, ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		form := p.spell[s.Op]
		f := form[0]
		if s.Mode == ir.MTrap {
			if form[1] == "" {
				p.fail("the target has no checked form for %s", s.Op)
				return
			}
			f = form[1]
		}
		if f == "" {
			p.fail("the target has no spelling for %s", s.Op)
			return
		}
		expr := "(" + emit.Fill(f, p.refs(s.Args)...) + ")"
		if p.uses[p.res(s.Res[0])] == 0 {
			p.line("_ = %s", expr) // a trap kept for its effect
			return
		}
		p.define(s.Res[0], expr)
	case ir.OCall:
		p.call(s)
	case ir.OIndex:
		read := fmt.Sprintf("%s[%s]", p.ref(s.Args[0]), p.ref(s.Args[1]))
		if _, narrow := p.storage(s.Args[0]); narrow {
			read = p.tg.HostType("int") + "(" + read + ")"
		}
		p.define(s.Res[0], read)
	case ir.OLen:
		p.define(s.Res[0], "len("+p.ref(s.Args[0])+")")
	case ir.OArray:
		p.define(s.Res[0], fmt.Sprintf("%s{%s}", p.typeOf(s.Res[0]), strings.Join(p.refs(s.Args), ", ")))
	case ir.OMap:
		rows := make([]string, 0, len(s.Args)/2)
		for j := 0; j+1 < len(s.Args); j += 2 {
			rows = append(rows, p.ref(s.Args[j])+": "+p.ref(s.Args[j+1]))
		}
		p.define(s.Res[0], fmt.Sprintf("%s{%s}", p.typeOf(s.Res[0]), strings.Join(rows, ", ")))
	case ir.ORead:
		p.read(s)
	case ir.OKeys:
		p.keys(s)
	case ir.OSet:
		x := p.ref(s.Args[2])
		if h, narrow := p.storage(s.Args[0]); narrow {
			x = h + "(" + x + ")"
		}
		p.line("%s[%s] = %s", p.ref(s.Args[0]), p.ref(s.Args[1]), x)
	case ir.OInsert:
		p.line("%s[%s] = %s", p.ref(s.Args[0]), p.ref(s.Args[1]), p.ref(s.Args[2]))
	case ir.OIf:
		p.ifOp(s)
	case ir.OLoop:
		p.loop(s)
	case ir.OBuild, ir.OBuildMap:
		p.build(s)
	case ir.OTabulate:
		p.tabulate(s)
	case ir.ORestrict:
		// Go's slice expression is the restriction (L13): the same array, a
		// shorter view, which is what lets the host's prover share one bound.
		p.define(s.Res[0], fmt.Sprintf("%s[:%s]", p.ref(s.Args[0]), p.ref(s.Args[1])))
	case ir.OAssume:
		// An assumption: its caller guarantees it (ADR 0028). Nothing to run.
	case ir.OThe, ir.ORequire:
		p.fail("(%s …) in IR_P", s.Op)
	}
}

func (p *printer) call(s *ir.Stmt) {
	q, ok := p.tg.Prims[s.Name]
	if !ok {
		p.fail("%s is not a primitive of target %s", s.Name, p.tg.Name)
		return
	}
	if q.Import != "" {
		p.imps[q.Import] = true
	}
	expr := emit.Fill(q.Form, p.refs(s.Args)...)
	switch {
	case q.Kind == "stmt":
		p.line("%s", expr)
	case len(q.Results) >= 2:
		recv := make([]string, len(s.Res))
		var conv []string
		used := false
		for i, v := range s.Res {
			if p.uses[p.res(v)] == 0 {
				recv[i] = "_"
				continue
			}
			used = true
			recv[i] = p.name(v)
			if i < len(q.Results) && scalarRange(q.Results[i]) {
				h := p.name(v) + "h"
				recv[i] = h
				conv = append(conv, fmt.Sprintf("%s := %s(%s)", p.name(v), p.tg.HostType("int"), h))
			}
		}
		if !used {
			p.line("%s", expr)
			return
		}
		p.line("%s := %s", strings.Join(recv, ", "), expr)
		for _, c := range conv {
			p.line("%s", c)
		}
	default:
		if scalarRange(q.Result) {
			// Received into the language's integer, as a several-result call is.
			expr = p.tg.HostType("int") + "(" + expr + ")"
		} else {
			expr = "(" + expr + ")"
		}
		if p.uses[p.res(s.Res[0])] == 0 {
			p.line("%s", strings.TrimSuffix(strings.TrimPrefix(expr, "("), ")"))
			return
		}
		p.define(s.Res[0], expr)
	}
}

func scalarRange(ty string) bool {
	_, _, ok := core.IntRange(ty)
	return ok
}

// read is `((m k) (fn (#t #p) …))`: Go's comma-ok, with the tag as an integer
// the program tests (maps.md).
func (p *printer) read(s *ir.Stmt) {
	tag, pay := s.Res[0], s.Res[1]
	pn := p.name(pay)
	if p.uses[p.res(pay)] == 0 {
		pn = "_"
	}
	ok := fmt.Sprintf("ok%d", tag)
	if p.uses[p.res(tag)] == 0 {
		if pn == "_" {
			return
		}
		p.line("%s := %s[%s]", pn, p.ref(s.Args[0]), p.ref(s.Args[1]))
		return
	}
	p.line("%s, %s := %s[%s]", pn, ok, p.ref(s.Args[0]), p.ref(s.Args[1]))
	p.line("%s := 1", p.name(tag))
	p.line("if %s {", ok)
	p.line("\t%s = 0", p.name(tag))
	p.line("}")
}

// keys is the map's keys in ascending order (maps.md §7): the host's sort,
// because Go randomises its own iteration order.
func (p *printer) keys(s *ir.Stmt) {
	m := p.ref(s.Args[0])
	out := p.name(s.Res[0])
	p.line("%s := make(%s, 0, len(%s))", out, p.typeOf(s.Res[0]), m)
	p.line("for k := range %s {", m)
	p.line("\t%s = append(%s, k)", out, out)
	p.line("}")
	p.line("sort.Slice(%s, func(i, j int) bool { return %s[i] < %s[j] })", out, out, out)
	p.imps["sort"] = true
}

// ---------------------------------------------------------------- regions

func (p *printer) declare(res []ir.V) []string {
	dst := make([]string, len(res))
	for j, v := range res {
		if p.uses[p.res(v)] == 0 {
			continue
		}
		dst[j] = p.name(v)
		p.line("var %s %s", dst[j], p.typeOf(v))
	}
	return dst
}

func (p *printer) with(dst []string, f func()) {
	old := p.yieldTo
	p.yieldTo = dst
	f()
	p.yieldTo = old
}

func (p *printer) ifOp(s *ir.Stmt) {
	if e, ok := p.connective(s, p.ref); ok {
		p.define(s.Res[0], e)
		return
	}
	dst := p.declare(s.Res)
	p.line("if %s {", p.ref(s.Args[0]))
	p.ind++
	p.with(dst, func() { p.region(s.Sub[0], nil, false) })
	p.ind--
	p.line("} else {")
	p.ind++
	p.with(dst, func() { p.region(s.Sub[1], nil, false) })
	p.ind--
	p.line("}")
}

func (p *printer) build(s *ir.Stmt) {
	body := s.Sub[0]
	buf := body.Params[0]
	if sp, ok := p.spare[s]; ok {
		// A back-edge build writes into its loop's spare, cleared because a
		// build zero-fills (tables.md §14.3) and the program may rely on it.
		p.line("clear(%s)", sp)
		p.line("%s := %s", p.name(buf), sp)
	} else {
		p.line("%s := make(%s, %s)", p.name(buf), p.typeOf(buf), p.ref(s.Args[0]))
	}
	// A build whose every yield is its own buffer IS that buffer (ADR 0031's
	// freeze copies nothing).
	if len(s.Res) == 1 && yieldsOnly(body, buf, p) {
		p.alias[s.Res[0]] = buf
		p.with(nil, func() { p.region(body, nil, false) })
		return
	}
	dst := p.declare(s.Res)
	p.with(dst, func() { p.region(body, nil, false) })
}

func yieldsOnly(r *ir.Region, v ir.V, p *printer) bool {
	switch r.T {
	case ir.TYield:
		return len(r.Args) == 1 && p.res(r.Args[0]) == p.res(v)
	case ir.TBranch:
		return yieldsOnly(r.Then, v, p) && yieldsOnly(r.Else, v, p)
	}
	return false
}

// tabulate is `alloc (table n f)`: allocate, then store the rule at every
// index (L11).
func (p *printer) tabulate(s *ir.Stmt) {
	t := p.name(s.Res[0])
	n := fmt.Sprintf("n%d", s.Res[0])
	i := p.name(s.Sub[0].Params[0])
	p.line("var %s %s = %s", n, p.tg.HostType("int"), p.ref(s.Args[0]))
	p.line("%s := make(%s, %s)", t, p.typeOf(s.Res[0]), n)
	p.line("for %s := 0; %s < %s; %s++ {", i, i, n, i)
	p.ind++
	e := fmt.Sprintf("e%d", s.Res[0])
	var ety string
	if y := firstYield(s.Sub[0]); len(y) == 1 {
		ety = p.typeOf(y[0])
	}
	p.line("var %s %s", e, ety)
	p.with([]string{e}, func() { p.region(s.Sub[0], nil, false) })
	x := e
	if h, narrow := p.storage(s.Res[0]); narrow {
		x = h + "(" + e + ")"
	}
	p.line("%s[%s] = %s", t, i, x)
	p.ind--
	p.line("}")
}

func firstYield(r *ir.Region) []ir.V {
	switch r.T {
	case ir.TYield:
		return r.Args
	case ir.TBranch:
		if y := firstYield(r.Then); y != nil {
			return y
		}
		return firstYield(r.Else)
	}
	return nil
}

func exits(r *ir.Region, k ir.Term) [][]ir.V {
	var out [][]ir.V
	var walk func(r *ir.Region)
	walk = func(r *ir.Region) {
		if r.T == k {
			out = append(out, r.Args)
		}
		if r.T == ir.TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
	return out
}

// loop prints the Elgot iterate as `for`, deciding soleExit and PostVars.
func (p *printer) loop(s *ir.Stmt) {
	body := s.Sub[0]
	lp := &loopCtx{op: s, post: map[int]bool{}}
	for j, q := range body.Params {
		init := p.ref(s.Args[j])
		if p.f.Types[q] != "" && (p.constOf[p.res(s.Args[j])] != nil || strings.HasPrefix(p.typeOf(q), "int")) {
			p.line("var %s %s = %s", p.name(q), p.typeOf(q), init)
		} else {
			p.line("%s := %s", p.name(q), init)
		}
		if p.uses[p.res(q)] == 0 {
			p.line("_ = %s", p.name(q))
		}
	}
	// soleExit: result j is parameter k at every break.
	breaks := exits(body, ir.TBreak)
	lp.result = make([]string, len(s.Res))
	for j, rv := range s.Res {
		k := -1
		for _, b := range breaks {
			idx := -1
			if j < len(b) {
				for pi, q := range body.Params {
					if p.res(b[j]) == p.res(q) {
						idx = pi
					}
				}
			}
			if idx < 0 || (k >= 0 && idx != k) {
				k = -2
				break
			}
			k = idx
		}
		if k >= 0 && len(breaks) > 0 {
			p.alias[rv] = body.Params[k]
			continue
		}
		if p.uses[p.res(rv)] > 0 {
			lp.result[j] = p.name(rv)
			p.line("var %s %s", lp.result[j], p.typeOf(rv))
		}
	}
	// PostVars: parameter j advanced by one literal step at every continue, by
	// a value nothing else reads.
	conts := exits(body, ir.TContinue)
	defs := map[ir.V]*ir.Stmt{}
	var collect func(r *ir.Region)
	collect = func(r *ir.Region) {
		for i := range r.Stmts {
			if len(r.Stmts[i].Res) > 0 {
				defs[r.Stmts[i].Res[0]] = &r.Stmts[i]
			}
		}
		if r.T == ir.TBranch {
			collect(r.Then)
			collect(r.Else)
		}
	}
	collect(body)
	var postL, postR []string
	for j, q := range body.Params {
		step := ""
		ok := len(conts) > 0
		for _, c := range conts {
			d := defs[c[j]]
			if d == nil || d.Op != ir.OAdd || d.Mode == ir.MTrap || p.res(d.Args[0]) != p.res(q) ||
				p.constOf[d.Args[1]] == nil || p.uses[p.res(c[j])] != 1 {
				ok = false
				break
			}
			st := p.ref(d.Args[1])
			if step != "" && step != st {
				ok = false
				break
			}
			step = st
		}
		if ok {
			lp.post[j] = true
			for _, c := range conts {
				p.uses[p.res(c[j])] = 0 // the post clause computes it
			}
			postL = append(postL, p.name(q))
			postR = append(postR, fmt.Sprintf("(%s + %s)", p.name(q), step))
		}
	}
	p.reuse(lp)
	if len(postL) > 0 {
		p.line("for ; ; %s = %s {", strings.Join(postL, ", "), strings.Join(postR, ", "))
	} else {
		p.line("for {")
	}
	p.ind++
	old := p.yieldTo
	p.yieldTo = nil
	p.region(body, lp, false)
	p.yieldTo = old
	p.ind--
	p.line("}")
}

// reuse is LOOP-CARRIED BUFFER REUSE, Rule R one level up (emit/target.go's
// LoopBufferReuse, 2.5–2.7× measured), stated on the IR. Parameter j
// alternates with a spare allocated once when:
//  1. the continue's argument j is a build of literal length n in the body;
//  2. j's initial value is a build of the same length;
//  3. no other argument of the continue carries the old buffer;
//  4. the loop has exactly one continue.
//
// The body always reads one buffer and writes the other, so no aliasing
// argument is needed: the buffer it writes was last read an iteration ago.
func (p *printer) reuse(lp *loopCtx) {
	body := lp.op.Sub[0]
	conts := exits(body, ir.TContinue)
	if len(conts) != 1 {
		return // (4)
	}
	builds := map[ir.V]*ir.Stmt{}
	var collect func(r *ir.Region)
	collect = func(r *ir.Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == ir.OBuild && len(s.Res) == 1 {
				builds[s.Res[0]] = s
			}
			for _, sub := range r.Stmts[i].Sub {
				collect(sub)
			}
		}
		if r.T == ir.TBranch {
			collect(r.Then)
			collect(r.Else)
		}
	}
	collect(p.f.Body)
	constLen := func(s *ir.Stmt) (int64, bool) {
		if d := p.constOf[p.res(s.Args[0])]; d != nil && d.Kind == core.KInt {
			return d.Int, true
		}
		return 0, false
	}
	for j, q := range body.Params {
		arg := conts[0][j]
		b, ok := builds[p.res(arg)]
		if !ok {
			continue
		}
		n, ok := constLen(b)
		if !ok {
			continue // (1)
		}
		ib, ok := builds[p.res(lp.op.Args[j])]
		if !ok {
			continue
		}
		if m, ok := constLen(ib); !ok || m != n {
			continue // (2)
		}
		carried := false
		for k, a := range conts[0] {
			if k != j && p.res(a) == p.res(q) {
				carried = true
			}
		}
		if carried {
			continue // (3)
		}
		if lp.spare == nil {
			lp.spare = map[int]string{}
		}
		sp := fmt.Sprintf("sp%d", p.res(q))
		lp.spare[j] = sp
		if p.spare == nil {
			p.spare = map[*ir.Stmt]string{}
		}
		p.spare[b] = sp
		p.line("%s := make(%s, %d)", sp, p.typeOf(q), n)
	}
}

// ---------------------------------------------------------------- connectives

func (p *printer) connective(s *ir.Stmt, ref func(ir.V) string) (string, bool) {
	if len(s.Res) != 1 || p.f.Types[s.Res[0]] != "bool" {
		return "", false
	}
	return p.branchExpr(ref(s.Args[0]), s.Sub[0], s.Sub[1], ref)
}

func (p *printer) branchExpr(c string, th, el *ir.Region, ref func(ir.V) string) (string, bool) {
	lit := func(r *ir.Region) (bool, bool) {
		if r.T != ir.TYield || len(r.Args) != 1 {
			return false, false
		}
		for i := range r.Stmts {
			if r.Stmts[i].Op != ir.OConst {
				return false, false
			}
		}
		d := p.constOf[p.res(r.Args[0])]
		if d == nil || d.Kind != core.KBool {
			return false, false
		}
		return d.IsTrue(), true
	}
	if b, ok := lit(th); ok && b {
		if e, ok := p.exprOf(el, ref); ok {
			return "(" + c + " || " + e + ")", true
		}
	}
	if b, ok := lit(el); ok && !b {
		if e, ok := p.exprOf(th, ref); ok {
			return "(" + c + " && " + e + ")", true
		}
	}
	return "", false
}

// exprOf is a region's value as one expression, when the region is an
// expression tree: pure, every value read once, no loop, no store.
func (p *printer) exprOf(r *ir.Region, outer func(ir.V) string) (string, bool) {
	if !(r.T == ir.TYield && len(r.Args) == 1) && r.T != ir.TBranch {
		return "", false
	}
	exprs := map[ir.V]string{}
	arg := func(v ir.V) string {
		if e, ok := exprs[p.res(v)]; ok {
			return e
		}
		return outer(v)
	}
	for i := range r.Stmts {
		s := &r.Stmts[i]
		once := len(s.Res) == 1 && p.uses[p.res(s.Res[0])] == 1
		switch {
		case s.Op == ir.OConst:
			continue
		case s.Op.IsCmp() || (s.Op.IsArith() && s.Mode != ir.MTrap):
			f := p.spell[s.Op][0]
			if !once || f == "" {
				return "", false
			}
			args := make([]string, len(s.Args))
			for k, a := range s.Args {
				args[k] = arg(a)
			}
			exprs[s.Res[0]] = "(" + emit.Fill(f, args...) + ")"
		case s.Op == ir.OIndex:
			if !once {
				return "", false
			}
			e := arg(s.Args[0]) + "[" + arg(s.Args[1]) + "]"
			if _, narrow := p.storage(s.Args[0]); narrow {
				e = p.tg.HostType("int") + "(" + e + ")"
			}
			exprs[s.Res[0]] = e
		case s.Op == ir.OLen:
			if !once {
				return "", false
			}
			exprs[s.Res[0]] = "len(" + arg(s.Args[0]) + ")"
		case s.Op == ir.OCall:
			q := p.tg.Prims[s.Name]
			if !once || !q.Pure || q.Kind != "expr" || len(q.Results) >= 2 {
				return "", false
			}
			if q.Import != "" {
				p.imps[q.Import] = true
			}
			args := make([]string, len(s.Args))
			for k, a := range s.Args {
				args[k] = arg(a)
			}
			e := "(" + emit.Fill(q.Form, args...) + ")"
			if scalarRange(q.Result) {
				e = p.tg.HostType("int") + e
			}
			exprs[s.Res[0]] = e
		case s.Op == ir.OIf:
			if !once {
				return "", false
			}
			e, ok := p.connective(s, arg)
			if !ok {
				return "", false
			}
			exprs[s.Res[0]] = e
		default:
			return "", false
		}
	}
	if r.T == ir.TBranch {
		return p.branchExpr(arg(r.Cond), r.Then, r.Else, arg)
	}
	return arg(r.Args[0]), true
}

// FromResidual is the IR's whole path to Go for one definition: lower (L),
// finalize (IR_A → IR_P), verify, print (docs/spec/ir.md §7–§9).
func FromResidual(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) (string, error) {
	f, err := ir.Lower(tg, name, sig, nf, ir.Options{Decided: true})
	if err != nil {
		return "", err
	}
	p := &ir.Program{Target: tg.Name, Funcs: []*ir.Func{f}}
	if err := ir.Finalize(tg, p); err != nil {
		return "", err
	}
	if err := ir.Verify(tg, p); err != nil {
		return "", fmt.Errorf("%s: IR_P does not verify:\n%v", name, err)
	}
	return Func(tg, f)
}
