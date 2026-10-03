# Refinements

Written before the code, per [state.md §6](state.md). Layer 1's second half; the first half is
[types.md](types.md).

> **Status, 2026-08-15. Built.** `emit/linear.go` and `emit/refine.go`. `aindex` carries its
> bounds refinement on Go, and every obligation in every example is discharged.
>
> **It found a real latent bug in two of our own gauntlet programs on the day it was written.**
> `dot` and `centroid` each index *two* arrays with one loop bounded by the *first* — nothing
> related the lengths, so `(aindex q i)` was genuinely unproven. Both now declare
> `(where (int.eq (alen p) (alen q)))`, which is the precondition moving to the caller exactly as
> [types-sketch §2](../types-sketch.md) described.
>
> Two things the implementation taught. An **equality must be a substitution, not two
> inequalities** — that is what lets a fact about `p` discharge an obligation about `q`. And
> entailment needed **sums of two facts**: `i < alen p` plus `alen p ≤ alen q` gives `i < alen q`,
> a shape one fact can never reach and every two-array loop produces.

---

## 1. What this is for

Two holes in the specification are **shaped exactly like a refinement**, and both currently say
*stay inside, nothing checks it*:

| | the obligation | what happens outside it |
|---|---|---|
| [primitives.md §2](primitives.md) | `0 ≤ i < alen v` for `aindex` | Go panics, Java throws, **JS silently returns `undefined`** |
| [arithmetic.md §4](arithmetic.md) | `-(2⁵³−1) ≤ n ≤ 2⁵³−1` | Go and the JVM wrap, **JS silently rounds** |

The first is the one to close. It is the only place in the language where a *Tier 1* primitive is
Tier 1 only conditionally, and the condition is unchecked.

**This is a correctness deliverable, not a speed one.** The bounds-check *performance* win was
already collected as an emitter pattern with no types at all
([bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md)), and types-direction §2.2 was explicit
that the type system must not be justified by cases the emitter can already see.

## 2. Syntax

A `where` clause on a `sig` or a `prim`, holding an ordinary **boolean term** over the parameter
names:

```lisp
(sig dot ((a vec-f64) (b vec-f64)) f64
  (where (int.eq (alen a) (alen b))))

(prim aindex ((v vec-f64) (i int)) f64 expr "%s[%s]" pure index
  (where (logic.and (int.le 0 i) (int.lt i (alen v)))))
```

On a `prim`, `where` is a **trailing attribute** beside `pure` and `import`, because that is where
the grammar already puts them ([target-files.md §1](target-files.md)). On a `sig` it follows the
result type. The named argument form `((v vec-f64) (i int))` is now accepted by `prim` as well as
`sig`, so the two have not diverged.

There is **no predicate syntax**. A refinement is a term of type `bool` — the predicate language
is a *fragment of the term language*, which is why parameters were named in
[types.md §7](types.md) before anything read them.

## 3. Classify, do not restrict

> **Any boolean term may appear in a `where`. A term inside the decided fragment is *proven*. A
> term outside it is an *opaque atom* — propagated and matched by name, never decided.**

Consequences, all of them wanted:

- Nothing is rejected for being too expressive **as a declaration**: any boolean term may be
  written in a `where`.
- It **degrades gracefully**, and the fragment can grow later without any program changing.

### 3a. An obligation is discharged, or the program is refused

*Amended 2026-09-25 ([noprop-2026-09-25](../../gauntlet/results/noprop-2026-09-25.md)).* This section
said an undecided obligation "can only be discharged by an assumption that matches it, **or by a
runtime check at a boundary**", and the compiler read that as licence to emit the program with a note:
*propagated, not proven*. No runtime check was ever emitted. An obligation is the **domain condition**
of an application: `0 ≤ i < len t` for `(t i)` (tables.md §6), a primitive's `where` for a call.
Outside its domain an application has no value, and each host then does something different:

| out of domain | Go | Java | JavaScript | x86-64 |
|---|---|---|---|---|
| `(t i)` | panics | throws | **`undefined`**, silently | **reads past the table** |

Measured on the witness below: an `int` function returned `undefined` on JavaScript.

A program denotes only if every application in it is defined. So an obligation is **discharged**, by
exactly one of these, or the program is **refused**, naming the obligation and what was known:

1. **a proof in the fragment** (§4), including a proof by cases (joinConditional) and a content fact
   about a table (array-facts.md);
2. **an assumption that is the same term**: an opaque atom matched by name, the one thing an atom
   outside the fragment can be matched against. A comparison is the same term when it is the same
   **relation** on operands printed alike: `u64<` is `<` realised in U, emitted only where both
   operands lie in U (ADR 0026), so a guard in one spelling discharges a `where` in the other;
3. **evaluation**, when the obligation is **closed**: a comparison between two literals. `(!= 3.0 0)`
   is true, and comparing two literals is exact on every host, so it is decided at compile time
   without folding any arithmetic (ADR 0009).

There is no fourth route. "Propagated" survives in one sense only, the one §6b already has: an
**exported** definition's own `where` is *assumed* inside it and is its caller's to discharge. The
caller is outside the program, and the obligation is stated in its interface, not dropped.

**`-checked` does not clear an index.** It asks for the host's checked arithmetic, which every target
declares. A checked index would need every target to trap, and JavaScript and x86 do not. Not built,
and named.

The witness, accepted before this amendment and refused now:

```lisp
(let b (build b 8 (loop ((b b) (i 0)) (>= i 8) b else (again (set b i (% (+ i n) 10)) (+ i 1))))
     t (build c 6 c)
  (t (b 3)))             ; b's cells reach 9, t has six
```

### 3b. A disjunction in a goal (2026-09-30)

The fragment is conjunctions of linear inequalities (§4), and a disjunction reaches it only as a
**goal**, never as a fact the layer reasons from. `d ≠ 0` was the first, and it was proven by proving
one side. Scalar's precondition on `string-of` needed the general rule:
0 ≤ c ≤ 10FFFF and (c ≤ D7FF or c ≥ E000) (string-operations.md §7, gotarget-2026-09-30).

A goal built from linear atoms by ∧ and ∨ is decided by the right-hand rules of Gentzen's sequent
calculus (1935):

```
Γ ⊢ A ∧ B   iff  Γ ⊢ A  and  Γ ⊢ B
Γ ⊢ A ∨ B   if   Γ ⊢ A,  or  Γ ⊢ B,  or  A ∨ B ∈ Γ        (an assumption that is the same term)
```

It is **sound**, and **incomplete** in one named way: a disjunctive fact is never split. So a guard
`(if (or (<= c 55295) (>= c 57344)) (string-of c) …)` proves the call, because the guard is the same
term as the goal's disjunction, but Γ = {c ≤ 5 ∨ c ≥ 9} does not prove c ≠ 7. The rule applies only to
a goal containing a disjunction, so every other obligation is decided as before.

## 4. The fragment

**Linear integer arithmetic over difference constraints**, which is what every bounds obligation
actually is.

A term is *linear* if it is built from integer literals, names, `int.add`, `int.sub`,
multiplication by a literal, and **opaque length terms** like `(alen v)`, which are treated as
variables. Comparisons `int.lt`, `int.le`, `int.gt`, `int.ge`, `int.eq` over linear terms are
atoms; `logic.and` joins them.

Everything else — `f64` comparison, `ascii?`, a call to a defined function — is an **opaque atom**,
matched by syntactic identity and nothing more. `num/f64.eq` is opaque *by necessity*: IEEE
equality is not reflexive, so it is not an equivalence relation and a solver must not treat it as
one ([arithmetic.md §6](arithmetic.md)).

### Entailment

Facts are normalised to `e ≤ 0` where `e` is a linear expression. An obligation is discharged if,
after substituting known equalities, it is **implied by a single fact**: same variable part, and a
constant offset in the right direction.

Deliberately **incomplete**. It is not Fourier–Motzkin and it is not an SMT solver. It is the
smallest thing that decides the obligations this language actually generates, and being incomplete
is safe: an undischarged obligation is *refused* (§3a), never assumed.

**The equalities are a solved form** (`facts.assumeEQ`, 2026-10-03). They are kept as a substitution
σ, x ↦ eₓ, with no eliminated variable on a right side: Gaussian elimination over the variables. An
equation s = t is **added**: L = σ(s − t) is redundant when it is 0, a contradiction when it is a
nonzero constant (the facts then entail everything, which is right for a path that cannot be taken),
and otherwise solved for a variable of coefficient ±1, so the solution is integral. An equation with
no unit coefficient is kept as two inequalities. Until then it was an assignment: `len a = len out`
and `len b = len out`, both keyed under `len out`, kept only the second, so `len b = len a` followed
from one orientation of a precondition and not from the other
([lengtheq-2026-10-03](../../gauntlet/results/lengtheq-2026-10-03.md)). A key names one variable
wherever the facts reach because a residual's binders are named apart (ADR 0036).

## 5. Where facts come from

| | fact |
|---|---|
| `(fold-range z n f)` binding `i` | `0 ≤ i` and `i < n` |
| `(make-vec n f)` binding `i` | `0 ≤ i` and `i < n` |
| a `let` binding `x` to a linear term `e` | `x = e` |
| the enclosing `sig`'s own `where` | assumed |
| `alen`, `slen` | `0 ≤ alen v` |

That last one is free and worth having: a length is never negative on any target.

### 5a. A name bound to a value is bounded by the value's tails (2026-10-01)

`(let x V …)` gives x what holds of **every tail** of V. The tails are the terms whose value V's can
be, through the four forms of a clause chain (ADR 0015, ADR 0027) and the loop they sit in:

```
tails(if c a b)             = tails(a) ∪ tails(b), under c and under ¬c
tails(let v (fn (y) b))     = tails(b)
tails((p a…) (fn (ys) b))   = tails(b)        a host call's continuation runs once, now
tails(loop (fn (vs) b) z…)  = tails(b) without `again`
tails(again …)              = ∅               a jump has no value
tails(t)                    = {t}             otherwise
```

**The rule.** Let ℓⱼ be V's tails, each linear, reached under path facts Pⱼ: the facts in scope and
the guards on the way. For an inequality φ, if Pⱼ ⊢ φ(ℓⱼ) for every j, then φ(x).

*Proof.* Exactly one tail is evaluated, on a path whose guards hold, and x is its value. A loop has a
value only at an exit, in some iteration, where the guards on that path hold of that iteration's
variables. The loop's variables are binders: the path assumes nothing of them but those guards, not
even their initial values, which hold on the first iteration only. (Partial correctness: a loop that
does not end has no value to be wrong about.) ∎

**A loop's tail that names one of the loop's variables is not used**, and the rule gives x nothing.
That tail's value is the variable's at the exit, and what bounds it is the loop's invariants, which
the loop-summary rule has and this walk has not. This is a limit, not a soundness condition: the
walk is sound on such a tail and proves almost nothing of it. It is also what keeps the rule off
every scanner loop a program binds, whose exit is its index: without it the rule cost 60 ms of a
450 ms compile (`examples/json/tree.oro`, paired against the commit before).

The φ tried are a finite template set: x compared with each side of each guard, with 0, and with
each tail itself, when the side names no binder of V. Over constant tails that last template is the
interval hull [min ℓⱼ, max ℓⱼ]. It is the join of template constraint domains (Sankaranarayanan,
Sipma & Manna, VMCAI 2005).

Until bufio-2026-10-01 the walk knew `if` and `let` only. `examples/lines/lines.oro` binds its exit
status to a conditional over a loop whose exits are 0 or 1, some under a host call's continuation,
and `os.Exit`'s 0 ≤ code ≤ 255 was refused.

| rule | planted fault | caught by |
|---|---|---|
| a continuation's tails are the call's | the continuation not walked | `TestAStatusIsBoundedByItsExits` |
| a loop variable's initial value is not assumed on a tail's path | it is assumed | `TestATailsPathDoesNotAssumeALoopVariablesStart`: with i = 0 on the path, a guard i ≥ m gives m ≤ 0, and an index of −m was accepted |

### 5b. A loop variable that threads a buffer has the buffer's length (2026-10-01)

**The law.** `set` preserves length, len (set c i x) = len c: it is `set`'s declared postcondition,
as passing a table on unchanged trivially is. For a loop variable v with initial value z,

```
every back edge passes, at v's position, a term of length len v
  ⟹  len v = len z   at every iteration
```

by induction on the iterations: v = z on entry, and each step keeps the length.

**What is tried, and what is kept** (`threadLengths`). Every variable whose initial value has a
length starts with the equation len v = len z. A call with a length contract has one (a `build` of
n, a `set` on something that has one); so does a plain name, the quantity len(z). A position some
back edge does not preserve is dropped and the rest are re-checked under what remains, until nothing
changes: the **greatest inductive subset** (Houdini; Flanagan & Leino, FME 2001). Each equation kept
is preserved by every back edge assuming only the kept ones, which is the induction step.

**Two lengths are equal when the facts at the loop's entry say so**, not only when they are spelled
alike. A merge sort swaps two buffers on its back edge; len a and len b are both the n of their
`build`. Every name a length mentions is bound outside the loop and immutable, so what the entry
facts prove of it holds at every iteration.

**The walk to a back edge carries the lengths of names bound on the way**: `(let w (set u i x) (again
w …))` passes w, which has u's length. A binder with no known length is bound to *unknown*, and a
name bound inside the value never falls back to its own length variable: that variable would outlive
its scope, and two sibling loops' variables may share a name.

Until bufio-2026-10-01 a plain name had no length here and the check was all or nothing. The rule
never fired on `(loop ((c c) …))`, the idiom it exists for, and the programs were accepted because the
loop variable was spelled like its buffer.

| rule | planted fault | caught by |
|---|---|---|
| the equation is verified at every back edge | the back edge not checked | `TestALoopVariableHasItsBuffersLength` |
| lengths are equal under the entry facts | any two lengths equal | the same test |

**The facts are keyed by name, so the residual's names must be apart.** A binder that reused an
enclosing binder's name entered a scope whose facts were about another variable
([state.md](state.md), the residual's naming invariant).

## 6. The diagnostic

An obligation that cannot be discharged is an **error**, naming the obligation and what was known:

```
smooth: (aindex a (int.add i 2)) requires i + 2 < alen a
  known: 0 <= i, i < alen a - 2
```

There is no softer case. A refinement *propagated* rather than proven used to be a note, and the
program was emitted (§3a). [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) had already
established that a transformation which silently does not fire is indistinguishable from one that
does, and a note beside an emitted program was exactly that silence.

## 6b. A `where` on a DEFINITION, and a declared parameter range

**[ADR 0028](../decisions/0028-a-definitions-contract-is-checked-at-its-calls.md): both are obligations
at every call the definition is inlined into.** This section recorded the opposite until
2026-09-24, and the argument it made is kept because half of it still holds.

**A primitive's `where` is discharged at every call site** (§5). A definition's has no call site left
by the time the residual reaches `Refine`, because reduction inlines every non-exported call. What
protects the program's **safety** is that the obligations *inside* the body land at the call site with
the caller's own values:

```lisp
(sig safe ((a (array f64)) (n int)) f64 (where (and (<= 0 n) (< n (len a)))))
(def safe (fn (a n) (a n)))
(def f (fn (a) (safe a (go.- 0 5))))
```

```
(a (go.- 0 5)) is an indexing, and (<= 0 (go.- 0 5)) does not follow
```

That half stands, and it is still how every obligation inside a body is checked. What it does not
protect is **meaning**: a body that is *total* and merely wrong outside its domain fires nothing.
`lib/win/fmt.oro`'s `print-int -13` printed a blank line, and `num/u128`'s `digits18` given a value
past 10¹⁸ printed the low 18 digits. A range on a parameter is a type (ADR 0003), and a type is
checked at application. So the declaration is now checked *as well as* the propagated obligations,
never instead of them.

### And on a host function, the same (2026-09-30)

A host declaration's parameter range is the same claim: `bits.Len32 ((x uint32))` says the host's
domain is [0, 2³² − 1]. Until [gotarget-2026-09-30](../../gauntlet/results/gotarget-2026-09-30.md) it
was read as a type and nothing more, and a template's conversion made the claim false without a
word: `(bits.Len32 x)` with `x : int` compiled to `bits.Len32(uint32(x))` and answered 32 for −1. So a
host function's ranged parameters are marked at every call, exactly as a definition's are
(`Target.Env`, `hostRequires`), and decided by the same route below. A range holding the whole signed
word is vacuous for an `int` and is not marked.

### How it is checked

- **A range** is marked on the argument when the call is reduced, as `(#req "def" "param" "type" a)`,
  whose value is `a`. The mark reaches wherever the argument does: each use of the parameter, or the
  binding when β binds the argument to a name. A parameter the body never uses carries no
  obligation.
- **A `where`** is marked on the call's result at β, `(#reqw "def" cond body)`, with the arguments as
  β passes them substituted into it. Each reduction rule that inspects a result's shape hoists the mark
  out first.
- The marks are decided immediately after reduction and erased before anything else runs
  (`emit.DischargeRequires`), in order:
  1. a literal, decided when it reduces;
  2. the interval analysis;
  3. this layer, with every fact in scope, including a pure call's `ensures`.

  An obligation none of them proves is refused, naming the call.
- **The obligations of a program are in its residual, and nowhere else**
  ([ADR 0037](../decisions/0037-a-programs-obligations-are-in-its-residual.md)). O(P, T), the domain
  conditions of P's applications on T, is a function of P and T:
  - `Target.Env` installs every contract, the host functions' and the program's, so reduction under
    a target marks every one. There is no environment without them;
  - a literal inside its range is the obligation discharged by evaluation, and leaves nothing. A
    literal outside it **stays as its mark**, and a `where` that reduced to false stays as
    `(#reqw "def" (#false clause) body)`: marks nothing proves, read from the term by whoever
    decides it;
  - **deciding is one procedure**, `ir.DecideMarks`, the three routes above. `ir.Entry` and each
    backend's `FromResidual` call it; the drivers call `emit.DischargeRequires` themselves, to report
    on the way, and then there is nothing left for it;
  - **a consumer that is transparent to a mark requires the marks decided.** `emit.Refine` refuses a
    residual that carries one, and `ir.ToP`, which all four printers come through, refuses a function
    lowered with one.

  Until hazard-2026-10-02 reduction marked only when a driver had installed the tables, and reported
  a failed literal to a callback only that driver held. `build`, `gen` and `intervals` did; any other
  path from a program to emitted text had no obligations.
- **Above the target's word a range denotes a set its enforcement can decide**
  ([ADR 0029](../decisions/0029-above-the-word-one-set-on-every-representation.md)): [0, 2ᵇ) when LO ≥ 0 and (−2ᵇ, 2ᵇ) when LO < 0, b the
  bit length of max(|LO|, |HI|): the least set containing the declaration that a sign and a bit
  length decide. A declared result above the word, carried as `(the T e)`, tells only that its value
  is in the ONE set the program enforces (the join of all its types' sets, since the bound is one per
  program), so it proves a parameter's range only when that range is at least as wide.

### One syntax, two meanings

| on | meaning |
|---|---|
| a `prim` | an obligation, discharged at every call site |
| a definition, called from inside the program | an obligation at every call it is inlined into |
| an **exported** definition, called from outside | a published contract, assumed: the caller is outside the program, exactly what SAL is for a C header. `Refine` assumes it so the body may rely on it |

**Not covered, and named** (ADR 0028): a definition passed as a value and applied later, which is not
a direct call by name. A range with an infinite endpoint keeps its finite side, `(int 0 +inf)` as
`0 <= n`, on the call's condition.

## 7. What this does not do

- **No solver for non-linear arithmetic.** Undecidable over the integers (Hilbert's tenth), and no
  obligation needs it.
- **No quantifiers.** `∀i. a[i] > 0` is a whole-array property; the fragment is per-index.
- **No proofs.** Layer 2 ([types-sketch §7](../types-sketch.md)).
- **No refinement inference.** A `where` is declared, never guessed. Liquid Types infers them by
  Horn-clause fixpoint; that is a larger machine and no program has asked.
- **No enforcement of a precondition that states MEANING.** §6b: a definition whose body is total
  and merely wrong outside its domain has nothing to catch. `win/fmt.print-int` is the instance.
- **Not applied to the integer range yet.** §1's second hole needs `int` literals to carry ranges,
  which touches every arithmetic primitive. One hole at a time.
