# Precision by declaration

Research, 2026-08-28. **No decision, no specification.**

hamza's proposal, as a third option beside the two
[precision-integers.md](precision-integers.md) and
[checked-2026-08-28](../gauntlet/results/checked-2026-08-28.md) leave open:

> **Bounded by default, but when a range is declared above the bound, the compiler moves to
> precision. Is that possible, and feasible?**

Short answers: **possible, and it is the natural completion of ADR 0003 rather than an addition to
it.** **Feasible, and cheaper than it looks** — the blocker everyone would expect is not the blocker.
But it does **not** buy what it might appear to buy, and §7 is the part to read before liking it.

Naming, for the rest of this document: **A** is *unbounded by default, boxed where unproven*; **B**
is *bounded by default, unprovable is an error, precision is a separate type*; **C** is the proposal.

---

## 1. The proposal, stated precisely

A range is already a type, and a target already declares which representations it has:

```lisp
(int-repr -128 127        "byte")     ; narrowest first
(int-repr -32768 32767    "short")
(int-repr -2147483648 2147483647 "int")
```

`reprFor(lo, hi)` picks the **narrowest declared representation containing `[lo,hi]`**. That is
built, measured, and shipping ([elemwidth-2026-08-27](../gauntlet/results/elemwidth-2026-08-27.md)):
`(array (int 0 255))` is `[]byte` on Go and `short[]` on Java, because the JVM's `byte` is signed.

**C is one more rung at the top of that ladder.**

| declared range | representation |
|---|---|
| `(int 0 255)` | a byte |
| `(int 0 65535)` | two bytes |
| *(bare `int`)* | the host's word — ADR 0012's window |
| **above the window** | **the host's bignum** |

Nothing new is invented. The rule is the one already in force — *the target declares what it can
hold, and the range picks* — extended past the machine word instead of stopping there.

> **The ladder is symmetric, and today only the bottom half exists.** ADR 0003 says *mathematical
> semantics, machine representation*. Below the word that is built. Above it, the language currently
> says nothing, and ADR 0012's window is where the ladder was cut off.

## 2. What C is, operationally

- **Default is B.** An integer operation the compiler cannot bound is a **compile error**. Nothing
  wraps, nothing traps, nothing boxes, silently.
- **The error is cleared by saying something.** Three ways, all written in the source:
  1. **narrow** — declare a tighter range, so the operation becomes provable;
  2. **widen past the window** — the value becomes arbitrary precision, and pays for it;
  3. **ask for the trap** — keep the word, panic on overflow.
- **The cost is named at the site the programmer chose it**, which is the move this project keeps
  making: `alloc` names where the allocation is, the explicit stack in `tokenize.oro` makes the depth
  limit visible, `sig` makes the contract visible.

## 3. Is it possible?

**Yes, and most of the mechanism exists.** What is missing is small and specific.

| | state |
|---|---|
| a range is a type | **built** — `core.TypeName` accepts `(int LO HI)` |
| a target declares its representations | **built** — `int-repr`, narrowest-first |
| a range selects one | **built** — `reprFor`, `NarrowedElem`, `reprBytes` |
| an element's width follows its range | **built** — Go, Java, x86 |
| a value's *own* representation follows its range | **not built** — and see §3.1: a scalar range is not usable at all today |
| a spelling for "unbounded" | **not built** — `(int LO HI)` cannot say ℤ |
| bignum operations declared per target | **not built** — R3 |
| lifting a word into a bignum at a mixed site | **not built** |

Two of those need comment.

**"A range never narrows a local" is a deliberate rule, and C changes it.**
[elemwidth](../gauntlet/results/elemwidth-2026-08-27.md) records it: *a range says what a value IS;
the width belongs to its storage alone* — `ValueType` normalises a range to `int` everywhere except
a table's element slot, because otherwise a counter over a byte array would overflow at 255. C
reverses that only at the **top** of the ladder, and the asymmetry is principled: narrowing a local
below the word is unsound without a proof about the local's own arithmetic, whereas widening it
above the word is unconditionally sound. **Going up the ladder can never lose a value; going down
can.**

**`(int LO HI)` cannot say ℤ.** A range has two finite endpoints. Unboundedness is not a range, so C
needs a spelling for the top rung — `(int *)`, `integer`, `exact`, something. That is a surface
question, and it is the one place C is not purely "the existing mechanism, one rung further".

### 3.1 A scalar range is not usable today, and that is C's real prerequisite

Checked rather than assumed:

```lisp
(sig f ((n (int 0 1000))) int)
(def f (fn (n) (go.+ n 1)))
```

```
gen: f: in argument 1 of go.+: n is int 0 1000, but int is required here
```

**The range language works on array ELEMENTS and nowhere else.** Two independent reasons, both
one-liners:

- `core.ValueType` — which turns `int 0 255` into `int`, *"a range says what a value IS; the width
  belongs to its storage alone"* — is called at **exactly one site**, the table-read path in
  `emit/golang.go`. The type checker never applies it, so a declared scalar range is compared as the
  string `"int 0 1000"` against `"int"` and disagrees.
- `paramIval` matches `sp.Type == "int"`, so even if the checker allowed it, the interval analysis
  would **ignore** a parameter's declared range.

So today a scalar's range is stated with a **`where`**, not with a range type — which is what both
parsers do, and what took them to 100%. That is a prerequisite for C rather than an objection to it,
and it is small; but it is not zero, and C cannot be described as "the mechanism already exists" until
it is done.

### 3.2 Two possible surfaces, and `where` already selects representations

- **C-type** — `(sig f ((n (int 0 HUGE))) …)`. Needs §3.1 first. Uniform with the element case, and a
  range is a claim about the value, which is what ADR 0003 wants stated.
- **C-where** — `(sig f ((n int)) int (where (<= n HUGE)))`. Works with today's surface, and the
  representation is read off the parameter's *inferred* interval at the boundary.

C-where looks like a category error — a `where` is a precondition, not a type — except that **it is
already a representation lever, one step removed**: `(where (go.< (len src) 1024))` on `measure` is
what makes `tree.oro`'s node table sixteen bits wide
([rebench-2026-08-27](../gauntlet/results/rebench-2026-08-27.md)). The precedent exists, and it was
not designed; it fell out.

The fork is worth deciding deliberately rather than by whichever is easier to build.

## 4. The feasibility crux, and it is not what it looks like

The obvious objection is that the analysis cannot reason about huge numbers. It is true, and it is
worse than it looks:

```go
const sat = int64(1) << 62          // emit/interval.go
type ival struct{ lo, hi int64; loInf, hiInf bool }
type IntRepr struct{ Lo, Hi int64; Spell string }
func IntRange(ty string) (int64, int64, bool)      // parses "int %d %d"
```

**`int64` is hardcoded as the width of the range language, the abstract domain, and the
representation table**, and arithmetic saturates to ⊤ above 2⁶². So today the domain's ⊤ means two
different things at once — *"larger than 2⁶²"* and *"we do not know"* — and it has no resolution
above the saturation point at all.

The expected conclusion is that the interval domain must become arbitrary-precision. **That
conclusion is wrong, and this is the document's main finding.**

> **C needs a representation lattice, not a wider numeral.** An operation on a value that is *already*
> arbitrary precision cannot overflow, so it needs no bound — the analysis is never asked. The
> analysis is consulted only about **word-represented** values, and about those, `int64` with
> saturation is exactly the right width.

So the addition is a two-point lattice per binding:

```
    word  ⊑  big
```

with `big` **absorbing**: any operation with a `big` operand is a `big` operation, and the word
operands are lifted. Lifting is total, so it cannot fail.

**The soundness rule falls out in one line:**

> An integer operation is an error **only if every operand is word-represented and its result cannot
> be proven to fit the window.** If any operand is `big`, the operation is exact and there is nothing
> to prove.

`emit/interval.go` is untouched by this. That is the difference between C and A: A needs the abstract
domain to reason *about* arbitrary precision; C needs it only to decide *whether* arbitrary precision
is required, which is a question about the word case.

### 4.1 What the lattice costs: propagation

The representation of every integer binding must be solved, not read off locally. Constraints:

- a declared parameter or result **fixes** its binding;
- an operation's result is `big` if any operand is;
- a table's elements **share** a representation (equality constraint — tables are homogeneous, and
  tables.md's dynamic index forces it);
- a call **unifies** argument with parameter and result with result;
- a `loop` variable unifies its initialiser, every `again` argument, and its uses.

That is a unification problem over a two-point lattice, which is the same shape
[q5c-representation-choice.md](spec/q5c-representation-choice.md) already solved for pull/push
collections, resolved before reduction. So there is precedent for both the machinery and its placement.

**And it is bidirectional, which is the sharp design question.** Forward propagation alone is not
enough, and factorial is the witness:

```lisp
(sig fact ((n (int 0 30))) ???)
(def fact (fn (n) (loop ((acc 1) (i 1)) (> i n) acc else (again (* acc i) (+ i 1)))))
```

Every *input* is small. The accumulator reaches 30! ≈ 2.65 × 10³², which is not. So `acc` must be
`big`, and nothing forward makes it so — the pressure comes from the declared **result**. Either the
solver runs in both directions, or a local needs an annotation, and neither is free. **Naming this
now matters, because a forward-only implementation would look correct on every program in the
current corpus and fail on the first factorial.**

## 5. What it costs to build

1. **A spelling for the top rung**, and a decision about whether finite-but-huge ranges
   (`(int 0 10^30)`) are expressible at all.

   > **Recommendation WITHDRAWN 2026-08-28 by measurement** —
   > [bigarith-2026-08-28](../gauntlet/results/bigarith-2026-08-28.md) §3a. This said *"they are not,
   > at first"*, reasoning that a 128-bit rung exists on only two of four hosts and so could carry no
   > portability claim. That is still right about the **middle** of the ladder and wrong about the
   > **top**: a finite wide range gives a limb count, which gives a `build` of known length, which is
   > what buys **zero allocations and 3.97× over `math/big`**. An *unbounded* declaration gives none
   > of that and lands back on allocate-per-operation. **Bounded-but-huge and unbounded are two
   > different rungs**, worth a factor of four, and the language should be able to say which it has.
2. **The representation solver** of §4.1, bidirectional, over `word ⊑ big`.
3. **R3 declarations per target** — `math/big`, `BigInteger`, `BigInt`, and **one written for
   windows**. Unavoidable in *any* option that has precision at all, including A and B-with-a-bignum-type.
4. **Lifting at mixed sites**, and it is mandatory rather than an optimisation on JavaScript, where
   `1n + 1` is a `TypeError`.
5. **A narrowing conversion**, `big → word`, which is fallible and is therefore
   [a sum](spec/sums.md) — already built, already measured at 1.00× with 0 allocations when consumed
   locally.

Items 3–5 are owed by every option. **C's own cost is items 1 and 2.**

## 6. What it buys over B

- **One mechanism instead of two.** B introduces a `bignum` type beside the range language; C extends
  the range language. A range is a claim about the **value**; a type name is a claim about the
  **representation**. ADR 0003's thesis is that you state the former and the compiler picks the
  latter — C keeps that, and B inverts it at exactly one point.
- **The declaration is checked, not merely obeyed.** `(int 0 10^30)` is a specification the compiler
  can hold the body to. `bignum` says only "use the slow one".
- **It suits JavaScript best, which is the host that makes A hardest.** `BigInt` and `Number` do not
  mix, so a *dynamic* promotion is not implementable there. C's choice is static and per-binding, so
  the two representations never meet at run time — the constraint that kills A on that host is
  automatically satisfied by C.
- **The ladder is uniform.** Sub-word narrowing and above-word widening become one rule with one
  declaration form, read in both directions.

## 7. What it does NOT buy, and this is the part to weigh

**C does not reduce B's annotation burden. It is B, with a third answer available at the sites where
B already demanded one.**

Measured today, B would refuse six real programs in the corpus. C refuses exactly the same six. What
changes is what the programmer may write in response — not how often they must write something.

So if the objection to B is *"a general-purpose application will need declarations everywhere and
that is unusable"*, **C does not answer it.** Only A answers that, and A pays with
[silent-slow failure and a whole-program boxing story](precision-integers.md) that this repository's
diagnostic culture has rejected at every previous opportunity.

Two more honest costs:

- **A signature's representation is ABI.** An exported function taking an unbounded `int` has a
  bignum in its interface, and changing that later is a breaking change. That is true of any option
  with precision.
- **Performance is a cliff, not a slope.** Crossing the top rung costs 10–50× on V8's `BigInt` and
  allocates per operation on Go's `math/big` and Java's `BigInteger`. C makes the cliff **visible in
  the source**, which is the best that can be done, but it does not make it small.

## 8. Where C sits among the alternatives

| | default | unprovable operation | precision reached by | failure mode |
|---|---|---|---|---|
| **today** | window | silently wraps / loses precision | — | silent wrong answer |
| **A** | ℤ | boxed automatically | always available | **silently slow** |
| **B** | window | compile error | a separate `bignum` type | loud, at compile time |
| **C** | window | compile error | **widening the declared range** | loud, at compile time |
| Rust | fixed | panics in debug, wraps in release | `BigInt` crate | mode-dependent |
| Swift | fixed | **traps** (`&+` wraps) | explicit | loud, at run time |
| **Zig** | fixed | **traps** (`+%` wraps) | `comptime_int` — **compile time only** | loud, at run time |
| Haskell | `Int` wraps | — | `Integer`, programmer chooses | silent (this is B) |
| SBCL | tower | auto-promotes | automatic | slow, **plus an optimization note** |
| Python | ℤ | boxed | always | slow, accepted |

Two patterns, both already noted and both reinforced by C:

**Everyone who chose A is dynamically typed and accepted a large constant factor.** No language
targeting C-parity has chosen it. **SBCL, the most serious attempt at A with representation
selection, bolted a B-style diagnostic on top** — because silent-slow was not liveable.

**And Zig is this project's structural twin** — two-level, systems target — and it put arbitrary
precision at *compile time only*. Under C that is not a rival design but a *consequence*: at the
static level reduction can fold at arbitrary precision (and
[precision-integers.md](precision-integers.md) notes the ADR 0009 hazard disappears once integers are
exact), while the dynamic level stays on the ladder. **C is Zig's answer with the top rung also
reachable at run time, by writing it down.**

## 9. Open questions

1. **The spelling** of the top rung, and whether finite-but-huge ranges are expressible. §5
   recommends not, at first.
2. **Bidirectional solving**, and the factorial witness of §4.1. A forward-only implementation would
   pass the entire current corpus and be wrong.
3. **Does a local ever need an annotation**, or does unification from the signature always suffice?
   Reduction inlines every non-exported call, so most locals get their pressure from a boundary — but
   `fact`'s `acc` shows the pressure can come from the *result*, and a `build` buffer's elements are
   the case where it may come from neither.
4. **§3.1's gap: is a scalar range a missing feature or a deliberate restriction?**
   [elemwidth](../gauntlet/results/elemwidth-2026-08-27.md) states the intent — *`ValueType`
   normalises a range to `int` everywhere but a table's element slot* — and the implementation
   normalises in one slot and never looks elsewhere. Closing it is small and is worth doing
   regardless of which option wins, because it is the difference between a range being a type and a
   range being an array annotation.
5. **windows' bignum**, which should be written in Oroboros for the reason
   [growth.md §5](growth.md) gives about the map: it is the test of whether the language can write
   its own libraries.
6. **The measurement that decides between B/C and A** is not about integers at all. It is: *how many
   declarations does a real application need?* Today's 30-of-39 is a corpus of numeric kernels and
   two parsers, written by people who knew the analysis. That number for an app with timestamps, IDs,
   money and string offsets is unknown, and it is the only thing that can kill B and C.
