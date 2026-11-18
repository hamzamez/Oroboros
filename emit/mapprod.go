package emit

import (
	"fmt"
	"strings"

	"oroboros/core"
)

// A PRODUCT AS A MAP'S VALUE IS ITS COMPONENTS (ADR 0128 item 7, data.md §3.3).
//
// A map is a partial function m : K ⇀ V, and for V = V₁ × … × Vₙ its value at
// a key is n components. The IR says so directly: `insert m k v₁ … vₙ` stores
// them and `read m k` yields the tag and v₁ … vₙ, the option's tag and its n
// slots (ADR 0042), so each backend holds the components the host's way, Go a
// struct, the JVM a record, JavaScript an array, and windows n cells of a
// slot (ADR 0120). A table of products is flattened into ordinary tables
// before any backend reads it (product.go); a map is not, because the host's
// own map holds a struct where hand-written code would put one, and n maps
// with one domain would probe n times.
//
// The rewrite, after the checker and before lowering, which binds one value
// per name:
//
//	(insert m k (tuple v₁ … vₙ))     ->  (insert m k v₁ … vₙ)
//	((m k) (fn (t p) B))              ->  ((m k) (fn (t p₁ … pₙ) B[p := (tuple p₁ … pₙ)]))
//
// and in B each destructuring of p, (p (fn (x₁ … xₙ) M)), is the β-redex it
// becomes, M[xᵢ := pᵢ]. A map's arity is read off its inserts, followed
// through every name that holds a version of it, as a table's is.

// FlattenMapProducts rewrites the maps of products in t and reports how many
// operations it rewrote. It returns t itself when nothing fired.
func FlattenMapProducts(tgt *Target, t *core.Term) (*core.Term, int, error) {
	if PlantMapProductsUnflattened || !anyMapProduct(tgt, t) {
		return t, 0, nil
	}
	f := &mapFlattener{tgt: tgt, arity: map[string]int{}}
	out, err := f.walk(t)
	if err != nil || f.n == 0 {
		return t, 0, err
	}
	return out, f.n, nil
}

// PlantMapProductsUnflattened leaves a map of products to lowering, which
// refuses it as several values where one is wanted
// (TestAProductIsAMapsValue).
var PlantMapProductsUnflattened bool

type mapFlattener struct {
	tgt   *Target
	arity map[string]int // a map's name -> its value's arity
	n     int
}

func isPrimKind(tgt *Target, t *core.Term, kind string) bool {
	if t == nil || t.Kind != core.KApp || len(t.Kids) == 0 || t.Kids[0].Kind != core.KName {
		return false
	}
	p, ok := tgt.Prims[t.Kids[0].Name]
	return ok && p.Kind == kind
}

// tupleParts is (tuple v₁ … vₙ)'s components, n ≥ 2, or nil.
func tupleParts(t *core.Term) []*core.Term {
	if t == nil || t.Kind != core.KFn || len(t.Params) != 1 {
		return nil
	}
	b := t.Body()
	if b.Kind != core.KApp || len(b.Args()) < 2 {
		return nil
	}
	parts, _ := multiValue(t, len(b.Args()))
	return parts
}

// anyMapProduct is the cheap test: an insert of a tuple is the only way a map
// of products is made.
func anyMapProduct(tgt *Target, t *core.Term) bool {
	if t == nil {
		return false
	}
	if isPrimKind(tgt, t, "map-insert") && len(t.Kids) == 4 && tupleParts(t.Kids[3]) != nil || literalArity(tgt, t) > 0 {
		return true
	}
	if t.Kind == core.KFn {
		return anyMapProduct(tgt, t.Closed())
	}
	for _, k := range t.Kids {
		if anyMapProduct(tgt, k) {
			return true
		}
	}
	return false
}

// mapRoot is the map a term is a version of: a name, or an insert into one.
func (f *mapFlattener) mapRoot(t *core.Term) string {
	for t != nil {
		if t.Kind == core.KName {
			return t.Name
		}
		if isPrimKind(f.tgt, t, "map-insert") && len(t.Kids) >= 4 {
			t = t.Kids[1]
			continue
		}
		// a contract mark's value is its body's: an insert carries its
		// capacity's `where` until the marks are decided (ADR 0121)
		if core.IsRequireWhere(t) {
			t = t.Kids[3]
			continue
		}
		if core.IsRequire(t) {
			t = t.Kids[4]
			continue
		}
		return ""
	}
	return ""
}

// arityOf is the arity a term carries: a map's name, an insert chain rooted at
// one, a `build-map` that inserts products, or a loop threading one.
func (f *mapFlattener) arityOf(t *core.Term) int {
	if t == nil {
		return 0
	}
	if root := f.mapRoot(t); root != "" {
		return f.arity[root]
	}
	if isPrimKind(f.tgt, t, "iterate") && len(t.Kids) >= 2 {
		for _, z := range t.Kids[2:] {
			if a := f.arityOf(z); a > 0 {
				return a
			}
		}
		// a loop that starts from an empty map fills it in its body: the
		// arity is its variable's, read off the inserts there
		if t.Kids[1].Kind == core.KFn {
			body, raw, _ := openFresh(t.Kids[1], map[string]bool{}, mangle)
			for i, z := range t.Kids[2:] {
				if i < len(raw) && f.mapRoot(z) != "" {
					if a := f.inserted(body, raw[i]); a > 0 {
						return a
					}
				}
			}
		}
	}
	if isPrimKind(f.tgt, t, "map-build") && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1 {
		body, raw, _ := openFresh(t.Kids[2], map[string]bool{}, mangle)
		return f.inserted(body, raw[0])
	}
	return literalArity(f.tgt, t)
}

// literalArity is a map literal's value arity, when every row's value is a
// tuple of one arity n ≥ 2; 0 otherwise.
func literalArity(tgt *Target, t *core.Term) int {
	if !isPrimKind(tgt, t, "map") || len(t.Kids) < 2 {
		return 0
	}
	n := 0
	for _, row := range t.Kids[1:] {
		if row.Kind != core.KApp || len(row.Kids) != 2 {
			return 0
		}
		vs := tupleParts(row.Kids[1])
		if len(vs) < 2 || n != 0 && len(vs) != n {
			return 0
		}
		n = len(vs)
	}
	return n
}

// inserted reads a map's value arity off the inserts into it and into every
// name that holds a version of it: a `let`'s binder, a loop's variable. Two
// arities into one map are no product, and answer 0.
func (f *mapFlattener) inserted(body *core.Term, name string) int {
	alias := map[string]bool{name: true}
	found := 0
	var walk func(*core.Term)
	walk = func(t *core.Term) {
		if t == nil || found < 0 {
			return
		}
		if isPrimKind(f.tgt, t, "iterate") && len(t.Kids) >= 2 && t.Kids[1].Kind == core.KFn {
			b, raw, _ := openFresh(t.Kids[1], map[string]bool{}, mangle)
			for i, z := range t.Kids[2:] {
				if i < len(raw) && alias[f.mapRoot(z)] {
					alias[raw[i]] = true
				}
				walk(z)
			}
			walk(b)
			return
		}
		if t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == "let" &&
			t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1 {
			b, raw, _ := openFresh(t.Kids[2], map[string]bool{}, mangle)
			if alias[f.mapRoot(t.Kids[1])] {
				alias[raw[0]] = true
			}
			walk(t.Kids[1])
			walk(b)
			return
		}
		if isPrimKind(f.tgt, t, "map-insert") && len(t.Kids) == 4 && alias[f.mapRoot(t.Kids[1])] {
			n := 1
			if vs := tupleParts(t.Kids[3]); vs != nil {
				n = len(vs)
			}
			if found != 0 && found != n {
				found = -1
				return
			}
			found = n
		}
		if t.Kind == core.KFn {
			b, _, _ := openFresh(t, map[string]bool{}, mangle)
			walk(b)
			return
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(body)
	if found < 2 {
		return 0
	}
	return found
}

// bind runs walk on a λ's body with its i-th binder holding a map of arity k.
func (f *mapFlattener) bindWalk(lam *core.Term, ks map[int]int) (*core.Term, error) {
	body, raw, _ := openFresh(lam, map[string]bool{}, mangle)
	for i, k := range ks {
		f.arity[raw[i]] = k
	}
	out, err := f.walk(body)
	for i := range ks {
		delete(f.arity, raw[i])
	}
	if err != nil {
		return nil, err
	}
	return core.Fn(raw, out), nil
}

func (f *mapFlattener) walk(t *core.Term) (*core.Term, error) {
	if t == nil {
		return nil, nil
	}
	// AN INSERT OF A PRODUCT: its components are the insert's values.
	if isPrimKind(f.tgt, t, "map-insert") && len(t.Kids) == 4 {
		if k := f.arityOf(t.Kids[1]); k > 0 {
			vs := tupleParts(t.Kids[3])
			if len(vs) != k {
				return nil, fmt.Errorf("a map whose value is a %d-component product is written with a "+
					"tuple of %d, `m[k] := (v1, …, v%d)`", k, k, k)
			}
			kids := []*core.Term{t.Kids[0]}
			for _, z := range append([]*core.Term{t.Kids[1], t.Kids[2]}, vs...) {
				zz, err := f.walk(z)
				if err != nil {
					return nil, err
				}
				kids = append(kids, zz)
			}
			f.n++
			return core.App(kids[0], kids[1:]...), nil
		}
	}
	// A READ UNDER ITS ELIMINATOR, ((m k) (fn (t p) B)): the payload is n slots.
	if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[1].Kind == core.KFn && len(t.Kids[1].Params) == 2 {
		op := t.Kids[0]
		if op.Kind == core.KApp && len(op.Kids) == 2 {
			if k := f.arityOf(op.Kids[0]); k > 0 {
				m, err := f.walk(op.Kids[0])
				if err != nil {
					return nil, err
				}
				key, err := f.walk(op.Kids[1])
				if err != nil {
					return nil, err
				}
				body, raw, _ := openFresh(t.Kids[1], map[string]bool{}, mangle)
				slots := make([]*core.Term, k)
				params := []string{raw[0]}
				for j := range slots {
					nm := mapSlot(raw[1], j)
					slots[j] = core.Name(nm)
					params = append(params, nm)
				}
				b, err := f.walk(substTuple(body, raw[1], slots))
				if err != nil {
					return nil, err
				}
				f.n++
				return core.App(core.App(m, key), core.Fn(params, b)), nil
			}
		}
	}
	// A LITERAL OF PRODUCTS IS THE MAP ITS INSERTS BUILD, map {k₁ : v₁, …} =
	// build-map(c){m. insert(… insert(m, k₁, v₁) …)}, c its row count (its
	// capacity is its size, maps.md §6); each insert is then one above.
	if k := literalArity(f.tgt, t); k > 0 && !PlantMapProductsUnflattened {
		rows := t.Kids[1:]
		bm := "build-map"
		for n, p := range f.tgt.Prims {
			if p.Kind == "map-build" {
				bm = n
				break
			}
		}
		ins := "insert"
		for n, p := range f.tgt.Prims {
			if p.Kind == "map-insert" {
				ins = n
				break
			}
		}
		names := make([]string, len(rows)+1)
		for i := range names {
			names[i] = fmt.Sprintf("#lit·%d", i)
		}
		body := core.Name(names[len(rows)])
		for i := len(rows) - 1; i >= 0; i-- {
			body = core.App(core.Name("let"),
				core.App(core.Name(ins), core.Name(names[i]), rows[i].Kids[0], rows[i].Kids[1]),
				core.Fn([]string{names[i+1]}, body))
		}
		return f.walk(core.App(core.Name(bm), core.Int(int64(len(rows))), core.Fn([]string{names[0]}, body)))
	}
	// A BUILD-MAP OF PRODUCTS: its binder holds the map in its body.
	if isPrimKind(f.tgt, t, "map-build") && len(t.Kids) == 3 && t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1 {
		if k := f.arityOf(t); k > 0 {
			c, err := f.walk(t.Kids[1])
			if err != nil {
				return nil, err
			}
			lam, err := f.bindWalk(t.Kids[2], map[int]int{0: k})
			if err != nil {
				return nil, err
			}
			return core.App(t.Kids[0], c, lam), nil
		}
	}
	// A LOOP'S VARIABLE AND A LET'S BINDER CARRY A MAP'S ARITY with them.
	if isPrimKind(f.tgt, t, "iterate") && len(t.Kids) >= 2 && t.Kids[1].Kind == core.KFn {
		ks := map[int]int{}
		for i, z := range t.Kids[2:] {
			if a := f.arityOf(z); a > 0 {
				ks[i] = a
			}
		}
		if len(ks) == 0 {
			// an empty map filled by the loop: each variable's arity off its inserts
			body, raw, _ := openFresh(t.Kids[1], map[string]bool{}, mangle)
			for i, z := range t.Kids[2:] {
				if i < len(raw) && f.mapRoot(z) != "" {
					if a := f.inserted(body, raw[i]); a > 0 {
						ks[i] = a
					}
				}
			}
		}
		if len(ks) > 0 {
			lam, err := f.bindWalk(t.Kids[1], ks)
			if err != nil {
				return nil, err
			}
			kids := []*core.Term{t.Kids[0], lam}
			for _, z := range t.Kids[2:] {
				zz, err := f.walk(z)
				if err != nil {
					return nil, err
				}
				kids = append(kids, zz)
			}
			return core.App(kids[0], kids[1:]...), nil
		}
	}
	if t.Kind == core.KApp && len(t.Kids) == 3 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == "let" &&
		t.Kids[2].Kind == core.KFn && len(t.Kids[2].Params) == 1 {
		if k := f.arityOf(t.Kids[1]); k > 0 {
			v, err := f.walk(t.Kids[1])
			if err != nil {
				return nil, err
			}
			lam, err := f.bindWalk(t.Kids[2], map[int]int{0: k})
			if err != nil {
				return nil, err
			}
			return core.App(t.Kids[0], v, lam), nil
		}
	}
	if t.Kind == core.KFn {
		body, raw, _ := openFresh(t, map[string]bool{}, mangle)
		out, err := f.walk(body)
		if err != nil {
			return nil, err
		}
		if out == body {
			return t, nil
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

// A SLOT'S NAME is the payload's with `·j`, which no program can write: it is
// how lowering tells a read of n slots from a tuple bound to n + 1 names.
func mapSlot(p string, j int) string { return fmt.Sprintf("%s·%d", p, j) }

// IsMapSlots reports binders a map read's slots were given (mapSlot).
func IsMapSlots(names []string) bool {
	for j, n := range names {
		if !strings.HasSuffix(n, fmt.Sprintf("·%d", j)) {
			return false
		}
	}
	return len(names) > 0
}

// substTuple replaces the name p in t by the tuple of slots, and each
// destructuring (p (fn (x₁ … xₙ) M)) by M with xᵢ the slots, the β-redex it
// is. p's other occurrences become the tuple value itself.
func substTuple(t *core.Term, p string, slots []*core.Term) *core.Term {
	var walk func(*core.Term) *core.Term
	walk = func(t *core.Term) *core.Term {
		if t == nil {
			return nil
		}
		if t.Kind == core.KName && t.Name == p {
			sel := "#k"
			return core.Fn([]string{sel}, core.App(core.Name(sel), slots...))
		}
		if t.Kind == core.KApp && len(t.Kids) == 2 && t.Kids[0].Kind == core.KName && t.Kids[0].Name == p &&
			t.Kids[1].Kind == core.KFn && len(t.Kids[1].Params) == len(slots) {
			return walk(t.Kids[1].OpenWith(slots))
		}
		if len(t.Kids) == 0 {
			return t
		}
		kids := make([]*core.Term, len(t.Kids))
		changed := false
		for i, k := range t.Kids {
			kids[i] = walk(k)
			if kids[i] != k {
				changed = true
			}
		}
		if !changed {
			return t
		}
		out := *t
		out.Kids = kids
		return &out
	}
	return walk(t)
}
