package ir

import (
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
