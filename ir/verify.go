package ir

import (
	"errors"
	"fmt"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// THE VERIFIER: W1–W10 of spec §3. It runs after lowering and after every pass
// that rewrites the IR, and every consumer may assume what it accepts. Each
// message begins with the rule it enforces, so a planted-fault test can say
// which rule caught which fault.

// Verify checks a program against the target it was lowered for.
func Verify(tg *emit.Target, p *Program) error {
	var errs []string
	add := func(format string, args ...any) {
		if len(errs) < 20 {
			errs = append(errs, fmt.Sprintf(format, args...))
		}
	}
	if p.Header != nil { // W10: covering, for a file that declared its operations
		declared := map[string]bool{}
		for _, h := range p.Header {
			declared[h] = true
		}
		for _, o := range p.Ops() {
			if !declared[o] {
				add("W10: %s is used and not in the header's (ops …)", o)
			}
		}
	}
	globals := map[string]bool{}
	for _, g := range p.Globals {
		globals[g.Name] = true
	}
	for _, f := range p.Funcs {
		v := &verifier{tg: tg, p: p, f: f, add: func(format string, args ...any) {
			add(f.Name+": "+format, args...)
		}, globals: globals}
		v.run()
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

type verifier struct {
	tg      *emit.Target
	p       *Program
	f       *Func
	add     func(string, ...any)
	globals map[string]bool

	defined []bool
	scope   []bool
	undo    []V
	buf     []bool
	alias   []V   // a π or an ascription is its source renamed (L9)
	depth   []int // the loop depth at each value's definition (W7)
	def     map[V]*Stmt
	lits    map[litKey]litSet // implied's memo: a guard's literals, by its value and side
}

type litKey struct {
	c V
	b bool
}

type litSet struct {
	lits      []literal
	reachable bool
}

func (v *verifier) run() {
	nv := v.f.NV()
	v.defined = make([]bool, nv)
	v.scope = make([]bool, nv)
	v.depth = make([]int, nv)
	v.alias = make([]V, nv)
	v.def = map[V]*Stmt{}
	for i := range v.alias {
		v.alias[i] = V(i)
	}
	// W1: one definition per value.
	def := func(x V, d int) {
		if x < 0 || int(x) >= nv {
			v.add("W1: %%%d is outside the function's values", x)
			return
		}
		if v.defined[x] {
			v.add("W1: %%%d is defined twice", x)
		}
		v.defined[x] = true
		v.depth[x] = d
	}
	var defs func(r *Region, d int)
	defs = func(r *Region, d int) {
		for _, x := range r.Params {
			def(x, d)
		}
		for _, pi := range r.Pis {
			def(pi.V, d)
			if pi.Of >= 0 && int(pi.Of) < nv && int(pi.V) < nv {
				v.alias[pi.V] = pi.Of
			}
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, x := range s.Res {
				def(x, d)
				v.def[x] = s
			}
			if s.Op == OThe && len(s.Res) == 1 && len(s.Args) == 1 {
				v.alias[s.Res[0]] = s.Args[0]
			}
			for _, sub := range s.Sub {
				if s.Op == OLoop {
					defs(sub, d+1)
				} else {
					defs(sub, d)
				}
			}
		}
		if r.T == TBranch {
			defs(r.Then, d)
			defs(r.Else, d)
		}
	}
	for _, x := range v.f.Params {
		def(x, 0)
	}
	defs(v.f.Body, 0)
	for x := 0; x < nv; x++ {
		if v.defined[x] && v.f.Types[x] == "" {
			v.add("W5: %%%d has no type", x)
		}
	}
	v.buf = buffers(v.tg, v.f)

	// W2–W6: one walk, with the scope and the region's kind.
	for _, x := range v.f.Params {
		v.enter(x)
	}
	yields := v.region(v.f.Body, ctx{kind: kFunc})
	for _, ys := range yields {
		if len(ys) != len(v.f.Results) {
			v.add("W4: the function yields %d values and declares %d results", len(ys), len(v.f.Results))
			break
		}
		for j, y := range ys {
			v.flowDeclared(y, v.f.Results[j], "the function's result")
		}
	}
	// W7: linearity, path by path.
	lin := &linear{v: v}
	lin.region(v.f.Body, map[V]bool{}, 0)
	// W9: the print stage.
	if v.p.Stage == StageP {
		v.f.Walk(func(r *Region) {
			for i := range r.Stmts {
				s := &r.Stmts[i]
				switch {
				case s.Op == OThe || s.Op == ORequire:
					v.add("W9: (%s …) in IR_P; the step to IR_P erases it", s.Op)
				case s.Op.IsArith() && s.Mode != MExact && s.Mode != MTrap && s.Mode != MWrap && s.Mode != MWrapPair &&
					!(s.Op == ONeg && v.f.Types[s.Args[0]] == core.F64Name):
					v.add("W9: (%s …) with no mode in IR_P", s.Op)
				}
			}
		})
		// A type is final where the host types its values; on a host with no
		// static types (JavaScript, x86) `any` is the faithful type.
		for x := 0; x < nv && Typed(v.tg); x++ {
			if v.defined[x] && strings.Contains(v.f.Types[x], "any") {
				v.add("W9: %%%d's type %s is not final", x, v.f.Types[x])
			}
		}
	}
}

func (v *verifier) enter(x V) {
	if x >= 0 && int(x) < len(v.scope) && !v.scope[x] {
		v.scope[x] = true
		v.undo = append(v.undo, x)
	}
}

func (v *verifier) use(x V, where string) {
	if x < 0 || int(x) >= len(v.scope) || !v.scope[x] {
		v.add("W2: %%%d is used in %s outside its scope", x, where)
	}
}

func (v *verifier) ty(x V) string {
	if x < 0 || int(x) >= len(v.f.Types) {
		return ""
	}
	return v.f.Types[x]
}

// flow is W5's flow rule on one edge: the checker's own relation (types.md),
// with a buffer read as the table it is (ADR 0020), and a host alias equal to
// the language type it realizes (spec §4.2).
func (v *verifier) flow(x V, want, where string) {
	if !v.agrees(v.ty(x), want) {
		v.add("W5: %%%d is %s and flows into %s, which is %s", x, v.ty(x), where, want)
	}
}

// flowDeclared is W5 on an edge into a DECLARED type: a primitive's argument
// or the function's result. That a value lies in a declared range is an
// obligation (ADR 0028) the pipeline discharged before IR_P; the IR's own
// domain may be weaker and need not re-prove it, so the edge is checked by
// sort in either stage. Edges between values, which the IR's domain wrote,
// are checked by containment in IR_P.
func (v *verifier) flowDeclared(x V, want, where string) {
	got := v.ty(x)
	if sortOf(v.tg, got) == sortOf(v.tg, want) || v.agrees(sortOf(v.tg, got), sortOf(v.tg, want)) {
		return
	}
	v.flow(x, want, where)
}

func (v *verifier) agrees(got, want string) bool {
	if v.p.Stage == StageP {
		if subtype(v.tg, got, want) {
			return true
		}
		// A value above the word held as LIMBS is a table on this target
		// (ρ_T's kernel, as in IR_A): a `big` and a limb table are compared as
		// the sort they share. Its bound is enforced by big-fit (ADR 0029), not
		// by a range on the table.
		if v.tg.BigRepr == "limbs" && (involvesBig(v.tg, got) || involvesBig(v.tg, want)) {
			got, want = sortOf(v.tg, got), sortOf(v.tg, want)
		}
	} else {
		// IR_A: SORTS, not ranges. Whether a value lies in a declared range is
		// an obligation (ADR 0028) the analyses discharge, not a typing
		// question, so every range is read as the representation ρ_T gives it.
		got, want = sortOf(v.tg, got), sortOf(v.tg, want)
	}
	got, want = unbuffer(got), unbuffer(want)
	if got == want || got == "any" || want == "any" || got == "" || want == "" {
		return true
	}
	// THE TWO WORD REALIZATIONS DO NOT AGREE (ADR 0033). The term checker
	// types both as ℤ, because it has no representation to check; W5 is where
	// representation is checked, and a u64 flowing where an int is held (or
	// the reverse) is a missing conversion, which a host would type-reject or
	// compute in the wrong width.
	if rg, rw := sortOf(v.tg, got), sortOf(v.tg, want); rg != rw && isWordRep(rg) && isWordRep(rw) {
		return false
	}
	// NOR DO A WORD AND ARBITRARY PRECISION: an exact value where a word is
	// held is a narrowing no one proved, and a word where an exact value is
	// held is a missing `big-of` (ir/big.go).
	if rg, rw := sortOf(v.tg, got), sortOf(v.tg, want); (rg == core.BigType && isWordRep(rw)) || (rw == core.BigType && isWordRep(rg)) {
		return false
	}
	if v.tg.Agrees(got, want) {
		return true
	}
	scalar := map[string]bool{"int": true, "f64": true, "bool": true, "string": true}
	if scalar[got] || scalar[want] {
		return false // ADR 0030: a language scalar is not a host type by realization
	}
	hg, hw := v.tg.HostType(got), v.tg.HostType(want)
	return hg != "" && hg == hw && !strings.HasPrefix(hg, "/*")
}

// hasLength is whether a value of type t has a length on this target: a
// property of its REPRESENTATION, ρ_T(t). A table, a buffer, a map, a string
// and a host type have one; a float and a bool do not; an integer has one only
// where ρ_T realizes it as a table of limbs, `(big-repr limbs)` (ADR 0029).
func (v *verifier) hasLength(t string) bool {
	switch t {
	case "f64", "bool":
		return false
	}
	if s := sortOf(v.tg, t); s == "int" || s == core.U64Type {
		return false
	} else if s == core.BigType {
		return v.tg.BigRepr == "limbs"
	}
	return true
}

type kind uint8

const (
	kFunc  kind = iota
	kArm        // an `if` arm
	kScope      // a build, build-map or tabulate body
	kLoop       // a loop body
)

type ctx struct {
	kind kind
	// The arm's guard: entered only when guard has the value side (Theorem
	// E). guarded is false for a region no branch enters.
	guard   V
	side    bool
	guarded bool
}

// region walks r in scope, checks W2–W6, and returns the argument lists of the
// yields (outside a loop) or of the breaks and continues (in a loop body) of
// its terminator tree, which its owner checks against its results (W4).
func (v *verifier) region(r *Region, c ctx) [][]V {
	mark := len(v.undo)
	defer func() {
		for _, x := range v.undo[mark:] {
			v.scope[x] = false
		}
		v.undo = v.undo[:mark]
	}()
	for _, x := range r.Params {
		v.enter(x)
	}
	for _, pi := range r.Pis {
		v.use(pi.Of, "a π")
		v.use(pi.Other, "a π")
		if pi.Len {
			if t := v.ty(pi.Of); !v.hasLength(t) {
				v.add("W6: a length π on %%%d, whose type %s has no length", pi.Of, t)
			}
		} else if !v.agrees(v.ty(pi.V), v.ty(pi.Of)) {
			v.add("W6: π %%%d is %s and its source %%%d is %s", pi.V, v.ty(pi.V), pi.Of, v.ty(pi.Of))
		}
		v.justified(pi, c)
		v.enter(pi.V)
	}
	for i := range r.Stmts {
		v.stmt(&r.Stmts[i])
	}
	return v.terminator(r, c)
}

// A π IS JUSTIFIED BY ITS ARM'S GUARD (Theorem E, spec §6): the region is
// entered only when its guard c has the value b, and the π states a literal
// ⟦c⟧ = b implies. The literals implied are a homomorphism over how lowering
// builds a guard (implied), compared modulo π and ascription, a π being its
// source renamed (L9), and a constant by its literal. W6 checked a π's types
// only, and a π on the wrong table passed it (work-2026-10-22 §5).
func (v *verifier) justified(pi Pi, c ctx) {
	if !c.guarded {
		v.add("W6: π %%%d is on a region no branch enters, so no guard justifies it (Theorem E)", pi.V)
		return
	}
	lits, reachable := v.implied(c.guard, c.side, 0)
	if !reachable {
		return // no execution enters the arm: every literal holds of it
	}
	for _, l := range lits {
		if v.states(pi, l) {
			return
		}
	}
	what := fmt.Sprintf("%%%d %s %%%d", pi.Of, pi.Rel, pi.Other)
	if pi.Len {
		what = fmt.Sprintf("(len %%%d) %s %%%d", pi.Of, pi.Rel, pi.Other)
	}
	side := "true"
	if !c.side {
		side = "false"
	}
	v.add("W6: π %%%d states %s, which its arm's guard %%%d = %s does not imply (Theorem E)", pi.V, what, c.guard, side)
}

// literal is x REL y.
type literal struct {
	x, y V
	rel  string
}

// implied is the set of literals ⟦c⟧ = b implies. A comparison implies its
// literal, or its negation's, and an ascription or a π its source's. A guard
// built as `c = if a { arm₀ } else { arm₁ }` (lowering's ∧, ∨ and ¬, and
// chains of them) implies the MEET over the routes by which c can be b: a
// route is a path through an arm's branch tree to a yield, and implies its
// path's guards and what its yield implies being b; a constant yield is a
// route only if it is b.
//
//	I(x REL y, b) = {x REL y} if b, {x ¬REL y} if not
//	I(if a {arm₀} else {arm₁}, b) = ⋂ { I(a, true) ∪ R : R a route of arm₀ to b }
//	                              ∩ ⋂ { I(a, false) ∪ R : R a route of arm₁ to b }
//
// So a ∧ t is I(a, true) ∪ I(t, true) when true, and nothing atomic when
// false; a ∨ e is I(a, false) ∪ I(e, false) when false. The second result is
// false when no route reaches b: the arm is unreachable, and every literal
// holds of it vacuously.
func (v *verifier) implied(c V, b bool, depth int) ([]literal, bool) {
	if depth > 64 {
		return nil, true
	}
	c = v.root(c)
	if m, ok := v.lits[litKey{c, b}]; ok {
		return m.lits, m.reachable
	}
	lits, reachable := v.implied1(c, b, depth)
	if v.lits == nil {
		v.lits = map[litKey]litSet{}
	}
	v.lits[litKey{c, b}] = litSet{lits, reachable}
	return lits, reachable
}

func (v *verifier) implied1(c V, b bool, depth int) ([]literal, bool) {
	s := v.def[c]
	if s == nil {
		return nil, true
	}
	switch {
	case s.Op.IsCmp() && len(s.Args) == 2:
		rel := relOf(s.Op)
		if !b {
			rel = negate(rel)
		}
		return []literal{{s.Args[0], s.Args[1], rel}}, true
	case s.Op == OConst && s.Lit != nil && s.Lit.Kind == core.KBool:
		return nil, (s.Lit.Int != 0) == b
	case s.Op == OCall && len(s.Args) == 2 && v.callRel(s.Name) != "":
		// The unsigned word's order is ℤ's on the integers it holds (words.go),
		// and a host's comparison is the one the target's classification says,
		// as lowering read it when it placed the π (lower.go, cond): on a
		// value no analysis reads as an integer, the π is idle (L9).
		rel := v.callRel(s.Name)
		if !b {
			rel = negate(rel)
		}
		return []literal{{s.Args[0], s.Args[1], rel}}, true
	case s.Op == OThe && len(s.Args) == 1:
		return v.implied(s.Args[0], b, depth+1)
	case s.Op == OIf && len(s.Sub) == 2 && len(s.Res) == 1:
		var meet []literal
		any := false
		for i, arm := range s.Sub {
			guard, ok := v.implied(s.Args[0], i == 0, depth+1)
			if !ok {
				continue
			}
			for _, route := range v.routes(arm, b, depth+1) {
				lits := append(append([]literal(nil), guard...), route...)
				if !any {
					meet, any = lits, true
				} else {
					meet = v.intersect(meet, lits)
				}
			}
		}
		return meet, any
	}
	return nil, true
}

// routes are the literal sets of r's paths to a yield of a value that may be
// b: each branch's guard on the side taken, and the yield's being b.
func (v *verifier) routes(r *Region, b bool, depth int) [][]literal {
	switch r.T {
	case TBranch:
		var out [][]literal
		for _, side := range []struct {
			arm  *Region
			take bool
		}{{r.Then, true}, {r.Else, false}} {
			g, ok := v.implied(r.Cond, side.take, depth+1)
			if !ok {
				continue
			}
			for _, rt := range v.routes(side.arm, b, depth+1) {
				out = append(out, append(append([]literal(nil), g...), rt...))
			}
		}
		return out
	case TYield:
		if len(r.Args) != 1 {
			return nil
		}
		lits, ok := v.implied(r.Args[0], b, depth+1)
		if !ok {
			return nil
		}
		return [][]literal{lits}
	}
	return nil // no value leaves it: no route
}

// intersect is the literals of a that b states too.
func (v *verifier) intersect(a, b []literal) []literal {
	var out []literal
	for _, x := range a {
		for _, y := range b {
			if v.sameLit(x, y) {
				out = append(out, x)
				break
			}
		}
	}
	return out
}

func (v *verifier) sameLit(x, y literal) bool {
	return (x.rel == y.rel && v.same(x.x, y.x) && v.same(x.y, y.y)) ||
		(x.rel == flip(y.rel) && v.same(x.x, y.y) && v.same(x.y, y.x))
}

// root is a value with its π-parameters and ascriptions taken off: the value
// it renames (L9).
func (v *verifier) root(x V) V {
	for i := 0; i < 256 && x >= 0 && int(x) < len(v.alias); i++ {
		if v.alias[x] != x {
			x = v.alias[x]
			continue
		}
		// A CONVERSION BETWEEN REPRESENTATIONS is the identity on the integer
		// it holds (ADR 0083): the word selection converts only a value in
		// U ∩ S (words.go), so its comparison's operands are the π's.
		if s := v.def[x]; s != nil && s.Op == OCall && len(s.Args) == 1 && (s.Name == "u64-of" || s.Name == "int-of-u64") {
			x = s.Args[0]
			continue
		}
		break
	}
	return x
}

// callRel is a comparison call's relation: the unsigned word's (wordRel), or a
// host primitive's by the target's classification (classify), "" for another.
func (v *verifier) callRel(name string) string {
	if r := wordRel[name]; r != "" {
		return r
	}
	if p, ok := v.tg.Prims[name]; ok {
		if o, _, ok := classify(v.tg, name, p, 2, Options{}); ok && o.IsCmp() {
			return cmpRel[o]
		}
	}
	return ""
}

// wordRel is the unsigned word's comparisons' relations (words.go, wordOp).
var wordRel = map[string]string{"u64=": "eq", "u64<": "lt", "u64<=": "le", "u64>": "gt", "u64>=": "ge"}

// same reports one value: equal modulo π, or two constants of one literal.
func (v *verifier) same(a, b V) bool {
	a, b = v.root(a), v.root(b)
	if a == b {
		return true
	}
	sa, sb := v.def[a], v.def[b]
	if sa == nil || sb == nil || sa.Op != OConst || sb.Op != OConst || sa.Lit == nil || sb.Lit == nil ||
		sa.Lit.String() != sb.Lit.String() {
		return false
	}
	// one literal is one element of its sort: a constant's range type, which
	// the decision gives it at IR_P, is a fact about the same integer
	ta, tb := v.f.Types[a], v.f.Types[b]
	return ta == tb || (isIntegerType(ta) && isIntegerType(tb))
}

func isIntegerType(ty string) bool {
	if ty == "int" {
		return true
	}
	_, _, ok := core.RangeEnds(ty)
	return ok
}

// states reports whether π states literal l, as written or flipped: x REL y is
// y flip(REL) x. A length π states (len Of) REL Other, so its literal's side
// is a value defined as the length of a table that is Of, modulo π.
func (v *verifier) states(pi Pi, l literal) bool {
	match := func(x V, rel string, y V) bool {
		if rel != pi.Rel || !v.same(y, pi.Other) {
			return false
		}
		if !pi.Len {
			return v.same(x, pi.Of)
		}
		s := v.def[v.root(x)]
		return s != nil && s.Op == OLen && len(s.Args) == 1 && v.root(s.Args[0]) == v.root(pi.Of)
	}
	return match(l.x, l.rel, l.y) || match(l.y, flip(l.rel), l.x)
}

func (v *verifier) terminator(r *Region, c ctx) [][]V {
	for _, a := range r.Args {
		v.use(a, r.T.String())
	}
	switch r.T {
	case TYield:
		if c.kind == kLoop {
			v.add("W3: yield in a loop body; a loop is left by break")
			return nil
		}
		return [][]V{r.Args}
	case TBreak, TContinue:
		if c.kind != kLoop {
			v.add("W3: %s outside a loop body's terminator tree (exits are single-level, spec §1.3)", r.T)
			return nil
		}
		return [][]V{append([]V{V(r.T)}, r.Args...)} // tagged: the owner splits break from continue
	case TBranch:
		v.use(r.Cond, "a branch")
		if !v.agrees(v.ty(r.Cond), "bool") {
			v.add("W5: a branch on %%%d, which is %s", r.Cond, v.ty(r.Cond))
		}
		then, els := c, c
		then.guard, then.side, then.guarded = r.Cond, true, true
		els.guard, els.side, els.guarded = r.Cond, false, true
		return append(v.region(r.Then, then), v.region(r.Else, els)...)
	case TUnreachable:
		// NO EXIT, so it contributes nothing to its owner's results. It is
		// sound only if control really cannot fall out: the region's last
		// statement is the crash (spec/errors.md §8).
		if n := len(r.Stmts); n == 0 || r.Stmts[n-1].Op != OCall || !v.tg.IsBottom(r.Stmts[n-1].Name) {
			v.add("W3: unreachable, but the region does not end in a call into ∅ (abandon, or a declaration whose result is ∅), so control may fall out of it")
		}
		return nil
	}
	return nil
}

// sigma is Σ's arity table: operands and results, -1 for "any number".
var sigma = map[Op][2]int{
	OConst: {0, 1}, OGlobal: {0, 1},
	OAdd: {2, 1}, OSub: {2, 1}, OMul: {2, 1}, ONeg: {1, 1}, ODiv: {2, 1}, ORem: {2, 1},
	OAnd: {2, 1}, OOr: {2, 1}, OXor: {2, 1}, ONot: {1, 1}, OShl: {2, 1}, OShr: {2, 1}, OMod: {2, 1},
	OEq: {2, 1}, ONe: {2, 1}, OLt: {2, 1}, OLe: {2, 1}, OGt: {2, 1}, OGe: {2, 1},
	OIndex: {2, 1}, OLen: {1, 1}, OArray: {-1, 1}, OMap: {-1, 1}, ORead: {2, 2},
	OKeys: {1, 1}, OSet: {3, 1}, OInsert: {3, 1},
	OIf: {1, -1}, OLoop: {-1, -1}, OBuild: {1, -1}, OBuildMap: {1, -1}, OTabulate: {1, 1},
	OThe: {1, 1}, ORequire: {1, 0},
	ORestrict: {2, 1}, OAssume: {1, 0}, OZero: {0, 1},
	OCellRef: {1, 1}, OCellFresh: {0, 1}, OCellGet: {1, 1},
}

var subs = map[Op]int{OIf: 2, OLoop: 1, OBuild: 1, OBuildMap: 1, OTabulate: 1}

func (v *verifier) stmt(s *Stmt) {
	where := "(" + s.Op.String() + " …)"
	for _, a := range s.Args {
		v.use(a, where)
	}
	// A TABLE'S STORAGE (ADR 0092): one pointer, used by a statement that makes
	// a table.
	if len(s.Store) > 0 {
		if len(s.Store) != 1 || (s.Op != OBuild && s.Op != OTabulate && s.Op != OArray) {
			v.add("W4: %s takes no storage", where)
		}
		for _, a := range s.Store {
			v.use(a, where)
		}
	}
	// W4: arities.
	if s.Op == OCall {
		p, ok := v.tg.Prims[s.Name]
		if !ok {
			v.add("W10: %s is not a primitive of target %s", s.Name, v.tg.Name)
		} else {
			if len(p.Args) > 0 && len(s.Args) != len(p.Args) {
				v.add("W4: %s takes %d arguments, given %d", s.Name, len(p.Args), len(s.Args))
			}
			want := 1
			if len(p.Results) >= 2 {
				want = len(p.Results)
			}
			if len(s.Res) != want {
				v.add("W4: %s has %d results, bound to %d", s.Name, want, len(s.Res))
			}
			for j, a := range s.Args {
				if j < len(p.Args) {
					v.flowDeclared(a, p.Args[j], fmt.Sprintf("%s's argument %d", s.Name, j+1))
				}
			}
		}
	} else if ar, ok := sigma[s.Op]; ok {
		if ar[0] >= 0 && len(s.Args) != ar[0] {
			v.add("W4: %s takes %d operands, given %d", where, ar[0], len(s.Args))
		}
		if ar[1] >= 0 && len(s.Res) != ar[1] {
			v.add("W4: %s has %d results, bound to %d", where, ar[1], len(s.Res))
		}
		if s.Op == OMap && len(s.Args)%2 != 0 {
			v.add("W4: a map literal's operands are pairs")
		}
	} else {
		v.add("W4: an operation outside Σ, %d", s.Op)
	}
	if n, ok := subs[s.Op]; ok && len(s.Sub) != n {
		v.add("W4: %s owns %d regions, not %d", where, n, len(s.Sub))
		return
	} else if !ok && len(s.Sub) > 0 {
		v.add("W4: %s owns no region", where)
		return
	}
	if s.Op == OGlobal && !v.globals[s.Name] {
		v.add("W2: the global %s is not declared", s.Name)
	}
	// W5: operand sorts.
	// THE DOUBLES' OWN OPERATIONS (ADR 0086, numbers.md §6.2) are those F64 is
	// closed under, so no rounding is needed: N's order and equality on two
	// doubles, which IEEE 754 compares as reals, and negation. Every other
	// operation on a double is a realization's call (doubles.go).
	doubles := len(s.Args) > 0
	for _, a := range s.Args {
		doubles = doubles && v.f.Types[a] == core.F64Name
	}
	switch {
	case s.Op == ONeg && doubles:
		v.flow(s.Res[0], core.F64Name, where)
	case s.Op.IsCmp() && doubles:
	case s.Op.IsArith():
		for _, a := range s.Args {
			v.flow(a, "int", where)
		}
	case s.Op.IsCmp():
		for _, a := range s.Args {
			v.flow(a, "int", where)
		}
	case s.Op == OIf || s.Op == ORequire || s.Op == OAssume:
		v.flow(s.Args[0], "bool", where)
	case s.Op == ORestrict:
		v.flow(s.Args[1], "int", where+"'s length")
	case s.Op == OIndex, s.Op == OSet:
		if len(s.Args) >= 2 {
			v.flow(s.Args[1], "int", where+"'s index")
		}
	}
	// The regions, and W3/W4 at their exits.
	switch s.Op {
	case OIf:
		for i, sub := range s.Sub {
			v.results(v.region(sub, ctx{kind: kArm, guard: s.Args[0], side: i == 0, guarded: true}), s.Res, where)
		}
	case OBuild, OBuildMap, OTabulate:
		body := s.Sub[0]
		if len(body.Params) != 1 {
			v.add("W4: %s's region has %d parameters, not 1", where, len(body.Params))
		}
		ys := v.region(body, ctx{kind: kScope})
		if s.Op == OTabulate {
			for _, y := range ys {
				if len(y) != 1 {
					v.add("W4: a tabulate's rule yields %d values, not 1", len(y))
				}
			}
		} else {
			v.results(ys, s.Res, where)
		}
	case OLoop:
		body := s.Sub[0]
		if len(body.Params) != len(s.Args) {
			v.add("W4: a loop with %d parameters and %d initial values", len(body.Params), len(s.Args))
		}
		for j, a := range s.Args {
			if j < len(body.Params) {
				v.flow(a, v.ty(body.Params[j]), "a loop parameter")
			}
		}
		for _, exit := range v.region(body, ctx{kind: kLoop}) {
			vals := exit[1:]
			if Term(exit[0]) == TContinue {
				if len(vals) != len(body.Params) {
					v.add("W4: continue passes %d values to %d loop parameters", len(vals), len(body.Params))
					continue
				}
				for j, a := range vals {
					v.flow(a, v.ty(body.Params[j]), "a loop parameter")
				}
			} else {
				v.results([][]V{vals}, s.Res, "a loop's break")
			}
		}
	}
	for _, x := range s.Res {
		v.enter(x)
	}
}

// results checks exits against an owner's results (W4, W5).
func (v *verifier) results(exits [][]V, res []V, where string) {
	for _, ys := range exits {
		if len(ys) != len(res) {
			v.add("W4: %s's region leaves with %d values and it has %d results", where, len(ys), len(res))
			continue
		}
		for j, y := range ys {
			v.flow(y, v.ty(res[j]), where+"'s result")
		}
	}
}

// ---------------------------------------------------------------- W7

// linear checks W7 on every path: a buffer is consumed at most once, and read
// only before it is consumed. A path through a loop body that ends in
// `continue` runs again, so on it no buffer defined outside the loop may be
// consumed; a path that ends in `break` runs once, and may.
type linear struct {
	v     *verifier
	entry []map[V]bool // per enclosing loop: what was consumed before its body began
}

func (l *linear) root(x V) V {
	for x >= 0 && int(x) < len(l.v.alias) && l.v.alias[x] != x {
		x = l.v.alias[x]
	}
	return x
}

func (l *linear) isBuf(x V) bool {
	r := l.root(x)
	return r >= 0 && int(r) < len(l.v.buf) && l.v.buf[r]
}

// region walks r with the consumed set of its path; loop is the depth of the
// innermost loop body r is in. It returns the set after r (a union over r's
// own branch arms, for an owner that joins).
func (l *linear) region(r *Region, consumed map[V]bool, loop int) map[V]bool {
	cons := func(x V, what string) {
		if !l.isBuf(x) {
			return
		}
		rt := l.root(x)
		if consumed[rt] {
			l.v.add("W7: buffer %%%d is consumed twice on one path (%s)", rt, what)
		}
		consumed[rt] = true
	}
	read := func(x V, what string) {
		if l.isBuf(x) && consumed[l.root(x)] {
			l.v.add("W7: buffer %%%d is used after it was consumed (%s)", l.root(x), what)
		}
	}
	for i := range r.Stmts {
		s := &r.Stmts[i]
		where := "(" + s.Op.String() + " …)"
		// A LENGTH IS NOT A USE: `set` passes its buffer's length through, so
		// len ∘ set = len ∘ π₁ and the length of a consumed buffer is its
		// successor's. Linearity orders what a store can change, and a length
		// sees no store (linearity.go, the Windows map library).
		if s.Op != OLen {
			for _, a := range s.Args {
				read(a, where)
			}
		}
		switch s.Op {
		case OSet, OInsert:
			if len(s.Args) > 0 {
				if !l.isBuf(s.Args[0]) {
					l.v.add("W7: %s stores into %%%d, which is not a live buffer (a frozen table, ADR 0031)", where, s.Args[0])
				}
				cons(s.Args[0], where)
			}
		case OCall:
			if l.v.tg.Prims[s.Name].Kind == "stmt" && len(s.Args) > 0 {
				cons(s.Args[0], s.Name)
			}
		case OLoop:
			for _, a := range s.Args {
				cons(a, "a loop's initial value")
			}
			after := copySet(consumed)
			l.entry = append(l.entry, copySet(consumed))
			l.loopBody(s.Sub[0], consumed, after, loop+1)
			l.entry = l.entry[:len(l.entry)-1]
			consumed = after
		case OIf:
			a := l.region(s.Sub[0], copySet(consumed), loop)
			b := l.region(s.Sub[1], copySet(consumed), loop)
			consumed = union(a, b)
		case OBuild, OBuildMap, OTabulate:
			consumed = l.region(s.Sub[0], copySet(consumed), loop)
		}
	}
	switch r.T {
	case TYield, TBreak:
		for _, a := range r.Args {
			read(a, r.T.String())
			cons(a, r.T.String())
		}
	case TContinue:
		for _, a := range r.Args {
			read(a, "continue")
			cons(a, "continue")
		}
		// This path runs again: nothing from outside the loop may be consumed on
		// it. What was consumed before the loop began is not this path's.
		entry := map[V]bool{}
		if len(l.entry) > 0 {
			entry = l.entry[len(l.entry)-1]
		}
		for x := range consumed {
			if !entry[x] && int(x) < len(l.v.depth) && l.v.depth[x] < loop && l.v.defined[x] {
				l.v.add("W7: buffer %%%d, defined outside a loop, is consumed on a path that continues it, so on every iteration", x)
			}
		}
	case TBranch:
		read(r.Cond, "branch")
		a := l.region(r.Then, copySet(consumed), loop)
		b := l.region(r.Else, copySet(consumed), loop)
		return union(a, b)
	}
	return consumed
}

// loopBody walks a loop body; after collects what its breaking paths consumed.
func (l *linear) loopBody(r *Region, consumed, after map[V]bool, loop int) {
	out := l.region(r, copySet(consumed), loop)
	for x := range out {
		after[x] = true
	}
}

func copySet(m map[V]bool) map[V]bool {
	out := make(map[V]bool, len(m))
	for k := range m {
		out[k] = true
	}
	return out
}

func union(a, b map[V]bool) map[V]bool {
	for k := range b {
		a[k] = true
	}
	return a
}

// isWordRep reports a word-width integer realization: the signed word or U.
func isWordRep(sort string) bool { return sort == "int" || sort == "u64" }
