package core

import (
	"reflect"
	"testing"
)

// TYPE ARGUMENTS ON VARIANT TYPES (spec/data.md §5.5.3–§5.5.5).
//
// A declaration `(variant (F T…) (c P)…)` is a type constructor F : Typeⁿ → Type,
// an instance is the substitution σ = [A⃗/T⃗] applied to every payload, and it
// is identified APPLICATIVELY by the declaration and its arguments. Arguments
// are never inferred: inside a program reduction erases every variant, and at a
// boundary the signature writes them, which is the only place one is needed.

func TestAnAppliedTypeHasOneCanonicalSpelling(t *testing.T) {
	for src, want := range map[string]string{
		"(result int string)":         "result(int, string)",
		"(result (option int) int)":   "result(option(int), int)",
		"(result (int 0 255) string)": "result(int 0 255, string)",
		"(int int)":                   "", // `int` is the type language's own head
		"(result (buffer int) int)":   "", // ADR 0020 rule 6
	} {
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := TypeTerm(forms[0].Term); got != want {
			t.Errorf("%s spells %q, want %q", src, got, want)
		}
	}
	// THE AMBIGUITY WITNESS: `(p string)` is a named parameter wherever a
	// parameter list is read, so TypeName, which reads parameter lists, must not
	// take it for `p` applied to `string`. Admitting it there renamed every
	// named parameter and broke every refinement keyed by one.
	if forms, _ := Read("(p string)"); TypeName(forms[0].Term) != "" {
		t.Errorf("TypeName read the named parameter (p string) as an applied type")
	}
	head, args, ok := Applied("result(option(int), int 0 255)")
	if !ok || head != "result" || !reflect.DeepEqual(args, []string{"option(int)", "int 0 255"}) {
		t.Errorf("an argument may itself be applied or contain spaces: %q %q %v", head, args, ok)
	}
	if _, _, ok := Applied("prod(int, int)"); ok {
		t.Error("a tuple is not an applied variant type")
	}
}

// AN INSTANCE AT A BOUNDARY is its tag and its substituted payload — k = 1, the
// representation every existing sum already has, so nothing new is emitted.
func TestAnInstanceAtABoundaryIsItsTagAndItsPayload(t *testing.T) {
	p, err := loadSrc(t, `(variant (result T) (ok T) (err T))
(sig pick ((n int)) (result string))
(def pick (fn (n) (ok "x")))
(sig find ((n int)) (option int))
(def find (fn (n) (some n)))
`)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]string{"pick": {"int", "string"}, "find": {"int", "int"}} {
		if got := p.Sigs[name].Results; !reflect.DeepEqual(got, want) {
			t.Errorf("%s returns %v, want %v", name, got, want)
		}
	}
}

func TestTypeArgumentsAreWrittenAndCounted(t *testing.T) {
	decl := "(variant (result T E) (ok T) (err E))\n(def f (fn (n) (ok n)))\n"
	mustLoadFail(t, decl+"(sig f ((n int)) result)", "write (result T E) with its arguments")
	mustLoadFail(t, decl+"(sig f ((n int)) (result int))", "takes 2 type arguments, not 1")
	mustLoadFail(t, "(def f (fn (n) n))\n(sig f ((n int)) (nothing int))",
		"not a parameterised variant type in scope")
	mustLoadFail(t, decl+"(sig g ((r (result int int))) int)\n(def g (fn (r) 0))",
		"not built")
	// k > 1 is Theorem R's encoding, specified and named as a limitation.
	mustLoadFail(t, decl+"(sig f ((n int)) (result int string))", "compiler limitation")
}

// §5.5.4, each rule the consequence of one stated elsewhere.
func TestAParameterisedDeclarationIsWellFormed(t *testing.T) {
	mustLoadFail(t, `(variant (r T T) (ok T) (err T))`, "declared twice")
	mustLoadFail(t, `(variant (r r) (ok r) (err int))`, "the type's own name")
	mustLoadFail(t, `(variant (r (F int)) (ok int) (err int))`, "higher-kinded")
	mustLoadFail(t, `(variant (r T E) (ok T) (err T))`, "phantom")
	mustLoadFail(t, `(variant tree leaf (node tree))`, "recursive type")
}

// Inside a program the argument is never needed: a parameterised variant reduces
// exactly as an unparameterised one does.
func TestAParameterisedVariantVanishesInsideAProgram(t *testing.T) {
	got := reduceTo(t, `
		(use go)
		(variant (result T E) (ok T) (err E))
		(def f (fn (n) (case (if (go.> n 0) (ok n) (err 0)) (ok v) (go.+ v 1) (err e) e)))`, "f")
	if got != "(fn (n) (if (go.> n 0) (go.+ n 1) 0))" {
		t.Errorf("a parameterised variant must leave nothing behind: %s", got)
	}
}
