package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A HOST CALL HAS A TEMPLATE. A primitive with none is a structural kind
// lowering does not know, and printing it as a call printed `()`: the cell
// forms did, before their lowering existed (outcells-2026-10-09). So lowering
// refuses it, on every backend at once.
func TestACallWithNoTemplateIsRefused(t *testing.T) {
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	tg.Prims["notemplate"] = emit.Prim{Name: "notemplate", Kind: "expr", Args: []string{"int"}, Result: "int"}
	term := core.Fn([]string{"x"}, core.App(core.Name("notemplate"), core.Name("x")))
	sig := &core.Sig{Params: []core.SigParam{{Name: "x", Type: "int"}}, Result: "int"}
	if _, err := Lower(tg, "f", sig, term, Options{}); err == nil || !strings.Contains(err.Error(), "has no template") {
		t.Errorf("a call with no template was lowered: %v", err)
	}
}
