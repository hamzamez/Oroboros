package emit_test

import (
	"regexp"
	"testing"

	"oroboros/emit"
	"oroboros/ir/golang"
)

// THE SAME HAZARD AT THE EMITTER, which the checker-level test above cannot
// see. `seedFromSig` put the declared type straight into the emitter's type map
// and `Target.ty` spells a range as its narrowest representation, so
// `(sig sq ((n (int 0 1000))) int)` emitted `func GenSq(n uint16)` — and `n * n`
// at uint16 wraps at 65536, returning 16960 for 1000*1000.
//
// It was LATENT: a scalar range was refused by the checker, so nothing ever
// reached that line. A refusal was standing in front of a wrong answer, and
// removing the refusal is what exposed it.
//
// Stated as the theorem rather than as a spelling: a range and the `where` it
// means are the same declaration, so they must EMIT THE SAME FUNCTION. That is
// stronger than asserting `int` appears, and it is the property that would have
// caught this.
func TestScalarRangeEmitsWhatTheWhereEmits(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (go.* n n))")

	ranged := sigOf(t, "(int 0 1000)")
	whered := sigOf(t, "int")
	whered.Where = mustRead(t, "(and (<= 0 n) (<= n 1000))")

	a, err := golang.FromResidual(tg, "sq", ranged, body)
	if err != nil {
		t.Fatalf("a declared range failed to emit: %v", err)
	}
	b, err := golang.FromResidual(tg, "sq", whered, body)
	if err != nil {
		t.Fatalf("the `where` spelling failed to emit: %v", err)
	}
	if a != b {
		t.Errorf("a range and the `where` it means emit different functions:\n"+
			"range:\n%s\nwhere:\n%s", a, b)
	}
	// And the control, so the test cannot pass by both being degenerate: the
	// parameter really is the host's own word, not a two-byte one.
	if !regexp.MustCompile(`\(v\d+ int\)`).MatchString(a) {
		t.Errorf("parameter is not `int`:\n%s\nA scalar's range is what the "+
			"value IS; only a table's element slot consults a width", a)
	}
}
