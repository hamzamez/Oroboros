package emit

import "testing"

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
	tg, err := LoadTarget("../targets/js.oro")
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
	for _, name := range []string{"portable-go", "js", "java"} {
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
