package emit

import (
	"strings"
	"testing"

	"oroboros/core"
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
	tg, err := LoadTarget("../targets/go")
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
	code, err := Func(tg, "f", &core.Sig{Params: []core.SigParam{{Name: "n", Type: "int"}}, Result: "int"}, nf)
	if err != nil {
		t.Fatal(err)
	}
	// Every declared result temporary must be read somewhere other than the
	// line that assigns it.
	for _, line := range strings.Split(code, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || f[0] != "var" || !strings.HasPrefix(f[1], "r") {
			continue
		}
		name := f[1]
		reads := 0
		for _, l := range strings.Split(code, "\n") {
			l = strings.TrimSpace(l)
			if strings.Contains(l, name) && !strings.HasPrefix(l, "var "+name) && !strings.HasPrefix(l, name+" = ") {
				reads++
			}
		}
		if reads == 0 {
			t.Errorf("%s is declared and never read, which Go refuses:\n%s", name, code)
		}
	}
	if !strings.Contains(code, "var r") {
		t.Fatalf("the loop was not emitted as a loop with a result, so this test checks nothing:\n%s", code)
	}
}
