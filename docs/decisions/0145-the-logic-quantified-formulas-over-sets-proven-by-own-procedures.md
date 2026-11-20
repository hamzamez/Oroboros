# 0145 — The logic: quantified formulas over sets, recursion in the logic only, lemmas proven step by step, a host's quantified promise believed with evidence

Date: 2026-11-20
Status: Accepted (hamza, 2026-11-20: "L1. yes, I agree L2. yes L3. yes L4. yes L5. yes L6. yes
L7. yes", on the seven decisions put to him after "what do you need from me to remove all
blockers and we implement it all"). Amends [ADR 0062](0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md),
whose `spec`, `lemma` and `assert` it makes buildable; under [ADR 0049](0049-types-are-sets-and-the-logic-never-prints.md)'s
two languages and its own procedures with no SMT. Not built.

## Context

ADR 0049 split the language in two: a computation that reduces and prints, and a logic whose types
are sets and whose claims are formulas, which never prints. ADR 0062 gave the logic its three
definitions, `fn`, `spec` and `lemma`, and `assert`, and left the surface "built when it is
chosen". What is built of the logic is its quantifier-free, linear part: `where` and `ensure` as
conjunctions of linear comparisons, decided by the refinement layer; `invariant` over scalars; the
Omega test (`presburger/`) as a soundness witness.

settypes-research §8 measured, in October, that refusals lacked facts and not proof strength, so
the logic waited for programs. Since then three kinds of program have asked for a quantified claim,
and each met a wall:
- **a C string from Win32**, `GetWindowTextA`: the text is ours only if ∀ i < k : b[i] ≠ 0;
- **a sort through a host**, whose comparator's law makes the answer one on every target;
- **a fact relating two reads of one table**, which T1-4 and finding 101's rows and offsets
  needed, and which a fact, a schema over one trigger, cannot state.

The blockers were decisions, not walls. hamza: "the idea is to remove what is decided, in the hope
that unlocks a lot of our walls. and we don't decide again."

## Decision

1. **L1, the notation**, TLA+'s as J's sets are (ADR 0065), the set former's colon:
   `∀ i ∈ 0..k-1 : b[i] != 0`, `∃ x ∈ S : φ`, `⇒` and `⇔` between formulas, in `where`, `ensure`,
   `assert`, `invariant` and `lemma`.
2. **L2, every quantifier ranges over a set**: the binder's set is its type (ADR 0049); there is no
   bare `∀ x : φ`, and a quantifier over all of ℤ is `∀ x ∈ Int`.
3. **L3, recursion in the logic only**: a `spec` and a `lemma` may be recursive, each checked to
   terminate by size-change termination (`ir/sct.go`'s procedure); a `fn` never (ADR 0014 stands
   for the computation, whose reason, a stack at run time, the logic does not have). A recursive
   lemma is a proof by induction; a recursive `spec` a definition like a table's sum.
4. **L4, a lemma's proof is its body's steps** (Dafny's way): a sequence of `assert`s, invariants
   and calls of other lemmas, each decided by the procedures; an empty body is a conclusion decided
   from the premises alone. **No axioms**: the extension stays conservative, and the one trusted
   source is a host declaration's `ensure`, as today.
5. **L5, a host's quantified promise is a formula in its declaration's `ensure`, believed as `pure`
   is, its documentation's sentence kept as evidence and checked both ways as `ordered`'s is**
   (ADR 0143). `GetWindowTextA` ensures ∀ i ∈ 0..r-1 : b[i] != 0 and b[r] = 0. This settles the
   question left open by ADR 0091's rest, "a C string from Win32 into ours": by the host's promise,
   not by proving strlen's invariant of a host's code we cannot read.
6. **L6, `-checked` and a quantified claim**: a claim the procedures cannot prove is refused, as
   every obligation is; under `-checked`, a ∀ over a bounded range `lo..hi` is run as a loop at its
   point, O(hi − lo), a trap where it fails; a ∀ over an unbounded range or a set is refused under
   `-checked` too.
7. **L7, the fragments**, own procedures (ADR 0049), each with its planted fault:
   - Presburger arithmetic with quantifiers, by quantifier elimination (Cooper 1972, or the Omega
     test's projection, Pugh 1991);
   - the array property fragment (Bradley, Manna and Sipma 2006), each universal instantiated over
     a finite set of index terms;
   - uninterpreted functions, a host's pure predicate and a λ's body, by congruence closure;
   - their combination (Nelson and Oppen 1979);
   - and, each when its program exists, BAPA (Kuncak and Rinard 2007) for the sizes of sets and maps'
     key sets, and regular string patterns by automata.

   The strong procedures run only on the claims that need them: run on every obligation, the Omega
   test cost 3.5–9.5× in compile time (presburger-2026-10-11), and the gate allows 1.5×.

## Why not

- **A bare quantifier, `∀ x : φ`** (L2): a binder with no set has no type, against ADR 0049; it
  would be a second, untyped logic beside the typed one.
- **No recursion in the logic** (L3): induction would then be impossible, and "the sum of a table"
  unwritable except as a loop, which is the computation's and prints.
- **Axioms, trusted lemmas** (L4 (c)): one false axiom proves everything (a conjunctive fragment
  derives anything from an inconsistency, postconditions.md §4, Lemma 1), and nothing could tell a
  program's axiom from its theorem.
- **Proving a host's promise from its code** (L5): the host's code is not ours to read (ADR 0001's
  parasite: a host is a black box), and for Win32 not readable at all; the evidence rule already
  governs `ordered`, `pure` and every `ensure`.
- **A quantified claim always refused under `-checked`** (L6): a bounded ∀ has a run-time meaning
  every target can compute, a loop; refusing it would make `-checked` weaker than the claim.
- **An SMT solver** (L7): ADR 0049's decision, for its trusted base and its reproducibility; and
  every fragment above has a complete procedure the project can write and plant faults in.
- **Every fragment at once, BAPA and strings included** (L7): each waits for a program that states
  its claim, so no procedure is built that nothing runs.

## Consequences

- The build, after "Build all 5"'s open items (the plan, CLAUDE.md), staged, each stage a program
  first and its book chapter:
  1. a research note: the formula language's abstract syntax, its sort check, ε the erasure
     homomorphism (spec/abstract.md) so a formula never reaches emission, and the procedures' order;
  2. formulas and `assert`, with quantifier elimination;
  3. the array property fragment, with GetWindowTextA's C string (L5's evidence), TinyRetroPad's
     window text;
  4. congruence closure and the combination, with a comparator's law through a host's sort, one
     answer on every target;
  5. `spec` and recursive `lemma`, with a table's sum proven by induction;
  6. `-checked`'s bounded loop (L6).
- Unlocked when built: T1-4's facts across a borrowed buffer, a table of tables by rows and offsets
  (their monotonicity a ∀ over two reads), a former's laws (formers-research, waiting on demand), a
  law a callback obeys, an invariant over a buffer's elements.

## As built

- **Stage 2** (2026-11-20, [logic2](../../gauntlet/results/logic2-2026-11-20.md)): the reader's `∀ ∃ ⇒ ⇔` and
  `assert`; `presburger.Valid`, quantifiers by polarity (Skolem, Cooper) and the Omega test; the
  translation from the residual, divisions by constants exact; a definition's `ensure` that is no
  constant bounds decided as `where ⇒ ψ[body/result]`. The Gregorian leap year and Bloch's midpoint on
  every target; the book's §16.7.
