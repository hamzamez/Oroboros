// Package emit turns a normal form into target source.
//
// This is the first code that produces output measurable against the gauntlet.
// Everything before it measured hand-written host code — the bar — and never
// what we produce.
//
// Deliberately narrow: it handles the residual we actually have, and the shape
// of what it needs is the finding. In particular the primitive table below is
// hardcoded, and what a *general* binding format must carry should be read off
// from what this file turned out to need, rather than designed in advance.
package emit

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"oroboros/core"
)

// ---------------------------------------------------------------- types
//
// A four-point lattice, not a type system. The language stays untyped; Go needs
// types, so the backend works them out. Nothing here is visible in the source
// language, which is the point — the emitter must not push types up into the
// core before we know we need them.

// prim is a primitive's Go form. Format uses {0}, {1}, … for operands.

// ---------------------------------------------------------------- emitter

type Emitter struct {
	tgt     *Target
	buf     strings.Builder
	imports map[string]bool
	types   map[string]string // variable -> inferred type
	weak    map[string]string // variable -> `any`, used only if nothing else fits
	tmp     int
	indent  int

	// sig and topParams are the enclosing function's contract and the names
	// this backend opened its parameters with. A `build` buffer's stores are
	// usually bounded by something the signature says, and the sub-pass that
	// works out the element range needs both to see it.
	sig       *core.Sig
	topParams []string

	// bound is every name already emitted in this function. A binder whose
	// hint collides with one gets a fresh one — see openFresh.
	bound map[string]bool
}

// Func emits a top-level abstraction as a Go function.
func Func(tgt *Target, name string, sig *core.Sig, t *core.Term) (string, error) {
	if t.Kind != core.KFn {
		return "", fmt.Errorf("top level must be an abstraction, got %s", t)
	}
	e := &Emitter{tgt: tgt, types: map[string]string{}, weak: map[string]string{},
		imports: map[string]bool{}, bound: map[string]bool{}}
	e.sig, e.topParams = sig, t.Params
	for _, p := range t.Params {
		e.bound[mangle(p)] = true
	}

	// Parameter types come from the declared signature first, then from how the
	// body uses them. Local propagation from primitive signatures — not
	// inference, just reading the table.
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
			return "", fmt.Errorf("cannot determine a Go type for parameter %q; "+
				"it is never passed to a primitive whose signature would fix it", p)
		}
		params[i] = mangle(p) + " " + e.tgt.ty(ty)
	}

	// SEVERAL RESULTS — the negative product reaching a boundary. The residual
	// must be `(values e₁ … eₙ)`, which is `(fn (k) (k e₁ … eₙ))`, and Go
	// returns them in registers.
	if sig != nil && len(sig.Results) > 1 {
		return e.multiFunc(name, sig, t, params)
	}

	var body strings.Builder
	e.buf = body
	e.indent = 1
	result, err := e.emit(t.Body())
	if err != nil {
		return "", err
	}
	inner := e.buf.String()

	var out strings.Builder
	fmt.Fprintf(&out, "func %s(%s) %s {\n", export(name), strings.Join(params, ", "),
		e.tgt.ty(e.typeOf(t.Body())))
	out.WriteString(inner)
	fmt.Fprintf(&out, "\treturn %s\n}\n", result)
	for imp := range e.imports {
		Imports[imp] = true
	}
	return out.String(), nil
}

// multiFunc emits a function with several results. Go returns them in
// registers, so there is no product to build and nothing to allocate — which
// is the whole reason the negative product measured 1.01x
// (product-2026-08-19) and the reason `values` is not a tuple.
func (e *Emitter) multiFunc(name string, sig *core.Sig, t *core.Term, params []string) (string, error) {
	var body strings.Builder
	e.buf = body
	e.indent = 1
	if err := e.multiTail(t.Body(), len(sig.Results), name, sig); err != nil {
		return "", err
	}
	tys := make([]string, len(sig.Results))
	for i, r := range sig.Results {
		tys[i] = e.tgt.ty(r)
	}
	// Go has multiple return natively, in registers. Nothing is built.
	var out strings.Builder
	fmt.Fprintf(&out, "func %s(%s) (%s) {\n", export(name), strings.Join(params, ", "),
		strings.Join(tys, ", "))
	out.WriteString(e.buf.String())
	out.WriteString("}\n")
	for imp := range e.imports {
		Imports[imp] = true
	}
	return out.String(), nil
}

// multiTail emits a function body whose value is several results, RETURNING
// from every leaf rather than joining the branches first.
//
// Sums are what demanded it. A function returning a sum returns from several
// places -- `(if (= b 0) (err 0) (ok (go./ a b)))` has a product in each branch
// -- and the single-value path would have built a temporary, assigned it in
// both branches and returned it at the end. Returning at the leaf is what a Go
// programmer writes, and it is the same finding native-js-2026-08-20 measured
// on V8, where a tail `return` beat a result variable by 1.31x.
//
// The forms walked are `if` and `let`, which is not a coincidence: they are
// exactly what beta can leave between a function and its result, and exactly
// what the commuting conversion in core/reduce.go pushes an eliminator through.
func (e *Emitter) multiTail(t *core.Term, n int, name string, sig *core.Sig) error {
	if vs, ok := multiValue(t, n); ok {
		outs := make([]string, len(vs))
		for i, v := range vs {
			s, err := e.emit(v)
			if err != nil {
				return err
			}
			outs[i] = s
		}
		e.line("return %s", strings.Join(outs, ", "))
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
			e.line("if %s {", cond)
			e.indent++
			if err := e.multiTail(args[1], n, name, sig); err != nil {
				return err
			}
			e.indent--
			e.line("}")
			return e.multiTail(args[2], n, name, sig)
		}
		// (let v (fn (x) B)) -- bind, then keep walking to the leaves.
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
				body, raw, out := openFresh(k, e.bound, mangle)
				e.types[raw[0]] = ty
				if ty == "int" {
					e.line("var %s %s = %s", out[0], e.tgt.ty(ty), val)
				} else {
					e.line("%s := %s", out[0], val)
				}
				return e.multiTail(body, n, name, sig)
			}
		}
	}
	return multiResultErr(name, sig, t)
}

// index emits `a[i]` — the host's own indexing, which is what the language's
// table lowers to when it survives to runtime.
func (e *Emitter) index(tab, i *core.Term) (string, error) {
	a, err := e.emit(tab)
	if err != nil {
		return "", err
	}
	idx, err := e.emit(i)
	if err != nil {
		return "", err
	}
	// A narrowed element widens on the way out. Go is typed and `[]byte`
	// indexed is a `byte`, so everything downstream would have to agree about
	// the width — which is precisely what the language says it must not, since
	// the value is an integer and only its storage is narrow. `int(b)` is a
	// zero-extend and free.
	if tab.Kind == core.KName {
		if _, ok := e.tgt.NarrowedElem(e.types[tab.Name]); ok {
			return fmt.Sprintf("%s(%s[%s])", e.tgt.ty("int"), a, idx), nil
		}
	}
	return fmt.Sprintf("%s[%s]", a, idx), nil
}

// arrayLit emits a table given by its GRAPH. Its element type comes from the
// elements, because a graph is data and the checker can read it off.
func (e *Emitter) arrayLit(t *core.Term) (string, error) {
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
	return fmt.Sprintf("%s{%s}", e.tgt.ty("array "+ty), strings.Join(out, ", ")), nil
}

// emitBuild is ADR 0018's scoped mutable buffer.
//
//	(build n (fn (b) body))   ⟶   b := make([]T, n); <body>; b
//
// The buffer is LINEAR: `set` consumes it and returns it, so the body threads
// one live name and the freeze on the way out copies nothing, because linearity
// guarantees nothing else holds it.
//
// Every mechanism this needs already existed. The stores are sequenced because
// `set` is impure and ADR 0010 never substitutes an impure argument; the buffer
// cannot escape because closures are refused as values; and it is lexically
// local in the residual because reduction is whole-program.
func (e *Emitter) emitBuild(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("build takes a length and (fn (b) …), got %s", t)
	}
	lam := args[1]
	if lam.Kind != core.KFn || len(lam.Params) != 1 {
		return "", fmt.Errorf("build's body must be (fn (b) …), got %s", lam)
	}
	count, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	body, raw, out := openFresh(lam, e.bound, mangle)
	elem := ElemType(e.tgt, lam, body, raw[0], e.typeOf, e.sig, e.topParams)
	e.types[raw[0]] = "array " + elem
	e.line("%s := make(%s, %s)", out[0], e.tgt.ty("array "+elem), count)
	return e.emit(body)
}

// emitSet is a store. It is a STATEMENT that returns the buffer, which is what
// makes `(set b i v)` consume and return `b` — the linear threading, spelled in
// the host's own assignment.
func (e *Emitter) emitSet(t *core.Term) (string, error) {
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
	// A NARROWED SLOT takes the value converted. Go is typed and `[]byte` holds
	// bytes; the value is an integer, so the store is where the two meet. The
	// range came from what is stored, so the conversion never truncates —
	// bufferElem widens to cover every store and the zero fill.
	if spell, ok := e.narrowedSlot(args[0]); ok {
		e.line("%s[%s] = %s(%s)", b, i, spell, v)
		return b, nil
	}
	e.line("%s[%s] = %s", b, i, v)
	return b, nil
}

// narrowedSlot reports the host spelling of a table's element when it is
// narrower than the target's own integer, given the term that names the table.
func (e *Emitter) narrowedSlot(tab *core.Term) (string, bool) {
	root := BufferRoot(tab)
	if root == "" {
		return "", false
	}
	return e.tgt.NarrowedElem(e.types[root])
}

// emitAlloc puts a rule in memory — the GATHER, pure and parallel by
// construction, as against `build`'s sequential scatter.
func (e *Emitter) emitAlloc(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 1 {
		return "", fmt.Errorf("alloc takes one table, got %s", t)
	}
	tab := args[0]
	if !isTableRule(e.tgt, tab) {
		// Allocating a graph is already memory; allocating a parameter is a
		// copy nobody asked for. Only a RULE has something to compute.
		return e.emit(tab)
	}
	rule := tab.Args()[1]
	if rule.Kind != core.KFn || len(rule.Params) != 1 {
		return "", fmt.Errorf("alloc's table must have an (fn (i) …) rule, got %s", rule)
	}
	count, err := e.emit(tab.Args()[0])
	if err != nil {
		return "", err
	}
	body, raw, out := openFresh(rule, e.bound, mangle)
	e.types[raw[0]] = "int"
	n := e.fresh("n")
	dst := e.fresh("t")
	e.line("var %s %s = %s", n, e.tgt.ty("int"), count)
	elem := e.typeOf(body)
	e.types[dst] = "array " + elem
	e.line("%s := make(%s, %s)", dst, e.tgt.ty("array "+elem), n)
	e.line("for %s := 0; %s < %s; %s++ {", out[0], out[0], n, out[0])
	e.indent++
	v, err := e.emit(body)
	if err != nil {
		return "", err
	}
	e.line("%s[%s] = %s", dst, out[0], v)
	e.indent--
	e.line("}")
	return dst, nil
}

// buildType is the type of a `(build n (fn (b) …))`, which is the type of its
// BODY and not necessarily the buffer.
//
// The first version assumed the body hands the buffer back, because that is
// what a sieve does and what ADR 0018 describes. A JSON tokeniser does not: it
// writes into a stack and returns a COUNT, and the buffer is dead at the
// boundary. That emitted a function declared `[]int` returning an `int`, which
// Go refused — found by writing the first program whose buffer is scratch
// rather than the result (json-2026-08-26).
//
// So the buffer's own type is bound and the body is asked. A body that returns
// the buffer still answers `array V`, because the parameter now has that type.
func (e *Emitter) buildType(lam *core.Term) string {
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

// Imports accumulates what emitted functions need. A package-level sink is
// crude — the real answer is that each binding declares its import and the file
// writer collects them, which is g5's Tier 2 binding format arriving in the
// emitter by need rather than by design.
var Imports = map[string]bool{}

// inferFrom walks the term assigning types to variables from the signatures of
// the primitives they are passed to.
func (e *Emitter) inferFrom(t *core.Term) {
	switch t.Kind {
	case core.KApp:
		op := t.Op()
		if op.Kind == core.KName {
			if p, ok := e.tgt.Prims[op.Name]; ok {
				for i, a := range t.Args() {
					if i < len(p.Args) && a.Kind == core.KName {
						// `any` is not a constraint, so it must not occupy the
						// slot a real one would fill. It is remembered weakly
						// instead: if nothing else ever constrains the name,
						// the host's own polymorphism is the honest answer.
						if p.Args[i] == "any" {
							e.weak[a.Name] = "any"
						} else if e.types[a.Name] == "" {
							e.types[a.Name] = p.Args[i]
						}
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

// inferLet seeds let-bound names, since their type comes from the bound value
// rather than from a primitive they are passed to.
func (e *Emitter) inferLet(t *core.Term) {
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

func (e *Emitter) typeOf(t *core.Term) string {
	switch t.Kind {
	case core.KInt:
		return "int"
	case core.KFloat:
		return "f64"
	case core.KBool:
		return "bool"
	case core.KName:
		return e.types[t.Name]
	case core.KApp:
		if op := t.Op(); op.Kind == core.KName {
			// INDEXING IS APPLICATION, so the type of `(a i)` is a's ELEMENT
			// type. This is the only place the checker needs to know tables
			// exist, and it is why `(array V)` lives in the signature language
			// only: a dynamic index forces homogeneity, so what the checker
			// sees is `Fin n → V` and no dependent type is needed (tables.md §5).
			if elem := core.ArrayElem(e.types[op.Name]); elem != "" {
				// A RANGE is an integer wherever it is used. The width belongs
				// to the storage and nowhere else, so a local reading a byte
				// array is an `int` and cannot overflow at 255.
				return core.ValueType(elem)
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
				// A fold's type is its accumulator's type, not a fixed one,
				// and a let's is its body's.
				if p.Kind == "loop" {
					return e.typeOf(t.Args()[0])
				}
				if p.Kind == "build" {
					return "vec-f64"
				}
				// A loop's type is the type of an exit leaf. `again` leaves are
				// not values, so they are skipped; a loop with none never
				// yields, and its type is whatever the context wants.
				if p.Kind == "iterate" && len(t.Args()) >= 1 {
					if lam := t.Args()[0]; lam.Kind == core.KFn {
						for i, n := range lam.Params {
							e.types[n] = e.typeOf(t.Args()[1+i])
						}
						if ty := exitType(e, lam.Body()); ty != "" {
							return ty
						}
					}
				}
				if p.Kind == "let" {
					if k := t.Args()[1]; k.Kind == core.KFn {
						return e.typeOf(k.Body())
					}
				}
				// A conditional's type is its branches'. There was no case for
				// this at all: an emitted function whose body was an `if`
				// returned `/*unknown*/`, and no program had one until `and`
				// and `or` became conditionals (ADR 0017). emitIf already
				// computed it for the temporary, one level down.
				if p.Kind == "cond" && len(t.Args()) == 3 {
					if ty := e.typeOf(t.Args()[1]); ty != "" {
						return ty
					}
					return e.typeOf(t.Args()[2])
				}
				// loop2's result is its FINISHER's type. This was never
				// implemented: it fell through to the declared result type,
				// which happened to say f64 — as false as fold-range's
				// accumulator type was, and only correct by luck. Removing the
				// declaration (target-files.md §4) exposed it.
				if p.Kind == "loop2" && len(t.Args()) == 6 {
					if fin := t.Args()[5]; fin.Kind == core.KFn && len(fin.Params) == 2 {
						e.types[fin.Params[0]], e.types[fin.Params[1]] = "f64", "f64"
						if ty := e.typeOf(fin.Body()); ty != "" {
							return ty
						}
					}
					return e.typeOf(t.Args()[0])
				}
				// A statement's value IS argument 0, which is what every
				// target file has said since dict-inc and what none of them
				// implemented — dict-inc got away with declaring `dict` for
				// both. print-line is the first primitive where they differ.
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

func (e *Emitter) line(format string, args ...any) {
	e.buf.WriteString(strings.Repeat("\t", e.indent))
	fmt.Fprintf(&e.buf, format, args...)
	e.buf.WriteString("\n")
}

func (e *Emitter) fresh(stem string) string {
	for {
		e.tmp++
		n := fmt.Sprintf("%s%d", stem, e.tmp)
		if !e.bound[n] {
			e.bound[n] = true
			return n
		}
	}
}

// emit writes any statements the term needs and returns a Go expression for its
// value. This is the expression/statement split that g3 §6 and g5 §4 both
// demanded under the name ANF — here it is, arriving for a third time.
func (e *Emitter) emit(t *core.Term) (string, error) {
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
		return mangle(t.Name), nil

	case core.KFn:
		return "", fmt.Errorf("a bare abstraction reached the emitter: %s\n"+
			"  This is an escaping closure. g6 measured its cost but the emitter\n"+
			"  does not implement closures yet.", t)

	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return "", fmt.Errorf("application of a non-name: %s\n"+
				"  The operator must be a primitive; recursive definitions were the other\n"+
				"  case and are rejected before reduction (ADR 0014).", t)
		}
		p, ok := e.tgt.Prims[op.Name]
		if !ok {
			// INDEXING IS APPLICATION (tables.md §3). A local name in operator
			// position can only be a table; see IsTableOperand for why.
			if IsTableOperand(mangle(op.Name), e.bound) && len(t.Args()) == 1 {
				return e.index(op, t.Args()[0])
			}
			return "", IndexingErr("Go", op.Name)
		}
		if p.Import != "" {
			e.imports[p.Import] = true
		}
		if p.Kind == "let" {
			return e.emitLet(t)
		}
		if p.Kind == "stmt" {
			args := t.Args()
			vals := make([]any, len(args))
			for i, a := range args {
				v, err := e.emit(a)
				if err != nil {
					return "", err
				}
				vals[i] = v
			}
			if len(args) > 0 && !atomicValue(vals[0].(string)) && strings.Contains(p.Form, "%s") {
				name := e.fresh("v")
				// The type is explicit when we know it. `v1 := (21 + 21)` gives
				// Go's default `int`, and a function declaring `int64` then
				// fails to compile — the literals were an untyped constant
				// before they were bound.
				if ty := e.tgt.ty(e.typeOf(args[0])); ty != "" {
					e.line("var %s %s = %s", name, ty, vals[0])
				} else {
					e.line("%s := %s", name, vals[0])
				}
				vals[0] = name
			}
			e.line("%s", fill(p.Form, vals))
			return vals[0].(string), nil
		}
		if p.Kind == "build" {
			return e.emitMakeVec(t)
		}
		if p.Kind == "iterate" {
			return e.emitLoop(t)
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
		switch p.Kind {
		case "table-build":
			return e.emitBuild(t)
		case "table-set":
			return e.emitSet(t)
		case "table-alloc":
			return e.emitAlloc(t)
		case "len":
			a, err := e.emit(t.Args()[0])
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("len(%s)", a), nil
		case "array":
			return e.arrayLit(t)
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
		// Parenthesised throughout rather than tracking precedence. Go's parser
		// does not care and neither does its optimiser; gofmt would strip them.
		return "(" + fmt.Sprintf(p.Form, vals...) + ")", nil
	}
	return "", fmt.Errorf("unhandled term: %s", t)
}

// emitFoldRange2 emits a loop with two accumulators. The pair is updated
// SIMULTANEOUSLY — g2 §6's parallel-assignment hazard, arriving in the emitter.
// Go has tuple assignment; other targets need temporaries.
func (e *Emitter) emitFoldRange2(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 6 {
		return "", fmt.Errorf("fold-range2 takes x0, y0, count, stepX, stepY, finish")
	}
	fin := args[5]
	if fin.Kind != core.KFn || len(fin.Params) != 2 {
		return "", fmt.Errorf("fold-range2's finisher must be (fn (ax ay) …), got %s", fin)
	}
	sx, sy := args[3], args[4]
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
	ax, ay, idx := mangle(sx.Params[0]), mangle(sx.Params[1]), mangle(sx.Params[2])
	e.types[sx.Params[0]], e.types[sx.Params[1]], e.types[sx.Params[2]] = "f64", "f64", "int"
	e.types[sy.Params[0]], e.types[sy.Params[1]], e.types[sy.Params[2]] = "f64", "f64", "int"
	n := e.fresh("n")

	syBody := core.Rename(sy.Body(), map[string]string{
		sy.Params[0]: sx.Params[0], sy.Params[1]: sx.Params[1], sy.Params[2]: sx.Params[2]})

	e.line("%s, %s := %s, %s", ax, ay, x0, y0)
	// The loop count is the language's `int`, which spells int64. A bare
	// `:=` would infer Go's `int` from a literal or from len(), and the
	// two do not compare — the declaration has to be explicit.
	e.line("var %s int64 = %s", n, count)
	e.emitNarrow(sx.Params[2], n, sx.Body(), syBody)
	e.line("for %s := int64(0); %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	bx, err := e.emit(sx.Body())
	if err != nil {
		return "", err
	}
	// The second step names its own parameters; rebind them to the first's.
	by, err := e.emit(syBody)
	if err != nil {
		return "", err
	}
	e.line("%s, %s = %s, %s", ax, ay, bx, by)
	e.indent--
	e.line("}")

	// The finisher consumes both accumulators, so compound loop state never
	// crosses the loop boundary as a compound value.
	e.types[fin.Params[0]], e.types[fin.Params[1]] = "f64", "f64"
	return e.emit(core.Rename(fin.Body(), map[string]string{
		fin.Params[0]: sx.Params[0], fin.Params[1]: sx.Params[1]}))
}

// emitMakeVec allocates an array and fills it. It is the one primitive that
// lets a program CONSTRUCT data rather than only compute over data it was given
// (construction.md).
//
// The destination is fresh and every element is written exactly once, so it is
// unique by construction — which is why this lands without answering g7's
// aliasing question — and Go already knows len(dst) == n from make, so no
// narrowing is needed.
func (e *Emitter) emitMakeVec(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("make-vec takes a length and an element function")
	}
	elem := args[1]
	if elem.Kind != core.KFn || len(elem.Params) != 1 {
		return "", fmt.Errorf("make-vec's element function must be (fn (i) …), got %s", elem)
	}
	elemBody, raw, out := openFresh(elem, e.bound, mangle)
	idxName := raw[0]
	e.types[idxName] = "int"

	count, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	n := e.fresh("n")
	dst := e.fresh("v")
	idx := out[0]

	// The loop count is the language's `int`, which spells int64. A bare
	// `:=` would infer Go's `int` from a literal or from len(), and the
	// two do not compare — the declaration has to be explicit.
	e.line("var %s int64 = %s", n, count)
	e.line("%s := make(%s, %s)", dst, e.tgt.ty("vec-f64"), n)
	e.line("for %s := int64(0); %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	body, err := e.emit(elemBody)
	if err != nil {
		return "", err
	}
	e.line("%s[%s] = %s", dst, idx, body)
	e.indent--
	e.line("}")
	e.types[dst] = "vec-f64"
	return dst, nil
}

// countedGuard finds the shape `(loop ((i z) …) (>= i BOUND) exit … )` — a
// clause that leaves the loop when one variable reaches a bound.
//
// That is the only shape worth narrowing for, and it is the shape every counted
// loop in the corpus has. The bound must not mention a loop variable, or
// hoisting it out of the loop would change what it means.
func countedGuard(e *Emitter, t *core.Term, raw []string) (string, *core.Term, bool) {
	for t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName {
		p, ok := e.tgt.Prims[t.Op().Name]
		if !ok || p.Kind != "cond" || len(t.Args()) != 3 {
			return "", nil, false
		}
		c := t.Args()[0]
		if c.Kind == core.KApp && c.Op().Kind == core.KName && len(c.Args()) == 2 {
			name := c.Op().Name
			if isOp(name, "ge") || isOp(name, "gt") {
				lhs, rhs := c.Args()[0], c.Args()[1]
				if lhs.Kind == core.KName && contains(raw, lhs.Name) && !mentions(rhs, raw) {
					return lhs.Name, rhs, true
				}
			}
		}
		t = t.Args()[2] // a later clause may carry the bound instead
	}
	return "", nil, false
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func mentions(t *core.Term, names []string) bool {
	switch t.Kind {
	case core.KName:
		return contains(names, t.Name)
	case core.KFn:
		return mentions(t.Body(), names)
	case core.KApp:
		for _, k := range t.Kids {
			if mentions(k, names) {
				return true
			}
		}
	}
	return false
}

// emitNarrow restricts every container the loop indexes by the BARE loop
// variable to the loop's own count, before the loop.
//
// This is the whole of bounds-check elimination, and it is worth 1.96x on Go
// (gauntlet/results/bce-2026-08-15.md). Our own proof buys nothing — Go's BCE
// has never heard of us — so the only thing that works is emitting a shape the
// host re-proves for itself. `q = q[:n]` turns one check per iteration into one
// check before the loop.
//
// It is legal precisely because docs/spec/primitives.md §2 specifies `aindex`
// as *unspecified out of bounds*: narrowing moves the panic earlier, and no
// program with defined meaning can tell.
//
// Conservative on purpose. A container is narrowed only if EVERY occurrence of
// it in the body is an index by the bare loop variable — the stencil indexes
// `a` at `j`, `j+1` and `j+2`, so `a` is left alone and stays correct.
func (e *Emitter) emitNarrow(idxName, n string, bodies ...*core.Term) {
	for _, name := range e.narrowTargets(idxName, bodies...) {
		m := mangle(name)
		e.line("%s", fmt.Sprintf(e.tgt.Narrow, m, m, n))
	}
}

// narrowTargets is the collection half, separated because a caller that has to
// INTRODUCE the bound needs to know whether anything will use it first. A
// `loop` over a single array narrows nothing — the guard already bounds it —
// and emitting the bound anyway is `declared and not used`, which is a Go
// compile error rather than a wasted line.
func (e *Emitter) narrowTargets(idxName string, bodies ...*core.Term) []string {
	if e.tgt.Narrow == "" {
		return nil // this target has no such shape; JS and Java do not
	}
	good, bad := map[string]bool{}, map[string]bool{}
	var walk func(t *core.Term)
	walk = func(t *core.Term) {
		if t.Kind == core.KApp && t.Op().Kind == core.KName {
			if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Index {
				a := t.Args()
				if len(a) == 2 && a[0].Kind == core.KName &&
					a[1].Kind == core.KName && a[1].Name == idxName {
					good[a[0].Name] = true
					return // accounted for; do not mark it used elsewhere
				}
			}
			// INDEXING IS APPLICATION, so the shape this looks for changed.
			// `(a i)` is what `(go.at-float64 a i)` used to be, and without
			// this the bounds-check elimination pattern silently stopped
			// firing — worth 1.96x on compute-bound loops
			// (bce-2026-08-15) and nothing on memory-bound ones.
			if _, isPrim := e.tgt.Prims[t.Op().Name]; !isPrim && len(t.Args()) == 1 {
				if a := t.Args()[0]; a.Kind == core.KName && a.Name == idxName {
					good[t.Op().Name] = true
					return
				}
			}
		}
		switch t.Kind {
		case core.KName:
			bad[t.Name] = true
		case core.KFn:
			walk(t.Body())
		case core.KApp:
			for _, k := range t.Kids {
				walk(k)
			}
		}
	}
	for _, b := range bodies {
		walk(b)
	}

	names := make([]string, 0, len(good))
	for name := range good {
		if !bad[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// emitLet binds a value to a name and continues with the body. The λ here is a
// binder, not a closure — which refines g6's "a surviving λ is an escaping
// closure" to "…unless it is let's continuation".
func (e *Emitter) emitLet(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 2 {
		return "", fmt.Errorf("let takes a value and a continuation")
	}
	k := args[1]
	if k.Kind != core.KFn || len(k.Params) != 1 {
		return "", fmt.Errorf("let's continuation must be (fn (x) …), got %s", k)
	}
	val, err := e.emit(args[0])
	if err != nil {
		return "", err
	}
	// A binder used zero times is a sequencing point, not a binding: β keeps an
	// impure argument whatever its occurrence count (effects.md §5), and that is
	// what `seq` desugars into. Emitting `x := v` here would be rejected by Go
	// as an unused variable.
	if !core.Occurs(k.Body(), k.Params[0]) {
		if !emitsStatement(e.tgt, args[0]) && !atomicValue(val) {
			e.line("_ = %s", val) // Go forbids a bare expression statement
		}
		return e.emit(k.Body())
	}
	ty := e.typeOf(args[0])
	body, raw, out := openFresh(k, e.bound, mangle)
	e.types[raw[0]] = ty
	if ty == "int" {
		e.line("var %s %s = %s", out[0], e.tgt.ty(ty), val)
	} else {
		e.line("%s := %s", out[0], val)
	}
	return e.emit(body)
}

// emitsStatement reports whether emitting this term already wrote a line for its
// effect, in which case a sequencing let needs to add nothing. Shared by the
// three backends, which agree here and differ only in how they spell a discarded
// expression.
func emitsStatement(tgt *Target, t *core.Term) bool {
	if t.Kind != core.KApp || t.Op().Kind != core.KName {
		return false
	}
	return tgt.Prims[t.Op().Name].Kind == "stmt"
}

// emitIf turns (if c then else) into a Go if statement assigning to a temporary,
// because Go has no conditional expression. The branches may themselves emit
// statements, which is why this cannot be a format string.
// emitConnective emits Go's own operator for a conditional that is one.
func (e *Emitter) emitConnective(c Connective) (string, error) {
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

func (e *Emitter) emitIf(t *core.Term) (string, error) {
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
	ty := e.typeOf(args[1])
	if ty == "" {
		ty = e.typeOf(args[2])
	}
	tmp := e.fresh("t")
	e.line("var %s %s", tmp, e.tgt.ty(ty))
	e.line("if %s {", cond)
	e.indent++
	thenV, err := e.emit(args[1])
	if err != nil {
		return "", err
	}
	e.line("%s = %s", tmp, thenV)
	e.indent--
	e.line("} else {")
	e.indent++
	elseV, err := e.emit(args[2])
	if err != nil {
		return "", err
	}
	e.line("%s = %s", tmp, elseV)
	e.indent--
	e.line("}")
	return tmp, nil
}

// emitFoldRange turns (fold-range init count (fn (acc i) body)) into a loop.
func (e *Emitter) emitFoldRange(t *core.Term) (string, error) {
	args := t.Args()
	if len(args) != 3 {
		return "", fmt.Errorf("fold-range takes init, count and a step function")
	}
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

	// Fresh binders, opened on the CLOSED body — see openFresh. Using the
	// hints emitted `for i := …` inside `for i := …` and `acc := acc`.
	accTy := e.typeOf(args[0])
	body, raw, out := openFresh(step, e.bound, mangle)
	accName, idxName := raw[0], raw[1]
	e.types[accName] = accTy
	e.types[idxName] = "int"
	acc, idx := out[0], out[1]
	n := e.fresh("n")

	// An integer accumulator declared with `:=` from a literal takes Go's
	// default `int`, which does not assign to our `int64`. No gauntlet program
	// folds over an integer, so this had never fired. Only `int` needs the
	// explicit form — `acc := 0.0` already gives float64, and writing
	// `var acc float64 = 0.0` would be noise a human would not write.
	if accTy == "int" {
		e.line("var %s %s = %s", acc, e.tgt.ty(accTy), init)
	} else {
		e.line("%s := %s", acc, init)
	}
	// The loop count is the language's `int`, which spells int64. A bare
	// `:=` would infer Go's `int` from a literal or from len(), and the
	// two do not compare — the declaration has to be explicit.
	e.line("var %s int64 = %s", n, count)
	e.emitNarrow(idxName, n, body)
	e.line("for %s := int64(0); %s < %s; %s++ {", idx, idx, n, idx)
	e.indent++
	got, err := e.emit(body)
	if err != nil {
		return "", err
	}
	if got != acc { // a statement-primitive already updated it in place
		e.line("%s = %s", acc, got)
	}
	e.indent--
	e.line("}")
	return acc, nil
}

// ---------------------------------------------------------------- names
//
// Our identifiers admit -, ?, !, and any Unicode letter. Go's do not. Mangling
// is therefore forced, and it is the first place the language and the target
// disagree about what a name is.
//
// Two jobs, and they must not be confused: making a name *legal*, and making it
// *exported*. mangle does only the first — capitalising every identifier turned
// locals into exported-looking names, which is wrong and was caught by the very
// first emission.

func mangle(s string) string {
	var b strings.Builder
	upper := false
	for _, r := range s {
		switch {
		// `-` word break, `.` qualifier, `/` module path separator: all three
		// become camel-case boundaries so a qualified name is one host identifier.
		case r == '-' || r == '.' || r == '/':
			upper = true
		// `#` starts a name the READER generates and a program cannot write —
		// `match`'s loop variables, `values`' selector. It becomes `_` rather
		// than an escape so the emitted code is readable; a collision with a
		// user's `_m0` is resolved by openFresh like any other.
		case r == '#':
			b.WriteString("_")
		case r == '?':
			b.WriteString("P")
		case r == '!':
			b.WriteString("B")
		case unicode.IsLetter(r) || r == '_':
			if upper {
				b.WriteString(strings.ToUpper(string(r)))
				upper = false
			} else {
				b.WriteRune(r)
			}
		case unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			fmt.Fprintf(&b, "U%04X", r)
		}
	}
	out := b.String()
	if out == "" || unicode.IsDigit(rune(out[0])) {
		out = "X" + out
	}
	if goKeywords[out] {
		out += "_"
	}
	return out
}

// export makes a legal name exported. Separate from mangle by design.
func export(s string) string {
	m := mangle(s)
	if m == "" {
		return m
	}
	return strings.ToUpper(m[:1]) + m[1:]
}

var goKeywords = map[string]bool{}

func init() {
	for _, k := range strings.Fields(`break case chan const continue default defer else
		fallthrough for func go goto if import interface map package range return select
		struct switch type var`) {
		goKeywords[k] = true
	}
}

// File wraps emitted functions in a compilable Go file.
func File(pkg string, funcs map[string]string) string {
	names := make([]string, 0, len(funcs))
	for n := range funcs {
		names = append(names, n)
	}
	sort.Strings(names)

	var out strings.Builder
	out.WriteString("// Code generated by oroboros. DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "package %s\n\n", pkg)
	if len(Imports) > 0 {
		imps := make([]string, 0, len(Imports))
		for i := range Imports {
			imps = append(imps, i)
		}
		sort.Strings(imps)
		for _, i := range imps {
			fmt.Fprintf(&out, "import %q\n", i)
		}
		out.WriteString("\n")
	}
	for _, n := range names {
		out.WriteString(funcs[n])
		out.WriteString("\n")
	}
	return out.String()
}

// ---------------------------------------------------------------- loop
//
// (loop (fn (x…) body) z…) — docs/spec/iteration.md.
//
// Emitted as the host's own `for`, with the clause chain as a statement
// if-chain: an `again` leaf assigns the loop variables and continues, any other
// leaf assigns the result and breaks.

func isAgain(t *core.Term) bool {
	return t.Kind == core.KApp && t.Op().Kind == core.KName && t.Op().Name == "again"
}

// exitType finds the type of the first non-`again` leaf, which is the loop's.
func exitType(e *Emitter, t *core.Term) string {
	if isAgain(t) {
		return ""
	}
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "let" && len(t.Args()) == 2 {
			if k := t.Args()[1]; k.Kind == core.KFn && len(k.Params) == 1 {
				return exitType(e, k.Body())
			}
		}
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			if ty := exitType(e, t.Args()[1]); ty != "" {
				return ty
			}
			return exitType(e, t.Args()[2])
		}
	}
	return e.typeOf(t)
}

func (e *Emitter) emitLoop(t *core.Term) (string, error) {
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

	// Types first: a loop variable's type is its initial value's.
	tys := make([]string, len(inits))
	for i := range inits {
		tys[i] = e.typeOf(inits[i])
	}
	vals := make([]string, len(inits))
	for i, z := range inits {
		v, err := e.emit(z)
		if err != nil {
			return "", err
		}
		vals[i] = v
	}

	body, raw, names := openFresh(lam, e.bound, mangle)
	for i, n := range raw {
		e.types[n] = tys[i]
	}
	for i := range names {
		if tys[i] == "int" || tys[i] == "" {
			e.line("var %s %s = %s", names[i], e.tgt.ty(orAny(tys[i])), vals[i])
		} else {
			e.line("%s := %s", names[i], vals[i])
		}
	}

	// BOUNDS-CHECK ELIMINATION, which `fold-range` had and `loop` lost.
	//
	// ADR 0015 replaced `fold-range` with `loop`, and bce-2026-08-15's emitter
	// pattern was wired into `emitFoldRange` only. So every program that moved
	// to a `loop` quietly gave up the 1.96× that measurement bought — visible
	// the moment the gauntlet's `dot` was written natively and came in at
	// 1455 ns against a hand-written 680.
	//
	// The transformation is the same one: narrow every container the loop
	// indexes to the loop's own bound, ONCE, before the loop. Go then knows
	// `i < n == len(v)` and drops the per-iteration check. A container shorter
	// than the bound panics on the slice expression instead of inside the loop,
	// which is the same failure moved earlier.
	if idx, bound, ok := countedGuard(e, body, raw); ok {
		if bv, err := e.emit(bound); err == nil && len(e.narrowTargets(idx, body)) > 0 {
			n := e.fresh("n")
			// The target's spelling of `int`, not Go's literal `int`: on the
			// portable layer that is `int64`, and `var n int = int64(len(a))`
			// does not compile.
			e.line("var %s %s = %s", n, e.tgt.ty("int"), bv)
			e.emitNarrow(idx, n, body)
		}
	}

	// If every exit clause yields the SAME name, that name is the loop's value
	// and no result temporary is needed. It is not only tidiness: the extra
	// `var r1 []bool` defeated Go's escape analysis on the sieve, turning a
	// stack-allocated slice into a heap allocation — 20,480 B/op against
	// hand-written's zero.
	rty := exitType(e, body)
	result := soleExit(e.tgt.Prims, body, raw, names, e.bound, mangle)
	if result == "" {
		result = e.fresh("r")
		e.line("var %s %s", result, e.tgt.ty(orAny(rty)))
	}
	// A uniformly-updated loop variable moves into the `for` statement's post
	// clause, which is what turns several back edges into one — see PostVars.
	post := PostVars(body, raw)
	if len(post) > 0 {
		var lhs, rhs []string
		for i := range names {
			if u, ok := post[i]; ok {
				v, err := e.emit(u)
				if err != nil {
					return "", err
				}
				lhs = append(lhs, names[i])
				rhs = append(rhs, v)
			}
		}
		e.line("for ; ; %s = %s {", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
	} else {
		e.line("for {")
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

func orAny(ty string) string {
	if ty == "" {
		return "any"
	}
	return ty
}

// emitLoopBody walks the clause chain, emitting statements rather than an
// expression. Leaves are the only thing that differ from emitIf.
func (e *Emitter) emitLoopBody(t *core.Term, raw, names []string, result string, post map[int]*core.Term) error {
	if t.Kind == core.KApp && t.Op().Kind == core.KName {
		if p, ok := e.tgt.Prims[t.Op().Name]; ok && p.Kind == "cond" && len(t.Args()) == 3 {
			cond, err := e.emit(t.Args()[0])
			if err != nil {
				return err
			}
			e.line("if %s {", cond)
			e.indent++
			if err := e.emitLoopBody(t.Args()[1], raw, names, result, post); err != nil {
				return err
			}
			e.indent--
			e.line("}")
			return e.emitLoopBody(t.Args()[2], raw, names, result, post)
		}
	}
	// `let binds; if branches` — the reader allows `again` under a `let`, so the
	// emitter has to walk through one. A `seq` is a `let` with an ignored
	// binder, which is how a program writes a mutation and then continues
	// WITHOUT threading the container through the loop state.
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
					if !emitsStatement(e.tgt, args[0]) && !atomicValue(val) {
						e.line("_ = %s", val)
					}
					return e.emitLoopBody(k.Body(), raw, names, result, post)
				}
				ty := e.typeOf(args[0])
				kb, kraw, kout := openFresh(k, e.bound, mangle)
				e.types[kraw[0]] = ty
				if ty == "int" {
					e.line("var %s %s = %s", kout[0], e.tgt.ty(ty), val)
				} else {
					e.line("%s := %s", kout[0], val)
				}
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
		e.line("%s = %s", result, v)
	}
	e.line("break")
	return nil
}

// emitAgain assigns the loop variables and continues.
//
// Two things it does not do. An argument that is syntactically the variable
// itself is UNCHANGED, so no assignment is emitted for it — which is hamza's
// optimisation, and it also shrinks the simultaneity problem, because an
// unchanged variable cannot be clobbered. And Go has parallel assignment, so
// the changed ones need no temporaries at all.
func (e *Emitter) emitAgain(t *core.Term, raw, names []string, post map[int]*core.Term) error {
	as := t.Args()
	if len(as) != len(names) {
		return fmt.Errorf("again takes %d argument(s), given %d", len(names), len(as))
	}
	var lhs, rhs []string
	for i, a := range as {
		if a.Kind == core.KName && a.Name == raw[i] {
			continue // unchanged: x = x is noise
		}
		if _, hoisted := post[i]; hoisted {
			continue // the `for` statement's post clause does this one
		}
		v, err := e.emit(a)
		if err != nil {
			return err
		}
		// A statement primitive yields its container, so `(again (set s i x) …)`
		// emits the write and then hands back `s`. Comparing the EMITTED value
		// catches that where comparing the term cannot.
		if v == names[i] {
			continue
		}
		lhs = append(lhs, names[i])
		rhs = append(rhs, v)
	}
	if len(lhs) > 0 {
		e.line("%s = %s", strings.Join(lhs, ", "), strings.Join(rhs, ", "))
	}
	e.line("continue")
	return nil
}
