# 0123 — A triple's runtime is a library in the language, linked into every program

Date: 2026-11-17
Status: Accepted. ADR 0122 item 5's (b), decided by hamza 2026-11-17 ("Go with your recommendations
on D1–D7"): Knuth's D over `divl`, in the language, "where LLVM would call MSVC's `__aulldiv`".
This ADR says where that is, which the algebra decides. Built with
[d6-2026-11-17](../../gauntlet/results/d6-2026-11-17.md).

## Context

On i686 a 64-bit `/` or `%` failed to link: `bits-edges` against `__allrem`, `word-wide` against
`__alldiv` (triple-research.md §4.6). Measured with LLVM 23.1.4 at -O2, every 64-bit instruction
Σ_LLVM offers is expanded inline on i686-pc-windows-msvc except four, at the divisors the code
generator cannot see through:

| | expanded inline | called |
|---|---|---|
| `udiv`, `urem` | by any constant (multiply-high), and with both operands known below 2³² (`divl`) | `__aulldiv`, `__aullrem` |
| `sdiv`, `srem` | with both operands known in 32 bits (`idivl`) | `__alldiv`, `__allrem`, **a constant divisor included** |

`mul`, the shifts, the overflow intrinsics, `ctlz` and `ctpop`, the conversions to and from F64, and
`mul`, `udiv` and the comparisons at i128 are all inline. The four are stdcall: the arguments on
the stack, popped by the callee, the result in EDX:EAX. On x86_64 the instructions call nothing,
with `"no-builtins"` keeping loops from becoming `memset` (llvm.md §2).

So the instructions of Σ_LLVM, and their meaning, are the same on both triples. What differs is
that i686's code generator leaves four symbols free. Σ_LLVM on a triple t is closed only together
with **Σ_RT(t)**, what t's code generator calls: ∅ on x86_64, these four on i686. It is a parameter
of the triple's model, as Σ_OS is a parameter of T_LLVM (ADR 0093).

## Decision

1. **A triple's runtime is target data** naming a module: `runtime "rt/i686.ll"` in
   `targets/windows32/windows32.oro`. The file is found under the target's library directories,
   compiled by the program's clang at the same -O2, and linked before the libraries, as a C runtime
   would be (`emit.buildScript`). A triple whose code generator calls nothing names none.
2. **Its functions are the language's**: `lib/rt/i686.oro`, whose exports are named by their
   symbols, written over the triple's one primitive outside Σ_LLVM, the 2-by-1 division `divl`
   (`llvm/isa.udiv_2by1_32`, `targets/windows32/x86.oro`), by Warren's divDu (Hacker's Delight,
   2nd ed., §9-5), Knuth's D on 32-bit digits. Every operation of it is decided, and every C
   `uint64_t` step is a residue mod 2⁶⁴ in ℤ (ADR 0082). `gen -runtime x86_stdcallcc` prints each
   export also at its symbol, `@"\01__aulldiv"`, at the convention the code generator calls it by,
   as a call to the export. `gauntlet/regen.txt` lists the file, so `cmd/check` regenerates it and
   requires it byte-identical.
3. **The runtime is defined without itself.** A division by a word inside it would be a call LLVM
   lowers to the helper, a recursion with no base. So the runtime, compiled for its triple, calls
   none of its symbols (`TestTheRuntimeCallsNoHelper`, with a planted fault: a runtime whose
   quotient is the language's own `/` is found calling `__aulldiv`).
4. **ι is unchanged.** `/` is `lang.div, llvm.sdiv, llvm.udiv_nat` on every member (llvm.oro).
   LLVM keeps its own expansions, a constant divisor and 32-bit operands, and only what it leaves
   reaches the runtime, as it would reach libcmt in a C program built by the same clang.
5. **A host operation's declared result set is a fact of its application for the refinement layer**,
   as the IR's intervals already read it (`emit.seedResultSets`). A `where`, which only that layer
   decides, could not use the set: the normalization shift `1 << s`, s from `clz`, was refused.

## Why not

- **A realization of `/` and `%` per member, a term inlined at each site.** Σ_LLVM would then be a
  different signature per member, whose closedness depends on which expansions LLVM performs:
  unsigned by a constant is inline, signed by a constant is a call. Every term over Σ_LLVM that
  divides would have to be restated per member: `render`, `map_n`, `emod`, Σ_OS's `class`, and the
  printer's own. One missed is a link failure no check sees until a program reaches it. It would
  also copy about sixty instructions into every division site. A per-site realization chosen by
  facts (a divisor proven below 2³²) stays available as an optimization, measured, on top of this.
- **libcmt's helpers**, (a): a C runtime linked for four functions, where the target links none.
- **compiler-rt's builtins**, (c): not installed with the release, and a C runtime too.
- **`-mllvm -expand-div-rem-bits=32`**: LLVM's ExpandLargeDivRem would expand every division wider
  than 32 bits inline. That is closed and correct, but it is a shift-subtract loop of 64 steps
  against one or two `divl`, and a backend option is a flag the target does not state as data.
- **The helpers in hand-written LLVM IR or assembly**: the algorithm is the language's and decided.
  The one primitive outside Σ_LLVM is `divl`, declared with its domain; off that domain the
  instruction raises #DE, a trap, never a wrong answer.

## Consequences

- windows32's programs link with no free symbol: `bits-edges` and `word-wide` pass on the triple.
- A third member names its own runtime, or none.
- The runtime is checked the way every file is: its decision and its emission by `cmd/check`, its
  answers by `TestTheTriplesRuntimeDivides` against Go's, and its well-foundedness by the compiled
  object.
