# Research: vectors, or a table the hardware applies an operation to all at once

2026-10-30. **For hamza's decision**: V1–V7 in §9. The fourth of the seven walls
[llvmsurvey-2026-10-27](../gauntlet/results/llvmsurvey-2026-10-27.md) found (§4.7), and the 10-24 plan's
6b, step 4. hamza: "go ahead with the research on wall 4, vectors. I supposed this related to simd, and
gpu's. mojo might be the best implementation so far in a programming language, correct me if I am
wrong. reach for the literature, and let's do the math." The measurements in §7 were taken on LLVM
23.1.4, Go 1.27 and JDK 17 on this machine.

**The short version:**
- **Yes, this wall is SIMD, and GPUs are its other half.** A SIMD register holds n lanes and one
  instruction acts on all of them. A GPU runs one function at every point of an index space. Both are
  the same mathematical object: **a function applied pointwise over a finite index set**.
- **The algebra.** A vector of n lanes of S is Sⁿ, a function [0, n) → S: a table of static length.
  Its operations are S's lifted pointwise, the direct power, which satisfies every equation S does
  (Birkhoff). A reduction is a fold of a monoid. A shuffle is precomposition with an index map, and a
  mask is a restriction of the domain. **So a vector is a representation of a table, never a sort**,
  as a width is (ADR 0083).
- **The theorem that decides when a loop may be vectorized** is the homomorphism theorem for lists
  (Bird 1987; Gibbons 1996). A fold may be split across lanes, or across GPU threads, exactly when it
  is a monoid homomorphism: its operation must be associative. ℤ's +, ·, &, |, ⊕, min and max are, so
  every integer fold the language writes may be reordered freely. **Floating-point addition is not
  associative.** Measured: LLVM refuses to vectorize a float sum unless told it may reassociate, and
  when told, the sum is about 2× faster **and gives a different answer**.
- **The language already has the three data-parallel forms**, each with its theorem: a table by rule,
  `array { e : k ∈ 0..n-1 }`, which is a map; a loop folding an exact associative operation, which is a
  homomorphism; and a loop of stores whose indices no two iterations share, which is dependence
  analysis. That last is what Pugh's Omega test was invented for, and the project already has it.
- **On Mojo: partly right.** Mojo is the most complete *explicit* design in a shipping language today:
  `SIMD[dtype, width]` as a type, scalars as width-1 vectors, compile-time parameters, and one language
  for CPU and GPU through MLIR. But it puts the width in the type, the choice ADR 0083 rejected for
  integers, and leaves portability across widths to the programmer. The literature's strongest
  results for **portable** data parallelism are elsewhere:
  - Halide separates the algorithm from its schedule;
  - Futhark and NESL flatten nested parallelism;
  - Dex types arrays by their index sets;
  - Lift derives code by rewrite rules.

  Their designs are closer to this language's, where the compiler chooses the representation from
  proofs (§3).
- **Recommended** (§9):
  1. No vector sort or type.
  2. The compiler vectorizes the three forms, from proofs.
  3. A float fold keeps its order unless the program states otherwise, which is wall 6's question.
  4. The facts are stated to hosts that vectorize, and explicit SIMD is printed only where measured
     to pay.
  5. Target intrinsics over vectors are host primitives over tables of static length.
  6. Shuffles and masks are rules.
  7. GPUs are a later target layer with the same algebra, not part of this step.

---

## 1. The issue

### 1.1 What LLVM offers

| group | members | count |
|---|---|---|
| the type | `<N x T>`, fixed; `<vscale x N x T>`, scalable (Arm SVE, RISC-V V) | |
| lane-wise instructions | every arithmetic, comparison and conversion instruction applies to vectors | |
| lane access | `extractelement`, `insertelement`, `shufflevector` | 3 |
| target-independent intrinsics | `vector.reduce.*`, masked loads and stores, gathers and scatters, `vp.*` (explicit vector length), `vector.splice`, `vector.reverse`, … | 158 |
| target-specific intrinsics | x86's 128- to 512-bit sets, AES, SHA, CRC32C, `pshufb`; Arm NEON and SVE; WebAssembly's SIMD128; GPU backends' | 10,101 in all, 61.5% of Σ_LLVM |

**And two optimizers that make vectors without being asked.** The loop vectorizer strip-mines a loop
into groups of lanes. The SLP vectorizer (Larsen and Amarasinghe, PLDI 2000) packs independent
scalar operations of one block into lanes. Both work only from facts the compiler can prove: no
aliasing, inbounds, no overflow, associativity. Those are the facts this language proves (ADR 0080).

### 1.2 What the four targets offer (§7 measures these)

| target | explicit SIMD | does the host vectorize by itself? | GPU |
|---|---|---|---|
| Go | `simd/archsimd`, **experimental** in Go 1.27, `GOEXPERIMENT=simd`, amd64 only: 44 types of fixed width (`Int8x16` … `Float64x8`, masks), methods named for the hardware (`ConcatPermute`, `Compress`) | **no**: the gc compiler emits scalar code for a sum and for `axpy` (measured) | none native |
| the JVM / Android | `jdk.incubator.vector` (present in JDK 17, still incubating); Android has no Vector API | C2's SuperWord (SLP) vectorizes simple loops; ART's loop optimizer vectorizes some | Android: Vulkan compute |
| the browser | JavaScript has none (SIMD.js was withdrawn); WebAssembly has SIMD128 | V8 does not vectorize JavaScript | **WebGPU**, compute shaders in WGSL |
| Windows (LLVM) | the IR's vectors, every x86 extension the machine has | yes, both vectorizers | D3D12, Vulkan, CUDA; LLVM's NVPTX and AMDGPU backends; DXIL and SPIR-V are LLVM-based |

So the four targets disagree about almost everything at the level of *how*. What they share is
*what*: a function applied at every index, and a fold.

### 1.3 What the language has today

- **A table is a function on [0, len)** (data.md), and **a rule** `array { e : k ∈ 0..n-1 }` is its
  tabulation, `table(n; λk. e)`. A rule fuses: a rule that is only read is never built (tables.md
  §2.4). And since ADR 0084's build, a rule's index carries its domain.
- **Iteration is `loop … again`**, a guarded recurrence. A fold is a loop with an accumulator.
- **Integers are ℤ** (ADR 0026), so every integer operation the program states is exact. Its algebra
  is the ring's, never the machine's.
- **Floats are F64**, IEEE 754's binary64, round-to-nearest. Wall 5 adds other formats, and wall 7 the
  rounding modes.
- **Element representations are chosen by place** (ADR 0083): a table of bytes is held in bytes. That
  is already the choice of lane width a vector register needs.

What is missing is the step from "a table whose elements are computed independently" to "a register
that computes several of them in one instruction", and the rule that says when that step preserves
the answer.

---

## 2. The algebra

### 2.1 A vector is a power

For a set S with operations Ω (an algebra), the **n-th direct power** Sⁿ is the set of functions
[0, n) → S, with each operation applied lane by lane:

```
(f ⊙ g)(k) = f(k) ⊙ g(k)          for every k ∈ [0, n)
```

**Birkhoff's theorem** (1935): a class of algebras defined by equations is closed under direct
products. So **Sⁿ satisfies every equation S does**: if + is associative and commutative on S, it is
on Sⁿ, lane by lane. Lane-wise vector arithmetic therefore never changes an answer the scalar program
would give, provided the lane operation is the scalar one. An `add <4 x i64>` is four `add i64`s.

**So a vector register is a representation** (ADR 0083's (H, d)) of a table of static length:
H = `<n x iK>`, d the lane-wise reading. It is chosen when a table's length is a constant the target's
register holds, and its operations are pointwise. That is the survey's recommendation (a), and it
needs no new sort.

### 2.2 A reduction is a homomorphism, and only associative operations are

A fold of an operation ⊙ over a table, h(t) = t₀ ⊙ t₁ ⊙ … ⊙ t_{n−1}, may be computed in lanes
(partial results per lane, combined at the end) or across threads (a tree) **exactly when ⊙ is
associative**. Then h is a monoid homomorphism from the free monoid (lists under ++), and any
bracketing gives one value:

```
h(x ++ y) = h(x) ⊙ h(y)
```

This is the **homomorphism lemma** of the Bird–Meertens formalism (Bird, "An introduction to the
theory of lists", 1987). The **third homomorphism theorem** (Gibbons, "The third homomorphism
theorem", JFP 1996) characterizes it: a function on lists is a homomorphism iff it can be computed
both left to right and right to left. Splitting into w lanes is splitting the list into w
interleaved parts, which needs commutativity too, or a lane order that the final combination
respects (§2.4).

| operation on | associative | commutative | so |
|---|---|---|---|
| ℤ: +, ·, &, \|, ⊕, min, max | yes | yes | any order, any split: vectorize freely |
| ℤ: −, / | no | no | not folds of a monoid; a loop with them is a recurrence |
| F64: +, · | **no** | yes | a reordered sum is a different number: §2.3 |
| F64: min, max (IEEE `minimum`, `maximum`) | yes | yes | vectorize freely |
| strings: ++ | yes | no | lanes must keep their order |

**The machine's integers are not the language's.** In C, `int` addition wraps, which is still
associative (ℤ/2ⁿ is a ring), so C vectorizes integer sums. In this language a sum is in ℤ, proven
inside the word (ADR 0019), and ℤ is associative. One subtlety: each *partial* sum must be proven
inside the word too. For a sum of nonnegative terms every partial sum is at most the whole, so the
proof of the whole covers the parts. With mixed signs it does not, and the vectorized form must be
proven on its own partial sums, or computed under `mod 2⁶⁴` where ADR 0082's fusion applies.

### 2.3 Floating-point addition is a commutative, non-associative magma

IEEE addition rounds after each operation, so (a + b) + c ≠ a + (b + c) in general: 1 + 2⁻⁵³ + 2⁻⁵³
summed left to right is 1, and right to left is 1 + 2⁻⁵². A vectorized sum adds in a different order,
so **its answer is a different number**, and §7 measures it: …76788393 against …76791671 on 2²⁴
values. The language's rule is that no transformation changes an answer (ADR 0009). So:

- **A float fold keeps the order the program wrote**, on every target, unless the program says its
  order does not matter.
- **Saying so is a claim that the result is one of a set of values**: the sums under every bracketing.
  That is a *relation*, not a function, which is exactly wall 6 (relations: "only deterministic
  refinements offered").
- **The algebraic alternative makes the fold associative.** Sum exactly, in a fixed-point
  superaccumulator wide enough for every binary64 (Kulisch; Neal, "Fast exact summation using small and
  large superaccumulators", 2015), and round once at the end. Or use a reproducible blocked summation
  (Demmel and Nguyen, "Parallel reproducible summation", IEEE TC 2015; ReproBLAS). The exact sum is a
  homomorphism, so it vectorizes and parallelizes, and its answer is the correctly rounded true sum,
  the same on every target and every width. It costs a few times a plain sum.

### 2.4 Lanes, strip-mining and the remainder

Vectorizing a loop over [0, N) at width w partitions its domain:

```
[0, N) = ⊔_{j < ⌊N/w⌋} [jw, jw + w)  ⊔  [w⌊N/w⌋, N)          N = qw + r, 0 ≤ r < w
```

The q full blocks run in lanes, and the r leftover iterations run scalar, masked (§2.6), or with an
explicit vector length (LLVM's `vp`). For a map this is a re-indexing, k = jw + ℓ, and nothing else.
For a fold of ⊙, each lane ℓ accumulates t_ℓ ⊙ t_{w+ℓ} ⊙ t_{2w+ℓ} ⊙ …, and the lanes are combined at the
end. That is a regrouping (associativity) and a reordering (commutativity), and §2.2's table says which
operations allow it.

**The width w is the target's choice, never the program's.** It may even be unknown when compiling:
Arm SVE's and RISC-V V's vectors are `vscale × n` lanes, vscale a constant of the machine read at run
time. Since a width is only a representation (ADR 0083), that costs the language nothing.

### 2.5 Dependence: when a loop of stores is a map

A loop `b[f(i)] := g(i, b[h(i)])` may run its iterations in lanes iff no iteration reads or writes
an element another one writes. That holds when there are no integers i ≠ i′ in the domain with
f(i) = f(i′), or f(i) = h(i′) for i′ < i (the write-after-read and read-after-write conflicts). For
affine f and h this is a question about integer points in a polyhedron. **That is the problem the
Omega test was invented for**: Pugh, "The Omega test: a fast and practical integer programming
algorithm for dependence analysis", Supercomputing 1991. The project's `presburger/` package
implements it (presburger-2026-10-11), as the refinement layer's witness. The polyhedral model
(Feautrier, "Some efficient solutions to the affine scheduling problem", 1992) builds schedules on the
same theory.

The language makes the analysis smaller than C's. Views are exclusive (ADR 0084), so two buffers never
alias, and only one buffer's index functions are compared. A rule needs no analysis at all, since its
elements are independent by construction.

### 2.6 Shuffles and masks are rules

- **A shuffle** `shufflevector a, b, σ` is precomposition with an index map: the table
  `array { (a ++ b)[σ(k)] : k ∈ 0..m-1 }` for a static σ. A byte reversal, lib/bytes' big-endian
  decode, is σ(k) = n − 1 − k. A broadcast is the constant σ(k) = c. An interleave is σ(k) = (k mod 2)·n
  + ⌊k/2⌋. These are rules, which the language has. The compiler recognizes a static σ and prints the
  shuffle.
- **A mask** m is a table of Booleans, and a masked operation is the pointwise conditional,
  `array { if m[k] { a[k] } else { b[k] } : k ∈ 0..n-1 }`, LLVM's `select` on vectors. A masked load
  reads only the lanes in m, a restriction of the domain, which is how the remainder of §2.4 is done
  without reading past the table: the rule's domain obligation (ADR 0084's build) is exactly what makes
  the masked lanes safe.

### 2.7 A GPU is the same algebra at a larger index set

A **kernel** is a function run at every point of an index space, a grid G = [0, X) × [0, Y) × [0, Z).
In the language's terms it is a rule over G: `array { f(p) : p ∈ G }`. GPUs add three things the
algebra must account for:

| GPU notion | algebra | in this language |
|---|---|---|
| a launch over a grid | a tabulation over an index set | a rule over a product of ranges |
| a reduction across threads | a tree fold, legal iff the operation is associative (§2.2) | a loop folding a monoid |
| global, shared and private memory | regions of different kinds (LLVM's address spaces) | views of a kind (ADR 0084, Part II §14.17) |
| a barrier inside a block | a synchronization point between phases | concurrency's (ADR 0081): a phase is a scope |
| moving data to and from the device | a copy between regions | an explicit move of a view between kinds |

**SIMT** (Lindholm et al., "NVIDIA Tesla: a unified graphics and computing architecture", IEEE Micro
2008) runs the kernel's threads in warps of 32 lanes. That is SIMD again, with the hardware handling a
lane's divergent branch by masking, which is §2.6.

---

## 3. The literature, and where Mojo stands

| language or system | the idea | what it contributes here | what it does not |
|---|---|---|---|
| **APL, J** (Iverson 1962; rank, Iverson 1987) | arrays as the unit, operations lifted over any rank | the direct power as a programming model | no static guarantees; interpreters |
| **NESL** (Blelloch, CACM 1996), flattening (Blelloch and Sabot 1990) | nested data parallelism, compiled to flat vector operations | nested maps become one flat map; work–depth cost model | a research language |
| **Sisal** (McGraw et al. 1985) | single-assignment arrays, parallel loops | no aliasing makes parallelism provable, this language's premise | |
| **ISPC** (Pharr and Mark, InPar 2012) | SPMD on SIMD: write one lane's program, the compiler runs w lanes with masks | divergent control flow as masking (§2.6) | C-like, x86/Arm CPUs |
| **Halide** (Ragan-Kelley et al., PLDI 2013) | the **algorithm** (what each pixel is) separated from the **schedule** (vectorize, tile, parallelize, GPU) | the strongest portability results: one algorithm, schedules per machine; the schedule cannot change the answer | a DSL for image pipelines; schedules written by hand or searched |
| **Futhark** (Henriksen et al., PLDI 2017) | a pure functional array language for GPUs; flattening; **uniqueness types for in-place updates** | the closest relative: its uniqueness types are this language's linear buffers, its `map`/`reduce`/`scan` are §2's forms, it requires `reduce`'s operator to be associative | GPU-first; no host ecosystems |
| **Dex** (Paszke et al., ICFP 2021) | arrays typed by their **index sets**, `n => a`; pointful loops that stay parallel | a table as a function on an index set, typed so | research |
| **Lift** (Steuwer, Fensch, Lindley and Dubach, ICFP 2015) | performance portability by **rewrite rules** over map/reduce algebra, searched per device | the derivation route: each transformation an equation | research |
| **Triton** (Tillet, Kung and Cox, MAPL 2019) | block-level GPU programming, the compiler handles threads within a tile | the tile as the programmer's unit | GPUs, Python-embedded |
| **The polyhedral model** (Feautrier 1992; Pugh 1991) | loops as integer polyhedra, dependences as integer constraints | §2.5's decision procedure | affine programs |
| **Rust `std::simd`, Zig `@Vector`, C++26 `std::simd` (P1928), Google Highway** | portable fixed-width vector types in a systems language | explicit control, one source across ISAs | the width in the type |
| **Julia** (SIMD.jl, LoopVectorization.jl) | vectors as types, and a macro that vectorizes loops | the macro is Halide's schedule in miniature | |
| **Mojo** (Modular, 2023–) | `SIMD[dtype, width]` a struct; a scalar is `SIMD[dtype, 1]`; compile-time parameters; `vectorize` and GPU kernels in one language over MLIR | the most complete explicit design in a shipping language; scalars as width-1 vectors is the embedding S ↪ S¹, elegant | the width is a type parameter, the representation chosen by the program; portability across widths is the programmer's (`simdwidthof`) |

**On Mojo, specifically.** It is fair to call Mojo the best-integrated *explicit* data-parallel design
in a general-purpose language today:
- one language for scalar code, SIMD and GPU kernels;
- compile-time metaprogramming, which is this language's static level by another name;
- MLIR underneath, so new hardware is a dialect.

It is not the best answer to this language's question, because its central choice is the one
ADR 0083 rejected: a value's representation written in its type. `SIMD[float32, 8]` and
`SIMD[float32, 16]` are different types, and a program portable across both is written over a width
parameter.

The designs whose results on *portability* are strongest put the width with the compiler or a
schedule: Halide, Futhark, Lift. Their algebra, maps and associative reductions over index sets, is
§2's. And none of them, Mojo included, makes the float-reassociation question part of the language's
meaning. Each takes it from a flag (`-ffast-math`), from an operator the programmer promises is
associative (Futhark's `reduce`), or from accepting the hardware's order (GPU atomics). §2.3 proposes
to decide it.

I could not run Mojo here, so nothing about its speed is claimed.

---

## 4. The type system

**No vector sort, and no width in any type.** `Array(S)` is the type, and a table whose length is a
constant is a candidate for a register. The decision is the representation's (ADR 0083), by place:

| where a table lives | its representation |
|---|---|
| a constant length that fits a register, operated on pointwise | a vector register, `<n x T>` |
| a loop over a long table, its body a map or an exact fold | the same table in memory, the loop strip-mined (§2.4); the target vectorizes it, or the printer does |
| a host primitive's parameter of declared length | the host's vector type (`__m128i`, `Int8x16`, `v128`) |
| anything else | as today |

**The facts a vectorization needs are ones the language proves already**:
- bounds (the decision);
- no aliasing (views, ADR 0084);
- no overflow, for each lane's partial result (§2.2);
- a rule's independence, by construction;
- a fold's associativity, by the operation's algebra: integer +, ·, &, |, ⊕, min and max yes, float
  + and · no.

The new rule is the last one, and it is a property of the operation, not a check on the program.

---

## 5. Examples

**A map: the sum of two tables, lane by lane.**

```
fn add(a ∈ Array(Int), b ∈ Array(Int)) (Array(Int)) where len(a) == len(b) {
    array { a[k] + b[k] : k ∈ 0..len(a)-1 }
}
```

A rule: every element independent. On LLVM the loop that fills it is vectorized at the machine's
width (`<4 x i64>` with AVX2), each lane's `add nsw` carrying its proof. On the JVM, C2 SuperWord
vectorizes the filled array. On Go the printer emits scalar code unless `archsimd` is measured to pay
(V4). On JavaScript it stays scalar.

**An exact fold: an integer sum.**

```
fn total(a ∈ Array(0..2^31-1)) (0..2^62) where len(a) <= 2^31 {
    loop i = 0, s = 0 { i < len(a) -> again(i + 1, s + a[i]); else -> s }
}
```

+ on ℤ is associative and commutative, and every partial sum of nonnegative terms is at most the
total, which is proven inside the word. So the lanes may each sum a residue class, and LLVM does it
(measured, §7).

**A float fold: the gauntlet's `dot`.**

```
fn dot(a ∈ Array(F64), b ∈ Array(F64)) (F64) where len(a) == len(b) {
    loop i = 0, s = 0.0 { i < len(a) -> again(i + 1, s + a[i] * b[i]); else -> s }
}
```

It keeps its order: the result is the left-to-right sum, the same number on every target, which is
what hand-written Go computes. It is vectorized only if the program states that any bracketing will
do (wall 6), or sums exactly (§2.3).

**A recurrence: CRC-32 and FNV** (examples/hash). crc ← tab[(crc ⊕ b) & 255] ⊕ (crc >> 8) carries
crc from one byte to the next. It is no fold of a monoid on the bytes, so it is never vectorized as
written. Its known vector form, CRC folding by carry-less multiplication (`pclmulqdq`), is a
different algorithm: the remainder in 𝔽₂[x] is linear, so blocks can be combined. That is the program's
algebra to state (bits-research.md §1.4), or a host primitive's (V5).

**A shuffle: lib/bytes' big-endian read**, σ(k) = n − 1 − k, `bswap` or `pshufb`. And a host
primitive over a table of static length, x86's `pshufb`, declared Tier 2 (V5):

```
fn pshufb(a ∈ Array(0..255), sel ∈ Array(0..255)) (r ∈ Array(0..255)) pure
    where len(a) == 16 && len(sel) == 16 ensure len(r) == 16 { host … }
```

**A GPU kernel, later (V7)**: a blur, `array { (img[p - 1] + img[p] + img[p + 1]) / 3 : p ∈ 1..n-2 }`, a
rule over an index set, which a GPU target would launch over a grid, with the edges handled by its
domain.

---

## 6. Where it meets the other walls

- **Wall 5 (float formats).** F32 doubles the lanes per register. The format is a representation
  (Figueroa's subsets of F64), so the lanes follow.
- **Wall 6 (relations).** A reassociated float fold is a relation, a set of possible results. Whether
  a program may ask for one is that wall's decision. This wall only shows that the question arises at
  the first float vector.
- **Wall 7 (the float environment).** A fixed rounding mode applies lane by lane, so it is unchanged.
- **Wall 3 (pointers, ADR 0084).** No aliasing between views is what makes §2.5 cheap. A GPU's memory
  spaces are view kinds.
- **ADR 0081 (concurrency).** A multi-core parallel loop is the same algebra across threads. Its
  fork–join fragment (C4) is a parallel map, and a reduction across threads needs §2.2's associativity.

---

## 7. Measured

On this machine (LLVM 23.1.4, `clang -O2`, default x86-64 target unless noted):

| loop | vectorized at `-O2`? |
|---|---|
| `long s += a[i]` over `int` | **yes**, `<4 x i32>` lanes |
| `double s += a[i]` | **no** |
| the same with `-fassociative-math -fno-signed-zeros -fno-trapping-math` | **yes**, `<2 x double>` |
| `y[i] += k * x[i]` with `restrict` | **yes**, `<2 x double>` |
| a recurrence `c = (c >> 8) ^ (c * 31 + a[i])` | **no** |

**The price of the float order**, a sum of 2²⁴ doubles of magnitudes from 2⁻²¹ to 2¹⁹, with
`-march=native`, best of 30:

| | time | result |
|---|---:|---|
| in order | 9–10 ms | −159651986.76788393 |
| reassociated (vectorized) | 5 ms | −159651986.76791671 |

The reassociated sum is about twice as fast **and a different number**: §2.3, measured.

**Go 1.27**: `go build -gcflags=-S` of an `int32` sum and of `axpy` shows scalar `ADDQ`, `MULSD` and
`ADDSD`, no vector instruction. `simd/archsimd` exists, experimental, behind `GOEXPERIMENT=simd`,
amd64 only, with 44 fixed-width types. **JDK 17** ships `jdk.incubator.vector`. **Node 26**: no SIMD in
JavaScript.

Not measured: Go's `archsimd` against scalar Go, the JVM's SuperWord on generated code, WebGPU. Each
is a measurement for the step that would use it.

---

## 8. Candidates

**A. A vector type in the language** (Mojo, Rust, Zig, C++26): `Vec(n, S)` or `SIMD(S, n)` with
lane-wise operators.
- Explicit control, and code that reads like the hardware.
- But it puts the width in the type (ADR 0083's rejected option). It is a second spelling of a table
  of static length, which fit-research would count as a misfit (two forms, one meaning).
- It is non-portable by default: a width fits one target's registers.

**B. Auto-vectorization only, by the hosts.** Print scalar code with every fact stated and let LLVM and
C2 vectorize.
- Free where the host vectorizes.
- Nothing on Go or JavaScript, and nothing for float folds.

**C. A representation, chosen from proofs** (recommended, V1–V6):
- no new type;
- the three forms of §1.3 vectorized where their theorems hold;
- the facts stated to hosts that vectorize, and explicit SIMD printed only where a host offers it and
  a measurement shows it pays;
- the floats' order kept until wall 6 decides otherwise.

**D. A schedule language** (Halide): the program states *what*, and a separate schedule states *how*:
vectorize this loop at width 8, tile that one, run this on the GPU. It is the most powerful option for
portability-with-control, and it fits the language's two levels: a schedule is a static value. It is
not needed until a program's measured speed asks for a choice the compiler cannot make. Recorded as the
next step after C, if one is needed.

**E. GPU kernels now.** The algebra is §2.7's, and the host side (launch, transfer, synchronization)
is a target layer per platform: WebGPU, Vulkan, D3D12. That is a large step of its own (V7).

---

## 9. Decisions for hamza

**V1. A vector is a representation of a table of static length, never a sort.** No vector type, no
width in any type. A register `<n x T>` is chosen for a table whose length is a constant the register
holds and whose operations are pointwise (ADR 0083's place rule).
*Why*: Birkhoff, so the lane-wise algebra is the scalar one; and a width is never part of what a value
is (ADR 0083). **Recommended.**

**V2. The compiler vectorizes the language's three data-parallel forms, each by its theorem:**
- a rule `array { e : k ∈ … }` is a map, independent by construction;
- a fold by an associative operation, the homomorphism lemma: ℤ's + · & | ⊕ min max, and F64's min and
  max, with each lane's partial results proven inside the word;
- a loop of stores whose index functions the Omega test shows conflict-free (§2.5).

On LLVM the compiler states the facts and the vectorizers act. Elsewhere it is the printer's, by V4.
**Recommended.**

**V3. A float fold keeps the order the program wrote, on every target.** A reordered sum is a
different number (measured), and staging never changes an answer (ADR 0009). Permitting a reordering
is a relation, wall 6's decision. Exact summation (superaccumulators, ReproBLAS) is the algebraic way
to make a float sum associative, and is recorded for a program that needs a fast, reproducible float
sum. **Recommended.**

**V4. Explicit SIMD is printed only where the host offers it and a measurement shows it pays:**
- LLVM vectorizes by itself;
- the JVM's C2 does in part;
- Go needs `simd/archsimd`, experimental and amd64-only, so Tier 2, measured against scalar Go before
  it is used;
- JavaScript has none, and WebAssembly's SIMD128 would come with a WebAssembly target.

**Recommended.**

**V5. A target's vector intrinsics are host primitives over tables of static length**, declared by
hand as every host primitive is (ADR 0022), each with its `where len(a) == n` and its undefined
behaviour as its `where`: `pshufb`, AES-NI, `pclmulqdq`, CRC32C, NEON's. They are Tier 2, with
portability computed. This is the survey's (a), and how the 10,101 intrinsics enter Σ_LLVM.
**Recommended.**

**V6. Shuffles and masks are rules**: a shuffle `array { v[σ(k)] : k }` with a static σ, a select
`array { if m[k] { a[k] } else { b[k] } : k }`, recognized by the printer. Scalable vectors are a
width read at run time, which is the representation's business. **Recommended.**

**V7. GPUs are a later target layer with the same algebra**:
- a kernel is a rule over a grid;
- a reduction across threads is V2's monoid;
- device memory is a view kind (ADR 0084);
- barriers are concurrency's (ADR 0081).

They are not in this step. The trigger is a program that needs one, an image or numerical pipeline,
with Futhark and Dex as the models and WebGPU, Vulkan and D3D12 as the hosts. **Recommended: record,
do not build.**

## 10. Not here

- **Multi-core parallel loops**: ADR 0081's fork–join fragment, the same algebra across threads.
- **A schedule language** (§8 D), until a program's measurement asks for a choice the compiler
  cannot make.
- **Exact summation** (§2.3), until a program needs a fast, reproducible float sum.
- **Float formats, relations, the float environment**: walls 5, 6 and 7.
