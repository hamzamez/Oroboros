package emit

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
)

// A TABLE'S CONSTRUCTOR IS TYPED WHERE ITS ELEMENT MAY NOT BE (types.md §3.1).
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
// `?` never reaches the IR. The IR types tables by unification and W5 reads
// this relation over the IR's types, which name each element or say `any`, so
// the relation W5 reads is unchanged on every input it sees — the lesson of
// ADR 0033, where relaxing a relation for one check weakened the verifier.

const unknownElem = "?"

var (
	arrayOfUnknown  = "array " + unknownElem
	bufferOfUnknown = "buffer " + unknownElem
	mapOfUnknown    = "map " + unknownElem + " " + unknownElem
)

// unknownTable reports a table or map type whose element is unknown.
func unknownTable(ty string) bool {
	return ty == arrayOfUnknown || ty == bufferOfUnknown || ty == mapOfUnknown
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

// sameConstructor reports that other is of the constructor u, a type with an
// unknown element, names: a map for a map, a table or table alias otherwise.
func (tg *Target) sameConstructor(u, other string) bool {
	if u == mapOfUnknown {
		return isMapType(other)
	}
	_, ok := tg.tableElem(other)
	return ok
}

// constructorWord names an unknown-element type's constructor in a refusal.
func constructorWord(ty string) string {
	switch ty {
	case bufferOfUnknown:
		return "buffer"
	case mapOfUnknown:
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
	if e == unknownElem {
		return ""
	}
	return e
}

// formValue is a table or map form's type against a demand: the demand
// itself when it is of the form's constructor — its entries were checked
// against the demand's element — and otherwise the constructor with an
// unknown element, which agrees only with its own constructor.
func (c *checker) formValue(what, unknown, want string) (string, error) {
	if want != "" && want != "any" && c.tgt.sameConstructor(unknown, want) {
		return want, nil
	}
	return unknown, c.agree(what, unknown, want)
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
		for i, a := range args {
			if _, err := c.walk(a, elem); err != nil {
				return "", true, fmt.Errorf("in entry %d of a table: %w", i+1, err)
			}
		}
		ty, err := c.formValue(what, arrayOfUnknown, want)
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
		_, err := c.walk(args[1].Body(), c.demandedElem(want, false))
		restore()
		if err != nil {
			return "", true, fmt.Errorf("in a table's rule: %w", err)
		}
		ty, err := c.formValue(what, arrayOfUnknown, want)
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
		inner := want
		if _, table := c.tgt.tableElem(want); table {
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
		} else if e, ok := c.tgt.tableElem(buf); ok && e != unknownElem {
			val = e
		}
		if _, err := c.walk(args[1], key); err != nil {
			return "", true, fmt.Errorf("in a store's key: %w", err)
		}
		if _, err := c.walk(args[2], val); err != nil {
			return "", true, fmt.Errorf("in a store's value: %w", err)
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
	if a == "" || a == "any" || unknownTable(a) && b != "" && b != "any" {
		return b
	}
	return a
}
