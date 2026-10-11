// Package presburger decides linear arithmetic over the integers: whether a
// conjunction of linear equalities and inequalities over ℤ has a solution.
//
// It is the first of ADR 0049's own complete decision procedures: the
// quantifier-free fragment of Presburger arithmetic (Presburger 1929), decided
// by Pugh's Omega test ("The Omega test: a fast and practical integer
// programming algorithm for dependence analysis", Supercomputing 1991).
//
// THE ALGORITHM, in three parts.
//
//   - NORMALIZATION. A constraint Σaᵢxᵢ + k ≥ 0 is divided by g = gcd(aᵢ), its
//     constant rounded down (the solutions are integers); an equality whose
//     constant g does not divide has none.
//   - EQUALITIES are eliminated exactly. A variable with coefficient ±1 is
//     solved for and substituted. Otherwise Pugh's mod-hat step introduces a new
//     variable σ with mσ = Σ (aᵢ mod̂ m) xᵢ, m = |aₖ| + 1 for the least |aₖ|, in
//     which xₖ has coefficient ∓1 and is solved for; the coefficients shrink, so
//     the step terminates.
//   - INEQUALITIES. A variable bounded on one side only is dropped with its
//     constraints, since it can be taken as large as needed. Otherwise a
//     variable z is eliminated against each pair of a lower bound L ≤ a·z and an
//     upper bound b·z ≤ U: the REAL SHADOW a·U − b·L ≥ 0 is Fourier–Motzkin's,
//     and the DARK SHADOW a·U − b·L ≥ (a − 1)(b − 1) holds only where an integer
//     z lies between. When every pair has a = 1 or b = 1 the two coincide and the
//     elimination is exact. Otherwise: a solution of the dark shadow is one of
//     the system; no solution of the real shadow means none of the system; and
//     between the two, the SPLINTERS a·z = L + i for i = 0 … ⌊(a·m − a − m)/m⌋,
//     m the largest upper coefficient, are each decided.
//
// The procedure is complete for the integers. It may still answer "unknown":
// when a coefficient would leave int64, or past a budget of steps (the problem
// is NP-complete, and the worst case of the splinters is exponential). Unknown is
// reported as such and never as an answer.
package presburger

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

// A Constraint is Σ Coef[x]·x + K ≥ 0, or = 0 when Eq.
type Constraint struct {
	Coef map[string]int64
	K    int64
	Eq   bool
}

// Result is the answer to a satisfiability question.
type Result int

const (
	Unsat   Result = iota // no integer solution
	Sat                   // an integer solution exists
	Unknown               // a coefficient overflowed, or the budget ran out
)

func (r Result) String() string { return [...]string{"unsat", "sat", "unknown"}[r] }

// Budget bounds the work of one question: steps of elimination and splinters.
const Budget = 200000

var (
	errOverflow = errors.New("overflow")
	errBudget   = errors.New("budget")
)

// Satisfiable decides whether the conjunction of cs has a solution in ℤ.
func Satisfiable(cs []Constraint) (r Result) {
	s := &solver{budget: Budget}
	defer func() {
		if e := recover(); e != nil {
			if e == errOverflow || e == errBudget {
				r = Unknown
				return
			}
			panic(e)
		}
	}()
	p := make([]row, 0, len(cs))
	for _, c := range cs {
		r := row{coef: map[string]int64{}, k: c.K, eq: c.Eq}
		for x, a := range c.Coef {
			if a != 0 {
				r.coef[x] = a
			}
		}
		p = append(p, r)
	}
	if s.solve(p) {
		return Sat
	}
	return Unsat
}

type row struct {
	coef map[string]int64
	k    int64
	eq   bool
}

func (r row) String() string {
	op := ">="
	if r.eq {
		op = "="
	}
	return fmt.Sprintf("%v + %d %s 0", r.coef, r.k, op)
}

func (r row) clone() row {
	c := row{coef: make(map[string]int64, len(r.coef)), k: r.k, eq: r.eq}
	for x, a := range r.coef {
		c.coef[x] = a
	}
	return c
}

type solver struct {
	budget int
	fresh  int
}

func (s *solver) tick() {
	s.budget--
	if s.budget < 0 {
		panic(errBudget)
	}
}

// solve decides a conjunction of rows.
func (s *solver) solve(p []row) bool {
	s.tick()
	p, ok := normalize(p)
	if !ok {
		return false
	}
	for i, r := range p {
		if r.eq {
			return s.solve(s.eliminateEquality(p, i))
		}
	}
	return s.solveInequalities(p)
}

// normalize divides every row by the gcd of its coefficients, decides the rows
// with none, and reports false on a row no integer satisfies.
func normalize(p []row) ([]row, bool) {
	out := p[:0:0]
	for _, r := range p {
		if len(r.coef) == 0 {
			if r.eq && r.k != 0 || !r.eq && r.k < 0 {
				return nil, false
			}
			continue
		}
		g := int64(0)
		for _, a := range r.coef {
			g = gcd(g, abs(a))
		}
		if g > 1 {
			r = r.clone() // rows are shared between branches: never divide one in place
			if r.eq {
				if r.k%g != 0 {
					return nil, false
				}
				r.k /= g
			} else {
				r.k = floorDiv(r.k, g)
			}
			for x, a := range r.coef {
				r.coef[x] = a / g
			}
		}
		out = append(out, r)
	}
	return out, true
}

// eliminateEquality removes the equality p[i] exactly: a unit coefficient is
// solved for; otherwise Pugh's mod-hat step makes one.
func (s *solver) eliminateEquality(p []row, i int) []row {
	e := p[i]
	k, ak := "", int64(0)
	for _, x := range sortedVars(e.coef) {
		a := e.coef[x]
		if ak == 0 || abs(a) < abs(ak) {
			k, ak = x, a
		}
	}
	if abs(ak) == 1 {
		// xₖ = −(Σ_{j≠k} aⱼxⱼ + c)/aₖ, and aₖ = ±1.
		def := row{coef: map[string]int64{}, k: mul(-e.k, ak)}
		for x, a := range e.coef {
			if x != k {
				def.coef[x] = mul(-a, ak)
			}
		}
		out := make([]row, 0, len(p)-1)
		for j, r := range p {
			if j != i {
				out = append(out, substitute(r, k, def))
			}
		}
		return out
	}
	// THE MOD-HAT STEP. m = |aₖ| + 1, and σ is new with
	//   m·σ = Σᵢ (aᵢ mod̂ m)·xᵢ + (c mod̂ m),
	// in which aₖ mod̂ m = −sign(aₖ), so
	//   xₖ = sign(aₖ)·(Σ_{i≠k} (aᵢ mod̂ m)·xᵢ + (c mod̂ m) − m·σ).
	m := abs(ak) + 1
	sign := int64(1)
	if ak < 0 {
		sign = -1
	}
	s.fresh++
	sigma := fmt.Sprintf("σ%d", s.fresh)
	def := row{coef: map[string]int64{sigma: mul(-m, sign)}, k: mul(modHat(e.k, m), sign)}
	for x, a := range e.coef {
		if x != k {
			if h := modHat(a, m); h != 0 {
				def.coef[x] = mul(h, sign)
			}
		}
	}
	out := make([]row, len(p))
	for j, r := range p {
		out[j] = substitute(r, k, def)
	}
	return out
}

// modHat is Pugh's symmetric residue, a − m·⌊a/m + 1/2⌋.
func modHat(a, m int64) int64 {
	return a - m*floorDiv(add(mul(2, a), m), mul(2, m))
}

// substitute replaces x in r by the linear expression def (coefficients and
// constant).
func substitute(r row, x string, def row) row {
	a, ok := r.coef[x]
	if !ok {
		return r
	}
	out := r.clone()
	delete(out.coef, x)
	for y, b := range def.coef {
		out.coef[y] = add(out.coef[y], mul(a, b))
		if out.coef[y] == 0 {
			delete(out.coef, y)
		}
	}
	out.k = add(out.k, mul(a, def.k))
	return out
}

// solveInequalities decides a conjunction of inequalities.
func (s *solver) solveInequalities(p []row) bool {
	if len(p) == 0 {
		return true
	}
	// A variable bounded on one side only can be taken as far as its
	// constraints need: they are all satisfiable, and leave with it.
	lower, upper := map[string]int{}, map[string]int{}
	for _, r := range p {
		for x, a := range r.coef {
			if a > 0 {
				lower[x]++
			} else {
				upper[x]++
			}
		}
	}
	for _, x := range sortedKeys(lower, upper) {
		if lower[x] == 0 || upper[x] == 0 {
			var rest []row
			for _, r := range p {
				if _, in := r.coef[x]; !in {
					rest = append(rest, r)
				}
			}
			return s.solve(rest)
		}
	}
	// Contradictory parallel rows, a·x + k₁ ≥ 0 and −a·x + k₂ ≥ 0 with
	// k₁ + k₂ < 0, are the common case and need no elimination.
	if contradictoryPair(p) {
		return false
	}
	z, exact := pickVariable(p, lower, upper)
	var los, ups, rest []row
	for _, r := range p {
		switch a := r.coef[z]; {
		case a > 0:
			los = append(los, r)
		case a < 0:
			ups = append(ups, r)
		default:
			rest = append(rest, r)
		}
	}
	shadow := func(dark bool) []row {
		out := append([]row(nil), rest...)
		for _, lo := range los {
			a := lo.coef[z]
			for _, up := range ups {
				b := -up.coef[z]
				// lo: a·z + L ≥ 0, up: −b·z + U ≥ 0, so b·L' … : the
				// combination b·lo + a·up eliminates z.
				r := row{coef: map[string]int64{}}
				for x, c := range lo.coef {
					if x != z {
						r.coef[x] = add(r.coef[x], mul(b, c))
					}
				}
				for x, c := range up.coef {
					if x != z {
						r.coef[x] = add(r.coef[x], mul(a, c))
					}
				}
				for x, c := range r.coef {
					if c == 0 {
						delete(r.coef, x)
					}
				}
				r.k = add(mul(b, lo.k), mul(a, up.k))
				if dark {
					r.k = add(r.k, -mul(a-1, b-1))
				}
				out = append(out, r)
			}
		}
		return out
	}
	if exact {
		return s.solve(shadow(false))
	}
	if s.solve(shadow(true)) {
		return true
	}
	if !s.solve(shadow(false)) {
		return false
	}
	// THE SPLINTERS. An integer solution of the real shadow that the dark
	// shadow misses lies close to a lower bound: a·z = −L' + i for some i in
	// 0 … ⌊(a·m − a − m)/m⌋, m the largest upper coefficient of z.
	m := int64(0)
	for _, up := range ups {
		if b := -up.coef[z]; b > m {
			m = b
		}
	}
	for _, lo := range los {
		a := lo.coef[z]
		top := floorDiv(add(mul(a, m), -add(a, m)), m)
		for i := int64(0); i <= top; i++ {
			eq := lo.clone()
			eq.eq = true
			eq.k = add(eq.k, -i)
			q := append(append([]row(nil), p...), eq)
			if s.solve(q) {
				return true
			}
		}
	}
	return false
}

// contradictoryPair finds two rows that are exact negations up to a constant
// that no integer meets.
func contradictoryPair(p []row) bool {
	for i := range p {
		for j := i + 1; j < len(p); j++ {
			if len(p[i].coef) != len(p[j].coef) {
				continue
			}
			neg := true
			for x, a := range p[i].coef {
				if p[j].coef[x] != -a {
					neg = false
					break
				}
			}
			if neg && add(p[i].k, p[j].k) < 0 {
				return true
			}
		}
	}
	return false
}

// pickVariable chooses the variable to eliminate: one whose elimination is
// exact if any, and of those the one making the fewest combinations.
func pickVariable(p []row, lower, upper map[string]int) (string, bool) {
	best, bestExact, bestCost := "", false, math.MaxInt
	for _, x := range sortedKeys(lower, upper) {
		exact := true
		for _, r := range p {
			if a := r.coef[x]; a > 1 {
				for _, q := range p {
					if b := -q.coef[x]; b > 1 {
						exact = false
					}
				}
			}
		}
		cost := lower[x] * upper[x]
		if best == "" || exact && !bestExact || exact == bestExact && cost < bestCost {
			best, bestExact, bestCost = x, exact, cost
		}
	}
	return best, bestExact
}

func sortedVars(m map[string]int64) []string {
	out := make([]string, 0, len(m))
	for x := range m {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}

func sortedKeys(a, b map[string]int) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range []map[string]int{a, b} {
		for x := range m {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	sort.Strings(out)
	return out
}

// ---- arithmetic, every step checked: an overflow is "unknown", never a
// wrong answer ----

func add(a, b int64) int64 {
	c := a + b
	if (c > a) != (b > 0) {
		panic(errOverflow)
	}
	return c
}

func mul(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	c := a * b
	if c/b != a || a == -1 && b == math.MinInt64 || b == -1 && a == math.MinInt64 {
		panic(errOverflow)
	}
	return c
}

func abs(a int64) int64 {
	if a == math.MinInt64 {
		panic(errOverflow)
	}
	if a < 0 {
		return -a
	}
	return a
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// floorDiv is ⌊a/b⌋ for b > 0.
func floorDiv(a, b int64) int64 {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}
