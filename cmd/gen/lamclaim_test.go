package main

import (
	"strings"
	"testing"

	"oroboros/core"
)

// A λ CARRIES ITS CLAIMS (the book's finding 121, ADR 0158). ADR 0051 made a λ
// carry what it claims and a definition the name bound to one; the reader
// refused `fn(n ∈ Int) { n }(1)` at the `∈` from 2026-10-16, a stopgap against
// a reader that took `∈` for a parameter. A claimed λ is lifted to a definition
// of its own, so its claim means a definition's: the domain an obligation where
// it is applied (ADR 0028), the codomain decided for every argument the domain
// admits (ADR 0072), and at a host's callback parameter the λ's domain must
// contain the host's and its answers lie in the host's result (the arrow's
// subsumption, contravariant and covariant). PlantLambdaClaimDropped
// lifts the λ without its claim: each false claim is then accepted.
func TestALambdaCarriesItsClaim(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		// 1 ∈ 0..9, but n ∉ 0..9 for every n ∈ Int: decided at the λ
		{"a false result", "export fn run() (Int) { fn(n ∈ Int) (0..9) { n }(1) }\n",
			"the λ at line 1: the body does not establish 0 <= result && result <= 9"},
		{"an argument outside the domain", "export fn run() (Int) { fn(d ∈ 0..9) (Int) { d * 2 }(10) }\n",
			"the λ at line 1's parameter d is declared in 0..9, and a call passes 10"},
		{"a false where", "export fn run() (Int) { fn(a ∈ Int, b ∈ Int) (Int) where a <= b { b - a }(10, 3) }\n",
			"the λ at line 1's `where` is false at a call"},
		{"a false ensure", "export fn run() (Int) { fn(a ∈ 0..9) (r ∈ Int) ensure r > a { a * 1 }(3) }\n",
			"does not establish its `ensure`"},
	}
	for _, c := range cases {
		for _, target := range []string{"go", "js", "java", "windows"} {
			_, err := surfaceGen(t, target, map[string]string{"p.oro": c.src})
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("%s on %s: want a refusal saying %q, got %v", c.name, target, c.want, err)
			}
		}
		core.PlantLambdaClaimDropped = true
		_, err := surfaceGen(t, "go", map[string]string{"p.oro": c.src})
		core.PlantLambdaClaimDropped = false
		if err != nil {
			t.Errorf("%s: with the claim dropped the program should compile, the witness: %v", c.name, err)
		}
	}
	// THE TRUE CLAIMS, the context's parameter keeping its set (k ∈ 0..5)
	const good = "export fn run(k ∈ 0..5) (Int) {\n" +
		"    a = fn(n ∈ 0..9) (0..14) { n + k }(3)\n" +
		"    b = fn(a ∈ Int, b ∈ Int) (Int) where a <= b ensure result >= 0 { b - a }(3, 10)\n" +
		"    a + b\n}\n"
	for _, target := range []string{"go", "js", "java", "windows"} {
		if _, err := surfaceGen(t, target, map[string]string{"p.oro": good}); err != nil {
			t.Errorf("%s: true claims on λs should compile: %v", target, err)
		}
	}
	// A NAME WITH NO SET IN THE CONTEXT is claimed by nothing in the lifted
	// definition: j = k + 1 is in 1..6, but the claim is decided for every j
	const ctx = "export fn run(k ∈ 0..5) (Int) {\n    j = k + 1\n    fn(n ∈ 0..9) (0..15) { n + j }(3)\n}\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": ctx}); err == nil || !strings.Contains(err.Error(), "the λ at line 3") {
		t.Errorf("a claim resting on a computed value should be refused at the λ: %v", err)
	}
}

// AT A HOST'S CALLBACK PARAMETER the host's claim meets the λ's by
// composition: the wrapper λ takes the host's parameter set and its call of the
// lifted definition obliges it to the λ's, dom(host) ⊆ dom(λ). strings.Map's
// mapping is int32 → int32: a λ claiming int32 compiles, one claiming 0..127
// is refused, and with the claim dropped it compiles.
func TestALambdaAtACallbackContainsTheHostsDomain(t *testing.T) {
	prog := func(set string) string {
		return "use io\nuse go/strings\n\nexport fn main() {\n" +
			"    io.print_line(strings.Map(fn(r ∈ " + set + ") (" + set + ") { r }, \"abc\"))\n}\n"
	}
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": prog("-2^31..2^31-1")}); err != nil {
		t.Errorf("a λ claiming the host's domain should compile: %v", err)
	}
	_, err := surfaceGen(t, "go", map[string]string{"p.oro": prog("0..127")})
	if err == nil || !strings.Contains(err.Error(), "the λ at line 5's parameter r") {
		t.Errorf("a λ claiming less than the host's domain should be refused at its parameter: %v", err)
	}
	core.PlantLambdaClaimDropped = true
	_, err = surfaceGen(t, "go", map[string]string{"p.oro": prog("0..127")})
	core.PlantLambdaClaimDropped = false
	if err != nil {
		t.Errorf("with the claim dropped the λ should compile, the witness: %v", err)
	}
	// COVARIANT: a λ claiming Int answers past the host's int32, and the host
	// receives its answer as int32, so it is refused at the host's claim
	const wide = "use io\nuse go/strings\n\nexport fn main() {\n" +
		"    io.print_line(strings.Map(fn(r ∈ -2^31..2^31-1) (Int) { r * 1099511627776 }, \"abc\"))\n}\n"
	if _, err := surfaceGen(t, "go", map[string]string{"p.oro": wide}); err == nil || !strings.Contains(err.Error(), "the claim's result: the host receives it as that set") {
		t.Errorf("a λ answering past the host's result should be refused at the host's claim: %v", err)
	}
}
