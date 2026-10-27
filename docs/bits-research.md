# Research: ℤ/2ⁿ and 𝔽₂ⁿ, the integers modulo 2ⁿ and the bits

2026-10-27. **Research, not a decision.** The first of the seven walls
[llvmsurvey-2026-10-27](../gauntlet/results/llvmsurvey-2026-10-27.md) found (§4.1), and the first
decision step of the 10-24 plan's item 6b. hamza: "make a research document explaining what it is? how
does it relate to our language? llvm? the other targets? the type system. with plenty of examples. and
the recommendations so I can make an informed decision".

It supersedes the candidates of [spec/bitwise.md](spec/bitwise.md) (2026-08-19), which was written before
the language had sets as types, tables, a checked word (ADR 0019) or facts that choose the representation
(ADR 0079). §3.4 says what that changes.

**The short version:**
- **ℤ/2ⁿ** is ℤ with 2ⁿ identified with 0: the arithmetic of an n-bit machine word. **𝔽₂ⁿ** is n
  independent bits under xor and and: a set of positions. A machine word carries both structures on one
  carrier, and LLVM's `iN` is exactly that carrier.
- **Every one of these operations has a meaning on ℤ itself**, with no width: and, or, xor and not are
  the 2-adic operations, and the n-bit word is ℤ seen through the map π_n(x) = x mod 2ⁿ. π_n is at once a
  ring homomorphism and a Boolean one.
- **So the language does not need a new sort.** It needs the Boolean operations on ℤ and π_n, and one
  theorem (§1.5): a term built from +, −, ·, and, or, xor, not and left shift, read through π_n, may be
  computed in n-bit wrapping machine arithmetic. Division, comparison and right shift are excluded, and
  the exclusion is what LLVM's separate signed and unsigned instructions are.
- **Recommended** (§8): the Boolean operations on ℤ promoted to the language, π_n written as `mod 2ⁿ`, and
  wrapping computed by the theorem. Each target realizes them by the facts: natively everywhere for
  n ≤ 32, and natively for n = 64 except on JavaScript, which uses the split pair measured in September.

---

## 1. What they are

### 1.1 ℤ/2ⁿ: the integers modulo 2ⁿ

Two integers are **congruent modulo m**, a ≡ b (mod m), when m divides a − b. Congruence respects
addition and multiplication: if a ≡ a′ and b ≡ b′ then a + b ≡ a′ + b′ and a·b ≡ a′·b′. So the classes
[a] = {a + km : k ∈ ℤ} form a ring, **ℤ/m**, and the map

  π : ℤ → ℤ/m,  π(a) = [a]

is a surjective ring homomorphism with kernel mℤ. That is the first isomorphism theorem, in any
algebra text (Lang, *Algebra*, II §4). For m = 2ⁿ this ring is the arithmetic of an n-bit word.

**Examples, n = 8** (classes of 256):

| in ℤ/2⁸ | because in ℤ |
|---|---|
| [200] + [100] = [44] | 300 = 256 + 44 |
| [16] · [16] = [0] | 256 ≡ 0: **zero divisors**, so ℤ/2⁸ is not a field |
| [255] = [−1] | 255 − (−1) = 256: one class, two names |
| [3] · [171] = [1] | 513 = 2·256 + 1: 3 has an inverse |
| [2] · x = [1] has no solution | 2x is even, and every member of [1] is odd |

The units, the elements with an inverse, are exactly the odd classes. Even classes cannot be divided by.
**So ℤ/2ⁿ has no division**, and a "division" a machine performs on n-bit words must be something else.
§1.3 says what.

### 1.2 Two names for each class: the unsigned and the signed residue

A class is a set; a machine stores one member of it. A **system of representatives** (a section of π)
picks one. Two are used everywhere:

- the **unsigned residue**, σ_u : ℤ/2ⁿ → U_n = [0, 2ⁿ), the member in U_n;
- the **signed residue**, σ_s : ℤ/2ⁿ → S_n = [−2ⁿ⁻¹, 2ⁿ⁻¹), the member in S_n (two's complement).

The composites are functions ℤ → ℤ, and they are the operations this document is about:

  π_n(x) = σ_u([x]) = x mod 2ⁿ  (the Euclidean remainder, always in [0, 2ⁿ))
  π_n^s(x) = σ_s([x]) = π_n(x + 2ⁿ⁻¹) − 2ⁿ⁻¹

| x | π₈(x) | π₈ˢ(x) | what a host calls it |
|---|---|---|---|
| 300 | 44 | 44 | Go `uint8(x)` at x = 300, C `(unsigned char)300` |
| 200 | 200 | −56 | Go `int8(x)` at x = 200, Java `(byte)200` |
| −1 | 255 | −1 | Go `uint8(x)` for `x = -1` |
| 2⁶³ | 0 | 0 | |
| 65,535 | 255 | −1 | Win32's `LOBYTE` and its signed reading |

**A machine word stores a class, and the program decides which residue it means.** The bits
`11111111` are 255 under σ_u and −1 under σ_s. Go and Rust record that choice in a type (`uint8`,
`int8`); LLVM records it in the instruction (§4).

### 1.3 What a class determines, and what only a residue does

An operation f on ℤ **descends** to ℤ/2ⁿ when a ≡ a′ ⇒ f(a) ≡ f(a′); then f computes on classes, whichever
residue holds them. Which of ours do:

| operation | descends? | witness, n = 8 |
|---|---|---|
| +, −, · | **yes** (π is a ring homomorphism) | |
| and, or, xor, not (§1.4) | **yes** (π is a Boolean homomorphism, §1.5) | |
| left shift x·2ᵏ | **yes** (multiplication by a constant) | |
| a < b | no | [255] = [−1], and 255 < 1 is false while −1 < 1 is true |
| a / b (truncating) | no | 255 / 2 = 127, but −1 / 2 = 0, and [127] ≠ [0] |
| a % b | no | 255 % 2 = 1, but −1 % 2 = −1 ≡ 255 |
| right shift ⌊a / 2ᵏ⌋ | no | ⌊255 / 2⌋ = 127, but ⌊−1 / 2⌋ = −1 ≡ 255 |
| popcount (number of ones) | no | 255 has 8 ones; −1 has infinitely many (§1.4) |
| equality | yes, as equality of classes | [255] = [−1] |

**This table is the whole design of LLVM's integer instructions** (§4): one `add`, `sub`, `mul`, `and`,
`or`, `xor` and `shl`, because they descend; a signed and an unsigned version of `div`, `rem`, the right
shift and every order comparison (`sdiv`/`udiv`, `srem`/`urem`, `ashr`/`lshr`, `slt`/`ult`), because they
do not. Each of those is ℤ's operation applied to one residue or the other.

### 1.4 𝔽₂ⁿ: the bits

𝔽₂ = {0, 1} is the field with two elements: addition is xor, multiplication is and. 𝔽₂ⁿ is its n-fold
power, the n-bit strings under bitwise xor and and. It has two readings, and both matter:

- **A vector space over 𝔽₂**, with xor as addition. Linear maps are n×n bit matrices. A rotation, a bit
  reversal or a byte swap is a permutation matrix, and CRC's table step is a linear map.
- **A Boolean ring**, x·x = x, under and. By Stone's representation theorem it is a ring of sets: a bit
  string is **the set of positions holding a 1**, and

  | bits | sets | example (n = 8) |
  |---|---|---|
  | x and y | x ∩ y | `0b1100 and 0b1010 = 0b1000` |
  | x or y | x ∪ y | `0b1100 or 0b1010 = 0b1110` |
  | x xor y | symmetric difference | `0b1100 xor 0b1010 = 0b0110` |
  | not x | complement in {0, …, n−1} | `not 0b00001111 = 0b11110000` |
  | popcount x | \|x\| | popcount `0b1011` = 3 |
  | x and 2ⁱ ≠ 0 | i ∈ x | |

Win32's window styles are such sets: `WS_OVERLAPPEDWINDOW` is `WS_OVERLAPPED | WS_CAPTION | WS_SYSMENU |
WS_THICKFRAME | WS_MINIMIZEBOX | WS_MAXIMIZEBOX`, the union of six one-position sets, 0x00CF0000.

**A warning about the name.** 𝔽₂ⁿ, n bits under xor, is a vector space, **not** the field 𝔽_{2ⁿ} =
GF(2ⁿ). GF(2ⁿ) is polynomials over 𝔽₂ modulo an irreducible polynomial of degree n, used by AES,
GCM and Reed–Solomon codes. Its multiplication is LLVM's `clmul` (carry-less multiplication, the
product in 𝔽₂[x]) followed by a reduction. CRC-32 lives there too: the checksum is the remainder of the
message, read as a polynomial over 𝔽₂, divided by a fixed polynomial of degree 32. §2 has both.

### 1.5 One carrier, and the 2-adic integers that hold both without a width

An n-bit word is ℤ/2ⁿ and 𝔽₂ⁿ on the same 2ⁿ strings. LLVM and SMT-LIB call this structure a
**bit-vector** (SMT-LIB's QF_BV theory). The two structures meet in identities that every bit trick
uses:

  a + b = (a xor b) + 2·(a and b)  (the sum is the carry-less sum plus the carries)
  not a = −a − 1  (in ℤ/2ⁿ, and, below, in ℤ)
  a − b = a + not b + 1
  a·2ᵏ = a shifted left by k

**Without a width.** The **2-adic integers** ℤ₂ are the inverse limit of ℤ/2 ← ℤ/4 ← ℤ/8 ← ⋯: infinite
bit strings extending leftward, with carries that never stop. ℤ sits inside ℤ₂ as the strings that are
**eventually constant** (Vuillemin, "On circuits and numbers", IEEE TC 1994; Warren, *Hacker's
Delight*):

| x ∈ ℤ | its 2-adic expansion | as a set of positions |
|---|---|---|
| 5 | …0000101 | {0, 2}, finite |
| 0 | …0000000 | ∅ |
| −1 | …1111111 | ℕ, all of it |
| −6 | …1111010 | ℕ ∖ {0, 2}, cofinite |

So **and, or, xor and not are total functions on ℤ**, with no width: they act position by position, and
an eventually constant string stays eventually constant. In sets, (ℤ, and, or, not) is the
**finite–cofinite Boolean algebra on ℕ**: the naturals are the finite sets, the negatives the cofinite
ones. That is why `~5 = −6` on all four hosts (bitwise-2026-08-19): it is −5 − 1. Right shift is ⌊x/2ᵏ⌋
and left shift is x·2ᵏ, both exact on ℤ.

**The theorem this document rests on.** Let π_n : ℤ → U_n keep positions 0 … n−1, which is x mod 2ⁿ.
Then:

> **π_n is simultaneously a ring homomorphism (ℤ, +, −, ·) → ℤ/2ⁿ and a Boolean homomorphism
> (ℤ, and, or, xor, not) → 𝔽₂ⁿ.**

The first is §1.1. The second holds because the bitwise operations act on each position independently,
and π_n forgets positions ≥ n.

**Corollary (fusion).** Let t be a term built from variables, integer constants, +, −, ·, and, or, xor,
not and left shift by a constant. Then

  π_n(t(x₁, …, x_k)) = t_n(π_n x₁, …, π_n x_k),

where t_n is t computed with **n-bit wrapping** operations. *Proof*: induction on t, each step one of the
two homomorphisms. ∎

The corollary is what lets the language mean ℤ while the machine computes n bits. It fails for anything
in the "no" column of §1.3, and the failure is a wrong answer, not a slow one:

  π₈((200 + 100) / 2) = π₈(150) = 150,  but computed wrapping: (200 +₈ 100) / 2 = 44 / 2 = 22.

### 1.6 Width-dependent functions

Some useful functions need n, because they look at the top of the word. Each is still an ordinary
function of (n, x) on U_n, so each is definable from the operations above:

| function | definition on x ∈ U_n | n = 8 example |
|---|---|---|
| bit length, len(x) | least L with x < 2ᴸ | len(0b00010110) = 5 |
| leading zeros | n − len(x) | 3 |
| trailing zeros, ν₂(x) | the 2-adic valuation, for x ≠ 0 (width-free) | ν₂(0b00010110) = 1 |
| popcount | \|x\| as a set; a function on ℕ, not on ℤ | 3 |
| rotate left by k | π_n(x·2ᵏ) or ⌊x / 2ⁿ⁻ᵏ⌋ | rotl₈(0b10010110, 1) = 0b00101101 |
| byte swap | the permutation of bytes, n a multiple of 8 | bswap₁₆(0x1234) = 0x3412 |
| bit reverse | position i ↦ n−1−i | rev₈(0b00000001) = 0b10000000 |
| n-bit complement | (2ⁿ − 1) xor x, which is π_n(not x) | ¬₈(0b00001111) = 0b11110000 |
| carry-less multiply | Σ over i ∈ b of a·2ⁱ, summed with xor | clmul(0b11, 0b11) = 0b101 |

Rotations are the action of the cyclic group ℤ/n on positions: rotating by a and then b is rotating by
a + b. A rotation, a reversal or a byte swap preserves and, or, xor and not (a Boolean automorphism), but
not +.

---

## 2. Where programs need it

Programs, in Go as written today, and what each needs. The surprise: **most need only the Boolean
operations on naturals, which are exact on ℤ and need no wrapping at all.** Only hashes, random
generators and cryptographic rounds need the ring ℤ/2ⁿ.

| program | Go's package | needs | wraps? |
|---|---|---|---|
| FNV-1a, 32 and 64 bits | `hash/fnv` | xor, · in ℤ/2ⁿ | **yes** |
| CRC-32 | `hash/crc32` | and, xor, ⌊/2⁸⌋ on [0, 2³²) | no |
| a bit set (sieve at one bit per number) | `math/bits` | and, or, ·2ᵏ on [0, 2⁶⁴), popcount | no |
| xorshift, SplitMix64, PCG | `math/rand/v2` | xor, shifts, + and · in ℤ/2⁶⁴ | **yes** |
| SHA-256's rounds | `crypto/sha256` | rotations on 32 bits, + in ℤ/2³² | **yes** |
| UTF-8 encoding | `unicode/utf8` | and, or, ⌊/2⁶⌋ | no |
| varints | `encoding/binary` | and, or, ⌊/2⁷⌋ | no |
| byte order | `encoding/binary` | byte swap, or ⌊/2⁸ᵏ⌋ and 0xFF | no |
| Win32 flags, `RGB`, `LOWORD`/`HIWORD` | (TinyRetroPad) | or, and, ⌊/2¹⁶⌋, ·2⁸ | no |
| Win32's `GET_X_LPARAM` | (TinyRetroPad) | π₁₆ˢ, the signed residue | the signed residue |
| a float's bits | `math.Float64bits` | a bijection F64 → U₆₄ | (the floats wall) |

The examples below are written in J with **provisional spellings**: `&` and, `|` or, `⊕` xor (J's `^`
is the power `2^64`), `~` not, `<<` and `>>` the shifts, and `x mod m` the Euclidean remainder. The
spelling is decision **B6**, and none of these names is in the language today.

### 2.1 FNV-1a, 64 bits: the ring

Go's `hash/fnv` (`sum64a.Write`):

```go
for _, c := range data {
    hash ^= uint64(c)
    hash *= prime64          // 1099511628211, wrapping
}
```

In the language, with π₆₄ written where the wrap happens:

```
offset64 = 14695981039346656037
prime64 = 1099511628211        // 2^40 + 435

fn fnv1a64(data ∈ Array(0..255)) (0..2^64-1) {
    loop i = 0, h = offset64 {
        i < len(data) -> again(i + 1, ((h ⊕ data[i]) * prime64) mod 2^64)
        else -> h
    }
}
```

What the compiler must see. h ∈ [0, 2⁶⁴) and data[i] ∈ [0, 255], so h ⊕ data[i] ∈ [0, 2⁶⁴) (xor of
naturals below 2ⁿ stays below 2ⁿ). The product can reach 2¹⁰⁴, far above any word, but it sits under
`mod 2^64`, built only from xor and ·. **By fusion it is computed as one wrapping 64-bit multiply**, and
no 104-bit value ever exists. Without fusion the program is refused today: the product is not proven
inside the word (ADR 0019).

### 2.2 FNV-1a, 32 bits

```
fn fnv1a32(data ∈ Array(0..255)) (0..2^32-1) {
    loop i = 0, h = 2166136261 {
        i < len(data) -> again(i + 1, ((h ⊕ data[i]) * 16777619) mod 2^32)
        else -> h
    }
}
```

On JavaScript this is native: `h = Math.imul(h ^ b, 16777619) >>> 0` (§5.3).

### 2.3 CRC-32: no wrapping at all

Go's `hash/crc32`, the table-driven IEEE form (`simpleUpdate`):

```go
crc = ^crc
for _, v := range p {
    crc = tab[byte(crc)^v] ^ (crc >> 8)
}
return ^crc
```

```
fn crc32(tab ∈ Array(0..2^32-1), p ∈ Array(0..255)) (0..2^32-1) {
    mask = 2^32 - 1
    c = loop i = 0, crc = mask {                         // ^0 in 32 bits
        i < len(p) -> again(i + 1, tab[(crc ⊕ p[i]) & 255] ⊕ (crc >> 8))
        else -> crc
    }
    c ⊕ mask                                             // Go's ^crc, in 32 bits
}
```

Every operation is exact on ℤ and stays in [0, 2³²): xor and and of naturals below 2³² stay below
2³², and ⌊crc / 2⁸⌋ < 2²⁴. Note `c ⊕ mask`, not `~c`. **Go's `^crc` on a `uint32` is the 32-bit complement
¬₃₂, while not on ℤ is −c − 1**, a negative number. The 2-adic not and the n-bit complement differ, by
π₃₂ (§1.6), and a program says which one it means.

The table:

```
fn crc_table() (Array(0..2^32-1)) {
    array { loop j = 0, c = k {
                j < 8 -> again(j + 1, if (c & 1) == 1 { 3988292384 ⊕ (c >> 1) } else { c >> 1 })
                else -> c
            } : k ∈ 0..255 }
}
```

3988292384 is 0xEDB88320, the reflected polynomial. J has no hexadecimal integer literals today, also
B6. This is 𝔽₂[x] arithmetic (§1.4) written with xor and shifts on naturals.

### 2.4 A bit set

```
fn sieve_bits(n ∈ 2..2^20) (Nat) {
    var w := array { 0 : k ∈ 0..n/64 }                   // n bits in ⌈n/64⌉ words
    ...  w[j >> 6] := w[j >> 6] | (1 << (j & 63))        // set bit j
    ...  (w[i >> 6] >> (i & 63)) & 1 == 0                // test bit i
}
```

Everything is exact on naturals. w[k] | 2^(j mod 64) < 2⁶⁴ because both operands are below 2⁶⁴, so the
words never leave U₆₄ and nothing wraps. The facts the analysis needs are the closure facts of §6.3.

### 2.5 SplitMix64 (Steele, Lea and Flood, OOPSLA 2014): the ring, with shifts

```
fn splitmix64(x ∈ 0..2^64-1) ((0..2^64-1, 0..2^64-1)) {
    s = (x + 11400714819323198485) mod 2^64              // 0x9E3779B97F4A7C15
    z1 = ((s ⊕ (s >> 30)) * 13787848793156543929) mod 2^64
    z2 = ((z1 ⊕ (z1 >> 27)) * 10723151780598845931) mod 2^64
    (s, z2 ⊕ (z2 >> 31))
}
```

Here **fusion must stop at `>>`**. `(s ⊕ (s >> 30)) * c mod 2^64` fuses the xor and the multiply, but the
right shift's operand s must already be the residue. It is, because s was bound to a `mod 2^64`. A term
like `((a * b) >> 30) mod 2^64`, with the product unreduced, does not fuse (§1.3), and the compiler
needs the product exactly: it is refused unless proven inside the word.

### 2.6 xorshift64 (Marsaglia, JSS 2003)

```
fn xorshift64(x ∈ 1..2^64-1) (1..2^64-1) {
    a = (x ⊕ (x << 13)) mod 2^64
    b = a ⊕ (a >> 7)
    (b ⊕ (b << 17)) mod 2^64
}
```

The declared result 1..2^64-1, never zero, is a true claim: xorshift's map is invertible on 𝔽₂⁶⁴ and
fixes 0. **No decision procedure in this document proves it.** It is a theorem of linear algebra over
𝔽₂, and the claim is refused until a lemma states it (§6.3). Declaring `0..2^64-1` is what a program
would write.

### 2.7 SHA-256's Σ₀: rotations

```
fn rotr32(x ∈ 0..2^32-1, k ∈ 1..31) (0..2^32-1) { (x >> k) | ((x << (32 - k)) mod 2^32) }
fn big_sigma0(a ∈ 0..2^32-1) (0..2^32-1) { rotr32(a, 2) ⊕ rotr32(a, 13) ⊕ rotr32(a, 22) }
```

`rotr32` is a definition in the language. A target recognizes it or provides it: Go's
`bits.RotateLeft32(x, -k)`, Java's `Integer.rotateRight`, LLVM's `fshr`. That is the capability graph's
"defined, and a target may provide" (§8, B5).

### 2.8 Win32: flags, packed fields, and one signed residue

```
WS_OVERLAPPEDWINDOW = 13565952                           // 0x00CF0000, a union of six flags

fn rgb(r ∈ 0..255, g ∈ 0..255, b ∈ 0..255) (0..2^24-1) { r | (g << 8) | (b << 16) }
fn loword(l ∈ 0..2^32-1) (0..2^16-1) { l & 65535 }
fn hiword(l ∈ 0..2^32-1) (0..2^16-1) { (l >> 16) & 65535 }

// windowsx.h's GET_X_LPARAM: ((int)(short)LOWORD(lp)). A mouse x left of a window is negative.
fn get_x_lparam(lp ∈ 0..2^64-1) (-2^15..2^15-1) { ((lp & 65535) + 2^15) mod 2^16 - 2^15 }
```

All but the last are exact on naturals. The last is π₁₆ˢ, the signed residue of §1.2, written with
π₁₆. A machine computes it as one sign extension (`movsx`, LLVM's `trunc` then `sext`); B4 decides
whether it gets a name.

---

## 3. How it relates to the language today

### 3.1 What exists

- **An `Int` is an integer** (ADR 0026), and its operations `+ - * / %` and the comparisons are ℤ's
  (integers.md §0a, §3, §4: `/` truncates, `%` takes the dividend's sign). **Bounded by default**
  (ADR 0019): an operation not proven inside the target's word is refused, or trapped under `-checked`.
- **The bitwise operations are not in the language.** integers.md §0a: "Bitwise and shifts are NOT
  promoted", because V8 coerces to int32 inside the integer window (bitwise-2026-08-19). Each target
  declares them under host names with no portability claim: `go.&`, `go.|`, `go.^`, `go.<<`, `go.>>` on
  `Int` (targets/go/builtin.oro), the same and `>>>` on Java, on `Any` on JavaScript, and `x64.and`,
  `x64.xor` and the rest on windows.
- **The unsigned word is a representation, not a type** (ADR 0033): a value proven in [0, 2⁶⁴) is held
  in Go's `uint64`. targets/go/u64.oro already states this document's algebra in its comments: "r : ℤ →
  ℤ/2^64 the residue map, a surjective ring homomorphism … Order and division do NOT commute with r".
  It already declares the modular operations as target-native names, `go.u64+` and `go.u64*`, "uncounted,
  with the set U as its result's fact", and records that "the language's ℤ/2^n is not built (the wall
  hash/fnv will hit)".
- **`math/bits` is declared by hand** (mathbits-2026-09-23): `Len`, `LeadingZeros`, `TrailingZeros`,
  `OnesCount`, `Reverse`, `ReverseBytes`, `RotateLeft`, and the double-word `Add`, `Sub`, `Mul`, `Div`, as
  host names on Go only.
- **How the other hosts would hold U₆₄ was measured** (u64repr-2026-09-22): the JVM's `long` holding the
  residue, with `Long.compareUnsigned`, `divideUnsigned` and `remainderUnsigned`, is within 1.25× of a
  signed `long` on multiply-add; on V8 a **split pair**, two Numbers holding the high and low 32 bits, is
  4.8× faster than `BigInt` and about 10× slower than a plain Number. The split pair is not built.
- **No program wraps.** No program in `examples/` or `lib/` uses a bitwise operation, and none hashes,
  checksums or generates a random number.

### 3.2 What a program cannot write today

FNV-1a (§2.1) cannot be written portably. On Go it can be written with target-native names:

```
use go
fn step(h ∈ 0..2^64-1, b ∈ 0..255) (0..2^64-1) { go.u64*(go.`^`(h, b), 1099511628211) }
```

That program is Go's alone. The analysis learns only that the result is in U₆₄, and on any other target
it does not compile. CRC-32 (§2.3) can be written with `go.&`, `go.^`, `go.>>`, again Go's alone, though
every operation in it means the same thing on every host for these operands.

### 3.3 Why the obvious design does not fit: a set is a claim

Go, Rust, Java and C put wrapping in the **type**: `uint64` arithmetic wraps, `int` arithmetic does
something else. Under ADR 0079, **a declared set is a claim, and it never changes what an operation
means**: `h ∈ 0..2^64-1` says the value lies in U₆₄, and `h * p` is still ℤ's product. Making `*` wrap
because of a declared set would make the claim choose the operation. That is the "representation by
declaration" ADR 0079 removed. So **the wrap must be in the term**: an operation, not a set. That is the
first reason this document arrives at π_n.

### 3.4 What changed since bitwise.md (2026-08-19)

bitwise.md recommended a module `bits` windowed to int32 (its B), `bits64` later (C), and rejected
full-width with JavaScript emulating (D) and fixed-width types (E). Its premises have moved:

| bitwise.md's premise | today |
|---|---|
| "we deliberately have [no type system]" (rejecting E) | types are sets, claims are checked (ADR 0049, 0072, 0079) |
| the int32 window "is an obligation nothing checks" | every operation is proven inside its set or refused (ADR 0019); a window is a range the IR proves |
| JS emulation is hidden cost (rejecting D) | ADR 0079: the facts choose the representation, and a value above JavaScript's word is already held above it; the cost is computed, per program, per target |
| "no integer array" (§6) | `Array(V)` and buffers exist (ADR 0052) |
| `shl` "truncated to int32", `ushr` width-bound | left shift is ·2ᵏ, exact; a width enters only through π_n |

Its measurements stand, and its central observation, that and/or/xor/not are 2-adic and width-free, is
§1.5's theorem. What changes is that the window is no longer a convention: it is a fact the IR proves,
and its edge chooses a representation instead of a portability claim.

---

## 4. LLVM

LLVM's `iN` **is** the bit-vector of §1.5, for any N from 1 to 2²³. It carries no signedness: a value is a
class, and each instruction says which residue it reads.

| LLVM | its meaning | descends (§1.3)? |
|---|---|---|
| `add`, `sub`, `mul` | ℤ/2ᴺ's ring operations | yes, one instruction |
| `and`, `or`, `xor` | 𝔽₂ᴺ's | yes, one instruction |
| `shl a, k` | π_N(a·2ᵏ), for k < N | yes |
| `sdiv`, `srem` / `udiv`, `urem` | ℤ's `/`, `%` on σ_s / σ_u | no, two instructions |
| `ashr` / `lshr` | ⌊a/2ᵏ⌋ on σ_s / σ_u | no, two |
| `icmp slt` / `icmp ult` (and the rest) | ℤ's order on σ_s / σ_u | no, two |
| `trunc iM to iN` | π_N, between widths | |
| `sext` / `zext` | σ_s / σ_u, embedded in the wider word | |
| `llvm.ctpop`, `ctlz`, `cttz` | popcount, leading zeros, ν₂ (§1.6) | |
| `llvm.fshl(a, a, k)` | rotate left | |
| `llvm.bswap`, `llvm.bitreverse` | §1.6's permutations | |
| `llvm.clmul` | carry-less multiply, 𝔽₂[x] (in the installed 23.1.4) | |
| `llvm.umul.with.overflow` | the product's residue and whether it left U_N | |

**The flags are the bridge to ℤ.** `add nuw` says "this class operation equals ℤ's + on the unsigned
residues": the poison condition is exactly the case where it would not. `nsw` says the same for signed
residues. So an operation the IR proves inside a word prints with its flag (ADR 0080 item 2), and an
operation under π_n prints **without** one, because it is meant to wrap. FNV-1a's step from §2.1, as the
printer would write it:

```llvm
  %b64 = zext i8 %b to i64          ; data[i] ∈ 0..255, its unsigned residue
  %x   = xor i64 %h, %b64
  %h2  = mul i64 %x, 1099511628211  ; under mod 2^64: no nuw, no nsw, the ring's product
```

`mod 2^64` itself prints as nothing, because on `i64` read unsigned, π₆₄ is the identity. Without fusion
the same source prints `mul nuw`, which is poison when the product leaves U₆₄. That is exactly the case
the program says it wants.

**What LLVM adds beyond 64.** `i128` is native (one `mul` gives the 128-bit product, which `math/bits.Mul64`
returns as a pair), and so is any width (`i24` for a packed RGB). That belongs to the widths wall, the next
step. The bit operations here are width-generic, so nothing in this document fixes n to 8, 16, 32 or 64.

---

## 5. The other targets

### 5.1 Go

| | |
|---|---|
| ℤ/2ⁿ | `uint8` … `uint64`, `int8` … `int64`: all wrap. The specification's "Integer overflow" section computes unsigned operations modulo 2ⁿ and defines signed overflow by the two's-complement representation |
| π_n | the conversion `uint32(x)`; π_nˢ is `int32(x)` |
| 𝔽₂ⁿ | `&`, `\|`, `^`, `&^`, unary `^` (n-bit complement), `<<`, `>>` (`lshr` on unsigned, `ashr` on signed) |
| width functions | `math/bits`: `OnesCount`, `LeadingZeros`, `TrailingZeros`, `RotateLeft`, `Reverse`, `ReverseBytes`, `Mul64`, `Add64` |
| shift count ≥ width | yields 0 (Go's choice); `1 << 64` is 0 (bitwise-2026-08-19) |

FNV-1a 64 is the loop of §2.1 with `hash ^= uint64(c); hash *= prime64`, two instructions per byte.

### 5.2 Java and Android's ART

| | |
|---|---|
| ℤ/2ⁿ | `int` (n = 32) and `long` (n = 64) wrap (JLS §15.17.1: an overflowing product is "the low-order bits of the mathematical product") |
| unsigned residues | no unsigned types; `long` holds σ_u, read by `Long.compareUnsigned`, `divideUnsigned`, `remainderUnsigned`, `toUnsignedString`; `Integer.toUnsignedLong` is σ_u for n = 32 |
| π_n | `(int) x` is π₃₂ˢ; `x & 0xFFFFFFFFL` is π₃₂; `(byte)`, `(short)`, `(char)` are π₈ˢ, π₁₆ˢ, π₁₆ |
| 𝔽₂ⁿ | `&`, `\|`, `^`, `~`, `<<`, `>>`, `>>>` |
| width functions | `Long.bitCount`, `numberOfLeadingZeros`, `numberOfTrailingZeros`, `rotateLeft`, `reverse`, `reverseBytes`; `Math.multiplyHigh` and `Math.unsignedMultiplyHigh` (JDK 18) |
| shift count | masked by 63 (`long`) or 31 (`int`) |

FNV-1a 64: `h ^= (b & 0xff); h *= 0x100000001b3L;`, the same two instructions as Go. Java's `byte` is
signed, so a byte read from a `byte[]` is π₈ˢ and needs `& 0xff` to become σ_u. That is §1.2's residue
choice showing up in an API.

### 5.3 JavaScript (the browser)

| | |
|---|---|
| ℤ/2³² | native, through the int32 operators: `(a + b) >>> 0` is π₃₂ of a sum below 2⁵³, `a \| 0` is π₃₂ˢ, `Math.imul(a, b)` is π₃₂ˢ of the product (a plain `*` loses bits above 2⁵³) |
| 𝔽₂³² | `&`, `\|`, `^`, `~`, `<<`, `>>`, `>>>`, all through `ToInt32`/`ToUint32`, which is π₃₂ˢ/π₃₂ of the operand |
| ℤ/2⁶⁴ | **no native form**: a split pair (two Numbers, the high and low 32 bits), measured 4.8× faster than `BigInt` (u64repr-2026-09-22); or `BigInt.asUintN(64, …)`, which ties the pair on multiply-add and loses badly on division |
| 𝔽₂ⁿ beyond 32 bits on naturals below 2⁵³ | a split: `hi = Math.floor(x / 2**32)`, `lo = x >>> 0`, operate on each, recombine |
| width functions | `Math.clz32` only; popcount and the rest are a few lines each (Hacker's Delight §5) |
| shift count | masked by 31 |

FNV-1a 32 is native: `h = Math.imul(h ^ b, 16777619) >>> 0`. FNV-1a 64 needs the pair. Since the prime
is 2⁴⁰ + 435, h·p mod 2⁶⁴ is h·435 + h·2⁴⁰, and the step is about eight operations instead of two:

```js
let x = (lo ^ b) >>> 0;                           // the xor touches only the low half; >>> 0 reads it unsigned
let t = x * 435;                                  // < 2^41, exact in a double
lo = t >>> 0;                                     // π32 of t
hi = (hi * 435 + Math.floor(t / 2**32) + (x << 8)) >>> 0;   // x·2^40 adds x·2^8 to the high half
```

This is the cost bitwise.md refused to hide. (Checked against exact arithmetic on 20,000 random inputs;
without the `>>> 0`, `lo ^ b` is a signed int32 and the carry is wrong, which the check caught in
this document's first draft.) Under ADR 0079 it is a **representation chosen by the facts
and reported**: a program whose values fit 2³² pays nothing on JavaScript; a program that says
`0..2^64-1` pays the pair, and `cmd/portable` can say so.

### 5.4 Windows, through LLVM

Native for every n (§4): one instruction for each operation of §1.3, and one for most of §1.6's
(`popcnt`, `lzcnt`, `tzcnt`, `rol`, `bswap`, `pclmulqdq`), given the CPU features. `-mpopcnt` and its
kin are target attributes, a question for the target file, not for this document.

### 5.5 Agreement, in one table

| | Go | Java / ART | JavaScript | LLVM |
|---|---|---|---|---|
| and, or, xor, not on S₃₂ | native | native | native | native |
| the same on S₆₄ or U₆₄ | native | native | split | native |
| π₃₂, ℤ/2³² arithmetic | native | native | native (`imul`, `>>> 0`) | native |
| π₆₄, ℤ/2⁶⁴ arithmetic | native | native (`long`) | split pair, ~10× | native |
| ⌊x/2ᵏ⌋ on naturals | native | native | native below 2³², split above | native |
| popcount, clz, ctz, rotate | `math/bits` | `Long.*` | `Math.clz32`, else defined | intrinsics |

**Every cell computes the same function.** The table differs in cost, never in answer. That is the
difference from bitwise-2026-08-19's table, where the hosts disagreed: they disagreed because the
operation was named by its host spelling (`js.|` is π₃₂ˢ of or, `go.|` is or). Named by its meaning,
or on ℤ, every host agrees, and some pay more.

---

## 6. The type system

### 6.1 Where ℤ/2ⁿ could live

The language's values are partitioned into sorts: ℤ, F64, 𝔹, Σ*, host types, products, variants, tables
(settypes-research.md §2.3). A type is a set of values. There are two places ℤ/2ⁿ could go:

- **(a) Inside ℤ, as the set U_n of canonical residues**, with π_n : ℤ → U_n an ordinary function. No
  new values: FNV-1a 64 of "a" is the integer 12638187200555641996. Wrapping is π_n written in the term (§3.3),
  and the Boolean operations are ℤ's.
- **(b) As a new sort Bits(n)**, whose values are classes (or bit strings) distinct from every integer.
  `[255]` in Bits(8) is not the integer 255; `+` on Bits(n) is ℤ/2ⁿ's; a literal needs a sort from its
  context; conversions `bits(n, x)`, `u(v)` and `s(v)` cross between them. This is Cryptol's model
  (`[64]`), SMT-LIB's `(_ BitVec 64)`, and Rust's `Wrapping<u64>`.

Both are sound. They differ in fit, in cost, and in what the analysis can say:

| | (a) U_n ⊆ ℤ with π_n | (b) the sort Bits(n) |
|---|---|---|
| new values | none | 2ⁿ per n, disjoint from ℤ |
| new operations | π_n; and, or, xor, not on ℤ | +, −, ·, and, or, xor, not, shifts, comparisons, division twice (s and u), per sort |
| a literal `255` | an integer, as now | ambiguous until its sort is known |
| `h ∈ 0..2^64-1` | a claim, as every set is | not the type; the type is Bits(64) |
| the residue choice (§1.2) | the program wrote π_n or π_nˢ, so it is in the term | `u(v)` or `s(v)` at every comparison and division |
| facts | §6.3's, about integers | a theory of bit-vectors, or none |
| fit (fit-research.md) | the Boolean operations ℤ already has as 2-adic integers, π_n, one theorem | a second copy of arithmetic, a misfit per duplicated operator |
| what (b) has that (a) lacks | a type error when an unreduced value meets a wrapped one | |

The last row is (b)'s real advantage, and §7 weighs it.

### 6.2 What π_n needs from the checker

- π_n(x) ∈ U_n for every x: a fact, from the definition.
- π_n is **Presburger-definable**: r = π_n(x) ⇔ ∃q. x = 2ⁿ·q + r ∧ 0 ≤ r < 2ⁿ. The refinement layer's
  linear fragment and the Omega test (`presburger/`) take it as it stands. The coefficient 2⁶⁴ exceeds
  int64, the wall mathbits-2026-09-23 recorded for the double-word laws; the Omega test is over ℤ, the
  linear layer is not.
- On x already in U_n, π_n(x) = x, so a `mod 2^64` on a value proven in U₆₄ costs nothing and says
  nothing new.

### 6.3 Facts about the Boolean operations

The IR's analysis is intervals (ir.Decide). What it needs, all exact on naturals:

| fact | for x, y ≥ 0 | use |
|---|---|---|
| closure | x, y < 2ⁿ ⇒ x & y, x \| y, x ⊕ y < 2ⁿ | every example of §2 stays in its word |
| and | 0 ≤ x & y ≤ min(x, y) | `crc & 255 ∈ 0..255`, a table index proven in bounds |
| or | max(x, y) ≤ x \| y ≤ x + y | RGB's `r \| (g << 8)` ≤ 2¹⁶ − 1 |
| xor | 0 ≤ x ⊕ y ≤ x \| y | |
| shifts | ⌊x/2ᵏ⌋ ≤ x/2ᵏ; x·2ᵏ exact | |
| on negatives | x & y ≥ min(x, y) when both < 0, … (sign tables, Warren §4) | rarely needed |

These are a few lines each in the interval domain (Warren, *Hacker's Delight*, §4-3, gives the tight
bounds for and and or of two intervals; Miné's work on abstract domains for machine integers is the
general treatment). Beyond intervals:

- **Popcount is cardinality**, so facts like |x | y| + |x & y| = |x| + |y| are BAPA formulas
  (settypes-research.md §5: QF BAPA, decidable, NP-complete), with naturals as finite sets.
- **The whole structure is decidable.** (ℕ, +, and, or, xor, ⌊·/2⌋, constants) is a 2-automatic
  structure, definable in Büchi arithmetic (ℕ, +, V₂). Its first-order theory is decided by automata
  reading the binary digits in parallel (Büchi 1960; Bruyère, Hansel, Michaux and Villemaire,
  "Logic and p-recognizable sets of integers", 1994). The cost is non-elementary in the worst case. For a
  fixed n, QF_BV is NP-complete, decided by bit-blasting to SAT. Neither is needed by any example of §2,
  and xorshift's never-zero claim (§2.6) needs linear algebra over 𝔽₂, which is a lemma, not a
  procedure.

So the decidability map gains one row, and the procedures the language owns cover what §2 needs: intervals
now, Presburger for π_n already, BAPA and automata only when a program names a refusal (the plan's item
10).

### 6.4 Representation

ρ is unchanged in kind. A value proven in U₃₂ is held in the narrowest word that holds it (32 bits
where widths are chosen, the next decision; the word today). A value proven in U₆₄ is held as ADR 0033
already holds it: `uint64` on Go, the residue in a `long` on the JVM, the split pair on V8 (to be built),
`i64` on LLVM. **Fusion is a property of a term, not of a representation**: it licenses computing a
subterm under π_n with wrapping n-bit operations, and the subterm's intermediates are **exempt from the
word obligation**, because they are observed only through π_n. That exemption is the one new soundness
rule, and it gets planted faults (§8, B3): a `/`, `%`, `>>`, comparison or popcount under π_n must stop
fusion, and each is a wrong answer if it does not (§1.5's 150 against 22).

### 6.5 Portability

Every operation here is Tier 1 by meaning: it is a function on ℤ, and every target computes it. What
differs is cost: native, split pair, or defined in the language from cheaper operations. That cost is
reported, as portability is (`cmd/portable`), never hidden and never a different answer. **This reverses
integers.md §0a's "Bitwise and shifts are NOT promoted"**, by a derivation: the disagreement it recorded
was between host spellings of different functions, not between hosts computing one function.

---

## 7. The candidates

Each written as FNV-1a's step, and judged on fit, portability, what the analysis can know, and cost.

### A. Host names only (today)

```
fn step(h ∈ 0..2^64-1, b ∈ 0..255) (0..2^64-1) { go.u64*(go.`^`(h, b), 1099511628211) }
```

Go's alone; the analysis knows the result's set and nothing else; CRC-32, which means the same on every
host, is locked to one. **Fit**: none; it is the absence of a design. Kept only as the target-native
escape every host already has.

### B. The Boolean operations on ℤ, and π_n (recommended)

```
fn step(h ∈ 0..2^64-1, b ∈ 0..255) (0..2^64-1) { ((h ⊕ b) * 1099511628211) mod 2^64 }
```

- **Meaning**: ℤ's, exactly, and the same on every target.
- **New in the language**: and, or, xor and not on ℤ; the shifts as ·2ᵏ and ⌊·/2ᵏ⌋; the Euclidean
  remainder `mod`. π_n is `mod 2ⁿ`; π_nˢ is defined (B4).
- **New in the compiler**: fusion on the IR (one rule, with planted faults), the interval facts of
  §6.3, each target's declarations, and the split pair on JavaScript (measured, not built).
- **What it cannot catch**: forgetting a `mod`. `h * p` without it is ℤ's product, and the program is
  refused because the product is not proven inside the word. Writing a `mod` where none was meant is
  not caught at all: it is what the program said.

### C. A sort Bits(n)

```
fn step(h ∈ Bits(64), b ∈ Bits(8)) (Bits(64)) { (h ⊕ zext(64, b)) * bits(64, 1099511628211) }
```

- **Meaning**: ℤ/2ⁿ's, by sort. `+` means something else on this sort.
- **New**: a sort per n, its literals, two of every non-descending operation (`<ᵤ`, `<ₛ`, `/ᵤ`, `/ₛ`, …),
  `zext`, `sext`, `trunc`, and the residue maps back to ℤ. This is LLVM's IR lifted into the language.
- **For**: an unreduced integer cannot be mixed with a residue by accident, and a type error says so
  (Cryptol's argument). It is also the shape SMT solvers decide.
- **Against**: it duplicates arithmetic. Every operation of §1.3's "no" column appears twice, and every
  law the analysis knows about ℤ must be restated for each sort. The settypes algebra gains a family of
  sorts disjoint from ℤ, while the values they hold are already in ℤ as U_n. The type error C buys is the
  error B's refusal already reports: an unreduced product above the word is refused, not silently wrong.
  And JavaScript pays the split pair under C as under B: the sort changes nothing a target does.

### D. Wrapping by declared set (Go's and Rust's model)

```
fn step(h ∈ U64, b ∈ 0..255) (U64) { (h ⊕ b) * 1099511628211 }    // `*` wraps because h ∈ U64
```

**Excluded by ADR 0079**: a declared set is a claim and does not change what `*` means (§3.3). Listed
because it is what every reader of Go expects, and the "Why not" must say so.

### E. Wrapping operators (Zig's `+%`, Swift's `&+`)

```
fn step(h ∈ 0..2^64-1, b ∈ 0..255) (0..2^64-1) { (h ⊕ b) *%64 1099511628211 }
```

B with sugar: `a *%64 b` is π₆₄(a·b), so the width must be in the operator. Fusion makes the sugar
unnecessary, since `mod 2^64` once at the end of a polynomial already costs nothing, and it multiplies
operators by widths, the suffix explosion bitwise.md warned of. A surface question, B6, if anything.

### Summary

| | meaning target-independent | new sorts | duplicated operations | the analysis knows | JS cost of FNV-64 |
|---|---|---|---|---|---|
| A host names | no | 0 | 0 | the result's set | does not compile |
| **B π_n on ℤ** | **yes** | **0** | **0** | **ℤ's laws, π_n's definition, §6.3** | split pair |
| C Bits(n) | yes | one per n | every non-descending one, twice | a BV theory, to be built | split pair |
| D by declared set | — | — | — | — | excluded by ADR 0079 |
| E wrapping operators | yes | 0 | one per width | as B | split pair |

---

## 8. Decisions for hamza

Six, in dependency order. Each states a recommendation and its reason.

**B1. Promote and, or, xor and not on ℤ, and the shifts as ·2ᵏ and ⌊·/2ᵏ⌋, to the language.**
*Recommended: yes*, as Tier 1 with the 2-adic meaning (§1.5). Each target realizes them from the facts:
the host operator where the operands are proven in a window the host handles natively (S₃₂ on
JavaScript, S₆₄ or U₆₄ elsewhere), a split above it. This reverses integers.md §0a and bitwise.md's
int32 window (§6.5), and costs every target file a declaration and JavaScript a lowering. Shift counts
carry `where 0 ≤ k`; there is no upper bound, because ·2ᵏ and ⌊·/2ᵏ⌋ are exact on ℤ, and a count beyond
the word is an ordinary overflow obligation.

**B2. ℤ/2ⁿ is π_n on ℤ (candidate B), not a sort Bits(n) (C).**
*Recommended: B.* It adds no values and no sort, keeps every law the analysis has, and fits ADR 0079 (the
wrap is in the term, the set stays a claim). C's one advantage, a type error for an unreduced value,
is already a refusal under bounded-by-default. C stays the "Why not", reopened if programs keep omitting
the `mod` and reading the refusal is worse than a type error.

**B3. Fusion: a term under π_n built from +, −, ·, and, or, xor, not and left shift computes in n-bit
wrapping arithmetic, its intermediates exempt from the word obligation.**
*Recommended: yes*, built on the IR with the theorem of §1.5 as its justification. Planted faults:
`/`, `%`, `>>`, a comparison, popcount and a host call under π_n must each stop fusion, each shown to
give a wrong answer if it does not (150 against 22). It is the single new soundness rule.

**B4. The signed residue π_nˢ gets a name.**
*Recommended*: defined in a library from π_n, π_nˢ(x) = π_n(x + 2ⁿ⁻¹) − 2ⁿ⁻¹, with targets recognizing
it as a sign extension (Go's `int16(x)`, Java's `(short)`, LLVM's `trunc`+`sext`). Needed by Win32's
`GET_X_LPARAM` (§2.8) and by every host API that hands back a signed narrow value.

**B5. The width functions of §1.6 are definitions in a library module, which targets may provide.**
*Recommended*: `lib/bits` defines popcount, bit length, leading and trailing zeros, rotations, byte swap,
bit reversal and carry-less multiplication in the language, each with its algebra (math/bits' comment is
the model). Each target overrides with its own native (Go's `math/bits`, Java's `Long.*`, LLVM's
intrinsics, `Math.clz32`), as the capability graph's layers do. A target without a native pays the
definition, and that is reported. Popcount is a function on ℕ only (§1.3), so its parameter is `Nat`.

**B6. The surface.** Three questions, each a matter of fit as much as algebra:
- *xor*: `^` is J's power (`2^64`) and must stay so. Candidates: `⊕` (the field's addition, a symbol J
  would add), `xor` (a word), or a function `xor(a, b)`.
- *and, or, not*: `&`, `|`, `~` (C's, read by every programmer; `&&` and `||` stay the connectives), or
  `∧`, `∨`, `¬` (TLA+'s logical symbols, confusable with the logic's connectives, which J writes `&&`
  and `||`).
- *the remainder*: `mod` as a new operator, Euclidean (always in [0, m) for m > 0), beside `%`
  (truncating, unchanged); and **hexadecimal literals** (`0xEDB88320`), which every example of §2
  wants.
*Recommended*: `&`, `|`, `~`, `<<`, `>>`, `⊕`, `mod`, and `0x` literals. An ADR records the spelling.

**What follows, if B1–B5 are taken**: an ADR, then a program first, as the standing goal asks. That is
`hash/fnv` (FNV-1a 32 and 64 over a file) and `hash/crc32`, at parity with Go's own on Go, Java,
JavaScript and LLVM. The JavaScript split pair is built for it, measured against u64repr-2026-09-22's
hand-written pair.

## 9. Not here

- **Widths** (i8 … i128, the representation of U_n as a narrower word): the next wall, the next step.
- **A float's bits** (`Float64bits`), a bijection between F64 and U₆₄: the floats wall.
- **GF(2ⁿ) arithmetic** (AES, GCM): `clmul` plus a reduction, defined in `lib/bits` under B5, and
  nothing more until a program asks.
- **Proving xorshift-style claims** (§2.6): lemmas over 𝔽₂-linear maps, above the decidability line.
- **Overflow-reporting operations** (`umul.with.overflow`, Go's `bits.Mul64`): already stateable as a
  variant or a pair (llvmsurvey §3); the double word as a product is the widths step's.
