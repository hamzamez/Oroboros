package j

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"oroboros/core"
)

// THE TRANSLATOR, tr_{T→J} = print_J ∘ (id × transport) ∘ μ (abstract.md §9.2,
// spec/j.md §7), on programs. Target files and their fragments (`provides`) are
// step 2d's; a program holding one is refused here, naming it.

// Width is the column print_J lays out within.
const Width = 100

// Result is a file printed in J, and what the printer could not decide.
type Result struct {
	J         string
	Uncertain int // one-argument applications of a local name not shown to be data or a function
	// UncertainNames are those applications' heads, in order.
	UncertainNames []string
	Collisions     [][2]string // names σ_J spells alike (J8): the file is refused when there is one
}

// TranslateFile prints one program source in J. root is the repository, whose
// lib/ holds the modules a program imports; their variants decide which names
// are constructors and which are sets.
func TranslateFile(path, root string) (*Result, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Translate(string(b), filepath.Dir(path), root)
}

// Translate prints a program source in J; dir is its directory.
func Translate(src, dir, root string) (*Result, error) {
	c, err := core.ReadConcrete(src)
	if err != nil {
		return nil, err
	}
	p := &printer{src: src, spans: c.Spans, gaps: c.Gaps,
		ctors: map[string]bool{"some": true, "none": true}, sets: map[string]bool{"option": true}}
	var names []string
	modules := 0
	for _, d := range c.Decls {
		switch d := d.(type) {
		case *core.VariantDecl:
			p.sets[d.Name] = true
			for _, it := range d.Items {
				if it.Ctor != "" {
					p.ctors[it.Ctor] = true
				}
			}
		case *core.UseDecl:
			p.imports(d, dir, root)
		case *core.ModuleDecl:
			modules++
		case *core.ForeignDecl:
			return nil, fmt.Errorf("a target fragment (provides, prim or target) is translated in step 2d")
		case *core.DefDecl:
			names = append(names, d.Name)
		}
	}
	res := &Result{Collisions: Collisions(names)}
	if len(res.Collisions) > 0 {
		return res, fmt.Errorf("σ_J spells %s and %s alike (spec/j.md J8)", res.Collisions[0][0], res.Collisions[0][1])
	}
	library := modules == 1 && isModuleFirst(c.Decls)
	// PROGRAM ORDER (ADR 0052): today's threaded buffers become places, and each
	// rewritten node takes its original's span, so its comments go with it.
	origin, err := unthread(c.Decls, borrowsOf(c.Decls, dir, root))
	if err != nil {
		return nil, err
	}
	for n, o := range origin {
		if sp, ok := p.spans[o]; ok {
			if _, own := p.spans[n]; !own {
				p.spans[n] = sp
			}
		}
	}
	p.analyze(c.Decls)
	doc := p.file(c.Decls, library)
	res.J = Render(doc, Width) + "\n"
	res.Uncertain, res.UncertainNames = p.Uncertain, p.UncertainNames
	return res, nil
}

// borrowsOf is what the callees a file can call borrow, by the names its calls
// write: a host's declaration as alias.member, an imported module's
// definition likewise (spec/local.md §2.1).
func borrowsOf(ds []core.Decl, dir, root string) map[string]core.BorrowDecl {
	host := hostBorrows(root)
	out := map[string]core.BorrowDecl{}
	for _, d := range ds {
		u, ok := d.(*core.UseDecl)
		if !ok {
			continue
		}
		alias := u.As
		if alias == "" {
			alias = u.Path[strings.LastIndex(u.Path, "/")+1:]
		}
		for k, v := range host {
			if rest, ok := strings.CutPrefix(k, u.Path+"."); ok && !strings.Contains(rest, ".") {
				out[alias+"."+rest] = v
			}
		}
		for _, file := range []string{filepath.Join(root, "lib", u.Path+".oro"), filepath.Join(dir, filepath.Base(u.Path)+".oro")} {
			if _, err := os.Stat(file); err == nil {
				for k, v := range moduleBorrows(file, nil) {
					out[alias+"."+k] = v
				}
				break
			}
		}
	}
	return out
}

// Unthreaded is a source's declaration trees in program order, U of today's
// (unthread.go): what print_J prints, and what J's reader must give back.
func Unthreaded(src, dir, root string) ([]core.Decl, error) {
	ds, err := core.ReadTrees(src)
	if err != nil {
		return nil, err
	}
	if _, err := unthread(ds, borrowsOf(ds, dir, root)); err != nil {
		return nil, err
	}
	return ds, nil
}

func isModuleFirst(ds []core.Decl) bool {
	for _, d := range ds {
		switch d.(type) {
		case *core.ModuleDecl:
			return true
		case *core.UseDecl:
		default:
			return false
		}
	}
	return false
}

// imports records an imported module's constructors and sets, `alias.c` and
// `alias.S`, reading its source: lib/PATH.oro, or PATH.oro beside the program.
func (p *printer) imports(u *core.UseDecl, dir, root string) {
	alias := u.As
	if alias == "" {
		alias = u.Path[strings.LastIndex(u.Path, "/")+1:]
	}
	for _, cand := range []string{filepath.Join(root, "lib", u.Path+".oro"), filepath.Join(dir, filepath.Base(u.Path)+".oro")} {
		b, err := os.ReadFile(cand)
		if err != nil {
			continue
		}
		ds, err := core.ReadTrees(string(b))
		if err != nil {
			return
		}
		for _, d := range ds {
			if v, ok := d.(*core.VariantDecl); ok {
				p.sets[alias+"."+v.Name] = true
				for _, it := range v.Items {
					if it.Ctor != "" {
						p.ctors[alias+"."+it.Ctor] = true
					}
				}
			}
		}
		return
	}
}

// file prints a module's declarations in order. A library file's one
// `(module X)` is its path restated, and goes (ADR 0064 item 1); an entry
// file's modules nest. In a library a definition no export names is private; in
// an entry file an exported one is an entry point (items 4–5).
func (p *printer) file(ds []core.Decl, library bool) *Doc {
	exported := map[string]bool{}
	for _, d := range ds {
		if e, ok := d.(*core.ExportDecl); ok {
			for _, n := range e.Names {
				exported[n] = true
			}
		}
	}
	// A section is the top level or one module of an entry file; each item
	// remembers whether the source left a blank line before it.
	type entry struct {
		doc   *Doc
		blank bool
	}
	type section struct {
		head    string
		entries []entry
	}
	sections := []*section{{}}
	prevEnd := -1
	for _, d := range ds {
		sp := p.spans[d]
		blank := prevEnd >= 0 && blankLine(p.src[prevEnd:max(prevEnd, sp.Start)])
		prevEnd = max(prevEnd, sp.End)
		cur := sections[len(sections)-1]
		var build func() *Doc
		switch d := d.(type) {
		case *core.ModuleDecl:
			if !library {
				sections = append(sections, &section{head: "module " + d.Path})
			}
			continue
		case *core.ExportDecl:
			continue
		case *core.UseDecl:
			build = func() *Doc {
				if d.As != "" && d.As != d.Path[strings.LastIndex(d.Path, "/")+1:] {
					return Text("use " + d.Path + " as " + d.As)
				}
				return Text("use " + d.Path)
			}
		case *core.DefDecl:
			build = func() *Doc { return Cat(p.visibility(d.Name, exported, library), p.def(d)) }
		case *core.ClaimDecl:
			build = func() *Doc { return p.claimOnly(d) }
		case *core.VariantDecl:
			build = func() *Doc { return p.variant(d, exported, library) }
		case *core.TermDecl:
			build = func() *Doc { return p.expr(d.E) }
		}
		cur.entries = append(cur.entries, entry{p.item(d, build), blank})
	}
	join := func(es []entry) *Doc {
		var out []*Doc
		for i, e := range es {
			if i > 0 {
				out = append(out, Hard())
				if e.blank {
					out = append(out, Hard())
				}
			}
			out = append(out, e.doc)
		}
		return Cat(out...)
	}
	var parts []*Doc
	for _, sec := range sections {
		if sec.head == "" {
			if len(sec.entries) > 0 {
				parts = append(parts, join(sec.entries))
			}
			continue
		}
		parts = append(parts, Cat(Text(sec.head+" {"), Nest(4, Cat(Hard(), join(sec.entries))), Hard(), Text("}")))
	}
	var rest []*Doc
	for p.next < len(p.gaps) {
		rest = append(rest, Hard(), comment(p.gaps[p.next]))
		p.next++
	}
	return Cat(Join(Cat(Hard(), Hard()), parts), Cat(rest...))
}

// blankLine reports whether a stretch of source holds an empty line.
func blankLine(s string) bool {
	lines := strings.Split(s, "\n")
	if len(lines) < 3 {
		return false
	}
	for _, l := range lines[1 : len(lines)-1] {
		if strings.TrimSpace(l) == "" {
			return true
		}
	}
	return false
}

func (p *printer) visibility(name string, exported map[string]bool, library bool) *Doc {
	switch {
	case library && !exported[name]:
		return Text("private ")
	case !library && exported[name]:
		return Text("export ")
	}
	return nil
}

// def prints a definition: a named λ with its claims, or a value.
func (p *printer) def(d *core.DefDecl) *Doc {
	p.locals, p.data = map[string]bool{}, map[string]bool{}
	lam, ok := d.Value.(*core.Lam)
	if !ok {
		return Cat(Text(Spell(d.Name)+" = "), p.expr(d.Value))
	}
	p.data, p.fns = p.evidence(d.Name, lam)
	head := Cat(Text("fn "+Spell(d.Name)), p.header(lam.Params, lam.Claim, lam.Borrows))
	return Cat(head, Text(" "), p.body(lam.Body))
}

// header prints parameters with their sets and modes, the result, `where` and
// `ensure` (spec/j.md §2.4; ADR 0062).
func (p *printer) header(params []string, c *core.Claim, borrows []string) *Doc {
	borrowed := map[string]bool{}
	for _, b := range borrows {
		borrowed[b] = true
	}
	var ps []*Doc
	for i, n := range params {
		p.locals[n] = true
		if c == nil || i >= len(c.Params) {
			if borrowed[n] {
				// A BORROWED PARAMETER, a place in the body (ADR 0052 item 3).
				p.data[n] = true
				ps = append(ps, Text(Spell(n)+" linear"))
				continue
			}
			ps = append(ps, Text(Spell(n)))
			continue
		}
		set, mode := p.paramSet(c.Params[i])
		if mode == core.Linear {
			p.data[n] = true
		}
		d := Cat(Text(Spell(n)+" ∈ "), set)
		if mode != core.NoMode {
			d = Cat(d, Text(" "+string(mode)))
		}
		ps = append(ps, d)
	}
	out := Cat(Text("("), Nest(4, Fill(",", ps)), Text(")"))
	if c == nil {
		return out
	}
	result := "r"
	for _, n := range params {
		if n == result {
			result = "r_"
		}
	}
	named := c.Post != nil && mentions(c.Post, core.ResultName)
	if c.Result != nil {
		if named {
			out = Cat(out, Text(" ("+result+" ∈ "), p.set(c.Result), Text(")"))
		} else {
			out = Cat(out, Text(" ("), p.set(c.Result), Text(")"))
		}
	}
	// A claim written apart, today's `sig`, names its parameters itself; its
	// formulas are read with the definition's names, the header's (ADR 0051).
	names := map[string]string{}
	for i, cp := range c.Params {
		if i < len(params) && cp.Name != "" && cp.Name != params[i] {
			names[cp.Name] = params[i]
		}
	}
	if c.Pre != nil {
		out = Cat(out, Nest(4, Cat(Line(), Text("where "), p.expr(renameExpr(c.Pre, names)))))
	}
	if c.Post != nil {
		names[core.ResultName] = result
		post := renameExpr(c.Post, names)
		out = Cat(out, Nest(4, Cat(Line(), Text("ensure "), p.expr(post))))
	}
	return Group(out)
}

// paramSet is a parameter's set and mode: `(buffer V)` is Array(V) in the
// linear mode and `(cell T)` is T passed inout (ADR 0050 item 2).
func (p *printer) paramSet(cp core.ClaimParam) (*Doc, core.Mode) {
	mode := cp.Mode
	if a, ok := cp.Set.(*core.Apply); ok && len(a.Args) == 1 {
		if v, ok := a.Fn.(*core.Var); ok {
			switch v.Name {
			case "buffer":
				return Cat(Text("Array("), p.set(a.Args[0]), Text(")")), core.Linear
			case "cell":
				return p.set(a.Args[0]), core.InOut
			}
		}
	}
	return p.set(cp.Set), mode
}

func (p *printer) claimOnly(d *core.ClaimDecl) *Doc {
	p.locals, p.data = map[string]bool{}, map[string]bool{}
	names := make([]string, len(d.Claim.Params))
	for i, cp := range d.Claim.Params {
		names[i] = cp.Name
		if names[i] == "" {
			names[i] = fmt.Sprintf("x%d", i+1)
		}
	}
	return Cat(Text("fn "+Spell(d.Name)), p.header(names, d.Claim, nil))
}

// variant prints a variant as a set definition, a union of tagged sets with its
// markers after it (ADR 0064 item 8).
func (p *printer) variant(v *core.VariantDecl, exported map[string]bool, library bool) *Doc {
	name := SetName(v.Name)
	if len(v.Params) > 0 {
		name += "(" + strings.Join(v.Params, ", ") + ")"
	}
	var tags, markers []*Doc
	hidden := library
	for _, it := range v.Items {
		switch {
		case it.Ctor != "" && it.Payload != nil:
			tags = append(tags, Cat(Text("'"+word(it.Ctor, true)+"("), p.set(it.Payload), Text(")")))
		case it.Ctor != "":
			tags = append(tags, Text("'"+word(it.Ctor, true)))
		case it.Success != "":
			markers = append(markers, Text("success '"+word(it.Success, true)))
		case it.Relevant:
			markers = append(markers, Text("relevant"))
		}
		if it.Ctor != "" && exported[it.Ctor] {
			hidden = false
		}
	}
	vis := ""
	if hidden {
		vis = "private "
	}
	d := Cat(Text(vis+"set "+name+" = "), Join(Text(" ∪ "), tags))
	if len(markers) > 0 {
		d = Cat(d, Text(" "), Join(Text(" "), markers))
	}
	return Group(d)
}

// ---- sets, in a claim's positions (spec/j.md §2.9) ----

func (p *printer) set(e core.Expr) *Doc {
	switch e := e.(type) {
	case *core.Var:
		return Text(p.setName(e.Name))
	case *core.Lit:
		return Text(p.lit(e))
	case *core.Apply:
		v, ok := e.Fn.(*core.Var)
		if !ok {
			break
		}
		switch {
		case v.Name == "int" && len(e.Args) == 2:
			return Cat(p.bound(e.Args[0]), Text(".."), p.bound(e.Args[1]))
		case v.Name == "tuple":
			if len(e.Args) == 0 {
				return Text("()")
			}
			ds := make([]*Doc, len(e.Args))
			for i, a := range e.Args {
				ds[i] = p.set(a)
			}
			return Join(Text(" × "), ds)
		case v.Name == "pow" && len(e.Args) == 2:
			return Cat(p.bound(e.Args[0]), Text("^"), p.bound(e.Args[1]))
		}
		ds := make([]*Doc, len(e.Args))
		for i, a := range e.Args {
			ds[i] = p.set(a)
		}
		return Cat(Text(p.setName(v.Name)+"("), Join(Text(", "), ds), Text(")"))
	}
	return p.expr(e)
}

// bound is a range's endpoint: a literal, a constant, or a power.
func (p *printer) bound(e core.Expr) *Doc {
	if a, ok := e.(*core.Apply); ok {
		if v, ok := a.Fn.(*core.Var); ok && v.Name == "pow" && len(a.Args) == 2 {
			return Cat(p.bound(a.Args[0]), Text("^"), p.bound(a.Args[1]))
		}
	}
	return p.prec(e, pRange+1)
}

// setName spells a set's name: the language's and the variants' by the
// convention, a host's as the host spells it.
func (p *printer) setName(n string) string {
	if s, ok := langSets[n]; ok {
		return s
	}
	if p.sets[n] {
		if i := strings.LastIndex(n, "."); i > 0 {
			return Spell(n[:i]) + "." + SetName(n[i+1:])
		}
		return SetName(n)
	}
	return Spell(n)
}

// ---- what a definition's body shows of its names ----

// prescan finds the names a definition uses as data: the table of `len`, a
// store's buffer, a map's `insert` and `keys`, a copy's source.
func (p *printer) prescan(l *core.Lam) {
	var walk func(t core.Tail)
	walk = func(t core.Tail) {
		visit(t, func(e core.Expr) {
			a, ok := e.(*core.Apply)
			if !ok || len(a.Args) == 0 {
				return
			}
			v, ok := a.Fn.(*core.Var)
			if !ok {
				return
			}
			switch v.Name {
			case "len", "keys", "alen", "insert":
				if x, ok := a.Args[0].(*core.Var); ok {
					p.data[x.Name] = true
				}
			case "set":
				if x, ok := a.Args[0].(*core.Var); ok && len(a.Args) == 3 {
					p.data[x.Name] = true
				}
			case "table":
				if len(a.Args) == 2 {
					if x, ok := a.Args[1].(*core.Var); ok {
						p.data[x.Name] = true
					}
				}
			}
		})
	}
	walk(l.Body)
}

func mentions(e core.Expr, name string) bool {
	found := false
	visit(e, func(x core.Expr) {
		if v, ok := x.(*core.Var); ok && v.Name == name {
			found = true
		}
	})
	return found
}
