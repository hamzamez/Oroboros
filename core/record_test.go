package core

import (
	"strings"
	"testing"
)

// normJ reduces definition f of a J program to its normal form's text; a
// refusal by the reader or by Load is its error.
func normJ(t *testing.T, src, f string) (string, error) {
	t.Helper()
	forms, err := Read(src)
	if err != nil {
		return "", err
	}
	p, _, err := Load(forms)
	if err != nil {
		return "", err
	}
	nf, err := Normalize(p.Defs[f], placeEnv(p), DefaultFuel)
	if err != nil {
		return "", err
	}
	return nf.String(), nil
}

// A RECORD IS A FUNCTION ON ITS LABELS (ADR 0128, data.md §4): its value is the
// same whatever order its graph was written in, and projection and override
// are a very well-behaved lens. Each law is evaluated on a record whose
// components differ, so an override that wrote the wrong label, or every
// label, gives another number (data.md §9's record-order and record-lens).
func TestARecordIsAFunctionOnItsLabels(t *testing.T) {
	const r = "r = record { y : 2, x : 1 }\n"
	cases := []struct{ name, body, want string }{
		{"order: a value is its graph, whatever order it is written in", r + "r.x * 10 + r.y", "12"},
		{"order: two spellings of one record", r + "s = record { x : 1, y : 2 }\n(r.x - s.x) * 10 + (r.y - s.y)", "0"},
		{"put–get", r + "(r with { x : 5 }).x", "5"},
		{"put–get at another label", r + "(r with { x : 5 }).y", "2"},
		{"get–put", r + "s = r with { x : r.x }\ns.x * 10 + s.y", "12"},
		{"put–put", r + "s = (r with { x : 5 }) with { x : 7 }\ns.x * 10 + s.y", "72"},
		{"two pairs commute", r + "s = r with { y : 4, x : 3 }\ns.x * 10 + s.y", "34"},
		{"a path is lens composition",
			"n = record { a : record { b : 1, c : 2 }, d : 3 }\nm = n with { a.b : 9, d : 4 }\nm.a.b * 100 + m.a.c * 10 + m.d", "924"},
		{"a path against nested withs",
			"n = record { a : record { b : 1, c : 2 }, d : 3 }\nm = n with { a : n.a with { c : 8 } }\nk = n with { a.c : 8 }\n(m.a.c - k.a.c) * 10 + m.a.b + k.a.b", "2"},
		{"a cell's component, written in program order",
			"var c := record { x : 1, y : 2 }\nc.x := 6\nc.y := c.x + 1\nc.x * 10 + c.y", "67"},
	}
	for _, c := range cases {
		src := "fn f() {\n" + c.body + "\n}\n"
		got, err := normJ(t, src, "f")
		if err != nil {
			t.Errorf("%s: %v\n%s", c.name, err, src)
			continue
		}
		if got != "(fn () "+c.want+")" {
			t.Errorf("%s: %s, want (fn () %s)\n%s", c.name, got, c.want, src)
		}
	}
	// A RECORD OBSERVED WHOLE, a body's value: the override on a value it sees,
	// in canonical order whatever the order written.
	whole := "fn f() {\n" + r + "r with { x : 5 }\n}\n"
	if got, err := normJ(t, whole, "f"); err != nil || got != `(fn () (#rec "x y" 5 2))` {
		t.Errorf("a record observed whole: %s, %v", got, err)
	}
	// THE FAULTS, one per rule: an override that writes the label after the one
	// it names, and a put–get that answers the base's stale component. Each
	// law's case must see its rule's.
	for _, c := range []struct {
		name       string
		plant      *bool
		src, right string
	}{
		{"an override writing the wrong label", &PlantWithWrongLabel, whole, `(fn () (#rec "x y" 5 2))`},
		{"a projection answering another label's component", &PlantPutGetStale,
			"fn f() {\n" + r + "(r with { x : 5 }).x\n}\n", "(fn () 5)"},
	} {
		*c.plant = true
		got, err := normJ(t, c.src, "f")
		*c.plant = false
		if err == nil && got == c.right {
			t.Errorf("%s still passes: the law's case cannot fail", c.name)
		}
	}
}

// A RECORD'S COMPONENTS ARE EVALUATED IN THE ORDER WRITTEN, though the record
// reads them in canonical order (recordFormer): sorting the terms would run
// the second-written effect first.
func TestARecordIsEvaluatedInTheOrderWritten(t *testing.T) {
	p, err := loadJ(t, "fn f() {\nr = record { b : first(1), a : second(2) }\nr.a + r.b\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	e := placeEnv(p)
	e.Prim["first"], e.Prim["second"] = true, true // impure: each an effect
	nf, err := Normalize(p.Defs["f"], e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	s := nf.String()
	i, j := strings.Index(s, "first"), strings.Index(s, "second")
	if i < 0 || j < 0 || i > j {
		t.Errorf("the components' effects run out of written order: %s", s)
	}
}

// Each refusal of data.md §4.2 and ADR 0128 names what it refuses, and is
// computed from its premise: the same program with the premise removed reads.
func TestARecordsRefusalsNameTheirReason(t *testing.T) {
	for _, c := range []struct{ name, body, ok, reason string }{
		{"X9: a former mixing a set and a value",
			"r = record { x : 1, y ∈ 0..9 }\nr.x", "r = record { x : 1, y : 2 }\nr.x", "mixes a set's"},
		{"X1: a label twice", "r = record { x : 1, x : 2 }\nr.x", "r = record { x : 1, w : 2 }\nr.x", "appears twice"},
		{"X10: a path another's prefix",
			"n = record { a : record { b : 1 } }\nm = n with { a : record { b : 2 }, a.b : 3 }\nm.a.b",
			"n = record { a : record { b : 1 } }\nm = n with { a.b : 3 }\nm.a.b", "not prefix-free"},
		{"X10: a path repeated", "r = record { x : 1 }\n(r with { x : 2, x : 3 }).x", "r = record { x : 1 }\n(r with { x : 3 }).x", "not prefix-free"},
		{"X3: a projection outside the domain", "r = record { x : 1 }\nr.y", "r = record { x : 1 }\nr.x", "application outside its domain"},
		{"X3: an override outside the domain", "r = record { x : 1 }\n(r with { y : 2 }).x", "r = record { x : 1 }\n(r with { x : 2 }).x", "on its domain"},
		{"X11: zero where a component has none",
			"set T = record { a ∈ 1..5 }\nzero(T).a", "set T = record { a ∈ 0..5 }\nzero(T).a", "has no zero"},
	} {
		wrap := func(b string) string {
			if strings.HasPrefix(b, "set ") {
				i := strings.Index(b, "\n")
				return b[:i+1] + "fn f() {\n" + b[i+1:] + "\n}\n"
			}
			return "fn f() {\n" + b + "\n}\n"
		}
		if _, err := normJ(t, wrap(c.ok), "f"); err != nil {
			t.Errorf("%s: the premise removed, the program is still refused: %v", c.name, err)
		}
		_, err := normJ(t, wrap(c.body), "f")
		if err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%s: %v, want a refusal saying %q", c.name, err, c.reason)
		}
	}
}

// THE BINDER DECIDES WHAT a.b IS (ADR 0128 item 3): a value is projected even
// where its name is an import's alias, and the fault that reads it as the
// import is refused.
func TestTheBinderDecidesAProjection(t *testing.T) {
	src := "fn f(io) { io.x + 1 }\nfn g() { f(record { x : 4 }) }\n"
	got, err := normJ(t, src, "g")
	if err != nil || got != "(fn () 5)" {
		t.Errorf("a bound name's projection: %s, %v", got, err)
	}
	PlantProjectionAsImport = true
	_, err = normJ(t, src, "g")
	PlantProjectionAsImport = false
	if err == nil {
		t.Errorf("a bound name's a.b read as an import still compiles: the rule's case cannot fail")
	}
}

// A RECORD IN A HEADER IS CURRIED (ADR 0128 item 7): the definition's
// parameters are its records' leaves and its result their product, the record
// view calls it, and every leaf's range is an obligation. The fault that drops
// a record's obligation must lose one.
func TestARecordInAHeaderIsCurried(t *testing.T) {
	src := "set P = record { y ∈ 0..9, x ∈ 0..9, n ∈ record { m ∈ 0..1 } }\n" +
		"fn swap(p ∈ P) (P) { p with { x : p.y, y : p.x } }\n" +
		"fn g(a ∈ 0..9) { q = swap(record { x : a, y : 3, n : record { m : 1 } })\nq.x * 10 + q.y }\n"
	p, err := loadJ(t, src)
	if err != nil {
		t.Fatal(err)
	}
	sig := p.Sigs["swap"]
	var names []string
	for _, pm := range sig.Params {
		names = append(names, pm.Name+"∈"+pm.Type)
	}
	if got := strings.Join(names, " "); got != "p#n#m∈int 0 1 p#x∈int 0 9 p#y∈int 0 9" {
		t.Errorf("the curried parameters are %s", got)
	}
	if strings.Join(sig.Results, ", ") != "int 0 1, int 0 9, int 0 9" {
		t.Errorf("the curried result is %v", sig.Results)
	}
	if p.Defs["swap#r"] == nil {
		t.Errorf("no record view swap#r")
	}
	e := placeEnv(p)
	e.Requires = map[string][]string{"swap": {"int 0 1", "int 0 9", "int 0 9"}}
	nf, err := Normalize(p.Defs["g"], e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	// q = swap(…) has q.x = 3 and q.y = a, each under its leaf's mark
	if s := nf.String(); !strings.Contains(s, `(+ (* (#req "swap" "p#y" "int 0 9" 3) 10) (#req "swap" "p#x" "int 0 9" a))`) {
		t.Errorf("g's normal form: %s", s)
	}
}

// A RECORD PARAMETER'S OBLIGATION IS ITS COMPONENTS' (splitRecordRequire), and
// a component the body never reads keeps its mark (keepClaims): the contract
// is the set's, not the body's reads. Each half has its fault.
func TestARecordsObligationIsEveryComponents(t *testing.T) {
	src := "fn f(p) { p.x }\nfn g(a, b) { f(record { x : a, y : b }) }\n"
	p, err := loadJ(t, src)
	if err != nil {
		t.Fatal(err)
	}
	reduce := func() string {
		e := placeEnv(p)
		e.Prim[RequireName], e.Pure[RequireName] = true, true
		nf, err := Normalize(App(Name(RequireName), Str("f"), Str("p"), Str("record{x: int 0 9; y: int 0 5}"),
			Name("#arg")), e, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		return nf.String()
	}
	s := reduce()
	if !strings.Contains(s, `"p.x" "int 0 9"`) || !strings.Contains(s, `"p.y" "int 0 5"`) {
		t.Errorf("a record's mark is not its components': %s", s)
	}
	PlantRecordRequireDropped = true
	s = reduce()
	PlantRecordRequireDropped = false
	if strings.Contains(s, `"p.y"`) {
		t.Errorf("the dropped-split fault keeps the mark: the case cannot fail: %s", s)
	}

	// The projection drops y and must keep its mark.
	e := placeEnv(p)
	e.Prim[RequireName], e.Pure[RequireName] = true, true
	proj := func() string {
		nf, err := Normalize(App(Name(GetName), Str("x"), App(Name(RequireName), Str("f"), Str("p"),
			Str("record{x: int 0 9; y: int 0 5}"), App(Name(RecName), Str("x y"), Name("a"), Name("b")))), e, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		return nf.String()
	}
	if s := proj(); !strings.Contains(s, "p.y") {
		t.Errorf("a projection dropped an unread component's obligation: %s", s)
	}
	PlantRecordClaimDropped = true
	s = proj()
	PlantRecordClaimDropped = false
	if strings.Contains(s, "p.y") {
		t.Errorf("the dropped-claim fault keeps the obligation: the case cannot fail: %s", s)
	}
}
