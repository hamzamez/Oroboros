package emit

import (
	_ "embed"
	"fmt"
	"strings"

	"oroboros/core"
)

// FACTS ARE AXIOM SCHEMATA OF A LOCAL THEORY EXTENSION (spec/theories.md §7).
//
// A fact is  ∀x̄. G₁(x̄) ∧ … ∧ Gₘ(x̄) → C(x̄)  over the linear fragment extended by
// one uninterpreted TRIGGER term. The refinement layer used to carry each such
// law as Go — `seedDivAxioms` was F1 and F2 — and a law written in the compiler
// is a law nobody can read, override, or check against the others. Here it is
// data, and instantiation is one algorithm for all of them.
//
// THE ALGORITHM IS INSTANTIATION ON PRESENT TERMS, TO A FIXPOINT. For every
// subterm s of the program matching a fact's trigger by σ, if the facts already
// assumed entail σG, assume σC. That is complete for a local extension
// (Ihlemann, Jacobs & Sofronie-Stokkermans, TACAS 2008) and it TERMINATES,
// because an instance adds inequalities and never a term: the candidate set is
// the finite set of (fact, subterm) pairs, and each is used at most once. A
// guard not yet entailed may become entailed by another instance — `div-floor`
// needs `len-nonneg` first — which is why it is a fixpoint and not one pass.
//
// THE GUARD MUST BE ENTAILED, NOT MERELY CONSISTENT: an implication with an
// unproven premise says nothing, and one false assumption makes a conjunctive
// fragment derive everything (postconditions.md §4, Lemma 1).

//go:embed lang-facts.oro
var langFactsSrc string

// langFacts is `lang`'s theory. A package variable so a test can put a WEAKENED
// theory in its place and watch a proof fail — every fact needs such a witness
// (theories.md §7.8).
var langFacts = mustFacts(langFactsSrc)

// Fact is one admitted F-B schema.
type Fact struct {
	Name    string
	Params  map[string]bool
	Trigger *core.Term   // the one extension term, parameters as names
	Guards  []*core.Term // conjoined
	Concl   *core.Term
}

func mustFacts(src string) []*Fact {
	fs, err := readFacts(src)
	if err != nil {
		panic("lang's facts do not load: " + err.Error())
	}
	return fs
}

func readFacts(src string) ([]*Fact, error) {
	terms, err := core.ReadAll(src)
	if err != nil {
		return nil, err
	}
	var out []*Fact
	for _, t := range terms {
		f, err := admitFact(t)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// admitFact reads `(fact NAME ((x τ)…) [(when φ…)] φ)` and decides §7.4's five
// conditions, refusing with the one that failed.
func admitFact(t *core.Term) (*Fact, error) {
	if formWord(t) == "lemma" {
		return nil, fmt.Errorf("`lemma` is F-E, reserved (facts.md §5)")
	}
	if formWord(t) != "fact" || len(t.Kids) < 4 || len(t.Kids) > 5 ||
		t.Kids[1].Kind != core.KName || t.Kids[2].Kind != core.KApp {
		return nil, fmt.Errorf("(fact NAME ((x τ)…) [(when φ…)] φ), got %s", t)
	}
	f := &Fact{Name: t.Kids[1].Name, Params: map[string]bool{}}
	for _, p := range t.Kids[2].Kids {
		if p.Kind != core.KApp || len(p.Kids) != 2 || p.Kids[0].Kind != core.KName {
			return nil, fmt.Errorf("fact %s: a parameter is (x τ), got %s", f.Name, p)
		}
		switch ty := core.TypeName(p.Kids[1]); {
		case ty == "f64":
			return nil, fmt.Errorf("fact %s: condition 5 — a float fact is refused: IEEE "+
				"arithmetic is not an ordered field (facts.md §8, ADR 0009)", f.Name)
		case ty == "":
			return nil, fmt.Errorf("fact %s: %s is not a type", f.Name, p.Kids[1])
		}
		f.Params[p.Kids[0].Name] = true
	}
	if len(t.Kids) == 5 {
		if formWord(t.Kids[3]) != "when" {
			return nil, fmt.Errorf("fact %s: guards are (when φ…), got %s", f.Name, t.Kids[3])
		}
		f.Guards = t.Kids[3].Kids[1:]
	}
	f.Concl = t.Kids[len(t.Kids)-1]
	if strings.Contains(f.Concl.String(), "(forall ") {
		return nil, fmt.Errorf("fact %s: a quantified fact is F-D, reserved (facts.md §5)", f.Name)
	}

	// A CONSTANT PARAMETER is one in a literal-only position: the divisor of `/`,
	// which the linear fragment reads as an atom only when it is a positive
	// literal (asLinear). So a match can bind it only to a literal, and `(* k …)`
	// is multiplication by a constant.
	constant := map[string]bool{}
	var constants func(*core.Term)
	constants = func(x *core.Term) {
		if x.Kind != core.KApp {
			return
		}
		if canonOp(x) == "div" && len(x.Args()) == 2 && x.Args()[1].Kind == core.KName &&
			f.Params[x.Args()[1].Name] {
			constant[x.Args()[1].Name] = true
		}
		for _, k := range x.Kids {
			constants(k)
		}
	}
	for _, g := range append([]*core.Term{f.Concl}, f.Guards...) {
		constants(g)
	}

	ext := map[string]*core.Term{}
	var nested bool
	for _, g := range append([]*core.Term{f.Concl}, f.Guards...) {
		collectExtensions(g, constant, ext, &nested)
	}
	switch {
	case nested:
		return nil, fmt.Errorf("fact %s: condition 2 — an extension term occurs inside another, "+
			"so the fact is not flat", f.Name)
	case len(ext) > 1:
		return nil, fmt.Errorf("fact %s: a fact relating two terms is F-C, reserved (facts.md §5)", f.Name)
	case len(ext) == 0:
		return nil, fmt.Errorf("fact %s: condition 1 — no trigger term; a fact about the linear "+
			"fragment alone is already decided by it", f.Name)
	}
	for _, x := range ext {
		f.Trigger = x
	}
	for p := range f.Params {
		if !occursName(f.Trigger, p) {
			return nil, fmt.Errorf("fact %s: condition 3 — parameter %s does not occur in the trigger "+
				"%s, so an instance is not determined by a term already present", f.Name, p, f.Trigger)
		}
	}
	// Condition 4: linear once the trigger is an atom and constants are literals.
	probe := map[string]*core.Term{}
	for p := range f.Params {
		if constant[p] {
			probe[p] = core.Int(2)
		}
	}
	for _, g := range append([]*core.Term{f.Concl}, f.Guards...) {
		if _, ok := obligation(core.Rename2(g, probe)); !ok {
			return nil, fmt.Errorf("fact %s: condition 4 — %s is not linear once %s is an atom",
				f.Name, g, f.Trigger)
		}
	}
	return f, nil
}

// canonOp is an application's operator as the fragment names it: `len` for
// every spelling of a length, the alias for a host operator, or the name.
func canonOp(x *core.Term) string {
	if x.Kind != core.KApp || x.Op().Kind != core.KName {
		return ""
	}
	n := x.Op().Name
	if isLenOp(n) {
		return "len"
	}
	if i := strings.LastIndex(n, "."); i >= 0 {
		n = n[i+1:]
	}
	if a, ok := opAlias[n]; ok {
		return a
	}
	return n
}

// collectExtensions finds the terms outside the linear fragment (theories.md
// §7.3): what is not a comparison, a conjunction, `+`, `−`, or `*` by a literal
// or a constant parameter.
func collectExtensions(x *core.Term, constant map[string]bool, ext map[string]*core.Term, nested *bool) {
	if x.Kind != core.KApp {
		return
	}
	lit := func(a *core.Term) bool { return a.Kind == core.KInt || (a.Kind == core.KName && constant[a.Name]) }
	args := x.Args()
	switch op := canonOp(x); {
	case op == "le" || op == "lt" || op == "ge" || op == "gt" || op == "eq" || op == "<=" ||
		op == "<" || op == ">=" || op == ">" || op == "=" || op == "and" || op == "if" ||
		op == "add" || op == "sub":
		for _, a := range args {
			collectExtensions(a, constant, ext, nested)
		}
	case op == "mul" && len(args) == 2 && (lit(args[0]) || lit(args[1])):
		for _, a := range args {
			collectExtensions(a, constant, ext, nested)
		}
	default:
		ext[x.String()] = x
		inner := map[string]*core.Term{}
		for _, a := range args {
			collectExtensions(a, constant, inner, nested)
		}
		if len(inner) > 0 {
			*nested = true
		}
	}
}

func occursName(t *core.Term, name string) bool {
	if t.Kind == core.KName {
		return t.Name == name
	}
	for _, k := range t.Kids {
		if occursName(k, name) {
			return true
		}
	}
	return false
}

// matchFact is first-order matching of a trigger against a subterm, operators
// compared as the fragment names them so one fact serves every host's spelling.
func matchFact(pat, t *core.Term, params map[string]bool, sigma map[string]*core.Term) bool {
	switch {
	case pat.Kind == core.KName && params[pat.Name]:
		if bound, ok := sigma[pat.Name]; ok {
			return bound.String() == t.String()
		}
		sigma[pat.Name] = t
		return true
	case pat.Kind == core.KInt:
		return t.Kind == core.KInt && t.Int == pat.Int
	case pat.Kind == core.KApp:
		if t.Kind != core.KApp || len(t.Kids) != len(pat.Kids) || canonOp(pat) == "" ||
			canonOp(pat) != canonOp(t) {
			return false
		}
		for i := 1; i < len(pat.Kids); i++ {
			if !matchFact(pat.Kids[i], t.Kids[i], params, sigma) {
				return false
			}
		}
		return true
	}
	return false
}

// seedFacts instantiates a theory on the terms of t, to a fixpoint.
func seedFacts(f *facts, t *core.Term, theory []*Fact) {
	var subterms []*core.Term
	var walk func(*core.Term)
	walk = func(x *core.Term) {
		if x == nil {
			return
		}
		if x.Kind == core.KFn {
			walk(x.Body())
			return
		}
		if x.Kind == core.KApp {
			subterms = append(subterms, x)
		}
		for _, k := range x.Kids {
			walk(k)
		}
	}
	walk(t)
	used := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for _, fact := range theory {
			for _, s := range subterms {
				sigma := map[string]*core.Term{}
				if !matchFact(fact.Trigger, s, fact.Params, sigma) {
					continue
				}
				key := fact.Name + " " + s.String()
				if used[key] || !instanceHolds(f, fact, sigma) {
					continue
				}
				goals, ok := obligation(core.Rename2(fact.Concl, sigma))
				if !ok {
					continue // not linear at this instance: assume nothing
				}
				used[key] = true
				changed = true
				for _, g := range goals {
					f.assumeLE(g, fmt.Sprintf("assumed %s <= 0 (fact %s)", g, fact.Name))
				}
			}
		}
	}
}

// instanceHolds reports whether every guard of σ(fact) is ENTAILED.
func instanceHolds(f *facts, fact *Fact, sigma map[string]*core.Term) bool {
	for _, g := range fact.Guards {
		goals, ok := obligation(core.Rename2(g, sigma))
		if !ok {
			return false
		}
		for _, goal := range goals {
			if !f.entails(goal) {
				return false
			}
		}
	}
	return true
}
