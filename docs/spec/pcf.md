# What the core actually is: PCF with a parameterised constant set

This supersedes [core-0 §0](core-0.md), which claimed three departures from λ-calculus. **All
three are wrong.** Recording the correction, and the algebra questions it answers.

---

## 1. The three departures, refuted

| Claimed departure | Verdict |
|---|---|
| **Literals** are primitive terms, because Church numerals would allocate | **Wrong.** λ-calculus *with constants* is standard — applied λ-calculus, Church's own. Reducing `(add 2 3) → 5` is precisely what Barendregt calls δ-reduction. The name was already borrowed while the thing it names was called a departure. |
| **Multi-argument λ**, because currying builds intermediate closures | **Wrong.** Currying is an isomorphism, `(A×B→C) ≅ (A→B→C)`. A representation choice with no semantic content. |
| **δ on definitions** instead of the **Y** combinator | **Wrong, and the correction is the useful one.** See below. |

### The third one properly

`(def f t)` where `t` mentions `f` is a recursive equation whose meaning is the **least fixed
point**. That is `fix`, taken as primitive rather than encoded.

**λ-calculus + constants + `fix` = PCF** (Plotkin, 1977). One of the most studied calculi there
is.

And the stated *reason* was wrong too. "Y allocates" is a performance claim. The real obstacle is
mathematical and sharper: **`Y f` has no normal form.** A calculus whose semantics is *reduce to
normal form* cannot express general recursion by encoding it — so `fix` must be primitive, and
must not be unfolded.

Which is exactly what `markRecursive` does, and that has a name too: the **unfolding strategy**
problem in partial evaluation. "Do not unfold recursive calls" is the standard answer
(Jones, Gomard & Sestoft).

## 2. So what is it

> **PCF, reduced to normal form at compile time, with the constant set as a per-target
> parameter.**

Every word has a literature behind it. **Nothing in the mathematics is new.**

What is new is architectural: making Σ a *per-target* parameter, and treating the resulting
normal form as the compilation output. A name that is a constant on one target and a defined
function on another is not a different calculus — it is the same calculus instantiated twice.

That is a smaller claim than "a new core" and a truer one, and it is the one worth defending.
It also means the whole body of PCF results — confluence, standardisation, adequacy — applies
directly rather than having to be redone.

## 3. Reduction order: the let discipline is call-by-need

| Strategy | Argument reduced | Result shared |
|---|---|---|
| Normal order — what `core/reduce.go` does today | once **per copy** | no |
| Applicative order | once | no |
| **Call-by-need** | **once** | **yes** |

[concerns.md §1.1](concerns.md) describes a "let-binding discipline". That description named a
mechanism without naming the thing: **it is call-by-need**, and the `let` node is Wadsworth's
sharing node made syntactic.

### The generalisation

Sharing a **closed value** gives a `let`. Sharing a **term with free variables** gives an
extracted function, called from each site. Same operation, two granularities:

```
shared closed value       →  let
shared term with holes    →  outline a function
```

Which one wins is already measured — [the size baseline](../../gauntlet/results/size-2026-08-13.md)
says *outline above the host's inlining budget, specialize below it*.

### Where the boundary goes

- **Call-by-need is core.** It is a reduction strategy; it changes the normal form.
- **Outlining is residual-layer.** It is a size/speed tradeoff on already-normal terms, with a
  measured answer.

Different machines, different times, and the split matches [q5](q5-do-we-need-rules.md)'s
source→residual versus residual→residual line.

## 4. The algebra of `def`

### Can a `def` contain `def`s?

```lisp
(def f
  (def n 1)
  (def m 1)
  (add n m))
```

Does not parse today; `def` takes one term. It is a **local `letrec`**, and it is **derivable**:
lambda-lift the inner definitions to top level with fresh names, passing any captured variables
as extra parameters.

**Decision: sugar, not core.** Flat top-level `def` remains the core. Nesting is a surface
convenience whose desugaring is lambda lifting, and whose cost is exactly closure conversion —
already priced in [g6](../derivations/g6-escaping-closures.md).

### Can a `def` call itself?

```lisp
(def f (f))
```

**Mathematically** this is `f = f`, whose least fixed point is **⊥** — divergence. A
well-defined denotation, not an error.

**Operationally** `markRecursive` marks `f`, δ never fires, `(f)` survives into the residual, and
the emitter produces a target function that calls itself. An infinite loop, which is the correct
compilation of ⊥.

**Bug found by asking this.** `Residual` reports any free name that is not primitive, so it flags
`f` — but `f` legitimately survives as a target function. The check must accept *primitive **or**
recursive definition*.

## 5. Do we need a macro system?

**No, and the reason is structural.**

Lisp needs macros because evaluation happens at runtime, so a second mechanism is required to
compute at compile time. Here **everything** is compile time — so a macro and a function are the
same object, and **δ+β *is* macro expansion.**

The Lisp insight, inverted: Lisp has macros because it has a runtime; a fully staged language does
not need them.

The rewrite rules of [q5](q5-do-we-need-rules.md) are therefore *not* a macro system. Macros
expand source **before** evaluation; those rules act on the residual **after** reduction. They are
peephole optimisation, they sit above the core, and they cover only residual→residual — exactly
where q5 put them.

## 6. What this changes in the documents

| Document | Change |
|---|---|
| [core-0 §0](core-0.md) | **Wrong.** The three-departures table is replaced by the PCF identification. |
| [the-atom.md](../the-atom.md) | Framing survives — "normal form as a parameter" is still the contribution — but "three constructs and one parameter, as small as λ-calculus" should say **PCF with Σ as a parameter**. The novelty is architectural, not mathematical. |
| [concerns.md §1.1](concerns.md) | Rename the concern: it is call-by-need, not a bespoke discipline. Add the outlining generalisation. |
| [q5](q5-do-we-need-rules.md) | Unaffected. The δ / rules split holds, and §5 now also answers "is this a macro system" — no. |
| `core/reduce.go` | `Residual` must accept recursive definitions as legitimate survivors. |

## 7. What was right, and worth keeping

Worth separating from the corrections, because the record should show both:

- **The parameterised normal form.** Still the contribution, still executable, still one word of
  target declaration between a BLAS call and a loop.
- **Fusion by δ+β with no fusion rules.** Predicted on paper in q5, confirmed by running code.
- **The recursion side condition**, derived from termination and confirmed to be the standard
  partial-evaluation answer.
- **Every measurement.** None of this touches the gauntlet.

## 8. What was wrong, and why it kept happening

Three framings have now been corrected: "a vocabulary" → the atom; "three departures from λ" →
PCF; "a bespoke let discipline" → call-by-need.

The pattern is the same each time: **an existing, well-named thing was described as if it were
new.** That is worth watching for directly, because it inflates the apparent novelty of the
project and — worse — it forfeits the literature. Calling `fix` a departure meant not reaching
for PCF's results. Calling call-by-need a discipline meant not reaching for Wadsworth.

The corrective is cheap: before describing something as new, name the closest existing thing and
say what the difference is. If the difference is empty, use the existing name.
