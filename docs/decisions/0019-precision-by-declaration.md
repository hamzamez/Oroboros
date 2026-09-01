# 0019 — Precision by declaration

Date: 2026-08-28
Status: Accepted — **provisionally**, with the reopening triggers in "What should reopen this"

Research: [precision-by-declaration.md](../precision-by-declaration.md),
[precision-integers.md](../precision-integers.md).
Measurements: [bigarith-2026-08-28](../../gauntlet/results/bigarith-2026-08-28.md),
[checked-2026-08-28](../../gauntlet/results/checked-2026-08-28.md).

## Context

[ADR 0003](0003-range-typed-integers.md) said *mathematical semantics, machine representation*, and
[ADR 0012](0012-portable-integer-range.md) fixed the portable window at ±(2⁵³−1). Below the machine
word that ladder is built and measured — `(array (int 0 255))` is `[]byte` on Go and `short[]` on
Java. **Above the word the language says nothing**, and what happens today when a value leaves the
window is that it silently wraps on Go, Java and x86 and silently loses precision on JavaScript.
Four hosts, four behaviours, in the middle of a Tier 1 construct.

hamza asked for precision integers, and named the bar: *"they should not interfere with getting the
best performance when the range is within the supported integers; beyond that they cost what they
would cost if we implemented them on the target."*

Three options were researched, and the third is hamza's:

- **A** — `int` is ℤ; the compiler boxes where it cannot prove a machine word suffices.
- **B** — bounded by default; an unprovable operation is a compile error; precision is a separate
  `bignum` type.
- **C** — bounded by default; an unprovable operation is a compile error; **a range declared above
  the window moves that value to arbitrary precision.**

## Decision

**C.** Precisely:

1. **`int` keeps ADR 0012's meaning.** This ADR does not supersede 0012 and does not change what a
   plain `int` is.
2. **An integer operation the compiler cannot prove stays inside the window is a compile error.**
   Not a wrap, not a silent trap, not a silent box.
3. **The error is cleared by saying something, and there are three things to say**: declare a
   narrower range so the operation becomes provable; declare a range **above** the window, which
   moves the value to arbitrary precision and pays for it; or ask for the trap explicitly.
4. **The representation ladder extends upward using the mechanism it already uses downward.** A
   target declares what it can hold and the declared range picks, exactly as `int-repr` does below
   the word.
5. **A value's representation is decided statically, per binding**, over the lattice `word ⊑ big`
   with `big` absorbing. The soundness rule is one line: *an operation is an error only if every
   operand is word-represented and its result cannot be proven to fit.*

### What makes this affordable, measured rather than assumed

**Turning the checks on costs almost nothing now.** `-checked` emits a **byte-identical file on 30 of
39 programs**, and **1.05×** — inside the noise floor — on the one program that still pays
([checked-2026-08-28](../../gauntlet/results/checked-2026-08-28.md)). The proof rate carries this
decision; it was 39% when [intervals-2026-08-19](../../gauntlet/results/intervals-2026-08-19.md) first
asked, and the analysis work of the past week is what moved it.

**And a declared range is what makes the bignum fast**, which is the argument for C over B and did
not exist when C was proposed. A finite range gives a limb count, which gives a `build` of known
length, which gives **zero allocations**: measured **3.97× over `math/big`** on Go, **6.2× over
`BigInteger`** on Java, **5.8× over `BigInt`** on V8, at the sizes a value that has just left the
window actually reaches. An **unbounded** declaration gives none of that and lands on
allocate-per-operation, which is naive `math/big` and 4–5× worse.

> **B's surface is a type name and carries no size; C's surface is a range and carries exactly the
> information the fast path needs.** That is a measured difference of about four, not a matter of
> taste.

## Why not

**(a) A — `int` is ℤ, boxed where unproven.** *Rejected*, on four grounds, in order of weight.

- **The failure mode is silent-slow.** A program that missed a proof is correct and slower, with no
  diagnostic; you find it with a profiler. This project has chosen loud every time —
  [bce-2026-08-15](../../gauntlet/results/bce-2026-08-15.md) required a diagnostic for an
  optimisation that silently does not fire, and ADR 0018 rejected SISAL's reuse analysis for giving
  *no source-level guarantee*. Note that **SBCL, the most serious attempt at A with representation
  selection, added compile-time optimization notes** — it hit this and could not live with it.
- **The tagged representation is disqualified by the parasite model.** Every language that does A
  well tags a fixnum in the low bits. That costs a tag test per operation, bits of range, and —
  decisively — **an integer that is not the host's integer**. `go.+` would stop emitting `a + b` on a
  Go `int`. That is lowering below what the host natively provides, which CLAUDE.md names as the
  single most common way to get the architecture wrong, and it breaks host interop: a Win32 call
  taking a `SIZE_T` cannot take a tagged word.
- **The untagged form makes representation a whole-program property that infects data.** An
  `(array int)` with any possibly-big element is an array of pointers, which destroys
  [elemwidth](../../gauntlet/results/elemwidth-2026-08-27.md) and
  [json-tree-bench](../../gauntlet/results/json-tree-bench-2026-08-26.md)'s flat-table result at once.
  C has the same constraint problem but its blast radius is **bounded and visible**: every source of
  `big` is a declaration somebody wrote.
- **Boxing in the core is the predecessor project's cause of death**, and A's boxing is implicit.

**(b) B — precision as a separate `bignum` type.** *Rejected, and it is the closest call.* B is
simpler and has excellent company: Haskell's `Int`/`Integer`, Java's `int`/`BigInteger`, Julia's.
What rules it out is measured: **a type name carries no size**, so it selects the allocate-per-
operation implementation and gives up a factor of four. C gets that back for free because a range
*is* a size. B also introduces a second concept beside the range language where C extends the one
already there, and it inverts ADR 0003's thesis — a range is a claim about the **value** and the
compiler picks the representation; a type name is a claim about the representation.

**(c) Trap by default, as Swift and Zig do.** *Rejected as the default, kept as one of the three
things a programmer may say.* Swift's `+` traps with `&+` to wrap, Zig's traps in safe modes with
`+%`; both are shipping, performance-sensitive, and made this exact call. It is a good default and
we decline it for one reason: **a trap is a run-time failure where we can afford a compile-time
one.** Our proof rate makes refusal affordable — 30 of 39 programs unaffected — and a refusal names
the operation while a trap names a stack frame. Where the proof fails and the programmer wants the
trap, they say so.

**(d) Silent wrapping, as Go, Java, C and Julia do.** *Rejected.* It is the status quo, and the four
hosts do not agree on it: three wrap and JavaScript silently loses precision past 2⁵³. That is a
cross-target divergence sitting inside a construct the language claims is portable, which is what
[ADR 0001](0001-parasite-model.md) exists to refuse.

**(e) Arbitrary precision at compile time only**, which is Zig's `comptime_int` and, as of today,
Go's untyped constants. *Not rejected — subsumed.* Under C the static level can fold at arbitrary
precision, and [ADR 0009](0009-staging-preserves-results.md)'s hazard disappears once integers are
exact, because folding at arbitrary precision *is* the runtime semantics. C is this plus a top rung
that is also reachable at run time, by writing it down.

**(f) Turning `-checked` on by default and calling it precision.** *Rejected, and it is a trap worth
naming because it is one line of code away.* `-checked` is **detection, not precision**: it panics
where exactness would promote. Wiring it into the default path would reverse ADR 0012 without an
ADR, which
[assessment-2026-08-20 §2](../assessment-2026-08-20.md) already recorded happening once — *a
demonstration wired into the default path is a decision, whether or not anyone made one.*

## Consequences

**What this commits us to building**, in order:

1. ~~**A scalar range must become usable.**~~ **DONE 2026-08-31** —
   [scalarrange-2026-08-31](../../gauntlet/results/scalarrange-2026-08-31.md). It was refused,
   because `core.ValueType` was called at exactly one site: the range language worked on array
   **elements** and nowhere else.

   A range turns out to have **three** effects and the build is about keeping them apart. It is an
   `int` **for typing**, normalised at the single point two types are compared; it is a **premise**,
   desugaring in the reader into the `where` it means, so no analysis learns a new thing exists; and
   it is a **representation** declaration, which is deliberately still owed below — the declared
   range is preserved on the signature and only its consumers normalise.

   Two things came out that the plan had not predicted. **The refusal was standing in front of a
   silent wrong answer**: with the checker no longer refusing, the declaration reached `seedFromSig`
   for the first time and Go emitted `func GenSq(n uint16)`, wrapping 1000×1000 to 16,960. And **a
   range in the RESULT position was a declaration nobody checked** — the same false claim spelled as
   an `ensures` was refused with the interval that disproves it, so it now desugars into one, which
   is postconditions.md's swap written in the type language.

   Cost: **byte-identical output on 41 programs × 4 targets**. No speed claim.
1b. **The refusal is BUILT AND IS THE DEFAULT — 2026-08-31.** `emit.Unbounded` refuses an integer operation that
   cannot be proven to stay inside the window and names the escapes. On the
   native corpus **7 of 41 programs refuse, and 3 of those are `examples/int/`,
   which exist to be refused** — so 4 legitimate programs, about 90%
   affordable, which is what this ADR claimed and had not checked.

   **The escape was broken and is now fixed.** `-checked` silently returned a
   different answer on windows — a five-entry map reported `len` of 1 under it
   and 5 without — which made the second escape useless and blocked the default.
   The cause was VARIABLE CAPTURE in the interval pass's rebuild: `openFresh`
   renames only against the set it is given and every call site passed a fresh
   empty map, so a loop over `i` containing an inlined helper whose own loop
   variable is `i` gave both binders the same fresh name and the inner
   `core.Fn` bound the outer's occurrences. Invisible by default, because the
   rebuilt term is discarded unless `-checked` is on — the same way the
   `FnClosed` bug hid in the same place. One shared, SCOPED set of names fixes
   it; scoped, because holding them forever renames siblings and renames the
   same binder again on each of the pass's sweeps.

   **Flipping it changed no generated code at all**: of the programs that still
   emit, every one is byte-identical. The refusal only refuses. Six programs stop
   emitting by default — `examples/int/`'s three, which exist for that, plus
   `tree.oro` and the two `wordcount`s, each of which takes the trap and says in
   its header why it cannot prove instead.

   There is **no flag to turn the check off**, deliberately: "ignore it" is not
   one of the three escapes, and `-checked` is the way out.

   The other measurement worth keeping: **10 of 16 differential cases refuse**,
   because they carry no signatures at all — the harness writes their `main` and
   each is the smallest program that exercises one construct. `(+ acc (t i))`
   sums values read out of a table and nothing bounds a table's elements unless
   someone declares it. That is not evidence against the design; it is evidence
   that a program with no declarations proves nothing, which is the design.

2. **A spelling for the unbounded rung**, since `(int LO HI)` has two finite endpoints and cannot
   say ℤ. This is the one place C is not purely "the existing mechanism, one rung further".

   **RESEARCHED 2026-08-31** — [unbounded-rung.md](../unbounded-rung.md), and it is **two questions,
   only one of which is about spelling.** The BOUNDED-but-huge rung needs no new syntax at all:
   `(int LO HI)` already says it and only the READER refuses, because `KInt` is an `int64`. That is
   the half where the measured factor of four lives. It is safe because **a range endpoint is a
   bound, not a value** — ADR 0012 constrains what a program computes with, and an endpoint
   describes a set — and safe *now* specifically because an out-of-window range makes its operations
   unprovable, which is a compile error rather than a silent fallback to the host word.

   **BUILT 2026-08-31, and in a shape none of the candidates proposed.** An endpoint is a
   compile-time EXPRESSION: `(int 0 (pow 2 70))`, `(int 0 (* 1000 1000))`. hamza's, after the
   recommended candidate hit a cost the research had not accounted for — types are built from
   ordinary terms, so a big literal needs an eighth term kind or an abused field. An expression
   needs neither: it is EVALUATED, never emitted, so `KInt` stays an `int64` and the only
   arbitrary-precision arithmetic in the compiler is one function.

   Rejected and recorded: a bare type name, because `bigint` names a representation, which is the
   distinction ADR 0003 exists to make; and `(int)`, because beside `int` it would be
   indistinguishable at a glance and opposite in meaning. `(int 0 +inf)` remains the follow-on for
   the genuinely unbounded rung.

   And the objection every candidate faces: **the unbounded rung is not a refinement of `int`, it is
   a WIDENING.** Every range today satisfies `[LO,HI] ⊆ W`, which is why a range normalises to `int`
   for typing; ℤ does not, so an unbounded value passed where an `int` is required must be REFUSED.
   That refusal is the surface — it is where a programmer finds out a value became a bignum.
3. **The representation solver** over `word ⊑ big`, and **it must be bidirectional**. Factorial is
   the witness: `(fact (int 0 30))` has every input small and an accumulator reaching 30! ≈ 2.65×10³²,
   so the pressure comes from the declared **result**. A forward-only solver would pass the entire
   current corpus and fail on the first factorial.
4. **R3 per target.** Go, Java and JavaScript ship a bignum; windows ships none and needs one
   written — in Oroboros, which is also the test of whether the language can write its own libraries.

**The top rung is not one implementation, and this is the part the aesthetics hid.** Measured across
all four hosts: **ours wins where the operation is linear and the host wins where it is quadratic.**

| | big × small, big + big | big × big |
|---|---|---|
| Go | ours to ~1,900 limbs | ours to ~6 limbs |
| Java | ours everywhere measured | ours to ~4 limbs |
| JavaScript | ours to ~100 limbs | **never ours** — 148× at 16,384 bits |

So the threshold is **per-operation as well as per-target**, and something must choose. Whole-program
reduction makes that decidable for us — the compiler sees which operations occur on a value, exactly
as [q5c](../spec/q5c-representation-choice.md) chooses pull versus push — but a value whose
representation changes needs a conversion at the boundary, and where that boundary sits is a design
question this ADR opens rather than closes.

**And Karatsuba is unreachable.** It needs recursion, [ADR 0014](0014-recursion-is-not-in-the-language.md)
removed recursion, and divide-and-conquer is the shape it removed. **This is the first measured case
where ADR 0014 has a concrete performance price on a real algorithm.** It is not an argument to
reverse that ADR — the price is confined to one operation and the answer is to call the host's
multiply — but it belongs in ADR 0014's consequences and is recorded there.

**What gets easier.** Constant folding at arbitrary precision becomes sound, which ADR 0009 currently
forbids. `int` gains a portable meaning outside the window for the first time. And the three
behaviours a program can have at an overflow — narrow, promote, trap — are all *written in the
source*, which is the same move as `alloc` naming where the allocation is and the explicit stack in
`tokenize.oro` making the depth limit visible.

**What gets harder.** An exported signature's representation is now ABI. Crossing the top rung is a
performance **cliff**, not a slope — 10–50× on V8's `BigInt`, allocation per operation on
`math/big` and `BigInteger` — and C makes the cliff visible in the source rather than small.

**What is bounded and known.** `adc` is a real hole on x86-64 and a **bounded** one: a declared
two-result `add-carry` recovers 2.12× of 3.94×, leaving **1.85× on addition-heavy bignum code and
nothing on multiplication**. `mul` is not a hole at all — [values.md](../spec/values.md)'s multiple
return passes two results in rax/rdx, which is exactly where x86's `mul` puts its halves.

### What should reopen this

1. **A real application needs declarations in more places than a programmer will tolerate.**
   **This is the one to watch, and nothing measured here can settle it.** The 30-of-39 figure is a
   corpus of numeric kernels and two parsers written by people who knew the analysis. An application
   — timestamps, identifiers, money, string offsets — is unmeasured. If C rejects a large fraction of
   ordinary code, then A's costs are the price of being usable and this ADR is wrong.
2. **The diagnostic proves untranslatable.** An unprovable operation is found on the **residual**,
   after inlining, so naming the source line the programmer should annotate is genuinely hard — ADR
   0018's third trigger in a harder form, because a linearity error has one site and this has a
   chain. A refusal that cannot explain itself is worse than a wrap.
3. **The representation solver turns out to need more than a two-point lattice.** §5's soundness rule
   assumes `word ⊑ big` with `big` absorbing. If a middle rung is added — the fixed 128-bit form the
   measurements make look attractive at two or three limbs, which is where values that just left the
   window actually live — the lattice grows and the rule needs restating.
4. **A host changes underneath a threshold.** V8's `BigInt` crossover at ~100 limbs and Go's at
   ~1,900 are properties of today's implementations. Both are the kind of number
   [ADR 0008](0008-measurement-over-principle.md) says must be re-run rather than quoted, and Java's
   `merge` has already failed to reproduce once.
