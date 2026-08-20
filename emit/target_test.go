package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTargets(t *testing.T) {
	for _, name := range []string{"go", "js", "java", "blas"} {
		tg, err := LoadTarget("../targets/" + name + ".oro")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if tg.Name != name {
			t.Errorf("%s: name is %q", name, tg.Name)
		}
		if len(tg.Prims) == 0 {
			t.Errorf("%s: no primitives", name)
		}
		t.Logf("%-5s %2d primitives, %d types", name, len(tg.Prims), len(tg.Types))
	}
}

// JS declares no types at all, which is the point.
func TestJSTargetIsUntyped(t *testing.T) {
	tg, err := LoadTarget("../targets/portable-js.oro")
	if err != nil {
		t.Fatal(err)
	}
	if len(tg.Types) != 0 {
		t.Errorf("JS should declare no types, got %v", tg.Types)
	}
}

// The acceptance test for requirement 4: a host function is added by editing a
// target file and nothing else. `sqrt` exists in all three targets and in no Go
// source — if this fails, primitives have leaked back into the compiler.
//
// The name is qualified since arithmetic.md §5 moved it into num/f64, which is
// itself part of what this test asserts: a primitive moving between modules is
// a target-file edit and nothing more.
func TestHostFunctionIsDeclaredNotCompiledIn(t *testing.T) {
	for _, name := range []string{"portable-go", "portable-js", "portable-java"} {
		tg, err := LoadTarget("../targets/" + name + ".oro")
		if err != nil {
			t.Fatal(err)
		}
		p, ok := tg.Prims["num/f64.sqrt"]
		if !ok {
			t.Errorf("%s does not declare sqrt", name)
			continue
		}
		if p.Kind != "expr" || p.Form == "" {
			t.Errorf("%s: sqrt should be an expression with a template, got %+v", name, p)
		}
	}
	// Go's declaration carries the import; the others need none.
	tg, _ := LoadTarget("../targets/portable-go.oro")
	if tg.Prims["num/f64.sqrt"].Import != "math" {
		t.Errorf("Go's sqrt should declare import math, got %q", tg.Prims["num/f64.sqrt"].Import)
	}
}

// A target may be a DIRECTORY of files, merged — docs/spec/target-native.md.
// targets/go/ is three files: the header, Go's builtins, and the whole of fmt.
func TestTargetDirectoryMerges(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if tg.Name != "go" {
		t.Errorf("merged target is named %q", tg.Name)
	}
	for _, n := range []string{"go.+", "go.%", "go.&^", "go.len", "go.delete",
		"go/fmt.Println", "go/fmt.Sprintf", "loop", "if", "let"} {
		if _, ok := tg.Prims[n]; !ok {
			t.Errorf("merged target is missing %s", n)
		}
	}
	// The build command comes from the header file, the primitives from the
	// others; merging is a union across all three.
	if tg.Build == "" {
		t.Error("the header's build command was lost in the merge")
	}
	// fold-range is deliberately absent: `loop` subsumes it (ADR 0015).
	if _, ok := tg.Prims["fold-range"]; ok {
		t.Error("the native target should not declare fold-range")
	}
}

// All three native targets are directories with the SAME structural set of
// three — let, if, loop — and each declares its host's own names.
func TestNativeTargetsAreThreeStructural(t *testing.T) {
	for _, c := range []struct {
		dir   string
		names []string
	}{
		{"go", []string{"go.+", "go.%", "go/fmt.Println"}},
		{"js", []string{"js.+", "js.===", "js.??", "js/Math.floor", "js/console.log"}},
		{"java", []string{"java.+", "java.>>>", "java/Math.abs", "java/Map.get"}},
	} {
		tg, err := LoadTarget("../targets/" + c.dir)
		if err != nil {
			t.Errorf("%s: %v", c.dir, err)
			continue
		}
		for _, n := range append(c.names, "let", "if", "loop") {
			if _, ok := tg.Prims[n]; !ok {
				t.Errorf("%s is missing %s", c.dir, n)
			}
		}
		for _, gone := range []string{"fold-range", "fold-range2", "make-vec"} {
			if _, ok := tg.Prims[gone]; ok {
				t.Errorf("%s should not declare %s", c.dir, gone)
			}
		}
		// The four reserved type names must be spelled by every target that
		// uses integer or float literals and conditionals.
		for _, ty := range []string{"int", "f64", "bool"} {
			if _, ok := tg.Types[ty]; !ok {
				t.Errorf("%s does not spell the reserved type %s", c.dir, ty)
			}
		}
	}
}

// The language's own constructs are INJECTED into every target, and a target
// may neither decline nor declare one.
//
// `if` was already like this (ADR 0017). `let` and `loop` were not: every one
// of eleven target files declared them identically, so a third-party author
// could forget one and make an ADR 0015 language construct silently
// unavailable — a construct in the core that a target can decline, which is a
// library with a portability claim rather than part of the language.
func TestEveryTargetHasTheLanguagesConstructs(t *testing.T) {
	for _, name := range []string{"go", "js", "java", "windows", "blas.oro",
		"portable-go.oro", "portable-js.oro", "portable-java.oro",
		"tutorial.oro", "tutorial-native.oro", "tutorial-sloppy.oro"} {
		tg, err := LoadTarget("../targets/" + name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, want := range []struct{ n, k string }{
			{"if", "cond"}, {"let", "let"}, {"loop", "iterate"},
		} {
			p, ok := tg.Prims[want.n]
			if !ok {
				t.Errorf("%s: %s is not available", name, want.n)
			} else if p.Kind != want.k {
				t.Errorf("%s: %s has kind %q, want %q", name, want.n, p.Kind, want.k)
			}
		}
	}
}

// And declaring one is an error, not a redundancy. A target author who writes
// the line has misunderstood where the boundary is, and the message says so.
func TestDeclaringALanguageConstructIsAnError(t *testing.T) {
	for _, line := range []string{
		`(structural let  let     pure)`,
		`(structural loop iterate pure)`,
		`(structural if   cond    pure)`,
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "t.oro")
		src := "(target t\n  (type int \"int\")\n  " + line + ")\n"
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTarget(path)
		if err == nil || !strings.Contains(err.Error(), "belongs to the language") {
			t.Errorf("%s must be refused, got %v", line, err)
		}
	}
}
