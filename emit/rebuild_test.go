package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE REBUILT TERM MUST BE THE SAME TERM.
//
// `Intervals` returns a rebuilt term beside its report — the representation
// selection ADR 0003 and selection-2026-08-19 describe, where an operation the
// compiler cannot bound is rewritten to the target's checked form. When nothing
// is selected the rebuild must be the INPUT, exactly.
//
// Nothing else exercises it. The default path throws the rebuilt term away and
// emits from the original, so a bug here is invisible until someone passes
// `-checked` — and one was. `evalR` opened a lambda's body with `Body()`, which
// turns KBound indices into KNames, and rewrapped it with `FnClosed`, which
// takes an ALREADY-CLOSED body and does not close. Every parameter occurrence
// came back a free name and the binder stopped binding, so
// `examples/json/tree.oro` compiled to Go that referred to a `nodes` belonging
// to a different function and `go build` refused the file.
//
// `p.let` has the same shape and gets it right, with a comment saying `core.Fn`
// closes an open body. The generic lambda case did not.
func TestIntervalsRebuildsTheSameTerm(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// A buffer threaded through a loop — the shape that broke — with every
	// operation provable, so selection has nothing to do and the rebuild must
	// be the identity.
	src := "(fn (a) (build 8 (fn (b) (loop ((b b) (i 0)) (go.>= i 8) b " +
		"else (again (set b i i) (go.+ i 1))))))"
	terms, err := core.ReadAll(src)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	in := terms[0]
	rep, out := Intervals(tg, nil, in, 0)

	if rep.Ops == 0 {
		t.Fatal("nothing was counted, so this test proves nothing")
	}
	if rep.Proven != rep.Ops {
		t.Fatalf("%d of %d operations proven; this program is meant to be fully "+
			"provable, so selection should have nothing to do",
			rep.Proven, rep.Ops)
	}
	if rep.Selected != 0 {
		t.Fatalf("%d operations were selected for checking in a fully provable "+
			"program", rep.Selected)
	}
	if got, want := out.String(), in.String(); got != want {
		t.Fatalf("the rebuilt term differs from the input when nothing was "+
			"selected.\n  in:  %s\n  out: %s", want, got)
	}

	// AND THE BINDERS STILL BIND. Printing can hide the failure — an open body
	// prints its parameters by the same names it was opened with — so the
	// structural invariant is checked directly: after `Fn`, a lambda's stored
	// body holds its own parameters as indices, never as names.
	var walk func(*core.Term, []string)
	walk = func(x *core.Term, bound []string) {
		if x == nil {
			return
		}
		if x.Kind == core.KName {
			for _, b := range bound {
				if x.Name == b {
					t.Fatalf("%q occurs as a free NAME inside the lambda that "+
						"binds it; the rebuilt body was never closed", b)
				}
			}
			return
		}
		if x.Kind == core.KFn {
			walk(x.Closed(), append(append([]string{}, bound...), x.Params...))
			return
		}
		for _, k := range x.Kids {
			walk(k, bound)
		}
	}
	walk(out, nil)
}

// AND WHEN SOMETHING IS SELECTED, IT IS ONLY THE UNPROVABLE OPERATION.
//
// The companion to the test above: a rebuild that changed nothing would pass
// that one and be useless. This program's accumulator genuinely cannot be
// bounded — `examples/int/power.oro`'s shape — so the target's checked
// primitive has to appear, and nothing else may.
func TestIntervalsSelectsOnlyTheUnprovable(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := "(fn (x) (loop ((acc 1) (i 0)) (go.>= i 40) acc " +
		"else (again (go.* acc x) (go.+ i 1))))"
	terms, err := core.ReadAll(src)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	rep, out := Intervals(tg, nil, terms[0], 0)
	if rep.Selected == 0 {
		t.Fatal("an unbounded accumulator must select a checked form; if the " +
			"target declares none this test needs a different target")
	}
	s := out.String()
	if !strings.Contains(s, "go.mul-exact") {
		t.Fatalf("selection reported %d but the rebuilt term shows no checked "+
			"primitive:\n%s", rep.Selected, s)
	}
	// The counter is provable, so it must keep the host's own operator.
	if !strings.Contains(s, "(go.+ i 1)") {
		t.Fatalf("the provable counter should keep `go.+`:\n%s", s)
	}
}
