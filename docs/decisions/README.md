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
