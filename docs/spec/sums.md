# Sums

**Status**: built, 2026-08-22. Closed, finite, non-recursive. Zero new term kinds.

The research is [sums-research.md](../sums-research.md) and the algebra is
[type-algebra.md](../type-algebra.md); this is what got built and what building it changed.

---

## 1. The one sentence

**A sum is Σ over a finite index set — the exact dual of a table's Π — and the difference is the
whole design: a Π can be given by a RULE and store nothing, while a Σ must carry WHICH.**

The tag is information the caller does not have, so it has to be transmitted. A sum value is
therefore a **tag and a payload**, which is a *product* — and the product was already built on all
four targets ([values.md](values.md)). Go's own `(T, error)` idiom is exactly this shape, which is
why the host that has no sum type turned out to need nothing.

## 2. Surface

```lisp
(sum result (ok int) (err int))
(sum colour red green blue)              ; no payloads — an enum

(ok 42)                                  ; construct
(case r
  (ok v)  (go.+ v 1)
  (err e) (go.- 0 e))                    ; eliminate
```

Sums are **named**; products are anonymous. That is forced rather than chosen: `(ok 3)` does not
determine its type, which is why every language without runtime types went nominal here.

A variant with no payload is the **degenerate case**, not a separate concept — so an enum needed
nothing added.

## 3. What it cost the compiler

| | before | after |
|---|---|---|
| term kinds | 7 | **7** |
| reduction rules | 3 | 3 + two narrow additions (§5) |
| backend changes | — | `multiTail` on each: return from every leaf |
| target declarations | — | **none** |

A sum declaration generates **definitions**:

```
ok      = (fn (#p) (fn (#x) (#x 0 #p)))     ; = (values 0 p)
ok#tag  = 0
err     = (fn (#p) (fn (#x) (#x 1 #p)))
err#tag = 1
```

So a constructor is an ordinary definition, and qualification, imports, δ and the occurrence
counter all apply to it without any of them learning that sums exist.

`case` expands in **`Load`**, not in the reader — the one structural difference from `match`. The
reader sees one file, and an error type is declared in another; `Load` sees every module. It also
makes exhaustiveness checkable, and `X#tag` is why a clause can emit a *name* and let δ resolve it
wherever the sum lives.

## 4. Both levels are free, and that was the claim to test

**Static** — the tag is known:

```lisp
(case (ok n) (ok v) (go.+ v 1) (err e) e)      ⟶      (go.+ n 1)
```

Nothing. No tag, no test, no product. The Church-encoded sum always did this
([sums-research §0](../sums-research.md)).

**Dynamic** — the tag depends on runtime data:

```lisp
(case (if (go.> n 0) (ok n) (err 0)) (ok v) (go.+ v 1) (err e) e)
  ⟶      (if (go.> n 0) (go.+ n 1) 0)
```

Also nothing — no tag, no closure, no allocation, and no dispatch the `if` was not already doing.
That is **case-of-case**, §5.

**Nested** — three sums deep, each with a runtime tag, reduces to plain control flow with no
struct, no interface and no allocation.

## 5. The two reduction additions, both narrow

**`=` folds on integer literals.** Without it the static sum reduced to `(if (= 0 0) … …)` — the
sum had vanished and left a *tautological test* behind, which is a static cost the two-level
language says should not exist. This is the first entry in the constant-folding table
[tables.md](tables.md) predicted, where `((array 1 2 3) 1) → 2` and `(go.+ 1 2) → 3` are the same
kind of step rather than new rules.

[ADR 0009](../decisions/0009-staging-preserves-results.md) permits exactly this and no more:
integer equality inside the portable window is bit-identical on every target. That is the thing
which is **not** true of float arithmetic — Go folds `0.1+0.2` one way at compile time and another
at run time — so nothing here folds a float, and `TestEqFoldsOnIntegersOnly` pins it.

**An eliminator commutes through `if` and `let`.**

```
((if c A B) k…)          ⟶  (if c (A k…) (B k…))
((let v (fn (x) B)) k…)  ⟶  (let v (fn (x) (B k…)))
```

The first is Prawitz's commuting conversion and GHC's case-of-case. The second is its companion,
and **the nested test is what demanded it**: β itself puts a `let` between a constructor and its
eliminator when a shared subterm is not duplicable, so handling only `if` reduced two levels of a
three-level nest and stopped.

So the rule is better stated as: **push an eliminator through anything β can leave in operator
position**, which in this language is exactly `if` and `let`.

**Only when every argument is pure.** The rule duplicates its arguments into both branches, and
[ADR 0010](../decisions/0010-effects-as-structural-rules.md) denies contraction for an impure term.
Left stuck, an impure eliminator is reported by the emitter rather than silently mis-ordered.

**Measured before shipping**, because the build order demanded it: across **184 residuals** — every
example on all four targets — case-of-case changes **nothing**. It fires only where a sum is
eliminated, so it is purely additive.

The known hazard is code growth: `k` appears twice, so nested cases multiply. GHC's answer is join
points and **`again` is one**, which is the direction if it ever bites. It has not yet.

## 6. Crossing a boundary

Inside a program reduction removes every sum. What survives is a sum in a **signature**, and there
the tag has to be transmitted:

```lisp
(sig div ((a int) (b int)) result)
```

becomes two results, and two results already exist on all four targets:

| | |
|---|---|
| **Go** | `func DDiv(a, b int) (int, int)` — registers, nothing built |
| **JavaScript** | `return {f0: …, f1: …}` — 1.62× better than an array ([multiresult](../../gauntlet/results/multiresult-2026-08-22.md)) |
| **Java** | a `record`, shared by result shape, scalar-replaced by C2 |
| **windows** | `rax`/`rdx` by our own convention |

**No target declares any of it.**

What each backend needed was `multiTail`: a function returning a sum **returns from several
places**, and the single-leaf path would have built a temporary, assigned it in both branches and
returned it at the end. Returning at the leaf is what a programmer writes — and on V8 it is also
worth 1.31× ([native-js-2026-08-20](../../gauntlet/results/native-js-2026-08-20.md)).

```go
func SumStep(a int, b int) (int, int) {
	if (b == 0) {
		return 1, 1
	}
	if ((a % b) > 0) {
		return 1, 2
	}
	return 0, (a / b)
}
```

**Measured at 1.00× against hand-written, zero allocations** —
[sums-2026-08-22](../../gauntlet/results/sums-2026-08-22.md).

**A sum crossing a boundary needs a uniform payload type**, because the payload gets one slot.
Inside a program a mixed sum is fine, since reduction removes it — so the refusal is on the
*signature*, not the declaration.

## 7. Exhaustiveness, and what it buys

A sum is closed and finite, so "did the clauses cover it" is decidable by **counting**. That is the
cheap half of what pattern matching costs; the expensive half — nested patterns, ML's usefulness
algorithm — is not owed, because our patterns are flat.

It pays for itself immediately: **the last clause carries no test.** Once the others are excluded
the last one is the only thing left, so exhaustiveness is not a safety tax here — it removes a
branch. An `else` is how a deliberately partial match says so, and an `else` under a complete match
is reported as dead code.

## 8. What is refused

| | why |
|---|---|
| **recursive sums** | a JSON node is a *non-recursive* sum plus indices into a table, which measured **2.02× faster** on irregular access ([indexgraph](../../gauntlet/results/indexgraph-2026-08-21.md)). μ buys nothing and costs the size-change termination argument |
| **untagged unions** | `float \| int` is idempotent, is not a coproduct, needs a runtime type test three of four hosts lack, and needs subtyping |
| **one-variant sums** | that is just the payload |
| **a variant in two sums** | a constructor names one sum; sums are nominal |
| **mixed payloads at a boundary** | the payload gets one slot (§6) |

## 9. Still open

**The niche encoding.** `(option ptr)` where `none` is `0` — NULL, −1 and HRESULT's sign bit are
all sums encoded in the payload's own value space, so declaring the niche *names* the
representation a host API already uses rather than adding one. Not built; it is a representation
choice under an unchanged semantics, which is the right shape for a later step.

**`try` as bind.** Sugar over `case`, per [sums-research §3.1](../sums-research.md).

**`match` on a sum.** `case` is the eliminator today, and it does not carry `again`. Making a
sum-typed scrutinee contribute *two* loop variables to a `match` would give state machines over
sums — the parser shape — and it is not free: `(again (err 3))` has to split a sum into two loop
variable updates, and doing that for an opaque sum puts `again` under a lambda, which
[ADR 0015](../decisions/0015-loop-and-again.md) forbids. Named here rather than guessed at.

---

## 7. Is `case` needed, or is `match` enough?

hamza asked, and the honest answer is that **`match` is not enough today, and the reason is smaller
than it looks.**

### Which reduces to which

Neither. They are siblings that land on different primitives:

```
match  →  loop      (reader sugar)
case   →  if        (expands in Load)
```

`case` reduces to `if` directly. `match` reduces to `loop`, and `loop` is a primitive every backend
implements. So there is no path from one to the other.

### What used to separate them, and no longer does

The **cost**. `match` desugars to `loop`, so using it as a plain conditional emitted a loop that
always breaks — where `case` emitted an `if` chain. That is gone: a `loop` with no `again` is now
dropped to a β-redex ([match.md §5b](match.md)), so for the same test both produce the *same*
residual.

### What still separates them

**Knowledge.** `match` runs in the **reader**, which sees one file. A variant pattern `(ok v)` needs
the sum's tag numbers and its variant list, and an error type is declared in another module. `case`
runs in **`Load`**, where every module's sums are visible — which is what buys two things `match`
cannot have today:

- **Exhaustiveness**, which is decidable by counting because a sum is closed and finite, and which
  is what earns the last clause needing no test.
- **A variant pattern at all.** `match`'s integer pattern must be a literal, because a bare name in
  a pattern *binds*. `ok#tag` is a name, so it cannot be written as a `match` pattern without a new
  pattern kind.

### The unification, and its one real cost

`match` could gain a variant pattern and `case` could disappear. The reader can even emit the tag
as a **name** and let δ resolve it — that is the trick `case` already uses. What it cannot do in the
reader is check exhaustiveness, so that check would have to move or be given up, and giving it up
costs the branch it removes.

The other cost is the one this document already named: `(again (err 3))` must split a sum into two
loop-variable updates, and doing that for an *opaque* sum puts `again` under a lambda, which
[ADR 0015](../decisions/0015-loop-and-again.md) forbids.

**Recommendation: keep both for now, and treat it as owed rather than settled.** Two eliminators is
a real smell and the argument for merging them is good. But the merge should be driven by a program
that wants `again` over a sum — a parser whose state *is* a sum is the obvious one — because that is
the case that decides whether the loop-variable split is worth building. Merging before that would
be arguing rather than measuring.
