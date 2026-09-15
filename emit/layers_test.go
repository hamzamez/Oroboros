package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTarget is a one-file layer, so the tests below read as the algebra does.
func writeTarget(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".oro"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// GLUE IS ORDER-FREE; OVERRIDE IS ORDERED. target-system.md §2.
//
// The two operations that compose targets differ only at a collision, and this
// is that difference stated as a test: within a layer a repeated name is an
// error whichever file is read first, and between layers the nearer one wins,
// silently and deterministically.
func TestGlueIsOrderFreeAndOverrideIsOrdered(t *testing.T) {
	root := t.TempDir()

	// Two layers, both declaring `f`, disagreeing on purpose.
	near, far := filepath.Join(root, "near"), filepath.Join(root, "far")
	writeTarget(t, filepath.Join(near, "x"), "a",
		`(target x (backend go) (sig f (int) int pure (host expr "NEAR(%s)")))`)
	writeTarget(t, filepath.Join(far, "x"), "a",
		`(target x (backend go) (sig f (int) int pure (host expr "FAR(%s)"))
		            (sig g (int) int pure (host expr "ONLYFAR(%s)")))`)

	tg, err := LoadTargetLayers("x", []string{near, far})
	if err != nil {
		t.Fatal(err)
	}
	if got := tg.Prims["f"].Form; got != "NEAR(%s)" {
		t.Errorf("`f` resolved to %q; the NEARER layer must win", got)
	}
	// And the far layer still contributes everything the near one did not say.
	if _, ok := tg.Prims["g"]; !ok {
		t.Error("`g` was lost; override replaces a NAME, not a layer")
	}

	// Reversing the order reverses the answer — which is what makes ▷ ordered
	// and distinguishes it from ⊔.
	tg2, err := LoadTargetLayers("x", []string{far, near})
	if err != nil {
		t.Fatal(err)
	}
	if got := tg2.Prims["f"].Form; got != "FAR(%s)" {
		t.Errorf("with the layers reversed `f` is %q; override must depend on order", got)
	}

	// WITHIN one layer the same collision is an ERROR, whichever file is read
	// first — the sheaf condition, and the reason a directory's file order
	// cannot change a program.
	one := filepath.Join(root, "one")
	writeTarget(t, filepath.Join(one, "x"), "a",
		`(target x (backend go) (sig f (int) int pure (host expr "A(%s)")))`)
	writeTarget(t, filepath.Join(one, "x"), "b",
		`(target x (sig f (int) int pure (host expr "B(%s)")))`)
	if _, err := LoadTargetLayers("x", []string{one}); err == nil {
		t.Error("two files in ONE layer declaring `f` must be refused; glue requires agreement")
	}
}

// A LAYER THAT DOES NOT HAVE THE TARGET CONTRIBUTES NOTHING.
//
// The empty fragment is `⊔`'s identity, so an absent layer needs no special
// case — which is the practical half of the question this was built for: a
// user's target can live beside the program AND the built-ins stay on the path.
func TestAnAbsentLayerIsTheIdentity(t *testing.T) {
	root := t.TempDir()
	writeTarget(t, filepath.Join(root, "x"), "a",
		`(target x (backend go) (sig f (int) int pure (host expr "%s")))`)
	tg, err := LoadTargetLayers("x", []string{filepath.Join(root, "nothing-here"), root})
	if err != nil {
		t.Fatalf("an absent layer must be skipped, not refused: %v", err)
	}
	if _, ok := tg.Prims["f"]; !ok {
		t.Error("the layer that does have the target contributed nothing")
	}
	// And a name no layer has is still an error, with the path in the message.
	_, err = LoadTargetLayers("nosuch", []string{root})
	if err == nil || !strings.Contains(err.Error(), root) {
		t.Errorf("a missing target must name where it looked; got %v", err)
	}
}

// A LIBRARY MAY DECLARE A TARGET'S NATIVE — target-system.md §8, modules.md §6.
//
// `(provides T M decl…)` is exactly `(target T (module M decl…))` written in a
// library file, so it is a target FRAGMENT and joins the same chain. It was
// specified in August and nothing parsed it, which is why a library with a
// native fast path needed a file inside `targets/` for every host it supported.
//
// It is the LOWEST layer: a library's opinion about a host loses to the target's
// own files, which are the authority on that host.
func TestALibraryMayProvideANative(t *testing.T) {
	root := t.TempDir()
	lib := filepath.Join(root, "lib")
	writeTarget(t, lib, "words-go",
		`(provides go std/words (sig shout (string) string pure (host expr "UP(%s)")))`)

	tg, err := LoadTargetLayers("go", []string{"../targets"}, []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	p, ok := tg.Prims["std/words.shout"]
	if !ok {
		t.Fatal("the library's (provides go std/words …) did not reach the target")
	}
	if p.Form != "UP(%s)" {
		t.Errorf("shout is %q, want the library's declaration", p.Form)
	}
	// A `provides` for a DIFFERENT target must not leak into this one.
	writeTarget(t, lib, "words-js",
		`(provides js std/words (sig shout (string) string pure (host expr "JS(%s)")))`)
	tg2, err := LoadTargetLayers("go", []string{"../targets"}, []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	if got := tg2.Prims["std/words.shout"].Form; got != "UP(%s)" {
		t.Errorf("shout is %q; a (provides js …) must not reach the go target", got)
	}
}
