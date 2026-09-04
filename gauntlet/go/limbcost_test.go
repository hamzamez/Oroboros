package gauntlet

import (
	"math/big"
	"testing"
)

// WHAT THE LIMB RUNG'S 5.85x IS MADE OF.
//
// bigrepr-2026-09-03 measured our emitted fixed-limb factorial at 13,730 ns
// against `math/big`'s 2,347, and §6 named the fix as "a target-declared limb
// width and carry extraction" — citing bigarith-2026-08-28's 2.75x for the limb
// count and 3.9x for a bitwise carry.
//
// THOSE ARE NUMBERS ABOUT HAND-WRITTEN HOST CODE and nobody has decomposed OURS.
// This does that, by writing the same factorial by hand once per suspected cost
// and removing one at a time. Each variant is the previous one minus a single
// thing, so a difference is attributable.
//
// The reference implementation is `GenFactLimbs` in gen_biglimbsel.go, and
// `limbAsEmitted` below is a line-by-line transcription of it — which is the
// only honest baseline: comparing against a variant that is faster for two
// reasons at once is the measurement error this repository has caught in itself
// four times.
//
//	go test -bench='LimbCost' -benchtime=20000x -count=5

const limbW = 55 // 1301 bits over 24, the width `(int 0 (pow 2 1300))` selects

// ─── A. exactly what we emit: clamped reads, a masked read, `%` and `/`
//
// The clamp is `at` in emit/bignum.oro; the mask on the read is there too, and
// its comment already names it as the honest limit rather than a shortcut — a
// limb is under 2^24 by construction, but that is an inductive invariant over a
// buffer's own contents, which the analysis correctly refuses to assume.
func limbAsEmitted(n int) []int32 {
	b := make([]int32, limbW)
	for i, r := 0, 1; i < limbW; i, r = i+1, r/16777216 {
		b[i] = int32(r % 16777216)
	}
	acc := b
	sp := make([]int32, limbW)
	for i := 2; i <= n; i++ {
		clear(sp)
		o := sp
		c := 0
		for j := 0; j < limbW; j++ {
			var v int
			if j < 0 {
				v = 0
			} else if j >= len(acc) {
				v = 0
			} else {
				v = int(acc[j])
			}
			t := (v%16777216)*i + c
			o[j] = int32(t % 16777216)
			c = t / 16777216
		}
		if c != 0 {
			panic("bignum overflow")
		}
		acc, sp = o, acc
	}
	return acc
}

// ─── B. minus the MASK on the read
func limbNoMask(n int) []int32 {
	b := make([]int32, limbW)
	for i, r := 0, 1; i < limbW; i, r = i+1, r/16777216 {
		b[i] = int32(r % 16777216)
	}
	acc := b
	sp := make([]int32, limbW)
	for i := 2; i <= n; i++ {
		clear(sp)
		o := sp
		c := 0
		for j := 0; j < limbW; j++ {
			var v int
			if j < 0 {
				v = 0
			} else if j >= len(acc) {
				v = 0
			} else {
				v = int(acc[j])
			}
			t := v*i + c
			o[j] = int32(t % 16777216)
			c = t / 16777216
		}
		if c != 0 {
			panic("bignum overflow")
		}
		acc, sp = o, acc
	}
	return acc
}

// ─── C. minus the CLAMP as well
func limbNoClamp(n int) []int32 {
	b := make([]int32, limbW)
	for i, r := 0, 1; i < limbW; i, r = i+1, r/16777216 {
		b[i] = int32(r % 16777216)
	}
	acc := b
	sp := make([]int32, limbW)
	for i := 2; i <= n; i++ {
		clear(sp)
		o := sp
		c := 0
		for j := 0; j < limbW; j++ {
			t := int(acc[j])*i + c
			o[j] = int32(t % 16777216)
			c = t / 16777216
		}
		if c != 0 {
			panic("bignum overflow")
		}
		acc, sp = o, acc
	}
	return acc
}

// ─── D. minus the CLEAR
//
// `mul-small` writes EVERY slot of the output before anything reads one, so the
// zero fill a reused buffer needs to honour `build`'s guarantee (tables.md
// §14.3) is dead here. Whether the compiler can know that is a separate
// question; what it costs is this.
func limbNoClear(n int) []int32 {
	b := make([]int32, limbW)
	for i, r := 0, 1; i < limbW; i, r = i+1, r/16777216 {
		b[i] = int32(r % 16777216)
	}
	acc := b
	sp := make([]int32, limbW)
	for i := 2; i <= n; i++ {
		o := sp
		c := 0
		for j := 0; j < limbW; j++ {
			t := int(acc[j])*i + c
			o[j] = int32(t % 16777216)
			c = t / 16777216
		}
		if c != 0 {
			panic("bignum overflow")
		}
		acc, sp = o, acc
	}
	return acc
}

// ─── E. minus `%` and `/`, using a shift and a mask
//
// This is the operation bigarith-2026-08-28 measured at 3.9x on V8 and the one
// integers.md §0a keeps out of the language, because V8 coerces both operands of
// `&` to int32 — an observable disagreement INSIDE the portable window. On Go
// the two forms should be close, because 16777216 is a constant power of two and
// the compiler strength-reduces both; the point of measuring is that "should be"
// is not a result.
func limbShift(n int) []int32 {
	b := make([]int32, limbW)
	for i, r := 0, 1; i < limbW; i, r = i+1, r>>24 {
		b[i] = int32(r & 0xFFFFFF)
	}
	acc := b
	sp := make([]int32, limbW)
	for i := 2; i <= n; i++ {
		o := sp
		c := 0
		for j := 0; j < limbW; j++ {
			t := int(acc[j])*i + c
			o[j] = int32(t & 0xFFFFFF)
			c = t >> 24
		}
		if c != 0 {
			panic("bignum overflow")
		}
		acc, sp = o, acc
	}
	return acc
}

// ─── F. 32-BIT LIMBS, which is what a declared width would buy
//
// 1301 bits over 32 is 41 limbs against 55, so this is 1.34x less work and not
// bigarith's 2.75x — that figure is 24-bit against 64-bit, and 64-bit limbs are
// NOT EXPRESSIBLE in this language: ADR 0012 makes an `int` exact to ±(2^53−1),
// so a limb of 2^60 is not a value the language can hold. 32 is the widest limb
// whose PRODUCT still needs a wide multiply, and 26 is the widest that does not.
func limbW32(n int) []uint32 {
	const w = 41
	b := make([]uint32, w)
	b[0] = 1
	acc := b
	sp := make([]uint32, w)
	for i := 2; i <= n; i++ {
		o := sp
		c := uint64(0)
		for j := 0; j < w; j++ {
			t := uint64(acc[j])*uint64(i) + c
			o[j] = uint32(t)
			c = t >> 32
		}
		if c != 0 {
			panic("bignum overflow")
		}
		acc, sp = o, acc
	}
	return acc
}

// ─── the oracle
//
// Every variant is checked against `math/big`'s own `MulRange`, not against each
// other: five implementations of ours agreeing proves only that they are the
// same program.

func limbCostToBig(l []int32) *big.Int {
	out, base := new(big.Int), big.NewInt(1<<24)
	for i := len(l) - 1; i >= 0; i-- {
		out.Mul(out, base)
		out.Add(out, big.NewInt(int64(l[i])))
	}
	return out
}

// The emitted form's element type is chosen by the compiler, so this reads it
// at whatever width the narrowing picked. Base 2^24 either way — the ELEMENT is
// a representation and the base is the algorithm.
func limbs32ToBig24(l []uint32) *big.Int {
	out, base := new(big.Int), big.NewInt(1<<24)
	for i := len(l) - 1; i >= 0; i-- {
		out.Mul(out, base)
		out.Add(out, new(big.Int).SetUint64(uint64(l[i])))
	}
	return out
}

func limbs32ToBig(l []uint32) *big.Int {
	out, base := new(big.Int), big.NewInt(1<<32)
	for i := len(l) - 1; i >= 0; i-- {
		out.Mul(out, base)
		out.Add(out, new(big.Int).SetUint64(uint64(l[i])))
	}
	return out
}

func TestLimbCostVariantsAreAllExact(t *testing.T) {
	for _, n := range []int{0, 1, 2, 20, 50, 100, 200} {
		want := new(big.Int).MulRange(1, int64(n))
		if n == 0 {
			want = big.NewInt(1)
		}
		for _, c := range []struct {
			name string
			f    func(int) []int32
		}{
			{"as-emitted", limbAsEmitted},
			{"no-mask", limbNoMask},
			{"no-clamp", limbNoClamp},
			{"no-clear", limbNoClear},
			{"shift", limbShift},
		} {
			if got := limbCostToBig(c.f(n)); got.Cmp(want) != 0 {
				t.Errorf("%s: fact(%d) = %s, want %s", c.name, n, got, want)
			}
		}
		if got := limbs32ToBig(limbW32(n)); got.Cmp(want) != 0 {
			t.Errorf("w32: fact(%d) = %s, want %s", n, got, want)
		}
	}
	// AND THE EMITTED FORM IS WHAT THIS BASELINE CLAIMS TO BE. A transcription
	// that had drifted would make every number below attribute a cost to the
	// wrong thing.
	if limbCostToBig(limbAsEmitted(200)).Cmp(limbs32ToBig24(GenFactLimbs(200))) != 0 {
		t.Error("limbAsEmitted disagrees with GenFactLimbs; the baseline is not the emitted form")
	}
}

var limbCostSink any

func BenchmarkLimbCostEmitted(b *testing.B)  { benchLimb(b, func() { limbCostSink = GenFactLimbs(200) }) }
func BenchmarkLimbCostAsEmitted(b *testing.B) {
	benchLimb(b, func() { limbCostSink = limbAsEmitted(200) })
}
func BenchmarkLimbCostNoMask(b *testing.B)  { benchLimb(b, func() { limbCostSink = limbNoMask(200) }) }
func BenchmarkLimbCostNoClamp(b *testing.B) { benchLimb(b, func() { limbCostSink = limbNoClamp(200) }) }
func BenchmarkLimbCostNoClear(b *testing.B) { benchLimb(b, func() { limbCostSink = limbNoClear(200) }) }
func BenchmarkLimbCostShift(b *testing.B)   { benchLimb(b, func() { limbCostSink = limbShift(200) }) }
func BenchmarkLimbCostW32(b *testing.B)     { benchLimb(b, func() { limbCostSink = limbW32(200) }) }

func benchLimb(b *testing.B, f func()) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		f()
	}
}
