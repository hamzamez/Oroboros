package emit

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
)

// THE TABLE TYPE: INTRODUCTION, ELIMINATION AND STORE (types.md §3.1).
//
// The checker's types are the scalars, the declared names, and two
// constructors: Table(σ), spelled `array σ` or `buffer σ`, and Map(κ, σ),
// spelled `map κ σ`. Every table and map form was typed UNKNOWN until
// tabletype-2026-09-29, and unknown agrees with everything, so a signature was
// never checked against a table-valued body: `(array 1.0 2.0)` under a result
// declared f64 compiled to a Go function returning []float64.
//
// The fix types each form by its CONSTRUCTOR and leaves the element unknown
// where the form does not state it: `?`. One rule joins the relation, for the
// unknown element only (compatible): Table(?) agrees with every table and every
// host type that realizes one; Map(?, ?) with every map. A known element is
// compared as it always was, invariantly.
//
// THE ELIMINATOR AND THE STORE (tableelim-2026-09-29). A read (a i) has type σ
// and (len a) has type int; a store demands its value at σ. An open element is
// a UNIFICATION VARIABLE, not a wildcard: the first store or read solves it, on
// the buffer's root name. Two open elements: `?`, nothing known, and `?int`,
// the element sort ℤ with its realization open — which realization holds an
// integer is the IR's (ADR 0033), so a store of 104 must not make a buffer that
// hex.Encode's (buffer (int 0 255)) then refuses.
//
// Open elements never reach the IR. The IR types tables by unification and W5
// reads this relation over the IR's types, which name each element or say
// `any`, so the relation W5 reads is unchanged on every input it sees — the
// lesson of ADR 0033, where relaxing a relation for one check weakened the
// verifier. The one rule for KNOWN elements, a language table against a host
// type that realizes it, compares host types, which W5's last step already does.

const (
	unknownElem = "?"
	openInt     = "?int" // the sort ℤ, realization open
)

var (
	arrayOfUnknown  = "array " + unknownElem
	bufferOfUnknown = "buffer " + unknownElem
	mapOfUnknown    = "map " + unknownElem + " " + unknownElem
)

// isOpen reports an open element: a unification variable.
func isOpen(e string) bool { return e == unknownElem || e == openInt }

// tableParts splits a language table type into its constructor and element.
func tableParts(ty string) (cons, elem string, ok bool) {
	for _, c := range []string{"array ", "buffer "} {
		if strings.HasPrefix(ty, c) {
			return c[:len(c)-1], ty[len(c):], true
		}
	}
	return "", "", false
}

// openTable reports a table or map type whose element is open.
func openTable(ty string) bool {
	if ty == mapOfUnknown {
		return true
	}
	_, e, ok := tableParts(ty)
	return ok && isOpen(e)
}

// elemAgrees is the relation on elements where one may be open: `?` agrees
// with anything, `?int` with any element of the integer sort.
func (tg *Target) elemAgrees(x, y string) bool {
	switch {
	case x == unknownElem || y == unknownElem || x == "" || y == "":
		return true
	case x == openInt:
		return y == openInt || isIntSort(tg, y)
	case y == openInt:
		return isIntSort(tg, x)
	}
	return compatible(tg, x, y)
}

// openAgrees is compatible's rule for a type with an open element: it agrees
// with its own constructor, element by element, and with nothing else.
func (tg *Target) openAgrees(a, b string) bool {
	if a == mapOfUnknown || b == mapOfUnknown {
		return isMapType(a) && isMapType(b)
	}
	ea, oka := tg.tableElem(a)
	eb, okb := tg.tableElem(b)
	return oka && okb && tg.elemAgrees(ea, eb)
}

// realizesTable reports that host is a host type that ρ_T gives the same
// realization as the language table lang: Go's `slice-float64` and `(array
// f64)` are both []float64. It compares host types, as W5's last step does.
func (tg *Target) realizesTable(lang, host string) bool {
	_, e, ok := tableParts(lang)
	if !ok || isOpen(e) || core.ArrayElem(host) != "" {
		return false
	}
	h := tg.HostType(host)
	return h != "" && !strings.HasPrefix(h, "/*") && tg.HostType("array "+e) == h
}

// elemDemand is what an element asks of a value: nothing for `?`, the sort
// for `?int`.
func elemDemand(e string) string {
	switch e {
	case unknownElem:
		return ""
	case openInt:
		return "int"
	}
	return e
}

// joinElems is the join of a graph's entries or a rule's body (types.md §3.1):
// their common type, `?int` if all are integers, `?` if they disagree.
func (c *checker) joinElems(tys []string) string {
	j := ""
	for _, t := range tys {
		switch {
		case t == "" || t == "any":
			return unknownElem
		case isIntSort(c.tgt, t):
			t = openInt
		}
		if j == "" {
			j = t
		} else if j != t {
			return unknownElem
		}
	}
	if j == "" {
		return unknownElem
	}
	return j
}

// solve fixes an open element of name's table type by a type learned for it:
// a store's value or a read's demand. An integer solves `?` to `?int`, never
// to a realization; `?int` is not solved further.
func (c *checker) solve(name, learned string) {
	if name == "" || learned == "" || learned == "any" || isOpen(learned) {
		return
	}
	cons, e, ok := tableParts(c.types[name])
	if !ok || e != unknownElem {
		return
	}
	if isIntSort(c.tgt, learned) {
		learned = openInt
	}
	c.types[name] = cons + " " + learned
}

// read types an application of a table, (a i) : σ (types.md §3.1). An open
// `?` is solved by the read's demand; `?int` reads as the sort.
func (c *checker) read(name, e string, idx *core.Term, want string) (string, error) {
	if _, err := c.walk(idx, "int"); err != nil {
		return "", fmt.Errorf("in an index of %s: %w", name, err)
	}
	what := "(" + name + " …)"
	switch e {
	case "", unknownElem:
		c.solve(name, want)
		return "", nil
	case openInt:
		return "int", c.agree(what, "int", want)
	}
	return e, c.agree(what, e, want)
}

// isMapType reports a map type.
func isMapType(ty string) bool {
	_, _, ok := core.MapTypes(ty)
	return ok
}

// tableElem reports whether ty is a table type and gives its element: a
// language table's or buffer's, or a host type's by ρ_T's kernel — Go's
// `slice-float64` is realized as []float64 = ρ(array f64), so it is a table of
// f64. The inverse is taken over a finite candidate set, and an element is
// given only when it is unique; the IR's unifier inverts an alias the same way
// (ir/typing.go aliasOf).
func (tg *Target) tableElem(ty string) (string, bool) {
	if e := core.ArrayElem(ty); e != "" {
		return e, true
	}
	h := tg.HostType(ty)
	if h == "" || strings.HasPrefix(h, "/*") {
		return "", false
	}
	var elems []string
	for _, e := range tg.elemCandidates() {
		if e != ty && tg.HostType("array "+e) == h {
			elems = append(elems, e)
		}
	}
	if len(elems) == 0 {
		return "", false
	}
	for _, e := range elems[1:] {
		if tg.HostType(e) != tg.HostType(elems[0]) {
			return "", true // two readings: a table, with no one element
		}
	}
	return elems[0], true
}

// elemCandidates are the element types an alias is inverted over: the
// language's scalars and every type the target declares, in a fixed order so
// the checker is a function of its input.
func (tg *Target) elemCandidates() []string {
	out := []string{"int", "f64", "bool", "string"}
	hosts := make([]string, 0, len(tg.Types))
	for k := range tg.Types {
		hosts = append(hosts, k)
	}
	sort.Strings(hosts)
	return append(out, hosts...)
}

// sameConstructor reports that other is of the constructor u names: a map for
// a map, a table or table alias otherwise.
func (tg *Target) sameConstructor(u, other string) bool {
	if isMapType(u) {
		return isMapType(other)
	}
	_, ok := tg.tableElem(other)
	return ok
}

// constructorWord names an open type's constructor in a refusal.
func constructorWord(ty string) string {
	switch {
	case strings.HasPrefix(ty, "buffer "):
		return "buffer"
	case isMapType(ty):
		return "map"
	}
	return "table"
}

// demandedElem is the element a demand asks a form's entries to have: the
// demanded table's (or map's value) when it is known, and nothing otherwise.
func (c *checker) demandedElem(want string, isMap bool) string {
	var e string
	if isMap {
		_, e, _ = core.MapTypes(want)
	} else {
		e, _ = c.tgt.tableElem(want)
	}
	return elemDemand(e)
}

// formValue is a table or map form's type against a demand: the demand
// itself when it is of the form's constructor — its entries were checked
// against the demand's element — and otherwise the form's own type, which
// agrees only with its own constructor.
func (c *checker) formValue(what, own, want string) (string, error) {
	if want != "" && want != "any" && c.tgt.sameConstructor(own, want) {
		return want, nil
	}
	return own, c.agree(what, own, want)
}

// tableForm types the language's table and map forms (types.md §3.1). It
// reports handled=false for a kind that is not one, or a form of an arity
// the reducer does not build, which the walk then treats as before.
func (c *checker) tableForm(kind string, args []*core.Term, want string) (ty string, handled bool, err error) {
	lambda := func(t *core.Term) bool { return t.Kind == core.KFn && len(t.Params) == 1 }
	switch kind {
	case "array": // a graph: every entry at the demanded element
		what := "(array …)"
		elem := c.demandedElem(want, false)
		tys := make([]string, len(args))
		for i, a := range args {
			t, err := c.walk(a, elem)
			if err != nil {
				return "", true, fmt.Errorf("in entry %d of a table: %w", i+1, err)
			}
			tys[i] = t
		}
		ty, err := c.formValue(what, "array "+c.joinElems(tys), want)
		return ty, true, err

	case "table": // a rule: its body at the demanded element, for every i
		if len(args) != 2 || !lambda(args[1]) {
			return "", false, nil
		}
		what := "(table …)"
		if _, err := c.walk(args[0], "int"); err != nil {
			return "", true, fmt.Errorf("in a table's length: %w", err)
		}
		restore := c.bind(args[1].Params, []string{"int"})
		body, err := c.walk(args[1].Body(), c.demandedElem(want, false))
		restore()
		if err != nil {
			return "", true, fmt.Errorf("in a table's rule: %w", err)
		}
		ty, err := c.formValue(what, "array "+c.joinElems([]string{body}), want)
		return ty, true, err

	case "table-alloc":
		if len(args) != 1 {
			return "", false, nil
		}
		if _, err := c.walk(args[0], ""); err != nil {
			return "", true, err
		}
		ty, err := c.formValue("(alloc …)", bufferOfUnknown, want)
		return ty, true, err

	case "table-build", "map-build": // the scope's value is its body's
		if len(args) != 2 || !lambda(args[1]) {
			return "", false, nil
		}
		what, bound := "a build's size", bufferOfUnknown
		if kind == "map-build" {
			what, bound = "a build-map's capacity", mapOfUnknown
		}
		if _, err := c.walk(args[0], "int"); err != nil {
			return "", true, fmt.Errorf("in %s: %w", what, err)
		}
		defer c.bind(args[1].Params, []string{bound})()
		// A TABLE DEMAND IS MET BY THE FROZEN VALUE, at the exit: inside, the
		// body gives back a live `buffer σ`, or a buffer-free table, and the
		// freeze is what makes the first an `array σ`. Any other demand is
		// passed in, since freezing does not change it.
		// So is a demand with a type below it, an interface or the empty
		// interface (types.md §3.3): what meets it is the frozen table, not
		// the live buffer inside, so it is checked at the exit.
		inner := want
		if _, table := c.tgt.tableElem(want); table || c.tgt.HasSubtypes(c.tgt.ValueType(want)) {
			inner = ""
		}
		ty, err := c.walk(args[1].Body(), inner)
		if err != nil {
			return "", true, err
		}
		ty = frozen(ty)
		return ty, true, c.agree("the scope's value", ty, want)

	case "table-set", "map-insert": // a store gives back its buffer
		if len(args) != 3 {
			return "", false, nil
		}
		buf, err := c.walk(args[0], "")
		if err != nil {
			return "", true, err
		}
		key, val, store := "int", "", "(set …)"
		if kind == "map-insert" {
			store = "(insert …)"
			key = ""
			if k, v, ok := core.MapTypes(buf); ok && k != unknownElem {
				key, val = k, v
			}
		} else if e, ok := c.tgt.tableElem(buf); ok {
			val = elemDemand(e)
		}
		if _, err := c.walk(args[1], key); err != nil {
			return "", true, fmt.Errorf("in a store's key: %w", err)
		}
		vt, err := c.walk(args[2], val)
		if err != nil {
			return "", true, fmt.Errorf("in a store's value: %w", err)
		}
		// THE STORE SOLVES AN OPEN ELEMENT, on the buffer's root name, so the
		// next store is checked against this one (types.md §3.1).
		if kind == "table-set" {
			if root := BufferRoot(args[0]); root != "" {
				c.solve(root, vt)
				if t := c.types[root]; t != "" {
					if _, _, ok := tableParts(t); ok {
						buf = t
					}
				}
			}
		}
		return buf, true, c.agree(store, buf, want)

	case "map": // a map literal
		for _, a := range args {
			if _, err := c.walk(a, ""); err != nil {
				return "", true, err
			}
		}
		ty, err := c.formValue("(map …)", mapOfUnknown, want)
		return ty, true, err

	case "len": // the domain bound: len : Table(σ) → int
		if len(args) != 1 {
			return "", false, nil
		}
		if _, err := c.walk(args[0], ""); err != nil {
			return "", true, err
		}
		return "int", true, c.agree("(len …)", "int", want)

	case "map-keys":
		if len(args) != 1 {
			return "", false, nil
		}
		if _, err := c.walk(args[0], ""); err != nil {
			return "", true, err
		}
		ty, err := c.formValue("(keys …)", arrayOfUnknown, want)
		return ty, true, err
	}
	return "", false, nil
}

// frozen is a scope's value as it leaves the scope: its buffers FROZEN, so a
// `buffer σ` is an `array σ` (ADR 0031, tables.md §2.5). A tuple's components
// are frozen one by one where the pattern takes it apart, since each
// projection is walked as a scope of its own.
func frozen(ty string) string {
	if strings.HasPrefix(ty, "buffer ") {
		return "array " + ty[len("buffer "):]
	}
	return ty
}

// joinOf is the more informative of two agreeing branch types: a known one
// over unknown or `any`, and a table with a known element over one without.
func joinOf(a, b string) string {
	if a == "" || a == "any" || openTable(a) && b != "" && b != "any" {
		return b
	}
	return a
}
