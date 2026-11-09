# 0096 — A value's storage is chosen by its size class and released at its death

Date: 2026-11-09
Status: Accepted (hamza, 2026-11-09: "I agree to the recommendations. add the steps to the plan. and
do the first one", on [integers-research.md](../integers-research.md) R1–R4 and Q1–Q4). Being built in
steps; the first, release at death on the heap, with
[release-2026-11-09](../../gauntlet/results/release-2026-11-09.md). Amends
[ADR 0084](0084-an-address-is-an-integer-and-ownership-is-a-view.md)'s persistent permission.

## Context

Nothing on the LLVM layer frees memory. ADR 0092 made a table's storage an operation of the IR,
`alloc(n, w)`, realized by a term over Σ_OS's `region`, and nothing ever released a region. Measured
(integers-research §5.1): a loop of 10⁶ limb additions on windows held 252 MB at its peak, and 10⁷ held
2.7 GB, about 264 bytes per iteration. On Go, the JVM and JavaScript the host's collector reclaims the
same tables, so nothing noticed. A variable-length representation of ℤ (R3) would make the leak
unbounded.

ADR 0084's views decide safety and when an exclusive region dies, at its scope's end. They leave a
frozen value's storage: its permission is **persistent**, linear logic's !A, which admits weakening, so
its view may vanish without being consumed and `free`'s full permission is never in hand
(integers-research §5.6).

## Decision

1. **A frozen value's view is counted fractions, not persistent** (Boyland 2003; Bornat, Calcagno,
   O'Hearn and Parkinson 2005). Freezing splits the full permission ↦₁ into one fraction per
   reference. Each reference's death returns its fraction, and at the last death the fractions sum to
   1 again, so `free`'s precondition holds there. **Persistent** stays for what is never freed:
   constants, literals and static data.
2. **The count is static where the references are SSA names.** A table value of the IR is *owned*
   when its storage can have only one reference at a time:
   - it is a **birth**, a `build`, `tabulate` or array literal with its storage (ADR 0092), or a
     renaming (`the`) of an owned value, or a loop parameter or a join's result every one of whose
     incoming values is owned and moved in;
   - it is **moved** at most once, by a loop's `init`, a `continue`, a `break`, a `yield` or `the`, and
     nothing uses it after the move;
   - no loop lies between its definition and its move, so the move happens at most once per
     definition;
   - everything else it meets **borrows** it: `len`, an index, a call that returns no table.

   An owned value that is not moved on a path is **released at its death** there, the point after
   its last use (integers-research Proposition 3). A value that is not owned (shared, inside another
   value, made by a host, a constant) is not released: its storage stays until the process ends, as
   every value's does today. That is never unsound, only a leak, and the result document counts it.
3. **`release` is an operation of the IR**, as `alloc` is: `release(t, n, w)` with n = `len t` and w
   its element width. Each target realizes it:
   - on LLVM, a term over Σ_OS's new header `release(p, b)` with b = n·w + 8. windows defines it by
     the rule `region` chooses by: `VirtualFree(p, 0, MEM_RELEASE)` from 64 KiB, `HeapFree` below, so
     release ∘ region is the identity on the heap;
   - on Go, the JVM and JavaScript, nothing: the host's collector owns `free`, and the pass does not
     run there.
4. **The address space bounds every length**, a fact of the target: on x64, user space is 2⁴⁷ bytes
   and a table occupies n·w + 8 of them, so `fact max_len (a ∈ Array(A)) len(a) <= (2^47-9)` on
   windows. release's n·w + 8 is then proven from `len t`.
5. **The verifier checks every release, independently of the pass** (W11): the released value is owned
   by the rules of item 2, re-derived; it is moved nowhere; nothing uses it after the release; and no
   loop lies between its definition and its release.
6. **Where storage lives is chosen by size class** (integers-research §5.7, after Goose):
   - a table of static length, every fixed limb value among them, in a frame slot;
   - a table of run-time length that dies inside the IR region that made it, on a data stack popped
     at the region's exit;
   - one that escapes its region, on the heap, released at death (items 2–3), its storage reused by a
     value of the same size made at the same point;
   - a value inside another, borrowed when liveness proves the borrow and copied otherwise.

   Built in steps, each measured, the heap's release first: it fixes the leak for every table, and
   the other classes then move values off the heap without changing what is released.

## Why not

- **A tracing collector** (McCarthy 1960; Boehm and Weiser 1988). A runtime and a hidden allocation
  policy, which pointers-research §13.6 refused for the core.
- **Reference counts on every value** (Collins 1960; Ullrich and de Moura 2019; Perceus, Reinking et
  al. 2021). Complete here, since immutable values make an acyclic heap (integers-research
  Proposition 2), but a count word per table and an update per share, for what liveness decides
  statically. They are the same fraction algebra executed, and stay for the case liveness cannot
  decide, if a program measures a copy there as too dear.
- **Regions per loop iteration** (Tofte and Talpin 1997). Each iteration would copy the values it
  carries, where a move costs nothing.
- **Keep the persistent permission and free at exit.** It is what the layer does now, and it is the
  leak.
- **Free at a scope's end only, as ADR 0084 decided for exclusive views.** A frozen value outlives the
  scope of the buffer it was frozen from, and a loop's carried value has no scope that ends while the
  loop runs.

## Consequences

- Every windows program that makes a table releases it at its death, where the rules decide it. The
  emission of every such program changes.
- Σ_OS has four operation symbols: `region`, `release`, `write_err` and `exit`.
- A value not owned keeps leaking, named and counted: a string from a helper, a table inside a table,
  a host's result.
- Still to build, in order: same-size reuse, frame slots for static lengths, the data stack, then
  strings and containment.
