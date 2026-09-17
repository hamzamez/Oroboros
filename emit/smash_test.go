package emit

import (
	"math/rand"
	"testing"

	"oroboros/core"
)

// γ-SOUNDNESS OF ARRAY SMASHING (smash.go), checked against execution.
//
// The containment harness's buffers store values but never FEED a read of the
// buffer being filled into counted arithmetic, which is exactly the case
// smashing newly bounds — so it could not fail against a smashing bug. These
// programs do:
//
//	(build N (fn (p) [(build N (fn (q)]
//	  (loop ((x p) [(y q)] (i 0) (a z))
//	    (if (>= i LIM) EXIT
//	        (again (set x IDX VAL) [y] (+ i 1) UPD))) … ))
//
// with VAL, UPD and EXIT built from reads `(x j)`, `(y j)` at in-domain indices,
// literals, `i` and `a`. In the two-buffer shape the `again` SWAPS them, so each
// variable's cell receives the other's, which is the joint induction.
//
// THE PROPERTY is the harness's: every counted operation's concrete result lies
// in γ(MaxOp). And it has an ANTI-VACUITY guard — refusing is always sound, so
// the test fails unless many programs get a bounded MaxOp that is only reachable
// through a smashed read.
type smashGen struct {
	r    *rand.Rand
	lim  int64
	bufs []string
	hasR bool // a clause-chain `let` binds r to a read of x
}

func (g *smashGen) index() *core.Term {
	switch g.r.Intn(3) {
	case 0:
		return core.Name("i")
	case 1:
		return core.App(core.Name("go.+"), core.Name("i"), core.Int(int64(g.r.Intn(8))))
	}
	return core.Int(g.r.Int63n(g.lim))
}

// stored is a value to store, and one time in three an INCREMENT — a slot read
// plus a literal, the shape the bounded-increment theorem counts rather than
// joins, and a count table's whole shape.
func (g *smashGen) stored() *core.Term {
	if g.r.Intn(3) == 0 {
		return core.App(core.Name("go.+"), g.read(), core.Int(int64(g.r.Intn(7)-2)))
	}
	if g.hasR && g.r.Intn(3) == 0 {
		// AN INCREMENT THROUGH A NAME: r is bound to a read, so r + d is one.
		return core.App(core.Name("go.+"), core.Name("r"), core.Int(int64(g.r.Intn(5)-1)))
	}
	return g.value(0)
}

// innerIndex is an index for a nested loop over m < lim.
func (g *smashGen) innerIndex() *core.Term {
	if g.r.Intn(2) == 0 {
		return core.Name("m")
	}
	return core.Int(g.r.Int63n(g.lim))
}

func (g *smashGen) read() *core.Term {
	return core.App(core.Name(g.bufs[g.r.Intn(len(g.bufs))]), g.index())
}

// value is a stored or computed value. The shapes are chosen so that some cells
// settle at a finite fixpoint only through the loop — `50 - (x j)` over a zero
// fill is [0, 50] after two steps — and some grow and must widen.
func (g *smashGen) value(depth int) *core.Term {
	if depth > 2 {
		if g.r.Intn(2) == 0 {
			return core.Int(int64(g.r.Intn(120) - 20))
		}
		return g.read()
	}
	switch g.r.Intn(9) {
	case 0:
		return core.Int(int64(g.r.Intn(300) - 50))
	case 1:
		return g.read()
	case 2:
		return core.App(core.Name("go.-"), core.Int(int64(g.r.Intn(100))), g.value(depth+1))
	case 3:
		return core.App(core.Name("go.+"), g.value(depth+1), g.value(depth+1))
	case 4:
		return core.App(core.Name("go.%"), g.value(depth+1), core.Int(int64(1+g.r.Intn(200))))
	case 5:
		return core.App(core.Name("if"),
			core.App(core.Name("go.<"), core.Name("i"), core.Int(int64(g.r.Intn(10)))),
			g.value(depth+1), g.value(depth+1))
	case 6:
		return core.App(core.Name("go.*"), core.Name("i"), core.Int(int64(g.r.Intn(9))))
	case 7:
		return core.App(core.Name("go.*"), g.value(depth+1), core.Int(int64(g.r.Intn(3))))
	}
	if g.hasR && g.r.Intn(2) == 0 {
		return core.Name("r")
	}
	return core.Name([]string{"i", "a"}[g.r.Intn(2)])
}

// valueOrScalar is a value with no read when no buffer is live.
func (g *smashGen) valueOrScalar() *core.Term {
	if len(g.bufs) == 0 {
		return core.App(core.Name("go.+"), core.Name("a"), core.App(core.Name("go.*"), core.Name("i"), core.Int(2)))
	}
	return g.value(0)
}

func (g *smashGen) program() *core.Term {
	g.lim = int64(3 + g.r.Intn(14))
	size := g.lim + 8
	swap := g.r.Intn(3) == 0
	g.bufs = []string{"x"}
	params := []string{"x", "i", "a"}
	if swap {
		g.bufs = []string{"x", "y"}
		params = []string{"x", "y", "i", "a"}
	}
	// LINEARITY (ADR 0018): the store consumes x, so a value computed after it —
	// the second store's, and every other `again` argument — may read only the
	// buffers still live. CheckLinear refuses the rest, and a runner that
	// mutates in place would observe the store.
	all := g.bufs
	// A `let` IN THE CLAUSE CHAIN, binding a read before the back edge — the
	// shape tree.oro's walk has, where the depth read out of the worklist is
	// stored back plus one and summed into an accumulator.
	g.hasR = g.r.Intn(2) == 0
	rIndex := g.index()
	store := core.App(core.Name("set"), core.Name("x"), g.index(), g.stored())
	switch g.r.Intn(6) {
	case 3:
		// A TABLE FROM OUTSIDE THE GROUP handed to x's position: a fresh buffer
		// holding one stored value. x itself is dropped, not consumed.
		store = core.App(core.Name("build"), core.Int(g.lim+8),
			core.Fn([]string{"w"}, core.App(core.Name("set"), core.Name("w"), g.index(), g.value(0))))
	case 2:
		// A NESTED LOOP THREADING x, which is a merge pass: it consumes x, so it
		// may read only z (its own version) and the other buffer, and it stores
		// both fresh values and copies. Its variable's delta is its own fixpoint.
		g.bufs = append([]string{"z"}, all[1:]...)
		inner := core.App(core.Name("again"),
			core.App(core.Name("set"), core.Name("z"), g.innerIndex(), g.stored()),
			core.App(core.Name("go.+"), core.Name("m"), core.Int(1)))
		stop := core.App(core.Name("go.>="), core.Name("m"), core.Int(1+g.r.Int63n(g.lim)))
		store = core.App(core.Name("loop"),
			core.Fn([]string{"z", "m"}, core.App(core.Name("if"), stop, core.Name("z"), inner)),
			store, core.Int(0))
	case 0:
		// A TABLE-VALUED CONDITIONAL: each branch consumes x once, and the cell
		// of the whole is the join of the two.
		store = core.App(core.Name("if"),
			core.App(core.Name("go.<"), core.Name("i"), core.Int(int64(g.r.Intn(10)))),
			store, core.App(core.Name("set"), core.Name("x"), g.index(), g.value(0)))
	case 1:
		// A TABLE BOUND BY `let`: the binder carries the value's cell.
		// z is the store's new version, so it may be read — and an increment of a
		// slot of z is a SECOND increment along this back edge.
		g.bufs = append([]string{"z"}, all[1:]...)
		v := g.stored()
		store = core.App(core.Name("let"), store,
			core.Fn([]string{"z"}, core.App(core.Name("set"), core.Name("z"), g.index(), v)))
	}
	g.bufs = all[1:]
	if g.r.Intn(3) == 0 && len(g.bufs) > 0 {
		store = core.App(core.Name("set"), store, g.index(), g.value(0))
	} else if g.r.Intn(3) == 0 {
		store = core.App(core.Name("set"), store, g.index(), core.Int(int64(g.r.Intn(90))))
	}
	upd := core.Name("a")
	if len(g.bufs) > 0 || g.r.Intn(2) == 0 {
		upd = g.valueOrScalar()
	}
	if g.hasR && g.r.Intn(2) == 0 {
		upd = core.App(core.Name("go.+"), core.Name("a"), core.App(core.Name("go.*"), core.Name("r"), core.Int(int64(g.r.Intn(4)))))
	}
	g.bufs = all
	var back *core.Term
	if swap {
		// x's new version is y's old one, and y's is the store into x's.
		back = core.App(core.Name("again"), core.Name("y"), store,
			core.App(core.Name("go.+"), core.Name("i"), core.Int(1)), upd)
	} else {
		back = core.App(core.Name("again"), store,
			core.App(core.Name("go.+"), core.Name("i"), core.Int(1)), upd)
	}
	if g.hasR {
		back = core.App(core.Name("let"), core.App(core.Name("x"), rIndex), core.Fn([]string{"r"}, back))
		g.hasR = false
	}
	var exit *core.Term
	switch g.r.Intn(3) {
	case 0:
		exit = core.Name("a")
	case 1:
		exit = core.App(core.Name("go.+"), g.read(), g.read())
	default:
		exit = core.App(core.Name("go.-"), core.Name("a"), g.read())
	}
	body := core.App(core.Name("if"),
		core.App(core.Name("go.>="), core.Name("i"), core.Int(g.lim)), exit, back)
	inits := []*core.Term{core.Name("p")}
	if swap {
		inits = append(inits, core.Name("q"))
	}
	inits = append(inits, core.Int(0), core.Int(int64(g.r.Intn(5))))
	loop := &core.Term{Kind: core.KApp, Kids: append([]*core.Term{core.Name("loop"), core.Fn(params, body)}, inits...)}
	inner := loop
	if swap {
		inner = core.App(core.Name("build"), core.Int(size), core.Fn([]string{"q"}, loop))
	}
	whole := core.App(core.Name("build"), core.Int(size), core.Fn([]string{"p"}, inner))
	if g.r.Intn(3) == 0 {
		// A BUILD USED AS A NUMBER: its value is its body's.
		whole = core.App(core.Name("go.*"), whole, core.Int(int64(1+g.r.Intn(5))))
	}
	return whole
}

func TestArraySmashingContains(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	checked, bounded, gained, skipped := 0, 0, 0, 0
	for seed := int64(1); seed <= 3000; seed++ {
		g := &smashGen{r: rand.New(rand.NewSource(seed))}
		term := g.program()
		rep, _ := Intervals(tg, nil, term, 0)
		run := &runner{tgt: tg, fuel: 2000000}
		if _, err := run.eval(term, map[string]bval{}); err != nil {
			skipped++
			continue
		}
		if len(run.ops) == 0 {
			skipped++
			continue
		}
		checked++
		if rep.MaxOp.bounded() {
			bounded++
			// The same program with smashing off: a bound only the cell gives.
			if off, _ := intervals(tg, nil, term, 0, nil, false, false, false, false, true); !off.MaxOp.bounded() {
				gained++
			}
		}
		for _, v := range run.ops {
			if !holds(rep.MaxOp, v) {
				t.Fatalf("seed %d: the analysis claims every operation is in %s, and one produced %d\n%s",
					seed, rep.MaxOpRange(), v, indentTerm(term))
			}
		}
	}
	if checked < 1500 {
		t.Fatalf("only %d of 3000 smashing programs ran (%d skipped)", checked, skipped)
	}
	// ANTI-VACUITY, measured rather than assumed: count the programs whose MaxOp
	// is bounded WITH smashing and unbounded with it switched off. Only those
	// test a computed cell; a program whose counted operations never touch a
	// read is bounded either way and proves nothing here.
	if gained < 300 {
		t.Fatalf("only %d of %d programs are bounded only through a cell; smashing is not reaching "+
			"the reads, so this test is passing on refusals", gained, checked)
	}
	t.Logf("%d programs checked, %d with a bounded MaxOp, %d of them only through smashing; %d skipped",
		checked, bounded, gained, skipped)
}

// THE BOUNDED-INCREMENT THEOREM on the two shapes whose count is TIGHT, which
// random programs do not press: an analysis that evaluates a store more than once
// over-counts, and an over-count is sound, so the random harness passes with the
// count ignored.
//
//	double: two increments of one slot along one back edge, so s = 2 is needed —
//	        slot 0 reaches 20 in 10 iterations, and T·1·1 does not contain it.
//	nested: an increment inside a nested loop of 200 iterations, whose body the
//	        analysis evaluates a handful of times — no count per outer iteration
//	        exists, so the cell must not be bounded by one.
func TestBoundedIncrementsContain(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	n := core.Name
	app := core.App
	i := core.Int
	inc := func(buf string) *core.Term { return app(n("go.+"), app(n(buf), i(0)), i(1)) }
	exit := app(n("go.+"), app(n("x"), i(0)), i(1))
	double := app(n("build"), i(16), core.Fn([]string{"p"},
		app(n("loop"), core.Fn([]string{"x", "k"},
			app(n("if"), app(n("go.>="), n("k"), i(10)), exit,
				app(n("again"),
					app(n("let"), app(n("set"), n("x"), i(0), inc("x")),
						core.Fn([]string{"z"}, app(n("set"), n("z"), i(0), inc("z")))),
					app(n("go.+"), n("k"), i(1))))),
			n("p"), i(0))))
	nested := app(n("build"), i(16), core.Fn([]string{"p"},
		app(n("loop"), core.Fn([]string{"x", "k"},
			app(n("if"), app(n("go.>="), n("k"), i(3)), exit,
				app(n("again"),
					app(n("loop"), core.Fn([]string{"z", "m"},
						app(n("if"), app(n("go.>="), n("m"), i(200)), n("z"),
							app(n("again"), app(n("set"), n("z"), i(0), inc("z")), app(n("go.+"), n("m"), i(1))))),
						n("x"), i(0)),
					app(n("go.+"), n("k"), i(1))))),
			n("p"), i(0))))
	for name, term := range map[string]*core.Term{"double": double, "nested": nested} {
		rep, _ := Intervals(tg, nil, term, 0)
		run := &runner{tgt: tg, fuel: 2000000}
		if _, err := run.eval(term, map[string]bval{}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, v := range run.ops {
			if !holds(rep.MaxOp, v) {
				t.Errorf("%s: the analysis claims every operation is in %s, and one produced %d",
					name, rep.MaxOpRange(), v)
			}
		}
		if name == "double" && !rep.MaxOp.bounded() {
			t.Errorf("double: the increments are bounded by T·s·Δ and must be proven, got %s", rep.MaxOpRange())
		}
	}
}
