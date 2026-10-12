# 0057 — The handled bind is a derived operator, written `try … else`; its success arm is a tail

Date: 2026-10-12
Status: Accepted (hamza, 2026-10-12, on surface-design.md §3.14: "I agree to all 4"; §3.9's item 2 and
2b, after §3.11–3.14). **Extends [ADR 0041](0041-a-success-arm-is-a-tail.md)** as 0041 extended
[ADR 0015](0015-loop-and-again.md): a success arm continues the loop, and every other arm leaves it,
now also when the other arms are the program's own handler. Amends
[spec/abstract.md](../spec/abstract.md) §2.5 and §2.7. Not built: today's notation has no handled bind,
so it is built with the surface.

## Context

Measured (`gauntlet/surface/control.go`): of the corpus's 42 `case` forms, 38 are a two-armed choice on
`result` or `option` whose failure arm computes a value of its own. None re-builds the failure, which
is `try`, and none abandons, which is `expect`. They are two populations: 11 steps (a program's `main`
reads its file or says why not, then the whole program; 10 in tail position) and 27 small choices of
two values.

surface-design §3.11–3.14 compared the spellings. A statement form, `ok(src) = e else h; rest`, keeps
the handler next to its call but hides the region (the rest of the block) and the destination (the
block's value). A braced form, `try ok(src) = e { rest } else h`, writes both but puts the handler after
the body. D's `TRY … IN` group has the strengths of both.

## Decision

1. **The handled bind is a derived operator of Σ**:
   ```
   bind(p; e; ā; b)  :=  case(e; p → b, ā)          the success arm b, the other arms ā a handler
   ```
   `try` is bind with ā re-building each other constructor, and `expect` is bind with ā abandoning, so
   both are its instances and keep their names, each naming a law (errors.md §5, §8).
2. **J writes it `try p₁ = e₁ [else ā₁], … { body } [else ā]`.** The header is a group of binders,
   sequential as `let`'s (ADR 0054), each with an optional handler next to its call; the body is
   braced; an optional handler after the body serves the binders without their own; with no handler
   a failure passes through re-built. A group of n binders is n nested binds. There is no statement
   form of `try`.
3. **D writes it `TRY p₁ == e₁ [ELSE ā₁], … IN body [ELSE ā]`**, the same structure.
4. **A small two-way choice stays `case`**, and `expect p = e else w` stays a statement: it abandons,
   so it has no body to show and no value to deliver.
5. **In a loop's arm, bind's success arm is a tail and its handler arms leave the loop.** The tail
   sort gains `bind(p; e; ā; t)`, t a Tail and each arm of ā an Expr, whose value is the loop's. That
   is ADR 0041's shape with a handler in place of the re-built failure: exactly one arm continues.

## Why not

- **The statement form** (§3.13's A): the region and the destination are a convention; it confused the
  first reader it met, and a statement list cannot show it.
- **B with the handler only after the body**: for the 11 steps the handler lands as far from its call
  as the program is long, and each step indents a level, the pyramid Swift added `guard` to escape.
- **A `guard` statement beside `try`** (Swift's pair): two layouts of one eliminator under two
  keywords. The group with adjacent handlers gives `guard`'s adjacency at the cost of one level of
  indentation per group, measured at 11 sites, all in a `main`.
- **A try block, `try { p = e; … }`** (§3.12): it moved the word off the binder onto a region and
  needed a new rule (a refutable binder only inside a try block). The tree was the same, but the page
  was not.
- **Leave handlers out of the tail sort**: a loop that reads and handles a failure by leaving with a
  value would have to re-build the failure into a sum and `case` on the loop's result, the round trip
  ADR 0041 removed for `try`.

## Consequences

- abstract.md §2.7 lists bind with its equation, §2.5's Tail admits it, and D maps it by its
  definition (§6, the derived operators), so nothing below D changes.
- W3 reads: in bind(p; e; ā; t) inside a loop, t is a Tail and ā's bodies are expressions; an `again`
  in a handler arm is refused, as one under `if` is.
- surface-design's C3 and C4 are written in the decided form in D and J.
- The 27 small choices stay `case`, the 11 steps become `try` groups, and the 3 `try` sites in loops'
  arms keep their `again` visibly inside the braces.
