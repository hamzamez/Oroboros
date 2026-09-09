package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A HOST CALL MAY GIVE BACK SEVERAL RESULTS, and the language needed nothing.
//
// `Prim.Result` was one string, and that field refused 19.8% of Go's callable
// standard library — more than every language-level limitation put together,
// and compounding, because `(T, error)` is Go's CONSTRUCTOR idiom, so every
// name it blocked also blocked every method on the type that name would have
// returned (gostdlib-2026-09-06 §4a). It is why this language had no file I/O.
//
// The elimination form already existed: `((f x) (fn (a b) …))` is how the
// negative product is consumed (values.md), and `values-product.oro` has been a
// differential case since August. With a `def` producer, β performs that
// application and the product vanishes. With a `prim` producer β cannot — the
// operator of the outer application is itself an application — so the redex is
// stuck and the shape arrives at the backend, which emits the host's own form.
func TestAPrimMayGiveBackSeveralResults(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	p, ok := tg.Prims["go/os.ReadFile"]
	if !ok {
		t.Fatal("targets/go declares no go/os.ReadFile")
	}
	if len(p.Results) != 2 {
		t.Fatalf("ReadFile has %d declared results, want 2: %v", len(p.Results), p.Results)
	}
	// A single COMPOUND result must not be mistaken for several. `(array int)`
	// and `(int 0 255)` are both an application of names, and only `TypeName`
	// knows which constructors exist — which is why `resultList` consults it
	// first rather than counting kids.
	for _, n := range []string{"go/os.WriteFile", "go/os.Args"} {
		q := tg.Prims[n]
		if len(q.Results) != 0 {
			t.Errorf("%s: %q was read as %d results; a compound type is ONE result",
				n, q.Result, len(q.Results))
		}
	}
}

// AND THE EMITTED FORM IS THE HOST'S OWN.
func TestSeveralResultsEmitTheHostsOwnForm(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	src := `(fn () ((go/os.ReadFile "f") (fn (src err)
	           (if (go/os.err-nil err) (go.len src) 0))))`
	terms, err := core.ReadAll(src)
	if err != nil || len(terms) != 1 {
		t.Fatalf("read: %v", err)
	}
	out, err := Func(tg, "gen-read", nil, terms[0])
	if err != nil {
		t.Fatal(err)
	}
	// One assignment, both names, no product built.
	if !strings.Contains(out, "src, err := os.ReadFile(\"f\")") {
		t.Fatalf("expected Go's own multiple assignment:\n%s", out)
	}
	if strings.Contains(out, "struct") || strings.Contains(out, "[2]") {
		t.Errorf("a product was built where the host has several results:\n%s", out)
	}
}

// A RESULT THE BODY NEVER READS is still received, because Go's multiple
// assignment is positional — and as `_`, because an unused variable is a
// compile error on that host.
func TestAnUnreadResultBecomesBlank(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	terms, err := core.ReadAll(
		`(fn () ((go/os.ReadFile "f") (fn (src err) (go.len src))))`)
	if err != nil {
		t.Fatal(err)
	}
	out, err := Func(tg, "gen-read", nil, terms[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "src, _ := os.ReadFile") {
		t.Fatalf("the unread result must be `_`, or Go refuses the file:\n%s", out)
	}
}

// AND A DECLARED RESULT RANGE IS READ BY THE INTERVAL LAYER.
//
// The other half of the same problem, and it was measured on two independent
// ecosystems before it was built: `emit/interval.go` returned ⊤ for an
// application of any primitive it did not structurally recognise, so ADR 0019's
// bounded-by-default refused arithmetic on EVERY host result in EVERY ecosystem
// and `-checked` was the only way through (gostdlib §4b, win32 §5).
//
// A primitive has no body, so a declaration is the only source there can be —
// the same reason `ensures` belongs on a `prim` and is redundant on an internal
// definition.
func TestADeclaredResultRangeIsRead(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	// Two primitives, identical but for the declared result.
	wide := Prim{Name: "h.wide", Args: []string{"int"}, Result: "int", Kind: "expr", Form: "h(%s)"}
	narrow := Prim{Name: "h.narrow", Args: []string{"int"}, Result: "int 0 64", Kind: "expr", Form: "h(%s)"}
	tg.Prims[wide.Name], tg.Prims[narrow.Name] = wide, narrow
	tg.Names = append(tg.Names, wide.Name, narrow.Name)

	for _, c := range []struct {
		name  string
		bound bool
	}{{"h.wide", false}, {"h.narrow", true}} {
		terms, err := core.ReadAll(`(fn (n) (go.+ (` + c.name + ` n) 1))`)
		if err != nil {
			t.Fatal(err)
		}
		rep, _ := Intervals(tg, nil, terms[0], 0)
		got := rep.Proven == rep.Ops && rep.Ops > 0
		if got != c.bound {
			t.Errorf("%s: %d of %d operations bounded; want bounded=%v.\n"+
				"  A declared result range is the only fact a primitive can offer,\n"+
				"  and without it every host call in every ecosystem is unprovable.",
				c.name, rep.Proven, rep.Ops, c.bound)
		}
	}
}
