package llvm

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// A TARGET'S CPU IS IN ITS MODULE (ADR 0142): windows-v3's functions carry
// "target-cpu"="x86-64-v3", and windows', the baseline, carry none. THE PLANTED
// FAULT prints no CPU, and windows-v3's module then says nothing of the level
// it is built for.
func TestATargetsCPUIsInItsModule(t *testing.T) {
	load := func(name string) *emit.Target {
		t.Helper()
		emit.ResetImports()
		tg, err := emit.LoadTargetLayers(name, []string{"../../targets"}, []string{"../../lib"})
		if err != nil {
			t.Fatal(err)
		}
		return tg
	}
	const attr = `"target-cpu"="x86-64-v3"`
	v3, base := load("windows-v3"), load("windows")
	if v3.CPU != "x86-64-v3" || v3.Triple != base.Triple {
		t.Fatalf("windows-v3: want windows' triple at x86-64-v3, got %q at %q", v3.Triple, v3.CPU)
	}
	if m := File(v3, map[string]string{}, ""); !strings.Contains(m, attr) {
		t.Errorf("windows-v3's module should state its CPU:\n%s", m)
	}
	if m := File(base, map[string]string{}, ""); strings.Contains(m, "target-cpu") {
		t.Errorf("windows is the baseline and states no CPU:\n%s", m)
	}
	PlantCPUUnprinted = true
	defer func() { PlantCPUUnprinted = false }()
	if m := File(v3, map[string]string{}, ""); strings.Contains(m, attr) {
		t.Error("with the CPU unprinted the module still states it: the witness cannot fail")
	}
}
