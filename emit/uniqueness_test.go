package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// ADR 0020's SIX RULES, as three properties.
//
// The decision is that a buffer is a NAMEABLE TYPE, so a function may take its
// workspace instead of building one every call. What makes it affordable is that
// ADR 0018 had already made uniqueness a distinction between two type
// CONSTRUCTORS rather than an attribute on every type — so there is no attribute
// lattice, no attribute variables and no inferred coercion, and the whole
// surface is one type name (uniqueness.md §6.1).

func checkOne(t *testing.T, src string) error {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
	if err != nil {
		return err
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		return err
	}
	env, err := tg.Env(prog)
	if err != nil {
		return err
	}
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		return err
	}
	return CheckLinear(nf, tg, prog.Sigs[q])
}

// RULE 1 AND 3: `(buffer V)` is a type, and the body must thread it.
//
// Rule 3 is the whole implementation of the linearity half: the SAME walk
// `build` already used, seeded from the signature instead of from `build`'s
// binder. `CheckLinear` with a different seed, as uniqueness.md §6.2 predicted.
func TestADeclaredBufferParameterIsLinear(t *testing.T) {
	good := `
(use go)
(export f)
(sig f ((w (buffer int))) (buffer int) (where (go.< 4 (go.len w))))
(def f (fn (w) (go.set w 0 1)))`
	if err := checkOne(t, good); err != nil {
		t.Fatalf("a threaded workspace must be accepted: %v", err)
	}

	// A USE AFTER MOVE. `set` consumes the buffer and returns it, so after the
	// store the ORIGINAL name is dead — and the read below is of the dead one.
	// This is the mistake ADR 0020 makes available that ADR 0018 did not: the
	// name is a parameter, so nothing lexically nearby says it was consumed.
	bad := `
(use go)
(export f)
(sig f ((w (buffer int))) int (where (go.< 4 (go.len w))))
(def f (fn (w) (go.+ ((go.set w 0 1) 0) (w 1))))`
	err := checkOne(t, bad)
	if err == nil {
		t.Fatal("reading a buffer parameter after storing into it must be refused")
	}
	// And the diagnostic must name ADR 0020's parameter rather than ADR 0018's
	// `build`, because the two are different promises by different parties.
	if !strings.Contains(err.Error(), "ADR 0020") {
		t.Errorf("the message should name the declared parameter:\n%v", err)
	}
}

// RULE 6: A BUFFER MAY NOT BE AN ELEMENT TYPE.
//
// Load-bearing rather than tidy. It is what keeps the read-borrow free: `(b i)`
// yields a scalar or a frozen array, so an observation CANNOT alias the buffer —
// which is why reads-do-not-consume needs none of Wadler's `let!` (1990) or
// Odersky's observers (1992). Allow a table of buffers and that argument fails.
func TestABufferIsNotAnElementType(t *testing.T) {
	for _, ty := range []string{
		"(array (buffer int))",
		"(buffer (buffer int))",
		"(map int (buffer int))",
	} {
		forms, err := core.Read("(sig f ((x " + ty + ")) int)\n(def f (fn (x) 0))")
		if err == nil {
			_, _, err = core.Load(forms)
		}
		if err == nil {
			t.Errorf("%s was accepted; a buffer may not be an element (ADR 0020 rule 6)", ty)
		}
	}
	// The control: the same shapes over an ARRAY are types, so the refusal above
	// is about buffers and not about compound types.
	for _, ty := range []string{"(array (array int))", "(array int)", "(map int int)"} {
		forms, err := core.Read("(sig f ((x " + ty + ")) int)\n(def f (fn (x) 0))")
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := core.Load(forms); err != nil {
			t.Errorf("%s must still be a type: %v", ty, err)
		}
	}
}

// A BUFFER IS A TABLE FOR EVERY PURPOSE BUT ALIASING.
//
// Its element, its width, its indexing and its bounds obligations are an
// array's; what differs is who may hold it. So every consumer that asks what a
// table holds gets one answer, and none of them learns that buffers exist.
func TestABufferIsATableEverywhereElse(t *testing.T) {
	if got := core.ArrayElem("buffer int 0 255"); got != "int 0 255" {
		t.Errorf("ArrayElem of a buffer is %q; the element question has one answer", got)
	}
	if !core.IsBuffer("buffer int") || core.IsBuffer("array int") {
		t.Error("IsBuffer must distinguish exactly the one thing that differs")
	}
}
