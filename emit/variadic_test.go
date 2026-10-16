package emit

import (
	"strings"
	"testing"
)

// A HOST PRECONDITION IS DECIDED BY EVALUATION where its goal is closed over
// literals (refinements.md §3a, the third route; variadic-2026-10-08).
// strings.NewReplacer requires an even count, `(= (% (len oldnew) 2) 0)`: the
// linear fragment can express neither `%` nor a literal table's length, so
// until the route a six-element literal was refused. Evaluation decides only
// what it evaluates: a table the prover knows nothing about is still refused,
// and one whose evenness the caller declares is proven by the assumption.
func TestAHostPreconditionOnLiteralsIsDecidedByEvaluation(t *testing.T) {
	const head = "use go\nuse go/strings as strings\nexport "
	for _, c := range []struct {
		body, want string // want "" when accepted
	}{
		{"fn f(s ∈ go.bytestring) (`go/strings`.Replacer) { strings.NewReplacer(\"a\", s, \"b\", \"2\") }", ""},
		{"fn f() (`go/strings`.Replacer) { strings.NewReplacer() }", ""},
		{"fn f(s ∈ go.bytestring) (`go/strings`.Replacer) { strings.NewReplacer(\"a\", s, \"b\") }", "which is false at this call"},
		{"fn f(xs ∈ Array(go.bytestring)) (`go/strings`.Replacer) { strings.NewReplacer(xs...) }", "which does not follow"},
		{"fn f(xs ∈ Array(go.bytestring)) (`go/strings`.Replacer) where len(xs) % 2 == 0 {\n    strings.NewReplacer(xs...)\n}", ""},
	} {
		err := refineGo(t, head+c.body)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("%s: refused: %v", c.body, err)
		case c.want != "" && err == nil:
			t.Errorf("%s: accepted, and it should be refused (%s)", c.body, c.want)
		case c.want != "" && !strings.Contains(err.Error(), c.want):
			t.Errorf("%s: want %q, got %v", c.body, c.want, err)
		}
	}
}
