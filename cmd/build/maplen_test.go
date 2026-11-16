package main

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// A MAP IS REALIZED AFTER THE DECISION (ADR 0120), and its len is the size of
// its domain, |dom m|, on every target (the book's finding 71). Windows has no
// host map; its map is the table of its cells, each operation a term over it
// (targets/llvm/lang.oro's map_*), substituted on the IR after the decision
// (ir/maps.go). Before, the map was rewritten into that table before
// reduction, by a tracker of which binders hold maps, which lost a `var`
// buffer handed on through a tuple: `len(m)` was the backing table's length,
// 12 for three keys at capacity 2. The programs below are the book's, three
// keys and one key written twice, then keys stored in descending order and
// read back by `keys` in ascending order. THE PLANTED FAULT leaves the map's operations unrealized:
// the LLVM printer has none, so the build fails.
func TestAMapIsRealizedAfterTheDecision(t *testing.T) {
	cases := []struct{ src, want string }{
		{"use io\n\nexport fn main() {\n    var m := map(8)\n    m[1] := 10\n    m[2] := 20\n" +
			"    m[1] := 30\n    m[3] := 40\n    io.print_int(len(m))\n}\n", "3"},
		{"use io\n\nfn look(m, k) { case m[k] { 'some(v) -> v; 'none -> 0 } }\n\n" +
			"export fn main() {\n    m = {\n        var m := map(16)\n        loop i = 0 {\n" +
			"            i >= 5 -> m\n            else -> { m[10 - 2 * i] := i; again(i + 1) }\n" +
			"        }\n    }\n    ks = keys(m)\n    loop j = 0 {\n" +
			"        j < len(ks) -> { io.print_int(ks[j]); io.print_int(look(m, ks[j])); again(j + 1) }\n" +
			"        else -> io.print_int(len(m))\n    }\n}\n", "2 4 4 3 6 2 8 1 10 0 5"},
	}
	for _, c := range cases {
		for _, target := range []string{"go", "windows"} {
			out, err := buildAndRun(t, c.src, target, "")
			if err != nil || strings.Join(strings.Fields(out), " ") != c.want {
				t.Errorf("%s: %q, %v; want %s", target, out, err, c.want)
			}
		}
	}
	ir.PlantMapsUnrealized = true
	defer func() { ir.PlantMapsUnrealized = false }()
	if _, err := buildAndRunNoExec(t, cases[0].src, "windows"); err == nil {
		t.Error("with the map's operations unrealized the windows build should fail, so the test reaches the rule")
	}
}
