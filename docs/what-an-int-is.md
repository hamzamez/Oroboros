# What an `int` is: the window, the target's word, and ℤ

Research. **Decided 2026-09-22: D, with portability reported — [ADR 0026](decisions/0026-an-int-is-an-integer.md).** Written 2026-09-21, on hamza's questions after `encoding/hex`'s `EncodedLen`
turned out to carry a precondition, `|n| ≤ 2⁵² − 1`, that Go itself does not have:

1. *why is `int` the portable window in a Go-only package?*
2. *what if `int` were the target's own integer?* — "Reading 2", and the one hamza wants;
3. *"my first suggestion was to make `int` mean unbounded by default"*;
4. **the decisive one**: *"`int` meaning a portable window is only true for these targets. If we add
   a target with int32, do we change the portable window again?"*

§1 is what `int` is today, §2–§3 are what it costs, measured, §4–§6 derive the design, and §7 tries
it against the next two packages (`strconv`, `encoding/binary`) before anything is built.

---

## 1. What `int` means today, in three layers

| layer | today | where |
|---|---|---|
| **semantics** | a mathematical integer; arithmetic is exact | [ADR 0003](decisions/0003-range-typed-integers.md), [ADR 0012](decisions/0012-portable-integer-range.md) |
| **legality** | every operation must be *proven* to stay in **W = ±(2⁵³−1)**, or the program is refused | [ADR 0019](decisions/0019-precision-by-declaration.md); one predicate, `emit/interval.go:64` |
| **representation** | the proven range picks a host type from the target's ladder, `(repr (int LO HI) (host "…"))`; above W there is exactly one rung, `big` | `targets/go/go.oro:26–31`, `emit/bigrep.go` |

W appears in three places in the compiler: the analysis's fits-test (`interval.go:64`), the
constant-folding bound (`core/reduce.go:1084`, ADR 0009) and `portableMaxLen` (`emit/target.go`).

## 2. Where W came from, and why the int32 question is decisive

ADR 0012 *measured* its way to W: three hosts, three different failure modes at two thresholds, and
exact agreement below 2⁵³. So W is not a number about integers. It is

```
W(S) = ⋂_{T ∈ S} Exact(T)          S = {go, java, js}, as of 2026-08-15
```

the meet of the targets' exact ranges — and the meet is **antitone in S**: adding a target can only
shrink it. hamza's question is exactly what this formula predicts. For an int32 target, the language
has two answers and **both are bad**:

- **shrink W to 2³¹.** Every existing program's legality changes because a target was *added*.
  A language whose meaning moves when its target set grows contradicts the premise of
  [ADR 0001](decisions/0001-parasite-model.md), that portability is a property computed per program.
- **keep W and make the new target emulate 53-bit integers**, which is what "anything promoted to the
  language works on every target" demands. Then 2⁵³ is revealed for what it is: **JavaScript's float64
  mantissa, frozen into the language** as of the day it had three targets.

**The literature ran this experiment and regretted it every time a type's extent depended on the
platform.** C's `int` is at least 16 bits, and C99 added `<stdint.h>` to escape it. Haskell's `Int`
covers at least [−2²⁹, 2²⁹−1]. Ada's predefined `Integer` is implementation-defined, and Ada style
guides say not to use it: declare your own range. Swift's and Go's `int` are 32 or 64 bits by
platform, which is why ADR 0012 already refused Go's `int`. Our W is the same mistake one level up:
not "depends on the platform" but "depends on which platforms existed on 2026-08-15".

## 3. What W costs at a host boundary — measured

### 3.1 The variance argument

A declaration `A → B` is a **sound abstraction** of a host function `A' → B'` exactly when it is a
supertype, which for functions is contravariant in the argument and covariant in the result
(Cardelli):

```
A → B  ⊒  A' → B'     ⟺     A ⊆ A'   and   B' ⊆ B
```

The Go survey spells `int64`, `uint` and `uint64` as our `int` (`survey.go:115`), and its own comment
argues this is safe because an operation on such a value is refused at the call site. That argument
is **correct for parameters** (`W ⊆ int64`: we promise to pass less) and **wrong for results**
(`int64 ⊆ W` is false: we must accept everything the host returns). Every generated declaration with
a 64-bit result claims a range the host does not honour.

### 3.2 How much of Go's API that is

Over the 4,867 platform-independent exported functions and methods in Go's API files, read one by one
before counting, because a Go `int` result is usually small (`RuneLen` is 1–4, an index is a length):

| | functions | share |
|---|---:|---:|
| an **explicitly 64-bit** integer (`int64`, `uint64`, `uint`, `uintptr`) in a **result** | **206** | **4.2%** |
| one only in **parameters** — sound to narrow | 137 | 2.8% |

Concentrated where the next packages are: `strconv` (`ParseInt`, `ParseUint`, and `Atoi` through a
64-bit `int`), `encoding/binary` (`Uvarint`, `Varint`, `ReadUvarint`, `ReadVarint`, `ByteOrder.Uint64`),
`time` (`Unix`, `UnixNano`, `Nanoseconds` — a nanosecond timestamp is ~1.8×10¹⁸, already past 2⁵³),
`math/bits` (16), `io` (byte counts, `Seek` offsets).

### 3.3 What happens today, probed

With the survey's generated `strconv` declarations:

- **the analysis is sound**: it reads an unranged host result as ⊤ = [−∞, +∞], so
  `(+ x 1)` on a `ParseInt` result is refused;
- **the emitter believes the false type**: `(strconv.Itoa x)` on a `ParseUint` result passes our
  checker, and **Go** refuses it — `cannot use x (variable of type uint64) as int value` — the refusal
  from the wrong place that literal-elements-2026-09-19 fixed for tables;
- **declaring the true range** today makes the value `big`: `*big.Int` on Go, where the host holds it
  in one machine word.

So a 64-bit host result can be printed or passed along, **never computed with** — unless it lies, or
it pays for a bignum. The window forces that choice at every covariant position.

### 3.4 W's size is invisible to every existing program

The experiment that decides how much a change of window can cost: every compile re-run with W widened
from ±2⁵³ to ±2⁶² (the analysis's own saturation limit). **All 456 compiles byte-identical; 2,361 of
2,413 operations proven, unchanged.** None of the 52 unproven operations is bounded within ±2⁶² either:
the analysis sees them as unbounded.

The proofs are about **boundedness**, and where the bound sits does not matter to them. W matters in
exactly two places: host boundaries (§3.1–3.3) and values that genuinely live above 2⁵³ — timestamps,
hashes, parsed numbers, varints, bit arithmetic.

## 4. The algebra: meaning, legality, representation

Three things were fused into "the window", and they separate cleanly.

**Meaning.** The ring (ℤ, +, ·, <). Every term that compiles denotes an element of ℤ — on every
target, the same one.

**Types.** Intervals I ⊆ ℤ ∪ {±∞}, the interval lattice under ⊆ (Cousot & Cousot 1977), which the
analysis already computes for every value.

**Representation.** A target T realizes a **sub-lattice by containment** — theories.md §5.5 already
says `repr` "chooses a realization for a sub-lattice of types by containment":

```
Rep_T = { (I_r, host type r) }          ρ_T(I) = the least r with I ⊆ I_r, if one exists
```

| T | realizations, least first |
|---|---|
| go | `byte`, `int8`, …, `int32`, **`int64` = [−2⁶³, 2⁶³−1]**, **`uint64` = [0, 2⁶⁴−1]**, `*big.Int` = ℤ |
| java | `byte`, `short`, `int`, **`long` = [−2⁶³, 2⁶³−1]**, `BigInteger` = ℤ |
| js | integer `Number` = [−(2⁵³−1), 2⁵³−1], `BigInt` = ℤ |
| windows | `db` … **qword**, signed and unsigned; limbs = ℤ |

Then:

> **A program compiles on T ⟺ every value's proven interval has a realization on T.**
> **Invariant: every target that accepts a program computes the same element of ℤ.**

That invariant is what the window was *for*. Legality on T now depends only on **(program, T)**, not
on a set of targets. **Portability over a set S** is "accepted by every T ∈ S" — computed, as
ADR 0001 says it should be — and W(S) survives as a **derived report**: the meet of the native
realizations of the targets you care about.

The int32 target, answered: it adds a row to the table. No existing program's meaning or legality on
any other target changes. The program either fits the new target's realizations — natively, or
through a wider rung its target file declares — or it is refused there, which is a true answer.

## 5. The candidates, against five properties

- **P1** meaning is independent of the target;
- **P2** meaning and legality are independent of the target **set** (the int32 property);
- **P3** a value the host holds in a machine word is held in one;
- **P4** loud, never silently slow (ADR 0019's reason for rejecting a boxed ℤ);
- **P5** a program with no host calls that compiles on one target compiles on all, **by construction**.

| | P1 | P2 | P3 | P4 | P5 |
|---|:---:|:---:|:---:|:---:|:---:|
| **A** today, plus a 64-bit rung for declared wide ranges | ✓ | ✗ (2⁵³ frozen) | partial — a boundary `int` still lies or goes `big` | ✓ | ✓ |
| **B** `int` := W_T everywhere (Reading 2, literally) | ✗ | ✓ | ✓ | ✓ | ✗ |
| **C** `int` := ℤ; representation by proof; `big` only when declared | ✓ | ✓ | ✓ | ✓ | in practice (§3.4), not by construction |
| **D** C, and at a host boundary an integer type is the **host's own** | ✓ | ✓ | ✓ | ✓ | in practice, and reported |

**Why P5 has to give way, derived rather than chosen.** P5 by construction needs one interval that
every target realizes natively, fixed in advance — a window. P2 forbids fixing it relative to the
current target set, so it would have to be realizable natively on *every possible* target: tiny (a
16-bit target gives 2¹⁵) or emulated (which breaks P3). So:

> **P2, P3 and P5-by-construction cannot all hold.** Today keeps P3 and P5 and pays with P2. D keeps
> P2 and P3 and turns P5 into a computed, reported property — which is ADR 0001's own stance on
> portability, applied to integers.

And §3.4 measured what giving up P5-by-construction costs on today's corpus: **nothing**. Every
proven operation sits far inside every target's realizations.

**For hamza's own case P5 was never at stake.** A Go package that nothing on another target will
import has no portability to lose. What the window costs *there* is P3 and the dishonest result types
of §3 — a `ParseInt` that can be printed and not computed with — and nothing in exchange.

**Why not B, which is what hamza asked for.** B gets P2 and P3 right and is right at the boundary. But
inside a program it makes `int` mean ±2⁶³ on Go and ±2⁵³ on JS — the same source denoting different
things. That is the project's own test for Tier 2 (targets disagree, observably), applied to its most
basic type. D keeps what B is right about — **at a host boundary, `int` is the host's integer** — and
keeps the meaning ℤ everywhere else.

## 6. What `int` would mean under D, precisely

**In a program**, `int` is ℤ with no premise. Each value's interval is inferred, the representation
comes from it on each target, and an operation with no provable bound on T is refused on T. **`big`
only by an explicit unbounded range**, `(int 0 +inf)`, exactly as ADR 0019 has it: bare `int` asks for
a machine word on every target that accepts the program, and an infinite bound asks for arbitrary
precision. ADR 0019's lattice `word ⊑ big` becomes per target: "word" is the target's widest native
realization.

**In a host declaration**, an integer type is the host's, stated as the range it is. Each target's
root module names them as manifest types, which exist since manifest-2026-09-16:

```lisp
(module go
  (type int    (int -9223372036854775808 9223372036854775807))
  (type int64  (int -9223372036854775808 9223372036854775807))
  (type uint64 (int 0 18446744073709551615))
  …)
```

and the survey's `scalar` map spells `int64` as `go.int64`, which it can justify (ADR 0023).

**At an export**, a bare `int` parameter's premise is the host's integer, because the caller is host
code. Formally it is an abstract sort of the module's theory, interpreted by each model
([ADR 0021](decisions/0021-declarations-are-theories.md)): what differs between targets is who is
calling, not what the function means.

**`EncodedLen` under D**, the question that started this:

```lisp
(sig EncodedLen ((n go.int)) go.int pure
      (where (and (<= -4611686018427387904 n) (<= n 4611686018427387903)))
      (host expr "hex.EncodedLen(%s)" (import "encoding/hex")))
```

The precondition is still f⁻¹ of the result range, computed the same way — but the range is now Go's,
so the bound is **exactly Go's own overflow boundary**, −2⁶² ≤ n ≤ 2⁶²−1 — asymmetric, because Go's `int` is two's complement, [−2⁶³, 2⁶³−1], and the symmetric window hid that. hamza's first reading — "safer
than Go, because of the overflow" — becomes literally true: the declaration states the one fact about
`EncodedLen` that Go leaves unchecked, and the compiler proves it at every call.

## 7. The next packages, on paper

hamza said to try the next packages if that is what it takes. Each function is stated as an algebra
first, then what it needs.

### 7.1 `strconv`

| function | the algebra | range | today | under D |
|---|---|---|---|---|
| `FormatInt(x, b)`, `Itoa` | an **injection** int64 → strings, onto the numerals of base b without leading zeros | param int64 | sound, narrowed to W | the host's int64 |
| `ParseInt(s, b, 64)`, `Atoi` | its **partial inverse**: `ParseInt ∘ FormatInt = id` on int64, a retraction whose domain is the numeral language | **result int64** | lies, or `big` | native `int64` |
| `ParseUint` | the same over uint64 | **result [0, 2⁶⁴−1]** | lies, or `big` | native `uint64` |
| `AppendInt`, `AppendUint` | FormatInt into a buffer, a consume | param | sound | host-buffers.md, unchanged |

**One thing D does not give, named rather than hidden:** `ParseInt(s, b, bitSize)` returns a value in
[−2^(bitSize−1), 2^(bitSize−1)−1], a **dependent** range. A result range that depends on an argument's
value is beyond `ensures`' linear fragment. With `bitSize` a literal, one declaration per width works
(`ParseInt32`, …); the general form is an open question, not a blocker.

### 7.2 `encoding/binary`

| function | the algebra | range | today | under D |
|---|---|---|---|---|
| `BigEndian.Uint16/32` | positional numeral base 256: `Σ bᵢ·256^(k−1−i)`, a **bijection** B^k ↔ [0, 256^k) | [0, 2¹⁶), [0, 2³²) | **declarable now** — inside W | unchanged |
| `BigEndian.Uint64` | the same bijection, k = 8 | **[0, 2⁶⁴−1]** | lies, or `big` | native `uint64` |
| `PutUint16/32/64` | its inverse, a write-borrow, precondition `len b ≥ k` (linear, like hex's) | — | 16/32 now | all |
| `Uvarint`, `PutUvarint` | LEB128, a **prefix-free code** ℕ → B*, `|code x| = max(1, ⌈bitlen x / 7⌉) ≤ 10`, so Kraft's inequality holds | value [0, 2⁶⁴−1] | lies, or `big` | native |
| `Varint`, `PutVarint` | Uvarint after **zig-zag**, `zz(x) = 2x` for x ≥ 0 and `−2x−1` otherwise — a bijection int64 ↔ uint64 (Protocol Buffers' encoding) | int64 | lies, or `big` | native |
| `MaxVarintLen16/32/64` | constants 3, 5, 10 | — | declarable now | unchanged |
| `Read`, `Write`, `Encode`, `Decode`, `Size` | reflective, over `interface{}` | — | refused as `any` | refused as `any` |

**One more thing the package demands that D does not supply:** `Uvarint` returns `(value, n)` with
`n > 0` success, `n = 0` a buffer too short, `n < 0` an overflow after −n bytes. That is a
**three-way sum encoded in a sign**, and the honest declaration is a `variant`. It is a separate
question from integers, and it is the kind of wall the standing rule says to name before building.

**Measured against today:** in `encoding/binary`, the 16- and 32-bit half is declarable now and the
64-bit half needs D's unsigned 64-bit realization. In `strconv`, every parser needs it and every
formatter already works. So the next two packages are exactly where D starts paying.

## 8. What D costs in the compiler

1. **The analysis must represent bounds to 2⁶⁴ exactly.** Today it saturates at 2⁶² (`interval.go:83`),
   so [0, 2⁶⁴−1] is ⊤ to it. **128-bit bound arithmetic** — double-word `bits.Add64`/`bits.Mul64`,
   saturating at 2¹²⁶ — is exact for every 64-bit range and costs a few instructions per bound
   operation. It is the one change that touches compile time, and compiletime-2026-09-21's gate is
   what will say if it costs.
2. **The fits-test becomes per target**: fits the widest native realization for the binding's class
   (word or big) on *this* target.
3. **Constant folding** folds within the target's realization of the value, so ADR 0009 holds per
   target — it already folds only where compile time and run time agree.
4. **Target files**: the 64-bit rungs as data — `int64` and `uint64` on Go; `long` on the JVM (whose
   `uint64` needs a measured choice: `long` with `Long.compareUnsigned`/`divideUnsigned`, or
   `BigInteger`); `BigInt` above 2⁵³ on JS; signed and unsigned qwords on windows.
5. **Manifest host integer types** in each target's root module, and the survey's `scalar` map made
   honest.
6. **Refusal texts** name the target's realization instead of "±(2^53−1)"; the emission baseline takes
   that as an accepted change.

## 9. What would refute D, measured before building

- **JavaScript.** A value proven in (2⁵³, 2⁶³] is a `BigInt` on V8. Measure `BigInt` against a split
  representation (two 32-bit halves in Numbers) for add, multiply and compare, at the sizes a 64-bit
  value actually has — before a JS rung is chosen. [bigrepr-2026-09-03](../gauntlet/results/bigrepr-2026-09-03.md)
  found the host's bignum beating our limbs by 75× on V8, so the prior is `BigInt`; it is still a
  prior.
- **The JVM's `uint64`**: `long` with unsigned operations, against `BigInteger`.
- **A library that accidentally stops being portable.** Under D, a module whose values are proven in
  (2⁵³, 2⁶³] compiles natively on Go and is refused on JS. The check's sweep already compiles every
  source on all four targets, so it is seen. A module-level `(portable go js java windows)` claim, which
  the check would enforce, is one way to make it a declaration instead of a surprise.
- **Compile time** of 128-bit bounds, against the gate.

## 10. Deliberately not proposed

- **Wrapping semantics** of any kind (Reading 1). Every proof the compiler makes is a theorem about ℤ.
- **A separate unsigned type family.** A range already says unsigned: `(int 0 18446744073709551615)`.
- **`int` meaning the target's word inside programs** (B). Right at the boundary, Tier 2 inside.
- **A silent bignum** (ADR 0019's A). `big` stays something a programmer writes.

## 11. Recommendation

**D**, in these steps:

1. **An ADR** that supersedes the part of ADR 0012 making W a language constant. The meaning ADR 0012
   chose, exact integers, stays. What goes is the claim that one interval is the language's `int` on
   every target. ADR 0019 is amended: its `word` is the target's widest native realization.
2. **Measure first**: JS `BigInt` against a split representation; JVM unsigned `long` against
   `BigInteger`.
3. **Build, in dependency order**: exact 128-bit bounds in the analysis; per-target realizations and
   the per-target fits-test; manifest host integer types and an honest survey map; `encoding/hex` and
   `unicode/utf8` re-declared under it, where `EncodedLen`'s precondition becomes Go's own; then
   **`strconv` and `encoding/binary`**, the packages that demanded it.

The one question this leaves open is the one §5 cannot settle by derivation: **whether portability
should be the default a program has to opt out of** (P5, today) **or a property the compiler reports**
(D). The algebra says both cannot be had together with P2; which to give up is a decision about what
kind of language this is, and it is hamza's.
