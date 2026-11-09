package ir

import (
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// fillOf lowers src on windows up to the release and reuse passes and answers,
// for each build, whether the written-prefix theorem proves its fill dead.
func fillOf(t *testing.T, src string) []bool {
	t.Helper()
	emit.ResetImports()
	tg, err := emit.LoadTargetLayers("windows", []string{"../targets"}, []string{"../lib"})
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
	f, err := Lower(tg, q, prog.Sigs[q], emit.EtaTails(tg, nf), Options{Decided: true})
	if err != nil {
		t.Fatal(err)
	}
	var out []bool
	f.Walk(func(r *Region) {
		for i := range r.Stmts {
			if r.Stmts[i].Op == OBuild {
				out = append(out, fillDead(f, &r.Stmts[i]))
			}
		}
	})
	return out
}

// THE WRITTEN-PREFIX THEOREM (ir/fill.go): a build whose loop writes every
// element at its counter before any read is born full, ascending or
// descending; one that reads its buffer before every element is written, or
// reads what it never wrote, keeps its zero fill.
func TestTheWrittenPrefixTheorem(t *testing.T) {
	for _, c := range []struct {
		name string
		src  string
		dead bool
	}{
		{"ascending, add's shape", "export fn f(n ∈ 1..100, k ∈ 0..1000) (Int) {\n" +
			"    t = { var o := array { 0 : _ ∈ 0..n-1 }; loop i = 0, c = k { i >= n -> o; else -> { o[i] := c % 7; again(i + 1, c / 7) } } }\n" +
			"    t[0]\n}\n", true},
		{"descending, div_small's shape", "export fn f(n ∈ 1..100, k ∈ 0..1000) (Int) {\n" +
			"    t = { var o := array { 0 : _ ∈ 0..n-1 }; loop i = n - 1, r = k { i < 0 -> o; else -> { o[i] := r % 7; again(i - 1, r / 7) } } }\n" +
			"    t[0]\n}\n", true},
		{"two continuing arms, each writing at the counter: the sign entry's shape", "export fn f(n ∈ 1..100, k ∈ 0..1000) (Int) {\n" +
			"    t = { var o := array { 0 : _ ∈ 0..n }; loop i = 0, c = k { i >= n + 1 -> o; i == n -> { o[i] := 1; again(i + 1, c) }; else -> { o[i] := c % 7; again(i + 1, c / 7) } } }\n" +
			"    t[0]\n}\n", true},
		{"two continuing arms, one not writing", "export fn f(n ∈ 1..100, k ∈ 0..1000) (Int) {\n" +
			"    t = { var o := array { 0 : _ ∈ 0..n }; loop i = 0, c = k { i >= n + 1 -> o; i == n -> again(i + 1, c); else -> { o[i] := c % 7; again(i + 1, c / 7) } } }\n" +
			"    t[0] + t[n]\n}\n", false},
		{"a second exit before the table is full", "export fn f(n ∈ 2..100, k ∈ 0..1000) (Int) {\n" +
			"    t = { var o := array { 0 : _ ∈ 0..n-1 }; loop i = 0, c = k { i >= n -> o; c == 3 -> o; else -> { o[i] := c % 7; again(i + 1, c / 7) } } }\n" +
			"    t[n - 1]\n}\n", false},
		{"a read of the buffer before it is full","export fn f(n ∈ 2..100, k ∈ 0..1000) (Int) {\n" +
			"    t = { var o := array { 0 : _ ∈ 0..n-1 }; loop i = 0 { i >= n -> o; else -> { o[i] := (o[n - 1] + k) % 7; again(i + 1) } } }\n" +
			"    t[0]\n}\n", false},
		{"a fill read where nothing wrote, the sieve's shape", "export fn f(n ∈ 2..100, k ∈ 0..99) (Int) {\n" +
			"    t = { var o := array { 0 : _ ∈ 0..n-1 }; o[k % n] := 1; o }\n" +
			"    t[0] + t[1]\n}\n", false},
	} {
		got := fillOf(t, c.src)
		if len(got) == 0 {
			t.Errorf("%s: no build", c.name)
			continue
		}
		if got[0] != c.dead {
			t.Errorf("%s: fill dead = %v, want %v", c.name, got[0], c.dead)
		}
	}
}
