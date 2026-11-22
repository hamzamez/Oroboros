package main

import (
	"testing"

	"oroboros/ir"
)

// A POINT IS ITS CONSTANT AT AN EXPORT (the book's finding 120, ADR 0156):
// each export compiled as its own unit, on every target, its yield into ℤ the
// closed term held exactly; PlantRecombinePoints recombines the point from its
// digits again, and Go refuses the export (cmd/build's TestAPointIsItsConstant
// runs the values).
func TestAPointIsItsConstantAtAnExport(t *testing.T) {
	const src = "export fn lit() (Int) { 18446744073709551616 }\n" +
		"export fn too_big() (Int) { 2^62 * 4 }\n" +
		"export fn neg() (Int) { 0 - 2^62 * 4 }\n"
	for _, target := range []string{"go", "js", "java", "windows"} {
		if _, err := surfaceGen(t, target, map[string]string{"p.oro": src}); err != nil {
			t.Errorf("%s: exports returning a constant past the word should compile: %v", target, err)
		}
	}
	ir.PlantRecombinePoints = true
	_, err := surfaceGen(t, "go", map[string]string{"p.oro": src})
	ir.PlantRecombinePoints = false
	if err == nil {
		t.Error("with a point recombined Go should refuse the export: the witness cannot fail")
	}
}
