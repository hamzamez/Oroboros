package ir

import "math/big"

// THE INTERVALS OF THE BOOLEAN OPERATIONS, THE SHIFTS AND MOD (ADR 0082,
// integers.md §0b). Each is a sound transfer function for ℤ's operation read
// 2-adically, checked exhaustively and at random against math/big by
// TestTheBitOperationsAreSound. The one law under the and, or and xor of
// signed operands:
//
//	a, b ∈ [−2ᵏ, 2ᵏ)  ⇒  a & b, a | b, a ⊕ b ∈ [−2ᵏ, 2ᵏ)
//
// since every position at or above k holds a's sign in a and b's in b, so it
// holds one bit in the result. That is why the three need no obligation: their
// operands are in the word, so they are.

// bitWindow is the least k with x ⊆ [−2ᵏ, 2ᵏ), for a finite x.
func bitWindow(x iv) (uint, bool) {
	if !x.finite() {
		return 0, false
	}
	k := 0
	if hi := x.hi.big(); hi.Sign() > 0 {
		k = hi.BitLen()
	}
	if lo := x.lo.big(); lo.Sign() < 0 {
		m := new(big.Int).Neg(lo)
		m.Sub(m, big.NewInt(1))
		if b := m.BitLen(); b > k {
			k = b
		}
	}
	return uint(k), true
}

// windowIV is [−2ᵏ, 2ᵏ − 1].
func windowIV(k uint) iv {
	p := new(big.Int).Lsh(big.NewInt(1), k)
	hi, okh := fromBig(new(big.Int).Sub(p, big.NewInt(1)))
	lo, okl := fromBig(new(big.Int).Neg(p))
	if !okh || !okl {
		return ivTop
	}
	return rangeE(lo, hi)
}

// below2 is [0, 2^bitlen(x.hi) − 1] for a non-negative x: every result of an
// or or xor of non-negative values no longer than x's longest.
func below2(hi ep) (ep, bool) {
	p := new(big.Int).Lsh(big.NewInt(1), uint(hi.big().BitLen()))
	return fromBig(p.Sub(p, big.NewInt(1)))
}

func ivNonNeg(x iv) bool { return !x.nlo && x.lo.sign() >= 0 }
func allNeg(x iv) bool   { return !x.phi && x.hi.sign() < 0 }

// andZ is the and on ℤ: andIV's mask law for a non-negative operand, below
// both negative operands, and the window otherwise.
func andZ(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if ivNonNeg(a) || ivNonNeg(b) {
		return andIV(a, b)
	}
	k, ok1 := bitWindow(a)
	k2, ok2 := bitWindow(b)
	if !ok1 || !ok2 {
		return ivTop
	}
	if k2 > k {
		k = k2
	}
	out := windowIV(k)
	if allNeg(a) && allNeg(b) {
		// both negative: the result's bits are a subset of each, and it is
		// negative, so it lies below both
		out.hi = minE(out.hi, minE(a.hi, b.hi))
	}
	return out.norm()
}

// orZ is the or: at least the larger of two non-negative operands, and no
// longer than the longer, nor above their sum; negative when either is.
func orZ(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if ivNonNeg(a) && ivNonNeg(b) {
		out := iv{lo: maxE(a.lo, b.lo)}
		if a.phi || b.phi {
			out.phi = true
			return out.norm()
		}
		hi, ok := below2(maxE(a.hi, b.hi))
		if !ok {
			out.phi = true
			return out.norm()
		}
		if s, ok := a.hi.add(b.hi); ok {
			hi = minE(hi, s)
		}
		out.hi = hi
		return out.norm()
	}
	k, ok1 := bitWindow(a)
	k2, ok2 := bitWindow(b)
	if !ok1 || !ok2 {
		return ivTop
	}
	if k2 > k {
		k = k2
	}
	out := windowIV(k)
	if allNeg(a) || allNeg(b) {
		out.hi = ei(-1)
		if allNeg(a) {
			out.lo = maxE(out.lo, a.lo)
		}
		if allNeg(b) {
			out.lo = maxE(out.lo, b.lo)
		}
	}
	return out.norm()
}

// xorZ is the xor: below the longer of two non-negative operands' powers.
func xorZ(a, b iv) iv {
	if a.bot || b.bot {
		return ivBot
	}
	if ivNonNeg(a) && ivNonNeg(b) {
		if a.phi || b.phi {
			return iv{phi: true}
		}
		hi, ok := below2(maxE(a.hi, b.hi))
		if !ok {
			return iv{phi: true}
		}
		return rangeE(ep{}, hi)
	}
	k, ok1 := bitWindow(a)
	k2, ok2 := bitWindow(b)
	if !ok1 || !ok2 {
		return ivTop
	}
	if k2 > k {
		k = k2
	}
	out := windowIV(k)
	if allNeg(a) && allNeg(b) || ivNonNeg(a) && allNeg(b) || allNeg(a) && ivNonNeg(b) {
		// equal signs give a non-negative result, different ones a negative
		if allNeg(a) == allNeg(b) {
			out.lo = ep{}
		} else {
			out.hi = ei(-1)
		}
	}
	return out.norm()
}

// notZ is −x − 1, which reverses the order.
func notZ(a iv) iv {
	if a.bot {
		return ivBot
	}
	return subIV(negIV(a), rangeIV(1, 1))
}

// shlZ is a·2ᵏ for k ∈ [k0, k1], k ≥ 0 by the operation's `where`: the
// product of a and the powers' interval.
func shlZ(a, k iv) iv {
	if a.bot || k.bot {
		return ivBot
	}
	if a.finite() && a.lo.sign() == 0 && a.hi.sign() == 0 {
		return rangeIV(0, 0)
	}
	k0, ok0 := k.lo.i64()
	k1, ok1 := k.hi.i64()
	if !k.finite() || !ok0 || !ok1 || k1 > 124 || k1 < 0 {
		return ivTop // a count past 124 leaves the domain; one below 0, the `where`
	}
	if k0 < 0 {
		k0 = 0
	}
	lo, _ := fromBig(new(big.Int).Lsh(big.NewInt(1), uint(k0)))
	hi, _ := fromBig(new(big.Int).Lsh(big.NewInt(1), uint(k1)))
	return mulIV(a, rangeE(lo, hi))
}

// shrZ is ⌊a/2ᵏ⌋ for k ∈ [k0, k1], k ≥ 0: monotone in a; in k, falling to 0 on
// a non-negative a and rising to −1 on a negative one. So each end is the
// floor at one of the counts' ends; a count past 200 is a's sign on every end
// the domain holds (|a| < 2¹²⁶).
func shrZ(a, k iv) iv {
	if a.bot || k.bot {
		return ivBot
	}
	k0 := int64(0)
	if !k.nlo {
		if v, ok := k.lo.i64(); ok && v > 0 {
			k0 = v
		} else if !ok && k.lo.sign() > 0 {
			k0 = 200
		}
	}
	k1 := int64(200)
	if !k.phi {
		if v, ok := k.hi.i64(); ok && v < 200 {
			k1 = v
		}
	}
	if k1 < 0 {
		k1 = 0
	}
	if k0 > 200 {
		k0 = 200
	}
	at := func(x ep, k int64) ep {
		e, _ := fromBig(new(big.Int).Rsh(x.big(), uint(k))) // Rsh floors a negative
		return e
	}
	out := iv{nlo: a.nlo, phi: a.phi}
	if !a.nlo {
		out.lo = minE(at(a.lo, k0), at(a.lo, k1))
	}
	if !a.phi {
		out.hi = maxE(at(a.hi, k0), at(a.hi, k1))
	}
	return out.norm()
}

// modZ is the Euclidean remainder by m, m > 0 by the operation's `where`:
// in [0, m − 1], and a itself where 0 ≤ a < m.
func modZ(a, m iv) iv {
	if a.bot || m.bot {
		return ivBot
	}
	if m.nlo || m.lo.sign() <= 0 {
		// the `where` holds m > 0 wherever this runs; claim only its range
		m.lo, m.nlo = ei(1), false
	}
	if ivNonNeg(a) && !a.phi && a.hi.lt(m.lo) {
		return a
	}
	if m.phi {
		if ivNonNeg(a) {
			return iv{lo: ep{}, hi: a.hi, phi: a.phi}.norm()
		}
		return iv{lo: ep{}, phi: true}
	}
	hi, _ := m.hi.sub(ei(1))
	if ivNonNeg(a) && !a.phi {
		hi = minE(hi, a.hi)
	}
	return rangeE(ep{}, hi)
}

// wrapIV is S_W = [−2^(W−1), 2^(W−1)), where a fused operation's signed
// residue lies (ADR 0082 item 3).
func wrapIV(w int64) iv {
	if w <= 0 || w > 64 {
		return ivTop
	}
	return windowIV(uint(w - 1))
}
