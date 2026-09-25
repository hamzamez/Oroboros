package plan

import (
	"fmt"
	"testing"
)

// TestMovesAreTheParallelMove checks Moves exhaustively on every parallel move
// over n ≤ 4 variables: every choice of sources (a variable or a constant) for
// every subset of destinations. The sequential moves, executed in order, must
// leave every destination with the value the simultaneous assignment gives,
// and use at most one temporary per cycle of the move graph (the algorithm's
// minimality: Rideau, Serpette and Leroy 2008).
func TestMovesAreTheParallelMove(t *testing.T) {
	vars := []string{"a", "b", "c", "d"}
	for n := 1; n <= 4; n++ {
		vs := vars[:n]
		// each destination i takes a source in vs ∪ {"k"} ∪ {"" = not assigned}
		choices := append(append([]string{}, vs...), "k", "")
		total := 1
		for range vs {
			total *= len(choices)
		}
		for code := 0; code < total; code++ {
			var dst, src []string
			c := code
			for _, v := range vs {
				pick := choices[c%len(choices)]
				c /= len(choices)
				if pick != "" {
					dst, src = append(dst, v), append(src, pick)
				}
			}
			tmp := 0
			fresh := func() string { tmp++; return fmt.Sprintf("t%d", tmp) }
			seq := Moves(dst, src, fresh)
			// simulate
			env := map[string]int{"k": 100}
			for i, v := range vs {
				env[v] = i + 1
			}
			want := map[string]int{}
			for i := range dst {
				want[dst[i]] = env[src[i]]
			}
			for _, m := range seq {
				env[m[0]] = env[m[1]]
			}
			for d, w := range want {
				if env[d] != w {
					t.Fatalf("dst %v ← src %v: sequence %v gives %s = %d, want %d", dst, src, seq, d, env[d], w)
				}
			}
			if cyc := cycles(dst, src); tmp > cyc {
				t.Fatalf("dst %v ← src %v: %d temporaries for %d cycles (%v)", dst, src, tmp, cyc, seq)
			}
		}
	}
}

// cycles counts the cycles of length ≥ 2 in a move graph d ← s.
func cycles(dst, src []string) int {
	next := map[string]string{} // d ← s, as an edge s → d reversed: follow sources
	for i := range dst {
		if dst[i] != src[i] {
			next[dst[i]] = src[i]
		}
	}
	seen := map[string]bool{}
	n := 0
	for start := range next {
		if seen[start] {
			continue
		}
		path := map[string]bool{}
		x := start
		for {
			if path[x] {
				n++
				break
			}
			if seen[x] {
				break
			}
			path[x] = true
			y, ok := next[x]
			if !ok {
				break
			}
			x = y
		}
		for k := range path {
			seen[k] = true
		}
	}
	return n
}

// TestMovesOfExpressions: sources that are expressions over the destinations,
// as a printer that inlines produces them. Each sequence is evaluated by a tiny
// interpreter of `name` and `(name + k)` and compared with the simultaneous
// assignment, over every pairing of three variables.
func TestMovesOfExpressions(t *testing.T) {
	vars := []string{"a", "b", "c"}
	forms := []func(string) string{
		func(x string) string { return x },
		func(x string) string { return "(" + x + " + 1)" },
		func(x string) string { return "(" + x + " + 10)" },
	}
	eval := func(e string, env map[string]int) int {
		var x string
		k := 0
		if _, err := fmt.Sscanf(e, "(%s + %d)", &x, &k); err == nil {
			return env[x] + k
		}
		return env[e]
	}
	for s0 := 0; s0 < 3; s0++ {
		for s1 := 0; s1 < 3; s1++ {
			for s2 := 0; s2 < 3; s2++ {
				for f := 0; f < 27; f++ {
					src := []string{forms[f%3](vars[s0]), forms[f/3%3](vars[s1]), forms[f/9](vars[s2])}
					dst := vars
					tmp := 0
					seq := Moves(dst, src, func() string { tmp++; return fmt.Sprintf("t%d", tmp) })
					env := map[string]int{"a": 1, "b": 2, "c": 3}
					want := map[string]int{}
					for i := range dst {
						want[dst[i]] = eval(src[i], env)
					}
					for _, m := range seq {
						env[m[0]] = eval(m[1], env)
					}
					for d, w := range want {
						if env[d] != w {
							t.Fatalf("dst %v ← src %v: %v gives %s = %d, want %d", dst, src, seq, d, env[d], w)
						}
					}
				}
			}
		}
	}
}
