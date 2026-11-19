package ir

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// A K(ℕ*) INSTANCE'S RESULT IS PROVEN BY ITS BODY (ADR 0072 on a table): before
// its callers read a returned table as a numeral's digits, every element is
// shown in [0, B) by the instance's own facts (proveResultElems). Every real
// instance passes, so no program shows the rule; its witness is an instance
// built wrong by hand, a table holding −1, which the rule refuses and the
// planted fault, the check skipped, accepts.
func TestAResultsElementsAreProvenByItsBody(t *testing.T) {
	tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	const text = `(ir 1
  (target windows)
  (stage P)
  (ops const array yield)
  (func bad (params) (results (array int))
    (region
      (val (%0 (int -1 -1)) (const -1))
      (val (%1 (array (int -1 -1))) (array %0))
      (yield %1))))
`
	p, err := Read(text)
	if err != nil {
		t.Fatal(err)
	}
	if err := proveResultElems(tg, p.Funcs[0], ei(0), u64Max); err == nil || !strings.Contains(err.Error(), "its result's elements") {
		t.Errorf("a result holding −1: want it refused naming its elements, got %v", err)
	}
	PlantResultUnproven = true
	defer func() { PlantResultUnproven = false }()
	if err := proveResultElems(tg, p.Funcs[0], ei(0), u64Max); err != nil {
		t.Errorf("with the check skipped the result should be accepted, so the case reaches the rule; got %v", err)
	}
}
