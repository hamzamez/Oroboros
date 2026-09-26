package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// THE PRODUCT, AND ITS REPRESENTATION.
//
// products.md derives the product as `Π` over a statically finite index set and
// its representation from one isomorphism, `Π_{I×J} V ≅ Π_I Π_J V` — currying.
// So the pass is one rewrite in four positions and nothing downstream learns
// that products exist, which is what these check: the emitted Go is what a
// hand-strided program has always emitted.

// A BUFFER MAY NOT BE A FIELD, and a product may not contain a product.
//
// The first is ADR 0020 rule 6 for the reason it always was — an observation
// must not be able to extract an alias — and the second is this build's own
// boundary: flattening is what gives a product a representation, and a nested
// one would need the layout machinery products.md §8 puts last.
func TestWhatMayNotBeAField(t *testing.T) {
	for _, ty := range []string{
		"(array (tuple (buffer int) int))",
		"(array (tuple (tuple int int) int))",
	} {
		forms, err := core.Read("(sig f ((x " + ty + ")) int)\n(def f (fn (x) 0))")
		if err == nil {
			_, _, err = core.LoadWith(forms, nil)
		}
		if err == nil {
			t.Errorf("%s was accepted as a product", ty)
		}
	}
	// The control: the same shapes with ordinary components must still be types.
	// AS AN ELEMENT, because a product is an element type and a signature is
	// where that is enforced — a bare product parameter has no width the caller
	// and callee could agree on, and is refused by name.
	for _, ty := range []string{"(array (tuple int int))",
		"(array (tuple (int 0 255) int int))", "(buffer (tuple int int))"} {
		forms, err := core.Read("(sig f ((x " + ty + ")) int)\n(def f (fn (x) 0))")
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := core.LoadWith(forms, nil); err != nil {
			t.Errorf("%s must be a type: %v", ty, err)
		}
	}
}

// A HETEROGENEOUS PRODUCT IS A TYPE AND HAS NO REPRESENTATION HERE, and saying
// so by name is better than flattening it into a slot that cannot hold both.
// `(array int string)` type-checks; `prodTable` declines it, so nothing is
// flattened and the program is refused where it tries to exist.
func TestAHeterogeneousProductHasNoFlatRepresentation(t *testing.T) {
	if _, _, ok := prodTable("array prod(int, string)"); ok {
		t.Error("a product of an int and a string has no flat representation")
	}
	if k, elem, ok := prodTable("array prod(int 0 255, int 0 65535)"); !ok || k != 2 ||
		elem != "int 0 65535" {
		t.Errorf("a product of two ranges joins them: k=%d elem=%q ok=%v", k, elem, ok)
	}
	// THROUGH IntRange AND NOT ValueType, which is the trap scalarrange-2026-08-31
	// names: `ValueType` normalises a range to `int` because that is the TYPING
	// answer, and this wants the REPRESENTATION one. Getting it wrong gives
	// `array int` where `array int 0 65535` was meant, and every operation
	// reading one goes unbounded.
	if _, elem, _ := prodTable("array prod(int 0 65535, int 0 65535)"); elem == "int" {
		t.Error("the components' range was normalised away")
	}
}

// THE COMPILER PROVES THE STRIDE IT GENERATES, and that is the claim
// products.md §11 says to make falsifiable.
//
// `((sp w) 1)` becomes `(sp (+ (* 2 w) 1))` guarded by `w < (len sp)/2`, and
// discharging it needs two facts the fragment did not have: a QUOTIENT is an
// atom (a division was outside the fragment entirely, not merely opaque), and
// `k·(x/k) <= x` relates that atom to the length it came from. Then one
// Fourier–Motzkin step combines them — `2·(w < q) + 1·(2q <= len)` — which is
// the principled form of the single Farkas multiplier json-tree-bench added.
//
// No clamp appears in the source. If any of the three is missing the obligation
// is reported instead of discharged, which is what this asserts.
func TestTheStrideIsProvenFromTheGuard(t *testing.T) {
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(`
(use go)
(export f)
(sig f ((sp (array (tuple (int 0 65535) (int 0 65535))))) int
  (where (< (len sp) 1000)))
(def f (fn (sp)
  (loop ((acc 0) (w 0))
    (>= w (len sp))  acc
    else (again (+ acc (- ((sp w) 1) ((sp w) 0))) (+ w 1)))))`)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["f"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, sig, k, err := FlattenProducts(tg, prog.Sigs["f"], nf)
	if err != nil || k == 0 {
		t.Fatalf("flatten: %d %v", k, err)
	}
	notes, err := Refine(tg, "f", sig, out)
	if err != nil {
		t.Fatalf("the generated stride must be PROVEN, not reported: %v", err)
	}
	for _, n := range notes {
		if strings.Contains(n, "propagated") {
			t.Errorf("propagated rather than proven: %s", n)
		}
	}
}

// A PRODUCT IS AN ELEMENT TYPE, AND A SIGNATURE IS WHERE THAT IS ENFORCED.
//
// All three of these were ACCEPTED until docs/spec/products.md was written, and
// writing it is what found them — which is the argument for the rule that a
// language addition needs a specification saying what each target does with it.
// A map of products emitted `map[int]/*prod(int, int)?*/`, a Go type that does
// not exist and that a host typing nothing would have taken silently; a product
// result emitted `[]int{n, n}`, which happens to be the flat form and happens to
// be right, and "happens to" is not a specification.
func TestAProductIsAnElementTypeAndNothingElse(t *testing.T) {
	for _, tc := range []struct{ what, src, want string }{
		{"a bare product parameter",
			"(sig f ((p (tuple int int))) int)\n(def f (fn (p) 0))", "ELEMENT type"},
		// A tuple RESULT is several results now (spec/data.md §3.3), so what stays
		// refused in that position is a tuple inside one.
		{"a tuple inside a tuple result",
			"(sig f ((n int)) (tuple (tuple int int) int))\n(def f (fn (n) 0))", "is not a type"},
		{"a map of products",
			"(sig f ((m (map int (tuple int int)))) int)\n(def f (fn (m) 0))", "is not a type"},
	} {
		forms, err := core.Read(tc.src)
		if err == nil {
			_, _, err = core.LoadWith(forms, nil)
		}
		if err == nil {
			t.Errorf("%s was accepted", tc.what)
			continue
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: the message should say %q, got %v", tc.what, tc.want, err)
		}
	}
	// The control: as an ELEMENT all three shapes are types, so the refusals
	// above are about where a product may stand and not about products.
	for _, ty := range []string{"(array (tuple int int))", "(buffer (tuple int int))"} {
		forms, err := core.Read("(sig f ((x " + ty + ")) int)\n(def f (fn (x) 0))")
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := core.LoadWith(forms, nil); err != nil {
			t.Errorf("%s must be a type: %v", ty, err)
		}
	}
}
