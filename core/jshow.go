package core

import (
	"math/big"
	"strconv"
	"strings"
)

// J'S RENDERING OF WHAT A MESSAGE QUOTES (spec/j.md, ADR 0068). A message is a
// function of the program, and the program is J, so every fragment of it a
// message quotes is rendered in J: a name by σ_J (ShowName), a set by J's
// formers (ShowSet), and a core term by J's expressions (ShowTerm). The core
// term's own printed form, Term.String, stays the s-expression the IR, the tests
// and ParseTerm read (jmsg-2026-10-20).
//
// ShowTerm inverts D on what D makes (core/desugar.go): a redex is a block, a
// Church tuple a pair, the connectives and comparisons infix, a right-nested
// chain of `if` a guarded `case`, a store `store(b, i, v)`. Its law, on every
// term D makes of the corpus with no unwritable name: ReadTerm(ShowTerm(t)) = t
// (jshow_test.go). A residual's forms D never makes (`let`, `loop`, `build`)
// are rendered as J would write the same meaning, and a mark or an internal
// name as the core spells it.

// ShowName is a name as J writes it: σ_J, a module path quoted
// (“ `go/fmt`.Println “).
func ShowName(n string) string {
	if strings.HasPrefix(n, "#") {
		return n
	}
	return Spell(n)
}

// ShowNames renders each of a list of names.
func ShowNames(ns []string) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = ShowName(n)
	}
	return out
}

// ShowSet is a type, in the compiler's canonical spelling (TypeName), as J
// writes the set: `0..9`, `Array(0..255)`, `Map(Int, V)`, `A × B`, `()`.
func ShowSet(ty string) string {
	ty = strings.TrimSpace(ty)
	switch {
	case ty == "":
		return ty
	case ty == UnitType:
		return "()"
	case strings.HasPrefix(ty, "?"):
		return ty
	case strings.Contains(ty, "|"):
		// A FINITE UNION OF RANGES, a host `where`'s disjunction of constant
		// bounds on one parameter (emit/wherebounds.go).
		parts := strings.Split(ty, "|")
		for i, p := range parts {
			parts[i] = ShowSet(p)
		}
		return strings.Join(parts, " ∪ ")
	case strings.HasPrefix(ty, "int "):
		f := strings.Fields(ty)
		if len(f) == 3 && f[1] == "0" && f[2] == "+inf" {
			return NatSet // ℕ, which J names (ADR 0079 item 6)
		}
		if len(f) == 3 {
			return endpointJ(f[1]) + ".." + endpointJ(f[2])
		}
	case strings.HasPrefix(ty, "array "):
		return "Array(" + ShowSet(ty[len("array "):]) + ")"
	case strings.HasPrefix(ty, "buffer "):
		return "buffer(" + ShowSet(ty[len("buffer "):]) + ")"
	case strings.HasPrefix(ty, "cell "):
		return "cell(" + ShowSet(ty[len("cell "):]) + ")"
	case strings.HasPrefix(ty, "map "):
		rest := ty[len("map "):]
		if i := strings.IndexByte(rest, ' '); i > 0 {
			return "Map(" + ShowSet(rest[:i]) + ", " + ShowSet(rest[i+1:]) + ")"
		}
	case strings.HasPrefix(ty, "prod(") && strings.HasSuffix(ty, ")"):
		parts := splitTop(ty[len("prod(") : len(ty)-1])
		for i, p := range parts {
			parts[i] = ShowSet(p)
			if strings.Contains(parts[i], " × ") {
				parts[i] = "(" + parts[i] + ")"
			}
		}
		return strings.Join(parts, " × ")
	case strings.HasSuffix(ty, ")") && strings.IndexByte(ty, '(') > 0:
		i := strings.IndexByte(ty, '(')
		parts := splitTop(ty[i+1 : len(ty)-1])
		for k, p := range parts {
			parts[k] = ShowSet(p)
		}
		return setHead(ty[:i]) + "(" + strings.Join(parts, ", ") + ")"
	}
	return setHead(ty)
}

// setHead is a set's name: the language's by the convention (ADR 0067 item 3),
// any other by σ_J.
func setHead(n string) string {
	if s, ok := langSets[n]; ok {
		return s
	}
	return ShowName(n)
}

// endpointJ is a range's end: an integer as J writes a bound, and an infinite
// one as nothing, since J omits an absent bound (ADR 0077): `0..`, `..5`.
//
// An end too long to read whole, and no power of two, keeps its size: `1.234e400
// (401 digits)`, as ShowType writes it, a note for the reader and no J.
func endpointJ(s string) string {
	if s == "+inf" || s == "-inf" {
		return ""
	}
	if j := Integer(s); len(j) <= 24 {
		return j
	}
	if v, ok := new(big.Int).SetString(s, 10); ok {
		return showEndpoint(v)
	}
	return s
}

// splitTop splits at the commas outside parentheses.
func splitTop(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if rest := strings.TrimSpace(s[start:]); rest != "" || len(out) > 0 {
		out = append(out, rest)
	}
	return out
}

// ShowIndex is an indexing, `t[i]`, which the core writes as the application
// (t i): a message that knows it is one says so.
func ShowIndex(table, at *Term) string { return showAt(table, pPost) + "[" + ShowTerm(at) + "]" }

// ShowTerm is a core term as J writes it.
func ShowTerm(t *Term) string { return showAt(t, pLowest) }

// ShowTerms renders each term of a list, separated as J separates a list.
func ShowTerms(ts []*Term) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = ShowTerm(t)
	}
	return strings.Join(parts, ", ")
}

// infix are the language's binary operators, by the core's name.
var infix = map[string]struct {
	op   string
	prec int
}{
	"+": {"+", pAdd}, "-": {"-", pAdd}, "*": {"*", pMul}, "/": {"/", pMul}, "%": {"%", pMul},
	"<": {"<", pCmp}, "<=": {"<=", pCmp}, "=": {"==", pCmp}, ">=": {">=", pCmp}, ">": {">", pCmp},
	"!=": {"!=", pCmp}, "concat": {"++", pAdd},
	"&": {"&", pMul}, "<<": {"<<", pMul}, ">>": {">>", pMul}, "mod": {"mod", pMul},
	"|": {"|", pAdd}, "⊕": {"⊕", pAdd},
}

func paren(s string, inner, min int) string {
	if inner < min {
		return "(" + s + ")"
	}
	return s
}

func showAt(t *Term, min int) string {
	switch t.Kind {
	case KName:
		return ShowName(t.Name)
	case KStr:
		return quoteJ(t.Str)
	case KInt:
		// DECIMAL, not Integer's powers: a power is folded with a neighbouring
		// literal when read (`2^63-1 + 1` is one literal), so it is a set's
		// spelling and not a term's.
		return paren(strconv.FormatInt(t.Int, 10), pPrefix, min)
	case KFn:
		if s, ok := showTuple(t); ok {
			return s
		}
		return paren("fn("+strings.Join(ShowNames(t.Params), ", ")+") { "+showBody(t.Body())+" }", pLowest+1, min)
	case KApp:
		if v, ok := bigLiteralValue(t); ok {
			return v
		}
		return showApp(t, min)
	}
	return t.String()
}

// bigLiteralValue is a literal past int64, which the core holds as Horner's
// spine over 10¹⁵ (bigLiteral), as the decimal it was written as. Only the
// exact spine: an arithmetic of literals the program wrote stays arithmetic.
func bigLiteralValue(t *Term) (string, bool) {
	v, ok := EvalEndpoint(t)
	if !ok || v.IsInt64() {
		return "", false
	}
	if IntLiteral(v.String()).String() != t.String() {
		return "", false
	}
	return v.String(), true
}

// showTuple is a Church tuple, `(fn (#k) (#k a b))`, as the pair it is.
func showTuple(t *Term) (string, bool) {
	if len(t.Params) != 1 || t.Params[0] != "#k" {
		return "", false
	}
	b := t.Body()
	if b.Kind != KApp || len(b.Kids) == 0 || b.Kids[0].Kind != KName || b.Kids[0].Name != "#k" {
		return "", false
	}
	return "(" + ShowTerms(b.Kids[1:]) + ")", true
}

// showBody is a block's contents: its bindings and its value, without braces.
func showBody(t *Term) string {
	var stmts []string
	for {
		if t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KFn && len(t.Kids[0].Params) == 1 {
			// A REDEX IS A BINDING (binding.md): ((fn (x) b) e) is { x = e; b },
			// and a discarded name a statement.
			p := t.Kids[0].Params[0]
			if p == "#_" {
				stmts = append(stmts, ShowTerm(t.Kids[1]))
			} else {
				stmts = append(stmts, ShowName(p)+" = "+ShowTerm(t.Kids[1]))
			}
			t = t.Kids[0].Body()
			continue
		}
		if t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName && t.Kids[0].Name == "let" &&
			t.Kids[2].Kind == KFn && len(t.Kids[2].Params) == 1 {
			// The residual's let, (let e (fn (x) b)).
			stmts = append(stmts, ShowName(t.Kids[2].Params[0])+" = "+ShowTerm(t.Kids[1]))
			t = t.Kids[2].Body()
			continue
		}
		if p, v, body, ok := tupleBinding(t); ok {
			stmts = append(stmts, "("+strings.Join(ShowNames(p), ", ")+") = "+ShowTerm(v))
			t = body
			continue
		}
		break
	}
	stmts = append(stmts, ShowTerm(t))
	return strings.Join(stmts, "; ")
}

// tupleBinding recognises a tuple pattern's binding, the product's eliminator
// applied to a continuation: (v (fn (a b) body)), D's image of
// `(a, b) = v; body`.
func tupleBinding(t *Term) ([]string, *Term, *Term, bool) {
	if t.Kind != KApp || len(t.Kids) != 2 || t.Kids[0].Kind == KFn || t.Kids[1].Kind != KFn ||
		len(t.Kids[1].Params) < 2 {
		return nil, nil, nil, false
	}
	if t.Kids[0].Kind == KName && infixOrForm(t.Kids[0].Name) {
		return nil, nil, nil, false
	}
	return t.Kids[1].Params, t.Kids[0], t.Kids[1].Body(), true
}

func infixOrForm(n string) bool {
	if _, ok := infix[n]; ok {
		return true
	}
	switch n {
	case "if", "let", "loop", "again", "case", "try", "expect", "set", "build", "build-map":
		return true
	}
	return false
}

func isBlock(t *Term) bool {
	if t.Kind != KApp || len(t.Kids) == 0 {
		return false
	}
	if len(t.Kids) == 2 && t.Kids[0].Kind == KFn && len(t.Kids[0].Params) == 1 {
		return true
	}
	if len(t.Kids) == 3 && t.Kids[0].Kind == KName && t.Kids[0].Name == "let" {
		return true
	}
	_, _, _, ok := tupleBinding(t)
	return ok
}

func showApp(t *Term, min int) string {
	if len(t.Kids) == 0 {
		return "()"
	}
	if isBlock(t) {
		// A block is a primary, but not a callee: `{ … }(i)` ends a statement.
		return paren("{ "+showBody(t)+" }", pPost-1, min)
	}
	head, args := t.Kids[0], t.Kids[1:]
	if head.Kind == KName {
		n := head.Name
		if op, ok := infix[n]; ok && len(args) == 2 {
			l := showAt(args[0], op.prec)
			r := showAt(args[1], op.prec+1)
			return paren(l+" "+op.op+" "+r, op.prec, min)
		}
		switch {
		case n == "-" && len(args) == 1:
			return paren("-"+showAt(args[0], pPrefix), pPrefix, min)
		case n == "~" && len(args) == 1:
			return paren("~"+showAt(args[0], pPrefix), pPrefix, min)
		case n == "if" && len(args) == 3:
			return showIf(args[0], args[1], args[2], min)
		case n == "again":
			return "again(" + ShowTerms(args) + ")"
		case n == "set" && len(args) == 3:
			return "store(" + ShowTerms(args) + ")"
		case n == "set" && len(args) == 2:
			return paren(showAt(args[0], pPost)+" := "+ShowTerm(args[1]), pLowest+1, min)
		case n == "loop" && len(args) >= 1 && args[0].Kind == KFn && len(args[0].Params) == len(args)-1:
			return showLoop(args[0], args[1:])
		case n == "case" && len(args) >= 3 && len(args)%2 == 1:
			var arms []string
			for i := 1; i+1 < len(args); i += 2 {
				arms = append(arms, showPat(args[i])+" -> "+ShowTerm(args[i+1]))
			}
			return "case " + showAt(args[0], pCmp+1) + " { " + strings.Join(arms, "; ") + " }"
		case n == "try" && len(args) == 3:
			return "try " + showPat(args[0]) + " = " + ShowTerm(args[1]) + " { " + showBody(args[2]) + " }"
		case n == "expect" && len(args) == 4:
			return "{ expect " + showPat(args[0]) + " = " + ShowTerm(args[1]) + " else " + ShowTerm(args[2]) +
				"; " + showBody(args[3]) + " }"
		}
	}
	return showAt(head, pPost) + "(" + ShowTerms(args) + ")"
}

// showIf is a conditional: a connective where D made one, `!c`, `c && a`,
// `c || b`, a chain of guards as `case { … }`, and otherwise `if … else …`.
func showIf(c, a, b *Term, min int) string {
	isB := func(t *Term, v bool) bool { return t.Kind == KBool && t.IsTrue() == v }
	switch {
	case isB(a, false) && isB(b, true):
		return paren("!"+showAt(c, pPrefix), pPrefix, min)
	case isB(b, false):
		return paren(showAt(c, pAnd+1)+" && "+showAt(a, pAnd), pAnd, min)
	case isB(a, true):
		return paren(showAt(c, pOr+1)+" || "+showAt(b, pOr), pOr, min)
	}
	if guards, bodies, last := ifChain(c, a, b); len(guards) > 1 {
		arms := make([]string, len(guards))
		for i := range guards {
			arms[i] = ShowTerm(guards[i]) + " -> " + ShowTerm(bodies[i])
		}
		return paren("case { "+strings.Join(arms, "; ")+"; else -> "+ShowTerm(last)+" }", pLowest+1, min)
	}
	return paren("if "+ShowTerm(c)+" { "+showBody(a)+" } else { "+showBody(b)+" }", pLowest+1, min)
}

// ifChain unfolds a right-nested chain of `if` that is not a connective.
func ifChain(c, a, b *Term) (guards, bodies []*Term, last *Term) {
	guards, bodies = []*Term{c}, []*Term{a}
	for {
		if b.Kind != KApp || len(b.Kids) != 4 || b.Kids[0].Kind != KName || b.Kids[0].Name != "if" {
			return guards, bodies, b
		}
		x, y, z := b.Kids[1], b.Kids[2], b.Kids[3]
		if y.Kind == KBool || z.Kind == KBool {
			return guards, bodies, b
		}
		guards, bodies, b = append(guards, x), append(bodies, y), z
	}
}

// showLoop is `(loop (fn (x…) arms) init…)` as `loop x = init, … { arms }`.
func showLoop(body *Term, inits []*Term) string {
	binds := make([]string, len(inits))
	for i, init := range inits {
		binds[i] = ShowName(body.Params[i]) + " = " + ShowTerm(init)
	}
	var arms []string
	t := body.Body()
	for isChoice(t) {
		c, a, b := t.Kids[1], t.Kids[2], t.Kids[3]
		if !isChoice(b) && isChoice(a) {
			// D BUILT (if (not c) b a) AS (if c a b) (mkIf), so an arm whose
			// guard was negated is the else branch, and the chain goes on in
			// the then branch: `!c -> b`.
			arms = append(arms, "!"+showAt(c, pPrefix)+" -> "+ShowTerm(b))
			t = a
			continue
		}
		arms = append(arms, ShowTerm(c)+" -> "+ShowTerm(a))
		t = b
	}
	arms = append(arms, "else -> "+ShowTerm(t))
	return "loop " + strings.Join(binds, ", ") + " { " + strings.Join(arms, "; ") + " }"
}

// isChoice is an `if` that is not a connective.
func isChoice(t *Term) bool {
	return t.Kind == KApp && len(t.Kids) == 4 && t.Kids[0].Kind == KName && t.Kids[0].Name == "if" &&
		t.Kids[2].Kind != KBool && t.Kids[3].Kind != KBool
}

// showPat is a case pattern: a constructor, quoted (ADR 0059), applied to its
// binding, or `else`.
func showPat(p *Term) string {
	switch {
	case p.Kind == KName && p.Name == "else":
		return "else"
	case p.Kind == KName:
		return tagJ(p.Name)
	case p.Kind == KApp && len(p.Kids) >= 1 && p.Kids[0].Kind == KName:
		args := make([]string, len(p.Kids)-1)
		for i, k := range p.Kids[1:] {
			args[i] = ShowTerm(k)
		}
		return tagJ(p.Kids[0].Name) + "(" + strings.Join(args, ", ") + ")"
	}
	return ShowTerm(p)
}

// tagJ is a tag, `'name`, or `alias.'name` for a qualified one.
func tagJ(n string) string {
	if i := strings.LastIndexByte(n, '.'); i > 0 {
		return ShowName(n[:i]) + ".'" + word(n[i+1:], true)
	}
	return "'" + word(n, true)
}

// quoteJ writes a string literal with string-literals.md's escapes.
func quoteJ(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f || isBidiControl(r):
			b.WriteString(`\u{` + strconv.FormatInt(int64(r), 16) + `}`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
