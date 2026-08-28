package emit

import (
	"os"
	"testing"

	"oroboros/core"
)

// THE NODE TABLE'S INFERRED RANGE MUST CONTAIN EVERY NODE INDEX.
//
// examples/json/tree.oro stores indices up to 511 into its node table, and the
// element width is chosen from that range — so a range that excludes them is
// not a slower program, it is a truncated link and a wrong answer.
//
// This passes today and is here because it BROKE, on 2026-08-27, from a change
// that touched nothing about buffers: teaching size change to see through a
// `let` proved the parse loop terminating, which enabled `tripCount` for the
// first time, which surfaced a latent fault in the interval fixpoint
// (fixpoint-2026-08-27.md). The table narrowed to `int 0 5` and windows
// returned 4030140 where the other three returned 4040171.
//
// So this is a tripwire for the fixpoint, placed where the damage shows.
func TestNodeTableRangeHoldsEveryIndex(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile("../examples/json/tree.oro")
	if err != nil {
		t.Skip("tree.oro not present")
	}
	forms, err := core.Read(string(src))
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.LoadWith(forms, nil)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["parse"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	var walk func(x *core.Term, d int)
	walk = func(x *core.Term, d int) {
		if x == nil || found {
			return
		}
		if x.Kind == core.KApp && x.Op().Kind == core.KName && len(x.Args()) == 2 {
			if p, ok := tg.Prims[x.Op().Name]; ok && p.Kind == "table-build" {
				if r, got := BufferRange(tg, x.Args()[1]); got {
					lo, hi, _ := core.IntRange(r)
					if lo > 0 || hi < 511 {
						t.Errorf("node table range %q excludes an index it stores; "+
							"0..511 must be contained, or a link truncates", r)
					}
					found = true
					return
				}
			}
		}
		for _, k := range x.Kids {
			walk(k, d+1)
		}
	}
	walk(nf, 0)
	if !found {
		t.Skip("no build with an inferred range — the shape has changed")
	}
}
