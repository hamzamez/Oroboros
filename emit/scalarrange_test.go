package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A SCALAR RANGE IS A TYPE, AND IT HAS THREE EFFECTS THAT MUST STAY APART.
//
// `(sig sq ((n (int 0 1000))) int)` parses today and is then REFUSED — "n is
// int 0 1000, but int is required here" — because `core.ValueType`, which says
// what a range MEANS as opposed to how it is stored, was called at exactly one
// site: the table-read path. So the range language worked on array elements and
// nowhere else, and a scalar's range had to be spelled as a `where`.
// ADR 0019 names closing that as owed, and owed whichever option wins.
//
// The three effects, because conflating any two of them is a real bug shape:
//
//	(1) TYPE. `(int LO HI)` IS `int`. Nothing about the range changes what
//	    operations accept the value, so a declared range must be normalised
//	    wherever a declared type becomes a DEMAND on a term.
//
//	(2) PREMISE. `n : (int LO HI)` means `LO ≤ n ≤ HI`, which is exactly the
//	    conjunct `(and (<= LO n) (<= n HI))`. Since `where` is already read by
//	    the refinement layer, the interval layer and termination, the range
//	    becomes that conjunct and no analysis learns a new thing exists.
//
//	(3) REPRESENTATION. Which rung of ADR 0003's ladder the value is stored on.
//	    That is ADR 0019's and is NOT this change: a scalar is the host's word
//	    at every finite range, and only a table's element slot consults a width.
//
// Effect (1) without (2) would make the declaration decorative. Effect (1)
// bleeding into (3) is the one `ValueType` was written to prevent — a byte-
// ranged parameter must not narrow a counter that reads it, or the counter
// overflows at 255 while the language says integers do not.

// The type half. This is the refusal ADR 0019 recorded, and it is a refusal of
// a LEGAL program: the syntax parses and then every use of the parameter is a
// type error.
func TestScalarRangeIsAnInt(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (go.* n n))")
	if err := CheckAgainstSig(tg, "sq", sigOf(t, "(int 0 1000)"), body); err != nil {
		t.Fatalf("a declared range was refused where `int` is required: %v\n"+
			"A range IS an int (core.ValueType). It says what the value is, not "+
			"what operations accept it.", err)
	}
}

// EFFECT (1) MUST NOT BECOME EFFECT (3), and the honest version of that claim
// is narrower than the one this test was first written to make.
//
// It first asserted that a buffer storing `n*1000`, for `n` declared 0..255,
// must NOT narrow. It narrowed to four bytes, and the compiler was right: the
// premise gives `n*1000 ∈ [0, 255000]`, which fits an int32, and deriving a
// buffer's element width from what its stores can hold is exactly what
// elemwidth's write side is for. So a scalar's declared range now propagates
// through arithmetic into representation selection, which is a gain rather than
// a leak — and the test was wrong, which is the correct way round.
//
// What must NOT happen is the thing `core.ValueType` exists to prevent: the
// range becoming the width of the value ITSELF. A parameter declared 0..255 is
// an integer that happens to satisfy a bound, so it is passed in the host's own
// word and it is `int` to every operation. Both halves are pinned here, and the
// second half is what stops "normalise a range to int" being implemented as
// "accept anything".
func TestAScalarRangeIsAnIntNotAWidth(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	sig := sigOf(t, "(int 0 255)")
	body := mustRead(t, "(fn (n) (go.* n n))")

	// It IS an int: an integer operation accepts it.
	if err := CheckAgainstSig(tg, "sq", sig, body); err != nil {
		t.Fatalf("a 0..255 parameter was refused by an integer operation: %v", err)
	}

	// And it is not a licence to accept anything: a float operation must still
	// refuse it. Without this the check above passes under a `compatible` that
	// gave up rather than one that normalised.
	f := mustRead(t, "(fn (n) (go.f* n n))")
	if fErr := CheckAgainstSig(tg, "sq", sig, f); fErr == nil {
		t.Error("a 0..255 parameter was accepted where f64 is required; " +
			"normalising a range to `int` must not weaken the checker")
	}
}

// A RANGE IN THE RESULT POSITION IS THE DUAL, a guarantee: its test is
// TestARangedResultIsChecked in ensures_ir_test.go, where the IR decides it.

// sigOf reads a one-parameter signature THROUGH THE READER, which is the only
// producer of a signature with named parameters and therefore the only place a
// range's premise half can be desugared. Building a `core.Sig` literal here
// would test a path no program takes.
// AN EMPTY RANGE IS NOT A TYPE. `(int 100 0)` denotes ∅: no value inhabits it,
// so a parameter declared with one can never be called. It is a transposition
// typo, and it used to flow on as the string "int 100 0" and surface much later
// as "n is int 100 0, but int is required here" — a type mismatch reported
// against the wrong thing, at the wrong place, for the wrong reason.
func TestAnEmptyRangeIsRefusedWhereItIsWritten(t *testing.T) {
	if _, err := core.Read("(sig f ((n (int 100 0))) int)"); err == nil {
		t.Error("an empty range was accepted as a type")
	} else if !strings.Contains(err.Error(), "(int 100 0)") {
		t.Errorf("the error does not name the range that is wrong: %v", err)
	}
	// The control: a single-point range is legal and must stay legal, or the
	// refusal above could be implemented as "lo must be less than hi".
	if _, err := core.Read("(sig f ((n (int 5 5))) int)"); err != nil {
		t.Errorf("a single-point range was refused: %v", err)
	}
}

func resultSigOf(t *testing.T, param, result string) *core.Sig {
	t.Helper()
	forms, err := core.Read("(sig sq ((n " + param + ")) " + result + ")")
	if err != nil || len(forms) != 1 || forms[0].Sig == nil {
		t.Fatalf("read sig %s -> %s: %v", param, result, err)
	}
	return forms[0].Sig
}

func sigOf(t *testing.T, decl string) *core.Sig {
	t.Helper()
	forms, err := core.Read("(sig sq ((n " + decl + ")) int)")
	if err != nil || len(forms) != 1 || forms[0].Sig == nil {
		t.Fatalf("read sig %s: %v", decl, err)
	}
	return forms[0].Sig
}
