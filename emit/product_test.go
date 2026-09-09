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

// genFlat is genOn with the flattening pass in front, which is where the real
// pipeline has it: `cmd/gen` and `cmd/build` run it FIRST, before the checker,
// so that everything after — including the four backends — sees a flat table.
func genFlat(t *testing.T, src, name string) (string, error) {
	t.Helper()
	tg, err := LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(src)
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
	q := prog.Exports[0]
	nf, err := core.Normalize(prog.Defs[q], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out, sig, k, err := FlattenProducts(tg, prog.Sigs[q], nf)
	if err != nil {
		return "", err
	}
	if k == 0 {
		t.Errorf("nothing was flattened")
	}
	return Func(tg, name, sig, out)
}

// A PROJECTION IS AN UNCURRIED INDEX, and the element width comes through the
// components. `((sp w) 1)` is `sp[2w+1]`, and a pair of `(int 0 65535)` is a
// `[]uint16` — the type flattens with the term or the representation is lost.
func TestAProjectionBecomesAStridedIndex(t *testing.T) {
	got, err := genFlat(t, `
(use go)
(export f)
(sig f ((sp (array (array (int 0 65535) (int 0 65535))))) int
  (where (< (len sp) 1000)))
(def f (fn (sp)
  (loop ((acc 0) (w 0))
    (>= w (len sp))  acc
    else (again (+ acc (- ((sp w) 1) ((sp w) 0))) (+ w 1)))))`, "f")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"sp []uint16", "sp[((2 * w) + 1)]", "sp[(2 * w)]"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	// AND THE LENGTH IS IN ELEMENTS, NOT SLOTS. A program that asks how many
	// words there are must not be told how many offsets there are, and getting
	// this wrong is a loop that runs twice as long over half a table.
	if !strings.Contains(got, "len(sp) >> 1") && !strings.Contains(got, "(len(sp) / 2)") {
		t.Errorf("the length must be divided by the arity:\n%s", got)
	}
}

// CONSTRUCTION AND FIELD UPDATE, which are the two directions of the same
// surface: `(set b i (array v…))` writes a whole element and `(set (b i) j v)`
// writes one field — set applied to a projection, the exact dual of read
// applied to one.
func TestConstructionAndFieldUpdate(t *testing.T) {
	got, err := genFlat(t, `
(use go)
(export mk)
(sig mk ((n (int 1 100))) (array int))
(def mk (fn (n)
  (build n (fn (t)
    (loop ((t t) (i 0))
      (>= i n)  (set (t 0) 1 7)
      else      (again (set t i (array i 0)) (+ i 1)))))))`, "mk")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"make([]byte, (2 * n))", // the capacity is in slots
		"[(2 * i)] = byte(i)",   // field 0
		"[((2 * i) + 1)]",       // field 1
		"[1] = byte(7)",         // the field update
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// THE ARITY TRAVELS WITH THE BUFFER, and this is not an extra.
//
// A `build`'s binder is almost always handed straight to the `loop` that fills
// it, so the stores are to the LOOP's variable and not to the binder; and a
// helper returning a table becomes `(let (build …) (fn (sp) …))` once reduction
// has inlined it. Without propagation through both, the arity is known for
// exactly one term and lost immediately — which is every program anyone writes.
func TestTheArityTravelsThroughLoopAndLet(t *testing.T) {
	got, err := genFlat(t, `
(use go)
(export use)
(sig use ((n (int 1 50))) int)
(def mk (fn (n)
  (build n (fn (t)
    (loop ((t t) (i 0))
      (>= i n)  t
      else      (again (set t i (array i i)) (+ i 1)))))))
(def use (fn (n)
  (let (mk n) (fn (sp)
    (loop ((acc 0) (w 0))
      (>= w n)  acc
      else      (again (+ acc ((sp w) 1)) (+ w 1)))))))`, "use")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "(2 * w) + 1") && !strings.Contains(got, "((2 * w) + 1)") {
		t.Errorf("the arity did not reach the reader through the let:\n%s", got)
	}
}

// A BUFFER MAY NOT BE A FIELD, and a product may not contain a product.
//
// The first is ADR 0020 rule 6 for the reason it always was — an observation
// must not be able to extract an alias — and the second is this build's own
// boundary: flattening is what gives a product a representation, and a nested
// one would need the layout machinery products.md §8 puts last.
func TestWhatMayNotBeAField(t *testing.T) {
	for _, ty := range []string{
		"(array (buffer int) int)",
		"(array (array int int) int)",
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
	for _, ty := range []string{"(array (array int int))",
		"(array (array (int 0 255) int int))", "(buffer (array int int))"} {
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
(sig f ((sp (array (array (int 0 65535) (int 0 65535))))) int
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
			"(sig f ((p (array int int))) int)\n(def f (fn (p) 0))", "ELEMENT type"},
		{"a product result",
			"(sig f ((n int)) (array int int))\n(def f (fn (n) (array n n)))", "in the result"},
		{"a map of products",
			"(sig f ((m (map int (array int int)))) int)\n(def f (fn (m) 0))", "is not a type"},
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
	for _, ty := range []string{"(array (array int int))", "(buffer (array int int))"} {
		forms, err := core.Read("(sig f ((x " + ty + ")) int)\n(def f (fn (x) 0))")
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := core.LoadWith(forms, nil); err != nil {
			t.Errorf("%s must be a type: %v", ty, err)
		}
	}
}
