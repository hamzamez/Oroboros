package core

import (
	"strings"
	"testing"
)

// entryTerm is the term a test reduces: the program's definition t, or, while
// a snippet is still today's notation, its bare top-level term (jswitch, step
// 5b: J has no bare term at the top level).
func entryTerm(t *testing.T, prog *Program, terms []*Term) *Term {
	t.Helper()
	if len(terms) > 0 {
		return terms[0]
	}
	if d, ok := prog.Defs["t"]; ok {
		return d
	}
	for name, d := range prog.Defs {
		if strings.HasSuffix(name, ".t") {
			return d
		}
	}
	t.Fatal("the program defines no t")
	return nil
}
