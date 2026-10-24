package emit

import (
	"testing"

	"oroboros/core"
)

// A LENGTH IS KEYED BY WHAT IT DENOTES (lengthSubject): a store keeps a table's
// length, len ∘ set = len ∘ π₁, and len commutes with a conditional whose arms
// agree. So a guarded store before a read whose law names its buffer argument
// keeps the buffer's length (examples/os/wc.oro), and two arms of different
// lengths, or a map's insert, which grows the map, are kept apart.
func TestALengthIsKeyedByWhatItDenotes(t *testing.T) {
	b, c := core.Name("b"), core.Name("c")
	set := func(x *core.Term) *core.Term { return core.App(core.Name("set"), x, core.Int(0), core.Int(1)) }
	cond := func(x, y *core.Term) *core.Term {
		return core.App(core.Name("if"), core.App(core.Name(">"), core.Name("t"), core.Int(0)), x, y)
	}
	claim := func(x *core.Term) *core.Term {
		return core.App(core.Name(core.AscribeName), core.Str("array int 0 255"), x)
	}
	same := map[string]*core.Term{
		"a store":                    set(b),
		"two stores":                 set(set(b)),
		"a guarded store":            cond(set(b), b),
		"a guarded store, nested":    cond(set(cond(set(b), b)), cond(set(b), b)),
		"a claim on a guarded store": claim(cond(set(b), b)),
	}
	for what, x := range same {
		if lengthVar("len", x) != lengthVar("len", b) {
			t.Errorf("%s: %s is not len(b)", what, lengthVar("len", x))
		}
	}
	apart := map[string]*core.Term{
		"arms of two tables":    cond(set(b), c),
		"a map's insert":        core.App(core.Name("insert"), b, core.Int(0), core.Int(1)),
		"a cell's write, 2-ary": core.App(core.Name("set"), b, core.Int(1)),
		"a conditional's guard": cond(core.Int(1), core.Int(2)),
	}
	for what, x := range apart {
		if lengthVar("len", x) == lengthVar("len", b) {
			t.Errorf("%s: %s was taken for len(b)", what, lengthVar("len", x))
		}
	}
}
