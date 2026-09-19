package core

import (
	"reflect"
	"strings"
	"testing"
)

// THE FLAT BINDING FORM IS THE TERM IT MEANS (docs/spec/binding.md).
//
// Sugar is admissible when it erases in the reader and has a unique expansion,
// so every positive test here is a COMMUTING SQUARE: the two spellings must
// read to the SAME term, not to terms that behave alike. Nothing downstream can
// then tell them apart by construction, which is why the emission baseline is
// the acceptance for the corpus rewrite.
func TestABindingIsTheApplicationItMeans(t *testing.T) {
	for _, c := range []struct{ flat, explicit string }{
		// one name
		{`(let x 5 (+ x 1))`, `((fn (x) (+ x 1)) 5)`},
		// SEQUENTIAL: the second value sees the first name
		{`(let x 5 y (+ x 1) (* x y))`, `((fn (x) ((fn (y) (* x y)) (+ x 1))) 5)`},
		// three, to pin the nesting rather than a special case for two
		{`(let a 1 b 2 c 3 (+ a (+ b c)))`, `((fn (a) ((fn (b) ((fn (c) (+ a (+ b c))) 3)) 2)) 1)`},
		// A TUPLE PATTERN IS THE PRODUCT'S ELIMINATOR, in binding order
		{`(let (tuple s e) (f p) (g s e))`, `((f p) (fn (s e) (g s e)))`},
		// mixed, and the pattern's names are in scope after it
		{`(let n 1 (tuple a b) (f n) (+ a b))`, `((fn (n) ((f n) (fn (a b) (+ a b)))) 1)`},
		// `_` is an ordinary name, so `seq` is an instance of the same rule
		{`(let _ (p 1) (q 2))`, `(seq (p 1) (q 2))`},
		// THE EMPTY CASE, forced by the identity (binding.md §4)
		{`(let (+ 1 2))`, `(+ 1 2)`},
		// shadowing across bindings is nesting, and legal (def.md §11)
		{`(let x 1 x (+ x 1) x)`, `((fn (x) ((fn (x) x) (+ x 1))) 1)`},
	} {
		got, err := ReadTerm(c.flat)
		if err != nil {
			t.Fatalf("%s: %v", c.flat, err)
		}
		want, err := ReadTerm(c.explicit)
		if err != nil {
			t.Fatalf("%s: %v", c.explicit, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s\n  read as %s\n  want    %s", c.flat, got, want)
		}
	}
}

// AND THE OLD SPELLING IS REFUSED NAMING THE NEW ONE, as `sum`→`variant` and
// `values`→`tuple` were (data.md §10). Two spellings of one construct is the
// shape that rule exists to refuse, so this is not a deprecation.
func TestTheBindingFormRefusesWhatItCannotMean(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		// the old form, and a body someone forgot: one shape, one message
		{"(let (f x) (fn (v) (g v)))", "the old `(let VALUE (fn (x) …))` spelling is refused"},
		{`(let x 5)`, "(let x VALUE BODY)"},
		// a pair with no body
		{`(let x 5 y 6)`, "NAME VALUE pairs and ONE body"},
		// a left-hand side that is neither a name nor a tuple pattern
		{`(let (f x) 5 x)`, "belongs in `case`"},
		{`(let 5 5 x)`, "belongs in `case`"},
		// a nested pattern is a second let
		{`(let (tuple a (tuple b c)) (f x) a)`, "write a second `let` for a nested one"},
		// a tuple of one, refused by the constructor's own rule
		{`(let (tuple a) (f x) a)`, "a tuple of one is just the value"},
		// a repeated name IN ONE PATTERN: the second binding is unreachable
		{`(let (tuple a a) (f x) a)`, "may not repeat a name"},
		// a binder is a simple name, on both left-hand sides
		{`(let m.x 5 m.x)`, "qualifies a module member"},
		{`(let (tuple m.a b) (f x) b)`, "qualifies a module member"},
		// and `seq` still sequences two or more
		{`(seq (p 1))`, "seq takes two or more terms"},
	} {
		got, err := ReadTerm(c.src)
		if err == nil {
			t.Errorf("%s: must be refused, read as %s", c.src, got)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: the refusal must say %q, got: %v", c.src, c.want, err)
		}
	}
}

// `again` UNDER A BINDING — ADR 0015 permits it under a `let`, and the flat
// form is nested one-name lets, so the rule reaches through a chain of them.
// Under a TUPLE pattern it is refused: that desugars to an application of the
// producing call, so the jump would sit inside a host call's continuation, and
// no backend emits that.
func TestAgainReachesThroughAChainOfBindings(t *testing.T) {
	ok := `(loop ((i 0))
	         (< i 10) (let a (+ i 1) b (* a 2) (again b))
	         else i)`
	if _, err := ReadTerm(ok); err != nil {
		t.Errorf("`again` under a chain of bindings must be legal: %v", err)
	}
	bad := `(loop ((i 0))
	          (< i 10) (let (tuple a b) (f i) (again a))
	          else i)`
	if _, err := ReadTerm(bad); err == nil {
		t.Error("`again` under a tuple binding must be refused (binding.md §7)")
	} else if !strings.Contains(err.Error(), "sit under a `let`") {
		t.Errorf("the refusal must be the existing one, got: %v", err)
	}
}

// A CONTROL AGAINST THE TEST SUITE PASSING VACUOUSLY: a λ the programmer wrote
// is NOT a pattern, however much it looks like the desugared `tuple`. The
// discriminator is the binder `#k`, which no source term can contain.
func TestAWrittenLambdaIsNotAPattern(t *testing.T) {
	if _, err := ReadTerm(`(let (fn (k) (k a b)) (f x) a)`); err == nil {
		t.Error("a written λ on the left must be refused, not read as a pattern")
	}
}
