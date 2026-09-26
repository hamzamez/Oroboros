package ir

import (
	"math"
	"math/big"
	"math/rand"
	"os"
	"testing"

	"oroboros/emit"
)

// THE SOUNDNESS TABLE OF THE INTERVAL DOMAIN (spec §7): one row per abstract
// operation, each checked against the concrete operation on ℤ. Local soundness
// is the property that makes the whole analysis sound by structural induction
// (Cousot and Cousot 1977):
//
//	for every x ∈ γ(a), y ∈ γ(b):   f(x, y) ∈ γ(f#(a, b))
//
// The concrete side is computed in big.Int, so a result past int64 is a true
// integer the abstract interval must contain (by an infinite end), never a
// wrapped one. The check is exhaustive over every interval inside [−6, 6],
// closed and half-open, at the int64 boundaries, and random elsewhere.

// member reports x ∈ γ(a).
func member(x *big.Int, a iv) bool {
	if a.bot {
		return false
	}
	if !a.nlo && x.Cmp(a.lo.big()) < 0 {
		return false
	}
	if !a.phi && x.Cmp(a.hi.big()) > 0 {
		return false
	}
	return true
}

// samples are concrete members of a: its ends, a few inside, 0 and ±1 when
// they are members, and far values on an infinite side (past every end, 2¹³⁰).
func samples(a iv) []*big.Int {
	if a.bot {
		return nil
	}
	var out []*big.Int
	add := func(x *big.Int) {
		if member(x, a) {
			out = append(out, x)
		}
	}
	far := new(big.Int).Lsh(big.NewInt(1), 130)
	if a.nlo {
		add(new(big.Int).Neg(far))
		add(big.NewInt(math.MinInt64))
		add(big.NewInt(-1 << 40))
		add(big.NewInt(-7))
	}
	if a.phi {
		add(far)
		add(big.NewInt(math.MaxInt64))
		add(big.NewInt(1 << 40))
		add(big.NewInt(7))
	}
	if !a.nlo {
		for k := int64(0); k < 20; k++ {
			add(new(big.Int).Add(a.lo.big(), big.NewInt(k)))
		}
	}
	if !a.phi {
		for k := int64(0); k < 3; k++ {
			add(new(big.Int).Sub(a.hi.big(), big.NewInt(k)))
		}
	}
	for _, x := range []int64{0, 1, -1} {
		add(big.NewInt(x))
	}
	return out
}

// domains: every closed interval inside [−6, 6], each with its half-open
// variants, the boundary intervals of int64, of U and of E, and ⊤.
func domains() []iv {
	var out []iv
	for lo := int64(-6); lo <= 6; lo++ {
		for hi := lo; hi <= 6; hi++ {
			out = append(out, rangeIV(lo, hi))
		}
		out = append(out, iv{lo: ei(lo), phi: true}, iv{hi: ei(lo), nlo: true})
	}
	e := func(x *big.Int) ep {
		v, ok := fromBig(x)
		if !ok {
			panic(x)
		}
		return v
	}
	p := func(k uint) *big.Int { return new(big.Int).Lsh(big.NewInt(1), k) }
	d := func(x *big.Int, k int64) *big.Int { return new(big.Int).Add(x, big.NewInt(k)) }
	out = append(out, ivTop,
		rangeIV(math.MaxInt64-3, math.MaxInt64), rangeIV(math.MinInt64, math.MinInt64+3),
		rangeIV(-1<<62, 1<<62), rangeIV(1<<32, 1<<33), rangeIV(-(1<<33), -(1<<32)),
		rangeE(two63, u64Max), rangeE(e(d(p(64), -3)), u64Max), ivU,
		rangeE(e(d(p(126), -4)), eMax), rangeE(eMax.neg(), e(d(new(big.Int).Neg(p(126)), 4))),
		rangeE(ep{}, e(p(100))), iv{lo: two64, phi: true}, iv{hi: two63.neg(), nlo: true})
	return out
}

type binRow struct {
	name string
	abs  func(a, b iv) iv
	con  func(x, y *big.Int) (*big.Int, bool) // false: undefined (division by zero)
}

var binRows = []binRow{
	{"add", addIV, func(x, y *big.Int) (*big.Int, bool) { return new(big.Int).Add(x, y), true }},
	{"sub", subIV, func(x, y *big.Int) (*big.Int, bool) { return new(big.Int).Sub(x, y), true }},
	{"mul", mulIV, func(x, y *big.Int) (*big.Int, bool) { return new(big.Int).Mul(x, y), true }},
	{"div", divIV, func(x, y *big.Int) (*big.Int, bool) {
		if y.Sign() == 0 {
			return nil, false
		}
		return new(big.Int).Quo(x, y), true // truncating, integers.md §3
	}},
	{"rem", remIV, func(x, y *big.Int) (*big.Int, bool) {
		if y.Sign() == 0 {
			return nil, false
		}
		return new(big.Int).Rem(x, y), true // the dividend's sign
	}},
}

func checkBin(t *testing.T, r binRow, a, b iv) {
	out := r.abs(a, b)
	for _, bx := range samples(a) {
		for _, by := range samples(b) {
			x, y := bx, by
			c, ok := r.con(bx, by)
			if !ok {
				continue
			}
			if !member(c, out) {
				t.Fatalf("%s: %d ∈ %s, %d ∈ %s gives %s, not in %s", r.name, x, show(a), y, show(b), c, show(out))
			}
		}
	}
}

// maskShiftRows are the mask and the shift SelectShifts writes: two's
// complement AND, and the right shift on a non-negative operand (where the
// logical and the arithmetic shift agree), by a count in [0, 62]. Outside that
// the transfer claims nothing, and the row checks only what it claims.
var maskShiftRows = []binRow{
	{"and", andIV, func(x, y *big.Int) (*big.Int, bool) { return new(big.Int).And(x, y), true }}, // big's And is two's complement
	{"shr", shrIV, func(x, k *big.Int) (*big.Int, bool) {
		if x.Sign() < 0 || !k.IsInt64() || k.Int64() < 0 || k.Int64() > 62 {
			return nil, false
		}
		return new(big.Int).Rsh(x, uint(k.Int64())), true
	}},
}

// TestTheMaskAndTheShiftAreSound: andIV and shrIV against ℤ's operations,
// exhaustively on the test domains and on shift counts up to 62.
func TestTheMaskAndTheShiftAreSound(t *testing.T) {
	ds := domains()
	ks := []iv{rangeIV(0, 0), rangeIV(1, 1), rangeIV(0, 3), rangeIV(5, 62), rangeIV(62, 62), rangeIV(2, 6)}
	for _, a := range ds {
		for _, b := range ds {
			checkBin(t, maskShiftRows[0], a, b)
		}
		for _, k := range append(ks, ds...) {
			checkBin(t, maskShiftRows[1], a, k)
		}
	}
}

func TestIntervalSoundnessExhaustive(t *testing.T) {
	ds := domains()
	for _, r := range binRows {
		for _, a := range ds {
			for _, b := range ds {
				checkBin(t, r, a, b)
			}
		}
	}
	for _, a := range ds {
		out := negIV(a)
		for _, x := range samples(a) {
			c := new(big.Int).Neg(x)
			if !member(c, out) {
				t.Fatalf("neg: %d ∈ %s gives %s, not in %s", x, show(a), c, show(out))
			}
		}
	}
}

func TestIntervalSoundnessRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	pick := func() iv {
		x, y := rng.Int63n(1<<20)-1<<19, rng.Int63n(1<<20)-1<<19
		if rng.Intn(4) == 0 {
			x, y = rng.Int63()-rng.Int63(), rng.Int63()-rng.Int63()
		}
		if x > y {
			x, y = y, x
		}
		return rangeIV(x, y)
	}
	for i := 0; i < 4000; i++ {
		a, b := pick(), pick()
		for _, r := range binRows {
			checkBin(t, r, a, b)
		}
	}
}

// TestNarrowIsSound: a π's fact (Theorem E). For x ∈ a and o ∈ b with
// `x rel o`, x ∈ narrow(a, rel, b).
func TestNarrowIsSound(t *testing.T) {
	rels := map[string]func(c int) bool{
		"lt": func(c int) bool { return c < 0 }, "le": func(c int) bool { return c <= 0 },
		"gt": func(c int) bool { return c > 0 }, "ge": func(c int) bool { return c >= 0 },
		"eq": func(c int) bool { return c == 0 }, "ne": func(c int) bool { return c != 0 },
	}
	ds := domains()
	for rel, holds := range rels {
		for _, a := range ds {
			for _, b := range ds {
				out := narrowIV(a, rel, b)
				for _, x := range samples(a) {
					for _, o := range samples(b) {
						if holds(x.Cmp(o)) && !member(x, out) {
							t.Fatalf("narrow %s: %d ∈ %s with %d ∈ %s holds, but %d ∉ %s", rel, x, show(a), o, show(b), x, show(out))
						}
					}
				}
			}
		}
	}
}

// TestLatticeLaws: join contains both, meet contains what both contain, and
// widening contains both (it is an upper bound).
func TestLatticeLaws(t *testing.T) {
	ds := domains()
	for _, a := range ds {
		for _, b := range ds {
			j, m, w := joinIV(a, b), meetIV(a, b), widenIV(a, b)
			for _, x := range append(samples(a), samples(b)...) {
				bx := x
				inA, inB := member(bx, a), member(bx, b)
				if (inA || inB) && !member(bx, j) {
					t.Fatalf("join: %d ∈ %s ∪ %s but ∉ %s", x, show(a), show(b), show(j))
				}
				if inA && inB && !member(bx, m) {
					t.Fatalf("meet: %d ∈ %s ∩ %s but ∉ %s", x, show(a), show(b), show(m))
				}
				if (inA || inB) && !member(bx, w) {
					t.Fatalf("widen: %d ∈ %s ∪ %s but ∉ %s", x, show(a), show(b), show(w))
				}
			}
		}
	}
}

// TestThresholdWideningIsAWidening: widenIVT's two laws, exhaustively over the
// test domains and a threshold set with gaps. (1) It contains both arguments.
// (2) An ascending chain of it stabilises: repeatedly widening against a
// growing sequence changes each end at most |T| + 1 times.
func TestThresholdWideningIsAWidening(t *testing.T) {
	th := []ep{ei(-5), ei(-1), ei(0), ei(2), ei(3), ei(6), ei(math.MaxInt64), u64Max}
	ds := domains()
	for _, a := range ds {
		for _, b := range ds {
			w := widenIVT(a, b, th)
			for _, x := range append(samples(a), samples(b)...) {
				bx := x
				if (member(bx, a) || member(bx, b)) && !member(bx, w) {
					t.Fatalf("widen with thresholds: %d ∈ %s ∪ %s but ∉ %s", x, show(a), show(b), show(w))
				}
			}
		}
	}
	// A counter growing by one forever: the chain must stabilise.
	cur := rangeIV(0, 0)
	for step := 0; ; step++ {
		next := widenIVT(cur, joinIV(cur, rangeIV(0, int64(step)+1)), th)
		if next == cur {
			break
		}
		if step > len(th)+2 {
			t.Fatalf("the chain did not stabilise within |T| + 2 steps: %s", show(cur))
		}
		cur = next
	}
}

// TestAnAssumptionReachesThroughAPi: a conjunction's second conjunct reads its
// operand through the first's π, and what `assume` narrows there it narrows in
// the source, since the two names denote one value. count-primes' `where`,
// (and (<= 0 n) (< n 1048576)), must leave n in [0, 1048575]; without the rule
// it stayed [0, +∞] and none of the function's eight operations was proven.
func TestAnAssumptionReachesThroughAPi(t *testing.T) {
	src, err := os.ReadFile("testdata/assume-pi.ir")
	if err != nil {
		t.Fatal(err)
	}
	p, err := Read(string(src))
	if err != nil {
		t.Fatal(err)
	}
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range p.Funcs {
		if f.Name != "gen-count-primes" {
			continue
		}
		n := analyse(tg, f)[f.Params[0]].v
		if n != rangeIV(0, 1048575) {
			t.Fatalf("n is %s after its where, want [0, 1048575]", show(n))
		}
		return
	}
	t.Fatal("gen-count-primes is not in the test data")
}

// TestTheBackwardRulesAreSound: each inverse the backward propagation uses,
// checked against the forward operation on ℤ. For every x and every z the
// operation lands in, x lies in what the inverse returns (Benhamou et al.'s
// HC4-revise is sound exactly when each of these is).
func TestTheBackwardRulesAreSound(t *testing.T) {
	ds := domains()
	for _, z := range ds {
		for x := int64(-40); x <= 40; x++ {
			if member(big.NewInt(x*x), z) {
				r, ok := invSquare(z)
				if ok && !member(big.NewInt(x), r) {
					t.Fatalf("invSquare: %d² = %d ∈ %s but %d ∉ %s", x, x*x, show(z), x, show(r))
				}
			}
			for _, c := range []int64{-7, -3, -2, -1, 1, 2, 3, 10} {
				if member(big.NewInt(x*c), z) {
					r, ok := invMulConst(z, c)
					if ok && !member(big.NewInt(x), r) {
						t.Fatalf("invMulConst: %d·%d ∈ %s but %d ∉ %s", x, c, show(z), x, show(r))
					}
				}
			}
		}
	}
	for _, n := range []int64{0, 1, 2, 3, 4, 15, 16, 17, 19999, 1<<62 - 1, math.MaxInt64} {
		r := isqrt(n)
		if r < 0 || new(big.Int).Mul(big.NewInt(r), big.NewInt(r)).Cmp(big.NewInt(n)) > 0 ||
			new(big.Int).Mul(big.NewInt(r+1), big.NewInt(r+1)).Cmp(big.NewInt(n)) <= 0 {
			t.Fatalf("isqrt(%d) = %d", n, r)
		}
	}
}

// TestTheUnsignedTransfersAreTheResidueMap: each u64 transfer against r, the
// residue map ℤ → ℤ/2⁶⁴ with its representatives in U = [0, 2⁶⁴) or in
// S = [−2⁶³, 2⁶³), over every sample of every domain, U's values past int64
// included (an infinite top end holds them).
func TestTheUnsignedTransfersAreTheResidueMap(t *testing.T) {
	two64 := new(big.Int).Lsh(big.NewInt(1), 64)
	two63 := new(big.Int).Lsh(big.NewInt(1), 63)
	rU := func(x *big.Int) *big.Int { return new(big.Int).Mod(x, two64) }
	rS := func(x *big.Int) *big.Int {
		y := rU(x)
		if y.Cmp(two63) >= 0 {
			y.Sub(y, two64)
		}
		return y
	}
	inU := func(x *big.Int) bool { return x.Sign() >= 0 && x.Cmp(two64) < 0 }
	wide := func(a iv) []*big.Int {
		var out []*big.Int
		out = append(out, samples(a)...)
		for _, x := range []*big.Int{new(big.Int).Set(two63), new(big.Int).Sub(two64, big.NewInt(1)), new(big.Int).Add(two63, big.NewInt(12345))} {
			if member(x, a) {
				out = append(out, x)
			}
		}
		return out
	}
	ds := domains()
	for _, a := range ds {
		for _, x := range samples(a) {
			bx := x
			if y := rU(bx); !member(y, u64Of(a)) {
				t.Fatalf("u64-of %d = %s ∉ %s (x ∈ %s)", x, y, show(u64Of(a)), show(a))
			}
		}
		for _, x := range wide(a) {
			if inU(x) && !member(rS(x), intOfU64(a)) {
				t.Fatalf("int-of-u64 %s = %s ∉ %s (x ∈ %s)", x, rS(x), show(intOfU64(a)), show(a))
			}
		}
		for _, b := range ds {
			for _, op := range []string{"u64+", "u64-", "u64*", "u64/", "u64%"} {
				out := u64Arith(op, a, b)
				for _, x := range wide(a) {
					for _, y := range wide(b) {
						if !inU(x) || !inU(y) {
							continue
						}
						var z *big.Int
						switch op {
						case "u64+":
							z = rU(new(big.Int).Add(x, y))
						case "u64-":
							z = rU(new(big.Int).Sub(x, y))
						case "u64*":
							z = rU(new(big.Int).Mul(x, y))
						case "u64/":
							if y.Sign() == 0 {
								continue
							}
							z = new(big.Int).Quo(x, y)
						default:
							if y.Sign() == 0 {
								continue
							}
							z = new(big.Int).Rem(x, y)
						}
						if !member(z, out) {
							t.Fatalf("%s %s %s = %s ∉ %s (%s, %s)", op, x, y, z, show(out), show(a), show(b))
						}
					}
				}
			}
		}
	}
}
