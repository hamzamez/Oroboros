# 0005 — The compiler is written in Go

Date: 2026-08-13
Status: Accepted

## Context

The compiler needs an implementation language. Go was the leading candidate, and is also the
first target ([0004](0004-first-targets.md)).

## Decision

**Go.**

The IR is represented as a **flat tagged struct with a kind enum and index-based children**,
not an interface hierarchy.

## Why not

**Rust.** Better suited to compilers — real sum types, `match`, exhaustiveness checking. But
slow builds and high friction, and this is a project that gets deliberately put down at dead
ends and picked up later. Friction raises the probability of a dead end and the cost of
returning.

**OCaml or Haskell.** Best in class for writing compilers. Worse for single-binary
distribution, smaller ecosystems, and substantially weaker LLM support — which requirement 8
makes a real consideration.

**Self-hosting in v1.** The eventual goal, but bootstrapping before the language exists is a
reliable way to stall.

## Consequences

**Gained:** single-binary distribution, which matters more than it sounds for a tool people
are meant to download and run; trivial cross-compilation via `GOOS`/`GOARCH`, so the compiler
runs everywhere for free; fast builds; a standard library that covers what a compiler needs;
and heavy representation in LLM training data.

**Lost:** sum types. An IR is a sum type and Go makes you emulate one. The flat-tagged-struct
representation is the mitigation — simpler than an interface hierarchy, better cache
behavior, and trivially serializable, which enables [0006](0006-ir-file-format.md).

**Watch for:** Go's lack of exhaustiveness checking means adding an IR node kind will not
produce compile errors at the places that must handle it. Compensate with a single registry of
node kinds and a test that every backend handles every kind.
