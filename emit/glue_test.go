package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A TARGET IS THE GLUE OF ITS LAYERS (ADR 0093, target-system.md §2.1). `glue
// base` makes the base's fragments this target's: a header with no body is an
// operation symbol, completed by a definition under the same header; two
// headers that disagree, a header nothing defines, a cycle and an LLVM target
// with no triple are each refused, naming why.
func TestATargetIsTheGlueOfItsLayers(t *testing.T) {
	write := func(dir, name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(dir, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name, name+".oro"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	const base = "backend llvm\nset Int = host \"i64\"\nrepr -2^63..2^63-1 word\n" +
		"module base/os {\n    fn tick(n ∈ 0..9) (Int)\n}\n"
	load := func(member string) error {
		t.Helper()
		dir := t.TempDir()
		write(dir, "base", base)
		write(dir, "member", member)
		_, err := LoadTarget(filepath.Join(dir, "member"))
		return err
	}
	const triple = "glue base\ntriple \"x86_64-pc-windows-msvc\"\n"
	if err := load(triple + "module base/os {\n    fn tick(n ∈ 0..9) (Int) { n + 1 }\n}\n"); err != nil {
		t.Errorf("a member that defines the base's header under the same header: want a model, got %v", err)
	}
	for _, c := range []struct{ what, member, reason string }{
		{"a header defined under another header", triple + "module base/os {\n    fn tick(n ∈ 0..99) (Int) { n + 1 }\n}\n", "headers disagree"},
		{"a header nothing defines", triple, "requires base/os.tick"},
		{"no triple", "glue base\nmodule base/os {\n    fn tick(n ∈ 0..9) (Int) { n + 1 }\n}\n", "names no `triple`"},
		{"a cycle", "glue member\n" + triple, "cycle"},
	} {
		err := load(c.member)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}

	// THE PLANTED FAULT: two headers of one name glued without comparing them.
	PlantGlueFault = true
	defer func() { PlantGlueFault = false }()
	if err := load(triple + "module base/os {\n    fn tick(n ∈ 0..99) (Int) { n + 1 }\n}\n"); err != nil && strings.Contains(err.Error(), "headers disagree") {
		t.Errorf("with the headers not compared the disagreement should pass, so the test reaches the rule; got %v", err)
	}
}
