# 0093 — The LLVM layer is a theory with a parameter, Σ_OS, and windows instantiates it by gluing

Date: 2026-11-04
Status: Accepted (hamza, 2026-11-04: "go ahead. always mathematical and algebraic", on the layering of
the LLVM target and Σ_OS). Built with [layers-2026-11-04](../../gauntlet/results/layers-2026-11-04.md).
Builds target-system.md §5's family and §2.1's gluing.

## Context

`targets/windows/` held two theories in one directory:

- **The LLVM layer, which is the same on every triple:**
  - Σ_LLVM (`llvm.oro`);
  - ι's terms realizing the language's names (`lang.oro`);
  - the word and the representations;
  - the `realize` lines.
- **Windows itself:**
  - the triple, the build, the link line;
  - kernel32 and the rest of Win32.

Only three of ι's terms reached an operating system: the allocator's region, and the crash's write
and exit. Every other target that LLVM serves (32-bit Windows, ARM, Linux) would have copied the
first theory to change the second. target-system.md §5.2 calls Windows a fibration with Σ_win32 as
its base, and §5.1 asks for a fragment shared by targets, which "cannot today be shared".

## Decision

1. **The LLVM layer is a theory with a formal parameter, T_LLVM[Σ_OS]** (`targets/llvm/`), in the
   sense of parameterized specifications (Goguen and Burstall's institutions, OBJ's parameterized
   theories, ML's functors).
   - **Σ_OS is the parameter's signature**, three operation symbols with their contracts and no
     bodies:

     ```
     region(b ∈ 0..2^63-1) (ptr)            b fresh zeroed bytes, or the trap (exhaustion)
     write_err(b ∈ Array(0..255)) (Bool)    the bytes to standard error, and whether they were written
     exit(code ∈ 0..255) (∅)                the process ends with code; never returns (ADR 0090)
     ```

     The exit code's set is 0..255, the codes every host reports the same way.
   - **No LLVM instruction computes them**: getting memory and ending a process are calls into an
     operating system. So the layer only *requires* them.
   - **ι's terms are typed against the signature**: the allocator's guard and its proof of n·w + 8,
     and the crash's sequence, hold for every operating system that meets Σ_OS's contracts, and are
     proved once.
2. **A header with no body is an operation symbol, and gluing completes it.** In a target file,
   `fn region(b ∈ 0..2^63-1) (ptr)` with no body is a section of the sheaf of declarations over its
   name. It glues with a definition of the same name exactly when their headers are equal (§2.1's
   agreement on the overlap), and the glue is the definition. A header that no fragment defines is a
   requirement unmet, and the target is refused when it is loaded, naming the operation. A
   parameterized theory is not a model until the parameter is given.
3. **`glue NAME` in a target file glues target NAME's fragments into this one**, found beside it on
   the same layer: Δ = Δ_NAME ⊔ Δ_self. It is §2.1's ⊔ (agreement required, order-free), never §2.2's
   ▷, so a glued layer cannot silently replace what the target declares. A cycle is refused.
4. **Windows is the view Σ_OS → T_Win32 that makes T_LLVM a model**:

   ```
   windows = (llvm, T_LLVM[Σ_OS ↦ Win32's terms] ⊔ Σ_win32, triple x86_64-pc-windows-msvc)
   ```

   `targets/windows/` writes `glue llvm`, its `triple`, its build and link line, kernel32, and
   module `llvm/os`'s three definitions over `VirtualAlloc`/`HeapAlloc`, `WriteFile` and
   `ExitProcess`.
5. **The triple is target data**, `triple "x86_64-pc-windows-msvc"`, read by the printer. It is no
   longer a constant in `ir/llvm`. A target with backend `llvm` and no triple is refused: no default,
   because a wrong default is a silent miscompilation (target-system.md §1.1).
6. **A string's representation is the layer's** (llvm.md §4), so `llvm/str.utf8`, the identity view of
   a string's bytes, is the layer's too. `string_of` and `++` stay Windows' while their helpers
   allocate from the process heap; they move when they are terms over `region` (ADR 0091's
   consequences).

## Why not

- **Σ_OS's definitions in the LLVM layer.** No instruction computes them. Written there, they would
  be one operating system's calls in the layer every operating system shares.
- **The allocator and the crash wholly in each OS layer.** Their guard and its proof depend only on
  the layout, and would be repeated, and could drift, per OS. The seam is where the operating system
  begins, the three primitive steps.
- **`extends llvm`, or an override layer below windows.** Override (▷) licenses disagreement, so
  windows could silently replace a term of ι. The layer is shared because it must mean the same in
  every member of the family, and that is gluing's agreement.
- **A new `interface … implements …` form** (target-system.md §5.4's proposal). A header with no body
  already says f ∈ (S → R) (ADR 0062), and gluing already requires agreement. The interface is the
  headers, so no form is needed.
- **One triple constant in the printer.** It made the printer Windows' and the layer's other members
  impossible.

## Consequences

- `targets/llvm/`:
  - `llvm.oro`, the layer's model data and realizations;
  - `instructions.oro`, Σ_LLVM;
  - `lang.oro`, ι's terms;
  - `os.oro`, Σ_OS's signature;
  - `str.oro`, the string's bytes.

  Building `-target=llvm` alone is refused, naming the three operations its parameter lacks.
- `targets/windows/` is the instance: `windows.oro`, `os.oro` (the view), `kernel32.oro`,
  `strings.oro`.
- A second member is a triple and an OS layer. The next is i686-windows, which differs from windows
  only in its triple; the C ABI's derivation of a Win32 call is the following step.
