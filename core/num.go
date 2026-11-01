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
