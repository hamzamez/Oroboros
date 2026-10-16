package core

import (
	"strings"
	"testing"
)

// THREE READINGS OF A TARGET FILE the survey generators found when they began
// writing J (jswitch5b, step 5b.3), each a declaration no hand file had made.
func TestJReadsWhatTheGeneratorsWrite(t *testing.T) {
	read := func(src, target string) string {
		t.Helper()
		ts, err := ReadTarget(src, target, ".", ".")
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		var b strings.Builder
		for _, f := range ts {
			b.WriteString(f.String())
		}
		return b.String()
	}
	// A QUOTED NAME IS THE NAME ITSELF: text/scanner's constant `Int` is not the
	// language's set, which `Int` unquoted is (σ_J, ADR 0067 item 3).
	if got := read("module m {\n    `Int` ∈ set { -3 } = host \"scanner.Int\"\n}\n", ""); !strings.Contains(got, "(const Int -3") {
		t.Errorf("a quoted constant was read as the set: %s", got)
	}
	if got := read("module m {\n    Int ∈ set { -3 } = host \"scanner.Int\"\n}\n", ""); !strings.Contains(got, "(const int -3") {
		t.Errorf("the control: unquoted, the name is the set's: %s", got)
	}
	// A quoted name is one operand of a model's form: `underlying `a_b` -1..1`
	// does not subtract.
	if got := read("underlying `elf-R_386` -1..1\n", ""); !strings.Contains(got, "(underlying elf-R_386 (int -1 1))") {
		t.Errorf("a quoted operand took the next one as a difference: %s", got)
	}
	// A QUOTED SPLICE NAMES ITS PARAMETER: `${`scope_id`}` is the hole of the
	// binder `scope_id`, as the binder is quoted.
	if got := read("module m {\n    fn f(`scope_id` ∈ Int) (Int) { host expr \"F(${`scope_id`})\" }\n}\n", ""); !strings.Contains(got, `"F(%s)"`) {
		t.Errorf("a quoted splice was left unfilled: %s", got)
	}
	// PAST THE NINTH OPERAND an x86 hole is braced: `%10` is operand 1 and `0`.
	var ps, holes []string
	for _, n := range strings.Split("abcdefghij", "") {
		ps = append(ps, n+" ∈ Int")
		holes = append(holes, "${"+n+"}")
	}
	src := "module m {\n    fn f(" + strings.Join(ps, ", ") + ") (Int) { host stmt \"" + strings.Join(holes, " ") + "\" }\n}\n"
	if got := read(src, "windows"); !strings.Contains(got, "%9 %{10}") {
		t.Errorf("the tenth operand's hole: %s", got)
	}
}

// σ_J IS INJECTIVE ON THE LANGUAGE'S SETS: the name `Int` is quoted, since `Int`
// unquoted is the set `int` (ADR 0067 item 3), and the two would spell alike.
func TestSpellQuotesALanguageSetsSpelling(t *testing.T) {
	for n, want := range map[string]string{"Int": "`Int`", "Bool": "`Bool`", "int": "int", "Index": "Index"} {
		if got := Spell(n); got != want {
			t.Errorf("Spell(%q) = %s, want %s", n, got, want)
		}
	}
}
