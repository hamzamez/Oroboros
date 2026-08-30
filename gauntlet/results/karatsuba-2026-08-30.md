# Karatsuba without recursion — 2026-08-30

[bigarith-2026-08-28 §8a](bigarith-2026-08-28.md) claimed:

> *We cannot fix it, because Karatsuba needs recursion. ADR 0014 removed recursion, and
> divide-and-conquer is the shape it removed.*

hamza refused it — *"I am sure we can write it using loops if we wanted"* — and **he is right. The
claim is withdrawn.** `gauntlet/go/karatsuba.go` is Karatsuba with no recursion, no explicit stack,
and every buffer sized before the first loop runs.

---

## 1. Why it works, and why I thought it did not

The explicit-stack trick in `examples/json/` answers a **traversal**, where a node returns nothing to
its parent. Divide-and-conquer needs each node to hand a **value** up, and that is the part that
looked like it needed either recursion or a second, value-carrying stack.

**It needs neither, because Karatsuba's recursion tree is balanced and data-independent.** Its shape
is a function of `(n, D)` alone — the split points are `n/2`, `n/4`, … and nothing about them depends
on the numbers being multiplied. So the tree can be **laid out in advance and walked level by level**,
bottom-up, and the value never has to travel up a call chain because the parent simply reads the
slots its children wrote.

That is the same reason an iterative FFT exists: Cooley-Tukey is usually written recursively and
is always *implemented* as loops over levels.

> **A balanced, data-independent recursion is a loop over levels.** That is the general statement,
> and it is more useful than this one algorithm.

## 2. The shape

Three nested loops — level, node, limb.

```
level L has 3^L nodes, operands of s[L] = (n >> L) + pad limbs, split at h[L] = n >> (L+1)

DOWNWARD   each node's three children are  (lo, lo'), (hi, hi'), (lo+hi, lo'+hi')
BASE       schoolbook on every node of the deepest level
UPWARD     product = z1·B² + (z2 − z0 − z1)·B + z0,      B = 2^(64·h)
```

The `pad` is what keeps every node at a level the same size: `lo+hi` needs one limb more than a half,
and `D+2` limbs of slack absorb every level's worth of that. Uniform slot sizes are what make the
whole workspace one `(n, D)` computation — which is what `build` requires.

**The subtraction never goes negative**, because the two subtractions follow the three additions and
`z2 ≥ z0 + z1` by construction, so every borrow resolves inside the buffer.

`math/big` is the oracle, checked at n = 16…256 and every depth D = 0…4.

## 3. What it is worth

Go 1.26, `-benchtime=1s -count=3`, medians.

| n = 1,024 limbs | ns/op | vs schoolbook |
|---|---|---|
| schoolbook (D = 0) | 1,151,241 | — |
| Karatsuba D = 3 | 538,252 | 2.14× |
| **Karatsuba D = 5** | **469,089** | **2.45×** |
| Karatsuba D = 7 | 925,039 | 1.24× |
| `math/big` | 119,991 | 9.6× |

| n = 256 limbs | ns/op | vs schoolbook |
|---|---|---|
| schoolbook (D = 0) | 71,720 | — |
| **Karatsuba D = 2** | **46,751** | **1.53×** |
| Karatsuba D = 4 | 51,470 | 1.39× |
| `math/big` | 12,829 | 5.6× |

**It closes most of the gap and does not close all of it.** At 1,024 limbs we go from **9.53× behind
`math/big` to 3.91×**; at 256 limbs from 5.48× to 3.64×. §3a takes it further.

**The optimal depth is finite**, and that is the honest limitation of this implementation rather than
of the approach. Theory says D = 5 should be `(3/4)^5 = 0.237` of schoolbook — 4.2× — and it measures
2.45×, so about 40% of the theoretical gain is eaten by per-level bookkeeping: this version copies
operands into fresh slots at every level, where a recursive implementation works in place. D = 7 is
slower than D = 5 because that overhead grows as `3^L` while the saving shrinks as `(3/4)^L`.

**What remains after Karatsuba is not the algorithm.** `math/big`'s inner loop is hand-written
assembly with ADX/MULX, and it switches to Toom-Cook above Karatsuba's range. Those are worth roughly
the 3.9× that is left, and neither is a consequence of ADR 0014.

## 3a. In place, without copying — and the depth goes deeper

hamza: *"now do the level-by-level layout in place, without copying."* `karatsuba2.go`.

**Two of the three children are subranges of the parent.** `(a_lo, b_lo)` and `(a_hi, b_hi)` are
already in memory; only `(a_lo+a_hi, b_lo+b_hi)` is new data. So a node is an **offset and a length**,
not a buffer — and the tree becomes a flat descriptor table `(aOff, bOff, len)` over one arena
holding the two inputs and the sum buffers, which is this repository's own answer to recursive data
arriving in a third place.

| n = 1,024 limbs | ns/op | vs schoolbook | vs `math/big` |
|---|---|---|---|
| schoolbook | 1,152,062 | — | 9.53× behind |
| Karatsuba, copying, D = 5 | 470,807 | 2.45× | 3.91× behind |
| **Karatsuba, in place, D = 5** | **337,535** | **3.41×** | **2.78× behind** |
| `math/big` | 121,546 | 9.5× | — |

| n = 256 limbs | ns/op | vs schoolbook |
|---|---|---|
| schoolbook | 71,914 | — |
| Karatsuba, copying, D = 2 | 47,043 | 1.53× |
| **Karatsuba, in place, D = 4** | **34,402** | **2.09×** |

**1.40× over the copying version at 1,024 limbs and 1.37× at 256**, and it reaches **81% of the
theoretical `(3/4)^D`** where the copying version reached 58%.

**And the optimal depth moved deeper**, which is the clearest evidence the diagnosis was right: at
n = 256 the copying version peaked at D = 2 and got worse at D = 4, while the in-place one is still
improving at D = 4. Cheaper levels mean more of them pay for themselves. At n = 1,024 the peak moved
from D = 5 to D = 5 with D = 7 now only 1.15× off instead of 1.96×.

**What is left is not copying.** The sum child is genuinely new data, and the upward combine's three
adds and two subtracts are the algorithm. Against `math/big` the residual 2.78× is close to the ~2×
its hand-written ADX/MULX inner loop shows at sizes where neither side is doing Karatsuba at all —
which is the honest reading: **we have most of the algorithm and none of the assembly.**

### One bug, and it was in the sizing

The first in-place version panicked at n = 16, D = 1 — a product buffer of 36 limbs where the write
wanted 38. A parent's product must reach `2h + (a child's product)`, and a flat slack of `+4` is not
that. `karatsuba.go` got it right by accident: its uniform padding makes `2h + 2·s[L+1]` come to
exactly `2·s[L]`. The ragged version has to compute the sizes bottom-up, which it now does exactly.

## 4. What this corrects

**[ADR 0014](../../docs/decisions/0014-recursion-is-not-in-the-language.md)'s consequence, added the
day before, is wrong as written and is corrected in place.** The price of having no recursion is not
*"Karatsuba is unreachable"*. It is:

- the divide-and-conquer must be written **level by level**, which is more code — about 170 lines
  here against maybe 30 recursive;
- the tree must be **materialised** as a descriptor table, and the sum operands with it, so the
  workspace is superlinear where a recursive implementation reuses one scratch buffer;
- and the per-level work still caps the useful depth, though §3a's in-place layout pushes that cap
  out and takes the shortfall against theory from 42% to 19%.

That is a real price and a much smaller one, and it is the same shape as every other price this
project pays: **the restriction makes the cost visible and the programmer state it**, rather than
making the algorithm impossible.

**And it sharpens what ADR 0014 actually forbids.** Not divide-and-conquer — divide-and-conquer whose
*tree shape depends on the data*. Mergesort, FFT, Karatsuba and binary search all have statically
known shapes and are loops over levels. Quicksort's pivot, a search that prunes on what it finds, and
anything whose recursion depth is decided at run time are the cases that genuinely need the stack —
and `examples/json/` shows even those are reachable when a *traversal* is enough.

## 5. What this does not change

The **design conclusion of bigarith-2026-08-28 stands**: at 1,024 limbs we are still 3.9× behind
`math/big`, and on V8 `BigInt` was 148× ahead at 16,384 bits, which Karatsuba would dent but not
close. **R3 is still ours for the linear operations and the host's for big × big past a small
threshold**, and [ADR 0019](../../docs/decisions/0019-precision-by-declaration.md) is unaffected.

What changes is the *reason*: we call the host's multiply because its inner loop is assembly and its
asymptotics go further, **not** because the language cannot express the algorithm. That is a better
reason, and it is one that could be revisited if a target ever declared a fused multiply-accumulate
worth building on.
