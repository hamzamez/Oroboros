# Research: pointers, or what a host does with an address

2026-10-28. **For hamza's decision**: P1–P6 in §8. The third of the seven walls
[llvmsurvey-2026-10-27](../gauntlet/results/llvmsurvey-2026-10-27.md) found (§4.6), and the 10-24 plan's 6b,
step 3. hamza: "go ahead with the research on wall 3, pointers". Like
[bits-research.md](bits-research.md) and [widths-research.md](widths-research.md), it explains the
issue, gives the algebra, and says what each target does, what the type system does, and what is
recommended, with examples. The LLVM claims in §3.5 were measured on LLVM 23.1.4; the Win32 census in §6
was read from TinyRetroPad's source (data, never built or run).

**The short version:**
- **A pointer is never a value of the language, and nothing here makes it one.** A host's pointer
  parameter always means something the language already has: a value passed in, a result handed back,
  a buffer borrowed, or a cell. The pointer is how the host *receives* that thing. It is never something
  the program computes.
- **The algebra.** An LLVM or C pointer is a pair (provenance, offset): an allocation, and a position in
  it (CompCert's block–offset memory; PNVI in C; LLVM's provenance). A language table is a function on
  [0, len), which is exactly an allocation and its extent. A segment `b[lo:hi]` is (b, lo) with a
  length. So every pointer the printer needs is one of these.
- **Theorem P (§2.3).** Take a host call that keeps no copy of the address once it returns, and whose
  memory behind the pointer nothing else it can reach overlaps. Observably, that call is a function from
  the memory's value to its new value. So by the state-passing translation:
  - `_In_ T*` is a value;
  - `_Out_ T*` is a result;
  - `_Inout_ T*` is both.

  What breaks the theorem is **capture**: a host that keeps the address past the call. That is the one
  thing here the language does not have, and it is the one real wall.
- **Measured on LLVM 23.1.4** (§3.5):
  - The theorem's premises are attributes LLVM already has: `captures(none)`, `noalias`, `readonly`,
    `writeonly`, `initializes`, `dereferenceable`.
  - Stating them made `-O2` fold a cell's value across the host call, and across a second, unrelated
    call.
  - The same pointer passed as `SendMessage`'s integer folded nothing.
  - A pointer stored inside a record also folded nothing, because no LLVM attribute speaks for a
    record's pointer fields. For those, the guarantee is the language's alone.
- **TinyRetroPad's 67 Win32 functions** (§6), each classified by its most demanding pointer parameter:

  | kind | functions | what it needs |
  |---:|---|---|
  | none | 29 | handles and integers, or NULL |
  | in | 16 | the host reads through a pointer |
  | out | 9 | the host writes a result through one |
  | in-out record | 5 | the common dialogs |
  | retained | 2 | `FindTextA` and `ReplaceTextA` keep the pointer past the call |
  | code pointer | 2 | the window procedure, step 6d |
  | returned memory | 2 | the host hands back memory |
  | message family | 2 | `SendMessageA`, 43 calls, and `DefWindowProcA` |

  Of the 67, **63 need no new construct**. Two need callbacks, which 6d already owns, and two (the Find
  dialog's) are the wall.
- **Recommended** (§8):
  1. No pointer values, ever.
  2. A pointer parameter is a mode on its binder, and its LLVM attributes are the mode's premises
     stated.
  3. An opaque handle is a host type with equality and a niche.
  4. A retained pointer is the wall, designed with 6d's window procedure.
  5. A message family is one declaration per message.
  6. Memory the host returns is decoded at the boundary.

---

## 1. The issue

### 1.1 What LLVM offers

Since LLVM 15 every pointer has one type, `ptr`, with no pointee type. What a pointer can do is the
instructions and intrinsics that take one:

| group | members | the survey's verdict |
|---|---|---|
| memory | `alloca`, `load`, `store`, `getelementptr`, `memcpy`, `memset`, `memmove` | the printer's: a table is an `alloca` or a heap block, `t[i]` a `gep inbounds` and a `load` |
| pointers as values | `ptrtoint`, `inttoptr`, `ptrtoaddr`, `addrspacecast`; `ptrmask`, `objectsize`, `load.relative` | the wall (§4.6) |
| atomics, thread-locals | `cmpxchg`, `atomicrmw`, `fence`, `threadlocal.address` … | concurrency's (ADR 0081) |
| attributes on a pointer parameter | `captures(…)`, `noalias`, `readonly`, `writeonly`, `readnone`, `initializes`, `dereferenceable(n)`, `nonnull`, `align`, `dead_on_unwind`, `writable` | not in the survey's count: they are claims about a call, not operations (§3.5) |

Of Σ_LLVM's instances, **1,225 (7.5%) take or give a pointer** (the survey's §2). Almost all of them
are the memory group, applied to a buffer, which the language already has as a table. Seven operations
treat an address *as a value*. The survey recommended offering none of them, and this document derives
that recommendation.

### 1.2 What Win32 asks

Win32 is C. Its API passes nearly everything through pointers:

- strings (`LPCSTR`);
- records (`LPRECT`, `LPOPENFILENAMEA`);
- out-parameters (`LPDWORD lpNumberOfBytesRead`);
- buffers (`LPVOID lpBuffer` with a length);
- opaque handles (`HWND`, `HANDLE`), which are pointers C's type system hides;
- code (`WNDPROC`);
- integers that carry pointers (`LPARAM`).

Most of the API states what it does with each pointer in SAL, Microsoft's source annotation language:
`_In_`, `_Out_`, `_Inout_`, `_In_reads_bytes_(n)`, `_Out_writes_to_(n, *m)`, `_Outptr_`, and an `_opt_`
form of each. outparams-2026-10-08 measured **3,461 of 11,575 flat entry points (29.9%) with at least
one SAL out-parameter**.

**SAL states direction and extent, never capture.** No SAL annotation says "the callee keeps this
pointer". That fact is in the documentation alone (`FindText`: "the FINDREPLACE structure must remain
valid until the dialog box is destroyed").

### 1.3 What the language has today

There is **no pointer type**, and no address. What exists:

- **A cell a host writes** (ADR 0048): `cell(T)` in a declaration, and `out S` at a call for a fresh
  one (G1). By outparams-research §1.1, an out-parameter is a result in disguise,
  f : X × Ref T → R ≅ X → T × R, when nothing else can see the cell during the call.
- **A borrowed buffer** (ADR 0070): `b ∈ Array(V) linear`, passed by reference. It is written in place,
  or the host hands its storage back as `b'`.
- **A pure host call observing a live buffer** (ADR 0069). **A segment** `b[lo:hi]`, its table's
  function restricted (ADR 0074).
- **Host types**: Go's `set File = host "*os.File"` already is a pointer, held opaquely. **A niche**:
  `repr error niche host expr "%s == nil"`, the absent point inside a host type's own values (ADR 0040).
- **On the windows target**, a type named `ptr`, with two out-parameters hidden in templates. `ReadFile`
  writes its count through `lea r9, __read`, a global cell. Its own comment says this "would NOT work
  under threads, which is stated rather than fixed". This document fixes it (P2).

What the language lacks is a stated rule saying which pointer parameters the existing constructs cover,
and what to do with the rest.

---

## 2. The algebra

### 2.1 A pointer is a provenance and an offset

The memory models that verified compilers and the C standard adopted agree on one point. **Memory is a
finite map from allocations (blocks) to arrays**, and a pointer is a block together with an offset:

- **CompCert**: Leroy and Blazy, "Formal verification of a C-like memory model and its uses for
  verifying program transformations", JAR 2008; Leroy et al., "The CompCert memory model, version 2",
  2012.
- **PNVI**: Memarian et al., "Exploring C semantics and pointer provenance", POPL 2019, adopted as ISO
  TS 6010's provenance-not-via-integers model.
- **LLVM**: the LangRef's "pointer aliasing rules" say a pointer may access only the memory of the
  allocation it is *based on*. Lee et al., "Reconciling high-level optimizations and low-level code in
  LLVM", OOPSLA 2018, give it a formal model.

```
Mem  = Block ⇀ Array(Byte)        ptr = Block × Offset
gep (b, o) k     = (b, o + k)            inbounds iff 0 ≤ o + k ≤ size(b)
load (b, o)      = Mem(b)[o]             defined iff (b, o) is inbounds and b is live
```

**A pointer's integer forgets the block.** `ptrtoint (b, o)` gives base(b) + o:

- base(b) is the allocator's choice, and it changes from run to run (ASLR);
- `inttoptr` must invent a block for an integer, which is why every model above has an "exposed
  provenance" rule (Rust's strict provenance API, 2024, names the two directions:
  `expose_provenance` and `with_exposed_provenance`).

So an address as an integer is nondeterministic, and to make an address from an integer is to guess.

### 2.2 The language already has the block and the offset

A table is a function on [0, len). As a memory, it is **one block whose extent is its length**:

| C or LLVM | the language |
|---|---|
| a block `b` of n elements | a table `t ∈ Array(V)`, `len(t) = n` |
| a pointer (b, o), with `size(b) − o` known | a segment `t[o:]` (ADR 0074) |
| a fat pointer (b, o, k): a Go slice, an LLVM `ptr` plus an `i64` | `t[o:o+k]` |
| `gep inbounds` | the obligation `0 ≤ o ≤ len(t)`, discharged by the decision |
| `load (b, o + i)` | `t[o + i]`, an application |
| `objectsize` | `len` |
| a scalar cell | a table of length one (outparams §1.1) |

Every pointer to memory the program made is one of these. **The provenance is the table's name**, and
the language never lets it be forgotten, because nothing turns a table into an integer.

### 2.3 Theorem P: a borrow that is not kept is a value passed and a result returned

Let `f` be a host function called as `f(x, p)`, where p points to an extent E (a table, a segment or a
cell) whose contents before the call are v. Suppose:

- **(C) no capture**: no copy of p, or of any pointer based on it, survives the call;
- **(S) separation**: during the call, nothing else the callee can reach overlaps E. That covers its
  other arguments, the globals it touches, and other threads. Formally, E is a separate conjunct,
  pre ∗ E ↦ v.

Then the call is observably a function

```
f̂ : X × V → V × R          f(x, p) with E ↦ v   ≡   (v', r) = f̂(x, v);  E ↦ v'
```

and its mode restricts f̂:

| SAL | LLVM | f̂ | the language |
|---|---|---|---|
| `_In_` | `readonly` | X × V → R (v' = v) | the value is an argument |
| `_Out_`, every element written | `writeonly initializes((0, n))` | X → V × R (v is not read) | the value is a result |
| `_Inout_` | neither | X × V → V × R | an argument and a result: `inout`, or `linear` on a table |

*Proof.* Separation logic's frame rule (O'Hearn, Reynolds and Yang, "Local reasoning about programs
that alter data structures", CSL 2001; Reynolds, LICS 2002) gives

```
{P} c {Q}  ⊢  {P ∗ R} c {Q ∗ R}
```

so the call changes nothing outside E and its own state. By (S), the callee's effect on E is a
function of v and x alone, since nothing else writes E during the call. By (C), no later operation
reaches E through the callee, so E's later contents are determined by the program's own operations on
it. Copy-in at the call, copy-out at its return then agrees with by-reference passing: this is Hoare's
condition for call-by-reference and call-by-value-result to coincide ("Procedures and parameters: an
axiomatic approach", 1971), and Reynolds' syntactic control of interference (1978).

The same translation, from capabilities over locations to state passing, is Charguéraud and Pottier's,
"Functional translation of a calculus of capabilities", ICFP 2008. In this language it is the state
translation S (storeorder-research.md), which already gives `linear` its meaning. ∎

**The language makes (S) true by construction.** Values are immutable and a buffer is linear
(ADR 0018, ADR 0052's R0–R4). So:

- an extent the program passes is unaliased;
- the same buffer passed twice to one call is refused by linearity.

**(C) is the host's promise.** It is the one premise the program cannot check.

### 2.4 What breaks the theorem: capture

If the host keeps p, then E is **a location shared across time** between the program and the host,
which by definition makes it aliased. A kept pointer is not a value and not a result, and it takes
three shapes:

| shape | lifetime of E | examples |
|---|---|---|
| **retained**: the program lends to the host until a later event | until the host lets go | `FindTextA(&fr)`, released at `FR_DIALOGTERM`; Go's `flag.IntVar(&n, …)`, written at `Parse`; an `OVERLAPPED` until the I/O completes |
| **lent**: the host lends to the program until a later call | until the host's next write | `bufio.Reader.ReadSlice`, `Scanner.Bytes`; `GlobalLock` until `GlobalUnlock`; `GetCommandLineA`, for the whole process |
| **code**: the host keeps a function | until unregistered | `WNDCLASS.lpfnWndProc`, `DLGPROC` |

The first two are one question asked in opposite directions: **a region whose lifetime spans several
calls**. That question has a literature:

- regions: Tofte and Talpin, "Region-based memory management", Inf. & Comp. 1997; Cyclone's regions,
  Grossman et al., PLDI 2002;
- lifetimes: Rust's, `'static` for `GetCommandLineA`;
- epochs: borrow-research.md's observer, Theorem L, `lent`, not built.

The third is the window procedure, which step 6d already owns.

### 2.5 An opaque handle is a name

`HWND`, `HMENU`, `HDC`, `HANDLE` and `HMODULE` are pointers in C's type, but a program may do nothing
with one except compare it and pass it back. That is the algebra of **pure names**: a set with equality
and nothing else, made fresh by its allocator (Pitts and Stark, "Observable properties of higher order
functions that dynamically create local names", 1993). Its failure value is a **niche** (ADR 0040), a
point of the host type that is no handle:

- NULL for `CreateWindowExA`, `LoadLibraryA` and `GetMenu`;
- `INVALID_HANDLE_VALUE` (−1) for `CreateFileA`.

The niche is a property of each *function*, not of the type `HANDLE`.

In `STRICT` mode, `windows.h`'s `DECLARE_HANDLE` makes each handle kind a distinct C type. That is the
host's own distinction, a newtype over `void *`.

### 2.6 Optional pointers

- **`_In_opt_ T*`** is `Option(T)` passed by reference, with NULL the niche for `none`.
- **`_Out_opt_ T*` passed NULL** is the projection "this result is not wanted": f̂ composed with
  dropping a component.

Each time TinyRetroPad passes a NULL, the program wants the projection. Today's templates already bake
such constants in:

- `GetFileSize(h, NULL)`: the high word is not wanted;
- `GetDlgItemInt(…, NULL, FALSE)`: no success flag;
- `TrackPopupMenu(…, NULL)`: a reserved rectangle;
- `GetModuleHandleA(NULL)`: the module itself.

### 2.7 An integer carrying a pointer: a dependent family

`SendMessageA(HWND h, UINT m, WPARAM w, LPARAM l)` has one C type. What it does depends on m:

| m | what `l` is |
|---|---|
| `WM_SETTEXT` | an in string |
| `WM_GETTEXT` | an out buffer, w its length |
| `EM_EXGETSEL` | an out `CHARRANGE` |
| `EM_EXSETSEL` | an in `CHARRANGE` |
| `EM_LINEINDEX` | nothing (w is a line number) |

Its true type is a **dependent function**, Π(m ∈ Msg). τ(m), in Martin-Löf's sense. C erases it into
integers, which is why `LPARAM` is pointer-sized. The faithful form restores τ(m), one function per
message, and Win32 itself ships it: `windowsx.h`'s *message crackers* and control macros:

- `Edit_ReplaceSel(h, s)` is `SendMessage(h, EM_REPLACESEL, 0, (LPARAM)s)`;
- `Edit_GetText(h, b, n)` is `GetWindowText`.

The `ptrtoint` (C's `(LPARAM)` cast) is then **in the printer's template, under the host's ABI**, and
never reaches the program. That costs the same as the hand-written C: LLVM sees an integer escape and
folds nothing (§3.5).

---

## 3. The targets

The same three calls on each host:

- a record the host fills (`GetClientRect`);
- a buffer the host fills (`ReadFile`);
- a pointer the host keeps (the Find dialog).

### 3.1 Win32 in C, the reference

```c
RECT rc;   GetClientRect(hwnd, &rc);                       // _Out_ LPRECT
DWORD got; ReadFile(h, buf, n, &got, NULL);                 // _Out_writes_bytes_to_(n, *got), _Out_opt_
static FINDREPLACEA fr; static char what[80];               // must outlive the call
fr.lpstrFindWhat = what; fr.wFindWhatLen = 80;
HWND dlg = FindTextA(&fr);                                  // returns at once; the dialog keeps &fr
// later, in the window procedure:
if (msg == uFindMsg) { FINDREPLACEA *p = (FINDREPLACEA *)lParam; /* read p->Flags, what */ }
```

### 3.2 Go

Go has pointers but no arithmetic, and the garbage collector traces them, so provenance is never lost
in safe code. `golang.org/x/sys/windows` calls Win32 the way C does. Its unsafe-pointer rule (4) is
Theorem P as a runtime rule: a pointer converted to `uintptr` *in the call expression itself* stays
alive and does not move **for that call only**.

```go
var mode uint32
err := windows.GetConsoleMode(h, &mode)            // out: &mode lives for the call
r, _, _ := syscall.SyscallN(proc, uintptr(unsafe.Pointer(&rc)))   // rule (4): only inside the call
n, err := f.Read(buf[lo:hi])                        // a slice: (block, offset, length)
```

A kept pointer, as in `flag.IntVar(&n, …)`, makes Go's escape analysis move `n` to the heap. That is
capture, measured by the compiler.

### 3.3 Java and Android

Java has references and no addresses, so an out record is an object the caller allocates and the
callee fills. A buffer is an array with an offset, the segment.

```java
Rect r = new Rect();  view.getDrawingRect(r);          // Android: out Rect
paint.getTextBounds(s, 0, s.length(), r);              // the same
int n = in.read(buf, off, len);                        // a segment
```

JNI's `GetPrimitiveArrayCritical` lends a Java array's address to native code until
`ReleasePrimitiveArrayCritical`: an epoch, and its rule that no JNI call may intervene is (S).

### 3.4 JavaScript

JavaScript has no pointers. A buffer is a typed array, a segment of an `ArrayBuffer` with its own
offset and length. The Web's out-parameters fill one:

```js
const { read, written } = new TextEncoder().encodeInto(s, u8.subarray(lo, hi));   // out segment
crypto.getRandomValues(u8);                                                       // out
worker.postMessage(buf, [buf]);   // a transfer: buf is detached here, ownership moved (linear)
```

**WebAssembly's** pointers are `i32` offsets into one linear memory, integers with the provenance erased.
That is why a wasm module is memory-unsafe inside itself, and also why LLVM's wasm triple prints `ptr` as
`i32`.

### 3.5 LLVM, measured

LLVM states Theorem P's premises as parameter attributes:

- `captures(none)` is (C);
- `noalias` is (S) for the pointer's own accesses;
- `readonly`, `writeonly` and `initializes((0, n))` are the mode;
- `dereferenceable(n)` says the extent has at least n bytes, and `nonnull` and `align` follow.

**What they buy**, on LLVM 23.1.4 with `opt -O2`. A cell is stored 5, passed to a host function, then
read:

| the host's declaration | after the call | after an unrelated call too |
|---|---|---|
| `declare void @host(ptr)` | `load`: not folded | not folded |
| `declare void @host(ptr captures(none) readonly)` | **`ret i32 5`** | **`ret i32 5`** |
| `declare void @host(ptr captures(none) writeonly initializes((0, 4)))` | the store of 5 is **deleted** as dead | |
| the pointer `ptrtoint`ed into `SendMessageA`'s `i64` | not folded | |
| a record `{ i32, ptr }` whose field points to the cell, record passed `captures(none)` | | not folded |

The second row folds even across an unrelated call, because the cell has not escaped: no other function
can hold its address. That is (C) used. The last row shows the limit. Storing a pointer into a record
counts as a capture to LLVM, and **no attribute says "the pointees of this record's fields are not
kept"**. So a borrow nested in a record, such as `OPENFILENAME.lpstrFile`, is unaliased only in the
language's own proof, and LLVM is conservative there. That is correct and costs little: those are
dialog calls.

**A wrong attribute gives a wrong answer.** If the host declared `readonly` writes the cell, the
program returns the stale 5. So on a host's declaration these attributes are **claims**, exactly as
`pure` is today. They are written by hand from SAL and the documentation, and inherit ADR 0022's trust.

### 3.6 Summary

| | addresses as values | a borrow for one call | a kept pointer |
|---|---|---|---|
| C, Win32 | yes | `T*`, SAL's modes | anything |
| Go | `uintptr` via `unsafe` only | `&x`, slices; rule (4) | escape to the heap |
| Java, ART | no | an out object, an array and offset | a field holding the object |
| JavaScript | no (wasm: `i32` offsets) | a typed array, `subarray` | the object itself |
| LLVM | `ptrtoint` and friends | `ptr` with the attributes of §3.5 | `captures(…)` other than none |

**Only the middle column exists on all four**, so it is the only portable meaning a pointer parameter
can have. That is the spec-first test (CLAUDE.md): an address as a value means nothing independent of a
target. JavaScript and Java have none, and where one exists its value changes from run to run.

---

## 4. The type system

**There is no sort of pointers.** A type is a set of values (ADR 0049), and an address is not a value
with a meaning in ℤ: its integer is the allocator's choice. Offering one even at Tier 2 would make the
core a superset of one host, which CLAUDE.md forbids. It would also break the invariant that every
proof rests on: a value cannot be aliased (ADR 0018).

**A pointer parameter is a mode on a binder**, and J's grammar already has the words,
`mode ::= "linear" | "inout" | "out"` (j.md §2.4). Theorem P maps SAL onto them:

| SAL on the host's parameter | the declaration's header | the printer passes |
|---|---|---|
| `_In_ const T*`, T a value or record | `x ∈ T` | the address of a temporary holding x, or of x's own storage |
| `_In_reads_(n) const T*` | `b ∈ Array(V)`, `where len(b) >= n` | the table's base; a pure call observes it live (ADR 0069) |
| `_Out_ T*`, T fixed | a **result** `(r ∈ T)` | the address of a fresh cell, read after the call |
| `_Out_ T*` where the caller chooses T | `a ∈ Array(cell(V))`, `out S` at the call | ADR 0048, as `fmt.Sscan` |
| `_Out_writes_(n)`, `_Inout_updates_(n)` | `b ∈ Array(V) linear`, `b'` if the host hands it back | ADR 0070 |
| `_Inout_ T*`, T a record | `x ∈ T inout`, `x'` its value on exit | value in, value out |
| `_In_opt_` | `x ∈ Option(T)` | NULL for none |
| `_Out_opt_` the program does not want | nothing: a constant NULL in the template | the projection of §2.6 |

**Each mode's attributes are facts the language already has**, so the printer states them:

| attribute | comes from |
|---|---|
| `captures(none)` | every mode, by definition: a mode is a borrow for one call |
| `noalias` | linearity (R0–R4): the extent is the call's alone |
| `readonly`, `writeonly` | the mode |
| `initializes((0, n))` | an out result whose every element the host writes |
| `dereferenceable(n)` | `len(b) >= n`, proven by the decision, or the record's size |
| `nonnull` | always: the language has no null; an `Option` is the one way to pass one |
| `align` | the element's representation (ADR 0083) |

On the language's own functions these are **proofs**. On a host's declaration they are **claims**,
checked against SAL where SAL says them (the Win32 survey already parses SAL) and trusted where it is
silent, which is capture. A planted fault is due for each rule the printer uses to state one: a
`dereferenceable` larger than the proven length, and `noalias` on a buffer passed twice.

---

## 5. Examples

In J, with the LLVM the printer would emit for 64-bit Windows, whose calling convention is the
default. On 32-bit, ADR 0080's other triple, each `declare` adds `x86_stdcallcc`. Templates for `ir/llvm`
are not designed here. A record is shown as a tuple until 6d decides records.

**An out record is a result.** `GetClientRect(HWND, _Out_ LPRECT)`:

```
fn GetClientRect(h ∈ HWND) (r ∈ result((Int32, Int32, Int32, Int32), win.error))
```

```llvm
declare dllimport i32 @GetClientRect(ptr, ptr captures(none) writeonly initializes((0, 16)) dereferenceable(16))
  %rc = alloca { i32, i32, i32, i32 }
  %ok = call i32 @GetClientRect(ptr %h, ptr %rc)        ; then four loads, or the BOOL's error
```

**A buffer is a borrow, and the count is a result.** `ReadFile`'s `lpNumberOfBytesRead` becomes a
result. The global `__read` cell of today's windows target goes, and with it the threading caveat:

```
fn ReadFile(h ∈ HANDLE, b ∈ Array(0..255) linear, n ∈ 0..len(b))
    (got ∈ 0..n, e ∈ result((), win.error))
```

```llvm
declare dllimport i32 @ReadFile(ptr, ptr captures(none) writeonly, i32,
                                ptr captures(none) writeonly initializes((0, 4)), ptr)
  %got = alloca i32
  %ok = call i32 @ReadFile(ptr %h, ptr %b, i32 %n, ptr %got, ptr null)
```

**A handle with a function's own niche.** `CreateFileA` fails with `INVALID_HANDLE_VALUE`, and
`CreateWindowExA` with NULL:

```
set HANDLE = host "HANDLE"
fn CreateFileA(path ∈ win.cstr, access ∈ Int32, …) (h ∈ result(HANDLE, win.error))
    // the result's niche: h == INVALID_HANDLE_VALUE, then GetLastError
```

**A message is a function.** Three of TinyRetroPad's 43 `SendMessageA` calls, as the crackers would
declare them:

```
fn Edit_ExGetSel(h ∈ HWND) (lo ∈ Int32, hi ∈ Int32)          // EM_EXGETSEL, lParam an _Out_ CHARRANGE
fn Edit_ReplaceSel(h ∈ HWND, s ∈ win.cstr, undo ∈ Bool) ()    // EM_REPLACESEL, lParam an _In_ string
fn Window_GetText(h ∈ HWND, b ∈ Array(0..255) linear) (n ∈ 0..len(b)-1)   // WM_GETTEXT
```

```llvm
  %cr = alloca { i32, i32 }
  %l  = ptrtoint ptr %cr to i64                 ; the host's ABI, in the template; never the program's
  call i64 @SendMessageA(ptr %h, i32 1076, i64 0, i64 %l)   ; EM_EXGETSEL = WM_USER + 52
```

**An in-out record carrying nested borrows.** `GetOpenFileNameA(_Inout_ LPOPENFILENAMEA)`. The record
holds the filter string (in), the file name buffer (in and out), and flags (in and out). Each pointer
field is a borrow for the same call, so Theorem P covers the record with every field's mode:

```
fn GetOpenFileNameA(ofn ∈ OpenFileName inout, file ∈ Array(0..255) linear) (ok ∈ Bool)
```

LLVM cannot see the inner borrows (§3.5's last row), and nothing is lost but a fold across a dialog.

**The wall.** `FindTextA` returns at once, and the dialog writes `fr` and `what` whenever the user
types, between messages, until `FR_DIALOGTERM`. No header for it says the truth with the modes above:
`fr` is neither an argument nor a result of this call. P4 is about it.

**Host memory returned.** `GetCommandLineA()` returns a pointer into the process's memory, valid for the
whole process. It is the host's string, entered by a total decode, exactly as Go's `bytestring` is
entered by `go.text` (ADR 0030):

```
fn GetCommandLineA() (win.cstr) pure
```

---

## 6. TinyRetroPad, classified

67 imported functions, 2,686 lines of 32-bit MASM, read as data. Each function is classified by its
most demanding pointer parameter, from the SDK's SAL and documentation:

| kind | n | functions | the language |
|---|---:|---|---|
| no pointer the program supplies | 29 | `CheckMenuItem` `CloseHandle` `CreateMenu` `CreatePopupMenu` `DeleteDC` `DestroyMenu` `DestroyWindow` `EndDialog` `EndDoc` `EndPage` `ExitProcess` `GetDeviceCaps` `GetKeyState` `GetMenu` `GetMessagePos` `GetSysColorBrush` `GetSystemMenu` `GlobalFree` `PostQuitMessage` `SetBkMode` `SetFocus` `SetMenu` `SetWindowPos` `ShowWindow` `StartPage`; NULL passed: `GetFileSize` `GetDlgItemInt` `TrackPopupMenu` `GetModuleHandleA` | handles (P3); NULLs are projections (§2.6) |
| in | 16 | `AppendMenuA` `CreateFileA` `CreateWindowExA` `DispatchMessageA` `TranslateMessage` `IsDialogMessageA` `EndPaint` `FillRect` `InvalidateRect` `LoadLibraryA` `MessageBoxA` `RegisterWindowMessageA` `SetWindowTextA` `ShellExecuteA` `StartDocA` `TextOutA` | values (P2) |
| out | 9 | `BeginPaint` `GetClientRect` `GetLocalTime` `GetMessageA` `GetDateFormatA` `GetTimeFormatA` `ReadFile` `WriteFile` (its count) `wsprintfA` (its buffer) | results and borrowed buffers (P2) |
| in-out record | 5 | `GetOpenFileNameA` `GetSaveFileNameA` `ChooseFontW` `PrintDlgA` `PageSetupDlgA` | `inout` records with nested borrows (P2; layout 6d) |
| retained | 2 | `FindTextA` `ReplaceTextA` | **the wall** (P4) |
| code pointer | 2 | `RegisterClassA` (`lpfnWndProc`), `DialogBoxIndirectParamA` (`DLGPROC`) | callbacks, 6d |
| returned memory | 2 | `GetCommandLineA`, `GlobalAlloc` | a decode; a buffer (P6) |
| message family | 2 | `SendMessageA` (43 calls, 26 messages), `DefWindowProcA` | one declaration per message (P5) |

**Three observations.**

1. **The only arithmetic on an address is a cursor.** The 83 `OFFSET`s are string literals, static
   buffers and records passed to a call, and the 81 `PTR`s are operand sizes and import slots. Where an
   address is incremented, a loop is walking a string, such as `ParseStartupFile`'s `inc esi` over the
   command line. That is indexing, `s[i]`, and no address outlives the loop.
2. **The retained pointers are read only inside the window procedure.** `OnFindReplaceMsg` reads
   `fr.Flags` when `uFindMsg` arrives, and the host writes `fr` only while it runs, which is only inside
   `GetMessageA`, `IsDialogMessageA` and `DispatchMessageA`. So the lent epoch is bounded by host
   calls, and its reads happen in a callback. That is the reason P4 belongs with 6d.
3. **`GlobalAlloc` makes two buffers the program owns.** Loading reads the file into one with
   `ReadFile`, hands it to `SetWindowTextA`, and frees it. Saving fills one with `WM_GETTEXT`, writes
   it with `WriteFile`, and frees it. A language buffer is that allocation, and the printer chooses
   `alloca`, `HeapAlloc` or `malloc` for it.

---

## 7. Candidates

**A. Pointers as a sort** (C, Zig, unsafe Rust, LLVM itself): offer `ptr` and the seven operations.
- It fails the spec-first test. JavaScript and Java have no meaning for it, and where it exists its
  value changes between runs, so it is observable and nondeterministic.
- It breaks ADR 0018's invariant that a value cannot be aliased. The state translation, β's
  substitution, linearity and every interval proof stand on that invariant.
- It makes the core a superset of one host.
- TinyRetroPad never needs it (§6, observation 1: its one address arithmetic is a cursor).

**B. References as values** (ML's `ref`, Java's objects): a value with identity, shared.
- It is portable: every host has an object.
- The logic would need separation logic to keep its proofs, with frame inference on every call
  (Berdine, Calcagno and O'Hearn, "Smallfoot", 2005), because any two references might alias.
- ADR 0018 rejected this when it chose linear buffers.
- It would be the right tool for the retained case, and is too much for the other 63 functions.

**C. Borrows: a mode on the binder** (Ada's parameter modes, SAL, Rust's `&` and `&mut` in argument
position, Wadler's `let!`).
- It adds no value, no sort, and no proof obligation the language lacks.
- Theorem P is its soundness, and linearity gives (S).
- It is what Go's rule (4), Java's out objects and JavaScript's typed arrays already are.
- **It covers 63 of TinyRetroPad's 67 functions.**

**D. For a kept pointer, choose with 6d:**
- **D1, a region**: the program declares a buffer at a scope enclosing every host call that may write
  it (the message loop), lends it to the host for that scope, and reads it only where the host cannot
  be running (Tofte–Talpin's `letregion`; Cyclone).
- **D2, an epoch**: borrow-research's Theorem L, mirrored. Every host call ends the epoch, so a read of
  the lent extent must happen before the next host call, and in a callback it is copied out at entry.
- **D3, a process** (ADR 0081): the Find dialog is a process, and each `uFindMsg` carries a copy of
  `fr`'s contents as a message. The kept memory is the printer's, never the program's.

D3 is the most algebraic, since a message is a value, and the most expensive. D1 is what the C program
does. The choice depends on the window procedure's design, so it belongs there.

---

## 8. Decisions for hamza

**P1. No pointer is a value, on any target.**
- `ptrtoint`, `inttoptr`, `ptrtoaddr`, `addrspacecast`, `ptrmask` and `load.relative` are not offered.
  `objectsize` is `len`.
- The printer may convert a pointer to an integer where the host's ABI types an argument as one
  (`LPARAM`, `UINT_PTR`). That conversion lives in a template, and the program never sees the integer.
- *Why*: §2.1 and §7A. It costs nothing TinyRetroPad, Go's standard library or the Web needs.

**P2. A host's pointer parameter is a mode on its binder, by Theorem P**:
- in → a value;
- out → a result in the declaration's header when the host fixes T, or `out S` at the call when the
  caller chooses T (ADR 0048);
- in-out → `inout` on a value or record, `linear` on a table (ADR 0070);
- `_In_opt_` → `Option(T)`, and an unwanted `_Out_opt_` → a NULL in the template.

On LLVM each mode prints its premises as attributes (§4): proofs on the language's functions, claims on
a host's, checked against SAL where it speaks. Today's hidden global cells (`__read`, `__written`)
become results. **Recommended.**

**P3. An opaque handle is a host type with equality, and its failure value is a niche.**
- One host type per kind `STRICT` `windows.h` distinguishes (`HWND`, `HMENU`, `HDC`, `HANDLE`,
  `HMODULE`, …), replacing the windows target's `ptr`.
- The niche goes on the type when every function shares it, and on the declaration's result when it
  varies (`CreateFileA`'s −1).
- *Why*: §2.5. A handle is a name, and its kinds are the host's own newtypes. **Recommended.**

**P4. A pointer the host keeps past the call is the wall. Nothing is built for it now, and it is
designed with 6d's window procedure.**
- Every retained pointer in TinyRetroPad is written only during host calls and read only in a callback
  (§6).
- The candidates are D1 (a region over the message loop), D2 (an epoch, borrow-research mirrored) and
  D3 (a process).
- **Recommended: decide it in 6d**, leaning to D1 for its cost and to D3 if 6d makes the window a
  process.

**P5. A family indexed by a constant is one declaration per member**: `SendMessage`'s messages,
`DeviceIoControl`'s codes, `ioctl`. This restores Π(m). τ(m), as `windowsx.h`'s crackers do, with no
language change. A message TinyRetroPad sends is declared when a program sends it. **Recommended.**

**P6. Memory the host returns is a host value, decoded at the boundary or owned as a buffer.**
- `GetCommandLineA` is the host's string, entered by a total decode (ADR 0030).
- A host allocation the program owns (`GlobalAlloc` with `GMEM_FIXED`) is not offered: it is a buffer,
  and the printer chooses its allocator.
- A host lending its own memory for an epoch (`GlobalLock`, `ReadSlice`) is borrow-research's `lent`,
  waiting on its trigger.
- **Recommended.**

---

## 9. Not here

- **Records' layout** (`RECT`, `OPENFILENAMEA`, `CHOOSEFONTW` with its `LOGFONTW`), which LLVM's data
  layout decides: 6d.
- **Callbacks** (`WNDPROC`, `DLGPROC`), and the pointers a callback *receives*, such as `WM_CREATE`'s
  `CREATESTRUCT*` and `WM_NOTIFY`'s `NMHDR*`. These are Theorem P with the roles swapped, the host
  lending to the program for one call: 6d.
- **`W` strings** (UTF-16, `ChooseFontW`), which trigger ADR 0083's re-declaration of representations as
  (H, d) data: a second string encoding.
- **Atomics, thread-locals and address spaces**: concurrency (ADR 0081) and GPU targets, none in the plan.

## 10. What it would cost

- **P1, P3, P5 and P6 add no construct.** They are rules for writing declarations, and the printer's
  part of `ir/llvm`.
- **P2 adds the attribute printing** and, on windows, turns two hidden cells into results.
- **Planted faults**: a `dereferenceable` larger than the proven length; `noalias` on a buffer passed
  twice to one call; and a host declaration whose SAL says `_Inout_` while it claims `readonly`, which
  the generator must refuse.
- **P4 is 6d's.**

None of it is built before hamza decides, and P2's attributes are built with `ir/llvm`, whose printer
they belong to.
