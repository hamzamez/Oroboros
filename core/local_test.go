package core

import (
	"strings"
	"testing"
)

// `local` (spec/local.md): the initializer decides a binder, and a cell is a
// local variable in program order, translated away by the loader into the
// pure core. These are the translation's results and its refusals; the four
// targets' answers are the differential cases local-cells, local-tables and
// local-case.

func TestACellIsALocalVariableInProgramOrder(t *testing.T) {
	for _, c := range []struct{ body, want string }{
		// a read sees the last write on its path; an if joins its branches
		{`(local c 0 (seq (if p (set c 1) (set c 2)) c))`, `(fn (p) (if p 1 2))`},
		// a write the path does not take leaves the value
		{`(local c 5 (seq (if p (set c 1) (tuple)) c))`, `(fn (p) (if p 1 5))`},
		// writes in sequence: the last one is read
		{`(local c 0 (seq (set c 1) (set c (+ c 1)) c))`, `(fn (p) 2)`},
		// two cells, each its own variable
		{`(local a 1 b 2 (seq (set a b) (set b a) (+ a b)))`, `(fn (p) 4)`},
		// a tuple is a value, its components read in order
		{`(local c 1 (seq (set c 2) (tuple c 3)))`, `(fn (p) (fn (#k) (#k 2 3)))`},
	} {
		got := reduceWith(t, `(def f (fn (p) `+c.body+`))`, "f", "if", "=", "+")
		if got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.body, got, c.want)
		}
	}
}

func TestACellIsRefusedWhereItsOrderWouldNotBeTheProgramsOrder(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		// a closure could run any number of times, or later
		{`(def f (fn (xs) (local c 0 (seq (go.each xs (fn (i) (set c i))) c))))`, "a function that mentions a cell (c)"},
		// a primitive passed a λ: a host callback
		{`(def f (fn (p) (local c 0 (seq (each (fn (i) (set c i))) c))))`, "a function that mentions a cell"},
		// a binder hiding the cell
		{`(def f (fn (p) (local c 0 (let c 1 c))))`, "hides the cell c"},
		// a buffer's store on a cell
		{`(def f (fn (p) (local c 0 (set c 0 1))))`, "c is a cell, written (set c v)"},
		// a cell holds a value, not a function
		{`(def f (fn (p) (local g (fn (x) x) 0)))`, "holds a function"},
		// build's binder form is local's now
		{`(def f (fn (p) (build b 3 b)))`, "a scope of local state is `local`"},
		{`(def f (fn (p) (local c 0)))`, "binds NAME VALUE pairs and ONE body"},
	} {
		_, err := loadSrc(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want a refusal mentioning %q, got %v", c.src, c.want, err)
		}
	}
}

// A buffer binder's initializer is a table, and the zero table reads as build's
// own core form, so every scope that build wrote emits what it emitted.
func TestAZeroTableIsBuildsOwnForm(t *testing.T) {
	forms, err := Read(`(local b (table n 0) b)`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := forms[0].Term.String(), "(build n (fn (b) b))"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// A CELL COSTS WHAT A LOOP VARIABLE COSTS, exactly: a program written with a
// cell reduces to a residual α-equivalent to the same program written with a
// loop variable (Term.Equal is α-equivalence), so every analysis and every
// printer sees the same term (spec/local.md §3).
func TestACellReducesToTheLoopVariableItMeans(t *testing.T) {
	for _, c := range []struct{ cell, loopvar string }{
		{`(local total 0
		   (loop ((i 0)) (>= i n) total
		     else (seq (set total (+ total i)) (again (+ i 1)))))`,
			`(loop ((i 0) (total 0)) (>= i n) total else (again (+ i 1) (+ total i)))`},
		{`(local a 0 b 1
		   (loop ((i 0)) (>= i n) (+ a b)
		     else (seq (set a (+ a i)) (if (= i 2) (set b (+ b 1)) (tuple)) (again (+ i 1)))))`,
			// the write under `if` is a statement, so each branch goes round
			// with its own state after the write before it: a program the
			// surface cannot spell, since `again` may not sit under `if` in
			// source, so it is pinned as the residual
			""},
	} {
		p, err := loadSrc(t, `(def f (fn (n) `+c.cell+`)) (def g (fn (n) `+loopvarOr(c.loopvar)+`))`)
		if err != nil {
			t.Fatal(err)
		}
		env := testEnv(p, "if", "=", "+", ">=", "loop")
		f, err := Normalize(p.Defs["f"], env, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if c.loopvar == "" {
			want := "(fn (n) (loop (fn (i a b) (if (>= i n) (+ a b) (let (+ a i) (fn (a1) " +
				"(if (= i 2) (again (+ i 1) a1 (+ b 1)) (again (+ i 1) a1 b)))))) 0 0 1))"
			if f.String() != want {
				t.Errorf("got  %s\nwant %s", f, want)
			}
			continue
		}
		g, err := Normalize(p.Defs["g"], env, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if !f.Equal(g) {
			t.Errorf("the cell and the loop variable differ:\n cell %s\n var  %s", f, g)
		}
	}
}

func loopvarOr(s string) string {
	if s == "" {
		return "0"
	}
	return s
}

// AN ELIMINATION IS DECIDED ON THE ELIMINATED TERM'S NORMAL FORM. The
// translation lets the state flow into (op λ) as into a let, which is right
// only when op's value is data, a product or a sum, by η: then λ runs once,
// now. The static level is higher-order, so op may instead be a function,
// which could run λ twice: here `twice` would have counted 1 for 2. The claim
// is marked and the reducer decides it, once β has substituted the static
// arguments.
func TestAnEliminationIsDecidedOnTheNormalForm(t *testing.T) {
	twice := `(def twice (fn (n) (fn (body) (seq (body n) (body n)))))`
	for _, c := range []struct{ src, want string }{
		{twice + ` (def f (fn (p) (local c 0 (seq ((twice 0) (fn (i) (set c (+ c 1)))) c))))`,
			"is a function and may run it any number of times"},
		// bound first, the same function
		{twice + ` (def f (fn (p) (local c 0 (let g (twice 0) (seq (g (fn (i) (set c (+ c 1)))) c)))))`,
			"is a function and may run it any number of times"},
		// a primitive bound first: a host function, passed a callback
		{`(def f (fn (p) (local c 0 (let g each (seq (g (fn (i) (set c (+ c 1)))) c)))))`,
			"is a function and may run it any number of times"},
	} {
		p, err := loadSrc(t, c.src)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Normalize(p.Defs["f"], testEnv(p, "if", "=", "+", "!each"), DefaultFuel)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want a refusal mentioning %q, got %v", c.src, c.want, err)
		}
	}
	// Data is eliminated, and no mark is left: a tuple from a definition, and
	// the results of an impure host call, whose argument the reducer let-binds.
	for _, c := range []struct{ src, want string }{
		{`(def dm (fn (a b) (tuple (+ a b) (+ a 1))))
		  (def f (fn (p) (local c 0 (seq (let (tuple q r) (dm p 3) (set c (+ q r))) c))))`,
			`(fn (p) (+ (+ p 3) (+ p 1)))`},
		{`(def f (fn (p) (local c 0 (seq ((two p) (fn (q r) (set c (+ q r)))) c))))`,
			`(fn (p) ((two p) (fn (q r) (+ q r))))`},
	} {
		p, err := loadSrc(t, c.src+` (def g `+c.want+`)`)
		if err != nil {
			t.Fatal(err)
		}
		env := testEnv(p, "if", "=", "+", "!two")
		got, err := Normalize(p.Defs["f"], env, DefaultFuel)
		if err != nil {
			t.Fatalf("%s: %v", c.src, err)
		}
		want, err := Normalize(p.Defs["g"], env, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Equal(want) || strings.Contains(got.String(), ElimName) {
			t.Errorf("%s:\n got  %s\n want %s", c.src, got, c.want)
		}
	}
}

// A RESIDUAL LET IS β WHEN ITS VALUE IS DUPLICABLE, as a source let is, and
// `(let v (fn (x) x))` is v, the monad's right unit law, whatever v's effects.
// The translation leaves such lets, and an instance of a residual (a limb
// operation's, ADR 0035) does when a constant replaces a parameter.
func TestAResidualLetObeysBetaAndTheUnitLaw(t *testing.T) {
	p, err := loadSrc(t, `(def f (fn (p) p))`)
	if err != nil {
		t.Fatal(err)
	}
	env := testEnv(p, "+", "!eff")
	plus := func(a, b *Term) *Term { return App(Name("+"), a, b) }
	for _, c := range []struct {
		term *Term
		want string
	}{
		{App(Name("let"), Int(2), Fn([]string{"x"}, plus(Name("x"), Name("x")))), `4`},
		{App(Name("let"), App(Name("eff"), Int(1)), Fn([]string{"x"}, Name("x"))), `(eff 1)`},
	} {
		got, err := Normalize(c.term, env, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		if got.String() != c.want {
			t.Errorf("%s: got %s, want %s", c.term, got, c.want)
		}
	}
}
