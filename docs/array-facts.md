# Facts about a table's contents — F-D researched

Research, 2026-09-16. **F-D₁ derived is built since**: the refinement half
([fdrefine-2026-09-16](../gauntlet/results/fdrefine-2026-09-16.md)), the interval half
([smashfd-2026-09-16](../gauntlet/results/smashfd-2026-09-16.md)) and stride components
([compfacts-2026-09-17](../gauntlet/results/compfacts-2026-09-17.md)); all fifteen clamps of §1.2 are deleted.
F-D₁ declared is not built, and nothing measured asks for it. A draft specification follows in
[spec/theories.md §7.11](spec/theories.md). Written on hamza's *"research and specify F-D, literature, math and
algebra"*.

[facts.md](facts.md) §5 reserved F-D, *"quantified facts over arrays: the array property fragment,
future, its own research"*, and named `tree.oro`'s `d` as its demand. This document is that research. It takes the
demand first (§1), then the logic that decides the facts (§2–§3), then the question that logic does not answer and
our language does — **where the facts come from** (§4–§6) — then what is refused (§7), and the recommendation (§8).

---

## 1. The demand, measured before designed

### 1.1 A correction first

joinindex-2026-09-16 §4 said the 103 propagated index obligations left in the corpus are *"an index computed from a
value read out of a table"*, bounded only by a content invariant. **That was a classification by note, and the notes do
not say which index.** Printing each obligation's index term says something else:

| where | count | the index | what it needs | F-D? |
|---|---|---|---|---|
| `tree.oro` | 110 under `-checked` | `4·clamp(n, 0, 512) + c < 2048` | read the clamp NESTED inside `+`/`*`: purification | **no** |
| `freq.oro`, `dt` | 12 | `2·clamp(…) + 1` | the same purification | **no** |
| `freq.oro`, `src` | 26 | a clamp into `src`, in range iff `len src ≥ 1` | `nw ≤ len src`, a loop RESULT bound (count ≤ trips) | **no** |

**None of the remaining notes needs F-D.** Two classes are a purification gap: a conditional nested inside linear
arithmetic has to be named before the skeleton is read (Nelson & Oppen's flattening, 1979). The third is a loop summary.
Both are recorded as their own next steps (§8.3).

### 1.2 Where the demand really is: in the source

F-D's demand is not in the notes, because programmers already paid for its absence by hand. **A value clamp** is a
conditional written on what a table read RETURNS, so that arithmetic downstream of it is bounded:

| program | clamp | why the programmer wrote it |
|---|---|---|
| `freq.oro` `pat` | `(cli n (t …))` | *"`ord` holds indices, but to the analysis a value read out of a table is just a value"* |
| `freq.oro` `dword` | `(cl sp (cli (+ (wmax) 1) ((dt d) 0)))` — two | a word index stored in `dt`, used to index `sp` |
| `freq.oro` `dcount` | `(cli (+ (wmax) 1) ((dt d) 1))` | an occurrence count, fed to `ndigits` |
| `freq.oro` `dists` | `(cli (+ (wmax) 1) ((dt s) 1))` | *"the one read of a buffer from INSIDE the build that fills it — stratum 0"* |
| `tally.oro` | `pat`, `dcount`, `dvalue`, the increment | freq's shape, four times |
| `tree.oro` `nslc` | `(cn k)` at 6 sites | *"a node index read out of the table is not bounded by anything the compiler can see"* |

**15 clamps at the source level.** Each one:

- costs a data-dependent compare in the emitted code. json-tree-bench-2026-08-26 measured clamped addressing at
  **1.35×**.
- is a place where the program tells the compiler something it cannot derive.
- is, by tree.oro's own comment, a place where *"a clamp hides the fact instead of establishing it"*.

**And one proof gap no clamp closes.** `tree.oro`'s walk proves **388 of 424** integer operations. The 36 left all
chain off `d`, a depth READ BACK OUT OF the worklist that stores it. frozen-2026-08-28 showed the interval fixpoint
cannot bound it: `E ⊇ {0,1} ∪ E ∪ (E+1)` widens to ⊤.

**Acceptance for anything built from this document** is therefore not a note count. It is:

- **the 15 clamps deleted**, with byte-identical program output and 100% of operations still bounded;
- **tree's 36 operations proven**;
- **a benchmark**, since removing a clamp is a claimed speed-up.

---

## 2. What a table is, logically

### 2.1 McCarthy's theory, and ours is its value-only reduct

A table `a : Fin(len a) → V` is a function with a finite domain ([tables.md](spec/tables.md)). The first-order
theory of arrays is McCarthy's (*Towards a mathematical science of computation*, 1962), two axioms over `read` and
`write`:

```
read(write(a, i, v), i) = v
i ≠ j  →  read(write(a, i, v), j) = read(a, j)
```

Its quantifier-free fragment is decidable (Stump, Barrett, Dill & Levitt, LICS 2001, with extensionality). **What a
program needs to say is quantified** — *"every offset in `sp` is at most `len src`"* — and unrestricted ∀ over
array indices is undecidable (decidability-map.md, Cliff 2). The literature's answer is a family of decidable
quantified fragments.

### 2.2 The array property fragment (Bradley, Manna & Sipma, VMCAI 2006)

*What's decidable about arrays?* defines **array properties**

```
∀ī.  G(ī)  →  C(ā[ī])
```

- the **index guard** G is a positive Boolean combination of `≤` and `=` between universally quantified index
  variables and index terms free of them;
- the **value constraint** C mentions the quantified variables **only as array reads** `a[i]` — never `a[a[i]]`,
  never `i` itself in arithmetic, never `a[i+1]`.

Conjoined with a quantifier-free formula over arrays and Presburger arithmetic, **satisfiability is decidable**, by
instantiating each ∀ over a finite **index set** built from the index terms that occur. That is the property this
document leans on: **reasoning is instantiation on present terms** — the same shape as F-B's local extensions.

Two things the fragment contains that decidability-map.md Cliff 2 got wrong:

- **Sortedness IS in it.** `∀i,j. i ≤ j → a[i] ≤ a[j]` is BMS's own running example. It is also the example in
  Bradley & Manna, *The Calculus of Computation* (2007) ch. 11.
- **What is outside** is the SUCCESSOR form `∀i. a[i] ≤ a[i+1]` and nested reads `a[a[i]]`. Both are shown
  undecidable in the same paper.

The map's sentence *"general sortedness is not, because it compares `a[i]` with `a[j]`"* is corrected with a pointer
here. What the map should have said is that **permutation** is outside: *"`b` is a sorted permutation of `a`"* needs
counting, which no fragment here has.

### 2.3 The neighbours

- **Habermehl, Iosif & Vojnar**, *What else is decidable about integer arrays?* (FoSSaCS 2008). This is the
  **periodic** fragment: index guards with modular constraints `i ≡ c (mod k)` and difference bounds, on flat
  counter automata. It is what a STRIDED table's property is written in (§5.3).
- **Ghilardi, Nicolini, Ranise & Zucchelli**, *Decision procedures for extensions of the theory of arrays*
  (AMAI 2007). Combination results, and local-extension readings of array axioms.
- **Alberti, Ghilardi & Sharygina**, *Decision procedures for flat array properties* (TACAS 2014; JAR 2015). The
  FLAT fragment: one quantified index, no nesting. This is the fragment §3 restricts further.
- **Sofronie-Stokkermans**, *Hierarchic reasoning in local theory extensions* (CADE 2005), and **Ihlemann, Jacobs &
  Sofronie-Stokkermans**, *On local reasoning in verification* (TACAS 2008). The array property fragment's
  instantiation is a local extension, which is why F-D sits beside F-B rather than beside SMT (§3.3).

---

## 3. The fragment chosen: F-D₁, flat and value-only

### 3.1 Definition

An **F-D₁ fact** about a table `b` is

```
∀s.  0 ≤ s < len b  →  φ(b[s], x̄)
```

where

1. the index guard is **exactly the domain** of `b`;
2. `φ` is a conjunction of **linear** inequalities over the element (or over each component, when `V` is a
   `(tuple …)`, §5.3) and **frame terms** `x̄`;
3. **`s` does not occur in `φ`**: a property of the VALUES, not of their positions;
4. frame terms are linear terms over names **not bound under the quantifier**: immutable names, or loop variables
   with a known direction (§5.2).

F-D₁ is the flat fragment (Alberti et al.) with its index guard fixed to the domain and its value constraint
index-free. It is the one-segment case of Cousot, Cousot & Logozzo's segmentation (*A parametric segmentation functor
for fully automatic and scalable array content analysis*, POPL 2011), with a relational rather than an interval
segment abstraction.

### 3.2 Why this restriction and not the whole fragment

Each restriction removes a decision the corpus has not asked for:

- **Guard = domain** removes index sets. Every clamp in §1.2 is about every slot. Guards `lo ≤ s < hi`
  (*"slots below `w` are written"*) are in BMS and have no demand yet, so they are reserved as F-D₂ (§7).
- **`s` not in `φ`** removes position-dependent properties. Those are identity-permutation, `a[s] = s` —
  `iota`'s — and it has no reader.
- **One table** removes pointwise relations between two tables, `∀s. a[s] ≤ b[s]` (in BMS, reserved).

### 3.3 Deciding with an F-D₁ fact is instantiation — F-B's mechanism

**Theorem D (instantiation).** Let F be facts including the F-D₁ fact `∀s. 0 ≤ s < len b → φ(b[s], x̄)`, and let
`(b t)` be a read whose domain condition `0 ≤ t < len b` is **proven** under F. Then `F ⊢ φ(b[t], x̄)`.

*Proof.* Universal instantiation at `s := t`, with the guard discharged. ∎

**Completeness for goals in the fragment.** For a quantifier-free linear goal over reads of `b`, instantiating at
every read term of `b` occurring in the goal and the facts is as complete as full instantiation. This is BMS's
index-set theorem specialised to a guard-free index set: the only index terms are the reads.

This is **a local theory extension whose trigger is the read `(b t)`** (Sofronie-Stokkermans 2005). It is F-B's
instantiation with two differences:

- the trigger is an APPLICATION OF A TABLE rather than of a function name;
- the guard is the read's **own domain obligation**.

The **proven** qualifier is Lemma 1 of postconditions.md in a new place. An out-of-range read has no value, so a fact
about its value is licensed only where the read is defined. A read whose bound is merely propagated gives nothing. It
does not refuse anything either: the propagation stands.

**One declaration, three consumers**, exactly as spec/theories.md §7.5:

- **the linear layer** instantiates Theorem D at reads;
- **the interval layer** gets an element range when `φ`'s sides are constants: Theorem T of facts.md, applied to the
  read;
- **element narrowing** reads that range, which makes `ElemType`'s zero-fill-and-stores rule the constant-sided special
  case of what follows.

---

## 4. Where a fact comes from: the part the literature does not decide for us

Deciding with a quantified fact is solved (§3.3). **Establishing** one is not. In a general language it needs an
invariant over every write through every alias, and the tools that do it pay for that:

- **Flanagan & Qadeer** (*Predicate abstraction for software verification*, POPL 2002) and **Lahiri & Bryant**
  (*Constructing quantified invariants via predicate abstraction*, VMCAI 2004) infer universally quantified loop
  invariants with Skolemized index predicates;
- **Gopan, Reps & Sagiv** (*A framework for numeric analysis of array operations*, POPL 2005) and **Halbwachs & Péron**
  (*Discovering properties about arrays in simple programs*, PLDI 2008) partition the index space and abstract each
  part;
- **Dillig, Dillig & Aiken** (*Fluid updates: beyond strong vs. weak updates*, ESOP 2010) exist because a write
  through a pointer that MAY alias has to weaken what is known about everything it may reach.

**We do not pay for aliasing, and the reason is ADR 0018.** This section is the design's core.

### 4.1 A buffer's history is a word over its stores

Within `(build b n e)` (the core form `(build n (fn (b) e))`, tables.md §2.4), every write is a `set` on the buffer's current version, and linearity makes each
version consumed exactly once. So the versions form a **chain**:

```
b₀ = 0ⁿ  →  b₁ = set(b₀, i₁, v₁)  →  …  →  b_m = set(b_{m−1}, i_m, v_m)  =  the frozen result
```

Algebraically, `set(·, i, v)` is an action of a generator on `V^n`, the chain is a word in the free monoid on those
generators, and the table is `0ⁿ` acted on by that word. **A property of every slot is a subset P ⊆ V^n, and it
survives the history exactly when P contains the unit's image and is closed under each generator used.** That is the
invariant set of a monoid action: an orbit stays inside a set containing its start and closed under the generators.

### 4.2 Theorem S (slot induction)

Let `b` be a buffer with length `n`, and `φ(v, x̄)` an F-D₁ value constraint whose frame terms are immutable across the
build. Suppose:

1. **zero fill:** `F ∧ n ≥ 1 ⊢ φ(0, x̄)`, where F is the facts at the `build`;
2. **every store:** at each `(set b′ i v)` on the chain, reached under facts P,
   `P ∧ ∀s. φ(b′[s], x̄) ⊢ φ(v, x̄)`.

Then `∀s. 0 ≤ s < n → φ(b_k[s], x̄)` holds for every version `b_k`, and in particular for the frozen result.

*Proof.* Induction on k.

- **Base:** `b₀` is zero-filled. If n = 0 the claim is vacuous, otherwise (1) applies.
- **Step:** `b_{k+1}[s]` is `v` at `s = i` and `b_k[s]` elsewhere (McCarthy's axioms). The first is (2), whose
  hypothesis is the IH for `b_k`; the second is the IH. ∎

Three things in it are ours and not the literature's.

- **Hypothesis 2 may use the IH at reads of `b′` ITSELF.** A store whose value was read out of the same buffer —
  `(set (dt s) 1 (+ ((dt s) 1) 1))` — is exactly where frozen-2026-08-28 drew its stratum-0 line: a read inside the
  `build` gets ⊤. **The line was a fixpoint's line, not a truth's.** Computing an element range by iterating from reads
  feeds the analysis to itself, which is non-well-founded, and widening answers it with ⊤. *Checking* that a given
  candidate is preserved uses the candidate as an induction hypothesis, which is well-founded by k. **Stratum 0
  becomes provable, not by lifting the stratification but by replacing iteration with verification.**
- **The zero fill is the unit's image, and the vacuous case is exact.** `0 ≤ v < n` holds of `0ⁿ` only when n ≥ 1,
  and that is precisely the case where there is a slot to hold it. So hypothesis 1 is taken under `n ≥ 1`. It is not
  weakened to "every slot written", which is the stronger, guarded property F-D₂ would state.
- **Linearity is what makes the history a chain.** With aliases the versions form a DAG — two writes to "the same"
  table from different histories — and the induction has no order. That is the problem separation logic's frame rule
  and Fluid updates solve. ADR 0018 made it not arise, and uniqueness.md's list of properties that depend on linearity
  gains an eighth.

### 4.3 Theorem S′ — a loop threading a buffer, and frames that move

The corpus's buffers are threaded through `loop`s. Their frame terms are often LOOP VARIABLES: tree's parse table
stores `nn`, which grows, and the walk stores depths bounded by `steps`, which grows.

Let `(loop ((b z_b) (x₁ z₁) …) clauses)` thread `b`, and let `φ(v, x̄)` mention loop variables `x̄`. Suppose:

1. **entry:** the initial value `z_b` satisfies `∀s. φ(z_b[s], z̄)`, by Theorem S on its own `build` with `x̄ := z̄`,
   or by an F-D₁ fact about it;
2. **back edges:** at each `(again b′ x₁′ …)` reached under P,
   - every store on the chain from `b` to `b′` inside the iteration satisfies hypothesis 2 with the frame at its
     CURRENT values, and
   - **the frame moves in φ's direction:** `φ(v, x̄) ∧ P ⊢ φ(v, x̄′)` **for a fresh `v`**.

Then `∀s. φ(b[s], x̄)` is a loop invariant, and holds of every exit value.

*Proof.* Induction on iterations, with Theorem S's induction inside each iteration. The second condition carries the
slots NOT written in an iteration from `x̄` to `x̄′`. ∎

For a template `v ≤ c·x + d` with c ≥ 0 the frame condition is `x ≤ x′`, and the invariant check of
inductive-2026-09-16 has just made that decidable: non-decreasing variables are exactly what it proves. **The
freshness of `v` is what keeps the check quantifier-free.** φ is linear in v, so an entailment with v fresh is an
entailment for all v.

### 4.4 Several buffers at once

freq's merge sort threads TWO buffers and swaps them every pass: `(again (mpass less a b n w) a (* 2 w))`. A store into
`b` copies a value read out of `a`, and next pass the roles exchange. Neither `∀s. 0 ≤ a[s] < n` nor the same for `b`
is inductive alone. **Together they are.** That is inductive-2026-09-16's simultaneous induction with array-valued
candidates: the hypotheses at a back edge are all candidates for all threaded buffers at once.

---

## 5. Finding the facts: Houdini over templates, again

### 5.1 The candidates

Theorems S and S′ **verify** a candidate. **Houdini** (Flanagan & Leino, FME 2001) turns verification into inference:
start from a finite candidate set, discard every candidate some condition fails to preserve, repeat. Discarding only
weakens hypotheses, so the iteration is monotone and stops at the **greatest inductive subset** within |C|+1 rounds.

The candidates for a buffer `b` are TEMPLATES — Sankaranarayanan, Sipma & Manna's shape, already used by
`joinConditional`:

```
0 ≤ v        v ≤ e        v < e
```

with `e` drawn from:

- the linear sides of **guards dominating a store into `b`**, e.g. `nn < nmax` gives `v < nmax`;
- **lengths of tables in scope**, e.g. `v ≤ len src`;
- **the loop variables** threading `b` and their offsets, e.g. `v ≤ steps + 1`.

A candidate per component when the element is a tuple.

The set is finite and fixed before the fixpoint, which is what makes Houdini terminate and keeps this out of Shen's
search (decidability-map.md: *"declared sound axioms, never search"*). A candidate the program needs and the templates
cannot express is a **declaration** (§6), not a larger search.

### 5.2 Every clamp in §1.2 is one template

| clamp | invariant that replaces it | template and frame |
|---|---|---|
| freq `pat` | `∀s. 0 ≤ ord[s] < nw` | `iota` stores `k` under `k < n`; `merge1` stores a value read from the other buffer, **jointly** inductive (§4.4) |
| freq `dword` (both) | `∀s. 0 ≤ π₀(dt[s]) < nw`, and `nw = len sp` | stores `(pat ord nw k)`, which is in range by the row above |
| freq `dcount`, increment | `∀s. 0 ≤ π₁(dt[s]) ≤ k + 1` | stores `1` (`1 ≤ k + 1` since `k ≥ 0`) and `c + 1` with `c ≤ k + 1` by IH, under `k′ = k + 1`; frame `k` non-decreasing. `v ≤ k` is NOT inductive — the first store is `1` at `k = 0` — which is why templates carry literal offsets |
| tally ×4 | the same three | the same |
| tree `nslc` on `stk` | `∀s. 0 ≤ π₀(stk[s]) < nmax` | stores `nn` under `nn < nmax` |
| tree `nslc` on links | `∀s. 0 ≤ π₂(nodes[s]), π₃(nodes[s]) < nmax` | stores `k = nn`; tags and lengths in components 0 and 1 are untouched |
| tree `d` (36 ops) | `∀s. 0 ≤ π₁(wl[s]) ≤ steps + 1` | entry `(set wl 1 1)` at `steps = 0`; stores `d ≤ steps+1` (IH) and `d+1` under `steps′ = steps+1` |

**Every row is F-D₁**, with a component projection where the table holds tuples.

### 5.3 A stride is a product, and the product removes the periodic fragment

`tree.oro`'s node table is written with stride 4, and freq's `dt` is a `(tuple …)` table that the product pass
FLATTENS to stride 2 before the refinement layer runs. As a property of the flat table, "component 2 of every node" is

```
∀s.  0 ≤ s < len nodes  ∧  s ≡ 2 (mod 4)  →  0 ≤ nodes[s] < nmax
```

That is a **modular index guard**, outside BMS and inside Habermehl–Iosif–Vojnar's periodic fragment. It does not need
that fragment, for an algebraic reason:

```
Π_{Fin(4n)} ℤ  ≅  Π_{Fin n} ℤ⁴        (currying — products.md's flattening isomorphism)
```

transports `∀s. s ≡ c (mod k) → φ(a[s])` to `∀t. φ(π_c(a′[t]))`, **a guard-free flat fact on components**. The
compiler does not need the modular guard even after flattening, because it generated the reads: every flattened read
has index `k·e + c` with a literal `0 ≤ c < k`, so an instance is chosen by **syntactic** decomposition of the index,
never by modular arithmetic in the solver. A hand-strided table (`nsl`/`nslc`) produces the same syntactic shape, so it
gets the same treatment.

**A store at an index NOT of that shape** writes an unknown component, so it must satisfy the conjunction of every
component's constraint. That is conservative, and it is what a literal index decomposes into anyway: `(set nodes 1 …)`
is `4·0 + 1`.

---

## 6. Declared facts: when the table has no stores to induct over

A table that arrives from outside has no history:

- an exported function's parameter;
- a host result such as `os.Args` or `strings.Split`;
- a frozen table passed through a boundary.

There, F-D₁ is a **declaration**, and spec/theories.md §7.6 already reserved its surface word, `forall`. Its two
positions follow refinements.md §6b and postconditions.md exactly:

| position | meaning | discharged |
|---|---|---|
| `(where (forall …))` on a sig's table parameter | a precondition | at every call site, by Theorem S/S′ or another declaration; **assumed** at an export |
| `(ensures (forall …))` naming `result` | a postcondition | **assumed** for a `prim`, **checked** against a body by Theorem S/S′ for an exported `def` |

The spelling keeps F-B's `when` for the guard, and the guard must be exactly the domain:

```lisp
(sig Split ((s string) (sep string)) (array string) (host expr "strings.Split(%s, %s)"))
(sig spans ((src (array (int 0 255))) (nw int)) (array (tuple int int))
  (ensures (forall ((w int)) (when (<= 0 w) (< w (len result)))
    (and (<= 0 ((result w) 0)) (<= ((result w) 1) (len src))))))
```

A `fact` never quantifies over a table this way. A `fact` states a law of a FUNCTION on all arguments (§7.1), and a
table's contents are not a law. The one exception worth naming is a law of `lang` whose result is a table:
`keys : Map K V → Array K` is ascending by definition (maps.md §7). That is **sortedness**, two quantified indices,
**F-D₂**.

---

## 7. What is refused, reserved, and why

| shape | status | reason |
|---|---|---|
| index guard other than the domain, `lo ≤ s < hi` | **reserved, F-D₂** | in BMS and decidable; no demand. Its first reader would be *"every slot below `w` is written"* |
| two quantified indices: sortedness, `keys` ascending | **reserved, F-D₂** | in BMS (§2.2). maps.md's `keys` is the natural first instance |
| two tables pointwise, `∀s. a[s] ≤ b[s]` | **reserved, F-D₂** | in BMS; no demand |
| `s` in the value constraint: `a[s] = s`, `a[s] ≤ s` | **reserved, F-D₂** | in the flat fragment with index arithmetic; `iota`'s law, no reader |
| successor: `∀s. a[s] ≤ a[s+1]` | **refused** | undecidable (BMS 2006 §5); write it with two indices instead |
| nested reads `a[a[s]]` | **refused** | undecidable (BMS 2006) |
| permutation and multiset equality | **refused** | needs counting; outside every fragment cited |
| existential, `∃s. a[s] = x` | **refused** | a search, and the reason F-F is refused |
| a modular index guard in SOURCE | **refused**, with a message pointing at `(tuple …)` | §5.3: the product states it guard-free |
| a table whose element is a buffer | **cannot arise** | ADR 0020 rule 6 |

The refusal messages follow spec/theories.md §10: each names the construct, the fragment it would belong to, and the
rewrite when one exists.

---

## 8. Recommendation

### 8.1 Build F-D₁, derived first, declared second

**Derived** (Theorems S, S′ and Houdini over §5.1's templates) covers every row of §5.2 with no source change except
deleting clamps. **Declared** (§6) is the same checker with the candidate given instead of guessed. Build it second,
when a host API asks.

The implementation is three pieces already standing:

- **inductive-2026-09-16's invariant loop**, generalised from `0 ≤ v` to templates over threaded buffers;
- **F-B's instantiation**, with a table read as trigger (Theorem D);
- **the dry refiner** that walks a body for the facts at each store and back edge.

### 8.2 Acceptance, written before the build

1. **Every clamp in §1.2 deleted.** The four programs print what they print today on every file and host they are
   tested with, and each still reports 100% of integer operations bounded. A clamp that cannot be deleted is named
   with the candidate that failed.
2. **`tree.oro`'s walk proves 424 of 424**, up from 388.
3. **Witnesses, each failing against the build:**
   - a store of `nn + 1` where `nn` is guarded, refused;
   - a zero-filled table with `1 ≤ v`, refused;
   - freq's two buffers with ONE planted out-of-range store, refused for both;
   - a frame variable that DECREASES under `v ≤ x`, refused;
   - a store at a non-literal strided index, requiring every component's constraint.
4. **The containment harness** gains buffer programs with a derived F-D₁ fact, checked concretely on every read.
   That is γ-soundness for contents, as containment-2026-08-27 did for element ranges.
5. **A benchmark**: tree.oro against itself with and without the six `nslc` clamps, on Go and the JVM, where 1.35×
   was measured.

### 8.3 Before it, cheaper, and found by this research

- **Purification of nested conditionals** (§1.1): 110 of tree's and 12 of freq's propagated obligations. Name a
  conditional nested inside linear arithmetic with a fresh variable carrying its join, then read the skeleton. This is
  joinindex's proof attempt made compositional.
- **A loop-result bound** (§1.1): `nw ≤ len src`, count ≤ trips. inductive-2026-09-16's check over a DIFFERENCE
  template `n − i ≤ 0`, plus the exit condition. It is the first genuine octagon-shaped demand in the corpus, and it
  arrives as a single template in a finite Houdini set, not as a domain. maxlen-2026-08-28's trigger for reopening
  octagons was *"a bounded difference of two unbounded quantities"*, and this is one.

Both are F-B-sized and independent of F-D. Doing them first means the F-D build is measured against a corpus whose
notes no longer mix the three problems — which is how §1.1's misclassification happened.

### 8.4 What would refute this

- **A clamp in §1.2 whose invariant is F-D₁ and not inductive over the templates**, so that the template set, rather
  than the fragment, is the limit. The remedy would then be declaration, and the claim *"derived covers the corpus"* is
  wrong.
- **A store chain that is not a chain**, meaning linearity admits two live versions somewhere. Theorem S would then
  be unsound, and the containment harness is the check that can find it.
