# 0 ≤ v is an inductive invariant, and the comparator was never a lost proof

2026-09-16. `emit/refine.go`, `emit/inductive_test.go`, `examples/tally/tally.oro`.

dt-2026-09-16 §5 recorded a finding and did not fix it: in tally, writing `h.cmp` at its use inside the sort
comparator was refused with `(cs u) is an indexing, and (< u (len cs)) does not follow`, and the program was
written destructuring the six operations instead. This is that finding taken apart.

## 1. It was not a lost proof

The two residuals differ in one place. With the destructured form the comparator's index reaches the checker
as a `let` term, `(cs (let (a j′) (fn (i) (clamp i))))`. With `h.cmp` it reaches it as a name, `(cs u)`, bound
by an enclosing `let`.

A `let` term is **outside the linear fragment**, so its obligation was **propagated, not proven** — that build
prints 62 such notes. A name is inside it, so the same obligation is read, and the fragment could not prove it.
The spelling that "worked" was the one that checked less. It is purecontract-2026-09-15's predicted gate firing
on a program: making a term readable turns silence into a refusal.

So the real question was whether `u < len cs` is TRUE and PROVABLE, and it is both.

## 2. What the proof needs

`u` is a clamp of a read at `j`, so `u < n` holds on every path provided `n ≥ 1` (the clamp's zero leaves need
`0 < n`). `n ≥ 1` follows from the merge's own guards, `k < hi ≤ n`, **provided `0 ≤ k`** — and nothing in scope
said so. `k` starts at `lo`, and `lo` is the merge sort's pass variable:

```
lo₀ = 0,     lo′ = min(n, lo + 2w),     w₀ = 1,     w′ = 2w
```

The syntactic rule licenses `0 ≤ v` only when every step is `v + c` with a literal `c ≥ 0`, or `e ⊒ S`. Neither
step here has that shape: `lo′` is non-negative because the clause guard says `lo < n` and because `lo` and `w`
already are, and `w′` because `w` already is. That is **induction**, and it is decided as induction is.

## 3. The theorem

**Theorem (inductive invariant).** Let C be a set of loop variables such that for each v ∈ C the entry facts F
prove `0 ≤ z_v`, and at every back edge `(again a₁ … aₙ)`, reached under facts P,

```
P ∧ ⋀_{u∈C} 0 ≤ u  ⊢  0 ≤ a_v        for every v ∈ C.
```

Then `0 ≤ v` holds at every iteration, for every v ∈ C.

*Proof.* Induction on iterations, simultaneously over C. On entry each v is z_v and F holds. If `⋀ 0 ≤ u` holds at
an iteration, the back edge taken is reached under facts that hold there, so each new value is non-negative. ∎

P may be used because the facts `iterate` keeps in a loop body never include a variable's initial value, only what
every iteration guarantees: clause guards, and facts about immutable names.

**Finding C is Houdini** (Flanagan & Leino, FME 2001): start from every candidate, discard any that some back edge
fails to preserve, and repeat. Discarding only weakens the hypotheses, so the iteration is monotone and stops within
`|C| + 1` rounds at the **greatest inductive subset**. Each round is a dry walk of the body, a refiner that proves
nothing, refuses nothing and records the facts at each of this loop's back edges. A nested loop met during a probe
keeps only the syntactic rule, so the rounds do not multiply with nesting depth.

## 4. Two readings the theorem needed

Both were found by the witness, not by the argument.

- **A back-edge argument is often a CONDITIONAL, not a name.** β substitutes a clamp used once straight into the
  `again`, so the argument is `(if (< n (+ lo w)) n (+ lo w))`, which the fragment cannot read. It is read as if
  bound to a fresh name and given what every branch satisfies, which is `joinConditional`'s theorem: exactly one leaf
  is evaluated, on a path whose facts hold.
- **A name bound to a linear value is that value.** When the clamp is `min` inlined, a `let` binds its operand first,
  `(let (+ lo w) (fn (b) (if (< n b) n b)))`, and the join walked through that `let` without assuming `b = lo + w`, so
  the leaf `b` was unknown. The equation now holds on every path below the binding, an exact fact rather than an
  approximation.

`TestALowerBoundIsProvedByInduction` covers the joint case (`lo` needs `w`), both shapes of the step, a control whose
step can go negative, and a candidate set that must SHRINK. **Removing any one of the three pieces fails it**: the
invariant check, the conditional argument, or the let equation.

## 5. What it is worth

- **tally is written the natural way**, `h.cmp` at its use, with the explanatory comment deleted because its reason
  is gone. Output identical on the sample log. The emitted Go differs from the destructured form by two copied locals
  and a renamed variable (16,291 → 16,336 bytes). No benchmark covers tally.
- **4 index obligations go from propagated to proven** in tally: 62 notes become 58. Four checks that were never
  made are made and pass.
- **Nothing else in the corpus moves**: every emitted file, note and proof count is byte-identical, and the one
  accepted change is which unbound name tally's JavaScript refusal reports first. The rule fires only where a step
  is not "plus a literal", and the swept corpus's only such loops are in programs it cannot build.

## 6. What it points at

The other 58 notes in tally are the same class: an index that is a `let` or a conditional TERM, read as outside
the fragment. The reading in §4 applies to them verbatim, so each is either proven or honestly refused. That is the
next measurement, and it is one purecontract said would move terms from *report it* to *refuse it*, so it may find
something.

## 7. Cost

- `emit/refine.go` 854 → 945 code lines, **+91**.
- Every check step passes; the tooling suite builds tally on both hosts from the new spelling.
