# Bytes, words, and bitwise operations

**Status, 2026-08-19. Research and candidates. Nothing built.** Written before any code, per the
rule [strings.md](strings.md) exists to enforce.

This reopens one entry in [ADR 0012](../decisions/0012-portable-integer-range.md)'s rejected list:

> **Division, bitwise operations and shifts.** … JavaScript's bitwise operators coerce to **32
> bits**, which would silently contradict the range.

That is still true. It is also not the whole story, and the part it leaves out is measurable.

---

## 1. What is true today

Every target already declares bitwise operations under its own host names, with no portability
claim:

| target | declared |
|---|---|
| `targets/go/builtin.oro` | `&`, `\|`, `^`, `&^`, `<<`, `>>` on `int` |
| `targets/java/lang.oro` | `&`, `\|`, `^`, `~`, `<<`, `>>`, `>>>` on `long` |
| `targets/js/builtin.oro` | `&`, `\|`, `^`, `~`, `<<`, `>>`, `>>>` on **`any`** |
| `targets/windows/x64.oro` | `and`, `or`, `xor`, `not`, `shl`, `shr`, `sar` on 64-bit registers |

So the operations are reachable, and `targets/js/builtin.oro` does already note that JavaScript's
"coerce to int32 in JS, which is a real semantic edge". What does not exist is any **portable**
name, any statement of what the operations *mean* independently of a host, and any enforcement:
`(js.& a b)` on a 40-bit value silently returns the wrong answer, and nothing in the pipeline
notices.

There is no byte array, no integer array, and no fixed-width type anywhere in the language.
`vec-f64` is the only container, it is opaque, and it holds doubles.

## 2. The measurements

Four hosts, run 2026-08-19. The x86 column was produced **by our own compiler** — the windows
target already declares `x64.shl`, so measuring it is one program.

| | Go | JavaScript | Java | x86-64 |
|---|---|---|---|---|
| `~5` | −6 | −6 | −6 | −6 |
| `-8 >> 1` | −4 | −4 | −4 | −4 |
| `-8 & 5` | 0 | 0 | 0 | 0 |
| `(1<<40) \| 0` | 1099511627776 | **0** | 1099511627776 | 64-bit |
| `3000000000 \| 0` | 3000000000 | **−1294967296** | 3000000000 | 64-bit |
| `1 << 32` | 4294967296 | **1** | 4294967296 | 4294967296 |
| `1 << 64` | **0** | 1 | **1** | **1** |
| `1 << 65` | **0** | 2 | 2 | 2 |

Three results, and they decide the design.

**AND, OR, XOR and NOT agree exactly on all four hosts** — for operands in
`[−2³¹, 2³¹−1]`. The boundary was measured, not assumed: `2147483647 & 1` and `-2147483648 | -1`
agree everywhere; `2147483648 | 1` and `3000000000 | 0` do not.

**JavaScript truncates to 32 bits, silently.** Every bitwise operator applies `ToInt32` and returns
a signed 32-bit result. It is a 32-bit island inside a number type that is exact to 2⁵³.

**Shift counts disagree three ways at four hosts.** `1 << 64` is **0** on Go, **1** on Java and
x86, and **1** on JavaScript for an entirely different reason (its count is masked by 31, so it
computed `1 << 0`). Go's specification says a shift by a count at or above the width yields zero;
Java's masks the count by 63; x86's `shl` masks by 63 in hardware; ECMAScript masks by 31.

## 3. The literature

### 3.1 Two's complement is 2-adic arithmetic — and this is the load-bearing one

Vuillemin, *On circuits and numbers* (1994), and Warren, *Hacker's Delight* (2002), set out what
this project needs: **`and`, `or`, `xor` and `not` are defined on the 2-adic integers ℤ₂, with no
width at all.** Two's complement at width *w* is the image of ℤ₂ in ℤ/2ʷ, and the bit operations
commute with that map.

Three consequences that answer [state.md §6](state.md)'s first question — *what does it mean
independently of any target?* — with no hedging:

- **`not x = −x − 1`**, exactly, at every width and in ℤ. The measurement confirms it: all four
  hosts give `~5 = −6`.
- **`and`, `or`, `xor` commute with sign extension.** That is *why* the hosts agree for in-range
  operands, and it is also why a 32-bit value costs nothing extra on a 64-bit host: no masking is
  needed, because the 64-bit answer already is the sign-extended 32-bit answer.
- **Arithmetic right shift is `⌊x / 2ⁿ⌋`** in ℤ. Exact, width-free, and it agrees on all four.

And the same framework says exactly which operations do **not** have a width-free meaning:

- **left shift** is `x · 2ⁿ` in ℤ, which the hosts truncate;
- **logical right shift** is meaningless without a width — it is defined by where the top is;
- **rotate, popcount, count-leading-zeros** are all width-quantified.

So the literature hands us a clean line. Four operations are exact mathematics; the rest are
machine operations wearing mathematical clothes. That line is not a compromise — it is the same
move [strings.md](strings.md) made, and the same move [arithmetic.md §4](arithmetic.md) made for
integers: **claim portability only over the region where the targets provably agree.**

### 3.2 WebAssembly: sub-word widths are access modes, not value types

CLAUDE.md names WebAssembly as the model — *minimal subject to lowering natively to every target at
zero cost*. So what Wasm actually chose is directly on-method, and it chose:

> **Four value types: `i32`, `i64`, `f32`, `f64`. No `i8`, no `i16`.**

Bytes and halfwords exist only as **memory access widths** — `i32.load8_u`, `i32.load16_s`,
`i32.store8`. A byte is something you load, not something you hold. The reason is exactly ours: the
hardware has no 8-bit registers worth modelling, so an `i8` value type would be a fiction the
backend has to legalize away.

LLVM took the other road — arbitrary-width `iN` — and legalization makes the cost invisible at the
point of use, which is the property this project most wants to avoid.

### 3.3 C's model, and its own retraction

`char`/`short`/`int`/`long` with implementation-defined widths, integer promotions that silently
change types mid-expression, undefined behaviour on signed overflow, and **undefined behaviour when
a shift count reaches the width**. `<stdint.h>` in C99 is the standard admitting the original was
wrong.

Two lessons, both directly applicable: if you have widths, **name them explicitly**; and never
leave shift-count behaviour to the host, because the hosts disagree and the disagreement is silent.

### 3.4 Java and Go made opposite deliberate choices

JLS §15.19 **masks** the shift count — by 31 for `int`, 63 for `long` — and says so, precisely
because C's undefined behaviour was a known wound. Java also has `>>>` because it has no unsigned
types.

The Go specification says a shift by a count at or above the operand's width yields **zero**. Go
chose the mathematically sensible answer; Java chose the hardware's answer.

Both are defensible, both are specified, and **they disagree** — measured above. Neither is a bug
to be worked around; it is a genuine semantic difference that any portable claim has to confront.

### 3.5 ECMAScript's ToInt32

ES §7.1: every bitwise operator converts its operands with `ToInt32` (or `ToUint32` for `>>>`),
operates on 32 bits, and yields a signed 32-bit result — except `>>>`, which yields an *unsigned*
32-bit result and therefore escapes the signed window upward.

`BigInt` (ES2020) has full-width bitwise. It is heap-allocated, and every published comparison puts
it an order of magnitude behind `Number` arithmetic. It is the emulation route and it is the one
[arithmetic.md §4](arithmetic.md) already recorded and refused for integers.

### 3.6 Why anyone wants this: the bit-twiddling literature

Knuth, TAOCP 4A §7.1, *Bitwise tricks and techniques*, and Warren's *Hacker's Delight* are the
canon. The concrete pulls for this project:

- **Hash functions.** FNV-1a, MurmurHash3, xxHash — every one is xor and shift. The word-count
  gauntlet program leans on a *host* dictionary; writing one natively needs a hash.
- **Pseudorandom generators.** xorshift (Marsaglia 2003), SplitMix64, PCG (O'Neill 2014). All
  bitwise. No program in this repository can generate a number.
- **Bit sets.** The sieve of Eratosthenes at one bit per number instead of one byte: 8× less
  memory. Our stencil measurement already showed that on memory-bound loops the memory traffic is
  the whole cost, so this is the case where the win should be largest.

### 3.7 The overflow question, which fixed widths drag in

Rust, Zig and Swift all take explicit widths — and all three then need **separate operations for
overflow behaviour**: `wrapping_add`, `checked_add`, `saturating_add`, Zig's `+%`. Rust's `<<`
panics in debug when the count reaches the width.

This is worth naming because it is the hidden cost of candidate E below: choosing fixed-width types
is not one decision, it is two, and the second one has three answers that the hosts also disagree
about.

## 4. Candidates

### A — Nothing. Bitwise stays host-named and Tier 2

**For:** zero growth; it already works; a program that wants bitwise picks a host and owns the
consequences, which is exactly [ADR 0001](../decisions/0001-parasite-model.md).

**Against:** there is no portable hash, no portable bit set, no portable checksum, and the gauntlet
is portable-first. And it leaves a live footgun: `js.&` is declared on `any`, truncates at 32 bits,
and says nothing about it. Doing nothing should at minimum mean *documenting* that.

### B — One portable family, windowed to int32

`bits.and`, `bits.or`, `bits.xor`, `bits.not`, `bits.shl`, `bits.shr`, `bits.ushr` — declared by
every target, defined only for operands in `[−2³¹, 2³¹−1]`.

**For:** the window is *measured*, not guessed, and inside it all four hosts agree exactly. JS is
native and free. Go, Java and x86 need no masking for and/or/xor/not, because those commute with
sign extension (§3.1). Most of §3.6's algorithms have well-known 32-bit forms — FNV-32,
xorshift32, MurmurHash3-32.

**Against:** 32 bits is a real ceiling; a 64-bit hash is better and three of four hosts can do one.
And the window is an obligation nothing checks — the same hole as `±(2⁵³−1)`.

### C — Two families, and let *covering* decide

B, plus `bits64.*` provided by Go, Java and windows and **not by JavaScript**.

**For:** this is what the capability graph is *for*. [ADR 0001](../decisions/0001-parasite-model.md)
says portability is a property the compiler computes, not a guarantee the language makes; a program
using `bits64.xor` is simply not portable to JS, and the covering check says so **by name, at
compile time**. That is strictly better than the status quo, where the same program is written with
`go.^` and is locked to one host although three could run it.

**Against:** two families is the suffix explosion that
[target-native.md](target-native.md) documents; and the second family only earns its place if a
program wants it.

### D — One family at the full `int` width, with JavaScript emulating

**For:** one integer, one set of names, no window to remember.

**Against:** JS pays 2–4× for a hi/lo split or an order of magnitude for `BigInt`, which fails
requirement 5 on that host. And it is strictly worse than C: C tells the truth (JS cannot do this)
where D hides it behind emulation. **Rejected** — this is the same emulation
[arithmetic.md §4](arithmetic.md) already refused for integers, and refusing it twice for the same
reason is consistency, not stubbornness.

### E — Fixed-width value types: `u8`, `i32`, `u64` in the language

**For:** precise, familiar, and what Rust and Zig do.

**Against:** three things, any one of which is disqualifying today.

1. It needs a **type system in the language**, and we deliberately have none — the checker runs on
   the residual ([types.md](types.md)) and the language has no annotations.
2. It multiplies the primitive table by width × signedness, which is the suffix explosion squared,
   on top of a type language that already has no constructors.
3. **WebAssembly did not do it** (§3.2), for our exact reason. And it drags in the overflow
   decision of §3.7.

**Rejected**, and the WebAssembly precedent is the argument rather than taste.

### F — Bytes as memory, not as a type

A `bytes` handle with `bytes.make`, `bytes.get`, `bytes.set`, `bytes.len`. **The element type is
`int`**, constrained to 0…255; there is no `u8` value.

Go has `[]byte`, JavaScript has `Uint8Array`, Java has `byte[]`, and the windows target already has
`x64.movzx` and `x64.movb`. All four are native, all four agree, and the index obligation
`0 ≤ i < len` is a conjunction of linear inequalities — **inside** the decidable fragment
`aindex` already uses.

**This is not a language change at all.** It is a target-provided opaque handle exactly like
`vec-f64`, plus ordinary `expr` primitives. Nothing structural, no new term kind, no new reduction
rule.

**For:** the Wasm answer; free on every host; and it is the thing the interesting algorithms
actually need.

**Against:** it is a container, and this project has been careful about containers. It also does
not by itself give an *integer* array, which is what a bit set wants (§6).

## 5. Recommendation

**F, then B, and C only when a program asks for it.**

### 5.1 What goes in

A module `bits`, declared natively by each of the four targets — not a new portable layer, and §5.3
argues why those are different things.

```lisp
(use bits)

(bits.and a b)   (bits.or a b)   (bits.xor a b)   (bits.not a)
(bits.shl a n)   (bits.shr a n)  (bits.ushr a n)
```

| | meaning, independent of any target | portable when |
|---|---|---|
| `and`, `or`, `xor` | the 2-adic operation (§3.1) | both operands in `[−2³¹, 2³¹−1]` |
| `not` | `−x − 1`, exactly | operand in the window |
| `shr` | `⌊x / 2ⁿ⌋`, exactly | operand in the window, `0 ≤ n < 32` |
| `shl` | `x · 2ⁿ`, **truncated to int32** | operand in the window, `0 ≤ n < 32` |
| `ushr` | logical shift of the low 32 bits; result in `[0, 2³²−1]` | operand in the window, `0 ≤ n < 32` |

`shl` and `ushr` are the two that are *not* exact mathematics, and they are marked as such rather
than smoothed over. `shl` truncates because JavaScript truncates and every other host can be made
to; `ushr` needs a width by definition.

**Emission.** `and`/`or`/`xor`/`not` and `shr` are the host operator on all four, with no masking
anywhere — that is §3.1's sign-extension property paying off. `shl` costs one extra instruction on
the 64-bit hosts (`int32(a << n)` on Go, `(int)(a << n)` on Java, `movsxd` on x86) and nothing on
JS. `ushr` costs a mask on the 64-bit hosts and nothing on JS.

**The shift count carries a refinement**, `(where (and (<= 0 n) (< n 32)))`. This is a conjunction
of linear inequalities, so it is **inside** the decidable fragment — unlike `d ≠ 0`
([assessment §3.2](../assessment-2026-08-19.md)) — and in practice the count is usually a literal,
which discharges immediately.

**The value window does not carry a refinement**, deliberately. It is a *range* obligation, and
[arithmetic.md §4](arithmetic.md) already records the identical hole for `±(2⁵³−1)`:

> "Stay inside the range" is an obligation on the programmer that nothing checks — and it is
> *precisely* a refinement.

Inventing a half-working range check here would produce an unprovable note at every call site.
What this does instead is make that hole **twice as motivated**, and it should be plugged once, for
both.

And byte arrays, per F:

```lisp
(bytes.make n)   (bytes.get b i)   (bytes.set b i v)   (bytes.len b)
```

with `(where (and (<= 0 i) (< i (bytes.len b))))` on get and set — discharged by the machinery that
already discharges `aindex`.

### 5.2 What stays out

**`bits64`** until a program wants it. The design is stated (candidate C) so that adding it later
is a declaration in three target files and a covering failure on the fourth, not a redesign.

**Fixed-width value types.** Rejected on WebAssembly's precedent (§3.2).

**`popcount`, `clz`, `ctz`, rotates.** Width-quantified, and the hosts' support is uneven —
JavaScript has only `Math.clz32`, and x86's `POPCNT` needs SSE4.2. These are exactly what a
*host-named* primitive is for: `go/bits.OnesCount64`, `java/Long.bitCount`, `x64.popcnt`. No
portable claim, available today, and a program that wants one has chosen its host.

### 5.3 The objection this has to answer

**"You just removed the portable layer. This puts one back."**

It is a fair challenge and the answer is that the two are different in kind.

The retired `portable-*.oro` layer was a **renaming of things every host already had** — `int.add`
for `+`, `io.print-line` for `fmt.Println`. Its whole content was the new name, and its cost was
hiding which host you were on. That is what got in the way.

`bits` is not a renaming, because **the hosts disagree**. Its content is the *restriction* — the
measured window in which four hosts compute the same answer, and the three operations that fall
outside it. That is the same thing [strings.md](strings.md) does, and strings are portable, in that
document's own words, "only by having almost no operations".

A portable name whose value is a renaming is decoration. A portable name whose value is a
**specification of the agreeing subset** is the parasite model working.

## 6. What would kill it, and what to measure first

**The bit set is the falsifier, and it needs something we do not have.** The canonical
demonstration is the sieve at one bit per number instead of one byte, and that needs an **integer
array**, which no target declares portably and which the language cannot construct
([construction.md](construction.md) is `vec-f64` only). So:

> **Bitwise operations without an integer array buy scalar wins only.** Hashing a value, packing
> two fields, advancing a PRNG state — all real, all small. The headline uses need a container that
> does not exist.

That is the honest ordering argument, and it is the strongest thing anyone can say against doing
this now.

Three measurements, in order:

1. **A bit-set sieve against a byte sieve**, hand-written, on all four hosts. If the memory win
   does not appear, the main case for bitwise weakens sharply and B is not worth it.
2. **FNV-1a and xorshift32**, generated against hand-written, on all four hosts. This is the scalar
   case and it is what B delivers on its own.
3. **`shl`'s extra truncation instruction** on Go, Java and x86 — is it free, or does it show up in
   a shift-heavy loop?

Two further falsifiers:

- **The count refinement produces noise.** If real programs shift by a variable the fragment cannot
  bound, every call site prints an unproven note and the diagnostic becomes something to ignore —
  which is worse than not having it.
- **`bytes` turns out to want an element type.** If programs immediately need `bytes.get` to
  produce something other than an `int`, the Wasm answer is wrong for us and E deserves reopening.

## 7. Cost

| | language | target files | backends |
|---|---|---|---|
| B (`bits`) | **nothing** | 7 declarations × 4 targets | nothing |
| F (`bytes`) | **nothing** | 4 declarations × 4 targets, one opaque type | nothing |
| C (`bits64`) | **nothing** | 7 more × 3 targets; JS declares none | nothing |
| E (fixed widths) | a type system | the suffix explosion squared | width legalization |

The recommendation costs **no language change at all** — which is the strongest argument for it and
also the reason it can wait behind the items in
[assessment §3](../assessment-2026-08-19.md) without anything being lost.
