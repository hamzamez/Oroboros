package ir

import (
	"fmt"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// A TABLE IS RELEASED AT ITS DEATH (ADR 0096, ir.md W11, llvm.md §4).
//
// A frozen table's view is counted fractions, not persistent: freezing splits
// the full permission into one fraction per reference, each reference's death
// returns its fraction, and at the last death `free`'s precondition holds
// (Boyland 2003; Bornat et al. 2005). Where the references are SSA names the
// count is static, and the last death is the point after the last use
// (integers-research Proposition 3). A table is OWNED when its storage has one
// reference at a time:
//   - it is a BIRTH, a build, tabulate or array literal with its storage, or a
//     loop parameter or a join's result whose every incoming value is owned and
//     MOVED in (a loop's init, a continue, a break, a yield, a `the`);
//   - it is moved at most once, nothing uses it after the move, and no loop
//     lies between its definition and the move;
//   - everything else it meets BORROWS it: len, an index, a call that returns
//     no table.
// An owned table that is not moved on a path is released there, after its
// last use. A table that is not owned is not released: a leak, never a use
// after free.

// The planted faults, each of which W11 must refuse.
var (
	// PlantReleaseFirstUse releases after the first use, not the last.
	PlantReleaseFirstUse bool
	// PlantReleaseMoved releases a table on a path where it was moved.
	PlantReleaseMoved bool
	// PlantReleaseInLoop admits a move inside a loop the value is defined
	// outside, so one storage is moved, and released, once per iteration.
	PlantReleaseInLoop bool
	// PlantCallArgsEscape reads a program function's call as a host's, its
	// arguments escaping and its result owned by nobody: every result leaks.
	PlantCallArgsEscape bool
	// PlantReleaseFrozenBuild releases every build's storage after the build,
	// its buffer frozen into a result or not: freed, then read.
	PlantReleaseFrozenBuild bool
)

// A BUILD'S BUFFER NOT FROZEN OUT DIES AT THE BUILD'S END (gauntlet-2026-11-16).
// A build's region holds its buffer's full view, and the region's yield is the
// build's value (ADR 0031): a product of frozen buffers and values. A buffer the
// yield freezes passes its view on, as a table, a birth the rules above follow.
// One the yield does not freeze returns the view at the region's end, and
// nothing outside the region names it, so its storage is dead right after the
// build: release(p, n, w), with p, n, w its allocation's own. Until this rule a
// buffer whose scope's value was an integer (a stack, a worklist) was never
// released: the JSON tree leaked 18 KiB a call on windows.
//
// own(b) is the least set holding the region's buffer parameter and closed
// under each statement's results: a result that is a buffer or a table and
// has an argument in own; by position, a loop's body parameter whose initial
// value or any continuing value is in own and a loop's, an if's or a nested
// build's result whose breaking or yielded value is; a π of a value in own. An
// integer read from the buffer is no member. Taking a member too many only
// keeps a storage alive, so own may over-approximate b's descendants and never
// under-approximate them: the build is dead at its end iff no value its region
// yields to it is in own.
func ownOf(f *Func, b *Stmt) map[V]bool {
	if len(b.Sub) == 0 || len(b.Sub[0].Params) == 0 {
		return nil
	}
	own := map[V]bool{b.Sub[0].Params[0]: true}
	tableOrBuffer := func(v V) bool {
		return v >= 0 && int(v) < len(f.Types) &&
			(strings.HasPrefix(f.Types[v], "array") || strings.HasPrefix(f.Types[v], "buffer"))
	}
	mark := func(v V) bool {
		if own[v] || !tableOrBuffer(v) {
			return false
		}
		own[v] = true
		return true
	}
	var exits func(r *Region, t Term, on func(*Region))
	exits = func(r *Region, t Term, on func(*Region)) {
		if r.T == TBranch {
			exits(r.Then, t, on)
			exits(r.Else, t, on)
		} else if r.T == t {
			on(r)
		}
	}
	// by position: result j is in own when some terminator of kind t yields a
	// member at j
	byPos := func(sub *Region, t Term, res []V) bool {
		changed := false
		exits(sub, t, func(x *Region) {
			for j, a := range x.Args {
				if own[a] && j < len(res) && mark(res[j]) {
					changed = true
				}
			}
		})
		return changed
	}
	for changed := true; changed; {
		changed = false
		var walk func(r *Region)
		walk = func(r *Region) {
			for _, p := range r.Pis {
				if own[p.Of] && mark(p.V) {
					changed = true
				}
			}
			for i := range r.Stmts {
				s := &r.Stmts[i]
				switch {
				case s.Op == OLoop:
					body := s.Sub[0]
					for j, a := range s.Args {
						if own[a] && j < len(body.Params) && mark(body.Params[j]) {
							changed = true
						}
					}
					changed = byPos(body, TContinue, body.Params) || changed
					changed = byPos(body, TBreak, s.Res) || changed
				case s.Op == OIf || s.Op == OBuild || s.Op == OBuildMap:
					for _, sub := range s.Sub {
						changed = byPos(sub, TYield, s.Res) || changed
					}
				case len(s.Sub) > 0:
					// an operation with a region this rule does not name: every
					// result a member when anything it reads or yields is one
					hit := false
					for _, a := range s.Args {
						hit = hit || own[a]
					}
					for _, sub := range s.Sub {
						for _, t := range []Term{TYield, TBreak, TContinue} {
							exits(sub, t, func(x *Region) {
								for _, a := range x.Args {
									hit = hit || own[a]
								}
							})
						}
					}
					for _, v := range s.Res {
						if hit && mark(v) {
							changed = true
						}
					}
				default:
					for _, a := range s.Args {
						if own[a] {
							for _, v := range s.Res {
								if mark(v) {
									changed = true
								}
							}
							break
						}
					}
				}
				for _, sub := range s.Sub {
					walk(sub)
				}
			}
			if r.T == TBranch {
				walk(r.Then)
				walk(r.Else)
			}
		}
		walk(b.Sub[0])
	}
	return own
}

// deadBuild reports a build whose region yields it no member of own: its
// buffer is frozen into none of its results.
func deadBuild(f *Func, b *Stmt) bool {
	if b.Op != OBuild || len(b.Store) != 1 {
		return false
	}
	if PlantReleaseFrozenBuild {
		return true
	}
	own := ownOf(f, b)
	if own == nil {
		return false
	}
	dead := true
	var exits func(r *Region)
	exits = func(r *Region) {
		if r.T == TBranch {
			exits(r.Then)
			exits(r.Else)
			return
		}
		if r.T == TYield {
			for _, a := range r.Args {
				dead = dead && !own[a]
			}
		}
	}
	exits(b.Sub[0])
	return dead
}

// allocOf is the statement in r before index i that allocates p: `alloc` or
// `alloc-raw` of (n, w), which Storage places before the build it serves.
func allocOf(r *Region, i int, p V) *Stmt {
	for k := i - 1; k >= 0; k-- {
		s := &r.Stmts[k]
		if len(s.Res) == 1 && s.Res[0] == p {
			if s.Op == OCall && (s.Name == "alloc" || s.Name == "alloc-raw") && len(s.Args) == 2 {
				return s
			}
			return nil
		}
	}
	return nil
}

// A loc is a point of the function: a statement of a region, or its
// terminator at index len(Stmts).
type loc struct {
	r *Region
	i int
}

// relCtx is the function's region tree and every table's uses.
type relCtx struct {
	tg     *emit.Target
	f      *Func
	parent map[*Region]loc  // the statement or terminator a region is nested in
	loopOf map[*Region]bool // a loop's body
	def    map[V]loc        // a value's definition: a statement, or a region's start (i = −1)
	uses   map[V][]use      // every use of a table
	births map[V]bool       // a build, tabulate or array literal with its storage
	edges  map[V][]V        // a parameter's or result's incoming values
	moves  map[V][]use      // each table's moves
	alias  map[V]V          // a π's name: the reference it renames
}

type use struct {
	at   loc
	kind int // useBorrow, useMove, useEscape
	to   V   // a move's target
}

const (
	useBorrow = iota
	useMove
	useEscape
)

// rootOf is the reference a π's name renames, or v.
func (c *relCtx) rootOf(v V) V {
	for {
		a, ok := c.alias[v]
		if !ok {
			return v
		}
		v = a
	}
}

func (c *relCtx) table(v V) bool {
	return v >= 0 && int(v) < len(c.f.Types) && strings.HasPrefix(c.f.Types[v], "array")
}

// analyse indexes the function: its region tree, its definitions, the table
// values' uses, and the edges a move follows.
func newRelCtx(tg *emit.Target, f *Func) *relCtx {
	c := &relCtx{tg: tg, f: f, parent: map[*Region]loc{}, loopOf: map[*Region]bool{}, def: map[V]loc{},
		uses: map[V][]use{}, births: map[V]bool{}, edges: map[V][]V{}, moves: map[V][]use{}, alias: map[V]V{}}
	// A π IS A RENAMING WITH A FACT: its name is the same reference as its
	// source, so its uses are the source's, at their own points.
	root := func(v V) V {
		for {
			a, ok := c.alias[v]
			if !ok {
				return v
			}
			v = a
		}
	}
	edge := func(to, from V) { c.edges[to] = append(c.edges[to], root(from)) }
	add := func(v V, u use) {
		v = root(v)
		if c.table(v) {
			c.uses[v] = append(c.uses[v], u)
			if u.kind == useMove {
				c.moves[v] = append(c.moves[v], u)
			}
		}
	}
	// exits walks a region's terminator tree, the arms of its branches, and
	// calls on with each terminator of kind t.
	var exits func(r *Region, t Term, on func(r *Region))
	exits = func(r *Region, t Term, on func(r *Region)) {
		switch r.T {
		case TBranch:
			exits(r.Then, t, on)
			exits(r.Else, t, on)
		default:
			if r.T == t {
				on(r)
			}
		}
	}
	var walk func(r *Region)
	walk = func(r *Region) {
		for _, p := range r.Params {
			c.def[p] = loc{r, -1}
		}
		for _, pi := range r.Pis {
			c.def[pi.V] = loc{r, -1}
			if c.table(pi.V) {
				c.alias[pi.V] = root(pi.Of)
			}
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			at := loc{r, i}
			for _, v := range s.Res {
				c.def[v] = at
			}
			for k, sub := range s.Sub {
				c.parent[sub] = at
				if s.Op == OLoop && k == 0 {
					c.loopOf[sub] = true
				}
				// A λ THE HOST APPLIES DURING A CALL (ADR 0143) runs at that call,
				// 0.. times: its region is placed at the call that uses its value,
				// a loop's body for the rule, so a table it reads lives to the
				// call's end. Placed at the `fn` statement, a release fell between
				// the two, and the λ read freed memory.
				if s.Op == OFn && !PlantFnReleasedEarly {
					if j := consumer(r, i, s.Res[0]); j >= 0 {
						c.parent[sub] = loc{r, j}
					}
					c.loopOf[sub] = true
				}
			}
			switch s.Op {
			case OIndex:
				for j, a := range s.Args {
					add(a, use{at: at, kind: map[bool]int{true: useBorrow, false: useEscape}[j == 0]})
				}
			case OLen, OStorage:
				add(s.Args[0], use{at: at, kind: useBorrow})
			case OCall:
				// AN IDENTITY COERCION IS A RENAMING, as a π is
				// (strings-2026-11-16): llvm/str.text's term is its argument, so
				// its String is the table's own storage. A String is no table
				// this pass follows, so the table escapes into it and is never
				// released, where a borrow would free it under the string.
				if identityCoercion(tg.Prims[s.Name]) && !PlantCoercionBorrows {
					for _, a := range s.Args {
						add(a, use{at: at, kind: useEscape})
					}
					continue
				}
				// A PROGRAM FUNCTION'S CALL IS THE FRAME RULE (ADR 0101): its
				// arguments are borrowed and its result is a birth, owned here;
				// W13 checks the callee returns no argument of its own.
				if tg.Prims[s.Name].Program && !PlantCallArgsEscape {
					for _, a := range s.Args {
						add(a, use{at: at, kind: useBorrow})
					}
					for _, r := range s.Res {
						if c.table(r) {
							c.births[r] = true
						}
					}
					continue
				}
				ret := false
				for _, r := range s.Res {
					ret = ret || c.table(r) || strings.HasPrefix(f.Types[r], "buffer")
				}
				for _, a := range s.Args {
					add(a, use{at: at, kind: map[bool]int{true: useEscape, false: useBorrow}[ret]})
				}
			case OThe:
				add(s.Args[0], use{at: at, kind: useMove, to: s.Res[0]})
				edge(s.Res[0], s.Args[0])
			case OLoop:
				body := s.Sub[0]
				for j, a := range s.Args {
					add(a, use{at: at, kind: useMove, to: body.Params[j]})
					edge(body.Params[j], a)
				}
				exits(body, TContinue, func(t *Region) {
					for j, a := range t.Args {
						add(a, use{at: loc{t, len(t.Stmts)}, kind: useMove, to: body.Params[j]})
						edge(body.Params[j], a)
					}
				})
				exits(body, TBreak, func(t *Region) {
					for j, a := range t.Args {
						add(a, use{at: loc{t, len(t.Stmts)}, kind: useMove, to: s.Res[j]})
						edge(s.Res[j], a)
					}
				})
			case OIf:
				for _, sub := range s.Sub {
					exits(sub, TYield, func(t *Region) {
						for j, a := range t.Args {
							add(a, use{at: loc{t, len(t.Stmts)}, kind: useMove, to: s.Res[j]})
							edge(s.Res[j], a)
						}
					})
				}
			default:
				for _, a := range s.Args {
					add(a, use{at: at, kind: useEscape})
				}
			}
			// A BIRTH: the one table a storage-bearing statement makes.
			if len(s.Store) > 0 && (s.Op == OBuild || s.Op == OTabulate || s.Op == OArray) {
				var made []V
				for _, v := range s.Res {
					if c.table(v) {
						made = append(made, v)
					}
				}
				if len(made) == 1 {
					c.births[made[0]] = true
				}
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
		end := loc{r, len(r.Stmts)}
		switch r.T {
		case TBranch:
			c.parent[r.Then] = end
			c.parent[r.Else] = end
			walk(r.Then)
			walk(r.Else)
		case TYield, TBreak, TContinue:
			// Moves were recorded by their owner; a yield no owner records is
			// the function's own, its result, which escapes. So is any value a
			// region yields to an operation that is not an `if` (a build's
			// buffer, a tabulate's element).
			for _, a := range r.Args {
				if !c.recorded(root(a), end) {
					add(a, use{at: end, kind: useEscape})
				}
			}
		}
	}
	walk(f.Body)
	return c
}

// recorded reports a terminator's argument a move already records.
func (c *relCtx) recorded(v V, at loc) bool {
	for _, u := range c.uses[v] {
		if u.kind == useMove && u.at == at {
			return true
		}
	}
	return false
}

// chain is a point's path from the function's body: each region and the
// index in it of the statement or terminator the next region is nested in.
func (c *relCtx) chain(at loc) []loc {
	var out []loc
	for p := at; ; {
		out = append(out, p)
		q, ok := c.parent[p.r]
		if !ok {
			break
		}
		p = q
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// depth is the index of region d in a chain, or −1.
func depth(ch []loc, d *Region) int {
	for k, p := range ch {
		if p.r == d {
			return k
		}
	}
	return -1
}

// loopBetween reports a loop body entered on the way from region d down to at.
func (c *relCtx) loopBetween(d *Region, at loc) bool {
	ch := c.chain(at)
	k := depth(ch, d)
	if k < 0 {
		return true
	}
	for j := k + 1; j < len(ch); j++ {
		if c.loopOf[ch[j].r] {
			return true
		}
	}
	return false
}

// mayFollow reports an execution in which b happens after a, both inside the
// region d a value is defined in: b later in program order, or both inside a
// loop entered below d, whose next iteration follows. Two arms of one choice
// are exclusive.
func (c *relCtx) mayFollow(d *Region, a, b loc) bool {
	if a == b {
		return false
	}
	ca, cb := c.chain(a), c.chain(b)
	k := depth(ca, d)
	if k < 0 || depth(cb, d) != k {
		return true
	}
	for ; k < len(ca) && k < len(cb); k++ {
		pa, pb := ca[k], cb[k]
		if pa.r != pb.r {
			return false // two arms of one choice
		}
		if pa.i != pb.i {
			return pb.i > pa.i
		}
		// one statement: a deeper point is inside it, after its arguments
		if k+1 == len(ca) || k+1 == len(cb) {
			return k+1 == len(ca) && k+1 < len(cb)
		}
		if c.loopOf[ca[k+1].r] && ca[k+1].r == cb[k+1].r {
			return true // both in one loop's body: the next iteration follows
		}
	}
	return false
}

// moveOK reports v's move m is its only one, nothing uses v after it, and no
// loop lies between v's definition and it.
func (c *relCtx) moveOK(v V, m use) bool {
	if len(c.moves[v]) != 1 {
		return false
	}
	d := c.def[v].r
	if !PlantReleaseInLoop && c.loopBetween(d, m.at) {
		return false
	}
	for _, u := range c.uses[v] {
		if u.at != m.at && c.mayFollow(d, m.at, u.at) {
			return false
		}
	}
	return true
}

// ownership is the greatest fixed point of the rules: every table owned to
// begin with, and each that a rule refuses removed until none is.
func (c *relCtx) ownership() map[V]bool {
	owned := map[V]bool{}
	for v := range c.uses {
		owned[v] = true
	}
	for v := range c.births {
		owned[v] = true
	}
	for v := range c.edges {
		owned[v] = true
	}
	for changed := true; changed; {
		changed = false
		for v := range owned {
			ok := c.births[v] || len(c.edges[v]) > 0
			for _, u := range c.uses[v] {
				ok = ok && u.kind != useEscape
			}
			for _, s := range c.edges[v] {
				if !owned[s] {
					ok = false
				}
			}
			for _, m := range c.moves[v] {
				ok = ok && c.moveOK(v, m)
			}
			if !ok {
				delete(owned, v)
				changed = true
			}
		}
	}
	return owned
}

// mentions reports v used at statement i of r or anywhere inside it.
func (c *relCtx) mentions(v V, r *Region, i int) bool {
	for _, u := range c.uses[v] {
		ch := c.chain(u.at)
		for _, p := range ch {
			if p.r == r && p.i == i {
				return true
			}
		}
	}
	return false
}

// movedAt reports a move of v at statement or terminator i of r, or inside it.
func (c *relCtx) movedAt(v V, r *Region, i int) bool {
	for _, m := range c.moves[v] {
		for _, p := range c.chain(m.at) {
			if p.r == r && p.i == i {
				return true
			}
		}
	}
	return false
}

// place finds where owned v dies in region r, from statement `from` on: after
// its last use there, or in each arm where it is live in, and nowhere on a
// path that moves it.
func (c *relCtx) place(v V, r *Region, from int, at map[*Region][]int) {
	end := len(r.Stmts)
	if c.movedAt(v, r, end) && r.T != TBranch {
		return // moved by the terminator
	}
	if r.T == TUnreachable {
		return
	}
	if r.T == TBranch && (c.mentions(v, r, end)) {
		c.place(v, r.Then, 0, at)
		c.place(v, r.Else, 0, at)
		return
	}
	last := -1
	for i := end - 1; i >= from; i-- {
		if c.mentions(v, r, i) {
			last = i
			break
		}
	}
	if PlantReleaseFirstUse {
		for i := from; i < end; i++ {
			if c.mentions(v, r, i) {
				last = i
				break
			}
		}
	}
	switch {
	case last < 0:
		at[r] = append(at[r], from)
	case c.movedAt(v, r, last) && !PlantReleaseMoved:
		s := &r.Stmts[last]
		if s.Op == OIf {
			for _, sub := range s.Sub {
				c.place(v, sub, 0, at)
			}
		}
		// moved by the statement itself (an init, a `the`): no release
	default:
		at[r] = append(at[r], last+1)
	}
}

// Release is the pass: a release after each owned table's last use on every
// path where it is not moved. It reports how many it placed.
func Release(tg *emit.Target, f *Func) int {
	if !termRealized(tg, "release") {
		return 0
	}
	c := newRelCtx(tg, f)
	owned := c.ownership()
	type ins struct {
		v V
		i int
	}
	plan := map[*Region][]ins{}
	for v := range owned {
		if !c.table(v) {
			continue
		}
		d := c.def[v]
		at := map[*Region][]int{}
		c.place(v, d.r, d.i+1, at)
		for r, is := range at {
			for _, i := range is {
				plan[r] = append(plan[r], ins{v, i})
			}
		}
	}
	n := 0
	fresh := func(ty string) V {
		v := V(len(f.Types))
		f.Types = append(f.Types, ty)
		return v
	}
	// a build's buffer frozen into none of its results, released right after
	// the build by its allocation's own (n, w)
	dead := map[*Region]map[int]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if !deadBuild(f, s) {
				continue
			}
			if a := allocOf(r, i, s.Store[0]); a != nil {
				if dead[r] == nil {
					dead[r] = map[int]*Stmt{}
				}
				dead[r][i] = a
			}
		}
	})
	for r := range dead { // a region with a dead build and no table's release
		if _, ok := plan[r]; !ok {
			plan[r] = nil
		}
	}
	for r, is := range plan {
		at := map[int][]V{}
		for _, x := range is {
			at[x.i] = append(at[x.i], x.v)
		}
		var out []Stmt
		emitAt := func(i int) {
			for _, v := range sortedVs(at[i]) {
				p, ln, w, ok := fresh("ptr"), fresh("int"), fresh("int"), fresh("bool")
				out = append(out,
					Stmt{Op: OStorage, Args: []V{v}, Res: []V{p}, FromTerm: true},
					Stmt{Op: OLen, Args: []V{v}, Res: []V{ln}, FromTerm: true},
					Stmt{Op: OConst, Lit: core.Int(int64(ElemBytes(tg, f.Types[v]))), Res: []V{w}, FromTerm: true},
					Stmt{Op: OCall, Name: "release", Args: []V{p, ln, w}, Res: []V{ok}, FromTerm: true})
				n++
			}
		}
		for i := range r.Stmts {
			emitAt(i)
			out = append(out, r.Stmts[i])
			if a := dead[r][i]; a != nil {
				ok := fresh("bool")
				out = append(out, Stmt{Op: OCall, Name: "release", Args: []V{r.Stmts[i].Store[0], a.Args[0], a.Args[1]},
					Res: []V{ok}, FromTerm: true})
				n++
			}
		}
		emitAt(len(r.Stmts))
		r.Stmts = out
	}
	return n
}

func sortedVs(vs []V) []V {
	out := append([]V(nil), vs...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

// CheckReleases is W11 (ir.md §3), re-derived from the IR alone, apart from
// the pass: every release frees a table that is owned, moved nowhere, used by
// nothing after the release, released once on every path, and released in the
// loop nesting it was defined in.
func CheckReleases(tg *emit.Target, f *Func) error {
	c := newRelCtx(tg, f)
	var errs []string
	var rels []loc
	relOf := map[loc]V{}
	var walk func(r *Region)
	walk = func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			// release(storage(t), len t, w): the table released is the one
			// whose storage the first argument is
			if s.Op == OCall && s.Name == "release" && len(s.Args) == 3 && i >= 1 &&
				r.Stmts[i-1].Op == OBuild && len(r.Stmts[i-1].Store) == 1 && r.Stmts[i-1].Store[0] == s.Args[0] {
				if err := checkDeadBuildRelease(f, r, i); err != nil {
					errs = append(errs, err.Error())
				}
			} else if s.Op == OCall && s.Name == "release" && len(s.Args) == 3 && i >= 3 {
				if st := &r.Stmts[i-3]; st.Op == OStorage && len(st.Res) == 1 && st.Res[0] == s.Args[0] {
					at := loc{r, i}
					rels = append(rels, at)
					relOf[at] = c.rootOf(st.Args[0])
				} else {
					errs = append(errs, fmt.Sprintf("W11: a release of %%%d, which is no table's storage", s.Args[0]))
				}
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(f.Body)
	if len(errs) > 0 {
		return fmt.Errorf("%s", errs[0])
	}
	// owned, re-derived: a birth, or an edge target every source of which is
	// owned and moved in once, with nothing after the move and no loop between.
	var owned func(v V, seen map[V]bool) bool
	owned = func(v V, seen map[V]bool) bool {
		if seen[v] {
			return true
		}
		seen[v] = true
		if c.births[v] {
			return true
		}
		srcs := c.edges[v]
		if len(srcs) == 0 {
			return false
		}
		for _, s := range srcs {
			if !owned(s, seen) || len(c.moves[s]) != 1 {
				return false
			}
			m := c.moves[s][0]
			d := c.def[s].r
			if c.loopBetween(d, m.at) {
				return false
			}
			for _, u := range c.uses[s] {
				if u.at != m.at && c.mayFollow(d, m.at, u.at) {
					return false
				}
				if u.kind == useEscape {
					return false
				}
			}
		}
		return true
	}
	for _, at := range rels {
		v := relOf[at]
		d := c.def[v]
		switch {
		case !owned(v, map[V]bool{}):
			return fmt.Errorf("W11: %%%d is released, and it is not owned: its storage may have another reference", v)
		case len(c.moves[v]) > 0:
			for _, m := range c.moves[v] {
				if !c.mayFollow(d.r, at, m.at) && !c.mayFollow(d.r, m.at, at) {
					continue // the move and the release are on exclusive paths
				}
				return fmt.Errorf("W11: %%%d is released on a path where it is moved on", v)
			}
		}
		if c.loopBetween(d.r, at) {
			return fmt.Errorf("W11: %%%d is released inside a loop it is defined outside of, once an iteration", v)
		}
		for _, u := range c.uses[v] {
			if u.at == at || relOf[u.at] == v {
				continue
			}
			// the release's own storage and length, taken just before it
			if s := stmtAt(u.at); s != nil && (s.Op == OLen || s.Op == OStorage) && u.at.r == at.r && at.i-3 <= u.at.i && u.at.i < at.i {
				continue
			}
			if c.mayFollow(d.r, at, u.at) {
				return fmt.Errorf("W11: %%%d is used after it is released", v)
			}
		}
		for _, other := range rels {
			if other != at && relOf[other] == v && c.mayFollow(d.r, at, other) {
				return fmt.Errorf("W11: %%%d is released twice on one path", v)
			}
		}
	}
	return nil
}

// checkDeadBuildRelease is W11 for a build's storage released after it,
// re-derived: the region freezes no member of own(b), the release's size is
// the allocation's own (n, w), and nothing else in the function names p, so
// this release is its one death and nothing reads it after.
func checkDeadBuildRelease(f *Func, r *Region, i int) error {
	b, rel := &r.Stmts[i-1], &r.Stmts[i]
	p := rel.Args[0]
	own := ownOf(f, b)
	frozen := own == nil
	var exits func(x *Region)
	exits = func(x *Region) {
		if x.T == TBranch {
			exits(x.Then)
			exits(x.Else)
		} else if x.T == TYield {
			for _, a := range x.Args {
				frozen = frozen || own[a]
			}
		}
	}
	exits(b.Sub[0])
	if frozen {
		return fmt.Errorf("W11: the storage of a build is released after it, and its buffer is frozen into the build's value")
	}
	a := allocOf(r, i-1, p)
	if a == nil || a.Args[0] != rel.Args[1] || a.Args[1] != rel.Args[2] {
		return fmt.Errorf("W11: a build's storage %%%d is released by a size that is not its allocation's", p)
	}
	names := 0
	f.Walk(func(x *Region) {
		for k := range x.Stmts {
			s := &x.Stmts[k]
			for _, v := range append(append([]V(nil), s.Args...), s.Store...) {
				if v == p {
					names++
				}
			}
		}
		for _, v := range x.Args {
			if v == p {
				names++
			}
		}
	})
	if names != 2 { // the build's store and the release
		return fmt.Errorf("W11: a build's storage %%%d is named %d times, not by its build and its release alone", p, names)
	}
	return nil
}

func stmtAt(at loc) *Stmt {
	if at.i >= 0 && at.i < len(at.r.Stmts) {
		return &at.r.Stmts[at.i]
	}
	return nil
}

// identityCoercion is a host operation whose term is its one argument's
// address, `getelementptr i8, ${x}, i64 0` (loaded as `%1`), from a table to
// a value this pass does not follow: its result is that table's own storage.
func identityCoercion(q emit.Prim) bool {
	if len(q.Args) != 1 || q.Result == "" || strings.HasPrefix(q.Result, "array") {
		return false
	}
	return strings.TrimSpace(q.Form) == "getelementptr i8, %1, i64 0"
}

// PlantCoercionBorrows reads llvm/str.text as borrowing its table, as before
// strings-2026-11-16: a program's table is then released under its string.
var PlantCoercionBorrows bool

// consumer is the index of the statement after i in r that uses v, or −1.
func consumer(r *Region, i int, v V) int {
	for j := i + 1; j < len(r.Stmts); j++ {
		for _, a := range r.Stmts[j].Args {
			if a == v {
				return j
			}
		}
	}
	return -1
}

// PlantFnReleasedEarly places a λ's region at its `fn` statement, as before:
// a table the λ reads is then released before the call that applies it
// (TestACallbackCapturesItsEnvironmentThroughTheContextWord, windows only).
var PlantFnReleasedEarly bool
