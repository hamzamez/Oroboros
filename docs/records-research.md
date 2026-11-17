# Research: records, or what a host does with a struct

2026-11-17. Item 6 of [assessment-2026-11-17](assessment-2026-11-17.md) §5, TinyRetroPad 6d, its first
wall. hamza: "Go with records first, always mathematical and algebraic. check the repo because we
asked this question before see what was decided, what was researched and not. and reach for the
literature". **Research, no decision; nothing built.** The measurements in §2 were made with clang
23.1.4 and the Windows SDK 10.0.26100.0 on both triples. TinyRetroPad's source was read as data
(commit `42f4c6c`, never built or run).

**The short version:**
- **Most of this was decided before.**
  - A record is the language's: a function on a static set of labels, structural, updated by
    `with` ([ADR 0021](decisions/0021-declarations-are-theories.md),
    [data.md](spec/data.md) §4, specified 2026-09-17, not built).
  - A host's struct is that record's representation, and its memory layout is a codec
    ([ADR 0083](decisions/0083-a-representation-is-an-abstraction-function-chosen-by-place.md)
    item "at a host boundary or a record field, the host's type",
    [ADR 0084](decisions/0084-an-address-is-an-integer-and-ownership-is-a-view.md) item 14).
  - A pointer the host does not keep is a value passed and a result returned (ADR 0084's
    Theorem P, item 9).
- **What was never settled**:
  - how a layout is computed, and whether LLVM's struct type is it;
  - what a field the program must not choose is (`lStructSize`, `nMaxFile`);
  - how a 23-field struct of which the program sets 6 is written;
  - a pointer field into the program's memory;
  - J's spelling.
- **The measurements answer the first question against expectation.**
  - Of the 34 layouts (17 types × 2 triples) TinyRetroPad needs, 7 are not the natural layout.
    commdlg.h packs to 1 byte on i686, and `PRINTDLGA` is 66 bytes there, not 68.
  - Clang prints no struct type at all: a field is a byte offset and a load aligned to
    gcd(the record's alignment, the offset).
- **Recommended: R**, the five theorems of §3 built as far as TinyRetroPad needs:
  - the record (data.md §4, in J);
  - a host struct, declared in its header, whose layout is derived like the C call (ADR 0094) and
    checked against clang on every triple;
  - Theorem F: a field determined by others is the declaration's, absent from the program's record;
  - `zero(T) with { … }` for the C idiom;
  - pointer fields lent through the record for one call.

  Six questions for hamza are in §8.

## 0. What was asked before, and what it decided

hamza asked for this check. The repository met records seven times:

| when | document | what it says about records | status |
|---|---|---|---|
| 2026-08-14 | [structs-2026-08-14](../gauntlet/results/structs-2026-08-14.md) | a Church-encoded point does not survive a loop; the first primitive the atom could not do | measured |
| 2026-08-19 | [data-structures.md](data-structures.md) §3.4, §4.6 | a record is a product with named projections, the names scoped to the record (Haskell's cautionary tale); **"a struct must not fix its layout"**, layout is a representation selection (g2: array-of-structs costs JavaScript 2.86×) | research |
| 2026-09-09 | [products.md](products.md) §1 | a record is a tuple up to a bijection on its index set (TLA+); the product built by currying | built (tuples) |
| 2026-09-12 | [structval-2026-09-12](../gauntlet/results/structval-2026-09-12.md) | Win32 struct *by value*: 140 entry points, classified by the ABI; sizes are the host's answer (`win32-sizes.txt`) | measured |
| 2026-09-14/17 | [theories.md](theories.md) §4, [data.md](spec/data.md) §4, **ADR 0021** | **decided**: six data forms, `record` among them; structural, canonical label order, `with` obeying the lens laws; well-formedness derived; X1–X3 refusals ([spec/theories.md](spec/theories.md)) | specified, not built |
| 2026-10-14 | [notation-research.md](notation-research.md) §4.3 | TLA+'s `[name ↦ "ana", age ↦ 42]` and `[name : String, age : Int]`, "reserved until a program needs records" | research |
| 2026-10-27 | [widths-research.md](widths-research.md) §3.3, **ADR 0083** | **decided**: a field's representation is the host's, stated, never chosen; MSG 48/28, `WNDCLASSEXW.cbSize` 80/48 | decided |
| 2026-10-30 | [pointers-research.md](pointers-research.md) §6, §12.7, §14.18, §22, **ADR 0084** | **decided**: Theorem P; P2's modes; ownership a tree; P4 (a struct the host keeps); **"a record's memory layout is a codec"**, a codec a partial isomorphism (Rendel and Ostermann 2010), closed under product, power, order and padding | the scalar codecs built (`lib/bytes`); **the product not built**, it needs "a codec holding codecs, a recursive static value" ([views-2026-10-30](../gauntlet/results/views-2026-10-30.md) §5) |

So the answer to "is a record a language construct or a Win32 type?", which I leaned on before
reading this record, was given in ADR 0021: **both, at different levels**. The record is the
language's set, and the struct is its representation at one host. This document builds on those
decisions and relitigates none of them.

What none of them answered is listed in the short version. §3 derives each answer.

## 1. The issue

TinyRetroPad's 18 entry points that take a struct (tinyretropad-2026-11-17 §2) use 11 struct types.
The program also passes six more through `SendMessage` (pointers-research P5) and inside other
structs: `POINT`, `LOGFONTW`, `CHARFORMATW`, `FINDTEXTEXA`, `CHARRANGE`, `FORMATRANGE`. **17 types.**
Read from the source:

| struct | fields | set or read by the program | how it is used |
|---|---|---|---|
| `OPENFILENAMEA` | 23 | 6: `lStructSize hwndOwner lpstrFilter lpstrFile nMaxFile Flags` | zeroed (`rep stosd`), 6 set, in/out |
| `CHOOSEFONTW` | 16 | 5, `iPointSize` read back | zeroed, set, in/out; `lpLogFont` → a `LOGFONTW` |
| `FINDREPLACEA` | 11 | 7 | a **global**: the modeless dialog keeps it (P4) |
| `PRINTDLGA` | 19 | 4, `hDC` read back | zeroed, set, in/out |
| `PAGESETUPDLGA` | 14 | 2 | zeroed, set, in/out |
| `WNDCLASSA` | 10 | 3: `lpfnWndProc hInstance lpszClassName` | zeroed by `push 10 … rep stosd`, **40 bytes, i686's size** |
| `DOCINFOA` | 5 | 2 | in |
| `MSG` | 6 | `message`, `wParam` read | out of `GetMessageA`, in to `TranslateMessage`/`DispatchMessageA` |
| `RECT` | 4 | all four | out of `GetClientRect`, in to `FillRect`; nested in `PAINTSTRUCT`, `FORMATRANGE` |
| `PAINTSTRUCT` | 6 | none by name | out of `BeginPaint`, in to `EndPaint` |
| `SYSTEMTIME` | 8 | none by name | out of `GetLocalTime`, in to `GetDateFormatA` |
| `POINT`, `CHARRANGE` | 2 | read | results of `EM_POSFROMCHAR`, `EM_EXGETSEL` |
| `LOGFONTW` | 14 | 3 read, `lfFaceName` a `WCHAR[32]` | filled by `ChooseFontW` through `CHOOSEFONTW` |
| `CHARFORMATW` | 9 | 6, `szFaceName` a `WCHAR[32]` | in to `EM_SETCHARFORMAT` |
| `FINDTEXTEXA`, `FORMATRANGE` | 3, 5 | all | in/out of `EM_FINDTEXTEXA`, `EM_FORMATRANGE`; nested records |

Four patterns, each the subject of a theorem in §3:
- **Zero, then set a few fields.** It occurs eight times: the C idiom `T x = {0}; x.f = …`.
- **Fields the program must not choose.** `lStructSize`/`cbSize` is the layout's size, 7 times;
  `nMaxFile` is the length of the buffer in `lpstrFile`, 5 such lengths; `pvReserved` and
  `dwReserved` are 0. The `WNDCLASSA` line is this bug class in the program itself: a hard-coded
  size, right on one triple only.
- **A struct that is a result** (`MSG`, `RECT`, `PAINTSTRUCT`, `SYSTEMTIME`, `POINT`, `CHARRANGE`),
  read by name or handed back unchanged.
- **Pointer fields into the program's memory.** Strings (`lpszClassName`, `lpstrFilter`), buffers
  the host fills (`lpstrFile`), a nested record by pointer (`lpLogFont`), and functions
  (`lpfnWndProc`, the hooks).

## 2. Measured

### 2.1 The layouts

Each type's layout on each triple is clang's (`-fdump-record-layouts`). Each was compared with the
**natural layout** computed from its fields' `sizeof` and `_Alignof`, also clang's
(`scratchpad/records/parse.py`):

| struct | x86_64 size/align | i686 size/align | natural? |
|---|---|---|---|
| `PAINTSTRUCT` | 72/8 | 64/4 | both |
| `MSG` | 48/8 | 28/4 | both |
| `RECT`, `POINT`, `CHARRANGE` | 16/4, 8/4, 8/4 | same | both |
| `WNDCLASSA` | 72/8 | 40/4 | both |
| `SYSTEMTIME` | 16/2 | 16/2 | both |
| `DOCINFOA` | 40/8 | 20/4 | both |
| `LOGFONTW`, `CHARFORMATW` | 92/4 | 92/4 | both |
| `OPENFILENAMEA` | 152/8 | **88/1** | x64; i686 packed, offsets natural |
| `CHOOSEFONTW` | 104/8 | **60/1** | x64; i686 packed, offsets natural |
| `FINDREPLACEA` | 80/8 | **40/1** | x64; i686 packed, offsets natural |
| `PAGESETUPDLGA` | 128/8 | **84/1** | x64; i686 packed, offsets natural |
| `PRINTDLGA` | 120/8 | **66/1** | x64; **i686: 8 fields moved**, `hInstance` at 34, not 36; natural would be 68 |
| `FINDTEXTEXA` | **24/4** | 20/4 | i686; x64 `pack(4)`, offsets natural, alignment 4 not 8 |
| `FORMATRANGE` | **56/4** | 48/4 | i686; x64 `pack(4)`, alignment 4 not 8 |

The causes are two lines of the SDK:
- **commdlg.h**, `#if !defined(_WIN64)`, then `#include <pshpack1.h>` "Assume byte packing throughout";
- **richedit.h**, an unconditional `pshpack4.h`. It changes nothing on i686, whose natural
  alignments are at most 4.

**The census.** Every struct clang laid out under `windows.h`, `commdlg.h` and `richedit.h`
(`census.py`). A struct with a bit-field or an anonymous member was set aside: 21 on x86_64, 20 on
i686.

| | natural | alignment only | offsets or size moved |
|---|---|---|---|
| x86_64, 125 | 121 | 3 (`_M128A`, which is `__declspec(align(16))`; `FINDTEXTEXA`; `FORMATRANGE`) | 1 (`IMAGE_AUX_SYMBOL_TOKEN_DEF`) |
| i686, 125 | 118 | 5 | 2 (`PRINTDLGA`, `IMAGE_AUX_SYMBOL_TOKEN_DEF`) |

So the natural layout is the rule, about 95%. The exceptions are a header's packing (`#pragma pack`,
which lowers alignments) and an over-alignment (`__declspec(align)`, which raises one). Both appear
in TinyRetroPad's own 17.

### 2.2 What clang prints

A field access as clang 23.1.4 prints it at -O1 (`use.c`):

```llvm
; i686, PRINTDLGA (align 1): p->hInstance
%2 = getelementptr inbounds nuw i8, ptr %0, i32 34
%3 = load ptr, ptr %2, align 1
; x86_64, PRINTDLGA (align 8): p->nFromPage, offset 44
%2 = getelementptr inbounds nuw i8, ptr %0, i64 44
%3 = load i16, ptr %2, align 4
; x86_64, FINDTEXTEXA (align 4): f->lpstrText, offset 8
%3 = load ptr, ptr %2, align 4
```

**No struct type at all.** A field is a byte offset from the base, and the load's alignment is
min(A, the largest power of two dividing the offset), where A is the record's alignment. §3.2
derives that rule. widths-research §3.3 assumed that LLVM's `%MSG = type { … }` computes the layout
from the data layout. That holds for the natural 95% only: LLVM's non-packed struct is the natural
fold, which is wrong for `PRINTDLGA` on i686. So the printer should print offsets, as clang does.

### 2.3 The modes

The SAL on TinyRetroPad's 18 entry points that take a struct:
- **`_In_`**: six (`EndPaint`, `DispatchMessageA`, `TranslateMessage`, `IsDialogMessageA`,
  `FillRect`, `RegisterClassA`), and `StartDocA`;
- **`_Out_`**: four (`BeginPaint`, `GetMessageA`, `GetClientRect`, `GetLocalTime`);
- **in/out, by their documentation**: the five modal common dialogs, which have no SAL;
- **kept by the host** (P4): `FindTextA` and `ReplaceTextA`.

SAL's field annotations (`_Field_size_`) appear in 82 SDK headers, but in none of commdlg.h's and
richedit.h's structs and once each in WinUser.h and wingdi.h. So a field's dependence on another is
read off the documentation, by hand (ADR 0022), as a C function's mode was.

## 3. The algebra

### 3.1 A record is a dependent function on its labels

data.md §1.1: a record of type `record { l₁ ∈ S₁, …, lₙ ∈ Sₙ }` is an element of the dependent
product

```
Π_{l ∈ L} S_l = { r : L → ⋃ S_l | r(l) ∈ S_l for every l ∈ L }
```

L is finite and static. This is TLA+'s `[l₁ : S₁, …]` (Lamport, *Specifying Systems*, 2002, §5.2),
TAPL's record type (Pierce 2002, §11.8), and the categorical product with its projections π_l.
Everything in data.md §4 follows, and is not repeated here:
- **well-formedness** is what "a function" means;
- **structural equality**, since a function has no order on its domain;
- **canonical order** as the representation's bijection L ≅ {0…n−1};
- **`with`** is TLA+'s EXCEPT. Projection and update form a very well-behaved lens (Foster et al.,
  TOPLAS 2007): put–get, get–put, put–put.

**Lemma 3.1 (independent fields commute).** For l ≠ m, `(r with {l ↦ a}) with {m ↦ b}` and
`(r with {m ↦ b}) with {l ↦ a}` are equal.
*Proof.* Both are the function equal to r except at l and m, where they are a and b. ∎

This is why a C program's field writes can be reordered. §3.4 needs its memory form.

### 3.2 A layout is a codec, and the natural layout is the least one

A **layout** of a record over a data model M gives each label l an offset off(l) ∈ ℕ and a codec
c_l : S_l ⇄ B^{k_l} (ADR 0084 item 14). It gives the record a size N and an alignment A. Two
conditions make it a layout:

```
(disjoint)  the byte ranges [off(l), off(l) + k_l) are pairwise disjoint and inside [0, N)
(aligned)   a_l | off(l) for every l,  and  A | N
```

Here a_l is the field's alignment in M: a power of two. Under (disjoint), the record's codec is the
product of its fields' codecs with `pad` in the gaps (pointers-research §22.1's table), so it is a
partial isomorphism Π S_l ⇄ B^N: **a layout is a codec, and every codec of this form is a layout.**
A layout is a function on L and has no order. C's declared order enters only through the fold that
computes the one C chooses.

**Theorem L (the natural layout is the least).** Fix the declared order l₁ … lₙ and the
requirement that offsets increase along it. Let

```
off(l₁) = 0
off(lᵢ₊₁) = ⌈off(lᵢ) + k_{lᵢ}⌉_{a_{lᵢ₊₁}}        (⌈x⌉_a: the least multiple of a that is ≥ x)
A = max_i a_{lᵢ}
N = ⌈off(lₙ) + k_{lₙ}⌉_A
```

Then (off, N) is a layout, and every layout in that order has off′(lᵢ) ≥ off(lᵢ) for all i and
N′ ≥ N.

*Proof.* By induction on i:
- off′(l₁) ≥ 0 = off(l₁).
- If off′(lᵢ) ≥ off(lᵢ), then disjointness in order gives off′(lᵢ₊₁) ≥ off′(lᵢ) + k ≥ off(lᵢ) + k.
- Alignment makes off′(lᵢ₊₁) a multiple of a_{lᵢ₊₁}, and ⌈·⌉_a is the least such that is ≥ that
  bound.
- N likewise, with A, which for powers of two is the lcm of the a_l.

The fold satisfies (disjoint) and (aligned) by construction. ∎

**Why the size is rounded to A: the power codec forces it.** A table of records `Array(T)`
(ADR 0084's `times`) puts element j at j·N. Its field l is aligned for every j iff a_l | N, for
every l; that is, iff A | N. So N's rounding is no convention. It is the least N under which the
power of the codec is a layout. That is C11 §6.7.2.1's trailing padding, derived.

**The two exceptions are the same fold with one parameter changed:**
- **`#pragma pack(p)`** replaces each a_l by min(a_l, p). In the divisibility lattice of powers of
  two, min is the meet. Microsoft's documentation states the rule: a member's alignment is the
  smaller of p and its natural alignment.
- **`__declspec(align(d))`** replaces A by max(A, d), the join.

So C's layout on a triple is L_{M,p,d}: one fold, parameterized by the model's sizes and alignments
and by the header's p and d. The 7 non-natural layouts of §2.1 are p = 1 (commdlg, i686) and p = 4
(richedit, x86_64). `PRINTDLGA` moves only because after its five `WORD`s the offset is 34, where
⌈34⌉₄ = 36 but ⌈34⌉₁ = 34.

**Corollary (the access's alignment).** A record placed at an address divisible by A, with a field
at offset o, has that field at an address divisible by gcd(A, 2^{ν₂(o)}) = min(A, o & −o), and by no
larger power of two in general. That is clang's `align` in §2.2. It is a proven fact about the
address, so the printer may state it (ADR 0080: "a flag only where the IR proved it"). A larger one
is a false claim, and §6 plants it.

### 3.3 Theorem F: a field determined by others is not data

Many Win32 fields are functions of the rest of the record:

| determined field | its value | in TinyRetroPad's 17 |
|---|---|---|
| `lStructSize`, `cbSize` | N, the layout's size | 7 |
| `nMaxFile`, `nMaxFileTitle`, `nMaxCustFilter`, `wFindWhatLen`, `wReplaceWithLen` | the length of the buffer in a sibling field | 5 |
| `pvReserved`, `dwReserved` | 0 | 2 |

**Theorem F.** Let the host's set be

```
S_C = { c ∈ Π_{l ∈ L} S_l | c(d) = f_d(c|_F) for every d ∈ D }
```

where L = F ⊎ D, the free and the determined labels, and each f_d reads only free labels. Then the
restriction c ↦ c|_F is a bijection S_C → Π_{l ∈ F} S_l. Its inverse extends r by
d ↦ f_d(r).
*Proof.* Restriction is injective on S_C, because c(d) is fixed by c|_F. The extension lands in S_C
by definition. The two compose to the identity both ways. ∎

The same thing seen three ways:
- A dependent sum with a definitional component, `Σ (r ∈ Π_F) {f(r)} ≅ Π_F`, since 1 × A ≅ A.
- Deputy's `count(n)` fields (Condit, Harren, Anderson, Gay and Necula, "Dependent types for
  low-level programming", ESOP 2007).
- SAL's `_Field_size_(n)`.

**What it decides.** The program's record type is Π over F, and **the declaration writes the D
fields**: the codec computes each f_d while encoding. So a program cannot write 80 into
`WNDCLASSEXW.cbSize` on i686, or a buffer's length different from the buffer's. There is no field to
write. On a result, the decoder may check a determined field (a host that answers a length) or
ignore it. Both are the declaration's.

This is not width subtyping, which data.md §1.3 refuses: Π_F and S_C are isomorphic, and the
isomorphism is fixed by the declaration.

### 3.4 Passing a record: Theorem P, on a tree

ADR 0084 item 9 (P2) gives a pointer parameter its mode. On a record pointer:
- `_In_`: the record is a value passed. The call encodes it into a fresh region of N bytes and
  passes the region's address.
- `_Out_`: the record is a result. A fresh region, the call, then decode. This is ADR 0048's cell
  with its set a product.
- `_Inout_`: both.

Theorem P (pointers-research §2.3) says this is observably the host's behaviour whenever the host
does not keep the pointer. ADR 0084 decided that already.

**Pointer fields make the record's view a tree** (pointers-research §12.7, "ownership is a tree").
`OPENFILENAMEA` points at a filter string and at a buffer the host fills. `CHOOSEFONTW` points at a
`LOGFONTW`.

**Theorem P′ (a tree lent for one call).** Suppose the host keeps no pointer of the tree past the
call. Then lending the tree's root lends every node's view for the call, and returns each one. The
call is a function of the nodes' contents, and its effect is a new content for each node lent
writable.

*Proof.* By induction on the tree's depth, using Theorem P at each node. Disjointness of the nodes'
regions holds because each is fresh, or is a distinct buffer of the program's, proven distinct as
ADR 0084 item 6 proves two segments. ∎

So a pointer field is a sub-value, never an address the program handles:
- **a C string**: `lpszClassName`, `lpstrFilter`; ADR 0091's `nulfree`;
- **a buffer, `linear`, lent writable**: `lpstrFile`. Its length is a determined field, by
  Theorem F;
- **a nested record**: `lpLogFont`, in its mode;
- **`none`**: a null field, `_In_opt_` as an `Option` (P2).

A record holding a linear buffer is itself linear for the call, as a product with a linear factor is
in linear logic. Lemma 3.1's memory form is (disjoint): the field writes of an encoding commute
because their byte ranges are disjoint, a separation of views (ADR 0084 item 2).

A pointer the host **keeps** (`FINDREPLACEA` in `FindTextA`) breaks the premise. It is P4's protocol
and belongs to callbacks, since it is answered through the window procedure.

### 3.5 The zero and the C idiom

TinyRetroPad's eight `rep stosd` blocks are C's `T x = {0}` followed by field writes. The algebra
names the element and the operation:

**zero(T)**, for `T = Π_{l ∈ F} S_l`, is the record whose every free field is its set's zero:
- the integer 0 where 0 ∈ S_l;
- `none` for an `Option`;
- zero of a nested record, recursively.

It is defined iff every S_l has a zero, and is otherwise refused, naming the field. That is
derived, not listed: a field whose set lacks 0 must be given. **`zero(T) with { l ↦ e, … }`** is
then the C idiom exactly. By Lemma 3.1 the writes' order does not matter, and by put–put a label
written twice is refused (data.md §4.3).

Zero is not a default added to the language. It is the record whose encoding, under every codec
here, is all zero bytes. That is why C's `{0}` works, and the encoder may realize it as the region's
zero fill (ADR 0097's "born full" otherwise).

A record cell (`var ofn := zero(T)`) written in program order, `ofn.Flags := f`, is G6's place
extended to a label, decided by hamza on 2026-10-15 for `dt[s][1] := v`. The state translation gives
the `with`.

### 3.6 Two kinds of struct, one rule

| kind | examples | needs records as values? |
|---|---|---|
| a **result**, read by name or handed back | `MSG`, `RECT`, `PAINTSTRUCT`, `SYSTEMTIME`, `POINT`, `CHARRANGE`, `LOGFONTW` | **yes**: `msg.message`, `rc.right` |
| a **parameter block**, built only to be passed | `OPENFILENAMEA`, `PRINTDLGA`, `WNDCLASSA`, `DOCINFOA`, `CHARFORMATW` | no: the call could take its fields |

A parameter block passed `_In_` is the call's arguments curried, `f(&(x₁, …, xₙ)) = f′(x₁, …, xₙ)`:
products.md's currying isomorphism, and ADR 0118's linear term over a C call's parameters extended
to a product. So a declaration could take the fields as parameters and build the struct in its
template.

But results need records as values. With records as values, a parameter block is an ordinary record
argument, `zero(T) with {…}`, and needs no second mechanism. **So one rule**: a struct is a record,
and a wrapper with parameters is a definition in J where a program wants one.

## 4. The targets

- **The record (§3.1, §3.5) is portable by construction.** It lowers as the tuple of its canonical
  order, and products are built on all four printers: several results natively on Go, an object on
  JavaScript, a record on Java (ir.md §1.1). data.md §0's verdict stands: Tier 1, since no backend
  receives anything it did not receive before. A conformance case catches the order hazard of
  data.md §4.4: an emitter that used written order.
- **A host struct (§3.2–§3.4) is the LLVM layer's.**
  - The layout L_{M,p,d} is derived by the target loader from the declaration and the data model,
    as ADR 0094 derives a C call's ABI from its prototype.
  - It is printed as clang prints it: an `alloca` of N bytes aligned A, then `getelementptr i8` and a
    store or load per field, each aligned by the corollary.
  - In the IR a record stays flattened into its components. Only the host call's template makes the
    region, so no IR form is added.
- **Go's host structs are already Go's** ([structlit-2026-09-10](../gauntlet/results/structlit-2026-09-10.md):
  462 constructors, composite literals and selectors, generated). They are the same abstraction
  function (H, d) with H the Go struct type. Making them record views is the same shape and **not
  next** (CLAUDE.md: records only as far as TinyRetroPad's walls need).
- **The JVM and JavaScript** have no struct at their boundaries that TinyRetroPad's track needs.

**The data model owns the packing.** commdlg's `pack(1)` is under `#if !defined(_WIN64)`, so ADR
0126's criterion applies: a definition that reaches `_WIN64` is the data model's, like basetsd.h's
four names. ILP32 states commdlg's p = 1. richedit's `pack(4)` is unconditional and is the header's.

## 5. The type system

- **Sets.** A field's set is its C type's whole value set (ADR 0126 item 4): `LONG` is S₃₂ and `WORD`
  0..65535. A handle is `ptr` today (ADR 0126); P3's typed handles wait. A fixed array `WCHAR[32]` is
  `{ t ∈ Array(U16) | len(t) = 32 }`, a set by separation (ADR 0061), whose codec is the power.
- **Obligations.**
  - Encoding owes each free field's membership in its set, decided as any value flowing into a
    declared set is.
  - A determined length owes the buffer's `len` inside its field's set: `wFindWhatLen` is a `WORD`,
    so the buffer must be at most 65535. This is the narrowing ADR 0119 owes a store.
  - `zero(T)` owes 0 ∈ S_l per field, decided statically.
- **The layout is checked, never trusted** (ADR 0022, 0126). For every declared struct, on every
  triple, the derived (off, N, A) must equal clang's `-fdump-record-layouts`. That is rust-bindgen's
  generated layout tests (size, alignment and every offset asserted), and GHC's `hsc2hs`, whose
  `#peek` takes an offset from the C compiler. It is `TestWin32DeclarationsAreTheirPrototypes`'s
  method extended from prototypes to definitions.
- **Padding is written zero.** C leaves padding bytes unspecified (C11 §6.2.6.1, and Memarian et
  al., "Into the depths of C", PLDI 2016, on the de facto standard). A codec's `pad` writes 0 and
  ignores it on reading (pointers-research §22.1), so encoding is a function of the record, as the
  emitter must be.

## 6. Planted faults, each before its witness is trusted

| rule | the fault | what must fail |
|---|---|---|
| the pack (§3.2) | lay commdlg out naturally on i686 | the layout test on `PRINTDLGA` (68 against 66); at run time, the dialog's `lStructSize` check |
| over-alignment | ignore `__declspec(align)` | the layout test on `_M128A`, 16 |
| the access's alignment | state the field's natural alignment instead of the corollary's | a verifier rule comparing each `align` with min(A, o & −o), on `FINDTEXTEXA` x64 (8 against 4) |
| Theorem F | let the program's record carry `lStructSize` | X2-style refusal: a determined field given by the program |
| Theorem F, lengths | encode a buffer's length from a stale binding | the differential: the host reads past the buffer, or truncates |
| zero | allow zero(T) where a field's set lacks 0 | the refusal names the field; a plant that drops the check must be caught by a program giving a set without 0 |
| canonical order | encode in written order | data.md §9's record-order conformance case |
| Theorem P′ | lend a nested record and do not decode it after an `_Inout_` call | `ChooseFontW`'s `LOGFONTW` read back unchanged in the differential |

## 7. Candidates

- **A. A host type only.** A struct is a host type in its header, with a constructor and selectors
  per field, like Go's struct literals today. No record in the language.
  - *Against*: ADR 0021 decided the record, and a struct would be a product the logic cannot name,
    so `rc.right - rc.left` would be a host call.
  - Every Win32 struct would need 2n declarations, and TinyRetroPad's 17 have 157 fields.
  - The C idiom (§3.5) and Theorem F would have no algebra to live in.
- **B. A record with a fixed C layout.** The language's record is a C struct, as in C, Zig's
  `extern struct` or Rust's `#[repr(C)]`.
  - *Against*: data-structures.md §4.6, "a struct must not fix its layout". That is a portability
    hazard like a fixed integer width (ADR 0026), and it contradicts ADR 0083: a representation is
    chosen by place, the host's at the host.
- **C. The layout as `lib/bytes`' product codec, in J.** The codec of §3.2 written in the language,
  `pair`, `pad` and `times`, and the host declaration naming it.
  - *For*: one mechanism with file formats.
  - *Against, today*: it needs a codec holding codecs, a recursive static value, which is the
    algebra of sorts' step 2 (typevalues.md §10). Nothing else needs that yet, and a C struct's
    layout is a derivation from its C definition, not something the program should choose.
  - When a file format asks for it, the loader's derivation and the library's product must be
    proven to be one function, or the loader's becomes the library's.
- **R. The record, and a host struct as its representation (recommended).** §3.1 and §3.5 in the
  language. §3.2–§3.4 in the target loader and `ir/llvm`, derived from the header's C definition and
  the data model, and checked against clang. It is what ADR 0021, 0083 and 0084 decided, plus the
  five answers they lacked. Its debt is C's, named in §9.

## 8. Decisions for hamza

1. **Build the record now, as far as TinyRetroPad needs**: its introduction, projection, `with`,
   `zero(T)`, and a cell's component write (G6 extended). Recommended: yes. ADR 0021 decided it, and
   six of the 17 structs are results read by name.
2. **J's spelling.**
   - Recommended: the set `record { left ∈ LONG, top ∈ LONG, … }`, membership as a header writes it
     (ADR 0062); the value `record { left ↦ 0, … }`, a function's graph as `map { k ↦ v }` writes it;
     projection `r.left`, TLA+'s own sugar for r["left"]; update `r with { left ↦ 1 }`; `zero(T)`.
   - The alternative is TLA+'s brackets, `[left : LONG]`, `[left ↦ 0]`, `[r EXCEPT !.left = 1]`.
     They break ADR 0065 item 4 (a set former keeps its keyword), and brackets are already indexing
     and segments.
   - `r.left` meets J's `a.b`, a module's member. In one namespace (typevalues.md §2) the binder
     decides which: a value is projected, a module is qualified.
   - Labels are bare in a record former, as tags are not (ADR 0059 keeps `'x` for a label used as
     data).
3. **A host struct is a record set with its codec**: declared in its header module (`windef` `RECT`,
   `winuser` `MSG`, `commdlg` `OPENFILENAMEA`, consistent with ADR 0126), its layout L_{M,p,d}
   derived, and checked against clang on both triples. Recommended: yes.
4. **Theorem F**: a determined field is the declaration's, a size or a sibling buffer's length or a
   constant, and absent from the program's record. Recommended: yes. It removes a bug class the
   program itself has (`push 10` for `WNDCLASSA`).
5. **Pointer fields**: lent through the record for one call (Theorem P′). A C string, a linear buffer
   with its determined length, a nested record, or `none`. A function-pointer field is `none` until
   callbacks. Recommended: yes, with P4 (a struct the host keeps) left to the callbacks ADR.
6. **The packing as the data model's** where the header conditions it on `_WIN64` (commdlg), and the
   header's otherwise (richedit). Recommended: yes, ADR 0126's criterion.

## 9. Not here

- **Callbacks**: `lpfnWndProc`, the hooks, `DLGPROC`; P4's kept struct (`FINDREPLACEA`). That is
  6d's second wall, and its own research.
- **Unions and bit-fields**: none in TinyRetroPad's 17; 20 or 21 in the census's set-aside. Sums
  over bytes wait for a program (pointers-research §22.4 item 3).
- **A table of records' layout**, array-of-structs or struct-of-arrays (data-structures §4.6): no
  table of records in TinyRetroPad.
- **Go's and the JVM's structs as record views**, row polymorphism, width subtyping (refused,
  data.md §1.3), records in `lib/bytes` (C).
- **A first program**, chosen so a test can run it without a person:
  - `GetLocalTime` (an `_Out_` record);
  - `GetClientRect(GetDesktopWindow())` (an `_Out_` record, no window procedure needed);
  - a `PRINTDLGA` with `PD_RETURNDEFAULT`, the pack's run-time witness, to be measured before it is
    relied on.

  Then TinyRetroPad's parameter blocks, whose dialogs need a person and are checked by layout and by
  the differential's encoded bytes.

## Paid and bought (this research)

- **Compiler, emission, proofs**: unchanged; nothing built.
- **The measurement**: three scripts in the scratchpad (`records/parse.py`, `census.py`, `use.c`),
  clang's layouts on both triples, under an hour.
- **Owed by R when built**, an estimate, not a measurement:
  - the reader and D for four forms;
  - the checker's record type;
  - the target loader's struct declarations and L_{M,p,d};
  - the printer's region and accesses;
  - the layout test.

  Each is to be measured in its result document.
