# A claimed constant and an unread call, each decided where written (finding 45, the dead call)

2026-11-18. Two of the three items claims-2026-11-16 left owed. hamza: "go ahead with 45 and the
dead call". Both are false programs that compiled: a claim nobody checked, and an obligation nobody
decided.

## 1. What failed first

- **Finding 45.** `cap = 100; half ∈ 0..cap = cap * 2`, with `half` unread, compiled on every
  target. Read, it was refused under the reader's name. A read constant carries its claim as a mark
  on its value, `(the "int 0 100" (#req "#claim" "half" "int 0 100" 200))`, which is decided where
  the value is inlined. A constant nothing reads never reaches a residual, so its claim was decided
  by nothing.
- **The dead call.** `fn f(x ∈ 0..10)` and `a = f(-1)` with `a` unread compiled, as did
  `fn g(x) where x != 0` and `b = g(0)`. claims-2026-11-16 kept an unread binding's obligations
  when its argument already held a claim mark. A call's contract is marked only as β reduces the
  call, and an unread argument is never reduced, so there was nothing to keep.

## 2. What was built

**A constant is a definition of no parameters** (ADR 0072, carried to constants). Its contract is
⟦e⟧ ∈ S, and it holds of the definition, read or not. `ir.DecideConstantClaims` runs in `gen` and
`build` beside `CheckSignatures`.
- It normalizes each top-level constant holding a claim, alone, as a residual of no parameters.
- It decides the constant's marks by `DecideMarks`, the one procedure every residual's marks go
  through: a literal by evaluation, a range by the IR's intervals, the rest by the refinement layer.
- So the false constant is refused at its definition, under its own name, *half: the claim
  half ∈ 0..100 is false: its value is 200*, whether or not it is read. A true one builds.

**An unread call owes its contract** (ADR 0028: the obligations of P are P's). At the place β drops
an argument evaluated zero times, an argument that calls a name with a contract is now normalized,
and its obligations hoisted, as a claimed argument's already were.
- "A contract" means a definition's or a host's declared parameter ranges (`Env.Requires`) or a
  `where` (`Env.Wheres`).
- The test is syntactic and cheap (`Env.callsContract`): no other dead argument is normalized,
  which is the compile-time cost claims-2026-11-16 left unmeasured.
- Refused: *f's parameter x, declared in 0..10, … is false at a call: 0 <= -1 && -1 <= 10*, and
  *g's `where` is false at a call*. Unread calls whose contracts hold build.

## 3. The tests

`cmd/build/claim_test.go`, on Go and windows:
- **`TestAClaimedConstantIsDecidedWhereWritten`**: the false constant read and unread, each refused
  naming the claim; the true one prints 50. The fault `ir.PlantConstantClaimUndecided` decides a
  constant only where it is read, and the unread false claim must then build.
- **`TestAnUnreadCallOwesItsContract`**: an unread call with a false range and one with a false
  `where`, each refused naming it; unread calls whose contracts hold print 7. The fault
  `core.PlantDeadCallContractDropped` drops the contract with the value, and both false programs
  must then build.

## 4. Not built

- **A typing claim on an unread binder**, `s ∈ String = n; 0`, claims-2026-11-16's third item. It
  makes no mark, since its set is no range, and keeping it needs a typing obligation that survives
  as logic. No form for one exists. Still owed.
- **A constant's claim whose set is not a range** (`s ∈ String = 3`), the same item at the top level.

## Paid and bought

- **Compiler lines**: `ir/claimconst.go` 50 (new); `core/reduce.go` +40 (`callsContract`, the
  dead argument's rule, `HoldsClaim`, a plant); `cmd/gen` and `cmd/build` one call each.
- **Test lines**: two tests, 60 lines, two faults.
- **Emitted text changed**: none. Every program is byte-identical, with 473 emitted and 215 refused
  as before, so no program in the corpus held a false claimed constant or an unread call with a
  false contract.
- **Refusals changed**: a false claimed constant is refused at its definition and under its own
  name, where an unread one compiled and a read one was refused under its reader's; an unread call
  with a false contract is refused where it compiled.
- **The book**: chapter 6 §6.3 quoted the read constant's refusal under its reader's name,
  `integers-near-half`; the full check's book step failed on it, and the chapter now shows the
  refusal under the constant's own, read or not.
- **Compile time**: one normalization per claimed constant and per unread call with a contract:
  1.05× the baseline in the emission step's sweep, within its noise (the gate is 1.5× and 250 ms).
- **Proofs**: unchanged, 8,852 of 8,906 operations and 924 of 962 loops.
- **Language**: none.

