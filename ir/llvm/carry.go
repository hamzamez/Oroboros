package llvm

import (
	"math/big"

	"oroboros/ir"
)

// THE FULL ADDER (ADR 0108, llvm.md §3d).
//
// φ : [−B, 2B) → [0, B) × {0, 1}, B = 2⁶⁴, reads an integer by its two
// halves: t ↦ (t mod B, ⌊t/B⌋) on [0, 2B), and t ↦ (t mod B, [t < 0]) on
// [−B, B), the borrow. Each is a bijection (2B = B·2), and the machine's
// add-with-carry is φ ∘ (+) on U₆₄ × U₆₄ × {0, 1}:
//
//	adc(c, x, y) = φ(x + y + c),   sbb(b, x, y) = φ(x − y − b),
//
// since x + y + c ∈ [0, 2B) and x − y − b ∈ [−B, B) exactly when x, y < B
// and c, b ∈ {0, 1}. Knuth's A and S are folds of these, the carry their
// state (TAOCP 2 §4.3.1); in the double word (ADR 0103) the state passes
// through a register, setb and a zero extension, and by the triple's own
// operation it stays in the flag, which halves A's step (carry-2026-11-13).
//
// So a term of the double word is printed as the triple's adder when the
// IR's types prove its premise:
//   - it is x + y + c, x + c, x − y − b or x − b, each inner + or − of the
//     double word used by it alone, the leaves words: x, y ∈ [0, B), and c, b
//     ∈ [0, 1];
//   - its every use reads it by φ: t & m for a word m, which is (t mod B) & m
//     since & is 2-adic, and t >> 64, the carry, of a sum; t & m and t < 0,
//     the borrow, of a difference.
//
// The IR is unchanged (W14 holds of it as before): the choice is the
// printer's, a function of IR_P's text, as `nuw` is, and each premise is read
// off the types the decision wrote.
type adder struct {
	sub     bool
	c, x, y ir.V // y < 0: the constant 0
	call    string
}

const (
	addCarry  = "llvm/isa.add-carry"
	subBorrow = "llvm/isa.sub-borrow"
)

// PlantAdderFault takes a bit leaf in [0, 2] as the adder's carry, which the
// machine reads as 1 when it is 2: TestTheFullAdderNeedsItsCarryInZeroOne must
// see a wrong answer.
var PlantAdderFault bool

// PlantAdderReadFault takes any right shift of a sum as its carry, so t >> 63
// is printed as ⌊t / 2⁶⁴⌋: TestTheFullAdderReadsOnlyItsHalves must see a
// wrong answer.
var PlantAdderReadFault bool

var bigB1 = new(big.Int).SetUint64(1<<64 - 1)

// adders finds f's full adders: the top statement's result → its adder, and
// marks each inner + or − it absorbs, which is not printed.
func (p *printer) adders() {
	_, okA := p.tg.Prims[addCarry]
	_, okS := p.tg.Prims[subBorrow]
	if !okA && !okS || len(p.dbl) == 0 {
		return
	}
	def := map[ir.V]*ir.Stmt{}
	uses := map[ir.V][]*ir.Stmt{} // statement uses, by representative
	other := map[ir.V]bool{}      // used by anything else
	f := p.f
	f.Walk(func(r *ir.Region) {
		for _, a := range r.Args {
			other[p.pl.Res(a)] = true
		}
		if r.T == ir.TBranch {
			other[p.pl.Res(r.Cond)] = true
		}
		// a π is its source (the plan's aliasing), so its reads are counted
		// under the source's representative, here and in a region's exits
		for i := range r.Stmts {
			s := &r.Stmts[i]
			for _, a := range s.Args {
				uses[p.pl.Res(a)] = append(uses[p.pl.Res(a)], s)
			}
			for _, v := range s.Res {
				def[v] = s
			}
		}
	})
	word := func(v ir.V) bool {
		lo, hi := p.ends(v)
		return !p.dbl[v] && lo != nil && hi != nil && lo.Sign() >= 0 && hi.Cmp(bigB1) <= 0
	}
	bitHi := big.NewInt(1)
	if PlantAdderFault {
		bitHi = big.NewInt(2)
	}
	bit := func(v ir.V) bool {
		lo, hi := p.ends(v)
		return !p.dbl[v] && lo != nil && hi != nil && lo.Sign() >= 0 && hi.Cmp(bitHi) <= 0
	}
	// inner is an operand that is a + or − of the double word, read by s alone
	inner := func(v ir.V, op ir.Op, s *ir.Stmt) *ir.Stmt {
		d := def[v]
		r := p.pl.Res(v)
		if d == nil || d.Op != op || d.Mode != ir.MDouble || r != v || other[r] || len(uses[r]) != 1 || uses[r][0] != s {
			return nil
		}
		return d
	}
	isConst := func(v ir.V, want *big.Int) bool {
		lo, hi := p.ends(v)
		return lo != nil && hi != nil && lo.Cmp(want) == 0 && hi.Cmp(want) == 0
	}
	// reads says t's every use is φ's: the low half, and the carry or borrow
	reads := func(t ir.V, sub bool) bool {
		if other[t] || len(uses[t]) == 0 {
			return false
		}
		for _, u := range uses[t] {
			ok := false
			switch u.Op {
			case ir.OAnd:
				m := u.Args[0]
				if p.pl.Res(m) == t {
					m = u.Args[1]
				}
				// t & m = (t mod B) & m for any m ∈ [0, B): & is 2-adic, and m
				// has no bit at 64 or above
				ok = p.pl.Res(m) != t && word(m)
			case ir.OShr:
				ok = !sub && p.pl.Res(u.Args[0]) == t && (isConst(u.Args[1], big.NewInt(64)) || PlantAdderReadFault)
			case ir.OLt:
				ok = sub && p.pl.Res(u.Args[0]) == t && isConst(u.Args[1], big.NewInt(0))
			}
			if !ok || len(u.Res) != 1 || u.Mode == ir.MDouble {
				return false
			}
		}
		return true
	}
	found := map[ir.V]*adder{}
	skip := map[*ir.Stmt]bool{}
	f.Walk(func(r *ir.Region) {
		for i := range r.Stmts {
			s := &r.Stmts[i]
			if s.Mode != ir.MDouble || len(s.Res) != 1 || len(s.Args) != 2 {
				continue
			}
			t := s.Res[0]
			var a *adder
			var in *ir.Stmt
			switch {
			case s.Op == ir.OAdd && okA:
				// (x + y) + c, c + (x + y), or x + c, c + x
				for j := 0; j < 2 && a == nil; j++ {
					l, c := s.Args[j], s.Args[1-j]
					if !bit(c) {
						continue
					}
					if d := inner(l, ir.OAdd, s); d != nil && word(d.Args[0]) && word(d.Args[1]) {
						a, in = &adder{c: c, x: d.Args[0], y: d.Args[1]}, d
					} else if word(l) {
						a = &adder{c: c, x: l, y: -1}
					}
				}
			case s.Op == ir.OSub && okS:
				// (x − y) − b, or x − b
				l, b := s.Args[0], s.Args[1]
				if !bit(b) {
					break
				}
				if d := inner(l, ir.OSub, s); d != nil && word(d.Args[0]) && word(d.Args[1]) {
					a, in = &adder{sub: true, c: b, x: d.Args[0], y: d.Args[1]}, d
				} else if word(l) {
					a = &adder{sub: true, c: b, x: l, y: -1}
				}
			}
			if a == nil || !reads(t, a.sub) {
				continue
			}
			found[t] = a
			if in != nil {
				skip[in] = true
			}
		}
	})
	if len(found) > 0 {
		p.adder, p.absorbed = found, skip
	}
}

// adderStmt prints s if it is a full adder's: the adder itself at its top
// statement, nothing at an inner one, and a projection at each read.
func (p *printer) adderStmt(s *ir.Stmt) bool {
	if p.adder == nil {
		return false
	}
	if p.absorbed[s] {
		return true
	}
	if len(s.Res) == 1 {
		if a := p.adder[s.Res[0]]; a != nil && s.Mode == ir.MDouble {
			name := addCarry
			if a.sub {
				name = subBorrow
			}
			q := p.tg.Prims[name]
			p.imports(q)
			// THE ADDER'S OWN VALUES are the absorbed sums' operands, not only
			// this statement's, so each is read at the word here (ADR 0127)
			c := p.val(a.c)
			if p.pl.Const[p.pl.Res(a.c)] == nil {
				c8 := "%" + p.fresh("ci")
				p.line("%s = trunc %s %s to i8", c8, p.ty(a.c), c)
				c = c8
			}
			y := "0"
			if a.y >= 0 {
				y = p.as(a.y, "i64")
			}
			a.call = "%" + p.fresh("fa")
			p.line("%s = %s", a.call, p.fillOps(q.Form, a.call, []string{"i8 " + c, "i64 " + p.as(a.x, "i64"), "i64 " + y}))
			return true
		}
	}
	for _, x := range s.Args {
		a := p.adder[p.pl.Res(x)]
		if a == nil {
			continue
		}
		switch s.Op {
		case ir.OAnd:
			m := s.Args[0]
			if p.pl.Res(m) == p.pl.Res(x) {
				m = s.Args[1]
			}
			lo := "%" + p.fresh("lo")
			p.line("%s = extractvalue { i8, i64 } %s, 1", lo, a.call)
			p.line("%%v%d = and i64 %s, %s", s.Res[0], lo, p.as(m, "i64"))
		case ir.OShr:
			k := "%" + p.fresh("co")
			p.line("%s = extractvalue { i8, i64 } %s, 0", k, a.call)
			p.line("%%v%d = zext i8 %s to i64", s.Res[0], k)
		case ir.OLt:
			k := "%" + p.fresh("co")
			p.line("%s = extractvalue { i8, i64 } %s, 0", k, a.call)
			p.line("%%v%d = icmp ne i8 %s, 0", s.Res[0], k)
		}
		return true
	}
	return false
}
