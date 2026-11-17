package emit

import (
	"math"
	"math/big"
	"strings"
	"testing"
)

// THE BASE BLOCK IS ITS BASE (ADR 0124): K(ℕ*)'s library reads B only through
// its block, so each block must be the one function of dbits it says: dm =
// B − 1, bq·4 = B, bh1 = B/2 − 1, kb = 10^kd a digit below 2⁶², the largest
// such power, and kdd = ⌊dbits·log₁₀ 2⌋ + 1, the decimal digits a digit may
// hold. Read off the library as loadNatLib builds it for each triple.
// THE PLANTED FAULT is a 32-bit block with β = 10¹⁸, past its digit, which
// the test must report.
func TestTheBaseBlockIsItsBase(t *testing.T) {
	check := func(name, src string) []string {
		var bad []string
		val := map[string]*big.Int{}
		for _, line := range strings.Split(src, "\n") {
			k, v, ok := strings.Cut(line, " = ")
			if !ok {
				continue
			}
			switch k {
			case "dbits", "dm", "bq", "bh1", "kb", "kd", "kdd":
				if _, seen := val[k]; !seen {
					n, ok := new(big.Int).SetString(strings.TrimSpace(v), 10)
					if !ok {
						bad = append(bad, k+" is not a literal")
						continue
					}
					val[k] = n
				}
			}
		}
		for _, k := range []string{"dbits", "dm", "bq", "bh1", "kb", "kd", "kdd"} {
			if val[k] == nil {
				return []string{name + ": no " + k}
			}
		}
		w := val["dbits"].Int64()
		b := new(big.Int).Lsh(big.NewInt(1), uint(w))
		one := big.NewInt(1)
		if new(big.Int).Sub(b, one).Cmp(val["dm"]) != 0 {
			bad = append(bad, "dm is not B − 1")
		}
		if new(big.Int).Mul(val["bq"], big.NewInt(4)).Cmp(b) != 0 {
			bad = append(bad, "bq·4 is not B")
		}
		if new(big.Int).Sub(new(big.Int).Rsh(b, 1), one).Cmp(val["bh1"]) != 0 {
			bad = append(bad, "bh1 is not B/2 − 1")
		}
		kd := val["kd"].Int64()
		beta := new(big.Int).Exp(big.NewInt(10), big.NewInt(kd), nil)
		lim := new(big.Int).Lsh(one, 62)
		if b.Cmp(lim) < 0 {
			lim = b
		}
		next := new(big.Int).Mul(beta, big.NewInt(10))
		if beta.Cmp(val["kb"]) != 0 || beta.Cmp(lim) >= 0 || next.Cmp(lim) < 0 {
			bad = append(bad, "kb is not 10^kd, the largest power of 10 below min(B, 2^62)")
		}
		if want := int64(math.Floor(float64(w)*math.Log10(2))) + 1; val["kdd"].Int64() != want {
			bad = append(bad, "kdd is not ⌊dbits·log₁₀ 2⌋ + 1")
		}
		for i := range bad {
			bad[i] = name + ": " + bad[i]
		}
		return bad
	}
	for _, triple := range []string{"x86_64-pc-windows-msvc", "i686-pc-windows-msvc"} {
		src, err := natSource(&Target{Name: triple, Triple: triple})
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range check(triple, src) {
			t.Error(b)
		}
	}
	planted := strings.Replace(natNumSrc, natBase64, natBase32, 1)
	if bad := check("planted", planted); len(bad) == 0 {
		t.Error("a 32-bit block with β = 10^18 should be reported, so the test reaches its rule")
	}
}
