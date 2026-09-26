// Package plan is what every printer of IR_P shares (docs/spec/ir.md §9).
//
// A printer is a model of Σ in a host's source language. Its work divides in
// two, and only one half depends on the host:
//
//   - DECISIONS, which are facts about the IR justified by a law of spec §1.4:
//     which values are aliases (L9), which operations are live (L7), which loop
//     result IS a parameter (soleExit), which parameter moves to a post clause
//     (L4), which back-edge build reuses a spare (Rule R), which `if` is a
//     connective (L10), and how a parallel move is sequentialised. They are
//     here, once;
//   - SPELLING, which is the host's: declarations, types, the forms of its
//     operators and calls. Each printer supplies it.
//
// The four term backends wrote the decisions four times (21 methods existed
// three or four times, research §0). This package is the one copy.
package plan

import (
	"sort"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// Plan is the host-independent analysis of one function a printer reads.
type Plan struct {
	Tg    *emit.Target
	F     *ir.Func
	Alias []ir.V // a π is its source; a store's or statement's value is its buffer
	Uses  []int  // reads of each value (after aliasing), the least fixpoint of liveness
	Const map[ir.V]*core.Term
	Spell map[ir.Op][2]string // an integer operation's form: exact, and trap
}

// NewPlan computes the aliases, the constants, the spellings and liveness.
func New(tg *emit.Target, f *ir.Func) *Plan {
	nv := f.NV()
	p := &Plan{Tg: tg, F: f, Alias: make([]ir.V, nv), Const: map[ir.V]*core.Term{}}
	for i := range p.Alias {
		p.Alias[i] = -1
	}
	f.Walk(func(r *ir.Region) {
		for _, pi := range r.Pis {
			p.Alias[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch s.Op {
			case ir.OConst:
				p.Const[s.Res[0]] = s.Lit
			case ir.OSet, ir.OInsert:
				p.Alias[s.Res[0]] = s.Args[0]
			case ir.OCall:
				if tg.Prims[s.Name].Kind == "stmt" && len(s.Args) > 0 {
					p.Alias[s.Res[0]] = s.Args[0]
				}
			}
		}
	})
	p.Spell = Spellings(tg)
	p.CountUses()
	return p
}

// Res is a value's representative.
func (p *Plan) Res(v ir.V) ir.V {
	for v >= 0 && int(v) < len(p.Alias) && p.Alias[v] >= 0 && p.Alias[v] != v {
		v = p.Alias[v]
	}
	return v
}

// Read reports whether anything reads v.
func (p *Plan) Read(v ir.V) bool { return p.Uses[p.Res(v)] > 0 }

// ---------------------------------------------------------------- spellings

// Spellings is the target's form for each integer operation: a primitive the
// target classifies as that operation on integers, and its checked form. A
// primitive declared on integers is preferred to one overloaded over `any`
// (Go's `!=`); among equals the choice is by name, so a printer is a function
// of its input.
func Spellings(tg *emit.Target) map[ir.Op][2]string {
	out := map[ir.Op][2]string{}
	names := make([]string, 0, len(tg.Prims))
	for n := range tg.Prims {
		names = append(names, n)
	}
	sort.Strings(names)
	arith := map[string]ir.Op{"add": ir.OAdd, "sub": ir.OSub, "mul": ir.OMul, "neg": ir.ONeg, "div": ir.ODiv, "rem": ir.ORem}
	cmp := map[string]ir.Op{"eq": ir.OEq, "ne": ir.ONe, "lt": ir.OLt, "le": ir.OLe, "gt": ir.OGt, "ge": ir.OGe}
	// Three passes, each filling only what the earlier left: integer arguments;
	// then arguments overloaded over `any` with a declared result; then an
	// undeclared result too (JavaScript declares no types at all).
	for pass := 0; pass < 3; pass++ {
		for _, n := range names {
			q := tg.Prims[n]
			if q.Form == "" || q.Kind != "expr" || emit.IsCheckedName(n) {
				continue
			}
			if pass == 0 && !intArgs(tg, q) || pass > 0 && !openArgs(tg, q) {
				continue
			}
			var o ir.Op
			var ok bool
			if a := emit.ArithOp(n, len(q.Args)); a != "" && (tg.ValueType(q.Result) == "int" || pass > 0 && open(q.Result)) {
				o, ok = arith[a]
				if ok && (o == ir.ODiv || o == ir.ORem) && !ir.IntegerDivision(tg, n, q) {
					ok = false // JavaScript's `/` is division in the reals
				}
			} else if c := emit.CmpOp(n); c != "" && len(q.Args) == 2 && (q.Result == "bool" || pass == 2 && open(q.Result)) {
				o, ok = cmp[c]
			}
			if !ok {
				continue
			}
			if _, have := out[o]; have && pass == 2 {
				continue
			}
			cur, have := out[o]
			if have && (cur[1] != "" || q.Checked == "") {
				continue
			}
			trap := ""
			if c, ok := tg.Prims[q.Checked]; ok {
				trap = c.Form
			}
			out[o] = [2]string{q.Form, trap}
		}
	}
	return out
}

func open(ty string) bool { return ty == "" || ty == "any" }

func intArgs(tg *emit.Target, q emit.Prim) bool {
	if len(q.Args) == 0 {
		return false
	}
	for _, a := range q.Args {
		if tg.ValueType(a) != "int" {
			return false
		}
	}
	return true
}

func openArgs(tg *emit.Target, q emit.Prim) bool {
	if len(q.Args) == 0 {
		return false
	}
	for _, a := range q.Args {
		if tg.ValueType(a) != "int" && !open(a) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- liveness (L7)

// CountUses is the least fixpoint of liveness: an operation's operands are
// read only if it is live, and a yield's or break's operand j only if its
// owner's result j is read.
func (p *Plan) CountUses() {
	nv := p.F.NV()
	prev := make([]int, nv)
	for {
		p.Uses = make([]int, nv)
		use := func(vs []ir.V) {
			for _, v := range vs {
				if v >= 0 {
					p.Uses[p.Res(v)]++
				}
			}
		}
		gate := func(vs, res []ir.V) {
			for j, v := range vs {
				if v >= 0 && (res == nil || j < len(res) && prev[p.Res(res[j])] > 0) {
					p.Uses[p.Res(v)]++
				}
			}
		}
		var region func(r *ir.Region, yieldRes, breakRes []ir.V)
		region = func(r *ir.Region, yieldRes, breakRes []ir.V) {
			for i := range r.Stmts {
				s := &r.Stmts[i]
				if !p.liveIn(s, prev) {
					continue
				}
				use(s.Args)
				for _, sub := range s.Sub {
					switch s.Op {
					case ir.OLoop:
						region(sub, nil, s.Res)
					case ir.OTabulate:
						region(sub, nil, breakRes) // its yield is stored, always read
					default:
						region(sub, s.Res, breakRes)
					}
				}
			}
			switch r.T {
			case ir.TYield:
				gate(r.Args, yieldRes)
			case ir.TBreak:
				gate(r.Args, breakRes)
			case ir.TContinue:
				use(r.Args)
			case ir.TBranch:
				use([]ir.V{r.Cond})
				region(r.Then, yieldRes, breakRes)
				region(r.Else, yieldRes, breakRes)
			}
		}
		region(p.F.Body, nil, nil)
		same := true
		for i := range prev {
			if (prev[i] > 0) != (p.Uses[i] > 0) {
				same = false
			}
		}
		if same {
			return
		}
		prev = p.Uses
	}
}

// Live reports whether a statement is printed.
func (p *Plan) Live(s *ir.Stmt) bool { return p.liveIn(s, p.Uses) }

func (p *Plan) liveIn(s *ir.Stmt, uses []int) bool {
	switch s.Op {
	case ir.OIf:
		// Total arms make an unread `if` droppable (L7); a LOOP is not total:
		// dropping one that may not terminate would turn ⊥ into a value.
		if !p.PureRegion(s.Sub[0]) || !p.PureRegion(s.Sub[1]) {
			return true
		}
	case ir.OLoop, ir.OBuild, ir.OBuildMap, ir.OTabulate, ir.OSet, ir.OInsert:
		return true
	case ir.OCall:
		q := p.Tg.Prims[s.Name]
		if !q.Pure || q.Kind == "stmt" {
			return true
		}
	case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg:
		if s.Mode == ir.MTrap {
			return true // a trap is an effect
		}
	}
	for _, r := range s.Res {
		if uses[p.Res(r)] > 0 {
			return true
		}
	}
	return false
}

// PureRegion: every operation in r is pure and total.
func (p *Plan) PureRegion(r *ir.Region) bool {
	for i := range r.Stmts {
		s := &r.Stmts[i]
		switch s.Op {
		case ir.OLoop, ir.OBuild, ir.OBuildMap, ir.OTabulate, ir.OSet, ir.OInsert:
			return false
		case ir.OCall:
			if q := p.Tg.Prims[s.Name]; !q.Pure || q.Kind == "stmt" {
				return false
			}
		case ir.OAdd, ir.OSub, ir.OMul, ir.ONeg:
			if s.Mode == ir.MTrap {
				return false
			}
		case ir.ODiv, ir.ORem:
			return false // a zero divisor stops the program: not total
		}
		for _, sub := range s.Sub {
			if !p.PureRegion(sub) {
				return false
			}
		}
	}
	if r.T == ir.TBranch {
		return p.PureRegion(r.Then) && p.PureRegion(r.Else)
	}
	return true
}

// ---------------------------------------------------------------- regions

// Exits are the argument lists of every terminator of kind k in r's tree.
func Exits(r *ir.Region, k ir.Term) [][]ir.V {
	var out [][]ir.V
	var walk func(r *ir.Region)
	walk = func(r *ir.Region) {
		if r.T == k {
			out = append(out, r.Args)
		}
		if r.T == ir.TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
	return out
}

// FirstYield is a region's yield values, at its first yield.
func FirstYield(r *ir.Region) []ir.V {
	switch r.T {
	case ir.TYield:
		return r.Args
	case ir.TBranch:
		if y := FirstYield(r.Then); y != nil {
			return y
		}
		return FirstYield(r.Else)
	}
	return nil
}

// Terminates: control never falls out of r; every leaf jumps, or returns
// when r is in the function's tail.
func Terminates(r *ir.Region, top bool) bool {
	switch r.T {
	case ir.TBreak, ir.TContinue:
		return true
	case ir.TYield:
		return top
	case ir.TBranch:
		return Terminates(r.Then, top) && Terminates(r.Else, top)
	}
	return false
}

// YieldsOnly: every yield of r is exactly v (a build that IS its buffer).
func (p *Plan) YieldsOnly(r *ir.Region, v ir.V) bool {
	switch r.T {
	case ir.TYield:
		return len(r.Args) == 1 && p.Res(r.Args[0]) == p.Res(v)
	case ir.TBranch:
		return p.YieldsOnly(r.Then, v) && p.YieldsOnly(r.Else, v)
	}
	return false
}

// ---------------------------------------------------------------- loops

// Loop is what a printer needs to decide about one loop.
type Loop struct {
	// Coalesced[j] is the parameter result j IS (soleExit), or -1.
	Coalesced []int
	// Post[j] is the literal step parameter j advances by at every continue,
	// moved to a post clause (L4), or nil.
	Post map[int]*core.Term
	// Spare[j] is the build parameter j alternates with (Rule R), and its
	// literal length.
	Spare map[int]SpareBuild
}

// SpareBuild is a back-edge build that writes into a spare.
type SpareBuild struct {
	Build *ir.Stmt
	Len   int64
}

// DecideLoop computes soleExit, PostVars and buffer reuse for a loop. PostVars
// updates the plan's uses: the post clause computes the stepped value, so its
// defining statement is no longer read.
func (p *Plan) DecideLoop(s *ir.Stmt) Loop {
	out := p.DecideExits(s)
	p.decidePost(s, &out)
	return out
}

// DecideExits is DecideLoop without PostVars, for a host with no post clause
// (x86): soleExit and buffer reuse, which leave the plan's uses unchanged.
func (p *Plan) DecideExits(s *ir.Stmt) Loop {
	body := s.Sub[0]
	out := Loop{Coalesced: make([]int, len(s.Res)), Post: map[int]*core.Term{}, Spare: map[int]SpareBuild{}}
	// soleExit: result j is parameter k at every break.
	breaks := Exits(body, ir.TBreak)
	for j := range s.Res {
		k := -1
		for _, b := range breaks {
			idx := -1
			if j < len(b) {
				for pi, q := range body.Params {
					if p.Res(b[j]) == p.Res(q) {
						idx = pi
					}
				}
			}
			if idx < 0 || (k >= 0 && idx != k) {
				k = -2
				break
			}
			k = idx
		}
		if k < 0 || len(breaks) == 0 {
			k = -1
		}
		out.Coalesced[j] = k
	}
	p.decideReuse(s, &out)
	return out
}

// decidePost is PostVars: parameter j advanced by one literal step at every
// continue, by a value nothing else reads.
func (p *Plan) decidePost(s *ir.Stmt, out *Loop) {
	body := s.Sub[0]
	conts := Exits(body, ir.TContinue)
	defs := map[ir.V]*ir.Stmt{}
	var collect func(r *ir.Region)
	collect = func(r *ir.Region) {
		for i := range r.Stmts {
			if len(r.Stmts[i].Res) > 0 {
				defs[r.Stmts[i].Res[0]] = &r.Stmts[i]
			}
		}
		if r.T == ir.TBranch {
			collect(r.Then)
			collect(r.Else)
		}
	}
	collect(body)
	for j, q := range body.Params {
		var step *core.Term
		ok := len(conts) > 0
		for _, c := range conts {
			d := defs[c[j]]
			if d == nil || d.Op != ir.OAdd || d.Mode == ir.MTrap || p.Res(d.Args[0]) != p.Res(q) ||
				p.Const[d.Args[1]] == nil || p.Uses[p.Res(c[j])] != 1 {
				ok = false
				break
			}
			st := p.Const[d.Args[1]]
			if step != nil && (st.Kind != step.Kind || st.Int != step.Int) {
				ok = false
				break
			}
			step = st
		}
		if ok {
			out.Post[j] = step
			for _, c := range conts {
				p.Uses[p.Res(c[j])] = 0 // the post clause computes it
			}
		}
	}
}

// decideReuse is Rule R (emit/target.go's LoopBufferReuse, 2.5–2.7× measured):
// parameter j alternates with a spare when (1) the continue's argument j is a
// build of literal length n, (2) j's initial value is a build of the same
// length, (3) no other continue argument carries the old buffer, and (4) the
// loop has exactly one continue.
func (p *Plan) decideReuse(s *ir.Stmt, out *Loop) {
	body := s.Sub[0]
	conts := Exits(body, ir.TContinue)
	if len(conts) != 1 {
		return // (4)
	}
	builds := map[ir.V]*ir.Stmt{}
	p.F.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			if b := &r.Stmts[i]; b.Op == ir.OBuild && len(b.Res) == 1 {
				builds[b.Res[0]] = b
			}
		}
	})
	constLen := func(b *ir.Stmt) (int64, bool) {
		if d := p.Const[p.Res(b.Args[0])]; d != nil && d.Kind == core.KInt {
			return d.Int, true
		}
		return 0, false
	}
	for j, q := range body.Params {
		b, ok := builds[p.Res(conts[0][j])]
		if !ok {
			continue
		}
		n, ok := constLen(b)
		if !ok {
			continue // (1)
		}
		ib, ok := builds[p.Res(s.Args[j])]
		if !ok {
			continue
		}
		if m, ok := constLen(ib); !ok || m != n {
			continue // (2)
		}
		carried := false
		for k, a := range conts[0] {
			if k != j && p.Res(a) == p.Res(q) {
				carried = true
			}
		}
		if !carried { // (3)
			out.Spare[j] = SpareBuild{Build: b, Len: n}
		}
	}
}

// ---------------------------------------------------------------- expressions (L10)

// Speller is a host's spelling of the operations an expression tree may hold.
type Speller interface {
	// Op spells an integer operation or comparison, "" if the host has none.
	Op(s *ir.Stmt, args []string) string
	// Index spells a table read of tab at i.
	Index(tab ir.V, t, i string) string
	// Len spells a length.
	Len(v ir.V, t string) string
	// Call spells a pure call with one result, "" if it cannot be an
	// expression.
	Call(s *ir.Stmt, args []string) string
	// Or and And spell the connectives.
	Or(a, b string) string
	And(a, b string) string
	// Cond spells a conditional EXPRESSION, the coproduct as a value, or ""
	// when the host has none (Go).
	Cond(c, a, b string) string
}

// Connective prints a boolean `if` with a literal arm as the operator it is
// (L10): (if c true E) is c ∨ E, and (if c E false) is c ∧ E, when E is an
// expression tree (pure, every value read once, no loop, no store). L6 lets a
// pure E run under the short circuit.
func (p *Plan) Connective(s *ir.Stmt, ref func(ir.V) string, sp Speller) (string, bool) {
	if len(s.Res) != 1 || p.F.Types[s.Res[0]] != "bool" {
		return "", false
	}
	return p.branchExpr(ref(s.Args[0]), s.Sub[0], s.Sub[1], ref, sp)
}

func (p *Plan) branchExpr(c string, th, el *ir.Region, ref func(ir.V) string, sp Speller) (string, bool) {
	lit := func(r *ir.Region) (bool, bool) {
		if r.T != ir.TYield || len(r.Args) != 1 {
			return false, false
		}
		for i := range r.Stmts {
			if r.Stmts[i].Op != ir.OConst {
				return false, false
			}
		}
		d := p.Const[p.Res(r.Args[0])]
		if d == nil || d.Kind != core.KBool {
			return false, false
		}
		return d.IsTrue(), true
	}
	if b, ok := lit(th); ok && b {
		if e, ok := p.ExprOf(el, ref, sp); ok {
			return sp.Or(c, e), true
		}
	}
	if b, ok := lit(el); ok && !b {
		if e, ok := p.ExprOf(th, ref, sp); ok {
			return sp.And(c, e), true
		}
	}
	return "", false
}

// ExprOf is a region's value as one expression, when the region is an
// expression tree.
func (p *Plan) ExprOf(r *ir.Region, outer func(ir.V) string, sp Speller) (string, bool) {
	if !(r.T == ir.TYield && len(r.Args) == 1) && r.T != ir.TBranch {
		return "", false
	}
	exprs := map[ir.V]string{}
	arg := func(v ir.V) string {
		if e, ok := exprs[p.Res(v)]; ok {
			return e
		}
		return outer(v)
	}
	args := func(vs []ir.V) []string {
		out := make([]string, len(vs))
		for k, a := range vs {
			out[k] = arg(a)
		}
		return out
	}
	for i := range r.Stmts {
		s := &r.Stmts[i]
		once := len(s.Res) == 1 && p.Uses[p.Res(s.Res[0])] == 1
		var e string
		switch {
		case s.Op == ir.OConst:
			continue
		case !once:
			return "", false
		case s.Op.IsCmp() || (s.Op.IsArith() && s.Mode != ir.MTrap):
			e = sp.Op(s, args(s.Args))
		case s.Op == ir.OIndex:
			e = sp.Index(s.Args[0], arg(s.Args[0]), arg(s.Args[1]))
		case s.Op == ir.OLen:
			e = sp.Len(s.Args[0], arg(s.Args[0]))
		case s.Op == ir.OCall:
			q := p.Tg.Prims[s.Name]
			if !q.Pure || q.Kind != "expr" || len(q.Results) >= 2 {
				return "", false
			}
			e = sp.Call(s, args(s.Args))
		case s.Op == ir.OIf:
			var ok bool
			if e, ok = p.Connective(s, arg, sp); !ok {
				// Not a connective: a conditional expression, where the host
				// has one and both arms are expression trees.
				a, okA := p.ExprOf(s.Sub[0], arg, sp)
				b, okB := p.ExprOf(s.Sub[1], arg, sp)
				if !okA || !okB {
					return "", false
				}
				if e = sp.Cond(arg(s.Args[0]), a, b); e == "" {
					return "", false
				}
			}
		default:
			return "", false
		}
		if e == "" {
			return "", false
		}
		exprs[s.Res[0]] = e
	}
	if r.T == ir.TBranch {
		if e, ok := p.branchExpr(arg(r.Cond), r.Then, r.Else, arg, sp); ok {
			return e, true
		}
		// A branch terminator whose arms are expression trees: a conditional
		// expression, where the host has one.
		a, okA := p.ExprOf(r.Then, arg, sp)
		b, okB := p.ExprOf(r.Else, arg, sp)
		if !okA || !okB {
			return "", false
		}
		if e := sp.Cond(arg(r.Cond), a, b); e != "" {
			return e, true
		}
		return "", false
	}
	return arg(r.Args[0]), true
}

// ---------------------------------------------------------------- the parallel move (spec §9.3)

// Moves sequentialises a parallel move dst[i] ← src[i] into ordinary
// assignments, with one temporary per cycle: Rideau, Serpette and Leroy,
// "Tilting at windmills with Coq: formal verification of a compilation
// algorithm for parallel moves" (JAR 41, 2008), the algorithm CompCert uses.
//
// A source is an EXPRESSION, not only a name (a printer may inline a value at
// its use, β for let), so the dependency is on its READ SET: the identifiers
// it mentions. A move whose destination no remaining source reads is emitted;
// when every remaining destination is read by another move, the moves form
// cycles, and one destination is saved into a temporary and renamed, as a
// whole identifier, in the remaining sources. Self-moves are dropped.
// Destinations must be distinct.
func Moves(dst, src []string, fresh func() string) [][2]string {
	type mv struct{ d, s string }
	var pend []mv
	for i := range dst {
		if dst[i] != "" && dst[i] != src[i] {
			pend = append(pend, mv{dst[i], src[i]})
		}
	}
	var out [][2]string
	read := func(x string, skip int) bool {
		for k, m := range pend {
			if k != skip && mentions(m.s, x) {
				return true
			}
		}
		return false
	}
	for len(pend) > 0 {
		progress := false
		for i := 0; i < len(pend); i++ {
			if !read(pend[i].d, i) {
				out = append(out, [2]string{pend[i].d, pend[i].s})
				pend = append(pend[:i], pend[i+1:]...)
				progress = true
				break
			}
		}
		if progress {
			continue
		}
		// Every destination is read by another move: a cycle. Save one.
		d := pend[0].d
		t := fresh()
		out = append(out, [2]string{t, d})
		for k := range pend {
			pend[k].s = rename(pend[k].s, d, t)
		}
	}
	return out
}

func identChar(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// mentions reports whether the expression s contains x as a whole identifier.
func mentions(s, x string) bool {
	for i := 0; i+len(x) <= len(s); i++ {
		if s[i:i+len(x)] == x && (i == 0 || !identChar(s[i-1])) && (i+len(x) == len(s) || !identChar(s[i+len(x)])) {
			return true
		}
	}
	return false
}

// rename replaces x by y wherever it occurs in s as a whole identifier.
func rename(s, x, y string) string {
	var b []byte
	for i := 0; i < len(s); {
		if i+len(x) <= len(s) && s[i:i+len(x)] == x && (i == 0 || !identChar(s[i-1])) && (i+len(x) == len(s) || !identChar(s[i+len(x)])) {
			b = append(b, y...)
			i += len(x)
			continue
		}
		b = append(b, s[i])
		i++
	}
	return string(b)
}

// ---------------------------------------------------------------- inlining (β for let)

// Inlinable is the set of values a printer may write at their use instead of
// binding them. Substituting a pure, total value at its unique use is β for
// `let` (L1), and moving a central operation across others is L6, provided the
// move crosses no store the value could observe and does not carry the
// computation into a loop, which would repeat it. So v is inlinable when:
//   - it is an integer operation not in mode trap, a comparison, a length, a
//     read of a table that is not a buffer, or a pure host call with one result;
//   - it is read exactly once;
//   - that read is in the region that defines it (a statement's operand, a
//     terminator's argument, a branch's condition), not in a nested region.
//
// A read of a BUFFER is inlined only when no effect lies between it and its
// use: a store there would change what it reads (W7's order, carried by
// position here).
func (p *Plan) Inlinable() map[ir.V]bool {
	out := map[ir.V]bool{}
	isBuf := func(v ir.V) bool {
		t := p.F.Types[p.Res(v)]
		return len(t) >= 7 && t[:7] == "buffer "
	}
	p.F.Walk(func(r *ir.Region) {
		local := map[ir.V]int{} // reads of each value within r itself
		for i := range r.Stmts {
			for _, a := range r.Stmts[i].Args {
				local[p.Res(a)]++
			}
		}
		for _, a := range r.Args {
			local[p.Res(a)]++
		}
		if r.T == ir.TBranch {
			local[p.Res(r.Cond)]++
		}
		// useAt[v] is the index of the statement in r that reads v, or
		// len(r.Stmts) for the terminator.
		useAt := map[ir.V]int{}
		for i := range r.Stmts {
			for _, a := range r.Stmts[i].Args {
				useAt[p.Res(a)] = i
			}
		}
		for _, a := range r.Args {
			useAt[p.Res(a)] = len(r.Stmts)
		}
		if r.T == ir.TBranch {
			useAt[p.Res(r.Cond)] = len(r.Stmts)
		}
		effect := func(s *ir.Stmt) bool {
			switch s.Op {
			case ir.OSet, ir.OInsert, ir.OLoop, ir.OBuild, ir.OBuildMap, ir.OTabulate:
				return true
			case ir.OCall:
				q := p.Tg.Prims[s.Name]
				return !q.Pure || q.Kind == "stmt"
			case ir.OIf:
				return !p.PureRegion(s.Sub[0]) || !p.PureRegion(s.Sub[1])
			}
			return false
		}
		// quiet: no statement strictly between i and j has an effect, so no
		// store can come between a read defined at i and its use at j.
		quiet := func(i, j int) bool {
			for k := i + 1; k < j && k < len(r.Stmts); k++ {
				if effect(&r.Stmts[k]) {
					return false
				}
			}
			return true
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if len(s.Res) != 1 || p.Res(s.Res[0]) != s.Res[0] {
				continue
			}
			v := s.Res[0]
			if p.Uses[v] != 1 || local[v] != 1 {
				continue
			}
			ok := false
			switch {
			case s.Op.IsCmp(), s.Op.IsArith() && s.Mode != ir.MTrap, s.Op == ir.OLen:
				ok = true
			case s.Op == ir.OIndex:
				// A table's read moves freely; a BUFFER's only where no effect
				// lies between the read and its use, so no store can intervene.
				ok = !isBuf(s.Args[0]) || quiet(i, useAt[v])
			case s.Op == ir.OCall:
				q := p.Tg.Prims[s.Name]
				ok = q.Pure && q.Kind == "expr" && len(q.Results) < 2
			case s.Op == ir.OIf:
				// A pure, total `if` is central too; it is inlined only if the
				// printer spells it as an expression (a connective or a
				// conditional), which is where its definition calls define.
				ok = p.PureRegion(s.Sub[0]) && p.PureRegion(s.Sub[1])
			}
			if ok {
				out[v] = true
			}
		}
	})
	return out
}
