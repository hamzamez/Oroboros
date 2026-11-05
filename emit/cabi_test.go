package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A C FUNCTION IS ITS PROTOTYPE (ADR 0094). `host expr call "F"` declares one
// by its symbol, and its sets are its C types: a narrow integer's full range, the
// word, `ptr`, or a C string carrying `where nulfree`. Anything else is refused
// when the target loads, naming why.
func TestACFunctionIsItsPrototype(t *testing.T) {
	load := func(decl string) error {
		t.Helper()
		dir := t.TempDir()
		src := "backend llvm\ntriple \"x86_64-pc-windows-msvc\"\nset Int = host \"i64\"\nrepr -2^63..2^63-1 word\n" +
			"set ptr = host \"ptr\"\nset String = host \"ptr\"\nmodule k {\n" + decl + "\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "t.oro"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTarget(filepath.Join(dir, "t.oro"))
		return err
	}
	for what, decl := range map[string]string{
		"an int, a DWORD, a pointer and a SIZE_T":     `    fn F(a ∈ -2^31..2^31-1, b ∈ 0..2^32-1, c ∈ ptr, d ∈ 0..2^63-1) (r ∈ -2^15..2^15-1) { host expr call "F" stdcall }`,
		"a C string with its obligation":              `    fn S(s ∈ String) (r ∈ 0..2^8-1) where nulfree(s) { host expr call "S" }`,
		"a void function":                             `    fn V(a ∈ 0..2^32-1) (0..2^32-1) { host stmt call "V" cdecl }`,
		"a C string among other obligations, by name": `    fn T(n ∈ 0..2^32-1, s ∈ String) (ptr) where n >= 1 && nulfree(s) { host expr call "T" }`,
	} {
		if err := load(decl); err != nil {
			t.Errorf("%s: want it loaded, got %v", what, err)
		}
	}
	for _, c := range []struct{ what, decl, reason string }{
		{"a narrow domain, not its type's range", `    fn F(a ∈ 0..9) (ptr) { host expr call "F" }`, "no C type"},
		{"a Boolean", `    fn F(a ∈ Bool) (ptr) { host expr call "F" }`, "no C type"},
		{"a result that is no C type", `    fn F(a ∈ ptr) (r ∈ 0..99) { host expr call "F" }`, "no C type it can return"},
		{"a C string without nulfree", `    fn S(s ∈ String) (ptr) { host expr call "S" }`, "needs `where nulfree(s)`"},
	} {
		err := load(c.decl)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}

	// THE PLANTED FAULT: a C string's obligation not required.
	PlantCStringFault = true
	defer func() { PlantCStringFault = false }()
	if err := load(`    fn S(s ∈ String) (ptr) { host expr call "S" }`); err != nil {
		t.Errorf("with the obligation not required the declaration should load, so the test reaches the rule; got %v", err)
	}
}
