package main

import (
	"strings"
	"testing"

	"oroboros/ir/llvm"
)

// A RANGE ATTRIBUTE IS THE CONTRACT (φ's first kind, assessment-2026-11-16
// item 5; ir/llvm.rangeAttr). A kept definition's parameter in 0..10 is printed
// `range(i64 0, 11)`: every call proves its argument in the set, or P4 inlines
// it, so no argument outside [0, 11) occurs and the attribute adds no poison.
// The program calls f at 8, 9 and 10, the set's top end, where f's answer
// differs. THE PLANTED FAULT prints each range one narrower, [0, 10): LLVM may
// then take x == 10 as false, and the answer at 10 changes, so the witness
// shows that the attribute is read and that its end is the set's.
func TestARangeAttributeIsTheContract(t *testing.T) {
	const src = "use win/print\n\nfn f(x ∈ 0..10) (Int) {\n    loop i = 0, s = 0 {\n" +
		"        i >= 40 -> if x == 10 { s + 1000000 } else { s }\n" +
		"        else -> again(i + 1, s + (i * x) % 7 + (i + x) % 5 + (i * i + x) % 3 + (x * x + i) % 11)\n" +
		"    }\n}\n\nexport fn main() {\n    loop k = 8 {\n        k > 10 -> print.print_int(0)\n" +
		"        else -> { print.print_int(f(k)); again(k + 1) }\n    }\n}\n"
	// (the exit arm prints: a `main` whose value is a loop's () does not lower
	// on windows, the book's finding 59)
	want, err := buildAndRun(t, src, "windows", "")
	if err != nil {
		t.Fatal(err)
	}
	llvm.PlantNoRangeAttr = true
	without, err := buildAndRun(t, src, "windows", "")
	llvm.PlantNoRangeAttr = false
	if err != nil || without != want {
		t.Fatalf("the attribute changed the answer: %q with it, %q without (%v)", want, without, err)
	}
	if fs := strings.Fields(want); len(fs) != 4 || len(fs[2]) < 7 {
		t.Fatalf("f(10) should carry its 1000000: %q", want)
	}
	llvm.PlantRangeAttrNarrow = true
	narrow, err := buildAndRun(t, src, "windows", "")
	llvm.PlantRangeAttrNarrow = false
	if err != nil {
		t.Fatal(err)
	}
	if narrow == want {
		t.Errorf("with the range one narrower f(10) should change (LLVM reads x == 10 as false), "+
			"so the test reaches the rule; it printed %q both ways", want)
	}
}
