package ir

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
)

// A DEAD TABLE'S STORAGE IS REUSED BY THE NEXT TABLE OF ITS SIZE (ADR 0096,
// R1b). After W11 a released region has no reference, so its address is
// unobservable, and as transformers of the heap
//
//	release(p, b) ; q ← region(b)   ≡   q ← zero(p, b)
//
// region's contract being b fresh bytes, every one 0. That is Perceus's reuse
// (Reinking et al. 2021), and frame-limited reuse when the token is carried to
// the next allocation (Lorenzen and Leijen 2022). A token flows forward from a
// death to the next allocation of its size:
//   - LOCALLY, a release followed in its region by an allocation of its size:
//     the allocation takes the region, and the release is gone;
//   - ACROSS A LOOP'S BACK EDGE, where a carried table dies at the end of an
//     iteration and the next is made early in the next one. The body gains a
//     token parameter, null on entry, and the token is LINEAR: on every path
//     through the body it is consumed once, by the allocation that takes it
//     (`reuse`: the token zeroed, or a fresh region when it is null), or given
//     back (`discard`: released unless null) at the start of an arm that does
//     not allocate. Every continuing arm passes the next token: the storage of
//     a table of the size it releases, or null.
// Sizes are static, the facts' singletons, so a token never meets a table it
// does not fit. W12 checks every token apart from the pass.
//
// A SIZE OF THE RUN'S (R1d): a region answered for b serves any b' ≤ b of the
// same class (Σ_OS's `class`, the rule `region` and `release` dispatch on),
//
//	release(p, b) ; q ← region(b')   ≡   q ← zero(p, b')     for b' ≤ b, class(b') = class(b)
//
// and a reuse keeps the region's class, so the table's own release later gives
// it back by the right rule. Where either size is not static the token carries
// its table's element count k with it, and `reuse-fit` compares the request
// with it at run time, taking the region when it fits and giving it back
// otherwise. A K(ℕ*) value's size grows slowly across a loop, so its dead
// predecessor's region almost always fits.

// The planted faults, each of which W12 must refuse.
var (
	// PlantFitCount hands a fitting token the allocation's own count as k, not
	// its table's: a region too small taken as fitting.
	PlantFitCount bool
	// PlantFitUnchecked takes a token of a run-time size by `reuse`, unchecked.
	PlantFitUnchecked bool
	// PlantReuseSize hands a token to an allocation of another size.
	PlantReuseSize bool
	// PlantReuseKeepRelease keeps the release of a region handed on as a
	// token: freed, then written.
	PlantReuseKeepRelease bool
	// PlantReuseNoDiscard leaves a token unconsumed on an arm that does not
	// allocate.
	PlantReuseNoDiscard bool
)

// a release statement: release(storage(t), len t, w) at r.Stmts[i]
type relStmt struct {
	r    *Region
	i    int
	p    V     // the storage
	size int64 // n·w + 8, or −1 when not static
	n, w V     // the count and the width
}

// an allocation statement: q = alloc(count, w) at r.Stmts[i]
type allocStmt struct {
	r    *Region
	i    int
	size int64
	n, w V
}

// staticInt is v's value when a constant or a singleton fact gives it.
func staticInt(def map[V]*Stmt, fs []fact, v V) (int64, bool) {
	if s := def[v]; s != nil && s.Op == OConst && s.Lit != nil && s.Lit.Kind == core.KInt {
		return s.Lit.Int, true
	}
	if int(v) >= len(fs) {
		return 0, false
	}
	x := fs[v].v
	if !x.bot && !x.nlo && !x.phi && !x.lo.lt(x.hi) && x.lo.big().IsInt64() {
		return x.lo.big().Int64(), true
	}
	return 0, false
}

// terminalArms is a loop body's terminator tree's leaves: the regions an
// iteration ends in, one on every path.
func terminalArms(body *Region) []*Region {
	var arms []*Region
	var walk func(r *Region)
	walk = func(r *Region) {
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
			return
		}
		arms = append(arms, r)
	}
	walk(body)
	return arms
}

// reuseOf is the reuse an allocation becomes: one that zeroes the token for a
// table that reads its fill, and one that does not for a table born full.
func reuseOf(alloc string) string {
	if alloc == "alloc-raw" {
		return "reuse-raw"
	}
	return "reuse"
}

// fitOf is the reuse by fit an allocation becomes, by its kind as reuseOf.
func fitOf(alloc string) string {
	if PlantFitUnchecked {
		return reuseOf(alloc)
	}
	if alloc == "alloc-raw" {
		return "reuse-fit-raw"
	}
	return "reuse-fit"
}

// regionOp reports an operation that answers a fresh region of (n, w), its
// last two arguments: an allocation, or a reuse of a token for one.
func regionOp(name string) bool {
	switch name {
	case "alloc", "alloc-raw", "reuse", "reuse-raw", "reuse-fit", "reuse-fit-raw":
		return true
	}
	return false
}

// Reuse is the pass. It reports how many allocations take a token.
func Reuse(tg *emit.Target, f *Func) int {
	if !termRealized(tg, "reuse") || !termRealized(tg, "discard") {
		return 0
	}
	fs := analyse(tg, f)
	def := map[V]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			for _, v := range r.Stmts[i].Res {
				def[v] = &r.Stmts[i]
			}
		}
	})
	size := func(n, w V) int64 {
		k, ok1 := staticInt(def, fs, n)
		c, ok2 := staticInt(def, fs, w)
		if !ok1 || !ok2 {
			return -1
		}
		return k*c + 8
	}
	isRelease := func(r *Region, i int) (relStmt, bool) {
		s := &r.Stmts[i]
		if s.Op != OCall || s.Name != "release" || len(s.Args) != 3 {
			return relStmt{}, false
		}
		return relStmt{r, i, s.Args[0], size(s.Args[1], s.Args[2]), s.Args[1], s.Args[2]}, true
	}
	isAlloc := func(r *Region, i int) (allocStmt, bool) {
		s := &r.Stmts[i]
		if s.Op != OCall || (s.Name != "alloc" && s.Name != "alloc-raw") || len(s.Args) != 2 {
			return allocStmt{}, false
		}
		return allocStmt{r, i, size(s.Args[0], s.Args[1]), s.Args[0], s.Args[1]}, true
	}
	fits := func(a, b int64) bool { return PlantReuseSize || (a >= 0 && a == b) }
	// by fit: either size the run's, the widths one constant
	fitting := termRealized(tg, "reuse-fit")
	byFit := func(relSize int64, relW V, a allocStmt) bool {
		if !fitting || (relSize >= 0 && a.size >= 0) {
			return false
		}
		x, ok1 := staticInt(def, fs, relW)
		y, ok2 := staticInt(def, fs, a.w)
		return ok1 && ok2 && x == y
	}
	fitArgs := func(tok, k V, a allocStmt) []V {
		if PlantFitCount {
			k = a.n
		}
		if PlantFitUnchecked {
			return []V{tok, a.n, a.w}
		}
		return []V{tok, k, a.n, a.w}
	}
	n := 0
	used := map[*Region]map[int]bool{} // a release already handed on
	use := func(r *Region, i int) {
		if used[r] == nil {
			used[r] = map[int]bool{}
		}
		used[r][i] = true
	}
	// LOCALLY: in each region, a release's token goes to the next allocation
	// of its size at the same level.
	f.Walk(func(r *Region) {
		var pending []relStmt
		for i := range r.Stmts {
			if rel, ok := isRelease(r, i); ok && (rel.size >= 0 || fitting) {
				pending = append(pending, rel)
				continue
			}
			if a, ok := isAlloc(r, i); ok {
				for k, rel := range pending {
					s := &r.Stmts[i]
					switch {
					case rel.size >= 0 && fits(rel.size, a.size):
						s.Name, s.Args = reuseOf(s.Name), []V{rel.p, s.Args[0], s.Args[1]}
					case byFit(rel.size, rel.w, a):
						s.Name, s.Args = fitOf(s.Name), fitArgs(rel.p, rel.n, a)
					default:
						continue
					}
					use(rel.r, rel.i)
					pending = append(pending[:k], pending[k+1:]...)
					n++
					break
				}
			}
		}
	})
	// ACROSS A LOOP'S BACK EDGE.
	fresh := func(ty string) V {
		v := V(len(f.Types))
		f.Types = append(f.Types, ty)
		return v
	}
	type at struct {
		r *Region
		i int
	}
	var loops []at
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if r.Stmts[i].Op == OLoop {
				loops = append(loops, at{r, i})
			}
		}
	})
	before := map[*Region]map[int][]Stmt{} // statements before a loop: its null tokens
	front := map[*Region][]Stmt{}          // statements at an arm's start: its discards
	for _, l := range loops {
		s := &l.r.Stmts[l.i]
		body := s.Sub[0]
		arms := terminalArms(body)
		for _, home := range arms {
			for i := range home.Stmts {
				a, ok := isAlloc(home, i)
				if !ok {
					continue
				}
				// a size of the run's takes a token by fit, its count k with it
				dyn := a.size < 0
				if dyn && !fitting {
					continue
				}
				// a producer: some continuing arm's release of the size
				tokOf := map[*Region]relStmt{}
				for _, arm := range arms {
					if arm.T != TContinue {
						continue
					}
					for j := range arm.Stmts {
						// the last release first; the planted fault takes the first
						k := len(arm.Stmts) - 1 - j
						if PlantReuseSize {
							k = j
						}
						rel, ok := isRelease(arm, k)
						if ok && !used[arm][k] && ((!dyn && fits(rel.size, a.size)) || (dyn && byFit(rel.size, rel.w, a))) {
							tokOf[arm] = rel
							break
						}
					}
				}
				if len(tokOf) == 0 {
					continue
				}
				as := &home.Stmts[i]
				count, wv := as.Args[0], as.Args[1]
				tok := fresh("ptr")
				body.Params = append(body.Params, tok)
				// by fit, the token's element count travels beside it: 0 on entry
				kp := V(-1)
				if dyn {
					kp = fresh("int")
					body.Params = append(body.Params, kp)
				}
				null := fresh("ptr")
				if before[l.r] == nil {
					before[l.r] = map[int][]Stmt{}
				}
				before[l.r][l.i] = append(before[l.r][l.i], Stmt{Op: OCall, Name: "llvm.null", Res: []V{null}, FromTerm: true})
				s.Args = append(s.Args, null)
				if dyn {
					zero := fresh("int")
					before[l.r][l.i] = append(before[l.r][l.i], Stmt{Op: OConst, Lit: core.Int(0), Res: []V{zero}, FromTerm: true})
					s.Args = append(s.Args, zero)
				}
				for _, arm := range arms {
					// consumed by the allocation on its arm, given back on the others
					// with constants of its own where the size is static, and with
					// the token's own count where it is the run's
					if arm != home && arm.T != TUnreachable && !PlantReuseNoDiscard {
						c, _ := staticInt(def, fs, wv)
						kv, cv, ok := kp, fresh("int"), fresh("bool")
						if !dyn {
							k, _ := staticInt(def, fs, count)
							kv = fresh("int")
							front[arm] = append(front[arm], Stmt{Op: OConst, Lit: core.Int(k), Res: []V{kv}, FromTerm: true})
						}
						front[arm] = append(front[arm],
							Stmt{Op: OConst, Lit: core.Int(c), Res: []V{cv}, FromTerm: true},
							Stmt{Op: OCall, Name: "discard", Args: []V{tok, kv, cv}, Res: []V{ok}, FromTerm: true})
					}
					if arm.T != TContinue {
						continue
					}
					if rel, ok := tokOf[arm]; ok {
						arm.Args = append(arm.Args, rel.p)
						if dyn {
							arm.Args = append(arm.Args, rel.n)
						}
						use(arm, rel.i)
						continue
					}
					nv := fresh("ptr")
					arm.Stmts = append(arm.Stmts, Stmt{Op: OCall, Name: "llvm.null", Res: []V{nv}, FromTerm: true})
					arm.Args = append(arm.Args, nv)
					if dyn {
						zv := fresh("int")
						arm.Stmts = append(arm.Stmts, Stmt{Op: OConst, Lit: core.Int(0), Res: []V{zv}, FromTerm: true})
						arm.Args = append(arm.Args, zv)
					}
				}
				if dyn {
					as.Name, as.Args = fitOf(as.Name), fitArgs(tok, kp, a)
				} else {
					as.Name, as.Args = reuseOf(as.Name), []V{tok, count, wv}
				}
				n++
			}
		}
	}
	// the releases handed on, the null tokens before loops, the discards
	f.Walk(func(r *Region) {
		if used[r] == nil && before[r] == nil && front[r] == nil {
			return
		}
		out := append([]Stmt(nil), front[r]...)
		for i := range r.Stmts {
			out = append(out, before[r][i]...)
			if !used[r][i] || PlantReuseKeepRelease {
				out = append(out, r.Stmts[i])
			}
		}
		r.Stmts = out
	})
	return n
}

// CheckReuse is W12, re-derived apart from the pass: every token is null, or
// the storage of a table no one uses after it was taken and whose region
// nothing else releases, or a dead build's region (release.go), each of the
// size its allocation asks for; and a loop's
// token is consumed, by one `reuse` or one `discard`, exactly once on every
// path through the body.
func CheckReuse(tg *emit.Target, f *Func) error {
	fs := analyse(tg, f)
	def := map[V]*Stmt{}
	defAt := map[V]loc{}
	paramOf := map[V]*Region{}
	loopOf := map[*Region]*Stmt{}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, v := range s.Res {
				def[v], defAt[v] = s, loc{r, i}
			}
			if s.Op == OLoop {
				loopOf[s.Sub[0]] = s
				for _, p := range s.Sub[0].Params {
					paramOf[p] = s.Sub[0]
				}
			}
		}
	})
	released := map[V]bool{}
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if s.Op == OCall && s.Name == "release" && len(s.Args) == 3 {
				released[s.Args[0]] = true
			}
		}
	})
	c := newRelCtx(tg, f)
	// A DEAD BUILD'S REGION (release.go): allocated for a build whose buffer
	// is frozen into none of its results, by `alloc` or by a `reuse` that took
	// a token for it (a region of the same size), named by nothing but that
	// build and the one point that hands it on; its size, (n, w), its
	// allocation's last two arguments
	deadRegion := func(v V) (*Stmt, error) {
		a := def[v]
		at := defAt[v]
		if released[v] {
			return nil, fmt.Errorf("W12: a dead build's region %%%d is handed on as a token and released too", v)
		}
		if at.i+1 >= len(at.r.Stmts) {
			return nil, fmt.Errorf("W12: a token %%%d allocated for no build", v)
		}
		b := &at.r.Stmts[at.i+1]
		if b.Op != OBuild || len(b.Store) != 1 || b.Store[0] != v {
			return nil, fmt.Errorf("W12: a token %%%d allocated for no build", v)
		}
		own := ownOf(f, b)
		frozen := own == nil
		var exits func(x *Region)
		exits = func(x *Region) {
			if x.T == TBranch {
				exits(x.Then)
				exits(x.Else)
			} else if x.T == TYield {
				for _, y := range x.Args {
					frozen = frozen || own[y]
				}
			}
		}
		exits(b.Sub[0])
		if frozen {
			return nil, fmt.Errorf("W12: the region of a build whose buffer is frozen into its value is handed on as a token")
		}
		names := 0
		f.Walk(func(x *Region) {
			for k := range x.Stmts {
				for _, y := range append(append([]V(nil), x.Stmts[k].Args...), x.Stmts[k].Store...) {
					if y == v {
						names++
					}
				}
			}
			for _, y := range x.Args {
				if y == v {
					names++
				}
			}
		})
		if names != 2 { // its build and the point that hands it on
			return nil, fmt.Errorf("W12: a dead build's region %%%d is named %d times, not by its build and one hand-on", v, names)
		}
		return a, nil
	}
	token := func(v V, b int64) error {
		s := def[v]
		switch {
		case s != nil && s.Op == OCall && s.Name == "llvm.null":
			return nil
		case s != nil && s.Op == OCall && regionOp(s.Name):
			a, err := deadRegion(v)
			if err != nil {
				return err
			}
			k, ok1 := staticInt(def, fs, a.Args[len(a.Args)-2])
			w, ok2 := staticInt(def, fs, a.Args[len(a.Args)-1])
			if !ok1 || !ok2 {
				return fmt.Errorf("W12: a dead build's region %%%d of a size not static, so its token fits nothing", v)
			}
			if k*w+8 != b {
				return fmt.Errorf("W12: a token of %d bytes is handed to an allocation of %d", k*w+8, b)
			}
			return nil
		case s != nil && s.Op == OStorage:
			t := c.rootOf(s.Args[0])
			if released[v] {
				return fmt.Errorf("W12: the region of %%%d is handed on as a token and released too", t)
			}
			k, ok := int64(0), false
			for _, u := range c.uses[t] {
				if st := stmtAt(u.at); st != nil && st.Op == OLen {
					if x, ok2 := staticInt(def, fs, st.Res[0]); ok2 {
						k, ok = x, true
					}
				}
			}
			if !ok {
				return fmt.Errorf("W12: %%%d's size is not static, so its token fits nothing", t)
			}
			w := int64(ElemBytes(tg, f.Types[t]))
			if k*w+8 != b {
				return fmt.Errorf("W12: a token of %d bytes is handed to an allocation of %d", k*w+8, b)
			}
			here := defAt[v]
			for _, u := range c.uses[t] {
				if st := stmtAt(u.at); st != nil && (st.Op == OLen || st.Op == OStorage) && u.at.r == here.r {
					continue
				}
				if c.mayFollow(c.def[t].r, here, u.at) {
					return fmt.Errorf("W12: %%%d is used after its region is handed on", t)
				}
			}
			return nil
		}
		return fmt.Errorf("W12: a token %%%d that is neither null nor a table's storage", v)
	}
	// BY FIT (R1d): the token is null, or the storage of a table handed on as
	// a static token is, and k beside it is that table's own length, so the
	// comparison n ≤ k and a token given back are the region's (W12, re-derived)
	fitToken := func(v, k V, w V) error {
		s := def[v]
		switch {
		case s != nil && s.Op == OCall && s.Name == "llvm.null":
			return nil
		case s != nil && s.Op == OCall && regionOp(s.Name):
			a, err := deadRegion(v)
			if err != nil {
				return err
			}
			if k != a.Args[len(a.Args)-2] {
				return fmt.Errorf("W12: the count handed with a dead build's region %%%d is not its allocation's", v)
			}
			x, ok1 := staticInt(def, fs, w)
			y, ok2 := staticInt(def, fs, a.Args[len(a.Args)-1])
			if !ok1 || !ok2 || x != y {
				return fmt.Errorf("W12: a token of a dead build's width is handed to an allocation of another")
			}
			return nil
		case s != nil && s.Op == OStorage:
			t := c.rootOf(s.Args[0])
			if released[v] {
				return fmt.Errorf("W12: the region of %%%d is handed on as a token and released too", t)
			}
			if d := def[k]; d == nil || d.Op != OLen || c.rootOf(d.Args[0]) != t {
				return fmt.Errorf("W12: the count handed with %%%d's region is not its length", t)
			}
			if x, ok := staticInt(def, fs, w); !ok || x != int64(ElemBytes(tg, f.Types[t])) {
				return fmt.Errorf("W12: a token of %%%d's width is handed to an allocation of another", t)
			}
			here := defAt[v]
			for _, u := range c.uses[t] {
				if st := stmtAt(u.at); st != nil && (st.Op == OLen || st.Op == OStorage) && u.at.r == here.r {
					continue
				}
				if c.mayFollow(c.def[t].r, here, u.at) {
					return fmt.Errorf("W12: %%%d is used after its region is handed on", t)
				}
			}
			return nil
		}
		return fmt.Errorf("W12: a token %%%d that is neither null nor a table's storage", v)
	}
	sizeOf := func(s *Stmt) (int64, bool) {
		n, ok1 := staticInt(def, fs, s.Args[1])
		w, ok2 := staticInt(def, fs, s.Args[2])
		return n*w + 8, ok1 && ok2
	}
	var err error
	checked := map[V]bool{}
	// a reuse by fit, [tok, k, n, w]: local, its token checked where it stands;
	// a loop's, its pair checked at every source, and the token consumed once
	// on every path, a discard giving it back by the same k
	checkFit := func(s *Stmt) error {
		tok, k, w := s.Args[0], s.Args[1], s.Args[3]
		body, isParam := paramOf[tok]
		if !isParam {
			return fitToken(tok, k, w)
		}
		if checked[tok] {
			return nil
		}
		checked[tok] = true
		ls := loopOf[body]
		j, jk := -1, -1
		for x, p := range body.Params {
			if p == tok {
				j = x
			}
			if p == k {
				jk = x
			}
		}
		if jk < 0 {
			return fmt.Errorf("W12: a loop's fitting token travels without its count")
		}
		type pair struct{ t, k V }
		srcs := []pair{{ls.Args[j], ls.Args[jk]}}
		for _, arm := range terminalArms(body) {
			if arm.T == TContinue {
				srcs = append(srcs, pair{arm.Args[j], arm.Args[jk]})
			}
			if arm.T == TUnreachable {
				continue
			}
			uses := 0
			for p := arm; ; {
				for _, st := range p.Stmts {
					if st.Op != OCall || len(st.Args) < 2 || st.Args[0] != tok {
						continue
					}
					switch st.Name {
					case "reuse-fit", "reuse-fit-raw":
						uses++
					case "discard":
						if st.Args[1] != k {
							return fmt.Errorf("W12: a fitting token is given back by a count not its own")
						}
						uses++
					case "reuse", "reuse-raw":
						return fmt.Errorf("W12: a token of a run-time size is taken unchecked")
					}
				}
				q, ok := c.parent[p]
				if !ok || p == body {
					break
				}
				p = q.r
			}
			if uses != 1 {
				return fmt.Errorf("W12: a loop's token is consumed %d times on a path through its body, not once", uses)
			}
		}
		for _, src := range srcs {
			if d := def[src.t]; d != nil && d.Op == OCall && d.Name == "llvm.null" {
				continue
			}
			if e := fitToken(src.t, src.k, w); e != nil {
				return e
			}
		}
		return nil
	}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if err == nil && s.Op == OCall && (s.Name == "reuse-fit" || s.Name == "reuse-fit-raw") && len(s.Args) == 4 {
				err = checkFit(s)
				continue
			}
			if err != nil || s.Op != OCall || (s.Name != "reuse" && s.Name != "reuse-raw") || len(s.Args) != 3 {
				continue
			}
			b, ok := sizeOf(s)
			if !ok {
				err = fmt.Errorf("W12: a reuse whose size is not static")
				return
			}
			tok := s.Args[0]
			body, isParam := paramOf[tok]
			if !isParam {
				err = token(tok, b)
				continue
			}
			if checked[tok] {
				continue
			}
			checked[tok] = true
			ls := loopOf[body]
			j := -1
			for k, p := range body.Params {
				if p == tok {
					j = k
				}
			}
			srcs := []V{ls.Args[j]}
			for _, arm := range terminalArms(body) {
				if arm.T == TContinue {
					srcs = append(srcs, arm.Args[j])
				}
				// LINEAR: consumed once on the arm's path, by a reuse or a discard
				if arm.T == TUnreachable {
					continue
				}
				uses := 0
				for p := arm; ; {
					for _, st := range p.Stmts {
						if st.Op == OCall && (st.Name == "reuse" || st.Name == "reuse-raw" || st.Name == "discard") && len(st.Args) > 0 && st.Args[0] == tok {
							uses++
						}
					}
					q, ok := c.parent[p]
					if !ok || p == body {
						break
					}
					p = q.r
				}
				if uses != 1 {
					err = fmt.Errorf("W12: a loop's token is consumed %d times on a path through its body, not once", uses)
					return
				}
			}
			for _, src := range srcs {
				if e := token(src, b); e != nil {
					err = e
					return
				}
			}
		}
	})
	return err
}
