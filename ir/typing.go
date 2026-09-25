package ir

import (
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// TYPING (spec §2.2): every value is typed where it is defined, by the compiler,
// once. The residual is monomorphic and first-order, so this is Robinson
// unification with no generalisation: the most general unifier of a set of
// equations over the free algebra of type constructors
//
//	int, f64, bool, string, host atoms   (arity 0)
//	table                                 (arity 1: the element)
//	map                                   (arity 2: key, value)
//
// computed by union-find. Two refinements, each a quotient of that algebra:
//
//   - A HOST ALIAS OF A LANGUAGE TYPE is read as the language type. Go declares
//     `slice-float64`, which ρ_T realizes exactly as `(array f64)`; the equation
//     slice-float64 = table(f64) is ρ_T's kernel on that pair. The alias is
//     inverted over a finite candidate set, and only when the inverse is unique.
//   - `any` is the top of the language's type relation (types.md), not a
//     constructor: it satisfies every equation and fixes nothing.
//
// A buffer and a table are ONE sort, `table` (ADR 0020: a buffer is a table for
// every purpose but aliasing); which one a value is, is a separate property,
// computed by `buffers` below and rendered into its type.
//
// Ranges are not unified. A value's range is what its definition DECLARES (a
// signature, a primitive's result, an ascription); every other integer is
// `int`, the target's word, until the analyses write the final range (IR_P,
// spec §7). A clash between two constructors is not an error here: the program
// passed the type checker, whose relation is looser than equality, and the
// verifier judges each flow edge with that relation (W5).

type tnode struct {
	parent int
	con    string // "" a variable; "any"; "int" "f64" "bool" "string"; "table"; "map"; "@NAME" a host atom
	kids   []int
}

type unifier struct {
	tg      *emit.Target
	n       []tnode
	pending [][2]int
	elems   []string // candidate element types for inverting an alias
}

func newUnifier(tg *emit.Target) *unifier {
	u := &unifier{tg: tg}
	u.elems = []string{"int", "f64", "bool", "string"}
	var hosts []string
	for k := range tg.Types {
		hosts = append(hosts, k)
	}
	sort.Strings(hosts)
	u.elems = append(u.elems, hosts...)
	return u
}

func (u *unifier) mk(con string, kids ...int) int {
	u.n = append(u.n, tnode{parent: len(u.n), con: con, kids: kids})
	return len(u.n) - 1
}

func (u *unifier) fresh() int { return u.mk("") }

func (u *unifier) find(i int) int {
	for u.n[i].parent != i {
		u.n[i].parent = u.n[u.n[i].parent].parent
		i = u.n[i].parent
	}
	return i
}

// node builds the term a canonical type denotes.
func (u *unifier) node(ty string) int {
	switch {
	case ty == "" || ty == "any":
		return u.mk("any")
	case ty == "int":
		return u.mk("int")
	case ty == "f64" || ty == "bool" || ty == "string":
		return u.mk(ty)
	}
	if _, _, ok := core.IntRangeBig(ty); ok || ty == core.BigType {
		switch s := sortOf(u.tg, ty); s {
		case "int":
			return u.mk("int")
		case "array int":
			return u.mk("table", u.mk("int")) // big as limbs (sortOf)
		default:
			return u.mk("@" + s) // u64, or big held by the host
		}
	}
	if strings.HasPrefix(ty, "array ") || strings.HasPrefix(ty, "buffer ") {
		return u.mk("table", u.node(ty[strings.IndexByte(ty, ' ')+1:]))
	}
	if k, v, ok := core.MapTypes(ty); ok {
		return u.mk("map", u.node(k), u.node(v))
	}
	if e, ok := u.aliasOf(ty); ok {
		return u.mk("table", u.node(e))
	}
	return u.mk("@" + ty)
}

// aliasOf inverts ρ_T on a host atom that realizes `(array E)`: the element E,
// when exactly one candidate realizes it.
func (u *unifier) aliasOf(atom string) (string, bool) {
	h := u.tg.HostType(atom)
	if h == "" || strings.HasPrefix(h, "/*") {
		return "", false
	}
	found := ""
	for _, e := range u.elems {
		if e == atom {
			continue
		}
		if u.tg.HostType("array "+e) == h {
			if found != "" && u.tg.HostType(found) != u.tg.HostType(e) {
				return "", false // not unique: two readings
			}
			if found == "" {
				found = e
			}
		}
	}
	return found, found != ""
}

// unify adds the equation a = b.
func (u *unifier) unify(a, b int) {
	a, b = u.find(a), u.find(b)
	if a == b {
		return
	}
	ca, cb := u.n[a].con, u.n[b].con
	switch {
	case ca == "any" || cb == "any":
		return
	case ca == "":
		u.n[a].parent = b
		return
	case cb == "":
		u.n[b].parent = a
		return
	case ca == cb && len(u.n[a].kids) == len(u.n[b].kids):
		u.n[b].parent = a
		ka, kb := u.n[a].kids, u.n[b].kids
		for i := range ka {
			u.unify(ka[i], kb[i])
		}
		return
	}
	// A clash: left to the verifier, which judges it with the checker's own
	// relation (W5). Recorded so a second round can retry it once a variable
	// under it is known.
	u.pending = append(u.pending, [2]int{a, b})
}

// render is a node's canonical type; buffer says whether a table is one.
func (u *unifier) render(i int, buffer bool) string {
	r := u.find(i)
	nd := u.n[r]
	switch nd.con {
	case "", "any":
		return "any"
	case "table":
		e := u.render(nd.kids[0], false)
		if buffer {
			return "buffer " + e
		}
		return "array " + e
	case "map":
		return "map " + u.render(nd.kids[0], false) + " " + u.render(nd.kids[1], false)
	}
	return strings.TrimPrefix(nd.con, "@")
}

// typeFunc assigns every value of f its type. decl holds the declared types
// lowering found; the rest are solved for.
func typeFunc(tg *emit.Target, f *Func, decl map[V]string) {
	u := newUnifier(tg)
	nv := f.NV()
	node := make([]int, nv)
	for v := range node {
		node[v] = u.fresh()
	}
	for v, ty := range decl {
		u.unify(node[v], u.node(ty))
	}
	eq := func(a, b V) { u.unify(node[a], node[b]) }
	is := func(v V, ty string) {
		if ty != "" && ty != "any" {
			u.unify(node[v], u.node(ty))
		}
	}
	elem := func(t V) int {
		e := u.fresh()
		u.unify(node[t], u.mk("table", e))
		return e
	}
	mapOf := func(m V) (int, int) {
		k, v := u.fresh(), u.fresh()
		u.unify(node[m], u.mk("map", k, v))
		return k, v
	}
	yields := func(r *Region, k Term, res []V) {
		var walk func(r *Region)
		walk = func(r *Region) {
			if r.T == k {
				for j, a := range r.Args {
					if j < len(res) {
						eq(res[j], a)
					}
				}
			}
			if r.T == TBranch {
				walk(r.Then)
				walk(r.Else)
			}
		}
		walk(r)
	}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			eq(pi.V, pi.Of)
		}
		if r.T == TBranch {
			is(r.Cond, "bool")
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch s.Op {
			case OConst:
				switch s.Lit.Kind {
				case core.KInt:
					is(s.Res[0], "int")
				case core.KFloat:
					is(s.Res[0], "f64")
				case core.KBool:
					is(s.Res[0], "bool")
				case core.KStr:
					is(s.Res[0], "string")
				}
			case OAdd, OSub, OMul, ONeg, ODiv, ORem:
				for _, a := range s.Args {
					is(a, "int")
				}
				is(s.Res[0], "int")
			case OEq, ONe, OLt, OLe, OGt, OGe:
				for _, a := range s.Args {
					is(a, "int")
				}
				is(s.Res[0], "bool")
			case OCall:
				p := tg.Prims[s.Name]
				for j, a := range s.Args {
					if j < len(p.Args) {
						is(a, p.Args[j])
					}
				}
				switch {
				case p.Kind == "stmt" && len(s.Args) > 0:
					eq(s.Res[0], s.Args[0]) // a statement's value is argument 0
				case len(p.Results) >= 2:
					for j, v := range s.Res {
						if j < len(p.Results) {
							is(v, p.Results[j])
						}
					}
				case len(s.Res) == 1:
					is(s.Res[0], p.Result)
				}
			case OIndex:
				u.unify(node[s.Res[0]], elem(s.Args[0]))
				is(s.Args[1], "int")
			case OLen:
				is(s.Res[0], "int")
			case OArray:
				e := u.fresh()
				for _, a := range s.Args {
					u.unify(node[a], e)
				}
				u.unify(node[s.Res[0]], u.mk("table", e))
			case OMap:
				k, v := mapOf(s.Res[0])
				for j := 0; j+1 < len(s.Args); j += 2 {
					u.unify(node[s.Args[j]], k)
					u.unify(node[s.Args[j+1]], v)
				}
			case ORead:
				k, v := mapOf(s.Args[0])
				u.unify(node[s.Args[1]], k)
				is(s.Res[0], "int")
				u.unify(node[s.Res[1]], v)
			case OKeys:
				k, _ := mapOf(s.Args[0])
				u.unify(node[s.Res[0]], u.mk("table", k))
			case OSet:
				u.unify(node[s.Args[2]], elem(s.Args[0]))
				is(s.Args[1], "int")
				eq(s.Res[0], s.Args[0])
			case OInsert:
				k, v := mapOf(s.Args[0])
				u.unify(node[s.Args[1]], k)
				u.unify(node[s.Args[2]], v)
				eq(s.Res[0], s.Args[0])
			case OIf:
				is(s.Args[0], "bool")
				yields(s.Sub[0], TYield, s.Res)
				yields(s.Sub[1], TYield, s.Res)
			case OLoop:
				body := s.Sub[0]
				for j, p := range body.Params {
					eq(p, s.Args[j])
				}
				yields(body, TContinue, body.Params)
				yields(body, TBreak, s.Res)
			case OBuild:
				is(s.Args[0], "int")
				elem(s.Sub[0].Params[0])
				yields(s.Sub[0], TYield, s.Res)
			case OBuildMap:
				is(s.Args[0], "int")
				mapOf(s.Sub[0].Params[0])
				yields(s.Sub[0], TYield, s.Res)
			case OTabulate:
				is(s.Args[0], "int")
				is(s.Sub[0].Params[0], "int")
				e := u.fresh()
				var walk func(r *Region)
				walk = func(r *Region) {
					if r.T == TYield && len(r.Args) == 1 {
						u.unify(node[r.Args[0]], e)
					}
					if r.T == TBranch {
						walk(r.Then)
						walk(r.Else)
					}
				}
				walk(s.Sub[0])
				u.unify(node[s.Res[0]], u.mk("table", e))
			case OThe:
				eq(s.Res[0], s.Args[0])
			case ORequire:
				is(s.Args[0], "bool")
			}
		}
	})
	yields(f.Body, TYield, nil) // the function's own results carry no equation
	// Retry the clashes once more: a variable below one may be known now.
	for round := 0; round < 2 && len(u.pending) > 0; round++ {
		p := u.pending
		u.pending = nil
		for _, e := range p {
			u.unify(e[0], e[1])
		}
	}
	buf := buffers(tg, f)
	f.Types = make([]string, nv)
	for v := 0; v < nv; v++ {
		if ty, ok := decl[V(v)]; ok {
			f.Types[v] = ty
			continue
		}
		f.Types[v] = u.render(node[v], buf[v])
	}
	// A π is its source renamed (L9): in IR_A it has its source's type, and only
	// the analyses narrow it (IR_P). Outer regions are visited first, so a π of
	// a π takes the already-copied type.
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			f.Types[pi.V] = f.Types[pi.Of]
		}
	})
	// The function's results are the types of what its body yields.
	f.Results = nil
	var first func(r *Region) []V
	first = func(r *Region) []V {
		switch r.T {
		case TYield:
			return r.Args
		case TBranch:
			if y := first(r.Then); y != nil {
				return y
			}
			return first(r.Else)
		}
		return nil
	}
	for _, v := range first(f.Body) {
		f.Results = append(f.Results, f.Types[v])
	}
}

// buffers is which values are buffers (ADR 0018, 0020): a build's parameter,
// what a store returns, and whatever one flows into inside its scope — a loop's
// parameter, an if's result, a π, a statement's value. A build's RESULT is not:
// leaving the scope freezes it (ADR 0031). The least solution of those rules,
// by iteration from no buffers.
func buffers(tg *emit.Target, f *Func) []bool {
	b := make([]bool, f.NV())
	for v, ty := range f.Types {
		if strings.HasPrefix(ty, "buffer ") {
			b[v] = true
		}
	}
	for changed := true; changed; {
		changed = false
		set := func(v V, x bool) {
			if x && !b[v] {
				b[v], changed = true, true
			}
		}
		flowTo := func(r *Region, k Term, res []V) {
			var walk func(r *Region)
			walk = func(r *Region) {
				if r.T == k {
					for j, a := range r.Args {
						if j < len(res) {
							set(res[j], b[a])
						}
					}
				}
				if r.T == TBranch {
					walk(r.Then)
					walk(r.Else)
				}
			}
			walk(r)
		}
		f.Walk(func(r *Region) {
			for _, pi := range r.Pis {
				set(pi.V, b[pi.Of])
			}
			for i := range r.Stmts {
				s := &r.Stmts[i]
				switch s.Op {
				case OBuild, OBuildMap:
					set(s.Sub[0].Params[0], true)
				case OSet, OInsert:
					set(s.Res[0], true)
				case OCall:
					if len(s.Args) > 0 && len(s.Res) == 1 {
						set(s.Res[0], b[s.Args[0]] && tg.Prims[s.Name].Kind == "stmt")
					}
				case OThe:
					set(s.Res[0], b[s.Args[0]])
				case OIf:
					flowTo(s.Sub[0], TYield, s.Res)
					flowTo(s.Sub[1], TYield, s.Res)
				case OLoop:
					for j, p := range s.Sub[0].Params {
						set(p, b[s.Args[j]])
					}
					flowTo(s.Sub[0], TContinue, s.Sub[0].Params)
					flowTo(s.Sub[0], TBreak, s.Res)
				}
			}
		})
	}
	return b
}
