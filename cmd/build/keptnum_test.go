package main

import (
	"fmt"
	"strings"
	"testing"

	"oroboros/ir"
)

// A KEPT DEFINITION'S RESULT ABOVE THE WORD IS AN INTEGER AT ITS CALLERS
// (keptnum-2026-11-22). A representation is an abstraction function (H, d)
// (ADR 0083), and a kept definition (ADR 0113) is read through its interface:
//
//  1. on K(ℕ*), a numeral has no width (ADR 0099), so its d is the caller's
//     too: `calc`, kept, returns a numeral its caller renders by the limb
//     library, on windows, where nothing else holds 3^100. Registered by its
//     carrier (PlantKeptNumeralAsTable), the call's value was a table, and
//     `decimal` of it took the word's rendering: W5 refused the IR_P;
//  2. on fixed limbs, d is at the one set's width (ADR 0029), which a caller
//     lowered under its own set does not share: such a definition is not
//     kept, and the program, inlined, answers. Kept (PlantKeepFixedLimbs),
//     its caller read a table where an integer was meant, and it stopped;
//  3. a definition on fixed limbs calling a kept numeral leaves them for
//     K(ℕ*) (TestAKeptNumeralIsReadByItsAbstractionFunction).
func TestAKeptNumeralIsAnIntegerAtItsCallers(t *testing.T) {
	const numeral = "use win/print\n\n" +
		"fn calc(n ∈ 0..1) (0..2^300) {\n" +
		"    a = 3^100\n" +
		"    case {\n" +
		"        n == 0 -> a & (2^200 - 1)\n" +
		"        else -> a >> 7\n" +
		"    }\n" +
		"}\n\n" +
		"export fn main() {\n" +
		"    loop i = 0 {\n" +
		"        i <= 1 -> { print.print_str(decimal(calc(i))); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	// Python: a = 3**100; a & (2**200 - 1), a >> 7
	const want = "515377520732011331036461129765621272702107522001 4026386880718838523722352576293916192985215015"
	// a body this small is inlined by P2's threshold; lowered, the witness's
	// definition is kept, as a larger one is by the threshold itself (big-bits)
	defer func(k int) { ir.KeepThreshold = k }(ir.KeepThreshold)
	ir.KeepThreshold = 1
	got, err := runOnMayFail(t, numeral, "windows")
	if err != nil || got != want {
		t.Errorf("a numeral returned by a kept definition on windows: want %s, got %q, %v", want, got, err)
	}
	if !keptHere("calc") {
		t.Error("calc was not kept: the witness does not reach a kept definition's numeral")
	}
	ir.PlantKeptNumeralAsTable = true
	got, err = runOnMayFail(t, numeral, "windows")
	ir.PlantKeptNumeralAsTable = false
	if err == nil || !strings.Contains(err.Error(), "W5") {
		t.Errorf("with the numeral registered by its carrier, want W5's refusal, got %q, %v", got, err)
	}

	const limbs = "use win/print\n\n" +
		"fn calc(n ∈ 0..1) (-2^300..2^300) {\n" +
		"    a = 3^100\n" +
		"    b = 0 - 5^60\n" +
		"    case {\n" +
		"        n == 0 -> a * b + n\n" +
		"        else -> a - b * a\n" +
		"    }\n" +
		"}\n\n" +
		"export fn main() {\n" +
		"    loop i = 0 {\n" +
		"        i <= 1 -> { print.print_str(decimal(calc(i))); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	// Python: a, b = 3**100, -5**60; a*b + 0, a - b*a
	const wantL = "-447018742102271829233201280861411252341454660234366935700478506987565197050571441650390625 " +
		"447018742102271829233201280861411252341455175611887667711809543448694962671844143757912626"
	got, err = runOnMayFail(t, limbs, "windows")
	if err != nil || got != wantL {
		t.Errorf("fixed limbs behind a definition on windows: want %s, got %q, %v", wantL, got, err)
	}
	if keptHere("calc") {
		t.Error("calc, its result on fixed limbs, was kept")
	}
	ir.PlantKeepFixedLimbs = true
	got, err = runOnMayFail(t, limbs, "windows")
	ir.PlantKeepFixedLimbs = false
	t.Logf("kept on fixed limbs: %q, %v", got, firstLine(fmt.Sprint(err)))
	if err == nil && got == wantL {
		t.Error("kept on fixed limbs, calc still answers: the witness cannot fail")
	}
}

// A DEFINITION ON FIXED LIMBS READS A KEPT NUMERAL BY K(ℕ*)'s d
// (keptnum-2026-11-22), the third case of the one above.
func TestAKeptNumeralIsReadByItsAbstractionFunction(t *testing.T) {
	defer func(k int) { ir.KeepThreshold = k }(ir.KeepThreshold)
	ir.KeepThreshold = 1
	// 3. a definition on fixed limbs by its own claim, calling a kept numeral,
	// reads it by K(ℕ*)'s d: LowerLimbs refuses the numeral on its limbs, and
	// `outer` leaves them for K(ℕ*) whole, as for an operation with no
	// instance. Read as its own limbs (PlantNumeralOnFixedLimbs), the numeral's
	// digits gave a wrong answer
	const outer = "use win/print\n\n" +
		"fn calc(n ∈ 0..1) (0..2^300) {\n" +
		"    a = 3^100\n" +
		"    case {\n" +
		"        n == 0 -> a & (2^200 - 1)\n" +
		"        else -> a >> 7\n" +
		"    }\n" +
		"}\n\n" +
		"fn outer(n ∈ 0..1) (0..2^301) { calc(n) + n }\n\n" +
		"export fn main() {\n" +
		"    loop i = 0 {\n" +
		"        i <= 1 -> { print.print_str(decimal(outer(i))); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	// Python: a = 3**100; a & (2**200 - 1), (a >> 7) + 1
	const wantO = "515377520732011331036461129765621272702107522001 4026386880718838523722352576293916192985215016"
	got, err := runOnMayFail(t, outer, "windows")
	if err != nil || got != wantO {
		t.Errorf("a definition on fixed limbs over a kept numeral: want %s, got %q, %v", wantO, got, err)
	}
	if !keptHere("calc") || !keptHere("outer") {
		t.Errorf("calc and outer are not both kept: %v", ir.LastKept())
	}
	ir.PlantNumeralOnFixedLimbs = true
	got, err = runOnMayFail(t, outer, "windows")
	ir.PlantNumeralOnFixedLimbs = false
	t.Logf("a numeral read on fixed limbs: %q, %v", got, firstLine(fmt.Sprint(err)))
	if err == nil && got == wantO {
		t.Error("a numeral read on fixed limbs still answers: the witness cannot fail")
	}
}

// keptHere says the last build kept n.
func keptHere(n string) bool {
	return ir.LastKept()[n]
}
