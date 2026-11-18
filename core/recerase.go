package core

import (
	"fmt"
	"strings"
)

// WHERE LABELS BECOME POSITIONS ON THE RESIDUAL (ADR 0128, data.md §4.7).
//
// The reducer erases every record it sees built: the δ-rules at a value, and
// the currying isomorphism at each binder a record reaches (a let, β, a loop's
// variable, a header). What it cannot see is a record stored as data, a table
// element read back through `t[i].l`: there the record's labels are its
// table's, known from the table's rule. So the residual, closed, first-order
// and monomorphic (ADR 0036), is walked once with each term's SHAPE, the label
// tree its record set has, computed forward from its values and the names they
// are bound to, and the bijection L ≅ {0 … n−1} is applied:
//
//	(#rec L e…)       the tuple of its leaves, nested records flattened by
//	                  associativity, A × (B × C) ≅ A × B × C
//	(#get l r)        r's leaf at l's position, or the tuple of l's leaves
//	(#with l r x)     the tuple of r's leaves with l's replaced by x's
//
// Then the residual is reduced again, so the tuple's own rules (β-tab, product
// flattening, a loop's exits) apply as to any tuple. A projection whose
// operand has no known shape is refused, saying where.

// shape is a term's record structure: a record's labels and each component's
// shape, or a table's element shape. nil is no record.
type shape struct {
	labels []string
	comps  []*shape
	elem   *shape
}

func (s *shape) record() bool { return s != nil && s.labels != nil }

// leaves is how many leaves the flattened record has.
func (s *shape) leaves() int {
	if !s.record() {
		return 1
	}
	n := 0
	for _, c := range s.comps {
		n += c.leaves()
	}
	return n
}

// offset is the first leaf of label l, and its component's shape.
func (s *shape) offset(l string) (int, *shape, bool) {
	o := 0
	for i, m := range s.labels {
		if m == l {
			return o, s.comps[i], true
		}
		o += s.comps[i].leaves()
	}
	return 0, nil, false
}

func sameShape(a, b *shape) bool {
	switch {
	case a == nil || b == nil:
		return a == b
	case strings.Join(a.labels, " ") != strings.Join(b.labels, " ") || len(a.comps) != len(b.comps):
		return false
	case !sameShape(a.elem, b.elem):
		return false
	}
	for i := range a.comps {
		if !sameShape(a.comps[i], b.comps[i]) {
			return false
		}
	}
	return true
}

// containsRecordForms reports a residual holding a record's form.
func containsRecordForms(t *Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName {
		switch t.Kids[0].Name {
		case RecName, GetName, WithName, RecSetName:
			return true
		}
	}
	for _, k := range t.Kids {
		if containsRecordForms(k) {
			return true
		}
	}
	return false
}

type eraser struct {
	e     *Env
	env   map[string]*shape
	fresh int
	// infer is the first passes: shapes are collected, nothing refused, until
	// a pass learns nothing (a least fixed point: a loop variable's shape is
	// its initializer's joined with what its loop's agains pass, and a table's
	// with what is stored into it)
	infer   bool
	changed bool
	loops   [][]string // the enclosing loops' variables, innermost last
}

// learn records n's shape where it had none.
func (x *eraser) learn(n string, s *shape) {
	if s == nil || s == noTail || x.env[n] != nil {
		return
	}
	x.env[n], x.changed = s, true
}

// noTail is the shape of a jump, which has no value: an `again`.
var noTail = &shape{}

func join(a, b *shape) (*shape, bool) {
	switch {
	case a == noTail:
		return b, true
	case b == noTail:
		return a, true
	case sameShape(a, b):
		return a, true
	}
	return nil, false
}

func (x *eraser) name() string {
	x.fresh++
	return fmt.Sprintf("#re%d", x.fresh)
}

// leavesOf is the leaves of a record term r of shape s, r a name: its
// projections at each leaf position.
func leavesOf(r *Term, s *shape) []*Term {
	out := make([]*Term, s.leaves())
	for i := range out {
		out[i] = App(r, Int(int64(i)))
	}
	return out
}

func tupleOf(cs []*Term) *Term {
	return Fn([]string{TupleBinder}, App(Name(TupleBinder), cs...))
}

// bind is `let n = v in k(n)`, or k(v) when v is a name.
func (x *eraser) bind(v *Term, k func(*Term) *Term) *Term {
	if v.Kind == KName {
		return k(v)
	}
	n := x.name()
	return App(Name("let"), v, Fn([]string{n}, k(Name(n))))
}

func (x *eraser) walk(t *Term) (*Term, *shape, error) {
	switch t.Kind {
	case KName:
		return t, x.env[t.Name], nil
	case KFn:
		b, _, err := x.walk(openNamed(t))
		if err != nil {
			return nil, nil, err
		}
		return Fn(t.Params, b), nil, nil
	case KApp:
	default:
		return t, nil, nil
	}
	op := t.Kids[0]
	// A MAP READ UNDER ITS ELIMINATOR, ((m k) (fn (t p) B)), of a map whose
	// values are records: the payload p has the map's element shape.
	if t.Kids[0].Kind == KApp && len(t.Kids) == 2 && t.Kids[1].Kind == KFn && len(t.Kids[1].Params) == 2 &&
		len(t.Kids[0].Kids) == 2 && t.Kids[0].Kids[0].Kind == KName && !PlantMapRecordUnshaped {
		if s := x.env[t.Kids[0].Kids[0].Name]; s != nil && s.elem.record() {
			k, _, err := x.walk(t.Kids[0].Kids[1])
			if err != nil {
				return nil, nil, err
			}
			f := t.Kids[1]
			x.learn(f.Params[1], s.elem)
			b, bs, err := x.walk(openNamed(f))
			if err != nil {
				return nil, nil, err
			}
			// THE PAYLOAD TAKEN APART ONCE, as `(a, b) = p` takes a tuple
			// apart: its leaves bound by the product's eliminator at the arm's
			// head, each projection (p j) the j-th, so no pass reads (p j) as
			// a table's indexing.
			if !x.infer && s.elem.leaves() >= 2 {
				p := f.Params[1]
				names := make([]string, s.elem.leaves())
				for j := range names {
					names[j] = x.name()
				}
				b = App(Name(p), Fn(names, leafNames(b, p, names)))
			}
			return App(App(t.Kids[0].Kids[0], k), Fn(f.Params, b)), bs, nil
		}
	}
	if op.Kind == KName {
		switch {
		case op.Name == RecName && len(t.Kids) >= 3 && t.Kids[1].Kind == KStr:
			s := &shape{labels: strings.Fields(t.Kids[1].Str)}
			var leaves []*Term
			var binds []*Term
			var names []string
			for _, c := range t.Kids[2:] {
				c2, cs, err := x.walk(c)
				if err != nil {
					return nil, nil, err
				}
				s.comps = append(s.comps, cs)
				if !cs.record() {
					leaves = append(leaves, c2)
					continue
				}
				// a nested record: its leaves, through a name bound once
				n := x.name()
				binds, names = append(binds, c2), append(names, n)
				leaves = append(leaves, leavesOf(Name(n), cs)...)
			}
			out := tupleOf(leaves)
			for i := len(names) - 1; i >= 0; i-- {
				out = App(Name("let"), binds[i], Fn([]string{names[i]}, out))
			}
			return out, s, nil
		case op.Name == GetName && len(t.Kids) == 3 && t.Kids[1].Kind == KStr:
			r, s, err := x.walk(t.Kids[2])
			if err != nil {
				return nil, nil, err
			}
			if !s.record() && x.infer {
				return t, nil, nil
			}
			if !s.record() {
				return nil, nil, fmt.Errorf("the projection %s reads a term whose record labels are not "+
					"known where it is a value (ADR 0128, data.md §4.7): %s", Spell(t.Kids[1].Str), ShowTerm(t.Kids[2]))
			}
			o, cs, ok := s.offset(t.Kids[1].Str)
			if !ok {
				return nil, nil, outsideProjection(t.Kids[1].Str, s.labels)
			}
			if !cs.record() {
				return App(r, Int(int64(o))), cs, nil
			}
			return x.bind(r, func(n *Term) *Term {
				cs := make([]*Term, cs.leaves())
				for i := range cs {
					cs[i] = App(n, Int(int64(o+i)))
				}
				return tupleOf(cs)
			}), cs, nil
		case op.Name == WithName && len(t.Kids) == 4 && t.Kids[1].Kind == KStr:
			r, s, err := x.walk(t.Kids[2])
			if err != nil {
				return nil, nil, err
			}
			v, vs, err := x.walk(t.Kids[3])
			if err != nil {
				return nil, nil, err
			}
			if !s.record() && x.infer {
				return t, nil, nil
			}
			if !s.record() {
				return nil, nil, fmt.Errorf("the override of %s is of a term whose record labels are not "+
					"known where it is a value (ADR 0128, data.md §4.7): %s", Spell(t.Kids[1].Str), ShowTerm(t.Kids[2]))
			}
			o, cs, ok := s.offset(t.Kids[1].Str)
			if !ok {
				return nil, nil, outsideOverride(t.Kids[1].Str, s.labels)
			}
			if !sameShape(cs, vs) && cs.record() {
				return nil, nil, fmt.Errorf("the override of %s gives a value that is not a record of its labels",
					Spell(t.Kids[1].Str))
			}
			out := x.bind(r, func(rn *Term) *Term {
				return x.bind(v, func(vn *Term) *Term {
					leaves := leavesOf(rn, s)
					if cs.record() {
						for i, c := range leavesOf(vn, cs) {
							leaves[o+i] = c
						}
					} else {
						leaves[o] = vn
					}
					return tupleOf(leaves)
				})
			})
			return out, s, nil
		case op.Name == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn && len(t.Kids[2].Params) == 1:
			v, vs, err := x.walk(t.Kids[1])
			if err != nil {
				return nil, nil, err
			}
			k := t.Kids[2]
			x.learn(k.Params[0], vs)
			b, bs, err := x.walk(openNamed(k))
			if err != nil {
				return nil, nil, err
			}
			return App(op, v, Fn(k.Params, b)), bs, nil
		case op.Name == "if" && len(t.Kids) == 4:
			c, _, err := x.walk(t.Kids[1])
			if err != nil {
				return nil, nil, err
			}
			a, as, err := x.walk(t.Kids[2])
			if err != nil {
				return nil, nil, err
			}
			b, bs, err := x.walk(t.Kids[3])
			if err != nil {
				return nil, nil, err
			}
			s, _ := join(as, bs)
			return App(op, c, a, b), s, nil
		case op.Name == "again":
			kids, shapes, err := x.walkShapes(t.Kids[1:])
			if err != nil {
				return nil, nil, err
			}
			if len(x.loops) > 0 {
				ps := x.loops[len(x.loops)-1]
				for i, s := range shapes {
					if i < len(ps) {
						x.learn(ps[i], s)
					}
				}
			}
			return App(op, kids...), noTail, nil
		case op.Name == "set" && len(t.Kids) == 4:
			kids, shapes, err := x.walkShapes(t.Kids[1:])
			if err != nil {
				return nil, nil, err
			}
			s := shapes[0]
			if shapes[2].record() {
				s = &shape{elem: shapes[2]}
			}
			return App(op, kids...), s, nil
		case (op.Name == RequireWhereName && len(t.Kids) == 4 || op.Name == RequireName && len(t.Kids) == 5) && !PlantMapRecordUnshaped:
			// A CONTRACT MARK'S VALUE IS ITS BODY'S, and so is its shape: a
			// map's insert carries its capacity's `where` (ADR 0121).
			kids, shapes, err := x.walkShapes(t.Kids[1:])
			if err != nil {
				return nil, nil, err
			}
			return App(op, kids...), shapes[len(shapes)-1], nil
		case op.Name == "insert" && len(t.Kids) == 4:
			// A MAP'S VALUE A RECORD (ADR 0128 item 7): the map's element shape,
			// as a table's store gives its table one.
			kids, shapes, err := x.walkShapes(t.Kids[1:])
			if err != nil {
				return nil, nil, err
			}
			s := shapes[0]
			if shapes[2].record() {
				s = &shape{elem: shapes[2]}
			}
			return App(op, kids...), s, nil
		case op.Name == "build-map" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn,
			op.Name == "build" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
			n, _, err := x.walk(t.Kids[1])
			if err != nil {
				return nil, nil, err
			}
			f := t.Kids[2]
			b, bs, err := x.walk(openNamed(f))
			if err != nil {
				return nil, nil, err
			}
			if len(f.Params) == 1 {
				x.learn(f.Params[0], bs)
			}
			return App(op, n, Fn(f.Params, b)), bs, nil
		case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
			f := t.Kids[1]
			var inits []*Term
			for i, z := range t.Kids[2:] {
				z2, zs, err := x.walk(z)
				if err != nil {
					return nil, nil, err
				}
				inits = append(inits, z2)
				if i < len(f.Params) {
					x.learn(f.Params[i], zs)
				}
			}
			x.loops = append(x.loops, f.Params)
			b, bs, err := x.walk(openNamed(f))
			x.loops = x.loops[:len(x.loops)-1]
			if err != nil {
				return nil, nil, err
			}
			if bs == noTail {
				bs = nil
			}
			return App(op, append([]*Term{Fn(f.Params, b)}, inits...)...), bs, nil
		case (op.Name == "table" || op.Name == "array") && len(t.Kids) >= 2:
			kids, shapes, err := x.walkShapes(t.Kids[1:])
			if err != nil {
				return nil, nil, err
			}
			var elem *shape
			if op.Name == "table" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn {
				f := t.Kids[2]
				b, bs, err := x.walk(openNamed(f))
				if err != nil {
					return nil, nil, err
				}
				kids[1], elem = Fn(f.Params, b), bs
			} else if op.Name == "array" {
				elem = shapes[0]
			}
			if elem.record() {
				return App(op, kids...), &shape{elem: elem}, nil
			}
			return App(op, kids...), nil, nil
		}
		// A TABLE READ, (t i), of a table whose elements are records.
		if s := x.env[op.Name]; s != nil && s.elem != nil && len(t.Kids) == 2 {
			i, _, err := x.walk(t.Kids[1])
			if err != nil {
				return nil, nil, err
			}
			return App(op, i), s.elem, nil
		}
	}
	kids, err := x.walkAll(t.Kids)
	if err != nil {
		return nil, nil, err
	}
	return &Term{Kind: KApp, Kids: kids}, nil, nil
}

func (x *eraser) walkAll(ts []*Term) ([]*Term, error) {
	out, _, err := x.walkShapes(ts)
	return out, err
}

func (x *eraser) walkShapes(ts []*Term) ([]*Term, []*shape, error) {
	out := make([]*Term, len(ts))
	shapes := make([]*shape, len(ts))
	for i, k := range ts {
		k2, s, err := x.walk(k)
		if err != nil {
			return nil, nil, err
		}
		out[i], shapes[i] = k2, s
	}
	return out, shapes, nil
}

// eraseRecords applies the bijection to a residual that still holds a record's
// form, and reduces the result again (data.md §4.7).
func (e *Env) eraseRecords(nf *Term, fuel *int) (*Term, error) {
	if PlantRecordsUnerased {
		return nf, nil
	}
	x := &eraser{e: e, env: map[string]*shape{}, infer: true}
	for i := 0; i < 64; i++ {
		x.changed = false
		if _, _, err := x.walk(nf); err != nil {
			return nil, err
		}
		if !x.changed {
			break
		}
	}
	x.infer, x.fresh = false, 0
	t, _, err := x.walk(nf)
	if err != nil {
		return nil, err
	}
	return normalize(t, e, fuel)
}

// PlantRecordsUnerased leaves a residual's records unplaced, as before the
// pass: a table of records reaches the printers as forms no target has.
var PlantRecordsUnerased bool

// PlantMapRecordUnshaped forgets a map read's payload shape: a record as a
// map's value is refused as a projection of no known labels again
// (TestAProductIsAMapsValue).
var PlantMapRecordUnshaped bool

// leafNames replaces each projection (p j) in t, j a literal leaf, by names[j].
func leafNames(t *Term, p string, names []string) *Term {
	if t == nil {
		return nil
	}
	if t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == p &&
		t.Kids[1].Kind == KInt && t.Kids[1].Int >= 0 && int(t.Kids[1].Int) < len(names) {
		return Name(names[t.Kids[1].Int])
	}
	if len(t.Kids) == 0 {
		return t
	}
	kids := make([]*Term, len(t.Kids))
	changed := false
	for i, k := range t.Kids {
		kids[i] = leafNames(k, p, names)
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
