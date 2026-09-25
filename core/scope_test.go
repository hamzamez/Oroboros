package core

import (
	"strings"
	"testing"
)

// A SCOPED BUFFER IS A BINDER (scope.go, tables.md §2.4). The binder form must
// read as the core form TERM FOR TERM, since nothing below the reader may know
// it exists; the scope must be sequential; and the refusals must name the rule.

func mustRead(t *testing.T, src string) *Term {
	t.Helper()
	got, err := ReadTerm(src)
	if err != nil {
		t.Fatalf("%s: %v", src, err)
	}
	return got
}

func TestTheBinderFormIsTheCoreForm(t *testing.T) {
	for _, c := range []struct{ sugar, core string }{
		{`(build b n (set b 0 1))`, `(build n (fn (b) (set b 0 1)))`},
		{`(build a n  b m  (g a b))`, `(build n (fn (a) (build m (fn (b) (g a b)))))`},
		{`(build a n  b (len a)  c 3  (h a b c))`,
			`(build n (fn (a) (build (len a) (fn (b) (build 3 (fn (c) (h a b c)))))))`},
		{`(build-map m cap (insert m 1 2))`, `(build-map cap (fn (m) (insert m 1 2)))`},
		{`(build b n ; a comment is a gap
		    (set b 0 1))`, `(build n (fn (b) (set b 0 1)))`},
	} {
		if got, want := mustRead(t, c.sugar).String(), mustRead(t, c.core).String(); got != want {
			t.Errorf("%s\n got %s\nwant %s", c.sugar, got, want)
		}
	}
}

// SEQUENTIAL, checked on the structure and not on the print: a printed term
// shows `a` whether or not a binder holds it (CLAUDE.md, "Printing hides an
// unbound binder"). In `(build a n b (len a) body)` the second size's `a` must
// be BOUND by the first scope's λ.
func TestTheScopeIsSequential(t *testing.T) {
	outer := mustRead(t, `(build a n  b (len a)  (g a b))`)
	lam := outer.Kids[2]
	if lam.Kind != KFn {
		t.Fatalf("the outer scope's body is %s, want a λ", lam)
	}
	inner := lam.Kids[0]
	size := inner.Kids[1] // (len a)
	if size.Kind != KApp || len(size.Kids) != 2 || size.Kids[1].Kind != KBound {
		t.Errorf("in the second size, a is %v; want a bound variable of the first scope", size)
	}
	// And the first size sees nothing the scope binds.
	if n := outer.Kids[1]; n.Kind != KName || n.Name != "n" {
		t.Errorf("the first size is %v; want the free name n", n)
	}
}

// WHAT IS NOT THE BINDER FORM is left alone: the core form, and a target
// file's one-argument directive.
func TestTheCoreFormAndTheDirectiveAreUntouched(t *testing.T) {
	for _, src := range []string{`(build n f)`, `(build n (fn (b) b))`, `(build "go build -o %s %s")`} {
		got := mustRead(t, src)
		if got.Kind != KApp || got.Kids[0].Name != "build" || len(got.Kids) != len(mustRead(t, src).Kids) {
			t.Errorf("%s read as %s", src, got)
		}
	}
	if got := mustRead(t, `(build "go build -o %s %s")`); len(got.Kids) != 2 || got.Kids[1].Kind != KStr {
		t.Errorf("the directive read as %s", got)
	}
}

func TestTheBinderFormsRefusals(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`(build a n b m)`, "NAME SIZE pairs and ONE body"},
		{`(build-map m cap k 4)`, "NAME SIZE pairs and ONE body"},
		{`(build (f x) n body)`, "must be a name"},
		{`(build go.b n body)`, "a binder is a simple name"},
		{`(build a n  a m  body)`, "binds a twice"},
	} {
		_, err := ReadTerm(c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want a refusal containing %q, got %v", c.src, c.want, err)
		}
	}
}
