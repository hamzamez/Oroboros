# 0028 — A definition's declared parameter range and `where` are obligations at its calls

Date: 2026-09-24
Status: Accepted. **Reverses the decision recorded in [refinements.md §6b](../spec/refinements.md)**
that a definition's `where` is dropped at its calls, and extends it to declared parameter ranges, which
had been dropped silently. An export's contract is still *assumed* when its caller is outside the
program, as §6b said.

Measurement: [requires-2026-09-24](../../gauntlet/results/requires-2026-09-24.md).

## Context

Reduction inlines every non-exported call, so by the time anything is analysed a definition has no
call site left. refinements.md §6b drew the consequence for `where`: dropped, and argued it was safe,
because the body's own obligations land on the caller's values. That is true for **safety**, and it is
why no program was ever told something false. It is not true for **meaning**: a total body computes
something else outside its declared domain, and nothing fires.

```lisp
(sig digits18 ((g (int 0 999999999999999999))) string)
(digits18 1000000000000000005)          ; compiled, and printed 000000000000000005
```

§6b named the gap for `where` (`print-int -13` printing a blank line). Parameter ranges fell into it
too, unremarked: 25 of the corpus's 50 signatures on non-exported definitions put a range on a
parameter, against one `where`.

[inlining-and-declarations.md](../inlining-and-declarations.md) §3 stated the principle behind §6b:
*"a fact declared at a boundary is redundant once the boundary is gone"*, because inlining gives
strictly more information. That holds for a declaration read as a **premise**, something the body may
rely on, which the caller's concrete values re-derive. A declared parameter range is also an
**obligation on the caller**, and inlining does not make an obligation redundant. It removes the
only place it could be checked, and nothing re-derives it.

A range is a type (ADR 0003), and a type in a refinement discipline is checked at application: to
apply `f : {x | x ∈ I} → R` to `e` is to prove `e ∈ I` (Liquid Haskell, F*). β is application
carried out, and it preserves typing only for an application that was checked. A declared range that
no call is checked against is a comment claiming meaning, which ADR 0024 rules out for comments.

## Decision

**1. At every call a definition is inlined into, each declared parameter range and the definition's
`where` are obligations,** discharged in the caller's context. That covers exported definitions called
from inside the program too. From outside, an export's contract is assumed.

**2. Where each obligation is examined:**
- A range is checked wherever the argument's value reaches the body: at each use of the parameter, or,
  when β binds the argument to a name (because it is impure or shared), at the binding, which is the
  call. A parameter the body never uses carries no obligation, since no computation relies on it.
  A literal argument is decided when the call reduces.
- A `where` is checked at the call, over the arguments as β passes them.

**3. Discharge, in order:**
- the literal decision;
- the interval analysis;
- the refinement layer, which reads the facts in scope, including a pure call's `ensures`
  (theories.md §7.9).

An obligation none of them proves refuses the program, naming the call and the declaration.

**4. Above the word, a range denotes the set its enforcement admits.** A range above the target's
word is enforced at its bit length (bigrepr-2026-09-03 §3a): `(int LO HI)` admits every `x` with
`|x| < 2ᵇ`, where b is the bit length of max(|LO|, |HI|). So an obligation on such a range is that
set, and a declared result above the word, carried on the term as `(the T e)`, is read as the fact its
enforcement guarantees. A type then means one set in both positions. Within the word, a range is exact
in both.

**5. It is a measurement's consequence, not a guess.** Over the corpus's 346 such obligations:
- 241 literal arguments, all in range;
- 73 proven by the interval analysis;
- 4 proven by the refinement layer, from `Rem64`'s `ensures`;
- 21 proven by rule 4;
- 7 in one program whose exported `run` declared no range for the argument it passed on. Those are
  refused, correctly, and fixed with one declaration.

## Why not

- **Keep §6b, and write the gap down** (option B). Cheapest, and it leaves every declared range on an
  internal definition a promise nothing keeps.
- **Check the declaration *instead of* inlining.** §6b's argument against enforcement was that a
  declared clause is a conservative summary, so checking it in place of the propagated obligations
  would reject a legal `(get a 400)` under `(where (< n 100))`. Nothing here replaces inlining. The
  propagated obligations are still checked, and the declaration is checked as well. A program refused
  under a too-tight declaration is violating its own declaration, and the fix is the declaration.
- **Enforce ranges and not `where`.** A range is a `where`: `x ∈ [lo, hi]` is `lo ≤ x ≤ hi`. Two
  spellings of one claim would mean different things.
- **Check above the word exactly.** Then `(int 0 (pow 2 200))` would admit values up to 2²⁰¹ − 1 as a
  result and only up to 2²⁰⁰ as an argument. `render`'s argument is `fact`'s result, both declared
  with that type, and it could not be discharged. A type that denotes two sets is two types.
- **Mark the call's result instead of its arguments.** A mark around a β result blocks every rule that
  inspects a result's shape: folding, `if` on a known condition, a table lookup, case-of-case. A mark
  around an integer argument meets one shape test, `duplicable`, which looks through it. That is why
  the measurement could show byte-identical emission. `where` does mark the result, because it spans
  several arguments, and each of those rules hoists its mark outward.

## Consequences

- A declared parameter range on any definition is a promise the compiler keeps, and a caller that
  cannot prove it is refused with the declaration named.
- `render.oro`'s exported `run` declares `(int 0 15)`: the one program the measurement found.
- **The measurement's scope was too narrow, and the build found the rest.** It counted `where`
  clauses only on non-exported definitions. `lib/win/fmt.oro`'s `print-int` is an export, called by
  every Windows harness print, and its `where`, `0 ≤ n < 2⁵³ − 1`, could not be discharged at any
  of them. Its upper bound was ADR 0012's window, stale. Its lower bound was §6b's meaning gap
  (`print-int -13` printed a blank line). `print-int` is now total, printing a sign, and declares its
  parameter as the word.
- A range with an infinite endpoint keeps its finite side as an obligation: `(int 0 +inf)` is
  `0 <= n`, carried by the call's own condition.
- Not covered, and named: a definition passed as a value and applied later is not a direct call by
  name, so its contract is not checked at that application.
- A new reduction rule or analysis must see through the two marks, or strip them first. The pipeline
  decides and strips them immediately after reduction, so nothing downstream sees one.
