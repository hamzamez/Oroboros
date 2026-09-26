// Package js prints IR_P as JavaScript: a model of Σ in JavaScript
// (docs/spec/ir.md §9). The decisions are ir/plan's; this is the spelling.
//
// What is JavaScript's own, each measured on V8 (native-js-2026-08-20,
// multiresult-2026-08-22, maps-2026-08-30):
//   - a loop in the function's tail RETURNS from its exits instead of
//     assigning a result and breaking (1.31×): L2, the join's continuation is
//     the function's return;
//   - several results are an OBJECT {f0, f1}, not an array (1.62× when the
//     caller reads a field, equal when it destructures);
//   - a table is `new Array(n).fill(0)`, PACKED (a sparse array is a
//     dictionary on V8); a map is a plain object;
//   - JavaScript has no tuple assignment (the comma operator is not one, which
//     the term backend was caught by), so the back edge is ir/plan's parallel
//     move, sequentialised with one temporary per cycle;
//   - an `if` whose arms are expression trees is a conditional expression.
//
// A number is a double and the word is ±(2⁵³−1), so an integer needs no
// conversion; ℤ's division is the target's `idiv`, never `/` (IntegerDivision).
package js

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

// Func prints one IR_P function. Imports go to emit.JSImports, which the file
// writer reads, as the term backend does.
func Func(tg *emit.Target, f *ir.Func) (string, error) {
	p := &printer{tg: tg, f: f, pl: plan.New(tg, f), pending: map[ir.V]string{}}
	p.inline = p.pl.Inlinable()
	out, err := p.function()
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.Name, err)
	}
	return out, nil
}

type printer struct {
	tg      *emit.Target
	f       *ir.Func
	pl      *plan.Plan
	b       strings.Builder
	ind     int
	err     error
	tmp     int
	yieldTo []string
	spare   map[*ir.Stmt]string
	tailOp  *ir.Stmt // the loop whose exits return: the function's tail

	inline  map[ir.V]bool   // values written at their use (ir/plan Inlinable)
	pending map[ir.V]string // their expressions, until the use reads them
}

// loopCtx is what a loop's body needs at its exits.
type loopCtx struct {
	op     *ir.Stmt
	post   map[int]bool
	result []string
	spare  map[int]string
	tail   bool // its breaks return
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

func (p *printer) fresh(stem string) string {
	p.tmp++
	return fmt.Sprintf("%s%d", stem, p.tmp)
}

// ---------------------------------------------------------------- names

func (p *printer) name(v ir.V) string { return fmt.Sprintf("v%d", p.pl.Res(v)) }

func (p *printer) ref(v ir.V) string {
	v = p.pl.Res(v)
	if e, ok := p.pending[v]; ok {
		// Not consumed: the value is read once in the PROGRAM, but a printer
		// may spell a read more than once while it tries alternatives (the
		// connective, then the ternary); only one spelling is kept.
		return e
	}
	if d := p.pl.Const[v]; d != nil {
		return lit(d)
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

func lit(d *core.Term) string {
	switch d.Kind {
	case core.KInt:
		return strconv.FormatInt(d.Int, 10)
	case core.KFloat:
		switch {
		case math.IsInf(d.Float, 1):
			return "Infinity"
		case math.IsInf(d.Float, -1):
			return "-Infinity"
		case math.IsNaN(d.Float):
			return "NaN"
		}
		return strconv.FormatFloat(d.Float, 'g', -1, 64)
	case core.KBool:
		if d.IsTrue() {
			return "true"
		}
		return "false"
	case core.KStr:
		return emit.UTF16StringLit(d.Str)
	}
	return d.String()
}

func (p *printer) isMap(v ir.V) bool { return strings.HasPrefix(p.f.Types[p.pl.Res(v)], "map ") }

// ---------------------------------------------------------------- the function

func (p *printer) function() (string, error) {
	params := make([]string, len(p.f.Params))
	for i, v := range p.f.Params {
		params[i] = p.name(v)
	}
	p.tailOp = p.tailLoop()
	p.line("export function %s(%s) {", emit.JSMangle(p.f.Name), strings.Join(params, ", "))
	p.ind++
	p.region(p.f.Body, nil, true)
	p.ind--
	p.line("}")
	return p.b.String(), p.err
}

// tailLoop is the loop, if any, whose results the function returns directly:
// the body's last statement, yielded as it is. Its exits return (L2).
func (p *printer) tailLoop() *ir.Stmt {
	r := p.f.Body
	if r.T != ir.TYield || len(r.Stmts) == 0 {
		return nil
	}
	s := &r.Stmts[len(r.Stmts)-1]
	if s.Op != ir.OLoop || len(s.Res) != len(r.Args) {
		return nil
	}
	for j, a := range r.Args {
		if a != s.Res[j] {
			return nil
		}
	}
	return s
}

// ret is the return of the function's results.
func (p *printer) ret(vs []ir.V) {
	switch len(vs) {
	case 0:
		p.line("return;")
	case 1:
		p.line("return %s;", p.ref(vs[0]))
	default:
		fields := make([]string, len(vs))
		for i, v := range vs {
			fields[i] = fmt.Sprintf("f%d: %s", i, p.ref(v))
		}
		// An OBJECT, not an array (multiresult-2026-08-22).
		p.line("return {%s};", strings.Join(fields, ", "))
	}
}

func (p *printer) region(r *ir.Region, lp *loopCtx, top bool) {
	for i := range r.Stmts {
		p.stmt(&r.Stmts[i])
	}
	switch r.T {
	case ir.TYield:
		if top {
			if p.tailOp != nil && r == p.f.Body {
				return // the tail loop returned from every exit
			}
			p.ret(r.Args)
			return
		}
		p.move(p.yieldTo, r.Args)
	case ir.TContinue:
		p.again(lp, r.Args)
		p.line("continue;")
	case ir.TBreak:
		if lp != nil && lp.tail {
			p.ret(r.Args)
			return
		}
		if lp != nil {
			p.move(lp.result, r.Args)
		}
		p.line("break;")
	case ir.TBranch:
		p.line("if (%s) {", p.ref(r.Cond))
		p.ind++
		p.region(r.Then, lp, top)
		p.ind--
		if plan.Terminates(r.Then, top) || lp != nil && lp.tail && returns(r.Then) {
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

// returns: every leaf of r leaves by a break of a tail loop, which returns.
func returns(r *ir.Region) bool {
	switch r.T {
	case ir.TBreak, ir.TContinue:
		return true
	case ir.TBranch:
		return returns(r.Then) && returns(r.Else)
	}
	return false
}

// move is a parallel move dst ← vs, sequentialised by ir/plan.
func (p *printer) move(dst []string, vs []ir.V) {
	src := make([]string, len(dst))
	d := make([]string, len(dst))
	for j := range dst {
		if j < len(vs) {
			d[j], src[j] = dst[j], p.ref(vs[j])
		}
	}
	p.moves(d, src)
}

func (p *printer) moves(dst, src []string) {
	temps := map[string]bool{}
	for _, m := range plan.Moves(dst, src, func() string { t := p.fresh("u"); temps[t] = true; return t }) {
		if temps[m[0]] {
			p.line("const %s = %s;", m[0], m[1])
		} else {
			p.line("%s = %s;", m[0], m[1])
		}
	}
}

// again is the back edge: the parameters' parallel move, minus what the post
// clause updates, plus the swap of each reused buffer with its spare.
func (p *printer) again(lp *loopCtx, vs []ir.V) {
	var dst, src []string
	for j, v := range vs {
		if lp.post[j] {
			continue
		}
		q := p.name(lp.op.Sub[0].Params[j])
		dst, src = append(dst, q), append(src, p.ref(v))
		if sp, ok := lp.spare[j]; ok {
			dst, src = append(dst, sp), append(src, q) // the spare takes the old buffer
		}
	}
	p.moves(dst, src)
}

func (p *printer) define(v ir.V, expr string) {
	if !p.pl.Read(v) {
		return
	}
	if p.inline[v] {
		p.pending[v] = expr // β: written where it is read
		return
	}
	p.line("const %s = %s;", p.name(v), expr)
}

// ---------------------------------------------------------------- statements

func (p *printer) stmt(s *ir.Stmt) {
	if !p.pl.Live(s) {
		return
	}
	switch s.Op {
	case ir.OConst, ir.OAssume:
	case ir.OGlobal:
		p.define(s.Res[0], emit.JSMangle(s.Name))
	case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg, ir.ODiv, ir.ORem, ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		form := p.pl.Spell[s.Op]
		f := form[0]
		if s.Mode == ir.MTrap {
			f = form[1]
		}
		if f == "" {
			p.fail("the target has no spelling for (%s %s)", s.Op, s.Mode)
			return
		}
		expr := "(" + emit.Fill(f, p.refs(s.Args)...) + ")"
		if !p.pl.Read(s.Res[0]) {
			p.line("%s;", expr) // a trap kept for its effect
			return
		}
		p.define(s.Res[0], expr)
	case ir.OCall:
		p.call(s)
	case ir.OIndex:
		p.define(s.Res[0], fmt.Sprintf("%s[%s]", p.ref(s.Args[0]), p.ref(s.Args[1])))
	case ir.OLen:
		p.define(s.Res[0], p.Len(s.Args[0], p.ref(s.Args[0])))
	case ir.OArray:
		p.define(s.Res[0], "["+strings.Join(p.refs(s.Args), ", ")+"]")
	case ir.OMap:
		rows := make([]string, 0, len(s.Args)/2)
		for j := 0; j+1 < len(s.Args); j += 2 {
			rows = append(rows, p.ref(s.Args[j])+": "+p.ref(s.Args[j+1]))
		}
		p.define(s.Res[0], "{"+strings.Join(rows, ", ")+"}")
	case ir.ORead:
		// A plain object's absent key is `undefined`, which no value of the
		// language is, so `=== undefined` decides the domain exactly
		// (maps-2026-08-30).
		m, k := p.ref(s.Args[0]), p.ref(s.Args[1])
		tag, pay := s.Res[0], s.Res[1]
		pn := p.name(pay)
		if !p.pl.Read(pay) {
			pn = p.fresh("p")
		}
		p.line("const %s = %s[%s];", pn, m, k)
		if p.pl.Read(tag) {
			p.line("const %s = %s === undefined ? 1 : 0;", p.name(tag), pn)
		}
	case ir.OKeys:
		// Object keys are STRINGS, and `sort` without a comparator is
		// lexicographic: both would be wrong answers (maps.md §7).
		p.define(s.Res[0], fmt.Sprintf("Object.keys(%s).map(Number).sort((a, b) => a - b)", p.ref(s.Args[0])))
	case ir.OSet, ir.OInsert:
		p.line("%s[%s] = %s;", p.ref(s.Args[0]), p.ref(s.Args[1]), p.ref(s.Args[2]))
	case ir.ORestrict:
		// A view buys V8 nothing: the restriction is its table (L13 holds of
		// the table itself on every index below n).
		p.pl.Alias[s.Res[0]] = s.Args[0]
	case ir.OIf:
		p.ifOp(s)
	case ir.OLoop:
		p.loop(s)
	case ir.OBuild:
		p.build(s)
	case ir.OBuildMap:
		// A plain object; the capacity is the declaration four targets agree on,
		// and this host has none to give (maps.md §6).
		buf := s.Sub[0].Params[0]
		p.line("const %s = {};", p.name(buf))
		p.body(s, buf)
	case ir.OTabulate:
		p.tabulate(s)
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
		emit.JSImports[q.Import] = true
	}
	args := p.refs(s.Args)
	switch {
	case q.Kind == "stmt":
		p.line("%s;", emit.Fill(q.Form, args...))
	case len(q.Results) >= 2:
		out := make([]string, len(s.Res))
		for i, v := range s.Res {
			out[i] = p.name(v)
		}
		if emit.MultiPrimDests(q.Form, len(q.Results)) {
			// The template assigns into destinations: a host that signals
			// failure out of band, whose `catch` arm writes them.
			p.line("let %s;", strings.Join(out, ", "))
			for _, l := range strings.Split(emit.Fill(emit.FillDests(q.Form, out), args...), "\n") {
				if t := strings.TrimSpace(l); t != "" {
					p.line("%s", t)
				}
			}
			return
		}
		fields := make([]string, len(out))
		for i, n := range out {
			fields[i] = fmt.Sprintf("f%d: %s", i, n)
		}
		p.line("const {%s} = %s;", strings.Join(fields, ", "), emit.Fill(q.Form, args...))
	default:
		expr := "(" + emit.Fill(q.Form, args...) + ")"
		if !p.pl.Read(s.Res[0]) {
			p.line("%s;", expr)
			return
		}
		p.define(s.Res[0], expr)
	}
}

// ---------------------------------------------------------------- regions

func (p *printer) declare(res []ir.V) []string {
	dst := make([]string, len(res))
	for j, v := range res {
		if p.pl.Read(v) {
			dst[j] = p.name(v)
			p.line("let %s;", dst[j])
		}
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
	// A conditional EXPRESSION when both arms are expression trees: the
	// coproduct as a value, with no temporary (the term backend's emitIf).
	if len(s.Res) == 1 {
		if a, ok := p.pl.ExprOf(s.Sub[0], p.ref, p); ok {
			if b, ok := p.pl.ExprOf(s.Sub[1], p.ref, p); ok {
				p.define(s.Res[0], fmt.Sprintf("(%s ? %s : %s)", p.ref(s.Args[0]), a, b))
				return
			}
		}
	}
	dst := p.declare(s.Res)
	p.line("if (%s) {", p.ref(s.Args[0]))
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
	buf := s.Sub[0].Params[0]
	if sp, ok := p.spare[s]; ok {
		// A back-edge build writes into its loop's spare; `fill(0)` restores the
		// zero fill (tables.md §14.3) and keeps the array packed.
		p.line("%s.fill(0);", sp)
		p.line("const %s = %s;", p.name(buf), sp)
	} else {
		p.line("const %s = new Array(%s).fill(0);", p.name(buf), p.ref(s.Args[0]))
	}
	p.body(s, buf)
}

// body prints a scope's region: a scope whose every yield is its buffer IS
// its buffer (ADR 0031's freeze copies nothing).
func (p *printer) body(s *ir.Stmt, buf ir.V) {
	if len(s.Res) == 1 && p.pl.YieldsOnly(s.Sub[0], buf) {
		p.pl.Alias[s.Res[0]] = buf
		p.with(nil, func() { p.region(s.Sub[0], nil, false) })
		return
	}
	dst := p.declare(s.Res)
	p.with(dst, func() { p.region(s.Sub[0], nil, false) })
}

func (p *printer) tabulate(s *ir.Stmt) {
	t := p.name(s.Res[0])
	n := p.fresh("n")
	i := p.name(s.Sub[0].Params[0])
	e := p.fresh("e")
	p.line("const %s = %s;", n, p.ref(s.Args[0]))
	p.line("const %s = new Array(%s).fill(0);", t, n)
	p.line("for (let %s = 0; %s < %s; %s++) {", i, i, n, i)
	p.ind++
	p.line("let %s;", e)
	p.with([]string{e}, func() { p.region(s.Sub[0], nil, false) })
	p.line("%s[%s] = %s;", t, i, e)
	p.ind--
	p.line("}")
}

func (p *printer) loop(s *ir.Stmt) {
	body := s.Sub[0]
	lp := &loopCtx{op: s, post: map[int]bool{}, tail: s == p.tailOp}
	for j, q := range body.Params {
		p.line("let %s = %s;", p.name(q), p.ref(s.Args[j]))
	}
	d := p.pl.DecideLoop(s)
	lp.result = make([]string, len(s.Res))
	if !lp.tail {
		for j, rv := range s.Res {
			if k := d.Coalesced[j]; k >= 0 {
				p.pl.Alias[rv] = body.Params[k]
				continue
			}
			if p.pl.Read(rv) {
				lp.result[j] = p.name(rv)
				p.line("let %s;", lp.result[j])
			}
		}
	}
	var ups []string
	for j, q := range body.Params {
		if step, ok := d.Post[j]; ok {
			lp.post[j] = true
			// Sequential in the post clause is sound: PostVars steps each
			// parameter by a literal, reading no other parameter.
			ups = append(ups, fmt.Sprintf("%s = (%s + %s)", p.name(q), p.name(q), lit(step)))
		}
	}
	for j, q := range body.Params {
		if sb, ok := d.Spare[j]; ok {
			if lp.spare == nil {
				lp.spare = map[int]string{}
			}
			sp := fmt.Sprintf("sp%d", p.pl.Res(q))
			lp.spare[j] = sp
			if p.spare == nil {
				p.spare = map[*ir.Stmt]string{}
			}
			p.spare[sb.Build] = sp
			p.line("let %s = new Array(%d).fill(0);", sp, sb.Len)
		}
	}
	if len(ups) > 0 {
		p.line("for (;; %s) {", strings.Join(ups, ", "))
	} else {
		p.line("for (;;) {")
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

func (p *printer) Index(tab ir.V, t, i string) string { return t + "[" + i + "]" }

// Len is a table's `.length`, and a map's |dom m|: a plain object has no
// length, and `{}.length` is `undefined` (maps.md §8.2).
func (p *printer) Len(v ir.V, t string) string {
	if p.isMap(v) {
		return "Object.keys(" + t + ").length"
	}
	return t + ".length"
}

func (p *printer) Call(s *ir.Stmt, args []string) string {
	q := p.tg.Prims[s.Name]
	if q.Import != "" {
		emit.JSImports[q.Import] = true
	}
	return "(" + emit.Fill(q.Form, args...) + ")"
}

func (p *printer) Or(a, b string) string  { return "(" + a + " || " + b + ")" }
func (p *printer) And(a, b string) string { return "(" + a + " && " + b + ")" }
func (p *printer) Not(c string) string    { return "(!" + c + ")" }

// Cond is JavaScript's conditional expression: the coproduct as a value.
func (p *printer) Cond(c, a, b string) string { return "(" + c + " ? " + a + " : " + b + ")" }

// FromResidual is the IR's whole path to JavaScript for one definition.
func FromResidual(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) (string, error) {
	f, err := ir.Lower(tg, name, sig, nf, ir.Options{Decided: true})
	if err != nil {
		return "", err
	}
	return FromFunc(tg, f)
}

// FromFunc prints a lowered and DECIDED function (ir.Decide): the step to
// IR_P, the verifier, and the printer. It is what the pipeline calls, so the
// modes the IR decided are the ones printed.
func FromFunc(tg *emit.Target, f *ir.Func) (string, error) {
	if err := ir.ToP(tg, f); err != nil {
		return "", err
	}
	return Func(tg, f)
}
