package ir

import (
	"fmt"

	"oroboros/core"
	"oroboros/emit"
)

// THE UNSIGNED WORD, CHOSEN ON THE IR (ADR 0033, ADR 0026 (10)).
//
// A target that realizes U = [0, 2⁶⁴) beside its signed word S holds each
// integer value in the least realization containing its interval, S
// preferred:
//
//	ρ(I) = S  if I ⊆ S        ρ(I) = U  if I ⊆ U, I ⊄ S
//
// A declaration fixes a value's realization where it defines one (a U-typed
// parameter, a host primitive's U result). Then, by ADR 0026's theorem (the
// quotient q : ℤ → ℤ/2⁶⁴ is a ring homomorphism, and S and U are systems of
// representatives):
//   - `+ − ·` compute in their result's realization, operands converted by the
//     residue map, and the result is exact because it is proven in it;
//   - `/ %` and the comparisons compute in a realization holding BOTH
//     operands, where the conversions are the identity on the values that
//     occur (q preserves neither order nor truncating division);
//   - every flow into a position of the other realization is converted:
//     `u64-of` into U, `int-of-u64` into S. Into a DECLARED position (a host
//     argument, an index, a length, the function's result) the value must lie
//     in the receiver's realization: an obligation, discharged by its fact or
//     refused naming the interval.
//
// The facts are the decision's, on the unselected function. After the
// rewrite the function is renumbered and its facts are recomputed, since a
// conversion of a value outside S ∩ U changes its integer (by the residue map).

// SelectWords chooses the realization of f's integer values and rewrites f to
// match. It reports whether anything changed, or the first obligation no fact
// discharges.
func SelectWords(tg *emit.Target, f *Func) (bool, error) {
	if !tg.Word.Unsigned {
		return false, nil
	}
	fs := f.facts
	if len(fs) != f.NV() {
		fs = analyse(tg, f)
	}
	w := &words{tg: tg, f: f, fs: fs, u: map[V]bool{}, s: ei(tg.Word.Lo), S: ei(tg.Word.Hi), convOf: map[V]V{}}
	w.choose()
	if !w.any {
		return false, nil
	}
	w.rewrite()
	if w.err != nil {
		return false, w.err
	}
	if w.rewrote == 0 && w.retyped == 0 {
		return false, nil
	}
	Canonicalize(f)
	f.facts = nil
	return true, nil
}

type words struct {
	tg    *emit.Target
	f     *Func
	fs    []fact
	u     map[V]bool // the values realized in U
	s, S  ep         // the signed word's ends
	any   bool
	err   error
	stmts []Stmt // the region being rebuilt
	// loopParams is the stack of enclosing loops' parameters, which a
	// continue's arguments are received in; region rebuilds one region.
	loopParams [][]V
	region     func(r *Region, recv []bool)
	// convOf is each conversion's source. The residue maps between U and S
	// are mutually inverse bijections, so a conversion of a conversion back is
	// its source: u64-of ∘ int-of-u64 is the identity on U, and the reverse on
	// S.
	convOf map[V]V
	// rewrote counts conversions inserted and operations moved into U;
	// retyped, values whose type is set to their chosen realization.
	rewrote, retyped int
}

func (w *words) isU(ty string) bool { return w.tg.ValueType(ty) == core.U64Type }

// integer reports a value of the integer sort, in either realization.
func (w *words) integer(v V) bool {
	ty := w.f.Types[v]
	if ty == "int" || w.isU(ty) || ty == "u64" {
		return true
	}
	_, _, ok := core.IntRangeBig(ty)
	return ok && w.tg.ValueType(ty) == "int"
}

func (w *words) inS(v V) bool { return w.fs[v].v.within(w.s, w.S) }
func (w *words) inU(v V) bool { return w.fs[v].v.within(ep{}, u64Max) }

// choose fixes each value's realization: declared where a definition declares
// one, ρ of its fact otherwise, a π's source's for a π.
func (w *words) choose() {
	f := w.f
	for _, p := range f.Params {
		if w.isU(f.Types[p]) || f.Types[p] == "u64" {
			w.u[p] = true
		}
	}
	pis := map[V]V{}
	f.Walk(func(r *Region) {
		for _, pi := range r.Pis {
			pis[pi.V] = pi.Of
		}
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Op == OCall {
				p := w.tg.Prims[s.Name]
				for j, v := range s.Res {
					ty := p.Result
					if len(p.Results) >= 2 && j < len(p.Results) {
						ty = p.Results[j]
					}
					if w.isU(ty) {
						w.u[v] = true
					}
				}
				continue
			}
			for _, v := range s.Res {
				if w.integer(v) && !w.inS(v) && w.inU(v) {
					w.u[v] = true
				}
			}
			if s.Op == OLoop {
				for _, p := range s.Sub[0].Params {
					if w.integer(p) && !w.inS(p) && w.inU(p) {
						w.u[p] = true
					}
				}
			}
		}
	})
	for v, of := range pis {
		for {
			if o, ok := pis[of]; ok {
				of = o
				continue
			}
			break
		}
		w.u[v] = w.u[of]
	}
	// THE TYPE IS THE REPRESENTATION (ADR 0032): every integer value's type is
	// set to the realization chosen here. Unification typed a value u64 when it
	// flows from a U declaration, and ρ of its fact may be S; the choice is
	// written back in both directions, and a declared range inside S keeps its
	// spelling.
	for v := 0; v < f.NV(); v++ {
		x := V(v)
		if !w.integer(x) {
			continue
		}
		ty := f.Types[x]
		switch isU := w.isU(ty) || ty == "u64"; {
		case w.u[x] && !isU:
			f.Types[x] = "u64"
			w.retyped++
		case !w.u[x] && isU:
			f.Types[x] = "int"
			w.retyped++
		}
	}
	w.any = len(w.u) > 0 || w.retyped > 0
}

// fresh is a new value of the given type.
func (w *words) fresh(ty string) V {
	w.rewrote++
	v := V(len(w.f.Types))
	w.f.Types = append(w.f.Types, ty)
	w.fs = append(w.fs, fact{v: ivTop})
	return v
}

// conv returns v in realization toU, emitting a conversion into w.stmts when
// v is in the other one. declared says membership is an OBLIGATION: into a
// declaration, and into a join (a result, a loop variable, a stored value),
// whose incoming fact lies in the receiver's, so the conversion is the
// identity unless the receiver itself is unbounded, where a silent residue
// would change the integer. Only an operand of + − · converts freely, by the
// homomorphism.
func (w *words) conv(v V, toU bool, declared bool, where string) V {
	if v < 0 || !w.integer(v) || w.u[v] == toU {
		return v
	}
	if src, ok := w.convOf[v]; ok && w.u[src] == toU {
		return src // a conversion back cancels
	}
	if toU {
		if declared && !w.inU(v) && w.err == nil {
			w.err = fmt.Errorf("%s is %s, which is not inside [0, 18446744073709551615], the unsigned word "+
				"it is declared in (ADR 0026, ADR 0033); prove it non-negative and below 2^64", where, termShow(w.fs[v].v))
		}
		out := w.fresh("u64")
		w.stmts = append(w.stmts, Stmt{Op: OCall, Name: "u64-of", Args: []V{v}, Res: []V{out}})
		w.u[out] = true
		w.convOf[out] = v
		return out
	}
	if declared && !w.inS(v) && w.err == nil {
		w.err = fmt.Errorf("%s is %s, which is outside target %s's signed word [%d, %d] and inside "+
			"[0, 18446744073709551615], so it is held in the UNSIGNED word, not an `int` (ADR 0026, ADR 0033).\n"+
			"  The two realizations agree only modulo 2^64: +, − and · convert freely where the result "+
			"is proven in one of them, and nothing else does. Prove the value back inside the signed "+
			"word (narrow it, or divide it down), or declare the destination (int 0 18446744073709551615).",
			where, termShow(w.fs[v].v), w.tg.Name, w.tg.Word.Lo, w.tg.Word.Hi)
	}
	out := w.fresh("int")
	w.stmts = append(w.stmts, Stmt{Op: OCall, Name: "int-of-u64", Args: []V{v}, Res: []V{out}})
	w.convOf[out] = v
	return out
}

var wordOp = map[Op]string{OAdd: "u64+", OSub: "u64-", OMul: "u64*", ODiv: "u64/", ORem: "u64%",
	OEq: "u64=", OLt: "u64<", OLe: "u64<=", OGt: "u64>", OGe: "u64>="}

// rewrite rebuilds every region, statement by statement.
func (w *words) rewrite() {
	var region func(r *Region, recv []bool)
	// recv: for a region's yields/breaks, the realization each exit value is
	// received in (nil: decided by the terminator's own receiver below).
	region = func(r *Region, recv []bool) {
		for _, pi := range r.Pis {
			if w.u[pi.V] {
				w.f.Types[pi.V] = "u64"
			}
		}
		outer := w.stmts
		w.stmts = nil
		for i := range r.Stmts {
			s := r.Stmts[i]
			w.stmt(&s)
		}
		// terminators
		switch r.T {
		case TYield, TBreak:
			for j, a := range r.Args {
				if j < len(recv) {
					r.Args[j] = w.conv(a, recv[j], true, "a result")
				}
			}
		case TContinue:
			if len(w.loopParams) > 0 {
				ps := w.loopParams[len(w.loopParams)-1]
				for j, a := range r.Args {
					if j < len(ps) {
						r.Args[j] = w.conv(a, w.u[ps[j]], true, "a loop variable")
					}
				}
			}
		}
		r.Stmts = w.stmts
		w.stmts = outer
		if r.T == TBranch {
			region(r.Then, recv)
			region(r.Else, recv)
		}
	}
	w.region = region
	resU := make([]bool, len(w.f.Results))
	for j, ty := range w.f.Results {
		resU[j] = w.isU(ty) || ty == "u64"
	}
	region(w.f.Body, resU)
}

func (w *words) stmt(s *Stmt) {
	switch {
	case (s.Op == OAdd || s.Op == OSub || s.Op == OMul) && s.Name == "" && len(s.Res) == 1:
		toU := w.u[s.Res[0]]
		for j, a := range s.Args {
			s.Args[j] = w.conv(a, toU, false, "an operand")
		}
		if toU {
			s.Op, s.Mode, s.Name = OCall, MNone, wordOp[s.Op]
			w.f.Types[s.Res[0]] = "u64"
			w.rewrote++
		}
		w.stmts = append(w.stmts, *s)
	case s.Op == ONeg && len(s.Res) == 1:
		s.Args[0] = w.conv(s.Args[0], false, false, "an operand")
		w.u[s.Res[0]] = false
		w.stmts = append(w.stmts, *s)
	case (s.Op == ODiv || s.Op == ORem || s.Op.IsCmp()) && s.Name == "" && len(s.Args) == 2:
		a, b := s.Args[0], s.Args[1]
		bothS := (!w.integer(a) || w.inS(a)) && (!w.integer(b) || w.inS(b))
		bothU := w.integer(a) && w.integer(b) && w.inU(a) && w.inU(b)
		switch {
		case bothS && !w.u[a] && !w.u[b]:
			w.stmts = append(w.stmts, *s)
		case bothS:
			s.Args[0], s.Args[1] = w.conv(a, false, false, "an operand"), w.conv(b, false, false, "an operand")
			w.stmts = append(w.stmts, *s)
		case bothU && s.Op != ONe:
			s.Args[0], s.Args[1] = w.conv(a, true, false, "an operand"), w.conv(b, true, false, "an operand")
			name := wordOp[s.Op]
			w.rewrote++
			if s.Op.IsCmp() {
				s.Op, s.Mode, s.Name = OCall, MNone, name
				w.stmts = append(w.stmts, *s)
				return
			}
			// the quotient or remainder is in U; converted back where its own
			// value is realized in S
			res := s.Res[0]
			if w.u[res] {
				s.Op, s.Mode, s.Name = OCall, MNone, name
				w.f.Types[res] = "u64"
				w.stmts = append(w.stmts, *s)
				return
			}
			q := w.fresh("u64")
			w.u[q] = true
			w.stmts = append(w.stmts, Stmt{Op: OCall, Name: name, Args: s.Args, Res: []V{q}})
			w.stmts = append(w.stmts, Stmt{Op: OCall, Name: "int-of-u64", Args: []V{q}, Res: []V{res}})
			w.convOf[res] = q
		default:
			// NO REALIZATION HOLDS BOTH (a value past 2⁶³ against a possibly
			// negative one): q preserves order and division in neither, so the
			// operation has no word-width meaning. Refused by name when a U
			// value is involved; otherwise left for the decision.
			if (w.u[a] || w.u[b]) && w.err == nil {
				w.err = fmt.Errorf("a %s of %s against %s: one is held in the unsigned word and the other "+
					"may be negative, and no realization holds both (ADR 0026, ADR 0033). The two agree only "+
					"modulo 2^64, which preserves neither order nor division; prove both in one word, or compare "+
					"in two parts", s.Op, termShow(w.fs[a].v), termShow(w.fs[b].v))
			}
			w.stmts = append(w.stmts, *s)
		}
	case s.Op == OCall:
		p := w.tg.Prims[s.Name]
		for j, a := range s.Args {
			if j < len(p.Args) && p.Kind != "stmt" {
				ty := p.Args[j]
				switch {
				case w.isU(ty):
					s.Args[j] = w.conv(a, true, true, fmt.Sprintf("argument %d of %s", j+1, s.Name))
				case ty == "int" || w.tg.ValueType(ty) == "int":
					s.Args[j] = w.conv(a, false, true, fmt.Sprintf("argument %d of %s", j+1, s.Name))
				}
			}
		}
		for j, v := range s.Res {
			if w.u[v] {
				w.f.Types[v] = "u64"
			}
			_ = j
		}
		w.stmts = append(w.stmts, *s)
	case s.Op == OIndex || s.Op == OSet:
		s.Args[1] = w.conv(s.Args[1], false, true, "an index")
		if s.Op == OSet {
			s.Args[2] = w.conv(s.Args[2], false, true, "a stored value")
		}
		w.stmts = append(w.stmts, *s)
	case s.Op == OBuild || s.Op == OTabulate || s.Op == OBuildMap:
		s.Args[0] = w.conv(s.Args[0], false, true, "a size")
		w.stmts = append(w.stmts, *s)
		recv := w.recvOf(s.Res)
		for _, sub := range s.Sub {
			w.region(sub, recv)
		}
	case s.Op == OIf:
		recv := w.recvOf(s.Res)
		for _, v := range s.Res {
			if w.u[v] {
				w.f.Types[v] = "u64"
			}
		}
		w.stmts = append(w.stmts, *s)
		for _, sub := range s.Sub {
			w.region(sub, recv)
		}
	case s.Op == OLoop:
		body := s.Sub[0]
		for j, a := range s.Args {
			if j < len(body.Params) {
				s.Args[j] = w.conv(a, w.u[body.Params[j]], true, "a loop's initial value")
			}
		}
		for _, p := range body.Params {
			if w.u[p] {
				w.f.Types[p] = "u64"
			}
		}
		for _, v := range s.Res {
			if w.u[v] {
				w.f.Types[v] = "u64"
			}
		}
		w.stmts = append(w.stmts, *s)
		w.loopParams = append(w.loopParams, body.Params)
		w.region(body, w.recvOf(s.Res))
		w.loopParams = w.loopParams[:len(w.loopParams)-1]
	default:
		w.stmts = append(w.stmts, *s)
	}
}

func (w *words) recvOf(res []V) []bool {
	out := make([]bool, len(res))
	for j, v := range res {
		out[j] = w.u[v]
	}
	return out
}

// WordOps counts the unsigned word's operations and conversions in f, as the
// note reports them: what the selection left.
func WordOps(f *Func) int {
	n := 0
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			if s.Op == OCall && (s.Name == "u64-of" || s.Name == "int-of-u64" || isWordName(s.Name)) {
				n++
			}
		}
	})
	return n
}

func isWordName(name string) bool {
	for _, w := range wordOp {
		if w == name {
			return true
		}
	}
	return false
}
