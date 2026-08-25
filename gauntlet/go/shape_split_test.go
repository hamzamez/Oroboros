package gauntlet

import "testing"

// Which half of the shape costs: hoisting the CONDITION into `for ; cond;`, or
// hoisting the increment into `for ;; post`? Everything else is identical.

// post hoisted, condition still a break inside the body.
func splitPostSieve(n int) int {
	c := make([]bool, n)
	for i := 2; ; i = i + 1 {
		if i*i >= n {
			break
		}
		if c[i] {
			continue
		}
		for j := i * i; j < n; j += i {
			c[j] = true
		}
	}
	acc := 0
	for k := 2; k < n; k++ {
		if !c[k] {
			acc++
		}
	}
	return acc
}

// condition hoisted, increment still duplicated into each clause.
func splitCondSieve(n int) int {
	c := make([]bool, n)
	for i := 2; i*i < n; {
		if c[i] {
			i = i + 1
			continue
		}
		for j := i * i; j < n; j += i {
			c[j] = true
		}
		i = i + 1
	}
	acc := 0
	for k := 2; k < n; k++ {
		if !c[k] {
			acc++
		}
	}
	return acc
}

func BenchmarkSieveSplitPost(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tsSink = splitPostSieve(200000)
	}
}
func BenchmarkSieveSplitCond(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tsSink = splitCondSieve(200000)
	}
}
func TestSplitProbesAgree(t *testing.T) {
	if splitPostSieve(200000) != handSieve(200000) || splitCondSieve(200000) != handSieve(200000) {
		t.Error("split probes disagree")
	}
}
