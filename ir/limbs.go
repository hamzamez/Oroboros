package ir

import (
	"errors"
	"fmt"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// THE FIXED-LIMB RUNG, ON THE IR (ADR 0035).
//
// SelectBig chooses the values held exactly and writes the host's shape:
// `big+ − · / %`, `big%-small`, the comparisons, `big-of` and `big-str`. On the
// fixed-limb rung each of those is an INSTANCE of a lemma of the limb library
// (emit.LimbFunc), checked once per width: this pass replaces each call by its
// instance, inlined. Inlining is substitution into a verified body, so what is
// left to decide at the site is only what depends on the site, the word
// arithmetic under the site's facts, and the decision taken again decides it.
//
// Two shapes are chosen by FACTS:
//   - a product or a quotient by a widened word k with k ∈ [0, 2²⁸) is one pass,
//     `mul-small` or `div-small`, where `limb·k + carry` stays under 2⁵³;
//   - `big-of v` is `of`, whose limbs are v's digits in base 2²⁴, which is a
//     representation of v iff v ≥ 0. Limbs hold a MAGNITUDE (ADR 0029), so a
//     widened value that may be negative is refused, naming its interval.
//
// An operation with no instance (a quotient or remainder by a bignum) returns
// ErrNoLimbForm; a driver then takes the host's bignum for the program where
// the target has one.

// ErrNoLimbForm says the limb library has no instance for an operation.
var ErrNoLimbForm = errors.New("the limb library has no form for an operation")

// smallWord is the largest one-pass multiplier or divisor, 2²⁸ − 1.
var smallWord = ei(1<<28 - 1)

// LowerLimbs replaces f's bignum operations by instances of the limb library at
// the width the program's one set gives (emit.LimbWidth). It reports how many
// it replaced.
func LowerLimbs(tg *emit.Target, f *Func, bits int) (int, error) {
	n, lim := emit.LimbWidth(bits)
	fs := analyse(tg, f)
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	// widened is k when v = big-of k with k ∈ [0, 2²⁸).
	widened := func(v V) (V, bool) {
		s := def[v]
		if s == nil || s.Op != OCall || s.Name != "big-of" || len(s.Args) != 1 {
			return -1, false
		}
		k := s.Args[0]
		return k, fs[k].v.within(ep{}, smallWord)
	}
	// THE PLAN: for each call, the instance and its arguments.
	type use struct {
		fn   string
		args []V
	}
	plan := map[*Stmt]use{}
	var widenings []*Stmt
	var err error
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OCall || err != nil {
				continue
			}
			switch s.Name {
			case "big-of":
				widenings = append(widenings, s)
			case "big*":
				a, b := s.Args[0], s.Args[1]
				if k, ok := widened(b); ok {
					plan[s] = use{"mul-small", []V{a, k}}
				} else if k, ok := widened(a); ok {
					plan[s] = use{"mul-small", []V{b, k}}
				} else {
					plan[s] = use{"mul", s.Args}
				}
			case "big/":
				if k, ok := widened(s.Args[1]); ok {
					plan[s] = use{"div-small", []V{s.Args[0], k}}
				} else {
					err = fmt.Errorf("%w: division by another arbitrary-precision value", ErrNoLimbForm)
				}
			case "big%":
				err = fmt.Errorf("%w: the remainder by another arbitrary-precision value", ErrNoLimbForm)
			case "big-str":
				if !tg.HasBig() {
					err = fmt.Errorf("this program renders a fixed-limb bignum as a string, and target %s "+
						"declares no arbitrary-precision integer to convert it through.\n"+
						"  The arithmetic works there; only the rendering does not. Decimal conversion from\n"+
						"  limbs needs a string surface this language does not have yet (docs/spec/strings.md)", tg.Name)
					return
				}
				plan[s] = use{"to-host", s.Args}
			case "big-fit", "big-fit-signed":
				err = fmt.Errorf("internal: a bound check on the fixed-limb rung, whose instances enforce the bound themselves")
			default:
				if fn := emit.LimbShape(s.Name, false); fn != "" && isBigName(s.Name) {
					plan[s] = use{fn, s.Args}
				}
			}
		}
	})
	if err != nil {
		return 0, err
	}
	// A WIDENING IS AN INSTANCE only where something still reads it as limbs:
	// one whose every reader became a one-pass instance, which takes the word
	// itself, is dead.
	uses := useCounts(f)
	for p, u := range plan {
		if u.fn == "mul-small" || u.fn == "div-small" {
			for _, a := range p.Args {
				if d := def[a]; d != nil && d.Name == "big-of" && d.Args[0] == u.args[1] {
					uses[a]--
				}
			}
		}
	}
	for _, s := range widenings {
		if uses[s.Res[0]] == 0 {
			continue
		}
		v := s.Args[0]
		if !fs[v].v.within(ep{}, u64Max) {
			return 0, fmt.Errorf("a value widened into arbitrary precision is %s, which may be negative, and "+
				"target %s holds arbitrary precision as fixed limbs, which hold a magnitude: they "+
				"realize [0, 2^k) and nothing signed (ADR 0029)", termShow(fs[v].v), tg.Name)
		}
		plan[s] = use{"of", s.Args}
	}
	if len(plan) == 0 {
		return 0, nil
	}
	callees := map[string]*Func{}
	callee := func(fn string) (*Func, error) {
		if c, ok := callees[fn]; ok {
			return c, nil
		}
		inst, err := emit.LimbFunc(tg, fn, n, lim)
		if err != nil {
			return nil, err
		}
		c, err := Lower(tg, "limb-"+fn, inst.Sig, inst.Term, Options{Decided: true})
		if err != nil {
			return nil, fmt.Errorf("lowering the limb library's %s: %v", fn, err)
		}
		callees[fn] = c
		return c, nil
	}
	// A VALUE HELD EXACTLY IS A TABLE OF LIMBS here, which is ρ_T of a range
	// above the word on this rung (spec §4.2).
	for v, ty := range f.Types {
		if ty == core.BigType {
			f.Types[v] = "array int"
		}
	}
	for j, ty := range f.Results {
		if ty == core.BigType {
			f.Results[j] = "array int"
		}
	}
	ren := map[V]V{}
	count := 0
	var region func(r *Region)
	region = func(r *Region) {
		var out []Stmt
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, sub := range s.Sub {
				region(sub)
			}
			u, ok := plan[s]
			if !ok {
				out = append(out, *s)
				continue
			}
			c, e := callee(u.fn)
			if e != nil {
				err = e
				return
			}
			count++
			if u.fn == "to-host" {
				// the rendering: the instance converts, and big-str renders
				t := V(len(f.Types))
				f.Types = append(f.Types, core.BigType)
				out = inlineInto(f, out, c, u.args, []V{t}, ren)
				s.Args = []V{t}
				out = append(out, *s)
				continue
			}
			out = inlineInto(f, out, c, u.args, s.Res, ren)
		}
		r.Stmts = out
		if r.T == TBranch {
			region(r.Then)
			region(r.Else)
		}
	}
	region(f.Body)
	if err != nil {
		return 0, err
	}
	renameAway(f, ren)
	dropDeadWidenings(f)
	Canonicalize(f)
	f.facts = nil
	return count, nil
}

func isBigName(name string) bool {
	if name == "big-of" || name == "big%-small" || name == "big-str" {
		return true
	}
	for _, x := range bigArith {
		if x == name {
			return true
		}
	}
	for _, x := range bigCmp {
		if x == name {
			return true
		}
	}
	return false
}

// inlineInto appends a copy of callee's body to out, its parameters bound to
// args and its results to res, and returns out. Every other value of the callee
// is a fresh value of f. A body ending in a yield hands its values over by
// renaming (ren); one ending in a branch becomes an `if` whose arms are the
// branch's, since an `if`'s arms are regions whose exits are yields through
// branches (spec §1.3).
func inlineInto(f *Func, out []Stmt, callee *Func, args, res []V, ren map[V]V) []Stmt {
	m := make([]V, callee.NV())
	for i := range m {
		m[i] = -1
	}
	for j, p := range callee.Params {
		m[p] = args[j]
	}
	for v := range m {
		if m[v] < 0 {
			m[v] = V(len(f.Types))
			f.Types = append(f.Types, callee.Types[v])
		}
	}
	mv := func(v V) V {
		if v < 0 {
			return v
		}
		return m[v]
	}
	body := copyRegion(callee.Body, mv)
	out = append(out, body.Stmts...)
	switch body.T {
	case TYield:
		for j, y := range body.Args {
			ren[res[j]] = y
		}
	case TBranch:
		body.Stmts = nil
		out = append(out, Stmt{Op: OIf, Args: []V{body.Cond}, Res: res, Sub: []*Region{body.Then, body.Else}})
	}
	return out
}

// copyRegion is a deep copy of r with every value mapped by m.
func copyRegion(r *Region, m func(V) V) *Region {
	if r == nil {
		return nil
	}
	c := cloneRegion(r)
	var walk func(x *Region)
	walk = func(x *Region) {
		for i := range x.Params {
			x.Params[i] = m(x.Params[i])
		}
		for i := range x.Pis {
			x.Pis[i].V, x.Pis[i].Of, x.Pis[i].Other = m(x.Pis[i].V), m(x.Pis[i].Of), m(x.Pis[i].Other)
		}
		for i := range x.Stmts {
			s := &x.Stmts[i]
			for j := range s.Args {
				s.Args[j] = m(s.Args[j])
			}
			for j := range s.Res {
				s.Res[j] = m(s.Res[j])
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
		for j := range x.Args {
			x.Args[j] = m(x.Args[j])
		}
		if x.T == TBranch {
			x.Cond = m(x.Cond)
			walk(x.Then)
			walk(x.Else)
		}
	}
	walk(c)
	return c
}

// dropDeadWidenings removes a `big-of` nothing reads: the widening a one-pass
// instance took its word from instead.
func dropDeadWidenings(f *Func) {
	uses := useCounts(f)
	f.Walk(func(r *Region) {
		out := r.Stmts[:0]
		for _, s := range r.Stmts {
			if s.Op == OCall && s.Name == "big-of" && len(s.Res) == 1 && uses[s.Res[0]] == 0 {
				continue
			}
			out = append(out, s)
		}
		r.Stmts = out
	})
}

// SelectRung chooses the rung above the word for f, a decided function, as the
// program's plan says: the host's bignum, or fixed limbs. On limbs, an
// operation with no instance takes the host's bignum for the program where the
// target has one (it holds it from here: tg.BigRepr), and is refused by name
// where it has none.
func SelectRung(tg *emit.Target, f *Func, sig *core.Sig, plan emit.BigPlan) (int, error) {
	if plan.Host {
		if _, err := SelectBig(tg, f, sig, plan.Bits, plan.Signed); err != nil {
			return 0, err
		}
		return BigOps(f), nil
	}
	keep := f.Clone()
	keep.facts = f.facts
	if _, err := SelectBig(tg, f, sig, 0, false); err != nil {
		return 0, err
	}
	n, err := LowerLimbs(tg, f, plan.Bits)
	if !errors.Is(err, ErrNoLimbForm) {
		return n, err
	}
	if !tg.HasBig() {
		return 0, fmt.Errorf("this program needs %s.\n"+
			"  Target %s stores arbitrary precision as fixed limbs (big-repr) and the built-in\n"+
			"  limb library does not implement that; it declares no bignum of its own to\n"+
			"  fall back to either. What it does have is addition, subtraction,\n"+
			"  multiplication, the comparisons, and division and remainder by a machine\n"+
			"  word (emit/bignum.oro)", strings.TrimPrefix(err.Error(), ErrNoLimbForm.Error()+": "), tg.Name)
	}
	*f = *keep
	tg.BigRepr = "host"
	if _, err := SelectBig(tg, f, sig, plan.Bits, plan.Signed); err != nil {
		return 0, err
	}
	return BigOps(f), nil
}

// CheckClaim is the representation half of a signature's claim, for
// emit.CheckSignatures: the definition, lowered on its own and decided, has
// its values held exactly chosen (SelectBig), which refuses a value held
// exactly where the claim declares a word. That is all a claim says. Whether
// the target's rung has a form for each operation is the program's question,
// asked where the definition is used, and so is not asked here: windows has no
// limb form for a quotient by 2³², and a helper that divides by it is refused
// only by a program that calls it there.
func CheckClaim(tg *emit.Target, all []*core.Sig) func(string, *core.Sig, *core.Term) error {
	return func(name string, sig *core.Sig, nf *core.Term) error {
		plan, err := emit.PlanBig(tg, sig, nf, all...)
		if err != nil || (!plan.Host && !plan.Limbs) {
			return err
		}
		nf = emit.EraseWordAscriptions(tg.Word, nf)
		f, err := Lower(tg, name, sig, nf, Options{Decided: true})
		if err != nil {
			return err
		}
		Decide(tg, f, false)
		_, err = SelectBig(tg, f, sig, 0, false)
		return err
	}
}
