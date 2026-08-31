package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE BUILT-IN MAP, for the target that ships none (maps.md §8.4).
//
// Go, JavaScript and the JVM all bring a hash map and we parasitize it.
// windows brings none, and a target does not get to decline a language
// construct — so the language supplies one, written in Oroboros and lowered
// into ordinary buffers and loops before reduction.

// It must PARSE AND LOAD, which is not a given for a file nothing imports: it
// is reached only when a program on a mapless target uses a map, so a typo in
// it would lie dormant until then.
func TestTheBuiltInMapLoads(t *testing.T) {
	forms, err := core.Read(winMapSrc)
	if err != nil {
		t.Fatalf("the built-in map implementation does not parse: %v", err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatalf("the built-in map implementation does not load: %v", err)
	}
	for _, n := range []string{"wm-find", "wm-put", "wm-fill", "wm-slots", "wm-c"} {
		if _, ok := prog.Defs[mapImplPrefix+n]; !ok {
			t.Errorf("%s is missing; the rewrite in winmap.go calls it", n)
		}
	}
}

// WHICH TARGETS GET IT IS DECLARED, and this pins the reason. An empty
// `map-type` means "no map" on windows and "no TYPES" on JavaScript, which has
// a perfectly good map and spells nothing at all — so inferring it would hand
// JavaScript our hash table and throw away a measured 3.67x.
func TestOnlyTheMaplessTargetGetsTheBuiltIn(t *testing.T) {
	for _, c := range []struct {
		dir  string
		want bool
	}{
		{"../targets/windows", true},
		{"../targets/js", false},
		{"../targets/go", false},
		{"../targets/java", false},
	} {
		tg, err := LoadTarget(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := tg.NeedsMapImpl(); got != c.want {
			t.Errorf("%s: NeedsMapImpl is %v, want %v", c.dir, got, c.want)
		}
	}
}

// THE SLOT COUNT IS FOLDED when the capacity is a literal, which it almost
// always is — a capacity is a declaration somebody wrote. That removes a
// runtime loop from every program that builds a map.
//
// The load factor is what the doubling is for: at most half the slots are ever
// occupied, so linear probing stays short.
func TestSlotsOfFoldsALiteralCapacity(t *testing.T) {
	for _, c := range []struct{ cap, want int64 }{
		{1, 2}, {2, 4}, {3, 8}, {4, 8}, {5, 16}, {8, 16},
	} {
		got := slotsOf(core.Int(c.cap))
		if got.Kind != core.KInt {
			t.Fatalf("capacity %d did not fold: %s", c.cap, got)
		}
		if got.Int != c.want {
			t.Errorf("capacity %d gave %d slots, want %d", c.cap, got.Int, c.want)
		}
		if got.Int < 2*c.cap {
			t.Errorf("capacity %d gave %d slots, which is under the 1/2 load factor",
				c.cap, got.Int)
		}
	}
	// A dynamic capacity keeps the loop, because there is nothing to fold.
	dyn := slotsOf(core.Name("n"))
	if dyn.Kind != core.KApp || !strings.Contains(dyn.String(), "wm-slots") {
		t.Errorf("a dynamic capacity should keep the loop, got %s", dyn)
	}
}

// A SUM-RETURNING CALL MUST NOT BE REWRITTEN, and this is the sharp edge of the
// whole lowering.
//
// `(case (f x) (ok v) … )` expands to `((f x) (fn (#t #p) …))` — exactly the
// shape a map read has. Telling them apart is the operator's KIND: a map inside
// `build-map` is a lambda PARAMETER and therefore a KBound, while `f` is a
// KName. Accepting a bare KName would silently compile a sum elimination into a
// hash-table probe, which is a wrong answer rather than a missed case.
//
// `examples/sum/parse.oro` produces exactly this shape.
func TestASumReturningCallIsNotAMapRead(t *testing.T) {
	tg, err := LoadTarget("../targets/windows")
	if err != nil {
		t.Fatal(err)
	}
	defs := map[string]*core.Term{"f": core.Fn([]string{"x"}, core.Name("x"))}
	call := core.App(core.Name("f"), core.Name("x"))
	cont := core.Fn([]string{"#t", "#p"}, core.Name("#p"))
	env := newMapEnv(tg)
	if _, done := rewriteRead(tg, defs, env, core.App(call, cont)); done {
		t.Error("a sum-returning CALL was rewritten as a map read; that compiles " +
			"a different program, and it is what examples/sum/parse.oro looks like")
	}
	// AND A BOUND VARIABLE IS NOT ENOUGH EITHER: an array buffer is a KBound in
	// exactly the same position, so the binder must be one the tracking MARKED.
	buf := &core.Term{Kind: core.KBound}
	if _, done := rewriteRead(tg, defs, env, core.App(core.App(buf, core.Int(1)), cont)); done {
		t.Error("a read of an UNMARKED bound variable was rewritten as a map " +
			"read; that is what an array buffer looks like")
	}
	// The control: marked, it IS a map read — or the two checks above would
	// pass by rewriting nothing at all.
	marked := env.under(map[int]bool{0: true})
	if _, done := rewriteRead(tg, defs, marked, core.App(core.App(buf, core.Int(1)), cont)); !done {
		t.Error("a read of a MARKED buffer was not rewritten, so the checks " +
			"above prove nothing")
	}
}
