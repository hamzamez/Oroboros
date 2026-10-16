package emit

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/j"
)

// THE REFINEMENT LAYER'S ENTAILMENT IS SOUND AGAINST A COMPLETE PROCEDURE
// (presburger-2026-10-11). Every question entails answers on a few programs of
// the corpus is put to the Omega test as well, and entails must never prove
// what the Omega test refutes: a question with an integer counterexample.
// Measured on the whole corpus, 27,693 distinct questions, it never did; this
// keeps it so.
func TestEntailmentIsSoundAgainstPresburger(t *testing.T) {
	var buf bytes.Buffer
	entailLog = &entailRecorder{out: &buf}
	defer func() { entailLog = nil }()
	tg := goNative(t)
	for _, path := range []string{"../examples/io/count.oro", "../examples/io/wc.oro",
		"../examples/lines/lines.oro", "../examples/table/sieve.oro", "../examples/json/tokenize.oro"} {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		forms, err := j.ReadEntry(string(src), path)
		if err != nil {
			t.Fatal(err)
		}
		prog, _, err := tg.LoadProgram(forms, "../lib")
		if err != nil {
			continue
		}
		env, err := tg.Env(prog)
		if err != nil {
			continue
		}
		for _, q := range prog.Exports {
			nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
			if err != nil {
				continue
			}
			Refine(tg, q, prog.Sigs[q], core.StripRequires(nf))
		}
	}
	n, proved := 0, 0
	for _, l := range strings.Split(buf.String(), "\n") {
		p := strings.Split(l, "\t")
		if len(p) < 5 {
			continue
		}
		n++
		if p[0] == "true" {
			proved++
			if p[1] == "refuted" {
				t.Errorf("entails proved a question with an integer counterexample: %s", p[4])
			}
		}
	}
	if n < 1000 || proved < 100 {
		t.Fatalf("only %d questions, %d proved: too few to witness anything", n, proved)
	}
}
