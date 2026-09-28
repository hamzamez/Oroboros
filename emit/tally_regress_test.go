package emit

import (
	"testing"

	"oroboros/core"
)

// Two bugs tally.oro found (tally-2026-09-11), each pinned where it lives.

// A BINDER'S TYPE MUST NOT OUTLIVE ITS BODY. The checker keeps types by name, and
// a binder used to leave its entry behind — so a later binder with the same hint
// and a value the checker cannot type (a table read) was checked against the
// stranger's type. Byte programs got away with it because every leaked type was
// `int`; the first program to hand a STRING table read to an impure host call
// was told "s is int, but string is required".
func TestABindersTypeDoesNotOutliveItsBody(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	let := func(v *core.Term, p string, body *core.Term) *core.Term {
		return core.App(core.Name("let"), v, core.Fn([]string{p}, body))
	}
	// (fn (t u) (let (let (u 0) (fn (s) (go.+ s 1)))
	//                (fn (x) (let (t 0) (fn (s) (go/strings.Compare s "a"))))))
	first := let(core.App(core.Name("u"), core.Int(0)), "s",
		core.App(core.Name("go.+"), core.Name("s"), core.Int(1)))
	second := func(v *core.Term) *core.Term {
		return let(v, "s", core.App(core.Name("go/strings.Compare"), core.Name("s"), core.Str("a")))
	}
	ok := core.Fn([]string{"t", "u"}, let(first, "x", second(core.App(core.Name("t"), core.Int(0)))))
	if err := Check(tg, "t", ok); err != nil {
		t.Errorf("a binder's type leaked into a sibling binder of the same name: %v", err)
	}
	// THE CONTROL: the same shape with the second `s` bound to an INT must still
	// be refused, or the test would pass against a checker that checks nothing.
	bad := core.Fn([]string{"t", "u"}, let(first, "x", second(core.Int(5))))
	if err := Check(tg, "t", bad); err == nil {
		t.Errorf("an int passed where a string is required was accepted")
	}
}
