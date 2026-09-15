package core

import (
	"reflect"
	"strings"
	"testing"
)

// A VARIANT TYPE IS ITS DECLARATION, AND RESOLUTION MUST BE INJECTIVE ON THEM
// (spec/data.md §5.5.1–§5.5.2).
//
// `Load` kept every module's sums in one table keyed by the bare name. That key
// is resolution composed with forgetting the module, which is not injective, so
// two modules each declaring `result` collided and the later OVERWROTE the
// earlier. Measured 2026-09-15 on Go:
//
//	module a before module b:  func GenPick(n int) (int, string)
//	module b before module a:  func GenPick(n int) (int, int)
//
// Our checker accepted both. The interim rule refused two different sums with
// one name; with the key qualified, they are simply two types and both load.
//
// THE PROPERTY IS LOAD-ORDER INDEPENDENCE: Load(a ++ b) and Load(b ++ a) give
// every signature the same meaning. It fails against a bare key in one order.
func TestTwoModulesMayDeclareDifferentSumsWithOneName(t *testing.T) {
	a := `(module a)
(variant result (ok int) (err int))
(export pick)
(sig pick ((n int)) result)
(def pick (fn (n) (if (< n 0) (err 0) (ok n))))
`
	b := `(module b)
(variant result (ok string) (err string))
(export label)
(sig label ((n int)) result)
(def label (fn (n) (err "none")))
`
	want := map[string][]string{"a.pick": {"int", "int"}, "b.label": {"int", "string"}}
	for _, c := range []struct{ order, src string }{
		{"a before b", a + b},
		{"b before a", b + a},
	} {
		p, err := loadSrc(t, c.src)
		if err != nil {
			t.Errorf("%s: two variant types named result in two modules are two types: %v",
				c.order, err)
			continue
		}
		for name, results := range want {
			if got := p.Sigs[name].Results; !reflect.DeepEqual(got, results) {
				t.Errorf("%s: %s returns %v, want %v — its variant type was taken from the "+
					"other module", c.order, name, got, results)
			}
		}
	}
}

// A root module's variant was CAPTURED by a module that never imported it: the
// pattern found `ok` in the global table, and its tag `ok#tag` then resolved by
// δ to the root's definition, because the root's names are unqualified.
func TestAPatternDoesNotReachAnUnimportedModule(t *testing.T) {
	mustLoadFail(t, `(variant result (ok int) (err int))
(module b)
(def f (fn (r) (case r (ok v) v (err e) 0)))
`, "declared by result")
}

// Imported, but written unqualified: the pattern was accepted and its tag left
// FREE, so the residual was `(if (= 0 ok#tag) n 0)` with nothing to say why.
func TestAnImportedConstructorIsWrittenThroughItsAlias(t *testing.T) {
	mustLoadFail(t, `(module a)
(variant result (ok int) (err int))
(module b)
(use a)
(def f (fn (n) (case (a.ok n) (ok v) v (err e) 0)))
`, "import that module and write the pattern through its alias")
}

// And the qualified spelling, which is the one that means what it says, was the
// one refused. It now reduces exactly as a local variant does: to nothing.
func TestAQualifiedPatternEliminatesAnImportedVariant(t *testing.T) {
	p, err := loadSrc(t, `(module a)
(variant result (ok int) (err int))
(module b)
(use a)
(def f (fn (n) (case (a.ok n) (a.ok v) v (a.err e) 0)))
`)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := Normalize(p.Defs["b.f"], testEnv(p, "if", "="), DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if got := nf.String(); got != "(fn (n) n)" {
		t.Errorf("a static variant from another module must leave nothing behind: %s", got)
	}
}

// The tag is exported with its constructor, and only with it.
func TestAPatternRespectsTheExportList(t *testing.T) {
	mustLoadFail(t, `(module a)
(variant result (ok int) (err int))
(export ok)
(module b)
(use a)
(def f (fn (n) (case (a.ok n) (a.ok v) v (a.err e) 0)))
`, "does not export it")
}

// Two declarations with one spelling are two types, so one `case` may not mix
// them — and the refusal names them by their qualified names, since the
// spelling alone is exactly what cannot tell them apart.
func TestOneCaseDoesNotMixTwoTypesWithOneSpelling(t *testing.T) {
	_, err := loadSrc(t, `(module a)
(variant result (ok int) (err int))
(module b)
(use a)
(variant result (ok int) (err int))
(def f (fn (r) (case r (a.ok v) v (err e) 0)))
`)
	if err == nil {
		t.Fatal("a case on a.result and b.result together must be refused")
	}
	for _, want := range []string{"a.result", "b.result", "one variant type"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal must name %q; got: %v", want, err)
		}
	}
}

// `option` IS ONE DECLARATION, IN `lang` (spec/data.md §5.5.1). Every module
// held a copy — `a.some`, `b.some`, … — reconciled by name wherever two met.
// Now the program holds the constructors once, under the bare names the
// compiler's own map reads already produce, and no module holds any.
func TestOptionIsOneDeclaration(t *testing.T) {
	p, err := loadSrc(t, `(module a)
(export f)
(def f (fn (n) (case (some n) (some v) v none 0)))
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"some", "none", "some#tag", "none#tag"} {
		if _, ok := p.Defs[n]; !ok {
			t.Errorf("lang's %s must be in the program once, unqualified", n)
		}
		if _, ok := p.Defs["a."+n]; ok {
			t.Errorf("module a holds its own copy a.%s; option is one declaration", n)
		}
	}
}

// A module's own declaration SHADOWS the language's, as any inner name shadows
// an outer one (§5.5.2) — and the compiler's `some` from a map read is still
// the language's, because the module's `option` has no `some`.
func TestAModuleMayShadowOption(t *testing.T) {
	p, err := loadSrc(t, `(module a)
(variant option (yes int) no)
(export f)
(def f (fn (n) (+ (case (yes n) (yes v) v no 0)
                  (case ((map (1 10)) 1) (some v) v none 0))))
`)
	if err != nil {
		t.Fatalf("a module may declare its own option: %v", err)
	}
	nf, err := Normalize(p.Defs["a.f"], testEnv(p, "map", "if", "=", "+"), DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if got := nf.String(); got != "(fn (n) (+ n 10))" {
		t.Errorf("both options must eliminate, each by its own declaration: %s", got)
	}
}

// The main module's names are unqualified, like `lang`'s, so the two would be
// one name there: refused, where a (module …) would shadow.
func TestTheMainModuleCannotRedeclareTheLanguagesOwn(t *testing.T) {
	mustLoadFail(t, `(variant option (yes int) no)`, "which is the language's own")
	mustLoadFail(t, `(def some 3)`, "which is the language's own")
}

// THE CONTROL: a two-module program eliminates the one option in both.
func TestOptionIsOneTypeInEveryModule(t *testing.T) {
	src := `(module a)
(export f)
(def f (fn (n) (case (some n) (some v) v none 0)))

(module b)
(export g)
(def g (fn (n) (case none (some v) v none n)))
`
	if _, err := loadSrc(t, src); err != nil {
		t.Errorf("option in two modules must load: %v", err)
	}
}
