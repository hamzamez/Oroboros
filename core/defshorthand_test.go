package core

import (
	"reflect"
	"strings"
	"testing"
)

// THE SHORTHAND IS AN EQUATION, AND THIS IS IT (docs/program-surface.md §2.1).
//
//	(def f (x…) body)  =  (def f (fn (x…) body))
//
// Sugar is admissible here when it erases in the reader and has a unique
// expansion, so the test is a COMMUTING SQUARE on what the reader produces: the
// two spellings must load to the same term, not merely to terms that behave
// alike. Anything downstream — the checker, the analyses, every backend — then
// cannot tell them apart by construction.
func TestTheShorthandIsTheLambdaItMeans(t *testing.T) {
	pairs := []struct{ short, long string }{
		{`(def sq (n) (* n n))`, `(def sq (fn (n) (* n n)))`},
		{`(def add (a b) (+ a b))`, `(def add (fn (a b) (+ a b)))`},
		{`(def main () (print 1))`, `(def main (fn () (print 1)))`},
		{`(def k (a b c) a)`, `(def k (fn (a b c) a))`},
	}
	for _, p := range pairs {
		s, err := Read(p.short)
		if err != nil {
			t.Fatalf("%s: %v", p.short, err)
		}
		l, err := Read(p.long)
		if err != nil {
			t.Fatalf("%s: %v", p.long, err)
		}
		if !reflect.DeepEqual(s, l) {
			t.Errorf("%s\n  reads as %v\n  want     %v", p.short, s[0].Term, l[0].Term)
		}
		if s[0].Term.Kind != KFn {
			t.Errorf("%s: the shorthand must produce a λ, got %v", p.short, s[0].Term.Kind)
		}
	}
}

// A DEFINITION BINDS A TERM, and the shorthand must not take that away: a def of
// three elements still means what it meant, whatever its body looks like.
func TestTheShorthandTakesNothingFromTheOldForm(t *testing.T) {
	for _, src := range []string{
		`(def cap 65536)`,             // a value
		`(def greeting "hi")`,         // a value
		`(def t (array 1 2 3))`,       // an application that is a table literal
		`(def x (g))`,                 // an application of one name
		`(def f (fn (x) x))`,          // the long spelling
		`(def c (fn (a) (fn (b) a)))`, // curried, which the shorthand does not cover
	} {
		forms, err := Read(src)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if len(forms) != 1 || forms[0].Kind != "def" {
			t.Fatalf("%s: got %v", src, forms)
		}
		again, err := Read(src)
		if err != nil || !reflect.DeepEqual(forms, again) {
			t.Errorf("%s: reading is not a function", src)
		}
	}
}

// THE DISCRIMINATOR (program-surface.md §8.3). A parameter list is a list of
// NAMES ONLY, so the shape several arities under one name would take — a list
// whose first element is a list — cannot be read as one. It is refused by NAME
// rather than by a generic message, because the whole point of fixing the rule
// now is that the next person reads it.
func TestASecondArityIsRefusedByName(t *testing.T) {
	_, err := Read(`(def f ((a) 1) ((a b) 2))`)
	if err == nil {
		t.Fatal("several arities under one name must be refused")
	}
	for _, want := range []string{"several arities", "program-surface.md", "list of names"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q, got: %v", want, err)
		}
	}
}

// ONE BODY. Scheme's `define` has an implicit `begin`; here that would delete a
// PURE leading expression in silence, because β is allowed to drop one. The
// refusal says so, and names the form that asks for the ordering.
func TestSeveralBodiesAreRefusedAndNameSeq(t *testing.T) {
	_, err := Read(`(def f (a) 1 2)`)
	if err == nil {
		t.Fatal("two body forms must be refused")
	}
	if !strings.Contains(err.Error(), "seq") || !strings.Contains(err.Error(), "ONE body") {
		t.Errorf("the refusal must offer seq: %v", err)
	}
}

// ONE PARAMETER LIST, ONE SET OF RULES. `fn` and the shorthand read the same
// object, so they refuse the same things — two implementations would be two
// rules that can disagree, which is a shape this repository has been bitten by.
func TestBothSpellingsRefuseTheSameParameterLists(t *testing.T) {
	cases := []struct{ short, long, want string }{
		{`(def f (a a) a)`, `(def f (fn (a a) a))`, "twice"},
		{`(def f (go.x) 1)`, `(def f (fn (go.x) 1))`, "qualifies a module member"},
	}
	for _, c := range cases {
		es, errShort := Read(c.short)
		_, errLong := Read(c.long)
		if errShort == nil {
			t.Errorf("%s: must be refused, got %v", c.short, es)
			continue
		}
		if errLong == nil {
			t.Errorf("%s: must be refused", c.long)
			continue
		}
		for _, e := range []error{errShort, errLong} {
			if !strings.Contains(e.Error(), c.want) {
				t.Errorf("want %q in %v", c.want, e)
			}
		}
	}
	// `(def f (a 1) 1)` is not a parameter list, so it is not the shorthand at
	// all: the message is the form's own.
	if _, err := Read(`(def f (a 1) 1)`); err == nil ||
		!strings.Contains(err.Error(), "def takes a name and one term") {
		t.Errorf("a non-parameter-list must fall back to the def message, got %v", err)
	}
}

// `()` IS A PARAMETER LIST AND NOT A TERM, and admitting it after `def` must not
// admit it anywhere else (core-0.md).
func TestAnEmptyListIsStillNotATerm(t *testing.T) {
	if _, err := Read(`(def f (fn (x) (x ())))`); err == nil ||
		!strings.Contains(err.Error(), "empty list is not a term") {
		t.Errorf("() must stay illegal as a term, got %v", err)
	}
	if _, err := Read(`(def main () 1)`); err != nil {
		t.Errorf("() must be a parameter list after def: %v", err)
	}
}
