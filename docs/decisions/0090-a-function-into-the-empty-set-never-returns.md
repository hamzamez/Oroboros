# 0090 — A function into the empty set never returns, and a host's product is taken apart by its projections

Date: 2026-11-03
Status: Accepted (hamza, 2026-11-03: "go ahead with the two-result call and the never-returning call,
always mathematical and algebraic"). Built for `windows-llvm`
([llvm-2026-11-02](../../gauntlet/results/llvm-2026-11-02.md)). Generalizes the crash of
[spec/errors.md §8](../spec/errors.md) from one name to a set; completes
[ADR 0089](0089-a-realization-is-a-term-over-the-hosts-operations.md) for the checked arithmetic.

## Context

ADR 0089 made a target's realizations terms over its host's operations. Three kinds of template were
left because the language could not yet say what they do:
- **LLVM's overflow intrinsics** answer two results, `{i64, i1}`: the checked arithmetic is
  `(s, o) = sadd.with.overflow(a, b); if o then trap else s`;
- **a call that never returns**, `llvm.trap` and Win32's `ExitProcess`: a branch ending in one has no
  value, and the region after it is unreachable;
- the allocator's answer, a pointer as a value (ADR 0084's LLVM part), not this decision's.

The second was already in the language for one name. `abandon` is crash : E → 0 (errors.md §8), and
the compiler knew 0 by that name in six places: the reducer's strictness rule, the type checker,
`unselected`, lowering, the verifier's W3, and usage. settypes-research §1 had named the set: the
Boolean algebra of types has ⊥ = ∅, "the type of `abandon`'s value".

## Decision

1. **∅ is a set of the language**, the empty set, ⊥ of the Boolean algebra of sets, written `∅` or
   `\emptyset` (j.md §1.4's convention, TeX's spelling as `\oplus` is ⊕'s). It needs no representation
   on any target, having no element.
2. **A function into ∅ never returns.** ∅ is the initial object: for every Y there is exactly one
   arrow 0 → Y, so a call h : A → ∅ satisfies k ∘ h = h for every continuation k. That one law gives every
   rule the compiler had for `abandon`, now for every operation whose declared result is ∅:
   - **the reducer**: a call into ∅ absorbs its strict context;
   - **the checker**: its type agrees with every demand, so a branch it ends is typed by the other;
   - **lowering**: it is its region's last statement, and the region ends `unreachable`;
   - **the verifier (W3)**: an unreachable region ends in a call into ∅.
   `abandon` is one such operation, by the language's own declaration.
3. **A host operation into a product B₁ × … × Bₙ is taken apart by its projections.** The IR already
   carries a call's several results; the LLVM printer receives them as the aggregate
   `{ ρB₁, …, ρBₙ }` the host answers and binds each result to `extractvalue j`, the product's
   universal property. A term takes them apart by a tuple pattern, `(s, o) = …`.
4. **A host operation is declared as it is** (ADR 0088): `llvm.sadd_with_overflow(a, b) (Int × Bool)`
   is W × W → W × 𝔹, (a, b) ↦ (the residue of a + b, [a + b ∉ W]); `llvm.trap() (∅)`;
   `kernel32.ExitProcess(n) (∅)`.
5. **Legality is the program's, decided once, before ι is applied** (ADR 0089 item 4): a realized
   operation is counted where it stood, with the mode its first decision gave it; a term's own
   operation is counted only where it is unproven, an obligation at the site. A trapped `+` replaced
   by its checked term, a call and a branch, is still the program's one trapped operation.

## Why not

- **Keep 0 as the name `abandon`.** Then each host operation that never returns needs its own case in
  six places, or stays a template whose branch the compiler cannot type. The law is about the result
  set, not the name: an operation into ∅ is what makes k ∘ h = h true.
- **Type a non-returning call by its host's result** (`void`, or `Int` as `ExitProcess` was declared).
  `ExitProcess(1)` would then be a value of `Int`, and `if c { ExitProcess(1) } else { 5 }` would join
  two integers, one of which never exists; worse, the region after it would be reachable to every
  analysis. ∅ is the type that says it.
- **A unit result for `trap`** (1 → 1, the host's `void`). A call into 1 returns, so the analyses would
  continue past it and the branch would need a value of Int from nowhere. Only ∅ has the arrow into
  every Y.
- **Give the overflow intrinsic an `Option(Int)` result** and retract the host's pair into it, as
  `lib/result`'s host bindings are. The tag and payload of a sum are exactly the pair (ADR 0042), so
  this is a fine reading, and a program could choose it; the declaration states the host's own
  operation, which is the pair, and the term reads its flag.
- **Count the term's operations as the program's** (as ADR 0089 first did). A trapped operation
  replaced by its checked term disappeared from the count, and a term's proven internal steps (the
  branch's arms) were counted beside the operation they realize. Legality is a property of the
  program, and ι is the target's.

## Consequences

- `targets/windows-llvm/lang.oro`'s checked arithmetic, `<<`'s checked form, `trap_if` and the crash
  are terms; one template remains, the allocator.
- `core.EmptyType`, `emit.(*Target).IsBottom`, `core.Env.Bottom`; the decision's count reads the
  realized operation (`Stmt.Realized`) and a term's statements (`Stmt.FromTerm`), both in memory only.
- Relevance (ADR 0043) is decided in the reducer, whose "a path that ends" is the same predicate, so
  a relevant value need not be used on a path ending in any call into ∅. The usage of abstract
  syntax trees (`core/usage.go`, ADR 0050's 𝒰, not yet wired into the pipeline) still names `abandon`;
  it takes the predicate when it is wired.
