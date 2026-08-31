package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE DYNAMIC MAP PATH ON GO (maps.md §5.3, §8.1).
//
// A map read is `(option V)`, and a `case` on it expands to the Church
// eliminator applied to the read — so what reaches the emitter is an
// application whose OPERATOR is itself an application, the one shape where that
// is legitimate.
//
// It must emit as Go's own comma-ok. That is "emit at the highest layer the
// target natively provides" applied here: the host's fallible read IS the sum,
// so F2's option is not a thing we add but a thing Go already has and we were
// discarding. If this ever emits a helper function or a struct, the map has
// been lowered further than the target requires.
func TestAMapReadEmitsCommaOk(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	out := emitMapRun(t, tg)

	for _, want := range []string{
		"make(map[int]int, 8)", // build-map, with the capacity
		"[i] = (i * 10)",       // insert, Go's own in-place store
		", _tok := ",           // the comma-ok read
	} {
		if !strings.Contains(out, want) {
			t.Errorf("emitted Go is missing %q:\n%s", want, out)
		}
	}
	// And nothing was invented. A helper or a struct here would mean the sum
	// was materialised instead of eliminated against the host's own form.
	for _, bad := range []string{"struct", "func(", "interface"} {
		if strings.Contains(out, bad) {
			t.Errorf("a map read materialised %q; the host's fallible read IS "+
				"the sum and should need nothing:\n%s", bad, out)
		}
	}
}

// The function's RESULT TYPE comes from the clause bodies. `typeOf` assumed
// every application had a NAME as its operator, so a function whose value is a
// map read came out `/*unknown*/` — which compiles on `cmd/build`, where the
// definition is inlined into main, and not on `cmd/gen`, where it is emitted as
// a function. Two paths, one of which never exercised it.
func TestAMapReadHasTheClauseType(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	out := emitMapRun(t, tg)
	if strings.Contains(out, "unknown") {
		t.Errorf("the function's result type is unknown:\n%s", out)
	}
}

// A MAP TYPE RESOLVES THROUGH ONE DECLARATION, which is `array`'s argument one
// constructor over. The alternative is an entry per (K, V) pair —
// `targets/java/util.oro` calls that "the same limitation squared", having had
// to declare Map<String,Long> and nothing else.
func TestMapTypeResolvesThroughOneDeclaration(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	if got := tg.ty("map int int"); got != "map[int]int" {
		t.Errorf("map int int spelled %q, want map[int]int", got)
	}
	// A value type that is itself compound must survive the split. K may not be
	// compound — it is restricted to what `=` decides — so splitting on the
	// first space is unambiguous.
	if got := tg.ty("map int array f64"); got != "map[int][]float64" {
		t.Errorf("map int array f64 spelled %q, want map[int][]float64", got)
	}
}

// mapSrc is the program these tests emit. It goes through the REAL pipeline —
// read, load, reduce — because the emitter never sees source: reduction turns
// `case` into the Church eliminator applied to the read, and
// `((m k) (fn (#t #p) …))` is the shape the backend has to know. Handing the
// emitter a beta-redex would test a term nothing emits, and the residual cannot
// simply be pasted in because the reader does not round-trip a desugared loop.
const mapSrc = `
	(use go)
	(export run)
	(def run (fn (n k)
	  (let (build-map 8 (fn (m)
	         (loop ((m m) (i 0))
	           (go.>= i n)  m
	           else         (again (insert m i (go.* i 10)) (go.+ i 1)))))
	    (fn (m) (case (m k) (some v) v none -1)))))
	(sig run ((n int) (k int)) int)
`

func emitMapRun(t *testing.T, tg *Target) string {
	t.Helper()
	forms, err := core.Read(mapSrc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	nf, err := core.Normalize(prog.Defs["run"], env, core.DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	out, err := Func(tg, "run", prog.Sigs["run"], nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return out
}
