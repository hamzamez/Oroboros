package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// THE BOOK'S WRONG ANSWERS (docs/book/README.md, plan item 3): programs the
// compiler accepted and should not, each refused for its reason and accepted
// again with that rule's fault planted. A correct twin of each still builds.

// bookCase compiles src on Go: nil, or the refusal. The source is written
// under the repository, whose lib/ an import's variants are read from.
func bookCase(t *testing.T, src string) error {
	_, err := bookGen(t, src)
	return err
}

// bookGen compiles src on Go and answers the emitted text.
func bookGen(t *testing.T, src string) (string, error) {
	t.Helper()
	dir, err := os.MkdirTemp(".", "booktest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "p.go")
	if err := run("../../targets", path, "go", out, "", "../../lib", false, ""); err != nil {
		return "", err
	}
	b, err := os.ReadFile(out)
	return string(b), err
}

// THE UNIT IS NO INTEGER (findings 19 and 10). 1 = {()} is the terminal object,
// disjoint from ℤ: a one-armed `if c { 1 }` is `if c {1} else {()}`, whose type
// is Int ∨ 1, so it is no Int; and a declared `()` is a claim decided at the
// definition (ADR 0072), so a body yielding n is refused. THE PLANTED FAULTS:
// the unit typed unknown, which agreed with Int; and `()` read as a product of
// no components, no claim at all.
func TestTheUnitIsNoInteger(t *testing.T) {
	const oneArm = "export fn f(c ∈ Bool) (Int) { if c { 1 } }\n"
	const value = "export fn nothing(n ∈ 0..9) () { n }\n"
	const unit = "export fn nothing(n ∈ 0..9) () { () }\n"
	if err := bookCase(t, unit); err != nil {
		t.Fatalf("a () result yielding (): %v", err)
	}
	if err := bookCase(t, oneArm); err == nil || !strings.Contains(err.Error(), "the unit ()") {
		t.Errorf("if c { 1 } as an Int: %v; want the unit refused as an Int", err)
	}
	if err := bookCase(t, value); err == nil || !strings.Contains(err.Error(), "() is required") {
		t.Errorf("a () result yielding n: %v; want it refused", err)
	}
	emit.PlantUnitUnknown = true
	err := bookCase(t, oneArm)
	emit.PlantUnitUnknown = false
	if err != nil {
		t.Errorf("with the unit typed unknown: %v; want if c { 1 } accepted", err)
	}
	core.PlantUnitNoClaim = true
	err = bookCase(t, value)
	core.PlantUnitNoClaim = false
	if err != nil {
		t.Errorf("with () read as no claim: %v; want the body n accepted", err)
	}
}

// A STRING IS NO TABLE (finding 1). Σ* is the free monoid over Unicode scalars;
// len : Table(σ) → ℕ and an indexing are a table's, and a string's length is not
// in the language because Go counts bytes and JavaScript and the JVM UTF-16
// units (state.md §3). THE PLANTED FAULT lets both take a String: len(s)
// compiles, to three answers.
func TestAStringIsNoTable(t *testing.T) {
	const length = "export fn n(s ∈ String) (Int) { len(s) }\n"
	const index = "export fn n(s ∈ String) (Int) { s[0] }\n"
	const bytes = "export fn n(b ∈ Array(0..255)) (Int) { len(b) }\n"
	if err := bookCase(t, bytes); err != nil {
		t.Fatalf("len of a table: %v", err)
	}
	for _, src := range []string{length, index} {
		if err := bookCase(t, src); err == nil || !strings.Contains(err.Error(), "a String is text") {
			t.Errorf("%s: %v; want it refused, a String being no table", strings.TrimSpace(src), err)
		}
	}
	emit.PlantStringIsTable = true
	defer func() { emit.PlantStringIsTable = false }()
	if err := bookCase(t, length); err != nil {
		t.Errorf("with a String a table: %v; want len(s) accepted", err)
	}
}

// A DEFINITION'S RELEVANT RESULT IS NOT DROPPED (finding 2, ADR 0043). Relevance
// is the type's, and a call has its definition's body's type: half's tails are
// 'ok and 'err of a relevant Result, so `half(n); 0` and an unread `r = half(n)`
// are refused as a host's are. THE PLANTED FAULT reads a call of a definition as
// relevant only when the definition is a constructor.
func TestADefinitionsRelevantResultIsNotDropped(t *testing.T) {
	const head = "use result\n\nfn half(n ∈ 0..1000000) (result.Result(Int, Int)) {\n" +
		"    if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) }\n}\n\n"
	dropped := head + "export fn quarter(n ∈ 0..1000000) (Int) {\n    half(n)\n    0\n}\n"
	unread := head + "export fn quarter(n ∈ 0..1000000) (Int) {\n    r = half(n)\n    0\n}\n"
	used := head + "export fn quarter(n ∈ 0..1000000) (Int) {\n    case half(n) {\n" +
		"        result.'ok(q) -> q\n        result.'err(e) -> { ignore(e); 0 }\n    }\n}\n"
	if err := bookCase(t, used); err != nil {
		t.Fatalf("half's Result taken apart: %v", err)
	}
	for _, src := range []string{dropped, unread} {
		if err := bookCase(t, src); err == nil || !strings.Contains(err.Error(), "the result of half is a value of a relevant type") {
			t.Errorf("half's Result dropped: %v; want it refused", err)
		}
	}
	core.PlantRelevanceShallow = true
	defer func() { core.PlantRelevanceShallow = false }()
	if err := bookCase(t, dropped); err != nil {
		t.Errorf("with relevance read only off a constructor: %v; want half(n); 0 accepted", err)
	}
}

// A MODULE BLOCK ENDS AT ITS BRACE (finding 15, j.md §2.1). The core's
// `(module X)` runs to the next one, so the parser opens the enclosing module
// again at the block's close: `run` after `module inner { … }` is the file's,
// so its bare `k` is the file's k, u · 2, not the block's, u + 1. THE PLANTED
// FAULT leaves the close unmarked: run is then read into inner, and its k is
// the block's.
func TestAModuleBlockEndsAtItsBrace(t *testing.T) {
	const src = "fn k(u ∈ 0..9) (Int) { u * 2 }\n\n" +
		"module inner {\n    fn k(u ∈ 0..9) (Int) { u + 1 }\n}\n\n" +
		"export fn run(u ∈ 0..9) (Int) { k(u) }\n"
	code, err := bookGen(t, src)
	if err != nil || !strings.Contains(code, "* 2") {
		t.Fatalf("run after inner's block: %v; want the file's k, u * 2\n%s", err, code)
	}
	core.PlantModuleNoClose = true
	defer func() { core.PlantModuleNoClose = false }()
	if code, err := bookGen(t, src); err == nil && strings.Contains(code, "* 2") {
		t.Errorf("with the block's close unmarked: the file's k; want run read into inner, and its k")
	}
}

// A LITERAL OF Σ_L IS NOT DEFINABLE (finding 20, ADR 0017). `true` is a term
// kind of its own, so a definition of it is read by nothing: accepted and
// ignored, the literal stayed `true`. THE PLANTED FAULT leaves the literals
// out of Σ_L.
func TestALiteralIsNotDefinable(t *testing.T) {
	const src = "true = 1\nexport fn t() (Bool) { true }\n"
	if err := bookCase(t, src); err == nil || !strings.Contains(err.Error(), "which is the language's own") {
		t.Errorf("true = 1: %v; want it refused as the language's own", err)
	}
	core.PlantLiteralsDefinable = true
	defer func() { core.PlantLiteralsDefinable = false }()
	if err := bookCase(t, src); err != nil {
		t.Errorf("with the literals definable: %v; want true = 1 accepted and ignored", err)
	}
}
