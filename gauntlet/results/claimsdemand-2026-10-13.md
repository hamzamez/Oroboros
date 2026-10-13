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
missing proof.* Every clamp the corpus documents, each removed in a scratch copy and recompiled on Go:

| clamp | what it guards | with it removed | the missing fact | its theory |
|---|---|---|---|---|
| `json/tokenize.oro`, the popped stack index `(if (< sp 1) 0 (- sp 1))` | `stk[sp − 1]` | **compiles**: 116 of 116 and 29 of 29 operations bounded | none: the outer guard `sp ≥ 1` is used now | — |
| `big/limbs.oro`, `at` | `t[i]` with i under another table's bound | **compiles** once the export states `(k (int 0 63))`, writable today | none inside; the export's parameter range | — |
| `io/freq.oro`, `cl`, a word's byte offset | `src[beg + k]`, k < end − beg | refused: `(+ (sp (* 2 u)) k1) < (len src)` not proven | ∀w. sp[2w] ≤ sp[2w+1] ≤ len(src) | the array property fragment (Bradley, Manna and Sipma 2006): one universal, a strided pair, a frame term |
| `io/freq.oro`, `slen`, a span's length | d = end − beg in [0, cap) | refused: an allocation sized by d may be negative | the same: beg ≤ end | the same |
| `kara/core.oro`, `c4` (three uses) | `out[i + j]`, i, j < desc[2] | refused: `i + j < len out` not proven, `desc[2]` opaque | desc[2] = 2, desc built by three stores | read over write (McCarthy 1962), the quantifier-free theory of arrays |
| `kara/core.oro`, `pack` | `acc·100 + p[i]` | refused (it is the program's third unproven operation) | every product limb is under 100 | an invariant of the multiply loops over `out`'s contents |

**Every missing fact is about the program's own construction, and every one is in a decided theory.**
freq's is an F-D₁ content fact (array-facts.md), but relational between a stored pair's two
components and a frame term, which one abstract cell per component (smashing) cannot hold: the spans
loop stores (i, word-end(src, i)) with i ≤ word-end ≤ len(src), so Houdini over candidates of that shape
would find it, as it finds today's content facts. kara's is the content of a literal store chain. Two
clamps are stale: the analysis has grown past them.

## 4. What it decides

1. **No program in the corpus demands a programmer-written claim.** What its clamps lack is
   derivable: settypes' rule *prefer deriving a fact over declaring it* (CLAUDE.md) applies, and content
   facts are derived, never declared (array-facts.md). An `assert` or invariant written by hand would
   state what the analysis should find.
2. **What is demanded is derivation**, the analysis growing for refusals programs name:
   - relational content facts, Houdini over pairs of a strided table's components and frame terms
     (freq's two clamps);
   - read over write on a store chain of known indices (kara's `c4`);
   - and two stale clamps to delete (`tokenize`, `limbs`), which changes emission and, for
     `tokenize`, a gauntlet program.
3. **Claims proper are demanded only by specifications**, and a specification is never needed to
   compile. Whether programs should state and prove their correctness (a sort's output sorted and a
   permutation of its input, a search's failure meaning absence: surface-design's G3–G5) is a choice,
   not a measurement, and the program-first rule cannot make it.
