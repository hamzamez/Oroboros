# Numbers: the ring ℤ[1/10], its sets and its rounding

Status: **specified 2026-11-01 (ADR 0086); being built in step 1** (§9). This file says what a number
means on every target, independently of any. [floats-research.md](../floats-research.md) is the
derivation, and its Part III the surface. Where this file and the code disagree, this file is the
specification and the code is wrong.

## 1. The sort

**N = ℤ[1/10]**, the least subring of ℚ containing ℤ, every binary float and every decimal numeral
(Theorem R). +, −, · are N's, and = and < are decidable on it.

**N is a filtered ring**: N = ⋃ₖ Nₖ with Nₖ = 10⁻ᵏℤ, a chain of free ℤ-modules of rank one. Then
Nₐ + N_b ⊆ N_max(a,b) and Nₐ · N_b ⊆ N_(a+b). Each x ∈ N has a least k with x ∈ Nₖ, its **scale**
σ(x) = max(0, −v₂(x), −v₅(x)), where v_p is the p-adic valuation. Each x then has a unique
**canonical form** (m, k):
- k = σ(x) and m = x·10ᵏ ∈ ℤ;
- if k > 0, then 10 ∤ m.

**The checker's signature is order-sorted** (Goguen and Meseguer 1992): two sorts, ℤ ≤ N.
- ℤ is a subring, so +, −, · are declared at ℤ×ℤ→ℤ and N×N→N, which agree on ℤ (monotonicity).
- = and < are N's, on either sort.
- `/`, `%`, `mod`, `& | ⊕ ~ << >>`, an index, a length and a count are ℤ's only.
- A term's sort is its least sort, and a variable's is the join of every value it is bound to. So
  `loop s = 0 { … again(s + a[i] * b[i]) }` gives s the sort N.

No other numeric sort exists. F64, the decimal grids and the ranges are **sets** (§2). A set never
chooses a sort, except as §3's least-sort rule says.

**A literal is an element of N, exactly**: `3`, `0.1`, `1e-3`, `2.50` and hexadecimal integers. The
core holds its canonical form (m, k) (`core/num.go`). A literal is never rounded unless the program
writes a rounding.

**A numeral with a point or an exponent is a constant declared at sort N**, even when its value is an
integer. An order-sorted signature may declare a constant at a sort above its least one (Goguen and
Meseguer 1992), and Haskell's `0.0 :: Fractional a` against `0 :: Num a` makes the same distinction.
- `2.0` and `2` are the same element, but only `2` is also declared at ℤ, so `a[2.0]` is refused.
- An integer literal is declared at ℤ and so, by ℤ ≤ N, at N too: `s = 0` may begin a sum of numbers
  (§3, built in step 3).

## 2. Sets

Each of these is a set of N, with a characteristic function the compiler decides exactly. Their
definitions are written in `lib/num.oro`, and a test checks the procedure against each definition on a
small instance (ADR 0035's "a theory checked once").

| set | definition | grid G and bounds |
|---|---|---|
| `lo..hi` (integers) | {n ∈ ℤ : lo ≤ n ≤ hi} | G = ℤ |
| `D(k, lo..hi)` | {x ∈ Nₖ : lo ≤ x ≤ hi} | G = Nₖ = 10⁻ᵏℤ |
| `F(p, emin, emax)` | {m·2^(e−p+1) : \|m\| < 2ᵖ, emin ≤ e ≤ emax} | G = F(p, emin, +∞); bounds ±(2 − 2^(1−p))·2^emax |
| `F64` = `F(53, -1022, 1023)` | binary64's finite values, subnormals included, one zero | as above |
| `F32` = `F(24, -126, 127)` | binary32's | as above |
| `{x ∈ S : φ}` | separation (ADR 0084) | S's grid, φ's interval |

**Every set a rounding targets is S = G ∩ [a, b]**, a grid restricted to an interval. A rounding onto
any other set is refused, saying which.

## 3. Rounding, the one lossy operation

For a grid G, closed and discrete, and x ∈ ℝ, let ⌊x⌋_G = max{g ∈ G : g ≤ x} and
⌈x⌉_G = min{g ∈ G : g ≥ x}. Then **r_G,m(x)** is:

| mode `m` | r_G,m(x) |
|---|---|
| `'down` | ⌊x⌋_G |
| `'up` | ⌈x⌉_G |
| `'zero` | ⌊x⌋_G if x ≥ 0, ⌈x⌉_G otherwise |
| `'away` | ⌈x⌉_G if x ≥ 0, ⌊x⌋_G otherwise, on a tie or not |
| `'nearest` (the default) | the nearer of the two; on a tie, the one whose last digit is even (m even in G's canonical generator) |
| `'half_away` | the nearer; on a tie, away from zero (commerce's rounding) |

Each is a **monotone retraction** onto G: x ≤ y ⇒ r(x) ≤ r(y), and r(g) = g for g ∈ G.

- **`round(S, e)` and `round(S, e, m)`** with S = G ∩ [a, b] is r_G,m(e), with the **obligation**
  a ≤ r_G,m(e) ≤ b. For F64 under `'nearest` this is exactly IEEE 754's "the result is finite".
- **`'sat`** is clamp_[a,b] ∘ r_G,nearest, total, with no obligation.
- **Least sort**: ℤ if G ⊆ ℤ, else N.
- **Integer division is a rounding**: `a / b` is r_ℤ,zero(a ÷ b), `a % b` is a − b·(a / b), and both
  need a, b ∈ ℤ and b ≠ 0. `3.5 / 2` is refused, pointing to `round(S, 3.5 ÷ 2)`.

**The operations that leave N** are written only as the root of a rounding's argument, one per rounding
(Theorem T):
- **`a ÷ b`** (ASCII `\div`), with the obligation b ≠ 0;
- **`sqrt(a)`**, with the obligation a ≥ 0.

In a claim they may stand anywhere, because the logic never prints. The transcendentals are specified
(floats-research §14.3) and **not built**: a program naming one is refused, saying so.

**`round(S) { body }`** is a static, lexical scope. D rewrites, inside `body` only:
- every `a + b`, `a - b`, `a * b` and `a ÷ b` to `round(S, …)` of it;
- every `sqrt(a)` to `round(S, sqrt(a))`;
- every literal not in S to `round(S, literal)`.

Comparisons, integer operators and calls are not rewritten, and nothing outside `body` is. Rounding an
integer that is already in S is the identity, so a loop index inside the scope stays an integer.

## 4. Every answer is a function of the inputs

Every operation is N's (exact), a rounding of an N-term, or a rounding of one ÷ or √ of N-terms, so the
program's value is a function of its inputs (Theorem D). Staging folds a constant rounding exactly: a
rounding of a literal, or of a term of literals, is computed when compiling, by exact rational
arithmetic, and the result is a literal (ADR 0009).

## 5. Facts and representations

Each value of sort N has two facts: its **scale** σ, an upper bound on its least k, and its
**interval**. A value proven in F64 has a third, **|x| ≤ 2ᵉ**, its magnitude exponent. ρ chooses,
cheapest first:

| proven | representation | operations |
|---|---|---|
| ℤ, inside the word | the word | today's |
| σ ≤ k and x·10ᵏ inside the word | **the scaled word**: m = x·10ᵏ, d(m) = m·10⁻ᵏ (ADR 0083's (H, d)) | expanded on the IR into integer operations on m (§6.1), proven by the decision |
| ∈ F64 | **a double** | a rounding realized by a target declaration (§6.2); = and < native |
| anything else | not built in step 1 | refused: "x has no representation yet: an exact interior is step 2; round it, or write round(F64) { … }" |

**The magnitude facts.** e(round(F64, t)) = e(t), because 2ᵉ ∈ F64 for e ≤ 1023 and r is monotone.
Then:
- e(a ± b) = max(e(a), e(b)) + 1;
- e(a · b) = e(a) + e(b);
- e(a ÷ c) = e(a) − ⌊log₂ c⌋ for a constant c ≥ 1;
- e(sqrt(a)) = ⌈e(a)/2⌉;
- e of an integer in ±2ⁿ is n.

A loop variable x updated by x ← t(x) + y, with |y| ≤ 2^e_y and a trip bound 2ᵗ the existing analysis
proves, has e(x) = max(e(x₀), e_y + t) + 1. Otherwise its bound is +∞ and the obligation fails. The
overflow obligation of `round(F64, t)` under `'nearest` holds when e(t) ≤ 1023, since
2¹⁰²³ < 2¹⁰²⁴ − 2⁹⁷⁰.

## 6. Realization

### 6.1 The scaled word

Each operation on scaled values is a law of N under d(m, k) = m·10⁻ᵏ, with k the static scale chosen
for each value:

| operation | expansion |
|---|---|
| a + b at scale k | m_a·10^(k−k_a) + m_b·10^(k−k_b) |
| a · b | m_a·m_b at k_a + k_b |
| a < b, a = b | at k = max: m_a·10^(k−k_a) < m_b·10^(k−k_b) |
| round(D(j, R), a, m) with j ≤ k_a | q = r_ℤ,m(m_a ÷ 10^(k_a−j)), at scale j |
| round(D(j, R), a ÷ b, m) | q = r_ℤ,m((m_a · 10^(j + k_b − k_a)) ÷ m_b), at scale j (the exponent ≥ 0, or the divisor carries the power) |

r_ℤ,m(u ÷ v) on integers is **integer division with a mode**, from `/` and `%`: each mode corrects the
truncated quotient by the remainder's sign and size (`'nearest` compares 2|r| with |v|). Every
operation the expansion writes is an integer operation, so the decision then proves it inside the word,
or refuses the program. That is the soundness argument, the same as `ir/pairs.go`'s.

### 6.2 Doubles: a rounding realized by a declaration's `ensure`

A target declaration with `ensure r == round(F64, t)`, where t is a term over its parameters,
**realizes** that rounding. The pass matches each `round(F64, u)` in the program against the
declarations, modulo the commutativity of + and · (sound, because u is an exact N-term and a ring
identity preserves its value). Its operands must be doubles, or integers proven in ±2⁵³, which a
conversion makes doubles exactly. The `where` clauses become obligations.
- **No match: refused**, naming the term. JavaScript declares no `fma`, so `round(F64, a * b + c)` is
  refused there in step 1.
- **Two roundings are two calls, and never one.** A double rounding that the host would fuse (Go's
  specification permits `x*y + z` to fuse unless a conversion intervenes) is prevented by the
  declaration's template, `float64(${a} * ${b})`.
- **A literal under a rounding** is folded when compiling (§4).

### 6.3 The host's double

A host's double type (`go.float64`, Java's `double`, JavaScript's `number`) holds NaN, ±∞ and −0, so it
is not F64. F64 ⊆ it (the identity), and the way back is the decode `num.of_double`, which fails on
NaN and ±∞ and reads −0 as 0. A host function whose result is a host double is Tier 2, as today.

## 7. Refusals, each with its reason

- A ÷ or √ not at the root of a rounding.
- Two at one root.
- A transcendental, not built.
- `/` or `%` on a non-integer.
- A rounding onto a set that is not a grid restricted to an interval.
- A rounding's obligation unproven.
- A value with no representation.
- A rounding of a double term no declaration realizes.

## 8. Conformance

Each Tier 1 name has a case in `gauntlet/conformance/`, run on Go, the JVM and JavaScript:
- `round` in every mode, on ℤ, a decimal grid and F64, at ties;
- `÷` under a rounding, with negative operands;
- the scope against the per-operation spelling;
- the literal 0.1, exact and rounded;
- `/` on integers, unchanged.

## 9. Step 1: what is built, in order, each with a time limit

1. **This specification** and primitives.md's entries.
2. **Exact numerals** (built 2026-11-01):
   - the core's float literal becomes an exact element of N (`core/num.go`, kind `KDec`), which the
     IR's text carries exactly;
   - a numeral outside F64 at a double is refused, where the reader used to round it.

   The corpus's emission stays byte-identical. The checker's ℤ ≤ N and the internal sort's renaming
   (`f64` to `num`) move to part 3, the first part with a value of N that is not a double.
3. **Decimals**:
   - `round`, `÷` and modes;
   - `D(k, R)`;
   - the scaled-word expansion (§6.1) and `lib/num`'s rendering;
   - a program, a ledger read from a file, against hand-written Go.

   Limit: one session.
4. **Doubles**:
   - F64 as the language's set;
   - the magnitude facts (§5);
   - realization by `ensure` (§6.2), with each target's declarations;
   - the scope;
   - the decode;
   - the gauntlet's six float programs (dot, centroid, generic, report, search, smooth) ported to the
     language's numbers, one source each where the rest of the program allows, at parity.

   Limit: two sessions.

**Not in step 1**: exact interiors (expansions, the accumulator, step 2), the transcendentals, F(p, …)
other than F64 and F32, D at an entry point's boundary, Q(k, R), stochastic rounding, accuracy claims.

**Planted faults** (each must fail before its rule is trusted):
- a scaled add whose alignment overflows the word, accepted;
- a `'nearest` tie rounded away;
- a ÷ not at a rounding's root, accepted;
- `round(F64, round(F64, a*b) + c)` printed as `fma`;
- Go's `x*y + z` printed without the conversion;
- a decode letting NaN in;
- Go's `math.Exp` claimed as `round(F64, exp(x))`;
- a loop accumulator's magnitude without a trip bound, accepted.
