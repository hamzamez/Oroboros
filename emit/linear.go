package emit

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
)

// The decidable fragment of docs/spec/refinements.md §4: linear integer
// arithmetic over difference constraints, which is what every bounds obligation
// actually is.
//
// Deliberately incomplete. This is not Fourier-Motzkin and not an SMT solver —
// it is the smallest thing that decides the obligations this language generates,
// and being incomplete is safe because an undischarged obligation is REPORTED
// rather than assumed.

// linear is a linear expression: coefficients over opaque variables, plus a
// constant. A variable is a parameter name, or a length term like "alen(a)"
// which is opaque and never inspected.
type linear struct {
	coef  map[string]int64
	konst int64
}

func constant(k int64) *linear { return &linear{coef: map[string]int64{}, konst: k} }

func variable(name string) *linear {
	return &linear{coef: map[string]int64{name: 1}}
}

func (l *linear) clone() *linear {
	c := &linear{coef: make(map[string]int64, len(l.coef)), konst: l.konst}
	for k, v := range l.coef {
		c.coef[k] = v
	}
	return c
}

func (l *linear) addScaled(o *linear, s int64) *linear {
	out := l.clone()
	for k, v := range o.coef {
		out.coef[k] += v * s
		if out.coef[k] == 0 {
			delete(out.coef, k)
		}
	}
	out.konst += o.konst * s
	return out
}

// sameVars reports whether two expressions differ only in their constant.
func sameVars(a, b *linear) bool {
	if len(a.coef) != len(b.coef) {
		return false
	}
	for k, v := range a.coef {
		if b.coef[k] != v {
			return false
		}
	}
	return true
}

func (l *linear) String() string {
	parts := make([]string, 0, len(l.coef)+1)
	names := make([]string, 0, len(l.coef))
	for k := range l.coef {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, k := range names {
		switch l.coef[k] {
		case 1:
			parts = append(parts, k)
		case -1:
			parts = append(parts, "-"+k)
		default:
			parts = append(parts, fmt.Sprintf("%d*%s", l.coef[k], k))
		}
	}
	if l.konst != 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%+d", l.konst))
	}
	return strings.Join(parts, " + ")
}

// facts is a conjunction of `e <= 0`, plus equalities used for substitution.
type facts struct {
	le  []*linear
	eq  map[string]*linear // name -> the linear expression it equals
	log []string           // human-readable, for the diagnostic

	// opaque are assumptions OUTSIDE the fragment, kept as printed terms and
	// matched by syntactic identity — refinements.md §3's "propagated and
	// matched by name, never decided". They were being dropped, so a `where`
	// the solver could not read left the diagnostic saying `known: nothing`
	// while the program declared something.
	opaque []string
}

func newFacts() *facts { return &facts{eq: map[string]*linear{}} }

// assumeOpaque records an assumption the fragment cannot decide.
func (f *facts) assumeOpaque(printed string) {
	for _, o := range f.opaque {
		if o == printed {
			return
		}
	}
	f.opaque = append(f.opaque, printed)
	f.log = append(f.log, "assumed "+printed)
}

// entailsOpaque discharges an obligation the fragment cannot decide, and only
// by an assumption that is syntactically the same term.
func (f *facts) entailsOpaque(printed string) bool {
	for _, o := range f.opaque {
		if o == printed {
			return true
		}
	}
	return false
}

func (f *facts) clone() *facts {
	c := &facts{le: append([]*linear(nil), f.le...), eq: make(map[string]*linear, len(f.eq)),
		log: append([]string(nil), f.log...), opaque: append([]string(nil), f.opaque...)}
	for k, v := range f.eq {
		c.eq[k] = v
	}
	return c
}

// assumeLE records `e <= 0`.
func (f *facts) assumeLE(e *linear, why string) {
	f.le = append(f.le, f.substitute(e))
	f.log = append(f.log, why)
}

// assumeEQ records `name = e`, which is what makes a let-bound loop count
// usable: the stencil's `n1 = alen a - 2` is the fact that discharges
// `i + 2 < alen a`.
func (f *facts) assumeEQ(name string, e *linear) {
	f.eq[name] = f.substitute(e)
}

// substitute replaces known equalities, repeatedly, so a chain of lets resolves.
func (f *facts) substitute(e *linear) *linear {
	out := e
	for pass := 0; pass < 8; pass++ {
		changed := false
		for name, def := range f.eq {
			c, ok := out.coef[name]
			if !ok || c == 0 {
				continue
			}
			out = out.clone()
			delete(out.coef, name)
			out = out.addScaled(def, c)
			changed = true
		}
		if !changed {
			break
		}
	}
	return out
}

// entails reports whether the facts prove `goal <= 0`.
//
// The method is single-fact implication after substitution: if a known fact has
// the same variable part and a constant at least as tight, the goal follows.
// That decides every obligation the language currently generates, including the
// stencil's `i + 2 < alen a` from `i < alen a - 2`, and it fails honestly on
// anything needing two facts combined.
func (f *facts) entails(goal *linear) bool {
	g := f.substitute(goal)
	if len(g.coef) == 0 {
		return g.konst <= 0
	}
	// A single fact, after substitution.
	for _, fact := range f.le {
		// `fact` says L + a <= 0, i.e. L <= -a. `g` says L + b <= 0, i.e.
		// L <= -b. The fact implies the goal when -a <= -b, i.e. a >= b.
		// Getting this backwards made the stencil's j+1 < alen(a) unprovable
		// from j < alen(a)-2, which is strictly stronger.
		if sameVars(fact, g) && fact.konst >= g.konst {
			return true
		}
	}
	// Or the sum of two. `i < alen p` plus `alen p <= alen q` gives
	// `i < alen q`, which is the shape a two-array loop always produces and
	// which one fact can never reach. Cheap: the fact set is tiny.
	for i, a := range f.le {
		for _, b := range f.le[i+1:] {
			sum := a.addScaled(b, 1)
			if sameVars(sum, g) && sum.konst >= g.konst {
				return true
			}
		}
	}
	return false
}

// isVar reports whether e is exactly one variable with coefficient 1 and no
// constant — the form an equality can be turned into a substitution for.
func isVar(e *linear) (string, bool) {
	if len(e.coef) != 1 || e.konst != 0 {
		return "", false
	}
	for k, v := range e.coef {
		if v == 1 {
			return k, true
		}
	}
	return "", false
}

func (f *facts) known() string {
	if len(f.log) == 0 {
		return "nothing"
	}
	return strings.Join(f.log, ", ")
}

// ---------------------------------------------------------------- reading terms

// asLinear interprets a term as a linear expression, or reports that it is
// outside the fragment. A length term is opaque: `(alen a)` becomes the variable
// "alen(a)", which is exactly what lets bounds reasoning work without the
// checker knowing anything about arrays.
func asLinear(t *core.Term) (*linear, bool) {
	switch t.Kind {
	case core.KInt:
		return constant(t.Int), true
	case core.KName:
		return variable(t.Name), true
	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return nil, false
		}
		args := t.Args()
		switch {
		case isOp(op.Name, "add") && len(args) == 2:
			a, ok1 := asLinear(args[0])
			b, ok2 := asLinear(args[1])
			if ok1 && ok2 {
				return a.addScaled(b, 1), true
			}
		case isOp(op.Name, "sub") && len(args) == 2:
			a, ok1 := asLinear(args[0])
			b, ok2 := asLinear(args[1])
			if ok1 && ok2 {
				return a.addScaled(b, -1), true
			}
		case isOp(op.Name, "mul") && len(args) == 2:
			// Linear only when one side is a literal.
			if args[0].Kind == core.KInt {
				if b, ok := asLinear(args[1]); ok {
					return constant(0).addScaled(b, args[0].Int), true
				}
			}
			if args[1].Kind == core.KInt {
				if a, ok := asLinear(args[0]); ok {
					return constant(0).addScaled(a, args[1].Int), true
				}
			}
		case (isOp(op.Name, "alen") || isOp(op.Name, "slen")) && len(args) == 1:
			return variable(lengthVar(op.Name, args[0])), true
		}
	}
	return nil, false
}

// lengthVar names a length term opaquely. Two occurrences of `(alen a)` must
// produce the same variable or nothing is provable.
func lengthVar(op string, arg *core.Term) string {
	return op + "(" + arg.String() + ")"
}

// isOp matches a qualified primitive name by its last segment, so `num/int.add`
// and a target that declares `add` unqualified are both recognised.
// opAlias maps a host's own operator spelling to the fragment's name for it.
//
// The decidable fragment was keyed to the portable layer — `int.le`, `logic.and`
// — so a target declaring Go's own `<=` and `&&` degraded every refinement to an
// opaque atom. The fragment is about the OPERATION, not about who named it.
var opAlias = map[string]string{
	"+": "add", "-": "sub", "*": "mul",
	"<": "lt", "<=": "le", ">": "gt", ">=": "ge", "==": "eq",
	"&&": "and", "||": "or", "!": "not",
	// The strict branchless connectives a target may declare under its own
	// name (ADR 0017 kept `x64.andb` for the Ada reason). As a PRECONDITION
	// they are conjunction and disjunction like any other, and a `where`
	// written with them should not degrade to an opaque atom.
	"andb": "and", "orb": "or", "notb": "not",
	"f<": "flt", "f<=": "fle", "f>": "fgt", "f>=": "fge",
}

func isOp(name, want string) bool {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if a, ok := opAlias[name]; ok {
		name = a
	}
	return name == want
}

// obligation turns a boolean term into goals of the form `e <= 0`, or reports
// that it is outside the fragment and must be treated as an opaque atom.
func obligation(t *core.Term) ([]*linear, bool) {
	if t.Kind != core.KApp || t.Op().Kind != core.KName {
		return nil, false
	}
	op := t.Op().Name
	args := t.Args()

	// Conjunction, in both spellings it can arrive in.
	//
	// A target that declares the host's own `&&` gives `(&& a b)`. The
	// language's `and` is sugar for a conditional (ADR 0017), so it arrives as
	// `(if a b false)` — and the fragment has to see through the desugaring or
	// a refinement written with `and` silently degrades to an opaque atom.
	//
	// Only conjunction needs this. `(if a true b)` is a DISJUNCTION and
	// `(if a false true)` a negation, and neither is in a fragment that is
	// conjunctions of linear inequalities — which is the same wall `d ≠ 0`
	// hits (assessment-2026-08-19 §3.2).
	conj := isOp(op, "and") && len(args) == 2
	if !conj && isOp(op, "if") && len(args) == 3 &&
		args[2].Kind == core.KBool && !args[2].IsTrue() {
		conj = true
		args = args[:2]
	}
	if conj {
		a, ok1 := obligation(args[0])
		b, ok2 := obligation(args[1])
		if ok1 && ok2 {
			return append(a, b...), true
		}
		return nil, false
	}
	if len(args) != 2 {
		return nil, false
	}
	l, ok1 := asLinear(args[0])
	r, ok2 := asLinear(args[1])
	if !ok1 || !ok2 {
		return nil, false
	}
	// Integers, so strict `<` is `<= -1`.
	switch {
	case isOp(op, "lt"): // l < r   ⟶  l - r + 1 <= 0
		return []*linear{l.addScaled(r, -1).addScaled(constant(1), 1)}, true
	case isOp(op, "le"): // l <= r  ⟶  l - r <= 0
		return []*linear{l.addScaled(r, -1)}, true
	case isOp(op, "gt"): // l > r   ⟶  r - l + 1 <= 0
		return []*linear{r.addScaled(l, -1).addScaled(constant(1), 1)}, true
	case isOp(op, "ge"): // l >= r  ⟶  r - l <= 0
		return []*linear{r.addScaled(l, -1)}, true
	case isOp(op, "eq"): // both directions
		return []*linear{l.addScaled(r, -1), r.addScaled(l, -1)}, true
	}
	return nil, false
}

// constantValue reports the value of a linear expression with no variables.
func (l *linear) constantValue() (int64, bool) {
	for _, c := range l.coef {
		if c != 0 {
			return 0, false
		}
	}
	return l.konst, true
}
