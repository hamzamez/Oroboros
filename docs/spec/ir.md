# The IR: structured SSA with π-parameters

Status: **specified 2026-09-25** ([ADR 0032](../decisions/0032-the-ir-is-structured-ssa.md)). **Step 1 is built**
([irstep1-2026-09-25](../../gauntlet/results/irstep1-2026-09-25.md)): lowering, typing, the verifier and
the canonical printer and reader, in `ir/`. Every program that emits is lowered and verified, and no
printer reads the IR yet. **Step 2 is built**
([irstep2-2026-09-25](../../gauntlet/results/irstep2-2026-09-25.md)): the Go backend is `ir/golang`,
printing IR_P, at parity on the gauntlet. **Step 3 is built**
([irstep3-2026-09-25](../../gauntlet/results/irstep3-2026-09-25.md)): the decisions every printer
shares are `ir/plan`, and the JavaScript backend is `ir/js`. The Java backend is `ir/java`
([irstep3java-2026-09-25](../../gauntlet/results/irstep3java-2026-09-25.md)), and the x86 backend is
`ir/x86` ([irstep3x86-2026-09-26](../../gauntlet/results/irstep3x86-2026-09-26.md)). It realizes [ADR 0006](../decisions/0006-ir-file-format.md), which decided that the
backend interface is a file format and never wrote the format. The derivation is
[docs/ir-research.md](../ir-research.md). The prototypes are in `experiments/irproto`, and their
measurements are [irp1](../../gauntlet/results/irp1-2026-09-25.md) (lowering),
[irp3](../../gauntlet/results/irp3-2026-09-25.md) (a sparse analysis) and
[irp2](../../gauntlet/results/irp2-2026-09-25.md) (a Go printer).

This document says what the IR **is**: its algebra, its syntax, which programs are well formed, what
each one means, how the residual lowers to it, what a printer may assume, and the file it is written
to. Section numbers are stable, and other documents cite them.

---

## 0. Where it sits

```
source ──read──▶ terms ──stage──▶ residual ──contracts, term rewrites──▶ residual′
                                                                    │
                                                                    L  (§8)
                                                                    ▼
                                         IR_A  ──analyses: types, facts, obligations──▶  IR_P
                                                                                         │
                                                                               printer_T (§9)
                                                                                         ▼
                                                                                    host code
```

- **Staging stays on terms**, and so do the passes that rewrite terms before lowering (`FlattenProducts`,
  `PromoteBig`, `SelectWords`). The residual that reaches L is closed, monomorphic and first-order
  (research Theorem A).
- **One format, two stages.** IR_A is what lowering produces and the analyses read. IR_P is what a
  printer reads. They share the syntax. IR_P is IR_A with two operations removed and every type final
  (§7). A printer never sees an IR_A program.
- **The IR is per (program, target).** Legality is per target (ADR 0026), so the residual is too, and
  so is its IR. Portability is computed by lowering once per target; the IR does not abstract over
  targets.

---

## 1. The algebra

### 1.1 The category

The IR is a presentation of a **distributive Freyd category with Elgot iteration** (research §1.2):

- **Objects** are finite products of the types of §4. A product of one type is that type, and the
  empty product is 1.
- **Morphisms** are the IR's programs from their inputs to their outputs.
- **Composition** is sequencing within a region. The identity is the empty sequence.
- **Products.** An operation with several results, a region with several parameters, and a
  terminator with several arguments are all products. There is no tuple *value* in the IR. A tuple is
  a list of values, and ADR 0031's scope results and ADR 0027's host results are this.
- **Coproducts.** `bool = 1 + 1`. A branch is the copairing `[f, g]` composed with the distributivity
  isomorphism X × (1 + 1) ≅ X + X (Cockett 1993).
- **Iteration.** A loop body is a morphism f : X → Y + X, where X is its parameters, Y is its results,
  `continue` is the right injection and `break` the left. The loop is the **Elgot iterate** f† : X → Y.
- **Effects.** Operations are **central** (pure) or not (Power and Robinson 1997). Central morphisms
  commute with every morphism, and the non-central ones keep their order. Pure plus impure is a
  Freyd category (Levy, Power and Thielecke 2003). The purity of a primitive is ADR 0010's declared
  bit.

### 1.2 The signature Σ

Every operation, with its arity (inputs → outputs), its purity, and the regions it owns. "Pure" means
central.

| operation | arity | pure | regions | denotes (§5) |
|---|---|---|---|---|
| `const k` | 0 → 1 | yes | | the literal k |
| `global g` | 0 → 1 | yes | | a module-level constant (§2.4) |
| `add` `sub` `mul` | 2 → 1 | yes | | ℤ's operation, with a mode (§4.4) |
| `neg` | 1 → 1 | yes | | ℤ's negation, with a mode |
| `div` `rem` | 2 → 1 | yes | | truncating division and its remainder (integers.md §3), with a mode |
| `eq` `ne` `lt` `le` `gt` `ge` | 2 → 1 | yes | | the order and equality on ℤ, into `bool` |
| `call p` | n → m | p's bit | | the target primitive p (§5.3) |
| `index` | 2 → 1 | yes* | | application of a table to an index (tables.md §3) |
| `len` | 1 → 1 | yes | | a table's or buffer's length |
| `array` | n → 1 | yes | | the table given by its graph (tables.md §7) |
| `map` | 2n → 1 | yes | | the map given by its graph (maps.md §3.1) |
| `read` | 2 → 2 | yes | | a map read as `option V`: a tag and a payload (maps.md) |
| `keys` | 1 → 1 | yes | | a map's keys in ascending order (maps.md §7) |
| `set` | 3 → 1 | no | | a store into a live buffer, consuming it and returning it (ADR 0018) |
| `insert` | 3 → 1 | no | | a store into a live map buffer |
| `if` | 1 → m | yes | 2 | a branch in value position, joined (§1.3) |
| `loop` | n → m | as its body | 1 | the Elgot iterate of its body |
| `build` | 1 → m | no | 1 | a scope with one fresh zero-filled buffer (ADR 0018, 0031) |
| `build-map` | 1 → m | no | 1 | a scope with one fresh empty map buffer, of a capacity |
| `tabulate` | 1 → 1 | as its body | 1 | the table `Fin n → V` given by its rule: `alloc (table n f)` |
| `the τ` | 1 → 1 | yes | | ascription: the identity on ⟦τ⟧ (IR_A only, §7) |
| `require` | 1 → 0 | yes | | an obligation: no runtime meaning (IR_A only, §7) |
| `assume` | 1 → 0 | yes | | an assumption, the dual of `require`: `if c then skip else ⊥`. An export's `where` (ADR 0028). No runtime code; an analysis may use c after it |
| `restrict` | 2 → 1 | yes | | the table t restricted to Fin n: `i ↦ t i` on i < n. Its domain condition, n ≤ len t, is an obligation (§7) |

\* A read of a **buffer** is pure in the algebra, and its order is enforced by linearity: a buffer is
read only before the store that consumes it (§3, W7). That is ADR 0018's "reads are impure so that
stores stay ordered", with the order carried by data dependence (Wadler 1990).

Two notes on the table:
- **Integer arithmetic and comparison are operations of the IR, not calls.** They are the language's
  own operators, which every target has injected (CLAUDE.md, "Anything promoted to the LANGUAGE"). So
  lowering classifies a call once, by the target's classification, and every analysis reads the
  operation. The 19 entries into the interval analysis and the classification by spelling in each
  consumer (research §0) have one replacement: lowering.
- **Float arithmetic stays a call** (`call go.f+`). It is not promoted to the language.

### 1.3 Regions and terminators

A **region** is a list of parameters, a list of π-parameters (§6), a sequence of operations, and a
**terminator**:

| terminator | meaning | allowed in |
|---|---|---|
| `yield v̄` | leave the region with v̄ | a function body, an `if` arm, a `build`'s or `tabulate`'s body |
| `break v̄` | leave the innermost loop with v̄: the left injection | a loop body |
| `continue v̄` | go round the innermost loop with v̄: the right injection | a loop body |
| `branch c R₁ R₂` | continue in R₁ if c, else in R₂. R₁ and R₂ end in terminators of the same kind | anywhere |

`branch` and `if` are the **two readings of one coproduct**. `if` composes the copairing with what
follows it, which is a join point. `branch` copies what follows into each arm, which is a tail.
Case-of-case, K ∘ [g, h] = [K ∘ g, K ∘ h], says the two mean the same, and they differ only in size.
The reducer chose between them, and the IR keeps its choice.

**There are no multi-level exits.** A `break` or `continue` leaves only the innermost loop, and no
terminator leaves any other region. This is a theorem about the residual, not a restriction added
here: `again` may not sit under an `if` in value position (ADR 0015), so a loop is left only by its own
clauses. Peterson, Kasami and Tokura (1973) need multi-level exits for *every* reducible graph. The
residual's graphs are a subclass, the one that single-level exits express. P2 printed seven programs
with no label and no `goto` (irp2 §5).

### 1.4 The laws

Every rewrite on the IR (an analysis's, an optimisation's, a printer's policy) preserves meaning,
because it is an instance of one of these equations. Each is named where it is used.

| | law | holds because |
|---|---|---|
| **L1** composition | (h ∘ g) ∘ f = h ∘ (g ∘ f), id ∘ f = f | category |
| **L2** coproduct | K ∘ [g, h] = [K ∘ g, K ∘ h] | the copairing is universal |
| **L3** branch on a known | `branch true R₁ R₂` = R₁ | the injection's β |
| **L4** fixpoint | f† = [id, f†] ∘ f | Elgot iteration |
| **L5** naturality | ((g + id) ∘ f)† = g ∘ f† | Elgot iteration (Bloom and Ésik 1993) |
| **L6** centrality | a pure operation commutes with every operation it does not depend on | Freyd category |
| **L7** weakening of the pure | a pure, total operation whose results are not read may be removed | centrality. In IR_A an operation with an undischarged obligation is not total, and removing it would remove the obligation, so L7 applies there only to operations with none. In IR_P every operation is total (§7) |
| **L8** contraction of the pure | two pure operations with equal operation and arguments have equal results | centrality and determinism |
| **L9** π | a π-parameter equals its source | §6 |
| **L10** connectives | `(if c true E)` is `c ∨ E`, and `(if c E false)` is `c ∧ E` | their definition (ADR 0017, booleans.md) |
| **L11** tabulate | `tabulate n f` = `build n`, then store `f i` at every `i < n` | tables.md: a rule table's allocation |
| **L13** restriction | `index (restrict t n) i` = `index t i`, for i < n ≤ len t | the definition of restriction. It is what bounds-check re-slicing is (§9.4) |
| **L12** η for tables | `tabulate (len t) (i ↦ t i)` = `t`, for an immutable table t | a table is a function on Fin (len t) (tables.md §3), and extensional equality. It does **not** hold for a live buffer, whose later stores the copy must not see |

**What is not a law.** Weakening and contraction of an **impure** operation (ADR 0010), and either
one for a **buffer** value (ADR 0018). No rewrite may copy, drop or reorder one.

---

## 2. Abstract syntax

### 2.1 Grammar

```
program  ::= (ir VERSION (target T) (stage A|P) (ops OPNAME…) global… func…)
global   ::= (global NAME τ const-graph)                      §2.4
func     ::= (func NAME (params PARAM…) (results τ…) region)
PARAM    ::= (%N τ)
region   ::= (region [(params PARAM…)] pi… stmt… term)
pi       ::= (pi %N τ (%M REL %K))                             a guard on a value
           | (pi %N τ ((len %M) REL %K))                       a guard on a table's length
REL      ::= eq | ne | lt | le | gt | ge
stmt     ::= (val PARAM… op)                                   one or more results
           | (do op)                                           no result
op       ::= (const LIT) | (global NAME)
           | (ARITH [MODE] %a…) | (CMP %a %b)
           | (call NAME %a…)
           | (index %t %i) | (len %t) | (array %a…) | (map (%k %v)…)
           | (read %m %k) | (keys %m) | (set %b %i %x) | (insert %m %k %x)
           | (if %c region region)
           | (loop (init %a…) region)
           | (build %n region) | (build-map %n region) | (tabulate %n region)
           | (the τ %a) | (require %c) | (assume %c) | (restrict %t %n)
term     ::= (yield %a…) | (break %a…) | (continue %a…) | (branch %c region region)
ARITH    ::= add | sub | mul | neg | div | rem
CMP      ::= eq | ne | lt | le | gt | ge
MODE     ::= exact | trap                                      §4.4; absent only in IR_A
```

`%N` is a value: a decimal id, unique in its function. `τ` is a type (§4.1), and `LIT` an integer,
float, string or `true`/`false` literal as the reader reads one.

**Every list has a head.** `(params)` and `(init)` may be empty of values, but no form is `()`. Every
node of the syntax tree names its constructor, so a reader dispatches on the head alone and an
unknown head is an error that names it. `[ … ]` marks an optional part: a region with no parameters
omits `(params …)`.

**The lexical syntax is the language's**: its atoms, literals and strings, with `;` comments as gaps
(ADR 0024). The **list structure is raw**. `core.Read` cannot read the file, because it desugars
`loop`, `let` and `build` as it reads them, so the IR's reader is the same lexer with that dispatch
off.

### 2.2 Every value is typed where it is defined

A parameter, a π and every result of a `val` carry their type. **A printer infers nothing.** P2's
275 lines of unification (irp2 §6) are the compiler's job, done once, and their result is written into
the file.

**How lowering types IR_A** (`ir/typing.go`). The residual is monomorphic and first-order, so a
value's *sort* is the **most general unifier** (Robinson) of a set of equations over the free algebra
of type constructors: `int`, `f64`, `bool`, `string` and host atoms of arity 0, `table` of arity 1 and
`map` of arity 2. Literals, signatures and each primitive's declared arguments and results seed the
equations. The algebra is taken modulo two identifications:
- **ρ_T's kernel on a declared alias.** Go's `slice-float64` is realized exactly as `(array f64)`, so
  the two are one sort. The alias is inverted over a finite candidate set, and only when the inverse
  is unique.
- **`any` is the top of the language's type relation** (types.md), not a constructor: it satisfies
  every equation and fixes nothing.

A buffer and a table are one sort, and which one a value is, is the least solution of the flow rules
of ADR 0018 and 0020. A range is written only where a definition **declares** one (a signature, a
primitive's result, an ascription). Every other integer is `int`, the target's word, until the
analyses write the final range (§7). A value no equation reaches is `any`, which IR_A allows and IR_P
does not (W9).

### 2.3 Regions own their parameters

- `loop`'s region has one parameter per loop variable, and the `loop`'s operands are their initial
  values.
- `build`'s and `build-map`'s region has one parameter, the buffer.
- `tabulate`'s region has one parameter, the index.
- An `if` arm, a `branch` arm and a function body have none. A function's parameters are the `func`'s.

### 2.4 Globals

A global is a **closed constant**: an `array` or `map` graph of literals, named at module level
because staging did not substitute it into every use. A free name that is not one is a closure, which
callbacks.md refuses before lowering.

---

## 3. Static semantics: well-formed programs

A program is well formed when all of the following hold. Lowering produces only well-formed programs,
and every analysis and printer may assume them. **A check for each is part of the implementation**:
a verifier, run after lowering and after every pass that rewrites the IR.

- **W1 (single definition).** Each `%N` is defined once in its function: by the `func`, a region's
  parameters, a π, or a `val`.
- **W2 (scope is nesting).** A value is used only after its definition in the same region, or inside a
  region nested there. Dominance is lexical, so no dominator tree is ever computed (research §3).
- **W3 (terminators).**
  - `break` and `continue` appear only in a loop body's terminator tree: the body's own terminator,
    or recursively the arms of its `branch`es. Their target is that loop.
  - `yield` appears only in a function body's or an `if`, `build`, `build-map` or `tabulate`
    region's terminator tree.
  - A region nested in an operation (`if`, `build`, `tabulate` and the rest) cannot `break` out of an
    enclosing loop (§1.3).
- **W4 (arities).** Every terminator of one region's tree passes the same number of values: its
  owner's results for `yield` and `break`, and the loop's parameters for `continue`. An operation's
  results match Σ, and a `call`'s match the primitive's declared results.
- **W5 (typing).** Every operation is applied to operands of the types its rule requires (§4.3), and
  every value flowing along an edge (an operand, `yield`, `break` or `continue` into a parameter or
  result) satisfies the flow rule. Its reading depends on the stage:
  - **in IR_A it compares sorts**. Every range is read as the representation ρ_T gives it (`int`,
    `u64`, `big`), through tables and maps, and the relation is the type checker's own (types.md).
    Whether a value lies in a declared range is an **obligation** (ADR 0028) that the analyses
    discharge. It is not a typing question: an unranged `(array int)` flowing into `os.text-of`'s
    `(array (int 0 255))` is well typed in IR_A, and its range is proven or the program is refused;
  - **in IR_P it is containment**, τ ≤ σ iff ⟦τ⟧ ⊆ ⟦σ⟧ (§4.1), covariant in elements, with equal
    element representations for tables (§4.3).
- **W6 (π).** A π's type is contained in its source's type, and its source and other operand are in
  scope. A length π's source is a table or buffer.
- **W7 (linearity).** A buffer-typed value is **consumed** (a `set`, an `insert`, a `yield` or
  `break` out of its scope, a `continue`, or a `call` declared to consume it, host-buffers.md) **at
  most once on every path**. It is read (`index`, a borrowing `call`) only before it is consumed.
  **`len` is not a use**: `set` passes its buffer's length through, so len ∘ set = len ∘ π₁ and the
  length of a consumed buffer is its successor's. The language's checker makes the same exemption
  (`emit/linearity.go`), which the Windows map library needs. `set` and `insert` take a live buffer and never a frozen table (ADR 0031).
- **W8 (effects).** Impure operations are ordered by their position in the region, and a rewrite
  keeps that order (L6 applies only to pure ones).
- **W9 (the stage).** An IR_P program contains no `the` and no `require`, every arithmetic
  operation has a mode, and every type in it is final (§7): no `any`.
- **W10 (covering).** Every operation used is in the header's `ops`, and every `call` names a
  primitive the target declares (ADR 0002).

---

## 4. Types and representation

### 4.1 The types

```
τ ::= (int LO HI) | int | f64 | bool | string | NAME
    | (array τ) | (buffer τ) | (map τ τ)
```

These are the language's types (types.md), with the range written out. `string` is Σ* (ADR 0030).
- A bare `int` abbreviates `(int LO_T HI_T)`, the target's word. It is what IR_A writes for an
  integer whose range no definition declared.
- A bare NAME is a type the target declares: a host's own type (`go.bytestring`, `slice-float64`), or
  a representation above the word (`u64`, `big`).
- `any` is written for a value no equation reached (§2.2), in IR_A only.

Inside the compiler a type is its canonical spelling (`array int 0 255`), and `TypeText` and the
reader are inverse bijections between the two spellings.

Their sets:
- ⟦(int LO HI)⟧ = { n ∈ ℤ | LO ≤ n ≤ HI };
- ⟦f64⟧ is IEEE 754 binary64;
- ⟦bool⟧ = 1 + 1;
- ⟦(array τ)⟧ = ⋃ₙ (Fin n → ⟦τ⟧), where Fin n = {0, …, n−1};
- ⟦(buffer τ)⟧ is the same set as ⟦(array τ)⟧, used linearly;
- ⟦(map κ τ)⟧ is the finite partial functions ⟦κ⟧ ⇀ ⟦τ⟧.

Every type denotes a set, and **containment is subtyping**: τ ≤ σ iff ⟦τ⟧ ⊆ ⟦σ⟧. Arrays and maps are
covariant, because they are immutable values.

### 4.2 Representation is a function of the type

A target T realizes each type with a host type, by a function

    ρ_T : types → host types

that is **data**. It is read from the target's `repr` and `int-repr` declarations and never computed
from a fact:
- ρ_T(int LO HI) is the target's word when [LO, HI] ⊆ S_T. It is U on Go when [LO, HI] ⊆ U∖S. Past
  both it is the declared big representation (ADR 0026, 0029).
- Inside a table, ρ_T picks the narrowest `int-repr` that holds the range (elemwidth-2026-08-27).
- ρ_T is **monotone**: τ ≤ σ implies ρ_T(τ) is no wider than ρ_T(σ).
- **A scalar's range is written in IR_P too** (irstep3java-2026-09-25). Finalize gives every
  integer value that is not a function parameter the range of its interval fixpoint. A parameter
  keeps its declared type, because callers were compiled against it. On Java, ρ picks between
  two host types: `int` when the range lies within [−2³¹, 2³¹−1], `long` otherwise. So an index
  proven small is Java's `int`, and no access takes a cast.
- **An operation is computed in the join of its representations.** Let R be the widest of
  ρ(operands) and ρ(result). The printer computes in R, then narrows to ρ(result). This is exact:
  every operand and the true result lie in R, so the host's operation on R is ℤ's, and the result
  lies in ρ(result). It holds for `div` and `rem` as well. The residue map ℤ → ℤ/2ᵏ covers only
  `+ − ·`.

So **the type on a value is the representation decision**, made by the analyses before printing and
written into the file (research §6). A printer spells ρ_T(τ), which is a lookup.

### 4.3 The flow rule, and the narrowest uniform representation

Along a flow edge from a value of type τ into a parameter or result of type σ (an initial value,
`continue`, `break`, `yield`), the rule is τ ≤ σ, and:

- **For a scalar, a change of representation is a coercion** the printer emits (int to u64, word to
  big). It preserves the value, because the value lies in ⟦τ⟧ ⊆ ⟦σ⟧, which both representations
  contain. It is the embedding of ℤ that ADR 0026's residue map commutes with.
- **For a table, a buffer or a map, the representation must be equal**: ρ_T(τ) = ρ_T(σ). Converting
  one would be a copy, and a copy is hidden allocation, which is forbidden.

**Theorem D (the narrowest uniform representation).** Take the graph whose nodes are a function's
table-, buffer- and map-typed values and whose edges are its flow edges. Let r(x) be the element range
the analysis proves for node x, and let hull(C) be the least interval containing r(y) for every y in a
connected component C. Then:
1. in every typing that satisfies the flow rule, the element representation is **constant on each
   component**;
2. a representation R is admissible for C iff it holds hull(C);
3. so the **narrowest** admissible one is ρ_T(hull C). It is attained by giving every node of C the
   element type hull(C), which satisfies the flow rule;
4. it is computed by union-find over the edges, then one join per class.

*Proof.*
1. Each edge forces equal representations at its ends, and equality is an equivalence relation, so
   it holds between any two nodes joined by a path.
2. Soundness requires the representation of each x to hold r(x). One R serves the whole component,
   so R holds every r(y). An integer representation's set is an interval, so it holds all of them iff
   it holds their hull.
3. By definition, ρ_T picks the narrowest representation holding a range. Giving every node the same
   type hull(C) makes each edge an equality, so both τ ≤ σ and equal representation hold.
4. Union-find computes the components, and the hull is a join over each one. ∎

Along an edge that is not on a cycle (an initial value into a loop parameter), the ranges may differ
and the representation may not. That is why the theorem is about representations and not about
ranges.

**Theorem D′ (with fixed members).** A class that touches a **declared** representation takes it:
a signature's parameter or result, or a primitive's argument or result. The host compiled that
declaration, so no other representation is admissible. It is sound because every member's values lie
in the declared set. **That premise is an obligation Finalize discharges**: the class's proven element
range (the reduced product below) must lie inside the declared element, or the program is refused,
naming a literal element when one is at fault (literal-elements.md). Until irstep4a-2026-09-26 it was
assumed: `(x.take (array 104 300 33))` against a declared `(array (int 0 255))` printed
`[]byte{104, 300, 33}`. The term backends had checked it at emission. **A signature's declared arity is
the same kind of boundary**: a body yielding a different number of values than the signature declares
is refused in lowering. Two different fixed
representations in one class are a program the host cannot type, and are reported. A class with no
fixed member takes ρ_T of its hull, as in Theorem D (`ir/final.go`).

**The hull is a reduced product of two factors** (Cousot and Cousot 1979). Each factor is a sound
over-approximation of the class's elements, so their meet is: γ(a ⊓ b) ⊇ γ(a) ∩ γ(b).
- **The term factor** is the hull of the class's sources as the analyses on terms see them. For a
  `build` it is the interval analysis on the build's own λ, written at lowering (`BufferRange`: its
  stores joined with the zero fill). For a literal it is its constants. A source nothing bounds is the
  word.
- **The IR factor** is **array smashing, flow-insensitive** (Blanchet et al. 2003): the join of every
  value ever placed in one of the class's tables. That means a build's zero fill, each stored value, a
  literal's elements, a tabulation's yields, and what a declaration says of a table from outside. Each
  value's fact comes from the IR's own interval domain (`ir/interval.go`), which reads `assume`s
  (Theorem E's reasoning, applied at entry). A value read back out of the same table carries that
  table's own fact, so a circular dependence is sound.
- **A class a host may write** through an argument no declaration fixes takes the word.

This is LoopOneJoin's rule and P2's "widths once per class" (irp2 §6), derived rather than
special-cased. The analysis supplies each node's element range.

### 4.4 Modes of integer arithmetic

An arithmetic operation carries a **mode**, which is decided by the analyses and final in IR_P:

| mode | meaning | printed as |
|---|---|---|
| `exact` | the result is proven inside ρ_T(result type) | the host's operation, unchecked |
| `trap` | the program asked for `-checked`, and the result is not proven | the host's checked operation, which traps outside the representation |

An operation with neither mode is not in IR_P. Such a program is refused, by ADR 0019 as amended by
0026. A `div` or `rem` by a divisor that may be zero keeps integers.md's meaning, which this
specification does not restate.

---

## 5. Dynamic semantics

### 5.1 The monad

Let W be the host world (the state host calls read and write). A morphism X → Y denotes a function

    X → T Y,   T Y = W → ((Y × W) + Trap + ⊥)

- `Trap` is a `trap`-mode operation leaving its representation, or a host's own failure.
- ⊥ is non-termination.
- T Y is ordered with ⊥ least. Morphisms ordered pointwise form an ω-complete partial order, and
  composition is continuous (the Kleisli category of T is CPO-enriched).
- A **pure** operation factors through the unit: it neither reads nor writes W.

### 5.2 Regions

A region denotes a morphism from its parameters, together with the values in scope, to T(Exit):

    Exit = Yield Ȳ + Break B̄ + Continue X̄

The operations run in sequence, each extending the environment. A `branch c R₁ R₂` is R₁'s meaning
if ⟦c⟧ = true and R₂'s otherwise.

### 5.3 Operations

- **Integer operations** are ℤ's, computed exactly (ADR 0026). In IR_P mode `exact`, the result is in
  ⟦τ⟧ by proof. In mode `trap`, a result outside ρ_T is Trap.
- **`call p ā`** is the target's primitive: its model is the host's function (ADR 0021, "a target is a
  model"), and its purity is its declared bit.
- **`index t i`** is t(i). It is defined only on 0 ≤ i < len t. That is its domain, which in IR_P is
  **discharged** (refinements.md §3a), so a printer may emit an unchecked access, and a host's own
  check is never relied on (noprop-2026-09-25).
- **`if c R₁ R₂`** runs the chosen arm to its `yield` and binds its results. It is [R₁, R₂] composed
  with what follows (L2).
- **`loop ā R`.** Let f = ⟦R⟧ read as X → T(B + X), with `break` the left injection and `continue` the
  right. Then

      ⟦loop ā R⟧ = f†(ā),   f† = ⨆ₖ fₖ,   f₀ = ⊥,   fₖ₊₁ = [η, fₖ] ⊙ f

  where ⊙ is Kleisli composition and η the unit. That is the least solution of L4, which exists by
  Kleene's theorem because composition is continuous. Termination (sct) is a separate proof that
  f†(ā) ≠ ⊥ for every ā.
- **`build n R`** allocates a buffer of length n, zero-filled (tables.md §14.3), binds it to R's
  parameter, and runs R to its `yield`, whose values are the results. A buffer among them is frozen:
  it is from then on an `(array τ)` (ADR 0031). **`build-map`** is the same for an empty map of
  capacity n (maps.md §6).
- **`tabulate n R`** is the table i ↦ ⟦R⟧(i) on Fin n (L11).
- **`read m k`** is (0, m(k)) when k ∈ dom m, and (1, ⊤) otherwise, where ⊤ is a value of V the
  program cannot observe (maps.md: the payload is read only under tag 0).
- **`the τ a`** is a; **`require c`** has no runtime meaning (§7).

---

## 6. π-parameters

A π-parameter `(pi %N τ (%M REL %K))` of a region that is a `branch` or `if` arm **is %M, renamed** for
that arm (L9). Its type τ is %M's type narrowed by the guard. A length π renames a table, and its
guard narrows the table's length.

**Theorem E (a π's guard holds).** Suppose an arm is entered only when its guard condition c holds,
and c implies `%M REL %K`. That is the case when c is that comparison, or, on the true side of a
conjunction and the false side of a disjunction, a conjunct of it. Then every execution that enters
the arm satisfies ⟦%M⟧ REL ⟦%K⟧.

*Proof.* By §5.2 the arm runs only if ⟦c⟧ = true (or false, for the else arm with the negated
relation). The implication is the definition of ∧ and ∨ (L10). ∎

**Consequence (sparse analysis).** An abstract domain gives each value **one** fact, at its
definition, and a π's fact is its source's fact met with the guard's constraint. By Theorem E and L9
the concretisation is sound. No abstract state per program point is kept, so nothing is copied at a
branch. That is the 57.7% of freq's compile allocation that P1 measured and P3 removed
(12–115×; irp3 §1). This is Bodík, Gupta and Sarkar's e-SSA and Ananian's SSI, placed as region
parameters, and it adds no operation to Σ.

**Lowering places a π** on each operand of a guard that is a bound value or the length of a bound
table, and on each operand of the comparisons a connective implies (irp3). More π-parameters are
never wrong (L9) and cost only their definition.

---

## 7. Obligations, and the step from IR_A to IR_P

An **obligation** is a condition every execution reaching a point must satisfy for the program to
denote (refinements.md §3a). There are three sources:
- **implicit in an operation**: `index t i` requires 0 ≤ i < len t. An arithmetic operation requires
  its result in ρ_T, unless its mode is `trap`;
- **`require c`**: a declared precondition (ADR 0028), placed by lowering where the reducer marked it;
- **`the τ a`**: ascription, whose range the analyses may use and must justify (types.md).

**The step from IR_A to IR_P** is the only place a program can be refused after lowering. It happens
exactly when:
1. every obligation is discharged by one of refinements.md §3a's three routes: a proof in the
   product of the domains, an assumption that is the same term, or the evaluation of a closed
   comparison. Otherwise the program is refused, naming the obligation;
2. every arithmetic operation's mode is decided (§4.4);
3. every type is final: the analyses' ranges are written onto the values, and table classes take
   their representation (Theorem D′, the reduced product of §4.3). The rewrites the step makes, such
   as bounds-check re-slicing by L13 (§9.4), happen here, where their premises are discharged;
4. `require` and `the` are erased: a `require` is removed, and a `the`'s result is replaced by its
   operand.

**Soundness per (domain, operation).** An analysis is a model of Σ (research Theorem B). Its soundness
is that each abstract operation over-approximates §5's concrete one, with π by Theorem E and `loop` by
a post-fixpoint reached by widening. So the planted-fault table of a domain has one row per operation
of Σ, and an operation with no row is a gap by construction, not by oversight.

---

## 8. Lowering: residual → IR

### 8.1 The translation

L is defined on the residual's normal forms (research Theorem A). It carries an environment from the
residual's de Bruijn binders to values, and it **never rebuilds a term**: a λ's body is read through
its binder's frame, and `openFresh` is not called.

| residual | IR |
|---|---|
| a literal k | `(const k)` |
| a bound variable | the value its binder is mapped to |
| a free name bound to a constant graph | `(global g)` |
| `(let e (fn (x) b))`, `((fn (x̄) b) ē)` | lower e (or ē), map x to its value, lower b |
| `(p ā)`, p an integer operator or comparison of the target | `(ARITH MODE …)` or `(CMP …)`: the classification happens once, here. **Division and remainder are ℤ's only where the target says so**: its declaration fixes integer operands (Go's `/`), or it is named integer division (`idiv`, `irem`). JavaScript's `/` is division in the reals (7/2 is 3.5) and stays a `call` (`ir.IntegerDivision`). A primitive declared over `any` is a `call` until typing shows its operands are integers, then it is promoted |
| `(p ā)`, p another primitive | `(call p …)`, one result |
| `((p ā) (fn (x̄) b))`, p with several results (ADR 0027) | `(call p …)` with \|x̄\| results, then b |
| `(t i)`, or a primitive declared `index` on a language table | `(index t i)` |
| `((m k) (fn (#t #p) b))` | `(read m k)`, then b |
| `(P (fn (x̄) b))`, P a producer of \|x̄\| values (a join point, ADR 0031) | lower P to its values and map x̄ to them. No projection and no copy: this is L5 read left to right |
| `(tuple ā)` in tail position | `yield` or `break` with ā |
| `(if c a b)` in value position | `(if c R_a R_b)` |
| `(if c a b)` in tail position | `(branch c R_a R_b)`, with π-parameters (§6) |
| `(loop (fn (x̄) body) z̄)` | `(loop z̄ R)`: `again` becomes `continue`, and a leaf becomes `break` |
| `(build n (fn (b) body))`, `build-map` | `(build n R)`, `(build-map n R)` |
| `(alloc (table n (fn (i) e)))` | `(tabulate n R)` |
| `(alloc t)`, t not a rule | `(tabulate (len t) R)` with R = i ↦ `(index t i)`: the table of t's contents now. For an immutable t this equals t (L12), and a printer drops the copy where t's type is not a buffer. For a buffer it is the meaning: today's backends emit the identity there, and a later store shows through (irstep1-2026-09-25) |
| `(array ā)`, a map literal | `(array …)`, `(map …)` |
| `(len t)`, and a primitive the target classifies as a length (`go.len`) | `(len t)` |
| `(keys m)`, `(set b i x)`, `(insert m k x)` | the operation of the same name |
| `(the τ e)` | `(the τ e)` |
| a contract mark (ADR 0028) | `(require c)` |
| an export's `where` | `(do (assume c))` at the function's entry: at an export the precondition is assumed, because its callers are outside the program (ADR 0028). Not lowered when a representation pass moved a parameter it names above the word (`big`, `u64`), since the source's term would no longer mean what it says; dropping an assumption is always sound |

**Anything else is a compile error that names the term.** The IR has no opaque operation. P1's
prototype counted opaque terms and found none on four programs, and the specification makes zero the
rule. CLAUDE.md's lesson "a shape a pass does not know is a shape it does not check" becomes a
property of one function.

### 8.2 One walker

The residual's control has four forms that every walker of a clause chain had to know: `again`, `if`,
`let`, and a host call's continuation (ADR 0027). A walker that missed one was unsound, and two did
(prodresult-2026-09-25). In the IR, `let` and a continuation are sequencing, and the other two are
terminators. **A consumer walks regions and meets four terminators, all listed in §1.3.** A Go
`switch` over them with no default fails to cover when a fifth is added.

### 8.3 L is faithful

**Theorem C.** For every residual t accepted by L, ⟦L(t)⟧ = ⟦t⟧, where the right side is the residual's
semantics (state.md, effects.md) read in §5's monad.

*Proof.* Structural induction on t, over the rows of §8.1.
- **Renaming rows** (a variable, `let`, a β-redex, a join point, a continuation) change only names.
  The semantics is invariant under α, and a join point is L5.
- **Operation rows** map a residual form to the IR operation whose §5 meaning is that form's, by
  definition.
- **Branch rows** are the coproduct in both readings (L2).
- **The loop row.** Both sides are the least fixpoint of the same functional. By the induction
  hypothesis the body lowers faithfully, so their approximants agree at every k, and so do their
  suprema.
- **A π** is the identity (L9).
∎

L is also checked by translation validation (research §10): the differential suite runs every case on
four targets against hand-computed answers.

---

## 9. Printing

### 9.1 What a printer is, and what it may assume

A printer for T is a **model of Σ in T's source language** (research Theorem B). It is faithful when
each operation it prints computes §5's meaning. Faithfulness of a whole program then follows by
structural induction.

A printer reads IR_P and may assume:
- W1–W10;
- every index is in range (§5.3);
- every arithmetic operation is exact or explicitly traps (§4.4);
- every type is final.

**It spells types and never recomputes a fact.** Its input is the IR_P file and T's declarations
(`repr`, the forms of `call`s), and nothing else, so a third party can write one (ADR 0006).

### 9.2 Structure per host

| IR | Go | JavaScript | Java | x86 |
|---|---|---|---|---|
| `loop` | `for { … }` | `for (;;) { … }` | `for (;;) { … }` | a label and a back jump |
| `continue v̄` | parallel copy, `continue` | parallel copy, `continue` | parallel copy, `continue` | parallel copy, jump to the head |
| `break v̄` | copy to the results, `break` | the same, or `return` (§9.4) | the same | copy, jump to the exit |
| `branch` / `if` | `if … { } else { }` | the same | the same | a compare and a jump |
| `yield v̄` | assignment to the owner's results, then fall through; `return` at a function's body | the same | the same | the same |

No row needs a label, because exits are single-level (§1.3).

### 9.3 Out of SSA

`continue`'s arguments are a **parallel copy** into the loop's parameters, and so are `break`'s and
`yield`'s into the results.
- **Go** has a parallel assignment.
- **JavaScript, Java and x86** do not: the comma operator is not one, which the term backend was once
  caught by. They sequentialise it with `plan.Moves`, Rideau, Serpette and Leroy's algorithm ("Tilting
  at windmills with Coq", JAR 2008, CompCert's): emit a move whose destination no remaining source
  reads, and when every remaining destination is read, save one into a temporary and rename it. That
  is one temporary per cycle.
- **A source may be an expression, not only a name**, when a printer inlines (§9.4). So the
  dependency is on its **read set**, the identifiers it mentions, and a renaming replaces a whole
  identifier. With name-equality instead, `a ← (a+1), c ← (a+1)` assigned a first and gave c the new
  value; the exhaustive test over three variables finds that planted fault.
- A copy of a value onto its own parameter is omitted.

### 9.4 The measured decisions, as rules on the IR

These are **requirements** (research §7). Each one is either a rewrite justified by a law of §1.4 or
a printing policy that preserves meaning trivially. Each is kept because it was measured against
hand-written code, and a change to it is re-measured.

| rule | statement on the IR | law, and side condition | evidence |
|---|---|---|---|
| **soleExit** (coalescing) | a loop result that every `break` passes as the same parameter p *is* p. Print no temporary | by §5.3, r = p on every exit | 20,480 B/op against 0 on Go (escape analysis) |
| **PostVars** | a parameter that every `continue` passes as p + k, for one literal k and a value read nowhere else, is updated in Go's post clause | the body is rewritten and f† is kept (L4) | 1.4× on the sieve (loopshape-2026-08-25) |
| **bounds-check re-slicing** | before a loop guarded by p against `len X`, a table Y read in the loop only at p's π is replaced in the loop by `restrict Y (len X)`, which Go prints `Y[:n]` | L13, **with `restrict`'s obligation `len X ≤ len Y` discharged at the loop's entry**. Today by an `assume` of the same term (refinements.md §3a's second route: dot's `where len p = len q`); no assumption, no rewrite (`ir/restrict.go`, `TestRestrictNeedsItsPremise`) | 1.96× on compute-bound loops (bce-2026-08-15) |
| **connectives** | `(if c true E)` prints as `c \|\| E` and `(if c E false)` as `c && E`, when E's region is an expression tree: pure, every value read once, no loop. The same holds for a `branch` **terminator** whose arms each yield one boolean, the shape case-of-case leaves at a tail (`plan.BranchConnective`), and `(if c false true)` is `!c` where a printer implements `Negator` | L10, and L6 lets a pure E run under the short circuit; negation is the coproduct's swap | irp2 §3: equal to today's backend, and ±5% for the alternative |
| **JavaScript tail return** | a `break` from a loop whose results the function yields directly prints as `return` | L2: the join's continuation is the function's return | 1.31× on V8 (native-js-2026-08-20) |
| **Java index narrowing** | a value whose range is inside Java's `int` is printed as `int`, and so an index takes no cast | ρ is a choice among the representations containing the type (§4.2). The range is the value's interval fixpoint, written into IR_P. A per-variable guess taken one inductive step failed here: the term backend answered 705032704 for 5·10⁹ (`narrow-from-wide`, irstep3java-2026-09-25) | 1.04–1.45× (native-java-2026-08-25) |
| **several results** | a function's results print as an object on JavaScript, natively on Go and as a record on Java | products (§1.1) | multiresult-2026-08-22 |
| **buffer reuse** | a `build` in a loop body whose buffer is dead at the `continue` alternates with a spare allocated once | W7: the old buffer has no later read or consumer | 2.5–2.7× (native-gauntlet-2026-08-20) |
| **element width** | a table's element type is its class's join (Theorem D) | §4.3 | elemwidth-2026-08-27; a soundness question on x86 (wintables-2026-08-25) |
| **β-inlining (JavaScript only)** | a value read once, in the region that defines it, is written at its use instead of bound to a `const`. **Not on Java**: inlining booleans measured ±10% in opposite directions on two programs, under the noise floor (irstep3java-2026-09-25 §3) | L1 (β for `let`) and L6. The value is pure and total, and is not moved into a nested region (a loop would repeat it). A read of a **buffer** is inlined only when no effect lies between it and its use, so no store can come between | with conditional expressions (next row), recovered the JSON tokeniser from 1.21× to 1.01× of the term backend (irstep3-2026-09-25) |
| **conditional expressions (JavaScript)** | an `if`, or a branch terminator, whose arms are expression trees prints `c ? a : b` | the coproduct as a value (L2). Go has none (`Speller.Cond` answers ""), which keeps Go byte-identical | the same measurement |
| **packed JavaScript arrays** | `build` of a numeric table prints `new Array(n).fill(0)` or a typed array | zero fill is `build`'s meaning (§5.3) | a sparse array is a dictionary on V8 |

**Why bounds-check re-slicing has a side condition.** Today's Go backend applies the rule without one,
and the rule is unsound. The witness (found while writing this section, 2026-09-25):

```lisp
(sig count-zeros ((a slice-int) (b slice-int)) int
  (where (and (go.< (go.len a) 65536) (go.>= (go.len b) 10))))
(def count-zeros (a b)
  (loop ((i 0))
    (go.>= i (go.len a))         i
    (go.>= i 10)                 i
    (go.!= (go.at-int b i) 0)    i
    else                         (again (go.+ i 1))))
```

Every read of `b` is proven, by `i < 10` together with `len b ≥ 10`. `gen` emits `s2 := b[:len(a)]`,
which on `len a = 20`, `len b = 10` panics with "slice bounds out of range [:20] with capacity 10",
where the answer is 10. The rule assumed `len b ≥ len a`, and nothing proves that. The comment in
`emitNarrow`, that moving a panic earlier is invisible, holds only when the original program itself
reads out of range, and this one never does. On the IR the rule's premise is an obligation like any
other, discharged at the loop's entry or the rule does not fire. In dot and centroid it is discharged
by the `where` `len p = len q`.

### 9.5 A printer is a function of the IR

A printer's output depends only on the IR_P file and the target's declarations. It iterates nothing
in an order the host randomises (CLAUDE.md, "the emitter must be a function of its input"). Its test is
the same: print twice and compare.

### 9.6 The x86 printer: a value's place is a colour of an interval graph

x86 is the one host with no variables, so its printer decides where each value lives: a callee-saved
register (rbx, rsi, rdi, r12–r15; xmm6–xmm13 for `f64`), an immediate, or a frame slot. The other
hosts' compilers make that decision themselves. Here the IR makes it, and the decision is a theorem,
not a heuristic.

**Program points.** Number the function in pre-order. Each statement s gets a point. One with
sub-regions (`if`, `loop`, `build`) owns an interval [start s, end s] around its sub-regions' points,
and so does a `branch` terminator. A region's parameters are defined at its entry. An `if`'s or a
`loop`'s results are defined at `end s`.

**Live sets.** The live set of a value v is the set of points on the paths from its definition to
its reads, a union of ranges. For a read at q:
- in v's own region, [def v, q];
- into a statement or branch t that does not contain the definition, `start t` together with the
  path through the one sub-region the read is in, not through its siblings;
- **except a loop**, which is covered whole, since its back edge returns to every point.

A loop parameter is defined at the body's entry and is live to its last read. A `continue` writes
it: the parallel move of §9.3, which under Rule R also reads the spare and its parameter. Aliases
(L9: a π, a store's value, a statement call's first argument, a build's frozen buffer) share one
place, and a class's live set is the union of its members'. Merging them keeps the program strict
SSA: a π is its source, and a store's value is its linear buffer.

**The printer's own reads count.** An instruction the printer adds reads what it reads. The length
header written after the allocator returns reads the size where the buffer is defined, and the swap
at a `continue` reads the parameter. Both were missing at first, and each gave a wrong answer
(`map-len`, and `big-limbs`): a register the numbering believed dead was reused.

**Theorem (optimal colouring).** Two values interfere when their live sets meet. Colouring in order
of definition, which in structured SSA is dominance order, each value taking a register no
interfering value holds, uses exactly ω registers, the most values live at one point. Nothing uses
fewer. For strict SSA the interference graph is chordal, and definition order reversed is a perfect
elimination order (Hack, Grund and Goos 2006; Bouchez, Darte and Rastello 2007). Without the holes,
every live set would be an interval, which is linear scan's model (Poletto and Sarkar 1999).
Measured, the holes matter: an interval model kept the sieve's counter live through the sibling arm
of its guard, which blocked the in-place `add` on the back edge.

**Early clobber.** A template writes `%r` before it reads its operands (windows-target.md §2). So a
result interferes with every operand: the result's live set begins at the statement's point, where
each operand is read. There is one exception, the textual proof emit/asm.go already states. A
template that begins `mov %r, %1` and names `%1` nowhere else may put its result in `%1`'s register
when the two live sets meet only at that point: its first instruction is then a no-op, and no path
reads the clobbered value. That is how `i + 1` on a back edge prints as `add rdi, 1`.

**Spilling.** When ω exceeds the 7 general registers, a value goes to a frame slot for its whole
life (spill everywhere). Either the value being coloured takes a slot, or the values blocking one
register do, whichever weighs less. A value's weight is Σ 8^depth over its uses and its definition,
where depth is loop nesting. A slot operand is loaded into r10 or r11 for the one instruction that
reads it, which is the term backend's convention. A template operand beyond those two **borrows** a
value register, one holding none of the instruction's operands and not its destination. The register
is saved to a frame slot and restored after the template. It is callee-saved, so a call in the
template preserves it, and a slot rather than a push keeps rsp's alignment for any callee.

**Jumping code.** A boolean is not materialised when it is read once, as the condition of the
`branch` or `if` that immediately follows its definition, and it is either:
- a comparison with a `(jump cc)` form, which prints as the flag-setting instruction and `jcc`;
- a pure `if` whose arms yield booleans, which prints as jumps to the two continuations
  (Aho, Sethi and Ullman §8.4; booleans.md §2.7).

Adjacency keeps the emission order the program's, so the lemma needs no change.

**Ordered back edges** (windows-target.md §5 item 3). A `continue` is a parallel assignment
pⱼ ← eⱼ(p), and x86's destructive `add` computes pⱼ ⊕ k in place only if nothing reads pⱼ afterwards.
So before numbering, an in-place candidate v = pⱼ ⊕ k whose value is the continue's argument j moves
after the last statement that reads pⱼ, when none of those reads v. That is L6: pure arithmetic reads
no memory, so it commutes with every statement it passes, stores included. `(again (+ i 1) (+ acc i))`
prints `add rdi, rsi` / `add rsi, 1`.

**Jump threading.** A jump to a label whose first instruction is `jmp X` goes to X, and code after an
unconditional `jmp` up to the next label something jumps to is reached by no path and is dropped.
Both keep every path. Jumping code makes such trampolines by construction: an arm that yields a
constant is one `jmp`. With threading, both failures of `and` in a guard leave for one label
(booleans.md §2.7). A template's own labels are opaque and left alone (`ir/x86/thread.go`).

---

## 10. The file format

### 10.1 Text

The format is the s-expression text of §2.1, in the language's lexical syntax (§2.1). There is one
**canonical printing**:
- values numbered in definition order;
- one `val` per line;
- regions indented;
- globals and functions sorted by name.

The canonical printing is what a test compares and what `cmd/check`'s baseline would hold. A binary
form is not specified (§12).

### 10.2 The header

- **`(stage A)` or `(stage P)`** says which stage the file is (§0, §7). A printer reads only P.
- **`(ir VERSION …)`.** VERSION is an integer. It changes only when **the meaning of an existing
  operation or the syntax changes**. Adding an operation does not change it, because covering (next)
  handles additions.
- **`(target T)`** names the target the program was lowered for.
- **`(ops …)`** lists every operation the program uses. A printer declares the operations it prints,
  and a program is printable by it iff its `ops` are a subset: ADR 0002's covering, applied to the
  IR's own operations as it already is to primitives. A new operation costs an old printer only the
  programs that use it, and it says so by name.

### 10.3 An example

`examples/native/dot-go.oro`, lowered for Go, in IR_P. Lowering writes exactly this in IR_A
(`gen -ir`), except for the ranges, which are `int` there: the loop variable's (int 0 65535) comes
from the `where` on `len p`, and it is the analyses that write it. Each arm renames both the index
(`%9`, `%11`) and the table whose length it was compared with (`%10`, `%12`).

```lisp
(ir 1
  (target go)
  (stage P)
  (ops const add ge call index len loop yield break continue branch)
  (func native-dot (params (%0 slice-float64) (%1 slice-float64)) (results f64)
    (region
      (val (%2 f64) (const 0.0))
      (val (%3 (int 0 0)) (const 0))
      (val (%4 f64)
        (loop (init %2 %3)
          (region
            (params (%5 f64) (%6 (int 0 65535)))
            (val (%7 (int 0 65535)) (len %0))
            (val (%8 bool) (ge %6 %7))
            (branch %8
              (region
                (pi %9 (int 0 65535) (%6 ge %7))
                (pi %10 slice-float64 ((len %0) le %6))
                (break %5))
              (region
                (pi %11 (int 0 65534) (%6 lt %7))
                (pi %12 slice-float64 ((len %0) gt %6))
                (val (%13 f64) (index %12 %11))
                (val (%14 f64) (index %1 %11))
                (val (%15 f64) (call go.f* %13 %14))
                (val (%16 f64) (call go.f+ %5 %15))
                (val (%17 (int 1 1)) (const 1))
                (val (%18 (int 1 65535)) (add exact %11 %17))
                (continue %16 %18))))))
      (yield %4))))
```

What a Go printer does with it (irp2):
- `%4` is `%5` by soleExit;
- `%18`, the index plus one, goes to the post clause by PostVars;
- `%1` is re-sliced to `len %0`, because `len %0 = len %1` is the `where`'s, discharged at the loop's
  entry.

The result is the loop `gen` emits today, one statement per value, at the same speed (irp2 §2:
0.99× of `gen`).

---

## 11. What checks this specification

| property | check |
|---|---|
| L is total on the corpus | `cmd/check`'s `ir` step: every program the sweep emits is lowered and verified |
| W1–W10 | a verifier (`ir/verify.go`), run after lowering and after every IR pass, with a planted-fault test per rule (`ir/ir_test.go`) |
| the printing is canonical | `cmd/check`'s `ir` step: print ∘ read ∘ print = print on every program |
| Theorem C (L faithful) | the differential suite, on all four targets |
| a printer is faithful | the differential suite and the gauntlet against hand-written code |
| an analysis is sound | one planted-fault row per (domain, operation of Σ), and the programs meant to be refused |
| proof counts never fall | `cmd/check`'s pinned counts, per domain as it is ported |
| §9.4's rules keep their evidence | the gauntlet, re-timed when a rule's statement changes |
| a printer is a function | print twice and compare |

---

## 12. Not specified here

- **A binary form.** Text is enough until a measurement says otherwise.
- **Facts in the format.** The file carries types, which are the decisions, and not the facts that
  justified them. A printer needs a representation, not its reason (ADR 0032, "Why not"). An analysis
  consumer that wants facts recomputes them from IR_A, which is cheap (irp3).
- **Which optimisations** (research §8). Each is a rewrite by a law of §1.4, and none is built
  without a measurement after the host's own passes.
- **One analysis engine, or domains in a fixed order with reduction** (research §5). This
  specification fixes what a domain is a model *of*, and leaves the engine open.
- **Host callbacks** (callbacks.md, tier 1). They would add a `call` owning a region: the host's
  lambda, printed in the host's syntax. They are not built, and the operation is reserved rather than
  specified.
- **The retired portable layer's kinds** (`loop`, `loop2`, `build` as a vector constructor). They
  stay on the old path with the old benchmarks and never reach L.
- **What moves off terms later**: `PromoteBig`, `SelectWords`, `FlattenProducts`.

## References

The research document's list, and in addition:
- Wadler, P. "Linear types can change the world!". *Programming Concepts and Methods*, 1990. Linearity
  carrying the order of effects on a buffer as data dependence (W7).
