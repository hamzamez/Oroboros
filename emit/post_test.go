package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE LOOP SHAPE (tables-write-2026-08-25 §1, loopshape-2026-08-25).
//
// Our loops emitted `for { if guard { break }; …; i = i + 1; continue }` where a
// person writes `for i := 2; i*i < n; i++`. The increment was duplicated into
// every clause, so the loop had several back edges and Go's SSA did not see a
// counted loop — worth 1.4x on the sieve.
//
// A loop variable updated identically by every `again` moves into the `for`
// statement's post clause.

// The uniform update is hoisted; the per-clause one is not.
func TestPostHoistsTheUniformUpdate(t *testing.T) {
	code, err := genOn(t, "go", `
		(use go)
		(export f) (sig f ((a (array f64))) f64)
		(def f (fn (a)
			(loop ((best 0.0) (i 0))
				(go.>= i (len a))        best
				(go.f> (a i) best)       (again (a i) (go.+ i 1))
				else                     (again best (go.+ i 1)))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(code, "for ; ; i = (i + 1) {") {
		t.Errorf("the uniform update belongs in the post clause:\n%s", code)
	}
	if !strings.Contains(code, "best = a[i]") {
		t.Errorf("a per-clause update stays in the body:\n%s", code)
	}
}

// THE BUG THIS TEST EXISTS FOR. The first version patched each backend's
// `emitAgain` separately; JavaScript's routes through the shared `changedArgs`
// instead, so the increment was emitted BOTH in the post clause and in the
// body. The sieve advanced `i` twice per iteration and got 1984 of 2000 answers
// wrong — silently, because the code still compiled and ran.
//
// The update must appear EXACTLY ONCE, on every target.
func TestTheHoistedUpdateAppearsExactlyOnce(t *testing.T) {
	cases := []struct{ target, src, want string }{
		{"go", `(use go)
			(export f) (sig f ((n int)) int)
			(def f (fn (n) (loop ((acc 0) (i 0))
				(go.>= i n) acc
				(go.> i 3)  (again acc (go.+ i 1))
				else        (again (go.+ acc 1) (go.+ i 1)))))`, "i = (i + 1)"},
		{"js", `(use js)
			(export f) (sig f ((n any)) any)
			(def f (fn (n) (loop ((acc 0) (i 0))
				(js.>= i n) acc
				(js.> i 3)  (again acc (js.+ i 1))
				else        (again (js.+ acc 1) (js.+ i 1)))))`, "i = (i + 1)"},
		{"java", `(use java)
			(export f) (sig f ((n int)) int)
			(def f (fn (n) (loop ((acc 0) (i 0))
				(java.>= i n) acc
				(java.> i 3)  (again acc (java.+ i 1))
				else          (again (java.+ acc 1) (java.+ i 1)))))`, "i = (i + 1)"},
	}
	for _, c := range cases {
		code, err := genOn(t, c.target, c.src, "f")
		if err != nil {
			t.Errorf("%s: %v", c.target, err)
			continue
		}
		if n := strings.Count(code, c.want); n != 1 {
			t.Errorf("%s: %q appears %d times, must be exactly 1:\n%s",
				c.target, c.want, n, code)
		}
	}
}

// SOUNDNESS. `again`'s arguments are evaluated simultaneously with every
// variable's OLD value; a post clause runs after the body. So an update that
// reads ANOTHER loop variable cannot be hoisted — the body may already have
// assigned it, and the post clause would see the new value.
func TestAnUpdateReadingAnotherVariableIsNotHoisted(t *testing.T) {
	raw := []string{"i", "j"}
	body := mustRead(t, `(if c (again (go.+ i j) (go.+ j 1)) i)`)
	post := PostVars(body, raw, map[string]bool{})
	if _, ok := post[0]; ok {
		t.Error("(go.+ i j) reads j, which is also changing — it must stay in the body")
	}
	if _, ok := post[1]; !ok {
		t.Error("(go.+ j 1) reads only j and should be hoisted")
	}
}

// Updates that DISAGREE between clauses have no single update to hoist.
func TestDisagreeingUpdatesAreNotHoisted(t *testing.T) {
	raw := []string{"i"}
	body := mustRead(t, `(if c (again (go.+ i 1)) (again (go.+ i 2)))`)
	if len(PostVars(body, raw, map[string]bool{})) != 0 {
		t.Error("two different updates cannot become one post clause")
	}
}

// A variable passed through unchanged is already skipped and needs no hoist.
func TestUnchangedVariablesAreNotHoisted(t *testing.T) {
	raw := []string{"i"}
	body := mustRead(t, `(if c (again i) i)`)
	if len(PostVars(body, raw, map[string]bool{})) != 0 {
		t.Error("an unchanged variable has nothing to hoist")
	}
}

func mustRead(t *testing.T, src string) *core.Term {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	return forms[0].Term
}

// AN UPDATE UNDER A `let` MAY NOT BE HOISTED (json-tree-2026-08-26).
//
// ADR 0015 permits `again` under a `let`, so an update can mention a name the
// loop body bound. The post clause is written on the `for` statement, OUTSIDE
// every binder the body opened — so hoisting one takes it out of scope.
//
// `collectAgains` walks the CLOSED body, so such a name is a bound index rather
// than a name, and the emitter used to reach it and give up with
// `unhandled term: #0.0`. Nothing had hit it because no program before had a
// non-trivial update under a `let`; a JSON tree walk did.
func TestPostDoesNotHoistOutOfALet(t *testing.T) {
	code, err := genOn(t, "go", `
		(use go)
		(export f) (sig f ((a (array f64))) int)
		(def f (fn (a)
			(loop ((mx 0) (i 0))
				(go.>= i (len a))  mx
				else
				  (let (go.+ i 1) (fn (d)
					(again (if (go.> d mx) d mx) (go.+ i 1)))))))
	`, "f")
	if err != nil {
		t.Fatal(err)
	}
	// `i` is still hoisted: its update mentions nothing the `let` bound.
	if !strings.Contains(code, "for ; ; ") {
		t.Errorf("the counter should still move into the post clause:\n%s", code)
	}
	if strings.Contains(code, "#0.") {
		t.Errorf("a bound index escaped into the emitted code:\n%s", code)
	}
}

// AN UPDATE NAMING SOMETHING BOUND INSIDE THE LOOP IS NOT HOISTED.
//
// The post clause runs at the `for` statement's head, outside every binder the
// body opens — so an update mentioning a `let`'s name is a reference to
// something that does not exist there. ADR 0015 permits `again` under a `let`
// precisely so a clause can compute with an intermediate value, which makes
// this reachable rather than theoretical: `examples/big/render.oro`'s digit
// loop binds `nv` to `v / 10^8` and passes it as the next `v`, and hoisting
// that emitted `undefined: nv`.
//
// The rule is stated positively — hoist only what is in scope AT THE POST
// CLAUSE — rather than by enumerating what to avoid. `scope` is the emitter's
// own set of bound names, so an enclosing loop's variable qualifies and a name
// introduced inside the body does not.
func TestAnUpdateNamingAnInnerBindingIsNotHoisted(t *testing.T) {
	raw := []string{"s", "v", "i"}
	body := mustRead(t, `(let (go./ v 100) (fn (nv) (again s nv (go.+ i 1))))`)
	post := PostVars(body, raw, map[string]bool{})
	if _, ok := post[1]; ok {
		t.Error("`nv` is bound by the `let` INSIDE the loop body; an update " +
			"naming it cannot run at the loop header")
	}
	// AND THE CONTROL: the counter beside it mentions nothing but itself, so it
	// must still hoist. A rule that refused everything would pass the check
	// above and prove nothing.
	if _, ok := post[2]; !ok {
		t.Error("(go.+ i 1) names only its own loop variable and must hoist; " +
			"the rule has become a blanket refusal")
	}
}

// AND A NAME THE EMITTER ALREADY HAS IN SCOPE IS FINE.
//
// The companion to the test above in the other direction: an update reading an
// enclosing function's parameter, or an outer loop's variable, refers to
// something that exists at the post clause and must not be refused. Without
// this the rule would cost the hoist on every loop that reads a parameter.
func TestAnUpdateReadingAnEnclosingNameIsHoisted(t *testing.T) {
	raw := []string{"i"}
	body := mustRead(t, `(if c (again (go.+ i n)) i)`)
	if _, ok := PostVars(body, raw, map[string]bool{"n": true})[0]; !ok {
		t.Error("`n` is bound outside the loop, so it is in scope at the post " +
			"clause and the update must hoist")
	}
	if _, ok := PostVars(body, raw, map[string]bool{})[0]; ok {
		t.Error("with `n` in nothing's scope the update must be refused; " +
			"otherwise this test is not testing the scope set")
	}
}
