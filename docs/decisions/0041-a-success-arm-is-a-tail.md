# 0041 — A success arm is a tail: `again` may sit under `try` and `expect`

Date: 2026-10-06
Status: Accepted (hamza: "if the algebra leads to your recommendation, do it"). **Extends
[ADR 0015](0015-loop-and-again.md)**, as [ADR 0027](0027-a-host-calls-continuation-is-a-tail.md) did:
"`again` may sit under a binding" now covers the binding a marked sum's success arm makes. Nothing
0015 decided is reversed.

## Context

The first read loop the error model meets (spec/errors.md §12, step 5) is "read, `try`, go round":

```lisp
(loop ((n 0))
  (>= n limit)  (result.ok n)
  else          (try (result.ok line) (read r)
                  (again (+ n 1))))
```

The reader refused it: `again` may not sit inside an expression, and `try` is one. With the reader
relaxed, as an experiment, the next refusal did not involve `try`. **A loop could not yield a sum**: its
exits were constructors, and the `case` on the loop was refused as a tuple pattern with no tuple in it
(expect-2026-10-06 §6). Both had to be closed together.

## Decision

1. **A success arm is a tail.** `(try (s x) e body)` and `(expect (s x) e why body)` bind the payload
   and run `body` once, now. That is a binding's tail in ADR 0015's sense, as a host call's continuation
   is in ADR 0027's. So `body` may be a clause body: an `again`, or anything that may hold one. The
   value, the pattern and the reason are expressions, and `again` is refused there.
2. **The other arms are exits, not back edges.** `try` returns the failure, so the loop's value is the
   sum, and `expect` goes into 0. Neither goes round, so the clause list is still every back edge, and
   0015's reason for its rule stands.
3. **A constructor is the tuple (tag, payload)**, the term `tuple` reads as, binder and all:
   `(fn (#k) (#k tag payload))`. sums.md §1 always said a sum is represented as that product. Spelled
   apart (`#x`), the passes that know a tuple did not know a sum. Spelled as one, a loop whose exits are
   constructors yields the sum as two results with no new rule.

After reduction the success arm is an arm of an `if`, which every walker of a clause chain already
walks. So nothing below the reader learns that `try` exists.

## Why not

- **Keep the rule and make the program recurse by hand.** The language has no recursion (ADR 0014).
  A loop whose step can fail could then be written only with the failure threaded as a loop variable
  and tested at every clause: the product the error model exists to retire (ADR 0040).
- **Allow `again` under any `if`.** That is the general `cond` 0015 rejected: the branches of an `if`
  are alternatives that may both go round, so the clause list would stop being the control flow. A
  success arm has one alternative that goes round and others that leave, which is what a binding with
  a failure exit is.
- **A sum out of a loop as its own construct.** Not needed: it is a tuple out of a loop, which
  tables.md §2.5 and ADR 0031 already built. What was missing was the spelling that let the passes
  see it.

## Consequences

- **A loop over a fallible step is written as it reads**, and on a host whose failure is a niche (step
  4), what is emitted is the host's own `if err != nil { return … }` inside the loop.
- **A binding of a product is taken apart**, by the η law for products, r = (π₁ r, …, πₙ r): a value
  whose every tail is an n-tuple is bound as a tuple pattern, so a sum read twice reduces, and a variant
  out of a scope is ADR 0031's product rather than R1 (sumloop-2026-10-06 §5).
- **The substitution β's recount uses was unsound for a value holding an index**, and writing the first
  program under this ADR found it. It is now the locally nameless substitution, which shifts a value by
  the depth it lands at and never opens a binder (sumloop-2026-10-06 §3).
