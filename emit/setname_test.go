package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// headerCheck loads a program in J on target, with the library, and runs the
// program-level check gen and build run.
func headerCheck(t *testing.T, target, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/" + target)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	return CheckSignatures(tg, prog, env, nil)
}

// A SET'S NAME DENOTES A SET (ADR 0073, spec/typevalues.md §1a): ⊢ S set is a
// scope check on S's atoms against Γ_T, decided on every header. An unbound
// name was an opaque type: `Array(Nonesuch)` printed Go's `[]/*Nonesuch?*/`.
func TestASetsNameDenotesASet(t *testing.T) {
	const main = "\nexport fn main() { 0 }\n"
	for what, c := range map[string]struct{ src, where string }{
		"an element":         {"export fn f(x ∈ Array(Nonesuch)) (Int) { len(x) }\n", "parameter x names Nonesuch"},
		"a parameter":        {"export fn g(x ∈ Nonesuch) (Int) { 0 }\n", "parameter x names Nonesuch"},
		"a result":           {"export fn h(x ∈ Int) (Nonesuch) { x }\n", "result names Nonesuch"},
		"never called":       {"fn k(x ∈ Nonesuch) (Int) { 0 }" + main, "parameter x names Nonesuch"},
		"a map's value":      {"fn m(x ∈ Map(Int, Nonesuch)) (Int) { 0 }" + main, "names Nonesuch"},
		"a product's factor": {"fn p(x ∈ Int) (Int × Nonesuch) { (x, x) }" + main, "result's component 2 names Nonesuch"},
		"misspelled":         {"fn q(x ∈ Strng) (Int) { 0 }" + main, "names Strng"},
		"a table's product":  {"fn r(x ∈ Array(Int × Nonesuch)) (Int) { 0 }" + main, "parameter x names Nonesuch"},
	} {
		err := headerCheck(t, "go", c.src)
		if err == nil || !strings.Contains(err.Error(), c.where) || !strings.Contains(err.Error(), "which is no set") {
			t.Errorf("%s: want a refusal naming %q, got %v", what, c.where, err)
		}
	}
	// THE NAMES THAT ARE SETS stay legal: the language's, the target's, a host
	// type through its alias, the formers, a variant result, ranges above the
	// word and an open end.
	for what, src := range map[string]string{
		"the base sets": "fn a(x ∈ Int, b ∈ Bool, s ∈ String, f ∈ F64, y ∈ Any) (Int) { 0 }" + main,
		"ranges":        "fn b(x ∈ 0..255, y ∈ -5..5) (0..) { x }" + main,
		"formers":       "fn c(x ∈ Array(0..255), y ∈ Array(F64), m ∈ Map(Int, Int)) (Int) { len(x) }" + main,
		"a buffer":      "fn d(b ∈ Array(F64) linear) (()) { () }" + main,
		"a host type":   "use go\nuse go/strings\nfn e(s ∈ go.bytestring, r ∈ strings.Replacer) (Int) { 0 }" + main,
		"a product":     "fn f(x ∈ 0..100) (0..50 × 0..1) { (x / 2, x % 2) }" + main,
		"a variant":     "use result\nfn g(n ∈ 0..10) (result.Result(Int, Int)) { result.'ok(n) }" + main,
	} {
		if err := headerCheck(t, "go", src); err != nil {
			t.Errorf("%s: a set is refused: %v", what, err)
		}
	}
	// A HOST SET THROUGH ITS IMPORT is the imported module's: ρ_m resolves the
	// alias as it does for a term, so the header holds the target's own name.
	// Unresolved, an export printed Go's `/*strings.Replacer?*/`.
	tg, _ := LoadTarget("../targets/go")
	forms, _ := core.Read("use go\nuse go/strings\nexport fn keep(r ∈ strings.Replacer) (strings.Replacer) { r }\n")
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	if s := prog.Sigs["keep"]; s == nil || s.Params[0].Type != "go/strings.Replacer" || s.Result != "go/strings.Replacer" {
		t.Errorf("an alias-qualified host set is the module's: %+v", s)
	}
	// A TARGET'S TYPE IS ITS TARGET'S: Java's `double_array` is no set on Go.
	if err := headerCheck(t, "go", "fn j(x ∈ double_array) (Int) { 0 }"+main); err == nil ||
		!strings.Contains(err.Error(), "not declared by target go") {
		t.Errorf("another target's type: %v", err)
	}
	if err := headerCheck(t, "java", "fn j(x ∈ double_array) (Int) { 0 }"+main); err != nil {
		t.Errorf("Java's own type: %v", err)
	}
}
