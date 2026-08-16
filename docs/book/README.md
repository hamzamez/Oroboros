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
| [3. modules](03-modules.md) | `module`, `use`, `export`, the four cells, one namespace for targets and libraries, and why a functor is just a function |
| [4. effects](04-effects.md) | one declared bit, the β side condition, `seq`, and the substructural logic it turns out to be |

Planned: the type system, targets.

Chapter 3's library files are real and live in [code/](code/). Its examples run against two
targets — `tutorial` and `tutorial-native`, which is the same file plus one capability — because
the four cells cannot be shown with only one.

## For the writer

The specifications are in [docs/spec/](../spec/) and are the authority. A chapter must not
contradict one, and where a chapter simplifies it should say so. Writing chapter 1 found two real
bugs — a parameter list could repeat a name, and could bind a qualified one. Chapter 2 found four
more: `(def a.b …)` was accepted and unreachable, an `export` or a `sig` naming nothing was
silently dropped, and two diagnostics printed `#1.0` instead of the names the source used. That is
six bugs from two chapters. Reviewing the finished chapter then produced a seventh finding and an
ADR: recursion reduced but could not build, which is a promise the language does not keep, so it
is now rejected outright (ADR 0014).

Chapter 3 found four more: a library file's extra modules were visible only after something else
imported it — load-order-dependent meaning, which is the one thing a module system exists to
prevent — and three diagnostics named the wrong half of a qualified name. Chapter 4 found one
more, and a good one: the emitter recomputed a statement's value instead of binding it, so
`fmt.Println((strings.Fields(s)))` was followed by `return (strings.Fields(s))` — the exact
duplication the chapter is about, one layer below the chapter. Eleven bugs from four chapters.

A chapter is also allowed to teach something that is not about this language. Chapter 2's last
section is Church encodings, because the compiler's own vector type turns out to be one; chapter
3's is ML functors, because it turns out we already have them and did not notice; chapter 4's is
substructural logic, because the effect rule derived from three concrete hazards turns out to be
Lambek's ordered fragment, named in 1958.

Chapter 4 also uses [targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro) —
`targets/tutorial.oro` with the word `pure` deleted from one primitive — so the cost of a
forgotten purity marker can be shown rather than asserted.
