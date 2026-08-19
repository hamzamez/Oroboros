package gauntlet

// Two ways to write a conjunction, measured against each other.
//
// docs/spec/booleans.md proposes that `and` becomes reader sugar for a nested
// conditional — `(and a b)` ⟶ `(if a b false)` — with each backend recognising
// the shape and emitting the host's own `&&`. The objection is CLAUDE.md's
// most-cited one: NEVER LOWER FURTHER THAN THE TARGET REQUIRES. If the nested
// form is slower, the peephole is load-bearing rather than cosmetic.
//
// There is also a reason to think the nested form might WIN. ADR 0015 measured
// that a compound loop condition defeats Go's bounds-check elimination at
// 1.61×. A nested conditional is not a compound condition. If splitting the
// guard restores BCE, the sugar is faster than the operator on the host we care
// most about — and the recommendation gets stronger for a reason nobody
// predicted.
//
// Carry both forms, per CLAUDE.md: the one expected to win and the one expected
// to lose.

// ---------------------------------------------------------------- value position

// AndOperator is `p && q` — what a target file emits today.
func AndOperator(p, q bool) bool { return p && q }

// AndNested is `(if p q false)` — what the sugar produces before any peephole.
func AndNested(p, q bool) bool {
	if p {
		return q
	}
	return false
}

// ---------------------------------------------------------------- guard position
//
// A two-array walk under one compound guard: the shape ADR 0015 measured, and
// the shape `dot` and `centroid` have.

// SumWhileOperator guards with a compound condition.
func SumWhileOperator(a, b []float64) float64 {
	acc := 0.0
	i := 0
	for i < len(a) && i < len(b) {
		acc += a[i] * b[i]
		i++
	}
	return acc
}

// SumWhileNested splits the same guard into two conditionals.
func SumWhileNested(a, b []float64) float64 {
	acc := 0.0
	i := 0
	for {
		if i < len(a) {
			if i < len(b) {
				acc += a[i] * b[i]
				i++
				continue
			}
		}
		break
	}
	return acc
}

// ---------------------------------------------------------------- three-term guard
//
// The same question one operand deeper, where a chain of `&&` is most likely to
// differ from a chain of conditionals.

// AnyRangeOperator scans for an element strictly inside a band.
func AnyRangeOperator(a []float64, lo, hi float64) int {
	n := 0
	for i := 0; i < len(a); i++ {
		if a[i] > lo && a[i] < hi && a[i] != 0 {
			n++
		}
	}
	return n
}

// AnyRangeNested is the same predicate as nested conditionals.
func AnyRangeNested(a []float64, lo, hi float64) int {
	n := 0
	for i := 0; i < len(a); i++ {
		if a[i] > lo {
			if a[i] < hi {
				if a[i] != 0 {
					n++
				}
			}
		}
	}
	return n
}
