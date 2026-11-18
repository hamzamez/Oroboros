# 0131 — An out-parameter is a result, and a C call's arguments are written as C writes them

Date: 2026-11-18
Status: Accepted (hamza, 2026-11-18: "go ahead, … write the ADR for 1", on the audit after
[ADR 0130](0130-const-t-star-is-the-calls-ref-x.md); and on writing the call in C's notation, "yes,
write it into ADR 0131"). **Not built.** Amends
[ADR 0048](0048-a-host-writes-a-cell-through-a-reference.md)'s declaration form for `_Out_`, and
[spec/local.md](../spec/local.md) §5–§6 where a program writes `out S` that restates its
declaration's type. Respells [ADR 0118](0118-a-c-calls-arguments-are-a-linear-term-over-its-parameters.md)'s
argument terms and ADR 0130's `ref(x)` in C's syntax; their algebra and checks are unchanged.

## Context

ADR 0130's test: what a program cannot observe is representation, and goes in the host clause;
what it can is meaning, and goes in the header. A Win32 `_Out_` parameter is declared as a cell,
and the program writes the receiving cell at the call:

```
fn GetClientRect(w ∈ windef.HWND, rc ∈ cell(windef.RECT)) (r ∈ minwindef.BOOL)   // var rc := zero(RECT); GetClientRect(w, rc)
fn GetLocalTime(t ∈ cell(minwinbase.SYSTEMTIME)) (Bool)                          // (d, u) = GetLocalTime(out minwinbase.SYSTEMTIME)
fn WriteFile(…, put ∈ cell(minwindef.DWORD)) (r ∈ minwindef.BOOL)                // WriteFile(h, b, lo, n, out minwindef.DWORD)
```

local.md §5 already states the meaning: a host function f : X × Ref T → R that writes its cell
and keeps no address is, observably, f′ : X → R × T, valid because the cell is unaliased. The
cell's prior value is never read, so it means nothing; the address is how C hands T back. What the
program observes is one more result.

Writing it as a parameter lets a program do three things that mean nothing:
- **pass a `var` cell that holds a value**, which the host overwrites unread, as if it were
  `_Inout_` (vcell-2026-11-18 made such a cell pass at a host's cell of a range);
- **read the cell's zero as an answer** where the host did not write it, which local.md §6 warns
  against and only the result's shape prevents;
- **restate the declaration's type at the call**, `out minwinbase.SYSTEMTIME`, a second statement
  of what the declaration already says.

Seven of the nine Win32 cell parameters declared today are `_Out_`: `WriteFile`'s, `ReadFile`'s,
`GetFileSize`'s, `GetFileSizeEx`'s, `GetLocalTime`'s, `GlobalMemoryStatusEx`'s and
`GetClientRect`'s. `PrintDlgA`'s is `_Inout_`, and `GetDlgItemInt`'s is `_Out_opt_`.

**The notation.** hamza asked whether Win32 calls should be written as C, the way Go calls are
templates in Go. A template is sound where its language is the printer's output language and that
language's compiler checks it: Go's printer prints Go, and Go's compiler checks it. A C template
printed as LLVM IR needs a C front end, its types and its ABI. Free text is also what ADR 0094
removed templates for: nothing checks one against its declaration, and the analyses see nothing
inside it. But the arguments of a C call are few in kind:
- a parameter;
- the address of one;
- the address of a byte table's element;
- `NULL`.

Restricted to those, C's syntax for them *is* ADR 0118's term algebra in concrete syntax, which the
reader parses into the same terms.

## Decision

1. **An `_Out_` parameter is a result of the declaration.** It is a component of the header's result
   tuple, with its set: `fn GetClientRect(w ∈ windef.HWND) (rc ∈ windef.RECT, r ∈ minwindef.BOOL)`.
2. **Its address is the call's argument term `out(r)`**, r a result's name: the address of a fresh
   cell of r's set, which becomes r after the call.
   - A struct's cell is its layout's region, its determined fields written first (Theorem F), so
     `GlobalMemoryStatusEx`'s `dwLength` stays the declaration's.
   - Each `out(r)` counts as r's one occurrence in ADR 0118's linear term. A result named by no
     `out` is the C function's own return.
3. **The result tuple's order is the cells' values, then the C function's result**, the order hamza
   chose for cells (mutscope-research §9, decision 4), so a program's destructuring is unchanged:
   `(d, u) = sysinfoapi.GetLocalTime()` where it was `GetLocalTime(out minwinbase.SYSTEMTIME)`.
4. **`_Inout_` stays a cell**, `pd ∈ cell(PRINTDLGA)`. The program supplies a value the host reads,
   and reads back what the host wrote, both observed: f : X × T → R × T, a parameter and a result.
5. **`out τ` at a call stays where the program chooses τ**: Go's `fmt.Sscan(s, out int, out
   go.bytestring)`, whose cells are `...any`, so the type is the program's meaning. A program no
   longer writes `out S` where S is the declaration's own.
6. **An `_Out_opt_` the program never reads is `NULL`** in the call, its parameter dropped, as
   `lpOverlapped` is. One some program reads is a result, under 1.
7. **A C call's arguments are written as C writes them**, `host expr call F(a₁, …, aₖ)`:
   - the callee is its C identifier, unquoted;
   - each argument is one of five forms, each read as a term:

   | written | read as | meaning |
   |---|---|---|
   | `x` | `x` | the parameter, by its C type; a bare struct by value (ADR 0129) |
   | `&x`, x a parameter | `ref(x)` | the address of a fresh copy of x's encoding, `const T*` (ADR 0130) |
   | `&r`, r a result | `out(r)` | the address of a fresh cell that becomes r (items 1, 2) |
   | `&b[lo]` | `elem(b, lo)` | the address of byte table b's element lo, defined for lo ≤ len(b) (ADR 0118) |
   | `NULL` | `null` | C's null pointer |

   `&x` is decided by its binder, a parameter or a result, as `a.b` is (ADR 0128 item 3).
   `&b[lo]` is the one spelling of an element's address: `b + lo` is the same address, and one
   spelling is one form to check. Nothing else is C here: no casts, no arithmetic, no macros, no
   fields. The convention and the library stay clauses: `stdcall`, `lib "user32"`.
8. **The checks are ADR 0118's and ADR 0130's, on the terms read**:
   - every parameter and every `out` result appears exactly once;
   - `elem`'s table is of bytes and its offset bounded by the `where`;
   - `ref`'s set has a C layout;
   - a C string's `where nulfree`.

   The prototype check (`TestWin32DeclarationsAreTheirPrototypes`) compares the call's arguments
   with the SDK's, which now read the same.

So:

```
fn PtInRect(rc ∈ windef.RECT, pt ∈ windef.POINT) (r ∈ minwindef.BOOL) {
    host expr call PtInRect(&rc, pt) stdcall lib "user32"
}
fn GetClientRect(w ∈ windef.HWND) (rc ∈ windef.RECT, r ∈ minwindef.BOOL) {
    host expr call GetClientRect(w, &rc) stdcall lib "user32"
}
fn WriteFile(h ∈ winnt.HANDLE, b ∈ Array(0..255), lo ∈ 0..2^63-1, n ∈ minwindef.DWORD) (put ∈ minwindef.DWORD, r ∈ minwindef.BOOL) where lo + n <= len(b) {
    host expr call WriteFile(h, &b[lo], n, &put, NULL) stdcall lib "kernel32"
}
```

## Why not

- **Keep `cell(T)` for `_Out_`, and refuse a cell that holds a value.** That is a rule about a
  meaningless value, enforced at every call. The result form has no such value to refuse.
- **`out` as a mode in the header**, `rc ∈ windef.RECT out`: `in`'s fault (ADR 0130), a parameter
  that is not one, with its passing written as meaning.
- **The C return first**, `(r, rc)`: the more common order in C's documentation, but it would
  reorder every existing destructuring against decision 4, which hamza took for cells.
- **Derive `_Out_` from SAL at compile time**: the compiler does not read the SDK's headers. The
  survey and `TestWin32DeclarationsAreTheirPrototypes` do, and they check `&r` against the
  prototype's pointer.
- **C templates as text the printer translates** (hamza's question). Printing C as LLVM needs C's
  type checker and ABI, a front end. Printing it as C means the output is C, which ADR 0080 measured
  and rejected: no `nuw` or `inbounds`, speed by which C compiler, C's undefined behaviours. And
  text is what nothing checks against its declaration (ADR 0094). The restricted grammar keeps the
  notation and the checks. A C printer, were one written, would print these terms back as exactly
  this C, and a MASM printer would print `lea` and `push`: each printer realizes the term.
- **The function-call spellings `ref(x)`, `out(r)`, `elem(b, lo)`, `null`**: the same terms with
  names C does not use. A reader of a declaration compares it with the SDK's prototype, so the
  call should read as the prototype does.
- **`b + lo` beside `&b[lo]`**: two spellings of one address.

## Consequences

- Seven Win32 declarations become result forms, and their callers lose a cell or an `out S`:
  `lib/os/windows.oro`, `lib/io/windows.oro`, `lib/win/print.oro` and their tests.
- **Every C declaration's call is respelled** in the notation, the 61 Win32 declarations by header
  and the LLVM layer's, mechanically, with the quoted callee and the function-call terms retired.
- vcell-2026-11-18's scalar cell at a host's cell remains, for `_Inout_` scalars, a value supplied
  and read back.
- The checker and the state translation gain one form: a C declaration's result named by `&r`
  is a cell of the call, fresh, read into its result component after the call. That is local.md
  §6's expansion, moved from the program's `out τ` to the declaration's `&r`.
- **Left open, with its trigger**: a C shim per function compiled by clang, so the host's compiler
  owns the host's ABI, the parasite model taken literally. The first variadic or struct-returning
  Win32 call is its trigger, since each needs an ABI derivation of our own today.
- **Spec first**: local.md §5–§6, llvm.md §6a's term table, j.md's host clause and the
  inventory, before the build.
