package emit

import (
	"testing"

	"oroboros/core"
)

// entryTerm is the term a test reduces: the program's definition t, J having
// no bare term at the top level (jswitch, step 5b).
func entryTerm(t *testing.T, prog *core.Program, terms []*core.Term) *core.Term {
	t.Helper()
	if len(terms) > 0 {
		return terms[0]
	}
	if d, ok := prog.Defs["t"]; ok {
		return d
	}
	t.Fatal("the program defines no t")
	return nil
}
