package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

// A READ OF THE ENVIRONMENT STAYS BEFORE A WRITE OF IT (lib/os/go.oro). The
// portable `os.Getenv` and Go's own `go/os.Setenv` may be loaded together, and
// Getenv reads what Setenv writes. Declared pure, the read bound before the
// store was substituted after it, so a program meaning the old value got the
// new one: the at-map reordering (gotarget-2026-09-30 §13) at a library name.
func TestAnEnvironmentReadStaysBeforeASetenv(t *testing.T) {
	tg, err := emit.LoadTargetLayers("go", []string{"../targets"}, []string{"../lib"})
	if err != nil {
		t.Fatal(err)
	}
	forms, err := core.Read(`(use os) (use go/os as gos)
(fn (k) (let x (os.Getenv k) (seq (ignore (gos.Setenv k "v")) x)))`)
	if err != nil {
		t.Fatal(err)
	}
	prog, terms, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	nf, err := core.Normalize(terms[0], env, core.DefaultFuel)
	if err != nil {
		t.Fatal(err)
	}
	out := nf.String()
	read, store := strings.Index(out, "host.Getenv"), strings.Index(out, "Setenv")
	if read < 0 || store < 0 || read > store {
		t.Errorf("the read must stay before the store:\n%s", out)
	}
}
