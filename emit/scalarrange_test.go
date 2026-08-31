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

// The premise half, with a control. A range must be BELIEVED, and the only way
// to know a test of that proves anything is to run the same program without it:
// `(go.* n n)` is 0 of 1 operations bounded with nothing declared and 1 of 1
// with the range, so the passing case and the failing case genuinely differ.
func TestScalarRangeIsAPremise(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (go.* n n))")

	for _, c := range []struct {
		what   string
		sig    *core.Sig
		proven int
	}{
		{"nothing declared", sigOf(t, "int"), 0},
		{"a declared range", sigOf(t, "(int 0 1000)"), 1},
	} {
		rep, _ := Intervals(tg, c.sig, body, 0)
		if rep.Ops != 1 {
			t.Fatalf("%s: %d operations counted, want 1", c.what, rep.Ops)
		}
		if rep.Proven != c.proven {
			t.Errorf("%s: %d of %d bounded, want %d of 1 (MaxOp %s)",
				c.what, rep.Proven, rep.Ops, c.proven, rep.MaxOpRange())
		}
	}
}

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

// THE THEOREM, STATED AS A TEST: a range and the `where` it means are the same
// declaration. `(n (int LO HI))` and `(n int) (where (and (<= LO n) (<= n HI)))`
// have the same denotation — γ(int LO HI) = {k | LO ≤ k ≤ HI} is exactly the
// satisfying set of that conjunct — so every analysis must reach the same
// answer, not merely a good enough one.
//
// Checked on a program where the declaration does work in three separate
// places: the multiply is bounded only if the range is believed, the loop
// terminates only if its bound is finite, and the buffer's element range comes
// out of the stores.
func TestScalarRangeAndWhereAgree(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (loop ((i 0) (s 0)) (go.>= i n) s "+
		"else (again (go.+ i 1) (go.+ s (go.* i 3)))))")

	ranged := sigOf(t, "(int 0 1000)")
	whered := sigOf(t, "int")
	whered.Where = mustRead(t, "(and (<= 0 n) (<= n 1000))")

	a, _ := Intervals(tg, ranged, body, 0)
	b, _ := Intervals(tg, whered, body, 0)

	if a.Ops == 0 || a.Loops == 0 {
		t.Fatal("nothing was counted, so this test proves nothing")
	}
	if a.Proven != b.Proven || a.Ops != b.Ops {
		t.Errorf("range says %d of %d bounded, `where` says %d of %d — a range "+
			"and the `where` it means must be the same declaration",
			a.Proven, a.Ops, b.Proven, b.Ops)
	}
	if a.Terminates != b.Terminates {
		t.Errorf("range proves %d of %d loops, `where` proves %d of %d",
			a.Terminates, a.Loops, b.Terminates, b.Loops)
	}
	if a.MaxOpRange() != b.MaxOpRange() {
		t.Errorf("MaxOp differs: range %s, `where` %s", a.MaxOpRange(), b.MaxOpRange())
	}
	// The control: without either, the same program must do WORSE. A test whose
	// two sides agree because neither learned anything proves nothing.
	c, _ := Intervals(tg, sigOf(t, "int"), body, 0)
	if c.Proven == a.Proven && c.Terminates == a.Terminates {
		t.Errorf("the undeclared program is as provable as the declared one "+
			"(%d of %d, %d of %d loops), so this test is vacuous",
			c.Proven, c.Ops, c.Terminates, c.Loops)
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
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (go.* n n))")

	ranged := sigOf(t, "(int 0 1000)")
	whered := sigOf(t, "int")
	whered.Where = mustRead(t, "(and (<= 0 n) (<= n 1000))")

	a, err := Func(tg, "sq", ranged, body)
	if err != nil {
		t.Fatalf("a declared range failed to emit: %v", err)
	}
	b, err := Func(tg, "sq", whered, body)
	if err != nil {
		t.Fatalf("the `where` spelling failed to emit: %v", err)
	}
	if a != b {
		t.Errorf("a range and the `where` it means emit different functions:\n"+
			"range:\n%s\nwhere:\n%s", a, b)
	}
	// And the control, so the test cannot pass by both being degenerate: the
	// parameter really is the host's own word, not a two-byte one.
	if !strings.Contains(a, "n int") {
		t.Errorf("parameter is not `int`:\n%s\nA scalar's range is what the "+
			"value IS; only a table's element slot consults a width", a)
	}
}

// A RANGE IN THE RESULT POSITION IS THE DUAL: a GUARANTEE, not a premise.
//
// postconditions.md's algebra is a swap, and this is that swap written in the
// type language: `result : (int LO HI)` is `(and (<= LO result) (<= result HI))`,
// desugared into `ensures` exactly as a parameter's range desugars into `where`.
//
// Before this, a range in the result position was a declaration NOBODY CHECKED.
// `(sig sq ((n (int 0 100))) (int 0 5))` is false — the body reaches 10000 — and
// was accepted in silence, while the identical claim spelled as an `ensures` was
// refused with the interval that disproves it. Two spellings of one claim, one
// enforced and one decorative.
//
// The false case is what makes this test discriminating: a test that only
// checked the TRUE claim would pass against a compiler that ignored the range.
func TestARangedResultIsChecked(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (go.* n n))")

	for _, c := range []struct {
		result string
		refuse bool
	}{
		{"(int 0 10000)", false}, // true: n ≤ 100, so n*n ≤ 10000
		{"(int 0 5)", true},      // false: the body reaches 10000
	} {
		sig := resultSigOf(t, "(int 0 100)", c.result)
		if sig.Ensures == nil {
			t.Fatalf("%s: a ranged result produced no postcondition, so it is "+
				"a declaration nothing checks", c.result)
		}
		ok, note := CheckEnsures(tg, sig, body)
		if c.refuse && ok {
			t.Errorf("%s: a FALSE ranged result was accepted (%q); the same "+
				"claim as an `ensures` is refused, and two spellings of one "+
				"claim must not disagree", c.result, note)
		}
		if !c.refuse && !ok {
			t.Errorf("%s: a true ranged result was refused: %s", c.result, note)
		}
		// And it must be DECIDED, not merely un-refused. `CheckEnsures` returns
		// SUCCESS with a note when a claim is outside its fragment, so a
		// conjunction synthesised as `(and …)` rather than the erased
		// `(if a b false)` — which is what the connectives desugar to, and what
		// nothing downstream has ever seen anything else of — passes both rows
		// above while checking nothing. That was the first version of this.
		if strings.Contains(note, "outside the decidable fragment") {
			t.Errorf("%s: %s — a ranged result is a constant bound on the "+
				"result, which is exactly what an interval decides; landing "+
				"outside the fragment means it was built in the wrong form",
				c.result, note)
		}
	}
}

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
