package gauntlet

import "math/bits"

// KARATSUBA WITHOUT RECURSION.
//
// bigarith-2026-08-28 §8 claimed Karatsuba is unreachable under
// ADR 0014 because divide-and-conquer is the shape recursion was removed for,
// and hamza refused it: "I am sure we can write it using loops if we wanted."
//
// This file is the test. It is written to the constraints the language actually
// has, so that a working version refutes the claim rather than merely
// suggesting it:
//
//   - NO RECURSION. Three nested loops over a level index, a node index and a
//     limb index. Nothing calls itself.
//   - EVERY BUFFER SIZED BEFORE THE LOOP RUNS. `build` needs its length up
//     front, and the whole layout is a function of (n, D) — the recursion tree
//     of Karatsuba is BALANCED and its shape does not depend on the data, which
//     is exactly why the levels can be walked bottom-up instead of descended.
//   - NO STACK EITHER. The explicit-stack trick in examples/json/ answers a
//     traversal, where each node returns nothing. Divide-and-conquer needs each
//     node to hand a VALUE to its parent, which is the part that looked hard —
//     and the level-by-level form sidesteps it completely, the same way an
//     iterative FFT sidesteps the recursion in Cooley-Tukey.
//
// THE LAYOUT. At level L there are 3^L nodes, each with operands of
// s[L] = (n >> L) + pad limbs, split at h[L] = n >> (L+1). A node's three
// children are its low halves, its high halves, and the SUMS of the halves:
//
//	z0 = lo·lo'      z1 = hi·hi'      z2 = (lo+hi)(lo'+hi')
//	product = z1·B² + (z2 − z0 − z1)·B + z0        where B = 2^(64·h)
//
// Three half-size products instead of four, which is the whole trick, and
// (3/4)^D of schoolbook's work after D levels.
//
// The padding is what makes every node at a level the same size: `lo+hi` needs
// one limb more than a half, so a slot of (n>>L)+pad absorbs D of those.

// KWork is the whole workspace, allocated once from (n, D). Its existence is
// the point: a `build` can produce it, because nothing here depends on a value.
type KWork struct {
	n, D    int
	s, h    []int
	A, B, P [][]uint64
}

func pow3(k int) int {
	p := 1
	for i := 0; i < k; i++ {
		p *= 3
	}
	return p
}

func NewKWork(n, D int) *KWork {
	pad := D + 2
	w := &KWork{n: n, D: D, s: make([]int, D+1), h: make([]int, D+1)}
	w.A = make([][]uint64, D+1)
	w.B = make([][]uint64, D+1)
	w.P = make([][]uint64, D+1)
	for L, nodes := 0, 1; L <= D; L, nodes = L+1, nodes*3 {
		w.s[L] = (n >> L) + pad
		w.h[L] = n >> (L + 1)
		w.A[L] = make([]uint64, nodes*w.s[L])
		w.B[L] = make([]uint64, nodes*w.s[L])
		w.P[L] = make([]uint64, nodes*2*w.s[L])
	}
	return w
}

// addAt does dst[off:] += src, carrying to the end of dst.
func addAt(dst, src []uint64, off int) {
	var c uint64
	i := 0
	for ; i < len(src); i++ {
		dst[off+i], c = bits.Add64(dst[off+i], src[i], c)
	}
	for ; c != 0 && off+i < len(dst); i++ {
		dst[off+i], c = bits.Add64(dst[off+i], 0, c)
	}
}

// subAt does dst[off:] -= src, borrowing to the end of dst. The caller
// guarantees the running total stays non-negative, so the borrow always
// resolves inside dst.
func subAt(dst, src []uint64, off int) {
	var b uint64
	i := 0
	for ; i < len(src); i++ {
		dst[off+i], b = bits.Sub64(dst[off+i], src[i], b)
	}
	for ; b != 0 && off+i < len(dst); i++ {
		dst[off+i], b = bits.Sub64(dst[off+i], 0, b)
	}
}

func zero(x []uint64) {
	for i := range x {
		x[i] = 0
	}
}

// KaratsubaLimbs multiplies a by b, both n limbs, using D levels of Karatsuba
// over a schoolbook base case. Loops only.
func KaratsubaLimbs(a, b []uint64, w *KWork) []uint64 {
	D := w.D

	zero(w.A[0])
	zero(w.B[0])
	copy(w.A[0], a)
	copy(w.B[0], b)

	// DOWNWARD: build every level's operands from the one above. Three
	// children per node — low, high, and the sum of the two.
	for L := 0; L < D; L++ {
		s, h, sc := w.s[L], w.h[L], w.s[L+1]
		for k, nodes := 0, pow3(L); k < nodes; k++ {
			pa := w.A[L][k*s : (k+1)*s]
			pb := w.B[L][k*s : (k+1)*s]
			for c := 0; c < 3; c++ {
				ca := w.A[L+1][(3*k+c)*sc : (3*k+c+1)*sc]
				cb := w.B[L+1][(3*k+c)*sc : (3*k+c+1)*sc]
				zero(ca)
				zero(cb)
				switch c {
				case 0:
					copy(ca, pa[:h])
					copy(cb, pb[:h])
				case 1:
					copy(ca, pa[h:])
					copy(cb, pb[h:])
				default:
					copy(ca, pa[h:])
					addAt(ca, pa[:h], 0)
					copy(cb, pb[h:])
					addAt(cb, pb[:h], 0)
				}
			}
		}
	}

	// BASE CASE: schoolbook on the deepest level.
	{
		s := w.s[D]
		for k, nodes := 0, pow3(D); k < nodes; k++ {
			MulLimbs(w.A[D][k*s:(k+1)*s], w.B[D][k*s:(k+1)*s], w.P[D][k*2*s:(k+1)*2*s])
		}
	}

	// UPWARD: combine each node's three children.
	for L := D - 1; L >= 0; L-- {
		s, h, sc := w.s[L], w.h[L], w.s[L+1]
		for k, nodes := 0, pow3(L); k < nodes; k++ {
			p := w.P[L][k*2*s : (k+1)*2*s]
			zero(p)
			z0 := w.P[L+1][(3*k+0)*2*sc : (3*k+1)*2*sc]
			z1 := w.P[L+1][(3*k+1)*2*sc : (3*k+2)*2*sc]
			z2 := w.P[L+1][(3*k+2)*2*sc : (3*k+3)*2*sc]
			addAt(p, z0, 0)
			addAt(p, z1, 2*h)
			// The middle term, added before the two subtractions so the running
			// total never goes negative: z2 >= z0 + z1 by construction.
			addAt(p, z2, h)
			subAt(p, z0, h)
			subAt(p, z1, h)
		}
	}
	return w.P[0]
}
