package ir

import (
	"fmt"
	"math/big"
	"sort"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// THE STEP FROM IR_A TO IR_P (spec §7), for what a printer needs.
//
// Obligations and modes are already decided when gen lowers (Options.Decided:
// the residual passed the legality and refinement checks on terms), so what
// remains is to make every TYPE final:
//
//   - `the` is erased: its result is its operand (L9's reading of an
//     ascription, spec §7 item 4); `require` never reaches L after discharge;
//   - every TABLE CLASS takes one representation, by Theorem D′ below;
//   - a scalar integer stays `int`, the target's word, which is a final type:
//     scalar ranges decide nothing a printer does, because u64 and big are
//     already their own sorts.
//
// THEOREM D′ (the narrowest uniform representation, with fixed members). Take
// the table-valued values of a function and the flow edges between them, and
// let a class be a connected component.
//   1. The representation is constant on a class (spec Theorem D, part 1).
//   2. If a member's representation is FIXED by a declaration — a signature's
//      parameter, a primitive's argument or result — the class takes it. Every
//      member's values lie in the declared set (the analyses proved it, or the
//      program was refused), so it is sound, and no other representation is
//      admissible, since the host compiled the declaration.
//   3. Otherwise the class takes ρ_T(hull), the hull of its SOURCES' element
//      ranges: a build's stores and its zero fill (the interval analysis on the
//      build's own λ, written at lowering), a literal graph's constants, and the
//      word for a source whose range nothing established. That is Theorem D.
//   Two different fixed representations in one class would be a program the
//   host cannot type; it is reported, not resolved.

// Finalize makes every type of p final and marks it IR_P.
func Finalize(tg *emit.Target, p *Program) error {
	for _, f := range p.Funcs {
		if err := finalizeFunc(tg, f); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	p.Stage = StageP
	return nil
}

type tclass struct {
	fixed   string // a declared representation's canonical type, "" if none
	lo, hi  *big.Int
	unknown bool // a source whose range nothing established: the word
	sawAny  bool // some source contributed
}

func finalizeFunc(tg *emit.Target, f *Func) error {
	// A PARAMETER NOTHING TYPES has no final type on a target that types its
	// values: no signature declares it and no operation it reaches fixes it.
	// Said here, in words, before W9 says it in a rule number.
	if tg.HostType("int") != "" {
		for i, x := range f.Params {
			if f.Types[x] == "any" {
				return fmt.Errorf("cannot determine a type for parameter %d: no signature declares it, "+
					"and it is never passed to an operation or a primitive whose signature would fix it", i+1)
			}
		}
	}
	restrictLoops(f) // L13, where an assumption discharges its premise
	nv := f.NV()
	// The IR's own interval analysis, on the IR_A types (before any is
	// rewritten): the second factor of the element range's reduced product.
	fs := analyse(tg, f)
	tainted := map[V]bool{} // a table a host call may write through an unfixed argument
	parent := make([]V, nv)
	for i := range parent {
		parent[i] = V(i)
	}
	var find func(V) V
	find = func(x V) V {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	isTable := func(v V) bool {
		t := unbuffer(f.Types[v])
		return strings.HasPrefix(t, "array ") || tableAlias(tg, t)
	}
	union := func(a, b V) {
		if a < 0 || b < 0 || !isTable(a) || !isTable(b) {
			return
		}
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	edge := func(r *Region, k Term, res []V) {
		var walk func(r *Region)
		walk = func(r *Region) {
			if r.T == k {
				for j, a := range r.Args {
					if j < len(res) {
						union(res[j], a)
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
	// The fixed representations: what a declaration says a value is, or what
	// a primitive demands of an argument.
	fixedAt := map[V]string{}
	fix := func(v V, ty string) {
		if v >= 0 && isTable(v) && declaresTable(tg, ty) {
			fixedAt[v] = ty
		}
	}
	for _, x := range f.Params {
		fix(x, f.Types[x])
	}
	// The function's declared results fix what its body yields.
	for _, ys := range exitsOf(f.Body, TYield) {
		for j, y := range ys {
			if j < len(f.Results) {
				fix(y, f.Results[j])
			}
		}
	}
	sources := map[V]tclass{} // a source's own contribution
	intConsts := map[V]*core.Term{}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			union(pi.V, pi.Of)
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch s.Op {
			case OConst:
				if s.Lit.Kind == core.KInt {
					intConsts[s.Res[0]] = s.Lit
				}
			case OThe:
				union(s.Res[0], s.Args[0])
			case OSet, OInsert, ORestrict:
				union(s.Res[0], s.Args[0]) // a restriction is a view: one representation
			case OCall:
				p := tg.Prims[s.Name]
				if p.Kind == "stmt" && len(s.Args) > 0 {
					union(s.Res[0], s.Args[0])
				}
				for j, a := range s.Args {
					if j < len(p.Args) {
						fix(a, p.Args[j])
					}
					// A host call may write into a table it is handed. Where its
					// declaration fixes the representation that is sound
					// whatever it writes; where nothing fixes it, nothing
					// bounds what it writes.
					if !p.Pure && isTable(a) && (j >= len(p.Args) || !declaresTable(tg, p.Args[j])) {
						tainted[a] = true
					}
				}
				for j, v := range s.Res {
					if len(p.Results) >= 2 && j < len(p.Results) {
						fix(v, p.Results[j])
					} else if len(p.Results) < 2 && p.Kind != "stmt" {
						fix(v, p.Result)
					}
				}
			case OIf:
				edge(s.Sub[0], TYield, s.Res)
				edge(s.Sub[1], TYield, s.Res)
			case OLoop:
				for j, q := range s.Sub[0].Params {
					union(q, s.Args[j])
				}
				edge(s.Sub[0], TContinue, s.Sub[0].Params)
				edge(s.Sub[0], TBreak, s.Res)
			case OBuild:
				b := s.Sub[0].Params[0]
				sources[b] = rangeSource(tg, f.Types[b])
				edge(s.Sub[0], TYield, s.Res)
			case OTabulate:
				sources[s.Res[0]] = tclass{unknown: true, sawAny: true}
			case OKeys:
				sources[s.Res[0]] = rangeSource(tg, f.Types[s.Res[0]])
			case OArray:
				c := tclass{sawAny: true}
				for _, a := range s.Args {
					k, ok := intConsts[a]
					if !ok {
						c.unknown = true
						break
					}
					c = joinConst(c, big.NewInt(k.Int))
				}
				if len(s.Args) == 0 {
					c = joinConst(c, big.NewInt(0))
				}
				sources[s.Res[0]] = c
			}
		}
	})
	// Classes: their fixed representation, and the hull of their sources.
	classes := map[V]*tclass{}
	get := func(v V) *tclass {
		r := find(v)
		c, ok := classes[r]
		if !ok {
			c = &tclass{}
			classes[r] = c
		}
		return c
	}
	var conflicts []string
	keys := make([]V, 0, len(fixedAt))
	for v := range fixedAt {
		keys = append(keys, v)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, v := range keys {
		ty := fixedAt[v]
		c := get(v)
		if c.fixed == "" {
			c.fixed = ty
		} else if tg.HostType(unbuffer(c.fixed)) != tg.HostType(unbuffer(ty)) {
			conflicts = append(conflicts, fmt.Sprintf("%%%d is fixed as %s and as %s", v, c.fixed, ty))
		}
	}
	for v, s := range sources {
		c := get(v)
		c.sawAny = true
		c.unknown = c.unknown || s.unknown
		if s.lo != nil {
			if c.lo == nil || s.lo.Cmp(c.lo) < 0 {
				c.lo = s.lo
			}
			if c.hi == nil || s.hi.Cmp(c.hi) > 0 {
				c.hi = s.hi
			}
		}
	}
	// The IR factor: ARRAY SMASHING, flow-insensitive (Blanchet et al. 2003).
	// A class's elements are exactly the values ever placed in one of its
	// tables, so its range is the join of their facts: the zero fill of a
	// build, each stored value, a literal's elements, a tabulation's yields,
	// and what a declaration says of a table handed in from outside. Each
	// placed value's scalar fact comes from the converged analysis, so it holds
	// on every execution; a value read back out of the same table carries that
	// table's own fact, so a circular dependence is sound, not assumed away.
	irHull := map[V]iv{}
	irUnknown := map[V]bool{}
	taintedClass := map[V]bool{}
	place := func(t V, x iv) {
		if t < 0 || !isTable(t) {
			return
		}
		r := find(t)
		if !x.finite() && !x.bot {
			irUnknown[r] = true
			return
		}
		if h, ok := irHull[r]; ok {
			irHull[r] = joinIV(h, x)
		} else {
			irHull[r] = x
		}
	}
	for v := 0; v < nv; v++ {
		if x := V(v); isTable(x) && tainted[x] {
			taintedClass[find(x)] = true
		}
	}
	outside := func(v V) { // a table from outside: what its declaration says
		if fs[v].hasEl {
			place(v, fs[v].el)
		} else {
			place(v, ivTop)
		}
	}
	for _, x := range f.Params {
		outside(x)
	}
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			switch s.Op {
			case OBuild:
				place(s.Sub[0].Params[0], exactIV(0)) // zero-filled (tables.md §14.3)
			case OSet:
				place(s.Args[0], fs[s.Args[2]].v)
			case OArray:
				for _, a := range s.Args {
					place(s.Res[0], fs[a].v)
				}
			case OTabulate:
				for _, y := range exitsOf(s.Sub[0], TYield) {
					if len(y) == 1 {
						place(s.Res[0], fs[y[0]].v)
					}
				}
			case OCall, OKeys, OGlobal:
				for _, v := range s.Res {
					if isTable(v) && !(s.Op == OCall && tg.Prims[s.Name].Kind == "stmt") {
						outside(v)
					}
				}
			}
		}
	})
	if len(conflicts) > 0 {
		return fmt.Errorf("Theorem D′: a table class with two fixed representations: %s", strings.Join(conflicts, "; "))
	}
	// Write the class's representation onto every member, keeping whether each
	// is a buffer.
	for v := 0; v < nv; v++ {
		x := V(v)
		if !isTable(x) {
			continue
		}
		c, ok := classes[find(x)]
		if !ok {
			continue
		}
		prefix := "array "
		if strings.HasPrefix(f.Types[x], "buffer ") {
			prefix = "buffer "
		}
		switch {
		case c.fixed != "":
			if e := core.ArrayElem(c.fixed); e != "" {
				f.Types[x] = prefix + e
			} else {
				f.Types[x] = c.fixed // a host alias: its realization is the representation
			}
		default:
			elem := core.ArrayElem(unbuffer(f.Types[x]))
			if tg.ValueType(elem) != "int" && elem != "int" {
				continue // not an integer table: nothing to narrow
			}
			r := find(x)
			if taintedClass[r] {
				continue // a host may write anything: the word
			}
			// THE REDUCED PRODUCT of the two factors: each is a sound
			// over-approximation of the class's elements, so their meet is.
			rng := ivTop
			if c.sawAny && !c.unknown && c.lo != nil && c.lo.IsInt64() && c.hi.IsInt64() {
				rng = rangeIV(c.lo.Int64(), c.hi.Int64())
			}
			if h, ok := irHull[r]; ok && !irUnknown[r] {
				rng = meetIV(rng, h)
			}
			if rng.bot {
				rng = exactIV(0) // no element ever stored or held: the zero fill's
			}
			if rng.finite() && rng.lo >= tg.Word.Lo && rng.hi <= tg.Word.Hi {
				f.Types[x] = prefix + fmt.Sprintf("int %d %d", rng.lo, rng.hi)
			}
		}
	}
	// `the` is erased: its result is renamed to its operand, and the statement
	// is dropped. Uses are rewritten through the renaming.
	eraseThe(f)
	return nil
}

func exitsOf(r *Region, k Term) [][]V {
	var out [][]V
	var walk func(r *Region)
	walk = func(r *Region) {
		if r.T == k {
			out = append(out, r.Args)
		}
		if r.T == TBranch {
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(r)
	return out
}

func rangeSource(tg *emit.Target, ty string) tclass {
	e := core.ArrayElem(ty)
	if lo, hi, ok := core.IntRangeBig(e); ok {
		return tclass{lo: lo, hi: hi, sawAny: true}
	}
	if e == "int" || tg.ValueType(e) == "int" {
		return tclass{unknown: true, sawAny: true}
	}
	return tclass{sawAny: true} // not an integer table: nothing to narrow
}

func joinConst(c tclass, k *big.Int) tclass {
	if c.lo == nil || k.Cmp(c.lo) < 0 {
		c.lo = k
	}
	if c.hi == nil || k.Cmp(c.hi) > 0 {
		c.hi = k
	}
	return c
}

// tableAlias reports whether a host atom realizes a language table.
func tableAlias(tg *emit.Target, ty string) bool {
	_, ok := newUnifier(tg).aliasOf(ty)
	return ok
}

// declaresTable reports whether a declared type fixes a table's
// representation: a language table type, or a host alias of one.
func declaresTable(tg *emit.Target, ty string) bool {
	if ty == "" || ty == "any" {
		return false
	}
	return core.ArrayElem(ty) != "" || tableAlias(tg, ty)
}

// eraseThe removes every `the`, renaming its result to its operand.
func eraseThe(f *Func) {
	ren := map[V]V{}
	f.Walk(func(r *Region) {
		out := r.Stmts[:0]
		for _, s := range r.Stmts {
			if s.Op == OThe && len(s.Res) == 1 && len(s.Args) == 1 {
				ren[s.Res[0]] = s.Args[0]
				continue
			}
			out = append(out, s)
		}
		r.Stmts = out
	})
	if len(ren) == 0 {
		return
	}
	m := func(v V) V {
		for {
			w, ok := ren[v]
			if !ok {
				return v
			}
			v = w
		}
	}
	ms := func(vs []V) []V {
		out := make([]V, len(vs))
		for i, v := range vs {
			out[i] = m(v)
		}
		return out
	}
	f.Walk(func(r *Region) {
		for i := range r.Pis {
			r.Pis[i].Of, r.Pis[i].Other = m(r.Pis[i].Of), m(r.Pis[i].Other)
		}
		for i := range r.Stmts {
			r.Stmts[i].Args = ms(r.Stmts[i].Args)
		}
		r.Args = ms(r.Args)
		if r.T == TBranch {
			r.Cond = m(r.Cond)
		}
	})
	Canonicalize(f)
}
