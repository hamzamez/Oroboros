# 0152 — The one set is checked where it claims, and fixed limbs hold a program within its width

Date: 2026-11-21
Status: Accepted (hamza, 2026-11-21, on the options for JavaScript's three gaps: "I agree, go in that
order: 2, then 1, then 3"; this is 3, option A: "check at the claims, store by the join"). Amends
[ADR 0029](0029-above-the-word-one-set-on-every-representation.md) (the program's one set enforced
at every operation of the rung) and completes
[ADR 0150](0150-a-set-is-a-claim-and-rho-alone-chooses.md) for a program that declares a set above
the word.

## Context

ADR 0029 made the program's declared sets above the word one set, closed by sign and bit length, and
enforced it on every representation alike: the fixed limbs trap where a value passes their width,
so the host's integer was made to trap at the same place, a fit (`big-fit`) after every operation of
the rung. ADR 0150 then made a declared set a claim and an intermediate no set claims ℤ's. The two
met where a program declares a set above the word and computes an intermediate past it: `x * 3 / 4`
over S₆₄ on JavaScript, where S₆₄ is above the word, stopped with "the declared range is too small",
its value proven and its answer in S₆₄; Go, the JVM and windows hold x·3 in the double word and
answer. The book's `a * b * c % 7` (§6.9) stopped so past 2⁶⁴, and `double-word` skipped JavaScript.

Two repairs failed first (pairsel-2026-11-21): raising the width to every proven fact lost
`big-sign`'s stop, a yield past its declared −2²⁰⁰..2²⁰⁰ printed on every host; fitting only an
export's own yields lost it again once the definition was inlined. One width did two things, the
representation's room and the claims' check, and after inlining a claim could not be told from an
intermediate. It can: a result declared above the word is ascribed its set at load
(`ascribeResult`, core/reduce.go), and the ascription `the(v)` survives inlining to the rung.

## Decision

1. **The one set is checked at its claims.** Where a value held exactly is ascribed a declared set
   above the word, the ascription is its fit: `the(x)` becomes `big-fit(x, k)`, k the program's one
   set (ADR 0029's, unchanged), before the ascriptions are erased (`fitClaims`, ir/big.go).
2. **And where nothing else bounds a value.** An operation of the rung whose result has no finite
   fact keeps its fit at the one set: the declared set is the only consent to its room, and fixed
   limbs of that width would stop it there too.
3. **An operation with a finite fact is not fitted.** Its fact is a theorem; where no set claims the
   value it is ℤ's (ADR 0150), and where one does, item 1's fit checks it.
4. **Fixed limbs hold a program only within their width.** Their instances check the width at every
   operation, which agrees with items 1–3 exactly when no finite fact of a value the rung holds
   passes it: so where an operation A holds exactly (D_A) has a finite fact past the width, or a
   negative one under an unsigned width, the program leaves fixed limbs for the host's integer, or
   K(ℕ\*) where the target has none, as a program with an operation no limb instance has does
   (`limbsBeyondWidth`, ir/limbs.go). Both check at a site.

## Why not

- **The width raised to the join of the facts** (exactWidth, pairsel-2026-11-21): the same width is
  every claim's check, so a yield past its declared set passed it; `big-sign` answered, a wrong
  answer on every host.
- **A fit only at an export's own yields**, followed back through joins: sound before inlining, and
  after it the claimed yield of an inlined definition is a value like any other; `big-sign` answered
  again.
- **Refuse at compile time where an intermediate's fact passes the width**: honest, and a refusal of
  a program every other target compiles, the kind ADR 0151 removed.
- **Each claim at its own set's width** rather than the program's one set: stricter and more exact,
  and a change of ADR 0029's meaning beyond this step (a value claimed in 0..2¹⁰⁰ under a program
  whose one set is 2²⁰⁰ would stop where it answers today); kept as a candidate, owed to a program
  that meets it.
- **A bound check on fixed limbs at a claim**: a new instance of the limb library, its theory checked
  once (ADR 0035), where leaving fixed limbs for a representation that already checks at a site
  costs nothing new.
- **Keep the stop** (option C): a program that stops with its value proven, where three targets
  answer.

## Consequences

- `x * 3 / 4` over S₆₄ and the book's product of three answer on JavaScript; `double-word` runs on
  four targets, its skip removed; `big-sign` still stops at its claim, on every target.
- `big-negmid`'s expectation changes: its negative intermediate under a non-negative set is ℤ's, and
  the answer `d + b = a` is in the set, so its third input answers where ADR 0029 stopped it. This
  is ADR 0150's law, now applied where a program declares a set.
- Programs that declare a set above the word lose the fits on operations with finite facts
  ([oneset-2026-11-21](../../gauntlet/results/oneset-2026-11-21.md)): 27 emitted texts, 3,893,601 →
  3,658,527 bytes, the hosts' mostly 2–5× smaller; windows' big-bits 19 K(ℕ\*) fits fewer, and with
  them 171 counted operations and 19 loops, the corpus's counts 12,595 of 12,649 and 1,588 of
  1,889.
