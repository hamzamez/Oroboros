# 0082 — Wrapping is the residue map on ℤ, and the bitwise operations are ℤ's own

Date: 2026-10-27
Status: Accepted (hamza, 2026-10-27: "I agree to the recommendations. xor is the field' addition with
asci spelling \xor (push back if there is a reason to)", then "go ahead with \oplus"), on
[bits-research.md](../bits-research.md) §8's decisions B1–B6. The first of the seven walls of
[llvmsurvey-2026-10-27](../../gauntlet/results/llvmsurvey-2026-10-27.md) §4. Reverses
[integers.md](../spec/integers.md) §0a's "Bitwise and shifts are NOT promoted"; supersedes
[spec/bitwise.md](../spec/bitwise.md)'s candidates. **Not built**: the build comes after the widths
decision (the plan's 6b, step 2), since the two meet in how a value of U_n is held.

## Context

ℤ/2ⁿ is the arithmetic of an n-bit word and 𝔽₂ⁿ its bits under xor and and, and a machine word, LLVM's
`iN`, carries both. Hashes, checksums, random generators, bit sets, Win32's flags and every host API
that hands back a narrow signed value need them. The language had none of it: the bitwise operations
were host names with no portability claim, because JavaScript's operators coerce to int32
(bitwise-2026-08-19), and wrapping existed only as Go's target-native `go.u64*`. targets/go/u64.oro
recorded that "the language's ℤ/2^n is not built (the wall hash/fnv will hit)".

bits-research.md derived the design from two facts:

1. **and, or, xor and not are total on ℤ**, read as 2-adic integers (Vuillemin 1994): each acts position
   by position on an eventually constant bit string. ⌊x/2ᵏ⌋ and x·2ᵏ are ℤ's. None needs a width.
2. **π_n(x) = x mod 2ⁿ is at once a ring homomorphism ℤ → ℤ/2ⁿ and a Boolean homomorphism ℤ → 𝔽₂ⁿ.**
   So, by induction on a term (the *fusion* corollary), a term built from +, −, ·, and, or, xor, not and
   left shift, read through π_n, is computed by the same term in n-bit wrapping arithmetic. Division,
   remainder, comparison, right shift and popcount do not descend to ℤ/2ⁿ, which is why LLVM has one
   `add` and two `div`s. Under π₈, (200 + 100)/2 is 150, and computed wrapping it is 22.

## Decision

1. **The Boolean operations are the language's** (B1): `a & b`, `a | b`, `a ⊕ b`, `~a` on ℤ with the
   2-adic meaning (`~a = −a − 1`), and the shifts `a << k = a·2ᵏ` and `a >> k = ⌊a/2ᵏ⌋`, with `where
   k ≥ 0` and no upper bound. They are Tier 1: a function on ℤ, the same on every target, with a
   conformance case each. Each target realizes them from the operands' facts: the host operator where
   the host computes the function natively (S₃₂ on JavaScript; S₆₄ or U₆₄ on Go, the JVM and LLVM), a
   split above it. **The cost differs by target; the answer never does**, and the cost is reported.
2. **ℤ/2ⁿ is π_n on ℤ, not a sort** (B2): `x mod m` is the Euclidean remainder, in [0, m) for m > 0 (a
   new operator beside `%`, which still truncates), and π_n is `x mod 2^n`. The residues U_n = [0, 2ⁿ)
   are integers. A declared set stays a claim (ADR 0079): `h ∈ 0..2^64-1` never makes `*` wrap. The wrap
   is written in the term.
3. **Fusion is the one new soundness rule** (B3): on the IR, a subterm under `mod 2^n` built only from
   `+ - * & | ⊕ ~ <<` and constants is computed in n-bit wrapping arithmetic, and its intermediates are
   **exempt from the word obligation** (ADR 0019), because they are observed only through π_n. Any
   other operation stops fusion, and its operands must be proven as everywhere else. Planted faults:
   `/`, `%`, `mod` by a non-power of two, `>>`, a comparison, popcount and a host call under π_n each
   stop it; letting any one through gives a wrong answer, which its test shows. The rule is checked
   against the concrete interpreter over ℤ (`ir/interp_test.go`).
4. **The signed residue is a library definition** (B4): `smod(x, n) = (x + 2^(n-1)) mod 2^n -
   2^(n-1)` in `lib/bits`, π_nˢ, which a target recognizes as a sign extension (Go's `int16(x)`, Java's
   `(short)`, LLVM's `trunc` then `sext`).
5. **The width functions are definitions, and a target may provide them** (B5): `lib/bits` defines
   popcount (on `Nat`), bit length, leading and trailing zeros, rotations, byte swap, bit reversal and
   carry-less multiplication in the language, each with its algebra, and each target's layer overrides
   them with its native (Go's `math/bits`, Java's `Long.*`, LLVM's intrinsics, `Math.clz32`). A target
   without a native pays the definition, and that is reported.
6. **The spelling** (B6): `&`, `|`, `~`, `<<`, `>>`, and **`⊕` with the ASCII spelling `\oplus`**,
   TLA+'s and LaTeX's (J's rule, ADR 0065: one ASCII spelling per symbol, TLA+'s); `mod` a word in
   operator position only (j.md §1.4's reservation); hexadecimal literals `0x…`. **Precedence is Go's**:
   `<< >> &` bind with `* / % mod`, `| ⊕` with `+ -`, all above the comparisons, so `x & 1 == 0` is
   `(x & 1) == 0`; `~` is a prefix operator like unary `-`.
7. **What is not built until a program asks**: the decision procedures beyond intervals for these
   operations (BAPA for popcount as cardinality; Büchi arithmetic, decidable by automata), and lemmas
   over 𝔽₂-linear maps (xorshift's never-zero). The interval facts for the Boolean operations
   (bits-research.md §6.3: closure below 2ⁿ, and ≤ min, or ≤ sum, xor ≤ or) are built with item 1.

## Why not

- **A sort Bits(n)** (Cryptol's `[n]`, SMT-LIB's bit-vectors, Rust's `Wrapping<u64>`): it adds 2ⁿ values
  per n disjoint from ℤ though U_n already holds them, duplicates every operation that does not descend
  (a signed and an unsigned `<`, `/`, `%`, `>>`), and restates every law the analysis knows. Its one
  advantage, a type error when an unreduced value meets a residue, is already a refusal under bounded by
  default: an unreduced product above the word is not proven and is refused. Reopened if programs keep
  omitting `mod` and the refusal reads worse than a type error would.
- **Wrapping chosen by a declared set** (Go's, Rust's, Java's and C's types): a set is a claim and never
  changes what an operation means (ADR 0079).
- **Wrapping operators** (Zig's `+%`, Swift's `&+`): the width must then be in the operator, one per width,
  and fusion makes them unnecessary, since one `mod 2^64` over a polynomial costs nothing.
- **bitwise.md's int32 window (its B) and `bits64` family (its C)**: a window was a convention nothing
  checked; now every operation is proven inside its set (ADR 0019), so the window is a fact that chooses
  a lowering, not a portability limit. Two families were two spellings of one function.
- **Host names only** (today): CRC-32, which means the same on every host, was locked to one.
- **`\xor`** as the ASCII spelling: it names the operation, but it would be J's first ASCII spelling that
  is not TLA+'s, and the rule would become a list. The file stores `⊕`.
- **`^` for xor**: `^` is J's power, `2^64`, which every range uses.
- **C's precedence**, with `&` below `==`: a known source of bugs, which Go corrected.

## Consequences

- **Built with the widths decision** (the plan's 6b, step 2), program first: `hash/fnv` (FNV-1a, 32 and
  64 bits over a file) and `hash/crc32`, at parity with Go's own on Go, Java and JavaScript, and on
  Windows once `ir/llvm` exists. Built for them:
  - the reader's tokens, `\oplus` and hexadecimal literals;
  - D's operators and Σ_L's names;
  - each target's declarations;
  - fusion on the IR, with its faults;
  - the interval facts;
  - `lib/bits`;
  - JavaScript's split pair for U₆₄, measured against u64repr-2026-09-22's hand-written pair.
- **Spec first**: integers.md gains the operations' meaning and each target's realization;
  primitives.md classifies them Tier 1 with conformance cases; j.md's lexical table gains `⊕`/`\oplus`.
- **The target-native names stay** (`go.&`, `java.>>>`, `js.|`, `go.u64*`): a host's own operator with
  the host's own meaning, as every target-native name is. A program that writes `js.|` on a 40-bit
  value still gets JavaScript's answer, which is what it named.
