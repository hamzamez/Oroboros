package emit

// Internal names the external tests (package emit_test, the *_ir_test.go
// files, which print through the IR and so cannot be internal) read.
var JSImportAlias = jsImportAlias

// PlantRangeMarkFault restores the refusal of an in-word range mark the
// interval route left (requires.go).
func PlantRangeMarkFault(on bool) { plantRangeMarkFault = on }

// PlantNegatedEqFault reads ¬(a = b) as an opaque atom again (linear.go).
func PlantNegatedEqFault(on bool) { plantNegatedEqFault = on }
