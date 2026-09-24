# Assessment: the round that went back over its own proofs

2026-09-24. Deliberately **not** an ADR. The previous nine are
[2026-09-23](assessment-2026-09-23.md), [2026-09-17](assessment-2026-09-17.md),
[2026-09-13](assessment-2026-09-13.md), [2026-09-11](assessment-2026-09-11.md),
[2026-09-09](assessment-2026-09-09.md), [2026-09-06](assessment-2026-09-06.md),
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

Written on hamza's request, with the 09-23 plan partly done. Two days, 17 commits, 11 result documents
and three ADRs (0027, 0028, 0029). The plan is rescored now rather than at its end, because three ADRs
and a corrected headline figure have changed what it was built on.

> **Short answer: the plan half done, the method working as designed, and the balance still bad.**
>
> **The plan.** Items 1 and 2 are done. Item 3 got one package, `math/bits`, and the program the plan
> demanded: `lib/num/u128.oro`, a 128-bit integer over `math/bits` and `strconv`. That program met a
> wall, which became ADR 0027. Item 4, the Windows application, has an owner at last: hamza has one to
> hand over.
>
> **The method found more than it built.** Closing the round's gaps turned up:
> - **two live false proofs** in the analysis;
> - **one cross-target disagreement** at run time;
> - **a counting error in the headline figure** since it has been quoted.
>
> Each was found by a program, a measurement or a planted fault, not by review. Each has a witness that
> fails against the old code.
>
> **The balance.** The compiler grew **3,101** lines (1,236 of them tests), and the programs written in
> the language grew **47**. That is **66 : 1** at the margin, against 368 : 1 last round. Counting hand
> declarations and acceptance programs it is **10.8 : 1**, worse than last round's 7.0 : 1.
> **The largest program did not grow.** `emit : core` rose from **4.23 to 4.28**.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | Profile the tokeniser's compile, and decide | **Done.** Two memos in the Houdini fixpoint: 720 → 405 ms, emission byte-identical ([tokenize-compile](../gauntlet/results/tokenize-compile-2026-09-23.md)) |
| 2 | Close two checking gaps: the unsigned word's value check as a committed test, a missing `-targets` layer refused | **Done** ([checkgaps](../gauntlet/results/checkgaps-2026-09-23.md)) |
| 3 | Packages, program-first: `math/bits`, then `bufio` or `sort`/`slices`; one program per two packages | **Half done.** `math/bits` declared, checked and exercised ([mathbits](../gauntlet/results/mathbits-2026-09-23.md)). The program over it and `strconv` is written ([u128](../gauntlet/results/u128-2026-09-23.md)). The second package is not started |
| 4 | A Windows application: hamza's decision | **Decided.** hamza has an application for when it is scheduled (§5) |

The round went in five phases, counted with the same script as §3.1:

| phase | commits | compiler | of which tests | Oroboros | hand declarations | acceptance |
|---|---|---:|---:|---:|---:|---:|
| **A. Plan items 1–2** | `d37875e`, `1fe5e06` | +219 | +118 | 0 | 0 | 0 |
| **B. `math/bits` and u128** | `22f6cdc`, `fcf8cac` | +397 | +240 | **+44** | +159 | +60 |
| **C. ADR 0027** | `f4f0cc5`, `f09a9fb` | +411 | +146 | −2 | 0 | 0 |
| **D. ADR 0028** | `0d37fbf` … `904e958` | +981 | +199 | +5 | 0 | 0 |
| **E. Closing, before moving on** | `18802dd` … `294728d` | +1,093 | +533 | 0 | +22 | 0 |

**Phase B is what the plan asked for, and it is where the language's own code grew.** Phases C and D
were decisions hamza made on a wall the program met. Each was specified first, with an ADR and a
measurement.

Phase E was hamza's too, in answer to "is there anything we should close before moving forward":
- one run-time disagreement between targets;
- one gap in the test harness;
- two analysis limits the harness then exposed.

## 2. What this round established

**A host call's continuation is a tail position** (ADR 0027). Its tail contexts are
`E ::= [] | let x = e in E | if c E E | (p a…)(λx̄. E)`. The continuation is second-class: it runs once,
now, and never escapes, which is the known-continuation case of CPS compilation (Appel; Kennedy). So a
loop may consume a fallible host call on each iteration, which is how every `bufio` reader is written.
Building it proved the ADR's warning within the hour. Two back-edge walkers that missed the new form
certified a product and a termination on a loop that diverges. They could not ship, because the form
was new, and each is pinned by a witness.

**A declared parameter range is a type, and a type is checked at application** (ADR 0028). Inlining
removes the only place a caller's obligation could be checked, and it re-derives premises, not
obligations. So the reducer marks each obligation, and the pipeline discharges it immediately after
reduction. Measured first: 346 obligations in the corpus, all but one program's already provable.
The build found an exported `print-int` whose declared domain was meaningless past it (`print-int -13`
printed a blank line), and made it total.

**Above the word, a type denotes a set decided by sign and bit length, and every representation
enforces the program's one set** (ADR 0029). The sets a sign-magnitude integer's two O(1) observables
decide form a lattice:

    H = {[0, 2ᵏ)} ∪ {(−2ᵏ, 2ᵏ)}

A type denotes its least member of H, and the program enforces the join. Before, Go, Java, JavaScript
and the fixed limbs admitted **four different sets** for one declaration. Java printed −2²⁰¹ where Go
trapped, and the limbs trapped on a value inside the program's own declared range.

**A buffer keeps its length, and a guard that is a connective narrows.** Neither is new mathematics:
- `len (set b i v) = len b`, extended by induction through loops;
- De Morgan applied to a narrowing, where a failed conjunction is the **join** of two paths.

Both were missing, and both were found by holding the differential cases to proof. Between them they
proved four cases that had needed `-checked`: `map-dynamic`, `map-static`, `len-bounded` and `match`.

## 3. Are we on the right track

### 3.1 The balance of effort

Counted exactly as before. The script reproduces every figure of the 09-23 assessment at its revision
`4ece268`. One definition is made explicit: survey tooling is every `.go` in `gauntlet/stdlib/`, tests
included, plus the two dump scripts.

| | 2026-09-13 | 2026-09-17 | 2026-09-23 | **2026-09-24** |
|---|---:|---:|---:|---:|
| compiler (with tests) | 37,611 | 45,503 | 48,445 | **51,546** |
| of which tests | 12,384 | 15,041 | 16,528 | **17,764** |
| Oroboros written | 1,513 | 1,516 | 1,524 | **1,571** |
| **at the margin** | 4.4 : 1 | 2,631 : 1 | 368 : 1 | **66 : 1** |
| overall | 24.9 : 1 | 30.0 : 1 | 31.8 : 1 | **32.8 : 1** |
| `emit` : `core` | 4.03 : 1 | 4.42 : 1 | 4.23 : 1 | **4.28 : 1** |
| analysis layer | 5,217 | 6,935 | 7,136 | **7,626** |
| analysis layer, extended | 5,410 | 8,727 | 8,942 | **9,432** |
| …plus `bound` and `wordsel` | | | 9,524 | **10,014** |
| largest program | 154 | 163 | 164 | **164** |
| survey tooling | 6,592 | 7,150 | 7,387 | **7,533** |
| hand declarations (`targets/*.oro`) | 2,951 | 3,223 | 3,516 | **3,697** |
| acceptance programs | 99 | 229 | 347 | **407** |

**With hand declarations and acceptance programs counted, the margin is 3,101 : 288 = 10.8 : 1.**

**The analysis layer grew +490 by its fixed definition, and +1,237 counting two new files that belong
to it:**
- `emit/requires.go` (515), which discharges contracts;
- `emit/buflen.go` (232), the buffer-length relation.

The largest non-test growth after those is `emit/interval.go` (+400 raw) and `core/reduce.go` (+331,
the contract marks and the n-ary let).

### 3.2 The analysis layer grew for named refusals, and was corrected more than extended

Every line traces to a named cause:
- a wall the u128 program met (ADR 0027);
- a measured gap in meaning (ADR 0028);
- a measured disagreement between targets (ADR 0029);
- a case the differential suite could no longer pass unproven (buflen, matchguard).

That is the rule 09-23 set, followed.

**But much of phase E is correction, and correction is the part to read.** The analysis had been
claiming things that were false:

| found | how long | how | cost of the fix |
|---|---|---|---|
| **The trip count divided by the last edge's step.** A loop stepping by 10 and then by 1 was numbered 11 trips where 46 happen, and a sum was proven in the word while it wrapped at run time | since trip counts existed. The field's comment said "per the LAST edge examined" | adding an arc for `match`'s reset clause | the meet over every edge. `jsonfmt` then needed the derived step's own measure, which a comment had said was withheld and was not |
| **ADR 0028 read a declared big result as its own type**, when the bound is enforced once per program. A parameter was "proven" and violated | one day | writing ADR 0029's rule for results | the fact is the program's set |
| **Four representations, three outcomes**, for one bounded big program | since bigrepr-2026-09-03 | reading the templates, then probes | two templates, one primitive |
| **Every corpus figure counted an operation in a guard up to three times** | since the figures have been quoted | a count that grew for no reason in a proven program | `refine` counts nothing |

The corrected figures are **2,007 of 2,054 operations** and **343 of 381 loops**. The numbers left
unproven, 47 and 38, are the same as before the correction. The proofs were right; the totals were
inflated.

### 3.3 Compile time: flat, and half the tokeniser bought back

- The tokeniser went 720 → 405 ms, from two memos a profile pointed at (plan item 1).
- Through the rest of the round the gate read 0.97× to 1.09× median.
- It flagged one real regression before commit: `render` on windows at +930 ms, and `u128` too. It
  came from the separated-interval arc evaluating each `again` argument once per pair of variables.
- Measured serially against a baseline worktree before and after the fix: `render` windows 2,103
  against 2,116 ms, `u128` 198 against 238.

### 3.4 The thesis: five Go packages deep

By ADR 0022's measure, the supported packages are `unicode/utf8`, `encoding/hex`, `strconv`,
`encoding/binary`'s varints, and **`math/bits`**, plus `io`'s interfaces. `math/bits` came with its
algebra:
- the measures `Len`, `LeadingZeros`, `TrailingZeros` and `OnesCount`;
- the double-word ring operations, with `Add64`'s carry law linear and so stated as an `ensures`;
- `Mul64`'s law, named as non-linear.

It is checked against the host on all 55 names, and exercised by 331 lines of output identical to Go's.

**For the first time a package's program is a library, not a test.** `lib/num/u128.oro` is positional
notation in base 2⁶⁴ (Knuth 4.3.1). `examples/u128/factorials.oro` uses it, and two differential cases
check it on every run. It found **four compiler faults**, and a wall that needed a design decision.

### 3.5 The gauntlet

Last benchmarked in full 2026-09-07. This round's accepted emission changes touched benchmarked code
in two places, each measured:
- **the bounded-bignum check** (`gen_bigbounded`): 200! at 1.03× on Go and 1.01× on Java, both inside
  the noise floor;
- **the windows programs that print**, through the total `print-int`: `sieve-win-bench` median 90 ms
  against 90.

The full check passes at HEAD.

### 3.6 Process: five things

**1. TWO FALSE PROOFS WERE DESCRIBED ACCURATELY IN THEIR OWN COMMENTS.** `scKind` was documented as
"per the LAST edge examined". fixpoint-2026-08-27 recorded that a withheld measure "did not help: i
still gets kind{1,1} from the other clauses". Both sentences describe the unsoundness exactly, and
neither reads as a warning. **A comment that states what code does is not a review of whether it
should.** The witnesses now exist; the lesson is that an accurate description of a rule is not
evidence the rule is sound.

**2. THE HEADLINE FIGURE WAS WRONG IN THE DENOMINATOR, AND NOTHING COULD HAVE NOTICED.** The unproven
counts were right, so every regression check on them was right. The total was inflated by
re-evaluation, and no test counted a known program's operations by hand.
`TestAGuardsOperationIsCountedOnce` now does. CLAUDE.md's lesson *a survey's first number describes
the measurer* gained its third form: a count is a measurement too.

**3. THE MEASUREMENT'S SCOPE WAS WRONG ONCE MORE.** requires-2026-09-24 counted `where` only on
non-exported definitions, and missed the export every Windows harness print calls. The build found it,
refusing 22 cases. That is the seventh correction of a first number, in the same direction as the
others: too narrow.

**4. THE PLANT DISCIPLINE CAUGHT FIVE WEAK TESTS BEFORE THEY SHIPPED.**
- Two shadowing tests in buflen passed with the shadowing removed, because a `let` made the collision
  they tested impossible.
- Three plants in matchguard were missed at first, each exposing a test that could not fail.

Every soundness test written this round has a plant that fails it, and one unreachable branch is
recorded as unchecked, not left untested silently. This is the method working, and it is worth saying so.

**5. A PROCEDURAL FIX I STATED LAST ROUND, I DID NOT FOLLOW.** Shell heredocs ate backslashes in edit
scripts **at least five times** this round. The memory note exists, and 09-23 recorded "the fix is procedural:
write edit scripts with the file tool". Each time it cost a failed edit or a broken file, caught before
commit. The rule stands: an edit script with a backslash is written with the file tool, always.

## 4. What is left, read off the outcomes

**47 unproven operations, 38 unproven loops**, by program:

| where | operations | loops | what it is |
|---|---:|---:|---|
| `shift-div`, `limb-subdiv` (differential) | 32 | | the sweep compiles each case alone, and its exported `run` has no signature. **The harness, which builds `main`, proves them**: the sweep's figure is the sweep's |
| `examples/int/` (collatz, power, fib), `kara/core` | 11 | 1 | meant to be refused |
| `map/dynamic`, the two wordcounts | 4 | | genuine |
| the JS natives (sieve, generic, report, smooth, centroid, dot, search, wordcount) | | 16 | loops the analysis cannot bound on V8's word |
| `freq` | | 12 | three targets × four loops |
| `render`, `big/power` | | 7 | bignum loops. The interval domain does not model a bignum |
| `u128` (library and example) | | 2 | `decimal`'s loop: the tuple-component law |

**Named and not built, from this round's documents:**
- a declared big result narrower than the program's widest, enforced at its own bound (ADR 0029);
- signed limbs;
- a sign enforced for `(int 0 +inf)`;
- element bounds for `alloc (table …)` and for a map's cells, which `map-len` and `map-keys` still
  declare;
- geometric accumulation;
- a definition passed as a value and applied later, whose contract is not checked (ADR 0028);
- a pure host call whose results are all discarded, still emitted;
- printing the literal −2⁶³ on windows, refused.

## 5. What is next

**1. `bufio`, PROGRAM-FIRST.** ADR 0027 was built for it and has one consumer, a differential case. The
algebra comes first: a `Reader` is a buffered view of a byte stream, `ReadByte`/`ReadRune`/`ReadString`
are partial maps with an error sum, and `Scanner` is a split function iterated to a fixed point. Its
program is a line tool over standard input, with the pair `bufio` and `strconv`: for example, number
each line, or sum a column. It is written in the language, it is counted, and it is not an acceptance
program. The tuple-component law and int64 fragment constants are the walls it is likeliest to meet
again. **Met a fourth time, the tuple-component law becomes the next ADR.**

**2. THE WINDOWS APPLICATION — hamza's.** It is taken off the carried list and made a plan item. It
begins when hamza hands over the application:
- **first, a measurement:** what it needs from Win32 (windows, messages, GDI, resources) against what
  the target declares, what is callable, and what links. The survey says 90.5% declarable and **31.0%
  callable and linkable**, and that gap is where this application will meet its walls;
- **then the spec,** and an ADR for each wall, as this round did.

It is the largest program the project will have written, and the balance measure is waiting for
exactly that.

**3. THE ANALYSIS GROWS ONLY FOR A REFUSAL ONE OF THOSE TWO PROGRAMS NAMES.** That was the rule this
round too, and it held. The addition is the lesson of §3.6.1: **every rule the analysis relies on gets
a witness that it is sound, not only one that it proves something.** Trip counts, facts and summaries
were built to prove more. Only planted faults show whether what they prove is true.

### Deliberately not next

- **The element-bound and geometric-accumulation limits** (bounds-2026-09-24), until a program is
  refused by one. `map-len` and `map-keys` declare them honestly.
- **Per-result enforcement above the word, and signed limbs.** No program declares two big bounds or a
  signed big range.
- **The higher-order contract gap.** No program in the corpus passes a signed definition as a value.
  Measure before building.
- **A phase-wise trip count**, tighter than the meet for loops whose edges step very differently.
  Honest now, only loose.
- **The windows, V8 and JVM unsigned rungs; modular arithmetic as a type (ℤ/2ⁿ); dependent result
  ranges; `ByteOrder`'s values; records, `with`, named views; re-benchmarking the gauntlet.** These
  carry their reasons from 09-23 unchanged.

## 6. The one sentence

This round wrote one real program, and let it and the checks around it find what the analysis had been
getting wrong: two false proofs, a cross-target disagreement and an inflated headline figure. Each is
fixed with a witness. But 47 lines of program against 3,101 of compiler says the next round's measure
is the same as the last one's: the programs, `bufio`'s tool and hamza's Windows application, have to
be what grows.
