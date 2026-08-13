# 0007 — Explore candidates against a fixed test, rather than specifying the core first

Date: 2026-08-13
Status: Accepted

## Context

[ADR 0006](0006-ir-file-format.md) says the IR format specification "should be written before
the compiler," and the design direction opened with "freeze the core on paper."

That advice guards against **drift** — exploring forever without converging. It is the wrong
guard for this project. The predecessor stalled for the opposite reason: a fixed language that
work then went into making viable. Committing to a specification now recreates that exact
failure, one project later.

But pure exploration has its own failure mode, and it is the one that actually stopped Shen:
the performance wall was discovered late, after years of investment.

## Decision

**Fix the test, not the language.**

The [gauntlet](../gauntlet.md) — five programs, three targets, parity with hand-written code —
is the project's one fixed commitment. Candidate cores are disposable and are expected to be
killed.

The first artifact is the gauntlet's hand-written reference implementations and baseline
numbers, not a specification.

Every candidate that dies gets an ADR naming what killed it.

## Why not

**Freeze the core, then implement it.** This is the Shen failure mode. A frozen specification
becomes something to make work rather than something to abandon when it stops earning its place.

**Explore with no fixed criterion.** This is the other Shen failure mode. The wall is found
late, and by then the investment is too large to walk away from cheaply.

**Specify the IR format first.** A serialization format cannot be designed before it is known
what flows through it. The decision in [0006](0006-ir-file-format.md) — that the backend
interface *is* a file format — still stands; only its sequencing claim is revised here. The
format gets written once a candidate core has survived the gauntlet.

## Consequences

- The next artifact is the gauntlet, not a specification. Roughly a day of work, reusable
  across every candidate for the life of the project.
- Dead ends become recorded results instead of lost time. Given that this project is
  deliberately put down and picked up later, the ADR trail is what prevents re-exploring the
  same failure.
- A candidate is killed by measurement, not by argument. Arguments select what is worth
  measuring.
- **Risk:** the gauntlet is only as good as its five programs. If they fail to represent real
  workloads, a core can pass and still be wrong. Programs 1 and 4 carry most of the weight —
  boxing and the Parasite thesis respectively — and should be reviewed if a candidate passes
  suspiciously easily.
