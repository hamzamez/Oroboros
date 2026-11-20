package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// THE BOOK'S FINDINGS 100 AND 101: a map read and then stored to, and a table
// of tables or of tuples read at an index not known when compiling, each a
// construct its spec states and each refused in the lowering's words. The
// programs, on every target:
//   - Collatz's stopping times memoized in a map (L(2k) = L(k) + 1, a read of
//     m[n/2] then a write of m[n]): L(27) = 111, 20 of 20 even ones agreeing
//     with iteration. Its read's arms mention no cell, so the elimination is a
//     value of the state, not a step of it; threaded, the map came back as a
//     version with no length fact (core.PlantElimThreaded).
//   - Two Sum by one pass and a map: the read's arms write the answer's cells,
//     then the insert; (0, 1) for 9 and 15 of 15 targets agreeing with the
//     quadratic search. The pattern pushed into the read's eliminator
//     (core.PlantStateElimOutside), the read's eliminator a continuation to
//     every walker (emit.PlantMapReadNoContinuation), and a tuple pattern over
//     a loop read through by projectTail (emit.PlantJoinTailUnread).
//   - Lo Shu's magic square, a table of tables: 8 lines sum to 15, 9 entries
//     once each. A graph read at an unknown index is the n-way choice
//     (core.PlantTableOfTablesStuck).
//   - the shoelace formula over a table of tuples: twice the area 12, 50, −50.
//     A literal table of tuples flattened by currying (emit.PlantNoLiteralProduct).
//   - and a case on a read whose arms write a cell, then an insert: 51. The
//     lowering's map read is a variable's (ir.PlantReadShapeLoose).
func TestTheBooksFindings100And101(t *testing.T) {
	const caseThenInsert = `use io
export fn main() {
    var m := map(4)
    var n := 0
    m[9] := 50
    case m[9] {
        'some(v) -> { n := v }
        'none -> { n := 0 }
    }
    m[10] := n + 1
    io.print_int(case m[10] { 'some(x) -> x; 'none -> 0 })
}
`
	progs := []struct{ name, src, want string }{
		{"map/collatz", "", "111 20"},
		{"map/twosum", "", "0 1 15"},
		{"table/magic", "", "8 9"},
		{"table/shoelace", "", "12 50 -50"},
		{"case then insert", caseThenInsert, "51"},
	}
	for i := range progs {
		if progs[i].src == "" {
			b, err := os.ReadFile("../../examples/" + progs[i].name + ".oro")
			if err != nil {
				t.Fatal(err)
			}
			progs[i].src = string(b)
		}
	}
	for _, p := range progs {
		for _, tg := range []string{"go", "js", "java", "windows", "windows32"} {
			run := runOnMayFail
			if tg == "js" || tg == "java" {
				run = runOn
			}
			if got, err := run(t, p.src, tg); err != nil || strings.Join(strings.Fields(got), " ") != p.want {
				t.Errorf("%s on %s: want %s, got %q, %v", p.name, tg, p.want, got, err)
			}
		}
	}
	faults := []struct {
		plant *bool
		name  string
		prog  int
	}{
		{&core.PlantElimThreaded, "the elimination threaded", 0},
		{&core.PlantStateElimOutside, "the pattern outside the read", 1},
		{&emit.PlantMapReadNoContinuation, "the read no continuation", 1},
		{&emit.PlantJoinTailUnread, "the join point's tail unread", 1},
		{&core.PlantTableOfTablesStuck, "the table of tables stuck", 2},
		{&emit.PlantNoLiteralProduct, "the literal of tuples unflattened", 3},
		{&ir.PlantReadShapeLoose, "any application read as a map", 4},
	}
	for _, f := range faults {
		*f.plant = true
		_, err := buildOnly(t, progs[f.prog].src, "go")
		*f.plant = false
		if err == nil {
			t.Errorf("with %s, %s should be refused, so the witness can fail", f.name, progs[f.prog].name)
		}
	}
	// THE DOMAIN STAYS AN OBLIGATION: a row chosen at an index the program
	// cannot bound is refused, not read from the last row
	const unbounded = `use io
export fn f(k ∈ Int) (Int) {
    array { array { 1, 2 }, array { 3, 4 } }[k][0]
}
`
	if _, err := buildOnly(t, unbounded, "go"); err == nil {
		t.Error("a table of tables read at an unbounded index was accepted: the choice's domain was lost")
	}
}
