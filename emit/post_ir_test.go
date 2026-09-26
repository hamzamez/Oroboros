package emit_test

import (
	"regexp"
	"strings"
	"testing"
)

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
	if postVar(code) == "" {
		t.Fatalf("the uniform update belongs in the post clause:\n%s", code)
	}
	best := regexp.MustCompile(`var (v\d+) float64 = 0\.0`).FindStringSubmatch(code)
	if best == nil || !strings.Contains(code, "\t"+best[1]+" = ") {
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
		// The IR numbers its values, so the update is found by its shape: the
		// variable the post clause steps, `v = (v + 1)`.
		v := postVar(code)
		if v == "" {
			t.Errorf("%s: no post clause:\n%s", c.target, code)
			continue
		}
		want := v + " = (" + v + " + 1)"
		if n := strings.Count(code, want); n != 1 {
			t.Errorf("%s: %q appears %d times, must be exactly 1:\n%s",
				c.target, want, n, code)
		}
	}
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
				  (let d (go.+ i 1)
        (again (if (go.> d mx) d mx) (go.+ i 1))))))
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

// postVar is the variable a `for` statement's post clause steps by one, on any
// of the three expression hosts (`for ; ; v = (v + 1) {`, `for (;; v = (v + 1)) {`).
func postVar(code string) string {
	for _, m := range regexp.MustCompile(`for \(?\s*;\s*;\s*(v\d+) = \((v\d+) \+ 1\)`).FindAllStringSubmatch(code, -1) {
		if m[1] == m[2] {
			return m[1]
		}
	}
	return ""
}

// THE POST CLAUSE'S RULES, on the IR (ir/plan DecideLoop). A parameter moves to
// the post clause only when every continue advances it by the SAME LITERAL, by
// a value nothing else reads. The term backend's five PostVars unit tests are
// these programs now (irstep4a-2026-09-26); each counts the post clause's
// assignments, since the IR's names are value numbers.
//   - an update reading another changing variable is not hoisted (i + j);
//   - disagreeing updates are not hoisted (+1 in one clause, +2 in another);
//   - an unchanged variable is not hoisted;
//   - an update naming a value the body computes is not hoisted;
//   - and, narrower than the term backend: an update by an ENCLOSING name
//     (i + n) is not hoisted, because its step is not a literal. Go measured
//     at parity without it (irstep2-2026-09-25), the sieve's j += i included.
func TestThePostClauseRules(t *testing.T) {
	for _, c := range []struct {
		name, loop string
		hoisted    int
	}{
		{"reads another variable", `(loop ((i 0) (j 0)) (go.>= j n) i else (again (go.+ i j) (go.+ j 1)))`, 1},
		{"disagreeing updates", `(loop ((i 0)) (go.>= i n) i (go.== (go.% i 3) 0) (again (go.+ i 1)) else (again (go.+ i 2)))`, 0},
		{"an unchanged variable", `(loop ((i 0) (k 5)) (go.>= i n) k else (again (go.+ i 1) k))`, 1},
		{"a value the body computes", `(loop ((v 0) (i 0)) (go.>= i n) v else (let nv (go./ (go.+ v 300) 2) (again nv (go.+ i 1))))`, 1},
		{"an enclosing name", `(loop ((i 0)) (go.>= i n) i else (again (go.+ i n)))`, 0},
	} {
		code, err := genOn(t, "go", `(use go)
			(export f) (sig f ((n (int 1 1000))) int)
			(def f (fn (n) `+c.loop+`))`, "f")
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		header := regexp.MustCompile(`for ; ; (.*) \{`).FindStringSubmatch(code)
		got := 0
		if header != nil {
			got = strings.Count(header[1], " = ")
		}
		if got != c.hoisted {
			t.Errorf("%s: %d update(s) in the post clause, want %d:\n%s", c.name, got, c.hoisted, code)
		}
	}
}
