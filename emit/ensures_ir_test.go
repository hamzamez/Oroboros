package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// A POSTCONDITION IS AN OBLIGATION, decided by the IR (ir.CheckEnsures,
// postconditions.md §2): the body's result fact must lie in the set the
// postcondition denotes (emit.EnsuresBounds). These tests moved here from
// scalarrange_test.go and bigrep_test.go when the check left the term analysis
// (irstep4e), keeping what each one pinned.

// ensuresOn lowers a residual, decides it, and checks its postcondition.
func ensuresOn(t *testing.T, tg *emit.Target, sig *core.Sig, body *core.Term) error {
	t.Helper()
	f, err := ir.Lower(tg, "f", sig, body, ir.Options{Decided: true})
	if err != nil {
		t.Fatal(err)
	}
	ir.Decide(tg, f, true)
	return ir.CheckEnsures(tg, f, sig)
}

func sigRead(t *testing.T, src string) *core.Sig {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil || len(forms) != 1 || forms[0].Sig == nil {
		t.Fatalf("read %s: %v", src, err)
	}
	return forms[0].Sig
}

// A RANGE IN THE RESULT POSITION IS A GUARANTEE: `result : (int LO HI)` is
// `(and (<= LO result) (<= result HI))`, and the false row is what makes the
// test discriminating. It must also be DECIDED, not merely unrefused: the
// synthesised conjunction has to land inside the fragment.
func TestARangedResultIsChecked(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, "(fn (n) (go.* n n))")
	for _, c := range []struct {
		result string
		refuse bool
	}{
		{"(int 0 10000)", false}, // true: n ≤ 100, so n·n ≤ 10000
		{"(int 0 5)", true},      // false: the body reaches 10000
	} {
		sig := sigRead(t, "(sig sq ((n (int 0 100))) "+c.result+")")
		if sig.Ensures == nil {
			t.Fatalf("%s: a ranged result produced no postcondition", c.result)
		}
		if _, _, decided := emit.EnsuresBounds(tg, sig.Ensures); !decided {
			t.Errorf("%s: a ranged result is a constant bound, which the fragment decides", c.result)
		}
		err := ensuresOn(t, tg, sig, body)
		if c.refuse && err == nil {
			t.Errorf("%s: a FALSE ranged result was accepted", c.result)
		}
		if !c.refuse && err != nil {
			t.Errorf("%s: a true ranged result was refused: %v", c.result, err)
		}
	}
}

// A RELATIONAL POSTCONDITION IS REFUSED, not noted: it is outside what an
// interval settles, and an obligation is discharged or the program is refused
// (refinements.md §3a). The term analysis passed it with "propagated and not
// proven".
func TestAnUndecidedPostconditionIsRefused(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	sig := sigRead(t, "(sig sq ((n (int 0 100))) int (ensures (>= result n)))")
	err = ensuresOn(t, tg, sig, mustRead(t, "(fn (n) (go.* n n))"))
	if err == nil || !strings.Contains(err.Error(), "refinements.md §3a") {
		t.Errorf("a relational postcondition was not refused: %v", err)
	}
}

// A BOUND PAST THE ENDS' RANGE is read exactly: `result <= 2^200` is met by
// every finite result, and not by an unbounded one.
func TestABoundPastTheEndsIsExact(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	big := "1606938044258990275541962092341162602522202993782792835301376" // 2^200
	bounded := sigRead(t, "(sig sq ((n (int 0 100))) int (ensures (<= result "+big+")))")
	if bounded.Ensures == nil {
		t.Skip("the reader does not keep a literal past int64 in an ensures")
	}
	if err := ensuresOn(t, tg, bounded, mustRead(t, "(fn (n) (go.* n n))")); err != nil {
		t.Errorf("a finite result against 2^200 was refused: %v", err)
	}
	unbounded := sigRead(t, "(sig sq ((n int)) (int 0 +inf) (ensures (<= result "+big+")))")
	if err := ensuresOn(t, tg, unbounded, mustRead(t, "(fn (n) (go.* n n))")); err == nil {
		t.Errorf("an unbounded result was accepted against 2^200")
	}
}

// A BIG RESULT'S RANGE IS NO GUARANTEE (ADR 0026): `(int 0 +inf)` synthesises
// nothing, and the program is accepted.
func TestABigResultRangeIsNotAnObligation(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(export fib)
(sig fib ((n (int 0 1000))) (int 0 +inf))
(def fib (fn (n) (loop ((a 0) (b 1) (i 0)) (>= i n) a else (again b (+ a b) (+ i 1)))))
`
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["fib"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	// The drivers' pipeline, which ends in the postcondition's check.
	if _, _, err := ir.Entry(tg, "fib", "fib", prog.Sigs["fib"], nf, nil, false, nil); err != nil {
		t.Errorf("a big result was refused: %v", err)
	}
}
