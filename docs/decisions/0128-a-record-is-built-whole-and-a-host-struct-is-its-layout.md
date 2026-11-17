# 0128 — A record is built whole, updated by override with paths; a host struct is its layout, derived and checked

Date: 2026-11-17
Status: Accepted (hamza, 2026-11-17, on [records-research.md](../records-research.md) §8: "the whole
record, and yes to r with {}"; items 3–6 "agree"; the value `record { left : 0, top : 1 }` hamza's).
Builds ADR 0021's `record` and ADR 0084 item 14's "a record's memory layout is a codec". Amends
data.md §3.3 (three positions lifted) and §4 (J's spelling, paths). Step 1 built ([records-2026-11-17](../../gauntlet/results/records-2026-11-17.md)) but for a record as a map's value; step 2 not built.

## Context

TinyRetroPad's first wall, 6d, is its structures (tinyretropad-2026-11-17). It uses 17 struct types,
reached through 18 entry points, through `SendMessage`, and nested inside one another.

The repository had decided most of the question before (records-research §0):
- **ADR 0021 and data.md §4**: the record as a function on its labels, structural, in canonical
  order, updated with `with`.
- **ADR 0083**: a field's representation at a host is the host's.
- **ADR 0084**:
  - Theorem P and the modes;
  - ownership a tree;
  - "a record's memory layout is a codec".

What none of them settled: how a layout is computed; a field the program must not choose; the C
idiom of zero-then-set; pointer fields; J's spelling; and how far the record goes.

Measured on both triples with clang 23.1.4 and SDK 10.0.26100.0 (records-research §2):
- **7 of the 34 layouts are not natural.** commdlg.h packs to 1 byte on i686, so `PRINTDLGA` is 66
  bytes there, not 68. richedit.h packs to 4.
- **The census**: 121 of 125 structs are natural on x86_64, and 118 of 125 on i686.
- **Clang prints a field as a byte offset**, loaded at alignment min(A, o & −o). No struct type.

## Decision

**The record, whole** (records-research §3.1, §8.1):

1. **The set** is `record { l₁ ∈ S₁, …, lₙ ∈ Sₙ }`: the dependent product Π_{l∈L} S_l over a static,
   finite label set L. It is structural: two record sets are equal when their labels and their
   components' sets are (data.md §4.4).
2. **A value** is `record { l₁ : e₁, …, lₙ : eₙ }`, its graph, total on L.
   - The pair separator is ADR 0061's `:`. The token after a label tells a set (`∈`) from a value
     (`:`), and a former mixing the two is refused.
   - Labels are bare identifiers.
3. **Projection** is `r.l`, TLA+'s sugar for r["l"]. In one namespace (typevalues.md §2) the binder
   decides: a value is projected, a module is qualified. A path `r.l.m` projects through nested
   records.
4. **Update is override**, `r with { p₁ : e₁, …, pₖ : eₖ }`, each pᵢ a path of labels.
   - Its meaning is (f ⊕ g)(x) = g(x) on dom g, else f(x), composed along each path by lens
     composition: `r with { rc.right : 5 }` is `r with { rc : r.rc with { right : 5 } }`.
   - The paths must be **prefix-free** (none is another's prefix, none repeats), so the updates touch
     disjoint parts and commute. A path not in the record's type is refused, naming it.
   - The result has r's set, and each eᵢ owes its component's set.
   - The lens laws hold: put–get, get–put, put–put (data.md §4.3).
5. **A cell's component is written by the same path** in program order, `p.rc.right := v`. This is
   G6's place (hamza, 2026-10-15) extended to labels, and the state translation gives the `with`.
6. **`zero(T)`** is the record whose every component is its set's zero: 0 where 0 ∈ S_l, `none` for
   an `Option`, `zero` of a nested record. It is defined iff every component has one, and otherwise
   refused naming the field. `zero(T) with { … }` is C's `{0}` followed by writes.
7. **Representation**: a record is the tuple of its components in ascending label order, the
   bijection L ≅ {0…n−1}. **It may go wherever a tuple goes, and three positions are lifted for both**
   (data.md §3.3), each with a conformance case as its program:
   - a product nested in a product at a representation position, flattened by associativity,
     A × (B × C) ≅ A × B × C;
   - a product as an export's parameter, represented per field by ρ (ADR 0079): Go a struct, Java a
     record class, JavaScript an object, LLVM its components;
   - a product as a map's value, held as its components' cells (ADR 0120 on LLVM, the host's struct
     value elsewhere).

   data.md §9's `record-order` and `record-lens` cases run on all four targets.

**A host struct** (records-research §3.2–§3.4):

8. **A host struct is a record set with its codec**, declared in its header module (`windef`'s
   `RECT`, `winuser`'s `MSG`, `commdlg`'s `OPENFILENAMEA`; ADR 0126). Its fields are listed in the C
   definition's order, each with its C type name, whose set is its type's whole value set
   (ADR 0126 item 4).
9. **Its layout is derived, never chosen** (ADR 0094's rule, from a prototype to a definition). It is
   the fold L_{M,p,d}:
   - off(l₁) = 0, and off(lᵢ₊₁) = ⌈off(lᵢ) + k_{lᵢ}⌉ at min(a_{lᵢ₊₁}, p);
   - A = max(maxᵢ min(aᵢ, p), d);
   - N = ⌈end⌉ at A;
   - with M the data model's sizes and alignments, p the header's `pack`, and d an over-alignment.

   **Theorem L**: the fold is the least layout in declared order, and N's rounding is forced by
   tables of the struct.
   - **The packing is the data model's** where the header conditions it on `_WIN64`: ILP32 states
     commdlg's p = 1 (ADR 0126's criterion).
   - It is **the header's** where unconditional: richedit's p = 4.
10. **It is checked, never trusted**: for every declared struct, on every triple, the derived offsets,
    size and alignment equal clang's `-fdump-record-layouts`. That is rust-bindgen's layout tests,
    by `TestWin32DeclarationsAreTheirPrototypes`'s method.
11. **Printed as clang prints it**: an `alloca` of N bytes aligned A, then `getelementptr i8` and a
    store or load per field. Each access is aligned min(A, o & −o), the corollary of Theorem L and a
    proven fact. Padding is written 0. In the IR a record stays its components; only a host call's
    template makes the region.
12. **Theorem F**: a field determined by others is the declaration's.
    - Such a field is a size (`lStructSize`, `cbSize`), a sibling buffer's length (`nMaxFile`,
      `wFindWhatLen`), or a constant (`pvReserved`).
    - The program's record is the product over the free labels, isomorphic to the host's set, and
      the codec writes each determined field.
    - The length owes the buffer's `len` inside its field's set.
13. **Passing** (Theorem P, ADR 0084 item 9): `_In_` encodes a value into a fresh region, `_Out_`
    decodes a result, `_Inout_` both.
    - **Theorem P′**: pointer fields make the views a tree, lent for the call when the host keeps
      none. A pointer field is therefore a sub-value: a C string (`nulfree`), a linear buffer with
      its determined length, a nested record in its mode, or `none`.
    - A record holding a linear buffer is linear for the call.
    - A function-pointer field is `none` until callbacks, and a struct the host keeps (P4) is the
      callbacks ADR's.

## Why not

- **The record only as far as TinyRetroPad needs** (records-research's first recommendation). The
  restriction was the plan's (assessment-2026-10-24's "deliberately not next"), not the algebra's. A
  record is a tuple under a bijection, so it costs the tuple's places. The three positions the tuple
  lacked are the product's limits, and two have better-posed answers for a record than for a tuple:
  names are the agreement a bare product lacks.
- **`record { left ↦ 0 }`** (records-research's first spelling). ADR 0061 already refused `↦` beside
  `:`: under 𝒫(S) ≅ Bool^S both are one λ, spelled two ways.
- **TLA+'s brackets, `[left : LONG]`, `[left ↦ 0]`, `[r EXCEPT !.left = 1]`.** Brackets are indexing
  and segments, `!` is negation, and ADR 0065 item 4 wants a keyword before a former.
- **Z's `r ⊕ g`**, the mathematical symbol for override: ⊕ is J's xor (ADR 0082), a homonym by sort.
  **B's `<+`**: a token no reader outside B knows. **Rust's `record { l : 1, ..r }`**: `..` is J's
  range, and the literal would not be total.
- **`with` without paths** (F#, OCaml, C#): a nested update would nest `with`s and repeat the base. A
  path is lens composition, TLA+'s `!.rc.right`, and the cell's write already takes one.
- **Overlapping paths with a last-wins order**: a path that is another's prefix is put–put.
  Refusing it keeps every `with` a commuting family.
- **A struct as a host type only**, constructors and selectors per field (records-research §7A): a
  product the logic could not name, 2n declarations per struct, 157 fields for TinyRetroPad's 17,
  and no home for Theorem F or `zero`.
- **The language's record with a fixed C layout** (§7B, `repr(C)`, `extern struct`): data-structures
  §4.6, "a struct must not fix its layout", and ADR 0083's choice by place.
- **The layout as `lib/bytes`' product codec in J** (§7C): it needs a codec holding codecs, the
  algebra of sorts' step 2. When a file format needs it, the loader's derivation and the library's
  product must be proven to be one function.
- **LLVM's struct type as the layout** (widths-research §3.3's assumption): LLVM's non-packed struct
  is the natural fold, which is wrong for `PRINTDLGA` on i686, and clang prints none.
- **Defaults in a partial literal** (`OPENFILENAMEA { hwndOwner : h }`): a literal is total
  (data.md §4.2). `zero(T) with` states the default and is defined only where it exists.

## Consequences

- **Amends** data.md §3.3 (the three positions, for tuples too) and §4 (J's spelling, override with
  paths), and spec/theories.md's X1–X3 (a path, a prefix, a mixed former). ADR 0083's record-field
  rule is built by item 9. ADR 0094's derivation is extended from a C call to a C definition.
- **Build order, program first**, each step with its result document and planted faults:
  1. the record in the language (items 1–7) on all four targets, with data.md §9's cases and one per
     lifted position;
  2. the host struct (items 8–13): the layout derivation and its clang check, a first program with no
     window (`GetLocalTime`, `GetClientRect(GetDesktopWindow())`), then TinyRetroPad's parameter
     blocks.
- **Planted faults** (records-research §6, and for the language):
  - a record encoded in written order;
  - a `with` writing the wrong label;
  - overlapping paths accepted;
  - zero where a set lacks 0;
  - commdlg laid out naturally on i686;
  - `__declspec(align)` ignored;
  - an access aligned above min(A, o & −o);
  - a determined field left to the program;
  - an `_Inout_` nested record not decoded.
- **Commits** the language to records everywhere a tuple goes, and the LLVM layer to every C struct
  without bit-fields or unions being declarable by its definition. Unions and bit-fields wait for a
  program.
