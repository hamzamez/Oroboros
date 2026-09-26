package ir

import (
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A RANGE OBLIGATION IS ON EVERY VALUE ITS ARGUMENT BECAME, and on both ends.
// Two values arise when lowering meets one mark twice (a shared subterm under
// two contexts); the verdict must hold of each, in either order. And an end
// the facts leave open is open: canonical form keeps no number under it.
func TestARangeObligationIsOnEveryValueAndBothEnds(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	mark := &core.Term{Kind: core.KApp, Kids: []*core.Term{core.Name(core.RequireName),
		core.Str("g"), core.Str("x"), core.Str("int 0 9"), core.Name("a")}}
	in, out := fact{v: rangeIV(0, 5)}, fact{v: rangeIV(3, 12)}
	for _, fs := range [][]fact{{in, out}, {out, in}} {
		x, ok := argumentFact(fs, []V{0, 1})
		if !ok {
			t.Fatal("two recorded values are known")
		}
		if proven, _ := decideMark(tg, mark, x); proven {
			t.Errorf("one value in [0, 9] and one reaching 12: proven")
		}
	}
	if x, _ := argumentFact([]fact{in, in}, []V{0, 1}); true {
		if proven, _ := decideMark(tg, mark, x); !proven {
			t.Errorf("both values in [0, 9]: not proven")
		}
	}
	open := iv{lo: ei(0), phi: true}.norm()
	if proven, _ := decideMark(tg, mark, open); proven {
		t.Errorf("[0, +∞) proven in [0, 9]")
	}
	if _, ok := argumentFact([]fact{in}, []V{-1}); ok {
		t.Errorf("a value numbering dropped is known")
	}
}

// AN ALLOCATION'S SIZE IS IN [0, max-len_T] (tables.md §2.3.1), the queued
// wrong answer of irstep3java §3: `(len (build b 4294967297 …))` was 1 on
// Java, which allocates `new T[(int) n]`. Refused there, accepted on Go whose
// max-len is its word, and a size that may be negative is refused on both.
func TestAnAllocationsSizeIsAnObligation(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, body string
		refused    map[string]bool
	}{
		{"past 2^31", "(len (build b 4294967297 b))", map[string]bool{"java": true, "go": false}},
		{"a size that may be negative", "(len (build b (- n 6) b))", map[string]bool{"java": true, "go": true}},
		{"a proven size", "(len (build b (+ n 1) b))", map[string]bool{"java": false, "go": false}},
	}
	for k, c := range cases {
		src := fmt.Sprintf("(export run)\n(sig run ((n (int 0 12))) int)\n(def run (n) %s)\n", c.body)
		path := filepath.Join(dir, fmt.Sprintf("z%d.oro", k))
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		for target, want := range c.refused {
			tg, p, err := compilePath(path, target, Options{Decided: true})
			if err != nil {
				t.Fatalf("%s on %s: %v", c.name, target, err)
			}
			got := Decide(tg, p.Funcs[0], true).SizeRefusal("run", tg) != nil
			if got != want {
				t.Errorf("%s on %s: refused = %v, want %v", c.name, target, got, want)
			}
		}
	}
}

// A PRIMITIVE'S ENSURES BOUNDS ITS RESULT (ADR 0028): for sampled parameters,
// a result the postcondition allows lies in ensuresBound's interval. Each row
// is a postcondition and a function giving the results it allows at x.
func TestAnEnsuresBoundsItsResult(t *testing.T) {
	read := func(src string) *core.Term {
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		return forms[0].Term
	}
	rows := []struct {
		q      string
		allows func(x int64) []int64
	}{
		{"(= result (* 2 n))", func(x int64) []int64 { return []int64{2 * x} }},
		{"(= result (+ (* 3 n) 1))", func(x int64) []int64 { return []int64{3*x + 1} }},
		{"(<= result n)", func(x int64) []int64 { return []int64{x, x - 5, -100} }},
		{"(< result n)", func(x int64) []int64 { return []int64{x - 1, x - 9} }},
		{"(>= result (- 0 n))", func(x int64) []int64 { return []int64{-x, 0, 50} }},
		{"(if (<= 0 result) (<= result (/ n 2)) false)", func(x int64) []int64 { return []int64{0, x / 2} }},
	}
	for _, r := range rows {
		q := read(r.q)
		for _, a := range []iv{rangeIV(0, 10), rangeIV(-4, 7), rangeIV(100, 100)} {
			b := ensuresBound(q, map[string]iv{"n": a})
			for x := a.lo.big().Int64(); x <= a.hi.big().Int64(); x++ {
				for _, y := range r.allows(x) {
					if r.q == "(if (<= 0 result) (<= result (/ n 2)) false)" && (y < 0 || y > x/2) {
						continue
					}
					if !member(big.NewInt(y), b) {
						t.Fatalf("%s at n = %d allows %d, outside %s", r.q, x, y, show(b))
					}
				}
			}
		}
	}
}
