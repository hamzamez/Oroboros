# 0135 — A cell parameter is the mode `inout`, not the set `cell(T)`

Date: 2026-11-19
Status: Accepted (hamza, 2026-11-19: "to choose we have to ask which is consistent, we use linear and
not buffer()", then "agree"). Reverses the spelling of
[ADR 0133](0133-the-window-procedure-is-a-lambda-at-the-hosts-field-over-a-cell-in-scope.md) item 6,
`s ∈ cell(S)`, and the target files' `x ∈ cell(T)`. Amends [spec/abstract.md](../spec/abstract.md)
W7. Corrects [words-audit.md](../words-audit.md)'s first verdict. **Not built.**

## Context

J has two spellings of one parameter: `x ∈ T inout`, the mode j.md §2.4 specifies, and
`x ∈ cell(T)`, which four target declarations and ADR 0133's step write. abstract.md's D maps the
first to the core's `(x (cell ⌜S⌝))`, as it maps `b ∈ Array(S) linear` to `(b (buffer ⌜S⌝))`. The
words audit's first pass counted 4 uses against 0, and kept `cell(T)` for them.

## Decision

1. **A binder is `x ∈ S mode`**: the set ranges over values, and the mode states what the program
   observes of the binder's relation to the caller's place. `x ∈ T inout` is a value-result
   parameter, the value in and the written value back (ADR 0048), as `b ∈ Array(V) linear` is a
   borrow (ADR 0020, 0070).
2. **`cell(T)` is not J at a parameter.** A cell is a place, not a value, and a set ranges over values
   (ADR 0049, 0084). `cell` and `buffer` are the core's types, D's image of the modes. Written at a
   parameter in J, `cell(T)` is refused, naming `x ∈ T inout`, as `buffer(V)` would be, naming
   `linear`. It stays the core's term and a target's `set cell(A) = host "…"`, which states the
   host's representation of the core's type.
3. **W7 is amended**: `inout` may stand on a program definition's parameter as well as a host
   declaration's, which ADR 0133 item 6 needs: `fn step(s ∈ S inout, …)`.
4. **The uses are respelled**:
   - SetFilePointer's `hi ∈ winnt.LONG inout`, PrintDlgA's `pd ∈ PRINTDLGA inout`, and every other
     target's `cell(…)` parameter;
   - ADR 0133's step, `examples/native/wndproc-win.oro`, `gauntlet/differential/cases/cell-param.oro`,
     the cmd/build tests;
   - callbacks.md §7 and local.md §3, §5.

   Both spellings are one core term, so the emitted code must not change. A full check showing no
   emission change is the build's witness.

## Why not

- **Keep `cell(T)` and withdraw `inout`** (the audit's first verdict): it writes a place where J
  writes sets, and it is inconsistent with `linear`. It was chosen for its four uses, and the uses
  were the leak.
- **Keep both**: a synonym, fit-research's F1 misfit, and a second spelling every reader must know.

## Consequences

- One spelling per parameter kind: `Array(V) linear`, `T inout`, and `out τ` at an argument.
- The reader's refusal of `cell(…)` at a parameter, with its message naming `inout`, gets a test;
  the respelled programs are its witnesses that nothing else changed.
