package emit

import (
	"strings"
	"testing"

	"oroboros/core"
)

// reduce runs a source program to normal form and returns the single term.
func goTarget(t *testing.T) *Target {
	t.Helper()
	tg, err := LoadTarget("../targets/go.oro")
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

const dotSrc = `
	(use num/f64)

	(def vec      (fn (n f) (fn (sel) (sel n f))))
	(def vlen     (fn (v)   (v (fn (n f) n))))
	(def vindex   (fn (v i) ((v (fn (n f) f)) i)))
	(def of-array (fn (a)   (vec (alen a) (fn (i) (aindex a i)))))

	(def zip (fn (g a b) (vec (vlen a) (fn (i) (g (vindex a i) (vindex b i))))))
	(def sum (fn (v)     (fold-range 0.0 (vlen v) (fn (acc i) (f64.add acc (vindex v i))))))
	(def dot (fn (a b)   (sum (zip f64.mul (of-array a) (of-array b)))))

	(fn (p q) (dot p q))
`

func TestEmitDot(t *testing.T) {
	got, err := Func(goTarget(t), "dot", reduce(t, dotSrc, "go"))
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	t.Logf("\n%s", got)

	for _, want := range []string{
		"func Dot(p []float64, q []float64) float64",
		"for i := 0; i < n1; i++ {",
		"return acc",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("emitted Go is missing %q", want)
		}
	}
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

// An escaping closure has no emission yet, and the error should say so rather
// than producing Go that does not compile.
func TestEscapingClosureIsARefusal(t *testing.T) {
	src := `
		(use num/f64)
		(def make-scaler (fn (f) (fn (v) (f64.mul v f))))
		(fn (k) (make-scaler k))
	`
	_, err := Func(goTarget(t), "ms", reduce(t, src, "go"))
	if err == nil {
		t.Fatal("expected a refusal for an escaping closure")
	}
	if !strings.Contains(err.Error(), "escaping closure") {
		t.Errorf("error should name the problem; got %v", err)
	}
}
