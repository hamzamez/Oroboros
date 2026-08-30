package gauntlet

import "testing"

// COUNT-THEN-BUILD AGAINST APPEND.
//
// growth.md's third measurement, and the one that decides whether a growable
// array is ever needed in the language. The array-language answer to `filter`
// is two passes — count the survivors, allocate exactly, fill — which needs no
// growth and is what Futhark, ISPC and every GPU library do. `append` needs
// growth and is what a person writes.
//
// Three forms, because the middle one is the honest comparison: `append` onto
// a nil slice pays reallocation, `append` onto a preallocated slice does not,
// and only the first is what a growable buffer would have to emulate.

func filterAppend(a []int64) []int64 {
	var out []int64
	for _, x := range a {
		if x&1 == 0 {
			out = append(out, x)
		}
	}
	return out
}

func filterAppendCap(a []int64) []int64 {
	out := make([]int64, 0, len(a))
	for _, x := range a {
		if x&1 == 0 {
			out = append(out, x)
		}
	}
	return out
}

func filterCountBuild(a []int64) []int64 {
	n := 0
	for _, x := range a {
		if x&1 == 0 {
			n++
		}
	}
	out := make([]int64, n)
	k := 0
	for _, x := range a {
		if x&1 == 0 {
			out[k] = x
			k++
		}
	}
	return out
}

func growInput(n int) []int64 {
	out := make([]int64, n)
	x := uint64(12345)
	for i := range out {
		x = x*6364136223846793005 + 1442695040888963407
		out[i] = int64(x >> 3)
	}
	return out
}

func TestFilterFormsAgree(t *testing.T) {
	for _, n := range []int{0, 1, 1000, 1 << 16} {
		a := growInput(n)
		x, y, z := filterAppend(a), filterAppendCap(a), filterCountBuild(a)
		if len(x) != len(y) || len(x) != len(z) {
			t.Fatalf("n=%d: lengths %d %d %d", n, len(x), len(y), len(z))
		}
		for i := range x {
			if x[i] != y[i] || x[i] != z[i] {
				t.Fatalf("n=%d index %d", n, i)
			}
		}
	}
}

func benchFilter(b *testing.B, n int, f func([]int64) []int64) {
	a := growInput(n)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sinkLimbsI = f(a)
	}
}

var sinkLimbsI []int64

func BenchmarkFilterAppend64k(b *testing.B)     { benchFilter(b, 1<<16, filterAppend) }
func BenchmarkFilterAppendCap64k(b *testing.B)  { benchFilter(b, 1<<16, filterAppendCap) }
func BenchmarkFilterCountBuild64k(b *testing.B) { benchFilter(b, 1<<16, filterCountBuild) }

func BenchmarkFilterAppend1k(b *testing.B)     { benchFilter(b, 1000, filterAppend) }
func BenchmarkFilterAppendCap1k(b *testing.B)  { benchFilter(b, 1000, filterAppendCap) }
func BenchmarkFilterCountBuild1k(b *testing.B) { benchFilter(b, 1000, filterCountBuild) }
