# Contracts on pure calls are facts: the linear fragment gets a signature of atoms

2026-09-15. [spec/theories.md §7.9, §7.10 item 3](../../docs/spec/theories.md),
[facts.md §6](../../docs/facts.md), [postconditions.md](../../docs/spec/postconditions.md).

## 1. What was missing

A primitive's contract is `∀x̄. P(x̄) → Q(x̄, f(x̄))`. For an **impure** call, ADR 0010 let-binds the
result, so `Q` is assumed about a variable the linear fragment can read. For a **pure** call there is
no binder: reduction substitutes it.

So HEAD carried the guarantee as an **opaque string** that discharged only an identical obligation.
`need` requiring `0 ≤ (size v)` against `size` ensuring `0 ≤ result` succeeded by syntactic match. `need1`
requiring `1 ≤ (size v) + 1`, one linear step away, was *propagated, not proven*. postconditions.md
named that limitation and its fix, and facts.md §6 named the mechanism.

**The witness was written first and failed against HEAD**: `tgt.need1: refinement propagated, not proven`.

## 2. The fragment is a theory over a signature

The linear fragment reads terms as linear forms over **atoms**: parameters, lengths `len t`, and
quotients by a literal `x / k`. That set is a *signature*: the uninterpreted terms of the theory the
refinement layer decides.

A **pure host call is a sound atom by referential transparency**: two occurrences of one printed
application in a closed residual denote one value. So the contract `Q` assumed about `(size v)` and the
goal mentioning `(size v)` can name one variable, `app(tgt.size v)`, and `0 ≤ app ⊢ 1 ≤ app + 1` is
ordinary linear entailment.

**The signature cannot be decided from the term alone, and that is the soundness point.** A bare
application is not pure in general. A buffer read `(b i)` is headed by a name, is impure (ADR 0018),
and two occurrences across a store differ. Keying it by its printed form would equate two different
values. So `Σ` comes from the target: `pureAtoms(tgt)` admits an application of a declared **pure**
primitive of kind `expr`, and nothing the fragment already interprets or keeps opaque on purpose.
- Arithmetic, masks and shifts stay out, so `(* x y)` and a non-literal `%` are exactly as opaque as
  before.
- Comparisons and connectives stay out: they are propositions, not terms.

**One signature, every reading.** `asLinear` and `obligation` are now `asLinearIn(Σ, t)` and
`obligationIn(Σ, t)`, with nil giving the old fragment exactly. `Σ` is carried on the refiner and on
every fact set, and every linear reading in the refinement layer goes through it:
- `assume`;
- `let`, `bind`, `joinConditional`, `iterate`;
- `discharge`, `indexObligation`, `valueLength`;
- fact instantiation.

A fact and a goal about one pure call cannot disagree about whether it is a variable, which is the
shape of *"two layers, and only one was ever told"*.

## 3. The gate facts.md predicted, observed in a unit test

facts.md §6: *"naming more atoms can turn propagated, not proven into refused, since a named atom makes
an obligation readable that was opaque before."*

It fired at once, in `TestPrimEnsuresDischargesADownstreamObligation`'s control. With no postcondition,
`0 ≤ size(v)` used to be propagated. It is now a linear goal nothing entails, and it is **refused**.
That is the honest report of an obligation that does not follow. The control asserted the old escape
route, and now asserts the property, *not proven*.

## 4. Witnesses

- **The capability** — `TestAPureContractProvesALinearConsequence`: `0 ≤ size(v)` proves
  `1 ≤ size(v) + 1`. It failed against HEAD. **Control**: with no `ensures` it is not proven.
- **Soundness of Σ** — `TestTheAtomSignatureAdmitsOnlyPureHostCalls`:
  - a pure `expr` call is an atom;
  - an impure call, a `stmt` template, `+`, `*`, `<=` and a non-primitive head (a buffer read's) are not.
- **The syntactic case still holds** — the existing contract test discharges exactly as before.

## 5. Cost

- **Code**: `linear.go` 384 → 394, `refine.go` 788 → 809, **+31**.
- **`go run ./cmd/check`, every step, passes**: emission 194 of 194 files byte-identical, **no compiler
  note changed**, proof counts identical (2,307 of 2,413 integer operations, 345 of 382 loops),
  differential on four targets, tooling. **The gate facts.md predicted fired in a unit test and nowhere in
  the corpus**: no program had an obligation about a pure host call that an opaque atom was hiding. That
  is a measurement of this corpus, not a guarantee about the next program, so the refusal path stays
  pinned by the control above.

## 6. Not built

- `(length N)` and `(length-of N)` respelled as `ensures` (theories.md §8.4). The gate this was waiting
  on is now open, since a pure call's result can be an atom.
- Facts in target layers beyond `max-len`.
- Reporting a proof that used a target's fact as not portable (§7.7).
