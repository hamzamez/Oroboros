package core

import "strings"

// THE CONCRETE TREE — docs/spec/abstract.md §9.1, spec/j.md §1.2.
//
// A concrete tree is an abstract tree decorated with layout and gaps: c = (a, g),
// g : Pos(a) → Gap*. Today's reader erases every gap at the first arrow (a comment
// is not a token, comments.md §1), which is right for compiling and wrong for
// translating: a translator must carry each comment to the same abstract position
// in the other rendering (abstract.md §9.2). ReadConcrete is the reading that
// keeps them. It builds the same trees as ReadTrees and, beside them, the source
// span of every tree node and declaration, and every comment with its position;
// a printer attaches each comment to a position by its offset. Nothing else
// changes: the trees are D's input exactly as before, and an ordinary read
// records no span and no gap.

// Span is a half-open byte range of the source, [Start, End).
type Span struct{ Start, End int }

// Gap is one comment: its byte range (the `;` through the last character before
// the newline), its line, its text after the `;`s, and whether it stands on a
// line of its own or trails code.
type Gap struct {
	Start, End int
	Line       int
	Text       string
	OwnLine    bool
}

// Concrete is a module read with its decoration: the declaration trees, the span
// of every tree node and declaration, and the gaps in source order.
type Concrete struct {
	Src   string
	Decls []Decl
	Spans map[any]Span
	Gaps  []Gap
}

// ReadConcrete reads a source as ReadTrees does, keeping its concrete tree.
func ReadConcrete(src string) (*Concrete, error) {
	if err := validSource(src); err != nil {
		return nil, err
	}
	r := newReader(src, false)
	r.concrete, r.spans = true, map[*Term]Span{}
	r, decls, _, err := readModuleWith(r)
	if err != nil {
		return nil, err
	}
	c := &Concrete{Src: src, Spans: map[any]Span{}, Gaps: r.gaps}
	// A node's span is its datum's; a tree a form shares with its operand
	// (`(and x)` is x's tree) takes the union, which still starts and ends
	// where some datum of it does.
	for k, t := range r.abs {
		if sp, ok := r.spans[k]; ok && t != nil {
			c.Spans[t] = union(c.Spans[t], sp, c.has(t))
		}
	}
	at := func(i int) Span { return r.spans[r.top[i]] }
	for _, d := range decls {
		if i, ok := declAt(d); ok && i < len(r.top) {
			c.Spans[d] = at(i)
		}
	}
	decls = carryClaims(decls)
	// A definition that took its `sig`'s claim spans both: a comment above the
	// `sig` belongs to the definition the two now are.
	for _, d := range decls {
		if def, ok := d.(*DefDecl); ok {
			if l, ok := def.Value.(*Lam); ok && l.Claim != nil && l.Claim.At < len(r.top) {
				c.Spans[d] = union(c.Spans[d], at(l.Claim.At), true)
			}
		}
	}
	c.Decls = decls
	return c, nil
}

func (c *Concrete) has(n any) bool { _, ok := c.Spans[n]; return ok }

func union(a, b Span, haveA bool) Span {
	if !haveA {
		return b
	}
	return Span{min(a.Start, b.Start), max(a.End, b.End)}
}

// declAt is the index of the top-level datum a declaration was read from.
func declAt(d Decl) (int, bool) {
	switch d := d.(type) {
	case *DefDecl:
		return d.At, true
	case *ClaimDecl:
		return d.Claim.At, true
	case *VariantDecl:
		return d.At, true
	case *ModuleDecl:
		return d.At, true
	case *UseDecl:
		return d.At, true
	case *ExportDecl:
		return d.At, true
	case *ForeignDecl:
		return d.At, true
	case *TermDecl:
		return d.At, true
	}
	return 0, false
}

// gap records the comment that begins at start, the reader now at its end.
func (r *reader) gap(start, line int) {
	text := strings.TrimLeft(r.src[start:r.pos], ";")
	own := true
	for i := start - 1; i >= 0 && r.src[i] != '\n'; i-- {
		if r.src[i] != ' ' && r.src[i] != '\t' && r.src[i] != '\r' {
			own = false
			break
		}
	}
	r.gaps = append(r.gaps, Gap{Start: start, End: r.pos, Line: line, Text: text, OwnLine: own})
}
