package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// tableSrc makes a table of a computed length, so its storage is `region`'s
// term over HeapAlloc and VirtualAlloc (targets/win32/os.oro).
const tableSrc = "export fn f(n ∈ 1..100000) (Int) {\n" +
	"    b = alloc(array { 7 : _ ∈ 0..n-1 })\n    b[n - 1]\n}\n"

// verifiedOn takes src's export through the pipeline on windows and verifies it.
func verifiedOn(t *testing.T, src string) error {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	f, _, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil, false, nil)
	if err != nil {
		return err
	}
	return Verify(tg, &Program{Target: tg.Name, Funcs: []*Func{f}})
}

// THE UNSIGNED WORD IS CHOSEN ON ι's IMAGE TOO (ADR 0126). On windows a table's
// storage is `region`'s term over HeapAlloc and VirtualAlloc, substituted after
// the decision, and their byte count is a SIZE_T, 0..2^64-1 on LLP64: the
// unsigned word. A count computed in the word flows into it, so the substituted
// call needs its conversion, u64-of, as every flow into the other realization
// does (ADR 0033). Selected only before ι, the count reaches the call as the
// word and W5 refuses the function; the planted fault is that order. And the
// selection on the image keeps every value the first one typed: the factorial
// loop, whose numerals' digits are the unsigned word, still compiles.
func TestTheWordsAreChosenOnTheRealizationsImage(t *testing.T) {
	if err := verifiedOn(t, tableSrc); err != nil {
		t.Fatalf("a table of a computed length, its storage realized: %v", err)
	}
	if _, err := callsOn(t, factLoopSrc); err != nil {
		t.Fatalf("the factorial loop, its digits the unsigned word: %v", err)
	}
	PlantNoWordsOnImage = true
	err := verifiedOn(t, tableSrc)
	PlantNoWordsOnImage = false
	if err == nil || !strings.Contains(err.Error(), "W5") || !strings.Contains(err.Error(), "Alloc") {
		t.Errorf("with the words chosen before ι only: want W5 at an allocator's SIZE_T, got %v", err)
	}
}
