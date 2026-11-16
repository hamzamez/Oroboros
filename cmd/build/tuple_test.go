package main

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// A TUPLE PATTERN ON A TABLE'S ELEMENT IS ITS PROJECTIONS (the book's finding
// 64). A table of pairs is flattened by currying, k·i + j, and `(x, y) = p[i]`
// is the product's eliminator on an element: by η, (λ(x, y). M)(π₀ p[i],
// π₁ p[i]), each projection uncurried. The flattening pass had the projection
// and not the eliminator, so the pair reached lowering as an application and
// was read as a map's (present, value), x the flag: Go did not compile and
// windows had no read to print. THE PLANTED FAULT drops the eliminator's case.
func TestATuplePatternOnAnElementIsItsProjections(t *testing.T) {
	const src = "use io\n" +
		"\n" +
		"fn width(p ∈ Array(0..100 × 0..100)) (Int) where len(p) <= 1000 {\n" +
		"    loop i = 0, s = 0 {\n" +
		"        i < len(p) -> {\n" +
		"            (x, y) = p[i]\n" +
		"            again(i + 1, s + 10 * x + y)\n" +
		"        }\n" +
		"        else -> s\n" +
		"    }\n" +
		"}\n" +
		"\n" +
		"export fn main() {\n" +
		"    var p := array { 0 : _ ∈ 0..3 }\n" +
		"    loop k = 0 {\n" +
		"        k >= 4 -> io.print_int(width(p))\n" +
		"        else -> { p[k] := (k, k + 1); again(k + 1) }\n" +
		"    }\n" +
		"}\n" +
		""
	for _, target := range []string{"go", "windows"} {
		out, err := buildAndRun(t, src, target, "")
		if err != nil || strings.TrimSpace(out) != "70" {
			t.Errorf("%s: Σ 10x + y over (k, k + 1), k < 4: %q, %v; want 70", target, out, err)
		}
	}
	emit.PlantNoProductElim = true
	defer func() { emit.PlantNoProductElim = false }()
	if _, err := buildAndRunNoExec(t, src, "go"); err == nil {
		t.Error("with the eliminator's case dropped the program should fail to build again")
	}
}
