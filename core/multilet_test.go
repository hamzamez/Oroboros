package core

import (
	"strings"
	"testing"
)

// THE N-ARY LET. A host call with several results, applied to its continuation,
// is a let binding each result, and an eliminator applied to it commutes inside:
//
//	(((p a…) (fn (x̄) M)) k…)  ⟶  ((p a…) (fn (x̄) (M k…)))
//
// Without the rule a function returning a tuple BUILT from such a call cannot be
// taken apart by its caller: the tuple stays a closure under the host call's
// continuation, and the caller's projection is applied to the whole call
// (u128-2026-09-23). The witness is exactly that: `swap` builds its tuple under
// `go.mul`'s continuation, `f` projects the first component.
func TestAnEliminatorCommutesIntoAHostCallsContinuation(t *testing.T) {
	p, err := loadSrc(t, `
		(use go)
		(def swap (fn (x) (let (tuple h l) (go.mul x 3) (tuple l h))))
		(def f (fn (x) (go.+ ((swap x) (fn (a b) a)) 1)))`)
	if err != nil {
		t.Fatal(err)
	}
	e := testEnv(p, "go.+", "!go.mul", "if", "=")
	nf, err := Normalize(p.Defs["f"], e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	got := nf.String()
	// The projection meets the tuple under the binders, so what is left is the
	// host call and a continuation that selects `l`, its SECOND result: a
	// projection, the shape every backend's several-results path emits
	// (values.md). No closure survives: no lambda is applied, and none is an
	// argument except the continuation itself.
	want := "(fn (x) (go.+ ((go.mul x 3) (fn (#nl1 #nl2) #nl2)) 1))"
	if got != want {
		t.Errorf("the eliminator must commute into the continuation:\n got  %s\n want %s", got, want)
	}
}

// The side condition is ADR 0010's: an eliminator that is not pure is not moved,
// because moving it under the host call would reorder its effect after the call's.
func TestAnImpureEliminatorStaysOutside(t *testing.T) {
	p, err := loadSrc(t, `
		(use go)
		(def f (fn (x) (((go.mul x 3) (fn (h l) (fn (k) (k h l)))) (go.eff x))))`)
	if err != nil {
		t.Fatal(err)
	}
	e := testEnv(p, "!go.mul", "!go.eff", "if", "=")
	nf, err := Normalize(p.Defs["f"], e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(nf.String(), "#nl") {
		t.Errorf("an impure argument must not be moved under the host call: %s", nf)
	}
}

// THE CONVERSION'S BINDERS ARE FRESH FOR WHAT THEY CLOSE OVER. A residual
// carries its binders' spellings into the next reduction, whose counter starts
// again, so an eliminator argument can mention a `#nl` name free. Built by hand,
// since no program can spell one: the free `#nl1` must stay free.
func TestTheConversionDoesNotCapture(t *testing.T) {
	p, err := loadSrc(t, `(use go) (def f (fn (x) x))`)
	if err != nil {
		t.Fatal(err)
	}
	e := testEnv(p, "go.+", "!go.mul", "if", "=")
	host := App(App(Name("go.mul"), Name("x"), Int(3)),
		Fn([]string{"h", "l"}, Fn([]string{"k"}, App(Name("k"), Name("h"), Name("l")))))
	elim := Fn([]string{"a", "b"}, App(Name("go.+"), Name("a"), Name("#nl1")))
	nf, err := Normalize(Fn([]string{"x"}, App(host, elim)), e, DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	if !freeVars(nf)["#nl1"] {
		t.Errorf("the free #nl1 was captured by a binder the conversion introduced: %s", nf)
	}
	if !strings.Contains(nf.String(), "#nl2") {
		t.Errorf("the conversion must have fired, skipping the name in use: %s", nf)
	}
}
