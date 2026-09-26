package ir

import (
	"oroboros/emit"
)

// THE MUTABLE BIGNUM, ON THE IR (emit/bigreuse.go's rule R, restated on SSA).
//
// Go's math/big writes into its receiver, so `d.Mul(a, b)` computes a·b into
// storage d instead of allocating. A bignum operation may write into the object
// o held by a loop parameter p when no read of o can follow the write. On a
// structured SSA function that is two conditions, and each is a theorem about
// the function rather than about a term's shape.
//
// OWNERSHIP. A loop parameter p is OWNED when every value bound to it, by the
// loop's initialiser and by each of its continues, is
//   - a FRESH object: the result of an allocating bignum call (`big-of`, or an
//     arithmetic form) through at most one `big-fit`, each link read exactly
//     once, and on a continue allocated inside this loop's own iteration; or
//   - the object of another owned parameter;
// and no one continue binds one object to two parameters. Owned is the
// GREATEST such set, reached by removing violators until none remain. By
// induction on iterations, an owned parameter's object is held by nothing
// else live: initially it is fresh and read once; at each back edge each owned
// slot receives a fresh object or one another owned slot held exclusively,
// and no object twice.
//
// LIVENESS. An arithmetic call s whose result reaches a continue may compute
// into owned p's object when p's object is one of s's operands (the shape the
// host's contract names) and no value that may hold it — p, a π of p, a
// `big-fit` or a destination of one, a join one reaches — is read after s on
// any path to the end of the iteration: the rest of s's region, the regions
// of the operations after it, and the rest of each enclosing region up to the
// loop's body. s must sit in the iteration itself, under branches and `if`
// arms only: a `tabulate` body runs again, and a nested loop is its own.
//
// Then after s, p's object is held only by s's result, which is read once, by
// one continue slot, so ownership is kept. The term rule's condition (4)
// checked the initialiser only; a continue binding an object from outside the
// loop would have been written into on the next iteration. Here that binding
// is not owned.
//
// Java's BigInteger and JavaScript's BigInt are immutable: the rule does not
// fire there, because those targets declare no destination forms.

var bigDest = map[string]string{"big+": "big+!", "big-": "big-!", "big*": "big*!", "big/": "big/!", "big%": "big%!"}

// bigParent is where a region sits: an arm of a branch, or a region owned by
// the statement at index idx of its region.
type bigParent struct {
	region *Region // the enclosing region
	idx    int     // the owning statement's index, or -1 for a branch arm
}

type reuse struct {
	tg  *emit.Target
	f   *Func
	def map[V]*Stmt
	at  map[V]struct {
		r *Region
		i int
	}
	parent map[*Region]bigParent
	uses   map[V]int
	pis    map[V]V
	joins  []bigJoin
}

func reuseBig(tg *emit.Target, f *Func) {
	u := &reuse{tg: tg, f: f, def: map[V]*Stmt{}, parent: map[*Region]bigParent{},
		uses: useCounts(f), pis: map[V]V{}}
	u.at = map[V]struct {
		r *Region
		i int
	}{}
	var loops []*Stmt
	var walk func(r *Region)
	walk = func(r *Region) {
		for _, pi := range r.Pis {
			u.pis[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, v := range s.Res {
				u.def[v] = s
				u.at[v] = struct {
					r *Region
					i int
				}{r, i}
			}
			for _, sub := range s.Sub {
				u.parent[sub] = bigParent{r, i}
				walk(sub)
			}
			switch s.Op {
			case OLoop:
				loops = append(loops, s)
				body := s.Sub[0]
				conts := ownExits(body, TContinue)
				for j, p := range body.Params {
					in := []V{s.Args[j]}
					for _, a := range conts {
						in = append(in, a[j])
					}
					u.joins = append(u.joins, bigJoin{p, in})
				}
				breaks := ownExits(body, TBreak)
				for j, v := range s.Res {
					var in []V
					for _, a := range breaks {
						in = append(in, a[j])
					}
					u.joins = append(u.joins, bigJoin{v, in})
				}
			case OIf:
				for j, v := range s.Res {
					var in []V
					for _, sub := range s.Sub {
						for _, a := range ownExits(sub, TYield) {
							in = append(in, a[j])
						}
					}
					u.joins = append(u.joins, bigJoin{v, in})
				}
			}
		}
		if r.T == TBranch {
			u.parent[r.Then] = bigParent{r, -1}
			u.parent[r.Else] = bigParent{r, -1}
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(f.Body)
	for _, l := range loops {
		u.loop(l)
	}
}

// obj follows a value to the one it renames: a π's source. (A continue's
// argument that is a parameter's object is the parameter or a π of it: fits
// and destinations wrap operations, never a bare parameter.)
func (u *reuse) obj(v V) V {
	for {
		o, ok := u.pis[v]
		if !ok {
			return v
		}
		v = o
	}
}

func destSource(name string) (string, bool) {
	for src, d := range bigDest {
		if d == name {
			return src, true
		}
	}
	return "", false
}

// fresh returns the allocating call a value is the object of, when every link
// from it is read exactly once; nil otherwise.
func (u *reuse) fresh(v V) *Stmt {
	for {
		if u.uses[v] != 1 {
			return nil
		}
		s := u.def[v]
		if s == nil || s.Op != OCall {
			return nil
		}
		if (s.Name == "big-fit" || s.Name == "big-fit-signed") && len(s.Args) == 2 {
			v = s.Args[0]
			continue
		}
		if _, ok := bigDest[s.Name]; ok || s.Name == "big-of" {
			return s
		}
		return nil
	}
}

// inIteration reports whether region r is body or an arm of its branches.
// A continue's argument is defined on the continue's own path, by dominance,
// so this asks one thing: whether it was allocated in this iteration rather
// than before the loop.
func (u *reuse) inIteration(r, body *Region) bool {
	for r != body {
		p, ok := u.parent[r]
		if !ok || p.idx >= 0 {
			return false
		}
		r = p.region
	}
	return true
}

func (u *reuse) loop(l *Stmt) {
	body := l.Sub[0]
	conts := ownExits(body, TContinue)
	owned := map[int]bool{}
	for j, p := range body.Params {
		if u.f.Types[p] == "big" {
			owned[j] = true
		}
	}
	isParam := func(v V) int {
		o := u.obj(v)
		for j, p := range body.Params {
			if p == o {
				return j
			}
		}
		return -1
	}
	for changed := true; changed; {
		changed = false
		drop := func(j int) {
			if owned[j] {
				delete(owned, j)
				changed = true
			}
		}
		for j := range body.Params {
			if !owned[j] {
				continue
			}
			if j >= len(l.Args) || u.fresh(l.Args[j]) == nil {
				drop(j)
				continue
			}
			for _, args := range conts {
				a := args[j]
				if q := isParam(a); q >= 0 && owned[q] {
					continue
				}
				if s := u.fresh(a); s != nil {
					if at, ok := u.at[s.Res[0]]; ok && u.inIteration(at.r, body) {
						continue
					}
				}
				drop(j)
				break
			}
		}
		for _, args := range conts {
			seen := map[V]int{}
			for j := range body.Params {
				if owned[j] {
					if q := isParam(args[j]); q >= 0 {
						if k, dup := seen[body.Params[q]]; dup {
							drop(k)
							drop(j)
						}
						seen[body.Params[q]] = j
					}
				}
			}
		}
	}
	if len(owned) == 0 {
		return
	}
	for _, args := range conts {
		taken := map[int]bool{}
		for _, a := range args {
			s := u.fresh(a)
			if s == nil || s.Name == "big-of" {
				continue
			}
			dst, ok := bigDest[s.Name]
			if _, have := u.tg.Prims[dst]; !ok || !have {
				continue
			}
			at, ok := u.at[s.Res[0]]
			if !ok || !u.inIteration(at.r, body) {
				continue
			}
			for j, p := range body.Params {
				if !owned[j] || taken[j] {
					continue
				}
				var recv V = -1
				for _, x := range s.Args {
					if u.obj(x) == p {
						recv = x
						break
					}
				}
				if recv < 0 || u.readAfter(at.r, at.i, u.holders(p), body) {
					continue
				}
				s.Args = append([]V{recv}, s.Args...)
				s.Name = dst
				taken[j] = true
				break
			}
		}
	}
}

// holders is every value that may hold p's object: p, and what a π, a
// `big-fit`, a destination or a join derives from one of them.
func (u *reuse) holders(p V) map[V]bool {
	h := map[V]bool{p: true}
	for changed := true; changed; {
		changed = false
		add := func(v V) {
			if !h[v] {
				h[v], changed = true, true
			}
		}
		for v, of := range u.pis {
			if h[of] {
				add(v)
			}
		}
		for v, s := range u.def {
			if s.Op == OCall && len(s.Args) > 0 && h[s.Args[0]] {
				if _, isDest := destSource(s.Name); isDest || s.Name == "big-fit" || s.Name == "big-fit-signed" {
					add(v)
				}
			}
		}
		for _, j := range u.joins {
			for _, x := range j.in {
				if h[x] {
					add(j.v)
				}
			}
		}
	}
	return h
}

// readAfter reports whether a holder is read after statement i of region r,
// on any path to the end of the iteration of the loop whose body is body.
func (u *reuse) readAfter(r *Region, i int, h map[V]bool, body *Region) bool {
	for {
		if readsIn(r.Stmts[i+1:], h) || readsExit(r, h) {
			return true
		}
		if r == body {
			return false
		}
		p := u.parent[r]
		if p.idx < 0 {
			// a branch arm: its parent's statements all came before
			for p.idx < 0 && p.region != body {
				p = u.parent[p.region]
			}
			if p.idx < 0 {
				return false
			}
		}
		r, i = p.region, p.idx
	}
}

// readsIn reports a read of a holder in statements and every region they own.
func readsIn(ss []Stmt, h map[V]bool) bool {
	for _, s := range ss {
		for _, a := range s.Args {
			if h[a] {
				return true
			}
		}
		for _, sub := range s.Sub {
			if readsRegion(sub, h) {
				return true
			}
		}
	}
	return false
}

func readsRegion(r *Region, h map[V]bool) bool {
	for _, pi := range r.Pis {
		if h[pi.Of] || (pi.Other >= 0 && h[pi.Other]) {
			return true
		}
	}
	return readsIn(r.Stmts, h) || readsExit(r, h)
}

// readsExit is a read in a region's terminator, and in its arms for a branch.
func readsExit(r *Region, h map[V]bool) bool {
	for _, a := range r.Args {
		if h[a] {
			return true
		}
	}
	if r.T == TBranch {
		return h[r.Cond] || readsRegion(r.Then, h) || readsRegion(r.Else, h)
	}
	return false
}
