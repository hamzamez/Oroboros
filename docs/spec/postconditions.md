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

[refinements.md §6b](refinements.md) records what a precondition means depending on where it is
written. A postcondition means three things too, and on a `prim` and an export **each is the swap of
the precondition's**:

| on | precondition `P` | postcondition `Q` |
|---|---|---|
| a `prim` | **obligation** — discharged at every call site | **assumption** — granted where `P` was discharged |
| an **exported** definition | **assumption** — the caller is outside the program | **obligation** — checked against the body |
| an **internal** definition | **obligation at every call it is inlined into** ([ADR 0028](../decisions/0028-a-definitions-contract-is-checked-at-its-calls.md)) | **redundant** — inlining is stronger |

The first two rows exchange the two roles. On the third, reduction removes the boundary, and the two
roles part: `Q` is re-derived at every inlined site, so declaring it adds nothing. `P` is the
caller's duty, and nothing re-derives a duty. Until ADR 0028 it was dropped as `Q` is, which was sound
for every obligation inside the body and left the declared domain unchecked.

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

refinements.md §6b drew the same conclusion for preconditions, and ADR 0028 kept half of it: the
propagated obligations are still what protect safety, and they are still checked. The declared
precondition is checked **as well**, because it is a duty and not a summary. A postcondition is a
summary of the body, so this section's argument stands for it.

## 4. Two soundness lemmas, and both are load-bearing

### Lemma 1 — an assumption needs its precondition

**If `P(a⃗)` has not been proven, `Q(a⃗, y)` must not be assumed.**

*Proof.* `C(f)` is an implication; from an unknown antecedent nothing follows about the consequent.
Concretely, let `f = λx. x` with `P ≜ x > 0` and `Q ≜ result > 0`. `C(f)` holds. At the call
`f(−5)`, `P(−5)` is false and `Q(−5, −5)` is false, so assuming `Q` introduces a false fact — and
the fragment is a conjunction of linear inequalities, from which one false fact derives everything.
∎

The subtlety is that in this compiler **"not refused" is not "proven"**. `discharge` had a path that
reported *"refinement propagated, not proven"* and returned success, because an atom outside the
decidable fragment was reported rather than assumed. Treating that as proof would have licensed `Q`
on an unproven `P`.

> This was not hypothetical. The first implementation did exactly that, because the refactor that
> gave `discharge` a *proven* result rewrote every `return nil` in it to `return true, nil`,
> including the propagated path. `TestEnsuresIsNotAssumedWhenThePreconditionIsUnproven` caught it.

**Since 2026-09-25 that path refuses the program** ([refinements.md §3a](refinements.md)), so a walk
that reaches `Q` has proven `P`. The distinction survives in the **speculative** walks (Houdini, the
proof-by-cases probes), which only report and never refuse. There `discharge` still returns *not
proven*, and `Q` is still withheld.

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

## 5. A pure call: an atom, and where its guarantee holds

A pure call is *substituted*, so it usually has no binder. **The linear fragment names it anyway, as an
atom keyed by its printed form** ([theories.md §7.9](theories.md), `pureAtoms`): the same closed term
denotes the same value, which is Lemma 2's condition for a pure call. So `(hex.EncodedLen (len src))`
is a variable of the fragment, and a fact about it can discharge an obligation that mentions it.

Its `ensures` is that fact, instantiated about the call term **wherever the refinement layer reads
the term**. There are two places:

1. **A primitive's arguments**, before its `where` is decided: a call's arguments are evaluated before
   it, so their guarantees are in scope for its precondition, and for everything after it in scope.
2. **A `build`'s size.** `(build b n …)` records `len b = n`. When n contains a pure call, the
   call's guarantee is what gives that equation content. Without it, `len b = EncodedLen(len src)`
   relates `len b` to an unknown and proves nothing. A `build` is a structural form, not a
   primitive, so rule 1 never read its size.

**A `let` needs no rule of its own.** A pure value is substituted by β and leaves no binder. A pure
call inside an impure value, such as an allocation sized by `(hex.EncodedLen (len src))`, is some
primitive's argument, and rule 1 has already assumed its guarantee when the binder is recorded.

**An instance makes its terms present.** A fact's trigger is a term present in the program (facts.md
§7), and an `ensures` instance can name one that no program term contains. DecodedLen's
`result = x/2` introduces the quotient `x/2`, whose floor facts, 2q ≤ x < 2q + 2 for x ≥ 0, are what
give it meaning. So the lang facts are instantiated on each instance as it is assumed.

The instance holds from the point the term is evaluated onward, in that scope, and **only where the
call's own `where` is proven** (Lemma 1): a guarantee is conditional on its precondition, and a
precondition not proven licenses nothing. Outside a speculative walk, it refuses the program
(refinements.md §3a). At each of the three places the call's `where` is
itself an obligation, decided in the same walk.

**A declaration states its law as an `ensures` when the law is linear.** `EncodedLen` is the map
n ↦ 2n, and its comment said so. But a comment carries no meaning (ADR 0024), so `Encode`'s
precondition `2·len src ≤ len dst` could not be discharged by the buffer Go's own documentation
allocates, `make([]byte, hex.EncodedLen(len(src)))`. It is now
`(ensures (= result (* 2 n)))`, and DecodedLen's is `(ensures (= result (/ x 2)))`: the fragment
reads a quotient by a positive literal as an atom, with its floor facts.

What remains opaque is a **non-linear** guarantee, such as `Mul64`'s `hi·2⁶⁴ + lo = x·y`. That law is
named in its declaration's comment, and nothing in the fragment can use it.

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
