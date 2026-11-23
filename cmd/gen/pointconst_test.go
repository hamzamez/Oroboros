package main

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// A POINT IS ITS CONSTANT AT AN EXPORT (the book's finding 120, ADR 0156):
// each export compiled as its own unit, on every target, its yield into ℤ the
// closed term held exactly; PlantRecombinePoints recombines the point from its
// digits again, and Go refuses the export (cmd/build's TestAPointIsItsConstant
// runs the values).
func TestAPointIsItsConstantAtAnExport(t *testing.T) {
	const src = "export fn lit() (Int) { 18446744073709551616 }\n" +
		"export fn too_big() (Int) { 2^62 * 4 }\n" +
		"export fn neg() (Int) { 0 - 2^62 * 4 }\n"
	for _, target := range []string{"go", "js", "java", "windows"} {
		if _, err := surfaceGen(t, target, map[string]string{"p.oro": src}); err != nil {
			t.Errorf("%s: exports returning a constant past the word should compile: %v", target, err)
		}
	}
	ir.PlantRecombinePoints = true
	_, err := surfaceGen(t, "go", map[string]string{"p.oro": src})
	ir.PlantRecombinePoints = false
	if err == nil {
		t.Error("with a point recombined Go should refuse the export: the witness cannot fail")
	}
}

// A CONSTANT IS THE HOST'S OWN (ADR 0157). A constant held exactly is a
// nullary operation of ℤ, realized by the host's own constant at the highest
// layer it provides: Go's package-level *big.Int made once, the JVM's static
// BigInteger, a BigInt literal, windows' numeral in read-only data. Built from
// its digits where it was read, a loop over 2¹⁰⁰ ran 2.4× slower on Go and 3.5×
// on windows. PlantConstantSpined builds it from its digits again.
func TestAConstantIsTheHostsOwn(t *testing.T) {
	const src = "export fn sum(n ∈ 0..1000) (Int) {\n" +
		"    loop i = 0, acc = 0 {\n" +
		"        i < n -> again(i + 1, acc + i * 1267650600228229401496703205376)\n" +
		"        else -> acc\n    }\n}\n"
	want := map[string]string{
		"go":      `SetString("1267650600228229401496703205376", 10)`,
		"java":    `static final java.math.BigInteger oroBig0 = new java.math.BigInteger("1267650600228229401496703205376")`,
		"js":      "1267650600228229401496703205376n",
		"windows": "= private unnamed_addr constant { i64, [3 x i64] } { i64 3, [3 x i64] [i64 0, i64 68719476736, i64 4] }",
	}
	for target, w := range want {
		code, err := surfaceGen(t, target, map[string]string{"p.oro": src})
		if err != nil || !strings.Contains(code, w) {
			t.Errorf("%s: the constant should be the host's own, %s; got %v\n%s", target, w, err, code)
		}
	}
	ir.PlantConstantSpined = true
	code, err := surfaceGen(t, "go", map[string]string{"p.oro": src})
	ir.PlantConstantSpined = false
	if err != nil || strings.Contains(code, "SetString") {
		t.Errorf("with the constant spined Go should build it from its digits again: %v", err)
	}
	// A RESULT IS A BIRTH ITS CALLER OWNS (the frame rule, ADR 0101): Go's
	// *big.Int is mutable, so a shared constant leaves a function as a copy;
	// PlantSharedConstantMoved hands the caller the constant itself
	const ret = "export fn k() (Int) { 18446744073709551616 }\n"
	if code, err := surfaceGen(t, "go", map[string]string{"p.oro": ret}); err != nil || !strings.Contains(code, "return new(big.Int).Set(oroBig0)") {
		t.Errorf("Go should return a copy of the shared constant: %v\n%s", err, code)
	}
	ir.PlantSharedConstantMoved = true
	code, err = surfaceGen(t, "go", map[string]string{"p.oro": ret})
	ir.PlantSharedConstantMoved = false
	if err != nil || !strings.Contains(code, "return oroBig0") {
		t.Errorf("with the shared constant moved, Go should return it itself: %v", err)
	}
}

// ON FIXED LIMBS A CLOSED TERM IS ITS TERM (ADR 0157). Fixed limbs give a
// constant no form of its own (their layout is the instance's width), so a
// constant there is its spine: one constant per closed term was one spine per
// term, and per chunk of its numeral, where the term's own structure shares
// its operands (big-signed-ops compiled 3.2× slower on windows, big-mul on the
// JVM's limbs passed its 64 KB method). PlantClosedTermsOnLimbs makes each
// closed term its constant there again, and the code larger.
func TestAClosedTermOnLimbsIsItsTerm(t *testing.T) {
	const src = "export fn calc(n ∈ 0..3) (-2^200..2^200) {\n" +
		"    a = 3^100\n    b = 0 - 5^60\n" +
		"    case {\n        n == 0 -> a + b\n        n == 1 -> b + b - a\n        else -> a * (n - 9)\n    }\n}\n"
	base, err := surfaceGen(t, "windows", map[string]string{"p.oro": src})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		plant *bool
	}{{"a closed term its constant on limbs", &ir.PlantClosedTermsOnLimbs}} {
		*c.plant = true
		code, err := surfaceGen(t, "windows", map[string]string{"p.oro": src})
		*c.plant = false
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(code) <= len(base) {
			t.Errorf("with %s the fixed-limb code should be larger: %d bytes, against %d", c.name, len(code), len(base))
		}
	}
}
