# Assessment: the round the declarations became theories, and the analysis outgrew the language

2026-09-17. Deliberately **not** an ADR. The previous seven are
[2026-09-13](assessment-2026-09-13.md), [2026-09-11](assessment-2026-09-11.md),
[2026-09-09](assessment-2026-09-09.md), [2026-09-06](assessment-2026-09-06.md),
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

Written on hamza's request after array-facts.md's plan finished. Four days and 33 commits since the last
one, and 23 result documents.

> **Short answer: yes on the method, yes on the thesis, and NO on the balance, by the widest margin
> recorded.**
>
> **The compiler grew 7,892 lines and the programs written in the language grew 3.** Counted as every
> assessment counts it, that is **2,631 : 1 at the margin**, against 4.4 : 1 last round. Counting the
> hand-written host declarations and acceptance programs too, which ADR 0022 made real Oroboros, it is
> still **19.5 : 1**. `emit : core` went **4.03 → 4.42**, and the analysis layer grew **5,410 → 8,727**
> lines. Most of that growth came in the last twenty hours.
>
> **What the growth bought is real, and every piece was checked.** Declarations are theories now, built in
> eight steps with byte-identical emission throughout. A table's contents are proven, not clamped: all
> fifteen value and node-index clamps are gone from freq, tally and tree, 296 propagated obligations were
> turned into proofs, and the limb library got 2.2× faster as a side effect. Every theorem came with a
> harness or a witness that failed against a planted bug.
>
> **And it cost something nobody measured.** Compiling the tokeniser takes **108 ms → 710 ms**, jsonfmt
> 91 → 196 ms, freq **4.2 → 6.9 s**. Two consecutive results each said *no slower*, and each was true
> against the commit before it. Neither measured against the commit before the regression.
>
> **The goal hamza set is where it was on 2026-09-14**: two Go standard-library packages declared by hand,
> of about 167.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | Have each host check every generated declaration; the witness names structval's 585 | **Superseded, not done.** [ADR 0022](decisions/0022-host-declarations-are-written-by-hand.md) (decided 2026-09-14) makes a supported package's declarations **hand-written**, with the generator as their **checker** (`TestHandDeclarationsAgreeWithTheHost`, six planted mistakes caught). That checks the declarations programs use. The 48,518 generated ones behind the survey percentages are still unchecked, and the 585 witness was never built |
| 2 | An ADR for *a generator does not make a claim it cannot justify* | **Done** — [ADR 0023](decisions/0023-a-generator-does-not-make-a-claim-it-cannot-justify.md) |
| 3 | A Windows application through generated declarations | **Not started** |

**One of three.** The plan did not fail. It was redirected, legitimately, by hamza's standing goal of
supporting Go's standard library package by package, which was written down with the utf8 result on the
last assessment's own day. The work then went in three phases, each following a demand from the one
before:

| phase | dates | commits | compiler (with tests) | Oroboros in `examples/` + `lib/` | what drove it |
|---|---|---|---:|---:|---|
| **A. Host packages** | 09-13 → 09-14 | 5 | +1,114 | +12 | `unicode/utf8` and `encoding/hex` by hand; the wall was the memory model |
| **B. Declarations are theories** | 09-15 → 09-16 13:00 | 20 | +3,606 | −3 | hand declarations need owned types, companions, constants, manifest types, `D_T` |
| **C. A table's contents** | 09-16 13:00 → 09-17 09:00 | 8 | +3,172 | −6 | tally's comparator finding, then the fifteen clamps |

## 2. What this round established

**A declaration's claims beyond its types need someone who read the host's source.** host-buffers.md found
49% of the readable Go slice parameters destructive, and a soundness hole: linearity was keyed by syntax
while it is a property of the type. So a buffer returned by a primitive was never checked. ADR 0022 is the
consequence, and `unicode/utf8` and `encoding/hex` are the evidence. hex found that **the refinement layer
had never looked inside a host call with several results**, so every file-reading program's body had gone
unchecked. It also found a real out-of-range read in `jsonfmt`.

**Declarations are theories, and the claim survived being built.** theories-b-or-c.md §9's eight-step
build order is essentially done. The loader glues forms, target files are respelled, `variant` and `tuple`
exist, a variant is its qualified declaration, `option` is one declaration, a type is a member of its
module, companions derive subtyping, a constant's name is a range endpoint, manifest types unfold by δ,
`D_T` makes tally one program, `lang`'s facts are declarations, and a length is an `ensures`. **Every step
kept emission byte-identical except where a change was intended and recorded with its reason.** That was
possible because `cmd/check` was built first (9a8a590) and made byte identity a gate rather than a
habit. It is the best process investment of the round.

**A value clamp is a missing proof, and the proof is derivable.** The chain was:
- **Inductive invariants.** Houdini over templates finds the greatest set of `0 ≤ v` facts that every back edge preserves.
- **Index terms read through joins.** A `let` or conditional index is proven branch by branch.
- **Goals split on conditionals.** Case-of-case applied inside the logic.
- **Loop-result summaries, and entailment by Fourier–Motzkin.** A fact about a loop's value comes from its exits.
- **F-D₁.** Facts about a table's contents, proven by induction on its stores.
- **Array smashing** (Blanchet et al. 2003), with two theorems for its extensive weak update.
- **Component facts.** Currying, with residues computed by a ring homomorphism.

Propagated obligations went **298 → 2**, and operations proven **2,307 → 2,361 of 2,413**. array-facts.md
§1.2's fifteen clamps are gone and tree builds without `-checked`.

**Checking kept pace with the analysis, which is the counterweight the analysis layer has always needed:**
- 3,000 generated programs for smashing, with anti-vacuity measured (2,006 bounded only through smashing);
- 15 plants for smashing, each caught;
- 4 plants for component facts;
- 4,000 enumerated systems checking Fourier–Motzkin.

The harness also found a real soundness bug during the build (seed 315).

## 3. Are we on the right track

### 3.1 The balance of effort: the worst recorded

Counted exactly as before, and reproducing the last two assessments' figures at their revisions (`af28756`:
37,037 / 1,382 / 5,175 / 4,559; `e820a05`: 37,611 / 1,513 / 5,217 / 6,592). The compiler is `.go` in `core/`
and `emit/`, tests included. Oroboros is non-blank, non-comment `.oro` in `examples/` and `lib/`. The
analysis layer is `interval`, `refine`, `linear` and `monotone`, as first defined. The extended row adds
the files this line of work created: `fact`, `content`, `smash` and `component`.

| | 2026-09-09 | 2026-09-11 | 2026-09-13 | **2026-09-17** |
|---|---:|---:|---:|---:|
| compiler (with tests) | 35,963 | 37,037 | 37,611 | **45,503** |
| of which tests | | 12,115 | 12,384 | **15,041** |
| Oroboros written | 1,324 | 1,382 | 1,513 | **1,516** |
| **at the margin** | 11.5 : 1 | 18.5 : 1 | 4.4 : 1 | **2,631 : 1** |
| overall | 27.1 : 1 | 26.8 : 1 | 24.9 : 1 | **30.0 : 1** |
| `emit` : `core` | 3.98 : 1 | 4.13 : 1 | 4.03 : 1 | **4.42 : 1** |
| analysis layer | 5,175 | 5,175 | 5,217 | **6,935** |
| analysis layer, extended | | 5,368 | 5,410 | **8,727** |
| largest program | 155 | 154 | 154 | **163** |
| survey tooling | 1,511 | 4,559 | 6,592 | **7,150** |
| hand declarations (`targets/*.oro`) | | 2,944 | 2,951 | **3,223** |
| acceptance programs | | | 99 | **229** |

**The strict ratio is not a fair picture of the language, and the fair one is still bad.**
- `examples/` + `lib/` show **+253 / −198** raw lines. Tally's two bindings became one program, and 15
  clamps with their comments came out.
- **The largest program grew** (freq, +9: five invariants over its permutation, stated for hex's checks).
- Under ADR 0022, hand declarations are written in Oroboros by someone who read the host: **+272**.
- Acceptance programs added **+130**: `unicode-utf8.oro` and `encoding-hex.oro` print the host's output.
- With those counted, the margin is **7,892 : 405 = 19.5 : 1**, worse than every round since 2026-09-06.

**Of the compiler's 7,892, 2,657 are tests.** The largest non-test growth by file:

| file | net | |
|---|---:|---|
| `emit/refine.go` | +771 | joins, splits, summaries, invariants |
| `emit/fact.go` | +612 | `lang`'s facts, local theory extensions |
| `emit/content.go` | +520 | F-D₁ |
| `emit/interval.go` | +443 | smashing hooks |
| `emit/linear.go` | +439 | Fourier–Motzkin, entailment caches |
| `emit/target.go` | +437 | the loader's glue |
| `emit/constend.go` | +260 | constants as endpoints |
| `emit/linearity.go` | +259 | seeded by type |
| `emit/smash.go` | +248 | |
| `emit/component.go` | +219 | |
| `emit/companion.go`, `emit/alias.go` | +204, +201 | |
| `core/` (reduce, read, sum) | +495 | variants, tuples, resolution |

**`core` grew too (+910), and `emit` grew by 6,982.** The core's term language did not change: seven term
kinds and three reduction rules, as before.

### 3.2 The analysis layer: the risk named in five assessments, now at its largest

It was 5,175 lines for three rounds and **8,727 extended today**. 2,424 of those lines came in phase C's
twenty hours. The argument every earlier assessment made still applies: *the part of the compiler that
decides whether a program is legal is the part hardest to check, and it is where every round adds.*

Two things are different this time, one in each direction:
- **Better.** The checking is the strongest it has been (§2): random harnesses with anti-vacuity measured,
  planted bugs that must fail, and exact witnesses for what random search cannot reach.
- **Worse.** Phase C was driven by **no new program**. Its demand was clamps programmers had written in
  three existing programs, and removing them bought:
  - **54** proven operations, 2.2% of the corpus;
  - tree at **1.06×** of hand-written unclamped Go, where the clamped form's gap was inside the noise floor;
  - the limb library **2.2×** faster, as a side effect.

  "Prove, don't clamp" is the right rule, and nothing here was wrong. But the *next* clamp will be met in
  a program that does not exist yet, and a proof built ahead of its program cannot be priced.

### 3.3 Compile time: a regression nobody was measuring

Measured for this assessment, per program, on binaries built at each commit. The numbers are the
second, warm run:

| commit | tokenize | jsonfmt | freq |
|---|---:|---:|---:|
| `e820a05` (last assessment) | 108 ms | 91 ms | 4.2 s |
| `80386e1` (goals split) | 117 ms | 111 ms | 4.2 s |
| **`7e36002`** (loop summaries, Fourier–Motzkin) | **1,361 ms** | **243 ms** | **10.0 s** |
| `87d5d37` (F-D₁, entailment caches) | 704 ms | 187 ms | 7.4 s |
| `c2cc351` (HEAD) | 777 ms | 212 ms | 7.0 s |

**The tokeniser compiles 6.6× slower than at the last assessment, jsonfmt 2.2× and freq 1.7×.** tree is
1.3×, though its old build needed `-checked`, so that comparison is not like for like.

**Two results each claimed no slowdown, and each claim was true only against the previous commit:**
- loopsum-2026-09-16 measured the **emission sweep**, which runs 456 compilations in parallel and so
  hid a 12× jump on one program. It said *"the sweep's time does not move"*.
- fdrefine-2026-09-16 measured freq **against loopsum's HEAD**, *"16.0 → 9.6 s, below HEAD's 10.7"*, so
  a fix to part of the regression was reported as a gain.

This is a ratchet, and the method error has a name now: **comparing against HEAD after a regression has
landed**. `cmd/check` gates emitted code and proof counts, but not compile time.

### 3.4 The thesis: unchanged, and the Go number is now two packages deep

The survey figures did not move this round (Go 60.4% usable, the JVM 60.9%, Win32 31.0% callable and
linkable). What moved is **what "supported" means**. ADR 0022 says a package is supported when its
declarations are written by hand, checked against the host, and exercised by a program that prints the
host's answer. By that measure the answer is **2 of about 167 Go packages**: `unicode/utf8` and
`encoding/hex`, plus `io`'s three interfaces with companions. No package has been added since 2026-09-14.

### 3.5 The gauntlet

Last benchmarked 2026-09-07. This round's accepted emission changes touched:
- `limbs`, `fact-limbs`, `render` and `big-divmod` on windows;
- `limb-subdiv`;
- `freq`;
- `tree`, whose benchmark file `gen_jsontree.go` was regenerated and re-measured in compfacts-2026-09-17.

Every other gauntlet program's emitted text is byte-identical under `cmd/check`, so parity holds by
construction. The full check passes at HEAD: vet, compiler, emission, differential and tooling, 6 m 19 s.

### 3.6 Process: four things

**1. THE ROUND BUILT FOR PROGRAMS THAT ALREADY EXISTED.** Phase A met walls in two real host packages and
answered them. Phase B's steps each trace to a hand-declaration need. Phase C traces to three programs
written before the round began. Not one line of a new program was written after 2026-09-14. That is the
failure four assessments named as *"write something awkward"*, recurring in the analysis layer's favour.
**The standing goal is package by package, and the package count is the measure this round did not
move.**

**2. COMPILE TIME WAS UNGATED, AND A 6.6× REGRESSION PASSED TWO "NO SLOWER" CLAIMS.** §3.3. The rule "kept
only if correct and no slower" was applied to the emitted code, which `cmd/check` measures, and not to
the compiler, which nothing measures.

**3. `CLAUDE.md` IS A CHANGELOG LOADED INTO EVERY SESSION.**
- **60,603 words**, against 51,995 four days ago and 48,761 a week ago; this round added 8,608.
- It contains **the same section twice** (*"AND THE TREE IS BUILT TOO"*).
- The file's own convention says *"keep documents current rather than accumulating drafts"*, and the file
  most read is the one that does not.
- This session was compacted twice. The instructions are a large, fixed share of every context window.

**4. STATUS LINES WENT STALE, AND ONE OF MY OWN ANSWERS WAS WRONG.**
- `spec/theories.md` and `theories.md` said *"nothing built"* after eight build steps.
- `spec/data.md` said *"not built under these names"* after `variant` and `tuple` shipped.
- `array-facts.md` said *"no decision"* after F-D₁ shipped.
- All four are corrected in this commit.
- And an hour before this assessment, asked what was left, I described `kara/core`'s unproven operations as
  *"increments by products inside nested loops"*, and left out `limb-subdiv` and `render`. The
  outcomes file says otherwise (§4). That is the "classification by memory" error
  array-facts.md §1.1 already recorded once, and it is why §4 is read off `gauntlet/check/outcomes.txt`.

## 4. What is left, read off the outcomes

**52 integer operations are unproven:**

| | ops | what |
|---|---:|---|
| `shift-div` (differential) | 28 | deliberately unbounded dividends |
| `examples/int/` | 8 | collatz, fib, power: meant to be refused |
| `render`, `limb-subdiv` (differential, built `-checked`) | 8 | one operation each per target |
| `kara/core` | 3 | `(* acc 100)` and `(+ … (p i))`: a Horner accumulator with no bound, plus one read |
| `map/dynamic`, `wordcount` ×2 | 4 | a map's value range |
| `match/runs` | 1 | a counter in a `match` loop with no trip bound |

**37 loops are unproven**, a number that has not moved all round (345 of 382). They include:
- freq's merge sort, 4 × 3 hosts: the level-walk μ = ⌈log₂(n/w)⌉ recorded in freq-2026-09-08;
- **11 JavaScript-native gauntlet sources** that count zero integer operations, because `js.+` returns
  `any`. That is bigrep-2026-09-02's vacuity on the native layer rather than the language's. It produces
  notes, not wrong answers;
- `power` on the bignum rung;
- `collatz`;
- `match/runs`.

**Unbuilt from this round's own documents:**
- F-D₁ declared, and per-component interval cells: nothing asks for either;
- the loader's program half: `const` and manifest types in a program module;
- records, symbols and `with`;
- named views;
- generated types named by module;
- interface methods in the generated surveys.

## 5. What is next

**1. RESUME THE GO STANDARD LIBRARY, PACKAGE BY PACKAGE, AND LET THE PACKAGES SET THE ANALYSIS WORK.**
The next two should hit walls the corpus has not:
- **`encoding/binary`.** `PutUvarint` is a write-borrow returning a count. `Uvarint` returns
  `(value, n)`, and `n ≤ 0` is an error encoded in the payload's own value space: a niche, per
  sums-research.md. Its byte order is an algebra of bijections.
- **`strconv`.** The everyday fallible package, where every parse has a result range and an error.

Each package follows ADR 0022:
- the whole package declared by hand, opening with its laws;
- the checker agreeing with the host in both directions;
- an acceptance program printing the host's answer.

**The rule for the analysis layer: it grows only when one of these programs is refused, and the refusal
is named first.** Acceptance for the round is **packages supported**, not operations proven.

**2. GATE COMPILE TIME IN `cmd/check`, AND DECIDE WHETHER TO BUY BACK THE 6.6×.**
- Record each source's compile time in the outcomes, and flag a change above the noise floor against the
  **baseline**, never against HEAD.
- Then profile tokenize at HEAD. At the last assessment it compiled in about 110 ms, and a program that
  small taking 700 ms points at an asymptotic cost (Houdini rounds, or Fourier–Motzkin elimination
  sizes) rather than a constant one.
- **Acceptance:** with a baseline taken at `80386e1` and the compiler at HEAD, the gate names tokenize,
  jsonfmt and freq. A gate that cannot see the regression this assessment found proves nothing.

**3. REWRITE `CLAUDE.md` AS A STATE, NOT A HISTORY** — *with hamza's agreement first*, since it is the
instruction file. What it must keep:
- current standing;
- the decisions table;
- the constraints that override instinct;
- the working conventions;
- the build commands;
- a one-line index into `gauntlet/results/` and the assessments.

The narrative already lives in the result documents it quotes. Delete the duplicated section in any case.

**4. A WINDOWS APPLICATION**, carried from the last assessment and still unstarted. It is behind the Go
packages because hamza's goal is package by package.

### Deliberately not next

- **F-D₁ declared, and per-component interval cells.** No refused program needs them (§4).
- **`kara/core`'s accumulator, `match/runs`, and a map's value range.** These wait for a package that needs
  them.
- **Loop termination** (the merge sort's level walk, JavaScript-native vacuity). These are notes, not
  wrong answers.
- **Records, `with`, named views, and the loader's program half.** These wait for a program module that
  needs one.
- **Checking the 48,518 generated declarations.** ADR 0022 made hand declarations the supported path; the
  surveys' percentages are measurements of reach, not of what ships.
- **Re-benchmarking the gauntlet.** Its emitted text is identical except tree, which was re-measured.

## 6. The one sentence

This round built the declaration system the hand-written host packages needed, and then proved away
every clamp in the corpus, with checking as strong as any the repository has had. But it did so by
growing the compiler 7,892 lines against 3 of new program, slowing the tokeniser's compile 6.6×
unnoticed, and leaving the Go standard library at the two packages it reached on the round's second day.
So the next round should write packages and let them demand the analysis, gate compile time, and shrink
the instruction file back to a state.
