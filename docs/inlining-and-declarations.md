# What a declaration means after inlining

Research, 2026-09-03. **BUILT** — the recommendation in §6 was implemented the
same day; see [ascribe-2026-09-03](../gauntlet/results/ascribe-2026-09-03.md) for
what it cost and for the three bugs it found. This document is kept as the
argument the build tested.

On hamza's *"maybe I am wrong, but this is the type system question."*

**Not wrong, and the precise sense in which it is right is the finding.** A range
has three effects (scalarrange-2026-08-31); reduction is correct to erase one of
them, has nothing to erase in another, and destroys the third. The third is the
only one that is not a fact about values, and that is exactly why the type layer
is where it belongs.

And the mechanism turns out to exist already: §4a shows `big-str` — a term-level
ascription that survives reduction — promoting the very loop that the same
declaration on a signature cannot reach.

## 1. The defect, stated without mentioning inlining

[ADR 0019](decisions/0019-precision-by-declaration.md) offers three ways to clear
an integer operation the compiler cannot prove stays inside the portable window:

1. narrow the range,
2. ask for the trap (`-checked`),
3. **declare a range ABOVE the window**, which promotes that value to arbitrary
   precision.

Escape 1 is a fact about a value and can be written wherever the value is
declared. Escape 2 is a whole-program flag. **Escape 3 is expressible only in a
signature — on a parameter or on a result — so a value that is neither has no
way to ask for it.**

That is the whole defect, and it is a hole in ADR 0019 rather than a bug in the
reducer. Escape 3 is the only one of the three that gives EXACTNESS, so the
programs it excludes are excluded from exact arithmetic altogether.

## 2. What it looks like, and why it looks like an inlining problem

```
(sig scaled ((n (int 0 50))) (int 0 (pow 2 200)))
(def scaled (fn (n) (loop ((acc (+ n 7)) (i 0))
                      (>= i 6) acc else (again (* acc 999983) (+ i 1)))))

(sig run ((n (int 0 50))) int)
(def run (fn (n) … (scaled n) …))
```

`scaled` says its result is arbitrary-precision. Emitted on its own it is
correct — `func GenScaled(n int) *big.Int`, the accumulator a `math/big.Int`.
Called from `run`, whose result is an ordinary `int`, the program is **refused**:
four operations cannot be proven in-window, because after inlining nothing tells
the solver the accumulator is meant to be arbitrary-precision.

So the natural way to write escape 3 for an internal value — put it on a helper —
does not work, and it fails in the direction that makes it look like a reduction
bug. It is not: reduction is doing what it always does. The declaration was
attached to a BOUNDARY, and whole-program reduction removes every boundary that
is not an entry point.

**Exporting `scaled` does not help.** It is emitted as its own function *and*
inlined at the call site; both were checked.

**The only escape that works today is accidental.** A big LITERAL carries its own
magnitude, so `constBigValue` seeds the promotion with no signature involved —
which is why `gauntlet/differential/cases/big-divmod.oro` is written with one.
That works when the magnitude comes from a constant and never when it comes from
a loop, so a factorial has no escape at all.

## 3. Why this is the fifth time, and the fifth time is different

refinements.md §6b records a trichotomy for a `where`:

| where it sits | what happens |
|---|---|
| a `prim` | an obligation, discharged at every call site |
| an **exported** definition | a published contract, **assumed** |
| an **internal** definition | **dropped** |

and argues that dropping is not a loss but a *strengthening*: reduction inlines
the call, so the body's own obligations land on the caller's concrete values,
which is better than checking a conservative summary. postconditions.md §3
reaches the same conclusion for `ensures`. indexnarrow-2026-08-27 and
scalarrange-2026-08-31 both record the same structural limit for a narrowed
parameter and a declared scalar range.

**Those four are all about FACTS, and the argument is sound for facts.** A fact
is something to be proven; inlining gives strictly more information, so a fact
declared at a boundary is redundant once the boundary is gone.

**This one is about a DIRECTIVE, and the argument does not transfer.** A range
above the window is not a claim the compiler checks — it is a choice the compiler
makes on the programmer's instruction. Inlining gives more information about
values and none at all about intent, so removing the boundary removes the only
record of the instruction. Dropping a fact is a strengthening; dropping a
directive is a loss of meaning.

> **The trichotomy holds for what a declaration ASSERTS and fails for what it
> REQUESTS.** Nothing in this repository had separated those, because until
> arbitrary precision every declaration was an assertion.

## 4. Testing the hypothesis: is it the type system question?

scalarrange-2026-08-31 established that a range has **three effects** kept
deliberately apart. Follow each through reduction:

| effect | what it is | after inlining |
|---|---|---|
| **a type** — `(int LO HI)` normalises to `int` at `compatible` | a fact | **survives**, because the checker re-derives it from the residual |
| **a premise** — desugared in the reader into the `where` it means | a fact | **correctly dropped**; inlining gives more |
| **a representation** — this value is stored as a bignum | a **directive** | **destroyed** |

So the answer is yes, with a sharpening. It is not that the type system is
missing — `emit/check.go` types the residual and does it well. It is that

> **the residual's types are INFERRED, and a representation directive is not
> inferrable. There is no way to write a type at a place that is not a boundary,
> so the only carrier for intent is erased along with the boundary.**

That is a type-system question in the precise sense that the fix is a type
ascription that survives reduction, and it is *not* a question about the analysis:
the interval pass already sees that the accumulator leaves the window — that is
why it refuses. **What is missing is not analytic power. It is intent.**

This also explains why the architecture did not notice. docs/spec/types.md types
the residual *after* reduction, and says this is cheap "because reduction has
already made the term monomorphic, first-order and closed". That is true and it
is a good trade — but it makes the type layer a CONSUMER of reduction's output,
so it cannot carry anything into it. Every declaration the programmer writes has
to survive reduction on reduction's terms.

## 4a. The mechanism already exists, and it already works

This is not a hypothesis. `big-str` is a **term-level ascription that survives
reduction**: its declared argument type is `big`, it is an application of a
primitive so δ never touches it, and `MentionsBig` reads it precisely because it
is "the one place a whole program must name arbitrary precision" (emit/bigrep.go).

So the §2 program was rewritten with no signature declaring anything big, and the
demand supplied by that marker alone:

```
(sig run ((n (int 0 50))) string)
(def run (fn (n)
  (big-str (loop ((acc (+ n 7)) (i 0))
             (>= i 6)  acc
             else      (again (* acc 999983) (+ i 1))))))
```

```go
func GenRun(n int) string {
	acc := (big.NewInt(int64((n + 7))))
	var i int = 0
	for ; ; acc, i = (acc.Mul(acc, (big.NewInt(int64(999983))))), (i + 1) {
```

**The internal loop is promoted, the accumulator is a `math/big.Int`, the mutable
rewrite fires, and every operation is bounded — 2 of 2, 1 of 1 loop.** The
identical loop with the demand coming from an inlined helper's signature is
refused.

That settles two things. The solver needs no change: given a demand it does the
right thing, wherever the demand comes from. And **what is missing is exactly
`big-str` minus the string** — a marker that says *this value is
arbitrary-precision* without also saying *render it*.

## 5. The design space

### (a) A marker the REDUCER preserves — recommended, see §6

When δ unfolds a definition whose signature declares a result above the window,
wrap the body: `(scaled n)` → `(the T body[n])`.

- **No new term kind.** `the` is an application of an injected structural name,
  the way `if`, `let` and `loop` already are (`coreNames` + `addCore`), and a
  target may neither decline nor declare it.
- **No new surface.** The programmer already wrote the declaration; the compiler
  stops losing it.
- **The reducer needs one map**, `Env.BigResult`, populated from `prog.Sigs`
  exactly as `Env.Rec` is — and `Rec` is the precedent that `unfoldable` already
  carries a per-name property that is not about terms.
- **A marker written by one pass and removed by another is an established
  shape**: `big-of-small` is one today, and §4a shows `big-str` already doing
  the whole job for the one case it happens to cover.

The honest risk is not speculative — it is what happened to `big%-small` in
subdiv-2026-09-03 two hours ago. A marker in the middle of a term is a shape
later passes do not know: `bigTerm` walked straight past it and promoted the sum
above it, which then refused its own operand, and on the fixed-limb rung the same
mistake indexed an `int` as a table. `PostVars`, `LoopBufferReuse` and the
element-width join all pattern-match on `again` arguments and `build` shapes, and
an ascription between them would break the match SILENTLY, which is worse.
**Any build of this checks that first, not last.**

### (b) A local ascription the PROGRAMMER writes

`(the (int 0 (pow 2 200)) e)` at the point of intent.

Strictly more general than (a): it also covers a value with no helper at all,
which (a) cannot reach — a factorial whose accumulator is a loop variable in
`main` has no signature anywhere. And it would later host a local `where`, which
refinements.md has no home for either.

It costs surface, and it makes the programmer say something the program in §2
already says. **(a) and (b) are not alternatives**: (a) is (b)'s mechanism, made
automatic for the case where a declaration already exists. Build (a) and (b) is
one reader rule away.

### (c) Do not inline across a declared representation boundary

Give `unfoldable` a third gate, so a definition declaring a result above the
window stays a call. Attractive on its face: the boundary survives *because it
was declared*, which reads as **a representation boundary is a side condition on
δ**, exactly parallel to ADR 0010's *effects are a side condition on β*.

**But the cost is not in the reducer, it is in the emitter.** `cmd/build` emits
exactly ONE function — `oro-main` — and every program in this repository is one
function by construction. Keeping a callee means the build pipeline must emit a
call graph, which it has never done. That is a large change to justify from one
declaration, and it makes a declaration alter the SHAPE of the emitted program
rather than the representation of a value.

### (d) Promote on unprovability instead

Refused, and it is ADR 0019's escape (A) under another name: promoting whatever
the analysis cannot bound is silent-slow failure and a whole-program boxing
story, which that ADR rejected on four grounds.

## 6. Recommendation

> **BUILT, and one thing the argument had not tested.** Before writing anything,
> the cheaper answer was measured: can the analysis simply DERIVE the magnitude?
> The loop has a constant trip count of six and a constant multiplier, so a bound
> is computable in principle. It reports **`[-inf, +inf]`** — and the general case
> is worse, since a factorial's bound is not expressible in an interval domain at
> all. That measurement is what makes §4's "what is missing is intent" a finding
> rather than an assumption, and it should have been in this document before the
> recommendation was made.

**(a), then (b) if a program needs it.**

(a) restores escape 3 for the shape that already looks like it should work, needs
no new term kind, no new surface and no change to the emission model — and §4a
shows the mechanism working today, which makes it a wiring job rather than a
design. What it adds to `big-str` is that the marker no longer has to be a
rendering. (b) is the general
answer and is cheap once (a) exists, but it should be motivated by a program
rather than by symmetry.

(c) is the more elegant story and the wrong trade today: it buys the same thing
for the price of an emission model this project has never needed.

## 7. What this does NOT settle, and one thing it corrects

**It does not settle where a representation is chosen.** ADR 0019 left open the
boundary between a value stored one way and a value stored another, and (a)
creates ascriptions in the middle of terms — which is a place a conversion might
have to sit. Worth knowing before building.

**It does not settle rendering.** Decimal conversion from limbs also needs
strings; this removes one of its two blockers.

**And it corrects a sentence in this repository.** refinements.md §6b says
*"enforcing a definition's `where` would be a regression, not a fix"* and the
reasoning generalised quietly to every declaration on an internal definition.
That reasoning is sound for the premise and unsound for the representation, and
the two travel in the same syntax. A range is three things, and only two of them
are safe to drop.

## 8. How to falsify this

The corpus cannot: **no program in it declares a big result on a definition that
is not the entry point**, so (a) changes nothing today and the 70 × 4
byte-identical sweep would stay byte-identical. That is a reason to be careful,
not a reason for confidence — a change nothing exercises is a change nothing
checks, which is the shape that hid two bugs in the `-checked` rebuild path.

Any build of (a) needs its own program: §2's, plus one where the helper's result
feeds a further big operation, plus one where the marker sits inside an `again`
argument — the case §5(a) names as the way this breaks.
