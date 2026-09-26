package emit_test

import (
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir/golang"
)

// A LOOP WHOSE VALUE IS DISCARDED STILL DECLARES ITS RESULT. `seq` is a β-redex
// with an unused binder (effects.md §5), and the Go emitter skipped the `_ = v`
// it writes for a discarded value whenever that value was a bare name — right
// for a parameter or a literal, wrong for the temporary a loop's result lands
// in, which Go refuses as declared and not used. No program had put a loop
// before another statement until unicode/utf8.oro (gostd-utf8-2026-09-13).
func TestADiscardedLoopResultIsRead(t *testing.T) {
	src := `
		(use go/fmt as fmt)
		(def main (fn (n)
		  (seq (loop ((j 0)) (>= j n) (fmt.Println j) else (again (+ j 1)))
		       7)))`
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := core.Load(forms)
	if err != nil {
		t.Fatal(err)
	}
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(prog.Defs["main"], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	emit.Imports = map[string]bool{}
	code, err := golang.FromResidual(tg, "f", &core.Sig{Params: []core.SigParam{{Name: "n", Type: "int"}}, Result: "int"}, nf)
	if err != nil {
		t.Fatal(err)
	}
	// Go itself is the judge of "declared and not used", which is what the
	// term backend got wrong; its IR printer declares only what is read (L7).
	goBuilds(t, map[string]string{"f": code})
}
