package core

import (
	"sort"
	"strings"
	"testing"
)

// A MODULE'S INTERFACE IS WHAT IT MARKS `pub`, AND ITS ENTRY POINTS WHAT IT MARKS
// `export` (ADR 0078): two relations, I and E ⊆ Σ, read by resolution and by
// the build.

func loadWithLibs(t *testing.T, libs map[string]string, src string) (*Program, error) {
	t.Helper()
	forms, err := Read(src)
	if err != nil {
		return nil, err
	}
	p, _, err := LoadWith(forms, snippetResolver(libs))
	return p, err
}

// An unmarked declaration is private, and an interface with nothing in it
// hides everything. Until ADR 0078 an empty interface read as "no signature
// yet", so a module that hid every name hid none.
func TestAnEmptyInterfaceHidesEverything(t *testing.T) {
	libs := map[string]string{"geo": "fn area(w, h) { w }\nhidden = 1\n"}
	for _, use := range []string{"geo.area(1, 2)", "geo.hidden"} {
		_, err := loadWithLibs(t, libs, "use geo\nt = "+use+"\n")
		if err == nil || !strings.Contains(err.Error(), "does not make it `pub`") {
			t.Errorf("%s from a module that marks nothing `pub`: want a refusal, got %v", use, err)
		}
	}
	// The control: the same module with its interface marked.
	libs["geo"] = "pub fn area(w, h) { w }\nhidden = 1\n"
	if _, err := loadWithLibs(t, libs, "use geo\nt = geo.area(1, 2)\n"); err != nil {
		t.Errorf("a `pub` definition must resolve: %v", err)
	}
	// A nested module is a module too.
	src := "module m {\n    fn f(x) { x }\n}\nmodule main {\n    use m\n    t = m.f(1)\n}\n"
	if _, err := loadWithLibs(t, nil, src); err == nil || !strings.Contains(err.Error(), "does not make it `pub`") {
		t.Errorf("a nested module's unmarked definition: want a refusal, got %v", err)
	}
}

// `private` is the default, and writing it is refused with the rule.
func TestPrivateIsTheDefault(t *testing.T) {
	_, err := Read("private fn f(x) { x }\n")
	if err == nil || !strings.Contains(err.Error(), "private to its module unless it is marked `pub`") {
		t.Errorf("want `private` refused naming the rule, got %v", err)
	}
}

// `pub` and `export` are independent: the build compiles exactly the `export`s
// of the module compiled, whatever its interface, and an importer sees exactly
// its `pub`s, whatever its entry points.
func TestPubAndExportAreTwoRelations(t *testing.T) {
	src := "module lib {\n    pub fn a(x) { x }\n    export fn b(x) { x }\n    pub export fn c(x) { x }\n    export pub fn d(x) { x }\n    fn e(x) { x }\n}\n"
	p, err := loadWithLibs(t, nil, src)
	if err != nil {
		t.Fatal(err)
	}
	entries := append([]string(nil), p.Exports...)
	sort.Strings(entries)
	if got := strings.Join(entries, " "); got != "lib.b lib.c lib.d" {
		t.Errorf("the entry points are the `export`s, b, c and d; got %s", got)
	}
	// What an importer sees.
	libs := map[string]string{"lib": "pub fn a(x) { x }\nexport fn b(x) { x }\npub export fn c(x) { x }\n"}
	for use, ok := range map[string]bool{"lib.a(1)": true, "lib.b(1)": false, "lib.c(1)": true} {
		_, err := loadWithLibs(t, libs, "use lib\nt = "+use+"\n")
		if (err == nil) != ok {
			t.Errorf("%s: visible %v, want %v (%v)", use, err == nil, ok, err)
		}
	}
	// A library compiled alone has the entry points it marks, and an imported
	// library's are not the program's.
	p, err = loadWithLibs(t, libs, "use lib\nexport fn main() { lib.c(1) }\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(p.Exports, " "); got != "main" {
		t.Errorf("a program's entry points are its own; got %s", got)
	}
}

// A `pub` variant puts its tags in the interface; an unmarked one keeps them.
func TestAPubVariantsTagsAreInItsInterface(t *testing.T) {
	use := "use a\nfn f(n) { case a.'ok(n) { a.'ok(v) -> v; a.'err(e) -> 0 } }\nexport fn main() { f(1) }\n"
	if _, err := loadWithLibs(t, map[string]string{"a": "pub set R = 'ok(Int) ∪ 'err(Int)\n"}, use); err != nil {
		t.Errorf("a `pub` variant's tags must resolve: %v", err)
	}
	if _, err := loadWithLibs(t, map[string]string{"a": "set R = 'ok(Int) ∪ 'err(Int)\n"}, use); err == nil {
		t.Errorf("an unmarked variant's tags must not resolve from another module")
	}
}
