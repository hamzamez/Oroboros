package emit_test

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
)

func winTarget(t *testing.T) *emit.Target {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/windows")
	if err != nil {
		t.Fatalf("load windows target: %v", err)
	}
	return tg
}

// between returns the emitted lines from the first label starting with `from`
// to the first starting with `to`.
func between(code, from, to string) string {
	lines := strings.Split(code, "\n")
	start := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if start < 0 && strings.HasPrefix(t, from) && strings.HasSuffix(t, ":") {
			start = i + 1
		} else if start >= 0 && strings.HasPrefix(t, to) && strings.HasSuffix(t, ":") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return ""
}

func allProgSigs(p *core.Program) []*core.Sig {
	out := make([]*core.Sig, 0, len(p.Sigs))
	for _, s := range p.Sigs {
		out = append(out, s)
	}
	return out
}

// These tests exercise the PORTABLE layer — num/f64, fold-range, io — which now
// lives in targets/portable-go.oro. targets/go/ is the target-native one and
// declares none of it (docs/spec/target-native.md).
func goTarget(t *testing.T) *emit.Target {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/portable-go.oro")
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
	tg, err := emit.LoadTarget("../targets/" + target + ".oro")
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

// goNative is the NATIVE Go target — targets/go/, not the portable layer.
// goTarget loads portable-go.oro, whose primitives are not the `go.` names
// these tests use, and a target that does not know a name raises no obligation
// for it: the first draft of these tests passed vacuously against it.
func goNative(t *testing.T) *emit.Target {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatalf("load native go target: %v", err)
	}
	return tg
}

// litTarget declares one host call over a byte table, as `hex.Encode` does —
// on the host asked for, because the two hosts spell that byte table
// differently and that difference is the point.
func litTarget(t *testing.T, host string) *emit.Target {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/" + host)
	if err != nil {
		t.Fatal(err)
	}
	form := "len(%s)"
	if host == "java" {
		form = "%s.length"
	}
	tg.Prims["x.take"] = emit.Prim{Name: "x.take", Args: []string{"array int 0 255"},
		Result: "int", Kind: "expr", Form: form, Pure: true}
	return tg
}

// lit builds `(array e…)`.
func lit(vs ...int64) *core.Term {
	kids := []*core.Term{core.Name("array")}
	for _, v := range vs {
		kids = append(kids, core.Int(v))
	}
	return &core.Term{Kind: core.KApp, Kids: kids}
}

// takes builds the two shapes a literal reaches a declared parameter through:
// straight into the call, and bound by a `let` the body then hands over.
func takes(l *core.Term) map[string]*core.Term {
	return map[string]*core.Term{
		"argument": core.Fn(nil, core.App(core.Name("x.take"), l)),
		"let-bound": core.Fn(nil, core.App(core.Name("let"), l,
			core.Fn([]string{"d"}, core.App(core.Name("x.take"), core.Name("d"))))),
	}
}

// mapSrc is the program these tests emit. It goes through the REAL pipeline —
// read, load, reduce — because the emitter never sees source: reduction turns
// `case` into the Church eliminator applied to the read, and
// `((m k) (fn (#t #p) …))` is the shape the backend has to know. Handing the
// emitter a beta-redex would test a term nothing emits, and the residual cannot
// simply be pasted in because the reader does not round-trip a desugared loop.
const mapSrc = `
	(use go)
	(export run)
	(def run (fn (n k)
	  (let m (build-map 8 (fn (m)
  	         (loop ((m m) (i 0))
  	           (go.>= i n)  m
  	           else         (again (insert m i (go.* i 10)) (go.+ i 1)))))
     (case (m k) (some v) v none -1))))
	(sig run ((n int) (k int)) int)
`

func mustRead(t *testing.T, src string) *core.Term {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	return forms[0].Term
}

func receiveTarget(t *testing.T) *emit.Target {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/go")
	if err != nil {
		t.Fatal(err)
	}
	tg.Prims["x.decode"] = emit.Prim{Name: "x.decode", Args: []string{"string"},
		Results: []string{"int -2147483648 2147483647", "int"},
		Kind:    "expr", Form: "utf8.DecodeRuneInString(%s)", Import: "unicode/utf8"}
	tg.Prims["x.rev8"] = emit.Prim{Name: "x.rev8", Args: []string{"int 0 255"},
		Result: "int 0 255", Kind: "expr", Form: "bits.Reverse8(uint8(%s))", Import: "math/bits"}
	tg.Prims["x.count"] = emit.Prim{Name: "x.count", Args: []string{"string"},
		Result: "int", Kind: "expr", Form: "utf8.RuneCountInString(%s)", Import: "unicode/utf8"}
	return tg
}

func strSig(p string) *core.Sig {
	return &core.Sig{Params: []core.SigParam{{Name: p, Type: "string"}}, Result: "int"}
}

func sigOf(t *testing.T, decl string) *core.Sig {
	t.Helper()
	forms, err := core.Read("(sig sq ((n " + decl + ")) int)")
	if err != nil || len(forms) != 1 || forms[0].Sig == nil {
		t.Fatalf("read sig %s: %v", decl, err)
	}
	return forms[0].Sig
}

// refineOn runs the REFINEMENT pass, which is where a bounds obligation is
// discharged. genOn only emits, and the hole below was invisible until this
// existed — the emitter is perfectly happy to write `a[i]` for any i.
func refineOn(t *testing.T, target, src, name string) error {
	t.Helper()
	tg, err := emit.LoadTarget("../targets/" + target)
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
	_, err = emit.Refine(tg, name, prog.Sigs[q], nf)
	return err
}

// normGo reads, loads and reduces a program's first export against the Go
// target, returning the residual and its signature.
func normGo(t *testing.T, src string) (*emit.Target, *core.Term, *core.Sig) {
	t.Helper()
	tg := goNative(t)
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
	return tg, nf, prog.Sigs[q]
}
