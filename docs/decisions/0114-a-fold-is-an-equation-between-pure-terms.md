# 0114 — A fold is an equation between pure terms

Date: 2026-11-15
Status: Accepted (hamza, 2026-11-15: "go ahead with item 3", the book's wrong answers, where this is
the 10-06 pass's finding 3). Amends ADR 0009's fold with ADR 0010's premise.

## Context

ADR 0009 lets staging fold the language's integer operators on literals inside the word, since
compile time and run time then agree bit for bit. ADR 0010 makes an effect a side condition on β:
a primitive is impure unless declared `pure`, and an impure argument is let-bound, never
substituted or dropped. The fold did not read the purity bit. On `targets/tutorial-sloppy.oro`,
where `*` is declared without `pure`:
- `1 * 2` reduced to `2`;
- `fn(n) { 7 }(3 * 4)` reduced to `n = 12; 7`, the binding kept because the primitive is impure,
  the call itself evaluated away.

The book recorded this as a design question (chapter 4 §4.10): which of the two decisions gives way.

## Decision

**A fold is an equation between pure terms, so it applies only to a pure application.**
- An application f(c⃗) of an impure f denotes a computation, an element of T(A) for the effect's
  monad T. A value is an element of A, reached by η : A → T(A).
- Rewriting the computation to its value equates f(c⃗) with η(v). That equation holds iff f's effect
  is trivial, which is what `pure` declares.
- So the fold's premise is the purity bit. An impure application is left a call, and ADR 0010
  let-binds it.

On every real target the language's operators are declared `pure`, so nothing they emit changes.

## Why not

- **The fold wins, the purity bit is ignored for the language's operators**: the language's
  arithmetic is pure by definition, and a target that says otherwise is wrong. But a target's
  declaration is what it realizes, and dropping an effect it declared is the hazard ADR 0010
  exists against. Chapter 4 shows a forgotten `pure` costing a binding; ignoring the bit would show
  instead a declared effect silently erased.
- **Refuse a target that declares a language operator impure**: consistent ("a construct promoted
  to the language works on every target"), but it would forbid the teaching target that shows what
  purity buys. No real target needs it, and the purity premise makes the fold sound either way.
- **Fold, and keep the call for its effect**: `(let (* 3 4) (fn (n) 7))` with the result used as 12
  elsewhere. That is two meanings for one application, and an effect run for a value already
  computed.

## Consequences

- `1 * 2` on the sloppy target stays a call. `TestAnImpureApplicationIsNotFolded`, with
  `PlantFoldImpure`.
- Emission on Go, JavaScript, the JVM and windows is unchanged: their operators are `pure`.
- The book's chapter 4 §4.10 question is answered here.
