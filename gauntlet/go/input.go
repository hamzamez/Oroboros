package gauntlet

import (
	"math/rand"
	"strings"
)

// Deterministic inputs. Every language's harness must produce the same values
// from the same parameters, so the numbers are comparable across targets.

const (
	NVec    = 1 << 16 // 65536 elements
	NPoints = 1 << 16
	NWords  = 1 << 16
)

func MakeVec(n int, seed int64) []float64 {
	r := rand.New(rand.NewSource(seed))
	xs := make([]float64, n)
	for i := range xs {
		xs[i] = r.Float64()*2 - 1
	}
	return xs
}

func MakeInts(n int, seed int64) []int32 {
	r := rand.New(rand.NewSource(seed))
	xs := make([]int32, n)
	for i := range xs {
		xs[i] = r.Int31n(200) - 100
	}
	return xs
}

func MakePoints(n int, seed int64) []Point {
	r := rand.New(rand.NewSource(seed))
	ps := make([]Point, n)
	for i := range ps {
		ps[i] = Point{r.Float64()*2 - 1, r.Float64()*2 - 1}
	}
	return ps
}

// A small fixed vocabulary keeps the map's key count stable and realistic:
// roughly 500 distinct words over NWords tokens.
func MakeText(n int, seed int64) string {
	r := rand.New(rand.NewSource(seed))
	vocab := make([]string, 500)
	for i := range vocab {
		b := make([]byte, 3+r.Intn(6))
		for j := range b {
			b[j] = byte('a' + r.Intn(26))
		}
		vocab[i] = string(b)
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteString(vocab[r.Intn(len(vocab))])
	}
	return sb.String()
}
