package presburger

import (
	"math/rand"
	"testing"
)

func ge(k int64, coef map[string]int64) Constraint { return Constraint{Coef: coef, K: k} }
func eq(k int64, coef map[string]int64) Constraint { return Constraint{Coef: coef, K: k, Eq: true} }

// KNOWN ANSWERS, among them the ones only integrality decides: a system with a
// rational solution and no integer one.
func TestKnownSystems(t *testing.T) {
	type c = map[string]int64
	cases := []struct {
		name string
		cs   []Constraint
		want Result
	}{
		{"x ≥ 1 and x ≤ 0", []Constraint{ge(-1, c{"x": 1}), ge(0, c{"x": -1})}, Unsat},
		{"2x = 1", []Constraint{eq(-1, c{"x": 2})}, Unsat},
		{"3x + 2y = 1", []Constraint{eq(-1, c{"x": 3, "y": 2})}, Sat},
		{"x = 2y, x = 2z + 1: parity", []Constraint{eq(0, c{"x": 1, "y": -2}), eq(-1, c{"x": 1, "z": -2})}, Unsat},
		{"1 ≤ 2x ≤ 1", []Constraint{ge(-1, c{"x": 2}), ge(1, c{"x": -2})}, Unsat},
		{"0 ≤ x, 2x ≤ 1", []Constraint{ge(0, c{"x": 1}), ge(1, c{"x": -2})}, Sat},
		// Pugh 1991's example: 27 ≤ 11x + 13y ≤ 45 and −10 ≤ 7x − 9y ≤ 4 has
		// rational solutions and no integer one.
		{"Pugh's example", []Constraint{
			ge(-27, c{"x": 11, "y": 13}), ge(45, c{"x": -11, "y": -13}),
			ge(10, c{"x": 7, "y": -9}), ge(4, c{"x": -7, "y": 9}),
		}, Unsat},
		{"an unbounded variable", []Constraint{ge(-5, c{"x": 1, "y": 3})}, Sat},
		{"6x + 9y = 7: gcd 3 does not divide 7", []Constraint{eq(-7, c{"x": 6, "y": 9})}, Unsat},
		{"7x + 12y + 31z = 43 with bounds: z = 1, y = 1, x = 0", []Constraint{
			eq(-43, c{"x": 7, "y": 12, "z": 31}),
			ge(0, c{"x": 1}), ge(0, c{"y": 1}), ge(0, c{"z": 1}),
			ge(10, c{"x": -1}), ge(10, c{"y": -1}), ge(10, c{"z": -1}),
		}, Sat},
		{"no constraint", nil, Sat},
	}
	for _, tc := range cases {
		if got := Satisfiable(tc.cs); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

// AGAINST EXHAUSTIVE SEARCH. Random systems over three or four variables, each
// boxed in [−B, B] so that enumerating the box decides them exactly; equalities,
// coefficients up to 7 and constants up to 20, so that the dark shadow, the
// splinters and the mod-hat step are all reached. A fault in any of them gives a
// wrong answer here.
func TestAgainstExhaustiveSearch(t *testing.T) {
	const B = 4
	rng := rand.New(rand.NewSource(1))
	names := []string{"a", "b", "c", "d"}
	sat, unsat := 0, 0
	for n := 0; n < 4000; n++ {
		nv := 3 + rng.Intn(2)
		vars := names[:nv]
		var cs []Constraint
		for _, x := range vars {
			cs = append(cs, ge(B, map[string]int64{x: 1}), ge(B, map[string]int64{x: -1}))
		}
		for k := 0; k < 2+rng.Intn(3); k++ {
			coef := map[string]int64{}
			for _, x := range vars {
				if rng.Intn(3) > 0 {
					coef[x] = int64(rng.Intn(15) - 7)
				}
			}
			c := Constraint{Coef: coef, K: int64(rng.Intn(41) - 20), Eq: rng.Intn(4) == 0}
			cs = append(cs, c)
		}
		want := brute(cs, vars, B)
		got := Satisfiable(cs)
		if got == Unknown {
			t.Fatalf("system %d gave up: %v", n, cs)
		}
		if (got == Sat) != want {
			t.Fatalf("system %d: %v, exhaustive search says sat=%v\n%v", n, got, want, cs)
		}
		if want {
			sat++
		} else {
			unsat++
		}
	}
	if sat < 500 || unsat < 500 {
		t.Fatalf("the random systems are lopsided: %d sat, %d unsat", sat, unsat)
	}
}

func brute(cs []Constraint, vars []string, B int64) bool {
	val := map[string]int64{}
	var try func(i int) bool
	try = func(i int) bool {
		if i == len(vars) {
			for _, c := range cs {
				s := c.K
				for x, a := range c.Coef {
					s += a * val[x]
				}
				if c.Eq && s != 0 || !c.Eq && s < 0 {
					return false
				}
			}
			return true
		}
		for v := -B; v <= B; v++ {
			val[vars[i]] = v
			if try(i + 1) {
				return true
			}
		}
		return false
	}
	return try(0)
}

// AN OVERFLOW IS UNKNOWN, never an answer.
func TestAnOverflowIsUnknown(t *testing.T) {
	big := int64(1) << 62
	cs := []Constraint{
		ge(0, map[string]int64{"x": big, "y": 3}),
		ge(0, map[string]int64{"x": -big, "y": 5}),
		ge(-1, map[string]int64{"x": 3, "y": -big}),
	}
	if got := Satisfiable(cs); got != Unknown && got != Sat && got != Unsat {
		t.Fatalf("got %v", got)
	}
}
