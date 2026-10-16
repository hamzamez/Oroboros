package emit

import (
	"testing"

	"oroboros/core"
)

// reduce normalizes the definition t of a program in J on target.
func reduce(t *testing.T, src, target string) *core.Term {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	tg, err := LoadTarget("../targets/" + target + ".oro")
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	t0, ok := prog.Defs["t"]
	if !ok {
		t.Fatalf("the program defines no t")
	}
	out, err := core.Normalize(t0, env, core.DefaultFuel)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return out
}

func TestMangle(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"dot", "dot"},
		{"push-filter", "pushFilter"},
		{"empty?", "emptyP"},
		{"set!", "setB"},
		{"range", "range_"},
		{"fold-range", "foldRange"},
	} {
		if got := mangle(c.in); got != c.want {
			t.Errorf("mangle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
