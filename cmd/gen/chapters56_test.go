package main

import (
	"strings"
	"testing"

	"oroboros/core"
	"oroboros/emit"
	"oroboros/ir"
)

// THE BOOK'S CHAPTERS 5 AND 6 (docs/book/README.md, findings 22–29; plan
// item 5): two rules already decided that the code broke, correct programs
// refused, refusals with the wrong reason, and one decision (ADR 0116).

// A VALUE PROVEN IN THE WORD COMES BACK INTO IT (finding 28, ADR 0079 item 4):
// `t ∈ Nat = 2^70 + n; t - 2^70` is held above the word, and its fact is
// [0, 100], so at a result declared 0..100 it is r(v), the retraction ℤ ⇀ S,
// the identity on S. THE PLANTED FAULT refuses every value above the word at a
// word position, its fact unread.
func TestAValueProvenInTheWordComesBackIntoIt(t *testing.T) {
	const src = "export fn g(n ∈ 0..100) (0..100) {\n    t ∈ Nat = 2^70 + n\n    t - 2^70\n}\n"
	for _, tg := range []string{"go", "js", "java", "windows"} {
		if _, err := surfaceGen(t, tg, map[string]string{"p.oro": src}); err != nil {
			t.Fatalf("on %s: %v", tg, err)
		}
	}
	ir.PlantNarrowRefused = true
	defer func() { ir.PlantNarrowRefused = false }()
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "WIDER than the word") {
		t.Errorf("with the fact unread: %v; want the result refused as above the word", err)
	}
}

// AN ENTRY POINT'S RESULT IS ITS DECLARED SET'S (finding 27, ADR 0033, 0079):
// a result declared Int is the word on every target, so n * n with n ≤ 2^31,
// above JavaScript's word, is refused there as Go refuses a value of U at a
// declared Int, and declared 0..2^62 it is returned in the unsigned word.
// THE PLANTED FAULT returns a pair whatever the declared set: a BigInt from an
// export declared Int.
func TestAnEntryPointsResultIsItsDeclaredSets(t *testing.T) {
	const asInt = "export fn medium(n ∈ 0..2^31) (Int) { n * n }\n"
	const asRange = "export fn medium(n ∈ 0..2^31) (0..2^62) { n * n }\n"
	if _, err := surfaceGen(t, "js", map[string]string{"p.oro": asRange}); err != nil {
		t.Fatalf("declared 0..2^62 on js: %v", err)
	}
	if _, err := surfaceGen(t, "js", map[string]string{"p.oro": asInt}); err == nil || !strings.Contains(err.Error(), "declared Int") {
		t.Fatalf("declared Int on js: %v; want it refused", err)
	}
	if _, err := onGo(t, asInt); err != nil {
		t.Fatalf("declared Int on Go, where n * n is in the word: %v", err)
	}
	ir.PlantPairResultAnySet = true
	defer func() { ir.PlantPairResultAnySet = false }()
	if code, err := surfaceGen(t, "js", map[string]string{"p.oro": asInt}); err != nil || !strings.Contains(code, "BigInt") {
		t.Errorf("with the pair returned whatever the set: %v; want a BigInt returned", err)
	}
}

// AN UNSTATED RESULT IS THE BODY'S (finding 24, ADR 0062 item 8): a header that
// states no result claims nothing, so a computed pair is two results. THE
// PLANTED FAULT counts the unstated result as one: refused for its arity.
func TestAnUnstatedResultIsTheBodys(t *testing.T) {
	const src = "export fn ops(a ∈ 0..100, b ∈ 1..100) { (a + b, a - b) }\n"
	if code, err := onGo(t, src); err != nil || !strings.Contains(code, "(int, int)") {
		t.Fatalf("a computed pair: %v\n%s", err, code)
	}
	ir.PlantAnyIsOneResult = true
	defer func() { ir.PlantAnyIsOneResult = false }()
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "declares 1 result(s)") {
		t.Errorf("with the unstated result one: %v; want its arity refused", err)
	}
}

// A CLAIM IS IN ITS SET (finding 23, ADR 0049): square's result is
// the(Nat, n1 · n1), whose value is no linear term; the claim is decided apart,
// so it is an atom in 0.. for the obligation at the outer call, and
// square(square(10^20)) builds. THE PLANTED FAULT reads such a claim as
// nothing: 0 <= the(Nat, n1 · n1) does not follow.
func TestAClaimIsInItsSet(t *testing.T) {
	const src = "fn square(n ∈ Nat) (Nat) { n * n }\n\nexport fn big() (Nat) { square(square(10^20)) }\n"
	if _, err := onGo(t, src); err != nil {
		t.Fatalf("square(square(10^20)): %v", err)
	}
	emit.PlantClaimOpaque = true
	defer func() { emit.PlantClaimOpaque = false }()
	if _, err := onGo(t, src); err == nil || !strings.Contains(err.Error(), "does not follow") {
		t.Errorf("with the claim opaque: %v; want the obligation unproven", err)
	}
}

// A FUNCTION STATEMENT IS REFUSED (finding 22, ADR 0116): a function has no
// effect until applied, so `io.print_line` on a line of its own is dead code;
// ignore(io.print_line) is the discard that means it. THE PLANTED FAULT accepts
// the statement, which is dropped without a word.
func TestAFunctionStatementIsRefused(t *testing.T) {
	const bare = "use io\n\nexport fn main() {\n    io.print_line(\"a\")\n    io.print_line\n    io.print_line(\"b\")\n}\n"
	const ignored = "use io\n\nexport fn main() {\n    io.print_line(\"a\")\n    ignore(io.print_line)\n    io.print_line(\"b\")\n}\n"
	if _, err := onGo(t, ignored); err != nil {
		t.Fatalf("ignore(io.print_line): %v", err)
	}
	if _, err := onGo(t, bare); err == nil || !strings.Contains(err.Error(), "a statement's value is a function, io.print_line") {
		t.Fatalf("io.print_line as a statement: %v; want it refused, named", err)
	}
	core.PlantFunctionStatement = true
	defer func() { core.PlantFunctionStatement = false }()
	if _, err := onGo(t, bare); err != nil {
		t.Errorf("with the statement accepted: %v; want it dropped", err)
	}
}

// THE REFUSALS SAY WHY (findings 25, 26): an empty range is said to be empty,
// in J; JavaScript's quotient and midpoint over values above its word are
// refused for that, not as "may be negative" or with an interval the program
// does not have.
func TestTheRefusalsSayWhy(t *testing.T) {
	if _, err := onGo(t, "export fn emp(n ∈ 9..0) (Int) { n }\n"); err == nil ||
		!strings.Contains(err.Error(), "the set 9..0 is empty") {
		t.Errorf("9..0: %v; want it said to be empty", err)
	}
	for src, not := range map[string]string{
		"export fn third(n ∈ 0..2^60) (Int) { n / 3 }\n":                          "may be negative",
		"export fn mid(lo ∈ 0..2^63-1, hi ∈ 0..2^63-1) (Int) { (lo + hi) / 2 }\n": "[0, 2147483647]",
	} {
		_, err := surfaceGen(t, "js", map[string]string{"p.oro": src})
		if err == nil || strings.Contains(err.Error(), not) || !strings.Contains(err.Error(), "declared Int") {
			t.Errorf("%s on js: %v\n  want the declared Int's refusal, not %q", strings.TrimSpace(src), err, not)
		}
	}
}
