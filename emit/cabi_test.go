package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A C FUNCTION IS ITS PROTOTYPE (ADR 0094). `host expr call "F"` declares one
// by its symbol, and its sets are its C types, each a C type's whole value set
// (ADR 0126): a narrow integer's full range, the word, the unsigned word on a
// target that realizes it, `ptr`, or a C string carrying `where nulfree`.
// Anything else is refused when the target loads, naming why: the word's
// naturals, 0..2^63-1, among them, a set no C type has.
func TestACFunctionIsItsPrototype(t *testing.T) {
	load := func(decl string) error {
		t.Helper()
		dir := t.TempDir()
		src := "backend llvm\ntriple \"x86_64-pc-windows-msvc\"\nset Int = host \"i64\"\nrepr -2^63..2^63-1 word\nrepr 0..2^64-1 word\n" +
			"set ptr = host \"ptr\"\nset String = host \"ptr\"\nmodule k {\n" + decl + "\n}\n"
		if err := os.WriteFile(filepath.Join(dir, "t.oro"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := LoadTarget(filepath.Join(dir, "t.oro"))
		return err
	}
	for what, decl := range map[string]string{
		"an int, a DWORD, a pointer and a SIZE_T":     `    fn F(a ∈ -2^31..2^31-1, b ∈ 0..2^32-1, c ∈ ptr, d ∈ 0..2^64-1) (r ∈ -2^15..2^15-1) { host expr call "F" stdcall }`,
		"a SIZE_T result":                             `    fn H(a ∈ ptr) (r ∈ 0..2^64-1) { host expr call "H" stdcall }`,
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
		{"the word's naturals, a parameter", `    fn F(a ∈ 0..2^63-1) (ptr) { host expr call "F" }`, "no C type"},
		{"the word's naturals, a result", `    fn F(a ∈ ptr) (r ∈ 0..2^63-1) { host expr call "F" }`, "no C type it can return"},
		{"a C string without nulfree", `    fn S(s ∈ String) (ptr) { host expr call "S" }`, "needs `where nulfree(s)`"},
	} {
		err := load(c.decl)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}

	// THE PLANTED FAULTS: the word's naturals a C type again (ADR 0094's
	// reading of a 64-bit unsigned type before ADR 0126, a claim no C type
	// makes at a result) …
	PlantNaturalsCType = true
	if err := load(`    fn F(a ∈ ptr) (r ∈ 0..2^63-1) { host expr call "F" }`); err != nil {
		t.Errorf("with the word's naturals a C type the declaration should load, so the test reaches the rule; got %v", err)
	}
	PlantNaturalsCType = false
	// … and a C string's obligation not required.
	PlantCStringFault = true
	defer func() { PlantCStringFault = false }()
	if err := load(`    fn S(s ∈ String) (ptr) { host expr call "S" }`); err != nil {
		t.Errorf("with the obligation not required the declaration should load, so the test reaches the rule; got %v", err)
	}
}

// A C CALL'S ARGUMENTS ARE TERMS OVER ITS PARAMETERS (ADR 0118): each a
// parameter, elem(b, lo) the address &b[lo] of a table of bytes, or null; each
// parameter used exactly once, so in : params → C's arguments is a function of
// all of them; elem defined only where the `where` states lo <= len(b).
func TestACCallsArgumentsAreTermsOverItsParameters(t *testing.T) {
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
	const head = `    fn W(h ∈ ptr, b ∈ Array(0..255), lo ∈ 0..2^63-1, n ∈ 0..2^32-1) (r ∈ -2^31..2^31-1) `
	for what, decl := range map[string]string{
		"a segment's address, the count and null": head + `where lo + n <= len(b) { host expr call "W"(h, elem(b, lo), n, null) stdcall }`,
		"the arguments reordered":                 head + `where n + lo <= len(b) { host expr call "W"(n, elem(b, lo), h) }`,
		"lo bounded alone":                        head + `where n >= 1 && lo <= len(b) { host expr call "W"(h, elem(b, lo), n) }`,
	} {
		if err := load(decl); err != nil {
			t.Errorf("%s: want it loaded, got %v", what, err)
		}
	}
	for _, c := range []struct{ what, decl, reason string }{
		{"a parameter left out", head + `where lo + n <= len(b) { host expr call "W"(h, elem(b, lo), null) }`, "appears 0 times"},
		{"a parameter twice", head + `where lo + n <= len(b) { host expr call "W"(h, elem(b, lo), n, n) }`, "appears 2 times"},
		{"a name no parameter", head + `where lo + n <= len(b) { host expr call "W"(h, elem(b, lo), n, m) }`, "which is no parameter"},
		{"elem of no table", `    fn W(h ∈ ptr, lo ∈ 0..2^63-1) (ptr) { host expr call "W"(elem(h, lo)) }`, "not a table of bytes"},
		{"elem of a table of words", `    fn W(b ∈ Array(Int), lo ∈ 0..2^63-1) (ptr) where lo <= len(b) { host expr call "W"(elem(b, lo)) }`, "not a table of bytes"},
		{"a signed offset", `    fn W(b ∈ Array(0..255), lo ∈ -1..9) (ptr) where lo <= len(b) { host expr call "W"(elem(b, lo)) }`, "not a natural"},
		{"elem with no bound on lo", head + `where n <= len(b) { host expr call "W"(h, elem(b, lo), n) }`, "defined for lo <= len(b)"},
		{"lo bounded through a signed m", `    fn W(b ∈ Array(0..255), lo ∈ 0..2^63-1, m ∈ -9..9) (ptr) where lo + m <= len(b) { host expr call "W"(elem(b, lo), m) }`, "defined for lo <= len(b)"},
	} {
		err := load(c.decl)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}

	// THE PLANTED FAULTS: a parameter dropped from the call, and elem's bound
	// not required; each refusal above must then be lost.
	PlantCArgDropped = true
	if err := load(head + `where lo + n <= len(b) { host expr call "W"(h, elem(b, lo), null) }`); err != nil {
		t.Errorf("with a dropped parameter allowed the declaration should load, so the test reaches the rule; got %v", err)
	}
	PlantCArgDropped = false
	PlantElemUnbounded = true
	defer func() { PlantElemUnbounded = false }()
	if err := load(head + `where n <= len(b) { host expr call "W"(h, elem(b, lo), n) }`); err != nil {
		t.Errorf("with elem's bound not required the declaration should load, so the test reaches the rule; got %v", err)
	}
}
