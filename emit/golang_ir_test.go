package emit_test

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
)

// These are the Go backend's tests, now of the IR's Go printer
// (irstep4a-2026-09-26). Four were retired with the term backend, each for a
// stated reason:
//   - TestEmitDot and the portable layer's other `fold-range` programs: the
//     portable layer is retired, no live target declares `fold-range`, and
//     native dot is the gauntlet's;
//   - TestNestedBindersDoNotShadow: an IR value is numbered, so two binders
//     cannot share a name by construction;
//   - TestStatementValueIsNotRecomputed: a statement call's value is an alias
//     of its first argument (L9), so it is computed once by construction;
//   - TestBoundStatementValueKeepsItsType: a portable-layer int64 question; on
//     the live target `int` is Go's `int`.

func goLive(t *testing.T) *emit.Target {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	return tg
}

// An escaping closure is refused, and the refusal says why rather than
// producing Go that does not compile (callbacks.md tier 3).
func TestEscapingClosureIsARefusal(t *testing.T) {
	src := `
		(use go)
		(def make-scaler (fn (f) (fn (v) (go.* v f))))
		(fn (k) (make-scaler k))
	`
	_, err := golang.FromResidual(goLive(t), "ms", nil, reduce(t, src, "go"))
	if err == nil {
		t.Fatal("expected a refusal for an escaping closure")
	}
	if !strings.Contains(err.Error(), "closure") {
		t.Errorf("error should name the problem; got %v", err)
	}
}

// A signature types the parameters. Without one, a parameter nothing reaches
// has no final type on a typed target, and it is refused in words; with one,
// the signature's types are the Go function's.
func TestSignatureTypesTheParameters(t *testing.T) {
	tg := goLive(t)
	nf := reduce(t, `
		(use go)
		(fn (n) (loop ((acc 0.0) (i 0))
		  (go.>= i n)  acc
		  else         (again (go.f+ acc 1.0) (go.+ i 1))))
	`, "go")
	// A parameter that reaches no typed operation is untypeable without a
	// signature. (`n` above reaches `go.>=`, whose declaration types it: the
	// IR's typing is unification, and needs no signature there.)
	if _, err := golang.FromResidual(tg, "id", nil, reduce(t, "(use go)\n(fn (x) x)", "go")); err == nil {
		t.Fatal("without a signature the parameter is untypeable, so this must fail")
	} else if !strings.Contains(err.Error(), "parameter") {
		t.Errorf("the refusal must name the parameter: %v", err)
	}
	sig := &core.Sig{Params: []core.SigParam{{Name: "n", Type: "int"}}, Result: "f64"}
	code, err := golang.FromResidual(tg, "total", sig, nf)
	if err != nil {
		t.Fatalf("with a signature it must emit: %v", err)
	}
	if !regexp.MustCompile(`func Total\(v\d+ int\) float64`).MatchString(code) {
		t.Errorf("signature types not used:\n%s", code)
	}
}

// docs/spec/iteration.md. `loop` with `again`: n variables, early exit, and
// unbounded iteration, emitted as the host's own `for`, with the uniformly
// updated variable in the post clause (PostVars) and the early exit assigning
// the loop's result.
func TestLoopEmitsHostFor(t *testing.T) {
	nf := reduce(t, `
		(use go)
		(fn (a k)
		  (loop ((i 0))
		    (go.>= i (len a))       -1
		    (go.f> (a i) k)         i
		    else                    (again (go.+ i 1))))
	`, "go")
	sig := &core.Sig{Params: []core.SigParam{{Name: "a", Type: "array f64"},
		{Name: "k", Type: "f64"}}, Result: "int"}
	code, err := golang.FromResidual(goLive(t), "find", sig, nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !regexp.MustCompile(`for ; ; (v\d+) = \((v\d+) \+ 1\) \{`).MatchString(code) {
		t.Errorf("the loop is not a host for with the update in its post clause:\n%s", code)
	}
	if !strings.Contains(code, "return -1") && !regexp.MustCompile(`v\d+ = -1`).MatchString(code) {
		t.Errorf("the early exit must produce -1:\n%s", code)
	}
}

// hamza's optimisation: an `again` argument that IS the loop variable needs no
// assignment. `i` is updated identically by every `again`, so it is the post
// clause (1.4x on the sieve); `best` differs per clause and stays in the body;
// the clause that changes nothing is a bare `continue`.
func TestLoopSkipsUnchangedArguments(t *testing.T) {
	nf := reduce(t, `
		(use go)
		(fn (a)
		  (loop ((best 0.0) (i 0))
		    (go.>= i (len a))         best
		    (go.f> (a i) best)        (again (a i) (go.+ i 1))
		    else                      (again best (go.+ i 1))))
	`, "go")
	sig := &core.Sig{Params: []core.SigParam{{Name: "a", Type: "array f64"}}, Result: "f64"}
	code, err := golang.FromResidual(goLive(t), "best", sig, nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	post := regexp.MustCompile(`for ; ; (v\d+) = \((v\d+) \+ 1\) \{`).FindStringSubmatch(code)
	if post == nil || post[1] != post[2] {
		t.Fatalf("the uniform update should be the loop's post clause:\n%s", code)
	}
	i := post[1]
	if n := strings.Count(code, i+" = ("+i+" + 1)"); n != 1 {
		t.Errorf("the update should appear exactly once, in the post clause (%d):\n%s", n, code)
	}
	// Nothing assigns a variable to itself.
	for _, m := range regexp.MustCompile(`(v\d+) = (v\d+)\n`).FindAllStringSubmatch(code, -1) {
		if m[1] == m[2] {
			t.Errorf("an unchanged variable was reassigned: %s", m[0])
		}
	}
}
