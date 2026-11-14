# Reaching the full host: why K(ℕ\*)'s inner loop is 3–4× `math/big`'s, and the ways out

Date: 2026-11-14. hamza: "I see this as a test of the language and the compiler. We expect to be able
to reach for LLVM instructions and take advantage of the full host. If we are failing to do so, this
is the time to think, research and decide. What are the roadblocks, and what are the ways to fix
them?" This is a research document. Nothing in it is built, and it ends with the decisions.

## 1. The finding

At about 10,400 words, with the operands built alike (squaring-2026-11-14):

| | ours, windows | Go `math/big` | ours / Go |
|---|---|---|---|
| a square | 11.5 ms | 3.65 ms | 3.1 |
| a product | 21.4 ms | 5.2 ms | 4.1 |

- **The algorithms are the same.** Karatsuba above a threshold of tens of words, a square by
  polarization, schoolbook leaves; Go's threshold is 40, ours 64. The C model of our design (`kara/k.c`)
  took 17 ms for this product, so the gap is not the J translation.
- **The leaves are 68% of a product** (the C model's phases), and a leaf is Algorithm M's row. Ours
  runs 0.88 ns a digit product. Go's row, `addMulVVW`, is hand-written amd64 assembly (as GMP's
  `mpn_addmul_1` is), of the order of 0.3 ns. So the gap is the inner loop, and this document asks why
  the language and compiler cannot reach it.

## 2. The roadblocks, each established

**R1. The row's form is one serial carry chain.** The emitted leaf row (`kmac` in `nat.mul`, read off
clang's assembly; every bounds test is gone since ADR 0109's clamps) is eight instructions a digit,
unrolled twice:

    mov y → rax ; mul x_i ; xor ; add o_i, k ; setb ; add lo, k ; adc hi, c ; mov → o_i

The carry k passes through add → setb/adc → add on every digit: three dependent operations, about 3
cycles, against `mul`'s throughput of one a cycle. This is the algebra of operand scanning: o + y·x
computed digit by digit as one fold whose state is k. **No instruction selection repairs a
dependency chain**: `-march=native` (BMI2's `mulx`, ADX) made the whole product 7% faster (22.6 →
21.0 ms).

**R2. LLVM IR has one carry, and its x86 backend uses one flag.** The kernels that break R1's chain
are GMP's and Go's ADX loops: o + y·x = o + L + B·H, where L and H are the numerals of the digit
products' low and high halves (`mulx` writes no flag). They add both into o in one pass, through two
independent full adders: `adcx` on CF and `adox` on OF. Measured: two carry chains written with
`_addcarryx_u64` under `-march=native` (scratchpad `kara/adx2.c`) compile to `adc`s with a `setb` and
`addb $-1` round trip per digit for each chain, and **no `adcx` or `adox` at all**.
`llvm.x86.addcarry.64`, the intrinsic ADR 0108 prints, is the same single flag. So the two-flag
kernel is unreachable from LLVM IR, and reachable only by inline assembly.

**R3. The target states no CPU.** `targets/windows` names the triple `x86_64-pc-windows-msvc` and
nothing more, so every function is compiled for baseline x86-64: no BMI2, no ADX, no AVX2. Even an
assembly kernel needs ADX, a property of the machine the program runs on (Broadwell, 2014, and
later). The target system has no notion of a CPU feature, of a function compiled for one, or of
choosing between two at run time (cpuid). Go's `math/big` chooses at run time, and GMP builds per CPU
(fat binaries choose at run time).

**R4. An inline-assembly host operation is opaque, so a kernel must be a whole loop.** ADR 0103's
`divq` is one instruction in a template, and that works because no flag crosses it. A carry chain
cannot cross an `asm` statement, which clobbers the flags. So the ADX kernel is not an instruction the
language composes: it is a function over tables, `addmul_1(o, x, n, y)`, as GMP's `mpn` layer is.

**R5. The language cannot state such a function's meaning.** The kernel's contract is

    ε(o′) = ε(o) + y · ε(x),     ε(t) = Σ_{i<n} t_i Bⁱ,

and ε is the numeral's abstraction function, ADR 0083's (H, d). Today a host declaration's `ensure` speaks of integers and
tables element by element, not of a table read as a numeral. So a kernel would be trusted, its
contract written in a comment. That would be a first for this project, whose host declarations are
checked against the host (ADR 0022) and whose library is a theory checked once (ADR 0035).

**R6. This machine cannot settle a kernel's bound today.** The same row measured 0.88 ns and then
3.85 ns in one session (thermal state), and a naive one-digit ADX loop with `jrcxz` ran 21 ns. A
kernel's speed has to be measured on a quiet machine against GMP's and Go's own kernels, not a
sketch of them.

## 3. The ways out

**F1. Restructure the row in the language, so that one flag suffices.** Product scanning (Comba):
column k is Σ_{i+j=k} x_i y_j in a three-word accumulator, whose words form three chains that
pipeline instead of one that serializes. Measured in C under clang: no better than the row (2.68
against 2.37 ns in one round; the machine was in its slow state). Several rows at once (GMP's
`addmul_2`) shorten the chain per product, but their step reaches 2B² and needs a carry of 65 bits.
Both are expressible in the language today, with the double word and the full adder. Neither has
been shown to reach the bound under LLVM.

**F2. The target states its CPU, and functions are compiled for it.** Target data names a CPU level
(x86-64-v3) or a feature list, printed as `"target-cpu"` and `"target-features"` on every function.
It is a fact of the target, as the triple is (ADR 0093 item 5). Alone it is worth 7% on the row (R1),
but it is the premise of F3, and AVX2 matters elsewhere (ADR 0085's vectors).

**F3. Kernels as host functions with stated contracts, chosen by the CPU.**
- `targets/windows` declares GMP's three primitives over tables: `addmul_1`, `add_n` and `sub_n`
  (o ± x with a borrow out). Each is a whole-loop assembly body, written from Go's amd64 `math/big`
  (BSD licence; GMP's is LGPL), in two forms: ADX, and baseline `mul`/`adc`.
- They are chosen at run time by cpuid, once per process. The decision is a fact of the run, not of
  the program, and the generated loop is the fallback.
- K(ℕ\*)'s rows call them.
- Each declaration's `ensure` states ε's law (R5), which needs the logic to name ε, and a witness
  against Algorithm M checks it with a planted fault, as ADR 0108's adder is checked.

This is the move ADR 0086 made for roundings: a target realizes an operation by a declaration whose
`ensure` states it.

**F4. Make LLVM select the two-flag form.** It would be an LLVM backend change: modelling OF as a
second carry, and pattern-matching two interleaved addcarry chains into `adcx`/`adox`. That is
outside the project, which treats host compilers as black boxes (CLAUDE.md).

**F5. Make the leaves a smaller share.** Toom-3 (rank 5, n^1.465) above some hundreds of words, and
the passes' copies replaced by offsets (33% of a product in the C model). This narrows the gap
without touching R1–R2, and both are owed anyway.

## 4. What the algebra says

The row is a fold with one state, so its latency is one carry chain a digit, whatever the
instructions. To go faster the computation must have two independent states. That is the
decomposition o + L + B·H, two folds whose sum is the row, and in hardware two flags. The language
can state that decomposition, and can state each fold as a full adder (ADR 0108). **What it cannot do
is have LLVM keep two carries in two flags (R2).** So on this host, the fastest form of a numeral's
row is reachable only as a whole-loop kernel (R4) whose meaning is stated through the numeral's
abstraction function (R5), on a CPU the target names (R3).

The honest reading of the test: the language reaches every LLVM instruction it was asked to, `adc`,
`sbb`, `mulq`, `divq` and i128. It cannot reach an instruction **LLVM does not select from any IR**.
The full host is then the target's own code, declared and stated. It is not more IR.

## 5. The decisions, hamza's

- **Q1. A kernel as a target's host function over tables (F3)?** It is trusted code outside the
  proofs, its contract stated through ε and checked by witnesses. Recommended: yes, since it is the
  only route to the bound (§4).
- **Q2. The CPU a target states (F2), and run-time choice between kernels?** Recommended: target
  data names the features, and a kernel declares the features it needs. The printer emits one body
  per feature set and a cpuid choice, once per process. Baseline remains the default, and a program
  never fails on an older CPU.
- **Q3. ε in the logic, for a kernel's `ensure` (R5)?** Recommended: as ADR 0083's abstraction
  function of K(ℕ\*)'s representation. It is the claim the library already relies on in every
  comment ("the numeral's image"). It is stated, checked by witness, and never printed (ADR 0049's
  logic).
- **Q4. The order?** Recommended:
  1. measure the bound first, on a quiet machine: Go's own `addMulVVW` and GMP's `mpn_addmul_1`, ADX
     and baseline, against our row, at the leaf sizes 32–128;
  2. then F2 and F3 for `addmul_1` alone, program first (fast doubling);
  3. then `add_n`/`sub_n`, and F5's Toom-3 and offsets;
  4. then divide-and-conquer printing.
