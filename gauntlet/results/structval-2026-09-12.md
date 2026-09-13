# Struct by value was the largest refusal because the survey was reading the pointer off the declarator

2026-09-12. `gauntlet/stdlib/win32.go`, `gauntlet/stdlib/win32-sizes.txt`,
`gauntlet/stdlib/acceptance/pointer-param.oro`. assessment-2026-09-11 item 4 —
*"then struct by value on Win32, research first, starting from the measured enum
share."*

The research was supposed to classify 945 names. It found that 805 of them are
not struct arguments at all.

> **`TYPE *name` WAS READ AS A VALUE.** C binds `*` to the DECLARATOR, so
> `SURFOBJ *pso` is a pointer whose name is `pso` — and `parseParams` took the
> last whitespace-separated field as the name and threw the star away. **Struct
> by value 945 (8.2%) → 140 (1.2%)**, from the largest remaining refusal to the
> third; **declarable 81.4% → 90.5%**.
>
> **AND IT RAN THE OTHER WAY TOO, WHICH IS THE DIRECTION THAT MATTERS.** `BOOL
> *pfOn` was read as a WORD, so a pointer the program cannot build counted as
> passable: **callable 36.8% → 35.4%, usable 34.1% → 32.4%**, and **585 of the
> 9,427 generated declarations typed a parameter as an integer where the host
> wants an address.** `GetDevicePowerState` was `(int int)`, and `kernel32`
> exports it.
>
> **THE ENUM DECISION MOVED TO THE BRACE**, after being wrong twice by regex:
> three SDK spellings `reEnumTD` cannot see, worth another 116 refusals and
> **6,031** names the struct table had held. The host confirms all 403 of them,
> with a control that must be refused.
>
> **THE 140 THAT REMAIN, CLASSIFIED BY WHAT THE ABI DOES, with MSVC as the
> oracle**: **12 aggregate types in one register** (1/2/4/8 bytes, 84 refusals)
> and **12 by reference** (56). The result position is 8 refusals, and **exactly
> ONE** needs the hidden-pointer return convention.
>
> **AND THE REAL CEILING IS THE LINK LINE: 666 of 4,093 callable names, 16.3%.**
> The other 77.2% are in one of 145 import libraries `build.bat` does not name.
>
> **No compiler change.** Every emitted file in the repository is untouched.

---

## 1. What the research was for, and what it ran into

assessment-2026-09-11 §5 item 4 put struct by value last of four because it was
the largest Win32 refusal left: 945 names, 8.2%, and win32enum-2026-09-11 had
just written *"now what it was always said to be: aggregates."*

The plan was maxlen-2026-08-28's discipline — classify every blocked name by the
fact that would settle it, before building anything. Win64 splits an aggregate
argument two ways and the split is not ours to choose:

| size | argument | result |
|---|---|---|
| 1, 2, 4 or 8 bytes | in a register, by value | in RAX |
| anything else | a POINTER to a caller-allocated copy | a hidden pointer in RCX, returned in RAX |

So the first question is how the 945 divide, and the second is which aggregates
they are. Printing the second answer is what found the bug: **`SURFOBJ` blocking
78 entry points, `FONTOBJ` 18, `PATHOBJ` 10** — GDI driver types that are never
passed by value, always through a pointer.

## 2. The bug: C binds `*` to the declarator

```c
BOOL EngBitBlt(_In_ SURFOBJ *psoTrg, _In_ SURFOBJ *psoSrc, …);
```

`parseParams` normalises the whitespace and takes the last field as the
parameter's name:

```
fields   ["SURFOBJ", "*psoTrg"]
name     "*psoTrg"        ← does not END with `*`, so it is taken as the name
type     "SURFOBJ"        ← and the pointer is gone
```

`-sig`, added for this, says it without a sum in the way:

```
EngBitBlt  [winddi.h]
  arg 0  type "SURFOBJ"   name "*psoTrg"   sal "_In_"   shape STRUCT BY VALUE
```

There is a second hole of the same shape: `TYPE name[]` also lost its pointer,
because `TrimSuffix(d, "[]")` removed the brackets before the test that looks
for them could see them. Both are now read off the declarator, where C puts
them.

**This is the tool's dominant style blind spot rather than an edge case.** The
SDK writes `LPDWORD lpcb` in the old headers and `DWORD *lpcb` in the newer
ones, and the survey read the first correctly and the second not at all.

### 2.1 The direction that is a wrong answer, not a wrong number

Losing a star off `SURFOBJ` under-counts declarability, which is embarrassing.
Losing it off `BOOL` does something else: `BOOL` is a word, so the parameter
became a value the program can compute, and `canPass` accepted it with no SAL
annotation needed.

**585 of the 9,427 declarations the generator wrote had at least one parameter
typed that way**, every one of them an address typed as an integer:

```
ImageList_GetIconSize   (int int int)                  ->  (int ptr ptr)
LoadIconMetric          (int string int int)           ->  (int string int ptr)
ActivateActCtx          (int int)                      ->  (int ptr)
GetDevicePowerState     (int int)                      ->  (int ptr)
GetCurrentActCtx        (int)                          ->  (ptr)
```

`GetDevicePowerState(HANDLE, BOOL *pfOn)` writes a `BOOL` through the second
argument. Declared `(int int)`, a program could pass `0`, the type checker would
accept it, and the host would write to address 0. Twelve of the 585 are in a
library the build links, so the reach was not hypothetical.

**That is assessment-2026-09-11 §4 item 1 arriving with an instance**: *"a
generated declaration should be checked by the host that will run it, not by the
next program that happens to use it."* Nothing checked these, and 4,257 callable
names were reported on the strength of them.

### 2.2 The witness, and both of its sides

`gauntlet/stdlib/acceptance/pointer-param.oro`, the tenth acceptance program.
`IsBadReadPtr(_In_opt_ VOID *lp, _In_ UINT_PTR ucb)` is the right witness
because the annotation says the pointer may be NULL, so the program **runs**
instead of being refused:

```lisp
(seq (fmt.print-int (w.IsBadReadPtr (x.null) 1))
     (fmt.print-int (w.IsBadReadPtr (x.null) 0)))
```

It prints **1** then **0**, which is what a C program on this host prints —
address 0 is not readable, and with a length of zero there is nothing to check.
**Two answers from one declaration, so the program cannot be right by
accident.**

And the two declarations are each other's negative, which is what makes this a
test rather than a demonstration:

| | old declaration `(int int)` | fixed declaration `(ptr int)` |
|---|---|---|
| `(x.null)` | refused — *"x64.null is ptr, but int is required here"* | builds, prints 1 then 0 |
| literal `0` | **builds** | refused — *"an integer literal is int, but ptr is required here"* |

The bottom-left cell is the finding: the old declaration compiled a program that
hands an integer to a host function expecting an address.

## 3. The enum decision moved to the brace

With the declarator fixed, the residue still held names that are plainly enums —
`JsErrorCode` alone blocked 99 results. win32enum-2026-09-11 decided
enum-or-struct with a regex over `typedef enum TAG [: BASE] { … } NAME;`, and
the SDK writes three things it cannot see:

```c
typedef _Return_type_success_(return == 0) enum _JsErrorCode : unsigned int
    { … } JsErrorCode;          /* junk between typedef and enum, AND a
                                   two-token base where one is allowed */

enum tagEapHostPeerMethodResultReason { … } EapHostPeerMethodResultReason;
                                /* no typedef at all — legal C++, and used */
```

`reStruct` finds `} NAME;` and says nothing about what that brace **opened**. So
the decision now goes to where the fact is: **match the brace backwards and read
the keyword in front of it**, stopping at the nearest `;`, `{` or `}` so nothing
from a neighbouring declaration or a nested member can leak in. That answers
every spelling at once — the difference between fixing a bug and fixing one of
its instances.

Worth **116 more refusals**, and it takes the enum tables from 10,028 names to
**10,109**, of which **6,031** (was 5,831) the struct table had been holding.

### 3.1 Getting this wrong the other way is the dangerous direction, so the host decides

A struct read as an enum is worse than an enum read as a struct. An aggregate
over 8 bytes is passed as a **pointer to a copy**, so declaring it a word makes
the callee dereference whatever integer the program passed.

`-check-enums` asks MSVC. `T v = (T)0; return (int)v;` compiles for an enum or an
integer typedef and fails for a struct or a union, so the host settles it:

```
ENUM CHECK: 407 names a declaration relies on, 403 spellable in C
  the control RECT was refused, so a struct read as an enum would be caught
  and the host accepts every one of the rest as an integer type
```

**The control is not decoration.** The probe compiles 407 functions and reports
the ones cl refuses; if cl accepted everything the check would pass while
testing nothing. `RECT` is put in the list on purpose and must be refused.
Four names are not spellable in a C translation unit at all — behind a version
macro or C++-only — and are reported as that rather than counted either way.

## 4. What the 945 turned into

| | 2026-09-11 | declarator | + brace |
|---|---:|---:|---:|
| declarable | 9,427  81.4% | 10,369  89.6% | **10,479  90.5%** |
| callable | 4,257  36.8% | 4,089  35.3% | **4,093  35.4%** |
| and the call says it all | 3,950  34.1% | 3,753  32.4% | **3,756  32.4%** |
| struct by value | 945  8.2% | 256  2.2% | **140  1.2%** |
| unresolved typedef | 792  6.8% | 538  4.6% | **538  4.6%** |
| function pointer | 329  2.8% | 337  2.9% | **343  3.0%** |
| floating point | 36  0.3% | 29  0.3% | **29  0.3%** |

Three things in that table are worth reading twice.

**Declarability went UP 9.1 points and callability went DOWN 1.4**, from one
fix. A pointer is declarable where a struct by value is not, and a pointer is
unpassable where a word is not — so the same misread inflated one number and
deflated the other. A survey of one's own language must not round in its own
favour, and this rounded both ways at once.

**Function pointer went UP.** The classifier stops at a function's FIRST
refusal, so clearing struct-by-value on an early parameter exposes a callback on
a later one. A refusal table is a table of first refusals, and the report now
says so.

**Unresolved typedef fell 254** with nothing written for it: `SOMETHING *p` is a
pointer whether or not `SOMETHING` resolves, so a star recovered is a typedef
that no longer needs resolving.

## 5. The 140 that remain, by ABI class

The sizes are MSVC's, written by `-sizes` into `gauntlet/stdlib/win32-sizes.txt`
and read from there. **Not computed here**, and the reason is not laziness:
Win64 layout is natural alignment plus `#pragma pack` plus bitfield packing plus
anonymous unions, and a size wrong by one byte moves an aggregate between
classes and so moves the roadmap. MSVC already knows. All 24 were measured and
the host refused none.

```
STRUCT BY VALUE: 24 aggregate types across 140 refusals
  ABI class                           types   args results
  1/2/4/8 bytes — one register           12     77      7
  other — by reference / RCX             12     55      1
```

| aggregate | size | class | args | results |
|---|---:|---|---:|---:|
| COORD | 4 | one register | 19 | 2 |
| GUID | 16 | by reference | 18 | 0 |
| EAP_METHOD_TYPE | 16 | by reference | 16 | 0 |
| POINT | 8 | one register | 14 | 0 |
| CY | 8 | one register | 12 | 0 |
| LUID | 8 | one register | 11 | 0 |
| IORING_HANDLE_REF | 16 | by reference | 6 | 0 |
| FILETIME | 8 | one register | 5 | 1 |
| IID | 16 | by reference | 5 | 0 |
| LARGE_INTEGER | 8 | one register | 4 | 1 |
| SIZEL | 8 | one register | 4 | 0 |
| RECT | 16 | by reference | 2 | 0 |
| WINBIO_IDENTITY | 76 | by reference | 2 | 0 |
| CLS_LSN | 8 | one register | 2 | 3 |
| … | | | | |

**Sixty per cent of what is left needs no layout at all.** A `POINT` is two
32-bit fields in one qword, and this target's convention already passes one
value per register — `windows-target.md`'s reason for a table being one
register. What is missing is not a representation but a way to **build the
word**: two `int`s packed into one. That is the `(array A B)` product this
language already has, with a declared element range deciding the field widths,
and `POINT` as the first program.

**Forty per cent needs the heterogeneous product's layout**, which is
products.md §7's deferral and what `GUID` wants — 18 arguments, and the largest
single entry in this bucket.

**And the hidden-pointer return convention is worth exactly one entry point.**
Of the 8 refusals in result position, 7 are register-class, returned in RAX,
which is what `values.md`'s two-result form already emits there. The one that is
not is `D3DCOLORVALUE`, 16 bytes, in `d3dtypes.h`. **A convention nobody has
built, costing one name** — measured before being built, which is the point of
doing the research first.

## 6. The ceiling nobody had measured: the link line

win32enum-2026-09-11 named this and said the survey *"cannot map a header to its
DLL without reading the SDK's import libraries"*. It can, and it needs no
toolchain — a COFF archive carries its own symbol table:

> 8 bytes `!<arch>\n`, then members with a 60-byte header, and the FIRST member
> is named `/` and holds a big-endian count, that many offsets, then that many
> NUL-terminated symbol names.

461 import libraries, 328,290 symbols, read off the same SDK the headers come
from — so this is a measurement rather than a list somebody maintains.

```
LINKING, of the 4093 callable (461 import libraries read)
  in a library the build links:     666   16.3%
  in another import library:       3160   77.2%   [emitter: the link line is hard-coded]
  in no import library at all:      267    6.5%   [a header with no static import]
```

| library | callable names |
|---|---:|
| user32 | 504 |
| onecore | 281 |
| gdi32 | 220 |
| msi | 209 |
| clusapi | 206 |
| mincore | 197 |
| … | 145 in total |

*(Two rows of that table are wrong, corrected the next day —
[linkline-2026-09-13](linkline-2026-09-13.md). It named each missing
library by the SHORTEST name exporting the symbol, which picks `onecore` (7
letters) over `kernel32` (8). `onecore` and `mincore` bind 129 and 209 DLLs, and
list most of kernel32's and user32's names beside those libraries; ranked by the
library a declaration now names, neither appears, `setupapi` is 160 and
`advapi32` 147. The totals above are unaffected.)*

**So "callable by a program" is 4,093 as a property of the declarations and 666
as a property of a program.** `asmBuildBat` links `kernel32 msvcrt ucrt
vcruntime legacy_stdio_definitions` and the list is a constant in
`emit/target.go`. That is the same sentence win32-2026-09-08 wrote about a name
counted declarable and never emitted: **a name this survey counts and a program
cannot link is a claim.**

It is also a small fix with a shape the project already has. Every generated
prim carries `(import "Name")`, so the emitter knows which entry points a
program calls; what it does not know is which library each lives in, and that is
a target declaration — one line per library, or one `(library …)` field beside
`(import …)`. Linking all 461 would be the wrong answer. **Not built here**, and
it is the largest single move available on this target: 666 → about 3,826.
*(Built the next day, and 3,826 was **3,593** — 233 of those names are bound
to two different DLLs by different libraries, reachable only through an API set,
or bound to no DLL at all, and a declaration that cannot name its library
honestly names none: linkline-2026-09-13.)*

## 7. Cost

| | |
|---|---|
| compiler change | **none** — nothing under `core/`, `emit/`, `targets/`, `lib/` or `examples/` |
| emitted files | unchanged by construction, and checked: the differential suite is green on four targets |
| generated Win32 declarations | 9,427 → **10,479**, of which **585 had a parameter retyped from a word to a pointer** |
| tooling suite | green, 10 acceptance programs, new pins |
| new files | `win32-sizes.txt` (the host's answer, 24 aggregates), `acceptance/pointer-param.oro` |

Four new affordances in the tool, each of which exists because a number here was
wrong without anything noticing:

- **`-sig NAME`** prints one entry point as the tool sees it. Every figure in
  this document is a sum over 11,575 signatures, and a sum cannot say that a
  signature was misread.
- **`-sizes FILE`** asks MSVC for the layouts, so the ABI class is the host's
  answer.
- **`-check-enums`** asks MSVC to confirm every enum a declaration relies on,
  with a control that must be refused.
- **a parser-residue row**: 5 parameter declarators are still not fully read
  (an inline function pointer, `int (*cb)(void)`, whose name sits inside
  parentheses). Counted rather than assumed absent — the SDK writes callbacks as
  typedefs, and if that stops being true the row says so.

And one test: the size table must cover every aggregate the report classifies,
so a new SDK adding one is a failing test rather than a silent *"size not
measured"*.

**The honest accounting against this repository's own standing criticism**:
`win32.go` grows **922 lines** and the tooling layer reaches **6,383**, against
**34 lines of Oroboros** — one acceptance program. assessment-2026-09-11's
verdict was *1,074 lines of compiler against six lines of programs*, and this
round is worse on that ratio, not better. Two things in mitigation and neither
cancels it: the tooling has tests now, so this growth is tested growth; and
about a third of what was added asks the HOST a question rather than answering
it here, which is the only kind of survey code that cannot be wrong about the
host. The balance is still the thing to watch, and the next item — the link
line — is a target declaration and a compiler change rather than more tooling.

## 8. What this leaves

**Item 4's premise is gone.** Struct by value is 1.2% and the largest remaining
refusals are *unresolved typedef* at 4.6% — this tool's own residue, not a
language limit — and *function pointer* at 3.0%, which callbacks.md tier 3
argued and declined. Solving struct by value completely would buy 140 names of
11,575, and 84 of those want one packed word rather than a layout.

**The two things this makes next, in order.**

1. **The link line should be computed from the declarations.** It is the
   difference between 666 and 3,826 callable names, it is a target declaration
   rather than a compiler rule, and it is the only place on this host where the
   gap between what we declare and what a program can do is a factor of six.

2. **A generated declaration is a claim, and 30,940 JVM, 10,479 Win32, 4,773 Go
   and 2,326 JavaScript of them have been checked by ten acceptance programs.**
   585 were wrong here and nothing found them for six days. coercion-2026-09-09
   host-filtered 1,651 candidate edges down to 1,466 by asking `go build`; this
   round asked MSVC twice, for 24 sizes and 407 enums, and both times the host
   had the answer immediately. **The checkable half of a Win32 template is its
   TYPES**, and a probe per declaration — `void check_X(void){ (void)sizeof(&X);
   }`, or a call with typed arguments that is compiled and not run — is the same
   trick a third time.

**And a measurement note.** Three surveys have now had a number corrected
because the first one described the measurer — methods at 0%, the typedef
residue at 23.5%, `*File` collapsed across packages, the enum share, and this.
The pattern is sharper than any instance: **when a measurement of what a
language can do is written by the same hands as the language, its first number
is about the tool.** What is new here is the form of the answer. Two of the four
new affordances are *ask the host*, and the host answered in under a minute in
both cases — which is cheaper than being careful.
