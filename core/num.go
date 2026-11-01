package core

import (
	"fmt"
	"math"
	"math/big"
	"strings"
)

// A NUMERAL OF SORT N = ℤ[1/10] (ADR 0086, spec/numbers.md §1).
//
// N is the directed union of the free ℤ-modules Nₖ = 10⁻ᵏℤ, and each x ∈ N has
// one canonical form (m, k): k ≥ 0 the least with x ∈ Nₖ, its scale, and
// m = x·10ᵏ, so 10 ∤ m when k ≥ 1. A numeral written with a point or an
// exponent (`0.1`, `2.50`, `1e-3`, `0.0`) is a constant DECLARED AT SORT N, as
// an order-sorted signature may declare a constant at a sort above its least
// one (Goguen and Meseguer 1992): `0.0` and `0` are the same element, and only
// `0` is also declared at ℤ. The value is exact. `0.1` is one tenth, and
// nothing rounds it unless the program writes a rounding.
type Dec struct {
	M *big.Int // m; 10 ∤ m when K ≥ 1
	K int      // the scale k ≥ 0
}

var ten = big.NewInt(10)

// ParseNum reads a numeral, digits with an optional fraction and an optional
// exponent (`0.1`, `2.50`, `1e-3`, `6.02e23`), as the element of N it denotes.
func ParseNum(text string) (*Dec, error) {
	s := text
	neg := strings.HasPrefix(s, "-")
	if neg || strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	exp := 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		var e int
		if _, err := fmt.Sscanf(s[i+1:], "%d", &e); err != nil {
			return nil, fmt.Errorf("%q is not a numeral", text)
		}
		if e > 1<<20 || e < -(1<<20) {
			return nil, fmt.Errorf("%q: the exponent is past what this compiler holds (|e| ≤ 2²⁰)", text)
		}
		exp, s = e, s[:i]
	}
	digits := s
	if i := strings.IndexByte(s, '.'); i >= 0 {
		digits = s[:i] + s[i+1:]
		exp -= len(s) - i - 1
	}
	if digits == "" || strings.ContainsAny(digits, "+-.") {
		return nil, fmt.Errorf("%q is not a numeral", text)
	}
	m, ok := new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, fmt.Errorf("%q is not a numeral", text)
	}
	if neg {
		m.Neg(m)
	}
	return NewDec(m, -exp), nil
}

// NewDec is m·10⁻ᵏ, for any integer k, in canonical form.
func NewDec(m *big.Int, k int) *Dec {
	m = new(big.Int).Set(m)
	if m.Sign() == 0 {
		return &Dec{M: m, K: 0}
	}
	r := new(big.Int)
	for k > 0 {
		q, rem := new(big.Int).QuoRem(m, ten, r)
		if rem.Sign() != 0 {
			break
		}
		m, k = q, k-1
	}
	if k < 0 {
		m.Mul(m, new(big.Int).Exp(ten, big.NewInt(int64(-k)), nil))
		k = 0
	}
	return &Dec{M: m, K: k}
}

// DecOfFloat64 is the element of N a finite double denotes, exactly: every
// binary float is m·2ᵉ = m·5⁻ᵉ·10ᵉ (Theorem R).
func DecOfFloat64(f float64) *Dec {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		panic("DecOfFloat64: ∞ and NaN are not numbers of N")
	}
	r, _ := new(big.Float).SetFloat64(f).Rat(nil)
	num, den := new(big.Int).Set(r.Num()), r.Denom() // den = 2ʲ
	k := 0
	for p := big.NewInt(1); new(big.Int).Rem(p, den).Sign() != 0; p.Mul(p, ten) {
		k++
	}
	// m/2ʲ = m·5ʲ/10ʲ, and k = j
	num.Mul(num, new(big.Int).Exp(big.NewInt(5), big.NewInt(int64(k)), nil))
	return NewDec(num, k)
}

// Decimal is a numeral of sort N.
func Decimal(d *Dec) *Term { return &Term{Kind: KDec, Dec: d} }

// Rat is the element as a rational.
func (d *Dec) Rat() *big.Rat {
	return new(big.Rat).SetFrac(d.M, new(big.Int).Exp(ten, big.NewInt(int64(d.K)), nil))
}

// IsInt reports an element of ℤ.
func (d *Dec) IsInt() bool { return d.K == 0 }

// Float64 is the nearest double, and whether it is the element exactly: whether
// the element lies in F64, which a double represents (spec/numbers.md §5).
func (d *Dec) Float64() (float64, bool) {
	f, exact := d.Rat().Float64()
	return f, exact && !math.IsInf(f, 0)
}

// Double is the nearest double: the element itself where it lies in F64.
func (d *Dec) Double() float64 {
	f, _ := d.Float64()
	return f
}

// Equal is equality of elements, which is of canonical forms.
func (d *Dec) Equal(e *Dec) bool { return d.K == e.K && d.M.Cmp(e.M) == 0 }

// String is the exact numeral, which every reader reads back as the same
// element at sort N: positional while the point is near, `me-k` past it, and an
// integer with `.0`.
func (d *Dec) String() string {
	neg := d.M.Sign() < 0
	digits := new(big.Int).Abs(d.M).String()
	var s string
	switch {
	case d.K == 0:
		s = digits + ".0"
	case d.K > 40:
		s = digits + fmt.Sprintf("e-%d", d.K)
	case len(digits) > d.K:
		s = digits[:len(digits)-d.K] + "." + digits[len(digits)-d.K:]
	default:
		s = "0." + strings.Repeat("0", d.K-len(digits)) + digits
	}
	if neg {
		return "-" + s
	}
	return s
}

// RoundName is the rounding's former in the core, (round "S" e "m") (ADR 0086,
// numbers.md §3): S a set's canonical type, e the term rounded, m the mode.
const RoundName = "round"

// QuoName is the real quotient a ÷ b, written only at the root of a rounding's
// argument (Theorem T).
const QuoName = "÷"

// RoundScopeName is the scope round(S) { body } in the abstract syntax, and
// RoundScoped the fifth word of a rounding the scope wrote, (round "S" e "m"
// "scope"): onto S when e is N's, and the identity on an integer (numbers.md §3).
const (
	RoundScopeName = "#scope"
	RoundScoped    = "scope"
)

// ScopeRounds is the scope's rewrite of its body t onto set (a canonical type,
// as a string term): every +, −, · and ÷ becomes its rounding, and so does
// every numeral, which is folded when it is in S. A rounding the program wrote
// is its own, and its argument is left as written; comparisons, integer
// operators and calls are not rewritten. The body is a closed term and the
// rewrite adds no binder, so a λ's body is rewritten as it stands.
func ScopeRounds(set *Term, t *Term) *Term {
	wrap := func(e *Term) *Term {
		return &Term{Kind: KApp, Kids: []*Term{Name(RoundName), set, e, Str(RoundNearest), Str(RoundScoped)}}
	}
	switch t.Kind {
	case KDec:
		return wrap(t)
	case KFn:
		n := *t
		n.Kids = []*Term{ScopeRounds(set, t.Kids[0])}
		return &n
	case KApp:
		if len(t.Kids) > 0 && t.Kids[0].Kind == KName && t.Kids[0].Name == RoundName {
			return t
		}
		n := *t
		n.Kids = make([]*Term, len(t.Kids))
		for i, k := range t.Kids {
			n.Kids[i] = ScopeRounds(set, k)
		}
		if len(n.Kids) == 3 && n.Kids[0].Kind == KName {
			switch n.Kids[0].Name {
			case "+", "-", "*", QuoName:
				return wrap(&n)
			}
		}
		return &n
	}
	return t
}

// RoundNearest is the default mode: the nearer neighbour, a tie to the even one.
const RoundNearest = "nearest"

// RoundModes are the modes a rounding takes (numbers.md §3), by their names.
var RoundModes = map[string]bool{
	"nearest": true, "half-away": true, "down": true, "up": true, "zero": true, "away": true, "sat": true,
}

// DecType is D(k, lo..hi)'s canonical type, "dec K LO HI": the elements of
// 10⁻ᵏℤ between lo and hi (numbers.md §2).
func DecType(k int, lo, hi *big.Int) string {
	return fmt.Sprintf("dec %d %s %s", k, lo, hi)
}

// DecRange reads "dec K LO HI" back.
func DecRange(ty string) (k int, lo, hi *big.Int, ok bool) {
	f := strings.Fields(ty)
	if len(f) != 4 || f[0] != "dec" {
		return 0, nil, nil, false
	}
	if _, err := fmt.Sscanf(f[1], "%d", &k); err != nil || k < 0 || k > 1000 {
		return 0, nil, nil, false
	}
	lo, ok1 := new(big.Int).SetString(f[2], 10)
	hi, ok2 := new(big.Int).SetString(f[3], 10)
	if !ok1 || !ok2 || lo.Cmp(hi) > 0 {
		return 0, nil, nil, false
	}
	return k, lo, hi, true
}

// DecSetName is D, the decimal grid's former in a set, D(k, lo..hi).
const DecSetName = "D"

// rangeType reads a range written in a term, (.. lo hi), as the type former
// (int lo hi) a header's set already is, and a decimal grid's range with it.
func rangeType(t *Term) *Term {
	if t != nil && t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName {
		switch t.Kids[0].Name {
		case "..":
			return &Term{Kind: KApp, Kids: []*Term{Name("int"), t.Kids[1], t.Kids[2]}}
		case DecSetName:
			return &Term{Kind: KApp, Kids: []*Term{t.Kids[0], t.Kids[1], rangeType(t.Kids[2])}}
		}
	}
	return t
}

// NumType is the sort N = ℤ[1/10] of a value of the language's arithmetic that
// is not an integer (ADR 0086, numbers.md §1): ℤ ≤ N, so an integer goes where
// a number does, and not back. The host's double is still `f64`, its own type,
// until numbers.md §9 part 4 makes F64 a set of N with the double its
// representation.
const NumType = "num"

// F64Name is F64's canonical type, binary64's finite values, which a double
// represents (numbers.md §2, §5).
const F64Name = "f64"

// F64MaxRat is F64's greatest element, (2 − 2⁻⁵²)·2¹⁰²³.
func F64MaxRat() *big.Rat { return new(big.Rat).SetFloat64(math.MaxFloat64) }

// F64Type is F64 ∩ [lo, hi]'s canonical type, "f64 LO HI", each end an exact
// numeral: the grid restricted to an interval (numbers.md §2), which a
// separation { x ∈ F64 : lo <= x && x <= hi } is.
func F64Type(lo, hi *Dec) string { return "f64 " + lo.String() + " " + hi.String() }

// IsF64Set reports F64 or F64 restricted to an interval.
func IsF64Set(ty string) bool {
	if ty == F64Name {
		return true
	}
	_, _, ok := F64Range(ty)
	return ok
}

// F64Range reads "f64 LO HI" back.
func F64Range(ty string) (lo, hi *Dec, ok bool) {
	f := strings.Fields(ty)
	if len(f) != 3 || f[0] != F64Name {
		return nil, nil, false
	}
	l, err1 := ParseNum(f[1])
	h, err2 := ParseNum(f[2])
	if err1 != nil || err2 != nil || l.Rat().Cmp(h.Rat()) > 0 {
		return nil, nil, false
	}
	return l, h, true
}

// F64GridName is the hidden former of F64 ∩ [lo, hi] in a term, (#f64 lo hi),
// which TypeName reads as F64Type: a name a program cannot write.
const F64GridName = "#f64"

// SeparatedGrid is the normal form of a separation { x ∈ S : φ } whose base S
// is a grid restricted to an interval, G ∩ [a₀, b₀] (F64, or F64 ∩ [lo, hi];
// ℤ waits, gridOfExpr), and whose φ is a conjunction of bounds of x by constants
// (numbers.md §2). Its characteristic function is χ_G ∧ a ≤ x ∧ x ≤ b, so the
// set is G ∩ [a, b], and that has ONE spelling by the Galois connection of
// G ↪ ℚ: G ∩ [a, b] = G ∩ [⌈a⌉_G, ⌊b⌋_G], whose ends are elements of G. A
// strict bound is the same adjunction applied once more: x < b is x ≤ the
// greatest element of G below b. An end
// not stated is S's own (F64's are ±MAX, every element of F64 lying between
// them). Any other formula, or an empty set, is not this shape: the separation
// stays one. The result is the set's expression: F64 ∩ [lo, hi] (F64GridExpr),
// or F64 itself when its ends are F64's.
func SeparatedGrid(e Expr) (Expr, bool) {
	a, isApp := e.(*Apply)
	if !isApp || len(a.Args) != 3 {
		return nil, false
	}
	v, isVar := a.Fn.(*Var)
	x, isBinder := a.Args[0].(*Var)
	if !isVar || v.Name != SepName || !isBinder {
		return nil, false
	}
	g, lq, hq, ok := gridOfExpr(a.Args[1])
	if !ok {
		return nil, false
	}
	var conj []Expr
	if and, isAnd := a.Args[2].(*And); isAnd {
		conj = and.Xs
	} else {
		conj = []Expr{a.Args[2]}
	}
	for _, c := range conj {
		cmp, ok := c.(*Apply)
		if !ok || len(cmp.Args) != 2 {
			return nil, false
		}
		op, ok := cmp.Fn.(*Var)
		if !ok {
			return nil, false
		}
		rel := op.Name
		l, r := cmp.Args[0], cmp.Args[1]
		if isVarNamed(r, x.Name) { // c rel x is x rel' c
			l, r = r, l
			rel = map[string]string{"<=": ">=", ">=": "<=", "<": ">", ">": "<"}[rel]
		}
		q, isConst := constRat(r)
		if !isVarNamed(l, x.Name) || !isConst {
			return nil, false
		}
		var end *big.Rat
		switch rel {
		case "<=", "<":
			end = g.Round(q, "down") // the greatest element ≤ q
			if rel == "<" && end.Cmp(q) == 0 {
				end = below(g, q)
			}
			if lowerBound(end, hq) {
				hq = end
			}
		case ">=", ">":
			end = g.Round(q, "up")
			if rel == ">" && end.Cmp(q) == 0 {
				end = new(big.Rat).Neg(below(g, new(big.Rat).Neg(q)))
			}
			if lq == nil || end.Cmp(lq) > 0 {
				lq = end
			}
		default:
			return nil, false
		}
	}
	if lq != nil && hq != nil && lq.Cmp(hq) > 0 {
		return nil, false
	}
	m := g.Max()
	if lq.Cmp(new(big.Rat).Neg(m)) == 0 && hq.Cmp(m) == 0 {
		return &Var{Name: F64Name}, true
	}
	return F64GridExpr(DecOfRat(lq), DecOfRat(hq)), true
}

// lowerBound reports a new upper end tighter than the current, nil for none.
func lowerBound(end, cur *big.Rat) bool { return cur == nil || end.Cmp(cur) < 0 }

// below is the greatest element of g strictly below q, for q ∈ g: one spacing
// down, the spacing of the cell below q (at a binade's bottom, the binade below).
func below(g Grid, q *big.Rat) *big.Rat {
	probe := new(big.Rat).Sub(q, new(big.Rat).Quo(g.Spacing(q), big.NewRat(4, 1)))
	return g.Round(probe, "down")
}

// gridOfExpr is a separation's base as a grid and its interval's ends: F64,
// or F64 ∩ [lo, hi].
//
// NOT ℤ YET, though its normal form is the range ⌈a⌉..⌊b⌋: a range with an
// infinite end on a result is read as a representation, not a claim
// (read.go), so `Nat` and `10..` there are decided by nothing (grids-
// 2026-11-01), while a separation of ℤ is a `where` and an `ensure`, decided.
// The normal form waits on that repair, so that it never makes a decided claim
// undecided.
func gridOfExpr(s Expr) (Grid, *big.Rat, *big.Rat, bool) {
	switch s := s.(type) {
	case *Var:
		if s.Name == F64Name || s.Name == "F64" {
			m := GridF64.Max()
			return GridF64, new(big.Rat).Neg(m), m, true
		}
	case *Apply:
		if f, ok := s.Fn.(*Var); ok && f.Name == F64GridName && len(s.Args) == 2 {
			lo, ok1 := constRat(s.Args[0])
			hi, ok2 := constRat(s.Args[1])
			if ok1 && ok2 {
				return GridF64, lo, hi, true
			}
		}
	}
	return Grid{}, nil, nil, false
}

// F64GridExpr is F64 ∩ [lo, hi] in the abstract syntax.
func F64GridExpr(lo, hi *Dec) Expr {
	return &Apply{Fn: &Var{Name: F64GridName}, Args: []Expr{&Lit{T: Decimal(lo)}, &Lit{T: Decimal(hi)}}}
}

func isVarNamed(e Expr, name string) bool {
	v, ok := e.(*Var)
	return ok && v.Name == name
}

// constRat is a constant's exact value: a literal, a negation of one, or an
// integer power of integer literals.
func constRat(e Expr) (*big.Rat, bool) {
	switch e := e.(type) {
	case *Lit:
		switch e.T.Kind {
		case KInt:
			return new(big.Rat).SetInt64(e.T.Int), true
		case KDec:
			return e.T.Dec.Rat(), true
		}
		if v, ok := EvalEndpoint(e.T); ok { // a literal past int64, a term (bigLiteral)
			return new(big.Rat).SetInt(v), true
		}
	case *Apply:
		f, ok := e.Fn.(*Var)
		if !ok {
			return nil, false
		}
		switch {
		case (f.Name == "-" || f.Name == "neg") && len(e.Args) == 1:
			q, ok := constRat(e.Args[0])
			return new(big.Rat).Neg(q), ok && q != nil
		case f.Name == "-" && len(e.Args) == 2:
			a, ok1 := constRat(e.Args[0])
			b, ok2 := constRat(e.Args[1])
			if ok1 && ok2 {
				return new(big.Rat).Sub(a, b), true
			}
		case f.Name == "^" && len(e.Args) == 2:
			a, ok1 := constRat(e.Args[0])
			b, ok2 := constRat(e.Args[1])
			if ok1 && ok2 && a.IsInt() && b.IsInt() && b.Sign() >= 0 && b.Num().IsInt64() && b.Num().Int64() <= 4096 {
				return new(big.Rat).SetInt(new(big.Int).Exp(a.Num(), b.Num(), nil)), true
			}
		}
	}
	return nil, false
}
