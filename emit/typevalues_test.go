package emit_test

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
	"oroboros/ir/golang"
)

// TYPES AS STATIC VALUES, step 1 (spec/typevalues.md, ADR 0063): a type is a
// set, a set is a value of the static level, a declaration over sets is
// instantiated by applying it to one, and no set survives staging. Each rule
// has a witness that fails when the rule is broken, planted in the target the
// rule reads.

// asType is the instance at work: errors.AsType applied to the set NumError.
const asType = `use go/strconv as sc
use go/errors
export fn f(e ∈ error) (Bool) {
    case errors.AsType(sc.NumError)(e) { 'some(n) -> true; 'none -> false }
}
`

// loadGo loads src on tg and returns the program and its environment.
func loadGo(t *testing.T, tg *emit.Target, src string) (*core.Program, *core.Env) {
	t.Helper()
	forms, err := core.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	prog, _, err := tg.LoadProgram(forms)
	if err != nil {
		t.Fatal(err)
	}
	env, err := tg.Env(prog)
	if err != nil {
		t.Fatal(err)
	}
	return prog, env
}

// normalizeOn reduces src's export f on tg, giving the reducer's refusal.
func normalizeOn(t *testing.T, tg *emit.Target, src string) (*core.Program, *core.Term, error) {
	t.Helper()
	prog, env := loadGo(t, tg, src)
	nf, err := core.Normalize(prog.Defs[prog.Exports[0]], env, core.DefaultFuel)
	return prog, nf, err
}

// irOf is the drivers' pipeline (ir.Entry) from a residual, as cmd/gen runs it.
func irOf(t *testing.T, tg *emit.Target, prog *core.Program, name string, nf *core.Term) *ir.Func {
	t.Helper()
	f, _, err := ir.Entry(tg, name, name, prog.Sigs[name], nf, allProgSigs(prog), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// AN INSTANCE IS MADE BY APPLICATION (§5): Go's own generic call, its type
// argument spelled by the host, the comma-ok flag read as the option (§7) and
// never E's nil, and the type argument's package imported though the program
// calls nothing in it.
func TestAnInstanceIsMadeByApplication(t *testing.T) {
	tg := goNative(t)
	prog, nf, err := normalizeOn(t, tg, asType)
	if err != nil {
		t.Fatal(err)
	}
	emit.ResetImports()
	code, err := golang.FromFunc(tg, irOf(t, tg, prog, "f", nf))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`, (\w+) := errors\.AsType\[\*strconv\.NumError\]\(\w+\)`).MatchString(code) {
		t.Errorf("the instance is not Go's errors.AsType[*strconv.NumError] with its flag bound:\n%s", code)
	}
	if strings.Contains(code, "== nil") {
		t.Errorf("the option was read off E's nil, not off the comma-ok flag (spec/typevalues.md §7):\n%s", code)
	}
	if !emit.Imports["strconv"] || !emit.Imports["errors"] {
		t.Errorf("the instance's packages are not imported: %v", emit.Imports)
	}
}

// THE BOUND IS MEMBERSHIP (§5): NumError ⊆ error by the declared edge, string
// in no subset of error. Both directions are planted in the relation the bound
// reads: without the edge the instance is refused, and with a false edge the
// refused one is made.
func TestTheBoundIsMembership(t *testing.T) {
	refusedAt := func(tg *emit.Target, set string) error {
		_, _, err := normalizeOn(t, tg, strings.Replace(asType, "sc.NumError", set, 1))
		return err
	}
	if err := refusedAt(goNative(t), "sc.NumError"); err != nil {
		t.Fatalf("NumError is below error, and AsType at it is refused: %v", err)
	}
	err := refusedAt(goNative(t), "string")
	if err == nil || !strings.Contains(err.Error(), "is not a subset of error") {
		t.Fatalf("AsType at string, which is no subset of error, was not refused for its bound: %v", err)
	}
	// PLANTED: the edge removed. The bound must read it.
	tg := goNative(t)
	delete(tg.Implements, "go/strconv.NumError")
	if refusedAt(tg, "sc.NumError") == nil {
		t.Error("with no (implements go/strconv.NumError error), AsType at NumError was still made")
	}
	// PLANTED: a false edge, string ⊆ error. The refusal must be the bound's.
	tg = goNative(t)
	tg.Implements["string"] = append(tg.Implements["string"], "error")
	if err := refusedAt(tg, "string"); err != nil {
		t.Errorf("with string declared below error, AsType at string is still refused: %v", err)
	}
}

// NO SET SURVIVES STAGING (Law S, §1 and §3), and no declaration over sets
// stays unapplied; the same program without either passes.
func TestNoSetSurvivesStaging(t *testing.T) {
	for src, want := range map[string]string{
		`use go/strconv as sc
use go/fmt
export fn f() { fmt.Println(sc.NumError) }`: "is a set, a static value",
		`use go/errors
use go/fmt
export fn f() { fmt.Println(errors.AsType) }`: "static argument",
		`use go/fmt
export fn f() { fmt.Println(error) }`: "is a set, a static value",
	} {
		if _, _, err := normalizeOn(t, goNative(t), src); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s\n  was not refused for %q: %v", src, want, err)
		}
	}
	if _, _, err := normalizeOn(t, goNative(t), `use go/fmt
export fn f() { fmt.Println("error") }`); err != nil {
		t.Errorf("a program holding no set is refused: %v", err)
	}
	// A CLAIM'S SET IS NOT A VALUE: a header names its sets in positions erasure
	// drops (abstract.md §5), so Law S does not refuse them. (examples/os/du.oro
	// found the refusal on `the`, which J does not have, ADR 0062.)
	if _, _, err := normalizeOn(t, goNative(t), `export fn f(n ∈ 0..5) (0..9) { n + 1 }`); err != nil {
		t.Errorf("a set in a claim position was refused as a value: %v", err)
	}
}

// A PROGRAM CANNOT YET NAME ITS OWN SET (§1): a type in a sig that names one of
// the program's definitions is refused at the sig, before step 2 gives the
// definition a set to denote. Accepted, it was an opaque type: refused at the
// first `+`, or printed as an unknown host type in a table (du-2026-10-14).
func TestAProgramsOwnSetIsRefusedAtItsSig(t *testing.T) {
	const want = "cannot yet name its own set"
	for _, src := range []string{
		`Byte = int(0, 255)
export fn f(x ∈ Byte) (0..256) { x + 1 }`,
		`Byte = int(0, 255)
export fn f(x ∈ 0..9) (Byte) { x + 1 }`,
		`Byte = int(0, 255)
export fn f(x ∈ Array(Byte)) (Int) { len(x) }`,
		`most = 2^62-1
Count = int(0, most)
export fn f(x ∈ Count) (Int) { x }`,
		`module sets {
    export Byte = int(0, 255)
}

module main {
    use sets as s
    export fn f(x ∈ s.Byte) (Int) { x }
}`,
	} {
		forms, err := core.Read(src)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := goNative(t).LoadProgram(forms); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s\n  was not refused at loading for %q: %v", src, want, err)
		}
	}
	// A DEFINITION NAMED IN A VALUE POSITION is no claim: the same constant used
	// as a bound's value passes.
	if _, _, err := normalizeOn(t, goNative(t), `most = 255
export fn f(x ∈ 0..9) (Int) { if x < most { x } else { most } }`); err != nil {
		t.Errorf("a constant used as a value was refused: %v", err)
	}
}

// A STATIC λ (§6): a definition whose parameter is a set, applied to one; β
// gives the set and then the instance is made, with no rule but application.
func TestAStaticLambdaPassesASetOn(t *testing.T) {
	src := strings.Replace(asType, `fn f(e) { case errors.AsType(sc.NumError)(e) { 'some(n) -> true; 'none -> false } }`,
		`fn is_a(E) { fn(e) { case errors.AsType(E)(e) { 'some(n) -> true; 'none -> false } } }
fn f(e) { is_a(sc.NumError)(e) }`, 1)
	tg := goNative(t)
	_, nf, err := normalizeOn(t, tg, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(nf.String(), "#[go/strconv.NumError]") {
		t.Errorf("the static λ did not instantiate AsType at NumError: %s", nf)
	}
}

// AN INSTANCE IS A FUNCTION OF ITS NAME (§5): a fresh target re-derives exactly
// the primitive reduction made, and the IR's text, read back and printed from a
// fresh target, prints what the in-memory program printed (ADR 0038:
// e_T = e_T ∘ read ∘ print).
func TestAnInstanceIsAFunctionOfItsName(t *testing.T) {
	tg := goNative(t)
	prog, nf, err := normalizeOn(t, tg, asType)
	if err != nil {
		t.Fatal(err)
	}
	var inst string
	for n := range tg.Prims {
		if strings.Contains(n, "#[") {
			if inst != "" {
				t.Fatalf("two instances, %s and %s, where the program applies one declaration once", inst, n)
			}
			inst = n
		}
	}
	if inst == "" {
		t.Fatal("reduction made no instance")
	}
	fresh := goNative(t)
	if err := fresh.Instances([]string{inst}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tg.Prims[inst], fresh.Prims[inst]) {
		t.Errorf("the name re-derives another primitive:\n reduced %+v\n named   %+v", tg.Prims[inst], fresh.Prims[inst])
	}
	f := irOf(t, tg, prog, "f", nf)
	want, err := golang.FromFunc(tg, f)
	if err != nil {
		t.Fatal(err)
	}
	read, err := ir.Read(ir.Print(&ir.Program{Target: "go", Stage: ir.StageP, Funcs: []*ir.Func{f.Clone()}}))
	if err != nil {
		t.Fatal(err)
	}
	reader := goNative(t)
	if err := ir.Verify(reader, read); err == nil {
		t.Error("the text verified on a target that has not re-derived its instance: the check is vacuous")
	}
	if err := reader.Instances(read.Calls()); err != nil {
		t.Fatal(err)
	}
	if err := ir.Verify(reader, read); err != nil {
		t.Fatal(err)
	}
	got, err := golang.Func(reader, read.Funcs[0])
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("printed from the text:\n%s\nprinted in memory:\n%s", got, want)
	}
}

// ONE NAMESPACE (§2): a module that declares a name as a type and as a value is
// refused, which is how the hand NumError constructor was found.
func TestOneNamespace(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/t.oro", []byte(`backend go
repr -2^63..2^63-1 word
module m {
    set T = host "int"
    fn T(x ∈ Int) (T) { host expr "${x}" }
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := emit.LoadTarget(dir + "/t.oro")
	if err == nil || !strings.Contains(err.Error(), "as a type and as a value") {
		t.Errorf("a type and a value of one name in one module was not refused: %v", err)
	}
}
