# 5. Types

Four chapters in, and nothing has had a type. `fn` takes any argument, `def` names any term, β and
δ never ask what anything is. That was not an oversight — the calculus genuinely does not need
types to reduce.

This chapter is about the checker that exists anyway, and it starts from an uncomfortable place:
**for most of this project's life, whether a type error was caught depended on which target you
compiled for.** Not "was caught late". Was caught *at all*.

Two things about it are unusual enough to say up front:

- **It is not in the language.** No annotations on `fn`, no types on terms. It runs after
  reduction, over the residual, and the language has no idea it is there.
- **It checks a claim in two directions**, and one of those directions is a job no host compiler
  can do, because the two things being compared live on different targets.

Everything below runs. Chapters 1–4 used `oro`, which only reduces; a type checker exists at the
*emission* boundary, so this chapter uses the commands that emit:

```bash
go run ./cmd/gen -path lib FILE.oro go OUT.go
go run ./cmd/build -target=go -o prog FILE.oro
```

Later fragments show only the lines under discussion; each still needs its `(use num/f64)`,
`(use num/int)` and so on, exactly as [chapter 3](03-modules.md) requires.

That change of command is itself the first lesson. There is no `oro -typecheck`, because until you
are emitting there is nothing a type is *for*.

---

## 5.1 The measurement that made the case

```lisp
(use num/f64)
(use io)
(export main)
(def main (fn () (io.print-line (f64.add "hello" 1.0))))
```

Adding a string to a float. Reduction is perfectly happy:

```bash
go run ./cmd/oro -target=go FILE.oro
```

```lisp
main =
(fn () (io.print-line (num/f64.add "hello" 1.0)))
```

Of course it is — β and δ are untyped, and `num/f64.add` is just a name reduction stops on. Now
emit it, on each of the three targets, *before this checker existed*:

| | |
|---|---|
| Go | build fails, with the error pointing at **generated** code |
| Java | build fails |
| **JavaScript** | **builds, runs, prints `hello1`** |

Three targets, three different amounts of safety, from one source file. That is the exact opposite
of what this project claims to sell, and it is the whole argument for a checker of our own — a
**measurement**, not a principle. There is no host to delegate to when one of your hosts is
JavaScript.

Today, all three say the same thing:

```
gen: gen-main: in argument 1 of io.print-line: in argument 1 of num/f64.add:
     a string literal is string, but f64 is required here
```

One checker, three targets, including the one with no type layer at all.

> A bug from building it is worth recording. The first version skipped `KFn`, reasoning that a bare
> abstraction is an escaping closure the emitter rejects anyway. But **the top-level term of every
> residual is an abstraction** — so it checked nothing, and passed the very program it was written
> for. A checker that accepts everything is indistinguishable from a correct one until you have a
> case it must reject.

## 5.2 What is checked: the residual

> **The residual, before emission.** Not the source.

This is the choice that makes the whole thing small, and it is worth seeing why.

By the time reduction finishes, the term is:

- **monomorphic** — every generic definition has been specialised by β and δ. Chapter 3's
  `accumulate` instantiated at `+` and at `*` produced two *different residuals*, each with one
  arithmetic in it. There is no polymorphism left to check.
- **first-order** — every backend refuses an escaping closure, so no function is being passed
  around at runtime.
- **closed** — every name is a parameter, a primitive, or gone.

So the checker needs **no type schemes, no generalisation, and no unification**. It is a walk. The
whole of Hindley–Milner — the algorithm every ML-family language is built on — exists to handle
exactly the three things reduction has already removed.

That is not a claim that types are easy. It is a claim about *where* we put the checker: after the
hard part has already been done for a different reason.

## 5.3 The judgement, in one table

A type is one of the target's declared type names, or **unknown**.

```
type(integer literal)  = int
type(float literal)    = f64
type(string literal)   = string
type(name)             = whatever has been demanded of it, or unknown
type((p a…))           = p's declared result type
```

At every primitive application each argument is **demanded** to have the primitive's declared
argument type. Four cases:

| the argument's type | what happens |
|---|---|
| equal to the demand | fine |
| **unknown**, and it is a name | the name is **bound** to the demand — this is the inference half |
| unknown, otherwise | fine; nothing learned |
| a **different** concrete type | **error** |

Two parameters typed purely by use:

```lisp
(use num/f64)
(export f)
(def f (fn (a b) (f64.add a b)))
```

```
wrote out.go
```

`a` and `b` were unknown; `num/f64.add` demanded `f64` of both; both are now `f64`. No annotation
anywhere.

Demand the same name twice, differently, and the second demand is the error:

```lisp
(use num/f64)
(use num/int)
(export f)
(def f (fn (a) (f64.add a (int.add a 1))))
```

```
gen-f: in argument 2 of num/f64.add: in argument 1 of num/int.add:
       a is f64, but int is required here
```

> **Inference and checking are the same walk.** The emitters had always inferred this way and kept
> the first answer they found. The only change the checker makes is that a second, *different*
> answer is an error instead of being quietly discarded.

There is **no implicit conversion**:

```lisp
(def f (fn (a) (f64.add a 1)))     ;; an integer literal is int, but f64 is required here
(def f (fn (a) (f64.add a 1.0)))   ;; fine
```

`1` and `1.0` are different literals of different types, in a language where
[compile-time arithmetic must be bit-identical to runtime](../decisions/0009-staging-preserves-results.md).
Silent numeric coercion is exactly how that promise gets broken.

And `any` demands nothing, so it never conflicts and never binds:

```lisp
(use io)
(export f)
(def f (fn (a) (io.print-line a)))
```

```
wrote out.go
```

## 5.4 The structural primitives

`if`, `fold-range`, `let` and `make-vec` are implemented in the backend rather than declared in a
target file ([chapter 3](03-modules.md) explained why targets are data — these are the exception),
so the checker knows their types the same way the emitter does.

**A conditional's branches must agree:**

```lisp
(def f (fn (a b) (if (f64.gt a b) a 1)))
```

```
gen-f: the branches of a conditional are f64 and int
```

**And its condition must be `bool`:**

```lisp
(def f (fn (a b) (if 1.0 a b)))
```

```
gen-f: in a condition: a float literal is f64, but bool is required here
```

**A loop's bound is `int`, its index is `int`, and its body must match its accumulator:**

```lisp
(def f (fn (n) (fold-range 0.0 n (fn (acc i) (f64.add acc i)))))
```

```
gen-f: in a loop body: in argument 2 of num/f64.add: i is int, but f64 is required here
```

The accumulator starts at `0.0`, so the body must produce `f64`; `i` is the index, so it is `int`.
Both facts come from the loop itself, not from any table.

## 5.5 `sig` — a claim, checked in two directions

Everything so far was inferred. `sig` is the one place you *state* something:

```lisp
(sig dot ((p vec-f64) (q vec-f64)) f64)
```

It attaches to a definition, and it is checked against **both** implementations of that name.

### Direction one — against the definition

```lisp
(use num/f64)
(export f)
(sig f ((a vec-f64) (b vec-f64)) int)
(def f (fn (a b) (f64.mul (aindex a 0) (aindex b 0))))
```

```
gen: f: num/f64.mul is f64, but int is required here
```

The parameters take their declared types and the body must produce the declared result. Arity is
checked too:

```lisp
(sig f ((a vec-f64)) f64)
(def f (fn (a b) (f64.add (aindex a 0) (aindex b 0))))
```

```
gen: f takes 2 argument(s), but its signature declares 1
```

### Direction two — against the target

This is the interesting one.

[examples/dot.oro](../../examples/dot.oro) declares `(sig dot ((p vec-f64) (q vec-f64)) f64 …)` and
defines `dot` in terms of `num/vec.dot`. On a plain target, `num/vec.dot` is the library's
definition and unfolds into a loop. On `blas` it is a **primitive the target provides**, declared
in `targets/blas.oro`:

```lisp
(module num/vec
  (prim dot (vec-f64 vec-f64) f64 expr "cblas_ddot(%s.n, %s, 1, %s, 1)" pure))
```

Break that declaration — change `vec-f64` to `int` in the target file — and:

```
gen: num/vec.dot: argument 2 is int in target blas, but vec-f64 in its signature
```

**No host compiler can produce that message.** The library's definition and the target's native
declaration are two implementations of one claim, and they live on *different targets*. A Go
compiler sees the Go one. A C compiler sees `cblas_ddot`. Nothing sees both — except the compiler
that owns the substitution, which is this one.

That is chapter 3's four cells becoming machine-checked. Until this existed, the only evidence
that a target's native implementation agreed with the library's was a conformance suite that ran
the code — and [chapter 3 §3.10](03-modules.md)'s whole mechanism rests on them agreeing.

### Parameters are named, and it matters

```lisp
(sig dot ((p vec-f64) (q vec-f64)) f64)     ;; not (vec-f64 vec-f64)
```

Names were added before anything read them, because a refinement attaches to a **name** —
`(where (int.eq (alen p) (alen q)))` — and adding names later would change the one thing that
cannot be taken back. §5.7 is what they are for.

### And the claim is used, not just checked

```lisp
(use num/f64)
(export f)
(sig f ((n int)) f64)
(def f (fn (n) (fold-range 0.0 n (fn (acc i) (f64.add acc 1.0)))))
```

```go
func GenF(n int64) float64 {
	acc := 0.0
	var n1 int64 = n
	for i := int64(0); i < n1; i++ {
		acc = (acc + 1.0)
	}
	return acc
}
```

Drop the `sig` and the same program does not compile:

```
gen: cannot determine a Go type for parameter "n"; it is never passed to a primitive
     whose signature would fix it
```

`n` is used only as a loop bound, and a loop is a structural primitive with no table entry, so use
alone never types it. This is the shape a signature exists for — and until this chapter was
written, the signature was **checked and then thrown away**: the compiler verified the claim and
then refused the program anyway. Two inference passes, and only one of them had been told.

## 5.6 What it deliberately does not do

- **No polymorphism.** The residual has none left. The *source* does — chapter 3's `accumulate` is
  genuinely generic — and checking source would need the whole ML machine. We check after
  specialisation instead, which is why one instantiation being wrong is caught and the general
  definition is never typed at all.
- **No inference of function types.** There are no function types; there are no functions left.
- **No proofs.** §5.7 checks refinements; it does not carry evidence.
- **Nothing in the language.** You cannot annotate a `fn`. `sig` attaches to a definition and is
  the entire surface.

And one rule the checker had to satisfy before it could ship, which is more interesting than the
negative tests:

> **It must reject nothing that was already correct.** Every example, every gauntlet program, every
> generated file, unchanged.

That held — and it was a live risk, because a target file is a *claim* about a host and this
project has had three false ones: `fold-range`'s declared type was wrong for months, `stmt`'s
result type was never implemented, and `loop2`'s was right only by luck. A checker is also a test
of the target files.

---

## 5.7 Refinements: the type that carries a proposition

Now the other half, and the part with the war story.

A type says `int`. A **refinement** says `int, and it is in bounds`:

```lisp
(prim aindex ((v vec-f64) (i int)) f64 expr "%s[%s]" pure index
  (where (logic.and (int.le 0 i) (int.lt i (alen v)))))
```

That is `targets/go.oro`. Reading an array is Tier 1 — portable, same meaning everywhere — **only
within bounds**. Outside them, Go panics, Java throws, and **JavaScript silently returns
`undefined`**. It is the one place in the language where a portable primitive is portable
*conditionally*, and the condition was unchecked.

### The obligation, discharged

```lisp
(use num/f64)
(export f)
(sig f ((a vec-f64)) f64)
(def f (fn (a) (fold-range 0.0 (alen a) (fn (acc i) (f64.add acc (aindex a i))))))
```

```
wrote out.go
```

The loop is bounded by `(alen a)`, so `0 ≤ i` and `i < alen a`, so `(aindex a i)` is in bounds. The
facts come from the loop, and the loop was already there.

### The obligation, not discharged

```lisp
(sig f ((a vec-f64) (k int)) f64)
(def f (fn (a k) (aindex a k)))
```

```
gen-f: aindex requires -k <= 0, which does not follow
  known: nothing
```

`k` is any integer. Nothing says it is in bounds, and the checker will not pretend otherwise. Say
it and the program compiles:

```lisp
(sig f ((a vec-f64) (k int)) f64
  (where (logic.and (int.le 0 k) (int.lt k (alen a)))))
(def f (fn (a k) (aindex a k)))
```

```
wrote out.go
```

The obligation moved to the caller, which is what a precondition is.

### The bug it found

This is the one worth the whole feature. Here is a dot product — two arrays, one loop:

```lisp
(use num/f64)
(export f)
(sig f ((a vec-f64) (b vec-f64)) f64)
(def f (fn (a b)
  (fold-range 0.0 (alen a) (fn (acc i) (f64.add acc (f64.mul (aindex a i) (aindex b i)))))))
```

```
gen-f: aindex requires -alen(b) + i + +1 <= 0, which does not follow
  known: 0 <= i, i < alen(a)
```

Read the `known:` line. We know `i < alen a`. We are indexing **`b`**. Nothing relates the two
lengths.

**This was a real, latent bug in this repository**, in `dot` and in `centroid`, written by someone
who knew exactly what they were doing and never noticed. Pass two arrays of different lengths and
you get a panic on Go, an exception on Java, and `undefined` quietly poisoning an arithmetic result
on JavaScript.

The fix is one line, and it is the reason parameters are named:

```lisp
(sig f ((a vec-f64) (b vec-f64)) f64
  (where (int.eq (alen a) (alen b))))
```

```
wrote out.go
```

`(alen a) = (alen b)` is recorded as a *substitution*, so the obligation about `b` is discharged by
the fact about `a`. [examples/dot.oro](../../examples/dot.oro) carries that clause today, and its
comment says why.

The same shape catches a window walking off the end:

```lisp
(def f (fn (a) (fold-range 0.0 (alen a) (fn (acc i) (f64.add acc (aindex a (int.add i 1)))))))
```

```
gen-f: aindex requires -alen(a) + i + +2 <= 0, which does not follow
  known: 0 <= i, i < alen(a)
```

Bound the loop correctly — `(int.sub (alen a) 2)` — and `i + 2 < alen a` follows. That is
[examples/smooth.oro](../../examples/smooth.oro)'s stencil, and it passes.

### Where facts come from

There are only five sources, and none of them is a proof you have to write:

| | fact |
|---|---|
| `(fold-range z n f)` binding `i` | `0 ≤ i`, `i < n` |
| `(make-vec n f)` binding `i` | `0 ≤ i`, `i < n` |
| a `let` binding `x` to a linear `e` | `x = e` |
| the enclosing `sig`'s own `where` | assumed |
| `alen`, `slen` | `0 ≤ alen v` |

The last is free and worth having: a length is never negative on any target.

## 5.8 Classify, do not restrict

The decidable fragment is **linear integer arithmetic over difference constraints**, which is what
every bounds obligation actually is. So what happens when you write something outside it?

Not a syntax error:

> **Any boolean term may appear in a `where`. A term inside the fragment is *proven*. A term
> outside it is an *opaque atom* — propagated and matched by name, never decided.**

```lisp
(sig f ((a vec-f64) (k int)) f64
  (where (ascii? k)))
(def f (fn (a k) (aindex a k)))
```

```
gen-f: aindex requires -k <= 0, which does not follow
  known: assumed (ascii? k)
```

The assumption is *kept and reported* — it just cannot decide a linear obligation, so the program
is refused. It is never assumed true, and it never silently discharges anything.

Three consequences, all wanted: nothing is rejected for being too expressive; it is always sound;
and the fragment can grow later without any program changing.

> Until this chapter, that assumption was **dropped**, and the diagnostic said `known: nothing`
> while the source plainly declared a `where`. The spec said "propagated and matched by name"; the
> code recognised the linear shapes and silently returned for everything else. Writing down what a
> system does is not the same as it doing it.

There is also **no predicate syntax**. A refinement is an ordinary boolean term — the predicate
language is a *fragment of the term language*. `logic.and`, `int.le`, `int.eq` are the same names
a program uses. That is why nothing had to be added to the reader for any of this.

### Deliberately incomplete

The entailment check is not Fourier–Motzkin and is not an SMT solver. Facts are normalised to
`e ≤ 0`; an obligation is discharged if, after substituting known equalities, a *single* fact
implies it.

That is much weaker than the literature. **Presburger arithmetic** — integer linear arithmetic with
quantifiers — is decidable, a result from 1929, and there are excellent solvers for it. Liquid
Types (Rondon, Kawaguchi & Jhala, 2008) go further and *infer* refinements by Horn-clause fixpoint;
Dependent ML and F\* go further still.

We take almost none of it, on purpose:

- **Incomplete is safe here** because an undischarged obligation is **reported, never assumed**. The
  failure mode of a weak solver is a false alarm you fix by writing a `where`. The failure mode of
  an unsound one is a silent `undefined` on JavaScript.
- **No refinement inference.** A `where` is declared, never guessed. Liquid Types' fixpoint is a
  much larger machine and no program here has asked for it.
- **No quantifiers.** `∀i. a[i] > 0` is a whole-array property; every obligation this language
  generates is per-index.
- **No non-linear arithmetic.** Undecidable over the integers — Hilbert's tenth problem — and
  nothing needs it.

The rule is: build the smallest thing that decides the obligations this language actually
generates, and make being wrong about the boundary *safe*.

### One thing that is opaque by necessity

`num/f64.eq` is not usable as an equality fact, and not for want of effort. **IEEE-754 equality is
not reflexive** — `NaN ≠ NaN` — so it is not an equivalence relation, and a solver that treated it
as one would be unsound about float comparisons in a language whose
[staging rules](../decisions/0009-staging-preserves-results.md) are built on IEEE semantics being
exact. It is an opaque atom forever.

---

## 5.9 The measurement that limits all of this

A closing result, because it is the one that decides how much a type system here can ever be worth.

You might expect proving `i` in bounds to *remove* the bounds check. It does not:

> **Our proofs do not transfer.** We emit host source. Go's bounds-check elimination runs on Go's
> own analysis and has never heard of us. A theorem in our checker removes exactly zero
> instructions.

The only thing that works is emitting a **shape the host re-proves for itself**:

```go
func GenDot(p []float64, q []float64) float64 {
	acc := 0.0
	var n1 int64 = (int64(len(p)))
	p = p[:n1]        // ← this
	q = q[:n1]        // ← and this
	for i := int64(0); i < n1; i++ {
		acc = (acc + ((p[i]) * (q[i])))
	}
	return acc
}
```

Two assignments, and Go's own BCE now proves what we already knew. **Worth 1.96× on compute-bound
loops — and nothing at all on memory-bound ones**, which is the condition an earlier write-up left
off and which matters more than the number.

Three things about that are worth carrying away.

**It needed no types.** The pattern is declared as data — one `(narrow "%s = %s[:%s]")` line in
`targets/go.oro`, plus `index` on `aindex` — and a target that declares no `narrow` gets no
transformation. Which is correct for JavaScript (no bounds checks to remove):

```javascript
export function genDot(p, q) {
	let acc = 0;
	const n1 = (p.length);
	for (let i = 0; i < n1; i++) {
		acc = (acc + ((p[i]) * (q[i])));
	}
	return acc;
}
```

**It is conservative by construction.** A container is narrowed only if *every* occurrence of it in
the loop body is indexed by the bare loop variable. The stencil indexes `a` at `i`, `i+1`, `i+2`,
so `a` is left alone — correctly, since `a = a[:n]` would break `a[i+2]`:

```go
func GenSmooth(a []float64) []float64 {
	var n1 int64 = ((int64(len(a))) - 2)
	v2 := make([]float64, n1)
	for i := int64(0); i < n1; i++ {
		v2[i] = ((((a[i]) + (a[(i + 1)])) + (a[(i + 2)])) / 3.0)
	}
	return v2
}
```

**And it settles what the refinement checker is for.** Not speed — the speed was already collected
without types. The checker is a **correctness deliverable**: it closes the hole where a Tier 1
primitive was Tier 1 only conditionally, and it found a live bug in two programs on its first run.

The general principle is worth more than the number:

> **A proof you can state and no host can re-derive is decoration.** On a compiler that emits
> source, the useful theorem is the one you can shape the output around.

Still open, and named rather than hidden: the *other* refinement-shaped hole. `int` is exact within
±(2⁵³−1), and outside it Go and the JVM wrap while **JavaScript silently rounds**. Closing that
needs integer literals to carry ranges, which touches every arithmetic primitive.
[One hole at a time.](../spec/refinements.md)

---

## What to remember

- **The checker is not in the language.** No annotations, no types on terms. It runs on the
  residual, at the emission boundary, and `oro` never invokes it.
- **It exists because of a measurement**, not a principle: the same bad program failed on Go, failed
  on Java, and printed `hello1` on JavaScript.
- **Checking the residual is cheap** because reduction already made it monomorphic, first-order and
  closed — the three things Hindley–Milner exists to handle.
- **Inference and checking are one walk.** A second, different demand on a name is the error.
- **`sig` is a claim checked in two directions**, and the direction no host compiler can check —
  library definition against target primitive — is the one that keeps chapter 3's four cells honest.
- **Parameters are named because refinements attach to names.**
- **A refinement is an ordinary boolean term.** No predicate language, no new syntax.
- **Classify, do not restrict.** Outside the fragment a term is opaque: kept, reported, never
  decided, never assumed.
- **Incompleteness is safe; unsoundness is not.** An undischarged obligation is reported.
- **Our proofs do not transfer.** The emitted shape has to let the host re-prove them, and that
  pattern needed no type system at all.

Next chapter: targets — the file a third party writes, and the only thing standing between this
language and a host it has never heard of.
