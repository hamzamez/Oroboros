# 0026 — An `int` is an integer, and each target realizes what it can

Date: 2026-09-22
Status: Accepted. **Supersedes the part of [ADR 0012](0012-portable-integer-range.md) that made one
interval, ±(2⁵³−1), the language's `int` on every target.** What 0012 chose for meaning — exact
integers — stands. **Amends [ADR 0019](0019-precision-by-declaration.md)**: its `word` is the
target's word, not the window.

Research: [what-an-int-is.md](../what-an-int-is.md).

## Context

`encoding/hex`'s `EncodedLen` carried a precondition, `|n| ≤ 2⁵²−1`, that Go does not have. Asking
why led to hamza's question: *if we add a target whose integer is 32 bits, do we change the window
again?*

The window was never a fact about integers. ADR 0012 measured three hosts and took their overlap:

```
W(S) = ⋂_{T ∈ S} Exact(T)          S = {go, java, js}
```

An intersection is **antitone** in S — adding a target can only shrink it. So a new target leaves
two answers, both wrong: shrink W, and every existing program's legality changes because a target
was *added*; or keep W, and 2⁵³ is revealed as JavaScript's float mantissa frozen into the language
on 2026-08-15. C's `int`, Haskell's `Int`, Ada's `Integer` and Swift's `Int` each made a type's
extent depend on the platform, and each patched it with explicitly sized types.

Measured in the research:

- **At a host boundary the window is unsound for results.** A declaration `A → B` abstracts a host
  function `A' → B'` iff `A ⊆ A'` and `B' ⊆ B` (contravariant in arguments, covariant in results —
  Cardelli). Narrowing a 64-bit *parameter* to W is sound; narrowing a 64-bit *result* is a lie. 206
  of Go's 4,867 platform-independent functions return an explicitly 64-bit integer, among them every
  parser in `strconv` and the 64-bit half of `encoding/binary`.
- **The window's size is invisible to the programs.** Every compile re-run with W widened to ±2⁶²:
  456 of 456 byte-identical, 2,361 of 2,413 operations proven, unchanged. The proofs are about
  whether a value is bounded, not where the bound sits.

## Decision

hamza chose D, and **portability is a property the compiler reports**.

1. **Meaning is ℤ.** `int` is the ring (ℤ, +, ·, <). There is no window in the language.

2. **A type is an interval** `I ⊆ ℤ ∪ {±∞}`, ordered by containment, as the analysis already
   computes (Cousot & Cousot 1977). `(int LO HI)` names a set.

3. **A target realizes a sub-lattice of intervals, by containment.** It declares its realizations
   as data, and a type's representation on T is the least realization containing it:

   ```
   Rep_T = { (I_r, r) }           ρ_T(I) = the least r with I ⊆ I_r, if one exists
   ```

   Its **word**, `word_T`, is the widest realization native arithmetic runs on. It is data in the
   target file, never a constant in the compiler:

   | T | `word_T` |
   |---|---|
   | go | `int64` = [−2⁶³, 2⁶³−1] |
   | java | `long` = [−2⁶³, 2⁶³−1] |
   | windows | signed qword = [−2⁶³, 2⁶³−1] |
   | js | integer `Number` = [−(2⁵³−1), 2⁵³−1] |

4. **Legality is per (program, T).** ADR 0019's rule holds with W replaced by `word_T`: an operation
   whose operands are word-represented is refused on T unless its result is proven inside `word_T`.
   Bounded by default stays, and so does its loudness.

5. **`big` only by declaration.** A declared range not contained in `word_T` is arbitrary precision
   on T. `(int 0 (pow 2 62))` is a word on Go and a `BigInt` on JS: the representation differs, the
   integer does not, and the difference is visible because someone wrote the range.

6. **The invariant that replaces the window:**

   > **Every target that accepts a program computes the same element of ℤ for every term.**

   This is what W was for, and it no longer depends on the target set. Adding a target adds a row to
   the table in (3) and changes no other target's legality or meaning.

7. **Constant folding folds within `word_T`** (ADR 0009, per target). Compile time and run time
   still agree, because a fold happens exactly where the target's run time would compute the same
   integer.

8. **Portability over S is computed and reported.** A program is portable to S iff every T ∈ S
   accepts it. `W(S) = ⋂ word_T` survives as a derived number, reported, never assumed. A refusal on
   T names T's word and the targets on which the same program would be accepted.

9. **At a host boundary an integer type is the host's own**, stated as the range it is. Each target's
   root module names them as manifest types (`go.int64 = (int -9223372036854775808
   9223372036854775807)`), and a generator spells a host `int64` as `go.int64`, which it can justify
   (ADR 0023). So `EncodedLen`'s precondition becomes Go's own overflow boundary, −2⁶² ≤ n ≤ 2⁶²−1.

10. **A target may realize more than one interval at word width.** [0, 2⁶⁴−1] is native on Go
    (`uint64`) and windows (unsigned qword). Emission rests on one theorem: the quotient map
    ℤ → ℤ/2⁶⁴ is a ring homomorphism, and each 64-bit realization is a set of representatives of
    ℤ/2⁶⁴. So `+`, `−` and `·` may be computed in any 64-bit type with wrapping and read back in the
    result's realization, provided the result lies in it — the result is proven in its range, so the
    residue class names exactly one member. Order is not preserved by the quotient, so `<`, `/` and
    `%` need a realization containing **both** operands. The JVM's and V8's choice for this interval
    is a measurement ([u64repr-2026-09-22](../../gauntlet/results/u64repr-2026-09-22.md)), not part
    of this decision.

## Why not

**A — keep W and add a 64-bit rung for declared wide ranges.** It fixes the survey's lie, but by
making every 64-bit host value `big` or a special case, and it keeps W frozen at 2⁵³. The int32
question has the same two bad answers it had before.

**B — `int` := the target's word everywhere (Reading 2, literally).** Right at the boundary, where
a host's `int64` is the host's integer. Inside a program it makes one source mean ±2⁶³ on Go and
±2⁵³ on JavaScript: targets disagreeing observably on the language's most basic type, which is the
project's own definition of Tier 2. D keeps what B gets right (9) and keeps the meaning ℤ elsewhere.

**A larger constant window — ±2⁶³, or ±2³¹ to be safe for any future target.** 2⁶³ everywhere makes
JavaScript emulate 64-bit integers for every `int`, which lowers below what the host provides and
breaks rule (3)'s point. 2³¹ makes every legal program pay, today, for a target nobody has asked for,
and a 16-bit target would move it again. Any fixed constant is W(S) for some imagined
S, and antitonicity is exactly the objection.

**Portability by construction, with a required `(portable …)` claim.** P2 (independence from the
target set), P3 (a host word is held in a host word) and P5 (portability by construction) cannot all
hold: P5 needs one interval every target realizes natively, fixed in advance, which P2 forbids
relative to today's targets and which, over every possible target, is either tiny or emulated. Today
kept P3 and P5 and paid with P2. hamza chose P2 and P3, and turned P5 into a report — ADR 0001's own
stance on portability, applied to integers. A declared `(portable …)` claim that the check enforces
stays available as a later opt-in; it is not the default.

**Wrapping semantics.** Every proof the compiler makes is a theorem about ℤ. The homomorphism in
(10) is an *emission* technique, legal only where the result is proven in range; it is never the
meaning.

**A separate unsigned type family.** A range already says unsigned: `(int 0 18446744073709551615)`.

**A silent bignum** (ADR 0019's A). Unchanged: `big` is something a programmer writes.

## Consequences

- **The int32 question is answered**: a target is a row, and no existing program moves.
- **Portability becomes a fact about a program that can change as its values grow.** A library whose
  values are proven in (2⁵³, 2⁶³] compiles natively on Go, Java and windows and is refused on
  JavaScript, with a refusal that says so. `cmd/check` compiles every source on all four targets, so
  such a change is seen. §3.4 of the research measured what giving up portability by construction
  costs on today's corpus: nothing.
- **The analysis must represent bounds to 2⁶⁴ exactly**; it saturates at 2⁶² today. 128-bit bound
  arithmetic, gated for compile time.
- **Host declarations become honest at results.** `strconv`'s parsers and `encoding/binary`'s 64-bit
  half become declarable as what they are.
- **Refusal texts name the target's word** instead of ±(2^53−1): an accepted change to the baseline.
- **Every document that says "the portable window" as a language constant is stale**, and is
  corrected as the build reaches it.
