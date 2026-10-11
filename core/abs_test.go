package core

import (
	"strings"
	"testing"
)

// treeOf reads one form and returns the tree μ built for it.
func treeOf(t *testing.T, src string) (Tail, *reader) {
	t.Helper()
	r := newReader(src, false)
	r.skipSpace()
	d, err := r.term()
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return r.abs[d], r
}

// THE TREES ARE Σ's (abstract.md §2.4): each form of today's notation is read
// into its operator, a clause body into a Tail, and a match into a loop whose
// arms have patterns.
func TestTheReaderBuildsTreesOfSigma(t *testing.T) {
	cases := []struct {
		src  string
		want string
	}{
		{`(let x 1 y 2 (+ x y))`, "*core.Let"},
		{`(let (tuple a b) p (+ a b))`, "*core.Let"},
		{`(seq (f) (g))`, "*core.Seq"},
		{`(if c a b)`, "*core.If"},
		{`(and a b)`, "*core.And"},
		{`(or a b)`, "*core.Or"},
		{`(not a)`, "*core.Not"},
		{`(cond a 1 b 2 else 3)`, "*core.Cond"},
		{`(tuple a b)`, "*core.Apply"},
		{`(fn (x y) (+ x y))`, "*core.Lam"},
		{`(loop ((i 0)) (>= i 3) i else (again (+ i 1)))`, "*core.Loop"},
		{`(match (s) 0 1 else 2)`, "*core.Loop"},
		{`(local b (table n 0) c 0 (f b c))`, "*core.Local"},
		{`(build-map m 8 (f m))`, "*core.BuildMap"},
		{`(case r (ok v) v (err e) e)`, "*core.Case"},
		{`(try (ok v) r v)`, "*core.Monad"},
		{`(the (int 0 9) x)`, "*core.The"},
		{`(again 1)`, "*core.Again"},
		{`(f x)`, "*core.Apply"},
		// THE IDENTITY LAWS, as today's reader had them: a monoid's operation on
		// one operand is that operand, and a binding group of no pairs is its body.
		{`(and x)`, "*core.Var"},
		{`(or x)`, "*core.Var"},
		{`(let x)`, "*core.Var"},
		{`(cond else x)`, "*core.Var"},
	}
	for _, c := range cases {
		a, _ := treeOf(t, c.src)
		if got := typeName(a); got != c.want {
			t.Errorf("%s: tree %s, want %s", c.src, got, c.want)
		}
	}
	// A match is a loop over its scrutinees, its arms' patterns over them.
	a, _ := treeOf(t, `(match (s i) 0 _ (when (> i 3)) 1 x 7 (again x i) else 2)`)
	m := a.(*Loop)
	if !m.Match || len(m.Vars) != 2 || m.Vars[0] != "s" || len(m.Arms) != 3 {
		t.Fatalf("match read as %+v", m)
	}
	if _, ok := m.Arms[0].Pats[0].(*PLit); !ok || m.Arms[0].When == nil {
		t.Errorf("first arm: %+v", m.Arms[0])
	}
	if p, ok := m.Arms[1].Pats[0].(*PVar); !ok || p.Name != "x" {
		t.Errorf("second arm's pattern: %+v", m.Arms[1].Pats[0])
	}
	if _, ok := m.Arms[1].Body.(*Again); !ok {
		t.Errorf("second arm's body is the back edge, got %s", typeName(m.Arms[1].Body))
	}
	// A local's binder is a buffer or a cell by its initializer's form (ADR 0047).
	l, _ := treeOf(t, `(local b (table n 0) a (array 1 2) c 0 (f b a c))`)
	binds := l.(*Local).Binds
	if !binds[0].Buffer || !binds[1].Buffer || binds[2].Buffer {
		t.Errorf("buffer or cell: %+v", binds)
	}
}

func typeName(a Tail) string {
	switch a.(type) {
	case *Var:
		return "*core.Var"
	case *Lit:
		return "*core.Lit"
	case *Lam:
		return "*core.Lam"
	case *Apply:
		return "*core.Apply"
	case *Let:
		return "*core.Let"
	case *Seq:
		return "*core.Seq"
	case *If:
		return "*core.If"
	case *And:
		return "*core.And"
	case *Or:
		return "*core.Or"
	case *Not:
		return "*core.Not"
	case *Cond:
		return "*core.Cond"
	case *Loop:
		return "*core.Loop"
	case *Local:
		return "*core.Local"
	case *BuildMap:
		return "*core.BuildMap"
	case *Case:
		return "*core.Case"
	case *Monad:
		return "*core.Monad"
	case *The:
		return "*core.The"
	case *Again:
		return "*core.Again"
	case *TLet:
		return "*core.TLet"
	case *TSeq:
		return "*core.TSeq"
	case *TMonad:
		return "*core.TMonad"
	case *TRedex:
		return "*core.TRedex"
	}
	return "?"
}

// THE TAIL RULE IS A SORT (W3, ADR 0015, abstract.md §2.5). A clause body is a
// Tail: a back edge with one argument per loop variable, or a binding — a let, a
// tuple pattern, a seq, a success arm, a λ of one parameter applied to one
// argument — whose body is one. Anything else is an Expr, and an `again`
// demoted into an Expr is refused by the loop. Every tail form, and every way to
// put an `again` where it is not one, is here.
func TestTheTailRuleIsASort(t *testing.T) {
	wrap := func(body string) string {
		return `(loop ((i 0) (j 0)) (>= i 3) i else ` + body + `)`
	}
	ok := []string{
		`(again (+ i 1) j)`,
		`(let x (f i) (again x j))`,
		`(let x 1 y 2 (again x y))`,
		`(let (tuple a b) (p i) (again a b))`,
		`(seq (f i) (again i j))`,
		`(seq (f i) (g) (let x 1 (again x j)))`,
		`(try (ok v) (r i) (again v j))`,
		`(expect (ok v) (r i) "why" (seq (f v) (again v j)))`,
		`((fn (x) (again x j)) (f i))`,
		`(let x (loop ((k 0)) (>= k 2) k else (again (+ k 1))) (again x j))`,
		`(match (i) 0 (again 1) else 7)`,
		`(if c 1 2)`,
	}
	for _, b := range ok {
		if _, err := ReadTerm(wrap(b)); err != nil {
			t.Errorf("%s must be accepted: %v", b, err)
		}
	}
	refused := []struct{ body, why string }{
		{`(if c (again i j) 2)`, "may be a clause body"},
		{`(cond c (again i j) else 2)`, "may be a clause body"},
		{`(and c (again i j))`, "may be a clause body"},
		{`(not (again i j))`, "may be a clause body"},
		{`(f (again i j))`, "may be a clause body"},
		{`(tuple (again i j) 1)`, "may be a clause body"},
		{`(let x (again i j) x)`, "may be a clause body"},
		{`(let x (+ 1 (again i j)) (again x j))`, "may be a clause body"},
		{`(seq (again i j) 1)`, "may be a clause body"},
		{`(try (ok v) (again i j) v)`, "may be a clause body"},
		{`(case r (ok v) (again v j) (err e) 0)`, "may be a clause body"},
		{`(f (fn (a b) (again a b)))`, "may be a clause body"},
		{`(fn (x) (again x j))`, "may be a clause body"},
		{`((fn (x y) (again x y)) 1 2)`, "may be a clause body"},
		{`((fn (x y) (again x y)) 1)`, "may be a clause body"},
		{`(local c 0 (again c j))`, "may be a clause body"},
		{`(the int (again i j))`, "may be a clause body"},
		{`(loop ((k (again i j))) (>= k 2) k else (again (+ k 1)))`, "may be a clause body"},
		{`(again i)`, "again takes 2 argument(s)"},
		{`(let x 1 (again x j 3))`, "again takes 2 argument(s)"},
		{`(try (ok v) (r i) (again v))`, "again takes 2 argument(s)"},
	}
	for _, c := range refused {
		_, err := ReadTerm(wrap(c.body))
		if err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: want refused (%s), got %v", c.body, c.why, err)
		}
	}
	// Outside every loop an `again` is the free name it is, an application
	// refused later as unbound — as today.
	if _, err := ReadTerm(`(f (if c (again 1) 2))`); err != nil {
		t.Errorf("an again with no loop is not the reader's to refuse: %v", err)
	}
	// A nested loop's body is its own: its `again`s are not the outer clause's,
	// and its initial values are.
	if _, err := ReadTerm(wrap(`(let x (loop ((k 0)) (>= k 2) k (= k 1) (again 5) else (again (+ k 1))) (again x j))`)); err != nil {
		t.Errorf("a nested loop's agains are its own: %v", err)
	}
}

// THE SORTS ARE TYPES. An Expr slot cannot hold an *Again: demotion is the only
// way a tail enters an expression, and it marks the tree.
func TestADemotedTailIsStray(t *testing.T) {
	a, r := treeOf(t, `(f (again 1))`)
	app := a.(*Apply)
	if _, ok := app.Args[0].(*Apply); !ok {
		t.Fatalf("an again in an argument is the name applied, got %s", typeName(app.Args[0]))
	}
	if !r.stray[app] || !r.stray[app.Args[0]] {
		t.Errorf("a demoted again marks its tree")
	}
	b, r2 := treeOf(t, `(let x 1 (again x))`)
	if _, ok := b.(*TLet); !ok {
		t.Fatalf("a let around a back edge is a TLet, got %s", typeName(b))
	}
	if e := r2.demote(b); !r2.stray[e] {
		t.Errorf("a TLet demoted is stray")
	}
}

// D OF A DERIVED OPERATOR IS D OF ITS DEFINITION (Proposition D2,
// abstract.md §2.7), and D of Index is application (decision 6: indexing is
// application in the meaning).
func TestDerivedOperatorsAreTheirDefinitions(t *testing.T) {
	d := newDesugarer()
	a, b := &Var{Name: "a"}, &Var{Name: "b"}
	pairs := []struct {
		name          string
		derived, prim Tail
	}{
		{"seq", &Seq{Xs: []Expr{a, b}}, &Let{Binds: []LetBind{{Pat: &PWild{}, Value: a}}, Body: b}},
		{"index", &Index{Table: a, At: b}, &Apply{Fn: a, Args: []Expr{b}}},
		{"not", &Not{X: a}, &If{C: a, A: &Lit{T: Bool(false)}, B: &Lit{T: Bool(true)}}},
		{"and", &And{Xs: []Expr{a, b}}, &If{C: a, A: b, B: &Lit{T: Bool(false)}}},
		{"or", &Or{Xs: []Expr{a, b}}, &If{C: a, A: &Lit{T: Bool(true)}, B: b}},
	}
	for _, p := range pairs {
		if got, want := d.term(p.derived).String(), d.term(p.prim).String(); got != want {
			t.Errorf("%s: D gives %s, its definition %s", p.name, got, want)
		}
	}
	// (and) and (or) are their units.
	if d.term(&And{}).String() != "true" || d.term(&Or{}).String() != "false" {
		t.Errorf("the connectives' units")
	}
}

// A BINDER GROUP NAMES DISTINCT THINGS (W2, abstract.md §3): a pattern's names,
// a λ's parameters, a loop's state and a scope's binders are each one group.
func TestABinderGroupIsLinear(t *testing.T) {
	for _, src := range []string{
		`(fn (x x) x)`,
		`(let (tuple a a) p a)`,
		`(loop ((i 0) (i 1)) else i)`,
		`(local c 0 c 1 c)`,
		`(build-map m 8 m 8 m)`,
		`(match (a b) x x 1 else 2)`,
	} {
		if _, err := ReadTerm(src); err == nil || !strings.Contains(err.Error(), "twice") {
			t.Errorf("%s: want a repeated binder refused, got %v", src, err)
		}
	}
}
