package emit

import (
	"testing"

	"oroboros/core"
)

// These tests exercise the PORTABLE layer — num/f64, fold-range, io — which now
// lives in targets/portable-go.oro. targets/go/ is the target-native one and
// declares none of it (docs/spec/target-native.md).
func goTarget(t *testing.T) *Target {
	t.Helper()
	tg, err := LoadTarget("../targets/portable-go.oro")
	if err != nil {
		t.Fatalf("load target: %v", err)
	}
	return tg
}

func reduce(t *testing.T, src, target string) *core.Term {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	prog, terms, err := core.Load(forms)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	tg, err := LoadTarget("../targets/" + target + ".oro")
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	out, err := core.Normalize(terms[0], env, core.DefaultFuel)
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
