# 0111 — Printing is a tree of divisions, each node Algorithm D in place

Date: 2026-11-15
Status: Accepted (hamza, 2026-11-15: "go ahead with printing, always mathematical and algebraic";
"go ahead with the in-place D, always mathematical and algebraic").
Extends [ADR 0095](0095-text-from-ascii-is-the-inclusion-and-rendering-is-integers-at-every-sort.md)
(render per representation) and [ADR 0106](0106-k-n-star-is-in-base-2-64-the-machines-digit.md).

## Context

On windows `fact(20,000)` took 97 ms against Go's 35, and about 65 of them were printing. K(ℕ\*)'s
render was repeated short division by 10¹⁸, n²/2 two-by-one divisions on one latency chain (`divq`,
12 ns a step, carry-2026-11-13). [radix-research.md](../radix-research.md) gives the algebra and a C
model: the tree of divisions with Algorithm D at each node is 9.8 ms, repeated short division 33.

## Decision

1. **Printing is ε_β⁻¹ ∘ ε_B by a tree of divisions**, from 32 digits (repeated short division
   below):
   - β = 10¹⁸, the largest power of 10 a constant divisor of the double word takes (≤ 2⁶²,
     ADR 0103);
   - with P_j = β^(2^j), x = q·P_j + r splits x's base-β digits in two, so the tree is balanced, its
     shape a function of x's size alone, and a loop over levels (ADR 0014);
   - level L is one table of 2^L slots (no table of tables, R1e), each node below P_{J−L}²;
   - the powers come by squaring (ADR 0110), the last level splits by β with one 2-by-1 division a
     node, and the text is 18 characters a base-β digit, then ι (ADR 0095).
2. **Each node is divided by Algorithm D in place.** A node u below P_j² ≤ P_j·Bᵏ (k = size(P_j))
   normalizes to u′ = u·2ˢ < v·Bᵏ, v = P_j·2ˢ. So:
   - u′'s word 2k is 0, the quotient has k digits, and the window of step jq is U[jq .. jq + k];
   - after a step the window is below v < Bᵏ, so its top word is 0, and the quotient digit is stored
     there: the slot ends as remainder·2ˢ in words 0 .. k − 1 and the quotient in words k .. 2k − 1;
   - D4 leaves the window in (−2v, v), negative exactly when its top word is not 0 in k + 1 words of
     complement, so D6 adds back while it is (at most twice, Theorem B);
   - q̂ lives in word 2k while D6 counts it down, (q̂ − 1) mod B: a digit in [0, 2⁶⁴) is no `Int`,
     so no loop variable carries one.
   No step allocates.
3. **A node starts at its first nonzero window.** With L significant words, u′ < B^(L+1) and
   v ≥ B^(k−1)·2⁶³, so q < B^(L−k+2): the steps above jq₀ = min(k − 1, L + 1 − k) have a zero window,
   q̂ = 0, and their digit's word is already 0. The tree is sized by the power of two above x's base-β
   length, so without this the top node and every quotient below it ran their full k steps on zeros
   (29 M row steps for `fact`'s 8 M).
4. **Every index of the level loop is linear in the slot's base**, sb = j·w₂ carried: sb + w₂ ≤ n,
   jq ≤ k − 1 and w₂ = 2k + 1 put the window inside the buffer, and k ≥ 2 (P_j ≥ β² > B for j ≥ 1)
   is the loop's guard, a lemma's premise. The reads are then unconditional.

## Why not

- **Repeated short division**: the measured 3.4× of the C model, a latency chain no rewrite cheapens.
- **The library's Algorithm D at each node** (`div`'s steps, `qhat`, `msub`, `fix1`): built first,
  correct, and slower than chunking at 20000! (155 ms against 95), since its window is a table born
  per quotient digit (16.7 ms for one 4,000-by-2,000-word division in the language).
- **A guarded or clamped read of the live buffer**, `if i in range { o[i] } else { 0 }`, or the index
  clamped first: the reducer sequences an impure read under a conditional into each branch, its
  continuation copied (case-of-case, `core/reduce.go`). D3's three reads made 27 copies of the step,
  66 KB of residual where all of `div` is 8 KB, past the reducer's fuel. The linear index of item 4 is
  a proof, not a clamp, and the residual is 4.5 KB.
- **The quotient digit carried as a loop variable**: [0, 2⁶⁴) is the unsigned word, which no loop
  variable may be. Word 2k is the place algebra leaves free.
- **D6 as a loop after D4's row in the same arm**: two loops on one buffer in sequence do not reduce
  within the fuel; the add-back is an arm of the level loop.
- **Newton's reciprocal and Barrett's reduction, or Burnikel and Ziegler's recursive division, now**:
  two products a node at 3× Go's product (kernels-research.md §7) are no better than D. They are the
  subquadratic step once multiplication is faster, the postponed design work (CLAUDE.md).
- **The rule transfer's ⊥ for a body that never yields**: the allocating version needed it, a rule
  built in the level loop's arm before that arm's facts arrived putting ⊤ into the loop's table. The
  final form masks every read of a level table by its digit's invariant, so no program names it, and
  the analysis stays as it was.

## Consequences

- On windows, medians on a quiet machine (radix-2026-11-15): `fact(20,000)` 97 → 46 ms against Go's
  35, its printing about 65 → 14 (the C model 9.8); fib(10⁶) printed 502 → 106 against Go's 29, its
  printing about 473 → 77; fib(10⁵) 36 → 32 against 28.
- `big-render`, a differential case on four targets: 32 to 158 digits, 10ᵏ, 10ᵏ ± 1, P₆ itself,
  β¹²⁸ − 1, 2²⁰⁴⁸ ± 1, powers of 3 and 7, zeros inside, a negative value, against Python's text.
- `PlantTreeNoAddBack` drops D6. A node c·P_j − 1 is c·v − 2ˢ, and P_j's low 18·2ʲ bits are 0, so
  the borrow dies below the window's top words and the last step's q̂ is q + 1. Exactly the three
  inputs with such nodes (8, 32 and 9 add-backs, counted by a model of the tree) go wrong
  (`TestPrintingIsATreeOfDivisions`).
- Printing stays quadratic, about 5× Go's subquadratic `String` at 4,000 words and 6× at 10,850;
  the gap is Go's recursive division over Karatsuba, after the kernels.
