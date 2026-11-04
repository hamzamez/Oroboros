package ir

import (
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A TABLE'S STORAGE IS ITS LAYOUT (ADR 0092). The pass asks `alloc(n, w)` for
// each table, and w must be the bytes every access reads an element by: one
// for a Boolean, eight for an integer. A wrong w is a region shorter than the
// accesses, so the expectations are literals, not ElemBytes.
func TestATablesStorageIsItsLayout(t *testing.T) {
	widths := func(src string) []int64 {
		t.Helper()
		tg, err := emit.LoadTarget("../targets/windows")
		if err != nil {
			t.Fatal(err)
		}
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, _, err := tg.LoadProgram(forms)
		if err != nil {
			t.Fatal(err)
		}
		env, err := tg.Env(prog)
		if err != nil {
			t.Fatal(err)
		}
		q := prog.Exports[0]
		nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		nf, err = DecideMarks(tg, q, prog.Sigs[q], emit.EtaTails(tg, nf), nil)
		if err != nil {
			t.Fatal(err)
		}
		f, err := Lower(tg, q, prog.Sigs[q], nf, Options{Decided: true})
		if err != nil {
			t.Fatal(err)
		}
		if n := Storage(tg, f); n == 0 {
			t.Fatal("no table was given storage")
		}
		konst := map[V]int64{}
		var out []int64
		f.Walk(func(r *Region) {
			for _, s := range r.Stmts {
				if s.Op == OConst && s.Lit.Kind == core.KInt {
					konst[s.Res[0]] = s.Lit.Int
				}
				if s.Op == OCall && s.Name == "alloc" {
					out = append(out, konst[s.Args[1]])
				}
			}
		})
		return out
	}
	for _, c := range []struct {
		what, src string
		want      int64
	}{
		{"a Boolean table", "export fn f(n ∈ 1..100) (Bool) { var t := array { false : _ ∈ 0..n-1 }; t[0] := true; t[0] }", 1},
		{"an integer table", "export fn f(n ∈ 1..100) (Int) { var t := array { 0 : _ ∈ 0..n-1 }; t[0] := n; t[0] }", 8},
		{"an array literal", "export fn f(n ∈ 0..1) (Int) { t = array { 5, 6, 7 }; t[n] }", 8},
	} {
		got := widths(c.src)
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: alloc asked %v bytes an element, want [%d]", c.what, got, c.want)
		}
	}

	// THE PLANTED FAULT: every table one byte an element.
	PlantStorageFault = true
	defer func() { PlantStorageFault = false }()
	if got := widths("export fn f(n ∈ 1..100) (Int) { var t := array { 0 : _ ∈ 0..n-1 }; t[0] := n; t[0] }"); len(got) == 1 && got[0] == 8 {
		t.Error("with every table one byte an element the integer table's width should be wrong, so the test reaches the pass")
	}
}
