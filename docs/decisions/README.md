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
