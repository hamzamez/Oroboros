# 0108 — The full adder is the triple's, and a fold over a split domain is two loops

Date: 2026-11-13
Status: Accepted (hamza, 2026-11-13: "go ahead", the next step recommended: the sum's carry kept in the
flags). Extends [ADR 0103](0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md)'s
double word and [ADR 0104](0104-k-n-star-is-in-base-2-63-the-largest-base-the-decision-proves.md)'s
use of the triple's 2-by-1 division to the triple's add-with-carry.

## Context

After ADR 0107, `fib(10⁵)` was 1.9× Go's process, and its time was the sum: 10⁵ additions of numerals
growing to 1,085 digits, 5.4·10⁷ digit steps. Knuth's A step is t = x + y + c, read back by its halves
t & (B − 1) and t >> 64 (B = 2⁶⁴). In the double word that is `add i128` of zero extensions, and its
carry goes through a register on every digit: `setb`, a zero extension, and an add back.

Measured in C on the same 5.4·10⁷ steps (scratchpad `div/b.c`, three runs across the machine's speed
states), ns per digit:

| form of A | double word (i128) | the machine's carry (`_addcarry_u64`) |
|---|---|---|
| one loop, a clause testing i < my at every digit (what K(ℕ\*) was) | 0.67–0.71 | 0.66–0.71 |
| two loops in sequence, [0, my) then [my, mx) | 0.66–0.71 | **0.33–0.35** |

LLVM's target-independent forms, the `uadd.with.overflow` pair and `__builtin_addcll`, measured 0.88
even in two loops: LLVM 23 does not combine them into `adc`. **Only both changes together pay**, and
each alone gives nothing. With one loop, clang merges the two clauses into one body that tests i < my
at every digit, and the carry has to leave the flag across that branch. The double word moves the
carry through a register either way.

## Decision

1. **A fold over a split index domain is two loops in sequence**: [0, mx) = [0, my) ⊔ [my, mx), and a
   fold over a concatenation is the composite of the folds. The fold is a monoid homomorphism from
   (List, ++) to (End(S), ∘), its state the carry, or the borrow and the size so far. K(ℕ\*)'s A and S
   (`addlp`, `sublp`) are written so (`emit/natnum.oro`), the second loop a call in the first's exit
   arm (`addhi`, `subhi`), borrowing the buffer. That is the composite in tail position, since a buffer
   a loop yields with a value is frozen (ADR 0031).
2. **The full adder is the triple's.** φ : t ↦ (t mod B, ⌊t/B⌋) on [0, 2B) and t ↦ (t mod B,
   [t < 0]) on [−B, B) are bijections onto [0, B) × {0, 1}, and adc(c, x, y) = φ(x + y + c),
   sbb(b, x, y) = φ(x − y − b) for x, y ∈ [0, B) and c, b ∈ {0, 1}. `llvm/isa.add-carry` and
   `sub-borrow` are declared in `targets/windows/x64.oro` (`llvm.x86.addcarry.64` and
   `subborrow.64`), beside `divq`.
3. **The printer chooses it where IR_P's types prove its premise** (llvm.md §3d, `ir/llvm/carry.go`):
   - a + or − of the double word whose leaves are words in [0, B) and one bit in [0, 1];
   - every inner + or − is used by it alone;
   - every use reads it by φ: t & m for a word m, which is (t mod B) & m since & is 2-adic, then
     t >> 64 of a sum, t < 0 of a difference.

   The IR is unchanged and W14 holds of it as before. The choice is a function of IR_P's text, as `nuw`
   is, so a reprint from the text chooses the same.

## Why not

- **The two loops alone**, in the double word: measured no faster (0.66 against 0.67). The step's
  latency is the carry's path through a register, so splitting the domain does not shorten it.
- **The machine's carry in one loop**: measured no faster (0.66–0.71). clang sinks the two clauses'
  adders into one, so the carry leaves the flag across the per-digit branch. In fib's emitted code the
  hand-edited adc gave 51 against 51 ms.
- **LLVM's target-independent `uadd.with.overflow`**, declared in Σ_LLVM for both triples: measured
  0.88 ns, slower than the double word. LLVM does not combine the pair into `adc`, so the triple's
  own operation is the realization, as `divq` is for the 2-by-1 division.
- **Inline assembly** (`adcq`, as `divq` is written): an `asm` clobbers the flags, so the carry could
  not stay in them from one step to the next, which is the point.
- **A new IR operation for the full adder**, chosen on the IR as SelectDouble chooses: every printer
  would have to print it, and only LLVM has the double word. Its premise is in IR_P's types already,
  as the 2-by-1 division's dividend halves are read by the printer.
- **The carry crossing the two loops in the buffer** (written at index my and read back): a digit
  slot's fact is [0, B), the table's element type, so the carry's {0, 1} would be lost and the second
  loop's adder refused. A mask would recover it only as a clamp. The carry crosses as an argument.
- **Index-set splitting by the compiler** (Wolfe 1996, *High Performance Compilers for Parallel Computing*): a loop whose clauses are guarded by a
  monotone predicate in the index split into loops. Only one library writes such loops, so the library
  writes the split. It waits on a program that writes one.

## Consequences

- `fib(10⁵)` 1.9× → 1.23× Go's process (162 → 108 ms against 88, carry-2026-11-13); the product, the
  quotients and `fact` unchanged (0.94×, 0.87×, 2.7×).
- Every windows `.ll` with K(ℕ\*)'s sum or difference prints `addcarry`/`subborrow`; the proof counts
  are unchanged.
- Two planted faults: a bit leaf in [0, 2] (`PlantAdderFault`), and a shift other than 64 read as the
  carry (`PlantAdderReadFault`). Each gives a wrong answer, which `cmd/build/adder_test.go` sees.
