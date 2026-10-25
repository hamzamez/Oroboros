package core

import (
	"strings"
	"testing"
)

// core-0 §1.1 — NFC. Without it, `é` as U+00E9 and as e+U+0301 are two
// distinct identifiers that display identically.
func TestNonNFCIsRejected(t *testing.T) {
	if _, err := Read("caf\u0065\u0301 = 1"); err == nil || !strings.Contains(err.Error(), "NFC") {
		t.Fatal("a decomposed é must be rejected")
	}
	if _, err := Read("café = 1"); err != nil {
		t.Errorf("a precomposed é is NFC and must pass: %v", err)
	}
	// A combining mark with no precomposed form is NFC-stable, and rejecting
	// it would break every script that needs one.
	if _, err := Read("q̇x = 1"); err != nil {
		t.Errorf("q + combining dot above has no precomposed form: %v", err)
	}
}

// §1.1 — case is preserved and significant for IDENTITY. What the rule forbids
// is case carrying MEANING, as in Shen's capitals-are-variables or Go's
// capitals-are-exported.
func TestCaseIsSignificantForIdentity(t *testing.T) {
	forms, err := Read("t = f(Xr, xr, XR)")
	if err != nil {
		t.Fatal(err)
	}
	kids := forms[0].Term.Kids
	seen := map[string]bool{}
	for _, k := range kids[1:] {
		seen[k.Name] = true
	}
	if len(seen) != 3 {
		t.Errorf("Xr, xr and XR must be three distinct names, got %d: %v", len(seen), seen)
	}
}

// A repeated binder in ONE abstraction is ill-formed: β substituted parameter
// by parameter, so ((fn (x x) x) 1 2) reduced to 2 and the first argument
// vanished with no way to name it.
func TestDuplicateParameterIsRejected(t *testing.T) {
	if _, err := Read("fn t(x, x) { x }"); err == nil || !strings.Contains(err.Error(), "binds x twice") {
		t.Fatalf("fn t(x, x) must be rejected, got %v", err)
	}
	if _, err := Read("fn t(a, b) { a }"); err != nil {
		t.Errorf("distinct parameters must pass: %v", err)
	}
	// Nested shadowing is two abstractions, and stays legal.
	if _, err := Read("fn t(x) { fn(x) { x } }"); err != nil {
		t.Errorf("nested shadowing must stay legal: %v", err)
	}
}

// A λ'S PARAMETERS ARE NAMES separated by commas. The reader took any token
// as one, so `fn(m ∈ 0..9) { m }` was a λ of five parameters, `m`, `∈`, `0`,
// `..` and `9`, and failed at its call with an arity no source states
// (jswitch5b). A claim on a λ is ADR 0066's header, not built.
func TestALambdasParametersAreNames(t *testing.T) {
	for _, src := range []string{"fn t(n) { g = fn(m ∈ 0..9) { m }; g(n) }", "fn t(n) { g = fn(a b) { a }; g(n) }"} {
		if _, err := Read(src); err == nil || !strings.Contains(err.Error(), "in a λ's parameters") {
			t.Errorf("%s must be refused at its parameters, got %v", src, err)
		}
	}
	if _, err := Read("fn t(n) { g = fn(a, b) { a }; g(n, n) }"); err != nil {
		t.Errorf("names separated by commas are a λ's parameters: %v", err)
	}
}

// Name resolution — a separate question from the covering check, and one this
// project had been answering only by accident.
//
//	scope    — is this name bound anywhere at all?  A PROGRAM error.
//	covering — can THIS target provide it?          A portability property.
//
// Three holes came from conflating them: `oro` warned and exited 0, `gen` never
// checked, and a name appearing only in an unreached definition was never
// looked at.
func TestScopeCheck(t *testing.T) {
	env := func(src string) (*Env, []*Term) {
		t.Helper()
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, terms, err := Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		e := &Env{Defs: prog.Defs, Prim: map[string]bool{"add": true},
			Pure: map[string]bool{"add": true}, Rec: map[string]bool{}}
		e.MarkRecursive()
		return e, terms
	}

	e, terms := env("fn t(x) { y }")
	if err := e.CheckProgram(terms); err == nil {
		t.Error("a free variable in a body must be reported")
	}

	// The classic one: a typo in a definition the program never reaches.
	e, terms = env(`fn unused(x) { nope }
t = add(1, 2)`)
	err := e.CheckProgram(terms)
	if err == nil {
		t.Fatal("a typo in an unreached definition must be reported")
	}
	if !strings.Contains(err.Error(), "unused") {
		t.Errorf("the report should name the definition, got %v", err)
	}

	// Parameters, definitions and primitives are all in scope.
	e, terms = env(`fn f(a) { add(a, 1) }
fn t(x) { f(x) }`)
	if err := e.CheckProgram(terms); err != nil {
		t.Errorf("bound names must pass: %v", err)
	}

	// A recursive definition refers to itself, which IS in scope — the two
	// questions are separate, and only the second one rejects it (ADR 0014).
	e, terms = env(`fn loop_forever(n) { loop_forever(n) }
t = loop_forever(1)`)
	if err := e.checkScope(terms); err != nil {
		t.Errorf("a recursive definition is in its own scope: %v", err)
	}
	err = e.CheckProgram(terms)
	if err == nil {
		t.Fatal("recursion must be rejected")
	}
	for _, want := range []string{"loop-forever", "fold-range", "0014"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

// ADR 0014. Recursion reduces correctly and no backend emits it, so it must
// fail at the earliest honest point rather than at the emitter — `oro` used to
// accept a program `build` refused.
func TestRecursionIsRejected(t *testing.T) {
	load := func(src string, prims ...string) error {
		t.Helper()
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, terms, err := Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		return testEnv(prog, prims...).CheckProgram(terms)
	}

	// Mutual recursion, named in full.
	err := load(`fn even?(n) { odd?(n) }
fn odd?(n) { even?(n) }
t = even?(3)`, "sub")
	if err == nil {
		t.Fatal("mutual recursion must be rejected")
	}
	for _, want := range []string{"even?", "odd?", "mutually recursive"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}

	// An unused recursive definition is still rejected: every definition is
	// checked, which is what turns a typo'd self-reference into an error.
	if err := load(`fn size(v) { size(v) }
t = add(1, 2)`, "add"); err == nil {
		t.Error("an unused recursive definition must be rejected")
	}

	// But NOT when the target provides the name natively: δ never unfolds the
	// definition, so the cycle is unreachable. This is ADR 0002's "compiling
	// up", and rejecting it would break every program built against a target
	// that happens to implement one of the library's functions.
	if err := load(`fn sort(v) { sort(v) }
t = sort(1)`, "sort"); err != nil {
		t.Errorf("a definition the target shadows must not be rejected: %v", err)
	}
}

// A binder must be a SIMPLE name. Binding a qualified one is meaningless — a λ
// cannot bind into a module — and it let a parameter shadow a module-qualified
// primitive, after which reduction applied a number to two arguments.
func TestQualifiedNameCannotBeAParameter(t *testing.T) {
	if _, err := Read("fn t(a.b) { a.b }"); err == nil {
		t.Fatal("fn t(a.b) must be rejected")
	}
	if _, err := Read("fn t(`num/f64`.add) { 1 }"); err == nil {
		t.Fatal("binding a module-qualified name must be rejected")
	}
	// A quoted name is one name, whatever it holds.
	if _, err := Read("fn t(`a/b`) { `a/b` }"); err != nil {
		t.Errorf("a quoted name is one simple name: %v", err)
	}
}

// A definition names a member of THIS module, so `.` cannot appear in it — a
// qualified name in a term always resolves to an import. Before this rule,
// `(def a.b …)` was accepted and no term could ever refer to it.
func TestQualifiedNameCannotBeDefined(t *testing.T) {
	if _, err := Read("fn a.b(n) { n }"); err == nil {
		t.Fatal("fn a.b must be rejected")
	}
	if _, err := Read("fn `a/b`(n) { n }"); err != nil {
		t.Errorf("a quoted name is one simple name: %v", err)
	}
}

// Reduction never opens a binder, so a term captured mid-flight has bound
// variables with no name and prints as `#1.0`. Both errors that carry a term
// reclose it as the stack unwinds, so the message is spelled like the source.
func TestDiagnosticsUnderABinderKeepTheirNames(t *testing.T) {
	forms, err := Read("fn t(n) { fn(a, b) { h(a, b) }(n) }")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, err := Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = Normalize(entryTerm(t, prog, terms), testEnv(prog, "h"), DefaultFuel)
	if err == nil {
		t.Fatal("applying a 2-parameter fn to 1 argument must fail")
	}
	if strings.Contains(err.Error(), "#") {
		t.Errorf("diagnostic leaks the internal representation: %v", err)
	}
	for _, want := range []string{"fn(n)", "fn(a, b)", "given 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}

	forms, err = Read("t = fn(n) { fn(a) { a(a) }(fn(a) { a(a) }) }")
	if err != nil {
		t.Fatal(err)
	}
	prog, terms, err = Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Normalize(entryTerm(t, prog, terms), testEnv(prog), DefaultFuel); err == nil {
		t.Fatal("Ω under a binder must exhaust fuel")
	}
	if strings.Contains(err.Error(), "#") {
		t.Errorf("fuel diagnostic leaks the internal representation: %v", err)
	}
}

// def.md §11. A definition the target provides natively is the parasite model
// working — the note exists because the file that decides it is one the program
// never names. It must fire on exactly that collision and nothing else.
func TestShadowedByTargetIsReported(t *testing.T) {
	forms, err := Read(`fn dot(a, b) { mul(a, b) }
fn near(a) { dot(a, a) }`)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	if got := testEnv(prog, "mul").Shadowed(); len(got) != 0 {
		t.Errorf("no collision, but reported %v", got)
	}
	e := testEnv(prog, "mul", "dot")
	got := e.Shadowed()
	if len(got) != 1 || got[0] != "dot" {
		t.Fatalf("expected [dot], got %v", got)
	}
	// And the definition really is the one that loses.
	out, err := Normalize(prog.Defs["near"], e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if s := out.String(); s != "(fn (a) (dot a a))" {
		t.Errorf("the target's dot must win, got %s", s)
	}
}

// Chapter 3. Four module diagnostics, each of which pointed at the wrong thing.
func TestModuleDiagnostics(t *testing.T) {
	load := func(files map[string]string, src string, prims ...string) error {
		t.Helper()
		forms, err := Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, terms, err := LoadWith(forms, snippetResolver(files))
		if err != nil {
			return err
		}
		e := testEnv(prog, prims...)
		e.SetUnresolved(prog.Unresolved)
		return e.CheckProgram(terms)
	}
	geo := "pub fn area(w, h) { mul(w, h) }\nhidden = 1\n"
	files := map[string]string{"geo": geo}

	for _, tc := range []struct{ name, src, want string }{
		// A member that exists but is not `pub`, versus one that does not exist (ADR 0078).
		{"private", `use geo
t = geo.hidden()`, "defines hidden and does not make it `pub`"},
		{"absent", `use geo
t = geo.volume()`, "has no member volume"},
		// The anonymous entry scope printed as `module ""`.
		{"clash", `use geo
use geo/other as geo
t = geo.area(1, 2)`, "the program binds geo to"},
		// A misspelled path is silent at the import and fails on the member,
		// which is the wrong half of the name to look at.
		{"typo", `use gee
t = gee.area(1, 2)`, "matched no file on the search path"},
	} {
		err := load(files, tc.src, "mul")
		if err == nil {
			t.Errorf("%s: expected an error", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q does not contain %q", tc.name, err, tc.want)
		}
	}

	// A module the TARGET provides must not get the misspelled-path hint.
	if err := load(files, `use math/trig
t = trig.sin(1)`, "math/trig.sin"); err != nil {
		t.Errorf("a target-provided module must resolve: %v", err)
	}

	// A file is the module its path names (ADR 0064 item 1): it writes no
	// header, so neither a second module in one file nor a header that names
	// another path can be written, which today's notation refused here.
}

// An integer literal is never a FLOAT, whatever its size.
//
// One too large for int64 used to fall through to ParseFloat, so
// `9223372036854775808` silently became a float and the program's type changed
// underneath it — at a threshold ten bits past the portable window and named in
// no specification (docs/spec/data-model.md §1.1). It is now desugared into
// in-window arithmetic instead, and the thing that must not regress is that it
// stays an integer.
func TestOversizedIntegerLiteralIsRefused(t *testing.T) {
	for _, ok := range []string{"0", "-7", "9007199254740991", "9223372036854775807",
		"-9223372036854775808", "1e20", "1.5", "-0.25"} {
		if _, err := ReadTerm(ok); err != nil {
			t.Errorf("%s should read: %v", ok, err)
		}
	}
	// AND ONE PAST int64 IS NO LONGER REFUSED — it desugars into in-window
	// arithmetic (see TestABigLiteralIsAValueBuiltFromSmallOnes). What must not
	// regress is that it is never a FLOAT: the bug this test was written for was
	// `9223372036854775808` falling through to ParseFloat.
	for _, big := range []string{"9223372036854775808", "99999999999999999999",
		"-99999999999999999999"} {
		got, err := ReadTerm(big)
		if err != nil {
			t.Errorf("%s should read as a desugared spine: %v", big, err)
			continue
		}
		if got.Kind == KFloat {
			t.Errorf("%s became a float", big)
		}
	}
	// And the ones that DO read are integers, not floats.
	for _, n := range []string{"9007199254740991", "9223372036854775807"} {
		got, err := ReadTerm(n)
		if err != nil {
			t.Fatal(err)
		}
		if got.Kind != KInt {
			t.Errorf("%s read as kind %d, want KInt", n, got.Kind)
		}
	}
}
