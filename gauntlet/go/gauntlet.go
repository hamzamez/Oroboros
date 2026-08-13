// Package gauntlet holds the hand-written reference implementations that any
// candidate core must reach parity with. These are the numbers to beat.
//
// Where a derivation made a claim about what Go's compiler does, this file
// carries BOTH forms so the claim can be measured rather than assumed:
//
//	G1  DotNaive vs DotHoisted        — does ys[:len(xs)] clear the bounds check?
//	G2  Centroid vs CentroidStructAcc — does Go scalarize a struct accumulator?
//	G3  SumF64 vs SumF64Generic       — what does a func-value parameter cost?
//	G4  WordCountIncr vs ...ReadWrite — is m[k]++ one hash lookup or two?
//	G6  BuildOps, MakeScaler          — are non-capturing closures static?
package gauntlet

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------- G1: dot product

// DotNaive cannot prove len(ys) >= len(xs), so ys[i] should stay bounds-checked.
func DotNaive(xs, ys []float64) float64 {
	acc := 0.0
	for i := 0; i < len(xs); i++ {
		acc += xs[i] * ys[i]
	}
	return acc
}

// DotHoisted is the idiom the g1 derivation claims range types would emit.
func DotHoisted(xs, ys []float64) float64 {
	ys = ys[:len(xs)]
	acc := 0.0
	for i := 0; i < len(xs); i++ {
		acc += xs[i] * ys[i]
	}
	return acc
}

// DotRange is the most idiomatic form, for comparison.
func DotRange(xs, ys []float64) float64 {
	acc := 0.0
	for i, x := range xs {
		acc += x * ys[i]
	}
	return acc
}

// ---------------------------------------------------------------- G2: structs

type Point struct{ X, Y float64 }

type BBox struct{ Lo, Hi Point }

// Centroid is the hand-written reference: two scalar accumulators.
func Centroid(ps []Point) Point {
	n := len(ps)
	accX, accY := 0.0, 0.0
	for i := 0; i < n; i++ {
		accX += ps[i].X
		accY += ps[i].Y
	}
	return Point{accX / float64(n), accY / float64(n)}
}

// CentroidStructAcc is shaped like the Oroboros source before SROA: it
// constructs a Point on every iteration. If Go scalarizes this itself, we do
// not need to; if it does not, SROA is our job.
func CentroidStructAcc(ps []Point) Point {
	acc := Point{0, 0}
	for i := 0; i < len(ps); i++ {
		acc = Point{acc.X + ps[i].X, acc.Y + ps[i].Y}
	}
	n := float64(len(ps))
	return Point{acc.X / n, acc.Y / n}
}

func Bounds(ps []Point) BBox {
	lo, hi := ps[0], ps[0]
	for i := 1; i < len(ps); i++ {
		p := ps[i]
		if p.X < lo.X {
			lo.X = p.X
		}
		if p.Y < lo.Y {
			lo.Y = p.Y
		}
		if p.X > hi.X {
			hi.X = p.X
		}
		if p.Y > hi.Y {
			hi.Y = p.Y
		}
	}
	return BBox{lo, hi}
}

// ---------------------------------------------------------------- G3: generics

// Monomorphic references — the bar.

func SumF64(xs []float64) float64 {
	acc := 0.0
	for i := 0; i < len(xs); i++ {
		acc += xs[i]
	}
	return acc
}

func CountPositive(xs []int32) int32 {
	acc := int32(0)
	for i := 0; i < len(xs); i++ {
		if xs[i] > 0 {
			acc++
		}
	}
	return acc
}

// Fold is what "emit at the highest layer the target natively provides" would
// produce: Go generics plus a function-valued parameter.
func Fold[T any, A any](xs []T, init A, step func(A, T) A) A {
	acc := init
	for i := 0; i < len(xs); i++ {
		acc = step(acc, xs[i])
	}
	return acc
}

func SumF64Generic(xs []float64) float64 {
	return Fold(xs, 0.0, func(a, x float64) float64 { return a + x })
}

func CountPositiveGeneric(xs []int32) int32 {
	return Fold(xs, int32(0), func(n int32, x int32) int32 {
		if x > 0 {
			return n + 1
		}
		return n
	})
}

// ---------------------------------------------------------------- G4: word count

// WordCountIncr is the reference. The g4 derivation claims m[w]++ compiles to a
// single mapassign returning a value pointer.
func WordCountIncr(text string) map[string]int {
	counts := make(map[string]int)
	for _, w := range strings.Fields(text) {
		counts[w]++
	}
	return counts
}

// WordCountReadWrite is what a naive get-then-set lowering would emit.
func WordCountReadWrite(text string) map[string]int {
	counts := make(map[string]int)
	for _, w := range strings.Fields(text) {
		counts[w] = counts[w] + 1
	}
	return counts
}

// WordCountGetOr models a dict-get-or / dict-set capability pair explicitly.
func WordCountGetOr(text string) map[string]int {
	counts := make(map[string]int)
	for _, w := range strings.Fields(text) {
		v, ok := counts[w]
		if !ok {
			v = 0
		}
		counts[w] = v + 1
	}
	return counts
}

// ---------------------------------------------------------------- G5: output

func Report(w io.Writer, label string, xs []float64) {
	fmt.Fprintln(w, label)
	fmt.Fprintln(w, len(xs))
	fmt.Fprintln(w, DotHoisted(xs, xs))
}

// ReportFast avoids fmt's interface boxing and reflection entirely.
func ReportFast(w io.Writer, label string, xs []float64) {
	buf := make([]byte, 0, 64)
	buf = append(buf, label...)
	buf = append(buf, '\n')
	buf = strconv.AppendInt(buf, int64(len(xs)), 10)
	buf = append(buf, '\n')
	buf = strconv.AppendFloat(buf, DotHoisted(xs, xs), 'g', -1, 64)
	buf = append(buf, '\n')
	w.Write(buf)
}

// ---------------------------------------------------------------- G6: closures

// BuildOps returns closures that capture nothing.
func BuildOps() []func(int32) int32 {
	return []func(int32) int32{
		func(v int32) int32 { return v + 1 },
		func(v int32) int32 { return v * 2 },
		func(v int32) int32 { return -v },
	}
}

func RunOp(ops []func(int32) int32, k int, x int32) int32 {
	return ops[k](x)
}

// MakeScaler returns a closure that captures f, so the environment must escape.
func MakeScaler(f int32) func(int32) int32 {
	return func(v int32) int32 { return v * f }
}
