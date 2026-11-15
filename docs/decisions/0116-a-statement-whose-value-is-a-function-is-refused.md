# 0116 — A statement whose value is a function is refused

Date: 2026-11-15
Status: Accepted (hamza, 2026-11-15: "I agree to the recommendations", the book's finding 22).

## Context

A statement is a binding whose name is discarded, `e; b` = (λ#_. b) e (ADR 0054). A pure value nothing
reads is dropped by β. So `io.print_line` on a line of its own, a function named without its
arguments, compiled and vanished: two prints around it printed two lines. Chapter 5 §5.11 teaches it
as a trap, and it is the likeliest mistake of a newcomer from C. Go refuses the same line, "f
evaluated but not used".

## Decision

**A statement whose value is a function is refused.** A function has no effect until it is applied: a
λ is a value, its body evaluated at each application, none here. Discarding one is therefore dead
code, and it cannot be what the program means. The function values are:
- a λ that is no tuple (the unit and a tuple are λk. k v⃗, so `()` as a statement stays legal);
- a definition that is a λ with parameters;
- a primitive that takes arguments (`Env.Callable`).

A nullary primitive is not one, since it may be a host's constant. `ignore(f)` (ADR 0043's !_A) is the
discard that says it means to.

## Why not

- **Keep it, the rules allow it**: they do, and the rule that allows it is β dropping a pure value,
  which is right for an argument and wrong for a statement. A statement exists for its effect, and a
  function has none until applied.
- **Refuse every pure statement whose value is not the unit** (Go's rule for any unused
  expression): the general form, `x + 1` on its own line. It may refuse statements the corpus writes
  on purpose, so it is measured on the corpus before it is proposed.
- **A warning**: the compiler has notes, not warnings, and a note no build fails on is read by no one.
  An obligation is discharged or the program is refused (refinements.md §3a).

## Consequences

- `io.print_line` alone is refused, naming it, and `ignore(io.print_line)` builds.
- On the sloppy teaching target, `shout; 7` is refused too.
- `TestAFunctionStatementIsRefused`, with `PlantFunctionStatement`.
- Chapter 5 §5.11 can show the refusal where it now shows the trap.
