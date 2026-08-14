# Derivation: gauntlet program 5 — formatted output, and the binding format

Exploration only. No commitments, no ADR.

> **Correction, 2026-08-14 — implemented, and wrong in two places.**
> The discipline of §5 is now specified in [docs/spec/effects.md](../spec/effects.md) and built,
> measured in [effects-2026-08-14](../../gauntlet/results/effects-2026-08-14.md). Two claims below
> did not survive:
>
> 1. **§5 says effects "arrive" with program 5. They were already here.** `dict-inc` mutates a
>    dictionary in place and `dict-empty` has a fresh identity, both since word count. Program 5
>    made them *observable*; it did not introduce them.
> 2. **§5 lists two hazards and there are three.** Duplication and context change are contraction
>    and exchange; the missing one is **weakening** — an argument used zero times is dropped, and
>    dropping an effect deletes it. That is the hazard that makes `seq` expressible at all, so a
>    discipline built from this derivation alone could not have written program 5.
>
> What did survive: §4's "a binding is a vocabulary entry plus an emission template" — `print-line`
> is one line per target file and no Go — and §6's Tier 2 verdict, now confirmed by our own output
> (Go `1`, JS `1`, Java `1.0` for the same value).

The question: does calling into a host ecosystem cost any new machinery, and does the "file
listing function names" of requirement 4 actually work?

**Result: bindings cost nothing — an extern is a vocabulary entry, so rewriting halts on it.
But program 5 finds the first *correctness* defect in the rewriting core, where every earlier
one was a performance defect.**

---

## 1. What "unreadable output is fine" changes

The stated position: emitted target source may be structurally different per target and may be
cryptic; performance is what matters, readability is a bonus.

This dissolves half of [g2's](g2-structs.md) finding and sharpens the other half.

**Dissolved.** The *interior* of a function is ours. Emit AoS on Go and struct-of-arrays on JS,
mangled names, flattened control flow, duplicated loops, `Float64Array` views, no comments.
None of it needs to correspond across targets, and g2's discomfort about "structurally
different source programs" was aesthetic, not technical.

**Not dissolved.** The *boundary* is not ours. When host code calls our function, or we call a
host function, representation is dictated by the host. That is a semantic contract, not a
readability preference, and no amount of tolerance for cryptic output removes it.

> **Representation is free in the interior and fixed at the boundary.**

Good news follows: since non-recursive functions are rules and get inlined
([g3](g3-generics.md)), boundaries mostly evaporate. The only places representation is pinned
are the program's actual edges — exported functions and extern calls. A small surface.

**One thing the relaxation does not buy:** requirement 6. Small binaries still fight
specialization, and cryptic output does not make duplicated loops smaller.

## 2. The binding format — requirement 4

Tier 2, per [ADR 0002](../decisions/0002-capability-graph.md): names, types, import line. One
file per host package.

```lisp
(binding go/fmt
  (import "fmt")
  (extern print-line ((s string))  -> unit  "fmt.Println")
  (extern print-line ((n i64))     -> unit  "fmt.Println")
  (extern print-line ((x f64))     -> unit  "fmt.Println"))
```

```lisp
(binding js/console
  (extern print-line ((s string))  -> unit  "console.log")
  (extern print-line ((n i64))     -> unit  "console.log")
  (extern print-line ((x f64))     -> unit  "console.log"))
```

```lisp
(binding java/system
  (import "java.lang.System")
  (extern print-line ((s string))  -> unit  "System.out.println")
  (extern print-line ((n i64))     -> unit  "System.out.println")
  (extern print-line ((x f64))     -> unit  "System.out.println"))
```

One Oroboros name, three host calls. That is requirement 4, and it is genuinely a file of
names.

## 3. The program

```lisp
(fn report ((label string) (xs (slice f64))) -> unit
  (require (> (len xs) 0))
  (print-line label)
  (print-line (len xs))
  (print-line (dot xs xs)))
```

`dot` is reused from [g1](g1-dot-product.md), so this also tests a pure computation feeding an
effectful call.

## 4. Externs cost nothing

**Step 0.** `(print-line label)` — is `print-line` in go's vocabulary? Yes, the binding put it
there. **Rewriting halts immediately.**

That is the whole story:

> **A Tier 2 binding is a vocabulary entry plus an emission template.**

No new mechanism, no FFI subsystem, no marshalling layer for scalar types. The capability graph
already had the concept it needed. This is the cheapest result in any of the five derivations.

**Step 1.** `(print-line (dot xs xs))` — `dot ∉ vocab`, so it rewrites per g1 into a `block`.
Which puts a *statement sequence* in an *argument position*, so it must be hoisted:

```lisp
(block
  (let n (len xs))
  (var acc f64 0.0) ...loop...
  (let t acc)
  (print-line t))
```

Confirms [g3 §6](g3-generics.md): ANF normalization is required, and now for a second
independent reason.

## 5. The finding — rewriting can change effect count and order

Every earlier derivation operated on pure terms. g4's Defect 1 duplicated *work*, which was a
performance bug. Program 5 introduces effects, and the same mechanism now produces **wrong
output** rather than slow output.

Two distinct hazards:

**Duplication.** A metavariable occurring twice on a right-hand side duplicates its effect:

```lisp
(rule (log-and-return ?e) => (seq (print-line ?e) ?e))
```

With `?e := (read-line)`, the input is consumed twice. g4's auto let-binding already fixes
this — but note the fix was adopted for *performance* and turns out to be load-bearing for
*correctness*.

**Context change.** This one is new, and the g4 fix does **not** cover it. A metavariable may
occur exactly once on the right-hand side and still be wrong, if its execution context differs:

- Moved *into* a loop → the effect happens *n* times instead of once.
- Moved *into* a conditional arm → the effect may not happen at all.
- Reordered relative to another metavariable → output order changes.

g1's `fold-range` rule is a near miss. `?z` occurs once and stays outside the loop, so
`(fold-range (read-int) n f)` reads once, correctly. A rule that placed `?z` inside the loop
would read *n* times, and nothing in the machinery so far would object.

**The discipline, and it is statically checkable per rule:**

> For every metavariable, its execution-context depth (loop nesting, conditional guarding) and
> its order relative to other metavariables must be the same on both sides of the rule.
> Purity-inferred terms are exempt.

Same species as the deforestation measure check from [g1 §5](g1-dot-product.md): a structural
property of the rule, verified once when the rule is written, not an analysis of every program.

## 6. `print-line` is not portable, and that is the honest answer

Float formatting diverges across all three targets. Measured, not predicted:

| Value | Go `fmt.Println` | JS `console.log` | Java `System.out.println` |
|---|---|---|---|
| `1.0` | `1` | `1` | `1.0` |
| `1e8` | `1e+08` | `100000000` | `1.0E8` |
| `1e21` | `1e+21` | `1e+21` | `1.0E21` |
| `-0.0` (runtime) | `-0` | `-0` | `-0.0` |
| `1.0/3.0` | `0.3333333333333333` | same | same |

Three targets, three answers, on the program whose entire purpose is producing output.

The same check turned up something larger that has nothing to do with formatting: Go prints
`0.3` for the *constant* `0.1+0.2` and `0.30000000000000004` for the same sum computed at
runtime. That is arbitrary-precision constant folding, and it lands on the core's identity
rather than on `print-line`. See
[ADR 0009](../decisions/0009-staging-preserves-results.md) and
[g6 §2](g6-escaping-closures.md).

This is [g4's](g4-word-count.md) `split-words` conformance problem again, but worse, because
the divergence *is* the observable behaviour. Two resolutions:

- **`print-line` stays Tier 2.** Output differs per target, no portability claimed, cost zero.
- **A Tier 1 `print-f64` exists** with a specified algorithm — shortest round-tripping, Ryū or
  Grisu — which means implementing float formatting ourselves rather than parasitizing the
  host's. Costs binary size (requirement 6) on every target.

Both are legitimate; they are different products. Recording it rather than deciding.

It also refines [g3's](g3-generics.md) principle. That derivation said: parasitize the host's
data structures and runtime services, never its abstraction mechanisms. Program 5 adds a
qualifier — **parasitize a host service only where its semantics are pinned down enough to be
interchangeable.** `map` qualifies. Float formatting does not.

## 7. Boundary marshalling, narrowed

g2's crack, restated under §1. If our interior representation of `(slice point)` is SoA on JS
and a host function wants an array of objects, something must convert. The cost is real and
per-call.

But the surface is small: exported functions and extern calls only. And the conversion is
visible — it can be reported, counted, and made an error in a `portable` library. The right
treatment is probably to surface marshalling cost as a diagnostic rather than hide it, in the
spirit of "no mystery about what is emitted."

Scalars, strings, and slices of scalars — which is most of what crosses a boundary — need no
conversion at all.

## 8. Findings

1. **Bindings cost nothing.** An extern is a vocabulary entry plus an emission template;
   rewriting halts on it. Requirement 4 is satisfied by machinery that already existed.
2. **Effects break the rewriting model in a way purity concealed.** Rules can change effect
   count and order — the first *correctness* defect found, where all earlier ones were
   performance defects.
3. **The fix is a static per-rule check** on execution-context depth and relative order, with
   pure terms exempt. Same species as the deforestation measure check.
4. **g4's let-binding fix was adopted for performance and turns out to be required for
   correctness.**
5. **`print-line` cannot be Tier 1 without implementing float formatting ourselves.** All three
   hosts disagree on `1.0` and `1e8`.
6. **Parasitize a host service only where its semantics are pinned enough to be
   interchangeable** — a qualifier on g3's principle.
7. **ANF normalization confirmed necessary** for a second independent reason.
8. **Boundary marshalling is the only surviving part of g2's concern**, and its surface is
   small: exported functions and externs.

## 9. Verdict, and a pattern across all five

The meta-observation from [g2 §7](g2-structs.md) now has a fourth row, and it is the most
serious one:

| Derivation | Property the original term held | Lost by | Severity |
|---|---|---|---|
| g4 | **Sharing** — subterm evaluated once | Naive substitution | Performance |
| g1, g3 | **Capture-freedom** — binders distinct | Rule-introduced binders | Correctness |
| g2 | **Simultaneity** — fields assigned at once | Splitting into a sequence | Correctness |
| g5 | **Effect count and order** | Substitution across contexts | Correctness |

Four independent reasons the same question keeps arising: **when may a term be copied, moved,
or deleted?** Sharing says *copying costs*. Effects say *copying and moving change meaning*.
Linearity, which appeared in g4, g1, and g2 for unrelated reasons, says *moving a value costs*.

That is a substructural question, and it recurring four times from four directions is a strong
hint that the core's type discipline should answer it once, rather than four analyses answering
it separately. Worth exploring before anything is built — it may be the actual core, in the
sense the original question was asking about.

**All five gauntlet programs now derived. Still untested: escaping closures** — every
derivation had function arguments literal at their call sites, so no closure ever formed. That
is the one remaining place "closures are not a core primitive" is assumed rather than
demonstrated, and it is now the last cheap experiment available before the baselines.
