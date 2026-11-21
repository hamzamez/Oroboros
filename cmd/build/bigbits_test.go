package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
)

// ADR 0082'S OPERATIONS ABOVE THE WORD, BY TWO PROGRAMS (bigbits-2026-11-19):
// Stein's binary gcd over Fibonacci numbers and Kernighan's popcount, each
// printing the same lines on Go's math/big and on windows' K(ℕ*). Each fact the
// programs needed has its fault, under which the program is refused again:
//   - a let's claim read only when both its ends are words: Stein's odd(x),
//     x bound to fib's Nat, owes 0 <= x and no longer has it;
//   - a name bound to a closed integer opaque: big-bits' `mod a`, a = 3^100.
func TestTheBitOperationsAboveTheWordRun(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs programs on two targets")
	}
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	stein, pop := read("../../examples/big/stein.oro"), read("../../examples/big/popcount.oro")
	cases := []struct{ name, src, want string }{
		{"stein", stein, "354224848179261915075 7896325826131730509282738943634332893686268675876375 1"},
		{"popcount", pop, "758 4423"},
	}
	for _, c := range cases {
		for _, target := range []string{"go", "windows"} {
			if got, err := runOnMayFail(t, c.src, target); err != nil || got != c.want {
				t.Errorf("%s on %s: got %q, %v; want %q", c.name, target, got, err, c.want)
			}
		}
	}
	emit.PlantClaimBothEnds = true
	_, err := buildOnly(t, stein, "go")
	emit.PlantClaimBothEnds = false
	if err == nil || !strings.Contains(err.Error(), "odd requires 0 <=") {
		t.Errorf("with a half-line claim unread, Stein's odd(x) should owe 0 <= x unproven, got %v", err)
	}
	// a is read twice, so β binds it rather than substituting 3^100 into the
	// divisor, where the closed comparison alone would decide 0 < a
	mod := "use io\nexport fn main() {\n    a = 3^100\n    b = 0 - 5^60\n    c ∈ Nat = b mod a + a\n    io.print_line(decimal(c))\n}\n"
	if got, err := runOnMayFail(t, mod, "go"); err != nil || got != "1030754174102284673669375053569001849450845903377" {
		t.Errorf("(−5^60) mod 3^100 + 3^100: got %q, %v", got, err)
	}
	emit.PlantClosedLetOpaque = true
	_, err = buildOnly(t, mod, "go")
	emit.PlantClosedLetOpaque = false
	if err == nil || !strings.Contains(err.Error(), "mod requires") {
		t.Errorf("with a closed binder opaque, `mod a` should owe 0 < a unproven, got %v", err)
	}
}
