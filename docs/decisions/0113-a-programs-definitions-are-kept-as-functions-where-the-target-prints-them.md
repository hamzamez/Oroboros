# 0113 — A program's definitions are kept as functions where the target prints them

Date: 2026-11-15
Status: Accepted (hamza, 2026-11-15: "go with your recommendations", Q1–Q4 of
[p8-research.md](../p8-research.md)). Builds ADR 0102's P8, with P3.

## Context

ADR 0102 decided when to reduce and when to call, and kept only ADR 0101's seven K(ℕ\*) instances,
waiting on P8's trigger. Windows' `os`, written in J (ADR 0112), met it. At K = ∅, `roundtrip`,
`ledger` and `pe` pass the reducer's depth. With their dynamic definitions kept they reduce in
6–24 ms, and residuals fall to 0.05–0.42 of their size (p8-research.md §1).

## Decision

1. **The compiler chooses K; a program names nothing.** K is computed per program and target by
   P2's rule, and the source neither marks nor annotates it. By Theorem 1 no answer changes, and by
   P4 no verdict does.
2. **K is non-empty only on a target that prints program functions** (Q1). This is a capability the
   target declares, the LLVM layer's today. K = ∅ elsewhere, so Go's, JavaScript's and the JVM's
   emitted text does not change, until a printer of theirs learns functions.
3. **K's members** (P2):
   - a dynamic definition, first-order in every parameter and its result;
   - kept at a call only when no argument there is static (a λ, a set, a type), else unfolded (P1);
   - **proven under its contract**, else inlined (P4);
   - **paying**: |f| · (π(f) − 1) ≥ T, where π(f) is the number of paths from the entry to f in the
     call graph, a DAG since there is no recursion (ADR 0014). T = 64 nodes to start, measured, not a
     constant of the design (Q4).
4. **A parameter with no declared set takes its representation's codomain** (Q3, Proposition D): one
   instance per (f, ρ⃗, target), the word's range for an `Int` held in the word. It is ADR 0079 at a
   boundary.
5. **A contract is completed by lifting** (Q2, P3, Theorem 4). An obligation the body cannot prove
   under its contract, and which is a formula over the parameters, becomes a precondition each call
   decides. An obligation over anything else leaves the definition inlined (P4).
6. **The boundary is the export's** (Proposition C):
   - scalars, references, a product's factors;
   - a sum's tag and slots, rebuilt at the call by a retraction, as a host's fallible call is
     (errors.md §4.2);
   - a `linear` parameter written in place;
   - a result born in the callee (W13);
   - the call impure iff the body is.

## Why not

- **Raise the reducer's depth and fuel**: Proposition A's sum grows with every call. It hides the
  trigger and keeps the size.
- **A keep or inline annotation in the source**: by Theorem 1 the choice is invisible to meaning, so
  it is no program's to make. ADR 0102's P4 is the rule that makes it safe for the compiler.
- **Every target at once**: three printers would learn functions, and three targets' text would
  change, with no program asking. The capability lets each join when its printer does. The cost of
  windows first is that the shared passes' new paths run on one target, and it is paid down by a test
  that takes a kept program through the IR on all four.
- **Hand-written join points in libraries, or join points in the reducer now**: they cure
  Proposition B only (p8-research.md §6).
- **Only fully annotated definitions** (Q3's alternative): libraries would have to be annotated by
  hand, and an unannotated parameter is already held in a known representation.
- **P3 later** (Q2's alternative): `write` (π 171), `print_int` and `slice` would stay inlined, and
  the ledger would buy less.

## As built, step 1 (p8-2026-11-15)

The build corrected four premises above. Each is stated here, and the research is left as it was:
- **The pays rule** is |f| · max(π(f) − 1, 1) ≥ T. The call graph's π undercounts the copies the
  reducer makes, because case-of-case copies a consumer into each constructor site (Proposition B).
  So a definition used once pays when its own body reaches T, and LLVM's inliner takes it back where
  that is cheaper (P7).
- **A `where` that is only the parameters' range premises is the contract itself**: the reader
  desugars `k ∈ 0..100` into one (`core.WhereIsRanges`), and the call decides it by its arguments'
  sets. Only a `where` beyond them waits on lifting.
- **A `linear` buffer is kept** as B × X → B × R written in place:
  - its handed-back storage is checked to be the argument's, by `sameStorage`, a greatest fixed point
    over joins, loops, πs and stores;
  - a call that lends one is never pure, so the caller's facts of the buffer take the callee's writes.
- **An unstated result's type is the body's hull**, or else its representation's codomain
  (Proposition D). It is never typing's: typing's ranges are representations that unification
  shares, and windows' `open` returned a 0 typed 1..2³²−1. A stated result's hull is met with its
  claim, so a caller reads what the body proves (ADR 0072). This recovers part of Theorem 3's loss.
- **P4 has a net**: a program refused with K ≠ ∅ is built again with K = ∅, and that build's verdict
  is the program's. A finer search, dropping one definition at a time, is owed.
- **A verdict is monotone in K, not invariant** (corrects item 1). Every program accepted with
  K = ∅ is accepted with K, with the same answer. The converse fails: `freq` is refused with K = ∅,
  on an obligation the long inlined residual loses, and accepted with K, where the contract proves it
  once. That acceptance is by a proof, so it is sound.

## Consequences

- Steps (p8-research.md §8), each with its ledger in its result document:
  1. kept calls at reduction, instances lowered and decided once, called on LLVM;
  2. P4's fallback per definition;
  3. Proposition D and lifting;
  4. the ledger closed.
- The proof counts are the witness of P4: unchanged on every program accepted before.
- Every pass that meets a call meets a program function where it met only a host call, and is audited
  for it, with a planted fault where a rule must hold of both.
