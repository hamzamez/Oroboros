package emit

import (
	"fmt"
	"strings"
	"testing"

	"oroboros/core"
)

// A MODULE NAME WITH NO TARGET PREFIX IS A CLAIM, AND THIS IS THE CHECK.
//
// `go/fmt` names Go's `fmt` package and claims nothing beyond that host.
// `os` and `io` name a `Σ` several targets share — target-system.md's
// `Decl ≅ Σ × I`, with the interface common and the implementation per host —
// and a program whose free names lie inside `Σ` is portable by the computation
// ADR 0001 already does.
//
// A claim nothing checks is decoration, which is `split-words`'s lesson: it
// passed every review for two months while returning different answers on
// different targets. So the claim is checked structurally here — same names,
// same argument types, same result types — and behaviourally by the three tools
// in `examples/io/`, which produce byte-identical output on three hosts.
func TestTheUnprefixedModulesShareOneInterface(t *testing.T) {
	targets := []string{"go", "js", "java"}
	loaded := map[string]*Target{}
	for _, n := range targets {
		tg, err := LoadTarget("../targets/" + n)
		if err != nil {
			t.Fatalf("%s: %v", n, err)
		}
		loaded[n] = tg
	}
	// The signature of one module on one target: name -> "args -> result".
	sigsOf := func(tg *Target, mod string) map[string]string {
		out := map[string]string{}
		for name, p := range tg.Prims {
			if !strings.HasPrefix(name, mod+".") {
				continue
			}
			out[strings.TrimPrefix(name, mod+".")] =
				fmt.Sprintf("(%s) -> %s", strings.Join(p.Args, ", "),
					strings.Join(append(append([]string{}, p.Results...), p.Result), ""))
		}
		return out
	}
	for _, mod := range []string{"os", "io"} {
		ref := sigsOf(loaded["go"], mod)
		if len(ref) == 0 {
			t.Fatalf("targets/go declares no module %q", mod)
		}
		for _, n := range targets[1:] {
			got := sigsOf(loaded[n], mod)
			for name, want := range ref {
				have, ok := got[name]
				if !ok {
					t.Errorf("%s declares %s.%s and %s does not — "+
						"an unprefixed module is a claim every target must meet",
						"go", mod, name, n)
					continue
				}
				if have != want {
					t.Errorf("%s.%s: go says %s, %s says %s — same name, different Σ",
						mod, name, want, n, have)
				}
			}
			// And the other direction: a target may not quietly add to a shared
			// interface, because a program written against the richer one would
			// look portable and not be.
			for name := range got {
				if _, ok := ref[name]; !ok {
					t.Errorf("%s declares %s.%s and go does not", n, mod, name)
				}
			}
		}
	}
}

// THE DESTINATION HOLES ARE FILLED BEFORE THE ARGUMENT HOLES, AND THE ORDER IS
// THE WHOLE OF WHY THE FIRST JAVASCRIPT BUILD EMITTED NONSENSE.
//
// `fill` is `fmt.Sprintf`, so a `%r` it meets is an unknown verb and the WHOLE
// template comes back as `%!r(MISSING)` — silently, as a string that compiles to
// nothing on any host. The control below is that exact wrong order.
func TestDestinationHolesAreFilledFirst(t *testing.T) {
	form := `try { %r0 = fs.readFileSync(%s); %r1 = null; } ` +
		`catch (e) { %r0 = null; %r1 = e; }`
	dests := []string{"src", "err"}
	vals := []any{`path`}

	got := fill(fillDests(form, dests), vals)
	want := `try { src = fs.readFileSync(path); err = null; } ` +
		`catch (e) { src = null; err = e; }`
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
	// The control. A test whose passing and failing cases look the same proves
	// nothing, so the wrong order is exercised and required to be wrong.
	if bad := fillDests(fill(form, vals), dests); !strings.Contains(bad, "%!") {
		t.Errorf("filling arguments first must mangle the template, got %s", bad)
	}

	// And the SHAPE test, which is what selects between the two emissions: a
	// template naming every destination assigns them; anything else is an
	// expression whose value carries them.
	if !multiPrimDests(form, 2) {
		t.Error("a template naming %r0 and %r1 assigns its results")
	}
	if multiPrimDests("os.ReadFile(%s)", 2) {
		t.Error("a template naming no destination does not assign them")
	}
	// A template naming SOME of them is not the assigning shape — a partial
	// answer here would leave a destination declared and never written, which is
	// a definite-assignment error on Java and `undefined` on JavaScript.
	if multiPrimDests("try { %r0 = f(%s); }", 2) {
		t.Error("a template naming only %r0 does not assign two results")
	}
}

// THE TOTALISATION, EMITTED, ON THE HOST THAT SIGNALS FAILURE BY THROWING.
//
// A fallible host call is a partial map `A ⇀ B`, which is a total `A → B + E`.
// No host gives the coproduct: Go gives a product plus a convention and Node
// throws. So the target declares the CALL that produces the pair and the
// DISCRIMINATOR that reads it, and the compiler learns nothing about exceptions
// — which is what this checks, on the emitted text.
func TestJavaScriptEmitsTheTotalisationAndItsImport(t *testing.T) {
	for k := range JSImports {
		delete(JSImports, k)
	}
	tg, err := LoadTarget("../targets/js")
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
	out, err := JSFunc(tg, "main", prog.Sigs["main"], nf)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"let src", "try {", "fs.readFileSync", "catch (e)", "=== null",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// THE IMPORT IS THE OTHER HALF, and it is the half this backend did not
	// have: `(import …)` is a field Go, Java and x86 all honour and JavaScript
	// dropped, because no target file for this host had ever declared one.
	if !JSImports["node:fs"] {
		t.Error("the primitive's (import \"node:fs\") was dropped")
	}
	if got := jsImportAlias("node:fs"); got != "fs" {
		t.Errorf("the alias of node:fs is fs, got %q", got)
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
	for k := range JavaImports {
		delete(JavaImports, k)
	}
	tg, err := LoadTarget("../targets/java")
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
	out, err := JavaMethod(tg, "main", prog.Sigs["main"], nf)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"short[] src", // the widened element, not byte[]
		"Exception err",
		"try {", "Files.readAllBytes", "catch (Exception",
		"== null",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	// DECLARED, THEN ASSIGNED — not `final var`, because every arm of the
	// template writes every destination and javac's definite-assignment check is
	// what then enforces that the two halves of a totalisation agree.
	if strings.Contains(out, "final short[] src") {
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
	tg, err := LoadTarget("../targets/java")
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
    else (let (word-end s i) (fn (ni) (again ni (if (> ni i) 1 0)))))))`
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
	out, err := JavaMethod(tg, "scan", prog.Sigs["scan"], nf)
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
