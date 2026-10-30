package core

import (
	"reflect"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// parse_J, spec/j.md §2–3: J's text read into abstract.md's trees
// (core/abs.go), from which D gives the core term (abstract.md §9.1, R2). Where
// J has one form for what today's notation wrote two ways, the tree is the one
// whose D-image is the same (spec/j.md §7); today's reader is deleted (step 5b).

type parser struct {
	toks   []token
	i      int
	parens []bool // the open brackets: true for ( and [, where a line break is a space

	sets    map[string]string // a set's J name → its name in the abstract syntax (Result → result)
	folded  map[Expr]*big.Int
	indexed map[Expr]bool // applications written as indexing, t[i]
	resultR string        // the name an `ensure` gives the result binder
	at      int
	x86     bool // reading x86's model, whose templates number their holes
	// grouped are the expressions written in parentheses: `(A × B) × C` is a
	// pair whose first component is a pair, TLA+'s product, where the chain
	// `A × B × C` is a triple (ADR 0065).
	grouped map[Expr]bool
	// setDefs are the module's set definitions, `set Head = S`, by J name: an
	// explicit definition, eliminated by unfolding where it is used (Padoa's
	// eliminability: a definitional extension is conservative), so a header
	// names its own set and the checker sees the set it denotes (ADR 0084).
	setDefs map[string]Expr
	// resultHoles name a host template's destinations by its result binders,
	// for a group result: `${n}` is %r0 when n is the first (jreadtarget.go).
	resultHoles map[string]string
	// outScopes are the scopes outTables made, by the scope: the region's
	// name and the call lent it.
	outScopes map[*Local]outScope
}

type outScope struct {
	region string
	call   Expr
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) peekN(n int) token {
	if p.i+n < len(p.toks) {
		return p.toks[p.i+n]
	}
	return p.toks[len(p.toks)-1]
}

// next consumes a token; the end of input is never consumed, so a parse that
// reads past it reports where it stands rather than reading beyond the tokens.
func (p *parser) next() token {
	t := p.toks[p.i]
	if t.kind != tEOF {
		p.i++
	}
	return t
}

func (p *parser) is(text string) bool {
	t := p.peek()
	return (t.kind == tSym || t.kind == tIdent) && t.text == text
}

func (p *parser) accept(text string) bool {
	if p.is(text) {
		p.next()
		return true
	}
	return false
}

func (p *parser) expect(text string) error {
	if !p.accept(text) {
		return p.errorf("%s where %q was expected", p.peek(), text)
	}
	return nil
}

func (p *parser) errorf(f string, args ...any) error {
	return fmt.Errorf("line %d: "+f, append([]any{p.peek().line}, args...)...)
}

// inParens reports whether a line break is a space here: inside ( or [.
func (p *parser) inParens() bool { return len(p.parens) > 0 && p.parens[len(p.parens)-1] }

func (p *parser) open(paren bool) { p.parens = append(p.parens, paren) }
func (p *parser) close()          { p.parens = p.parens[:len(p.parens)-1] }

// lineEnds reports whether a line break before the next token ends what is
// being read (spec/j.md §1.7).
func (p *parser) lineEnds() bool { return p.peek().nl && !p.inParens() }

// endOfStatement consumes the separators after a statement: `;` or line breaks.
func (p *parser) endOfStatement() {
	for p.accept(";") {
	}
}

// ---- declarations ----

// ReadProgram reads a program in J into declaration trees. module is the path
// a library file is found at, whose header J does not write (ADR 0064 item 1),
// and "" for an entry file; root is the repository, whose lib/ holds the
// modules imported, to know which of their names are sets.
func ReadProgram(src, module, dir, root string) ([]Decl, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, sets: map[string]string{"Option": "option"}, folded: map[Expr]*big.Int{},
		indexed: map[Expr]bool{}}
	p.registerSets(src, dir, root)
	var decls []Decl
	if module != "" {
		decls = append(decls, &ModuleDecl{Path: module, At: p.nextAt()})
	}
	items, marked, err := p.items()
	if err != nil {
		return nil, err
	}
	decls = append(decls, p.interfaceDecls(module != "", marked)...)
	decls = append(decls, items...)
	return decls, nil
}

func (p *parser) nextAt() int { p.at++; return p.at }

// registerSets knows a set's J name from its declaration in this file or in an
// imported library: `set OsError = …` names `os-error`.
func (p *parser) registerSets(src, dir, root string) {
	toks := p.toks
	for i := 0; i+1 < len(toks); i++ {
		if toks[i].kind == tIdent && toks[i].text == "set" && toks[i+1].kind == tIdent && declStart(toks, i) && isVariantAt(toks, i+2) {
			p.sets[toks[i+1].text] = unsetName(toks[i+1].text)
		}
		if toks[i].kind == tIdent && toks[i].text == "use" && toks[i].nl {
			path, alias := "", ""
			j := i + 1
			for ; j < len(toks) && (toks[j].kind == tIdent || toks[j].text == "/") && !(j > i+1 && toks[j].nl); j++ {
				path += toks[j].text
			}
			if j+1 < len(toks) && toks[j].text == "as" && !toks[j].nl {
				alias = toks[j+1].text
			}
			if alias == "" {
				alias = path[strings.LastIndex(path, "/")+1:]
			}
			for _, name := range importedVariants(path, dir, root) {
				p.sets[Spell(alias)+"."+name] = alias + "." + unsetName(name)
			}
		}
	}
}

// declStart reports whether toks[i] begins a declaration: it begins a line, or
// it follows the markers that begin one, `pub` and `export` (ADR 0078).
func declStart(toks []token, i int) bool {
	for j := i; j >= 0; j-- {
		if toks[j].nl || j == 0 {
			return true
		}
		if k := toks[j-1]; k.kind != tIdent || (k.text != "pub" && k.text != "export") {
			return false
		}
	}
	return false
}

// importedVariants is the variant set names a module declares, as J spells
// them, read from its source in J: lib/PATH.oro, or PATH.oro beside the file.
func importedVariants(path, dir, root string) []string {
	for _, file := range []string{filepath.Join(root, "lib", path+".oro"), filepath.Join(dir, filepath.Base(path)+".oro")} {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		toks, err := lex(string(b))
		if err != nil {
			return nil
		}
		var out []string
		for i := 0; i+1 < len(toks); i++ {
			if toks[i].kind == tIdent && toks[i].text == "set" && toks[i+1].kind == tIdent && declStart(toks, i) && isVariantAt(toks, i+2) {
				out = append(out, toks[i+1].text)
			}
		}
		return out
	}
	return nil
}

// unsetName inverts SetName on a variant's name: OsError is os-error.
func unsetName(s string) string {
	for t, l := range langSets {
		if l == s {
			return t
		}
	}
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('-')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// items reads declarations to the end of the file or a closing brace. In a
// "pub" the names marked `pub`, the module's interface, and "export" the ones
// marked `export`, its entry points (ADR 0078).
func (p *parser) items() ([]Decl, map[string][]string, error) {
	var out []Decl
	marked := map[string][]string{}
	for {
		p.endOfStatement()
		if p.peek().kind == tEOF || p.is("}") {
			return out, marked, nil
		}
		ds, names, err := p.item()
		if err != nil {
			return nil, nil, err
		}
		out = append(out, ds...)
		for k, v := range names {
			marked[k] = append(marked[k], v...)
		}
	}
}

// interfaceDecls are a module's two relations (ADR 0078): its interface, always
// present for a library or a nested module so that an empty one hides every
// name, and its entry points when it marks any.
func (p *parser) interfaceDecls(library bool, marked map[string][]string) []Decl {
	var out []Decl
	if library || len(marked["pub"]) > 0 {
		out = append(out, &PubDecl{Names: marked["pub"], At: p.nextAt()})
	}
	if len(marked["export"]) > 0 {
		out = append(out, &ExportDecl{Names: marked["export"], At: p.nextAt()})
	}
	return out
}

func (p *parser) item() ([]Decl, map[string][]string, error) {
	marked := map[string][]string{}
	pub, export := false, false
	// THE MARKERS ARE A SET, each at most once, in either order (ADR 0078):
	// `pub` puts a name in the interface, `export` makes it an entry point.
	for {
		switch {
		case p.is("private"):
			return nil, nil, p.errorf("`private` is the default: a declaration is private to its module " +
				"unless it is marked `pub` (ADR 0078)")
		case p.is("pub") && !pub && p.peekN(1).kind == tIdent:
			p.next()
			pub = true
			continue
		case p.is("export") && !export:
			p.next()
			export = true
			continue
		}
		break
	}
	public := func(names ...string) {
		if pub {
			marked["pub"] = append(marked["pub"], names...)
		}
		if export {
			marked["export"] = append(marked["export"], names...)
		}
	}
	t := p.peek()
	switch {
	case p.is("use"):
		p.next()
		path := p.path()
		u := &UseDecl{Path: path, At: p.nextAt()}
		if p.accept("as") {
			u.As = p.next().text
		}
		return []Decl{u}, nil, nil
	case p.is("module"):
		p.next()
		path := p.path()
		if err := p.expect("{"); err != nil {
			return nil, nil, err
		}
		p.open(false)
		m := &ModuleDecl{Path: path, At: p.nextAt()}
		inner, names, err := p.items()
		if err != nil {
			return nil, nil, err
		}
		p.close()
		if err := p.expect("}"); err != nil {
			return nil, nil, err
		}
		out := append([]Decl{m}, p.interfaceDecls(true, names)...)
		return append(out, inner...), nil, nil
	case p.is("fn") && p.peekN(1).text != "(":
		p.next()
		name := p.name()
		claimAt := p.nextAt()
		lam, err := p.lamHeader()
		if err != nil {
			return nil, nil, err
		}
		if lam.Claim != nil {
			lam.Claim.At = claimAt
		}
		public(name)
		return []Decl{&DefDecl{Name: name, Value: lam, At: p.nextAt()}}, marked, nil
	case p.is("set") && p.peekN(1).kind == tIdent && p.peekN(2).text == "=" && p.peekN(3).kind != tTag:
		// A SET DEFINITION, `set Head = S`: ⟦Head⟧ = ⟦S⟧, unfolded where it is
		// used. Defined before its uses, as TLA+'s are, so the unfolding is the
		// order the module is read in.
		p.next()
		jn := p.next().text
		p.next()
		if pub || export {
			return nil, nil, p.errorf("set %s is a definition unfolded where its module uses it; naming "+
				"it from another module is the algebra of sorts' (spec/typevalues.md §10), not built", jn)
		}
		if _, dup := p.setDefs[jn]; dup {
			return nil, nil, p.errorf("set %s is defined twice", jn)
		}
		e, err := p.expr()
		if err != nil {
			return nil, nil, err
		}
		s := p.set(e)
		if err := p.nestedSep(s, true); err != nil {
			return nil, nil, err
		}
		if p.setDefs == nil {
			p.setDefs = map[string]Expr{}
		}
		p.setDefs[jn] = s
		return nil, marked, nil
	case p.is("set") && p.peekN(1).kind == tIdent:
		p.next()
		v, err := p.variant()
		if err != nil {
			return nil, nil, err
		}
		for _, it := range v.Items {
			if it.Ctor != "" {
				public(it.Ctor)
			}
		}
		return []Decl{v}, marked, nil
	case (t.kind == tIdent || t.kind == tQuoted) && p.peekN(1).text == "=" && p.peekN(1).kind == tSym:
		name := p.name()
		p.next()
		e, err := p.expr()
		if err != nil {
			return nil, nil, err
		}
		public(name)
		return []Decl{&DefDecl{Name: name, Value: e, At: p.nextAt()}}, marked, nil
	}
	// A FILE IS A THEORY (ADR 0064): it holds declarations, and a bare
	// expression at the top level is none — read as one, junk became terms.
	return nil, nil, p.errorf("%s begins no declaration; a file holds fn, set, use, module and NAME = e "+
		"(ADR 0064)", t)
}

func (p *parser) path() string {
	// A module path is a file's path under its library (ADR 0025, 0064): its
	// segments are separated by `/`, and a segment may hold a `-`, as a file
	// name may.
	var b strings.Builder
	for {
		t := p.next()
		b.WriteString(t.text)
		if (!p.is("/") && !p.is("-")) || p.peek().nl {
			return b.String()
		}
		b.WriteString(p.next().text)
	}
}

// name reads a defined name: an identifier, σ_J⁻¹, or a quoted name.
func (p *parser) name() string {
	t := p.next()
	if t.kind == tQuoted {
		return t.text
	}
	return Unspell(t.text)
}

// lamHeader reads a definition's parameters, result, clauses and body: a λ
// carrying its claims (ADR 0051), the claim present when the header states a
// set, a result, a precondition or a postcondition.
func (p *parser) lamHeader() (*Lam, error) {
	line := p.peek().line
	if err := p.expect("("); err != nil {
		return nil, err
	}

	p.open(true)
	lam := &Lam{}
	claim := &Claim{}
	claimed := false
	for !p.is(")") {
		n := p.name()
		lam.Params = append(lam.Params, n)
		cp := ClaimParam{Name: n}
		if p.accept("∈") {
			s, err := p.expr()
			if err != nil {
				return nil, err
			}
			cp.Set = s
			claimed = true
		}
		mode := ""
		if p.is("linear") || p.is("inout") {
			mode = p.next().text
		}
		if mode == "linear" {
			// A BORROWED PARAMETER is a place in the body (ADR 0052 item 3).
			lam.Borrows = append(lam.Borrows, n)
		}
		cp.Set = p.toSet(cp.Set, mode)
		if err := p.nestedSep(cp.Set, true); err != nil {
			return nil, err
		}
		claim.Params = append(claim.Params, cp)
		if !p.accept(",") {
			break
		}
	}
	p.close()
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	if err := distinct(lam.Params, "fn", line); err != nil {
		return nil, err
	}
	if p.is("(") {
		p.next()
		p.open(true)
		claimed = true
		if p.is(")") {
			claim.Result = &Apply{Fn: &Var{Name: "tuple"}}
		} else {
			var parts []Expr
			for {
				if p.peek().kind == tIdent && p.peekN(1).text == "∈" {
					p.resultR = p.next().text
					p.next()
				}
				s, err := p.expr()
				if err != nil {
					return nil, err
				}
				// A LINEAR RESULT, `(r ∈ Array(V) linear)`: a writable region the
				// definition owns and moves to its caller (ADR 0084).
				if p.is("linear") {
					p.next()
					claim.ResultModes = append(claim.ResultModes, Linear)
				}
				parts = append(parts, p.toSet(s, ""))
				if err := p.nestedSep(parts[len(parts)-1], true); err != nil {
					return nil, err
				}
				if !p.accept(",") {
					break
				}
			}
			claim.Result = parts[0]
			if len(parts) > 1 {
				claim.Result = &Apply{Fn: &Var{Name: "tuple"}, Args: parts}
			}
		}
		p.close()
		if err := p.expect(")"); err != nil {
			return nil, err
		}
	}
	for {
		switch {
		case p.accept("where"):
			e, err := p.expr()
			if err != nil {
				return nil, err
			}
			claim.Pre, claimed = e, true
			continue
		case p.accept("ensure"):
			e, err := p.expr()
			if err != nil {
				return nil, err
			}
			if p.resultR != "" {
				e = renameExpr(e, map[string]string{Unspell(p.resultR): ResultName})
			}
			claim.Post, claimed = e, true
			continue
		}
		break
	}
	p.resultR = ""
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	e, ok := body.(Expr)
	if !ok {
		return nil, p.errorf("a definition's body ends in a jump")
	}
	lam.Body = e
	if claimed {
		lam.Claim = claim
	}
	if len(claim.ResultModes) == 1 && claim.ResultModes[0] == Linear {
		lam.Body = &Apply{Fn: &Var{Name: OwnedName}, Args: []Expr{e}}
	}
	return lam, nil
}

// variant reads `set N(T…) = 'a ∪ 'c(S) … success 'a relevant` (ADR 0064 item 8).
func (p *parser) variant() (*VariantDecl, error) {
	jname := p.next().text
	v := &VariantDecl{Name: unsetName(jname)}
	if p.accept("(") {
		for !p.is(")") {
			v.Params = append(v.Params, p.next().text)
			p.accept(",")
		}
		p.next()
	}
	if err := p.expect("="); err != nil {
		return nil, err
	}
	v.At = p.nextAt()
	for {
		if p.peek().kind != tTag {
			return nil, p.errorf("%s where a tag was expected in a set of tags", p.peek())
		}
		it := VariantItem{Ctor: Unspell(p.next().text)}
		if p.is("(") && !p.lineEnds() {
			p.next()
			p.open(true)
			s, err := p.expr()
			if err != nil {
				return nil, err
			}
			it.Payload = p.toSet(s, "")
			p.close()
			if err := p.expect(")"); err != nil {
				return nil, err
			}
		}
		v.Items = append(v.Items, it)
		if !p.accept("∪") {
			break
		}
	}
	for !p.lineEnds() && !p.is("}") && !p.is(";") && p.peek().kind != tEOF {
		switch {
		case p.accept("success"):
			v.Items = append(v.Items, VariantItem{Success: Unspell(p.next().text)})
		case p.accept("relevant"):
			v.Items = append(v.Items, VariantItem{Relevant: true})
			if !p.lineEnds() && !p.is("success") && !p.is("relevant") && !p.is("}") && !p.is(";") && p.peek().kind != tEOF {
				return nil, p.errorf("variant %s: (relevant) takes nothing; got %s", v.Name, p.peek())
			}
		default:
			// The line is the declaration: a token after its tags and markers
			// is no part of it, and must not be read as the next item.
			return nil, p.errorf("variant %s: %s after its tags; a variant's markers are success 'tag and relevant", v.Name, p.peek())
		}
	}
	return v, nil
}

// toSet reads a set expression back into today's spelling of a set: the
// language's sets by name (Int is int), a range, a product, a variant's set by
// its declared name, and a mode on a binder as today's former: Array(V) linear
// is (buffer V), T inout is (cell T) (ADR 0050 item 2).
func (p *parser) toSet(e Expr, mode string) Expr {
	if e == nil {
		return nil
	}
	out := p.set(e)
	switch mode {
	case "linear":
		if a, ok := out.(*Apply); ok && len(a.Args) == 1 {
			if v, ok := a.Fn.(*Var); ok && v.Name == "array" {
				return &Apply{Fn: &Var{Name: "buffer"}, Args: a.Args}
			}
		}
	case "inout":
		return &Apply{Fn: &Var{Name: "cell"}, Args: []Expr{out}}
	}
	return out
}

func (p *parser) set(e Expr) Expr {
	switch e := e.(type) {
	case *Var:
		if e.Name == NatSet {
			// ℕ, TLA+'s Nat, is ↑0 by definition: `0..`, the core's (int 0 +inf).
			// A set names no representation (ADR 0079), so this is ℕ and only ℕ.
			return &Apply{Fn: &Var{Name: "int"}, Args: []Expr{&Lit{T: IntLiteral("0")}, &Var{Name: "+inf"}}}
		}
		if d, ok := p.setDefs[e.Name]; ok {
			return d
		}
		return &Var{Name: p.setName(e.Name)}
	case *Apply:
		v, ok := e.Fn.(*Var)
		if !ok {
			return e
		}
		if v.Name == SepName && len(e.Args) == 3 {
			// { x ∈ S : φ }: the binder and the formula are no sets.
			return &Apply{Fn: v, Args: []Expr{e.Args[0], p.set(e.Args[1]), e.Args[2]}}
		}
		args := make([]Expr, len(e.Args))
		for i, a := range e.Args {
			args[i] = p.set(a)
		}
		switch v.Name {
		case "..":
			return &Apply{Fn: &Var{Name: "int"}, Args: e.Args}
		case "×":
			return &Apply{Fn: &Var{Name: "tuple"}, Args: args}
		case "tuple":
			return &Apply{Fn: v, Args: args}
		}
		return &Apply{Fn: &Var{Name: p.setName(v.Name)}, Args: args}
	}
	return e
}

// setName is a set's name in the abstract syntax: the language's and the
// variants' by the convention's inverse, a host's as written.
func (p *parser) setName(n string) string {
	if t, ok := p.sets[n]; ok {
		return t
	}
	for t, l := range langSets {
		if l == n {
			return t
		}
	}
	return n
}

// ---- blocks and statements ----

type stmt struct {
	kind  string // "bind", "var", "buffer", "map", "expect", "expr"
	pat   Pat
	name  string
	value Tail
	items []Expr
}

// block reads `{ statements }`: a scope (ADR 0054), folded from its last
// statement as μ builds today's let, seq, local and the handled bind.
func (p *parser) block() (Tail, error) {
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	p.open(false)
	var stmts []stmt
	for {
		p.endOfStatement()
		if p.is("}") {
			break
		}
		s, err := p.statement()
		if err != nil {
			return nil, err
		}
		stmts = append(stmts, s)
		if !p.is("}") && !p.is(";") && !p.peek().nl {
			return nil, p.errorf("%s after a statement; a statement ends at a line break or `;`", p.peek())
		}
	}
	p.close()
	p.next()
	if len(stmts) == 0 {
		return &Apply{Fn: &Var{Name: "tuple"}}, nil
	}
	return p.foldStmts(stmts)
}

func (p *parser) foldStmts(stmts []stmt) (Tail, error) {
	last := stmts[len(stmts)-1]
	if last.kind != "expr" {
		return nil, fmt.Errorf("a block ends in a %s, not a value", last.kind)
	}
	acc := last.value
	for i := len(stmts) - 2; i >= 0; i-- {
		s := stmts[i]
		body, isExpr := acc.(Expr)
		switch s.kind {
		case "bind":
			// AN OUT TABLE BOUND BY ITS PAIR, `(buf, r) = f(…, out S, …)`: the
			// rest of the block runs inside the region's scope, the region named
			// buf there, so nothing but the block's value crosses the scope's
			// boundary (outTables).
			if loc, ok := s.value.(*Local); ok && isExpr {
				if o, ok := p.outScopes[loc]; ok {
					if pt, ok := s.pat.(*PTuple); ok && len(pt.Names) == 2 {
						// The region takes the program's name, and the call's
						// argument is renamed to it.
						if mentions(o.call, pt.Names[0]) {
							return nil, fmt.Errorf("%s names both the out table and an argument of its call",
								pt.Names[0])
						}
						bind := loc.Binds[0]
						bind.Name = pt.Names[0]
						call := renameExpr(o.call, map[string]string{o.region: pt.Names[0]})
						acc = &Local{Binds: []LocalBind{bind}, Body: &Let{Binds: []LetBind{{Pat: &PVar{Name: pt.Names[1]},
							Value: call}}, Body: body}}
						continue
					}
				}
			}
			b := LetBind{Pat: s.pat, Value: s.value.(Expr)}
			if isExpr {
				acc = &Let{Binds: []LetBind{b}, Body: body}
			} else {
				acc = &TLet{Binds: []LetBind{b}, Body: acc}
			}
		case "expr":
			e, ok := s.value.(Expr)
			if !ok {
				return nil, fmt.Errorf("a jump stands before the end of its block")
			}
			if isExpr {
				if seq, ok := body.(*Seq); ok {
					acc = &Seq{Xs: append([]Expr{e}, seq.Xs...)}
				} else {
					acc = &Seq{Xs: []Expr{e, body}}
				}
			} else if ts, ok := acc.(*TSeq); ok {
				acc = &TSeq{Xs: append([]Expr{e}, ts.Xs...), Last: ts.Last}
			} else {
				acc = &TSeq{Xs: []Expr{e}, Last: acc}
			}
		case "var", "buffer":
			if !isExpr {
				return nil, fmt.Errorf("local state around a jump")
			}
			init := s.value.(Expr)
			// A STATE BINDER HOLDING A FRESH TABLE IS A BUFFER, in program order
			// (ADR 0052, spec/local.md §2.1); and so is one its scope writes
			// element by element, `b[i] := v`, whatever its initializer: a fresh
			// region holding the initializer's value, moved in when that value is
			// itself a fresh region (ADR 0084: a linear result), copied in
			// otherwise (scope.go, adopt). Any other state binder is a cell.
			acc = &Local{Binds: []LocalBind{{Name: s.name, Init: init, Buffer: s.kind == "buffer",
				Place: s.kind == "var" && (freshTable(init) || writesElement(body, s.name))}}, Body: body}
		case "map", "mapvar":
			if !isExpr {
				return nil, fmt.Errorf("a map buffer around a jump")
			}
			acc = &BuildMap{Names: []string{s.name}, Caps: []Expr{s.value.(Expr)}, Body: body, Place: s.kind == "mapvar"}
		case "expect":
			if isExpr {
				acc = &Monad{Form: "expect", Items: s.items, Body: body}
			} else {
				acc = &TMonad{Form: "expect", Items: s.items, Body: acc}
			}
		}
	}
	return acc, nil
}

func (p *parser) statement() (stmt, error) {
	t := p.peek()
	switch {
	case p.accept("var"):
		name := p.name()
		if err := p.expect(":="); err != nil {
			return stmt{}, err
		}
		if p.is("map") && p.peekN(1).text == "(" {
			// A MAP BUFFER, a place whose value is a map (ADR 0058).
			p.next()
			p.next()
			p.open(true)
			c, err := p.expr()
			p.close()
			if err != nil {
				return stmt{}, err
			}
			return stmt{kind: "mapvar", name: name, value: c}, p.expect(")")
		}
		e, err := p.expr()
		return stmt{kind: "var", name: name, value: e}, err
	case p.accept("expect"):
		pat, err := p.exprAt(pCmp + 1)
		if err != nil {
			return stmt{}, err
		}
		if err := p.expect("="); err != nil {
			return stmt{}, err
		}
		v, err := p.expr()
		if err != nil {
			return stmt{}, err
		}
		if err := p.expect("else"); err != nil {
			return stmt{}, err
		}
		w, err := p.expr()
		return stmt{kind: "expect", items: []Expr{pat, v, w}}, err
	case (t.kind == tIdent || t.kind == tQuoted) && p.peekN(1).text == "linear" && p.peekN(2).text == "=":
		name := p.name()
		p.next()
		p.next()
		if p.is("map") && p.peekN(1).text == "(" {
			p.next()
			p.next()
			p.open(true)
			c, err := p.expr()
			p.close()
			if err != nil {
				return stmt{}, err
			}
			return stmt{kind: "map", name: name, value: c}, p.expect(")")
		}
		e, err := p.expr()
		return stmt{kind: "buffer", name: name, value: e}, err
	case (t.kind == tIdent || t.kind == tQuoted) && p.peekN(1).text == "∈":
		// A BINDER WITH ITS SET, `b ∈ S = e` (spec/j.md §5): b bound to the(S; e),
		// a claim on the value, an obligation discharged as a declared range is
		// (ADR 0028, desugar.go), and once proven the set the IR reads of b
		// (ownbuf-2026-10-21). Today's reader had the form; J's reader lacked it.
		name := p.name()
		p.next()
		set, err := p.expr()
		if err != nil {
			return stmt{}, err
		}
		if err := p.expect("="); err != nil {
			return stmt{}, err
		}
		e, err := p.expr()
		if err != nil {
			return stmt{}, err
		}
		return stmt{kind: "bind", pat: &PVar{Name: name}, value: &The{Set: p.toSet(set, ""), E: e, Name: name}}, nil
	case (t.kind == tIdent || t.kind == tQuoted) && p.peekN(1).text == "=" && p.peekN(1).kind == tSym:
		name := p.name()
		p.next()
		e, err := p.expr()
		var pat Pat = &PVar{Name: name}
		if name == "-" || t.text == "_" {
			pat = &PWild{}
		}
		return stmt{kind: "bind", pat: pat, value: e}, err
	case (t.kind == tIdent || t.kind == tQuoted) && p.isWrite():
		w, err := p.write()
		return stmt{kind: "expr", value: w}, err
	case p.is("(") && p.tupleBinding():
		p.next()
		var names []string
		for !p.is(")") {
			names = append(names, p.name())
			p.accept(",")
		}
		p.next()
		p.next() // =
		e, err := p.expr()
		var pat Pat = &PTuple{Names: names}
		if len(names) == 1 {
			pat = &PVar{Name: names[0]}
		}
		if err == nil {
			err = distinct(names, "a tuple pattern", t.line)
		}
		return stmt{kind: "bind", pat: pat, value: e}, err
	}
	e, err := p.tail()
	return stmt{kind: "expr", value: e}, err
}

// freshTable reports a table former, `array { … }`: what a buffer's binder and
// whole write may hold (R0).
func freshTable(e Expr) bool {
	if a, ok := e.(*Apply); ok {
		if v, ok := a.Fn.(*Var); ok {
			return v.Name == "table" || v.Name == "array"
		}
	}
	return false
}

// isWrite reports whether a statement is a write: places, `x`, `b[i]`,
// `b[i][j]`, separated by commas, then `:=` (spec/j.md §2.6).
func (p *parser) isWrite() bool {
	k := p.i
	for {
		if t := p.toks[k]; t.kind != tIdent && t.kind != tQuoted {
			return false
		}
		k++
		for p.toks[k].kind == tSym && p.toks[k].text == "[" {
			depth := 0
			for ; k < len(p.toks); k++ {
				switch p.toks[k].text {
				case "[":
					depth++
				case "]":
					depth--
				}
				if depth == 0 {
					break
				}
			}
			k++
		}
		switch t := p.toks[k]; {
		case t.kind == tSym && t.text == ":=":
			return true
		case t.kind == tSym && t.text == ",":
			k++
		default:
			return false
		}
	}
}

// write reads places, `:=`, and as many values: one write, or a concurrent
// assignment, every value in the state before (spec/local.md §2.1).
func (p *parser) write() (*Write, error) {
	w := &Write{}
	for {
		pl := Place{Name: p.name()}
		for p.accept("[") {
			p.open(true)
			i, err := p.expr()
			p.close()
			if err != nil {
				return nil, err
			}
			if err := p.expect("]"); err != nil {
				return nil, err
			}
			pl.At = append(pl.At, i)
		}
		w.Places = append(w.Places, pl)
		if !p.accept(",") {
			break
		}
	}
	if err := p.expect(":="); err != nil {
		return nil, err
	}
	for {
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		w.Values = append(w.Values, e)
		if !p.accept(",") {
			break
		}
	}
	if len(w.Values) != len(w.Places) {
		return nil, p.errorf("%d places and %d values in one assignment", len(w.Places), len(w.Values))
	}
	return w, nil
}

// tupleBinding reports whether `(` begins a tuple pattern bound by `=`.
func (p *parser) tupleBinding() bool {
	for k := p.i + 1; k < len(p.toks); k++ {
		t := p.toks[k]
		switch {
		case t.kind == tSym && t.text == ")":
			n := p.toks[k+1]
			return n.kind == tSym && n.text == "="
		case t.kind == tIdent || t.kind == tQuoted || (t.kind == tSym && t.text == ","):
		default:
			return false
		}
	}
	return false
}

// ---- expressions ----

var binary = map[string]int{
	"||": pOr, "&&": pAnd,
	"==": pCmp, "!=": pCmp, "<": pCmp, "<=": pCmp, ">": pCmp, ">=": pCmp, "∈": pCmp, "∉": pCmp, "⊆": pCmp, "⊂": pCmp,
	"∪": pSet, "∩": pSet, "∖": pSet, "×": pProd, "..": pRange,
	"+": pAdd, "-": pAdd, "++": pAdd, "*": pMul, "/": pMul, "%": pMul, "^": pPow,
	// The Boolean operations and the shifts at Go's levels (ADR 0082 item 6):
	// `x & 1 == 0` is `(x & 1) == 0`. `mod` is a word, read as an operator only
	// between two operands (binaryFrom).
	"<<": pMul, ">>": pMul, "&": pMul, "mod": pMul, "|": pAdd, "⊕": pAdd,
}

var opName = map[string]string{"==": "=", "++": "concat"}

func (p *parser) expr() (Expr, error) { return p.exprAt(pLowest) }

// tail reads an expression or a jump.
func (p *parser) tail() (Tail, error) {
	if p.is("again") {
		p.next()
		args, err := p.args()
		return &Again{Args: args}, err
	}
	if p.is("{") {
		return p.block()
	}
	if p.is("try") {
		return p.try()
	}
	// A CASE IN A TAIL may have arms that are tails (ADR 0076). One whose arms
	// are all expressions is an expression, and the expression goes on from it.
	if p.is("case") && p.peekN(1).text != "{" {
		p.next()
		t, err := p.caseAny()
		if err != nil {
			return nil, err
		}
		if e, ok := t.(Expr); ok {
			if e, err = p.postfix(e); err != nil {
				return nil, err
			}
			return p.binaryFrom(e, pLowest)
		}
		// A tail is no operand: what continues an expression after it would make
		// a jump inside one.
		if n := p.peek(); !p.lineEnds() && n.kind == tSym {
			if _, op := binary[n.text]; op || n.text == "(" || n.text == "[" || n.text == "." {
				return nil, p.errorf("a case with an arm that jumps is a tail, and stands only where a tail " +
					"may, a clause's body or the body of a binding in one (ADR 0076)")
			}
		}
		return t, nil
	}
	return p.expr()
}

func (p *parser) exprAt(min int) (Expr, error) {
	left, err := p.unary()
	if err != nil {
		return nil, err
	}
	return p.binaryFrom(left, min)
}

// binaryFrom reads the binary operators that follow an operand already read.
func (p *parser) binaryFrom(left Expr, min int) (Expr, error) {
	for {
		t := p.peek()
		prec, ok := binary[t.text]
		if t.kind != tSym && !(t.kind == tIdent && t.text == "mod") || !ok || prec < min || p.lineEnds() {
			return left, nil
		}
		p.next()
		if t.text == ".." && !p.beginsOperand() {
			// lo.. is ↑lo: the range with its upper bound absent (ADR 0077),
			// the core's (int lo +inf).
			left = &Apply{Fn: &Var{Name: ".."}, Args: []Expr{left, &Var{Name: "+inf"}}}
			continue
		}
		next := prec + 1
		if t.text == "^" || t.text == "&&" || t.text == "||" {
			// `^` is right-associative; `&&` and `||` are associative, and read
			// right-nested they are today's n-ary connective (core/desugar.go).
			next = prec
		}
		right, err := p.exprAt(next)
		if err != nil {
			return nil, err
		}
		left = p.combine(t.text, left, right)
	}
}

// rangeEnds are the header's contextual words (j.md §2.4), which may follow a
// range in a binder and would otherwise be read as its bound: `n ∈ 0.. inout`.
var rangeEnds = map[string]bool{
	"where": true, "ensure": true, "pure": true, "index": true, "variadic": true,
	"linear": true, "inout": true, "out": true,
}

// beginsOperand reports whether the next token can begin a range's bound. When
// it cannot, the bound is absent (ADR 0077 item 2): a line end, the file's end,
// a symbol that begins no expression, or a word of the header's grammar.
func (p *parser) beginsOperand() bool {
	t := p.peek()
	if p.lineEnds() {
		return false
	}
	switch t.kind {
	case tEOF:
		return false
	case tSym:
		switch t.text {
		case "(", "{", "-", "!", "~":
			return true
		}
		return false
	case tIdent:
		return !rangeEnds[t.text]
	}
	return true
}

// combine builds a binary operation. `^` folds, as do `+` and `-` beside a
// folded power: a power is a constant (ADR 0067 item 4), and `2^62 - 1` is the
// literal it was written for.
func (p *parser) combine(op string, l, r Expr) Expr {
	lv, lf := p.folded[l]
	rv, rf := p.literal(r)
	switch {
	case op == "^":
		// A NEGATIVE OR ABSURD EXPONENT IS NOT FOLDED, so a bound written with
		// one is refused as no compile-time integer: big.Int's Exp gives 1 for a
		// negative exponent, and the cap — 2^1000000 is far past any
		// representation anyone will ask for — keeps a typo from asking the
		// reader for a gigabyte-long integer.
		base, bok := p.literal(l)
		if bok && rf && rv.Sign() >= 0 && rv.IsInt64() && rv.Int64() <= 1000000 {
			v := new(big.Int).Exp(base, rv, nil)
			return p.fold(v)
		}
	case (op == "-" || op == "+") && lf && rf:
		v := new(big.Int)
		if op == "-" {
			v.Sub(lv, rv)
		} else {
			v.Add(lv, rv)
		}
		return p.fold(v)
	case op == "×":
		// A product chain is one n-ary product; a parenthesized one is a
		// component (p.grouped).
		if a, ok := l.(*Apply); ok && !p.grouped[l] {
			if v, ok := a.Fn.(*Var); ok && v.Name == "×" {
				return &Apply{Fn: v, Args: append(append([]Expr(nil), a.Args...), r)}
			}
		}
	case op == "&&":
		if a, ok := r.(*And); ok {
			return &And{Xs: append([]Expr{l}, a.Xs...)}
		}
		return &And{Xs: []Expr{l, r}}
	case op == "||":
		if o, ok := r.(*Or); ok {
			return &Or{Xs: append([]Expr{l}, o.Xs...)}
		}
		return &Or{Xs: []Expr{l, r}}
	case op == "!=":
		// a ≠ b is ¬(a = b), a derived operator over the language's equality,
		// which every target has (ADR 0017): erased here, as `!` is. Read as a
		// name it was one nothing binds (shape-2026-10-21).
		return &Not{X: &Apply{Fn: &Var{Name: "="}, Args: []Expr{l, r}}}
	}
	name := op
	if n, ok := opName[op]; ok {
		name = n
	}
	return &Apply{Fn: &Var{Name: name}, Args: []Expr{l, r}}
}

// transitional maps the calls print_J writes for today's constructs an ADR has
// not yet replaced (spec/j.md §7): a threaded store, `store(b, i, v)`, is
// today's three-argument `set`.

func (p *parser) fold(v *big.Int) Expr {
	e := &Lit{T: IntLiteral(v.String())}
	p.folded[e] = v
	return e
}

// literal is an integer literal's value.
func (p *parser) literal(e Expr) (*big.Int, bool) {
	if v, ok := p.folded[e]; ok {
		return v, true
	}
	if l, ok := e.(*Lit); ok && l.T.Kind == KInt {
		return big.NewInt(l.T.Int), true
	}
	return nil, false
}

func (p *parser) unary() (Expr, error) {
	switch {
	case p.is("-"):
		p.next()
		if p.peek().kind == tInt && p.peekN(1).text != "^" {
			v, _ := new(big.Int).SetString(p.next().text, 10)
			v.Neg(v)
			l := &Lit{T: IntLiteral(v.String())}
			return p.postfix(l)
		}
		x, err := p.exprAt(pPrefix)
		if err != nil {
			return nil, err
		}
		if v, ok := p.folded[x]; ok {
			return p.fold(new(big.Int).Neg(v)), nil
		}
		return &Apply{Fn: &Var{Name: "-"}, Args: []Expr{x}}, nil
	case p.is("!"):
		p.next()
		x, err := p.exprAt(pPrefix)
		return &Not{X: x}, err
	case p.is("~"):
		// not on ℤ, −x − 1 (ADR 0082), folded on a literal as `-` is.
		p.next()
		x, err := p.exprAt(pPrefix)
		if err != nil {
			return nil, err
		}
		if v, ok := p.literal(x); ok {
			return p.fold(new(big.Int).Sub(new(big.Int).Neg(v), big.NewInt(1))), nil
		}
		return &Apply{Fn: &Var{Name: "~"}, Args: []Expr{x}}, nil
	case p.is(".."):
		// ..hi is ↓hi, the lower bound absent (ADR 0077), the core's (int -inf hi).
		p.next()
		if !p.beginsOperand() {
			return nil, p.errorf("`..` with neither end is ℤ, which is `Int`; a range omits at most one end (ADR 0077)")
		}
		x, err := p.exprAt(pRange + 1)
		if err != nil {
			return nil, err
		}
		return &Apply{Fn: &Var{Name: ".."}, Args: []Expr{&Var{Name: "-inf"}, x}}, nil
	}
	e, err := p.primary()
	if err != nil {
		return nil, err
	}
	return p.postfix(e)
}

func (p *parser) postfix(e Expr) (Expr, error) {
	// A PRIMED NAME, `x'`: a binder's value on exit (ADR 0066 item 7), read as the
	// name `x'`. Only a host declaration's `ensure` gives it a meaning, the
	// storage a `linear` parameter hands back (ADR 0070, jreadtarget.go); anywhere
	// else it is a name nothing binds.
	if v, ok := e.(*Var); ok && p.peek().kind == tPrime {
		p.next()
		e = &Var{Name: v.Name + "'"}
	}
	for !p.lineEnds() {
		switch {
		case p.is("("):
			args, err := p.args()
			if err != nil {
				return nil, err
			}
			if v, ok := e.(*Var); ok && v.Name == "store" && len(args) == 3 {
				e = &Var{Name: "set"}
			}
			if v, ok := e.(*Var); ok && v.Name == "tuple" && len(args) == 1 {
				return nil, p.errorf("tuple takes no terms, the unit, or two or more; a tuple of one is just the value")
			}
			if call, ok, err := p.outTables(e, args); err != nil {
				return nil, err
			} else if ok {
				e = call
				continue
			}
			e = &Apply{Fn: e, Args: args}
		case p.is("["):
			p.next()
			p.open(true)
			i, err := p.expr()
			if err != nil {
				return nil, err
			}
			if p.accept(":") {
				// A SEGMENT, b[lo:hi] or b[lo:]: the rule k ↦ b[lo + k] on
				// [0, hi − lo), formed where 0 ≤ lo ≤ hi ≤ len b (ADR 0084;
				// amends ADR 0074). The reducer gives it its meaning
				// (SegmentName); here it is only read.
				var hi Expr = &Apply{Fn: &Var{Name: "len"}, Args: []Expr{e}}
				if !p.is("]") {
					if hi, err = p.expr(); err != nil {
						return nil, err
					}
				}
				p.close()
				if err := p.expect("]"); err != nil {
					return nil, err
				}
				e = &Apply{Fn: &Var{Name: SegmentName}, Args: []Expr{e, i, hi}}
				continue
			}
			p.close()
			if err := p.expect("]"); err != nil {
				return nil, err
			}
			e = &Apply{Fn: e, Args: []Expr{i}}
			p.indexed[e] = true
		default:
			return e, nil
		}
	}
	return e, nil
}

// args reads `( arg, … )`: an expression, a spread `xs...`, or an out-cell
// `out S` (spec/j.md G1, G2).
func (p *parser) args() ([]Expr, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}
	p.open(true)
	var out []Expr
	for !p.is(")") {
		var a Expr
		if p.is("out") && (p.peekN(1).kind == tIdent || p.peekN(1).kind == tQuoted) {
			p.next()
			s, err := p.exprAt(pCmp + 1)
			if err != nil {
				return nil, err
			}
			a = &Apply{Fn: &Var{Name: "out"}, Args: []Expr{p.toSet(s, "")}}
		} else {
			e, err := p.expr()
			if err != nil {
				return nil, err
			}
			a = e
			if p.accept("...") {
				a = &Apply{Fn: &Var{Name: "spread"}, Args: []Expr{e}}
			}
		}
		out = append(out, a)
		if !p.accept(",") {
			break
		}
	}
	p.close()
	return out, p.expect(")")
}

func (p *parser) primary() (Expr, error) {
	t := p.peek()
	switch t.kind {
	case tInt:
		p.next()
		return &Lit{T: IntLiteral(t.text)}, nil
	case tFloat:
		p.next()
		f, err := strconv.ParseFloat(t.text, 64)
		return &Lit{T: Float(f)}, err
	case tString:
		p.next()
		return &Lit{T: Str(t.text)}, nil
	case tTag:
		p.next()
		return &Var{Name: Unspell(t.text)}, nil
	case tQuoted:
		if t.text == "+inf" || t.text == "-inf" {
			// AN ABSENT BOUND IS NO ELEMENT (ADR 0077): ↑a is a set of integers,
			// and ±∞ are the ends of the IR's abstract domain, not of a set.
			return nil, p.errorf("`%s` is no integer: an unbounded end is omitted, `lo..` or `..hi` (ADR 0077)", t.text)
		}
		p.next()
		return p.qualified(t.text)
	case tIdent:
		switch t.text {
		case "true", "false":
			p.next()
			return &Lit{T: Bool(t.text == "true")}, nil
		case "fn":
			p.next()
			return p.lambda()
		case "the":
			// THE CORE'S ASCRIPTION IS NOT SURFACE (ADR 0062: "`the` leaves the
			// surface"). Written as a call it reached the core as a trusted claim,
			// `the("int 0 5", 10)` compiling; a claim is `b ∈ S = e`, an obligation
			// (ownbuf-2026-10-21).
			if p.peekN(1).text == "(" {
				return nil, p.errorf("`the` is the core's ascription, not J's (ADR 0062): " +
					"claim a value's set with a binder, `b ∈ S = e`, which is proven or refused")
			}
		case "array":
			if p.peekN(1).text == "{" {
				p.next()
				return p.array()
			}
		case "set":
			if p.peekN(1).text == "{" {
				p.next()
				return p.separation()
			}
		case "map":
			if p.peekN(1).text == "{" {
				p.next()
				return p.mapGraph()
			}
		case "if":
			p.next()
			return p.ifExpr()
		case "case":
			p.next()
			t, err := p.caseAny()
			if err != nil {
				return nil, err
			}
			e, ok := t.(Expr)
			if !ok {
				return nil, p.errorf("a case with an arm that jumps is a tail, and stands only where a tail " +
					"may, a clause's body or the body of a binding in one (ADR 0076)")
			}
			return e, nil
		case "loop":
			p.next()
			return p.loop()
		case "try":
			e, err := p.try()
			if err != nil {
				return nil, err
			}
			if x, ok := e.(Expr); ok {
				return x, nil
			}
			return nil, p.errorf("a try whose body jumps stands where a value is needed")
		}
		p.next()
		return p.qualified(Unspell(t.text))
	case tSym:
		switch t.text {
		case "(":
			p.next()
			p.open(true)
			if p.is(")") {
				p.close()
				p.next()
				return &Apply{Fn: &Var{Name: "tuple"}}, nil
			}
			e, err := p.expr()
			if err != nil {
				return nil, err
			}
			if v, ok := p.folded[e]; ok {
				// A parenthesized power is a literal and no longer a power: `(2^16) - 1`
				// is a subtraction (print.go writes the parentheses).
				e = &Lit{T: IntLiteral(v.String())}
			}
			if p.is(",") {
				parts := []Expr{e}
				for p.accept(",") {
					x, err := p.expr()
					if err != nil {
						return nil, err
					}
					parts = append(parts, x)
				}
				e = &Apply{Fn: &Var{Name: "tuple"}, Args: parts}
			} else {
				if p.grouped == nil {
					p.grouped = map[Expr]bool{}
				}
				p.grouped[e] = true
			}
			p.close()
			return e, p.expect(")")
		case "{":
			b, err := p.block()
			if err != nil {
				return nil, err
			}
			if x, ok := b.(Expr); ok {
				return x, nil
			}
			return nil, p.errorf("a block that jumps stands where a value is needed")
		}
	}
	return nil, p.errorf("%s begins no expression", t)
}

// qualified reads the rest of a qualified name, `a.b`, `go.\`+\“, `result.'ok`.
func (p *parser) qualified(first string) (Expr, error) {
	name := first
	for p.is(".") && !p.peek().nl {
		p.next()
		m := p.next()
		switch m.kind {
		case tIdent, tTag:
			name += "." + Unspell(m.text)
		case tQuoted:
			name += "." + m.text
		default:
			return nil, p.errorf("%s after `.`", m)
		}
	}
	return &Var{Name: name}, nil
}

func (p *parser) lambda() (Expr, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}
	var params []string
	line := p.peek().line
	for !p.is(")") {
		// A λ's parameter is a name: a claim on one is a definition's header
		// (ADR 0051), and a λ's own header, ADR 0066's, is not built.
		if k := p.peek().kind; k != tIdent && k != tQuoted {
			return nil, p.errorf("%s in a λ's parameters, which are names separated by \",\"", p.peek())
		}
		params = append(params, p.name())
		if !p.accept(",") && !p.is(")") {
			return nil, p.errorf("%s in a λ's parameters, which are names separated by \",\"", p.peek())
		}
	}
	p.next()
	if err := distinct(params, "fn", line); err != nil {
		return nil, err
	}
	b, err := p.block()
	if err != nil {
		return nil, err
	}
	e, ok := b.(Expr)
	if !ok {
		return nil, p.errorf("a λ's body is never a tail (ADR 0051)")
	}
	return &Lam{Params: params, Body: e}, nil
}

// array reads `array { e, … }`, a table's graph, or `array { e : k ∈ 0..n-1 }`,
// its rule (ADR 0061): today's (table n f), where a rule that only indexes a
// table is that table, by η (a table is its own rule, tables.md §2.4), and a
// rule whose binder is `_` is a fill.
func (p *parser) array() (Expr, error) {
	p.next() // {
	p.open(true)
	defer p.close()
	if p.accept("}") {
		return &Apply{Fn: &Var{Name: "array"}}, nil
	}
	first, err := p.expr()
	if err != nil {
		return nil, err
	}
	if p.accept(":") {
		k := p.next().text
		if err := p.expect("∈"); err != nil {
			return nil, err
		}
		lo, err := p.exprAt(pRange + 1)
		if err != nil {
			return nil, err
		}
		// A table is a function on [0, n) (tables.md §1): its rule's binder
		// ranges from 0, or the former would mean another domain silently.
		if v, ok := p.literal(lo); !ok || v.Sign() != 0 {
			return nil, p.errorf("a table's index ranges over 0..n-1, so its binder's set starts at 0")
		}
		if err := p.expect(".."); err != nil {
			return nil, err
		}
		hi, err := p.exprAt(pRange + 1)
		if err != nil {
			return nil, err
		}
		if err := p.expect("}"); err != nil {
			return nil, err
		}
		n := p.count(hi)
		var f Expr = first
		if k != "_" {
			if a, ok := first.(*Apply); ok && len(a.Args) == 1 && p.indexed[a] {
				if v, ok := a.Args[0].(*Var); ok && v.Name == Unspell(k) {
					if t, ok := a.Fn.(*Var); ok && t.Name != v.Name {
						return &Apply{Fn: &Var{Name: "table"}, Args: []Expr{n, t}}, nil
					}
				}
			}
			f = &Lam{Params: []string{Unspell(k)}, Body: first}
		} else if _, lit := first.(*Lit); !lit {
			// `_` binds nothing: every element is the one value. A literal is
			// today's fill; anything else is the constant rule, never a rule
			// itself, which (table n e) would read it as.
			f = &Lam{Params: []string{"_"}, Body: first}
		}
		return &Apply{Fn: &Var{Name: "table"}, Args: []Expr{n, f}}, nil
	}
	elems := []Expr{first}
	for p.accept(",") {
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		elems = append(elems, e)
	}
	return &Apply{Fn: &Var{Name: "array"}, Args: elems}, p.expect("}")
}

// count is n from a rule's upper bound n − 1: a literal plus one, or the n of
// `n-1`, as print_J wrote it (print.go, upper).
func (p *parser) count(hi Expr) Expr {
	if v, ok := p.literal(hi); ok {
		return p.fold(new(big.Int).Add(v, big.NewInt(1)))
	}
	if a, ok := hi.(*Apply); ok && len(a.Args) == 2 {
		if v, ok := a.Fn.(*Var); ok && v.Name == "-" {
			if one, ok := p.literal(a.Args[1]); ok && one.Int64() == 1 {
				return a.Args[0]
			}
		}
	}
	return &Apply{Fn: &Var{Name: "+"}, Args: []Expr{hi, &Lit{T: Int(1)}}}
}

func (p *parser) ifExpr() (Expr, error) {
	c, err := p.expr()
	if err != nil {
		return nil, err
	}
	a, err := p.valueBlock()
	if err != nil {
		return nil, err
	}
	if !p.is("else") {
		return &If{C: c, A: a, B: &Apply{Fn: &Var{Name: "tuple"}}}, nil
	}
	p.next()
	if p.accept("if") {
		b, err := p.ifExpr()
		return &If{C: c, A: a, B: b}, err
	}
	b, err := p.valueBlock()
	return &If{C: c, A: a, B: b}, err
}

func (p *parser) valueBlock() (Expr, error) {
	b, err := p.block()
	if err != nil {
		return nil, err
	}
	e, ok := b.(Expr)
	if !ok {
		return nil, p.errorf("`again` under an if (ADR 0015)")
	}
	return e, nil
}

// caseAny reads `case { g -> b … }`, the guarded choice (ADR 0056 item 2), whose
// arms are expressions (a guard is the loop's to hold, ADR 0015), or
// `case e { p -> b … }`, whose arms may be tails (ADR 0076): a Case when every
// arm is an expression, a TCase when one is not.
func (p *parser) caseAny() (Tail, error) {
	if p.is("{") {
		p.next()
		p.open(false)
		var arms []CondArm
		for {
			p.endOfStatement()
			if p.accept("}") {
				break
			}
			var guard Expr
			if !p.accept("else") {
				g, err := p.expr()
				if err != nil {
					return nil, err
				}
				guard = g
			}
			if err := p.expect("->"); err != nil {
				return nil, err
			}
			b, err := p.armValue()
			if err != nil {
				return nil, err
			}
			arms = append(arms, CondArm{Guard: guard, Body: b})
		}
		p.close()
		return &Cond{Arms: arms}, nil
	}
	scrut, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	p.open(false)
	tc := &TCase{Scrut: scrut}
	for {
		p.endOfStatement()
		if p.accept("}") {
			break
		}
		var pat Expr
		if p.accept("else") {
			pat = &Var{Name: "else"}
		} else {
			x, err := p.expr()
			if err != nil {
				return nil, err
			}
			pat = x
		}
		if err := p.expect("->"); err != nil {
			return nil, err
		}
		b, err := p.tail()
		if err != nil {
			return nil, err
		}
		tc.Arms = append(tc.Arms, TCaseArm{Pat: pat, Body: b})
	}
	p.close()
	c := &Case{Scrut: tc.Scrut}
	for _, a := range tc.Arms {
		e, ok := a.Body.(Expr)
		if !ok {
			return tc, nil
		}
		c.Arms = append(c.Arms, CaseArm{Pat: a.Pat, Body: e})
	}
	return c, nil
}

func (p *parser) armValue() (Expr, error) {
	t, err := p.tail()
	if err != nil {
		return nil, err
	}
	e, ok := t.(Expr)
	if !ok {
		return nil, p.errorf("a case arm is no tail")
	}
	return e, nil
}

// loop reads `loop x = e, … { guard -> tail … }` (ADR 0055, 0015).
func (p *parser) loop() (Expr, error) {
	l := &Loop{}
	line := p.peek().line
	for {
		l.Vars = append(l.Vars, p.name())
		if err := p.expect("="); err != nil {
			return nil, err
		}
		e, err := p.expr()
		if err != nil {
			return nil, err
		}
		l.Inits = append(l.Inits, e)
		if !p.accept(",") {
			break
		}
	}
	if err := p.expect("{"); err != nil {
		return nil, err
	}
	p.open(false)
	for {
		p.endOfStatement()
		if p.accept("}") {
			break
		}
		arm := LoopArm{}
		if p.accept("else") {
			arm.Else = true
		} else {
			g, err := p.expr()
			if err != nil {
				return nil, err
			}
			arm.Guard = g
		}
		if err := p.expect("->"); err != nil {
			return nil, err
		}
		b, err := p.tail()
		if err != nil {
			return nil, err
		}
		arm.Body = b
		l.Arms = append(l.Arms, arm)
	}
	p.close()
	if err := distinct(l.Vars, "loop", line); err != nil {
		return nil, err
	}
	// ADR 0015: the last arm is `else`, and only the last, so that every path
	// out of the loop is an arm and the arm list is its whole control flow.
	for i, arm := range l.Arms {
		if arm.Else != (i == len(l.Arms)-1) {
			return nil, fmt.Errorf("line %d: the last arm of loop must be `else`, and only the last, "+
				"so that every path out of the loop is one of its arms (ADR 0015)", line)
		}
	}
	for _, arm := range l.Arms {
		if arm.Guard != nil && strays(arm.Guard) {
			return nil, strayAgain(line)
		}
		if err := tailOK(arm.Body, len(l.Vars), line); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// distinct is W2 (abstract.md §3): a binder group names distinct things, or
// the second binder would silently shadow the first. `_` binds nothing.
func distinct(names []string, what string, line int) error {
	seen := map[string]bool{}
	for _, n := range names {
		if n == "_" || n == "-" {
			continue
		}
		if seen[n] {
			return fmt.Errorf("line %d: %s binds %s twice; a binder group may not repeat a name, "+
				"because the second would silently shadow the first", line, what, n)
		}
		seen[n] = true
	}
	return nil
}

// tailOK is W3, the tail rule at a loop (ADR 0015, abstract.md §2.5): a clause
// body is a back edge with one argument per loop variable, a binding whose body
// is one — a let, a sequence, a success arm — or an expression with no `again`
// in it. A binding may wrap an `again`; a branch or an expression may not.
func tailOK(t Tail, arity, line int) error {
	switch t := t.(type) {
	case *Again:
		if len(t.Args) != arity {
			return fmt.Errorf("line %d: again takes %d argument(s), one per loop variable, "+
				"given %d", line, arity, len(t.Args))
		}
		for _, a := range t.Args {
			if strays(a) {
				return strayAgain(line)
			}
		}
		return nil
	case *TLet:
		for _, b := range t.Binds {
			if strays(b.Value) {
				return strayAgain(line)
			}
		}
		return tailOK(t.Body, arity, line)
	case *TSeq:
		for _, x := range t.Xs {
			if strays(x) {
				return strayAgain(line)
			}
		}
		return tailOK(t.Last, arity, line)
	case *TMonad:
		for _, x := range t.Items {
			if strays(x) {
				return strayAgain(line)
			}
		}
		return tailOK(t.Body, arity, line)
	case *TCase:
		// The tail sort closed under the eliminator (ADR 0076): the scrutinee
		// and the patterns are expressions, each arm a tail.
		if strays(t.Scrut) {
			return strayAgain(line)
		}
		for _, a := range t.Arms {
			if err := tailOK(a.Body, arity, line); err != nil {
				return err
			}
		}
		return nil
	case Expr:
		if strays(t) {
			return strayAgain(line)
		}
	}
	return nil
}

// strays reports an `again` inside an expression: in J an `again` that is no
// clause body reads as the name applied, which inside a loop is a jump where
// none may stand.
func strays(e Expr) bool {
	found := false
	visit(e, func(x Expr) {
		if a, ok := x.(*Apply); ok {
			if v, ok := a.Fn.(*Var); ok && v.Name == "again" {
				found = true
			}
		}
	})
	return found
}

func strayAgain(line int) error {
	return fmt.Errorf("line %d: `again` may be a clause body, or sit under a `let`, but "+
		"not under an `if` or inside an expression — write another clause instead, so "+
		"the clause list stays the loop's whole control flow", line)
}

// try reads `try p = e { body }`, the handled bind with no handler: try's law,
// a failure passes through re-built (ADR 0057).
func (p *parser) try() (Tail, error) {
	p.next()
	pat, err := p.exprAt(pCmp + 1)
	if err != nil {
		return nil, err
	}
	if err := p.expect("="); err != nil {
		return nil, err
	}
	v, err := p.expr()
	if err != nil {
		return nil, err
	}
	body, err := p.block()
	if err != nil {
		return nil, err
	}
	if b, ok := body.(Expr); ok {
		return &Monad{Form: "try", Items: []Expr{pat, v}, Body: b}, nil
	}
	return &TMonad{Form: "try", Items: []Expr{pat, v}, Body: body}, nil
}

// isVariantAt reports whether a set declaration's right side, after its
// optional parameters and `=`, begins with a tag: a variant, not a host's
// carrier or a set expression.
func isVariantAt(toks []token, k int) bool {
	if k < len(toks) && toks[k].text == "(" {
		for k < len(toks) && toks[k].text != ")" {
			k++
		}
		k++
	}
	return k+1 < len(toks) && toks[k].text == "=" && toks[k+1].kind == tTag
}

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

// mapGraph reads `map { k : v, … }`, a map given by its graph (spec/j.md §2,
// ADR 0061): Σ₀'s `map` applied to the pairs, each the key applied to its
// value, today's `(map (k v) …)`.
func (p *parser) mapGraph() (Expr, error) {
	p.next() // {
	p.open(true)
	defer p.close()
	var pairs []Expr
	for !p.is("}") {
		k, err := p.exprAt(pCmp + 1)
		if err != nil {
			return nil, err
		}
		if err := p.expect(":"); err != nil {
			return nil, err
		}
		v, err := p.expr()
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, &Apply{Fn: k, Args: []Expr{v}})
		if !p.accept(",") {
			break
		}
	}
	return &Apply{Fn: &Var{Name: "map"}, Args: pairs}, p.expect("}")
}

// SepName is separation's former in the abstract syntax, `(#sep x S φ)`, the
// set { x ∈ S : φ } (ADR 0061 item 1; Zermelo 1908): a name a program cannot
// write, so the former is only J's.
const SepName = "#sep"

// separation reads `set { x ∈ S : φ }`, the elements of S for which φ holds.
// Its characteristic function is χ_S ∧ φ, so membership in it is membership in
// S and the formula φ, which is what a header states as its `where` and its
// `ensure` (decl.go, sig). The binder-last form `set { e : x ∈ S }` is
// replacement (Fraenkel 1922), decided by ADR 0061 and not built.
func (p *parser) separation() (Expr, error) {
	p.next() // {
	p.open(true)
	defer p.close()
	if p.peek().kind != tIdent || p.peekN(1).text != "∈" {
		return nil, p.errorf("set { e : x ∈ S } is replacement, the image of S under x ↦ e, which is " +
			"not built (ADR 0061); separation is set { x ∈ S : φ }")
	}
	x := Unspell(p.next().text)
	p.next() // ∈
	s, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect(":"); err != nil {
		return nil, err
	}
	phi, err := p.expr()
	if err != nil {
		return nil, err
	}
	if err := p.expect("}"); err != nil {
		return nil, err
	}
	return &Apply{Fn: &Var{Name: SepName}, Args: []Expr{&Var{Name: x}, s, phi}}, nil
}

// nestedSep refuses a separation anywhere but a binder's whole set. Inside a
// former, Array({x ∈ S : φ}), it would claim φ of every element, a ∀ over the
// table, which the claims do not state yet (settypes-research's array property
// fragment).
func (p *parser) nestedSep(e Expr, top bool) error {
	a, ok := e.(*Apply)
	if !ok {
		return nil
	}
	if v, ok := a.Fn.(*Var); ok && v.Name == SepName && len(a.Args) == 3 {
		if !top {
			return p.errorf("a separation, set { x ∈ S : φ }, is a binder's whole set; inside a former it " +
				"would claim φ of every element, a ∀ the claims do not state yet")
		}
		return p.nestedSep(a.Args[1], false)
	}
	for _, x := range a.Args {
		if err := p.nestedSep(x, false); err != nil {
			return err
		}
	}
	return nil
}

// writesElement reports a write `x[i] := v` in t: x is then a buffer, since a
// cell is written whole.
func writesElement(t any, x string) bool {
	found := false
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if found || !v.IsValid() {
			return
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if v.IsNil() {
				return
			}
			if w, ok := v.Interface().(*Write); ok {
				for _, p := range w.Places {
					if p.Name == x && len(p.At) > 0 {
						found = true
						return
					}
				}
			}
			walk(v.Elem())
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(t))
	return found
}

// OwnedName marks the body of a definition whose result is linear, (#owned e):
// e must be a region the definition owns, which δ decides (reduce.go, owned).
const OwnedName = "#owned"

// outTables is `out S` at an argument whose set is a set of tables, `out
// set { t ∈ Array(V) : len(t) <= n }` (ADR 0084 item 8, U3): a fresh region of
// S's greatest length, lent to the call, whose value is then the region's
// contents and the call's own, as ADR 0048's out cell is:
//
//	f(…, out S, …)   ⇝   { var #o := array { 0 : _ ∈ 0..sup-1 }; r = f(…, #o, …); (#o, r) }
//
// The supremum is read off S's formula, a conjunct len(t) <= e, len(t) < e or
// len(t) == e with e free of t; a set with none has no greatest element, and
// nothing to allocate. A cell's `out S` (a set that is no set of tables) is
// left to the cell translation (outcell.go).
func (p *parser) outTables(f Expr, args []Expr) (Expr, bool, error) {
	at := -1
	var sup Expr
	for i, a := range args {
		ap, ok := a.(*Apply)
		if !ok || len(ap.Args) != 1 {
			continue
		}
		if v, ok := ap.Fn.(*Var); !ok || v.Name != "out" {
			continue
		}
		sep, ok := ap.Args[0].(*Apply)
		if !ok || len(sep.Args) != 3 {
			continue
		}
		if v, ok := sep.Fn.(*Var); !ok || v.Name != SepName {
			continue
		}
		base, ok := sep.Args[1].(*Apply)
		if v, isVar := base.Fn.(*Var); !ok || !isVar || v.Name != "array" {
			continue
		}
		x := sep.Args[0].(*Var).Name
		s, ok := supLen(sep.Args[2], x)
		if !ok {
			return nil, false, p.errorf("`out S` allocates S's longest element, and S's formula bounds no " +
				"length: write len(t) <= n, len(t) < n or len(t) == n among its conjuncts")
		}
		if at >= 0 {
			return nil, false, p.errorf("one `out` table to a call; bind the others first")
		}
		at, sup = i, s
	}
	if at < 0 {
		return nil, false, nil
	}
	o := fmt.Sprintf("#out%d", p.nextAt())
	r := fmt.Sprintf("#res%d", p.nextAt())
	call := append([]Expr(nil), args...)
	call[at] = &Var{Name: o}
	zero := &Apply{Fn: &Var{Name: "table"}, Args: []Expr{sup, &Lit{T: Int(0)}}}
	app := &Apply{Fn: f, Args: call}
	body := &Let{Binds: []LetBind{{Pat: &PVar{Name: r}, Value: app}},
		Body: &Apply{Fn: &Var{Name: "tuple"}, Args: []Expr{&Var{Name: o}, &Var{Name: r}}}}
	loc := &Local{Binds: []LocalBind{{Name: o, Init: zero, Place: true}}, Body: body}
	if p.outScopes == nil {
		p.outScopes = map[*Local]outScope{}
	}
	p.outScopes[loc] = outScope{region: o, call: app}
	return loc, true, nil
}

// supLen is the greatest length φ allows t: a conjunct len(t) <= e, len(t) < e
// (e − 1) or len(t) == e, e free of t.
func supLen(phi Expr, x string) (Expr, bool) {
	var cs []Expr
	if a, ok := phi.(*And); ok {
		cs = a.Xs
	} else {
		cs = []Expr{phi}
	}
	for _, c := range cs {
		ap, ok := c.(*Apply)
		if !ok || len(ap.Args) != 2 {
			continue
		}
		op, ok := ap.Fn.(*Var)
		if !ok {
			continue
		}
		l, ok := ap.Args[0].(*Apply)
		if !ok || len(l.Args) != 1 {
			continue
		}
		if v, ok := l.Fn.(*Var); !ok || v.Name != "len" {
			continue
		}
		if v, ok := l.Args[0].(*Var); !ok || v.Name != x || mentions(ap.Args[1], x) {
			continue
		}
		switch op.Name {
		case "<=", "=":
			return ap.Args[1], true
		case "<":
			return &Apply{Fn: &Var{Name: "-"}, Args: []Expr{ap.Args[1], &Lit{T: Int(1)}}}, true
		}
	}
	return nil, false
}

// mentions reports a variable x in e.
func mentions(e Expr, x string) bool {
	found := false
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		if found || !v.IsValid() {
			return
		}
		switch v.Kind() {
		case reflect.Interface, reflect.Pointer:
			if v.IsNil() {
				return
			}
			if w, ok := v.Interface().(*Var); ok && w.Name == x {
				found = true
				return
			}
			walk(v.Elem())
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(e))
	return found
}
