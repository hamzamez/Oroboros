package core

import (
	"strings"
	"testing"
)

// TWO PAYLOADS IN SCOPE ARE TWO NAMES (the book's finding 53, and 76 through
// `try`). A `case` renames its pattern variable to its slot's binder; a `case`
// inside a clause, already expanded, binds the same compiler spellings, and the
// outer payload renamed under the inner binder is a term no source can write.
// Every pass that opens a λ and closes it again by its hints then captures it:
// `10 * x + y` reduced to `10 * b + b`. The binders are now the first
// generation the clause bodies do not mention (Barendregt's convention, ADR
// 0036), so the normal forms below are the program's meaning.
func TestTwoPayloadsInScopeAreTwoNames(t *testing.T) {
	const decl = "use go\nset Maybe(T) = 'just(T) ∪ 'nothing\n" +
		"set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok\n"
	reduce := func(body string) string {
		t.Helper()
		p, err := loadSrc(t, decl+"fn f(a, b) {\n"+body+"\n}\n")
		if err != nil {
			t.Fatal(err)
		}
		nf, err := Normalize(p.Defs["f"], testEnv(p, "go.+", "go.*", "if", "=", "try", "loop"), DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		return nf.String()
	}
	cases := []struct{ what, body, want string }{
		{"the chapter's nesting: the outer payload under the inner case",
			"case 'just(a) { 'nothing -> 0; 'just(x) -> case 'just(b) { 'just(y) -> go.`+`(go.`*`(10, x), y); 'nothing -> 0 } }",
			"(go.+ (go.* 10 a) b)"},
		{"the inner payload read first",
			"case 'just(a) { 'just(x) -> case 'just(b) { 'just(y) -> go.`+`(go.`*`(10, y), x); 'nothing -> 0 }; 'nothing -> 0 }",
			"(go.+ (go.* 10 b) a)"},
		{"three deep, every payload read at the bottom",
			"case 'just(a) { 'just(x) -> case 'just(b) { 'just(y) -> case 'just(go.`+`(a, b)) { 'just(z) -> go.`+`(go.`+`(x, go.`*`(10, y)), go.`*`(100, z)); 'nothing -> 0 }; 'nothing -> 0 }; 'nothing -> 0 }",
			"(go.+ (go.+ a (go.* 10 b)) (go.* 100 (go.+ a b)))"},
		{"through try, two results joined (finding 76)",
			"try 'ok(x) = 'ok(a) { try 'ok(y) = 'ok(b) { 'ok(go.`+`(x, y)) } }",
			""},
	}
	for _, c := range cases {
		got := reduce(c.body)
		switch {
		case c.want != "" && got != "(fn (a b) "+c.want+")":
			t.Errorf("%s: got %s, want %s", c.what, got, c.want)
		case c.want == "" && !strings.Contains(got, "(go.+ a b)"):
			t.Errorf("%s: got %s, want the sum of both payloads, (go.+ a b)", c.what, got)
		}
	}

	// THE PLANTED FAULT, every case given the generation-0 spellings: the
	// chapter's program must compute the captured answer again, or this
	// witness cannot fail.
	PlantCaseBinderCaptures = true
	defer func() { PlantCaseBinderCaptures = false }()
	if got := reduce(cases[0].body); got != "(fn (a b) (go.+ (go.* 10 b) b))" {
		t.Errorf("with the binders unrenamed the capture should return, 10 * b + b; got %s", got)
	}
}

// TRY WITH SEVERAL BINDS IS NESTED (j.md §2.8, the book's finding 77): bind is
// associative, so try b₁, …, bₙ { e } = try b₁ { try b₂, …, bₙ { e } }, each
// bind seeing the names bound before it. The comma form and its nesting reduce
// to one normal form, for two independent binds and for a chain whose second
// bind reads the first's name. THE PLANTED FAULT nests them last-first, so the
// chain's second bind reads a name not yet bound.
func TestTryWithSeveralBindsIsNested(t *testing.T) {
	const decl = "use go\nset Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok\n" +
		"fn half(n) { if go.`=`(go.`%`(n, 2), 0) { 'ok(go.`/`(n, 2)) } else { 'err(n) } }\n"
	reduce := func(body string) (string, error) {
		t.Helper()
		p, err := loadSrc(t, decl+"fn f(a, b) {\n"+body+"\n}\n")
		if err != nil {
			return "", err
		}
		nf, err := Normalize(p.Defs["f"], testEnv(p, "go.+", "go.%", "go./", "go.=", "if", "=", "try", "loop"), DefaultFuel)
		if err != nil {
			return "", err
		}
		return nf.String(), nil
	}
	for what, c := range map[string][2]string{
		"two independent binds": {"try 'ok(x) = half(a), 'ok(y) = half(b) { 'ok(go.`+`(x, y)) }",
			"try 'ok(x) = half(a) { try 'ok(y) = half(b) { 'ok(go.`+`(x, y)) } }"},
		"a chain": {"try 'ok(x) = half(a), 'ok(y) = half(x) { half(y) }",
			"try 'ok(x) = half(a) { try 'ok(y) = half(x) { half(y) } }"},
	} {
		comma, err1 := reduce(c[0])
		nested, err2 := reduce(c[1])
		if err1 != nil || err2 != nil || comma != nested {
			t.Errorf("%s: the comma form and its nesting differ:\n  %s %v\n  %s %v", what, comma, err1, nested, err2)
		}
	}
	PlantTryCommaReversed = true
	defer func() { PlantTryCommaReversed = false }()
	nested, _ := reduce("try 'ok(x) = half(a) { try 'ok(y) = half(x) { half(y) } }")
	if got, err := reduce("try 'ok(x) = half(a), 'ok(y) = half(x) { half(y) }"); err == nil && got == nested {
		t.Error("with the binds nested last-first the chain should not reduce as nested, so the test reaches the rule")
	}
}
