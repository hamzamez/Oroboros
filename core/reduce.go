package core

import (
	"math/big"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Reduction per core-0 §3: β on application of an abstraction, δ on unfolding a
// global definition, and a normal form parameterised by the target's primitive
// set P.
//
// β is call-by-need: it refuses to substitute where doing so would duplicate
// runtime work, and binds instead. The criterion came from measurement, not
// taste — see gauntlet/results/wordcount-2026-08-14.md:
//
//   - Duplicating a *pure* expression costs nothing; the host's CSE hoists it.
//     A duplicated a[i] compiled to byte-identical machine code.
//   - Duplicating an *allocating* expression costs 615x on Go and is quadratic,
//     because no host can hoist an allocation.
//
// Effects are handled by a side condition on β, specified in docs/spec/effects.md:
// an impure argument is never substituted, but normalised and let-bound at the
// application site, in argument order, whatever its occurrence count. The three
// clauses deny contraction, exchange and weakening respectively, which is what
// g5 §5 called the ordering discipline.

type Program struct {
	Defs map[string]*Term
	// Variadic are the variadic declarations whose calls the loader read as
	// variadic, by qualified name (spec/variadic.md §3). A target's Env refuses
	// a program that mentions a variadic declaration not in it: loaded without
	// the target's forms, a call's arguments were never read as the list.
	Variadic map[string]int
	// Cells are the declarations with cell parameters, by qualified name
	// (spec/local.md §5). A target's Env refuses a program that mentions one
	// not in it, as it does a variadic declaration.
	Cells map[string]CellDecl
	// ascribed is set once AscribeWide has run — see there.
	ascribed bool
	Order    []string // definition order, for stable diagnostics

	// Exports are the program's entry points, fully qualified, in declaration
	// order. A backend emits one function per export, named after it — which is
	// what replaced naming them by position (modules.md §1).
	Exports []string

	// Sigs are declared signatures, by fully qualified name. A signature is a
	// claim about a name that is checked against BOTH the library's definition
	// and any target's native implementation — which is the one job no host
	// compiler can do, since the two live on different targets.
	Sigs map[string]*Sig

	// Sums are the declared sum types, by name. A sum is Σ over a finite index
	// set, and its VALUES are ordinary products of a tag and a payload — so this
	// table is consulted for exhaustiveness and for signatures, and by nothing
	// in the reducer (sums-research.md, docs/spec/sums.md).
	Sums map[string]*Sum

	// TargetDefined are the names a library defines AND the target redefines in
	// `D_T`, so the build can say which one it used — `▷`, said out loud.
	TargetDefined []string

	// Unresolved are `use` paths that found no file on the search path. That is
	// not an error — it is how a target-provided module looks (modules.md §4) —
	// but it is also how a MISSPELLED path looks, and the two were
	// indistinguishable until the name failed to resolve much later.
	Unresolved []string
}

// Env is a program viewed through one target: which names reduce, and which are
// the irreducible floor.
type Env struct {
	Defs map[string]*Term
	Prim map[string]bool
	Pure map[string]bool // names whose APPLICATION is pure; see pureName
	// Observe names the host calls that may read a live buffer at a table
	// parameter (ADR 0069): pure, with scalar results. An application of one to
	// a bound variable is a read of a table through it, as `(b i)` is (§7c).
	Observe map[string]bool
	Rec     map[string]bool // recursive definitions are never δ-reduced

	// Word is the target's word (ADR 0026): constant folding stays inside it,
	// because that is where the target's run time computes the same integer.
	Word Word
	// initHole says the value firstUse places may go in a loop's initializer:
	// it holds no loop (reduce.go, headUse).
	initHole bool
	// floatSeq numbers the placeholders let-floating names a floated binder by
	// until β chooses its name (core/work.go).
	floatSeq int

	// fresh numbers the binders the n-ary let conversion introduces. Per Env,
	// so a compile is deterministic.
	fresh int

	// depth is how deep reduction has recursed (MaxDepth).
	depth int
	// consumed caches consumes for a definition's parameter (spec/errors.md
	// §7.2).
	consumed map[string]bool

	// Requires are the contracts whose parameter ranges are obligations at
	// their calls (ADR 0028). For each name in it — its entry lists a declared
	// range per parameter, "" for none — every direct call has each ranged
	// argument wrapped in a mark, `(#req "def" "param" "type" a)`, whose value
	// is a. The marks change no reduction decision: an argument is an integer,
	// and the only rule that inspects one's shape — duplicable — looks through
	// the mark.
	//
	// THE OBLIGATIONS OF A PROGRAM ARE IN ITS RESIDUAL, and nowhere else
	// (hazard-2026-10-02). A mark whose argument reduces to a literal is
	// decided here by InRange: one inside the range is dropped, and one outside
	// it STAYS, a mark no decider can prove. It used to go to a callback that
	// only a driver installed, and with no callback there were no marks at all,
	// so which obligations a program had depended on who compiled it.
	Requires map[string][]string
	// InRange decides a literal against a declared range. nil decides nothing,
	// and every literal mark stays.
	InRange func(ty string, v int64) bool
	// ReqParams names the parameters of a name in Requires that is not a
	// definition — a host function, whose declared parameter ranges are
	// obligations at its calls as a definition's are (ADR 0028,
	// gotarget-2026-09-30) — for the report.
	ReqParams map[string][]string

	// Wheres are the definitions whose `where` is an obligation at their calls
	// (ADR 0028). At β the clause is instantiated with the arguments as β
	// passes them — an impure one already bound to a name, so nothing is
	// duplicated — and reduced: `true` leaves nothing, and anything else marks
	// the call's result, `(#reqw "def" cond body)`, for the analyses to decide.
	// A clause that reduced to `false` is marked with the clause as written, so
	// the refusal can show it.
	Wheres map[string]WhereContract

	// unresolvedPaths carries Program.Unresolved through to diagnostics.
	unresolvedPaths map[string]bool

	// TYPES ARE STATIC VALUES (spec/typevalues.md, ADR 0063). Sets are the names
	// that denote a set: scope accepts one, β substitutes it, and a residual
	// that still holds one is refused (Law S: a set has no run-time value).
	// Static gives each declaration with static parameters their number, and
	// Instantiate makes its instance at sets, f#[T̄], the δ-rule of a static
	// application: (f T̄) ⟶ f#[T̄].
	Sets        map[string]bool
	Static      map[string]int
	Instantiate func(f string, args []string) (string, error)
}

// WhereContract is a definition's `where`, over its signature's parameter names.
type WhereContract struct {
	Params []string
	Cond   *Term
}

// SetUnresolved records the imports that found no file, for importHint.
func (e *Env) SetUnresolved(paths []string) {
	e.unresolvedPaths = map[string]bool{}
	for _, p := range paths {
		e.unresolvedPaths[p] = true
	}
}

// AscribeWide moves every declared widening onto its definition's body, for a
// target whose word is w.
func (p *Program) AscribeWide(w Word) {
	// ONCE PER PROGRAM: a second Env over the same program must not wrap a body
	// twice, and a program is only ever built for one target.
	if p.ascribed {
		return
	}
	p.ascribed = true
	// ═══ A DECLARED WIDENING SURVIVES INLINING, BECAUSE IT IS A DIRECTIVE
	//
	// A range has three effects (scalarrange-2026-08-31): it is a type, it is a
	// premise, and it is a representation. Reduction inlines every non-exported
	// call, and the first two survive that — the checker re-derives the type,
	// and dropping the premise is a STRENGTHENING, since the body's obligations
	// then land on the caller's concrete values (refinements.md §6b).
	//
	// THE THIRD DOES NOT SURVIVE, and the difference is that it is not a fact.
	// A range above the word does not assert something the compiler checks; it
	// REQUESTS arbitrary precision. Inlining gives more information about values
	// and none about intent, so removing the boundary removes the only record of
	// the instruction — and the interval analysis cannot supply it, because a
	// multiplicative loop widens to [-inf, +inf] and a factorial's bound is not
	// expressible in the domain at all.
	//
	// So the declaration is moved onto the TERM, where reduction preserves it:
	// `(the "int 0 …" body)`. `the` is a structural name the compiler injects
	// into every target, exactly as `if`, `let` and `loop` are, and it is erased
	// at emission — it carries a range and nothing else.
	//
	// Only a range ABOVE THE TARGET'S WORD is wrapped. Below it the analysis
	// derives what it needs and an ascription would be noise; above it there is
	// nothing to derive. That is the same line `Word.Exceeds` draws everywhere
	// else: a refinement on one side, a widening on the other.
	//
	// IT RUNS WHEN THE TARGET IS KNOWN, not while modules are resolved, because
	// under ADR 0026 the line is the target's: `(int 0 (pow 2 60))` is a word on
	// Go and arbitrary precision on JavaScript. Loading is resolution (ADR 0011)
	// and knows no target; the target's Env is where this belongs.
	for q, sig := range p.Sigs {
		body, ok := p.Defs[q]
		if !ok || sig == nil || w.ValueType(sig.Result) != BigType { // above the word, ℤ included
			continue
		}
		p.Defs[q] = ascribeResult(body, sig.Result)
	}
}

// RequireName marks an argument with the range its parameter declares — a
// measurement, see Env.Requires. `#` is not an identifier character, so no
// program can write one.
const RequireName = "#req"

// ClaimName is a binder's claim whose set is not yet canonical, (#claim "b" S e):
// the reducer normalizes S and makes it (#req ClaimDef "b" "ty" e) when S is a
// range, or e when it is not (desugar.go).
const ClaimName = "#claim-set"

// ClaimDef is the def a binder's claim marks carry, `b ∈ S = e` (desugar.go):
// no definition, the claim itself.
const ClaimDef = "#claim"

// markRequires wraps each ranged argument of a direct call to a measured
// definition. An argument already marked for the same range is left alone, so a
// call reduced twice is marked once.
func (e *Env) markRequires(t *Term) *Term {
	def := t.Op().Name
	tys, ok := e.Requires[def]
	if !ok {
		return t
	}
	kids := append([]*Term(nil), t.Kids...)
	changed := false
	for i, ty := range tys {
		if ty == "" || i+1 >= len(kids) || e.coversWord(ty) {
			continue
		}
		a := kids[i+1]
		if IsRequire(a) && a.Kids[3].Str == ty {
			continue
		}
		kids[i+1] = &Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(def), Str(e.paramName(def, i)), Str(ty), a}}
		changed = true
	}
	if !changed {
		return t
	}
	return &Term{Kind: KApp, Kids: kids}
}

// coversWord reports a declared range holding the whole signed word, which is
// vacuous for an integer argument — every `int` is held in the word (ADR 0026) —
// so a call is not marked for it, and a program whose only contracts are such
// ranges pays nothing to check them.
func (e *Env) coversWord(ty string) bool {
	lo, hi, ok := IntRangeBig(ty)
	return ok && lo.IsInt64() && hi.IsInt64() && lo.Int64() <= e.Word.Lo && hi.Int64() >= e.Word.Hi
}

// paramName is a measured definition's i-th parameter, for the report.
func (e *Env) paramName(def string, i int) string {
	if d, ok := e.Defs[def]; ok && d.Kind == KFn && i < len(d.Params) {
		return d.Params[i]
	}
	if ns := e.ReqParams[def]; i < len(ns) && ns[i] != "" {
		return ns[i]
	}
	return fmt.Sprint(i)
}

// IsRequire recognises a measurement mark.
func IsRequire(t *Term) bool {
	return t != nil && t.Kind == KApp && len(t.Kids) == 5 && t.Kids[0].Kind == KName && t.Kids[0].Name == RequireName
}

// RequireFalseName wraps the condition of a where-mark that reduced to false:
// `(#false cond)`, where cond is the clause as instantiated, kept for the
// refusal. It is not a primitive of any target, so no prover reads through it.
const RequireFalseName = "#false"

// IsRequireFalse recognises that condition, and returns the clause.
func IsRequireFalse(t *Term) (*Term, bool) {
	if t != nil && t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == RequireFalseName {
		return t.Kids[1], true
	}
	return nil, false
}

// SegmentName is the segment former, `(#segment b lo hi)`, J's `b[lo:hi]`:
// structural, a name a program cannot write.
const SegmentName = "#segment"

// SegmentDef labels a segment's formation obligation.
const SegmentDef = "a segment b[lo:hi]"

// segment is δ for the segment former. A segment is its table restricted and
// translated: (b[lo:hi])(k) = b(lo + k) on [0, hi − lo), the function ADR 0074
// gave a host's restricted parameter, now wherever a table is (ADR 0084):
//
//	(#segment b lo hi)  ⟶  (#reqw "segment" (0 ≤ lo ≤ hi ≤ len b) (table (− hi lo) (fn (k) (b (+ lo k)))))
//
// The formation obligation makes its length hi − lo a natural number, and the
// rule's own domain (betaTab) keeps every read inside it, so neither a read
// past hi nor a negative length can be written. It is a rule, so it fuses: a
// segment that is only indexed and measured is erased, with no copy and no
// representation, on every target. An impure operand is bound first (ADR 0010).
func (e *Env) segment(args []*Term, fuel *int) (*Term, error) {
	for _, a := range args {
		if !e.pureTerm(a, map[string]bool{}) {
			ns := e.freshNames(3, args...)
			inner := App(Name(SegmentName), Name(ns[0]), Name(ns[1]), Name(ns[2]))
			return normalize(App(Fn(ns, inner), args...), e, fuel)
		}
	}
	b, lo, hi := args[0], args[1], args[2]
	k := e.freshNames(1, args...)[0]
	// 0 ≤ lo is a range on lo, decided where lo is computed, inside the rule;
	// the rest a relation, the where-mark (betaTab).
	marked := lo
	if e.Word != (Word{}) {
		marked = e.nonnegative(SegmentDef, "lo", lo)
	}
	rule := App(Name("table"), App(Name("-"), hi, lo), Fn([]string{k}, App(b, App(Name("+"), marked, Name(k)))))
	plain := lo
	cond := App(Name("if"), App(Name("<="), plain, hi), App(Name("<="), hi, App(Name("len"), b)), Bool(false))
	if e.Word == (Word{}) {
		cond = App(Name("if"), App(Name("<="), Int(0), plain), cond, Bool(false))
	}
	c, err := normalize(cond, e, fuel)
	if err != nil {
		return nil, err
	}
	body, err := normalize(rule, e, fuel)
	if err != nil {
		return nil, err
	}
	switch {
	case c.Kind == KBool && c.IsTrue():
		return body, nil
	case c.Kind == KBool:
		return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(SegmentDef), App(Name(RequireFalseName), cond), body}}, nil
	}
	return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(SegmentDef), c, body}}, nil
}

// RuleIndexDef labels the obligation an index into a table given by a rule
// carries, 0 ≤ i < n (betaTab).
const RuleIndexDef = "an index into a table given by a rule"

// RequireWhereName marks a call's result with its definition's instantiated
// `where` (ADR 0028): `(#reqw "def" cond body)`, whose value is body.
const RequireWhereName = "#reqw"

// IsRequireWhere recognises a where-mark.
func IsRequireWhere(t *Term) bool {
	return t != nil && t.Kind == KApp && len(t.Kids) == 4 && t.Kids[0].Kind == KName && t.Kids[0].Name == RequireWhereName
}

// withWhere rebuilds a where-mark around a new body.
func withWhere(mark, body *Term) *Term {
	return &Term{Kind: KApp, Kids: []*Term{mark.Kids[0], mark.Kids[1], mark.Kids[2], body}}
}

// peelWheres strips the where-marks around a term, outermost first.
func peelWheres(t *Term) (*Term, []*Term) {
	var ms []*Term
	for IsRequireWhere(t) {
		ms = append(ms, t)
		t = t.Kids[3]
	}
	return t, ms
}

// wrapWheres puts peeled marks back around a term, keeping their order.
func wrapWheres(t *Term, ms []*Term) *Term {
	for i := len(ms) - 1; i >= 0; i-- {
		t = withWhere(ms[i], t)
	}
	return t
}

// markWhere instantiates def's `where` with the call's arguments, reduces it,
// and either discharges it (a literal `true`), reports it refuted (`false`), or
// marks body with it.
//
// A PURE argument goes in as itself, an impure one as the name β bound it to.
// The condition is erased, so copying a pure term into it costs nothing and runs
// nothing — and it is what lets `(len (array 123 …))` fold to 20 where β, seeing
// a table used twice, bound it to a name whose length nothing knows.
func (e *Env) markWhere(def string, subs, args []*Term, body *Term, fuel *int) (*Term, error) {
	w := e.Wheres[def]
	if len(w.Params) != len(subs) || len(args) != len(subs) {
		return body, nil
	}
	m := make(map[string]*Term, len(subs))
	// An argument's own range mark is not part of this obligation — its mark
	// checks it — so the condition reads the argument without it.
	for i, p := range w.Params {
		if e.pureTerm(args[i], map[string]bool{}) {
			m[p] = StripRequires(args[i])
		} else {
			m[p] = StripRequires(subs[i])
		}
	}
	cond, err := normalize(subst(w.Cond, m), e, fuel)
	if err != nil {
		return nil, err
	}
	// AN OBLIGATION INSIDE THE CONDITION IS ITS OWN: a static argument's
	// where-mark (a codec's, lib/bytes) is hoisted to stand around the body
	// beside this one, and its value is what the condition reads.
	var inner []*Term
	cond = hoistWheres(cond, &inner)
	body = wrapWheres(body, inner)
	if len(inner) > 0 {
		if cond, err = normalize(cond, e, fuel); err != nil {
			return nil, err
		}
	}
	if cond.Kind == KBool {
		if cond.IsTrue() {
			return body, nil
		}
		// FALSE AT THIS CALL. The mark carries the false literal, which nothing
		// proves, and the instantiated clause for the message.
		return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(def),
			App(Name(RequireFalseName), subst(w.Cond, m)), body}}, nil
	}
	return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(def), cond, body}}, nil
}

// StripRequires erases every measurement mark, leaving the argument it wrapped.
func StripRequires(t *Term) *Term {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case KApp:
		if IsRequire(t) {
			return StripRequires(t.Kids[4])
		}
		if IsRequireWhere(t) {
			return StripRequires(t.Kids[3])
		}
		kids := make([]*Term, len(t.Kids))
		changed := false
		for i, k := range t.Kids {
			kids[i] = StripRequires(k)
			changed = changed || kids[i] != k
		}
		if !changed {
			return t
		}
		c := *t
		c.Kids = kids
		return &c
	case KFn:
		b := StripRequires(t.Kids[0])
		if b == t.Kids[0] {
			return t
		}
		c := *t
		c.Kids = []*Term{b}
		return &c
	}
	return t
}

// freshNames returns n binder names free in none of avoid.
//
// FRESH FOR WHAT THE BINDERS WILL CLOSE OVER, not merely new: Fn abstracts every
// free occurrence of its parameters, so a binder spelled like a free name of an
// eliminator argument captures it. The prefix is the conversion's own — `#m` is
// `match`'s loop variables (read.go), and the first spelling used here collided
// with them — and the counter alone is not enough, because it restarts with each
// Env while a residual carries its binders' spellings into the next reduction.
func (e *Env) freshNames(n int, avoid ...*Term) []string {
	free := map[string]bool{}
	for _, t := range avoid {
		for v := range freeVars(t) {
			free[v] = true
		}
	}
	out := make([]string, n)
	for i := range out {
		for {
			e.fresh++
			out[i] = fmt.Sprintf("#nl%d", e.fresh)
			if !free[out[i]] {
				break
			}
		}
	}
	return out
}

func NewProgram() *Program {
	return &Program{Defs: map[string]*Term{}, Sigs: map[string]*Sig{}, Sums: map[string]*Sum{}}
}

// A module is a scope: a path, what it imports, what it exports, and what it
// defines. It is NOT a unit of compilation and NOT a unit of reduction — by the
// time Load returns, every module has been flattened into one qualified
// namespace and the reducer cannot tell modules existed (modules.md §5, R1).
//
// That flattening is the whole design. Resolution happens before reduction, so
// `P_T` and `Defs` are keyed the same way and the four cells still work.
type Module struct {
	Path    string
	Uses    map[string]string // alias -> module path
	Exports map[string]bool   // its entry points: compiled when this module is (ADR 0078)
	// Pub is its interface, the names an importer may use, and HasPub whether it
	// has one: a module read from J always does, and an empty one hides every
	// name. One with none (a core term written by hand) shows everything.
	Pub    map[string]bool
	HasPub bool
	Defs   map[string]*Term // unqualified name -> body, before resolution
	Order  []string
	Sigs   map[string]*Sig
	Sums   map[string]*Sum
}

// qualify joins a module path to a local name. The root module has no path, so
// a program that never says `(module …)` behaves exactly as before.

// bindName picks the name a call-by-need binding may safely use.
//
// Reduction's invariant is stated at the KFn case of normalize: it works on
// CLOSED bodies and never reopens one, so a bound variable can never be
// mistaken for a global. Call-by-need is the one place that breaks the
// invariant on purpose — when β declines to substitute, it puts the parameter's
// own NAME back into the body and wraps a `let` around it.
//
// That is safe only if δ will not unfold the name it puts back. It usually is,
// because `resolve` qualifies every definition with its module path. It is not
// in the MAIN module, where `qualify("", n)` leaves definitions bare: a
// parameter spelled like a top-level `def` in the program being compiled was
// replaced by that definition, silently and with no error.
//
//	(def d1 (array 7 8 9))
//	(def run (fn (k) ((fn (d1) (go.+ d1 d1)) (go.+ k 100))))
//	→ (let (go.+ k 100) (fn (d1) (go.+ (array 7 8 9) (array 7 8 9))))
//
// One occurrence substituted and compiled correctly; two occurrences reached
// this path and did not. Found by a JSON tree builder with a local `d1` and a
// document named `d1` (json-tree-2026-08-26).
func (e *Env) bindName(p string, params []string, used map[string]bool) string {
	clash := func(n string) bool {
		if used[n] {
			return true
		}
		if _, ok := e.Defs[n]; ok {
			return true
		}
		return false
	}
	if !clash(p) {
		return p
	}
	taken := map[string]bool{}
	for _, q := range params {
		taken[q] = true
	}
	for i := 1; ; i++ {
		n := fmt.Sprintf("%s%d", p, i)
		if !clash(n) && !taken[n] {
			return n
		}
	}
}

func qualify(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

// A Resolver finds the source of a module by path. It returns found=false for a
// path the target provides rather than a library — `go/strings` has no file.
//
// This is the whole of multi-file support: resolution already turns every name
// into a fully qualified one, so loading more modules only means having more of
// them in scope. Nothing downstream can tell whether a module came from the
// entry file or from disk.
//
// It returns the module's forms, read by the rendering its source is written in
// (J; spec/j.md): core reads no source itself.
type Resolver func(path string) (forms []Form, found bool, err error)

// Load reads one source with no file resolution — every module it uses must be
// declared in the same text, or provided by the target.
func Load(forms []Form) (*Program, []*Term, error) { return LoadWith(forms, nil) }

// TargetDefs is `D_T` — the definitions a TARGET contributes, keyed by module
// path, in the order they were declared (target-system.md §6.2).
//
// A target has always been able to say *this host spells that name like this*;
// `D_T` lets it say *this host DEFINES that name, in Oroboros*. It is the third
// source of an implementation, and selection is the same `▷` used everywhere
// else:
//
//	impl  =  P_T  ▷  D_T  ▷  D
//
// Nothing in the reducer changes. A name in `D_T` is more entries in the
// definition environment, δ unfolds it exactly as it unfolds a library's, and
// covering already reports a name that reaches no primitive.
type TargetDefs map[string][]Form

// LoadWithDefs is LoadWith plus `D_T`.
//
// The defs are injected AFTER the import fixpoint, and that is not an
// implementation detail: declaring the module up front would put its path in
// `seen`, and the library file that really defines it would never be read.
func LoadWithDefs(forms []Form, resolve Resolver, dt TargetDefs) (*Program, []*Term, error) {
	return loadWith(forms, resolve, dt)
}

// LoadWith reads an entry source and follows its imports, transitively.
//
// Only the ENTRY file contributes entry points: a library's bare terms and
// exports are not the program's. That is what makes `(use …)` a dependency
// rather than an inclusion.
func LoadWith(forms []Form, resolve Resolver) (*Program, []*Term, error) {
	return loadWith(forms, resolve, nil)
}

func loadWith(forms []Form, resolve Resolver, dt TargetDefs) (*Program, []*Term, error) {
	mods, entries, err := partition(forms)
	if err != nil {
		return nil, nil, err
	}
	entryPaths := map[string]bool{}
	for _, m := range mods {
		entryPaths[m.Path] = true
	}
	var unresolved []string

	// Fixpoint over imports. A module already in scope is never re-read, which
	// is also what terminates on a cycle. It is a closure, called again once
	// D_T is glued in: a target's library may `use` a library module itself
	// (retract.go's definitions use `result`), and a module's imports are
	// those of the module WITH its target library, so the closure is taken
	// after the gluing. A second call finds nothing new where nothing was added.
	seen := map[string]bool{}
	for _, m := range mods {
		seen[m.Path] = true
	}
	closeImports := func() error {
		if resolve == nil {
			return nil
		}
		for {
			var want string
			for _, m := range mods {
				for _, path := range m.Uses {
					if !seen[path] {
						want = path
						break
					}
				}
				if want != "" {
					break
				}
			}
			if want == "" {
				return nil
			}
			seen[want] = true
			sub, found, err := resolve(want)
			if err != nil {
				return fmt.Errorf("use %s: %w", want, err)
			}
			if !found {
				unresolved = append(unresolved, want)
				continue // provided by the target, not by a library
			}
			subMods, _, err := partition(sub)
			if err != nil {
				return fmt.Errorf("%s: %w", want, err)
			}
			declared := false
			var extra []string
			for _, m := range subMods {
				if m.Path == "" && len(m.Defs) == 0 {
					continue // the empty anonymous scope every file starts with
				}
				if m.Path == "" {
					return fmt.Errorf(
						"%s: a library file is the module its path names, `module %s { … }` (ADR 0064 item 1), and holds nothing outside it", want, want)
				}
				// A library file declares exactly the module its path names.
				// Adding the extras made their visibility depend on LOAD ORDER:
				// `(use extra/helper)` failed on its own and succeeded after
				// `(use extra/pair)` had pulled in the file that declared both.
				// Meaning that depends on what else is in scope is the thing
				// qualified imports exist to prevent (modules.md §3).
				if m.Path != want {
					extra = append(extra, m.Path)
					continue
				}
				declared = true
				seen[m.Path] = true
				mods = append(mods, m)
			}
			switch {
			case !declared && len(extra) == 1:
				return fmt.Errorf("%s holds `module %s`; a library file's module "+
					"is the path that imports it (ADR 0064 item 1)", want, extra[0])
			case !declared:
				return fmt.Errorf("%s holds no `module %s`", want, want)
			case len(extra) > 0:
				return fmt.Errorf("%s also holds `module %s`; a library file holds "+
					"one module, and it is the one its path names — put %s in its own file, or its "+
					"members are reachable only after something else has imported this one",
					want, strings.Join(extra, ", "), extra[0])
			}
		}
	}
	if err := closeImports(); err != nil {
		return nil, nil, err
	}
	// `D_T` — the target's own definitions, injected once every library the
	// program uses has been read.
	//
	// A name the library ALSO defines is `▷`: the target's wins, and the program
	// records it so the build can say so, exactly as it says so for a native
	// primitive shadowing a definition. A module the program never uses gets
	// nothing — covering is demand-driven, and a `provides` for a module nobody
	// imports contributes no more than a `prim` for one does.
	// GLUING AND CLOSING ALTERNATE TO A JOINT FIXPOINT. A target's library may
	// `use` a module, which closing the imports then reads or finds provided by
	// the target, and that module may have a target library of its own to glue.
	// Each pass glues every module present or wanted that is not yet glued, then
	// closes the imports; it stops when a pass glues nothing, which it must,
	// since each pass glues at least one of finitely many paths.
	var overridden []string
	glued := map[string]bool{}
	// A VARIADIC DECLARATION's fixed parameter count, by the qualified name a
	// call resolves to (spec/variadic.md §3): the target's `variadic` forms.
	variadic := map[string]int{}
	cells := map[string]CellDecl{}
	borrows := map[string]BorrowDecl{}
	for len(dt) > 0 {
		byPath := map[string]*Module{}
		for _, m := range mods {
			byPath[m.Path] = m
		}
		wanted := map[string]bool{}
		for _, u := range unresolved {
			wanted[u] = true
		}
		paths := make([]string, 0, len(dt))
		for path := range dt {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		progress := false
		for _, path := range paths {
			if glued[path] {
				continue
			}
			m, ok := byPath[path]
			if !ok {
				// A TARGET-PROVIDED MODULE: no file declares it, so the target's
				// definitions ARE it (modules.md §4). This is the shape that lets
				// one program pick up a different host binding on each target.
				//
				// ONLY IF THE PROGRAM ASKED FOR IT. A `use` that found no file is
				// exactly how such a module looks, and a path nothing imported is
				// a module this program does not have — injecting it anyway put
				// every library's host binding into every program, and
				// `CheckProgram` reads EVERY definition, so `hello.oro` was
				// refused for a name in tally's Go binding.
				if !wanted[path] {
					continue
				}
				m = newModule(path)
				mods = append(mods, m)
				byPath[path] = m
			}
			glued[path] = true
			progress = true
			for _, f := range dt[path] {
				switch f.Kind {
				case "use":
					if prev, dup := m.Uses[f.Alias]; dup && prev != f.Name {
						return nil, nil, fmt.Errorf("a target's `provides … %s { … }` binds %s to %s, "+
							"and the module binds it to %s", path, ShowName(f.Alias), f.Name, prev)
					}
					m.Uses[f.Alias] = f.Name
				case "def":
					if _, dup := m.Defs[f.Name]; dup {
						overridden = append(overridden, qualify(path, f.Name))
					} else {
						m.Order = append(m.Order, f.Name)
					}
					m.Defs[f.Name] = f.Term
				case "variadic":
					variadic[qualify(path, f.Name)] = int(f.Term.Int)
				case "cells":
					d, err := readCellDecl(f.Term)
					if err != nil {
						return nil, nil, fmt.Errorf("%s: %w", qualify(path, f.Name), err)
					}
					cells[qualify(path, f.Name)] = d
				case "borrows":
					d, err := readBorrowDecl(f.Term)
					if err != nil {
						return nil, nil, fmt.Errorf("%s: %w", qualify(path, f.Name), err)
					}
					borrows[qualify(path, f.Name)] = d
				default:
					return nil, nil, fmt.Errorf("a target's `provides … %s { … }` may hold `use` and "+
						"definitions; got a %s", path, f.Kind)
				}
			}
		}
		if !progress {
			break
		}
		if err := closeImports(); err != nil {
			return nil, nil, err
		}
	}

	// An `export` or a `sig` naming nothing was silently dropped, because both
	// were read off m.Order — the list of DEFINITIONS. A misspelled export left
	// a program with no entry points, and build then reported the absence of a
	// `main` it could see two lines above.
	for _, m := range mods {
		var noDef []string
		for n := range m.Exports {
			if _, ok := m.Defs[n]; !ok {
				noDef = append(noDef, n)
			}
		}
		if len(noDef) > 0 {
			sort.Strings(noDef)
			return nil, nil, fmt.Errorf("%s exports %s, which it does not define",
				modLabel(m.Path), strings.Join(noDef, ", "))
		}
		for n := range m.Pub {
			if _, ok := m.Defs[n]; !ok && !isSumMember(m, n) {
				noDef = append(noDef, n)
			}
		}
		if len(noDef) > 0 {
			sort.Strings(noDef)
			return nil, nil, fmt.Errorf("%s makes %s `pub`, which it does not define",
				modLabel(m.Path), strings.Join(noDef, ", "))
		}
		noDef = nil
		for n := range m.Sigs {
			if _, ok := m.Defs[n]; !ok {
				noDef = append(noDef, n)
			}
		}
		if len(noDef) > 0 {
			sort.Strings(noDef)
			return nil, nil, fmt.Errorf("%s states a header for %s but gives no body: %s",
				modLabel(m.Path), ShowName(noDef[0]), strings.Join(ShowNames(noDef), ", "))
		}
	}

	// `case` expands here, before name resolution, and with EVERY module in
	// scope. That is the reason it is not reader sugar like `match`: the reader
	// sees one file, and an error type is declared in another. Its patterns are
	// resolved in the module that wrote them (sumRef).
	byPath := map[string]*Module{}
	sums := map[string]*Sum{}
	for k, s := range langSums {
		sums[k] = s
	}
	for _, m := range mods {
		byPath[m.Path] = m
		for _, sum := range m.Sums {
			k := sumKey(m.Path, sum)
			if _, own := langSums[k]; own {
				return nil, nil, fmt.Errorf("%s declares the variant %s, which is the language's "+
					"own. The main module's names are unqualified, like the language's, so they "+
					"would be one name; declare it in a `module … { … }`, where it shadows the "+
					"language's", modLabel(m.Path), ShowSet(sum.Name))
			}
			sums[k] = sum
		}
	}
	for _, m := range mods {
		look := func(sp string) (ctorRef, error) { return m.constructor(sp, byPath, mods) }
		for n, body := range m.Defs {
			x, err := expandCase(body, look)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", qualify(m.Path, n), err)
			}
			m.Defs[n] = x
		}
	}
	// AND OVER BARE ENTRY TERMS, which this loop did not reach. A `case` inside
	// a `(def …)` expanded and the identical `case` written at top level did
	// not — it stayed an application of an unbound name and reduced to a stuck
	// term rather than an error. A construct that works in one position and
	// silently does not in another is the shape of bug this repository keeps
	// finding; every real program puts its code in a `def`, which is why it was
	// never seen.
	for i := range entries {
		m := entries[i].mod
		look := func(sp string) (ctorRef, error) { return m.constructor(sp, byPath, mods) }
		x, err := expandCase(entries[i].term, look)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", modLabel(entries[i].mod.Path), err)
		}
		entries[i].term = x
	}

	// A PROGRAM CANNOT YET NAME ITS OWN SET (spec/typevalues.md §1). A type in a
	// sig that names one of the program's definitions has no set to denote
	// before step 2, and the checker read it as an opaque type: refused at its
	// first use, or printed as an unknown host type (du-2026-10-14). Refused
	// here, at the sig, by its own name.
	for _, m := range mods {
		for n, sig := range m.Sigs {
			for _, ty := range sigTypes(sig) {
				if def, ok := m.definitionNamed(ty, byPath); ok {
					return nil, nil, fmt.Errorf("%s: its header names %s, a definition, as a set; "+
						"a program cannot yet name its own set (spec/typevalues.md §1, step 2 of §10), "+
						"so write the set where it is used, as 0..255", ShowName(qualify(m.Path, n)), ShowName(def))
				}
			}
		}
	}

	// A SUM IN A SIGNATURE is the product of its tag and its payload, which is
	// the whole representation story: `(sig f ((n int)) result)` becomes two
	// results, and two results are already Go's multiple return, Java's record,
	// JavaScript's object and x86's rax/rdx (values.md). Nothing new is emitted
	// on any target, and Go's own `(T, error)` idiom IS this shape.
	for _, m := range mods {
		for n, sig := range m.Sigs {
			q := qualify(m.Path, n)
			// An applied type in a PARAMETER is not built: a variant crosses a
			// boundary as a result (sums.md §6).
			for _, pm := range sig.Params {
				if _, _, applied := Applied(pm.Type); applied {
					return nil, nil, fmt.Errorf("%s takes a %s. A variant type crosses a boundary "+
						"as a RESULT today; as a parameter it is not built", q, pm.Type)
				}
			}
			if len(sig.Results) > 0 {
				continue
			}
			inst, err := m.instance(q, sig.Result, byPath)
			if err != nil {
				return nil, nil, err
			}
			if inst == nil {
				continue
			}
			// A SUM CROSSING A BOUNDARY is its tag and its slots (data.md
			// §5.5.5), one result each: the representation every constructor
			// already builds, with a slot its tag does not select holding the
			// zero of that slot's type.
			sig.Result = ""
			sig.Results = append([]string{"int"}, inst.slots...)
		}
	}

	// A HOST SET WRITTEN THROUGH ITS IMPORT is the set of the module the alias
	// names, `strings.Replacer` ↦ `go/strings.Replacer`: ρ_m for a set atom, as
	// for a term (spec/typevalues.md §1a, ADR 0073). After the step above,
	// which resolved a variant crossing a boundary into its tag and slots.
	for _, m := range mods {
		for _, sig := range m.Sigs {
			res := func(ty string) string { return m.resolveSetAliases(ty) }
			sig.Result = res(sig.Result)
			for i := range sig.Results {
				sig.Results[i] = res(sig.Results[i])
			}
			for i := range sig.Params {
				sig.Params[i].Type = res(sig.Params[i].Type)
			}
		}
	}

	p := NewProgram()
	sort.Strings(unresolved)
	p.Unresolved = unresolved
	p.TargetDefined = overridden
	p.Sums = sums
	// The language's declarations are in the program ONCE, unqualified, which is
	// the spelling the compiler already produces — β on a map literal builds
	// `some` and `none` wherever the read occurs. A module finds them after its
	// own declarations because `resolve` leaves a name it does not define bare.
	// A DEFINITION INTRODUCES A FRESH NAME (a definitional extension: Shoenfield
	// 1967, §4.6). Σ_L, the language's signature, is the names J's reader emits
	// bare, so a module defining one does not extend the theory, it redefines a
	// symbol of it: resolution would capture every use of the construct in that
	// module (`if c { a } else { b }` reached the module's `if`, and a function
	// answered 7 whatever its argument), and in the root δ would ignore it beside
	// the target's (ADR 0002's ▷), a definition with no effect. Both are refused.
	for _, m := range mods {
		for _, n := range m.Order {
			if LanguageSignature[n] {
				return nil, nil, fmt.Errorf("%s defines %s, which is the language's own: J's reader "+
					"writes its constructs with these names (`if c { a } else { b }` is `if`, `a + b` is "+
					"`+`), so a definition would capture them in its module or be ignored beside the "+
					"target's. A definition introduces a fresh name; choose another",
					modLabel(m.Path), ShowName(n))
			}
		}
	}
	langOrder, langDefs := OptionSum().Defs()
	for _, n := range langOrder {
		p.Defs[n] = langDefs[n]
		p.Order = append(p.Order, n)
	}
	for _, m := range mods {
		for _, n := range m.Order {
			body, err := m.resolve(m.Defs[n], map[string]bool{}, mods)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", qualify(m.Path, n), err)
			}
			q := qualify(m.Path, n)
			if _, own := langDefs[q]; own {
				return nil, nil, fmt.Errorf("%s defines %s, which is the language's own. The main "+
					"module's names are unqualified, like the language's, so they would be one "+
					"name; define it in a `module … { … }`, where it shadows the language's",
					modLabel(m.Path), ShowName(q))
			}
			if _, dup := p.Defs[q]; dup {
				return nil, nil, fmt.Errorf("%s is defined twice", q)
			}
			p.Defs[q] = body
			p.Order = append(p.Order, q)
		}
	}
	for _, m := range mods {
		for n, sig := range m.Sigs {
			p.Sigs[qualify(m.Path, n)] = m.qualifySig(sig)
		}
	}
	for _, m := range mods {
		if !entryPaths[m.Path] {
			continue // a library's exports are not the program's entry points
		}
		for _, n := range m.Order {
			if m.Exports[n] {
				p.Exports = append(p.Exports, qualify(m.Path, n))
			}
		}
	}
	terms := make([]*Term, 0, len(entries))
	for _, e := range entries {
		t, err := e.mod.resolve(e.term, map[string]bool{}, mods)
		if err != nil {
			return nil, nil, err
		}
		terms = append(terms, t)
	}
	// VARIADIC CALLS, once, after resolution (spec/variadic.md §3): a call's
	// trailing arguments become the list. Every definition and every entry term.
	for _, m := range mods {
		if _, ok := m.Defs[SpreadName]; ok {
			return nil, nil, fmt.Errorf("%s defines %s, which is the language's: `f(xs...)` passes a "+
				"table as a variadic call's arguments (spec/variadic.md)", modLabel(m.Path), SpreadName)
		}
	}
	p.Variadic = variadic
	// CELLS, after the arms they flow through exist and after resolution,
	// where a call names the declaration whose cell parameters it fills
	// (core/cell.go, spec/local.md §3, §5); before the variadic rewrite, which
	// then gathers a call's references into its list like any arguments.
	p.Cells = cells
	// BORROWS (spec/local.md §2.1): the target's declarations, and every
	// definition whose body is a borrow scope. Whether a body is a unit depends
	// on its callees', so the definitions' are a least fixpoint: a body starts
	// with a result and loses it once every tail is known to be a unit.
	for changed := true; changed; {
		changed = false
		for _, q := range p.Order {
			d, ok := BorrowsOf(p.Defs[q], borrows)
			if !ok {
				continue
			}
			if old, seen := borrows[q]; !seen || old.Results != d.Results {
				borrows[q] = d
				changed = true
			}
		}
	}
	for _, q := range p.Order {
		t, err := translateCells(p.Defs[q], cells, borrows)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", q, err)
		}
		p.Defs[q] = t
	}
	for i, t := range terms {
		out, err := translateCells(t, cells, borrows)
		if err != nil {
			return nil, nil, err
		}
		terms[i] = out
	}
	for _, q := range p.Order {
		t, err := variadicCalls(p.Defs[q], variadic, false)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", q, err)
		}
		p.Defs[q] = t
	}
	for i, t := range terms {
		out, err := variadicCalls(t, variadic, false)
		if err != nil {
			return nil, nil, err
		}
		terms[i] = out
	}
	return p, terms, nil
}

// SpreadName is the form that passes a table as a variadic call's list, Go's
// `xs...` (spec/variadic.md).
const SpreadName = "spread"

// variadicCalls rewrites every call of a variadic declaration in t, whose
// fixed parameter counts v holds by qualified name (spec/variadic.md §3):
//
//	(f x₁ … xₖ a₁ … aₙ)      ⟶  (f x₁ … xₖ (array a₁ … aₙ))     the word a₁⋯aₙ
//	(f x₁ … xₖ (spread xs))  ⟶  (f x₁ … xₖ xs)                  a word one has
//
// A variadic f is a function of a list, f : X × T* → B, and a call with
// arguments is f applied to the word they spell: Go's spec builds "a new slice
// … whose successive elements are the actual arguments". `spread` stands only
// as a variadic call's sole list argument; mayspread says this term is one.
func variadicCalls(t *Term, v map[string]int, mayspread bool) (*Term, error) {
	if t == nil {
		return t, nil
	}
	isSpread := func(x *Term) bool {
		return x.Kind == KApp && len(x.Kids) > 0 && x.Kids[0].Kind == KName && x.Kids[0].Name == SpreadName
	}
	if isSpread(t) {
		if !mayspread {
			return nil, fmt.Errorf("%s: `xs...` passes a table as a variadic call's list, and stands "+
				"only as that call's argument after its fixed ones (spec/variadic.md)", ShowTerm(t))
		}
		if len(t.Kids) != 2 {
			return nil, fmt.Errorf("%s: `xs...` spreads one table", ShowTerm(t))
		}
		return variadicCalls(t.Kids[1], v, false)
	}
	if len(t.Kids) == 0 {
		return t, nil
	}
	k, variadic := -1, false
	if t.Kind == KApp && t.Kids[0].Kind == KName {
		k, variadic = v[t.Kids[0].Name]
	}
	if !variadic {
		out := *t
		out.Kids = make([]*Term, len(t.Kids))
		for i, c := range t.Kids {
			x, err := variadicCalls(c, v, false)
			if err != nil {
				return nil, err
			}
			out.Kids[i] = x
		}
		return &out, nil
	}
	name, args := t.Kids[0].Name, t.Kids[1:]
	if len(args) < k {
		return nil, fmt.Errorf("%s takes %d argument(s) before its list, and is given %d", ShowName(name), k, len(args))
	}
	kids := []*Term{t.Kids[0]}
	for _, a := range args[:k] {
		x, err := variadicCalls(a, v, false)
		if err != nil {
			return nil, err
		}
		kids = append(kids, x)
	}
	rest := args[k:]
	if len(rest) == 1 && isSpread(rest[0]) {
		x, err := variadicCalls(rest[0], v, true)
		if err != nil {
			return nil, err
		}
		return &Term{Kind: KApp, Kids: append(kids, x)}, nil
	}
	word := []*Term{Name("array")}
	for _, a := range rest {
		if isSpread(a) {
			return nil, fmt.Errorf("%s: `xs...` is the whole list of %s, so it stands alone after "+
				"the fixed arguments (spec/variadic.md)", ShowTerm(t), ShowName(name))
		}
		x, err := variadicCalls(a, v, false)
		if err != nil {
			return nil, err
		}
		word = append(word, x)
	}
	return &Term{Kind: KApp, Kids: append(kids, &Term{Kind: KApp, Kids: word})}, nil
}

type entry struct {
	mod  *Module
	term *Term
}

// OptionSum is the language's own `option`: ONE declaration, in `lang`, seen by
// every module after its own declarations (spec/data.md §5.5.1). It was a copy
// injected into every module, and the copies had to be reconciled by name
// wherever two met; with one declaration there is nothing to reconcile.
//
// A map read is `(option V)` (maps.md §4), and the compiler is what produces
// it — from β-tab on a literal, or from the host's own fallible read at a
// boundary. So `some` and `none` have to resolve in whatever module the read
// occurs in, and a program cannot be asked to import them any more than it can
// be asked to import `if`.
//
// It is an ORDINARY sum, so nothing downstream learns that maps exist: the
// constructors are definitions, and qualification, imports, δ, the occurrence
// counter and `case`'s reader desugaring all apply unchanged (sums.md).
//
// The Church encoding is polymorphic in the payload — `some = λp.λk. k 0 p` —
// so one declaration serves every V, and reduction has already made the term
// monomorphic by the time anything asks.
//
// It is `(variant (option T) (some T) none)`: a type constructor of one
// parameter, so a signature writes `(option int)` and the payload is found by
// substitution (spec/data.md §5.5.3). Inside a program the argument is never
// needed, because reduction inlines the constructor and the checker sees the
// payload's own type.
func OptionSum() *Sum {
	// `some` is the success constructor (spec/errors.md §2), so `try` reads a
	// map: (try (some v) (m k) …). Not relevant: a read may be ignored.
	return &Sum{Name: "option", Params: []string{"T"}, Variants: []Variant{
		{Name: "some", Payload: "T"},
		{Name: "none"},
	}, Success: "some"}
}

// LanguageSignature is Σ_L, the names the language owns: the ones J's reader and
// Load's expansions emit unqualified, so that no module may define one. It is
// the program's side of the rule a target file meets (emit's coreNames and
// langOps, which a test holds inside this set): a construct promoted to the
// language works on every target, and neither a target nor a program may
// declare it.
var LanguageSignature = map[string]bool{
	// control and binding (ADR 0015, 0017), and the connectives D erases
	"if": true, "let": true, "loop": true, "and": true, "or": true, "not": true, "cond": true,
	// the integer operators and equality (ADR 0026, match.md)
	"=": true, "+": true, "-": true, "*": true, "/": true, "%": true,
	"<": true, "<=": true, ">": true, ">=": true,
	// the Boolean operations, the shifts and the Euclidean remainder (ADR 0082)
	"&": true, "|": true, "⊕": true, "~": true, "<<": true, ">>": true, "mod": true,
	// tables, buffers and maps (tables.md, maps.md, ADR 0018)
	"array": true, "table": true, "len": true, "alloc": true, "build": true, "set": true,
	"map": true, "build-map": true, "insert": true, "keys": true,
	// strings (string-operations.md)
	"concat": true, "string-of": true,
	// a range on a term, the discard, and the crash `expect` expands into
	AscribeName: true, IgnoreName: true, "abandon": true,
	// the least and greatest elements of a range, static (ADR 0084's build)
	"inf": true, "sup": true,
}

// langSums are `lang`'s variant types, by key. The key of a `lang` declaration
// is its bare name, qualify("", name), which is why the main module — also
// unqualified — may not declare one.
var langSums = map[string]*Sum{"option": OptionSum()}

func newModule(path string) *Module {
	return &Module{Path: path, Uses: map[string]string{},
		Exports: map[string]bool{}, Pub: map[string]bool{}, Defs: map[string]*Term{},
		Sigs: map[string]*Sig{}, Sums: map[string]*Sum{}}
}

// partition splits a form list into module scopes. A file with no `(module …)`
// is one anonymous root module, which is why every existing program still loads.
func partition(forms []Form) ([]*Module, []entry, error) {
	root := newModule("")
	mods := []*Module{root}
	byPath := map[string]*Module{"": root}
	cur := root
	var entries []entry

	for _, f := range forms {
		switch f.Kind {
		case "module":
			m, ok := byPath[f.Name]
			if !ok {
				m = newModule(f.Name)
				mods = append(mods, m)
				byPath[f.Name] = m
			}
			cur = m
		case "use":
			if prev, ok := cur.Uses[f.Alias]; ok && prev != f.Name {
				return nil, nil, fmt.Errorf("%s binds %s to both %s and %s — "+
					"give one of them a different alias, `use … as …`",
					modLabel(cur.Path), ShowName(f.Alias), prev, f.Name)
			}
			cur.Uses[f.Alias] = f.Name
		case "sig":
			cur.Sigs[f.Name] = f.Sig
		case "export":
			for _, n := range f.Names {
				cur.Exports[n] = true
			}
		case "pub":
			cur.HasPub = true
			for _, n := range f.Names {
				cur.Pub[n] = true
			}
		case "def":
			if _, dup := cur.Defs[f.Name]; dup {
				return nil, nil, fmt.Errorf("%s is defined twice", qualify(cur.Path, f.Name))
			}
			cur.Defs[f.Name] = f.Term
			cur.Order = append(cur.Order, f.Name)
		case "provides":
			// A target fragment in a library file: read by emit.loadProvides, and
			// not part of the program (target-system.md §8).
		case "sum":
			// A sum declaration is DEFINITIONS. The constructors become ordinary
			// defs, so qualification, imports, delta and the occurrence counter
			// all apply without anything downstream learning that sums exist.
			if _, dup := cur.Sums[f.Name]; dup {
				return nil, nil, fmt.Errorf("sum %s is declared twice", qualify(cur.Path, f.Name))
			}
			cur.Sums[f.Name] = f.Sum
			order, defs := f.Sum.Defs()
			for _, n := range order {
				if _, dup := cur.Defs[n]; dup {
					return nil, nil, fmt.Errorf("%s is defined twice — sum %s declares a "+
						"variant of that name", qualify(cur.Path, n), f.Name)
				}
				cur.Defs[n] = defs[n]
				cur.Order = append(cur.Order, n)
			}
		case "prim", "target":
			return nil, nil, fmt.Errorf(
				"a program cannot declare %s; primitives are declared in targets/NAME.oro", f.Kind)
		case "term":
			entries = append(entries, entry{mod: cur, term: f.Term})
		}
	}
	return mods, entries, nil
}

// resolve rewrites every name in a term to its fully qualified form.
//
// Three cases, and the third is the one that keeps λ-bound variables safe: a
// name bound by an enclosing abstraction is never qualified, however it is
// spelled.
func (m *Module) resolve(t *Term, bound map[string]bool, mods []*Module) (*Term, error) {
	switch t.Kind {
	case KInt, KFloat, KStr, KBool, KBound:
		return t, nil

	case KName:
		if bound[t.Name] {
			return t, nil
		}
		// A RANGE IS A STATIC VALUE (spec/typevalues.md §1): it may be an
		// argument, consumed by staging, `inf` and `sup` (ADR 0084's build:
		// lib/bytes' codec). Law S refuses one a residual still holds (staged).
		if t.Name == ".." {
			return t, nil
		}
		if err := setInTerm(t.Name); err != nil {
			// A SET IS A VALUE OF THE LOGIC, which never prints (ADR 0049). Until
			// this, `..` was read as a path and refused as "`` is not imported".
			return nil, err
		}
		if i := strings.Index(t.Name, "."); i >= 0 {
			alias, rest := t.Name[:i], t.Name[i+1:]
			path, ok := m.Uses[alias]
			if !ok {
				// A qualified name in SOURCE always names an import. Letting
				// this fall through silently made `(use …)` a no-op whenever
				// the alias happened to equal the module path — `io.print-line`
				// resolved to itself and worked with no import at all, which
				// is precisely the "meaning depends on what is in scope"
				// that qualified imports exist to prevent (modules.md §3).
				return nil, fmt.Errorf("%s is not imported: add `use %s` or `use PATH as %s`",
					ShowName(alias), alias, ShowName(alias))
			}
			if err := checkExported(mods, path, rest); err != nil {
				return nil, err
			}
			return Name(qualify(path, rest)), nil
		}
		if _, ok := m.Defs[t.Name]; ok {
			return Name(qualify(m.Path, t.Name)), nil
		}
		return t, nil // a primitive, or free

	case KFn:
		inner := make(map[string]bool, len(bound)+len(t.Params))
		for k := range bound {
			inner[k] = true
		}
		for _, p := range t.Params {
			inner[p] = true
		}
		b, err := m.resolve(t.Body(), inner, mods)
		if err != nil {
			return nil, err
		}
		return Fn(t.Params, b), nil
	}

	kids := make([]*Term, len(t.Kids))
	for i, k := range t.Kids {
		r, err := m.resolve(k, bound, mods)
		if err != nil {
			return nil, err
		}
		kids[i] = r
	}
	return &Term{Kind: KApp, Kids: kids}, nil
}

// constructor is ρ_m on a constructor's spelling — the rule `resolve` applies to
// every other name, applied to a `case` pattern (sumRef). `c` is looked up in
// m's own declarations, `alias.c` in the module the alias imports, and nothing
// else is in scope: a constructor another module declares is reachable only
// through a `use`, exactly as a function it defines is.
func (m *Module) constructor(sp string, byPath map[string]*Module, mods []*Module) (ctorRef, error) {
	owner, local, err := m.scope(sp, byPath)
	if err == nil && owner != m {
		err = checkExported(mods, owner.Path, local)
	}
	if err != nil {
		return ctorRef{}, err
	}
	for _, s := range owner.Sums {
		if s.has(local) {
			return ctorRef{ref: sumRef{key: sumKey(owner.Path, s), sum: s},
				local: local, tag: sp + "#tag"}, nil
		}
	}
	if owner == m {
		for k, s := range langSums {
			if s.has(sp) {
				return ctorRef{ref: sumRef{key: k, sum: s}, local: sp, tag: sp + "#tag"}, nil
			}
		}
		for _, other := range mods {
			for _, s := range other.Sums {
				if other != m && s.has(sp) {
					return ctorRef{}, fmt.Errorf("case: %s is not a constructor in %s; it is "+
						"declared by %s in %s — import that module and write the pattern "+
						"through its alias, as for any name it defines",
						sp, modLabel(m.Path), sumKey(other.Path, s), modLabel(other.Path))
				}
			}
		}
	}
	return ctorRef{}, fmt.Errorf("case: %s is not a constructor of any variant type in scope. "+
		"A variant type is declared `set S = 'c(T) ∪ …` (ADR 0064 item 7)", tagJ(sp))
}

// instance resolves a result type that may be a variant type INSTANCE: a bare
// name for a declaration of no parameters, `F(A⃗)` for one of |A⃗|. It returns
// nil for a type that is not a variant at all, and the substituted declaration
// keyed applicatively, `a.result(int, string)`, for one that is.
func (m *Module) instance(q, ty string, byPath map[string]*Module) (*sumRef, error) {
	head, args, applied := Applied(ty)
	if !applied {
		head = ty
	}
	ref, ok := m.sumType(head, byPath)
	switch {
	case !ok && applied:
		return nil, fmt.Errorf("%s returns %s, but %s is not a parameterised variant type in "+
			"scope. Several results are a product, A × B (spec/data.md §10)", ShowName(q), ShowSet(ty), ShowSet(head))
	case !ok:
		return nil, nil
	case len(args) != len(ref.sum.Params) && !applied:
		return nil, fmt.Errorf("%s returns %s, which takes %d type argument%s — write (%s %s) "+
			"with its argument%s (spec/data.md §5.5.3)", q, head, len(ref.sum.Params),
			plural(len(ref.sum.Params), "", "s"), head, strings.Join(ref.sum.Params, " "),
			plural(len(ref.sum.Params), "", "s"))
	case len(args) != len(ref.sum.Params):
		return nil, fmt.Errorf("%s returns %s, but %s takes %d type argument%s, not %d",
			q, ty, ref.key, len(ref.sum.Params), plural(len(ref.sum.Params), "", "s"), len(args))
	}
	if !applied {
		ref.slots = ref.sum.SlotTypes(nil)
		return &ref, nil
	}
	return &sumRef{key: ref.key + "(" + strings.Join(args, ", ") + ")",
		sum: ref.sum.instantiate(args), slots: ref.sum.SlotTypes(args)}, nil
}

// sumType is ρ_m on a type spelling that may name a variant type: `result` in
// m's own declarations, `alias.result` through a `use`.
func (m *Module) sumType(sp string, byPath map[string]*Module) (sumRef, bool) {
	owner, local, err := m.scope(sp, byPath)
	if err != nil {
		return sumRef{}, false
	}
	if s, ok := owner.Sums[local]; ok {
		return sumRef{key: sumKey(owner.Path, s), sum: s}, true
	}
	if s, ok := langSums[sp]; ok && owner == m {
		return sumRef{key: sp, sum: s}, true
	}
	return sumRef{}, false
}

// definitionNamed reports whether a name written in one of m's signatures as a
// type resolves to a definition of the program, m's own or, through a `use`, an
// imported module's, and that definition's qualified name. A name ρ_m cannot
// resolve is not this rule's to judge.
func (m *Module) definitionNamed(sp string, byPath map[string]*Module) (string, bool) {
	owner, local, err := m.scope(sp, byPath)
	if err != nil || owner == nil {
		return "", false
	}
	if _, ok := owner.Defs[local]; ok {
		return qualify(owner.Path, local), true
	}
	return "", false
}

// sigTypes lists the names a signature writes in its types, parameters and
// results: every word of a canonical type spelling (`array int 0 255`,
// `result(FileInfo, error)`) that is neither a number nor an open element.
func sigTypes(sig *Sig) []string {
	tys := []string{sig.Result}
	tys = append(tys, sig.Results...)
	for _, pm := range sig.Params {
		tys = append(tys, pm.Type)
	}
	var names []string
	for _, ty := range tys {
		for _, w := range strings.FieldsFunc(ty, func(r rune) bool {
			return r == ' ' || r == '(' || r == ')' || r == ','
		}) {
			if w[0] == '?' || w[0] == '-' || (w[0] >= '0' && w[0] <= '9') {
				continue
			}
			names = append(names, w)
		}
	}
	return names
}

// scope is the half of ρ_m every namespace shares: a spelling names a member of
// m itself, or `alias.x` a member of the module the alias imports.
func (m *Module) scope(sp string, byPath map[string]*Module) (*Module, string, error) {
	i := strings.Index(sp, ".")
	if i < 0 {
		return m, sp, nil
	}
	alias, local := sp[:i], sp[i+1:]
	path, ok := m.Uses[alias]
	if !ok {
		return nil, "", fmt.Errorf("%s is not imported: add `use %s` or `use PATH as %s`",
			ShowName(alias), alias, ShowName(alias))
	}
	if byPath[path] == nil {
		return nil, "", fmt.Errorf("%s names module %s, which has no source, so it declares "+
			"no variant types", sp, path)
	}
	return byPath[path], local, nil
}

// checkExported enforces the export list of a module we can see. A module we
// cannot see is one the TARGET provides, and its surface is the target file's
// business rather than ours.
//
// A constructor's tag, `c#tag`, is exported exactly when `c` is: `#` is not an
// identifier character, so no program can write the tag, and it exists only as
// what `case` expands a pattern into.
func checkExported(mods []*Module, path, name string) error {
	name = strings.TrimSuffix(name, "#tag")
	for _, m := range mods {
		if m.Path != path {
			continue
		}
		// A MODULE'S INTERFACE IS WHAT IT MARKS `pub` (ADR 0078). Until then an
		// empty interface read as "no signature yet", so a module that hid every
		// name hid none.
		if !m.HasPub {
			return nil
		}
		if !m.Pub[name] {
			if _, defined := m.Defs[name]; defined || isSumMember(m, name) {
				return fmt.Errorf("module %s defines %s and does not make it `pub`: a declaration is private "+
					"to its module unless marked `pub` (ADR 0078)", path, ShowName(name))
			}
			return fmt.Errorf("module %s has no member %s", path, ShowName(name))
		}
		return nil
	}
	return nil
}

// markRecursive finds definitions reachable from their own bodies. core-0 §6
// makes this the side condition on termination: δ on a recursive definition does
// not terminate, so recursive definitions stay in the residual as target
// functions. This is g3's "recursive functions cannot be rules", arriving as a
// proof obligation.
// MarkRecursive is exported so a Target can build an Env.
func (e *Env) MarkRecursive() { e.markRecursive() }

func (e *Env) markRecursive() {
	for name := range e.Defs {
		if e.Prim[name] {
			// The target provides this name natively, so δ never unfolds the
			// definition and its cycle is unreachable — ADR 0002's "compiling
			// up". Marking it recursive would reject a program that builds.
			continue
		}
		seen := map[string]bool{}
		if e.reaches(name, name, seen) {
			e.Rec[name] = true
		}
	}
}

func (e *Env) reaches(from, target string, seen map[string]bool) bool {
	body, ok := e.Defs[from]
	if !ok {
		return false
	}
	var walk func(t *Term) bool
	walk = func(t *Term) bool {
		switch t.Kind {
		case KName:
			if t.Name == target {
				return true
			}
			if e.Prim[t.Name] || seen[t.Name] {
				return false
			}
			seen[t.Name] = true
			return e.reaches(t.Name, target, seen)
		case KInt, KFloat, KStr, KBool:
			return false
		}
		for _, k := range t.Kids {
			if walk(k) {
				return true
			}
		}
		return false
	}
	return walk(body)
}

// CheckDefs enforces the one restriction that makes δ safe without a side
// condition of its own (effects.md §4). Unfolding copies a definition's body to
// every occurrence, which is contraction; it is sound exactly when the body is a
// value. A λ is a value whatever its body does, so this rejects only a name
// bound to a computation — `(def x (print-line "a"))`, whose two occurrences
// would print twice. Every call-by-value language makes the same decision.
func (e *Env) CheckDefs() error {
	for _, name := range sortedKeys(e.Defs) {
		if e.pureTerm(e.Defs[name], map[string]bool{}) {
			continue
		}
		return fmt.Errorf("the body of %s is a computation, not a value, "+
			"so unfolding it would repeat its effects\n"+
			"  Give it an empty parameter list — fn %s() { … } — and apply it,\n"+
			"  or bind it in a block at the point of use.", ShowName(name), ShowName(name))
	}
	return nil
}

func sortedKeys(m map[string]*Term) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// pureTerm is the judgement of effects.md §3: may this term be copied, dropped,
// or moved without changing what the program does.
//
// The subtle case is the λ, and it is subtle in two directions at once.
//
// As an argument to a β-redex a λ is a VALUE — writing it does nothing, and its
// body's effects fire at each application, which the body already contains. So
// it stays opaque, and it must: `(fn (acc i) (print-line i))` judged impure
// would be let-bound as a bare λ and reach the emitter as an escaping closure.
//
// As an argument to a PRIMITIVE it is transparent, because the structural
// primitives — loop, loop2, cond, let — apply the λ they are given. Treating
// `(fold-range z n (fn (acc i) (print-line i)))` as pure because its third
// argument is a λ would license moving the whole loop into another loop.
// readsBoundTable reports whether a term reads a table through a BOUND
// variable — `(b i)` with `b` a parameter rather than a definition.
//
// That is the shape a buffer read has, and also the shape an array read has:
// nothing in the term distinguishes them, which is why the caller tests the
// destination rather than this. Recognising it at all is what keeps the rule
// narrow — an ordinary arithmetic argument is still substituted into an impure
// body, as it always was.
func (e *Env) readsBoundTable(t *Term) bool {
	if t == nil {
		return false
	}
	// NOT UNDER A λ. A read inside a λ runs where the λ is APPLIED, and
	// substituting the term moves no application, which is why β exempts an
	// argument that IS a λ. The same holds for a λ nested inside one: a sum's
	// constructor is `(fn (#k) (#k tag payload))`, whose `(#k …)` applies its
	// own parameter, and reading it as a table read let-bound every
	// conditional sum flowing into an arm with effects (sumofsums-2026-10-06).
	if t.Kind == KFn {
		return false
	}
	if t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KBound {
		return true
	}
	// AN OBSERVER APPLIED TO A BOUND VARIABLE reads a table through it too
	// (ADR 0069): `(bytes.Count b sep)` reads b as `(b i)` does, and moved into an
	// impure body it would read b after a store there. Which argument is the
	// table is the target's knowledge, so every bound argument is taken for one:
	// conservative, as the rule is.
	if t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName && e.Observe[t.Kids[0].Name] {
		for _, k := range t.Kids[1:] {
			if k.Kind == KBound {
				return true
			}
		}
	}
	for _, k := range t.Kids {
		if e.readsBoundTable(k) {
			return true
		}
	}
	return false
}

func (e *Env) pureTerm(t *Term, seen map[string]bool) bool {
	switch t.Kind {
	case KName, KInt, KFloat, KStr, KBool, KFn, KBound:
		return true // values
	case KApp:
		op := t.Op()
		for _, a := range t.Args() {
			// A λ ARGUMENT MAY BE APPLIED, by a primitive or by a λ head that
			// calls its parameter, so its body counts as run, through every λ
			// it returns. Taken for a value under a λ head, ((fn (k) (k a a))
			// (fn (s a) (fn (#k) (#k (Println s) a)))) was pure: calling the
			// variable k is "a value, not a call", and the redex a discarded
			// cell scope reduces to was weakened, its Println with it
			// (outcells-2026-10-09).
			b := a
			for b.Kind == KFn {
				b = b.Body()
			}
			if !e.pureTerm(b, seen) {
				return false
			}
		}
		switch op.Kind {
		case KFn:
			return e.pureTerm(op.Body(), seen) // applying a λ runs its body
		case KName:
			return e.pureName(op.Name, seen)
		}
		return e.pureTerm(op, seen)
	}
	return false
}

// pureName answers "does calling this thing do something", which is a different
// question from "is this name safe to move" — a name is always a value. Keeping
// the two apart is what makes the λ rule above sound.
func (e *Env) pureName(n string, seen map[string]bool) bool {
	if e.Prim[n] {
		return e.Pure[n]
	}
	body, ok := e.Defs[n]
	if !ok {
		return true // a free variable is a value, not a call
	}
	if seen[n] {
		return true // recursion contributes nothing new to a least fixed point
	}
	seen[n] = true
	for body.Kind == KFn {
		body = body.Body()
	}
	return e.pureTerm(body, seen)
}

// unfoldable reports whether δ applies to this name under this environment.
// langFoldable is which of the language's integer operators fold on two
// literals. Bitwise and shifts are not the language's at all (JavaScript
// truncates them to int32), and no float operator is here — ADR 0009 is the
// whole reason: Go folds `0.1+0.2` to `0.3` at compile time and
// `0.30000000000000004` at run time, because its untyped constants are
// arbitrary-precision.
var langFoldable = map[string]bool{
	"+": true, "-": true, "*": true, "/": true, "%": true,
	"<": true, "<=": true, ">": true, ">=": true, "=": true,
}

// Evaluate decides a closed obligation by evaluation, the third route an
// obligation has (refinements.md §3a): the goal with its arguments
// substituted, reduced by the reducer's own folding and nothing else — `len`
// of a table written as its graph, the language's integer operators on
// literals inside the word w (ADR 0009), `=`, `if`, and the connectives. It
// answers (value, true) when the goal reduces to a boolean literal, and
// (false, false) when it does not, which leaves the goal to the prover.
//
// A host declaration's `where` is decided on the residual by the refinement
// layer, whose fragment is linear: until this route it never saw
// `(len (array …))` as 6, nor `(% 6 2)` as 0, so NewReplacer's even count was
// refused on a literal (variadic-2026-10-08). A definition's `where` already
// had the route, through markWhere's normalisation.
func Evaluate(goal *Term, w Word) (bool, bool) {
	e := &Env{Defs: map[string]*Term{}, Prim: map[string]bool{}, Pure: map[string]bool{}, Word: w}
	for n := range langFoldable {
		e.Prim[n], e.Pure[n] = true, true
	}
	for _, n := range []string{"if", "len", "array", "let"} {
		e.Prim[n], e.Pure[n] = true, true
	}
	if !e.pureTerm(goal, map[string]bool{}) {
		return false, false
	}
	v, err := Normalize(goal, e, DefaultFuel)
	if err != nil || v.Kind != KBool {
		return false, false
	}
	return v.IsTrue(), true
}

// foldInt evaluates one of the language's integer operators on two literals,
// reporting whether it was safe to do so at all.
//
// TWO SIDE CONDITIONS, and both are ADR 0009 — staging must not change an
// answer.
//
//  1. THE RESULT MUST STAY INSIDE THE PORTABLE WINDOW. Compile time here is
//     Go's int64; run time on JavaScript is a float64 whose integers are exact
//     only to ±(2^53−1). So `(* 9007199254740991 2)` folds to a value the
//     runtime could not have produced, and is therefore left alone — where the
//     overflow analysis reports it against the operation the programmer wrote.
//     The operands are checked too: a literal outside the window was never a
//     value the language could represent.
//
//  2. DIVISION BY ZERO IS NOT FOLDED. It is a PRECONDITION, not a behaviour
//     (integers.md §5), and the refinement layer is what reports it with the
//     call site. Folding it would panic the compiler instead.
//
// The division semantics are the language's and were measured: truncation
// toward zero, and the remainder takes the DIVIDEND's sign. Go's own `/` and
// `%` on int64 are exactly that, so the fold and the emission agree by
// construction rather than by coincidence.
func foldInt(op string, a, b int64, w Word) (*Term, bool) {
	// AN ENV WITH NO WORD FOLDS NOTHING. A fold is a promise that compile time
	// and run time agree, and without a target there is no run time to agree
	// with (ADR 0009, per target since ADR 0026).
	if w == (Word{}) {
		return nil, false
	}
	lo, hi := w.Lo, w.Hi
	if a < lo || a > hi || b < lo || b > hi {
		return nil, false
	}
	switch op {
	case "<":
		return Bool(a < b), true
	case "<=":
		return Bool(a <= b), true
	case ">":
		return Bool(a > b), true
	case ">=":
		return Bool(a >= b), true
	case "=":
		return Bool(a == b), true
	}
	// EVERY OPERATION IS CHECKED, because a word can be all of int64: two
	// operands inside [−2^63, 2^63−1] can sum, differ or multiply to something
	// int64 cannot hold, and the range test below would then inspect a wrapped
	// value and pass. The window's 2^53 used to make + and − safe by width; a
	// target's word does not.
	var v int64
	switch op {
	case "+":
		v = a + b
		if (b > 0 && v < a) || (b < 0 && v > a) {
			return nil, false
		}
	case "-":
		v = a - b
		if (b < 0 && v < a) || (b > 0 && v > a) {
			return nil, false
		}
	case "*":
		// Checked by DIVIDING BACK, and MinInt64 · −1 separately: it is the one
		// product dividing back cannot see, since MinInt64 / −1 wraps too.
		v = a * b
		if (a != 0 && (v/a != b)) || (a == -1 && b == math.MinInt64) || (b == -1 && a == math.MinInt64) {
			return nil, false
		}
	case "/", "%":
		if b == 0 {
			return nil, false
		}
		// MinInt64 / −1 is 2^63, which no int64 holds.
		if a == math.MinInt64 && b == -1 {
			return nil, false
		}
		if op == "/" {
			v = a / b
		} else {
			v = a % b
		}
	default:
		return nil, false
	}
	if v < lo || v > hi {
		return nil, false
	}
	return &Term{Kind: KInt, Int: v}, true
}

func (e *Env) unfoldable(name string) bool {
	if e.Prim[name] || e.Rec[name] {
		return false
	}
	_, ok := e.Defs[name]
	return ok
}

const DefaultFuel = 1_000_000

type FuelError struct{ Term *Term }

func (f *FuelError) Error() string {
	return fmt.Sprintf("reduction did not terminate within the step limit; last term head: %s",
		head(f.Term))
}

func (f *FuelError) reclose(params []string) error {
	return &FuelError{Term: FnClosed(params, f.Term)}
}

// ArityError is application to the wrong number of arguments. It carries the
// term rather than a formatted string so that reclose can put names back.
type ArityError struct {
	Fn          *Term
	Want, Given int
}

func (a *ArityError) Error() string {
	// The term is printed LAST and introduced as context, because reclose may
	// have wrapped the offending function in its enclosing binders — which is
	// what supplies the names. "%s expects N" would then name the wrong λ.
	return fmt.Sprintf("arity: expects %d argument(s), given %d, in: %s", a.Want, a.Given, ShowTerm(a.Fn))
}

func (a *ArityError) reclose(params []string) error {
	return &ArityError{Fn: FnClosed(params, a.Fn), Want: a.Want, Given: a.Given}
}

// A recloser carries a term captured mid-reduction. Under a binder, that term's
// bound variables have no name yet and print as `#1.0`; each enclosing binder
// recloses as the stack unwinds, so the message that reaches the user is
// spelled with the names the source used.
type recloser interface{ reclose(params []string) error }

func head(t *Term) string {
	for t.Kind == KApp {
		t = t.Op()
	}
	return t.String()
}

// Normalize reduces t to normal form: no reducible β-redex and no name outside
// P. Reduction is normal order — leftmost-outermost, and under abstractions —
// which is what a partial evaluator needs: it must not get stuck on an argument
// it cannot evaluate, and it must reduce inside the function bodies that become
// loop bodies.
func Normalize(t *Term, e *Env, fuel int) (*Term, error) {
	f := &fuel
	nf, err := normalize(t, e, f)
	if err != nil {
		return nil, err
	}
	if err := e.staged(nf); err != nil {
		return nil, err
	}
	// Every consumer opens the residual by its hints, so the hints must be a
	// faithful naming of the indices — see hygiene.go.
	return hygienic(stripElim(nf)), nil
}

// stripElim removes the elimination marks left over a variable (see the
// #elim rule in normalizeStep): ((#elim x) a…) ⟶ (x a…).
func stripElim(t *Term) *Term {
	if t == nil || len(t.Kids) == 0 {
		return t
	}
	var kids []*Term
	for i, k := range t.Kids {
		n := stripElim(k)
		if i == 0 && t.Kind == KApp && isElimMark(n) {
			n = n.Kids[1]
		}
		if n != k && kids == nil {
			kids = append([]*Term(nil), t.Kids...)
		}
		if kids != nil {
			kids[i] = n
		}
	}
	if kids == nil {
		return t
	}
	c := *t
	c.Kids = kids
	return &c
}

// MaxDepth bounds how deep reduction recurses. Fuel bounds the number of
// steps, not their depth, and depth is what the stack pays for: Ω under a
// binder recursed about 210,000 frames before its fuel ran out, at the edge of
// Go's 1 GB stack, so 256 bytes more in normalize's frame overflowed it
// (local-2026-10-08). Exceeding the depth is the fuel's diagnosis, a reduction
// that does not end, and it bounds the stack whatever the frame's size.
var MaxDepth = 100_000

func normalize(t *Term, e *Env, fuel *int) (*Term, error) {
	e.depth++
	if e.depth > MaxDepth {
		e.depth--
		return nil, &FuelError{Term: t}
	}
	out, err := normalizeStep(t, e, fuel)
	e.depth--
	return out, err
}

func normalizeStep(t *Term, e *Env, fuel *int) (*Term, error) {
	if *fuel <= 0 {
		return nil, &FuelError{Term: t}
	}
	*fuel--

	switch t.Kind {
	case KInt, KFloat, KStr, KBool, KBound:
		return t, nil

	case KName:
		if e.unfoldable(t.Name) { // δ
			return normalize(e.Defs[t.Name], e, fuel)
		}
		return t, nil

	case KFn:
		// The CLOSED body. Reduction never opens, so it never re-closes, so a
		// colliding hint can never merge two variables. That is what makes the
		// representation buy anything.
		b, err := normalize(t.Closed(), e, fuel)
		if err != nil {
			if rc, ok := err.(recloser); ok {
				return nil, rc.reclose(t.Params)
			}
			return nil, err
		}
		return FnClosed(t.Params, b), nil

	case KApp:
		// A BINDER'S CLAIM, ITS SET CANONICAL NOW: the mark a range claim needs, as
		// D makes it for a set it can spell (desugar.go).
		if t.Op().Kind == KName && t.Op().Name == ClaimName && len(t.Kids) == 4 && t.Kids[1].Kind == KStr {
			set, err := normalize(t.Kids[2], e, fuel)
			if err != nil {
				return nil, err
			}
			ty := TypeTerm(set)
			if _, _, isRange := IntRangeBig(ty); isRange {
				return normalize(&Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(ClaimDef), t.Kids[1], Str(ty), t.Kids[3]}}, e, fuel)
			}
			return normalize(t.Kids[3], e, fuel)
		}
		// AN ASCRIPTION'S SET IS ITS CANONICAL SPELLING (desugar.go), once a
		// constant in it is unfolded.
		if t.Op().Kind == KName && t.Op().Name == AscribeName && len(t.Kids) == 3 && t.Kids[1].Kind != KStr {
			set, err := normalize(t.Kids[1], e, fuel)
			if err != nil {
				return nil, err
			}
			if ty := TypeTerm(set); ty != "" {
				return normalize(&Term{Kind: KApp, Kids: []*Term{t.Kids[0], Str(ty), t.Kids[2]}}, e, fuel)
			}
		}
		if e.Requires != nil && t.Op().Kind == KName {
			if t.Op().Name == RequireName && len(t.Kids) == 5 {
				a, err := normalize(t.Kids[4], e, fuel)
				if err != nil {
					return nil, err
				}
				// A LITERAL IN ITS RANGE is the obligation discharged, by
				// evaluation. One outside it keeps its mark.
				if a.Kind == KInt && e.InRange != nil && e.InRange(t.Kids[3].Str, a.Int) {
					return a, nil
				}
				return &Term{Kind: KApp, Kids: []*Term{t.Kids[0], t.Kids[1], t.Kids[2], t.Kids[3], a}}, nil
			}
			t = e.markRequires(t)
		}
		callee := ""
		if e.Wheres != nil && t.Op().Kind == KName {
			if _, ok := e.Wheres[t.Op().Name]; ok {
				callee = t.Op().Name
			}
		}
		if IsRequireWhere(t) {
			b, err := normalize(t.Kids[3], e, fuel)
			if err != nil {
				return nil, err
			}
			// A CONDITION A VALUE WAS SUBSTITUTED INTO after it was formed (β
			// of a let whose name it read) is reduced again when that value
			// left something to reduce: an obligation of its own, hoisted to
			// stand beside this one, or the length of a table that is no name
			// (a segment's, lib/bytes' get). Pure reasoning: the condition is
			// erased once decided.
			if cond := t.Kids[2]; needsReduction(cond) {
				var inner []*Term
				cond = hoistWheres(cond, &inner)
				if c, err := normalize(cond, e, fuel); err == nil {
					cond = c
				}
				m := &Term{Kind: KApp, Kids: []*Term{t.Kids[0], t.Kids[1], cond, b}}
				return wrapWheres(m, inner), nil
			}
			return withWhere(t, b), nil
		}
		op, err := normalize(t.Op(), e, fuel)
		if err != nil {
			return nil, err
		}
		args := t.Args()
		// A STATIC APPLICATION, (f T̄) with f a declaration over sets: δ to its
		// instance once every static argument has reduced to a set. One still a
		// variable waits for the β that gives it (Law 1 of spec/typevalues.md §5).
		if op.Kind == KName && e.Static[op.Name] > 0 && len(args) >= e.Static[op.Name] {
			if inst, rest, ok, err := e.instantiate(op.Name, args, fuel); err != nil {
				return nil, err
			} else if ok {
				if len(rest) == 0 {
					return inst, nil
				}
				return normalize(&Term{Kind: KApp, Kids: append([]*Term{inst}, rest...)}, e, fuel)
			}
		}
		// A WHERE-MARK IN OPERATOR POSITION is hoisted: the mark has no value of
		// its own, so `((#reqw d c M) a…)` is `(#reqw d c (M a…))`, and β, the
		// commuting conversions and every other rule then see M.
		if IsRequireWhere(op) {
			inner, err := normalize(&Term{Kind: KApp, Kids: append([]*Term{op.Kids[3]}, args...)}, e, fuel)
			if err != nil {
				return nil, err
			}
			return withWhere(op, inner), nil
		}

		// AN ELIMINATION A CELL'S TRANSLATION CLAIMED, ((#elim op) λ): op's
		// normal form must be data, so that λ runs once, now (core/cell.go).
		// A function instead is a closure over the scope's cells, refused.
		if isElimMark(op) {
			// A VARIABLE IS NOT DECIDED YET: a λ's body is normalized before
			// the λ is applied, and β renormalizes it once the variable has a
			// value. One left at the end is a parameter or an impure let's
			// value, which a first-order residual holds only as data, and
			// Normalize strips its mark (stripElim).
			if v := op.Kids[1]; v.Kind == KBound || (v.Kind == KName && !e.Prim[v.Name]) {
				kids := []*Term{op}
				for _, a := range args {
					na, err := normalize(a, e, fuel)
					if err != nil {
						return nil, err
					}
					kids = append(kids, na)
				}
				return &Term{Kind: KApp, Kids: kids}, nil
			}
			if !e.isData(op.Kids[1]) {
				return nil, fmt.Errorf("a function that mentions a cell is passed to %s, which is "+
					"a function and may run it any number of times, or later, so the cell's order "+
					"would not be the program's. A cell may be read and written in the scope, a case "+
					"arm, a loop and the elimination of a value; callbacks over a scope's cells are "+
					"tier 1, not built (spec/local.md §3)", ShowTerm(op.Kids[1]))
			}
			return normalize(&Term{Kind: KApp, Kids: append([]*Term{op.Kids[1]}, args...)}, e, fuel)
		}

		// β-tab — THE SECOND CLAUSE OF β, not a fourth rule.
		//
		//	((array 10 20 30) 2)  ⟶  30      a function given by its GRAPH
		//	((table n f) i)       ⟶  (f i)   a function given by a RULE
		//
		// A function can be written down two ways, so application has two cases.
		// Ordinary β handles the intensional presentation — substitute into the
		// body. β-tab handles the extensional one — look the argument up. Same
		// judgement, *apply this function to this argument*, and calling it "the
		// extensional counterpart of β" is not a flourish: extensionality is
		// precisely that a function is determined by its input/output pairs
		// (tables.md §4.1).
		//
		// It needs NO constant folder. Looking up element k of a literal table
		// is not arithmetic and cannot disagree with anything at runtime, so
		// ADR 0009 has nothing to say about it — which is why this is the easy
		// path and folding waits for integers to need it (tables.md §4.3).
		if idx, ok := e.betaTab(op, args, fuel); ok {
			return normalize(idx, e, fuel)
		}
		// A FUNCTION UNDER AN OBLIGATION IS STILL THAT FUNCTION: applying
		// (#reqw c t) is applying t, under c, as `len` reads through one. A
		// segment's formation obligation stands around its rule (segment), and
		// a codec's around its tuple, which is eliminated by applying it.
		if IsRequireWhere(op) {
			in, ms := peelWheres(op)
			if in.Kind == KFn || isForm(in, e, "tuple") ||
				((isForm(in, e, "table") || isForm(in, e, "array")) && len(args) == 1) {
				inner, err := normalize(&Term{Kind: KApp, Kids: append([]*Term{in}, args...)}, e, fuel)
				if err != nil {
					return nil, err
				}
				return wrapWheres(inner, ms), nil
			}
		}
		if op.Kind == KName && op.Name == SegmentName && len(args) == 3 {
			return e.segment(args, fuel)
		}
		if op.Kind == KName && op.Name == AdoptName && len(args) == 2 && args[1].Kind == KFn {
			return e.adopt(args, fuel)
		}
		if op.Kind == KName && op.Name == OwnedName && len(args) == 1 {
			return e.owned(args[0], fuel)
		}

		// `ignore` IS !_A : A → 1, the unique arrow into the terminal object
		// (spec/errors.md §7.3). A relevant calculus cannot define it, since
		// `(fn (x) (tuple))` is the λ relevance refuses, so the language has it
		// as a constant, and its reduction is the universal property: any two
		// arrows into 1 are equal, so on a value whose tails are tuples it is
		// the eliminator with a constant body, and otherwise the value's
		// effects followed by the unit.
		//
		//	(ignore e)  ⟶  (e (fn (x̄) (tuple)))      e's tails n-tuples
		//	(ignore e)  ⟶  (tuple)                   e pure
		//	(ignore e)  ⟶  (let e (fn (_) (tuple)))  e impure (ADR 0010)
		if op.Kind == KName && op.Name == IgnoreName && e.Prim[IgnoreName] {
			if len(args) != 1 {
				return nil, fmt.Errorf("ignore takes one term, the value it discards; got %d", len(args))
			}
			v, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			if n, ok := e.tupleTails(v, false); ok {
				return normalize(App(v, Fn(e.freshNames(n, v), Unit())), e, fuel)
			}
			if e.pureTerm(v, map[string]bool{}) {
				return Unit(), nil
			}
			out := App(Name("let"), v, Fn(e.freshNames(1, v), Unit()))
			if ab, ok := e.absorb(out.Kids); ok {
				return ab, nil
			}
			return out, nil
		}

		// CASE-OF-CASE, and it is what makes a DYNAMIC sum cost nothing.
		//
		//	((if c A B) k…)  ⟶  (if c (A k…) (B k…))
		//
		// Prawitz's commuting conversion; GHC's case-of-case; and for us the
		// rule sums-research.md §0.1 said was one step away from free. A sum
		// whose tag is known reduces away by β alone, but one whose tag depends
		// on runtime data gets STUCK as `((if c A B) F G)` — the constructor is
		// under the branch and the eliminator is outside it, so neither can see
		// the other. Pushing the eliminator INTO both branches reunites each
		// constructor with it, and then β finishes the job: no tag, no closure,
		// no allocation, and no runtime dispatch that the `if` was not already
		// doing.
		//
		// Only when every argument is PURE. The rule duplicates the arguments
		// into both branches, and ADR 0010 denies contraction for an impure
		// term — duplicating one would run an effect twice. Left stuck, an
		// impure eliminator is reported by the emitter rather than silently
		// mis-ordered.
		//
		// The known hazard is code growth: `k` appears twice, so nested cases
		// multiply. GHC's answer is join points, and `again` IS one
		// (types-direction.md §6) — which is the direction if this ever bites.
		// The LET companion, and the nested test is what demanded it:
		//
		//	((let v (fn (x) B)) k…)  ⟶  (let v (fn (x) (B k…)))
		//
		// The same commuting conversion, one constructor over. It is needed
		// because β itself puts a `let` between a constructor and its
		// eliminator: a shared subterm that is not duplicable gets let-bound
		// (ADR 0010), and the eliminator then sits outside a binder rather than
		// outside an `if`. Handling only `if` reduced two levels of a
		// three-level nest and stopped.
		//
		// So the rule is not "case-of-case" so much as: PUSH AN ELIMINATOR
		// THROUGH ANYTHING β CAN LEAVE IN OPERATOR POSITION, which in this
		// language is `if`, `let` — and a host call with several results,
		// below, which was missed.
		if isLetApp(op, e) && allPure(args, e) {
			lam := op.Kids[2]
			// THE BINDER KEEPS ITS NAME unless that would capture a free name of
			// what moves under it: hygiene asks no more, and the analyses key a
			// binding's facts on its name, so a program's `c` renamed `#c` lost
			// them and freq's digit count was summarised six times over
			// (storeorder-2026-10-15). A version keeps its mark (VersionPrefix).
			c := lam.Params[0]
			for _, a := range args {
				if occurrences(a, c) > 0 {
					c = "#c"
					if strings.HasPrefix(lam.Params[0], VersionPrefix) {
						c = VersionPrefix + "c"
					}
					break
				}
			}
			inner := lam.OpenWith([]*Term{Name(c)})
			app := append([]*Term{inner}, args...)
			out := &Term{Kind: KApp, Kids: []*Term{op.Kids[0], op.Kids[1],
				Fn([]string{c}, &Term{Kind: KApp, Kids: app})}}
			return normalize(out, e, fuel)
		}

		// THE N-ARY LET: a host call with several results, eliminated by its
		// continuation (values.md, multiPrimCall), IS a let binding each result:
		//
		//	(((p a…) (fn (x̄) M)) k…)  ⟶  ((p a…) (fn (x̄) (M k…)))
		//
		// It is the let rule above at arity n, and it was missing, so a function
		// that returned a tuple BUILT from such a call could not be taken apart by
		// its caller: `(def split (x) (let (tuple h l) (b.Mul64 x 3) (tuple l h)))`
		// left `(((Mul64 …) (fn (h l) (tuple l h))) (fn (a c) …))` stuck
		// (u128-2026-09-23). The same conditions hold for the same reasons: the
		// host call still runs once and first, the eliminator is pure so moving
		// it inside reorders no effect (ADR 0010), and the binders are fresh.
		if isMultiApp(op, e) && allPure(args, e) {
			lam := op.Kids[1]
			names := e.freshNames(len(lam.Params), append([]*Term{lam}, args...)...)
			fresh := make([]*Term, len(lam.Params))
			for i := range names {
				fresh[i] = Name(names[i])
			}
			inner := lam.OpenWith(fresh)
			app := append([]*Term{inner}, args...)
			out := &Term{Kind: KApp, Kids: []*Term{op.Kids[0], Fn(names, &Term{Kind: KApp, Kids: app})}}
			return normalize(out, e, fuel)
		}

		if isIfApp(op, e) && allPure(args, e) {
			br := func(b *Term) *Term {
				kids := make([]*Term, 0, len(args)+1)
				return normalizeLater(append(append(kids, b), args...))
			}
			out := &Term{Kind: KApp, Kids: []*Term{
				op.Kids[0], op.Kids[1], br(op.Kids[2]), br(op.Kids[3])}}
			return normalize(out, e, fuel)
		}

		// A RESIDUAL LET OF A DUPLICABLE VALUE IS β: (let v k) is (k v), and a
		// residual `let` exists only to keep an effect in place or to share a
		// computation (ADR 0010). A literal, a name or a λ needs neither. The
		// commuting conversions build a `let` directly, so a value that folds
		// afterwards, (+ 1 1) to 2, was left bound (local-2026-10-08).
		// (Each rule below is a function of its own, so normalize's frame does
		// not grow: reduction recurses as deep as its fuel.)
		if isResidualLet(op, args, e) {
			return e.reduceLet(op, args, fuel)
		}
		// A SELECTION OF A LOOP: ((loop F z…) (fn (x₁ … xₙ) σ)), σ a variable xⱼ
		// or a tuple of the λ's own variables, ⟶ the loop with σ at each exit.
		// A loop's value is its taken exit's, so σ ∘ loop = loop with σ at its
		// exits. Only a selection moves: its body is variables, so it holds no
		// `again` the loop would capture and no code the loop would repeat; any
		// other eliminator stays where it is, ADR 0031's join point. A state
		// scope's translation selects its own state off a loop this way: the
		// outermost keeps the value (πⱼ), an inner one drops its binders and
		// keeps the outer state (spec/local.md §2.1, §4).
		if len(args) == 1 && isLoopTerm(op, e) {
			if sel, n, ok := selection(args[0]); ok {
				if body, ok := e.selectTails(op.Kids[1].Closed(), sel, n); ok {
					lam := op.Kids[1]
					loop := &Term{Kind: KApp, Kids: append([]*Term{op.Kids[0],
						{Kind: KFn, Params: lam.Params, Kids: []*Term{body}}}, op.Kids[2:]...)}
					return normalize(loop, e, fuel)
				}
			}
		}
		// A SELECTION OF A REGION, ((build n F) σ) ⟶ build with σ at its body's
		// tails: a region's value is its body's, with its buffer frozen, which
		// a selection does not touch. A buffer's scope inside an outer state
		// scope yields the outer state with its value, and this takes it off.
		if len(args) == 1 && op.Kind == KApp && len(op.Kids) == 3 && op.Kids[0].Kind == KName &&
			(op.Kids[0].Name == "build" || op.Kids[0].Name == "build-map") && e.Prim[op.Kids[0].Name] &&
			op.Kids[2].Kind == KFn {
			if sel, n, ok := selection(args[0]); ok {
				if moved, ok := e.selectTails(op, sel, n); ok {
					return normalize(moved, e, fuel)
				}
			}
		}

		if op.Kind == KFn { // β
			if len(op.Params) != len(args) {
				return nil, &ArityError{Fn: op, Want: len(op.Params), Given: len(args)}
			}
			subs := make([]*Term, len(args))
			var bound []struct {
				name string
				val  *Term
			}
			// recount marks the bindings made ONLY because a pure value occurred
			// more than once: the impure and the table-read bindings are not in
			// it, and never will be substituted.
			recount := map[string]bool{}
			// Names already chosen for bindings of THIS application, so two
			// parameters cannot be renamed onto each other.
			used := map[string]bool{}
			// RELEVANCE (spec/errors.md §7.2): a program's binder that meets a
			// value of a relevant type must use it on every path of the body
			// as written, which is the λ before reduction.
			src := t.Op()
			if src.Kind == KName && e.unfoldable(src.Name) {
				src = e.Defs[src.Name]
			}
			if src.Kind != KFn || len(src.Params) != len(op.Params) {
				src = op
			}
			// Which arguments are λs: a parameter holding one, applied to a λ,
			// is a static function, not an eliminator (core/work.go).
			fnArgs := make([]bool, len(args))
			for i, a := range args {
				fnArgs[i] = a.Kind == KFn
			}
			relevant := func(i int, val *Term) error {
				p := op.Params[i]
				if !programBinder(p) || !e.relevant(val, false) ||
					e.usedOnEveryPath(src.Closed(), 0, i) {
					return nil
				}
				return relevanceError(p, args[i])
			}
			for i, p := range op.Params {
				if e.pureTerm(args[i], map[string]bool{}) {
					if err := relevant(i, args[i]); err != nil {
						return nil, err
					}
				}
				// Impure: never substituted. Bound here, at the application
				// site, which is where the programmer wrote it — at its
				// original loop depth and under its original guards. Binding
				// at the USE site instead would be the bug. (effects.md §4)
				// A TABLE READ IS NOT SUBSTITUTED INTO AN IMPURE BODY.
				//
				// `pureTerm` answers "value" for every bound variable, so
				// `(b 0)` — a buffer read — is judged pure and may be moved.
				// ADR 0018 says a buffer read is IMPURE and ADR 0010 says an
				// impure argument is never substituted, precisely so that
				// exchange is denied; without this a swap
				//
				//	(let (b 0) (fn (vx) (let (b 1) (fn (vy)
				//	  (set (set b 0 vy) 1 vx)))))
				//
				// emits as `b[0] = b[1]; b[1] = b[0]` and silently copies.
				//
				// The term cannot say which bound variables are buffers — an
				// ARRAY read has the identical shape and is genuinely pure — so
				// the test is on the DESTINATION instead: a read may move freely
				// into a body that has no effects to be reordered against, and
				// not into one that has. That is exactly the property at stake,
				// and it is decidable here because the body is in hand.
				//
				// Testing the OPERAND instead — "any application of a bound
				// variable is impure" — was tried and is wrong: a rule-table's
				// rule reads its parameter table, so `(table n f)` becomes
				// impure, is no longer substituted, and reaches the backend
				// unfused. `dot` and `smooth` on Java stop compiling.
				//
				// A λ IS EXEMPT, for the reason pureTerm gives: as an argument it
				// is a VALUE, and the read inside it runs at each APPLICATION,
				// which the body already contains at the place the programmer
				// wrote it. Substituting a λ moves no evaluation, so it cannot
				// move a read across a store. Without the exemption a
				// comparator `(fn (u v) (< (cmp (cs u) (cs v)) 0))` handed to a
				// sort that fills a buffer was let-bound as a bare λ — and so
				// was the continuation handed to a dictionary of host
				// operations, which reached the emitter as an "escaping
				// closure" (tally-2026-09-11). freq.oro escaped only because
				// its comparator hides the read behind a call to `wcmp`.
				if args[i].Kind != KFn && e.readsBoundTable(args[i]) && !e.pureTerm(op.Body(), map[string]bool{}) {
					na, err := normalize(args[i], e, fuel)
					if err != nil {
						return nil, err
					}
					nm := e.bindName(p, op.Params, used)
					used[nm] = true
					bound = append(bound, struct {
						name string
						val  *Term
					}{nm, na})
					subs[i] = Name(nm)
					continue
				}
				// A VERSION USED ONCE, FIRST: let x = e in E[x] is E[e] when E
				// evaluates its hole first and unconditionally, with nothing
				// impure before it, Moggi's let-β for a linear use in evaluation
				// position. It neither contracts, weakens nor exchanges, so it
				// holds of an impure e too (ADR 0010), and a table read moves
				// across no store. It is applied to the versions the state
				// translation binds as it threads a buffer, a store's or a
				// borrow's (spec/local.md §2.1), named VersionPrefix: a program's
				// own binding of an impure value keeps its name, which is what the
				// analyses key a postcondition and an exit's bound on
				// (storeorder-2026-10-15 §3).
				if strings.HasPrefix(p, VersionPrefix) && boundOccurrences(op.Closed(), 0, i) == 1 &&
					e.firstUse(op.Closed(), 0, i, args[i]) {
					if err := relevant(i, args[i]); err != nil {
						return nil, err
					}
					subs[i] = args[i]
					continue
				}
				if !e.pureTerm(args[i], map[string]bool{}) {
					na, err := normalize(args[i], e, fuel)
					if err != nil {
						return nil, err
					}
					if err := relevant(i, na); err != nil {
						return nil, err
					}
					nm := e.bindName(p, op.Params, used)
					used[nm] = true
					bound = append(bound, struct {
						name string
						val  *Term
					}{nm, na})
					subs[i] = Name(nm)
					continue
				}
				// One evaluation or none: substituting duplicates neither code nor
				// work. An occurrence under a λ applied many times, a loop's body
				// or a table's rule, is ω: there e would run per iteration where
				// the program wrote it once (ADR 0071, core/work.go), so it takes
				// the many-occurrence path below, which substitutes a value and
				// binds anything else where it was written.
				if e.workOccurrences(op.Closed(), 0, i, fnArgs) <= 1 {
					subs[i] = args[i]
					continue
				}
				// More than one. Normalise the argument to find out what it is —
				// this is the only place reduction is not lazy, and it is what
				// makes the classification possible at all.
				na, err := normalize(args[i], e, fuel)
				if err != nil {
					return nil, err
				}
				// LET-FLOATING TO EXPOSE A VALUE: (let a (fn (x) V)), V a value,
				// is bound as x := a here, where the argument is evaluated, and V
				// substituted (Peyton Jones, Partain and Santos 1996, §3.1). A
				// static pair whose length is computed, (let (alen p) (fn (n)
				// (fn (sel) …))), is a λ once its length is bound, and fuses; the
				// length is computed once, where the program wrote it. A floated
				// binder is pure, so the recount substitutes it back when it is
				// left with one evaluation, as β's one-occurrence rule would.
				if v, fl, ok := e.floatToValue(na); ok {
					for _, f := range fl {
						nm := e.freshFor(f.hint, op, args, used)
						used[nm] = true
						recount[nm] = true
						bound = append(bound, struct {
							name string
							val  *Term
						}{nm, f.val})
						v = subst(v, map[string]*Term{f.name: Name(nm)})
					}
					na = v
				}
				if duplicable(na) || e.duplicableTable(na) {
					subs[i] = na
					continue
				}
				nm := e.bindName(p, op.Params, used)
				used[nm] = true
				recount[nm] = true
				bound = append(bound, struct {
					name string
					val  *Term
				}{nm, na})
				subs[i] = Name(nm)
			}
			body, err := normalize(op.OpenWith(subs), e, fuel)
			if err != nil {
				return nil, err
			}
			// THE RECOUNT. The occurrences above were counted in the body BEFORE
			// it was normalised, and normalising can remove some: a sum's
			// eliminator names its payload in every arm, and folding the known
			// tag keeps one arm. So a pure value bound for occurring twice may
			// occur once, or not at all, in the body that is left. Then binding
			// it serves nothing, and it is in the way: with the payload a
			// conditional, `(let (if c A B) (fn (p) (p k…)))` leaves an
			// eliminator applied to a VARIABLE, which neither commuting
			// conversion reaches, and a sum of sums stops reducing
			// (sumofsums-2026-10-06). So substitute it, which is β's own
			// one-occurrence rule applied after the fact, and normalise again so
			// the conversions see the `if` in operator position. Sound for the
			// reason that rule is: the value is pure, and one occurrence
			// duplicates nothing (ADR 0010).
			again := map[string]*Term{}
			kept := bound[:0]
			for _, b := range bound {
				if recount[b.name] {
					switch occurrences(body, b.name) {
					case 0:
						continue // pure and unread: dropped, as β drops an unused pure argument
					case 1:
						// Once in the text, and once in evaluation: not under a
						// loop's body or a table's rule (ADR 0071).
						if e.workNamed(body, b.name) <= 1 {
							again[b.name] = b.val
							continue
						}
					}
				}
				kept = append(kept, b)
			}
			bound = kept
			if len(again) > 0 {
				if body, err = normalize(subst(body, again), e, fuel); err != nil {
					return nil, err
				}
			}
			if callee != "" {
				if body, err = e.markWhere(callee, subs, args, body, fuel); err != nil {
					return nil, err
				}
			}
			// Wrap innermost-last so the bindings nest in source order.
			for i := len(bound) - 1; i >= 0; i-- {
				// η FOR PRODUCTS: a value whose every tail is an n-tuple is
				// taken apart where it is bound, so each `(r k)` in the body is
				// a β-redex rather than an eliminator applied to a variable.
				// Read or not: r = (π₁ r, …, πₙ r) holds of every r, so an
				// unread binding keeps its value's effects and drops its
				// components, where it reached lowering as a closure.
				//
				// n = 0 is η FOR THE TERMINAL OBJECT: r = () for every r : 1,
				// so a binding of a unit, `ignore`'s value under `seq`, is
				// taken apart into no components (spec/errors.md §7.3).
				if n, ok := e.tupleTails(bound[i].val, false); ok && n != 1 {
					cs := e.freshNames(n, body, bound[i].val)
					comps := []*Term{Name(TupleBinder)}
					for _, c := range cs {
						comps = append(comps, Name(c))
					}
					tup := Fn([]string{TupleBinder}, &Term{Kind: KApp, Kids: comps})
					k := Fn(cs, subst(body, map[string]*Term{bound[i].name: tup}))
					if body, err = normalize(App(bound[i].val, k), e, fuel); err != nil {
						return nil, err
					}
					continue
				}
				body = App(Name("let"), bound[i].val, Fn([]string{bound[i].name}, body))
				if out, ok := e.absorb(body.Kids); ok {
					body = out
				}
			}
			return body, nil
		}
		// The conditional on a known condition. It was the ONLY evaluation
		// reduction performed until `=` on two integer literals joined it above
		// (2026-08-22, for sums) — so the pair of them is now the whole of it,
		// and it is worth being exact about why neither violates "no primitive
		// is ever evaluated" (state.md §3): `if` and `=` are the LANGUAGE's
		// names, injected into every target, and `true`/`false` are not
		// primitive applications. Nothing about the target is being decided
		// here (booleans.md §4.3). `(go.+ 1 2)` still does not fold.
		//
		// Sound for an IMPURE untaken branch, and for a different reason than
		// β's. β may not drop an impure argument because the argument would
		// have run; here the branch genuinely does not run. The structural
		// rules are not involved.
		//
		// ADR 0009 is satisfied trivially: boolean algebra is exact, so there
		// is no compile-time/runtime discrepancy to preserve.
		// `=` ON TWO INTEGER LITERALS folds, and this is the first entry in the
		// constant-folding table tables.md predicted — where `((array 1 2 3) 1)
		// → 2` and `(go.+ 1 2) → 3` are the same kind of step, not new rules.
		//
		// It is here because SUMS need it. A sum's tag is a literal after
		// reduction, so `(case (ok n) …)` reduced to `(if (= 0 0) … …)` — the
		// sum had vanished and left a tautological test behind, which is a
		// static cost the two-level language says should not exist.
		//
		// Narrow deliberately: integers only, `=` only. ADR 0009 permits it
		// because integer equality inside the portable window is bit-identical
		// on every target — the thing that is NOT true of float arithmetic,
		// which is why Go folds `0.1+0.2` two different ways and why nothing
		// here folds a float.
		//
		// GENERALISED to the rest of integer arithmetic once the operators
		// became the LANGUAGE's. That order matters: folding `go.+` would have
		// meant assuming a particular host's semantics, and `+` has semantics
		// the language defines and integers.md verified on all four.
		if op.Kind == KName && langFoldable[op.Name] && e.Prim[op.Name] && len(args) == 2 {
			a, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			b, err := normalize(args[1], e, fuel)
			if err != nil {
				return nil, err
			}
			// A where-mark on an operand is hoisted, so the fold still sees the
			// literal the mark wraps (ADR 0028).
			if IsRequireWhere(a) || IsRequireWhere(b) {
				ia, ma := peelWheres(a)
				ib, mb := peelWheres(b)
				inner, err := normalize(&Term{Kind: KApp, Kids: []*Term{op, ia, ib}}, e, fuel)
				if err != nil {
					return nil, err
				}
				return wrapWheres(wrapWheres(inner, mb), ma), nil
			}
			if a.Kind == KInt && b.Kind == KInt {
				if out, ok := foldInt(op.Name, a.Int, b.Int, e.Word); ok {
					return out, nil
				}
			}
			// A COMPARISON OF CLOSED INTEGERS FOLDS EXACTLY, a literal past the
			// word included (2^64 − 1 is staged as 18446·10¹⁵ + …): its value is
			// a Boolean, which every target holds, so no answer can change
			// (ADR 0009 governs integers a fold would produce, and this produces
			// none). lib/bytes' width of a 64-bit codec is decided so.
			if e.Word != (Word{}) {
				if out, ok := foldClosedComparison(op.Name, a, b); ok {
					return out, nil
				}
			}
			if out, ok := e.absorb([]*Term{op, a, b}); ok {
				return out, nil
			}
			return &Term{Kind: KApp, Kids: []*Term{op, a, b}}, nil
		}
		// inf and sup ARE A RANGE'S ENDS: on lo..hi with literal ends, the least
		// and the greatest element, the order's infimum and supremum, which a
		// finite range attains. Static: a range has no run-time value, so either
		// folds here or the residual is refused (Law S, staged).
		if op.Kind == KName && (op.Name == "inf" || op.Name == "sup") && e.Prim[op.Name] && len(args) == 1 {
			na, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			if na.Kind == KApp && len(na.Kids) == 3 && na.Kids[0].Kind == KName && na.Kids[0].Name == ".." {
				lo, err := normalize(na.Kids[1], e, fuel)
				if err != nil {
					return nil, err
				}
				hi, err := normalize(na.Kids[2], e, fuel)
				if err != nil {
					return nil, err
				}
				l, okl := closedBig(lo)
				h, okh := closedBig(hi)
				if okl && okh && l.Cmp(h) <= 0 {
					if op.Name == "inf" {
						return lo, nil
					}
					return hi, nil
				}
			}
			return &Term{Kind: KApp, Kids: []*Term{op, na}}, nil
		}
		// `len` on a table whose length is known. See tableLen.
		if op.Kind == KName && op.Name == "len" && e.Prim["len"] && len(args) == 1 {
			// The ARGUMENT has to be normalised first: `(len a)` where `a` is a
			// definition arrives as a name, and δ is what turns it into the
			// `(array …)` whose length is visible.
			na, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			if IsRequireWhere(na) {
				in, ms := peelWheres(na)
				inner, err := normalize(&Term{Kind: KApp, Kids: []*Term{op, in}}, e, fuel)
				if err != nil {
					return nil, err
				}
				return wrapWheres(inner, ms), nil
			}
			if n, ok := e.tableLen([]*Term{na}); ok {
				return normalize(n, e, fuel)
			}
			if out, ok := e.absorb([]*Term{op, na}); ok {
				return out, nil
			}
			return &Term{Kind: KApp, Kids: []*Term{op, na}}, nil
		}

		var cond *Term
		if op.Kind == KName && op.Name == "if" && e.Prim["if"] && len(args) == 3 {
			c, err := normalize(args[0], e, fuel)
			if err != nil {
				return nil, err
			}
			if IsRequireWhere(c) {
				in, ms := peelWheres(c)
				inner, err := normalize(&Term{Kind: KApp, Kids: []*Term{op, in, args[1], args[2]}}, e, fuel)
				if err != nil {
					return nil, err
				}
				return wrapWheres(inner, ms), nil
			}
			if c.Kind == KBool {
				if c.IsTrue() {
					return normalize(args[1], e, fuel)
				}
				return normalize(args[2], e, fuel)
			}
			cond = c
		}
		out := make([]*Term, 0, len(t.Kids))
		out = append(out, op)
		for i, a := range args {
			if i == 0 && cond != nil { // already normalised above
				out = append(out, cond)
				continue
			}
			na, err := normalize(a, e, fuel)
			if err != nil {
				return nil, err
			}
			out = append(out, na)
		}
		if ab, ok := e.absorb(out); ok {
			return ab, nil
		}
		// TWO LAWS OF THE CONDITIONAL, on bool = 1 + 1 (ADR 0017):
		//
		//	(if c true false)  ⟶  c       η: the eliminator applied to the
		//	                               constructors is the identity
		//	(if c a a)         ⟶  a       idempotence, for a c with no effect
		//
		// The second needs c pure: dropping its evaluation is weakening, which
		// ADR 0010 allows only for a pure term. Both arrive with the error model,
		// where `(case o none true (some e) false)` and a discard that fails
		// the same way on both arms reduce to exactly these (oskinds-2026-10-06).
		if cond != nil && len(out) == 4 {
			a, b := out[2], out[3]
			if a.Kind == KBool && b.Kind == KBool && a.IsTrue() && !b.IsTrue() {
				return cond, nil
			}
			if a.Equal(b) && e.pureTerm(cond, map[string]bool{}) {
				return a, nil
			}
		}
		return &Term{Kind: KApp, Kids: out}, nil
	}
	return nil, fmt.Errorf("unknown term kind %d", t.Kind)
}

// duplicable reports whether a normalised term may be copied freely.
//
// Literals and variables are obviously free. Abstractions are the interesting
// case: a duplicated λ MUST be substituted or fusion dies — in the dot product
// the two copies of the zip term reduce to *different* small things, (alen p)
// and a multiply, and that is the entire mechanism. Duplicating a λ that does
// not reduce away costs code size, which is the measured specialize-versus-
// outline tradeoff, not a correctness problem.
//
// Everything else is an application of a primitive, which may allocate, and
// allocation is what no host can hoist.
func duplicable(t *Term) bool {
	if IsRequire(t) {
		return duplicable(t.Kids[4]) // a measurement mark is its argument
	}
	if IsRequireWhere(t) {
		return duplicable(t.Kids[3]) // a where-mark is its body
	}
	switch t.Kind {
	case KInt, KFloat, KStr, KBool, KName, KFn, KBound:
		return true
	}
	// A CLOSED INTEGER is a constant, a literal past the word staged as closed
	// arithmetic (2^64 − 1 is 18446·10¹⁵ + …): free to copy, as a literal is.
	if t.Kind == KApp {
		if _, ok := closedBig(t); ok {
			return true
		}
	}
	// A RANGE IS A STATIC VALUE with no run-time existence, so copying it costs
	// nothing, as copying a λ does: lo..hi with copyable ends.
	if t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName && t.Kids[0].Name == ".." {
		_, closedLo := closedBig(t.Kids[1])
		_, closedHi := closedBig(t.Kids[2])
		return (duplicable(t.Kids[1]) || closedLo) && (duplicable(t.Kids[2]) || closedHi)
	}
	// A TUPLE OF COPYABLE COMPONENTS is a function too, eliminated by applying
	// it, so copying it copies a description: a codec, (k, lo, hi, order).
	if t.Kind == KApp && len(t.Kids) > 1 && t.Kids[0].Kind == KName && t.Kids[0].Name == "tuple" {
		for _, k := range t.Kids[1:] {
			if !duplicable(k) {
				return false
			}
		}
		return true
	}
	return false
}

// VersionPrefix begins the name of a version the state translation binds
// (core/cell.go): a store's new table, or a borrow's handed-back buffer.
const VersionPrefix = "#v"

// firstUse reports whether the bound variable (depth, index) of t is the first
// thing t evaluates, unconditionally: reached through an application's
// operator and arguments in order, a let's value, a loop's initializers or an
// if's condition, with only pure terms evaluated before it, and not under a λ, a
// loop's body or a branch.
func (e *Env) firstUse(t *Term, depth, index int, value *Term) bool {
	e.initHole = !containsLoop(value)
	found, _ := e.headUse(t, depth, index)
	return found
}

// containsLoop reports a loop in t.
func containsLoop(t *Term) bool {
	if t == nil {
		return false
	}
	if t.Kind == KApp && len(t.Kids) > 0 && t.Kids[0].Kind == KName && t.Kids[0].Name == "loop" {
		return true
	}
	for _, k := range t.Kids {
		if containsLoop(k) {
			return true
		}
	}
	return false
}

// headUse walks t in evaluation order: found when it reaches the variable,
// stop when it reaches an effect, a branch or a body first.
func (e *Env) headUse(t *Term, depth, index int) (found, stop bool) {
	switch t.Kind {
	case KBound:
		if t.Depth == depth && t.Index == index {
			return true, true
		}
		return false, false
	case KName, KInt, KFloat, KStr, KBool:
		return false, false
	case KFn:
		// A value: its body runs where it is applied, later or never.
		return false, boundOccurrences(t, depth, index) > 0
	}
	if len(t.Kids) == 0 {
		return false, false
	}
	op := t.Kids[0]
	args := t.Kids[1:]
	seq := func(ts ...*Term) (bool, bool) {
		for _, k := range ts {
			if f, s := e.headUse(k, depth, index); f || s {
				return f, true
			}
			if !e.pureTerm(k, map[string]bool{}) {
				return false, true
			}
		}
		return false, false
	}
	if op.Kind == KName {
		switch {
		case op.Name == "if" && len(args) == 3:
			if f, _ := seq(args[0]); f {
				return true, true
			}
			return false, true
		case op.Name == "loop" && len(args) >= 1:
			// A LOOP'S INITIALIZER is a hole for a value that holds no loop: a
			// store chain there is what the content facts induct on (array-
			// facts.md), and a loop there would be analysed again for every
			// candidate invariant Houdini checks against the initializer
			// (freq compiled 1.5× slower; storeorder-2026-10-15 §3).
			if !e.initHole {
				return false, true
			}
			if f, s := seq(args[1:]...); f || s {
				return f, true
			}
			return false, true
		case op.Name == "let" && len(args) == 2 && args[1].Kind == KFn:
			if f, s := seq(args[0]); f || s {
				return f, true
			}
			return e.headUse(args[1].Kids[0], depth+1, index)
		}
		return seq(args...)
	}
	if op.Kind == KFn && len(op.Params) == len(args) {
		// A β-redex: its arguments, then its body.
		if f, s := seq(args...); f || s {
			return f, true
		}
		return e.headUse(op.Kids[0], depth+1, index)
	}
	if len(args) == 1 && args[0].Kind == KFn && op.Kind == KApp {
		// An elimination: the eliminated term, then the continuation.
		if f, s := seq(op); f || s {
			return f, true
		}
		return e.headUse(args[0].Kids[0], depth+1, index)
	}
	// AN OPERATOR IS NO HOLE: a table read and a call keep a variable there,
	// and lowering takes no other application as an operator.
	if op.Kind == KBound && op.Depth == depth && op.Index == index {
		return false, true
	}
	return seq(t.Kids...)
}

// boundOccurrences counts uses of ONE bound variable, saturating at 2. No
// shadowing check: an inner binder simply sits at a different depth.
func boundOccurrences(t *Term, depth, index int) int {
	switch t.Kind {
	case KBound:
		if t.Depth == depth && t.Index == index {
			return 1
		}
		return 0
	case KName, KInt, KFloat, KStr, KBool:
		return 0
	case KFn:
		return boundOccurrences(t.Kids[0], depth+1, index)
	}
	n := 0
	for _, k := range t.Kids {
		n += boundOccurrences(k, depth, index)
		if n >= 2 {
			return 2
		}
	}
	return n
}

// occurrences counts free occurrences of name in t, saturating at 2 — the
// decision only needs to distinguish "at most once" from "more than once".
func occurrences(t *Term, name string) int {
	switch t.Kind {
	case KName:
		if t.Name == name {
			return 1
		}
		return 0
	case KInt, KFloat, KStr, KBool, KBound:
		return 0
	case KFn:
		for _, p := range t.Params {
			if p == name {
				return 0 // shadowed
			}
		}
		return occurrences(t.Body(), name)
	}
	n := 0
	for _, k := range t.Kids {
		n += occurrences(k, name)
		if n >= 2 {
			return 2
		}
	}
	return n
}

// Occurs reports whether name is used in t. Backends need this to recognise a
// residual `let` whose binder is used zero times: that is a sequencing point
// (effects.md §5), the binding has no reader, and Go rejects the program if one
// is emitted anyway.
func Occurs(t *Term, name string) bool { return occurrences(t, name) > 0 }

// subst replaces FREE NAMES, in the locally nameless representation (core-0,
// concerns.md §1.3): a binder holds indices, so inside a closed body every name
// is free, and substitution needs no opening, no shadowing test and no
// freshening. Capture is unrepresentable, not merely avoided.
//
// A substituend may itself hold indices: a value normalised inside a λ's closed
// body refers to that λ's parameters, and to every binder around it, by depth.
// Put under k more binders, each such index must count k more, so it is shifted
// by the depth it lands at, as openWith shifts β's arguments. That is the whole
// difference from the substitution this replaced, which opened each λ it passed
// and put the value in unshifted: sound only for a value with no index in it.
// β's recount handed it one, a loop variable inside a `try`'s payload, and the
// result referred to a binder one level out (sumloop-2026-10-06).
//
// It rests on the representation's invariant (CLAUDE.md, "Body() and openFresh
// rebuild terms"): a λ's closed body never contains a free name equal to its own
// parameter. So no binder below can mean a name in m.
func subst(t *Term, m map[string]*Term) *Term {
	if len(m) == 0 {
		return t
	}
	return substAt(t, m, 0)
}

func substAt(t *Term, m map[string]*Term, depth int) *Term {
	switch t.Kind {
	case KName:
		if r, ok := m[t.Name]; ok {
			return shift(r, 0, depth)
		}
		return t
	case KInt, KFloat, KStr, KBool, KBound:
		return t
	case KFn:
		return &Term{Kind: KFn, Params: t.Params, Kids: []*Term{substAt(t.Kids[0], m, depth+1)}}
	case KApp:
		kids := make([]*Term, len(t.Kids))
		for i, k := range t.Kids {
			kids[i] = substAt(k, m, depth)
		}
		return &Term{Kind: KApp, Kids: kids}
	}
	return t
}

func occupied(name string, danger map[string]bool, params []string) bool {
	if danger[name] {
		return true
	}
	for _, p := range params {
		if p == name {
			return true
		}
	}
	return false
}

func freeVars(t *Term) map[string]bool {
	out := map[string]bool{}
	var walk func(t *Term, bound map[string]bool)
	walk = func(t *Term, bound map[string]bool) {
		switch t.Kind {
		case KName:
			if !bound[t.Name] {
				out[t.Name] = true
			}
		case KInt, KFloat, KStr, KBool, KBound:
		case KFn:
			inner := make(map[string]bool, len(bound)+len(t.Params))
			for k := range bound {
				inner[k] = true
			}
			for _, p := range t.Params {
				inner[p] = true
			}
			walk(t.Body(), inner)
		default:
			for _, k := range t.Kids {
				walk(k, bound)
			}
		}
	}
	walk(t, map[string]bool{})
	return out
}

// Residual reports names in a normalised term that the target cannot express —
// the ways in which it failed to reach normal form. An empty result means the
// term is fully in the target's vocabulary.
//
// A recursive definition is NOT a failure. δ deliberately does not unfold it
// (core-0 §6), so it survives as a target function, which is correct: `(def f
// (f))` denotes ⊥ and compiles to a function that calls itself. Reporting it
// here would flag the correct compilation of a well-defined term as an error.
func Residual(t *Term, e *Env) []string {
	found := map[string]bool{}
	// `again` is bound by the enclosing loop and by nothing else, so it is not
	// residual there — the same rule scope() applies, arriving here late.
	//
	// It arrived late because it was never reached: ADR 0015 was built and
	// benchmarked entirely through `gen`, which emits a function, and this
	// check lives in `build`, which makes a binary. Every loop program was
	// therefore refused by the one command that produces an artifact, and no
	// test noticed, because no test built one. Found writing the fourth target.
	var walk func(t *Term, bound map[string]bool, inLoop bool)
	walk = func(t *Term, bound map[string]bool, inLoop bool) {
		switch t.Kind {
		case KName:
			if bound[t.Name] || (t.Name == "again" && inLoop) {
				return
			}
			if !e.Prim[t.Name] && !e.Rec[t.Name] {
				found[t.Name] = true
			}
		case KInt, KFloat, KStr, KBool, KBound:
		case KFn:
			inner := make(map[string]bool, len(bound)+len(t.Params))
			for k := range bound {
				inner[k] = true
			}
			for _, p := range t.Params {
				inner[p] = true
			}
			walk(t.Body(), inner, inLoop)
		default:
			// (loop (fn (x…) body) z…): only the abstraction is under the
			// binder. The initial values are evaluated outside it.
			isLoop := t.Kind == KApp && t.Op().Kind == KName && t.Op().Name == "loop"
			for i, k := range t.Kids {
				walk(k, bound, inLoop || (isLoop && i == 1))
			}
		}
	}
	walk(t, map[string]bool{}, false)
	out := make([]string, 0, len(found))
	for n := range found {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// substPublic exposes capture-avoiding substitution for backends.
func substPublic(t *Term, m map[string]*Term) *Term { return subst(t, m) }

// CheckScope reports names that are bound nowhere.
//
// This is **name resolution**, and it is a separate question from the covering
// check even though both look at free names:
//
//	scope    — is this name bound anywhere at all?   A PROGRAM error.
//	covering — can THIS target provide it?           ADR 0001's portability property.
//
// Conflating them left three holes. `oro` printed a warning and exited 0; `gen`
// never checked at all; and a name that appears only in a definition the program
// never reaches was never looked at, so a typo in unused code was invisible.
// That last one is the classic reason name resolution walks EVERYTHING rather
// than only what reduction happens to visit.
//
// It runs before reduction, so the report names the definition the mistake is
// in rather than wherever substitution later carried it.
//
// It also rejects recursion, for the reason in ADR 0014. The two live in one
// method because they are the same question — can this target run this program
// — and because the two-call-site version of the recursion check drifted from
// the scope check within a day of being written.
func (e *Env) CheckProgram(terms []*Term) error {
	if err := e.checkScope(terms); err != nil {
		return err
	}
	return e.checkRecursion()
}

func (e *Env) checkScope(terms []*Term) error {
	for _, name := range sortedKeys(e.Defs) {
		if err := e.scope(e.Defs[name], map[string]bool{}, "in "+ShowName(name)); err != nil {
			return err
		}
	}
	for _, t := range terms {
		if err := e.scope(t, map[string]bool{}, "at the top level"); err != nil {
			return err
		}
	}
	return nil
}

func (e *Env) scope(t *Term, bound map[string]bool, where string) error {
	switch t.Kind {
	case KInt, KFloat, KStr, KBool, KBound:
		return nil
	case KName:
		// `#any`, the unconstrained value, is the language's own (Theorem R,
		// data.md §5.5.5), on every target, and no source can write it; so is
		// `#elim`, the cell translation's claim the reducer decides; and
		// `#segment`, J's b[lo:hi], which δ rewrites to a rule (segment); and a
		// range, a static value Law S keeps out of the residual (staged).
		if bound[t.Name] || e.Prim[t.Name] || e.Sets[t.Name] || t.Name == AnyName || t.Name == ElimName || t.Name == SegmentName ||
			t.Name == ".." || t.Name == AdoptName || t.Name == OwnedName {
			return nil
		}
		if _, ok := e.Defs[t.Name]; ok {
			return nil
		}
		return fmt.Errorf("%s: %s is not bound — it is not a parameter, not a definition, "+
			"and not a primitive on this target%s", where, ShowName(t.Name), e.importHint(t.Name))
	case KFn:
		inner := make(map[string]bool, len(bound)+len(t.Params))
		for k := range bound {
			inner[k] = true
		}
		for _, p := range t.Params {
			inner[p] = true
		}
		return e.scope(t.Body(), inner, where)
	}
	// `again` is bound by the enclosing loop, and only there. The reader has
	// already checked its arity and position (iteration.md §2); this is the
	// scope half, so an `again` outside any loop is reported like any other
	// unbound name rather than reaching the emitter.
	if t.Kind == KApp && t.Op().Kind == KName && t.Op().Name == "loop" && len(t.Args()) >= 1 {
		if lam := t.Args()[0]; lam.Kind == KFn {
			inner := make(map[string]bool, len(bound)+len(lam.Params)+1)
			for k := range bound {
				inner[k] = true
			}
			for _, p := range lam.Params {
				inner[p] = true
			}
			inner["again"] = true
			if err := e.scope(lam.Body(), inner, where); err != nil {
				return err
			}
			for _, a := range t.Args()[1:] {
				if err := e.scope(a, bound, where); err != nil {
					return err
				}
			}
			return nil
		}
	}
	for _, k := range t.Kids {
		if err := e.scope(k, bound, where); err != nil {
			return err
		}
	}
	return nil
}

// checkRecursion rejects a definition that is defined in terms of itself.
//
// Recursion REDUCES correctly — δ declines to unfold a cycle, which is the
// standard partial-evaluation answer — and no backend emits one. That gap was
// reported at the emitter for one day, which meant `oro` accepted a program
// that `build` refused. ADR 0014 closes it the other way: recursion is not in
// the language, so it must fail at the earliest honest point.
//
// That point is here rather than in the reader, because whether a name is
// recursive depends on the TARGET: a target that provides the name natively
// never unfolds the definition, and markRecursive already skips those.
func (e *Env) checkRecursion() error {
	names := make([]string, 0, len(e.Rec))
	for n := range e.Rec {
		names = append(names, n)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	if len(names) == 1 {
		return fmt.Errorf("%s is recursive: it is defined in terms of itself, and recursion "+
			"is not in the language, since δ unfolds a definition and a cycle has no normal form "+
			"(ADR 0014). Iteration is `loop` with `again` (ADR 0015).\n"+
			"  If the self-reference was not deliberate, this definition is shadowing "+
			"the %s you meant.", names[0], names[0])
	}
	return fmt.Errorf("%s are mutually recursive, and recursion is not in the language, since δ "+
		"unfolds a definition and a cycle has no normal form (ADR 0014). Iteration is `loop` with "+
		"`again` (ADR 0015)", strings.Join(names, ", "))
}

// Shadowed reports definitions the target provides natively.
//
// This is ADR 0002 working — δ declines to unfold a name in P_T, so the target's
// own implementation is used and the definition is a fallback for targets that
// lack it. It is the single most consequential thing the compiler decides about
// a program, it is decided by a file the program does not name, and it was
// silent. So it is reported as a note rather than left to be discovered.
//
// It is deliberately NOT an error and deliberately not extended to a general
// shadowing report — see def.md §11 for what is not reported and why.
func (e *Env) Shadowed() []string {
	var out []string
	for n := range e.Defs {
		if e.Prim[n] {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// importHint explains an unresolved qualified name when the cause is an import
// that matched nothing at all.
//
// `(use geometrie)` for a module spelled `geometry` is silent at the import —
// a path with no file is how a TARGET-provided module looks — and then fails on
// the first member with a message about the member. Which is the wrong half of
// the name to look at.
func (e *Env) importHint(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return ""
	}
	path := name[:i]
	if !e.unresolvedPaths[path] {
		return ""
	}
	for p := range e.Prim {
		if strings.HasPrefix(p, path+".") {
			return "" // the target does provide this module, just not this member
		}
	}
	return fmt.Sprintf("\n  `use %s` matched no file on the search path and this target "+
		"provides no module %s either, so every name from it is unbound. Check the path.",
		path, path)
}

// modLabel names a module in a diagnostic. The entry file's anonymous scope has
// no path, and calling it "" reads as a bug.
func modLabel(path string) string {
	if path == "" {
		return "the program"
	}
	return "module " + path
}

// isIfApp reports whether t is a residual `if` — a conditional whose branches
// survived because the condition is not known. That is the only shape
// case-of-case applies to.
func isIfApp(t *Term, e *Env) bool {
	return t.Kind == KApp && len(t.Kids) == 4 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "if" && e.Prim["if"]
}

// isResidualLet reports a residual `(let v (fn (x) b))`.
func isResidualLet(op *Term, args []*Term, e *Env) bool {
	return op.Kind == KName && op.Name == "let" && e.Prim["let"] && len(args) == 2 &&
		args[1].Kind == KFn && len(args[1].Params) == 1
}

// reduceLet is a residual let's whole rule: β when its value is duplicable
// (see normalize), and otherwise the generic application's.
func (e *Env) reduceLet(op *Term, args []*Term, fuel *int) (*Term, error) {
	v, err := normalize(args[0], e, fuel)
	if err != nil {
		return nil, err
	}
	if duplicable(v) && !IsRequire(v) && !IsRequireWhere(v) {
		return normalize(args[1].OpenWith([]*Term{v}), e, fuel)
	}
	// A VALUE UNDER AN OBLIGATION: the obligation is moved out, to stand
	// around the let, where it is decided once, and the value it guards is
	// bound: let x = (#reqw c v) in b is (#reqw c (let x = v in b)). A codec's
	// width under its `where` (lib/bytes) is then the literal it is.
	if IsRequireWhere(v) {
		in, ms := peelWheres(v)
		if duplicable(in) && !IsRequire(in) {
			inner, err := normalize(args[1].OpenWith([]*Term{in}), e, fuel)
			if err != nil {
				return nil, err
			}
			return wrapWheres(inner, ms), nil
		}
	}
	// β'S ONE-EVALUATION RULE, ON A RESIDUAL LET (ADR 0071). β binds a pure
	// value whose use, when it looked, sat under a λ that might run many times:
	// a λ handed to a definition. Once that λ is applied, by staging, the use
	// may be evaluated once, or not at all; then the binding serves nothing, and
	// β's own rule applies: substitute, or drop a pure value unread. A table
	// read stays bound before an impure body, as β binds it (effects.md §7c).
	if b := args[1].Closed(); !IsRequire(v) && !IsRequireWhere(v) &&
		e.pureTerm(v, map[string]bool{}) && e.workOccurrences(b, 0, 0, []bool{false}) <= 1 &&
		!(e.readsTable(v) && !e.pureTerm(args[1].Body(), map[string]bool{})) {
		return normalize(args[1].OpenWith([]*Term{v}), e, fuel)
	}
	// A VERSION USED ONCE, FIRST: β's own rule (normalize).
	if b := args[1].Closed(); strings.HasPrefix(args[1].Params[0], VersionPrefix) &&
		boundOccurrences(b, 0, 0) == 1 && e.firstUse(b, 0, 0, v) && !IsRequire(v) && !IsRequireWhere(v) {
		return normalize(args[1].OpenWith([]*Term{v}), e, fuel)
	}
	k, err := normalize(args[1], e, fuel)
	if err != nil {
		return nil, err
	}
	// THE RIGHT UNIT LAW, m >>= return = m: `(let v (fn (x) x))` is v,
	// whatever v's effects, since they run once, in place, either way.
	if b := k.Closed(); b.Kind == KBound && b.Depth == 0 && b.Index == 0 {
		return v, nil
	}
	out := []*Term{op, v, k}
	if ab, ok := e.absorb(out); ok {
		return ab, nil
	}
	return &Term{Kind: KApp, Kids: out}, nil
}

// isMultiApp recognises a host call with several results applied to its
// continuation: ((p a…) (fn (x̄) M)) with p a primitive — the n-ary let.
func isMultiApp(t *Term, e *Env) bool {
	return t.Kind == KApp && len(t.Kids) == 2 && t.Kids[1].Kind == KFn &&
		t.Kids[0].Kind == KApp && len(t.Kids[0].Kids) >= 1 &&
		t.Kids[0].Kids[0].Kind == KName && e.Prim[t.Kids[0].Kids[0].Name]
}

// isLetApp reports whether t is a residual `let` — a binding β introduced
// because its value was shared and not duplicable.
func isLetApp(t *Term, e *Env) bool {
	return t.Kind == KApp && len(t.Kids) == 3 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == "let" && e.Prim["let"] &&
		t.Kids[2].Kind == KFn && len(t.Kids[2].Params) == 1
}

// allPure reports whether every term is safe to DUPLICATE. Case-of-case copies
// its arguments into both branches, and ADR 0010 denies contraction for an
// impure term, so an impure argument leaves the redex stuck rather than running
// an effect twice.
func allPure(ts []*Term, e *Env) bool {
	for _, t := range ts {
		if !e.pureTerm(t, map[string]bool{}) {
			return false
		}
	}
	return true
}

// THE CRASH ABSORBS ITS CONTEXT (spec/errors.md §8).
//
//	(f a₁ … aₖ₋₁ (abandon w) …)  ⟶  (let aᵢ (fn (_) … (abandon w)))   for each impure aᵢ, i < k
//
// `abandon` is crash : E → 0, and 0 is the initial object: there is exactly one
// arrow 0 → Y, so k ∘ abandon = abandon for every continuation k. It is the
// commuting conversion at arity 0: 0 is the empty coproduct, its eliminator the
// empty copairing, and K ∘ [g₁…gₙ] = [K∘g₁…K∘gₙ] at n = 0 is K ∘ [] = []. An application
// of a primitive evaluates its arguments in order, each one not a λ, and an
// `if` only its condition; so a crash in such a position is the whole
// application's value, after the effects evaluated before it, which stay, in
// order (ADR 0010). A pure one before it is dropped: it has no effect, and an
// unused pure term is weakened as β weakens it. `let` is a primitive whose
// value is its strict argument, so `(let (abandon w) k)` ⟶ `(abandon w)`, which
// is also `seq`'s case. A λ argument is a value and is never run here, so a
// crash inside one, and in an `if`'s branch, stays where it is.
//
// Without the rule the crash's "value", argument 0 by the statement rule, reached
// what followed it: a string flowed into the integer a driver printed (W5), and
// javac refused the statement after the throw as unreachable.
func (e *Env) absorb(kids []*Term) (*Term, bool) {
	if len(kids) < 2 || kids[0].Kind != KName || !e.Prim[kids[0].Name] {
		return nil, false
	}
	strict := kids[1:]
	if kids[0].Name == "if" {
		strict = strict[:1]
	}
	for i, a := range strict {
		if !isAbandon(a, e) {
			continue
		}
		out := a
		for j := i - 1; j >= 0; j-- {
			b := strict[j]
			if b.Kind == KFn || e.pureTerm(b, map[string]bool{}) {
				continue
			}
			out = App(Name("let"), b, Fn(e.freshNames(1, out, b), out))
		}
		return out, true
	}
	return nil, false
}

// tupleTails reports n when every tail of t is an n-tuple, `(fn (#k) (#k c₁ … cₙ))`:
// the value of a product, and of a sum, whose constructor is the tuple (tag,
// payload) (ADR 0041). It is what η for products needs: r = (π₁ r, …, πₙ r) for
// any r of a product type, so a binding of such a value may be taken apart,
// `(let V (fn (r) B))` ⟶ `(V (fn (c̄) B[r := (tuple c̄)]))`, which is ADR 0031's
// tuple pattern (a join point over a scope or a loop), and each `(r k)` in B is
// then a β-redex. Without it an eliminator applied to the variable was stuck:
// a sum read twice, and a variant out of a scope (sumloop-2026-10-06).
//
// The tails are walked through the forms a value's tails can sit under: `if`,
// a residual `let`, a host call's continuation, a `build` or `build-map` scope,
// a where-mark, and a `loop`'s exits, where an `again` is no tail. Any other
// shape answers no, so a form this walk does not know leaves the binding as it
// was: imprecise, never unsound. inLoop is set while walking a loop body, where
// `again` is a back edge.
func (e *Env) tupleTails(t *Term, inLoop bool) (int, bool) {
	const none = -1 // a subterm with no tail: an `again`
	join := func(a, b int, oka, okb bool) (int, bool) {
		switch {
		case !oka || !okb:
			return 0, false
		case a == none:
			return b, true
		case b == none || a == b:
			return a, true
		}
		return 0, false
	}
	if t.Kind == KFn && len(t.Params) == 1 && IsTupleBinder(t.Params[0]) {
		// n = 0 is the unit, `(fn (#k) (#k))`; n = 1 does not exist,
		// (tuple T) ≅ T (data.md §3.6).
		b := t.Closed()
		if b.Kind == KApp && len(b.Kids) != 2 && b.Kids[0].Kind == KBound &&
			b.Kids[0].Depth == 0 && b.Kids[0].Index == 0 {
			return len(b.Kids) - 1, true
		}
		return 0, false
	}
	if t.Kind != KApp || len(t.Kids) == 0 {
		return 0, false
	}
	// A host call's continuation: ((p a…) (fn (x̄) M)), which runs once, now.
	if op := t.Kids[0]; op.Kind == KApp && len(op.Kids) > 0 && op.Kids[0].Kind == KName &&
		e.Prim[op.Kids[0].Name] && len(t.Kids) == 2 && t.Kids[1].Kind == KFn {
		return e.tupleTails(t.Kids[1].Closed(), inLoop)
	}
	op := t.Kids[0]
	if op.Kind != KName {
		return 0, false
	}
	switch {
	case op.Name == "again" && inLoop:
		return none, true
	case op.Name == RequireWhereName && len(t.Kids) == 4:
		return e.tupleTails(t.Kids[3], inLoop)
	case !e.Prim[op.Name]:
		return 0, false
	case op.Name == "if" && len(t.Kids) == 4:
		a, oka := e.tupleTails(t.Kids[2], inLoop)
		b, okb := e.tupleTails(t.Kids[3], inLoop)
		return join(a, b, oka, okb)
	case op.Name == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return e.tupleTails(t.Kids[2].Closed(), inLoop)
	case (op.Name == "build" || op.Name == "build-map") && len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return e.tupleTails(t.Kids[2].Closed(), inLoop)
	case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
		n, ok := e.tupleTails(t.Kids[1].Closed(), true)
		if !ok || n == none {
			return 0, false // a loop with no exit has no value to take apart
		}
		return n, true
	}
	return 0, false
}

// projection reports πⱼ of an n-tuple, (fn (x₁ … xₙ) xⱼ), with n ≥ 2.
func projection(f *Term) (j, n int, ok bool) {
	if f.Kind != KFn || len(f.Params) < 2 {
		return 0, 0, false
	}
	if b := f.Closed(); b.Kind == KBound && b.Depth == 0 {
		return b.Index, len(f.Params), true
	}
	return 0, 0, false
}

// selection reports a λ over an n-tuple whose body selects components: one
// variable, πⱼ, or a tuple of its own variables, (fn (#k) (#k xᵢ …)). It returns
// the components' indices in order, a projection's alone.
func selection(f *Term) (sel []int, n int, ok bool) {
	if j, n, ok := projection(f); ok {
		return []int{j}, n, true
	}
	if f.Kind != KFn || len(f.Params) < 2 {
		return nil, 0, false
	}
	b := f.Closed()
	if b.Kind != KFn || len(b.Params) != 1 || !IsTupleBinder(b.Params[0]) {
		return nil, 0, false
	}
	t := b.Closed()
	if t.Kind != KApp || len(t.Kids) < 3 || t.Kids[0].Kind != KBound || t.Kids[0].Depth != 0 || t.Kids[0].Index != 0 {
		return nil, 0, false
	}
	for _, k := range t.Kids[1:] {
		if k.Kind != KBound || k.Depth != 1 {
			return nil, 0, false
		}
		sel = append(sel, k.Index)
	}
	return sel, len(f.Params), true
}

func isLoopTerm(t *Term, e *Env) bool {
	return t.Kind == KApp && len(t.Kids) >= 2 && t.Kids[0].Kind == KName &&
		t.Kids[0].Name == "loop" && e.Prim["loop"] && t.Kids[1].Kind == KFn
}

// projectTails replaces every tail of a loop body t, each an n-tuple, by its
// j-th component; an `again` is a back edge and stays. It walks the forms
// tupleTails walks, under the binders it meets without opening them, since a
// component is a term of the tuple's own scope. Any other tail answers no, and
// the projection stays outside.
func (e *Env) projectTails(t *Term, j, n int) (*Term, bool) {
	return e.selectTails(t, []int{j}, n)
}

// selectTails is projectTails for a selection: each n-tuple tail becomes its
// selected component, or the tuple of its selected components.
func (e *Env) selectTails(t *Term, sel []int, n int) (*Term, bool) {
	with := func(i int, k *Term) *Term {
		kids := append([]*Term(nil), t.Kids...)
		kids[i] = k
		return &Term{Kind: KApp, Kids: kids}
	}
	under := func(i int) (*Term, bool) { // t.Kids[i] is a λ whose body holds tails
		lam := t.Kids[i]
		b, ok := e.selectTails(lam.Closed(), sel, n)
		if !ok {
			return nil, false
		}
		return with(i, &Term{Kind: KFn, Params: lam.Params, Kids: []*Term{b}}), true
	}
	if t.Kind == KFn && len(t.Params) == 1 && IsTupleBinder(t.Params[0]) {
		b := t.Closed()
		if b.Kind == KApp && len(b.Kids) == n+1 && b.Kids[0].Kind == KBound &&
			b.Kids[0].Depth == 0 && b.Kids[0].Index == 0 {
			// the component lives under the tuple's binder #k, which it does
			// not mention: opening drops that binder
			comps := t.OpenWith([]*Term{Name("#k")}).Kids[1:]
			if len(sel) == 1 {
				return comps[sel[0]], true
			}
			picked := make([]*Term, len(sel))
			for i, j := range sel {
				picked[i] = comps[j]
			}
			return Fn([]string{"#k"}, App(Name("#k"), picked...)), true
		}
		return nil, false
	}
	if t.Kind != KApp || len(t.Kids) == 0 {
		return nil, false
	}
	op := t.Kids[0]
	if op.Kind == KApp && len(op.Kids) > 0 && op.Kids[0].Kind == KName &&
		e.Prim[op.Kids[0].Name] && len(t.Kids) == 2 && t.Kids[1].Kind == KFn {
		return under(1) // a host call's continuation
	}
	if op.Kind != KName {
		return nil, false
	}
	switch {
	case op.Name == "again":
		return t, true
	case op.Name == "abandon" && e.Prim["abandon"] && len(t.Kids) == 2:
		return t, true // π(abandon w) = abandon w: a crash absorbs a strict context
	case op.Name == RequireWhereName && len(t.Kids) == 4:
		k, ok := e.selectTails(t.Kids[3], sel, n)
		if !ok {
			return nil, false
		}
		return with(3, k), true
	case !e.Prim[op.Name]:
		return nil, false
	case op.Name == "if" && len(t.Kids) == 4:
		a, oka := e.selectTails(t.Kids[2], sel, n)
		b, okb := e.selectTails(t.Kids[3], sel, n)
		if !oka || !okb {
			return nil, false
		}
		return with(3, b).withKid(2, a), true
	case op.Name == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return under(2)
	case (op.Name == "build" || op.Name == "build-map") && len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return under(2)
	case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
		return under(1) // an inner loop's exits are this loop's; its agains are its own
	}
	return nil, false
}

// withKid returns a copy of the application t with kid i replaced.
func (t *Term) withKid(i int, k *Term) *Term {
	kids := append([]*Term(nil), t.Kids...)
	kids[i] = k
	return &Term{Kind: KApp, Kids: kids}
}

// isAbandon reports whether t is an application of the language's crash.
func isAbandon(t *Term, e *Env) bool {
	return t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName &&
		t.Kids[0].Name == "abandon" && e.Prim["abandon"]
}

// IgnoreName is the language's discard, !_A : A → 1 (spec/errors.md §7.3).
// Like `let`, it is the language's, never reaches a target, and a target may
// not declare it.
const IgnoreName = "ignore"

// relevant reports whether a tail of t is a value of a relevant type: a tuple
// built by a relevant variant's constructor, binder `#k!`, or a tuple with
// such a component, since a product discarded discards its components
// (spec/errors.md §7.2). In a typed program every tail has the one type, so
// one tail decides. The tails are walked through the forms tupleTails walks;
// on a term as written, a constructor applied, `(result.ok 1)`, is one too.
func (e *Env) relevant(t *Term, inLoop bool) bool {
	ctor := func(d *Term, args int) bool { // a constructor's definition
		for ; args > 0 && d.Kind == KFn && len(d.Params) == 1; args-- {
			d = d.Kids[0]
		}
		return args == 0 && d.Kind == KFn && len(d.Params) == 1 && IsRelevantBinder(d.Params[0])
	}
	switch t.Kind {
	case KFn:
		if len(t.Params) != 1 || !IsTupleBinder(t.Params[0]) {
			return false
		}
		if IsRelevantBinder(t.Params[0]) {
			return true
		}
		b := t.Closed()
		if b.Kind != KApp {
			return false
		}
		for _, c := range b.Kids[1:] {
			if e.relevant(c, false) {
				return true
			}
		}
		return false
	case KName:
		return e.unfoldable(t.Name) && ctor(e.Defs[t.Name], 0)
	case KApp:
	default:
		return false
	}
	op := t.Kids[0]
	if op.Kind == KApp && len(op.Kids) > 0 && op.Kids[0].Kind == KName &&
		e.Prim[op.Kids[0].Name] && len(t.Kids) == 2 && t.Kids[1].Kind == KFn {
		return e.relevant(t.Kids[1].Closed(), inLoop) // a host call's continuation
	}
	if op.Kind != KName {
		return false
	}
	switch {
	case !e.Prim[op.Name]:
		return e.unfoldable(op.Name) && ctor(e.Defs[op.Name], len(t.Kids)-1)
	case op.Name == RequireWhereName && len(t.Kids) == 4:
		return e.relevant(t.Kids[3], inLoop)
	case op.Name == "if" && len(t.Kids) == 4:
		return e.relevant(t.Kids[2], inLoop) || e.relevant(t.Kids[3], inLoop)
	case (op.Name == "let" || op.Name == "build" || op.Name == "build-map") &&
		len(t.Kids) == 3 && t.Kids[2].Kind == KFn:
		return e.relevant(t.Kids[2].Closed(), inLoop)
	case op.Name == "loop" && len(t.Kids) >= 2 && t.Kids[1].Kind == KFn:
		return e.relevant(t.Kids[1].Closed(), true)
	}
	return false
}

// usedOnEveryPath reports whether the variable bound at (depth, index) is used
// on every path of t (spec/errors.md §7.1). The eliminators are additive: [f, g]
// takes f and g in one context, so `(if c a b)` uses it when c does or both a
// and b do. `(abandon w)` uses it vacuously: 0 is initial, and a crash absorbs
// its context (§8). Everything else is strict in all it contains, a λ's body
// included: an eliminator runs its λ once, and a λ never applied is not checked
// (§7.4).
//
// `(again …)` USES A VARIABLE VACUOUSLY ONLY IF THE VARIABLE OUTLIVES THE JUMP:
// bound outside the loop the jump goes round, its path continues into the next
// iteration, and the loop uses it when each of its exits does. A variable bound
// INSIDE the iteration ends at the jump, so there `again` is an ordinary term,
// strict in its arguments (abstract.md §8.1). Until abstract2-2026-10-11 every
// `again` was vacuous, and `(loop … else (seq (write i) (again …)))` dropped a
// relevant value in silence that `(seq (write n) 1)` would have been refused for.
//
// HANDING IT ON is a use exactly when the receiver uses it: the variable passed
// bare to a λ, or to a definition, is used when that parameter is used on every
// path of the receiver's body as written (consumes). That is usage inference,
// a definition's parameter read off its body, and it terminates because no
// definition reaches itself (ADR 0014). It is needed because reduction under a
// λ runs a definition's β before the λ meets its argument, with the variable
// still abstract, so `(drop r)` has already become 0 when r is bound.
func (e *Env) usedOnEveryPath(t *Term, depth, index int) bool {
	return e.usedOn(t, depth, index, false)
}

// usedOn is usedOnEveryPath with outer set once the walk has entered a loop's
// body: the variable is then bound outside the loop its `again`s go round.
func (e *Env) usedOn(t *Term, depth, index int, outer bool) bool {
	switch t.Kind {
	case KBound:
		return t.Depth == depth && t.Index == index
	case KFn:
		return e.usedOn(t.Kids[0], depth+1, index, outer)
	case KApp:
	default:
		return false
	}
	if isIfApp(t, e) {
		return e.usedOn(t.Kids[1], depth, index, outer) ||
			e.usedOn(t.Kids[2], depth, index, outer) && e.usedOn(t.Kids[3], depth, index, outer)
	}
	if isAbandon(t, e) || outer && t.Kids[0].Kind == KName && t.Kids[0].Name == "again" {
		return true
	}
	op, args := t.Kids[0], t.Kids[1:]
	if op.Kind == KName && op.Name == "loop" && len(args) >= 1 && args[0].Kind == KFn {
		// A loop's initial values are evaluated here; its body is the loop's,
		// where the variable is bound outside the loop.
		for _, z := range args[1:] {
			if e.usedOn(z, depth, index, outer) {
				return true
			}
		}
		return e.usedOn(args[0], depth, index, true)
	}
	var fn *Term // the receiver, when its parameters are known
	switch {
	case op.Kind == KFn:
		fn = op
	case op.Kind == KName && e.unfoldable(op.Name) && e.Defs[op.Name].Kind == KFn:
		fn = e.Defs[op.Name]
	}
	if fn == nil || len(fn.Params) != len(args) {
		for _, k := range t.Kids {
			if e.usedOn(k, depth, index, outer) {
				return true
			}
		}
		return false
	}
	for j, a := range args {
		if a.Kind == KBound && a.Depth == depth && a.Index == index {
			if e.consumes(op, fn, j) {
				return true
			}
			continue
		}
		if e.usedOn(a, depth, index, outer) {
			return true
		}
	}
	// a λ written in place sees the variable itself; a definition is closed
	return op.Kind == KFn && e.usedOn(op, depth, index, outer)
}

// consumes reports whether fn uses its parameter j on every path of its body
// as written: whether handing it a relevant value uses the value. A
// definition's answer is kept, per Env, since it is read at every call.
func (e *Env) consumes(op, fn *Term, j int) bool {
	if op.Kind != KName {
		return e.usedOnEveryPath(fn.Closed(), 0, j)
	}
	key := fmt.Sprintf("%s/%d", op.Name, j)
	if v, ok := e.consumed[key]; ok {
		return v
	}
	if e.consumed == nil {
		e.consumed = map[string]bool{}
	}
	v := e.usedOnEveryPath(fn.Closed(), 0, j)
	e.consumed[key] = v
	return v
}

// programBinder reports whether the program wrote p: a name, or `seq`'s and a
// pattern's `_` (binding.md). The binders the compiler makes, `case`'s slots,
// η's components and a conversion's continuation, bind values whose use the
// program already wrote.
func programBinder(p string) bool {
	return !strings.HasPrefix(p, "#") || strings.HasPrefix(p, "#_")
}

// relevanceError names what was discarded and how.
func relevanceError(p string, arg *Term) error {
	// A call is named by what the program called; a value that reached the
	// binder already reduced (a conversion's `if`, a tuple's component) by
	// the binder it reached.
	what := "the value bound to " + ShowName(p)
	if strings.HasPrefix(p, "#_") {
		what = "a value"
	}
	if h := head(arg); arg.Kind == KApp && arg.Op().Kind == KName && !strings.HasPrefix(h, "#") &&
		h != "if" && h != "let" && h != "loop" && h != "build" && h != "build-map" {
		what = "the result of " + ShowName(h)
	}
	how := "the binder " + ShowName(p) + " is not used on every path"
	if strings.HasPrefix(p, "#_") {
		how = "it is discarded"
	}
	return fmt.Errorf("%s is a value of a relevant type (spec/errors.md §7), and %s: use it, or "+
		"discard it with ignore(…)", what, how)
}

// normalizeLater builds the application case-of-case pushes into one branch.
func normalizeLater(kids []*Term) *Term { return &Term{Kind: KApp, Kids: kids} }

// isTableForm reports whether t is `(array e…)` or `(table n f)` — the two ways
// a table is written down.
func isForm(t *Term, e *Env, name string) bool {
	return t != nil && t.Kind == KApp && len(t.Kids) > 0 &&
		t.Kids[0].Kind == KName && t.Kids[0].Name == name && e.Prim[name]
}

// betaTab is β's second clause: application of a table to an index.
//
// The graph form needs the index to be a LITERAL, because looking up element k
// of `(array a b c)` is only possible when k is known. A dynamic index leaves
// the application alone and the backend emits the host's own indexing — which
// is the whole point of indexing being application: `(a i)` is the same text
// whether it reduces here or is emitted (tables.md §3.1).
//
// The rule form needs no literal: `((table n f) i)` is `(f i)`, which is what
// makes a table-of-a-rule FUSE, the mechanism `dot` runs on. But a table is a
// function on [0, n) (tables.md §1), and f may be defined beyond it: a segment
// `b[lo:hi]` is the rule k ↦ b[lo + k], defined wherever b is. So the
// application keeps its domain as an obligation, 0 ≤ i < n, a where-mark on
// (f i) decided like every other (DischargeRequires) and erased after. Without
// it an index past a rule's end read the function outside the table, a value
// the program never described (ADR 0084's build: examples/pe).
func (e *Env) betaTab(op *Term, args []*Term, fuel *int) (*Term, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if isForm(op, e, "table") && len(op.Kids) == 3 {
		body := &Term{Kind: KApp, Kids: []*Term{op.Kids[2], args[0]}}
		i := StripRequires(args[0])
		cond := App(Name("if"), App(Name("<="), Int(0), i), App(Name("<"), i, op.Kids[1]), Bool(false))
		if i.Kind == KInt && op.Kids[1].Kind == KInt {
			if i.Int >= 0 && i.Int < op.Kids[1].Int {
				return body, true
			}
			return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(RuleIndexDef),
				App(Name(RequireFalseName), cond), body}}, true
		}
		c, err := normalize(cond, e, fuel)
		if err == nil && c.Kind == KBool && c.IsTrue() {
			return body, true
		}
		if err == nil && c.Kind == KBool {
			return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(RuleIndexDef),
				App(Name(RequireFalseName), cond), body}}, true
		}
		// TWO OBLIGATIONS OF TWO KINDS (ADR 0075's split): 0 ≤ i, a constant
		// bound, is a range on the index, decided by a literal, the IR's
		// intervals, then the refinement layer; i < n, a relation, is the
		// where-mark the refinement layer decides from the guards.
		if e.Word != (Word{}) {
			idx := e.nonnegative(RuleIndexDef, "index", args[0])
			body = &Term{Kind: KApp, Kids: []*Term{op.Kids[2], idx}}
			return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(RuleIndexDef),
				App(Name("<"), i, op.Kids[1]), body}}, true
		}
		if err != nil {
			c = cond
		}
		return &Term{Kind: KApp, Kids: []*Term{Name(RequireWhereName), Str(RuleIndexDef), c, body}}, true
	}
	// A MAP LITERAL, which is β-tab with a SUM in the result position.
	//
	// `((map (1 10) (2 20)) 1)` is `(some 10)` and `((map (1 10)) 5)` is
	// `(none)`. Both need the index AND every literal key to be integer
	// literals, because the domain condition `k ∈ dom m` is decided by equality
	// and this is the only place equality can be decided by inspection.
	//
	// Absence is a RESULT here, not a stuck term — unlike an array, where an
	// out-of-range index is left alone for the refinement layer to report with
	// the bound and the call site. That difference IS the difference between
	// the two constructs: an array's domain condition is discharged statically
	// and a map's is discharged by the program (maps.md §1.1), so `none` is the
	// answer rather than an error.
	if isForm(op, e, "map") {
		k, err := normalize(args[0], e, fuel)
		if err != nil || k == nil || k.Kind != KInt {
			return nil, false
		}
		for _, row := range op.Kids[1:] {
			if row.Kind != KApp || len(row.Kids) != 2 {
				return nil, false
			}
			key, err := normalize(row.Kids[0], e, fuel)
			if err != nil || key == nil || key.Kind != KInt {
				return nil, false
			}
			if key.Int == k.Int {
				return &Term{Kind: KApp, Kids: []*Term{Name("some"), row.Kids[1]}}, true
			}
		}
		// A payload-less variant IS the constructor — its definition is the
		// encoded value, not a function — so absence is the NAME `none`, never
		// an application of it.
		return Name("none"), true
	}
	if isForm(op, e, "array") {
		i, err := normalize(args[0], e, fuel)
		if err != nil || i == nil || i.Kind != KInt {
			return nil, false
		}
		elems := op.Kids[1:]
		if i.Int < 0 || i.Int >= int64(len(elems)) {
			// Out of the domain. NOT an error here — the refinement layer is
			// what reports it, with the bound and the call site. Reduction
			// leaves it alone so the diagnostic comes from the place that can
			// explain it (tables.md §6).
			return nil, false
		}
		return elems[i.Int], true
	}
	return nil, false
}

// tableLen folds `len` when the table's length is known: `(len (array a b c))`
// is 3 and `(len (table n f))` is n.
//
// This joins `if` on a boolean literal and `=` on two integers under the same
// rule — a construct decided by a literal it can see. `if` was already exactly
// that, so this is a widening of a rule that existed rather than a new one
// (tables.md §4.2), and the granularity change is stated rather than hidden.
func (e *Env) tableLen(args []*Term) (*Term, bool) {
	if len(args) != 1 {
		return nil, false
	}
	t := args[0]
	if isForm(t, e, "array") || isForm(t, e, "map") {
		return &Term{Kind: KInt, Int: int64(len(t.Kids) - 1)}, true
	}
	if isForm(t, e, "table") && len(t.Kids) == 3 {
		return t.Kids[1], true
	}
	return nil, false
}

// duplicableTable reports whether a term is a table given by a RULE, which is
// free to copy.
//
// `(table n f)` has no runtime existence: it is a length and a function, and
// copying it copies a description rather than data. That is the whole reason
// the rule form is free, and it is what makes fusion work — `sum` mentions its
// argument twice, as `(len v)` and as `(v i)`, so without this β let-binds the
// table and the intermediate survives to the residual instead of vanishing.
//
// It is exactly the argument that makes `KFn` duplicable, one constructor over:
// a rule-table is a lambda with a length attached. `(array e…)` is deliberately
// NOT included — a graph is data, and duplicating it duplicates the elements.
// `(alloc t)` is where memory happens, and it is not duplicable either.
//
// The condition is PURITY, not duplicability, and the reason is that copying a
// rule-table does not actually duplicate anything: substituting it puts
// `(len (table n f))` where `(len v)` was — which folds to `n` — and
// `((table n f) i)` where `(v i)` was — which is `(f i)`. The table is gone on
// both sides, so `n` still appears once. What looked like duplication is the
// step that ERASES the intermediate.
//
// Purity is still required, because β may not move an impure term at all
// (ADR 0010).
//
// A rule under an obligation, a segment's formation (segment), is still a
// description: copying it copies the obligation, which is decided at each copy
// and erased.
func (e *Env) duplicableTable(t *Term) bool {
	in, ms := peelWheres(t)
	for _, m := range ms {
		if !e.pureTerm(m.Kids[2], map[string]bool{}) {
			return false
		}
	}
	return isForm(in, e, "table") && len(in.Kids) == 3 &&
		e.pureTerm(in, map[string]bool{})
}

// AscribeName is the structural name that carries a range on a TERM.
//
// `(the "int 0 1606938…" e)` says the value of `e` is in that set. It exists
// because a declaration on a signature is a BOUNDARY, and whole-program
// reduction removes every boundary that is not an entry point — so a range that
// the analysis cannot re-derive has nowhere to live once its `sig` is inlined
// away.
//
// The type is a STRING, canonicalised by TypeName exactly as a signature's is,
// so no new term kind is needed and every consumer uses the same IntRangeBig
// and ValueType it already uses on a signature. An endpoint may be an
// expression — `(int 0 (pow 2 200))` — and TypeName has already evaluated it at
// arbitrary precision, which is why the payload is a string and not a term.
const AscribeName = "the"

// ascribeResult wraps a definition's result in an ascription, under whatever
// binders it has. A `Fn` stores its body with de Bruijn indices, so wrapping
// that body disturbs nothing: the ascription introduces no binder and no free
// name.
func ascribeResult(body *Term, ty string) *Term {
	if body == nil || ty == "" {
		return body
	}
	if body.Kind == KFn {
		return FnClosed(body.Params, ascribeResult(body.Closed(), ty))
	}
	return App(Name(AscribeName), Str(ty), body)
}

// ParamUsed reports whether the definition `name` uses its parameter j on every
// path of its body as written: the usage inference the relevance check reads
// (consumes). abstract.md §8.1's tree usage is held to the same answer.
func (e *Env) ParamUsed(name string, j int) bool {
	fn, ok := e.Defs[name]
	if !ok || fn.Kind != KFn || j >= len(fn.Params) {
		return false
	}
	return e.consumes(Name(name), fn, j)
}

// instantiate is the δ-rule of a static application (spec/typevalues.md §5):
// f's first Static[f] arguments, normalized, must each be a set; then the
// result is f#[T̄] and the arguments after them. ok is false while one is not a
// set yet, a variable a later β fills.
func (e *Env) instantiate(f string, args []*Term, fuel *int) (*Term, []*Term, bool, error) {
	n := e.Static[f]
	sets := make([]string, n)
	for i := 0; i < n; i++ {
		a, err := normalize(args[i], e, fuel)
		if err != nil {
			return nil, nil, false, err
		}
		if a.Kind != KName || !e.Sets[a.Name] {
			if a.Kind == KName && !e.Prim[a.Name] && e.Defs[a.Name] == nil || a.Kind == KBound {
				return nil, nil, false, nil // a variable: not decided yet
			}
			return nil, nil, false, fmt.Errorf("%s's static argument %d is %s, which is not a set; a "+
				"declaration over sets is applied to types (spec/typevalues.md §5)", ShowName(declared(f)), i+1, ShowTerm(a))
		}
		sets[i] = a.Name
	}
	if e.Instantiate == nil {
		return nil, nil, false, fmt.Errorf("%s is applied to sets, and this environment instantiates none", declared(f))
	}
	inst, err := e.Instantiate(f, sets)
	if err != nil {
		return nil, nil, false, err
	}
	e.Prim[inst], e.Pure[inst] = true, e.Pure[f]
	if r, ok := e.Requires[f]; ok {
		e.Requires[inst], e.ReqParams[inst] = r, e.ReqParams[f]
	}
	return Name(inst), args[n:], true, nil
}

// declared is how a diagnostic names a primitive: a fallible declaration's raw
// call, #raw:MODULE:NAME (emit/retract.go), by the name the declaration wrote.
func declared(n string) string {
	if strings.HasPrefix(n, "#raw:") {
		return strings.ReplaceAll(strings.TrimPrefix(n, "#raw:"), ":", ".")
	}
	return n
}

// staged is Law S of spec/typevalues.md §1: a residual holds no set, and no
// declaration over sets that was not applied to them.
func (e *Env) staged(t *Term) error {
	var walk func(t *Term) error
	walk = func(t *Term) error {
		if t.Kind == KName {
			switch {
			case t.Name == "..":
				return fmt.Errorf("a range, lo..hi, is a set, a static value, and has no value at run time: " +
					"it is an argument consumed by staging, by inf, sup or a definition over sets " +
					"(spec/typevalues.md §1); here it reaches the program as a value")
			case (t.Name == "inf" || t.Name == "sup") && e.Prim[t.Name]:
				return fmt.Errorf("%s takes a range with literal ends, lo..hi, and gives its %s element at "+
					"compile time; here its argument is not one, so it has no value", t.Name,
					map[string]string{"inf": "least", "sup": "greatest"}[t.Name])
			}
			switch {
			case e.Static[t.Name] > 0:
				return fmt.Errorf("%s takes %d static argument%s, a set for each, and is not applied to "+
					"them, so which function it is is not determined; a static argument is never defaulted "+
					"(spec/typevalues.md §3)", declared(t.Name), e.Static[t.Name], plural(e.Static[t.Name], "", "s"))
			case e.Sets[t.Name] && !e.Prim[t.Name]:
				return fmt.Errorf("%s is a set, a static value, and has no value at run time; a set is an "+
					"argument of a declaration's static parameter (spec/typevalues.md §1)", t.Name)
			}
			return nil
		}
		kids := t.Kids
		// A CLAIM'S SET is not a value: `(the S e)` ascribes e to S, and erasure
		// drops S (abstract.md §5, ε(the(S; e)) = ε(e)), so Law S does not
		// reach it. Only e is a value.
		if t.Kind == KApp && len(kids) == 3 && kids[0].Kind == KName && kids[0].Name == "the" {
			kids = kids[2:]
		}
		for _, k := range kids {
			if err := walk(k); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(t)
}

// resolveSetAliases rewrites each atom `alias.X` of the canonical set ty whose
// alias the module imports to that module's path, `strings.Replacer` ↦
// `go/strings.Replacer`: ρ_m for a set. Every other atom is kept.
func (m *Module) resolveSetAliases(ty string) string {
	if !strings.Contains(ty, ".") {
		return ty
	}
	var b strings.Builder
	word := func(w string) string {
		i := strings.Index(w, ".")
		if i <= 0 {
			return w
		}
		path, ok := m.Uses[w[:i]]
		if !ok || path == w[:i] {
			return w
		}
		return path + w[i:]
	}
	start := 0
	for i := 0; i <= len(ty); i++ {
		if i == len(ty) || ty[i] == ' ' || ty[i] == '(' || ty[i] == ')' || ty[i] == ',' {
			b.WriteString(word(ty[start:i]))
			if i < len(ty) {
				b.WriteByte(ty[i])
			}
			start = i + 1
		}
	}
	return b.String()
}

// isSumMember reports whether name is a constructor of a variant m declares: a
// `pub` variant's tags are in its module's interface (ADR 0078 item 1).
func isSumMember(m *Module, name string) bool {
	for _, s := range m.Sums {
		for _, v := range s.Variants {
			if v.Name == name {
				return true
			}
		}
	}
	return false
}

// setInTerm refuses a set operator where a term is: a computation, which needs
// a representation, and no set has one (ADR 0049); or a claim's formula, where
// membership is not built and an atom over an unbound name would be assumed at
// an entry point and undecidable at a call.
func setInTerm(name string) error {
	why, ok := setOperators[name]
	if !ok {
		return nil
	}
	shown := ShowName(name)
	if name == SepName {
		shown = "set { x ∈ S : φ }"
	}
	return fmt.Errorf("%s is %s. A set stands in a set position, a binder `x ∈ S` or a binder's claim "+
		"`b ∈ S = e`; in a computation, or a claim's formula, a set has no representation (ADR 0049, "+
		"spec/j.md's status)", shown, why)
}

// setOperatorIn is the first set operator a term names, or "".
func setOperatorIn(t *Term) string {
	if t == nil {
		return ""
	}
	if t.Kind == KName {
		if _, ok := setOperators[t.Name]; ok {
			return t.Name
		}
	}
	for _, k := range t.Kids {
		if n := setOperatorIn(k); n != "" {
			return n
		}
	}
	return ""
}

// setOperators are J's operators on sets, which D passes through as names
// wherever they were written; in a term they are refused with what each would
// be (resolve). Membership as a test is the characteristic function χ_S, not
// built: for a range it is the derived lo ≤ e ∧ e ≤ hi.
var setOperators = map[string]string{
	"∈":  "membership, the test χ_S(e), which is not built; for a range write `lo <= e && e <= hi`",
	"∉":  "non-membership, ¬χ_S(e), which is not built; for a range write `e < lo || e > hi`",
	"..": "a range, a set of integers",
	SepName: "a separation, the set { x ∈ S : φ } of S's elements for which φ holds",
	"∪":  "union, an operation of the set algebra",
	"∩":  "intersection, an operation of the set algebra",
	"∖":  "difference, an operation of the set algebra",
	"×":  "the product of sets",
	"⊆":  "inclusion, a relation between sets, which a binder `E ⊆ S` and an `axiom` state",
	"⊂":  "strict inclusion, a relation between sets",
}

// qualifySig resolves a signature's formulas as its definition's body is
// resolved: a name its module defines, and no parameter binds, is that module's
// definition. A `where` calling a helper of its module, lib/bytes' width(c),
// was read as a bare name nothing defines, and the obligation never reduced
// (ADR 0084's build). The main module's names are unqualified already.
func (m *Module) qualifySig(sig *Sig) *Sig {
	if sig == nil || m.Path == "" {
		return sig
	}
	params := map[string]bool{ResultName: true}
	for _, p := range sig.Params {
		params[p.Name] = true
	}
	var q func(t *Term, bound map[string]bool) *Term
	q = func(t *Term, bound map[string]bool) *Term {
		if t == nil {
			return nil
		}
		switch t.Kind {
		case KName:
			if _, own := m.Defs[t.Name]; own && !bound[t.Name] && !params[t.Name] {
				return Name(qualify(m.Path, t.Name))
			}
			return t
		case KFn:
			inner := map[string]bool{}
			for k := range bound {
				inner[k] = true
			}
			for _, p := range t.Params {
				inner[p] = true
			}
			body := q(t.Kids[0], inner)
			if body == t.Kids[0] {
				return t
			}
			c := *t
			c.Kids = []*Term{body}
			return &c
		}
		var kids []*Term
		for i, k := range t.Kids {
			nk := q(k, bound)
			if nk != k && kids == nil {
				kids = append([]*Term(nil), t.Kids...)
			}
			if kids != nil {
				kids[i] = nk
			}
		}
		if kids == nil {
			return t
		}
		c := *t
		c.Kids = kids
		return &c
	}
	out := *sig
	out.Where = q(sig.Where, map[string]bool{})
	out.Written = q(sig.Written, map[string]bool{})
	out.Ensures = q(sig.Ensures, map[string]bool{})
	out.ResultRange = q(sig.ResultRange, map[string]bool{})
	return &out
}

// hoistWheres replaces each where-mark inside t by its body, collecting the
// marks, outermost first, so an obligation a condition reached is kept as one.
func hoistWheres(t *Term, out *[]*Term) *Term {
	if t == nil {
		return nil
	}
	if IsRequireWhere(t) {
		*out = append(*out, t)
		return hoistWheres(t.Kids[3], out)
	}
	var kids []*Term
	for i, k := range t.Kids {
		nk := hoistWheres(k, out)
		if nk != k && kids == nil {
			kids = append([]*Term(nil), t.Kids...)
		}
		if kids != nil {
			kids[i] = nk
		}
	}
	if kids == nil {
		return t
	}
	c := *t
	c.Kids = kids
	return &c
}

// adopt is δ for a buffer whose region is given, (adopt e (fn (b) B)): a fresh
// region holding e's value, then B. Under the state-passing translation
// (Theorem V) a scope is a state transformer, so when e is itself a fresh
// region, a scope (build n (fn (r) R)) that hands back its buffer, the two
// compose into one region:
//
//	(adopt (build n (fn (r) R)) (fn (b) B))  ⟶  (build n (fn (r) ((fn (b) B) R)))
//
// the MOVE of a linear result (ADR 0084): one allocation, nothing copied, and
// nothing else can name the region, since only the producer's scope did. Any
// other e is a table something else may hold, and its value is copied into a
// fresh region, element by element, which is what `var b := e` means.
func (e *Env) adopt(args []*Term, fuel *int) (*Term, error) {
	v, err := normalize(args[0], e, fuel)
	if err != nil {
		return nil, err
	}
	in, ms := peelWheres(v)
	// A LET AROUND A FRESH REGION is still one: the adoption moves inside it,
	// under a fresh name, so nothing of the caller's is captured.
	// Built closed (locally nameless): the let's body stays under its binder,
	// and the caller's λ moves under it, shifted by one.
	if lam, val, ok := letClosed(in); ok && e.freshRegion(lam.Body()) {
		inner := App(Name(AdoptName), lam.Closed(), shift(args[1], 0, 1))
		out, err := normalize(App(FnClosed(lam.Params, inner), val), e, fuel)
		if err != nil {
			return nil, err
		}
		return wrapWheres(out, ms), nil
	}
	if isForm(in, e, "build") && len(in.Kids) == 3 && in.Kids[2].Kind == KFn && len(in.Kids[2].Params) == 1 {
		lam := in.Kids[2]
		moved := App(Name("build"), in.Kids[1], FnClosed(lam.Params, App(shift(args[1], 0, 1), lam.Closed())))
		out, err := normalize(moved, e, fuel)
		if err != nil {
			return nil, err
		}
		return wrapWheres(out, ms), nil
	}
	if !e.pureTerm(v, map[string]bool{}) {
		ns := e.freshNames(1, v, args[1])
		return normalize(App(Fn(ns, App(Name(AdoptName), Name(ns[0]), args[1])), v), e, fuel)
	}
	// THE COPY: element i of the fresh region is e's, by build's fill loop.
	ns := e.freshNames(3, v, args[1])
	i, x, n := ns[0], ns[1], ns[2]
	fill := App(Name("loop"),
		Fn([]string{i, x}, App(Name("if"), App(Name(">="), Name(i), Name(n)),
			Name(x),
			App(Name("again"), App(Name("+"), Name(i), Int(1)),
				App(Name("set"), Name(x), Name(i), App(v, Name(i)))))),
		Int(0), Name(x))
	copied := App(Fn([]string{n}, App(Name("build"), Name(n), Fn([]string{x}, App(args[1], fill)))),
		App(Name("len"), v))
	return normalize(copied, e, fuel)
}

// owned decides a linear result (ADR 0084 item 7): the definition moves out a
// region it owns, so its value is a fresh one, a buffer's scope that hands its
// buffer back, (build n (fn (b) …)). Anything else, a table it was lent, a
// parameter, a table something else names, would give the caller a writable
// name for a region another holds, and is refused. Decided where the
// definition is inlined, per instance, like its other claims.
func (e *Env) owned(body *Term, fuel *int) (*Term, error) {
	v, err := normalize(body, e, fuel)
	if err != nil {
		return nil, err
	}
	if e.freshRegion(v) {
		return v, nil
	}
	in, _ := peelWheres(v)
	return nil, fmt.Errorf("a definition whose result is linear moves out a region it owns, a buffer " +
		"its body makes (`var r := array { … }`, then `r`); this one returns %s, which it does not " +
		"own, so the caller would hold a writable name for a region another holds (ADR 0084)",
		ShowTerm(in))
}

// freshRegion reports a fresh region: a buffer's scope, (build n (fn (b) …)),
// under where-marks and lets.
func (e *Env) freshRegion(t *Term) bool {
	in, _ := peelWheres(t)
	if _, _, body, ok := letParts(in); ok {
		return e.freshRegion(body)
	}
	return isForm(in, e, "build") && len(in.Kids) == 3 && in.Kids[2].Kind == KFn && len(in.Kids[2].Params) == 1
}

// letParts reads a let, (let v (fn (x) b)) or ((fn (x) b) v): its binder, its
// value and its body, opened with the binder's name.
func letParts(t *Term) (string, *Term, *Term, bool) {
	if t.Kind != KApp || len(t.Kids) == 0 {
		return "", nil, nil, false
	}
	if t.Kids[0].Kind == KName && t.Kids[0].Name == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn &&
		len(t.Kids[2].Params) == 1 {
		return t.Kids[2].Params[0], t.Kids[1], t.Kids[2].Body(), true
	}
	if t.Kids[0].Kind == KFn && len(t.Kids[0].Params) == 1 && len(t.Kids) == 2 {
		return t.Kids[0].Params[0], t.Kids[1], t.Kids[0].Body(), true
	}
	return "", nil, nil, false
}

// letClosed reads a let as its λ and its value, the λ's body still closed.
func letClosed(t *Term) (*Term, *Term, bool) {
	if t.Kind != KApp || len(t.Kids) == 0 {
		return nil, nil, false
	}
	if t.Kids[0].Kind == KName && t.Kids[0].Name == "let" && len(t.Kids) == 3 && t.Kids[2].Kind == KFn &&
		len(t.Kids[2].Params) == 1 {
		return t.Kids[2], t.Kids[1], true
	}
	if t.Kids[0].Kind == KFn && len(t.Kids[0].Params) == 1 && len(t.Kids) == 2 {
		return t.Kids[0], t.Kids[1], true
	}
	return nil, nil, false
}

// closedBig is the value of a closed integer term, literals under +, − and ·,
// exactly in ℤ: how staging leaves a literal past the word.
func closedBig(t *Term) (*big.Int, bool) {
	switch {
	case t.Kind == KInt:
		return big.NewInt(t.Int), true
	case t.Kind == KApp && len(t.Kids) == 3 && t.Kids[0].Kind == KName:
		a, ok1 := closedBig(t.Kids[1])
		b, ok2 := closedBig(t.Kids[2])
		if !ok1 || !ok2 {
			return nil, false
		}
		switch t.Kids[0].Name {
		case "+":
			return a.Add(a, b), true
		case "-":
			return a.Sub(a, b), true
		case "*":
			return a.Mul(a, b), true
		}
	case t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == "-":
		if a, ok := closedBig(t.Kids[1]); ok {
			return a.Neg(a), true
		}
	}
	return nil, false
}

// foldClosedComparison decides a comparison of two closed integer terms.
func foldClosedComparison(op string, a, b *Term) (*Term, bool) {
	x, ok1 := closedBig(a)
	y, ok2 := closedBig(b)
	if !ok1 || !ok2 {
		return nil, false
	}
	c := x.Cmp(y)
	switch op {
	case "<":
		return Bool(c < 0), true
	case "<=":
		return Bool(c <= 0), true
	case ">":
		return Bool(c > 0), true
	case ">=":
		return Bool(c >= 0), true
	case "=":
		return Bool(c == 0), true
	}
	return nil, false
}

// needsReduction reports a where-mark's condition holding an obligation of
// its own, or the length of a table that is no name, which normalization
// folds.
func needsReduction(t *Term) bool {
	if t == nil {
		return false
	}
	if IsRequireWhere(t) {
		return true
	}
	if t.Kind == KApp && len(t.Kids) == 2 && t.Kids[0].Kind == KName && t.Kids[0].Name == "len" &&
		t.Kids[1].Kind == KApp {
		return true
	}
	for _, k := range t.Kids {
		if needsReduction(k) {
			return true
		}
	}
	return false
}

// nonnegative marks a ≥ 0 as a range obligation, a ∈ [0, the word's top]:
// decided as a parameter's range is, the IR's intervals included, which know
// a table's element set and a loop's exit value where the refinement layer
// does not. A literal is decided here.
func (e *Env) nonnegative(def, param string, a *Term) *Term {
	if a.Kind == KInt && a.Int >= 0 {
		return a
	}
	ty := fmt.Sprintf("int 0 %d", e.Word.Hi)
	return &Term{Kind: KApp, Kids: []*Term{Name(RequireName), Str(def), Str(param), Str(ty), a}}
}
