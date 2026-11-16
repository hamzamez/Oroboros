package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A LOOP'S INVARIANT IS HOARE'S RULE (j.md §2.8, the book's finding 60):
//
//	{I(z⃗)} entry,   {I ∧ guard} body {I(x⃗′)} at every again,   so I in the body
//
// `invariant I` is a claim decided at the loop: I on the initial values, and I
// a conjunct of Houdini's greatest inductive set, which is the induction step,
// since a conjunct kept is preserved by every back edge assuming the kept set.
// One dropped refuses the program. Kept, I is a fact of the body, so a
// relation outside Houdini's templates (s ≤ 2·i, whose coefficient none has)
// proves an index the program could not prove before.
//
// Four planted faults:
//   - the stated invariant read as kept whatever Houdini found
//     (PlantInvariantUnchecked);
//   - the invariant's mark read as a closure over the state (PlantMarkIsAClosure);
//   - a buffer's version at the loop head without its initial length, len t =
//     len z, the store law (PlantNoVersionLength);
//   - an out cell's value without its cell's set (PlantCellValueUnbounded).
func TestALoopsInvariantIsHoaresRule(t *testing.T) {
	// s rises by 1 or 2 a step, so s ≤ 2·i, and t[s] is inside a table of 201.
	loop := func(inv string) string {
		return "use io\n\nfn walk(n ∈ 0..100) (Int) {\n    t = alloc(array { k % 10 : k ∈ 0..200 })\n" +
			"    loop i = 0, s = 0" + inv + " {\n" +
			"        i >= n -> s\n" +
			"        else -> { x = t[s]; again(i + 1, if x > 4 { s + 2 } else { s + 1 }) }\n" +
			"    }\n}\n\n" +
			"export fn main() {\n    io.print_int(walk(100))\n    io.print_int(walk(7))\n}\n"
	}
	held := loop(" invariant 0 <= s && s <= 2 * i")
	for _, target := range []string{"go", "windows"} {
		out, err := buildAndRun(t, held, target, "")
		if err != nil || strings.Join(strings.Fields(out), " ") != "142 9" {
			t.Errorf("%s: s <= 2 * i proves t[s]: %q, %v; want 142 9", target, out, err)
		}
		if _, err := buildAndRunNoExec(t, loop(""), target); err == nil {
			t.Errorf("%s: with no invariant t[s] should be unproven, so the invariant is what proves it", target)
		}
	}
	refused := map[string]struct{ src, says string }{
		"not inductive": {loop(" invariant 0 <= s && s <= i"), "at every iteration"},
		"false on entry": {"use io\n\nexport fn main() {\n    loop i = 0, s = 1 invariant s == 0 {\n" +
			"        i >= 3 -> io.print_int(s)\n        else -> again(i + 1, s)\n    }\n}\n", "on entry"},
		"a buffer's element": {"use io\n\nexport fn main() {\n    var b := array { 0 : _ ∈ 0..3 }\n" +
			"    loop i = 0 invariant b[0] == 0 {\n        i >= 4 -> io.print_int(b[0])\n" +
			"        else -> { b[i] := i; again(i + 1) }\n    }\n}\n", "other than through its length"},
	}
	for _, target := range []string{"go", "windows"} {
		for what, c := range refused {
			_, err := buildAndRunNoExec(t, c.src, target)
			if err == nil || !strings.Contains(err.Error(), c.says) {
				t.Errorf("%s, %s: want it refused saying %q, got %v", target, what, c.says, err)
			}
		}
	}

	// A buffer threaded through the loop: the invariant reads len(b), the body's
	// facts its version at the loop head, joined by len t = len z.
	const lent = "use io\n\nfn fill(b ∈ Array(0..255) linear, g ∈ 0..4294967295) (Int) {\n" +
		"    loop lo = 0, got = 1 invariant 0 <= lo && lo <= len(b) {\n" +
		"        got == 0 || lo >= len(b) -> lo\n        else -> {\n" +
		"            rest = len(b) - lo\n            n = if rest > 64 { 64 } else if rest < 1 { 0 } else { rest }\n" +
		"            k = if g > n { n } else { g }\n            b[lo] := 7\n            again(lo + k, k)\n        }\n    }\n}\n\n" +
		"export fn main() {\n    var b := array { 0 : _ ∈ 0..99 }\n    io.print_int(fill(b, 30))\n}\n"
	for _, target := range []string{"go", "windows"} {
		out, err := buildAndRun(t, lent, target, "")
		if err != nil || strings.TrimSpace(out) != "100" {
			t.Errorf("%s: an invariant over a lent buffer: %q, %v; want 100", target, out, err)
		}
	}

	// The out cell: readinto's invariant, 0 <= lo, holds because ReadFile's
	// count is a DWORD, written through the cell (lib/os/windows.oro).
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "in.txt"))
	if err := os.WriteFile(path, []byte("twelve bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	reader := "use os\nuse io\nuse result\n\nexport fn main() {\n" +
		"    case os.ReadFile(\"" + path + "\") {\n" +
		"        result.'ok(b) -> io.print_int(len(b))\n        result.'err(e) -> io.print_int(-1)\n    }\n}\n"
	if out, err := buildAndRun(t, reader, "windows", ""); err != nil || strings.TrimSpace(out) != "12" {
		t.Errorf("windows: os.ReadFile through readinto: %q, %v; want 12", out, err)
	}

	// s rises by 2 a step, so s <= i is false at the second iteration and
	// refused by nothing but the induction step.
	const twice = "use io\n\nexport fn main() {\n    loop i = 0, s = 0 invariant 0 <= s && s <= i {\n" +
		"        i >= 5 -> io.print_int(s)\n        else -> again(i + 1, s + 2)\n    }\n}\n"
	if _, err := buildAndRunNoExec(t, twice, "go"); err == nil || !strings.Contains(err.Error(), "at every iteration") {
		t.Errorf("s <= i with s rising by 2 should be refused as not kept, got %v", err)
	}
	for name, c := range map[string]struct {
		plant  *bool
		src    string
		target string
		builds bool // with the fault planted
	}{
		"the invariant unchecked":            {&emit.PlantInvariantUnchecked, twice, "go", true},
		"the decided invariant erased":       {&core.PlantInvariantErased, held, "go", false},
		"the mark read as a closure":         {&core.PlantMarkIsAClosure, lent, "go", false},
		"a version without its length":       {&emit.PlantNoVersionLength, reader, "windows", false},
		"a cell's value unbounded":           {&emit.PlantCellValueUnbounded, reader, "windows", false},
		"an allocated rule without a length": {&emit.PlantRuleNoLength, held, "go", false},
	} {
		*c.plant = true
		_, err := buildAndRunNoExec(t, c.src, c.target)
		*c.plant = false
		if (err == nil) != c.builds {
			t.Errorf("with %s the program should build: %v, but got %v, so the test does not reach the rule", name, c.builds, err)
		}
	}
}
