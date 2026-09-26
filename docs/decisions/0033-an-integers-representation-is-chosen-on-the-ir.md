# 0033 — An integer's representation is chosen on the IR; the checker's integer sort is ℤ

Date: 2026-09-27
Status: Accepted. Realized for the unsigned word in irstep4g; the rung above the word (`PromoteBig`)
follows.

## Context

ADR 0026 says an `int` is an integer: a range is a set, and a target realizes a sub-lattice of
intervals by containment, S (the signed word) and on Go also U = [0, 2⁶⁴). ρ of a value's interval
picks its realization: `int` where I ⊆ S, `u64` where I ⊆ U and I ⊄ S, and arbitrary precision past
both, by declaration.

That choice was made on terms. `SelectWords` ran the term interval analysis with its selection flag,
rewrote the residual with `u64` primitives and residue-map conversions, and ran before the term type
checker. It had to, because the checker kept the realizations as three integer **sorts** (`int`, the
U range, `big`), and refused one where another was declared. Two of its messages state conditions:
- an `int` where U is required;
- a U value where an `int` is required, since the two agree only modulo 2⁶⁴.

Both are true conditions, and both are about **values**: this value lies in U, that one lies in S. The
checker has no facts, so it approximated a membership condition by a sort, and that is why the
representation had to be chosen before it, on terms, by the term analysis. ADR 0032 moved
representation onto the IR, where it is a type and the verifier checks its flow, and the IR's interval
domain now decides legality with ends exact to 2¹²⁶. The term interval analysis is kept alive by
`SelectWords` and `PromoteBig` alone.

## Decision

**The term checker's integer sort is ℤ.** An `int`, a range inside the word and a range in U all
denote sets of integers, and the checker types them alike: a declared range is a membership claim, not
a sort. (Arbitrary precision keeps its own sort until its selection moves too.)

**Membership is an obligation, discharged by the IR's facts or refused.** A value flowing into a
position declared in U (a host parameter, a signature's result) must be proven in U. A value held in
U flowing into an `int` position must be proven in S. These are refinements.md §3a obligations:
discharged by a proof, or refused, naming the value's interval.

**The IR chooses each integer value's representation.** ρ(fact) with S preferred, fixed where a
declaration fixes it (a U-typed parameter or result, a host primitive's argument or result). Then:
- **+, − and ·** are computed in the realization of their result, their operands converted by the
  residue map. This is exact because q : ℤ → ℤ/2⁶⁴ is a ring homomorphism and S and U are both systems
  of representatives, so a result proven in one of them is named by its residue;
- **<, =, / and %** are computed in a realization holding both operands, where the conversions are
  the identity on the values that occur. q preserves neither order nor truncating division;
- **a conversion** is the residue map, `u64-of` into U or `int-of-u64` into S, inserted where a value
  crosses between realizations.

The verifier's representation rule (W5) checks that every value flows into a position of its own
representation, as it does for table classes.

## Why not

**Keep the selection on terms.** It keeps a 4,000-line analysis alive to answer a question the IR
already answers with better facts, and it makes the term checker depend on a representation it has
no facts to justify. Its sort approximation refuses a value proven in U flowing into a U position
unless selection happened to run first, and that is the declared path's whole reason to exist.

**Keep the checker's sorts and make the IR select only the computed path.** Then the declared path
still needs the term selection, and two passes answer one question with two analyses, which is the
arrangement that let them disagree.

**Make the IR choose, and the checker still check sorts after it.** The checker runs on terms before
lowering, so it cannot see a choice made later. Checking representation belongs where the
representation is: W5, on IR_P.

**A separate unsigned type family** (ADR 0026's own rejection). Unchanged: a range already says
unsigned.

## Consequences

- **The refusals keep their meaning and gain their evidence.** "Not in the unsigned word" and "not in
  the signed word" are said by the IR with the interval that fails, instead of by a sort mismatch.
- **Word selection happens after the decision**, on the decided function: an operation proven in U is
  a `u64` operation, and the decision is taken again on the selected function, where it is proven in
  U. Only programs with a value in U pay the second analysis.
- **`SelectWords`, `DeclaresWord` and the term analysis's selection mode are deleted**, with the
  checker's U-versus-`int` refusals.
- **The rung above the word is the same shape**, a representation chosen by ρ of a fact, and is the
  next to move. When it has, the term interval analysis has no user.
