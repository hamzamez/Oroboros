# Derivation: escaping closures — and whether this is just lambda calculus again

> **⚠ Constrained by measurement, 2026-08-13.** The cost model in §7 is confirmed: Go allocates
> nothing for non-capturing closures and 16 bytes for a capturing one, with a 1.55ns indirect
> call. But §2's identity gains a hard constraint. Go folds `0.1+0.2` to `0.3` at compile time
> and to `0.30000000000000004` at runtime, because untyped constants are arbitrary-precision.
> **If compile-time arithmetic differs from runtime arithmetic, binding-time analysis changes
> the program's answers and partial evaluation is unsound.** Compile-time float arithmetic must
> be exactly IEEE-754 binary64. See
> [baseline C1, U3](../../gauntlet/results/baseline-2026-08-13.md).

Exploration only. No commitments, no ADR.

Not a gauntlet program. This is the one case all five derivations avoided: every function
argument was literal at its call site, so no closure ever formed.

It is also the answer to a theoretical objection raised during the session: *"everything is a
rewrite" looks a lot like "everything is a function" — variables, definitions, applications,
except we return a rule instead of a value.* Those turn out to be the same question.

---

## 1. The objection is correct

Not approximately. Exactly:

- **Lambda calculus is a term rewriting system.** One rule — `((λx.M) N) → M[x:=N]` — plus
  alpha. Rewriting is the general case; lambda calculus is the instance with a single rule.
- **Our rules are lambda abstractions over terms.** `(rule (dot ?a ?b) => body)` binds `?a` and
  `?b` and substitutes. That is λ-abstraction, one level up.
- **The defects are shared because the substrate is shared.** Capture is alpha. The sharing
  problem is why call-by-need and graph reduction exist. Neither was a coincidence, and both
  were predicted by the objection before they were found.

So the honest statement is: **the rewriting core is not an alternative to lambda calculus. It
is lambda calculus, generalized.**

## 2. The distinction is stage, not mechanism

Which raises the real question: if this is lambda calculus, why is it not Shen's wall again?

> **Lambda calculus as a runtime model allocates. The same substitution at compile time costs
> nothing, because it is gone before the program runs.**

**That last clause carries a hidden requirement, and measurement found it.** "The same
substitution" must genuinely be *the same* — compile-time evaluation must give the same answer
as runtime evaluation, bit for bit. Go violates this: `0.1+0.2` folds to `0.3` at compile time
and evaluates to `0.30000000000000004` at runtime, because untyped constants are
arbitrary-precision.

If the core inherited that, **binding-time analysis would be semantically observable** — whether
an abstraction was eliminated would change the program's output — and the whole staging argument
collapses. Closed by [ADR 0009](../decisions/0009-staging-preserves-results.md): compile-time
float arithmetic is IEEE-754 binary64, exactly.

This was found by running a program, not by reasoning about one, and it is the closest anything
has come to killing the candidate.

In KLambda, beta happens at runtime. A closure must be a heap object because the environment has
to survive until application, and application time is unknown. That is the allocation, and it is
unavoidable *in that stage*.

Here, beta happens during rewriting. The residual contains no lambdas — it contains loops,
assignments, and machine scalars. Nothing survives to allocate.

**And the escaping closure is precisely where that fails.** Where compile-time beta cannot
eliminate the abstraction, a lambda survives into the residual, and there we are back in lambda
calculus at runtime, with the allocation. So the theoretical objection and this test are one
thing: *how much of the language falls back to runtime lambda calculus, and what does it cost
when it does?*

## 3. This has a name

The structure being described — a compile-time level that is always eliminated, over a runtime
level that survives — is a **two-level language**, and the compile-time level is a **partial
evaluator**. "Residual" is that literature's own word.

Prior art that should be read before building anything:

- **Partial evaluation** (Jones, Gomard, Sestoft). The residual/static/dynamic vocabulary, and
  the Futamura projections.
- **Binding-time analysis** — the discipline that decides, *statically*, which parts of a
  program are compile-time and which survive. This is the missing piece, and §8 argues it is
  the most valuable thing in this document.
- **MetaML / MetaOCaml, LMS, Terra, Zig `comptime`** — staged languages in production.

One nuance on "we return a rule instead of a value": currently rules rewrite terms to terms.
Rules that *produce rules* — macros producing macros — would make this a full two-level lambda
calculus, with the termination questions that implies. Nothing so far has needed it. Worth not
adding until something does.

## 4. The test

```lisp
;; A — eliminated: literal at the application site
(fn bump-twice ((x i32)) -> i32
  (app (fn (v) (+ v 1)) (app (fn (v) (+ v 1)) x)))

;; B — escapes: the callee is selected at runtime
(fn build-ops () -> (slice (fn (i32) -> i32))
  (slice-of (fn (v) (+ v 1)) (fn (v) (* v 2)) (fn (v) (- 0 v))))

(fn run-op ((ops (slice (fn (i32) -> i32))) (k (int 0 (len ops))) (x i32)) -> i32
  ((at ops k) x))

;; C — escapes, and captures
(fn make-scaler ((f i32)) -> (fn (i32) -> i32)
  (fn (v) (* v f)))
```

## 5. Case A — eliminated

`(rule (app ?f ?x) => (?f ?x))` gives `((fn (v) (+ v 1)) x)`, and beta fires because the
operator is a literal lambda:

```lisp
(+ (+ x 1) 1)
```

No closure, no allocation, no call. This is what all five gauntlet derivations were doing
without naming it.

## 6. Case B — the residual keeps a lambda

```lisp
((at ops k) x)
```

`at` is in the vocabulary, `ops` is a slice, and `k` is a runtime value. The operator position
holds `(at ops k)`, which is **not** a literal lambda, so no beta rule matches. Rewriting halts
with a function-typed term in the residual.

Which yields a result worth stating on its own:

> **Rewriting is its own escape analysis.** A lambda that can be eliminated is eliminated by
> beta. A lambda that survives rewriting *is* the escaping closure. There is no separate
> analysis.

Emission on Go:

```go
func buildOps() []func(int32) int32 {
	return []func(int32) int32{
		func(v int32) int32 { return v + 1 },
		func(v int32) int32 { return v * 2 },
		func(v int32) int32 { return -v },
	}
}

func runOp(ops []func(int32) int32, k int, x int32) int32 { return ops[k](x) }
```

These three capture nothing, so Go allocates them statically — no per-call cost, one indirect
call at application.

## 7. Case C — capture, and what the environment is

```go
func makeScaler(f int32) func(int32) int32 {
	return func(v int32) int32 { return v * f }
}
```

Go heap-allocates the closure, because `f` escapes. **One allocation per `makeScaler` call.**

The environment is exactly the **free variables of the residual lambda** — which is the
lexical-scoping half of the objection, and it is right. Computing free variables is what gives
the environment its layout, so closure conversion needs nothing that rewriting does not already
compute.

**Cost, measured:**

| Go | ns/op | B/op | allocs/op |
|---|---|---|---|
| `BuildOps` — three non-capturing closures | 18–27 | 24 | 1 |
| `MakeScaler` — captures `f` | 14–15 | 16 | 1 |
| `RunOp` — indirect call | 1.55 | 0 | 0 |

`BuildOps`'s single 24-byte allocation is the *slice* of three function pointers; the closures
themselves cost nothing, confirming that non-capturing closures are statically allocated. The
capturing closure allocates exactly its 16-byte environment. Indirect dispatch is 1.55ns.

**Parity:** hand-written Go for a runtime-selected handler allocates the same closure and makes
the same indirect call. We match it, and the cost model above is what a binding-time report
would quote to the programmer (§9).

> **The Shen wall was universality, not the mechanism.** In KLambda *every* function is a
> closure, so every call pays. Here payment is confined to genuine runtime dispatch, and equals
> what the host language charges for the same program.

## 8. `closure` is a capability, and g3's principle needs a bound

[CLAUDE.md](../../CLAUDE.md) says closures are not a core primitive and belong above the core,
lowered by defunctionalization or environment structs. That survives, and the capability graph
already expresses it:

| Target | `closure` |
|---|---|
| Go, JS, Java | Provided natively — halt, emit a host closure |
| C | Not provided — shim via environment struct or defunctionalization |

But [g3](g3-generics.md) said: *never parasitize the host's abstraction mechanisms, because
they pay at runtime while ours pay at compile time.* Here we deliberately parasitize Go's
closures. Is that a contradiction?

No — it bounds the principle:

> g3's rule applies **only where the abstraction is statically resolvable.** Where dispatch is
> genuinely dynamic, we have no compile-time advantage to lose, and the host's mechanism is the
> right answer.

Defunctionalization stays available as a whole-program optimization, but it should not be a core
mechanism: it needs closed-world knowledge of every lambda reaching a call site, which breaks
separate compilation and requires a global control-flow analysis.

## 9. The most valuable thing here: binding-time analysis as a guarantee

Partial evaluation's binding-time analysis decides *statically* whether a given abstraction is
eliminated or survives. Applied here, that is not an implementation detail — it is a
**user-facing feature**, and arguably the strongest one the design has produced:

- "This `fold` is eliminated; the loop is emitted inline with no call."
- "This handler survives as a closure: 1 allocation, 8-byte environment, indirect call."

A language whose pitch is *no performance compromise* can then give a **checkable answer**
rather than a hope. It fits ADR 0003's philosophy — no mystery about what is emitted — and it
directly serves requirement 8, since a model can be told which level a construct lives on and
reason locally about cost.

It also makes the one remaining tension inspectable: g3's specialization-versus-binary-size
problem becomes a number the programmer can see, per abstraction.

## 10. Findings

1. **The objection is correct.** Rewriting is lambda calculus generalized; the shared defects
   follow from the shared substrate and were predicted rather than discovered.
2. **The difference is stage, not mechanism.** Beta at compile time costs nothing because the
   residual holds no lambdas. Beta at runtime allocates.
3. **The escaping closure is exactly where compile-time beta fails**, so the theoretical
   question and this test are the same question.
4. **Rewriting is its own escape analysis** — the surviving lambda *is* the escaping closure.
   No separate pass.
5. **The environment is the free variables of the residual**, which is the lexical-scoping half
   of the objection, and is already computed.
6. **Escaping closures cost exactly what hand-written costs.** Parity holds. Shen's wall was
   universality, not the mechanism.
7. **`closure` is a capability** — native on Go/JS/Java, shimmed on C.
8. **g3's principle is bounded**: never parasitize host abstraction *where it is statically
   resolvable*. Where dispatch is dynamic, parasitize.
9. **This is a two-level language and the compile-time level is a partial evaluator.** There is
   a literature; use it.
10. **Binding-time analysis should be surfaced to the programmer** as a per-abstraction
    guarantee of elimination or a stated runtime cost. Measured numbers to quote: 0 bytes for a
    non-capturing closure, 16 for a capturing one, 1.55ns per indirect call.
11. **Staging must preserve results, and this is not free.** Go's arbitrary-precision constant
    folding would make binding-time decisions semantically observable. See
    [ADR 0009](../decisions/0009-staging-preserves-results.md). Found by measurement, not
    reasoning.

## 11. Verdict

The last place boxing could hide is not a hiding place. Closures survive only where the program
genuinely needs runtime dispatch, and there they cost what the host charges — which is the
standard.

The more useful outcome is that the design now has a name and an existing literature, and the
question *"is this just lambda calculus?"* has a precise answer: **yes, staged so that it
terminates before the program runs — and the residue where it cannot is the only place it
costs anything.**

That reframes candidate B's identity. Not "everything is a rewrite," which is true but says
nothing about why it is fast. Rather:

> **Everything is a function, evaluated at compile time. What survives is what the target must
> do at runtime, and the compiler will tell you exactly what that is.**
