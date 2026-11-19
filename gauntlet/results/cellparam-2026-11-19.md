# A definition's cell parameter is the caller's cell (ADR 0133 item 6)

2026-11-19. ADR 0133 item 6, hamza's 1.4b (i): "a definition's cell parameter is the caller's cell,
read and written in program order, as a host's is". [slot](slot-2026-11-19.md) built it for the
cell a callback captures and refused an ordinary cell. This builds the rest.

## 1. The derivation

A cell parameter's meaning is the state-passing translation's at a call (local.md §3, Moggi's state
monad): f : S × X → S × R, the cell's value in and its last value out, the caller's cell written with
it. A slot realizes that by the global, which Theorem T makes equal. An ordinary cell in a scope has
no global, and the threaded pair is the translation itself, so nothing needs proving beyond it.

## 2. What was built

`core/slot.go`, before the state translation:
- **The value variant**, made once per definition with a cell parameter: `f~cellv`, each cell
  parameter s a value `s#in` (its set the cell's, so an obligation at every call, ADR 0028) held in a
  local cell s. Its body is f's own, and its value is the tuple of f's result and each s's last
  value, claimed in its set.
- **The call**: `f(…, x, …)` with x a cell in scope is `((f~cellv … x …) (fn (r v) (seq (set x v)
  r)))`, which the state translation then threads as any write to x.
- **Names in messages**: `ShowName` spells the variant and its value as the definition and the cell
  the program wrote, `bump's parameter s`.

## 3. Measured

`bump(s ∈ cell(0..100))` reads the cell and writes one more. Two bumps of `var n := 5` print `5 6 7`
on Go, windows and windows32 (`TestADefinitionsCellParameterIsTheCallersCell`). The differential
case `cell-param` agrees on Go, JavaScript, the JVM and windows for inputs 0, 5, 99 and 100. The
residual on all four is the program with the cells reduced away: three prints of constants.

## 4. The test

The fault, `core.PlantCellWriteBackDropped`, loses the callee's writes, so the program prints 5 5 5.
The refusals:
- a cell holding 200 at a `cell(0..100)`, "bump's parameter s is declared in 0..100, and a call
  passes 200";
- a write of `c + 1000` in the callee, "the claim s ∈ 0..100".

## Paid and bought

- **Compiler lines**: `core/slot.go` +150 (the variant and the call), `core/jshow.go` +4.
- **Tests**: +35 in `cmd/build`; one differential case.
- **Emitted text changed**: the new case only, emitted on all four targets (704 runs: 486 emitted, 218 refused); no other program changed, and the proof counts are unchanged, the case reduced to constants.
- **Language**: no new word. `x ∈ cell(S)` on a definition's parameter, decided by ADR 0133 item 6,
  now holds for every cell.
- **A rule made less general**: none. A refusal is removed, "an ordinary cell at a cell parameter is
  not built".
