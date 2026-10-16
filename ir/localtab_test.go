package ir

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// entryOn takes a program's first export through the drivers' pipeline
// (ir.Entry) and the step to IR_P, verified (ToP), on a target.
func entryOn(t *testing.T, target, src string) (*Func, error) {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/" + target)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read("use " + target + "\n" + src)
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
	var sigs []*core.Sig
	for _, s := range prog.Sigs {
		sigs = append(sigs, s)
	}
	f, _, err := Entry(tg, q, q, prog.Sigs[q], emit.EtaTails(tg, nf), sigs, false, nil)
	if err != nil {
		return nil, err
	}
	return f, ToP(tg, f)
}

// A TABLE NOTHING STORES INTO IS TYPED (types.md §3.1, localtab-2026-10-14).
// `(table n 0)` is the zero-filled buffer with no fill loop, so when no store
// follows, its element is fixed by a read's demand or by nothing. Go types every
// value, and the element was left `any`: "W9: %11's type array any is not
// final", the verifier's failure where a program was correct.
func TestATableNothingStoresIsTyped(t *testing.T) {
	prog := func(result, body string) string {
		return "export fn f(n ∈ 1..100) (" + result + ") {\n" +
			"    src = { var s := array { 0 : _ ∈ 0..n+1-1 }; s }\n    " + body + "\n}\n"
	}
	elem := func(f *Func) string {
		for _, ty := range f.Types {
			if strings.HasPrefix(ty, "array ") {
				return strings.TrimPrefix(ty, "array ")
			}
		}
		return ""
	}
	cases := []struct {
		result, body string
		want         string  // the frozen table's element, by prefix
		faults       []*bool // the planted faults that, together, lose it
	}{
		// a read the function returns: the declared result is its demand.
		// At int the default agrees, so only both faults together lose it.
		{"Int", "src[n]", "int", []*bool{&plantResultDemandFault, &plantZeroFillFault}},
		{"F64", "src[n]", "f64", []*bool{&plantResultDemandFault}},
		{"Bool", "src[n]", "bool", []*bool{&plantResultDemandFault}},
		// nothing reads an element: the zero fill's sort, the integer
		{"Int", "len(src)", "int", []*bool{&plantZeroFillFault}},
	}
	for _, c := range cases {
		for _, target := range []string{"go", "java"} {
			f, err := entryOn(t, target, prog(c.result, c.body))
			if err != nil {
				t.Errorf("%s: %s returning %s: %v", target, c.body, c.result, err)
				continue
			}
			if got := elem(f); !strings.HasPrefix(got, c.want) {
				t.Errorf("%s: %s returning %s: the table's element is %q, want %s", target, c.body, c.result, got, c.want)
			}
		}
		// PLANTED: the rule removed. The program must fail again, so this
		// test sees the rule.
		for _, p := range c.faults {
			*p = true
		}
		_, err := entryOn(t, "go", prog(c.result, c.body))
		for _, p := range c.faults {
			*p = false
		}
		if err == nil {
			t.Errorf("%s returning %s: with the rule's fault planted it still compiles, so the test cannot see the rule", c.body, c.result)
		}
	}
}

// A VALUE NOTHING TYPES IS REFUSED IN WORDS, not by the verifier. A map built
// and never written or read has neither key nor value fixed.
func TestAValueNothingTypesIsRefusedInWords(t *testing.T) {
	_, err := entryOn(t, "go", `export fn f(n ∈ 1..100) (Int) {
    m = build_map(4, fn(m) { m })
    n
}
`)
	if err == nil || !strings.Contains(err.Error(), "cannot determine a type for the value of (build-map") ||
		strings.Contains(err.Error(), "W9") {
		t.Errorf("want the untyped map refused in words: %v", err)
	}
	// And the zero-filled table's planted fault, past the typing, meets this
	// refusal and not W9.
	plantZeroFillFault = true
	defer func() { plantZeroFillFault = false }()
	_, err = entryOn(t, "go", `export fn f(n ∈ 1..100) (Int) {
    src = { var s := array { 0 : _ ∈ 0..n + 1-1 }; s }
    len(src)
}
`)
	if err == nil || !strings.Contains(err.Error(), "cannot determine a type") || strings.Contains(err.Error(), "W9") {
		t.Errorf("want the open element refused in words: %v", err)
	}
}
