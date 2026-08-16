package core

import (
	"strings"
	"testing"
)

// core-0 §1.1 — NFC. Without it, `é` as U+00E9 and as e+U+0301 are two
// distinct identifiers that display identically.
func TestNonNFCIsRejected(t *testing.T) {
	if _, err := Read("(def caf\u0065\u0301 1)"); err == nil {
		t.Fatal("a decomposed é must be rejected")
	}
	if _, err := Read("(def caf\u00e9 1)"); err != nil {
		t.Errorf("a precomposed é is NFC and must pass: %v", err)
	}
	// A combining mark with no precomposed form is NFC-stable, and rejecting
	// it would break every script that needs one.
	if _, err := Read("(def q\u0307x 1)"); err != nil {
		t.Errorf("q + combining dot above has no precomposed form: %v", err)
	}
}

// §1.1 — case is preserved and significant for IDENTITY. What the rule forbids
// is case carrying MEANING, as in Shen's capitals-are-variables or Go's
// capitals-are-exported.
func TestCaseIsSignificantForIdentity(t *testing.T) {
	forms, err := Read("(f Xr xr XR)")
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
	if _, err := Read("(fn (x x) x)"); err == nil {
		t.Fatal("(fn (x x) x) must be rejected")
	}
	if _, err := Read("(fn (a b) a)"); err != nil {
		t.Errorf("distinct parameters must pass: %v", err)
	}
	// Nested shadowing is two abstractions, and stays legal.
	if _, err := Read("(fn (x) (fn (x) x))"); err != nil {
		t.Errorf("nested shadowing must stay legal: %v", err)
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

	e, terms := env("(fn (x) y)")
	if err := e.CheckProgram(terms); err == nil {
		t.Error("a free variable in a body must be reported")
	}

	// The classic one: a typo in a definition the program never reaches.
	e, terms = env("(def unused (fn (x) nope))\n(add 1 2)")
	err := e.CheckProgram(terms)
	if err == nil {
		t.Fatal("a typo in an unreached definition must be reported")
	}
	if !strings.Contains(err.Error(), "unused") {
		t.Errorf("the report should name the definition, got %v", err)
	}

	// Parameters, definitions and primitives are all in scope.
	e, terms = env("(def f (fn (a) (add a 1)))\n(fn (x) (f x))")
	if err := e.CheckProgram(terms); err != nil {
		t.Errorf("bound names must pass: %v", err)
	}

	// A recursive definition refers to itself, which IS in scope — the two
	// questions are separate, and only the second one rejects it (ADR 0014).
	e, terms = env("(def loop-forever (fn (n) (loop-forever n)))\n(loop-forever 1)")
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
	err := load("(def even? (fn (n) (odd? n)))\n(def odd? (fn (n) (even? n)))\n(even? 3)", "sub")
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
	if err := load("(def size (fn (v) (size v)))\n(add 1 2)", "add"); err == nil {
		t.Error("an unused recursive definition must be rejected")
	}

	// But NOT when the target provides the name natively: δ never unfolds the
	// definition, so the cycle is unreachable. This is ADR 0002's "compiling
	// up", and rejecting it would break every program built against a target
	// that happens to implement one of the library's functions.
	if err := load("(def sort (fn (v) (sort v)))\n(sort 1)", "sort"); err != nil {
		t.Errorf("a definition the target shadows must not be rejected: %v", err)
	}
}

// A binder must be a SIMPLE name. Binding a qualified one is meaningless — a λ
// cannot bind into a module — and it let a parameter shadow a module-qualified
// primitive, after which reduction applied a number to two arguments.
func TestQualifiedNameCannotBeAParameter(t *testing.T) {
	if _, err := Read("(fn (a.b) a.b)"); err == nil {
		t.Fatal("(fn (a.b) …) must be rejected")
	}
	if _, err := Read("(fn (num/f64.add) 1)"); err == nil {
		t.Fatal("binding a module-qualified name must be rejected")
	}
	// A path segment separator is still fine inside a simple name.
	if _, err := Read("(fn (a/b) a/b)"); err != nil {
		t.Errorf("`/` is an ordinary identifier character: %v", err)
	}
}

// A definition names a member of THIS module, so `.` cannot appear in it — a
// qualified name in a term always resolves to an import. Before this rule,
// `(def a.b …)` was accepted and no term could ever refer to it.
func TestQualifiedNameCannotBeDefined(t *testing.T) {
	if _, err := Read("(def a.b (fn (n) n))"); err == nil {
		t.Fatal("(def a.b …) must be rejected")
	}
	if _, err := Read("(def a/b (fn (n) n))"); err != nil {
		t.Errorf("`/` is an ordinary identifier character: %v", err)
	}
}

// An `export` or a `sig` was read off the list of DEFINITIONS, so one naming
// nothing was silently dropped: a misspelled export left a program with no
// entry points and build then reported a missing `main`.
func TestExportAndSigMustNameADefinition(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{"(def area (fn (w h) (h w)))\n(export are)", "exports are"},
		{"(def area (fn (w h) (h w)))\n(sig aera ((w num)) num)", "aera"},
	} {
		forms, err := Read(tc.src)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		_, _, err = Load(forms)
		if err == nil {
			t.Errorf("%q should be rejected", tc.src)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: error %q does not mention %q", tc.src, err, tc.want)
		}
	}
	// The same names, spelled right, must still load.
	forms, err := Read("(def area (fn (w h) (h w)))\n(sig area ((w num)) num)\n(export area)")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if _, _, err := Load(forms); err != nil {
		t.Fatalf("a correct export and sig must load: %v", err)
	}
}

// Reduction never opens a binder, so a term captured mid-flight has bound
// variables with no name and prints as `#1.0`. Both errors that carry a term
// reclose it as the stack unwinds, so the message is spelled like the source.
func TestDiagnosticsUnderABinderKeepTheirNames(t *testing.T) {
	forms, err := Read("(fn (n) ((fn (a b) (h a b)) n))")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, err := Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	_, err = Normalize(terms[0], testEnv(prog, "h"), DefaultFuel)
	if err == nil {
		t.Fatal("applying a 2-parameter fn to 1 argument must fail")
	}
	if strings.Contains(err.Error(), "#") {
		t.Errorf("diagnostic leaks the internal representation: %v", err)
	}
	for _, want := range []string{"(fn (n)", "(fn (a b)", "given 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}

	forms, _ = Read("(fn (n) ((fn (a) (a a)) (fn (a) (a a))))")
	prog, terms, _ = Load(forms)
	if _, err = Normalize(terms[0], testEnv(prog), DefaultFuel); err == nil {
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
	forms, err := Read("(def dot (fn (a b) (mul a b)))\n(def near (fn (a) (dot a a)))")
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
		prog, terms, err := LoadWith(forms, func(p string) (string, bool, error) {
			s, ok := files[p]
			return s, ok, nil
		})
		if err != nil {
			return err
		}
		e := testEnv(prog, prims...)
		e.SetUnresolved(prog.Unresolved)
		return e.CheckProgram(terms)
	}
	geo := "(module geo)\n(export area)\n(def area (fn (w h) (mul w h)))\n(def hidden 1)\n"
	files := map[string]string{"geo": geo}

	for _, tc := range []struct{ name, src, want string }{
		// A member that exists but is private, versus one that does not exist.
		{"private", "(use geo)\n(geo.hidden)", "defines hidden but does not export it"},
		{"absent", "(use geo)\n(geo.volume)", "has no member volume"},
		// The anonymous entry scope printed as `module ""`.
		{"clash", "(use geo)\n(use geo/other as geo)\n(geo.area 1 2)", "the program binds geo to"},
		// A misspelled path is silent at the import and fails on the member,
		// which is the wrong half of the name to look at.
		{"typo", "(use gee)\n(gee.area 1 2)", "matched no file on the search path"},
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
	if err := load(files, "(use math/trig)\n(trig.sin 1)", "math/trig.sin"); err != nil {
		t.Errorf("a target-provided module must resolve: %v", err)
	}

	// One file, one module: extras were visible only after something else had
	// imported the file that declared them, which is load-order-dependent.
	two := map[string]string{"sub/one": "(module sub/one)\n(export k)\n(def k 1)\n" +
		"(module sub/two)\n(export j)\n(def j 2)\n"}
	err := load(two, "(use sub/one)\n(sub/one.k)")
	if err == nil || !strings.Contains(err.Error(), "also declares (module sub/two)") {
		t.Errorf("a second module in a library file must be rejected, got %v", err)
	}
	// And a file whose module simply does not match the path it was found at.
	one := map[string]string{"mixup": "(module geo)\n(export a)\n(def a 1)\n"}
	err = load(one, "(use mixup)\n(mixup.a)")
	if err == nil || !strings.Contains(err.Error(), "must be the path that imports it") {
		t.Errorf("a mismatched module path must be rejected, got %v", err)
	}
}
