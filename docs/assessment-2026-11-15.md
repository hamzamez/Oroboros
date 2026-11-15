# Assessment: the Windows target prints LLVM IR, big integers on it, and `os` written in the language

2026-11-15. Deliberately **not** an ADR. The previous sixteen are [2026-10-24](assessment-2026-10-24.md),
[2026-10-21](assessment-2026-10-21.md), [2026-10-20](assessment-2026-10-20.md),
[2026-10-14](assessment-2026-10-14.md), [2026-10-03](assessment-2026-10-03.md),
[2026-09-28](assessment-2026-09-28.md), [2026-09-24](assessment-2026-09-24.md),
[2026-09-23](assessment-2026-09-23.md), [2026-09-17](assessment-2026-09-17.md),
[2026-09-13](assessment-2026-09-13.md), [2026-09-11](assessment-2026-09-11.md),
[2026-09-09](assessment-2026-09-09.md), [2026-09-06](assessment-2026-09-06.md),
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

Written at hamza's request before P8 ("so we don't lose the big picture"), with the 10-24 plan not
finished: its item 6, the Windows track, is in its third step. **Three weeks, 98 commits, 36 ADRs**
(0077–0112), **50 result documents**. The last round was three days and 8 commits.

> **Short answer: the Windows target is real, and the round was too long.**
> - The native target prints LLVM IR (ADR 0080), MASM is retired, and the target is a theory with
>   a parameter (ADR 0093).
> - Seven walls of Σ_LLVM were researched and decided: bits, widths, pointers, vectors, numbers.
> - Integers above the word run on windows with no host bignum (K(ℕ\*), ADR 0099–0111), and five of
>   their programs are 0.79–1.8× Go.
> - Since today `os` runs there, written in the language over kernel32 (ADR 0112).
>
> **The margin is 29 : 1** (16,901 lines of compiler against 582 of the language counted the usual
> way; 11.7 : 1 with `emit/natnum.oro`, 867 lines of J the count does not see).
>
> **What did not move:**
> - TinyRetroPad, the application the track exists for (6c, 6d);
> - the second triple;
> - three plan items (9, 10, 12);
> - the gauntlet's full benchmark, last run 09-29.
>
> One standing measure, "windows faster than Go", turned the round's second half into a campaign on
> big-integer arithmetic.

---

## 1. What the last plan asked for, scored

| | asked | happened |
|---|---|---|
| 1 | **The documents tell the truth** | **Done** (docs-2026-10-25), and `cmd/check`'s `gauntlet` step regenerates the 26 files |
| 2 | **Two surface questions** | **Done**: `lo..` (ADR 0077), `pub`/`export` (ADR 0078) |
| 3 | **Three defects the audit found** | **Done** (defects-2026-10-26): no module defines a name of Σ_L |
| 4 | **A declared set is a claim; the facts choose ρ** | **Done** (ADR 0079), then `Nat` |
| 5 | **CLAUDE.md rewritten as state** | **Done 10-26, and undone by accretion**: 12,464 words today, 72% of the 17,309 it cut (§3.4) |
| 6a | **MASM or LLVM IR** | **Done**: LLVM IR (ADR 0080), MASM dominated on speed and size |
| 6b | **`ir/llvm`, then MASM retired; Σ_LLVM's walls** | **Done**. The printer at windows parity (the sieve 0.64 of hand-written MASM's process); MASM retired; seven walls decided (ADR 0082–0086), C strings (0091), the allocator a term (0092) |
| 6b′ | **The LLVM target "fully"** (hamza 11-04) | **In its third step**. The layering (0093), the C ABI (0094), the unsigned word, F64, rendering (0095), storage (0096–0100, 0107), K(ℕ\*) (0098–0111), `os` (0112). Two cases still skip windows (`u128`, hamza's question). The gauntlet on windows and the second triple are not started |
| 6c, 6d | **TinyRetroPad's surface and walls** | **Not started** |
| 7 | **A π justified by its branch** | **Done** (pijustified-2026-11-02): W6 is Theorem E |
| 8 | **Two defects the LLVM printer named** | **Done** (quotient-2026-11-03): −2⁶³ / −1 decided like `+` |
| 9 | **An `again` after a nested loop's join point** (`wc`, 16%) | **Not started** |
| 10 | **The Go library: `cmp`** | **Not started** |
| 11 | **The analysis grows only for a refusal a program names** | **Kept**. It refused one fix twice this week: the rule transfer's ⊥, applied for a suspected refusal, removed again when its own planted fault showed no program needed it |
| 12 | **The JVM's JSON tokenizer slows mid-run** | **Not started** |
| 13 | **When to reduce and when to call** | **Decided** (ADR 0102, Q1–Q4); M-S1, M-S2, M-S4 measured; **P8 now triggered** (oswin-2026-11-15) |

The round in phases, from the commit log:

| phase | dates | ADRs | what it was for |
|---|---|---:|---|
| **A. Plan items 1–5** | 10-25 – 10-26 | 3 | the documents, the surface, the claim set |
| **B. LLVM IR and Σ_LLVM's walls** | 10-26 – 11-04 | 13 | 6a, 6b: bits, widths, pointers, vectors, numbers, C strings, the printer, MASM retired |
| **C. The LLVM layer, fully** | 11-04 – 11-09 | 3 | the layering, the C ABI, the unsigned word, F64, rendering |
| **D. Storage and big integers** | 11-09 – 11-15 | 16 | R1–R4, then K(ℕ\*)'s speed under the standing measure: base 2⁶⁴, the double word, the full adder, Karatsuba, squaring, printing; factoring (0101, 0102) |
| **E. `os` on windows** | 11-15 | 1 | 6b′ step 3's next item |

## 2. What this round established

**Wrapping is a quotient, not a sort** (ADR 0082). The ring operations on ℤ and the residue map
ℤ → ℤ/2ⁿ commute, so `& | ⊕ ~ << >>` are ℤ's own operations, and fusion computes a term under `mod 2ⁿ`
in n-bit arithmetic. No new type entered the language.

**A representation is an abstraction function** (H, d), chosen by place (ADR 0083). The word, the
unsigned word, pairs of digits, limbs and the double word are one mechanism, and ρ picks by the proven
fact.

**A number is an element of ℤ[1/10], and rounding is its one lossy operation** (ADR 0086). Every answer
is a function of the inputs (Theorem D). F64 is a set, IEEE's arithmetic a scope, and there is no NaN.

**A realization is a term over the host's operations** (ADR 0088, 0089), and **∅ is initial** (ADR
0090): a call into it never returns, whatever declares it.

**A C string is the kernel of a monoid homomorphism** (ADR 0091), and today **valid UTF-8 is a
recognizable set** (ADR 0112): V = φ⁻¹(F) for φ into a finite transition monoid. Both are decided by
their homomorphism on a term and by a guard at run time.

**Storage is a fixed point and a class** (ADR 0096, 0097, 0100, 0107). Ownership is the greatest fixed
point of birth, move and borrow, and release follows death. A dead region is a linear token, handed to
the next allocation of its capacity class. A table born full takes no zero fill.

**An integer of any size is a numeral** (ADR 0098, 0099, 0103–0111). Signed values are the group
completion of the magnitudes. The base is 2⁶⁴, every step's intermediate held in the double word that
the decision proves.
- a product above 64 digits is evaluation and interpolation, Karatsuba's levels a tensor power;
- a square is the product's quadratic form, by polarization;
- printing is a tree of divisions, each node Algorithm D in place.

**Factoring never changes an answer or a verdict** (ADR 0102): which definitions are kept as functions
is invisible to meaning (Theorem 1), and a contract proves less than its sites (Theorem 3).

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-10-21 | 2026-10-24 | **2026-11-15** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 71,377 | 72,967 | **89,868** |
| of which tests | 23,532 | 23,871 | **27,677** |
| `core` / `emit` / `ir` | | 19,773 / 33,080 / 20,114 | **23,150 / 35,626 / 31,092** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 2,009 | 2,086 | **2,668** |
| … in core-term nodes | 11,894 | 12,491 | **15,680** |
| J outside that count: `emit/natnum.oro` | | 0 | **867** |
| **at the margin** | 6.6 : 1 | 20.6 : 1 | **29 : 1** (16,901 : 582); 11.7 : 1 with natnum |
| overall | 35.5 : 1 | 35.0 : 1 | **33.7 : 1** |
| `emit` : `core` | 1.69 : 1 | 1.67 : 1 | **1.54 : 1** |
| (`emit` + `ir`) : `core` | 2.72 : 1 | 2.69 : 1 | **2.88 : 1** |
| the IR's analysis | 3,342 | 3,350 | **4,070** |
| hand declarations (`targets/*.oro`) | 4,444 | 4,518 | **5,011** |
| survey tooling | 9,833 | 10,035 | **10,060** |

**Where the compiler grew**: the IR, by 10,978 lines. By theme, of the largest files:
- numbers: `ir/doubles.go`, `numbers.go`, `core/num.go`, `emit/numnames.go`, about 3,300;
- the LLVM backend: `ir/llvm/llvm.go`, `carry.go`, `ir/realize.go`, about 2,000;
- storage: `ir/release.go`, `reuse.go`, about 1,300;
- widths and fusion: `ir/pairs.go`, `fuse.go`, `bitival.go`, about 1,400;
- big integers: `ir/progfunc.go`, `double.go`, `limbs.go`, about 1,000, besides 1,245 changed lines of
  `natnum.oro`.

**Why the margin is high.** Phase B decided seven walls, and each wall is mostly a decision procedure
and a printer: compiler. The language's growth in phase D is K(ℕ\*)'s 867 lines of J. That J is the
largest program in the language now, and the balance script counts only `examples/` and `lib/`.
`freq`, 194 lines, is still the largest program it sees. **Most of the round wrote compiler for a
library, and the library counted nowhere.**

### 3.2 Precision and performance

**Provability**:
- 7,462 of 7,516 operations and 826 of 856 loops, against 2,103 of 2,148 and 353 of 376 at 10-24.
- The counts rose with the corpus on windows. Its unproven operations are still 54: `examples/int/`,
  `kara` and three cases, each by design or by an unbounded input.
- 30 loops are not proven to terminate: input-driven, and six of them windows' `os` cell's, whose
  progress is a count the host returns.

**Performance, the standing measure** (windows against Go, quiet machine, radix-2026-11-15):

| program | windows / Go |
|---|---:|
| 2,000 quotients | **0.79** |
| a numeral times a word | **0.94** |
| fib(10⁵), printed | 1.14 |
| fact(20,000), printed | 1.31 (2.77 before this week's printing) |
| fib(10⁶) by fast doubling | 1.8 |
| the Mersenne hash (ADR 0103) | **0.49** of idiomatic Go |
| the sieve | 0.64 of hand-written MASM's process |

- **Per operation**, our product is 4.2× `math/big`'s and our square 3.3×. The leaf row alone is 1.6×
  Go's assembly, and the C model of our design is already 3×.
- The kernels are decided (a target may name CPU features; kernels in the language) and postponed.

**The gauntlet** (the seven programs, the one fixed commitment) was last benchmarked in full on
**09-29**: 0.990, 0.977, 0.987. Emission is byte-identical by gate on Go, JavaScript and the JVM, so
its text there is unchanged. But nothing measured it this round, and on windows only the sieve is
measured. The float programs emit there since llvmf64-11-06, and their parity is not measured.

**Compile time** is gated and flat on the corpus. Two costs this round were large and measured:
- the 2,000-quotient program took 10 minutes to compile, cut to 19.5 s by ADR 0101;
- today, programs that call windows' `os` take 0.3–11 s, and `freq` 50 s to reach a refusal.

### 3.3 Soundness: the round's findings

| found | lived | how |
|---|---|---|
| A nested `fn if` captured every `if` | since modules | the audit; **closed**: no module defines a name of Σ_L |
| A rule's index past its end; a buffer a host filled read as its zero fill | since rules / ADR 0070 | building `pe` (views-2026-10-30); **closed** |
| The open-range hole in a result's claim | since ADR 0072 | the grids' checks against binary64; **closed** (ADR 0087) |
| −2⁶³ / −1 decided by nothing | since `/` | the LLVM printer's programs; **closed** |
| `search` accepted printing `icmp sgt double`, which clang refuses | since llvmf64 | running it: the emission step compares text and never assembles. **Open** as an item |
| Three JVM cases skipped for a month after the JVM could run them | ADR 0083 – llvm64 | by chance; a skip states no checked reason. **Open** as an item |
| W14 on a dead arm: a live-buffer read under a guard copied the step | since the state translation | squaring; **avoided**, a shape rule |
| `DischargeRanges` swallows a lowering error, and the refinement layer then refuses naming the wrong obligation | since irstep4f | oswin, twice misleading. **Open** |
| `alloc(o[a:b])` of a segment is erased though it flows into a host call | since ADR 0074 | oswin. **Open** |

**Today's thirteen compiler paths** (oswin-2026-11-15) were not soundness bugs, but untried code. The
first library written in J took:
- a one-result cell call;
- a borrow that also fills cells;
- a product with a sum among its factors;
- a borrowing definition answering a product;
- η-reduced and constant rules under `alloc`;
- and eight more of the kind.

Each failed loudly. None miscompiled.

### 3.4 Process: five things

**1. THE ROUND WAS THREE WEEKS LONG, AND ITS PLAN GREW IN PLACE.** Item 6 became 6a–6d, then 6b′, then
R1–R4, M4a–M4c, steps 1–4 and their sub-steps. Each was recorded in CLAUDE.md as it happened. The 10-26
rewrite aimed at a third of 17,309 words, and the file is back at 12,464. **A plan that grows in place
has no end at which to assess it.** The rule exists ("an assessment when a plan finishes"), and a plan
that never finishes evades it. The fix: an assessment at least every ten days or 30 commits, whichever
comes first, and a sub-plan of more than three steps gets its own assessment.

**2. A STANDING MEASURE BECAME THE PLAN.** "The windows implementation is faster than the Go one; as long
as it is slower, the gap is work owed" (hamza, 11-10) is a measure, and it is right. Read as an order,
it made six days of K(ℕ\*) speed: the base, the double word, the full adder, Karatsuba, squaring,
printing. Meanwhile the application (6c, 6d), the second triple and three plan items did not move.
Each step was the derivation's and was measured. The cost was opportunity, and no measure saw it.
**A standing measure names debt; the plan orders it.** The K(ℕ\*) programs are 5 of the corpus's
hundreds.

**3. THE GAUNTLET WAS NOT RUN.** The one fixed commitment's full benchmark is seven weeks old. The gate
keeps its text byte-identical on three hosts, so nothing there should have moved. But "should" is a
claim, and windows is its fourth target with only the sieve measured.

**4. MEASURING ON A BUSY MACHINE.** The kernels' first numbers were taken while the machine trained a
model, and were corrected a day later (kernels-research §7). A benchmark-method species to add: **ask
whether the machine is quiet, and record it.**

**5. AN UNTRIED PATH COSTS A DAY WHEN A PROGRAM FIRST TAKES IT.** `os` on windows found thirteen
compiler paths no program had taken, because no library had been written in J before. Two diagnoses
were misled by a swallowed error. And a suspected analysis fix was made twice and removed twice by its
own planted fault: that rule worked, and it cost time. **Write the libraries in the language early,
and let them find the paths**, which is the project's own rule ("write programs, and let them demand
the analysis"), applied to libraries.

## 4. What is left, read off the outcomes

**Owed now (triggered):**
- **ADR 0102's P8**: `roundtrip`, `ledger`, `pe` and `freq` on windows pass the reducer's depth;
  P3's lifted preconditions with it.

**The 10-24 plan's unfinished items:**
- 6b′'s gauntlet on windows at parity and the second triple;
- 6c, 6d, TinyRetroPad;
- 9, an `again` after a nested loop;
- 10, `cmp`;
- 12, the JVM tokenizer.

**The harness** (6b′ step 3's last bullet, carried):
- stale skips found by running every skipped case;
- every windows `.ll` assembled;
- a compared file that holds K(ℕ\*)'s text.

**Found this round and open**: the swallowed lowering error, the erased segment `alloc`.

**Decided and postponed**:
- the kernels (design, LLVM's wide product, assembly if measured);
- Toom-3;
- subquadratic division at a node;
- `nat.div` in place;
- V3, the decision of the repair's image.

**hamza's open questions**:
- whether `u128-fact` and `u128-ops`, a Go-only library's cases, count against "fully";
- concurrency's six questions (ADR 0081).

## 5. The next plan

**P8 first, with a ledger; then the measurements the round skipped; then the application.**

0. **A ledger in every step** (hamza, 2026-11-15: "if we are going to pay a cost, I would like we know
   what we paid, and what for"). Each result document gets a section, **Paid and bought**. It covers
   compile time, run time, code size, proofs (counts, and any obligation that moved), refusals changed,
   compiler lines, the language's lines, emitted text changed per target, and a rule made less general.
   Each is a measurement, or named as owed. A cost no one measured is not recorded as zero.
1. **P8, a program's definitions kept as functions** (ADR 0102), research first, hamza's decision after.
   Its ledger, registered before building:
   - **to buy**:
     - reduction steps, depth and compile time for programs calling a J library;
     - residual and code size, one copy a definition instead of π(f);
     - no consumer copied into a kept function's constructor sites;
     - `roundtrip`, `ledger`, `pe`, `freq` and the gauntlet's file programs on windows;
   - **to pay, each measured**:
     - proofs: Theorem 3, a contract proves less than its sites. By P4 every lost proof falls back
       to inlining, so verdicts don't change, but the count of definitions kept against those that
       fall back is the price;
     - facts at the boundary: a call's result is read through its declared set or `ensure`, so
       relational facts are lost, and obligations move to callers;
     - run time: a call, and constant arguments not folded into the callee. The host's inliner may
       recover them (M-S3, on four targets, against the noise floor);
     - a sum returned across the boundary is tag and slots and a branch, not eliminated;
     - the compiler's size: P3's lifting, and a second mode (kept or inlined) every rule must be
       tested in;
     - emitted text on Go, JavaScript and the JVM: every kept definition changes a file, so the first
       build is restricted (P2's threshold, or a definition the trigger names) and each change is
       accepted with its reason.
2. **The gauntlet benchmarked in full**, on four targets, windows included, at parity or explained. It
   was last run 09-29.
3. **The harness**:
   - a skip states a checked reason (every skipped case × target run, a stale skip reported, a planted
     one caught first);
   - every windows `.ll` assembled by `clang -c`;
   - a compared file holding K(ℕ\*)'s text;
   - `DischargeRanges` reporting a lowering error instead of swallowing it.
4. **The gauntlet's file programs on windows** (`wc`, `freq`, `jsonfmt`, …) at hand-written speed: the
   standing measure, read on the programs it was meant for.
5. **The second triple**, `i686-pc-windows-msvc`.
6. **TinyRetroPad, 6c then 6d**: its Win32 surface measured, then its walls (the window procedure,
   records), spec first.
7. **CLAUDE.md as state again**, after 1–6 settle what is current, to under 8,000 words. The plan's
   history goes to this assessment and the results.

**The measure of the round**: P8's ledger closed in measurements, the gauntlet run on four targets, and
TinyRetroPad's surface measured. An assessment at ten days or 30 commits, whichever comes first.

**Kept as rules**:
- a refusal test's premise is computed;
- an `-accept` that changes a count updates every document quoting it;
- every experiment gets a time limit;
- a check refuses something false before it is trusted to pass something true;
- the analysis grows only for a refusal a program names;
- **and now a ledger**.

### Deliberately not next

- **The kernels, Toom-3, subquadratic division, `nat.div` in place**: decided or researched, postponed.
  The standing measure names them as debt, and none is on this plan.
- **Items 9, 10, 12** (`again` after a nested loop, `cmp`, the JVM tokenizer): carried, each with its
  measurement. They come after the application's surface.
- **V3, errors.md §4.4's `fails` form, folding by pattern**: decided against or waiting on a program.
- **ARM, Linux and macOS targets themselves; concurrency's surface; Android's toolkit**: unchanged.

## 6. The one sentence

The round made Windows a real target: LLVM IR, big integers without a host, and `os` written in the
language. It also let one standing measure run the plan for a week and let the plan grow for three. The
next round should keep a ledger of every cost, build P8 against its ledger, run the gauntlet on four
targets, and reach the Windows application.
