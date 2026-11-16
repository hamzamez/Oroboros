package main

import (
	"strings"
	"testing"

	"oroboros/emit"
)

// A MAP'S len IS ITS DOMAIN'S SIZE, |dom m|, on every target (the book's
// finding 71). Windows realizes a map as a table of 3 cells per slot, so `len`
// must be rewritten to the count of used slots while the map's type is still
// visible. A `var m := map(c)` buffer is handed on by the state translation
// through a tuple's elimination, (λk. k () m′)(λ(_, m). M), which the tracker
// did not follow, so `len(m)` was the backing table's length: 12 for three keys
// at capacity 2, 48 here. THE PLANTED FAULT forgets the tuple's map.
func TestAMapsLenIsItsDomainsSize(t *testing.T) {
	const src = "use io\n\nexport fn main() {\n    var m := map(8)\n    m[1] := 10\n    m[2] := 20\n" +
		"    m[1] := 30\n    m[3] := 40\n    io.print_int(len(m))\n}\n"
	for _, target := range []string{"go", "windows"} {
		out, err := buildAndRun(t, src, target, "")
		if err != nil || strings.TrimSpace(out) != "3" {
			t.Errorf("%s: len of a map with keys 1, 2, 3: %q, %v; want 3", target, out, err)
		}
	}
	emit.PlantMapTupleLost = true
	defer func() { emit.PlantMapTupleLost = false }()
	if out, err := buildAndRun(t, src, "windows", ""); err != nil || strings.TrimSpace(out) == "3" {
		t.Errorf("with the tuple's map forgotten len should be the table's length again; got %q, %v", out, err)
	}
}
