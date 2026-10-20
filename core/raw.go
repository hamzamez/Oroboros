package core

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// THE RAW READER: s-expressions in the language's lexical syntax — its atoms,
// literals and strings, with `;` comments as gaps (ADR 0024), and the same
// UTF-8, NFC and bidirectional-control checks as J's lexer — and with the LIST
// structure raw: a list is an application of its elements, and no form is
// desugared. It reads the IR's text (docs/spec/ir.md §2.1) and a core term's
// printed form (ParseTerm), the two texts that are s-expressions now that the
// language's own notation is J (jswitch-2026-10-16, step 5b).

// ReadRaw reads every datum of src. A list may not be empty, except as a
// nullary λ's parameter list, `(fn () b)`, which a core term prints.
func ReadRaw(src string) ([]*Term, error) {
	if err := validSource(src); err != nil {
		return nil, err
	}
	r := &rawReader{src: src, line: 1}
	var out []*Term
	for {
		r.skipSpace()
		if r.done() {
			return out, nil
		}
		t, err := r.term()
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
}

type rawReader struct {
	src  string
	pos  int
	line int
}

func (r *rawReader) done() bool { return r.pos >= len(r.src) }

func (r *rawReader) peek() rune {
	if r.done() {
		return -1
	}
	c, _ := utf8.DecodeRuneInString(r.src[r.pos:])
	return c
}

func (r *rawReader) next() rune {
	c, w := utf8.DecodeRuneInString(r.src[r.pos:])
	r.pos += w
	if c == '\n' {
		r.line++
	}
	return c
}

func (r *rawReader) skipSpace() {
	for !r.done() {
		c := r.peek()
		switch {
		case c == ';':
			for !r.done() && r.peek() != '\n' {
				r.next()
			}
		case unicode.IsSpace(c):
			r.next()
		default:
			return
		}
	}
}

func (r *rawReader) term() (*Term, error) {
	r.skipSpace()
	if r.done() {
		return nil, fmt.Errorf("line %d: unexpected end of input", r.line)
	}
	switch r.peek() {
	case '"':
		return r.str()
	case '(':
		return r.list()
	case ')':
		return nil, fmt.Errorf("line %d: unexpected ')'", r.line)
	}
	return r.atom()
}

// str reads a double-quoted literal with string-literals.md's six escapes, the
// ones J's lexer reads (jlex.go, unquote).
func (r *rawReader) str() (*Term, error) {
	s, n, err := unquote(r.src[r.pos:], r.line)
	if err != nil {
		return nil, err
	}
	r.pos += n // a literal holds no raw line break
	return Str(s), nil
}

func (r *rawReader) list() (*Term, error) {
	line := r.line
	r.next() // '('
	var kids []*Term
	for {
		r.skipSpace()
		if r.done() {
			return nil, fmt.Errorf("line %d: unclosed '('", line)
		}
		if r.peek() == ')' {
			r.next()
			break
		}
		// A nullary λ's parameter list is the one empty list.
		if len(kids) == 1 && kids[0].Kind == KName && kids[0].Name == "fn" && r.peek() == '(' {
			save, saveLine := r.pos, r.line
			r.next()
			r.skipSpace()
			if !r.done() && r.peek() == ')' {
				r.next()
				kids = append(kids, &Term{Kind: KApp})
				continue
			}
			r.pos, r.line = save, saveLine
		}
		k, err := r.term()
		if err != nil {
			return nil, err
		}
		kids = append(kids, k)
	}
	if len(kids) == 0 {
		return nil, fmt.Errorf("line %d: empty list is not a term", line)
	}
	return &Term{Kind: KApp, Kids: kids}, nil
}

// atom reads an integer, a float, a boolean or a name. A literal made only of
// digits IS an integer, and one past int64 is the big literal's spine
// (bigLiteral), never a float.
func (r *rawReader) atom() (*Term, error) {
	line := r.line
	start := r.pos
	for !r.done() {
		c := r.peek()
		if c == '(' || c == ')' || c == ';' || c == '"' || unicode.IsSpace(c) {
			break
		}
		r.next()
	}
	text := r.src[start:r.pos]
	if looksInteger(text) {
		if v, err := strconv.ParseInt(text, 10, 64); err == nil {
			return Int(v), nil
		}
		if t := bigLiteral(text); t != nil {
			return t, nil
		}
		return nil, fmt.Errorf("line %d: %s is not an integer", line, text)
	}
	if v, err := strconv.ParseFloat(text, 64); err == nil && looksNumeric(text) {
		return Float(v), nil
	}
	if text == "true" || text == "false" {
		return Bool(text == "true"), nil
	}
	if err := validName(text); err != nil {
		return nil, fmt.Errorf("line %d: %w", line, err)
	}
	return Name(text), nil
}

// validName accepts a name, which is one or more identifier segments separated
// by `.`. A qualified reference like `words.split-words` is ONE name whose text
// carries the separator; splitting it is resolution's job, not the reader's, so
// that reduction never sees an unresolved name (modules.md §5).
func validName(text string) error {
	for _, seg := range strings.Split(text, ".") {
		if seg == "" {
			return fmt.Errorf("%q has an empty segment; `.` separates qualifiers "+
				"and cannot begin, end, or double", text)
		}
		first, _ := utf8.DecodeRuneInString(seg)
		if !isIdentStart(first) {
			return fmt.Errorf("%q is not a valid identifier or number", text)
		}
		for _, c := range seg {
			if !isIdentContinue(c) {
				return fmt.Errorf("%q contains %q, which is not an identifier character",
					text, c)
			}
		}
	}
	return nil
}

// looksInteger reports whether the text is integer SYNTAX — an optional sign
// and then digits, nothing else.
func looksInteger(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '+' || s[0] == '-' {
		s = s[1:]
	}
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// looksNumeric keeps ParseFloat from swallowing names like `-` or `inf`.
func looksNumeric(s string) bool {
	for _, c := range s {
		if unicode.IsDigit(c) {
			return true
		}
	}
	return false
}
