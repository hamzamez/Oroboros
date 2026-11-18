# 0131 — An out-parameter is a result; its address is the call's out(r)

Date: 2026-11-18
Status: Accepted (hamza, 2026-11-18: "go ahead, … write the ADR for 1", on the audit after
[ADR 0130](0130-const-t-star-is-the-calls-ref-x.md)). **Not built.** Amends
[ADR 0048](0048-a-host-writes-a-cell-through-a-reference.md)'s declaration form for `_Out_`, and
[spec/local.md](../spec/local.md) §5–§6 where a program writes `out S` that restates its
declaration's type. Extends [ADR 0118](0118-a-c-calls-arguments-are-a-linear-term-over-its-parameters.md)'s
argument terms.

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

## Decision

1. **An `_Out_` parameter is a result of the declaration.** It is a component of the header's result
   tuple, with its set: `fn GetClientRect(w ∈ windef.HWND) (rc ∈ windef.RECT, r ∈ minwindef.BOOL)`.
2. **Its address is the call's argument term `out(r)`**, r a result's name: the address of a fresh
   cell of r's set, which becomes r after the call:
   `host expr call "GetClientRect"(w, out(rc)) stdcall lib "user32"`.
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
6. **An `_Out_opt_` the program never reads is `null`** in the call, its parameter dropped, as
   `lpOverlapped` is. One some program reads is a result, under 1.

## Why not

- **Keep `cell(T)` for `_Out_`, and refuse a cell that holds a value.** That is a rule about a
  meaningless value, enforced at every call. The result form has no such value to refuse.
- **`out` as a mode in the header**, `rc ∈ windef.RECT out`: `in`'s fault (ADR 0130), a parameter
  that is not one, with its passing written as meaning.
- **The C return first**, `(r, rc)`: the more common order in C's documentation, but it would
  reorder every existing destructuring against decision 4, which hamza took for cells.
- **Derive `_Out_` from SAL at compile time**: the compiler does not read the SDK's headers. The
  survey and `TestWin32DeclarationsAreTheirPrototypes` do, and they check `out(r)` against the
  prototype's pointer, which the term makes the same shape as C's `&r`.

## Consequences

- Seven Win32 declarations become result forms, and their callers lose a cell or an `out S`:
  `lib/os/windows.oro`, `lib/io/windows.oro`, `lib/win/print.oro` and their tests.
- vcell-2026-11-18's scalar cell at a host's cell remains, for `_Inout_` scalars, a value supplied
  and read back.
- The checker and the state translation gain one form: a C declaration's result named by `out(r)`
  is a cell of the call, fresh, read into its result component after the call. That is local.md
  §6's expansion, moved from the program's `out τ` to the declaration's `out(r)`.
- **Spec first**: local.md §5–§6, llvm.md §6a's term table, and j.md's host clause, before the
  build.
