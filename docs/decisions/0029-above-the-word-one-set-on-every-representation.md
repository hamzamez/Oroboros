# 0029 — Above the word, a type denotes a set decided by sign and bit length, and every representation enforces the program's one set

Date: 2026-09-24
Status: Accepted. **Amends [ADR 0028](0028-a-definitions-contract-is-checked-at-its-calls.md)'s
decision 4** (what a range above the word denotes) and corrects its reading of a declared result as a
fact. Refines [bigrepr-2026-09-03](../../gauntlet/results/bigrepr-2026-09-03.md) §3a, which made the
bound a bit length and left the sign unsaid.

## Context

Above the target's word, a declared range is not proven. It is **enforced**: every operation that can
grow a value is checked at run time (`big-fit` on a host bignum, the carry and top-limb `guard` on
fixed limbs). The enforcement was chosen at the bound's bit length because that is O(1) on every
host, and bigrepr §3a proved that the limb rung and the host rung admit the same *magnitudes*. It
did not say what either admits for a *negative* value, and they differ.
Measured with `b = 201` (a program declaring `(pow 2 200)`), on every target that has a bignum and on
fixed limbs:

| representation | check | admits |
|---|---|---|
| Go `math/big` | `BitLen() > b`, the length of \|x\| | −2ᵇ < x < 2ᵇ |
| Java `BigInteger` | `bitLength() > b`, two's complement | −2ᵇ ≤ x < 2ᵇ: **printed −2²⁰¹** |
| JS `BigInt` | `(x >> b) !== 0n` | 0 ≤ x < 2ᵇ: **traps on every negative** |
| fixed limbs, all three hosts | the final borrow of `sub` | 0 ≤ x < 2ᵇ: **traps on every negative, even one inside a declared signed range** |

So one program computing a negative intermediate is accepted everywhere and answers on Go and Java
while trapping on JavaScript and on limbs. That breaks the rule the whole integer design rests on:
*every target that accepts a program computes the same integer* (ADR 0026), and representation never
changes which programs are legal (ADR 0009 at the representation boundary).

ADR 0028's decision 4 stated the set as `|x| < 2ᵇ`, Go's. Building the fix found a second fault in
the same decision. The bound is enforced **once per program**, at the largest bit length any
signature declares, because β has erased every other boundary by the time the representation is
chosen (`emit.BigBound`). ADR 0028 read a declared result `(the T e)` as the fact "e is in T's set".
With two bounds in one program that fact is false:

```lisp
(sig h ((n (int 0 1))) (int 0 (pow 2 100)))
(def h (n) (* (pow 2 150) (+ n 1)))           ; violates its own declared result
(sig g ((x (int 0 (pow 2 100)))) (int 0 (pow 2 200)))
(def g (x) (+ x 0))
(export calc)
(sig calc ((n (int 0 1))) (int 0 (pow 2 200)))
(def calc (n) (g (h n)))                      ; compiled without -checked; g received 2^150
```

The program's enforced bound is 201 bits, so `h`'s result is checked against 2²⁰¹, not 2¹⁰¹. The
fact "h's result < 2¹⁰¹" discharged `g`'s obligation, and the obligation was violated at run time: a
false proof, the one kind of error ADR 0028 exists to rule out.

## Decision

**1. The enforceable sets.** A sign-magnitude integer has two observables that cost O(1) at any
size: its sign and the bit length of its magnitude. Every host bignum here is sign-magnitude
(`big.Int` is a sign and a `nat`, `BigInteger` a signum and a magnitude array, a V8 `BigInt` a sign
bit and digits), and fixed limbs are a magnitude. The sets those two observables decide, ordered by
inclusion, are the lattice

    H = { Nₖ = [0, 2ᵏ) } ∪ { Zₖ = (−2ᵏ, 2ᵏ) } ∪ { ℤ },    k ≥ 0
    Nⱼ ⊆ Nₖ ⟺ j ≤ k,   Zⱼ ⊆ Zₖ ⟺ j ≤ k,   Nⱼ ⊆ Zₖ ⟺ j ≤ k,   Zⱼ ⊄ Nₖ
    Nⱼ ⊔ Nₖ = N_max(j,k),   otherwise the join is Z_max(j,k)

**2. What a type above the word denotes.** `(int LO HI)` denotes α(LO, HI), the least member of H
containing [LO, HI]:

    α(LO, HI) = N_b  if LO ≥ 0,     Z_b  if LO < 0,     b = bitlen(max(|LO|, |HI|))

and a range with an infinite endpoint denotes ℤ, which is enforced nowhere. α is the lower adjoint of
the inclusion of H into the intervals (a Galois connection), so it is monotone and α(I) ⊇ I: no value
the declaration admits is ever refused. As an obligation at a call (ADR 0028), an argument must be
proven in α of the parameter's type.

**3. What a program enforces.** One set, E_P = ⊔ α(T) over every type above the word in every
signature of the program, at every operation that can grow a value, **on every representation,
exactly**. The bits were already program-wide, for BigBound's reason. The sign now is too:
- **N_b**: Go `Sign() < 0 || BitLen() > b`; Java `signum() < 0 || bitLength() > b`; JS
  `(x >> b) !== 0n`, unchanged; limbs, unchanged, since the borrow is the sign test.
- **Z_b**: Go `BitLen() > b`, unchanged; JS the same shift on the magnitude; Java excludes −2ᵇ, the
  one negative value whose two's-complement bit length is b and whose magnitude's is b + 1.

  It is a second primitive, `big-fit-signed`, so a target declares what it can enforce, and a target
  lacking it refuses the program rather than dropping the bound (as `fitBig` already does for
  `big-fit`).

**4. Fixed limbs realize Nₖ only.** A program whose E_P is some Z_b does not take the limb rung. On a
target with a host bignum it takes the host's, exactly as a program with an operation the limb library
lacks already does; on a target with none it is **refused at compile time**, naming the signed type.
A refusal is a capability (ADR 0026: each target realizes what it can); a trap on a declared value
would be a different answer.

**5. A declared result above the word is a fact about E_P, not about its own type.** `(the T e)`
tells the analyses only that e ∈ E_P, because nothing else is enforced. So an obligation α(T') is
discharged by an ascription only when E_P ⊆ α(T'), which holds when T' is at least as wide as every
big type in the program. This replaces ADR 0028's reading of the ascription.

## Why not

- **`|x| < 2ᵇ` everywhere (ADR 0028's text, Go's behaviour).** Fixed limbs would need a sign, which
  means signed addition, subtraction, comparison and division in the limb library, and `(int 0 N)`,
  which is every big type the corpus declares, would admit negative values it says it excludes. N_b
  is strictly tighter for LO ≥ 0 and costs the same one comparison.
- **Java's two's-complement set, [−2ᵇ, 2ᵇ).** Asymmetric: it is not α of any symmetric declaration,
  and Go and V8 would need an extra case to admit −2ᵇ. It is what one host's method happens to
  compute.
- **Exact endpoints.** A full-width comparison per operation, which costs what the operation costs
  (bigrepr §3a). H is exactly the sets that are cheaper than that.
- **A per-operation or per-definition sign.** The bits are program-wide because β erased the
  boundaries a finer bound would need; a finer sign has the same obstacle.
- **Enforce each declared result at its own bound**, so `(the T e)` means α(T). It is the other
  way to make decision 5's fact true, and the stronger one: `h` above would trap. It adds a check at
  every inlined call of such a definition. On limbs it needs a narrower-bound test the library does
  not have, since the limb width is one per program. No program in the corpus declares two big
  bounds. Named, not built.
- **Leave it as bigrepr §3a left it.** "What matters is that both representations enforce the same
  thing": they did not, and the rule was stated for magnitudes only.

## Consequences

- Every target that accepts a program with a range above the word admits the same values. A
  differential case with negative values in a signed range, and one with a negative intermediate in
  `(int 0 N)`, pin it; before, both disagreed.
- Emission changes on Go and Java for every program with a bounded big type: one sign test per
  checked operation, O(1).
- A declared big result narrower than the program's widest big type **is not enforced at its own
  bound**. `h` above now refuses the *call* to `g` instead of being proven, but `h` itself is not
  refused. That is the meaning gap refinements.md §6b named, for results above the word, and it
  stays named.
- `(int 0 +inf)` denotes ℤ for enforcement, which is nothing: a negative value in a declared ℕ is
  admitted on every host, consistently. The unbounded rung has no limb form, so no representation
  disagrees; enforcing ℕ's sign would be O(1) and is not built.
- H is the vocabulary for any future bound above the word, such as a finer one if β ever keeps
  boundaries: each member costs one sign test and one bit length.
