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
	// A TABLE'S ELEMENT IS A SINK there too: JavaScript's table of integers is
	// Numbers, and `[9223372036854775807]` printed 2⁶³ at HEAD; refused now
	table := "use io\n\nexport fn main() {\n    t = array { 9223372036854775807, 9223372036854775806 }\n" +
		"    loop i = 0 {\n        i < 2 -> { io.print_int(t[i]); again(i + 1) }\n        else -> ()\n    }\n}\n"
	if got, err := runOn(t, table, "js"); err == nil {
		t.Errorf("a literal table past JavaScript's word should be refused, not printed %q", got)
	}
	ir.PlantConstantInTheWord = true
	got, err := runOn(t, src, "js")
	ir.PlantConstantInTheWord = false
	if err == nil && got == want {
		t.Error("with the constant a word, JavaScript still printed it exactly: the witness cannot fail")
	}
}

// A STATIC NUMERAL IS NEVER RELEASED (ADR 0157). On windows a constant held
// exactly is its numeral in read-only data, a table no birth makes; a program
// function's result is a birth its caller owns and releases (the frame rule,
// ADR 0101), so a numeral leaving its function by its result is fresh storage,
// its spine, and is static only where it is borrowed. A kept function returning
// a constant, called in a loop: PlantStaticNumeralEscapes returns the read-only
// numeral, which its caller then releases.
func TestAStaticNumeralIsNeverReleased(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a windows program")
	}
	src := "use win/print\n\n" +
		"fn pick(k ∈ 0..1) (Int) { if k == 0 { 18446744073709551616 } else { 0 - 340282366920938463463374607431768211456 } }\n\n" +
		"export fn main() {\n    loop i = 0 {\n        i < 6 -> { print.print_str(decimal(pick(i % 2) + i)); again(i + 1) }\n        else -> ()\n    }\n}\n"
	const want = "18446744073709551616 -340282366920938463463374607431768211455 18446744073709551618 " +
		"-340282366920938463463374607431768211453 18446744073709551620 -340282366920938463463374607431768211451"
	old := ir.KeepThreshold
	ir.KeepThreshold = 1
	defer func() { ir.KeepThreshold = old }()
	if got, err := runOnMayFail(t, src, "windows"); err != nil || got != want {
		t.Errorf("a kept function returning constants should print %s, got %q, %v", want, got, err)
	}
	ir.PlantStaticNumeralEscapes = true
	got, err := runOnMayFail(t, src, "windows")
	ir.PlantStaticNumeralEscapes = false
	if err == nil && got == want {
		t.Error("with the read-only numeral returned and released, the program still printed right: the witness cannot fail")
	}
}
