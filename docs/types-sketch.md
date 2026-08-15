# What a type would look like

**A sketch, not a specification.** Nothing here is decided, none of it is implemented, and the
syntax is illustrative — it exists so the idea in [types-direction.md](types-direction.md) can be
held in the head and argued with. Read that document for *why*; this one is *what it would look
like*.

---

## 1. The mental model, in one sentence

> **A type is a set. A refinement is a predicate that narrows it. You write them only where data
> crosses a boundary, and the compiler infers them everywhere else — then uses what it knows to
> pick which host function to call.**

Three consequences, and the third is the one that pays:

1. `int` is the set of integers.
2. `{int | (< i n)}` is the subset below `n`.
3. Knowing which subset a value is in lets the compiler **choose a different host function**.

## 2. The worked example: what the 1.94× looks like

Today `examples/dot.oro` reduces to this, and the emitted Go carries a bounds check on `q`:

```lisp
(fold-range 0.0 (alen p) (fn (acc i) (add acc (mul (aindex p i) (aindex q i)))))
```

### The target declares two forms of the same name

```lisp
; targets/go/array.oro
(module array
  ; Always applicable. Go inserts a bounds check.
  (prim get ((v vec-f64) (i int)) f64
    expr "%s[%s]" pure)

  ; Applicable only when the index is PROVEN in range. Emits a loop shape
  ; whose bounds check Go's own BCE eliminates — measured at 1.94x.
  (prim get ((v vec-f64) (i int)) f64
    (where (and (<= 0 i) (< i (alen v))))
    expr "%s[%s]" pure (narrowed)))
```

Two declarations, same name. That is not a mistake — see §5.

### The source does not change at all

```lisp
(def dot (fn (a b) (sum (zip mul (of-array a) (of-array b)))))
```

No annotations. The interior is **inferred**: `fold-range 0.0 (alen p)` binds `i` to
`{int | (and (<= 0 i) (< i (alen p)))}`, which is the loop's own meaning, and the checker
propagates it.

### What the compiler then knows

```
i : {int | 0 ≤ i < (alen p)}          from the loop
(aindex p i)  ⊢  0 ≤ i < (alen p)     discharged directly
(aindex q i)  ⊢  0 ≤ i < (alen q)     NOT discharged — nothing relates alen q to alen p
```

So `p` gets the narrowed form and `q` does not. **That is exactly the bounds check Go leaves in
today**, found by the same reasoning, and it tells the programmer precisely what is missing: a fact
relating the two lengths.

### Which the signature supplies, at the boundary

```lisp
(module num/vec)
(export dot)

(sig dot ((a vec-f64) (b vec-f64)) f64
  (where (= (alen a) (alen b))))

(def dot (fn (a b) (sum (zip mul (of-array a) (of-array b)))))
```

**One line, at the module boundary.** Now `alen q = alen p` is a hypothesis, both obligations
discharge, both accesses select the narrowed form, and the emitter produces the shape Go's BCE
eliminates:

```go
func Dot(a, b []float64) float64 {
	b = b[:len(a)]              // discharges Go's own obligation, once
	acc := 0.0
	for i := range a {
		acc = acc + a[i]*b[i]   // no per-iteration check
	}
	return acc
}
```

`1005 ns/op → 525 ns/op`. And note where the annotation went: **on the export, not in the loop.**

### And the caller now owes the precondition

```lisp
(vec.dot p q)     ; ⊢ (= (alen p) (alen q))  —  or it does not compile
```

Which is the whole "checked once at the door" idea arriving in its useful form: the length
equality is proven **where the arrays are created**, and every loop downstream is free.

## 3. How a programmer extends the predicates

This is the part that replaces sequent calculus, and the trick is that there are **two kinds of
predicate** and only one of them touches the solver.

### Interpreted — the solver reasons about these

Built from a fixed vocabulary: `+ - * <= < = and or not` over integers, and `alen`. This is linear
integer arithmetic, it is decidable, and the compiler knows what every symbol means.

```lisp
(pred in-range ((i int) (lo int) (hi int))
  (and (<= lo i) (< i hi)))          ; a definition, not a rule
```

`in-range` is *sugar*. It expands and the solver sees arithmetic. You have extended the
**vocabulary**, not the **logic**.

### Uninterpreted — the solver propagates these without understanding them

```lisp
(module text/ascii)
(export ascii? check split)

(pred ascii? (string))               ; NO body. An opaque atom.
```

The checker cannot prove `(ascii? s)`. It can only carry it: if one signature *produces* it and
another *requires* it, they match. That is exactly SMT's uninterpreted-function discipline, and it
is what keeps decidability while letting the vocabulary grow without limit.

Facts about opaque predicates enter the world in exactly one place — a function that **checks at
runtime, once**:

```lisp
; The door. The one place the fact is created, and the only place a loop runs
; over every character.
(sig check ((s string)) bool
  (ensures (implies result (ascii? s))))

(def check (fn (s) (fold-chars-all s (fn (c) (< c 128)))))
```

And then everything downstream is free:

```lisp
(sig split ((s string)) vec-string
  (where (ascii? s)))                ; requires the fact
```

```lisp
; targets/go/text.oro
(module text/ascii
  ; General. Correct on any input, and pays for it.
  (prim split ((s string)) vec-string
    expr "strings.Fields(%s)" pure (import "strings"))

  ; Selected only when the string is known ASCII: no rune decoding.
  (prim split ((s string)) vec-string
    (where (ascii? s))
    expr "fastsplit.ASCIIFields(%s)" pure (import "fastsplit")))
```

> **This is the pitch.** You wrote *one* runtime check, at the door. The compiler then chose a
> different host function for every call downstream, because it could prove the precondition. No
> loop contains a check, and nothing was checked twice.

### What you may *not* do

You may not write inference rules. There is no way to say "from `P` and `Q` conclude `R`" for
opaque `P, Q, R`. That is deliberate, and it is the entire difference from Shen: the rules are
fixed so that the compiler retains a meaning for every type, and so that checking terminates.

If you need a fact to follow from another fact, you write a function whose signature says so, and
you prove it the only way this system allows — **by checking it at runtime, once, at a boundary.**

## 4. Comparison

| system | the programmer extends | checking | can the **compiler** use it for codegen? |
|---|---|---|---|
| C, Go | nothing — types are fixed | trivial | a little: layout only |
| ML, Haskell (HM) | type constructors, ADTs | decidable, inferred | layout, not values |
| **Shen** | **inference rules**, in sequent form | **proof search with a depth limit** | **no** — the compiler has no semantics for a user's rules |
| Coq, Agda, Idris | **the whole logic** — dependent types | checking decidable; *finding* proofs is not, so you write them | **no, by design** — extraction **erases** proofs; the OCaml Coq emits is not faster for having been verified |
| Rust | traits, lifetimes | decidable | yes for layout and ownership; not for values |
| Liquid Haskell, F\* | **predicates** over a decidable theory | decidable (SMT) | **yes** — refinements bound values |
| ATS | both, in two stratified languages | decidable | yes — and it is famously hard to use |
| **this sketch** | **predicates**: interpreted, or opaque atoms | decidable; declared at boundaries, inferred inside | **yes — selects the host function and deletes the check** |

The row worth staring at is **Coq**. It has the most powerful system on the list and its extraction
mechanism **throws the proofs away**: verified Coq code is not faster than unverified Coq code,
because the proof was about the program, not about the machine. That is the clearest possible
demonstration that *proving* and *compiling* are different jobs, and it is why "the most powerful
type system" does not imply "the fastest output".

Shen's row is the same lesson from the other direction: enormous expressive reach, and the compiler
cannot act on any of it, because it does not know what a user-defined rule *means*.

## 5. The overload problem turns into the feature

[effects-2026-08-14 §6](../gauntlet/results/effects-2026-08-14.md) recorded that primitive names are
unique keys, so `print-line` had to take `any`, and
[modules.md §9](spec/modules.md) listed overload resolution as unsolved and the first thing
generated target files would break on.

Under this sketch it is not a separate problem. Both `array.get` declarations above are candidates
for the same name, and resolution is:

> **Among the candidates whose `where` clause is provable, choose the one with the strongest
> precondition.**

Decidable, because provability is. Deterministic, because preconditions are ordered by implication.
And it is the *same rule* as `P_T ∩ D` selecting a native over a fallback — one selects by
**availability**, this selects by **provability**. Two mechanisms collapse into one.

## 6. Three mechanisms, not one — a correction

§4 said Coq's extraction throws the proofs away, and used it to suggest that verification does not
make programs faster. That is a true fact supporting a false conclusion, and it missed the
mechanism that matters most in practice.

**Who deletes the check?**

| mechanism | who acts | example | does the compiler need to understand the proof? |
|---|---|---|---|
| **A** the programmer deletes a defensive check | **the programmer** | dropping `if i < 0 then error` because the type forbids it | **no** |
| **B** the programmer chooses a better algorithm | **the programmer** | binary search instead of a scan, because `sorted?` holds | **no** |
| **C** the compiler deletes a host-inserted check | the compiler, by emitted shape | Go's `IsInBounds` | **yes** |

§1–§5 argued only about **C**, which is why the argument came out narrow. **A** and **B** are the
mechanisms behind "move everything statically checkable into the type checker", they are real, and
they are how SPARK, HACL\* and seL4 actually get their performance: not by the compiler exploiting
a proof, but by a human confidently writing code that would be reckless without one.

The division is clean and it is worth stating, because the two need different things:

> **A and B need the *programmer* to understand the predicate. C needs the *compiler* to.**

So a proof system with unbounded, compiler-opaque expressiveness is not useless for performance —
it is fully sufficient for A and B, and only useless for C. And C is the smaller half: on our three
hosts, C reaches bounds checks and representation selection and essentially nothing else
([types-direction §3.4](types-direction.md)).

### The sketch already delivers A and B

This was undersold. An **uninterpreted** predicate (§3) is exactly the tool:

```lisp
(module num/sorted)
(export sorted? sort search)

(pred sorted? (vec-f64))               ; opaque. The solver never reasons about it.

(sig sort   ((v vec-f64)) vec-f64  (ensures (sorted? result)))
(sig search ((v vec-f64) (x f64)) int (where (sorted? v)))

(def search (fn (v x) …binary search, no fallback, no defensive scan…))
```

`search` is a *different algorithm* — O(log n) rather than O(n) — and it is safe to write only
because the type forbids calling it on unsorted input. **The compiler has no idea what `sorted?`
means and does not need one.** It only has to propagate an atom.

That is mechanism B, delivered by the decidable layer, with no proof assistant.

## 7. Where dependent types genuinely win

Being honest about the ceiling, because it is a real one.

The difference is **how a fact enters the world**:

| | facts enter by | cost |
|---|---|---|
| refinements (this sketch) | a **runtime check at a boundary**, once — or an assumption | one pass over the data, at the door |
| dependent types (Coq, Idris, Agda, F\*) | a **proof** | zero runtime cost, ever |

Concretely: this sketch cannot prove that `merge` *preserves* sortedness. `sorted?` is opaque, so
`(sig merge … (ensures (sorted? result)))` is an **assumption** — a conformance obligation exactly
like a target declaring `pure`. Coq proves it, by induction, and then the door check disappears
too.

That is genuinely more powerful and it is what the ambition is reaching for. Two costs, both
measured by other people rather than asserted here:

- **Proof-to-code ratio.** seL4 is roughly 20 lines of proof per line of C. HACL\* is better but
  still multiples. Induction is where SMT stops and a human starts.
- **Staging.** Ours is a partial evaluator: the source is checked, the *residual* runs. Dependent
  types over a staged language is an open research area, not an engineering choice —
  [types-direction §3.6](types-direction.md) names the multi-stage typing literature, and its
  results are for simply-typed and polymorphic staging, not dependent staging.

## 8. The migration path, which is the actual argument for starting small

The reason to begin with the decidable layer is not that it is the ceiling. It is that **it is the
floor of the other one, and nothing written on it has to be rewritten.**

An opaque predicate is precisely a proposition with its proof missing. Today:

```lisp
(pred sorted? (vec-f64))
(sig merge ((a vec-f64) (b vec-f64)) vec-f64
  (where (and (sorted? a) (sorted? b)))
  (ensures (sorted? result)))          ; ASSUMED
```

Later, if a proof layer arrives, the same signature acquires a justification:

```lisp
(proof merge-preserves-sorted …)       ; discharges the `ensures`
```

**The signature does not change. Callers do not change. Emitted code does not change.** What
changes is only whether the `ensures` is believed or proven — which is the difference between a
conformance obligation and a theorem, and exactly the seam
[types-direction §3.5](types-direction.md) drew between layer 1 and layer 2.

So the choice is not "refinements *instead of* dependent types". It is "refinements *first*,
because they are the part that is decidable, implementable now, and load-bearing for the compiler —
and they are where the proofs would attach."

## 9. Where this is ugly, honestly

- **`(sig …)` is new surface**, and surface is the thing that cannot be taken back. It is confined
  to module exports and target declarations, which is the smallest place it can live, but it is not
  nothing.
- **Error messages from a solver are bad.** "Could not discharge `(< i (alen q))`" is precise and
  unhelpful. Liquid Haskell's reputation is mostly this.
- **A missing fact is silent.** It does not fail — it selects the *slower* correct function. That
  is the right default and it means performance can quietly regress. It needs a diagnostic:
  *"selected the checked form because `(< i (alen q))` was not provable"*, in the spirit of
  "no mystery about what is emitted".
- **Opaque predicates are only as good as the door.** `(pred ascii? (string))` with a wrong `check`
  is a miscompilation the checker cannot catch — it is a conformance obligation, exactly like a
  target file that lies about `pure`.
- **None of it helps JavaScript**, which has no bounds checks to delete and no types to select on.
  The whole performance argument is Go and the JVM.
