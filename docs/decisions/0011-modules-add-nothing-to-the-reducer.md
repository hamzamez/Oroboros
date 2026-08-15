# 0011 — Modules are resolution, not reduction

Date: 2026-08-15
Status: Accepted

## Context

Three times the absence of a library mechanism pushed something into the core that did not belong
there: `seq` went into the reader, `print-line` into every target file separately, and the string
literal into the language to serve target templates.

> Without a library mechanism, "put it in a library" is not an available answer, so every pressure
> to grow lands on the language.

A small core cannot be maintained by intention; it needs somewhere else for things to go.

## Decision

Modules are **scopes with qualified names, resolved before reduction**. `core/reduce.go` gained no
reduction rule, no term kind, and no second parameter to normalisation.

- `.` is the qualifier separator; `/` stays an ordinary identifier character, so a module path is
  one token.
- `(module PATH)`, `(use PATH [as ALIAS])`, `(export …)`. Imports stay **qualified**.
- A target declares into module namespaces; it may provide **any subset**, including none.
- **R1 — one namespace.** Targets and libraries name into the same qualified namespace.
- `(use …)` resolves against a search path, so a module may live in its own file. A path with no
  file is not an error — it is one the target provides.

## Why not

**Flat (unqualified) imports.** They make a program's meaning depend on import order and on which
names a target happens to provide — both of which change under exactly the conditions this system
exists to make cheap.

**Separate namespaces for targets and libraries.** This is the one that would have failed
silently. The conditional lowering is `P_T ∩ D`; namespacing the two apart makes that intersection
permanently empty, so every program would quietly get the portable fallback and never the native.
The rule exists because the specification was written before the code.

**Functors (ML's parameterised modules).** Our parameterisation is the target, and it is already
the parameter to reduction. A functor would be a second, competing mechanism for the same job.

**A new form for conditional lowering** — an `ifdef`-shaped construct selecting an implementation
per target. An earlier draft of the design claimed N targets need N bodies. That was wrong: N
natives plus one fallback falls out of the four cells of `P_T` and `D` with no new syntax, which is
the same mechanism `examples/dot.oro` has used since the first commit, read in the other direction.

**Separate compilation.** Reduction is whole-program by construction and fusion crosses every
boundary a module would draw. Whether that is a scaling problem is a measurement, not a design
question, and it has not been taken.

**Blessing a `std` namespace.** A module's tier is whether it carries a **signature**, not what it
is called. That removed a concept and lets the library emerge and acquire signatures where it earns
them.

## Consequences

Emitted functions are named after their exports instead of by position. Programs span files;
`examples/dot.oro` and `examples/report.oro` now share `lib/num/vec.oro` instead of duplicating it,
and emit byte-identical source.

The covering check proves a name is *provided* and **can never prove it is right**. `split-words`
satisfied every check for two months while returning different answers on Go and JS. Every Tier 1
name therefore needs a conformance suite ([gauntlet/conformance/](../../gauntlet/conformance/)) or
it is decoration.

Still unbuilt: a target is one file rather than a directory, which the parasite thesis will
eventually outgrow. Overload resolution is unsolved and is a *type* problem, not a module one.
