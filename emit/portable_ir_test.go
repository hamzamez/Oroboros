package emit_test

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
	"oroboros/ir/java"
	"oroboros/ir/js"
)

// THE TOTALISATION, EMITTED, ON THE HOST THAT SIGNALS FAILURE BY THROWING.
//
// A fallible host call is a partial map `A ⇀ B`, which is a total `A → B + E`.
// No host gives the coproduct: Go gives a product plus a convention and Node
// throws. So the target declares the CALL that produces the pair and the
// DISCRIMINATOR that reads it, and the compiler learns nothing about exceptions
// — which is what this checks, on the emitted text.
func TestJavaScriptEmitsTheTotalisationAndItsImport(t *testing.T) {
	for k := range emit.JSImports {
		delete(emit.JSImports, k)
	}
	tg, err := emit.LoadTargetLayers("js", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(`
(use os)
(export main)
(def main (fn () ((os.ReadFile "f") (fn (src err)
  (if (os.err-nil err) (len src) 0)))))`)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["main"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, err := js.FromResidual(tg, "main", prog.Sigs["main"], nf)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`let v\d+`, `try \{`, `fs\.readFileSync`, `catch \(e\)`, `=== null`,
	} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	// THE IMPORT IS THE OTHER HALF, and it is the half this backend did not
	// have: `(import …)` is a field Go, Java and x86 all honour and JavaScript
	// dropped, because no target file for this host had ever declared one.
	if !emit.JSImports["node:fs"] {
		t.Error("the primitive's (import \"node:fs\") was dropped")
	}
	if got := emit.JSImportAlias("node:fs"); got != "fs" {
		t.Errorf("the alias of node:fs is fs, got %q", got)
	}
}

// AND AN IMPORT IS RECORDED WHEREVER A PRIM IS RESOLVED, not only on the
// several-results path.
//
// The test above passes with the bug present, because `os.ReadFile` is fallible
// and therefore multi-result, and `emitMultiPrim` collected the import while the
// ORDINARY prim path did not. Every prim that had ever carried an `(import …)`
// on this host was multi-result, so the hole was invisible until a generated
// declaration of the whole Node runtime called `path.join()` — which emitted a
// ReferenceError with no diagnostic from us.
//
// *A path nothing runs is a path nothing checks*, and the fix is one line at the
// site where the prim is looked up. This case is deliberately the SIMPLEST
// shape: one result, one import, no continuation.
func TestASingleResultPrimKeepsItsImport(t *testing.T) {
	for k := range emit.JSImports {
		delete(emit.JSImports, k)
	}
	tg, err := emit.LoadTargetLayers("js", []string{"../targets"})
	if err != nil {
		t.Fatal(err)
	}
	// The declaration a generated file writes, added directly rather than
	// through a temporary layer: what is under test is the EMITTER, and going
	// through the loader would test the loader too.
	tg.Prims["js/pathgen.join"] = emit.Prim{
		Name: "js/pathgen.join", Result: "any", Kind: "expr",
		Form: "path.join()", Import: "node:path",
	}
	src, err := core.Read(`
(use js/pathgen as p)
(export main)
(def main (fn () (p.join)))`)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(src)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["main"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.FromResidual(tg, "main", prog.Sigs["main"], nf); err != nil {
		t.Fatal(err)
	}
	if !emit.JSImports["node:path"] {
		t.Error("a single-result prim dropped its (import \"node:path\"): " +
			"the emitted call names a binding nothing creates")
	}
}

// THE SAME CALL ON THE HOST WHERE FAILURE IS IN THE TYPES, plus the price the
// JVM charges for a byte.
//
// `Files.readAllBytes` gives a `byte[]` and the JVM's byte is SIGNED, so a file
// byte does not fit it (elemwidth-2026-08-27). Declaring the element −128..127
// would make `(src i)` answer −1 for 0xFF and the same program differ across
// hosts, so this target widens at the boundary — and the DECLARATION is what
// must survive, which is what the emitted `short[]` checks.
func TestJavaEmitsDeclaredDestinationsAndTheWidening(t *testing.T) {
	for k := range emit.JavaImports {
		delete(emit.JavaImports, k)
	}
	tg, err := emit.LoadTargetLayers("java", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(`
(use os)
(export main)
(def main (fn () ((os.ReadFile "f") (fn (src err)
  (if (os.err-nil err) (len src) 0)))))`)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["main"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, err := java.FromResidual(tg, "main", prog.Sigs["main"], nf)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`short\[\] v\d+;`, // the widened element, not byte[]
		`Exception v\d+;`,
		`try \{`, `Files\.readAllBytes`, `catch \(Exception`,
		`== null`,
	} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	// DECLARED, THEN ASSIGNED — not `final var`, because every arm of the
	// template writes every destination and javac's definite-assignment check is
	// what then enforces that the two halves of a totalisation agree.
	if strings.Contains(out, "final short[]") {
		t.Error("a destination the template assigns may not be final")
	}
}

// A NARROWED LOOP VARIABLE TAKES A CAST ON ITS INITIALISER.
//
// Narrowing is decided per loop, so an INNER loop can narrow while the outer
// does not — and the inner's initial value is computed from the outer's
// variable, which is then a `long` assigned to an `int`. javac calls it
// "possible lossy conversion" and refuses the file; nothing else would see it.
// monotone-2026-08-27 closed the half where the inner loop's EXITS are read;
// this is the half where its ENTRY is written.
//
// Found by the first program with a nested scanner to reach this host —
// examples/io/freq.oro's word scanner — which is the argument for programs over
// construct suites arriving again.
func TestANarrowedLoopInitialiserIsCast(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/java")
	if err != nil {
		t.Fatal(err)
	}
	src := `
(use java)
(export scan)
(sig scan ((s (array (int 0 255)))) int (where (< (len s) 1000)))
(def word-end (fn (s i)
  (loop ((j (+ i 1)))
    (>= j (len s))  j
    (= (s j) 32)    j
    else            (again (+ j 1)))))
(def scan (fn (s)
  (loop ((i 0) (n 0))
    (>= i (len s)) n
    else (let ni (word-end s i)
           (again ni (if (> ni i) 1 0))))))`
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["scan"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, err := java.FromResidual(tg, "scan", prog.Sigs["scan"], nf)
	if err != nil {
		t.Fatal(err)
	}
	// The property, stated rather than spelled: every `int` local whose
	// initialiser mentions a `long` local carries a cast. Checking it directly
	// is what makes the test survive a change of variable names.
	longs := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) >= 4 && f[0] == "long" && f[2] == "=" {
			longs[f[1]] = true
		}
		if len(f) >= 4 && f[0] == "int" && f[2] == "=" {
			rhs := strings.Join(f[3:], " ")
			for l := range longs {
				if strings.Contains(rhs, l) && !strings.Contains(rhs, "(int)") {
					t.Errorf("a narrowed initialiser reads the wide %s "+
						"without a cast: %s", l, strings.TrimSpace(line))
				}
			}
		}
	}
	// The anti-vacuity guard. Refusing to narrow is always safe, so a test that
	// only ever saw wide locals would pass forever while checking nothing.
	if !strings.Contains(out, "int ") || len(longs) == 0 {
		t.Skipf("neither shape present; this test needs one narrow and one wide "+
			"local to mean anything:\n%s", out)
	}
}

// A BUFFER NOBODY WRITES TO TAKES ITS ELEMENT TYPE FROM THE HOST CALL IT IS
// PASSED TO, because the host is the thing that writes it.
//
// `build` zero-fills and a scratch buffer handed to `(*os.File).Read` has no
// `set` anywhere, so the syntactic inference correctly has nothing to say — and
// the buffer came out `[]int`, which that method does not take. Found on the
// first program to call a GENERATED Go method, which is 3,098 of the 4,932
// callable names in that ecosystem and none of which had ever been called.
//
// The rule is consulted ONLY where the stores decide nothing, and the control
// below is that half: a buffer the program writes literals into keeps the range
// its own stores give it, because narrowing it to what a host expects would
// truncate them silently.
func TestAnUnwrittenBufferTakesTheHostsDeclaredElement(t *testing.T) {
	tg, err := emit.LoadTargetLayers("go", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	// `os.WriteFile`'s second parameter is `(array (int 0 255))` — a real
	// declaration from `lib/os/go.oro`, so the test cannot drift from what a
	// target actually says.
	body := func(src string) *core.Term {
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, _, err := core.Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		env, err := tg.Env(prog)
		if err != nil {
			t.Fatal(err)
		}
		nf, err := core.Normalize(prog.Defs["f"], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		return nf
	}
	got, err := golang.FromResidual(tg, "f", nil, body(`
(use os)
(export f)
(def f (fn () (build 8 (fn (b) (os.WriteFile "x" b 420)))))`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "make([]byte, 8)") {
		t.Errorf("a buffer only the host writes takes the host's element:\n%s", got)
	}
	// THE CONTROL. A buffer the program stores into must hold what its stores
	// say, and here the host's declaration fixes it at a byte while a store puts
	// 100000 in it. That is refused: the declared element is an obligation on
	// the class (Theorem D′'s premise, irstep4a-2026-09-26). The term backend
	// printed a []int and handed it to a function taking []byte, which Go
	// rejects; and its version of this control consumed `b` twice, which only
	// passed because the linearity check is not on this path.
	_, err = golang.FromResidual(tg, "g", nil, body(`
(use os)
(export f)
(def f (fn () (build 8 (fn (b) (os.WriteFile "x" (set b 0 100000) 420)))))`))
	if err == nil || !strings.Contains(err.Error(), "int 0 255") {
		t.Errorf("a byte buffer storing 100000 must be refused, naming the declared element; got %v", err)
	}
}
