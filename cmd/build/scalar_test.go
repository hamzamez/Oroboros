package main

import (
	"strings"
	"testing"

	"oroboros/ir/llvm"
)

// wideSrc holds 3·10⁹, a natural in [2³¹, 2³²): outside S₃₂, so in the word on
// windows32, and the value a 32-bit register read signed gets wrong. Its range
// is [0, 3·10⁹], so a printer that took the naturals below 2³² for S₃₂ would
// hold it in an i32. And a sum and a product of values in S₃₂, a quotient, a
// remainder and both shifts, each printed at 32 bits.
const wideSrc = "use win/print\nuse os\n\n" +
	"export fn main() {\n" +
	"    a = len(os.Args())\n" +
	"    x = if a <= 1 { a * 1000000000 + 2000000000 } else { 0 }\n" +
	"    ignore(print.print_int(x))\n" +
	"    y = if a <= 1 { a * 7 - 100 } else { 0 }\n" +
	"    ignore(print.print_int(y * 3 + y / 7 + y % 5 + (y >> 2) + (a << 20)))\n" +
	"}\n"

// A SCALAR IS HELD WHERE ITS RANGE IS, RUN (ADR 0127): on windows32 the
// program prints what it prints on windows, whose integers are all the word's.
// THE PLANTED FAULT takes the naturals below 2³² for S₃₂: 3·10⁹ is then held
// in an i32, and the program answers otherwise, or traps where LLVM reads the
// poison of an `nsw` that no longer holds.
func TestAScalarIsHeldWhereItsRangeIsRun(t *testing.T) {
	want, err := buildAndRun(t, wideSrc, "windows", "")
	if err != nil {
		t.Fatal(err)
	}
	if f := strings.Fields(want); len(f) != 2 || f[0] != "3000000000" {
		t.Fatalf("windows: %q", want)
	}
	got, err := buildAndRun(t, wideSrc, "windows32", "")
	if err != nil || got != want {
		t.Fatalf("windows32: %q, %v; want windows's %q", got, err, want)
	}
	llvm.PlantScalarWide = true
	exe, err := buildAndRunNoExec(t, wideSrc, "windows32")
	llvm.PlantScalarWide = false
	if err != nil {
		t.Fatalf("with the fault planted the program should still build: %v", err)
	}
	out, err := program(exe).CombinedOutput()
	if err == nil && string(out) == want {
		t.Errorf("with naturals below 2^32 taken for S32 the program still printed %q", out)
	}
}
