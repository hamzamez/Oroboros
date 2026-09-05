package emit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"oroboros/core"
)

// JavaScript backend.
//
// Written standalone rather than factored against the Go backend on purpose:
// what the two turn out to share is the finding, and designing the shared
// interface first would have assumed the answer. See
// gauntlet/results/js-2026-08-14.md for what the duplication actually was.
//
// JS is the hostile host of ADR 0004 — no integers, no structs, no int64 — and
// the point of building it second is to find out which parts of emit/golang.go
// were general and which were Go-shaped assumptions.

type jsEmitter struct {
	tgt    *Target

	// bufReuse and spareOf are LoopBufferReuse's answer: a back-edge `build`
	// writes into storage the loop already owns. Keyed by the build's lambda
	// and by the loop variable's index — see the Go emitter for the rule and
	// for why it is two buffers and a swap rather than one in place.
	bufReuse map[*core.Term]string
	spareOf  map[int]string
	buf    strings.Builder
	tmp    int
	indent int

	// bound is every name already emitted in this function — see openFresh.
	bound map[string]bool

	// maps is which emitted names hold a MAP, which JavaScript needs and the
	// other backends do not: `targets/js` declares no types on purpose, so
	// there is no `typeOf` to ask whether `(m k)` is a map read or an array
	// read, and the two lower to different code.
	//
	// It is populated at `build-map` and propagated through `let`, and it is
	// deliberately CONSERVATIVE: a name it does not know falls through to the
	// existing diagnostic rather than to a guess, so it can under-fire but
	// never produce a wrong answer.
	maps map[string]bool

	// tail is true while emitting a term whose value IS the function's value.
	// A loop in that position returns directly instead of assigning a result
	// variable and breaking, and `returned` records that it did so the wrapper
	// omits its own `return`.
	//
	// A flag rather than the term itself: `Term.Body()` opens de Bruijn
	// indices into names and ALLOCATES, so two calls to it are different
	// pointers and identity cannot be used to recognise a position.
	//
	// Measured, not tidied. On V8 the same search loop written with an early
	// `return` runs at 36,973 ns and written with a result variable and
	// `break` at 48,383 — 1.31x for the shape alone. On Go the two are
	// indistinguishable, which is why this lives here and not in the shared
	// path: JS is the hostile host and this is the sort of thing it exists to
	// surface (native-js-2026-08-20).
	tail     bool
	returned bool
}

// JSFunc emits a top-level abstraction as a JavaScript function.
//
// Note what is absent compared with Func: there is no type lattice, no
// inference, and no way for a parameter to fail to have a type. Everything
// emit/golang.go does with Ty exists to satisfy Go, not the language.
// The signature is accepted and unused: JavaScript needs no parameter types,
// which is the measurement targets/js.oro records by declaring none.
func JSFunc(tgt *Target, name string, sig *core.Sig, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &jsEmitter{tgt: tgt, indent: 1, bound: map[string]bool{}, maps: map[string]bool{}}
	// The PARAMETERS are already bound, and forgetting them is not a cosmetic
	// bug on this host: `(loop ((n n)) …)` emitted `let n = n;` inside
	// `function f(n)`, which is a SyntaxError — the module does not parse.
	// Go and Java seeded this from the start; JavaScript did not, and nothing
	// noticed until `match` made reusing a parameter name the common case.
	for _, p := range t.Params {
		e.bound[jsMangle(p)] = true
	}

	// SEVERAL RESULTS. This is the ONE place JSFunc reads the signature, and
	// the reason is that JavaScript has no multiple return: the arity has to
	// come from somewhere and the host cannot supply it. Everywhere else JS
	// needs no types, which is the measurement targets/js.oro records by
	// declaring none.
	if sig != nil && len(sig.Results) > 1 {
		return e.multiFunc(name, sig, t)
	}

	e.tail = true
	result, err := e.emit(t.Body())
	if err != nil {
		return "", err
	}
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = jsMangle(p)
	}

	var out strings.Builder
	fmt.Fprintf(&out, "export function %s(%s) {\n", jsMangle(name), strings.Join(params, ", "))
	out.WriteString(e.buf.String())
	if e.returned {
		// A loop in tail position returned from every exit, so there is no
		// value left to return and no reachable statement after the loop.
		out.WriteString("}\n")
	} else {
		fmt.Fprintf(&out, "\treturn %s;\n}\n", result)
	}
	return out.String(), nil
}

// multiFunc emits a function with several results.
//
// JavaScript has no native multiple return, so the language construct is built
// out of what the host has — which is the compiler's job, not a target author's.
// An OBJECT LITERAL with a fixed shape, measured rather than chosen for how it
// reads (mrshape-2026-08-22):
//
//	                   caller uses p.f0   caller destructures
//	return [a, b]           8,348 ns            955 ns
//	return {f0, f1}         5,164 ns            956 ns
//	no product at all            —              940 ns
//
// The object is 1.62x faster when the caller reads a property and identical
// when it destructures, so it is better or equal in both. The first version
// emitted an array because `const [a, b] = f()` reads well — clarity over
// requirement 5, argued from a measurement of a different shape.
//
// And the larger finding, which belongs with the CALLER rather than here:
// destructuring at the call site costs nothing, because V8 scalar-replaces the
// object entirely and lands on the no-product number. Binding it and reading
// fields keeps the allocation, at 5.4x.
func (e *jsEmitter) multiFunc(name string, sig *core.Sig, t *core.Term) (string, error) {
	params := make([]string, len(t.Params))
	for i, p := range t.Params {
		params[i] = jsMangle(p)
		e.bound[params[i]] = true
	}
	if err := e.multiTail(t.Body(), len(sig.Results), name, sig); err != nil {
		return "", err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "export function %s(%s) {\n", jsMangle(name), strings.Join(params, ", "))
	out.WriteString(e.buf.String())
	out.WriteString("}\n")
	return out.String(), nil
}

// multiTail returns from every leaf rather than joining the branches first.
// See the Go emitter for the argument; on V8 it is also worth 1.31x over a
// result variable (native-js-2026-08-20).
func (e *jsEmitter) multiTail(t *core.Term, n int, name string, sig *core.Sig) error {
	if vs, ok := multiValue(t, n); ok {
		fields := make([]string, len(vs))
		for i, v := range vs {
			x, err := e.emit(v)
			if err != nil {
				return err
			}
			fields[i] = fmt.Sprintf("f%d: %s", i, x)
		}
		// An OBJECT, not an array: 1.62x faster when the caller reads a
		// property and identical when it destructures (multiresult-2026-08-22).
		e.line("return {%s};", strings.Join(fields, ", "))
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
				body, _, out := openFresh(k, e.bound, jsMangle)
				e.line("let %s = %s;", out[0], val)
				return e.multiTail(body, n, name, sig)
			}
		}
	}
	return multiResultErr(name, sig, t)
}

// isMapTerm reports whether a term produces a MAP, conservatively.
//
// Two sources and no inference: a `build-map`, and a surviving map literal —
// which survives exactly when the key is dynamic, because a literal key lets
// beta-tab decide the domain condition and the whole thing folds.
func (e *jsEmitter) isMapTerm(t *core.Term) bool {
	if t == nil || t.Kind != core.KApp {
		return false
	}
	op := t.Op()
	if op.Kind != core.KName {
		return false
	}
	if p, ok := e.tgt.Prims[op.Name]; ok {
		if p.Kind == "map-build" || p.Kind == "map" {
			return true
		}
		// `insert` hands the map back, which is what makes it thread.
		if p.Kind == "map-insert" && len(t.Args()) > 0 {
			return e.isMapName(t.Args()[0]) || e.isMapTerm(t.Args()[0])
		}
	}
	return false
}

func (e *jsEmitter) isMapName(t *core.Term) bool {
	return t != nil && t.Kind == core.KName && e.maps[jsMangle(t.Name)]
}

// emitMapCase is the Go emitter's, in JavaScript's own idiom.
//
// A PLAIN OBJECT, not `Map`, and that is measured rather than assumed:
// maps-2026-08-30 re-took the first baseline's 3.25x and found 1.56x on string
// keys and 3.67x on INTEGER keys — more than twice the string gap, because V8
// keeps integer-like properties in the elements backing store rather than a
// hash. `(map int V)` is therefore the case where the host choice matters most,
// which is the opposite of how it was framed when it was picked to dodge
// strings. A plain `{}` also beat `Object.create(null)`, against the folklore.
//
// Absence is `undefined`, and that is SOUND rather than convenient: every value
// in a map came from an `insert` of a term the checker typed, and no type in
// the language has `undefined` as a value. So `=== undefined` distinguishes
// absent from present exactly. It is one lookup where `k in m` plus `m[k]`
// would be two.
func (e *jsEmitter) emitMapCase(t *core.Term) (string, bool, error) {
	op := t.Op()
	args := t.Args()
	if op.Kind != core.KApp || len(args) != 1 || len(op.Args()) != 1 {
		return "", false, nil
	}
	k := args[0]
	if k.Kind != core.KFn || len(k.Params) != 2 {
		return "", false, nil
	}
	inner := op.Op()
	if !e.isMapName(inner) && !e.isMapTerm(inner) {
		return "", false, nil
	}
	m, err := e.emit(inner)
	if err != nil {
		return "", false, err
	}
	key, err := e.emit(op.Args()[0])
	if err != nil {
		return "", false, err
	}
	body, _, out := openFresh(k, e.bound, jsMangle)
	e.line("const %s = %s[%s];", out[1], m, key)
	e.line("const %s = %s === undefined ? 1 : 0;", out[0], out[1])
	s, err := e.emit(body)
	return s, true, err
}

func (e *jsEmitter) line(format string, args ...any) {
	e.buf.WriteString(strings.Repeat("\t", e.indent))
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *jsEmitter) fresh(stem string) string {
	for {
		e.tmp++
		n := fmt.Sprintf("%s%d", stem, e.tmp)
		if !e.bound[n] {
			e.bound[n] = true
			return n
		}
	}
}

func (e *jsEmitter) emit(t *core.Term) (string, error) {
	// Tail position is consumed here, so no nested emit inherits it by
	// accident. The two constructs that genuinely preserve it — a `let`
	// continuation and the loop itself — put it back explicitly.
	tail := e.tail
	e.tail = false
	switch t.Kind {
	case core.KInt:
		return strconv.FormatInt(t.Int, 10), nil

	case core.KFloat:
		// JS has one number type, so a float literal needs no decorating —
		// unlike Go, where 1 and 1.0 are different tokens.
		return strconv.FormatFloat(t.Float, 'g', -1, 64), nil

	case core.KBool:
		if t.IsTrue() {
			return "true", nil
		}
		return "false", nil

	case core.KStr:
		// docs/spec/string-literals.md §6: every scalar value gets a
		// spelling this host has, and the output is ASCII only. See
		// emit/strlit.go for what `strconv.Quote` got wrong here, and for
		// the measurement that found it.
		return UTF16StringLit(t.Str), nil

	case core.KName:
		return jsMangle(t.Name), nil

	case core.KFn:
		return "", fmt.Errorf("a bare abstraction reached the emitter: %s\n"+
			"  This is an escaping closure. JS has first-class functions and could\n"+
			"  emit one directly, but g6's cost model has not been checked here.", t)

	case core.KApp:
		op := t.Op()
		if out, done, err := e.emitMapCase(t); err != nil || done {
			return out, err
		}
		if op.Kind != core.KName {
			return "", fmt.Errorf("application of a non-name: %s", t)
		}
		p, ok := e.tgt.Prims[op.Name]
		if !ok {
			if IsTableOperand(jsMangle(op.Name), e.bound) && len(t.Args()) == 1 {
				a, err := e.emit(op)
				if err != nil {
					return "", err
				}
				idx, err := e.emit(t.Args()[0])
				if err != nil {
					return "", err
				}
				return fmt.Sprintf("%s[%s]", a, idx), nil
			}
			return "", IndexingErr("JavaScript", op.Name)
		}
		if p.Kind == "let" {
			args := t.Args()
			k := args[1]
			if k.Kind != core.KFn || len(k.Params) != 1 {
				return "", fmt.Errorf("let's continuation must be (fn (x) …), got %s", k)
			}
			val, err := e.emit(args[0])
			if err != nil {
				return "", err
			}
			// A binder used zero times is a sequencing point rather than a
			// binding — see effects.md §5 and the Go backend's emitLet.
			if !core.Occurs(k.Body(), k.Params[0]) {
				if !emitsStatement(e.tgt, args[0]) && !atomicValue(val) {
					e.line("%s;", val)
				}
				e.tail = tail
				return e.emit(k.Body())
			}
			kBody, _, kOut := openFresh(k, e.bound, jsMangle)
			// The value's own map-ness, and also the emitted NAME's: a `let`
			// bound to a loop gets the loop's result variable, which was
			// already marked when the loop's variables were.
			if e.isMapTerm(args[0]) || e.isMapName(args[0]) || e.maps[val] {
				e.maps[kOut[0]] = true
			}
			e.line("const %s = %s;", kOut[0], val)
			e.tail = tail
			return e.emit(kBody)
		}
		if p.Kind == "stmt" {
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
				name := e.fresh("v")
				e.line("const %s = %s;", name, vals[0])
				vals[0] = name
			}
			e.line("%s;", fill(p.Form, vals))
			return vals[0].(string), nil
		}
		if p.Kind == "build" {
			return e.emitMakeVec(t)
		}
		if p.Kind == "iterate" {
			return e.emitLoop(t, tail)
		}
		if p.Kind == "loop" {
			return e.emitFoldRange(t)
		}
		if p.Kind == "loop2" {
			return e.emitFoldRange2(t)
		}
		// TABLES (docs/spec/tables.md). No target declares any of this; the
		// backends implement it exactly like `if`, `let` and `loop`.
		//
		// A surviving `(table n f)` is a rule with NO MEMORY, so there is
		// nothing to emit — it has to be `(alloc …)`ed first. That refusal is
		// the construct doing its job: the rule form exists to FUSE, and one
		// that reaches a backend did not.
		// THE WRITE SIDE — ADR 0018. The buffer is linear and scoped, so the
		// freeze on the way out copies nothing.
		switch p.Kind {
		case "map-keys":
			args := t.Args()
			if len(args) != 1 {
				return "", fmt.Errorf("keys takes a map, got %s", t)
			}
			m, err := e.emit(args[0])
			if err != nil {
				return "", err
			}
			// `Object.keys` returns STRINGS — a JavaScript object's keys always
			// are, whatever was written — so they are converted back before
			// sorting. And the comparator is REQUIRED: `Array.sort` with no
			// comparator sorts lexicographically, so [2,10] would come back
			// [10,2]. That is the same shape of host default that
			// `split-words` was, and it is silent.
			out := jsMangle(e.fresh("ks"))
			e.line("const %s = Object.keys(%s).map(Number).sort((a, b) => a - b);", out, m)
			return out, nil
		case "map-build":
			args := t.Args()
			if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
				return "", fmt.Errorf("build-map takes a capacity and (fn (m) …), got %s", t)
			}
			body, _, out := openFresh(args[1], e.bound, jsMangle)
			// A PLAIN OBJECT, and the capacity is DROPPED here rather than
			// ignored: JavaScript objects have no capacity to give, and the
			// declaration exists so that four targets agree on what a program
			// means, not because every host needs the number (maps.md §6).
			// windows is the target that cannot grow, and it is the reason.
			e.line("const %s = {};", out[0])
			e.maps[out[0]] = true
			return e.emit(body)
		case "map-insert":
			args := t.Args()
			if len(args) != 3 {
				return "", fmt.Errorf("insert takes a map, a key and a value, got %s", t)
			}
			m, err := e.emit(args[0])
			if err != nil {
				return "", err
			}
			k, err := e.emit(args[1])
			if err != nil {
				return "", err
			}
			v, err := e.emit(args[2])
			if err != nil {
				return "", err
			}
			e.maps[m] = true
			e.line("%s[%s] = %s;", m, k, v)
			return m, nil
		case "map":
			rows := t.Args()
			parts := make([]string, len(rows))
			for i, row := range rows {
				if row.Kind != core.KApp || len(row.Kids) != 2 {
					return "", fmt.Errorf("a map literal row is (key value), got %s", row)
				}
				k, err := e.emit(row.Kids[0])
				if err != nil {
					return "", err
				}
				v, err := e.emit(row.Kids[1])
				if err != nil {
					return "", err
				}
				parts[i] = k + ": " + v
			}
			return "{" + strings.Join(parts, ", ") + "}", nil
		case "table-build":
			args := t.Args()
			if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
				return "", fmt.Errorf("build takes a length and (fn (b) …), got %s", t)
			}
			count, err := e.emit(args[0])
			if err != nil {
				return "", err
			}
			body, _, out := openFresh(args[1], e.bound, jsMangle)
			// `new Array(n).fill(…)` rather than a bare `new Array(n)`: a
			// sparse array on V8 is a dictionary, and every store into one is a
			// map insert — which is `js.set`'s refusal (native-gauntlet §…)
			// arriving here. Filling makes it a packed elements array.
			// JavaScript declares no types, so there is nothing to read an
			// element type off — and nothing that needs one. Zero fills both
			// numeric and boolean buffers usefully enough, and what matters is
			// that the array is PACKED rather than sparse.
			// A BACK-EDGE BUILD WRITES INTO THE LOOP'S SPARE. `fill(0)` restores
			// the zero fill `build` guarantees (tables.md §14.3) and keeps the
			// array PACKED, which is the same reason the fresh one is filled.
			if sp, ok := e.bufReuse[args[1]]; ok {
				e.line("%s.fill(0);", sp)
				e.line("const %s = %s;", out[0], sp)
				return e.emit(body)
			}
			e.line("const %s = new Array(%s).fill(0);", out[0], count)
			return e.emit(body)
		case "table-set":
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
			e.line("%s[%s] = %s;", b, i, v)
			return b, nil
		case "table-alloc":
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
			body, _, out := openFresh(rule, e.bound, jsMangle)
			n := e.fresh("n")
			dst := e.fresh("t")
			e.line("const %s = %s;", n, count)
			e.line("const %s = new Array(%s).fill(0);", dst, n)
			e.line("for (let %s = 0; %s < %s; %s++) {", out[0], out[0], n, out[0])
			e.indent++
			v, err := e.emit(body)
			if err != nil {
				return "", err
			}
			e.line("%s[%s] = %s;", dst, out[0], v)
			e.indent--
			e.line("}")
			return dst, nil
		case "len":
			a, err := e.emit(t.Args()[0])
			if err != nil {
				return "", err
			}
			// A MAP'S LENGTH IS |dom m|, NOT `.length`. A map lowers to a plain
			// object here, and `{}.length` is `undefined` — a silent wrong
			// answer that compiled, ran and printed. `len m = len (keys m)` is
			// the cardinality of the index set and every table has one, so this
			// is the same construct with the same meaning at a different index
			// set, not a special case.
			//
			// O(n) where Go and Java are O(1). The ANSWER is identical, so it
			// is not a Tier 2 disagreement — only a different price, which
			// maps.md §8.2 names rather than hides.
			if e.isMapName(t.Args()[0]) || e.isMapTerm(t.Args()[0]) || e.maps[a] {
				return fmt.Sprintf("Object.keys(%s).length", a), nil
			}
			return fmt.Sprintf("%s.length", a), nil
		case "array":
			elems := t.Args()
			out := make([]string, len(elems))
			for i, x := range elems {
				v, err := e.emit(x)
				if err != nil {
					return "", err
				}
				out[i] = v
			}
			return "[" + strings.Join(out, ", ") + "]", nil
		case "table":
			return "", UnallocatedTableErr()
		}
		if p.Kind == "cond" {
			return e.emitIf(t)
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

// emitIf uses a conditional *expression* when both branches are pure, and falls
// back to a statement otherwise.
//
// This is the first real divergence from the Go backend: Go has no conditional
// expression, so emit/golang.go must always introduce a temporary. JS does, so
// the ANF that g3 §6 and g5 §4 both derived as necessary is **target-dependent**,
// not a property of the language.
// emitConnective emits the host's own operator for a conditional that is one
// of the three boolean connectives (booleans.md §4.4).
func (e *jsEmitter) emitConnective(c Connective) (string, error) {
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

func (e *jsEmitter) emitIf(t *core.Term) (string, error) {
	if c, ok := connective(e.tgt, t); ok {
		return e.emitConnective(c)
	}
	args := t.Args()
	if len(args) != 3 {
		return "", fmt.Errorf("if takes a condition and two branches")
	}
	cond, err := e.emit(args[0])
	if err != nil {
		return "", err
	}

	// Emit both branches into a scratch buffer to see whether either needs
	// statements. If neither does, a ternary suffices.
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

	tmp := e.fresh("t")
	e.line("let %s;", tmp)
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

// emitFoldRange2 carries two accumulators. Go has tuple assignment so it can
// write `ax, ay = …, …` directly; JS destructuring allocates an array, so the
// simultaneous update needs temporaries. That is g2 §6's parallel-assignment
// discipline, and the two targets need different code for it.
func (e *jsEmitter) emitFoldRange2(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 6 {
		return "", fmt.Errorf("fold-range2 takes x0, y0, count, stepX, stepY, finish")
	}
	sx, sy, fin := args[3], args[4], args[5]
	if fin.Kind != core.KFn || len(fin.Params) != 2 {
		return "", fmt.Errorf("fold-range2's finisher must be (fn (ax ay) …), got %s", fin)
	}
	for _, s := range []*core.Term{sx, sy} {
		if s.Kind != core.KFn || len(s.Params) != 3 {
			return "", fmt.Errorf("fold-range2 steps must be (fn (ax ay i) …), got %s", s)
		}
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
	ax, ay, idx := jsMangle(sx.Params[0]), jsMangle(sx.Params[1]), jsMangle(sx.Params[2])
	n := e.fresh("n")

	e.line("let %s = %s, %s = %s;", ax, x0, ay, y0)
	e.line("const %s = %s;", n, count)
	e.line("for (let %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
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
	tx, ty := e.fresh("u"), e.fresh("u")
	e.line("const %s = %s, %s = %s;", tx, bx, ty, by)
	e.line("%s = %s; %s = %s;", ax, tx, ay, ty)
	e.indent--
	e.line("}")
	return e.emit(core.Rename(fin.Body(), map[string]string{
		fin.Params[0]: sx.Params[0], fin.Params[1]: sx.Params[1]}))
}

func (e *jsEmitter) emitFoldRange(t *core.Term) (string, error) {
	args := t.Args()
	step := args[2]
	if step.Kind != core.KFn || len(step.Params) != 2 {
		return "", fmt.Errorf("fold-range's third argument must be (fn (acc i) …), got %s", step)
	}
	init, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	count, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	body, _, out := openFresh(step, e.bound, jsMangle)
	acc, idx := out[0], out[1]
	n := e.fresh("n")

	e.line("let %s = %s;", acc, init)
	e.line("const %s = %s;", n, count)
	e.line("for (let %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
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

// jsMangle differs from mangle only in its keyword set and in permitting `$`.
// The transformation itself — hyphen to camelCase, ? and ! to letters — is
// identical, which is evidence it belongs to the *language* rather than to
// either target.
func jsMangle(s string) string {
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
	if jsKeywords[out] {
		out += "_"
	}
	return out
}

var jsKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`await break case catch class const continue debugger
		default delete do else enum export extends false finally for function if implements
		import in instanceof interface let new null package private protected public return
		static super switch this throw true try typeof var void while with yield`) {
		jsKeywords[k] = true
	}
}

// JSFile wraps emitted functions in an ES module.
func JSFile(funcs map[string]string) string {
	names := make([]string, 0, len(funcs))
	for n := range funcs {
		names = append(names, n)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\n")
	for _, n := range names {
		out.WriteString(funcs[n])
		out.WriteString("\n")
	}
	return out.String()
}

func (e *jsEmitter) emitMakeVec(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("make-vec takes a length and an element function")
	}
	elem := args[1]
	if elem.Kind != core.KFn || len(elem.Params) != 1 {
		return "", fmt.Errorf("make-vec's element function must be (fn (i) ...), got %s", elem)
	}
	count, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	n := e.fresh("n")
	dst := e.fresh("v")
	elemBody, _, eOut := openFresh(elem, e.bound, jsMangle)
	idx := eOut[0]
	e.line("const %s = %s;", n, count)
	e.line("const %s = new Array(%s);", dst, n)
	e.line("for (let %s = 0; %s < %s; %s++) {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(elemBody)
	if err != nil {
		return "", err
	}
	e.line("%s[%s] = %s;", dst, idx, body)
	e.indent--
	e.line("}")
	return dst, nil
}

// ---------------------------------------------------------------- loop
//
// (loop (fn (x…) body) z…) — docs/spec/iteration.md. Identical in shape to the
// Go emitter, minus the types, plus temporaries: JavaScript has no parallel
// assignment, so a simultaneous update needs them — the same ones fold-range2
// already emits, and measured free.

func (e *jsEmitter) emitLoop(t *core.Term, tail bool) (string, error) {
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
	vals := make([]string, len(inits))
	for i, z := range inits {
		v, err := e.emit(z)
		if err != nil {
			return "", err
		}
		vals[i] = v
	}
	body, raw, names := openFresh(lam, e.bound, jsMangle)
	for i := range names {
		// A LOOP VARIABLE INHERITS map-ness from its init, which is how a map
		// threaded through a loop stays identifiable. Without it the tracking
		// depends on some `insert` in the body happening to mark the name, so
		// a loop that only READS a map — or one whose map is empty — would
		// fall through to "application of a non-name".
		if e.isMapName(inits[i]) || e.isMapTerm(inits[i]) {
			e.maps[names[i]] = true
		}
		e.line("let %s = %s;", names[i], vals[i])
	}
	// THE SPARE BUFFERS, one per reusable loop variable, allocated once outside
	// the loop and alternated with it. Scoped, because a spare belongs to ONE
	// loop and it is that loop's `again` that swaps it.
	outerSpares := e.spareOf
	spares := map[int]string{}
	for j, blam := range LoopBufferReuse(e.tgt, inits, body, raw) {
		sp := e.fresh("sp")
		e.line("let %s = new Array(%d).fill(0);", sp, buildLen(e.tgt, inits[j]))
		spares[j] = sp
		if e.bufReuse == nil {
			e.bufReuse = map[*core.Term]string{}
		}
		e.bufReuse[blam] = sp
	}
	e.spareOf = spares
	defer func() { e.spareOf = outerSpares }()
	result := ""
	if !tail {
		result = soleExit(e.tgt.Prims, body, raw, names, e.bound, jsMangle)
		if result == "" {
			result = e.fresh("r")
			e.line("let %s;", result)
		}
	}
	// A uniformly-updated loop variable moves into the `for` statement's post
	// clause — see PostVars. On Go this recovered 1.4x on the sieve, by giving
	// the loop one back edge instead of several.
	post := PostVars(body, raw)
	if len(post) > 0 {
		// ONE ASSIGNMENT PER VARIABLE. This emitter had Go's tuple assignment
		// verbatim — `a, b = x, y` — and in JavaScript that is the COMMA
		// OPERATOR: it evaluates `a`, assigns `b = x`, and throws `y` away. So
		// the first variable never updated and the second got the wrong value,
		// with no syntax error anywhere.
		//
		// The same emitted shape was a compile error on Java and a SILENT WRONG
		// ANSWER here, which is the whole argument for the differential suite in
		// one bug (json-tree-2026-08-26). It was latent because a single
		// hoisted variable emits `a = x`, which is correct on all three hosts,
		// and no program had two until the tree walk's `seen` and `steps`.
		//
		// Sequential assignment is safe because PostVars refuses any update that
		// reads a loop variable other than its own.
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
	if tail {
		e.returned = true
	}
	return result, nil
}

func (e *jsEmitter) emitLoopBody(t *core.Term, raw, names []string, result string, post map[int]*core.Term) error {
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
					if !emitsStatement(e.tgt, args[0]) {
						e.line("%s;", val)
					}
					return e.emitLoopBody(k.Body(), raw, names, result, post)
				}
				kb, _, kout := openFresh(k, e.bound, jsMangle)
				e.line("const %s = %s;", kout[0], val)
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
	// An empty result means this loop is the function's value: return from the
	// loop rather than assigning and breaking. Worth 1.31x on V8 for a loop
	// with two exits, and nothing at all on Go.
	if result == "" {
		e.line("return %s;", v)
		return nil
	}
	if v != result {
		e.line("%s = %s;", result, v)
	}
	e.line("break;")
	return nil
}

// emitAgain: skip unchanged variables, and use temporaries only when a changed
// one is read by another changed one — otherwise sequential assignment is
// already simultaneous.
func (e *jsEmitter) emitAgain(t *core.Term, raw, names []string, post map[int]*core.Term) error {
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
		vals[i] = v
		real = append(real, i)
	}
	changed = real
	// THE SWAP FOR A REUSED BUFFER, and it needs its own temporary whatever
	// `needTemps` says: `sp` takes the variable's OLD value, which the
	// assignment below is about to overwrite. JavaScript has no simultaneous
	// assignment — the comma operator is not one, which this emitter has
	// already been caught by once — so the old value is named first.
	for _, i := range changed {
		if sp, ok := e.spareOf[i]; ok {
			old := e.fresh("sw")
			e.line("const %s = %s;", old, names[i])
			e.line("%s = %s;", names[i], vals[i])
			e.line("%s = %s;", sp, old)
			delete(vals, i)
		}
	}
	var rest []int
	for _, i := range changed {
		if _, done := vals[i]; done {
			rest = append(rest, i)
		}
	}
	changed = rest
	if needTemps(as, raw, changed) {
		tmp := make(map[int]string, len(changed))
		for _, i := range changed {
			tmp[i] = e.fresh("u")
			e.line("const %s = %s;", tmp[i], vals[i])
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
