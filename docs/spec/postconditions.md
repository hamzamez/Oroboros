# Postconditions

A `where` says what a call **requires**. An `ensures` says what it **guarantees**.

```lisp
(prim size ((v any)) int expr "size(%s)" pure (ensures (<= 0 result)))

(sig clampish ((n int)) int
  (where   (and (<= 0 n) (< n 1000)))
  (ensures (and (<= 0 result) (<= result 100))))
```

`result` names the value the call produces. It is reserved **only inside an `ensures`**, and a
parameter may not take the name where one is present — otherwise the clause would mean two things
and the checker would pick one silently.

---

## 1. The contract

Let `f` have parameters `x⃗`, precondition `P(x⃗)` and postcondition `Q(x⃗, result)`. The contract is

> **C(f) ≜ ∀x⃗. P(x⃗) ⟹ Q(x⃗, ⟦f⟧(x⃗))**

It is used in two directions, which is the same shape `sig` already had — *a claim checked in two
directions* ([types.md](types.md)):

- **assume**: at a call `f(a⃗)` whose `P(a⃗)` has been discharged, `Q(a⃗, y)` may be added to the
  facts, where `y` names that call's result;
- **check**: given a body `B` with `⟦B⟧ = ⟦f⟧`, verify `∀x⃗. P(x⃗) ⟹ Q(x⃗, ⟦B⟧(x⃗))`.

## 2. Where each direction applies, and why it is exactly two places

[refinements.md §6b](refinements.md) found that a precondition means three different things
depending on where it is written. A postcondition means three things too, and **each is the swap of
the precondition's**:

| on | precondition `P` | postcondition `Q` |
|---|---|---|
| a `prim` | **obligation** — discharged at every call site | **assumption** — granted where `P` was discharged |
| an **exported** definition | **assumption** — the caller is outside the program | **obligation** — checked against the body |
| an **internal** definition | **dropped** — inlining is stronger | **redundant** — inlining is stronger |

Every row exchanges the two roles, and both vanish on the third for the same reason: **reduction
removes the boundary.** So the implementation has two cases, not six.

The asymmetry is not arbitrary. `P` is the caller's duty and `Q` is the callee's, so whichever side
of the boundary we can see is the side that gets checked, and the other is assumed. For a `prim` we
can see the caller and never the body. For an exported definition we can see the body and never the
caller.

## 3. Why an internal definition's postcondition is redundant

**Theorem.** Let `f` be a non-exported definition with body `B`. Reduction replaces every call
`f(a⃗)` with `B[x⃗ := a⃗]`. Then a declared `Q` on `f`:

1. cannot be *assumed*, because after reduction there is no call site to assume at; and
2. can be *checked* only by applying the analysis to `B` — and at each inlined occurrence the
   analysis is applied to `B` again, with the caller's concrete arguments, which refine the
   parameters' abstractions. So anything the declaration could establish, the analysis already has
   at the site.

**Caveat, stated rather than hidden.** Step 2 assumes the analysis is monotone under refinement of
its inputs. Widening is not monotone in general, so there is a theoretical case where `Q` is
provable at the definition and not re-derivable at the site. The empirical evidence points the other
way: [intervals-2026-08-19](../../gauntlet/results/intervals-2026-08-19.md) records that *where a
call site is concrete, everything is provable* — the site is the stronger position, not the weaker.

This is [refinements.md §6b](refinements.md)'s conclusion for preconditions, arriving from the other
end: *"a naive fix would be a regression"*, because the declaration is a conservative **summary** and
the propagated truth is not.

## 4. Two soundness lemmas, and both are load-bearing

### Lemma 1 — an assumption needs its precondition

**If `P(a⃗)` has not been proven, `Q(a⃗, y)` must not be assumed.**

*Proof.* `C(f)` is an implication; from an unknown antecedent nothing follows about the consequent.
Concretely, let `f = λx. x` with `P ≜ x > 0` and `Q ≜ result > 0`. `C(f)` holds. At the call
`f(−5)`, `P(−5)` is false and `Q(−5, −5)` is false, so assuming `Q` introduces a false fact — and
the fragment is a conjunction of linear inequalities, from which one false fact derives everything.
∎

The subtlety is that in this compiler **"not refused" is not "proven"**. `discharge` has a path that
reports *"refinement propagated, not proven"* and returns success, because an atom outside the
decidable fragment is reported rather than assumed ([refinements.md §3](refinements.md)). Treating
that as proof would license `Q` on an unproven `P`.

> This is not hypothetical. The first implementation did exactly that, because the refactor that
> gave `discharge` a *proven* result rewrote every `return nil` in it to `return true, nil` —
> including the propagated path. `TestEnsuresIsNotAssumedWhenThePreconditionIsUnproven` is what
> caught it, and it is the reason the test exercises the **propagated** path specifically: a
> precondition that is *refused* aborts the walk, so the downstream effect is never reached and
> nothing is learned.

### Lemma 2 — a postcondition attaches to the binder, not to the call

**For an impure `f`, `Q` must be recorded about the name the result is bound to, not about the term
`f(a⃗)`.**

*Proof.* Two occurrences of `f(a⃗)` denote different values when `f` is impure. The fact layer keys
by printed term, so recording `Q` about `f(a⃗)` would let one occurrence's guarantee discharge
another occurrence's obligation. ∎

The binder always exists, and that is [ADR 0010](../decisions/0010-effects-as-structural-rules.md)'s
doing rather than a new requirement: **an impure argument is never substituted, it is let-bound at
the application site.** So `let` is exactly where an impure call's guarantee attaches, and it is
there because of a rule written for `print-line`.

For a **pure** call the printed term is a sound key, by referential transparency in a closed
residual.

## 5. What this does not do, and the reason is Lemma 2

A pure call is *substituted*, so it usually has no binder — and the linear fragment cannot name the
value of a general application. It can name an integer literal, a parameter, and `alen(t)`; a call
is not among them.

So for a pure primitive, `ensures` is carried as an **opaque atom** and discharges an obligation only
when the two are syntactically identical. `(ensures (< 0 result))` will not discharge a downstream
`(<= 0 k)`, because `0 < e` and `0 <= e` are different atoms and nothing relates them.

**The fix is real and is not built**: treat a pure application as an opaque linear *variable* keyed
by its printed form. That is sound — same closed term, same value — but it moves terms out of the
"outside the fragment, report it" path into the fragment, where a previously-reported obligation
becomes a hard refusal. That is a behaviour change across every existing program and wants its own
measurement.

Until then the rule of thumb is exact: **`ensures` on an impure primitive works through the linear
layer; on a pure one it works only by syntactic match.** The Win32 case — `_Ret_maybenull_` on
`VirtualAlloc` — is impure, which is the case that works.

## 6. Checking the other direction

For an **exported** definition the caller is outside the program, so nothing else can establish `Q`
and the body is the only evidence. `CheckEnsures` decides it against the interval analysis's value
for the body, because a `loop` has no linear form and the refinement layer cannot read one.

It decides the **constant-bounded** fragment — `K ≤ result`, `result ≤ K`, and conjunctions of those.
Three outcomes, and all three are visible:

```lisp
(ensures (and (<= 0 result) (<= result 100)))   ; holds        → accepted
(ensures (and (<= 0 result) (<= result 50)))    ; false        → refused
(ensures (<= result n))                         ; relational   → reported
```

```
gen: gen-clampish: the body does not establish (if (<= 0 result) (<= result 50) false);
     its result is [0, 100]
note: gen-clampish: postcondition is outside the decidable fragment, propagated and not
     proven: (<= result n)
```

A conjunction is decided only when **both** halves are: a conjunction is as good as its weakest
part, and one undecidable half makes the whole undecidable rather than making the decidable half
count.

## 7. The relational case, and the theorem that would settle it

The postcondition this project most wants is relational:

```lisp
(sig scan-string ((src (array int)) (i int)) int
  (ensures (and (< i result) (<= result (len src)))))
```

That is what [precision-integers.md §3](../precision-integers.md) identified as the fact both the
JSON tokeniser and Java's index narrowing are blocked on. §6 cannot decide it — an interval has no
`i` in it — and §3 says that on an *internal* definition the declaration is redundant anyway.

So the honest route is to **derive** it. It is derivable, and the proof is short.

**Theorem (loop monotonicity).** Let `L = (loop (fn (v₁ … vₘ) body) z₁ … zₘ)`. Say position `k` is
**non-decreasing** if for every `again` reachable in `body` with arguments `a⃗`, `⊢ aₖ ≥ vₖ`. Then
every value `vₖ` takes satisfies `vₖ ≥ zₖ`, and every exit expression `e` with `⊢ e ≥ vₖ` satisfies
`e ≥ zₖ`.

*Proof.* Induction on the iteration count `n`. For `n = 0`, `vₖ = zₖ`. For `n → n+1`,
`vₖ⁽ⁿ⁺¹⁾ = aₖ ≥ vₖ⁽ⁿ⁾ ≥ zₖ` by the hypothesis and the induction hypothesis. An exit expression `e`
is evaluated at some iteration `n`, where `e ≥ vₖ⁽ⁿ⁾ ≥ zₖ`. ∎

**Corollary.** In `scan-string`, `j` starts at `i+1` and every `again` gives `j+1` or `j+2`; every
exit is `j` or `j+1`. So the result is `≥ i+1 > i` — which is the size-change witness the tokeniser's
outer loop is missing, and it needs no declaration at all.

`⊢ e ≥ v` needs only a syntactic relation: `e` is `v`, or `v + c` with `c ≥ 0`, or a conditional
both of whose branches satisfy it. That is a small closed rule set and it is **not built**; it is
the next thing, and it is what would make §3's redundancy claim true in practice rather than only in
principle.
