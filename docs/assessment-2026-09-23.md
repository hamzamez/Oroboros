# Assessment: the round an `int` became an integer, and the package count moved

2026-09-23. Deliberately **not** an ADR. The previous eight are
[2026-09-17](assessment-2026-09-17.md), [2026-09-13](assessment-2026-09-13.md),
[2026-09-11](assessment-2026-09-11.md), [2026-09-09](assessment-2026-09-09.md),
[2026-09-06](assessment-2026-09-06.md), [2026-08-20](assessment-2026-08-20.md),
[2026-08-19](assessment-2026-08-19.md) and [2026-08-13](assessment-2026-08-13.md).

Written on hamza's request when the 09-17 plan's first item finished. Six days, 29 commits after the
last assessment, 10 result documents and three ADRs (0024, 0025, 0026).

> **Short answer: yes on the goal, yes on the method, and the balance improved but is still bad.**
>
> **The goal moved for the first time since 2026-09-14.** Supported Go packages went from **2 to 4**:
> `strconv`, and `encoding/binary`'s varint half, joined `unicode/utf8` and `encoding/hex`. Both new
> packages hit the integer window first. hamza decided that as
> [ADR 0026](decisions/0026-an-int-is-an-integer.md), and it was built in five steps with every check
> passing after each. No program in the corpus changed legality.
>
> **The method held, and the new gate earned its keep the day after it was built.** The compile-time
> gate flagged the unsigned-word pass at **7.81×** on one program before it was committed, and the fix
> made accepted programs pay nothing.
>
> **The balance.** The compiler grew **2,942** lines (1,487 of them tests), and the programs written in
> the language grew **8**. That is **368 : 1** at the margin, against 2,631 : 1 last round. Counting
> the hand declarations and acceptance programs that ADR 0022 made real Oroboros, it is **7.0 : 1**,
> the best since 2026-09-13. `emit : core` fell **4.42 → 4.23**. The analysis layer grew **+215**, plus
> **582** in two new files that belong to it (`bound`, `wordsel`).
>
> **Six days in, no new program has been written.** 77 files were respelled for the new surface,
> and two acceptance programs were added. Nothing else was written in the language.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | Resume the Go standard library: `encoding/binary`, `strconv`, by ADR 0022, with the analysis growing only for a named refusal | **Done.** Both hit the same named refusal: a 64-bit host *result* declared narrower than the host returns, and `EncodedLen` demanding a precondition Go does not have. Research ([what-an-int-is.md](what-an-int-is.md)), a decision (ADR 0026), a build ([word-2026-09-22](../gauntlet/results/word-2026-09-22.md)), then the packages ([strconv-2026-09-22](../gauntlet/results/strconv-2026-09-22.md)) |
| 2 | Gate compile time against the baseline, then decide whether to buy back the 6.6× | **Half done.** The gate is built and proven on the historical regression ([compiletime-2026-09-21](../gauntlet/results/compiletime-2026-09-21.md)). The tokeniser has **not** been profiled and nothing was bought back |
| 3 | Rewrite `CLAUDE.md` as a state, not a history | **Done** the same day, with hamza's agreement: 60,603 words → a state |
| 4 | A Windows application | **Not started**, for the fourth assessment running |

**Two and a half of four.** The work went in four phases:

| phase | dates | compiler (net) | Oroboros (net) | what drove it |
|---|---|---:|---:|---|
| **A. The program surface** | 09-17 → 09-20 | +1,611 | +15 | hamza's questions on how a program reads: the `def` shorthand, constants as values, literal element types, flat `let`, `cond`, comments specified, module paths as the host's, nested modules, the inventory test |
| **B. Compile time gated** | 09-21 | 0 (in `cmd/check`) | 0 | plan item 2 |
| **C. What an `int` is** | 09-21 → 09-22 | +1,252 | 0 | the named refusal in `encoding/hex` and the next two packages |
| **D. The packages** | 09-22 | +80 | 0 | plan item 1; +293 lines of hand declarations and +118 of acceptance programs |

Phase A was not driven by a package. It was hamza's, it was specified first each time, and every step
was byte-identical or accepted with a reason. It re-spelled the corpus (+739 / −724 raw lines in
`examples/` and `lib/`) without writing into it.

## 2. What this round established

**A window fixed in the language is W(S) for some set of targets, and W(S) = ⋂ Exact(T) shrinks when
a target is added.** Antitonicity is the whole argument against ADR 0012's window. It is now a test
(`TestTheMeetIsAntitone`) on the target files as they are:
- int64 for Go, the JVM and windows;
- ±(2⁵³−1) once JavaScript joins;
- 2³¹ once `blas` does. `blas` realizes `int` as C's `int` and turned out to be the int32 target
  hamza's question was about, already in the tree.

**At a host boundary a declaration is an abstraction, and abstraction is variance.** A declaration
covers a host function when it promises to pass less and accepts all that is returned (Cardelli).
The survey's `uint64 → int` was sound for 137 parameter-only functions and a lie at 206 result
positions. Honest now, the survey's headline numbers do not move, because "usable" is syntactic;
what moved is what a usable declaration *says*.

**Portability is computed, and the three properties were derived to be incompatible, not chosen
between:**
- P2, independence from the target set;
- P3, a host word held in a host word;
- P5, portability by construction.

Today's compiler keeps P2 and P3 and reports P5 (`cmd/portable`), which is ADR 0001's stance applied
to integers. The research measured the cost of giving up P5-by-construction on the corpus: nothing.

**The unsigned word rests on one theorem.** q : ℤ → ℤ/2⁶⁴ is a ring homomorphism, and
S = [−2⁶³, 2⁶³−1] and U = [0, 2⁶⁴−1] are both sets of representatives. So `+ − ·` compute in either
type after the residue map. `< = / %` don't commute with q, so they are emitted only where one type
holds both operands. It was checked on 100,010 values against `math/big`, it runs at **0.99×**
hand-written Go, and a planted division on signed residues is caught by a differential case. The type
checker cannot see that bug; only the expected answers can.

**Widening the analysis found six latent faults, each hidden by the old saturation or the old window:**
- `isqrt` capped at 2³¹, an unsound bound past 2⁶²;
- a `%d` of an endpoint that would print a struct;
- constant folding that could wrap int64 under a 64-bit word;
- a program refused on **every** target by a contradiction the compiler made with itself (`ValueType`
  said `int` while the solver said `big`);
- `PromoteBig`'s erased term discarded when nothing was promoted;
- a repeated `(import …)` silently replacing the first.

That is the lesson *a refusal can hide a wrong answer* again, in its other form: here a refusal hid
a program that was perfectly legal.

## 3. Are we on the right track

### 3.1 The balance of effort

Counted exactly as before. The counting script was checked first: it reproduces every figure of the
09-17 assessment at its revision `c2cc351`, including two definitions the table leaves implicit —
the extended analysis layer includes `sct.go`, and survey tooling includes the two dump scripts.

| | 2026-09-11 | 2026-09-13 | 2026-09-17 | **2026-09-23** |
|---|---:|---:|---:|---:|
| compiler (with tests) | 37,037 | 37,611 | 45,503 | **48,445** |
| of which tests | 12,115 | 12,384 | 15,041 | **16,528** |
| Oroboros written | 1,382 | 1,513 | 1,516 | **1,524** |
| **at the margin** | 18.5 : 1 | 4.4 : 1 | 2,631 : 1 | **368 : 1** |
| overall | 26.8 : 1 | 24.9 : 1 | 30.0 : 1 | **31.8 : 1** |
| `emit` : `core` | 4.13 : 1 | 4.03 : 1 | 4.42 : 1 | **4.23 : 1** |
| analysis layer | 5,175 | 5,217 | 6,935 | **7,136** |
| analysis layer, extended | 5,368 | 5,410 | 8,727 | **8,942** |
| …plus `bound` and `wordsel` (new) | | | | **9,524** |
| largest program | 154 | 154 | 163 | **164** |
| survey tooling | 4,559 | 6,592 | 7,150 | **7,387** |
| hand declarations (`targets/*.oro`) | 2,944 | 2,951 | 3,223 | **3,516** |
| acceptance programs | | 99 | 229 | **347** |

**With hand declarations and acceptance programs counted, the margin is 2,942 : 419 = 7.0 : 1**,
against 19.5 : 1 last round. `core` grew +879, most of it the reader's new surface (`let`, `cond`,
comments, the word) and none of it new term kinds: still seven kinds and the same reduction rules.

**Where the compiler grew, non-test:** `emit/interval.go` and `emit/wordsel.go` (the unsigned word),
`emit/bound.go` (exact endpoints), `emit/target.go` (words, layers, constants past int64), and
`core/let.go` / `core/read.go` (the surface). The inventory test alone is +464, and it checks the
spec's word list against the compiler's.

### 3.2 The analysis layer: grew less, and grew for a named refusal

Last round it grew 3,317 lines for no new program. This round it grew 797 counting the two new files,
and every line traces to one refusal: a host integer the window made dishonest. That is the rule the
last assessment set, followed.

It still grew, and the new part is on the legality path: the unsigned-word selection decides how a
value in U is computed. The counterweight was built with it:
- legality stays in `record`, so an operation in U that the selection misses is **refused**, never
  wrapped;
- 100,010 values checked against `math/big`;
- two differential witnesses, with a planted wrong division caught.

**But the 100,010-value check lives in a scratchpad, not in the repository** (§3.6).

### 3.3 Compile time: no regression, and nothing bought back

Measured serially and warm, today, on binaries built at `e9a0ac5` (the gate's baseline) and at HEAD,
each with its own target files:

| | `e9a0ac5` | HEAD |
|---|---:|---:|
| tokenize, go | 745 · 765 ms | 778 · 743 ms |
| jsonfmt, go | 227 · 214 ms | 223 · 244 ms |
| tree, go | 853 · 811 ms | 881 · 867 ms |
| freq, go | 13.6 · 13.7 s | 14.8 · 14.6 s |

Every row is within 1.07×, which is inside the noise floor. The machine is slower today than on 09-21:
freq took 8.8 s then on the same kind of binary, so today's figures must not be compared with that
document's. **The tokeniser's 6.6× from the round before is still there**, and profiling it was the
cheap half of plan item 2 that did not happen.

The gate's own record this round:
- it flagged the unsigned-word pass at 7.81× on json-tree (95 → 235 ms serially) before commit;
- that led to the right design, selecting when U is declared or as one retry after a refusal that found
  an operation in U;
- the full check has read 1.00×–1.10× on every run since.

### 3.4 The thesis: the Go number is four packages deep

By ADR 0022's measure — declared by hand, checked against the host, exercised by a program that prints
the host's answer — the answer is **4 of about 167 Go packages**, one of them in part:
- `unicode/utf8`;
- `encoding/hex`;
- **`strconv`**, the whole package;
- **`encoding/binary`**, the varint half. The `ByteOrder` half hangs off unexported types reached
  through package variables.

Plus `io`'s interfaces, now five with `ByteReader`.

Each new package brought its algebra first:
- **strconv:** F_b injective, P_b a partial left inverse whose composite F_b ∘ P_b is idempotent, and
  saturation past the word.
- **encoding/binary:** LEB128 a prefix-free code over U, and zig-zag a bijection from the word onto U.

Each also named what it cannot say:
- `ParseInt`'s range depends on `bitSize`'s value;
- `Uvarint`'s count is a three-way sum in a sign;
- `PutUvarint`'s exact buffer requirement is nonlinear.

### 3.5 The gauntlet

Last benchmarked in full 2026-09-07. This round's accepted emission changes touched gauntlet code in
one place: tokenize and tree on Go (literal element types, `[]int` → `[]byte`). tree was re-measured
in [literalelem-2026-09-19](../gauntlet/results/literalelem-2026-09-19.md): 4,747 → 4,688 ns, inside
the noise floor. Every other accepted change was a refusal text, a baseline, or a new source. Parity
holds by construction. The full check passes at HEAD, all five steps.

### 3.6 Process: five things

**1. THE STRONGEST EVIDENCE FOR THE NEW RUNG IS NOT IN THE REPOSITORY.** The 100,010-value check of the
unsigned word lives in a session scratchpad. What the repository runs is:
- **two differential cases, on one target of four**: they skip js, java and windows, because U is
  `big` there and those rungs don't exist;
- `u64-digits`'s six inputs.

*A path nothing runs is a path nothing checks*, and *a harness that cannot fail proves nothing*. This
harness can fail, but nobody will ever run it again.

**2. A LOADER THAT SILENTLY SKIPS A MISSING LAYER hid a mistake of mine.** Git Bash converted only
part of a `-targets a;b` argument, so the first layer named a directory that does not exist. The
loader treated that as "this layer does not have the target" and moved on. I then spent time
diagnosing an override bug that wasn't there. The same silence would hide a typo in any `-targets`.
A layer named explicitly should exist.

**3. A NEW SPECIES OF BENCHMARK-METHOD ERROR, caught twice before recording:**
- a HEAD binary built from `git stash` failed to compile, because the stash leaves untracked files
  behind, and its "timings" were fast failures;
- a correctly built HEAD binary read *today's* target files, which it cannot parse, and so refused.

Both looked like numbers. The rule — time a binary at the baseline's commit — needs its corollary:
**with that commit's own target files**, which means a worktree. Added to `CLAUDE.md`'s list.

**4. SIX DAYS AND NO NEW PROGRAM.** The same failure as last round, smaller. Phase A re-spelled the
corpus into the new surface, and phases C and D wrote two acceptance programs, which are tests. The
largest program grew by one line. The surface work was requested, specified and good. But the
language's own code is where its surface should be tested, and nothing new was written in it.

**5. TWO DOCUMENTS CARRY A SUPERSESSION NOTE INSTEAD OF A REWRITE.** `docs/spec/integers.md` and
`tables.md` open with a note pointing at ADR 0026, but their bodies still speak of "the window" some
two dozen times as if it were the language's. `state.md` was corrected properly. The same working hazard
recurred too: shell heredocs ate backslashes in edit scripts five times despite a memory note, and
each cost a revert. The fix is procedural — write edit scripts with the file tool — and it is now
what I do.

## 4. What is left, read off the outcomes

**Unchanged from 09-17 §4**: 52 unproven integer operations and 37 unproven loops, with the same
members. The proof counts did not move all round, and the only outcome changes for those programs were
refusal texts.

**Unbuilt from this round's own documents:**
- the JVM's realization of U (a `long` holding the residue, 60× over `BigInteger` measured);
  windows' unsigned qword; V8's split pair (4–5× over `BigInt` measured);
- a comparison between a possibly-negative `int` and a value past 2⁶³ (refused by name);
- `u64` table elements;
- a declared `(portable …)` claim enforced by the check (`-require` is its command-line form);
- the refinement layer unfolding a constant's name inside a `where`;
- a result range that depends on an argument's value; a sum in a sign as a `variant` at a boundary;
  `ByteOrder`'s package variables.

## 5. What is next

**1. PROFILE THE TOKENISER'S COMPILE, AND DECIDE.** It is carried from 09-17 and cheap. The last
assessment's guess — Houdini rounds or Fourier–Motzkin sizes — is a claim to measure, not to repeat.
The gate will show a fix as clearly as it showed the regression.

**2. CLOSE THIS ROUND'S TWO CHECKING GAPS**, small and first:
- the unsigned word's value check as a committed test, the emitted `u64` functions against
  `math/big` over edges and random values, shown to fail against the planted division;
- a layer named explicitly on `-targets` that does not exist is refused, not skipped.

**3. PACKAGES, PROGRAM-FIRST.** Next is **`math/bits`**. It is the algebra of the words the last round
built: `Len`, `OnesCount`, `LeadingZeros`, and the double-word ring operations `Add64`, `Sub64`,
`Mul64`, `Div64`. It is pure, it is almost entirely U, and part of its law is linear and so checkable
as an `ensures`: `Add64`'s carry satisfies `sum + carry·2⁶⁴ = x + y + carryIn`. `Mul64`'s
`hi·2⁶⁴ + lo = x·y` is not linear, and should be named as such.

Then `bufio` or `sort`/`slices`. The comparator in `slices.SortFunc` is a callback, and callbacks.md's
tiers are untested by any package.

**New in this plan's acceptance: for every two packages, one program that is not an acceptance
program**, written in the language against them. For example, a varint dump of a file, or a number
formatter. The balance measure counts it.

**4. A WINDOWS APPLICATION: hamza's decision.** It has been carried unstarted through four
assessments, behind a goal that is package by package. A plan item that never starts is not a plan
item. Either give it a scope and a round, or move it to *deliberately not next* with that reason. This
assessment recommends the second until the packages have a program to put a window around.

### Deliberately not next

- **The windows and V8 unsigned rungs.** No program needs U on those targets.
- **The JVM's unsigned rung,** until a JVM program meets a U value. `tally` on the JVM is the likeliest.
- **Dependent result ranges, a sum in a sign, and `ByteOrder`'s values.** Each waits for a program
  refused by one.
- **Modular arithmetic as a type (ℤ/2ⁿ).** `hash/fnv` and `hash/crc64` compute modulo 2⁶⁴. The
  language can say `(% (* h p) 2^64)`, but its intermediate is 128 bits and so `big`. That is a real
  wall, and the one a hash package will hit. It is named here so it is met on purpose.
- **Rewriting `integers.md` and `tables.md` wholesale.** Their notes are accurate. The rewrite happens
  when either spec is next touched.
- **Records, `with`, named views; checking the 48,518 generated declarations; re-benchmarking the
  gauntlet.** These are unchanged reasons from 09-17.

## 6. The one sentence

This round answered the question a package asked: what an `int` is. The answer, ℤ realized per target
with portability reported, was derived before it was built and checked as it was built, and the Go
package count went from two to four. But six days produced eight lines of new program, and the next
round's measure has to be code written in the language, not code that decides what the language
means.
