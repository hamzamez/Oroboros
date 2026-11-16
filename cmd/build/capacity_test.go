package main

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A MAP'S CAPACITY IS A CLAIM ON ITS len (maps.md §6, hamza's B′). A map of
// capacity c is in {m ∈ Map(K, V) : len(m) <= c}, so an insert m[k ↦ v] stays
// in it when len(m) < c; S marks each insert with that claim (core/cell.go), and
// it is decided from four laws, each with a planted fault:
//   - a fresh map is empty, |dom m| = 0 at map(c) (PlantMapNotEmpty);
//   - an insert adds at most one key, len(m) ≤ len(m′) ≤ len(m) + 1
//     (PlantInsertUnbounded);
//   - Houdini's |dom m| ≤ v for a loop threading a map (PlantNoMapTemplate);
//   - a length is an atom of its subject, not split inside (PlantSplitInsideLen).
//
// A program within its capacity builds and answers as on Go; one past it is
// refused on every target, where before Go grew the map and windows dropped
// the key.
func TestAMapsCapacityIsAClaimOnLen(t *testing.T) {
	const tally = "use io\n\nfn look(m, k) { case m[k] { 'some(v) -> v; 'none -> 0 } }\n\n" +
		"export fn main() {\n    t = array { 3, 1, 4, 1, 5, 9, 2, 6, 5, 3, 5 }\n" +
		"    m = {\n        var m := map(16)\n        loop i = 0 {\n            i >= len(t) -> m\n" +
		"            else -> { c = look(m, t[i]); m[t[i]] := if 0 <= c && c < 1000000 { c + 1 } else { c }; again(i + 1) }\n" +
		"        }\n    }\n    io.print_int(len(m))\n    io.print_int(look(m, 5))\n}\n"
	const three = "use io\n\nexport fn main() {\n    var m := map(8)\n    m[42] := 1\n    m[7] := 2\n    m[42] := 3\n    io.print_int(len(m))\n}\n"
	for _, target := range []string{"go", "windows"} {
		for what, c := range map[string]struct{ src, want string }{"the tally": {tally, "7 3"}, "three inserts": {three, "2"}} {
			out, err := buildAndRun(t, c.src, target, "")
			if err != nil || strings.Join(strings.Fields(out), " ") != c.want {
				t.Errorf("%s, %s: %q, %v; want %s", target, what, out, err, c.want)
			}
		}
	}
	over := map[string]string{
		"nine keys into map(8)": "use io\n\nexport fn main() {\n    m = {\n        var m := map(8)\n" +
			"        loop i = 0 { i >= 9 -> m; else -> { m[i] := i; again(i + 1) } }\n    }\n    io.print_int(len(m))\n}\n",
		"a count past the capacity": "use io\n\nfn fill(n ∈ 0..100) {\n    var m := map(8)\n" +
			"    loop i = 0 { i >= n -> len(m); else -> { m[i] := i; again(i + 1) } }\n}\n\n" +
			"export fn main() {\n    loop r = 0 { r >= 20 -> (); else -> { io.print_int(fill(r)); again(r + 1) } }\n}\n",
	}
	for _, target := range []string{"go", "windows"} {
		for what, src := range over {
			_, err := buildAndRunNoExec(t, src, target)
			if err == nil || !strings.Contains(err.Error(), core.MapCapacityDef) {
				t.Errorf("%s, %s: want it refused for the map's capacity, got %v", target, what, err)
			}
		}
	}
	for name, plant := range map[string]*bool{
		"the empty map forgotten": &emit.PlantMapNotEmpty,
		"an insert unbounded":     &emit.PlantInsertUnbounded,
		"no |dom m| ≤ i template": &emit.PlantNoMapTemplate,
		"a split inside a length": &emit.PlantSplitInsideLen,
		"no capacity mark at all": &core.PlantCapacityUnmarked,
	} {
		*plant = true
		_, errTally := buildAndRunNoExec(t, tally, "go")
		_, errOver := buildAndRunNoExec(t, over["nine keys into map(8)"], "go")
		*plant = false
		if plant == &core.PlantCapacityUnmarked {
			if errOver != nil {
				t.Errorf("with no capacity mark the nine keys should build again, so the test reaches the rule; got %v", errOver)
			}
			continue
		}
		if errTally == nil {
			t.Errorf("with %s the tally should be refused, so the test reaches the rule", name)
		}
	}
}
