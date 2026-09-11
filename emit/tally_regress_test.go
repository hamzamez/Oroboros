package emit

import (
	"strings"
	"testing"
	"time"

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

// NESTED CONDITIONS ANALYSE IN LINEAR TIME. The interval pass evaluated both
// operands of every comparison once per branch as well as the condition as a
// whole, five evaluations per `if`, and an operand containing an `if` paid that
// again: exponential in how deeply conditions nest inside conditions. tally.oro's
// build did not finish in five minutes. Forty levels is 5^40 under the old rule
// and forty evaluations under the new one.
func TestNestedConditionsAnalyseInLinearTime(t *testing.T) {
	x := "i"
	for k := 0; k < 40; k++ {
		x = "(if (= " + x + " 0) 1 0)"
	}
	nf := reduce(t, "(use go) (fn (i) "+x+")", "go")
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		Intervals(tg, nil, nf, 0)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("the interval pass did not finish on forty nested conditions")
	}
	if !strings.Contains(nf.String(), "(= (if") {
		t.Errorf("the term did not keep its nesting, so it tests nothing: %s", nf)
	}
}
