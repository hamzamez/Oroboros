package emit_test

import (
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
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
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	out := emitMapRun(t, tg)

	for _, want := range []string{
		`make\(map\[int\]int, 8\)`,    // build-map, with the capacity
		`v\d+\[v\d+\] = v\d+`,         // insert, Go's own in-place store
		`v\d+, ok\d+ := v\d+\[v\d+\]`, // the comma-ok read
	} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("emitted Go is missing %s:\n%s", want, out)
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
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	out := emitMapRun(t, tg)
	if strings.Contains(out, "unknown") {
		t.Errorf("the function's result type is unknown:\n%s", out)
	}
}

func emitMapRun(t *testing.T, tg *emit.Target) string {
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
	out, err := golang.FromResidual(tg, "run", prog.Sigs["run"], nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	return out
}
