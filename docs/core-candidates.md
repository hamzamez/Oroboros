# Core candidates

Status: **open exploration**. Nothing here is decided. Candidates are added, tested against
[the gauntlet](gauntlet.md), and killed with a recorded reason.

---

## 1. The axis problem

The instinct behind this document is: small languages with real identity have a tiny, powerful
core, and everything is built on it. Lisp has lambda calculus — functions, application,
variables, and that is Turing complete. Smalltalk has objects and messages. Forth has the
stack and words.

That instinct is correct about identity and dangerous about performance. Look at what each
core cost:

| Core | Bought | Then required to be fast |
|---|---|---|
| Lambda calculus | Three constructs, Turing complete | Closure conversion, lambda lifting, defunctionalization, strictness analysis, unboxing |
| Objects + messages | Total uniformity | Maps, polymorphic inline caches, dynamic deoptimization — the Self → StrongTalk → HotSpot lineage |
| Stack + words | Trivial implementation | Subroutine threading, then native compilation; threaded code is an interpreter |

Each of these cores spawned a decades-long research program whose purpose was to undo the
core's own elegance.

The reason is that all three are minimal along one specific axis: **fewest constructs needed
to express all computation.** That is a real property. It is not the property this project
needs, and it is frequently opposed to it. Lambda calculus is minimal *because* it makes
everything a function — which is exactly why it allocates. The elegance and the heap
allocation are one fact, not two.

This is not hypothetical. Shen's core is KLambda, KLambda is lambda calculus, and the
performance wall was the bill. **Searching for a beautiful minimal core along the same axis
will produce another one that fails the same way.**

The counter-example: WebAssembly's core is not minimal in this sense — roughly 170
instructions, typed, structured control flow, nobody calls it elegant. It reaches near-native
speed and lowers to everything. It was minimized *subject to lowering well*, not minimized
absolutely. C is the same story with less taste.

**So: minimal subject to a constraint, not minimal absolutely.** The constraint is that the
core must be natively expressible by every target and must accept every abstraction without
cost.

> **⚠ §2's framing corrected by [s6](derivations/s6-rule-legibility.md).** "Everything is a
> rewrite" does not survive contact with an implementation. The compiler is a hybrid: pattern
> matching for *dispatch*, imperative construction for *output*, and several ordinary
> whole-function analyses that are not rewrites at all — five of the eight machinery items. The
> honest version is **"lowering is pattern-directed; analysis is not."** The *semantic* identity
> from [g6](derivations/g6-escaping-closures.md) and [s1](derivations/s1-substructural.md) —
> staged, graded lambda calculus — is untouched, and is a claim about what the language means
> rather than how the compiler is built.

## 2. Where identity can come from instead

If the identity cannot live in the computation mechanism without paying for it, it can live in
the **lowering mechanism** — which is this project's actual novelty. No existing language's
identity is "choose how far down to lower, per target, to exploit each ecosystem maximally."

Stated as a core: **everything is a rewrite.** That single mechanism would cover macros,
lowering between layers, targeting, optimization including fusion, pattern matching, and the
capability graph. Six features, one mechanism — the same economy the classic cores have, but
counted in mechanisms rather than constructs. And the runtime cost is nil, because rewriting
happens at compile time; what runs is the residual.

---

## 3. Candidate A — Imperative core

**Identity:** none. This is the baseline every other candidate must beat.

```lisp
(fn dot ((xs (slice f64)) (ys (slice f64))) -> f64
  (var acc f64 0.0)
  (var i (int 0 (len xs)) 0)
  (loop
    (when (= i (len xs)) (break))
    (set acc (+ acc (* (at xs i) (at ys i))))
    (set i (+ i 1)))
  acc)
```

`(int 0 (len xs))` is a range type per [ADR 0003](decisions/0003-range-typed-integers.md). The
declared range makes the bounds check provably unnecessary, so this can beat naive hand-written
Go, which bounds-checks. The type system earns its keep on the first program.

**For:** lowers to every target with zero surprises. Trivially fast. Easy for models to read.

**Against:** no identity, and it does not answer the question that matters — *how do
abstractions get built and where do they go?* Without an answer, layers become ad hoc compiler
passes and the project drifts toward being a small C with parentheses.

**Verdict:** almost certainly the right *vocabulary*. Almost certainly not the whole answer.

---

## 4. Candidate B — Rewriting core

**Identity:** a program is a term; compilation is rewriting it until only terms the target
understands remain. Layers are rule sets. Targets are vocabularies.

```lisp
(layer vectors
  (rule (dot ?xs ?ys) => (sum (zip * ?xs ?ys)))

  ;; loop fusion is a rule, not a compiler pass
  (rule (sum (zip ?f ?a ?b))
     => (reduce 0.0 (len ?a)
          (fn (acc i) (+ acc (?f (at ?a i) (at ?b i)))))))

(layer loops
  (rule (reduce ?z ?n ?body)
     => (block (var acc ?z) (var i 0)
          (loop (when (= i ?n) (break))
                (set acc (?body acc i))
                (set i (+ i 1)))
          acc)))
```

The target's vocabulary is the **termination condition** for rewriting:

```lisp
(target go (vocab core dicts strings))   ; has map and string -> stop at dicts
(target c  (vocab core))                 ; has neither -> dict rules keep firing
```

That is the whole parasite model in two lines. Word-count on Go rewrites down to `dicts` and
stops, emitting Go's `map`. The same source on C keeps rewriting into a real hash table.

**For:**

- The rewriting core and the capability graph ([ADR 0002](decisions/0002-capability-graph.md))
  are the *same object*. The graph stops being separate machinery and becomes a consequence of
  the core.
- Macros, lowering, targeting, optimization, and pattern matching are one mechanism.
- "Emit at the highest layer the target natively provides" is not a rule the compiler enforces;
  it is what rewriting-to-a-vocabulary *is*.
- ~~The implementation is small. A pattern matcher and a rule engine are a few hundred lines.~~
  **Withdrawn** after the g1 derivation — see the second bullet under "Against."
- Rules are `pattern => replacement` — extremely uniform, very legible to models, and
  mechanically checkable.
- Lowering decisions are inspectable: dump the derivation and see exactly why the output looks
  the way it does.

**Status:** survived hand-derivations of gauntlet programs
[4](derivations/g4-word-count.md), [1](derivations/g1-dot-product.md),
[3](derivations/g3-generics.md), and [2](derivations/g2-structs.md).

Program 3 gave the strongest positive result — generics need no mechanism, because a
non-recursive definition *is* a rewrite rule and instantiation is a side effect of matching. It
also produced a counterexample to ADR 0002's governing rule (see §8 there). Program 2 found no
boxing but forced the largest open question: `(slice T)` for a struct `T` cannot share one
representation across targets, so an emitted signature's *arity* varies.

Program 5 found the first *correctness* defect — effects. Rules can change how many times an
effect happens and in what order, where all earlier defects were performance defects. Bindings
themselves cost nothing: an extern is a vocabulary entry, so rewriting halts on it.

All defects found so far are one family — naive rewriting loses sharing, capture-freedom,
simultaneity, or effect ordering — each with a textbook fix predating the project. Four
independent routes to the same question, *when may a term be copied, moved, or deleted?*, which
is substructural and may deserve a single answer in the core's type discipline rather than four
analyses. See [g5 §9](derivations/g5-bindings.md).

**Escaping closures now tested** — [g6](derivations/g6-escaping-closures.md). They cost exactly
what hand-written costs, because hand-written uses the same construct; Shen's wall was
universality, not the mechanism. That derivation also settles what this candidate *is*:
rewriting is lambda calculus generalized, staged so it terminates at compile time. The residual
is where it could not, and that is the only place it costs anything.

Identity, restated:

> **Everything is a function, evaluated at compile time. What survives is what the target must
> do at runtime, and the compiler will tell you exactly what that is.**

**Against, and these are real:**

- **Termination and confluence.** *Largely resolved by the two derivations.* Rules fall in
  three classes: layer-decreasing (free, structurally checkable), measure-decreasing
  (deforestation — count collection-producing nodes; also checkable), and permutative
  (commutativity, reassociation — diverges). Only the third is dangerous, and it can be
  excluded outright, because hosts already do algebra and the only optimization they *cannot*
  do is undoing a materialized intermediate. See [g1 §5](derivations/g1-dot-product.md).
- **The implementation is not small.** The claim below that a rule engine is "a few hundred
  lines" is false. Required so far: auto let-binding, layer stratification, linearity analysis,
  hygiene, range analysis with `require` facts, and a deforestation measure check.
- **The runtime model is still undecided.** Rewriting is a compile-time model; the residual
  vocabulary is a separate question — which is what Candidate A supplies.
- **Compiler performance.** Rewriting is search, and naive search is slow.
- Prior art exists and should be read before committing: Stratego/Spoofax, MLIR's pattern
  rewriting, egg/egglog, Pure, Maude.

---

## 5. Candidate C — Equational core

**Identity:** a function is a set of equations over patterns. Shen's surface, and Haskell's.

```lisp
(defn dot
  [()        ()]        -> 0.0
  [(x . xs)  (y . ys)]  -> (+ (* x y) (dot xs ys)))
```

**For:** genuinely beautiful, and the thing most worth keeping from Shen. Pattern matching is
the feature that makes a language pleasant to write compilers and interpreters in.

**Against:** cons cells (allocation), list fusion to remove them, tail call optimization that
the JVM does not have, and unboxing. Approximately the entire GHC optimization stack stands
between this and Candidate A's speed. As a *core*, this is the Shen wall with new syntax.

**Verdict:** keep it as a **surface**, not a core.

---

## 6. Synthesis — the candidates are not rivals

- **A is the vocabulary** — what rewriting terminates at, and what targets natively express.
- **B is the mechanism** — how everything above reaches A.
- **C is a layer written in B** and rewritten into A.

"Everything is built on top of the core" holds exactly as intended. The core is the rewriter,
and the vocabulary is what it rewrites toward.

If this survives [the gauntlet](gauntlet.md), it is the shape of the language. If it does not,
the failure gets an ADR and the next candidate gets written.

---

## 7. What would prove this wrong

Stated in advance, so the recommendation is falsifiable on the same terms as the candidates.
Status after five derivations and one baseline run:

| Falsifier | Status |
|---|---|
| Rule sets for the five programs turn out non-confluent, needing manual phase ordering everywhere — making B a pass framework with extra steps | **Did not fire.** Termination split into three classes; lowering rules terminate structurally, and the dangerous permutative class can be excluded outright. See [g1 §5](derivations/g1-dot-product.md). |
| The residual is measurably worse than hand-written target code on program 1 or 4 | **Untested — needs a compiler.** The derivations show the intended residual *is* the hand-written code, but nothing has generated it yet. |
| Writing rules is harder to reason about than writing passes, for a person or a model | **Did not fire** — [s6](derivations/s6-rule-legibility.md) wrote both and ran them against each other. Rules are half the lines per layer against a fixed 84-line engine, and one analysis leaves the layer entirely. **But the claim it defended came down:** every rule's right-hand side is imperative Go, and rules share ordering-dependent context that no rule states. "Everything is a rewrite" is false. |

A fourth falsifier was added by measurement rather than foresight:

| Falsifier | Status |
|---|---|
| Compile-time evaluation gives different answers than runtime, making binding-time analysis semantically observable | **Fired, and was fixable.** Go's arbitrary-precision constant folding does exactly this. Closed by [ADR 0009](decisions/0009-staging-preserves-results.md), which forbids it — but it was found by running a program, not by reasoning, which is the point of [ADR 0007](decisions/0007-exploration-over-specification.md). |
