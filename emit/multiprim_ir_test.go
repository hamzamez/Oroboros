package emit_test

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
)

// AND THE EMITTED FORM IS THE HOST'S OWN.
func TestSeveralResultsEmitTheHostsOwnForm(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(fn () ((go/os.ReadFile "f") (fn (src err)
	           (if (go/os.err-nil err) (go.len src) 0))))`
	terms, err := core.ReadAll(src)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	out, err := golang.FromResidual(tg, "gen-read", nil, terms[0])
	if err != nil {
		t.Fatal(err)
	}
	// One assignment, both names, no product built.
	if !regexp.MustCompile(`v\d+, v\d+ := os\.ReadFile\("f"\)`).MatchString(out) {
		t.Fatalf("expected Go's own multiple assignment:\n%s", out)
	}
	if strings.Contains(out, "struct") || strings.Contains(out, "[2]") {
		t.Errorf("a product was built where the host has several results:\n%s", out)
	}
}

// A RESULT THE BODY NEVER READS is still received, because Go's multiple
// assignment is positional — and as `_`, because an unused variable is a
// compile error on that host.
func TestAnUnreadResultBecomesBlank(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	terms, err := core.ReadAll(
		`(fn () ((go/os.ReadFile "f") (fn (src err) (go.len src))))`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := golang.FromResidual(tg, "gen-read", nil, terms[0])
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`v\d+, _ := os\.ReadFile`).MatchString(out) {
		t.Fatalf("the unread result must be `_`, or Go refuses the file:\n%s", out)
	}
}
