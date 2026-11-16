package ir

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// A PROGRAM'S DEFINITIONS KEPT AS FUNCTIONS (ADR 0113, ADR 0102's P8).
//
// Keeping a definition is an extension by definitions, so no answer changes
// (Theorem 1), and a definition is kept only where its contract proves it, so
// no verdict does (P4): one its contract cannot prove, or whose call a program
// cannot make, is inlined, which is how every definition was compiled before.
// What keeping buys is Proposition A's and B's: inlined, a sequence of calls
// nests each continuation at the previous body's tails, so depth adds, and a
// sum's consumer is copied into every constructor site; kept, depth takes a
// maximum and the consumer is written once (p8-research.md §2).
//
// K is the compiler's, per program and target: the source names nothing.

// KeepThreshold is P2's T: a definition pays when |f| · (π(f) − 1) reaches it,
// |f| its body's nodes and π(f) its copies inlined (p8-research.md §4, Q4: a
// measured starting point, not a constant of the design).
var KeepThreshold = 64

// PlantKeepNothing makes K empty on every target: the witness that a program
// compiled with K = ∅ is today's.
var PlantKeepNothing bool

// PrintsFunctions says the target's printer defines and calls a program
// function (ADR 0113 item 2): LLVM's, since ADR 0101. K is ∅ elsewhere, so Go's,
// JavaScript's and the JVM's text is unchanged until their printers learn it.
// ORO_KEEP_NONE=1 makes K ∅ here too: the ledger's other build of one commit.
func PrintsFunctions(tg *emit.Target) bool {
	b, err := tg.ResolveBackend()
	return err == nil && b == "llvm" && !PlantKeepNothing && os.Getenv("ORO_KEEP_NONE") == ""
}

// KeepDrop is a kept definition that cannot be kept: its contract does not
// prove it, a call of it is outside its contract, or a call passes it a
// static argument. The driver drops it from K and builds again (P4).
type KeepDrop struct {
	Name string
	Why  error
}

func (e *KeepDrop) Error() string { return fmt.Sprintf("%s is inlined: %v", e.Name, e.Why) }

// keptOn are the definitions registered as program functions on a target, so
// a rebuild unregisters them first.
var keptOn = map[*emit.Target][]string{}

// KeptCount is a kept definition's decision: its operations and loops,
// proven once for the program (ADR 0113), so a driver reports them once, as
// it reports an entry's, and a count of the program's proofs keeps them.
type KeptCount struct {
	Name string
	*Legality
}

var keptLeg = map[*emit.Target][]KeptCount{}

// KeptCounts are the decisions of the definitions the last Keep on tg kept,
// callees first.
func KeptCounts(tg *emit.Target) []KeptCount { return keptLeg[tg] }

// LastKept are the definitions the most recent Keep kept, on any target: what
// a test of the choice reads.
func LastKept() map[string]bool {
	out := map[string]bool{}
	for _, kc := range lastKept {
		out[kc.Name] = true
	}
	return out
}

var lastKept []KeptCount

// Keep chooses K for a program's entries on tg, builds each kept definition
// once (callees first), registers it as a program function, and marks it kept
// in env, so the entries' reduction calls it. drop is what earlier attempts
// found cannot be kept. A *KeepDrop error names one more; the driver adds it
// to drop and calls Keep again.
func Keep(tg *emit.Target, prog *core.Program, env *core.Env, reqs *emit.RequireSet,
	entries []string, all []*core.Sig, drop map[string]bool) (map[string]bool, error) {
	for _, n := range keptOn[tg] {
		delete(tg.Prims, n)
		delete(programFuncs, n)
		calleeMu.Lock()
		delete(callees, calleeKey{tg, n, false})
		calleeMu.Unlock()
	}
	keptOn[tg] = nil
	keptLeg[tg] = nil
	lastKept = nil
	for name, w := range liftedWheres[env] {
		if w == nil {
			delete(env.Wheres, name)
		} else {
			env.Wheres[name] = *w
		}
	}
	delete(liftedWheres, env)
	env.Kept = map[string]bool{}
	if !PrintsFunctions(tg) {
		return nil, nil
	}
	K := chooseKept(env, prog, entries, drop)
	if len(K) == 0 {
		return K, nil
	}
	for n := range K {
		env.Kept[n] = true
	}
	for _, n := range calleesFirst(prog, K) {
		f, lent, leg, post, err := buildKept(tg, prog, env, reqs, n, all)
		if err != nil {
			return nil, &KeepDrop{n, err}
		}
		keptLeg[tg] = append(keptLeg[tg], KeptCount{n, leg})
		lastKept = keptLeg[tg]
		calleeMu.Lock()
		callees[calleeKey{tg, f.Name, false}] = f
		calleeMu.Unlock()
		registerProgramFunc(tg, f)
		keptOn[tg] = append(keptOn[tg], f.Name)
		if len(lent) > 0 {
			registerThreaded(tg, n, f, lent)
			keptOn[tg] = append(keptOn[tg], n)
		}
		// ψ, assumed at each call where its `where` held (ADR 0028's Lemma 1;
		// the call's mark decides φ, or the program is refused)
		if p := tg.Prims[n]; post.core != nil {
			p.Names, p.Ensures = post.names, post.core
			tg.Prims[n] = p
		}
		if p := tg.Prims[f.Name]; f.Name != n && post.ir != nil {
			p.Names, p.Ensures = post.names, post.ir
			tg.Prims[f.Name] = p
		}
	}
	return K, nil
}

// chooseKept is P2: each dynamic definition, first-order in a header that
// states every parameter's set (Proposition D, a representation's codomain for
// an unstated one, is step 3), no `where` and no borrowed buffer yet (steps 2–3),
// that pays: |f| · (π(f) − 1) ≥ T.
func chooseKept(env *core.Env, prog *core.Program, entries []string, drop map[string]bool) map[string]bool {
	pi := paths(prog, entries)
	entry := map[string]bool{}
	for _, e := range entries {
		entry[e] = true
	}
	K := map[string]bool{}
	for n, body := range prog.Defs {
		if entry[n] || drop[n] || body == nil || body.Kind != core.KFn || env.Static[n] != 0 || env.Prim[n] {
			continue
		}
		sig := prog.Sigs[n]
		// ANY `where`: a kept call carries it, decided at the call (the
		// reducer's mark, ADR 0028), and the body assumes it
		if sig == nil || len(sig.Params) != len(body.Params) {
			continue
		}
		ok := true
		for _, p := range sig.Params {
			if strings.HasPrefix(p.Type, "fn") {
				ok = false
			}
		}
		if !ok {
			continue
		}
		// WHAT INLINING COSTS is not only the copies, |f| · (π − 1): one copy
		// adds its depth to the chain it is called in (Proposition A) and copies
		// its consumer into each constructor site (B), and the call-graph π
		// undercounts the reducer's copies, which case-of-case makes. So a
		// definition used once pays when its own body reaches T (measured on
		// windows' os, p8-2026-11-15), and LLVM inlines a function called once
		// back (P7).
		if termSize(body)*max(pi[n]-1, 1) >= KeepThreshold && pi[n] >= 1 {
			K[n] = true
		}
	}
	return K
}

// paths is π, the number of paths from the entries to each definition in the
// call graph: a DAG, since no definition recurs (ADR 0014), so π is its
// path-count semiring's sum of products, π(f) = Σ_g π(g) · occ(g, f) in
// callers-first order. It counts the copies inlining makes of f's body.
func paths(prog *core.Program, entries []string) map[string]int {
	occ := map[string]map[string]int{}
	for n, b := range prog.Defs {
		m := map[string]int{}
		countNames(b, prog.Defs, m)
		occ[n] = m
	}
	order := topo(prog, occ)
	pi := map[string]int{}
	for _, e := range entries {
		pi[e] = 1
	}
	for _, g := range order {
		for f, k := range occ[g] {
			pi[f] += pi[g] * k
		}
	}
	return pi
}

func countNames(t *core.Term, defs map[string]*core.Term, into map[string]int) {
	if t == nil {
		return
	}
	if t.Kind == core.KName {
		if _, ok := defs[t.Name]; ok {
			into[t.Name]++
		}
	}
	for _, k := range t.Kids {
		countNames(k, defs, into)
	}
}

// topo is the definitions callers first.
func topo(prog *core.Program, occ map[string]map[string]int) []string {
	indeg := map[string]int{}
	for n := range prog.Defs {
		indeg[n] += 0
		for f := range occ[n] {
			indeg[f]++
		}
	}
	var ready, out []string
	for n, d := range indeg {
		if d == 0 {
			ready = append(ready, n)
		}
	}
	sort.Strings(ready)
	for len(ready) > 0 {
		n := ready[0]
		ready = ready[1:]
		out = append(out, n)
		var next []string
		for f := range occ[n] {
			if indeg[f]--; indeg[f] == 0 {
				next = append(next, f)
			}
		}
		sort.Strings(next)
		ready = append(ready, next...)
	}
	return out
}

// calleesFirst is K in an order where each kept definition comes after every
// kept definition it calls, so a body is lowered with its callees registered.
func calleesFirst(prog *core.Program, K map[string]bool) []string {
	occ := map[string]map[string]int{}
	for n, b := range prog.Defs {
		m := map[string]int{}
		countNames(b, prog.Defs, m)
		occ[n] = m
	}
	order := topo(prog, occ)
	var out []string
	for i := len(order) - 1; i >= 0; i-- {
		if K[order[i]] {
			out = append(out, order[i])
		}
	}
	return out
}

func termSize(t *core.Term) int {
	if t == nil {
		return 0
	}
	n := 1
	for _, k := range t.Kids {
		n += termSize(k)
	}
	return n
}

// buildKept is one kept definition as a function: its body normalized once
// (its kept callees left as calls), its obligations decided, and the pipeline
// an entry takes, under its header as its contract: its parameters' sets
// assumed at its boundary (ADR 0028), every operation proven under them
// (Theorem 2), its result tables born in it (W13).
//
// A DEFINITION THAT LENDS BUFFERS (Proposition C) is B × X → B × R at the source
// (spec/local.md §2.1), and written in place (ADR 0070) the handed-back B is
// the identity on its argument: so its body, whose value is the buffers'
// versions then the result (lentSig), must hand back each parameter's own
// storage, and the function returns R alone. lent are those parameters'
// indices, in order.
func buildKept(tg *emit.Target, prog *core.Program, env *core.Env, reqs *emit.RequireSet,
	n string, all []*core.Sig) (*Func, []int, *Legality, keptPost, error) {
	sig := prog.Sigs[n]
	var lent []int
	for i, p := range sig.Params {
		if core.IsBuffer(p.Type) {
			lent = append(lent, i)
		}
	}
	name := n
	esig := sig
	// NO RESULT CLAIMED (`any`): the results are the body's, whatever their
	// arity, a product included, and with lent buffers the buffers first.
	unstated := (sig.Result == "any" || sig.Result == "") && len(sig.Results) == 0
	if unstated {
		s := *sig
		s.Result = ""
		esig = &s
	}
	if len(lent) > 0 {
		// the function the IR calls is not the threaded name the core binds:
		// `~` is an identifier character of IR_P's text that no J name holds
		// (it is J's bitwise not), so it cannot collide with a definition
		name = n + "~lent"
		if !unstated {
			esig, _ = lentSig(sig)
		}
	}
	nf, err := core.Normalize(prog.Defs[n], env, core.DefaultFuel)
	if err != nil {
		return nil, nil, nil, keptPost{}, err
	}
	nf = emit.EtaTails(tg, nf)
	if err := emit.CheckJoins(tg, nf); err != nil {
		return nil, nil, nil, keptPost{}, err
	}
	esig = wordInstance(tg, esig, nf)
	// THEOREM 4, LIFTING: an obligation the body leaves unproven, over the
	// parameters alone, becomes a conjunct of φ, assumed here and decided at
	// each call (the kept call's where-mark). One over anything else leaves f
	// inlined.
	body := nf
	var f *Func
	var leg *Legality
	for round := 0; ; round++ {
		nf, err = emit.DischargeRequires(reqs, tg, n, esig, body,
			func(x *core.Term) (*core.Term, error) { return DischargeRanges(tg, esig, x) })
		if err == nil {
			if left := core.Residual(nf, env); len(left) > 0 {
				return nil, nil, nil, keptPost{}, fmt.Errorf("not in normal form: %s", strings.Join(left, ", "))
			}
			f, leg, err = Entry(tg, n, name, esig, nf, all, false, nil)
		}
		if err == nil {
			break
		}
		lifted, ok := liftable(env, body, err)
		if !ok || round >= 4 || !sameNames(esig, body) {
			return nil, nil, nil, keptPost{}, err
		}
		esig = lift(env, n, esig, lifted)
	}
	if err := leg.Refusal(n, tg); err != nil {
		return nil, nil, nil, keptPost{}, err
	}
	if err := leg.SizeRefusal(n, tg); err != nil {
		return nil, nil, nil, keptPost{}, err
	}
	if len(lent) > 0 {
		if err := inPlace(tg, f, lent); err != nil {
			return nil, nil, nil, keptPost{}, err
		}
	}
	if err := CheckBornIn(tg, f); err != nil {
		return nil, nil, nil, keptPost{}, err
	}
	resultElems(tg, f, unstated)
	SelectShifts(tg, f)
	return f, lent, leg, derivePost(tg, prog.Sigs[n], nf, f, len(lent)), nil
}

// liftable are the conditions of a refusal of a kept body that are formulas
// over its parameters alone (Theorem 4), or ok false if any is not: a
// condition naming a local, a bound above the word (no term), or a body whose
// parameters are not its header's names.
func liftable(env *core.Env, body *core.Term, err error) ([]*core.Term, bool) {
	var re *emit.RequireError
	if !errors.As(err, &re) || body.Kind != core.KFn {
		return nil, false
	}
	params := map[string]bool{}
	for _, p := range body.Params {
		params[p] = true
	}
	var out []*core.Term
	for _, f := range re.Fails {
		if f.Cond == nil || !overParams(env, f.Cond, params) {
			return nil, false
		}
		out = append(out, core.StripRequires(f.Cond))
	}
	return out, len(out) > 0
}

// sameNames says a body's parameters are its header's names, so a condition
// over the one is a condition over the other.
func sameNames(sig *core.Sig, body *core.Term) bool {
	if len(sig.Params) != len(body.Params) {
		return false
	}
	for i, p := range sig.Params {
		if p.Name != body.Params[i] {
			return false
		}
	}
	return true
}

func overParams(env *core.Env, t *core.Term, params map[string]bool) bool {
	switch t.Kind {
	case core.KName:
		return params[t.Name] || env.Prim[t.Name]
	case core.KFn:
		return false
	}
	for _, k := range t.Kids {
		if !overParams(env, k, params) {
			return false
		}
	}
	return true
}

// liftedWheres are the contracts lifting extended, as they were before, per
// environment: restored when Keep starts, so a definition dropped from K is
// held at an inlined call to its own `where` only, as with K = ∅.
var liftedWheres = map[*core.Env]map[string]*core.WhereContract{}

// lift conjoins conds to f's `where`: in sig, which its body assumes, and in
// env's contract, which each kept call's mark instantiates.
func lift(env *core.Env, n string, sig *core.Sig, conds []*core.Term) *core.Sig {
	c := *sig
	ps := make([]string, len(sig.Params))
	for i, p := range sig.Params {
		ps[i] = p.Name
	}
	w, had := env.Wheres[n]
	if liftedWheres[env] == nil {
		liftedWheres[env] = map[string]*core.WhereContract{}
	}
	if _, saved := liftedWheres[env][n]; !saved {
		if had {
			orig := w
			liftedWheres[env][n] = &orig
		} else {
			liftedWheres[env][n] = nil
		}
	}
	if !had {
		w = core.WhereContract{Params: ps}
	}
	for _, q := range conds {
		c.Where = conjoin(c.Where, q)
		if !PlantLiftUnmarked {
			w.Cond = conjoin(w.Cond, q)
		}
	}
	if w.Cond != nil {
		if env.Wheres == nil {
			env.Wheres = map[string]core.WhereContract{}
		}
		env.Wheres[n] = w
	}
	return &c
}

func conjoin(a, b *core.Term) *core.Term {
	if a == nil {
		return b
	}
	return core.App(core.Name("if"), a, b, core.Bool(false))
}

// PlantLiftUnmarked lifts into the body's assumptions but not into the call's
// mark: a call outside the lifted precondition must then be accepted.
var PlantLiftUnmarked bool

// PROPOSITION D, THE INSTANCE (f, W) (p8-research §9). An integer parameter
// whose declared set exceeds every word the target holds an integer in,
// [−2⁶³, 2⁶⁴ − 1] on a 64-bit triple, is held in the word at a call the facts
// keep there (ADR 0079): ℤ (`Int`), a set above the words, and a parameter
// with no set at all whose use makes it an integer (emit.ParamTypes). Its
// instance's set is S ∩ W, W the signed word, assumed in the body as the
// parameter's type and its premise, and decided at each call by the
// arguments' sets (CheckCallSets): a call above the word fails it, and P4
// inlines f. A set inside the words is left to the facts, which choose S or U.
// Instances for U and the rung above are further keys of ρ⃗, not built.
func wordInstance(tg *emit.Target, sig *core.Sig, nf *core.Term) *core.Sig {
	if PlantNoWordInstance {
		return sig
	}
	used := emit.ParamTypes(tg, nf)
	wlo, whi := big.NewInt(tg.Word.Lo), big.NewInt(tg.Word.Hi)
	ulo, uhi := wlo, new(big.Int).Lsh(big.NewInt(1), 64)
	uhi.Sub(uhi, big.NewInt(1))
	var out *core.Sig
	for i, p := range sig.Params {
		unannotated := p.Type == "" || p.Type == "any"
		isInt := unannotated && i < len(used) && (used[i] == "int" || strings.HasPrefix(used[i], "int "))
		lo, hi, ranged := core.RangeEnds(p.Type)
		if !isInt && !(ranged || p.Type == "int") {
			continue
		}
		if ranged && lo != nil && hi != nil && lo.Cmp(ulo) >= 0 && hi.Cmp(uhi) <= 0 {
			continue // inside the words: the facts choose
		}
		l, h := wlo, whi
		if ranged && lo != nil && lo.Cmp(l) > 0 {
			l = lo
		}
		if ranged && hi != nil && hi.Cmp(h) < 0 {
			h = hi
		}
		if l.Cmp(h) > 0 || p.Name == "" {
			continue
		}
		if out == nil {
			c := *sig
			c.Params = append([]core.SigParam(nil), sig.Params...)
			out = &c
		}
		out.Params[i].Type = fmt.Sprintf("int %s %s", l, h)
		prem := core.App(core.Name("if"), core.App(core.Name("<="), core.Int(l.Int64()), core.Name(p.Name)),
			core.App(core.Name("<="), core.Name(p.Name), core.Int(h.Int64())), core.Bool(false))
		if out.Where == nil {
			out.Where = prem
		} else {
			out.Where = core.App(core.Name("if"), out.Where, prem, core.Bool(false))
		}
	}
	if out == nil {
		return sig
	}
	return out
}

// PlantNoWordInstance leaves an unstated or unbounded integer parameter
// without the word's set: write, slice and print_int are then refused under
// their contracts and inlined (TestAnUnannotatedParameterIsTheWords).
var PlantNoWordInstance bool

// keptPost is a kept definition's derived postcondition: core over the
// threaded view's components (the core's call), ir over the function's own
// results (the IR's call, when they differ), both over names, the body's
// parameters.
type keptPost struct {
	core, ir *core.Term
	names    []string
}

// derivePost is ψ (p8-research §9): the constant length hull the IR proves of
// each table result, and the greatest subset of T the refinement layer decides
// at every return (emit.ProvePostconditions, Houdini). T, over the integer
// parameters x and the table parameters p, is r ≤ x and r ≤ len(p) for an
// integer result, and len(r) = x, len(r) ≤ x, len(r) = len(p), len(r) ≤ len(p)
// for a table result. k is the count of lent buffers, the first components of
// the core's view, whose length is the arrow's theorem and needs no candidate.
func derivePost(tg *emit.Target, sig *core.Sig, nf *core.Term, f *Func, k int) keptPost {
	if nf == nil || nf.Kind != core.KFn || len(nf.Params) != len(f.Params) {
		return keptPost{}
	}
	comps := k + len(f.Results)
	isInt := func(ty string) bool { return ty == "int" || strings.HasPrefix(ty, "int ") }
	isTab := func(ty string) bool { return strings.HasPrefix(ty, "array") || strings.HasPrefix(ty, "buffer") }
	lenOf := func(t *core.Term) *core.Term { return core.App(core.Name("len"), t) }
	var cands []*core.Term
	for c := k; c < comps; c++ {
		rt := f.Results[c-k]
		r := core.Name(emit.PostResult(c))
		for i, pv := range f.Params {
			x := core.Name(nf.Params[i])
			switch pt := f.Types[pv]; {
			case isInt(rt) && isInt(pt):
				cands = append(cands, core.App(core.Name("<="), r, x))
			case isInt(rt) && isTab(pt):
				cands = append(cands, core.App(core.Name("<="), r, lenOf(x)))
			case isTab(rt) && isInt(pt):
				cands = append(cands, core.App(core.Name("="), lenOf(r), x), core.App(core.Name("<="), lenOf(r), x))
			case isTab(rt) && isTab(pt):
				cands = append(cands, core.App(core.Name("="), lenOf(r), lenOf(x)), core.App(core.Name("<="), lenOf(r), lenOf(x)))
			}
		}
	}
	holds := emit.ProvePostconditions(tg, sig, nf, comps, cands)
	// THE CONSTANT LENGTH HULL, the IR's: decided there, so no candidate
	fs := analyse(tg, f)
	for j, rt := range f.Results {
		if !isTab(rt) {
			continue
		}
		hull, any, ok := ivBot, false, true
		f.Walk(func(r *Region) {
			if !ok || r.T != TYield || !returns(f, r) || j >= len(r.Args) {
				return
			}
			x := fs[r.Args[j]]
			if !x.hasLn {
				ok = false
				return
			}
			hull, any = joinIV(hull, x.ln), true
		})
		if !ok || !any || hull.bot || hull.nlo || hull.phi {
			continue
		}
		lo, okl := hull.lo.i64()
		hi, okh := hull.hi.i64()
		if okl && okh {
			r := lenOf(core.Name(emit.PostResult(k + j)))
			cands = append(cands, core.App(core.Name("<="), core.Int(lo), r), core.App(core.Name("<="), r, core.Int(hi)))
			holds = append(holds, true, true)
		}
	}
	one := -1
	if comps == 1 {
		one = 0
	}
	out := keptPost{names: append([]string(nil), nf.Params...), core: emit.Proven(cands, holds, comps, one)}
	if k > 0 && len(f.Results) == 1 {
		out.ir = emit.Proven(cands, holds, comps, k)
	}
	return out
}

// PlantNotInPlace accepts a kept definition's handed-back buffer without asking
// whether it is the parameter's own storage: TestALentBufferIsWrittenInPlace
// must then see the caller read a table the callee did not write.
var PlantNotInPlace bool

// inPlace checks that each of f's first len(lent) results, at every return, is
// its lent parameter's own storage, and drops them: the caller's buffer after
// the call is its argument (ADR 0070's write-borrow).
func inPlace(tg *emit.Target, f *Func, lent []int) error {
	c := newRelCtx(tg, f)
	k := len(lent)
	same := sameStorage(c, f)
	if len(f.Results) < k {
		return fmt.Errorf("its body hands back %d result(s), fewer than the %d buffer(s) it lends", len(f.Results), k)
	}
	var err error
	f.Walk(func(r *Region) {
		if err != nil || r.T != TYield || !returns(f, r) {
			return
		}
		for j := 0; j < k && j < len(r.Args); j++ {
			if !PlantNotInPlace && !same[f.Params[lent[j]]][c.rootOf(r.Args[j])] {
				err = fmt.Errorf("its buffer %d is handed back as storage other than its own, which a write in place cannot be", j+1)
				return
			}
		}
		r.Args = r.Args[k:]
	})
	if err != nil {
		return err
	}
	f.Results = f.Results[k:]
	return nil
}

// sameStorage is, for each table parameter p, the values that are p's own
// storage: the greatest fixed point of S ∋ v iff v is p, or v has sources and
// every one is in S. The sources are relCtx's edges (a loop parameter's initial
// and back-edge values, a join's arms) and a buffer's own steps, each B ⊸ B on
// one storage: a π of it (a renaming with a fact) and a store `set b i x`
// (ADR 0052, a write in place). An in-place host call's buffer result is its
// argument's value already (threadedCall). A table born otherwise, an
// allocation, has no edge back to p and is never in it (ADR 0096's ownership
// is the same construction).
func sameStorage(c *relCtx, f *Func) map[V]map[V]bool {
	edges := map[V][]V{}
	for v, srcs := range c.edges {
		edges[v] = append(edges[v], srcs...)
	}
	buffer := func(v V) bool { return v >= 0 && int(v) < len(f.Types) && strings.HasPrefix(f.Types[v], "buffer") }
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			if buffer(pi.V) {
				edges[pi.V] = append(edges[pi.V], pi.Of)
			}
		}
		for _, s := range r.Stmts {
			if s.Op == OSet && len(s.Res) == 1 && len(s.Args) > 0 && buffer(s.Res[0]) {
				edges[s.Res[0]] = append(edges[s.Res[0]], s.Args[0])
			}
		}
	})
	out := map[V]map[V]bool{}
	for _, p := range f.Params {
		if !c.table(p) && !buffer(p) {
			continue
		}
		S := map[V]bool{p: true}
		for v := range edges {
			S[v] = true
		}
		for changed := true; changed; {
			changed = false
			for v := range S {
				if v == p {
					continue
				}
				srcs := edges[v]
				ok := len(srcs) > 0
				for _, u := range srcs {
					if !S[c.rootOf(u)] {
						ok = false
					}
				}
				if !ok {
					delete(S, v)
					changed = true
				}
			}
		}
		out[p] = S
	}
	return out
}

// registerThreaded is the kept definition's name as the core binds it, the
// threaded view: its buffers handed back, then its results, each buffer the
// argument's own storage, and the call the function's (emit's in-place
// primitive, ADR 0070, for a program function).
func registerThreaded(tg *emit.Target, n string, f *Func, lent []int) {
	h := tg.Prims[f.Name]
	var res []string
	var ips []emit.InPlace
	for j, i := range lent {
		res = append(res, f.Types[f.Params[i]])
		ips = append(ips, emit.InPlace{Result: j, Arg: i})
	}
	res = append(res, f.Results...)
	t := emit.Prim{Name: n, Args: h.Args, Kind: "expr", Pure: h.Pure, InPlace: ips, Host: f.Name}
	if len(res) == 1 {
		t.Result = res[0]
	} else {
		t.Results = res
	}
	tg.Prims[n] = t
}

// resultElems types each table result by the element set its body proves, the
// hull of every returned table's elements (ADR 0101's numeral, generalized; a
// claim decided at the definition, ADR 0072): a caller reads the result
// through its type alone, and inlined it read the body's facts. A result whose
// elements are not bounded keeps its type, which is already its elements' ρ
// (ADR 0106: a table's elements by their hull). unstated says the header
// claims no result (`any`), so the integer results' types are typing's.
func resultElems(tg *emit.Target, f *Func, unstated bool) {
	fs := analyse(tg, f)
	for j, ty := range f.Results {
		if ty == "int" || strings.HasPrefix(ty, "int ") {
			resultHull(tg, f, fs, j, unstated)
			continue
		}
		e := ElemOf(tg, ty)
		if e != "int" {
			continue
		}
		hull, any, ok := ivBot, false, true
		f.Walk(func(r *Region) {
			if !ok || r.T != TYield || !returns(f, r) || j >= len(r.Args) {
				return
			}
			x := fs[r.Args[j]]
			if !x.hasEl {
				ok = false
				return
			}
			hull, any = joinIV(hull, x.el), true
		})
		if !ok || !any || hull.bot {
			continue
		}
		lo, okl := hull.lo.i64()
		hi, okh := hull.hi.i64()
		if okl && okh && !hull.nlo && !hull.phi {
			f.Results[j] = fmt.Sprintf("array int %d %d", lo, hi)
		}
	}
}

// resultHull types an integer result by the hull of its values at every
// return: a claim decided at the definition (ADR 0072), so a caller reads of
// the call what the body proves of it, and not only its header (Theorem 3's
// loss, recovered where the body proves an interval).
//
// A STATED RESULT's set is a claim the definition is held to, so the hull is
// met with it. AN UNSTATED ONE's type is typing's, a representation and not a
// fact: unification gives every value flowing into a result one node, so in
// windows' `open` the constant 0 of its success took `failed`'s 1..2³²−1, and
// a caller that read the type as the call's fact found its success arm empty
// (p8-2026-11-15). So an unstated result is the hull alone, and with no
// bounded hull its representation's codomain (Proposition D): the word's
// range for a value held in the word.
func resultHull(tg *emit.Target, f *Func, fs []fact, j int, unstated bool) {
	if PlantTypingResult {
		unstated = false
	}
	hull, any := ivBot, false
	f.Walk(func(r *Region) {
		if r.T != TYield || !returns(f, r) || j >= len(r.Args) {
			return
		}
		hull, any = joinIV(hull, fs[r.Args[j]].v), true
	})
	dlo, dhi, hasLo, hasHi := core.RangeBounds(f.Results[j])
	codomain := func() {
		if unstated && (!hasLo || dlo >= tg.Word.Lo) && (!hasHi || dhi <= tg.Word.Hi) {
			f.Results[j] = "int"
		}
	}
	if !any || hull.bot || hull.phi || hull.nlo {
		codomain()
		return
	}
	lo, okl := hull.lo.i64()
	hi, okh := hull.hi.i64()
	if !okl || !okh {
		codomain()
		return
	}
	if !unstated {
		if hasLo && dlo > lo {
			lo = dlo
		}
		if hasHi && dhi < hi {
			hi = dhi
		}
	}
	if lo <= hi {
		f.Results[j] = fmt.Sprintf("int %d %d", lo, hi)
	}
}

// PlantTypingResult reads an unstated result's type from typing, met with the
// hull, as before p8-2026-11-15: windows' os, kept, must then go wrong
// (TestASequenceOfKeptCallsTakesTheMaximumDepth).
var PlantTypingResult bool

// KeptCallee says a call failed its contract at a kept definition: the
// definition the driver drops from K (P4).
func KeptCallee(err error, K map[string]bool) (string, bool) {
	var cs *CallSetError
	if errors.As(err, &cs) && K[cs.Callee] {
		return cs.Callee, true
	}
	// AN OBLIGATION OF A KEPT DEFINITION'S CONTRACT that does not follow at a
	// call, its `where` (lifted conjuncts included) or a parameter's range: the
	// refinement layer names the definition, and P4 inlines that one alone
	var re *emit.RequireError
	if errors.As(err, &re) {
		for _, f := range re.Fails {
			if K[f.Def] {
				return f.Def, true
			}
		}
	}
	return "", false
}
