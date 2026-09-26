package emit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
)

// A MODULE NAME WITH NO TARGET PREFIX IS A CLAIM, AND THIS IS THE CHECK.
//
// `go/fmt` names Go's `fmt` package and claims nothing beyond that host, and it
// lives in `targets/`. `os` and `io` name a `Σ` several targets share —
// target-system.md's `Decl ≅ Σ × I`, interface common and implementation per
// host — and they live in `lib/`, as `(provides T M …)` cells, because the
// host's API is what this project claims it can parasitize and a portable name
// over it is a claim about several hosts agreeing. See `lib/os/README.md`.
//
// So the target must be loaded through its LAYERS here: a `provides` is the
// lowest layer of `Δ_T`, and `LoadTarget` on one directory cannot see it.
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
		tg, err := LoadTargetLayers(n, []string{"../targets"}, []string{"../lib"})
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

// A CONCRETE TYPE GOES WHERE AN INTERFACE IS WANTED, AND THE COERCION IS THE
// IDENTITY.
//
// An interface is an existential type and PACKING one is manufacturing a closure
// (callbacks.md tier 3, refused). PASSING one is neither: `io.ReadAll(f)` asks
// for an `*os.File` and the HOST inserts the coercion, so what is missing is a
// fact the type checker needs and the backend does not — `⟦coerce⟧ = id`
// (docs/interfaces.md §3).
//
// The relation is DECLARED, ground and finite, so this is a lookup: no variance,
// no inference, and none of Pierce's F<: because after staging nothing is
// quantified.
func TestAConcreteTypeGoesWhereAnInterfaceIsWanted(t *testing.T) {
	load := func(src string) *Target {
		t.Helper()
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "x", "x.oro"), []byte(withWord(src)), 0o644); err != nil {
			t.Fatal(err)
		}
		tg, err := LoadTargetLayers("x", []string{dir})
		if err != nil {
			t.Fatal(err)
		}
		return tg
	}
	tg := load(`(target x
  (backend go)
  (type file (host "*os.File"))
  (type reader (host "io.Reader"))
  (type closer (host "io.Closer"))
  (type rc (host "io.ReadCloser"))
  (implements file rc)
  (implements rc reader closer)
  (module x
    (sig open ((p string)) file (host expr "os.Open(%s)"))
    (sig slurp ((r reader)) int (host expr "io.ReadAll(%s)"))))`)

	// TRANSITIVITY IS NOT DECORATION: `io.ReadCloser` embeds `io.Reader`, so a
	// type declared to satisfy the first satisfies the second. A target file
	// that had to spell out every consequence would be stating a closure by
	// hand and getting it wrong.
	if !tg.Subsumes("file", "reader") {
		t.Errorf("the relation must be transitive: file -> rc -> reader")
	}
	// ANTISYMMETRIC, which is the whole difference from `compatible`.
	// Subsumption FORGETS every method but the interface's own, and forgetting
	// has a direction.
	if tg.Subsumes("reader", "file") {
		t.Errorf("an io.Reader does not go where an *os.File is wanted")
	}
	if tg.Subsumes("closer", "reader") {
		t.Errorf("a Closer is not a Reader; nothing declared that")
	}

	prog := func(tg *Target, src string) error {
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		p, _, err := core.Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		env, err := tg.Env(p)
		if err != nil {
			t.Fatal(err)
		}
		nf, err := core.Normalize(p.Defs["f"], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		return Check(tg, "f", nf)
	}
	const src = `
(use x)
(export f)
(def f (fn () (x.slurp (x.open "go.mod"))))`
	if err := prog(tg, src); err != nil {
		t.Errorf("a file where a reader is wanted: %v", err)
	}

	// THE CONTROL, and it is what makes the test mean anything: with the edges
	// removed the same program must be REFUSED. A test whose passing and
	// failing cases look identical proves nothing.
	bare := load(`(target x
  (backend go)
  (type file (host "*os.File"))
  (type reader (host "io.Reader"))
  (module x
    (sig open ((p string)) file (host expr "os.Open(%s)"))
    (sig slurp ((r reader)) int (host expr "io.ReadAll(%s)"))))`)
	err := prog(bare, src)
	if err == nil {
		t.Fatal("without a declared edge the program must be refused")
	}
	if !strings.Contains(err.Error(), "file") || !strings.Contains(err.Error(), "reader") {
		t.Errorf("the refusal should name both types, got %v", err)
	}
}
