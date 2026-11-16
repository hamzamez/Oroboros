# 0118 — A C call's arguments are a linear term over its parameters

Date: 2026-11-16
Status: Accepted (hamza, 2026-11-16: "Go ahead, always mathematical and algebraic", on the proposal to
derive kernel32's segment calls and to split `store8`). Amends [ADR 0094](0094-a-c-function-is-declared-by-its-prototype-and-the-c-abi-is-derived.md)
item 2. Built with [cabi2-2026-11-16](../../gauntlet/results/cabi2-2026-11-16.md).

## Context

ADR 0094 derived a C function's call from its prototype: out ∘ F ∘ (in₁ × … × inₙ), one conversion
per parameter, in order. kernel32's three file functions did not fit, so they stayed LLVM text,
four or five instructions each:
- `ReadFile(h, p, n, &read, NULL)` and `WriteFile(h, p, n, &written, NULL)` take the *address of an
  element*, &b[lo]. A segment is two parameters, b and lo, so it is not the image of one parameter
  under one conversion;
- each passes `NULL`, which corresponds to no parameter;
- the Boolean `WriteFile` passed a target global, `@__written`, which is not reentrant and whose
  count nothing could read. A second declaration of the same function, `WriteFileC`, took a cell.

The audit after ADR 0117 found one more piece of text, `llvm.store8`. It was three instructions
(`getelementptr`, `trunc`, `store`) and a fourth, `icmp eq i64 0, 0`, which existed only to give it a
result. ADR 0089 allows a template only for one host instruction.

## Decision

1. **`in` is a term over the parameters.** `host expr call "F"(t₁, …, t_k) [stdcall | cdecl]`
   states C's arguments, each tⱼ a term in the signature:
   - **x**, a parameter, converted by its C type as in ADR 0094;
   - **elem(b, lo)**, the address &b[lo] of a table of bytes, the table decayed to its first element
     (s + 8, llvm.md §4) and offset by lo. This is the action of (ℕ, +) on the region's addresses
     (ADR 0084: an address is an offset in a region);
   - **null**, C's null pointer, the constant 1 → ptr.

   The call is out ∘ F ∘ ⟨t₁, …, t_k⟩. With no argument list it is ADR 0094's, ⟨x₁, …, xₙ⟩.
2. **The term is linear: every parameter appears exactly once.** That makes ⟨t⟩ a permutation of the
   parameters, some pairs of them composed with elem, and constants beside them. It is checked when
   the target loads, and it has a planted fault (`PlantCArgDropped`). Linearity is what the
   declaration's claims need:
   - **none dropped.** A dropped parameter is a claim, its set and its `where`, about an input the
     host never sees. A lent buffer that is never passed would still be read as written (ADR 0084);
   - **none duplicated.** A table passed twice is two addresses into one view, aliasing that ADR
     0084's separation forbids for a linear one. Linear terms are the ones that carry each view to
     exactly one argument, as in linear logic's cut.
3. **elem is partial, and its domain is the declaration's.** &b[lo] is defined for 0 ≤ lo ≤ len b
   (C11 §6.5.6¶8: one past the end included; LLVM's `inbounds` the same).
   - The offset's set must be a natural inside the word.
   - The `where` must state lo ≤ len(b), either directly or as lo + m ≤ len(b) for a parameter m
     whose set is a natural's (`emit.boundsElem`, planted fault `PlantElemUnbounded`). Each call
     then decides it (ADR 0028).
   - The extent the host reads or writes, [lo, lo + n), is that same `where` read as the host's
     domain. The printed `getelementptr` is `inbounds` because the term proved it.
4. **One Win32 function, one declaration.** `WriteFile` takes its count's cell, as `ReadFile` does
   (ADR 0048, 0112), and returns its BOOL. `WriteFileC` and the global `@__written` are deleted. A
   caller that wants only success writes `(w, r) = kernel32.WriteFile(h, b, lo, n, out
   kernel32.DWORD); r != 0`.
5. **`store8` is a composite, so it is a term.** It is split into two single-instruction host
   operations:
   - `llvm.offset(p, o)` is `getelementptr inbounds i8`, pure;
   - `llvm.store_zero(p)` is `store i8 0`, a statement whose value is p.

   The allocator writes its NUL as `store_zero(offset(p, n·w + 8))`.

After this, no template in the LLVM or windows layers is longer than one instruction. The one left
that is no LLVM instruction is `divq`, the triple's own 2-by-1 division (ADR 0104), one line of
inline assembly.

## Why not

- **A `ptr` parameter, with the caller computing &b[lo].** The language has no address of a
  table's element, and giving it one would separate the address from the view that licenses it
  (ADR 0084: no load without a view). elem keeps the address inside the declaration that holds the
  view.
- **A segment as a C type.** A C type is the set of one parameter (ADR 0094 item 1), but a segment
  is a function of two. Making it a type would need a product sort the prototype does not have.
- **Affine terms**, where a parameter may be dropped, for a ghost parameter that is used only in
  the `where`. No declaration needs one, and allowing it would weaken item 2's two claims. It can
  come when a program needs one.
- **Keeping both `WriteFile`s.** Two declarations of one symbol are two claims about one function,
  which can disagree. The Boolean one hid its count in a global, and was not reentrant.
- **A general `store8(p, o, v)` for any byte v.** The only use is v = 0. A computed byte needs
  `trunc` to i8 before the store, a second instruction and an i8 sort the language does not have.
  It is owed until a program stores a computed byte through a pointer.
- **Leaving `store8` as text because it is small.** Size is not the criterion. A template is text
  the compiler cannot check, and its fake `icmp` result was a value with no meaning.

## Consequences

- kernel32's file functions are `host expr call "ReadFile"(h, elem(b, lo), n, got, null) stdcall`,
  re-derived per triple: on i686 they become `x86_stdcallcc` with no edit (6b′ step 4).
- The interpreter that checks realizations models a cell (ADR 0048) as a one-slot reference, and
  states WriteFile's meaning: n written, a nonzero BOOL.
