package j

import (
	"fmt"
	"strings"

	"oroboros/core"
)

// print_J ON THE COMPUTATION'S TREES (spec/j.md §2.6–2.8, §3). One clause per
// operator of abstract.md's Σ, as core/abs.go builds it from today's notation.
// Where today's notation writes a synonym J does not have, the clause writes J's
// one form, and the two have one D-image (core/desugar.go), so meaning is kept:
//   - a continuation ((f a) (fn (q r) e)) is the binding (q, r) = f(a) (ADR 0066
//     item 8): D of a tuple let is exactly that application;
//   - a λ applied where it is written, ((fn (x) e) a), is the binding x = a,
//     which D of a let is;
//   - a match's patterns are guards over the loop's variables, and its name
//     patterns a renaming (match.md §2, as D builds it);
//   - a constructor is a tag, `'ok(v)`, `result.'ok(v)`.

// Precedence, highest first (spec/j.md §3), as numbers: higher binds tighter.
const (
	pLowest = iota
	pIff
	pImp
	pOr
	pAnd
	pCmp
	pSet
	pProd
	pRange
	pAdd
	pMul
	pPrefix
	pPow
	pPost
	pAtom
)

var infix = map[string]struct {
	op   string
	prec int
}{
	"+": {"+", pAdd}, "-": {"-", pAdd}, "*": {"*", pMul}, "/": {"/", pMul}, "%": {"%", pMul},
	"<": {"<", pCmp}, "<=": {"<=", pCmp}, ">": {">", pCmp}, ">=": {">=", pCmp}, "=": {"==", pCmp},
	"concat": {"++", pAdd},
}

// printer holds what a file's printing needs: the gaps not yet printed, the
// spans that place them, the constructors and sets in scope, and the names
// known to hold data (indexing, spec/j.md §2.7).
type printer struct {
	src   string
	spans map[any]core.Span
	gaps  []core.Gap
	next  int // the first gap not yet printed

	ctors map[string]bool // constructor spellings in scope: "ok", "result.ok", "os.not-found"
	sets  map[string]bool // variant names in scope, bare or qualified: spelled as sets

	data        map[string]bool // names known to hold a table or a map, in the current definition
	locals      map[string]bool // the current definition's variables
	fns         map[string]bool // the current definition's names known to hold a function
	argKinds    map[string][]evid
	returnsData map[string]bool          // the file's definitions whose value is a table or a map
	rawSpans    map[*core.Term]core.Span // a target file's datums' spans (target.go)
	resultName  string                   // what a raw `ensures` calls `result`
	root        string                   // the repository, for a target fragment's imports // what each parameter of the file's definitions receives (data.go)
	fresh       int

	// Uncertain counts the one-argument applications of a local name the
	// printer could not show to be data or a function, printed as applications.
	Uncertain      int
	UncertainNames []string
}

// ---- gaps ----

// leading prints, before an item that begins at start, every gap before it.
func (p *printer) leading(start int) *Doc {
	var ds []*Doc
	for p.next < len(p.gaps) && p.gaps[p.next].Start < start {
		ds = append(ds, comment(p.gaps[p.next]), Hard())
		p.next++
	}
	return Cat(ds...)
}

// trailing prints the gaps that trail code on the line an item ends on.
func (p *printer) trailing(end int) *Doc {
	var ds []*Doc
	for p.next < len(p.gaps) {
		g := p.gaps[p.next]
		if g.OwnLine || g.Start < end || strings.Contains(p.src[end:g.Start], "\n") {
			break
		}
		ds = append(ds, Text("  "), comment(g))
		p.next++
	}
	if len(ds) == 0 {
		return nil
	}
	return Cat(append(ds, Force())...)
}

func comment(g core.Gap) *Doc { return Text("//" + strings.TrimRight(g.Text, " \t\r")) }

// item prints a line-start item, a statement, an arm or a declaration, with the
// gaps before it and after it.
//
// The leading gaps are printed before the item is built, so that a gap above an
// item is never taken by the first item inside it.
func (p *printer) item(node any, build func() *Doc) *Doc {
	sp, ok := p.spans[node]
	if !ok {
		return build()
	}
	lead := p.leading(sp.Start)
	d := build()
	return Cat(lead, d, p.trailing(sp.End))
}

// closing prints, before a container's closing brace, the gaps still inside it.
func (p *printer) closing(node any) *Doc {
	sp, ok := p.spans[node]
	if !ok {
		return nil
	}
	var ds []*Doc
	for p.next < len(p.gaps) && p.gaps[p.next].Start < sp.End {
		ds = append(ds, Hard(), comment(p.gaps[p.next]))
		p.next++
	}
	return Cat(ds...)
}

// ---- names ----

func (p *printer) name(n string) *Doc {
	if p.ctors[n] {
		return Text(tag(n))
	}
	return Text(Spell(n))
}

// tag spells a constructor as a tag: `'ok`, `result.'ok`.
func tag(n string) string {
	if i := strings.LastIndex(n, "."); i > 0 {
		return Spell(n[:i]) + ".'" + word(n[i+1:], true)
	}
	return "'" + word(n, true)
}

// ---- expressions ----

func (p *printer) expr(e core.Expr) *Doc { return p.prec(e, pLowest) }

func paren(d *Doc, inner, outer int) *Doc {
	if inner < outer {
		return Cat(Text("("), d, Text(")"))
	}
	return d
}

func (p *printer) prec(e core.Expr, outer int) *Doc {
	switch e := e.(type) {
	case *core.Var:
		return p.name(e.Name)
	case *core.Lit:
		return Text(p.lit(e))
	case *core.EmptyList:
		return Text("()")
	case *core.Lam:
		for _, x := range e.Params {
			p.locals[x] = true
		}
		return paren(Cat(Text("fn("), p.params(e.Params), Text(") "), p.block(e.Body)), pPost, outer)
	case *core.Apply:
		return p.apply(e, outer)
	case *core.Index:
		return Cat(p.prec(e.Table, pPost), Text("["), p.expr(e.At), Text("]"))
	case *core.And:
		return paren(Join(Text(" && "), p.each(chainAnd(e.Xs), pAnd+1)), pAnd, outer)
	case *core.Or:
		return paren(Join(Text(" || "), p.each(chainOr(e.Xs), pOr+1)), pOr, outer)
	case *core.Not:
		return paren(Cat(Text("!"), p.prec(e.X, pPrefix)), pPrefix, outer)
	case *core.If:
		return p.ifExpr(e)
	case *core.Cond:
		var arms []*Doc
		for _, a := range e.Arms {
			if a.Guard == nil {
				arms = append(arms, Cat(Text("else -> "), p.expr(a.Body)))
				continue
			}
			arms = append(arms, Cat(p.expr(a.Guard), Text(" -> "), p.expr(a.Body)))
		}
		return p.braced(Text("case "), arms, e)
	case *core.Case:
		return p.caseExpr(e)
	case *core.Loop:
		return p.loop(e)
	case *core.The:
		// `the` leaves the surface (ADR 0062 item 7): a claim on a value is a
		// binder's set, so the value is bound under it and given back.
		v := p.freshName()
		return Group(Cat(Text("{ "+v+" ∈ "), p.set(e.Set), Text(" = "), p.expr(e.E), Text("; "+v+" }")))
	case *core.Let, *core.Seq, *core.Local, *core.BuildMap, *core.Monad:
		return p.block(e)
	}
	panic(fmt.Sprintf("j: no printing for %T", e))
}

func (p *printer) each(es []core.Expr, prec int) []*Doc {
	out := make([]*Doc, len(es))
	for i, e := range es {
		out[i] = p.prec(e, prec)
	}
	return out
}

func (p *printer) args(es []core.Expr) *Doc {
	return Cat(Text("("), Nest(4, Fill(",", p.each(es, pLowest))), Text(")"))
}

func (p *printer) params(ps []string) *Doc {
	ds := make([]*Doc, len(ps))
	for i, n := range ps {
		ds[i] = Text(Spell(n))
	}
	return Join(Text(", "), ds)
}

func (p *printer) lit(l *core.Lit) string {
	if sp, ok := p.spans[core.Tail(l)]; ok {
		text := p.src[sp.Start:sp.End]
		if strings.Trim(text, "-0123456789") == "" {
			return Integer(text)
		}
		return text
	}
	switch l.T.Kind {
	case core.KBool:
		if l.T.IsTrue() {
			return "true"
		}
		return "false"
	}
	return l.T.String()
}

func (p *printer) freshName() string {
	p.fresh++
	return fmt.Sprintf("v%d_", p.fresh)
}

func (p *printer) apply(e *core.Apply, outer int) *Doc {
	switch fn := e.Fn.(type) {
	case *core.Var:
		n := fn.Name
		if op, ok := infix[n]; ok && len(e.Args) == 2 {
			l := p.prec(e.Args[0], op.prec)
			r := p.prec(e.Args[1], op.prec+1)
			return paren(Group(Cat(l, Text(" "+op.op), Line(), r)), op.prec, outer)
		}
		switch {
		case n == "-" && len(e.Args) == 1:
			return paren(Cat(Text("-"), p.prec(e.Args[0], pPrefix)), pPrefix, outer)
		case n == "tuple":
			if len(e.Args) == 0 {
				return Text("()")
			}
			return p.args(e.Args)
		case n == "array":
			return Group(Cat(Text("array {"), Nest(4, Cat(Line(), Join(Cat(Text(","), Line()), p.each(e.Args, pLowest)))), Line(), Text("}")))
		case n == "table" && len(e.Args) == 2:
			return p.table(e.Args[0], e.Args[1])
		case n == "out" && len(e.Args) == 1:
			// An out-cell, a fresh cell at an out parameter (spec/j.md G1).
			return Cat(Text("out "), p.set(e.Args[0]))
		case n == "spread" && len(e.Args) == 1:
			// A table as a variadic call's list (spec/j.md G2).
			return Cat(p.prec(e.Args[0], pPost), Text("..."))
		case n == "set" && len(e.Args) == 2:
			// A cell's write (local.md §3), a statement whose value is the unit.
			return Cat(p.name(e.Args[0].(*core.Var).Name), Text(" := "), p.expr(e.Args[1]))
		case n == "set" && len(e.Args) == 3:
			// The threaded store, today's (ADR 0052 is not built): a call.
			return Cat(Text("store"), p.args(e.Args))
		case len(e.Args) == 1 && p.data[n]:
			return Cat(p.name(n), Text("["), p.expr(e.Args[0]), Text("]"))
		case len(e.Args) == 1 && p.local(n) && !p.ctors[n] && !p.fns[n] && !isLam(e.Args[0]):
			p.Uncertain++
			p.UncertainNames = append(p.UncertainNames, n)
		}
		return Cat(p.name(n), p.args(e.Args))
	case *core.Apply:
		if len(e.Args) == 1 {
			if _, ok := e.Args[0].(*core.Lam); ok {
				return p.block(e) // a continuation: a binding
			}
			if len(fn.Args) == 0 || p.isRead(fn) {
				// A call's table, or a table's element, indexed: `os.Args()[1]`,
				// `sp[w][0]`, a record's component (data.md §3: a tuple is a
				// function on Fin n, and data).
				return Cat(p.prec(fn, pPost), Text("["), p.expr(e.Args[0]), Text("]"))
			}
		}
		return Cat(p.prec(fn, pPost), p.args(e.Args))
	case *core.Lam:
		if len(e.Args) == len(fn.Params) {
			return p.block(e) // a λ applied where it stands: a binding
		}
	}
	return Cat(p.prec(e.Fn, pPost), p.args(e.Args))
}

// local reports whether a name is a variable of the definition being printed,
// as opposed to a definition, a host's name or an owned one.
func (p *printer) local(n string) bool { return !strings.Contains(n, ".") && p.locals[n] }

// table prints `(table n f)`: a rule `array { e : k ∈ 0..n-1 }`, a copy of a
// table by its own indexing, or a constant fill (tables.md §2.4, ADR 0061).
func (p *printer) table(n, f core.Expr) *Doc {
	upper := p.upper(n)
	switch f := f.(type) {
	case *core.Lam:
		if len(f.Params) == 1 {
			return Group(Cat(Text("array { "), p.expr(f.Body), Text(" : "+Spell(f.Params[0])+" ∈ 0.."), upper, Text(" }")))
		}
	case *core.Var:
		if !p.ctors[f.Name] {
			k := p.freshName()
			return Group(Cat(Text("array { "), p.name(f.Name), Text("["+k+"] : "+k+" ∈ 0.."), upper, Text(" }")))
		}
	}
	return Group(Cat(Text("array { "), p.expr(f), Text(" : _ ∈ 0.."), upper, Text(" }")))
}

// upper is n − 1, folded when n is a literal.
func (p *printer) upper(n core.Expr) *Doc {
	if l, ok := n.(*core.Lit); ok && l.T.Kind == core.KInt {
		return Text(Integer(fmt.Sprint(l.T.Int - 1)))
	}
	return Cat(p.prec(n, pAdd), Text("-1"))
}

func (p *printer) ifExpr(e *core.If) *Doc {
	head := Cat(Text("if "), p.expr(e.C), Text(" "), p.block(e.A))
	if isUnit(e.B) {
		return head // a one-armed if is if(c; a; ()) (ADR 0056 item 6)
	}
	if b, ok := e.B.(*core.If); ok {
		return Cat(head, Text(" else "), p.ifExpr(b))
	}
	return Cat(head, Text(" else "), p.block(e.B))
}

func isUnit(e core.Expr) bool {
	a, ok := e.(*core.Apply)
	if !ok || len(a.Args) != 0 {
		return false
	}
	v, ok := a.Fn.(*core.Var)
	return ok && v.Name == "tuple"
}

func (p *printer) caseExpr(e *core.Case) *Doc {
	var arms []*Doc
	for i := range e.Arms {
		a := &e.Arms[i]
		arms = append(arms, p.item(a.Body, func() *Doc { return arm(p.pattern(a.Pat), p.expr(a.Body)) }))
	}
	return p.braced(Cat(Text("case "), p.expr(e.Scrut), Text(" ")), arms, e)
}

// pattern prints a case pattern as written: a constructor, possibly applied to
// names, a literal, `_`, a name, or `else`.
func (p *printer) pattern(e core.Expr) *Doc {
	switch e := e.(type) {
	case *core.Var:
		if e.Name == "else" {
			return Text("else")
		}
		return p.name(e.Name)
	case *core.Apply:
		if v, ok := e.Fn.(*core.Var); ok {
			return Cat(p.name(v.Name), p.args(e.Args))
		}
	}
	return p.expr(e)
}

func (p *printer) loop(e *core.Loop) *Doc {
	if e.Match {
		e = p.nameState(e)
	}
	var binds []*Doc
	for i, v := range e.Vars {
		p.locals[v] = true
		binds = append(binds, Cat(Text(Spell(v)+" = "), p.expr(e.Inits[i])))
	}
	var arms []*Doc
	for i := range e.Arms {
		a := &e.Arms[i]
		arms = append(arms, p.item(a.Body, func() *Doc {
			var guard *Doc
			body := a.Body
			switch {
			case a.Else:
				guard = Text("else")
			case e.Match:
				guard, body = p.matchArm(a, e.Vars)
			default:
				guard = p.expr(a.Guard)
			}
			return arm(guard, p.tail(body))
		}))
	}
	head := Cat(Text("loop "), Nest(5, Fill(",", binds)), Text(" "))
	return p.braced(head, arms, e)
}

// matchArm is a match arm as a guard over the loop's variables and its body
// renamed, as D builds it (core/desugar.go, matchGuard).
func (p *printer) matchArm(a *core.LoopArm, vars []string) (*Doc, core.Tail) {
	var tests []*Doc
	rename := map[string]string{}
	for i, pt := range a.Pats {
		switch pt := pt.(type) {
		case *core.PVar:
			rename[pt.Name] = vars[i]
		case *core.PLit:
			switch {
			case pt.T.Kind == core.KBool && pt.T.IsTrue():
				tests = append(tests, Text(Spell(vars[i])))
			case pt.T.Kind == core.KBool:
				tests = append(tests, Text("!"+Spell(vars[i])))
			default:
				tests = append(tests, Text(Spell(vars[i])+" == "+pt.T.String()))
			}
		}
	}
	body := renameTail(a.Body, rename)
	if a.When != nil {
		tests = append(tests, p.prec(renameExpr(a.When, rename), pAnd+1))
	}
	if len(tests) == 0 {
		return Text("true"), body
	}
	return Join(Text(" && "), tests), body
}

// braced prints a head and items in braces: flat, `head { a; b }`, when it fits
// and holds no comment, one item a line otherwise.
//
// More than two arms always take a line each.
func (p *printer) braced(head *Doc, items []*Doc, node any) *Doc {
	close := p.closing(node)
	if len(items) > 2 {
		return Cat(head, Text("{"), Nest(4, Cat(Hard(), Join(Hard(), items), close)), Hard(), Text("}"))
	}
	return Group(Cat(head, Text("{"), Nest(4, Cat(Line(), Join(Sep(), items), close)), Line(), Text("}")))
}

// ---- tails and blocks ----

// tail prints a loop arm's body: an expression, a back edge, or a block whose
// statements end in one.
func (p *printer) tail(t core.Tail) *Doc {
	switch t := t.(type) {
	case *core.Again:
		return Cat(Text("again"), p.args(t.Args))
	case *core.TLet, *core.TSeq, *core.TMonad:
		return p.block(t)
	case core.Expr:
		if needsBlock(t) {
			return p.block(t)
		}
		return p.expr(t)
	}
	panic(fmt.Sprintf("j: no printing for the tail %T", t))
}

func needsBlock(e core.Expr) bool {
	switch e.(type) {
	case *core.Let, *core.Seq, *core.Local, *core.BuildMap, *core.Monad:
		return true
	}
	return false
}

// block prints a term as a block: its binders and statements in order, then its
// value (spec/j.md §2.6, ADR 0054: a block is a scope, and J writes no head).
// It lies on one line when it fits.
func (p *printer) block(t core.Tail) *Doc {
	stmts := p.stmts(t)
	d := Group(Cat(Text("{"), Nest(4, Cat(Line(), Join(Sep(), stmts))), Line(), Text("}")))
	d.block = true
	return d
}

// body prints a definition's body: one statement a line when it has more than
// one (spec/j.md §6), on the header's line when it is one expression that fits.
func (p *printer) body(t core.Tail) *Doc {
	stmts := p.stmts(t)
	if len(stmts) == 1 {
		return Group(Cat(Text("{"), Nest(4, Cat(Line(), stmts[0])), Line(), Text("}")))
	}
	return Cat(Text("{"), Nest(4, Cat(Hard(), Join(Hard(), stmts))), Hard(), Text("}"))
}

// stmts flattens a term into a block's statements, the last its value.
func (p *printer) stmts(t core.Tail) []*Doc {
	var out []*Doc
	for {
		switch e := t.(type) {
		case *core.Let:
			for _, b := range e.Binds {
				out = append(out, p.item(b.Value, func() *Doc { return p.bind(b) }))
			}
			t = e.Body
			continue
		case *core.TLet:
			for _, b := range e.Binds {
				out = append(out, p.item(b.Value, func() *Doc { return p.bind(b) }))
			}
			t = e.Body
			continue
		case *core.Seq:
			for _, x := range e.Xs[:len(e.Xs)-1] {
				out = append(out, p.item(x, func() *Doc { return p.stmt(x) }))
			}
			t = e.Xs[len(e.Xs)-1]
			continue
		case *core.TSeq:
			for _, x := range e.Xs {
				out = append(out, p.item(x, func() *Doc { return p.stmt(x) }))
			}
			t = e.Last
			continue
		case *core.Local:
			for _, b := range e.Binds {
				out = append(out, p.item(b.Init, func() *Doc { return p.localBind(b) }))
			}
			t = e.Body
			continue
		case *core.BuildMap:
			for i, n := range e.Names {
				p.data[n] = true
				out = append(out, Cat(Text(Spell(n)+" linear = map("), p.expr(e.Caps[i]), Text(")")))
			}
			t = e.Body
			continue
		case *core.Monad:
			out = append(out, p.monad(e.Form, e.Items))
			if e.Form == "try" {
				return append(out[:len(out)-1], Cat(out[len(out)-1], Text(" "), p.block(e.Body)))
			}
			t = e.Body
			continue
		case *core.TMonad:
			out = append(out, p.monad(e.Form, e.Items))
			if e.Form == "try" {
				return append(out[:len(out)-1], Cat(out[len(out)-1], Text(" "), p.block(e.Body)))
			}
			t = e.Body
			continue
		case *core.Apply:
			// A continuation, ((f a) (fn (q r) e)): the binding (q, r) = f(a).
			if lam, ok := cont(e); ok {
				out = append(out, p.item(e.Fn, func() *Doc { return Cat(p.tuplePat(lam.Params), Text(" = "), p.expr(e.Fn)) }))
				t = lam.Body
				continue
			}
			// A λ applied where it is written: x = a, in order.
			if lam, ok := e.Fn.(*core.Lam); ok && len(lam.Params) == len(e.Args) {
				for i, x := range lam.Params {
					p.locals[x] = true
					out = append(out, Cat(Text(Spell(x)+" = "), p.expr(e.Args[i])))
				}
				t = lam.Body
				continue
			}
		}
		break
	}
	out = append(out, p.item(t, func() *Doc { return p.tail(t) }))
	return out
}

// cont reports whether an application is a continuation: a call's results
// applied to a λ that names them.
func cont(e *core.Apply) (*core.Lam, bool) {
	if len(e.Args) != 1 {
		return nil, false
	}
	lam, ok := e.Args[0].(*core.Lam)
	if !ok {
		return nil, false
	}
	if _, ok := e.Fn.(*core.Apply); !ok {
		return nil, false
	}
	return lam, true
}

func (p *printer) tuplePat(names []string) *Doc {
	for _, n := range names {
		p.locals[n] = true
	}
	if len(names) == 1 {
		return Text(Spell(names[0]))
	}
	return Cat(Text("("), p.params(names), Text(")"))
}

func (p *printer) bind(b core.LetBind) *Doc {
	var lhs *Doc
	switch pat := b.Pat.(type) {
	case *core.PVar:
		p.locals[pat.Name] = true
		if p.producesData(b.Value, p.data) {
			p.data[pat.Name] = true
		}
		lhs = Text(Spell(pat.Name))
	case *core.PWild:
		lhs = Text("_")
	case *core.PTuple:
		lhs = p.tuplePat(pat.Names)
	}
	return Cat(lhs, Text(" = "), p.expr(b.Value))
}

// localBind prints a binder of local state as today's semantics has it: a buffer
// is a definitional binder in the linear mode, threaded as a value (ADR 0047,
// 0050 item 2), until ADR 0052 makes it a state binder written in program
// order; a cell is a state binder.
func (p *printer) localBind(b core.LocalBind) *Doc {
	p.locals[b.Name] = true
	if b.Buffer {
		p.data[b.Name] = true
		return Cat(Text(Spell(b.Name)+" linear = "), p.expr(b.Init))
	}
	return Cat(Text("var "+Spell(b.Name)+" := "), p.expr(b.Init))
}

// stmt prints a term in a statement position: a cell's write is a statement.
func (p *printer) stmt(e core.Expr) *Doc { return p.expr(e) }

func (p *printer) monad(form string, items []core.Expr) *Doc {
	pat := p.pattern(items[0])
	val := p.expr(items[1])
	if form == "expect" {
		return Cat(Text("expect "), pat, Text(" = "), val, Text(" else "), p.expr(items[2]))
	}
	return Cat(Text("try "), pat, Text(" = "), val)
}

// producesData reports whether a binder's value is a table or a map by its form.
func producesData(e core.Expr, data map[string]bool) bool {
	switch e := e.(type) {
	case *core.Var:
		return data[e.Name]
	case *core.Apply:
		if v, ok := e.Fn.(*core.Var); ok {
			switch v.Name {
			case "array", "table", "set", "insert":
				return true
			}
		}
	}
	return false
}

// chainAnd and chainOr splice a connective's last operand when it is the same
// connective: (and a (and b c)) and (and a b c) have one D-image, the
// right-nested conjunction (core/desugar.go), so J writes `a && b && c`. A
// left-nested one has another image and keeps its parentheses.
func chainAnd(xs []core.Expr) []core.Expr {
	if n := len(xs); n > 0 {
		if a, ok := xs[n-1].(*core.And); ok && len(a.Xs) > 0 {
			return append(append([]core.Expr(nil), xs[:n-1]...), chainAnd(a.Xs)...)
		}
	}
	return xs
}

func chainOr(xs []core.Expr) []core.Expr {
	if n := len(xs); n > 0 {
		if o, ok := xs[n-1].(*core.Or); ok && len(o.Xs) > 0 {
			return append(append([]core.Expr(nil), xs[:n-1]...), chainOr(o.Xs)...)
		}
	}
	return xs
}

// arm prints `head -> body`, the body on the next line, indented, when it does
// not fit after the arrow.
func arm(head, body *Doc) *Doc {
	if body.block {
		return Cat(head, Text(" -> "), body)
	}
	return Group(Cat(head, Text(" ->"), Nest(4, Cat(Line(), body))))
}

// isLam reports a λ: a table is never applied to one, so its application is a call.
func isLam(e core.Expr) bool { _, ok := e.(*core.Lam); return ok }

// nameState names a match's anonymous state variables, which μ calls `#m…`
// since today's match does not name them (match.md): J's loop names its state.
// A column whose arms bind one name takes it; another takes a fresh `s1`, `s2`.
// The renaming reaches every use, since a match arm's body refers to its state
// only through the names its patterns bind.
func (p *printer) nameState(e *core.Loop) *core.Loop {
	vars := append([]string(nil), e.Vars...)
	taken := map[string]bool{}
	for _, v := range vars {
		taken[v] = true
	}
	for i, v := range vars {
		if !strings.HasPrefix(v, "#") {
			continue
		}
		name := ""
		for _, a := range e.Arms {
			if i < len(a.Pats) {
				if pv, ok := a.Pats[i].(*core.PVar); ok {
					if name == "" {
						name = pv.Name
					} else if name != pv.Name {
						name = "-"
					}
				}
			}
		}
		if name == "" || name == "-" || taken[name] {
			for k := 1; ; k++ {
				name = fmt.Sprintf("s%d", k)
				if !taken[name] {
					break
				}
			}
		}
		taken[name] = true
		vars[i] = name
	}
	out := *e
	out.Vars = vars
	return &out
}

// isRead reports whether an application reads data: a data name applied to one
// argument, printed as indexing.
func (p *printer) isRead(e *core.Apply) bool {
	v, ok := e.Fn.(*core.Var)
	return ok && len(e.Args) == 1 && p.data[v.Name]
}
