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
)

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
			}
			switch s.Op {
			case OIndex:
				for j, a := range s.Args {
					add(a, use{at: at, kind: map[bool]int{true: useBorrow, false: useEscape}[j == 0]})
				}
			case OLen, OStorage:
				add(s.Args[0], use{at: at, kind: useBorrow})
			case OCall:
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
			if s.Op == OCall && s.Name == "release" && len(s.Args) == 3 && i >= 3 {
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

func stmtAt(at loc) *Stmt {
	if at.i >= 0 && at.i < len(at.r.Stmts) {
		return &at.r.Stmts[at.i]
	}
	return nil
}
