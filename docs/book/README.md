# The book

Chapters, in order. Each is written for someone learning the language — human or model — and
leans on examples rather than explanation.

**Every reduction shown in a chapter is real output.** The examples run against
[`targets/tutorial.oro`](../../targets/tutorial.oro), a target that exists to teach: it spells
arithmetic `+ - * /` so a chapter need not also be about module paths, and it declares `f`, `g`,
`h`, `x`, `y`, `z` as primitives so reduction halts exactly where a lesson wants it to.

```bash
go run ./cmd/oro -target=tutorial FILE.oro
```

That a teaching target is *possible* is the thesis in miniature: the normal form is a parameter, so
choosing where reduction stops is choosing a target.

| | |
|---|---|
| [1. `fn`](01-fn.md) | functions, parameters, binding, shadowing, capture, what survives |
| [2. `def`](02-def.md) | naming a term, δ, why a definition duplicates, why a primitive wins, recursion, and what λ-calculus alone can do |

Planned: modules, effects, the type system, targets.

## For the writer

The specifications are in [docs/spec/](../spec/) and are the authority. A chapter must not
contradict one, and where a chapter simplifies it should say so. Writing chapter 1 found two real
bugs — a parameter list could repeat a name, and could bind a qualified one. Chapter 2 found four
more: `(def a.b …)` was accepted and unreachable, an `export` or a `sig` naming nothing was
silently dropped, and two diagnostics printed `#1.0` instead of the names the source used. That is
six bugs from two chapters, which is a reasonable argument for writing more of them.

A chapter is also allowed to teach something that is not about this language. Chapter 2's last
section is Church encodings, because the compiler's own vector type turns out to be one.
