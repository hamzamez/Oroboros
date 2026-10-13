package emit

import (
	"math/big"

	"oroboros/core"
)

// TRANSLATION INVARIANTS — Karr's affine relations (Karr, "Affine relationships
// among variables of a program", Acta Informatica 6, 1976) in the case where
// every back edge is a translation (claimsdemand-2026-10-13).
//
// THEOREM (translations). Let a loop's variables x̄ be updated on every back edge
// e by x̄ ↦ x̄ + δₑ, δₑ a vector of literals, on a set V of them. For every c with
// c·δₑ = 0 on every edge, c·x̄ is the same at every iteration: c·x̄ = c·z̄, z̄ the
// initial values. *Proof.* Induction on the iterations: c·(x̄ + δₑ) = c·x̄ + 0. ∎
// The c form the null space of the matrix whose rows are the δₑ, computed exactly
// over ℚ. A merge, which advances i or j and always k, has δ ∈ {(0,1,1), (1,0,1)}
// and so i + j − k = mid: the relation its reads need.
//
// They are CANDIDATES. Houdini (loopInvariants) still checks each at entry and at
// every back edge, so a wrong vector, or an edge this syntactic reading missed,
// costs a candidate and never a false fact.

// plantTranslationFault is a planted fault for TestTranslationInvariants: it
// replaces the null space by a vector no back edge preserves.
var plantTranslationFault = false

// translationCandidates gives each invariant as the pair (c·x̄, c·z̄).
func translationCandidates(lam *core.Term, inits []*core.Term) [][2]*core.Term {
	params := lam.Params
	if len(inits) < len(params) {
		return nil
	}
	edges := againArgs(lam.Body())
	if len(edges) == 0 {
		return nil
	}
	// δ for each variable on each edge; a variable whose argument is not v + L on
	// some edge is outside V.
	inV := make([]bool, len(params))
	for i := range inV {
		inV[i] = true
	}
	delta := make([][]int64, len(edges))
	for e, args := range edges {
		delta[e] = make([]int64, len(params))
		if len(args) != len(params) {
			return nil
		}
		for i, p := range params {
			l, ok := asLinear(args[i])
			if !ok {
				inV[i] = false
				continue
			}
			d := l.clone().addScaled(variable(p), -1)
			if len(d.coef) != 0 {
				inV[i] = false
				continue
			}
			delta[e][i] = d.konst
		}
	}
	var vars []int
	for i, ok := range inV {
		if ok {
			vars = append(vars, i)
		}
	}
	if len(vars) < 2 || len(vars) > 8 {
		return nil
	}
	rows := make([][]*big.Rat, len(edges))
	for e := range edges {
		rows[e] = make([]*big.Rat, len(vars))
		for j, i := range vars {
			rows[e][j] = new(big.Rat).SetInt64(delta[e][i])
		}
	}
	var out [][2]*core.Term
	basis := nullSpace(rows, len(vars))
	if plantTranslationFault {
		// A WRONG VECTOR, all ones, which no merge preserves: Houdini must drop it.
		ones := make([]int64, len(vars))
		for j := range ones {
			ones[j] = 1
		}
		basis = [][]int64{ones}
	}
	for _, c := range basis {
		nonzero := 0
		var lhs, rhs *core.Term
		for j, i := range vars {
			if c[j] == 0 {
				continue
			}
			nonzero++
			lhs = sumTerm(lhs, scaled(c[j], core.Name(params[i])))
			rhs = sumTerm(rhs, scaled(c[j], inits[i]))
		}
		// One variable alone is a variable no edge changes, which the templates
		// already relate to its initial value; four or more is past any relation
		// a program's index needs, and only costs Houdini rounds.
		if nonzero < 2 || nonzero > 3 {
			continue
		}
		out = append(out, [2]*core.Term{lhs, rhs})
	}
	return out
}

// againArgs collects the arguments of every `again` of this loop: the walk goes
// through λs (a `let`'s body, an arm's), and not into a nested loop's own body,
// whose `again`s are its own (the tail rule).
func againArgs(t *core.Term) [][]*core.Term {
	var out [][]*core.Term
	var walk func(t *core.Term, depth int)
	walk = func(t *core.Term, depth int) {
		if t == nil || depth > 256 {
			return
		}
		switch t.Kind {
		case core.KFn:
			walk(t.Body(), depth+1)
			return
		case core.KApp:
		default:
			return
		}
		if op := t.Op(); op.Kind == core.KName {
			if op.Name == "again" {
				out = append(out, t.Args())
				return
			}
			if loopKinds[op.Name] {
				for _, z := range t.Args()[1:] {
					walk(z, depth+1)
				}
				return
			}
		}
		for _, k := range t.Kids {
			walk(k, depth+1)
		}
	}
	walk(t, 0)
	return out
}

// nullSpace is a basis of {c : M c = 0} over ℚ, each vector scaled to coprime
// integers with its first nonzero entry positive.
func nullSpace(m [][]*big.Rat, n int) [][]int64 {
	a := make([][]*big.Rat, len(m))
	for i, row := range m {
		a[i] = make([]*big.Rat, n)
		for j := range row {
			a[i][j] = new(big.Rat).Set(row[j])
		}
	}
	pivotCol := []int{}
	r := 0
	for col := 0; col < n && r < len(a); col++ {
		p := -1
		for i := r; i < len(a); i++ {
			if a[i][col].Sign() != 0 {
				p = i
				break
			}
		}
		if p < 0 {
			continue
		}
		a[r], a[p] = a[p], a[r]
		inv := new(big.Rat).Inv(a[r][col])
		for j := 0; j < n; j++ {
			a[r][j].Mul(a[r][j], inv)
		}
		for i := range a {
			if i != r && a[i][col].Sign() != 0 {
				f := new(big.Rat).Set(a[i][col])
				for j := 0; j < n; j++ {
					a[i][j].Sub(a[i][j], new(big.Rat).Mul(f, a[r][j]))
				}
			}
		}
		pivotCol = append(pivotCol, col)
		r++
	}
	isPivot := map[int]int{}
	for i, c := range pivotCol {
		isPivot[c] = i
	}
	var basis [][]int64
	for free := 0; free < n; free++ {
		if _, p := isPivot[free]; p {
			continue
		}
		v := make([]*big.Rat, n)
		for j := range v {
			v[j] = new(big.Rat)
		}
		v[free].SetInt64(1)
		for row, pc := range pivotCol {
			v[pc].Neg(a[row][free])
		}
		basis = append(basis, integral(v))
	}
	return basis
}

// integral scales a rational vector to coprime integers, first nonzero positive.
func integral(v []*big.Rat) []int64 {
	l := big.NewInt(1)
	for _, x := range v {
		d := x.Denom()
		g := new(big.Int).GCD(nil, nil, l, d)
		l.Mul(l, new(big.Int).Quo(d, g))
	}
	ints := make([]*big.Int, len(v))
	g := new(big.Int)
	for i, x := range v {
		n := new(big.Int).Mul(x.Num(), new(big.Int).Quo(l, x.Denom()))
		ints[i] = n
		g.GCD(nil, nil, g, new(big.Int).Abs(n))
	}
	out := make([]int64, len(v))
	sign := int64(0)
	for i, n := range ints {
		if g.Sign() != 0 {
			n.Quo(n, g)
		}
		out[i] = n.Int64()
		if sign == 0 && out[i] != 0 {
			sign = 1
			if out[i] < 0 {
				sign = -1
			}
		}
	}
	for i := range out {
		out[i] *= sign
	}
	return out
}

func scaled(c int64, t *core.Term) *core.Term {
	if c == 1 {
		return t
	}
	return core.App(core.Name("*"), core.Int(c), t)
}

func sumTerm(a, b *core.Term) *core.Term {
	if a == nil {
		return b
	}
	return core.App(core.Name("+"), a, b)
}
