package golang

import (
	"fmt"
	"math/big"

	"oroboros/core"
	"oroboros/ir"
)

// THE DOUBLE WORD ON GO (ADR 0103 on every host; sets-are-claims-research §6;
// the book's finding 118).
//
// E is ℤ/2¹²⁸ read by each value's fact, signed on (−2¹²⁷, 2¹²⁷) or unsigned
// on [0, 2¹²⁸) (ADR 0105), and Go holds an element as two uint64, its high and
// low digits in base 2⁶⁴, the two's complement of the signed reading: x =
// h·2⁶⁴ + l mod 2¹²⁸. Each operation is ℤ/2¹²⁸'s, by math/bits:
//
//	x + y:  l, c = Add64(xₗ, yₗ, 0);  h = xₕ + yₕ + c
//	x − y:  l, b = Sub64(xₗ, yₗ, 0);  h = xₕ − yₕ − b
//	−x:     0 − x
//	x · y:  (h₀, l) = Mul64(xₗ, yₗ);  h = h₀ + xₗ·yₕ + xₕ·yₗ   (xₕ·yₕ·2¹²⁸ ≡ 0)
//	x >> k: the digits' shifts, the bits crossing between them; logical for a
//	        natural, arithmetic otherwise
//
// each exact on the values the decision proved in E, the residue map being
// injective there. A word enters by its sign's extension, (x >> 63, x), and a
// natural by its zero's, (0, x).
//
// A READ-BACK is a function of the integer, into a word:
//   - + − · into the word: the low digits' ring operation, ℤ → ℤ/2⁶⁴ being a
//     ring homomorphism, so the high digits are not read;
//   - & with a mask in U: the low digits' &, the mask's high digit 0;
//   - x / d, x % d, x mod d, d a positive constant, the quotient proven in a
//     word: on the magnitude, Div64(|x|ₕ, |x|ₗ, d), whose precondition
//     |x|ₕ < d is the quotient's < 2⁶⁴ (d·q ≤ |x| < d·(q + 1)); the sign put
//     back by truncation, `/`'s and `%`'s, and `mod` lifted into [0, d);
//   - a comparison: lexicographic on (h, l), h signed for the signed reading
//     and unsigned for two naturals.
//
// A HIGH DIGIT IS PRINTED WHERE IT IS READ: by an intermediate's own high
// digit, a shift, a quotient, a remainder or a comparison, the need flowing
// back from each use to the operands (needHigh); Go refuses a variable
// declared and not used, and a digit no use reads is none of the integer's.

// isDbl says v is held in the double word.
func (p *printer) isDbl(v ir.V) bool { return p.dbl[v] || p.dbl[p.pl.Res(v)] }

// readsDouble says s computes an intermediate in E or reads one back.
func (p *printer) readsDouble(s *ir.Stmt) bool { return p.pl.ReadsDouble(s, p.dbl) }

// lo is v's lower end, by its type or its constant; ok false when unknown.
func (p *printer) loEnd(v ir.V) (*big.Int, bool) {
	if d := p.pl.Const[p.pl.Res(v)]; d != nil && d.Kind == core.KInt {
		return big.NewInt(d.Int), true
	}
	t := p.f.Types[p.pl.Res(v)]
	if lo, _, ok := core.IntRangeBig(t); ok && lo != nil {
		return lo, true // an intermediate's range, or a word's (never its Go type: a
		// range past the word would name math/big)
	}
	if t == core.U64Type || !p.isDbl(v) && p.typeOf(v) == "uint64" {
		return new(big.Int), true
	}
	return nil, false
}

// nat says v is a natural.
func (p *printer) nat(v ir.V) bool {
	lo, ok := p.loEnd(v)
	return ok && lo.Sign() >= 0
}

// halves are v's two digits as typed uint64 expressions: an intermediate's
// variables, a constant's literals, a word's sign or zero extension.
func (p *printer) halves(v ir.V) (h, l string) {
	if p.isDbl(v) {
		n := p.name(v)
		return n + "h", n + "l"
	}
	if d := p.pl.Const[p.pl.Res(v)]; d != nil && d.Kind == core.KInt {
		return fmt.Sprintf("uint64(%d)", uint64(d.Int>>63)), fmt.Sprintf("uint64(%d)", uint64(d.Int))
	}
	x := p.ref(v)
	if p.typeOf(v) == "uint64" {
		return "uint64(0)", x
	}
	return "uint64(" + x + " >> 63)", "uint64(" + x + ")"
}

// back is a uint64 expression of a read-back's result in the result's type.
func (p *printer) back(res ir.V, u64 string) string {
	if p.typeOf(res) == "uint64" {
		return u64
	}
	return p.tg.HostType("int") + "(" + u64 + ")"
}

// dop prints s, an operation of the double word.
func (p *printer) dop(s *ir.Stmt) {
	p.imps["math/bits"] = true
	res := s.Res[0]
	n := p.name(res)
	inter := s.Mode == ir.MDouble
	needH := inter && p.highs[p.pl.Res(res)]
	h := make([]string, len(s.Args))
	l := make([]string, len(s.Args))
	for i, a := range s.Args {
		h[i], l[i] = p.halves(a)
	}
	// the result: an intermediate's digits, or a word's value
	needL := !inter || p.lows[p.pl.Res(res)]
	lv := n + "l" // the low digit's variable, or `_` where no use reads it
	if !needL {
		lv = "_"
	}
	put := func(hExpr, lExpr string) {
		if inter {
			if needL {
				p.line("%sl := %s", n, lExpr)
			}
			if needH {
				p.line("%sh := %s", n, hExpr)
			}
			return
		}
		p.define(res, p.back(res, lExpr))
	}
	switch s.Op {
	case ir.OAdd, ir.OSub:
		f, sign := "Add64", "+"
		if s.Op == ir.OSub {
			f, sign = "Sub64", "-"
		}
		if !needH {
			put("", fmt.Sprintf("(%s %s %s)", l[0], sign, l[1]))
			return
		}
		p.line("%s, %sc := bits.%s(%s, %s, 0)", lv, n, f, l[0], l[1])
		if PlantDoubleCarryDropped {
			p.line("_ = %sc", n)
			p.line("%sh := %s %s %s", n, h[0], sign, h[1])
			return
		}
		p.line("%sh := %s %s %s %s %sc", n, h[0], sign, h[1], sign, n)
	case ir.ONeg:
		if !needH {
			put("", fmt.Sprintf("(0 - %s)", l[0]))
			return
		}
		p.line("%s, %sc := bits.Sub64(0, %s, 0)", lv, n, l[0])
		p.line("%sh := 0 - %s - %sc", n, h[0], n)
	case ir.OMul:
		if !needH {
			put("", fmt.Sprintf("(%s * %s)", l[0], l[1]))
			return
		}
		p.line("%sh0, %s := bits.Mul64(%s, %s)", n, lv, l[0], l[1])
		p.line("%sh := %sh0 + %s*%s + %s*%s", n, n, l[0], h[1], h[0], l[1])
	case ir.OAnd:
		put("", fmt.Sprintf("(%s & %s)", l[0], l[1]))
	case ir.OShr:
		p.shr(s, n, h[0], l[0], needH, needL)
	case ir.ODiv, ir.ORem, ir.OMod:
		p.divConst(s, n, h[0], l[0])
	case ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		p.define(res, p.cmp(s.Op, p.nat(s.Args[0]) && p.nat(s.Args[1]), h, l))
	default:
		p.fail("(%s …) of the double word is not printed on Go", s.Op)
	}
}

// shr is x >> k, k in [0, 127], logical for a natural and arithmetic otherwise.
func (p *printer) shr(s *ir.Stmt, n, xh, xl string, needH, needL bool) {
	arith := !p.nat(s.Args[0])
	high := func(k string) string { // the high digit's own shift
		if arith {
			return "uint64(int64(" + xh + ") >> " + k + ")"
		}
		return xh + " >> " + k
	}
	fill := "uint64(0)" // the high digit past 64
	if arith {
		fill = "uint64(int64(" + xh + ") >> 63)"
	}
	inter := s.Mode == ir.MDouble
	resL, resH := n+"l", n+"h"
	if !inter {
		resL = n + "sl"
	}
	k := s.Args[1]
	if d := p.pl.Const[p.pl.Res(k)]; d != nil && d.Kind == core.KInt {
		c := d.Int
		var lo, hi string
		switch {
		case c == 0:
			lo, hi = xl, xh
		case c < 64:
			lo, hi = fmt.Sprintf("(%s>>%d | %s<<%d)", xl, c, xh, 64-c), high(fmt.Sprint(c))
		default:
			lo, hi = high(fmt.Sprint(c-64)), fill
		}
		if needL {
			p.line("%s := %s", resL, lo)
		}
		if inter && needH {
			p.line("%s := %s", resH, hi)
		}
	} else {
		kk := "uint(" + p.ref(k) + ")"
		if needL {
			p.line("var %s uint64", resL)
		}
		if inter && needH {
			p.line("var %s uint64", resH)
		}
		p.line("if %s < 64 {", kk)
		p.ind++
		if needL {
			p.line("%s = %s>>%s | %s<<(64-%s)", resL, xl, kk, xh, kk)
		}
		if inter && needH {
			p.line("%s = %s", resH, high(kk))
		}
		p.ind--
		p.line("} else {")
		p.ind++
		if needL {
			p.line("%s = %s", resL, high(kk+"-64"))
		}
		if inter && needH {
			p.line("%s = %s", resH, fill)
		}
		p.ind--
		p.line("}")
	}
	if !inter {
		p.define(s.Res[0], p.back(s.Res[0], resL))
	}
}

// divConst is x / d, x % d or x mod d for a positive constant d, the quotient
// proven in a word: Div64 on the magnitude, whose high digit is then below d.
func (p *printer) divConst(s *ir.Stmt, n, xh, xl string) {
	d := p.pl.Const[p.pl.Res(s.Args[1])]
	if d == nil || d.Kind != core.KInt {
		// BY A WORD: a natural dividend, the quotient proven in the word
		// (ir/double.go's byWord), so its high digit is below the divisor
		q, r := "_", "_"
		if s.Op == ir.ODiv {
			q = n + "q"
		} else {
			r = n + "r"
		}
		p.line("%s, %s := bits.Div64(%s, %s, uint64(%s))", q, r, xh, xl, p.ref(s.Args[1]))
		if s.Op == ir.ODiv {
			p.define(s.Res[0], p.back(s.Res[0], q))
		} else {
			p.define(s.Res[0], p.back(s.Res[0], r))
		}
		return
	}
	if d.Int <= 0 {
		p.fail("a quotient of the double word by %d", d.Int)
		return
	}
	D := fmt.Sprintf("uint64(%d)", d.Int)
	// BY 2ᵏ, a shift: x mod 2ᵏ is x's low k bits whatever its sign (the 2-adic
	// residue); a natural's quotient is its digits shifted; a signed one's is
	// (x + [x < 0]·(2ᵏ − 1)) >> k, truncation's bias; a remainder is
	// x − q·2ᵏ, in the low digit since it is a word's
	if k := log2(d.Int); k > 0 && !PlantDoubleDivByShiftOff {
		mask := fmt.Sprintf("uint64(%d)", d.Int-1)
		if s.Op == ir.OMod {
			p.define(s.Res[0], p.back(s.Res[0], "("+xl+" & "+mask+")"))
			return
		}
		ql := n + "ql"
		if p.nat(s.Args[0]) {
			p.line("%s := %s>>%d | %s<<%d", ql, xl, k, xh, 64-k)
		} else {
			p.line("%sbl, %sbc := bits.Add64(%s, uint64(int64(%s)>>63)&%s, 0)", n, n, xl, xh, mask)
			p.line("%s := %sbl>>%d | (%s+%sbc)<<%d", ql, n, k, xh, n, 64-k)
		}
		if s.Op == ir.ODiv {
			p.define(s.Res[0], p.back(s.Res[0], ql))
		} else {
			p.define(s.Res[0], p.back(s.Res[0], fmt.Sprintf("(%s - %s<<%d)", xl, ql, k)))
		}
		return
	}
	q, r := "_", "_"
	if s.Op == ir.ODiv {
		q = n + "q"
	} else {
		r = n + "r"
	}
	// a remainder's quotient need not be a word, and Div64's precondition is
	// hi < d: Horner in base 2⁶⁴, x mod d = ((h mod d)·2⁶⁴ + l) mod d
	reduce := func(hi string) string {
		if s.Op == ir.ODiv || PlantRemOneDivision {
			return hi
		}
		p.line("_, %sr1 := bits.Div64(0, %s, %s)", n, hi, D)
		return n + "r1"
	}
	if p.nat(s.Args[0]) {
		xh = reduce(xh)
		p.line("%s, %s := bits.Div64(%s, %s, %s)", q, r, xh, xl, D)
		if s.Op == ir.ODiv {
			p.define(s.Res[0], p.back(s.Res[0], q))
		} else {
			p.define(s.Res[0], p.back(s.Res[0], r))
		}
		return
	}
	// the magnitude, divided, the sign put back
	sg, mh, ml := n+"s", n+"mh", n+"ml"
	p.line("%s := int64(%s) < 0", sg, xh)
	p.line("%s, %s := %s, %s", mh, ml, xh, xl)
	p.line("if %s {", sg)
	p.ind++
	p.line("var %sb uint64", n)
	p.line("%s, %sb = bits.Sub64(0, %s, 0)", ml, n, xl)
	p.line("%s = 0 - %s - %sb", mh, xh, n)
	p.ind--
	p.line("}")
	p.line("%s, %s := bits.Div64(%s, %s, %s)", q, r, reduce(mh), ml, D)
	v := q
	if s.Op != ir.ODiv {
		v = r
	}
	p.line("if %s {", sg)
	p.ind++
	p.line("%s = -%s", v, v)
	p.ind--
	p.line("}")
	if s.Op == ir.OMod {
		// the remainder, sign the dividend's, lifted into [0, d)
		p.line("if int64(%s) < 0 {", v)
		p.ind++
		p.line("%s += %s", v, D)
		p.ind--
		p.line("}")
	}
	p.define(s.Res[0], p.back(s.Res[0], v))
}

// cmp is a comparison of two elements of E, lexicographic on their digits.
func (p *printer) cmp(op ir.Op, unsigned bool, h, l []string) string {
	hi := func(x string) string {
		if unsigned {
			return x
		}
		return "int64(" + x + ")"
	}
	less := func(a, b int) string {
		return fmt.Sprintf("(%s < %s || %s == %s && %s < %s)", hi(h[a]), hi(h[b]), h[a], h[b], l[a], l[b])
	}
	eq := fmt.Sprintf("(%s == %s && %s == %s)", h[0], h[1], l[0], l[1])
	switch op {
	case ir.OEq:
		return eq
	case ir.ONe:
		return "(!" + eq + ")"
	case ir.OLt:
		return less(0, 1)
	case ir.OGt:
		return less(1, 0)
	case ir.OLe:
		return "(!" + less(1, 0) + ")"
	}
	return "(!" + less(0, 1) + ")" // OGe
}

// log2 is k where d = 2ᵏ, and 0 for any other d.
func log2(d int64) int {
	if d <= 1 || d&(d-1) != 0 {
		return 0
	}
	k := 0
	for d > 1 {
		d >>= 1
		k++
	}
	return k
}

// PlantDoubleDivByShiftOff divides by 2ᵏ with Div64, as any constant: the
// answers are the same, and only the time differs (a measurement's switch).
var PlantDoubleDivByShiftOff bool

// PlantDoubleCarryDropped leaves the low digits' carry out of the high digit
// of a sum: (−2⁶³ + −2⁶³)/2 is then not −2⁶³ (TestTheDoubleWordOnEveryHost).
var PlantDoubleCarryDropped bool

// PlantRemOneDivision divides a remainder's dividend in one Div64, whose
// precondition, the quotient a word's, a remainder does not give: Div64
// panics on a high digit at or past d (TestTheDoubleWordOnEveryHost).
var PlantRemOneDivision bool
