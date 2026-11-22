package main

import (
	"testing"

	"oroboros/ir"
)

// A POINT IS ITS CONSTANT (the book's finding 120, ADR 0156). An export
// returning a constant past the word as `Int`, `18446744073709551616` or the
// product `2^62 * 4`, was refused on Go, the JVM and windows: its numeral's sum
// lies in the double word, so the yield into ℤ was recombined from its digits
// (ADR 0153) as if E computed it, and the rung then held the sum in ℤ, leaving
// the recombination's word operations over it unbounded. A value whose fact is
// one integer is computed by nothing; the rung holds the closed term as it is.
// The planted fault is cmd/gen's TestAPointIsItsConstantAtAnExport: cmd/build
// compiles main, where the constant is inlined and meets no export's yield.
func TestAPointIsItsConstant(t *testing.T) {
	if testing.Short() {
		t.Skip("builds programs on four targets")
	}
	src := "use io\n\n" +
		"export fn lit() (Int) { 18446744073709551616 }\n" +
		"export fn too_big() (Int) { 2^62 * 4 }\n" +
		"export fn neg() (Int) { 0 - 2^62 * 4 }\n\n" +
		"export fn main() {\n    io.print_int(lit())\n    io.print_int(too_big())\n    io.print_int(neg())\n}\n"
	const want = "18446744073709551616 18446744073709551616 -18446744073709551616"
	for _, target := range []string{"go", "windows"} {
		if got, err := runOnMayFail(t, src, target); err != nil || got != want {
			t.Errorf("%s: constants past the word returned as Int should print %s, got %q, %v", target, want, got, err)
		}
	}
	for _, target := range []string{"js", "java"} {
		if got, err := runOn(t, src, target); err != nil || got != want {
			t.Errorf("%s: constants past the word returned as Int should print %s, got %q, %v", target, want, got, err)
		}
	}
}

// A CONSTANT OUTSIDE S IS HELD EXACTLY (integers.md §1a, ADR 0156 item 5). On
// JavaScript, whose word is ±(2⁵³ − 1), the literal 2⁶² printed
// 4611686018427388000: the constant was a word, the host's Number, at
// `decimal`. It is supply, held exactly where a call or the result reads it,
// and the pair's two digits where only the pair's operations do.
// PlantConstantInTheWord leaves it a word again: the low digits are lost.
func TestAConstantOutsideTheWordIsHeldExactly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a JavaScript program")
	}
	src := "use io\n\nexport fn main() {\n    io.print_int(4611686018427387904)\n    io.print_int(9223372036854775807 / 3)\n}\n"
	const want = "4611686018427387904 3074457345618258602"
	if got, err := runOn(t, src, "js"); err != nil || got != want {
		t.Errorf("constants past JavaScript's word should print %s, got %q, %v", want, got, err)
	}
	ir.PlantConstantInTheWord = true
	got, err := runOn(t, src, "js")
	ir.PlantConstantInTheWord = false
	if err == nil && got == want {
		t.Error("with the constant a word, JavaScript still printed it exactly: the witness cannot fail")
	}
}
