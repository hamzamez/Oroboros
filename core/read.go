package core

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	unicodenorm "golang.org/x/text/unicode/norm"
)

// Reader for the surface syntax of core-0 §1–2.
//
// Implemented from the spec: UTF-8 required, NFC required, bidirectional
// controls rejected, identifiers per UAX #31 plus a fixed set of symbol
// characters, and case never assigning meaning — though names remain
// case-SENSITIVE for identity, which is a different property.

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

// `.` is NOT here. It is the qualifier separator (modules.md §3), and it was
// free to reserve because no name in targets/ or examples/ contained one.
// `/` stays an ordinary identifier character, so a module path like `go/strings`
// is a single segment rather than a compound.
const symbolChars = "-+*/<>=!?_"

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

// A Sig is a declared signature on a module export: argument names and types,
// and a result type. The names are carried even though nothing reads them yet,
// because a refinement attaches to a NAME — `(where (= (alen a) (alen b)))` —
// and adding them later would change the one thing that cannot be taken back.
type Sig struct {
	Params []SigParam
	Result string
	Where  *Term // a boolean term over the parameter names, or nil
}

type SigParam struct{ Name, Type string }

type Form struct {
	Kind  string // "def", "sig", "module", "use", "export", "prim", "target", or "term"
	Name  string // def name, target name, module path, imported path
	Alias string // `use`: the name the import is bound to
	Names []string
	Term  *Term
	Sig   *Sig
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
	// NFC, per core-0 §1.1. Rejected rather than normalised, for the same
	// reason invalid UTF-8 is rejected rather than repaired: silently rewriting
	// the input would mean the file on disk is not the file that was compiled.
	//
	// Without this, `é` as U+00E9 and as e+U+0301 are two DISTINCT identifiers
	// that display identically — which is the same class of hazard as the
	// bidirectional controls below, and was open from the first commit.
	if !unicodenorm.NFC.IsNormalString(src) {
		// Report the first prefix that is not normal, so the message points at
		// the offending sequence rather than at the file.
		at := len(src)
		for j := range src {
			if !unicodenorm.NFC.IsNormalString(src[:j]) {
				at = j
				break
			}
		}
		return nil, fmt.Errorf("source is not NFC-normalised, at or before byte %d; "+
			"two identifiers can look identical and not be equal. Save the file as NFC.", at)
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
		// `()` is not a term, but it IS a legal parameter list: `(fn () b)` is
		// a nullary abstraction, which is what a program's entry point has to
		// be (build.md §2). Nothing else may be empty.
		if len(kids) == 1 && isFnHead(kids[0]) && r.peek() == '(' {
			save := r.pos
			r.next()
			r.skipSpace()
			if !r.done() && r.peek() == ')' {
				r.next()
				kids = append(kids, &Term{Kind: KApp}) // the empty parameter list
				continue
			}
			r.pos = save
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

	// `seq` is the same trick with the binder thrown away, and it is the whole
	// of our sequencing construct — no statement form, no unit type.
	//
	//   (seq a b c)  ⟶  ((fn (_) ((fn (_) c) b)) a)
	//
	// It works only because β denies weakening to impure terms (effects.md §5):
	// `_` occurs zero times, so a pure `a` is correctly deleted and an impure
	// `a` is correctly kept. The hazard g5 did not list is the one that makes
	// sequencing expressible at all.
	if kids[0].Kind == KName && kids[0].Name == "seq" {
		if len(kids) < 3 {
			return nil, fmt.Errorf("line %d: seq takes two or more terms", line)
		}
		t := kids[len(kids)-1]
		for i := len(kids) - 2; i >= 1; i-- {
			t = &Term{Kind: KApp, Kids: []*Term{Fn([]string{"_"}, t), kids[i]}}
		}
		return t, nil
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
// isFnHead reports whether this kid makes the enclosing list an abstraction,
// which is the one place an empty list is admissible.
func isFnHead(t *Term) bool {
	return t.Kind == KName && (t.Name == "fn" || t.Name == "λ")
}

func paramList(t *Term, line int) ([]string, error) {
	if t.Kind == KName {
		return nil, fmt.Errorf("line %d: fn parameters must be a list, got %s", line, t.Name)
	}
	if t.Kind != KApp {
		return nil, fmt.Errorf("line %d: fn parameters must be a list", line)
	}
	params := make([]string, 0, len(t.Kids))
	seen := make(map[string]bool, len(t.Kids))
	for _, k := range t.Kids {
		if k.Kind != KName {
			return nil, fmt.Errorf("line %d: fn parameter must be a name, got %s", line, k)
		}
		// A repeated binder in ONE abstraction is ill-formed. β substitutes
		// parameter by parameter, so the later argument silently won and the
		// earlier one vanished: ((fn (x x) x) 1 2) reduced to 2, with no way to
		// name the first x at all.
		//
		// Nested shadowing — (fn (x) (fn (x) …)) — is unaffected and still
		// legal, because those are two abstractions.
		// A binder must be a SIMPLE name. `.` is the qualifier separator, and a
		// qualified name denotes a module member — a λ cannot bind into a
		// module. Allowing it let ((fn (f64.add) (f64.add 1.0 2.0)) 9.0) reduce
		// to (9.0 1.0 2.0): a parameter shadowed a module-qualified primitive
		// and reduction happily applied a number to two arguments.
		if strings.Contains(k.Name, ".") {
			return nil, fmt.Errorf("line %d: %s cannot be a parameter; a binder is a simple "+
				"name, and `.` qualifies a module member", line, k.Name)
		}
		if seen[k.Name] {
			return nil, fmt.Errorf("line %d: fn binds %s twice; a parameter list may not repeat "+
				"a name, because the second would silently shadow the first", line, k.Name)
		}
		seen[k.Name] = true
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
	case "module":
		if len(t.Kids) != 2 || t.Kids[1].Kind != KName {
			return Form{}, fmt.Errorf("module takes one path: %s", t)
		}
		return Form{Kind: "module", Name: t.Kids[1].Name}, nil
	case "use":
		// (use PATH) binds the last path segment; (use PATH as A) binds A.
		if t.Kids[1].Kind != KName {
			return Form{}, fmt.Errorf("use takes a module path: %s", t)
		}
		f := Form{Kind: "use", Name: t.Kids[1].Name, Alias: lastSegment(t.Kids[1].Name)}
		switch len(t.Kids) {
		case 2:
		case 4:
			if t.Kids[2].Kind != KName || t.Kids[2].Name != "as" || t.Kids[3].Kind != KName {
				return Form{}, fmt.Errorf("use takes (use PATH) or (use PATH as ALIAS): %s", t)
			}
			f.Alias = t.Kids[3].Name
		default:
			return Form{}, fmt.Errorf("use takes (use PATH) or (use PATH as ALIAS): %s", t)
		}
		return f, nil
	case "sig":
		// (sig NAME ((p TYPE)…) RESULT)
		if len(t.Kids) < 4 || t.Kids[1].Kind != KName || t.Kids[3].Kind != KName {
			return Form{}, fmt.Errorf("sig takes a name, a parameter list and a result type: %s", t)
		}
		sig := &Sig{Result: t.Kids[3].Name}
		for _, rest := range t.Kids[4:] {
			if rest.Kind == KApp && rest.Kids[0].Kind == KName &&
				rest.Kids[0].Name == "where" && len(rest.Kids) == 2 {
				sig.Where = rest.Kids[1]
				continue
			}
			return Form{}, fmt.Errorf("sig %s: unexpected %s", t.Kids[1].Name, rest)
		}
		if t.Kids[2].Kind == KApp {
			for _, a := range t.Kids[2].Kids {
				switch {
				case a.Kind == KName:
					// A bare type, positional — the same shape a target file's
					// (prim …) uses today.
					sig.Params = append(sig.Params, SigParam{Type: a.Name})
				case a.Kind == KApp && len(a.Kids) == 2 &&
					a.Kids[0].Kind == KName && a.Kids[1].Kind == KName:
					sig.Params = append(sig.Params, SigParam{a.Kids[0].Name, a.Kids[1].Name})
				default:
					return Form{}, fmt.Errorf("sig %s: a parameter is TYPE or (name TYPE), got %s",
						t.Kids[1].Name, a)
				}
			}
		}
		return Form{Kind: "sig", Name: t.Kids[1].Name, Sig: sig}, nil
	case "export":
		names, err := nameList(t.Kids[1:])
		if err != nil {
			return Form{}, fmt.Errorf("export: %w", err)
		}
		return Form{Kind: "export", Names: names}, nil
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

// lastSegment is the default alias for an import: `go/strings` binds `strings`.
// Path separators are `/`; the qualifier separator is `.`.
func lastSegment(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}
	return path
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
