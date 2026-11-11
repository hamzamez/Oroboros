# When to reduce, and when to call

Date: 2026-11-10. **Decided** (hamza, 2026-11-11, Q1–Q4 as recommended):
[ADR 0102](decisions/0102-factoring-never-changes-an-answer-or-a-verdict.md), which built P4 for
ADR 0101's instances. **M-S1, M-S2 and M-S4 measured** (factoring-2026-11-11): P8's trigger is not
met. Asked by hamza after ADR 0101: "when to reduce and inline everything, and when not? and should we
reduce everything down, and then turn every repeated pattern into a function up?"

**The answer in one paragraph.** Whether a call is unfolded never changes what a program means
(Theorem 1). It changes only four things: what the decision can prove, the code's size, the time to
compile it, and the time to run it. The static level must be unfolded, because the dynamic level has no
values for it. The dynamic level is a free choice, so the choice must be made by proofs and costs,
never by habit. Interval analysis is not distributive, so a function decided once under one contract
proves strictly less than its inlined copies (Theorem 3). Most of that loss comes back if the function
hands its unproven obligations to its callers as preconditions over its parameters (Theorem 4). That is
ADR 0037, obligations in the residual, at a function's boundary. "Reduce everything, then fold every
repeated pattern back into a function" splits in two:
- **folding by pattern** is the smallest-grammar problem, which is NP-hard, and it leaves the cost
  that was measured;
- **folding by origin** (merging the instances of one definition after the decision) is linear and
  exact. It keeps inlining's proofs, but not its compile time.

The recommendation (§8): unfold the static level; keep a dynamic definition as a function when it is
proven under its contract plus its lifted preconditions and keeping it pays; otherwise inline it. A
choice of what to unfold may never change a program's verdict (Q1, a principle in the place ADR 0009
holds for staging).

## 1. What the question is, algebraically

A program is a term over Σ_L with a definition environment Δ, a set of equations f = λx̄. e
([abstract.md](spec/abstract.md)). The reducer has two kinds of step: β applies an abstraction and δ
unfolds a defined name. [the-atom.md](the-atom.md) already makes the normal form a parameter: a target
partitions names into primitive and defined, and reduction runs until only primitives remain.

The question adds **a second partition, of the program's own definitions**. Let K ⊆ dom Δ be the
definitions **kept**. NF_K(e) is the normal form under β and under δ restricted to dom Δ ∖ K, with each
kept f reduced once in its own body.
- **Today K = ∅** for a program's own definitions: "Reduction inlines every non-exported call"
  ([ADR 0028](decisions/0028-a-definitions-contract-is-checked-at-its-calls.md)).
- **ADR 0101 put seven of K(ℕ\*)'s instances in K**: `add`, `sub`, `mul`, `mul-small`, `div`, `rem`
  and `render`.

This is the logician's **extension by definitions** (Shoenfield 1967, §4.6):
- adding a symbol f with the axiom ∀x̄. f(x̄) = e is conservative;
- the elimination theorem rewrites every formula that mentions f into one that does not, and that
  rewriting is δ;
- NF_∅ eliminates every defined symbol, and NF_K keeps the symbols in K.

A kept f is therefore **a derived operation promoted to a symbol of the residual's signature**.
`ir/progfunc.go` does exactly this: `registerProgramFunc` writes the callee into `tg.Prims`. Its meaning
is still its definition, and its specification is its contract: the parameters' sets, its `where` and
its `ensure`. In ADR 0089's terms it is a realization ι(f) = `call @f`, whose body is ι of f's body,
compiled once. So keeping a definition does not leave the atom's framework. It **extends the target by
the program's own theory**, and the target's primitives are no longer only the host's.

## 2. Meaning is invariant

**Theorem 1 (the choice of K is not observable).** Let K and K′ be two sets of kept definitions. Then
⟦NF_K(e)⟧ = ⟦NF_K′(e)⟧ = ⟦e⟧, where a kept name is interpreted by its definition.

*Proof.* δ is the defining axiom read left to right, so it is sound in every model of Δ. β is sound
under ADR 0010's side condition: an impure argument is let-bound and never substituted. Under call by
need (ADR 0071; Moran and Sands 1999) substitution is an improvement, never a change of value. Calling
a closed λ and β-reducing it into the site are the same application. Hence every NF_K is a sequence of
sound equations away from e. ∎

So K decides nothing about outputs. It decides:
- **(P) proofs**, what the decision can prove;
- **(S) size**, the emitted code;
- **(C) compile time**, the decision's and the host compiler's;
- **(R) run time**, call overhead against the site's specialization.

The rest of this document is about these four, and **(P) is the one particular to this language**. A
compiler for C or Haskell asks only S, C and R, because nothing it accepts depends on where it inlined.
Here the verdict does: the decision refuses an operation it cannot prove inside its word (ADR 0019), and
what it can prove depends on the facts present where the operation is decided.

## 3. The static level is always unfolded: binding times

The two levels ([closures-direction.md](closures-direction.md)) answer half the question before any
measurement:
- the static level is unrestricted and higher-order, and staging erases it;
- the dynamic level has only first-order tables and loops;
- a function value, a set or a type has **no representation at the dynamic level**.

So every application whose operator or argument is static must be reduced. In partial evaluation's
words, the static level is the part a binding-time analysis marks S (Jones, Gomard and Sestoft 1993,
ch. 4–5; Futamura 1971). Its calls are unfolded or specialized, never residualized as calls.

**Specialization is polyvariant and memoized.** A definition applied to different static values gets
one instance per value, and an instance already made is reused. These are Bulyonkov's specialization
points (Bulyonkov 1984; Jones et al. §4.4.2). The memo tables already exist:
- `emit.LimbFunc(n, lim)` and `emit.NatFunc(ml)` key the library's instances by their static
  arguments;
- `natCallee`'s cache keys each callee by target and name.

**Termination of specialization is free.** Partial evaluation's hardest problem is infinite
specialization: unfolding a recursive call whose static arguments never repeat. Supercompilation solves
it by generalization (Turchin 1986; Sørensen, Glück and Jones 1996). This language has no recursion
(ADR 0014), so the call graph is a DAG, and a DAG reaches finitely many static configurations. No
generalization is needed. The open question is therefore only the **dynamic** level: a definition whose
parameters are all dynamic, which may be inlined or called.

**Corrected by M-S2** (factoring-2026-11-11): a definition is dynamic only if its **whole type** is
first-order, its result included. One that returns a table rule, or a sum in its continuation-passing
encoding, returns a λ, which is a static value. Kept, it would allocate what inlining fuses into its
consumer, a hidden allocation (`data` in three differential cases; `os.ReadAt`).

## 4. Size: why inlining everything cannot be the rule

Let G be the static call graph, a DAG. For a definition f, write |f| for the size of its body without
its calls, and π(f) for the number of paths from the root to f in G, counted with each syntactic call's
multiplicity.

**Proposition 2 (size).** |NF_∅| = Σ_f |f| · π(f). If every definition is kept,
|NF_dom Δ| = Σ_f |f| · s(f), where s(f) is f's number of static instances.

π(f) is exponential in the depth of G. Take f₀(x) = x + 1 and f_{i+1}(x) = f_i(f_i(x)). Inlined, f_d
is 2^d additions; kept, it is 2d calls. Each inlined addition is also an obligation the decision
proves on its own.

The measured cases:
- the 2,000-quotient program was 21,102 operations, and 29,579 after Knuth's A and S were split;
- clang took about ten minutes on it (addsub-2026-11-10);
- with ADR 0101's seven functions its `main` is 2,762 operations, it compiles in 19.5 s, and the binary
  is 73 KB instead of 483 KB (progfunc-2026-11-10).

The cost of compiling is superlinear in size, for two reasons:
- **our decision** runs a fixed point per loop, and the refinement layer uses Fourier–Motzkin, whose
  worst case is doubly exponential in the variables it eliminates;
- **the host's compiler** has superlinear passes on one large function (register allocation, SROA,
  scheduling).

The measurement that is still owed: **how the ten minutes divided between our decision and clang**
(M-S4). It decides which of §7's options is enough.

## 5. Proofs: what a function boundary costs

This section carries the argument.

### 5.1 The decision as an abstract interpretation

`ir.Decide` is an abstract interpretation (Cousot and Cousot 1977) over intervals with thresholds,
trip bounds and a relational difference. Write F_f : A → A for the abstract transformer of f's body on
the domain A of facts about its parameters. Each obligation o in f, such as "this `+` is inside S₆₄",
is a downward-closed predicate on facts: if it holds at a, it holds at every a′ ⊑ a.

There are two ways to decide a call f(ā) at a site whose facts about ā are a:
- **inlined**, decide o at F_f(a): the site's own facts. This is full context sensitivity, Sharir and
  Pnueli's functional approach with an exact summary per context (Sharir and Pnueli 1981). For a
  language without recursion it is finite; Astrée makes the same choice for C without recursion
  (Blanchet et al. 2003);
- **as a function under a contract C**, decide o once at F_f(C), and require a ⊑ C at every call:
  `CheckCallSets`, ADR 0028's obligation at a call. The caller then reads the result only through the
  declared result set R, a claim the body proves (ADR 0072, `proveResultElems`).

### 5.2 The function is never stronger than the site

**Theorem 2 (adaptation).** Suppose F_f is monotone. If a ⊑ C and o holds at F_f(C), then o holds at
F_f(a).

*Proof.* F_f(a) ⊑ F_f(C) by monotonicity, and o is downward-closed. ∎

This is Hoare's rule for procedures (Hoare 1971): a body proven under a precondition is correct at
every call that establishes it. The consequences:
- **a function proven under its contract is proven at every call that satisfies the contract**, with no
  re-decision. That is what makes ADR 0101 sound;
- **inlining proves at least as much as the function**, and strictly more whenever a ⊏ C and some
  obligation fails at F_f(C) but holds at F_f(a).

**A caveat the implementation owes a measurement for.** Theorem 2 holds for the collecting semantics
and for any monotone domain. **Widening is not monotone** (Cousot and Cousot 1992): a smaller input can
reach a larger widened fixed point. `ir/interval.go` widens with each loop's own thresholds: its
constants, the ends of what it reads from outside, and its guards' images. "The ends of what it reads
from outside" are the site's facts, so an inlined loop and the same loop decided under C widen to
different thresholds. In principle an inlined copy could fail where the function is proven. Neither
case has been seen. M-S2 looks for it, because it decides whether "inline on failure" (§8, P4) is
always at least as accepting.

### 5.3 One contract cannot serve all sites

The natural contract for a function called at sites a₁, …, a_k is their join, C = ⊔ a_i. It is the
least contract that every call satisfies.

**Theorem 3 (the join is not a homomorphism).** For interval arithmetic,
F(a ⊔ b) ⊒ F(a) ⊔ F(b), and the inequality can be strict in a way that changes a verdict.

*Proof by example.*

```
fn scale(t ∈ Int, k ∈ Int) (Int) { t * k }
```

| facts | t | k | t · k |
|---|---|---|---|
| site A | [0, 2³²) | [0, 2²⁸) | [0, 2⁶⁰), inside S₆₄ |
| site B | [0, 2⁶⁰) | [0, 3] | [0, 3·2⁶⁰), inside S₆₄ |
| the join | [0, 2⁶⁰) | [0, 2²⁸) | [0, 2⁸⁸), outside |

Inlined, both sites are proven. Decided once under the join, `scale` is refused, or on a target with a
rung above the word it is promoted to it. ∎

This is Kam and Ullman's result in our terms (Kildall 1973; Kam and Ullman 1977): the meet over all
paths equals the maximal fixed point only for a distributive framework. When the framework is
distributive, as IFDS's are (Reps, Horwitz and Sagiv 1995), a summary is exact and nothing is lost at a
boundary. **Intervals are not distributive over multiplication, or over any operation whose bound
depends on two operands jointly.** The loss is structural: no implementation work removes it.

**Corollary.** A contract inferred as the join of its sites can refuse a program that inlining accepts,
so **it cannot be the default**. A declared contract is the program's own claim (ADR 0028); an inferred
one must be at least as precise as each site, and the join is not.

### 5.4 What recovers the sites' precision: lifted preconditions

In `scale`, the obligation that fails at F(C) is t · k ∈ S₆₄. It is a formula over the parameters. So
instead of deciding it in the body, **the function can require it of its callers**, as if `scale` were
written:

```
fn scale(t ∈ Int, k ∈ Int) (Int) where t * k ∈ S64 { t * k }
```

Each site then decides `where t * k ∈ S64` with its own facts and proves it: A and B as before. The
body is decided once, under its sets and its `where`. The operation is proven, because its obligation
is now an assumption.

**Theorem 4 (lifting is exact for parameter-expressible obligations).** Let o be an obligation in f's
body, and let wp(f, o) be its weakest precondition over f's parameters (Dijkstra 1975). A site with facts
a proves o in the inlined copy iff a ⊨ wp(f, o), where the site decides wp(f, o) by the same procedure
it would use on the inlined copy. Hence a function whose contract is C ∧ ⋀_o wp(f, o) is accepted at
exactly the sites where inlining is accepted.

*Proof.* The inlined copy's obligation is o with each of f's variables replaced by its value as a
function of the arguments. That substituted formula is wp(f, o) by definition. The site decides one
iff it decides the other: the same formula, the same facts, the same procedure. ∎

The limits are what an implementation would meet:
- **wp is computable only when it is expressible.** For straight-line code it is substitution. For a
  loop it needs the loop's invariant as a formula over the parameters: the index bounded by `len a`, a
  carry in {0, 1}. That is invariant synthesis, the facts' job ([facts.md](facts.md): Houdini
  invariants, trip bounds). Whatever the decision cannot state is not lifted. Such an obligation is
  either proven under C or makes the function unsuitable (P4);
- **soundness needs a sufficient precondition**, not a necessary one. Cousot, Cousot, Fähndrich and
  Logozzo (2013) infer necessary preconditions, which are right for warnings and wrong for refusing.
  What is lifted must imply o. An approximation is stronger than wp, so it proves less at the sites,
  never something false;
- **the lifted formula must be in the fragment the site decides**: the intervals, then the refinement
  layer's linear fragment. t · k is not linear, but at the site t and k are bounded, so the intervals
  decide it. This is the same route an inlined `t * k` takes, as Theorem 4 needs.

**This is not new machinery.** It is ADR 0037 ("a program's obligations are in its residual, decided by
one procedure") with the residual's granularity moved from the program to the function. ADR 0028
already states the rule for an export: its contract is assumed in its body and checked at internal
calls. A kept function is an internal export whose contract is completed by lifting.

### 5.5 What the result loses, and `ensure`

A call's result is known only by its declared set. Inlined, the site knows the body's exact relation to
the arguments. ADR 0101 met this already: `div-small`'s quotient digit is below B because its partial
remainder is below the divisor, a relation that no set states. Its result claim refused it, correctly,
and it stayed inlined.

The remedy is a **relational summary**: an `ensure ψ(x̄, r)`, a formula over the parameters and the
result that the body proves once and every site assumes. For `div-small`, ψ is q·d + r = a ∧ 0 ≤ r < d.
This is the functional approach's summary as a relation (Sharir and Pnueli 1981), and the
postconditions of modular verifiers (Hoare 1971; Leino 2010, Dafny). ADR 0072 decides a declared result
at its definition; `ensure` on a kept function is the same decision with a relation in place of a set.
Liquid types (Rondon, Kawaguchi and Jhala 2008) infer such refinements from a set of qualifiers. Their
inference is a join over the sites, so Theorem 3 bounds what it can find. Theirs is relational, which
recovers much of it.

### 5.6 What a boundary gains: facts only a boundary can state

The boundary is not only a loss:
- **`noalias` on a parameter** is LLVM's call-scoped form, sound by the frame rule (arguments
  borrowed, the result a birth). Its scoped form for inlined code is unsound for us (lenonce-2026-11-10).
  A function lets us tell the host something that inlined code cannot. This is ADR 0101's step 3;
- **the host decides physical inlining with its own cost model** (§6.2), which knows its machine.

Two things inlining gives that a boundary takes away:
- **representation by place** (ADR 0079, 0083): inlined, each site's facts choose ρ. A function's
  parameter has one ρ, chosen under C. A site whose facts would choose V_n is handed K(ℕ\*), and
  natdiv-2026-11-10 measured V₉ at 4.4× faster than K(ℕ\*) on bounded additions. The remedy is a clone
  per ρ-vector (§7). This is how representation-polymorphic code is compiled: one instance per
  representation (Leroy 1992; Eisenberg and Peyton Jones 2017);
- **reuse across the call** (ADR 0096, 0100): inlined, a dead table's token reaches the next
  allocation. Across a boundary it needs destination-passing, the token a linear argument (Minamide
  1998; Shaikhha et al. 2017; Lorenzen, Leijen and Swierstra 2023). This is ADR 0101's step 2.

## 6. Run time: whose decision physical inlining is

### 6.1 The classical cost model

Scheifler (1977) cast inlining as a knapsack problem: each call has a benefit and a size, and the
compiler maximizes benefit within a budget. The problem is NP-hard, so every production inliner is
greedy, with benefit predicted as Ball (1979) began to predict it:
- GHC counts occurrences: once, once inside a λ, many. It weighs work duplicated against code
  duplicated (Peyton Jones and Marlow 2002);
- Chez Scheme's cp0 inlines online, within effort and size counters (Waddell and Dybvig 1997);
- MLton contifies: a function always called with the same continuation becomes a block, not a
  function (Fluet and Weeks 2001; join points, Maurer et al. 2017);
- LLVM, HotSpot and V8 use size thresholds with profile-weighted bonuses. Learned policies replace the
  hand-tuned one (Trofin et al. 2021, MLGO).

Two of these are already in this language:
- **ADR 0071's {0, 1, ω}** is GHC's occurrence analysis for β. A definition used once has |f| · (π − 1)
  = 0, so inlining it costs no size;
- **contification** is a loop's form: a loop is a join point, `again` a jump.

### 6.2 Most of the classical benefit is already taken

In a functional language, inlining pays mostly by exposing a known function to β: a closure removed, an
allocation avoided. Here staging has already done that (§3). After staging, the dynamic level is
first-order and monomorphic, which is the C case. Two benefits are left:
- **(i)** the site's facts: a dead branch, a check proven, a representation chosen;
- **(ii)** what the host's optimizer does across the boundary.

Of these, (i) is ours, and (ii) the host does better. Go, the JVM, V8 and LLVM all inline, each with a
model of its own machine. **The parasite model decides the division** (ADR 0001, "emit at the highest
layer the target natively provides"):
- every host has functions, so a kept definition is emitted as the host's function;
- the physical decision is left to the host, measured (ADR 0008);
- the proof question stays ours, because the host cannot decide our obligations.

The caveat is that hosts' budgets are small: Go's inliner gives up at a cost of 80 nodes, and HotSpot
inlines a hot method only below 325 bytecodes. A kept function that the host leaves out of line may
cost a call where inlining paid. That is a measurement (M-S3). LLVM takes `inlinehint` and
`alwaysinline`; Go, the JVM and V8 take no hint.

Measured so far: the seven K(ℕ\*) calls cost nothing measurable on windows (fib 139 ms either way), and
D ran 10% faster as functions, plausibly because its code fits the caches (progfunc-2026-11-10).

## 7. Reduce everything, then fold back up?

hamza's second question has a precise form. Given NF_∅, find a set of functions and a factorization of
the residual into calls of them. There are two ways to look for the factors.

### 7.1 Folding by pattern

A repeated pattern with holes is a λ over the holes. The most specific pattern common to two terms is
their **anti-unifier**: their least general generalization in the subsumption lattice of terms (Plotkin
1970; Reynolds 1970). Supercompilation folds this way, on its process tree (Turchin 1986). Code
compaction does it on machine code: Fraser, Myers and Wendt (1984) with suffix trees, Debray et al.
(2000), LLVM's MachineOutliner, and function merging by sequence alignment (Rocha et al. 2019, 2020).

As an optimization problem, "turn every repeated pattern into a function" so that the result is
smallest is **the smallest grammar problem**: the smallest straight-line grammar generating a given
string or tree. It is NP-hard, approximable within O(log(n/g)), and not approximable below a constant
factor greater than 1 unless P = NP (Charikar et al. 2005). For trees, TreeRePair is the practical
heuristic (Lohrey, Maneth and Mennicke 2013).

Only one special case is easy. **Sharing identical subterms**, the minimal DAG, is computed exactly in
linear time by hash-consing (Downey, Sethi and Tarjan 1980). For whole functions this is identical code
folding (Tallam et al. 2010).

### 7.2 Folding by origin

The compiler does not need to discover patterns: **it knows where each piece of the residual came
from.** Every inlined copy of f is an instance of f's lowered body. After the decision, each copy is a
refinement of that body in four ways:
- (a) its representations, ρ per value;
- (b) the flags its proofs justify (`nsw`, `nuw`, `inbounds`, a check removed);
- (c) the branches its facts proved dead;
- (d) the constants its arguments propagated.

**Proposition 5 (merging instances of one definition).** Let c₁, …, c_k be the decided instances of f
in a program, all with the same ρ. Define their merge m from f's body:
- with flag set ⋂ flags(c_i), the meet: a flag stays only if every instance proves it;
- with every branch some instance keeps;
- with each constant that came from an argument abstracted back to its parameter.

Calling m at each site is equivalent to the inlined program, and every flag on m is justified.

*Proof.*
- **Equivalence** is Theorem 1: m is f's body, possibly with dead branches put back, and a dead branch
  changes no execution.
- **The flags**: each flag in m is proven at every instance, so it is true on every execution of m in
  this program.
- **The obligations**: each obligation inside a re-included branch was proven where that branch was
  live, and is vacuous where it was dead, because no execution there reaches it. ∎

This is sound only in a closed world: m is justified for this program's calls and no others. That is
the world an entry point's residual already lives in.

The merge is linear in the size of the instances, needs no search and is exact. **It keeps inlining's
verdicts, by construction.** What it cannot remove is our decision's cost, because the decision still
runs per instance. Astrée makes the same trade (§5.1): it analyses each call in its context and still
emits one copy of the code. Instances with different ρ do not merge; they are the clones per ρ-vector
(Cooper, Hall and Kennedy 1993).

Its own cost is construction: the residual would have to carry each fragment's origin through β and δ.
GHC carries that kind of note for profiling.

### 7.3 The verdict on "reduce everything, then fold"

| | folding by pattern | folding by origin |
|---|---|---|
| finds | any repetition | repetition of one definition |
| complexity | NP-hard (smallest grammar) | linear |
| our decision's cost | unchanged (runs on NF_∅) | unchanged (runs per instance) |
| the host's compile time | reduced | reduced |
| proofs | kept (decided before folding) | kept |
| against the host's inliner | it may inline the pattern back | the same |

Neither form reduces the decision's own cost, because the decision runs before the folding. Folding by
pattern adds an NP-hard search, to find repetitions that the source already names as definitions.
**Folding by origin is the useful half**, and it is §8's fallback, not the primary rule. Folding
unrelated code by pattern is not recommended. Identical code folding of whole functions is: it is
linear and exact (P6).

## 8. The policy, derived

All the options form a two-dimensional space:
- **analysis granularity**: decided per site, per class of sites, or once per contract;
- **code granularity**: emitted per site, per class, or once.

Code never needs to be finer than its analysis, and code coarser than its analysis is Proposition 5's
merge.

| | analysis | code | proofs | our decision | host compile | |
|---|---|---|---|---|---|---|
| inline | per site | per site | the sites' | Σ\|f\|·π(f) | the same | today, for a program's definitions |
| merge by origin | per site | one per ρ | the sites' | Σ\|f\|·π(f) | Σ\|f\|·s(f) | §7.2 |
| clone by class | per class | per class | the classes' joins | per class | per class | Cooper et al. |
| contract and lifted wp | once | one per ρ | the sites' where wp is expressible (Thm 4) | Σ\|f\|·s(f), plus the sites' wps | Σ\|f\|·s(f) | §5.4 |
| contract only | once | one | C's | Σ\|f\|·s(f) | Σ\|f\|·s(f) | ADR 0101 today |

The recommendation, in order:
- **P1. The static level is always unfolded**, specialized per static value and memoized (§3). This is
  already built and unchanged.
- **P2. A dynamic definition is kept iff it is proven, and keeping it pays.**
  - *Proven* means under its contract: its declared sets and `where`, completed by its lifted
    sufficient preconditions (P3).
  - *Pays* means |f| · (π(f) − 1) is above a threshold that a measurement sets (M-S1, M-S4), or a
    measured compile time names it.
  - Every other dynamic definition is inlined. One with π(f) ≤ 1 always is: it duplicates nothing,
    which is ADR 0071's count extended from work to code.
- **P3. A kept function's contract** has three parts:
  - its declared sets and `where`, which are the program's claim (ADR 0028);
  - its unproven obligations lifted to its callers as sufficient preconditions over its parameters,
    decided at each site by the same procedure (Theorem 4);
  - its result read through its declared set, or through its `ensure` relation (§5.5).
  
  Never the join of its sites (Theorem 3).
- **P4. A choice of K never changes a verdict.** A definition is kept only if every program that
  inlining accepts is accepted with it kept. When P3's contract does not prove a definition, it is
  inlined, or merged by origin (Proposition 5). **The program is never refused because of a factoring
  that is invisible to its meaning** (Q1). Today, a program whose call fails ADR 0101's call obligation
  is refused. K(ℕ\*)'s numerals satisfy that obligation by construction, so no program has met it,
  but under P4 such a call would be inlined instead.
- **P5. Clone per ρ-vector**, one instance per tuple of argument representations, when a measurement
  names the loss. V_n against K(ℕ\*) is the known case (natdiv-2026-11-10).
- **P6. No folding by pattern after the decision.** Identical code folding of whole functions is
  allowed, when a binary's size is measured to need it.
- **P7. Physical inlining is the host's.** A kept definition is emitted as the host's function and the
  host's inliner decides, measured (ADR 0008), with LLVM's hints only where measured.
- **P8. A program's own definitions are brought into K on a trigger**: a program whose compile time or
  size is measured large from repetition, after M-S1 and M-S2 have said how much the policy keeps and
  how much it loses. Until then ADR 0101's seven instances are the only ones kept.

### 8.1 Why not

- **Inline everything, always** (the status quo before ADR 0101): size is Σ|f|·π(f), exponential in
  the call graph's depth (Proposition 2), and it measured ten minutes in clang.
- **Reduce everything, then fold by pattern**: an NP-hard search for what the source already names; it
  leaves the decision's cost (§7.3).
- **Never inline the program's definitions**, separate compilation as in C: decided once under its
  contract, a definition loses the sites' facts (Theorem 3). A choice invisible to meaning would then
  refuse programs, contrary to P4.
- **A contract inferred as the join of the sites**: Theorem 3's `scale`.
- **Emit everything as functions and let the host inline**: the host cannot decide our obligations, so
  every definition would be proven by its contract alone. That is the previous option.
- **A learned inlining policy** (MLGO): a policy that cannot be derived or explained, against ADR 0008's
  "measure" and this project's derive-first rule. It also answers only R, never P.
- **Polyvariant analysis without a code decision**, inlining everything for the decision and letting
  the host fold: hosts do not merge functions that differ in flags, so the size stays.

## 9. Measurements owed, each with a time limit

- **M-S1, duplication** (2 hours): for every program in `examples/` and the gauntlet, each dynamic
  definition's |f|, π(f) and s(f), giving the residual's size at K = ∅ and at P2's K. This says how much
  repetition there is to save. The prediction, to be checked: little outside the libraries, since the
  largest program is 194 lines.
- **M-S2, proofs at a boundary** (3 hours):
  - for each of the program's definitions with declared sets: is it proven under its contract, and
    how many of its unproven obligations does lifting recover (Theorem 4);
  - for each one lifting cannot recover, why (a loop invariant not stated, a non-linear relation);
  - a search for widening's non-monotonicity (§5.2): an obligation proven under C and refused inlined.
- **M-S3, run time** (1 day): the gauntlet's seven at K = ∅ and at P2's K, on four targets, against the
  ~15% noise floor. It answers P7: whether the hosts' inliners recover what inlining gave.
- **M-S4, compile time** (with M-S3): the decision's and the host's time, separately. First on the
  2,000-quotient program at K = ∅, to split its ten minutes.

## 10. Questions for hamza

- **Q1. Is P4 a principle**, a sibling of ADR 0009 ("staging never changes an answer"): "factoring never
  changes an answer or a verdict"? *Recommended: yes, as an ADR.* Theorem 1 says the choice is
  invisible to meaning, so letting it change acceptance makes the verdict depend on a compiler
  heuristic.
- **Q2. When do a program's own definitions enter K?** *Recommended: M-S1 and M-S2 now (five hours),
  building on P8's trigger.* The alternative is to build P2 and P3 now.
- **Q3. A kept function's contract: declared only, or completed by lifted preconditions?**
  *Recommended: lifted (P3).* It is ADR 0037 at a function's boundary, and it recovers inlining's proofs
  wherever wp is expressible (Theorem 4). Declared-only is simpler, and refuses or inlines more.
- **Q4. Folding: no folding by pattern, merge by origin as a fallback, identical code folding on a
  trigger?** *Recommended: yes (P6, §7.3).*

## References

- Ball, J. E. 1979. Predicting the effects of optimization on a procedure body. *SIGPLAN Symposium on
  Compiler Construction*.
- Blanchet, B., Cousot, P., Cousot, R., Feret, J., Mauborgne, L., Miné, A., Monniaux, D., Rival, X. 2003.
  A static analyzer for large safety-critical software. *PLDI*.
- Bulyonkov, M. A. 1984. Polyvariant mixed computation for analyzer programs. *Acta Informatica* 21.
- Charikar, M., Lehman, E., Liu, D., Panigrahy, R., Prabhakaran, M., Sahai, A., Shelat, A. 2005. The
  smallest grammar problem. *IEEE Transactions on Information Theory* 51(7).
- Cooper, K. D., Hall, M. W., Kennedy, K. 1993. A methodology for procedure cloning. *Computer
  Languages* 19(2).
- Cousot, P., Cousot, R. 1977. Abstract interpretation: a unified lattice model for static analysis of
  programs by construction or approximation of fixpoints. *POPL*.
- Cousot, P., Cousot, R. 1992. Comparing the Galois connection and widening/narrowing approaches to
  abstract interpretation. *PLILP*.
- Cousot, P., Cousot, R., Fähndrich, M., Logozzo, F. 2013. Automatic inference of necessary
  preconditions. *VMCAI*.
- Debray, S. K., Evans, W., Muth, R., De Sutter, B. 2000. Compiler techniques for code compaction.
  *TOPLAS* 22(2).
- Dijkstra, E. W. 1975. Guarded commands, nondeterminacy and formal derivation of programs. *CACM*
  18(8).
- Downey, P. J., Sethi, R., Tarjan, R. E. 1980. Variations on the common subexpression problem. *JACM*
  27(4).
- Eisenberg, R. A., Peyton Jones, S. 2017. Levity polymorphism. *PLDI*.
- Fluet, M., Weeks, S. 2001. Contification using dominators. *ICFP*.
- Fraser, C. W., Myers, E. W., Wendt, A. L. 1984. Analyzing and compressing assembly code. *SIGPLAN
  Symposium on Compiler Construction*.
- Futamura, Y. 1971. Partial evaluation of computation process: an approach to a compiler-compiler.
  *Systems, Computers, Controls* 2(5).
- Hoare, C. A. R. 1971. Procedures and parameters: an axiomatic approach. *Symposium on Semantics of
  Algorithmic Languages*, LNM 188.
- Jones, N. D., Gomard, C. K., Sestoft, P. 1993. *Partial Evaluation and Automatic Program Generation*.
  Prentice Hall.
- Kam, J. B., Ullman, J. D. 1977. Monotone data flow analysis frameworks. *Acta Informatica* 7.
- Kildall, G. A. 1973. A unified approach to global program optimization. *POPL*.
- Leino, K. R. M. 2010. Dafny: an automatic program verifier for functional correctness. *LPAR*.
- Leroy, X. 1992. Unboxed objects and polymorphic typing. *POPL*.
- Lohrey, M., Maneth, S., Mennicke, R. 2013. XML tree structure compression using RePair. *Information
  Systems* 38(8).
- Lorenzen, A., Leijen, D., Swierstra, W. 2023. FP²: fully in-place functional programming. *ICFP*.
- Maurer, L., Downen, P., Ariola, Z. M., Peyton Jones, S. 2017. Compiling without continuations. *PLDI*.
- Minamide, Y. 1998. A functional representation of data structures with a hole. *POPL*.
- Moran, A., Sands, D. 1999. Improvement in a lazy context: an operational theory for call-by-need.
  *POPL*.
- Peyton Jones, S., Marlow, S. 2002. Secrets of the Glasgow Haskell Compiler inliner. *JFP* 12(4–5).
- Plotkin, G. D. 1970. A note on inductive generalization. *Machine Intelligence* 5.
- Reps, T., Horwitz, S., Sagiv, M. 1995. Precise interprocedural dataflow analysis via graph
  reachability. *POPL*.
- Reynolds, J. C. 1970. Transformational systems and the algebraic structure of atomic formulas.
  *Machine Intelligence* 5.
- Rocha, R. C. O., Petoumenos, P., Wang, Z., Cole, M., Leather, H. 2019. Function merging by sequence
  alignment. *CGO*. And 2020, Effective function merging in the SSA form. *PLDI*.
- Rondon, P. M., Kawaguchi, M., Jhala, R. 2008. Liquid types. *PLDI*.
- Scheifler, R. W. 1977. An analysis of inline substitution for a structured programming language.
  *CACM* 20(9).
- Shaikhha, A., Fitzgibbon, A., Peyton Jones, S., Vytiniotis, D. 2017. Destination-passing style for
  efficient memory management. *FHPC*.
- Sharir, M., Pnueli, A. 1981. Two approaches to interprocedural data flow analysis. In Muchnick and
  Jones (eds.), *Program Flow Analysis: Theory and Applications*. Prentice Hall.
- Shoenfield, J. R. 1967. *Mathematical Logic*. Addison-Wesley. §4.6, extensions by definitions.
- Sørensen, M. H., Glück, R., Jones, N. D. 1996. A positive supercompiler. *JFP* 6(6).
- Tallam, S., Coutant, C., Taylor, I. L., Li, X. D., Demetriou, C. 2010. Safe ICF: pointer safe and
  unwinding aware identical code folding in gold. *GCC Developers' Summit*.
- Trofin, M., Qian, Y., Brevdo, E., Lin, Z., Choromanski, K., Li, D. 2021. MLGO: a machine learning
  guided compiler optimizations framework. arXiv:2101.04808.
- Turchin, V. F. 1986. The concept of a supercompiler. *TOPLAS* 8(3).
- Waddell, O., Dybvig, R. K. 1997. Fast and effective procedure inlining. *SAS*.
