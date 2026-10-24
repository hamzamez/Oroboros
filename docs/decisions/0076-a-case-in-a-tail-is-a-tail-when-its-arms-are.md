# 0076 — A case in a tail is a tail when its arms are: the tail sort closed under the eliminator

Date: 2026-10-24
Status: Accepted (hamza, 2026-10-24: "go ahead with (a)", on wc-2026-10-24 §6). Extends
[ADR 0041](0041-a-success-arm-is-a-tail.md) and [ADR 0057](0057-the-handled-bind-and-its-tail.md) item 5,
whose rule was "exactly one arm continues"; amends [spec/abstract.md](../spec/abstract.md) §2.5's Tail
and [spec/j.md](../spec/j.md) §2.8.

## Context

`examples/os/wc.oro` reports a file that will not open and goes on to the next. Its loop needs
`case gos.Open(name) { 'ok(f) -> again(…); 'err(e) -> again(…) }`, and J's reader refused it: "a case
arm is no tail". Only `try`'s success arm continued a loop, and every other arm left it. So the program
held 1 + File as a product by hand, `(File, Bool)`, with standard input as the junk slot's filler, and
needed a fourth state for "would not open".

## Decision

1. **The Tail sort is closed under a sum's eliminator.** case(e; p₁ → t₁, …, pₙ → tₙ) is a Tail when
   each tᵢ is a Tail; e and the patterns are expressions. A case whose arms are all expressions stays an
   expression.
2. **Why it is sound.** The eliminator runs exactly one arm, once, now: it applies its continuation once,
   as a host call's continuation runs once (ADR 0027) and as `try`'s success arm does (ADR 0041, which
   is this rule's instance with one continuing arm). So a jump in an arm is a back edge of the loop the
   case stands in, taken on that arm's path. It is the commuting conversion of the loop's step with the
   eliminator: step ∘ case(e; pᵢ → tᵢ) = case(e; pᵢ → step ∘ tᵢ). Nothing below the reader changes. D
   gives the same core term, and case-of-case puts each arm into its branch, where a jump is the loop's
   own (`examples/os/wc.oro` lowers to Go's `if err == nil { …; continue }`).
3. **Only in a tail.** A case with an arm that jumps, used as an operand, an argument, a binding's value
   or a scrutinee, is refused with the rule: it would be a jump inside an expression.
4. **`if` stays as ADR 0015 has it, and the asymmetry is principled.** An `if`'s condition can always
   move into the loop's guards, which are its choice. A pattern binds the payload, which a guard cannot:
   J's loop arms have no patterns (abstract.md §2.4's loop-arm patterns are not built). So the case is
   needed exactly because the guarded choice lacks binders.
5. **A handled bind's handler arms follow by the same law**, since bind(p; e; ā; b) := case(e; p → b, ā)
   (ADR 0057 item 1). That replaces item 5's "its handler arms leave the loop" with "each arm is a tail or
   leaves". J's `try` has no handler yet, so nothing is built for it.

## Why not

- **(b), abstract.md §2.4's loop-arm patterns**: a loop's arm is a pattern over its state variables, so a
  sum is a state variable and its tag is chosen among the loop's arms. It is the decided abstract syntax
  and the more uniform. But it needs a sum as a loop variable across a back edge, which ADR 0042's
  representation gives only at a loop's exit. It stays for when J takes loop-arm patterns. This decision
  is its special case for a scrutinee that is not a state variable.
- **Keeping "exactly one arm continues"**: it forces a program to write a sum's representation by hand,
  the work ADR 0042 does for it, and it added a state to `wc`'s loop.
- **Letting an `if` jump too**: an `if` has no binder, so the loop's guards already say what it would.
  ADR 0015's reason stands.

## Consequences

- `wc.oro` loses its `open` helper, its junk slot and its fourth state: an unopenable file is named and
  skipped in the case's own `'err` arm.
- The reader's tests: the arms that may jump, nested cases, a case after a binding, and an expression
  continuing from a case all accepted; a jumping case as an operand, an argument, a value or a scrutinee,
  a wrong arity in an arm, and an `if` that jumps inside an arm all refused. Each rule has a planted fault.
