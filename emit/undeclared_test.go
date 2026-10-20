package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// AN INDEX IS AN INTEGER, so a parameter applied to a λ is no indexing: it is a
// value of unknown set applied to a continuation, a sum's elimination. Read as
// one, lib/result.oro's err_map compiled alone was refused for `0 <= λ`
// (jmsg-2026-10-20 §5).
func TestAnApplicationToALambdaIsNoIndexing(t *testing.T) {
	tg, err := LoadTarget("../targets/js")
	if err != nil {
		t.Fatal(err)
	}
	term, err := core.ParseTerm("(fn (r) (r (fn (x) x)))")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Refine(tg, "f", nil, term); err != nil && strings.Contains(err.Error(), "is an indexing") {
		t.Errorf("a λ argument was read as an index: %v", err)
	}
	// The control: an integer argument is an indexing, refused when unbounded.
	term, _ = core.ParseTerm("(fn (r k) (r k))")
	if _, err := Refine(tg, "f", nil, term); err == nil || !strings.Contains(err.Error(), "is an indexing") {
		t.Errorf("an unbounded index must be refused as an indexing, got %v", err)
	}
}

// A PARAMETER NO HEADER GIVES A SET, READ AS A TABLE, IS SAID TO BE: a table's
// read and a function's call are one form, so the refusal names the reading and
// what a function there would mean. A declared table gets no such note.
func TestAnUndeclaredParametersReadingIsNamed(t *testing.T) {
	tg, err := LoadTarget("../targets/js")
	if err != nil {
		t.Fatal(err)
	}
	term, _ := core.ParseTerm("(fn (f k) (f k))")
	_, err = Refine(tg, "g", nil, term)
	if err == nil || !strings.Contains(err.Error(), "f is a parameter no header gives a set") {
		t.Errorf("an undeclared parameter's reading as a table must be named, got %v", err)
	}
	sig := &core.Sig{Params: []core.SigParam{{Name: "f", Type: "array int"}, {Name: "k", Type: "int"}}, Result: "int"}
	_, err = Refine(tg, "g", sig, term)
	if err == nil || strings.Contains(err.Error(), "no header gives a set") {
		t.Errorf("a declared table is refused for its index, with no note about its set: %v", err)
	}
}
