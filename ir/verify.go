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
}

func (v *verifier) run() {
	nv := v.f.NV()
	v.defined = make([]bool, nv)
	v.scope = make([]bool, nv)
	v.depth = make([]int, nv)
	v.alias = make([]V, nv)
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
			v.flow(y, v.f.Results[j], "the function's result")
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
				case s.Op.IsArith() && s.Mode != MExact && s.Mode != MTrap:
					v.add("W9: (%s …) with no mode in IR_P", s.Op)
				}
			}
		})
		for x := 0; x < nv; x++ {
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

func (v *verifier) agrees(got, want string) bool {
	if v.p.Stage == StageP {
		if subtype(v.tg, got, want) {
			return true
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
		v.enter(pi.V)
	}
	for i := range r.Stmts {
		v.stmt(&r.Stmts[i])
	}
	return v.terminator(r, c)
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
		return append(v.region(r.Then, c), v.region(r.Else, c)...)
	}
	return nil
}

// sigma is Σ's arity table: operands and results, -1 for "any number".
var sigma = map[Op][2]int{
	OConst: {0, 1}, OGlobal: {0, 1},
	OAdd: {2, 1}, OSub: {2, 1}, OMul: {2, 1}, ONeg: {1, 1}, ODiv: {2, 1}, ORem: {2, 1},
	OEq: {2, 1}, ONe: {2, 1}, OLt: {2, 1}, OLe: {2, 1}, OGt: {2, 1}, OGe: {2, 1},
	OIndex: {2, 1}, OLen: {1, 1}, OArray: {-1, 1}, OMap: {-1, 1}, ORead: {2, 2},
	OKeys: {1, 1}, OSet: {3, 1}, OInsert: {3, 1},
	OIf: {1, -1}, OLoop: {-1, -1}, OBuild: {1, -1}, OBuildMap: {1, -1}, OTabulate: {1, 1},
	OThe: {1, 1}, ORequire: {1, 0},
}

var subs = map[Op]int{OIf: 2, OLoop: 1, OBuild: 1, OBuildMap: 1, OTabulate: 1}

func (v *verifier) stmt(s *Stmt) {
	where := "(" + s.Op.String() + " …)"
	for _, a := range s.Args {
		v.use(a, where)
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
					v.flow(a, p.Args[j], fmt.Sprintf("%s's argument %d", s.Name, j+1))
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
	switch {
	case s.Op.IsArith():
		for _, a := range s.Args {
			v.flow(a, "int", where)
		}
	case s.Op.IsCmp():
		for _, a := range s.Args {
			v.flow(a, "int", where)
		}
	case s.Op == OIf || s.Op == ORequire:
		v.flow(s.Args[0], "bool", where)
	case s.Op == OIndex, s.Op == OSet:
		if len(s.Args) >= 2 {
			v.flow(s.Args[1], "int", where+"'s index")
		}
	}
	// The regions, and W3/W4 at their exits.
	switch s.Op {
	case OIf:
		for _, sub := range s.Sub {
			v.results(v.region(sub, ctx{kind: kArm}), s.Res, where)
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
