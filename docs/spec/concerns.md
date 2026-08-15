# Concerns about the atom as built

Written alongside the first implementation ([`core/`](../../core/)), which implements
[core-0](core-0.md) and nothing else.

The atom works: 16 tests pass, every one of them a worked example lifted from the specification,
and the two-target demo produces two normal forms from one file. What follows is what is
**wrong, missing, or unproven** — recorded now, while it is cheap, rather than discovered later.

---

## 1. Gaps that are known and deliberate

### 1.1 ~~β always substitutes~~ — **CLOSED, 2026-08-14**

[callbyneed-2026-08-14](../../gauntlet/results/callbyneed-2026-08-14.md). β is now call-by-need:
at most one occurrence substitutes, more than one normalises the argument and binds it unless it
is a literal, a variable, or a λ. The residual is `(let e (fn (x) b))`.

The history is worth keeping, because the justification moved twice:

1. Claimed as "a silent 2× on the hot loop" — an unmeasured prediction.
2. **Withdrawn** when [Go's CSE](../../gauntlet/results/duplicate-read-2026-08-14.md) turned out
   to erase it: three variants of the filter loop compiled to byte-identical machine code.
3. **Reinstated** by [word count](../../gauntlet/results/wordcount-2026-08-14.md) at 615× on Go
   and 1,089× on JS, because `strings.Fields` allocates and no host can hoist an allocation.

The two together gave the criterion — *duplication is free exactly when the duplicated term is
pure* — and the asymmetry between the costs meant the fix needed **no grades and no cost model**,
which is what this section originally said was blocking it. Occurrence counting plus a four-case
syntactic test.

`TestFilterFusesToOneLoop` asserted the wrong answer on purpose for two days and now matches
[q5b §3](q5b-filter.md). The spec and the code agree again.

### 1.6 ~~No notion of effect~~ — **CLOSED, 2026-08-14**

[effects.md](effects.md), measured in
[effects-2026-08-14](../../gauntlet/results/effects-2026-08-14.md). β carries a side condition:
an impure argument is never substituted, but let-bound at the application site, whatever its
occurrence count. The three clauses deny contraction, weakening and exchange.

Two things are worth keeping from how it went. First, [g5](../derivations/g5-bindings.md) listed
*two* hazards and there are three — the missing one, weakening, is the one that makes `seq`
expressible. Second, g5 said effects would arrive with program 5, and they were already here:
`dict-inc` has mutated in place since word count.

**Still open:** the aliasing half. Ordering is preserved; destructive update is not checked. See
§3.4.

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

### 3.2 ~~`.` is currently an identifier character~~ — **CLOSED, 2026-08-15**

Reserved as the qualifier separator ([modules.md §3](modules.md)). It was free: no name in
`targets/` or `examples/` contained one. `/` stays an ordinary identifier character, so a module
path like `go/strings` is a single segment. A name may not begin, end, or double the separator.

The original text follows, because it predicted this exactly.

### 3.2a `.` was an identifier character

Admitted so that `fold-range` and friends read naturally, but it also makes `a.b` a single name.
If field access is ever written `a.b`, this collides. Nothing depends on it yet.

### 3.4 Destructive update is unchecked

`dict-inc` emits `m[k]++` — it mutates its argument and returns it. That is correct only while the
pre-mutation dictionary is dead, which nothing verifies. Today no program can observe the
difference because **no primitive reads a dictionary**, the same accident that made
[strings](strings.md) cheap.

[effects.md](effects.md) fixes ordering and deliberately does not fix this. It becomes real the
moment a primitive reads a dictionary, and it is [g7](../derivations/g7-aliasing.md)'s question.

### 3.3 The residual check reports free variables

`(dot p q)` at top level reports `p` and `q` as not-in-normal-form, which is technically true and
practically noise — they are inputs. The example was changed to bind them. A real module system
would distinguish "unbound" from "not lowered", and there is no module system
([design-direction open decision 5](../design-direction.md)).

### 1.5 String escapes are Go's, not core-0's

The reader scans a string literal to its closing quote and hands the whole thing to
`strconv.Unquote`, so the accepted escape set is **Go's** — unicode escapes, hex, octal, and the
rest — where [core-0](core-0.md) specifies only the four a target template needs.

Narrowing it is a specification question rather than a reader one, and nothing depends on the
difference yet. Strings exist at all because target declarations carry emission templates; the
*operations* on them are still absent, per [def.md §5](def.md).

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
