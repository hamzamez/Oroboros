package core

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Reader for the surface syntax of core-0 §1–2.
//
// Implemented from the spec: UTF-8 required, bidirectional controls rejected,
// identifiers per UAX #31 plus a fixed set of symbol characters, case never
// significant.
//
// GAP: NFC normalisation is specified and NOT checked here — Go's standard
// library has no normaliser, and golang.org/x/text is a dependency the atom
// does not otherwise need. Until it is added, `é` written as U+00E9 and as
// e+U+0301 are two distinct identifiers that display identically. Tracked in
// docs/spec/concerns.md.

// Bidirectional overrides and isolates. Rejecting these closes Trojan Source
// (CVE-2021-42574): source that displays differently than it parses.
func isBidiControl(r rune) bool {
	switch r {
	case 0x202A, 0x202B, 0x202C, 0x202D, 0x202E, // embeddings and overrides
		0x2066, 0x2067, 0x2068, 0x2069: // isolates
		return true
	}
	return false
}

const symbolChars = "-+*/<>=!?_."

func isIdentStart(r rune) bool {
	if strings.ContainsRune(symbolChars, r) {
		return true
	}
	// Approximation of UAX #31 XID_Start using stdlib categories.
	return unicode.IsLetter(r) || r == '_'
}

func isIdentContinue(r rune) bool {
	if strings.ContainsRune(symbolChars, r) {
		return true
	}
	return unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsMark(r)
}

type Form struct {
	Kind  string // "def", "prim", "target", or "term"
	Name  string // def name, target name
	Names []string
	Term  *Term
}

type reader struct {
	src  string
	pos  int
	line int
}

func Read(src string) ([]Form, error) {
	if !utf8.ValidString(src) {
		return nil, fmt.Errorf("source is not valid UTF-8")
	}
	for i, r := range src {
		if isBidiControl(r) {
			return nil, fmt.Errorf("byte %d: bidirectional control U+%04X is not permitted "+
				"(source must display as it parses)", i, r)
		}
	}
	r := &reader{src: src, line: 1}
	var forms []Form
	for {
		r.skipSpace()
		if r.done() {
			return forms, nil
		}
		t, err := r.term()
		if err != nil {
			return nil, err
		}
		f, err := toForm(t)
		if err != nil {
			return nil, err
		}
		forms = append(forms, f)
	}
}

// ReadAll reads raw terms without interpreting def/prim/target. Target files
// are s-expressions but not programs, so they are parsed by their consumer.
func ReadAll(src string) ([]*Term, error) {
	r := &reader{src: src, line: 1}
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

// ReadTerm reads exactly one term, for tests and the REPL.
func ReadTerm(src string) (*Term, error) {
	r := &reader{src: src, line: 1}
	r.skipSpace()
	t, err := r.term()
	if err != nil {
		return nil, err
	}
	r.skipSpace()
	if !r.done() {
		return nil, fmt.Errorf("line %d: trailing input after term", r.line)
	}
	return t, nil
}

func (r *reader) done() bool { return r.pos >= len(r.src) }

func (r *reader) peek() rune {
	if r.done() {
		return -1
	}
	c, _ := utf8.DecodeRuneInString(r.src[r.pos:])
	return c
}

func (r *reader) next() rune {
	c, w := utf8.DecodeRuneInString(r.src[r.pos:])
	r.pos += w
	if c == '\n' {
		r.line++
	}
	return c
}

func (r *reader) skipSpace() {
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

func (r *reader) term() (*Term, error) {
	r.skipSpace()
	if r.done() {
		return nil, fmt.Errorf("line %d: unexpected end of input", r.line)
	}
	if r.peek() == '"' {
		return r.str()
	}
	if r.peek() == '(' {
		return r.list()
	}
	if r.peek() == ')' {
		return nil, fmt.Errorf("line %d: unexpected ')'", r.line)
	}
	return r.atom()
}

const backslash = rune(92)

// str reads a double-quoted literal.
//
// Scanning only has to find the closing quote — strconv.Unquote does the escape
// handling, so the set of escapes we accept is Go's, which is a larger set than
// core-0 specifies. Narrowing it is a spec question, not a reader one, and is
// noted in docs/spec/concerns.md.
func (r *reader) str() (*Term, error) {
	line := r.line
	start := r.pos
	r.next() // opening quote
	for {
		if r.done() {
			return nil, fmt.Errorf("line %d: unterminated string literal", line)
		}
		c := r.next()
		if c == backslash {
			if r.done() {
				return nil, fmt.Errorf("line %d: unterminated escape in string literal", line)
			}
			r.next()
			continue
		}
		if c == '"' {
			break
		}
	}
	v, err := strconv.Unquote(r.src[start:r.pos])
	if err != nil {
		return nil, fmt.Errorf("line %d: bad string literal: %w", line, err)
	}
	return Str(v), nil
}

func (r *reader) list() (*Term, error) {
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
		k, err := r.term()
		if err != nil {
			return nil, err
		}
		kids = append(kids, k)
	}
	if len(kids) == 0 {
		return nil, fmt.Errorf("line %d: empty list is not a term", line)
	}
	// A source-level `let` is SUGAR for an application, and desugars here.
	//
	//   (let e (fn (x) b))  ⟶  ((fn (x) b) e)
	//
	// This is the difference between the two designs we could have had. If `let`
	// stayed a primitive in source, writing one would *prevent* substitution —
	// a knob. It would also be a footgun: a `let` written for readability around
	// a value that later reduces to a λ would silently kill fusion, because the
	// bound name could no longer be substituted into.
	//
	// Instead the programmer's `let` states intent and is erased, and the
	// compiler re-introduces sharing wherever β declines to substitute
	// (gauntlet/results/callbyneed-2026-08-14.md). A `let` in a *residual* can
	// therefore only have come from the reducer, which makes the two roles
	// unambiguous despite sharing a name.
	if kids[0].Kind == KName && kids[0].Name == "let" {
		if len(kids) != 3 {
			return nil, fmt.Errorf("line %d: let takes a value and a continuation", line)
		}
		return &Term{Kind: KApp, Kids: []*Term{kids[2], kids[1]}}, nil
	}

	// (fn (p...) body) is the only special form inside a term.
	if kids[0].Kind == KName && (kids[0].Name == "fn" || kids[0].Name == "λ") {
		if len(kids) != 3 {
			return nil, fmt.Errorf("line %d: fn takes a parameter list and one body", line)
		}
		params, err := paramList(kids[1], line)
		if err != nil {
			return nil, err
		}
		return Fn(params, kids[2]), nil
	}
	return &Term{Kind: KApp, Kids: kids}, nil
}

// paramList reads (a b c). The reader produced it as an application, so it is
// unpacked here rather than parsed specially.
func paramList(t *Term, line int) ([]string, error) {
	if t.Kind == KName {
		return nil, fmt.Errorf("line %d: fn parameters must be a list, got %s", line, t.Name)
	}
	if t.Kind != KApp {
		return nil, fmt.Errorf("line %d: fn parameters must be a list", line)
	}
	params := make([]string, 0, len(t.Kids))
	for _, k := range t.Kids {
		if k.Kind != KName {
			return nil, fmt.Errorf("line %d: fn parameter must be a name, got %s", line, k)
		}
		params = append(params, k.Name)
	}
	return params, nil
}

func (r *reader) atom() (*Term, error) {
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

	// Integer before float, and both before name, so that -1 is a number.
	if v, err := strconv.ParseInt(text, 10, 64); err == nil {
		return Int(v), nil
	}
	if v, err := strconv.ParseFloat(text, 64); err == nil && looksNumeric(text) {
		return Float(v), nil
	}
	first, _ := utf8.DecodeRuneInString(text)
	if !isIdentStart(first) {
		return nil, fmt.Errorf("line %d: %q is not a valid identifier or number", line, text)
	}
	for _, c := range text {
		if !isIdentContinue(c) {
			return nil, fmt.Errorf("line %d: %q contains %q, which is not an identifier character",
				line, text, c)
		}
	}
	return Name(text), nil
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

func toForm(t *Term) (Form, error) {
	if t.Kind != KApp || t.Kids[0].Kind != KName {
		return Form{Kind: "term", Term: t}, nil
	}
	switch t.Kids[0].Name {
	case "def":
		if len(t.Kids) != 3 || t.Kids[1].Kind != KName {
			return Form{}, fmt.Errorf("def takes a name and one term: %s", t)
		}
		return Form{Kind: "def", Name: t.Kids[1].Name, Term: t.Kids[2]}, nil
	case "prim":
		names, err := nameList(t.Kids[1:])
		if err != nil {
			return Form{}, fmt.Errorf("prim: %w", err)
		}
		return Form{Kind: "prim", Names: names}, nil
	case "target":
		if len(t.Kids) < 3 || t.Kids[1].Kind != KName {
			return Form{}, fmt.Errorf("target takes a name and a (prim ...) list: %s", t)
		}
		inner := t.Kids[2]
		if inner.Kind != KApp || inner.Kids[0].Kind != KName || inner.Kids[0].Name != "prim" {
			return Form{}, fmt.Errorf("target %s: expected (prim ...)", t.Kids[1].Name)
		}
		names, err := nameList(inner.Kids[1:])
		if err != nil {
			return Form{}, fmt.Errorf("target %s: %w", t.Kids[1].Name, err)
		}
		return Form{Kind: "target", Name: t.Kids[1].Name, Names: names}, nil
	}
	return Form{Kind: "term", Term: t}, nil
}

func nameList(ts []*Term) ([]string, error) {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		if t.Kind != KName {
			return nil, fmt.Errorf("expected a name, got %s", t)
		}
		out = append(out, t.Name)
	}
	return out, nil
}
