package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// reuseSrc carries a table of 10 round a loop and makes, on the continuing
// arm, a table of 20 and then its successor of 10, the carried table dying at
// the arm's end: one token of 10's size crosses the back edge, the table of 20
// must not take it, and the exit arm must give it back.
const reuseSrc = "export fn f(n ∈ 0..100) (Int) {\n" +
	"    loop k = 0, t = { var a := array { 0 : _ ∈ 0..9 }; a } {\n" +
	"        k >= n -> len(t)\n" +
	"        else -> {\n" +
	"            x = (if len(t) > 3 { t[2] } else { 0 }) % 7\n" +
	"            c = { var d := array { 0 : _ ∈ 0..19 }; d[0] := x; d }\n" +
	"            again(k + 1, { var b := array { 0 : _ ∈ 0..9 }; b[0] := x; b[1] := (c[0] + c[1] + len(t)) % 7; b })\n" +
	"        }\n" +
	"    }\n}\n"

// reuseEntry runs the windows pipeline on src and counts the zeroings and the
// heap allocations printed.
func reuseEntry(t *testing.T, src string) (zeros, allocs int, err error) {
	t.Helper()
	emit.ResetImports()
	tg, e := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
	if e != nil {
		t.Fatal(e)
	}
	forms, e := core.Read(src)
	if e != nil {
		t.Fatal(e)
	}
	prog, _, e := tg.LoadProgram(forms)
	if e != nil {
		t.Fatal(e)
	}
	env, e := tg.Env(prog)
	if e != nil {
		t.Fatal(e)
	}
	q := prog.Exports[0]
	nf, e := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if e != nil {
		t.Fatal(e)
	}
	f, _, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil, false, nil)
	if err != nil {
		return 0, 0, err
	}
	if err := ToP(tg, f); err != nil {
		return 0, 0, err
	}
	f.Walk(func(r *Region) {
		for _, s := range r.Stmts {
			switch {
			case s.Op == OCall && strings.HasSuffix(s.Name, "RtlZeroMemory"):
				zeros++
			case s.Op == OCall && strings.HasSuffix(s.Name, "HeapAlloc"):
				allocs++
			}
		}
	})
	return zeros, allocs, nil
}

// A DEAD TABLE'S STORAGE IS TAKEN BY THE NEXT TABLE OF ITS SIZE (ADR 0096,
// R1b): the carried table's region crosses the back edge as a token and is
// zeroed for its successor; W12 accepts it.
func TestADeadTablesStorageIsReused(t *testing.T) {
	zeros, _, err := reuseEntry(t, reuseSrc)
	if err != nil {
		t.Fatal(err)
	}
	if zeros < 1 {
		t.Errorf("want the carried table's region reused, zeroed at least once; got %d", zeros)
	}
}

// THE PLANTED FAULTS, each refused by W12: a token handed to a table of
// another size (the table of 20 would write past the 10's region), a region
// both handed on and released (freed, then written), and a token the exit
// arm keeps (consumed on no path there).
func TestAReuseW12RefusesIsAFault(t *testing.T) {
	for _, c := range []struct {
		name  string
		plant *bool
		why   string
	}{
		{"a token of another size", &PlantReuseSize, "bytes is handed to an allocation"},
		{"a region handed on and released", &PlantReuseKeepRelease, "released too"},
		{"a token the exit arm keeps", &PlantReuseNoDiscard, "consumed 0 times"},
	} {
		*c.plant = true
		_, _, err := reuseEntry(t, reuseSrc)
		*c.plant = false
		if err == nil || !strings.Contains(err.Error(), "W12") || !strings.Contains(err.Error(), c.why) {
			t.Errorf("%s: want W12's refusal (%q), got %v", c.name, c.why, err)
		}
	}
}
