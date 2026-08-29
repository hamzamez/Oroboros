package gauntlet

import (
	"math/big"
	"math/bits"
)

// R3: THE HOST'S BIGNUM AGAINST ONE WE COULD EMIT.
//
// `bignum.go` prices DETECTION — what a checked multiply costs when the value
// stays inside a word. This file prices the other half: what arithmetic costs
// once the value genuinely does not fit, which is the question
// precision-by-declaration.md leaves open and the one that decides whether R3
// is a target capability we declare or a library we write.
//
// THE TWO WORKLOADS ARE THE TWO PROGRAMS `examples/int/` REFUSES.
// `power.oro`'s accumulator overflows by multiplication; `fib.oro`'s by
// addition. They are also the two operation shapes: a big value times a small
// one is linear in the limb count, and big plus big is linear with a carry
// chain, while big times big is quadratic. Measuring only one would answer
// half the question.
//
// WHAT "ONE WE COULD EMIT" MEANS, precisely. The limb form here is written to
// the constraints Oroboros actually has, so that a good number is a number we
// could reach:
//
//   - limbs live in a table of a length known BEFORE the loop, because `build`
//     needs its length up front (ADR 0018). That is not a restriction here:
//     a product has at most `len(a)+len(b)` limbs and a sum at most
//     `max(len a, len b)+1`, so **every bignum result's size is computable from
//     its operands' sizes**. A bignum needs no growable storage, which is worth
//     stating because it was not obvious — see growth.md.
//   - the accumulator is ONE buffer threaded through the loop and written in
//     place, which is exactly what ADR 0018's linear buffer is and what
//     `(again (set b i v) …)` compiles to today.
//   - `bits.Mul64` and `bits.Add64` stand for target-declared primitives. Go
//     and x86-64 have them; the JVM has `Math.multiplyHigh` since 9;
//     **JavaScript has no 64×64→128 multiply at all**, which is why the JS
//     answer is expected to differ and is measured separately.

// ---------------------------------------------------------------- factorial

// FactBigNaive is math/big with a fresh result per step — how a `bignum` type
// lowers if nothing is done about allocation.
func FactBigNaive(n int) *big.Int {
	acc := big.NewInt(1)
	for k := int64(2); k <= int64(n); k++ {
		acc = new(big.Int).Mul(acc, big.NewInt(k))
	}
	return acc
}

// FactBigReuse is math/big used as well as it can be: one receiver, one scratch
// multiplicand, nothing fresh in the loop.
func FactBigReuse(n int) *big.Int {
	acc, k := big.NewInt(1), new(big.Int)
	for i := int64(2); i <= int64(n); i++ {
		k.SetInt64(i)
		acc.Mul(acc, k)
	}
	return acc
}

// FactLimbs is the shape we could emit: one buffer, sized up front, multiplied
// in place by a small value. No allocation in the loop at all.
//
// `acc` is a little-endian magnitude in base 2^64. The returned slice is the
// significant prefix.
func FactLimbs(n int, acc []uint64) []uint64 {
	for i := range acc {
		acc[i] = 0
	}
	acc[0] = 1
	used := 1
	for k := uint64(2); k <= uint64(n); k++ {
		var carry uint64
		for i := 0; i < used; i++ {
			hi, lo := bits.Mul64(acc[i], k)
			var c uint64
			acc[i], c = bits.Add64(lo, carry, 0)
			// hi <= 2^64-2 for any full product, so hi+c cannot overflow.
			carry = hi + c
		}
		if carry != 0 {
			acc[used] = carry
			used++
		}
	}
	return acc[:used]
}

// FactLimbCount is how many limbs `n!` needs, computed the way the compiler
// would have to: from the operand sizes, before the loop runs.
func FactLimbCount(n int) int {
	// log2(n!) = sum log2(k) <= n*log2(n); one limb per 64 bits, plus slack.
	bitsNeeded := 1
	for k := 2; k <= n; k++ {
		bitsNeeded += bits.Len(uint(k))
	}
	return bitsNeeded/64 + 2
}

// ---------------------------------------------------------------- fibonacci

func FibBigReuse(n int) *big.Int {
	a, b, t := big.NewInt(0), big.NewInt(1), new(big.Int)
	for i := 0; i < n; i++ {
		t.Add(a, b)
		a, b, t = b, t, a
	}
	return a
}

// FibLimbs is big+big: a ripple carry, with three buffers rotating — which is
// what three loop variables threaded through `again` would be.
//
// IT TRACKS THE SIGNIFICANT LENGTH, and that is not a micro-optimisation. The
// first version added over the whole fixed buffer every iteration and measured
// 1.21x SLOWER than math/big; the buffer is sized for fib(n) and the first
// iterations need one limb, so it was doing twelve times the necessary work at
// the start. math/big tracks its length and does not.
//
// The lesson generalises past this file: `build` gives a buffer a length up
// front, so a bignum emitted from Oroboros gets the CAPACITY for free and must
// carry the significant count as an ordinary loop variable. That is expressible
// today — it is one more `again` argument — but it has to be written, and
// leaving it out is a silent 1.2x.
func FibLimbs(n int, a, b, t []uint64) []uint64 {
	for i := range a {
		a[i], b[i], t[i] = 0, 0, 0
	}
	b[0] = 1
	ua, ub := 1, 1
	for i := 0; i < n; i++ {
		u := ub
		if ua > u {
			u = ua
		}
		var carry uint64
		for j := 0; j < u; j++ {
			t[j], carry = bits.Add64(a[j], b[j], carry)
		}
		ut := u
		if carry != 0 {
			t[u] = carry
			ut = u + 1
		}
		a, b, t = b, t, a
		ua, ub = ub, ut
	}
	return a[:ua]
}

// FibLimbCount is the size known before the loop: fib(n) < 2^n, and the golden
// ratio gives a much tighter bound, but the loose one is what a compiler could
// derive syntactically.
func FibLimbCount(n int) int { return n*695/1000/64 + 2 }

// ------------------------------------------------------------- big x big
//
// The case §4 named and all three earlier workloads avoid: both operands
// large. Factorial and fibonacci multiply big by SMALL and add big to big,
// which are linear for everyone. A big x big product is QUADRATIC in the limb
// count for schoolbook and O(n^1.585) for Karatsuba, so this is where a host
// library's asymptotics and hand-written assembly should start to matter.
//
// math/big switches to Karatsuba at 40 words and Toom-Cook above that; ours is
// schoolbook and always will be, because Karatsuba needs recursion and
// temporary allocation and ADR 0014 has no recursion.

// MulLimbs is schoolbook: for each limb of a, multiply through b and add into
// the running product. `out` must have len(a)+len(b) limbs.
func MulLimbs(a, b, out []uint64) []uint64 {
	for i := range out {
		out[i] = 0
	}
	for i, ai := range a {
		var carry uint64
		for j, bj := range b {
			hi, lo := bits.Mul64(ai, bj)
			var c uint64
			lo, c = bits.Add64(lo, carry, 0)
			hi += c // hi <= 2^64-2, so this cannot overflow
			out[i+j], c = bits.Add64(out[i+j], lo, 0)
			carry = hi + c
		}
		out[i+len(b)] += carry
	}
	return out
}

// MulBigReuse is math/big with the receiver preallocated — the careful form,
// which is the one that gave math/big its best showing everywhere else.
func MulWideBig(a, b, r *big.Int) *big.Int { return r.Mul(a, b) }

// LimbsOf makes a deterministic n-limb value with every limb full-width, so
// the product genuinely needs 2n limbs and nothing degenerates.
func LimbsOf(n int, seed uint64) []uint64 {
	out := make([]uint64, n)
	x := seed | 1<<63
	for i := range out {
		x = x*6364136223846793005 + 1442695040888963407
		out[i] = x | 1<<63
	}
	return out
}
