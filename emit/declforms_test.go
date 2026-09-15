package emit

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// THE DECLARATION FORMS OF theories.md §8, as a target file writes them. Each
// refusal is checked to fire and to name what was wrong; each legitimate shape
// beside it is checked to load, so a test cannot pass by refusing everything.

func loadOne(t *testing.T, body string) (*Target, error) {
	t.Helper()
	dir := t.TempDir()
	writeTarget(t, filepath.Join(dir, "x"), "a", body)
	return LoadTargetLayers("x", []string{dir})
}

func TestARespelledFormIsRefusedWithItsNewSpelling(t *testing.T) {
	for old, spelled := range respelled {
		for _, body := range []string{
			"(target x (" + old + " a b))",
			"(target x (module m (" + old + " a b)))",
		} {
			_, err := loadOne(t, body)
			if err == nil || !strings.Contains(err.Error(), spelled) {
				t.Errorf("%s: want a refusal naming %s, got %v", body, spelled, err)
			}
		}
	}
}

func TestASigSeparatesItsTwoKindsOfClaim(t *testing.T) {
	refused := map[string]string{
		`(sig f (int) int pure)`:                                    "no (host …) clause",
		`(sig f (int) int (import "x") (host expr "F(%s)"))`:        "goes inside (host …)",
		`(sig f (int) int (host expr "F(%s)" pure))`:                "(host …) takes",
		`(sig f (int) int (host expr "F(%s)") (host expr "G(%s)"))`: "twice",
		`(sig f (int) int (host expr))`:                             `(host KIND "template"`,
		`(sig f (int) int "F(%s)" (host expr "F(%s)"))`:             "unexpected",
	}
	for decl, why := range refused {
		if _, err := loadOne(t, "(target x "+decl+")"); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
	// And the legitimate shapes: every clause in its place, and arity zero as ().
	tg, err := loadOne(t, `(target x
	  (sig g ((a int) (b int)) int pure (where (<= 0 a)) (ensures (<= 0 result))
	       (host expr "G(%s, %s)" (import "g") (checked g) (jump "l")))
	  (sig null () ptr pure (host expr "0")))`)
	if err != nil {
		t.Fatal(err)
	}
	g := tg.Prims["g"]
	if !g.Pure || g.Where == nil || g.Ensures == nil || g.Import != "g" || g.Checked != "g" ||
		g.Jump != "l" || g.Form != "G(%s, %s)" || len(g.Args) != 2 {
		t.Errorf("a clause was lost: %+v", g)
	}
	if n := tg.Prims["null"]; len(n.Args) != 0 || !n.Pure {
		t.Errorf("() must be arity zero: %+v", n)
	}
}

// A CONSTANT IS THE SIG IT MEANS: E(const) = sig, checked as structural equality
// of the loaded declarations, at the top level and inside a module.
func TestAConstIsTheSigItMeans(t *testing.T) {
	for _, wrap := range []func(string) string{
		func(d string) string { return "(target x " + d + ")" },
		func(d string) string { return "(target x (module m " + d + "))" },
	} {
		c, err := loadOne(t, wrap(`(const Max -7 (host "p.Max" (import "p")))`))
		if err != nil {
			t.Fatal(err)
		}
		s, err := loadOne(t, wrap(`(sig Max () (int -7 -7) pure (host expr "p.Max" (import "p")))`))
		if err != nil {
			t.Fatal(err)
		}
		for n, p := range s.Prims {
			if q := c.Prims[n]; !reflect.DeepEqual(p, q) {
				t.Errorf("%s: a const must load to the sig it elaborates to:\n const %+v\n sig   %+v", n, q, p)
			}
		}
		if len(c.Prims) != len(s.Prims) {
			t.Errorf("a const declared %d names where its sig declares %d", len(c.Prims), len(s.Prims))
		}
	}
	refused := map[string]string{
		`(const Pi 3.14 (host "math.Pi"))`: "not an integer literal",
		`(const S "x" (host "p.S"))`:       "not an integer literal",
		`(const N 4)`:                      "(const NAME INTEGER",
		`(const N 4 (host expr "p.N"))`:    "has no kind",
		`(const N 4 pure (host "p.N"))`:    "(const NAME INTEGER",
	}
	for decl, why := range refused {
		if _, err := loadOne(t, "(target x "+decl+")"); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
}

func TestReprAndFactAreRefusedOffTheirShapes(t *testing.T) {
	refused := map[string]string{
		`(repr big medium)`:                       "(repr big host) or (repr big limbs)",
		`(repr (int 5 1) (host "x"))`:             "is empty",
		`(repr shift 64)`:                         "1 <= N <= 63",
		`(repr map hash)`:                         "(repr map host) or (repr map library)",
		`(repr float (host "double"))`:            "(repr (int LO HI)",
		`(fact f ((a (array A))) (< (len a) 3))`:  "facts are specified",
		`(fact f ((a (array A))) (<= (len b) 3))`: "facts are specified",
		`(fact f ((a (array A))) (<= (len a) 0))`: "at least 1",
		`(type t "spelling")`:                     `(type NAME (host "spelling"))`,
	}
	for decl, why := range refused {
		if _, err := loadOne(t, "(target x "+decl+")"); err == nil || !strings.Contains(err.Error(), why) {
			t.Errorf("%s: want a refusal containing %q, got %v", decl, why, err)
		}
	}
	tg, err := loadOne(t, `(target x (repr big limbs) (repr shift 31) (repr (ref int) (host "Long"))
	  (fact max-len ((xs (array A))) (<= (len xs) 2147483647)) (type (map K V) (host "M<%s,%s>")))`)
	if err != nil {
		t.Fatal(err)
	}
	if tg.BigRepr != "limbs" || tg.ShiftWidth != 31 || tg.Boxed["int"] != "Long" ||
		tg.MaxLen != 2147483647 || tg.MapType != "M<%s,%s>" {
		t.Errorf("a declaration was lost: %+v", tg)
	}
}

// A HEADER AND THE FORMS AFTER IT ARE ONE FILE'S GLUE, so writing a form inside
// the header's parentheses or after them is the same target.
func TestFormsAfterTheHeaderAreTheSameFragment(t *testing.T) {
	inside, err := loadOne(t, `(target x (backend go) (type t (host "T")) (sig f (int) int (host expr "F(%s)")))`)
	if err != nil {
		t.Fatal(err)
	}
	after, err := loadOne(t, "(target x (backend go))\n(type t (host \"T\"))\n(sig f (int) int (host expr \"F(%s)\"))")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inside, after) {
		t.Error("the same forms inside and after the header load to different targets")
	}
}

// THE MAP REPRESENTATION COMPOSES BY OVERRIDE (theories.md §5.8). It was a flag
// joined with ||, so a nearer layer could turn the language's own map on and
// never off.
func TestANearerLayerCanTurnTheLibraryMapOff(t *testing.T) {
	root := t.TempDir()
	host, lib := filepath.Join(root, "host"), filepath.Join(root, "lib")
	writeTarget(t, filepath.Join(host, "x"), "a", `(target x (repr map host))`)
	writeTarget(t, filepath.Join(lib, "x"), "a", `(target x (repr map library))`)
	for _, c := range []struct {
		layers []string
		want   bool
	}{{[]string{host, lib}, false}, {[]string{lib, host}, true}} {
		tg, err := LoadTargetLayers("x", c.layers)
		if err != nil {
			t.Fatal(err)
		}
		if got := tg.NeedsMapImpl(); got != c.want {
			t.Errorf("layers %v: NeedsMapImpl is %v, want %v — the nearer layer must win", c.layers, got, c.want)
		}
	}
}

// A LIBRARY'S DECLARATION THAT DOES NOT LOAD IS REFUSED. loadProvides skipped
// every form but `prim` and ended a file's walk on the first error, in silence,
// so a native fast path could vanish from a build with no diagnostic.
func TestABrokenProvidesIsRefusedNotDropped(t *testing.T) {
	root := t.TempDir()
	targets, lib := filepath.Join(root, "targets"), filepath.Join(root, "lib")
	writeTarget(t, filepath.Join(targets, "x"), "a", `(target x (backend go))`)
	writeTarget(t, filepath.Join(lib, "m"), "x", `(provides x m (sig f (int) int pure))`)
	_, err := LoadTargetLayers("x", []string{targets}, []string{lib})
	if err == nil || !strings.Contains(err.Error(), "no (host …) clause") {
		t.Errorf("a provides with no realization must be refused, got %v", err)
	}
	writeTarget(t, filepath.Join(lib, "m"), "x", `(provides x m (sig f (int) int pure (host expr "F(%s)")))`)
	tg, err := LoadTargetLayers("x", []string{targets}, []string{lib})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tg.Prims["m.f"]; !ok {
		t.Error("the corrected provides must contribute m.f")
	}
}
