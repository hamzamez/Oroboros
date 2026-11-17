# φ's second kind: `icmp samesign` where the operands' signs are proven

2026-11-17. Item 5 of [assessment-2026-11-16](../../docs/assessment-2026-11-16.md) §5, after
[phi-range](phi-range-2026-11-17.md). hamza: "keep working all night, but build only what is
decided".

**A signed comparison whose operands the IR proves on one side of 0 is printed `icmp samesign`.**
Neutral on the five programs timed. The witness reads the printed text, not an answer, because
LLVM's use of the flag depends on its own rewrites. Planted on operands of either sign, the flag
did not change an answer either.

## 1. The algebra

- **samesign(a, b) ⟺ (a ≥ 0 ∧ b ≥ 0) ∨ (a < 0 ∧ b < 0)**, and LangRef makes the comparison poison
  where it fails. On that set the signed order and the unsigned order (the two's-complement reading)
  agree, a <ₛ b ⟺ a <ᵤ b, which is the fact the flag gives LLVM.
- **Proven** where both operands' ends (their IR_P types, or literals) are lo ≥ 0, or both hi < 0.
  An unsigned word's operands are excluded, since their comparisons are already unsigned.
- **Not arithmetic**: a comparison has no overflow, so the decision never marks it exact, and the
  flag reads only its operands' facts. It is written beside `flags`, not inside it.

## 2. The witness

`TestSamesignIsProvenOfItsOperands` (ir/llvm):
- `x < 3` over -5..5 takes no flag;
- the same comparison over 0..9 takes it, and `x > -3` over -9..-1 takes it;
- **THE PLANTED FAULT** (`PlantSamesignAlways`) prints the flag on the first.

**Why the witness reads text.** The first witness, at run time on windows, compared x < 3 for x
from -2 (a run-time value, len(os.Args()) − 3, so LLVM could not fold it). With the fault planted
the answer did not change: LLVM did not rewrite that comparison, so the poison was never observed.
A run-time witness would pass or fail on LLVM's heuristics, not on this rule, so the witness
checks the rule's premise, the sign facts.

## 3. What failed first

1. **The first run-time witness folded**: with constant inputs, LLVM evaluated the whole program,
   and a poison folds to whatever LLVM chooses.
2. **The second, with a run-time input, could not fail** (§2).

## Paid and bought

- **Compile time**: 1.01× the baseline in the reviewing run. A second sweep kept 5 of 6 suspects,
  and none was confirmed slower serially against `gen` at 9fe2045.
- **Run time**: serial, interleaved, 11 rounds, whole processes, quiet machine, medians (minima),
  ms, against HEAD built in a worktree. Outputs are identical.

  | program | before | with `samesign` | ratio |
  |---|---|---|---|
  | `fact(20000)`, printed | 43.6 (42.8) | 43.0 (40.2) | 0.986 |
  | `fib(10⁵)`, printed | 31.3 (30.0) | 30.7 (30.2) | 0.981 |
  | `cnt` (triple-research §3) | 441.6 (437.5) | 441.5 (438.4) | 1.000 |
  | `examples/table/sieve-win` | 30.5 (30.0) | 30.2 (29.4) | 0.990 |
  | `examples/io/wc`, 15 MB | 15.4 (14.7) | 15.6 (14.9) | 1.013 |

  All are inside the noise floor.
- **Code size**: the five executables are byte-for-byte the same size.
- **Emitted text**: 47 windows files and 7 of the gauntlet's 33 change, and with the flag removed
  every one is identical to the baseline. Go, JavaScript and the JVM are byte-identical. Accepted
  with `-accept`.
- **The gauntlet's generated cases** on windows, `bench.exe` built from the committed files and
  from the regenerated ones, five interleaved rounds, medians, ns/op; all 22 cases agree with Go
  in both builds:

  | case | before | after | ratio |
  |---|---|---|---|
  | g1 dot 1024 | 441.01 | 442.46 | 1.003 |
  | g1 dot 65536 | 30,373 | 30,071 | 0.990 |
  | g2 centroid | 30,066 | 30,019 | 0.998 |
  | g3 sum | 29,898 | 30,014 | 1.004 |
  | g6 alloc | 177,662 | 167,655 | 0.944 |
  | g6 into | 46,063 | 46,053 | 1.000 |
  | g7 tok | 6,179 | 6,194 | 1.003 |

  Inside the noise floor; g6's 0.944 is one in seven and inside it.
- **Proofs and refusals**: unchanged.
- **Compiler lines**: `ir/llvm/llvm.go` +40; test `ir/llvm/samesign_test.go` 45; llvm.md §8.
  Two printer tests that pinned `icmp slt i64` exactly now accept the flag; each asserts the
  realization, and their operands are natural.
- **Rules made less general**: none.

## What was not built

- **A comparison the printer writes itself**, outside the templates (a table's index bound in
  `tabulate`, the double word's sign test), which takes no flag yet.
- **The other kinds** (phi-range-2026-11-17 lists them).
