# Numbers: the ring ℤ[1/10], its sets and its rounding

Status: **specified 2026-11-01 (ADR 0086)**; step 1's four parts built (§9): exact numerals,
decimals held as scaled words, and doubles, each rounding the call of a target declaration's `ensure`.
The host's double as its own type, and its decode, are not built (§6.3). This file says what a number
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
definitions belong in `lib/num.oro`, with a test checking the compiler's procedure against each
definition on a small instance (ADR 0035's "a theory checked once").

**Built so far, and what waits** (grids-2026-11-01): the definitions are not yet written in
`lib/num.oro`, because J cannot state them. F(p, …) is a replacement set, the image
{ m·2^(e−p+1) : m, e } (ADR 0061, not built), and its separation form needs ∃ or a logarithm. Until then
**one procedure decides every grid** (`core/grid.go`), and its test stands in for the library's
definition: `TestTheGridIsItsDefinition` enumerates F(3, −2, 2) from the definition above and checks
every mode at every quarter of the finest spacing, and `TestF64IsBinary64` checks the p = 53 instance
against the host's binary64. The procedure is the cell's algebra. A grid is **locally uniform**: x lies
in a closed cell whose ends are in G and on which G is u(x)·ℤ, u = 1 on ℤ, 10⁻ᵏ on 10⁻ᵏℤ, and
2^(E−p+1) on F(p, emin, ∞) with E = max(⌊log₂|x|⌋, emin). So r_G,μ(x) = u·r_ℤ,μ(x/u) for every mode,
a tie to even being m even in the cell's generator.

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

**A separation of F64 by constant bounds is F64 ∩ [lo, hi]** (`core.SeparatedGrid`): `set { x ∈ F64 :
a <= x && x <= b }` has the characteristic function χ_F64 ∧ a ≤ x ∧ x ≤ b, and that set has one
spelling by the Galois connection of G ↪ ℚ: **G ∩ [a, b] = G ∩ [⌈a⌉_G, ⌊b⌋_G]**, ends in G, so two
spellings of one set are one canonical type, `f64 LO HI`. A strict bound is the adjunction once more:
x < b is x ≤ the greatest element below b. An end not written is F64's own, ±MAX. The reader gives it
that form wherever a set may stand: a binder's set, a table's element, a named set. Any other formula
stays a separation.

**ℤ's normal form, the range ⌈a⌉..⌊b⌋, waits on a repair** (grids-2026-11-01): a result declared in a
range with an infinite end (`Nat`, `10..`) is read as a representation and not a claim (`core/read.go`),
so it is decided by nothing, and `fn h(y ∈ -5..5) (Nat) { y }` returns −5. A separation of ℤ is a
`where` and an `ensure`, which are decided, so it stays a separation until the hole is closed.

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

Comparisons, integer operators and calls are not rewritten, nothing outside `body` is, and a rounding
the program wrote inside is its own, its argument left as written. **The scope rounds N's operations:
a rewritten operation whose least sort is ℤ is the identity** (`(round S e m scope)`, decided on the
IR's least sorts and, conservatively, on the residual), so a loop index inside the scope stays an
integer. This is IEEE's reading, where an integer operation is not a floating one, and it keeps the
sorts stable: round(S, t) has sort N exactly when t has, so the rewrite changes no operation's sort.

## 4. Every answer is a function of the inputs

Every operation is N's (exact), a rounding of an N-term, or a rounding of one ÷ or √ of N-terms, so the
program's value is a function of its inputs (Theorem D). Staging folds a constant rounding exactly: a
rounding of a literal, or of a term of literals, is computed when compiling, by exact rational
arithmetic, and the result is a literal (ADR 0009).

## 5. Facts and representations

Each value of sort N has two facts: its **scale** σ, an upper bound on its least k, and its
**interval**. ρ chooses, cheapest first:

| proven | representation | operations |
|---|---|---|
| ℤ, inside the word | the word | today's |
| σ ≤ k and x·10ᵏ inside the word | **the scaled word**: m = x·10ᵏ, d(m) = m·10⁻ᵏ (ADR 0083's (H, d)) | expanded on the IR into integer operations on m (§6.1), proven by the decision |
| ∈ F64 | **a double** | a rounding realized by a target declaration (§6.2); = and < native |
| anything else | not built in step 1 | refused: "x has no representation yet: an exact interior is step 2; round it, or write round(F64) { … }" |

**A double's facts are an interval of reals with double ends** (`ir/doubles.go`), each operation's
ends rounded outward (Moore 1966), the realized rounding of [lo, hi] being [lo, hi] itself, since r
is monotone and fixes the ends. Every element of F64 lies in [−MAX, MAX], so that is a double's ⊤, by
induction over an execution once every obligation is proven. A guard narrows closed: over the reals
x < c gives x ≤ c, never c − 1. The overflow obligation of `round(F64, t)` holds when t's interval lies
in [−MAX, MAX], since the nearest rounding overflows only from 2¹⁰²⁴ − 2⁹⁷⁰.

*Revised in step 1 part 4*: this section first gave a magnitude exponent, |x| ≤ 2ᵉ. The interval is
finer at no cost, and the exponent's loop rule, e(x) = max(e(x₀), e_y + t) + 1, is false for a rounded
accumulator: each rounding may add a relative 2⁻⁵³. The rule that holds is Theorem A.

**Lemma.** For q ∈ F64 and r the nearest rounding onto F64, |r(q ± y) − q| ≤ 2|y|, and |r(q·y)| ≤ |q|
when |y| ≤ 1. *Proof.* r(v) is an element of F64 nearest v, and q is one, so |r(q ± y) − (q ± y)| ≤ |y|;
for the product, |q·y| ≤ |q| ∈ F64, and r is monotone, odd and fixes F64. ∎ No relative error is used,
so it holds below the normal range and at every magnitude.

**Theorem A.** Let a loop take at most T back edges, and let every continue pass for p a value built
from p by a chain of such roundings, each addition's other operand bounded by its sup |y| at every
iteration, and set β = 2·Σ sup|y|. Then |p| ≤ |p₀| + T·β at every iteration. *Proof.* By the lemma one
iteration moves |p| by at most β; induct on the back edges taken. ∎ It is Theorem 1 of ir.md §7 for a
step the rounding perturbs. T is the integer analysis's trip bound, and when the loop has none, or the
chain multiplies by more than 1, the accumulator's bound is F64's own and its obligation fails.

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

A double represents F64 by the abstraction function d(x) = the real a finite binary64 x denotes: onto
F64, with d(−0) = d(+0) = 0. IEEE 754 compares finite values as reals (§5.11), so **N's order and
equality on two doubles are the host's comparisons, and negation is the host's**, F64 being closed
under it. Every other operation leaves F64 and stands only under a rounding.

A target declaration with `ensure r == round(F64, t)`, where t is a ring term over its parameters
with at most one ÷ at its root, **realizes** that rounding: the equation round(F64, t) = f(x̄), read
right to left (`ir/doubles.go`). Go, the JVM and JavaScript each declare `fadd`, `fsub`, `fmul`, `fdiv`
and `fint` (`targets/*/`), IEEE's nearest-even operations and the conversion of an integer in ±2⁵³.
- **The match is modulo the commutativity of + and ·**, sound because the program's term is an exact
  N-term and a ring identity preserves its value.
- **A pattern's variable binds a leaf**: a value no operation of N defines. A double binds as it is,
  an integer by the conversion (its obligation: |n| ≤ 2⁵³), and a constant only when it is a double,
  so `round(F64, x + 0.1)` is refused, and `round(F64) { x + 0.1 }` rounds the literal first.
- **No match: refused**, naming the term. No target declares a fused multiply-add yet, so
  `round(F64, a * b + c)` is refused on all three.
- **Two roundings are two calls, and never one**, because an inner rounding is a leaf. A double
  rounding the host would fuse is prevented by the declaration's template: Go's specification lets
  `x*y + z` fuse "possibly across statements" unless a conversion rounds the product, so Go's `fmul`
  is `float64(${a} * ${b})`. Java (strict since 17) and JavaScript never contract.
- **A rounding of constants is folded when compiling** (§4), by exact rational arithmetic, to the
  nearest double, a tie to the even one. One past F64 is refused.
- **A comparison of a double with a constant is exact**, by a Galois connection: the inclusion
  ι : F64 ↪ ℚ has both adjoints, ι(x) ≤ q ⟺ x ≤ ⌊q⌋ and q ≤ ι(x) ⟺ ⌈q⌉ ≤ x, so x < q is x ≤ ⌊q⌋ when
  q ∉ F64, x = q is false there, and a constant past ±MAX gives a constant truth. A comparison of a
  double with an integer converts the integer (|n| ≤ 2⁵³); with any other term of N it is refused.
- **Only `'nearest`.** IEEE's directed modes are the host's dynamic state, so `round(F64, e, 'down)`
  is refused; the error-free transformations that would realize it are not built.

The obligations are decided on §5's domain: each rounding's term within ±MAX, each ÷'s divisor away
from 0, each conversion's integer in ±2⁵³, a rounding onto F64 ∩ [lo, hi] inside it, a declared result
F64 ∩ [lo, hi] at its definition, and a parameter F64 ∩ [lo, hi] at every call (ADR 0028). An export's
F64 is assumed of its arguments.

### 6.3 The host's double

A host's double type (`go.float64`, Java's `double`, JavaScript's `number`) holds NaN, ±∞ and −0, so it
is not F64. F64 ⊆ it (the identity), and the way back is the decode `num.of_double`, which fails on
NaN and ±∞ and reads −0 as 0. A host function whose result is a host double is Tier 2, as today.

**Built in step 1, part 4**: the half of this that soundness needs. A double a host computed (a call
that no realization made, a cell or a map the host wrote, and what the dataflow carries one to) is
refused wherever a rounding's operand or N's order would read it, saying so; it may stand where the
host's own operations read it. **Not built**: the host's double as its own type, which every target's
float declarations and the surveys' `float64 → F64` would move to, and the decode, which needs an
option of a double on Java and JavaScript. Until then a Tier 2 declaration types its result F64, and
a program that returns one at an export claims, as before, the host's value.

## 7. Refusals, each with its reason

- A ÷ or √ not at the root of a rounding.
- Two at one root.
- A transcendental, not built.
- `/` or `%` on a non-integer.
- A rounding onto a set that is not a grid restricted to an interval.
- A rounding's obligation unproven.
- A value with no representation.
- A rounding of a double term no declaration realizes, or onto F64 but to the nearest.
- A double a host computed, where the language's numbers would read it.
- A term of doubles not rounded: an exact interior, step 2.

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
3. **Decimals** (built 2026-11-01, [decimals-2026-11-01](../../gauntlet/results/decimals-2026-11-01.md)):
   - `round`, `÷` and the modes;
   - `D(k, R)`;
   - ℤ ≤ N in the checker and the IR's literal sorts;
   - the scaled word (§6.1, `ir/numbers.go`), its obligations, and `lib/num`'s `decimal`;
   - `examples/num/ledger.oro`, equal to hand-written Go on three targets.

   Found and repaired on the way:
   - the refinement layer's ℤ no longer reasons about numbers (`emit/numnames.go`);
   - a π-parameter on a number is realigned to one scale;
   - a D claim at a call is an obligation;
   - the thresholds are closed under the body's scalings (widening up to).
4. **Doubles** (built 2026-11-01, [doubles-2026-11-01](../../gauntlet/results/doubles-2026-11-01.md)):
   - F64 as the language's set, held as a double, and F64 ∩ [lo, hi] as a separation's normal form;
   - the doubles' interval domain and Theorem A (§5);
   - realization by `ensure` (§6.2), with Go's, the JVM's and JavaScript's declarations, comparisons by
     the Galois connection, and constants folded;
   - the scope;
   - a host's double refused where the language's numbers read it (§6.3);
   - the gauntlet's six float programs in the language's numbers: dot, centroid and search one source
     each, generic, report and smooth one per target for their maps, printing and buffers, at parity.

   Found and repaired on the way:
   - `where len(a) < …` never reached the loop: the integer analysis narrowed a π of the table, not
     the table (interval.go, narrowAssumed);
   - a typing's sort depended on the order it met a value (`numSorts`, a least fixed point);
   - doubles are names of N for the refinement layer, which read `x > 9` over ℤ;
   - `SelectNumbers` did not renumber the values it made, so the printer did not factor through the
     IR's text (ADR 0038).

   **Not built**: the host's double as a type and its decode (§6.3); `sqrt` and a fused multiply-add,
   which no target declares; the directed modes; F32; a rounding onto F64 of a decimal; `-checked`
   for a rounding.

**Not in step 1**: exact interiors (expansions, the accumulator, step 2), the transcendentals, F(p, …)
other than F64 and F32, D at an entry point's boundary, Q(k, R), stochastic rounding, accuracy claims.

**Planted faults** (each must fail before its rule is trusted; part 4's in `ir/doubles_test.go`, Go's
fusion in `ir/golang`, a false `ensure` by `gauntlet/differential/cases/f64-round.oro`):
- a scaled add whose alignment overflows the word, accepted;
- a `'nearest` tie rounded away;
- a ÷ not at a rounding's root, accepted;
- `round(F64, round(F64, a*b) + c)` realized as a fused operation (`plantFuseFault`);
- Go's `x*y + z` printed without the conversion (`TestGoNeverFusesTwoRoundings`);
- a strict comparison with a constant kept strict against ⌊q⌋ (`plantGaloisFault`);
- a product by more than 1 taken as a link of an accumulator's chain (`plantChainFault`);
- a host's double let into a rounding (`plantHostFault`);
- a realization whose template computes something else: Go's sum printed as a difference answers
  `0 0 1 1 1 0 0` in the differential case;
- Go's `math.Exp` claimed as `round(F64, exp(x))`: not planted, since a declaration whose `ensure` is
  no ring term realizes nothing (`ringTerm`), and no target declares one;
- a decode letting NaN in: waits for the decode (§6.3).
