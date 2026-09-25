package ir

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"oroboros/core"
)

// THE CANONICAL PRINTING (spec §10.1). One program has one text:
//   - values are renumbered in DEFINITION ORDER, the order a reader meets their
//     definitions, so the text does not depend on how lowering allocated ids;
//   - one statement per line, regions indented two spaces per level;
//   - globals and functions sorted by name.
// print ∘ read ∘ print = print is pinned by the round-trip test.

// Print is the canonical text of a program.
func Print(p *Program) string {
	var b strings.Builder
	fmt.Fprintf(&b, "(ir %d\n  (target %s)\n  (stage %s)\n  (ops%s)", Version, p.Target, p.Stage, lead(p.Ops()))
	gs := append([]Global(nil), p.Globals...)
	sort.Slice(gs, func(i, j int) bool { return gs[i].Name < gs[j].Name })
	for _, g := range gs {
		fmt.Fprintf(&b, "\n  (global %s %s %s)", g.Name, TypeText(g.Type), litText(g.Lit))
	}
	fs := append([]*Func(nil), p.Funcs...)
	sort.Slice(fs, func(i, j int) bool { return fs[i].Name < fs[j].Name })
	for _, f := range fs {
		b.WriteString("\n")
		b.WriteString(indentLines(funcLines(f), "  "))
	}
	b.WriteString(")\n")
	return b.String()
}

type printer struct {
	f   *Func
	num []int // canonical number of each value, -1 until met
	n   int
}

// number assigns canonical numbers in definition order.
func (pr *printer) number() {
	pr.num = make([]int, pr.f.NV())
	for i := range pr.num {
		pr.num[i] = -1
	}
	def := func(v V) {
		if v >= 0 && int(v) < len(pr.num) && pr.num[v] < 0 {
			pr.num[v] = pr.n
			pr.n++
		}
	}
	var region func(r *Region)
	region = func(r *Region) {
		for _, v := range r.Params {
			def(v)
		}
		for _, pi := range r.Pis {
			def(pi.V)
		}
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def(v)
			}
			for _, s := range r.Stmts[i].Sub {
				region(s)
			}
		}
		if r.T == TBranch {
			region(r.Then)
			region(r.Else)
		}
	}
	for _, v := range pr.f.Params {
		def(v)
	}
	region(pr.f.Body)
}

func (pr *printer) v(x V) string {
	if x < 0 || int(x) >= len(pr.num) || pr.num[x] < 0 {
		return fmt.Sprintf("%%?%d", x) // an undefined value: the verifier names it (W1, W2)
	}
	return "%" + strconv.Itoa(pr.num[x])
}

func (pr *printer) vs(xs []V) string {
	s := make([]string, len(xs))
	for i, x := range xs {
		s[i] = pr.v(x)
	}
	return strings.Join(s, " ")
}

func (pr *printer) param(x V) string {
	ty := ""
	if x >= 0 && int(x) < len(pr.f.Types) {
		ty = pr.f.Types[x]
	}
	return "(" + pr.v(x) + " " + TypeText(ty) + ")"
}

func funcLines(f *Func) []string {
	pr := &printer{f: f}
	pr.number()
	params := make([]string, len(f.Params))
	for i, p := range f.Params {
		params[i] = pr.param(p)
	}
	results := make([]string, len(f.Results))
	for i, r := range f.Results {
		results[i] = TypeText(r)
	}
	head := fmt.Sprintf("(func %s (params%s) (results%s)", f.Name, lead(params), lead(results))
	return wrap(head, pr.region(f.Body))
}

// lead joins with a leading space, so an empty list prints as `(params)`.
func lead(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return " " + strings.Join(xs, " ")
}

// wrap puts a head line over indented body lines and closes the form on the
// last line.
func wrap(head string, body []string) []string {
	out := []string{head}
	for _, l := range body {
		out = append(out, "  "+l)
	}
	out[len(out)-1] += ")"
	return out
}

func indentLines(ls []string, in string) string {
	return in + strings.Join(ls, "\n"+in)
}

func (pr *printer) region(r *Region) []string {
	var body []string
	if len(r.Params) > 0 {
		ps := make([]string, len(r.Params))
		for i, p := range r.Params {
			ps[i] = pr.param(p)
		}
		body = append(body, "(params "+strings.Join(ps, " ")+")")
	}
	for _, pi := range r.Pis {
		of := pr.v(pi.Of)
		if pi.Len {
			of = "(len " + of + ")"
		}
		body = append(body, fmt.Sprintf("(pi %s %s (%s %s %s))", pr.v(pi.V), TypeText(pr.f.Types[pi.V]), of, pi.Rel, pr.v(pi.Other)))
	}
	for i := range r.Stmts {
		body = append(body, pr.stmt(&r.Stmts[i])...)
	}
	switch r.T {
	case TBranch:
		body = append(body, wrap("(branch "+pr.v(r.Cond), append(pr.region(r.Then), pr.region(r.Else)...))...)
	default:
		body = append(body, "("+r.T.String()+lead([]string{pr.vs(r.Args)})+")")
		if len(r.Args) == 0 {
			body[len(body)-1] = "(" + r.T.String() + ")"
		}
	}
	return wrap("(region", body)
}

func (pr *printer) stmt(s *Stmt) []string {
	op := pr.op(s)
	if len(s.Res) == 0 {
		if len(op) == 1 {
			return []string{"(do " + op[0] + ")"}
		}
		return wrap("(do", op)
	}
	rs := make([]string, len(s.Res))
	for i, v := range s.Res {
		rs[i] = pr.param(v)
	}
	head := "(val " + strings.Join(rs, " ")
	if len(op) == 1 {
		return []string{head + " " + op[0] + ")"}
	}
	return wrap(head, op)
}

// op is an operation's text: one line, or several when it owns regions.
func (pr *printer) op(s *Stmt) []string {
	name := s.Op.String()
	switch s.Op {
	case OConst:
		return []string{"(const " + litText(s.Lit) + ")"}
	case OGlobal:
		return []string{"(global " + s.Name + ")"}
	case OCall:
		return []string{"(call " + s.Name + lead([]string{pr.vs(s.Args)}) + ")"}
	case OAdd, OSub, OMul, ONeg, ODiv, ORem:
		m := ""
		if s.Mode == MExact || s.Mode == MTrap {
			m = " " + s.Mode.String()
		}
		return []string{"(" + name + m + " " + pr.vs(s.Args) + ")"}
	case OMap:
		rows := make([]string, 0, len(s.Args)/2)
		for j := 0; j+1 < len(s.Args); j += 2 {
			rows = append(rows, "("+pr.v(s.Args[j])+" "+pr.v(s.Args[j+1])+")")
		}
		return []string{"(map" + lead(rows) + ")"}
	case OThe:
		return []string{"(the " + TypeText(s.Type) + " " + pr.vs(s.Args) + ")"}
	case OIf:
		return wrap("(if "+pr.vs(s.Args), append(pr.region(s.Sub[0]), pr.region(s.Sub[1])...))
	case OLoop:
		return wrap("(loop (init"+lead([]string{pr.vs(s.Args)})+")", pr.region(s.Sub[0]))
	case OBuild, OBuildMap, OTabulate:
		return wrap("("+name+" "+pr.vs(s.Args), pr.region(s.Sub[0]))
	}
	if len(s.Args) == 0 {
		return []string{"(" + name + ")"}
	}
	return []string{"(" + name + " " + pr.vs(s.Args) + ")"}
}

// litText is a literal as the language's reader reads it.
func litText(t *core.Term) string {
	switch t.Kind {
	case core.KInt:
		return strconv.FormatInt(t.Int, 10)
	case core.KFloat:
		f := t.Float
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return fmt.Sprintf("%v", f) // not a literal the reader has; the verifier refuses none, the round trip would
		}
		s := strconv.FormatFloat(f, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s
	case core.KBool:
		if t.IsTrue() {
			return "true"
		}
		return "false"
	case core.KStr:
		return quote(t.Str)
	}
	return t.String()
}

// quote writes a string literal in the language's notation (string-literals.md):
// six escapes, each forced, and every other scalar as itself.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\t':
			b.WriteString(`\t`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f || !strconv.IsPrint(r) {
				fmt.Fprintf(&b, `\u{%X}`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Canonicalize renumbers f's values in definition order, the printer's
// numbering, so an id in a verifier's message is the id in the text. Lowering
// ends with it; a renaming changes no meaning (the semantics is invariant under
// α).
func Canonicalize(f *Func) {
	pr := &printer{f: f}
	pr.number()
	m := func(v V) V {
		if v >= 0 && int(v) < len(pr.num) && pr.num[v] >= 0 {
			return V(pr.num[v])
		}
		return v
	}
	// Fresh slices: lowering may share one slice between two places (a leaf's
	// values are an operation's results), and mapping it in place would rename
	// those values twice.
	ms := func(vs []V) []V {
		if vs == nil {
			return nil
		}
		out := make([]V, len(vs))
		for i, v := range vs {
			out[i] = m(v)
		}
		return out
	}
	types := make([]string, pr.n)
	for v, n := range pr.num {
		if n >= 0 {
			types[n] = f.Types[v]
		}
	}
	f.Params = ms(f.Params)
	f.Walk(func(r *Region) {
		r.Params = ms(r.Params)
		for i := range r.Pis {
			pi := &r.Pis[i]
			pi.V, pi.Of, pi.Other = m(pi.V), m(pi.Of), m(pi.Other)
		}
		for i := range r.Stmts {
			r.Stmts[i].Args = ms(r.Stmts[i].Args)
			r.Stmts[i].Res = ms(r.Stmts[i].Res)
		}
		r.Args = ms(r.Args)
		if r.T == TBranch {
			r.Cond = m(r.Cond)
		}
	})
	f.Types = types
}
