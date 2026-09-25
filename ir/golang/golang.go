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
	"strconv"
	"strings"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/plan"
)

// Func prints one IR_P function. Imports are registered in emit.Imports, which
// emit.File reads, exactly as the term backend does.
func Func(tg *emit.Target, f *ir.Func) (string, error) {
	p := &printer{tg: tg, f: f, imps: map[string]bool{}, rename: map[ir.V]string{}}
	p.pl = plan.New(tg, f)
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
	pl   *plan.Plan // the host-independent decisions (ir/plan)
	b    strings.Builder
	ind  int
	imps map[string]bool
	err  error

	yieldTo []string
	rename  map[ir.V]string
	spare   map[*ir.Stmt]string // a back-edge build that writes into its loop's spare (reuse)
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
	t := p.f.Types[p.pl.Res(tab)]
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

func (p *printer) name(v ir.V) string { return fmt.Sprintf("v%d", p.pl.Res(v)) }

func (p *printer) ref(v ir.V) string {
	v = p.pl.Res(v)
	if s, ok := p.rename[v]; ok {
		return s
	}
	if d := p.pl.Const[v]; d != nil {
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
		if p.pl.Uses[v] == 0 {
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
		if plan.Terminates(r.Then, top) {
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
	if p.pl.Uses[p.pl.Res(v)] == 0 {
		return
	}
	p.line("%s := %s", p.name(v), expr)
}

// ---------------------------------------------------------------- statements

func (p *printer) stmt(s *ir.Stmt) {
	if !p.pl.Live(s) {
		return
	}
	switch s.Op {
	case ir.OConst:
		// inline at every read
	case ir.OGlobal:
		p.define(s.Res[0], emit.ExportName(s.Name))
	case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg, ir.ODiv, ir.ORem, ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		form := p.pl.Spell[s.Op]
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
		if p.pl.Uses[p.pl.Res(s.Res[0])] == 0 {
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
			if p.pl.Uses[p.pl.Res(v)] == 0 {
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
		if p.pl.Uses[p.pl.Res(s.Res[0])] == 0 {
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
	if p.pl.Uses[p.pl.Res(pay)] == 0 {
		pn = "_"
	}
	ok := fmt.Sprintf("ok%d", tag)
	if p.pl.Uses[p.pl.Res(tag)] == 0 {
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
		if p.pl.Uses[p.pl.Res(v)] == 0 {
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
	if e, ok := p.pl.Connective(s, p.ref, p); ok {
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
	if len(s.Res) == 1 && p.pl.YieldsOnly(body, buf) {
		p.pl.Alias[s.Res[0]] = buf
		p.with(nil, func() { p.region(body, nil, false) })
		return
	}
	dst := p.declare(s.Res)
	p.with(dst, func() { p.region(body, nil, false) })
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
	if y := plan.FirstYield(s.Sub[0]); len(y) == 1 {
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

// loop prints the Elgot iterate as `for`, deciding soleExit and PostVars.
func (p *printer) loop(s *ir.Stmt) {
	body := s.Sub[0]
	lp := &loopCtx{op: s, post: map[int]bool{}}
	for j, q := range body.Params {
		init := p.ref(s.Args[j])
		if p.f.Types[q] != "" && (p.pl.Const[p.pl.Res(s.Args[j])] != nil || strings.HasPrefix(p.typeOf(q), "int")) {
			p.line("var %s %s = %s", p.name(q), p.typeOf(q), init)
		} else {
			p.line("%s := %s", p.name(q), init)
		}
		if p.pl.Uses[p.pl.Res(q)] == 0 {
			p.line("_ = %s", p.name(q))
		}
	}
	d := p.pl.DecideLoop(s)
	// soleExit: a result that IS a parameter is that parameter.
	lp.result = make([]string, len(s.Res))
	for j, rv := range s.Res {
		if k := d.Coalesced[j]; k >= 0 {
			p.pl.Alias[rv] = body.Params[k]
			continue
		}
		if p.pl.Uses[p.pl.Res(rv)] > 0 {
			lp.result[j] = p.name(rv)
			p.line("var %s %s", lp.result[j], p.typeOf(rv))
		}
	}
	// PostVars: the stepped parameters, in the post clause.
	var postL, postR []string
	for j, q := range body.Params {
		if step, ok := d.Post[j]; ok {
			lp.post[j] = true
			postL = append(postL, p.name(q))
			postR = append(postR, fmt.Sprintf("(%s + %s)", p.name(q), p.lit(step)))
		}
	}
	// Buffer reuse: a spare per alternating parameter, allocated once.
	for j, q := range body.Params {
		sb, ok := d.Spare[j]
		if !ok {
			continue
		}
		if lp.spare == nil {
			lp.spare = map[int]string{}
		}
		sp := fmt.Sprintf("sp%d", p.pl.Res(q))
		lp.spare[j] = sp
		if p.spare == nil {
			p.spare = map[*ir.Stmt]string{}
		}
		p.spare[sb.Build] = sp
		p.line("%s := make(%s, %d)", sp, p.typeOf(q), sb.Len)
	}
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

// ---------------------------------------------------------------- the Speller (ir/plan)

func (p *printer) Op(s *ir.Stmt, args []string) string {
	f := p.pl.Spell[s.Op][0]
	if f == "" {
		return ""
	}
	return "(" + emit.Fill(f, args...) + ")"
}

func (p *printer) Index(tab ir.V, t, i string) string {
	e := t + "[" + i + "]"
	if _, narrow := p.storage(tab); narrow {
		e = p.tg.HostType("int") + "(" + e + ")"
	}
	return e
}

func (p *printer) Len(v ir.V, t string) string { return "len(" + t + ")" }

func (p *printer) Call(s *ir.Stmt, args []string) string {
	q := p.tg.Prims[s.Name]
	if q.Import != "" {
		p.imps[q.Import] = true
	}
	e := "(" + emit.Fill(q.Form, args...) + ")"
	if scalarRange(q.Result) {
		e = p.tg.HostType("int") + e
	}
	return e
}

func (p *printer) Or(a, b string) string  { return "(" + a + " || " + b + ")" }
func (p *printer) And(a, b string) string { return "(" + a + " && " + b + ")" }

// Cond: Go has no conditional expression.
func (p *printer) Cond(c, a, b string) string { return "" }

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
