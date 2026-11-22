package main

import (
	"strings"
	"testing"

	"oroboros/ir"
)

// A LOOP VARIABLE IN E IS THE DOUBLE WORD'S (ADR 0153). The residue map
// r : ℤ → ℤ/2¹²⁸ commutes with a join's choice, so a loop's variable whose
// inputs are words or held in E is carried as its residue: two digits on Go
// (bits.Add64's carry), where it was a *big.Int each step. Two programs on Go,
// the sum of i for i below n ≤ 2⁴⁰, at most 2⁸⁰:
//
//  1. read back into the word by `% 1000000007`: no math/big at all.
//     PlantDoubleNoJoins keeps E off the join, and the sum is a *big.Int again;
//     PlantW14JoinsUnread has the verifier refuse a double word at a loop's exit;
//  2. returned into ℤ, `(Int)`: the loop in E, the exit its digits'
//     recombination by Euclid's identity in radix 2³² (RecombineYields).
//     PlantNoRecombine leaves the yield, whose demand then holds the loop in ℤ.
//
// The values on four targets are the differential's e-loop.
func TestALoopVariableInEIsTheDoubleWords(t *testing.T) {
	const back = "export fn f(n ∈ 1..2^40) (Int) {\n" +
		"    loop i = 0, s = 0 {\n        i < n -> again(i + 1, s + i)\n        else -> s % 1000000007\n    }\n}\n"
	const up = "export fn f(n ∈ 0..2^40) (Int) {\n" +
		"    loop i = 0, s = 0 {\n        i < n -> again(i + 1, s + i)\n        else -> s\n    }\n}\n"
	gen := func(src string) (string, error) { return surfaceGen(t, "go", map[string]string{"p.oro": src}) }
	code, err := gen(back)
	if err != nil || strings.Contains(code, "math/big") || !strings.Contains(code, "bits.Add64") {
		t.Fatalf("a sum read back by %% M: want it in the double word, no math/big: %v\n%s", err, code)
	}
	ir.PlantDoubleNoJoins = true
	code, err = gen(back)
	ir.PlantDoubleNoJoins = false
	if err == nil && !strings.Contains(code, "math/big") {
		t.Errorf("with E off the joins, the sum is still no *big.Int:\n%s", code)
	}
	ir.PlantW14JoinsUnread = true
	_, err = gen(back)
	ir.PlantW14JoinsUnread = false
	if err == nil || !strings.Contains(err.Error(), "W14") {
		t.Errorf("with W14 refusing a double word at a loop's exit: want W14, got %v", err)
	}

	code, err = gen(up)
	if err != nil || !strings.Contains(code, "bits.Add64") {
		t.Fatalf("a sum returned into ℤ: want its loop in the double word: %v\n%s", err, code)
	}
	if loop := code[strings.Index(code, "for "):strings.Index(code, "break")]; strings.Contains(loop, "big.") {
		t.Errorf("a sum returned into ℤ: its loop still computes a *big.Int:\n%s", loop)
	}
	ir.PlantNoRecombine = true
	code, err = gen(up)
	ir.PlantNoRecombine = false
	if err == nil && strings.Contains(code, "bits.Add64") {
		t.Errorf("without the recombination, the loop is still the double word's:\n%s", code)
	}
}
