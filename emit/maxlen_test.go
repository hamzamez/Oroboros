package emit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A LENGTH IS BOUNDED AT BOTH ENDS: by max-len_T, which is the word's bound
// where a target declares nothing (ADR 0026) and 2^31−1 on Java, whose
// `arraylength` returns an `int`. So a counter under a `len` guard is bounded
// on every target. Before lengths were bounded, `(len a)` was `[0, +inf)` and
// this exact shape was 32 of the corpus's unproven operations. (Which host type
// the counter then takes is ρ of its range, the printer's: an `int` on Java.)
func TestLengthIsBounded(t *testing.T) {
	for _, c := range []struct {
		dir, op string
	}{
		{"../targets/go", "go"},     // the word's bound
		{"../targets/java", "java"}, // declared: 2^31−1
	} {
		tg, err := emit.LoadTarget(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		src := "(fn (a) (loop ((i 0)) (" + c.op + ".>= i (len a)) i " +
			"else (again (" + c.op + ".+ i 1))))"
		terms, err := core.ReadAll(src)
		if err != nil || len(terms) != 1 {
			t.Fatalf("%s: read: %v", c.op, err)
		}
		rep := decideOn(t, tg, nil, terms[0])

		if rep.Ops == 0 {
			t.Fatalf("%s: nothing was counted, so this test proves nothing", c.op)
		}
		if rep.Proven != rep.Ops {
			t.Fatalf("%s: %d of %d operations bounded, want all of them; a "+
				"counter under a `len` guard is bounded because a LENGTH is",
				c.op, rep.Proven, rep.Ops)
		}
	}
}

// A DECLARED BOUND MUST BE TIGHTER THAN THE LANGUAGE'S, not looser. A target
// claiming it can hold more elements than an `int` can count is claiming a
// length the language cannot represent, which is not a length.
func TestMaxLenBeyondTheWindowIsRefused(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if got := tg.MaxLenOf(); got != tg.Word.Hi || got != 9223372036854775807 {
		t.Fatalf("a target that declares no max-len gets its word's bound (ADR 0026); "+
			"got %d, want %d", got, tg.Word.Hi)
	}
	jv, err := emit.LoadTarget("../targets/java")
	if err != nil {
		t.Fatal(err)
	}
	if got := jv.MaxLenOf(); got != 2147483647 {
		t.Fatalf("java declares max-len 2147483647; got %d", got)
	}

	// A bound one past JavaScript's word, on a target whose word that is: a
	// length the target cannot count is not a length (ADR 0026).
	path := filepath.Join(t.TempDir(), "bad.oro")
	src := "(target bad (repr (int -9007199254740991 9007199254740991) word) " +
		"(fact max-len ((a (array A))) (<= (len a) 9007199254740992)))"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := emit.LoadTarget(path); err == nil {
		t.Fatal("a max-len past the target's word must be refused")
	} else if !strings.Contains(err.Error(), "outside its word") {
		t.Fatalf("the refusal should say why: %v", err)
	}
}
