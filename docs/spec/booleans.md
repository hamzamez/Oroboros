# Booleans and control flow

**Status, 2026-08-19. Built** — [ADR 0017](../decisions/0017-booleans-are-in-the-language.md).
Candidate C, as specified in §4, with the falsifier of §6 run first. Written before any code, which
is the order [strings.md](strings.md) exists to enforce, and the specification is unchanged by the
implementation except where §8 says otherwise.

This **reopens** a decision. [arithmetic.md §3](arithmetic.md) records a position, and
[ADR 0012](../decisions/0012-portable-integer-range.md)'s *Why not* rejected two of the candidates
below. Both rested on a premise that is now false:

> "All three hosts have `&&` and `||` natively, and they short-circuit."

The fourth host does not have `&&` at all. It has `and` — one ALU instruction, strict — and it has
branches. Those are different operators, and the target file currently claims they are one.

ADR 0012's actual decision (an `int` is exact within ±(2⁵³−1)) is untouched. Only two entries in
its rejected-alternatives list are overturned, by evidence that did not exist when it was written.

---

## 1. What is true today

All checkable by grep.

**The reader already hardcodes two names.** `core/read.go:628` emits `Name("if")` and `:630` emits
`Name("loop")` when desugaring a `loop`'s clause chain. `if` is therefore *de facto* part of the
language while [target-files.md](target-files.md) describes it as a primitive the target declares.
A target that named its conditional anything else would compile straight-line code and fail on
every loop.

**A boolean literal cannot be written portably.** All four targets declare `true` and `false` under
exactly those names — the only names besides the structural three that all four agree on — and a
program must still write `go.true`, `js.true`, `java.true`, `x64.true`.

**`&&` denotes three different functions across four targets:**

| target | declared | short-circuits |
|---|---|---|
| Go, Java | `(bool bool) bool` | yes |
| JavaScript | `(any any) any` — returns an *operand*, not a boolean | yes |
| windows (`x64.andb`) | `(bool bool) bool` | **only in a guard** |

**And two different functions inside one target.** Emitted, not argued:

```lisp
(fn (a b) (x64.andb (x64.setl a b) (x64.setg a 0)))          ; value position
(fn (a b) (if (x64.andb (x64.setl a b) (x64.setg a 0)) 1 0)) ; guard position
```

```asm
; value — both operands computed          ; guard — short circuit
cmp rdi, rsi                              cmp rbx, rsi
setl dil                                  jge Lelse1
movzx edi, dil                            cmp rbx, 0
cmp r12, 0                                jle Lelse1
setg r12b
movzx r12d, r12b
and r13, r12
```

**The condition's type is demanded but not enforced everywhere.** `emit/check.go:251` walks a
conditional's condition against `bool`. JavaScript declares `&&` as `(any any) any`, and `any` is
not a constraint, so on that target the demand is vacuous.

**`if` never folds.** Reduction evaluates no primitive ([state.md §3](state.md)), and `true` is a
primitive, so `(if go.true a b)` survives to emission on every target.

**The precedent that decides the shape.** A source `let` is *erased* by the reader — `(let e k)`
reads as `(k e)` — and a `let` in a residual can only have been re-introduced by the reducer at
`core/reduce.go:714`, where β declines to substitute. So this language already has **two
vocabularies**: what a program may write, and what a residual may contain. The boolean question is
mostly a question of which vocabulary each name belongs to.

---

## 2. The literature

### 2.1 McCarthy 1960 — the conditional cannot be a function

The conditional expression `(p₁ → e₁, …, pₙ → eₙ)` is the original contribution of *Recursive
Functions of Symbolic Expressions*, and McCarthy is explicit that it is **not** a function: a
function in a strict language receives all its arguments evaluated, and a conditional must not
evaluate the branch it does not take. Sixty-six years later this is still the whole of the matter.

It applies directly to `and` and `or`. In a strict setting they are functions of two *evaluated*
booleans; short-circuiting `and` is control flow wearing an operator's clothes.

### 2.2 Landin 1966 — definitional sugar

*The Next 700 Programming Languages* draws the line this document needs: a construct is either part
of the abstract machine or **definitionally** reducible to it. Sugar that erases in the reader costs
the semantics nothing. Our `let` and `seq` are already exactly this.

### 2.3 Scheme and Standard ML — `and`/`or` are derived

R7RS defines `and` and `or` as **derived expressions**:

```scheme
(and)            = #t
(and e)          = e
(and e₁ e₂ …)    = (if e₁ (and e₂ …) #f)
(or)             = #f
(or e₁ e₂ …)     = (let ((t e₁)) (if t t (or e₂ …)))
```

Standard ML reaches the same answer in a strict, typed setting: `andalso` and `orelse` are
**syntax** in the Definition, derived from `if`, precisely because they cannot be functions.

The `let` in Scheme's `or` exists because Scheme's `or` returns *the value* of the first true
operand, over arbitrary values. Over a two-element `bool` it is unnecessary — `(or a b)` is
`(if a true b)`, and `a` is still evaluated once.

### 2.4 The dragon book — two translations, chosen by context

Aho, Sethi and Ullman give a boolean expression **two** translations: *jumping code*, with inherited
`B.true` and `B.false` labels, for a boolean in control position; and a *numerical representation*
for one in value position. Short-circuiting is a property of the first translation, not of an
operator.

This is precisely the `(jump …)` mechanism added for the windows target — and the literature's
framing shows where that was put wrong. The two translations are a property of **the compiler's
handling of `if`**, not a claim a target author makes about an operator. Making it a target
declaration means a target author can silently get it wrong, which is the exact failure mode
CLAUDE.md already records for `split-words`: *a Tier 1 name without a conformance suite is
decoration*.

### 2.5 Polarity and call-by-push-value — `bool` is data, its eliminator is a branch

Andreoli's focusing (1992), Zeilberger's polarity, and Levy's call-by-push-value (1999) all give
the same classification: `bool = 1 ⊕ 1` is a **positive** type. Positive types are *constructed*
and *eliminated by case analysis*. The eliminator of a positive type is a branch.

So the modern theory agrees with McCarthy: the primitive is the case analysis, and `and`/`or`/`not`
are derived functions over the data. It also says what booleans *are* — the degenerate sum — which
matters for §6.

### 2.6 Church encoding — already refuted, here, by this project

`true = λt.λf.t`, `false = λt.λf.f`, `if b t e = b t e`. [arithmetic.md §2](arithmetic.md) killed
it and derived a rule worth restating:

> An encoding is free exactly when the eliminator's scrutinee is **statically known**. A boolean's
> constructor is a runtime value — that is what makes it a conditional — so `(c a b)` puts a
> variable in operator position, no redex exists, and every emitter rejects the term.

Plus: Church booleans in a strict setting evaluate both branches, and fixing that needs thunks,
which are closures, which allocate.

### 2.7 Commuting conversions, case-of-case, and join points

`(if (if c a b) x y) → (if c (if a x y) (if b x y))` is a commuting conversion (Prawitz 1965) and,
in compilers, GHC's **case-of-case** (Peyton Jones & Santos 1998). It is what makes nested
short-circuit conditions collapse into a chain of branches.

It also **duplicates the continuation**: `x` and `y` each appear twice above. GHC needs *join
points* to avoid the blowup (Maurer, Downen, Ariola & Peyton Jones 2017, *Compiling without
continuations*).

The lesson for us is precise, and it is the crux of the recommendation: **do the conversion in the
backend, in branch position, where the duplicated continuations are the same label** — which costs
nothing — and **never in the term**, where they would be copied.

### 2.8 Multi-way: McCarthy or Dijkstra

McCarthy's `cond` is first-match-wins and deterministic. Dijkstra's guarded commands (1975) choose
nondeterministically among true guards and abort when none holds.

[ADR 0015](../decisions/0015-loop-and-again.md) already chose: first match wins, `else` required,
so every way out is written down. A standalone `cond` should be the same form or the language has
two conventions for one idea.

### 2.9 Languages that ship both operators

Ada has `and` and `and then`. C has `&` and `&&`. Visual Basic has `And` and `AndAlso`. Pascal had a
compiler switch. Every one of these languages, having distinguished them, kept **both** — which is
evidence that the strict, branchless form is genuinely useful and not merely a mistake.

The reason is measurable: on x86 a strict `and` is one instruction with no branch, while a
short-circuit `and` is a branch that can mispredict at roughly fifteen cycles. When the predicate is
unpredictable, branchless wins. This project already has a neighbouring measurement — a compound
loop condition **defeats Go's bounds-check elimination**, costing 1.61× (ADR 0015's context).

### 2.10 Truthiness, and boolean blindness

C's integer conditions produced `if (x = 0)`; Java, Go and Rust all responded by requiring a
genuine `bool`. Our checker already demands one, and the JavaScript target's `(any any) any`
declaration is the one place it leaks.

Harper's *boolean blindness* is the counter-pressure, and it is worth stating so it is not
rediscovered later: a `bool` throws away *why* it is true. That is an argument against ever building
much on top of `bool`, and an argument for making whatever we do the **n = 2 case** of a sum
eliminator rather than a special construct.

---

## 3. Candidates

### A — Status quo, patched

Keep `and`/`or`/`not`/`true`/`false` as per-target primitives. Delete `(jump "and")` and
`(jump "or")` from the windows target so `x64.andb` means one thing.

**For:** smallest possible change; nothing enters the language.
**Against:** every listed defect survives. `if` stays hardcoded in the reader and declared in four
target files. No boolean literal. JS keeps `any`. `if` never folds. Every new target re-declares
five names, and a target author who declares a *strict* `and` gets short-circuiting wrong silently
— with no conformance suite able to catch it, because the difference is only observable through a
trapping or effectful operand.

**Verdict:** this is the do-nothing option and it should be named as such. It is what we do if the
measurement in §6 goes badly.

### B — Church-encoded booleans

**Verdict: dead**, and already dead in this repository — §2.6. Listed so the next person does not
re-derive it.

### C — `bool` is language data, `if` is its eliminator, `and`/`or`/`not`/`cond` are sugar

The Scheme/SML/CBPV answer. Two new literals and one new term kind; `if` moves from target-declared
to core; the connectives erase in the reader; each backend recognises the shapes and emits its
host's native form.

**For:** every defect in §1 closes at once, the theory is settled and sixty years old, and the
residual gains exactly one term kind.
**Against:** a seventh term kind; four target files change; and it appears to violate *never lower
further than the target requires* — see §4.4, where that objection is answered and turned into a
measurement.

**Verdict: recommended.**

### D — Ship both operators, like Ada

`and`/`or` short-circuit, `and*`/`or*` strict and branchless, both in the language.

**For:** §2.9 says every language that distinguished them kept both, and branchless is measurably
better for unpredictable predicates.
**Against:** it doubles the vocabulary for something that is a **target** concern, not a language
one — and this project already has the mechanism for target concerns. `x64.andb` can stay exactly
where it is, host-named, no portability claim, available to a program that measured and wants it.
The language should carry the meaning programs intend; the target carries the instruction.

**Verdict:** rejected as a language feature, adopted as a target-level one that already exists.

### E — Condition contexts as a target obligation

Keep `and`/`or` as primitives and require every target to declare both translations — the current
windows design, generalised.

**For:** no language change; the fastest form is available on each host.
**Against:** it makes short-circuiting **a claim a target author makes**, unverifiable by the
format, and observable only through a trapping or effectful operand. That is `split-words` again:
a name that passed every check for two months while returning different answers on different
targets. The dragon book's framing (§2.4) says the two translations belong to the compiler's
handling of the conditional, not to the operator.

**Verdict:** rejected. It is what we have, and it is the source of the defect.

### F — Sum types now, `bool = 1 ⊕ 1`

**For:** theoretically the right home; `if` becomes the n = 2 case of `case`.
**Against:** blocked on the product/sum allocation measurement
([assessment §3.3](../assessment-2026-08-19.md)), which is the exact place the predecessor project
died. Doing booleans first is not wasted work **provided** the design is the n = 2 case of what a
sum eliminator would be — which C is.

**Verdict:** correct destination, wrong order.

---

## 4. The recommendation, specified

### 4.1 Grammar

Two new literals and one new term kind, `KBool`:

```
term ::= name | integer | float | string | true | false | (fn (name…) term) | (term term…)
```

**Why a term kind is forced, rather than chosen.** The alternative is that `true`/`false` stay
primitives the target declares, as today. But the reader's desugaring of `and` must *produce* a
false value, and the reader does not know which target it is reading for. `go.false` and
`x64.false` are different names and the reader cannot pick one. A reader-level sugar therefore
requires a reader-level literal. That is decisive, and it is why ADR 0012's "not needed" no longer
holds: nothing needed a boolean constant until a desugaring did.

### 4.2 Four new reader forms, all sugar, all erased

```lisp
(and)          ⟶ true
(and e)        ⟶ e
(and a b …)    ⟶ (if a (and b …) false)

(or)           ⟶ false
(or e)         ⟶ e
(or a b …)     ⟶ (if a true (or b …))

(not a)        ⟶ (if a false true)

(cond c₁ e₁ … else e)  ⟶ (if c₁ e₁ (cond … else e))
```

`or` needs no `let` because our booleans are the two-element type, not Scheme's arbitrary values.
Each operand is evaluated at most once, which is the property that matters.

`cond` is `loop`'s clause chain with `again` removed — the same syntax, the same first-match rule,
the same mandatory `else`. `core/read.go` already contains the code; it is factored out, not
written.

**None of the four survives the reader**, exactly as `let` and `seq` do not. A residual contains
`if`, `true` and `false` and nothing else new.

### 4.3 Two reduction rules

```
(if true  a b) ⟶ a
(if false a b) ⟶ b
```

This is the first evaluation the reducer performs, and it needs saying carefully:

- [state.md §3](state.md)'s "no primitive is ever evaluated" **stays true** — `if` is no longer a
  primitive, and `true`/`false` are literals rather than primitive applications.
- [ADR 0009](../decisions/0009-staging-preserves-results.md) is satisfied trivially. Boolean
  algebra is exact; there is no compile-time/runtime discrepancy to preserve.
- Dropping the untaken branch is sound **even when it is impure**, and for a different reason than
  β's. β may not drop an impure argument because the argument would have run. Here the branch
  genuinely does not run. The structural rules are not involved.

**What this buys, and it is new capability.** A condition known at compile time compiles to no code
at all:

```lisp
(def debug? false)
(def f (fn (x) (if debug? (seq (log x) x) x)))     ⟶  (fn (x) x)
```

Conditional compilation with no preprocessor, no `#ifdef`, and no build tags — from a definition
and δ. That is [the-atom.md](../the-atom.md)'s staging claim collecting a concrete win, and it is
available on every target including the one with no optimiser.

It also makes `(and true p)` reduce to `p`, and `(not (not p))` reduce to `p`, without a single
rewrite rule about booleans.

### 4.4 Emission — answering "never lower further than the target requires"

This is the objection that killed the idea in ADR 0012, and it deserves a direct answer.

**The rule is about the emitted artifact, not the internal representation.** If the Go backend emits
`p && q`, no lowering has occurred, whatever the residual looked like on the way. The real question
is whether the backend can *reliably recognise* the shape, and here it can: `(if p q false)` with a
**literal** `false` in the third position is a one-node syntactic match. This is not "recover
structure from `goto`"; it is a pattern with no analysis in it.

Per backend, in value position:

| residual | Go / Java | JavaScript | windows |
|---|---|---|---|
| `(if p q false)` | `p && q` | `p && q` | branch to a shared label |
| `(if p true q)` | `p \|\| q` | `p \|\| q` | branch to a shared label |
| `(if p false true)` | `!p` | `!p` | invert the condition code |

In **guard** position every backend already has the machinery: Go, JS and Java emit the host
operator into their `if`, and windows recurses `branchUnless` through the nested conditional to
shared labels — which is the dragon book's translation and the case-of-case of §2.7, done where the
duplicated continuations are one label rather than two copies.

`(jump "and")` and `(jump "or")` are **deleted** from the format. They were a target's claim about
short-circuiting; the language now owns the answer.

### 4.5 What leaves the target files, and what stays

**Leaves:** `(structural if cond pure)` from all target files — `if` is core. `true` and `false` as
primitives. `logic.and`/`or`/`not` from the portable layer.

**Stays:** `(type bool "…")`, because how a boolean is *spelled* is genuinely the target's business
— `boolean` on Java, `qword` on windows.

**Stays, deliberately:** `x64.andb`, `x64.orb`, `x64.notb` as host-named, no-portability-claim
primitives. They are the strict branchless operators of §2.9, and a program that has measured its
predicate as unpredictable can reach for them. This is exactly the two-tier structure the project
already has, and it is why candidate D is unnecessary.

### 4.6 Typing

`if` demands `bool`. The JavaScript target's `(any any) any` declaration for `&&` disappears with
the primitive, closing the one place the demand was vacuous. `bool` remains one of the four reserved
type names.

### 4.7 Conformance

*A Tier 1 name without a conformance suite is decoration.* `gauntlet/conformance/cases.json` gains,
at minimum: short-circuit with a trapping second operand on all four targets; `(if 1 …)` rejected;
`(and)` and `(or)` at zero and one operand; static folding of a definition-known condition; and De
Morgan on all four.

---

## 5. The three questions

[state.md §6](state.md) demands these of every addition.

**1. What does it mean, independently of any target?** `bool` is the two-element type with
inhabitants `true` and `false`. `(if c a b)` requires `c : bool` and yields `a` or `b`, evaluating
exactly one. `and`, `or`, `not` and `cond` are the abbreviations in §4.2 and have no independent
meaning at all.

**2. What does each target do with it, and do they agree?** Go, Java and JavaScript emit `&&`,
`||`, `!` and their native `if`; windows emits branches. All four evaluate at most one branch and
at most one operand beyond the first. They agree.

**3. If they disagree, is the disagreement observable?** They agree — which is the point of the
change. *Today* they disagree and it **is** observable: `(x64.andb p (x64.setl (x64.idiv n d) 5))`
with `d = 0` raises #DE on windows and returns cleanly on Go. That is what makes this a correctness
item rather than a convenience.

---

## 6. The falsifier, run

**The objection ADR 0012 killed this on does not reproduce** —
[and-form-2026-08-19](../../gauntlet/results/and-form-2026-08-19.md), measured before any of §4 was
written.

| host | shape | result |
|---|---|---|
| Go | `p && q` in value position | no difference |
| Go | two-term loop guard | no difference |
| Go | three-term predicate | nested ahead 1.11× — **below the noise floor**, recorded as "not slower" |
| JavaScript | three-term predicate | no difference |

`-d=ssa/check_bce/debug=1` reports every bounds check eliminated in **both** forms, so ADR 0015's
1.61× does not reproduce here either: that was an array index under a compound *bound*, and this is
a compound *predicate* over already-loaded values. Different shapes, and only the first defeats BCE.
The hypothesis was reasonable and wrong.

**This lowers the stakes on §4.4.** The peepholes are for legibility of emitted code, not for speed.

Java and windows are not measured. On windows the comparison does not exist — the host has no `&&`
— and Java is expected to track Go; both should be run before the change lands.

Three falsifiers remain:

- **The peepholes turn out not to fire.** If a residual `(if p q false)` is reliably reachable in
  real programs the match is trivial; if reduction rearranges it into something else first, the
  emitted code degrades to an if-statement and a temporary. Cheap to test on the seven gauntlet
  programs once they carry an `and`.
- **Static folding turns out to be unreachable.** If no program can produce a definition-known
  condition, §4.3's headline benefit is theoretical. Testable immediately by writing one.
- **The seventh term kind spreads.** If `KBool` requires more than a case in the reader, the
  printer, `freeVars`, `subst`, the checker and each backend, the estimate was wrong and the cost
  should be re-argued.

## 7. Cost

| | before | after |
|---|---|---|
| term kinds | 6 (+`KBound`) | **7** (+`KBound`) |
| reader forms | `fn`, `let`, `seq`, `loop` | + `and`, `or`, `not`, `cond` — all erased |
| reduction rules | β, δ | + `if` on a literal |
| structural primitives a target declares | `let`, `if`, `loop` | **`let`, `loop`** |
| names a target must declare for booleans | 5 (`&&`, `\|\|`, `!`, `true`, `false`) | **0** |
| backend work | — | one shape-match per backend, three shapes |

The language gains one term kind and one reduction rule. Everything else is sugar that does not
survive the reader, or work that moves *out* of the target files.

---

## 8. What building it changed

Nothing in the design. Four things worth recording:

**Two latent bugs.** Neither the Go nor the Java backend's `typeOf` had a case for `cond`, so an
emitted function whose body was a conditional returned `/*unknown*/`. No program had one until
`and` became a conditional.

**The refinement fragment had to learn the desugaring.** `emit/linear.go` recognised `and`; a
`where` written with the language's `and` now arrives as `(if a b false)`. Conjunction only — `or`
and `not` desugar to a disjunction and a negation, and the fragment is conjunctions of linear
inequalities.

**`(jump …)` kept its single-predicate job and lost the pseudo-codes.** `"and"` and `"or"` are
gone from the format; a target still declares a condition code for a comparison, which is what it
was for.

**Conformance is a program, not a claim.** `examples/native/shortcircuit-{go,win}.oro` put an
`idiv`/`/` by a zero divisor behind the guard that must short-circuit. Both print `222` then `111`.
The divisor is a loop variable rather than a literal on purpose: reduction substitutes literals and
Go's compiler then rejects `10 / 0` outright, which would have tested the Go front end instead of
the semantics.
