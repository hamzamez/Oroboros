package emit

import (
	"oroboros/core"
)

// A LITERAL TABLE'S ELEMENTS DECIDE ITS ELEMENT TYPE (docs/literal-elements.md).
//
// Until this, `(array 104 105 33)` emitted `[]int` — the type of its FIRST
// element, which is the shape of the `BufferElemBytes` bug elemwidth-2026-08-27
// fixed for buffers, surviving in the safe direction. The consequence was not
// safe: a literal handed to a host call declaring `(array (int 0 255))` was
// refused BY GO — *"cannot use src (variable of type []int) as []byte value"* —
// naming a type nobody wrote.
//
// Two rules, tested separately because they answer different questions.
// SYNTHESIS gives a local literal the hull of its elements; CHECKING lets a
// declaration at a boundary decide, and refuses a literal that does not fit.

// lit builds `(array e…)`.
func lit(vs ...int64) *core.Term {
	kids := []*core.Term{core.Name("array")}
	for _, v := range vs {
		kids = append(kids, core.Int(v))
	}
	return &core.Term{Kind: core.KApp, Kids: kids}
}
