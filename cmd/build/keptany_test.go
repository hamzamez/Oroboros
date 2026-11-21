package main

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// A KEPT PARAMETER OF NO SET IS INLINED (ADR 0113's P4 net). The book's chapter
// 14 sort keeps `sink(b ∈ Array(Int) linear, start)` once a table's element is
// a word (ADR 0147), and `start`, unannotated, is used only as a loop's
// initializer, which emit.ParamTypes does not see through: lowered, it stayed
// `any`, and the LLVM printer held it as a pointer where the loop wants an
// i64. Such a definition is inlined. THE PLANTED FAULT keeps it, and the
// printer's refusal comes back.
func TestAKeptParameterOfNoSetIsInlined(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a windows program twice")
	}
	src := "../../docs/book/code/state/sort.oro"
	if err := run("../../targets", src, "windows", t.TempDir()+"/p.exe", "../../lib", false, false, ""); err != nil {
		t.Fatalf("sort on windows: %v", err)
	}
	ir.PlantKeepAnyParam = true
	defer func() { ir.PlantKeepAnyParam = false }()
	if err := run("../../targets", src, "windows", t.TempDir()+"/p.exe", "../../lib", false, false, ""); err == nil ||
		!strings.Contains(err.Error(), "ptr where i64") {
		t.Errorf("with the parameter kept as any: %v; want the printer's refusal", err)
	}
}
