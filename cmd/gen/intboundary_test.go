package main

import (
	"os"
	"strings"
	"testing"

	"oroboros/emit"
	"oroboros/ir"
)

// `Int` IS ℤ AT AN ENTRY POINT TOO (ADR 0147, integers-boundary-research.md).
// Each rule against the planted fault that undoes it, the old behaviour, which
// must fail first:
//
//  1. an export's unbounded `Int` parameter is the host's integer of any size
//     (`*big.Int`), not its word: PlantEntryIntIsTheWord refuses `square`'s
//     product again, as the book's §0.5 printed;
//  2. on JavaScript a bounded range above the word at an entry point is a
//     `BigInt`, not the unsigned word's pair: PlantBoundedAboveTheWordIsTheWord
//     refuses `wide(n ∈ 0..2^63-1)` there again;
//  3. a result declared `Int` is ℤ, its yields demanded: PlantResultIntIsTheWord
//     refuses finding 27's `medium` on JavaScript, its product a pair at `Int`;
//  4. `decimal` of a value in the word is the host's own formatting,
//     `strconv.Itoa`, no widening: PlantDecimalAlwaysBig widens it to a
//     `*big.Int` to print it, which io.print_int checked on its own, its n
//     unbounded, cannot do, and refuses;
//  5. a big integer in a function's type imports its package
//     (finding 109): `any(n ∈ Int) (Int) { n }` names `*big.Int` and computes
//     nothing; checked by its text;
//  6. a residue's spine is a literal, not an exact value: with
//     PlantResidueSpineExact, chapter 7's hash64 is refused at the target's `+`
//     once a print makes the rung run.
func TestIntIsTheIntegersAtTheBoundary(t *testing.T) {
	const square = "export fn square(n ∈ Int) (Int) { n * n }\n"
	if code, err := onGo(t, square); err != nil || !strings.Contains(code, "func GenSquare(v0 *big.Int) *big.Int") {
		t.Fatalf("square on Go: %v\n%s", err, code)
	}
	emit.PlantEntryIntIsTheWord = true
	if _, err := onGo(t, square); err == nil {
		t.Errorf("with an entry point's Int the word, square should be refused")
	}
	emit.PlantEntryIntIsTheWord = false

	const wide = "export fn wide(n ∈ 0..2^63-1) (0..2^126) { n * n }\n"
	if code, err := surfaceGen(t, "js", map[string]string{"p.oro": wide}); err != nil || !strings.Contains(code, "BigInt") {
		t.Fatalf("wide on JavaScript: %v\n%s", err, code)
	}
	ir.PlantBoundedAboveTheWordIsTheWord = true
	if _, err := surfaceGen(t, "js", map[string]string{"p.oro": wide}); err == nil {
		t.Errorf("with the pair at the boundary, wide should be refused on JavaScript")
	}
	ir.PlantBoundedAboveTheWordIsTheWord = false

	const medium = "export fn medium(n ∈ 0..2^31) (Int) { n * n }\n"
	if code, err := surfaceGen(t, "js", map[string]string{"p.oro": medium}); err != nil || !strings.Contains(code, "BigInt") {
		t.Fatalf("medium on JavaScript: %v\n%s", err, code)
	}
	ir.PlantResultIntIsTheWord = true
	if _, err := surfaceGen(t, "js", map[string]string{"p.oro": medium}); err == nil {
		t.Errorf("with a result's Int undemanded, medium should be refused on JavaScript (finding 27)")
	}
	ir.PlantResultIntIsTheWord = false

	const printed = "use io\n\nexport fn main() { io.print_int(6 * 7) }\n"
	if code, err := onGo(t, printed); err != nil || !strings.Contains(code, "strconv.Itoa(42)") || strings.Contains(code, "big.") {
		t.Fatalf("a word printed on Go: %v\n%s", err, code)
	}
	ir.PlantDecimalAlwaysBig = true
	// widened to print it, or, in io.print_int checked alone where n is ⊤,
	// refused: either way not the host's own formatting
	if code, err := onGo(t, printed); err == nil && !strings.Contains(code, "big.NewInt") {
		t.Errorf("with decimal always big, a word was printed without widening\n%s", code)
	}
	ir.PlantDecimalAlwaysBig = false

	const any = "export fn any(n ∈ Int) (Int) { n }\n"
	if code, err := onGo(t, any); err != nil || !strings.Contains(code, `import "math/big"`) {
		t.Errorf("any on Go: %v, want math/big imported\n%s", err, code)
	}

	const hash = "use io\n\nset U64 = 0..2^64-1\n\n" +
		"fn step(h ∈ U64, b ∈ 0..255) (U64) { ((h ⊕ b) * 1099511628211) mod 2^64 }\n\n" +
		"export fn main() {\n    h = step(step(step(0xCBF29CE484222325, 97), 98), 99)\n" +
		"    io.print_int(h >> 32)\n    io.print_int(h & 0xFFFFFFFF)\n}\n"
	if _, err := onGo(t, hash); err != nil {
		t.Fatalf("hash64 on Go: %v", err)
	}
	ir.PlantResidueSpineExact = true
	if _, err := onGo(t, hash); err == nil || !strings.Contains(err.Error(), "target's own operator") {
		t.Errorf("with the residue's spine exact: %v; want the target's + to refuse it", err)
	}
	ir.PlantResidueSpineExact = false
}

// AN INDEX BOUNDED BY A TABLE'S LENGTH IS IN THE WORD, AND A TABLE'S ELEMENT IS
// A WORD (ADR 0147, book chapter 13). `promised(t, i ∈ Int) where 0 <= i && i <
// len(t)` takes i in the word, a length being at most the target's largest
// table; PlantNoLenBound reads only constant bounds, and the read t[i] refuses
// i as a big integer. `position(t ∈ Array(Int), x ∈ Int)` compares t[i] with
// an x of any size, widening t[i], whose storage is a word; PlantElementTop
// reads the element as ⊤, and t[0] / 2 is no longer proven in the word.
func TestATablesElementIsAWord(t *testing.T) {
	// THE WORDS' AND E'S OWN RULE, as -words decides it: past them a value
	// is ℤ's under ADR 0150, and this witness is about the readings
	ir.WordsOnly = true
	defer func() { ir.WordsOnly = false }()
	const promised = "export fn promised(t ∈ Array(Int), i ∈ Int) (Int) where 0 <= i && i < len(t) { t[i] }\n"
	if code, err := onGo(t, promised); err != nil || !strings.Contains(code, "func GenPromised(v0 []int, v1 int) int") {
		t.Fatalf("promised: %v\n%s", err, code)
	}
	emit.PlantNoLenBound = true
	if _, err := onGo(t, promised); err == nil {
		t.Errorf("with no bound by a length, promised should be refused at its index")
	}
	emit.PlantNoLenBound = false

	// THROUGH ANOTHER PARAMETER: i < n && n <= len(t), the difference bounds'
	// closure (chapter 16's chain); PlantNoDiffBound reads none
	const chain = "export fn chain(i ∈ Int, n ∈ Int, t ∈ Array(Int)) (Int) where 0 <= i && i < n && n <= len(t) {\n    t[i]\n}\n"
	if code, err := onGo(t, chain); err != nil || !strings.Contains(code, "func GenChain(v0 int, v1 int, v2 []int) int") {
		t.Fatalf("chain: %v\n%s", err, code)
	}
	emit.PlantNoDiffBound = true
	if _, err := onGo(t, chain); err == nil {
		t.Errorf("with no bound between parameters, chain should be refused at its index")
	}
	emit.PlantNoDiffBound = false

	const position = "export fn position(t ∈ Array(Int), x ∈ Int) (Int) {\n" +
		"    loop i = 0 {\n        i >= len(t) -> -1\n        t[i] == x -> i\n        else -> again(i + 1)\n    }\n}\n"
	for _, target := range []string{"go", "js", "java"} {
		if _, err := surfaceGen(t, target, map[string]string{"p.oro": position}); err != nil {
			t.Fatalf("position on %s: %v", target, err)
		}
	}
	// the element widens by its representation, a word (ir/big.go, wordByDef);
	// its FACT is the word's where arithmetic reads it: t[0] / 2 is proven in
	// the word, and with the element ⊤ (PlantElementTop) it is refused
	const half = "export fn half(t ∈ Array(Int)) (-2^62..2^62) where len(t) >= 1 { t[0] / 2 }\n"
	if _, err := onGo(t, half); err != nil {
		t.Fatalf("half of an element: %v", err)
	}
	ir.PlantElementTop = true
	if _, err := onGo(t, half); err == nil {
		t.Errorf("with an element ⊤, half should be refused at its quotient")
	}
	ir.PlantElementTop = false
}

// A WHERE'S CONJUNCTS ON WORD PARAMETERS ARE ASSUMED WITH ONE ABOVE THE WORD
// (ADR 0147, book chapter 13's binary search). `x ∈ Int` is a big integer at
// the entry, and assumeWhere dropped the whole `where` for it, len(t) <= 10^9
// with it: the search's loop variables then widened past the signed word and
// the result, an `Int` of them, was refused. Only a conjunct naming x means
// something else above the word. PlantWhereDroppedWhole drops it all again,
// and `sq`'s n, bounded only by its conjuncts beside x's, is ℤ and its square
// leaves the claim (the search itself is proven since ADR 0148 without the
// bound: its midpoint is decided in the word).
func TestAWheresWordConjunctsAreAssumed(t *testing.T) {
	const search = "fn half(lo, hi) { lo + (hi - lo) / 2 }\n\n" +
		"export fn search(t ∈ Array(Int), x ∈ Int) (Int) where len(t) <= 10^9 {\n" +
		"    loop lo = 0, hi = len(t), m = len(t) / 2 {\n" +
		"        lo >= hi -> -1\n" +
		"        lo < 0 || m < lo || m >= hi || hi > len(t) -> -1\n" +
		"        t[m] < x -> again(m + 1, hi, half(m + 1, hi))\n" +
		"        t[m] > x -> again(lo, m, half(lo, m))\n" +
		"        else -> m\n    }\n}\n"
	if code, err := onGo(t, search); err != nil || !strings.Contains(code, "func GenSearch(v0 []int, v1 *big.Int) int") {
		t.Fatalf("search with x ∈ Int: %v\n%s", err, code)
	}
	const sq = "export fn sq(n ∈ Int, x ∈ Int) (0..1000000) where 0 <= n && n <= 1000 && x > 0 { n * n }\n"
	if code, err := onGo(t, sq); err != nil || !strings.Contains(code, "func GenSq(v0 int, v1 *big.Int) int") {
		t.Fatalf("sq with n bounded by its conjuncts: %v\n%s", err, code)
	}
	ir.PlantWhereDroppedWhole = true
	defer func() { ir.PlantWhereDroppedWhole = false }()
	if _, err := onGo(t, sq); err == nil {
		t.Errorf("with the whole where dropped, sq should be refused")
	}
}

// A JOIN OF HOST RESULTS IS NOT EXACT (ADR 0147's result demand): sieve-js
// counts with JavaScript's own `js.+` on `Any`, so its accumulator is a join of
// host results, and a result declared `Int` demanded it exact, which then
// refused the host's operator. A join is exact by demand only where some input
// can be, a language operation or a join of one. THE PLANTED FAULT takes it
// exact anyway, and the gauntlet's sieve is refused on JavaScript again.
func TestAJoinOfHostResultsIsNotExact(t *testing.T) {
	src, err := os.ReadFile("../../examples/native/sieve-js.oro")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := surfaceGen(t, "js", map[string]string{"p.oro": string(src)}); err != nil {
		t.Fatalf("sieve-js on js: %v", err)
	}
	ir.PlantJoinOfHostsExact = true
	defer func() { ir.PlantJoinOfHostsExact = false }()
	if _, err := surfaceGen(t, "js", map[string]string{"p.oro": string(src)}); err == nil {
		t.Errorf("with a join of host results exact, sieve-js should be refused")
	}
}
