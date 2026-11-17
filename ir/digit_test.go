package ir

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
)

// numeralTargets are the targets whose integers above the word are K(ℕ*):
// those of ../targets that load, have no bignum and hold ℤ by the limb rung
// (Target.HoldsZ). Derived, so a new member is checked when it is added.
func numeralTargets(t *testing.T) []*emit.Target {
	t.Helper()
	ents, err := os.ReadDir("../targets")
	if err != nil {
		t.Fatal(err)
	}
	var out []*emit.Target
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		tg, err := emit.LoadTargetLayers(e.Name(), []string{"../targets"}, []string{"../lib"})
		if err != nil {
			continue // a layer (llvm, win32) is no target alone
		}
		if !tg.HasBig() && tg.HoldsZ() {
			out = append(out, tg)
		}
	}
	return out
}

// undecidedInstances is every called K(ℕ*) instance (natCalled) that is not
// decided under its parameters' sets on tg, with its reason.
func undecidedInstances(tg *emit.Target) []string {
	var bad []string
	for fn := range natCalled {
		if _, err := buildNatCallee(tg, fn, "nat."+fn); err != nil {
			s := err.Error()
			if i := strings.Index(s, "\n"); i > 0 {
				s = s[:i]
			}
			bad = append(bad, tg.Name+": "+s)
		}
	}
	return bad
}

// THE LIBRARY IS A THEORY CHECKED ONCE, ON EVERY TARGET IT SERVES (ADR 0035,
// 0101, noend-2026-11-17). Every instance a program calls is decided under its
// parameters' sets on every target whose integers above the word are K(ℕ*).
// P4 inlines an instance that is not (ADR 0102), which keeps every program's
// verdict, but hides the library's defect and makes each program pay for it:
// windows32's `div`, undecided before ADR 0124's lemma, made two programs
// compile for over 20 minutes. So the theory is checked here, once, and not by
// the program that happens to call it. THE PLANTED FAULT reads the two-digit
// lemma on ℤ's operations alone: windows32's `div` is then undecided, and the
// check reports it.
func TestTheLibraryIsDecidedOnEveryTargetItServes(t *testing.T) {
	tgs := numeralTargets(t)
	var names []string
	for _, tg := range tgs {
		names = append(names, tg.Name)
	}
	if !strings.Contains(" "+strings.Join(names, " ")+" ", " windows32 ") || len(tgs) < 2 {
		t.Fatalf("K(ℕ*)'s targets should include windows and windows32, got %v", names)
	}
	for _, tg := range tgs {
		for _, b := range undecidedInstances(tg) {
			t.Errorf("an instance a program calls is not decided: %s", b)
		}
	}
	PlantQuotientWordOnly = true
	defer func() { PlantQuotientWordOnly = false }()
	for _, tg := range tgs {
		if tg.Name != "windows32" {
			continue
		}
		_, err := buildNatCallee(tg, "div", "nat.div")
		if err == nil || !strings.Contains(err.Error(), "K(ℕ*)'s div is not decided") {
			t.Errorf("with the lemma on ℤ's operations alone windows32's div should be undecided, so the check reaches the rule; got %v", err)
		}
	}
}
