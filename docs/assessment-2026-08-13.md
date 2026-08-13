# Assessment: does the rewriting core survive?

Dated snapshot at the end of the first exploration pass. **Deliberately not an ADR** — writing
"candidate B is the core" as a decision would be the predecessor project's mistake in ADR form.
This is evidence and a recommendation, and it stays falsifiable.

State: six hand-derivations ([docs/derivations/](derivations/)), one measured baseline run
across Go, JS, and Java ([gauntlet/results/](../gauntlet/results/)). No compiler exists.

---

## 1. Is it Turing complete?

**Two levels, two answers — and that split is the point.**

Term rewriting is Turing complete in general; lambda calculus is a rewriting system with one
rule, and [g6](derivations/g6-escaping-closures.md) established ours is that generalized. So the
raw formalism is TC and the question is almost trivial.

The useful question is which level is which:

| Level | Turing complete? | Why that is right |
|---|---|---|
| **Compile time** (rules) | **No — strongly normalizing by construction** | Lowering rules move strictly down a layer DAG; deforestation rules decrease a structural measure; permutative rules are excluded. Compilation is guaranteed to terminate. |
| **Runtime** (residual) | **Yes** | Loops, plus recursive functions, which cannot be rules and stay residual. |

A non-terminating compile-time level is a known design failure — C++ templates are accidentally
TC and it is regarded as a mistake; Zig bounds `comptime` with a quota. **Ours terminates by
structure rather than by budget**, which is a stronger property than either.

**Gap:** compile-time *computation* — building a lookup table at compile time, say — needs
bounded recursion at compile time. Since all recursion is currently residual, you cannot. The L2
metaprogramming layer was never derived. That is the largest unexplored area of the design.

## 2. Is it easy to implement?

**Harder than claimed, easier than the alternatives, and — decisively — no research risk.**

The "a few hundred lines" claim was withdrawn. Actual machinery, accumulated across six
derivations:

> auto let-binding · layer stratification · linearity analysis · hygiene · range analysis with
> `require` facts · deforestation measure check · ANF normalization · monomorphization for
> recursive generics · polymorphic type checking · SROA with parallel-assignment temporaries ·
> effect-context checking on rules

Eleven items. But every single one has a textbook solution predating this project, and several
are a few hundred lines each. **Nothing on that list is an open problem.**

Compare with what the rejected candidates would have required. A lambda-calculus core needs
closure conversion, escape analysis, unboxing analysis, and a GC — and general unboxing is
genuinely open research. That is the difference between "a lot of known work" and "a research
program," and it is the difference that stopped the predecessor.

Honest scale estimate: a rule engine plus these analyses, no backends, is plausibly 8–15k lines
of Go; a backend perhaps 1–3k each. Months, not weeks. Bounded, not open-ended.

## 3. Is it easy to extend?

**Yes — this is the strongest dimension, and it is what the project is for.**

Measured or derived, not asserted:

- **Bindings cost nothing.** An extern is a vocabulary entry plus an emission template;
  rewriting halts on it. Requirement 4 was satisfied by machinery that already existed
  ([g5](derivations/g5-bindings.md)).
- **Generics cost nothing.** A non-recursive definition *is* a rewrite rule; instantiation is a
  side effect of matching. No monomorphization pass exists
  ([g3](derivations/g3-generics.md)).
- **New layers are rule sets**, added without touching the compiler.
- **New targets are a vocabulary plus emission templates**, and
  [ADR 0006](decisions/0006-ir-file-format.md) makes the backend interface a file format, so a
  backend need not be written in Go or live in this repository.

One real cost, added by measurement:
[ADR 0008](decisions/0008-measurement-over-principle.md) requires a benchmark behind every
parasite decision. **Extension is cheap in mechanism and expensive in verification.**

## 4. Is it easy to reason about?

**Mixed, and one half is untested.**

For:
- Rules are `pattern => replacement` — uniform, and mechanically checkable rather than merely
  readable.
- **Binding-time analysis gives a checkable answer about cost**: "this fold is eliminated" vs
  "this handler survives: 16 bytes, 1.55ns indirect call." Very few languages can tell you that.
- Lowering is inspectable — dump the derivation and see why the output looks as it does.
- The defect family is closed and known in advance (see §7 below).

Against:
- **Rule interaction versus pass ordering is untested**, and it is the falsifier that would be
  fatal to requirement 8. Listed as open in [core-candidates §7](core-candidates.md).
- Six derivations were done by hand and five claims were wrong.

That last point needs care, because it cuts the opposite way from how it reads. **Every one of
the five refutations was about host behaviour — Go's inliner, JS's `Map`, Java's allocator —
not about the rewriting core.** The core's own reasoning survived six derivations intact. What
failed was the part that was always going to need measuring.

## 5. Is there a cost to abstractions?

**Measured: none for the eliminated case, host-parity for the surviving case.**

| Case | Cost |
|---|---|
| Generics, higher-order functions, layers — non-escaping | **Zero.** `fold` has no runtime existence in the output. |
| Escaping closure, non-capturing | **0 bytes**; only the containing slice allocates |
| Escaping closure, capturing | **16 bytes**, one allocation — identical to hand-written Go |
| Indirect dispatch | **1.55 ns** |

Shen's wall was *universality* — every function a closure, so every call pays. Here payment is
confined to genuine runtime dispatch and equals what the host charges.

Three caveats, stated plainly:

- **Untested end-to-end.** The derivations show the intended residual. Nothing has generated it.
- **Code size versus specialization is unresolved**, and requirements 5 and 6 pull opposite
  ways.
- **Staging soundness was not free.** It had to be bought with
  [ADR 0009](decisions/0009-staging-preserves-results.md) — Go's arbitrary-precision constant
  folding would have made binding-time decisions change program output.

## 6. Will it scale?

**Two unmeasured risks and one unresolved tension.**

| Dimension | Status |
|---|---|
| Rule count | **Good.** Deforestation keeps the lowering set linear rather than combinatorial. |
| Capability count | **Good.** The Tier 1 / Tier 2 split means most of an ecosystem costs nothing. |
| Ecosystem / third parties | **Good.** File-format backend interface; layers are libraries. |
| **Compile time** | **Unmeasured.** Rewriting is search. A large program against many rules could be slow, and nothing has tested it. |
| **Binary size** | **No data at all.** Half of requirement 6, and an unchecked box in the gauntlet. |
| Specialization vs size | **Unresolved tension.** Binding-time analysis makes it *visible* per abstraction without deciding it. |

## 7. The pattern worth naming

Every defect found across six derivations is one shape: **naive rewriting loses a property the
term held implicitly.**

| Derivation | Property lost | Classical fix |
|---|---|---|
| g4 | Sharing | Let-binding / graph reduction |
| g1, g3 | Capture-freedom | Hygiene, fresh binders |
| g2 | Simultaneity | Temporaries |
| g5 | Effect count and order | Context-depth checking |

Four independent routes to one question: **when may a term be copied, moved, or deleted?** That
is substructural, and every entry has a solution predating the project by decades.

A bounded, known failure family is a much better position than open-ended risk, and it is the
single strongest argument in the candidate's favour.

---

## Verdict — superseded, see §"Revised verdict" below

The original call, made before the substructural thread and the size measurement, was
**continue, explore no new candidate, do not build yet.** Every experiment it named has since
been done and none of them fired. The verdict that follows from that is different, and it is
recorded at the end of this document rather than by editing this section.

**Continue. Do not explore another candidate yet — but do not start building the compiler
either.**

Reasoning:

1. **It survived both derivations designed to kill it.** Program 1 (same-layer fusion) and
   escaping closures were the two most likely failure points; the first turned out mostly not to
   be same-layer, and the second costs exactly what hand-written costs.
2. **The identity question got a better answer than it asked for.** The original question was
   what the core looks like — expecting lambda calculus, objects, or a stack. The answer is
   *staged lambda calculus*, which is simultaneously an identity and an explanation of why it is
   fast. That is rare and worth a great deal.
3. **The refutations were all about hosts, not about the core** (§4).
4. **There is no obvious rival to explore.** Candidates A and C were never rivals — A is the
   vocabulary the rewriting terminates at, C is a layer written in B. The rejections (Forth,
   runtime lambda calculus, TLA+, flat instruction streams) were all sound and none was
   weakened by measurement. "Explore another" has no specific alternative behind it, and
   undirected exploration is not the discipline that
   [ADR 0007](decisions/0007-exploration-over-specification.md) asks for.

Against building yet: three risks remain **cheap to test now and expensive to discover late**,
which is precisely the shape of the failure that stopped the predecessor.

## What would make this stop

Stated in advance, so continuing stays a falsifiable position:

- **Rule legibility fails.** If a layer written as rules is harder for a person or a model to
  follow than the same layer written as compiler passes, requirement 8 is lost and the
  legibility half of the pitch is false.
- **The substructural thread does not collapse.** If sharing, capture, simultaneity, effects,
  and linearity stay five separate analyses, the machinery list stays at eleven and "small,
  easy to implement" is no longer honest.
- **Compile time scales badly.** If rewriting a realistic program is slow enough to be felt,
  the model has a ceiling that no amount of runtime performance compensates for.
- **Binary size loses badly to hand-written.** Requirement 6 is currently unmeasured, and
  specialization pushes the wrong way.

Any one of these firing is grounds to reopen the candidate question — with a specific
alternative in hand, not just dissatisfaction.

## Next three experiments, in order

1. ~~**The substructural thread.**~~ **Done** — [s1](derivations/s1-substructural.md). Four of
   five disciplines collapse onto structural rules over two axes; capture is not structural and
   exits into a representation choice. **Machinery drops from eleven items to eight**, and grade
   0 turns out to *be* the staging annotation, so g6's per-abstraction cost report comes out of
   the soundness machinery rather than beside it. §2's answer improves. New risk taken on: the
   usability record of substructural systems is poor, and the mitigation — infer and report
   rather than declare and check — is untested.
2. ~~**Multiplicity inference.**~~ **Done** — [s2](s2-multiplicity-inference.md). **Zero
   annotations needed in application code** across all five programs. Grade 0 is *observed* in
   the residual rather than inferred, so it costs nothing and never has to hedge. One
   unavoidable declaration — extern purity — which lives in binding files, defaults safely to
   stateful, and application programmers never see. The usability risk s1 flagged is materially
   reduced. It also refuted s1's accumulator row: that is uniqueness, not multiplicity.
3. ~~**Mutation through an aliased slice.**~~ **Done** — built as gauntlet program 6,
   [g7](derivations/g7-aliasing.md). Split cleanly in two. **For slices the hazard is
   correctness, not performance**: conservative codegen costs 0% on Go, and in-place is 4.6–6.2×
   *slower*, so forbidding mutable slice parameters closes it for free and beats what a
   programmer would hand-write. **For heap structures the news is worse**: a uniqueness false
   negative costs 40× at eight entries and 1,540× at five hundred, unbounded in structure size —
   a different severity class from anything else measured here.
4. ~~**Liveness-based reuse across function boundaries.**~~ **Done** —
   [s3](derivations/s3-cross-boundary-reuse.md). The risk comes down substantially. **Most
   boundaries do not survive rewriting**, since non-recursive functions are rules and inline;
   only recursive, outlined, extern, and indirect calls remain. For those, **parameter
   multiplicity in the signature is the ownership annotation** — modular, no whole-program
   analysis. Where it cannot decide, an RC check costs **2–4%**. The negative result worth
   keeping: **naive RC costs 14×**, so reference counting depends on the static analysis rather
   than replacing it.
5. ~~**A structure stored inside another structure.**~~ **Done** — [s4](s4-nesting.md). The
   uniqueness story is now closed end to end. **Value semantics narrows to scalar fields**:
   deep-copying a struct with a heap field costs 281×–16,300×, so struct copy is shallow and the
   grading tracks the field. The case predicted to be hardest — a dynamic index, where `cs[i]`
   is not a variable and liveness cannot reach it — costs **nothing** to check at runtime. No
   separation logic or borrow checker needed. Cost recorded: the gauntlet's parity standard
   gains the qualifier **"at equal semantics"**, since hand-written Go would not write the
   program that costs us 281×.
6. ~~**Cycles.**~~ **Done** — [s5](derivations/s5-cycles.md). They are **unrepresentable by
   construction**: reuse requires the old value dead, and a value stored inside the new one is
   live, so the liveness check that exists for performance forbids the aliasing that would close
   a cycle. That makes the RC fallback **sound**, not merely cheap — its one fatal weakness
   cannot occur, where Koka and Lean have to impose acyclicity as a deliberate rule. The forced
   workaround, arena plus indices, measured **1.88× faster** than pointers for scattered access
   and 1.21× slower for near-local. Real costs are ergonomic and safety-related, not
   performance.
7. ~~**Rule legibility.**~~ **Done** — [s6](derivations/s6-rule-legibility.md), by writing both
   and running them against each other. **The falsifier did not fire**: rules are half the lines
   per layer against a fixed 84-line engine, one analysis leaves the layer entirely, and sequence
   patterns closed the arity gap for twelve lines. **But the claim came down.** Every rule's
   right-hand side is imperative Go, and rules share ordering-dependent context that no rule
   states — the confirmed version of the worry, caught only because the rule set is four rules
   long. "Everything is a rewrite" is false; the *semantic* identity is untouched.
8. **Explicit context dependencies in rule declarations**, so the engine can check ordering
   rather than the traversal accidentally providing it. Small, concrete, and the difference
   between a rule set that scales to forty rules and one that does not. Follows directly from s6.
9. ~~**Output size.**~~ **Done** — [size baseline](../gauntlet/results/size-2026-08-13.md). Go's
   floor is **1.43 MB** and 200 specialized call sites add 1.0% of it, so on GC'd hosts
   requirement 6 is about a runtime we do not control; it is really a C and embedded requirement,
   deferred with that target. **g3's specialization-versus-size tension resolves**: below the
   host's inlining budget specialization is 26% *cheaper*, above it up to 14.4× more expensive,
   and the crossover is the host's own budget — the same number from the performance baseline.
   On JS, gzip erases ~95% of the penalty.

Then, and only then: reader, front end, Go backend, JS backend before any front-end features.

## One-line summary

*The candidate has survived everything aimed at it, the costs it does carry are known and
bounded rather than open-ended, and the three things that could still kill it are all cheap to
test before writing a compiler — so: continue, test those three, and keep the question open.*

---

# Revised verdict

Added after every experiment above was completed. **The recommendation changes: build the
vertical slice.**

## What is established, and what is not

**Established:** the approach has no known fatal flaw, and its costs are bounded and largely
textbook. Six gauntlet derivations, six substructural explorations, no falsifier fired, and
several results better than predicted — generics need no mechanism, cycles are unrepresentable,
the size tension inverted.

**Not established: that it works.** Nothing has been compiled. Every number in
[gauntlet/results/](../gauntlet/results/) measures **hand-written host code** — the bar. Not one
measures output this project produced. The parity claim the whole design rests on has never been
tested.

## Three reasons to discount the confidence

1. **The error rate on unmeasured reasoning is high.** Five claims refuted in the first baseline
   run, plus [s1](derivations/s1-substructural.md)'s accumulator row,
   [s6](derivations/s6-rule-legibility.md)'s framing, the biased graph benchmark in
   [s5](derivations/s5-cycles.md), and the degenerate test data in
   [g7](derivations/g7-aliasing.md). Roughly half of what was reasoned and then checked was
   wrong. The derivations that have *not* been checked deserve the same discount — and none has
   produced actual output.
2. **The strongest claim came down.** "Everything is a rewrite" is false. What survives is
   pattern-directed lowering plus ordinary analyses: a good architecture, and an unremarkable
   one.
3. **The experiment queue regenerated every time.** This document originally named three next
   steps; those produced s1 → s2 → s3 → s4 → s5, g7 came out of s2, s6 produced "explicit
   context dependencies," and the size run produced "measure V8 and HotSpot budgets." The list
   never got shorter — it changed contents.

That third point is the important one. **[ADR 0007](decisions/0007-exploration-over-specification.md)
protects against stalling on a specification. It has no stopping rule for stalling on
exploration**, and exploration is the more comfortable of the two failure modes. That is the
predecessor's failure wearing different clothes.

## Why build now

The remaining uncertainty has concentrated into questions only a working compiler can answer:

- **Does the residual match hand-written code?** Never tested. This is the whole claim.
- **Does compile time scale?** Never measured.
- **Do rules compose across six layers?** Tested with four rules in one layer.
- **Is multiplicity inference implementable?** Derived by hand, never run.

None of these gets cheaper by deriving further, and all of them get answered by one vertical
slice.

## The slice

1. Reader, printer, canonical formatter for s-expressions.
2. Front end: functions, locals, structs, range-typed scalars, structured control flow.
3. **Go backend.** One non-trivial gauntlet program compiling, running, and benchmarked against
   the existing baseline.
4. **JS backend before any front-end features** — the discipline that has been in this document
   since the first draft and is still the one most likely to be skipped.

If the first non-trivial program lands at parity, that is worth more than every derivation in
`docs/`. If it does not, that is known within weeks instead of years, which was the entire point
of building the gauntlet first.

**What would send us back to exploration:** the residual missing parity on gauntlet 1 or 4 by
more than the noise floor, or rule ordering proving unmanageable past roughly forty rules. Both
are visible early in the slice, and neither is visible from here.
