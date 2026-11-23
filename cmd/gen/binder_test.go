package main

import (
	"strings"
	"testing"

	"oroboros/core"
)

// NO BINDER OF A LANGUAGE NAME (ADR 0159). A symbol of Σ_L is an operation of
// the abstract syntax, not a variable: `1 + 2` is app(+; 1, 2) and no
// occurrence of a binder, so a binder of `+` or `if` would capture the
// construct in its scope. Every binder is a λ's in the core, so each form is
// refused: a λ's parameter, a definition's, a block's binding, a `var`, a loop
// variable, a pattern's. The witness: with the check planted away,
// `fn(`if`) { if c { 1 } else { 2 } }(fn(x, a, b) { 7 })` compiles to
// `return 7` whatever c is, the wrong answer defects-2026-10-26 found for a
// module's `if`.
func TestNoBinderOfALanguageName(t *testing.T) {
	const want = "names `if`, which is the language's own"
	forms := map[string]string{
		"a λ's parameter":     "export fn run(c ∈ Bool) (Int) { fn(`if`) { if c { 1 } else { 2 } }(fn(x, a, b) { 7 }) }\n",
		"a definition's":      "fn pick(`if`, c ∈ Bool) (Int) { if c { 1 } else { 2 } }\n\nexport fn run(c ∈ Bool) (Int) { pick(fn(x, a, b) { 7 }, c) }\n",
		"a block's binding":   "export fn run(c ∈ Bool) (Int) {\n    `if` = fn(x, a, b) { 7 }\n    if c { 1 } else { 2 }\n}\n",
		"a var":               "export fn run(c ∈ Bool) (Int) {\n    var `if` := 3\n    `if` := 4\n    if c { 1 } else { 2 }\n}\n",
		"a loop variable":     "export fn run(c ∈ Bool) (Int) {\n    loop `if` = 0 {\n        `if` < 1 -> again(1)\n        else -> 2\n    }\n}\n",
		"a claimed λ's":       "export fn run(c ∈ Bool) (Int) { fn(`if` ∈ Int) (Int) { 1 }(3) }\n",
		"a case arm's binder": "export fn run(k ∈ 0..9) (Int) {\n    m = map { 1 : 5 }\n    case m[k] {\n        'some(`if`) -> 1\n        'none -> 2\n    }\n}\n",
	}
	for name, src := range forms {
		_, err := surfaceGen(t, "go", map[string]string{"p.oro": src})
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s binding `if` should be refused saying %q, got %v", name, want, err)
		}
	}
	// `+` and `len`, an operator and a function of the language's
	for _, src := range []string{
		"export fn run(x ∈ 0..9) (Int) { fn(`+`) { x + 2 }(fn(a, b) { a * b }) }\n",
		"export fn run() (Int) {\n    len = 3\n    len + 1\n}\n",
	} {
		if _, err := surfaceGen(t, "go", map[string]string{"p.oro": src}); err == nil || !strings.Contains(err.Error(), "which is the language's own") {
			t.Errorf("%q should be refused as a binder of a language name: %v", src, err)
		}
	}
	// a name of the target's may be bound: `h` is no symbol of Σ_L
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": "export fn run(x ∈ 0..9) (Int) { fn(h) { h(x, 2) }(fn(a, b) { a + b }) }\n"}); err != nil {
		t.Errorf("a binder of a name that is not the language's should compile: %v", err)
	}
	core.PlantLanguageBinder = true
	code, err := surfaceGen(t, "go", map[string]string{"p.oro": forms["a λ's parameter"]})
	core.PlantLanguageBinder = false
	if err != nil || !strings.Contains(code, "return 7") {
		t.Errorf("with the check planted away the binder of `if` should capture it, `return 7`: %v\n%s", err, code)
	}
}
