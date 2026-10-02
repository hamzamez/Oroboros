package emit_test

import (
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/golang"
)

// THE OBLIGATIONS OF A PROGRAM ARE A FUNCTION OF THE PROGRAM AND THE TARGET
// (hazard-2026-10-02). None of these tests performs a driver's ritual: each
// makes the target's environment, reduces, and goes on. Until this step that
// gave a residual with no marks at all, and every program below compiled.

// hazardProgram is one obligation of each kind: a host function's parameter
// range, a definition's, a definition's `where`, with an argument the program
// bounds, one it does not, a literal inside and a literal outside.
func hazardProgram(body string) string {
	return `(use go) (use go/math/bits as bits)
(sig digit ((x (int 0 9))) int)
(def digit (x) (+ x 48))
(sig half ((x int)) int (where (>= x 0)))
(def half (x) (/ x 2))
(export f)
(sig f ((n int) (k (int 0 5))) int)
(def f (n k) ` + body + `)`
}

// THE RESIDUAL CARRIES THEM. With nothing but Target.Env and reduction: an
// argument that is not a literal is marked; a literal inside its range is
// discharged by evaluation and leaves nothing; a literal outside it, and a
// `where` that reduced to false, stay as marks.
func TestAResidualCarriesItsObligations(t *testing.T) {
	for _, c := range []struct{ body, want, why string }{
		{"(bits.Len32 n)", `(#req "go/math/bits.Len32" "x" "int 0 4294967295" n)`, "a host parameter's range"},
		{"(digit n)", `(#req "digit" "x" "int 0 9" n)`, "a definition's range"},
		{"(half n)", `(#reqw "half" (>= n 0)`, "a definition's where"},
		{"(digit 12)", `(#req "digit" "x" "int 0 9" 12)`, "a literal outside its range"},
		{"(half -4)", `(#reqw "half" (#false (>= -4 0))`, "a where that is false at the call"},
	} {
		_, nf, _ := normGo(t, hazardProgram(c.body))
		if got := nf.String(); !strings.Contains(got, c.want) {
			t.Errorf("%s: the residual of %s carries no %s\n  %s", c.why, c.body, c.want, got)
		}
	}
	for _, body := range []string{"(digit 7)", "(half 4)", "(bits.Len32 9)"} {
		_, nf, _ := normGo(t, hazardProgram(body))
		if emit.HasRequireMarks(nf) {
			t.Errorf("%s: a literal that satisfies its contract left a mark: %s", body, nf)
		}
	}
}

// NOTHING READS A MARKED RESIDUAL AS IF IT WERE DECIDED. The refinement layer
// and the printers are transparent to a mark, so each refuses one: Refine at
// its entrance, and ir.ToP, which all four printers come through.
func TestAnUndecidedObligationIsNotCompiled(t *testing.T) {
	tg, nf, sig := normGo(t, hazardProgram("(digit n)"))
	if _, err := emit.Refine(tg, "f", sig, nf); err == nil || !strings.Contains(err.Error(), "nothing has decided") {
		t.Errorf("the refinement layer read a marked residual: %v", err)
	}
	f, err := ir.Lower(tg, "f", sig, nf, ir.Options{Decided: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := golang.FromFunc(tg, f); err == nil || !strings.Contains(err.Error(), "reached the printer undecided") {
		t.Errorf("a function lowered with its marks was printed: %v", err)
	}
	// A where-mark leaves no value behind in the IR, and is counted too, where
	// it is the function's tail and where it is a bound value: lowering meets
	// the two in different places. (Inside an operator's argument reduction
	// hoists it to the tail, so `(+ (half n) 1)` would test the first twice.)
	for _, body := range []string{"(half n)", "(let x (half n) (+ x x))"} {
		tg, nf, sig = normGo(t, hazardProgram(body))
		if f, err = ir.Lower(tg, "f", sig, nf, ir.Options{Decided: true}); err != nil {
			t.Fatal(err)
		}
		if _, err := golang.FromFunc(tg, f); err == nil || !strings.Contains(err.Error(), "reached the printer undecided") {
			t.Errorf("%s: a function lowered with a where-mark was printed: %v", body, err)
		}
	}
}

// EVERY WAY FROM A RESIDUAL TO EMITTED TEXT DECIDES THEM. The pipeline
// (ir.Entry, through entryGo) and a backend's FromResidual each decide the
// marks they are given, by the routes the drivers use, with no set and no
// callback: what the program bounds is proven, and the rest is refused with
// the obligation's own message.
func TestEveryPipelineDecidesItsObligations(t *testing.T) {
	for _, c := range []struct{ body, refused string }{
		{"(bits.Len32 k)", ""},
		{"(digit k)", ""},
		{"(if (>= n 0) (half n) 0)", ""},
		{"(digit 7)", ""},
		{"(bits.Len32 n)", "go/math/bits.Len32's parameter x, declared (int 0 4294967295), requires"},
		{"(bits.Len32 -1)", "go/math/bits.Len32's parameter x is declared (int 0 4294967295), and a call passes -1"},
		{"(digit n)", "digit's parameter x, declared (int 0 9), requires"},
		{"(digit 12)", "digit's parameter x is declared (int 0 9), and a call passes 12"},
		{"(half n)", "half requires (>= n 0) at a call"},
		{"(half -4)", "half's `where` is false at a call: (>= -4 0)"},
	} {
		src := hazardProgram(c.body)
		_, viaEntry := entryGo(t, src)
		tg, nf, sig := normGo(t, src)
		_, viaResidual := golang.FromResidual(tg, "f", sig, nf)
		for how, err := range map[string]error{"ir.Entry": viaEntry, "FromResidual": viaResidual} {
			switch {
			case c.refused == "" && err != nil:
				t.Errorf("%s through %s: %v", c.body, how, err)
			case c.refused != "" && (err == nil || !strings.Contains(err.Error(), c.refused)):
				t.Errorf("%s through %s: want a refusal naming %q, got %v", c.body, how, c.refused, err)
			}
		}
	}
}
