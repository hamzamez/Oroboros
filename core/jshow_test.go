package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// SHOWTERM IS A RIGHT INVERSE OF READING ON D'S IMAGE: for every definition the
// corpus's programs read to, ReadTerm(ShowTerm(t)) is t, so a message quoting a
// term quotes J that means it. A term holding a name no source can write (`#…`
// but the tuple's and the discard's binders) or an ascription, which J writes
// only as a binder's claim, is skipped and counted.
func TestShowTermReadsBackToItsTerm(t *testing.T) {
	var files []string
	for _, root := range []string{"../examples", "../lib", "../gauntlet/differential/cases", "../gauntlet/stdlib/acceptance"} {
		filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && strings.HasSuffix(p, ".oro") {
				files = append(files, p)
			}
			return nil
		})
	}
	checked, skipped := 0, 0
	var walk func(*Term) bool
	walk = func(x *Term) bool {
		switch x.Kind {
		case KName:
			return !strings.HasPrefix(x.Name, "#") || x.Name == "#k"
		case KFn:
			for _, p := range x.Params {
				if strings.HasPrefix(p, "#") && p != "#k" && p != "#_" {
					return false
				}
			}
			return walk(x.Body())
		case KApp:
			if len(x.Kids) > 0 && x.Kids[0].Kind == KName && x.Kids[0].Name == AscribeName {
				return false
			}
			for _, k := range x.Kids {
				if !walk(k) {
					return false
				}
			}
		}
		return true
	}
	for _, path := range files {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if IsFragment(string(b)) {
			continue
		}
		forms, err := ReadEntry(string(b), path)
		if err != nil {
			continue // a program meant to be refused by its reader
		}
		for _, f := range forms {
			if f.Kind != "def" || f.Term == nil {
				continue
			}
			if !walk(f.Term) {
				skipped++
				continue
			}
			j := ShowTerm(f.Term)
			back, err := ReadTerm(j)
			if err != nil {
				t.Errorf("%s: %s: J does not read back: %v\n  %s", path, f.Name, err, j)
				continue
			}
			if back.String() != f.Term.String() {
				t.Errorf("%s: %s reads back differently\n  J    %s\n  was  %s\n  got  %s", path, f.Name, j, f.Term, back)
			}
			checked++
		}
	}
	if checked < 200 {
		t.Errorf("only %d definitions checked (%d skipped): the law is vacuous", checked, skipped)
	}
	t.Logf("%d definitions read back to their terms; %d hold an unwritable name or an ascription", checked, skipped)
}

// THE SET RENDERING, ON EACH FORMER (ShowSet).
func TestShowSetWritesJsFormers(t *testing.T) {
	for ty, want := range map[string]string{
		"int":     "Int",
		"int 0 9": "0..9",
		"int -9223372036854775808 9223372036854775807": "-2^63..2^63-1",
		"int 0 +inf":               "0..`+inf`",
		"array int 0 255":          "Array(0..255)",
		"map int f64":              "Map(Int, F64)",
		"prod(int, array bool)":    "Int × Array(Bool)",
		"prod()":                   "()",
		"buffer int":               "buffer(Int)",
		"go.bytestring":            "go.bytestring",
		"result(int, go/io.error)": "result(Int, `go/io`.error)",
		"ptr-os-File":              "ptr_os_File",
	} {
		if got := ShowSet(ty); got != want {
			t.Errorf("ShowSet(%q) = %s, want %s", ty, got, want)
		}
	}
}

// THE TERM RENDERING ON THE FORMS A MESSAGE QUOTES: an obligation, a known fact,
// an indexing.
func TestShowTermWritesWhatTheProgramWrote(t *testing.T) {
	for src, want := range map[string]string{
		"(>= n 0)":                          "n >= 0",
		"(if (<= 0 i) (< i (len a)) false)": "0 <= i && i < len(a)",
		"(if c false true)":                 "!c",
		"(= (% x 2) 0)":                     "x % 2 == 0",
		"(* (+ a b) c)":                     "(a + b) * c",
		"(- a (- b c))":                     "a - (b - c)",
		"(go.< a b)":                        "go.`<`(a, b)",
		"(io.print-line s)":                 "io.print_line(s)",
		"((fn (x) (+ x 1)) (f y))":          "{ x = f(y); x + 1 }",
	} {
		term, err := ParseTerm(src)
		if err != nil {
			t.Fatal(err)
		}
		if got := ShowTerm(term); got != want {
			t.Errorf("ShowTerm(%s) = %s, want %s", src, got, want)
		}
	}
	if pair, err := ReadTerm("(1, f(x))"); err != nil || ShowTerm(pair) != "(1, f(x))" {
		t.Errorf("a Church tuple is its pair: %v %v", pair, err)
	}
	b, _ := ParseTerm("buf")
	i, _ := ParseTerm("(+ k 1)")
	if got := ShowIndex(b, i); got != "buf[k + 1]" {
		t.Errorf("ShowIndex = %s", got)
	}
}

// a != b IS ¬(a = b), erased by the reader as `!` is (ADR 0017): the same term
// as `!(a == b)`, and no name `!=` that a program cannot bind (shape-2026-10-21).
func TestNotEqualIsTheNegationOfEquality(t *testing.T) {
	ne, err := ReadTerm("a != b")
	if err != nil {
		t.Fatal(err)
	}
	not, _ := ReadTerm("!(a == b)")
	if ne.String() != not.String() || ne.String() != "(if (= a b) false true)" {
		t.Errorf("a != b reads as %s, !(a == b) as %s", ne, not)
	}
}
