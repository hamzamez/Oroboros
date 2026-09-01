# How a value says it is bigger than a word

Research, and **§3a is now built** — the spelling landed on 2026-08-31 in a shape none of the
candidates below proposed. The rest is kept as the reasoning that produced it.

**No decision on the unbounded rung itself** — this exists to say what
[ADR 0019](decisions/0019-precision-by-declaration.md)'s second owed item actually is, why the
question is really two questions, what each candidate spelling costs, and what has to be true before
any of them is built.

ADR 0019 owes *"a spelling for the unbounded rung, since `(int LO HI)` has two finite endpoints and
cannot say ℤ."* That sentence is right and it hides the more valuable half of the problem.

---

## 0. What is already settled, so it is not relitigated

| | where |
|---|---|
| `int` is exact within ±(2⁵³−1), and outside it there is no claim | [ADR 0012](decisions/0012-portable-integer-range.md) |
| Mathematical semantics, machine representation — the compiler picks the width | [ADR 0003](decisions/0003-range-typed-integers.md) |
| Bounded by default; an unprovable integer operation is a **compile error**, with the trap as the escape | [ADR 0019](decisions/0019-precision-by-declaration.md) |
| A range is a type, and it works on scalars, array elements and map values | [scalarrange-2026-08-31](../gauntlet/results/scalarrange-2026-08-31.md) |
| Arbitrary precision is **ours**, with a per-target and per-**operation** threshold | [bigarith-2026-08-28](../gauntlet/results/bigarith-2026-08-28.md) |
| A range is three things kept apart: a type, a premise, and a representation | scalarrange-2026-08-31 §1 |

---

## 1. The type language is a lattice of subsets of ℤ

An integer type here denotes a set of integers:

```
    int          W = [-(2⁵³−1), 2⁵³−1]        the portable window
    (int LO HI)  [LO, HI]                     a finite interval
    ???          ℤ                            the top
```

Ordered by inclusion, these are intervals with ℤ as ⊤. Representation is a **monotone map** out of
that lattice:

```
    ⊆ a declared int-repr   →  that width          (elemwidth)
    ⊆ W                     →  the host's word
    finite, wider than W    →  FIXED-LIMB big
    ℤ                       →  ARBITRARY-PRECISION big
```

`int` is not special in the lattice; it is the particular finite interval every host agrees on.

## 2. So it is two questions, and only one of them is about spelling

bigarith-2026-08-28 measured the last two rungs as different things — *"bounded-but-huge and
unbounded are two different rungs, worth a factor of four"* — because a finite range gives a **limb
count**, which gives a `build` of known length, which gives **zero allocations**. An unbounded
declaration gives none of that and lands back on allocate-per-operation.

| | what blocks it | measured value |
|---|---|---|
| **bounded-but-huge** | the **reader**. `KInt` is an `int64`, so `(int 0 9223372036854775808)` is refused: *"does not fit in an integer; the portable range is ±(2⁵³−1) and the widest target is 64 bits"* | ours beats the host's bignum **3.97× on Go, 6.2× on Java, 5.8× on V8** |
| **unbounded** | there is genuinely no spelling | none measured |

**The bounded rung needs no new syntax at all.** `(int LO HI)` already says it; the reader will not
read the literal. That is the half where the factor of four lives, and ADR 0019's phrasing points at
the other one.

### 2.1 A range endpoint is a bound, not a value

This is what makes the reader change small and safe. ADR 0012 constrains the integers a program can
**compute with**; the endpoints of a range are not computed with — they are the description of a set.
So endpoints may be arbitrary precision while `KInt` stays an `int64` and nothing about the value
language moves.

Conflating the two is the obvious mistake, and it would be a bad one: allowing a term literal past
the window would silently reintroduce exactly the divergence ADR 0019 just closed.

---

## 3. The objection every candidate has to face

> **The unbounded rung is not a refinement of `int`. It is a WIDENING.**

Every range today satisfies `[LO,HI] ⊆ W`. That is why `core.ValueType` can normalise a range to
`int`, and why `compatible` accepts one anywhere an `int` is wanted — a 0..255 parameter *is* an
integer that happens to satisfy a bound.

**ℤ ⊄ W.** An unbounded value passed where an `int` is required must therefore be **refused**. The
promotion cannot be transparent, and that refusal is not a wart — it is the surface. It is where a
programmer finds out that a value has become a bignum, at the boundary where the representation
actually changes.

This is decidable and it is the half of subtyping [type-algebra.md](type-algebra.md) already keeps:
`{i | 0 ≤ i < n} ⊆ int` is bounded subtyping in QF-LIA, and this is the same test in the other
direction. What it costs is that scalarrange-2026-08-31's **three effects become four** — a range is
an `int` for typing, a premise for the analyses, a representation for the emitter, and now
*possibly not an `int` at all*, which every one of those three has to be told about.

---

## 3a. BUILT 2026-08-31 — and the shape came from hamza rather than from §4

The candidates below all assumed an endpoint is a *literal*, and implementing 4.1 immediately hit
something none of them accounted for: **types are built from ordinary terms**, and the reader refuses
a big literal at tokenisation, before `TypeName` ever sees it. So 4.1 needed either an eighth term
kind — against *"Seven term kinds. That is the entire grammar of what a program can say"* — or a
literal carried in an unused field, which is the silent-truncation shape deliberately.

hamza's answer dissolves it: **an endpoint is a compile-time EXPRESSION.**

```lisp
(int 0 (pow 2 70))          [0, 2^70 − 1]
(int 0 (* 1000 1000))       [0, 1000000]
(int (- 0 5) 5)             [-5, 5]
```

It works because §2.1 is doing real work: **the expression is evaluated, never emitted.** ADR 0012
constrains the integers a program computes with; an endpoint describes a set. So there is no eighth
term kind, no big literal for the reader to refuse, and no widening of `KInt` — and the only
arbitrary-precision arithmetic in the compiler is `evalEndpoint`, **one function**, where §4.1's
honest cost was a twelve-site migration.

It also makes the endpoint more legible than the thing it replaces: `(pow 2 70)` says what a
seventy-digit literal would have hidden.

**The grammar is deliberately tiny** — literals, unary `-`, `+`, `-`, `*`, `pow` — because an
endpoint is written by a person to say how big something gets, not computed. **Division is absent**
for the reason ADR 0009 gives about folding: it carries a precondition, and a bound with a
precondition is not a bound. `pow` and not `^`, because `^` is XOR on Go, JavaScript and Java.

**And §3's refusal fell out with nothing written for it.** `ValueType` normalises a range to `int`
only when `IntRange` can read it, and `IntRange` narrows to `int64` and reports failure otherwise —
so a range past the word is simply not an `int`, and `compatible` already refused it. The only
addition was to make the message say *why*, which is the surface §3 predicted:

```
n is int 0 1180591620717411303424, which is WIDER than the portable window ±(2^53−1),
so it is not an `int` — arbitrary precision is a rung above the host's word and is not
implemented yet.
  A range above the window is a WIDENING, not a refinement: a value that may leave the
  machine word cannot silently be used where an `int` is required, and this refusal is
  where that is said.
```

**Cost: no emitted file changes on any target.** The spelling exists and refuses honestly; what it
does not yet do is *select a representation*, which is R3 — §5's ordering is unchanged.

## 4. Candidates



### 4.1 Arbitrary-precision endpoints — `(int 0 100000000000000000000)`

**Appropriate.** No new surface: the constructor, the reader's shape, the desugaring into a `where`,
and the representation selection all stay. §2.1 is why it is safe. And it is safe *now* in a way it
was not last week: an out-of-window range makes its operations unprovable, and unprovable is now a
**compile error** rather than a silent fallback to the host word. Adding the spelling before the
refusal would have produced a truncating wrong answer.

**Might not be.** `core.IntRange` returns `(int64, int64, bool)` and there are **twelve call sites**
across `core/read.go`, `emit/interval.go` and `emit/target.go`, plus `reprFor(lo, hi int64)` and
`reprBytes(lo, hi int64)`. Widening the endpoints means widening all of them at once, and a partial
migration leaves a path that truncates silently. That failure shape has landed three times in one
week — `seedFromSig` spelling a range as `uint16`, `BufferElemBytes` taking the first store, the
map's `len` reading the buffer's length — and each was invisible until something reached it.

A second cost: the interval domain is `int64` throughout (`iMin`, `iMax`, `sat = 1<<62`).
precision-by-declaration.md §"the expected blocker is not the blocker" argues it does **not** need to
go arbitrary-precision, because an operation on a value that is already exact cannot overflow and so
the analysis is never asked. That argument should be re-checked rather than assumed, because it was
written before the refusal existed and the refusal is what decides which operations get asked about.

### 4.2 Infinite endpoints — `(int 0 +inf)`, `(int -inf +inf)`

**Appropriate.** It keeps one constructor. It subsumes the half-open case for free, which is not
cosmetic: a factorial accumulator is non-negative, and bigarith measured sign handling as a real cost
— `Long.compareUnsigned`, a three-term correction per multiply where the host has no unsigned high
multiply — so `(int 0 +inf)` carries something the representation can spend. And it is **the
vocabulary the compiler already prints**: every report says `idx -inf..+inf`, so the declaration and
the diagnostic would finally use the same word for the same thing.

**Might not be.** ℤ has no infinite elements, so `inf` is an endpoint in the extended reals rather
than an integer. That is standard for an interval domain and it is exactly what this lattice is, but
it is a small imprecision in a type language that has otherwise been literal. And `+inf` as a
*token* wants a reader rule that exists nowhere else.

### 4.3 A distinct type name — `bigint`, `integer`, `exact`

**Appropriate.** Unmistakable and one token, with no reader arithmetic at all.

**Might not be.** This is ADR 0019's option **B**, rejected because *"B's surface is a type name and
carries no size; C's is a range and carries exactly what the fast path needs"*. That objection is
weaker at ⊤ — there is no size to carry — but the naming problems are real. `bigint` names a
**representation**, which is the distinction ADR 0003 spent its whole argument establishing.
`integer` beside `int` is a one-letter difference for a windowed-versus-unbounded gap, which is a
trap rather than a name. `exact` says the meaning and not the machine, and is the only one of the
three worth keeping on the list.

### 4.4 `(int)` with no endpoints

**Appropriate.** Maximally uniform: the endpoints are simply optional.

**Might not be.** **`int` already means the window.** `int` and `(int)` differing would be
indistinguishable at a glance and opposite in meaning — the worst available outcome for a surface,
and the reason match.md gives for `=` over `tag=` applies in reverse: a name must say what a thing
IS, and these two say the same thing and mean different ones.

---

## 5. What this suggests, and what would falsify it

**Do 4.1 first, and treat 4.2 as the follow-on.** The bounded rung is where the measurement is, it
needs no new syntax, and it is the prerequisite for a representation solver that has anything to
solve. The unbounded rung has no measurement behind it at all yet.

**4.3 and 4.4 are rejected** on the grounds above, and recorded so they are not re-proposed.

Three things would change this:

1. **A program that genuinely needs ℤ rather than a wide bound.** Every case measured so far —
   factorial, Fibonacci, exponentiation, Karatsuba — has a bound the programmer knows. If a real one
   does not, the unbounded rung stops being the smaller half.
2. **The twelve-site widening turning out to be more than mechanical.** If `reprFor` or the interval
   domain resists arbitrary-precision endpoints for a reason that is not just types, 4.1 is not the
   cheap half after all.
3. **A measurement on the boundary refusal.** §3 says a bignum may not silently become an `int`. If
   that refusal fires constantly in real code, the promotion needs a conversion surface — and where
   that sits is the question ADR 0019 opened and did not close.
