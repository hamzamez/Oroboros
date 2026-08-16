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
	got, err := Func(goTarget(t), "dot", nil, reduce(t, dotSrc, "go"))
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	t.Logf("\n%s", got)

	for _, want := range []string{
		"func Dot(p []float64, q []float64) float64",
		"for i := int64(0); i < n1; i++ {",
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
	_, err := Func(goTarget(t), "ms", nil, reduce(t, src, "go"))
	if err == nil {
		t.Fatal("expected a refusal for an escaping closure")
	}
	if !strings.Contains(err.Error(), "escaping closure") {
		t.Errorf("error should name the problem; got %v", err)
	}
}

// A `stmt` primitive's VALUE is its first argument, and returning that
// argument's expression wrote it twice — `fmt.Println((strings.Fields(s)))`
// followed by `return (strings.Fields(s))`. Two allocations for one source
// call, in a compiler whose call-by-need discipline exists to prevent exactly
// that. Found writing chapter 4.
func TestStatementValueIsNotRecomputed(t *testing.T) {
	tg := goTarget(t)
	nf := reduce(t, `
		(use io)
		(fn (s) (io.print-line (split-words s)))
	`, "go")
	code, err := Func(tg, "show", nil, nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if n := strings.Count(code, "strings.Fields"); n != 1 {
		t.Errorf("split-words emitted %d times, want 1:\n%s", n, code)
	}

	// But an argument that is already a name or a literal must NOT gain a
	// temporary — that would be noise in every print, and `report`'s
	// `fmt.Println(acc); return acc` is the hand-written shape.
	nf = reduce(t, `
		(use io)
		(use num/f64)
		(fn (a) (io.print-line (fold-range 0.0 (alen a) (fn (acc i) (f64.add acc (aindex a i))))))
	`, "go")
	code, err = Func(tg, "total", nil, nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if strings.Contains(code, "v1 :=") {
		t.Errorf("a name needs no temporary:\n%s", code)
	}
	if !strings.Contains(code, "fmt.Println(acc)") {
		t.Errorf("expected fmt.Println(acc):\n%s", code)
	}
}

func TestAtomicValue(t *testing.T) {
	for _, s := range []string{"acc", "n1", "_x", "42", "-3", `"hi"`, "$v"} {
		if !atomicValue(s) {
			t.Errorf("%q should be atomic", s)
		}
	}
	for _, s := range []string{"", "(n + n)", "strings.Fields(s)", "xs[i]", "a.b"} {
		if atomicValue(s) {
			t.Errorf("%q should not be atomic", s)
		}
	}
}

// The bound statement value needs its DECLARED type, not Go's default. A
// `v1 := (21 + 21)` is an `int`, and hello.oro's main returns `int64` — the
// literals were an untyped constant until they were bound, and binding them
// broke the build.
func TestBoundStatementValueKeepsItsType(t *testing.T) {
	tg := goTarget(t)
	nf := reduce(t, `
		(use io)
		(use num/int)
		(fn () (io.print-line (int.add 21 21)))
	`, "go")
	code, err := Func(tg, "main", nil, nf)
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !strings.Contains(code, "var v1 int64 = ") {
		t.Errorf("expected an explicitly typed temporary:\n%s", code)
	}
}

// A signature was CHECKED against the residual and then thrown away, so a
// program whose claim about it had been verified still failed to emit:
// `(sig f ((n int)) f64)` over `(fn (n) (fold-range 0.0 n …))` types nothing,
// because a loop bound is a structural primitive with no table entry.
func TestSignatureTypesTheParameters(t *testing.T) {
	tg := goTarget(t)
	nf := reduce(t, `
		(use num/f64)
		(fn (n) (fold-range 0.0 n (fn (acc i) (f64.add acc 1.0))))
	`, "go")

	if _, err := Func(tg, "total", nil, nf); err == nil {
		t.Fatal("without a signature the parameter is untypeable, so this must fail")
	}
	sig := &core.Sig{Params: []core.SigParam{{Name: "n", Type: "int"}}, Result: "f64"}
	code, err := Func(tg, "total", sig, nf)
	if err != nil {
		t.Fatalf("with a signature it must emit: %v", err)
	}
	if !strings.Contains(code, "func Total(n int64) float64") {
		t.Errorf("signature types not used:\n%s", code)
	}
}
