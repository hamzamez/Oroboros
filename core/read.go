package core

import (
	"fmt"
	"math/big"
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
	// Ensures is a POSTCONDITION: a boolean term over the parameter names and
	// `result`. On an exported definition it is an obligation checked against
	// the body; on an internal one it is redundant, because reduction inlines
	// the call and the analysis sees the body at the site with the caller's own
	// values (postconditions.md §3).
	Ensures *Term
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
	Params   []string // type parameters: the declaration is a constructor Typeⁿ → Type
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

// str reads a double-quoted literal — string-literals.md.
//
// A literal denotes an element of Scalar*, and the notation mirrors the object:
// a scalar denotes itself, and its value is the concatenation. Six escapes, and
// every one of them is FORCED rather than borrowed from a host:
//
//	\"        the delimiter cannot delimit and denote at once
//	\        the escape introducer, likewise
//	\t \n \r  the source grammar's own whitespace. A token that could contain a
//	          raw line terminator makes an unterminated literal indistinguishable
//	          from a long one, so the error would be reported at end of file
//	          rather than where it happened.
//	\u{H…}    totality: most scalars cannot be typed and many are invisible
//
// AN UNKNOWN ESCAPE IS AN ERROR, which follows from unambiguity: if `\a` meant
// `a`, two different literals would denote one string. That is the rule
// JavaScript does not have — `"\a"` is the letter there — and it is the first of
// three divergences string-literals.md §6.2 measured.
//
// `\xHH` and octal are absent because they denote a BYTE. A byte is not a scalar
// value, so a notation that can build `\xff` is a notation for a different
// object — and Go's `strconv.Unquote`, which this replaces, could build one.
func (r *reader) str() (*Term, error) {
	line := r.line
	r.next() // opening quote
	var b strings.Builder
	for {
		if r.done() {
			return nil, fmt.Errorf("line %d: unterminated string literal", line)
		}
		c := r.next()
		if c == '"' {
			return Str(b.String()), nil
		}
		if c == '\t' || c == '\n' || c == '\r' {
			return nil, fmt.Errorf("line %d: a string literal may not contain a raw "+
				"tab, newline or carriage return; write \t, \n or \r", line)
		}
		if c != backslash {
			b.WriteRune(c)
			continue
		}
		if r.done() {
			return nil, fmt.Errorf("line %d: unterminated escape in string literal", line)
		}
		switch e := r.next(); e {
		case '"':
			b.WriteByte('"')
		case backslash:
			b.WriteRune(backslash)
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			v, err := r.scalarEscape(line)
			if err != nil {
				return nil, err
			}
			b.WriteRune(v)
		default:
			return nil, fmt.Errorf("line %d: %c%c is not an escape. The escapes are "+
				"%c\" %c%c %ct %cn %cr and %cu{H…} for any scalar value "+
				"(docs/spec/string-literals.md §3)",
				line, backslash, e, backslash, backslash, backslash,
				backslash, backslash, backslash, backslash)
		}
	}
}

// scalarEscape reads the body of `\u{H…}`, having consumed the `u`.
//
// One to six hexadecimal digits, and the value must be a SCALAR VALUE: Unicode
// excludes the surrogate range D800–DFFF, because a surrogate is an artefact of
// the UTF-16 encoding form rather than a character. That exclusion is what makes
// a string's value encoding-independent — every scalar sequence has a unique
// representation in every encoding form, and a sequence containing a lone
// surrogate has none in UTF-8 (string-literals.md §1).
func (r *reader) scalarEscape(line int) (rune, error) {
	if r.done() || r.next() != '{' {
		return 0, fmt.Errorf("line %d: %cu must be written %cu{H…} with one to six "+
			"hexadecimal digits", line, backslash, backslash)
	}
	var digits []rune
	for {
		if r.done() {
			return 0, fmt.Errorf("line %d: unterminated %cu{…} escape", line, backslash)
		}
		c := r.next()
		if c == '}' {
			break
		}
		digits = append(digits, c)
		if len(digits) > 6 {
			return 0, fmt.Errorf("line %d: %cu{…} takes at most six hexadecimal "+
				"digits; the largest scalar value is 10FFFF", line, backslash)
		}
	}
	if len(digits) == 0 {
		return 0, fmt.Errorf("line %d: %cu{} has no digits", line, backslash)
	}
	v, err := strconv.ParseUint(string(digits), 16, 32)
	if err != nil {
		return 0, fmt.Errorf("line %d: %cu{%s} is not hexadecimal", line, backslash,
			string(digits))
	}
	if v > 0x10FFFF {
		return 0, fmt.Errorf("line %d: %cu{%s} is above the largest scalar value, 10FFFF",
			line, backslash, string(digits))
	}
	if v >= 0xD800 && v <= 0xDFFF {
		return 0, fmt.Errorf("line %d: %cu{%s} is a SURROGATE, which is not a scalar "+
			"value — it exists only inside the UTF-16 encoding form, and a sequence "+
			"containing one has no representation in UTF-8 at all "+
			"(docs/spec/string-literals.md §1)", line, backslash, string(digits))
	}
	return rune(v), nil
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
		// be (build.md §2), and `(sig null () ptr …)` declares one. Nothing else
		// may be empty.
		if (len(kids) == 1 && isFnHead(kids[0]) || len(kids) == 2 && kids[0].Kind == KName &&
			(kids[0].Name == "sig" || kids[0].Name == "def")) && r.peek() == '(' {
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
	// A source-level `let` is SUGAR for an application, and desugars here
	// (spec/binding.md).
	//
	//   (let x e b)  ⟶  ((fn (x) b) e)
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
	// unambiguous despite sharing a name — and since the source spelling became
	// the flat one, they do not even look alike.
	if kids[0].Kind == KName && kids[0].Name == "let" {
		return readLet(kids, line)
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
					t = conj(kids[i], t)
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
			return nil, fmt.Errorf("line %d: `values` is spelled `tuple` (spec/data.md §3)", line)
		case "tuple":
			// (tuple a b …)  ⟶  (fn (k) (k a b …))
			//
			// A TUPLE IS A FUNCTION ON Fin n (spec/data.md §1), and this is its
			// Church presentation: the currying eliminator is β itself. It is the
			// term `values` read as, so every backend's multiple return and the
			// product pass see exactly the shape they always have.
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
				return nil, fmt.Errorf("line %d: tuple takes two or more terms; "+
					"a tuple of one is just the value", line)
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

func paramList(t *Term, line int) ([]string, error) { return paramListOf("fn", t, line) }

// paramListOf reads a parameter list for `fn` or for `def`'s shorthand. One
// implementation, because a parameter list means the same thing in both: two
// would be two rules that can disagree.
func paramListOf(what string, t *Term, line int) ([]string, error) {
	if t.Kind == KName {
		return nil, fmt.Errorf("%s%s parameters must be a list, got %s", at(line), what, t.Name)
	}
	if t.Kind != KApp {
		return nil, fmt.Errorf("%s%s parameters must be a list", at(line), what)
	}
	params := make([]string, 0, len(t.Kids))
	seen := make(map[string]bool, len(t.Kids))
	for _, k := range t.Kids {
		if k.Kind != KName {
			return nil, fmt.Errorf("%s%s parameter must be a name, got %s", at(line), what, k)
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
			return nil, fmt.Errorf("%s%s cannot be a parameter; a binder is a simple "+
				"name, and `.` qualifies a module member", at(line), k.Name)
		}
		if seen[k.Name] {
			return nil, fmt.Errorf("%s%s binds %s twice; a parameter list may not repeat "+
				"a name, because the second would silently shadow the first", at(line), what, k.Name)
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

// OnlyFragments reports whether a file says nothing but `(provides …)` — a
// TARGET FRAGMENT rather than a program. It is what lets a command refuse such a
// file by what it IS, instead of reporting the first name inside it as unbound.
func OnlyFragments(forms []Form) bool {
	n := 0
	for _, f := range forms {
		if f.Kind != "provides" {
			return false
		}
		n++
	}
	return n > 0
}

// nameOr is the name at position i, or "" — for a form whose meaning belongs to
// another reader and whose shape is therefore not checked here.
func nameOr(kids []*Term, i int) string {
	if i < len(kids) && kids[i].Kind == KName {
		return kids[i].Name
	}
	return ""
}

// ToForm reads one already-parsed term as a declaration form. The target loader
// asks, for the `(def …)` and `(use …)` of `D_T`: a target file is
// s-expressions rather than a program, so it reads terms and hands back the two
// forms that are a program's.
func ToForm(t *Term) (Form, error) { return toForm(t) }

func toForm(t *Term) (Form, error) {
	if t.Kind != KApp || t.Kids[0].Kind != KName {
		return Form{Kind: "term", Term: t}, nil
	}
	switch t.Kids[0].Name {
	case "def":
		// THE EQUATIONAL SHORTHAND (docs/program-surface.md). `(def f (x…) body)`
		// reads as `(def f (fn (x…) body))`, which is what mathematics means by
		// `f(x) = e` and what Scheme's `(define (f x) …)` has always been: SUGAR,
		// erased here, with a unique expansion. def.md §4 recorded the position
		// before there was a form.
		//
		// THE DISCRIMINATOR, fixed here so a later form cannot collide with it: a
		// PARAMETER LIST is a list of NAMES ONLY. Anything else in that position
		// is not one — in particular a list whose first element is itself a list,
		// which is the shape several arities under one name would take
		// (program-surface.md §8.3). This is the rule type arguments and constant
		// endpoints already use: admit a shape only where it cannot mean the
		// other thing.
		if len(t.Kids) >= 4 && t.Kids[1].Kind == KName && allNames(t.Kids[2]) {
			if len(t.Kids) > 4 {
				return Form{}, fmt.Errorf("def %s takes ONE body; `(seq a b)` is how two "+
					"expressions are sequenced, and it is written because a pure first one "+
					"would otherwise be deleted in silence: %s", t.Kids[1].Name, t)
			}
			ps, err := paramListOf("def", t.Kids[2], 0)
			if err != nil {
				return Form{}, err
			}
			t = &Term{Kind: KApp, Kids: []*Term{t.Kids[0], t.Kids[1], Fn(ps, t.Kids[3])}}
		}
		if len(t.Kids) != 3 || t.Kids[1].Kind != KName {
			if len(t.Kids) >= 4 && t.Kids[1].Kind == KName && t.Kids[2].Kind == KApp &&
				len(t.Kids[2].Kids) > 0 && t.Kids[2].Kids[0].Kind == KApp {
				return Form{}, fmt.Errorf("def %s: several arities under one name are not built "+
					"(docs/program-surface.md §8.3); a parameter list is a list of names: %s",
					t.Kids[1].Name, t)
			}
			return Form{}, fmt.Errorf("def takes a name and one term, or a name, a parameter "+
				"list and one body: %s", t)
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
		case r.Kind != KName && TypeName(r) != "": // a tuple type reads as a function term
			// A COMPOUND TYPE, not a result list. `(array f64)` is one result
			// whose type happens to be written as a list, and reading it as two
			// made a table-returning function look like a product — which is
			// what `(sig squares ((n int)) (array int))` did until this case
			// existed.
			//
			// SEVERAL RESULTS ARE A TUPLE (spec/data.md §3.3): a function whose
			// result is `(tuple A B)` returns two, which is values.md's negative
			// product at the one position a product becomes the host's own form.
			if ty := TypeName(r); IsProd(ty) {
				sig.Results = ProdTypes(ty)
			} else {
				sig.Result = ty
			}
		case r.Kind == KApp && TypeTerm(r) != "": // a variant type applied to its arguments
			sig.Result = TypeTerm(r)
		case r.Kind == KFn: // a tuple type TypeName refused, such as one nested in another
			return Form{}, fmt.Errorf("sig %s: %s is not a type", t.Kids[1].Name, r)
		case r.Kind == KApp && len(r.Kids) > 0 && r.Kids[0].Kind == KName &&
			(r.Kids[0].Name == "tuple" || r.Kids[0].Name == "array" || r.Kids[0].Name == "buffer" ||
				r.Kids[0].Name == "map" || r.Kids[0].Name == "int"):
			return Form{}, fmt.Errorf("sig %s: %s is not a type", t.Kids[1].Name, r)
		case r.Kind == KApp:
			return Form{}, fmt.Errorf("sig %s: a result list is a tuple type — write (tuple A B …) "+
				"(spec/data.md §10), got %s", t.Kids[1].Name, r)
		default:
			return Form{}, fmt.Errorf("sig takes a name, a parameter list and a result type: %s", t)
		}
		for _, rest := range t.Kids[4:] {
			if rest.Kind == KApp && rest.Kids[0].Kind == KName &&
				rest.Kids[0].Name == "where" && len(rest.Kids) == 2 {
				sig.Where = rest.Kids[1]
				continue
			}
			if rest.Kind == KApp && rest.Kids[0].Kind == KName &&
				rest.Kids[0].Name == "ensures" && len(rest.Kids) == 2 {
				sig.Ensures = rest.Kids[1]
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
					a.Kids[0].Kind == KName && a.Kids[1].Kind != KName:
					// (name (array f64)) — named, with a compound type.
					if ty := TypeTerm(a.Kids[1]); ty != "" {
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
		// A SCALAR RANGE IS A PREMISE, and it desugars into the `where` it
		// means. `(n (int LO HI))` and `(n int) (where (and (<= LO n) (<= n HI)))`
		// have the same denotation — γ(int LO HI) = {k | LO ≤ k ≤ HI} is exactly
		// the satisfying set of that conjunct — so making one the sugar of the
		// other is a definition, not an approximation.
		//
		// Doing it HERE is what costs nothing. `where` is already read by the
		// refinement layer, the interval layer and termination; a range that
		// became a fact by its own path would need each of those taught, and
		// this repository has three recorded cases of a helper that existed and
		// was not called at every site. Sugar that erases in the reader is what
		// `let`, `seq`, `and`, `cond`, `match` and `values` all are.
		//
		// The TYPE is deliberately left alone. A range still says which rung of
		// ADR 0003's ladder the value is stored on, which is what representation
		// selection will read (ADR 0019); only the FACT is desugared.
		//
		// A range on an UNNAMED parameter is a type and nothing else, because a
		// refinement attaches to a name (refinements.md) and there is no name to
		// attach to. That is the existing rule, not a new limitation.
		for _, pm := range sig.Params {
			if pm.Name == "" {
				continue
			}
			c := rangePremise(pm.Type, Name(pm.Name))
			if c == nil {
				continue
			}
			if sig.Where == nil {
				sig.Where = c
			} else {
				sig.Where = conj(sig.Where, c)
			}
		}
		// AND THE RESULT'S RANGE IS THE DUAL — a GUARANTEE, desugaring into the
		// `ensures` it means, exactly as a parameter's range desugars into its
		// `where`. postconditions.md's algebra is a swap and this is that swap:
		// `result : (int LO HI)` is `(and (<= LO result) (<= result HI))`.
		//
		// Without it a range in the result position is a declaration NOBODY
		// CHECKS. `(sig sq ((n (int 0 100))) (int 0 5))` is false — the body
		// reaches 10000 — and it was accepted in silence, while the same claim
		// written as an `ensures` was refused with the interval that disproves
		// it. A claim that is quietly ignored is worse than one that is refused,
		// and the machinery that refuses it already existed.
		//
		// It rides on postconditions.md's trichotomy unchanged: assumed on a
		// prim, CHECKED against the body on an exported definition, and gone on
		// an internal one, where reduction removes the boundary.
		//
		// This must run BEFORE the `result`-name check below, or a signature
		// with a ranged result and a parameter called `result` would mean two
		// things and the checker would silently pick one.
		//
		// ABOVE THE WINDOW A RANGE GUARANTEES NOTHING, and that asymmetry with
		// the parameter case is a soundness fact rather than a convenience. A
		// premise is ASSUMED, so a half of it is a weaker assumption and safe; a
		// guarantee is CHECKED AGAINST THE BODY, and the interval domain does not
		// model a bignum — it reports [-inf, +inf] for one by construction. So
		// `(int 0 (pow 2 1000))` on a result would demand `(<= 0 result)` of a
		// value the analysis can never bound, and every arbitrary-precision
		// program would be refused for a claim nothing can discharge.
		//
		// That is the refusal-in-front-of-nothing shape, and this is where it is
		// declined: above the window a range is a REPRESENTATION declaration and
		// not a contract, which is the third of scalarrange-2026-08-31's three
		// effects surviving alone.
		if c := rangePremise(sig.Result, Name(ResultName)); c != nil &&
			ValueType(sig.Result) == "int" {
			if sig.Ensures == nil {
				sig.Ensures = c
			} else {
				sig.Ensures = conj(sig.Ensures, c)
			}
		}
		// `result` NAMES THE RESULT in a postcondition, so a parameter may not
		// take the name — otherwise `(ensures (< i result))` would mean two
		// things and the checker would silently pick one. The same refusal
		// `array` gets, for the same reason.
		if sig.Ensures != nil {
			for _, pm := range sig.Params {
				if pm.Name == ResultName {
					return Form{}, fmt.Errorf(
						"sig %s: a parameter may not be named %q — an `ensures` uses that name "+
							"for the result", t.Kids[1].Name, ResultName)
				}
			}
		}
		// A PRODUCT IS AN ELEMENT TYPE, and a signature is where that is
		// enforced because a signature is where a type crosses a boundary.
		//
		// `(array A B)` has a representation as the ELEMENT of a table — the
		// flat form, which is currying (products.md §6) — and none on its own.
		// A bare product parameter has no width the caller and callee could
		// agree on, and a bare product result duplicates `values`, which is the
		// negative product and is what that position already has.
		//
		// Refused by name rather than left to fail downstream: an unrepresented
		// product reached the emitter as `/*prod(int, int)?*/` on Go and would
		// have reached JavaScript, which types nothing, as whatever it liked.
		for _, pm := range sig.Params {
			if IsProd(pm.Type) {
				return Form{}, fmt.Errorf(
					"sig %s: %s is a product, and a product is an ELEMENT type — "+
						"a TABLE of them is `(array (tuple %s))`. On its own a product "+
						"has no representation; a tuple RESULT is several results at a "+
						"function boundary", t.Kids[1].Name, pm.Type,
					strings.Join(ProdTypes(pm.Type), " "))
			}
		}
		return Form{Kind: "sig", Name: t.Kids[1].Name, Sig: sig}, nil
	case "sum":
		return Form{}, fmt.Errorf("`sum` is spelled `variant` (spec/data.md §5.2): %s", t)
	case "variant":
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
	case "provides":
		// A `(provides TARGET PATH …)` IS A TARGET FRAGMENT, not a program —
		// `(target T (module PATH …))` written where the library lives
		// (target-system.md §8). A library file may hold one beside its
		// definitions, so reading that file as a program must SKIP it rather
		// than take it for a term: read as a term, its inner `(use … as re)`
		// is invisible and every name under it is reported unimported.
		return Form{Kind: "provides", Name: nameOr(t.Kids, 2)}, nil
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
	// A one-name binding has already been desugared to ((fn (x) k) e).
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
	if len(t.Kids) < 3 {
		return nil, fmt.Errorf("variant takes a name and at least two constructors: %s", t)
	}
	sum := &Sum{}
	// The head is NAME, or (NAME T₁ … Tₙ): a type constructor of n parameters
	// (spec/data.md §5.5.4). A parameter has kind `type`, so it is a bare name;
	// an applied parameter would be higher-kinded, which the shape refuses.
	switch h := t.Kids[1]; {
	case h.Kind == KName:
		sum.Name = h.Name
	case h.Kind == KApp && len(h.Kids) >= 2 && h.Kids[0].Kind == KName:
		sum.Name = h.Kids[0].Name
		distinct := map[string]bool{sum.Name: true}
		for _, p := range h.Kids[1:] {
			if p.Kind != KName {
				return nil, fmt.Errorf("variant %s: a type parameter is a name — a parameter that "+
					"is itself applied is higher-kinded, which is refused; got %s", sum.Name, p)
			}
			if distinct[p.Name] {
				return nil, fmt.Errorf("variant %s: type parameter %s is declared twice, or has "+
					"the type's own name", sum.Name, p.Name)
			}
			distinct[p.Name] = true
			sum.Params = append(sum.Params, p.Name)
		}
	default:
		return nil, fmt.Errorf("variant takes a name, or (NAME T …), and at least two "+
			"constructors: %s", t)
	}
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
			return nil, fmt.Errorf("variant %s: a constructor is a name, or a name and one "+
				"payload type — `(ok int)`; got %s", sum.Name, k)
		}
		if seen[v.Name] {
			return nil, fmt.Errorf("variant %s: %s is declared twice", sum.Name, v.Name)
		}
		if v.Name == sum.Name {
			return nil, fmt.Errorf("variant %s: a constructor may not have the type's own name, "+
				"because the constructor and the type would be one name", sum.Name)
		}
		seen[v.Name] = true
		sum.Variants = append(sum.Variants, v)
	}
	if len(sum.Variants) < 2 {
		return nil, fmt.Errorf("variant %s: a variant type has two or more constructors; one "+
			"is just the payload", sum.Name)
	}
	used := map[string]bool{}
	for _, v := range sum.Variants {
		// A DECLARATION MAY NOT MENTION ITSELF: that is μ, which the well-founded
		// order of declarations refuses (theories.md §1.3, type-algebra.md §3.1).
		if v.Payload == sum.Name {
			return nil, fmt.Errorf("variant %s: constructor %s carries a %s, so the type "+
				"contains itself. That is a recursive type, which is refused; recursive data is a "+
				"flat table plus indices (type-algebra.md §3.1)", sum.Name, v.Name, sum.Name)
		}
		used[v.Payload] = true
	}
	for _, p := range sum.Params {
		// NO PHANTOMS: an instance is identified by its arguments, and a parameter
		// that occurs in no payload would make two instances with equal values
		// different types for no reason a program has shown.
		if !used[p] {
			return nil, fmt.Errorf("variant %s: type parameter %s occurs in no constructor's "+
				"payload, and a phantom parameter is refused until a program needs one", sum.Name, p)
		}
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
// conj is `(and a b)` ALREADY ERASED — `(if a b false)`.
//
// The connectives do not survive reading (booleans.md, ADR 0017): `and`, `or`,
// `not` and `cond` are sugar and nothing downstream has ever seen one. A range's
// premise and its guarantee are synthesised AFTER that erasure has run, so
// building them with `(and …)` would put a term in a signature that the
// refinement layer cannot read — it reported "outside the decidable fragment"
// for a conjunction it decides perfectly well when the reader writes it.
//
// So there is one spelling of a conjunction and it lives here.
func conj(a, b *Term) *Term {
	return &Term{Kind: KApp, Kids: []*Term{Name("if"), a, b, Bool(false)}}
}

// litChunk is the base a big literal is split into: 10^15, the largest power of
// ten under ADR 0012's window, so every chunk and every digit group a person
// reads back is a plain decimal number.
const litChunk = 1000000000000000

// bigLiteral reads an integer too large for an `int64` and returns it as a TERM
// rather than a value.
//
// A LITERAL PAST THE WORD IS THE SAME PROBLEM A RANGE ENDPOINT HAD, and it has
// the same answer. `KInt` holds an `int64`, so the obvious route was an eighth
// term kind — against state.md's "seven term kinds, the entire grammar of what
// a program can say". unbounded-rung.md §3a dissolved that for endpoints by
// making them EXPRESSIONS; this is the same move one level down, for values.
//
// The literal becomes Horner over base 10^15:
//
//	123456789012345678901  ->  (+ (* 123456 1000000000000000) 789012345678901)
//
// Every leaf is inside the portable window and every operator is the language's
// own, so nothing new enters the term language. What makes it CORRECT is what
// the rest of the integer work built:
//
//   - constant folding refuses a result outside the window (ADR 0009), so the
//     spine survives reduction instead of silently wrapping;
//   - the representation solver promotes it, because the spine's intermediate
//     values are not provably inside the window — so a literal used where a
//     bignum is wanted computes in arbitrary precision, and one used where an
//     `int` is required is REFUSED by bounded-by-default, which is the right
//     answer for a value that does not fit a machine word.
//
// The cost is a few operations for a constant, which an emission-time fold
// could remove later. The benefit is that a 400-digit literal needs no new term
// kind, no widening of `KInt`, and no reader that knows what a bignum is.
func bigLiteral(text string) *Term {
	v, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil
	}
	neg := v.Sign() < 0
	if neg {
		v = new(big.Int).Neg(v)
	}
	base := big.NewInt(litChunk)
	var chunks []int64
	for v.Sign() > 0 {
		q, r := new(big.Int).QuoRem(v, base, new(big.Int))
		chunks = append(chunks, r.Int64())
		v = q
	}
	if len(chunks) == 0 {
		chunks = []int64{0}
	}
	// Horner from the most significant chunk down, so each step is one multiply
	// and one add and the tree is a spine rather than a sum of powers — a sum
	// would need 10^30 as a literal, which is the thing being avoided.
	out := Int(chunks[len(chunks)-1])
	for i := len(chunks) - 2; i >= 0; i-- {
		out = App(Name("*"), out, Int(litChunk))
		if chunks[i] != 0 {
			out = App(Name("+"), out, Int(chunks[i]))
		}
	}
	if neg {
		out = App(Name("-"), Int(0), out)
	}
	return out
}

// evalEndpoint evaluates a range endpoint at compile time, at ARBITRARY
// PRECISION.
//
// This is the only place in the compiler that does big-integer arithmetic, and
// that is the point: the endpoint describes a set and is consumed as a width,
// so nothing downstream needs to carry a big value. `KInt` stays an `int64` and
// the value language does not move.
//
// The grammar is deliberately tiny — literals, negation, `+`, `-`, `*`, `pow` —
// because an endpoint is written by a person to say how big something gets, not
// computed.
//
// DIVISION IS ABSENT, and the honest reason is weaker than "it has a
// precondition". An endpoint has no variables, so every divisor here is a
// literal and division by zero is decidable at read time — that case could
// simply be refused. What is left is ROUNDING: `(int 0 (/ 100 3))` denotes
// [0, 33] and integer division reaches it silently, so an author who meant
// 100/3 has named a different set than they wrote. Nothing is unsound about
// admitting it; the rounded value can always be written directly, so the
// grammar declines to make the choice.
func evalEndpoint(t *Term) (*big.Int, bool) {
	v, inf, ok := endpoint(t)
	return v, ok && inf == 0
}

// endpoint evaluates a range endpoint, which may be INFINITE.
//
// The third result is -1, 0 or +1 for −inf, a finite value, and +inf, and an
// infinite endpoint has no value. It is the top of the representation lattice
// (unbounded-rung.md §1): `(int 0 +inf)` denotes ℕ, `(int -inf +inf)` denotes ℤ,
// and neither is a set any machine word or any FIXED number of limbs contains.
//
// `+inf` and `-inf` are the vocabulary the compiler already prints — every
// interval report says `idx -inf..+inf` — so the declaration and the diagnostic
// finally use the same word for the same thing. They are ordinary names to the
// reader, because `+` and `-` are name characters here.
//
// Arithmetic on an infinity is REFUSED rather than defined. `(pow 2 +inf)` and
// `(+ +inf 1)` are not endpoints anyone means to write, and the extended reals'
// answers to them (`+inf`, `+inf`, and `+inf − +inf` undefined) are a semantics
// this language has no use for. An infinity may only stand alone.
func endpoint(t *Term) (*big.Int, int, bool) {
	if t == nil {
		return nil, 0, false
	}
	if t.Kind == KInt {
		return big.NewInt(t.Int), 0, true
	}
	if t.Kind == KName {
		switch t.Name {
		case "+inf":
			return nil, 1, true
		case "-inf":
			return nil, -1, true
		}
		return nil, 0, false
	}
	if t.Kind != KApp || len(t.Kids) < 2 || t.Kids[0].Kind != KName {
		return nil, 0, false
	}
	op, args := t.Kids[0].Name, t.Kids[1:]
	if op == "-" && len(args) == 1 {
		v, ok := evalEndpoint(args[0])
		if !ok {
			return nil, 0, false
		}
		return new(big.Int).Neg(v), 0, true
	}
	if len(args) != 2 {
		return nil, 0, false
	}
	a, ok1 := evalEndpoint(args[0])
	b, ok2 := evalEndpoint(args[1])
	if !ok1 || !ok2 {
		return nil, 0, false
	}
	switch op {
	case "+":
		return new(big.Int).Add(a, b), 0, true
	case "-":
		return new(big.Int).Sub(a, b), 0, true
	case "*":
		return new(big.Int).Mul(a, b), 0, true
	case "pow":
		// A NEGATIVE OR ABSURD EXPONENT IS REFUSED rather than saturated. The
		// cap is generous — 2^1000000 is far past any representation anyone
		// will ask for — and it exists so a typo cannot ask the compiler to
		// build a gigabyte-long integer.
		if b.Sign() < 0 || !b.IsInt64() || b.Int64() > 1000000 {
			return nil, 0, false
		}
		return new(big.Int).Exp(a, b, nil), 0, true
	}
	return nil, 0, false
}

func TypeName(t *Term) string {
	if t == nil {
		return ""
	}
	// THE READER IS CONTEXT-FREE, so `(tuple A B)` written as a TYPE arrives as
	// the term it reads as everywhere, `(fn (#k) (#k A B))`. A type is built from
	// a term here, so this is where that reading is inverted — the one place both
	// meet — and `#k` cannot be written in source, so nothing else has this shape.
	if t.Kind == KFn && len(t.Params) == 1 && t.Params[0] == "#k" {
		if b := t.Body(); b.Kind == KApp && len(b.Kids) >= 3 &&
			b.Kids[0].Kind == KName && b.Kids[0].Name == "#k" {
			return TypeName(&Term{Kind: KApp, Kids: append([]*Term{Name("tuple")}, b.Kids[1:]...)})
		}
	}
	if t.Kind == KName {
		return t.Name
	}
	if t.Kind == KApp && len(t.Kids) == 2 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "array" {
		// A BUFFER MAY NOT BE AN ELEMENT — ADR 0020 rule 6, and it is enforced
		// here because this is the one place a compound type is built.
		if elem := TypeName(t.Kids[1]); elem != "" && !IsBuffer(elem) && !prodHoldsBuffer(elem) {
			return "array " + elem
		}
	}
	// `(array T1 T2 … Tn)`, n >= 2 — THE PRODUCT, and the spelling is the term's.
	//
	// products.md: a tuple is a function whose domain is a finite set, and
	// tables.md §5.3 had already written the consequence — *"a statically-indexed
	// heterogeneous `(array x y)` is a pair, and `(a 0)`/`(a 1)` are its
	// projections; the tuple is not a separate feature."* That sentence is the
	// type former, so the type former is spelled the way the term already is.
	//
	// ARITY DISAMBIGUATES AND IT IS NOT A TRICK. `(array V)` names one index set
	// — `Fin n` for an n nobody has stated — so it is homogeneous and its length
	// is dynamic. `(array A B)` names `Fin 2` exactly, so the family may vary.
	// One argument or many is precisely the static/dynamic distinction that
	// tables.md §5.3 says forces homogeneity, written in the type.
	//
	// The canonical form is DELIMITED where every other one is space-separated,
	// because a component may itself contain spaces — `int 0 255` does — and
	// `MapTypes` splits on the first space only because a map's key is always one
	// token. A comma cannot occur inside a type, so `prod(A, B)` parses back.
	//
	// RESPELLED `(tuple T1 … Tn)` (spec/data.md §3, §6): `(array int)` could not
	// tell a table of ints from a product of one, and `(array A B)` in a result
	// was read as two results, so arity was carrying a meaning a NAME should. A
	// buffer may be a component — several results may hand one back — and may
	// not be inside an array's element, which the `array` case above refuses.
	if t.Kind == KApp && len(t.Kids) >= 3 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "tuple" {
		parts := make([]string, 0, len(t.Kids)-1)
		for _, k := range t.Kids[1:] {
			e := TypeName(k)
			// Rule 6 again, and one of its own: a buffer may not be a field,
			// for the reason it may not be an element — an observation must not
			// be able to extract an alias (ADR 0020). And a product may not
			// contain a product yet: flattening is what gives it a
			// representation, and a nested one would need the layout machinery
			// products.md §8 puts last.
			if e == "" || IsProd(e) {
				return ""
			}
			parts = append(parts, e)
		}
		return "prod(" + strings.Join(parts, ", ") + ")"
	}
	// `(buffer V)` — ADR 0020. A buffer is a NAMEABLE TYPE, so a function may
	// take its workspace instead of building one every call.
	//
	// IT IS A TYPE CONSTRUCTOR AND NOT AN ATTRIBUTE, which is the whole reason
	// this is affordable. Clean puts uniqueness on every type — `*[*Int]` — and
	// pays for it with attribute variables, an inequality lattice and inferred
	// coercions; ADR 0018 had already made the distinction one between two
	// CONSTRUCTORS, `(array V)` shared and immutable against a buffer linear and
	// scoped, so there is nothing to attach an attribute to. The one coercion
	// Clean infers we already write, as the freeze at `build`'s boundary.
	//
	// A BUFFER MAY NOT BE AN ELEMENT TYPE, and that is load-bearing rather than
	// tidy: it is what keeps the read-borrow free. `(b i)` yields a scalar or a
	// frozen array, so an observation CANNOT alias the buffer — which is why
	// reads-do-not-consume needs none of Wadler's `let!` or Odersky's observer
	// machinery. Refused here, at the one place a type is built.
	if t.Kind == KApp && len(t.Kids) == 2 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "buffer" {
		if elem := TypeName(t.Kids[1]); elem != "" && !strings.HasPrefix(elem, "buffer ") {
			return "buffer " + elem
		}
	}
	// `(int LO HI)` — a RANGE is a type, which is ADR 0003's "mathematical
	// semantics, machine representation" written in the type language. The
	// range says what the value IS; the target says how wide it is stored.
	//
	// Canonicalised to "int LO HI" for the same reason `(array V)` becomes
	// "array V": types are strings here, and a compound one has to print.
	// `(map K V)` — a table whose index set is a finite subset of K
	// (maps.md §2). Canonicalised the same way `(array V)` is.
	//
	// K must be a type on which the language's `=` is defined, and today that
	// is exactly `int`. That is not a staging convenience: a map's domain
	// condition is `k ∈ dom m`, decided by equality on K, so `(map K V)` is
	// well-formed exactly where `=` is. The refusal is in MapType's caller so
	// that it can name the reason.
	if t.Kind == KApp && len(t.Kids) == 3 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "map" {
		k, v := TypeName(t.Kids[1]), TypeName(t.Kids[2])
		// Rule 6 again: a buffer is not a key and not a value. And a PRODUCT is
		// not a value either — not because the algebra refuses it, but because
		// this build gives a product a representation by FLATTENING an array of
		// them, and a map is not an array. Accepting it emitted
		// `map[int]/*prod(int, int)?*/`, a Go type that does not exist and that
		// a host with no types would have taken silently.
		if k != "" && v != "" && !IsBuffer(k) && !IsBuffer(v) && !IsProd(k) && !IsProd(v) {
			return "map " + k + " " + v
		}
		return ""
	}
	// `(int LO HI)` where each endpoint is a compile-time EXPRESSION, not only a
	// literal: `(int 0 (pow 2 70))`, `(int 0 (* 1000 1000))`.
	//
	// AN ENDPOINT IS A BOUND, NOT A VALUE, and that one distinction is what
	// makes this cheap. ADR 0012 constrains the integers a program COMPUTES
	// WITH; an endpoint describes a set. So the expression is evaluated here, at
	// arbitrary precision, and never becomes a term — which means no new term
	// kind, no big literal in the value language, and no widening of `KInt`.
	//
	// It also removes the need to write a big literal at all: the reader refuses
	// one, and `(pow 2 70)` says the same thing more legibly than seventy digits
	// would. `pow` and not `^`, because `^` is XOR on Go, JavaScript and Java
	// and a name should say what an operation IS (match.md's reason for `=`).
	if t.Kind == KApp && len(t.Kids) == 3 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "int" {
		lo, loInf, ok1 := endpoint(t.Kids[1])
		hi, hiInf, ok2 := endpoint(t.Kids[2])
		if !ok1 || !ok2 {
			return ""
		}
		// AN INFINITE ENDPOINT MUST POINT OUTWARD. `(int +inf 0)` and
		// `(int 0 -inf)` denote the empty set, and `(int +inf +inf)` denotes a
		// point that is not an integer — none is a type, and refusing them here
		// is the same refusal `lo > hi` already gets.
		if loInf > 0 || hiInf < 0 {
			return ""
		}
		if loInf == 0 && hiInf == 0 && lo.Cmp(hi) > 0 {
			return "" // an empty range is not a type — see below
		}
		return "int " + endpointStr(lo, loInf) + " " + endpointStr(hi, hiInf)
	}
	return ""
}

// TypeTerm is TypeName extended by `(F A₁ … Aₙ)` — A TYPE CONSTRUCTOR APPLIED TO
// ITS ARGUMENTS (spec/data.md §5.5.1). A parameterised variant declaration is
// F : Typeⁿ → Type, and an instance is identified APPLICATIVELY, by the
// declaration and its arguments, so its canonical spelling is `F(A₁, …, Aₙ)`,
// delimited for products.md §2's reason: an argument may contain spaces.
//
// IT IS A SEPARATE FUNCTION BECAUSE THE GRAMMAR IS AMBIGUOUS WITHOUT POSITION.
// A parameter is `TYPE | (NAME TYPE)`, and `(i int)` is both a named parameter
// and `i` applied to `int`. So the applied production is admitted only where a
// named parameter cannot stand — a result, the type of a named parameter, and a
// type argument — and TypeName, which every parameter list reads, keeps exactly
// the language it had. The reader is context-free and cannot know whether F is
// a variant type; Load resolves F and refuses it if it is not one.
func TypeTerm(t *Term) string {
	if ty := TypeName(t); ty != "" {
		return ty
	}
	if t == nil || t.Kind != KApp || len(t.Kids) < 2 || t.Kids[0].Kind != KName || typeFormers[t.Kids[0].Name] {
		return ""
	}
	args := make([]string, 0, len(t.Kids)-1)
	for _, k := range t.Kids[1:] {
		a := TypeTerm(k)
		// ADR 0020 rule 6: an argument lands in a payload, which `case` binds,
		// and binding is an observation.
		if a == "" || IsBuffer(a) {
			return ""
		}
		args = append(args, a)
	}
	return t.Kids[0].Name + "(" + strings.Join(args, ", ") + ")"
}

// typeFormers are the heads the type language owns; any other head applied to
// arguments is a declared type constructor.
var typeFormers = map[string]bool{
	"array": true, "tuple": true, "buffer": true, "map": true, "int": true,
	"fn": true, "record": true, "prod": true,
}

// IsTypeFormer reports whether a name is one of them. The emitter asks when it
// has to tell a NAMED PARAMETER from a compound type — `(i int)` from
// `(array int)` — which is the one place the declaration grammar is ambiguous
// without position (spec/data.md §5.5.1).
func IsTypeFormer(n string) bool { return typeFormers[n] }

// Applied splits a canonical applied type `F(A, B)` into its constructor and
// arguments. Arguments are split at parenthesis depth zero, because an
// argument may itself be applied: `result(option(int), int)`.
func Applied(ty string) (string, []string, bool) {
	open := strings.IndexByte(ty, '(')
	if open <= 0 || !strings.HasSuffix(ty, ")") || IsProd(ty) || strings.ContainsRune(ty[:open], ' ') {
		return "", nil, false
	}
	var args []string
	depth, start := 0, open+1
	inner := ty[:len(ty)-1]
	for i := open + 1; i < len(inner); i++ {
		switch inner[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				args = append(args, strings.TrimSpace(inner[start:i]))
				start = i + 1
			}
		}
	}
	args = append(args, strings.TrimSpace(inner[start:]))
	return ty[:open], args, true
}

// ResultName is what a postcondition calls the value a call produces. It is a
// reserved name inside an `ensures` and nowhere else; the language has no
// `result` keyword and a program may still use the name for anything that is
// not a parameter of a function carrying one.
const ResultName = "result"

// prodHoldsBuffer is ADR 0020's rule 6 for a product element: a buffer may not
// be a field of a table's element any more than the element itself.
func prodHoldsBuffer(ty string) bool {
	for _, p := range ProdTypes(ty) {
		if IsBuffer(p) {
			return true
		}
	}
	return false
}

// ExceedsWindow reports whether a range type names a set wider than ADR 0012's
// portable window — the rung above the host's word.
//
// It is the test that separates a REFINEMENT from a WIDENING. Every range
// inside the window satisfies `[LO,HI] ⊆ W`, which is why `ValueType`
// normalises one to `int` and an `int` is accepted wherever it is wanted. A
// range outside it does not, so it must be refused there instead — and that
// refusal is the surface: it is where a programmer finds out a value has left
// the machine word.
func ExceedsWindow(ty string) bool {
	lo, hi, ok := IntRangeBig(ty)
	if !ok {
		return false
	}
	w := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 53), big.NewInt(1))
	return lo.CmpAbs(w) > 0 || hi.CmpAbs(w) > 0
}

func endpointStr(v *big.Int, inf int) string {
	switch {
	case inf > 0:
		return "+inf"
	case inf < 0:
		return "-inf"
	}
	return v.String()
}

// UnboundedRange reports whether a range type has an INFINITE endpoint — the
// top of the representation lattice, where no fixed number of limbs suffices.
//
// It is the distinction the whole ladder turns on and the one the type language
// could not make until `+inf` existed: `(int 0 (pow 2 1000))` is finite, so it
// has a limb count and a `build` of known length; `(int 0 +inf)` has neither and
// must fall back to whatever arbitrary-precision the target ships.
//
// `IntRangeBig` deliberately does NOT read one — an infinite endpoint is not a
// `*big.Int` — so every existing consumer sees "not a finite range", which is
// exactly what it is.
func UnboundedRange(ty string) bool {
	if !strings.HasPrefix(ty, "int ") {
		return false
	}
	f := strings.Fields(ty)
	if len(f) != 3 {
		return false
	}
	return f[1] == "-inf" || f[2] == "+inf"
}

// RangeBounds reads whichever endpoints of a range are USABLE AS FACTS: finite,
// and small enough to be a term.
//
// It is deliberately weaker than `IntRangeBig`, and the weakness is the point.
// A premise is a conjunction, so dropping one half is sound — it says less —
// while dropping the half that IS expressible says nothing at all, which is
// what happened before this existed:
//
//	(int 0 +inf)          non-negative, and unbounded above
//	(int 0 (pow 2 1000))  non-negative, with an upper bound no term can hold
//
// Both are half-open as far as the linear fragment is concerned, and both were
// contributing NOTHING, because the desugaring demanded two int64 endpoints.
// Non-negativity is exactly what a bignum representation can spend —
// bigarith-2026-08-28 measured sign handling as a real cost — so this is
// unbounded-rung.md §4.2's claim that `+inf` "carries something the
// representation can spend", made true.
func RangeBounds(ty string) (lo, hi int64, haveLo, haveHi bool) {
	if !strings.HasPrefix(ty, "int ") {
		return 0, 0, false, false
	}
	f := strings.Fields(ty)
	if len(f) != 3 {
		return 0, 0, false, false
	}
	if v, ok := new(big.Int).SetString(f[1], 10); ok && v.IsInt64() {
		lo, haveLo = v.Int64(), true
	}
	if v, ok := new(big.Int).SetString(f[2], 10); ok && v.IsInt64() {
		hi, haveHi = v.Int64(), true
	}
	return lo, hi, haveLo, haveHi
}

// rangePremise is what a range says about a name, as a term — the conjunction
// of whichever halves RangeBounds could read. It returns nil when the range
// says nothing expressible, which `(int -inf +inf)` does.
func rangePremise(ty string, n *Term) *Term {
	lo, hi, haveLo, haveHi := RangeBounds(ty)
	var out *Term
	if haveLo {
		out = App(Name("<="), Int(lo), n)
	}
	if haveHi {
		c := App(Name("<="), n, Int(hi))
		if out == nil {
			out = c
		} else {
			out = conj(out, c)
		}
	}
	return out
}

// ShowType renders a type for a human. It exists for exactly one case: a range
// above the portable window can have hundreds of digits in it, and a diagnostic
// that prints all of them is a diagnostic nobody reads.
//
// The abbreviation keeps the SIZE, which is the thing that matters at this rung
// — how many digits, not which — and it keeps small endpoints verbatim, so
// nothing a programmer typed is hidden from them.
func ShowType(ty string) string {
	lo, hi, ok := IntRangeBig(ty)
	if !ok {
		return ty
	}
	return "int " + showEndpoint(lo) + " " + showEndpoint(hi)
}

func showEndpoint(v *big.Int) string {
	s := v.String()
	neg := ""
	if strings.HasPrefix(s, "-") {
		neg, s = "-", s[1:]
	}
	if len(s) <= 20 {
		return neg + s
	}
	return fmt.Sprintf("%s%s.%se%d (%d digits)", neg, s[:1], s[1:4], len(s)-1, len(s))
}

// IntRangeBig reads a `(int LO HI)` type back at full precision.
//
// `IntRange` is this narrowed to `int64` and reports failure when an endpoint
// does not fit — which is what keeps every existing consumer honest: a range it
// cannot represent looks like "not a range" rather than like a smaller one.
func IntRangeBig(ty string) (*big.Int, *big.Int, bool) {
	if !strings.HasPrefix(ty, "int ") {
		return nil, nil, false
	}
	f := strings.Fields(ty)
	if len(f) != 3 {
		return nil, nil, false
	}
	lo, ok1 := new(big.Int).SetString(f[1], 10)
	hi, ok2 := new(big.Int).SetString(f[2], 10)
	if !ok1 || !ok2 || lo.Cmp(hi) > 0 {
		return nil, nil, false
	}
	return lo, hi, true
}

// IntRange reads a `(int LO HI)` type back. A plain `int` is not a range: it is
// the portable window (ADR 0012) and carries no representation claim.
func IntRange(ty string) (int64, int64, bool) {
	if !strings.HasPrefix(ty, "int ") {
		return 0, 0, false
	}
	var lo, hi int64
	if n, err := fmt.Sscanf(ty, "int %d %d", &lo, &hi); n != 2 || err != nil {
		return 0, 0, false
	}
	if lo > hi {
		return 0, 0, false
	}
	return lo, hi, true
}

// ValueType is what a range MEANS, as opposed to how it is stored. A range is an
// integer; only a table's element slot ever consults the width.
//
// Keeping these apart is what stops a narrowed array from narrowing the LOCALS
// that read it — a byte array read into a byte-wide counter would overflow at
// 255 while the language says the value is an integer.
func ValueType(ty string) string {
	if _, _, ok := IntRange(ty); ok {
		return "int"
	}
	// AND A RANGE ABOVE THE WINDOW IS NOT AN `int` AT ALL — it is the rung
	// above the host's word (unbounded-rung.md §3). `IntRange` narrows to
	// `int64` and so cannot read one, which is why the case below is reached at
	// all; without it such a range normalised to ITSELF and every consumer that
	// compares type strings saw a name no target declares.
	//
	// This is the FOURTH effect a range has, after the three
	// scalarrange-2026-08-31 separated: a type, a premise, a representation —
	// and now, above the window, a DIFFERENT type. The promotion is a widening,
	// not a refinement, so `compatible` refuses it against `int` and that
	// refusal is the surface where a programmer finds out a value became a
	// bignum.
	if ExceedsWindow(ty) || UnboundedRange(ty) {
		return BigType
	}
	return ty
}

// BigType is what a range above the portable window means: arbitrary precision.
// It is the top of the representation ladder ADR 0003 opened — narrower than a
// word, a word, and above it this — and a target spells it in its own file.
const BigType = "big"

// MapTypes reads a `(map K V)` type back into its key and value types.
//
// A map type is `map K V` where both halves are themselves canonical type
// strings. V may be compound — `map int (array f64)` is `map int array f64` —
// so the split is on the FIRST space of the remainder, which is unambiguous
// because K may not be compound: K is restricted to types the language's `=`
// decides, and every one of those is a bare name.
func MapTypes(ty string) (string, string, bool) {
	if !strings.HasPrefix(ty, "map ") {
		return "", "", false
	}
	rest := ty[len("map "):]
	i := strings.Index(rest, " ")
	if i <= 0 || i == len(rest)-1 {
		return "", "", false
	}
	return rest[:i], rest[i+1:], true
}

// MapKeyOK reports whether K may be a map's key type.
//
// `=` is the language's equality and it is integer equality only (match.md):
// floats are out because NaN is not an equivalence relation, strings because no
// two of the four targets agree on comparing them. A map's domain condition is
// decided by equality on K, so the set of legal key types is exactly the set
// `=` is defined on — the language's own equality already refuses the key types
// a map cannot have.
func MapKeyOK(k string) bool {
	if _, _, ok := IntRange(k); ok {
		return true
	}
	return k == "int"
}

// ArrayElem returns the element type of an `(array V)` type, or "".
func ArrayElem(ty string) string {
	if strings.HasPrefix(ty, "array ") {
		return ty[len("array "):]
	}
	// A BUFFER IS A TABLE FOR EVERY PURPOSE BUT ALIASING. Its element range, its
	// width, its indexing and its bounds obligations are an array's — what
	// differs is who may hold it, which is ADR 0018's distinction and is not a
	// question about elements. So every consumer that asks "what does this table
	// hold" gets the same answer for both, and none of them learns that buffers
	// exist.
	if strings.HasPrefix(ty, "buffer ") {
		return ty[len("buffer "):]
	}
	return ""
}

// BufferElem is ArrayElem restricted to a buffer, for the one caller that has to
// tell them apart: the linearity check, which applies to a buffer and not to an
// array.
func BufferElem(ty string) string {
	if strings.HasPrefix(ty, "buffer ") {
		return ty[len("buffer "):]
	}
	return ""
}

// IsProd reports whether a type is the n-ary product, and ProdTypes reads its
// components back. The canonical form is `prod(A, B, …)`; a component never
// contains a comma, because a type never does.
func IsProd(ty string) bool {
	return strings.HasPrefix(ty, "prod(") && strings.HasSuffix(ty, ")")
}

// ProdTypes returns the components of a product type, or nil.
func ProdTypes(ty string) []string {
	if !IsProd(ty) {
		return nil
	}
	inner := ty[len("prod(") : len(ty)-1]
	if inner == "" {
		return nil
	}
	parts := strings.Split(inner, ", ")
	for _, p := range parts {
		if p == "" {
			return nil
		}
	}
	return parts
}

// IsBuffer reports whether a declared type is ADR 0020's unique, linear table.
func IsBuffer(ty string) bool { return strings.HasPrefix(ty, "buffer ") }

// allNames reports whether a term is a list of names — a parameter list, and
// nothing else. `(a b)` is one; `((a b) c)`, `(a 1)` and a bare name are not.
func allNames(t *Term) bool {
	if t == nil || t.Kind != KApp {
		return false
	}
	for _, k := range t.Kids {
		if k.Kind != KName {
			return false
		}
	}
	return true
}

// at prefixes a message with a line when there is one. A form read by ToForm
// carries no line, and "line 0" is worse than no line at all.
func at(line int) string {
	if line <= 0 {
		return ""
	}
	return fmt.Sprintf("line %d: ", line)
}
