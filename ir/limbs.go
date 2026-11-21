package ir

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// THE FIXED-LIMB RUNG, ON THE IR (ADR 0035).
//
// SelectBig chooses the values held exactly and writes the host's shape:
// `big+ − · / %`, `big%-small`, the comparisons, `big-of` and `decimal`. On the
// fixed-limb rung each of those is an INSTANCE of a lemma of the limb library
// (emit.LimbFunc), checked once per width: this pass replaces each call by its
// instance, inlined. Inlining is substitution into a verified body, so what is
// left to decide at the site is only what depends on the site, the word
// arithmetic under the site's facts, and the decision taken again decides it.
//
// Two shapes are chosen by FACTS:
//   - a product or a quotient by a widened word k with k ∈ [0, 2²⁸) is one pass,
//     `mul-small` or `div-small`, where `limb·k + carry` stays under 2⁵³;
//   - `big-of v` is `of`, whose limbs are v's digits in base 2²⁴, which is a
//     representation of v iff v ≥ 0. Limbs hold a MAGNITUDE (ADR 0029), so a
//     widened value that may be negative is refused, naming its interval.
//
// An operation with no instance (a quotient or remainder by a bignum) returns
// ErrNoLimbForm; a driver then takes the host's bignum for the program where
// the target has one.

// PlantRenderEverywhere renders through the limb library on every target, a
// host bignum or not: a witness that render agrees with each host's numeral.
var PlantRenderEverywhere = false

// PlantSignFault takes the magnitude's `add` for a signed sum, ignoring the
// signs: TestASignedSumIsTheGroupCompletions must see the wrong numeral.
var PlantSignFault bool

// PlantInlineAll inlines every K(ℕ*) instance, as before ADR 0101: a
// measurement's baseline.
var PlantInlineAll bool

// PlantNoInlineFallback refuses a program whose K(ℕ*) instance cannot be a
// function, as ADR 0101 did, instead of inlining it (ADR 0102, P4).
var PlantNoInlineFallback bool

// PlantNatFault takes K(ℕ*)'s sum for its difference:
// TestAnUnboundedDifferenceIsTheNumeralsOfAnyLength must see the wrong numeral.
var PlantNatFault bool

// PlantNoNatFallback keeps a bounded program the fixed limbs cannot finish on
// them, refused: TestABoundedQuotientMovesToTheNumeralsOfAnyLength must see the
// refusal.
var PlantNoNatFallback bool

// signedLimb is each magnitude instance's form on Z(V_n) (ADR 0098).
var signedLimb = map[string]string{
	"of": "sof", "add": "sadd", "sub": "ssub", "mul": "smul", "mul-small": "smul-small",
	"div-small": "sdiv-small", "rem-small": "srem-small",
	"lt": "slt", "le": "sle", "gt": "sgt", "ge": "sge", "eq": "seq", "render": "srender",
}

// ErrNoLimbForm says the limb library has no instance for an operation.
var ErrNoLimbForm = errors.New("the limb library has no form for an operation")

// smallWord is the largest one-pass multiplier or divisor, 2²⁸ − 1.
var smallWord = ei(1<<28 - 1)

// LowerLimbs replaces f's bignum operations by instances of the limb library at
// the width the program's one set gives (emit.LimbWidth), or, for an unbounded
// set (rung.Var), by instances of K(ℕ*), numerals of any length (ADR 0099),
// which are signed by construction and have no width. It reports how many it
// replaced.
// natBit is each of ADR 0082's operations' definition on K(ℕ*).
var natBit = map[string]string{"big&": "band", "big|": "bor", "big⊕": "bxor", "big~": "bnot",
	"big<<": "bshl", "big>>": "bshr", "big-mod": "bmod"}

func LowerLimbs(tg *emit.Target, f *Func, rung emit.BigPlan) (int, error) {
	bits, signed, nat := rung.Bits, rung.Signed, rung.Var
	n, lim := emit.LimbWidth(bits)
	fs := analyse(tg, f)
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	// widened is k when v = big-of k with k ∈ [0, 2²⁸), the one-pass bound of
	// the fixed limbs' base 2³²; product is the same for a product's operand,
	// which on K(ℕ*), base B = 2^dbits, is any digit: a·k + c ≤ (B − 1)² +
	// (B − 1) is the double word's (ADR 0103, 0124). At 64 bits a digit is
	// any natural word; at 32, a natural below 2³².
	widenedTo := func(v V, bound ep) (V, bool) {
		s := def[v]
		if s == nil || s.Op != OCall || s.Name != "big-of" || len(s.Args) != 1 {
			return -1, false
		}
		k := s.Args[0]
		return k, fs[k].v.within(ep{}, bound)
	}
	widened := func(v V) (V, bool) { return widenedTo(v, smallWord) }
	productBound := smallWord
	if nat {
		productBound = digitMax(tg)
	}
	product := func(v V) (V, bool) { return widenedTo(v, productBound) }
	// THE PLAN: for each call, the instance and its arguments.
	type use struct {
		fn   string
		args []V
	}
	plan := map[*Stmt]use{}
	var widenings []*Stmt
	var err error
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OCall || err != nil {
				continue
			}
			switch s.Name {
			case "big-of":
				widenings = append(widenings, s)
			case "big-word":
				// r onto the word (ADR 0079 item 4): K(ℕ*)'s one digit and its
				// sign; fixed limbs, whose word the instance does not know,
				// have none yet
				if nat {
					plan[s] = use{"word", s.Args}
				} else {
					err = fmt.Errorf("%w: a value above the word brought back into it, from fixed limbs", ErrNoLimbForm)
				}
			case "big*":
				a, b := s.Args[0], s.Args[1]
				if nat && a == b {
					// x·x IS THE SQUARE, q(x) = μ(x, x): one value twice, so the
					// identity of its operands is a fact of the term, and
					// polarization halves its products (karatsuba-research.md §12)
					plan[s] = use{"sqr", []V{a}}
				} else if k, ok := product(b); ok {
					plan[s] = use{"mul-small", []V{a, k}}
				} else if k, ok := product(a); ok {
					plan[s] = use{"mul-small", []V{b, k}}
				} else {
					plan[s] = use{"mul", s.Args}
				}
			case "big/":
				if k, ok := widened(s.Args[1]); ok {
					plan[s] = use{"div-small", []V{s.Args[0], k}}
				} else if nat {
					// Algorithm D, by a numeral (ADR 0099)
					plan[s] = use{"div", s.Args}
				} else {
					err = fmt.Errorf("%w: division by another arbitrary-precision value", ErrNoLimbForm)
				}
			case "big%":
				if nat {
					plan[s] = use{"rem", s.Args}
				} else {
					err = fmt.Errorf("%w: the remainder by another arbitrary-precision value", ErrNoLimbForm)
				}
			case "decimal":
				// THE RENDERING: through the host's bignum where the target has
				// one, whose own numeral is its toString; elsewhere the limb
				// library's render, ι of the digits (ADR 0095).
				if !tg.HasBig() || PlantRenderEverywhere {
					// only a value held as limbs: one the facts kept in the
					// word is rendered by the word's term (RealizeOps)
					if f.Types[s.Args[0]] == core.BigType {
						plan[s] = use{"render", s.Args}
					}
					continue
				}
				plan[s] = use{"to-host", s.Args}
			case "big&", "big|", "big⊕", "big~", "big<<", "big>>", "big-mod":
				// ADR 0082's operations on K(ℕ*); the fixed limbs have none yet
				if nat {
					plan[s] = use{natBit[s.Name], s.Args}
				} else {
					err = fmt.Errorf("%w: %s on fixed limbs", ErrNoLimbForm, s.Name)
				}
			case "big-fit", "big-fit-signed":
				if nat {
					// a bounded program on K(ℕ*): its one set, by sign and bit length
					plan[s] = use{strings.TrimPrefix(s.Name, "big-"), s.Args}
				} else {
					err = fmt.Errorf("internal: a bound check on the fixed-limb rung, whose instances enforce the bound themselves")
				}
			default:
				if fn := emit.LimbShape(s.Name, false); fn != "" && isBigName(s.Name) {
					plan[s] = use{fn, s.Args}
				}
			}
		}
	})
	if err != nil {
		return 0, err
	}
	// A WIDENING IS AN INSTANCE only where something still reads it as limbs:
	// one whose every reader became a one-pass instance, which takes the word
	// itself, is dead.
	uses := useCounts(f)
	for p, u := range plan {
		if u.fn == "mul-small" || u.fn == "div-small" {
			for _, a := range p.Args {
				if d := def[a]; d != nil && d.Name == "big-of" && d.Args[0] == u.args[1] {
					uses[a]--
				}
			}
		}
	}
	for _, s := range widenings {
		if uses[s.Res[0]] == 0 {
			continue
		}
		v := s.Args[0]
		if !signed && !nat && !fs[v].v.within(ep{}, u64Max) {
			return 0, fmt.Errorf("a value widened into arbitrary precision is %s, which may be negative, and "+
				"target %s holds arbitrary precision as fixed limbs, which hold a magnitude: they "+
				"realize [0, 2^k) and nothing signed (ADR 0029)", termShow(fs[v].v), tg.Name)
		}
		plan[s] = use{"of", s.Args}
	}
	// THE SIGNED PROGRAM, Z(V_n) (ADR 0098): each magnitude instance is its
	// signed one, the same shape over a table one entry longer.
	if signed && !nat {
		for s, u := range plan {
			sf, ok := signedLimb[u.fn]
			if PlantSignFault && u.fn == "add" {
				sf = "add"
			}
			if !ok {
				return 0, fmt.Errorf("internal: the limb library's %s has no signed form", u.fn)
			}
			plan[s] = use{sf, u.args}
		}
	}
	if len(plan) == 0 {
		return 0, nil
	}
	callees := map[string]*Func{}
	callee := func(fn string) (*Func, error) {
		if c, ok := callees[fn]; ok {
			return c, nil
		}
		inst, err := emit.LimbFunc(tg, fn, n, lim)
		prefix := "limb-"
		if nat {
			nf := fn
			if PlantNatFault && fn == "sub" {
				nf = "add"
			}
			inst, err = emit.NatFunc(tg, nf)
			prefix = "nat-"
		}
		if err != nil {
			return nil, err
		}
		c, err := Lower(tg, prefix+fn, inst.Sig, inst.Term, Options{Decided: true})
		if err != nil {
			return nil, fmt.Errorf("lowering the library's %s: %v", fn, err)
		}
		callees[fn] = c
		return c, nil
	}
	// A VALUE HELD EXACTLY IS A TABLE OF LIMBS here, which is ρ_T of a range
	// above the word on this rung (spec §4.2).
	for v, ty := range f.Types {
		if ty == core.BigType {
			f.Types[v] = "array int"
		}
	}
	// AND ON K(ℕ*) A RESULT HELD EXACTLY IS A NUMERAL'S TABLE, its digits in
	// [0, 2⁶⁴): the type each instance's own result and parameters have
	// (progfunc.go). Declared `array int`, an export's result was a second fixed
	// representation in the class of the tables it joins (Theorem D′, final.go),
	// which W5 reported: gen refused examples/big's fib, pair, fact and power
	// on windows while their code was sound (gauntlet-2026-11-16's harness). It
	// is a claim, proven after the rung's decision by the body's own facts
	// (pipeline.go), as an instance's is.
	for j, ty := range f.Results {
		if ty == core.BigType {
			f.Results[j] = "array int"
			if nat && !PlantNumeralResultInt {
				f.Results[j] = NumeralType(tg)
			}
		}
	}
	ren := map[V]V{}
	count := 0
	var region func(r *Region)
	region = func(r *Region) {
		var out []Stmt
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, sub := range s.Sub {
				region(sub)
			}
			u, ok := plan[s]
			if !ok {
				out = append(out, *s)
				continue
			}
			// A LIBRARY INSTANCE IS A FUNCTION OF THE PROGRAM (ADR 0101): the
			// large ones are called, decided once under their parameters' sets.
			// AN INSTANCE ITS CONTRACT DOES NOT PROVE IS INLINED (ADR 0102, P4):
			// which definitions are kept never changes a verdict, so a callee
			// refused under its parameters' sets is decided at its sites, as
			// before ADR 0101, and so is one whose call this program could not
			// make (rung.Inline).
			if nat && natCalled[u.fn] && !PlantInlineAll && !rung.Inline[u.fn] {
				name, e := natCallee(tg, u.fn)
				if e == nil {
					count++
					out = append(out, Stmt{Op: OCall, Name: name, Args: u.args, Res: s.Res, Src: s.Src})
					continue
				}
				if PlantNoInlineFallback {
					err = e
					return
				}
			}
			c, e := callee(u.fn)
			if e != nil {
				err = e
				return
			}
			count++
			if u.fn == "to-host" {
				// the rendering: the instance converts, and decimal renders
				t := V(len(f.Types))
				f.Types = append(f.Types, core.BigType)
				out = inlineInto(f, out, c, u.args, []V{t}, ren)
				s.Args = []V{t}
				out = append(out, *s)
				continue
			}
			out = inlineInto(f, out, c, u.args, s.Res, ren)
		}
		r.Stmts = out
		if r.T == TBranch {
			region(r.Then)
			region(r.Else)
		}
	}
	region(f.Body)
	if err != nil {
		return 0, err
	}
	renameAway(f, ren)
	dropDeadWidenings(f)
	Canonicalize(f)
	f.facts = nil
	return count, nil
}

func isBigName(name string) bool {
	if name == "big-of" || name == "big%-small" || name == "decimal" {
		return true
	}
	for _, x := range bigArith {
		if x == name {
			return true
		}
	}
	for _, x := range bigCmp {
		if x == name {
			return true
		}
	}
	return false
}

// inlineInto appends a copy of callee's body to out, its parameters bound to
// args and its results to res, and returns out. Every other value of the callee
// is a fresh value of f. A body ending in a yield hands its values over by
// renaming (ren); one ending in a branch becomes an `if` whose arms are the
// branch's, since an `if`'s arms are regions whose exits are yields through
// branches (spec §1.3).
func inlineInto(f *Func, out []Stmt, callee *Func, args, res []V, ren map[V]V) []Stmt {
	m := make([]V, callee.NV())
	for i := range m {
		m[i] = -1
	}
	for j, p := range callee.Params {
		m[p] = args[j]
	}
	for v := range m {
		if m[v] < 0 {
			m[v] = V(len(f.Types))
			f.Types = append(f.Types, callee.Types[v])
		}
	}
	mv := func(v V) V {
		if v < 0 {
			return v
		}
		return m[v]
	}
	body := copyRegion(callee.Body, mv)
	out = append(out, body.Stmts...)
	switch body.T {
	case TYield:
		for j, y := range body.Args {
			ren[res[j]] = y
		}
	case TBranch:
		body.Stmts = nil
		out = append(out, Stmt{Op: OIf, Args: []V{body.Cond}, Res: res, Sub: []*Region{body.Then, body.Else}})
	case TUnreachable:
		// A BODY INTO ∅ (ADR 0090) never returns: its last statement ends the
		// region it is inlined into, which is unreachable from here on.
	}
	return out
}

// copyRegion is a deep copy of r with every value mapped by m.
func copyRegion(r *Region, m func(V) V) *Region {
	if r == nil {
		return nil
	}
	c := cloneRegion(r)
	var walk func(x *Region)
	walk = func(x *Region) {
		for i := range x.Params {
			x.Params[i] = m(x.Params[i])
		}
		for i := range x.Pis {
			x.Pis[i].V, x.Pis[i].Of, x.Pis[i].Other = m(x.Pis[i].V), m(x.Pis[i].Of), m(x.Pis[i].Other)
		}
		for i := range x.Stmts {
			s := &x.Stmts[i]
			for j := range s.Args {
				s.Args[j] = m(s.Args[j])
			}
			for j := range s.Res {
				s.Res[j] = m(s.Res[j])
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
		for j := range x.Args {
			x.Args[j] = m(x.Args[j])
		}
		if x.T == TBranch {
			x.Cond = m(x.Cond)
			walk(x.Then)
			walk(x.Else)
		}
	}
	walk(c)
	return c
}

// dropDeadWidenings removes a `big-of` nothing reads: the widening a one-pass
// instance took its word from instead.
func dropDeadWidenings(f *Func) {
	uses := useCounts(f)
	f.Walk(func(r *Region) {
		out := r.Stmts[:0]
		for _, s := range r.Stmts {
			if s.Op == OCall && s.Name == "big-of" && len(s.Res) == 1 && uses[s.Res[0]] == 0 {
				continue
			}
			out = append(out, s)
		}
		r.Stmts = out
	})
}

// SelectRung chooses the rung above the word for f, a decided function, as the
// program's plan says: the host's bignum, or fixed limbs. On limbs, an
// operation with no instance takes the host's bignum for the program where the
// target has one (it holds it from here: tg.BigRepr), and is refused by name
// where it has none.
func SelectRung(tg *emit.Target, f *Func, sig *core.Sig, plan emit.BigPlan) (int, error) {
	if plan.Host {
		if _, err := SelectBig(tg, f, sig, plan.Bits, plan.Signed); err != nil {
			return 0, err
		}
		return BigOps(f), nil
	}
	keep := f.Clone()
	keep.facts = f.facts
	if _, err := SelectBig(tg, f, sig, 0, false); err != nil {
		return 0, err
	}
	n, err := LowerLimbs(tg, f, plan)
	if !errors.Is(err, ErrNoLimbForm) {
		return n, err
	}
	// NO HOST BIGNUM: K(ℕ*), the general representation, takes the program
	// whole, as a host's bignum would (ADR 0099). V_n's carrier lies inside
	// K(ℕ*)'s and both are injective into ℤ, so the move changes no answer; the
	// program's one set is enforced there by its sign and bit length.
	if !tg.HasBig() && !PlantNoNatFallback {
		*f = *keep
		if _, err := SelectBig(tg, f, sig, plan.Bits, plan.Signed); err != nil {
			return 0, err
		}
		return LowerLimbs(tg, f, emit.BigPlan{Var: true, Bits: plan.Bits, Signed: plan.Signed, Inline: plan.Inline})
	}
	if !tg.HasBig() {
		return 0, fmt.Errorf("this program needs %s.\n"+
			"  Target %s stores arbitrary precision as fixed limbs (`repr big limbs`) and the built-in\n"+
			"  limb library does not implement that; it declares no bignum of its own to\n"+
			"  fall back to either. What it does have is addition, subtraction,\n"+
			"  multiplication, the comparisons, and division and remainder by a machine\n"+
			"  word (emit/bignum.oro)", strings.TrimPrefix(err.Error(), ErrNoLimbForm.Error()+": "), tg.Name)
	}
	*f = *keep
	tg.BigRepr = "host"
	if _, err := SelectBig(tg, f, sig, plan.Bits, plan.Signed); err != nil {
		return 0, err
	}
	return BigOps(f), nil
}

// CheckClaim is the representation half of a signature's claim, for
// emit.CheckSignatures: the definition, lowered on its own and decided, has
// its values held exactly chosen (SelectBig), which refuses a value held
// exactly where the claim declares a word. That is all a claim says. Whether
// the target's rung has a form for each operation is the program's question,
// asked where the definition is used, and so is not asked here: windows has no
// limb form for a quotient by 2³², and a helper that divides by it is refused
// only by a program that calls it there.
func CheckClaim(tg *emit.Target, all []*core.Sig) func(string, *core.Sig, *core.Term) error {
	return func(name string, sig *core.Sig, nf *core.Term) error {
		// THE RESULT'S CLAIM, decided at the definition (ADR 0072): the
		// header is f ∈ Π(x ∈ P). R, so the obligation is ∀x ∈ P. ⟦B⟧(x) ∈ R,
		// the export's postcondition (postconditions.md §2), whether or not
		// the definition is exported or called.
		if err := checkResultClaim(tg, name, sig, nf); err != nil {
			return err
		}
		// A DEFINITION OVER AN APPLIED PARAMETER, a function or a static
		// tuple (a tuple is eliminated by applying it), has no values until it
		// is instantiated: what it holds, and in what representation, is each
		// instance's, decided where it is inlined, as its result claim is
		// (ADR 0072). Lowered on its own, its parameter is an opaque value and
		// the plan is of nothing the program runs (lib/bytes' codecs).
		if _, ok := appliedParam(nf); ok {
			return nil
		}
		plan, err := emit.PlanBig(tg, sig, nf, all...)
		// A RESULT CLAIMED IN ℤ ALONE claims nothing a representation can
		// fail: ∀x ∈ P. ⟦B⟧(x) ∈ ℤ holds, and which yields are held exactly is
		// each instance's, chosen where it is inlined or entered (ADR 0147)
		if err != nil || !plan.Holds() || plan.ResultsOnly {
			return err
		}
		nf = emit.EraseWordAscriptions(tg.Word, nf)
		// A DEFINITION CHECKED ALONE holds for every argument its header admits
		// (ADR 0072), so a parameter declared `Int` is ℤ here, as an entry
		// point's is (ADR 0147): `fn add(a ∈ Int, b ∈ Int) (Int) { a + b }` sums
		// two integers of any size, and lowered over words its sum was refused
		if !core.PlantZAscribedAsWord {
			c := *sig
			c.Params = append([]core.SigParam(nil), sig.Params...)
			for i, p := range c.Params {
				if p.Type == "int" {
					c.Params[i].Type = core.ZRange
				}
			}
			sig = &c
		}
		lsig, _ := lentSig(sig)
		f, err := Lower(tg, name, lsig, nf, Options{Decided: true})
		if err != nil {
			return err
		}
		Decide(tg, f, false)
		_, err = SelectBig(tg, f, sig, 0, false)
		return err
	}
}

// lentSig is the header of a borrowing definition's body as lowered: the lent
// buffers' versions, then the declared result (spec/local.md §2.1), and how many
// buffers lead. A definition that lends nothing is its own header.
func lentSig(sig *core.Sig) (*core.Sig, int) {
	var bufs []string
	for _, p := range sig.Params {
		if core.IsBuffer(p.Type) {
			bufs = append(bufs, p.Type)
		}
	}
	if len(bufs) == 0 {
		return sig, 0
	}
	// IDEMPOTENT: a header whose results already lead with its buffers is one
	if len(sig.Results) >= len(bufs) {
		lead := true
		for j, b := range bufs {
			if sig.Results[j] != b {
				lead = false
			}
		}
		if lead {
			return sig, len(bufs)
		}
	}
	s := *sig
	res := s.Results
	if len(res) == 0 && s.Result != "" {
		res = []string{s.Result}
	}
	s.Results, s.Result = append(bufs, res...), ""
	return &s, len(bufs)
}

// checkResultClaim decides a definition's postcondition, its result's set
// included (the reader makes `(R)` the conjunct `lo <= result && result <= hi`
// of sig.Ensures), against the definition's own body: lowered with each
// parameter at its declared set and the `where` assumed at entry, as an export
// is (assumeWhere), so what is decided is P ⟹ Q. Every conjunct is decided,
// above the word too: the reader makes one of each end of the result's set
// inside the word, and an end past it is ADR 0029's, enforced at run time
// (resultclaim-2026-11-02). The body is decided before any representation is
// chosen, so the facts are ℤ's.
//
// A contract mark inside the body is its value here (Lower): the obligations of
// the calls it makes are decided where it is inlined, with its arguments.
func checkResultClaim(tg *emit.Target, name string, sig *core.Sig, nf *core.Term) error {
	if sig == nil {
		return nil
	}
	// A CLAIM THE INTERVALS DO NOT STATE, a formula or a relation to the
	// parameters, is the logic's (ADR 0145, stage 2): decided on the residual,
	// the result's range, if any, still decided here.
	if emit.FormulaClaim(tg, sig) {
		if err := emit.DecideClaim(tg, name, sig, nf); err != nil {
			return err
		}
		s := *sig
		s.Ensures = sig.ResultRange
		sig = &s
	}
	ensures := sig.Ensures != nil
	// A TUPLE'S SET IS A PRODUCT, R₁ × … × Rₙ, and a tuple is in it iff each
	// component is in its factor: the reader keeps a tuple's factors as
	// sig.Results and makes no postcondition of them, so each is decided here.
	type factor struct {
		j      int
		ty     string
		lo, hi *big.Int
	}
	var factors []factor
	if len(sig.Results) > 1 {
		for j, ty := range sig.Results {
			if tg.ValueType(ty) == core.BigType {
				continue
			}
			if lo, hi, ok := core.IntRangeBig(ty); ok {
				factors = append(factors, factor{j, ty, lo, hi})
			}
		}
	}
	// A RESULT IN THE UNSIGNED WORD is a claim too. The reader makes a
	// postcondition only of int64 ends, since it reads before any target is
	// known; on a target that holds the set in a word (U on Go), the IR's
	// interval domain is exact there, and the set is decided like any other.
	// Above every word it is ADR 0029's representation, enforced there.
	_, _, expressed := core.IntRange(sig.Result) // both ends int64: the reader's conjuncts are the whole range
	if len(sig.Results) <= 1 && !expressed && tg.ValueType(sig.Result) != core.BigType {
		if lo, hi, ok := core.IntRangeBig(sig.Result); ok {
			factors = append(factors, factor{0, sig.Result, lo, hi})
		}
	}
	if !ensures && len(factors) == 0 {
		return nil
	}
	// A DEFINITION OVER A FUNCTION quantifies its contract over that function,
	// and a function has no set yet (the arrow, ADR 0053, is not built): the
	// claim is decidable only per instance, where staging knows the function,
	// which is not built either. Refused with that reason, not with the [-inf,
	// +inf] an opaque application of the parameter would give.
	if p, ok := appliedParam(nf); ok {
		return fmt.Errorf("its declared result is a claim for every function its parameter %s may be, "+
			"and a function has no set to decide it over (ADR 0053, not built); a result claim on a "+
			"definition over a function is decided per instance, which is not built either (ADR 0072). "+
			"Drop the claim, or state it on a first-order definition", core.ShowName(p))
	}
	nf = emit.EraseAscriptionsButClaims(tg.Word, nf)
	fsig := sig
	if nfl, s, k, err := emit.FlattenProducts(tg, sig, nf); err != nil {
		return err
	} else if k > 0 {
		nf, fsig = nfl, s
	}
	// A BORROWING BODY'S VALUE IS ITS LENT BUFFERS' VERSIONS, THEN THE DECLARED
	// RESULT (spec/local.md §2.1): the claim is about the factors after them
	// (ADR 0112's windows ReadAt and readinto).
	lent := 0
	if !ensures {
		fsig, lent = lentSig(fsig)
		for i := range factors {
			factors[i].j += lent
		}
	}
	f, err := Lower(tg, name, fsig, nf, Options{Decided: true})
	if err != nil {
		return fmt.Errorf("its declared result is a claim, decided on its body alone (ADR 0072), "+
			"and the body cannot be lowered on its own: %v", err)
	}
	Decide(tg, f, false)
	hint := "\n  A definition's declared result is a claim about every argument its parameters' sets " +
		"admit (ADR 0072): narrow a parameter's set, state a `where`, or widen the result's set if " +
		"the definition is right outside it"
	// AN UNCLAIMED PARAMETER APPLIED TO ONE VALUE is a table read or a
	// function's application, one term by design (tables.md §3.1); its set is
	// what would tell them apart, and it has none. So a refusal names both.
	if p, ok := unclaimedApplied(sig, nf); ok {
		hint += fmt.Sprintf("; or, if %s is a function, the claim is about every function it may be, "+
			"which no set expresses yet (ADR 0053): decided per instance, which is not built", core.ShowName(p))
	}
	if ensures {
		// the logic decides what the intervals, which forget relations, do not
		// establish, before it is refused (emit.DecideRangeClaim)
		if err := CheckEnsures(tg, f, fsig); err != nil && emit.DecideRangeClaim(tg, name, sig, nf) != nil {
			return fmt.Errorf("%v%s", err, hint)
		}
	}
	for _, c := range factors {
		if r, ok := resultWithin(tg, f, c.j, c.lo, c.hi); !ok {
			what := fmt.Sprintf("component %d of its result", c.j-lent+1)
			if len(sig.Results) <= 1 {
				what = "its result"
			}
			return fmt.Errorf("the body does not establish that %s is in %s; it is %s%s",
				what, core.ShowSet(c.ty), termShow(r), hint)
		}
	}
	return nil
}

// appliedParam reports a parameter of the definition t that its body applies:
// one standing in an operator's place, which a first-order parameter never
// does, since a table read applies a table and a table is data (tables.md
// §3.1). A read of a table parameter is told apart by its arity and by its
// argument being no λ: a static function is applied to one at least once
// where it is handed a continuation, and else to anything; so only a
// parameter applied to a λ, or to several arguments, is taken for a function.
func appliedParam(t *core.Term) (string, bool) {
	if t == nil || t.Kind != core.KFn {
		return "", false
	}
	var found string
	var walk func(k *core.Term, d int)
	walk = func(k *core.Term, d int) {
		if found != "" || k == nil {
			return
		}
		if k.Kind == core.KFn {
			walk(k.Kids[0], d+1)
			return
		}
		if k.Kind == core.KApp && len(k.Kids) > 0 {
			op := k.Kids[0]
			if op.Kind == core.KBound && op.Depth == d && op.Index < len(t.Params) {
				args := k.Kids[1:]
				if len(args) != 1 || args[0].Kind == core.KFn {
					found = t.Params[op.Index]
					return
				}
			}
		}
		for _, c := range k.Kids {
			walk(c, d)
		}
	}
	walk(t.Kids[0], 0)
	return found, found != ""
}

// unclaimedApplied reports a parameter with no declared set that t's body
// applies: a table read, or a function's application.
func unclaimedApplied(sig *core.Sig, t *core.Term) (string, bool) {
	if t == nil || t.Kind != core.KFn {
		return "", false
	}
	var found string
	var walk func(k *core.Term, d int)
	walk = func(k *core.Term, d int) {
		if found != "" || k == nil {
			return
		}
		if k.Kind == core.KFn {
			walk(k.Kids[0], d+1)
			return
		}
		if k.Kind == core.KApp && len(k.Kids) > 0 {
			if op := k.Kids[0]; op.Kind == core.KBound && op.Depth == d && op.Index < len(t.Params) &&
				(op.Index >= len(sig.Params) || sig.Params[op.Index].Type == "" || sig.Params[op.Index].Type == "any") {
				found = t.Params[op.Index]
				return
			}
		}
		for _, c := range k.Kids {
			walk(c, d)
		}
	}
	walk(t.Kids[0], 0)
	return found, found != ""
}
