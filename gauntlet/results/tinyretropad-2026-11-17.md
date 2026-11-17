# TinyRetroPad 6c: its Win32 surface, measured by the survey's own rules

2026-11-17. Item 3 of [assessment-2026-11-17](../../docs/assessment-2026-11-17.md) §5, its half 6c
(D1 by header, W1, is the other half). A measurement: no compiler code changed. The survey's parser
was corrected on the way, which changes its published figures.

**TinyRetroPad imports 67 Win32 functions, and 47 are reachable today.** 43 are usable by the
survey's rules, 3 answer through an out-parameter (ADR 0048's cell), and `ReadFile` is declared by
hand. The other 20 stop at three walls: **a pointer to a record** (18 functions, 11 struct types),
**a callback** (`DialogBoxIndirectParamA`'s `DLGPROC`, plus the window procedure as a field of
`WNDCLASSA`), and **a variadic C call** (`wsprintfA`). Those are 6d's, as the 2026-10-24
assessment named them.

## 1. The source

`github.com/PlummersSoftwareLLC/TinyRetroPad`, branch `main`, which is commit `42f4c6c`: the one
measured on 2026-10-24. The earlier download had left no record of its location, so this time it is
written down (hamza, 2026-11-17, giving the repository). It was cloned as data into its own empty
directory, `scratchpad/trp-data/`. Its prebuilt `trpad.exe` (2,794 bytes) was never run, nothing was
built, and its `agents.md` was not followed. The source is 2,686 lines of 32-bit `stdcall` MASM in
30 procedures, including MASM32's `windows.inc`, `user32.inc` and `kernel32.inc`. Each import is
reached through its import-table slot, `[_imp__Name@n]`.

## 2. The measurement

- **The imports**: every `_imp__Name@n` the source names, 67, and one function loaded at run time,
  `LoadLibraryA("Msftedit")` for the `RICHEDIT50W` control.
- **Each classified by `gauntlet/stdlib/win32.go`'s own rules**: its header, its verdict, its arity
  and its DLL, through a new flag, `-names FILE`. That flag records the main loop's verdict per
  name, so a program's surface is judged by exactly the rule that produced the survey's
  percentages, not by a second copy that could drift.
- **The parse is checked against an independent fact.** The stdcall decoration `@n` is the bytes the
  callee pops, and on i686 every argument is 4 bytes, so n/4 is the arity: 64 of 64 agree. The 65th,
  `wsprintfA`, has no `@n` because it is cdecl, which the survey's "variadic" verdict already says.

| class | n | which | what it needs |
|---|---|---|---|
| usable | 43 | 29 of user32's 37, all 7 of gdi32's but `StartDocA`, 10 of kernel32's, `ShellExecuteA` | nothing new |
| an answer through an out-parameter | 3 | `WriteFile`, `GetFileSize`, `GetDlgItemInt` | ADR 0048's cell, built (`WriteFile` is used today) |
| an unresolved typedef to the survey | 1 | `ReadFile` (`LPOVERLAPPED`) | declared by hand already |
| **a pointer to a record** | 18 | `BeginPaint`, `EndPaint` (`PAINTSTRUCT`), `GetMessageA`, `DispatchMessageA`, `TranslateMessage`, `IsDialogMessageA` (`MSG`), `GetClientRect`, `FillRect` (`RECT`), `RegisterClassA` (`WNDCLASSA`), `GetLocalTime` (`SYSTEMTIME`), `StartDocA` (`DOCINFOA`), `GetOpenFileNameA`, `GetSaveFileNameA` (`OPENFILENAMEA`), `ChooseFontW` (`CHOOSEFONTW`), `FindTextA`, `ReplaceTextA` (`FINDREPLACEA`), `PrintDlgA` (`PRINTDLGA`), `PageSetupDlgA` (`PAGESETUPDLGA`) | records laid out by the data layout (6d) |
| **a function pointer** | 1 | `DialogBoxIndirectParamA` (`DLGPROC`) | callbacks, tier 2 (6d) |
| **variadic C** | 1 | `wsprintfA` | a cdecl variadic call |

- **By DLL**: user32 37, kernel32 14, gdi32 8, comdlg32 7, shell32 1.
- **By header**: WinUser.h 37, wingdi.h 8, commdlg.h 7, fileapi.h 4, datetimeapi.h, libloaderapi.h
  and WinBase.h 2 each, and handleapi.h, processthreadsapi.h, processenv.h, sysinfoapi.h and
  shellapi.h 1 each.
- **Declared by hand today**: 6 (`CreateFileA`, `ReadFile`, `WriteFile`, `CloseHandle`,
  `ExitProcess`, `GetCommandLineA`), all kernel32's.

The window procedure is the program's centre, and it meets both walls at once: it is a `WNDPROC`
(a callback), handed to the system as a field of `WNDCLASSA` (a record).

## 3. What failed first: the survey did not see two of the 67

`DefWindowProcA` and `ShellExecuteA` were "not among the SDK's entry points". Neither was the
SDK's omission; both were the survey's parser:
- **`ShellExecuteA`** is `SHSTDAPI_(HINSTANCE) ShellExecuteA(…)`. The shell, the light-weight utility
  library and the property system declare with `SHSTDAPI`, `LWSTDAPI` and `PSSTDAPI` (and two small
  ones). Each is `STDAPI`'s expansion under another name, `EXTERN_C DECLSPEC_IMPORT t
  STDAPICALLTYPE`, and the parser read only `STDAPI`. That left 773 entry points unseen.
- **`DefWindowProcA`** is `#ifndef _MAC LRESULT WINAPI #else LRESULT CALLBACK #endif
  DefWindowProcA(…)`, a convention the preprocessor chooses. The parser read neither the branch nor
  `CALLBACK`, which is `__stdcall` too. With `_MAC` undefined on Windows the declaration is its
  first branch, and the parser now reads it so, and `_M_CEE_PURE`'s block likewise.

So **the survey's universe was 802 entry points short, 6.5%** of what it reported. A survey's first
number describes the measurer (CLAUDE.md's lessons), and this one had stood since 2026-09-08. The
corrected figures:

| | before | after |
|---|---|---|
| flat C entry points | 11,575 | **12,377** |
| declarable | 10,479 (90.5%) | 11,270 (91.1%) |
| callable | 4,093 (35.4%) | 4,474 (36.1%) |
| callable and it links | 3,593 (31.0%) | **3,963 (32.0%)** |

`TestPublishedCountsAreTheCounts` pins the new report, and CLAUDE.md and the README quote it. ADRs
and earlier results keep the figures they measured.

Also: the first regex edit, made through a heredoc, turned `\b` into a backspace character in the
Go source. It was found because `DefWindowProcA` stayed unread, and removed. CLAUDE.md's rule for
backslashes again.

## Paid and bought

- **Compiler, run time, emission, proofs**: unchanged; nothing in the compiler was touched.
- **Tool lines**: `gauntlet/stdlib/win32.go` +75: `-names` and its report, the `STDAPI` family,
  `CALLBACK`, the two preprocessor branches, and the blocking argument's type.
- **Pinned figures**: `tooling_test.go`'s win32 block (18 lines), CLAUDE.md, README.
- **Not measured**: whether each of the 43 usable functions links on windows32. user32, gdi32,
  comdlg32 and shell32 join the link line from each declaration's `lib`, but no program calls them
  yet.

## What it sets up

- **D1 by header (W1)**, item 3's other half. The survey already emits a module per header
  (`win/fileapi`, 424 of them) with each declaration's `lib`, but in LLP64's sets. The hand
  declarations are by DLL (`windows/kernel32`). W1 makes both one namespace by header, with the
  type names in their headers interpreted per data model (ADR 0122 item 2).
- **6d**: records (11 struct types) and callbacks (two procedures), spec first, an ADR each, and the
  variadic C call.
