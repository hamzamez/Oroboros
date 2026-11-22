package core

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// RECORDS IN THE CORE (ADR 0128, data.md §4).
//
// A record is the dependent product Π_{l∈L} S_l over a static, finite label
// set. Its four forms (jrecord.go) are applications of names no program
// writes, so every pass that does not know them treats them as the free
// applications they are: pure, since none is a primitive or a definition
// (pureName), and copied as any application is.
//
// Labels become positions in two places (data.md §4.7), and this file has both:
//   - the reducer's δ-rules, at a value it can see: the lens laws read left to
//     right, which erase every record built and consumed in one reduction, as
//     a tuple's β erases a tuple;
//   - EraseRecords, on the residual, by each term's label set, computed
//     forward from its values, its declared sets and its loops' initializers.

// PlantProjectionAsImport reads `r.l`, r bound, as an import's member, as
// before ADR 0128: the planted fault TestARecordIsAFunctionOnItsLabels must
// see fail.
var PlantProjectionAsImport bool

// PlantRecordLetWhole binds a record whose components are not all copyable as
// one value, as before: its projections then stay unresolved on the residual.
var PlantRecordLetWhole bool

// PlantPutGetStale answers a projection with the next label's component: the
// fault the projection's and put–get's cases must see.
var PlantPutGetStale bool

// PlantWithWrongLabel overrides the label after the one named, the fault the
// lens laws' case must catch (data.md §9's record-lens).
var PlantWithWrongLabel bool

// recordTypeName is a record set's canonical type, `record{l₁: T₁; …; lₙ: Tₙ}`.
// BRACES AND SEMICOLONS, so that no reader of the other canonical types, which
// split at a depth-zero ", " and read `F(A, B)` as an applied type, can cut a
// record or mistake it for a variant's instance.
func recordTypeName(t *Term) (string, bool) {
	if t == nil || t.Kind != KApp || len(t.Kids) < 3 || t.Kids[0].Kind != KName ||
		t.Kids[0].Name != RecSetName || t.Kids[1].Kind != KStr {
		return "", false
	}
	labels := strings.Fields(t.Kids[1].Str)
	if len(labels) != len(t.Kids)-2 {
		return "", false
	}
	parts := make([]string, len(labels))
	for i, l := range labels {
		ty := TypeName(t.Kids[2+i])
		if ty == "" {
			return "", false
		}
		parts[i] = l + ": " + ty
	}
	return "record{" + strings.Join(parts, "; ") + "}", true
}

// IsRecord reports a record's canonical type.
func IsRecord(ty string) bool {
	return strings.HasPrefix(ty, "record{") && strings.HasSuffix(ty, "}")
}

// RecordFields is a record type's labels and their types, in canonical order.
func RecordFields(ty string) ([]string, []string) {
	if !IsRecord(ty) {
		return nil, nil
	}
	var labels, types []string
	for _, part := range splitFields(ty[len("record{") : len(ty)-1]) {
		i := strings.Index(part, ": ")
		if i < 0 {
			return nil, nil
		}
		labels = append(labels, part[:i])
		types = append(types, part[i+2:])
	}
	return labels, types
}

// splitFields splits a record type's fields at its semicolons of depth zero.
func splitFields(s string) []string {
	var parts []string
	depth, start := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(', '{':
			depth++
		case ')', '}':
			depth--
		case ';':
			if depth == 0 && i+1 < len(s) && s[i+1] == ' ' {
				parts = append(parts, s[start:i])
				start = i + 2
			}
		}
	}
	return append(parts, s[start:])
}

// isRecValue reports a record's value, (#rec "l…" e…), and its labels.
func isRecValue(t *Term) ([]string, bool) {
	if t == nil || t.Kind != KApp || len(t.Kids) < 3 || t.Kids[0].Kind != KName ||
		t.Kids[0].Name != RecName || t.Kids[1].Kind != KStr {
		return nil, false
	}
	labels := strings.Fields(t.Kids[1].Str)
	return labels, len(labels) == len(t.Kids)-2
}

func labelIndex(labels []string, l string) int {
	for i, m := range labels {
		if m == l {
			return i
		}
	}
	return -1
}

// curryRecordArgs is β's redex (λx̄. b) ā with each argument that is a record
// value not copyable as a whole replaced by its components:
// (λ…p…. b)(…⟨e₁, …, eₙ⟩…) = (λ…p#l₁ … p#lₙ…. b[⟨p#l₁, …⟩/p])(…e₁ … eₙ…). The
// components keep their order, so evaluation order is the record's, which the
// reader made the written one.
func curryRecordArgs(op *Term, args []*Term) (*Term, bool) {
	changed := false
	var params []string
	var nargs []*Term
	subs := make([]*Term, len(args))
	for i, a := range args {
		labels, ok := isRecValue(a)
		if !ok || duplicable(a) {
			params = append(params, op.Params[i])
			nargs = append(nargs, a)
			subs[i] = Name(op.Params[i])
			continue
		}
		changed = true
		names := []*Term{Name(RecName), a.Kids[1]}
		for j, l := range labels {
			n := op.Params[i] + "#" + l
			params = append(params, n)
			nargs = append(nargs, a.Kids[2+j])
			names = append(names, Name(n))
		}
		subs[i] = &Term{Kind: KApp, Kids: names}
	}
	if !changed {
		return nil, false
	}
	return App(Fn(params, op.OpenWith(subs)), nargs...), true
}

func outsideProjection(l string, labels []string) error {
	return fmt.Errorf("projection of %s from a record whose labels are %s (data.md §4.2, X3): "+
		"application outside its domain", Spell(l), showLabels(labels))
}

func outsideOverride(l string, labels []string) error {
	return fmt.Errorf("a `with` names %s, and the record's labels are %s (data.md §4.3, X3): an "+
		"override is a function's, on its domain", Spell(l), showLabels(labels))
}

// recordTails reports a term every one of whose tails is a record value of one
// label set L, through what tupleTails sees through (a conditional's arms, a
// let's body, a host call's continuation, a loop's exits, an obligation's
// body): such a term is a product under the bijection L ≅ {0 … n−1}, taken
// apart as one (tuplifyTails). In a loop an `again` has no tail, and a loop
// with no exit has no value. A term that is itself a record value is not one
// (its rules are recordRule's).
func (e *Env) recordTails(t *Term, inLoop bool) (string, bool) {
	const none = "\x00"
	join := func(a, b string, oka, okb bool) (string, bool) {
		switch {
		case !oka || !okb:
			return "", false
		case a == none:
			return b, true
		case b == none || a == b:
			return a, true
		}
		return "", false
	}
	if labels, ok := isRecValue(t); ok {
		return strings.Join(labels, " "), true
	}
	if t.Kind != KApp || len(t.Kids) == 0 {
		return "", false
	}
	if op := t.Kids[0]; op.Kind == KApp && len(op.Kids) > 0 && op.Kids[0].Kind == KName &&
		(e.Prim[op.Kids[0].Name] || e.Kept[op.Kids[0].Name]) && len(t.Kids) == 2 && t.Kids[1].Kind == KFn {
		return e.recordTails(t.Kids[1].Closed(), inLoop)
	}
	op := t.Kids[0]
	if op.Kind != KName {
		return "", false
	}
	switch {
	case op.Name == "again" && inLoop:
		return none, true
	case op.Name == RequireWhereName && len(t.Kids) == 4:
		return e.recordTails(t.Kids[3], inLoop)
	case !e.Prim[op.Name]:
		return "", false
	case op.Name == "if" && len(t.Kids) == 4:
		a, oka := e.recordTails(t.Kids[2], inLoop)
		b, okb := e.recordTails(t.Kids[3], inLoop)
		return join(a, b, oka, okb)
	case op.Name == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return e.recordTails(t.Kids[2].Closed(), inLoop)
	case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
		l, ok := e.recordTails(t.Kids[1].Closed(), true)
		if !ok || l == none {
			return "", false
		}
		return l, true
	}
	return "", false
}

// tuplifyTails is t with each record tail recordTails sees made the tuple of
// its components in canonical order, (fn (#k) (#k c₁ … cₙ)): the bijection
// applied where the record is made. Under a binder the body is opened by name
// and closed again, so no index moves.
func (e *Env) tuplifyTails(t *Term, inLoop bool) *Term {
	if _, ok := isRecValue(t); ok {
		return Fn([]string{TupleBinder}, App(Name(TupleBinder), t.Kids[2:]...))
	}
	under := func(f *Term, inLoop bool) *Term { return Fn(f.Params, e.tuplifyTails(openNamed(f), inLoop)) }
	if t.Kind != KApp || len(t.Kids) == 0 {
		return t
	}
	kids := append([]*Term{}, t.Kids...)
	if op := t.Kids[0]; op.Kind == KApp && len(t.Kids) == 2 && t.Kids[1].Kind == KFn {
		kids[1] = under(t.Kids[1], inLoop)
		return &Term{Kind: KApp, Kids: kids}
	}
	op := t.Kids[0]
	switch {
	case op.Kind != KName:
	case op.Name == RequireWhereName && len(t.Kids) == 4:
		kids[3] = e.tuplifyTails(t.Kids[3], inLoop)
	case op.Name == "if" && len(t.Kids) == 4:
		kids[2], kids[3] = e.tuplifyTails(t.Kids[2], inLoop), e.tuplifyTails(t.Kids[3], inLoop)
	case op.Name == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		kids[2] = under(t.Kids[2], inLoop)
	case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
		kids[1] = under(t.Kids[1], true)
	}
	return &Term{Kind: KApp, Kids: kids}
}

// takeApart is `let q = v in b` for a v whose tails are records of the labels
// L (recordTails), the product's η: v made a tuple, applied to the
// continuation that binds its components and reads q as the record of them,
// (tuplify(v) (fn (q#l₁ … q#lₙ) b[⟨q#l₁, …⟩/q])). Every record is so a record of
// names wherever it is bound, which the projection rule erases.
func (e *Env) takeApart(v *Term, labels string, k *Term) *Term {
	ls := strings.Fields(labels)
	names := []*Term{Name(RecName), Str(labels)}
	cs := make([]string, len(ls))
	for i, l := range ls {
		cs[i] = k.Params[0] + "#" + l
		names = append(names, Name(cs[i]))
	}
	return App(e.tuplifyTails(v, false), Fn(cs, k.OpenWith([]*Term{{Kind: KApp, Kids: names}})))
}

// curryLoop is a loop whose variables include records, (loop (fn (… p …) b) …
// ⟨e₁, …⟩ …), made a loop over their components: p is the record of its
// components' names in the body, and each `again` passes R's components where
// it passed R, R bound first so its effects run once, in place. The currying
// isomorphism at a loop's binder, as at β's (curryRecordArgs).
func (e *Env) curryLoop(op *Term, args []*Term) (*Term, bool) {
	f := args[0]
	if f.Kind != KFn || len(f.Params) != len(args)-1 {
		return nil, false
	}
	var params []string
	var inits []*Term
	subs := make([]*Term, len(f.Params))
	rec := make([][]string, len(f.Params))
	for i, z := range args[1:] {
		labels, ok := isRecValue(z)
		if !ok {
			params = append(params, f.Params[i])
			inits = append(inits, z)
			subs[i] = Name(f.Params[i])
			continue
		}
		rec[i] = labels
		names := []*Term{Name(RecName), z.Kids[1]}
		for j, l := range labels {
			n := f.Params[i] + "#" + l
			params = append(params, n)
			inits = append(inits, z.Kids[2+j])
			names = append(names, Name(n))
		}
		subs[i] = &Term{Kind: KApp, Kids: names}
	}
	if len(params) == len(f.Params) {
		return nil, false
	}
	k := 0
	var again func(t *Term) *Term
	again = func(t *Term) *Term {
		switch t.Kind {
		case KFn:
			return Fn(t.Params, again(openNamed(t)))
		case KApp:
		default:
			return t
		}
		if h := t.Kids[0]; h.Kind == KName && h.Name == "loop" {
			// a nested loop's `again` is its own: only its initializers are ours
			kids := append([]*Term{h, t.Kids[1]}, nil...)
			for _, z := range t.Kids[2:] {
				kids = append(kids, again(z))
			}
			return &Term{Kind: KApp, Kids: kids}
		}
		kids := make([]*Term, len(t.Kids))
		for i, c := range t.Kids {
			kids[i] = again(c)
		}
		if h := t.Kids[0]; h.Kind != KName || h.Name != "again" || len(t.Kids) != len(f.Params)+1 {
			return &Term{Kind: KApp, Kids: kids}
		}
		out := []*Term{Name("again")}
		var binds []string
		var vals []*Term
		for i, a := range kids[1:] {
			if rec[i] == nil {
				out = append(out, a)
				continue
			}
			k++
			n := fmt.Sprintf("#ag%d", k)
			binds, vals = append(binds, n), append(vals, a)
			for _, l := range rec[i] {
				out = append(out, App(Name(GetName), Str(l), Name(n)))
			}
		}
		body := App(out[0], out[1:]...)
		for i := len(binds) - 1; i >= 0; i-- {
			body = App(Name("let"), vals[i], Fn([]string{binds[i]}, body))
		}
		return body
	}
	body := again(f.OpenWith(subs))
	return App(op, append([]*Term{Fn(params, body)}, inits...)...), true
}

// openNamed opens an abstraction by its parameters' names, lowering every
// index that looked through it, so Fn closes it back to itself: Body, which
// lowers nothing, is for a term with no outer binders, and the reducer's terms
// have them.
func openNamed(f *Term) *Term {
	names := make([]*Term, len(f.Params))
	for i, p := range f.Params {
		names[i] = Name(p)
	}
	return f.OpenWith(names)
}

// isRecordTail reports a residual let whose body, through nested lets, is a
// record value.
func isRecordTail(t *Term) bool {
	for t != nil && t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName && t.Kids[0].Name == "let" &&
		t.Kids[2].Kind == KFn {
		t = t.Kids[2].Closed()
	}
	_, ok := isRecValue(t)
	return ok
}

// isRecForm reports an application of one of the record's forms of arity n.
func isRecForm(t *Term, name string, n int) bool {
	return t != nil && t.Kind == KApp && len(t.Kids) == n+1 && t.Kids[0].Kind == KName &&
		t.Kids[0].Name == name && t.Kids[1].Kind == KStr
}

// showLabels is a label set as J writes it, for a message.
func showLabels(labels []string) string {
	s := make([]string, len(labels))
	for i, l := range labels {
		s[i] = Spell(l)
	}
	return strings.Join(s, ", ")
}

// recordRule is δ on the record's forms (data.md §4.3, §4.7), at a value the
// reducer can see:
//
//	(#get l (#rec L … e_l …))       = e_l                projection
//	(#with l (#rec L … e_l …) x)    = (#rec L … x …)     override
//
// The lens laws follow from these two at every value, so no rule reads through
// an override: put–get is (#get l (#with l v x)) = (#get l v′) = x for the
// value v′ the override rule makes. A rule through an override would fire only
// where its base is unseen, where an override outside the domain could not be
// refused (X3); and every component the reducer meets is a name or pure (the
// reader binds a value's components, currying and a let name them), so the
// override rule always fires where a base is seen. A term the rules discard
// keeps its obligations (keepClaims). Where no value is seen the form stays,
// for EraseRecords.
func (e *Env) recordRule(op *Term, args []*Term, fuel *int) (*Term, bool, error) {
	l := args[0].Str
	r, err := normalize(args[1], e, fuel)
	if err != nil {
		return nil, true, err
	}
	// A RECORD UNDER ITS OBLIGATION is first its components' (splitRecordRequire),
	// so a projection reads a component and keeps the component's own mark.
	if IsRequire(r) && r.Kids[3].Kind == KStr && IsRecord(r.Kids[3].Str) {
		if s, ok, err := e.splitRecordRequire(Name(RequireName), r.Kids[1:], fuel); err != nil {
			return nil, true, err
		} else if ok {
			r = s
		}
	}
	pure := func(ts ...*Term) bool {
		for _, t := range ts {
			if !e.pureTerm(t, map[string]bool{}) {
				return false
			}
		}
		return true
	}
	if op.Name == GetName {
		if labels, ok := isRecValue(r); ok {
			j := labelIndex(labels, l)
			if j < 0 {
				return nil, true, outsideProjection(l, labels)
			}
			others := append(append([]*Term{}, r.Kids[2:2+j]...), r.Kids[3+j:]...)
			if pure(others...) {
				c := r.Kids[2+j]
				if PlantPutGetStale && len(labels) > 1 {
					c = r.Kids[2+(j+1)%len(labels)]
				}
				t, err := normalize(c, e, fuel)
				return e.keepClaims(others, t, err, fuel)
			}
		}
		if labels, ok := e.recordTails(r, false); ok {
			return e.bindRecordTails(op, args, r, labels, fuel)
		}
		return App(op, args[0], r), true, nil
	}
	x, err := normalize(args[2], e, fuel)
	if err != nil {
		return nil, true, err
	}
	if labels, ok := isRecValue(r); ok {
		j := labelIndex(labels, l)
		if j < 0 {
			return nil, true, outsideOverride(l, labels)
		}
		if PlantWithWrongLabel {
			j = (j + 1) % len(labels)
		}
		if pure(r.Kids[2+j]) {
			kids := append([]*Term{}, r.Kids...)
			kids[2+j] = x
			return e.keepClaims(r.Kids[2+j:3+j], &Term{Kind: KApp, Kids: kids}, nil, fuel)
		}
	}
	if labels, ok := e.recordTails(r, false); ok {
		return e.bindRecordTails(op, []*Term{args[0], r, x}, r, labels, fuel)
	}
	return App(op, args[0], r, x), true, nil
}

// bindRecordTails is a projection or an override of a term whose tails are
// records: the term is bound, `(#get l t)` = let q = t in q.l, and the let
// takes it apart (takeApart), so q is a record of names the rule then erases.
func (e *Env) bindRecordTails(op *Term, args []*Term, r *Term, labels string, fuel *int) (*Term, bool, error) {
	if labelIndex(strings.Fields(labels), args[0].Str) < 0 {
		if op.Name == GetName {
			return nil, true, outsideProjection(args[0].Str, strings.Fields(labels))
		}
		return nil, true, outsideOverride(args[0].Str, strings.Fields(labels))
	}
	rest := append([]*Term{op, args[0], Name("#rt")}, args[2:]...)
	k := Fn([]string{"#rt"}, &Term{Kind: KApp, Kids: rest})
	t, err := normalize(e.takeApart(r, labels, k), e, fuel)
	return t, true, err
}

// splitRecordRequire is a record's obligation, (#req d p T v) with T a record
// set, made its components': v ∈ Π_{l∈L} S_l iff v.l ∈ S_l for every l, so it
// is (#rec L (#req d p.l S_l v_l) …), each component marked with its own set
// and decided as any argument's range is. v is seen as a value, whose
// components are its own, or as a pure term, read through its projections
// (get–put: v = record { l : v.l, … }); any other term keeps the mark whole,
// and erasure refuses it if it reaches the residual.
func (e *Env) splitRecordRequire(op *Term, args []*Term, fuel *int) (*Term, bool, error) {
	v, err := normalize(args[3], e, fuel)
	if err != nil {
		return nil, true, err
	}
	labels, types := RecordFields(args[2].Str)
	if labels == nil {
		return nil, false, nil
	}
	var comps []*Term
	if ls, ok := isRecValue(v); ok && strings.Join(ls, " ") == strings.Join(labels, " ") {
		comps = v.Kids[2:]
	} else if e.pureTerm(v, map[string]bool{}) && duplicable(v) {
		for _, l := range labels {
			comps = append(comps, App(Name(GetName), Str(l), v))
		}
	} else {
		return App(op, args[0], args[1], args[2], v), true, nil
	}
	kids := []*Term{Name(RecName), Str(strings.Join(labels, " "))}
	for i, l := range labels {
		if PlantRecordRequireDropped {
			kids = append(kids, comps[i])
			continue
		}
		kids = append(kids, App(Name(RequireName), args[0], Str(args[1].Str+"."+Spell(l)), Str(types[i]), comps[i]))
	}
	t, err := normalize(&Term{Kind: KApp, Kids: kids}, e, fuel)
	return t, true, err
}

// splitRecordClaim is (the record{l: S…} v) for v a record value: its labels
// the claim's, every one given (X2) and none other (X3), and each component
// claimed in its own set, nested records split in turn.
func (e *Env) splitRecordClaim(args []*Term, fuel *int) (*Term, bool, error) {
	v, err := normalize(args[1], e, fuel)
	if err != nil {
		return nil, true, err
	}
	labels, types := RecordFields(args[0].Str)
	have, ok := isRecValue(v)
	if labels == nil || !ok {
		return App(Name(AscribeName), args[0], v), true, nil
	}
	at := map[string]int{}
	for i, l := range have {
		at[l] = i
	}
	want := map[string]bool{}
	kids := []*Term{Name(RecName), Str(strings.Join(labels, " "))}
	for i, l := range labels {
		want[l] = true
		j, ok := at[l]
		if !ok {
			return nil, true, fmt.Errorf("a value of %s gives no %s (data.md §4.2, X2): a record is total on its "+
				"labels; a default is `zero(T) with { … }`", ShowSet(args[0].Str), Spell(l))
		}
		// a range's claim is an obligation, decided where it is written; a
		// record splits again; any other set stays its typing claim
		if _, _, isRange := IntRangeBig(types[i]); isRange && !PlantRecordClaimUnmarked {
			kids = append(kids, &Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(ClaimDef), Str(l), Str(types[i]), v.Kids[2+j]}})
			continue
		}
		kids = append(kids, App(Name(AscribeName), Str(types[i]), v.Kids[2+j]))
	}
	for _, l := range have {
		if !want[l] {
			return nil, true, fmt.Errorf("a value of %s gives %s, which is not among its labels (data.md §4.2, X3)",
				ShowSet(args[0].Str), Spell(l))
		}
	}
	t, err := normalize(&Term{Kind: KApp, Kids: kids}, e, fuel)
	return t, true, err
}

// PlantClaimHidesRecord leaves a claim on a record unsplit, as before
// book-2026-11-20: `p ∈ Point = record { … }` then `p.x` is refused again
// (TestAClaimOnARecordKeepsItsLabels).
var PlantClaimHidesRecord bool

// PlantRecordClaimUnmarked leaves a record claim's ranged components as typing
// claims, as an internal ascription: `p ∈ Pen = record { ink : 300, … }` with
// ink ∈ 0..255 compiled and printed 300 (TestAClaimOnARecordKeepsItsLabels).
var PlantRecordClaimUnmarked bool

// PlantRecordRequireDropped splits a record's obligation into none: the fault
// a record parameter's contract must be seen to refuse.
var PlantRecordRequireDropped bool

// keepClaims is t with the obligations of the terms a rule discarded moved onto
// it: a contract holds of a component whether or not the body reads it
// (ADR 0028), so a projection that drops r.y keeps r.y's mark, as β keeps a
// dead argument's claim (hoistObligations).
func (e *Env) keepClaims(dropped []*Term, t *Term, err error, fuel *int) (*Term, bool, error) {
	if err != nil || PlantRecordClaimDropped {
		return t, true, err
	}
	for _, d := range dropped {
		// every obligation, a contract's as well as a binder's claim: a record
		// parameter's set is checked whole (stricter than β's dead argument, whose
		// contract is owed, CLAUDE.md's "a dead call's contract")
		if containsRequire(d) {
			nd, err := normalize(d, e, fuel)
			if err != nil {
				return nil, true, err
			}
			t = e.hoistObligations(nd, t, fuel)
		}
	}
	return t, true, nil
}

// PlantRecordClaimDropped drops a discarded component's obligation, as the
// first build of the projection rule did.
var PlantRecordClaimDropped bool

// A RECORD IN A HEADER IS CURRIED (ADR 0128 item 7).
//
// The currying isomorphism, Π_{l∈L} S_l → R ≅ S_{l₁} → … → S_{lₙ} → R,
// applied at every definition whose header states a record set, with nested
// records flattened by associativity, A × (B × C) ≅ A × B × C:
//
//   - q keeps its name and becomes the CURRIED definition: its parameters are
//     the leaves of its records, p#x ∈ S_x, its body reads p as the record of
//     those names, and a record result is the product of its leaves. So an
//     export, a kept definition and a claim's lowering (ADR 0072) see only
//     the sets they always have, each leaf's representation chosen by ρ;
//   - q#r, a name no program writes, is the RECORD VIEW, a wrapper that
//     projects its arguments' leaves, calls q, and rebuilds a record result;
//   - every other use of q is a use of q#r, whose inlining is the isomorphism's
//     other half.
//
// Every obligation is then a leaf's range on q (installContracts), which the
// existing deciders decide as any parameter's.
func curryRecords(p *Program, terms []*Term) ([]*Term, error) {
	renamed := map[string]string{}
	var order []string
	for _, q := range p.Order {
		order = append(order, q)
		sig := p.Sigs[q]
		if sig == nil || !headerHasRecord(sig) {
			continue
		}
		body := p.Defs[q]
		if body == nil || body.Kind != KFn || len(body.Params) != len(sig.Params) {
			return nil, fmt.Errorf("%s: its header states a record set, and its body is not a function of "+
				"its %d parameters, so the record cannot be curried (ADR 0128)", ShowName(q), len(sig.Params))
		}
		if sig.Ensures != nil && IsRecord(sig.Result) {
			return nil, fmt.Errorf("%s: an `ensure` over a record result is not built (ADR 0128); state "+
				"the claim on a component's set", ShowName(q))
		}
		var params []string
		var sps []SigParam
		subs := make([]*Term, len(sig.Params))
		args := make([]*Term, 0, len(sig.Params))
		for i, pm := range sig.Params {
			if !IsRecord(pm.Type) {
				params = append(params, body.Params[i])
				sps = append(sps, pm)
				subs[i] = Name(body.Params[i])
				args = append(args, Name(body.Params[i]))
				continue
			}
			for _, lf := range recordLeaves(body.Params[i], pm.Type, nil) {
				params = append(params, lf.name)
				sps = append(sps, SigParam{Name: lf.name, Type: lf.ty})
				args = append(args, lf.read(Name(body.Params[i])))
			}
			subs[i] = recordOfLeaves(body.Params[i], pm.Type)
		}
		cbody := body.OpenWith(subs)
		nsig := *sig
		nsig.Params = sps
		for i, pm := range sig.Params {
			if IsRecord(pm.Type) {
				nsig.Where = substFree(nsig.Where, pm.Name, subs[i])
				nsig.Written = substFree(nsig.Written, pm.Name, subs[i])
				nsig.Ensures = substFree(nsig.Ensures, pm.Name, subs[i])
			}
		}
		var call *Term = App(Name(q), args...)
		if IsRecord(sig.Result) {
			leaves := recordLeaves("#v", sig.Result, nil)
			tys := make([]string, len(leaves))
			reads := []*Term{Name(TupleBinder)}
			outs := make([]string, len(leaves))
			for j, lf := range leaves {
				tys[j] = lf.ty
				reads = append(reads, lf.read(Name("#v")))
				outs[j] = lf.name
			}
			// the body's value, η-expanded into the product of its leaves
			cbody = App(Fn([]string{"#v"}, Fn([]string{TupleBinder}, App(reads[0], reads[1:]...))), cbody)
			if len(tys) == 1 {
				nsig.Result, nsig.Results = tys[0], nil
				call = App(Fn([]string{outs[0]}, recordOfLeaves("#v", sig.Result)), call)
			} else {
				nsig.Result, nsig.Results = "", tys
				call = App(call, Fn(outs, recordOfLeaves("#v", sig.Result)))
			}
			nsig.ResultRange = nil
		}
		p.Defs[q] = Fn(params, cbody)
		p.Sigs[q] = &nsig
		view := q + "#r"
		p.Defs[view] = Fn(body.Params, call)
		order = append(order, view)
		renamed[q] = view
	}
	if len(renamed) == 0 {
		return terms, nil
	}
	p.Order = order
	for _, q := range p.Order {
		if _, own := renamed[q]; own {
			continue // the curried definition calls nothing of its own record view
		}
		if strings.HasSuffix(q, "#r") && renamed[strings.TrimSuffix(q, "#r")] == q {
			continue // the record view calls the curried definition by its name
		}
		p.Defs[q] = renameFree(p.Defs[q], renamed)
	}
	for i, t := range terms {
		terms[i] = renameFree(t, renamed)
	}
	return terms, nil
}

// headerHasRecord reports a header stating a record set, as a parameter's or
// the result's.
func headerHasRecord(sig *Sig) bool {
	if IsRecord(sig.Result) {
		return true
	}
	for _, r := range sig.Results {
		if IsRecord(r) {
			return true
		}
	}
	for _, pm := range sig.Params {
		if IsRecord(pm.Type) {
			return true
		}
	}
	return false
}

// leaf is one leaf of a record set flattened: its name, its set, and its path
// of labels from the record.
type leaf struct {
	name, ty string
	path     []string
}

// read is the leaf read from a record r, r.l₁.l₂…
func (lf leaf) read(r *Term) *Term {
	for _, l := range lf.path {
		r = App(Name(GetName), Str(l), r)
	}
	return r
}

// recordLeaves are a record set's leaves, depth first in canonical order: the
// associativity isomorphism's order, so the flattening is a function of the
// set alone.
func recordLeaves(name, ty string, path []string) []leaf {
	labels, types := RecordFields(ty)
	var out []leaf
	for i, l := range labels {
		p := append(append([]string{}, path...), l)
		n := name + "#" + l
		if IsRecord(types[i]) {
			out = append(out, recordLeaves(n, types[i], p)...)
			continue
		}
		out = append(out, leaf{name: n, ty: types[i], path: p})
	}
	return out
}

// RecordLeafTypes are a record set's leaves' sets, depth first in canonical
// order: what leaf j of a struct's cell holds (llvm.md §6b).
func RecordLeafTypes(ty string) []string {
	var out []string
	for _, lf := range recordLeaves("", ty, nil) {
		out = append(out, lf.ty)
	}
	return out
}

// recordOfLeaves is the record value built from a set's leaf names, the
// isomorphism's inverse.
func recordOfLeaves(name, ty string) *Term {
	labels, types := RecordFields(ty)
	kids := []*Term{Name(RecName), Str(strings.Join(labels, " "))}
	for i, l := range labels {
		n := name + "#" + l
		if IsRecord(types[i]) {
			kids = append(kids, recordOfLeaves(n, types[i]))
			continue
		}
		kids = append(kids, Name(n))
	}
	return &Term{Kind: KApp, Kids: kids}
}

// substFree replaces the free name n by v. Bound variables are indices, so no
// capture is possible, and v's names are free.
func substFree(t *Term, n string, v *Term) *Term {
	if t == nil {
		return nil
	}
	if t.Kind == KName {
		if t.Name == n {
			return v
		}
		return t
	}
	if len(t.Kids) == 0 {
		return t
	}
	c := *t
	c.Kids = make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		c.Kids[i] = substFree(k, n, v)
	}
	return &c
}

// zeroOf is zero(T) for a record set's canonical type (data.md §4.6): every
// component its set's zero, 0 where 0 ∈ S_l, `none` for an Option, the zero of
// a nested record; refused naming the component where one has none (X11).
func zeroOf(ty string) (*Term, error) {
	labels, types := RecordFields(ty)
	if labels == nil {
		return nil, fmt.Errorf("zero(T) is a record set's zero (data.md §4.6), and %s is no record set",
			ShowSet(ty))
	}
	kids := []*Term{Name(RecName), Str(strings.Join(labels, " "))}
	for i, ct := range types {
		z, err := zeroComponent(ct)
		if err != nil {
			return nil, fmt.Errorf("zero(%s): its component %s ∈ %s has no zero (data.md §4.6, X11): %v",
				ShowSet(ty), Spell(labels[i]), ShowSet(ct), err)
		}
		kids = append(kids, z)
	}
	return &Term{Kind: KApp, Kids: kids}, nil
}

func zeroComponent(ty string) (*Term, error) {
	switch {
	case IsRecord(ty):
		return zeroOf(ty)
	case ty == "int":
		return Int(0), nil
	case strings.HasPrefix(ty, "option(") || ty == "option":
		return Name("none"), nil
	case ty == "ptr":
		// A POINTER'S ZERO IS NULL, the all-zero encoding, which is how C's {0}
		// makes one; a pointer is a set only on the LLVM layer, whose null it is.
		return App(Name("llvm.null")), nil
	}
	if lo, hi, ok := IntRangeBig(ty); ok {
		if lo.Sign() <= 0 && hi.Sign() >= 0 {
			return Int(0), nil
		}
		return nil, fmt.Errorf("0 is not in %s", ShowSet(ty))
	}
	return nil, fmt.Errorf("only an integer set holding 0, an Option and a record have one")
}

// InStruct is an `_In_` struct parameter of a host declaration: its index, and
// the C type whose layout its argument is encoded by.
type InStruct struct {
	Index int
	CType string
	// LeafTypes are the struct's leaves' sets in canonical order: a leaf of a
	// callback type (`callback Q`) is a function of ours (ADR 0133). Rec is
	// its record set.
	LeafTypes []string
	Rec       string
}

// inStructArgs replaces each `_In_` struct argument that is a record value by
// (#ref "C" leaves…), the reference to a fresh copy of its encoding, its leaves
// in canonical order, nested records flattened (Theorem P: a pointer the host
// does not keep is a value passed). An argument not yet a value is left, and
// is reduced first.
func (e *Env) inStructArgs(op *Term, args []*Term, fuel *int) (*Term, bool, error) {
	changed := false
	out := append([]*Term{}, args...)
	for _, in := range e.InStructs[op.Name] {
		if in.Index >= len(args) {
			continue
		}
		a, err := normalize(args[in.Index], e, fuel)
		if err != nil {
			return nil, true, err
		}
		out[in.Index] = a
		var leaves []*Term
		// the value walked with its record set: a component whose set is a
		// nested record and which is no record value yet is a name bound to
		// one (MSG's pt after `(m, r) = GetMessageA(…)`), so its leaves are its
		// projections along each leaf's path, (#get l … c)
		var flat func(t *Term, ty string) bool
		flat = func(t *Term, ty string) bool {
			labels, ok := isRecValue(t)
			if !ok {
				return false
			}
			tl, tt := RecordFields(ty)
			for i, c := range t.Kids[2:] {
				ct := ""
				if k := labelIndex(tl, labels[i]); k >= 0 {
					ct = tt[k]
				}
				if _, ok := isRecValue(c); ok {
					if !flat(c, ct) {
						return false
					}
					continue
				}
				if IsRecord(ct) && !PlantNestedLeafUnprojected {
					for _, lf := range recordLeaves("", ct, nil) {
						g := c
						for _, l := range lf.path {
							g = App(Name(GetName), Str(l), g)
						}
						leaves = append(leaves, g)
					}
					continue
				}
				leaves = append(leaves, c)
			}
			return true
		}
		if flat(a, in.Rec) {
			// A CALLBACK'S LEAF IS A FUNCTION OF OURS (ADR 0133): the λ there,
			// or a definition's η-expansion, lifted, and the leaf its address.
			for j, lt := range in.LeafTypes {
				if j < len(leaves) && strings.HasPrefix(lt, CallbackPrefix) {
					v, err := e.liftCallback(strings.TrimPrefix(lt, CallbackPrefix), leaves[j])
					if err != nil {
						return nil, true, fmt.Errorf("%s: %w", ShowTerm(App(op, args...)), err)
					}
					leaves[j] = v
				}
			}
			out[in.Index] = App(Name(RefName), append([]*Term{Str(in.CType)}, leaves...)...)
			changed = true
		}
	}
	if !changed {
		return nil, false, nil
	}
	t, err := normalize(App(op, out...), e, fuel)
	return t, true, err
}

// LeafIndex is the leaf a record set's top-level label l is, in canonical
// order, when l's component is no record.
func LeafIndex(ty, l string) (int, bool) {
	labels, types := RecordFields(ty)
	j := 0
	for i, m := range labels {
		if m == l {
			return j, !IsRecord(types[i])
		}
		if IsRecord(types[i]) {
			j += len(recordLeaves("", types[i], nil))
		} else {
			j++
		}
	}
	return 0, false
}

// ProjectBound is t with each name `p.l…` whose head p is a key of bound read
// as the projection (#get "l" p)…: the binder decides a.b (ADR 0128 item 3),
// for a term no Load resolved, a host declaration's `where`.
func ProjectBound(t *Term, bound map[string]*Term) *Term {
	if t == nil {
		return nil
	}
	if t.Kind == KName {
		if i := strings.Index(t.Name, "."); i > 0 {
			if _, ok := bound[t.Name[:i]]; ok {
				r := Name(t.Name[:i])
				for _, l := range strings.Split(t.Name[i+1:], ".") {
					r = App(Name(GetName), Str(l), r)
				}
				return r
			}
		}
		return t
	}
	if len(t.Kids) == 0 {
		return t
	}
	c := *t
	c.Kids = make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		c.Kids[i] = ProjectBound(k, bound)
	}
	return &c
}

// CallbackPrefix begins a callback type's set, `callback Q` (ADR 0133).
const CallbackPrefix = "callback "

// CallbackDecl is a named claim declared as a callback type, its parameters'
// and result's sets resolved (ADR 0053, 0133).
type CallbackDecl struct {
	Names  []string
	Params []string
	Result string
	// AtLeastOnce is the host's count's lower end, at least 1 (`calls 1`):
	// a name used in the λ is then used on every path of the call (ADR 0143,
	// 𝒰's clause k · 𝒰(e)). The default count, 0.., has lower end 0.
	AtLeastOnce bool
}

// Lifted is a λ at a callback's field, lifted to a function of ours: its
// name, the claim Q it is checked under, and the closed λ (ADR 0133).
type Lifted struct {
	Name, Claim string
	Lam         *Term
}

// AddrSuffix names the primitive whose value is a lifted function's address
// at its claim's convention (llvm.md §6c).
const AddrSuffix = "~addr"

// liftCallback is the leaf v of a callback type Q: a closed λ of Q's arity,
// or the name of a definition or a host function, its η-expansion. It is
// recorded in e.Lifted, and the leaf becomes the call of its address.
func (e *Env) liftCallback(q string, v *Term) (*Term, error) {
	d, ok := e.Callbacks[q]
	if !ok {
		return nil, fmt.Errorf("%s is a callback type no header declares", ShowSet(q))
	}
	var lam *Term
	var name string
	if isCbFn(v) {
		v = v.Kids[1]
	}
	switch {
	case v.Kind == KName && (e.Prim[v.Name] || e.Defs[v.Name] != nil):
		// a definition or a host function by name is its η-expansion,
		// fn(x̄) { f(x̄) } (callbacks-tier1-research §3.2)
		ps := make([]string, len(d.Params))
		args := make([]*Term, len(d.Params))
		for i := range ps {
			ps[i] = fmt.Sprintf("a%d", i)
			if i < len(d.Names) {
				ps[i] = d.Names[i]
			}
			args[i] = Name(ps[i])
		}
		lam = Fn(ps, App(Name(v.Name), args...))
		name = strings.NewReplacer("/", "_", ".", "_", "-", "_").Replace(v.Name) + "~cb"
	case v.Kind == KFn && len(v.Params) == len(d.Params):
		lam = v
		// named by its text as the program wrote it: a call's sort claim is the
		// checker's, no part of the λ (ADR 0154)
		h := sha256.Sum256([]byte(StripSortClaims(v).String()))
		name = fmt.Sprintf("cb%x~cb", h[:4])
	case v.Kind == KFn:
		return nil, fmt.Errorf("a %s takes %d parameter(s), and this λ takes %d", ShowSet(q), len(d.Params), len(v.Params))
	default:
		return nil, fmt.Errorf("a %s is a λ or a function's name (ADR 0133), not %s", ShowSet(q), ShowTerm(v))
	}
	// CLOSED: a λ the host calls after the call that installed it may capture
	// no value of its scope (a captured cell is ADR 0133's step 3)
	if escapes(lam.Kids[0], 0) && !PlantCallbackOpen {
		return nil, fmt.Errorf("the %s captures a value of its scope; a callback the host keeps is closed "+
			"(callbacks.md §7, ADR 0133)", ShowSet(q))
	}
	if e.Lifted == nil {
		e.Lifted = map[string]Lifted{}
	}
	e.Lifted[name] = Lifted{Name: name, Claim: q, Lam: lam}
	addr := name + AddrSuffix
	e.Prim[addr] = true
	if e.Pure == nil {
		e.Pure = map[string]bool{}
	}
	e.Pure[addr] = true
	return App(Name(addr)), nil
}

// escapes says t, under n binders of a λ's body, refers to a binder outside
// the λ: an index past every binder between it and the λ.
func escapes(t *Term, n int) bool {
	switch t.Kind {
	case KBound:
		return t.Depth > n
	case KFn:
		return escapes(t.Kids[0], n+1)
	}
	for _, k := range t.Kids {
		if escapes(k, n) {
			return true
		}
	}
	return false
}

// PlantCallbackOpen lifts a λ that captures a value of its scope: the lifted
// function then refers to a binder it does not have (TestACallbackIsAFunctionOfOurs).
var PlantCallbackOpen bool

// PlantNestedLeafUnprojected passes a nested record bound to a name as one
// leaf, as before: MSG's pt is then one value where the struct has two
// (TestAWindowRunsItsMessageLoop).
var PlantNestedLeafUnprojected bool
