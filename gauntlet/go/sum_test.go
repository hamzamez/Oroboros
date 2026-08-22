package gauntlet

import (
	"errors"
	"testing"
)

// A sum crossing a boundary, measured three ways. The question is not only
// whether the emitted form reaches hand-written parity — it is what the
// IDIOMATIC Go alternative costs, because Go's answer to a sum is the `error`
// interface and that is a different data structure with a different price.

// 1. Hand-written in the same shape the compiler emits: a tag and a payload.
func handSumStep(a, b int) (int, int) {
	if b == 0 {
		return 1, 1
	}
	if a%b > 0 {
		return 1, 2
	}
	return 0, a / b
}

// 2. Idiomatic Go: `(int, error)` with sentinel errors. Sentinels are the
// CHEAP idiomatic form — no allocation per call — so this is the fair one.
var errZero = errors.New("divide by zero")
var errRem = errors.New("not divisible")

func handErrStep(a, b int) (int, error) {
	if b == 0 {
		return 0, errZero
	}
	if a%b > 0 {
		return 0, errRem
	}
	return a / b, nil
}

// 3. Idiomatic Go with a CONSTRUCTED error, which is what a real program does
// as soon as the message carries any detail.
type stepErr struct{ code int }

func (e *stepErr) Error() string { return "step failed" }

func handErrAllocStep(a, b int) (int, error) {
	if b == 0 {
		return 0, &stepErr{1}
	}
	if a%b > 0 {
		return 0, &stepErr{2}
	}
	return a / b, nil
}

var sumSink int

func BenchmarkSumHand(b *testing.B) {
	for i := 0; i < b.N; i++ {
		acc := 0
		for a := 0; a < 64; a++ {
			tag, v := handSumStep(a, a%5)
			if tag == 0 {
				acc += v
			} else {
				acc -= v
			}
		}
		sumSink = acc
	}
}

func BenchmarkSumGen(b *testing.B) {
	for i := 0; i < b.N; i++ {
		acc := 0
		for a := 0; a < 64; a++ {
			tag, v := SumStep(a, a%5)
			if tag == 0 {
				acc += v
			} else {
				acc -= v
			}
		}
		sumSink = acc
	}
}

func BenchmarkSumGoErrSentinel(b *testing.B) {
	for i := 0; i < b.N; i++ {
		acc := 0
		for a := 0; a < 64; a++ {
			v, err := handErrStep(a, a%5)
			if err == nil {
				acc += v
			} else if err == errZero {
				acc--
			} else {
				acc -= 2
			}
		}
		sumSink = acc
	}
}

func BenchmarkSumGoErrAlloc(b *testing.B) {
	for i := 0; i < b.N; i++ {
		acc := 0
		for a := 0; a < 64; a++ {
			v, err := handErrAllocStep(a, a%5)
			if err == nil {
				acc += v
			} else {
				acc -= err.(*stepErr).code
			}
		}
		sumSink = acc
	}
}

// The emitted code must agree with the hand-written form everywhere.
func TestSumStepAgrees(t *testing.T) {
	for a := -200; a <= 200; a++ {
		for b := -7; b <= 7; b++ {
			g0, g1 := SumStep(a, b)
			h0, h1 := handSumStep(a, b)
			if g0 != h0 || g1 != h1 {
				t.Fatalf("a=%d b=%d: emitted (%d,%d), hand-written (%d,%d)", a, b, g0, g1, h0, h1)
			}
		}
	}
}
