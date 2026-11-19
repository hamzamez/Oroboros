package main

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A CLAIM ON A RECORD IS A CLAIM ON EACH COMPONENT (book-2026-11-20, found
// writing the book's chapter 11): e ∈ Π_l S_l iff e.l ∈ S_l. So
// `p ∈ Point = record { … }` keeps the record's labels for `p.x`, a component
// outside its set is refused naming it, and a value missing a label or giving
// an extra one is refused as X2 or X3. And `==` on a record says it is integer
// equality only. The faults: the claim left unsplit (`p.x` refused again), its
// ranged components left as typing claims (300 accepted for 0..255), and a
// product let through `==` to the lowering.
func TestAClaimOnARecordKeepsItsLabels(t *testing.T) {
	const point = "use io\nset Point = record { x ∈ Int, y ∈ Int }\n"
	cases := []struct{ name, src, want string }{
		{"projection", point + "export fn main() {\n    p ∈ Point = record { x : 2, y : 3 }\n    io.print_int(p.x)\n}\n", "2"},
		{"component", "use io\nset Pen = record { ink ∈ 0..255, x ∈ Int }\nexport fn main() {\n    p ∈ Pen = record { ink : 300, x : 1 }\n    io.print_int(p.x)\n}\n",
			"the claim ink ∈ 0..255 is false"},
		{"missing", point + "export fn main() {\n    p ∈ Point = record { x : 2 }\n    io.print_int(p.x)\n}\n", "gives no y (data.md §4.2, X2)"},
		{"extra", point + "export fn main() {\n    p ∈ Point = record { x : 2, y : 3, z : 4 }\n    io.print_int(p.x)\n}\n", "gives z, which is not among its labels"},
		{"equality", "use io\nexport fn main() {\n    a = record { x : 1, y : 2 }\n    b = record { x : 1, y : 2 }\n    io.print_line(if a == b { \"same\" } else { \"different\" })\n}\n",
			"is a tuple or a record, several values, and `==` is the language's integer equality only"},
		{"payload", "use io\nset Rect = record { width ∈ Int, height ∈ Int }\nset Shape = 'rect(Rect) ∪ 'point\nexport fn main() { io.print_int(1) }\n",
			"carries a record, and a variant's value is its tag and one slot per payload type"},
	}
	result := func(src string) string {
		t.Helper()
		got, err := runOnMayFail(t, src, "go")
		if err != nil {
			return err.Error()
		}
		return got
	}
	for _, c := range cases {
		if got := result(c.src); !strings.Contains(got, c.want) {
			t.Errorf("%s: want %q, got %q", c.name, c.want, got)
		}
	}
	core.PlantClaimHidesRecord = true
	got := result(cases[0].src)
	core.PlantClaimHidesRecord = false
	if got == "2" {
		t.Error("with a record's claim left unsplit, p.x should be refused again: the witness cannot fail")
	}
	core.PlantRecordClaimUnmarked = true
	got = result(cases[1].src)
	core.PlantRecordClaimUnmarked = false
	if strings.Contains(got, "is false") {
		t.Error("with a component's claim left a typing claim, ink = 300 should compile: the witness cannot fail")
	}
	// A SET MAY NOT MENTION ITSELF (type-algebra.md §3.1), refused where it is
	// defined; the fault reads it, and it compiles unused
	self := "use io\nset Node = record { value ∈ Int, next ∈ Node }\nexport fn main() { io.print_int(1) }\n"
	if got := result(self); !strings.Contains(got, "line 2: set Node mentions Node itself") {
		t.Errorf("a record set mentioning itself: want it refused at its definition, got %q", got)
	}
	core.PlantSelfSetAccepted = true
	got = result(self)
	core.PlantSelfSetAccepted = false
	if got != "1" {
		t.Errorf("with a set mentioning itself read, the unused definition should compile: got %q", got)
	}
	emit.PlantProductEqualityUnsaid = true
	got = result(cases[4].src)
	emit.PlantProductEqualityUnsaid = false
	if strings.Contains(got, "integer equality only") {
		t.Error("with a product let through `==`, the refusal should be the lowering's: the witness cannot fail")
	}
}

// A CLAIM THROUGH AN IMPORT NAMES ITS SET (book-2026-11-20, chapter 18):
// `box ∈ windef.RECT = record { … }` handed to PtInRect. The body's claim
// spelled the set by its alias, `windef.RECT`, which no reader resolved, so the
// record became a tuple under a claim no rule read, and the lowering refused
// it. The fault leaves the alias unresolved.
func TestAClaimThroughAnImportNamesItsSet(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs a windows program")
	}
	const src = "use win/print\nuse windows/winuser\nuse windows/windef\n" +
		"export fn main() {\n    box ∈ windef.RECT = record { left : 0, top : 0, right : 10, bottom : 5 }\n" +
		"    print.print_int(winuser.PtInRect(box, record { x : 3, y : 4 }))\n" +
		"    print.print_int(winuser.PtInRect(box, record { x : 10, y : 4 }))\n}\n"
	if got, err := runOnMayFail(t, src, "windows"); err != nil || got != "1 0" {
		t.Errorf("a claimed RECT at PtInRect: want 1 0, got %q, %v", got, err)
	}
	core.PlantClaimAliasUnresolved = true
	_, err := buildOnly(t, src, "windows")
	core.PlantClaimAliasUnresolved = false
	if err == nil {
		t.Error("with the claim's alias unresolved the program should be refused again: the witness cannot fail")
	}
}
