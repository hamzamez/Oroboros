# Assessment: the vertical round, and the thing that has now been named three times

2026-09-06. Deliberately **not** an ADR — writing "the direction is correct" as a decision would
recreate the predecessor's failure. The previous three are
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

> **Short answer: yes on the method and yes on the thesis, and the thesis now has its first
> number. No on the balance of effort.** The last assessment named a risk —
> *"nothing here is a program anyone would want to run"* — and it has now been named in three
> consecutive assessments and acted on in none. In the seventeen days since, the compiler grew by
> **19,611 lines** and the corpus written *in the language* grew by **477**.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | Decide the integer semantics, then turn selection on | **Done, completely.** [integers.md](spec/integers.md), [ADR 0019](decisions/0019-precision-by-declaration.md), bounded-by-default on with no flag to turn it off, and all four rungs of the representation ladder built |
| 2 | The product | **Done.** [values.md](spec/values.md), 0.99× on Go with 0 allocations, no target declares it |
| 3 | `bytes` and scalar bitwise | **Half, and the other half is a refusal with a reason.** Element width from a range gives `[]byte`; bitwise is deliberately **not** promoted, because V8 coerces to int32 — an observable divergence inside the portable window |
| 4 | Move the gauntlet onto the native targets | **Done for the programs that matter**, and the gauntlet still runs green — `dot` spot-checked today at 456 ns native against 478 hand-written |
| 5 | **Write something awkward** | **No.** Again |

Four of five, and the fifth is the one that has been on the list since August 13.

## 2. What this round actually established

Not a small list, and it should be weighed before the criticism.

**Arbitrary precision, end to end, on four targets.** ADR 0019's third escape is real and at parity
(1.01×/0.99×/0.97× against hand-written bignum code), the representation is a **target declaration**
rather than a shape of the source, a fixed-limb library is written *in Oroboros* and gives windows
arbitrary precision for the first time, and the mutable rewrite beats careful hand-written Go.

**A declaration survives inlining.** `the` — no new term kind, no new surface — after an experiment
that *measured* the alternative: the analysis reports `[-inf, +inf]` for a loop with a constant trip
count of six, so derivation provably cannot replace the declaration.

**Strings are derived rather than borrowed.** `Scalar*`, six forced escapes, and the first real text
program folds rather than indexing.

**And the parasite thesis has its first number, from two ecosystems.** Go: **70.0% declarable,
19.0% usable.** Win32: **72.6% and 33.5%**, with 7,813 generated primitives that build under MASM
and run. This is the central claim of [ADR 0001](decisions/0001-parasite-model.md) measured for the
first time in the project's life.

**The method is still the strongest asset here, and it caught itself twice this week.** The Go
survey's first pass scored methods at 0% usable by *defining* an opaque receiver as unusable; the
Win32 survey's first pass attributed 23.5% of the API to a language limit that was its own missing
typedef resolution. Both were caught, corrected, and written down as method errors. *In both
surveys the first number was the tool talking about itself* — and knowing to look is worth more
than either number.

## 3. Are we on the right track

### 3.1 The thesis: yes, and it is now falsifiable rather than rhetorical

70% declarable is a real result for a claim that had been argued for five months. And the honest
half is better than the number: **the surveys located the blockers precisely, and the top two are
cheap.**

- **Several results in a `prim` — 19.8% of Go's callable standard library.** `Prim.Result` is a
  single Go string. The *language* has had several results since August, as reader sugar, measured
  at parity, with no target declaring anything. This is a field in a data format.
- **A declared result range the interval layer reads.** A host call is ⊤, so ADR 0019 refuses
  arithmetic on **every host result in every ecosystem** — verified on Go and then reproduced,
  independently, on Win32.

Those two are why a program cannot open a file today. `os.Open` returns `(*File, error)`. The
reason this language has no file I/O is not a design position; it is one struct field.

### 3.2 The core: yes, and the discipline held

Seven writable term kinds, unchanged. Sums, `match`, maps, multiple return, strings and the
ascription all landed with **zero or one** reduction rules between them. Every construct promoted
to the language works on all four targets and no target declines one. The minimality trap has been
avoided in the direction that matters: the core did not grow to accommodate features.

### 3.3 The balance of effort: no, and here is the measurement

| | 2026-08-20 | 2026-09-06 | |
|---|---:|---:|---|
| `core/` | 3,590 | 7,073 | ×1.97 |
| `emit/` | 9,508 | 25,636 | ×2.70 |
| compiler : core | 2.65 : 1 | **3.62 : 1** | |
| Oroboros written (`examples/` + `lib/`) | 572 | 1,049 | ×1.83 |

**19,611 lines of compiler for 477 lines of Oroboros — 41 to 1 at the margin, 31 to 1 overall.**
`emit/interval.go` alone is 2,972 lines, which is *larger than the entire core was* at the last
assessment.

The last assessment named this exactly — *"the compiler is growing faster than the language… the
ratio deserves watching, because a language whose claims depend on a clever compiler is a language
whose claims are hard to check."* **It was watched and it got worse.**

And the corpus number is the one that matters more:

> **The largest program ever written in this language is 112 lines.** The whole corpus is 1,049
> lines across 71 files. There is no file I/O on the Go target, no network on any target, and
> windows cannot construct a string.

[general-purpose.md](general-purpose.md) says the goal is *apps on Windows and Android, websites in
the browser, backends in the cloud*. Nothing in this round moved toward that, and the two surveys
are the first work that even measured the distance.

### 3.4 Two pieces of process debt, both self-inflicted

**ADR 0020 is owed and unwritten.** [arrays-revisited.md](arrays-revisited.md) established that
ADR 0018's **trigger 2 has fired** — Karatsuba's workspace costs 1.07×–1.66× because a buffer is
not a nameable type — and ADR 0018 says firing it *"should produce an ADR adopting uniqueness on
parameters rather than a workaround."* The evidence has since arrived from **four** independent
directions: Karatsuba's arena, ADR 0013's stencil, the mutable bignum, and the limb library's
per-operation allocation. One ADR closes ADR 0013 *and* ADR 0018's trigger at once, and the
project's own convention says a fired trigger produces a decision.

> **RE-RUN 2026-09-07** — [gauntlet-2026-09-07](../gauntlet/results/gauntlet-2026-09-07.md). All
> seven programs at parity on all three targets, nineteen comparisons, largest gap 1.13x. And the
> portable-layer debt below is **not** as scored: three programs are still benchmarked on the
> retired layer, and the reason is that their benchmarks were written against a different
> interface.

**Ten results in September, none of them a gauntlet measurement.** The gauntlet is *"the one fixed
commitment"*, and the last time a gauntlet program was benchmarked against hand-written code was
`rebench-2026-08-27`. It still passes — that was checked rather than assumed — but a fixed
commitment nobody runs is a commitment in name.

## 4. The risks, ranked

**1. The corpus is not evidence for the claim being made.** Every measurement in
`gauntlet/results/` is sound and every one is of a numeric kernel, a parser, or a bignum. The claim
is general-purpose. Three assessments have said this.

**2. The analysis layer is where complexity is accumulating, and it is the least checkable part.**
It is well-tested — the containment harness generates 1,877 programs and had to be made to fail
before it was trusted — but 2,972 lines of interval analysis is a lot of machinery to stand behind
`int`.

**3. Ecosystem access is 19% and the fix is not being worked on.** The surveys measured it and the
next thing built was another survey.

**4. `-checked` is load-bearing in a way nothing intended.** Both surveys found that any program
touching a host result needs it. That was meant to be an escape, not a mode.

## 5. What is next

**Ordered by value per line of compiler, which is the ratio §3.3 says to start watching.**

**1. ~~The backend is chosen by the flag string.~~ DONE, 2026-09-06** —
[backend-2026-09-06](../gauntlet/results/backend-2026-09-06.md). It had **two** live instances
rather than the one this list assumed: `targets/portable-js.oro` had been emitting Go source from a
JavaScript target since August, unnoticed because nothing had ever run `cmd/gen` against it. And
verifying the fix turned up a worse bug — **the emitter was not a function of its input**, five
lookups taking the first match out of a Go map, so six identical runs produced two different
programs. Both are fixed and both are pinned by tests that fail against them.

**2. ~~The two format changes the surveys located.~~ DONE, 2026-09-06** —
[multiresult-2026-09-06](../gauntlet/results/multiresult-2026-09-06.md). Go's callable standard
library goes from **70.0% declarable and 19.0% usable to 88.0% and 32.0%**, obtainable host types
from 108 to 198, and `os` from 18 usable names to 76. The acceptance test passed on its own terms:
[examples/io/wc.oro](../examples/io/wc.oro) opens `go.mod`, counts its newlines and prints 5.
The language needed nothing — both changes are fields in a data format and one branch in an
analysis, and the elimination form `((f x) (fn (a b) …))` already existed.

**3. ADR 0020 — uniqueness on parameters.** Owed. **Researched 2026-09-06**
([uniqueness.md](uniqueness.md)), and the reading corrects this item: **rule R closed two of the
four demands**, so the live case is Karatsuba's workspace alone. It also finds the feature much
smaller than expected — ADR 0018 already made uniqueness a distinction between two *types* rather
than an attribute, and whole-program reduction removes the propagation problem, so the surface is
one missing type name and the implementation is `CheckLinear` with a different seed. **Written 2026-09-06** —
[ADR 0020](decisions/0020-uniqueness-on-parameters.md), with the corrected count, superseding
ADR 0013 and amending ADR 0018's consequence 3. **Built 2026-09-07** —
[uniqueness-2026-09-07](../gauntlet/results/uniqueness-2026-09-07.md). The research's estimate held:
**51 non-comment lines, one type name, `CheckLinear` with a different seed, and no backend change**.
Measured at the boundary the trigger named — **0 allocations against 512 KB per call**, 4.5x on a
kernel whose work is one pass over its workspace. ADR 0013 is superseded and ADR 0018's trigger 2 is
discharged.

**4. ~~Write an application.~~ DONE, 2026-09-07** —
[jsonfmt-2026-09-07](../gauntlet/results/jsonfmt-2026-09-07.md). A JSON pretty-printer, 95 lines,
the first tool. **It found four bugs, two of them silent wrong answers** — one shipped the previous
day (ADR 0019 vacuous inside a multi-result continuation) and one three weeks old (the
bounds-check narrowing truncating its own source). That is the answer to why this item kept being
deferred and should not have been. The original entry follows.

**Write an application.** Not a benchmark and not a kernel. Something with input, output, error
handling and a shape nobody chose to suit the analysis — the smallest honest candidate is a
command-line tool that reads a file, and it is blocked on exactly item 2, which is the argument for
doing item 2 first. This is the item that has been deferred three times, and the way to stop
deferring it is to make it the *acceptance test* for item 2 rather than a separate task.

**5. ~~windows can print a string and cannot construct one.~~ DONE, 2026-09-07** —
[winstrings-2026-09-07](../gauntlet/results/winstrings-2026-09-07.md). Two assembly templates over
a static arena; `render.oro` runs on all four targets. **The gap was not the strings**: once they
existed, a one-conjunct bug in the limb rung — a fixpoint gated on `HasBig()` as well as `bigOK()` —
was still refusing the program on the only target with no host bignum. The original entry follows.

**windows can print a string and cannot construct one.** `concat` and `string-of` over `build`,
keeping the bare NUL-terminated pointer — the last capability gap in the integer work, and it makes
`render.oro` run on four targets instead of three.

### Deliberately not next

**More interval or termination work.** It is 2,972 lines and it is not what is blocking anything on
this list. Octagons stay refuted by measurement.

**Concurrency.** Measured at **4 symbols** of Go's portable API surface. The absence costs almost
nothing at the boundary, which is the opposite of what the missing-feature list suggests.

**Interfaces, closures and structs.** All three are refused on arguments this project has already
made, and the surveys *priced* them rather than reopening them: 1.9%, 1.8% and 13.9% respectively.
Pricing a refusal is not the same as reversing it.

**Another survey.** Two is a finding; three would be a habit.

## 6. The one sentence

The method is right, the core is right, the thesis is now measured rather than argued — and the
project has spent seventeen days making a very good compiler for programs it has not written. The
next round should be judged by what the language can be *asked to do*, not by what the compiler can
*prove*.
