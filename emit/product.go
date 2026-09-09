package emit

import (
	"fmt"

	"oroboros/core"
)

// FLATTENING A PRODUCT IS CURRYING, AND THAT IS THE WHOLE PASS.
//
// products.md derives the product as `Π` over a statically finite index set,
// and derives its representation from one isomorphism:
//
//	Π_{(i,j) ∈ I×J} V  ≅  Π_{i∈I} Π_{j∈J} V
//
// Right to left is an array of k-tuples; left to right is a flat table indexed
// by `k·i + j`. They are the same object, so **which one is emitted is a target
// decision** (ADR 0003), and this pass emits the flat one — for a reason that
// is measured rather than chosen: it is what `tree.oro`, `freq.oro` and
// `kara/core.oro` already write by hand, so adopting the type cannot cost
// speed on any host. The other representations are a target declaration and a
// benchmark, in that order, and products.md §8 says not to pick one by argument.
//
// So the pass is one rewrite in four positions, and after it the term is
// EXACTLY what a hand-strided program is: ordinary tables and ordinary index
// arithmetic. Nothing downstream — the checker, the refinement layer, the
// interval analysis, the four backends — learns that products exist. That is
// the point, and it is why this could be built without touching a backend.
//
//	((t i) j)              ->  (t (+ (* k i) j))          projection, j static
//	(set b i (array v…))   ->  k nested sets              construction
//	(len t)                ->  (/ (len t) k)              elements, not slots
//	(build n f)            ->  (build (* k n) f)          capacity in slots
//
// WHY THE FIELD INDEX MUST BE STATIC IS NOT A RESTRICTION WE IMPOSE.
// tables.md §5.3: a dynamic index forces homogeneity and forces existence, and
// it is the same condition doing both. Read backwards it defines the product —
// a statically indexed table may vary its family — so a product whose field
// index were dynamic would simply be an array, and it is one.

// prodTable reports the arity of a table whose element is a product, and the
// element type the flat table gets.
//
// COMPONENTS MUST SHARE A REPRESENTATION, and that is what this build covers.
// `(array int int)` is `Π_{Fin 2} int`, a fixed-length HOMOGENEOUS table, and it
// flattens into a table of the same element. `(array int string)` is the
// genuinely heterogeneous product; it is a type, it type-checks, and it has no
// representation here — that is the layout machinery products.md §8 puts last,
// and refusing it by name is better than flattening it into a slot that cannot
// hold both.
func prodTable(ty string) (int, string, bool) {
	elem := core.ArrayElem(ty)
	if elem == "" {
		return 0, "", false
	}
	parts := core.ProdTypes(elem)
	if len(parts) < 2 {
		return 0, "", false
	}
	// The flat element must hold every component. Where each is an integer —
	// possibly a range — that is the join, which is the same rule `bufferElem`
	// uses when a buffer has several stores.
	// THROUGH IntRange AND NOT ValueType, which is scalarrange-2026-08-31's own
	// trap: `ValueType` normalises a range to `int` because that is the TYPING
	// answer, and this wants the REPRESENTATION one. Calling the wrong one gave
	// `array int` where `array int 0 65535` was meant, so a pair of bytes lost
	// its element width and every operation reading one went unbounded — the
	// same distinction `storedRange` records one layer down.
	lo, hi, any, wide := int64(0), int64(0), false, false
	for _, p := range parts {
		l, h, ok := core.IntRange(p)
		if !ok {
			// A component with no range is an `int` if it is one, and
			// otherwise has no place in a flat table of integers.
			if p != "int" {
				return 0, "", false
			}
			wide = true
			continue
		}
		if !any || l < lo {
			lo = l
		}
		if !any || h > hi {
			hi = h
		}
		any = true
	}
	if wide || !any {
		return len(parts), "int", true
	}
	return len(parts), fmt.Sprintf("int %d %d", lo, hi), true
}

// FlattenType rewrites a type so that no product survives in it: an array or
// buffer of a k-product becomes a table of the flat element. Every other type
// is returned unchanged.
func FlattenType(ty string) string {
	k, elem, ok := prodTable(ty)
	if !ok {
		return ty
	}
	if core.IsBuffer(ty) {
		return "buffer " + elem
	}
	_ = k
	return "array " + elem
}

// FlattenProducts rewrites an array of products into a flat table and returns
// the number of projections it uncurried. It returns the ORIGINAL term when
// nothing fired: a pass that rebuilds a term it did not change hands back a
// different pointer, and this compiler has been bitten four times by a map keyed
// on one.
func FlattenProducts(tgt *Target, sig *core.Sig, t *core.Term) (*core.Term, *core.Sig, int, error) {
	f := &flattener{tgt: tgt, arity: map[string]int{}}
	if sig != nil && t != nil && t.Kind == core.KFn {
		// The signature is the only place a product can be DECLARED, so it is
		// the only seed. A local `build` is found on the way down, from the
		// products it stores.
		body, raw, _ := openFresh(t, map[string]bool{}, mangle)
		for i, name := range raw {
			if i >= len(sig.Params) {
				break
			}
			if k, _, ok := prodTable(sig.Params[i].Type); ok {
				f.arity[name] = k
			}
		}
		if len(f.arity) == 0 && !f.anyProduct(body) {
			return t, sig, 0, nil
		}
		out, err := f.walk(body)
		if err != nil {
			return t, sig, 0, err
		}
		if f.n == 0 {
			return t, sig, 0, nil
		}
		return core.Fn(raw, out), flattenSig(sig), f.n, nil
	}
	if !f.anyProduct(t) {
		return t, sig, 0, nil
	}
	out, err := f.walk(t)
	if err != nil || f.n == 0 {
		return t, sig, 0, err
	}
	return out, flattenSig(sig), f.n, nil
}

// flattenSig rewrites the declared types so the checker, which runs after this
// pass, sees the flat table the program has become.
func flattenSig(sig *core.Sig) *core.Sig {
	if sig == nil {
		return nil
	}
	out := *sig
	out.Params = append([]core.SigParam(nil), sig.Params...)
	for i := range out.Params {
		out.Params[i].Type = FlattenType(out.Params[i].Type)
	}
	out.Results = append([]string(nil), sig.Results...)
	for i := range out.Results {
		out.Results[i] = FlattenType(out.Results[i])
	}
	return &out
}

type flattener struct {
	tgt   *Target
	arity map[string]int // a table's name -> the arity of its product element
	n     int
}

// anyProduct is the cheap test that lets a program with no products pay nothing:
// a product literal in a store is the only way one can appear without being
// declared.
func (f *flattener) anyProduct(t *core.Term) bool {
	if t == nil {
		return false
	}
	if f.isKind(t, "table-set") && len(t.Kids) == 4 && f.prodLit(t.Kids[3]) != nil {
		return true
	}
	if t.Kind == core.KFn {
		return f.anyProduct(t.Closed())
	}
	for _, k := range t.Kids {
		if f.anyProduct(k) {
			return true
		}
	}
	return false
}

func (f *flattener) isKind(t *core.Term, kind string) bool {
	if t == nil || t.Kind != core.KApp || len(t.Kids) == 0 || t.Kids[0].Kind != core.KName {
		return false
	}
	p, ok := f.tgt.Prims[t.Kids[0].Name]
	return ok && p.Kind == kind
}

// prodLit returns the components of `(array v1 … vk)` used as a VALUE — the
// product's introduction form — or nil. Arity two and up, because `(array v)`
// is a one-element table and means what it always meant.
func (f *flattener) prodLit(t *core.Term) []*core.Term {
	if t == nil || t.Kind != core.KApp || len(t.Kids) < 3 ||
		t.Kids[0].Kind != core.KName || t.Kids[0].Name != "array" {
		return nil
	}
	return t.Kids[1:]
}

func mul(k int, e *core.Term) *core.Term {
	if e.Kind == core.KInt {
		return core.Int(int64(k) * e.Int)
	}
	return core.App(core.Name("*"), core.Int(int64(k)), e)
}

func add(e *core.Term, j int) *core.Term {
	if j == 0 {
		return e
	}
	if e.Kind == core.KInt {
		return core.Int(e.Int + int64(j))
	}
	return core.App(core.Name("+"), e, core.Int(int64(j)))
}

func (f *flattener) walk(t *core.Term) (*core.Term, error) {
	if t == nil {
		return nil, nil
	}

	// PROJECTION — `((t i) j)`, the curried form, uncurried.
	if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[1].Kind == core.KInt {
		inner := t.Kids[0]
		if inner.Kind == core.KApp && len(inner.Kids) == 2 &&
			inner.Kids[0].Kind == core.KName {
			if k, ok := f.arity[inner.Kids[0].Name]; ok {
				j := int(t.Kids[1].Int)
				if j < 0 || j >= k {
					return nil, fmt.Errorf("field %d of a %d-component product: "+
						"a product's field index is static and must name a field", j, k)
				}
				idx, err := f.walk(inner.Kids[1])
				if err != nil {
					return nil, err
				}
				f.n++
				return core.App(inner.Kids[0], add(mul(k, idx), j)), nil
			}
		}
	}

	// FIELD UPDATE — `(set (b i) j v)`, the exact dual of `((b i) j)`.
	//
	// Set applied to a PROJECTION, because read applied to a projection is what
	// a field access is; one surface, both directions. Without it a program that
	// changes one field has to rewrite the whole element, which is what the
	// run-length counter in freq.oro would have had to do — and rewriting a
	// field you did not change is how a swap becomes a copy (effects.md §7c).
	if f.isKind(t, "table-set") && len(t.Kids) == 4 && t.Kids[2].Kind == core.KInt {
		if proj := t.Kids[1]; proj.Kind == core.KApp && len(proj.Kids) == 2 &&
			proj.Kids[0].Kind == core.KName {
			if k, ok := f.arity[proj.Kids[0].Name]; ok {
				j := int(t.Kids[2].Int)
				if j < 0 || j >= k {
					return nil, fmt.Errorf("field %d of a %d-component product", j, k)
				}
				idx, err := f.walk(proj.Kids[1])
				if err != nil {
					return nil, err
				}
				v, err := f.walk(t.Kids[3])
				if err != nil {
					return nil, err
				}
				f.n++
				return core.App(t.Kids[0], proj.Kids[0], add(mul(k, idx), j), v), nil
			}
		}
	}

	// CONSTRUCTION — `(set b i (array v1 … vk))` becomes k stores, and the
	// order is the field order so the emitted code reads like the source.
	if f.isKind(t, "table-set") && len(t.Kids) == 4 {
		if root := BufferRoot(t.Kids[1]); root != "" {
			if k, ok := f.arity[root]; ok {
				vs := f.prodLit(t.Kids[3])
				if vs == nil {
					return nil, fmt.Errorf("a table whose element is a %d-component "+
						"product is written with `(array v1 … v%d)`, not with a scalar", k, k)
				}
				if len(vs) != k {
					return nil, fmt.Errorf("%d values stored into a %d-component product",
						len(vs), k)
				}
				b, err := f.walk(t.Kids[1])
				if err != nil {
					return nil, err
				}
				idx, err := f.walk(t.Kids[2])
				if err != nil {
					return nil, err
				}
				for j, v := range vs {
					vv, err := f.walk(v)
					if err != nil {
						return nil, err
					}
					b = core.App(t.Kids[0], b, add(mul(k, idx), j), vv)
				}
				f.n++
				return b, nil
			}
		}
	}

	// LENGTH — a table of products has as many ELEMENTS as it has slots divided
	// by the arity, and a program that asks how many words there are must not be
	// told how many offsets there are.
	if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[0].Kind == core.KName &&
		t.Kids[1].Kind == core.KName {
		if p, ok := f.tgt.Prims[t.Kids[0].Name]; ok && p.Kind == "len" {
			if k, ok := f.arity[t.Kids[1].Name]; ok {
				f.n++
				return core.App(core.Name("/"), t, core.Int(int64(k))), nil
			}
		}
	}

	// A `build` whose stores are products: its capacity is in slots, and its
	// binder joins the environment for the body.
	if f.isKind(t, "table-build") && len(t.Kids) == 3 &&
		t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1 {
		lam := t.Kids[2]
		body, raw, _ := openFresh(lam, map[string]bool{}, mangle)
		if k := f.storedArity(body, raw[0]); k > 0 {
			f.arity[raw[0]] = k
			inner, err := f.walk(body)
			delete(f.arity, raw[0])
			if err != nil {
				return nil, err
			}
			n, err := f.walk(t.Kids[1])
			if err != nil {
				return nil, err
			}
			f.n++
			return core.App(t.Kids[0], mul(k, n), core.Fn(raw, inner)), nil
		}
	}

	// A BINDER THAT RENAMES A PRODUCT TABLE CARRIES THE ARITY WITH IT, and this
	// is not an extra: a `build`'s binder is almost always rebound by the `loop`
	// that fills it, so without propagation the arity is known for exactly one
	// term and lost immediately. ADR 0018's threaded buffer travels through
	// `loop` variables and `let`s, and the arity travels with it.
	if f.isKind(t, "loop") && len(t.Kids) >= 2 && t.Kids[1].Kind == core.KFn {
		lam := t.Kids[1]
		inits := t.Kids[2:]
		body, raw, _ := openFresh(lam, map[string]bool{}, mangle)
		var bound []string
		for i, z := range inits {
			if i < len(raw) {
				if k := f.arityOf(z); k > 0 {
					f.arity[raw[i]] = k
					bound = append(bound, raw[i])
				}
			}
		}
		inner, err := f.walk(body)
		for _, n := range bound {
			delete(f.arity, n)
		}
		if err != nil {
			return nil, err
		}
		kids := make([]*core.Term, len(t.Kids))
		kids[0], kids[1] = t.Kids[0], core.Fn(raw, inner)
		for i, z := range inits {
			zz, err := f.walk(z)
			if err != nil {
				return nil, err
			}
			kids[2+i] = zz
		}
		out := *t
		out.Kids = kids
		return &out, nil
	}
	if t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName &&
		t.Kids[0].Name == "let" && t.Kids[2].Kind == core.KFn &&
		len(t.Kids[2].Params) == 1 {
		v, err := f.walk(t.Kids[1])
		if err != nil {
			return nil, err
		}
		body, raw, _ := openFresh(t.Kids[2], map[string]bool{}, mangle)
		k := f.arityOf(t.Kids[1])
		if k > 0 {
			f.arity[raw[0]] = k
		}
		inner, err := f.walk(body)
		if k > 0 {
			delete(f.arity, raw[0])
		}
		if err != nil {
			return nil, err
		}
		return core.App(t.Kids[0], v, core.Fn(raw, inner)), nil
	}
	if t.Kind == core.KFn {
		body, raw, _ := openFresh(t, map[string]bool{}, mangle)
		out, err := f.walk(body)
		if err != nil {
			return nil, err
		}
		return core.Fn(raw, out), nil
	}

	kids := make([]*core.Term, len(t.Kids))
	changed := false
	for i, k := range t.Kids {
		out, err := f.walk(k)
		if err != nil {
			return nil, err
		}
		kids[i] = out
		if out != k {
			changed = true
		}
	}
	if !changed {
		return t, nil
	}
	out := *t
	out.Kids = kids
	return &out, nil
}

// arityOf is the arity a term carries: a name that is a product table, or a
// store chain rooted at one — `(set b i v)` hands back `b`, which is what makes
// a threaded buffer a single value through a whole loop.
func (f *flattener) arityOf(t *core.Term) int {
	if t == nil {
		return 0
	}
	if t.Kind == core.KName {
		return f.arity[t.Name]
	}
	if root := BufferRoot(t); root != "" {
		return f.arity[root]
	}
	// A `build` that fills itself with products IS a product table, and this is
	// how one reaches a `let`: `(let (build …) (fn (sp) …))` is what a helper
	// returning a table becomes once reduction has inlined it, so without this
	// the arity stops at the definition boundary that no longer exists.
	if f.isKind(t, "table-build") && len(t.Kids) == 3 &&
		t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1 {
		body, raw, _ := openFresh(t.Kids[2], map[string]bool{}, mangle)
		return f.storedArity(body, raw[0])
	}
	return 0
}

// storedArity reads a local buffer's product arity off its stores, which is the
// same syntactic evidence `bufferElem` reads an element width from.
//
// IT MUST FOLLOW THE REBINDING, and that is the whole subtlety: a `build`'s
// binder is almost always handed straight to a `loop`, so the stores are to the
// LOOP's variable and not to the binder. Looking for stores to the binder alone
// finds none, on every program anyone would write. So the walk carries the set
// of names that alias the buffer — the same propagation the rewrite does, done
// once in the other direction.
func (f *flattener) storedArity(body *core.Term, name string) int {
	alias := map[string]bool{name: true}
	rootOf := func(t *core.Term) string {
		if t == nil {
			return ""
		}
		if t.Kind == core.KName {
			return t.Name
		}
		return BufferRoot(t)
	}
	found := 0
	var walk func(*core.Term)
	walk = func(t *core.Term) {
		if t == nil || found < 0 {
			return
		}
		if f.isKind(t, "loop") && len(t.Kids) >= 2 && t.Kids[1].Kind == core.KFn {
			b, raw, _ := openFresh(t.Kids[1], map[string]bool{}, mangle)
			for i, z := range t.Kids[2:] {
				if i < len(raw) && alias[rootOf(z)] {
					alias[raw[i]] = true
				}
			}
			walk(b)
			for _, z := range t.Kids[2:] {
				walk(z)
			}
			return
		}
		if t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName &&
			t.Kids[0].Name == "let" && t.Kids[2].Kind == core.KFn &&
			len(t.Kids[2].Params) == 1 {
			b, raw, _ := openFresh(t.Kids[2], map[string]bool{}, mangle)
			if alias[rootOf(t.Kids[1])] {
				alias[raw[0]] = true
			}
			walk(t.Kids[1])
			walk(b)
			return
		}
		if f.isKind(t, "table-set") && len(t.Kids) == 4 && alias[rootOf(t.Kids[1])] {
			if vs := f.prodLit(t.Kids[3]); vs != nil {
				if found != 0 && found != len(vs) {
					found = -1
					return
				}
				found = len(vs)
			}
		}
		if t.Kind == core.KFn {
			b, raw, _ := openFresh(t, map[string]bool{}, mangle)
			_ = raw
			walk(b)
			return
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(body)
	if found < 0 {
		return 0
	}
	return found
}
