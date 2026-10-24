# 0075 — A host `where`'s constant bounds on one parameter are that parameter's range obligation

Date: 2026-10-24
Status: Accepted (hamza, 2026-10-24: "go ahead with item 3", the program `examples/os/wc.oro` naming the
refusal; CLAUDE.md's deliberately-not-next list held "a host `where` on the IR's intervals… ready, none
asked for"). Reads [ADR 0062](0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md)
item 2, bounds in binder sets and relations in `where`, as a rule about meaning, not spelling.

## Context

A host declaration's precondition is an obligation at every call (ADR 0028). Two spellings of one claim
were decided by two procedures:
- a ranged parameter, `n ∈ lo..hi`, by a range mark: a literal, then the IR's intervals, then the
  refinement layer (`DischargeRequires`);
- a `where`, by the refinement layer alone, which reads facts off comparisons and does not see a
  loop's exit value.

So `gos.Exit(status)`, with status a loop variable that is 0 at entry and 0 or 1 on every back edge, was
refused, while the IR's intervals hold status ∈ [0, 1] exactly. `Exit`'s bound is a `where`,
`code >= 0 && code <= 255`, because its comment chose it: "the type states the host's domain, and this
is a claim beyond it".

Writing it as `code ∈ 0..255` instead broke the hand check's planted mistakes: a narrowed parameter is,
mechanically, a transposed function (`Len64` given a 32-bit argument). CLAUDE.md's rule: a relation
relaxed for one check weakens every check that reads it.

## Decision

1. **A conjunct of a host `where` that bounds one parameter by an integer literal** (lo ≤ x, x ≥ lo,
   lo < x, x ≤ hi, x < hi, …) states x ∈ [lo, hi]. When a parameter has both ends, the pair is that
   parameter's range obligation (`Prim.Bounds`, intersected with the parameter's own range). It is
   decided as a ranged parameter's is, and it is taken out of the `where`. The rest of the `where` stays,
   for the refinement layer as before.
2. **A one-sided bound stays in the `where`**, as a definition's infinite end does (`installContracts`).
3. **The host's type is unchanged**, so the hand check keeps holding each parameter to the host's.
   What changes is how the bound is decided, not what the declaration claims.
4. **It is never weaker**: the range route ends in the refinement layer, which decided the bound before.
   It is stronger by the IR's intervals.
5. **It is done once, as the target is read** (`emit/wherebounds.go`, before the host primitives are
   derived), so every pipeline sees it.

## Why not

- **The bound as the binder's set in the declaration**, `code ∈ 0..255`: the hand check could no longer
  tell a justified range from a type confusion.
- **Relaxing the hand check for narrowed parameters**: two planted mistakes passed (above).
- **Deciding a `where` on the IR's intervals as a whole**: a `where` is a formula, and the intervals
  decide a set. Only a conjunct that is a bound on one parameter is a set, and that is the part moved.
- **Clamping in the program**, `gos.Exit(if status == 0 { 0 } else { 1 })`: the fact is derivable, and
  a clamp is a missing proof.

## Consequences

- `os.Exit`, `string_of`, `hex.EncodedLen`, `strconv`'s bases and every host declaration with a
  two-sided constant bound decide it on the IR's intervals too. The corpus's emission and refusal texts
  are unchanged.
- `TestAStatusIsBoundedByItsExits` had pinned a false refusal ("an exit of 3i, up to 300": the loop
  exits by i = 50, so 3i ≤ 150). It is now an accepted control, and 6i is the refusal.
- Tests of the refinement layer alone through such a bound changed: two (EncodedLen's boundary, Lemma 1)
  run the pipeline now (`hexbound_ir_test.go`), and two (the clamp's join) write their bound
  relationally, `k + 1 <= 10`, which stays a `where`.
- A range refusal's text says "declared in lo..hi" for a bound a `where` declared.
