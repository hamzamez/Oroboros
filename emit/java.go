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

type javaEmitter struct {
	tgt     *Target
	buf     strings.Builder
	imports map[string]bool
	types   map[string]string
	weak    map[string]string // `any`, used only if nothing else constrains the name
	tmp     int
	indent  int
	// rec is the record a multi-result method returns, set by multiFunc and
	// read at every leaf multiTail returns from.
	rec string

	// sig and topParams are the enclosing function's contract and the names
	// this backend opened its parameters with. A `build` buffer's stores are
	// usually bounded by something the signature says, and the sub-pass that
	// works out the element range needs both to see it.
	sig       *core.Sig
	topParams []string

	// fitsIdx is whether EVERY integer operation in this method stays inside
	// [-2^31, 2^31-1] — computed once, with the signature, because a loop's
	// bound usually comes from the enclosing `where` and the loop alone does
	// not have it. See NarrowByInterval.
	fitsIdx bool

	// bound is every name already emitted in this method — see openFresh.
	bound map[string]bool

	// narrow is the loop variables emitted as a host `int` rather than as the
	// language's `int`, which is 64-bit here. See NarrowIndex: it is what
	// removes the `(int)` cast from every array access, worth 1.04x to 1.45x
	// against hand-written Java (native-java-2026-08-25).
	narrow map[string]bool
}

// JavaImports accumulates what emitted methods need, like the Go sink.
var JavaImports = map[string]bool{}

// JavaMethod emits a top-level abstraction as a static method.
func JavaMethod(tgt *Target, name string, sig *core.Sig, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &javaEmitter{tgt: tgt, types: map[string]string{}, weak: map[string]string{},
		imports: map[string]bool{}, indent: 2, bound: map[string]bool{}}
	// INDEX-TYPE SELECTION asks one question of the whole method, once: does
	// every integer operation in it stay inside the host's index type? A loop
	// cannot answer that alone, because its bound usually comes from the
	// signature's `where`.
	e.sig, e.topParams = sig, t.Params
	rep, _ := Intervals(tgt, sig, t, 0)
	if rep.FitsIndex() {
		e.fitsIdx = true
	}
	for _, p := range t.Params {
		e.bound[javaMangle(p)] = true
	}
	seedFromSig(e.types, t.Params, sig)
	e.inferFrom(t.Body())
	e.inferLet(t.Body())
	e.inferFrom(t.Body())

	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		ty := e.types[p]
		if ty == "" {
			ty = e.weak[p]
		}
		if ty == "" {
			return "", fmt.Errorf("cannot determine a Java type for parameter %q", p)
		}
		params[i] = e.tgt.ty(ty) + " " + javaMangle(p)
	}

	// SEVERAL RESULTS — the negative product reaching a boundary.
	if sig != nil && len(sig.Results) > 1 {
		return e.multiFunc(name, sig, t, params)
	}

	result, err := e.emit(t.Body())
	if err != nil {
		return "", err
	}
	for imp := range e.imports {
		JavaImports[imp] = true
	}

	var out strings.Builder
	fmt.Fprintf(&out, "\tpublic static %s %s(%s) {\n", e.tgt.ty(e.typeOf(t.Body())),
		javaMangle(name), strings.Join(params, ", "))
	out.WriteString(e.buf.String())
	fmt.Fprintf(&out, "\t\treturn %s;\n\t}\n", result)
	return out.String(), nil
}

// JavaRecords collects the record types generated for functions with several
// results, keyed by name so two functions with the same result shape share one.
var JavaRecords = map[string]string{}

// multiFunc emits a function with several results.
//
// The JVM has no multiple return, so the compiler builds the construct out of
// what the host has — which is the compiler's job and not a target author's.
// The first attempt at this feature let Java DECLINE it and was reverted for
// exactly that.
//
// A record, and shared by result SHAPE rather than generated per function, so
// two functions returning (int int) use one type. product-2026-08-19 measured
// C2 scalar-replacing a record returned from a hot call at 0.96x — inside the
// noise — so this should cost nothing when it does not escape. Whether that
// still holds across a compilation-unit boundary is the open question this
// build exists to answer.
func (e *javaEmitter) multiFunc(name string, sig *core.Sig, t *core.Term, params []string) (string, error) {
	tys := make([]string, len(sig.Results))
	for i, r := range sig.Results {
		tys[i] = e.tgt.ty(r)
	}
	rec := javaRecordName(tys)
	if _, have := JavaRecords[rec]; !have {
		fields := make([]string, len(tys))
		for i, ty := range tys {
			fields[i] = fmt.Sprintf("%s f%d", ty, i)
		}
		JavaRecords[rec] = fmt.Sprintf("\tpublic record %s(%s) {}\n", rec, strings.Join(fields, ", "))
	}

	e.rec = rec
	if err := e.multiTail(t.Body(), len(sig.Results), name, sig); err != nil {
		return "", err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "\tpublic static %s %s(%s) {\n", rec, javaMangle(name),
		strings.Join(params, ", "))
	out.WriteString(e.buf.String())
	out.WriteString("\t}\n")
	return out.String(), nil
}

// javaRecordName builds a valid identifier from the component types, so the
// name is a function of the SHAPE. `long[]` becomes `longArr`.
// multiTail returns from every leaf rather than joining the branches first.
// See the Go emitter for the argument.
func (e *javaEmitter) multiTail(t *core.Term, n int, name string, sig *core.Sig) error {
	if vs, ok := multiValue(t, n); ok {
		outs := make([]string, len(vs))
		for i, v := range vs {
			x, err := e.emit(v)
			if err != nil {
				return err
			}
			outs[i] = x
		}
		e.line("return new %s(%s);", e.rec, strings.Join(outs, ", "))
		return nil
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		p, known := e.tgt.Prims[t.Op().Name]
		_, isConn := connective(e.tgt, t)
		if known && p.Kind == "cond" && len(t.Args()) == 3 && !isConn {
			args := t.Args()
			cond, err := e.emit(args[0])
			if err != nil {
				return err
			}
			e.line("if (%s) {", cond)
			e.indent++
			if err := e.multiTail(args[1], n, name, sig); err != nil {
				return err
			}
			e.indent--
			e.line("}")
			return e.multiTail(args[2], n, name, sig)
		}
		if known && p.Kind == "let" && len(t.Args()) == 2 {
			args := t.Args()
			k := args[1]
			if k.Kind == core.KFn && len(k.Params) == 1 {
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					return e.multiTail(k.Body(), n, name, sig)
				}
				ty := e.typeOf(args[0])
				body, raw, out := openFresh(k, e.bound, javaMangle)
				e.types[raw[0]] = ty
				e.line("%s %s = %s;", e.tgt.ty(ty), out[0], val)
				return e.multiTail(body, n, name, sig)
			}
		}
	}
	return multiResultErr(name, sig, t)
}

func javaRecordName(tys []string) string {
	var b strings.Builder
	b.WriteString("Tup")
	for _, ty := range tys {
		b.WriteByte('_')
		for _, r := range ty {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
				b.WriteRune(r)
			case r == '[':
				b.WriteString("Arr")
			}
		}
	}
	return b.String()
}

func (e *javaEmitter) inferFrom(t *core.Term) {
	switch t.Kind {
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			if p, ok := e.tgt.Prims[op.Name]; ok {
				for i, a := range t.Args() {
					if i >= len(p.Args) || a.Kind != core.KName {
						continue
					}
					// `any` is the absence of a constraint — see the Go backend.
					if p.Args[i] == "any" {
						e.weak[a.Name] = "any"
					} else if e.types[a.Name] == "" {
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

func (e *javaEmitter) typeOf(t *core.Term) string {
	switch t.Kind {
	case core.KInt:
		return "int"
	case core.KFloat:
		return "f64"
	case core.KBool:
		// Missing until the write side needed it: `bufferElem` reads a
		// buffer's element type off the value a `set` stores, and the sieve
		// stores `true`, so an absent KBool case made every boolean buffer a
		// long[] and javac refused the file.
		return "bool"
	case core.KName:
		return e.types[t.Name]
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			// Indexing is application, so `(a i)` has a's ELEMENT type.
			if elem := core.ArrayElem(e.types[op.Name]); elem != "" {
				return elem
			}
			if p, ok := e.tgt.Prims[op.Name]; ok {
				// The write side's result types. A `build` yields the array
				// its buffer became; `alloc` and `set` pass one through. Java
				// needs this because a local needs a written type, and without
				// it the frozen buffer bound by a `let` emitted
				// `final /*unknown*/ c4 = c2;`.
				if p.Kind == "table-build" && len(t.Args()) == 2 {
					if lam := t.Args()[1]; lam.Kind == core.KFn && len(lam.Params) == 1 {
						return e.buildType(lam)
					}
				}
				if (p.Kind == "table-alloc" || p.Kind == "table-set") && len(t.Args()) >= 1 {
					return e.typeOf(t.Args()[0])
				}
				if p.Kind == "table" && len(t.Args()) == 2 {
					if rule := t.Args()[1]; rule.Kind == core.KFn && len(rule.Params) == 1 {
						return "array " + e.typeOf(rule.Body())
					}
				}
				if p.Kind == "array" && len(t.Args()) > 0 {
					return "array " + e.typeOf(t.Args()[0])
				}
				if p.Kind == "loop" {
					return e.typeOf(t.Args()[0])
				}
				if p.Kind == "build" {
					return "vec-f64"
				}
				if p.Kind == "iterate" && len(t.Args()) >= 1 {
					if lam := t.Args()[0]; lam.Kind == core.KFn {
						for i, n := range lam.Params {
							e.types[n] = e.typeOf(t.Args()[1+i])
						}
						if ty := javaExitType(e, lam.Body()); ty != "" {
							return ty
						}
					}
				}
				if p.Kind == "let" {
					if k := t.Args()[1]; k.Kind == core.KFn {
						return e.typeOf(k.Body())
					}
				}
				// A conditional's type is its branches'. Missing here for the
				// same reason it was missing on Go: no program's body was an
				// `if` until `and` and `or` became conditionals (ADR 0017).
				if p.Kind == "cond" && len(t.Args()) == 3 {
					if ty := e.typeOf(t.Args()[1]); ty != "" {
						return ty
					}
					return e.typeOf(t.Args()[2])
				}
				// loop2's result is its FINISHER's type — see the Go backend.
				if p.Kind == "loop2" && len(t.Args()) == 6 {
					if fin := t.Args()[5]; fin.Kind == core.KFn && len(fin.Params) == 2 {
						e.types[fin.Params[0]], e.types[fin.Params[1]] = "f64", "f64"
						if ty := e.typeOf(fin.Body()); ty != "" {
							return ty
						}
					}
					return e.typeOf(t.Args()[0])
				}
				// A statement's value is argument 0 — see the Go backend.
				if p.Kind == "stmt" && len(t.Args()) > 0 {
					if ty := e.typeOf(t.Args()[0]); ty != "" {
						return ty
					}
				}
				return p.Result
			}
		}
	}
	return ""
}

func (e *javaEmitter) line(format string, args ...any) {
	e.buf.WriteString(strings.Repeat("\t", e.indent))
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *javaEmitter) fresh(stem string) string {
	for {
		e.tmp++
		n := fmt.Sprintf("%s%d", stem, e.tmp)
		if !e.bound[n] {
			e.bound[n] = true
			return n
		}
	}
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
	case core.KBool:
		if t.IsTrue() {
			return "true", nil
		}
		return "false", nil

	case core.KStr:
		// A literal was added to the language for target templates and no
		// backend could emit one, because no program had ever used one
		// (strings.md 1). strconv.Quote escapes non-ASCII to \u, which is
		// valid in all three hosts and sidesteps javac's platform-charset
		// hazard entirely (strings.md 5).
		return strconv.Quote(t.Str), nil

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
		p, ok := e.tgt.Prims[op.Name]
		if !ok {
			if IsTableOperand(javaMangle(op.Name), e.bound) && len(t.Args()) == 1 {
				a, err := e.emit(op)
				if err != nil {
					return "", err
				}
				idx, err := e.emit(t.Args()[0])
				if err != nil {
					return "", err
				}
				// The (int) CAST is Java's, and it is not optional: our `int`
				// maps to `long` and a Java array index must be an `int`, so
				// without it javac rejects the file with "possible lossy
				// conversion". The declaration `at-double` used to carry it as
				// `%s[(int) %s]`; moving indexing into the backend moves the
				// cast with it, which is the host detail a target author no
				// longer has to know.
				//
				// Unless the index is already a host `int` — see NarrowIndex.
				// Then the cast is not merely redundant, it is the thing that
				// cost 1.04x to 1.45x.
				if e.narrowIdx(t.Args()[0]) {
					return fmt.Sprintf("%s[%s]", a, idx), nil
				}
				return fmt.Sprintf("%s[(int) %s]", a, idx), nil
			}
			return "", IndexingErr("Java", op.Name)
		}
		if p.Import != "" {
			e.imports[p.Import] = true
		}
		switch {
		// THE WRITE SIDE — ADR 0018.
		case p.Kind == "table-build":
			args := t.Args()
			if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
				return "", fmt.Errorf("build takes a length and (fn (b) …), got %s", t)
			}
			count, err := e.emit(args[0])
			if err != nil {
				return "", err
			}
			body, raw, out := openFresh(args[1], e.bound, javaMangle)
			elem := ElemType(e.tgt, args[1], body, raw[0], e.typeOf, e.sig, e.topParams)
			e.types[raw[0]] = "array " + elem
			e.line("final %s %s = new %s[(int) %s];", e.tgt.ty("array "+elem), out[0],
				e.tgt.ty(elem), count)
			return e.emit(body)
		case p.Kind == "table-set":
			args := t.Args()
			if len(args) != 3 {
				return "", fmt.Errorf("set takes a buffer, an index and a value, got %s", t)
			}
			b, err := e.emit(args[0])
			if err != nil {
				return "", err
			}
			i, err := e.emit(args[1])
			if err != nil {
				return "", err
			}
			v, err := e.emit(args[2])
			if err != nil {
				return "", err
			}
			// A NARROWED SLOT takes a cast on the VALUE. Java widens `short`
			// and `byte` to `long` implicitly on the way out and refuses the
			// way in, so the store is the only direction that needs saying.
			//
			// The index is a separate question with its own cast, and the two
			// are independent: this had the value cast on only one of the two
			// index paths, and the one it missed is the one a program without
			// index narrowing takes — which is every program until the analysis
			// can bound the counter (elemwidth-2026-08-27 §5).
			if spell, ok := e.narrowedSlot(args[0]); ok {
				v = fmt.Sprintf("(%s) %s", spell, v)
			}
			if e.narrowIdx(args[1]) {
				e.line("%s[%s] = %s;", b, i, v)
			} else {
				e.line("%s[(int) %s] = %s;", b, i, v)
			}
			return b, nil
		case p.Kind == "table-alloc":
			args := t.Args()
			if len(args) != 1 {
				return "", fmt.Errorf("alloc takes one table, got %s", t)
			}
			tab := args[0]
			if !isTableRule(e.tgt, tab) {
				return e.emit(tab)
			}
			rule := tab.Args()[1]
			if rule.Kind != core.KFn || len(rule.Params) != 1 {
				return "", fmt.Errorf("alloc's table needs an (fn (i) …) rule, got %s", rule)
			}
			count, err := e.emit(tab.Args()[0])
			if err != nil {
				return "", err
			}
			body, raw, out := openFresh(rule, e.bound, javaMangle)
			e.types[raw[0]] = "int"
			// The fill index is narrow BY CONSTRUCTION. The destination is
			// `new T[(int) n]`, so if n did not fit in a host int the
			// allocation itself would have failed — which makes `j < n` a
			// bound an int can hold, without any analysis.
			if e.narrow == nil {
				e.narrow = map[string]bool{}
			}
			e.narrow[raw[0]] = true
			n := e.fresh("n")
			dst := e.fresh("t")
			e.line("final %s %s = %s;", e.tgt.ty("int"), n, count)
			elem := e.typeOf(body)
			e.types[dst] = "array " + elem
			e.line("final %s %s = new %s[(int) %s];", e.tgt.ty("array "+elem), dst,
				e.tgt.ty(elem), n)
			e.line("for (int %s = 0; %s < %s; %s++) {", out[0], out[0], n, out[0])
			e.indent++
			v, err := e.emit(body)
			if err != nil {
				return "", err
			}
			e.line("%s[%s] = %s;", dst, out[0], v)
			e.indent--
			e.line("}")
			return dst, nil
		// TABLES (docs/spec/tables.md). No target declares any of this.
		case p.Kind == "len":
			a, err := e.emit(t.Args()[0])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s.length", a), nil
		case p.Kind == "array":
			elems := t.Args()
			out := make([]string, len(elems))
			ty := ""
			for i, x := range elems {
				v, err := e.emit(x)
				if err != nil {
					return "", err
				}
				out[i] = v
				if ty == "" {
					ty = e.typeOf(x)
				}
			}
			return fmt.Sprintf("new %s{%s}", e.tgt.ty("array "+ty),
				strings.Join(out, ", ")), nil
		case p.Kind == "table":
			return "", UnallocatedTableErr()
		case p.Kind == "let":
			return e.emitLet(t)
		case p.Kind == "build":
			return e.emitMakeVec(t)
		case p.Kind == "iterate":
			return e.emitLoop(t)
		case p.Kind == "loop":
			return e.emitFoldRange(t)
		case p.Kind == "loop2":
			return e.emitFoldRange2(t)
		case p.Kind == "cond":
			return e.emitIf(t)
		case p.Kind == "stmt":
			args := t.Args()
			vals := make([]any, 0, 2*len(args))
			for _, a := range args {
				v, err := e.emit(a)
				if err != nil {
					return "", err
				}
				vals = append(vals, v)
			}
			if len(args) > 0 && !atomicValue(vals[0].(string)) && strings.Contains(p.Form, "%s") {
				name := javaMangle(e.fresh("v"))
				e.line("final var %s = %s;", name, vals[0])
				vals[0] = name
			}
			e.line("%s;", fill(p.Form, vals))
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
		return "(" + fmt.Sprintf(p.Form, vals...) + ")", nil
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
	// A binder used zero times is a sequencing point rather than a binding — see
	// effects.md §5 and the Go backend's emitLet. Java tolerates an unused local
	// where Go does not, but emitting one would still be noise.
	if !core.Occurs(k.Body(), k.Params[0]) {
		if !emitsStatement(e.tgt, args[0]) && !atomicValue(val) {
			e.line("final var %s = %s;", javaMangle(e.fresh("discard")), val)
		}
		return e.emit(k.Body())
	}
	ty := e.typeOf(args[0])
	kBody, kRaw, kOut := openFresh(k, e.bound, javaMangle)
	e.types[kRaw[0]] = ty
	e.line("final %s %s = %s;", e.tgt.ty(ty), kOut[0], val)
	return e.emit(kBody)
}

// emitIf uses Java's conditional expression when both branches are pure — like
// JS and unlike Go, which has none. So the ANF that g3 §6 derived is required by
// exactly one of the three targets.
// emitConnective emits the host's own operator for a conditional that is one
// of the three boolean connectives (booleans.md §4.4).
func (e *javaEmitter) emitConnective(c Connective) (string, error) {
	vals := make([]string, len(c.Args))
	for i, a := range c.Args {
		v, err := e.emit(a)
		if err != nil {
			return "", err
		}
		vals[i] = v
	}
	switch c.Op {
	case "not":
		return "(!" + vals[0] + ")", nil
	case "and":
		return "(" + vals[0] + " && " + vals[1] + ")", nil
	}
	return "(" + vals[0] + " || " + vals[1] + ")", nil
}

func (e *javaEmitter) emitIf(t *core.Term) (string, error) {
	if c, ok := connective(e.tgt, t); ok {
		return e.emitConnective(c)
	}
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
	if ty == "" {
		ty = e.typeOf(args[2])
	}
	tmp := e.fresh("t")
	e.line("%s %s;", e.tgt.ty(ty), tmp)
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
	accTy := e.typeOf(args[0])

	init, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	count, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	body, raw, out := openFresh(step, e.bound, javaMangle)
	accName, idxName := raw[0], raw[1]
	e.types[accName] = accTy
	e.types[idxName] = "int"
	acc, idx := out[0], out[1]
	n := e.fresh("n")

	e.line("%s %s = %s;", e.tgt.ty(accTy), acc, init)
	e.line("final int %s = %s;", n, count)
	e.line("for (int %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	got, err := e.emit(body)
	if err != nil {
		return "", err
	}
	if got != acc { // a statement-primitive already updated it in place
		e.line("%s = %s;", acc, got)
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
	e.types[sx.Params[0]], e.types[sx.Params[1]], e.types[sx.Params[2]] = "f64", "f64", "int"
	e.types[sy.Params[0]], e.types[sy.Params[1]], e.types[sy.Params[2]] = "f64", "f64", "int"
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
	e.types[fin.Params[0]], e.types[fin.Params[1]] = "f64", "f64"
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
		// `#` starts a name the READER generates and a program cannot write —
		// `match`'s loop variables, `values`' selector. It becomes `_` rather
		// than an escape so the emitted code is readable; a collision with a
		// user's `_m0` is resolved by openFresh like any other.
		case r == '#':
			b.WriteString("_")
		// `-` word break, `.` qualifier, `/` module path separator: all three
		// become camel-case boundaries so a qualified name is one host identifier.
		case r == '-' || r == '.' || r == '/':
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
	if len(JavaRecords) > 0 {
		recs := make([]string, 0, len(JavaRecords))
		for r := range JavaRecords {
			recs = append(recs, r)
		}
		sort.Strings(recs)
		for _, r := range recs {
			out.WriteString(JavaRecords[r])
		}
		out.WriteString("\n")
	}
	for _, n := range names {
		out.WriteString(funcs[n])
		out.WriteString("\n")
	}
	out.WriteString("}\n")
	return out.String()
}

func (e *javaEmitter) emitMakeVec(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("make-vec takes a length and an element function")
	}
	elem := args[1]
	if elem.Kind != core.KFn || len(elem.Params) != 1 {
		return "", fmt.Errorf("make-vec's element function must be (fn (i) ...), got %s", elem)
	}
	elemBody, eRaw, eOut := openFresh(elem, e.bound, javaMangle)
	e.types[eRaw[0]] = "int"
	count, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	n := e.fresh("n")
	dst := e.fresh("v")
	idx := eOut[0]
	// Java array indices are int, not long, because an array cannot exceed
	// 2^31-1 elements -- so the host's own limit decides here, not our int.
	e.line("final int %s = (int) (%s);", n, count)
	e.line("final double[] %s = new double[%s];", dst, n)
	e.line("for (int %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(elemBody)
	if err != nil {
		return "", err
	}
	e.line("%s[%s] = %s;", dst, idx, body)
	e.indent--
	e.line("}")
	e.types[dst] = "vec-f64"
	return dst, nil
}

// ---------------------------------------------------------------- loop

func javaExitType(e *javaEmitter, t *core.Term) string {
	if isAgain(t) {
		return ""
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "let" && len(t.Args()) == 2 {
			if k := t.Args()[1]; k.Kind == core.KFn && len(k.Params) == 1 {
				return javaExitType(e, k.Body())
			}
		}
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			if ty := javaExitType(e, t.Args()[1]); ty != "" {
				return ty
			}
			return javaExitType(e, t.Args()[2])
		}
	}
	return e.typeOf(t)
}

func (e *javaEmitter) emitLoop(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) < 2 || args[0].Kind != core.KFn {
		return "", fmt.Errorf("loop takes (fn (x…) body) and one initial value per variable")
	}
	lam := args[0]
	inits := args[1:]
	if len(lam.Params) != len(inits) {
		return "", fmt.Errorf("loop has %d variable(s) and %d initial value(s)",
			len(lam.Params), len(inits))
	}
	tys := make([]string, len(inits))
	vals := make([]string, len(inits))
	for i, z := range inits {
		tys[i] = e.typeOf(z)
		v, err := e.emit(z)
		if err != nil {
			return "", err
		}
		vals[i] = v
	}
	body, raw, names := openFresh(lam, e.bound, javaMangle)
	for i, n := range raw {
		e.types[n] = tys[i]
	}
	// INDEX-TYPE SELECTION. A loop variable this target can prove small enough
	// is declared as the host's own `int`, so the cast at every access goes
	// away. Nothing about the program changes; only what is emitted does, which
	// is what selection-2026-08-19 established a declared range may do.
	if e.narrow == nil {
		e.narrow = map[string]bool{}
	}
	nw := NarrowIndex(e.tgt, e.fitsIdx, body, raw, inits)
	for i, n := range raw {
		if nw[n] && tys[i] == "int" {
			e.narrow[n] = true
		}
	}
	for i := range names {
		ty := e.tgt.ty(tys[i])
		if e.narrow[raw[i]] {
			ty = "int"
		}
		e.line("%s %s = %s;", ty, names[i], vals[i])
	}
	rty := javaExitType(e, body)
	if rty == "" {
		rty = "any"
	}
	result := soleExit(e.tgt.Prims, body, raw, names, e.bound, javaMangle)
	if result == "" {
		result = javaMangle(e.fresh("r"))
		// THE RESULT VARIABLE NARROWS WITH THE REST. A loop whose exits all fit
		// the host's index type produces a value that fits, and its result is
		// assigned to variables this method has already narrowed — leaving it a
		// `long` puts a wide value into a narrow slot, which javac refuses.
		//
		// Narrowing is a whole-METHOD decision, so this is consistent by
		// construction: either every int local here is the host's own int or
		// none is (indexnarrow-2026-08-27 §2).
		rt := e.tgt.ty(rty)
		if rty == "int" && e.fitsIdx && loopExitsFit(e.tgt, t, raw) {
			rt = "int"
		}
		e.line("%s %s = %s;", rt, result, zeroOf(rt))
	}
	// A uniformly-updated loop variable moves into the `for` statement's post
	// clause — see PostVars. On Go this recovered 1.4x on the sieve, by giving
	// the loop one back edge instead of several.
	post := PostVars(body, raw)
	if len(post) > 0 {
		// ONE ASSIGNMENT PER VARIABLE, not Go's tuple assignment. Go's post
		// clause is `a, b = x, y`, which is a simultaneous assignment; Java's is
		// a comma-separated list of STATEMENTS, so it has to be
		// `a = x, b = y` and each one takes effect before the next is
		// evaluated. This emitter had Go's shape verbatim, and it was a syntax
		// error that nothing reached until a loop had TWO uniformly-updated
		// variables — examples/json/tree.oro's walk, with `seen` and `steps`
		// (json-tree-2026-08-26).
		//
		// Sequential assignment would be wrong if one update read another
		// hoisted variable, and it cannot: PostVars already refuses any update
		// that reads a loop variable other than its own, a condition written to
		// make the Go hoist correct that turns out to be exactly what makes the
		// Java form correct too.
		var ups []string
		for i := range names {
			if u, ok := post[i]; ok {
				v, err := e.emit(u)
				if err != nil {
					return "", err
				}
				ups = append(ups, names[i]+" = "+v)
			}
		}
		e.line("for (;; %s) {", strings.Join(ups, ", "))
	} else {
		e.line("for (;;) {")
	}
	e.indent++
	if err := e.emitLoopBody(body, raw, names, result, post); err != nil {
		return "", err
	}
	e.indent--
	e.line("}")
	e.types[result] = rty
	return result, nil
}

// zeroOf is Java's definite-assignment tax: a local read after a loop must be
// assigned on every path, and javac cannot see that `for (;;)` only exits
// through a `break` that assigns it.
func zeroOf(ty string) string {
	switch ty {
	case "double":
		return "0.0"
	case "long", "int":
		return "0"
	case "boolean":
		return "false"
	default:
		return "null"
	}
}

func (e *javaEmitter) emitLoopBody(t *core.Term, raw, names []string, result string, post map[int]*core.Term) error {
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			cond, err := e.emit(t.Args()[0])
			if err != nil {
				return err
			}
			e.line("if (%s) {", cond)
			e.indent++
			if err := e.emitLoopBody(t.Args()[1], raw, names, result, post); err != nil {
				return err
			}
			e.indent--
			e.line("}")
			return e.emitLoopBody(t.Args()[2], raw, names, result, post)
		}
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "let" && len(t.Args()) == 2 {
			args := t.Args()
			k := args[1]
			if k.Kind == core.KFn && len(k.Params) == 1 {
				val, err := e.emit(args[0])
				if err != nil {
					return err
				}
				if !core.Occurs(k.Body(), k.Params[0]) {
					// A discarded binding of a NAME or a literal is pure noise;
					// only a computation has to be kept, and only if it is not
					// already a statement.
					if !emitsStatement(e.tgt, args[0]) && !atomicValue(val) {
						e.line("final var %s = %s;", javaMangle(e.fresh("discard")), val)
					}
					return e.emitLoopBody(k.Body(), raw, names, result, post)
				}
				ty := e.typeOf(args[0])
				kb, kraw, kout := openFresh(k, e.bound, javaMangle)
				e.types[kraw[0]] = ty
				e.line("final %s %s = %s;", e.tgt.ty(ty), kout[0], val)
				return e.emitLoopBody(kb, raw, names, result, post)
			}
		}
	}
	if isAgain(t) {
		return e.emitAgain(t, raw, names, post)
	}
	v, err := e.emit(t)
	if err != nil {
		return err
	}
	if v != result {
		e.line("%s = %s;", result, v)
	}
	e.line("break;")
	return nil
}

func (e *javaEmitter) emitAgain(t *core.Term, raw, names []string, post map[int]*core.Term) error {
	as := t.Args()
	if len(as) != len(names) {
		return fmt.Errorf("again takes %d argument(s), given %d", len(names), len(as))
	}
	changed := changedArgs(as, raw, post)
	vals := make(map[int]string, len(changed))
	var real []int
	for _, i := range changed {
		v, err := e.emit(as[i])
		if err != nil {
			return err
		}
		if v == names[i] {
			continue // a statement primitive handed the variable back
		}
		// A NARROWED VARIABLE takes a cast on assignment. The value fits — the
		// whole-method gate says every integer operation stays inside a 32-bit
		// index — but Java TYPES the expression, and reading a `long[]` element
		// makes the arithmetic `long` whatever its range. Without this,
		// `acc = (acc + a[i])` with `acc` narrowed is "possible lossy
		// conversion" and javac refuses the file.
		//
		// It appeared the moment the analysis got good enough to narrow a loop
		// that reads a table, which the differential suite caught on the
		// array-literal case (fixpoint-2026-08-27 §11).
		if e.narrow[raw[i]] {
			v = "(int) (" + v + ")"
		}
		vals[i] = v
		real = append(real, i)
	}
	changed = real
	if needTemps(as, raw, changed) {
		tmp := make(map[int]string, len(changed))
		for _, i := range changed {
			tmp[i] = javaMangle(e.fresh("u"))
			e.line("final var %s = %s;", tmp[i], vals[i])
		}
		for _, i := range changed {
			e.line("%s = %s;", names[i], tmp[i])
		}
	} else {
		for _, i := range changed {
			e.line("%s = %s;", names[i], vals[i])
		}
	}
	e.line("continue;")
	return nil
}

// narrowIdx reports whether an index expression is already a host `int`.
//
// A narrowed loop variable is one; so is that variable plus or minus a small
// literal, which is what a stencil writes — `(a (+ j 1))` and `(a (+ j 2))`.
// Java's own arithmetic on two ints is an int, so the whole expression needs no
// cast, and the bound that made `j` narrow covers `j + 2` because the rule
// requires the guard to exit at the length.
func (e *javaEmitter) narrowIdx(t *core.Term) bool {
	if t == nil || e.narrow == nil {
		return false
	}
	switch t.Kind {
	case core.KInt:
		// A literal index needs no cast when it fits: `new short[(int) 32]` is
		// noise, and the value is right there.
		return t.Int >= -2147483648 && t.Int <= 2147483647
	case core.KName:
		return e.narrow[t.Name]
	case core.KApp:
		if t.Op().Kind != core.KName {
			return false
		}
		// A CONDITIONAL index is narrow when both branches are — the same rule
		// every other part of this analysis uses for `if`, and the shape a
		// clamped stack index takes: `(if (< sp 1) 0 (- sp 1))`.
		if p, known := e.tgt.Prims[t.Op().Name]; known &&
			p.Kind == "cond" && len(t.Args()) == 3 {
			return e.narrowIdx(t.Args()[1]) && e.narrowIdx(t.Args()[2])
		}
		if len(t.Args()) != 2 {
			return false
		}
		if !isOp(t.Op().Name, "add") && !isOp(t.Op().Name, "sub") {
			return false
		}
		a, b := t.Args()[0], t.Args()[1]
		return e.narrowIdx(a) && b.Kind == core.KInt
	}
	return false
}

// buildType is the type of a `(build n (fn (b) …))` — its BODY's type, which is
// not necessarily the buffer's. See the Go emitter for what found that.
func (e *javaEmitter) buildType(lam *core.Term) string {
	body, raw, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
	elem := ElemType(e.tgt, lam, body, raw[0], e.typeOf, e.sig, e.topParams)
	saved, had := e.types[raw[0]], false
	if _, ok := e.types[raw[0]]; ok {
		had = true
	}
	e.types[raw[0]] = "array " + elem
	ty := e.typeOf(body)
	if had {
		e.types[raw[0]] = saved
	} else {
		delete(e.types, raw[0])
	}
	if ty == "" {
		return "array " + elem
	}
	return ty
}

// narrowedSlot reports the host spelling of a table's element when it is
// narrower than the target's own integer. See the Go emitter.
func (e *javaEmitter) narrowedSlot(tab *core.Term) (string, bool) {
	root := BufferRoot(tab)
	if root == "" {
		return "", false
	}
	return e.tgt.NarrowedElem(e.types[root])
}
