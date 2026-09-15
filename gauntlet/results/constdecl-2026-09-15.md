# `const` in target layers: a constant is the sig it means

2026-09-15. [spec/theories.md §8.3](../../docs/spec/theories.md),
[spec/target-files.md](../../docs/spec/target-files.md).

## 1. What a constant is

theories.md §2's four cells put a declaration by whether it has a **definiens** (an equation the
compiler may use) and a **realization** (a host spelling). A constant is the conditional cell: the
emitter writes the host's name, and the analysis reads the value.

In a target layer that was already expressible, as a zero-argument pure `sig` whose result is the exact
range `[v, v]`:

```lisp
(sig MaxRune () (int 1114111 1114111) pure (host expr "utf8.MaxRune" (import "unicode/utf8")))
```

It states `v` twice, once in each endpoint, and the host's name is a third statement of it. A value
written in several places is several claims that can disagree. `const` writes it once:

```lisp
(const MaxRune 1114111 (host "utf8.MaxRune" (import "unicode/utf8")))
```

**The elaboration E** is one function on declarations:

```
E (const N v (host "s" h…))  =  (sig N () (int v v) pure (host expr "s" h…))
```

So `const` adds nothing to what a target can say. It removes one way of saying a false thing.

## 2. Only an integer is a constant, and that is the definition

A definiens matters only if something reads it. An integer's value is read: `(+ (u.MaxRune) 1)` is
provable because the interval analysis knows `[v, v]`. A float's value may not be folded (ADR 0009),
and nothing analyses a string or a bool. So for those types the definiens is never used, the declaration
is in the host cell, and a zero-argument `sig` already says everything true.

`const` refuses a non-integer literal and names that spelling. The Go generator follows the same line:
integer constants become `const`, and float, string and bool constants stay `sig`. The attached
`host` clause has no kind, because a constant is a value and the only kind a value can have is `expr`.

## 3. Checked as a commuting square

- **Unit level.** `TestAConstIsTheSigItMeans` loads a `const` and its spelled-out `sig`, at the top
  level and inside a module, and requires identical `Prim`s. Five malformed shapes are refused by name.
- **File level.** `targets/go/unicode-utf8.oro` loaded at HEAD and after its four constants were
  respelled: **33 of 33 declarations `DeepEqual`**.
- **Both fail against a planted bug**: an elaboration that drops `pure`. That bug matters,
  because an impure constant is let-bound rather than substituted, and ADR 0010's discipline would then
  treat reading `MaxRune` as an effect.
- **The generator.** The tooling suite regenerates Go's standard library twice, counts `const` lines as
  declarations (`primLine`), and checks every hand declaration against the generated host types.

Every declaration path — top level, `(module …)` and `provides` — goes through `declare` and so through
`parseSig`, which elaborates `const` first. So the three positions agree by construction, not because
each was taught separately.

## 4. Cost

- **Code**: `emit/target.go` 2,299 → 2,324 (+25, most of it the refusal messages), `survey.go`
  1,441 → 1,446.
- **`go run ./cmd/check`, every step, passes**: emission 194 of 194 files byte-identical, proof counts
  identical (2,307 of 2,413 integer operations, 345 of 382 loops), differential on four targets, and
  tooling, which regenerated every survey twice and checked the hand declarations against the host.

## 5. Not built

- `const` in a **program** module. There it has a `def` and no realization, so it is the defined cell,
  and today's `(def N 5)` already is that. What it would add is the declared exact range at a boundary,
  which no program has asked for.
- A constant's **name as a range endpoint**, `(int 0 utf8.MaxRune)` (theories.md §8.3). The endpoint
  evaluator sees literals and `pow`, and a target's constants live in another layer from the program's
  signature.
