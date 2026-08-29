# Precision integers: how to get there, and what must be right first

> **Re-measured 2026-08-28 — [checked-2026-08-28](../gauntlet/results/checked-2026-08-28.md).** The
> affordability question this document opens is now answered: `-checked` emits a **byte-identical
> file on 30 of 39 programs**, and costs **1.05×** — inside the noise floor — on the one program that
> still pays. The central worry recorded below, that declaring ranges *"changes NOTHING on the two
> parsers"*, was [the fixpoint bug](../gauntlet/results/fixpoint-2026-08-27.md) and is withdrawn.
>
> Two things that measurement does **not** settle, and they are now the whole question. `-checked` is
> **detection, not precision** — it panics where exactness would promote. And promotion is a
> **representation change**: a value that may leave the window is a boxed bignum, which is the one
> thing the core may never contain. So the proof rate decides *whether a boxed value can appear at
> all*, and the open decision is the policy at an unprovable site — box, trap, or **refuse**. That is
> an ADR, not a build.


Research, 2026-08-27. **Deliberately not an ADR** — naming a design as a decision before
measurement is the predecessor project's failure in ADR form.

## The goal, as hamza states it

> I want precision integers. They should not interfere with us getting the best performance on the
> target when the range is within the supported integers. Beyond that we have precision integers on
> all targets and they will cost us the same they would cost if we implemented them on the target.

Three claims, and they are separable:

| | | the bar |
|---|---|---|
| **P1** | integers are exact — no wrapping, no silent truncation, no undefined behaviour | correctness |
| **P2** | where the compiler can bound the range, the emitted code is the host's own operation | **zero** cost |
| **P3** | where it cannot, precision still holds on every target | the cost of a hand-written bignum **on that target** |

P3's bar is the gauntlet's bar pointed at a different thing, which is the right way to state it. It
is also the honest one: it does not promise bignums are fast, it promises we do not make them
slower than the host already would.

---

## 1. This is mostly already decided, and the blockers are mostly already cleared

Two documents predate this one and neither should be re-argued.

**[ADR 0003](decisions/0003-range-typed-integers.md)** decided P1 and P2 in August:
*"mathematical semantics, machine representation, range declared in the type"*, with `(int 0 255)`
becoming `uint8_t` in C and a plain `int` on the JVM, and the sentence that matters most here:
**"the range is the portability contract."** It also refused unbounded-by-default (*"this repeats
Shen exactly"*) and machine-types-by-default. What it left as *"a Tier 1 capability in a package"*
is P3.

**[data-model.md §7](spec/data-model.md)** lists **eleven questions** that one decision drags behind
it, and **§8** draws the dependency graph. Its verdict was `N-E` — range-typed integers — *as the
direction*, blocked on two things.

**Both blockers have since been cleared, and nobody has noticed in one place.**

| §8 blocker | status |
|---|---|
| **the product**, needed for "a bignum with an inline fast path" | **built** — [values.md](spec/values.md), measured Go 0.99× / Java 0.97× with zero allocations ([multiresult-2026-08-22](../gauntlet/results/multiresult-2026-08-22.md)) |
| **interval analysis**, "what hamza's preferred integer design turns on" | **built** — `emit/interval.go`, plus size-change termination and, as of today, one Farkas multiplier |
| *(not on the graph, but needed)* an error result | **sums built** — [sums.md](spec/sums.md), 1.00× against hand-written Go |

So the question is no longer *"is the machinery there"*. It is **"how much does the machinery
actually prove, on the programs we now have"** — because P2's whole value is the fraction of
operations that reach the fast path, and every operation that misses it pays P3's price.

---

## 2. The load-bearing measurement, re-taken — and it has moved

[intervals-2026-08-19](../gauntlet/results/intervals-2026-08-19.md) answered this once:
**39% with nothing declared, 81% with one range declared**, and predicted 81% was a floor because
the entire residue was one implementable class.

That was measured on the corpus of the time: sieves, dot, smooth, search, report — **countable
numeric loops**. Re-taken today with `cmd/intervals` on a corpus that now includes two parsers:

| program | nothing declared | one range declared |
|---|---|---|
| sieve (native) | 100% | 100% |
| sieve (portable) | 100% | 100% |
| dot | 100% | 100% |
| digit-sum, sum-range | 100% | 100% |
| smooth | 0% | **100%** |
| search | 0% | **100%** |
| report | 50% | **100%** |
| wordcount | 0% | 67% |
| **tokenize** | **64.5%** | **64.5% — unchanged** |
| **tree** | **91.2%** | **91.2% — unchanged** |

**The old corpus reproduces and improves: one declared range takes every numeric loop to 100%.**
The 81% floor was indeed a floor; size-change termination lifted it.

**The parsers do not move at all.** Declaring ranges on their inputs changes nothing, which is a
qualitatively different failure from "not enough is declared". It is worth knowing exactly why.

> **CORRECTED 2026-08-27. That was a bug, not a property of the programs.** The interval fixpoint
> was not monotone — `restore` installed a snapshot by reference, so a branch's refinement leaked
> past the `if` and some loop variables settled at their initial values
> ([fixpoint-2026-08-27](../gauntlet/results/fixpoint-2026-08-27.md)). Two more gaps compounded it:
> tables.md's structural `len` was unrecognised by both the interval and linear layers, which knew
> only the retired portable layer's `alen`/`slen`; and an exactly-known length — a literal array's,
> or a `build`'s — was thrown away at the `let` that call-by-need introduces.
>
> With all three fixed the answer inverts: `tokenize.oro` is **86.7%** with nothing declared and
> **100% with one range declared**, and `tree.oro` is 88.9% and 91.3%. **Declaring a range is what
> unlocks the parsers, exactly as intervals-2026-08-19 predicted** — so this document's central
> worry is withdrawn and the plan is in better shape than it concluded. §3's postcondition is still
> wanted; it is no longer the thing everything is blocked on.

### 2.1 One cause, and it is the same one in both programs

```
tokens:  no descent on the cycle loop(stk,i,nt,sp,mx,ok): nt>nt
measure: no descent on the cycle loop(nodes,stk,i,nn,sp,ok): nodes>=nodes stk>=stk nn>=nn
```

Every unproven operation in the tokeniser traces to `nt`, the token counter, and every unproven
operation in the tree traces to the same shape. The chain is:

1. The loop's progress variable `i` is assigned a **scanner's return value**, not `i + k`.
2. So `stepOf` fails, `i` is not a size-change witness, and the loop has **no descent**.
3. So there is **no trip count**.
4. So an accumulator with no guard on itself — `nt`, `nn`, `acc`, `d` — is **unbounded**, and so is
   everything computed from it.

The trip count is what `emit/interval.go` calls *"THE POINT: a trip count bounds every variable the
guards say nothing about"*. When it is unavailable, nothing downstream is bounded.

**And `go.*` is 0 of 15 in the tokeniser — not one multiplication is bounded.** That is the worst
possible operation to lose, because [product-2026-08-19](../gauntlet/results/product-2026-08-19.md)
prices a checked multiply at **1.87× where the hardware high-multiply is reachable and 7.40× where
it is not** — and JavaScript has none.

### 2.2 Confirmed by construction

Adding an explicit trip counter to the tokeniser's main loop — the same device
[`tree.oro`](../examples/json/tree.oro)'s walk already carries for termination:

| | ops bounded | multiplications bounded |
|---|---|---|
| tokenize as written | 64.5% | **0 of 15** |
| tokenize + explicit trip counter | **78.5%** | **8 of 15** |

So the mechanism is confirmed and it is not mysterious. It is also a **program-level workaround**,
which is the wrong place for it to live.

> A structural note worth carrying: the tree's walk needed an explicit trip bound to be **proven
> terminating**, and that same bound is why the walk does not appear in the unbounded list. One
> device bought termination *and* range provability. They are the same fact — *this loop runs at
> most N times* — asked by two different analyses.

---

## 3. What actually unlocks it: a postcondition naming the result

The missing fact is one sentence: **`scan-string` returns something strictly greater than `i` and
at most `len src`.** With it, `i` descends toward `len src`, the loop has a trip count, and every
accumulator in it is bounded.

Three ways to supply it, and they are not equally good.

**(a) Declare it.** `(sig scan-string ((src (array int)) (i int)) int (where …))` — but `where`
today constrains *parameters* only. Naming the **result** is a postcondition, and
[general-purpose.md](general-purpose.md) already lists it as owed for a different reason:

> *What is missing: postconditions naming the result, a fallible result, and a surface for linear
> handles.*

This is SAL's `_Out_range_`. **The Win32 work and the integer work want the same feature**, which is
the strongest evidence available that it is the right feature.

**(b) Infer it.** After staging the scanner is inlined, so the fact is *present*; expressing it
requires a relational invariant (`j ≥ i+1`) that the interval domain cannot represent. This is the
octagon case, and it is now the third independent demand for one
([decidability-map.md](decidability-map.md) already calls octagons the highest-value move).

**(c) Restructure the program.** What `tokenize.oro` does today with its `(go.< j 0)` clauses, and
what §2.2's trip counter does. It works and it is a tax on the programmer for the analysis's
limitation.

**(a) is the cheapest, it is already owed, and it is the one that composes**: a declared
postcondition survives into the residual as an assumption at every inlined site, where an inferred
one has to be re-derived.

---

## 4. The design, once the analysis carries it

### 4.1 Three regimes, and only the third is new

| | when | emitted |
|---|---|---|
| **R1** | the compiler bounds the range inside the host's word | the host's own operator — zero cost |
| **R2** | it cannot, and the value stays small at run time | the host's checked form, then R1's operator |
| **R3** | the value genuinely exceeds the word | the host's bignum |

R1 exists today: [selection-2026-08-19](../gauntlet/results/selection-2026-08-19.md) shows one
`(where …)` on a `sig` taking the sieve from 10-of-10 operations checked to **0 checked and 3 of 3
loops proven terminating**.

R2 exists today, behind `-checked`. Three targets declare a checked form —
`go.add-exact`, `java.addExact`, `x64.add-checked` — and **JavaScript declares none**, which is
[overflow-2026-08-19](../gauntlet/results/overflow-2026-08-19.md)'s price list showing up as a
capability.

R3 does not exist at all.

### 4.2 R3 is a capability the target declares, not a runtime we ship

This is the part that fits the parasite model best, and it is better news than it looks.

| target | bignum | notes |
|---|---|---|
| Go | `math/big` | in the standard library |
| Java | `java.math.BigInteger` | in the standard library |
| JavaScript | `BigInt` | **in the language**, since ES2020 |
| windows | **nothing** | we would have to write one |

Three of four hosts already have exactly the thing, which means `(prim big-add …)` in a target file
and no compiler change — the same shape as `map`, and the reason
[ADR 0001](decisions/0001-parasite-model.md) exists. It also means **P3's bar is met by
construction on three targets**: using the host's bignum costs what the host's bignum costs.

Three cautions.

**JavaScript's `BigInt` does not mix with `Number`.** `1n + 1` is a `TypeError`. So the R1↔R3
boundary is a real conversion on that host, not a widening — which makes the fast-path-with-promotion
design *more* expensive there, and is consistent with data-model.md §1.2's measured **3.74×** for
JavaScript's fixnum check against Java's 1.31×.

**windows would need a bignum written, and that is a real cost** — but it is the same cost as the
allocator that target already needed for tables ([wintables-2026-08-25](../gauntlet/results/wintables-2026-08-25.md)),
and it is a target-file-plus-library job, not a compiler one.

**And a bignum in every binary violates requirement 6** if it is linked unconditionally. It must be
pulled in only when a program actually reaches R3 — which the capability graph already does for
every other name.

### 4.3 What must NOT happen: one name meaning three things

data-model.md §0's rule, and it is sharp: *a portable name must publish its price*. A single `+`
that silently means "native, or checked, or bignum" would mean something different on each host by
up to **3.74×**. §1.2 already ruled that inadmissible, and nothing here changes it.

The resolution is that the *range* selects the regime, and the range is written in the program. That
is not a hidden cost — it is a **derived** one, which data-model.md §1.4 identifies as N-E's whole
advantage over the alternatives: *"the only candidate where the price is not merely named but
derived."*

---

## 5. The eleven questions, with today's status

data-model.md §7 says choosing this drags eleven decisions behind it. Six are now answered by
[integers.md](spec/integers.md), which measured them on all four targets rather than reading four
specifications.

| # | question | status |
|---|---|---|
| 1 | representation | **the open design question.** §4.1's three regimes are the proposal |
| 2 | overflow: wrap, trap, or promote | **open, and it is the decision.** Today: wrap outside the window (ADR 0012) |
| 3 | division rounding | **settled** — truncates toward zero; all four hosts agree (integers.md) |
| 4 | remainder sign | **settled** — follows the dividend; all four agree |
| 5 | division by zero | **settled as refused** — a precondition, not a behaviour |
| 6 | `int`/`float` comparison | open |
| 7 | `int` → `float` | **settled** — exact, *because* the window is binary64's exact-integer range. **Exactness above 2⁵³ breaks this**, and it becomes a named conversion |
| 8 | `float` → `int` | **settled as refused** out of domain — three hosts, three answers |
| 9 | equality | open — structural on a bignum, bitwise on a word, and they must agree |
| 10 | ordering | open |
| 11 | constant folding precision | **inverts in our favour** — see below |

**Item 11 is the one that pays.** Today, folding integer arithmetic at compile time would be an
[ADR 0009](decisions/0009-staging-preserves-results.md) hazard: arbitrary precision at compile time
against fixed width at run time is the `0.1 + 0.2` bug in integer clothing. **If integers become
exact, the hazard disappears** — folding at arbitrary precision *is* the runtime semantics. Precision
by default buys back an optimisation we currently cannot do soundly.

**Item 7 is the one that costs**, and it is not small. `int → f64` is exact today *only* because the
window is binary64's exact-integer range — integers.md calls that out as the constraint that looked
like a concession to JavaScript and turned out to make the one conversion everybody needs free.
Exact integers past 2⁵³ end that. The conversion has to become explicit and lossy-by-name.

---

## 6. What must be right before, in order

Each of these has a reason to be where it is, and each is cheap to check.

**1. Postconditions naming the result.** §3. Without it, two of the corpus's most realistic programs
prove nothing, and the whole design is a tax rather than a mechanism. It is already owed for Win32.
*Trigger to move on: a `sig` can constrain the result and the tokeniser reaches ≥90%.*

**2. Re-measure provability on the parsers.** The number in §2 is the one that decides whether P2 is
"nearly always" or "about two thirds". It must be taken **after** (1), on programs that were not
written to please the analyser. *Do not build the integer design before this number exists* — that
is intervals-2026-08-19's own rule and it held up.

**3. Decide whether octagons are needed, or whether declared postconditions suffice.** These are
alternatives, not a sequence, and (1)+(2) answer it. The tree's remaining 40 undischarged
obligations are the sample to classify.

**4. Answer questions 1, 2, 6, 9 and 10 in a specification, before any code.** This is the order
[strings.md](spec/strings.md) exists to enforce — *"nothing goes in without a specification saying
how it behaves on every target"* — and booleans and sums both followed it successfully.

**5. Element width from the range.** `(array (int 0 255))` should be `[]byte` on Go, `byte[]` on
Java, `Uint8Array` on JS, `db` on x86. This is the *cheapest real payoff* and it needs no bignum: it
subsumes the boolean special case in `ElemBytes`, it subsumes
[indextype-2026-08-25](../gauntlet/results/indextype-2026-08-25.md)'s hardcoded platform fact, and
[json-tree-bench](../gauntlet/results/json-tree-bench-2026-08-26.md) measured our 64-bit element
costing **1.19× on the JVM** for exactly this reason. **Do this one first regardless of what happens
to the rest** — it is where the measured money is, and it is reversible.

**6. Then R3.** Declare the host bignums, keep them out of binaries that do not reach R3, and
measure against hand-written use of the same library on each host.

### What is explicitly NOT on this path

**Sub-byte storage.** `(int 0 7)` in three bits is expressible natively on **zero** of our four
targets — Go has no bit array (`[]bool` is one byte per element), the JVM has no sub-byte storage,
JavaScript has no integers. Getting it means hand-rolling shift-and-mask, which is *lowering further
than the target requires*. It also makes adjacent elements share a word, so a `set` becomes a
read-modify-write and two threads writing neighbours race — which silently breaks the aliasing
assumption ADR 0018 rests on. **Deferred with a named trigger**: a target that natively addresses
sub-byte elements, or a measured program where packing wins by more than the noise floor.

**Reinterpretation.** "Types are a representation choice and the operations decide what a value is"
is true of hardware and fatal here: the moment a byte array can be read as a word, every bound,
range and termination proof is unsound. The half we want is *types choose representation*. The half
we must refuse is *operations determine meaning*. They are separable and the slogan merges them.

---

## 7. The precedent this must not repeat

[assessment-2026-08-20 §2](assessment-2026-08-20.md) records that representation selection was wired
into the default path **once already**, by accident, and had to be reverted. It:

- contradicted a written ADR silently (ADR 0012),
- breached requirement 5 by up to **4.54×**, and
- made cross-target divergence *worse* — trapping on Go, Java and windows while silently losing
  precision on JavaScript.

The lesson recorded there is the governing one for this whole document:

> **Turning it on should be the consequence of deciding exact integers, not the cause.** A
> demonstration wired into the default path is a decision, whether or not anyone made one.

So: build (1) and (5), take (2)'s measurement, write (4)'s specification, and only then change what
`int` means — with an ADR that supersedes ADR 0012 and says plainly what a program's portability now
depends on.
