package gauntlet

import "testing"

// WHY THE EMITTED SIEVE IS SLOWER THAN HAND-WRITTEN, isolated.
//
// The table sieve measures ~1.4x hand-written Go, and the obvious suspects are
// wrong. These two probes are what separated them, and they are kept because
// the conclusion is a live constraint on the emitter rather than a one-off.
//
//   hand-written          315-345k ns
//   our loop shape only   457-500k ns   <- the cost is HERE
//   emitted, tables       447-484k ns
//
// So the emitted code matches hand-written code written in OUR SHAPE, and the
// gap is the shape itself. Same structure as ADR 0013's finding: the price is
// the shape, not the compiler.

// Aliasing was the first suspect and it is NOT the cost. The table version
// threads the buffer as a loop variable, so the output says `c2 := c`,
// `c3 := c2`, `c2 = c3`. Go coalesces all of it.
func aliasSieve(n int) int {
	c := make([]bool, n)
	c2 := c
	for i := 2; i*i < n; i++ {
		if c2[i] {
			continue
		}
		c3 := c2
		for j := i * i; j < n; j += i {
			c3[j] = true
		}
		c2 = c3
	}
	c4 := c2
	acc := 0
	for k := 2; k < n; k++ {
		if !c4[k] {
			acc++
		}
	}
	return acc
}

// The LOOP SHAPE is the cost. `for { if guard { break }; …; continue }` against
// `for init; cond; post`, hand-written, no tables anywhere. Narrowed further:
// it is the OUTER loop, where our form duplicates the increment into each
// clause and gives the loop several back edges, so Go cannot see a counted loop.
func shapeSieve(n int) int {
	c := make([]bool, n)
	i := 2
	for {
		if i*i >= n {
			break
		}
		if c[i] {
			i = i + 1
			continue
		}
		j := i * i
		for {
			if j < n {
				c[j] = true
				j = j + i
				continue
			}
			break
		}
		i = i + 1
		continue
	}
	acc := 0
	k := 2
	for {
		if k >= n {
			break
		}
		if c[k] {
			k = k + 1
			continue
		}
		acc, k = acc+1, k+1
		continue
	}
	return acc
}

func BenchmarkSieveAlias(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tsSink = aliasSieve(200000)
	}
}

func BenchmarkSieveShape(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tsSink = shapeSieve(200000)
	}
}

func TestSieveProbesAgree(t *testing.T) {
	for _, n := range []int{100, 20000, 200000} {
		if aliasSieve(n) != handSieve(n) || shapeSieve(n) != handSieve(n) {
			t.Errorf("n=%d: probes disagree with the reference", n)
		}
	}
}
