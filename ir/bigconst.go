package ir

import (
	"math/big"
	"strings"

	"oroboros/core"
	"oroboros/emit"
)

// A CONSTANT IS A NULLARY OPERATION OF ℤ (ADR 0157). Its realization on a
// target is a closed term over the host's signature (ADR 0089), and of those
// that denote c the one at the highest layer the host provides is the host's
// own constant: Go's *big.Int made once, the JVM's static BigInteger, a BigInt
// literal, and on a target with no bignum the numeral itself in read-only
// data. The spine, Horner over 10¹⁵ computed where it is read, denotes c too,
// and lowers further than the host requires: rebuilt at every evaluation, it
// made a loop over a constant 2.4× slower on Go and 3.5× on windows.
//
// In the IR a constant held exactly is an OConst whose type is the rung's,
// `big`, its literal the integer (a numeral past int64), read by every printer
// as its host's constant; on windows LowerLimbs makes it a numeral, a table no
// birth makes and no death releases (ir/release.go: not owned).

// ConstValue is a constant literal's integer: a literal, or a numeral past
// int64.
func ConstValue(d *core.Term) (*big.Int, bool) {
	if d == nil {
		return nil, false
	}
	if d.Kind == core.KInt {
		return big.NewInt(d.Int), true
	}
	if d.Kind != core.KApp {
		return nil, false
	}
	return core.EvalEndpoint(d)
}

// intLit is c as a literal: the word's where int64 holds it, else its numeral.
func intLit(c *big.Int) *core.Term {
	if c.IsInt64() {
		return core.Int(c.Int64())
	}
	return core.IntLiteral(c.String())
}

// pointOf is the integer an interval is the one point of.
func pointOf(x iv) (*big.Int, bool) {
	if x.bot || x.nlo || x.phi || x.lo != x.hi {
		return nil, false
	}
	return x.lo.big(), true
}

// IsBigConst says whether v is a constant held exactly: an OConst of the
// rung's type, the host's constant on its printer.
func IsBigConst(tg *emit.Target, f *Func, v V, d *core.Term) bool {
	if d == nil || PlantConstantSpined {
		return false
	}
	t := f.Types[v]
	return t == core.BigType || tg.ValueType(t) == core.BigType
}

// PlantClosedTermsOnLimbs makes each closed term its constant on fixed limbs
// too: big-signed-ops' compile on windows 3.2× (TestAClosedTermOnLimbsIsItsTerm).
var PlantClosedTermsOnLimbs bool

// PlantSharedConstantMoved returns Go's shared *big.Int constant itself from a
// terminator, where its caller may write into it (TestAConstantIsTheHostsOwn).
var PlantSharedConstantMoved bool

// PlantConstantSpined builds a constant past the word from its digits where it
// is read, as before ADR 0157 (TestAConstantIsTheHostsOwn).
var PlantConstantSpined bool

// bigConst is c held exactly, one constant of the rung's type.
func (g *bigs) bigConst(c *big.Int) V {
	v := g.fresh(core.BigType)
	if e, ok := fromBig(c); ok {
		g.fs[v] = fact{v: rangeE(e, e)}
	}
	g.stmts = append(g.stmts, Stmt{Op: OConst, Lit: intLit(c), Res: []V{v}})
	return v
}

// NumeralOf is c's numeral in base 2^bits, K(ℕ*)'s table (ADR 0099): the
// magnitude's digits, low first, then the header 2m + s, m the digit count and
// s 1 for a negative value; no slot between, so its length is m + 1.
func NumeralOf(c *big.Int, bits int) []uint64 {
	mag := new(big.Int).Abs(c)
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
	var out []uint64
	for mag.Sign() > 0 {
		out = append(out, new(big.Int).And(mag, mask).Uint64())
		mag.Rsh(mag, uint(bits))
	}
	h := uint64(2 * len(out))
	if c.Sign() < 0 {
		h++
	}
	return append(out, h)
}

// onlyBorrowed are the values every use of which, if any, is a call of the
// rung's own operations, each a borrow (ir/release.go): a static numeral may be
// read so, and nothing else; one nothing reads prints nothing. A program function's result is a birth its caller owns and
// releases (the frame rule, ADR 0101), so a numeral leaving by a yield, a join
// or a store must be fresh storage, its spine.
func onlyBorrowed(f *Func) map[V]bool {
	other := map[V]bool{}
	var walk func(r *Region)
	walk = func(r *Region) {
		for _, s := range r.Stmts {
			for _, a := range s.Args {
				if !(s.Op == OCall && (strings.HasPrefix(s.Name, "big") || s.Name == "decimal")) {
					other[a] = true
				}
			}
			for _, sub := range s.Sub {
				walk(sub)
			}
		}
		for _, a := range r.Args {
			other[a] = true
		}
		if r.T == TBranch {
			other[r.Cond] = true
			walk(r.Then)
			walk(r.Else)
		}
	}
	walk(f.Body)
	out := map[V]bool{}
	for v := range f.Types {
		if !other[V(v)] {
			out[V(v)] = true // borrowed, or read by nothing
		}
	}
	return out
}

// PlantStaticNumeralEscapes keeps a numeral static where it leaves its
// function: the caller releases read-only data (TestAStaticNumeralIsNeverReleased).
var PlantStaticNumeralEscapes bool

// spineConsts writes each constant held exactly as its spine, `big-of` of
// word digits in base 10¹⁵ with `big*` and `big+`, the value v itself the
// last: on fixed limbs, whose layout is the instance's own, the constant is
// computed by the limb library as before ADR 0157.
func spineConsts(f *Func, keep map[V]bool) {
	fresh := func(ty string) V {
		v := V(len(f.Types))
		f.Types = append(f.Types, ty)
		return v
	}
	base := big.NewInt(1000000000000000)
	var region func(r *Region)
	region = func(r *Region) {
		var out []Stmt
		for _, s := range r.Stmts {
			for _, sub := range s.Sub {
				region(sub)
			}
			c, ok := ConstValue(s.Lit)
			if s.Op != OConst || len(s.Res) != 1 || f.Types[s.Res[0]] != core.BigType || !ok || keep[s.Res[0]] {
				out = append(out, s)
				continue
			}
			var digits []int64
			for v := new(big.Int).Set(c); v.Sign() != 0; {
				q, d := new(big.Int).QuoRem(v, base, new(big.Int))
				digits = append(digits, d.Int64())
				v = q
			}
			if len(digits) == 0 {
				digits = []int64{0}
			}
			konst := func(k int64, res V) V {
				w := fresh("int")
				out = append(out, Stmt{Op: OConst, Lit: core.Int(k), Res: []V{w}})
				if res < 0 {
					res = fresh(core.BigType)
				}
				out = append(out, Stmt{Op: OCall, Name: "big-of", Args: []V{w}, Res: []V{res}})
				return res
			}
			last := len(digits) == 1
			acc := V(-1)
			if last {
				acc = konst(digits[0], s.Res[0])
			} else {
				acc = konst(digits[len(digits)-1], -1)
			}
			for i := len(digits) - 2; i >= 0; i-- {
				p := fresh(core.BigType)
				out = append(out, Stmt{Op: OCall, Name: "big*", Args: []V{acc, konst(base.Int64(), -1)}, Res: []V{p}})
				res := fresh(core.BigType)
				if i == 0 {
					res = s.Res[0]
				}
				out = append(out, Stmt{Op: OCall, Name: "big+", Args: []V{p, konst(digits[i], -1)}, Res: []V{res}})
				acc = res
			}
		}
		r.Stmts = out
		if r.T == TBranch {
			region(r.Then)
			region(r.Else)
		}
	}
	region(f.Body)
}
