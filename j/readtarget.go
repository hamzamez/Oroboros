package j

import (
	"fmt"
	"math/big"
	"strings"

	"oroboros/core"
)

// READING A TARGET FILE OR FRAGMENT IN J (spec/j.md §2.1–2.5, §2.11–2.12): the
// inverse of target.go. It gives the raw forms the target loader reads today
// (emit/target.go), each a datum of today's notation: a host declaration a
// `sig` with its host body, a set's carrier a `type`, a constant a `const`, an
// inclusion `implements`, the model's data in its words. A definition inside a
// fragment is a program's, read by ReadProgram and carried as D's term.

// ReadTarget reads a target file in J into today's raw forms. target is the
// target its directory names (ADR 0064 item 2), "" for a library's fragment
// file, whose `provides` names its own.
// root is the repository, whose lib/ holds the modules a fragment imports.
func ReadTarget(src, target, root string) ([]*core.Term, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, sets: map[string]string{"Option": "option"}, folded: map[core.Expr]*big.Int{}, indexed: map[core.Expr]bool{}}
	p.registerSets(src, root, root)
	forms, err := p.targetItems()
	if err != nil {
		return nil, err
	}
	if target == "" {
		return forms, nil
	}
	return []*core.Term{app(append([]*core.Term{core.Name("target"), core.Name(target)}, forms...)...)}, nil
}

func app(ks ...*core.Term) *core.Term { return &core.Term{Kind: core.KApp, Kids: ks} }

func (p *parser) targetItems() ([]*core.Term, error) {
	var out []*core.Term
	for {
		p.endOfStatement()
		if p.peek().kind == tEOF || p.is("}") {
			return out, nil
		}
		t, err := p.targetItem()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
}

func (p *parser) targetItem() (*core.Term, error) {
	switch {
	case p.accept("module"):
		path := p.path()
		inner, err := p.bracedItems()
		if err != nil {
			return nil, err
		}
		return app(append([]*core.Term{core.Name("module"), core.Name(path)}, inner...)...), nil
	case p.accept("provides"):
		target := p.next().text
		path := p.path()
		inner, err := p.bracedItems()
		if err != nil {
			return nil, err
		}
		return app(append([]*core.Term{core.Name("provides"), core.Name(target), core.Name(path)}, inner...)...), nil
	case p.accept("use"):
		u := []*core.Term{core.Name("use"), core.Name(p.path())}
		if p.accept("as") {
			u = append(u, core.Name("as"), core.Name(p.next().text))
		}
		return app(u...), nil
	case p.is("fn") && p.isProgramDef():
		// A program's definition in a fragment, D_T (target-system.md §6): the
		// program reader's, and today's loader reads it as D of its datum.
		ds, _, err := p.item(false)
		if err != nil {
			return nil, err
		}
		return core.DeclTerms(ds)[0], nil
	case p.is("fn"):
		return p.hostFn()
	case p.is("set") && p.peekN(1).kind != tSym:
		return p.hostSet()
	case p.accept("axiom"):
		t := p.rawName(p.next())
		if err := p.expect("⊆"); err != nil {
			return nil, err
		}
		out := []*core.Term{core.Name("implements"), t}
		for {
			out = append(out, p.rawName(p.next()))
			if !p.accept("∩") {
				break
			}
		}
		return app(out...), nil
	}
	t := p.peek()
	if (t.kind == tIdent || t.kind == tQuoted) && p.peekN(1).text == "∈" {
		return p.hostConst()
	}
	if t.kind == tIdent {
		switch t.text {
		case "repr", "include", "structural", "backend", "link", "build", "artifact", "data",
			"underlying", "max_len", "big_repr", "int_repr", "fact":
			return p.modelForm()
		}
	}
	return nil, p.errorf("%s begins no form of a target file", t)
}

func (p *parser) bracedItems() ([]*core.Term, error) {
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	p.open(false)
	inner, err := p.targetItems()
	if err != nil {
		return nil, err
	}
	p.close()
	return inner, p.expect("}")
}

// rawName is a name in today's spelling: σ_J⁻¹, a quoted name as written, a
// qualified one joined by `.`; a set's name by the convention's inverse.
func (p *parser) rawName(t token) *core.Term {
	name := Unspell(t.text)
	if t.kind == tQuoted {
		name = t.text
	}
	for p.is(".") && !p.peek().nl {
		p.next()
		m := p.next()
		if m.kind == tQuoted {
			name += "." + m.text
		} else {
			name += "." + Unspell(m.text)
		}
	}
	return core.Name(p.setName(name))
}

// rawExpr reads an expression and gives today's datum for it.
func (p *parser) rawExprTerm() (*core.Term, error) {
	e, err := p.expr()
	if err != nil {
		return nil, err
	}
	return datum(e), nil
}

// datum is an expression a target file writes as the loader reads it: D of
// its tree, as core.ReadAll gives every datum of today's target files (a
// tuple's Church form, the connectives as `if`).
func datum(e core.Expr) *core.Term { return core.Desugar(e) }

func datums(es []core.Expr) []*core.Term {
	out := make([]*core.Term, len(es))
	for i, e := range es {
		out[i] = datum(e)
	}
	return out
}

// setDatum is today's spelling of a set a target file writes.
func (p *parser) setDatum(e core.Expr, mode string) *core.Term { return datum(p.toSet(e, mode)) }

// hostFn reads a host declaration, `fn NAME(params) (result) clauses { host … }`,
// or a λ returning one, `fn NAME(E ⊆ B) { fn(params) … { host … } }` (ADR 0066),
// into today's `sig`.
func (p *parser) hostFn() (*core.Term, error) {
	p.next()
	nt := p.next()
	name := core.Name(Unspell(nt.text))
	if nt.kind == tQuoted {
		name = core.Name(nt.text)
	}
	var statics []string
	var head *core.Term = name
	if p.peekStatic() {
		p.next()
		heads := []*core.Term{name}
		for !p.is(")") {
			n := p.next().text
			statics = append(statics, n)
			if p.accept("⊆") {
				b, err := p.expr()
				if err != nil {
					return nil, err
				}
				heads = append(heads, app(core.Name(n), app(core.Name("subsets"), p.setDatum(b, ""))))
			} else if p.accept("∈") {
				b, err := p.expr()
				if err != nil {
					return nil, err
				}
				heads = append(heads, app(core.Name(n), p.setDatum(b, "")))
			} else {
				heads = append(heads, core.Name(n))
			}
			p.accept(",")
		}
		p.next()
		head = app(heads...)
		if err := p.expect("{"); err != nil {
			return nil, err
		}
		p.open(false)
		if err := p.expect("fn"); err != nil {
			return nil, err
		}
		sig, err := p.hostSigRest(head, statics)
		if err != nil {
			return nil, err
		}
		p.close()
		return sig, p.expect("}")
	}
	return p.hostSigRest(head, nil)
}

// isProgramDef reports whether the `fn` ahead is a program's definition: its
// body, the first brace at depth zero, does not begin with `host`, and no λ
// inside it returns one (ADR 0066's static λ, whose body is `fn(…) … { host … }`).
func (p *parser) isProgramDef() bool {
	depth := 0
	for k := p.i + 1; k < len(p.toks); k++ {
		t := p.toks[k]
		switch t.text {
		case "(", "[":
			depth++
		case ")", "]":
			depth--
		case "{":
			if depth == 0 && t.kind == tSym {
				nx := p.toks[k+1]
				return nx.text != "host" && nx.text != "fn"
			}
		}
		if t.kind == tEOF {
			return false
		}
	}
	return false
}

// peekStatic reports whether a definition's parentheses bind sets: `(E ⊆ B)`,
// followed by a block holding a λ.
func (p *parser) peekStatic() bool {
	if !p.is("(") {
		return false
	}
	depth := 0
	for k := p.i; k < len(p.toks); k++ {
		switch p.toks[k].text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return k+2 < len(p.toks) && p.toks[k+1].text == "{" && p.toks[k+2].text == "fn"
			}
		}
	}
	return false
}

func (p *parser) hostSigRest(head *core.Term, statics []string) (*core.Term, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}
	p.open(true)
	var params []*core.Term
	var names []string
	for !p.is(")") {
		n := p.name()
		names = append(names, n)
		if err := p.expect("∈"); err != nil {
			return nil, err
		}
		s, err := p.expr()
		if err != nil {
			return nil, err
		}
		mode := ""
		if p.is("linear") || p.is("inout") {
			mode = p.next().text
		}
		params = append(params, app(core.Name(n), p.setDatum(s, mode)))
		if !p.accept(",") {
			break
		}
	}
	p.close()
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	sig := []*core.Term{core.Name("sig"), head, app(params...)}
	result := ""
	if err := p.expect("("); err != nil {
		return nil, err
	}
	p.open(true)
	if p.is(")") {
		sig = append(sig, app())
	} else {
		if p.peek().kind == tIdent && p.peekN(1).text == "∈" {
			result = p.next().text
			p.next()
		}
		s, err := p.expr()
		if err != nil {
			return nil, err
		}
		sig = append(sig, p.resultDatum(s))
	}
	p.close()
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	for {
		switch {
		case p.is("pure") || p.is("index") || p.is("variadic"):
			sig = append(sig, core.Name(p.next().text))
			continue
		case p.accept("where"):
			e, err := p.rawExprTerm()
			if err != nil {
				return nil, err
			}
			sig = append(sig, app(core.Name("where"), e))
			continue
		case p.accept("ensure"):
			e, err := p.expr()
			if err != nil {
				return nil, err
			}
			if result != "" {
				e = renameExpr(e, map[string]string{Unspell(result): core.ResultName})
			}
			sig = append(sig, app(core.Name("ensures"), datum(e)))
			continue
		}
		break
	}
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	p.open(false)
	host, err := p.hostBodyDatum(names, statics, result)
	if err != nil {
		return nil, err
	}
	p.close()
	if err := p.expect("}"); err != nil {
		return nil, err
	}
	return app(append(sig, host)...), nil
}

// resultDatum is today's spelling of a sig's result: a product as the list of
// its factors, `(int int)`, as today's `sig` writes several results.
func (p *parser) resultDatum(e core.Expr) *core.Term { return p.setDatum(e, "") }

// hostBodyDatum reads `host [KIND] "template" clauses…` into today's `(host …)`,
// its named splices back to today's positional holes.
func (p *parser) hostBodyDatum(params, statics []string, result string) (*core.Term, error) {
	if err := p.expect("host"); err != nil {
		return nil, err
	}
	out := []*core.Term{core.Name("host")}
	if p.is("expr") || p.is("stmt") {
		out = append(out, core.Name(p.next().text))
	}
	if p.peek().kind == tString {
		out = append(out, core.Str(positionalHoles(p.next().text, params, statics, result)))
	}
	for !p.is("}") && !p.lineEnds() {
		switch {
		case p.is("import") || p.is("lib") || p.is("checked") || p.is("jump"):
			word := p.next().text
			c := []*core.Term{core.Name(word)}
			for !p.is("}") && !p.lineEnds() && (p.peek().kind == tString || (word == "checked" && p.peek().kind == tIdent)) {
				t := p.next()
				if t.kind == tString {
					s := t.text
					if word == "jump" && len(c) == 2 {
						s = positionalHoles(s, params, statics, result)
					}
					c = append(c, core.Str(s))
				} else {
					c = append(c, core.Name(Unspell(t.text)))
				}
				if word != "import" && word != "jump" {
					break
				}
			}
			out = append(out, app(c...))
		case p.accept("comma_ok"):
			out = append(out, app(core.Name("comma-ok")))
		default:
			return nil, p.errorf("%s in a host body", p.peek())
		}
	}
	return app(out...), nil
}

// positionalHoles is the inverse of named (target.go): `${x}` back to today's
// holes. A template that splices the result or a register's view is x86's,
// whose holes are numbered, `%n` and `%r`; any other's are `%s`, filled in
// order (emit/target.go, fill). `${#}` is `%u`, `$${` a literal `${`, and a
// literal `%` is `%%`.
func positionalHoles(tpl string, params, statics []string, result string) string {
	numbered := result != "" && strings.Contains(tpl, "${"+Spell(result))
	var b strings.Builder
	for i := 0; i < len(tpl); i++ {
		switch {
		case strings.HasPrefix(tpl[i:], "$${"):
			b.WriteString("${")
			i += 2
		case strings.HasPrefix(tpl[i:], "${"):
			end := strings.IndexByte(tpl[i:], '}')
			inner := tpl[i+2 : i+end]
			i += end
			name, view := inner, ""
			if k := strings.IndexByte(inner, ':'); k >= 0 {
				name, view = inner[:k], inner[k+1:]
			}
			b.WriteString(hole(Unspell(name), view, params, statics, result, numbered))
		case tpl[i] == '%':
			b.WriteString("%%")
		default:
			b.WriteByte(tpl[i])
		}
	}
	return b.String()
}

func hole(name, view string, params, statics []string, result string, numbered bool) string {
	if name == "#" {
		return "%u"
	}
	if result != "" && name == Unspell(result) {
		return "%" + view + "r"
	}
	for k, s := range statics {
		if s == name {
			return fmt.Sprintf("%%t%d", k+1)
		}
	}
	for k, n := range params {
		if n == name {
			if numbered || view != "" {
				return fmt.Sprintf("%%%s%d", view, k+1)
			}
			return "%s"
		}
	}
	return "${" + name + "}"
}

// hostSet reads `set N = host "s"`, `set F(A…) = host "…${A}…"` or `set N = S`.
func (p *parser) hostSet() (*core.Term, error) {
	p.next()
	nameTok := p.next()
	name := core.Name(p.setName(Unspell(nameTok.text)))
	if nameTok.kind == tQuoted {
		name = core.Name(nameTok.text)
	}
	var head *core.Term = name
	var params []string
	if p.accept("(") {
		hs := []*core.Term{name}
		for !p.is(")") {
			n := p.next().text
			params = append(params, n)
			hs = append(hs, core.Name(n))
			p.accept(",")
		}
		p.next()
		head = app(hs...)
	}
	if err := p.expect("="); err != nil {
		return nil, err
	}
	if p.is("host") {
		h, err := p.hostBodyDatum(params, nil, "")
		if err != nil {
			return nil, err
		}
		return app(core.Name("type"), head, h), nil
	}
	s, err := p.expr()
	if err != nil {
		return nil, err
	}
	return app(core.Name("type"), head, p.setDatum(s, "")), nil
}

// hostConst reads `N ∈ set { v } = host "s" clauses` into today's `const`.
func (p *parser) hostConst() (*core.Term, error) {
	name := p.rawName(p.next())
	p.next() // ∈
	if err := p.expect("set"); err != nil {
		return nil, err
	}
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	v, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect("}"); err != nil {
		return nil, err
	}
	if err := p.expect("="); err != nil {
		return nil, err
	}
	h, err := p.hostBodyDatum(nil, nil, "")
	if err != nil {
		return nil, err
	}
	return app(core.Name("const"), name, datum(v), h), nil
}

// modelForm reads one of the model's own forms, its word and its operands
// (spec/j.md §2.12), into today's datum.
func (p *parser) modelForm() (*core.Term, error) {
	word := Unspell(p.next().text)
	out := []*core.Term{core.Name(word)}
	for !p.lineEnds() && !p.is("}") {
		switch {
		case p.is("host") && (p.peekN(1).kind == tString || p.peekN(1).text == "expr" || p.peekN(1).text == "stmt"):
			p.next()
			h := []*core.Term{core.Name("host")}
			for p.peek().kind == tString || p.is("expr") || p.is("stmt") {
				t := p.next()
				if t.kind == tString {
					h = append(h, core.Str(t.text))
				} else {
					h = append(h, core.Name(t.text))
				}
			}
			out = append(out, app(h...))
		case p.accept("niche"):
			p.next() // host
			h := []*core.Term{core.Name("host")}
			for p.peek().kind == tString || p.is("expr") || p.is("stmt") {
				t := p.next()
				if t.kind == tString {
					h = append(h, core.Str(t.text))
				} else {
					h = append(h, core.Name(t.text))
				}
			}
			out = append(out, app(core.Name("niche"), app(h...)))
		case word == "fact" && len(out) == 1 && p.peek().kind == tIdent:
			out = append(out, core.Name(Unspell(p.next().text)))
		case word == "fact" && p.is("("):
			p.next()
			var ps []*core.Term
			for !p.is(")") {
				n := p.name()
				p.expect("∈")
				s, err := p.expr()
				if err != nil {
					return nil, err
				}
				ps = append(ps, app(core.Name(n), p.setDatum(s, "")))
				p.accept(",")
			}
			p.next()
			out = append(out, app(ps...))
			e, err := p.rawExprTerm()
			if err != nil {
				return nil, err
			}
			out = append(out, e)
		case p.peek().kind == tString:
			out = append(out, core.Str(p.next().text))
		case p.peek().kind == tIdent && !strings.Contains("0123456789", p.peek().text[:1]) && p.peekN(1).text != ".." && p.peekN(1).text != "(":
			out = append(out, core.Name(Unspell(p.next().text)))
		default:
			e, err := p.exprAt(pCmp + 1)
			if err != nil {
				return nil, err
			}
			out = append(out, p.setDatum(e, ""))
		}
	}
	return app(out...), nil
}
