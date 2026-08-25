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
// `%&|^~` were added when targets began declaring their host's operators under
// the host's own names — `go.%`, `go.&`, `go.|`. A target module that has to
// rename `%` to `rem` is teaching the reader's limitations rather than the
// host's, which is the opposite of what a parasite target file is for.
const symbolChars = "-+*/<>=!?_%&|^~"

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
	// Results is populated ONLY when the signature declares more than one —
	// `(sig divmod ((a int) (b int)) (int int))`. One result stays in Result,
	// so every existing path is untouched and multi-result code asks for it
	// explicitly. A function with several results is the NEGATIVE PRODUCT
	// (data-structures.md section 4.5): three of our four targets have a
	// native form and it is not a tuple.
	Results []string
	Where   *Term // a boolean term over the parameter names, or nil
}

type SigParam struct{ Name, Type string }

type Form struct {
	Kind  string // "def", "sig", "module", "use", "export", "prim", "target", or "term"
	Name  string // def name, target name, module path, imported path
	Alias string // `use`: the name the import is bound to
	Names []string
	Term  *Term
	Sig   *Sig
	Sum   *Sum
}

// Sum is a closed, finite, NON-RECURSIVE sum type — Σ over a finite index set,
// the exact dual of a table's Π (docs/sums-research.md §1.2).
//
// The difference from the table is the whole design: a Π can be given by a RULE
// and store nothing, while a Σ must carry WHICH — the tag is information the
// caller does not have, and it has to be transmitted. So a sum value is a tag
// and a payload, which is a PRODUCT, and we already built the product on all
// four targets (values.md). Go's own `(T, error)` idiom is this exact shape.
//
// Sums are NAMED, products anonymous, and that is forced rather than chosen:
// `(ok 3)` does not determine its type, which is why every language without
// runtime types went nominal here.
type Sum struct {
	Name     string
	Variants []Variant
}

// Variant is one summand. Payload is "" for a variant that carries nothing —
// the degenerate case, which makes an enum a sum with no payloads rather than a
// separate concept.
type Variant struct {
	Name    string
	Payload string
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

	// ---- Booleans and control flow, all four definitional (booleans.md 4.2).
	//
	// McCarthy 1960: a conditional CANNOT be a function, because a function in
	// a strict language receives every argument evaluated and a conditional
	// must not evaluate the branch it does not take. `and` and `or` inherit
	// that, which is why R7RS derives them and why the SML Definition makes
	// them syntax. None of the four survives this function.
	//
	//   (and a b …)  ⟶  (if a (and b …) false)
	//   (or  a b …)  ⟶  (if a true (or b …))
	//   (not a)      ⟶  (if a false true)
	//
	// `or` needs no `let` — Scheme's does only because its `or` returns the
	// VALUE of the first true operand, over arbitrary values. Over the
	// two-element bool, `(if a true b)` evaluates `a` once and is exact.
	if kids[0].Kind == KName {
		switch kids[0].Name {
		case "and", "or":
			short := kids[0].Name == "or"
			if len(kids) == 1 {
				return Bool(!short), nil // (and) is true, (or) is false — the units
			}
			t := kids[len(kids)-1]
			for i := len(kids) - 2; i >= 1; i-- {
				if short {
					t = &Term{Kind: KApp, Kids: []*Term{Name("if"), kids[i], Bool(true), t}}
				} else {
					t = &Term{Kind: KApp, Kids: []*Term{Name("if"), kids[i], t, Bool(false)}}
				}
			}
			return t, nil
		case "not":
			if len(kids) != 2 {
				return nil, fmt.Errorf("line %d: not takes one term", line)
			}
			return &Term{Kind: KApp,
				Kids: []*Term{Name("if"), kids[1], Bool(false), Bool(true)}}, nil
		case "cond":
			return clauseChain(kids[1:], "cond", line, nil)
		case "values":
			// (values a b …)  ⟶  (fn (k) (k a b …))
			//
			// The NEGATIVE PRODUCT, and it is sugar because beta already is its
			// algebra: a caller that consumes it in the same place reduces the
			// whole thing away, which is why it measured 1.01x with zero
			// allocations (product-2026-08-19). What survives reduction is a
			// function whose value is a selector-taking lambda, and THAT is
			// what a target with a native multiple-return emits.
			//
			// Scheme's `values` and Common Lisp's are deliberately not data
			// structures, for the same reason: an implementation should return
			// several results in registers rather than box them to unbox them.
			if len(kids) < 3 {
				return nil, fmt.Errorf("line %d: values takes two or more terms; "+
					"one value is just the value", line)
			}
			// The binder's name starts with `#`, which is not isIdentStart, so
			// no source term can contain a free occurrence of it and `Fn`
			// cannot capture one. `seq` uses `_`, which a user COULD write.
			app := append([]*Term{Name("#k")}, kids[1:]...)
			return Fn([]string{"#k"}, &Term{Kind: KApp, Kids: app}), nil
		}
	}

	// (loop ((x z)…) c e … else e) — docs/spec/iteration.md.
	//
	//   (loop ((acc 0.0) (i 0))
	//     (int.lt i n)  (again (f.add acc (aindex a i)) (int.add i 1))
	//     else          acc)
	//
	// desugars to
	//
	//   (loop (fn (acc i) (if (int.lt i n) (again …) acc)) 0.0 0)
	//
	// The binders are an ordinary `fn`, so the locally nameless representation,
	// capture-avoidance and the emitter's openFresh all work unchanged — the
	// same move `let` makes. The clause chain is ordinary `if`s, so reduction
	// needs no new rule either. What is left needing new machinery is exactly
	// one head, `loop`, and one marker, `again`.
	if kids[0].Kind == KName && kids[0].Name == "loop" {
		return readLoop(kids, line)
	}
	if kids[0].Kind == KName && kids[0].Name == "match" {
		return readMatch(kids, line)
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
	//
	// A literal made only of digits IS an integer, and one too large for int64
	// is an ERROR rather than a float. It used to fall through to ParseFloat,
	// which succeeds — so `9223372036854775808` silently became
	// `9.223372036854776e+18` and the program's type changed underneath it, at
	// a threshold ten bits past the portable window and mentioned in no
	// specification (data-model.md §1.1).
	if looksInteger(text) {
		v, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: %s does not fit in an integer; the portable range "+
				"is ±(2^53−1) and the widest target is 64 bits (docs/spec/arithmetic.md §4)",
				line, text)
		}
		return Int(v), nil
	}
	if v, err := strconv.ParseFloat(text, 64); err == nil && looksNumeric(text) {
		return Float(v), nil
	}
	// The two boolean literals. They are read here rather than declared by a
	// target because the reader's own desugaring of `and` has to PRODUCE one,
	// and the reader does not know which target it is reading for — it could
	// not emit `go.false` or `x64.false` even if it wanted to (booleans.md 4.1).
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
// and then digits, nothing else. Being an integer literal is a property of how
// it is written, not of whether it happens to fit.
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

func toForm(t *Term) (Form, error) {
	if t.Kind != KApp || t.Kids[0].Kind != KName {
		return Form{Kind: "term", Term: t}, nil
	}
	switch t.Kids[0].Name {
	case "def":
		if len(t.Kids) != 3 || t.Kids[1].Kind != KName {
			return Form{}, fmt.Errorf("def takes a name and one term: %s", t)
		}
		// A definition names a member of THIS module, so `.` cannot appear: a
		// qualified name in a term always means an import (modules.md §3), so
		// `(def a.b …)` defined something no term could ever refer to. Silent,
		// and dead. Same shape as the `fn` parameter rule.
		if strings.Contains(t.Kids[1].Name, ".") {
			return Form{}, fmt.Errorf("def %s: a definition names a member of this module, "+
				"and `.` qualifies a member of an imported one", t.Kids[1].Name)
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
		// (sig NAME ((p TYPE)…) RESULT)  or  (sig NAME ((p TYPE)…) (R1 R2 …))
		if len(t.Kids) < 4 || t.Kids[1].Kind != KName {
			return Form{}, fmt.Errorf("sig takes a name, a parameter list and a result type: %s", t)
		}
		sig := &Sig{}
		switch r := t.Kids[3]; {
		case r.Kind == KName:
			sig.Result = r.Name
		case r.Kind == KApp:
			// Several results. `(R)` with one entry is the same as a bare R,
			// so there is one spelling for one result and no ambiguity.
			for _, a := range r.Kids {
				if a.Kind != KName {
					return Form{}, fmt.Errorf("sig %s: a result is a type name, got %s",
						t.Kids[1].Name, a)
				}
				sig.Results = append(sig.Results, a.Name)
			}
			if len(sig.Results) == 0 {
				return Form{}, fmt.Errorf("sig %s: the result list is empty", t.Kids[1].Name)
			}
			if len(sig.Results) == 1 {
				sig.Result, sig.Results = sig.Results[0], nil
			}
		default:
			return Form{}, fmt.Errorf("sig takes a name, a parameter list and a result type: %s", t)
		}
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
					a.Kids[0].Kind == KName && a.Kids[0].Name == "array" &&
					a.Kids[1].Kind == KName:
					// `(array f64)` is a TYPE, positional. It is ambiguous with
					// `(name TYPE)` by shape alone, so `array` wins — which is
					// why a parameter may not be named `array`, checked below.
					sig.Params = append(sig.Params, SigParam{Type: TypeName(a)})
				case a.Kind == KApp && len(a.Kids) == 2 &&
					a.Kids[0].Kind == KName && a.Kids[1].Kind == KName:
					sig.Params = append(sig.Params, SigParam{a.Kids[0].Name, a.Kids[1].Name})
				case a.Kind == KApp && len(a.Kids) == 2 &&
					a.Kids[0].Kind == KName && a.Kids[1].Kind == KApp:
					// (name (array f64)) — named, with a compound type.
					if ty := TypeName(a.Kids[1]); ty != "" {
						sig.Params = append(sig.Params, SigParam{a.Kids[0].Name, ty})
						continue
					}
					return Form{}, fmt.Errorf("sig %s: %s is not a type", t.Kids[1].Name, a.Kids[1])
				default:
					return Form{}, fmt.Errorf("sig %s: a parameter is TYPE or (name TYPE), got %s",
						t.Kids[1].Name, a)
				}
			}
		}
		return Form{Kind: "sig", Name: t.Kids[1].Name, Sig: sig}, nil
	case "sum":
		sum, err := readSum(t)
		if err != nil {
			return Form{}, err
		}
		return Form{Kind: "sum", Name: sum.Name, Sum: sum}, nil
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

// readLoop desugars the surface loop form. See iteration.md §2.
//
// The clause-body restriction is checked HERE, before desugaring, because
// afterwards a clause body and an `if` branch are the same thing. That is what
// keeps the clause list the loop's complete control flow: `again` may be a
// clause body or sit under a `let`, but never under an `if`.
func readLoop(kids []*Term, line int) (*Term, error) {
	if len(kids) < 4 {
		return nil, fmt.Errorf("line %d: loop takes a binding list and at least one "+
			"clause ending in `else`", line)
	}
	names, inits, err := loopBindings(kids[1], line)
	if err != nil {
		return nil, err
	}
	clauses := kids[2:]
	body, err := clauseChain(clauses, "loop", line, func(b *Term) error {
		return checkClauseBody(b, len(names), line)
	})
	if err != nil {
		return nil, err
	}
	return loopOrLet(names, body, inits), nil
}

// hasAgain reports whether a clause chain ever jumps back.
//
// A `loop` that never repeats is not a loop. It is a `let` of its variables
// around a conditional, which is exactly what the application below IS —
// `(let e k)` reads as `(k e)`, so dropping the `loop` name leaves a β-redex
// and the reducer does the rest.
//
// This is what lets `match` be used as a plain conditional at NO cost. Without
// it, `(match (t) 0 1 else 2)` emitted `for { … break }` with a result variable
// on every backend, where a hand-written form is an `if` — so `match` was
// paying for iteration it did not use, and any sum eliminator built on `match`
// would have paid it on every error check.
func hasAgain(t *Term) bool {
	if t == nil {
		return false
	}
	if isAgain(t) {
		return true
	}
	for _, k := range t.Kids {
		if hasAgain(k) {
			return true
		}
	}
	return false
}

// loopOrLet builds the `loop`, or drops it when nothing jumps back.
func loopOrLet(names []string, body *Term, inits []*Term) *Term {
	fn := Fn(names, body)
	if !hasAgain(body) {
		return &Term{Kind: KApp, Kids: append([]*Term{fn}, inits...)}
	}
	return &Term{Kind: KApp, Kids: append([]*Term{Name("loop"), fn}, inits...)}
}

// readMatch desugars `match` into `loop`, which is the whole implementation.
//
//	(match (e1 … en)
//	  p11 … p1n            body1
//	  p21 … p2n (when c)   body2
//	  else                 bodyk)
//
// becomes
//
//	(loop ((#m0 e1) … (#mn-1 en))
//	  guard1  body1
//	  guard2  body2
//	  else    bodyk)
//
// docs/type-algebra.md §5: `loop` already gives guarded clauses over n
// variables and `match` gives pattern clauses over n scrutinees — a boolean
// guard IS a pattern on a bool, so they were the same construct. Building it
// this way costs ZERO reduction rules and ZERO term kinds; it joins `let`,
// `seq`, `and`, `or`, `not`, `cond` and `loop` as sugar that erases.
//
// `again` works in a clause body because it is the LOOP's `again`, and that is
// what makes a state machine writable: match on (state, input), transition with
// `again` — the shape a parser, an event loop and a protocol handler all have.
//
// A pattern is one of:
//
//	_            wildcard — matches, binds nothing
//	name         binds the scrutinee under that name
//	true/false   tests the scrutinee, which is already a bool
//	an integer   tests it with `=`
//
// Float and string patterns are deliberately absent: the language has no
// portable equality (`==` is target-native on all four), and a float pattern
// would inherit IEEE's NaN, which is not the equivalence relation a pattern
// needs.
//
// **`when` is not decoration.** Without it `match` is strictly WEAKER than
// `loop`, and building it is what showed that: ADR 0015 forbids `again` under
// an `if`, so a condition that guards a transition cannot live in the body — it
// has to be a clause. `(when c)` is how a clause says a condition that patterns
// cannot express, and `c` sees the names the patterns bound.
func readMatch(kids []*Term, line int) (*Term, error) {
	if len(kids) < 3 {
		return nil, fmt.Errorf("line %d: match takes a scrutinee list and at least one "+
			"clause ending in `else`", line)
	}
	scrut := kids[1]
	if scrut.Kind != KApp || len(scrut.Kids) == 0 {
		return nil, fmt.Errorf("line %d: match takes a LIST of scrutinees — (match (a b) …) — "+
			"so that clauses need no tuple built and taken apart; got %s", line, scrut)
	}
	n := len(scrut.Kids)

	// A scrutinee that is a bare NAME becomes the loop variable under that same
	// name, and this is not cosmetic. `(match (s i) …)` initialises the loop from
	// `s` and `i`; if the loop variables were fresh, a clause body reading `i`
	// would see the OUTER `i` — the value the loop started from — while `again`
	// advanced a hidden one. Every iteration after the first would read a stale
	// value, and the program would look right. Reusing the name shadows the outer
	// binding, which is what the state-machine reading of `match` means: `s` and
	// `i` are the state.
	//
	// Anything else — a literal, a call — gets a fresh `#m` name. `#` is not
	// isIdentStart, so no source term can contain a free occurrence of one and
	// `Fn` cannot capture it.
	vars := make([]string, n)
	seen := map[string]bool{}
	for i, e := range scrut.Kids {
		if e.Kind == KName && !seen[e.Name] && e.Name != "_" && e.Name != "else" {
			vars[i] = e.Name
			seen[e.Name] = true
			continue
		}
		vars[i] = fmt.Sprintf("#m%d", i)
	}

	rest := kids[2:]
	var pairs []*Term
	for i := 0; i < len(rest); {
		if rest[i].Kind == KName && rest[i].Name == "else" {
			if i+2 != len(rest) {
				return nil, fmt.Errorf("line %d: `else` must be the last clause of match; "+
					"the ones after it could never be reached", line)
			}
			pairs = append(pairs, Name("else"), rest[i+1])
			break
		}
		if i+n >= len(rest) {
			return nil, fmt.Errorf("line %d: match has %d scrutinees, so every clause is %d "+
				"pattern(s), an optional (when …), and a body; the last clause is short",
				line, n, n)
		}
		guard, binds, err := matchClause(rest[i:i+n], vars, line)
		if err != nil {
			return nil, err
		}
		k := i + n
		// An optional `(when c)` between the patterns and the body.
		if k < len(rest) && rest[k].Kind == KApp && len(rest[k].Kids) == 2 &&
			rest[k].Kids[0].Kind == KName && rest[k].Kids[0].Name == "when" {
			c := renameFree(rest[k].Kids[1], binds)
			if guard.Kind == KBool && guard.IsTrue() {
				// Patterns that test nothing: the `when` IS the guard, rather
				// than `(if true c false)`, which is the same thing spelled worse.
				guard = c
			} else {
				guard = &Term{Kind: KApp, Kids: []*Term{Name("if"), guard, c, Bool(false)}}
			}
			k++
		}
		if k >= len(rest) {
			return nil, fmt.Errorf("line %d: a match clause needs a body after its patterns", line)
		}
		pairs = append(pairs, guard, renameFree(rest[k], binds))
		i = k + 1
	}

	body, err := clauseChain(pairs, "match", line, func(b *Term) error {
		return checkClauseBody(b, n, line)
	})
	if err != nil {
		return nil, err
	}
	return loopOrLet(vars, body, scrut.Kids), nil
}

// matchClause turns one clause's patterns into a guard and a renaming.
//
// A name pattern is a RENAME rather than a `let`: the pattern variable is just
// another name for the loop variable, and renaming needs no binder. It is also
// what lets a `(when …)` see the bound names, which a `let` wrapping only the
// body could not.
func matchClause(pats []*Term, vars []string, line int) (*Term, map[string]string, error) {
	var tests []*Term
	binds := map[string]string{}
	for i, p := range pats {
		v := Name(vars[i])
		switch {
		case p.Kind == KName && p.Name == "_":
			// matches, binds nothing

		case p.Kind == KName && p.Name == "else":
			return nil, nil, fmt.Errorf("line %d: `else` is a clause of its own and takes no "+
				"patterns; it cannot appear in position %d", line, i+1)

		case p.Kind == KName:
			if _, dup := binds[p.Name]; dup {
				return nil, nil, fmt.Errorf("line %d: %s is bound twice in one match clause; "+
					"a repeated name would be an equality test, which patterns do not do",
					line, p.Name)
			}
			binds[p.Name] = vars[i]

		case p.Kind == KBool:
			// The scrutinee is already a bool, so the test IS the scrutinee.
			if p.IsTrue() {
				tests = append(tests, v)
			} else {
				tests = append(tests, &Term{Kind: KApp,
					Kids: []*Term{Name("if"), v, Bool(false), Bool(true)}})
			}

		case p.Kind == KInt:
			tests = append(tests, &Term{Kind: KApp, Kids: []*Term{Name("="), v, p}})

		default:
			return nil, nil, fmt.Errorf("line %d: %s is not a pattern. A pattern is `_`, a name "+
				"that binds, `true`/`false`, or an integer; float and string patterns are absent "+
				"because the language has no portable equality", line, p)
		}
	}
	if len(tests) == 0 {
		return Bool(true), binds, nil
	}
	guard := tests[len(tests)-1]
	for i := len(tests) - 2; i >= 0; i-- {
		// Conjunction, spelled the way `and` desugars: (if a b false).
		guard = &Term{Kind: KApp, Kids: []*Term{Name("if"), tests[i], guard, Bool(false)}}
	}
	return guard, binds, nil
}

// renameFree replaces free names according to `binds`.
//
// Safe without capture analysis, and the reason is worth stating: by the time
// readMatch runs, every inner `fn` has already been through `Fn`, which closes
// its body — so an occurrence bound by a nested binder is a KBound, not a
// KName. Only genuinely free names remain to rename.
func renameFree(t *Term, binds map[string]string) *Term {
	if t == nil || len(binds) == 0 {
		return t
	}
	if t.Kind == KName {
		if v, ok := binds[t.Name]; ok {
			return Name(v)
		}
		return t
	}
	if len(t.Kids) == 0 {
		return t
	}
	out := &Term{Kind: t.Kind, Name: t.Name, Int: t.Int, Float: t.Float, Str: t.Str,
		Params: t.Params, Index: t.Index, Depth: t.Depth,
		Kids: make([]*Term, len(t.Kids))}
	for i, k := range t.Kids {
		out.Kids[i] = renameFree(k, binds)
	}
	return out
}

// clauseChain folds `c₁ e₁ … else e` into a right-nested chain of `if`.
//
// Shared by `loop` and by `cond`, which is the same syntax with `again`
// removed — first match wins, `else` mandatory, so every way out is written
// down (ADR 0015, and McCarthy 1960 for the rule that the first true clause is
// the one taken).
//
// `check` is the only difference between the two callers: a loop restricts what
// a clause body may be, a `cond` does not.
func clauseChain(clauses []*Term, what string, line int, check func(*Term) error) (*Term, error) {
	if len(clauses) < 2 {
		return nil, fmt.Errorf("line %d: %s needs at least an `else` clause", line, what)
	}
	if len(clauses)%2 != 0 {
		return nil, fmt.Errorf("line %d: %s clauses come in pairs — a condition and a "+
			"result — and the last condition must be `else`", line, what)
	}
	last := clauses[len(clauses)-2]
	if last.Kind != KName || last.Name != "else" {
		return nil, fmt.Errorf("line %d: the last clause of %s must be `else`, so that every "+
			"path out is written down; got %s", line, what, last)
	}
	for i := 0; i < len(clauses); i += 2 {
		if i < len(clauses)-2 && clauses[i].Kind == KName && clauses[i].Name == "else" {
			return nil, fmt.Errorf("line %d: `else` must be the last clause; the ones after "+
				"it could never be reached", line)
		}
		if check != nil {
			if err := check(clauses[i+1]); err != nil {
				return nil, err
			}
		}
	}
	// Right to left. The `else` body is the chain's tail, so no condition is
	// emitted for it.
	body := clauses[len(clauses)-1]
	for i := len(clauses) - 4; i >= 0; i -= 2 {
		body = &Term{Kind: KApp, Kids: []*Term{Name("if"), clauses[i], clauses[i+1], body}}
	}
	return body, nil
}

// loopBindings reads ((x z) (y w)) into names and initial values.
func loopBindings(t *Term, line int) ([]string, []*Term, error) {
	if t.Kind != KApp {
		return nil, nil, fmt.Errorf("line %d: a loop's bindings are a list of (name init), "+
			"got %s", line, t)
	}
	var names []string
	var inits []*Term
	seen := map[string]bool{}
	for _, b := range t.Kids {
		if b.Kind != KApp || len(b.Kids) != 2 || b.Kids[0].Kind != KName {
			return nil, nil, fmt.Errorf("line %d: a loop binding is (name init), got %s", line, b)
		}
		n := b.Kids[0].Name
		switch {
		case n == "again" || n == "else":
			return nil, nil, fmt.Errorf("line %d: %s is reserved by `loop` and cannot be a "+
				"loop variable", line, n)
		case strings.Contains(n, "."):
			return nil, nil, fmt.Errorf("line %d: %s cannot be a loop variable; a binder is a "+
				"simple name, and `.` qualifies a module member", line, n)
		case seen[n]:
			return nil, nil, fmt.Errorf("line %d: loop binds %s twice", line, n)
		}
		seen[n] = true
		names = append(names, n)
		inits = append(inits, b.Kids[1])
	}
	if len(names) == 0 {
		return nil, nil, fmt.Errorf("line %d: a loop needs at least one variable", line)
	}
	return names, inits, nil
}

// checkClauseBody enforces iteration.md §2: `again` may be the whole of a clause
// body, or sit under a `let`, but never under an `if` or as an argument.
//
//	let binds; if branches. Binding may wrap an `again`, branching may not.
func checkClauseBody(t *Term, arity, line int) error {
	if isAgain(t) {
		if got := len(t.Kids) - 1; got != arity {
			return fmt.Errorf("line %d: again takes %d argument(s), one per loop variable, "+
				"given %d", line, arity, got)
		}
		return nil
	}
	// (let e (fn (x) k)) has already been desugared to ((fn (x) k) e).
	if t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KFn && len(t.Kids[0].Params) == 1 {
		if err := checkClauseBody(t.Kids[0].Body(), arity, line); err != nil {
			return err
		}
		return noAgain(t.Kids[1], line)
	}
	return noAgain(t, line)
}

func isAgain(t *Term) bool {
	return t.Kind == KApp && t.Kids[0].Kind == KName && t.Kids[0].Name == "again"
}

// noAgain rejects `again` anywhere inside a term that is not a tail position.
func noAgain(t *Term, line int) error {
	if isAgain(t) {
		return fmt.Errorf("line %d: `again` may be a clause body, or sit under a `let`, but "+
			"not under an `if` or inside an expression — write another clause instead, so "+
			"the clause list stays the loop's whole control flow", line)
	}
	switch t.Kind {
	case KFn:
		return noAgain(t.Body(), line)
	case KApp:
		for _, k := range t.Kids {
			if err := noAgain(k, line); err != nil {
				return err
			}
		}
	}
	return nil
}

// readSum reads `(sum name (variant type) … )`.
//
//	(sum result (ok int) (err int))
//	(sum shape circle square triangle)     ; no payloads — an enum
//
// Closed, finite and NON-RECURSIVE, all three deliberately
// (docs/sums-research.md §4): a recursive sum is refused because a JSON node is
// a non-recursive sum plus indices into a table, which measured 2.02x FASTER on
// irregular access than the pointer-chasing form. So `μ` buys nothing here and
// costs the size-change termination argument.
//
// A variant with no payload is the degenerate case rather than a separate
// concept, which is why an enum needs nothing added.
func readSum(t *Term) (*Sum, error) {
	if len(t.Kids) < 3 || t.Kids[1].Kind != KName {
		return nil, fmt.Errorf("sum takes a name and at least two variants: %s", t)
	}
	sum := &Sum{Name: t.Kids[1].Name}
	seen := map[string]bool{}
	for _, k := range t.Kids[2:] {
		var v Variant
		switch {
		case k.Kind == KName:
			v = Variant{Name: k.Name}
		case k.Kind == KApp && len(k.Kids) == 2 &&
			k.Kids[0].Kind == KName && k.Kids[1].Kind == KName:
			v = Variant{Name: k.Kids[0].Name, Payload: k.Kids[1].Name}
		default:
			return nil, fmt.Errorf("sum %s: a variant is a name, or a name and one "+
				"payload type — `(ok int)`; got %s", sum.Name, k)
		}
		if seen[v.Name] {
			return nil, fmt.Errorf("sum %s: %s is declared twice", sum.Name, v.Name)
		}
		if v.Name == sum.Name {
			return nil, fmt.Errorf("sum %s: a variant may not have the sum's own name, "+
				"because the constructor and the type would be one name", sum.Name)
		}
		seen[v.Name] = true
		sum.Variants = append(sum.Variants, v)
	}
	if len(sum.Variants) < 2 {
		return nil, fmt.Errorf("sum %s: a sum has two or more variants; one variant is "+
			"just the payload", sum.Name)
	}
	return sum, nil
}

// Defs are the definitions a sum declaration generates. There is no new term
// kind and no new reduction rule: a constructor is an ordinary definition, so
// module qualification, imports, δ and the occurrence counter all apply to it
// without knowing sums exist.
//
//	(sum result (ok int) (err int))
//
//	ok      = (fn (x) (values 0 x))       ⟶  (fn (x) (fn (#k) (#k 0 x)))
//	ok#tag  = 0
//	err     = (fn (x) (values 1 x))
//	err#tag = 1
//
// `#tag` is what lets `case` desugar in the READER, which cannot see a sum
// declared in another module: the clause emits a NAME, δ resolves it wherever
// the sum lives, and reduction folds it to the literal. So an imported error
// type works with no cross-module machinery at all.
//
// A payload-less variant is the constant `(values i 0)` rather than a bare `i`,
// so that every variant of a sum has ONE shape and `case` need not ask which.
func (s *Sum) Defs() ([]string, map[string]*Term) {
	order := make([]string, 0, 2*len(s.Variants))
	defs := map[string]*Term{}
	for i, v := range s.Variants {
		tag := &Term{Kind: KInt, Int: int64(i)}
		if v.Payload == "" {
			defs[v.Name] = Fn([]string{"#x"},
				&Term{Kind: KApp, Kids: []*Term{Name("#x"), tag, &Term{Kind: KInt}}})
		} else {
			defs[v.Name] = Fn([]string{"#p"}, Fn([]string{"#x"},
				&Term{Kind: KApp, Kids: []*Term{Name("#x"), tag, Name("#p")}}))
		}
		defs[v.Name+"#tag"] = tag
		order = append(order, v.Name, v.Name+"#tag")
	}
	return order, defs
}

// TypeName is the canonical spelling of a type as it is written in a `sig`.
//
// A bare name is itself. `(array f64)` becomes "array f64" — one string, because
// `SigParam.Type` is a string and the type language is small enough that it
// does not need a tree. It returns "" for anything that is not a type.
//
// tables.md §5: `(array V)` exists only in the SIGNATURE language and is erased
// by staging. A dynamic index forces homogeneity and reduction removes every
// static one, so the checker only ever sees `Fin n → V` and no dependent type is
// needed.
func TypeName(t *Term) string {
	if t == nil {
		return ""
	}
	if t.Kind == KName {
		return t.Name
	}
	if t.Kind == KApp && len(t.Kids) == 2 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "array" {
		if elem := TypeName(t.Kids[1]); elem != "" {
			return "array " + elem
		}
	}
	return ""
}

// ArrayElem returns the element type of an `(array V)` type, or "".
func ArrayElem(ty string) string {
	if strings.HasPrefix(ty, "array ") {
		return ty[len("array "):]
	}
	return ""
}
