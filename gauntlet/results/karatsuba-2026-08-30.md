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

## 3b. Java and JavaScript

Same shape ported to both: descriptor table over one arena, two of three children as offsets.
64-bit limbs on Java (bigarith §8c found L64 beats L31 for big x big); base 2^15 on JavaScript,
because [§5a](bigarith-2026-08-28.md) found the constraint there is int32, not 2^53.

| Java, 65,536 bits | ns/op | | JavaScript, 16,384 bits | ns/op |
|---|---|---|---|---|
| schoolbook | 1,914,360 | | schoolbook | 1,575,741 |
| **Karatsuba D = 6** | **533,505** | | **Karatsuba D = 4** | **710,776** |
| `BigInteger` | 340,367 | | `BigInt` | 22,866 |

**Java 3.59x over schoolbook**, and 5.62x behind `BigInteger` becomes **1.57x**.
**JavaScript 2.22x**, and 68.9x behind `BigInt` becomes **31.1x**.

Java is where this pays most; JavaScript is where it changes least — 31x behind is still a rout, and
no amount of Karatsuba closes a gap that is mostly 15-bit limbs against V8's 64-bit ones in C++.

### The JavaScript measurement was wrong twice, and bigarith §8 is corrected

Two method errors, found by re-measuring rather than by suspicion.

**A fixed iteration count gave a 4x spread on identical work.** V8's `BigInt` multiply at 16,384 bits
measured **5,900 ns at 20,000 iterations and 23,000 ns at 2,000** — from the count alone. A ratio
taken against a number like that is not a result. The harness is now **time-budgeted**: each side runs
for the same wall clock and the count falls out.

**And the result was dead.** With the operands loop-invariant and the product unused, V8 eliminated
`x * y` outright — a 16,384-bit multiply *measured* **48 ns/op**. The product now escapes into a sink
that is read afterwards. The Karatsuba side never needed one; it writes into its workspace.

The sanity check that settles which number is right: at 16,384 bits `BigInt` is 256 64-bit limbs, and
Go's `math/big` does that in 12.9 us. **22.9 us is the right order; 5.9 us would have made V8 faster
than `math/big`**, which is not credible.

| bits | old ratio | **corrected** |
|---|---|---|
| 256 | 6.6x | **5.3x** |
| 1,024 | 40x | **23.9x** |
| 4,096 | 83x | **35.7x** |
| 16,384 | **148x** | **68.9x** |

The direction is unchanged — `BigInt` wins at every size and by more as they grow — but the magnitudes
were roughly double. That is the **third** benchmark-method error in this repository's JavaScript
numbers, after the per-byte closure in jsontok and the module-namespace lookup in native-js, and the
rule that catches them keeps being the same one: **make a suspicious result explain itself before
recording it.**

## 3c. windows, where we control everything

`gauntlet/windows/karatsuba.asm`, hand-written MASM. This is the host with no bignum of its own, so
"as fast as possible" is the whole brief — and x86-64 has three things no other host in the set does:
`mul` (64x64 -> 128 in one instruction), `adc`, and `sbb`. Every carry chain below is the **flag**, so
each loop bumps its index with `lea` and its counter with `dec`, the two instructions that leave CF
alone.

**And one structural win the other three ports miss.** The descriptor table — every node's
`(aOff, bOff, len)` — is a function of `(n, D)` **alone**. It does not depend on the operands, so it is
computed once in `kara_setup` and the timed path never touches it. Go, Java and JavaScript rebuild it
on every multiply because it was cheap enough to ignore there.

| n = 1,024 limbs (65,536 bits) | ns/round |
|---|---|
| schoolbook | 844,682 |
| Karatsuba D = 3 | 374,675 |
| **Karatsuba D = 5** | **234,065** |
| Karatsuba D = 6 | 244,327 |

**3.61x over schoolbook, and the fastest of the four implementations** — 1.44x faster than ours on Go
and 2.28x faster than ours on Java.

**Cross-host verified, not just self-consistent.** The operand generator is Go's `LimbsOf` reproduced
exactly, and the top limb of the 2n-limb product is **10113443065733330941** on x86 and on Go, where
Go's is checked against `math/big`. All four depths agree with each other as well.

## 3d. All four hosts

| | schoolbook | **Karatsuba** | over schoolbook | the host's bignum |
|---|---|---|---|---|
| **x86-64, hand-written** | 844,682 | **234,065** | **3.61x** | *none exists* |
| Go | 1,152,062 | **337,535** | 3.41x | 121,546 |
| Java | 1,914,360 | **533,505** | 3.59x | 340,367 |
| JavaScript† | 1,575,741 | **710,776** | 2.22x | 22,866 |

*(All at 1,024 limbs / 65,536 bits except † JavaScript at 16,384 bits, where its 15-bit limbs make the
larger size impractical.)*

**Karatsuba is worth 2.2x to 3.6x on every host**, and the ranking of our own implementations follows
the limb width exactly: 64-bit on x86 and Go, 64-bit on Java behind `multiplyHigh`'s sign correction,
15-bit on JavaScript.

**It does not change who wins.** Against the host's own bignum we go from 9.53x to 1.93x behind on Go,
5.62x to 1.57x on Java, and 68.9x to 31.1x on JavaScript. On windows there is nothing to lose to, and
234 microseconds is simply the number.

What remains everywhere is the same two things and neither is about recursion: **hand-written
ADX/MULX inner loops, and Toom-Cook above Karatsuba's range.**

## 3e. Is windows faster than the host bignums? Not yet — and it is the COMBINE, not the algorithm

hamza: *"is our windows implementation faster compared to big in go, or bignum in javascript or not?
because it should, we control everything."*

Measured like-for-like at **65,536 bits**:

| | ns | |
|---|---|---|
| Go `math/big` | **122,836** | we are **1.50x behind** |
| **our x86-64** | **184,058** | |
| V8 `BigInt` | 205,439 | we are **1.12x ahead** |

**So: yes against JavaScript, no against Go.** And chasing it produced the diagnosis.

### It was never the algorithm

**Go's `math/big` has no Toom-Cook.** At 1,024 words it runs Karatsuba over a schoolbook base case —
the same algorithm we run. The gap was entirely its inner loop: `addMulVVW` is hand-written
**MULX/ADOX/ADCX**, and ours was the naive form, which serialises on `mul`'s fixed `rdx:rax` and on a
single carry chain.

Three instructions fix that. `MULX` writes two *chosen* registers and touches no flags, so several can
be in flight; `ADCX` and `ADOX` carry through **CF and OF**, which are independent, so two accumulation
chains run at once. The catch is that `dec` and `cmp` write OF, so ordinary loop control destroys the
ADOX chain — the answer is to keep both chains inside an **unrolled block of four** and fold them into
the running carry at the block boundary, where the flags are dead.

| | before | after |
|---|---|---|
| schoolbook, 1,024 limbs | 844,682 | **456,068** — 1.85x |
| Karatsuba, best depth | 234,065 | **184,058** — 1.27x |

The checksum is unchanged at every depth and still matches Go and `math/big`.

### What is left is the combine, and here is the arithmetic

Schoolbook does 1,024² = 1,048,576 limb-multiplies in 456,068 ns — **0.435 ns each, about 1.4 cycles**,
which is close to the one-`mulx`-per-cycle ceiling. The kernel is no longer the problem.

Multiply that rate by the base-case work each depth actually does:

| depth | base multiplies | base work | measured | **combine** |
|---|---|---|---|---|
| D = 4 | 81 x 66² | 153,483 ns | 184,817 | 31,300 (17%) |
| D = 5 | 243 x 34² | 122,195 ns | 187,364 | 65,200 (35%) |
| D = 6 | 729 x 18² | 102,745 ns | 215,558 | **112,800 (52%)** |

**The base-case work keeps falling as `(3/4)^D` and the combine keeps rising as `3^D`**, and they cross
between D = 4 and D = 5 — which is exactly why depth stops paying there. If the combine were cheaper we
could go deeper, and deeper is where `math/big` is.

Our combine makes **six passes** over each node's output: zero it, add z0, add z1, add z2, subtract z0,
subtract z1. `math/big` computes into the destination with a single scratch buffer and fewer passes.

**So the next move is the combine, not a better algorithm** — and that is a much more specific thing to
go after than "we are 1.5x behind".

### 3f. The combine, done

Two changes, both from noticing that the buffers are mostly zero.

**z0 and z1 have exact, short significant lengths.** z0 is child 0's product, so it is at most `2h`
limbs and zero above that; z1 is at most `2(l-h)`. Only z2, the sum-child's product, needs the full
`csz`. That turns `zero(sz); add z0; add z1` into `copy z0; copy z1; zero the tail above 2l`, and
shortens both subtractions from `csz` to their true lengths.

**And a latent hazard fell out on the way.** `k_school` zeroed `2n`, but a base-case slot is
`prodOf[D]` limbs and the lo/hi children have `ln = lenOf[D]-1` — so two limbs at the top of the slot
were never cleared, and the combine read them. It was invisible because the benchmark multiplies the
same operands every round, so a stale limb held exactly the value it should have held. The caller now
zeroes the whole slot.

| n = 1,024 limbs | naive kernel | + MULX/ADX | **+ combine** |
|---|---|---|---|
| schoolbook | 844,682 | 456,068 | 453,874 |
| Karatsuba, best | 234,065 | 184,058 | **173,602** |

**1.35x over where this section started**, and against the host bignums at 65,536 bits:

| | ns | |
|---|---|---|
| Go `math/big` | 122,836 | we are **1.41x behind** — was 1.91x |
| **our x86-64** | **173,602** | |
| V8 `BigInt` | 205,439 | we are **1.18x ahead** |

### What is still on the table, named precisely

The combine is still **six passes**, and the two copies are the largest of them — `2l` limbs against
`csz` for the others. A recursive Karatsuba pays neither, because it computes z0 and z1 **directly
into the destination** at offsets 0 and 2h.

That is reachable here too: alias child 0's product slot onto the parent's output at offset 0, child
1's at offset 2h, and give only child 2 a slot of its own. The combine then becomes **four passes** —
zero the tail, `z2 -= z0`, `z2 -= z1`, `out[h..] += z2` — with no copies at all.

**It cannot be done by simply changing the offsets**, which is why it is not done here: `out[h..] -= z0`
would read `out[0..2h)` while writing `out[h..)`, and those overlap. The subtraction has to be done
into z2's own buffer *first*, while z0 and z1 are still pristine in the destination. And the slot
sizes stop being uniform — child 0 needs `2h`, child 1 needs `2(l-h)`, child 2 needs `prodOf[L+1]` —
so `pOff` needs a per-node length beside it.

Worth doing, and a layout change rather than a tweak.

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
