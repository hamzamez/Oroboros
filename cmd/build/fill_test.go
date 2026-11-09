package main

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// A ZERO FILL READ IS KEPT, EVEN IN A REUSED REGION (fill-2026-11-09). Each
// iteration makes a table of 10 with one element set, whose other nine are its
// zero fill, and sums it; the carried table dies at the iteration's end, so its
// region is the next table's token. The sum is 1 an iteration, 1000 in all,
// only if the reused region is zeroed. THE PLANTED FAULT takes every build as
// born full, so the reused region keeps the last table's element, and the sum
// is wrong.
func TestAZeroFillIsKeptWhereItIsRead(t *testing.T) {
	const src = "use win/print\n" +
		"export fn main() {\n" +
		"    print.print_int(loop k = 0, t = { var a := array { 0 : _ ∈ 0..9 }; a }, s = 0 {\n" +
		"        k >= 1000 -> s\n" +
		"        else -> {\n" +
		"            u = { var b := array { 0 : _ ∈ 0..9 }; b[k % 10] := 1 + len(t) % 2; b }\n" +
		"            again(k + 1, u, (s + u[0] + u[1] + u[2] + u[3] + u[4] + u[5] + u[6] + u[7] + u[8] + u[9]) % 1000000)\n" +
		"        }\n" +
		"    })\n}\n"
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.TrimSpace(out) != "1000" {
		t.Errorf("a fill read in a reused region: %q, %v; want 1000", out, err)
	}
	ir.PlantFillDead = true
	defer func() { ir.PlantFillDead = false }()
	out, err = buildAndRun(t, src, "windows", "")
	if err != nil {
		t.Fatalf("with every build taken as born full: %v", err)
	}
	if strings.TrimSpace(out) == "1000" {
		t.Errorf("with every build taken as born full, the reused region kept its old element and still summed to 1000: the witness cannot fail")
	}
}
