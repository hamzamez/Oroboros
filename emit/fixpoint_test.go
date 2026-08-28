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
		// SOUND, and it is the answer as of the fixpoint fix. `BufferRange`
		// runs on the `build` lambda ALONE, where `src` is free, so the stores
		// cannot be bounded and it declines. It used to answer `int 0 512`, and
		// it did so from a fixpoint that had settled `i` and `sp` at their
		// initial values — a sound range reached unsoundly.
		t.Skip("the analysis declines to bound the node table, which is correct " +
			"in the lambda alone; this test guards the answer if it starts giving one")
	}
}

// A SNAPSHOT MUST SURVIVE BEING RESTORED TWICE.
//
// The abstract step is
//
//	F(c⃗) = z⃗ ⊔ ⨆{ ⟦a⃗⟧#(R_branch(c⃗)) : each `again` }
//
// and it is MONOTONE only if each branch really is evaluated in R_branch(c⃗)
// rather than in R applied to something already narrowed. Monotonicity is what
// makes the widening sequence converge to a post-fixpoint and what makes
// narrowing's `within(next, cur)` test legitimate.
//
// `restore` installed the snapshot BY REFERENCE, and what follows a restore is
// `refine`, which narrows in place — so the second restore undid nothing and
// the environment leaving an `if` carried `¬c`. Measured consequence on
// examples/json/tree.oro: [0,0] ⊑ [0,2] and yet F([0,0])[i] = [0,2] while
// F([0,2])[i] = [0,0]. Non-monotone, so the narrowing phase accepted a value
// that is not an over-approximation and `i` settled at its initial value.
func TestSnapshotSurvivesTwoRestores(t *testing.T) {
	p := &intervalPass{env: map[string]ival{"x": top, "y": exact(3)}}
	saved := p.snapshot()

	p.restore(saved)
	p.env["x"] = exact(1) // what `refine` does to the then-branch

	p.restore(saved)
	p.env["x"] = exact(2) // and to the else-branch

	p.restore(saved)
	if got := p.env["x"]; got != top {
		t.Errorf("x = %v after restore, want the snapshot's %v — a branch's "+
			"refinement leaked into the environment that outlives the branch", got, top)
	}
	if got := p.env["y"]; got != exact(3) {
		t.Errorf("y = %v, want 3", got)
	}
}

// A LENGTH THE TERM DETERMINES is exact, and reduction is what makes it matter:
// call-by-need let-binds an argument used more than once, so a literal document
// passed to a tokeniser arrives as `(let (array …) (fn (src) … (len src) …))`.
func TestExactLen(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		src  string
		want int64
		ok   bool
	}{
		{"(array 1 2 3 4 5)", 5, true},
		{"(array)", 0, true},
		{"(build 32 (fn (b) b))", 32, true},
		{"(set (set (build 8 (fn (b) b)) 0 1) 1 2)", 8, true}, // a store hands it back
		{"(alloc (table 16 (fn (i) i)))", 16, true},
		{"n", 0, false},                    // a name says nothing
		{"(build n (fn (b) b))", 0, false}, // nor a computed length
	} {
		forms, err := core.Read(c.src)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		got, ok := exactLen(tg, forms[0].Term)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("exactLen(%s) = %d,%v want %d,%v", c.src, got, ok, c.want, c.ok)
		}
	}
}

// Every spelling of "how long is this" must key alike, or two mentions of one
// quantity are two variables and nothing composes. `len` is tables.md's
// structural name and was unrecognised: only the RETIRED portable layer's
// `alen`/`slen` were.
func TestIsLenOp(t *testing.T) {
	for _, n := range []string{"len", "go.len", "vec.alen", "alen", "str.slen"} {
		if !isLenOp(n) {
			t.Errorf("%q must be recognised as a length", n)
		}
	}
	for _, n := range []string{"length", "go.+", "table"} {
		if isLenOp(n) {
			t.Errorf("%q must not be", n)
		}
	}
}
