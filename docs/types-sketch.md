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

## 6. Where this is ugly, honestly

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
