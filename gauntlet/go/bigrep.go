package gauntlet

import "math/big"

// HAND-WRITTEN REFERENCES FOR ARBITRARY PRECISION — ADR 0019's third escape.
//
// `bigarith.go` asks a different question: what does the HOST's bignum cost
// against a limb form we could emit, which is what decides whether R3 should be
// a target capability or a library we write. This file asks the question this
// project always asks of emitted code: is what we emit at parity with what a
// person would write on that host.
//
// It reuses bigarith's references wherever they exist — `FactBigNaive`,
// `FactBigReuse`, `FibBigReuse` — because the comparison is only worth as much
// as its connection to the numbers already taken, and re-declaring the same
// loop under a second name is how two measurements drift apart.
//
// WHAT PARITY MEANS HERE, stated before the numbers so they cannot be read as
// more than they are. The emitted form calls `math/big` operation for
// operation, so the comparison is against the way a Go programmer FIRST writes
// `math/big` in a loop — and bigarith-2026-08-28 already measured that naive
// form at 4-5x worse than a careful one and 3.97x worse than a fixed-limb
// `build`. So the rung above this one is real, measured, and not built.
//
// Saying which form is the reference is the point rather than a hedge: "at
// parity" without "with what" is the shape of error this repository has caught
// in itself three times.

// FibBigNaive is fib(n) the way a Go programmer first writes it: two bignum
// accumulators, a machine-word counter, a fresh result per step.
//
// bigarith.go has `FibBigReuse`, the careful form, and not this one — because
// its question was how well `math/big` CAN be used. This one is here because it
// is the shape our emitter produces, and the gap between the two is what a
// mutable bignum buffer would be worth.
func FibBigNaive(n int) *big.Int {
	a, b := big.NewInt(0), big.NewInt(1)
	for i := 0; i < n; i++ {
		a, b = b, new(big.Int).Add(a, b)
	}
	return a
}

// PowerBigNaive is exponentiation by squaring with two bignum variables and a
// machine-word exponent.
//
// `x` is a bignum here because it SQUARES, and that is the fact rule (P) has to
// derive rather than be told: nothing declares `x`, it is never returned, and
// no big value is ever assigned to it. A representation solver that missed it
// would emit a program whose accumulator is scrupulously exact and whose
// multiplicand has silently wrapped.
func PowerBigNaive(b, e int) *big.Int {
	acc, x := big.NewInt(1), big.NewInt(int64(b))
	for k := e; k != 0; k /= 2 {
		if k%2 == 1 {
			acc = new(big.Int).Mul(acc, x)
		}
		x = new(big.Int).Mul(x, x)
	}
	return acc
}

// PowerBigReuse is the careful form, for the same reason FactBigReuse exists:
// to price what our emitter cannot yet reach. `math/big` writes into its
// receiver, so a caller that owns its temporaries allocates nothing per step.
//
// Reaching it needs the bignum to be a MUTABLE buffer threaded through the
// loop, which is ADR 0018's linear buffer at a type the language does not have
// — the same gap arrays-revisited.md's trigger 2 names for Karatsuba's
// workspace and ADR 0013 names for the stencil. Three places, one gap.
func PowerBigReuse(b, e int) *big.Int {
	acc, x, t := big.NewInt(1), big.NewInt(int64(b)), new(big.Int)
	for k := e; k != 0; k /= 2 {
		if k%2 == 1 {
			t.Mul(acc, x)
			acc, t = t, acc
		}
		t.Mul(x, x)
		x, t = t, x
	}
	return acc
}

// PairBig is the reference for `examples/big/pair.oro`, and it exists to make
// rule R's liveness condition FALSIFIABLE.
//
// Both accumulators are read twice at each step — once by their own update and
// once by the other's — so neither one's storage is free and neither update may
// be written in place. With condition (3) removed the compiler writes
// `a.Mul(a, b)` and the addition then adds the NEW a, and the answer changes.
//
// It took three attempts to find a shape where that is true. `power` and the
// two obvious two-accumulator loops are saved by the Go emitter's own
// scheduling: `PostVars` hoists an update into the post clause when it reads
// nothing but its own variable, and a hoisted update runs after the body, where
// the hazard cannot arise. Both updates here read the OTHER variable, so both
// stay in the body and the order is exposed. A test that relies on being saved
// by the scheduler is a test of the scheduler.
func PairBig(n int) *big.Int {
	a, b := big.NewInt(2), big.NewInt(1)
	for i := 0; i < n; i++ {
		a, b = new(big.Int).Mul(a, b), new(big.Int).Add(b, a)
	}
	return b
}
