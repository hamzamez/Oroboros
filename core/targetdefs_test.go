package core

import (
	"strings"
	"testing"
)

// noLibraries is a resolver that finds no file, so every `use` is unresolved —
// which is how a target-provided module looks.
func noLibraries(string) (string, bool, error) { return "", false, nil }

// `D_T` — THE TARGET'S OWN DEFINITIONS, target-system.md §6.2.
//
// A target has always been able to say how a host SPELLS a name. `D_T` lets it
// say that the host DEFINES one, in Oroboros. Selection is the same `▷` used
// everywhere else, `P_T ▷ D_T ▷ D`, and nothing in the reducer changes: δ
// unfolds a target's definition exactly as it unfolds a library's.
func TestATargetsDefinitionsAreUnfoldedByDelta(t *testing.T) {
	src, prims := splitPrims(`
		(target go (prim add))
		(use h)
		(h.twice 3)
	`, "go")
	forms, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	dtTwice, err := ReadTerm(`(fn (x) (add x x))`)
	if err != nil {
		t.Fatal(err)
	}
	dt := TargetDefs{"h": {{Kind: "def", Name: "twice", Term: dtTwice}}}
	prog, terms, err := LoadWithDefs(forms, noLibraries, dt)
	if err != nil {
		t.Fatal(err)
	}
	// A `use` that found no file is exactly how a target-provided module looks
	// (modules.md §4) — and now the target really does provide it.
	got, err := Normalize(terms[0], testEnv(prog, prims...), DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "(add 3 3)" {
		t.Errorf("got %s, want (add 3 3)", got)
	}
	// AND WITHOUT IT THE NAME IS UNBOUND, which is the control: a test whose
	// passing and failing cases look alike proves nothing.
	prog2, terms2, err := LoadWith(forms, noLibraries)
	if err != nil {
		t.Fatal(err)
	}
	if err := testEnv(prog2, prims...).CheckProgram(terms2); err == nil ||
		!strings.Contains(err.Error(), "h.twice") {
		t.Errorf("want h.twice unbound without D_T, got %v", err)
	}
}

// A MODULE THE PROGRAM NEVER IMPORTS GETS NOTHING. Covering is demand-driven, so
// a `provides` for a module nobody uses contributes no more than a `prim` for one
// does — and injecting it anyway put every library's host binding into every
// program, where `CheckProgram` reads EVERY definition and refused `hello.oro`
// for a name in tally's Go binding.
func TestATargetsDefinitionsForAnUnusedModuleAreNotInTheProgram(t *testing.T) {
	src, prims := splitPrims(`
		(target go (prim add))
		(add 1 2)
	`, "go")
	forms, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ReadTerm(`(fn (x) (nowhere.g x))`)
	if err != nil {
		t.Fatal(err)
	}
	dt := TargetDefs{"h": {{Kind: "def", Name: "f", Term: body}}}
	prog, terms, err := LoadWithDefs(forms, noLibraries, dt)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := prog.Defs["h.f"]; ok {
		t.Error("a module the program does not use must contribute nothing")
	}
	if err := testEnv(prog, prims...).CheckProgram(terms); err != nil {
		t.Errorf("an unused target definition must not refuse the program: %v", err)
	}
}

// `▷`: the target's definition wins over the library's, and the program says so.
func TestATargetsDefinitionOverridesTheLibrarys(t *testing.T) {
	lib := map[string]string{"h": "(module h)\n(def twice (fn (x) (add x 0)))"}
	resolve := func(p string) (string, bool, error) { s, ok := lib[p]; return s, ok, nil }
	src, prims := splitPrims(`
		(target go (prim add))
		(use h)
		(h.twice 3)
	`, "go")
	forms, err := Read(src)
	if err != nil {
		t.Fatal(err)
	}
	body, err := ReadTerm(`(fn (x) (add x x))`)
	if err != nil {
		t.Fatal(err)
	}
	dt := TargetDefs{"h": {{Kind: "def", Name: "twice", Term: body}}}
	prog, terms, err := LoadWithDefs(forms, resolve, dt)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Normalize(terms[0], testEnv(prog, prims...), DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "(add 3 3)" {
		t.Errorf("the target's definition must win; got %s", got)
	}
	if len(prog.TargetDefined) != 1 || prog.TargetDefined[0] != "h.twice" {
		t.Errorf("the override must be recorded so the build can say so; got %v", prog.TargetDefined)
	}
}
