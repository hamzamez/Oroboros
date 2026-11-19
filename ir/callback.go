package ir

import (
	"fmt"
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// A CALLBACK IS A FUNCTION OF OURS THE HOST CALLS (ADR 0133, llvm.md §6c). The
// reducer lifts a λ at a callback type's field (core.Env.Lifted); here each is
// built once, as a kept definition is (ADR 0113): its parameters assumed in
// the claim's sets, as an export's are (ADR 0028), its result a claim in the
// claim's result set (ADR 0072), every obligation decided under them. It is
// registered as a program function, beside its address, a primitive whose
// value is the wrapper the host calls, at the claim's C convention.

// CScalar is one C argument's or result's type at a callback's boundary: its
// LLVM type and whether C reads it signed.
type CScalar struct {
	LL     string
	Signed bool
}

// CallbackInfo is what ir/llvm prints a lifted function's wrapper from: the
// claim's convention, its C parameters in C's order, each the index of the
// function's parameter it is, and its C result.
type CallbackInfo struct {
	Conv    string
	Order   []int
	CParams []CScalar
	CResult CScalar
}

// WrapperSuffix names the wrapper the host calls, `F~cb~c`.
const WrapperSuffix = "~c"

// addrOf is a lifted function by its address primitive's name, so Callees
// defines the function whose address a program takes.
var addrOf = map[string]string{}

// CallbackSignature is the claim q's C signature on tg, read off the target
// as a prototype's is: what a printer prints a lifted function's wrapper from,
// the function's text naming q (ADR 0038).
func CallbackSignature(tg *emit.Target, q string) (CallbackInfo, error) {
	d, cs, ok := tg.CallbackDecl(q)
	if !ok {
		return CallbackInfo{}, fmt.Errorf("%s is a callback type no header declares", core.ShowSet(q))
	}
	return callbackSig(tg, q, cs, d)
}

// registerCallbackAddr registers f's address, the primitive `f~addr` whose
// value is its wrapper's address (llvm.md §6c).
func registerCallbackAddr(tg *emit.Target, f *Func) {
	n := f.Name
	addr := n + core.AddrSuffix
	calleeMu.Lock()
	addrOf[addr] = f.Name
	calleeMu.Unlock()
	tg.Prims[addr] = emit.Prim{Name: addr, Result: "ptr", Kind: "expr", Pure: true, Program: true,
		Form: fmt.Sprintf("getelementptr i8, ptr %s, i64 0", llvmName(n+WrapperSuffix))}
}

// PlantCallbackCdecl prints every callback at the C convention: on i686 the
// host's stdcall caller then finds its stack unbalanced after the first call
// (TestACallbackIsAFunctionOfOurs).
var PlantCallbackCdecl bool

// BuildCallbacks builds every λ env lifted at a callback's field, once.
func BuildCallbacks(tg *emit.Target, env *core.Env, reqs *emit.RequireSet, all []*core.Sig) error {
	names := make([]string, 0, len(env.Lifted))
	for n := range env.Lifted {
		if _, ok := tg.Prims[n]; !ok {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		l := env.Lifted[n]
		d, cs, ok := tg.CallbackDecl(l.Claim)
		if !ok {
			return fmt.Errorf("%s: %s is a callback type no header declares", n, core.ShowSet(l.Claim))
		}
		if len(l.Lam.Params) != len(d.Params) {
			return fmt.Errorf("%s: a %s takes %d parameter(s)", n, core.ShowSet(l.Claim), len(d.Params))
		}
		sig := &core.Sig{Result: d.Result}
		for i, t := range d.Params {
			sig.Params = append(sig.Params, core.SigParam{Name: l.Lam.Params[i], Type: t})
		}
		if _, err := callbackSig(tg, l.Claim, cs, d); err != nil {
			return fmt.Errorf("%s: %w", n, err)
		}
		f, leg, err := buildCallback(tg, env, reqs, n, sig, l.Lam, all)
		if err != nil {
			return fmt.Errorf("the %s at its field: %w", core.ShowSet(l.Claim), err)
		}
		f.Callback = l.Claim
		keptLeg[tg] = append(keptLeg[tg], KeptCount{n, leg})
		calleeMu.Lock()
		callees[calleeKey{tg, f.Name, false}] = f
		calleeMu.Unlock()
		registerProgramFunc(tg, f)
		registerCallbackAddr(tg, f)
		keptOn[tg] = append(keptOn[tg], f.Name, n+core.AddrSuffix)
	}
	return nil
}

// callbackSig is the claim's C signature: its convention, and each parameter
// and the result a C type, as a prototype's are (ADR 0094, 0126).
func callbackSig(tg *emit.Target, q string, cs emit.CallbackSig, d core.CallbackDecl) (CallbackInfo, error) {
	info := CallbackInfo{Conv: cs.Conv}
	if PlantCallbackCdecl {
		info.Conv = "cdecl"
	}
	if cs.Conv == "" {
		return info, fmt.Errorf("%s has no C signature: its header states `repr %s c(…) stdcall|cdecl` (llvm.md §6c)",
			core.ShowSet(q), core.ShowSet(q))
	}
	if len(cs.Order) != len(cs.Params) {
		return info, fmt.Errorf("%s's C signature lists %d parameter(s), and its claim %d", core.ShowSet(q), len(cs.Order), len(cs.Params))
	}
	seen := map[int]bool{}
	for _, o := range cs.Order {
		i := -1
		for j, p := range cs.Params {
			if p == o {
				i = j
			}
		}
		if i < 0 || seen[i] {
			return info, fmt.Errorf("%s's C signature names %s, which is no parameter of its claim, or twice", core.ShowSet(q), o)
		}
		seen[i] = true
		ll, _, signed, ok := tg.CType(d.Params[i])
		if !ok || ll == "void" {
			return info, fmt.Errorf("%s's parameter %s is %s, no C type (ADR 0126)", core.ShowSet(q), o, core.ShowSet(d.Params[i]))
		}
		info.Order = append(info.Order, i)
		info.CParams = append(info.CParams, CScalar{ll, signed})
	}
	ll, _, signed, ok := tg.CType(d.Result)
	if !ok {
		return info, fmt.Errorf("%s's result is %s, no C type (ADR 0126)", core.ShowSet(q), core.ShowSet(d.Result))
	}
	info.CResult = CScalar{ll, signed}
	return info, nil
}

// buildCallback is buildKept's path for a lifted λ: normalized, its
// obligations discharged under the claim, lowered and decided by ir.Entry,
// and refused as any entry is.
func buildCallback(tg *emit.Target, env *core.Env, reqs *emit.RequireSet, n string, sig *core.Sig,
	lam *core.Term, all []*core.Sig) (*Func, *Legality, error) {
	nf, err := core.Normalize(lam, env, core.DefaultFuel)
	if err != nil {
		return nil, nil, err
	}
	nf = emit.EtaTails(tg, nf)
	if err := emit.CheckJoins(tg, nf); err != nil {
		return nil, nil, err
	}
	nf, err = emit.DischargeRequires(reqs, tg, n, sig, nf,
		func(x *core.Term) (*core.Term, error) { return DischargeRanges(tg, sig, x) })
	if err != nil {
		return nil, nil, err
	}
	if left := core.Residual(nf, env); len(left) > 0 {
		return nil, nil, fmt.Errorf("not in normal form: %s", strings.Join(left, ", "))
	}
	f, leg, err := Entry(tg, n, n, sig, nf, all, false, nil)
	if err != nil {
		return nil, nil, err
	}
	if err := leg.Refusal(n, tg); err != nil {
		return nil, nil, err
	}
	if err := leg.SizeRefusal(n, tg); err != nil {
		return nil, nil, err
	}
	if err := CheckBornIn(tg, f); err != nil {
		return nil, nil, err
	}
	SelectShifts(tg, f)
	return f, leg, nil
}
