package emit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"oroboros/core"
)

// Java backend — the third target of ADR 0004, and the one that settles whether
// the shape shared by the Go and JS emitters is a line or a coincidence.
//
// Written standalone like the other two, for the same reason: what three
// backends turn out to share is the evidence for ADR 0006's backend interface,
// and factoring after two would have assumed the answer.
//
// Java diverges from both predecessors in ways worth naming up front:
//
//   - It has no free functions, so a file is a *class*. Neither Go nor JS
//     needed a wrapper.
//   - It needs types, like Go and unlike JS — so the type lattice is required by
//     two targets out of three, not one.
//   - Its dictionary boxes: HashMap<String,Integer>, not map[string]int.
//   - Its fused increment LOSES. Baseline R5 measured merge(k,1,Integer::sum)
//     at 2.6x slower than getOrDefault+put, the opposite of Go's m[k]++. This is
//     the Parasite thesis at its sharpest: the same capability, opposite idioms.

func (t Ty) Java() string {
	switch t {
	case TF64:
		return "double"
	case TInt:
		return "int"
	case TBool:
		return "boolean"
	case TVecF64:
		return "double[]"
	case TString:
		return "String"
	case TVecString:
		return "String[]"
	case TDict:
		return "HashMap<String,Integer>"
	}
	return "Object" // will not compile — deliberately loud
}

type javaPrim struct {
	Format string
	Args   []Ty
	Result Ty
	Loop   bool
	Loop2  bool
	Cond   bool
	Stmt   bool
	Let    bool
	Import string
}

var javaPrims = map[string]javaPrim{
	"add": {Format: "%s + %s", Args: []Ty{TF64, TF64}, Result: TF64},
	"mul": {Format: "%s * %s", Args: []Ty{TF64, TF64}, Result: TF64},
	"sub": {Format: "%s - %s", Args: []Ty{TF64, TF64}, Result: TF64},
	"gt":  {Format: "%s > %s", Args: []Ty{TF64, TF64}, Result: TBool},
	"lt":  {Format: "%s < %s", Args: []Ty{TF64, TF64}, Result: TBool},

	// Java arrays carry .length, so alen and slen are the same shape — unlike
	// Go, where len() is a function, and JS, where it is a property.
	"alen":   {Format: "%s.length", Args: []Ty{TVecF64}, Result: TInt},
	"aindex": {Format: "%s[%s]", Args: []Ty{TVecF64, TInt}, Result: TF64},
	"slen":   {Format: "%s.length", Args: []Ty{TVecString}, Result: TInt},
	"sat":    {Format: "%s[%s]", Args: []Ty{TVecString, TInt}, Result: TString},

	"split-words": {Format: "%s.split(\" \")", Args: []Ty{TString}, Result: TVecString},
	"dict-empty": {Format: "new HashMap<String,Integer>()", Args: nil, Result: TDict,
		Import: "java.util.HashMap"},

	// The UNFUSED form, deliberately. Baseline R5 measured Java's fused
	// merge(k, 1, Integer::sum) at 2.6x SLOWER than this, because merge boxes
	// and makes a functional call per entry. Go's fused m[k]++ wins; Java's
	// fused merge loses. Same capability, opposite idiom, one source.
	"dict-inc": {Format: "%s.put(%s, %s.getOrDefault(%s, 0) + 1)",
		Args: []Ty{TDict, TString}, Result: TDict, Stmt: true},

	"let":         {Args: []Ty{TUnknown, TUnknown}, Result: TUnknown, Let: true},
	"if":          {Args: []Ty{TBool, TUnknown, TUnknown}, Result: TUnknown, Cond: true},
	"fold-range":  {Args: []Ty{TF64, TInt, TUnknown}, Result: TF64, Loop: true},
	"fold-range2": {Args: []Ty{TF64, TF64, TInt, TUnknown, TUnknown, TUnknown}, Result: TF64, Loop2: true},
}

type javaEmitter struct {
	buf     strings.Builder
	imports map[string]bool
	types   map[string]Ty
	tmp     int
	indent  int
}

// JavaImports accumulates what emitted methods need, like the Go sink.
var JavaImports = map[string]bool{}

// JavaMethod emits a top-level abstraction as a static method.
func JavaMethod(name string, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &javaEmitter{types: map[string]Ty{}, imports: map[string]bool{}, indent: 2}
	e.inferFrom(t.Body())
	e.inferLet(t.Body())
	e.inferFrom(t.Body())

	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		ty := e.types[p]
		if ty == TUnknown {
			return "", fmt.Errorf("cannot determine a Java type for parameter %q", p)
		}
		params[i] = ty.Java() + " " + javaMangle(p)
	}

	result, err := e.emit(t.Body())
	if err != nil {
		return "", err
	}
	for imp := range e.imports {
		JavaImports[imp] = true
	}

	var out strings.Builder
	fmt.Fprintf(&out, "\tpublic static %s %s(%s) {\n", e.typeOf(t.Body()).Java(),
		javaMangle(name), strings.Join(params, ", "))
	out.WriteString(e.buf.String())
	fmt.Fprintf(&out, "\t\treturn %s;\n\t}\n", result)
	return out.String(), nil
}

func (e *javaEmitter) inferFrom(t *core.Term) {
	switch t.Kind {
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			if p, ok := javaPrims[op.Name]; ok {
				for i, a := range t.Args() {
					if i < len(p.Args) && a.Kind == core.KName && e.types[a.Name] == TUnknown {
						e.types[a.Name] = p.Args[i]
					}
				}
			}
		}
		for _, k := range t.Kids {
			e.inferFrom(k)
		}
	case core.KFn:
		e.inferFrom(t.Body())
	}
}

func (e *javaEmitter) inferLet(t *core.Term) {
	if t.Kind == core.KApp && t.Op().Kind == core.KName && t.Op().Name == "let" {
		if k := t.Args()[1]; k.Kind == core.KFn && len(k.Params) == 1 {
			e.types[k.Params[0]] = e.typeOf(t.Args()[0])
		}
	}
	for _, k := range t.Kids {
		e.inferLet(k)
	}
	if t.Kind == core.KFn {
		e.inferLet(t.Body())
	}
}

func (e *javaEmitter) typeOf(t *core.Term) Ty {
	switch t.Kind {
	case core.KInt:
		return TInt
	case core.KFloat:
		return TF64
	case core.KName:
		return e.types[t.Name]
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			if p, ok := javaPrims[op.Name]; ok {
				if p.Loop {
					return e.typeOf(t.Args()[0])
				}
				if p.Let {
					if k := t.Args()[1]; k.Kind == core.KFn {
						return e.typeOf(k.Body())
					}
				}
				return p.Result
			}
		}
	}
	return TUnknown
}

func (e *javaEmitter) line(format string, args ...any) {
	e.buf.WriteString(strings.Repeat("\t", e.indent))
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *javaEmitter) fresh(stem string) string {
	e.tmp++
	return fmt.Sprintf("%s%d", stem, e.tmp)
}

func (e *javaEmitter) emit(t *core.Term) (string, error) {
	switch t.Kind {
	case core.KInt:
		return strconv.FormatInt(t.Int, 10), nil
	case core.KFloat:
		s := strconv.FormatFloat(t.Float, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s, nil
	case core.KName:
		return javaMangle(t.Name), nil
	case core.KFn:
		return "", fmt.Errorf("a bare abstraction reached the emitter: %s\n"+
			"  An escaping closure. Java has lambdas but g6's cost model has not\n"+
			"  been checked against them.", t)
	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return "", fmt.Errorf("application of a non-name: %s", t)
		}
		p, ok := javaPrims[op.Name]
		if !ok {
			return "", fmt.Errorf("no Java form for primitive %q", op.Name)
		}
		if p.Import != "" {
			e.imports[p.Import] = true
		}
		switch {
		case p.Let:
			return e.emitLet(t)
		case p.Loop:
			return e.emitFoldRange(t)
		case p.Loop2:
			return e.emitFoldRange2(t)
		case p.Cond:
			return e.emitIf(t)
		case p.Stmt:
			args := t.Args()
			vals := make([]any, 0, 2*len(args))
			for _, a := range args {
				v, err := e.emit(a)
				if err != nil {
					return "", err
				}
				vals = append(vals, v)
			}
			// dict-inc names both operands twice.
			e.line("%s;", fmt.Sprintf(p.Format, append(vals, vals...)...))
			return vals[0].(string), nil
		}
		args := t.Args()
		if len(args) != len(p.Args) {
			return "", fmt.Errorf("%s takes %d argument(s), given %d", op.Name, len(p.Args), len(args))
		}
		vals := make([]any, len(args))
		for i, a := range args {
			v, err := e.emit(a)
			if err != nil {
				return "", err
			}
			vals[i] = v
		}
		return "(" + fmt.Sprintf(p.Format, vals...) + ")", nil
	}
	return "", fmt.Errorf("unhandled term: %s", t)
}

func (e *javaEmitter) emitLet(t *core.Term) (string, error) {
	args := t.Args()
	k := args[1]
	if k.Kind != core.KFn || len(k.Params) != 1 {
		return "", fmt.Errorf("let's continuation must be (fn (x) …), got %s", k)
	}
	val, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	ty := e.typeOf(args[0])
	e.types[k.Params[0]] = ty
	e.line("final %s %s = %s;", ty.Java(), javaMangle(k.Params[0]), val)
	return e.emit(k.Body())
}

// emitIf uses Java's conditional expression when both branches are pure — like
// JS and unlike Go, which has none. So the ANF that g3 §6 derived is required by
// exactly one of the three targets.
func (e *javaEmitter) emitIf(t *core.Term) (string, error) {
	args := t.Args()
	cond, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	saved, savedIndent := e.buf.String(), e.indent
	e.buf.Reset()
	thenV, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	elseV, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	branchStmts := e.buf.String()
	e.buf.Reset()
	e.buf.WriteString(saved)
	e.indent = savedIndent

	if branchStmts == "" {
		return fmt.Sprintf("(%s ? %s : %s)", cond, thenV, elseV), nil
	}
	ty := e.typeOf(args[1])
	if ty == TUnknown {
		ty = e.typeOf(args[2])
	}
	tmp := e.fresh("t")
	e.line("%s %s;", ty.Java(), tmp)
	e.line("if (%s) {", cond)
	e.indent++
	tv, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	e.line("%s = %s;", tmp, tv)
	e.indent--
	e.line("} else {")
	e.indent++
	ev, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	e.line("%s = %s;", tmp, ev)
	e.indent--
	e.line("}")
	return tmp, nil
}

func (e *javaEmitter) emitFoldRange(t *core.Term) (string, error) {
	args := t.Args()
	step := args[2]
	if step.Kind != core.KFn || len(step.Params) != 2 {
		return "", fmt.Errorf("fold-range's third argument must be (fn (acc i) …), got %s", step)
	}
	accName, idxName := step.Params[0], step.Params[1]
	e.types[accName] = e.typeOf(args[0])
	e.types[idxName] = TInt

	init, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	count, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	acc, idx := javaMangle(accName), javaMangle(idxName)
	n := e.fresh("n")

	e.line("%s %s = %s;", e.types[accName].Java(), acc, init)
	e.line("final int %s = %s;", n, count)
	e.line("for (int %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(step.Body())
	if err != nil {
		return "", err
	}
	if body != acc { // a statement-primitive already updated it in place
		e.line("%s = %s;", acc, body)
	}
	e.indent--
	e.line("}")
	return acc, nil
}

// emitFoldRange2 needs temporaries for the simultaneous update, like JS and
// unlike Go — Java has no tuple assignment either. g2 §6's parallel-assignment
// discipline, required by two targets of three.
func (e *javaEmitter) emitFoldRange2(t *core.Term) (string, error) {
	args := t.Args()
	sx, sy, fin := args[3], args[4], args[5]
	for _, s := range []*core.Term{sx, sy} {
		if s.Kind != core.KFn || len(s.Params) != 3 {
			return "", fmt.Errorf("fold-range2 steps must be (fn (ax ay i) …), got %s", s)
		}
	}
	if fin.Kind != core.KFn || len(fin.Params) != 2 {
		return "", fmt.Errorf("fold-range2's finisher must be (fn (ax ay) …), got %s", fin)
	}
	x0, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	y0, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	count, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	ax, ay, idx := javaMangle(sx.Params[0]), javaMangle(sx.Params[1]), javaMangle(sx.Params[2])
	e.types[sx.Params[0]], e.types[sx.Params[1]], e.types[sx.Params[2]] = TF64, TF64, TInt
	e.types[sy.Params[0]], e.types[sy.Params[1]], e.types[sy.Params[2]] = TF64, TF64, TInt
	n := e.fresh("n")

	e.line("double %s = %s, %s = %s;", ax, x0, ay, y0)
	e.line("final int %s = %s;", n, count)
	e.line("for (int %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	bx, err := e.emit(sx.Body())
	if err != nil {
		return "", err
	}
	by, err := e.emit(core.Rename(sy.Body(), map[string]string{
		sy.Params[0]: sx.Params[0], sy.Params[1]: sx.Params[1], sy.Params[2]: sx.Params[2]}))
	if err != nil {
		return "", err
	}
	tx, ty2 := e.fresh("u"), e.fresh("u")
	e.line("final double %s = %s, %s = %s;", tx, bx, ty2, by)
	e.line("%s = %s; %s = %s;", ax, tx, ay, ty2)
	e.indent--
	e.line("}")
	e.types[fin.Params[0]], e.types[fin.Params[1]] = TF64, TF64
	return e.emit(core.Rename(fin.Body(), map[string]string{
		fin.Params[0]: sx.Params[0], fin.Params[1]: sx.Params[1]}))
}

// javaMangle is the THIRD instance of the same transformation. Hyphen to
// camelCase, ? to P, ! to B, everything else escaped — identical to mangle and
// jsMangle. Only the keyword set differs. Three targets agreeing is strong
// evidence that mangling belongs to the language rather than to any backend.
func javaMangle(s string) string {
	var b strings.Builder
	upper := false
	for _, r := range s {
		switch {
		case r == '-' || r == '.':
			upper = true
		case r == '?':
			b.WriteString("P")
		case r == '!':
			b.WriteString("B")
		case unicode.IsLetter(r) || r == '_' || r == '$':
			if upper {
				b.WriteString(strings.ToUpper(string(r)))
				upper = false
			} else {
				b.WriteRune(r)
			}
		case unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "u%04X", r)
		}
	}
	out := b.String()
	if out == "" || unicode.IsDigit(rune(out[0])) {
		out = "x" + out
	}
	if javaKeywords[out] {
		out += "_"
	}
	return out
}

var javaKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`abstract assert boolean break byte case catch char class
		const continue default do double else enum extends final finally float for goto if
		implements import instanceof int interface long native new package private protected
		public return short static strictfp super switch synchronized this throw throws
		transient try void volatile while true false null var record yield sealed permits`) {
		javaKeywords[k] = true
	}
}

// JavaFile wraps emitted methods in a class. Java has no free functions, so the
// wrapper is mandatory — the first structural requirement neither Go nor JS had.
func JavaFile(class string, funcs map[string]string) string {
	names := make([]string, 0, len(funcs))
	for n := range funcs {
		names = append(names, n)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\n")
	if len(JavaImports) > 0 {
		imps := make([]string, 0, len(JavaImports))
		for i := range JavaImports {
			imps = append(imps, i)
		}
		sort.Strings(imps)
		for _, i := range imps {
			fmt.Fprintf(&out, "import %s;\n", i)
		}
		out.WriteString("\n")
	}
	fmt.Fprintf(&out, "public final class %s {\n", class)
	for _, n := range names {
		out.WriteString(funcs[n])
		out.WriteString("\n")
	}
	out.WriteString("}\n")
	return out.String()
}
