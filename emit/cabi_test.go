package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
)

// A C FUNCTION IS ITS PROTOTYPE (ADR 0094). `host expr call F` declares one
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
		"an int, a DWORD, a pointer and a SIZE_T":     `    fn F(a ∈ -2^31..2^31-1, b ∈ 0..2^32-1, c ∈ ptr, d ∈ 0..2^64-1) (r ∈ -2^15..2^15-1) { host expr call F stdcall }`,
		"a SIZE_T result":                             `    fn H(a ∈ ptr) (r ∈ 0..2^64-1) { host expr call H stdcall }`,
		"a C string with its obligation":              `    fn S(s ∈ String) (r ∈ 0..2^8-1) where nulfree(s) { host expr call S }`,
		"a void function":                             `    fn V(a ∈ 0..2^32-1) (0..2^32-1) { host stmt call V cdecl }`,
		"a C string among other obligations, by name": `    fn T(n ∈ 0..2^32-1, s ∈ String) (ptr) where n >= 1 && nulfree(s) { host expr call T }`,
	} {
		if err := load(decl); err != nil {
			t.Errorf("%s: want it loaded, got %v", what, err)
		}
	}
	for _, c := range []struct{ what, decl, reason string }{
		{"a narrow domain, not its type's range", `    fn F(a ∈ 0..9) (ptr) { host expr call F }`, "no C type"},
		{"a Boolean", `    fn F(a ∈ Bool) (ptr) { host expr call F }`, "no C type"},
		{"a result that is no C type", `    fn F(a ∈ ptr) (r ∈ 0..99) { host expr call F }`, "no C type it can return"},
		{"the word's naturals, a parameter", `    fn F(a ∈ 0..2^63-1) (ptr) { host expr call F }`, "no C type"},
		{"the word's naturals, a result", `    fn F(a ∈ ptr) (r ∈ 0..2^63-1) { host expr call F }`, "no C type it can return"},
		{"a C string without nulfree", `    fn S(s ∈ String) (ptr) { host expr call S }`, "needs `where nulfree(s)`"},
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
	if err := load(`    fn F(a ∈ ptr) (r ∈ 0..2^63-1) { host expr call F }`); err != nil {
		t.Errorf("with the word's naturals a C type the declaration should load, so the test reaches the rule; got %v", err)
	}
	PlantNaturalsCType = false
	// … and a C string's obligation not required.
	PlantCStringFault = true
	defer func() { PlantCStringFault = false }()
	if err := load(`    fn S(s ∈ String) (ptr) { host expr call S }`); err != nil {
		t.Errorf("with the obligation not required the declaration should load, so the test reaches the rule; got %v", err)
	}
}

// A C CALL'S ARGUMENTS ARE TERMS OVER ITS PARAMETERS (ADR 0118), written as C
// writes them (ADR 0131): each a parameter, &b[lo] the address of a table of
// bytes offset by lo, or NULL; each parameter used exactly once, so in :
// params → C's arguments is a function of all of them; &b[lo] defined only
// where the `where` states lo <= len(b).
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
		"a segment's address, the count and null": head + `where lo + n <= len(b) { host expr call W(h, &b[lo], n, NULL) stdcall }`,
		"the arguments reordered":                 head + `where n + lo <= len(b) { host expr call W(n, &b[lo], h) }`,
		"lo bounded alone":                        head + `where n >= 1 && lo <= len(b) { host expr call W(h, &b[lo], n) }`,
	} {
		if err := load(decl); err != nil {
			t.Errorf("%s: want it loaded, got %v", what, err)
		}
	}
	for _, c := range []struct{ what, decl, reason string }{
		{"a parameter left out", head + `where lo + n <= len(b) { host expr call W(h, &b[lo], NULL) }`, "appears 0 times"},
		{"a parameter twice", head + `where lo + n <= len(b) { host expr call W(h, &b[lo], n, n) }`, "appears 2 times"},
		{"a name no parameter", head + `where lo + n <= len(b) { host expr call W(h, &b[lo], n, m) }`, "which is no parameter"},
		{"elem of no table", `    fn W(h ∈ ptr, lo ∈ 0..2^63-1) (ptr) { host expr call W(&h[lo]) }`, "not a table of bytes"},
		{"elem of a table of words", `    fn W(b ∈ Array(Int), lo ∈ 0..2^63-1) (ptr) where lo <= len(b) { host expr call W(&b[lo]) }`, "not a table of bytes"},
		{"a signed offset", `    fn W(b ∈ Array(0..255), lo ∈ -1..9) (ptr) where lo <= len(b) { host expr call W(&b[lo]) }`, "not a natural"},
		{"elem with no bound on lo", head + `where n <= len(b) { host expr call W(h, &b[lo], n) }`, "defined for lo <= len(b)"},
		{"lo bounded through a signed m", `    fn W(b ∈ Array(0..255), lo ∈ 0..2^63-1, m ∈ -9..9) (ptr) where lo + m <= len(b) { host expr call W(&b[lo], m) }`, "defined for lo <= len(b)"},
	} {
		err := load(c.decl)
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}

	// THE PLANTED FAULTS: a parameter dropped from the call, and elem's bound
	// not required; each refusal above must then be lost.
	PlantCArgDropped = true
	if err := load(head + `where lo + n <= len(b) { host expr call W(h, &b[lo], NULL) }`); err != nil {
		t.Errorf("with a dropped parameter allowed the declaration should load, so the test reaches the rule; got %v", err)
	}
	PlantCArgDropped = false
	PlantElemUnbounded = true
	defer func() { PlantElemUnbounded = false }()
	if err := load(head + `where n <= len(b) { host expr call W(h, &b[lo], n) }`); err != nil {
		t.Errorf("with elem's bound not required the declaration should load, so the test reaches the rule; got %v", err)
	}
}

func loadCDecl(t *testing.T, decl string) (*Target, error) {
	t.Helper()
	dir := t.TempDir()
	src := "backend llvm\ntriple \"x86_64-pc-windows-msvc\"\nset Int = host \"i64\"\nrepr -2^63..2^63-1 word\n" +
		"set ptr = host \"ptr\"\nset String = host \"ptr\"\nset cell(A) = host \"ptr\"\nmodule k {\n" + decl + "\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "t.oro"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadTarget(filepath.Join(dir, "t.oro"))
}

// AN _Out_ PARAMETER IS A RESULT (ADR 0131): `&r`, r a declared result, is
// the address of a fresh cell the host writes, and r leads the result tuple,
// so the declaration takes only what the caller supplies and the call's value
// is the cells' values, then the C function's own. The old spellings are
// refused naming the new ones, and a scalar written `&x` that is neither a
// struct nor a cell is refused: a scalar the host writes is a result.
func TestAnOutParameterIsAResult(t *testing.T) {
	tg, err := loadCDecl(t, `    fn G(h ∈ ptr) (n ∈ 0..2^32-1, r ∈ -2^31..2^31-1) { host expr call G(h, &n) stdcall }`)
	if err != nil {
		t.Fatalf("an out-result: want it loaded, got %v", err)
	}
	q, ok := tg.Prims["k.G"]
	if !ok || len(q.Args) != 2 || len(q.Outs) != 1 || q.Outs[0] != 1 {
		t.Errorf("k.G: want two parameters, the second the out-result's cell, Outs [1]; got %v %v (%v)", q.Args, q.Outs, ok)
	}
	for _, c := range []struct{ what, decl, reason string }{
		{"the out-result after the C result", `    fn G(h ∈ ptr) (r ∈ -2^31..2^31-1, n ∈ 0..2^32-1) { host expr call G(h, &n) }`, "lead the result tuple"},
		{"the symbol quoted", `    fn G(h ∈ ptr) (ptr) { host expr call "G"(h) }`, "as C writes it"},
		{"elem", `    fn W(b ∈ Array(0..255), lo ∈ 0..2^63-1) (ptr) where lo <= len(b) { host expr call W(elem(b, lo)) }`, "`&b[lo]`"},
		{"ref", `    fn W(h ∈ ptr) (ptr) { host expr call W(ref(h)) }`, "`&x`"},
		{"null", `    fn W(h ∈ ptr) (ptr) { host expr call W(h, null) }`, "`NULL`"},
		{"& of a scalar parameter", `    fn W(h ∈ ptr, n ∈ 0..2^32-1) (ptr) { host expr call W(h, &n) }`, "a scalar the host writes is a result or a cell"},
	} {
		if _, err := loadCDecl(t, c.decl); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}
}

// A C FUNCTION'S POINTER AND COUNT ARE ONE SEGMENT (ADR 0132): `s` and
// `len(s)` are a table's address and length, its two projections; the width
// of len(s) is the C integer the `where` states, and the declaration gains its
// segment variant F#seg(…, s, s#lo, s#hi, …), whose call is &s[lo] and
// hi − lo and whose `where` is the segment's formation, 0 <= lo <= hi <=
// len(s), with the original's len(s) read as hi − lo. A length without its
// address, a length twice, and a length with no stated width are refused.
func TestASegmentIsACArgument(t *testing.T) {
	const decl = `    fn W(h ∈ ptr, s ∈ Array(0..255)) (r ∈ -2^31..2^31-1) where len(s) <= 2^32-1 { host expr call W(h, s, len(s), NULL) stdcall }`
	tg, err := loadCDecl(t, decl)
	if err != nil {
		t.Fatalf("a segment: want it loaded, got %v", err)
	}
	v, ok := tg.Prims["k.W#seg"]
	if !ok || len(v.Args) != 4 || len(v.CArgs) != 4 {
		t.Fatalf("k.W#seg: want four parameters and four C arguments, got %v (%v)", v.Args, ok)
	}
	if v.CArgs[1].Kind != CElem || v.CArgs[2].Kind != CDiff || v.CArgs[2].K != 32 {
		t.Errorf("k.W#seg: want &b[lo] and a 32-bit hi − lo, got %v", v.CArgs)
	}
	w := core.ShowTerm(v.Where)
	for _, part := range []string{"0 <= `s#lo`", "`s#lo` <= `s#hi`", "`s#hi` <= len(s)", "`s#hi` - `s#lo` <= 4294967295"} {
		if !strings.Contains(w, part) {
			t.Errorf("k.W#seg's where %s: want it to state the segment's formation, missing %s", w, part)
		}
	}
	if strings.Contains(w, "len(s) <=") {
		t.Errorf("k.W#seg's where %s still reads the original's len(s)", w)
	}
	for _, c := range []struct{ what, decl, reason string }{
		{"a length without its address", `    fn W(h ∈ ptr, s ∈ Array(0..255)) (ptr) where len(s) <= 2^32-1 { host expr call W(h, len(s)) }`, "goes with its address"},
		{"a length twice", `    fn W(s ∈ Array(0..255)) (ptr) where len(s) <= 2^32-1 { host expr call W(s, len(s), len(s)) }`, "appears 2 times"},
		{"a length with no width", `    fn W(s ∈ Array(0..255)) (ptr) { host expr call W(s, len(s)) }`, "must state its type's top"},
	} {
		if _, err := loadCDecl(t, c.decl); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: want it refused naming %q, got %v", c.what, c.reason, err)
		}
	}
	// THE PLANTED FAULT: no variant derived, so a program's b[lo:hi] reaches
	// no declaration (its build witness is cmd/build's TestASegmentIsPassed).
	PlantNoSegmentVariant = true
	defer func() { PlantNoSegmentVariant = false }()
	if tg, err := loadCDecl(t, decl); err != nil {
		t.Errorf("with no variant the declaration should still load; got %v", err)
	} else if _, ok := tg.Prims["k.W#seg"]; ok {
		t.Errorf("with the variant not derived, k.W#seg should be absent")
	}
}
