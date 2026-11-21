# Research: a set is a claim, and ρ alone chooses the representation

2026-11-21. hamza, on the book's finding 116 (a helper's `(Int)` asks for nothing, its `(-2^64..2^64)`
asks): "I agree with what you said about Int, but maybe that should be true about every other range as
well, as far as it is consistent cross targets. what does the algebra say"; then, shown the examples
under the current rule, under B (every set above the word asks) and under A (no set asks): "write the
research note for A … these are all examples that should be taught in the book".

**Decided** (hamza, 2026-11-21: "1. E on Go and the JVM, yes. 2. A an intermediate outside E yes 3.
report is enough, a flag (-words is just a bonus option for the compiler)"). **Built**: E on every
64-bit host (ADR 0149, [doubleword](../gauntlet/results/doubleword-2026-11-21.md)); A (ADR 0150,
[exact](../gauntlet/results/exact-2026-11-21.md)), with the taint the build found it needs (a value
flowing into a sink stays a word); on JavaScript, a value of U held in its pair only where the pair
computes its uses (ADR 0151, [pairsel](../gauntlet/results/pairsel-2026-11-21.md)). This note derives A, measures what it costs, and finds that most of
its cost is paid by the double word.

## 1. The question

A declared set S has two jobs today (ADR 0079 item 3):

1. **A claim**, a sentence of the logic: the value is in S, proven or the program is refused (ADR 0072,
   0087); its ends past the word enforced at run time (ADR 0029).
2. **Consent**: S above the word permits the values it covers to be held above the word. Without it an
   operation not proven inside some word is refused ("bounded by default", ADR 0019 item 2).

The second job is the subject. Three findings meet in it: 108 (`(lo + hi) / 2` over words refused,
though the average of two words is a word), 116 (consent at a helper is upward-open: its bounded set
asks, its `Int` does not), 118 (Go and the JVM refuse what windows computes in the double word).

## 2. Four laws, each already decided

- **L1, one bridge.** A representation is an abstraction function (ADR 0083): changing it never changes
  a value. "The logic never prints … the one bridge is ρ, a representation chosen from *proven* sets"
  (ADR 0049).
- **L2, β-invariance.** A definition is a named λ carrying its claims (ADR 0051), so `f(t)` reduces to
  `(e[t/a] : R)`, which is the binder `x ∈ R = e[t/a]`. Inlining changes no verdict.
- **L3, upward closure.** A claim is a set under ⊆; weakening one (S ⊆ S′) never turns an accepted
  program into a refused one. If consent comes from sets, it is upward-closed: S consents ⟹ S′ does.
- **L4, one rule on every target.** A verdict may differ between targets only by what each target's
  representations hold, and that difference is computed and reported (ADR 0026, `cmd/portable`).

| | L1 | L2 | L3 | L4 |
|---|---|---|---|---|
| **current** | broken: a declared set chooses | broken: `s ∈ Int = lo + hi` compiles, `add(lo, hi)` with `(Int)` does not | broken: −2⁶⁴..2⁶⁴ ⊆ ℤ, and only the smaller asks | broken: whether `Int` asks depends on how a target spells it; the double word on one host only |
| **B**: every set above the word consents, as permission | broken | holds | holds | holds once E is on every host |
| **A**: no set consents | holds | holds | holds, trivially | holds once E is on every host |

B needs one repair to hold at all: consent must be permission and not demand. As a demand (built
2026-11-21 for a helper's `(Int)` and taken back) it put a value whose fact leaves S into the rung ahead
of U, so `strconv.FormatUint`'s argument, an index and a tuple's components became big integers.

## 3. A, stated

**ρ(v) is the cheapest representation whose domain contains v's proven fact**, over the chain each
target provides:

| | S, the signed word | U, the unsigned word | E, the double word | ℤ, the rung |
|---|---|---|---|---|
| domain | [−2⁶³, 2⁶³) (JavaScript ±(2⁵³ − 1)) | [0, 2⁶⁴) | [−2¹²⁷, 2¹²⁷) | ℤ |
| Go | `int` | `uint64` | a pair by `math/bits` (**not built**) | `*big.Int` |
| the JVM | `long` | `long` read unsigned | a pair by `Math.multiplyHigh` (**not built**) | `BigInteger` |
| JavaScript | `number` | two 32-bit digits (ADR 0083) | none native | `BigInt` |
| windows | `i64` | `i64` read unsigned | `i128` (ADR 0103) | K(ℕ\*) (ADR 0099) |

"Cheapest" is measured, never asserted (ADR 0008): S and U are one instruction, E a few, ℤ an
allocation. S ∪ U ⊆ E ⊆ ℤ, so every value has a representation and **no operation is refused for its
size**. The decision is the one it is today: an operation's fact, by intervals, the zone entries and the
Omega test where an atom repeats (ADR 0148).

**A refusal remains where a value must enter a bounded set**, a sink:
- a host's parameter whose set is a word (WriteFile's DWORD, `strconv.FormatUint`'s `uint64`);
- a table's index, a table's length and an allocation's size (≤ max-len);
- a table's element and a map's key and value, words by storage (ADR 0147);
- an entry point's parameter or result declared inside the word, the host's word at its interface;
- an operation under `-checked`, which traps where a word is required.

**A claim is unchanged**: decided at its definition, refused if false whatever the representation;
an end past the word enforced at run time as ADR 0029 enforces it. `(S₆₄)` claimed of a table's sum is
false (the sum reaches 2.35·10²¹) under every design.

So A is integers-boundary-research §7 taken whole: "a refusal happens only where a value must enter a
bounded set it is not proven in". ADR 0147 built it at entry points and at printing; A builds it for an
intermediate.

## 4. The programs

Five programs, each refused today on Go and the JVM for an intermediate past every word, each a
well-known bug where integers are the machine's. Each is written as one means it; under A that is the
program.

| | the program | the bug elsewhere | the intermediate | in E |
|---|---|---|---|---|
| 1 | `(lo + hi) / 2` | the JDK's binary search, nine years (Bloch 2006) | lo + hi ∈ [−2⁶⁴, 2⁶⁴ − 2] | yes |
| 2 | `d = a - b; sign(d)` | the subtraction comparator (Java's `compareTo` returning `a - b`) | a − b ∈ (−2⁶⁴, 2⁶⁴) | yes |
| 3 | `x * 3 / 4` | multiply-then-divide wraps | 3x ∈ (−2⁶⁵, 2⁶⁵) | yes |
| 4 | `w * h <= 1000000` | an allocation's size check wrapping small (CWE-190) | w·h < 2¹²⁶ | yes |
| 5 | `a + b > 2^63 - 1` | C's overflow test, undefined behaviour the compiler may delete | a + b ∈ [−2⁶⁴, 2⁶⁴ − 2] | yes |

Today each compiles on windows (`i128`) and on JavaScript (its parameters are already BigInt), and is
refused on Go and the JVM. Under A each compiles on every target, computed in E.

Two programs are refused under every design, and stay so under A: the table's sum claimed in S₆₄ (a
false claim), and the tour's `prefix_sums` stored into `Array(Int)` (a sink: an element is a word). Both
are repaired by stating the length assumed, `where len(t) <= 2^20`.

One program shows A's real cost: the table's sum with a result `(Int)`. Its accumulator's fact is
[0, 2.35·10²¹], outside E, so it is ℤ: **today already**, by ADR 0147's result demand, and under A too.

## 5. Measured

On Go (this laptop, median of 5, `go test -bench`; the E forms written by hand as a printer would, each
checked against `math/big` on a million pairs including both ends of the word):

| | ns | allocations |
|---|---|---|
| midpoint, sum in ℤ (`*big.Int`) | 39 | 3 |
| midpoint, sum in E (`bits.Add64`, then a truncated halving) | **1.70** | 0 |
| midpoint, Bloch's `lo + (hi - lo) / 2` | 1.79 | 0 |
| `w * h <= N`, product in E (`bits.Mul64`) | **0.58** | 0 |
| `w * h <= N` by algebra, `h == 0 \|\| w <= N / h` | 2.34 | 0 |
| a 1 MiB table's sum, accumulator in ℤ | 6.15 ms | 3 |
| the same, accumulator a word (`where len(t) <= 2^20`) | 0.25 ms | 0 |

**The double word costs what the algebra costs, or less.** Program 1 in E is Bloch's form's speed;
program 4 in E is 4× faster than its algebraic rewrite, a product being cheaper than a division. **ℤ
costs 22× on the midpoint and 25× on the accumulator**, and that is A's cost, paid only where a value
leaves E. On windows, E is built; its timings against the algebraic forms are owed.

## 6. What A changes, and in what order

**E on Go and the JVM first, under the current rule.** ADR 0103 already decides that an intermediate
proven in E is computed in the double word, and it needs no consent: windows compiles all five programs
today without one. Printing E on Go (`math/bits`: `Add64`, `Sub64`, `Mul64`, `Div64`, a signed product
by the identity hi = hiᵤ − (a<0)·b − (b<0)·a) and on the JVM (`Math.multiplyHigh`, Java 9; its
presence on ART is owed) closes finding 118, and with it every program of §4, on every target, before A
is decided. JavaScript has no double word: its E is BigInt.

**Then A is a question about ℤ alone.** With E on every host, current and A differ only for a value
outside E in a program that asks nothing: today refused, under A computed in ℤ and reported. The
corpus: none of its 13 programs that split Go against windows splits by E; the language operations among
the 54 unproven (`kara`, `shift-div`, `limb-subdiv`, `wordcount`, named in CLAUDE.md) are the verdicts
A would change, each to be read before the build.

**Documents A amends**: ADR 0019 item 2 (bounded by default) reversed for intermediates; ADR 0079
item 3, consent struck, the claim kept; ADR 0148 item 1, a binder's `∈ Int` a claim only; CLAUDE.md's
"never introduce boxing or hidden allocation into the core" read as it was meant: no value proven in a
word is boxed, and a value held in ℤ is the integer the program means, each one reported.

## 7. The algebra under A

A rewrite is an equation between two terms of ℤ: `(lo + hi) / 2 = lo + (hi - lo) / 2` under lo ≤ hi.
It never changes the meaning; it changes the facts, every intermediate's proven set, and so ρ. Under A
the rewrite is no longer how a program compiles but how it gets cheaper, and §5 says when:
- **against ℤ** it pays, 22–25×: an accumulator bounded by a stated length, a sum kept inside E;
- **against E** it may not: the midpoint is equal, `fits` is 4× slower rewritten.

So the book teaches the algebra as what it is, a proof that a value is small, and measures, target by
target, what the proof buys. The rewrite whose value is "it compiles" disappears; the one whose value is
"it is a word" stays.

## 8. For hamza

1. **E on Go and the JVM** (ADR 0103 on every host): no new decision, a realization owed; closes
   finding 118 and compiles every program of §4 under the current rule.
2. **A**: an intermediate outside E computed in ℤ, reported, where today it is refused; A's whole
   remaining difference. Its cost is §5's 22–25× wherever a program does not state the bound that keeps
   it in a word.
3. **If A**: whether the report is enough, or a flag (`-words`) makes an operation held in ℤ a refusal,
   for a program that must allocate nothing.

## 9. The book

Chapter 6's §6.9 becomes the machine's four readings, S, U, E and ℤ, taught through §4's five programs:
each written as meant; the bug it is in C, Java or Go and why (the wrap, the undefined behaviour, the
deleted test); what each target holds it in (`i128`, `bits.Add64`, `BigInt`); the algebraic form, its
proof, and §5's measurement of what it buys on each target. Then the accumulator, the one place ℤ is
paid, and the `where` that removes it. It is written with the build, from programs run on every target
(the book moves with the language); until then chapter 6 teaches the rule in force.

## 10. Not built, owed

- E on Go and the JVM (§6); on windows, E's timings against the algebraic forms.
- The JavaScript verifier refusal of `fits`'s rewrite (finding 117).
- The corpus's language-operation verdicts A changes, each read (§6).
- Compile time: the rung would run in more programs under A, measured at the build.
