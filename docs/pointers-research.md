# Research: pointers, or what a host does with an address

2026-10-28. **Decided by [ADR 0084](decisions/0084-an-address-is-an-integer-and-ownership-is-a-view.md)**
(hamza, 2026-10-30): P1′ and P2–P6; Part II's direction; Part III's D, U3, the linear result and codecs.
Part I is the first research, Part II (2026-10-29) develops hamza's sketch (an address is an integer and
ownership a view) and replaces P1 with P1′ (§17), and Part III answers four questions on it (§23). The third of the seven walls
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

---

# Part II. hamza's sketch: an address is an integer, and ownership is a view

2026-10-29. Appended at hamza's request; Part I is not yet rewritten. hamza: "don't rewrite just yet.
You can elaborate and append … the restriction of our language are not on what we can do (we hope to do
anything c/c++ can, more than that everything a target offers we can do, even if c can't). the
restriction should on how. our way of doing should safer, cleaner, and simply better."

The sketch (2026-10-28, abridged):
- No pointers *in* the language, but pointers a target provides, and pointers as a library. "Addresses
  are just integers, operation on them can be handled by the language. Dereferencing is just an operator
  (function) provided by the target. The ability to modify the content of the memory being pointed at by
  the address is just a cell. Reading is just a read."
- Shen's globals, `(set *s* v)` and `(value *s*)`, are similar, except that Shen lets anyone set one.
  Here, "once the address is returned it would freeze, same rules as buffers and cells".
- So "an address will be owned by the scope that introduces it, passed and borrowed, or returned and
  frozen". That reaches down to an operating system: "the first process owns the whole address space, it
  passes addresses to another process which can modify it knowing nobody else can, when the process
  dies, they are freed, and returned to the owner."

**The short version of Part II:**
- **The sketch is right, once it is split in two.**
  - An address really is an integer: an offset into a region.
  - **Ownership cannot sit on the integer**, because an integer can be copied, recomputed or read from
    input. It sits on a second object, a **view**: a linear claim that this scope may read and write
    this range of this region.
  - That split is ATS's stateful views (Zhu and Xi 2005) and L³'s locations and capabilities (Ahmed,
    Fluet and Morrisett 2007). The operating system is seL4's untyped memory, and its processes are
    Singularity's exchange heap.
- **It fits this language exactly.** ADR 0049 already has two languages. The address belongs to the
  computation, as an integer that prints. **The view belongs to the logic, which never prints.** A buffer
  is already a view on a region the compiler made, and `i ∈ 0..len(b)-1` is already the type of an
  address into it.
- **The algebra** (§12):
  - Regions are names, and the addresses of one region are an affine space over ℤ.
  - Views form a separation algebra, split and joined by ranges with linear integer bounds, so the Omega
    test decides disjointness.
  - A view is exclusive, fractional, persistent (frozen), a device's, or uninitialized.
  - A view's contents have a type only through an abstraction function, which says exactly when type
    punning is sound.
  - **Theorem V** (§12.6): an exclusive view's contents are a value threaded through the program. So with
    views, memory *is* cells and buffers, and every proof the language has carries over.
- **The reach** (§14): every C and C++ pointer idiom surveyed in §14 has a form here. So do things C
  cannot express: an ArrayBuffer moved between workers, W^X as a freeze, offsets that survive `realloc`,
  CHERI's hardware capabilities.
- **The restriction is on *how***: every access lies inside a view the scope holds, proven or checked.
  That excludes by construction use-after-free, double free, leaks, a dangling stack address,
  out-of-bounds, uninitialized reads, type confusion, null dereference and data races (§15).
- **Measured**: a proven `noalias` is worth something (§16). On LLVM 23.1.4, a loop C writes without
  `restrict` carries a run-time overlap check and a second, scalar copy of itself: 96 lines. With
  `restrict` it has neither: 47 lines. In C `restrict` is an unchecked promise, and here it would be a
  theorem.
- **What it changes in Part I** (§17):
  - P1 becomes P1′.
  - P2's modes become views lent for one call, a special case.
  - P4's wall becomes a protocol of views: the Find dialog's struct is moved to the host and lent back to
    the window procedure for each message.

  Open questions are in §18.

---

## 11. Five corrections to the sketch

**11.1 You cannot own an integer.** To own a thing means nothing else holds it. An integer can be
duplicated and discarded at will: contraction and weakening hold on ℤ, as on every set of values. It can
also be recomputed: `a + 8 − 8`, a value read from a file, a guess. So "the scope that introduces an
address owns it" cannot be enforced on the address. It has to be enforced on an object the program
cannot copy or forge: ATS's view `T @ l`, L³'s `Cap ℓ τ`, seL4's capability slot, CHERI's tag bit. **The
address stays an integer anyone may hold, and the view is what is owned.** That split makes the sketch
work, and it is also what makes it better than C (§13.1).

**11.2 An address is an integer only relative to a region.** The addresses of one region form an
**affine space over ℤ** (a ℤ-torsor):
- an address plus an integer is an address;
- the difference of two addresses in the *same* region is an integer;
- the sum of two addresses means nothing, and neither does the difference of two in *different* regions.

C's rule on pointer subtraction (C17 §6.5.6: both pointers must point into the same array object) and
LLVM's provenance encode exactly this.

To turn an address into a bare integer is to choose an origin, and the allocator chooses a different one
every run, which is why `ptrtoint` gives a new answer each time. **One region has a canonical origin: the
whole address space, [0, 2⁶⁴), with origin 0.** For whoever holds *its* view, the operating system,
addresses are integers outright, and `ptrtoint` and `inttoptr` are the identity. So "addresses are just
integers" is true **in the root region**, and true relative to a region everywhere else.

**11.3 Memory someone else can write is not a cell.** A cell here is a fiction the compiler keeps. It has
no address, and it becomes SSA values and loop variables (ADR 0047). That is sound only because nobody
else writes it. Memory under an **exclusive** view can be a cell. Memory that a device, another process
or the host may write between two reads cannot. For such memory each read is an effect, never folded or
reordered: C's `volatile`, or concurrency's question when the other writer is a process.

**11.4 Returning a borrow is not freezing.** The sketch's "returned and frozen" names two operations:
- **When a borrow ends**, the exclusive view goes back to the lender, who can write again.
- **When the owner's scope ends**, the region either dies or, if its contents are the scope's value,
  **freezes**. A frozen view is read-only and freely copyable. It is ADR 0031's frozen buffer, and Iris's
  persistent points-to ℓ ↦□ v (Vindum and Birkedal, CPP 2021).

The sketch implies a third operation, **moving** a view out of a scope unfrozen (`malloc` returns
writable memory). It is not in the language today (§14.1).

**11.5 "When the process dies, its memory returns" is a typing rule only for a child inside its parent's
scope.**
- A child whose lifetime is one of the parent's scopes (fork–join, ADR 0081's fragment) is a **borrow**,
  and the view comes back when the scope ends.
- A child that outlives the parent's scope needs **revocation**, which is a run-time act: seL4's revoke
  over its capability derivation tree, Singularity's reclamation of a dead process's exchange-heap
  blocks, and in this language ADR 0081's supervisor handler.
- If the child **crashed**, the view comes back with **no facts about its contents**, which may be half
  written. That is Rust's poisoned mutex: the handler must re-establish its facts (validate,
  re-initialize) before trusting them.

---

## 12. The algebra

### 12.1 Regions and addresses

A **region** is a name: made fresh by its allocator, compared by equality, and nothing else (Pitts and
Stark 1993). It is §2.1's provenance. A region r has an extent [0, n_r) and an **element
representation**, bytes at the lowest level. An **address** in r is an integer a, an offset:

```
Addr_r = ℤ            an affine space: a + k ∈ Addr_r, and a − b ∈ ℤ for a, b ∈ Addr_r
valid_r = [0, n_r)    the addresses a load or store may use
```

A pointer of C's is the pair (r, a). The language never forms the pair as a value: r is always known
*statically*, as the name of the view the program holds, and only a is dynamic. That is how the language
keeps provenance without carrying it at run time. It is also why the printer always knows the base:
`getelementptr inbounds base, a`.

**The root region** R has n_R = 2⁶⁴ (2³² on a 32-bit triple) and a canonical origin. Its addresses are
machine addresses.
- A target that offers R (an OS, bare metal, an embedded board) offers `ptrtoint` and `inttoptr` as
  coordinates in it.
- A target that does not (JavaScript, the JVM, an application under Windows) has no R. That is a true
  "this target cannot do it" (Tier 2), and every other region stays portable.

### 12.2 Views form a separation algebra

A **view** is a claim on part of a region:

```
r[a..b) ↦_π σ      this scope may access r's addresses a ≤ x < b, which hold σ, with permission π
```

where σ is a sequence of b − a elements and π a permission (§12.3). Views form a **partial commutative
monoid** (V, ∗, emp), a separation algebra (Calcagno, O'Hearn and Yang, "Local action and abstract
separation logic", LICS 2007; Dockins, Hobor and Appel, "A fresh look at separation algebras and share
accounting", APLAS 2009). ∗ is defined on views whose ranges are disjoint, and on views of one range
whose permissions add (§12.3).

**The splitting law**, for a ≤ m ≤ b:

```
r[a..b) ↦_π σ·τ   ⊣⊢   r[a..m) ↦_π σ  ∗  r[m..b) ↦_π τ        where |σ| = m − a
```

- Read left to right it is a split. A segment `b[lo:hi]` (ADR 0074) is a middle piece, borrowed.
- Read right to left it is a join.

**Disjointness is linear arithmetic.** Two views on r are separate when b₁ ≤ a₂ or b₂ ≤ a₁. When the
bounds are linear terms over the program's integers, that is Presburger arithmetic, decided by the Omega
test already in the project (`presburger/`). Two views on different regions are separate by their names
alone. So the view fragment needs **no new decision procedure**, and ADR 0049's "own procedures, no SMT"
stands.

**What is not ranges.** A linked structure inside one region is one view of the whole arena, and its
links are integers in the contents, not views (§14.3). That choice keeps the fragment decidable:
- views are ranges of regions;
- the shape of the data is a fact about the contents (Part I's content facts), never a separating
  conjunction iterated over a recursive predicate.

### 12.3 Permissions

| permission π | may | duplicable | literature | the language today |
|---|---|---|---|---|
| **1, exclusive** | read, write | no (linear) | ℓ ↦ v | a `var` buffer; `linear` |
| **q ∈ (0, 1), a fraction** | read | splits, q = q₁ + q₂ | Boyland, SAS 2003; Bornat, Calcagno, O'Hearn and Parkinson, POPL 2005 | an observer (ADR 0069) |
| **□, persistent** | read | yes, □P ⊢ □P ∗ □P | Iris's ℓ ↦□ v | a frozen buffer, a constant |
| **dev, a device's** | read and write, each an effect | no | `volatile`; MMIO | nothing |
| **uninit** | write | no | typestate (Strom and Yemini, TSE 1986) | nothing |

The laws:
- **Sharing**: 1 = q₁ + q₂ splits a writer into readers, who join back into a writer. That is a shared
  borrow and its end.
- **Freezing** is 1 ⊢ □: the writer gives up writing forever and gets a duplicable view. It is one-way.
  There is no □ ⊢ 1, since a copy may still be reading.
- **uninit is a typestate**: r[a..b) ↦_uninit ⊥ⁿ becomes r[a..b) ↦_1 σ as each element is written. A
  read of uninit memory is refused. LLVM's `initializes((a, b))` states this transition on a host call.
- **A device view is not a cell** (§11.3). It is linear, so one scope drives the device, but its reads
  are not a function of its writes.

### 12.4 Contents have a type only through an abstraction function

At the bottom, a region holds bytes. A **typed view** reads k bytes as a value of T through an
abstraction function d_T : Bᵏ ⇀ T, which is ADR 0083's (H, d) with H = Bᵏ:

```
r[a..a+k) ↦ β   and   d_T(β) = v      gives      r[a..a+k) ↦_T v
```

For most T, d_T is **partial**: `Bool` is defined on 0 and 1 only, a variant on its tags. Two laws
follow, and they are the whole theory of type punning:

- **Encoding is total.** From r[a..a+k) ↦_T v, the bytes can always be seen, as β = e_T(v).
- **Decoding is total exactly when d_T is.** Bytes may be read as T without a check iff every bit
  pattern is a T. That holds for `Int32`, `0..2^64-1` and `F64` (every bit pattern is a float, NaNs
  included). It fails for `Bool`, a variant, and a range narrower than its width, where the read needs a
  **validity check**, or a fact proving the bytes lie in d_T's domain.

That is Rust's "validity invariant", stated as a property of the abstraction function. It replaces
**C's strict aliasing rule** (C17 §6.5¶7: an object may be accessed only through its own type or a
character type). That rule approximates this one by type identity: it forbids the sound pun of an `int`
as a `float`, and forbids nothing the abstraction function would allow.

**Retyping** a range is a change of the view's T with no run-time effect:
r[a..a+k) ↦ β ⊢ r[a..a+k) ↦_T d_T(β), under the condition above. C++ calls it placement `new`, and
seL4 calls it retype, of untyped memory. L³ calls it a **strong update**: a location's type changes
while one view holds it, which is sound because nothing else can see the location.

**Byte order lives in d.** d_{Int32,le} and d_{Int32,be} are different functions, so a pun here says
which order it reads, and means the same on every target. C's `*(int32_t *)p` reads the host's order.

### 12.5 The operations, as small axioms

These are in the style of O'Hearn, Reynolds and Yang (CSL 2001): each axiom mentions only what it
touches, and the frame rule supplies the rest.

```
{ r[a] ↦_π v }          load(r, a)        { ret = v  ∗  r[a] ↦_π v }        π ≠ uninit
{ r[a] ↦_1 _ }          store(r, a, w)    { r[a] ↦_1 w }
{ r[a] ↦_uninit ⊥ }     store(r, a, w)    { r[a] ↦_1 w }
{ emp }                 alloc(T, n)       { ∃r. r[0..n) ↦_uninit ⊥ⁿ }      r fresh
{ r[0..n) ↦_1 _ }       free(r)           { emp }
{ r[0..n) ↦_1 σ }       grow(r, m)        { ∃r'. r'[0..n) ↦_1 σ  ∗  r'[n..m) ↦_uninit ⊥ }
{ r[a] ↦_dev _ }        vload(r, a)       { ret = ?  ∗  r[a] ↦_dev _ }      an effect
a + k, a − b, a & m, a ⊕ b                no axiom: these are ℤ's operations
```

Every access needs its address inside the view's range. That is the **bounds obligation** every table
index already carries (tables.md), decided by the analysis or checked under `-checked`. **No axiom loads
without a view.** That missing axiom is the whole restriction, and it is a restriction on *how*: to load
from a, hold a view covering a.

### 12.6 Theorem V: with views, memory is values

**Theorem V.** Let p be a program whose every load and store lies in a view held by the scope
executing it, with views split, joined, lent and returned by the laws above. Then:

1. **Safety**: no execution of p accesses memory outside a live allocation, reads uninitialized memory,
   or touches a region freed or revoked. In LLVM's terms, p performs none of the memory operations whose
   behaviour is undefined.
2. **Erasure**: deleting the views changes nothing p computes. The views are in the logic, and the
   program's meaning is the computation's.
3. **Values**: an exclusive view's contents behave as a value threaded through p by the state-passing
   translation. A load reads the current value, a store gives a new one, and a lent view is an argument
   and a result (Theorem P, §2.3).

*Why.*
- (1) is the soundness of separation logic over a separation algebra (Reynolds, LICS 2002; Calcagno,
  O'Hearn and Yang 2007), and the type soundness of ATS's stateful views (Zhu and Xi, PADL 2005) and of
  L³ (Ahmed, Fluet and Morrisett, Fundamenta Informaticae 2007).
- (2) holds because a view never flows into a computed value: ADR 0049's "the logic never prints".
- (3) is Charguéraud and Pottier's translation of capabilities into state passing (ICFP 2008). In this
  language it is ADR 0052's S, and a buffer is the case where the compiler made the region.

**So nothing the language proves today is weakened.** β's substitution, the effect discipline, the
intervals and the claims all reason about values, and Theorem V says an exclusive view's memory is a
value. Only the other kinds behave differently:
- a fraction and □ are read-only, so they are still values;
- a device view, and a shared view under a lock, are effects, which the effect discipline already treats
  as impure.

### 12.7 Ownership is a tree

```
          root R[0..2⁶⁴) ↦ …              the OS: the entry point's view
           /             \
   kernel[0..k)       user[k..2⁶⁴)        split
                       /        \
                  proc A       proc B     moved to a process (ADR 0081)
                    |
                buffer b                  lent to a call (P2), returned
```

| operation | on the tree | law | the language |
|---|---|---|---|
| **split / join** | a node's range into children, and back | §12.2 | segments (ADR 0074), for a borrowed piece |
| **lend** | a subtree to a callee or child scope, returned at its end | the frame rule | `linear` parameters, borrows (ADR 0070) |
| **move** | a subtree to another owner, never returned | ∗ transferred | not yet: a linear result, a message (ADR 0081) |
| **freeze** | a subtree to □: copyable and read-only, forever | 1 ⊢ □ | a buffer's scope ending (ADR 0031) |
| **free** | a subtree consumed | free's axiom | a scope's end |
| **revoke** | an ancestor takes a subtree back from a live owner | a run-time act | a supervisor (ADR 0081) |

This is **seL4's capability derivation tree** (Elkaduwe, Klein and Elphinstone, "Verified protection
model of the seL4 microkernel", VSTTE 2008), whose two operations are retype and revoke. It is also
**Singularity's exchange heap** (Fähndrich et al., EuroSys 2006): a block has one owner, which changes at
each send, and a dead process's blocks are reclaimed. The sketch's operating system is this tree with R at
its root.

---

## 13. Why this design, and why not the others

**13.1 Why the address and the view are separate, unlike Rust's `&mut T`.** Rust fuses them: a reference
*is* its permission, so an address cannot be stored apart from it. A doubly linked list then needs
`Rc<RefCell<…>>`, with run-time counts and flags, or `unsafe`. ATS, L³ and Mezzo (Pottier and Protzenko,
ICFP 2013) separate them instead:
- addresses are data, copied into structures freely;
- the one permission is on the region.

**The separation costs nothing at run time**, since views are erased. In the proofs, a dereference needs
its address proven in the view's range, which is the obligation every table index already pays.

**13.2 Why views are erased, unlike CHERI's run-time capabilities.** A run-time capability is a fat
pointer: 128 bits carrying bounds and a tag, checked by hardware (Woodruff et al., ISCA 2014; Watson et
al., S&P 2015). It is sound without any proof, and it doubles every pointer and checks every access.
Erased views cost nothing wherever the proof succeeds. **On a CHERI target** (Morello), a view could
*print* as a hardware capability: a realization of the view, not a replacement for it (§14.17).

**13.3 Why linear, not affine.** An affine view may be dropped silently, which is a leak. Rust's
`mem::forget` is safe for exactly that reason. A linear view must be consumed: freed, returned, moved or
frozen. A scope's end consumes whatever the scope owns (Tofte and Talpin's `letregion`), so the rule
costs no syntax. A deliberate leak is spelled out: freeze it forever, as Rust's `Box::leak` does.

**13.4 Why addresses are offsets into a region, not absolute.** An offset is:
- **deterministic**: the same every run;
- **relocatable**: it survives `realloc` and a move between processes (§14.9);
- **portable**: every target has buffers.

An absolute address is none of these, and is offered only in the root region.

**13.5 Why there is no `unsafe`.** Rust's `unsafe` block suspends the proof inside it. RustBelt (Jung et
al., POPL 2018) shows how to re-establish the proof semantically, one library at a time. Here the only
trusted code is a **host declaration** (ADR 0022), written against the host's documentation. A program
never suspends a rule. What it cannot prove, it **checks**:
- a bounds test on an address read from data;
- a validity test on punned bytes;
- a count test on a shared object (§14.16).

So the price of what cannot be proven is a run-time check, never undefined behaviour.

**13.6 Why not a garbage collector of the language's own.** A collector is a hidden allocation policy
plus a runtime, which CLAUDE.md forbids in the core. A *host's* collector is a region kind like any
other: Go's, the JVM's and JavaScript's heaps are regions whose `free` belongs to the host (§14.17).

**13.7 Why not full separation logic.** Entailment with inductive predicates is undecidable in general,
and the tools draw lines inside it: Smallfoot's lists (Berdine, Calcagno and O'Hearn 2005), VeriFast's
annotations. §12.2 draws the line here: views are ranges of regions, with disjointness in Presburger
arithmetic, and shapes are content facts. That decides less than full separation logic does, using the
procedures the project already has.

**13.8 Why not C's `restrict` and strict aliasing.** Both are **promises**. The compiler assumes them, and
a program that breaks them has undefined behaviour that no tool reports. Here the same facts are
**theorems**: `noalias` because a view is exclusive, a pun because its abstraction function is total. The
optimizer gets what C gives it (§16), from a proof instead of a promise.

**13.9 Why not Shen's globals.** `(set *s* v)` from anywhere means every function may hold the view of
`*s*`. That is aliasing everywhere, and then β cannot substitute a read past a call. Here a global is a
region whose view the entry point holds and lends where it is needed (§14.11). A constant global is a
frozen region, readable anywhere.

**13.10 Why not region inference.** Tofte and Talpin infer ML's regions. This language's regions are its
lexical scopes and its buffers, each written where it is, so inference would have nothing to find. Cyclone
(Grossman et al., PLDI 2002) also chose explicit regions for C.

**13.11 Why not ownership types on objects** (Clarke, Potter and Noble, OOPSLA 1998; Pony's reference
capabilities, Clebsch et al. 2015). They own objects in a heap graph, which presumes a heap of objects.
This language has regions of elements, and an object graph is an arena with integer links (§14.3).

---

## 14. Everything C and C++ do with pointers, and how it is done here

The notation below is a **sketch**, and its spelling is not decided.
- **Already exist**: `var b := …`, `linear`, `b[lo:hi]`, `i ∈ 0..len(b)-1`, an `ensure`, `mod`.
- **New in the sketch**:
  - `uninit(T, n)`;
  - a `linear` result (a view moved out);
  - `split` and `join`;
  - typed reads of bytes (`load_i32le(b, a)`, `store_i32le(b, a, v)`), library functions over ADR 0083's
    abstraction functions;
  - the view kinds `dev` and `shared`;
  - `freeze` and `spawn`.

Each item gives C, then the form here, why it is safe, and what it costs.

### 14.1 `malloc` and `free`; use-after-free and double free

```c
int *p = malloc(n * sizeof *p);  init(p, n);
free(p);  p[0] = 1;    /* use after free: undefined */
free(p);               /* double free: undefined */
```

```
fn make(n ∈ Nat) (p ∈ Array(Int32) linear) {    // a linear result: a writable view moved out
    var p := uninit(Int32, n)
    fill(p, 0)                                 // uninit → initialized, element by element
    p
}
{ p = make(n)
  use(p)
}                                              // the scope ends: p's view is consumed, the region freed
```

- **Use after free** cannot be written. After the scope, `p` is not in scope. An integer address into it
  may survive, but no view covers it, and a load needs one.
- **Double free** cannot be written. Freeing consumes the view, and linearity refuses a second use
  (R0–R4).
- **A leak** cannot be written, because the scope's end consumes the view.
- **Cost**: none at run time. The printer picks the allocator by the view's place (ADR 0083's principle
  again): a view that never leaves its scope is an `alloca`, and one moved out is `malloc` or
  `HeapAlloc`, the host's allocator. That is Go's escape analysis, decided here by the program's own
  linearity rather than by a heuristic.

### 14.2 Pointer arithmetic and walking a string

```c
size_t strlen(const char *s) { const char *p = s; while (*p) p++; return p - s; }
/* without a NUL, it reads past the end */
```

```
fn strlen(s ∈ Array(0..255)) (n ∈ 0..len(s)) {
    loop p = 0 { p < len(s) && s[p] != 0 -> again(p + 1); else -> p }
}
```

- The address p is an integer: `p++` is `p + 1`, and `p − s` is `p − 0`. This is affine arithmetic, ℤ's.
- **Out of bounds** cannot be written: the load `s[p]` needs p ∈ [0, len(s)), which the guard proves.
- **Cost**: one comparison, which C's `strnlen` also pays. On LLVM the access is a `gep inbounds`, and the
  guard is the one bound the loop needs.

### 14.3 Linked lists, doubly linked lists, graphs with cycles

The folklore says linear types cannot express a doubly linked list. **That is true only when the views
are on the nodes.** Put the view on the **arena**, and the links are integers:

```c
struct node { int v; struct node *prev, *next; };     /* each node its own allocation */
```

```
// one region per field (a structure of arrays) until 6d decides records; the niche n is nil
fn unlink(n ∈ Nat, next ∈ Array(0..n) linear, prev ∈ Array(0..n) linear, i ∈ 0..n-1) () {
    a = prev[i]
    b = next[i]
    if a < n { next[a] := b } else { () }
    if b < n { prev[b] := a } else { () }
}
```

- The two regions' views are exclusive, held by the caller and lent for the call. The links `a` and `b`
  are integers in the contents, copied freely, and a cycle is just a cycle in the integers.
- **The hazard that remains is a link outside the arena.** The set `Array(0..n)` makes that a fact about
  the contents (0..n−1, or the niche n): proven where the link is stored, or else a bounds check where it
  is loaded.
- **A link to a slot that was freed and reused is a logic error, not a memory error**: the memory is
  still the arena's, and still typed. Generational indices close that gap with a check (slotmap; West,
  RustConf 2018).
- **This is how Rust programmers write graphs** (petgraph's indices), and how ATS and Mezzo verify them.
  The language already has every piece except moving the arena's view out of its scope (§14.1).
- **Cost**: an index is often *cheaper* than a pointer. A 32-bit link takes half the memory of a 64-bit
  pointer, and x86 addresses base + 4·i in one instruction.

### 14.4 `memcpy` and `memmove`

```c
memcpy(d, s, n);    /* overlap: undefined, and undetected */
memmove(d, s, n);   /* overlap allowed */
```

```
fn copy(d ∈ Array(V) linear, s ∈ Array(V), n ∈ 0..min(len(d), len(s))) ()   // two views, disjoint by linearity
fn move(b ∈ Array(V) linear, from ∈ Nat, to ∈ Nat, n ∈ Nat)
    where from + n <= len(b) && to + n <= len(b) ()                         // one view, two ranges in it
```

- C tells the two apart only in its documentation. Here the **types** do: two views are separate, so
  `copy` can never be handed overlapping memory, while `move` works within one view.
- **Cost**: `copy` prints `llvm.memcpy` with `noalias` as a theorem. §16 measures what that is worth.

### 14.5 Type punning: a float's bits, a header from a file, an invalid `bool`

```c
float f; uint32_t u = *(uint32_t *)&f;      /* strict aliasing: undefined */
struct hdr h; memcpy(&h, buf, sizeof h);    /* host byte order, and padding */
bool b; memcpy(&b, buf, 1);                 /* 2 is not a bool: undefined */
```

```
u = bits32(f)                       // the inverse of d_F32: total
magic = load_i32le(buf, 0)          // the byte order is explicit: the same on every target
flag = load_bool(buf, 4)            // d_Bool is partial: a validity check, or a fact
```

- This is §12.4's law: decoding is free when d_T is total, and checked otherwise. **Type confusion**
  cannot be written, since a T is read only through d_T.
- **Better than C, twice.**
  - The byte order is part of the read, so a format parser is portable.
  - There is no padding to get wrong: a record's layout is 6d's, and its bytes are read field by field,
    each through its own d.

### 14.6 Uninitialized memory and the growable vector

```c
int *v = malloc(cap * sizeof *v); size_t len = 0;   /* v[len..cap) is garbage; reading it is undefined */
```

```
// the view splits: r[0..k) ↦_1 σ  ∗  r[k..len(r)) ↦_uninit ⊥
fn push(r ∈ Array(Int) linear, k ∈ 0..len(r)-1, x ∈ Int) (k2 ∈ Int) ensure k2 == k + 1 {
    r[k] := x        // one element goes from uninit to initialized
    k + 1
}
```

- **An uninitialized read** cannot be written, because r[k..len(r)) has no read axiom.
- This is Go's slice and C++'s `std::vector`. (base, len, cap) is one region: an initialized prefix and
  an uninitialized suffix, with §12.3's law at the boundary between them.

### 14.7 The stack: `&local` and the dangling return

```c
int *f(void) { int x = 1; return &x; }      /* dangling */
```

- A block's region's view cannot leave the block, except **moved** (§14.1) or **frozen** (as its value).
- In either case the printer allocates the region where it outlives the block. So a dangling stack
  address cannot exist, and the choice of stack or heap is the compiler's, from the view's place.

### 14.8 C++: references, smart pointers, iterators, `span`, `launder`

| C++ | in the algebra | here |
|---|---|---|
| `T&` | an address with a view lent for its scope | a borrowed parameter |
| `std::unique_ptr<T>` | an address and an exclusive view, moved | a linear value: a view and its region |
| `std::shared_ptr<T>` | a counted fraction: the view is recovered when the count reaches zero (Bornat et al. 2005) | a shared view with a run-time count, the one place a count is needed |
| `std::weak_ptr<T>` | **an address without a view**, upgraded by a run-time test | exactly that: an integer address, and a checked `upgrade` |
| iterators | addresses (affine) | integers |
| `std::span<T>` | a segment | `b[lo:hi]` (ADR 0074) |
| placement `new` | a retype (strong update) | §12.4's retype |
| `std::launder` | recovering provenance the optimizer lost | not needed: provenance is the view's name, never lost |
| `offsetof`, pointer-to-member | an offset inside a record type | an integer constant from the layout (6d) |
| RAII destructor | the view consumed at the scope's end | the scope's end (§13.3) |

`weak_ptr` is the plainest witness for the sketch: C++ itself has an address without ownership as a
type, and recovers the ownership only by a check.

### 14.9 `realloc`: offsets survive a move

```c
int *q = realloc(p, 2 * n * sizeof *p);   /* every old pointer into p now dangles */
```

```
p2 = grow(p, 2 * n)                        // consumes p's view; p2's view covers [0, 2n)
```

- **Better than C**: addresses are offsets, so **every address into the region survives the move**, in
  every data structure that stored one. In C, a `realloc` that moves invalidates all of them, which is
  why C programs store indices into growable arrays anyway.
- The same holds when a region is moved to another process, when a wasm memory grows, and when a buffer
  is written to a file and mapped back: offsets are position-independent.

### 14.10 Tagged pointers and the XOR list

```c
uintptr_t w = (uintptr_t)p | tag;                       /* the low bits of an aligned pointer */
node *q = (node *)(w & ~(uintptr_t)7);
uintptr_t link = (uintptr_t)prev ^ (uintptr_t)next;     /* the XOR linked list */
```

- **Tagging.** In a region of 8-byte elements, an address is 8q. With ADR 0082's bits, `w = a | t` and
  `a = w & ~7` are ℤ's operations, and the decision proves `w & ~7` back in range from the fact that w
  was built that way. That proof is a content fact; without it, a bounds check.
- **NaN-boxing** is the same, with the payload in a float's mantissa: d_F64 is total, so the bits are free.
- **The XOR list** stores `prev ⊕ next`. Recovering `next = link ⊕ prev` is ℤ's arithmetic, and the load
  needs it in range: either a content invariant over the region, or a check. **It is memory-safe either
  way**, which C's is not.
- The **high bits** of a machine pointer (x86-64's 48-bit canonical form) exist only in the root region,
  on a target that offers it.

### 14.11 Globals, and Shen

```c
static int counter;  void tick(void) { counter++; }     /* written from anywhere */
```

```
export fn main() {
    var state := array { 0 }      // the "global": a region the entry point owns
    run(state)                    // lent where it is needed, visibly
}
```

- A **mutable global** is a region whose view the outermost scope holds. The functions that touch it say
  so in their headers, which is exactly what C's globals hide.
- A **constant global** is a frozen region (□), readable anywhere. Today's module constants are this.
- **Shen's** `(value *s*)` is a read under a view everyone holds, which is what breaks β (§13.9). A global
  that every process may write is a shared view under a lock (§14.12), which is concurrency's.

### 14.12 Shared mutable memory under a lock

```c
pthread_mutex_lock(&m);  shared->n++;  pthread_mutex_unlock(&m);
```

- **Concurrent separation logic** (O'Hearn, CONCUR 2004) says a lock *owns* a view, its resource
  invariant. Acquiring the lock gives the view to the critical section, and releasing it gives the view
  back.
- So a critical section is a borrow, and between acquire and release the memory is a cell again
  (Theorem V). An unsynchronized race cannot be written: without the lock there is no view.
- **Atomics** are a view kind whose every operation is a single atomic effect (SharedArrayBuffer's
  `Atomics`, LLVM's `atomicrmw`). Their orderings belong to concurrency's research (ADR 0081), not this
  one.

### 14.13 Function pointers, vtables and a JIT

- **A code address** is an address in the code region, which is frozen (□). So it is a value, and needs
  no linear view.
  - A function value carries its set, `fn(…) (…)`, so the wrong-type call that a C cast allows cannot be
    written.
  - A vtable is a frozen table of function values.
  - Callbacks are 6d's.
- **A JIT, and W^X as a theorem.** Write the code under an exclusive view, then **freeze** it, and allow
  only a frozen view to be executed.
  - "Writable or executable, never both" is then the law 1 ⊢ □, with execution allowed only at □.
  - Win32's `VirtualProtect(PAGE_EXECUTE_READ)` and POSIX's `mprotect` realize the freeze.
  - C has no way to say this.

### 14.14 Comparing, ordering and hashing addresses

- **Within one region**, addresses are integers: equality, order and hash are deterministic and portable.
- **Across regions**, only equality of region names is defined.
  - C leaves `p < q` across objects undefined. C++'s `std::less` defines a total order whose values are
    implementation-defined.
  - Here an order or hash across regions needs the root coordinate, which changes every run. So it is an
    **effect**, an impure read like the clock, and Tier 2.
  - A hash table keyed by object identity keys by (region id, offset) instead, where the region id is a
    counter the program assigns.

### 14.15 An operating system: MMIO, the page allocator, processes

```
export fn kernel(mem ∈ Root linear) {          // the root view: R[0..2⁶⁴)
    (low, rest) = split(mem, 0x10000000)
    uart = device(low, 0x09000000, 0x1000)     // a dev view: each read and write an effect
    vstore_u8(uart, 0, 'H')                    // LLVM: store volatile
    (heap, user) = split(rest, …)
    child = spawn(init, user)                  // the view moved to a process
    …                                          // on its exit, the supervisor's handler receives it back
}
```

- **MMIO**: a device range split off the root, with permission `dev`. It is linear, so one driver owns the
  device, and its reads are effects (§11.3).
- **The page allocator** is a region of pages whose free list is integers in its contents (§14.3).
- **Processes**: a view moved by `spawn`.
  - If the child is joined within the scope, the view was lent and comes back.
  - If the child lives on, the supervisor holds the right to revoke it.
  - After a crash, the view returns with its content facts erased (§11.5).
- **Open, and deep: page tables.** Their contents *define* other views: mapping physical range P at
  virtual V turns a view of P into a view of V. That is a morphism between two root regions, written by
  storing into a third, and it is seL4's mapping of frames. It is research for when an OS target exists.

### 14.16 Reference counting and dynamic borrowing: when the proof is a check

Some correct C programs share memory in a way no static rule proves: an object freed when the last of
several owners lets go, or a cache read while another path may write. The view algebra has a run-time
image for each:
- a **counted view** (`shared_ptr`): the count is a run-time number, and the view is recovered when it
  reaches zero (counting permissions, Bornat et al. 2005);
- a **flagged borrow** (Rust's `RefCell`): a view's exclusivity is checked at run time.

Both are **checks, not escapes**. Where C would have undefined behaviour, this program traps at the check:
gradual ownership (Sergey and Clarke, ESOP 2012). Every such check is visible in the source, and the
number of checked accesses is reported as `-checked`'s are.

### 14.17 What the targets offer, including what C cannot

| the target offers | in the algebra | here |
|---|---|---|
| JavaScript's `postMessage(buf, [buf])` | a **move**: the host detaches the sender's view at run time | a view moved to a worker; linearity states what the host enforces |
| JavaScript's `SharedArrayBuffer` and `Atomics` | a shared view with atomic operations only | §14.12's atomic kind |
| WebAssembly's `memory.grow` | `grow` of the instance's root region | offsets survive (§14.9) |
| Go's `*T` and `[]T` | a region the host collects; a slice a segment with a capacity | a region whose `free` is the host's; `b[lo:hi]` |
| the JVM's references, `ByteBuffer`, `VarHandle` | objects the host collects; typed views of bytes | a region; `load_i32le` as a `VarHandle` view of a byte array |
| Java's `MemorySegment` (FFM, Java 22) | **a region with a lifetime arena**, confined, shared or global | the same algebra, built by the JVM: an arena is a scope |
| LLVM's `addrspace(n)` (a GPU's global, shared and constant memory) | region kinds; `addrspacecast` a coercion where the target says one space contains another | a view's kind, chosen by place |
| CHERI (Morello) | views carried by hardware: bounds and a tag on every pointer | a view printed as a capability (§13.2) |
| ARM MTE | a 4-bit tag on memory and on the pointer: a run-time image of the region's name | a view's region, checked by hardware |
| Win32's `VirtualAlloc` and `VirtualProtect` | owned regions; page permissions as view kinds | uninit, 1 and □ (execute), per range |
| Win32's `GlobalLock` and `GlobalUnlock` | a view the host lends for an epoch | borrow-research's `lent` |
| `GetCommandLineA` | a persistent view the host made | □, decoded at the boundary (P6) |

The JVM reached the same algebra independently. Java 22's FFM API has memory segments with bounds, arenas
whose closing frees everything allocated in them, and confinement to one thread: regions, scopes and
exclusive views, checked at run time where this language proves them.

### 14.18 The Find dialog, P4, as a protocol of views

Part I left `FindTextA` as the wall, because the host keeps `&fr` past the call. In views it is a
protocol:

```
FindTextA(fr)                      moves fr's view to the host                (the program cannot touch fr)
WndProc(uFindMsg, lParam = &fr)    the host lends fr's view to the callback   (Theorem P, roles swapped)
   …                               the callback reads fr.Flags and what, and returns: the view goes back
WndProc(uFindMsg, FR_DIALOGTERM)   the host returns fr's view for good        (the program owns it again)
```

- Every step is a law already listed: a move, a lend for one call, a return. No new kind of view is
  needed, only declarations saying which message carries which view, which are the window procedure's,
  6d's.
- **What the program cannot write is the bug C allows**: touching `fr` between messages, while the dialog
  may be writing it.

---

## 15. The classic memory bugs, each with the rule that excludes it

| bug (CWE) | in C | the rule here | its price |
|---|---|---|---|
| use after free (416) | undefined | a load needs a view, and `free` consumed it | none |
| double free (415) | undefined | a view is linear | none |
| leak (401) | allowed | a view is linear, not affine; a scope's end consumes it | none |
| dangling stack address (562) | undefined | a block's view leaves only moved or frozen | the printer may allocate on the heap |
| out of bounds (125, 787) | undefined | every access lies in a view's range | a proof, or a bounds check |
| uninitialized read (457) | undefined | uninit has no read axiom | none |
| type confusion (843) | undefined | a T is read only through d_T | a validity check where d_T is partial |
| null dereference (476) | undefined | no null address; `Option`, with a niche | the test the program writes anyway |
| data race (362) | undefined | a write needs an exclusive view; sharing is a lock's or an atomic's | the lock |
| `restrict` or `memcpy` overlap | undefined | two views are disjoint by linearity | none |

**No row costs a capability.** Each excluded behaviour is undefined in C, so a correct C program never
does it, and every correct C program has a form here. Where its correctness is beyond the static rules
(§14.16), that form includes a run-time check.

---

## 16. Measured: what a proven `noalias` is worth

The loop is `for (i = 0; i < n; i++) a[i] += b[i]`, compiled by clang 23.1.4 at `-O2`, once with plain
`int *` parameters and once with `restrict`:

| | lines of LLVM IR | overlap check | scalar fallback loop |
|---|---:|---|---|
| plain pointers | 96 | two pointer comparisons (`icmp ult ptr`), run on every call | yes: a second copy of the loop, for overlap |
| `restrict` (`noalias`) | 47 | none | none |

- In C, `restrict` is a promise: a caller that passes overlapping arrays gets a vectorized loop that
  silently computes a wrong answer.
- Here the same `noalias` follows from two exclusive views, and a call that passes one buffer twice is
  refused by linearity. **So the language would give LLVM the facts of a C program fully annotated with
  `restrict`, each one proven.**
- This is Fortran's historical advantage over C (Fortran forbids aliased dummy arguments), and why Rust's
  `&mut` has carried `noalias` to LLVM since Rust 1.54.
- The speed of the result is not measured here. That belongs to `ir/llvm`, against hand-written C.

Part I's §3.5 adds two measurements:
- `captures(none)` lets a value be folded across an unknown call, and views state it;
- a pointer inside a record, or cast to an integer, loses every fold. Those are §14.18's protocol and
  P5's per-message declarations.

---

## 17. What changes in Part I

- **P1 becomes P1′.**
  - An address is an integer offset in a region.
  - Access needs the region's view: a linear claim in the logic, erased, owned by a scope, and lent,
    moved or frozen.
  - A buffer is the view of a region the compiler allocated, and a target or library may provide others.
  - The root region, where addresses are absolute integers, belongs to a target (Tier 2). `ptrtoint` and
    `inttoptr` are coordinates in it, offered where it is.
  - There is no load without a view.
- **P2** is unchanged and becomes a special case: a mode is a view lent for one call.
- **P3** is unchanged: a handle names something the program has no view of, because the host holds it.
- **P4** becomes §14.18's protocol of views, declared with the window procedure (6d).
- **P5** is unchanged.
- **P6** becomes a choice of view kind for host memory: owned (`VirtualAlloc`), lent (`GlobalLock`) or
  persistent (`GetCommandLineA`).

## 18. Open questions for hamza

1. **The direction itself**: addresses in the computation as integers, ownership in the logic as views
   (P1′). Recommended: everything above follows from it, and it is the sketch made precise.
2. **Views in the surface.** Are they written, as ATS writes `T @ l`, or always inferred from the
   binders that exist already (`var`, `linear`, segments), with new words only for uninit, move, freeze
   and the kinds? Recommended: inferred, with new words only where a program needs them. A program today
   already writes its views without naming them.
3. **A linear result**: moving a writable view out of a function (§14.1). It is the one construct that
   `malloc`, arenas and growable vectors need, and the first to build.
4. **Typed views over bytes** (§12.4): `load_T(b, a)` as library functions over each T's abstraction
   function, or a view `b as Array(Int32 le)` over a byte region. Records join this question in 6d.
5. **Shared and atomic views**: concurrency's (ADR 0081), and not before.
6. **The root region and page tables**: an OS target, not in the plan. Recorded so the algebra is known
   to reach it.

---

# Part III. Four clarifications: views in the surface, uninit, the linear result, codecs

2026-10-29. hamza, on Part II: "I mostly agree. I need some clarifications." In his words:
- *"views in the surface: show me my options, I need to see it to choose. examples."*
- *"and uninit? will it be in the language? provided by the target? is it needed?"*
- *"linear result: does it cost us? what about the proofs in our language?"*
- *"anything that is a library is cheap. nevertheless we should try our best in designing the interface."*

Every option is shown on the same tasks, so they can be compared line by line. The spelling of each
option is a sketch. What differs between options is **where the view is written**, and that is the
decision.

**The short version of Part III:**
- **Views in the surface** (§19): four options.

  | option | where the view is written |
  |---|---|
  | A | never: inferred from the binders that exist |
  | B | as a value threaded through the code (ATS's and L³'s style) |
  | C | as a formula in every header (separation logic's style) |
  | D | A's binders as abbreviations of C's formulas, with a formula written only where a binder cannot say it |

  **Recommended: D.** It is A for every program in the corpus, so nothing changes. C's power is there
  when a program needs it, and the abbreviation is a desugaring, so it is checkable. B is never right,
  because it puts the logic in the computation.
- **Uninit** (§20): four options. **Recommended: uninit is never a typestate the program sees.** A table
  is born full, either from its rule (`array { e : k ∈ 0..n-1 }`, today) or **at a host's boundary**: a
  fresh `out` table the host fills, of which the program receives only the filled prefix. Both are the
  language's on every target. A typestate in the surface waits on a measured trigger, because on Go, the
  JVM and JavaScript the host zeroes every allocation anyway. A target-only `uninit` is wrong in
  principle.
- **The linear result** (§21) costs nothing at run time after inlining, and **one** proof obligation is
  new: **generativity**. Each call makes a fresh region, so no transformation may merge two calls. The
  linearity checker also gains one rule: a function moves out only a view it owns, never one it borrowed.
  Each gets a planted fault. Intervals, facts, claims and termination see a table value, as they do
  today.
- **Codecs** (§22): the library's interface is one algebra. A **codec** is a partial isomorphism between
  a set and k bytes. Codecs are closed under products, powers and byte order, so one interface reads an
  integer, a file header and a Win32 `RECT`. That makes **6d's record layout a codec too**. Each target
  prints a codec as its own fastest access, measured on Go: `binary.LittleEndian.Uint32` compiles to one
  `MOVL`, and the big-endian read to `MOVL` and `BSWAPL`.

---

## 19. Views in the surface

### 19.1 The five tasks

| | task | what it needs |
|---|---|---|
| T1 | sum part of a table | a read-only view of a range |
| T2 | swap two elements | an exclusive view, lent for a call |
| T3 | make a table and return it | a view moved out of a function (§21) |
| T4 | sort two halves in one call, then merge | two disjoint views of one region at once |
| T5 | read a file into a fresh buffer | a view the host fills (§20) |

### 19.2 Option A: inferred, never written

The view is the binder: `var` makes a region and its exclusive view, `linear` lends one, a parameter
without a mode observes, a segment `b[lo:hi]` is a view of a range. The only new word is the move
(`linear` on a result, §21).

```
// T1: a read; a call lends the view of b[lo:hi] for its duration
fn sum(s ∈ Array(Int)) (Int) { loop i = 0, t = 0 { i < len(s) -> again(i + 1, t + s[i]); else -> t } }
t = sum(b[lo:hi])

// T2: an exclusive view lent for the call
fn swap(b ∈ Array(V) linear, i ∈ 0..len(b)-1, j ∈ 0..len(b)-1) () {
    x = b[i]
    b[i] := b[j]
    b[j] := x
}

// T3: moved out (§21)
fn zeros(n ∈ Nat) (p ∈ Array(Int) linear) { var p := array { 0 : k ∈ 0..n-1 }; p }

// T4: two segments of one buffer at once, allowed because their ranges are proven disjoint
fn sort2(x ∈ Array(Int) linear, y ∈ Array(Int) linear) () { sort(x); sort(y) }
sort2(b[0:m], b[m:len(b)])                 // 0 ≤ m ≤ len(b): [0, m) and [m, len b) are disjoint
merge(b, m)

// T5: a fresh table the host fills (§20, U3)
(data, e) = ReadFile(h, out set { t ∈ Array(0..255) : len(t) <= 4096 })   // data: the filled prefix
```

- **What it can say**: everything the corpus and TinyRetroPad need. Most programs already write their
  views without naming them.
- **What it cannot say**:
  - a function whose effect on a view is more than "read" or "read and write": it turns uninit into
    initialized, it splits a view and keeps half, or it promises the contents of what it returns;
  - a protocol over several calls, like §14.18's Find dialog.
- **New over today**: segments as views anywhere, not only at host calls (ADR 0074). Two segments of
  one buffer may be lent at once when the decision proves their ranges disjoint. The linear result.
- **Rust is this option** for slices (`fn swap(b: &mut [V], i: usize, j: usize)`; `split_at_mut` for
  T4). Rust writes lifetimes because its references can be stored, and here a view is never stored, so
  scopes suffice.

### 19.3 Option B: views as values, threaded

The address and the view are separate parameters, and every load and store takes the view and gives it
back. This is how ATS writes proof terms (`pf`) and L³ writes capabilities. The views are erased, but
they are spelled in the computation.

```
// T2
fn swap(r ∈ Region(V), i ∈ Nat, j ∈ Nat, v ∈ View(r, 0, len(r))) (v' ∈ View(r, 0, len(r))) {
    (x, v1) = load(r, i, v)
    (y, v2) = load(r, j, v1)
    v3 = store(r, i, y, v2)
    store(r, j, x, v3)
}

// T4
(v1, v2) = split(v, m)
v1' = sort(r, 0, m, v1)
v2' = sort(r, m, len(r), v2)
v' = join(v1', v2')
merge(r, m, v')
```

- **What it can say**: everything. A view is a first-class proof, so the page table of §14.15 can be
  written.
- **Why not**:
  - Every access threads a proof, which roughly doubles the code of T2.
  - It puts the logic in the computation's syntax, which ADR 0049 separates and fit-research would
    count as a misfit: a binder of the computation whose value never prints.
  - ATS's users report it as the language's main cost (Xi's own tutorials introduce views late, and
    mostly through library abbreviations).

### 19.4 Option C: views as formulas in every header

The code is plain loads and stores. Every function's header states what it needs and what it gives, in
separation logic, as VeriFast, Viper (Müller, Schwerhoff and Summers, VMCAI 2016) and Prusti do.

```
// T1
fn sum(r ∈ Region(Int), lo ∈ Nat, hi ∈ Nat) (Int)
    needs r[lo..hi) ↦_q σ          ensure r[lo..hi) ↦_q σ
{ … }

// T2: the separating conjunction makes i ≠ j a precondition
fn swap(r ∈ Region(V), i ∈ Nat, j ∈ Nat) ()
    needs r[i] ↦ x ∗ r[j] ↦ y      ensure r[i] ↦ y ∗ r[j] ↦ x
{ t = r[i]; r[i] := r[j]; r[j] := t }

// T3
fn zeros(n ∈ Nat) (r ∈ Region(Int))
    needs emp                       ensure r[0..n) ↦ 0ⁿ
```

- **What it can say**: everything B can, with code as plain as A's, and contents as well as access
  (`↦ 0ⁿ`).
- **Why not as the default**:
  - Every header carries a formula, even where a mode would say it all: compare T1 and T2 with A's.
  - The precision has a price that shows in T2. `r[i] ↦ x ∗ r[j] ↦ y` is false when i = j, so the
    separation logic version refuses a swap of an element with itself, which A's version allows.
    Writing the case i = j means a disjunction in every header of the kind.

### 19.5 Option D: A's binders are abbreviations of C's formulas (recommended)

A **mode is a formula**, and D, the desugaring, already turns surface forms into core terms (ADR 0050).
So each binder of A expands to the view formula it means:

| A writes | it abbreviates (C) |
|---|---|
| `b ∈ Array(V)` (a parameter) | needs b[0..len b) ↦_q σ, ensure b[0..len b) ↦_q σ: a fraction, returned unchanged |
| `b ∈ Array(V) linear` | needs b[0..len b) ↦_1 σ, ensure b[0..len b) ↦_1 σ′: exclusive, returned, contents may change |
| `b[lo:hi]` as an argument | the split law: lend r[lo..hi), frame the rest, join after |
| `var b := e` | ∃r. r[0..n) ↦_1 e: a fresh region, exclusive, consumed at the scope's end |
| `(p ∈ Array(V) linear)` as a result | ensure ∃r. r[0..len p) ↦_1 σ: moved out |
| `out set { t ∈ Array(V) : len(t) <= n }` at a host call | lend r[0..n) ↦_uninit; the host gives back r[0..k) ↦_1 σ (§20) |

**A formula is written only where no binder says it**, in the header's existing `where` and `ensure`.
These are the three cases A cannot express:

```
// what a function promises about the contents it returns
fn zeros(n ∈ Nat) (p ∈ Array(Int) linear) ensure len(p) == n && ∀k ∈ 0..n-1. p[k] == 0

// a host declaration's protocol across calls (the Find dialog, §14.18): which call moves, which lends
fn FindTextA(fr ∈ FindReplace moved) (h ∈ result(HWND, win.error))
// the window procedure's message carries the view back for one call: 6d's declaration

// a split kept: hand back the first half, keep the second (rare)
fn take(b ∈ Array(V) linear, m ∈ 0..len(b)) (head ∈ Array(V) linear) ensure len(head) == m
```

- **Why D**:
  - Every program in the corpus is unchanged.
  - A reader sees modes, which are short.
  - The checker sees formulas, so there is one rule set: the abbreviations are definitions, and a test
    can expand each one and check that it means what the table says.
  - The power of C is available exactly where a program needs it.
- **This is how the language already treats sets**: `Nat` abbreviates `0..`, and a range is a formula's
  solved form (ADR 0077). A mode abbreviating a view formula is the same move one level up.
- **What D needs that A does not**: the formula language of `ensure` grows the points-to ↦ and ∗, but
  only for the three uses above. ∀ over a range is already the claims' (settypes-research's array
  property fragment).

### 19.6 Side by side

| | A | B | C | D |
|---|---|---|---|---|
| T2's lines | 4 | 5 | 2 + header formulas | 4 |
| new words | `linear` result, `out` table | `Region`, `View`, `load`, `store`, `split`, `join` | `needs`, ↦, ∗ | A's, plus ↦ and ∗ in `ensure` where needed |
| a swap with i = j | allowed | allowed | refused unless written | allowed |
| contents promised | no | no | yes | yes, where written |
| the Find protocol | no | yes | yes | yes (a mode on the host declaration) |
| the page table (§14.15) | no | yes | yes | yes, by formulas |
| where the logic lives | in binders | in the computation | in claims | in binders, which are claims |
| programs changed | none | all | all | none |

---

## 20. Uninit: in the language, from the target, or not at all?

### 20.1 What it is for

An uninit view r[a..b) ↦_uninit ⊥ exists so that memory **is not written twice**: once to make it
valid, again with what the program wants. It has two uses:
- **speed**, skipping a zeroing pass;
- **meaning at a boundary**: a host's `_Out_writes_` buffer, whose old contents nobody reads.

### 20.2 What the targets do

| target | a fresh allocation | uninit possible? |
|---|---|---|
| Go | `make([]T, n)` is zeroed, always | no (only through `unsafe` or the runtime's internals) |
| the JVM | `new T[n]` is zeroed, always (JLS §15.10.2) | no |
| JavaScript | `new Uint8Array(n)` is zeroed, always | no |
| LLVM | `alloca` and `malloc` give undefined contents (`poison`/`undef` on load) | yes |
| Win32 | `VirtualAlloc(MEM_COMMIT)` and fresh pages are zeroed by the OS; `HeapAlloc` without `HEAP_ZERO_MEMORY` is not | partly |

**On three of the four targets, uninit memory does not exist.** Only LLVM has it, and even there the
OS's fresh pages are zero.

### 20.3 The four options, on the same three tasks

The tasks: **U-a**, fill a table of n squares; **U-b**, read a file into a fresh buffer; **U-c**, a
growable vector (append with capacity).

**U0. Never uninit: every table is born valid.**
```
sq = array { k * k : k ∈ 0..n-1 }                       // U-a: the rule writes each element once
var buf := array { 0 : k ∈ 0..4095 }                    // U-b: zeroed, then written by the host
(got, e) = ReadFile(h, buf)
var v := array { 0 : k ∈ 0..cap-1 }                     // U-c: the capacity zeroed
```
U-a costs nothing: a rule *is* an initialization without a prior zero (the compiler allocates and the
rule covers [0, n), which the decision proves). U-b and U-c pay a zeroing on LLVM only. On Go, the JVM
and JavaScript the host zeroes regardless, so U0 costs nothing there.

**U1. Uninit as a typestate in the surface.**
```
var buf := uninit(0..255, 4096)                         // r[0..4096) ↦_uninit
(got, e) = ReadFile(h, buf)                             // r[0..got) ↦_1 σ ∗ r[got..4096) ↦_uninit
x = buf[0]                                              // refused unless 0 < got is proven
```
- The checker tracks the initialized ranges as facts, a prefix [0, k) or a union of ranges, which
  Presburger arithmetic decides.
- A fill whose coverage is not a range, such as a permutation `b[p[i]] := x`, cannot be proven
  initialized, and the program would need a check or a rule.
- **Cost**: a typestate on every buffer in the checker, and new refusals to explain, to save a memset on
  one target of four.

**U2. Uninit offered by the LLVM target only.** Wrong in principle. Uninit means the same thing on every
target (memory not yet written, never read before it is), and every target can realize it (by zeroing).
So it passes the spec-first test as a language construct, and a target-only version would make a
portable meaning non-portable. CLAUDE.md: "Anything promoted to the language works on every target, and
the compiler finds the implementation."

**U3. Uninit only where the program cannot see it** (recommended):
```
sq = array { k * k : k ∈ 0..n-1 }                       // U-a: a rule, as today
set Block = set { t ∈ Array(0..255) : len(t) <= 4096 }
(data, e) = ReadFile(h, out Block)                      // U-b: the compiler makes r[0..4096) uninit,
                                                        //      the host fills r[0..got), and the
                                                        //      program receives data = r[0:got] ∈ Block
v = push(v, x)                                          // U-c: a vector in a library, whose capacity
                                                        //      is a rule-built table, or U0's zeros
```
- **Uninit exists only between the compiler's allocation and the first write that covers it**: a
  rule's, or a host's. The program never holds an uninit view, so it needs no typestate and no new
  refusal.
- It is ADR 0048's `out S`, a fresh cell the host writes, extended from a cell to a table. That is
  outparams-research §5's caller-typed out-cell with a length.
- **It is the language's on every target**: Go `make`, the JVM `new`, JavaScript `new Uint8Array`, and on
  LLVM an `alloca` or `malloc` with no memset, its call carrying `initializes`.

### 20.4 No new word: the set says it

An earlier draft wrote `out Array(0..255) of 4096`. hamza asked whether the set operations could not say
it already. They can, so `of` is withdrawn:

- **A table of length n over V is a function [0, n) → V**, an element of the exponential V^[0,n), and
  `Array(V)` is the union of these over every n. A table of n bytes is therefore the subset
  `set { t ∈ Array(0..255) : len(t) == n }`, which is ADR 0061's separation, already decided. TLA+ writes
  the same set `[0..n-1 → 0..255]`. It is only the separation with `==`, so the language needs no
  function-set former for it.
- **The set can state the host's result exactly.** `ReadFile` fills a prefix, so the true set of what
  it returns is `set { t ∈ Array(0..255) : len(t) <= 4096 }`, and one set gives two things:
  - **the allocation** is the set's supremum length, read off its formula as a range's bounds are read
    off theirs (ADR 0075, 0077). A set with no greatest length is refused at `out`, since there is
    nothing to allocate;
  - **the result** `data` is an element of the set, so `len(data) <= 4096` is a fact with no check.
- **A family over n is a definition over sets** (ADR 0066), for a library to name:
  `fn Bytes(n ∈ Nat) { set { t ∈ Array(0..255) : len(t) <= n } }`, then `ReadFile(h, out Bytes(4096))`.
- **What is missing is only the build of separation** (j.md's "not built" column). `len` inside a
  formula is already used, in host declarations' `where` and `ensure` (`targets/go/encoding/hex.oro`).

### 20.5 Is it needed?

| need | where it shows | U3 covers it? |
|---|---|---|
| a host fills a buffer (`ReadFile`, `WM_GETTEXT`, `io.Reader.Read`, `getRandomValues`) | TinyRetroPad, every I/O program | yes |
| a table computed element by element | every rule in the corpus | yes (a rule, today) |
| a growable vector's capacity | `append`-style programs; none in the corpus | as U0's zeros, until measured |
| an arena's free slots | §14.3's linked structures | as U0's zeros |

**Recommended: U3 now. U1 waits on a trigger**: a program on LLVM whose zeroing is measured above the
noise floor, against hand-written C that skips it. Until then the zeroing cost is a hypothesis. On three
of four targets it is zero by construction.

---

## 21. The linear result: what it costs, and what it does to the proofs

### 21.1 What it is

`fn make(n ∈ Nat) (p ∈ Array(V) linear)`: a function returns a **writable** view it owns, and the caller
becomes its one holder. Under the state translation (Theorem V, part 3) it is

```
make : ℕ → Table(V)          the caller binds the result as a state binder: var p := make(n)
```

a function returning a table **value**, with the promise that nobody else holds it, so the caller may
write it in place. Nothing in that is new to the core. A host declaration already returns a buffer this
way (ADR 0070's `p′`), and the linearity checker already seeds itself on every binder of a buffer
("a buffer is linear wherever it is bound", `emit/linearity.go`). What is new is a *definition* returning
one.

### 21.2 What it costs at run time

| where | cost |
|---|---|
| a call inlined (every definition today, ADR 0028) | nothing: after inlining, the region is the caller's, and `var p := make(n)` is `var p := array {…}` in the caller |
| the region's storage | chosen by the view's place: `alloca` if it never leaves the caller's scope, the heap if it does (§14.1) |
| Go, the JVM, JavaScript | the host's `make`, `new`, `new Uint8Array`, as today's buffers |
| an entry point returning one | the host's allocation, as today's frozen result |

### 21.3 What it does to the proofs

| procedure | effect |
|---|---|
| intervals, `ir.Decide` | none: the result is a table, its elements' sets the table's |
| facts and content facts | none new: after inlining, the store chain that fills the table is in the caller, and the facts flow as today. A declared `ensure` on the result is decided at the definition (ADR 0072) |
| termination (SCT) | none |
| claims, the refinement layer | none |
| **linearity**, one new rule | **a function moves out only a view it owns**: one made in its body, or received by a move. A borrowed (`linear`) parameter returned as an owned result would give the caller a second name for the lender's region, and is refused |
| **β and the reducer**, one new obligation | **generativity**: each call makes a fresh region (Pitts and Stark's ν; ML's generative functors). Two calls `make(n)` with equal arguments are *not* equal, because writing one must not change the other. So no transformation may merge two such calls: not β's sharing (ADR 0071 counts evaluations of one argument, so it never merges two calls), not a future CSE (which the plan deliberately excludes). Today this holds because nothing merges; the fault makes it a checked property |
| relevance (ADR 0043) | the result is relevant: it must be consumed (freed at a scope's end, returned, moved or frozen) |

### 21.4 Planted faults, before it is trusted

1. **A borrow moved out**: `fn f(b ∈ Array(V) linear) (c ∈ Array(V) linear) { b }`. Refused, with
   "b is lent by the caller; a function moves out only what it owns".
2. **Generativity**: `x = make(n); y = make(n); x[0] := 1; print(y[0])` must print 0 on every target.
   It fails if any pass merges the calls.
3. **A moved result used twice**: `p = make(n); q = p; p[0] := 1; q[0]`. Refused by the existing rule,
   now on a definition's result.

**A segment as a moved result** (`take` in §19.5) is not in the first build. Moving part of an allocation
leaves the rest owned by nobody, and on LLVM `free` needs the allocation's base. Go's and the JVM's
collectors would not care. On LLVM it needs the (base, offset, length) triple to be kept. It waits for a
program.

---

## 22. The bytes library: codecs

### 22.1 The algebra

A **codec** for a set S is a pair (e, d) with

```
e : S → Bᵏ           (encode, total and injective)
d : Bᵏ ⇀ S           (decode, partial)
d ∘ e = id_S         (round trip)          and  e ∘ d = id on dom(d)
```

It is a **partial isomorphism** (Rendel and Ostermann, "Invertible syntax descriptions", Haskell
Symposium 2010), and ADR 0083's representation (H, d) with H = Bᵏ. Its width k is static, and it is
**total** when dom(d) = Bᵏ. Codecs are closed under three constructions:

| construction | encode | width | total when |
|---|---|---|---|
| a product c₁ × c₂ (a record, a tuple) | e(a, b) = e₁(a) ++ e₂(b) | k₁ + k₂ | both are |
| a power cⁿ (a fixed array) | e(v) = e(v₀) ++ … ++ e(vₙ₋₁) | n·k | c is |
| a byte order, `be(c)` | e_be = reverse ∘ e_le | k | c is |
| padding, `pad(j)` | j bytes, written 0, ignored when read | j | yes |

**A record's memory layout is a codec.** C's struct with natural alignment is the product of its fields'
codecs, with `pad` where the data layout puts padding. So Win32's `RECT` is
`le(Int32) × le(Int32) × le(Int32) × le(Int32)`, and `OPENFILENAMEA` is its fields' product with pads.
**6d's record layout question becomes this library's**, and the printer realizes a codec that matches
LLVM's data layout as an LLVM struct type, and any other codec as loads at offsets.

The literature built exactly this, verified:
- **Narcissus** (Delaware, Suriyakarn, Pit-Claudel, Ye and Chlipala, ICFP 2019): encoders and decoders
  derived from binary formats, correct by construction.
- **EverParse** (Ramananandro et al., USENIX Security 2019): zero-copy validators and accessors for
  message formats, verified in Low* and shipped in Windows' Hyper-V.
- **Pickler combinators** (Kennedy, JFP 2004): the product and sum algebra.

### 22.2 The interface (a sketch)

```
// lib/bytes
set Order = 'le ∪ 'be

fn codec(S ⊆ Int, o ∈ Order)      // static: the narrowest k bytes, signed or unsigned, that hold S
fn f64(o ∈ Order)                 // static: 8 bytes, total (every bit pattern is a float)
fn pair(c1, c2)                   // static: the product
fn times(c, n ∈ Nat)              // static: the power
fn pad(j ∈ Nat)                   // static
fn width(c) (Nat)                 // static

fn get(b ∈ Array(0..255), a ∈ Nat, c) (v ∈ set(c)) where a + width(c) <= len(b) && total(c)
fn check(b ∈ Array(0..255), a ∈ Nat, c) (option(set(c))) where a + width(c) <= len(b)
fn put(b ∈ Array(0..255) linear, a ∈ Nat, c, v ∈ set(c)) () where a + width(c) <= len(b)
fn table(b ∈ Array(0..255), c) (t ∈ Array(set(c))) ensure len(t) == len(b) / width(c)
```

Why each choice:
- **A codec is a static value** (ADR 0063: a type is a static value), erased by staging. So
  `get(b, 0, codec(Int32, 'le))` prints as one host access, with nothing left of the codec.
- **The width comes from the set**, by ADR 0083's own rule (the narrowest representation holding S). The
  program says what values it reads, not how many bytes. `codec(0..65535, 'be)` is two bytes, read
  unsigned.
- **Totality decides the result's type**: `get` takes only a total codec and returns S, and `check`
  takes any codec and returns `option(S)`. `codec(0..9, 'le)` is one byte and partial, so the program
  must `check` it, or prove the bytes valid.
- **Round trip is a fact**: after `put(b, a, c, v)`, `get(b, a, c)` is v without a check, because
  d ∘ e = id. That is a store-chain fact (facts.md), and it lets a program read back what it wrote,
  partial codec or not.
- **The bounds are an obligation**, `a + width(c) <= len(b)`, decided like every index.
- **`table` is a typed view of the whole region**: element i at bytes [i·k, (i+1)·k), read through c, a
  representation of `Array(S)` by bytes. Java's `MethodHandles.byteArrayViewVarHandle` is exactly this.
- **Byte order is never implicit.** There is no `native` order. A program that wants the host's order
  is not portable, and §22.4's question 1 asks whether to offer it at Tier 2.

### 22.3 What each target prints

| `get(b, a, codec(Int32, 'le))` | prints | measured |
|---|---|---|
| Go | `int32(binary.LittleEndian.Uint32(b[a:]))` | **one `MOVL`** on amd64 (Go 1.27, `-gcflags=-S`); `'be` is `MOVL` and `BSWAPL` |
| the JVM | a `VarHandle` from `byteArrayViewVarHandle(int[].class, LITTLE_ENDIAN)`, `.get(b, a)` | intrinsified by C2 (JEP 193); not measured here |
| JavaScript | `dv.getInt32(a, true)` on a `DataView` of b's buffer | not measured here |
| LLVM | `load i32, ptr (gep b, a), align 1`; `'be` adds `@llvm.bswap.i32` | an unaligned load is one instruction on x86 |

A product prints as its fields' accesses at their offsets. Go's and LLVM's compilers then combine
adjacent loads where they can, as the `MOVL` shows.

### 22.4 Questions on the interface

1. **The host's byte order**, for a program that maps a host struct in place: offer `'native` at Tier 2,
   or never? Recommended: never in `lib/bytes`. A host struct's layout is the host declaration's codec
   (6d), and that declaration knows its order.
2. **Zero-copy access to a record**: `get` decodes a whole product into a tuple. EverParse's accessors
   read one field in place. Recommended: both, `get` for small records and a field accessor
   `field(c, i)`, a codec offset, for large ones.
3. **Sums** (a tag, then a payload chosen by it): the next construction after products, needed for file
   formats and for Win32's unions. It waits for a program.

---

## 23. What is asked of hamza

**Decided** (hamza, 2026-10-30): D, U3, the linear result, and the codecs, recorded in ADR 0084.

1. **Views in the surface**: A, B, C or D (§19.6). Recommended: D.
2. **Uninit**: U0, U1, U2 or U3 (§20.3). Recommended: U3 now, U1 on a measured trigger, U2 never.
3. **The linear result**: build it, with its two rules (owned-only, generativity) and three faults
   (§21.4). Segments as moved results wait for a program.
4. **Codecs**: the interface of §22.2, its three questions (§22.4), and **6d's record layout as a codec**,
   which moves part of 6d into this library.
