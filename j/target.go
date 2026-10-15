package j

import (
	"fmt"
	"os"
	"strings"

	"oroboros/core"
)

// TARGET FILES AND FRAGMENTS IN J (spec/j.md §2.1–2.5, §2.11–2.12; ADR 0064,
// 0066). A target file is a set of fragments of one model, the target its
// directory, so `(target T …)` goes; `(provides T M …)` is a fragment where a
// library lives. A host declaration is a definition with a host body whose
// template's holes are named; the model's own forms keep their words. Σ_T has
// no abstract tree (abstract.md §2.6), so this printer reads the raw forms.

// TranslateTargetFile prints a target file, or a library's fragment file, in J;
// root is the repository, whose lib/ holds the modules a fragment imports.
func TranslateTargetFile(path, root string) (*Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return TranslateTarget(string(b), root)
}

// TranslateTarget prints the raw forms of a target source in J.
func TranslateTarget(src, root string) (*Result, error) {
	rc, err := core.ReadRawConcrete(src)
	if err != nil {
		return nil, err
	}
	p := &printer{src: src, spans: map[any]core.Span{}, gaps: rc.Gaps, ctors: map[string]bool{},
		sets: map[string]bool{}, data: map[string]bool{}, locals: map[string]bool{}, fns: map[string]bool{}}
	for t, sp := range rc.Spans {
		p.spans[t] = sp
	}
	p.rawSpans, p.root = rc.Spans, root
	items, err := p.targetForms(rc.Terms, 0)
	if err != nil {
		return nil, err
	}
	var rest []*Doc
	for p.next < len(p.gaps) {
		rest = append(rest, Hard(), comment(p.gaps[p.next]))
		p.next++
	}
	return &Result{J: Render(Cat(items, Cat(rest...)), Width) + "\n"}, nil
}

// targetForms prints forms in order, one a line, a blank line where the source
// has one; depth counts the modules around them.
func (p *printer) targetForms(ts []*core.Term, depth int) (*Doc, error) {
	// An inclusion is printed in the module of its type (ADR 0064 item 9): a
	// target file is the glue of its forms, so a form's place carries nothing,
	// and inside its module the type is named without its path.
	moved := map[*core.Term]bool{}
	for _, m := range ts {
		if head(m) != "module" {
			continue
		}
		path := atom(m.Kids[1])
		for _, t := range ts {
			if head(t) == "implements" && len(t.Kids) > 1 && t.Kids[1].Kind == core.KName &&
				strings.HasPrefix(t.Kids[1].Name, path+".") && !strings.Contains(t.Kids[1].Name[len(path)+1:], ".") {
				moved[t] = true
				rel := &core.Term{Kind: core.KApp, Kids: append([]*core.Term{t.Kids[0], core.Name(t.Kids[1].Name[len(path)+1:])}, t.Kids[2:]...)}
				p.spans[rel] = p.rawSpans[t]
				p.rawSpans[rel] = p.rawSpans[t]
				m.Kids = append(m.Kids, rel)
			}
		}
	}
	var out []*Doc
	prevEnd := -1
	for _, t := range ts {
		if moved[t] {
			continue
		}
		sp := p.rawSpans[t]
		blank := prevEnd >= 0 && blankLine(p.src[prevEnd:max(prevEnd, sp.Start)])
		prevEnd = max(prevEnd, sp.End)
		if head(t) == "target" {
			// The target is the directory (ADR 0064 item 2): its forms stand alone.
			inner, err := p.targetForms(t.Kids[2:], depth)
			if err != nil {
				return nil, err
			}
			if len(out) > 0 {
				out = append(out, Hard())
			}
			out = append(out, Cat(p.leading(sp.Start), inner))
			continue
		}
		var ferr error
		d := p.item(t, func() *Doc {
			doc, err := p.targetForm(t, depth)
			ferr = err
			return doc
		})
		if ferr != nil {
			return nil, ferr
		}
		if len(out) > 0 {
			out = append(out, Hard())
			if blank {
				out = append(out, Hard())
			}
		}
		out = append(out, d)
	}
	return Cat(out...), nil
}

func head(t *core.Term) string {
	if t.Kind == core.KApp && len(t.Kids) > 0 && t.Kids[0].Kind == core.KName {
		return t.Kids[0].Name
	}
	return ""
}

func atom(t *core.Term) string {
	switch t.Kind {
	case core.KName:
		return t.Name
	case core.KInt:
		return fmt.Sprint(t.Int)
	}
	return t.String()
}

// targetForm prints one form of a target file.
func (p *printer) targetForm(t *core.Term, depth int) (*Doc, error) {
	k := t.Kids
	switch head(t) {
	case "module":
		inner, err := p.targetForms(k[2:], depth+1)
		if err != nil {
			return nil, err
		}
		return Cat(Text("module "+atom(k[1])+" {"), Nest(4, Cat(Hard(), inner)), p.rawClosing(t), Hard(), Text("}")), nil
	case "provides":
		inner, err := p.targetForms(k[3:], depth+1)
		if err != nil {
			return nil, err
		}
		return Cat(Text("provides "+atom(k[1])+" "+atom(k[2])+" {"), Nest(4, Cat(Hard(), inner)), p.rawClosing(t), Hard(), Text("}")), nil
	case "use":
		u := &core.UseDecl{Path: atom(k[1])}
		if len(k) == 4 {
			u.As = atom(k[3])
		}
		p.imports(u, p.root, p.root)
		if len(k) == 4 && atom(k[2]) == "as" && atom(k[3]) != atom(k[1])[strings.LastIndex(atom(k[1]), "/")+1:] {
			return Text("use " + atom(k[1]) + " as " + atom(k[3])), nil
		}
		return Text("use " + atom(k[1])), nil
	case "sig":
		return p.hostSig(t)
	case "type":
		return p.hostType(t)
	case "const":
		// A host constant's value is its claim (ADR 0064 item 6): a singleton,
		// and the host's spelling its body.
		return Cat(Text(Spell(atom(k[1]))+" ∈ set { "), p.rawExpr(k[2], pLowest), Text(" } = "), p.hostBody(k[3], nil, nil, "")), nil
	case "implements":
		ds := make([]*Doc, len(k)-2)
		for i, x := range k[2:] {
			ds[i] = p.rawSet(x)
		}
		return Cat(Text("axiom "), p.rawSet(k[1]), Text(" ⊆ "), Join(Text(" ∩ "), ds)), nil
	case "def":
		return p.fragmentDef(t)
	case "repr", "include", "structural", "backend", "link", "build", "artifact", "data", "underlying",
		"max-len", "big-repr", "int-repr", "fact":
		return p.modelData(t), nil
	}
	return nil, fmt.Errorf("a target form %q has no J printing", head(t))
}

// rawClosing prints the gaps still inside a module before its closing brace.
func (p *printer) rawClosing(t *core.Term) *Doc { return p.closing(t) }

// modelData prints a model's own form with its word, its operands as J
// (spec/j.md §2.12; ADR 0064 item 10). A template here keeps the model form's
// positional holes: the model's forms bind no names to splice.
func (p *printer) modelData(t *core.Term) *Doc {
	k := t.Kids
	ds := []*Doc{Text(word(k[0].Name, true))}
	for i, x := range k[1:] {
		switch {
		case head(x) == "host":
			ds = append(ds, p.rawHost(x))
		case head(x) == "niche":
			ds = append(ds, Cat(Text("niche "), p.rawHost(x.Kids[1])))
		case x.Kind == core.KApp && head(t) == "fact" && i == 1:
			ds = append(ds, p.rawParams(x))
		case x.Kind == core.KApp && head(t) == "fact":
			ds = append(ds, p.rawExpr(x, pLowest))
		case x.Kind == core.KApp:
			ds = append(ds, p.rawSet(x))
		default:
			ds = append(ds, Text(p.rawAtom(x)))
		}
	}
	return Join(Text(" "), ds)
}

func (p *printer) rawAtom(x *core.Term) string {
	switch x.Kind {
	case core.KName:
		return Spell(x.Name)
	case core.KStr:
		return p.source(x)
	case core.KInt:
		return Integer(fmt.Sprint(x.Int))
	}
	return p.source(x)
}

// source is a datum's text as written.
func (p *printer) source(x *core.Term) string {
	if sp, ok := p.rawSpans[x]; ok {
		return p.src[sp.Start:sp.End]
	}
	return x.String()
}

// rawHost prints a model form's `(host …)` operand as written, kind and all.
func (p *printer) rawHost(x *core.Term) *Doc {
	ds := []*Doc{Text("host")}
	for _, y := range x.Kids[1:] {
		ds = append(ds, Text(p.rawAtom(y)))
	}
	return Join(Text(" "), ds)
}

// ---- host declarations ----

// hostSig prints `(sig NAME (params) RESULT clauses… (host …))` as a definition
// with a host body; a head with static parameters, `(sig (F (E (subsets B))) …)`,
// as a λ returning a λ (ADR 0066 item 1).
func (p *printer) hostSig(t *core.Term) (*Doc, error) {
	k := t.Kids
	if len(k) < 4 {
		return nil, fmt.Errorf("sig %s: too short", t)
	}
	name, statics := k[1], []string(nil)
	var staticDocs []*Doc
	if name.Kind == core.KApp {
		for _, s := range name.Kids[1:] {
			if s.Kind == core.KName {
				statics = append(statics, s.Name)
				staticDocs = append(staticDocs, Text(s.Name))
				continue
			}
			statics = append(statics, s.Kids[0].Name)
			bound := s.Kids[1]
			if head(bound) == "subsets" {
				staticDocs = append(staticDocs, Cat(Text(s.Kids[0].Name+" ⊆ "), p.rawSet(bound.Kids[1])))
			} else {
				staticDocs = append(staticDocs, Cat(Text(s.Kids[0].Name+" ∈ "), p.rawSet(bound)))
			}
		}
		name = name.Kids[0]
	}
	var host *core.Term
	var clauses []*core.Term
	for _, c := range k[4:] {
		if head(c) == "host" {
			host = c
		} else {
			clauses = append(clauses, c)
		}
	}
	if host == nil {
		return nil, fmt.Errorf("sig %s has no host body", atom(name))
	}
	params, names := p.rawParamList(k[2])
	tpl := ""
	if len(host.Kids) > 2 && host.Kids[2].Kind == core.KStr {
		tpl = host.Kids[2].Str
	}
	result := ""
	if strings.Contains(tpl, "%r") || strings.Contains(tpl, "%br") || strings.Contains(tpl, "%er") || mentionsResult(clauses) {
		result = "r"
		for _, n := range names {
			if n == "r" {
				result = "r_"
			}
		}
	}
	hdr := Cat(Text("("), Nest(4, Fill(",", params)), Text(")"))
	if res := k[3]; !(res.Kind == core.KName && res.Name == "any" && tpl != "" && host.Kids[1].Name == "stmt" && false) {
		if result != "" {
			hdr = Cat(hdr, Text(" ("+result+" ∈ "), p.rawResult(res), Text(")"))
		} else {
			hdr = Cat(hdr, Text(" ("), p.rawResult(res), Text(")"))
		}
	}
	for _, c := range clauses {
		switch {
		case c.Kind == core.KName:
			hdr = Cat(hdr, Text(" "+c.Name))
		case head(c) == "where":
			hdr = Cat(hdr, Nest(4, Cat(Line(), Text("where "), p.rawExpr(c.Kids[1], pLowest))))
		case head(c) == "ensures":
			p.resultName = result
			hdr = Cat(hdr, Nest(4, Cat(Line(), Text("ensure "), p.rawExpr(c.Kids[1], pLowest))))
			p.resultName = ""
		default:
			return nil, fmt.Errorf("sig %s: clause %s", atom(name), c)
		}
	}
	body := p.hostBody(host, names, statics, result)
	braced := Cat(Text(" {"), Nest(4, Cat(Line(), body)), Line(), Text("}"))
	fn := Group(Cat(Text("fn "+Spell(atom(name))), hdr, braced))
	if staticDocs == nil {
		return fn, nil
	}
	inner := Group(Cat(Text("fn"), hdr, braced))
	return Cat(Text("fn "+Spell(atom(name))+"("), Join(Text(", "), staticDocs), Text(") {"),
		Nest(4, Cat(Hard(), inner)), Hard(), Text("}")), nil
}

func mentionsResult(cs []*core.Term) bool {
	found := false
	for _, c := range cs {
		walkRaw(c, func(t *core.Term) {
			if t.Kind == core.KName && t.Name == core.ResultName {
				found = true
			}
		})
	}
	return found
}

func walkRaw(t *core.Term, f func(*core.Term)) {
	f(t)
	for _, k := range t.Kids {
		walkRaw(k, f)
	}
}

// rawParamList names a sig's parameters, the written name or a, b, c… for a
// parameter given only its set, and prints each with its set and mode.
func (p *printer) rawParamList(t *core.Term) ([]*Doc, []string) {
	taken := map[string]bool{}
	for _, x := range t.Kids {
		if x.Kind == core.KApp && len(x.Kids) == 2 && x.Kids[0].Kind == core.KName && !isTypeHead(x) {
			taken[x.Kids[0].Name] = true
		}
	}
	next := 0
	freshName := func() string {
		for {
			n := string(rune('a' + next%26))
			if next >= 26 {
				n += fmt.Sprint(next / 26)
			}
			next++
			if !taken[n] {
				taken[n] = true
				return n
			}
		}
	}
	var docs []*Doc
	var names []string
	for _, x := range t.Kids {
		var n string
		var set *core.Term
		if x.Kind == core.KApp && len(x.Kids) == 2 && x.Kids[0].Kind == core.KName && !isTypeHead(x) {
			n, set = x.Kids[0].Name, x.Kids[1]
		} else {
			n, set = freshName(), x
		}
		names = append(names, n)
		d := Cat(Text(Spell(n)+" ∈ "), p.rawSet(stripMode(set)))
		if m := rawMode(set); m != "" {
			d = Cat(d, Text(" "+m))
		}
		docs = append(docs, d)
	}
	return docs, names
}

// rawParams prints a fact's parameter list.
func (p *printer) rawParams(t *core.Term) *Doc {
	docs, _ := p.rawParamList(t)
	return Cat(Text("("), Join(Text(", "), docs), Text(")"))
}

// isTypeHead reports whether a two-element list is a set former applied, not a
// parameter's name and set: `(array int)` against `(src int)`.
func isTypeHead(x *core.Term) bool {
	switch x.Kids[0].Name {
	case "array", "map", "buffer", "cell", "out", "option", "tuple", "subsets", "int":
		return true
	}
	return strings.Contains(x.Kids[0].Name, ".")
}

func stripMode(t *core.Term) *core.Term {
	switch head(t) {
	case "buffer":
		return &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name("array"), t.Kids[1]}}
	case "cell":
		return t.Kids[1]
	}
	return t
}

func rawMode(t *core.Term) string {
	switch head(t) {
	case "buffer":
		return "linear"
	case "cell":
		return "inout"
	}
	return ""
}

// rawResult prints a sig's result: a set, or a product written as a list of
// sets, `(int int)`.
func (p *printer) rawResult(t *core.Term) *Doc {
	if t.Kind == core.KApp && len(t.Kids) > 1 && t.Kids[0].Kind == core.KName && langSets[t.Kids[0].Name] != "" &&
		!(t.Kids[0].Name == "int" && len(t.Kids) == 3 && isBound(t.Kids[1]) && isBound(t.Kids[2])) &&
		t.Kids[0].Name != "array" && t.Kids[0].Name != "map" && t.Kids[0].Name != "option" {
		ds := make([]*Doc, len(t.Kids))
		for i, x := range t.Kids {
			ds[i] = p.rawSet(x)
		}
		return Join(Text(" × "), ds)
	}
	if t.Kind == core.KApp && len(t.Kids) == 0 {
		return Text("()")
	}
	return p.rawSet(t)
}

// isBound reports whether t is a range's endpoint: a literal, a constant, a
// power, or a literal too big for a word, which the reader writes in Horner
// form, sums and products of literals.
func isBound(t *core.Term) bool {
	switch head(t) {
	case "pow":
		return true
	case "+", "-", "*":
		for _, k := range t.Kids[1:] {
			if !isBound(k) {
				return false
			}
		}
		return len(t.Kids) > 1
	}
	return t.Kind == core.KInt || (t.Kind == core.KName && !isLangSet(t.Name))
}

func isLangSet(n string) bool { _, ok := langSets[n]; return ok }

// hostBody prints `(host KIND "template" clauses…)`: the template's holes named
// (spec/j.md §2.11), each host clause in J's spelling. A carrier or a constant's
// spelling has no kind.
func (p *printer) hostBody(h *core.Term, params, statics []string, result string) *Doc {
	k := h.Kids
	ds := []*Doc{Text("host")}
	i := 1
	if i < len(k) && k[i].Kind == core.KName {
		ds = append(ds, Text(k[i].Name))
		i++
	}
	if i < len(k) && k[i].Kind == core.KStr {
		ds = append(ds, Text(quote(named(k[i].Str, params, statics, result))))
		i++
	}
	for _, c := range k[i:] {
		switch head(c) {
		case "import", "lib", "checked", "jump":
			cd := []*Doc{Text(head(c))}
			for i, y := range c.Kids[1:] {
				if head(c) == "jump" && i == 1 && y.Kind == core.KStr {
					// A jump's comparison is a template over the same parameters.
					cd = append(cd, Text(quote(named(y.Str, params, statics, result))))
					continue
				}
				cd = append(cd, Text(p.rawAtom(y)))
			}
			ds = append(ds, Join(Text(" "), cd))
		case "comma-ok":
			ds = append(ds, Text("comma_ok"))
		default:
			ds = append(ds, Text(p.source(c)))
		}
	}
	return Join(Text(" "), ds)
}

// named rewrites a template's positional holes as named splices (ADR 0066 item
// 9): the i-th `%s` the i-th parameter, cyclically as the filler repeats them
// (emit/target.go, fill); `%tᵢ` the i-th static parameter; x86's `%n` the n-th
// parameter, `%r` the result, and `%bX`, `%eX` their 8- and 32-bit views; `%%` a
// percent sign; `%u`, a number unique to each use, `${#}`; and a `${` already
// there escaped as `$${`.
func named(tpl string, params, statics []string, result string) string {
	var b strings.Builder
	seq := 0
	splice := func(n, view string) {
		b.WriteString("${" + Spell(n))
		if view != "" {
			b.WriteString(":" + view)
		}
		b.WriteString("}")
	}
	for i := 0; i < len(tpl); i++ {
		c := tpl[i]
		if c == '$' && i+1 < len(tpl) && tpl[i+1] == '{' {
			b.WriteString("$$")
			continue
		}
		if c != '%' || i+1 >= len(tpl) {
			b.WriteByte(c)
			continue
		}
		rest := tpl[i+1:]
		switch {
		case rest[0] == '%':
			b.WriteByte('%')
			i++
		case rest[0] == 'u':
			// A number unique to each use of the template, x86's labels:
			// `${#}`, which no binder can be called (spec/j.md §2.11).
			b.WriteString("${#}")
			i++
		case rest[0] == 's' && len(params) > 0:
			splice(params[seq%len(params)], "")
			seq++
			i++
		case rest[0] == 't' && len(rest) > 1 && rest[1] >= '1' && rest[1] <= '9' && int(rest[1]-'1') < len(statics):
			splice(statics[rest[1]-'1'], "")
			i += 2
		case rest[0] >= '1' && rest[0] <= '9' && int(rest[0]-'1') < len(params):
			splice(params[rest[0]-'1'], "")
			i++
		case rest[0] == 'r' && result != "":
			splice(result, "")
			i++
		case (rest[0] == 'b' || rest[0] == 'e') && len(rest) > 1 && rest[1] == 'r' && result != "":
			splice(result, string(rest[0]))
			i += 2
		case (rest[0] == 'b' || rest[0] == 'e') && len(rest) > 1 && rest[1] >= '1' && rest[1] <= '9' && int(rest[1]-'1') < len(params):
			splice(params[rest[1]-'1'], string(rest[0]))
			i += 2
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// quote writes a string literal with string-literals.md's escapes.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// hostType prints `(type NAME (host "s"))` as a set with a host carrier,
// `(type (F A…) (host "…%s…"))` as a set-valued function whose template names
// its parameters, and `(type NAME S)` as a set definition (ADR 0064 item 6).
func (p *printer) hostType(t *core.Term) (*Doc, error) {
	k := t.Kids
	name, params := k[1], []string(nil)
	if name.Kind == core.KApp {
		for _, x := range name.Kids[1:] {
			params = append(params, x.Name)
		}
		name = name.Kids[0]
	}
	lhs := "set " + p.setName(atom(name))
	if len(params) > 0 {
		lhs += "(" + strings.Join(params, ", ") + ")"
	}
	if head(k[2]) == "host" {
		return Cat(Text(lhs+" = "), p.hostBody(k[2], params, nil, "")), nil
	}
	return Cat(Text(lhs+" = "), p.rawSet(k[2])), nil
}

// fragmentDef prints a definition inside a fragment (D_T) as a program's.
func (p *printer) fragmentDef(t *core.Term) (*Doc, error) {
	sp := p.rawSpans[t]
	r, err := Translate(p.src[sp.Start:sp.End], ".", ".")
	if err != nil {
		return nil, err
	}
	// Its comments were printed with it; the outer printer skips them.
	for p.next < len(p.gaps) && p.gaps[p.next].Start < sp.End {
		p.next++
	}
	lines := strings.Split(strings.TrimRight(r.J, "\n"), "\n")
	ds := make([]*Doc, len(lines))
	for i, l := range lines {
		ds[i] = Text(l)
	}
	return Join(Hard(), ds), nil
}

// ---- sets and expressions from raw forms ----

func (p *printer) rawSet(t *core.Term) *Doc { return p.set(p.raw(t)) }

func (p *printer) rawExpr(t *core.Term, prec int) *Doc { return p.prec(p.raw(t), prec) }

// raw reads a raw datum as the computation's tree, with the connectives as
// connectives (core/mu.go reads them so) and each literal keeping its span.
func (p *printer) raw(t *core.Term) core.Expr {
	switch t.Kind {
	case core.KName:
		if t.Name == core.ResultName && p.resultName != "" {
			return &core.Var{Name: p.resultName}
		}
		return &core.Var{Name: t.Name}
	case core.KInt, core.KFloat, core.KStr, core.KBool:
		l := &core.Lit{T: t}
		if sp, ok := p.rawSpans[t]; ok {
			p.spans[core.Tail(l)] = sp
		}
		return l
	}
	// An integer past the word, which the reader writes in Horner's form over
	// 10¹⁵ (core/read.go, bigLiteral): its source is the literal.
	if sp, ok := p.rawSpans[t]; ok && strings.Trim(p.src[sp.Start:sp.End], "-0123456789") == "" {
		l := &core.Lit{T: core.Int(0)}
		p.spans[core.Tail(l)] = sp
		return l
	}
	if len(t.Kids) == 0 {
		return &core.EmptyList{}
	}
	args := make([]core.Expr, len(t.Kids)-1)
	for i, k := range t.Kids[1:] {
		args[i] = p.raw(k)
	}
	switch head(t) {
	case "and":
		return &core.And{Xs: args}
	case "or":
		return &core.Or{Xs: args}
	case "not":
		if len(args) == 1 {
			return &core.Not{X: args[0]}
		}
	case "if":
		if len(args) == 3 {
			return &core.If{C: args[0], A: args[1], B: args[2]}
		}
	}
	return &core.Apply{Fn: p.raw(t.Kids[0]), Args: args}
}
