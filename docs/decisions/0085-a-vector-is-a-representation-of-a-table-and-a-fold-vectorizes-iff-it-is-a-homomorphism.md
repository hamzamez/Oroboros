# 0085 — A vector is a representation of a table, and a fold vectorizes iff it is a homomorphism

Date: 2026-10-30
Status: Accepted; not built (its main consumer is `ir/llvm`)

## Context

The fourth of the seven walls [llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md)
found in Σ_LLVM is vectors: `<N x T>` and `<vscale x N x T>`, `extractelement`, `insertelement`,
`shufflevector`, 158 target-independent intrinsics and 10,101 in all, 61.5% of Σ_LLVM. GPUs are the
same question at a larger index set. [vectors-research.md](../vectors-research.md) derived the algebra,
surveyed the literature (Mojo included) and measured LLVM, Go and the JVM. hamza (2026-10-30): "I agree
to all recommendations."

## Decision

1. **A vector is a representation of a table of static length, never a sort.** There is no vector type
   and no width in any type. A register `<n x T>` is the representation ρ chooses (ADR 0083, by place)
   for a table whose length is a constant the register holds and whose operations are pointwise.
   Birkhoff: the direct power Sⁿ satisfies every equation S does, so lane-wise arithmetic is the
   scalar algebra.

2. **The compiler vectorizes the language's three data-parallel forms, each by its theorem:**
   - **a rule** `array { e : k ∈ … }`, a map whose elements are independent by construction;
   - **a fold by an associative operation**, by the homomorphism lemma (Bird 1987; Gibbons 1996): ℤ's
     +, ·, &, |, ⊕, min and max, and F64's min and max. Each lane's partial result is proven inside the
     word, which for nonnegative terms the whole sum's proof covers;
   - **a loop of stores** whose index functions the Omega test shows conflict-free (Pugh 1991), with
     views exclusive (ADR 0084), so one buffer's indices are compared.

   On LLVM the facts are stated and its vectorizers act. Elsewhere it is the printer's, by item 4.

3. **A float fold keeps the order the program wrote, on every target.** Floating-point addition is a
   commutative, non-associative magma, and a reordered sum is a different number (measured: about 2×
   faster and different in the last digits on 2²⁴ values). Staging never changes an answer (ADR 0009).
   Permitting a reordering is a relation, a set of permitted results, decided by wall 6. Exact
   summation (superaccumulators; ReproBLAS) makes a float sum associative, and is recorded for a
   program that needs a fast, reproducible one.

4. **Explicit SIMD is printed only where the host offers it and a measurement shows it pays.**
   - LLVM vectorizes by itself, and the JVM's C2 does in part.
   - Go's compiler does not. Its `simd/archsimd` is experimental and amd64-only, so it is Tier 2 and
     measured against scalar Go before use.
   - JavaScript has none, and WebAssembly's SIMD128 comes with a WebAssembly target.

5. **A target's vector intrinsics are host primitives over tables of static length**, declared by hand
   (ADR 0022) with `where len(a) == n` and their undefined behaviour as their `where`: `pshufb`, AES-NI,
   `pclmulqdq`, CRC32C, NEON's. They are Tier 2, with portability computed. This is how the 10,101
   enter Σ_LLVM.

6. **Shuffles and masks are rules**: a shuffle `array { v[σ(k)] : k }` with σ static, a select
   `array { if m[k] { a[k] } else { b[k] } : k }`, recognized by the printer. A masked remainder is safe
   by a rule's domain obligation (ADR 0084's build). A scalable vector's width, read at run time, is the
   representation's.

7. **GPUs are a later target layer with the same algebra**:
   - a kernel is a rule over a grid;
   - a reduction across threads is item 2's monoid;
   - device memory is a view kind (ADR 0084);
   - a barrier is concurrency's (ADR 0081).

   Not built. The trigger is a program that needs one, with Futhark and Dex as models and WebGPU,
   Vulkan and D3D12 as hosts.

## Why not

- **A vector type in the language** (Mojo's `SIMD[dtype, width]`, Rust's `std::simd`, Zig's
  `@Vector`, C++26's `std::simd`):
  - It puts the width in the type, the representation-in-the-type that ADR 0083 rejected for
    integers.
  - It is a second spelling of a table of static length, a misfit by fit-research's count.
  - It is non-portable by default: a width fits one target's registers, and portability across
    widths falls to the programmer (`simdwidthof`). Mojo is the most complete explicit design in a
    shipping language; its central choice is the rejected one.
- **Auto-vectorization by the hosts alone**: nothing on Go or JavaScript, which do not vectorize, and
  nothing for a float fold.
- **Reassociating float folds by default**, as `-ffast-math` and GPU atomics do: the answer would depend
  on the target and its width, which ADR 0009 forbids.
- **A schedule language now** (Halide): the most powerful route to portability with control, and a
  natural static value, but unneeded until a program's measured speed asks for a choice the compiler
  cannot make. Recorded as the step after this one if needed.
- **GPU kernels now**: a host layer per platform (launch, transfer, synchronization), a step of its own
  that no program asks for yet.

## Consequences

- **Nothing in the language changes.** The decisions are about representation and printing.
- **Built with `ir/llvm`**:
  - the facts stated (`noalias`, `nsw`, `inbounds`, and never `reassoc` on a float fold);
  - a static-length table's vector representation;
  - the shuffles and selects;
  - the intrinsics as host primitives.
- **Built when measured**: Go's `archsimd` realization of a rule or an exact fold.
- **Commits the IR** to recording, for a fold, whether its operation is associative: a property of the
  operation, read off its algebra. ℤ's yes, F64's + and · no.
- **Hands wall 6** the first relation a program may want: a float fold in any order.
- **Planted faults, when built**:
  - a float sum vectorized without permission;
  - an integer fold with mixed signs whose partial sums leave the word;
  - a loop of stores the Omega test cannot show independent, vectorized anyway;
  - a masked remainder reading past a table.
