# 0133 — The window procedure is a λ at the host's field, over a cell in scope

Date: 2026-11-18
Status: Accepted (hamza, 2026-11-18, on [callbacks-sketches.md](../callbacks-sketches.md) Part 1:
"1.1 agree to B", "1.2 agree to 1", "1.3 option 3", "1.4b option (i)", "1.5 option a", and on 1.4,
"doesn't our language already handles these scopes? why do we need a new word ui()?", the cell's own
`var` scope as the extent). Builds [ADR 0053](0053-the-arrow-is-a-claim-on-a-host-parameter.md)'s
arrow for a host struct's field and a named claim, and [callbacks.md](../spec/callbacks.md) tier 2.
Revises [callbacks-research.md](../callbacks-research.md)'s candidate D, refused there for escaping
its scope. **Built**, steps 1–3 ([cbfield](../../gauntlet/results/cbfield-2026-11-19.md),
[msgloop](../../gauntlet/results/msgloop-2026-11-19.md), [slot](../../gauntlet/results/slot-2026-11-19.md)):
item 5's lending is the slot itself, Theorem T's global; item 6 for an ordinary cell too
([cellparam](../../gauntlet/results/cellparam-2026-11-19.md)).

## Context

A window procedure is a function of ours the host calls (callbacks-research §1), installed at
WNDCLASSA's `lpfnWndProc` and called from inside the host's own calls (`CreateWindowExA`,
`DispatchMessageA`, `SendMessageA`), re-entrantly (Theorem N). It needs four things the language does
not have: a set for "a function of this shape" at a struct's field, a function of ours with C's
convention whose address is a value, a state that survives from one message to the next, and a
meaning for that state when the host re-enters the procedure.

## Decision

1. **The claim is the arrow** (ADR 0053, research decision 5). A header declares the callback type as
   a named claim, a λ's header without a body:

   ```
   set WNDPROC = fn(h ∈ windef.HWND, m ∈ minwindef.UINT, w ∈ minwindef.WPARAM, l ∈ minwindef.LPARAM) (r ∈ minwindef.LRESULT)
   repr WNDPROC c(h, m, w, l) stdcall
   ```

   The `repr` is its C signature, as a struct's `repr … c(…)` is its layout: the C parameters in
   order and the convention (`CALLBACK` is `__stdcall`). A struct field may have the claim as its
   set (`lpfnWndProc ∈ WNDPROC`), and at a C boundary it is a pointer.
2. **A value at a claim's field is a closed λ or a definition's name**, which is its η-expansion
   (callbacks-tier1-research §3.2). The λ is checked under the claim (ADR 0053 item 4): its body
   assumes the claim's parameter sets, as an export's are assumed (ADR 0028), and its result is a
   claim in R decided at the λ (ADR 0072). A definition's own header is then an obligation at the
   inlined call, discharged under those assumptions.
3. **The λ is lifted to a function of ours** and printed with the claim's C convention: a wrapper at
   C's types around the function at the IR's (callbacks-research §4). The field holds the wrapper's
   address. A crash inside it is `abandon`, which exits without unwinding through user32.
4. **The state is a cell in scope, captured by the λ** (research decision 1, B, with 1.4b (i)). The
   λ at the field is written where the cell is in scope, `fn(h, m, w, l) { step(s, h, m, w, l) }`, so
   the state exists before any call that runs it: the extent is the cell's own `var` scope, and no
   word installs it (1.4). There is one state per λ, one per UI process (decision 2).
5. **The state is lent to every host call in its scope** (Theorem T, research decision 3, option 3):
   written to the λ's slot before the call and read back after it, inside the λ as outside it, with
   no classification of which calls dispatch.
6. **A step may take the cell as a parameter**, `fn step(s ∈ cell(S), …)` (1.4b (i)): a definition's
   cell parameter is the caller's cell, read and written in program order, as a host's is (ADR 0048).

## Why not

- **A, the state by hand at `GWLP_USERDATA`**: an address the program reads back, with C's stale-read
  bug available (research §3.3).
- **C, ADR 0081's process**: its kind is not built and its surface is open. B is its mechanism.
- **A word for the extent, `ui(s₀, step) { … }`**: the cell's `var` scope already begins before
  `CreateWindowExA` and ends where the program says, so the word bought nothing (hamza's question,
  1.4). It came from the handler literature's surface, not from a failure of the language.
- **Classifying which calls dispatch** (research decision 3's per-header effect): it buys back a store
  and a load per non-dispatching call and the facts across one, and is built when a program's
  refusal or a measurement names it (1.3).
- **A state per window**: a state per process whose value is a map from windows, the map's place a
  representation (1.2); built when a program has two windows that do not share state.
- **Threading the state by hand at each dispatching call, `lend(s, …)`** (1.4b (ii)): a word at every
  call for what a cell already means.
- **D as the research refused it, a closure that outlives its scope with a linear state**: the cell
  here is not a buffer, and its storage is the slot, not the frame.

## Set aside for hamza

**What a message after the cell's scope sees.** The class stays registered after the scope ends, so
the host may call the λ again. callbacks-sketches §1.4c wrote that the slot is cleared and the message
answered by `DefWindowProcA`, which needs the target to state a default for a claim, a word. Built
instead, until hamza decides: the slot is static storage, one per λ, never freed, so a late message
runs the λ on the state the scope left, and the program, whose cell is out of scope, cannot observe
it. Neither choice gives a wrong answer to the program.

## Consequences

- Built in steps, each its result document: (1) a closed λ or a definition at a claim's field,
  lifted and printed at the claim's convention; (2) MSG and the message loop's declarations
  (`GetMessageA`'s MSG an `_Out_` result, ADR 0131); (3) a captured cell, its slot and Theorem T's
  lending; (4) a definition's cell parameter.
- The planted faults of callbacks-research §5: the convention printed `cdecl` on i686, a host call in
  the scope not lent the state, the slot read fresh, a λ that captures a value it may not.
- callbacks.md's tier 2 and ADR 0053's arrow are built for a struct's field. A claim at a host
  function's parameter, and the markers, wait on tier 1.
