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
	post := PostVars(body, raw)
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
	if len(PostVars(body, raw)) != 0 {
		t.Error("two different updates cannot become one post clause")
	}
}

// A variable passed through unchanged is already skipped and needs no hoist.
func TestUnchangedVariablesAreNotHoisted(t *testing.T) {
	raw := []string{"i"}
	body := mustRead(t, `(if c (again i) i)`)
	if len(PostVars(body, raw)) != 0 {
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
