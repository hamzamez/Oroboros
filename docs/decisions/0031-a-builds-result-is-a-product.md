# 0031 — A `build`'s result is a product, and each buffer in it is frozen

Date: 2026-09-25
Status: Accepted — **not built**. **Completes [ADR 0018](0018-immutable-values-linear-buffers.md)**,
whose `build` returns one table. **States the tuple-component law**, which
[assessment-2026-09-24](../assessment-2026-09-24.md) said would become an ADR on its fourth meeting;
this is that meeting. The surface is [tables.md §2.4](../spec/tables.md), and the escape argument
this corrects is §14.1. Measured in [buildbind-2026-09-25](../../gauntlet/results/buildbind-2026-09-25.md).

## Context

hamza asked what a `build` returns. The specification and the compiler give different answers.

**The specification** (tables.md §14.1, ADR 0018) types it

```
build : ℕ → (Buf V ⊸ Buf V) → Arr V
```

one buffer in, the same buffer out, frozen. §14.1 rested half of its escape argument on that type: a
body whose value is a rule-table capturing the buffer would be a type error.

**The compiler** (probed 2026-09-25) returns whatever the body returns, and no check enforces the
type:

| the body returns | outcome |
|---|---|
| its buffer | works: the frozen table |
| an enclosing scope's buffer (msort's inner `build` returns `a`) | works; the walk treats it as a move |
| an `int`, `(build b 8 (let b (set b 0 7) (b 0)))` | **accepted on all four targets**, and sound: the buffer is dropped |
| a rule-table capturing the buffer | refused, but by *a rule-table has no memory*, not by a type |
| `(tuple b i)`, taken apart by a tuple pattern | **refused on all four targets, with an internal error**: "application of a non-name" |

The last row is not specific to `build`. **A `loop` whose exit value is a tuple cannot be taken apart
by a tuple pattern either**: the same probe with no buffer fails the same way on every target. A tuple
that a *host call* returns can be taken apart, because ADR 0027 made its continuation a tail, and a
tuple that β builds and consumes in place can too. A tuple that comes out of a loop or a scope cannot.

**Programs pay for it.** The parser in `examples/json/tree.oro`, and its twin in
`gauntlet/differential/cases/json-tree.oro`, must return the node table, the node count and an ok
flag. It writes the count and the flag **into slots 0 and 1 of the node table**:
`(set (set nodes 0 nn) 1 ok)`. That is a product hand-encoded into an array, which is what
[products.md](../spec/products.md) exists to make unnecessary. Its readers then index slot 0 for a
count and must prove the slot holds one. And "build to a bound, return the count, slice", named in
tables.md §14.3 as the answer to unbounded accumulation, cannot be written, because the count has no
way out.

**And the length is lost.** `build` records `len b = n` (ADR 0018's first correction). When the buffer
leaves through a tuple component, the binder the pattern names knows nothing: in the probe, `(t 4)`
was refused with *known: nothing*. That is the tuple-component law again:
- **mathbits-2026-09-23:** a host call's component range was not a fact of its binder (built, for host
  calls only);
- **u128-2026-09-23:** Div64's quotient bound, a component fact, would prove `decimal`'s loop
  terminates (not built);
- the assessment counted **three** meetings;
- **this is the fourth:** a `build`'s length through a component.

## Decision

### 1. The result of a scope is a product of its frozen buffers and buffer-free values

For a scope binding `b₁ : Buf_{n₁} V₁, …, bₖ : Buf_{nₖ} Vₖ` (tables.md §2.4, so k ≥ 1):

```
build : (n̄ : ℕᵏ) → (Buf_{n₁} V₁ ⊗ … ⊗ Buf_{nₖ} Vₖ  ⊸  F(Buf̄))  →  F(Arr̄)
```

where **F is a finite product**, a tuple of arity m ≥ 1. Each component is **either one of the
scope's buffers `bᵢ`**, at most once, **or a value of a buffer-free type**. The freeze is `F(φ)`: the
family `φᵢ : Buf_{nᵢ} Vᵢ → Arr_{nᵢ} Vᵢ`, applied in the buffer positions and the identity elsewhere.
Each `φᵢ` is the identity on representation (ADR 0018, consequence 4).

- F = the identity functor is today's one-table `build`.
- F constant, so no buffer is returned, is today's accepted `int` case, now specified.
- `(tuple nodes nn ok)` is the parser's result, frozen in its first position.

This is `createT :: Traversable f => (∀s. ST s (f (MVector s a))) → ST s (f (Vector a))` from Haskell's
`vector` package, with *Traversable* restricted to products. It is Linear Haskell's
`alloc :: Int → a → (Array a ⊸ Ur b) ⊸ Ur b` (Bernardy et al., POPL 2018), with `freeze` implicit at
the scope's end. It is Futhark's in-place loops returning several arrays (Henriksen et al., PLDI 2017).
All three have the same property: the result mentions no live buffer.

### 2. The theorem that makes the freeze free, and the three refusals it needs

**Theorem (no-copy freeze).** Suppose that at the scope's exit every buffer component of the result is
the only live reference to its storage, and no buffer-free component reaches a buffer. Then `F(φ)`
copies nothing, and no store can ever be observed through the frozen result.

*Proof.* Linearity (ADR 0018, the walk on the residual) makes the occurrence of `bᵢ` in the result its
last use. The scope's end kills every name it bound. So after the exit, only the result can reach the
storage of any `bᵢ`. A frozen component is read purely, and no name that could store into it exists,
so a read observes the last store before the exit, whenever it runs. That is what makes treating the
read as pure (ADR 0010) sound. ∎

The hypothesis is decidable on the residual, and it is exactly three refusals:

- **(R1) A buffer only at top level.** A buffer inside an array's element is already refused
  (ADR 0020, rule 6). This adds a buffer inside a variant's payload and a rule-table whose rule mentions
  a scope buffer. The latter is §14.1's case, refused today by accident and after this by a rule that
  names it. A closure over a buffer is already refused as a value.
- **(R2) Each buffer at most once.** `(tuple b b)` would be two references. Linearity refuses it
  already, since forming the tuple moves `b`, and a test pins it.
- **(R3) An enclosing buffer returned through an inner scope is a move.** This holds today, and the
  probe pins it: a later read or store through the old name is refused.

### 3. The tuple-component law

> **For `(let (tuple x₁ … xₘ) e body)`, every fact the analyses hold of the i-th component of `e`
> holds of `xᵢ` in `body`.**

*Why it is sound.* When `e` reduces to a tuple `(fn (#k) (#k a₁ … aₘ))`, the pattern's eliminator is β
and `xᵢ := aᵢ`. A fact about `aᵢ` is a fact about `xᵢ` by substitution, which is the eliminator's
definition (data.md §3.2: currying is the product's universal property). When `e` does not reduce (a
host call, a scope, a loop), `xᵢ` is the parameter of a continuation that runs **once, now**, applied to
`e`'s i-th result. That is ADR 0027's property of a host call's continuation, and §4 gives it to scopes
and loops. So `xᵢ` denotes that result, and the facts the producer states of its component hold of
`xᵢ`:

| producer | the component facts it states |
|---|---|
| a host call | its declared result ranges and `ensures` (built in mathbits-2026-09-23) |
| a scope | `len xᵢ = nⱼ` when component i is buffer `bⱼ`; the loop's facts for the rest |
| a loop | each exit's facts about the value in position i, joined over the exits |

**The law is stated for every producer and built for each one when a program needs it.** The scope's
length comes first, because the parser needs it. The loop's joined facts come next; Div64's quotient
for `decimal` is one of them. This follows the assessment's rule that the analysis grows only for a
refusal a program names.

### 4. The lowering: the eliminator commutes into a scope and out of a loop

The internal error in the Context is a missing **commuting conversion**. A product eliminator `K`
applied to a term that is not a tuple must be moved to where the tuple is made.

- **Into a scope:**
  `((build n (fn (b) M)) K)  ⟶  (build n (fn (b) (M K)))`.
  This is valid because the scope's body runs once, now, and `K` is closed with respect to `b` (α).
  After the conversion, the buffers `M` passes to `K` have been moved. So `K` holds them frozen: typed
  `(array V)`, read purely. That is the Theorem's conclusion, reached without copying.
- **Out of a loop, through a join point:**
  `((loop F z̄) K)` runs `K` once, on the value of whichever exit is taken. That is case-of-case,
  Girard's commuting conversion for ⊕-elimination, applied to a loop's exits. Copying `K` into every
  exit would duplicate code. So `K` becomes a **join point**: the exits assign the components to
  result variables and jump, and `K` runs once after the loop (Maurer, Downen, Ariola & Peyton Jones,
  *Compiling without continuations*, PLDI 2017). Every backend already has the pieces:
  - Go and JavaScript: locals assigned before a `break`;
  - Java: locals, or a record where ADR `values`' precedent chose one;
  - windows: registers or stack slots.

Nothing new reaches a backend's input except a loop or scope with m > 1 results. The product pass
(products.md) already flattens a product's components by currying.

### 5. What the surface is

Unchanged from tables.md §2.4: `(build b n  c m  body)`. The body's value is the result, and a tuple
body is a tuple result:

```lisp
(let (tuple nodes nn ok)
     (build nodes (* 4 nmax)
            stk   (* 2 dmax)
       (loop ((nodes nodes) (stk stk) (i 0) (nn 1) (sp 0) (ok 1))
         (>= i (len src))  (tuple nodes nn (if (= sp 0) ok 0))
         …))
  …)
```

No new keyword and no `freeze` term. The freeze stays where it always was, at the end of the scope.

## Why not

**(a) Enforce the type the specification wrote, `Buf V ⊸ Buf V`.** Refuse every non-buffer result.
*Rejected.* It would refuse the sound `int` case, which is accepted today on every target. It would
leave the parser's product encoded in array slots. And "build to a bound and return the count" would
stay inexpressible. The type was a description of the first program written with `build`, never a
derivation. Nothing about linearity needs the result to be exactly the buffer: the Theorem needs only
that no live buffer escapes.

**(b) Linear Haskell's explicit `freeze : Buf V ⊸ Ur (Arr V)`, with a scope that returns any
unrestricted value.** More general: a buffer could be frozen mid-scope and read purely while another
is still written. *Rejected for now.* It adds a term, and it moves the escape check from the scope's end
to every `Ur` boundary. The only program that would use a mid-scope freeze is none; the parser freezes
at the end. The implicit freeze already has its argument (ADR 0018) and a check (the walk). If a
program needs to freeze early, this reopens.

**(c) Keep encoding products in array slots.** `tree.oro`'s way. *Rejected:* it is a product written
by hand into the wrong type. The count's slot must hold an element of the node table's element type, so
a count wider than a node index cannot be stored. And a slot's content is a fact the refinement layer
must prove on every read (the frozen-read stratum). A tuple component's range is a declaration, not a
proof obligation.

**(d) Return the scalars through a second `build` of length one.** It works today and costs an
allocation per scalar, which is the hidden allocation the language exists not to have
(design-direction.md §2).

**(e) Only the scope, not the loop.** *Rejected by the probe.* A scope's body that computes a count is
a loop, so the loop's tuple exit is the same wall one layer in. Converting into the scope and then
stopping at the loop would move the internal error, not remove it. ADR 0027 was one layer of this same
wall (a host call's continuation). This is the other two layers.

## Consequences

**Built when it is built:**
- the commuting conversions (§4) in the reducer, or at the product pass;
- (R1) as a refusal with its own text, and a pin for each of (R2) and (R3);
- the scope's component length (§3);
- a join point per backend;
- differential cases on all four targets:
  - a scope returning `(tuple b k)`;
  - a loop returning a tuple taken apart by a pattern;
  - two buffers from one scope;
  - each refusal.

**The first program is the parser.** `tree.oro` and `json-tree.oro` return `(tuple nodes nn ok)`, and
slots 0 and 1 go back to being nodes. Emission changes there, and only there, and is accepted as
correct and no slower against `TreeGen`.

**The escape argument has one mechanism again.** §14.1 names two halves. After this, the first is a
rule of this ADR, and no longer an accident of the rule-table refusal.

**What this does not decide.**
- A buffer inside a variant is refused (R1), not frozen through a `case`. A sum of buffers would need
  the freeze to commute with the eliminator of ⊕, and no program has one.
- A result arity above the host's native several-results is a record on Java and a stack block on
  windows, the `values` precedent. How wide is cheap is measured when a program is that wide.

**What reopens this.**
1. A program that must read a buffer purely **while** another is still written, so needs a freeze
   before the scope ends. That is (b).
2. A result the join point makes slower than the hand-written equivalent on any target, measured.
3. A component fact that holds of the producer but not of the binder. That would refute §3's
   soundness argument, and it is the witness every rule built from §3 must be tested against.
