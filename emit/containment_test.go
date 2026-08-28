package emit

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"oroboros/core"
)

// THE CONTAINMENT HARNESS.
//
// The property, and it is the one everything else now rests on:
//
//	γ-SOUNDNESS.  For every reachable concrete state σ and every integer
//	operation e evaluated at σ,  ⟦e⟧σ ∈ γ(MaxOp).
//
// `MaxOp` is the join of every checkable operation's abstract result, and it is
// what index narrowing trusts to decide how many BITS a value gets
// (indexnarrow-2026-08-27). A claim that is too WIDE costs space; one that is
// too NARROW is a silent wrong answer. So the test is containment, never
// tightness.
//
// WHY RANDOM AND NOT HAND-WRITTEN. `TestIntervalsNeverOverclaim` exists, is
// hand-written, and PASSED for months while the fixpoint was not monotone
// (fixpoint-2026-08-27). Hand-written adversarial cases only catch what someone
// thought to write, and two of the ones written for `BufferRange` the same week
// turned out to expect a refusal where the analysis was right. A generator does
// not have opinions.
//
// It would have caught that bug on the first seed: the fixpoint settled
// `cur[i]` at `[0,0]` for a variable that plainly grows, so `⟦i+1⟧#` was `[1,1]`
// while the program reaches 20.
//
// THE PASS CONDITION, set before the harness was believed: it must FAIL when
// the fixpoint bug is put back. Reverting `restore` to install its snapshot by
// reference makes it fail at seed 15 — the analysis claims every operation is
// in -153..765 and the program produces 918.
//
// The first version of the generator did NOT catch it, and the reason is worth
// keeping: every conditional it produced sat in TAIL position, where the
// environment after an `if` is never used again, so the leak was invisible. The
// generator now puts conditionals in VALUE position too — as an operand, where
// whatever the analysis believes afterwards is immediately spent on the other
// operand. A harness that cannot fail proves nothing, and this one could not
// until that shape was in it.
//
// HOW THE INTERPRETER IS TRUSTED. It is a second implementation of the
// semantics, so it could be wrong on its own. What keeps it honest is that it
// is direct — no abstraction, no fixpoint, no joins, and `arithOp` is shared
// with the analysis so the two cannot drift about WHICH operations are counted.
//
// WHAT IT DOES NOT COVER, and the honest next step: buffers. The element range
// is the other place an interval decides bits, and a wrong one there truncates
// stored data. Generating `build`/`set` and checking every stored value against
// `ElemType`s answer is the same property one level along.

// ---------------------------------------------------------------- generation

type gen struct {
	r    *rand.Rand
	tgt  *Target
	vars []string
}

// program builds a loop in the fragment the analysis actually meets: several
// variables, guards that exit on one of them, and `again` arguments that are
// arithmetic or a conditional over arithmetic.
//
// One variable is always a counter with a guard, so the loop terminates and the
// interpreter has something to observe.
func (g *gen) program() (*core.Term, []string) {
	n := 1 + g.r.Intn(3)
	g.vars = nil
	for i := 0; i < n; i++ {
		g.vars = append(g.vars, fmt.Sprintf("v%d", i))
	}
	inits := make([]*core.Term, n)
	for i := range inits {
		inits[i] = core.Int(int64(g.r.Intn(5)))
	}
	// v0 is the counter: the guard exits on it and every `again` advances it.
	body := g.clauses(0)
	lam := core.Fn(g.vars, body)
	kids := []*core.Term{core.Name("loop"), lam}
	kids = append(kids, inits...)
	return &core.Term{Kind: core.KApp, Kids: kids}, g.vars
}

func (g *gen) clauses(depth int) *core.Term {
	// The exit guard, always on the counter, so the loop is finite.
	limit := 3 + g.r.Intn(20)
	exit := g.expr(0)
	again := make([]*core.Term, len(g.vars))
	again[0] = g.step("v0")
	for i := 1; i < len(g.vars); i++ {
		again[i] = g.update(g.vars[i])
	}
	akids := append([]*core.Term{core.Name("again")}, again...)
	back := &core.Term{Kind: core.KApp, Kids: akids}

	var inner *core.Term = back
	// An optional second clause, so the chain has more than one guard and the
	// refinement has to compose.
	if depth == 0 && g.r.Intn(2) == 0 {
		inner = core.App(core.Name("if"),
			core.App(core.Name("go.<"), core.Name(g.vars[g.r.Intn(len(g.vars))]),
				core.Int(int64(g.r.Intn(4)))),
			g.expr(0), back)
	}
	return core.App(core.Name("if"),
		core.App(core.Name("go.>="), core.Name("v0"), core.Int(int64(limit))),
		exit, inner)
}

// step advances the counter by a positive amount, so the loop makes progress.
func (g *gen) step(v string) *core.Term {
	return core.App(core.Name("go.+"), core.Name(v), core.Int(int64(1+g.r.Intn(3))))
}

// update is any of the shapes an `again` argument takes in real programs: a
// pass-through, an arithmetic step, or a conditional over two of them — which
// is the running-extremum shape (monotone.go).
func (g *gen) update(v string) *core.Term {
	switch g.r.Intn(5) {
	case 0:
		return core.Name(v)
	case 1:
		return g.expr(0)
	case 2:
		return core.App(core.Name("go.*"), core.Name(v), core.Int(int64(g.r.Intn(3))))
	case 3:
		return core.App(core.Name("if"),
			core.App(core.Name("go.>"), core.Name(v), g.expr(0)),
			core.Name(v), g.expr(0))
	}
	return core.App(core.Name("go.-"), core.Name(v), core.Int(int64(g.r.Intn(3))))
}

func (g *gen) expr(depth int) *core.Term {
	if depth > 2 || g.r.Intn(3) == 0 {
		if g.r.Intn(2) == 0 {
			return core.Int(int64(g.r.Intn(9) - 3))
		}
		return core.Name(g.vars[g.r.Intn(len(g.vars))])
	}
	// A CONDITIONAL IN VALUE POSITION, which is the shape that matters and the
	// one the first version of this generator missed entirely.
	//
	// A clause chain puts every  in TAIL position, where the environment
	// after it is never used again — so the restore-by-reference leak was
	// invisible and the harness passed against the bug it was written for. Here
	// the  is an operand: whatever the analysis believes after it is
	// immediately spent on the other operand.
	if g.r.Intn(4) == 0 {
		return core.App(core.Name("go.+"),
			core.App(core.Name("if"), g.pred(), g.expr(depth+1), g.expr(depth+1)),
			g.expr(depth+1))
	}
	op := []string{"go.+", "go.-", "go.*"}[g.r.Intn(3)]
	return core.App(core.Name(op), g.expr(depth+1), g.expr(depth+1))
}

// pred is a comparison on a loop variable, so refining it actually narrows
// something the rest of the expression depends on.
func (g *gen) pred() *core.Term {
	op := []string{"go.<", "go.>=", "go.>", "go.<="}[g.r.Intn(4)]
	return core.App(core.Name(op), core.Name(g.vars[g.r.Intn(len(g.vars))]),
		core.Int(int64(g.r.Intn(8)-2)))
}

// ------------------------------------------------------------- interpretation

type runner struct {
	tgt  *Target
	ops  []int64 // every CHECKABLE operation's result, in evaluation order
	fuel int
}

type againVals struct{ v []int64 }

func (a *againVals) Error() string { return "again" }

// eval runs a term concretely. It is the semantics, written once and directly:
// no widening, no fixpoint, no joins.
func (r *runner) eval(t *core.Term, env map[string]int64) (int64, error) {
	r.fuel--
	if r.fuel <= 0 {
		return 0, fmt.Errorf("fuel")
	}
	switch t.Kind {
	case core.KInt:
		return t.Int, nil
	case core.KName:
		v, ok := env[t.Name]
		if !ok {
			return 0, fmt.Errorf("unbound %s", t.Name)
		}
		return v, nil
	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return 0, fmt.Errorf("higher-order")
		}
		args := t.Args()
		switch op.Name {
		case "if":
			c, err := r.eval(args[0], env)
			if err != nil {
				return 0, err
			}
			if c != 0 {
				return r.eval(args[1], env)
			}
			return r.eval(args[2], env)
		case "again":
			out := &againVals{}
			for _, a := range args {
				v, err := r.eval(a, env)
				if err != nil {
					return 0, err
				}
				out.v = append(out.v, v)
			}
			return 0, out
		case "loop":
			return r.loop(t, env)
		}
		vals := make([]int64, len(args))
		for i, a := range args {
			v, err := r.eval(a, env)
			if err != nil {
				return 0, err
			}
			vals[i] = v
		}
		return r.prim(op.Name, vals)
	}
	return 0, fmt.Errorf("kind %v", t.Kind)
}

func (r *runner) prim(name string, v []int64) (int64, error) {
	switch name {
	case "go.<":
		return b2i(v[0] < v[1]), nil
	case "go.<=":
		return b2i(v[0] <= v[1]), nil
	case "go.>":
		return b2i(v[0] > v[1]), nil
	case "go.>=":
		return b2i(v[0] >= v[1]), nil
	case "=":
		return b2i(v[0] == v[1]), nil
	}
	var out int64
	switch arithOp(name, len(v)) {
	case "add":
		out = v[0] + v[1]
	case "sub":
		out = v[0] - v[1]
	case "mul":
		out = v[0] * v[1]
	case "neg":
		out = -v[0]
	default:
		return 0, fmt.Errorf("prim %s", name)
	}
	// Exactly the operations MaxOp joins — arithOp is the one place that list
	// lives, so the harness cannot drift from what it is checking.
	r.ops = append(r.ops, out)
	return out, nil
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (r *runner) loop(t *core.Term, env map[string]int64) (int64, error) {
	args := t.Args()
	lam := args[0]
	cur := make([]int64, len(args)-1)
	for i, z := range args[1:] {
		v, err := r.eval(z, env)
		if err != nil {
			return 0, err
		}
		cur[i] = v
	}
	// OPENED, not Closed(). A lambda body holds its parameters as KBound — by
	// index, never written in source — so every walker in this package opens it
	// first, and an interpreter is no different.
	body, raw, _ := openFresh(lam, map[string]bool{}, func(x string) string { return x })
	for iter := 0; iter < 10000; iter++ {
		inner := map[string]int64{}
		for k, v := range env {
			inner[k] = v
		}
		for i, n := range raw {
			inner[n] = cur[i]
		}
		out, err := r.eval(body, inner)
		if a, ok := err.(*againVals); ok {
			if len(a.v) != len(cur) {
				return 0, fmt.Errorf("again arity")
			}
			copy(cur, a.v)
			continue
		}
		return out, err
	}
	return 0, fmt.Errorf("did not terminate")
}

// ------------------------------------------------------------------ the check

func TestIntervalContainment(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	checked, skipped := 0, 0
	for seed := int64(1); seed <= 2000; seed++ {
		g := &gen{r: rand.New(rand.NewSource(seed)), tgt: tg}
		term, _ := g.program()

		rep, _ := Intervals(tg, nil, term, 0)

		run := &runner{tgt: tg, fuel: 2000000}
		if _, err := run.eval(term, map[string]int64{}); err != nil {
			skipped++ // did not terminate, or left the fragment
			continue
		}
		if len(run.ops) == 0 {
			skipped++
			continue
		}
		checked++
		for _, v := range run.ops {
			if !holds(rep.MaxOp, v) {
				t.Fatalf("seed %d: the analysis claims every operation is in %s, "+
					"and one produced %d\n%s",
					seed, rep.MaxOpRange(), v, indentTerm(term))
			}
		}
	}
	if checked < 500 {
		t.Fatalf("only %d of 2000 programs were actually checked (%d skipped); "+
			"the generator is producing programs the harness cannot run, so it "+
			"is not testing what it claims", checked, skipped)
	}
	t.Logf("%d programs checked, %d skipped", checked, skipped)
}

// holds is γ: is the concrete value in the abstract one?
func holds(v ival, x int64) bool {
	if !v.loInf && x < v.lo {
		return false
	}
	if !v.hiInf && x > v.hi {
		return false
	}
	return true
}

func indentTerm(t *core.Term) string {
	return "    " + strings.ReplaceAll(t.String(), "\n", "\n    ")
}

// THE HARNESS MUST BE ABLE TO FAIL. A containment test that cannot detect an
// unsound interval proves nothing, so this feeds it one.
func TestContainmentDetectsAnUnsoundClaim(t *testing.T) {
	// A program whose operations reach 20, against a claim that stops at 5.
	narrow := ival{lo: 0, hi: 5}
	for _, v := range []int64{0, 5, 6, 20} {
		want := v <= 5
		if holds(narrow, v) != want {
			t.Fatalf("holds(%v, %d) must be %v", narrow, v, want)
		}
	}
	// And the shape the fixpoint bug produced: a variable pinned at its initial
	// value while the program advances it.
	if holds(ival{lo: 0, hi: 0}, 1) {
		t.Fatal("a claim of [0,0] must not admit 1 — this is the fixpoint bug's " +
			"signature, and the harness exists to see it")
	}
}
