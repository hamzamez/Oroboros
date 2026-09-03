package emit

import (
	_ "embed"
	"fmt"
	"math/big"
	"strings"

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
//	every big type finite   → limbs, width = ceil(maxbits / 24)
//	any big type unbounded  → the host's bignum
//	an operation not in the library → the host's bignum, for the whole program
//
// The last one is coarse on purpose. Mixing representations needs a conversion
// at every boundary between them, and where that boundary sits is a design
// question ADR 0019 opened and did not close — so a program that subtracts or
// divides keeps the host's bignum whole rather than half.
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
// it. What is ABSENT is the point: subtraction can go negative and these are
// magnitudes, and division needs a quotient estimate — so a program using one
// keeps the host's bignum whole (see the header).
var limbOf = map[string]string{
	"big-of":       limbPrefix + "of",
	"big-of-small": limbPrefix + "of",
	"big+":   limbPrefix + "add",
	"big*":   limbPrefix + "mul",
}

// LimbWidth is how many base-2^24 limbs a signature's declared ranges need, and
// whether the fixed-limb rung applies at all.
//
// It is the MAXIMUM over every big type in EVERY signature the program has, and
// taking the whole program is not laziness. Reduction inlines every
// non-exported call, so by the time the representation is selected a helper's
// declared range is gone — the same structural limit refinements.md §6b records
// for a `where`, arriving for the fourth time. `main` has no signature at all,
// so a per-function width would mean no whole program could ever take this rung.
//
// One width, because one function holds one: `add` reads two operands and
// writes a third, and three different lengths would be three different
// functions. The maximum is the declaration the programmer made, and exceeding
// it traps.
//
// An UNBOUNDED range takes the whole signature to the host's bignum, which is
// the distinction `(int 0 +inf)` was added to make expressible: ℤ has no limb
// count, and no fixed number of limbs contains it.
func LimbWidth(sigs ...*core.Sig) (int, bool) {
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
	w, any := 0, false
	for _, ty := range tys {
		if core.ValueType(ty) != core.BigType {
			continue
		}
		if core.UnboundedRange(ty) {
			return 0, false // ℤ has no limb count
		}
		lo, hi, ok := core.IntRangeBig(ty)
		if !ok {
			return 0, false
		}
		any = true
		if n := limbsFor(lo, hi); n > w {
			w = n
		}
	}
	if !any || w < 3 {
		// Under three limbs cannot happen for a range above the window — three
		// base-2^24 limbs hold 2^72 — so this is a guard rather than a case,
		// and it is what lets `of` skip its own overflow check.
		return 0, false
	}
	return w, true
}

func limbsFor(lo, hi *big.Int) int {
	bits := hi.BitLen()
	if n := lo.BitLen(); n > bits {
		bits = n
	}
	return (bits + limbBits - 1) / limbBits
}

// LimbSupported reports whether every big operation in a promoted residual has
// a limb form. One that does not takes the program back to the host's bignum.
func LimbSupported(t *core.Term) bool {
	if t == nil {
		return true
	}
	if t.Kind == core.KName && isBigOp(t.Name) {
		if _, ok := limbOf[t.Name]; !ok && t.Name != "big-str" {
			return false
		}
	}
	for _, k := range t.Kids {
		if !LimbSupported(k) {
			return false
		}
	}
	return true
}

// limbLib is the embedded library, loaded once per target.
type limbLib struct {
	prog *core.Program
	env  *core.Env
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

// LowerLimbs rewrites every big operation into the embedded library, at width w.
//
// BOTTOM-UP, because an operand must already be a limb table before the
// operation that reads it is spliced — and because `big-str`'s conversion back
// to the host's bignum must NOT be walked again, or its own `big+` would be
// lowered to limbs.
func LowerLimbs(tgt *Target, w int, t *core.Term) (*core.Term, int, error) {
	lib, err := loadLimbLib(tgt)
	if err != nil {
		return nil, 0, err
	}
	n := 0
	out, err := lib.rewrite(tgt, w, t, &n)
	return out, n, err
}

func (l *limbLib) rewrite(tgt *Target, w int, t *core.Term, n *int) (*core.Term, error) {
	if t == nil {
		return nil, nil
	}
	// A LAMBDA IS WALKED THROUGH, INCLUDING AT THE TOP. The residual handed to
	// this pass is a function, so a rewrite that only descended into
	// applications would silently do nothing at all — which is what it did.
	if t.Kind == core.KFn {
		b, err := l.rewrite(tgt, w, t.Closed(), n)
		if err != nil {
			return nil, err
		}
		return core.FnClosed(t.Params, b), nil
	}
	if t.Kind != core.KApp {
		return t, nil
	}
	// A MULTIPLY BY A WIDENED WORD IS A DIFFERENT ALGORITHM, and it has to be
	// recognised BEFORE the children are rewritten: once `(big-of i)` has been
	// spliced it is an ordinary `build` and nothing distinguishes it from any
	// other limb table.
	//
	// It is worth a factor of w — `mul-small` is one pass where `mul` is w
	// passes — and a factorial is nothing but this. The shape is exactly what
	// the promotion produces for `(* acc i)` with `i` a machine word, which is
	// rule (P)'s provability gate leaving it one.
	if op := t.Op(); op.Kind == core.KName && op.Name == "big*" && len(t.Args()) == 2 {
		if bigSide, word, ok := l.widenedOperand(t.Args()); ok {
			a, err := l.rewrite(tgt, w, bigSide, n)
			if err != nil {
				return nil, err
			}
			*n++
			return l.splice(limbPrefix+"mul-small", w, a, word)
		}
	}
	kids := make([]*core.Term, len(t.Kids))
	for i, k := range t.Kids {
		// A lambda's body is rewritten in place: its binders are de Bruijn
		// indices and the splice introduces no free names, so nothing has to be
		// opened and nothing can capture.
		if k.Kind == core.KFn {
			b, err := l.rewrite(tgt, w, k.Closed(), n)
			if err != nil {
				return nil, err
			}
			kids[i] = core.FnClosed(k.Params, b)
			continue
		}
		b, err := l.rewrite(tgt, w, k, n)
		if err != nil {
			return nil, err
		}
		kids[i] = b
	}
	t = &core.Term{Kind: core.KApp, Kids: kids}
	op := t.Op()
	if op.Kind != core.KName {
		return t, nil
	}
	args := t.Args()

	if op.Name == "big-str" && len(args) == 1 {
		return l.toHost(tgt, w, args[0], n)
	}
	fn, ok := limbOf[op.Name]
	if !ok {
		return t, nil
	}
	*n++
	return l.splice(fn, w, args...)
}

// widenedOperand splits a product into (big, word) when one side is a machine
// word that was widened for the multiply AND is small enough to be a limb
// multiplier.
func (l *limbLib) widenedOperand(args []*core.Term) (*core.Term, *core.Term, bool) {
	isWiden := func(t *core.Term) (*core.Term, bool) {
		if t != nil && t.Kind == core.KApp && t.Op().Kind == core.KName &&
			t.Op().Name == "big-of-small" && len(t.Args()) == 1 {
			// AND THE WORD HAS TO BE SMALL ENOUGH TO BE A MULTIPLIER, which the
			// shape alone cannot say. `mul-small` computes `limb * k + carry` in
			// one word, so k must stay under 2^28; a big literal's Horner spine
			// multiplies by 10^15 and would overflow every column.
			//
			// The analysis named the widenings that qualify (widenTo); anything
			// spelled `big-of` is spread over limbs and multiplied properly.
			return t.Args()[0], true
		}
		return nil, false
	}
	if v, ok := isWiden(args[1]); ok {
		return args[0], v, true
	}
	if v, ok := isWiden(args[0]); ok {
		return args[1], v, true
	}
	return nil, nil, false
}

// splice applies a library definition to its arguments and REDUCES, which is
// what inlines it. The width goes first and is a literal, so every loop bound
// and every buffer length in the result is a constant.
func (l *limbLib) splice(fn string, w int, args ...*core.Term) (*core.Term, error) {
	kids := []*core.Term{core.Name(fn), core.Int(int64(w))}
	kids = append(kids, args...)
	nf, err := core.Normalize(&core.Term{Kind: core.KApp, Kids: kids}, l.env, core.DefaultFuel)
	if err != nil {
		return nil, fmt.Errorf("splicing %s: %w", strings.TrimPrefix(fn, limbPrefix), err)
	}
	return nf, nil
}

// toHost converts a limb table back to the host's bignum so that it can be
// printed, because no host prints a limb table and every program that computes
// a big value has to say it eventually.
//
// A TARGET WITHOUT A BIGNUM CANNOT DO THIS, and refusing is the honest answer:
// windows can compute in limbs — which is what ADR 0019 item 4 asks for — and
// cannot render the result until decimal conversion is written, which needs a
// string surface this language does not have.
func (l *limbLib) toHost(tgt *Target, w int, x *core.Term, n *int) (*core.Term, error) {
	if !tgt.HasBig() {
		return nil, fmt.Errorf("this program renders a fixed-limb bignum as a string, "+
			"and target %s declares no arbitrary-precision integer to convert it "+
			"through.\n"+
			"  The arithmetic works there; only the rendering does not. Decimal\n"+
			"  conversion from limbs needs a string surface this language does not\n"+
			"  have yet (docs/spec/strings.md).", tgt.Name)
	}
	*n++
	acc, err := l.splice(limbPrefix+"to-host", w, x)
	if err != nil {
		return nil, err
	}
	return core.App(core.Name("big-str"), acc), nil
}

// LimbSig is a signature as the fixed-limb rung means it: every type above the
// portable window becomes `array int`, because on this rung a big value IS a
// table of limbs.
//
// The checker needs it for the same reason the promotion has to run before the
// checker at all — the representation is part of what the program MEANS. A body
// that produces limbs would otherwise be refused against a signature that says
// `big`, which is true of the declaration and false of the code.
func LimbSig(sig *core.Sig, on bool) *core.Sig {
	if !on || sig == nil {
		return sig
	}
	out := *sig
	out.Params = append([]core.SigParam(nil), sig.Params...)
	out.Results = append([]string(nil), sig.Results...)
	limb := func(ty string) string {
		if core.ValueType(ty) == core.BigType {
			return "array int"
		}
		return ty
	}
	out.Result = limb(out.Result)
	for i := range out.Results {
		out.Results[i] = limb(out.Results[i])
	}
	for i := range out.Params {
		out.Params[i].Type = limb(out.Params[i].Type)
	}
	return &out
}
