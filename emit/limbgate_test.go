package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A LIMB-ONLY TARGET MUST PROPAGATE SUPPLY THROUGH A LOOP'S INITIALISER.
//
// `bigOK()` is `limbs || HasBig()`, and the loop's representation fixpoint was
// additionally gated on `HasBig()` — so it never ran on the one kind of target
// the limb rung exists for. windows therefore left
//
//	(loop ((v x)) (= v 0) k else (again (/ v 100000000) …))
//
// with `v` a machine word although `x` is a declared arbitrary-precision
// parameter, and the checker refused the guard by name:
// *"v is array int, but int is required here"*.
//
// biglimb.go warns about this in as many words — every gate asks `bigOK` rather
// than `HasBig`, or a target with nothing to fall back to promotes nothing and
// is then refused for producing a word — and the comment beside the gate already
// named decimal rendering as the program the rule was written for. It was
// written for it and switched off on the only target that could not do without
// it. Found by `examples/big/render.oro` refusing to build for windows once
// `concat` and `string-of` existed there.
func TestALimbOnlyTargetPromotesThroughALoopInitialiser(t *testing.T) {
	src := `
(export f)
(sig f ((x (int 0 (pow 2 200)))) int)
(def f (fn (x)
  (loop ((v x) (k 0))
    (= v 0)   k
    (>= k 8)  k
    else (again (/ v 100000000) (+ k 1)))))`

	// The SAME program on a target with a host bignum forced onto the limb rung,
	// and on a target that has no host bignum at all. They must agree: the
	// storage is the same, so the promotion must be too.
	for _, c := range []struct{ dir, repr string }{
		{"../targets/go", "limbs"},
		{"../targets/windows", ""},
	} {
		tg, err := LoadTarget(c.dir)
		if err != nil {
			t.Fatal(err)
		}
		if c.repr != "" {
			tg.BigRepr = c.repr
		}
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		prog, _, err := core.LoadWith(forms, nil)
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
		var all []*core.Sig
		for _, s := range prog.Sigs {
			all = append(all, s)
		}
		out, n, err := PromoteBig(tg, prog.Sigs[q], nf, all...)
		if err != nil {
			t.Fatalf("%s: %v", c.dir, err)
		}
		if n == 0 {
			t.Fatalf("%s: nothing was promoted. `x` is a declared "+
				"arbitrary-precision parameter and `v` is initialised from it, so "+
				"supply must reach the loop variable — otherwise the guard `(= v 0)` "+
				"compares a limb table against an integer and the program is refused.",
				c.dir)
		}
		// And the guard must be the limb library's equality, not the language's.
		if s := out.String(); strings.Contains(s, "(= v") {
			t.Errorf("%s: a bare `=` survives on the big loop variable:\n%s", c.dir, s)
		}
	}
}
