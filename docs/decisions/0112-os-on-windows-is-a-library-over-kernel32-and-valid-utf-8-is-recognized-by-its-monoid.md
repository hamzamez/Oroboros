# 0112 — `os` on windows is a library over kernel32, and valid UTF-8 is recognized by its monoid

Date: 2026-11-15
Status: Accepted (hamza, 2026-11-15: "go ahead with os on windows, always mathematical and
algebraic"; V2 of [oswin-research.md](../oswin-research.md) §3.3, as recommended).
Builds ADR 0048 item 6 on LLVM; extends [ADR 0091](0091-a-c-string-is-the-kernel-of-the-nul-homomorphism.md)
and [ADR 0094](0094-a-c-function-is-declared-by-its-prototype-and-the-c-abi-is-derived.md).

## Context

`lib/os.oro` is one definition over a cell `os/host` per host. Windows has none, so `read-loop` and
`text-of` skip it, and so does every program that opens a file. Win32's files are handles. Its
failures are a per-function sentinel and `GetLastError`, its counts come back through
out-parameters, and on windows Σ\* is represented by its UTF-8 bytes.

## Decision

1. **The windows cell of `os/host` is written in J over kernel32** (`lib/os/windows.oro`): each name
   is a short program over the handle API (open; size; read until full or 0; close), the failure
   threaded by `try` and `case`, the handle closed on every path. It declares exactly Go's names at
   Go's sets, as every cell does.
2. **A Win32 failure is a term, not a form.** The retraction r(b, e) = err e if b is the function's
   sentinel, else ok b, is written where the call is. `GetLastError` follows the call by the effect
   discipline (ADR 0010), which is Win32's contract. errors.md §4.4's `(fails (sentinel s) (error
   GetLastError))` is not built. The error type is the set `error = 0..2^32-1`, the DWORDs; the
   failure is the sentinel's, not the code's. The kinds are Go-on-Windows' (`Errno.Is`): not found
   {2, 3, 53}, exists {80, 145, 183}, permission {5}.
3. **An out-parameter is a cell** (ADR 0048), realized on LLVM:
   - `cell-fresh`, `cell-ref` and `cell-get` are an `alloca` of the cell's C type in the entry
     block, a `store`, and a `load` with `zext` or `sext`;
   - in a C prototype, `cell(T)` with T a C integer type is `T*`;
   - a byte table `Array(0..255)` is its elements' address, s + 8, which the callee may write when
     the parameter is `linear` (ADR 0070);
   - a buffer at an offset stays a template, the host's own operation (ADR 0089).
4. **A C string from Win32** (`GetCommandLineA`) is copied into a byte table up to its NUL: the rest
   of ADR 0091, total, B\* and not Σ\*. Its bytes become text only through text_of.
5. **V, the valid UTF-8, is a recognizable language, and two names enter Σ_L for it:**
   - `utf8_valid(b)`, V's predicate on a byte table: V = φ⁻¹(F) for φ : B\* → M, the monoid
     homomorphism into the transition monoid of the Unicode Standard's Table 3-7 automaton, and F
     the transformations taking the start state to itself (Eilenberg's recognizable sets; ν of
     ADR 0091 is the case M = (𝔹, ∨)). It is decided on an array literal by φ, and otherwise only
     by an identical guard on the arm it enters (Theorem E), as `nulfree` is;
   - `utf8_text(b) where utf8_valid(b)`, the inclusion ι_V : V ↪ Σ\*, the inverse of Σ\*'s UTF-8
     representation, total on its domain and one copy on every target.
6. **text_of on windows is d = ι_V ∘ r** for the maximal-subpart repair r : B\* → V, written in J.
   Valid input takes the guard first: `if utf8_valid(b) { utf8_text(b) } else { … r … }`. The
   repair's output is guarded again, and that guard's false arm is an `abandon` no correct r
   reaches.

## Why not

- **errors.md §4.4's `fails` form**: it would be a template of the retraction, which is a term over
  two host operations; ADR 0089 keeps templates for the host's own operations.
- **The error as an opaque host type with a niche**: Win32 has no type-wide niche. 0 is
  `ERROR_SUCCESS`, but a failing call need not set the code, so 0 cannot mark success.
- **Out-parameters as extra results of the host call**: ADR 0048's Why not, unchanged.
- **A one-element buffer for a count**: ADR 0048's Why not, unchanged; and Win32's count is a DWORD,
  whose C type the cell's set names.
- **text_of as a target helper in LLVM IR** (V1): the other hosts' trust (a declaration claiming
  d), but about 60 lines of IR that no analysis reads, and a claim about bytes that no check reads.
- **The decision proving r's image in V now** (V3): rational transductions preserve regularity, so
  it is decidable, but it is a new abstract domain over a buffer's written prefix. It would prove
  item 6's second guard, and it waits on a program that pays for that guard's pass.
- **`utf8` as ι_V's name**: `llvm/str.utf8` is already Σ\*'s representation read as bytes, the
  other direction.

## Consequences

- `read-loop` and `text-of` run on windows, equal to the three hosts and their expect lines; each
  name of `os/host` runs there in a program of its own (oswin-2026-11-15).
- **`examples/io/roundtrip.oro` on windows waits on ADR 0102's P8**: its nine calls inline nine J
  definitions into one `main`, past the reducer's depth (`core.MaxDepth`). A J library inlined at
  every call is that step's trigger.
- **Each name builds its sum once, at its tail**, as a host cell's retraction does (errors.md §4.2),
  and its raw steps are loops of one iteration, join points (ADR 0031). Case-of-case copies a
  consumer into each constructor site, and sums built in every arm of nested branches multiplied the
  program past the fuel.
- Every target realizes `utf8_valid` and `utf8_text`: Go `utf8.Valid` and `string(b)`; JavaScript
  and the JVM their decoders' strict modes; LLVM a DFA written in J (`lang.utf8_valid`) and a copy.
- Planted faults: `PlantUTF8Fault`, a literal read as valid without φ; `PlantFragment`, the repair
  rewritten to write U+FFFD as two bytes, so the guard's abandon is reached; `PlantCellUnread`, a cell
  read from a fresh slot, so GetFileSizeEx's size comes back 0.
