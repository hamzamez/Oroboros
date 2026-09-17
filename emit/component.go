package emit

import (
	"oroboros/core"
)

// FACTS ABOUT ONE COMPONENT OF A STRIDED TABLE — array-facts.md §5.3.
//
// THEOREM (currying). Π_{s < k·n} V ≅ Π_{c < k} Π_{j < n} V: a table read at
// stride k is k tables, component c holding the slots s ≡ c (mod k). So a fact
//
//	∀s. s ≡ c (mod k) ∧ 0 ≤ s < len t → φ(t[s])
//
// is an ordinary F-D₁ fact about component c, and the periodic fragment
// (Habermehl, Iosif & Vojnar 2008) is never needed: the modular guard is decided
// by the INDEX, syntactically, not by the solver. It is written `(#at k c φ)`, and
// a bare φ is the case k = 1, so every other piece of the content machinery —
// printed-form intersection, fingerprints, Houdini — carries it unchanged.
//
// DECIDING THE GUARD IS A RING HOMOMORPHISM. Reduction mod m, ℤ → ℤ/mℤ, commutes
// with + and ×, so an index's residue is computed structurally:
//
//	L            exactly L
//	x + y, x − y (gcd(mx, my), rx ± ry)
//	L · x        (|L|·mx, L·rx)
//	if c a b     (gcd(ma, mb, |ra − rb|), ra)     — both branches are ≡ ra mod that
//	Σ aᵥ·v + b   (gcd(aᵥ), b)                     — a name's linear form
//
// with m = 1 meaning nothing is known and m = 0 meaning the value is exactly r.
// The fact applies at index i when k | m and r ≡ c (mod k); a store at i leaves
// component c untouched when k | m and r ≢ c (mod k) — McCarthy's second axiom,
// with the disequality i ≠ s proved by the congruence.

// residue is i ≡ r (mod m); m = 0 means i = r exactly, m = 1 means nothing.
type residue struct{ m, r int64 }

var noResidue = residue{m: 1}

func gcd64(a, b int64) int64 {
	if a < 0 {
		a = -a
	}
	if b < 0 {
		b = -b
	}
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func mod64(a, m int64) int64 {
	if m == 0 {
		return a
	}
	r := a % m
	if r < 0 {
		r += m
	}
	return r
}

func (x residue) norm() residue {
	if x.m == 1 {
		return noResidue
	}
	if x.m != 0 {
		x.r = mod64(x.r, x.m)
	}
	return x
}

func addRes(x, y residue, sign int64) residue {
	return residue{m: gcd64(x.m, y.m), r: x.r + sign*y.r}.norm()
}

func joinRes(x, y residue) residue {
	return residue{m: gcd64(gcd64(x.m, y.m), x.r-y.r), r: x.r}.norm()
}

// decides reports whether a residue decides membership of component c at stride
// k, and if so whether the index is in it.
func (x residue) decides(k, c int64) (decided, in bool) {
	if k <= 1 {
		return true, true
	}
	if x.m != 0 && x.m%k != 0 {
		return false, false
	}
	return true, mod64(x.r, k) == c
}

// residueOf is the congruence class of an integer index term under facts f.
func (r *refiner) residueOf(t *core.Term, f *facts) residue {
	return r.residueIn(t, f, map[string]residue{}, 0)
}

func (r *refiner) residueIn(t *core.Term, f *facts, env map[string]residue, depth int) residue {
	if t == nil || depth > 16 {
		return noResidue
	}
	switch t.Kind {
	case core.KInt:
		return residue{m: 0, r: t.Int}
	case core.KName:
		if x, ok := env[t.Name]; ok {
			return x
		}
		return linearResidue(f, t)
	case core.KApp:
	default:
		return noResidue
	}
	if t.Op().Kind != core.KName {
		return noResidue
	}
	if v, lam, isLet := asLet(r.tgt, t); isLet {
		inner := make(map[string]residue, len(env)+1)
		for k, x := range env {
			inner[k] = x
		}
		inner[lam.Params[0]] = r.residueIn(v, f, env, depth+1)
		return r.residueIn(lam.Body(), f, inner, depth+1)
	}
	args := t.Args()
	if p, known := r.tgt.Prims[t.Op().Name]; known && p.Kind == "cond" && len(args) == 3 {
		return joinRes(r.residueIn(args[1], f, env, depth+1), r.residueIn(args[2], f, env, depth+1))
	}
	if len(args) != 2 {
		return linearResidue(f, t)
	}
	switch arithOp(t.Op().Name, 2) {
	case "add", "sub":
		sign := int64(1)
		if arithOp(t.Op().Name, 2) == "sub" {
			sign = -1
		}
		return addRes(r.residueIn(args[0], f, env, depth+1), r.residueIn(args[1], f, env, depth+1), sign)
	case "mul":
		for i := range args {
			if lit := args[i]; lit.Kind == core.KInt {
				x := r.residueIn(args[1-i], f, env, depth+1)
				if x.m == 0 {
					return residue{m: 0, r: lit.Int * x.r}
				}
				return residue{m: gcd64(lit.Int, 0) * x.m, r: lit.Int * x.r}.norm()
			}
		}
	}
	return linearResidue(f, t)
}

// linearResidue reads a term's linear form: Σ aᵥ·v + b ≡ b (mod gcd(aᵥ)).
func linearResidue(f *facts, t *core.Term) residue {
	if f == nil {
		return noResidue
	}
	l, ok := f.lin(t)
	if !ok {
		return noResidue
	}
	l = f.substitute(l)
	var m int64
	for _, a := range l.coef {
		m = gcd64(m, a)
	}
	return residue{m: m, r: l.konst}.norm()
}

// atFact writes φ restricted to component c of stride k.
func atFact(k, c int64, phi *core.Term) *core.Term {
	if k <= 1 {
		return phi
	}
	return core.App(core.Name("#at"), core.Int(k), core.Int(c), phi)
}

// splitFact reads a fact as (k, c, φ); a bare φ is k = 1.
func splitFact(t *core.Term) (int64, int64, *core.Term) {
	if t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName && t.Op().Name == "#at" {
		if a := t.Args(); len(a) == 3 && a[0].Kind == core.KInt && a[1].Kind == core.KInt {
			return a[0].Int, a[1].Int, a[2]
		}
	}
	return 1, 0, t
}

// strides are the moduli k ≥ 2 the stores of a loop body use, each with its
// residues read off the index — the candidate components for Houdini.
func (r *refiner) strides(body *core.Term, f *facts) []int64 {
	seen := map[int64]bool{}
	var out []int64
	var walk func(t *core.Term, depth int)
	walk = func(t *core.Term, depth int) {
		if t == nil || depth > 64 {
			return
		}
		if t.Kind == core.KFn {
			walk(t.Body(), depth+1)
			return
		}
		if t.Kind != core.KApp {
			return
		}
		if t.Op().Kind == core.KName {
			if p, known := r.tgt.Prims[t.Op().Name]; known && p.Kind == "table-set" && len(t.Args()) == 3 {
				if x := r.residueOf(t.Args()[1], f); x.m >= 2 && x.m <= 8 && !seen[x.m] {
					seen[x.m] = true
					out = append(out, x.m)
				}
			}
		}
		for _, k := range t.Kids {
			walk(k, depth+1)
		}
	}
	walk(body, 0)
	return out
}
