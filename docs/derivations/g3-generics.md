# Derivation: gauntlet program 3 — one generic operation, two instantiations

Exploration only. Nothing here is a commitment, and no ADR follows from it.

The question: does one definition, used at two element types, produce output identical to two
hand-written specializations — with no boxing, no dictionary, and no indirect call?

**Result: it survives, and produces the strongest positive result so far — monomorphization
and higher-order specialization turn out to be the same operation the core already does.** It
also produces the strongest *negative* result: a direct counterexample to
[ADR 0002](../decisions/0002-capability-graph.md)'s central rule.

---

## 1. The references to hit

```go
func sumF64(xs []float64) float64 {
	acc := 0.0
	for i := 0; i < len(xs); i++ { acc += xs[i] }
	return acc
}

func countPositive(xs []int32) int32 {
	acc := int32(0)
	for i := 0; i < len(xs); i++ { if xs[i] > 0 { acc++ } }
	return acc
}
```

Two loops, no shared code, nothing generic. That is the bar.

## 2. The source — one definition, two uses

```lisp
(fn fold (T A) ((xs (slice T)) (init A) (step (fn (A T) -> A))) -> A
  (fold-range init (len xs)
    (fn (acc i) (step acc (at xs i)))))

(fn sum-f64 ((xs (slice f64))) -> f64
  (fold xs 0.0 +))

(fn count-positive ((xs (slice i32))) -> (int 0 2^31)
  (fold xs 0 (fn (n x) (if (> x 0) (+ n 1) n))))
```

One instantiation passes a bare operator, the other a literal lambda. Both are realistic and
they exercise different substitution paths.

## 3. What is a generic function in a rewriting core?

This is the question program 3 exists to answer, and the answer is unexpectedly small:

> **A non-recursive function definition is a rewrite rule.**

```lisp
(rule (fold ?xs ?init ?step) => <body>[xs:=?xs, init:=?init, step:=?step])
```

Type parameters do not appear. Term-level matching is untyped, so `T` and `A` are bound
implicitly by whatever matched — instantiation is a side effect of substitution, not a
mechanism. **No monomorphization pass exists.**

The same is true of the function-valued parameter. `?step` binds to `+` or to a literal lambda,
and substitution puts it in operator position. **No closure conversion pass exists either.**

Monomorphization and higher-order specialization collapse into one thing the core already does.

## 4. Derivation — `sum-f64`

**Step 0.** `(fold xs 0.0 +)` — `fold ∉ go.vocab`, rewrite.

**Step 1.** Substitute `?xs := xs`, `?init := 0.0`, `?step := +`. `?xs` occurs twice on the
right but matched a bare variable, so by the [g1](g1-dot-product.md) refinement it is not
let-bound:

```lisp
(fold-range 0.0 (len xs) (fn (acc i) (+ acc (at xs i))))
```

`(?step acc (at ?xs i))` with `?step := +` becomes `(+ acc (at xs i))` — a metavariable
substituted into operator position, the same move used in g1 §6.

**Step 2.** `fold-range` lowers exactly as in g1:

```lisp
(block
  (let n (len xs))
  (var acc f64 0.0)
  (var i (int 0 n) 0)
  (loop (when (= i n) (break))
        (set acc (+ acc (at xs i)))
        (set i (+ i 1)))
  acc)
```

Emits the reference exactly. Zero allocation, no function value, nothing generic.

## 5. Derivation — `count-positive`, and hygiene stops being hypothetical

**Step 1.** Substitute, with `?step` bound to the literal lambda:

```lisp
(fold-range 0 (len xs)
  (fn (acc i) ((fn (n x) (if (> x 0) (+ n 1) n)) acc (at xs i))))
```

**Step 2.** Beta the inner lambda. `n := acc` (bare variable, no binding); `x := (at xs i)`
(non-trivial, but occurs once, so no binding):

```lisp
(fold-range 0 (len xs) (fn (acc i) (if (> (at xs i) 0) (+ acc 1) acc)))
```

**Step 3.** `fold-range` lowers — and the lowering rule introduces a binder named `n`, while
the user's lambda **also** bound `n`. In [g1 §6](g1-dot-product.md) the collision was
accidental and harmless. Here it is a real collision, and without hygiene the user's `n` would
be captured by the rule's loop bound:

```lisp
(block
  (let n#1 (len xs))                        ; freshened
  (var acc (int 0 2^31) 0)
  (var i#1 (int 0 n#1) 0)
  (loop (when (= i#1 n#1) (break))
        (set acc (if (> (at xs i#1) 0) (+ acc 1) acc))
        (set i#1 (+ i#1 1)))
  acc)
```

Hygiene is now demonstrated rather than predicted.

## 6. A statement/expression impedance mismatch

`(set acc (if (> ...) (+ acc 1) acc))` assigns an `if`-**expression**. Go and Java have `if` as
a **statement**. So emission needs a normalization step — hoist the conditional, push the
assignment into both arms, then recognize `acc = acc` as dead:

```go
if xs[i] > 0 { acc++ }
```

This is administrative normal form, and it is required by every source-language backend, not
just this program. Add it to the machinery list.

## 7. Emission and parity

```go
func countPositive(xs []int32) int32 {
	n := len(xs)
	acc := int32(0)
	for i := 0; i < n; i++ {
		if xs[i] > 0 { acc++ }
	}
	return acc
}
```

`fold` has no runtime existence — no Go generic function, no `func` value, no dictionary, no
call. Both sites are fully specialized. Parity with the references, and zero allocations.

## 8. The counterexample to ADR 0002

ADR 0002's governing rule is **emit at the highest layer the target natively provides.**

Go natively provides both generics and first-class functions. Obeying the rule literally means
emitting:

```go
func fold[T any, A any](xs []T, init A, step func(A, T) A) A {
	acc := init
	for i := 0; i < len(xs); i++ { acc = step(acc, xs[i]) }
	return acc
}
```

This is **slower**, by an indirect call through `step` on every element. Go will not remove it:
the callee is a parameter, and the enclosing function contains a loop, which Go's inliner
generally declines. (Worth confirming in the baseline rather than assuming, alongside the
`mapassign` and BCE claims from g4 and g1.)

So the rule as written is false. The refinement:

> **Parasitize the host's data structures and runtime services. Never parasitize its
> abstraction mechanisms.**

The asymmetry has a cause, which is what makes it trustworthy rather than a patch:

| | Host's version | Ours | Winner |
|---|---|---|---|
| Data structures (`map`, `string`) | Native code, tuned, part of the runtime | A reimplementation | **Host** |
| Abstraction (generics, closures) | Must work at **runtime** — dictionaries, indirect calls | Works at **compile time** — specialization | **Ours** |

A host's abstraction mechanism has to survive separate compilation and dynamic linking, so it
pays at runtime. Ours resolves before emission and pays nothing. Compile time is free, so we
always win that column — and we can never win the first one.

## 9. What is not free

**Recursion breaks the "definitions are rules" trick.** A recursive function as a rule rewrites
forever — its right-hand side contains its own head, at the same layer, so layer stratification
does not apply. Recursive functions must stay as residual functions. Generic recursive
functions therefore need **actual monomorphization**: name mangling and one emitted copy per
instantiation. And polymorphic recursion — a generic function calling itself at a different
type — generates infinitely many instantiations, which is why Rust and C++ both stop at a
recursion limit. It must be banned or depth-bounded.

So generics are free for non-recursive functions and cost a real mechanism for recursive ones.

**Types are not free.** Rewriting makes generics free in *code generation*, not in the front
end. Checking after rewriting means diagnostics point at expanded code, which is fatal for
requirement 8. So polymorphic checking of the *definition* is still required — Hindley-Milner
style inference, or explicit parameters with a checking pass.

**Code size is a live tension.** Two instantiations, two emitted loops. Fifty uses, fifty loops
— the Rust/C++ binary bloat problem, against requirement 6. One idea worth exploring: dedupe
identical residual bodies after rewriting, which recovers some sharing where instantiations
coincide. Not a decision.

**Escaping closures are untested.** No closure formed here because every function argument was
literal at the call site. A closure that is returned or stored still needs defunctionalization
or an explicit environment struct. Program 3 does not exercise it, and something should.

## 10. Findings

1. **A non-recursive function definition is a rewrite rule.** Generics need no mechanism —
   instantiation is a side effect of matching.
2. **Monomorphization and higher-order specialization are the same operation**, and it is one
   the core already performs. The strongest support yet for candidate B's identity claim.
3. **ADR 0002's rule is false as stated.** Parasitize data structures and runtime services;
   never abstraction mechanisms. The host's abstractions pay at runtime; ours resolve at
   compile time.
4. **Hygiene demonstrated, not predicted** — a real binder collision, not g1's near-miss.
5. **Statement/expression normalization (ANF) is required** by every source-language backend.
6. **Recursive generics need real monomorphization**, with polymorphic recursion banned or
   bounded.
7. **Polymorphic type checking is still required** for diagnostics, whatever codegen does.
8. **Code size versus specialization is unresolved**, and requirements 5 and 6 pull opposite
   ways.
9. **Escaping closures remain untested.**

## 11. Verdict

Program 3 was flagged as the remaining structural risk. It is not one — it is the best result
of the three derivations, because the mechanism it was supposed to require turns out not to
exist.

The cost is that ADR 0002 needs revising, and the machinery list grows again:

> auto let-binding · layer stratification · linearity analysis · hygiene · range analysis with
> `require` facts · deforestation measure check · ANF normalization · monomorphization for
> recursive generics · polymorphic type checking

**Still untested and worth doing before anything is built:** escaping closures, and program 2
(struct layout), which is the only remaining place boxing could still be hiding.
