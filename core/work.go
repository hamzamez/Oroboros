package core

import "strconv"

// WORK — how many times a hole is evaluated (ADR 0071, spec/effects.md §7d).
//
// β's one-occurrence rule, let x = e in E[x] ⟶ E[e] for a pure e, keeps the
// meaning (ADR 0010: purity licenses exchange) and changes the cost: e is
// evaluated as often as E evaluates its hole. Under call-by-need the let runs e
// at most once, so the rule is an improvement exactly when the hole runs at most
// once on every path (Moran and Sands 1999, improvement theory; GHC's
// occurrence analysis, Peyton Jones and Marlow 2002 §2.2: OnceInLam is not
// once), or when e is a value, which costs nothing to evaluate again.
//
// The count is in the semiring M = {0, 1, ω}: a use is 1, sequence adds
// (1 + 1 = ω), and a λ multiplies its body by how many times it is applied,
// its SHOT σ ∈ {1, ω}. A branch adds as well, as boundOccurrences does: two
// arms each holding a copy is code duplicated, which β declines for its own
// reason, though not work.
//
// σ = 1, the λ is applied exactly once:
//   - the operator of a β-redex ((fn x̄ b) ā);
//   - the continuation of `let`, `build` and `build-map`;
//   - the continuation of an elimination (e (fn x̄ b)) whose e is not a name of
//     a definition or a primitive, nor a variable the walk knows holds a λ: a
//     product's or a sum's eliminator (binding.md, data.md §3.2), a host call's
//     continuation (ADR 0027), or a cell's marked elimination, which the
//     reducer decides runs once (ADR 0047).
//
// σ = ω otherwise: `loop`'s body runs once per iteration, `table`'s rule once
// per element read, and a λ handed to a definition, a primitive or a static
// function runs as often as the receiver chooses.

// workOccurrences is the evaluation count of the bound variable (depth, index)
// in t, saturating at 2 for ω. fnArg reports, for each parameter of the λ whose
// body t is, whether its argument is a λ: a variable holding a static function
// applied to a λ is no eliminator.
func (e *Env) workOccurrences(t *Term, depth, index int, fnArg []bool) int {
	w := workWalk{e: e, isVar: func(k *Term, d int) bool {
		return k.Kind == KBound && k.Depth == d+depth && k.Index == index
	}}
	w.fns = [][]bool{fnArg}
	return w.count(t, 0)
}

// workNamed is workOccurrences for a free name, on a normalised term.
func (e *Env) workNamed(t *Term, name string) int {
	w := workWalk{e: e, isVar: func(k *Term, d int) bool {
		return k.Kind == KName && k.Name == name
	}}
	return w.count(t, 0)
}

type workWalk struct {
	e     *Env
	isVar func(t *Term, d int) bool
	// fns[i] is the λ-argument flags of the binder i levels out from the root,
	// for the β-redexes the walk has entered; nil where unknown.
	fns [][]bool
}

func (w *workWalk) count(t *Term, d int) int {
	if w.isVar(t, d) {
		return 1
	}
	switch t.Kind {
	case KBound, KName, KInt, KFloat, KStr, KBool:
		return 0
	case KFn:
		return w.lam(t, d, nil, false)
	}
	op := t.Kids[0]
	args := t.Kids[1:]
	n := 0
	add := func(k int) bool {
		n += k
		if n >= 2 {
			n = 2
			return true
		}
		return false
	}
	switch {
	case op.Kind == KFn && len(op.Params) == len(args):
		// A β-redex: its arguments once each, its body once.
		flags := make([]bool, len(args))
		for i, a := range args {
			flags[i] = a.Kind == KFn
			if add(w.count(a, d)) {
				return 2
			}
		}
		add(w.lam(op, d, flags, true))
		return n
	case op.Kind == KName && w.e.Prim[op.Name] &&
		(op.Name == "let" || op.Name == "build" || op.Name == "build-map") &&
		len(args) == 2 && args[1].Kind == KFn:
		if add(w.count(args[0], d)) {
			return 2
		}
		add(w.lam(args[1], d, []bool{args[0].Kind == KFn}, true))
		return n
	case len(args) == 1 && args[0].Kind == KFn && w.eliminates(op, d):
		if add(w.count(op, d)) {
			return 2
		}
		add(w.lam(args[0], d, nil, true))
		return n
	}
	for _, k := range t.Kids {
		if add(w.count(k, d)) {
			return 2
		}
	}
	return n
}

// lam counts a λ's body, multiplied by its shot.
func (w *workWalk) lam(f *Term, d int, flags []bool, once bool) int {
	w.fns = append(w.fns, flags)
	k := w.count(f.Kids[0], d+1)
	w.fns = w.fns[:len(w.fns)-1]
	if !once && k > 0 {
		return 2
	}
	return k
}

// eliminates reports whether (op λ) applies λ once: op is data or a host call
// in progress, not a definition, a primitive or a variable holding a λ.
func (w *workWalk) eliminates(op *Term, d int) bool {
	switch op.Kind {
	case KApp:
		return true
	case KName:
		_, def := w.e.Defs[op.Name]
		return !w.e.Prim[op.Name] && !def
	case KBound:
		// The binder op.Depth levels out from here; its flags, when the walk
		// entered it as a β-redex, say whether its argument was a λ.
		lvl := len(w.fns) - 1 - op.Depth
		if lvl >= 0 && lvl < len(w.fns) {
			if fl := w.fns[lvl]; op.Index < len(fl) && fl[op.Index] {
				return false
			}
		}
		return true
	}
	return false
}

// floated is a binding let-floating takes out of a value: hint is its binder's
// name, name the placeholder its value's body refers to it by.
type floated struct {
	hint, name string
	val        *Term
}

// floatToValue takes a normal form (let a₁ (fn (x₁) … (let aₖ (fn (xₖ) V)))),
// residual lets around a value V, apart: the bindings, outermost first, and V
// with each xᵢ a placeholder name. It answers no when there is no let or what is
// under them is not a value.
func (e *Env) floatToValue(t *Term) (*Term, []floated, bool) {
	var fl []floated
	for t.Kind == KApp && len(t.Kids) == 3 && isResidualLet(t.Kids[0], t.Kids[1:], e) {
		lam := t.Kids[2]
		ph := "#float" + strconv.Itoa(e.floatSeq)
		e.floatSeq++
		fl = append(fl, floated{hint: lam.Params[0], name: ph, val: t.Kids[1]})
		t = lam.OpenWith([]*Term{Name(ph)})
	}
	if len(fl) == 0 || !(duplicable(t) || e.duplicableTable(t)) {
		return nil, nil, false
	}
	return t, fl, true
}

// freshFor is bindName for a binder β adds that no parameter named: apart from
// this application's bindings, the definitions, and every free name of the
// redex, so the binding captures nothing.
func (e *Env) freshFor(hint string, op *Term, args []*Term, used map[string]bool) string {
	free := func(n string) bool {
		if occurrences(op.Closed(), n) > 0 {
			return true
		}
		for _, a := range args {
			if occurrences(a, n) > 0 {
				return true
			}
		}
		return false
	}
	nm := e.bindName(hint, op.Params, used)
	for i := 1; free(nm); i++ {
		nm = e.bindName(hint+strconv.Itoa(i), op.Params, used)
	}
	return nm
}

// readsTable is readsBoundTable for a normalised term, where a table's variable
// may be spelled as a name: an application whose operator is a variable, bound
// or named, and not a primitive or a definition, reads a table through it; so
// does an observer applied to one (effects.md §7c, ADR 0069). A λ is exempt,
// as there: a read inside one runs where it is applied.
func (e *Env) readsTable(t *Term) bool {
	if t == nil || t.Kind == KFn {
		return false
	}
	variable := func(k *Term) bool {
		if k.Kind == KBound {
			return true
		}
		if k.Kind != KName || e.Prim[k.Name] {
			return false
		}
		_, def := e.Defs[k.Name]
		return !def
	}
	if t.Kind == KApp && len(t.Kids) > 0 {
		if variable(t.Kids[0]) {
			return true
		}
		if t.Kids[0].Kind == KName && e.Observe[t.Kids[0].Name] {
			for _, k := range t.Kids[1:] {
				if variable(k) {
					return true
				}
			}
		}
	}
	for _, k := range t.Kids {
		if e.readsTable(k) {
			return true
		}
	}
	return false
}
