# Refinements

Written before the code, per [state.md §6](state.md). Layer 1's second half; the first half is
[types.md](types.md).

> **Status, 2026-08-15. Built.** `emit/linear.go` and `emit/refine.go`. `aindex` carries its
> bounds refinement on Go, and every obligation in every example is discharged.
>
> **It found a real latent bug in two of our own gauntlet programs on the day it was written.**
> `dot` and `centroid` each index *two* arrays with one loop bounded by the *first* — nothing
> related the lengths, so `(aindex q i)` was genuinely unproven. Both now declare
> `(where (int.eq (alen p) (alen q)))`, which is the precondition moving to the caller exactly as
> [types-sketch §2](../types-sketch.md) described.
>
> Two things the implementation taught. An **equality must be a substitution, not two
> inequalities** — that is what lets a fact about `p` discharge an obligation about `q`. And
> entailment needed **sums of two facts**: `i < alen p` plus `alen p ≤ alen q` gives `i < alen q`,
> a shape one fact can never reach and every two-array loop produces.

---

## 1. What this is for

Two holes in the specification are **shaped exactly like a refinement**, and both currently say
*stay inside, nothing checks it*:

| | the obligation | what happens outside it |
|---|---|---|
| [primitives.md §2](primitives.md) | `0 ≤ i < alen v` for `aindex` | Go panics, Java throws, **JS silently returns `undefined`** |
| [arithmetic.md §4](arithmetic.md) | `-(2⁵³−1) ≤ n ≤ 2⁵³−1` | Go and the JVM wrap, **JS silently rounds** |

The first is the one to close. It is the only place in the language where a *Tier 1* primitive is
Tier 1 only conditionally, and the condition is unchecked.

**This is a correctness deliverable, not a speed one.** The bounds-check *performance* win was
already collected as an emitter pattern with no types at all
([bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md)), and types-direction §2.2 was explicit
that the type system must not be justified by cases the emitter can already see.

## 2. Syntax

A `where` clause on a `sig` or a `prim`, holding an ordinary **boolean term** over the parameter
names:

```lisp
(sig dot ((a vec-f64) (b vec-f64)) f64
  (where (int.eq (alen a) (alen b))))

(prim aindex ((v vec-f64) (i int)) f64 expr "%s[%s]" pure index
  (where (logic.and (int.le 0 i) (int.lt i (alen v)))))
```

On a `prim`, `where` is a **trailing attribute** beside `pure` and `import`, because that is where
the grammar already puts them ([target-files.md §1](target-files.md)). On a `sig` it follows the
result type. The named argument form `((v vec-f64) (i int))` is now accepted by `prim` as well as
`sig`, so the two have not diverged.

There is **no predicate syntax**. A refinement is a term of type `bool` — the predicate language
is a *fragment of the term language*, which is why parameters were named in
[types.md §7](types.md) before anything read them.

## 3. Classify, do not restrict

> **Any boolean term may appear in a `where`. A term inside the decided fragment is *proven*. A
> term outside it is an *opaque atom* — propagated and matched by name, never decided.**

Consequences, all of them wanted:

- Nothing is rejected for being too expressive **as a declaration**: any boolean term may be
  written in a `where`.
- It **degrades gracefully**, and the fragment can grow later without any program changing.

### 3a. An obligation is discharged, or the program is refused

*Amended 2026-09-25 ([noprop-2026-09-25](../../gauntlet/results/noprop-2026-09-25.md)).* This section
said an undecided obligation "can only be discharged by an assumption that matches it, **or by a
runtime check at a boundary**", and the compiler read that as licence to emit the program with a note:
*propagated, not proven*. No runtime check was ever emitted. An obligation is the **domain condition**
of an application: `0 ≤ i < len t` for `(t i)` (tables.md §6), a primitive's `where` for a call.
Outside its domain an application has no value, and each host then does something different:

| out of domain | Go | Java | JavaScript | x86-64 |
|---|---|---|---|---|
| `(t i)` | panics | throws | **`undefined`**, silently | **reads past the table** |

Measured on the witness below: an `int` function returned `undefined` on JavaScript.

A program denotes only if every application in it is defined. So an obligation is **discharged**, by
exactly one of these, or the program is **refused**, naming the obligation and what was known:

1. **a proof in the fragment** (§4), including a proof by cases (joinConditional) and a content fact
   about a table (array-facts.md);
2. **an assumption that is the same term**: an opaque atom matched by name, the one thing an atom
   outside the fragment can be matched against. A comparison is the same term when it is the same
   **relation** on operands printed alike: `u64<` is `<` realised in U, emitted only where both
   operands lie in U (ADR 0026), so a guard in one spelling discharges a `where` in the other;
3. **evaluation**, when the obligation is **closed**: a comparison between two literals. `(!= 3.0 0)`
   is true, and comparing two literals is exact on every host, so it is decided at compile time
   without folding any arithmetic (ADR 0009).

There is no fourth route. "Propagated" survives in one sense only, the one §6b already has: an
**exported** definition's own `where` is *assumed* inside it and is its caller's to discharge. The
caller is outside the program, and the obligation is stated in its interface, not dropped.

**`-checked` does not clear an index.** It asks for the host's checked arithmetic, which every target
declares. A checked index would need every target to trap, and JavaScript and x86 do not. Not built,
and named.

The witness, accepted before this amendment and refused now:

```lisp
(let b (build b 8 (loop ((b b) (i 0)) (>= i 8) b else (again (set b i (% (+ i n) 10)) (+ i 1))))
     t (build c 6 c)
  (t (b 3)))             ; b's cells reach 9, t has six
```

## 4. The fragment

**Linear integer arithmetic over difference constraints**, which is what every bounds obligation
actually is.

A term is *linear* if it is built from integer literals, names, `int.add`, `int.sub`,
multiplication by a literal, and **opaque length terms** like `(alen v)`, which are treated as
variables. Comparisons `int.lt`, `int.le`, `int.gt`, `int.ge`, `int.eq` over linear terms are
atoms; `logic.and` joins them.

Everything else — `f64` comparison, `ascii?`, a call to a defined function — is an **opaque atom**,
matched by syntactic identity and nothing more. `num/f64.eq` is opaque *by necessity*: IEEE
equality is not reflexive, so it is not an equivalence relation and a solver must not treat it as
one ([arithmetic.md §6](arithmetic.md)).

### Entailment

Facts are normalised to `e ≤ 0` where `e` is a linear expression. An obligation is discharged if,
after substituting known equalities, it is **implied by a single fact**: same variable part, and a
constant offset in the right direction.

Deliberately **incomplete**. It is not Fourier–Motzkin and it is not an SMT solver. It is the
smallest thing that decides the obligations this language actually generates, and being incomplete
is safe: an undischarged obligation is *refused* (§3a), never assumed.

## 5. Where facts come from

| | fact |
|---|---|
| `(fold-range z n f)` binding `i` | `0 ≤ i` and `i < n` |
| `(make-vec n f)` binding `i` | `0 ≤ i` and `i < n` |
| a `let` binding `x` to a linear term `e` | `x = e` |
| the enclosing `sig`'s own `where` | assumed |
| `alen`, `slen` | `0 ≤ alen v` |

That last one is free and worth having: a length is never negative on any target.

## 6. The diagnostic

An obligation that cannot be discharged is an **error**, naming the obligation and what was known:

```
smooth: (aindex a (int.add i 2)) requires i + 2 < alen a
  known: 0 <= i, i < alen a - 2
```

There is no softer case. A refinement *propagated* rather than proven used to be a note, and the
program was emitted (§3a). [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) had already
established that a transformation which silently does not fire is indistinguishable from one that
does, and a note beside an emitted program was exactly that silence.

## 6b. A `where` on a DEFINITION, and a declared parameter range

**[ADR 0028](../decisions/0028-a-definitions-contract-is-checked-at-its-calls.md): both are obligations
at every call the definition is inlined into.** This section recorded the opposite until
2026-09-24, and the argument it made is kept because half of it still holds.

**A primitive's `where` is discharged at every call site** (§5). A definition's has no call site left
by the time the residual reaches `Refine`, because reduction inlines every non-exported call. What
protects the program's **safety** is that the obligations *inside* the body land at the call site with
the caller's own values:

```lisp
(sig safe ((a (array f64)) (n int)) f64 (where (and (<= 0 n) (< n (len a)))))
(def safe (fn (a n) (a n)))
(def f (fn (a) (safe a (go.- 0 5))))
```

```
(a (go.- 0 5)) is an indexing, and (<= 0 (go.- 0 5)) does not follow
```

That half stands, and it is still how every obligation inside a body is checked. What it does not
protect is **meaning**: a body that is *total* and merely wrong outside its domain fires nothing.
`lib/win/fmt.oro`'s `print-int -13` printed a blank line, and `num/u128`'s `digits18` given a value
past 10¹⁸ printed the low 18 digits. A range on a parameter is a type (ADR 0003), and a type is
checked at application. So the declaration is now checked *as well as* the propagated obligations,
never instead of them.

### How it is checked

- **A range** is marked on the argument when the call is reduced, as `(#req "def" "param" "type" a)`,
  whose value is `a`. The mark reaches wherever the argument does: each use of the parameter, or the
  binding when β binds the argument to a name. A parameter the body never uses carries no
  obligation.
- **A `where`** is marked on the call's result at β, `(#reqw "def" cond body)`, with the arguments as
  β passes them substituted into it. Each reduction rule that inspects a result's shape hoists the mark
  out first.
- The marks are decided immediately after reduction and erased before anything else runs
  (`emit.DischargeRequires`), in order:
  1. a literal, decided when it reduces;
  2. the interval analysis;
  3. this layer, with every fact in scope, including a pure call's `ensures`.

  An obligation none of them proves is refused, naming the call.
- **Above the target's word a range denotes a set its enforcement can decide**
  ([ADR 0029](../decisions/0029-above-the-word-one-set-on-every-representation.md)): [0, 2ᵇ) when LO ≥ 0 and (−2ᵇ, 2ᵇ) when LO < 0, b the
  bit length of max(|LO|, |HI|): the least set containing the declaration that a sign and a bit
  length decide. A declared result above the word, carried as `(the T e)`, tells only that its value
  is in the ONE set the program enforces (the join of all its types' sets, since the bound is one per
  program), so it proves a parameter's range only when that range is at least as wide.

### One syntax, two meanings

| on | meaning |
|---|---|
| a `prim` | an obligation, discharged at every call site |
| a definition, called from inside the program | an obligation at every call it is inlined into |
| an **exported** definition, called from outside | a published contract, assumed: the caller is outside the program, exactly what SAL is for a C header. `Refine` assumes it so the body may rely on it |

**Not covered, and named** (ADR 0028): a definition passed as a value and applied later, which is not
a direct call by name. A range with an infinite endpoint keeps its finite side, `(int 0 +inf)` as
`0 <= n`, on the call's condition.

## 7. What this does not do

- **No solver for non-linear arithmetic.** Undecidable over the integers (Hilbert's tenth), and no
  obligation needs it.
- **No quantifiers.** `∀i. a[i] > 0` is a whole-array property; the fragment is per-index.
- **No proofs.** Layer 2 ([types-sketch §7](../types-sketch.md)).
- **No refinement inference.** A `where` is declared, never guessed. Liquid Types infers them by
  Horn-clause fixpoint; that is a larger machine and no program has asked.
- **No enforcement of a precondition that states MEANING.** §6b: a definition whose body is total
  and merely wrong outside its domain has nothing to catch. `win/fmt.print-int` is the instance.
- **Not applied to the integer range yet.** §1's second hole needs `int` literals to carry ranges,
  which touches every arithmetic primitive. One hole at a time.
