package emit

import (
	"fmt"
	"strings"
	"sync"

	"oroboros/core"
)

// THE LIMB LIBRARY AS A THEORY (ADR 0035).
//
// Each definition f of bignum.oro is a lemma about V_n = [0, Bⁿ), B = 2²⁴,
// presented by ι(o) = Σ oᵢ·Bⁱ: under its precondition P, ι(f(ā)) is the
// operation on ι(ā) when the result lies in the program's one set, and f traps
// otherwise (guard). Its typing, its linearity and its refinement obligations
// (every index, every divisor) are properties of the definition under P, not
// of a call: so they are checked ONCE per specialisation (n, lim), here, and a
// use is an INSTANCE whose obligation is P at its arguments.
//
// The only precondition is k ≠ 0, for `div-small` and `rem-small`; at a use it
// is the program's own division obligation, which the refinement layer has
// discharged on the residual before the rung is chosen. Everything else about
// a use (that its word arithmetic stays in the word) is decided at the site by
// the IR, with the site's facts, like any code.

// limbSigs is each definition's type once n and lim are applied: its
// parameters, its result, and whether its second parameter is a divisor.
var limbSigs = map[string]struct {
	params  []string
	result  string
	divisor bool
}{
	"of":        {[]string{"int"}, "array int", false},
	"add":       {[]string{"array int", "array int"}, "array int", false},
	"sub":       {[]string{"array int", "array int"}, "array int", false},
	"mul":       {[]string{"array int", "array int"}, "array int", false},
	"mul-small": {[]string{"array int", "int"}, "array int", false},
	"div-small": {[]string{"array int", "int"}, "array int", true},
	"rem-small": {[]string{"array int", "int"}, "int", true},
	"lt":        {[]string{"array int", "array int"}, "bool", false},
	"le":        {[]string{"array int", "array int"}, "bool", false},
	"gt":        {[]string{"array int", "array int"}, "bool", false},
	"ge":        {[]string{"array int", "array int"}, "bool", false},
	"eq":        {[]string{"array int", "array int"}, "bool", false},
	"to-host":   {[]string{"array int"}, core.BigType, false},
}

// LimbInstance is one definition specialised to (n, lim): a closed function of
// its remaining parameters, with the signature it was checked under.
type LimbInstance struct {
	Name string
	Term *core.Term
	Sig  *core.Sig
}

var limbCache sync.Map // key: target name, fn, n, lim → *LimbInstance or error

// LimbFunc is the library definition fn specialised to width n (limbs) and
// top-limb ceiling lim, checked as a theory: the type checker, linearity and
// the refinement layer, under the precondition.
func LimbFunc(tgt *Target, fn string, n int, lim int64) (*LimbInstance, error) {
	key := fmt.Sprintf("%p|%s|%d|%d", tgt, fn, n, lim)
	if v, ok := limbCache.Load(key); ok {
		if err, bad := v.(error); bad {
			return nil, err
		}
		return v.(*LimbInstance), nil
	}
	inst, err := limbFunc(tgt, fn, n, lim)
	if err != nil {
		limbCache.Store(key, err)
		return nil, err
	}
	limbCache.Store(key, inst)
	return inst, nil
}

func limbFunc(tgt *Target, fn string, n int, lim int64) (*LimbInstance, error) {
	ty, ok := limbSigs[fn]
	if !ok {
		return nil, fmt.Errorf("the limb library has no %s", fn)
	}
	lib, err := loadLimbLib(tgt)
	if err != nil {
		return nil, err
	}
	name := "limb-" + fn
	// THE SPECIALISATION IS AN η-EXPANSION, λx̄. f n lim x̄, normalised: a
	// definition takes all its arguments, and n and lim are literals, so every
	// loop bound and buffer length in the result is a constant.
	def, ok := lib.prog.Defs[limbPrefix+fn]
	if !ok || def.Kind != core.KFn || len(def.Params) != 2+len(ty.params) {
		return nil, fmt.Errorf("the limb library's %s is not a function of %d parameter(s)", fn, 2+len(ty.params))
	}
	xs := def.Params[2:]
	app := []*core.Term{core.Name(limbPrefix + fn), core.Int(int64(n)), core.Int(lim)}
	for _, x := range xs {
		app = append(app, core.Name(x))
	}
	t := core.Fn(xs, &core.Term{Kind: core.KApp, Kids: app})
	nf, err := core.Normalize(t, lib.env, core.DefaultFuel)
	if err != nil {
		return nil, fmt.Errorf("specialising %s: %w", fn, err)
	}
	if nf.Kind != core.KFn || len(nf.Params) != len(ty.params) {
		return nil, fmt.Errorf("specialising %s: not a function of %d parameter(s)", fn, len(ty.params))
	}
	sig := &core.Sig{Result: ty.result}
	for i, p := range nf.Params {
		sig.Params = append(sig.Params, core.SigParam{Name: p, Type: ty.params[i]})
	}
	if ty.divisor {
		sig.Where = core.App(core.Name("!="), core.Name(nf.Params[1]), core.Int(0))
	}
	// THE THEORY IS CHECKED HERE, ONCE. A failure is the library's, and says so.
	fail := func(err error) error {
		return fmt.Errorf("the built-in bignum's %s does not check at %d limbs: %w", fn, n, err)
	}
	if err := Check(tgt, name, nf); err != nil {
		return nil, fail(err)
	}
	if err := CheckAgainstSig(tgt, name, sig, nf); err != nil {
		return nil, fail(err)
	}
	if err := CheckLinear(nf, tgt, sig); err != nil {
		return nil, fail(err)
	}
	if _, err := Refine(tgt, name, sig, nf); err != nil {
		return nil, fail(err)
	}
	return &LimbInstance{Name: strings.TrimPrefix(fn, limbPrefix), Term: nf, Sig: sig}, nil
}

// LimbShape reports how the limb rung takes a host-shaped bignum operation:
// the library definition, and whether its second operand is a machine word
// (the widened word of `big* x (big-of k)`, `big/ x (big-of k)`, or
// `big%-small`'s k). "" means the library has no form for it.
func LimbShape(name string, wordOperand bool) string {
	switch name {
	case "big*":
		if wordOperand {
			return "mul-small"
		}
		return "mul"
	case "big/":
		if wordOperand {
			return "div-small"
		}
		return ""
	case "big%-small":
		return "rem-small"
	case "big%":
		return ""
	case "big-str":
		return "to-host"
	}
	if f, ok := limbOf[name]; ok {
		return strings.TrimPrefix(f, limbPrefix)
	}
	return ""
}

// LimbWidth is the rung's width and top-limb ceiling for a program enforcing
// [0, 2^bits) (ADR 0029): n = ⌈bits/24⌉ limbs, and the top one under
// 2^(bits − 24(n−1)).
func LimbWidth(bits int) (n int, lim int64) {
	n = (bits + limbBits - 1) / limbBits
	return n, limbLimit(bits, n)
}
