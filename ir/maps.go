package ir

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
)

// A MAP ON THE LLVM LAYER IS REALIZED AFTER THE DECISION (ADR 0120).
//
// ρ_M takes a map of capacity c to the table t of 3n cells, n = slots(c), and
// each map operation to a term over that table (targets/llvm/lang.oro's
// map_*), a homomorphism h with d(h(op(m))) = op(d(m)) for the abstraction
// d(t) = {(t[3s+1], t[3s+2]) : t[3s] = 1}:
//
//	build-map(c){m. B}  ↦  build(cells(c)){t. h(B)}
//	insert(m, k, v)     ↦  map_put(t, k, v)     the table lent, written in place
//	read(m, k)          ↦  map_get(t, k)        the option's (tag, payload)
//	keys(m)             ↦  map_keys(t)
//	len(m)              ↦  map_size(t)
//	map {k : v, …}      ↦  map_of(ks, vs)
//
// It runs on the decided function and before storage, so the decision, the
// refinement layer before it and the facts saw the map, and the table a
// `build-map` becomes is allocated and released as every table is. Each
// instance is checked once against its own header and refined (Instance), and
// the decision taken again decides its arithmetic under the site's facts.
//
// A map's type becomes the table's, the instance's own parameter type, so
// nothing after this pass learns a map existed.

// RealizeMaps substitutes the map operations' realizations in f and reports
// how many.
func RealizeMaps(tg *emit.Target, f *Func) (int, error) {
	if !tg.States() || PlantMapsUnrealized {
		return 0, nil
	}
	def := func(op string) (string, bool) {
		names, _, ok := tg.RealizedNames(op)
		if !ok || len(names) == 0 || !tg.IsDefinition(names[0]) {
			return "", false
		}
		return names[0], true
	}
	if _, ok := def("insert"); !ok {
		return 0, nil
	}
	isMap := func(v V) bool {
		_, _, ok := core.MapTypes(f.Types[v])
		return ok
	}
	table := ""
	lit := map[V]bool{} // the literals' key and value tables
	count := 0
	ren := map[V]V{}
	var err error
	inline := func(out []Stmt, op string, args, res []V) []Stmt {
		name, ok := def(op)
		if !ok {
			if err == nil {
				err = fmt.Errorf("a map's %s has no realization on target %s (ADR 0120)", op, tg.Name)
			}
			return out
		}
		c, e := Instance(tg, name)
		if e != nil {
			if err == nil {
				err = e
			}
			return out
		}
		// THE TABLE'S TYPE is the realization's own parameter, a table of the
		// word: the cells hold flags, keys and values alike.
		if table == "" {
			for _, p := range c.Params {
				if core.ArrayElem(c.Types[p]) != "" {
					table = c.Types[p]
					break
				}
			}
		}
		count++
		return inlineInto(f, out, c, args, res, ren)
	}
	var region func(r *Region)
	region = func(r *Region) {
		var out []Stmt
		for i := range r.Stmts {
			s := r.Stmts[i]
			for _, sub := range s.Sub {
				region(sub)
			}
			switch {
			case s.Op == OBuildMap && len(s.Args) == 1:
				// build-map(c){m. B} ↦ build(cells(c)){t. h(B)}
				cells := V(len(f.Types))
				f.Types = append(f.Types, "int")
				out = inline(out, "build-map", s.Args, []V{cells})
				s.Op, s.Args = OBuild, []V{cells}
				out = append(out, s)
			case s.Op == OInsert:
				out = inline(out, "insert", s.Args, s.Res)
			case s.Op == ORead:
				out = inline(out, "read", s.Args, s.Res)
			case s.Op == OKeys:
				out = inline(out, "keys", s.Args, s.Res)
			case s.Op == OLen && len(s.Args) == 1 && isMap(s.Args[0]):
				out = inline(out, "map-len", s.Args, s.Res)
			case s.Op == OMap:
				// a literal: its keys and values as two tables, filled by one loop
				var ks, vs []V
				for j := 0; j+1 < len(s.Args); j += 2 {
					ks, vs = append(ks, s.Args[j]), append(vs, s.Args[j+1])
				}
				kt, vt := V(len(f.Types)), V(len(f.Types)+1)
				f.Types = append(f.Types, "", "")
				lit[kt], lit[vt] = true, true
				out = append(out, Stmt{Op: OArray, Args: ks, Res: []V{kt}}, Stmt{Op: OArray, Args: vs, Res: []V{vt}})
				out = inline(out, "map", []V{kt, vt}, s.Res)
			default:
				out = append(out, s)
			}
		}
		r.Stmts = out
		if r.T == TBranch {
			region(r.Then)
			region(r.Else)
		}
	}
	region(f.Body)
	if err != nil || count == 0 {
		return count, err
	}
	// A MAP'S TYPE IS THE TABLE'S from here on, and the literal's two tables
	// are tables of the word.
	for v, ty := range f.Types {
		if _, _, ok := core.MapTypes(ty); ok || lit[V(v)] {
			f.Types[v] = table
		}
	}
	renameAway(f, ren)
	Canonicalize(f)
	f.facts = nil
	return count, nil
}

// PlantMapsUnrealized leaves the map operations for the printer, which on the
// LLVM layer has none: TestAMapIsRealizedAfterTheDecision must catch it.
var PlantMapsUnrealized bool
