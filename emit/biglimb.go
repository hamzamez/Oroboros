package emit

import (
	_ "embed"
	"fmt"
	"math/big"

	"oroboros/core"
)

// THE FIXED-LIMB RUNG, SELECTED — ADR 0019's ladder, third step.
//
// A range that is FINITE and above the portable window gives a LIMB COUNT,
// which gives a `build` of known length, which gives zero allocations.
// bigarith-2026-08-28 measured that at 3.97x over `math/big`, 6.2x over
// `BigInteger` and 5.8x over `BigInt` — and on the last two it is the ONLY way
// there, because those bignums are immutable. On windows it is the only way
// there at all, because that host ships no bignum.
//
// Until this, a declared endpoint was compared against the window and thrown
// away: `(int 0 (pow 2 1000))` and `(int 0 +inf)` produced identical code. The
// spelling was necessary and nowhere near sufficient.
//
// ═══ THE DECISION
//
//	every big type finite   → the target's declared storage (BigRepr)
//	any big type unbounded  → the host's bignum
//	an operation not in the library → the host's bignum, for the whole program
//
// The last one is coarse on purpose. Mixing representations needs a conversion
// at every boundary between them, and where that boundary sits is a design
// question ADR 0019 opened and did not close — so a program using an operation
// the library lacks keeps the host's bignum whole rather than half, and on a
// target with no bignum it is refused by name.
//
// The library has addition, subtraction, multiplication and division by a
// machine word. What it lacks is the remainder (whose result is a word rather
// than a limb table, so a change of type), the comparisons, and big-by-big
// division (Knuth D, needing a quotient estimate).
//
// ═══ WHY THE WIDTH HAS TO TRAP
//
// This is the rung's real difficulty and it is worth stating plainly. The
// host's bignum is exact whatever the declared range says, so an under-declared
// bound costs nothing there. A FIXED width truncates — and then selecting a
// representation would change the answer, which is ADR 0009's rule at a
// different boundary and the one thing this project refuses.
//
// So the library checks every operation's final carry with `trap-if`, which all
// four targets declare: `panic`, `throw`, `throw`, `ud2`. One comparison per
// operation, not per limb.
//
// That makes the two upper rungs a genuine choice rather than an
// implementation detail, and the choice is exactly the trade bigarith measured:
//
//	(int 0 (pow 2 1300))   fixed width, no allocation, traps at the bound
//	(int 0 +inf)           the host's bignum, exact, allocates
//
// ═══ HOW THE LIBRARY GETS IN
//
// It is spliced and REDUCED at each site, rather than injected before reduction
// the way `win/map` is. The difference is forced: a map operation is visible in
// the source, so `lowerMaps` can rewrite it before the reducer runs — but which
// `+` is a bignum is decided by a solver that needs the RESIDUAL, so by the
// time the answer exists reduction is over. Normalising `(of w v)` at the site
// is what inlines it, and `core.Normalize` on an application of a closed
// definition to residual arguments is exactly that.

//go:embed bignum.oro
var bigLimbSrc string

// bigLimbHostSrc is the conversion back to the host's own bignum, kept apart
// because it NAMES one: a target that declares no arbitrary-precision integer
// cannot even load it, and that target can still do the arithmetic. Computing
// in limbs and rendering the result are two capabilities, and only one of them
// needs a host bignum.
//
//go:embed bignum-host.oro
var bigLimbHostSrc string

// limbBits is the base's exponent. See bignum.oro: a limb product plus its
// carry must stay inside ADR 0012's window, so 2W < 53 and W = 24 leaves four
// bits of headroom.
const limbBits = 24

// limbPrefix is the module the embedded source declares. Every name the rewrite
// produces is qualified with it, so nothing can collide with a program's own.
const limbPrefix = "big/limb."

// limbOf maps a promoted big operation to the library function that replaces
// it, by NAME. What is absent from it is not necessarily unsupported: `big/`
// is handled by shape rather than by name, because dividing by a machine WORD
// is one pass and dividing by another bignum needs a quotient estimate.
//
// Subtraction is here and returns a magnitude: the final borrow is an underflow
// and traps, exactly as the final carry of an addition is an overflow and traps.
// A program declares `(int 0 N)` to reach this rung, so a negative result is
// outside its own declaration.
//
// The comparisons are here and return a bool, which is what lets a bignum
// control a loop at all; without them no `while (x > 1)`, no gcd and no Newton
// iteration was expressible on a target with no host bignum.
//
// `big/` and `big%` are handled by SHAPE below, not here: by a machine word
// each is one pass, and by another bignum each needs a quotient estimate. Only
// that last case is still absent, and a program using it keeps the host's
// bignum whole (or is refused by name on a target with none).
var limbOf = map[string]string{
	"big-of":       limbPrefix + "of",
	"big-of-small": limbPrefix + "of",
	"big+":         limbPrefix + "add",
	"big-":         limbPrefix + "sub",
	"big*":         limbPrefix + "mul",
	"big<":         limbPrefix + "lt",
	"big<=":        limbPrefix + "le",
	"big>":         limbPrefix + "gt",
	"big>=":        limbPrefix + "ge",
	"big=":         limbPrefix + "eq",
}

// BigHull is BigBound with the sign of the set: signed is true when the program
// enforces (−2^bits, 2^bits), false when [0, 2^bits).
func BigHull(w core.Word, sigs ...*core.Sig) (bits int, signed, bounded bool) {
	var tys []string
	for _, sig := range sigs {
		if sig == nil {
			continue
		}
		tys = append(tys, sig.Result)
		tys = append(tys, sig.Results...)
		for _, sp := range sig.Params {
			tys = append(tys, sp.Type)
		}
	}
	bits, any := 0, false
	for _, ty := range tys {
		if w.ValueType(ty) != core.BigType {
			continue
		}
		if core.UnboundedRange(ty) {
			return 0, false, false // ℤ is not an interval
		}
		lo, hi, ok := core.IntRangeBig(ty)
		if !ok {
			return 0, false, false
		}
		any = true
		if n := bitsFor(lo, hi); n > bits {
			bits = n
		}
		if lo.Sign() < 0 {
			signed = true
		}
	}
	if !any || bits <= 2*limbBits {
		// Under three limbs cannot happen for a range above the window — three
		// base-2^24 limbs hold 2^72 — so this is a guard rather than a case,
		// and it is what lets `of` skip its own overflow check.
		return 0, false, false
	}
	return bits, signed, true
}

func bitsFor(lo, hi *big.Int) int {
	bits := hi.BitLen()
	if n := lo.BitLen(); n > bits {
		bits = n
	}
	return bits
}

// BigRepr chooses how a bounded arbitrary-precision value is STORED, and it is
// the target that decides — not the shape of the declaration.
//
// Before this, a finite range selected fixed limbs and `+inf` selected the
// host's bignum, so a declaration about MAGNITUDE was read as a command about
// storage. On V8 that cost 100x for the same computation — 528,334 ns against
// 5,290 for 200! — because `BigInt` is C++ with 64-bit limbs and ours is
// portable Oroboros with 24-bit ones and `/` for its carry (biglimb-2026-09-02).
// A programmer who wrote the more informative declaration got the slower
// program, silently, from a choice they did not make.
//
// There is no total order to select from the way `int-repr` has one, so this
// cannot be derived the way an element width is: bigarith-2026-08-28 measured
// ours winning where the operation is LINEAR and the host winning where it is
// QUADRATIC. So a target declares what somebody measured, and when the limb
// library gets 64-bit limbs and a bitwise carry the thing that changes is a
// target file and no program moves.
//
// The default when a target says nothing is the only one it can do: the host's
// bignum where there is one, fixed limbs where there is not.
//
// It returns the limb width as well, which is derived from the bound rather
// than declared — one width, because one function holds one: `add` reads two
// operands and writes a third, and three different lengths would be three
// different functions.
//
// LIMBS HOLD A MAGNITUDE, so they realize [0, 2ᵏ) and nothing signed (ADR 0029,
// decision 4). A signed program takes the host's bignum where there is one, as
// a program with an operation the limb library lacks already does; where there
// is none it stays here and PromoteBig refuses it by name.
func BigRepr(tgt *Target, sigs ...*core.Sig) (limbs bool, w, bits int) {
	bits, signed, bounded := BigHull(tgt.Word, sigs...)
	if !bounded {
		return false, 0, 0
	}
	kind := tgt.BigRepr
	if kind == "" {
		kind = "host"
		if !tgt.HasBig() {
			kind = "limbs"
		}
	}
	if signed && tgt.HasBig() {
		kind = "host"
	}
	if kind == "host" && tgt.HasBig() {
		return false, 0, bits
	}
	return true, (bits + limbBits - 1) / limbBits, bits
}

// limbLimit is the ceiling on the TOP limb that makes the limb rung enforce the
// same bound as the host rung: with w limbs holding a bound of `bits` bits, the
// carry check alone would admit everything under 2^(24w), which is up to 24
// bits more than declared.
//
// When the bound is a whole number of limbs this is 2^24, and a limb is under
// 2^24 by construction — so the check is present, free, and never fires. That
// is the right shape for a check that exists to make two representations agree.
func limbLimit(bits, w int) int64 {
	return int64(1) << uint(bits-limbBits*(w-1))
}

// limbLib is the embedded library, loaded once per target.
type limbLib struct {
	prog *core.Program
	env  *core.Env
	w    int   // limbs
	lim  int64 // ceiling on the top limb (see limbLimit)
}

func loadLimbLib(tgt *Target) (*limbLib, error) {
	src := bigLimbSrc
	if tgt.HasBig() {
		src += "\n" + bigLimbHostSrc
	}
	forms, err := core.Read(src)
	if err != nil {
		return nil, fmt.Errorf("the built-in bignum does not parse: %w", err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		return nil, fmt.Errorf("the built-in bignum does not load: %w", err)
	}
	env, err := tgt.Env(prog)
	if err != nil {
		return nil, fmt.Errorf("the built-in bignum does not cover on %s: %w", tgt.Name, err)
	}
	return &limbLib{prog: prog, env: env}, nil
}
