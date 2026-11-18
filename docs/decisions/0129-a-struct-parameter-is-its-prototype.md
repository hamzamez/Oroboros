# 0129 — A struct parameter is its prototype: the bare set is the value, `in` marks const T*

Date: 2026-11-18
Status: Accepted (hamza, 2026-11-18, on the choice between this and keeping the bare set as
`const T*`: "I agree to recommended"). Amends [ADR 0128](0128-a-record-is-built-whole-and-a-host-struct-is-its-layout.md)
item 13 and llvm.md §6b, where step 2b read a parameter whose set is a C struct as `_In_`'s
`const T*`. Built ([sbv-2026-11-18](../../gauntlet/results/sbv-2026-11-18.md)).

## Context

A Win32 struct parameter is passed two ways: by its address, `_In_ CONST RECT *lprc`, or by value,
`_In_ POINT pt`, in one prototype (`PtInRect`). Both are a value passed in ADR 0084's sense (P2):
the host reads the record and keeps nothing. They differ only in the ABI's representation, so the
declaration must say which.

Step 2b had given the bare struct set the first meaning, `fn RegisterClassA(wc ∈ WNDCLASSA)`.
struct by value was owed, with 140 Win32 entry points (structval-2026-09-12): 84 in one register and
56 by reference, by MSVC's answer.

Measured against clang 23.1.4 for nine shapes (1, 2, 3, 4, 8, 12 and 16 bytes, a padded one and a
`double`):
- **x86_64 (Win64)**: an aggregate of 1, 2, 4 or 8 bytes is the integer of its bytes, padding and
  `double` included; any other size is the address of a copy the callee may overwrite.
- **i686**: the bytes on the stack. clang writes `i32` fields where they fill the struct exactly, a
  `double` as itself, and `byval` otherwise, all the same stack bytes.

## Decision

1. **A struct parameter's bare set is the struct by value**, as its prototype writes it. ADR 0094's
   rule, "a C function is declared by its prototype, its sets its C types", is read literally: the C
   type `POINT` is the value.
2. **`in` after the set marks `_In_`'s `const T*`**, a mode as `linear` and `inout` are,
   `wc ∈ WNDCLASSA in`. `cell(T)` remains `_Out_` and `_Inout_`. `in` is a contextual word of a host
   declaration's parameter, refused on a set with no layout.
3. **Both are the same fresh copy** of the record's encoding (`(#ref "T" leaves…)`, Theorem P). The
   call passes the copy's address for `in`, and for the value what the triple's ABI passes:
   - on x86_64, `iN` loaded from the copy for N ∈ {1, 2, 4, 8} bytes, and the copy's address for any
     other N;
   - on i686, `ptr byval([N x i8]) align 4`, whose size LLVM counts in stdcall's `@N`.

## Why not

- **Keep the bare set as `const T*` and mark the value** (`pt ∈ POINT value`). Nothing existing
  would change, but the declaration would differ from its prototype by the pointer's star, the one
  thing a reader checks first, and ADR 0094's rule would hold only up to a convention.
- **One mode word for both, read off the struct's size.** The ABI's choice depends on the size, but
  the prototype's does not: an 8-byte struct is passed by address under `const T*` and as an `i64`
  by value. The two are different C types, and a declaration states its C types.
- **Clang's per-field expansion on i686**, `i32, i32` for `POINT`. It is the same stack bytes as
  `byval` and needs a rule per shape, padded, mixed or `double`. `byval` of the copy's N bytes is one
  rule for every shape.
- **Teach the Win32 survey struct by value now.** The generator refuses it (`rStructVal`), so a hand
  declaration taking one is exempt from the prototype check with its reason, and checked instead by
  running it (`TestAStructIsPassedByValue`).

## Consequences

- `RegisterClassA`, the one `_In_` struct declaration, is respelled `wc ∈ WNDCLASSA in`.
  `PtInRect(rc ∈ windef.RECT in, pt ∈ windef.POINT)` is declared, the first struct by value, run on
  both triples.
- `Prim.ByValue` lists the by-value parameters among `Prim.InStructs`, and the LLVM printer's C call
  passes each by the triple's ABI (`ir/llvm`'s `cCall`).
- **Not built**: a struct returned by value (one entry point needs the hidden-pointer return,
  structval-2026-09-12), and struct by value on any host but the LLVM layer's, where no declaration
  has one.
