package java

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// THE DOUBLE WORD ON THE JVM (ADR 0103 on every host; sets-are-claims-research
// §6; the book's finding 118), the same algebra as ir/golang/double.go's: an
// element of E is two longs (h, l), x = h·2⁶⁴ + l mod 2¹²⁸, each read by its
// fact. The JVM has no carry and no 128-by-64 division, so each is derived
// from Java's own operations:
//
//   - the carry of l = a + b, unsigned, is the majority function's top bit,
//     ((a ∧ b) ∨ ((a ∨ b) ∧ ¬l)) >>> 63; the borrow of l = a − b is
//     ((¬a ∧ b) ∨ (¬(a ⊕ b) ∧ l)) >>> 63 (Warren, Hacker's Delight §2-16);
//   - the unsigned high product of a, b is Math.multiplyHigh's signed one
//     corrected, mulhs(a, b) + (a >> 63 ∧ b) + (b >> 63 ∧ a), since a's
//     unsigned value is a + 2⁶⁴·[a < 0] (Warren §8-3);
//   - a shift's count is taken mod 64 by Java, so a count of 0 or 64 and more
//     is its own case;
//   - x / 2ᵏ, x % 2ᵏ, x mod 2ᵏ by the shifts and the mask (Go's), and by any
//     other positive constant d, Knuth's two-digit division by one in base
//     2³² (Algorithm 4.3.1 D; Warren's divlu, §9-4) on the magnitude, its
//     precondition |x|ₕ < d the quotient's < 2⁶⁴, a static method of the class
//     (oroDivlu, registered once as a member beside the records).

// divlu is oroDivlu's text, a member of the program's class.
const divlu = `    // THE QUOTIENT OF A 128-BIT NATURAL BY A 64-BIT ONE, u1 < v so that it
    // fits 64 bits: Knuth's Algorithm D for two digits by one in base 2^32
    // (Warren, Hacker's Delight, divlu), each digit's estimate corrected at
    // most twice. ADR 0103 on the JVM (ir/java/double.go).
    static long oroDivlu(long u1, long u0, long v) {
        final long b = 1L << 32;
        final int s = Long.numberOfLeadingZeros(v);
        v <<= s;
        final long vn1 = v >>> 32, vn0 = v & 0xFFFFFFFFL;
        final long un32 = (u1 << s) | (s == 0 ? 0L : u0 >>> (64 - s));
        final long un10 = u0 << s;
        final long un1 = un10 >>> 32, un0 = un10 & 0xFFFFFFFFL;
        long q1 = Long.divideUnsigned(un32, vn1);
        long rhat = un32 - q1 * vn1;
        while (Long.compareUnsigned(q1, b) >= 0 || Long.compareUnsigned(q1 * vn0, b * rhat + un1) > 0) {
            q1--;
            rhat += vn1;
            if (Long.compareUnsigned(rhat, b) >= 0) break;
        }
        final long un21 = un32 * b + un1 - q1 * v;
        long q0 = Long.divideUnsigned(un21, vn1);
        rhat = un21 - q0 * vn1;
        while (Long.compareUnsigned(q0, b) >= 0 || Long.compareUnsigned(q0 * vn0, b * rhat + un0) > 0) {
            q0--;
            rhat += vn1;
            if (Long.compareUnsigned(rhat, b) >= 0) break;
        }
        return q1 * b + q0;
    }
`

func (p *printer) isDbl(v ir.V) bool           { return p.dbl[v] || p.dbl[p.pl.Res(v)] }
func (p *printer) readsDouble(s *ir.Stmt) bool { return p.pl.ReadsDouble(s, p.dbl) }

// nat says v is a natural, by its range or its constant.
func (p *printer) nat(v ir.V) bool {
	if d := p.pl.Const[p.pl.Res(v)]; d != nil && d.Kind == core.KInt {
		return d.Int >= 0
	}
	t := p.f.Types[p.pl.Res(v)]
	if lo, _, ok := core.IntRangeBig(t); ok && lo != nil {
		return lo.Sign() >= 0
	}
	return t == core.U64Type
}

// halves are v's two digits as long expressions.
func (p *printer) halves(v ir.V) (h, l string) {
	if p.isDbl(v) {
		n := p.name(v)
		return n + "h", n + "l"
	}
	if d := p.pl.Const[p.pl.Res(v)]; d != nil && d.Kind == core.KInt {
		return fmt.Sprintf("%dL", d.Int>>63), fmt.Sprintf("%dL", d.Int)
	}
	x := p.ref(v)
	if p.tyOf(v) != "long" {
		x = "((long) " + x + ")"
	}
	if p.f.Types[p.pl.Res(v)] == core.U64Type {
		return "0L", x // the unsigned word's value, read unsigned
	}
	return "(" + x + " >> 63)", x
}

// back is a long expression of a read-back's result, in its Java type.
func (p *printer) back(res ir.V, e string) string { return coerce(e, "long", p.tyOf(res)) }

// dop prints s, an operation of the double word.
func (p *printer) dop(s *ir.Stmt) {
	res := s.Res[0]
	n := p.name(res)
	inter := s.Mode == ir.MDouble
	needH := inter && p.highs[p.pl.Res(res)]
	h := make([]string, len(s.Args))
	l := make([]string, len(s.Args))
	for i, a := range s.Args {
		h[i], l[i] = p.halves(a)
	}
	put := func(hExpr, lExpr string) {
		if inter {
			p.line("final long %sl = %s;", n, lExpr)
			if needH {
				p.line("final long %sh = %s;", n, hExpr)
			}
			return
		}
		p.define(res, p.back(res, lExpr))
	}
	switch s.Op {
	case ir.OAdd:
		lE := fmt.Sprintf("(%s + %s)", l[0], l[1])
		if !needH {
			put("", lE)
			return
		}
		p.line("final long %sl = %s;", n, lE)
		if PlantDoubleCarryDropped {
			p.line("final long %sh = %s + %s;", n, h[0], h[1])
			return
		}
		p.line("final long %sh = %s + %s + (((%s & %s) | ((%s | %s) & ~%sl)) >>> 63);", n, h[0], h[1], l[0], l[1], l[0], l[1], n)
	case ir.OSub, ir.ONeg:
		a0, a1, b0, b1 := h[0], l[0], "", ""
		if s.Op == ir.ONeg {
			a0, a1, b0, b1 = "0L", "0L", h[0], l[0]
		} else {
			b0, b1 = h[1], l[1]
		}
		lE := fmt.Sprintf("(%s - %s)", a1, b1)
		if !needH {
			put("", lE)
			return
		}
		p.line("final long %sl = %s;", n, lE)
		p.line("final long %sh = %s - %s - (((~%s & %s) | (~(%s ^ %s) & %sl)) >>> 63);", n, a0, b0, a1, b1, a1, b1, n)
	case ir.OMul:
		lE := fmt.Sprintf("(%s * %s)", l[0], l[1])
		if !needH {
			put("", lE)
			return
		}
		p.line("final long %sl = %s;", n, lE)
		p.line("final long %sh = Math.multiplyHigh(%s, %s) + ((%s >> 63) & %s) + ((%s >> 63) & %s) + %s * %s + %s * %s;",
			n, l[0], l[1], l[0], l[1], l[1], l[0], l[0], h[1], h[0], l[1])
	case ir.OAnd:
		put("", fmt.Sprintf("(%s & %s)", l[0], l[1]))
	case ir.OShr:
		p.shr(s, n, h[0], l[0], needH)
	case ir.ODiv, ir.ORem, ir.OMod:
		p.divConst(s, n, h[0], l[0])
	case ir.OEq, ir.ONe, ir.OLt, ir.OLe, ir.OGt, ir.OGe:
		p.define(res, p.cmp(s.Op, p.nat(s.Args[0]) && p.nat(s.Args[1]), h, l))
	default:
		p.fail("(%s …) of the double word is not printed on the JVM", s.Op)
	}
}

// shr is x >> k, k in [0, 127]: logical for a natural, arithmetic otherwise,
// Java's count being taken mod 64.
func (p *printer) shr(s *ir.Stmt, n, xh, xl string, needH bool) {
	sh := ">>"
	if p.nat(s.Args[0]) {
		sh = ">>>"
	}
	fill := "0L"
	if sh == ">>" {
		fill = "(" + xh + " >> 63)"
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
			lo, hi = fmt.Sprintf("((%s >>> %d) | (%s << %d))", xl, c, xh, 64-c), fmt.Sprintf("(%s %s %d)", xh, sh, c)
		default:
			lo, hi = fmt.Sprintf("(%s %s %d)", xh, sh, c-64), fill
		}
		p.line("final long %s = %s;", resL, lo)
		if inter && needH {
			p.line("final long %s = %s;", resH, hi)
		}
	} else {
		kk := "((int) " + p.ref(k) + ")"
		p.line("final long %s;", resL)
		if inter && needH {
			p.line("final long %s;", resH)
		}
		p.line("if (%s == 0) {", kk)
		p.ind++
		p.line("%s = %s;", resL, xl)
		if inter && needH {
			p.line("%s = %s;", resH, xh)
		}
		p.ind--
		p.line("} else if (%s < 64) {", kk)
		p.ind++
		p.line("%s = (%s >>> %s) | (%s << (64 - %s));", resL, xl, kk, xh, kk)
		if inter && needH {
			p.line("%s = %s %s %s;", resH, xh, sh, kk)
		}
		p.ind--
		p.line("} else {")
		p.ind++
		p.line("%s = %s %s (%s - 64);", resL, xh, sh, kk)
		if inter && needH {
			p.line("%s = %s;", resH, fill)
		}
		p.ind--
		p.line("}")
	}
	if !inter {
		p.define(s.Res[0], p.back(s.Res[0], resL))
	}
}

// divConst is x / d, x % d or x mod d for a positive constant d, the quotient
// proven in a word.
func (p *printer) divConst(s *ir.Stmt, n, xh, xl string) {
	d := p.pl.Const[p.pl.Res(s.Args[1])]
	if d == nil || d.Kind != core.KInt {
		// BY A WORD: a natural dividend, the quotient proven in the word
		// (ir/double.go's byWord), so its high digit is below the divisor
		emit.JavaRecords["oroDivlu"] = divlu
		dv := p.ref(s.Args[1])
		if p.tyOf(s.Args[1]) != "long" {
			dv = "((long) " + dv + ")"
		}
		q := n + "q"
		p.line("final long %s = oroDivlu(%s, %s, %s);", q, xh, xl, dv)
		if s.Op == ir.ODiv {
			p.define(s.Res[0], p.back(s.Res[0], q))
		} else {
			p.define(s.Res[0], p.back(s.Res[0], fmt.Sprintf("(%s - %s * %s)", xl, q, dv)))
		}
		return
	}
	if d.Int <= 0 {
		p.fail("a quotient of the double word by %d", d.Int)
		return
	}
	D := fmt.Sprintf("%dL", d.Int)
	if k := log2(d.Int); k > 0 {
		mask := fmt.Sprintf("%dL", d.Int-1)
		if s.Op == ir.OMod {
			p.define(s.Res[0], p.back(s.Res[0], "("+xl+" & "+mask+")"))
			return
		}
		ql := n + "ql"
		if p.nat(s.Args[0]) {
			p.line("final long %s = (%s >>> %d) | (%s << %d);", ql, xl, k, xh, 64-k)
		} else {
			bias, bl := n+"bp", n+"bl"
			p.line("final long %s = (%s >> 63) & %s;", bias, xh, mask)
			p.line("final long %s = %s + %s;", bl, xl, bias)
			p.line("final long %s = (%s >>> %d) | ((%s + (((%s & %s) | ((%s | %s) & ~%s)) >>> 63)) << %d);",
				ql, bl, k, xh, xl, bias, xl, bias, bl, 64-k)
		}
		if s.Op == ir.ODiv {
			p.define(s.Res[0], p.back(s.Res[0], ql))
		} else {
			p.define(s.Res[0], p.back(s.Res[0], fmt.Sprintf("(%s - (%s << %d))", xl, ql, k)))
		}
		return
	}
	emit.JavaRecords["oroDivlu"] = divlu
	// a remainder's quotient need not be a word, and divlu's precondition is
	// u1 < v: Horner in base 2⁶⁴, x mod d = ((h mod d)·2⁶⁴ + l) mod d
	reduce := func(hi string) string {
		if s.Op == ir.ODiv || PlantRemOneDivision {
			return hi
		}
		return "Long.remainderUnsigned(" + hi + ", " + D + ")"
	}
	if p.nat(s.Args[0]) {
		q := n + "q"
		p.line("final long %s = oroDivlu(%s, %s, %s);", q, reduce(xh), xl, D)
		switch s.Op {
		case ir.ODiv:
			p.define(s.Res[0], p.back(s.Res[0], q))
		default:
			p.define(s.Res[0], p.back(s.Res[0], fmt.Sprintf("(%s - %s * %s)", xl, q, D)))
		}
		return
	}
	// the magnitude, divided, the sign put back
	sg, mh, ml, q := n+"s", n+"mh", n+"ml", n+"q"
	p.line("final boolean %s = %s < 0;", sg, xh)
	p.line("final long %s = %s ? -%s : %s;", ml, sg, xl, xl)
	p.line("final long %s = %s ? -%s - (%s != 0 ? 1L : 0L) : %s;", mh, sg, xh, xl, xh)
	p.line("final long %s = oroDivlu(%s, %s, %s);", q, reduce(mh), ml, D)
	switch s.Op {
	case ir.ODiv:
		p.define(s.Res[0], p.back(s.Res[0], fmt.Sprintf("(%s ? -%s : %s)", sg, q, q)))
	case ir.ORem:
		r := fmt.Sprintf("(%s - %s * %s)", ml, q, D)
		p.define(s.Res[0], p.back(s.Res[0], fmt.Sprintf("(%s ? -%s : %s)", sg, r, r)))
	case ir.OMod:
		r := n + "r"
		p.line("final long %s = %s - %s * %s;", r, ml, q, D)
		p.define(s.Res[0], p.back(s.Res[0], fmt.Sprintf("(%s && %s != 0 ? %s - %s : %s)", sg, r, D, r, r)))
	}
}

// cmp is a comparison of two elements of E, lexicographic on their digits.
func (p *printer) cmp(op ir.Op, unsigned bool, h, l []string) string {
	hiLess := func(a, b string) string {
		if unsigned {
			return "Long.compareUnsigned(" + a + ", " + b + ") < 0"
		}
		return a + " < " + b
	}
	less := func(a, b int) string {
		return fmt.Sprintf("(%s || %s == %s && Long.compareUnsigned(%s, %s) < 0)", hiLess(h[a], h[b]), h[a], h[b], l[a], l[b])
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
	return "(!" + less(0, 1) + ")"
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

// PlantDoubleCarryDropped leaves the majority function's carry out of the high
// digit of a sum: (−2⁶³ + −2⁶³)/2 is then not −2⁶³ (TestTheDoubleWordOnEveryHost).
var PlantDoubleCarryDropped bool

// PlantRemOneDivision divides a remainder's dividend in one divlu, whose
// precondition, the quotient a word's, a remainder does not give
// (TestTheDoubleWordOnEveryHost).
var PlantRemOneDivision bool
