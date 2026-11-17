# 0102 — Factoring never changes an answer or a verdict

Date: 2026-11-11
Status: Accepted (hamza, 2026-11-11: "I agree to the recommendations", Q1–Q4 of
[specialization-research.md](../specialization-research.md)). **Amends
[ADR 0101](0101-a-library-instance-is-a-function-of-the-program.md)**: a call a program cannot make
is inlined, not refused. The policy (P1–P8) is the research's §8.

## Context

ADR 0101 made K(ℕ\*)'s large instances functions of the program. That raised the general question:
which definitions should the compiler unfold, and which should it keep as functions? And should it
reduce everything and then fold repeated code back into functions? specialization-research.md answers
it:
- **Theorem 1**: which definitions are kept (the set K) never changes what a program computes. Keeping
  a definition is an extension by definitions, and β and δ are sound;
- **Theorem 2**: a function proven under its contract is correct at every call that satisfies the
  contract (Hoare's procedure rule). Inlining proves at least as much;
- **Theorem 3**: one contract proves less than its sites, because interval analysis is not distributive
  (`t * k`), so a contract inferred as the sites' join refuses programs inlining accepts;
- **Theorem 4**: an obligation the function cannot prove under its contract, lifted to its callers as a
  sufficient precondition over its parameters, is decided at each site as inlining decides it;
- **Folding by pattern after reduction** is the smallest-grammar problem, NP-hard; **folding by origin**
  (merging one definition's instances) is linear and exact.

Under ADR 0101 a call whose argument fell outside its parameter's set refused the program. No numeral
has done so, because K(ℕ\*)'s numerals satisfy the set by construction, but the rule made a verdict
depend on a choice invisible to meaning.

## Decision

1. **Factoring never changes an answer or a verdict.** The set K of definitions kept as functions is
   chosen so that every program inlining accepts is accepted with K. A definition its contract does not
   prove, or whose call a program cannot make, is inlined there.
   - Theorem 2 makes the converse true of the collecting semantics: a kept function accepts nothing
     inlining refuses.
   - The implementation widens, and widening is not monotone, so with K it may accept a program
     inlining refuses. Each proof is still sound, and M-S2 looks for such a case.
   - This is ADR 0009's principle for staging, stated for factoring.
2. **The static level is always unfolded**, specialized per static value and memoized (P1).
3. **A dynamic definition is kept iff its contract proves it and keeping it pays** (P2): its size times
   its extra copies above a threshold that a measurement sets, or a measured compile time naming it. A
   definition used at most once is always inlined.
4. **A kept function's contract** (P3): its declared sets and `where`, completed by its unproven
   obligations lifted to its callers as sufficient preconditions over its parameters, with its result
   read through its declared set or its `ensure`. **Never the join of its sites.**
5. **A clone per tuple of argument representations**, when a measurement names the loss (P5).
6. **No folding by pattern after the decision** (P6). Identical code folding of whole functions is
   allowed when a binary's size calls for it. Merging by origin is the fallback for a definition whose
   obligations cannot be lifted.
7. **Physical inlining is the host's** (P7): a kept definition is the host's function, and the host's
   inliner decides, measured.
8. **A program's own definitions are kept on a trigger** (P8), after M-S1 (duplication) and M-S2
   (proofs at a boundary), each with its time limit. Until then ADR 0101's seven instances are the only
   ones kept.

**Built with this ADR**: item 1 for ADR 0101's instances.
- A callee its contract does not prove is inlined at its sites.
- A call failing its obligation (`CallSetError`) makes `Entry` run the pipeline again with that
  instance inlined, at most once per instance.
- Witnesses:
  - `TestACallOutsideItsContractIsInlined`, with a numeral parameter narrowed to 0..1
    (`PlantNarrowNumeral`);
  - `TestAnInstanceItsContractDoesNotProveIsInlined`, with `div-small` made a callee, its result's
    claim refusing it.
  - Each fails with `PlantNoInlineFallback`, for its stated reason.

## Why not

- **Inline everything, always** (before ADR 0101). Size is Σ|f|·π(f), exponential in the call graph's
  depth. It measured ten minutes in clang on the 2,000-quotient program (addsub-2026-11-10).
- **Reduce everything, then fold by pattern.** It is an NP-hard search for repetition the source
  already names as definitions. It also leaves the decision's cost, which runs before the folding.
- **Separate compilation: never inline the program's definitions.** Decided under its contract alone, a
  definition loses its sites' facts (Theorem 3). A choice invisible to meaning would then refuse
  programs.
- **A contract inferred as the join of the sites**: Theorem 3's `t * k`. Site A (t < 2³², k < 2²⁸) and
  site B (t < 2⁶⁰, k ≤ 3) are each proven, but their join reaches 2⁸⁸.
- **Refuse a call outside its contract** (ADR 0101 as built). The verdict would depend on which
  definitions the compiler kept, which Theorem 1 says is not part of the program's meaning.
- **Emit everything as functions and let the host inline.** The host cannot decide our obligations, so
  every definition would be proven by its contract alone, which is separate compilation again.
- **Refuse a program whose library instance is not decided** (raised 2026-11-17, noend-2026-11-17,
  after windows32's undecided `div` made two programs compile for over 20 minutes). The verdict
  would depend on the library's proofs on the target, which are no part of the program's meaning,
  and inlined the program is decided and right. The defect is the library's on that target: ADR
  0035's "checked once" is enforced instead by a check of the compiler, every called instance
  decided on every target K(ℕ\*) serves.
- **Bound the compile's work per program** (raised then too). The compile already ends: reduction
  runs on fuel, `Keep` drops a definition per retry, `Entry` reruns at most once per instance, storage
  and ι stop at 8 rounds, and the decision widens. A further constant would refuse a long compile of
  a program inlining accepts, a verdict changed by a measure of effort.
- **A learned inlining policy** (MLGO). It cannot be derived or explained, and it answers only run time,
  never what is proven.

## Consequences

- A program is accepted with ADR 0101's functions wherever it was accepted before them.
- A fallback costs one more pass through the pipeline per instance, only on the path a program takes
  when its call fails; the corpus takes it nowhere.
- Lifted preconditions (P3) and keeping a program's own definitions (P8) are owed. They are built
  when M-S1 and M-S2 say they pay, or a program's measured compile time names one.
- `ensure` on a kept function becomes the way to carry a relational result (P3), and so the way
  `div-small` and `rem-small` could be called.
