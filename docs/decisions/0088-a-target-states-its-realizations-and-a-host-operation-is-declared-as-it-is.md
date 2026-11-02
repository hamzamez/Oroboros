# 0088 — A target states its realizations of the language's names, and a host operation is declared as it is

Date: 2026-11-02
Status: Accepted (hamza, 2026-11-02: "the language operator << >> has the same meaning regardless of
any target. but shl is not the language, this a target instruction. all we care about is can we
express as is and reason about it"). Built for `windows-llvm`
([llvm-2026-11-02](../../gauntlet/results/llvm-2026-11-02.md)); the four targets before it are found by
spelling until each is restated. Refines [ADR 0021](0021-declarations-are-theories.md)'s "a target is
a model" and [ADR 0080](0080-the-native-targets-print-llvm-ir.md) item 7.

## Context

A target is a model of the language's signature Σ_L (ADR 0021): it interprets each of the language's
names, `+`, `<<`, `=`, `string_of`, `abandon`, by something its host computes. Until now the compiler
found that interpretation by guessing from names (emit/target.go's `langOps`, `findBySpelling`): the
language's `<<` was the first declaration named `shl`, then `<<`.

The guess assumes a host's operation of that name *is* the language's operation. It is for Go's `+` on
the word and for LLVM's `and`. It is not for LLVM's `shl`, which is poison for a count of 64 or more,
where the language's `<<` is a·2ᵏ for every k ≥ 0; nor for `srem`, undefined for −2⁶³ by −1, where
ℤ's remainder is 0. To keep `<<` total under the guess, the first windows-llvm declared `llvm.shl` as
a total form of three instructions, so `llvm.shl` in a program did not mean LLVM's `shl`. Its domain
was hidden, and the operation the target offers under LLVM's name was not LLVM's. Two different
things were one declaration:
- **a host operation**, which is what it is, with its own domain;
- **a realization**, the target's interpretation of a language name, which must agree with the
  language's meaning on all of that name's domain.

## Decision

1. **A host operation is declared as it is**: what it computes, and its domain (the inputs where it is
   neither poison nor undefined) as its `where` or its parameters' sets, an obligation at every call.
   `llvm.shl(a ∈ Int, b ∈ 0..63)`, `llvm.sdiv` where b ≠ 0 and not −2⁶³ by −1. A module named after a
   host holds only that host's operations.
2. **A target states its realization of each language name**, `realize OP = A, B … [checked C]`: an
   interpretation map Σ_L → the target's declarations, written down, not inferred. A target that states
   any states all, and none of its names is found by spelling.
3. **A realization agrees with the language on the name's whole domain.** Where a host operation does
   on only part of it, the realization is a form over host operations that splits the rest off, in the
   target's own module of such forms (windows-llvm's `lang.oro`): `<<` past 63 by `select`, which takes
   no poison from the operand it does not choose.
4. **Several realizations are chosen per instance by proof**: the first realizes the name everywhere;
   each later one where its parameters' sets hold the operands' proven facts, the narrowest such
   (ADR 0083 item 2's mechanism, ir/final.go). So `<<` is LLVM's own `shl`, with the flags the IR
   proved, where the count is proven below 64, and the total form elsewhere.
5. **The checked form is part of the realization**, `checked C`, the trap `-checked` asks for (ADR 0019).

## Why not

- **Keep guessing by spelling, and make the host's instruction total** (what was built first). It
  falsifies the host operation: `llvm.shl` would not be LLVM's `shl`, and a program writing it would
  get a domain it did not ask for. The survey of Σ_LLVM (llvmsurvey-2026-10-27) is a list of operations
  as they are; a declaration that is not one is no longer that survey's.
- **Narrow the language's operation on this target**, `<<` with `where b <= 63` as MASM's `sar` did. A
  target may neither decline nor narrow a language construct (CLAUDE.md); this refuses programs the
  language accepts, on one target, for a reason that is the host's.
- **Guess better** (a priority among spellings, a suffix convention, as `and_u32` names a
  realization). Any rule over names conflates a host's name with the language's meaning: LLVM's
  operations are named by words that the other hosts' realizations also use (`and`, `shl`, `sar`). The
  interpretation of a signature is a function, and a function is stated.
- **Write a realization as a definition in the language** over host operations (`if k <= 63 {
  llvm.shl(a, k) } else { 0 }`). The IR would see a call and a branch where it now sees ℤ's `<<`, and
  lose its interval transfer, its exactness obligation (the `else` answers 0 for a ≠ 0) and the flags.
  A realization is a primitive the IR classifies as the operation it realizes.
- **Restate every target now.** The four before this one are found by spelling and agree with the
  language as they are (their emission is pinned byte for byte); each is restated when next touched,
  MASM not at all, since it retires.

## Consequences

- `targets/windows-llvm/llvm.oro` is LLVM's operations as they are; `lang.oro` the forms realizing the
  language where an instruction is not its operation on the whole domain (`~`, `<<`, `>>`, `mod`, `/`,
  `%`, the checked arithmetic, `trap_if`, `abandon`, the allocator); `windows-llvm.oro` the `realize`
  map. `kernel32.WriteFile` is Win32's, with the segment and the DWORD count as its domain.
- A `<<` whose count is proven below 64 prints as `shl nuw nsw`, its flags from the IR; elsewhere the
  total form. `-checked` gives `<<` a trap (`lang.shl_checked`).
- `/` and `%` are total on every divisor but 0: a divisor −1 is read as 1, so `%` gives ℤ's 0 and `/`
  gives −a, which for −2⁶³ is −2⁶³ as Go and the JVM answer. That quotient, 2⁶³, is outside the word,
  and no target's analysis decides it: a separate defect, found here (llvm-2026-11-02).
- Stating the host's domain exposed a defect of the refinement layer, repaired with it: a range
  obligation inside the word that the interval route could not lower was refused as if above the word.
