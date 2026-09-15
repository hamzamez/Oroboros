package main

import (
	"strings"
	"testing"
)

// THE COMPARISON MUST SEE EVERY KIND OF CHANGE, and the old sweep did not see
// two of them: it deleted a file that failed to emit, so "now refused" looked
// like "never swept", and it recorded no compiler output, so a weaker proof was
// invisible until it became a refusal.

func o(src, tgt, hash string, lines ...string) outcome {
	return outcome{Source: src, Target: tgt, Emitted: hash != "", Hash: hash, Lines: lines}
}

func kinds(cs []change) string {
	var s []string
	for _, c := range cs {
		s = append(s, string(c.kind)+"@"+c.key)
	}
	return strings.Join(s, "; ")
}

func TestIdenticalSweepsHaveNoChanges(t *testing.T) {
	a := []outcome{o("x.oro", "go", "h1", "note: 3 of 3 integer operations bounded; 1 of 1 loop(s) proven terminating")}
	if cs := compare(a, a); len(cs) != 0 {
		t.Errorf("identical sweeps reported changes: %s", kinds(cs))
	}
}

func TestEveryKindOfChangeIsReported(t *testing.T) {
	old := []outcome{
		o("text.oro", "go", "h1"),
		o("notes.oro", "go", "h2", "note: 3 of 3 integer operations bounded; 1 of 1 loop(s) proven terminating"),
		o("refused.oro", "go", "h3"),
		o("emits.oro", "go", "", "gen: refused"),
		o("gone.oro", "go", "h4"),
	}
	new := []outcome{
		o("text.oro", "go", "h1-changed"),
		o("notes.oro", "go", "h2", "note: 2 of 3 integer operations bounded; 1 of 1 loop(s) proven terminating"),
		o("refused.oro", "go", "", "gen: now refused"),
		o("emits.oro", "go", "h5"),
		o("new.oro", "go", "h6"),
	}
	got := kinds(compare(old, new))
	for _, want := range []string{
		string(textChanged) + "@text.oro go",
		string(notesChanged) + "@notes.oro go",
		string(nowRefused) + "@refused.oro go",
		string(nowEmitted) + "@emits.oro go",
		string(removed) + "@gone.oro go",
		string(added) + "@new.oro go",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in: %s", want, got)
		}
	}
}

// A refusal whose MESSAGE changed is a change: a diagnostic is part of what the
// compiler does (spec/theories.md §10).
func TestARefusalWhoseMessageChangedIsAChange(t *testing.T) {
	old := []outcome{o("x.oro", "js", "", "gen: x is not bound")}
	new := []outcome{o("x.oro", "js", "", "gen: x (m.x) is not bound in module m, its parents, or lang")}
	if cs := compare(old, new); len(cs) != 1 || cs[0].kind != notesChanged {
		t.Errorf("a changed refusal must be reported as changed output, got: %s", kinds(cs))
	}
}

func TestOutcomesRoundTrip(t *testing.T) {
	in := []outcome{
		o("examples/b.oro", "java", "", "gen: refused because", "  known: nothing"),
		o("examples/a.oro", "windows", "abc"),
		o("examples/a.oro", "go", "def", "note: gen-a: 3 of 3 integer operations bounded; 1 of 1 loop(s) proven terminating"),
	}
	text := formatOutcomes(in)
	back, err := parseOutcomes(text)
	if err != nil {
		t.Fatal(err)
	}
	if cs := compare(in, back); len(cs) != 0 {
		t.Errorf("format then parse lost something: %s\n%s", kinds(cs), text)
	}
	// Sorted by source, then by target in the fixed order go, js, java, windows.
	if i, j := strings.Index(text, "a.oro go"), strings.Index(text, "a.oro windows"); i < 0 || j < 0 || i > j {
		t.Errorf("outcomes are not in a total order:\n%s", text)
	}
}

// The proof totals are what shows a weaker proof before it becomes a refusal.
func TestProofTotalsAddUp(t *testing.T) {
	os := []outcome{
		o("a.oro", "go", "h", "note: gen-a: 3 of 4 integer operations bounded; 1 of 2 loop(s) proven terminating"),
		o("b.oro", "go", "h", "note: gen-b: 5 of 5 integer operations bounded; 2 of 2 loop(s) proven terminating"),
	}
	b, ops, p, loops := proofTotals(os)
	if b != 8 || ops != 9 || p != 3 || loops != 4 {
		t.Errorf("totals: got %d of %d and %d of %d, want 8 of 9 and 3 of 4", b, ops, p, loops)
	}
}

func TestWhichSourcesAreCheckedIsReadNotGuessed(t *testing.T) {
	cases := []struct {
		src, first string
		want       bool
	}{
		{"gauntlet/differential/cases/sum-case.oro", "; whatever", true},
		{"examples/json/tree.oro", "; BUILT WITH `-checked` -- `go run ./cmd/build -checked …`.", true},
		{"examples/dot.oro", "; dot product", false},
		{"examples/int/fib.oro", "; fibonacci", false}, // meant to be refused, and swept that way
	}
	for _, c := range cases {
		if got := needsChecked(c.src, c.first); got != c.want {
			t.Errorf("%s: needsChecked = %v, want %v", c.src, got, c.want)
		}
	}
}

func TestNormaliseRemovesMachineSpecifics(t *testing.T) {
	got := normalise("gen: C:\\repo\\x.oro: no\r\n\r\nnote: fine  \r\n", `C:\repo`, `C:\repo\.check\emitted`)
	want := []string{`gen: <root>\x.oro: no`, "note: fine"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("normalise: got %q, want %q", got, want)
	}
}
