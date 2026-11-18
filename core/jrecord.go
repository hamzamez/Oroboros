package core

import (
	"fmt"
	"sort"
	"strings"
)

// RECORDS IN J (ADR 0128, data.md §4).
//
// A record is a function on a static, finite set of labels L: its set is the
// dependent product Π_{l∈L} S_l, written `record { l ∈ S, … }`, and a value is
// its graph, `record { l : e, … }` (ADR 0061's pair). The reader writes four
// core forms, each an application of a name no program can write:
//
//	(#recset "l₁ … lₙ" S₁ … Sₙ)    the set, labels in canonical order
//	(#rec "l₁ … lₙ" e₁ … eₙ)       a value, its components in that order
//	(#get "l" r)                   projection, r.l
//	(#with "l" r e)                override at one label, r with { l : e }
//
// THE CANONICAL ORDER is the labels' spellings compared as sequences of
// Unicode scalars (data.md §2.5): a record is a function, which has no order
// on its domain, and the representation's bijection L ≅ {0 … n−1} is this one
// (data.md §4.4). Written order is erased here, so nothing after the reader
// can depend on it.
const (
	RecName    = "#rec"
	RecSetName = "#recset"
	GetName    = "#get"
	WithName   = "#with"
	// ZeroName is the language's zero of a record set, `zero(T)` (data.md
	// §4.6). It is not in Σ_L: like `some` and `none` it is found after a
	// module's own declarations, so a module may define its own `zero` (the
	// book's Church numerals do).
	ZeroName = "zero"
)

// recordFormer reads `record { l ∈ S, … }` or `record { l : e, … }`, after
// `record`. A former mixing the two is refused (X9), a repeated label (X1).
func (p *parser) recordFormer() (Expr, error) {
	p.next() // {
	p.open(true)
	type field struct {
		label string
		e     Expr
	}
	var fs []field
	kind := ""
	first := map[string]string{} // the first label of each kind, for X9
	seen := map[string]bool{}
	for !p.is("}") {
		t := p.next()
		if t.kind != tIdent {
			p.close()
			return nil, p.errorf("a record's label is an identifier, not %s (data.md §4.2): a label "+
				"computed at run time is a map's key", t)
		}
		l := Unspell(t.text)
		if seen[l] {
			p.close()
			return nil, p.errorf("the label %s appears twice in one record (data.md §4.2, X1): a "+
				"function has one value at each point", t.text)
		}
		seen[l] = true
		var k string
		switch {
		case p.accept("∈"):
			k = "∈"
		case p.accept(":"):
			k = ":"
		default:
			p.close()
			return nil, p.errorf("a record's label %s is followed by `∈ S` in a record set or by "+
				"`: e` in a record value (ADR 0128)", t.text)
		}
		if _, ok := first[k]; !ok {
			first[k] = t.text
		}
		if kind != "" && kind != k {
			p.close()
			return nil, p.errorf("a record former mixes a set's `%s ∈ S` and a value's `%s : e` "+
				"(ADR 0128, X9): `record { l ∈ S }` is a set, a family of claims, and `record { l : e }` "+
				"a value, its graph", first["∈"], first[":"])
		}
		kind = k
		e, err := p.expr()
		if err != nil {
			p.close()
			return nil, err
		}
		fs = append(fs, field{l, e})
		if !p.accept(",") {
			break
		}
	}
	p.close()
	if err := p.expect("}"); err != nil {
		return nil, err
	}
	if len(fs) == 0 {
		return nil, p.errorf("a record has at least one label (data.md §4.1): the product of no " +
			"factors is the unit, `()`")
	}
	// A VALUE'S COMPONENTS ARE EVALUATED IN THE ORDER WRITTEN: each is bound,
	// in that order, by a β-redex no program can name, and the record reads
	// the names in canonical order. Sorting the terms themselves would run
	// `record { b : f(), a : g() }`'s g first.
	var params []string
	var written []Expr
	if kind == ":" && len(fs) > 1 {
		for i := range fs {
			params = append(params, fmt.Sprintf("#rc%d", i))
			written = append(written, fs[i].e)
			fs[i].e = &Var{Name: params[i]}
		}
	}
	sort.SliceStable(fs, func(i, j int) bool { return Spell(fs[i].label) < Spell(fs[j].label) })
	labels := make([]string, len(fs))
	args := []Expr{nil}
	for i, f := range fs {
		labels[i] = f.label
		args = append(args, f.e)
	}
	args[0] = &Lit{T: Str(strings.Join(labels, " "))}
	name := RecName
	if kind == "∈" {
		name = RecSetName
	}
	var out Expr = &Apply{Fn: &Var{Name: name}, Args: args}
	if params != nil {
		out = &Apply{Fn: &Lam{Params: params, Body: out}, Args: written}
	}
	return out, nil
}

// projection is `e.l`, (#get "l" e).
func recordProjection(e Expr, label string) Expr {
	return &Apply{Fn: &Var{Name: GetName}, Args: []Expr{&Lit{T: Str(label)}, e}}
}

// withPath is base with { path : v }, one pair: the base and the value bound
// once, as override binds them.
func withPath(base Expr, path []string, v Expr) Expr {
	var up func(from Expr, path []string) Expr
	up = func(from Expr, path []string) Expr {
		var inner Expr = &Var{Name: "#wv0"}
		if len(path) > 1 {
			inner = up(recordProjection(from, path[0]), path[1:])
		}
		return &Apply{Fn: &Var{Name: WithName}, Args: []Expr{&Lit{T: Str(path[0])}, from, inner}}
	}
	return &Apply{Fn: &Lam{Params: []string{"#wb", "#wv0"}, Body: up(&Var{Name: "#wb"}, path)},
		Args: []Expr{base, v}}
}

// override reads `{ p : e, … }` after `with`, and builds base with the pairs
// (data.md §4.3). Its meaning is function override composed along each path,
// r with { l.m : e } = r with { l : r.l with { m : e } } (lenses compose).
//
// THE PATHS ARE A TRIE, AND THAT IS BOTH RULES AT ONCE. A family is
// prefix-free iff inserting each path into a trie ends at a fresh leaf, never
// at or through an existing one (X10), and a prefix-free family touches
// disjoint parts of the record, so its pairs commute. Building the update from
// the trie reads each nested component from the base once, so no term is
// copied however many paths share a prefix.
//
// THE BASE AND EACH VALUE ARE BOUND ONCE, in the order written, by a β-redex
// whose binders no program can name: the update reads only those names, so
// no computation is duplicated and the effects happen in J's order.
func (p *parser) override(base Expr) (Expr, error) {
	p.next() // {
	p.open(true)
	type node struct {
		label string
		leaf  Expr
		path  string
		kids  []*node
	}
	root := &node{}
	var vals []Expr
	for !p.is("}") {
		var path []string
		for {
			t := p.next()
			if t.kind != tIdent {
				p.close()
				return nil, p.errorf("a `with` names a label or a path of labels, `l` or `l.m`, not %s "+
					"(data.md §4.3)", t)
			}
			path = append(path, Unspell(t.text))
			if !p.accept(".") {
				break
			}
		}
		if err := p.expect(":"); err != nil {
			p.close()
			return nil, err
		}
		v, err := p.expr()
		if err != nil {
			p.close()
			return nil, err
		}
		spelled := make([]string, len(path))
		for i, l := range path {
			spelled[i] = Spell(l)
		}
		ps := strings.Join(spelled, ".")
		n := root
		for i, l := range path {
			var next *node
			for _, k := range n.kids {
				if k.label == l {
					next = k
				}
			}
			switch {
			case next == nil:
				next = &node{label: l}
				n.kids = append(n.kids, next)
			case next.leaf != nil || i == len(path)-1:
				other := next.path
				for m := next; other == "" && len(m.kids) > 0; m = m.kids[0] {
					other = m.kids[0].path
				}
				p.close()
				return nil, p.errorf("a `with`'s paths %s and %s are not prefix-free (data.md §4.3, X10): "+
					"one writes a part of the record the other writes, which is put–put; write one "+
					"update", other, ps)
			}
			n = next
		}
		n.leaf = &Var{Name: fmt.Sprintf("#wv%d", len(vals))}
		n.path = ps
		vals = append(vals, v)
		if !p.accept(",") {
			break
		}
	}
	p.close()
	if err := p.expect("}"); err != nil {
		return nil, err
	}
	if len(vals) == 0 {
		return nil, p.errorf("a `with` overrides at least one label (data.md §4.3): `r with { }` is r")
	}
	var update func(acc, from Expr, n *node) Expr
	update = func(acc, from Expr, n *node) Expr {
		for _, k := range n.kids {
			v := k.leaf
			if v == nil {
				part := recordProjection(from, k.label)
				v = update(part, part, k)
			}
			acc = &Apply{Fn: &Var{Name: WithName}, Args: []Expr{&Lit{T: Str(k.label)}, acc, v}}
		}
		return acc
	}
	params := []string{"#wb"}
	args := []Expr{base}
	for i, v := range vals {
		params = append(params, fmt.Sprintf("#wv%d", i))
		args = append(args, v)
	}
	b := &Var{Name: "#wb"}
	return &Apply{Fn: &Lam{Params: params, Body: update(b, b, root)}, Args: args}, nil
}
