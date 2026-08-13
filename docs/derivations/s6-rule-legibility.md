# Exploration: are rules more legible than passes?

Exploration only. No commitments, no ADR.

The last outstanding falsifier, and the only one that is not a benchmark question. From
[core-candidates §7](../core-candidates.md):

> If writing rules turns out to be harder to reason about than writing passes — for a person or
> a model — the legibility claim is false, and that is a fatal objection given requirement 8.

The honest way to answer it is to write both, run them on the same input, and check they agree.
Both implementations are real and executable, in
[`experiments/legibility/`](../../experiments/legibility/): a minimal flat-tagged IR, SROA as a
hand-written Go pass, SROA as a rule set plus an engine, and tests asserting the two produce
identical output on three inputs — the g2 centroid accumulator, the g2 §6 swap hazard, and a
three-field struct to check arity generality.

**Result: the falsifier did not fire, but the claim it was testing is much weaker than it was
stated. "Everything is a rewrite" is not what this is.**

---

## 1. The numbers

Non-comment, non-blank lines:

| | lines | amortised? |
|---|---|---|
| SROA as a pass | **102** | no — per layer |
| Rule engine | 84 | **yes** — across every layer |
| SROA as rules | **51** | no — per layer |

Rules are **half the lines per layer**, against a fixed 84-line engine. Break-even is at two
layers: 204 versus 186. The language will have many more than two.

One bias worth naming: the pass was written first and the rules second, so the rules benefited
from having already worked the problem out.

## 2. Where rules genuinely won

**The simultaneity analysis left the layer.** [g2 §6](g2-structs.md) requires that splitting a
struct assignment preserve simultaneity — sequencing `acc#0 = acc#1; acc#1 = acc#0` miscompiles a
swap. The pass has to decide, so it carries a recursive `readsVar` traversal and a `simultaneous`
flag. The rules just always emit `KPar` and let the backend decide whether the fields conflict.

This is relocation rather than elimination — the conflict check still has to exist somewhere —
but it is written **once in the backend** instead of in every pass that emits an assignment.

**Rules are locally readable.** `field-of-constructor` can be read without knowing anything about
`split-declaration`. In the pass, the `KVar` and `KField` cases are coupled through the `arity`
map, and you must read both to understand either.

**Sequence patterns made them arity-general for twelve lines.** The first version of the rules
was fixed at two fields, which looked like a fundamental expressiveness gap — a fixed-shape
pattern cannot say "one binding per field" for an unknown field count. Adding a `Rest` pattern
that binds all remaining children took twelve lines in the engine, after which the rules handle
any arity and the three-field test passes. **The gap was real and the fix was cheap**, which is
the opposite of what it looked like an hour earlier.

## 3. Where rules lost

**All four rule right-hand sides are imperative Go.** Not one is a pattern-to-pattern rewrite.
Every rule matches declaratively and then *constructs* its output with `out.Seq(...)`,
`out.Var(...)`, and a `mapFields` helper. So what was built is:

> pattern matching for **dispatch**, imperative code for **construction**

which is a good deal less than "everything is a rewrite."

**Rules share mutable context, and the dependency is invisible.** `field-of-split-local` reads
`c.Split`, which `split-declaration` writes. Nothing in either rule says so. It works because the
engine walks top-down and a declaration precedes its uses in the tree — **an accident of
traversal order, not a property of the rule set.** Reorder the walk and it breaks silently.

That is precisely the concern core-candidates raised, confirmed rather than dispelled: rule
interaction really is harder to reason about than an explicit pass pipeline, because a pass
pipeline states its order and a rule set does not.

The available fixes both push back toward passes: declare each rule's context reads and writes so
the engine can check ordering (a pass manager's analysis dependencies, renamed), or split into a
collect phase and a rewrite phase (two passes).

**No derivation trace.** When the pass misbehaves, Go gives a stack trace. When a rule set
misbehaves, the output is wrong with no indication which rule produced it. Cheap to add, but it
is more machinery, and it is machinery a pass gets for free.

## 4. What this does to the identity

Two claims have been conflated across this project, and they need separating.

**The semantic identity survives**, from [g6](g6-escaping-closures.md) and
[s1](s1-substructural.md):

> Everything is a function, evaluated at compile time. Every term graded by how many times it may
> be used and at which stage. Grade 0 means it is gone before the program runs.

Nothing here touches that. It is about what the language *means*, and this experiment was about
how the compiler is *built*.

**The implementation claim does not survive intact.** "Everything is a rewrite" is false. The
compiler is a hybrid, and the honest version is:

> **Lowering is pattern-directed. Analysis is not.**

Which was already visible before this experiment and not stated: liveness, grading, and range
analysis are whole-function analyses, not local rewrites, and they are five of the eight items on
the machinery list. MLIR reached the same shape — pattern rewriting *plus* analyses — and it is
probably the honest shape for anything of this kind.

## 5. Findings

1. **Rules are half the lines per layer** against a fixed 84-line engine; break-even at two
   layers.
2. **One analysis genuinely left the layer** — simultaneity moves to the backend, written once
   rather than per-pass. Relocation, not elimination.
3. **Rules are locally readable** where pass cases are coupled through shared tables.
4. **Sequence patterns cost twelve lines** and closed what looked like a fundamental
   expressiveness gap.
5. **Every rule's right-hand side is imperative Go.** Pattern matching for dispatch, imperative
   construction for output.
6. **Rules share invisible ordering-dependent context** — the confirmed version of
   core-candidates' worry, and both fixes make the system more pass-like.
7. **No derivation trace**, where a pass gets one from the host language for free.
8. **"Everything is a rewrite" is false; the semantic identity is untouched.**

## 6. Verdict

The falsifier asked whether rules are *harder* to reason about than passes. They are not — they
are half the size, locally readable, and one analysis disappears from the layer entirely. So the
fatal objection to requirement 8 does not land.

But the claim being defended has to come down. This is not a rewriting system with a language
draped over it; it is a compiler with pattern-directed dispatch, imperative construction, and
several ordinary analyses. That is a good architecture and an unremarkable one, and it is worth
saying plainly because the project's appeal has partly rested on the stronger version.

The one result that should worry: **invisible ordering dependencies between rules.** It was
caught here only because the rule set is four rules long and one person wrote both sides. At
forty rules across six layers it would not be caught by reading, and both fixes are a step back
toward the pass pipeline the design was trying to avoid.

**Next, and it follows directly:** make context dependencies explicit in the rule declaration, so
the engine can check ordering rather than the traversal accidentally providing it. That is a
small, concrete piece of design work, and it is the difference between a rule set that scales to
forty rules and one that does not.
