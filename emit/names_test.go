package emit

import (
	"path/filepath"
	"strings"
	"testing"
)

// A BARE TYPE NAME RESOLVES LEXICALLY (theories.md §3.4, ADR 0021 item 2):
// the declaring module, then each enclosing module by PATH, then the root —
// on the glued target, with shadowing refused. Every test loads a real layer.

func loadNames(t *testing.T, files map[string]string) (*Target, error) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "x")
	writeTarget(t, dir, "a", `(target x (backend go))`)
	for name, body := range files {
		writeTarget(t, dir, name, body)
	}
	return LoadTargetLayers("x", []string{filepath.Dir(dir)})
}

func TestABareTypeResolvesInItsOwnModule(t *testing.T) {
	tg, err := loadNames(t, map[string]string{"m": `(target x (module m
  (type T (host "t"))
  (sig f ((a T)) (tuple T error) pure (host expr "f(%s)"))))`})
	if err != nil {
		t.Fatal(err)
	}
	p := tg.Prims["m.f"]
	if len(p.Args) != 1 || p.Args[0] != "m.T" || len(p.Results) != 2 || p.Results[0] != "m.T" || p.Results[1] != "error" {
		t.Errorf("m.f is %v -> %v; want (m.T) -> (m.T error): a root name stays bare", p.Args, p.Results)
	}
	// AND BY THE OTHER LOADER. LoadTarget (one directory) and LoadTargetLayers
	// each carried a copy of the finishing sequence, and the resolution first
	// went into one of them only.
	dir := filepath.Join(t.TempDir(), "x")
	writeTarget(t, dir, "a", `(target x (backend go))`)
	writeTarget(t, dir, "m", `(target x (module m (type T (host "t")) (sig f ((a T)) int pure (host expr "f(%s)"))))`)
	one, err := LoadTarget(dir)
	if err != nil {
		t.Fatal(err)
	}
	if q := one.Prims["m.f"]; len(q.Args) != 1 || q.Args[0] != "m.T" {
		t.Errorf("LoadTarget left m.f taking %v; want m.T", q.Args)
	}
}

// A COMPANION NAMES ITS TYPE WITHOUT QUALIFICATION — theories.md §3.4's own
// example — and it is the path that encloses, so the flat spelling agrees.
func TestACompanionNamesItsTypeBare(t *testing.T) {
	for _, body := range []string{
		`(target x (module m (type T (host "t")) (module T (sig g ((self T)) int pure (host expr "g(%s)")))))`,
		`(target x (module m (type T (host "t"))) (module m/T (sig g ((self T)) int pure (host expr "g(%s)"))))`,
	} {
		tg, err := loadNames(t, map[string]string{"m": body})
		if err != nil {
			t.Fatal(err)
		}
		if p := tg.Prims["m/T.g"]; len(p.Args) != 1 || p.Args[0] != "m.T" {
			t.Errorf("m/T.g takes %v; want m.T, found in the enclosing module\n%s", p.Args, body)
		}
	}
}

// RESOLUTION RUNS ON THE GLUED TARGET: the type in one file, its use bare in
// another file of the same module and layer.
func TestResolutionSeesTheWholeTarget(t *testing.T) {
	tg, err := loadNames(t, map[string]string{
		"types": `(target x (module m (type T (host "t"))))`,
		"sigs":  `(target x (module m (sig f ((a T)) int pure (host expr "f(%s)"))))`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p := tg.Prims["m.f"]; len(p.Args) != 1 || p.Args[0] != "m.T" {
		t.Errorf("m.f takes %v; want m.T, declared in another file of the module", p.Args)
	}
}

// A CONSTANT IS A NAME LIKE ANY OTHER: one declared in an enclosing module is
// a range endpoint in its child.
func TestAConstantResolvesThroughAnEnclosingModule(t *testing.T) {
	tg, err := loadNames(t, map[string]string{"m": `(target x (module m
  (const Max 99 (host "Max"))
  (module k (sig f ((a (int 0 Max))) int pure (host expr "f(%s)")))))`})
	if err != nil {
		t.Fatal(err)
	}
	if p := tg.Prims["m/k.f"]; len(p.Args) != 1 || p.Args[0] != "int 0 99" {
		t.Errorf("m/k.f takes %v; want (int 0 99), Max found in m", p.Args)
	}
}

// SHADOWING IS REFUSED, NAMING BOTH: a type in a child named like one in its
// parent would rebind every bare use below it; so would one named like a root
// type. Siblings do not shadow.
func TestAModuleTypeMayNotShadowAnEnclosingOne(t *testing.T) {
	for _, c := range []struct{ body, want string }{
		{`(target x (module m (type T (host "t")) (module k (type T (host "u")))))`, "m/k.T shadows m.T"},
		{`(target x (type T (host "t")) (module m (type T (host "u"))))`, "m.T shadows T"},
	} {
		_, err := loadNames(t, map[string]string{"m": c.body})
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("want a refusal naming %q, got %v\n%s", c.want, err, c.body)
		}
	}
	if _, err := loadNames(t, map[string]string{"m": `(target x (module a (type T (host "t"))) (module b (type T (host "u"))))`}); err != nil {
		t.Errorf("a.T and b.T are siblings, not shadows: %v", err)
	}
}
