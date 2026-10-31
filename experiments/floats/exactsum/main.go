package main

// Exact summation (a large superaccumulator, Neal 2015) against naive
// left-to-right summation: is "the interior of a rounding is exact" affordable?
import (
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"sort"
	"time"
)

type acc struct {
	bin [2048]int64 // bin[e] holds signed mantissas of exponent field e
	cnt [2048]int32
}

func (a *acc) add(x float64) {
	b := math.Float64bits(x)
	e := (b >> 52) & 0x7ff
	m := int64(b & (1<<52 - 1))
	if e != 0 {
		m |= 1 << 52
	} else {
		e = 1
	}
	if b>>63 != 0 {
		m = -m
	}
	a.bin[e] += m
	a.cnt[e]++
	if a.cnt[e] == 1024 { // |bin| < 2^63 after 1024 adds of < 2^53
		a.flush(int(e))
	}
}

func (a *acc) flush(e int) {
	// move the high part up: bin[e] = q*2^? ... keep it simple: carry into bin[e+?]
	// 2^53 at exponent e equals 1 at exponent e+53, and exponent fields step by 1 bit,
	// so carry the bits above 2^40 into bin[e+40] (exact: shift by exponent difference).
	v := a.bin[e]
	hi := v >> 40
	a.bin[e] = v - hi<<40
	if e+40 < 2048 {
		a.bin[e+40] += hi
		a.cnt[e+40]++
		if a.cnt[e+40] >= 1024 {
			a.flush(e + 40)
		}
	} else {
		panic("overflow")
	}
	a.cnt[e] = 0
}

func (a *acc) round() float64 {
	s := new(big.Float).SetPrec(2400)
	t := new(big.Float).SetPrec(2400)
	for e := 2047; e >= 1; e-- {
		if a.bin[e] != 0 {
			t.SetInt64(a.bin[e])
			t.SetMantExp(t, e-1075)
			s.Add(s, t)
		}
	}
	f, _ := s.Float64()
	return f
}

func main() {
	n := 1 << 24
	r := rand.New(rand.NewSource(1))
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = (r.Float64()*2 - 1) * math.Ldexp(1, r.Intn(60)-30)
	}
	best := func(f func() float64) (time.Duration, float64) {
		var b time.Duration = 1 << 62
		var v float64
		for k := 0; k < 7; k++ {
			t := time.Now()
			v = f()
			if d := time.Since(t); d < b {
				b = d
			}
		}
		return b, v
	}
	tn, vn := best(func() float64 {
		s := 0.0
		for _, x := range xs {
			s += x
		}
		return s
	})
	tn4, vn4 := best(func() float64 { // four partial sums, the reordered fold
		var s0, s1, s2, s3 float64
		for i := 0; i < n; i += 4 {
			s0 += xs[i]
			s1 += xs[i+1]
			s2 += xs[i+2]
			s3 += xs[i+3]
		}
		return (s0 + s1) + (s2 + s3)
	})
	te, ve := best(func() float64 {
		a := new(acc)
		for _, x := range xs {
			a.add(x)
		}
		return a.round()
	})
	// the exact sum's rounding is order-independent: check on a shuffled and a sorted copy
	ys := append([]float64(nil), xs...)
	sort.Float64s(ys)
	a := new(acc)
	for _, x := range ys {
		a.add(x)
	}
	vs := a.round()
	zs := append([]float64(nil), xs...)
	r.Shuffle(n, func(i, j int) { zs[i], zs[j] = zs[j], zs[i] })
	b := new(acc)
	for _, x := range zs {
		b.add(x)
	}
	vz := b.round()
	// a reference, by big.Float directly
	ref := new(big.Float).SetPrec(2400)
	for _, x := range xs {
		ref.Add(ref, new(big.Float).SetPrec(2400).SetFloat64(x))
	}
	vr, _ := ref.Float64()
	ns := float64(n)
	fmt.Printf("naive     %.3f ns/elt  %.17g\n", float64(tn.Nanoseconds())/ns, vn)
	fmt.Printf("4 sums    %.3f ns/elt  %.17g\n", float64(tn4.Nanoseconds())/ns, vn4)
	fmt.Printf("exact     %.3f ns/elt  %.17g  (%.2fx naive)\n", float64(te.Nanoseconds())/ns, ve, float64(te)/float64(tn))
	fmt.Printf("exact sorted %.17g shuffled %.17g reference %.17g\n", vs, vz, vr)
	sn := 0.0
	for _, x := range ys {
		sn += x
	}
	fmt.Printf("naive sorted %.17g\n", sn)
}
