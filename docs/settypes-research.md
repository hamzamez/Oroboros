# Types are sets: a logic above the computation

**Decided 2026-10-10: [ADR 0049](decisions/0049-types-are-sets-and-the-logic-never-prints.md)
accepts §9's six recommendations. A primary document in CLAUDE.md until everything here is built.**

Research, 2026-10-10, on hamza's request (quoted in full in §0.1). It reworks
[typevars-research.md](typevars-research.md), which mirrored Go's generics, from first principles,
and it answers [types-direction.md](types-direction.md) (2026-08), whose two layers it keeps and whose
ceiling it raises. Nothing here is built yet.

## 0. The answer, in brief

1. **The language is two languages, and only one of them runs.** The *computation* is terms that
   reduce to a residual and print as host code. The *logic* is sentences about the computation's
   values: types, contracts, invariants, lemmas. **A type is a set of values, and the logic never
   emits code.** What the emitted code may depend on is one thing only: **sets the logic has proven a
   value to lie in**, from which the IR chooses a representation (ADR 0033, generalised). That is
   TLA+'s arrangement, a specification over a program, with the specification also driving the
   compiler, which TLA+'s never does.
2. **Types form a Boolean algebra**, closed under ∪, ∩ and ∖, with ⊥ = ∅ and ⊤ = all values, plus
   the constructors the language already has (×, tagged +, tables, maps), comprehension
   {x ∈ S : φ}, and quantification over sets for polymorphism. "int or f64 or string" is
   int ∪ f64 ∪ string, a set; so is "a nonzero integer", ℤ ∖ {0}; so is "GET or POST".
3. **One theorem makes it cheap to run.** The sorts (int, f64, bool, string, each host type, each
   tuple shape, each variant, tables, maps) partition the values, so every type is uniquely the
   disjoint union of its pieces in each sort: S = ⊎σ (S ∩ σ), and S ⊆ T iff S ∩ σ ⊆ T ∩ σ for every
   sort. A union within one sort is a refinement of that sort and costs nothing at run time. A union
   across sorts is isomorphic to a variant whose tag is the sort, which ADR 0042 already represents
   with no allocation. So untagged unions need no new representation.
4. **The ceiling is a decidability map, drawn in §5.** Per sort, the definable sets have decidable
   Boolean algebras:
   - integers: Presburger-definable sets, which are exactly the semilinear sets (Ginsburg and
     Spanier 1966), so intervals, unions, complements and congruences;
   - strings: regular languages with length constraints;
   - floats: finite unions of IEEE intervals;
   - products and variants: componentwise, by semantic subtyping (Frisch, Castagna and Benzaken
     2008);
   - tables: the array property fragment (Bradley, Manna and Sipma 2006), which covers
     "sorted" and "every element positive";
   - maps' key sets with cardinalities: BAPA (Kuncak, Nguyen and Rinard 2006);
   - host predicates: uninterpreted, decided by congruence closure;
   - the combination: Nelson–Oppen.

   Every check below the line is decided by a complete, terminating, deterministic procedure.
5. **The cliff** is nonlinear integer arithmetic (Hilbert's tenth problem), quantifier alternation,
   string equations with lengths (open), and anything needing induction. Above it we do not fall:
   a programmer climbs by **stating intermediate assertions and lemmas, each of which must be decided
   below the line** (Dafny's "auto-active" verification). That reaches most of what Coq proves
   without proof terms, and keeps lowstar-lessons' rule: no heuristic solver, no instability.
6. **TLA+, Shen and Coq each contribute one principle**, and each has one thing we decline:

   | system | we take | we decline |
   |---|---|---|
   | TLA+ | types are sets; every binder carries its set; checking is a portfolio (decide, explore, prove) | typing as a theorem proved after the fact |
   | Shen | judgements written as sequents | user-written inference rules, which are unproven axioms with an undecidable search |
   | Coq | a small trusted base; extension only by definitions and proven lemmas | proof terms written by hand |
7. **Types become static values.** The language's static level is unrestricted, higher-order and
   erased by staging, so a type is a value computed there:
   - a type operator is a static function, `(or int f64)`;
   - a named type is a definition, `(def nonzero (not 0))`;
   - polymorphism is a static λ over sets, instantiated by β at compile time.

   typevars-research's surface survives unchanged, with this as its meaning. Sets of dynamic values
   are static, and no set is a dynamic value, so the hierarchy is stratified and Russell's and
   Girard's paradoxes cannot arise.
8. **It is a rewrite of the checking half, not of the language.** The checker's ad-hoc relation
   (`agree`, `compatible`, `JoinSameRepr`, open elements) becomes inclusion in the algebra. The
   refinement layer's single-fact entailment becomes a Presburger procedure. The IR's interval
   analysis stays, as the fast abstract domain in a reduced product with the logic. This also
   answers the assessment's open question, "one prover or two": one logic, a stack of procedures.

### 0.1 The request

> I can see that this is mirroring go, c++, mojo and the likes, while I want to push in the direction
> of tla+, shen sequent calculus, coq? since types are sets, and I want to be able to use set
> operation on them. we have union, intersections, subsets .... I want be able to say, this type is
> int or float or string, which means and element can be either one of 3. there are two parts to the
> language, the functions which reduce to computation, and the type system which lives on top and
> reasons about them. the type system does not tranlate to code, it is much more like tla+, it reasons
> about code. I want to reach for the stars, and get everything I could. … let rethink this from first
> principles, do the math, the algebra, the litrature, work out how it would fit in the dicidability
> map. how much can we get without falling off the cliff. compare to what we have, what what is
> proposed, what is researched or specified and not built. let's do this right. and remember, this is
> oroboros, if we have to rewrite, that is not an issue.

## 1. First principles: two languages

### 1.1 The computation

Terms reduce (the atom, the-atom.md): β, δ, the table rule, `if true`. The static level is
higher-order and is erased by staging; the residual is first-order tables and loops, lowered to the
IR and printed. **Nothing in this layer is a type.** A residual has *sorts*, the carriers its values
live in (an integer, a float, a host value of type `*os.File`, a table, a tuple of two things), and
the IR's typing by unification finds them, because a printer needs a carrier for every value.

### 1.2 The values

The universe of values the computation can produce, V, is built in finitely many steps, since data is
finite and non-recursive (ADR 0014):

```
V  =  ℤ  ⊎  F  ⊎  𝔹  ⊎  Σ*  ⊎  ⊎ₕ Hₕ             atoms: integers, IEEE doubles, booleans,
                                                strings of scalar values, each host type's values
   ⊎  V × … × V                                 tuples
   ⊎  ⊎_c {c} × Vᶜ                              a variant's tag with its payload
   ⊎  ⋃ₙ ([0, n) → V)                           tables: finite functions on an initial segment
   ⊎  (ℤ ⇀_fin V)                               maps: finite partial functions
```

Each summand is a **sort**, and the sorts are disjoint by construction: a value carries its sort, so
the integer 1 and the float 1.0 are different values, as on every host that has both. Functions are
not in V. They live at the static level and do not survive staging (closures-direction.md), and so
do types.

### 1.3 The logic

A **type** is a subset of V. The logic is a first-order language whose terms denote values and whose
atomic formulas are membership `v ∈ S`, equality, arithmetic comparison, and the language's pure
operations applied to values. A **claim** is a sentence: a parameter's type is the claim that the
argument is in a set, a `where` is a formula, an `ensures` relates a result to its arguments, and a
loop invariant is a formula at the loop head. **The logic is about the computation and never part of
it**: no formula is printed, and erasing every claim leaves a program with the same meaning
(ADR 0009's staging law, stated for claims).

### 1.4 The one bridge

The emitted code may depend on the logic in one way: **the representation of a value is a function
of a set the value has been proven to lie in**, ρ(S). It is chosen on the IR as integers already are
(ADR 0033: `int` in S, `u64` in U ∖ S, `big` beyond both). A proof that a value lies in [0, 255]
lets it be a byte. A proof that a value lies in int ∪ string makes it a two-slot variant. A proof
that a table holds only positive numbers changes nothing, until a host offers something faster for
positive numbers. The logic constrains code only through ρ, and ρ reads only proven sets. That is
Hoare's data refinement ("Proof of correctness of data representations", 1972) as the single
interface between the two languages, which is the "representation as an instance of a type's
signature" item CLAUDE.md already lists.

This keeps types-direction §3.5's reason for two layers, codegen needs types whose meaning is
fixed, and dissolves its cost. The emitter consults only proven sets, so the logic may grow without a
ceiling and never miscompile: a claim it cannot prove is refused, or proven with help (§6.3). It is
never assumed.

## 2. The algebra

### 2.1 A Boolean algebra with constructors

The types are closed under:

| operation | set | example |
|---|---|---|
| union | S ∪ T | int ∪ f64 ∪ string |
| intersection | S ∩ T | the even numbers ∩ [0, 100] |
| difference, complement | S ∖ T, V ∖ S | ℤ ∖ {0}, a nonzero divisor |
| empty, everything | ∅, V | the type of `abandon`'s value, and `ignore`'s argument |
| singleton | {v} | `3`, `"GET"`, `true` |
| product | S × T | `(tuple int string)` |
| tagged sum | ⊎_c {c} × Sᶜ | `(variant (result T E) (ok T) (err E))` |
| table | ⋃_{n ∈ N} ([0, n) → S) | `(array S)` with its lengths N |
| map | finite K ⇀ S | `(map int S)` |
| comprehension | {x ∈ S : φ(x)} | `{n ∈ ℤ : 0 ≤ n < len a}` |
| quantification over sets | ∀T ∈ 𝒦. …, ∃T ∈ 𝒦. … | polymorphism; Go's `any` as ∃X. X (ADR 0045) |
| dependence | Π(x ∈ S). T(x) | a result whose type mentions an argument: `ensures` |

The laws are the Boolean algebra's, plus the constructors' distributivity: A × (B ∪ C) =
(A × B) ∪ (A × C), and componentwise intersection of products. These are the laws semantic subtyping
normalizes by.

### 2.2 The partition theorem

**Theorem P.** Let σ₁, …, σₖ be the sorts a program mentions. For every type S,

```
S  =  (S ∩ σ₁) ⊎ … ⊎ (S ∩ σₖ)        and        S ⊆ T  ⟺  ∀i. S ∩ σᵢ ⊆ T ∩ σᵢ
```

*Proof.* The sorts partition V (§1.2), and intersection distributes over a partition. ∎

So every question about types is a question about one sort at a time, and each sort has its own
decidable algebra (§2.3). This is the normal form semantic subtyping computes with its "kinds"
(Frisch, Castagna and Benzaken, "Semantic subtyping: dealing set-theoretically with function, union,
intersection, and negation types", J. ACM 55(4), 2008), and it is what makes unions cheap here:
- **within one sort**, a union is just a smaller set of that sort, held in that sort's carrier:
  [0, 10] ∪ [20, 30] is an integer, and `(or "GET" "POST")` is a string;
- **across sorts**, a union of disjoint sets is isomorphic to their tagged sum. int ∪ f64 ∪ string ≅
  int + f64 + string, the tag being which sort the value is in. ADR 0042 already represents a tagged
  sum as its tag and one slot per payload type, with no allocation.

**Elimination is a membership test.** A value of int ∪ f64 ∪ string is taken apart by asking which
piece it is in, and each arm then knows the smaller set:

```lisp
(case x
  (int i)    …i is an int here…
  (f64 f)    …
  (string s) …)
```

That is occurrence typing (Tobin-Hochstadt and Felleisen, "The design and implementation of Typed
Scheme", POPL 2008; "Logical types for untyped languages", ICFP 2010), and with Theorem P it is a
`case` on the tag the representation already has.

### 2.3 Each sort's definable sets

Theorem P reduces everything to the sorts, so the logic is as strong as each sort's algebra:

| sort | definable sets | closed under ∪ ∩ ∖ | inclusion decided by |
|---|---|---|---|
| ℤ | Presburger formulas in one free variable = the **semilinear sets** (Ginsburg and Spanier 1966): finite unions of arithmetic progressions | yes | Presburger arithmetic (Presburger 1929; Cooper 1972; the Omega test, Pugh 1991) |
| F (IEEE doubles) | finite unions of float intervals, with NaN and −0 as their own points | yes | finite comparison; the set is finite |
| 𝔹 | the four subsets | yes | trivially |
| Σ* | regular languages, intersected with semilinear length constraints | yes | automata (Rabin and Scott 1959), with the Parikh image (Parikh 1966) for lengths |
| a host type | the declared subsumption lattice, refined by opaque predicates | yes, as formulas | congruence closure (Nelson and Oppen 1980) |
| tuples, variants | Boolean combinations of products and tagged sums of the above | yes | semantic subtyping's decomposition |
| tables | a length set, an element set, and **array property formulas** ∀i. G(i) → φ(a[i]) | yes, within the fragment | Bradley, Manna and Sipma, "What's decidable about arrays?", VMCAI 2006 |
| maps | key sets with cardinality constraints, value sets | yes | BAPA (Kuncak, Nguyen and Rinard 2006); quantifier-free BAPA is NP-complete (Kuncak and Rinard 2007) |

Formulas spanning sorts are combined by Nelson–Oppen ("Simplification by cooperating decision
procedures", 1979), and theories with finite sorts (𝔹, F) by Tinelli and Zarba's extension (2005).

### 2.4 Polymorphism, re-read

A type variable ranges over sets, and its bound is a set of sets, a member of 𝒫(𝒫(V)) built at the
static level (§7). Two bounds matter:
- **T ⊆ S**, "any subset of S": TLA+'s `T \in SUBSET S`. An interface bound is this: T ⊆ error.
- **T within one of several sorts**: Go's `cmp.Ordered` is ⋃_{σ ∈ {ints, floats, strings}} 𝒫(σ),
  so both arguments of `Max` lie in one sort and `<` applies. T ⊆ int ∪ f64 ∪ string would also
  allow a mixed set, on which `<` is undefined. typevars-research §2's "a bound is a set of types"
  is this, and the algebra now says which set.

A polymorphic declaration is a static function from sets to a signature, and instantiation is β at
compile time (§7.2). Parametricity (Reynolds 1983; Wadler 1989) is checked by typing the body once
with T opaque: T satisfies only what its bound says.

### 2.5 What is not a set

**Usage is not membership.** Two of the language's properties say how a value is used, not what it
is:
- a buffer is linear (ADR 0018, ADR 0020);
- a result is relevant (ADR 0043).

These are substructural: linear logic (Girard 1987), not set theory. They stay a separate judgement,
a modality on top of the sets, as separation logic sits on top of a program's values (Reynolds,
"Separation logic", LICS 2002). The same holds for effects, the purity bit of ADR 0010. The set reading
does not replace them and must not try to.

**ℤ/2ⁿ is not a subset of ℤ.** It is a quotient, and the word's arithmetic is not ℤ's. The research
item "ℤ/2ⁿ as a type" is a new sort, not a refinement.

## 3. What we have, re-read in the algebra

Each construct the language has or has researched, as a set operation, with where it is decided
today:

| construct | in the algebra | decided by | status |
|---|---|---|---|
| `int`, `f64`, `bool`, `string`, host types | sorts | the checker's names | built |
| `(int LO HI)` | an interval: a semilinear set | the IR's intervals; ρ chooses the word | built |
| `string ≤ go.bytestring` | an injection Σ* ↪ B* (UTF-8) | declared subsumption | built |
| `implements` | declared S ⊆ T between host types | the checker's `agree` | built |
| a conditional's join (`JoinSameRepr`) | S ∪ T, when one contains the other | the checker | built, for comparable pairs only |
| `go.Value` | ∃X. X over one-value sorts, a union of sorts, boxed | the checker | built (ADR 0045) |
| `(tuple A B)` | A × B | the checker | built |
| `variant` | a tagged sum, closed, non-recursive | the checker, `case` | built |
| `option` | {none} ⊎ A | the same | built |
| `(array S)` | ⋃ₙ ([0, n) → S) | the checker's open element | built |
| `(map int V)` | finite partial functions | partly; a read is not typed | built, partial |
| `where` | comprehension, φ in linear arithmetic over difference constraints | the refinement layer: entailed by a single fact | built, weaker than Presburger |
| `ensures` | Π: the result's set depends on the arguments | the refinement layer | built |
| facts (F-B) | axioms of a local theory extension (Sofronie-Stokkermans 2005) | instantiated on present terms | built |
| content facts (Houdini, store chains) | array properties, inferred | the refinement layer | built, narrow |
| the interval analysis | an abstract interpretation: a superset of each value's set in the interval domain (Cousot and Cousot 1977; types as abstract interpretation, Cousot 1997) | `ir.Decide` | built |
| size-change termination | not a set: a termination argument | `ir/sct.go`, PSPACE-complete in general (Lee, Jones and Ben-Amram 2001) | built |
| buffers, relevance, purity | usage and effect modalities (§2.5) | linearity, relevance at β, the effect discipline | built |
| refinement sorts for strings, (B* → (B*)*) ∧ (V → V*) | an intersection of function types (Freeman and Pfenning 1991) | — | researched (strings.md §8) |
| representation as an instance of a signature | ρ as data refinement (§1.4) | — | researched (CLAUDE.md "to do") |
| type variables | ∀ over 𝒫(V), §2.4 | — | researched (typevars-research.md) |
| `typeset` | a set of types, §2.4 | — | proposed (typevars-research.md) |
| ℤ/2ⁿ | a new sort, a quotient (§2.5) | — | named as a question |
| unions across sorts, ∩ in general, ∖, singletons, named types, quantified table properties, lemmas | — | — | **not in the language** |

**Read as a whole, the language already has most of the constructors.** It has products, tagged
sums, tables, maps, intervals, comprehension and dependence. It lacks three things:
1. the Boolean connectives on types (union, intersection, difference), beyond joins of comparable
   pairs;
2. a decision procedure as strong as the fragments it could decide: the refinement layer entails a
   goal only from a single fact, where Presburger decides any linear formula;
3. a way for a programmer to climb when the procedure stops (lemmas, invariants).

## 4. TLA+, Shen and Coq

### 4.1 TLA+: types are sets, and checking is a portfolio

TLA+ (Lamport, *Specifying Systems*, 2002) is untyped Zermelo–Fraenkel set theory with choice, plus
temporal operators. **A type is a set, and a type annotation is a formula**: `TypeOK == x \in Nat /\
f \in [Proc -> Nat]` is an invariant, true or false of a behaviour. Lamport and Paulson argue the
case ("Should your specification language be typed?", ACM TOPLAS 21(3), 1999): untyped set theory is
more expressive and closer to ordinary mathematics.

How TLA+ *checks* matters as much as what it says. It uses three tools, each where it is strong:
- **TLC** model-checks finite instances by enumerating states (Yu, Manolios and Lamport 1999):
  complete for the instance, silent beyond it;
- **Apalache** checks bounded executions symbolically with an SMT solver (Konnov, Kukovec and Tran,
  OOPSLA 2019);
- **TLAPS** checks hierarchical proofs, each step discharged by a backend: Zenon, Isabelle, or an SMT
  solver (Cousineau et al., FM 2012).

The specifier writes the proof's *structure*, and the backends fill in each step. That is the
architecture §6.3 proposes: decide what is decidable, and let the programmer cut the rest into
decidable steps.

**What we take**:
- types as sets;
- every binder carrying its set (`\A x \in S`, `{x \in S : P}`);
- operators parametric by set arguments;
- instantiation as substitution (`INSTANCE … WITH`);
- checking as a portfolio.

**What we decline**: typing as an after-the-fact theorem. A compiler's hosts need representations,
so the static half is not optional (§1.4).

### 4.2 Shen: the judgement as sequents, the rules as axioms

Shen (Tarver, *The Book of Shen*, 4th ed. 2021) defines types by sequent-calculus rules, Gentzen's
presentation (1935), which the programmer writes in `datatype` declarations. They are checked by a
Prolog-style search, and the type system is Turing-complete. That is the reach hamza names. It has
two consequences, and types-direction §3.5 recorded both:
- **A user rule is an axiom.** Nothing checks that it is sound, and an unsound rule types false
  programs. A compiler that chooses representations from the types would then miscompile.
- **The search may not terminate**, so "ill-typed" and "the search gave up" look the same.

**What we take**: the judgement Γ ⊢ e ∈ S, presented as sequents, with a fixed set of rules proven
sound once. Sequents are the right notation for stating what the checker does.

**What we decline**: user-written rules. A programmer may extend the logic only *conservatively*:
- by **definitions**, which never prove anything new about the old symbols. That is the theorem on
  extensions by definitions (Shoenfield, *Mathematical Logic*, 1967);
- by **lemmas**, each proven.

This keeps Shen's open-endedness in what a program can state, and drops what it can assume.

### 4.3 Coq: a small trusted base, and proofs as terms

Coq's Calculus of Inductive Constructions (Coquand and Huet 1988, with inductive types after
Paulin-Mohring) lets a type mention terms, and a proof is a term the kernel checks. Checking is
decidable given the proof; finding one is not. Extraction (Letouzey 2002) erases proofs, and
types-direction §6.4 recorded that what remains is correct but not fast. Low\* and HACL\* are
the counterexample that matters: a restricted subset, everything erased, at parity with C.

**What we take**:
- a **small trusted base**, which here is the decision procedures and the host declarations (the
  only axioms, ADR 0022, checked against the hosts by the tooling);
- the discipline that **everything else is a definition or a proven lemma**.

**What we decline**: proof terms written by hand. Dafny (Leino 2010), F\* (Swamy et al. 2016),
Liquid Haskell (Vazou et al. 2014) and SPARK show the other route: the programmer states assertions,
invariants and lemmas, and a procedure proves each one. Leino and Moskal named it "auto-active"
verification (2010). We take that route, with one change forced by lowstar-lessons.md: **the
procedures are complete on decidable fragments, never heuristic over undecidable ones**. F\*'s proof
instability comes from SMT quantifier instantiation over undecidable theories, not from decision
procedures. A complete procedure for a decidable fragment gives the same answer on every build.

### 4.4 The closest working systems

- **CDuce and semantic subtyping** (Frisch, Castagna and Benzaken 2008): types as sets of values, with
  ∪, ∩, ¬, products and arrows, and subtyping decided as emptiness of a Boolean combination.
- **XDuce** (Hosoya and Pierce, 2003): regular expression types for XML, the string algebra of §2.3.
- **Elixir's type system** (Castagna, Duboc and Valim, "The design principles of the Elixir type
  system", 2024): set-theoretic types in a production language with unions, intersections and
  negations, the nearest thing to what hamza asks.
- **Typed Racket**: occurrence typing, unions eliminated by predicates.
- **DML and Liquid types** (Xi and Pfenning 1998–99; Rondon, Kawaguchi and Jhala, PLDI 2008):
  refinements over a decidable index domain. types-direction §6.5 calls DML this project's lineage,
  and `emit/refine.go` is a small DML.

Castagna's survey ("Programming with union, intersection, and negation types", 2023) collects the
theory.

## 5. The decidability map

The line is drawn where a complete, terminating procedure stops existing. Below it, the logic
decides; above it, a programmer supplies steps that are each below it, or the claim is refused.

| fragment | what it gives a program | decidable? | complexity | procedure |
|---|---|---|---|---|
| the Boolean algebra of sorts | int or f64 or string; occurrence typing | yes | linear | bit sets |
| difference constraints | `i < len a`, today's obligations | yes | polynomial | Bellman–Ford; today's single-fact entailment is weaker |
| intervals (abstract) | each value's range, cheaply | sound, incomplete | near-linear | the IR's analysis (built) |
| QF Presburger (linear integer arithmetic) | any linear relation, disjunctions, `d ≠ 0` | yes | NP-complete | simplex with branch and bound, or the Omega test |
| full Presburger, with quantifiers | ∀∃ over integers; the even numbers; "a multiple of 8" | yes | 2^2^(cn) lower bound (Fischer and Rabin 1974) | Cooper 1972, Omega |
| semilinear sets of integers | named integer types, closed under ∪ ∩ ∖ | yes | as Presburger | the same |
| products, variants (semantic subtyping) | unions of tuples and sums | yes | exponential worst case, fast in practice | DNF with decomposition |
| float intervals | ranges of doubles, NaN excluded or not | yes | finite | comparisons |
| regular languages | `(or "GET" "POST")`, string patterns | yes | PSPACE for regular expressions, polynomial for automata | automata |
| regular + length | "a 6-letter word of hex digits" | yes | via the Parikh image | automata + Presburger |
| word equations | x · y = y · x | yes (Makanin 1977; Plandowski 2004, PSPACE) | high | rarely needed |
| QF arrays (McCarthy 1962) | stores and reads | yes | NP | the theory of arrays |
| array property fragment | sorted, all positive, initialised up to k | yes | by reduction to the base theories: each universal is instantiated over a finite set of index terms | Bradley, Manna and Sipma 2006 |
| QF BAPA | key sets, their sizes, maps' domains | yes | NP-complete (Kuncak and Rinard 2007) | reduction to Presburger |
| uninterpreted functions | host predicates (`sorted?`), pure host calls | yes | O(n log n) | congruence closure |
| combination | everything above in one formula | yes, for disjoint stably infinite theories | as the parts | Nelson–Oppen 1979; Tinelli and Zarba 2005 |
| size-change termination | loops terminate | yes | PSPACE-complete | `ir/sct.go` (built) |
| finite-state temporal logic | protocols, concurrency (later) | yes | PSPACE for LTL | model checking, TLC's way |
| **the cliff** | | | | |
| word equations with length constraints | concatenation and lengths together | **open** (Ganesh et al. 2012) | — | — |
| nonlinear integer arithmetic | x · y = z, every variable unknown | **no** (Matiyasevich 1970) | — | intervals over-approximate it soundly |
| nonlinear real arithmetic | — | yes (Tarski 1951; Collins 1975) | doubly exponential | not applicable: floats are not the reals |
| first-order logic with quantifier alternation and functions | arbitrary specifications | **no** (Church 1936; Turing 1936), semi-decidable | — | — |
| induction | the sum of a table, a recursive specification | **no** in general | — | a user's loop invariant, or a lemma |
| full F<: subtyping | higher-rank bounded polymorphism | **no** (Pierce 1994) | — | not needed |
| System F inference; intersection type inference | — | **no** (Wells 1994; inference with intersections characterizes strong normalization) | — | we check, never infer schemes |
| user-defined rules (Shen) | anything | **no**, Turing-complete | — | declined |

**How much we get without falling off**: everything in the table above the line. That covers every
example in the request, every obligation the language makes today, and all of the following:
- **unions of sorts:** "int or f64 or string";
- **finite sets of values:** "a nonzero divisor", "an HTTP method";
- **integer patterns:** "an even number in [0, 100]", "a multiple of 8";
- **table properties:** "a sorted table", "a table whose first k elements are written";
- **map properties:** "a map whose keys are exactly these";
- **string patterns:** "a string of at most 64 hex digits";
- **numeric postconditions:** "a function returning a number in the input's range".

**What lies above it**, and how a program climbs:

| beyond the line | climbed by |
|---|---|
| a claim needing induction ("this loop returns the sum") | a loop invariant the programmer states. Each check, entry, preservation and exit, is one decidable query. Dafny's way |
| a claim needing a lemma ("merge preserves sortedness") | a lemma the programmer states and proves by the same procedures, possibly with its own invariants. A lemma is a definition with a proof obligation, so the extension stays conservative |
| a nonlinear bound | the interval analysis's over-approximation, which is sound; or narrowing a range so the product is linear in one unknown |
| a claim no step reaches | refusal with the undischarged formula shown; or `-checked`, a run-time check where a host can make one; or, for a finite model, exploration labelled as evidence, not proof (TLC's role) |

## 6. The proposal

### 6.1 One logic, one judgement

The checker's judgement is Γ ⊢ e ∈ S: in context Γ (the facts on the path), the value of e lies in S.
It is presented as sequents with fixed rules, proven sound once against the reduction rules (§4.2).
A residual is checked by computing, for each value, **the strongest set the decided fragment can
state**. The interval analysis gives a fast first answer, and Presburger and the other procedures
refine it where a claim needs more. That is the reduced product of abstract domains facts.md §3.2
already calls the principled combination.

**Every claim is an inclusion query.** A parameter's type, a `where`, an `ensures`, a `sig`'s result,
a user's assertion and a lemma's statement each become one, and each is decided below the line or
refused. Today's two provers become one logic with a stack of procedures:
- the term-level refinement layer;
- the IR's interval decision.

### 6.2 Types are static values

The static level already computes at compile time, unrestricted and higher-order. Types belong there:

- `int`, `f64`, `string`, a host type and a variant are **static constants** denoting sets;
- `or`, `and`, `not`, `tuple`, `array`, `map`, `option` and `where` are **static functions** on sets;
- a literal in a type position denotes its **singleton**;
- a named type is an ordinary definition, `(def nonzero (not 0))`;
- a polymorphic declaration is a **static function from sets**, and instantiating it is **β at
  compile time**. typevars-research's `((errors.AsType strconv.NumError) e)` is exactly a static
  application.

This is how Zig treats types (types are compile-time values), and how F\* and Idris let types be
terms. It is also safe. The sets range over V, and V contains no sets (§1.2), so the hierarchy is
stratified: dynamic values, sets of them, sets of sets for bounds. There is no Type : Type, so no
Girard paradox (Girard 1972; Hurkens 1995), and no set contains itself, so no Russell paradox.

### 6.3 How a programmer climbs: assertions, invariants, lemmas

Three forms, each a claim the logic must prove, never one it assumes:

- `(assert φ)` in a body: a formula at that point, proven from the path's facts, and then a fact
  itself. Dafny's `assert`, TLAPS's proof step.
- an invariant on a `loop`: proven at entry and preserved by each back edge, then known at each exit.
  The refinement layer already infers some (Houdini); a stated one is checked the same way.
- `(lemma name (∀ … φ) proof-steps)`: a named, proven sentence a later proof may instantiate. Its
  proof is a sequence of assertions, each decided. A lemma is a definitional extension with an
  obligation, so the logic stays conservative over its trusted base.

Coq's power is reached by structure, not by proof terms: the programmer cuts a hard claim into
decided steps, as a TLAPS proof does.

**The logic may define more than the computation can run.** A specification function, such as
`(sum a i j)`, the sum of a table's elements from i to j, is naturally recursive, and the computation
has no recursion (ADR 0014). The logic may still define it by recursion over finite data, since it
never runs. TLA+ has `RECURSIVE` operators and Dafny has `function`s for this reason. Such a
definition is well founded and conservative. Reasoning about it is exactly where induction is
needed, so a claim about `sum` is climbed by a loop invariant and a lemma, never decided outright.

### 6.4 A union at run time

By Theorem P, ρ(S) depends only on which sorts S meets:
- **one sort**: that sort's carrier, narrowed by its refinement (a byte for [0, 255]). A refinement
  costs nothing at run time.
- **several sorts**: ADR 0042's representation, a tag and one slot per sort, with no allocation.
  `case` on sorts reads the tag. Across a host boundary into Go's `any`, the box (ADR 0045).
- **a singleton**: no storage at all; the value is known.

A union never introduces a hidden allocation (CLAUDE.md's constraint), because the tagged-slots form
is the one variants already use.

### 6.5 What it replaces

| today | becomes |
|---|---|
| the checker's `agree`, `compatible`, `joinOf`, `JoinSameRepr`, open elements (a syntactic approximation of ⊆, emit/check.go and emit/tabletype.go) | inclusion in the algebra, by Theorem P and each sort's procedure |
| the refinement layer's single-fact entailment over difference constraints | a Presburger procedure (QF first), with arrays, BAPA and strings added as programs need them |
| the IR's typing by unification | sort inference only: carriers, which printers need; sets come from the logic |
| ρ chosen for integers alone (ADR 0033) | ρ chosen for every sort from proven sets (§6.4) |
| facts F-B as axioms | unchanged: they are instances of the trusted base |
| the interval analysis | unchanged: the fast domain in the reduced product |
| linearity, relevance, purity | unchanged: modalities on top of the sets (§2.5) |

**The cost, honestly estimated.** The checking half is about 6,900 lines today, measured 2026-10-10
without tests: the checker and table typing in `emit` (1,327) and the refinement layer (5,561). A Presburger core is a few thousand lines
whichever algorithm is chosen (the Omega test, or simplex with branch and bound and Cooper for the
quantified case). The set algebra's normal forms are about a thousand. The array, string and BAPA
procedures come later, each when a program needs it. This is an estimate, not a measurement, and §8's
first step is the measurement that would correct it.

## 7. The syntax: pleasant and uniform

### 7.1 Principles

1. **A type is a set, and its operators are the connectives of membership.** `(or int f64)` is
   {x : x ∈ int ∨ x ∈ f64}, the same `or` the language's terms use, read as a predicate. TLA+
   makes the same identification between a set and its membership predicate.
2. **Every binder is a name and its set**, for values and for types alike: `(x int)`, `(E error)`,
   `(where (x int) …)`. This is TLA+'s `\A x \in S`.
3. **A head binds and an application instantiates**, as variants already do: `(variant (result T E) …)`
   is declared, and `(result int error)` is applied.
4. **One spelling per idea.** Ranges stay `(int 0 255)`, the most common refinement, now read as one
   semilinear set among others.

### 7.2 The forms

```lisp
; types are static values
(def number  (or int f64))
(def method  (or "GET" "POST" "PUT"))         ; literals are singletons
(def nonzero (not 0))                         ; ℤ ∖ {0}, within int where it is used
(def even    (where (n int) (= (% n 2) 0)))   ; comprehension: binder, then formula
(def byte    (int 0 255))                     ; a range, as today

; in signatures, as today
(sig div ((a int) (b nonzero)) int)
(sig mean ((xs (where (t (array f64)) (> (len t) 0)))) f64)

; a union, taken apart by sort
(sig show ((v (or int f64 string))) string)
(def show (v)
  (case v
    (int i)    (string-of i)
    (f64 f)    (format-float f)
    (string s) s))

; polymorphism: a head binding sets
(sig (AsType (E error)) ((err error)) (option E) …)
((errors.AsType strconv.NumError) e)

; climbing: assertions, invariants, lemmas
(assert (< k (len a)))
(loop ((i 0) (s 0)) (invariant (= s (sum a 0 i))) …)
(lemma merge-sorted (… ) …)
```

Each form is one the language already has, generalised:
- **`or`, `and`, `not`:** the reader's connectives, in a type position.
- **`where`:** the comprehension a `where` clause already is.
- **literal types:** literals.
- **`def` for named types:** `def`.
- **binding heads:** `variant`'s heads.

Only `assert`, `invariant` and `lemma` are new words. Each says what it is.

## 8. What to do first

**A measurement before a rewrite.** Every obligation the corpus makes today can be run through a
Presburger procedure beside the current single-fact entailment. The counts would show:
- what the stronger procedure proves that the current one refuses;
- what unions, intersections and differences would let programs state that they currently clamp or
  guard;
- the compile-time cost on `freq`, measured by pairs.

That decides the order. Then, in the order the algebra builds:

1. **The Presburger core**, replacing the refinement layer's entailment, checked against the
   current one on every obligation. A disagreement is a bug in one of them, found by a planted
   fault.
2. **The Boolean algebra of sorts and Theorem P**, replacing `agree` and `compatible`, with unions
   across sorts represented as ADR 0042's slots, and `case` on sorts.
3. **Types as static values**: `def` of a type, literal singletons, `or`, `and`, `not` and `where` in
   type positions, and polymorphism as static λ. typevars-research's first instances, `errors.AsType`
   and `errors.As`, are built in this form.
4. **Assertions and loop invariants**, then lemmas.
5. **Arrays, strings and BAPA**, each when a program needs it.

The balance risk is real, and this plan is all compiler. The order above lets each step be demanded
by a program: the error-model program for (3), a sorted-table program for the array fragment, a
protocol for temporal logic when concurrency arrives.

## 9. Decisions for hamza

1. **Two languages**: a computation that reduces and prints, and a logic of sets that reasons about it
   and never prints. The only bridge is ρ, a representation chosen from proven sets. *Recommended.*
2. **The decided fragment is the ceiling of automation**, as in §5's table above the line. Anything
   else is refused, or proven by a programmer's decided steps. *Recommended.*
3. **Our own complete decision procedures**, starting with Presburger, **not an external SMT
   solver**. This keeps lowstar-lessons' rule: complete procedures on decidable fragments are stable,
   while heuristic quantifier instantiation is what makes F\*'s proofs unstable. An external solver
   could be a cross-check in tests, never the checker. *Recommended.*
4. **Extension only by definitions and proven lemmas**, never by user rules or unproven axioms. The
   host declarations stay the only axioms, checked against the hosts. *Recommended.*
5. **Types are static values**, with `or`, `and`, `not` and `where`, literal singletons, named types
   by `def`, polymorphism as static λ, and binders that are a name and its set. *Recommended.*
6. **The order of §8**, starting with the measurement. *Recommended.*

## 10. Not here

- **Recursive types and inductive data**: excluded with recursion (ADR 0014). Semantic subtyping
  handles them coinductively, if the language ever wants them.
- **Floats as reals**: IEEE arithmetic is not a field, and facts.md already refuses float facts. Sets
  of floats are finite unions of intervals, and arithmetic on them is the host's.
- **Proof terms**: the climb is by decided steps (§6.3), not by hand-written proofs.
- **Gradual typing**: the language's `any`, the absence of a claim (target-files.md §2), stays for
  untyped host declarations. In the logic it is "no claim", which is different from ⊤ = V (every
  value), and the two must not be merged.
- **Temporal properties of concurrent programs**: TLA+'s home ground. It is decidable for finite
  state by model checking, and belongs with concurrency-research.md's decisions.
