# 0126 — Win32 is by header, and a C type name is its type's whole value set, which the data model interprets

Date: 2026-11-17
Status: Accepted (hamza, 2026-11-17: "Go with item 3, always mathematical and algebraic", on
assessment-2026-11-17 §5 item 3, which is ADR 0122 decision 2's W1). Amends ADR 0094 decision 1:
a 64-bit unsigned C type's set.
Built ([byheader-2026-11-17](../../gauntlet/results/byheader-2026-11-17.md)).

## Context

ADR 0122 decision 2 states the principle: a Win32 prototype written once, as its header writes it,
is a term with free set names, and a data model interprets them. Its building was left to W1,
Win32 by header. Before W1:
- the hand declarations were by DLL, in `windows/kernel32`;
- the survey emitted modules by header (`win/fileapi`), in LLP64's sets written out.

ADR 0094 gave a 64-bit unsigned C type two sets:
- at an argument, the word's naturals, 0..2⁶³−1;
- at a result, the signed word, the bits read signed.

A name is one set, so `SIZE_T` written once is wrong at one of the two positions. HeapSize answers
(SIZE_T)−1, 2⁶⁴ − 1, on failure:
- `r ∈ SIZE_T` with SIZE_T = 0..2⁶³−1 would assume a false fact;
- the word, written by hand, was the wrong ABI on i686, where SIZE_T is 32 bits (an `i64` declared
  for an `i32` return).

TinyRetroPad 6c measured the surface W1 must cover: 47 reachable functions in 14 headers
(tinyretropad-2026-11-17).

## Decision

1. **Win32 is by header** (W1, printing-research.md §4 and M1). The SDK's header H.h is the module
   `windows/H`:
   - H is case-folded: Windows' names are equal up to case, and the documentation writes them in
     lower case;
   - each declaration's `lib` is the DLL that resolves it, by the survey's linkage;
   - `windows/kernel32` is retired, its 20 declarations moved to their 9 headers;
   - the survey's generated module for the same header is `win/H`, and the two are compared name by
     name.

2. **A C type name is a set, and its header's typedef is its definition.** `typedef ULONG_PTR
   SIZE_T` is `set SIZE_T = ULONG_PTR` in `windows/basetsd`, resolved through a module's `use` as a
   term name is (ADR 0073).

   The typedefs form a **definitional extension** of C's base types: each name is defined, without
   cycles, by a term over base types and earlier names. A data model M interprets the base types
   (char, short, int, long, long long, pointers, and `__int3264`). By the uniqueness of an
   interpretation over a definitional extension (each defined name means the composite of its
   definition), M determines the meaning ⟦T⟧_M of every name T.

   So two models differ exactly on the names whose definition reaches a pointer-width base type.
   **In basetsd.h's own text that is its `#if defined(_WIN64)` branch: four names**, `INT_PTR`,
   `UINT_PTR`, `LONG_PTR` and `ULONG_PTR`.
   - **The member's fibre is those four** (`targets/windows/model.oro`,
     `targets/windows32/model.oro`).
   - **Every other name is the base's**, written once in its header's module: SIZE_T, SSIZE_T and
     DWORD_PTR in `basetsd`; WPARAM, LPARAM and LRESULT in `minwindef`, through the four.

3. **A C integer type's set is its type's whole value set, at an argument and at a result**
   (amending ADR 0094). A 64-bit unsigned type is 0..2⁶⁴−1, the unsigned word (ADR 0083), on a
   target that realizes it. The proper subsets are each wrong somewhere:
   - **at a result**, a subset is a claim no C function makes;
   - **at an argument**, a subset is a domain restriction, which is a `where`;
   - **0..2⁶³−1 is no C type**, and is refused (`PlantNaturalsCType` restores ADR 0094's reading,
     and `TestACFunctionIsItsPrototype` catches it).

   On LLP64 the byte count a program passes is a word value proven in U. It crosses by `u64-of`, the
   identity on S ∩ U.

4. **The unsigned word is selected on ι's image too.** The target's realizations (ADR 0089) are
   substituted after the decision. `region`'s term calls HeapAlloc and VirtualAlloc, whose SIZE_T is
   now U, so the selection runs again on the substituted function (`SelectWordsOnImage`):
   - every value keeps the realization its type already is (ADR 0032);
   - only a flow into a declared position of the other realization is converted.

   `PlantNoWordsOnImage` skips it, and W5 refuses the allocator's call
   (`TestTheWordsAreChosenOnTheRealizationsImage`).

5. **A handle is `ptr`.** DECLARE_HANDLE(H) is `struct H__ *`, and HANDLE is PVOID. The survey's
   generated sets now write a handle as the C type it is:
   - the verdicts still count a handle as passable, since a program holds the one a call answered;
   - on i686 the word, eight bytes, was the wrong ABI for a four-byte handle.

6. **Each hand declaration is checked against its prototype** (`TestWin32DeclarationsAreTheirPrototypes`).
   Both are loaded into one target, the generated headers a layer over `targets/`. A hand
   declaration agrees with the generated one when:
   - its callee, convention, library and kind are the generated one's;
   - it passes as many C arguments (ADR 0118's terms or its parameters);
   - each argument has the same LLVM type, and for an integer the same set;
   - a pointer is `ptr`, a cell, a byte table's elements or null, and a C string appears only where
     the SDK has one;
   - its result is the prototype's or, for an integer, a subset of it;
   - ∅ stands only where the SDK's result is a DECLSPEC_NORETURN void.

   A name the generator cannot declare is exempt, with its reason, and a stale exemption is refused.
   Twelve planted mistakes, and one name the SDK lacks, are each caught.

## Why not

- **By DLL** (the old `windows/kernel32`). kernel32 spans 45 headers. The DLL is a fact about
  linking, which each declaration carries as `lib`. The documentation and C's `#include` are keyed
  by header (M1: 303 of 327 headers bind one DLL).
- **By metadata namespace** (W2, `windows/System/Console`). A second source of truth outside the
  SDK, and a projection's opinion rather than the C API's own path. A module may re-export several
  headers if a program wants W2's grouping.
- **SIZE_T = 0..2⁶³−1, kept as the name's set.** Unsound at a result (HeapSize's −1), and a
  restriction at an argument that the header does not state.
- **SIZE_T as the word.** It passes a negative integer as its residue, 2⁶⁴ + n, so the callee would
  see a different integer from the one the program computed.
- **Two names per type**, one for arguments and one for results. C has one name, and the set a
  program states would depend on where it appears.
- **Selecting the words again from the facts.** Measured: the factorial loop, whose numerals' digits
  are the unsigned word, was refused. A conversion the first selection inserted was read as a
  declared position, and the facts were recomputed less tightly. The type already is the
  representation.
- **Moving the selection after the realizations, once.** It would move the pair selection with it
  (ADR 0083), and the realizations choose their instances from facts the selection's conversions
  must already have.
- **A handle as the word, as the survey had it.** It is the parasite model's intuition, a handle is
  a word, applied to an ABI where a pointer and an integer are different types. On i686 the widths
  differ.
- **The survey writing type names** (every header's typedefs as modules, its output model-free).
  Not rejected; owed. The check resolves the hand file's names in the target, against the generated
  file's LLP64 sets, which is exact on LLP64 and checks i686 only through the C ABI's derivation.

## Consequences

- A Win32 declaration is written once, by its header's names, and is right on both members: its
  `declare` follows the data model, `i32 @HeapSize` on i686 and `i64` on x86-64.
- `examples` and `lib` call Win32 by header: `fileapi.WriteFile(h, b, lo, n, out minwindef.DWORD)`.
- A 64-bit unsigned value crosses into C exactly, and a 64-bit unsigned result is the unsigned word.
  HeapSize's −1 is `r == 2^64 - 1`, compared by `icmp eq`, and its order by `icmp uge`.
- windows' emission changes by the conversions alone, `or i64 %x, 0` at each `u64-of`, which clang
  -O2 folds: all 35 changed files compile to the same machine code as before.
