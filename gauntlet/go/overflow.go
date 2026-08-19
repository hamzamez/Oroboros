package gauntlet

// What arbitrary-precision integers cost when they DO NOT overflow.
//
// Erlang, Common Lisp, Python, Ruby and Smalltalk all present one integer type
// that silently becomes a bignum. The bignum path allocates and is obviously
// expensive; the interesting number is the other one — what every ordinary
// addition pays for the *possibility*, when the value stays small and nothing
// is allocated at all.
//
// That number decides whether unbounded integers are affordable here, and no
// amount of argument produces it. See docs/spec/numbers.md.
//
// Three shapes, because the check has three plausible costs:
//   - the branch itself, perfectly predicted;
//   - the dependency it adds to the loop-carried chain;
//   - what it does to vectorisation and unrolling.

// SumPlain is the baseline: machine addition, wrapping, no check.
func SumPlain(a []int64) int64 {
	s := int64(0)
	for _, x := range a {
		s += x
	}
	return s
}

// SumChecked is the fixnum path of a bignum representation: add, then test the
// standard signed-overflow condition and branch away on the rare case.
//
// The test is the usual one — the sum's sign disagrees with the addend's — and
// it is what a runtime that unboxes small integers actually emits.
func SumChecked(a []int64) int64 {
	s := int64(0)
	for _, x := range a {
		t := s + x
		if (t > s) != (x > 0) {
			return overflowed(s, x)
		}
		s = t
	}
	return s
}

// SumWindowed is the OTHER way to price it, and the one this project already
// half-uses: instead of detecting overflow after the fact, check the value
// stays inside the portable window (ADR 0012's +/-(2^53-1)).
func SumWindowed(a []int64) int64 {
	const lim = 1<<53 - 1
	s := int64(0)
	for _, x := range a {
		s += x
		if s > lim || s < -lim {
			return overflowed(s, 0)
		}
	}
	return s
}

//go:noinline
func overflowed(s, x int64) int64 { return s ^ x }
