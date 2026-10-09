# Architecture Decision Records

One decision per file, numbered, never deleted. When a decision is reversed, write a new ADR
that supersedes the old one and mark the old one `Superseded by NNNN` — do not edit history.

This exists because this project gets put down and picked up again. The value of an ADR is
almost entirely in the **Why not** section: six months from now, the decision itself will be
visible in the code, but the rejected alternatives and their reasons will not.

## Template

```markdown
# NNNN — Title

Date: YYYY-MM-DD
Status: Accepted | Superseded by NNNN | Reversed

## Context
What forced a decision.

## Decision
What was decided, stated flatly.

## Why not
The alternatives, and what specifically ruled each one out.

## Consequences
What this makes easy, what it makes hard, and what it commits us to.
```

## Index

| # | Decision |
|---|---|
| [0001](0001-parasite-model.md) | Targets are ecosystems; portability is a program property |
| [0002](0002-capability-graph.md) | A capability graph replaces the fixed layer tower |
| [0003](0003-range-typed-integers.md) | Range-typed integers with mathematical semantics |
| [0004](0004-first-targets.md) | Go, JavaScript, and Java/Android first; C deferred |
| [0005](0005-implementation-language.md) | The compiler is written in Go |
| [0006](0006-ir-file-format.md) | The backend interface is a file format |
| [0007](0007-exploration-over-specification.md) | Explore candidates against a fixed test, don't specify the core first |
| [0008](0008-measurement-over-principle.md) | Parasite decisions are per-target measurements, not principles |
| [0009](0009-staging-preserves-results.md) | Staging must not change results |
| [0010](0010-effects-as-structural-rules.md) | Effects are a side condition on β, not a feature |
| [0011](0011-modules-add-nothing-to-the-reducer.md) | Modules are resolution, not reduction |
| [0012](0012-portable-integer-range.md) | `int` is exact within ±(2⁵³−1) — partly superseded by 0026 |
| [0013](0013-accept-the-allocation-price.md) | Accept the allocation price, provisionally |
| [0014](0014-recursion-is-not-in-the-language.md) | Recursion is not in the language |
| [0015](0015-loop-and-again.md) | `loop`/`again` — guarded clauses over n variables |
| [0016](0016-targets-need-not-have-expressions.md) | A target need not be an expression language |
| [0017](0017-booleans-are-in-the-language.md) | Booleans and control flow are in the language |
| [0018](0018-immutable-values-linear-buffers.md) | Immutable values, one scoped linear buffer |
| [0019](0019-precision-by-declaration.md) | Precision by declaration |
| [0020](0020-uniqueness-on-parameters.md) | A buffer is a nameable type: uniqueness on parameters |
| [0021](0021-declarations-are-theories.md) | Declarations are theories, and the surface that follows |
| [0022](0022-host-declarations-are-written-by-hand.md) | Host declarations are written by hand; the generator is their checker |
| [0023](0023-a-generator-does-not-make-a-claim-it-cannot-justify.md) | A generator does not make a claim it cannot justify |
| [0024](0024-comments-are-erased.md) | A comment never carries meaning; documentation is a term |
| [0025](0025-a-module-path-is-the-hosts.md) | A module path is the host's path, and the file is the path |
| [0026](0026-an-int-is-an-integer.md) | An `int` is an integer, and each target realizes what it can |
| [0027](0027-a-host-calls-continuation-is-a-tail.md) | A host call's continuation is a tail position; `again` may sit under a tuple binding |
| [0028](0028-a-definitions-contract-is-checked-at-its-calls.md) | A definition's declared parameter range and `where` are obligations at its calls |
| [0029](0029-above-the-word-one-set-on-every-representation.md) | Above the word, a type denotes a set decided by sign and bit length, and every representation enforces the program's one set |
| [0030](0030-a-hosts-string-is-the-hosts.md) | At a host boundary a string is the host's; ours is Σ*, entered by one total decode |
| [0031](0031-a-builds-result-is-a-product.md) | A `build`'s result is a product, and each buffer in it is frozen |
| [0032](0032-the-ir-is-structured-ssa.md) | The IR is structured SSA with π-parameters, and representation is a type |
| [0033](0033-an-integers-representation-is-chosen-on-the-ir.md) | An integer's representation is chosen on the IR; the checker's integer sort is ℤ |
| [0034](0034-the-rung-above-the-word-is-a-least-fixed-point-on-the-ir.md) | The rung above the word is a least fixed point on the IR, and a bignum destination needs ownership |
| [0035](0035-a-compiler-library-is-a-theory-checked-once.md) | A compiler library is a theory checked once; a use is an instance, inlined on the IR |
| [0036](0036-a-residuals-binders-are-named-apart.md) | A residual's binders are named apart along the scope chain |
| [0037](0037-a-programs-obligations-are-in-its-residual.md) | A program's obligations are in its residual, and nothing reads one undecided |
| [0038](0038-the-irs-text-is-read-not-written.md) | The IR's text is the printers' whole input (IR_P, checked on every program); others may read it, and nobody else writes it until the IR proves index obligations |
| [0039](0039-the-targets-are-go-windows-android-and-the-browser.md) | The targets are Go, Windows, Android and the browser; Node is a third-party layer; supersedes 0004's list |
| [0040](0040-a-failure-is-a-value-of-a-marked-sum.md) | A failure is a value of a marked sum: the compiler knows `success` and `relevant`, `try` is bind, the host's encoding is a niche, `expect` is the one way to give up |
| [0041](0041-a-success-arm-is-a-tail.md) | A success arm is a tail: `again` may sit under `try` and `expect`; a constructor is the tuple (tag, payload) — extends 0015 |
| [0042](0042-a-variants-value-is-its-tag-and-its-slots.md) | A variant's value is its tag and one slot per payload type of its declaration, everywhere it must exist as data; an unselected slot holds `#any`, the zero of its type |
| [0043](0043-relevance-is-checked-where-a-binder-meets-its-value.md) | Relevance is checked at β, where a binder meets its value, on every path (the eliminators are additive), with usage inference through definitions; `ignore` is !_A : A → 1, and η extends to the unit |
| [0044](0044-a-partial-successs-error-is-relevant.md) | A partial success's error is the relevant 1 + E, `(result (tuple) E)`; `(option error)` stays only where the host makes the error ignorable (the builder's writes, the sticky writer's), with the reason stated |
| [0045](0045-a-hosts-empty-interface-is-a-type.md) | A host's empty interface is a type every one-value type is below, derived from its empty method set; a variadic over it is a table of boxes, spread; `fmt` declares one value under Go's name and any number as NAME-All |
| [0046](0046-a-variadic-call-is-its-declaration-applied-to-a-word.md) | A variadic is one declaration under the host's name, f : X × T* → B; a call's trailing arguments are the list, `(spread xs)` passes a table; rewritten by the loader from the target's forms; a variadic statement's value is the unit; the printer writes the host's own call; supersedes 0045's NAME-All |
| [0048](0048-a-host-writes-a-cell-through-a-reference.md) | A host writes a cell through a reference to a fresh copy, `#ref`/`#deref` around the call, an ordinary argument through the retraction, the variadic list and the box; `(cell T)` in declarations, `(cell go.Value)` a cell of any type in Go's box; `(out τ)` a cell for one call, its value the cells' then the result's; the IR's `cell T`; extends 0047 |
| [0047](0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md) | `local` is a scope of local state: the initializer decides buffer (a table) or cell (anything else); cells are loop variables by the state-passing translation in the loader; an elimination it assumes is marked and decided by the reducer on a normal form; a projection of a loop moves into its exits; generalizes 0018's scope and respells `build`'s binder |
