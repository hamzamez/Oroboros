# An entry point's parameter in U on JavaScript enters the pair from its caller's BigInt

Date: 2026-11-21. hamza, 2026-11-21: "I agree, go in that order: 2, then 1, then 3". This is 1,
option A: the inverse of `pair-bigint` at the boundary.

## The law

On JavaScript U is a pair of 32-bit digits (ADR 0083), d(h, l) = h·2³² + l, and an entry point's
result in U already leaves as the BigInt its caller holds (`pair-bigint`). A parameter in U past the
word W = ±(2⁵³ − 1) is no Number either: its caller holds a BigInt. It enters by d's inverse,

    d⁻¹(x) = (⌊x/2³²⌋, x mod 2³²) = (Number(x >> 32n), Number(x & 4294967295n)),

each digit below 2³², a Number exactly; d ∘ d⁻¹ is the identity on U, so the pair is the same
integer. The digits are fixed once at the body's head, and every use reads them.

## What was built

1. **`pair_hi`, `pair_lo`** (targets/js/builtin.oro), `pair_bigint`'s inverse, among the unsigned
   word's names (emit/opnames.go).
2. **`entryParams`** (ir/pairs.go): an entry point's parameter whose fact is in U and not in W is
   typed as the host's integer, its digits `pair-hi` and `pair-lo` of it, placed at the body's head;
   the refusal of an entry parameter held as a pair stays for any other. Only entry points reach the
   pass on a pair target (K(ℕ\*)'s program functions are the LLVM layer's).
3. **`TestAnEntryParameterInUIsAPair`** (cmd/gen): four exports called from node with BigInts against
   BigInt's ℤ, `n + 1`, `(lo + hi) / 2` over naturals, an FNV step mod 2⁶⁴, and `x / 3`, a parameter
   that also leaves the pair (the rung's `big-of` read back on its digits as the same BigInt); THE
   PLANTED FAULT `PlantPairEntryAsWord` splits the parameter as a word again, and they are refused.
4. **The book**: §6.11's halves, a value of U crossing an entry point both ways, §6.9's `next` on
   JavaScript.

The book's `midpoint` (§6.9), `next-u` (§6.9) and `wrap64` (§7.8) compile on JavaScript, and agree
with BigInt on 7,533 checks across U's edges.

## Measured

node, 1,000,000 calls, best of 7, two runs:

| | the pair (built) | BigInt throughout |
|---|---|---|
| `next`, n + 1 | 36.3, 38.2 ns | 11.4, 11.8 ns |
| `step`, an FNV step mod 2⁶⁴ | 65.2, 64.6 ns | 27.6, 28.4 ns (`BigInt.asUintN(64, …)`, hand-written) |
| `spin`, 100 FNV steps, 100,000 calls, best of 5 | 414, 420 ns | 422, 383 ns |

The boundary costs about 25 ns a call: two BigInt operations in, a BigInt built out. So a function
that does one or two operations on the value is 2–3× slower as a pair than as a BigInt, and one that
does a hundred ties, as u64repr-2026-09-22 measured the pair and `asUintN` tied on multiply-add (40
against 42 ns) on the same Node; the pair wins 4.8× against plain BigInt, what the rung emits, and
4.3× on division. BigInt throughout is not the compiler's option B either: the rung holds no residue,
so `step` and `spin`, mod 2⁶⁴, would be refused there; the hand-written `asUintN` is the bound.

## What failed first

- **A parameter split as a residue**: since ADR 0151 a ring operation's operands are read by their
  residues, and a parameter past W, not a pair, took the residue case, `(x >> 32) & m` and `x & m`,
  as for an ℤ value held in a word; the realization rule refused the `>>` of a value past W, which is
  what kept it from computing on a rounded Number.
- **A planted fault lost its witness**: `TestIntIsTheIntegersAtTheBoundary` item 2 (ADR 0147: an
  export's bounded parameter above JavaScript's word is a BigInt) planted the word at the boundary,
  `PlantBoundedAboveTheWordIsTheWord`, and wanted `wide(n ∈ 0..2^63-1)` refused; with the entry pair
  built it compiles, split into the digits and put straight back, a round trip. The witness is now
  the text: no split without the plant, the split with it.
- **The book's export name**: a chapter's programs are named after the chapter (`integersNext`).

## Paid and bought

| | |
|---|---|
| compile time | 0.97× and 1.03× the baseline (`cmd/check`'s gate, two runs, medians of 113 and 129 compiles over 300 ms), no suspect |
| run time | a program accepted before: unchanged. One refused before: the boundary's ≈ 25 ns a call (above) |
| code size | unchanged for every program in the corpus; the book's three programs, outside it, are new on JavaScript |
| emitted text changed | none: the emission byte-identical to the baseline on every target (no program of the corpus has an entry parameter in U past 2⁵³ on JavaScript) |
| proofs | unchanged: 12,766 of 12,820 operations, 1,607 of 1,908 loops |
| refusals changed | accepted now on JavaScript: an entry point's parameter in U past 2⁵³ (the book's `midpoint`, `next-u`, `wrap64`) |
| compiler lines | +66 −10 (`ir/pairs.go` +62 −7: `entryParams`, the head; `emit/opnames.go` +4 −3; `targets/js/builtin.oro` +6); tests +66 (`cmd/gen/pairentry_test.go`) and one witness restated; the book +16 |
| rule made less general | none |
| not built | the choice at an entry point by cost: a parameter whose function does a few operations on it is cheaper in ℤ (BigInt) than through the pair's boundary, where the rung can hold it (not a residue); owed with its measurement |
