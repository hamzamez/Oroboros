# A scoped buffer is a binder: `(build b n body)`

2026-09-25. hamza asked:
- should `(build n (fn (b) body))` be written `(buffer b n body)`?
- can several buffers share one form?
- what does a `build` return?

Spec: [tables.md §2.4](../../docs/spec/tables.md) (the form), §14.1 (the escape argument,
corrected), [binding.md §6b](../../docs/spec/binding.md), [maps.md §3.3](../../docs/spec/maps.md).
Decision on the result: [ADR 0031](../../docs/decisions/0031-a-builds-result-is-a-product.md), not
built.

## 1. What `build` is

```
build : (n : ℕ) → (Buf_n V ⊸ R) → R'
```

It opens a scope with one fresh, zero-filled, linear buffer, and freezes it on exit without a copy
(ADR 0018). The λ in the core form is never a function value: §14.1 accepts a λ only where a backend
consumes it structurally. So it is a binding occurrence written as a λ, which is higher-order abstract
syntax (Pfenning & Elliott 1988). `let` had already made the move this makes: it writes
`((fn (x) b) e)` as `(let x e b)`.

- **Unary:** `(build b n body) ≡ (build n (fn (b) body))`.
- **N-ary:** `(build b₁ n₁ … bₖ nₖ body) ≡` the right fold, which is sequential, like `let*`.
- **Several buffers are one region.** Nested scopes that end together have equal lifetimes, so the
  flat form is equal to nesting (Tofte–Talpin `letregion`). So the names are one parameter list, and a
  repeated name is refused as `(fn (x x) …)` is.
- **`build-map`** is the same construct at another index set, so it has the same surface.
- **The keyword stays `build`.** The form's value is a table, and introduction forms are named after
  what they produce (`array`, `tuple`, `map`, `table`). `(buffer V)` is already a type, and the reader
  is context-free.

## 2. What was measured first

**The corpus.**
- **56 `build`s and 4 `build-map`s** in programs, plus 7 in the compiler's own `.oro` sources. Every
  one wrote the λ literally; none passed a named filler.
- **5 nested pairs**, each one level deep: msort in `freq` and `tally`, the `merge-sort` case, and the
  parser in `tree` and `json-tree`. Each returns one buffer, and the other is scratch space.
- **`tree`'s parser returns a count and an ok flag in slots 0 and 1 of its node table.** That is a
  product encoded by hand into an array, because a `build` has one way out.

**What a `build` returns** (probes, `gen -checked`, all four targets):

| body returns | result |
|---|---|
| its buffer | the frozen table |
| an `int` | **accepted on all four**, although the spec typed the body `Buf V ⊸ Buf V` |
| an enclosing scope's buffer | accepted, as a move: a later store or read through the old name is refused |
| a rule-table capturing the buffer | refused, by *a rule-table has no memory*, not by the type §14.1 named |
| `(tuple b i)` under a tuple pattern | **internal error on JS and windows** ("application of a non-name"); Go loses `len t` |
| a `loop` whose exit is a tuple, under a tuple pattern, with no buffer at all | **internal error on all four** |

So the specification's type was never enforced, and §14.1's escape argument rested on a check that did
not exist. The escape is prevented anyway, by a different refusal. And a product cannot leave a loop or
a scope. ADR 0031 decides both: a `build`'s result is a product of frozen buffers and buffer-free
values; the eliminator commutes into a scope and out of a loop (a join point); and each component's
facts hold of its binder, which is the tuple-component law on its fourth meeting.

## 3. What is built

- **`core/scope.go`:** the binder form, erased in the reader to the core form, term for term. Arity
  decides, as it does for `def`:
  - 2 elements is the core form;
  - an odd number, at least 3, is the binder form;
  - an even number, at least 4, is refused;
  - 1 is a target file's `(build "…")` directive, left alone.
- **Every program respelled.** 67 forms in 33 `.oro` files: examples, `lib`, the differential cases,
  the acceptance programs and the compiler's own `emit/*.oro`. Every comment is unchanged, checked by
  comparing each file's comment text before and after. So is every file's line count (255 lines out,
  255 in). The five nests became the n-ary form:

  ```lisp
  (build a n
         b n
    (loop ((a (iota a n)) (b b) (w 1))
      …))
  ```
- **The README's two programs** (the sieve and `encoding/hex`), respelled and run again. The sieve
  prints 168 on Go, JS and Java; hex prints 686921 on Go, as the page says.
- **The docs.** The language's current documents now use the binder form:
  - tables.md §2, §2.4, §9, §14.1–§14.3; binding.md; maps.md; state.md; postconditions.md;
  - array-facts, closures-direction, type-algebra and theories, where the analyses are said to see the
    core form;
  - README and CLAUDE.md.

  Decision history, derivations, results and the research behind ADR 0018 keep the core form, which is
  still legal.

## 4. Checked, and how each check fails

`core/scope_test.go`:
- the binder form reads as the core form, for one, two and three pairs, for `build-map`, and with a
  comment inside;
- the scope is sequential, **checked on the structure**: the second size's `a` must be a bound
  variable, because a printed term shows `a` either way;
- the core form and the directive are untouched;
- four refusals, each naming its rule.

| planted in `scope.go` | caught by |
|---|---|
| pairs folded in the wrong order | the core-form test and the sequential test |
| names checked one at a time (repeats allowed) | the refusal test |
| an even arity accepted | the refusal test |
| no binder form at all | all three reading tests |

**`go run ./cmd/check`**: every step passes, including tooling, which runs the respelled acceptance
programs. **Emission is byte-identical** over 508 compiles after 67 forms were respelled. That is the
measurement that the equation of §1 holds of the implementation.

## 5. What failed first

- **The respell tool read `()` as a list head.** It crashed in `freq.oro` on `(def main ()`. Three
  files had already been written correctly, and the rewrite is idempotent, so a rerun finished the
  job.
- **I took the README's code blocks for the programs by position**, and the numbering missed an
  earlier block. So the first run compiled the wrong two programs. They are now found by content.
- **The first probe of a tuple result looked like a `build` problem.** It is a loop problem first:
  the same tuple out of a plain `loop` fails on all four targets. That moved ADR 0031's lowering from
  "into a scope" to "into a scope and out of a loop".

## 6. What it cost

- **Code:** `core/scope.go` (53 lines) and its test (85). No change below the reader.
- **Emission** byte-identical; compile time 1.05× the baseline (noise).
- **The balance:** +53 lines of compiler against 255 respelled lines of programs, which grew nothing.
  The program that ADR 0031 unblocks, the parser returning `(tuple nodes nn ok)`, waits for that ADR's
  build.

## 7. Not built

- **ADR 0031.** Product results from a scope and a loop, the three refusals (R1–R3), the component
  length, and a join point per backend. `tree`'s header slots stay until it is built.
- **The respell tool** lives in the session's scratchpad, not in `cmd/`. The rewrite was done once,
  and the core form stays legal, so nothing has to be migrated again.
- **Go test sources that embed programs** (`emit/*_test.go`) keep the core form. They test the core,
  and the core form is what the residual prints.
