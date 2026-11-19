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
	if got, err := normJ(t, whole, "f"); err != nil || got != `(fn () (fn (#k) (#k 5 2)))` {
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
		{"an override writing the wrong label", &PlantWithWrongLabel, whole, `(fn () (fn (#k) (#k 5 2)))`},
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
	// the reducer's split alone: the free name #arg has no shape to place
	PlantRecordsUnerased = true
	defer func() { PlantRecordsUnerased = false }()
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

// A RECORD STORED AS DATA IS PLACED BY ITS SHAPE (recerase.go, data.md §4.7):
// a buffer's element labels are those of the records stored into it, a least
// fixed point through its versions, so b[i].y is b[i]'s leaf at y's position.
// (A table given by a rule is copied to each read, β-tab, so only a store
// makes a record data.) The fault that leaves the residual's records unplaced
// keeps a projection no target has.
func TestARecordStoredAsDataIsPlacedByItsShape(t *testing.T) {
	src := "fn f(n) {\nvar b := array { record { x : 0, y : 0 } : _ ∈ 0..n }\n" +
		"loop k = 0 {\nk <= n -> { b[k] := record { y : k * 2, x : k + 1 }; again(k + 1) }\nelse -> b[n].y - b[n].x\n}\n}\n"
	p, err := loadJ(t, src)
	if err != nil {
		t.Fatal(err)
	}
	reduce := func() string {
		e := placeEnv(p)
		for _, n := range []string{"table", "array", "set", "build"} {
			e.Prim[n], e.Pure[n] = true, true
		}
		nf, err := Normalize(p.Defs["f"], e, DefaultFuel)
		if err != nil {
			t.Fatal(err)
		}
		return nf.String()
	}
	if s := reduce(); containsAny(s, GetName, RecName, WithName) {
		t.Errorf("a table of records keeps a record's form: %s", s)
	}
	PlantRecordsUnerased = true
	s := reduce()
	PlantRecordsUnerased = false
	if !containsAny(s, GetName) {
		t.Errorf("the unerased fault leaves no projection: the case cannot fail: %s", s)
	}
}

func containsAny(s string, ws ...string) bool {
	for _, w := range ws {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// A CELL IS A MODE, NOT A SET (ADR 0135): a set ranges over values, and a cell
// or a buffer is a place, so a binder writes the value's set and the mode,
// `x ∈ T inout` and `b ∈ Array(V) linear`. The core's formers written at a
// binder are refused, naming the mode; the same declaration with the mode
// reads, so each refusal's premise is the former. The fault: the formers read
// again, as before.
func TestACellIsAModeNotASet(t *testing.T) {
	for _, c := range []struct{ bad, good, reason string }{
		{"fn f(s ∈ cell(0..9)) (Int) { s }\n", "fn f(s ∈ 0..9 inout) (Int) { s }\n", "`x ∈ T inout`, not `cell(T)`"},
		{"fn f(b ∈ buffer(0..255)) (Int) { 0 }\n", "fn f(b ∈ Array(0..255) linear) (Int) { 0 }\n", "`b ∈ Array(V) linear`, not `buffer(V)`"},
	} {
		if _, err := Read(c.bad); err == nil || !strings.Contains(err.Error(), c.reason) {
			t.Errorf("%q: want it refused naming %q, got %v", c.bad, c.reason, err)
		}
		if _, err := Read(c.good); err != nil {
			t.Errorf("%q: the mode's spelling should read, got %v", c.good, err)
		}
		PlantCoreFormerAccepted = true
		_, err := Read(c.bad)
		PlantCoreFormerAccepted = false
		if err != nil {
			t.Errorf("with the formers accepted %q should read, so the case reaches the rule; got %v", c.bad, err)
		}
	}
}

// A LENT BUFFER HANDED BACK IS A PRIMED RESULT (ADR 0140): `(b' ∈ Array(V)
// linear)`, b a `linear` parameter, reads as the core's (buffer V) result, the
// term `(buffer(V))` gave; `buffer(V)` at a result is refused naming the primed
// form; a prime naming no linear parameter is refused; an unprimed linear
// result is still ADR 0084's moved-out region.
func TestALentBufferHandedBackIsAPrimedResult(t *testing.T) {
	primed, err := Read("fn f(b ∈ Array(0..9) linear) (b' ∈ Array(0..9) linear) { b }\n")
	if err != nil {
		t.Fatalf("the primed result should read: %v", err)
	}
	PlantCoreFormerAccepted = true
	old, err := Read("fn f(b ∈ Array(0..9) linear) (buffer(0..9)) { b }\n")
	PlantCoreFormerAccepted = false
	if err != nil {
		t.Fatalf("the old spelling, with the formers accepted: %v", err)
	}
	if a, b := primed[0].Sig.Result, old[0].Sig.Result; a != b {
		t.Errorf("the primed result is %q and the old one %q: they should be one term", a, b)
	}
	if _, err := Read("fn f(b ∈ Array(0..9) linear) (buffer(0..9)) { b }\n"); err == nil || !strings.Contains(err.Error(), "ADR 0140") {
		t.Errorf("buffer(V) at a result should be refused naming the primed form, got %v", err)
	}
	if _, err := Read("fn f(b ∈ Array(0..9) linear) (c' ∈ Array(0..9) linear) { b }\n"); err == nil || !strings.Contains(err.Error(), "no `linear` parameter") {
		t.Errorf("a prime naming no linear parameter should be refused, got %v", err)
	}
	moved, err := Read("fn f(n ∈ 0..9) (r ∈ Array(0..9) linear) { var r := array { 0 : _ ∈ 0..n }; r }\n")
	if err != nil {
		t.Fatalf("an unprimed linear result should read: %v", err)
	}
	if moved[0].Sig.Result == old[0].Sig.Result {
		t.Errorf("an unprimed linear result should stay the moved-out region, not the lent buffer: %q", moved[0].Sig.Result)
	}
}
