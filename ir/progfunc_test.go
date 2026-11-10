package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// lowerOn lowers src's one export on windows, unchanged by the pipeline.
func lowerOn(t *testing.T, src string) (*emit.Target, *Func) {
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
	f, err := Lower(tg, q, prog.Sigs[q], nf, Options{Decided: true})
	if err != nil {
		t.Fatal(err)
	}
	return tg, f
}

// W13, A PROGRAM FUNCTION'S SIDE OF THE FRAME RULE (ADR 0101): one returning
// its parameter is refused, for the reason, and one returning a table born in
// it passes, so the check refuses something false before it passes something
// true.
func TestAProgramFunctionReturnsATableBornInIt(t *testing.T) {
	_, f := lowerOn(t, "export fn f(a ∈ Array(Int)) (Array(Int)) { a }\n")
	if err := CheckBornIn(nil, f); err == nil || !strings.Contains(err.Error(), "W13") {
		t.Errorf("a function returning its parameter: want W13's refusal, got %v", err)
	}
	_, g := lowerOn(t, "export fn f(n ∈ 1..10) (Array(Int)) { var b := array { 0 : _ ∈ 0..n-1 }; b }\n")
	if err := CheckBornIn(nil, g); err != nil {
		t.Errorf("a function returning a table it made is refused: %v", err)
	}
}

// THE OBLIGATION AT A CALL (ADR 0101): a program function is decided under
// its parameters' sets, so a call whose argument table has elements outside
// them is refused. THE PLANTED FAULT asks nothing, and the call passes.
func TestACallsArgumentsAreInItsParametersSets(t *testing.T) {
	tg, f := lowerOn(t, "export fn f(a ∈ Array(Int), b ∈ Array(Int)) (Int) { len(a) + len(b) }\n")
	name, err := natCallee(tg, "add")
	if err != nil {
		t.Fatal(err)
	}
	r := V(len(f.Types))
	f.Types = append(f.Types, "array int")
	f.Body.Stmts = append([]Stmt{{Op: OCall, Name: name, Args: []V{f.Params[0], f.Params[1]}, Res: []V{r}}}, f.Body.Stmts...)
	if err := CheckCallSets(tg, f); err == nil || !strings.Contains(err.Error(), "not inside its parameter's") {
		t.Errorf("a call with tables of unknown elements: want the obligation's refusal, got %v", err)
	}
	PlantNoCallSets = true
	defer func() { PlantNoCallSets = false }()
	if err := CheckCallSets(tg, f); err != nil {
		t.Errorf("with the obligation unasked the call is still refused: the witness cannot fail: %v", err)
	}
}

// THE FRAME RULE AT A CALL (ADR 0101): a program function's result is a birth
// its caller owns, so the caller releases it at its death. THE PLANTED FAULT
// reads the call as a host's, its result owned by nobody, and fewer tables are
// released.
func TestACallsResultIsReleasedByItsCaller(t *testing.T) {
	const src = "export fn f(n ∈ 0..100) (String) {\n" +
		"    big_str(loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) })\n}\n"
	count := func() int {
		emit.ResetImports()
		tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
		if err != nil {
			t.Fatal(err)
		}
		forms, _ := core.Read(src)
		prog, _, err := tg.LoadProgram(forms)
		if err != nil {
			t.Fatal(err)
		}
		env, _ := tg.Env(prog)
		q := prog.Exports[0]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		f, _, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil, false, nil)
		if err != nil {
			t.Fatal(err)
		}
		// a release is realized as the OS layer's free, by its size class
		n := 0
		f.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op == OCall && (strings.HasSuffix(s.Name, "HeapFree") || strings.HasSuffix(s.Name, "VirtualFree")) {
					n++
				}
			}
		})
		return n
	}
	want := count()
	if want < 1 {
		t.Fatalf("want the call's results released; %d releases", want)
	}
	PlantCallArgsEscape = true
	defer func() { PlantCallArgsEscape = false }()
	if got := count(); got >= want {
		t.Errorf("with the call read as a host's, %d releases, not fewer than %d: the witness cannot fail", got, want)
	}
}
