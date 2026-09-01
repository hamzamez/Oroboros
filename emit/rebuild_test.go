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

// NO TWO ENCLOSING BINDERS MAY SHARE A NAME IN THE REBUILT TERM, which is the
// invariant that makes `core.Fn` safe to call on an opened body.
//
// `openFresh` renames only against the set it is given, and the interval pass
// passed a FRESH EMPTY MAP at every call site. So a loop over `i` containing a
// loop over `i` — what an inlined helper produces constantly — gave both
// binders the same fresh name, and the inner `core.Fn` then bound occurrences
// that belonged to the outer one.
//
// INVISIBLE BY DEFAULT, because the rebuilt term is discarded unless `-checked`
// is on. Under `-checked` the windows hash table read its probe counter where
// the key should have been, so every insert after the first hashed to slot 0,
// found it taken and reported the table full: a five-entry map answered `len`
// of 1 while the unchecked build answered 5. Same class as the `FnClosed` bug
// this file was written for, and hidden the same way.
//
// Asserted STRUCTURALLY rather than by printing, because renaming the inner
// binder is exactly what the fix does — the output is alpha-equivalent to the
// input and deliberately not identical to it.
func TestNoTwoEnclosingBindersShareAName(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// THE SHAPE ONLY EXISTS AFTER INLINING, so this goes through the real
	// pipeline. A helper whose own loop variable is also `i` is called with the
	// caller's `i`; reduction substitutes it, and the reference then reaches
	// PAST a binder spelled the same as the one it points at. That cannot be
	// written directly — in surface syntax the inner `i` shadows — which is why
	// the bug needed an inlined helper to appear, and `wm-put` inside a
	// caller's insert loop is exactly that.
	src := `
		(use go)
		(export run)
		(def probe (fn (k) (loop ((i 0) (s 0)) (go.>= i 2) s
		  else (again (go.+ i 1) (go.+ s k)))))
		(def run (fn (n) (loop ((i 0) (acc 0)) (go.>= i 4) acc
		  else (again (go.+ i 1) (go.+ acc (probe i))))))
		(sig run ((n int)) int)
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
	nf, err := core.Normalize(prog.Defs["run"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	_, out := Intervals(tg, prog.Sigs["run"], nf, 0)

	var walk func(*core.Term, map[string]bool)
	walk = func(n *core.Term, live map[string]bool) {
		if n == nil {
			return
		}
		if n.Kind == core.KFn {
			for _, p := range n.Params {
				if live[p] {
					t.Errorf("binder %q is nested inside another binder of the "+
						"same name in the rebuilt term; the inner `core.Fn` "+
						"captures the outer's occurrences:\n  %s", p, out)
				}
			}
			inner := map[string]bool{}
			for k := range live {
				inner[k] = true
			}
			for _, p := range n.Params {
				inner[p] = true
			}
			walk(n.Body(), inner)
			return
		}
		for _, k := range n.Kids {
			walk(k, live)
		}
	}
	walk(out, map[string]bool{})
}

// AND SIBLING BINDERS MUST NOT BE RENAMED EITHER, which is the other half: the
// set has to hold exactly the ENCLOSING binders, so a name is released when the
// pass leaves its scope. Holding them forever renames the second `(loop ((i 0))
// …)` in a function to `i2` though no `i` is in scope — and across the pass's
// several sweeps it renames the same binder again on every one, so the rebuilt
// term stops being the input even when nothing was selected.
func TestSiblingBindersKeepTheirNames(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := "(fn (n) (go.+ (loop ((i 0)) (go.>= i 3) i else (again (go.+ i 1))) " +
		"(loop ((i 0)) (go.>= i 5) i else (again (go.+ i 1)))))"
	terms, err := core.ReadAll(src)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	in := terms[0]
	_, out := Intervals(tg, nil, in, 0)
	if got, want := out.String(), in.String(); got != want {
		t.Errorf("sibling binders were renamed.\n  in:  %s\n  out: %s", want, got)
	}
}
