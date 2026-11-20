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

// callbackParams are the positions of p's callback parameters (ADR 0143): a
// parameter whose set is a callback type's claim, `callback Q`.
func callbackParams(p emit.Prim) map[int]bool {
	var cb map[int]bool
	for i, a := range p.Args {
		if strings.HasPrefix(a, core.CallbackPrefix) {
			if cb == nil {
				cb = map[int]bool{}
			}
			cb[i] = true
		}
	}
	return cb
}

// CallbackClaims are the claims of p's callback parameters, in order: the
// claim of the call's region k is CallbackClaims(p)[k].
func CallbackClaims(p emit.Prim) []string {
	var qs []string
	for _, a := range p.Args {
		if strings.HasPrefix(a, core.CallbackPrefix) {
			qs = append(qs, strings.TrimPrefix(a, core.CallbackPrefix))
		}
	}
	return qs
}

// PlantCallbackParamUndeclared leaves a callback region's parameters with no
// fact, as before the region was visited: an unset fact is [0, 0], a proof
// from no premise, which TestACallbackDuringACallIsDecidedUnderItsClaim must
// see accept a division by r − 65.
var PlantCallbackParamUndeclared bool

// CheckCallbackResults decides each λ's result claim (ADR 0143, ADR 0053
// item 4): the claim is Π(x ∈ P). R, and the host assumes the λ answers in R,
// so ∀x ∈ P. ⟦body⟧(x) ∈ R is our obligation, decided as a definition's
// declared result is (ADR 0072), on ℤ's facts under the parameters' sets. A
// result the host receives in a narrower type than ours (Go's `rune` for
// strings.Map) would otherwise be truncated where the claim is not proven.
func CheckCallbackResults(tg *emit.Target, f *Func) error {
	if PlantCallbackResultUnchecked {
		return nil
	}
	fs := f.facts
	if len(fs) != f.NV() {
		fs = analyse(tg, f)
	}
	var err error
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OFn || err != nil {
				continue
			}
			q := strings.TrimPrefix(s.Type, core.CallbackPrefix)
			d, _, ok := tg.CallbackDecl(q)
			if !ok {
				continue
			}
			lo, hi, isRange := core.IntRangeBig(d.Result)
			if !isRange {
				continue
			}
			want := iv{}
			want.lo, _ = fromBig(lo)
			want.hi, _ = fromBig(hi)
			res := ivBot
			for _, y := range exitsOf(s.Sub[0], TYield) {
				if len(y) == 1 {
					res = joinIV(res, fs[y[0]].v)
				}
			}
			if !res.bot && meetIV(res, want) != res {
				who := core.ShowSet(q)
				if s.Src != nil {
					who = core.ShowTerm(s.Src)
				}
				err = fmt.Errorf("%s answers %s, which is not proven inside %s, the claim's result: the host "+
					"receives it as that set (ADR 0053 item 4, ADR 0143). Narrow the λ's result, or guard it",
					who, termShow(res), core.ShowSet(d.Result))
			}
		}
	})
	return err
}

// PlantCallbackResultUnchecked skips CheckCallbackResults: a λ answering
// outside its claim's result is then printed, and the host truncates it.
var PlantCallbackResultUnchecked bool

// CheckCallbackEffects is Theorem D at every λ a host applies during a call
// (ADR 0143): under a host that does not promise the order of its calls, the
// call's answer is the host's only if the λ's body is pure, so an effect in
// it, a host call that prints or a crash, is refused. Under `ordered`, or a
// count of at most one, the activations are a sequence, and an effect's
// history is that sequence's.
func CheckCallbackEffects(tg *emit.Target, f *Func) error {
	var err error
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op != OFn || err != nil {
				continue
			}
			q := strings.TrimPrefix(s.Type, core.CallbackPrefix)
			_, cs, ok := tg.CallbackDecl(q)
			if !ok || cs.InOrder() || PlantCallbackEffectUnchecked {
				continue
			}
			if name, impure := firstEffect(tg, s.Sub[0]); impure {
				who := "the λ"
				if s.Src != nil {
					who = core.ShowTerm(s.Src)
				}
				err = fmt.Errorf("%s calls %s, an effect, and its host does not promise the order or the number "+
					"of its calls (%s is not `ordered`): the effect's history would be the host's, which may "+
					"differ between releases (Theorem D, ADR 0143). Make the λ pure, or pass it to a host whose "+
					"declaration is `ordered`", who, core.ShowName(name), core.ShowSet(q))
			}
		}
	})
	return err
}

// firstEffect is the first impure host call in r or a region inside it.
func firstEffect(tg *emit.Target, r *Region) (string, bool) {
	var name string
	var found func(r *Region) bool
	found = func(r *Region) bool {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OCall {
				if p, ok := tg.Prims[s.Name]; ok && !p.Pure {
					name = s.Name
					return true
				}
			}
			for _, sub := range s.Sub {
				if found(sub) {
					return true
				}
			}
		}
		if r.T == TBranch {
			return found(r.Then) || found(r.Else)
		}
		return false
	}
	return name, found(r)
}

// PlantCallbackEffectUnchecked skips Theorem D's check: a print in a λ under
// an unordered host is then printed, its output the host's order.
var PlantCallbackEffectUnchecked bool
