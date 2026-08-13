# 0009 — Staging must not change results

Date: 2026-08-13
Status: Accepted

## Context

[The escaping-closure derivation](../derivations/g6-escaping-closures.md) established what the
candidate core actually is: a **two-level language** whose compile-time level is a partial
evaluator. Abstraction is free because beta-reduction happens before the program runs; what
survives into the residual is what the target must do at runtime.

That identity depends on a property nobody had stated: **moving a computation between stages
must not change its answer.**

The [baseline run](../../gauntlet/results/baseline-2026-08-13.md) found that Go violates it:

```
const-folded  0.1+0.2 = 0.3
runtime       a+b     = 0.30000000000000004
const-folded  -0.0    = 0
runtime       -a*0    = -0
```

Go evaluates untyped float constants at arbitrary precision and rounds once at the end. The
same expression evaluated at runtime uses IEEE-754 binary64 throughout.

If Oroboros inherited that, **binding-time analysis would change program results.** Whether an
abstraction was eliminated at compile time or survived to runtime would be observable in the
output — which makes partial evaluation unsound, and makes the guarantee proposed in
[g6 §9](../derivations/g6-escaping-closures.md) ("this fold is eliminated, that one survives") a
statement about semantics rather than only about cost.

## Decision

**Compile-time evaluation must produce bit-identical results to runtime evaluation.**

Concretely:

- Compile-time float arithmetic is **IEEE-754 binary64, exactly** — the same as runtime. No
  arbitrary-precision constant folding, no extended intermediate precision, no contraction of
  multiply-add.
- Signed zero and NaN are preserved through folding.
- The same applies to every operation whose result could differ by stage: integer division and
  modulo on negative operands, shift semantics past the width, and float-to-string conversion.
- Any operation that cannot be guaranteed stage-invariant may not be folded at compile time.

**This is a correctness property of the core, not an optimization setting.** It is not
adjustable by a flag.

## Why not

**Arbitrary-precision constants, as Go and Scheme have.** More accurate, genuinely nicer for
writing literals, and unsound here. The accuracy gain applies only to expressions the compiler
happens to fold, which makes the *value* depend on the *staging decision*. That is exactly the
thing that must not happen.

**Ban compile-time float arithmetic entirely.** Sound, and it removes partial evaluation from
all numeric code — which is where the performance argument lives. Too expensive.

**Treat it as a fast-math-style opt-in.** Any flag that makes staging observable reintroduces
the unsoundness for the programs that set it, and the guarantee is worth more than the accuracy.

## Consequences

- **The compiler must not use Go's untyped constants when implementing its own folder.** Every
  compile-time float operation must be forced through explicit `float64`. This is unusually easy
  to get wrong, precisely because the compiler is written in Go
  ([ADR 0005](0005-implementation-language.md)) and the natural way to write it is the wrong
  one.
- It needs a test, not a convention: fold an expression at compile time, compute it at runtime,
  assert bit equality. That test belongs in the gauntlet.
- It interacts with [g1's](../derivations/g1-dot-product.md) decision that `sum` is
  left-to-right. Both are instances of the same commitment — **the answer does not depend on how
  the compiler chose to get there** — and that commitment now has a measured price of 5–7×
  (see [baseline U1](../../gauntlet/results/baseline-2026-08-13.md)).
- A target whose native arithmetic cannot honour binary64 semantics cannot host compile-time
  float folding, and must receive the unfolded residual.
