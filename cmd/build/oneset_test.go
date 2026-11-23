package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/ir"
)

// THE ONE SET IS CHECKED WHERE IT CLAIMS (ADR 0152, amending ADR 0029). A
// declared set above the word is the program's one set, by sign and bit
// length; it is checked at each claim, a value a declared set is ascribed to,
// and at an operation whose result the decision does not bound, and not at an
// operation with a finite fact, ℤ's where nothing claims it. So:
//
//  1. x·3/4 over a declared S₆₄, a set above JavaScript's word: x·3 is past S₆₄,
//     claimed by nothing, and the answer is in S₆₄; it answers on JavaScript,
//     as on Go (the double word). PlantFitEveryOp, ADR 0029's every operation,
//     stops it;
//  2. on JavaScript's fixed limbs, x·x − x·x + x over naturals declared
//     0..2^100 answers, the program leaving 101-bit limbs, which cannot hold
//     x·x, for the host's integer: PlantLimbsBeyondWidth keeps it there, and
//     it stops;
//  3. a yield past its declared −2^200..2^200 still stops, at the claim, when
//     the definition is inlined into main (big-sign's): PlantNoClaimFit lets
//     it answer, the wrong answer the claim exists to prevent.
func TestTheOneSetIsCheckedWhereItClaims(t *testing.T) {
	const quarters = "use io\n\n" +
		"fn tq(x ∈ -2^63..2^63-1) (-2^63..2^63-1) { x * 3 / 4 }\n\n" +
		"export fn main() {\n" +
		"    loop i = 0 {\n" +
		"        i < 2 -> { io.print_int(tq(9223372036854775807 - i)); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	// x unknown, a loop's: read from a one-element table x was a point, and
	// x·3 a closed term, its constant (ADR 0157), with no operation for a fit
	// to stop; and on JavaScript a table of integers is Numbers, which a literal
	// 2⁶³ − 1 does not fit (it printed 2⁶³ rounded, and is refused now)
	const want = "6917529027641081855 6917529027641081854"
	for _, tg := range []string{"js", "go"} {
		if got, err := runOn(t, quarters, tg); err != nil || got != want {
			t.Errorf("x·3/4 at S₆₄'s top on %s: want %s, got %q, %v", tg, want, got, err)
		}
	}
	ir.PlantFitEveryOp = true
	got, err := runJSRepr(t, quarters, "")
	ir.PlantFitEveryOp = false
	if err == nil && got == want {
		t.Error("with every operation fitted, x·3/4 still answers on JavaScript: the witness cannot fail")
	}
	// fixed limbs have no division by a value held exactly, and on JavaScript no
	// signed form, so x·3/4 leaves them in any case; x·x − x·x + x over
	// naturals, ring operations alone, declared 0..2^100, is a program they
	// hold, and x·x reaches 2^120, past their 101 bits
	const back = "use io\n\n" +
		"fn back(x ∈ 0..2^60) (0..2^100) { x * x - x * x + x }\n\n" +
		"export fn main() {\n" +
		"    t = array { 1073741824 }\n" +
		"    loop i = 0 {\n" +
		"        i < 1 -> { io.print_line(decimal(back(t[i] * 1073741824))); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	const top = "1152921504606846976"
	if got, err := runJSRepr(t, back, "limbs"); err != nil || got != top {
		t.Errorf("x·x − x·x + x on JavaScript's fixed limbs: want %s, got %q, %v", top, got, err)
	}
	ir.PlantLimbsBeyondWidth = true
	got, err = runJSRepr(t, back, "limbs")
	ir.PlantLimbsBeyondWidth = false
	if err == nil && got == top {
		t.Error("kept on 101-bit limbs, x·x − x·x + x still answers on JavaScript: the witness cannot fail")
	}

	const claim = "use io\n\n" +
		"fn calc(n ∈ 0..2) (-2^200..2^200) {\n" +
		"    p = 2^200\n" +
		"    b = if n == 0 { 0 } else if n == 1 { p - 1 } else { p }\n" +
		"    0 - p - b\n" +
		"}\n\n" +
		"export fn main() {\n" +
		"    loop i = 0 {\n" +
		"        i < 3 -> { io.print_line(decimal(calc(i))); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	const two = "-1606938044258990275541962092341162602522202993782792835301376 " +
		"-3213876088517980551083924184682325205044405987565585670602751"
	for _, repr := range []string{"", "limbs"} {
		got, err := runJSRepr(t, claim, repr)
		if err == nil || !strings.HasPrefix(got, two) || strings.Contains(got, "-3213876088517980551083924184682325205044405987565585670602752") {
			t.Errorf("calc(2), past its declared set, on JavaScript %q: want two answers and a stop, got %q, %v", repr, got, err)
		}
	}
	ir.PlantNoClaimFit = true
	got, err = runJSRepr(t, claim, "")
	ir.PlantNoClaimFit = false
	if err == nil {
		t.Errorf("with no fit at the claim, calc(2) answers %q: the witness cannot fail", got)
	}
}

// runJSRepr builds src for JavaScript, its rung the host's integer ("") or
// fixed limbs ("limbs"), and runs it; a refusal or a run's failure is
// returned, not fatal.
func runJSRepr(t *testing.T, src, repr string) (string, error) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "p.oro")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	art := filepath.Join(dir, "p.mjs")
	if err := run("../../targets", path, "js", art, "../../lib", false, false, repr); err != nil {
		return "", err
	}
	b, err := program("node", art).Output()
	return strings.Join(strings.Fields(string(b)), " "), err
}
