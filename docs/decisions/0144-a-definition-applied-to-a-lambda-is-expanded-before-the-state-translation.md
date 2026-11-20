# 0144 — A definition applied to a λ that writes a cell is expanded before the state translation

Date: 2026-11-20
Status: Accepted (hamza, 2026-11-20, asking why `twice(fn(x) { n := n + x })` was refused when a
host's `ordered` call is not: "why? Our own higher-order definitions: they still refuse a λ that an
ordered host would accept." The derivation below answers it, and the algebra decides it.)

## Context

ADR 0143 lets a λ at a host's callback parameter write a cell of its caller's where the host
promises the order of its calls: the cell is threaded through the host's activations (Theorem T₁,
the state monad's lift of a traversal). The same λ given to a definition of the program's,
`twice(fn(x) { n := n + 1; x + 3 }, 1)` with `fn twice(f, x) { f(f(x)) }`, was refused as a closure
(callbacks-sketches §3.2, "one asymmetry").

The refusal came from the order of two passes, not from the meaning. The state translation S runs
in `Load`, before β. For a host call the target tells S the host's promise, `(callbacks i o)`. For a
definition nothing tells S anything, and S does not look into the definition, so it refused.

What a definition promises is stronger than any host's:
- **no recursion** (ADR 0014), so its applications of a parameter are finitely many;
- **no escape**: a closure may not survive staging, so the parameter is applied during the call or
  the program is refused;
- **program order**: its body runs in the order it is written, every application of the parameter
  at a point β sees.

So its activations of the λ are a sequence the program itself fixes, exactly once each where its
body says. That is `ordered` with the count exact. Theorem T₁ then gives the translation:
S(d(λ)) = d_S(λ_S), where λ_S : S × A → S × R is the λ with the cell threaded and d_S is d's body in
the Kleisli category of State S. It is S itself, applied to the expanded term.

## Decision

1. **A definition of the program's applied to a λ argument that mentions a cell in scope is expanded
   before the state translation**, and S translates the expansion. Expanding a definition is β on
   values:
   - each λ argument is substituted for its parameter, which copies a value and runs nothing;
   - every other argument is let-bound first, in order, so its effects stay where they were
     written (ADR 0010, 0071);
   - an application of a substituted λ to its arguments becomes the lets of its parameters, the
     arguments in order.
2. **Nothing else changes.** A λ bound to a name (`add = fn(x) { n := n + x }`) is still refused, as
   is a λ given to anything whose body S cannot see. A definition taking no λ that mentions a cell
   is inlined by β as before, so no residual changes.
3. **The substitution captures nothing.** A term is locally nameless (a `Fn` closes its body over
   indices), so the λ's free names cannot meet the definition's binders.

Built 2026-11-20 (`core/cell.go`, `expandHigher`): Horner's rule through a definition `each`, and
`twice` counting its applications, print 89 10 7 2 on Go, JavaScript, the JVM and both windows
triples (callbacks-written-2026-11-20). The cell becomes a variable of `each`'s own loop: no
reference is left, since no host holds the λ.

## Why not

- **Keep the refusal.** It refuses a program whose meaning is defined, and admits the same λ given
  to a host that promises less. The refusal's reason, "it may run any number of times, or later",
  is false of a definition.
- **S after β for every term.** β erases the cell's scope boundaries that S reads (a scope, a loop,
  a host call's continuation). It is the larger change and buys nothing here: only a λ that mentions
  a cell needs its definition seen first.
- **S derives d_S from d's body without expanding.** It is the same lift, computed by hand, for a
  definition that β would inline anyway (a closure may not survive staging). Expanding gives S the
  term it already knows how to translate.
- **A marker on the definition, `ordered` on its parameter.** It would state what the body already
  shows, and could be wrong where the body is right there. A host's promise is a marker because its
  body is not ours to read.

## Consequences

- A higher-order definition of the program's is at least as strong a caller as any host: a λ given
  to it may write a cell under every target, `ordered` host or none.
- The expansion copies the definition's code at the call, which β does anyway: no residual is
  larger than the definition inlined.
- Planted fault: `PlantHigherNotExpanded`, the definition left unexpanded, refuses Horner's program
  as a closure, as before (`TestADefinitionAppliedToALambdaThatWritesACellIsExpanded`).
- The book's chapter 14 shows `twice` given a λ that writes a cell, beside the refused `add`.
