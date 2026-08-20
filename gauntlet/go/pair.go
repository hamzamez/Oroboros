package gauntlet

// Is a two-field product free? That is the gate: strings, binaries, error
// results, one `idiv` yielding two answers, and a bignum with an inline fast
// path all wait behind it, and CLAUDE.md names this as exactly where the
// predecessor project died.
//
// Go has multiple return values, which ARE a product, and a value struct, which
// is another. Neither should allocate. Measured rather than assumed.

type pairV struct{ a, b int64 }

func divmodTuple(n, d int64) (int64, int64) { return n / d, n % d }
func divmodStruct(n, d int64) pairV         { return pairV{n / d, n % d} }
func divmodPtr(n, d int64) *pairV           { return &pairV{n / d, n % d} }

// PairInline is the baseline: no product at all, both answers computed where
// they are used. This is what the language forces today.
func PairInline(xs []int64) int64 {
	s := int64(0)
	for _, x := range xs {
		s += (x / 7) + (x % 7)
	}
	return s
}

func PairTuple(xs []int64) int64 {
	s := int64(0)
	for _, x := range xs {
		q, r := divmodTuple(x, 7)
		s += q + r
	}
	return s
}

func PairStruct(xs []int64) int64 {
	s := int64(0)
	for _, x := range xs {
		p := divmodStruct(x, 7)
		s += p.a + p.b
	}
	return s
}

// PairPtr is the version that SHOULD allocate unless escape analysis removes
// it — the control that tells us the other two mean anything.
func PairPtr(xs []int64) int64 {
	s := int64(0)
	for _, x := range xs {
		p := divmodPtr(x, 7)
		s += p.a + p.b
	}
	return s
}
