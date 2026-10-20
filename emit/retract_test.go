package emit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A FALLIBLE DECLARATION IS THE HOST'S PRODUCT COMPOSED WITH A RETRACTION
// (spec/errors.md §4, retract.go). H(R) is the raw call's results, and the
// definition applies r. Each row is one shape of R.
func retractTarget(t *testing.T, decls string) (*Target, error) {
	t.Helper()
	dir := t.TempDir()
	lib := filepath.Join(dir, "lib")
	if err := os.MkdirAll(lib, 0o755); err != nil {
		t.Fatal(err)
	}
	res := `set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant
set Two(T, E) = 'fine(T) ∪ 'bad(E) ∪ 'worse(E) success 'fine
`
	if err := os.WriteFile(filepath.Join(lib, "result.oro"), []byte(res), 0o644); err != nil {
		t.Fatal(err)
	}
	src := `backend go
repr -2^63..2^63-1 word
set error = host "error"
repr error niche host expr "%s == nil"
module m {
    use result
    ` + decls + `
}
`
	// The target file beside the library, where J's reader finds the sets
	// `result` declares.
	p := filepath.Join(lib, "t.oro")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return LoadTargetLayers("t.oro", []string{lib}, []string{lib})
}

func TestARetractionSplitsADeclarationIntoItsRawCallAndADefinition(t *testing.T) {
	for _, c := range []struct {
		decl, raw string
		results   []string
		def       string
	}{
		{`fn f(n ∈ Int) (result.Result(Int, error)) { host expr "f(${n})" }`, "#raw:m:f",
			[]string{"int", "error"}, "(if (#niche:error #h2) (result.ok #h1) (result.err #h2))"},
		{`fn g(n ∈ Int) (result.Result((), error)) { host expr "g(${n})" }`, "#raw:m:g",
			[]string{"error"}, "(if (#niche:error #h1) (result.ok (fn (#k) (#k))) (result.err #h1))"},
		{`fn h(n ∈ Int) (Int × Option(error)) { host expr "h(${n})" }`, "#raw:m:h",
			[]string{"int", "error"}, "(fn (#k) (#k #h1 (if (#niche:error #h2) none (some #h2))))"},
	} {
		tg, err := retractTarget(t, c.decl)
		if err != nil {
			t.Errorf("%s: %v", c.decl, err)
			continue
		}
		raw, ok := tg.Prims[c.raw]
		if !ok {
			t.Errorf("%s: no raw call %s", c.decl, c.raw)
			continue
		}
		got := raw.Results
		if len(got) == 0 {
			got = []string{raw.Result}
		}
		if strings.Join(got, " ") != strings.Join(c.results, " ") {
			t.Errorf("%s: H(R) = %v, want %v", c.decl, got, c.results)
		}
		def := ""
		for _, f := range tg.Defs["m"] {
			if f.Kind == "def" {
				def = f.Term.String()
			}
		}
		if !strings.Contains(def, c.def) {
			t.Errorf("%s: r is %s, want it to contain %s", c.decl, def, c.def)
		}
		if _, still := tg.Prims["m."+strings.TrimPrefix(c.raw, "#raw:m:")]; still {
			t.Errorf("%s: the declared name is still a primitive; a program would reach the product", c.decl)
		}
	}
}

func TestARetractionIsRefusedWhereTheAlgebraFails(t *testing.T) {
	for _, c := range []struct{ decl, want string }{
		{`fn f(n ∈ Int) (result.Two(Int, error)) { host expr "f(${n})" }`, "exactly one"},
		{`fn f(n ∈ Int) (result.Result(Int, Int)) { host expr "f(${n})" }`, "no niche"},
		{`fn f(n ∈ Int) (other.Result(Int, error)) { host expr "f(${n})" }`, "no `use`"},
	} {
		if _, err := retractTarget(t, c.decl); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: want a refusal mentioning %q, got %v", c.decl, c.want, err)
		}
	}
}
