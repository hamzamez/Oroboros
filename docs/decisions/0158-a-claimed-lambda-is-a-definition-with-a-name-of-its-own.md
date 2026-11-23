# 0158 — A claimed λ is a definition with a name of its own

Date: 2026-11-23
Status: Accepted (hamza, 2026-11-23: "yes, record it and build it now. always mathematical and
algebraic", on the book's finding 121). Builds [ADR 0051](0051-a-definition-is-a-named-lambda.md)'s
D1 and its consequence ("An anonymous λ may carry a claim in the tree. No notation writes one yet"),
j.md's `lambda ::= "fn" params result? clause* block`, and [ADR 0066](0066-a-definition-over-sets-is-a-lambda-returning-a-lambda.md)
item 5 for a λ that writes its own claim. Built the same day
([lamclaim](../../gauntlet/results/lamclaim-2026-11-23.md)).

## Context

Chapter 1 §1.4 showed `export fn run() { fn(n ∈ Int) { n }(1) }` refused at the `∈`, and said "an
anonymous one takes what it is given". No decision says so:

- ADR 0051 made a λ carry what it claims, lam(x̄ ∈ S̄; r ∈ U; φ; ψ; e), and a definition the name
  bound to one. Its "Why not" refuses two operators that differ by a name.
- j.md's grammar gives `fn` the full header, and its §4 maps `fn(b̄) (r ∈ U) where φ ensure ψ { e }`
  to lam(b̄; r ∈ U; φ; ψ; e). Its status table listed "a λ's header" as not built.
- The tree already had `Lam.Claim`.

The refusal came from jswitch5b-2026-10-16, finding 2. J's first reader took any tokens as a λ's
parameters, so `fn(m ∈ 0..9) { m }` was a λ of five parameters, which failed at its call with an
arity no source states. The fix accepted names only and refused the `∈` as "not built". A stopgap
became a sentence of the book.

## Decision

1. **An anonymous λ reads a definition's header**: binders `x ∈ S`, a result in any of its three
   forms, `where` and `ensure`. A mode (`inout`, `linear`) is refused on its parameter, as a place
   is passed only to a named definition (ADR 0135, 0070).
2. **A claimed λ is a definition with a name of its own**, given by λ-lifting (Johnsson 1985). Let
   ȳ be the λ's free variables that are bound around it:

       fn(x̄ ∈ S̄) (R) where φ ensure ψ { e }  ↦  fn(x̄) { f#λN(ȳ, x̄) }
       fn f#λN(ȳ, x̄ ∈ S̄) (R) where φ ensure ψ { e }

   f is the enclosing definition and N the λ's line; a message calls it *the λ at line N*. The
   lifting is an equation by β, (λȳ.λx̄.e) ȳ = λx̄.e, so staging changes no answer (ADR 0009).
3. **Its claim means a definition's**. The meaning is the set membership f ∈ Π(x̄ ∈ P). R, with
   P = {x̄ ∈ S̄ : φ} and R = {r ∈ U : ψ} (ADR 0051 D2):
   - **the domain** is an obligation at each application, where the definition is inlined
     (ADR 0028, 0154);
   - **the codomain** is decided at the λ for every x̄ ∈ P (ADR 0072), not at each application. A
     claim proven only at the arguments it meets is not the claim: `fn(n ∈ Int) (0..9) { n }(1)`
     is false, though 1 ∈ 0..9.
4. **The context's declared part, Γ, is kept**. A yᵢ that is a parameter of the enclosing definition
   keeps that parameter's set. The definition's body assumes it, and the call of f#λN obliges it.
   Any other yᵢ claims nothing (`any`), so the codomain is decided for every value it may take. That
   implies the claim for the value it has, and refuses a claim that rests only on what the body
   derived; the remedy is to pass the value as a parameter with its set.
5. **At a host's callback parameter the claim meets the host's by composition**. The η-expanded
   wrapper takes the host's parameter sets (ADR 0066 item 5), and its call of f#λN obliges them to
   the λ's: dom(host) ⊆ dom(λ), contravariant. The host's result claim is checked on the wrapper's
   answer, which lies in R: answers ⊆ host's result, covariant. That is the arrow's subsumption
   without a rule of its own.
6. **A claimed λ capturing state is refused**: a `var` cell, a buffer, a map buffer or a borrowed
   buffer. A value parameter of the lifted definition cannot carry a place. The refusal names the
   form that works: a named definition taking the cell as `inout`, called by an unclaimed λ.

## Why not

- **The refusal kept, the book's sentence made true**: it would contradict ADR 0051's D1 and its
  "Why not", and j.md's grammar and §4, for no reason anyone stated. Three decisions agree, and
  the reader did not.
- **Each claim decided at each application** (the claim's marks left in the λ's body, β putting
  them on the arguments): the domain half is exactly that, but the codomain half would then be
  ∀ applications, not ∀x̄ ∈ P. It accepts `fn(n ∈ Int) (0..9) { n }(1)`, a false claim, which is
  ADR 0072's reason for deciding a result at its definition.
- **A second form in the core**, a claim carried by the core's `fn`: the residual has seven term
  kinds and its claims live in `sig`s and marks. A claim on a `fn` term would be read by every
  pass, where a lifted definition is read by the passes that already read definitions. It is also
  the synonym fit-research counts as a misfit.
- **Every yᵢ claimed by nothing**: simpler, and it refuses `fn(n ∈ 0..9) (0..14) { n + k }` inside
  `fn run(k ∈ 0..5)`. The enclosing definition's own claim is part of Γ, assumed in its body; using
  it costs one obligation, discharged by the same assumption.
- **All of Γ** (each binder's derived facts, a binder's own claim `j ∈ 1..6 = k + 1`): it would
  decide more claims. It is not built here; a binder's claim is the next part of Γ, waiting on a
  program that needs it.
- **State threaded through the lifted definition as `inout`**, ADR 0144's strongest host: a result
  claim on an `inout` definition is not decided on its own body today (*#setref is not a
  primitive*), so the form would compile only without a result claim. That is the named
  definition the refusal already points to.

## Consequences

- Chapter 1 §1.4 shows the claimed λ as a working form, and chapter 6 §6.12 shows a λ's header:
  its false result, its argument outside the domain, the context's parameter, and a computed value
  refused with its remedy. Chapter 14's `inout` section shows the refusal of a claimed λ writing a
  cell, then the named definition. All are checked by `docs/book/check.go`.
- A lifted definition is an ordinary definition. It is checked by `emit.CheckSignatures`, inlined
  with its contract, and kept as a function where the target keeps definitions (ADR 0113), under
  its name `f#λN`.
- A refusal at the definition's result names *the λ at line N* (`core.ShowLifted`); the other
  refusals name it through `core.ShowName`.
