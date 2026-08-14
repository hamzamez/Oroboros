# Call-by-need — 2026-08-14

The gap [word count](wordcount-2026-08-14.md) measured at 615× on Go and 1,089× on JS, closed.

**Result: word count reaches parity on both targets, fusion survives, and the rule turned out
simpler than the design proposed — no grades, no cost model, no per-primitive annotation.**

---

## 1. The rule

At β on `((fn (p) b) a)`:

| occurrences of `p` in `b` | action |
|---|---|
| 0 or 1 | **substitute** — duplication is impossible |
| ≥ 2 | normalise `a`, then: literal, variable, or **λ** → substitute; anything else → **bind** |

The residual of a declined substitution is `(let a' (fn (p) b))` — the primitive from
[def.md §6](../../docs/spec/def.md), so β does not apply to it and the normal form definition
needs no weakening.

`let` is primitive on **every** target without being declared. Every target has local bindings;
it is not a capability question.

## 2. The part that is not obvious

**A duplicated λ must be substituted, or fusion dies.**

It is tempting to treat an abstraction as expensive to copy — it may become a closure. But in the
dot product, `sum`'s parameter `v` occurs twice and is bound to the zip term, which normalises to
a λ. The two copies then reduce to *different, small* things — `(alen p)` and a multiply — and the
intermediate vector disappears. That is the entire fusion mechanism.

Copying a λ that does *not* reduce away costs code size, which is the measured
[specialize-versus-outline tradeoff](size-2026-08-13.md), not a correctness problem. So λ is
duplicable, and the check that this is right is `TestSameTermTwoNormalForms` still producing g1's
residual.

## 3. Why the rule needs no cost model

The design in [concerns.md §1.1](../../docs/spec/concerns.md) proposed deciding by *grade* —
whether the duplicated term survives to runtime — and noted the atom had no grades. It does not
need them.

Two measurements bracket the decision:

- Duplicating a **pure** term costs nothing ([the CSE finding](duplicate-read-2026-08-14.md)):
  three variants of the filter loop compiled to byte-identical machine code.
- Duplicating an **allocating** term costs 615× and grows ([word count](wordcount-2026-08-14.md)).

So over-binding is free and under-binding is unbounded, and the conservative choice — bind
anything that is not obviously duplicable — is correct without knowing which primitives are
expensive.

**Occurrence counting plus a four-case syntactic test on the normalised argument. That is all.**

## 4. Measured

**Go, word count, n=2000 words:**

| | ns/op |
|---|---|
| hand-written | 126,447 |
| generated, before | 46,587,398 |
| **generated, after** | **119,743** |

Structurally confirmed, which after [the alignment finding](duplicate-read-2026-08-14.md) is the
evidence that counts:

```
GenWordcount   1 strings.Fields   1 runtime.makemap_small   1 runtime.mapassign_faststr
```

**One** call to `strings.Fields` — hoisted out of the loop — and one hash lookup per word. That
is exactly the hand-written shape.

**JavaScript, word count, n=2000:**

| | µs/op |
|---|---|
| hand-written | 195.2 |
| generated, before | 214,959 |
| **generated, after** | **234.0** |

A 900× improvement. The remaining 1.2× is small enough to be measurement variance on this
harness but has **not** been confirmed structurally, and is the one number here that should not
be quoted as parity.

**The other three programs, both targets** — unchanged, which is the point:

| n=1024 | Go hand-written | Go generated | JS hand-written | JS generated |
|---|---|---|---|---|
| dot | 470–595 ns | 560–638 ns | 505.4 ns | 501.0 ns |
| filter | 602–618 ns | 636–647 ns | 545.9 ns | 500.4 ns |
| centroid | 498–579 ns | 498–502 ns | 481.2 ns | 494.4 ns |

The Go figures are noisier than earlier runs; per the alignment finding these spreads are not
evidence of anything and the machine-code diff is what settles parity.

## 5. What it closed

`TestFilterFusesToOneLoop` spent two days asserting the **wrong** answer on purpose.
[concerns.md §1.1](../../docs/spec/concerns.md) recorded that the specification and the
implementation disagreed and that the test encoded the implementation's answer. It now produces:

```lisp
(fold-range 0.0 (alen a) (fn (acc i) (let (aindex a i) (fn (x) (if (pos x) (add acc x) acc)))))
```

which is exactly what [q5b §3](../../docs/spec/q5b-filter.md) derived on paper before any of this
existed. **The spec and the code agree again.**

## 6. What is still open

- **Single use inside a loop is not caught.** `(fn (ws) (fold-range 0 n (fn (acc i) (slen ws))))`
  uses `ws` once, so it is substituted, and the term then runs *n* times. Occurrence counting is
  syntactic and does not know which λs become loop bodies. Contrived, not yet observed in a real
  program, and per [ADR 0007](../../docs/decisions/0007-exploration-over-specification.md) it
  waits for a measurement rather than a fix.
- **Effects are still absent**, so [g5](../../docs/derivations/g5-bindings.md)'s ordering
  discipline — a term moved into a loop or a conditional arm firing the wrong number of times —
  has no implementation. Call-by-need addresses duplication, not ordering.
- **The JS 1.2× is unexplained.** Structural verification on JS is harder than on Go and was not
  attempted.

## Reproducing

```bash
go test ./core/                                     # 17 tests, spec examples
cd gauntlet/go && go test -bench='WC' -benchtime=2s -count=3
cd gauntlet/js && node wc.mjs && node parity.mjs
```
