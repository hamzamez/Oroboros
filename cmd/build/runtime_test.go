package main

import (
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"oroboros/emit"
)

// THE TRIPLE'S RUNTIME DIVIDES (ADR 0123). On i686, LLVM's code generator
// calls __aulldiv, __aullrem, __alldiv and __allrem for a 64-bit `/` or `%` it
// does not expand, and windows32 links lib/rt/i686.ll, the language's
// definitions of them over divl. A program dividing words the facts cannot
// narrow, unsigned and signed, every edge class and 300 000 pairs of every
// width, prints Go's answer.
//
// THE PLANTED FAULT builds it without the runtime: the link must then refuse
// the helper, so the test reaches the calls and the runtime is what serves them.
func TestTheTriplesRuntimeDivides(t *testing.T) {
	edges := []uint64{0, 1, 2, 3, 7, 10, 1<<31 - 1, 1 << 31, 1<<32 - 1, 1 << 32, 1<<32 + 1, 1 << 33,
		1<<63 - 1, 1 << 63, 1<<63 + 1, 1<<64 - 1, 1<<64 - 2, 12345678901234567890, 18446744069414584320, 4294967296 * 12345}
	sedges := []int64{-1 << 63, -1<<63 + 1, -7, -1, 0, 1, 7, 1<<63 - 1, -4294967296, 4294967296, -4294967297,
		123456789012, -3, 1<<32 - 1, -(1<<32 - 1)}
	// the edges as chains of cases: the dividends, and the unsigned divisors,
	// every edge but 0, whose set excludes it
	var chain strings.Builder
	cases := func(name, set string, vs []string) {
		fmt.Fprintf(&chain, "fn %s(i ∈ 0..%d) (%s) {\n    case {\n", name, len(vs)-1, set)
		for k, v := range vs[:len(vs)-1] {
			fmt.Fprintf(&chain, "        i == %d -> %s\n", k, v)
		}
		fmt.Fprintf(&chain, "        else -> %s\n    }\n}\n", vs[len(vs)-1])
	}
	var us, ss []string
	for _, v := range edges {
		us = append(us, fmt.Sprint(v))
	}
	for _, v := range sedges {
		ss = append(ss, fmt.Sprint(v))
	}
	cases("ue", "0..2^64-1", us)
	cases("ye", "1..2^64-1", us[1:])
	cases("se", "-2^63..2^63-1", ss)
	src := "use win/print\n\n" + chain.String() + `
fn uval(i ∈ 0..2^20) (0..2^64-1) { (i * 11400714819323198485 + 7) mod 2^64 }
fn umag(i ∈ 0..2^20, k ∈ 0..63) (1..2^64-1) {
    z = ((i * 6364136223846793005 + 1442695040888963407) mod 2^64) >> k
    if z == 0 { 1 } else { z }
}
// a word of either sign: half the unsigned value, negated at every third i
fn sval(i ∈ 0..2^20) (-2^63+1..2^63-1) {
    m = ((i * 11400714819323198485 + 7) mod 2^64) >> 1
    if i % 3 == 0 { 0 - m } else { m }
}
fn smag(i ∈ 0..2^20, k ∈ 0..63) (1..2^63-1) {
    z = ((i * 6364136223846793005 + 1442695040888963407) mod 2^63) >> k
    if z == 0 { 1 } else { z }
}
// each quotient and remainder's hash under its divisor's guard, the facts the
// refinement layer reads: y ≥ 1, and b ≥ 1 or b ≤ -2, LLVM's domain without
// -1 and 0, its excluded points (the else arms are skipped by Go too)
fn uh(x ∈ 0..2^64-1, y ∈ 0..2^64-1) (0..2^63-1) {
    case {
        y >= 1 -> (x / y) % 1000000007 * 7 + (x % y) % 1000000007
        else -> 0
    }
}
fn sh(a ∈ -2^63..2^63-1, b ∈ -2^63..2^63-1) (0..2^63-1) {
    case {
        b >= 1 -> (a / b) mod 1000000007 * 3 + (a % b) mod 1000000007
        b <= -2 -> (a / b) mod 1000000007 * 3 + (a % b) mod 1000000007
        else -> 0
    }
}

export fn main() {
    h1 = loop i = 0, h = 0 {
        i >= 380 -> h
        else -> again(i + 1, (h * 31 + uh(ue(i / 19), ye(i % 19))) mod 1000000007)
    }
    print.print_int(h1)
    h2 = loop i = 0, h = 0 {
        i >= 225 -> h
        else -> again(i + 1, (h * 31 + sh(se(i / 15), se(i % 15))) mod 1000000007)
    }
    print.print_int(h2)
    h3 = loop i = 0, k = 0, h = 0 {
        i >= 300000 -> h
        else -> {
            m = smag(i, k)
            b = if i % 2 == 0 { m } else { 0 - m }
            again(i + 1, if k == 63 { 0 } else { k + 1 },
                (h * 31 + uh(uval(i), umag(i, k)) + sh(sval(i), b)) mod 1000000007)
        }
    }
    print.print_int(h3)
}
`
	const P = 1000000007
	emod := func(v int64) uint64 { return uint64(((v % P) + P) % P) }
	uh := func(x, y uint64) uint64 { return (x/y)%P*7 + (x%y)%P }
	sh := func(a, b int64) uint64 {
		if b == 0 || b == -1 {
			return 0
		}
		return emod(a/b)*3 + emod(a%b)
	}
	var h1 uint64
	for i := 0; i < 380; i++ {
		h1 = (h1*31 + uh(edges[i/19], edges[1+i%19])) % P
	}
	var h2 uint64
	for i := 0; i < 225; i++ {
		h2 = (h2*31 + sh(sedges[i/15], sedges[i%15])) % P
	}
	var h3 uint64
	k := uint(0)
	for i := uint64(0); i < 300000; i++ {
		y := (i*6364136223846793005 + 1442695040888963407) >> k
		if y == 0 {
			y = 1
		}
		m := int64(((i*6364136223846793005 + 1442695040888963407) % (1 << 63)) >> k)
		if m == 0 {
			m = 1
		}
		if i%2 == 1 {
			m = -m
		}
		a := int64((i*11400714819323198485 + 7) >> 1)
		if i%3 == 0 {
			a = -a
		}
		h3 = (h3*31 + uh(i*11400714819323198485+7, y) + sh(a, m)) % P
		if k == 63 {
			k = 0
		} else {
			k++
		}
	}
	want := fmt.Sprintf("%d\n%d\n%d", h1, h2, h3)
	out, err := buildAndRun(t, src, "windows32", "")
	if err != nil {
		t.Fatalf("windows32: %v", err)
	}
	if got := strings.ReplaceAll(strings.TrimSpace(out), "\r\n", "\n"); got != want {
		t.Errorf("windows32 divides: got\n%s\nwant\n%s", got, want)
	}
	emit.PlantRuntimeUnlinked = true
	_, err = buildAndRunNoExec(t, src, "windows32")
	emit.PlantRuntimeUnlinked = false
	if err == nil {
		t.Error("without the runtime the link should refuse __aulldiv, so the test reaches the helpers")
	}
}

// THE RUNTIME IS DEFINED WITHOUT ITSELF (ADR 0123 item 3): a helper whose body
// divided by a word would be a call LLVM lowers to the helper, a recursion with
// no base. So lib/rt/i686.ll, compiled for its triple at the build's -O2,
// calls none of the four symbols. THE PLANTED FAULT is a runtime whose
// unsigned quotient is the language's own `/`: compiled, it is found calling
// __aulldiv.
func TestTheRuntimeCallsNoHelper(t *testing.T) {
	clang := filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "LLVM", "bin", "clang.exe")
	if _, err := os.Stat(clang); err != nil {
		t.Skip("the LLVM release's clang is not installed")
	}
	selfCalls := func(ll string) []string {
		t.Helper()
		asm := filepath.Join(t.TempDir(), "rt.s")
		cmd := exec.Command(clang, "--target=i686-pc-windows-msvc", "-O2", "-Wno-override-module", "-S", "-o", asm, ll)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("clang: %v\n%s", err, out)
		}
		b, err := os.ReadFile(asm)
		if err != nil {
			t.Fatal(err)
		}
		return regexp.MustCompile(`call[l]?\s+(__a(ll|ull)(div|rem))\b`).FindAllString(string(b), -1)
	}
	if c := selfCalls("../../lib/rt/i686.ll"); len(c) > 0 {
		t.Errorf("the runtime calls its own helpers: %v", c)
	}
	src, err := os.ReadFile("../../lib/rt/i686.oro")
	if err != nil {
		t.Fatal(err)
	}
	bad := plantOnce(t, string(src), "export fn __aulldiv(x ∈ 0..2^64-1, y ∈ 0..2^64-1) (0..2^64-1) { udiv(x, y) }",
		"export fn __aulldiv(x ∈ 0..2^64-1, y ∈ 1..2^64-1) (0..2^64-1) { x / y }")
	dir := t.TempDir()
	path := filepath.Join(dir, "rt.oro")
	if err := os.WriteFile(path, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	ll := filepath.Join(dir, "rt.ll")
	gen := exec.Command("go", "run", "./cmd/gen", "-runtime", "x86_stdcallcc", "-name", "rt", path, "windows32", ll)
	gen.Dir = "../.."
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("gen: %v\n%s", err, out)
	}
	if c := selfCalls(ll); len(c) == 0 {
		t.Error("a runtime dividing by a word should be found calling __aulldiv, so the check reaches the rule")
	}
}

// A HOST RESULT'S SET BOUNDS A WHERE (ADR 0123 item 5): `clz_low` is declared
// into 32..63, so `1 << (clz_low(v) - 32)` has a count the refinement layer,
// which alone decides `<<`'s `where`, reads as non-negative, as the IR's
// intervals already did. THE PLANTED FAULT seeds no result set, and the shift
// is refused under its requirement.
func TestAHostResultBoundsAWhere(t *testing.T) {
	const src = "use win/print\nuse llvm/isa as isa\n\n" +
		"fn norm(v ∈ 1..4294967295) (1..2^31) { 1 << (isa.clz_low(v) - 32) }\n\n" +
		"export fn main() {\n    print.print_int(norm(1))\n    print.print_int(norm(2))\n    print.print_int(norm(3))\n}\n"
	out, err := buildAndRun(t, src, "windows32", "")
	if want := "2147483648\n1073741824\n1073741824"; err != nil || strings.ReplaceAll(strings.TrimSpace(out), "\r\n", "\n") != want {
		t.Errorf("the shift by a count of leading zeros: %q, %v; want %q", out, err, want)
	}
	emit.PlantHostResultUnassumed = true
	_, err = buildAndRunNoExec(t, src, "windows32")
	emit.PlantHostResultUnassumed = false
	if err == nil || !strings.Contains(err.Error(), "`<<` requires") {
		t.Errorf("with no result set seeded the shift should be refused under `<<`'s requirement, got %v", err)
	}
}

// K(ℕ*) IS IN THE TRIPLE'S DIGIT (ADR 0124): on windows32, B = 2³². A program
// whose integers have no bound computes 300!, the 500th Fibonacci number, and
// Algorithm D's quotients and remainders by numerals of 6 to 10 digits, and prints each by the tree
// of divisions (300! is 65 digits, past the threshold of 32), and every line is
// Go's math/big. THE PLANTED FAULT keeps the 64-bit block on the triple: the
// double word's division by a word is then i128's, which i686 has no 2-by-1
// division for, and the program is refused as it was before.
func TestKNStarIsInTheTriplesDigit(t *testing.T) {
	const src = `use win/print

fn fact(n ∈ 0..300) (Nat) { loop acc = 1, i = 2 { i > n -> acc; else -> again(acc * i, i + 1) } }
fn fib(n ∈ 0..500) (Nat) { loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) } }

export fn main() {
    f = fact(300)
    g = fib(500)
    print.print_str(decimal(f))
    print.print_str(decimal(g))
    print.print_str(decimal(f / 7^60))
    print.print_str(decimal(f % (3^130 + 1)))
    print.print_str(decimal((f * g + 7) % (2^300 + 1)))
    print.print_str(decimal((0 - f) / 7^60))
}
`
	fact := func(n int64) *big.Int { return new(big.Int).MulRange(1, n) }
	fib := func(n int) *big.Int {
		a, b := big.NewInt(0), big.NewInt(1)
		for i := 0; i < n; i++ {
			a, b = b, new(big.Int).Add(a, b)
		}
		return a
	}
	pow := func(b, e int64) *big.Int { return new(big.Int).Exp(big.NewInt(b), big.NewInt(e), nil) }
	one := big.NewInt(1)
	f, g := fact(300), fib(500)
	q := new(big.Int).Quo(f, pow(7, 60))
	r := new(big.Int).Rem(f, new(big.Int).Add(pow(3, 130), one))
	m := new(big.Int).Rem(new(big.Int).Add(new(big.Int).Mul(f, g), big.NewInt(7)), new(big.Int).Add(pow(2, 300), one))
	n := new(big.Int).Quo(new(big.Int).Neg(f), pow(7, 60))
	want := strings.Join([]string{f.String(), g.String(), q.String(), r.String(), m.String(), n.String()}, "\n")
	out, err := buildAndRun(t, src, "windows32", "")
	if err != nil {
		t.Fatalf("windows32: %v", err)
	}
	if got := strings.ReplaceAll(strings.TrimSpace(out), "\r\n", "\n"); got != want {
		t.Errorf("windows32's K(ℕ*):\n%s\nwant\n%s", got, want)
	}
	emit.PlantNatBase64 = true
	_, err = buildAndRunNoExec(t, src, "windows32")
	emit.PlantNatBase64 = false
	if err == nil || !strings.Contains(err.Error(), "no realization holds both") {
		t.Errorf("with the 64-bit block on windows32 the program should be refused as before, so the test reaches the block; got %v", err)
	}
}
