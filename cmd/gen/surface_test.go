package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// THE SURFACE'S PROMISES (docs/book/README.md, plan item 4): what j.md says is
// built and the reader refused, each built and its rule's fault planted; and
// the diagnostics, each in J.

// surfaceGen compiles files["p.oro"] on target with the other files beside it,
// under the repository: the emitted text, or the refusal.
func surfaceGen(t *testing.T, target string, files map[string]string) (string, error) {
	t.Helper()
	dir, err := os.MkdirTemp(".", "surfacetest")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	for n, src := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(dir, "p.out")
	if err := run("../../targets", filepath.Join(dir, "p.oro"), target, out, "", "../../lib", false, ""); err != nil {
		return "", err
	}
	b, err := os.ReadFile(out)
	return string(b), err
}

func onGo(t *testing.T, src string) (string, error) {
	t.Helper()
	return surfaceGen(t, "go", map[string]string{"p.oro": src})
}

// PREFIX MINUS IS THE ADDITIVE INVERSE (finding 5): −x = 0 − x in the ring ℤ,
// so `-n` reads as `0 - n`. THE PLANTED FAULT reads it as `-` applied to n
// alone, refused for its arity.
func TestPrefixMinusIsTheAdditiveInverse(t *testing.T) {
	const src = "export fn neg(n ∈ 0..100) (Int) { -n }\n"
	if code, err := onGo(t, src); err != nil || !strings.Contains(code, "0 - v0") {
		t.Fatalf("-n: %v\n%s", err, code)
	}
	core.PlantPrefixMinusUnary = true
	defer func() { core.PlantPrefixMinusUnary = false }()
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "takes 2 argument(s), given 1") {
		t.Errorf("with -n unary: %v; want its arity refused", err)
	}
}

// A CONSTANT IN A RANGE IS ITS VALUE (finding 13): `cap = 100` is an
// abbreviation, eliminable by unfolding (Padoa), so `0..cap` is the type
// 0..100, a parameter's and a result's. THE PLANTED FAULT leaves cap unfolded:
// `(int 0 cap)` is no type.
func TestAConstantInARangeIsItsValue(t *testing.T) {
	const src = "cap = 100\n\nexport fn small(n ∈ 0..cap) (0..cap) { n }\n"
	if _, err := onGo(t, src); err != nil {
		t.Fatalf("0..cap: %v", err)
	}
	core.PlantConstEndOpaque = true
	defer func() { core.PlantConstEndOpaque = false }()
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "is not a type") {
		t.Errorf("with cap unfolded nowhere: %v; want 0..cap no type", err)
	}
}

// A TOP-LEVEL VALUE MAY CLAIM ITS SET (finding 14, j.md §2.2): `cap ∈ 0..100 =
// 64` is the(0..100; 64), decided as a block binder's claim is, so 640 is
// refused. THE PLANTED FAULT refuses the form at the reader.
func TestATopLevelValueMayClaimItsSet(t *testing.T) {
	const ok = "cap ∈ 0..100 = 64\n\nexport fn c() (Int) { cap }\n"
	const bad = "cap ∈ 0..100 = 640\n\nexport fn c() (Int) { cap }\n"
	if code, err := onGo(t, ok); err != nil || !strings.Contains(code, "64") {
		t.Fatalf("cap ∈ 0..100 = 64: %v", err)
	}
	if _, err := onGo(t, bad); err == nil || !strings.Contains(err.Error(), "the claim cap ∈ 0..100 is false") {
		t.Errorf("cap ∈ 0..100 = 640: %v; want the claim refused", err)
	}
	core.PlantTopClaimRefused = true
	defer func() { core.PlantTopClaimRefused = false }()
	if _, err := onGo(t, ok); err == nil || !strings.Contains(err.Error(), "begins no declaration") {
		t.Errorf("with the form unread: %v; want it refused at the reader", err)
	}
}

// A TELESCOPE IS A SEPARATION (finding 9, ADR 0061). `i ∈ 0..len(a)-1` is
// i ∈ 0.. where i <= len(a) - 1, a dependent product's set, and its hull on the
// target, 0..max_len - 1, is the export's representation of i: the word. A call
// outside it is refused by the `where`. THE PLANTED FAULTS: the range kept as
// written, no type; and the open end left open, i held above the word.
func TestATelescopeIsASeparation(t *testing.T) {
	const src = "export fn at(a ∈ Array(Int), i ∈ 0..len(a)-1) (Int) { a[i] }\n"
	const call = "fn at(a ∈ Array(Int), i ∈ 0..len(a)-1) (Int) { a[i] }\n\n" +
		"export fn f() (Int) { at(array { 1, 2, 3 }, 3) }\n"
	for _, tg := range []string{"go", "windows"} {
		if code, err := surfaceGen(t, tg, map[string]string{"p.oro": src}); err != nil {
			t.Fatalf("a telescope on %s: %v\n%s", tg, err, code)
		}
	}
	if _, err := onGo(t, call); err == nil || !strings.Contains(err.Error(), "where") {
		t.Errorf("at(a, 3) on a table of three: %v; want the telescope's where refused", err)
	}
	core.PlantTelescopeRefused = true
	_, err := onGo(t, src)
	core.PlantTelescopeRefused = false
	if err == nil || !strings.Contains(err.Error(), "is not a type") {
		t.Errorf("with the range as written: %v; want no type", err)
	}
	emit.PlantNoTelescopeHull = true
	_, err = onGo(t, src)
	emit.PlantNoTelescopeHull = false
	if err == nil || !strings.Contains(err.Error(), "arbitrary precision") {
		t.Errorf("with the end left open: %v; want i above the word", err)
	}
}

// A MODULE FILE MAY HOLD AN UNDERSCORE (finding 16): a path is a file's name,
// kept as written, and its alias an identifier, spelled by σ_J⁻¹ as each
// reference is, so `use zz_b7` reads zz_b7.oro and `zz_b7.k` reaches it. THE
// PLANTED FAULT keeps the alias raw: zz_b7.k reads as zz-b7.k, which nothing
// imports.
func TestAModuleFileMayHoldAnUnderscore(t *testing.T) {
	files := map[string]string{
		"p.oro":     "use zz_b7\n\nexport fn run() (Int) { zz_b7.k() }\n",
		"zz_b7.oro": "pub fn k() (Int) { 7 }\n",
	}
	if code, err := surfaceGen(t, "go", files); err != nil || !strings.Contains(code, "7") {
		t.Fatalf("use zz_b7: %v\n%s", err, code)
	}
	core.PlantAliasRaw = true
	defer func() { core.PlantAliasRaw = false }()
	if _, err := surfaceGen(t, "go", files); err == nil || !strings.Contains(err.Error(), "is not imported") {
		t.Errorf("with the alias raw: %v; want zz_b7 not imported", err)
	}
}

// A RULE AT AN EXPORT'S BOUNDARY IS ITS EXTENSION (finding 6, ADR 0115):
// representation is fixed at the boundary (g5 §1), so a rule an export returns
// is the host's array, allocated there. THE PLANTED FAULT leaves the rule as it
// is: refused as never allocated.
func TestARuleAtTheBoundaryIsItsExtension(t *testing.T) {
	const src = "export fn sq(n ∈ 0..100) (Array(Int)) { array { i * i : i ∈ 0..n - 1 } }\n"
	for _, tg := range []string{"go", "js", "java", "windows"} {
		if _, err := surfaceGen(t, tg, map[string]string{"p.oro": src}); err != nil {
			t.Fatalf("a rule returned on %s: %v", tg, err)
		}
	}
	emit.PlantRuleAtBoundary = true
	defer func() { emit.PlantRuleAtBoundary = false }()
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "never allocated") {
		t.Errorf("with the rule unallocated: %v; want it refused", err)
	}
}

// A BIG VALUE AT A WORD PARAMETER IS REFUSED IN THE PROGRAM'S TERMS (finding 3):
// a statement's template reads its arguments as an expression's does, so Go's
// map store, a statement whose value parameter is go.Word, demands the word of
// its argument, and fact(30) is refused naming it. THE PLANTED FAULT skips a statement's
// arguments: the verifier's W5 refuses it in its own words.
func TestABigValueAtAWordParameterIsRefusedInTheProgramsTerms(t *testing.T) {
	// a host statement's word parameter: Go's map store `m[k] = v`, v ∈ go.Word
	// (io.print_int took every integer since ADR 0147, and was this test's
	// statement before)
	const src = "use go\n\nfn fact(n ∈ 0..30) (0..) {\n    loop i = 1, acc = 1 {\n        i > n -> acc\n" +
		"        else -> again(i + 1, acc * i)\n    }\n}\n\nexport fn main() {\n    m = go.make_map()\n" +
		"    ignore(go.set_map(m, \"k\", fact(30)))\n}\n"
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "argument 3 of go.set_map is held in arbitrary precision") {
		t.Fatalf("set_map(m, k, fact(30)) on Go: %v; want its argument refused, named", err)
	}
	ir.PlantStmtArgsUnasked = true
	defer func() { ir.PlantStmtArgsUnasked = false }()
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "W5") {
		t.Errorf("with a statement's arguments unasked: %v; want W5's refusal", err)
	}
}

// THE DIAGNOSTICS IN J (findings 4, 7, 11, 21): each refusal quotes the program
// as J writes it, not the core's terms or a deleted target's name.
func TestTheDiagnosticsAreInJ(t *testing.T) {
	for what, c := range map[string]struct{ src, want, not string }{
		"a write to a value (4)": {"export fn f(n ∈ 0..9) (Int) {\n    x = n\n    x := 5\n    x\n}\n",
			"`:=` writes a `var`", "frozen table"},
		"a range payload (7)": {"set Shape = 'circle(0..1000) ∪ 'square(0..1000)\n\nexport fn a() (Int) { 0 }\n",
			"'circle(0..1000) gives a range", "(int 0 1000)"},
		"the closure (7)": {"export fn mk(k ∈ 0..9) (Int) {\n    g = fn(v) { v * k }\n    g(2) + g\n}\n",
			"fn(v) { v * k }", "#1.0"},
		"the overflow hint (7)": {"export fn sum(t ∈ Array(Int)) (-2^63..2^63-1) {\n    var s := array { 0 : _ ∈ 0..len(t)-1 }\n    loop i = 0, acc = 0 {\n" +
			"        i < len(t) -> { s[i] := acc + t[i]; again(i + 1, acc + t[i]) }\n        else -> len(s)\n    }\n}\n", "acc + t[i]", "t(i)"},
		"the step limit (11)": {"export fn t() (Int) { fn(a) { a(a) }(fn(a) { a(a) }) }\n", "fn() {", "(fn ()"},
		"equality (21)":       {"export fn eq(a ∈ String, b ∈ String) (Bool) { a == b }\n", "java.`==`", "x64"},
	} {
		_, err := onGo(t, c.src)
		if err == nil || !strings.Contains(err.Error(), c.want) || strings.Contains(err.Error(), c.not) {
			t.Errorf("%s: %v\n  want %q and not %q", what, err, c.want, c.not)
		}
	}
}
