# Concerns about the atom as built

Written alongside the first implementation ([`core/`](../../core/)), which implements
[core-0](core-0.md) and nothing else.

The atom works: 16 tests pass, every one of them a worked example lifted from the specification,
and the two-target demo produces two normal forms from one file. What follows is what is
**wrong, missing, or unproven** — recorded now, while it is cheap, rather than discovered later.

---

## 1. Gaps that are known and deliberate

### 1.1 β always substitutes — the let-binding discipline is absent

[core-0 §3](core-0.md) requires a let-binding rather than a copy when a variable occurs more than
once and the argument is non-trivial. **The implementation does not do this.** It substitutes
unconditionally.

Why it was left out: deciding when to bind requires knowing whether the duplicated term is
compile-time (free to copy) or runtime (expensive). That is the *grade*, and the atom has no
grades. A cost model would also work, and the atom has no cost model.

Why it is currently harmless: the atom has no effects, so duplication costs compiler time and
duplicated runtime work, not wrong answers. [g5](../derivations/g5-bindings.md) showed that with
effects it becomes a **correctness** bug.

> **⚠ Measured and withdrawn, 2026-08-14** —
> [duplicate-read-2026-08-14](../../gauntlet/results/duplicate-read-2026-08-14.md). The claim
> below that this is "a silent 2× on the hot loop" is **false**. Go's CSE eliminates the
> duplicate read: the generated code, the naive two-read form, and the bind-once form all compile
> to **byte-identical machine code** with a single `MOVSD`. The 1.45× that the clock showed was
> **code alignment** — the two functions sharing a cache-line offset shared a runtime.
>
> Call-by-need is still worth having, for effects (g5) and for weaker hosts, but not for this.
> And the noise floor is not 15%: alignment alone produced a stable, reproducible 45% gap between
> identical code.

Why it matters anyway — this is visible in a passing test. `TestFilterFusesToOneLoop` produces:

```lisp
(fold-range 0.0 (alen a)
  (fn (acc i) (if (pos (aindex a i)) (add acc (aindex a i)) acc)))
```

`(aindex a i)` appears **twice**. [q5b §3](q5b-filter.md) predicted exactly this and showed the
correct residual binds it once. So the specification and the implementation disagree, the test
encodes the *implementation's* answer, and **the test is currently wrong on purpose**. It should
be changed when the discipline lands, not before.

> ~~The first thing to fix~~ — **not the first thing to fix.** Still the first place the spec and
> the code diverged, but the performance argument for closing it has been measured away.

### 1.2 NFC normalisation is specified and not checked

[core-0 §1.1](core-0.md) requires NFC. The reader checks UTF-8 validity and rejects
bidirectional controls, but does not normalise, because Go's standard library has no normaliser
and `golang.org/x/text` is a dependency the atom does not otherwise need.

Consequence today: `é` as U+00E9 and as `e`+U+0301 are two distinct identifiers that display
identically.

### 1.3 Capture avoidance is by freshening, not by representation

core-0 and [s1](../derivations/s1-substructural.md) specify a locally nameless representation,
under which capture is *unrepresentable*. The implementation uses names and freshens on collision
— capture is impossible but only because a function is careful, rather than because the bad state
cannot be written down.

`TestSubstitutionAvoidsCapture` covers the case. One test is not a proof, and this is the kind of
thing that is wrong in the case nobody wrote a test for.

### 1.4 Pointer trees, not the flat arena

[ADR 0005](../decisions/0005-implementation-language.md) specifies a flat tagged struct with
index-based children. The reducer uses a pointer tree, because substitution allocates constantly
and an append-only arena would grow without bound.

This is fine for the reducer and wrong for [ADR 0006](../decisions/0006-ir-file-format.md)'s file
format. Two representations will be needed, with a conversion.

## 2. Things that are unproven rather than missing

### 2.1 Confluence and termination are asserted, not proved

[core-0 §6](core-0.md) lists three theorems. **None is proved.** What exists:

- **Termination** has a guard — recursive definitions are never δ-reduced, computed by reachability
  in `markRecursive` — plus a fuel limit for everything else. Two tests cover recursion, one
  covers self-application. That is a *mechanism*, not a proof, and the fuel limit is an admission
  that the mechanism is incomplete.
- **Confluence** is untested entirely. Normal-order reduction is deterministic here, so the
  implementation cannot *observe* non-confluence; a different strategy might.
- **Stage soundness** is untested and currently untestable — the atom has no arithmetic, so there
  is nothing whose compile-time and runtime values could disagree. It becomes testable the moment
  a primitive is evaluated at compile time, and [ADR 0009](../decisions/0009-staging-preserves-results.md)
  says that is where the danger is.

### 2.2 Reduction order is one choice among several

`normalize` is normal order — leftmost-outermost, reducing under abstractions. That is the right
default for a partial evaluator: it does not get stuck on arguments it cannot evaluate, and it
reduces inside the function bodies that become loop bodies.

It is also the order that duplicates the most work, which interacts badly with §1.1. Call-by-need
would fix both at once and is the obvious next thing to try.

### 2.3 No primitive is ever evaluated

`(add 1 2)` reduces to `(add 1 2)`. The atom has no δ-rules for primitives, so no constant folds.

That is correct for the atom as specified — but it means the parts of the design that are about
*computing* at compile time are entirely untested, including the one property
([ADR 0009](../decisions/0009-staging-preserves-results.md)) that was derived from the algebra
rather than measured.

## 3. Design questions the implementation raised

### 3.1 Should mathematical symbols be identifiers?

UAX #31 admits letters, not symbols, so `＋` (U+FF0B), `≤`, and `∘` are **not** identifiers. A
test asserts this.

That is the standard's answer. It may not be ours — Agda and Julia both extend the identifier set
for exactly this reason, and a language that accepts UTF-8 and then rejects `≤` will be asked
about it. UAX #31 provides for profiles; adopting one is a decision, not an accident, and it has
not been made.

### 3.2 `.` is currently an identifier character

Admitted so that `fold-range` and friends read naturally, but it also makes `a.b` a single name.
If field access is ever written `a.b`, this collides. Nothing depends on it yet.

### 3.3 The residual check reports free variables

`(dot p q)` at top level reports `p` and `q` as not-in-normal-form, which is technically true and
practically noise — they are inputs. The example was changed to bind them. A real module system
would distinguish "unbound" from "not lowered", and there is no module system
([design-direction open decision 5](../design-direction.md)).

## 4. What is *not* a concern

Worth recording, because it is where the design earned something:

- **The two-target demo works and is one line of difference.** `(target blas (prim … dot))`
  against `(target go (prim …))` is the entire distinction between emitting a BLAS call and
  emitting a loop. The parasite thesis is executable.
- **Fusion falls out of β and δ**, with no fusion rules, exactly as
  [q5](q5-do-we-need-rules.md) predicted on paper. `TestSameTermTwoNormalForms` reduces the dot
  product to [g1](../derivations/g1-dot-product.md)'s residual with no machinery beyond the two
  reduction rules.
- **`filter` fuses too**, via the push representation, exactly as [q5b](q5b-filter.md) predicted.
- **Recursion is handled by the side condition the spec derived**, and mutual recursion falls out
  of the same reachability computation without special-casing.

Four predictions made on paper, four confirmed by running code. That is a better hit rate than
this project's predictions about *host compilers*, and the difference is the one
[q5 §1](q5-do-we-need-rules.md) named: formal questions settle on paper, empirical ones do not.

## 5. Order of work

1. **The let-binding discipline** (§1.1) — the spec and the code disagree, and a passing test
   currently encodes the wrong answer.
2. **Evaluate primitives** (§2.3), which makes stage soundness testable at all.
3. **Call-by-need** (§2.2), which may subsume item 1.
4. NFC (§1.2), locally nameless (§1.3), and the identifier profile (§3.1) — real, none urgent.

Everything above is the atom. **The Go emitter is still the thing that has never been tested**,
and none of this changes that: a normal form that reaches the baseline is still the first
measurement of our own output.
