# 0094 — A C function is declared by its prototype, and the C ABI derives its call

Date: 2026-11-05
Status: Accepted (hamza, 2026-11-05: "go ahead, with step 2. always mathematical and algebraic", 6b′
step 2). Built with [cabi-2026-11-05](../../gauntlet/results/cabi-2026-11-05.md). Decision 1's
64-bit unsigned sets (0..2⁶³−1 at an argument, the word at a result) amended by
[ADR 0126](0126-win32-is-by-header-and-a-c-type-name-is-its-whole-value-set.md): a C integer's set
is its type's whole value set, 0..2⁶⁴−1 at both.

## Context

Win32 is a C API. The survey wrote each of its 10,479 functions as an LLVM template: a `trunc` per
narrow parameter, a `getelementptr` per C string, the `call`, a `sext` or `zext` of a narrow result,
and the `declare` line. Every template was a function of two things, the function's prototype and
the triple. A second triple (6b′ step 4, i686, whose Win32 functions are `__stdcall`) would have
meant rewriting all of them. And none was checked against anything: a template is text.

## Decision

1. **A C type is a set, and over the C integer types that map is injective on full ranges.** The
   range of a w-bit signed integer is −2^(w−1)..2^(w−1)−1 and of a w-bit unsigned one 0..2^w−1, for
   w ∈ {8, 16, 32}. The word −2⁶³..2⁶³−1 is a 64-bit integer, and 0..2⁶³−1 a 64-bit unsigned one on a
   target without the unsigned word (`SIZE_T`); `ptr` is a pointer and a string a C string.
   - **A C integer's set is its type's full range.** A subset does not determine the width (0..9 is
     a `char` or an `int`), and the width is the ABI: 1 byte against 4 on i686's stack, and register
     bits nothing set on x86-64. A narrower domain is a `where`.
   - So a declaration's sets *are* its prototype, and no C type name is written. This is ADR 0083's
     rule, a representation chosen by place, the place a C call.
   - Win32's own constants are integers of the type: `STD_OUTPUT_HANDLE` is `((DWORD)-11)`, which is
     4294967285, and a program passes that.
2. **`host expr call "SYMBOL" [stdcall | cdecl]`** declares a C function by its symbol. Its call is
   derived by the C ABI, a map A_t from C types to LLVM types on the triple t, with two maps per
   type:
   - **into the call**, `in_τ : S(τ) → ⟦A_t τ⟧`: `trunc` from the word for a narrow integer, which
     is exact on its set (trunc ∘ embed = id); the identity for the word and a pointer; s ↦ s + 8 for
     a C string (llvm.md §4);
   - **out of the call**, `out_τ : ⟦A_t τ⟧ → S(τ)`: `sext` or `zext` into the word for a narrow
     integer, by the set's sign; the identity otherwise.

   The call is out_τ ∘ F ∘ (in_τ₁ × … × in_τₙ), and its `declare` is A_t of the prototype. A `void`
   function is `host stmt call`, its value its first argument as every statement's is; one into ∅
   never returns (ADR 0090).
3. **The triple chooses what the prototype does not say**:
   - `dllimport` on a Windows triple (COFF's import of a DLL's function), nothing elsewhere;
   - the convention: `stdcall` (Win32's `WINAPI`) is `x86_stdcallcc` on i686 and the C convention on
     x86-64, which has one; `cdecl` (`WINAPIV`) is the C convention everywhere.
4. **A prototype is checked when the target loads**:
   - a set that is no C type is refused, naming it;
   - a C-string parameter must carry `where nulfree(p)`, since s ↦ s + 8 is the C string only on
     ν's kernel (ADR 0091). **A soundness rule, with its planted fault.**
5. **The survey declares every Win32 function so**, its convention read from the header's macro.
   kernel32's hand declarations are prototypes too, except `WriteFile`, which takes a table's segment
   and is a term over the bytes, and stays a template.

## Why not

- **C type names (`c.int`, `c.dword`) as the declaration's types.** The sets already determine them,
  and a name beside a set is a second claim that can disagree with it. Names may come later as
  abbreviations of the sets, per data model (`long` is 32 bits on LLP64 and 64 on LP64).
- **Keep the templates, generated per triple.** A generator per triple is the functor applied by
  hand, 10,479 times a triple, and nothing checks a template against the declaration it sits in.
- **Derive in the survey, not the backend.** The survey is one producer of declarations; a hand
  declaration is another, and the derivation must be the same for both. A C call's lowering is
  backend knowledge, as a template's binding is.
- **Pass every integer as i64**, which is how MASM called them. Correct on x86-64's registers, it is
  wrong on i686's stack, where an int is 4 bytes, and it hides the C type from LLVM's own lowering.
- **A C string by a copy at each call.** The representation already ends a string with a NUL
  (llvm.md §4); a copy would add an allocation per call for nothing.

## Consequences

- The Win32 layer the survey writes has no templates. A second triple re-derives every call.
- `targets/windows/kernel32.oro` keeps one template, `WriteFile` (its buffer is a segment), and the
  target's own `@__written` cell. (Since ADR 0118, a call's arguments are a linear term over its parameters,
  elem(b, lo) a segment's address, and kernel32 has no template and no global.)
- The survey's figures are a property of the API and the language, so they are unchanged.
