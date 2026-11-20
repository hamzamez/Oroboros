# Research: the logic, quantified formulas over sets and the procedures that decide them

2026-11-20. hamza decided ADR 0145 (L1–L7), then: "yes, reorder and start the research note. always
mathematical and algebraic." This note is the first of the logic's six stages. It fixes four
things:
- the formulas' abstract syntax and sort;
- where they live from the reader to the residual;
- the procedure for each fragment, and how the fragments compose;
- the order of the build, each step with its program.

It derives from the parts already in force:
- [settypes-research.md](settypes-research.md) §5, the decidability map;
- [spec/abstract.md](spec/abstract.md) §5–§7: ε the erasure homomorphism, 𝒪 the obligations,
  Theorem O;
- [facts.md](facts.md): facts as local theory extensions;
- ADR 0049, 0062, 0145.

**Short answer.**
1. **A formula is a term of a second signature Φ over the computation's pure terms**: atoms are
   comparisons and the predicates of Σ_L (`nulfree`, `len`), closed under ¬ ∧ ∨ ⇒ ⇔ and the bounded
   quantifiers ∀ x ∈ S : φ, ∃ x ∈ S : φ. Its sort is 𝔹, it is never lowered, and ε erases it
   (Proposition E1, already proven).
2. **Every sequent Γ ⊢ φ is decided as the unsatisfiability of Γ ∧ ¬φ**, and the quantifiers are
   handled by polarity, before any procedure runs (§3):
   - **an existential in that formula** (a ∀ of the goal, an ∃ of a hypothesis) is a fresh constant,
     a Skolem constant, and costs nothing;
   - **a universal in it** (a ∀ hypothesis, an ∃ goal) is the only hard case: instantiated over a
     finite set of terms where the fragment makes that complete (the array property fragment), or
     eliminated (Presburger), or refused.
3. **The existing facts engine is already that instantiation**, for schemata with one trigger
   (facts.md: Ihlemann, Jacobs and Sofronie-Stokkermans 2008). The array property fragment's
   index-set theorem (Bradley, Manna and Sipma 2006) is the same algorithm with a completeness
   theorem for ∀ over indices. So stage 3 generalizes `emit/fact.go`, not a new engine.
4. **Ground formulas are decided by the Omega test** (`presburger/`, complete over ℤ), with reads of
   tables and calls of host predicates as uninterpreted terms. Those join by congruence closure, and
   the theories combine by Nelson–Oppen.
5. **The build order follows the polarity**:
   - stage 2: formulas and `assert`, ∀ goals by Skolem constants and Presburger elimination for the
     rest;
   - stage 3: ∀ hypotheses over tables, with GetWindowTextA's promise;
   - stage 4: congruence closure;
   - stage 5: `spec` and `lemma`;
   - stage 6: `-checked`'s loop.

   Each stage has its program (§7).

No decision is left for hamza; §8 names the two places where a choice could have been made and says
why the algebra makes it.

## 1. The formulas: a second signature over the computation's pure terms

ADR 0049: the language is two languages, a computation that reduces and prints, and a logic that
never prints; the one bridge is ρ. abstract.md §2 gives the computation's binding signature Σ. The
logic's is

    Φ ::= t₁ ⋈ t₂                  ⋈ ∈ {=, ≠, <, ≤, >, ≥}, tᵢ pure terms of Σ (§1.1)
        | p(t̄)                     p a predicate: nulfree, a host's `pure` Bool function, a spec
        | ⊤ | ⊥ | ¬Φ | Φ ∧ Φ | Φ ∨ Φ | Φ ⇒ Φ | Φ ⇔ Φ
        | ∀ x ∈ S : Φ  |  ∃ x ∈ S : Φ          S a set term (L2: always a set)

**1.1 The terms a formula may mention are the pure ones.** Effects are a side condition on β
(ADR 0010), and a formula is never run, so an impure term in it would have no meaning. That is
𝒰's discipline restated: a formula's atoms are terms of Σ whose every operator is pure, plus the
logic's own spec functions (§5). So `b[i]`, `len(b)`, `a + 2*b` and a host's `pure` predicate may
appear, and a print may not. This is a sort check, done when the reader builds the tree. It is
refused there with the operator named.

**1.2 The sort.** Φ's sort is 𝔹, the logic's Booleans. It is not the computation's `Bool`, since a
formula is never a value. A binder's set S is a set term:
- a range `lo..hi`;
- a set name;
- `0..len(b)-1`, the index set of a table, the only dependent set, a term over pure terms.

`x ∈ S` adds S's facts to Γ in the body, as a parameter's set does at a definition (ADR 0028).

**1.3 The bridge, both ways.** A formula reads the computation's values but never writes them, and
the computation reads a formula only through legality. Theorem E (abstract.md §5.1) holds by
construction: ε(assert(φ; e)) = ε(e), so for P and P′ with ε(P) = ε(P′), ⟦P⟧ = ⟦P′⟧. What a formula
may change is whether the program is accepted. Through ρ it may also change a representation,
which changes no answer (ADR 0049 §1).

## 2. Where a formula lives: the reader to the residual

The pipeline already carries claims as marks in the residual (ADR 0037): D maps a claim operator to
a mark, β moves marks with the terms they guard, the decision reads them, and emission erases them.
A formula travels the same way, with one new term shape.

- **The reader** parses Φ wherever L1 allows it (`where`, `ensure`, `assert`, `invariant`, a
  lemma's conclusion and steps). The tokens `∀ ∃ ⇒ ⇔` are lexed today (`core/jlex.go`) and parsed
  nowhere.
- **D** maps a quantifier to a binder term the core already has, a λ: `∀ x ∈ S : φ` is
  `(#forall "S" (fn (x) φ))` and `∃` is `(#exists "S" (fn (x) φ))`. ⇒ is `(#implies φ ψ)` and ⇔ is
  `(#iff φ ψ)`. The λ is the logic's binder, never a closure: the place checker already reads a
  mark's condition as logic and no use of a buffer (core/place.go, "A MARK'S CONDITION IS LOGIC").
  The closure refusals extend that reading to these four heads.
- **β substitutes into formulas**, as it does into a `where` at a call (ADR 0028): a call of
  `f(x ∈ Array(Int)) where ∀ i ∈ 0..len(x)-1 : x[i] ≥ 0` at `f(t)` owes `∀ i ∈ 0..len(t)-1 : t[i] ≥ 0`.
  A formula's λ is never applied by β, since no computation applies it: it is a binder, opened only
  by the decision.
- **ε** removes every mark before printing (`emit.EraseAscriptionsButClaims`'s successors). A
  formula reaching a printer is a compiler bug, and the verifier's W-rules gain one: no `#forall`,
  `#exists`, `#implies` or `#iff` in IR_P.

## 3. Deciding a sequent: polarity first

𝒪 (abstract.md §7) maps a program to sequents Γ ⊢ φ. A sequent is valid iff Γ ∧ ¬φ is
unsatisfiable. Put Γ ∧ ¬φ in negation normal form, pushing ¬ inward past the quantifiers:
¬∀ x : ψ ≡ ∃ x : ¬ψ. Then every quantifier has a polarity:

| in Γ ∧ ¬φ | comes from | treatment | cost |
|---|---|---|---|
| ∃ x ∈ S : ψ | a ∀ goal, or an ∃ hypothesis | **Skolemize**: a fresh constant c with c ∈ S ∧ ψ[c/x] | none: one more variable |
| ∀ x ∈ S : ψ | a ∀ hypothesis, or an ∃ goal | **instantiate** over a finite term set where complete (§4.2), **eliminate** where Presburger (§4.1), or refuse | the hard case |

**Theorem S (Skolemization preserves satisfiability).** Γ ∧ ∃x. ψ(x) is satisfiable iff Γ ∧ ψ(c)
is, for c fresh. *Proof*: a model of the first gives a witness to interpret c by; a model of the
second is one of the first, with x := c. ∎

So **every ∀ a program asks to prove is free**. `ensure ∀ i ∈ 0..len(a)-1 : a[i] ≤ r` (a maximum)
becomes a fresh i, the facts 0 ≤ i < len(a), and the goal a[i] ≤ r: a quantifier-free sequent of
today's kind. The quantifiers that cost are the ones a program assumes: a `where` with a ∀ (a
sorted table), a host's quantified `ensure` (GetWindowTextA, L5), a lemma's premise.

## 4. The procedures

### 4.1 Presburger arithmetic: ground by the Omega test, quantified by elimination

**Ground.** A conjunction of linear (in)equalities over ℤ is decided by `presburger.Satisfiable`, Pugh's
Omega test, complete over ℤ. Disjunction is case-splitting into DNF, exponential in the worst case
and small for the formulas programs write. Divisibility atoms d | t (`n % d == 0` with d a constant)
are linear with one fresh variable, t = d·k.

**Quantified.** A ∀ over an integer range in Γ ∧ ¬φ, whose body is Presburger, is eliminated:
- by the Omega test's projection. That is exact elimination of one variable, dark shadow and
  splinters, which the package already computes on the way to satisfiability, and needs only to be
  returned as a formula;
- or by Cooper's algorithm (1972).

Each removes one variable and leaves a quantifier-free formula equivalent over ℤ.

**Complexity, and where it stops.** Full Presburger arithmetic is 2^2^(cn) in the worst case
(Fischer and Rabin 1974), and QF Presburger is NP-complete. The programs' formulas have one or two
nested quantifiers over small bodies. The strong route runs only on marks containing a quantifier
or ⇒/⇔ (L7), and today's entailment keeps every other obligation. Run on everything, the Omega test
cost 3.5–9.5× in compile time (presburger-2026-10-11).

**Multiplication** of two unknowns is outside (Matiyasevich 1970). Such a formula is refused with
its nonlinear term named, the interval analysis's over-approximation kept for computation.

### 4.2 The array property fragment: instantiation over the index set

**The fragment** (Bradley, Manna and Sipma, "What's decidable about arrays?", VMCAI 2006). An
array property is

    ∀ ī ∈ ℤⁿ : φ_I(ī) ⇒ φ_V(ī)

with two restrictions:
- the index guard φ_I is a Boolean combination of i ≤ e, e ≤ i and i ≤ j, e containing no
  universal variable;
- in the value constraint φ_V, a universal variable occurs only as an array index, a[i], never in
  arithmetic.

For example:
- *sorted*: ∀ i, j : 0 ≤ i ≤ j < n ⇒ a[i] ≤ a[j];
- *nonzero up to k*: ∀ i : 0 ≤ i < k ⇒ b[i] ≠ 0;
- *a function's graph*: ∀ i : lo ≤ i ≤ hi ⇒ c[i] = 0.

**Theorem A (index set, BMS 2006 Thm. 1).** Let I be the set of index terms of a formula F in the
fragment: every e in a read a[e] and every bound e in a guard, plus 0. Then F is satisfiable iff
F′ is, where F′ replaces each ∀ ī by the conjunction of its instances at ī ∈ Iⁿ. *Consequence*: the
fragment reduces to quantifier-free arrays and Presburger, which §4.1 and §4.3 decide.

**This is the facts engine's algorithm with a completeness theorem.** facts.md's schemata are
∀x̄. G(x̄) ⇒ C(x̄) instantiated on the program's present terms to a fixpoint (local theory
extensions, Ihlemann, Jacobs and Sofronie-Stokkermans 2008). An array property is such a schema
with the read a[i] as its trigger, and Theorem A says which instances suffice. So stage 3 lets
`emit/fact.go` take a program's own schemata, from `where`, `ensure` and a host's promise, beside
`lang`'s. It instantiates them over the index set, and gains Theorem A as the statement its planted
faults test.

**Reads after writes**: a store gives a new version, b′ = b[k ↦ v], and its reads follow McCarthy's
axioms:
- b′[i] = v when i = k;
- b′[i] = b[i] when i ≠ k.

A ∀ over b is carried to b′ for i ≠ k. This is how an invariant over a buffer's elements, owed
since ADR 0097, becomes provable: `∀ i ∈ 0..k-1 : b[i] ≠ 0` is preserved by a store at k.

### 4.3 Uninterpreted functions: congruence closure

A table read a[e] and a host's pure predicate p(t̄) are function symbols the procedures do not
interpret. Their only law is congruence: s̄ = t̄ ⇒ f(s̄) = f(t̄). Congruence closure decides equality
with uninterpreted functions in O(n log n) (Downey, Sethi and Tarjan 1980; Nelson and Oppen 1980).
Reads already enter the refinement layer as opaque terms, so congruence closure is what lets two
reads at proven-equal indices be one value.

### 4.4 Combination: Nelson–Oppen

Linear integer arithmetic, arrays and uninterpreted functions are each decided, and a formula mixes
them: a[i + 1] ≤ a[j] ∧ j = i + 1. Nelson–Oppen (1979):
- purify the formula into one part per theory, sharing variables;
- each theory decides its part and propagates the equalities between shared variables it entails;
- a non-convex theory, which ℤ is, propagates disjunctions of equalities, decided by case split.

It is complete for theories that are disjoint and stably infinite, which these three are (Tinelli
and Zarba 2005 for the general statement).

### 4.5 What each fragment buys

| fragment | stage | first program | what it retires |
|---|---|---|---|
| ∀ goal, Skolemized; Presburger elimination | 2 | the Gregorian leap year as a formula; Bloch's midpoint | nothing yet: formulas exist |
| array property fragment, reads over writes | 3 | a table's maximum; binary search on a sorted table; GetWindowTextA's C string | the C string from Win32 (ADR 0091's rest); an invariant over a buffer's elements; guards like `if x < 1000` where a table's elements are bounded |
| congruence closure, Nelson–Oppen | 4 | a sort through a host with its comparator's law | a host sort as Tier 1: one answer on every target |
| spec, recursive lemma | 5 | a table's sum, and the lemma sum(a ++ b) = sum(a) + sum(b) by induction | the sum's claims; a former's laws when one is asked for |
| `-checked`'s loop | 6 | the same claims, unprovable variants, trapped | L6 |

## 5. spec, lemma and assert (stage 5, L3 and L4)

abstract.md §7 already gives 𝒪's clauses for the three. What the decided L3 and L4 add:

**A `spec` is a function of the logic**, possibly recursive, never lowered (ε removes it). Its
recursion must be well-founded, decided by size-change termination over its calls (Lee, Jones and
Ben-Amram 2001), the procedure `ir/sct.go` runs on loops, applied to the spec's call graph. In a
formula, a spec call is an uninterpreted function symbol whose defining equation is an axiom:
- **non-recursive**: unfolded, which is β;
- **recursive**: unfolded a bounded number of times at each occurrence. That is limited
  functions, with fuel (Amin, Leino and Rompf, "Computing with an SMT solver", TAP 2014). Unfolding
  more is a lemma's job, so the procedure always terminates.

**A `lemma` is a theorem with its proof**: binders b̄ ∈ S̄, premise φ, conclusion ψ, steps ψ₁ … ψₘ.
𝒪 gives a sequent per step plus the conclusion's. A recursive lemma is an induction, sound when its
calls on itself are well founded, by size-change again (the measure decreasing at each call is the
induction's). **No axioms** (L4): a lemma with an empty body is one whose conclusion the procedures
decide from its premise.

**`assert φ`** is a sequent at its point, and φ joins Γ after it (abstract.md §7). `assert n(ā)`, a
lemma's use, owes n's premise at ā and gains its conclusion: the rule of a call (ADR 0028), in the
logic.

## 6. A host's quantified promise, and `-checked`

**L5.** A host declaration's `ensure` may be a formula, believed as `pure` is: an axiom of the
host's theory (ADR 0021: a target is a model). Its sentence is kept as evidence, checked both ways
as `ordered`'s is (`TestEveryCallbackMarkerIsItsHostsSentence`'s pattern). For GetWindowTextA:

    fn GetWindowTextA(h ∈ windef.HWND, b ∈ Array(B₈) linear) (r ∈ 0..len(b)-1)
        ensure (∀ i ∈ 0..r-1 : b[i] != 0) ∧ (r > 0 ⇒ b[r] == 0)

The page's sentences (Microsoft Learn, read 2026-11-20):
- the function copies the text "into a buffer";
- the count it returns is "the length, in characters, of the copied string, not including the
  terminating null character". An `A` function's characters are CHARs, bytes;
- on failure the return value is zero.

A C string's length is the index of its first NUL, so the first conjunct is exactly the length's
meaning. The page says nothing of the buffer on failure, so b[r] = 0 is promised only for r > 0.

With that promise in Γ, `nulfree(b[0:r])` (ADR 0091's kernel of ν) is a ∀ goal. It is Skolemized to a
fresh i < r and the goal b[i] ≠ 0, which the promise's instance at i proves (Theorem A). The text is
then ours, at r = 0 the empty string.

**L6.** Under `-checked`, an unproven bounded ∀ i ∈ lo..hi : ψ at its point is the loop
`loop i = lo { i ≤ hi ∧ ψ → again(i + 1); i ≤ hi → abandon(…); else → () }`, ψ pure so the loop has
no effect but the trap. Lemma C's statement (under `-checked` every operation is proven, trapped or
closed) extends to formulas: every claim is proven, trapped or refused.

## 7. The build, each stage a program first

**Stage 2, formulas and `assert`** (`core`: the reader's Φ, D's four heads, the sort check; `emit`:
the strong route in `refine.go`, NNF, Skolemization, Presburger elimination in `presburger/`).
Programs:
- **the Gregorian leap year** (the rule of 1582): `fn leap(y ∈ 1..9999) (r ∈ Bool) ensure r ⇔
  ((∃ k ∈ Int : y == 4 * k) ∧ ¬(∃ k ∈ Int : y == 100 * k)) ∨ (∃ k ∈ Int : y == 400 * k)`, its body
  `y % 4 == 0 && (y % 100 != 0 || y % 400 == 0)`. Divisibility under quantifiers is Presburger, and
  the proof is elimination's;
- **Bloch's midpoint** (the binary search bug, 2006): `ensure lo ≤ m ∧ m ≤ hi` for m = lo + (hi −
  lo) / 2, and the same claim refused for (lo + hi) / 2 above the word, with the word's facts.

Faults planted:
- a Skolem constant reused across two quantifiers;
- an eliminated variable left free;
- a formula reaching IR_P (the new W-rule).

**Stage 3, the array property fragment** (`emit/fact.go` taking the program's schemata, the index
set, reads over writes). Programs:
- **a table's maximum**, `ensure ∀ i ∈ 0..len(a)-1 : a[i] ≤ r`: a ∀ goal over reads, with the
  loop's invariant over the prefix, `∀ j ∈ 0..i-1 : a[j] ≤ m`;
- **binary search** (Bentley's, Programming Pearls), `where ∀ i ∈ 0..len(a)-2 : a[i] ≤ a[i+1]`,
  `ensure r ≥ 0 ⇒ a[r] == t`: a ∀ hypothesis instantiated at the search's reads;
- **GetWindowTextA's C string** on both windows triples, TinyRetroPad's window text, its promise
  with evidence (L5).

Faults planted:
- an instance missing from the index set, which must leave binary search's claim unproven;
- a write's version carrying the old ∀ at its own index.

**Stages 4–6** after the struct forms, the memory model and T1-4 (the plan): congruence closure and
Nelson–Oppen with a host's sort and its comparator's law; `spec` and recursive `lemma` with a
table's sum and sum(a ++ b) = sum(a) + sum(b) by induction; `-checked`'s loop.

**The book**: a chapter on claims, after chapter 14 (state) and before the hosts' chapters:
- `assert` and a ∀ first;
- each refusal before its working form;
- the leap year, the maximum and binary search as its examples.

## 8. What the algebra decided here

- **The formula's binder is the core's λ**, `(#forall "S" (fn (x) φ))`, not a new term kind. The
  core has seven term kinds and a binder already, and a mark's condition is logic already. A new
  kind would be an eighth for every walker to learn.
- **The array fragment extends the facts engine**: a schema with a trigger is what the facts already
  are, and Theorem A is the completeness statement they lacked. A second instantiation engine would
  be two procedures for one theory.

Neither is a choice between meanings, so neither goes to hamza.

## 9. Not here

- **Nonlinear formulas** (Matiyasevich 1970). Refused, the term named.
- **A quantifier over a non-index set inside a value constraint** (∀ x ∈ S : a[f(x)] …, f
  arithmetic): outside Theorem A's fragment, refused with the fragment's restriction named.
- **BAPA and regular strings**: each with its program (L7).
- **Temporal formulas**: with concurrency (ADR 0081).

## References

- Presburger, 1929; Cooper, "Theorem proving in arithmetic without multiplication", 1972; Fischer
  and Rabin, 1974; Pugh, "The Omega test", 1991.
- McCarthy, "Towards a mathematical science of computation", 1962 (read over write).
- Nelson and Oppen, "Simplification by cooperating decision procedures", TOPLAS 1979; "Fast
  decision procedures based on congruence closure", JACM 1980; Downey, Sethi and Tarjan, 1980.
- Bradley, Manna and Sipma, "What's decidable about arrays?", VMCAI 2006.
- Ihlemann, Jacobs and Sofronie-Stokkermans, "On local reasoning in verification", TACAS 2008.
- Tinelli and Zarba, "Combining nonstably infinite theories", JAR 2005.
- Lee, Jones and Ben-Amram, "The size-change principle for program termination", POPL 2001.
- Amin, Leino and Rompf, "Computing with an SMT solver", TAP 2014.
- Floyd 1967; Hoare 1969; Dijkstra 1975; Cook 1978 (the soundness of Hoare logic).
