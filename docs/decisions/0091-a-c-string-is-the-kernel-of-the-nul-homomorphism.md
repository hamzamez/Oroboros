# 0091 — A C string is the kernel of the NUL homomorphism, decided by structure

Date: 2026-11-04
Status: Accepted (hamza, 2026-11-04: "go ahead with your recommendation. always mathematical and
algebraic", on mastretire-2026-11-03's wall). Built with
[cstring-2026-11-04](../../gauntlet/results/cstring-2026-11-04.md).

## Context

A string is the free monoid Σ* on Unicode scalars (strings.md). Win32's `LPCSTR`, and every C API's
`const char *`, is a pointer to bytes ended by the first NUL, so a string that holds U+0000 reaches C
as its prefix. On windows a string's bytes are followed by a NUL outside its length (llvm.md §4), so
passing a string is the identity s ↦ s + 8. That is the C string exactly when s holds no NUL, and
the program's manifest makes the bytes UTF-8 (llvm.md §2). Nothing stated the first condition:
`CreateFileA("a\u{0}b", …)` opened `a`, a wrong answer and no refusal.

## Decision

1. **ν : Σ* → (𝔹, ∨, false)** is the monoid homomorphism true exactly on the generator U+0000:
   ν(ε) = false, ν(a ++ b) = ν(a) ∨ ν(b), ν(η(c)) = [c = 0]. **The C strings are its kernel**,
   ν⁻¹(false), the free submonoid on Σ ∖ {0}. It contains every literal without `\u{0}` and is closed
   under `++`.
2. **`nulfree : String → Bool` is in Σ_L**, the computation of ¬ν. It is total and pure, and every
   target realizes it: Go `strings.IndexByte(s, 0) < 0`, JavaScript `!s.includes("\0")`, the JVM
   `s.indexOf(0) < 0`, windows a term over the string's bytes.
3. **A C-string parameter is `a ∈ String where nulfree(a)`**, the separation
   {s ∈ String : nulfree(s)} written as its base set and its predicate. The Win32 survey declares
   every `LPCSTR` parameter so. The obligation is a `where` mark, decided by the one procedure that
   decides every mark (ADR 0037).
4. **The decision evaluates ν on the argument's term**, exactly, because ν is a homomorphism: the
   abstraction α(S) = ν(S) is complete for `++`, whose abstract operation is ∨ (Cousot and Cousot).
   The cases:
   - a literal is decided by its scalars;
   - `string_of(c)` by c's facts proving c ≠ 0;
   - `a ++ b` by both;
   - a conditional by both arms.

   Anything else is the opaque atom `nulfree(s)`, proven only by an identical assumption.
5. **The checked conversion is a guard.** `if nulfree(s) { f(s) } else { … }` assumes the atom on its
   arm (Theorem E), so a string from input reaches C through a branch that scans it once. No other
   conversion function exists, because the map into C strings is the identity on its domain.
   `-checked` does not apply: an unproven `where` is refused under it, as an unproven divisor is.

## Why not

- **A conversion function `cstr : String → C`.** Into C strings the map is the identity on the
  kernel and undefined off it, so a function would be the identity plus a scan. The guard is that
  scan, with its failure a branch the program writes rather than a value it must unwrap. A function
  that truncates or escapes instead loses information: the map would not be injective.
- **A sort `CString` in the checker.** It would need subtyping on strings throughout the checker,
  for one predicate a `where` already states. ADR 0049's types are sets, and a separation is a base
  set and a formula; this is that formula's first non-linear atom.
- **A run-time scan under `-checked`.** `-checked` exists because arithmetic outside the word has a
  host meaning that is wrong; a C call on a string with a NUL has no such default to trap at. The
  program decides with the guard.
- **Refusing every non-literal string.** ν's homomorphism proves `++` and `string_of`, the only ways
  this language builds a string, so refusing them refuses what is decidable.
- **Escaping a NUL.** No C API reads an escape. The APIs that take NULs inside (`lpstrFilter`,
  `REG_MULTI_SZ`, an environment block) take a list of C strings, (B*₀)*, which is its own codec.

## Consequences

- Every Win32 `LPCSTR` parameter carries the obligation; the acceptance programs pass literals,
  `string_of` and `++`, all proven.
- `nulfree` is a word of Σ_L, in the inventory, with a realization on four targets.
- Still to build, each when a program needs it:
  - a C string returned or filled by Win32, decoded by `strlen` and a copy, total, into the kernel;
  - the list of C strings (a filter, `REG_MULTI_SZ`), (B*₀)* → bytes, s₁ 0 … sₙ 0 0;
  - the `W` functions' UTF-16;
  - `string_of` and `++` on windows as terms over the bytes, which need the valid-UTF-8 refinement
    (V, the image of enc : Σ* → B*, a submonoid by the same argument).
