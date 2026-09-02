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
// Two properties, one machine. Both are the same shape — a concrete value must
// lie in the abstract one the compiler committed to — and both decide BITS, so
// a claim that is too narrow is a silent wrong answer rather than a slow
// program.
//
//	γ-SOUNDNESS (operations).  For every reachable concrete state σ and every
//	integer operation e evaluated at σ,  ⟦e⟧σ ∈ γ(MaxOp).
//
//	γ-SOUNDNESS (buffers).  For every `build` buffer b and every value v that
//	can be READ OUT of b,  v ∈ γ(ElemType(b)).
//
// `MaxOp` is what index narrowing trusts to hold a counter in the host's own
// `int` (indexnarrow-2026-08-27). `ElemType` is what element narrowing trusts
// to hold an element in a byte (elemwidth-2026-08-27). Neither is checked for
// TIGHTNESS and neither should be: the analysis is allowed to be imprecise, and
// a test demanding precision would fail on every conservative answer the design
// exists to give.
//
// WHY RANDOM AND NOT HAND-WRITTEN. `TestIntervalsNeverOverclaim` exists, is
// hand-written, and PASSED for months while the fixpoint was not monotone
// (fixpoint-2026-08-27). Hand-written adversarial cases only catch what someone
// thought to write, and two of the ones written for `BufferRange` the same week
// turned out to expect a refusal where the analysis was right. A generator does
// not have opinions.
//
// THE PASS CONDITION, set before either half was believed: each must FAIL when
// the bug it exists for is put back.
//
//   - Operations. Reverting `restore` to install its snapshot by reference —
//     the fixpoint bug — fails at seed 15: the analysis claims every operation
//     is in -153..765 and the program produces 918.
//
//     The first version of the generator did NOT catch it, and the reason is
//     worth keeping: every conditional it produced sat in TAIL position, where
//     the environment after an `if` is never used again, so the leak was
//     invisible. The generator now puts conditionals in VALUE position too — as
//     an operand, where whatever the analysis believes after the `if` is
//     immediately spent on the other operand.
//
//   - Buffers. Reverting `bufferElem` to stop at the FIRST store — the bug that
//     made examples/json/tree.oro's node table one byte wide, so windows
//     returned 4030140 where three other targets returned 4040171 — must fail
//     too.
//
// A harness that cannot fail proves nothing, and neither half could until the
// shape it needed was in it.
//
// HOW THE INTERPRETER IS TRUSTED. It is a second implementation of the
// semantics, so it could be wrong on its own. What keeps it honest is that it
// is direct — no abstraction, no fixpoint, no joins — and that `arithOp` is
// shared with the analysis, so the two cannot drift about WHICH operations are
// counted, which is the only thing they have to agree on.

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
	// A clause chain puts every `if` in TAIL position, where the environment
	// after it is never used again — so the restore-by-reference leak was
	// invisible and the harness passed against the bug it was written for. Here
	// the `if` is an operand: whatever the analysis believes after it is
	// immediately spent on the other operand.
	if g.r.Intn(4) == 0 {
		return core.App(core.Name("go.+"),
			core.App(core.Name("if"), g.pred(), g.expr(depth+1), g.expr(depth+1)),
			g.expr(depth+1))
	}
	// DIVISION AND REMAINDER, which this generator did not produce until a carry
	// chain needed them — so `divI` and `remI` had never been checked for
	// γ-soundness at all, and a change to either was unfalsifiable here.
	//
	// The divisor is a NONZERO LITERAL, for two reasons that happen to agree:
	// division by zero is a precondition the refinement layer discharges
	// separately (integers.md §5) and the concrete interpreter would have to
	// invent an answer three hosts disagree on; and a literal base is the shape
	// that matters, because it is what a limb split is.
	//
	// Negative divisors are generated too. Truncation is toward zero and the
	// remainder takes the DIVIDEND's sign, so the four sign combinations are
	// four different paths through both transfer functions.
	if g.r.Intn(6) == 0 {
		op := []string{"go./", "go.%"}[g.r.Intn(2)]
		d := int64(1 + g.r.Intn(2000))
		if g.r.Intn(4) == 0 {
			d = -d
		}
		return core.App(core.Name(op), g.expr(depth+1), core.Int(d))
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

// ------------------------------------------------------- generation: buffers

// bufProgram builds the shape every real buffer in this repository has:
//
//	(build N (fn (b)
//	   (loop ((b b) (i 0) (a z))
//	      (if (>= i LIM) exit
//	          (again (set b idx val) (+ i 1) upd)))))
//
// The buffer is a LOOP VARIABLE, threaded, because `set` consumes its argument
// and hands it back (ADR 0018) — so it is the same buffer at every iteration
// and linearity holds without anything here having to check it.
//
// The counter steps by exactly +1 and the buffer is sized LIM+8, so every index
// this generates is inside the domain. Bounds are a PRECONDITION rather than a
// behaviour (tables.md), and a generator that produced out-of-range stores
// would be asking the interpreter to invent an answer three hosts disagree on.
func (g *gen) bufProgram() *core.Term {
	t, _ := g.buildTerm(false)
	return t
}

// buildTerm is bufProgram with the two knobs the read-back test needs: whether
// the loop hands the buffer out, and how long it is.
func (g *gen) buildTerm(returnBuffer bool) (*core.Term, int64) {
	lim := int64(3 + g.r.Intn(12))
	size := lim + 8
	g.vars = []string{"i", "a"}

	set := core.App(core.Name("set"), core.Name("b"), g.index(lim), g.stored())
	if g.r.Intn(3) == 0 {
		// TWO STORES IN ONE ITERATION. This is a node table's shape — a tag of
		// 1..5 into one slot and an index of up to 511 into another — and it is
		// the shape whose FIRST store used to decide the width for the whole
		// buffer (elemwidth-2026-08-27 §4).
		set = core.App(core.Name("set"), set, g.index(lim), g.stored())
	}
	back := core.App(core.Name("again"), set,
		core.App(core.Name("go.+"), core.Name("i"), core.Int(1)),
		g.update("a"))

	var exit *core.Term
	switch {
	case returnBuffer:
		exit = core.Name("b")
	case g.r.Intn(3) == 0:
		exit = core.Name("b") // the buffer outlives the loop: ADR 0018's freeze
	case g.r.Intn(2) == 0:
		exit = core.Name("a") // scratch: the buffer is dead, a count comes out
	default:
		exit = g.expr(1)
	}
	body := core.App(core.Name("if"),
		core.App(core.Name("go.>="), core.Name("i"), core.Int(lim)),
		exit, back)

	loop := core.App(core.Name("loop"),
		core.Fn([]string{"b", "i", "a"}, body),
		core.Name("b"), core.Int(0), core.Int(int64(g.r.Intn(5))))
	return core.App(core.Name("build"), core.Int(size),
		core.Fn([]string{"b"}, loop)), size
}

// readProgram is the shape the new rule exists for: a buffer built, FROZEN, and
// then read back by someone else.
//
//	(let (build N (fn (b) … b))
//	     (fn (t) (loop ((i 0) (acc 0))
//	               (>= i N) acc
//	               else (again (+ i 1) (+ acc (t i))))))
//
// Reduction produces exactly this — call-by-need let-binds an argument used more
// than once (ADR 0010), so `examples/json/tree.oro`'s `walk` arrives with its
// node table bound by a `let` one line above the reads.
//
// The property under test is not the range itself but what is DONE with it: if
// the frozen element range were wrong, `acc` would be claimed bounded while the
// program runs past the claim, and MaxOp containment catches that.
func (g *gen) readProgram() *core.Term {
	build, size := g.buildTerm(true)
	g.vars = []string{"i", "acc"}
	read := core.App(core.Name("t"), core.Name("i"))
	body := core.App(core.Name("if"),
		core.App(core.Name("go.>="), core.Name("i"), core.Int(size)),
		core.Name("acc"),
		core.App(core.Name("again"),
			core.App(core.Name("go.+"), core.Name("i"), core.Int(1)),
			core.App(core.Name("go.+"), core.Name("acc"), read)))
	loop := core.App(core.Name("loop"),
		core.Fn([]string{"i", "acc"}, body), core.Int(0), core.Int(0))
	return core.App(core.Name("let"), build, core.Fn([]string{"t"}, loop))
}

func (g *gen) index(lim int64) *core.Term {
	switch g.r.Intn(3) {
	case 0:
		return core.Name("i")
	case 1:
		return core.App(core.Name("go.+"), core.Name("i"),
			core.Int(int64(g.r.Intn(3))))
	}
	return core.Int(g.r.Int63n(lim))
}

// stored is every shape a stored value takes, chosen so that BOTH derivations
// are exercised — the syntactic one in `bufferElem`, which is exact by
// construction, and the interval one in `BufferRange`, which is not.
func (g *gen) stored() *core.Term {
	switch g.r.Intn(7) {
	case 0:
		// A LITERAL: its own exact range, and the case that needs no analysis.
		return core.Int(int64(g.r.Intn(400) - 40))
	case 1:
		// A CONDITIONAL OVER LITERALS — a tag or a sentinel, `(if c 125 93)` —
		// which storedRange joins rather than picking a side of.
		return core.App(core.Name("if"), g.pred(),
			core.Int(int64(g.r.Intn(300))), core.Int(int64(g.r.Intn(300))))
	case 2:
		return core.Name(g.vars[g.r.Intn(len(g.vars))])
	case 3:
		return g.expr(1)
	case 4:
		// BOUNDED BY THE LOOP GUARD AND BY NOTHING A LITERAL CAN SHOW, which is
		// the case the interval analysis exists for and the one tree.oro's node
		// table is.
		return core.App(core.Name("go.*"), core.Name("i"),
			core.Int(int64(g.r.Intn(40))))
	case 5:
		// A READ BACK OUT OF THE BUFFER BEING BUILT. Nothing may narrow on
		// this — the fact would be feeding on itself — so the analysis has to
		// REFUSE, and this is the shape that finds out whether it does.
		return core.App(core.Name("b"), core.Name("i"))
	}
	// A MIXTURE, and it is the dangerous one: one branch exact and one not, so
	// a rule that takes the exact half and forgets the other narrows to a range
	// the program leaves.
	return core.App(core.Name("if"), g.pred(),
		core.Int(int64(g.r.Intn(50))),
		core.App(core.Name("go.+"), core.Name("a"), core.Int(int64(g.r.Intn(9)))))
}

// ------------------------------------------------------------- interpretation

// bval is a concrete value: an integer, or a table.
//
// A `set` MUTATES IN PLACE and hands the same object back, which is the
// operational reading of ADR 0018 — the buffer is linear, so nothing else can
// be holding it and nothing else can observe the difference between mutating
// and copying. That is one line here and a whole aliasing story in a language
// without the discipline.
type bval struct {
	n   int64
	buf *bufv
}

type bufv struct{ cell []int64 }

type runner struct {
	tgt    *Target
	ops    []int64 // every CHECKABLE operation's result, in evaluation order
	stores []int64 // every value written into a buffer
	fuel   int
}

type againVals struct{ v []bval }

func (a *againVals) Error() string { return "again" }

func ident(x string) string { return x }

func copyEnv(env map[string]bval) map[string]bval {
	out := make(map[string]bval, len(env)+3)
	for k, v := range env {
		out[k] = v
	}
	return out
}

// eval runs a term concretely. It is the semantics, written once and directly:
// no widening, no fixpoint, no joins.
func (r *runner) eval(t *core.Term, env map[string]bval) (bval, error) {
	r.fuel--
	if r.fuel <= 0 {
		return bval{}, fmt.Errorf("fuel")
	}
	switch t.Kind {
	case core.KInt:
		return bval{n: t.Int}, nil
	case core.KName:
		v, ok := env[t.Name]
		if !ok {
			return bval{}, fmt.Errorf("unbound %s", t.Name)
		}
		return v, nil
	case core.KApp:
		op := t.Op()
		if op.Kind != core.KName {
			return bval{}, fmt.Errorf("higher-order")
		}
		args := t.Args()
		switch op.Name {
		case "if":
			c, err := r.eval(args[0], env)
			if err != nil {
				return bval{}, err
			}
			if c.n != 0 {
				return r.eval(args[1], env)
			}
			return r.eval(args[2], env)
		case "again":
			out := &againVals{}
			for _, a := range args {
				v, err := r.eval(a, env)
				if err != nil {
					return bval{}, err
				}
				out.v = append(out.v, v)
			}
			return bval{}, out
		case "loop":
			return r.loop(t, env)
		case "let":
			if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
				return bval{}, fmt.Errorf("let shape")
			}
			v, err := r.eval(args[0], env)
			if err != nil {
				return bval{}, err
			}
			lbody, lraw, _ := openFresh(args[1], map[string]bool{}, ident)
			inner := copyEnv(env)
			inner[lraw[0]] = v
			return r.eval(lbody, inner)
		}
		// INDEXING IS APPLICATION (tables.md), so a read is told from a
		// primitive call by its operator being bound to a table — the same test
		// the emitter makes, and unambiguous because a surviving lambda in
		// operator position is a refused closure.
		if b, ok := env[op.Name]; ok && b.buf != nil && len(args) == 1 {
			return r.read(b.buf, args[0], env)
		}
		if p, known := r.tgt.Prims[op.Name]; known {
			switch p.Kind {
			case "table-build":
				return r.build(t, env)
			case "table-set":
				return r.set(t, env)
			case "len":
				b, err := r.eval(args[0], env)
				if err != nil {
					return bval{}, err
				}
				if b.buf == nil {
					return bval{}, fmt.Errorf("len of a non-table")
				}
				return bval{n: int64(len(b.buf.cell))}, nil
			}
		}
		vals := make([]int64, len(args))
		for i, a := range args {
			v, err := r.eval(a, env)
			if err != nil {
				return bval{}, err
			}
			if v.buf != nil {
				return bval{}, fmt.Errorf("table in operand position")
			}
			vals[i] = v.n
		}
		return r.prim(op.Name, vals)
	}
	return bval{}, fmt.Errorf("kind %v", t.Kind)
}

// build allocates the buffer ZERO-FILLED, which is tables.md §14.3's guarantee
// and the reason 0 is always an element of the element range.
func (r *runner) build(t *core.Term, env map[string]bval) (bval, error) {
	args := t.Args()
	if len(args) != 2 || args[1].Kind != core.KFn || len(args[1].Params) != 1 {
		return bval{}, fmt.Errorf("build shape")
	}
	n, err := r.eval(args[0], env)
	if err != nil {
		return bval{}, err
	}
	if n.n < 0 || n.n > 1<<16 {
		return bval{}, fmt.Errorf("build size")
	}
	body, raw, _ := openFresh(args[1], map[string]bool{}, ident)
	inner := copyEnv(env)
	inner[raw[0]] = bval{buf: &bufv{cell: make([]int64, n.n)}}
	return r.eval(body, inner)
}

func (r *runner) set(t *core.Term, env map[string]bval) (bval, error) {
	args := t.Args()
	if len(args) != 3 {
		return bval{}, fmt.Errorf("set arity")
	}
	b, err := r.eval(args[0], env)
	if err != nil {
		return bval{}, err
	}
	if b.buf == nil {
		return bval{}, fmt.Errorf("set on a non-table")
	}
	i, err := r.eval(args[1], env)
	if err != nil {
		return bval{}, err
	}
	v, err := r.eval(args[2], env)
	if err != nil {
		return bval{}, err
	}
	if v.buf != nil {
		return bval{}, fmt.Errorf("storing a table")
	}
	if i.n < 0 || i.n >= int64(len(b.buf.cell)) {
		// Bounds are a PRECONDITION, not a behaviour (tables.md): Go panics,
		// Java throws, JavaScript silently returns undefined. A program outside
		// the domain is outside the fragment, so it is skipped rather than
		// given an answer this interpreter invented.
		return bval{}, fmt.Errorf("store out of range")
	}
	b.buf.cell[i.n] = v.n
	r.stores = append(r.stores, v.n)
	return b, nil // a store hands the buffer back — ADR 0018
}

func (r *runner) read(b *bufv, idx *core.Term, env map[string]bval) (bval, error) {
	i, err := r.eval(idx, env)
	if err != nil {
		return bval{}, err
	}
	if i.n < 0 || i.n >= int64(len(b.cell)) {
		return bval{}, fmt.Errorf("read out of range")
	}
	return bval{n: b.cell[i.n]}, nil
}

func (r *runner) prim(name string, v []int64) (bval, error) {
	switch name {
	case "go.<":
		return bval{n: b2i(v[0] < v[1])}, nil
	case "go.<=":
		return bval{n: b2i(v[0] <= v[1])}, nil
	case "go.>":
		return bval{n: b2i(v[0] > v[1])}, nil
	case "go.>=":
		return bval{n: b2i(v[0] >= v[1])}, nil
	case "=":
		return bval{n: b2i(v[0] == v[1])}, nil
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
	// DIVISION AND REMAINDER ARE EVALUATED BUT NOT RECORDED, and that is the
	// analysis's contract rather than a gap in this harness.
	//
	// `transfer` returns them with `checkable = false`, so `MaxOp` — the join of
	// every operation that would need an overflow check — deliberately does not
	// see them (they cannot grow a value, so nothing can leave the window
	// through one). Checking their results against MaxOp therefore tests a
	// property the analysis has never claimed, and it fails on the third
	// program the generator writes.
	//
	// What DOES check them is TestDivisionAndRemainderContain, directly and
	// exhaustively over the four sign combinations — which is stronger than this
	// harness could be for them anyway, because it quantifies over the DIVISOR's
	// interval rather than over whatever one program happens to contain.
	//
	// Go's own `/` and `%` on int64 ARE the language's semantics — truncation
	// toward zero and a remainder taking the dividend's sign — which
	// integers.md §3 and §4 measured all four hosts agreeing on.
	case "div", "rem":
		if v[1] == 0 {
			return bval{}, fmt.Errorf("division by zero")
		}
		if arithOp(name, len(v)) == "div" {
			return bval{n: v[0] / v[1]}, nil
		}
		return bval{n: v[0] % v[1]}, nil
	default:
		return bval{}, fmt.Errorf("prim %s", name)
	}
	// Exactly the operations MaxOp joins — arithOp is the one place that list
	// lives, so the harness cannot drift from what it is checking.
	r.ops = append(r.ops, out)
	return bval{n: out}, nil
}

// γ-SOUNDNESS OF DIVISION AND REMAINDER, checked directly.
//
// These two transfer functions had never been checked at all: the containment
// generator produced only `+`, `-`, `*` and comparisons, so `divI` and `remI`
// were unfalsifiable there from the day they were written — and a carry chain
// is made of nothing else.
//
// The property is the same one, at the level of the operation: for every
// concrete a ∈ γ(A) and b ∈ γ(B) with b ≠ 0,
//
//	a / b ∈ γ(divI(A, B))    and    a % b ∈ γ(remI(A, B))
//
// CONTAINMENT, never tightness. A claim too wide costs precision; one too
// narrow is a silent wrong answer, and for `divI` specifically a narrow claim
// would let an operation be "proven" inside the portable window when it is not.
func TestDivisionAndRemainderContain(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	bounds := []int64{-1000000, -70000, -1000, -7, -1, 0, 1, 7, 1000, 70000, 1000000}
	pick := func() ival {
		lo := bounds[r.Intn(len(bounds))]
		hi := bounds[r.Intn(len(bounds))]
		if lo > hi {
			lo, hi = hi, lo
		}
		v := ival{lo: lo, hi: hi}
		// One in eight is half-open, because an unbounded dividend divided by a
		// bounded divisor is exactly the carry chain's shape before the fixpoint
		// settles, and it is the case the contraction must NOT claim to bound.
		switch r.Intn(8) {
		case 0:
			v.hiInf = true
		case 1:
			v.loInf = true
		}
		return v
	}
	tried := 0
	for round := 0; round < 20000; round++ {
		A, B := pick(), pick()
		dv, rv := divI(A, B), remI(A, B)
		for s := 0; s < 12; s++ {
			a, ok1 := sample(r, A)
			b, ok2 := sample(r, B)
			if !ok1 || !ok2 || b == 0 {
				continue
			}
			tried++
			if !holds(dv, a/b) {
				t.Fatalf("divI(%s, %s) = %s, but %d / %d = %d", A, B, dv, a, b, a/b)
			}
			if !holds(rv, a%b) {
				t.Fatalf("remI(%s, %s) = %s, but %d %% %d = %d", A, B, rv, a, b, a%b)
			}
		}
	}
	// ANTI-VACUITY. A generator that never produced a legal pair would pass
	// forever while testing nothing — the same guard the buffer half of this
	// file needs and for the same reason.
	if tried < 50000 {
		t.Fatalf("only %d concrete pairs were checked; the sampler is not "+
			"producing them", tried)
	}
}

// sample draws a concrete value from an interval, or reports that it cannot —
// an infinite end is sampled at a large magnitude rather than skipped, because
// the unbounded cases are the ones the contraction must handle.
func sample(r *rand.Rand, v ival) (int64, bool) {
	lo, hi := v.lo, v.hi
	if v.loInf {
		lo = -1 << 40
	}
	if v.hiInf {
		hi = 1 << 40
	}
	if lo > hi {
		return 0, false
	}
	span := hi - lo
	if span < 0 { // overflow on a doubly-infinite interval
		return r.Int63() - (1 << 40), true
	}
	return lo + r.Int63n(span+1), true
}

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (r *runner) loop(t *core.Term, env map[string]bval) (bval, error) {
	args := t.Args()
	lam := args[0]
	cur := make([]bval, len(args)-1)
	for i, z := range args[1:] {
		v, err := r.eval(z, env)
		if err != nil {
			return bval{}, err
		}
		cur[i] = v
	}
	// OPENED, not Closed(). A lambda body holds its parameters as KBound — by
	// index, never written in source — so every walker in this package opens it
	// first, and an interpreter is no different.
	body, raw, _ := openFresh(lam, map[string]bool{}, ident)
	for iter := 0; iter < 10000; iter++ {
		inner := copyEnv(env)
		for i, n := range raw {
			inner[n] = cur[i]
		}
		out, err := r.eval(body, inner)
		if a, ok := err.(*againVals); ok {
			if len(a.v) != len(cur) {
				return bval{}, fmt.Errorf("again arity")
			}
			copy(cur, a.v)
			continue
		}
		return out, err
	}
	return bval{}, fmt.Errorf("did not terminate")
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
		if _, err := run.eval(term, map[string]bval{}); err != nil {
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

// TestBufferElementContainment is the same property one level along, on the
// decision that TRUNCATES rather than merely widening.
//
//	THEOREM (buffer γ-soundness).  Let E = ElemType(b) = (int lo hi). If
//	0 ∈ [lo,hi] and every value stored into b lies in [lo,hi], then every value
//	ever READ OUT of b lies in [lo,hi].
//
//	PROOF.  A slot holds either the zero fill or the value of the most recent
//	`set` into it — there is no third source, because `build` is the only
//	allocator and `set` the only store, and ADR 0018's linearity means no other
//	reference can have written it. Both cases are in [lo,hi] by hypothesis. ∎
//
// So checking the stores and the zero is not merely NECESSARY for the property
// the emitter needs; it is SUFFICIENT. That is why this harness checks exactly
// those two things and nothing else, and it is the whole reason the check is
// cheap.
//
// It also re-checks MaxOp, because these programs contain a construct the first
// half never generated: a read out of a buffer, whose value the analysis cannot
// see at all.
func TestBufferElementContainment(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// A `typeOf` that knows nothing, which is the honest position for a
	// generated program with no signature: it forces every non-literal store
	// past the syntactic rule and onto the interval analysis — the derivation
	// with the weaker argument, and therefore the one worth testing.
	noTypes := func(*core.Term) string { return "" }

	checked, claimed, skipped := 0, 0, 0
	for seed := int64(1); seed <= 2000; seed++ {
		g := &gen{r: rand.New(rand.NewSource(seed)), tgt: tg}
		term := g.bufProgram()

		lam := term.Args()[1]
		body, raw, _ := openFresh(lam, map[string]bool{}, ident)
		elem := ElemType(tg, lam, body, raw[0], noTypes, nil, nil)
		width := BufferElemBytes(tg, lam, body, raw[0], nil, nil)
		rep, _ := Intervals(tg, nil, term, 0)

		run := &runner{tgt: tg, fuel: 2000000}
		if _, err := run.eval(term, map[string]bval{}); err != nil {
			skipped++
			continue
		}
		if len(run.stores) == 0 {
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

		lo, hi, narrowed := core.IntRange(elem)
		if narrowed {
			claimed++
			// THE ZERO FILL IS AN ELEMENT. Nothing writes a leaf slot, and a
			// read of one has to be in the range too.
			if lo > 0 || hi < 0 {
				t.Fatalf("seed %d: the element range is %q, which excludes the "+
					"zero fill\n%s", seed, elem, indentTerm(term))
			}
			for _, v := range run.stores {
				if v < lo || v > hi {
					t.Fatalf("seed %d: the analysis claims every element is in "+
						"%q, and the program stores %d\n%s",
						seed, elem, v, indentTerm(term))
				}
			}
		}
		// AND THE WIDTH, which is the decision on a host with no type system to
		// read a range off — x86 gets bytes and nothing else. It is a separate
		// claim from the range: it says the byte count chosen actually holds
		// the value, on one interpretation or the other.
		for _, v := range run.stores {
			if !fitsBytes(width, v) {
				t.Fatalf("seed %d: the element is stored in %d byte(s) and the "+
					"program stores %d (element type %q)\n%s",
					seed, width, v, elem, indentTerm(term))
			}
		}
	}
	if checked < 500 {
		t.Fatalf("only %d of 2000 buffer programs ran (%d skipped); the "+
			"generator is producing programs the harness cannot run", checked, skipped)
	}
	// THE ANTI-VACUITY GUARD, and this half needs it more than the first.
	// Refusing to narrow is always sound, so a harness that only ever saw
	// refusals would pass forever while testing nothing. It has to watch the
	// compiler COMMIT.
	if claimed < 200 {
		t.Fatalf("only %d of %d buffers got a range narrower than the host word; "+
			"the harness is passing because nothing is being claimed, not "+
			"because the claims are sound", claimed, checked)
	}
	t.Logf("%d buffer programs checked (%d with a narrowed element), %d skipped",
		checked, claimed, skipped)
}

// TestBufferDoesNotNarrowOnItsOwnContents pins the non-circularity rule.
//
// This is a POLICY test, not a soundness one, and the distinction is worth
// keeping straight. A buffer whose stores read out of itself may well have a
// narrow range in fact — the program below stores only 1s — so no random search
// will ever produce a counterexample here, and the containment harness above is
// the wrong instrument. What is being pinned is the DERIVATION: an element
// range may be built from a declared range or from a buffer's SYNTACTIC one,
// never from `BufferRange`, because that is a fixpoint and feeding it its own
// output is a fixpoint on a fixpoint with nothing establishing a base case.
//
// The rule is stated in interval.go and is the kind that is easy to lose to a
// refactor that looks like a simplification, so it gets a test.
func TestBufferDoesNotNarrowOnItsOwnContents(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := "(build 16 (fn (b) (loop ((b b) (i 0)) (go.>= i 8) b " +
		"else (again (set b i (go.+ (b i) 1)) (go.+ i 1)))))"
	terms, err := core.ReadAll(src)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	term := terms[0]
	lam := term.Args()[1]
	body, raw, _ := openFresh(lam, map[string]bool{}, ident)
	got := ElemType(tg, lam, body, raw[0],
		func(*core.Term) string { return "" }, nil, nil)
	if got != "int" {
		t.Fatalf("a buffer whose stores read out of itself must keep the host's "+
			"own width; the element type came back %q", got)
	}

	// THE CONTROL, because "int" is also what a test that never reached
	// ElemType would report. The same program storing a literal must narrow, so
	// the refusal above is the rule firing rather than the test missing.
	ctl := "(build 16 (fn (b) (loop ((b b) (i 0)) (go.>= i 8) b " +
		"else (again (set b i 7) (go.+ i 1)))))"
	terms, err = core.ReadAll(ctl)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	lam = terms[0].Args()[1]
	body, raw, _ = openFresh(lam, map[string]bool{}, ident)
	if got := ElemType(tg, lam, body, raw[0],
		func(*core.Term) string { return "" }, nil, nil); got != "int 0 7" {
		t.Fatalf("the control must narrow to `int 0 7` — zero fill joined with "+
			"the one literal stored — and came back %q", got)
	}
}

// TestFrozenBufferReadContainment tests the STRATUM-1 rule: what a reader is
// allowed to believe about a buffer that has been frozen and handed out.
//
// The claim under test is not the range itself but what is spent on it. The
// generated reader accumulates every element, so if the frozen element range
// were too narrow, `acc` would be claimed bounded while the program runs past
// the claim — and MaxOp containment sees that directly.
//
// The anti-vacuity guard is the same one the buffer half needs and for the same
// reason: refusing to bound the reader is always sound, so the test insists the
// analysis actually commits on a decent share of these.
func TestFrozenBufferReadContainment(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	checked, bounded, skipped := 0, 0, 0
	for seed := int64(1); seed <= 1000; seed++ {
		g := &gen{r: rand.New(rand.NewSource(seed)), tgt: tg}
		term := g.readProgram()

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
		if !rep.MaxOp.loInf && !rep.MaxOp.hiInf {
			bounded++
		}
		for _, v := range run.ops {
			if !holds(rep.MaxOp, v) {
				t.Fatalf("seed %d: the analysis claims every operation is in %s, "+
					"and one produced %d\n%s",
					seed, rep.MaxOpRange(), v, indentTerm(term))
			}
		}
	}
	if checked < 300 {
		t.Fatalf("only %d of 1000 read-back programs ran (%d skipped)", checked, skipped)
	}
	if bounded < 100 {
		t.Fatalf("only %d of %d read-back programs got a bounded MaxOp; the "+
			"frozen element range is not reaching the reader, so this test is "+
			"passing on refusals", bounded, checked)
	}
	t.Logf("%d read-back programs checked (%d with a bounded MaxOp), %d skipped",
		checked, bounded, skipped)
}

// fitsBytes is the weakest true statement about an n-byte slot: it holds a
// value if the value fits either the signed or the unsigned interpretation.
// Which of the two a target means is its own declaration's business — the JVM's
// `byte` is SIGNED and 0..255 does not fit it, which is why a byte range gets
// `short[]` on Java (elemwidth-2026-08-27).
func fitsBytes(n int, v int64) bool {
	if n >= 8 {
		return true
	}
	bits := uint(8 * n)
	return v >= -(int64(1)<<(bits-1)) && v <= int64(1)<<bits-1
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
// unsound claim proves nothing, so this feeds it one of each shape.
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
	// The buffer shape: one byte does not hold a node index of 511, which is
	// what the first-store rule effectively claimed it did.
	if fitsBytes(1, 511) {
		t.Fatal("one byte must not be said to hold 511 — this is the " +
			"first-store bug's signature")
	}
	if !fitsBytes(1, 255) || !fitsBytes(1, -128) || !fitsBytes(2, 511) {
		t.Fatal("fitsBytes is refusing values that fit")
	}
}
