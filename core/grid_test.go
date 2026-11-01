package core

import (
	"math"
	"math/big"
	"math/rand"
	"sort"
	"testing"
)

// THE GRID IS ITS DEFINITION (numbers.md §2–§3, grid.go). F(p, emin, emax) is
// defined as { m·2^(e−p+1) : m ∈ −(2ᵖ−1)..2ᵖ−1, e ∈ emin..emax } (floats-
// research §18.1). The rounding grid has no upper bound on e, so the reference
// enumerates e up to emax + 1, which holds both neighbours of every q with
// |q| ≤ 2^(emax+1). Each element's canonical generator, which a tie to even
// reads (§3), is its m at the least e that represents it: the normalized one,
// or the subnormal one at emin. Every mode is checked at every quarter of the
// finest spacing and at rationals no grid holds. Planted fault: the cell's
// exponent one binade too high (plantSpacingFault), which this test sees.
func TestTheGridIsItsDefinition(t *testing.T) {
	g := Grid{Kind: 'f', P: 3, Emin: -2, Emax: 2}
	check := func() []string {
		canon := map[string]*big.Int{} // element → its canonical m
		var elems []*big.Rat
		for e := g.Emax + 1; e >= g.Emin; e-- { // descending e: the least e is written last
			for m := int64(-(1<<g.P - 1)); m <= 1<<g.P-1; m++ {
				x := new(big.Rat).Mul(big.NewRat(m, 1), pow2(e-g.P+1))
				if _, seen := canon[x.RatString()]; !seen {
					elems = append(elems, x)
				}
				canon[x.RatString()] = big.NewInt(m)
			}
		}
		sort.Slice(elems, func(i, j int) bool { return elems[i].Cmp(elems[j]) < 0 })
		var queries []*big.Rat
		step := pow2(g.Emin - g.P + 1 - 2)
		top := pow2(g.Emax + 1)
		for q := new(big.Rat).Neg(top); q.Cmp(top) <= 0; q = new(big.Rat).Add(q, step) {
			queries = append(queries, q)
		}
		for _, q := range []*big.Rat{big.NewRat(1, 3), big.NewRat(-5, 7), big.NewRat(22, 7), big.NewRat(1, 1000), big.NewRat(-7, 1)} {
			queries = append(queries, q)
		}
		var bad []string
		for _, q := range queries {
			i := sort.Search(len(elems), func(i int) bool { return elems[i].Cmp(q) > 0 })
			down, up := elems[i-1], elems[i-1]
			if down.Cmp(q) != 0 {
				up = elems[i]
			}
			nonneg := q.Sign() >= 0
			want := map[string]*big.Rat{"down": down, "up": up}
			if nonneg {
				want["zero"], want["away"] = down, up
			} else {
				want["zero"], want["away"] = up, down
			}
			dd := new(big.Rat).Sub(q, down)
			du := new(big.Rat).Sub(up, q)
			switch dd.Cmp(du) {
			case -1:
				want["nearest"], want["half-away"] = down, down
			case 1:
				want["nearest"], want["half-away"] = up, up
			default:
				if canon[down.RatString()].Bit(0) == 0 {
					want["nearest"] = down
				} else {
					want["nearest"] = up
				}
				want["half-away"] = want["away"]
			}
			for mode, w := range want {
				if got := g.Round(q, mode); got.Cmp(w) != 0 {
					bad = append(bad, mode+" "+q.RatString()+": "+got.RatString()+" want "+w.RatString())
				}
			}
			in := down.Cmp(q) == 0
			if g.Contains(q) != in {
				bad = append(bad, "membership of "+q.RatString())
			}
		}
		if g.Max().Cmp(big.NewRat(7, 1)) != 0 { // (2 − 2⁻²)·2² = 7
			bad = append(bad, "max "+g.Max().RatString())
		}
		return bad
	}
	if bad := check(); len(bad) > 0 {
		t.Fatalf("%d disagreements with the definition, first %v", len(bad), bad[:min(5, len(bad))])
	}
	plantSpacingFault = true
	defer func() { plantSpacingFault = false }()
	if bad := check(); len(bad) == 0 {
		t.Errorf("the planted fault (the binade above) went unseen")
	}
}

// F64 = F(53, −1022, 1023) IS THE HOST'S BINARY64 (IEEE 754): the procedure's
// nearest rounding equals Go's correctly rounded conversion, its floor and
// ceiling the doubles either side, at ties, below the normal range, at the
// largest finite value, where the nearest rounding overflows, and at random
// rationals. Two implementations, one written from the cell's algebra and one
// the host's, agree.
func TestF64IsBinary64(t *testing.T) {
	g := GridF64
	half := pow2(-53)
	sub := pow2(-1074)
	m := g.Max()
	qs := []*big.Rat{
		new(big.Rat).Add(big.NewRat(1, 1), half),                                            // a tie, to 1
		new(big.Rat).Add(big.NewRat(1, 1), new(big.Rat).Mul(big.NewRat(3, 1), half)),        // a tie, to 1 + 2⁻⁵¹
		new(big.Rat).Quo(sub, big.NewRat(2, 1)),                                             // half the least subnormal: to 0
		new(big.Rat).Mul(sub, big.NewRat(3, 2)),                                             // a subnormal tie, to 2·2⁻¹⁰⁷⁴
		pow2(-1022), new(big.Rat).Sub(pow2(-1022), new(big.Rat).Quo(sub, big.NewRat(2, 1))), // the normal range's bottom
		m, new(big.Rat).Add(m, pow2(969)), new(big.Rat).Neg(new(big.Rat).Add(m, pow2(969))), // the largest, and below the overflow tie
		big.NewRat(1, 10), big.NewRat(-1, 3), big.NewRat(2, 3),
	}
	r := rand.New(rand.NewSource(1))
	for i := 0; i < 2000; i++ {
		num := new(big.Int).Rand(r, new(big.Int).Lsh(big.NewInt(1), uint(1+r.Intn(200))))
		den := new(big.Int).Rand(r, new(big.Int).Lsh(big.NewInt(1), uint(1+r.Intn(200))))
		if den.Sign() == 0 {
			continue
		}
		q := new(big.Rat).SetFrac(num, den)
		if r.Intn(2) == 0 {
			q.Neg(q)
		}
		qs = append(qs, q)
	}
	for _, q := range qs {
		host, _ := q.Float64()
		got := g.Round(q, "nearest")
		if math.IsInf(host, 0) {
			if g.InRange(got) {
				t.Errorf("nearest %s: the host overflows and the procedure gives %s", q.RatString(), got.RatString())
			}
			continue
		}
		if got.Cmp(new(big.Rat).SetFloat64(host)) != 0 {
			t.Errorf("nearest %s: %s, the host %v", q.RatString(), got.RatString(), host)
		}
		lo, hi := host, host
		if new(big.Rat).SetFloat64(host).Cmp(q) > 0 {
			lo = math.Nextafter(host, math.Inf(-1))
		}
		if new(big.Rat).SetFloat64(host).Cmp(q) < 0 {
			hi = math.Nextafter(host, math.Inf(1))
		}
		if d := g.Round(q, "down"); !math.IsInf(lo, 0) && d.Cmp(new(big.Rat).SetFloat64(lo)) != 0 {
			t.Errorf("down %s: %s, the host %v", q.RatString(), d.RatString(), lo)
		}
		if u := g.Round(q, "up"); !math.IsInf(hi, 0) && u.Cmp(new(big.Rat).SetFloat64(hi)) != 0 {
			t.Errorf("up %s: %s, the host %v", q.RatString(), u.RatString(), hi)
		}
	}
	if f, _ := m.Float64(); f != math.MaxFloat64 {
		t.Errorf("F64's max is %s, not the host's", m.RatString())
	}
}

// A SEPARATION OF A GRID BY BOUNDS HAS ONE NORMAL FORM, its ends the grid's by
// the Galois connection: two spellings of one set read as one type, strict
// bounds included, and any other formula stays a separation.
func TestASeparationOfAGridHasOneNormalForm(t *testing.T) {
	sigOf := func(set string) *Sig {
		forms, err := ReadEntry("export fn f(x ∈ "+set+") (Bool) { true }\n", "t.oro")
		if err != nil {
			t.Fatalf("%s: %v", set, err)
		}
		prog, _, err := Load(forms)
		if err != nil {
			t.Fatalf("%s: %v", set, err)
		}
		for _, sig := range prog.Sigs {
			return sig
		}
		t.Fatalf("%s: no signature", set)
		return nil
	}
	read := func(set string) string { return sigOf(set).Params[0].Type }
	same := [][2]string{
		{`set { y ∈ F64 : y <= 1e100 }`, // ⌊10¹⁰⁰⌋ onto F64, the double just below it
			`set { y ∈ F64 : y <= 9999999999999998216360018871870109548898901740426374747374488505608317520357971321909184780648316928 }`},
		{`set { y ∈ F64 : -1e400 <= y }`, `F64`},
		{`set { y ∈ Int : 0 <= y && y < 10 }`, `0..9`},
		{`set { y ∈ Int : 0.5 <= y && y <= 9.5 }`, `1..9`},
		{`set { y ∈ 0..100 : y > 50 }`, `51..100`},
		{`set { y ∈ Int : y >= 0 }`, `Nat`},
		{`set { y ∈ F64 : 0 < y && y < 1 }`, `set { y ∈ F64 : 1e-400 <= y && y <= 0.99999999999999999 }`},
	}
	for _, c := range same {
		if a, b := read(c[0]), read(c[1]); a != b {
			t.Errorf("one set, two forms:\n %s\n %s", a, b)
		}
	}
	// Any other formula stays a separation: the base set, and the formula the
	// `where`'s share (decl.go); a grid's normal form leaves no `where`.
	if sig := sigOf(`set { y ∈ F64 : y * y <= 2 }`); sig.Where == nil {
		t.Errorf("a separation by another formula lost its formula: %s", sig.Params[0].Type)
	}
	if sig := sigOf(`set { y ∈ F64 : y <= 1 }`); sig.Where != nil {
		t.Errorf("a grid's normal form kept a formula: %v", sig.Where)
	}
}
