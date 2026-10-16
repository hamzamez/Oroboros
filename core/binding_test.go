package core

import (
	"reflect"
	"strings"
	"testing"
)

// A BINDING IS THE TERM IT MEANS (docs/spec/binding.md; ADR 0054, a block is a
// scope).
//
// Sugar is admissible when it erases in the reader and has a unique expansion,
// so every positive test here is a COMMUTING SQUARE: the two spellings must
// read to the SAME term, not to terms that behave alike. Nothing downstream can
// then tell them apart by construction.
func TestABindingIsTheApplicationItMeans(t *testing.T) {
	for _, c := range []struct{ block, explicit string }{
		// one name
		{`{ x = 5; x + 1 }`, `fn(x) { x + 1 }(5)`},
		// SEQUENTIAL: the second value sees the first name
		{`{ x = 5; y = x + 1; x * y }`, `fn(x) { fn(y) { x * y }(x + 1) }(5)`},
		// three, to pin the nesting rather than a special case for two
		{`{ a = 1; b = 2; c = 3; a + (b + c) }`, `fn(a) { fn(b) { fn(c) { a + (b + c) }(3) }(2) }(1)`},
		// A TUPLE PATTERN IS THE PRODUCT'S ELIMINATOR, in binding order
		{`{ (s, e) = f(p); g(s, e) }`, `f(p)(fn(s, e) { g(s, e) })`},
		// mixed, and the pattern's names are in scope after it
		{`{ n = 1; (a, b) = f(n); a + b }`, `fn(n) { f(n)(fn(a, b) { a + b }) }(1)`},
		// `_` binds nothing, so a statement is an instance of the same rule
		{`{ _ = p(1); q(2) }`, `{ p(1); q(2) }`},
		// THE EMPTY CASE, forced by the identity (binding.md §4)
		{`{ 1 + 2 }`, `1 + 2`},
		// shadowing across bindings is nesting, and legal (def.md §11, ADR 0054)
		{`{ x = 1; x = x + 1; x }`, `fn(x) { fn(x) { x }(x + 1) }(1)`},
	} {
		got, err := ReadTerm(c.block)
		if err != nil {
			t.Fatalf("%s: %v", c.block, err)
		}
		want, err := ReadTerm(c.explicit)
		if err != nil {
			t.Fatalf("%s: %v", c.explicit, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s\n  read as %s\n  want    %s", c.block, got, want)
		}
	}
}

// A BINDER IS A SIMPLE NAME, and a pattern's names are distinct (W2): a
// qualified name is a module's member, and a repeated name's second binding
// would make the first unreachable.
func TestTheBindingFormRefusesWhatItCannotMean(t *testing.T) {
	for _, c := range []struct{ src, want string }{
		{`{ (a, a) = f(x); a }`, "binds a twice"},
		{`{ m.x = 5; m.x }`, ""},
		{`{ (m.a, b) = f(x); b }`, ""},
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

// `again` UNDER A BINDING — ADR 0015 permits it under a `let`, and a block's
// bindings are nested lets, so the rule reaches through a chain of them. A
// TUPLE pattern is a binding too (ADR 0027), and the chain may mix the two.
// What stays refused is a jump inside a λ the program wrote, whose body may run
// twice or never, and one under an `if`.
func TestAgainReachesThroughAChainOfBindings(t *testing.T) {
	for _, ok := range []string{
		`loop i = 0 { i < 10 -> { a = i + 1; b = a * 2; again(b) }; else -> i }`,
		`loop i = 0 { i < 10 -> { (a, b) = f(i); again(a) }; else -> i }`,
		`loop i = 0 { i < 10 -> { x = i + 1; (a, b) = f(x); y = a * 2; again(y) }; else -> i }`,
	} {
		if _, err := ReadTerm(ok); err != nil {
			t.Errorf("%s: `again` under a chain of bindings must be legal: %v", ok, err)
		}
	}
	for _, bad := range []struct{ src, why string }{
		{`loop i = 0 { i < 10 -> f(i, fn(a, b) { again(a) }); else -> i }`, "never a tail"},
		{`loop i = 0 { i < 10 -> f(i)(fn(a, b) { again(a) }); else -> i }`, "never a tail"},
		{`loop i = 0 { i < 10 -> { (a, b) = f(i); if a { again(a) } else { b } }; else -> i }`, "under an if"},
	} {
		if _, err := ReadTerm(bad.src); err == nil || !strings.Contains(err.Error(), bad.why) {
			t.Errorf("%s: want refused (%s), got %v", bad.src, bad.why, err)
		}
	}
	// THE MARK DOES NOT LEAK: what leaves the reader is the plain eliminator.
	got, err := ReadTerm(`{ (a, b) = f(1); g(a, b) }`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.String(), "#tuple-let") {
		t.Errorf("the reader's mark must be erased: %s", got)
	}
}
