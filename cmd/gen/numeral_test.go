package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// AN EXPORT'S NUMERAL IS A NUMERAL'S TABLE (the harness, plan item 7). On
// windows a natural held exactly is K(ℕ*), and examples/big's fib returns
// one. Its result was declared `array int` while every instance it calls has
// a numeral's table, so Theorem D′'s class held two fixed representations and
// W5 refused a sound program: none of the windows files the emission step
// compared held K(ℕ*)'s text. Declared a numeral and proven by its stores,
// fib emits, `nat.add` and its full adder in the file. THE PLANTED FAULT
// declares the result `array int` again: W5's refusal comes back.
func TestAnExportsNumeralIsANumeralsTable(t *testing.T) {
	src, err := os.ReadFile("../../examples/big/fib.oro")
	if err != nil {
		t.Fatal(err)
	}
	code, err := surfaceGen(t, "windows", map[string]string{"p.oro": string(src)})
	if err != nil {
		t.Fatalf("fib on windows: %v", err)
	}
	for _, want := range []string{"define ptr @nat.add(", "@llvm.x86.addcarry.64"} {
		if !strings.Contains(code, want) {
			t.Errorf("fib's file holds no %q: K(ℕ*)'s text is not in it", want)
		}
	}
	ir.PlantNumeralResultInt = true
	defer func() { ir.PlantNumeralResultInt = false }()
	if _, err := surfaceGen(t, "windows", map[string]string{"p.oro": string(src)}); err == nil || !strings.Contains(err.Error(), "W5") {
		t.Errorf("with the result declared array int: %v; want W5's refusal", err)
	}
}

// A REFUSAL CARRIES THE INTERVAL ROUTE'S ERROR (the harness, plan item 7). With
// ADR 0115's planted fault the rule g's argument rides is left unallocated, so
// the residual does not lower and the IR's intervals, which bound i · i, decide
// nothing; the refinement layer's linear fragment cannot bound a product, so it
// refuses g's parameter. The refusal must say why the intervals were silent:
// the rule never allocated, the error oswin-2026-11-15 was misled past twice.
// THE PLANTED FAULT swallows the route's error again.
func TestARefusalCarriesTheIntervalRoutesError(t *testing.T) {
	const src = "fn g(x ∈ 0..10000) (Int) { x + 1 }\n\n" +
		"export fn f(n ∈ 0..100) (Array(Int)) { array { g(i * i) : i ∈ 0..n - 1 } }\n"
	if _, err := onGo(t, src); err != nil {
		t.Fatalf("the rule allocated at the boundary: %v", err)
	}
	emit.PlantRuleAtBoundary = true
	defer func() { emit.PlantRuleAtBoundary = false }()
	_, err := onGo(t, src)
	if err == nil || !strings.Contains(err.Error(), "never allocated") {
		t.Fatalf("with the rule unallocated: %v; want the refusal to name the rule never allocated", err)
	}
	ir.PlantRangeRouteSilent = true
	defer func() { ir.PlantRangeRouteSilent = false }()
	// the premise: swallowed, the refusal is the refinement layer's alone, on
	// g's parameter, the obligation the intervals would have proven
	if _, err := onGo(t, src); err == nil || strings.Contains(err.Error(), "never allocated") ||
		!strings.Contains(err.Error(), "g's parameter x, declared in 0..10000") {
		t.Errorf("with the route's error swallowed: %v; want g's obligation alone, as before", err)
	}
}
