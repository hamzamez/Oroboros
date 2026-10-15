// Package j is the surface J (spec/j.md): its printer, and the translator from
// today's notation (tr_{T→J} = print_J ∘ transport ∘ μ, abstract.md §9.2).
package j

import (
	"strings"
	"unicode/utf8"
)

// THE DOCUMENT ALGEBRA, Wadler's "A prettier printer" (2003) with Leijen's
// alternative line: a document is text, a line break, a concatenation, a nesting
// or a group. A group is laid out flat, every line in it replaced by its flat
// text, when the rest of the line fits the width, and broken otherwise. Documents
// form a monoid under concatenation (empty the unit), and layout is a function of
// the document and the width, so the printer is deterministic by construction.

type kind int

const (
	kText kind = iota
	kLine      // a break, or its flat text when the group is flat
	kHard      // always a break; a group holding one is never flat
	kCat
	kNest
	kGroup
	kForce // prints nothing; a group holding one is never flat
)

type Doc struct {
	k      kind
	s      string // text, or a line's flat text
	indent int
	kids   []*Doc
	block  bool // a block, which an arm keeps on its arrow's line
}

func Text(s string) *Doc { return &Doc{k: kText, s: s} }

// Line breaks, or reads as a space when its group is flat.
func Line() *Doc { return &Doc{k: kLine, s: " "} }

// Soft breaks, or reads as nothing.
func Soft() *Doc { return &Doc{k: kLine, s: ""} }

// Sep breaks, or reads as "; ": statements and arms flat on one line.
func Sep() *Doc { return &Doc{k: kLine, s: "; "} }

// Hard always breaks.
func Hard() *Doc { return &Doc{k: kHard} }

// Force prints nothing and keeps its group from lying flat: what a comment that
// ends a line needs of the line after it.
func Force() *Doc { return &Doc{k: kForce} }

func Cat(ds ...*Doc) *Doc {
	var kids []*Doc
	for _, d := range ds {
		if d != nil {
			kids = append(kids, d)
		}
	}
	return &Doc{k: kCat, kids: kids}
}

func Nest(i int, d *Doc) *Doc { return &Doc{k: kNest, indent: i, kids: []*Doc{d}} }
func Group(d *Doc) *Doc       { return &Doc{k: kGroup, kids: []*Doc{d}} }

// Join puts sep between the documents.
func Join(sep *Doc, ds []*Doc) *Doc {
	var out []*Doc
	for i, d := range ds {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, d)
	}
	return Cat(out...)
}

type item struct {
	indent int
	flat   bool
	d      *Doc
}

// Render lays a document out within width columns.
func Render(d *Doc, width int) string {
	var b strings.Builder
	col := 0
	stack := []item{{0, false, d}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch it.d.k {
		case kText:
			b.WriteString(it.d.s)
			col += utf8.RuneCountInString(it.d.s)
		case kLine:
			if it.flat {
				b.WriteString(it.d.s)
				col += utf8.RuneCountInString(it.d.s)
				continue
			}
			newline(&b, it.indent)
			col = it.indent
		case kHard:
			newline(&b, it.indent)
			col = it.indent
		case kForce:
		case kCat:
			for i := len(it.d.kids) - 1; i >= 0; i-- {
				stack = append(stack, item{it.indent, it.flat, it.d.kids[i]})
			}
		case kNest:
			stack = append(stack, item{it.indent + it.d.indent, it.flat, it.d.kids[0]})
		case kGroup:
			flat := it.flat || fits(width-col, append(stack, item{it.indent, true, it.d.kids[0]}))
			stack = append(stack, item{it.indent, flat, it.d.kids[0]})
		}
	}
	return trimTrailing(b.String())
}

func newline(b *strings.Builder, indent int) {
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(" ", indent))
}

// fits reports whether the items, the last of them first, reach the end of the
// line, or the next break laid out broken, within w columns.
func fits(w int, stack []item) bool {
	s := append([]item(nil), stack...)
	for w >= 0 {
		if len(s) == 0 {
			return true
		}
		it := s[len(s)-1]
		s = s[:len(s)-1]
		switch it.d.k {
		case kText:
			w -= utf8.RuneCountInString(it.d.s)
		case kLine:
			if !it.flat {
				return true
			}
			w -= utf8.RuneCountInString(it.d.s)
		case kHard, kForce:
			return !it.flat
		case kCat:
			for i := len(it.d.kids) - 1; i >= 0; i-- {
				s = append(s, item{it.indent, it.flat, it.d.kids[i]})
			}
		case kNest:
			s = append(s, item{it.indent + it.d.indent, it.flat, it.d.kids[0]})
		case kGroup:
			s = append(s, item{it.indent, it.flat, it.d.kids[0]})
		}
	}
	return false
}

// trimTrailing removes the spaces an indented empty line leaves.
func trimTrailing(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

// Fill lays the documents out like words in a paragraph: each separator breaks
// only when the next document does not fit on the line (Wadler's fillwords).
func Fill(sep string, ds []*Doc) *Doc {
	if len(ds) == 0 {
		return Cat()
	}
	out := []*Doc{ds[0]}
	for _, d := range ds[1:] {
		out = append(out, Text(sep), Group(Cat(Line(), d)))
	}
	return Cat(out...)
}
