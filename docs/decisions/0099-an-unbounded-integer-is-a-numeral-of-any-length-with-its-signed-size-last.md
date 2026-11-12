# 0099 — An unbounded integer is a numeral of any length, its signed size last

Date: 2026-11-09
Status: Accepted (hamza, 2026-11-09: R3 of [integers-research.md](../integers-research.md), agreed
with R1–R4). Built in steps: [signfuse-2026-11-09](../../gauntlet/results/signfuse-2026-11-09.md)
(R3a), [natnum-2026-11-10](../../gauntlet/results/natnum-2026-11-10.md) (R3b),
[natdiv-2026-11-10](../../gauntlet/results/natdiv-2026-11-10.md) (R3c, D, the move, M3). **Item 3 reversed by [ADR 0104](0104-k-n-star-is-in-base-2-63-the-largest-base-the-decision-proves.md)**: B = 2⁶³, the double word's.

## Context

On a target with no bignum, windows today, an integer whose set has no upper bound has no
representation. Every carrier there is finite, and no finite carrier holds an infinite set
(integers-research Proposition 1). So `big-fib`, fib(n) for an unbounded n, compiles there only in
the word and traps at fib(93). ℤ is the language's integer sort, and a target may not decline it:
the LLVM layer owes a representation (H, d) with d(H) = ℤ (ADR 0083). The image of d is infinite, so
H holds strings of every length, and **the representation is variable length**.

## Decision

1. **ℕ\* is the positional numerals in base B = 2³²**, and **ℤ is K(ℕ\*)**, the group completion over
   them (ADR 0098's construction):
   - each m ∈ ℕ has one numeral with a nonzero top digit (positional notation, Knuth TAOCP 2 §4.1);
   - each integer has one normal form, a sign and a magnitude, with one zero.
2. **A value is one table t**:
   - the digits are t[0], …, t[len t − 2], with digit i at index i;
   - **the last entry is the signed size** h = ±m, as GMP's `_mp_size`, where m = 1 + the index of the
     top nonzero digit, or 0;
   - every slot from m to len t − 2 holds 0;
   - d(t) = sgn(h) · Σ_{i < len t − 1} t[i]·Bⁱ;
   - the normal form is h's: zero is h = 0, so it has no sign to choose.

   len t is the capacity, a bound the operation computes (a sum's is max(m_a, m_b) + 2). The
   header is last for the same reason as Z(V_n)'s sign entry: each operation's build writes
   [0, len t) at its counter, so the table is born full (ADR 0097, signfuse-2026-11-09).
3. **B = 2³² over U₆₄.** Algorithm M's inner step is uᵢ·vⱼ + w + k ≤ (B − 1)² + 2(B − 1) = B² − 1,
   exact in [0, 2⁶⁴), which windows holds as `i64` read unsigned (llvm.md §3a). The decision proves it
   on the library's own body, as it does for the fixed limbs. A digit is t & (B − 1), which is both the
   digit's definition (t mod B for t ≥ 0, ADR 0082) and the widening threshold the carry's bound
   needs.
4. **The operations are Knuth's**, on normal forms (TAOCP 2 §4.3.1):
   - A and S, as one recurrence over x + σ·y (signfuse-2026-11-09), with the group completion's case
     on the signs choosing σ and the operand order as data;
   - M, schoolbook;
   - a product, quotient and remainder by a word in one pass;
   - comparison by sign, then size, then digits from the top;
   - render by 10⁹, nine decimal digits per division: r·B + digit < 10⁹·2³² < 2⁶².

   - D, division by a numeral: long division is a Mealy machine whose state is the partial
     remainder, its output computed from q̂ with q̂ − 2 ≤ q ≤ q̂ (Theorem B) and D3's test, at most
     two add-backs a step; a one-digit divisor is the same machine.

   Every operation is checked once as a theory (ADR 0035), in `emit/natnum.oro`.
5. **Which rung.** On a target with no bignum, a program whose one set is bounded keeps the fixed
   limbs, V_n or Z(V_n); measured, V₉ is 4.4× faster than K(ℕ\*) on the same additions
   (natdiv-2026-11-10). A program whose set has no bound, on either side, takes K(ℕ\*). **A bounded
   program using an operation V_n lacks** (a quotient by a big value) **moves to K(ℕ\*) whole**, as
   on a host it moves to the host's bignum: V_n's carrier lies inside K(ℕ\*)'s and both are
   injective into ℤ, so no answer changes, and its one set is enforced by sign and bit length,
   ADR 0029's two observables, read off the header and the top digit.
6. **Nothing is enforced at run time** beyond item 7. ℤ has no bound. A finite end of an unbounded set, such as
   `Nat`'s 0, is inside the word, and ADR 0087 decides it at the definition, statically.
7. **No machine represents ℤ**, and the library says where it stops: a numeral is held while its
   table fits, len t ≤ max_len, the target's fact (2⁴⁷ − 9 on x64), passed to every definition as its
   first argument. A capacity past it is a crash, `abandon`, as a host bignum's allocation failure
   is, never a numeral cut short. Written as the guard of a case, so the bound is a path fact on the
   arm that continues (ADR 0090's absorption).
8. **Rendering demands an exact integer on every target**: render : ℤ → Σ\* is ℤ's, so `big-str`'s
   argument is demanded whatever a target declares its parameter to be. Only the widening of a word
   value is the target's choice, and on a target with no bignum the word renders itself (ADR 0095).
   This is what puts `fib`'s accumulators above the word on windows.

## Why not

- **Two header entries, a sign and a size**: one entry carries both, since h's sign is the sign, and
  h = 0 is the one zero, so the normal form needs no rule.
- **The header first**, as GMP lays out `mpz_t`: then the build writes [1, len) from its counter and
  entry 0 last, and the table is not born full. Its zero fill is a cost every operation pays.
- **An exact-length result**, a second pass or a copy once the length is known: every bignum library
  keeps a used size within a capacity, since the copy is a second O(m) pass and an allocation.
- **B = 2⁶⁴ with i128 products**: half the digits, a quarter of the digit products. It is ADR 0083's
  "native iN above the word", measured against B = 2³² when built (M4), not assumed.
- **B = 2²⁴, the fixed limbs' base**: 0.56 of the bits per digit, chosen for JavaScript's 2⁵³, which
  has `BigInt` and never takes this library.
- **K(ℕ\*) for every set above the word**: it is the general one, but the fixed limbs' bounds are
  enforced by the same guard, and V_n has no header to read or capacity to compute. Which is faster
  for a bounded set is a measurement.

## Consequences

- `big-fib` runs on windows, and so does every program over an unbounded `Int`.
- A value's storage is a run-time size. It is released at its death (ADR 0096, R1a), and reused by
  fit since ADR 0100: a dead region serves any request it fits, compared at run time.
- Operations whose result length depends on the operands, which is all of them, compute a capacity
  first, which is O(1) from the headers.
