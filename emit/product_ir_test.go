package emit_test

import (
	"regexp"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
)

// genFlat is genOn with the flattening pass in front, which is where the real
// pipeline has it: `cmd/gen` and `cmd/build` run it FIRST, before the checker,
// so that everything after — including the four backends — sees a flat table.
func genFlat(t *testing.T, src, name string) (string, error) {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/go")
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
	out, sig, k, err := emit.FlattenProducts(tg, prog.Sigs[q], nf)
	if err != nil {
		return "", err
	}
	if k == 0 {
		t.Errorf("nothing was flattened")
	}
	return golang.FromResidual(tg, name, sig, out)
}

// A PROJECTION IS AN UNCURRIED INDEX, and the element width comes through the
// components. `((sp w) 1)` is `sp[2w+1]`, and a pair of `(int 0 65535)` is a
// `[]uint16` — the type flattens with the term or the representation is lost.
func TestAProjectionBecomesAStridedIndex(t *testing.T) {
	got, err := genFlat(t, `
(use go)
(export f)
(sig f ((sp (array (tuple (int 0 65535) (int 0 65535))))) int
  (where (< (len sp) 1000)))
(def f (fn (sp)
  (loop ((acc 0) (w 0))
    (>= w (len sp))  acc
    else (again (+ acc (- ((sp w) 1) ((sp w) 0))) (+ w 1)))))`, "f")
	if err != nil {
		t.Fatal(err)
	}
	// The IR numbers its values, so the stride is checked by shape: a uint16
	// slice read at 2w and at 2w + 1.
	for _, want := range []string{`func F\(v\d+ \[\]uint16\)`, `\(2 \* v\d+\)`, `\(v\d+ \+ 1\)`} {
		if !regexp.MustCompile(want).MatchString(got) {
			t.Errorf("missing %s:\n%s", want, got)
		}
	}
	// AND THE LENGTH IS IN ELEMENTS, NOT SLOTS. A program that asks how many
	// words there are must not be told how many offsets there are, and getting
	// this wrong is a loop that runs twice as long over half a table.
	if !regexp.MustCompile(`\(v\d+ / 2\)|\(v\d+ >> 1\)`).MatchString(got) {
		t.Errorf("the length must be divided by the arity:\n%s", got)
	}
}

// CONSTRUCTION AND FIELD UPDATE, which are the two directions of the same
// surface: `(set b i (tuple v…))` writes a whole element and `(set (b i) j v)`
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
      else      (again (set t i (tuple i 0)) (+ i 1)))))))`, "mk")
	if err != nil {
		t.Fatal(err)
	}
	// The declared `(array int)` result is a boundary and fixes the word
	// (Theorem D′); the term backend narrowed it to byte with its contents.
	// The capacity is in slots, the fields are at 2i and 2i + 1, and the
	// field update writes slot 1.
	for _, want := range []string{
		`\(2 \* v\d+\)`,
		`make\(\[\]int, v\d+\)`,
		`\(v\d+ \+ 1\)`,
		`\[1\] = 7`,
	} {
		if !regexp.MustCompile(want).MatchString(got) {
			t.Errorf("missing %s:\n%s", want, got)
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
      else      (again (set t i (tuple i i)) (+ i 1)))))))
(def use (fn (n)
  (let sp (mk n)
    (loop ((acc 0) (w 0))
      (>= w n)  acc
      else      (again (+ acc ((sp w) 1)) (+ w 1))))))`, "use")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`\(2 \* v\d+\)`).MatchString(got) || !regexp.MustCompile(`\(v\d+ \+ 1\)`).MatchString(got) {
		t.Errorf("the arity did not reach the reader through the let:\n%s", got)
	}
}
