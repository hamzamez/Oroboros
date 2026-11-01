package ir

import (
	"fmt"
	"math/big"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// THE NUMBERS OF N = ℤ[1/10] AS SCALED WORDS (ADR 0086, numbers.md §5–§6.1).
//
// N is filtered by the modules Nₖ = 10⁻ᵏℤ, with Nₐ + N_b ⊆ N_max(a,b) and
// Nₐ·N_b ⊆ N_(a+b). So each value of sort N is given a static SCALE k, an upper
// bound on its least, by the least fixed point of those two laws over the
// function (a loop's variable joins its entries and its continues, by max).
// The value x is then held as its NUMERATOR m = x·10ᵏ, an integer: the
// abstraction function d(m) = m·10⁻ᵏ of ADR 0083, one representation per value.
//
// Every operation of N is then a law of arithmetic under d, and is rewritten
// into the integer operations on numerators that compute it:
//
//	a ± b at scale k:  m_a·10^(k−kₐ) ± m_b·10^(k−k_b)
//	a · b:             m_a·m_b, at kₐ + k_b
//	a < b, a = b:      the numerators aligned to one scale, compared
//	round(G ∩ [lo,hi], a, μ) onto 10⁻ʲℤ:  r_ℤ,μ(m_a ÷ 10^(kₐ−j)), at scale j
//	round(…, a ÷ b, μ):  r_ℤ,μ(m_a·10^(k_b+j−kₐ) ÷ m_b), the power on the side it is positive
//
// and r_ℤ,μ(u ÷ v), integer division with a mode, from the Euclidean remainder
// r = u mod v ∈ [0, v) and the floor f = (u − r)/v, for v > 0 (a divisor of
// unknown sign is made positive first):
//
//	'down f · 'up f + (r + v − 1)/v · 'zero u / v (truncation)
//	'away f, or f + 1 when r > 0 and u ≥ 0
//	'nearest f + 1 when 2r > v; f + (f mod 2) when 2r = v (a tie, to even); else f
//	'half_away f + 1 when 2r > v, or when 2r = v and u ≥ 0; else f
//	'sat 'nearest, clamped to the set's ends
//
// SOUNDNESS IS THE DECISION'S. Every operation written is an integer
// operation, so Decide, run after, proves each inside the target's word or the
// program is refused: an alignment that overflows is a refused multiply, not a
// wrong answer. What only this pass knows, it decides itself on the rewritten
// function's intervals before returning: each rounding's result inside its
// set's ends, a ÷'s divisor not zero, an ascription's value inside its grid.
//
// Not built here (numbers.md §9): a value of N held as a double (part 4), an N
// value at a host call, in a table, or in a map, and a scale past maxScale.
func SelectNumbers(tg *emit.Target, f *Func) (bool, error) {
	_, changed, err := selectNumbers(tg, f)
	if changed && err == nil {
		// in the printer's numbering, as every pass that makes values ends
		// (print.go): a value's id is then the text's (ADR 0038)
		Canonicalize(f)
	}
	return changed, err
}

// selectNumbers is SelectNumbers with the scales it chose, which a decision
// of x ∈ D(k, …) on the rewritten function reads (require.go).
func selectNumbers(tg *emit.Target, f *Func) (*numbers, bool, error) {
	return selectNumbersFor(tg, f, true)
}

// selectNumbersFor is selectNumbers deciding its own obligations or not.
func selectNumbersFor(tg *emit.Target, f *Func, decide bool) (*numbers, bool, error) {
	n, _, changed, err := selectBoth(tg, f, decide)
	return n, changed, err
}

// selectBoth is selectNumbersFor with the doubles' pass, which a decision of
// x ∈ F64 ∩ [lo, hi] reads (require.go).
func selectBoth(tg *emit.Target, f *Func, decide bool) (*numbers, *doubles, bool, error) {
	unscope(f)
	// THE DOUBLES FIRST (doubles.go): each rounding onto F64 becomes the
	// target's realization, so what is left of N is held in scaled words.
	dbl, err := realizeDoubles(tg, f)
	if err != nil {
		return nil, nil, false, err
	}
	n, changed, err := selectScaled(tg, f, decide)
	if err != nil || !decide {
		return n, dbl, changed || dbl != nil, err
	}
	return n, dbl, changed || dbl != nil, dbl.decide()
}

// unscope makes each rounding the scope wrote of an integer the identity it
// is (numbers.md §3): typing gave its value the operand's type.
func unscope(f *Func) {
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != ORound || !s.Scoped || len(s.Args) != 1 {
				continue
			}
			if ty := f.Types[s.Res[0]]; ty != core.NumType && ty != core.F64Name && !decType(ty) {
				*s = Stmt{Op: OThe, Type: ty, Args: s.Args, Res: s.Res, Src: s.Src}
			}
		}
	})
}

// selectScaled is the scaled word's part of selectNumbersFor.
func selectScaled(tg *emit.Target, f *Func, decide bool) (*numbers, bool, error) {
	n := &numbers{tg: tg, f: f, scale: map[V]int{}, konstOf: map[V]int64{}}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == OConst && s.Lit.Kind == core.KInt && len(s.Res) == 1 && !n.isNum(s.Res[0]) {
				n.konstOf[s.Res[0]] = s.Lit.Int
			}
		}
	})
	if !n.any() {
		return nil, false, nil
	}
	if err := n.scales(); err != nil {
		return nil, false, err
	}
	n.rewrite()
	if n.err != nil {
		return nil, false, n.err
	}
	if !decide {
		return n, true, nil
	}
	return n, true, n.decide()
}

// maxScale bounds a scale: 10³⁰ is past every word, so a value of a larger
// scale has no numerator a target holds, and a scale growing around a loop
// (x ← x·0.5) reaches it and is refused.
const maxScale = 30

type numbers struct {
	tg    *emit.Target
	f     *Func
	scale map[V]int
	// konstOf is each constant's numerator, as the rewrite has made it, at the
	// value's own scale: an integer constant's is itself.
	konstOf map[V]int64
	// obligations this pass decides, each on a rewritten value
	within  []bound
	nonzero []nonzero
	err     error
}

type bound struct {
	v      V
	lo, hi *big.Int // the numerator's ends; nil for none
	what   string
	// A ROUNDING'S BOUND is decided by its monotonicity, not on v's interval:
	// r is a monotone retraction onto a grid holding lo and hi, so u ÷ d inside
	// [lo, hi] puts r(u ÷ d) there, while the expansion's branches lose that
	// (2r > d has no interval). d < 0 for none: v is the value itself.
	u, d V
	mode string
}

type nonzero struct {
	v    V
	what string
}

func (n *numbers) isNum(v V) bool { return n.f.Types[v] == core.NumType || decType(n.f.Types[v]) }

func decType(ty string) bool { _, _, _, ok := core.DecRange(ty); return ok }

func (n *numbers) any() bool {
	found := false
	n.f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == ORound {
				found = true
			}
			for _, v := range s.Res {
				if n.isNum(v) {
					found = true
				}
			}
		}
	})
	for _, p := range n.f.Params {
		if n.isNum(p) {
			found = true
		}
	}
	return found
}

func (n *numbers) fail(format string, args ...any) {
	if n.err == nil {
		n.err = fmt.Errorf(format, args...)
	}
}

// sc is a value's scale: an integer's is 0.
func (n *numbers) sc(v V) int {
	if !n.isNum(v) {
		return 0
	}
	return n.scale[v]
}

// raise joins k into v's scale, reporting a change.
func (n *numbers) raise(v V, k int) bool {
	if !n.isNum(v) || k <= n.scale[v] {
		return false
	}
	if k > maxScale {
		n.fail("%s: a number's scale grows past 10⁻%d, where no word holds its numerator: a value "+
			"multiplied by a fraction around a loop has no fixed grid; round it each time (ADR 0086, numbers.md §5)",
			n.f.Name, maxScale)
		return false
	}
	n.scale[v] = k
	return true
}

// gridOf is a rounding's or an ascription's target grid: its scale j, and its
// ends as numerators at that scale (nil for an unbounded `int`).
func gridOf(ty string) (j int, lo, hi *big.Int, ok bool) {
	if k, l, h, isDec := core.DecRange(ty); isDec {
		p := pow10(k)
		return k, new(big.Int).Mul(l, p), new(big.Int).Mul(h, p), true
	}
	if l, h, isRange := core.IntRangeBig(ty); isRange {
		return 0, l, h, true
	}
	if ty == "int" {
		return 0, nil, nil, true
	}
	return 0, nil, nil, false
}

func pow10(k int) *big.Int { return new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(k)), nil) }

// scales is the least fixed point of the filtration's laws.
func (n *numbers) scales() error {
	for _, p := range n.f.Params {
		if k, _, _, ok := core.DecRange(n.f.Types[p]); ok {
			n.scale[p] = k
		} else if n.f.Types[p] == core.NumType {
			return fmt.Errorf("%s: a parameter of sort N names no grid, so no numerator holds it: "+
				"declare it in D(k, lo..hi) (ADR 0086, numbers.md §2)", n.f.Name)
		}
	}
	for iter := 0; ; iter++ {
		changed := false
		n.f.Walk(func(r *Region) {
			for _, pi := range r.Pis {
				changed = n.raise(pi.V, n.sc(pi.Of)) || changed
			}
			for i := range r.Stmts {
				changed = n.step(&r.Stmts[i]) || changed
			}
		})
		if n.err != nil {
			return n.err
		}
		if !changed {
			break
		}
		if iter > 4*maxScale+8 {
			return fmt.Errorf("%s: the scales did not settle", n.f.Name)
		}
	}
	// THE DECLARED RESULT'S GRID is where every exit goes.
	for j, ty := range n.f.Results {
		if k, _, _, ok := core.DecRange(ty); ok {
			for _, y := range exitsOf(n.f.Body, TYield) {
				if j < len(y) && n.sc(y[j]) > k {
					return fmt.Errorf("%s: the result is declared in %s, and its value's scale is 10⁻%d: "+
						"a finer number is not in a coarser grid; round it there (ADR 0086)",
						n.f.Name, core.ShowSet(ty), n.sc(y[j]))
				}
			}
		}
	}
	return nil
}

// step applies one statement's law, reporting a change.
func (n *numbers) step(s *Stmt) bool {
	changed := false
	res := func(k int) {
		if len(s.Res) == 1 {
			changed = n.raise(s.Res[0], k) || changed
		}
	}
	switch s.Op {
	case OConst:
		if s.Lit.Kind == core.KDec {
			res(s.Lit.Dec.K)
		}
	case OAdd, OSub, ONeg:
		k := 0
		for _, a := range s.Args {
			k = max(k, n.sc(a))
		}
		res(k)
	case OMul:
		k := 0
		for _, a := range s.Args {
			k += n.sc(a)
		}
		res(k)
	case ORound:
		if j, _, _, ok := gridOf(s.Type); ok {
			res(j)
		} else {
			n.fail("%s: round onto %s: a rounding's set is a grid restricted to an interval, D(k, lo..hi) "+
				"or a range; F64's is part 4 (numbers.md §2, §9)", n.f.Name, core.ShowSet(s.Type))
		}
	case OThe:
		if k, _, _, ok := core.DecRange(s.Type); ok {
			res(max(k, n.sc(s.Args[0])))
		} else {
			res(n.sc(s.Args[0]))
		}
	case OLoop:
		body := s.Sub[0]
		for j, p := range body.Params {
			changed = n.raise(p, n.sc(s.Args[j])) || changed
		}
		for _, c := range exitsOf(body, TContinue) {
			for j, a := range c {
				if j < len(body.Params) {
					changed = n.raise(body.Params[j], n.sc(a)) || changed
				}
			}
		}
		for _, b := range exitsOf(body, TBreak) {
			for j, a := range b {
				if j < len(s.Res) {
					changed = n.raise(s.Res[j], n.sc(a)) || changed
				}
			}
		}
	case OIf:
		for _, sub := range s.Sub {
			for _, y := range exitsOf(sub, TYield) {
				for j, a := range y {
					if j < len(s.Res) {
						changed = n.raise(s.Res[j], n.sc(a)) || changed
					}
				}
			}
		}
	case OCall:
		for _, v := range append(append([]V{}, s.Args...), s.Res...) {
			if n.isNum(v) {
				n.fail("%s: a number of N reaching %s, a host call: its representation there is not built "+
					"(numbers.md §9, part 4); round it to an integer first", n.f.Name, core.ShowName(s.Name))
			}
		}
	case OIndex, OSet, OArray, OBuild, OTabulate, OMap, ORead, OInsert, OBuildMap, OCellRef, OCellGet, OCellFresh:
		for _, v := range append(append([]V{}, s.Args...), s.Res...) {
			if n.isNum(v) {
				n.fail("%s: a number of N in a table, a map or a cell is not built (numbers.md §9)", n.f.Name)
			}
		}
	}
	return changed
}

// ---------------------------------------------------------------- the rewrite

// rewrite replaces every operation of N by the integer operations on
// numerators that compute it, aligns every value flowing into a join to the
// join's scale, and retypes every value of N as the integer it now is.
func (n *numbers) rewrite() {
	n.region(n.f.Body)
	// THE FUNCTION'S EXITS go to its declared grid.
	for j, ty := range n.f.Results {
		if k, lo, hi, ok := core.DecRange(ty); ok {
			p := pow10(k)
			n.f.Results[j] = "int " + new(big.Int).Mul(lo, p).String() + " " + new(big.Int).Mul(hi, p).String()
			n.alignExits(n.f.Body, TYield, j, k)
		}
	}
	n.realign()
	for _, p := range n.f.Params {
		if k, lo, hi, ok := core.DecRange(n.f.Types[p]); ok {
			q := pow10(k)
			n.f.Types[p] = "int " + new(big.Int).Mul(lo, q).String() + " " + new(big.Int).Mul(hi, q).String()
		}
	}
	for v, ty := range n.f.Types {
		if ty == core.NumType || decType(ty) {
			n.f.Types[v] = "int"
		}
	}
}

// alignExits multiplies the j-th value of each exit of kind k in r (and the
// branches below it) up to scale to.
func (n *numbers) alignExits(r *Region, k Term, j, to int) {
	if r.T == TBranch {
		n.alignExits(r.Then, k, j, to)
		n.alignExits(r.Else, k, j, to)
		return
	}
	if r.T == k && j < len(r.Args) {
		r.Args[j] = n.up(r, r.Args[j], n.sc(r.Args[j]), to)
	}
}

// up appends to r the multiplication taking v from scale from to scale to,
// and returns the result: v itself when the scales agree.
func (n *numbers) up(r *Region, v V, from, to int) V {
	if to <= from {
		return v
	}
	// A CONSTANT'S NUMERATOR AT ANOTHER SCALE is a constant: folded, so a guard
	// compares with it (and its thresholds read it, interval.go) rather than
	// with a product computed at run time.
	if m, ok := n.konstOf[v]; ok {
		p := new(big.Int).Mul(big.NewInt(m), pow10(to-from))
		if p.IsInt64() {
			return n.konst(r, p)
		}
	}
	c := n.konst(r, pow10(to-from))
	return n.emit(r, Stmt{Op: OMul, Args: []V{v, c}})
}

func (n *numbers) fresh() V {
	n.f.Types = append(n.f.Types, "int")
	return V(len(n.f.Types) - 1)
}

func (n *numbers) emit(r *Region, s Stmt) V {
	v := n.fresh()
	switch {
	case s.Op.IsCmp():
		n.f.Types[v] = "bool"
	case s.Op == ODiv:
		s.Mode = MExact // truncation; its divisor is a positive power of ten, or decided (decide)
	}
	s.Res = []V{v}
	r.Stmts = append(r.Stmts, s)
	return v
}

func (n *numbers) konst(r *Region, c *big.Int) V {
	if !c.IsInt64() {
		n.fail("%s: a numerator's power of ten, %s, is past the word: the scales are too far apart", n.f.Name, c)
		return n.emit(r, Stmt{Op: OConst, Lit: core.Int(0)})
	}
	v := n.emit(r, Stmt{Op: OConst, Lit: core.Int(c.Int64())})
	n.konstOf[v] = c.Int64()
	return v
}

// region rewrites r's statements in order, then aligns its terminator's values.
func (n *numbers) region(r *Region) {
	old := r.Stmts
	r.Stmts = nil
	for i := range old {
		s := old[i]
		for _, sub := range s.Sub {
			n.region(sub)
		}
		n.stmt(r, s)
	}
	if r.T == TBranch {
		n.region(r.Then)
		n.region(r.Else)
	}
}

// stmt appends to r the rewrite of s.
func (n *numbers) stmt(r *Region, s Stmt) {
	switch {
	case s.Op == OConst && len(s.Res) == 1 && n.isNum(s.Res[0]):
		k := n.sc(s.Res[0])
		var m *big.Int
		switch s.Lit.Kind {
		case core.KDec:
			m = new(big.Int).Mul(s.Lit.Dec.M, pow10(k-s.Lit.Dec.K))
		case core.KInt:
			m = new(big.Int).Mul(big.NewInt(s.Lit.Int), pow10(k))
		}
		if m == nil || !m.IsInt64() {
			n.fail("%s: the numeral %s at scale 10⁻%d has a numerator past the word", n.f.Name, core.ShowTerm(s.Lit), k)
			return
		}
		s.Lit = core.Int(m.Int64())
		n.konstOf[s.Res[0]] = m.Int64()
		r.Stmts = append(r.Stmts, s)
	case (s.Op == OAdd || s.Op == OSub || s.Op == ONeg) && n.isNum(s.Res[0]):
		k := n.sc(s.Res[0])
		for j, a := range s.Args {
			s.Args[j] = n.up(r, a, n.sc(a), k)
		}
		r.Stmts = append(r.Stmts, s)
	case s.Op == OMul && n.isNum(s.Res[0]):
		k := n.sc(s.Res[0])
		have := n.sc(s.Args[0]) + n.sc(s.Args[1])
		if have == k {
			r.Stmts = append(r.Stmts, s)
			return
		}
		res := s.Res[0]
		prod := n.emit(r, Stmt{Op: OMul, Args: s.Args})
		c := n.konst(r, pow10(k-have))
		r.Stmts = append(r.Stmts, Stmt{Op: OMul, Args: []V{prod, c}, Res: []V{res}})
	case s.Op.IsCmp() && (n.isNum(s.Args[0]) || n.isNum(s.Args[1])):
		k := max(n.sc(s.Args[0]), n.sc(s.Args[1]))
		for j, a := range s.Args {
			s.Args[j] = n.up(r, a, n.sc(a), k)
		}
		r.Stmts = append(r.Stmts, s)
	case s.Op == ORound:
		n.round(r, s)
	case s.Op == OThe && n.isNum(s.Res[0]):
		k := n.sc(s.Res[0])
		a := n.up(r, s.Args[0], n.sc(s.Args[0]), k)
		if _, lo, hi, ok := gridOf(s.Type); ok && lo != nil {
			n.within = append(n.within, bound{v: a, lo: lo, hi: hi, what: "the ascription to " + core.ShowSet(s.Type), d: -1})
		}
		r.Stmts = append(r.Stmts, Stmt{Op: OThe, Type: "int", Args: []V{a}, Res: s.Res, Src: s.Src})
	case s.Op == OLoop:
		body := s.Sub[0]
		for j, p := range body.Params {
			s.Args[j] = n.up(r, s.Args[j], n.sc(s.Args[j]), n.sc(p))
			n.alignExits(body, TContinue, j, n.sc(p))
		}
		for j, v := range s.Res {
			n.alignExits(body, TBreak, j, n.sc(v))
		}
		r.Stmts = append(r.Stmts, s)
	case s.Op == OIf:
		for j, v := range s.Res {
			for _, sub := range s.Sub {
				n.alignExits(sub, TYield, j, n.sc(v))
			}
		}
		r.Stmts = append(r.Stmts, s)
	default:
		r.Stmts = append(r.Stmts, s)
	}
}

// round appends the integer operations realizing a rounding (numbers.md §6.1).
func (n *numbers) round(r *Region, s Stmt) {
	j, lo, hi, _ := gridOf(s.Type)
	what := "round(" + core.ShowSet(s.Type) + ", …)"
	var u, v V
	unit := false // the divisor is 1: nothing to round
	if len(s.Args) == 1 {
		x := s.Args[0]
		kx := n.sc(x)
		if kx <= j {
			u, unit = n.up(r, x, kx, j), true
		} else {
			u, v = x, n.konst(r, pow10(kx-j))
		}
	} else {
		a, b := s.Args[0], s.Args[1]
		e := n.sc(b) + j - n.sc(a)
		u, v = n.up(r, a, 0, max(e, 0)), n.up(r, b, 0, max(-e, 0))
		n.nonzero = append(n.nonzero, nonzero{b, "the divisor of " + what})
		// A divisor of unknown sign is made positive: (u, v) ↦ (−u, −v) when v < 0.
		u, v = n.positive(r, u, v)
	}
	q := u
	d := V(-1)
	if !unit {
		q, d = n.divMode(r, u, v, s.Name), v
	}
	if s.Name == "sat" && lo != nil {
		q = n.clamp(r, q, lo, hi)
	} else if lo != nil {
		n.within = append(n.within, bound{v: q, lo: lo, hi: hi, what: what + "'s value inside its set",
			u: u, d: d, mode: s.Name})
	}
	k := 0
	if len(s.Res) == 1 && n.isNum(s.Res[0]) {
		k = n.sc(s.Res[0])
	}
	q = n.up(r, q, j, k)
	r.Stmts = append(r.Stmts, Stmt{Op: OThe, Type: "int", Args: []V{q}, Res: s.Res, Src: s.Src})
}

// positive is (u, v), or (−u, −v) when v < 0.
func (n *numbers) positive(r *Region, u, v V) (V, V) {
	zero := n.konst(r, big.NewInt(0))
	neg := n.emit(r, Stmt{Op: OLt, Args: []V{v, zero}})
	th, el := &Region{}, &Region{}
	nu := n.emit(th, Stmt{Op: ONeg, Args: []V{u}})
	nv := n.emit(th, Stmt{Op: ONeg, Args: []V{v}})
	th.T, th.Args = TYield, []V{nu, nv}
	el.T, el.Args = TYield, []V{u, v}
	ru, rv := n.fresh(), n.fresh()
	r.Stmts = append(r.Stmts, Stmt{Op: OIf, Args: []V{neg}, Res: []V{ru, rv}, Sub: []*Region{th, el}})
	return ru, rv
}

// divMode is r_ℤ,μ(u ÷ v) for v > 0, from the Euclidean remainder.
func (n *numbers) divMode(r *Region, u, v V, mode string) V {
	if mode == "zero" {
		return n.emit(r, Stmt{Op: ODiv, Args: []V{u, v}})
	}
	rem := n.emit(r, Stmt{Op: OMod, Args: []V{u, v}})
	d := n.emit(r, Stmt{Op: OSub, Args: []V{u, rem}})
	f := n.emit(r, Stmt{Op: ODiv, Args: []V{d, v}}) // exact: v divides u − r
	one := n.konst(r, big.NewInt(1))
	switch mode {
	case "down":
		return f
	case "up":
		vm1 := n.emit(r, Stmt{Op: OSub, Args: []V{v, one}})
		t := n.emit(r, Stmt{Op: OAdd, Args: []V{rem, vm1}})
		c := n.emit(r, Stmt{Op: ODiv, Args: []V{t, v}}) // 1 when r > 0, else 0
		return n.emit(r, Stmt{Op: OAdd, Args: []V{f, c}})
	case "away":
		zero := n.konst(r, big.NewInt(0))
		frac := n.emit(r, Stmt{Op: OGt, Args: []V{rem, zero}})
		nonneg := n.emit(r, Stmt{Op: OGe, Args: []V{u, zero}})
		return n.choose(r, frac, func(th *Region) V {
			return n.choose(th, nonneg, func(a *Region) V { return n.emit(a, Stmt{Op: OAdd, Args: []V{f, one}}) },
				func(*Region) V { return f })
		}, func(*Region) V { return f })
	}
	// 'nearest, 'half_away and 'sat compare twice the remainder with the divisor.
	two := n.konst(r, big.NewInt(2))
	tr := n.emit(r, Stmt{Op: OMul, Args: []V{rem, two}})
	above := n.emit(r, Stmt{Op: OGt, Args: []V{tr, v}})
	tie := n.emit(r, Stmt{Op: OEq, Args: []V{tr, v}})
	return n.choose(r, above, func(th *Region) V { return n.emit(th, Stmt{Op: OAdd, Args: []V{f, one}}) },
		func(el *Region) V {
			return n.choose(el, tie, func(t *Region) V {
				if mode == "half-away" {
					zero := n.konst(t, big.NewInt(0))
					nonneg := n.emit(t, Stmt{Op: OGe, Args: []V{u, zero}})
					return n.choose(t, nonneg, func(a *Region) V { return n.emit(a, Stmt{Op: OAdd, Args: []V{f, one}}) },
						func(*Region) V { return f })
				}
				// a tie to the even neighbour: f + (f mod 2)
				two2 := n.konst(t, big.NewInt(2))
				odd := n.emit(t, Stmt{Op: OMod, Args: []V{f, two2}})
				return n.emit(t, Stmt{Op: OAdd, Args: []V{f, odd}})
			}, func(*Region) V { return f })
		})
}

// choose appends `if c { th } else { el }` with one result.
func (n *numbers) choose(r *Region, c V, th, el func(*Region) V) V {
	a, b := &Region{}, &Region{}
	ya := th(a)
	yb := el(b)
	a.T, a.Args = TYield, []V{ya}
	b.T, b.Args = TYield, []V{yb}
	res := n.fresh()
	r.Stmts = append(r.Stmts, Stmt{Op: OIf, Args: []V{c}, Res: []V{res}, Sub: []*Region{a, b}})
	return res
}

// clamp is min(max(q, lo), hi), 'sat's total rounding.
func (n *numbers) clamp(r *Region, q V, lo, hi *big.Int) V {
	l := n.konst(r, lo)
	h := n.konst(r, hi)
	below := n.emit(r, Stmt{Op: OLt, Args: []V{q, l}})
	q1 := n.choose(r, below, func(*Region) V { return l }, func(*Region) V { return q })
	above := n.emit(r, Stmt{Op: OGt, Args: []V{q1, h}})
	return n.choose(r, above, func(*Region) V { return h }, func(*Region) V { return q1 })
}

// ---------------------------------------------------------------- obligations

// decide discharges this pass's obligations on the rewritten function's
// intervals, or refuses the program saying which (ADR 0037's procedure, the
// intervals' route).
func (n *numbers) decide() error {
	fs := analyse(n.tg, n.f)
	var bad []string
	proven := map[V]string{}
	for _, b := range n.within {
		x := fs[b.v].v
		l, okl := fromBig(b.lo)
		h, okh := fromBig(b.hi)
		ok := okl && okh && x.within(l, h)
		if !ok && b.d >= 0 {
			if rl, rh, known := roundedHull(fs[b.u].v, fs[b.d].v, b.mode); known {
				ok = rl.Cmp(b.lo) >= 0 && rh.Cmp(b.hi) <= 0
				x = rangeE(mustEnd(rl), mustEnd(rh))
			}
		}
		if !ok {
			bad = append(bad, fmt.Sprintf("%s: not proven (the numerator is %s, its set's ends %s..%s)",
				b.what, show(x), b.lo, b.hi))
			continue
		}
		proven[b.v] = "int " + b.lo.String() + " " + b.hi.String()
	}
	// WHAT IS PROVEN IS A FACT: the result's ascription carries its set's ends,
	// so the decision after reads them, as it reads a declared range.
	n.f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == OThe && len(s.Args) == 1 {
				if ty, ok := proven[s.Args[0]]; ok {
					s.Type = ty
				}
			}
		}
	})
	for _, z := range n.nonzero {
		x := fs[z.v].v
		if x.bot {
			continue
		}
		positive := !x.nlo && x.lo.sign() > 0
		negative := !x.phi && x.hi.sign() < 0
		if !positive && !negative {
			bad = append(bad, fmt.Sprintf("%s may be 0: not proven (it is %s)", z.what, show(x)))
		}
	}
	if len(bad) > 0 {
		return fmt.Errorf("%s: %s (ADR 0086, numbers.md §3)", n.f.Name, strings.Join(bad, "; "))
	}
	return nil
}

// roundedHull is [r(min u ÷ d), r(max u ÷ d)] over the box of u's and d's
// intervals, d of one sign: u ÷ d is monotone in each, so its extremes are
// corners, and r is monotone (numbers.md §3).
func roundedHull(u, d iv, mode string) (lo, hi *big.Int, ok bool) {
	if !u.finite() || !d.finite() || (d.lo.sign() <= 0 && d.hi.sign() >= 0) {
		return nil, nil, false
	}
	var qs []*big.Rat
	for _, a := range []ep{u.lo, u.hi} {
		for _, b := range []ep{d.lo, d.hi} {
			qs = append(qs, new(big.Rat).SetFrac(a.big(), b.big()))
		}
	}
	mn, mx := qs[0], qs[0]
	for _, q := range qs[1:] {
		if q.Cmp(mn) < 0 {
			mn = q
		}
		if q.Cmp(mx) > 0 {
			mx = q
		}
	}
	if mode == "sat" {
		mode = "nearest"
	}
	return roundRat(mn, mode), roundRat(mx, mode), true
}

// roundRat is r_ℤ,μ on a rational (numbers.md §3).
func roundRat(x *big.Rat, mode string) *big.Int {
	fl := new(big.Int).Div(x.Num(), x.Denom()) // Euclidean, so the floor
	if x.IsInt() {
		return fl
	}
	c := new(big.Int).Add(fl, big.NewInt(1))
	switch mode {
	case "down":
		return fl
	case "up":
		return c
	case "zero":
		if x.Sign() >= 0 {
			return fl
		}
		return c
	case "away":
		if x.Sign() >= 0 {
			return c
		}
		return fl
	}
	switch new(big.Rat).Sub(x, new(big.Rat).SetInt(fl)).Cmp(big.NewRat(1, 2)) {
	case -1:
		return fl
	case 1:
		return c
	}
	if mode == "half-away" {
		if x.Sign() >= 0 {
			return c
		}
		return fl
	}
	if fl.Bit(0) == 0 {
		return fl
	}
	return c
}

func mustEnd(x *big.Int) ep {
	e, _ := fromBig(x)
	return e
}

// realign makes each π-parameter on a number of N state its relation between
// numerators at one scale (spec §6, Theorem E). Lowering wrote `Of Rel Other`
// between the values; their numerators are at the scales kOf and kOther, so
// the relation holds of the numerators only when the scales agree. A constant
// at the lower scale is replaced by its numerator at Of's, hoisted to the
// function's entry, which dominates every arm. Any other π on a number is
// removed and its value renamed back to Of: the identity, which narrows
// nothing and is sound. Left as written, `sum < 10^10` on a sum held in cents
// said m_sum < 10¹⁰, a hundredth of the truth.
func (n *numbers) realign() {
	consts := map[V]*core.Term{}
	n.f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if s := &r.Stmts[i]; s.Op == OConst && len(s.Res) == 1 {
				consts[s.Res[0]] = s.Lit
			}
		}
	})
	var hoisted []Stmt
	var walk func(r *Region)
	walk = func(r *Region) {
		kept := r.Pis[:0]
		var drop []Pi
		for _, pi := range r.Pis {
			if pi.Len || (!n.isNum(pi.Of) && (pi.Other < 0 || !n.isNum(pi.Other))) {
				kept = append(kept, pi)
				continue
			}
			kOf, kOt := n.sc(pi.Of), 0
			if pi.Other >= 0 {
				kOt = n.sc(pi.Other)
			}
			switch {
			case kOf == kOt:
				kept = append(kept, pi)
			case kOf > kOt && pi.Other >= 0 && consts[pi.Other] != nil && consts[pi.Other].Kind == core.KInt:
				m := new(big.Int).Mul(big.NewInt(consts[pi.Other].Int), pow10(kOf-kOt))
				if !m.IsInt64() {
					drop = append(drop, pi)
					continue
				}
				c := n.fresh()
				hoisted = append(hoisted, Stmt{Op: OConst, Lit: core.Int(m.Int64()), Res: []V{c}})
				pi.Other = c
				kept = append(kept, pi)
			default:
				drop = append(drop, pi)
			}
		}
		r.Pis = kept
		for _, pi := range drop {
			renameIn(r, pi.V, pi.Of)
		}
		for i := range r.Stmts {
			for _, sub := range r.Stmts[i].Sub {
				walk(sub)
			}
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(n.f.Body)
	n.f.Body.Stmts = append(hoisted, n.f.Body.Stmts...)
}

// renameIn replaces every use of from by to in r and the regions below it.
func renameIn(r *Region, from, to V) {
	sub := func(vs []V) {
		for i, v := range vs {
			if v == from {
				vs[i] = to
			}
		}
	}
	for i := range r.Pis {
		if r.Pis[i].Of == from {
			r.Pis[i].Of = to
		}
		if r.Pis[i].Other == from {
			r.Pis[i].Other = to
		}
	}
	for i := range r.Stmts {
		sub(r.Stmts[i].Args)
		for _, s := range r.Stmts[i].Sub {
			renameIn(s, from, to)
		}
	}
	sub(r.Args)
	if r.Cond == from {
		r.Cond = to
	}
	if r.T == TBranch {
		renameIn(r.Then, from, to)
		renameIn(r.Else, from, to)
	}
}
