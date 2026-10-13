# claimsdemand-2026-10-13 — what the corpus lacks that a claim could supply

hamza, 2026-10-13: *"do item 2, always mathematical and algebraic"*. Item 2 was claims on trees:
`assert`, the loop invariant, `spec` and `lemma` in today's notation, with ε and 𝒪 on abstract.md's
trees, decided by the Omega test. Its trigger was named with it: **a program refused today that an
invariant or lemma would let through.** This document is the search for that program. Nothing in the
compiler changed.

## 1. The question, in the algebra

A claim enters the logic in one of two ways (settypes §6.3, ADR 0062):
- **a fact the analysis cannot derive**, stated by the program and proven, then used: `assert φ`, an
  invariant, a lemma's conclusion. It is needed exactly where the fact is true but outside what the
  decided procedures derive (settypes §5's cliff: nonlinearity, quantifier alternation, induction);
- **a specification**, a postcondition or invariant stating correctness. No program needs one to
  compile; it is a choice to prove more.

The first kind is demanded by a program, and is what was searched for.

## 2. The refusals

The 256 refused runs of `cmd/check`'s sweep (`gauntlet/check/outcomes.txt`), by cause:

| cause | runs |
|---|---:|
| a name the target lacks (a host function, a portable layer's provides) | 202 |
| a target fragment or a library's provides file compiled as a program | 38 |
| no bignum on windows (`examples/big/*`) | 4 |
| a type no signature fixes (`string-escapes`, `utf8-widths` on two targets) | 4 |
| a discarded relevant result (`examples/errors/discard.oro`, meant to be refused) | 3 |
| an index in `lib/result.oro`'s `err-map` on windows | 1 |
| **arithmetic not proven in the word** (Go only) | **4** |

The arithmetic refusals are `examples/int/collatz`, `fib` and `power` (meant to be refused: the values
are unbounded), and `examples/kara/core.oro` (§3). **No refusal is a true fact the analysis fails to
derive**, so no refused program is lifted by a claim.

## 3. The clamps

A program written around a wall does not show up as a refusal. CLAUDE.md: *a clamp in a program is a
missing proof.* Every clamp definition in the corpus (`grep` for `(if (< x 0) 0 (if (>= x …) 0 …))`, and
the tokenizer's inline one), each removed in a scratch copy, by use site where a definition has several,
and compiled on Go. `tally` is left out: it is refused in the sweep for want of its generated host layer.

**Stale: the analysis proves the index without the clamp.**

| clamp | uses |
|---|---|
| `json/tokenize.oro`, the popped stack index | 1 |
| `big/limbs.oro`, `at` (with `(k (int 0 63))` on the export, writable today) | all |
| `differential/cases/big-limbs.oro`, `at` | all |
| `differential/cases/json-tree.oro`, `cn`, `cd`, `cw` | all |
| `differential/cases/merge-sort.oro`, `cli` | all |
| `io/freq.oro`, `cli` in `iota`, `putnum`, `putword` and the line layout | 5 |

**Live: a fact is missing, and each is in a decided theory.**

| clamp | failing obligation | the missing fact | its theory |
|---|---|---|---|
| freq `cl`, a word's byte offset | `sp[2u] + k < len src` | ∀w. sp[2w] ≤ sp[2w+1] ≤ len(src): a relation between one stored pair's components | the array property fragment (Bradley, Manna and Sipma 2006) over the curried table Π_{Fin n} ℤ², with reads as atoms |
| freq `slen`, a span's length | an allocation sized by end − beg | the same, in the IR's interval domain, which an allocation's size is decided in | the same, in a second analysis |
| freq `cli` in `pat`, under `new?` | `0 ≤ k − 1` | k ≥ 1 from k ≥ 0 and the guard ¬(k = 0) | Presburger with a disequality: over ℤ, k ≥ 0 ∧ k ≠ 0 ⊢ k ≥ 1; Fourier–Motzkin reads no ≠ |
| freq `cli` in the merge | `j < len a` | i + j − k = mid, an equality invariant of the merge loop, and len a = n through `msort`'s buffer swap | linear equalities (Karr 1976) and the length equation across a swap |
| freq `cli` in `dists` | `0 ≤ 2(d − 1) + 1` | d ≥ 1 once k ≥ 1: the first word always opens a group | a disjunctive invariant, k = 0 ∨ d ≥ 1, convex after peeling the first iteration |
| kara `c4` (three uses) | `i + j < len out` | desc[2] = 2, desc built by three stores at literal indices | read over write (McCarthy 1962) |
| kara `pack` | `acc·100 + p[i]` in the word | every product limb is under 100 | a bound on contents that grows with the passes: geometric accumulation, deliberately not next |

**Every missing fact is about the program's own construction, and every one is in a decided theory.**
Four of the seven are one step from what the refinement layer already does (a disequality, an equality
template, a length equation, peeling); freq's record fact is F-D₁ (array-facts.md) with φ over a record's
components jointly, which smashing per component cannot hold. kara stays refused whatever is derived:
its `pack` needs geometric accumulation.

## 4. What it decides

1. **No program in the corpus demands a programmer-written claim.** What its clamps lack is
   derivable: settypes' rule *prefer deriving a fact over declaring it* (CLAUDE.md) applies, and content
   facts are derived, never declared (array-facts.md). An `assert` or invariant written by hand would
   state what the analysis should find.
2. **What is demanded is derivation**, the analysis growing for refusals programs name, in the order of
   cost: the stale clamps deleted (emission changes, and `tokenize` is a gauntlet program); a negated
   equality read as the disjunction a < b ∨ a > b with an infeasible side dropped; equality templates
   and the length equation across a swap; the first iteration peeled; record facts with reads as atoms,
   in the refinement layer and then the IR's. Read over write waits: kara would still be refused.
3. **Claims proper are demanded only by specifications**, and a specification is never needed to
   compile. Whether programs should state and prove their correctness (a sort's output sorted and a
   permutation of its input, a search's failure meaning absence: surface-design's G3–G5) is a choice,
   not a measurement, and the program-first rule cannot make it.
