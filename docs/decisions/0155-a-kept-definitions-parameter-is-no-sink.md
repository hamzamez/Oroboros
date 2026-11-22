# 0155 — A kept definition's parameter is no sink

Date: 2026-11-22
Status: Accepted (hamza, 2026-11-22: "yes, fix 112 now, always mathematical and algebraic", the
book's finding 112). Amends [ADR 0150](0150-a-set-is-a-claim-and-rho-alone-chooses.md) (its sinks),
by [ADR 0102](0102-factoring-never-changes-an-answer-or-a-verdict.md) (factoring never changes an
answer). **Built 2026-11-22** ([keptsink](../../gauntlet/results/keptsink-2026-11-22.md)).

## Context

Under `-checked`, `grow` (3 squared seven times, printed) printed 3¹²⁸ on Go, JavaScript and the JVM
and stopped on windows. The finding named the cause as the order of two passes, the traps marked
before the rung; that order changed alone, windows still stopped, and with the cause below removed,
the order changes no program. The cause is the taint (ADR 0150, `tainted`): a value is kept a word
where a sink uses it, and a call of a program function was read as a sink at a word parameter.
Windows keeps `win/print.print_int(n ∈ Int)` as a function, its parameter the word because a kept
parameter above the word is not built (ADR 0113); so p was a sink's, a word, its product trapped
under `-checked`, and the kept call met its parameter's set by the trap. Without `-checked` the same
p failed the call's set, and P4 inlined the definition, whose body holds p in ℤ.

## Decision

1. **A kept definition's parameter is no sink.** Keeping is a factoring, and a factoring never
   changes an answer (ADR 0102), so the sinks of a program are those of the program inlined: the
   argument of a call of a kept definition of the program is used by the body, and P4 inlines the
   call where the argument leaves the parameter's set (`CheckCallSets`). The rung's instances
   (K(ℕ\*)'s, `nat.`) are the target's library, not the program's factoring, and keep their sinks.
2. **The traps stay where they were marked.** A trap is judged on what stays a word; with no
   factoring's sink, ρ holds in ℤ what reaches none, and the order of the marks is no part of the
   rule.

## Why not

- **Mark the traps after the rung** (the finding's proposal): built and measured; windows still
  stopped, since the kept call's word was a sink before any trap, and with item 1 the order changes
  no program, a rule nothing could tell apart.
- **Judge P4 before the traps**: built and measured; it inlined print_int at a table's read whose
  word the store had fixed, and the rung refused to widen the read's unbounded fact: the trap's
  checked form is what bounds it, and HEAD's judgement after it was right.
- **Build a kept parameter above the word** (the owed item): the larger cure, a kept function taking
  ℤ, which would also remove the sink; it is a representation of kept parameters, owed with its
  trigger, and item 1 holds whatever kept parameters become.

## Consequences

- Under `-checked` every target prints `grow`; a value that flows into a genuine sink (a table's
  element) still stays a word and traps on every target.
- D_A now holds in ℤ an operation whose only sink was a kept definition's parameter, and P4 inlines
  that definition where its argument leaves the word, as without `-checked`.
