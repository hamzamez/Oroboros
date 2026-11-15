package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AN IMPURE APPLICATION IS NOT FOLDED (the book's 2026-10-06 pass, finding 3;
// ADR 0010). An effectful application is not its value: with `*` impure,
// `1 * 2` stays a call, and `fn(n) { 7 }(3 * 4)` keeps the call it binds; with
// `*` pure both fold. THE PLANTED FAULT folds the language's operators whatever
// their purity: the call is evaluated away.
func TestAnImpureApplicationIsNotFolded(t *testing.T) {
	reduce := func(prims ...string) string {
		forms, err := Read("t = fn() { 1 * 2 }\n")
		if err != nil {
			t.Fatal(err)
		}
		prog, terms, err := Load(forms)
		if err != nil {
			t.Fatal(err)
		}
		nf, err := Normalize(entryTerm(t, prog, terms), testEnv(prog, prims...), DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		return ShowTerm(nf)
	}
	if got := reduce("*"); !strings.Contains(got, "2") || strings.Contains(got, "*") {
		t.Errorf("1 * 2 with * pure: %s, want 2", got)
	}
	if got := reduce("!*"); !strings.Contains(got, "*") {
		t.Errorf("1 * 2 with * impure: %s, want the call kept", got)
	}
	PlantFoldImpure = true
	defer func() { PlantFoldImpure = false }()
	if got := reduce("!*"); strings.Contains(got, "*") {
		t.Errorf("with the fold blind to purity: %s, want 2", got)
	}
}

// A MODULE PATH IS RESOLVED CASE-EXACTLY (finding 17, ADR 0025). Resolution is
// a function of the source: `use Geometry` names Geometry.oro, and on a file
// system that ignores case it read geometry.oro, where elsewhere the same
// program was refused. THE PLANTED FAULT asks the file system: on a
// case-insensitive one the wrong file is then found.
func TestAModulePathIsResolvedCaseExactly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "geometry.oro"), []byte("pub fn area(w, h) { w * h }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	find := SearchPath([]string{dir})
	if _, ok, err := find("geometry"); !ok || err != nil {
		t.Fatalf("use geometry: found %v, %v", ok, err)
	}
	if _, ok, _ := find("Geometry"); ok {
		t.Errorf("use Geometry found geometry.oro; want no file")
	}
	_, blind := os.Stat(filepath.Join(dir, "GEOMETRY.oro"))
	PlantCaseBlind = true
	defer func() { PlantCaseBlind = false }()
	if _, ok, _ := find("Geometry"); blind == nil && !ok {
		t.Errorf("with the file system's comparison, on a case-insensitive one: Geometry not found; want geometry.oro")
	}
}
