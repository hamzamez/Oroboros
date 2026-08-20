# Integers, strings, and binaries

**Status, 2026-08-19. Research. Nothing built.** Written before any code, per the rule
[strings.md](strings.md) exists to enforce.

This is the deep version of the question [bitwise.md](bitwise.md) opened. It starts from a rule
hamza stated, which the project did not have and needs:

> **A portable layer has to know the cost, and name it, not hide it — maybe even justify it.**

§0 turns that into a test. Everything after applies it.

---

## 0. The rule: a portable name must publish its price

`fn`, `def`, `module`, `if` and `loop` came out clean because they are **target-independent by
construction** — they say nothing about representation, so there is no price to hide. A program
that wants a target's own operation reaches for it by name and pays whatever that target charges,
knowingly.

Everything between those two is where portable layers rot. A portable name that quietly costs 1×
on one host and 4× on another is not an abstraction, it is a **hidden tax with a friendly face** —
and it is exactly what killed the predecessor project (design-direction §2).

So, four obligations. The first two exist already, in [state.md §6](state.md); the last two are
new and are hamza's rule made checkable.

| | obligation |
|---|---|
| **1. Meaning** | one definition, independent of any target |
| **2. Agreement** | every target computes it — or the covering check names the ones that do not |
| **3. Price** | the cost on *each* target is measured and written down |
| **4. Justification** | if the price is nonzero, the document says what it buys and what you would write instead |

And one consequence, which is the sharp edge:

> **If a name's price differs across targets by more than the noise floor, the SPREAD is part of
> the name's meaning.** A name that costs 1.3× on Java and 3.7× on JavaScript does not mean the
> same thing on both. Either the spread is published at the point of use, or the name is Tier 2.

This is [ADR 0008](../decisions/0008-measurement-over-principle.md) — *parasite decisions are
measurements, not principles* — extended from target choices to **portable-layer** choices. It is
also the test that resolves the tension in [bitwise.md §5.3](bitwise.md): the retired portable
layer failed obligation 3 for every name it had, because nobody had ever measured any of them.

---

## 1. Integers

### 1.1 What is true today, including two defects

[ADR 0012](../decisions/0012-portable-integer-range.md): an `int` is a mathematical integer, exact
within ±(2⁵³−1), and outside that window the behaviour is the target's.

**Defect one: an integer literal past int64 silently becomes a float.** Measured — and **fixed
while writing this**, because it is a bug under every candidate below:

```
9007199254740991      ⟶  9007199254740991        int
9223372036854775807   ⟶  9223372036854775807     int
9223372036854775808   ⟶  9.223372036854776e+18   FLOAT
99999999999999999999  ⟶  1e+20                   FLOAT
```

`core/read.go` tried `ParseInt(text, 10, 64)`, and when that failed it tried `ParseFloat`, which
succeeds. So the literal changed *type*, silently, at 2⁶³ — a threshold the specification never
mentions and which is ten bits past the portable window it does mention.

Being an integer literal is now a property of **how it is written** — an optional sign and then
digits — rather than of whether it happens to fit, so one that does not fit is refused:

```
9223372036854775808 does not fit in an integer; the portable range is ±(2^53−1)
and the widest target is 64 bits (docs/spec/arithmetic.md §4)
```

**Defect two: nothing is folded.** [state.md §3](state.md) says no primitive is ever evaluated, and
that is visible in emitted code. A three-layer byte-accessor library (§3.4) fuses to one expression
and leaves this behind:

```go
var i int = (0 + 2)
return (((((b[0]) << 8) | (b[(0 + 1)])) << 16) | (((b[i]) << 8) | (b[(i + 1)])))
```

`0 + 1` and `0 + 2` survive to the artifact. Go's compiler folds them; on the windows target
nothing would. Worse, the unfolded `(0 + 2)` is not *duplicable*, so call-by-need bound it to a
name — a `let` that exists only because a constant was not a constant.

Also true, and relevant: the `int` **window is unchecked**. That is the hole
[arithmetic.md §4](arithmetic.md) names and the one
[assessment §3.2](../assessment-2026-08-19.md) reaches from the other side.

### 1.2 The measurement that decides it

[overflow-2026-08-19](../../gauntlet/results/overflow-2026-08-19.md). What an ordinary addition
pays for the *possibility* of a bignum, when the value stays small and nothing allocates:

| | plain | fixnum check | windowed check |
|---|---|---|---|
| Go | 1× | **2.61×** | 1.65× |
| JavaScript | 1× | **3.74×** | 1.81× |
| Java | 1× | **1.31×** | 1.19× |
| x86 | 1× | ~1× (`add` + `jo`) | — |

> **Corrected: this priced the CHEAP operation.** hamza's objection — *the real price is
> multiplication, not checking* — is right, and §1.2b has the numbers. Addition is where a bignum
> representation looks affordable; multiplication is where it is actually paid for.

**The cost is not one number; it is a 3× spread, and the spread has a mechanism.** The JVM has an
intrinsic that compiles `Math.addExact` to the hardware's overflow flag. Go has no such intrinsic,
so the check lands in the loop-carried dependency chain. JavaScript has no fixnum/bignum
unification at all to hook into.

By §0's sharp edge, that settles one thing immediately: **a single portable name meaning "add,
promoting to arbitrary precision if needed" is inadmissible.** It would mean something different on
each of our four targets, and the difference is 3×.

### 1.2b Multiplication, which is where it is actually paid for

[product-2026-08-19 §1](../../gauntlet/results/product-2026-08-19.md). 4096 independent
multiplies, Go:

| | vs plain | allocations |
|---|---|---|
| checked with a **hardware high-multiply** (`bits.Mul64`) | **1.87×** | 0 |
| checked **portably** (divide the product back) | **7.40×** | 0 |
| `math/big`, one receiver reused | **38.9×** | 3 |
| `math/big`, naive | **92.2×** | 4098 per call |

Multiplication is worse than addition for two reasons that compound. **Detecting** overflow needs
the full 128-bit product or a division — there is no single flag test that survives into a
high-level language. And **performing** a big multiply is quadratic in the limb count where a big
add is linear.

The check is cheap only where the hardware is reachable. Go and the JVM expose a high-multiply and
x86 has one in the instruction; **JavaScript has none**, so it is stuck at the 7.4× shape.

### 1.2c Three questions, answered

**Can precision numbers be implemented with bitwise operations?**

Yes — that is how every bignum library works: limbs, schoolbook addition with carry, double-width
products. But the two primitives it turns on are **add-with-carry** and **multiply-high**, and our
hosts do not agree about either:

| | add-with-carry | multiply-high | usable limb |
|---|---|---|---|
| Go | `bits.Add64` | `bits.Mul64` | 64 bits |
| Java | manual, via `>>>` | `Math.multiplyHigh` | 64 bits |
| JavaScript | neither | neither | **~26 bits**, so products stay exact in a double |
| x86 | `adc` | `mul` gives 128 bits | 64 bits |

So a bignum written in the language would need a **different limb size on JavaScript than
everywhere else** — which means it is not one implementation, it is four. And if it is four, the
hosts' own four are better: `math/big`, `BigInteger`, `BigInt`, and nothing.

**Except on windows, where there is no host implementation at all.** That target would need one
written, exactly as `lib/win/fmt.oro` is written. That is the honest asymmetry: three targets have
a bignum to parasitize and one does not.

**Is `math/big` the best way on Go?**

No, and the number says why: **38.9× even used perfectly**, with one reused receiver and no
allocation in the loop. The representation is the problem rather than the usage — every `big.Int`
is a heap object holding a `[]Word`, and there is **no inline small-value fast path in the value
itself**. Every fast bignum keeps the small case in the value: a tagged word, or a struct with an
inline `int64` beside a pointer.

That is a **product**. So the second question's answer points at §1.2d as well.

**Do strings and binaries need a data structure decided first?**

Less than it looks, and this corrects the framing in the earlier draft of this document.

A `bytes` is **a container, not a product** — an opaque handle the target provides, exactly like
`vec-f64` today. `make-bool` and `at-bool` work on the Go native target right now with no product
anywhere. The `vec-f64` pattern is proven and `bytes` follows it.

What genuinely needs a product is a shorter and different list: **error results**, **one `idiv`
yielding two answers**, **`fold-range2`'s pair**, and **a bignum with an inline fast path**.

### 1.2d The product is affordable — measured

[product-2026-08-19 §2](../../gauntlet/results/product-2026-08-19.md), and it is the most
consequential number in this document.

| host | product returned from a hot call |
|---|---|
| Go | **1.01×, zero allocations** — including for an explicit `*struct`, which is the control |
| Java | **0.96×** — C2 scalar-replaces the `record` completely |
| JavaScript | object literal **1.11×**; two-element array **1.32×** |
| x86 | two registers, free by construction |

**One caveat and it is the important one.** This is a product **created and consumed without
escaping**. One that escapes — stored in a container, or crossing a boundary the host declines to
inline — allocates on all three managed hosts. So the result reads:

> A product is free exactly where our reducer already makes things free: built and taken apart in
> the same place. It costs an allocation exactly where `materialize` costs one
> ([ADR 0013](../decisions/0013-accept-the-allocation-price.md)).

That is the boundary the language already has, not a new one — which is why this is a gate opening
rather than a new risk.

And **use an object, not an array, on JavaScript**: 1.11× against 1.32×, which is the opposite of
what the `[q, r]` intuition suggests, and exactly the kind of thing
[ADR 0008](../decisions/0008-measurement-over-principle.md) exists for.

### 1.3 The literature

**Fixnum/bignum as one type.** MacLisp had bignums in the 1970s; Smalltalk-80 has
`SmallInteger`/`LargePositiveInteger`; Scheme standardised the full numeric tower (R7RS §6.2);
Erlang, Python, Ruby and Common Lisp all present one seamless integer. The representation is
tagged — Gudeman's *Representing Type Information in Dynamically Typed Languages* (1993) is the
survey — and V8's `Smi` is the same idea in our own target's runtime.

The design is **fifty years old, thoroughly proven, and priced**. It buys correctness by default:
no program ever silently computes the wrong answer because a value grew. Erlang's reputation for
running telecom switches for decades without arithmetic surprises is not an accident.

What it costs is exactly what §1.2 measured, plus allocation when values do grow, plus a branch
that stops being predictable when they straddle the boundary.

**The explicit split.** Haskell has `Int` and `Integer` and makes you choose. Java has `long` and
`BigInteger`. Go has `int64` and `math/big.Int`. Rust has `i64` and a crate. In all four the fast
type is the default and the exact type is opt-in, verbose, and allocating.

This satisfies §0 trivially — the price is in the *name* — at the cost of making the correct thing
harder to write than the fast thing, which is how overflow bugs happen.

**Range types.** Ada 83 has `type Small is range 0 .. 100`; Pascal and Modula have subranges. The
compiler picks the representation *from the declared range* and inserts checks at the boundary,
raising `Constraint_Error`. This is the oldest form of the idea that a number's type should say
what values it can hold rather than how many bits it occupies.

**This is already this project's declared direction** —
[ADR 0003](../decisions/0003-range-typed-integers.md), *range-typed integers, mathematical
semantics, machine representation* — and [arithmetic.md §4](arithmetic.md) says the specification
"has a hole exactly where the type system will later plug it".

**Refinement types** are the modern form: Liquid Haskell (Rondon, Kawaguchi & Jhala, PLDI 2008),
F*, Dafny. And **Cryptol** (Galois) carries bit widths as type-level naturals, which is the same
idea aimed at exactly the bit-manipulation domain of [bitwise.md](bitwise.md).

**We have the machinery.** `emit/refine.go` discharges conjunctions of linear inequalities today,
for array bounds, from facts collected out of loop guards. A range is a conjunction of two linear
inequalities. It is the *same shape* as the obligation already being discharged.

**Compile-time arbitrary precision.** Go's untyped constants are arbitrary precision **at compile
time** and become fixed-width at runtime. So a language famous for not having bignums does have
them, in the half of the program where they are free.

That half-measure is also this project's recorded hazard:
[ADR 0009](../decisions/0009-staging-preserves-results.md) exists because Go folds `0.1 + 0.2` to
`0.3` at compile time and `0.30000000000000004` at runtime. **For integers the hazard is the same
shape**: folding `2⁶² + 2⁶²` at arbitrary precision gives 2⁶³, and at runtime an int64 wraps. Any
constant folding we add must fold with the *runtime* semantics, not with mathematics.

**On the reported regret.** hamza reports Rob Pike naming the absence of unbounded integers as a
regret about Go. I have not verified that quote and nothing below depends on it — but the substance
is checkable from the Go specification, and it is the half-measure above: Go's constants are exact
and its variables are not, and the seam between them is where the surprises live.

### 1.4 Candidates

**N-A — Status quo.** Mathematical `int`, window ±(2⁵³−1), unchecked.
*For:* zero cost, zero work. *Against:* fails obligation 3 by never having been measured, fails
obligation 4 by never justifying the window, and carries defects one and two.

**N-B — Status quo, defects fixed.** Reject out-of-range literals instead of silently floating them
(**done**); fold integer arithmetic at compile time with runtime semantics (**not done** — it needs
a decision about *whose* semantics, and ADR 0009 says it must be the runtime's).
*For:* both are bugs under every other candidate too, so this is unconditional. Folding removes the
`(0 + 1)` from the artifact and restores duplicability, which matters most on the host with no
optimiser. *Against:* nothing. **This should happen whatever else is decided.**

**N-C — One seamless integer, Erlang's answer.**
*For:* correctness by default; fifty years of proof; the thing hamza likes, and he is right to.
*Against:* §1.2. 2.61× on Go, 3.74× on JavaScript, and a 3× spread that §0 forbids hiding. This is
also precisely CLAUDE.md's *never introduce boxing or hidden allocation into the core*, which is
the recorded cause of the predecessor's death. **Rejected as the default**, and not because
unbounded integers are bad — because *hiding what they cost* is.

**N-D — Two integers, explicitly named.** `int` (machine, windowed) and `bignum` (exact,
allocating), the Haskell/Java split.
*For:* satisfies §0 by construction — the price is the name. A program that needs exactness says
so and pays. *Against:* on the windows target `bignum` is not a host feature at all; it would have
to be *written*, like `lib/win/fmt.oro`. That is a real cost and it is the honest kind: a target
that cannot provide it says so, and covering reports it.

**N-E — Range-typed integers, the declared direction.** `(sig f ((n (int 0 65535))) …)`, with the
representation chosen from the range and the obligation discharged by the machinery that already
discharges array bounds.
*For:* it is ADR 0003; it plugs arithmetic.md §4's hole; it makes the *window itself* checkable
instead of a promise; it subsumes the bitwise int32 window of [bitwise.md](bitwise.md) — that
window is just a range — and it is the only candidate where the price is not merely *named* but
**derived**: the compiler reads the range and picks the cheapest representation that holds it.
*Against:* it is the largest piece of work here, it needs range *inference* to avoid annotation
everywhere, and an undischarged range obligation is noise at every call site.

### 1.5 Recommendation for integers

**N-B now, unconditionally. N-E as the direction. N-D when a program needs exactness. N-C never as
the default.**

The ordering falls out of §0 rather than taste:

- N-B is two bug fixes and is owed regardless.
- N-E is the only candidate that **derives** the price from something written in the program, which
  is stronger than naming it. It is also already the declared direction, so choosing it is
  consistency rather than a new bet.
- N-D is what N-E cannot cover: a value whose range genuinely is not bounded. It is one more type
  with a loud name and a written-down price.
- N-C is the one design that makes the price invisible, and it is invisible by 3×.

**What this means for Erlang's integers specifically.** They are a good design that this project
cannot adopt *as the default* — not because they are wrong, but because our four hosts price them
between 1.31× and 3.74× and the rule says that spread cannot live inside one name. On a
single-runtime language, where BEAM is the only price, Erlang's choice is correct and ours would be
too.

---

## 2. Strings

### 2.1 The situation is worse than for integers

Two of our four hosts are UTF-16 (JavaScript, Java) and two are bytes (Go, x86). There is no
representation that is native on all four, and every index-based operation diverges.
[strings.md](strings.md) already records the consequence: `length` of `"🙂"` is **4** on Go, **2**
on JS and Java, and **1** counting characters, so `length` is not in the core, and strings are
portable "only by having almost no operations."

### 2.2 The literature, and Erlang's own retraction

**Erlang's strings are lists of code points**, and hamza's suspicion is correct — Erlang itself
says so. A cons cell is two words, so on a 64-bit BEAM a ten-character string is 160 bytes for ten
characters, and every traversal chases pointers. Erlang's *Efficiency Guide* recommends binaries
for text, and `iolist` exists so that concatenation can be deferred rather than performed.

**Elixir fixed it by making `String` a UTF-8 binary** — the same runtime, the opposite decision,
and it is the single clearest before/after in this literature. Gleam followed.

The rest of the field:

- **Rust**: `String` is UTF-8; indexing is by byte offset and panics mid-character; `chars()` is an
  iterator; there is no O(1) character index and Rust says so.
- **Swift**: `String` is a sequence of **grapheme clusters** by default, with `unicodeScalars`,
  `utf8` and `utf16` views. The most semantically correct choice available and the most expensive:
  no O(1) indexing of anything.
- **Python 3** (PEP 393): a flexible representation — latin-1, UCS-2 or UCS-4 chosen per string —
  giving O(1) code-point indexing at the cost of three internal representations.
- **Go**: immutable bytes, UTF-8 by convention, `range` yields runes, `s[i]` yields a byte. The
  most honest of the pragmatic designs.
- **Java, JavaScript, C#**: UTF-16 code units, because all three adopted UCS-2 before Unicode 2.0
  introduced surrogate pairs in 1996. Every one of them leaks surrogates into user-visible APIs.
  This is the industry's largest and least reversible encoding mistake, and **two of our four
  targets are on the wrong side of it**.

The two documents worth reading against each other are the **UTF-8 Everywhere** manifesto and
Unicode **TR#29** on grapheme clusters: the first says pick one encoding and never index by
character, the second says what a "character" even is, and neither is free.

### 2.3 What follows for us

The Elixir lesson is the right one, minus the conflation Elixir could afford and we cannot.

> **A string is text. A binary is bytes. They are different types, and the bridge between them is
> an explicit encode or decode at a named encoding.**

Elixir makes `String` *be* a UTF-8 binary because BEAM has one representation. We have four, two of
them UTF-16, so we cannot say a string *is* bytes without lying on half our targets. What we can
say is that `utf8-encode : string → bytes` is total, exact, and agreed on all four — because UTF-8
is a specification, not a host convention.

That gives a portable text story with a published price:

| operation | price | note |
|---|---|---|
| literal, equality, concatenation | ~0 | native everywhere; already portable |
| `utf8-encode` / `utf8-decode` | **O(n) and a copy** on JS and Java; **~0 on Go** | Go's strings are already UTF-8 bytes |
| everything else | not portable | do it on the `bytes`, or reach for the host |

The price of `utf8-encode` on the UTF-16 hosts is real, and by obligation 4 it has to be justified:
what it buys is that **every other string operation becomes a byte operation**, and byte operations
agree everywhere. You pay one conversion to leave a region where the hosts disagree.

---

## 3. Binaries, and Erlang's bit syntax

### 3.1 The correction hamza asked for

The claim was that Erlang's binary type is a great invention not found in any other language. That
is **close to right, and worth being precise about**, because the precise version is more useful.

The idea of describing binary layouts declaratively has been invented repeatedly:

- **Ada 83 representation clauses** — bit-level record layout, static, since 1983.
- **PacketTypes** (McCann & Chandra, SIGCOMM 2000) — a type system for packet formats.
- **DataScript** (Back, GPCE 2002) and **PADS** (Fisher & Gruber, PLDI 2005) — data description
  languages for ad hoc and binary formats.
- **Cryptol** (Galois) — sequences of bits with type-level widths, aimed at cryptography.
- **Hardware description languages** — Verilog, VHDL, Bluespec, Chisel — where bit vectors are the
  *native* type and everything else is built on them.
- **Combinator libraries** — Haskell's `binary` and `attoparsec`, Rust's `nom`, Python's
  `construct`, **Kaitai Struct** as an external DSL.
- **Zig packed structs**, Rust bitfield crates, Nim bitfields.

So the *concept* is well-explored. What Erlang has that none of the others combines is three
things at once:

1. **It is in the pattern matcher.** `<<Len:16, Body:Len/binary, Rest/binary>>` is a pattern, not a
   parser call, so binary decoding uses the same construct as every other decomposition in the
   language.
2. **It is bidirectional.** The same syntax constructs and destructures. Every other entry above
   makes you write, or generate, two things.
3. **It has a zero-copy runtime.** Sub-binaries are references into a larger binary; match contexts
   let a sequence of matches walk one buffer without copying. Bit-level (non-byte-aligned)
   bitstrings were generalised in R12B — Gustafsson & Sagonas, *Bit-level Binaries and Generalized
   Comprehensions in Erlang* (Erlang Workshop 2005) — on top of the original design by Rogvall and
   Wikström around 1999.

Elixir and Gleam inherit all three, being BEAM languages. So: **not unique, but the combination is
Erlang's, and it is the reason protocol code in Erlang reads like the specification's packet
diagram.** hamza is right that it is a great invention; the correction is that it is a great
*integration* of ideas that existed separately.

### 3.2 What our four hosts actually have

| | byte array | zero-copy slice | non-byte-aligned |
|---|---|---|---|
| Go | `[]byte` | **free** — `b[i:j]` | no |
| JavaScript | `Uint8Array` | **free** — `subarray` | no |
| Java | `byte[]` | **a copy** — `Arrays.copyOfRange` | no |
| x86 | raw memory | **free** — a pointer | no |

Two results.

**Byte arrays are native everywhere and agree exactly.** They are the one representation all four
hosts share, which is exactly why §2.3 routes text through them.

**Zero-copy slicing is free on three hosts and a copy on Java** — unless a program uses
`ByteBuffer`, which is a different type with a different API. That is a price, it is
target-specific, and by §0 it must be published rather than smoothed over. It is also the single
most valuable thing Erlang's runtime does, so its absence on Java is the most important number in
this whole section, and it has **not been measured**.

**Non-byte-aligned bitstrings are native nowhere.** Erlang's most distinctive feature would be
emulated on all four, with shifts and masks at every access. That is a large, uniform, unavoidable
price — and by obligation 4 the question becomes what it buys. The answer is: exactly the formats
whose fields are not byte-aligned, which is real (protocol headers, compressed streams) and narrow.

### 3.3 What we cannot have

**The pattern-matching half.** There is no pattern matching in this language, and adding it is a
much larger decision than this one — it is ι of Coq's βδιζη, which [state.md §3](state.md) lists as
deliberately absent. Erlang's syntax is a *pattern* language; without patterns we get accessors.

### 3.4 What we can have, and it is more than it looks

**A bit-syntax library, written in the language, that fuses to nothing.** Because δ and β are a
partial evaluator, a layered accessor library collapses into inline arithmetic — which is what a
macro would produce, without a macro system. Demonstrated:

```lisp
(def u8    (fn (b i) (go.at-int b i)))
(def u16be (fn (b i) (go.| (go.<< (u8 b i) 8) (u8 b (go.+ i 1)))))
(def u32be (fn (b i) (go.| (go.<< (u16be b i) 16) (u16be b (go.+ i 2)))))
(fn (b) (u32be b 0))
```

emits

```go
func GenBitsyn(b []int) int {
	var i int = (0 + 2)
	return (((((b[0]) << 8) | (b[(0 + 1)])) << 16) | (((b[i]) << 8) | (b[(i + 1)])))
}
```

Three layers, four indexes, no calls. **The declarative-layout half of Erlang's bit syntax is a
library here and it costs nothing** — which is the same result as generics-with-no-generics and
fusion-with-no-fusion-rules, arriving a third time.

And the blemish in that output is §1.1's defect two: `(0 + 1)` and `(0 + 2)` are not folded, and
the unfolded constant was not duplicable so it got bound to a name. **Constant folding would make
this output perfect**, and its absence is most visible on the host with no optimiser to clean up
after us.

---

## 4. How the three fit together

They are one design, and the shape is forced by which representation the four hosts share.

```
      string  ──utf8-encode──▶  bytes  ──bits.get/set──▶  int
       text      (priced)      the one      (2-adic,        the
      opaque                   shared      windowed)      number
                            representation
```

- **`bytes` is the keystone.** It is the only container all four hosts have natively and agree on
  exactly. Text reaches it by an explicit, priced encode; numbers reach it by the bit operations of
  [bitwise.md](bitwise.md); Erlang's layout syntax becomes a library over it that reduces away.
- **`string` stays opaque** and gains exactly one portable operation it does not have: the bridge.
- **`int` stays machine-shaped and windowed**, with the window eventually *checked* rather than
  promised, which is the range-typed direction ADR 0003 already declared.

Nothing here is a language change. `bytes` is an opaque target-provided handle exactly like
`vec-f64`; the bit operations are `expr` primitives; the layout library is Oroboros. The two
language-level items are both bug fixes (§1.1).

---

## 5. What to measure next, in order

1. **Java's slice copy.** `Arrays.copyOfRange` against `ByteBuffer.slice` against Go's free slice,
   on a realistic decode loop. This is the largest unpriced item in the document and it decides
   whether a portable `bytes.slice` is admissible at all under §0.
2. **`utf8-encode` on the UTF-16 hosts.** How much does leaving the disagreement region actually
   cost on JS and Java?
3. **Constant folding.** Implement it (§1.1 defect two) and re-measure the four sieves and the bit
   accessor. The claim is that it is free-to-positive everywhere and matters most on windows.
4. **A range check from a `sig`.** Take one gauntlet program, declare a range, and see whether the
   existing linear fragment discharges it from loop bounds — or floods the output with unproven
   notes. That is the cheapest possible probe of N-E and it can be done in an afternoon.

## 6. The price list

What §0 obliges. Everything here is either measured or explicitly marked unmeasured.

| name | Go | JS | Java | x86 | status |
|---|---|---|---|---|---|
| `int` arithmetic, windowed & unchecked | 1× | 1× | 1× | 1× | today |
| …with the window **checked** | 1.65× | 1.81× | 1.19× | — | measured |
| …with **fixnum/bignum** promotion | 2.61× | 3.74× | 1.31× | ~1× | measured, **rejected** |
| `bits.and/or/xor/not`, int32 window | ~0 | ~0 | ~0 | ~0 | measured (semantics) |
| `bytes` get/set | ~0 | ~0 | ~0 | ~0 | native everywhere |
| `bytes.slice` | ~0 | ~0 | **a copy** | ~0 | **unmeasured** |
| `utf8-encode` | ~0 | O(n) copy | O(n) copy | ~0 | **unmeasured** |
| non-byte-aligned bitstrings | emulated | emulated | emulated | emulated | not proposed |
| layout accessors (`u16be` …) | **0** | 0 | 0 | 0 | demonstrated (§3.4) |

---

## 7. What must be settled before an integer can be implemented

hamza's point, and it is the reason nothing should be built yet: choosing "precision by default"
is one decision that drags eleven more behind it. Each of these has hosts that disagree, and each
is cheap to get wrong and expensive to change.

| | the question | why it is not obvious |
|---|---|---|
| 1 | **representation** | machine word, word-with-fast-path, or host bignum — and the answer differs per target (§1.2c) |
| 2 | **overflow** | wrap, trap, or promote. Three answers, and §1.2/§1.2b price them |
| 3 | **division rounding** | toward zero (Go, Java, x86, C99) or floor (Python, Haskell `div`). Both are defensible and they disagree on negatives |
| 4 | **remainder sign** | follows the rounding choice, and is the half everyone forgets |
| 5 | **division by zero** | Go panics, Java throws, x86 raises #DE, JavaScript gives `Infinity`. Already recorded in ADR 0012 as unsolved |
| 6 | **`int` vs `float` comparison** | is `(== 1 1.0)` true? On JavaScript they are the same value. On Go they are different types and it will not compile |
| 7 | **`int` → `float`** | **lossy above 2⁵³** — and silently so. If integers become exact, this is the one place exactness is lost, and it must be a named conversion rather than a coercion |
| 8 | **`float` → `int`** | rounding mode (truncate, nearest, even), and what NaN and out-of-range do. The hosts disagree on all three |
| 9 | **equality** | structural on a bignum, bitwise on a word — and they must agree, or `==` means two things |
| 10 | **ordering** | total on integers; but mixed int/float ordering inherits IEEE's NaN, which is not a total order |
| 11 | **constant folding** | at what precision? If `int` is mathematical, folding at arbitrary precision **is** the runtime semantics and ADR 0009 is satisfied for free. If `int` stays machine, folding must reproduce the target's width — the ADR 0009 hazard exactly |

Item 11 is worth pausing on, because it inverts. Today, compile-time integer folding would be an
ADR 0009 hazard — arbitrary precision at compile time, fixed width at runtime, which is the `0.1 +
0.2` bug in integer clothing. **If integers become exact by default, that hazard disappears**:
folding at arbitrary precision is folding with the runtime semantics.

So "precision by default" does not only cost. It buys back the one optimisation this document
found missing in emitted code (§1.1, defect two), and it buys it *soundly*.

## 8. The dependency graph, and the next move

Corrected from the earlier draft — several things are less blocked than they looked.

```
   NOTHING BLOCKS THESE                     BLOCKED ON A PRODUCT
   ────────────────────                     ────────────────────
   bytes (the vec-f64 pattern)              error results  (value, error)
   scalar bitwise (int32 window)            idiv's two answers
   bit-syntax library (demonstrated)        fold-range2's pair
   string → bytes bridge                    a bignum with an inline fast path
   constant folding                             │
   range CHECKING from a sig                    ▼
        │                              BLOCKED ON INTERVALS
        ▼                              ────────────────────
   binaries, hashes, bitsets           "precision by default, ranges optimise"
```

**Two things are genuinely load-bearing and everything else waits on them or on neither.**

**The product** is measured affordable (§1.2d) and has now been demanded six independent times —
`fold-range2`, Go's `(int, error)`, Java's `Map.Entry`, JS destructuring, one `idiv` producing two
results, and a bignum's fast path. The measurement says it costs nothing where our reducer already
makes things cost nothing. **It is ready to be decided.**

**Interval analysis** is what hamza's preferred integer design turns on: exact by default, with
declared ranges and inferred intervals choosing the representation. Its viability rests on one
unknown that nobody has measured:

> **How often can the compiler actually prove an integer stays in a machine word?**

If the answer is "nearly always", the design gives correctness and performance at once, which is
the whole point. If it is "about half", then half the arithmetic in every program falls to a
representation that costs 39× on multiplication, and the design is a trap.

**That number is measurable today, with machinery that already exists.** `emit/refine.go` already
collects facts from loop guards and discharges conjunctions of linear inequalities for array
bounds. Pointing it at every integer *operation* instead, over the seven gauntlet programs and the
four sieves, and reporting the percentage it can bound, is a contained experiment — and it is
exactly [ADR 0007](../decisions/0007-exploration-over-specification.md)'s method: explore against a
fixed test, kill candidates by measurement.

### The recommendation

1. **Measure interval provability.** The one unknown that decides the integer design, using
   machinery we have. Nothing about integers should be built before this number exists.
2. **Decide the product**, on the measurement in §1.2d. It is the most-demanded missing thing in
   the language and it is now priced.
3. **Then integers**, with §7's eleven questions answered in a specification before any code —
   which is the order [strings.md](strings.md) exists to enforce and the order booleans followed.
4. **`bytes` and scalar bitwise whenever convenient.** They are blocked on nothing, they cost no
   language change, and they are what makes binaries and hashes writable.

And two that are owed regardless, from §1.1: constant folding once §7's item 11 is settled, and the
gauntlet's migration off the retired portable layer
([assessment §3.4](../assessment-2026-08-19.md)).
