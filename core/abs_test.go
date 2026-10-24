package core

import (
	"strings"
	"testing"
)

// THE TAIL RULE IS A SORT (W3, ADR 0015, abstract.md §2.5). A clause body is a
// Tail: a back edge with one argument per loop variable, or a binding — a let, a
// tuple pattern, a sequence, a success arm — whose body is one. Anything else is
// an Expr, and an `again` put into an Expr is refused by the loop. Every tail
// form, and every way to put an `again` where it is not one, is here, in J.
func TestTheTailRuleIsASort(t *testing.T) {
	wrap := func(body string) string {
		return "loop i = 0, j = 0 { i >= 3 -> i; else -> " + body + " }"
	}
	ok := []string{
		`again(i + 1, j)`,
		`{ x = f(i); again(x, j) }`,
		`{ x = 1; y = 2; again(x, y) }`,
		`{ (a, b) = p(i); again(a, b) }`,
		`{ f(i); again(i, j) }`,
		`{ f(i); g(); x = 1; again(x, j) }`,
		`try 'ok(v) = r(i) { again(v, j) }`,
		`{ expect 'ok(v) = r(i) else "why"; f(v); again(v, j) }`,
		`{ x = loop k = 0 { k >= 2 -> k; else -> again(k + 1) }; again(x, j) }`,
		`if c { 1 } else { 2 }`,
		// A CASE IN A TAIL is a tail when each arm is (ADR 0076): an arm may jump,
		// sit under a binding that jumps, or leave the loop with a value.
		`case r { 'ok(v) -> again(v, j); 'err(e) -> 0 }`,
		`case r { 'ok(v) -> { f(v); again(v, j) }; 'err(e) -> again(i, e) }`,
		`{ x = 1; case r { 'ok(v) -> again(v, x); 'err(e) -> { ignore(e); 0 } } }`,
		`case r { 'ok(v) -> case s { 'ok(w) -> again(v, w); 'err(e) -> 1 }; 'err(e) -> 2 }`,
		`case r { 'ok(v) -> v + 1; 'err(e) -> 0 } + 1`,
	}
	for _, b := range ok {
		if _, err := ReadTerm(wrap(b)); err != nil {
			t.Errorf("%s must be accepted: %v", b, err)
		}
	}
	refused := []struct{ body, why string }{
		{`if c { again(i, j) } else { 2 }`, "under an if"},
		{`c && again(i, j)`, "may be a clause body"},
		{`!again(i, j)`, "may be a clause body"},
		{`f(again(i, j))`, "may be a clause body"},
		{`(again(i, j), 1)`, "may be a clause body"},
		{`{ x = again(i, j); x }`, "may be a clause body"},
		{`{ x = 1 + again(i, j); again(x, j) }`, "may be a clause body"},
		{`{ again(i, j); 1 }`, "a jump stands before the end of its block"},
		{`try 'ok(v) = again(i, j) { v }`, "may be a clause body"},
		// …and only in a tail: as an operand, a binding's value, an argument or a
		// scrutinee it is a jump inside an expression.
		{`case r { 'ok(v) -> again(v, j); 'err(e) -> 0 } + 1`, "stands only where a tail may"},
		{`f(case r { 'ok(v) -> again(v, j); 'err(e) -> 0 })`, "stands only where a tail may"},
		{`{ x = case r { 'ok(v) -> again(v, j); 'err(e) -> 0 }; x }`, "stands only where a tail may"},
		{`case (case r { 'ok(v) -> again(v, j); 'err(e) -> 0 }) { 'ok(w) -> w; 'err(e) -> 0 }`, "stands only where a tail may"},
		{`case r { 'ok(v) -> again(v); 'err(e) -> 0 }`, "again takes 2 argument(s)"},
		{`case r { 'ok(v) -> if c { again(v, j) } else { 1 }; 'err(e) -> 0 }`, "under an if"},
		{`f(fn(a, b) { again(a, b) })`, "a λ's body is never a tail"},
		{`fn(x) { again(x, j) }`, "a λ's body is never a tail"},
		{`fn(x, y) { again(x, y) }(1, 2)`, "a λ's body is never a tail"},
		{`fn(x, y) { again(x, y) }(1)`, "a λ's body is never a tail"},
		// A λ of one parameter applied by hand is not a binding (ADR 0051, D5):
		// `let` is how a jump is bound.
		{`fn(x) { again(x, j) }(f(i))`, "a λ's body is never a tail"},
		{`loop k = again(i, j) { k >= 2 -> k; else -> again(k + 1) }`, "may be a clause body"},
		{`again(i)`, "again takes 2 argument(s)"},
		{`{ x = 1; again(x, j, 3) }`, "again takes 2 argument(s)"},
		{`try 'ok(v) = r(i) { again(v) }`, "again takes 2 argument(s)"},
	}
	for _, c := range refused {
		_, err := ReadTerm(wrap(c.body))
		if err == nil || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: want refused (%s), got %v", c.body, c.why, err)
		}
	}
	// Outside every loop an `again` in an expression is the free name it is, an
	// application refused later as unbound; under an `if`, J refuses it as a
	// jump wherever it stands.
	if _, err := ReadTerm(`f(again(1))`); err != nil {
		t.Errorf("an again with no loop is not the reader's to refuse: %v", err)
	}
	// A nested loop's body is its own: its `again`s are not the outer clause's,
	// and its initial values are.
	if _, err := ReadTerm(wrap(`{ x = loop k = 0 { k >= 2 -> k; k == 1 -> again(5); else -> again(k + 1) }; again(x, j) }`)); err != nil {
		t.Errorf("a nested loop's agains are its own: %v", err)
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
// a λ's parameters and a loop's state are each one group.
func TestABinderGroupIsLinear(t *testing.T) {
	for _, src := range []string{
		`fn(x, x) { x }`,
		`{ (a, a) = p; a }`,
		`loop i = 0, i = 1 { else -> i }`,
	} {
		if _, err := ReadTerm(src); err == nil || !strings.Contains(err.Error(), "twice") {
			t.Errorf("%s: want a repeated binder refused, got %v", src, err)
		}
	}
}
