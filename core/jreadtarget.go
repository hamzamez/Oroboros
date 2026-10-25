package core

import (
	"fmt"
	"math/big"
	"strings"
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
// dir is the file's directory and root the repository: an imported module
// is read from beside the file or from root/lib, to know which of its names are
// sets.
func ReadTarget(src, target, dir, root string) ([]*Term, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, sets: map[string]string{"Option": "option"}, folded: map[Expr]*big.Int{}, indexed: map[Expr]bool{}}
	p.registerSets(src, dir, root)
	p.x86 = target == "windows"
	forms, err := p.targetItems()
	if err != nil {
		return nil, err
	}
	forms = liftAxioms(forms)
	if target == "" {
		return forms, nil
	}
	return []*Term{app(append([]*Term{Name("target"), Name(target)}, forms...)...)}, nil
}

func app(ks ...*Term) *Term { return &Term{Kind: KApp, Kids: ks} }

func (p *parser) targetItems() ([]*Term, error) {
	var out []*Term
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

// liftAxioms takes each inclusion out of the module J writes it in, to the
// file's top level with its type's whole name, `(implements P.T I…)`, where the
// loader reads it: one law, ADR 0064 item 9, an inclusion is an axiom of the
// model, wherever it is written.
func liftAxioms(forms []*Term) []*Term {
	var out, lifted []*Term
	var walk func(t *Term, path string) *Term
	walk = func(t *Term, path string) *Term {
		p := t.Kids[1].Name
		if path != "" && !strings.Contains(p, "/") {
			p = path + "/" + p
		}
		kids := []*Term{t.Kids[0], t.Kids[1]}
		for _, k := range t.Kids[2:] {
			switch head(k) {
			case "implements":
				ax := append([]*Term{k.Kids[0], Name(p + "." + k.Kids[1].Name)}, k.Kids[2:]...)
				lifted = append(lifted, app(ax...))
			case "module":
				kids = append(kids, walk(k, p))
			default:
				kids = append(kids, k)
			}
		}
		return app(kids...)
	}
	for _, f := range forms {
		if head(f) == "module" {
			out = append(out, walk(f, ""))
			continue
		}
		out = append(out, f)
	}
	return append(out, lifted...)
}

func (p *parser) targetItem() (*Term, error) {
	switch {
	case p.accept("module"):
		path := p.path()
		inner, err := p.bracedItems()
		if err != nil {
			return nil, err
		}
		return app(append([]*Term{Name("module"), Name(path)}, inner...)...), nil
	case p.accept("provides"):
		target := p.next().text
		path := p.path()
		saved := p.x86
		p.x86 = target == "windows"
		inner, err := p.bracedItems()
		p.x86 = saved
		if err != nil {
			return nil, err
		}
		return app(append([]*Term{Name("provides"), Name(target), Name(path)}, inner...)...), nil
	case p.accept("use"):
		u := []*Term{Name("use"), Name(p.path())}
		if p.accept("as") {
			u = append(u, Name("as"), Name(p.next().text))
		}
		return app(u...), nil
	case p.is("fn") && p.isProgramDef(),
		(p.peek().kind == tIdent || p.peek().kind == tQuoted) && p.peekN(1).text == "=" && p.peekN(1).kind == tSym:
		// A program's definition in a fragment, D_T (target-system.md §6): the
		// program reader's, and today's loader reads it as D of its datum.
		ds, _, err := p.item()
		if err != nil {
			return nil, err
		}
		return DeclTerms(ds)[0], nil
	case p.is("fn"):
		return p.hostFn()
	case p.is("set") && p.peekN(1).kind != tSym:
		return p.hostSet()
	case p.accept("axiom"):
		t := p.rawName(p.next())
		if err := p.expect("⊆"); err != nil {
			return nil, err
		}
		out := []*Term{Name("implements"), t}
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
			"underlying", "max_len", "big_repr", "int_repr", "fact", "lemma":
			return p.modelForm()
		}
	}
	return nil, p.errorf("%s begins no form of a target file", t)
}

func (p *parser) bracedItems() ([]*Term, error) {
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
func (p *parser) rawName(t token) *Term {
	name := Unspell(t.text)
	if t.kind == tQuoted {
		name = t.text
		if !p.is(".") || p.peek().nl {
			// A QUOTED NAME IS THE NAME ITSELF (σ_J, spec/j.md §1.5): `Int` is
			// the language's set, and `` `Int` `` text/scanner's constant.
			return Name(name)
		}
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
	return Name(p.setName(name))
}

// rawExpr reads an expression and gives today's datum for it.
func (p *parser) rawExprTerm() (*Term, error) {
	e, err := p.expr()
	if err != nil {
		return nil, err
	}
	return datum(e), nil
}

// datum is an expression a target file writes as the loader reads it: D of
// its tree, as ReadAll gives every datum of today's target files (a
// tuple's Church form, the connectives as `if`).
func datum(e Expr) *Term { return Desugar(e) }

// setDatum is today's spelling of a set a target file writes.
func (p *parser) setDatum(e Expr, mode string) *Term { return datum(p.toSet(e, mode)) }

// hostFn reads a host declaration, `fn NAME(params) (result) clauses { host … }`,
// or a λ returning one, `fn NAME(E ⊆ B) { fn(params) … { host … } }` (ADR 0066),
// into today's `sig`.
func (p *parser) hostFn() (*Term, error) {
	p.next()
	nt := p.next()
	name := Name(Unspell(nt.text))
	if nt.kind == tQuoted {
		name = Name(nt.text)
	}
	var statics []string
	var head *Term = name
	if p.peekStatic() {
		p.next()
		heads := []*Term{name}
		for !p.is(")") {
			n := p.next().text
			statics = append(statics, n)
			if p.accept("⊆") {
				b, err := p.expr()
				if err != nil {
					return nil, err
				}
				heads = append(heads, app(Name(n), app(Name("subsets"), p.setDatum(b, ""))))
			} else if p.accept("∈") {
				b, err := p.expr()
				if err != nil {
					return nil, err
				}
				heads = append(heads, app(Name(n), p.setDatum(b, "")))
			} else {
				heads = append(heads, Name(n))
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

func (p *parser) hostSigRest(head *Term, statics []string) (*Term, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}
	p.open(true)
	var params []*Term
	var names []string
	// The `linear` parameters, in order: each is handed back first in the core's
	// threaded result (ADR 0052's borrow, ADR 0070), as its own storage or as the
	// storage the result `p'` names.
	type borrowed struct {
		name string
		set  Expr
		at   int
	}
	var linear []borrowed
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
		if mode == "linear" {
			linear = append(linear, borrowed{n, p.toSet(s, "linear"), len(names) - 1})
		}
		params = append(params, app(Name(n), p.setDatum(s, mode)))
		if !p.accept(",") {
			break
		}
	}
	p.close()
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	sig := []*Term{Name("sig"), head, app(params...)}
	result := ""
	// A GROUP RESULT `(q ∈ B, n ∈ N, r ∈ E)` is the tuple pattern of its components
	// (spec/j.md §2.4), its set the product B × N × E, and each named component
	// renamed in the `ensure` to its position (ResultComponent): `File.Read`'s
	// count is at most the length of the buffer it fills, io.Reader's law.
	var components map[string]string
	if err := p.expect("("); err != nil {
		return nil, err
	}
	p.open(true)
	var parts []Expr
	var rnames []string
	if !p.is(")") {
		for {
			name := ""
			switch {
			case p.peek().kind == tIdent && p.peekN(1).text == "∈":
				name = p.next().text
				p.next()
			case p.peek().kind == tIdent && p.peekN(1).kind == tPrime && p.peekN(2).text == "∈":
				name = p.next().text + "'"
				p.next()
				p.next()
			}
			s, err := p.expr()
			if err != nil {
				return nil, err
			}
			parts, rnames = append(parts, s), append(rnames, name)
			if !p.accept(",") {
				break
			}
		}
	}
	// A RESULT WRITTEN AS A PRODUCT, `(A × B)`, is its factors, as the group
	// `(a ∈ A, b ∈ B)` is: one shape for several results (ADR 0051).
	if len(parts) == 1 && rnames[0] == "" {
		if a, ok := parts[0].(*Apply); ok {
			if v, ok := a.Fn.(*Var); ok && v.Name == "×" && len(a.Args) >= 2 {
				parts = append([]Expr(nil), a.Args...)
				rnames = make([]string, len(parts))
			}
		}
	}
	// THE THREADED RESULT (ADR 0070). The header is the caller's view: its result
	// is what the call gives, and a `linear` parameter is borrowed by reference
	// (ADR 0052 item 3). The core's is the state-passing translation's: each
	// borrowed buffer handed back first, then the rest, a unit dropped (B × 1 ≅ B).
	// A borrowed p with a result `p'` is handed back as the storage that result
	// names (a consume-and-return, an append); one with none is handed back as
	// its own storage (a write-borrow), recorded `(inplace j i)` for the IR.
	var threaded []Expr
	var tnames []string
	var inplace []*Term
	used := map[int]bool{}
	for _, b := range linear {
		at := -1
		for k, n := range rnames {
			if n == b.name+"'" {
				at = k
			}
		}
		if at >= 0 {
			threaded, tnames = append(threaded, parts[at]), append(tnames, rnames[at])
			used[at] = true
			continue
		}
		inplace = append(inplace, app(Name("inplace"), Int(int64(len(threaded))), Int(int64(b.at))))
		threaded, tnames = append(threaded, b.set), append(tnames, b.name+"'")
	}
	for k, part := range parts {
		if used[k] {
			continue
		}
		if strings.HasSuffix(rnames[k], "'") {
			return nil, p.errorf("the result %s names no `linear` parameter's storage", rnames[k])
		}
		if len(linear) > 0 && rnames[k] == "" && isBufferSet(part) {
			return nil, p.errorf("a buffer among the results of a declaration that borrows one: the header " +
				"is the caller's view (ADR 0070), so name it `p'` when the host hands back storage, " +
				"or leave it out when the host writes p in place")
		}
		threaded, tnames = append(threaded, part), append(tnames, rnames[k])
	}
	switch len(threaded) {
	case 0:
		sig = append(sig, app())
	case 1:
		result = tnames[0]
		sig = append(sig, p.resultDatum(threaded[0]))
	default:
		components = map[string]string{}
		for j, n := range tnames {
			if n != "" {
				components[Unspell(strings.TrimSuffix(n, "'"))+suffixOf(n)] = ResultComponent(j)
			}
		}
		sig = append(sig, p.resultDatum(&Apply{Fn: &Var{Name: "×"}, Args: threaded}))
	}
	sig = append(sig, inplace...)
	p.close()
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	for {
		switch {
		case p.is("pure") || p.is("index") || p.is("variadic"):
			sig = append(sig, Name(p.next().text))
			continue
		case p.accept("where"):
			e, err := p.rawExprTerm()
			if err != nil {
				return nil, err
			}
			sig = append(sig, app(Name("where"), e))
			continue
		case p.accept("ensure"):
			e, err := p.expr()
			if err != nil {
				return nil, err
			}
			if result != "" {
				e = renameExpr(e, map[string]string{Unspell(strings.TrimSuffix(result, "'")) + suffixOf(result): ResultName})
			}
			if components != nil {
				e = renameExpr(e, components)
			}
			sig = append(sig, app(Name("ensures"), datum(e)))
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
func (p *parser) resultDatum(e Expr) *Term { return p.setDatum(e, "") }

// hostBodyDatum reads `host [KIND] "template" clauses…` into today's `(host …)`,
// its named splices back to today's positional holes.
func (p *parser) hostBodyDatum(params, statics []string, result string) (*Term, error) {
	if err := p.expect("host"); err != nil {
		return nil, err
	}
	out := []*Term{Name("host")}
	if p.is("expr") || p.is("stmt") {
		out = append(out, Name(p.next().text))
	}
	if p.peek().kind == tString {
		out = append(out, Str(positionalHoles(p.next().text, params, statics, result, p.x86)))
	}
	for !p.is("}") && !p.lineEnds() {
		switch {
		case p.is("import") || p.is("lib") || p.is("checked") || p.is("jump"):
			word := p.next().text
			c := []*Term{Name(word)}
			for !p.is("}") && !p.lineEnds() && (p.peek().kind == tString || (word == "checked" && p.peek().kind == tIdent)) {
				t := p.next()
				if t.kind == tString {
					s := t.text
					if word == "jump" && len(c) == 2 {
						s = positionalHoles(s, params, statics, result, p.x86)
					}
					c = append(c, Str(s))
				} else {
					c = append(c, Name(Unspell(t.text)))
				}
				if word != "import" && word != "jump" {
					break
				}
			}
			out = append(out, app(c...))
		case p.accept("comma_ok"):
			out = append(out, app(Name("comma-ok")))
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
func positionalHoles(tpl string, params, statics []string, result string, x86 bool) string {
	// THE HOLES' STYLE IS THE BACKEND'S: x86's assembly templates number every
	// hole, `%n`, `%r`; every other target's fill `%s` in order and name the
	// result `%r` (emit/target.go, fill). Read off the model, not the template:
	// a JavaScript template that splices its result keeps its `%s`
	// (jswitch-2026-10-16, lib/os/js.oro's WriteFile).
	numbered := x86
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
			// A quoted splice is the name itself, as a quoted binder is.
			if len(name) > 1 && name[0] == '`' && name[len(name)-1] == '`' {
				name = name[1 : len(name)-1]
			} else {
				name = Unspell(name)
			}
			b.WriteString(hole(name, view, params, statics, result, numbered))
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
				// Past the ninth operand the number is braced: `%10` is
				// operand 1 and then `0` (emit/host.go).
				if k+1 > 9 {
					return fmt.Sprintf("%%%s{%d}", view, k+1)
				}
				return fmt.Sprintf("%%%s%d", view, k+1)
			}
			return "%s"
		}
	}
	return "${" + name + "}"
}

// hostSet reads `set N = host "s"`, `set F(A…) = host "…${A}…"` or `set N = S`.
func (p *parser) hostSet() (*Term, error) {
	p.next()
	nameTok := p.next()
	name := Name(p.setName(Unspell(nameTok.text)))
	if nameTok.kind == tQuoted {
		name = Name(nameTok.text)
	}
	var head *Term = name
	var params []string
	if p.accept("(") {
		hs := []*Term{name}
		for !p.is(")") {
			n := p.next().text
			params = append(params, n)
			hs = append(hs, Name(n))
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
		return app(Name("type"), head, h), nil
	}
	s, err := p.expr()
	if err != nil {
		return nil, err
	}
	return app(Name("type"), head, p.setDatum(s, "")), nil
}

// hostConst reads `N ∈ set { v } = host "s" clauses` into today's `const`.
func (p *parser) hostConst() (*Term, error) {
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
	return app(Name("const"), name, datum(v), h), nil
}

// modelForm reads one of the model's own forms, its word and its operands
// (spec/j.md §2.12), into today's datum.
func (p *parser) modelForm() (*Term, error) {
	word := Unspell(p.next().text)
	out := []*Term{Name(word)}
	for !p.lineEnds() && !p.is("}") {
		switch {
		case p.is("host") && (p.peekN(1).kind == tString || p.peekN(1).text == "expr" || p.peekN(1).text == "stmt"):
			p.next()
			h := []*Term{Name("host")}
			for p.peek().kind == tString || p.is("expr") || p.is("stmt") {
				t := p.next()
				if t.kind == tString {
					h = append(h, Str(t.text))
				} else {
					h = append(h, Name(t.text))
				}
			}
			out = append(out, app(h...))
		case p.accept("niche"):
			p.next() // host
			h := []*Term{Name("host")}
			for p.peek().kind == tString || p.is("expr") || p.is("stmt") {
				t := p.next()
				if t.kind == tString {
					h = append(h, Str(t.text))
				} else {
					h = append(h, Name(t.text))
				}
			}
			out = append(out, app(Name("niche"), app(h...)))
		case (word == "fact" || word == "lemma") && len(out) == 1 && p.peek().kind == tIdent:
			out = append(out, Name(Unspell(p.next().text)))
		case (word == "fact" || word == "lemma") && p.is("("):
			p.next()
			var ps []*Term
			for !p.is(")") {
				n := p.name()
				p.expect("∈")
				s, err := p.expr()
				if err != nil {
					return nil, err
				}
				ps = append(ps, app(Name(n), p.setDatum(s, "")))
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
			out = append(out, Str(p.next().text))
		case word != "fact" && word != "lemma" && p.peek().kind == tQuoted &&
			p.peekN(1).text != ".." && p.peekN(1).text != "(" && p.peekN(1).text != ".":
			// A quoted name is the name itself, and an operand alone: the
			// operand after it is not subtracted from it.
			out = append(out, Name(p.next().text))
		case word != "fact" && word != "lemma" && p.peek().kind == tIdent && !strings.Contains("0123456789", p.peek().text[:1]) &&
			p.peekN(1).text != ".." && p.peekN(1).text != "(":
			out = append(out, Name(Unspell(p.next().text)))
		case word == "fact" || word == "lemma":
			// A FACT'S OPERANDS are formulas: its guard and its conclusion.
			e, err := p.expr()
			if err != nil {
				return nil, err
			}
			out = append(out, datum(e))
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

// suffixOf is a name's prime, "'" or "".
func suffixOf(n string) string {
	if strings.HasSuffix(n, "'") {
		return "'"
	}
	return ""
}

// isBufferSet reports a result written `buffer(…)`, the old threaded spelling
// of a borrowed buffer handed back.
func isBufferSet(e Expr) bool {
	a, ok := e.(*Apply)
	if !ok {
		return false
	}
	v, ok := a.Fn.(*Var)
	return ok && v.Name == "buffer"
}
