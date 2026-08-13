# 0006 — The backend interface is a file format

Date: 2026-08-13
Status: Accepted

## Context

Requirement 3 is that adding a new target must be low effort and doable by a third party. The
default design would be a Go interface that each backend implements, compiled into the
compiler binary.

That makes every backend author a Go programmer working inside this repository.

## Decision

The interface between the compiler and a backend is a **specified, serialized IR file format**,
not a Go interface.

A backend is any program that reads IR and emits target code. It may be written in any
language, live in any repository, and ship on any schedule.

The in-tree Go backends are simply the first consumers of that format, with no privileged
access.

## Why not

**A Go interface.** Requires backend authors to write Go, to build the compiler, and in
practice to get their backend merged upstream. Every one of those is friction against the
stated requirement.

**A plugin system (Go plugins, WASM, dynamic loading).** More machinery, platform-specific
loading problems, version coupling between plugin and host — and it still constrains the
implementation language. A file format has none of these problems.

## Consequences

- The IR format specification becomes the **most consequential artifact in the project**, and
  should be written before the compiler. It is the thing that is hard to change later.
- Backends become independently testable: capture IR, run the backend, diff the output. No
  compiler needed in the loop.
- The IR is dumpable for inspection and diffing, which is directly useful for LLM tooling and
  for debugging lowering decisions.
- Versioning becomes a real obligation. The format needs a version field and a compatibility
  policy from the first release, since third-party backends will lag.
- Some coupling is unavoidable: the capability graph ([0002](0002-capability-graph.md)) means a
  backend must also declare what it provides. That declaration is data, and belongs in the same
  format.
