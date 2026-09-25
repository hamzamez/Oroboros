// Package java prints IR_P as Java: a model of Σ in Java (docs/spec/ir.md §9).
// The decisions are ir/plan's; this is the spelling.
//
// JAVA'S OWN DECISION IS ρ ON SCALARS. The JVM indexes arrays with a 32-bit
// `int`, and the language's `int` is 64-bit here, so a value is held as the
// target's `(int -2³¹ 2³¹−1)` representation, `int`, exactly when its IR_P
// range lies in that set, and as `long` otherwise (native-java-2026-08-25:
// removing the `(int)` cast from an index is worth 1.04–1.45×). The flow
// rule's scalar coercions (spec §4.3) then decide every cast:
//   - widening (int → long) is implicit in Java;
//   - narrowing (long → int, int → byte, …) is a cast, sound because the
//     value lies in the smaller set by its range;
//   - an operation is computed in its RESULT's representation: a `long`
//     result of `int` operands widens an operand first (Java would overflow in
//     32 bits), and an `int` result of `long` operands is cast.
//
// Several results are a record shared by shape (product-2026-08-19); a map is a
// boxed HashMap whose keys are cast to the key's type, since an `int` key boxes
// to `Integer` and finds no `Long`.
package java

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

// Func prints one IR_P function as a static method. Imports and records go to
// emit.JavaImports and emit.JavaRecords, which emit.JavaFile reads.
func Func(tg *emit.Target, f *ir.Func) (string, error) {
	p := &printer{tg: tg, f: f, pl: plan.New(tg, f)}
	p.int32 = tg.HostType("int -2147483648 2147483647")
	p.word = tg.HostType("int")
	out, err := p.function()
	if err != nil {
		return "", fmt.Errorf("%s: %w", f.Name, err)
	}
	return out, nil
}

type printer struct {
	tg          *emit.Target
	f           *ir.Func
	pl          *plan.Plan
	b           strings.Builder
	ind         int
	err         error
	tmp         int
	int32, word string // Java's `int` and `long`, read off the target
	yieldTo     []string
	yieldTy     []string
	spare       map[*ir.Stmt]string
	rec         string // the record of a several-results method
	recTys      []string
}

type loopCtx struct {
	op     *ir.Stmt
	post   map[int]bool
	result []string
	resTy  []string
	spare  map[int]string
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

// ---------------------------------------------------------------- types (ρ)

// jty is the Java type holding a value of IR type t: a scalar integer by the
// scalar ρ above, a table by ρ_T of its element, anything else as the target
// spells it.
func (p *printer) jty(t string) string {
	if strings.HasPrefix(t, "buffer ") {
		t = "array " + t[len("buffer "):]
	}
	if strings.HasPrefix(t, "array ") || strings.HasPrefix(t, "map ") {
		return p.tg.HostType(t)
	}
	if t == "int" {
		return p.word
	}
	if lo, hi, ok := core.IntRange(t); ok {
		if lo >= math.MinInt32 && hi <= math.MaxInt32 {
			return p.int32
		}
		return p.word
	}
	if _, _, ok := core.IntRangeBig(t); ok {
		return p.tg.HostType(p.tg.ValueType(t))
	}
	return p.tg.HostType(t)
}

func (p *printer) tyOf(v ir.V) string { return p.jty(p.f.Types[p.pl.Res(v)]) }

// rank orders Java's integral types for widening.
func rank(t string) int {
	switch t {
	case "byte":
		return 1
	case "short":
		return 2
	case "int":
		return 3
	case "long":
		return 4
	case "double":
		return 5
	}
	return 0
}

// coerce is a value of Java type from, flowing where to is required: widening
// is implicit, narrowing is a cast (sound by the value's range).
func coerce(expr, from, to string) string {
	rf, rt := rank(from), rank(to)
	if rf == 0 || rt == 0 || rf <= rt {
		return expr
	}
	return "(" + to + ") (" + expr + ")"
}

func zeroOf(ty string) string {
	switch ty {
	case "double":
		return "0.0"
	case "long", "int", "short", "byte":
		return "0"
	case "boolean":
		return "false"
	}
	return "null"
}

// ---------------------------------------------------------------- names

func (p *printer) name(v ir.V) string { return fmt.Sprintf("v%d", p.pl.Res(v)) }

func (p *printer) ref(v ir.V) string {
	v = p.pl.Res(v)
	if d := p.pl.Const[v]; d != nil {
		return lit(d)
	}
	return p.name(v)
}

// refAs is a value's reference coerced to a Java type.
func (p *printer) refAs(v ir.V, to string) string { return coerce(p.ref(v), p.tyOf(v), to) }

func lit(d *core.Term) string {
	switch d.Kind {
	case core.KInt:
		if d.Int < math.MinInt32 || d.Int > math.MaxInt32 {
			return strconv.FormatInt(d.Int, 10) + "L"
		}
		return strconv.FormatInt(d.Int, 10)
	case core.KFloat:
		switch {
		case math.IsInf(d.Float, 1):
			return "Double.POSITIVE_INFINITY"
		case math.IsInf(d.Float, -1):
			return "Double.NEGATIVE_INFINITY"
		case math.IsNaN(d.Float):
			return "Double.NaN"
		}
		s := strconv.FormatFloat(d.Float, 'g', -1, 64)
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
		return emit.UTF16StringLit(d.Str)
	}
	return d.String()
}

// ---------------------------------------------------------------- the method

func (p *printer) function() (string, error) {
	params := make([]string, len(p.f.Params))
	for i, v := range p.f.Params {
		params[i] = p.tyOf(v) + " " + p.name(v)
	}
	ret := "void"
	switch n := len(p.f.Results); {
	case n == 1:
		ret = p.resultTy(p.f.Results[0])
	case n >= 2:
		p.recTys = make([]string, n)
		for i, r := range p.f.Results {
			p.recTys[i] = p.resultTy(r)
		}
		p.rec = emit.JavaRecordName(p.recTys)
		if _, have := emit.JavaRecords[p.rec]; !have {
			fields := make([]string, n)
			for i, ty := range p.recTys {
				fields[i] = fmt.Sprintf("%s f%d", ty, i)
			}
			emit.JavaRecords[p.rec] = fmt.Sprintf("\tpublic record %s(%s) {}\n", p.rec, strings.Join(fields, ", "))
		}
		ret = p.rec
	}
	p.ind = 1
	p.line("public static %s %s(%s) {", ret, emit.JavaMangle(p.f.Name), strings.Join(params, ", "))
	p.ind = 2
	p.region(p.f.Body, nil, true)
	p.ind = 1
	p.line("}")
	return p.b.String(), p.err
}

// resultTy is a result's Java type: a scalar in the WORD, as the term backend
// declares it (a caller compiled against `long` must still link), a table as
// its representation.
func (p *printer) resultTy(t string) string {
	if t == "int" || core.ArrayElem(t) == "" && p.tg.ValueType(t) == "int" {
		return p.word
	}
	return p.jty(t)
}

func (p *printer) ret(vs []ir.V) {
	switch {
	case len(vs) == 0:
		p.line("return;")
	case p.rec != "":
		out := make([]string, len(vs))
		for i, v := range vs {
			out[i] = p.refAs(v, p.recTys[i])
		}
		p.line("return new %s(%s);", p.rec, strings.Join(out, ", "))
	default:
		p.line("return %s;", p.refAs(vs[0], p.resultTy(p.f.Results[0])))
	}
}

func (p *printer) region(r *ir.Region, lp *loopCtx, top bool) {
	for i := range r.Stmts {
		p.stmt(&r.Stmts[i])
	}
	switch r.T {
	case ir.TYield:
		if top {
			p.ret(r.Args)
			return
		}
		p.move(p.yieldTo, p.yieldTy, r.Args)
	case ir.TContinue:
		p.again(lp, r.Args)
		p.line("continue;")
	case ir.TBreak:
		if lp != nil {
			p.move(lp.result, lp.resTy, r.Args)
		}
		p.line("break;")
	case ir.TBranch:
		p.line("if (%s) {", p.ref(r.Cond))
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

// move is the parallel move dst ← vs, each value coerced to its destination's
// type, sequentialised by ir/plan.
func (p *printer) move(dst, tys []string, vs []ir.V) {
	d := make([]string, len(dst))
	src := make([]string, len(dst))
	for j := range dst {
		if j < len(vs) && dst[j] != "" {
			d[j], src[j] = dst[j], p.refAs(vs[j], tys[j])
		}
	}
	p.moves(d, src, tys)
}

func (p *printer) moves(dst, src, tys []string) {
	tyOf := map[string]string{}
	for j := range dst {
		if dst[j] != "" {
			tyOf[dst[j]] = tys[j]
		}
	}
	temps := map[string]bool{}
	for _, m := range plan.Moves(dst, src, func() string { t := p.fresh("u"); temps[t] = true; return t }) {
		if temps[m[0]] {
			p.line("final %s %s = %s;", tyOf[m[1]], m[0], m[1])
			tyOf[m[0]] = tyOf[m[1]]
		} else {
			p.line("%s = %s;", m[0], m[1])
		}
	}
}

func (p *printer) again(lp *loopCtx, vs []ir.V) {
	var dst, src, tys []string
	params := lp.op.Sub[0].Params
	for j, v := range vs {
		if lp.post[j] {
			continue
		}
		q := p.name(params[j])
		t := p.tyOf(params[j])
		dst, src, tys = append(dst, q), append(src, p.refAs(v, t)), append(tys, t)
		if sp, ok := lp.spare[j]; ok {
			dst, src, tys = append(dst, sp), append(src, q), append(tys, t)
		}
	}
	p.moves(dst, src, tys)
}

func (p *printer) define(v ir.V, expr string) {
	if !p.pl.Read(v) {
		return
	}
	p.line("final %s %s = %s;", p.tyOf(v), p.name(v), expr)
}

// ---------------------------------------------------------------- statements

// arith spells an integer operation computed in its result's representation.
func (p *printer) arith(s *ir.Stmt, form string) string {
	res := p.tyOf(s.Res[0])
	args := make([]string, len(s.Args))
	promoted := p.int32
	for i, a := range s.Args {
		args[i] = p.ref(a)
		if rank(p.tyOf(a)) > rank(promoted) {
			promoted = p.tyOf(a)
		}
	}
	if s.Op.IsCmp() {
		return "(" + emit.Fill(form, args...) + ")"
	}
	if rank(res) > rank(promoted) && len(args) > 0 {
		args[0] = "((" + res + ") " + args[0] + ")" // widen first: Java would overflow in 32 bits
	}
	e := "(" + emit.Fill(form, args...) + ")"
	return coerce(e, promoted, res)
}

func (p *printer) stmt(s *ir.Stmt) {
	if !p.pl.Live(s) {
		return
	}
	switch s.Op {
	case ir.OConst, ir.OAssume:
	case ir.OGlobal:
		p.define(s.Res[0], emit.JavaMangle(s.Name))
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
		e := p.arith(s, f)
		if !p.pl.Read(s.Res[0]) {
			p.line("final var %s = %s;", p.fresh("discard"), e) // a trap kept for its effect
			return
		}
		p.define(s.Res[0], e)
	case ir.OCall:
		p.call(s)
	case ir.OIndex:
		p.define(s.Res[0], coerce(p.index(s.Args[0], s.Args[1]), p.elemTy(s.Args[0]), p.tyOf(s.Res[0])))
	case ir.OLen:
		p.define(s.Res[0], p.Len(s.Args[0], p.ref(s.Args[0])))
	case ir.OArray:
		e := p.elemTy(s.Res[0])
		out := make([]string, len(s.Args))
		for i, a := range s.Args {
			if d := p.pl.Const[p.pl.Res(a)]; d != nil {
				out[i] = lit(d) // a constant narrows in an array initializer
			} else {
				out[i] = p.refAs(a, e)
			}
		}
		p.define(s.Res[0], fmt.Sprintf("new %s{%s}", p.tyOf(s.Res[0]), strings.Join(out, ", ")))
	case ir.OMap:
		rows := make([]string, 0, len(s.Args)/2)
		for j := 0; j+1 < len(s.Args); j += 2 {
			rows = append(rows, fmt.Sprintf("java.util.Map.entry(%s, %s)", p.ref(s.Args[j]), p.ref(s.Args[j+1])))
		}
		// A literal is immutable, which Map.ofEntries gives.
		p.define(s.Res[0], "java.util.Map.ofEntries("+strings.Join(rows, ", ")+")")
	case ir.ORead:
		p.read(s)
	case ir.OKeys:
		p.define(s.Res[0], fmt.Sprintf("%s.keySet().stream().sorted().mapToLong(Long::longValue).toArray()", p.ref(s.Args[0])))
	case ir.OSet:
		p.line("%s = %s;", p.index(s.Args[0], s.Args[1]), p.refAs(s.Args[2], p.elemTy(s.Args[0])))
	case ir.OInsert:
		kt, vt := p.mapTys(s.Args[0])
		p.line("%s.put((%s) %s, (%s) %s);", p.ref(s.Args[0]), kt, p.ref(s.Args[1]), vt, p.ref(s.Args[2]))
	case ir.ORestrict:
		p.pl.Alias[s.Res[0]] = s.Args[0] // a view buys the JVM nothing (L13)
	case ir.OIf:
		p.ifOp(s)
	case ir.OLoop:
		p.loop(s)
	case ir.OBuild:
		p.build(s)
	case ir.OBuildMap:
		buf := s.Sub[0].Params[0]
		p.line("final %s %s = new java.util.HashMap<>((int) %s);", p.tyOf(buf), p.name(buf), p.ref(s.Args[0]))
		p.body(s, buf)
	case ir.OTabulate:
		p.tabulate(s)
	case ir.OThe, ir.ORequire:
		p.fail("(%s …) in IR_P", s.Op)
	}
}

// index is `t[i]`, with the index as Java's `int`: a cast where the index is
// held as `long`, sound because an index is below the length.
func (p *printer) index(tab, i ir.V) string {
	return p.ref(tab) + "[" + p.asInt32(i) + "]"
}

// asInt32 is a length or an index as Java's `int`: a cast where it is held as
// `long`. For an index it is sound, since an index is below a length and a
// length below 2³¹ (tables.md §2.3.1). For a build's size it relies on that
// same bound, which nothing yet obliges a size to meet. Both printers wrap
// there, as `(len (build b 4294967297 …))` = 1 shows.
func (p *printer) asInt32(x ir.V) string {
	if p.tyOf(x) != p.int32 {
		return "(int) " + p.ref(x)
	}
	return p.ref(x)
}

// elemTy is a table's element's Java type.
func (p *printer) elemTy(tab ir.V) string {
	t := p.f.Types[p.pl.Res(tab)]
	if strings.HasPrefix(t, "buffer ") {
		t = "array " + t[len("buffer "):]
	}
	if e := ir.ElemOf(p.tg, t); e != "" {
		return p.tg.HostType(e)
	}
	return ""
}

func (p *printer) mapTys(m ir.V) (string, string) {
	if k, v, ok := core.MapTypes(p.f.Types[p.pl.Res(m)]); ok {
		return p.tg.HostType(k), p.tg.HostType(v)
	}
	return p.word, p.word
}

func (p *printer) read(s *ir.Stmt) {
	kt, vt := p.mapTys(s.Args[0])
	_, vName, _ := core.MapTypes(p.f.Types[p.pl.Res(s.Args[0])])
	box := p.fresh("b")
	p.line("final %s %s = %s.get((%s) %s);", p.tg.BoxedType(vName), box, p.ref(s.Args[0]), kt, p.ref(s.Args[1]))
	if p.pl.Read(s.Res[0]) {
		p.line("final %s %s = %s == null ? 1 : 0;", p.tyOf(s.Res[0]), p.name(s.Res[0]), box)
	}
	if p.pl.Read(s.Res[1]) {
		p.line("final %s %s = %s == null ? %s : %s;", p.tyOf(s.Res[1]), p.name(s.Res[1]), box, zeroOf(vt), coerce(box, vt, p.tyOf(s.Res[1])))
	}
}

func (p *printer) call(s *ir.Stmt) {
	q, ok := p.tg.Prims[s.Name]
	if !ok {
		p.fail("%s is not a primitive of target %s", s.Name, p.tg.Name)
		return
	}
	if q.Import != "" {
		emit.JavaImports[q.Import] = true
	}
	args := make([]string, len(s.Args))
	for i, a := range s.Args {
		args[i] = p.ref(a)
		if i < len(q.Args) && p.tg.ValueType(q.Args[i]) == "int" {
			args[i] = coerce(args[i], p.tyOf(a), p.jty(q.Args[i]))
		}
	}
	expr := emit.Fill(q.Form, args...)
	switch {
	case q.Kind == "stmt":
		p.line("%s;", expr)
	case len(q.Results) >= 2:
		out := make([]string, len(s.Res))
		for i, v := range s.Res {
			out[i] = p.name(v)
		}
		if emit.MultiPrimDests(q.Form, len(q.Results)) {
			for i, n := range out {
				p.line("%s %s;", p.jty(q.Results[i]), n)
			}
			for _, l := range strings.Split(emit.FillDests(q.Form, out), "\n") {
				if t := strings.TrimSpace(emit.Fill(l, args...)); t != "" {
					p.line("%s", t)
				}
			}
			return
		}
		tmp := p.fresh("res")
		p.line("final var %s = %s;", tmp, expr)
		for i, v := range s.Res {
			if p.pl.Read(v) {
				p.line("final %s %s = %s.f%d();", p.tyOf(v), out[i], tmp, i)
			}
		}
	default:
		e := "(" + expr + ")"
		if p.tg.ValueType(q.Result) == "int" && p.tyOf(s.Res[0]) == p.int32 {
			e = "(" + p.int32 + ") " + e // the host's integer, held as Java's int by its range
		}
		if !p.pl.Read(s.Res[0]) {
			p.line("%s;", expr)
			return
		}
		p.define(s.Res[0], e)
	}
}

// ---------------------------------------------------------------- regions

func (p *printer) declare(res []ir.V) ([]string, []string) {
	dst := make([]string, len(res))
	tys := make([]string, len(res))
	for j, v := range res {
		tys[j] = p.tyOf(v)
		if p.pl.Read(v) {
			dst[j] = p.name(v)
			p.line("%s %s = %s;", tys[j], dst[j], zeroOf(tys[j]))
		}
	}
	return dst, tys
}

func (p *printer) with(dst, tys []string, f func()) {
	od, ot := p.yieldTo, p.yieldTy
	p.yieldTo, p.yieldTy = dst, tys
	f()
	p.yieldTo, p.yieldTy = od, ot
}

func (p *printer) ifOp(s *ir.Stmt) {
	if e, ok := p.pl.Connective(s, p.ref, p); ok {
		p.define(s.Res[0], e)
		return
	}
	dst, tys := p.declare(s.Res)
	p.line("if (%s) {", p.ref(s.Args[0]))
	p.ind++
	p.with(dst, tys, func() { p.region(s.Sub[0], nil, false) })
	p.ind--
	p.line("} else {")
	p.ind++
	p.with(dst, tys, func() { p.region(s.Sub[1], nil, false) })
	p.ind--
	p.line("}")
}

func (p *printer) build(s *ir.Stmt) {
	buf := s.Sub[0].Params[0]
	ty := p.tyOf(buf)
	if sp, ok := p.spare[s]; ok {
		// A back-edge build writes into its loop's spare; the zero needs the
		// element's own type, since a constant narrows in an assignment and not
		// in a method call (`Arrays.fill(byteArray, 0)` does not compile).
		zero := "false"
		if e := p.elemTy(buf); e != "boolean" {
			zero = "(" + e + ") 0"
		}
		p.line("java.util.Arrays.fill(%s, %s);", sp, zero)
		p.line("final %s %s = %s;", ty, p.name(buf), sp)
	} else {
		p.line("final %s %s = new %s[%s];", ty, p.name(buf), p.elemTy(buf), p.asInt32(s.Args[0]))
	}
	p.body(s, buf)
}

func (p *printer) body(s *ir.Stmt, buf ir.V) {
	if len(s.Res) == 1 && p.pl.YieldsOnly(s.Sub[0], buf) {
		p.pl.Alias[s.Res[0]] = buf
		p.with(nil, nil, func() { p.region(s.Sub[0], nil, false) })
		return
	}
	dst, tys := p.declare(s.Res)
	p.with(dst, tys, func() { p.region(s.Sub[0], nil, false) })
}

// tabulate fills a table by its rule. The fill index counts to n, which is
// Java's `int` (asInt32).
func (p *printer) tabulate(s *ir.Stmt) {
	t := p.name(s.Res[0])
	n := p.fresh("n")
	i := p.name(s.Sub[0].Params[0])
	e := p.fresh("e")
	elem := p.elemTy(s.Res[0])
	var ety string
	if y := plan.FirstYield(s.Sub[0]); len(y) == 1 {
		ety = p.tyOf(y[0])
	}
	p.line("final int %s = %s;", n, p.asInt32(s.Args[0]))
	p.line("final %s %s = new %s[%s];", p.tyOf(s.Res[0]), t, elem, n)
	p.line("for (%s %s = 0; %s < %s; %s++) {", p.tyOf(s.Sub[0].Params[0]), i, i, n, i)
	p.ind++
	p.line("%s %s = %s;", ety, e, zeroOf(ety))
	p.with([]string{e}, []string{ety}, func() { p.region(s.Sub[0], nil, false) })
	p.line("%s[%s] = %s;", t, i, coerce(e, ety, elem))
	p.ind--
	p.line("}")
}

func (p *printer) loop(s *ir.Stmt) {
	body := s.Sub[0]
	lp := &loopCtx{op: s, post: map[int]bool{}}
	for j, q := range body.Params {
		p.line("%s %s = %s;", p.tyOf(q), p.name(q), p.refAs(s.Args[j], p.tyOf(q)))
	}
	d := p.pl.DecideLoop(s)
	lp.result = make([]string, len(s.Res))
	lp.resTy = make([]string, len(s.Res))
	for j, rv := range s.Res {
		lp.resTy[j] = p.tyOf(rv)
		if k := d.Coalesced[j]; k >= 0 && p.tyOf(body.Params[k]) == p.tyOf(rv) {
			p.pl.Alias[rv] = body.Params[k]
			continue
		}
		if p.pl.Read(rv) {
			lp.result[j] = p.name(rv)
			p.line("%s %s = %s;", lp.resTy[j], lp.result[j], zeroOf(lp.resTy[j]))
		}
	}
	var ups []string
	for j, q := range body.Params {
		if step, ok := d.Post[j]; ok {
			lp.post[j] = true
			// The step is computed in the parameter's own representation: its
			// range, which includes every stepped value, proves no overflow.
			stepTy := p.int32
			if step.Int < math.MinInt32 || step.Int > math.MaxInt32 || p.tyOf(q) == p.word {
				stepTy = p.word
			}
			ups = append(ups, fmt.Sprintf("%s = %s", p.name(q), coerce(fmt.Sprintf("(%s + %s)", p.name(q), lit(step)), stepTy, p.tyOf(q))))
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
			p.line("%s %s = new %s[%d];", p.tyOf(q), sp, p.elemTy(q), sb.Len)
		}
	}
	if len(ups) > 0 {
		p.line("for (;; %s) {", strings.Join(ups, ", "))
	} else {
		p.line("for (;;) {")
	}
	p.ind++
	od, ot := p.yieldTo, p.yieldTy
	p.yieldTo, p.yieldTy = nil, nil
	p.region(body, lp, false)
	p.yieldTo, p.yieldTy = od, ot
	p.ind--
	p.line("}")
}

// ---------------------------------------------------------------- the Speller (ir/plan)

func (p *printer) Op(s *ir.Stmt, args []string) string {
	f := p.pl.Spell[s.Op][0]
	if f == "" {
		return ""
	}
	if !s.Op.IsCmp() {
		return "" // arithmetic inside an expression tree would lose its coercion
	}
	return "(" + emit.Fill(f, args...) + ")"
}

func (p *printer) Index(tab ir.V, t, i string) string { return "" }

func (p *printer) Len(v ir.V, t string) string {
	if strings.HasPrefix(p.f.Types[p.pl.Res(v)], "map ") {
		return t + ".size()"
	}
	return t + ".length"
}

func (p *printer) Call(s *ir.Stmt, args []string) string { return "" }

func (p *printer) Or(a, b string) string      { return "(" + a + " || " + b + ")" }
func (p *printer) And(a, b string) string     { return "(" + a + " && " + b + ")" }
func (p *printer) Cond(c, a, b string) string { return "" }

// FromResidual is the IR's whole path to Java for one definition.
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
