# Assessment: the 11-15 plan finished but for its last three items, and the book's wrong answers

2026-11-16. Deliberately **not** an ADR. The previous seventeen are [2026-11-15](assessment-2026-11-15.md),
[2026-10-24](assessment-2026-10-24.md), [2026-10-21](assessment-2026-10-21.md),
[2026-10-20](assessment-2026-10-20.md), [2026-10-14](assessment-2026-10-14.md),
[2026-10-03](assessment-2026-10-03.md), [2026-09-28](assessment-2026-09-28.md),
[2026-09-24](assessment-2026-09-24.md), [2026-09-23](assessment-2026-09-23.md),
[2026-09-17](assessment-2026-09-17.md), [2026-09-13](assessment-2026-09-13.md),
[2026-09-11](assessment-2026-09-11.md), [2026-09-09](assessment-2026-09-09.md),
[2026-09-06](assessment-2026-09-06.md), [2026-08-20](assessment-2026-08-20.md),
[2026-08-19](assessment-2026-08-19.md) and [2026-08-13](assessment-2026-08-13.md).

Written under the rule the last one set: an assessment every ten days or 30 commits. **One day, 29
commits** (19 of them the book's chapters, written by a second session), **6 ADRs** (0113–0118), **10
result documents**, each with its ledger.

> **Short answer: the plan's measurable items are done, and the book has found four wrong answers
> that are still open.**
> - Done: P8, the gauntlet benchmarked on four targets (parity on every one), the harness, the file
>   programs on windows (faster than Go by hand), and the book's first 29 findings. Two unplanned
>   steps followed at hamza's question: the LLVM layer's last templates went, so every host
>   template is now a single instruction.
> - Not started: the second triple, TinyRetroPad, and CLAUDE.md as state.
> - **The book's chapters 9–14 found four accepted programs that compute a wrong answer**, and they
>   reproduce today (§3.3):
>   - two payloads bound under one name;
>   - a store past a table's end, which on windows writes past the block;
>   - `len` of a map 12 on windows where it is 3;
>   - a tuple read from a table printed as a map's comma-ok.
>
>   Every check passed while they were open. They come first in the next plan, ahead of the second
>   triple I proposed an hour before this was written.
>
> **The margin, counted the usual way, is 407 : 1, and it means nothing this round.** The language
> written this round is in the book (2,221 lines of J) and in the LLVM layer, which the count does
> not see; with them it is 1.6 : 1 (§3.1).

---

## 1. What the last plan asked for, scored

| | asked | happened |
|---|---|---|
| 0 | **A ledger in every step** | **Kept**: all ten result documents have **Paid and bought**. Two costs are owed, each named: the string terms' compile time and a quiet-machine reading of cabi2 |
| 1 | **P8, with its ledger** | **Done**, steps 1–3 (ADR 0113; p8, p8step3): roundtrip, ledger and freq emit on windows; a verdict is monotone in K; no unproven count rose; run time 0.97–1.03. Owed: `readinto`'s invariant, a segment at a kept boundary, affine templates, instances for U |
| — | *added*: the book's wrong answers, its surface promises, its chapters 5–6 | **Done** (bookfix, surface, chapters56): findings 1–28 fixed, each with a planted fault; 29 deferred |
| 2 | **The gauntlet on four targets** | **Done** (gauntlet-2026-11-16): geometric means Go 1.001, JavaScript 0.965, Java 1.012, windows 0.978 against C; a leak of a build's buffer found and fixed; windows' two walls named |
| 3 | **The harness** | **Done** (harness-2026-11-16): a skip is a checked claim (one found stale); every windows `.ll` assembled; K(ℕ\*)'s text compared; the lowering error joined to its refusal |
| 4 | **The file programs on windows** | **Done** (files-2026-11-16): two crashes past 8 KiB fixed; all five faster than Go by hand, four of five faster than C; `wc`'s 64-bit counter the residue |
| — | *added at hamza's question*: the string templates, then kernel32's | **Done** (ADR 0117, 0118; strings, cabi2): a wrong answer (`string_of` outside the scalars on windows) and three gaps in the checks found and fixed |
| 5 | **The second triple** | **Not started** |
| 6 | **TinyRetroPad** | **Not started** |
| 7 | **CLAUDE.md as state, under 8,000 words** | **Not started**, and it grew from 12,464 to 13,700 words |

**The measure of the round** was three things: P8's ledger closed in measurements, the gauntlet run
on four targets, and TinyRetroPad's surface measured. Two of the three are met; TinyRetroPad's
surface is not.

## 2. What this round established

**A kept definition is a Hoare triple, and keeping is invisible to verdicts** (ADR 0113).
{P ∧ φ} f {ψ}:
- φ is marked at each call and completed by lifting the obligations over the parameters;
- ψ is the greatest subset of templates (Houdini) that holds at every value tail;
- ℤ at an instance is S ∩ W (Proposition D).

Every program accepted with K = ∅ is accepted with the same answer, by P4's net.

**A fold is an equation between pure terms** (ADR 0114). An effectful application is not its value,
so the fold consults the purity bit. The unit is no integer: a one-armed `if` is typed A ∨ 1.

**A string is the table of its UTF-8** (ADR 0117). ρ : Σ\* ≅ V is a monoid isomorphism and the
identity on the representation, and V = φ⁻¹({q₀}) is a submonoid, so `++` is decided structurally.
Agreement with η is proven by enumerating its finite domain, all 1,112,064 scalars.

**A C call is out ∘ F ∘ ⟨t⟩ for a linear term t over the parameters** (ADR 0118). Linearity is
what carries each view to exactly one argument. elem(b, lo) is the action of (ℕ, +) on a region,
partial at len b.

**A table's storage is its final layout** (Theorem D′'s classes, files-2026-11-16), and a dead
build's buffer is released by own(b), its descendants closed by position (gauntlet-2026-11-16).

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-10-24 | 2026-11-15 | **2026-11-16** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 72,967 | 89,868 | **93,533** |
| of which tests | 23,871 | 27,677 | **28,174** |
| `core` / `emit` / `ir` | 19,773 / 33,080 / 20,114 | 23,150 / 35,626 / 31,092 | **23,874 / 36,605 / 33,054** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 2,086 | 2,668 | **2,677** |
| J outside that count: `emit/natnum.oro` | 0 | 867 | **875** |
| J outside that count: the book's programs (`docs/book/code`) | | 334 | **2,555** |
| J outside that count: the LLVM layer (`targets/llvm/*.oro`) | | 249 | **288** |
| **at the margin**, counted the usual way | 20.6 : 1 | 29 : 1 | **407 : 1** (3,665 : 9) |
| at the margin, with the J outside the count | | 11.7 : 1 | **1.6 : 1** (3,665 : 2,278) |
| overall | 35.0 : 1 | 33.7 : 1 | **34.9 : 1** |
| (`emit` + `ir`) : `core` | 2.69 : 1 | 2.88 : 1 | **2.92 : 1** |
| hand declarations (`targets/*.oro`) | 4,518 | 5,011 | **5,095** |

**Where the compiler grew** (+3,810 / −129, of it tests +497):
- P8, about 1,400: `ir/keep.go` 877, `emit/post.go` 275, `emit/hull.go` 125, `core/reduce.go` +203;
- storage, about 380: `ir/release.go` +287, `reuse.go` +95;
- the target and the C ABI, about 350: `emit/target.go` +197, `emit/requires.go` +153.

**The count's blind spot is now the larger part of the language.** At 11-15 it missed natnum's 867
lines. Today it misses the book's 2,555 too: 428 files of J, each run against the compiler on every
check. The book has also been the round's best tester (§3.3). The script is not changed, so that
the series stays comparable. The rows above are its supplement, and the next assessment carries
them.

### 3.2 Precision and performance

- **Provability**: **8,857 of 8,911** operations and **924 of 962** loops, against 7,462 of 7,516
  and 826 of 856 at 11-15.
  - The unproven operations are still 54.
  - The unproven loops rose from 30 to 38, all with P8: the `os` cell's read and write loops in the
    three programs P8 made emit on windows, whose progress is a count the host returns
    (p8-2026-11-15).
- **The gauntlet** (gauntlet-2026-11-16), on four targets for the first time since 09-29: parity on
  every one, each residue above 1.05× explained. Windows runs five of the seven: a dictionary keyed
  by strings and F64's rendering are walls.
- **The file programs** (files, strings): as processes, all five are faster than Go written by hand.
  As processes they beat C on four of five, partly by a lower floor (no C runtime). In work, each
  toolchain's floor subtracted, against C: roundtrip 0.96, freq 0.83, jsonfmt 0.65 (1.66 before the
  strings step) and count 1.03; `wc` is 1.42, all of it its 64-bit counter.
- **Compile time**: the median is flat. The string terms cost `render` 2.06× and `pe` 1.52×, accepted
  with a reason and owed: large realization terms called as functions.

### 3.3 Soundness: the round's findings

| found | by | state |
|---|---|---|
| Eight accepted programs computing a wrong answer (findings 1–28's: the unit as an integer, `len` of a string, a module's scope, …) | the book, chapters 0–6 | **fixed** (bookfix, chapters56) |
| `hostWrites` never read a `branch` arm | P8 step 3 | **fixed** |
| A build's buffer never released (18 KiB a call in the JSON tree) | the gauntlet's peak working set | **fixed** |
| Two file programs crashed past 8 KiB: a table released to a class it was not taken from | running them on windows | **fixed** |
| `string_of` outside the scalars accepted on windows (the book's 48) | the book, and the strings step independently | **fixed** |
| A realization's host `where` never decided (ADR 0089's definedness held for integers alone) | the strings step | **fixed** |
| An identity coercion read as a borrow, so a string freed under its use | the strings step | **fixed** |
| **53/76: two payloads in scope are one name.** `'just(x) … 'just(y) -> 10 * x + y` computes 22 for "42" | the book, chapters 10 and 14 | **open**; reproduced today on Go and windows |
| **70: a store's position is not checked.** `b[n] := 1` compiles; Go panics, windows writes past the block and prints 0 | the book, chapter 13 | **open**; reproduced today |
| **71: `len` of a map is 12 on windows** where it is 3, and the declared capacity is not enforced | the book, chapter 13 | **open**; reproduced today |
| **64: a tuple read from a table** emits a map's comma-ok read of a slice on Go | the book, chapter 12 | **open**; reproduced today |
| 40: a double compared with a rounded decimal crashes the compiler; 39: three comparisons of one double fail the IR's own verifier (W6) | the book, chapter 8 | **open** |
| 78: `go test ./core/ ./cmd/gen/` fails one run in five, a race between two packages' temporary files | the book | **open** |

**Each open wrong answer is one of CLAUDE.md's recurring lessons**:
- 53 is "a context keyed by name is captured by the name alone". The payload's binder is the fixed
  spelling `#p`, which ADR 0036's naming apart does not reach.
- 70 is an obligation that tables.md §6 states and nothing enforces: a store is the update
  f[i ↦ v] of a function on [0, len), defined only on its domain, the same obligation a read has.
- 71 and 64 are representations, a map's count and a table of products, that no program on those
  targets had read before.

### 3.4 Process: five things

**1. THE BOOK FINDS WHAT THE CHECKS DO NOT, AND ITS FINDINGS WERE NOT READ INTO THE PLAN.** Chapters
9–14 recorded findings 48–79 in `docs/book/README.md`, "none fixed", while this session worked on
the string templates and the C ABI. The plan's order was not re-read against them. They were read
only for this assessment, so four wrong answers waited behind a refactoring of templates. The rule
the last round followed by hand ("two items added from the book's findings") was not a rule. **A
wrong answer, wherever it is found, goes ahead of the plan**, and the book's index is read at each
item's end.

**2. A REMEMBERED RULE IS NOT A CHECK.** Python's text mode wrote CRLF on thirteen files, although
the session's memory warned against exactly that. A planted fragment, a two-line source match,
stopped matching. Its witness asserts its own premise ("no longer writes U+FFFD as the test plants
it"), so it failed loudly, not vacuously. `.gitattributes` keeps the repository LF; nothing keeps
the working tree LF, and the tests read the working tree. The fix is mechanical: `cmd/check`
refuses a carriage return in a tracked text file.

**3. A WITNESS LOST ITS PREMISE, AND ONE KEPT IT.** `TestALimbValueRendersOnWindows`'s planted fault
stopped reaching its program when `ascii` became a definition, and the assertion was removed (owed).
The oswin witness checked that its fault was planted and caught the CRLF. **A planted fault should
assert that it reached its rule.** Which witnesses do not is unmeasured, and that is owed as an
audit with a time limit.

**4. THE MACHINE WAS LOADED FOR CABI2'S RUN TIMES.** The last assessment's rule was followed: the
load was noticed and recorded (q3 near twice the median, ratios changing sign between two runs).
There was no quiet machine to take the reading on, so it is owed, not zero.

**5. CLAUDE.md GREW AGAIN**, from 12,464 to 13,700 words, while "CLAUDE.md as state" sat in the plan.
Each step appended its state line, as the rules ask, and nothing removed the history beneath them.
The item stays last in the plan, but it is now measured: under 8,000 words, history to the
assessments.

## 4. What is left

**The book's open findings**, by kind (docs/book/README.md):
- wrong answers or a compiler fault: 53/76, 64, 70, 71, 39, 40; the flaky test 78;
- a promise of j.md that the reader refuses: 60 (`invariant`) and 77 (`try` with several binds),
  both listed as built;
- refusals of programs that mean something:
  - integers and bits: 31–36, 63, 67;
  - numbers: 37, 38, 41–43;
  - claimed constants: 45–46;
  - strings: 49, 50, 52;
  - variants: 54–57, 61;
  - loops: 59;
  - tables: 65, 66, 68;
  - state: 72–74;
- messages: 44, 47, 51, 58, 69, 75, 79.

**Owed from this round**:
- `readinto`'s n ≤ len(b), a relational loop invariant. It is the same thing as 60's `invariant`,
  stated in the source: Hoare's rule, I ∧ g ⇒ I[again's arguments], decided by the refinement layer;
- large string terms called as functions (compile time);
- ι_V as the identity;
- R1e;
- a precision witness for per-loop thresholds;
- cabi2's quiet reading.

**The plan's unfinished items**: the second triple, TinyRetroPad (6c, 6d), CLAUDE.md as state; and
the 10-24 plan's 9, 10 and 12.

**hamza's open questions**: `u128`'s cases against "fully"; the scalar count of a string
(strings.md); concurrency's six; and, at the second triple, the data model (§5 item 4).

## 5. The next plan

**The wrong answers first, then two promises the grammar makes, then the second triple, then the
application.**

1. **The book's wrong answers and faults**, each with a planted fault that fails before its fix is
   trusted:
   - **53/76**: a payload's binder is named apart like every other binder (ADR 0036). Elimination
     binds it fresh, by Barendregt's convention, never under a fixed spelling. Its witness is the
     chapter's two programs, 42 and 22;
   - **70**: a store's index is an obligation. f[i ↦ v] is defined on dom f = [0, len), as
     tables.md §6 says, and is decided like a read's. The off-by-one loop must be refused;
   - **71**: a map's count is |dom m| on every target, and its declared capacity a claim decided
     where the map is written (maps.md §6);
   - **64**: a table of products is the pair of its projections (products.md, currying), so p[i]
     read as (x, y) is (π₁ ∘ p)(i), (π₂ ∘ p)(i), never a map's read;
   - **39, 40**: the crash, and an IR that fails its own verifier. One of lowering or W6 is wrong,
     and the planted fault says which;
   - **78**: the race, since a harness that fails by chance teaches people to rerun it.
2. **Two promises of j.md, built or struck**:
   - **`invariant`** (60): Hoare's rule for `loop`, I decided at entry and preserved by every
     `again`, assumed in the body. It is also `readinto`'s owed invariant, so the clamp in
     `readfile` can go;
   - **`try` with several binds** (77): try a, b { e } = try a { try b { e } }, a derived form in D.
3. **Two checks for this round's process failures**:
   - `cmd/check` refuses a carriage return in a tracked text file;
   - the witnesses that plant a fault by rewriting source assert that the rewrite happened, audited
     within a time limit.
4. **The second triple**, `i686-pc-windows-msvc`, **research first, then hamza's decisions**. What a
   triple changes, beyond the convention ADR 0094 already derives:
   - **the data model**: a C type's set depends on it. `SIZE_T` is 64 bits in LLP64, declared
     0..2⁶³−1 today, and 32 bits in ILP32, 0..2³²−1. So the C type names become sets the
     triple's layer defines. This is ADR 0094's open
     "Why not", and hamza's to decide;
   - **K(ℕ\*)'s base**: B is the largest base whose B² − 1 the triple's double word holds natively.
     That is 2⁶⁴ with i128 on x86-64 (ADR 0106) and, by the same theorem, 2³² with i64 on i686.
     So the base is target data;
   - **`max_len`** from the triple's address space;
   - **`x64.oro`** split by triple;
   - **i32 scalars** (ADR 0083 item 2), whose trigger `wc` met.

   Program first: the suite run on the triple, every failure classified, before anything is built.
5. **φ maximal** (added 2026-11-16, hamza): the facts the decision proves, printed in LLVM's attribute
   lattice where it can express them, one kind at a time, each with its planted fault. With item 4,
   Win32 by header (W1), the DLL each declaration's `lib`.
6. **TinyRetroPad, 6c then 6d**: its Win32 surface measured, then its walls (the window procedure,
   records), spec first.
7. **CLAUDE.md as state**, under 8,000 words.

**The measure of the round**:
- the four wrong answers fixed, each with its witness;
- `invariant` built, and `readfile`'s clamp gone;
- the second triple's research in front of hamza;
- TinyRetroPad's surface measured.

**Kept as rules**, and one new:
- a ledger in every step;
- a refusal test's premise computed;
- an accept that changes a count updates every document quoting it;
- every experiment time-limited;
- the analysis grows only for a refusal a program names;
- **new: a wrong answer, wherever found, goes ahead of the plan.**

### Deliberately not next

- **The book's refusals of meaningful programs and its messages** (§4): each one waits for a
  program that needs it, as the analysis rule says. Their chapters already avoid each one and say
  so.
- **The string terms' compile time and code size** (ADR 0101's called instances for realizations),
  ι_V as the identity, and R1e. The trigger: TinyRetroPad's build, or the second triple's code size.
- **P8's owed items** beyond `readinto`: a segment at a kept boundary, affine templates, instances
  for U, per-call fallback.
- **The kernels, Toom-3, subquadratic division**: decided and postponed, unchanged.
- **The 10-24 plan's 9, 10, 12**: carried.
- **ARM, Linux and macOS targets; concurrency's surface; Android's toolkit**: unchanged.

## 6. The one sentence

The round finished what the last plan could measure (P8, the gauntlet on four targets, the file
programs) and removed the last LLVM text from the targets. Meanwhile the language's own book, the
round's largest body of J, found four wrong answers the checks did not, which the next plan fixes
before anything new.
