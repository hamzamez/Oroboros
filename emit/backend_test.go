package emit

import (
	"strings"
	"testing"
)

// WHICH BACKEND COMPILES A TARGET IS THE TARGET'S ANSWER, NOT THE FLAG'S.
//
// `cmd/build` and `cmd/gen` switched on the `-target` FLAG STRING and fell
// through to the Go backend for any name they did not recognise. So a target
// directory named anything of its own was compiled by the wrong code generator,
// silently. Two live instances:
//
//   - a generated Windows target in a directory called `wintest` emitted Go
//     control flow with x86 templates spliced into it, and the first sign of it
//     was MASM refusing the file (win32-2026-09-06 §6);
//   - `targets/portable-js.oro`, in this repository since August, emitted
//     `package gauntlet` and `func GenDot(p /*vec-f64?*/, …)` — Go source from a
//     JavaScript target.
//
// The second one is why this is a test rather than a note: nothing had ever run
// `cmd/gen` against that target, so a latent miscompilation sat in the tree
// looking exactly like a working one.
func TestTheTargetSaysWhichBackendCompilesIt(t *testing.T) {
	for _, c := range []struct{ dir, want string }{
		{"../targets/go", "go"},
		{"../targets/js", "js"},
		{"../targets/java", "java"},
		{"../targets/windows", "x86-64"},
		{"../targets/portable-go.oro", "go"},
		{"../targets/portable-js.oro", "js"},
		{"../targets/portable-java.oro", "java"},
	} {
		tg, err := LoadTarget(c.dir)
		if err != nil {
			t.Fatalf("%s: %v", c.dir, err)
		}
		got, err := tg.ResolveBackend()
		if err != nil {
			t.Fatalf("%s: %v", c.dir, err)
		}
		if got != c.want {
			t.Errorf("%s: backend %q, want %q", c.dir, got, c.want)
		}
	}
}

// AND A TARGET THAT CANNOT SAY IS REFUSED, rather than defaulting.
//
// There is no default because a wrong default here is a silent miscompilation.
// Declaring nothing stays LEGITIMATE — a target is a capability set first, and
// one that only parameterises the normal form (ADR 0002) has nothing to emit
// with. `blas` is exactly that: it declares `cblas_ddot(…)`, which is C, and it
// exists to show reduction stopping at a different point under `cmd/oro`.
// Emitting from it is what is refused.
func TestATargetWithNoBackendIsRefusedForEmission(t *testing.T) {
	for _, dir := range []string{"../targets/blas.oro", "../targets/tutorial.oro"} {
		tg, err := LoadTarget(dir)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		got, err := tg.ResolveBackend()
		if err == nil {
			t.Fatalf("%s: resolved to %q; a target that does not name a backend and is "+
				"not named after one must be refused, not defaulted", dir, got)
		}
		// The message has to name the fix, because the person seeing it is
		// writing a target and has no other way to learn the set is closed.
		for _, want := range []string{"(backend NAME)", "x86-64"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%s: the refusal does not mention %q:\n%s", dir, want, err)
			}
		}
	}
}

// AN EMITTED FILE IS A FUNCTION OF ITS INPUT.
//
// Five lookups here searched `tg.Prims` — a Go map, whose range order is
// randomised — and returned the FIRST match, so a target that declares one
// operation twice resolved it differently per run. `targets/js/` declares
// `concat` three times (the language's `%s + %s`, `js/Array.concat` and
// `js/String.concat`), and six identical `cmd/gen` runs over
// `examples/big/render.oro` produced TWO different programs.
//
// That is worse than a cosmetic wobble: every "byte-identical across N programs
// × 4 targets" claim in `gauntlet/results/` assumes this cannot happen, and one
// of them was written while it was happening.
//
// The order is least-qualified first, then by name — principled rather than
// merely stable, because a name in the target's core module is the operation and
// the same name in a sub-module is a host API binding that shares it
// (overloading.md §3).
func TestSpellingLookupIsDeterministic(t *testing.T) {
	tg, err := LoadTarget("../targets/js")
	if err != nil {
		t.Fatal(err)
	}
	cs := tg.spelled("concat")
	if len(cs) < 2 {
		t.Skip("this target no longer declares `concat` more than once, so there is " +
			"no tie to break and this test proves nothing")
	}
	// The core module wins over a sub-module binding.
	if strings.Contains(cs[0].Name, "/") {
		var names []string
		for _, p := range cs {
			names = append(names, p.Name)
		}
		t.Fatalf("the first candidate is a sub-module binding: %v", names)
	}
	// And repeating the whole load resolves the same way, which is the property
	// the map iteration broke.
	first, _ := tg.findBySpelling("concat", 2)
	for i := 0; i < 20; i++ {
		tg2, err := LoadTarget("../targets/js")
		if err != nil {
			t.Fatal(err)
		}
		got, ok := tg2.findBySpelling("concat", 2)
		if !ok || got.Name != first.Name || got.Form != first.Form {
			t.Fatalf("run %d resolved `concat` to %q %q, first run gave %q %q",
				i, got.Name, got.Form, first.Name, first.Form)
		}
	}
}
