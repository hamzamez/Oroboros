package core

import (
	"strings"
	"testing"
)

// A DEFINITION INTRODUCES A FRESH NAME: no module may define a name of Σ_L, the
// language's signature. In a nested module the definition captured the
// construct (a function returned 7 whatever its argument); in the root it was
// ignored beside the target's. Both are refused, and the refusal says why.
func TestNoModuleDefinesALanguageName(t *testing.T) {
	for name, src := range map[string]string{
		"if in the root":              "fn if(c, a, b) { 7 }\nexport fn run(x) { x }\n",
		"if in a nested module":       "module m {\n    fn if(c, a, b) { 7 }\n    pub fn f(x) { if x > 3 { 1 } else { 2 } }\n}\n",
		"an operator":                 "fn `+`(a, b) { 7 }\nexport fn run(x) { x + 1 }\n",
		"a table operation":           "fn len(a) { 0 }\nexport fn run(x) { x }\n",
		"the crash expect expands to": "fn abandon(w) { 0 }\nexport fn run(x) { x }\n",
	} {
		forms, err := Read(src)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		_, _, err = Load(forms)
		if err == nil || !strings.Contains(err.Error(), "which is the language's own") ||
			!strings.Contains(err.Error(), "introduces a fresh name") {
			t.Errorf("%s: want the refusal with its reason, got %v", name, err)
		}
	}
	// The control: a fresh name, and a language word as a member of another
	// module's name, which is no definition of it.
	forms, err := Read("fn iff(c, a, b) { a }\nexport fn run(x) { iff(true, x, 0) }\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(forms); err != nil {
		t.Errorf("a fresh name must load: %v", err)
	}
}
