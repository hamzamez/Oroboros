# 0122 — The windows family is a fibration over Win32, and a triple is a model of what it leaves open

Date: 2026-11-17
Status: Accepted (hamza, 2026-11-17: "Go with your recommendations on D1–D7, always mathematical and
algebraic", on [triple-research.md](../triple-research.md) §6). Built in steps, each with its result
document; this ADR states D1, D3, D4, D6 and D7 and the two derived consequences. D2 (the digit base,
amending ADR 0106) and D5 (i32 scalars, ADR 0083 item 2) have ADRs of their own at their steps;
where D6's realization sits, the symbols LLVM calls, is ADR 0123's.

## Context

The windows target is T_LLVM[Σ_OS] instantiated by Win32 and glued to one triple, x86_64 (ADR 0093).
The second triple, i686-pc-windows-msvc, is TinyRetroPad's (6c, 6d). triple-research.md ran the
suite on it through a scratch copy: 63 of 74 cases pass, and the rest stop at the address space, at
x86-64's own operations, and at the 64-bit division helpers. It also found that what a triple
decides is a set of functions of t:
- the data model's map from C types;
- the convention;
- the native double word;
- the address space;
- the ISA's own operations;
- the toolchain.

target-system.md §5.2a already names the shape: a family with a shared base Σ_win32 and a fibre per
triple.

## Decision

1. **The family is a fibration, built as gluing** (D7). `targets/win32/` is the base. It holds Win32's
   declarations (kernel32) and Σ_OS's view over them (`llvm/os`), and glues `llvm`. It is a theory
   with parameters: the data model's set names, the address space, the ISA's operations. Each member
   glues it and gives its fibre:
   - `targets/windows/`: x86_64-pc-windows-msvc, LLP64, `x64.oro`, A = 2⁴⁷, and `vcvars64`;
   - `targets/windows32/`, the target `windows32`: i686-pc-windows-msvc, ILP32, `x86.oro`, A = 2³¹, and `vcvars32` with
     clang's `--target`.

   The members do not override each other. Gluing needs agreement and ignores order (§2.1), and two
   members that agree on the base and each define the fibre are exactly its two sections.

   A library's `provides win32 M` is a fragment of the base, so it serves every member that glues
   the base. A fragment of a glued theory is a fragment of the gluing (§2.1's ⊔), and the loader
   reads `provides` for the target's own name and for each name it glues.
2. **C's type names are sets the data model defines** (D1). A declaration's sets are its prototype
   (ADR 0094 decision 1), so a Win32 prototype written once, as its header writes it (`SIZE_T`), is
   a term with free set names, and a data model is an interpretation of them:
   - LLP64: `SIZE_T ↦ 0..2⁶³−1`, the word's naturals where the unsigned word is not the declaration's;
   - ILP32: `SIZE_T ↦ 0..2³²−1`.

   The names the two models agree on (`DWORD`, `BOOL`) are the base's. The names live in their
   headers' modules, which is W1: Win32 by header, built by
   [ADR 0126](0126-win32-is-by-header-and-a-c-type-name-is-its-whole-value-set.md), where the
   models differ on exactly basetsd.h's four pointer-width names.
3. **The address space bounds bytes** (D3). Σ_OS gains `max_bytes`, A_t, and `region`, `region_raw` and
   `zero` require b ≤ max_bytes(), a precondition of the operation. `allocate` proves n·w + 9 ≤ A_t
   from its guard, so on i686 the byte count passed to a 32-bit `SIZE_T` is a theorem, not a trap.
   `max_len`, the longest table, is its consequence for one-byte elements: A_t − 9.
4. **The ISA's operations split by ISA** (D4). `llvm/isa` is defined once per member: x64.oro's `divq`
   and 64-bit `adc`/`sbb`, and x86.oro's at 32 bits. A member defines exactly the symbols its own
   digit base calls (D2).
5. **`/` and `%` of the word on i686 are a realization in the language** (D6), recommendation (b):
   Knuth's D on two 32-bit digits over the triple's 2-by-1 division `divl`, as K(ℕ\*) divides, where
   LLVM would call MSVC's `__aulldiv`. The target links no C runtime (llvm.md §2). libcmt's helpers
   are the measured fallback if (b) is slower.
6. **Derived**:
   - the build line is the member's: its toolchain environment and `--target=t`, since clang's default
     target overrides a module's own;
   - `llvm.address`'s integer is the pointer's width. A sentinel such as `(HANDLE)−1` is compared at
     that width, since on ILP32 `ptrtoint` to i64 zero-extends −1 to 2³² − 1.

## Why not

- **windows32 overriding windows.** Overriding licenses disagreement (§2.2): windows32 would replace
  x86_64's triple, data model and ISA by order, and anything windows later added would silently
  reach windows32. A fibration has no order between its sections.
- **A `-triple` flag on one target.** The build would be a function of a flag the target does not
  state. A triple is target data (ADR 0093 item 5).
- **A copy of each Win32 declaration per data model.** This is the generator per triple that ADR 0094
  rejected: 10,479 prototypes written twice, with nothing checking that the copies agree where the
  models do.
- **max_len as the address space.** It bounds elements, and a region's request is n·w + 9 bytes. On
  x86_64 the gap never mattered; on i686 a word table's bytes pass the 32-bit `SIZE_T`.
- **libcmt's `__aulldiv`**, or compiler-rt's builtins (not installed with the release): a C runtime
  the target otherwise does not link, for an algorithm the language already has and proves.

## Consequences

- A third member, ARM64 Windows or a 32-bit build with `/LARGEADDRESSAWARE`, is a directory: a triple,
  a data model, an ISA module, A_t and a toolchain.
- `numberedHoles` and `WriteProgram`'s layout, which test the name `windows`, ask about the LLVM
  backend instead.
- The windows target's emission is unchanged by the restructuring alone. The files move, and the
  module paths are each file's own `module` line.
