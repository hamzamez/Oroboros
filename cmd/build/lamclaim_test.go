package main

import (
	"testing"

	"oroboros/ir"
)

// A CLAIMED λ COMPUTES WHAT ITS BODY COMPUTES (ADR 0158): lifting it to a
// definition is an equation by β, fn(x̄ ∈ S̄) (R) { e } = fn(x̄) { f#λ(ȳ, x̄) },
// so staging changes no answer (ADR 0009). Applied in place, through a
// definition of the program's (twice), with a where and an ensure, and in a
// loop, on four targets; cmd/gen's TestALambdaCarriesItsClaim plants the
// claim dropped.
func TestAClaimedLambdaComputesItsBody(t *testing.T) {
	if testing.Short() {
		t.Skip("builds programs on four targets")
	}
	const src = "use io\n\n" +
		"export fn run(k ∈ 0..5) (Int) { fn(n ∈ 0..9) (0..14) { n + k }(3) }\n\n" +
		"fn twice(f, x) { f(f(x)) }\n\n" +
		"export fn main() {\n" +
		"    io.print_int(run(4))\n" +
		"    io.print_int(twice(fn(n ∈ 0..100) (0..200) { n * 2 }, 7))\n" +
		"    io.print_int(fn(a ∈ Int, b ∈ Int) (Int) where a <= b ensure result >= 0 { b - a }(3, 10))\n" +
		"    loop i = 0 {\n" +
		"        i < 3 -> { io.print_int(fn(j ∈ 0..2) (0..4) { j + j }(i)); again(i + 1) }\n" +
		"        else -> ()\n" +
		"    }\n" +
		"}\n"
	const want = "7 28 7 0 2 4"
	for _, target := range []string{"go", "windows"} {
		if got, err := runOnMayFail(t, src, target); err != nil || got != want {
			t.Errorf("%s: claimed λs should print %s, got %q, %v", target, want, got, err)
		}
	}
	for _, target := range []string{"js", "java"} {
		if got, err := runOn(t, src, target); err != nil || got != want {
			t.Errorf("%s: claimed λs should print %s, got %q, %v", target, want, got, err)
		}
	}
	// KEPT AS A FUNCTION (ADR 0113): the lifted definition is an ordinary one,
	// printed under its name where windows keeps definitions
	old := ir.KeepThreshold
	ir.KeepThreshold = 1
	defer func() { ir.KeepThreshold = old }()
	if got, err := runOnMayFail(t, src, "windows"); err != nil || got != want {
		t.Errorf("windows, every definition kept: claimed λs should print %s, got %q, %v", want, got, err)
	}
}
