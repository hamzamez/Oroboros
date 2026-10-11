# Program-order stores: a buffer as a place, or a buffer as a value

Research, 2026-10-11, on hamza's request. surface-design.md §2.4 wrote a store as `c[j] := TRUE`, in
program order, and counted it as notation-research §4.5's decision. That decision was never taken,
so hamza asked: *"research it first. the math the algebra and the literature."* **Decided**:
all four of §6's recommendations (hamza, 2026-10-11), as
[ADR 0052](decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md), which adds R0, the
freshness of a buffer's value, the inbound half of uniqueness this research left implicit. Not built.

## 0. The answer, in brief

1. **The two presentations are one semantics.** Today a buffer is a **value threaded linearly**: a
   functional array, `set : Aⁿ × [0,n) × A ⊸ Aⁿ`. The proposal makes it a **place**: a state binder
   holding a table, written by `b[i] := v`. The state-passing translation the language already uses
   for cells (local.md §3; Moggi's state monad, PlusCal's translation) maps the second onto the first.
   Its image is exactly the threaded programs that use each version once in one chain per name: the
   single-threaded ones (Schmidt 1985). That is the property today's linearity check verifies. So
   **the change is to the surface and to how the guarantee is checked, not to what programs mean or
   emit**. Array SSA (Knobe and Sarkar, POPL 1998) is the same translation in a compiler.
2. **What it buys.** Today's corpus, measured (§3), has:
   - 55 loop variables that exist only to carry a buffer;
   - 51 stores handed to `again`;
   - 27 stores nested in stores.

   In program order all three vanish: the loop forgets the buffer, the back edge forgets it, and a
   chain becomes a sequence of writes. **The linearity check stops being an ordering automaton** over
   evaluation order (abstract2: Wadler's `let!`) and becomes **syntactic rules**: Reynolds' syntactic
   control of interference (1978). A buffer is named only by its binder; a call borrows it for its
   duration; no closure over it escapes; and it freezes at its scope's end. "Read after move" stops
   being an error, because there is no move to read after. That is fit-research's F5, a checked
   invariant made grammar, applied to ADR 0018.
3. **It unifies two of the three binder kinds' uses, and removes installment 2's `linear` marker.** A
   buffer becomes **a state binder whose value is a table**, written element by element:
   - `b := table(n, 0)` is the buffer;
   - `t == table(n, 0)` is an immutable table;
   - `b[i] := v` writes an element in place;
   - `b := strconv.AppendInt(b, k, 10)` replaces the value, for the 18 host calls that may grow a
     buffer (§3).

   ADR 0047's "a cell is a buffer of one" read in reverse: a buffer is a cell that is written by
   element. surface-design §2.3's 26 written `linear` markers become unnecessary, because `:=` already
   says it, and its P9 (a cell holding a table) stops being a separate case.
4. **What it costs.**
   - **The 4 sites that rename a buffer version** (all differential cases about `alloc` copies) must
     copy explicitly. The new rule forbids naming a buffer twice, which is the point.
   - **A definition that threads a buffer through** takes it as a borrowed parameter instead. That is
     passed **by reference**, unlike ADR 0048's cells, which are copied, and it is sound only because
     nothing aliases it. Ada's rule for composite `in out` parameters is exactly this, and so is
     Rust's `&mut`.
   - **The claim that host write-borrows lose their wrapper** (notation-research §4.5) does **not**
     follow from the surface. It needs the IR to know that a borrow returns the same storage. That is
     a separate, measurable step, not part of this decision.
5. **Recommendation: take it**, as option B (§6), with §2.4's rules and §2.5's unification, built when
   the surface is, behind the byte-identical gate. **The prediction to test**: the translation
   produces today's threaded residual, so emission, proofs and the IR do not move.

## 1. The question, exactly

A buffer today (ADR 0018, 0020, 0031, 0047; local.md §2) is **linear and threaded**:

```lisp
(local c (table n false)
  (loop ((c c) (j (* i i)))                ; the buffer rides the loop as a variable
    (< j n)  (again (set c j true) (+ j i)) ; a store returns the buffer, the jump passes it
    else     c))                            ; the scope's value: frozen
```

The proposal writes the same program in program order:

```
c := [k ∈ 0..n-1 ↦ FALSE] ;
LOOP j := i * i :
    j < n  → c[j] := TRUE ; AGAIN(j + i)
 [] OTHER  → c
```

The question is whether the second can be *the* surface, what it means, what it checks, and what it
breaks.

## 2. The algebra

### 2.1 Today: a functional array, threaded linearly

The values are tables, functions on [0, n) (tables.md §1). The store is McCarthy's array theory ("Towards
a mathematical science of computation", IFIP 1962), with its two axioms, read over write:

```
get(set(b, i, v), j) = v            if i = j
get(set(b, i, v), j) = get(b, j)    if i ≠ j
```

A store makes a **new version**. In-place update is sound when the old version is never used again,
which is the aggregate update problem (Hudak and Bloss, "The aggregate update problem in functional
programming systems", POPL 1985). It is guaranteed by linearity (Wadler, "Linear types can change the
world!", 1990) or uniqueness (Clean: Barendsen and Smetsers, 1993). Today's check, `emit/linearity.go`,
is the ordering automaton abstract2 named. Per path, over reads r, lengths ℓ and a move m, the allowed
words are

```
(r | ℓ)* (m ℓ*)?
```

a store or a bare hand-on being the one move. Every version is threaded by hand: through a store's
result, a loop variable, a host call's tuple, `again`'s arguments.

### 2.2 Proposed: a place, in a state monad

A state binder names a **location**, and the store σ maps it to a table. Writing and reading are
operations of the state monad (Moggi, "Notions of computation and monads", 1991):

```
⟦b[i] := v⟧  = λσ. ((), σ[b ↦ set(σ(b), i, v)])
⟦b[i]⟧       = λσ. (get(σ(b), i), σ)
⟦b := e⟧     = λσ. ((), σ[b ↦ ⟦e⟧])
```

This is Idealized Algol's `new x in C` (Reynolds, "The essence of Algol", 1981), and Haskell's
`STArray` under `runST` (Launchbury and Peyton Jones, "Lazy functional state threads", PLDI 1994):
`writeArray :: STArray s i e → i → e → ST s ()`. The scope's encapsulation is `runST`'s, proved
observationally pure by parametricity (O'Hearn and Tennent, 1995), and **the freeze is `runST`'s
result**: the region ends, so no write can follow, and handing the table out copies nothing. local.md
§1 already cites this for cells; the proposal extends it to tables.

### 2.3 The bridge: the state-passing translation, and its image

**Theorem (one semantics).** Let S be the state-passing translation (local.md §3), extended with an
element write:

```
S⟦b[i] := e⟧  = let (v, σ) = S⟦e⟧ in ((), σ[b ↦ (set σ(b) i v)])
```

so that a buffer, like a cell, becomes a value threaded through every binding, branch and loop of its
scope, and `S⟦b[i] := e⟧` produces exactly today's `(set b i v)`. Then:

1. **S maps a program-order program to a threaded one in which each buffer's versions form one chain
   used linearly**: a store consumes the version before it, and every read sees the version current
   on its path. That is the store single-threadedness of Schmidt ("Detecting global variables in
   denotational specifications", TOPLAS 1985), which licenses in-place update. In a type system it is
   Guzmán and Hudak's single-threaded λ-calculus (1990).
2. **Every threaded program whose buffer versions form one chain per name is S of a program-order
   program**: unthread it, replacing each `(set b i v)` threaded on to b's next version by `b[i] := v`.

*Sketch.* (1) is induction on S's clauses: each clause passes σ along exactly once, and branches and
loops join the versions at a φ, as SSA does (Appel, "SSA is functional programming", 1998). (2) is the
converse on the chain. Where a threaded program keeps two versions alive, or names one version twice,
it is not single-threaded, and today's linearity check refuses it. ∎

So **the two surfaces denote the same residuals**, on the programs today's check accepts. The
difference is a program that names a version a second time and uses only the second name. That is
legal today, and §2.4's first rule forbids it (§3: 4 sites). In compiler form the translation is
**Array SSA** (Knobe and Sarkar, "Array SSA form and its use in parallelization", POPL 1998): program-
order array writes renamed into versions with φ at joins. It is what the IR already holds after
lowering.

### 2.4 What replaces the linearity check: syntactic control of interference

In program order the hazard linearity guards against, observing a store through a dead name, cannot be
reached by order: a read after a write reads the new version, by definition. What remains is
**aliasing**: two names for one place, so that a write through one is seen through the other. Reynolds
answered that for Algol in **syntactic control of interference** ("Syntactic control of
interference", POPL 1978; revisited by O'Hearn, Power, Takeyama and Tennent, *JFP* 9(3), 1999):
**distinct identifiers do not interfere**, and the grammar makes it so. For buffers that is four
rules, each checkable on the tree:

| | rule | what it forbids | precedent |
|---|---|---|---|
| **R1** | **a buffer is named only by its binder** | `d == c` copying the name: then `d[j] := 1` would be seen through `c` | SCI; surface-design §2.12 found it |
| **R2** | **a call borrows a buffer for its duration**, and no call is handed one buffer twice | `f(c, c)`, two parameters aliasing one place | SCI's interference of arguments; Rust's one `&mut`; Ada's composite `in out` |
| **R3** | **no closure over a buffer survives its scope** | a write after the region ends | already the rule for cells (local.md §3) and for closures (closures-direction.md) |
| **R4** | **the scope's value freezes the buffer** | a store into a frozen table | ADR 0031, unchanged |

With them, the ordering automaton (r|ℓ)*(m ℓ*)? has nothing left to rule out: there is no move to read
after. So `emit/linearity.go`'s evaluation-order walk (493 lines) becomes four syntactic conditions,
which is fit-research's F5 applied to ADR 0018.

**What R2 is, semantically.** A buffer passed to a definition or a host is passed **by reference**.
ADR 0048's cells are passed by **value-result**, a copy in and a copy out, because a cell is small. A
buffer is not, and copying it would be the hidden allocation CLAUDE.md forbids. By-reference is sound
exactly when the callee's parameter is the only path to the place during the call, which is R1 and R2.
**Ada's reference manual states the same contract** (RM 6.2: a composite `in out` parameter may be
passed by copy or by reference, and a program whose behaviour depends on which commits a bounded
error), and Rust
enforces it with `&mut` (Matsakis and Klock, 2014; RustBelt, Jung et al., POPL 2018). In separation
logic it is the frame rule's disjointness (Reynolds, LICS 2002).

**And at an export** the caller is outside the program, so uniqueness is **assumed**, exactly as
ADR 0020 has it for a `(buffer V)` parameter today. R1–R4 make linearity *through the body* structural;
the caller's promise stays a promise. surface-design §2.12 already reached this split.

### 2.5 The unification: a buffer is a state binder holding a table

Abstract Oroboros has three binder kinds (abstract.md §2.2): definitional `p == e`, state `x := e`, and
ranging `p ∈ S`. Today a buffer is a fourth thing in practice: a definitional binder with the linear
mode, decided by its initializer's form (ADR 0047). Installment 2 found the initializer cannot decide
it (`array` and `table` both build either), and proposed writing `linear` at 26 sites.

**Program order removes the fourth thing.** A buffer is the state binder holding a table:

| written | kind | what it is |
|---|---|---|
| `t == table(n, 0)` | definitional | an immutable table |
| `b := table(n, 0)` | state | a buffer: a place, written by element or replaced |
| `b[i] := v` | an element write | in place, on the place's current table |
| `b := strconv.AppendInt(b, k, 10)` | a whole write | the place takes the grown table; the old one is unreachable by R1 |
| `x := 0`, `x := x + 1` | state | a cell, as today |

ADR 0047 says **"a cell is a buffer of length one"**: Ref τ ≅ Buffer τ 1. Read in reverse, **a buffer
is a cell written element by element**: State(Seq(A)) with point update. Then:
- installment 2's 26 `linear` markers are not needed, since `:=` already says the binder may be
  written;
- installment 2's P9, a cell holding a table, is no longer a separate case. It *is* the buffer;
- and the growth calls need no special form: replacing a state binder's value is `:=`.

The usage mode `linear` remains where a buffer is **not** bound by a scope: a definition's or a host's
parameter (ADR 0050). There it means R2's borrow, and is written on the binder as decided.

### 2.6 What the translation must produce, and why emission should not move

S turns each state binder into loop variables and threaded values (local.md §3). For a buffer, that
yields:
- the loop variable today's programs write by hand;
- the store chain today's programs write by hand;
- the φ at an `if` that today's `(if c (set b i 1) b)` writes by hand.

So **the residual is today's residual**, and every stage after the loader is unchanged:
- the refinement layer's content facts, which induct on the store chain (array-facts.md);
- length preservation through `set` (refinements.md §5b);
- Theorem 3's bounded increments;
- the IR and the printers.

This is the claim the cells' build made and verified (`TestACellReducesToTheLoopVariableItMeans`,
α-equivalence); **for buffers it is a prediction, to be tested by building.**

### 2.7 Host write-borrows: what changes and what does not

Today a write-borrow is declared `B ⊸ B ⊗ R`, and its Go template hands the buffer back:

```
func(p []byte) ([]byte, int, error) { n, err := %s.Read(p); return p, n, err }(%s)
```

In program order the program writes `n := f.Read(buf)`, and the translation rebinds `buf` to the
borrow's returned version, so **the residual still needs a version handed back**. The template stays,
and emission is unchanged. Dropping the wrapper, emitting `f.Read(buf)` alone, needs the IR to know
that the returned version *is* the argument's storage. The declaration says so (a write-borrow hands
back the same buffer), so it is possible, but **it is an IR step with its own measurement**: Go
probably inlines the immediate function literal already, so the gain may be readability only.
notation-research §4.5's "the wrapper goes away" is therefore not a consequence of this decision.

## 3. Measured

Over `examples/`, `lib/`, `gauntlet/stdlib/acceptance/` and `gauntlet/differential/cases/`:

| | count | in program order |
|---|---:|---|
| buffer binders (`local` with a table or array) | 71 | `b := …` |
| cell binders | 13 | unchanged |
| stores `(set b i v)` | 114 | `b[i] := v` |
| … handed to `again`, threading a loop variable | 51 | the argument disappears |
| … nested in another store (a chain) | 27 | a sequence of writes |
| … elsewhere (an argument, a scope's result) | 30 | a write, then the name |
| loop variables that only carry a buffer | **55** | **disappear from the loop header** |
| a buffer handed to a host and taken back in a tuple | 5 | `n := f(b, …)`, the buffer not rebound |
| **a store's version bound to another name** | **4** | **refused by R1**: all in `gauntlet/differential/cases/alloc-copies.oro`, which tests that `alloc` copies; rewritten with an explicit copy |

And in the targets:

| declarations whose result hands a buffer back | count | in program order |
|---|---:|---|
| Go, a write-borrow wrapping the call to return the buffer (`io.Read`, `ReadFull`, …) | 12 | a borrow, R2; the template unchanged (§2.7) |
| Go, growth: the host returns a possibly new buffer (`strconv.Append*`, `fmt.Append*`, …) | 18 | `b := f(b, …)`, a whole write (§2.5) |
| JavaScript, Java, Windows | 0 | — |

So the change is large on the page, at 55 loop variables and 114 stores, small in what it refuses (4
sites, all testing copies), and nil in what it emits, if §2.6's prediction holds.

## 4. What is lost, and the risks

- **Two names for versions.** Today `(let c2 (set c 0 1) …)` may use `c2` and never `c`. R1 forbids it.
  The 4 sites test that `alloc` copies, and keep testing it with an explicit copy, `table(len c, c)`
  (local.md §1's thaw).
- **A version as a value in a tuple or a result.** A scope's result is still a table: R4, the freeze.
  Inside a scope, a buffer leaves only frozen, or borrowed by a call.
- **A definition that threads a buffer through** (`(def g (b) … (set b i v) …)`, returning b) becomes a
  definition with a borrowed parameter that writes in place and returns its other results. The corpus
  declares two (`mac-into`, `smooth-into`, both exports, ADR 0020). An internal definition is inlined
  before any check runs, so its buffer arrives through the caller's binder.
- **The proofs.** §2.6 predicts no change. If a content fact keyed on a hand-written chain does not
  survive, emission will show it: the gate is byte-identical.
- **Interference inside one expression**, `f(b[i] := 1, b[i])`, is defined by left-to-right
  evaluation, as cells already are (local.md §3). A lint could refuse a write and a read of one place
  in one argument list, but the semantics does not need it.
- **What is not verified.** Nothing here has been built. §3's counts are measured; §2.6 and §2.7 are
  predictions.

## 5. The literature

| | answers |
|---|---|
| McCarthy 1962 | the array theory: `select` and `store`, read over write |
| Reynolds 1978, "Syntactic control of interference"; O'Hearn et al. 1999 | distinct identifiers do not interfere: §2.4's rules |
| Reynolds 1981, "The essence of Algol" | a local variable as a place in a scope: `new x in C` |
| Hudak and Bloss 1985 | the aggregate update problem: when a functional store may be in place |
| Schmidt 1985 | single-threaded stores, the image of the translation: §2.3 |
| Wadler 1990; Barendsen and Smetsers 1993 (Clean) | linearity and uniqueness: today's guarantee |
| Guzmán and Hudak 1990 | the single-threaded λ-calculus |
| Moggi 1991 | the state monad, whose semantics §2.2 writes |
| Launchbury and Peyton Jones 1994; O'Hearn and Tennent 1995 | `runST` and `STArray`: encapsulated state, and its purity |
| Knobe and Sarkar 1998, Array SSA | the translation, in a compiler |
| Appel 1998; Kelsey 1995 | SSA is functional programming: the φ at a join |
| Ada RM 6.2 | composite `in out` by copy or by reference, and the program may not tell |
| Reynolds 2002, separation logic | the frame rule's disjointness, which R2 is |
| Matsakis and Klock 2014; Jung et al. 2018 (RustBelt) | one mutable borrow at a time, proved sound |
| Lamport 2009, PlusCal | variables translated into state: the language's own precedent, for cells |
| Lorenzen and Leijen 2023 ("FBIP"); Reinking et al. 2021 (Perceus) | the alternative: write functionally, update in place when unique; today's threading is this |

## 6. Decisions for hamza

1. **Which presentation:**
   - **A. Keep threading** (today). The linear functional array: no new rule, no rewrite, and the
     ordering check stays.
   - **B. Program order** (recommended). A buffer is a state binder holding a table. Writes are
     `b[i] := v` and `b := e`; reads are `b[i]`. The guarantee is R1–R4 (syntactic control of
     interference), and the translation produces today's chain.
   - **C. Both.** Two spellings of one operation, a synonym (fit-research F1). Not recommended.
2. **If B: the unification of §2.5**, the buffer as the state binder holding a table, so that `linear`
   is written only on parameters, and surface-design §2.3's 26 markers and option (a) are superseded.
   *Recommended.*
3. **If B: borrowed parameters by reference** (R2), with uniqueness assumed at an export as ADR 0020
   has it. *Recommended*: by value-result would copy a buffer.
4. **The wrapper on host write-borrows** stays, and its removal is a separate IR measurement (§2.7).
   *Recommended.*

What a B would need, in order, when the surface is built:
- a successor to ADR 0047: the operator decides the kind, and a buffer is state;
- local.md's translation extended by the element write;
- R1–R4 on the trees;
- then `emit/linearity.go` reduced to what R1–R4 do not cover, with byte-identical emission as the
  gate and a planted fault per rule.
