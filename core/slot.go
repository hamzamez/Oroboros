package core

import (
	"fmt"
	"sort"
	"strings"
)

// A CELL A CALLBACK CAPTURES IS ITS SLOT (ADR 0133 items 4–6, callbacks.md
// §7). The window procedure's state is a cell of the scope that installs it,
// captured by the λ at the class's field. The host calls the λ from inside its
// own calls, re-entrantly, so the state must be what the last activation left
// at every host call in the scope, inside the λ as outside it: Theorem T's
// lending, which is, observably, C's one global read and written in place
// (callbacks-research §3.3). So the cell's storage is that global: a static
// slot, one per captured cell, every read of the cell a load of it and every
// write a store, in program order. No host call need be classified (1.3, option
// 3): any host call may run the λ, and the next read sees what it wrote.
//
// The loader rewrites, before the state translation, so nothing below it sees
// such a cell:
//
//	(#cell e (fn (x) b))   ↦ ((fn (#_) b′) (#sset N e))
//	(set x v)              ↦ (#sset N v)          x read, x.l read: (#sget N), its projection
//	x at a cell parameter  ↦ (#sref N)
//
// and in a definition with a cell parameter p, `fn step(s ∈ S inout, …)` (1.4b):
//
//	p read                 ↦ (#deref p)           (set p v) ↦ (#setref p v)
//
// which β meets with p := (#sref N): (#deref (#sref N)) is (#sget N), and
// (#setref (#sref N) v) is (#sset N v). The reducer expands (#sget N) into the
// record of its leaves' loads and (#sset N v) into each leaf's store, its
// value claimed in the leaf's set (ADR 0048's claim, as a scalar cell's is at a
// host's cell). The slot's set is the cell parameter's set S, so a read is
// known in S and a write is proven in S.

// The slot forms' heads, and CbFnName, a λ at a callback's field marked as
// one: the host applies it, never the program, so it is a value however
// impure its body (pureTerm), substituted where its binder is used once.
const (
	CbFnName   = "#cbfn"
	SGetName   = "#sget"
	SSetName   = "#sset"
	SRefName   = "#sref"
	DerefPName = "#deref-p"
	SetRefName = "#setref"
)

// SlotLeafName is the primitive reading (get) or writing (set) leaf k of slot
// n, `n~k~get` and `n~k~set`.
func SlotLeafName(n string, k int, op string) string { return fmt.Sprintf("%s~%d~%s", n, k, op) }

// PlantSlotUnshared gives the λ at the field a slot of its own, apart from the
// scope's, both starting at the initial value: the state the procedure writes
// is then not the one the scope reads after a host call, the stale state
// Theorem T excludes (TestAWindowProcedureHasAState).
var PlantSlotUnshared bool

// slotCells rewrites p's captured cells and cell parameters (above), and
// records each slot's set in p.Slots.
func slotCells(p *Program) error {
	// THE CELL PARAMETERS of the program's own definitions
	cellParams := map[string][]int{}
	orig := map[string]*Term{}
	for _, q := range p.Order {
		sig := p.Sigs[q]
		if sig == nil || p.Defs[q] == nil || p.Defs[q].Kind != KFn {
			continue
		}
		var at []int
		for i, sp := range sig.Params {
			if IsCell(sp.Type) {
				at = append(at, i)
			}
		}
		if len(at) == 0 {
			continue
		}
		cellParams[q] = at
		d := p.Defs[q]
		if len(d.Params) != len(sig.Params) {
			return fmt.Errorf("%s: a definition with a cell parameter states every parameter", q)
		}
		names := map[string]bool{}
		for _, i := range at {
			names[d.Params[i]] = true
		}
		orig[q] = d
		body := rewriteCellParams(d.Body(), names)
		p.Defs[q] = Fn(d.Params, body)
	}
	if p.Slots == nil {
		p.Slots = map[string]string{}
	}
	for _, q := range p.Order {
		if p.Defs[q] == nil {
			continue
		}
		n := 0
		t, err := slotScopes(p, q, p.Defs[q], cellParams, &n)
		if err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
		p.Defs[q] = t
	}
	// AN ORDINARY CELL AT A CELL PARAMETER (ADR 0133 item 6): the call is the
	// state-passing translation's, f : S × X → S × R. The definition's value
	// variant takes the cell's value, holds it in a local cell, and returns
	// that cell's last value beside its result; the call writes it back.
	variants := map[string]string{}
	for _, q := range append([]string(nil), p.Order...) {
		if p.Defs[q] == nil {
			continue
		}
		t, err := cellCalls(p, p.Defs[q], map[string]bool{}, cellParams, orig, variants)
		if err != nil {
			return fmt.Errorf("%s: %w", q, err)
		}
		p.Defs[q] = t
	}
	return nil
}

// ValueVariantSuffix names a definition's value variant, `f~cellv`.
const ValueVariantSuffix = "~cellv"

// PlantCellWriteBackDropped forgets to write the variant's last value back
// into the caller's cell: a callee's write is then lost
// (TestADefinitionsCellParameterIsTheCallersCell).
var PlantCellWriteBackDropped bool

// cellCalls rewrites each call f(…, x, …) whose argument at a cell parameter
// is a cell x in scope to the value variant's, ((f~cellv … x …) (fn (#r #v…)
// (seq (set x #v) … #r))), and makes the variant.
func cellCalls(p *Program, t *Term, cells map[string]bool, cellParams map[string][]int, orig map[string]*Term, variants map[string]string) (*Term, error) {
	if t == nil || len(t.Kids) == 0 {
		return t, nil
	}
	if isCellScope(t) && t.Kids[0].Name == CellName {
		init, err := cellCalls(p, t.Kids[1], cells, cellParams, orig, variants)
		if err != nil {
			return nil, err
		}
		lam := t.Kids[2]
		inner := map[string]bool{lam.Params[0]: true}
		for k, v := range cells {
			inner[k] = v
		}
		b, err := cellCalls(p, lam.Body(), inner, cellParams, orig, variants)
		if err != nil {
			return nil, err
		}
		return &Term{Kind: KApp, Kids: []*Term{t.Kids[0], init, Fn(lam.Params, b)}}, nil
	}
	if t.Kind == KFn {
		inner := map[string]bool{}
		for k, v := range cells {
			inner[k] = v
		}
		for _, x := range t.Params {
			delete(inner, x)
		}
		b, err := cellCalls(p, t.Body(), inner, cellParams, orig, variants)
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, b), nil
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		x, err := cellCalls(p, k, cells, cellParams, orig, variants)
		if err != nil {
			return nil, err
		}
		kids[i] = x
	}
	if t.Kind != KApp || kids[0].Kind != KName {
		return &Term{Kind: t.Kind, Kids: kids}, nil
	}
	f := kids[0].Name
	at, ok := cellParams[f]
	if !ok {
		return &Term{Kind: KApp, Kids: kids}, nil
	}
	var passed []string
	for _, i := range at {
		if i+1 < len(kids) && kids[i+1].Kind == KName && cells[kids[i+1].Name] {
			passed = append(passed, kids[i+1].Name)
		} else {
			passed = nil
			break
		}
	}
	if len(passed) == 0 {
		return &Term{Kind: KApp, Kids: kids}, nil
	}
	v, err := valueVariant(p, f, at, orig, variants)
	if err != nil {
		return nil, err
	}
	ps := []string{"#cr"}
	for i := range passed {
		ps = append(ps, fmt.Sprintf("#cv%d", i))
	}
	var body *Term = Name("#cr")
	for i := len(passed) - 1; i >= 0; i-- {
		if PlantCellWriteBackDropped {
			continue
		}
		body = App(Fn([]string{"#_"}, body), App(Name("set"), Name(passed[i]), Name(ps[i+1])))
	}
	call := App(Name(v), kids[1:]...)
	return App(call, Fn(ps, body)), nil
}

// valueVariant is f's value variant, made once: each cell parameter s a
// value s#in held in a local cell s, and the result the tuple of f's result
// and each s's last value, claimed in its set.
func valueVariant(p *Program, f string, at []int, orig map[string]*Term, variants map[string]string) (string, error) {
	if v, ok := variants[f]; ok {
		return v, nil
	}
	d, sig := orig[f], p.Sigs[f]
	if d == nil || sig == nil {
		return "", fmt.Errorf("%s has a cell parameter and no definition of the program's", f)
	}
	v := f + ValueVariantSuffix
	params := append([]string(nil), d.Params...)
	vs := *sig
	vs.Params = append([]SigParam(nil), sig.Params...)
	vs.Result, vs.Results, vs.Unstated = "any", nil, true
	vs.Ensures = nil
	rest := []*Term{Name("#cr")}
	for _, i := range at {
		s := d.Params[i]
		set := CellElem(sig.Params[i].Type)
		params[i] = s + "#in"
		vs.Params[i] = SigParam{Name: params[i], Type: set}
		rest = append(rest, &Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(ClaimDef), Str(s), Str(set), Name(s)}})
	}
	body := App(Fn([]string{"#cr"}, tupleOf(rest)), d.Body())
	for k := len(at) - 1; k >= 0; k-- {
		s := d.Params[at[k]]
		body = App(Name(CellName), Name(s+"#in"), Fn([]string{s}, body))
	}
	if vs.Where != nil {
		ren := map[string]*Term{}
		for _, i := range at {
			ren[d.Params[i]] = Name(d.Params[i] + "#in")
		}
		vs.Where = substNames(vs.Where, ren)
	}
	p.Defs[v] = Fn(params, body)
	p.Sigs[v] = &vs
	p.Order = append(p.Order, v)
	variants[f] = v
	return v, nil
}

// substNames replaces each free name of ren in t.
func substNames(t *Term, ren map[string]*Term) *Term {
	if t == nil {
		return nil
	}
	for n, v := range ren {
		t = substFree(t, n, v)
	}
	return t
}

// rewriteCellParams is a body with each cell parameter's reads and writes made
// a reference's: p ↦ (#deref p), p.l ↦ (#get "l" (#deref p)), (set p v) ↦
// (#setref p v).
func rewriteCellParams(t *Term, ps map[string]bool) *Term {
	switch t.Kind {
	case KName:
		if ps[t.Name] {
			return App(Name(DerefPName), t)
		}
		if i := strings.Index(t.Name, "."); i > 0 && ps[t.Name[:i]] {
			r := App(Name(DerefPName), Name(t.Name[:i]))
			for _, l := range strings.Split(t.Name[i+1:], ".") {
				r = App(Name(GetName), Str(l), r)
			}
			return r
		}
		return t
	case KFn:
		inner := map[string]bool{}
		for k, v := range ps {
			inner[k] = v
		}
		for _, x := range t.Params {
			delete(inner, x)
		}
		return Fn(t.Params, rewriteCellParams(t.Body(), inner))
	case KApp:
		if headIs(t, "set") && len(t.Kids) == 3 && t.Kids[1].Kind == KName && ps[t.Kids[1].Name] {
			return App(Name(SetRefName), t.Kids[1], rewriteCellParams(t.Kids[2], ps))
		}
		kids := make([]*Term, len(t.Kids))
		for i, k := range t.Kids {
			kids[i] = rewriteCellParams(k, ps)
		}
		return &Term{Kind: KApp, Kids: kids}
	}
	return t
}

// slotScopes finds each cell scope of t whose cell a λ at a record's field
// captures, and makes the cell a slot.
func slotScopes(p *Program, q string, t *Term, cellParams map[string][]int, n *int) (*Term, error) {
	if t == nil || len(t.Kids) == 0 {
		return t, nil
	}
	if t.Kind == KFn {
		b, err := slotScopes(p, q, t.Body(), cellParams, n)
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, b), nil
	}
	if isCellScope(t) && t.Kids[0].Name == CellName {
		lam := t.Kids[2]
		x := lam.Params[0]
		body := lam.Body()
		if capturedAtField(body, x) {
			set, err := slotSet(p, body, x, cellParams)
			if err != nil {
				return nil, err
			}
			*n++
			name := fmt.Sprintf("%s~slot%d", strings.NewReplacer("/", "_", ".", "_", "-", "_").Replace(q), *n)
			p.Slots[name] = set
			body = fieldLambdas(body, x, func(l *Term) *Term { return App(Name(CbFnName), l) })
			outer := name
			if PlantSlotUnshared {
				outer = name + "x"
				p.Slots[outer] = set
				body = fieldLambdas(body, x, func(l *Term) *Term { return slotRewrite(l, x, name, cellParams) })
			}
			b := slotRewrite(body, x, outer, cellParams)
			b, err = slotScopes(p, q, b, cellParams, n)
			if err != nil {
				return nil, err
			}
			init, err := slotScopes(p, q, t.Kids[1], cellParams, n)
			if err != nil {
				return nil, err
			}
			if PlantSlotUnshared {
				b = App(Fn([]string{"#_"}, b), App(Name(SSetName), Str(outer), App(Name(SGetName), Str(name))))
			}
			return App(Fn([]string{"#_"}, b), App(Name(SSetName), Str(name), init)), nil
		}
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		x, err := slotScopes(p, q, k, cellParams, n)
		if err != nil {
			return nil, err
		}
		kids[i] = x
	}
	return &Term{Kind: t.Kind, Kids: kids}, nil
}

// recordComponents is the index of a record value's first component in t's
// kids, ok false where t is none: a record value, (#rec L e…), or the reader's
// builder, ((fn (#rc…) (#rec L #rc…)) e…), whose arguments are the components.
func recordComponents(t *Term) (int, bool) {
	if t.Kind != KApp || len(t.Kids) < 2 {
		return 0, false
	}
	if t.Kids[0].Kind == KName && t.Kids[0].Name == RecName && len(t.Kids) > 2 {
		return 2, true
	}
	if f := t.Kids[0]; f.Kind == KFn && len(f.Params) == len(t.Kids)-1 {
		if b := f.Kids[0]; b.Kind == KApp && len(b.Kids) > 0 && b.Kids[0].Kind == KName && b.Kids[0].Name == RecName {
			return 1, true
		}
	}
	return 0, false
}

// capturedAtField says x occurs free in a λ that is a component of a record
// value: the λ at a callback's field (ADR 0133 item 4).
func capturedAtField(t *Term, x string) bool {
	if at, ok := recordComponents(t); ok {
		for _, c := range t.Kids[at:] {
			if c.Kind == KFn && mentionsFree(c, x) || isCbFn(c) && mentionsFree(c.Kids[1], x) {
				return true
			}
		}
	}
	if t.Kind == KFn {
		for _, p := range t.Params {
			if p == x {
				return false
			}
		}
		return capturedAtField(t.Body(), x)
	}
	for _, k := range t.Kids {
		if capturedAtField(k, x) {
			return true
		}
	}
	return false
}

// fieldLambdas is t with each λ at a record's field that mentions x, marked
// or not, rewritten by f.
func fieldLambdas(t *Term, x string, f func(*Term) *Term) *Term {
	if at, ok := recordComponents(t); ok {
		kids := append([]*Term(nil), t.Kids...)
		for i, c := range kids[at:] {
			switch {
			case c.Kind == KFn && mentionsFree(c, x):
				kids[i+at] = f(c)
			case isCbFn(c) && mentionsFree(c.Kids[1], x):
				kids[i+at] = App(Name(CbFnName), f(c.Kids[1]))
			default:
				kids[i+at] = fieldLambdas(c, x, f)
			}
		}
		return &Term{Kind: KApp, Kids: kids}
	}
	if t.Kind == KFn {
		return Fn(t.Params, fieldLambdas(t.Body(), x, f))
	}
	if len(t.Kids) == 0 {
		return t
	}
	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		kids[i] = fieldLambdas(k, x, f)
	}
	return &Term{Kind: t.Kind, Kids: kids}
}

// mentionsFree says the λ's body names x, or x.l, free.
func mentionsFree(lam *Term, x string) bool {
	for _, p := range lam.Params {
		if p == x {
			return false
		}
	}
	found := false
	var walk func(t *Term)
	walk = func(t *Term) {
		if found {
			return
		}
		switch t.Kind {
		case KName:
			if t.Name == x || strings.HasPrefix(t.Name, x+".") {
				found = true
			}
		case KFn:
			for _, p := range t.Params {
				if p == x {
					return
				}
			}
			walk(t.Body())
		default:
			for _, k := range t.Kids {
				walk(k)
			}
		}
	}
	walk(lam.Body())
	return found
}

// slotSet is the set of the cell parameters x is passed at: the slot's set,
// one for every such parameter, or a refusal naming the step's form.
func slotSet(p *Program, t *Term, x string, cellParams map[string][]int) (string, error) {
	sets := map[string]bool{}
	var walk func(t *Term)
	walk = func(t *Term) {
		if t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName {
			if at, ok := cellParams[resolveDefName(p, t.Kids[0].Name)]; ok {
				sig := p.Sigs[resolveDefName(p, t.Kids[0].Name)]
				for _, i := range at {
					if i+1 < len(t.Kids) && t.Kids[i+1].Kind == KName && t.Kids[i+1].Name == x {
						sets[CellElem(sig.Params[i].Type)] = true
					}
				}
			}
		}
		if t.Kind == KFn {
			walk(t.Body())
			return
		}
		for _, k := range t.Kids {
			walk(k)
		}
	}
	walk(t)
	if len(sets) == 0 {
		return "", fmt.Errorf("the cell %s, which a λ at a callback's field captures, is passed to no cell "+
			"parameter, so its set is not stated: pass it to the step, `fn step(s ∈ S inout, …)`, whose S is the "+
			"state's set (ADR 0133 item 6)", Spell(x))
	}
	if len(sets) > 1 {
		var ss []string
		for s := range sets {
			ss = append(ss, ShowSet(s))
		}
		sort.Strings(ss)
		return "", fmt.Errorf("the cell %s is passed to cell parameters of different sets, %s: one state has one set",
			Spell(x), strings.Join(ss, " and "))
	}
	for s := range sets {
		return s, nil
	}
	return "", nil
}

// resolveDefName is a call's head as the program's definitions name it.
func resolveDefName(p *Program, n string) string {
	if _, ok := p.Defs[n]; ok {
		return n
	}
	return n
}

// slotRewrite is the scope's body with x made slot n: a write a store, a read
// a load, the cell at a cell parameter its reference.
func slotRewrite(t *Term, x, n string, cellParams map[string][]int) *Term {
	switch t.Kind {
	case KName:
		if t.Name == x {
			return App(Name(SGetName), Str(n))
		}
		if strings.HasPrefix(t.Name, x+".") {
			r := App(Name(SGetName), Str(n))
			for _, l := range strings.Split(t.Name[len(x)+1:], ".") {
				r = App(Name(GetName), Str(l), r)
			}
			return r
		}
		return t
	case KFn:
		for _, p := range t.Params {
			if p == x {
				return t
			}
		}
		return Fn(t.Params, slotRewrite(t.Body(), x, n, cellParams))
	case KApp:
		if headIs(t, "set") && len(t.Kids) == 3 && t.Kids[1].Kind == KName && t.Kids[1].Name == x {
			return App(Name(SSetName), Str(n), slotRewrite(t.Kids[2], x, n, cellParams))
		}
		kids := make([]*Term, len(t.Kids))
		kids[0] = slotRewrite(t.Kids[0], x, n, cellParams)
		at := map[int]bool{}
		if t.Kids[0].Kind == KName {
			for _, i := range cellParams[t.Kids[0].Name] {
				at[i] = true
			}
		}
		for i, k := range t.Kids[1:] {
			if at[i] && k.Kind == KName && k.Name == x {
				kids[i+1] = App(Name(SRefName), Str(n))
				continue
			}
			kids[i+1] = slotRewrite(k, x, n, cellParams)
		}
		return &Term{Kind: KApp, Kids: kids}
	}
	return t
}

// slotStep is the reducer's rule for the slot forms (above), ok false where
// none applies: kept out of normalizeStep, whose frame every step stacks.
//
//go:noinline
func (e *Env) slotStep(op *Term, args []*Term) (*Term, bool, error) {
	switch op.Name {
	case DerefPName:
		if len(args) == 1 && isSRef(args[0]) {
			return App(Name(SGetName), args[0].Kids[1]), true, nil
		}
		if len(args) == 1 && args[0].Kind != KName && args[0].Kind != KBound {
			return nil, true, fmt.Errorf("a definition's cell parameter takes a cell in scope, or a callback's "+
				"captured cell (ADR 0133 item 6); %s is a value, and a write to it would reach nothing",
				ShowTerm(args[0]))
		}
	case SetRefName:
		if len(args) == 2 && isSRef(args[0]) {
			return App(Name(SSetName), args[0].Kids[1], args[1]), true, nil
		}
		if len(args) == 2 && args[0].Kind != KName && args[0].Kind != KBound {
			return nil, true, fmt.Errorf("a definition's cell parameter takes a cell in scope (ADR 0133 item 6); "+
				"%s is a value", ShowTerm(args[0]))
		}
	case SGetName:
		if len(args) == 1 && args[0].Kind == KStr {
			ty, ok := e.Slots[args[0].Str]
			if !ok {
				return nil, true, fmt.Errorf("slot %s has no set", args[0].Str)
			}
			k := 0
			return slotRead(args[0].Str, ty, &k), true, nil
		}
	case SSetName:
		if len(args) == 2 && args[0].Kind == KStr {
			n := args[0].Str
			ty, ok := e.Slots[n]
			if !ok {
				return nil, true, fmt.Errorf("slot %s has no set", n)
			}
			// let t = v in each leaf stored, in canonical order, claimed in its set
			var stores []*Term
			if !IsRecord(ty) {
				stores = append(stores, App(Name(SlotLeafName(n, 0, "set")), slotClaim(n, ty, Name("#sv"))))
			} else {
				for k, lf := range recordLeaves("", ty, nil) {
					g := Name("#sv")
					for _, l := range lf.path {
						g = App(Name(GetName), Str(l), g)
					}
					stores = append(stores, App(Name(SlotLeafName(n, k, "set")), slotClaim(n+"#"+strings.Join(lf.path, "."), lf.ty, g)))
				}
			}
			body := tupleOf(nil) // the unit, a write's value
			for i := len(stores) - 1; i >= 0; i-- {
				body = App(Fn([]string{"#_"}, body), stores[i])
			}
			return App(Fn([]string{"#sv"}, body), args[1]), true, nil
		}
	}
	return nil, false, nil
}

// isCbFn says t is a λ marked as a callback's, (#cbfn λ).
func isCbFn(t *Term) bool {
	return t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == CbFnName && t.Kids[1].Kind == KFn
}

func isSRef(t *Term) bool {
	return t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == SRefName && t.Kids[1].Kind == KStr
}

// slotRead is slot n's value: its leaves' loads, the record of them where its
// set is a record.
func slotRead(n, ty string, k *int) *Term {
	if !IsRecord(ty) {
		r := App(Name(SlotLeafName(n, *k, "get")))
		*k++
		return r
	}
	labels, types := RecordFields(ty)
	kids := []*Term{Name(RecName), Str(strings.Join(labels, " "))}
	for _, ct := range types {
		kids = append(kids, slotRead(n, ct, k))
	}
	return &Term{Kind: KApp, Kids: kids}
}

// slotClaim is v claimed in the leaf's set, decided where it is written.
func slotClaim(name, ty string, v *Term) *Term {
	return &Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(ClaimDef), Str(name), Str(ty), v}}
}

// SlotLeaves are slot n's leaves' sets, canonical order: what each leaf's
// primitive reads and writes.
func SlotLeaves(ty string) []string {
	if !IsRecord(ty) {
		return []string{ty}
	}
	return RecordLeafTypes(ty)
}
