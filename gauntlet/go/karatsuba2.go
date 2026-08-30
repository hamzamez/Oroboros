package gauntlet

// KARATSUBA, LEVEL BY LEVEL, WITHOUT COPYING OPERANDS.
//
// karatsuba.go proved the shape is reachable without recursion and measured
// 2.45x over schoolbook at 1024 limbs — 40% short of the 4.2x the level count
// implies. The gap is copying: that version writes all three children's
// operands into fresh slots at every level.
//
// TWO OF THE THREE CHILDREN ARE SUBRANGES OF THE PARENT.
//
//	child 0 = (a_lo, b_lo)          a[0:h]     — already in memory
//	child 1 = (a_hi, b_hi)          a[h:]      — already in memory
//	child 2 = (a_lo+a_hi, …)                   — genuinely new data
//
// So only one child in three needs storage or a write. The other two are an
// OFFSET AND A LENGTH into memory that already exists, which removes two
// thirds of the copying and about two thirds of the operand storage.
//
// THE LAYOUT IS A FLAT TABLE PLUS INDICES, which is this repository's own
// answer to recursive data (data-structures.md, and json/tree.oro):
//
//	arena   a, then b, then every sum buffer, one contiguous run
//	node    (aOff, bOff, len) — three integers, no pointers
//
// A node's children are computed from its descriptor, so the whole tree is a
// table of 3^0 + … + 3^D entries filled by a loop. Nothing recurses, nothing is
// pushed, and the arena size is a function of (n, D) known before any of it
// runs — which is what `build` requires.
//
// Sizes are RAGGED here where karatsuba.go padded them uniform: a sum needs one
// limb more than a half, so `len` shrinks as len' = (len - len/2) + 1 rather
// than halving exactly. Carrying it per node in the descriptor is what makes
// that affordable — the uniform padding existed only to avoid having to.

type KWork2 struct {
	n, D    int
	arena   []uint64 // a, b, then the sum buffers
	aOff    []int    // per node
	bOff    []int
	ln      []int
	prod    []uint64 // every node's product, packed by level
	pOff    []int    // per node, into prod
	nodes   int
	lenOf   []int // operand len at each level
	prodOf  []int // product len at each level
	baseIdx []int // first node id of each level
}

func NewKWork2(n, D int) *KWork2 {
	w := &KWork2{n: n, D: D}
	w.lenOf = make([]int, D+1)
	w.lenOf[0] = n
	for L := 0; L < D; L++ {
		h := w.lenOf[L] / 2
		w.lenOf[L+1] = (w.lenOf[L] - h) + 1
	}
	w.baseIdx = make([]int, D+2)
	for L, acc, p := 0, 0, 1; L <= D+1-1; L, p = L+1, p*3 {
		w.baseIdx[L] = acc
		acc += p
		w.baseIdx[L+1] = acc
		w.nodes = acc
	}
	// The arena: the two inputs, then two sum buffers per node that has
	// children — 3^L of them at each level L < D, sized for level L+1.
	arena := 2 * n
	for L, p := 0, 1; L < D; L, p = L+1, p*3 {
		arena += p * 2 * w.lenOf[L+1]
	}
	w.arena = make([]uint64, arena)
	w.aOff = make([]int, w.nodes)
	w.bOff = make([]int, w.nodes)
	w.ln = make([]int, w.nodes)
	w.pOff = make([]int, w.nodes)
	// PRODUCT SIZES, COMPUTED BOTTOM-UP AND EXACTLY. A parent's buffer must
	// reach 2h + (a child's buffer), because the upward pass adds the high
	// child at offset 2h. A flat slack of +4 is not enough and the first
	// version panicked at n=16 D=1: the buffer was 36 limbs and the write
	// wanted 38.
	w.prodOf = make([]int, D+1)
	w.prodOf[D] = 2 * w.lenOf[D] // what MulLimbs writes
	for L := D - 1; L >= 0; L-- {
		w.prodOf[L] = 2*(w.lenOf[L]/2) + w.prodOf[L+1]
	}
	tot := 0
	for L, p := 0, 1; L <= D; L, p = L+1, p*3 {
		for k := 0; k < p; k++ {
			w.pOff[w.baseIdx[L]+k] = tot
			tot += w.prodOf[L]
		}
	}
	w.prod = make([]uint64, tot)
	return w
}

// KaratsubaInPlace multiplies a by b, both n limbs. The result is the low
// 2n limbs of the returned slice.
func KaratsubaInPlace(a, b []uint64, w *KWork2) []uint64 {
	D, ar := w.D, w.arena
	copy(ar[0:], a)
	copy(ar[w.n:], b)
	w.aOff[0], w.bOff[0], w.ln[0] = 0, w.n, w.n

	// DOWNWARD. Children 0 and 1 are offsets into what is already there;
	// only child 2 is written, and only its two operands.
	free := 2 * w.n
	for L := 0; L < D; L++ {
		base, cbase := w.baseIdx[L], w.baseIdx[L+1]
		for k, p := 0, pow3(L); k < p; k++ {
			id := base + k
			ao, bo, ln := w.aOff[id], w.bOff[id], w.ln[id]
			h := ln / 2
			cl := w.lenOf[L+1]

			c0 := cbase + 3*k
			w.aOff[c0], w.bOff[c0], w.ln[c0] = ao, bo, h

			c1 := c0 + 1
			w.aOff[c1], w.bOff[c1], w.ln[c1] = ao+h, bo+h, ln-h

			// The one child that is new data: lo + hi, for each operand.
			c2 := c0 + 2
			as, bs := free, free+cl
			free += 2 * cl
			w.aOff[c2], w.bOff[c2], w.ln[c2] = as, bs, cl
			sumInto(ar[as:as+cl], ar[ao:ao+h], ar[ao+h:ao+ln])
			sumInto(ar[bs:bs+cl], ar[bo:bo+h], ar[bo+h:bo+ln])
		}
	}

	// BASE CASE.
	{
		base := w.baseIdx[D]
		for k, p := 0, pow3(D); k < p; k++ {
			id := base + k
			ln := w.ln[id]
			po := w.pOff[id]
			MulLimbs(ar[w.aOff[id]:w.aOff[id]+ln], ar[w.bOff[id]:w.bOff[id]+ln],
				w.prod[po:po+w.prodOf[D]])
		}
	}

	// UPWARD, unchanged in shape from karatsuba.go.
	for L := D - 1; L >= 0; L-- {
		base, cbase := w.baseIdx[L], w.baseIdx[L+1]
		csz := w.prodOf[L+1]
		for k, p := 0, pow3(L); k < p; k++ {
			id := base + k
			h := w.ln[id] / 2
			po := w.pOff[id]
			out := w.prod[po : po+w.prodOf[L]]
			zero(out)
			c0 := w.pOff[cbase+3*k]
			z0 := w.prod[c0 : c0+csz]
			z1 := w.prod[c0+csz : c0+2*csz]
			z2 := w.prod[c0+2*csz : c0+3*csz]
			addAt(out, z0, 0)
			addAt(out, z1, 2*h)
			addAt(out, z2, h)
			subAt(out, z0, h)
			subAt(out, z1, h)
		}
	}
	po := w.pOff[0]
	return w.prod[po : po+2*w.n]
}

// sumInto writes lo+hi into dst, which is one limb longer than hi.
func sumInto(dst, lo, hi []uint64) {
	zero(dst)
	copy(dst, hi)
	addAt(dst, lo, 0)
}
