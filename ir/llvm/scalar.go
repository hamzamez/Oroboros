package llvm

import (
	"fmt"
	"math/big"
	"strings"

	"oroboros/core"
	"oroboros/ir"
)

// THE 32-BIT SCALAR (ADR 0127, ADR 0083 item 2, llvm.md §3d).
//
// On a target whose preferred register width is 32 (`repr scalar 32`), a value
// whose proven range lies in S₃₂ = [−2³¹, 2³¹) is held in an i32. The
// representation is (H, d) with H the i32 and d the signed reading, and it is a
// function of the value's IR_P type, its proven range, read off its
// representative (a π shares its source's register, so its source's). The IR
// is not changed: the range is the IR's own record of the proof (ADR 0038,
// ADR 0083 W6), and the representation is ρ₃₂ of it on this target.
//
// THEOREM (an operation at 32 bits is exact). The residue map r: ℤ → ℤ/2³² is
// a ring homomorphism, injective on S₃₂. So + − · and negation on operands in
// S₃₂ whose result is proven in S₃₂ give, at 32 bits, the residue of the
// result, whose lift in S₃₂ is the result; their `nsw` holds since the result
// is in S₃₂, and `nuw` where the operands and the result are naturals. The
// bitwise operations of ℤ (ADR 0082) map S₃₂ × S₃₂ into S₃₂ and agree with
// i32's on two's complement. The signed order on S₃₂ is i32's. Truncating
// division and its remainder on S₃₂ are i32's `sdiv` and `srem` except at
// (−2³¹, −1), which LLVM leaves undefined; and a shift by k is i32's for
// k ∈ [0, 31], poison past it. So an operation is printed at 32 bits iff
//   (a) each integer operand and an integer result are held in i32;
//   (b) its spelling is width-generic: it names no literal type and no ${r};
//   (c) its 32-bit domain holds by the operands' ranges: a signed quotient or
//       remainder whose dividend may be −2³¹ has a divisor that is not −1, and a
//       shift's count lies in [0, 31].
// Every other statement is printed as before, at the word, in a WIDE SCOPE: an
// i32 operand is first sign-extended, and an integer result proven in S₃₂ is
// then truncated, each the identity on ℤ (d and its inverse on S₃₂). ∎
//
// THE BOUNDARIES stay the word's: a function's parameters and results are i64,
// as a program function's callers and the C ABI's maps expect, a parameter in
// S₃₂ narrowed once at entry and a result widened at `ret`.

var (
	minS32 = big.NewInt(-1 << 31)
	maxS32 = big.NewInt(1<<31 - 1)
)

// s32Type says an IR_P type is held in an i32 on this target.
func (p *printer) s32Type(t string) bool {
	if p.tg.ScalarWidth != 32 || PlantNoScalar || p.tg.ValueType(t) == core.U64Type {
		return false
	}
	lo, hi, ok := core.IntRangeBig(t)
	if PlantScalarWide && ok && lo != nil && hi != nil && lo.Sign() >= 0 {
		// THE PLANTED FAULT: naturals up to 2³² taken for S₃₂, so a value in
		// [2³¹, 2³²) is held in an i32 and read back negative
		return hi.Cmp(new(big.Int).Lsh(big.NewInt(1), 32)) < 0
	}
	return ok && lo != nil && hi != nil && lo.Cmp(minS32) >= 0 && hi.Cmp(maxS32) <= 0
}

// PlantNoScalar holds every integer in the word, the printer before ADR 0127.
var PlantNoScalar bool

// PlantScalarWide takes naturals below 2³² for S₃₂: TestAScalarIsHeldWhereItsRangeIs
// must see an answer change.
var PlantScalarWide bool

// PlantScalarNoDomain prints a signed quotient, a remainder and a shift at 32
// bits whatever their 32-bit domain: TestAScalarIsHeldWhereItsRangeIs must see
// the condition the rule states reached.
var PlantScalarNoDomain bool

// s32 says v is held in an i32: its representative's range is S₃₂'s, it is no
// double word, and no wide scope holds it at the word.
func (p *printer) s32(v ir.V) bool {
	if p.tg.ScalarWidth != 32 {
		return false
	}
	r := p.pl.Res(v)
	if p.wideNow[r] || p.dbl[r] || p.dbl[v] {
		return false
	}
	return p.s32Type(p.f.Types[r])
}

// as is v's spelling as an operand of LLVM type want: itself, or sign-extended
// from i32 to i64, or truncated from i64 to i32, where v's range makes the
// truncation exact. A literal is the same spelling at either width.
func (p *printer) as(v ir.V, want string) string { return p.asTo(v, want, -1) }

// asTo is as into a join's value dst: what flows into a loop variable or a
// branch's result lies in dst's proven range on that edge, so dst's range
// makes the truncation exact where v's own type is wider (IR_P types a
// constant `int` in places).
func (p *printer) asTo(v ir.V, want string, dst ir.V) string {
	have, x := p.ty(v), p.val(v)
	if have == want || !strings.HasPrefix(x, "%") {
		return x
	}
	t := "%" + p.fresh("x")
	switch {
	case have == "i32" && want == "i64":
		p.line("%s = sext i32 %s to i64", t, x)
	case (have == "i64" || have == "i32") && want == "i128":
		// A WORD INTO THE DOUBLE WORD'S JOIN (ADR 0153): its residue is its
		// extension, by its sign, a natural's zero and sign extensions agreeing
		ext := "sext"
		if p.lo(v) >= 0 {
			ext = "zext"
		}
		p.line("%s = %s %s %s to i128", t, ext, have, x)
	case have == "i64" && want == "i32":
		// EXACT ONLY ON S₃₂: v's own range must lie there, as a π's may where
		// its source's does not
		if !p.inS32(v) && (dst < 0 || !p.inS32(dst)) {
			p.fail("%%v%d of range %s is truncated to 32 bits, which is not exact there (ADR 0127)", v, p.f.Types[v])
			return x
		}
		p.line("%s = trunc i64 %s to i32", t, x)
	default:
		p.fail("a value of type %s where %s is held (ADR 0127)", have, want)
		return x
	}
	return t
}

// inS32 says v's own proven range lies in S₃₂.
func (p *printer) inS32(v ir.V) bool {
	lo, hi := p.ends(v)
	return lo != nil && hi != nil && lo.Cmp(minS32) >= 0 && hi.Cmp(maxS32) <= 0
}

// wide prints s at the word, as the printer did before ADR 0127, and reports
// what print reports: each i32 operand sign-extended first, and each integer
// result proven in S₃₂ truncated after, `%vN.n`, which is then its spelling.
// print answers false when it printed nothing, and then nothing is narrowed.
func (p *printer) wide(s *ir.Stmt, print func() bool) bool {
	if p.tg.ScalarWidth != 32 {
		return print()
	}
	type saved struct {
		name string
		had  bool
	}
	restore := map[ir.V]saved{}
	for _, a := range s.Args {
		r := p.pl.Res(a)
		if _, done := restore[r]; done || !p.s32(r) {
			continue
		}
		x := p.val(r)
		n, had := p.names[r]
		restore[r] = saved{n, had}
		if strings.HasPrefix(x, "%") {
			w := "%" + p.fresh("w")
			p.line("%s = sext i32 %s to i64", w, x)
			x = w
		}
		p.names[r] = x
		p.wideNow[r] = true
	}
	var res []ir.V
	for _, v := range s.Res {
		if p.s32(v) {
			res = append(res, v)
			p.wideNow[v] = true
		}
	}
	ok := print()
	for r, sv := range restore {
		delete(p.wideNow, r)
		if sv.had {
			p.names[r] = sv.name
		} else {
			delete(p.names, r)
		}
	}
	for _, v := range res {
		delete(p.wideNow, v)
		if !ok {
			continue
		}
		x := p.val(v)
		if !strings.HasPrefix(x, "%") {
			continue
		}
		n := fmt.Sprintf("%%v%d.n", v)
		p.line("%s = trunc i64 %s to i32", n, x)
		p.names[v] = n
	}
	return ok
}

// narrow says the operation s is printed at 32 bits by (a)–(c) above.
func (p *printer) narrow(s *ir.Stmt) bool {
	if p.tg.ScalarWidth != 32 || s.Mode == ir.MWrap || s.Mode == ir.MDouble || s.Mode == ir.MTrap {
		return false
	}
	for _, a := range s.Args {
		if t := p.ty(a); t != "i1" && t != "i32" {
			return false
		}
	}
	for _, v := range s.Res {
		if t := p.ty(v); t != "i1" && t != "i32" {
			return false
		}
	}
	form := p.pl.Form(s)
	f := form[0]
	if f == "" || strings.Contains(f, "i64") || holes(f)['r'] {
		return false
	}
	if PlantScalarNoDomain {
		return true
	}
	word, _, _ := strings.Cut(f, " ")
	switch word {
	case "sdiv", "srem":
		// (−2³¹, −1) is undefined at 32 bits: excluded by the dividend's range
		// or the divisor's
		lo, _ := p.ends(s.Args[0])
		dl, dh := p.ends(s.Args[1])
		if lo == nil || lo.Cmp(minS32) == 0 {
			if dl == nil || dh == nil || dl.Cmp(big.NewInt(-1)) <= 0 && dh.Cmp(big.NewInt(-1)) >= 0 {
				return false
			}
		}
	case "shl", "ashr", "lshr":
		// a count past 31 is poison at 32 bits
		if len(s.Args) < 2 {
			return false
		}
		lo, hi := p.ends(s.Args[1])
		if lo == nil || hi == nil || lo.Sign() < 0 || hi.Cmp(big.NewInt(31)) > 0 {
			return false
		}
	}
	return true
}
