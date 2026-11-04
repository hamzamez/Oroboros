# 0092 — A table's storage is an operation of the IR, and the allocator a term over the host's

Date: 2026-11-04
Status: Accepted (hamza, 2026-11-04, asking why `lang.alloc` was "still a full program in a string,
and not calling and using llvm instructions"). Built with
[cstring-2026-11-04](../../gauntlet/results/cstring-2026-11-04.md). Amends ADR 0089's one exception.

## Context

ADR 0089 made each realization a term over the host's operations, ι : Σ_L → T(Σ_H), substituted on the
IR after the decision. One name escaped: the allocator, a 20-line LLVM `define` in a string.

The reason was not the algebra. The printer allocated inside its lowering of `build`, `tabulate` and
an array literal, by filling the template with `8 + n·w` bytes it computed itself. So ι, which acts on
the IR's operations, never saw the allocation, and the decision never saw its arithmetic. That `mul`
was unchecked: a table of more than 2⁶⁰ eight-byte elements would wrap to a small region and be
written past its end.

## Decision

1. **A table's storage is an operation of the IR.** On a target that realizes `alloc` by a term, a
   pass (`ir.Storage`) puts `p = alloc(n, w)` before each statement that makes a table of n elements
   of w bytes, and the statement takes p as its storage (`Stmt.Store`, printed `store %p`). w is
   `ir.ElemBytes`, the one layout every access reads by.
2. **The allocator is a term**, alloc : ℕ × {1, 8} → Region, over kernel32's operations as Win32 has
   them (`VirtualAlloc`, `GetProcessHeap`, `HeapAlloc`) and Σ_LLVM's `null` and `is_null`:

   ```
   allocate(n, w) = case { 0 ≤ n ≤ ⌊(2⁶³ − 9)/8⌋ -> region(n·w + 8); else -> trap }
   region(b)      = let p = (b ≥ 2¹⁶ ? VirtualAlloc(null, b, …) : HeapAlloc(GetProcessHeap(), 8, b))
                    in (is_null(p) ? trap : p)
   ```

   A request the guard turns away is above 2⁶⁰ elements, past any address space (x64's user space is
   2⁴⁷ bytes), so it is **exhaustion**, the answer a null region already is. The term is total. The
   guard bounds n by the widest element, so n·w + 8 is inside the word for every w ∈ 1..8 by the
   intervals alone (n ≤ ⌊K/w⌋ ⇒ n·w ≤ K is nonlinear), and the decision proves it once, over the
   term's declared domain.
3. **Storage and ι reach a fixed point**: a term ι substitutes may make a table (a crash's newline),
   which the pass then gives storage; the allocator's own term makes none, so it ends.
4. **ι does not change what the program proves** (ADR 0090's "legality before ι"). The intervals'
   widening thresholds are the program's constants, not a term's (`FromTerm`). A term's constants had
   moved where `json-tokenize`'s loops widened to, and 12 of its operations lost their proofs.

## Why not

- **Keep the template.** It is a program the decision never reads, its arithmetic unchecked, and a
  second language (LLVM IR) inside the target where ADR 0089 says terms are over the host's operations.
- **Compile the term as a function the module calls.** Its arithmetic would be decided once with no
  site facts, so the instance could not prove n·w at the site. A second kind of definition would also
  reach the module beside ι's substitution.
- **Bytes computed as IR arithmetic at the site, refused when unproven.** A table whose length the
  facts do not bound would be refused, yet the request has a meaning, exhaustion. The term's guard
  gives it, with no refusal.
- **Put the storage among `Args`.** The analyses read a build's first argument as its length and an
  array's arguments as its elements, so a field of its own is the one place nothing misreads.

## Consequences

- `targets/windows/lang.oro` has no template but strings' two helpers, which wait on V, the
  valid-UTF-8 refinement (ADR 0091's consequences).
- A table on windows is allocated by an inlined term, and its size is a proven `mul nsw`.
