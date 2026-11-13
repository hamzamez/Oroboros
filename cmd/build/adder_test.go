package main

import (
	"math/big"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir/llvm"
)

// THE FULL ADDER (ADR 0108): a sum of two words and a bit, read by its two
// halves, is printed as the triple's adc. The machine reads its carry in as
// c ≠ 0, so the premise is c ∈ {0, 1}. Here the third leaf is in [0, 2]: t =
// (2⁶⁴ − 1) + i + c with c = i mod 3, read by t & (2⁶⁴ − 1) and t >> 64, so it
// must stay in the double word. The program sums the low halves mod 7 and
// the carries, mod 1000. (Printed by print_int: big_str demands an exact
// integer, and the rung above the word is chosen before the double word.)
const adderSrc = "use win/print\n" +
	"\n" +
	"fn run(n ∈ 0..1000) (0..999) {\n" +
	"    loop i = 0, c = 0, acc = 0 {\n" +
	"        i >= n -> acc\n" +
	"        else -> {\n" +
	"            t = 18446744073709551615 + i + c\n" +
	"            again(i + 1, (i + 1) % 3, (acc + (t & 18446744073709551615) % 7 + (t >> 64)) % 1000)\n" +
	"        }\n" +
	"    }\n" +
	"}\n" +
	"\n" +
	"export fn main() {\n" +
	"    ignore(print.print_int(run(1000)))\n" +
	"}\n"

// adderWant is the program's answer, computed over ℤ.
func adderWant() string {
	b := new(big.Int).Lsh(big.NewInt(1), 64)
	m := new(big.Int).Sub(b, big.NewInt(1))
	c, acc := big.NewInt(0), int64(0)
	for i := int64(0); i < 1000; i++ {
		t := new(big.Int).Add(m, big.NewInt(i))
		t.Add(t, c)
		lo := new(big.Int).And(t, m)
		acc = (acc + new(big.Int).Mod(lo, big.NewInt(7)).Int64() + new(big.Int).Rsh(t, 64).Int64()) % 1000
		c = big.NewInt((i + 1) % 3)
	}
	return big.NewInt(acc).String()
}

const adcDecl = "declare { i8, i64 } @llvm.x86.addcarry.64(i8, i64, i64)"

// THE PLANTED FAULT takes a leaf in [0, 2] as the carry: the adder computes
// x + y + 1 where the program means x + y + 2, and the answer is wrong. And the
// witness is not vacuous: K(ℕ*)'s sum is printed by the adder.
func TestTheFullAdderNeedsItsCarryInZeroOne(t *testing.T) {
	want := adderWant()
	out, err := buildAndRun(t, adderSrc, "windows", "")
	if err != nil || strings.TrimSpace(out) != want {
		t.Fatalf("a carry in [0, 2] on windows: %q, %v; want %s", out, err, want)
	}
	if emit.LLVMImports[adcDecl] {
		t.Errorf("a carry in [0, 2] was printed by the adder")
	}
	llvm.PlantAdderFault = true
	out, err = buildAndRun(t, adderSrc, "windows", "")
	llvm.PlantAdderFault = false
	if err != nil {
		t.Fatal(err)
	}
	if !emit.LLVMImports[adcDecl] {
		t.Fatalf("with the fault planted the adder was not chosen: the witness cannot fail")
	}
	if strings.TrimSpace(out) == want {
		t.Errorf("with a carry of 2 read as 1 the answer is still %s: the witness cannot fail", want)
	}
	const fib = "use win/print\n\n" +
		"fn run(n) { big_str(loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) }) }\n\n" +
		"export fn main() { print.print_str(run(300)) }\n"
	out, err = buildAndRun(t, fib, "windows", "")
	if err != nil || strings.TrimSpace(out) != "222232244629420445529739893461909967206666939096499764990979600" {
		t.Fatalf("fib(300) on windows: %q, %v", out, err)
	}
	if !emit.LLVMImports[adcDecl] {
		t.Errorf("K(ℕ*)'s sum is not printed by the adder")
	}
}

// THE SECOND PREMISE, every use one of φ's reads: here the carry is a bit, i
// mod 2, but the sum is also read by t >> 63, which is no half of it. THE
// PLANTED FAULT takes any shift as the carry, and the answer is wrong.
func TestTheFullAdderReadsOnlyItsHalves(t *testing.T) {
	// t >> 63 is the carry plus 1 here, so the sum is taken mod 997, which 1000
	// such differences do not cancel in
	src := strings.Replace(strings.Replace(strings.Replace(adderSrc, "(i + 1) % 3", "(i + 1) % 2", 1), "(t >> 64)", "(t >> 63)", 1), "% 1000)", "% 997)", 1)
	if src == adderSrc {
		t.Fatal("the program no longer reads its sum by >> 64 with a carry i mod 3: restate the witness")
	}
	b := new(big.Int).Lsh(big.NewInt(1), 64)
	m := new(big.Int).Sub(b, big.NewInt(1))
	acc := int64(0)
	for i := int64(0); i < 1000; i++ {
		x := new(big.Int).Add(m, big.NewInt(i))
		x.Add(x, big.NewInt(i%2))
		acc = (acc + new(big.Int).Mod(new(big.Int).And(x, m), big.NewInt(7)).Int64() + new(big.Int).Rsh(x, 63).Int64()) % 997
	}
	want := big.NewInt(acc).String()
	out, err := buildAndRun(t, src, "windows", "")
	if err != nil || strings.TrimSpace(out) != want {
		t.Fatalf("a sum read by >> 63 on windows: %q, %v; want %s", out, err, want)
	}
	if emit.LLVMImports[adcDecl] {
		t.Errorf("a sum read by >> 63 was printed by the adder")
	}
	llvm.PlantAdderReadFault = true
	out, err = buildAndRun(t, src, "windows", "")
	llvm.PlantAdderReadFault = false
	if err != nil {
		t.Fatal(err)
	}
	if !emit.LLVMImports[adcDecl] {
		t.Fatalf("with the fault planted the adder was not chosen: the witness cannot fail")
	}
	if strings.TrimSpace(out) == want {
		t.Errorf("with t >> 63 read as the carry the answer is still %s: the witness cannot fail", want)
	}
}
