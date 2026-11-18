# Assessment: records whole, host structs by their layout, and three wrong answers before the plan

2026-11-18. Deliberately **not** an ADR. The earlier ones are listed in
[2026-11-16](assessment-2026-11-16.md), and this follows [2026-11-17](assessment-2026-11-17.md).

Written under the rule (an assessment every ten days or 30 commits) at **one day and 26 commits**,
because the 11-17 plan's items 1–5 are finished and item 6 now waits on decisions only hamza can
make. All 26 commits are this session's. The round wrote **5 ADRs** (0125–0129), **16 result
documents** and **3 research documents**, each result with its ledger.

> **Short answer: the 11-17 plan's items 1–5 are done or measured, and its item 6, TinyRetroPad's
> walls, has its first wall built and its second researched.**
> - **Items 1–5.**
>   - The compile ended: its premise was reversed.
>   - φ's `!range` and its remaining kinds were measured and not built.
>   - D1 by header (ADR 0126) and D5's i32 scalars (ADR 0127) were built.
>   - Three allocation steps were added at hamza's word, and windows32's 20000! went from 8.3× to
>     5.5× Go's on 386.
> - **Item 6, the first wall, records and host structs** (ADR 0128, 0129), whole:
>   - the record in the language, a map of records, a buffer of records;
>   - a C struct by its layout, checked against clang on both triples;
>   - determined fields, `_In_`, a struct by value, and a scalar cell at a host's cell.
> - **Item 6, the second wall, callbacks**: researched twice (the window procedure, and tier 1).
>   Eleven decisions are with hamza.
> - **Three faults found and fixed ahead of the plan** (§3.3), one of them a wrong answer on three
>   targets: `-checked` did not trap on JavaScript, nor on Go and the JVM for `<<`.
>
> **The margin is the round's worst: 573 : 1 counted the usual way, 77 : 1 with the J outside the
> count** (§3.1). No program was written in the language this round.

---

## 1. What the last plan asked for, scored

| | asked | happened |
|---|---|---|
| 1 | **A compile that does not end is a refusal** | **Done, its premise reversed** (noend): every loop of the compile has a measure; the 20 minutes were windows32's `div` undecided and inlined. The library is now checked once on every target it serves |
| 2 | **φ's `!range` on a load**, then D4's `divl` | **Measured, not built** (rangeload): LLVM's i686 backend ignores it; the 7.9× was not the digit step |
| — | *added at hamza's word*: the allocation, the written prefix with a tail, a dead region across a call | **Done** (alloc, filltail, reusecall; ADR 0125): 20,000 zeroed allocations became 22 |
| 3 | **D1 by header with TinyRetroPad 6c** | **Done** (tinyretropad, byheader; ADR 0126): 61 declarations in 14 headers, each checked against its prototype |
| 4 | **D5, i32 scalars** | **Done** (d5; ADR 0127): every windows32 program measured at 0.59–0.88× its time before |
| 5 | **φ's remaining kinds** | **Measured, not built** (phi5): `memory(…)` is the join LLVM computes itself; `willreturn` and `noundef` neutral |
| 6 | **TinyRetroPad 6d, its walls** | **The first wall built whole** (records, structs, mapvalue, vcell, sbv; ADR 0128, 0129). **The second researched** (callbacks-research, callbacks-tier1-research), eleven decisions with hamza. Owed: a buffer field with `= len(m)`, a nested record by pointer |

**The round's measure** was TinyRetroPad's first wall built. That is met, but for the two struct
fields that need a dialog to run or a spelling decision.

## 2. What this round established

- **A record is a dependent product, and a host struct is its encoding** (ADR 0128).
  - The language holds the record whole: projection, override along prefix-free paths, and `zero`.
  - The reducer erases it by the lens laws and by currying at each binder.
  - A C struct's layout is Theorem L's fold, derived, and checked against clang on both triples.
- **A map's value of n components is n slots on the IR**, not n maps (mapvalue). Each host holds the
  struct where hand-written code would, and windows a slot of 2 + n cells.
- **A cell parameter is the pair (the value in, the value out)** (vcell). A program's cell fits a
  host's cell of set T iff its value is in T at the call, which is a claim, and T is within the
  cell's set. So cells are invariant only when both directions are asked of the type alone.
- **A struct parameter is its prototype** (ADR 0129). The bare set is the value, passed as the
  triple's ABI passes an aggregate, measured against clang on nine shapes. `const T*` is the call's
  argument term `ref(x)` (ADR 0130, after the assessment, at hamza's question): first written as a
  mode `in`, which no program could observe.
- **A callback during the call is the language's own loop given to a host** (tier-1 research). Its
  markers are a usage in 𝕌 and an order, and an unordered body must be pure (Theorem D). The window
  procedure is a parameterised deep handler of the host's message effect (Theorems N and T).
- **The unit is terminal**: `(() (fn () b)) = b`. Lowering had no rule for it, so windows' `main`
  failed to lower on the interval route (vcell).

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-11-16 | 2026-11-17 | **2026-11-18** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 93,533 | 95,277 | **101,010** |
| of which tests | 28,174 | 28,615 | **29,376** |
| `core` / `emit` / `ir` | 23,874 / 36,605 / 33,054 | 24,595 / 36,907 / 33,775 | **27,138 / 38,241 / 35,631** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 2,677 | 2,746 | **2,756** |
| J outside that count: `emit/natnum.oro` | 875 | 901 | **901** |
| J outside that count: the book's programs (`docs/book/code`) | 2,555 | 3,254 | **3,254** |
| J outside that count: the LLVM layer (`targets/llvm/*.oro`) | 288 | 389 | **453** |
| **at the margin**, counted the usual way | 407 : 1 | 25 : 1 | **573 : 1** (5,733 : 10) |
| at the margin, with the J outside the count | 1.6 : 1 | 1.9 : 1 | **77 : 1** (5,733 : 74) |
| overall | 34.9 : 1 | 34.7 : 1 | **36.7 : 1** |
| (`emit` + `ir`) : `core` | 2.92 : 1 | 2.87 : 1 | **2.72 : 1** |
| hand declarations (`targets/*.oro`) | 5,095 | 5,369 | **5,898** |

`python gauntlet/balance.py c62b34e6 HEAD` reproduces the first eight rows. The J outside the count
is lines of code, counted by the same rule.

**Where the compiler grew** (+5,733):
- `core` +2,543: the record (its former, erasure, currying at every binder), and the cell's claim;
- `ir` +1,856: struct cells, map slots, the checked refusal and Lemma C, the unit, a constant's claim;
- `emit` +1,334: the layout fold, the map-product pass, `in`.

Beside these, `cmd/build`'s tests grew by 714 lines, outside the count.

**This is the worst margin the measure has recorded, and the reason is plain.** The round built a
wall for a program, TinyRetroPad, that is not yet written in the language. Each step had a program,
but each program was a test of a dozen lines. CLAUDE.md's rule, *write programs and let them demand
the analysis*, was met for TinyRetroPad's surface (its 67 imports, measured) and not for its body.
The next plan (§5) writes it.

### 3.2 Precision and performance

- **Provability**: **8,852 of 8,906** operations and **924 of 962** loops, unchanged all round. No
  step moved them, and Lemma C's closed operations are counted apart so as not to.
- **windows32**:
  - 20000!'s product went from 8.3× to **5.5×** Go's on 386 (alloc, filltail, reusecall), the rest
    the 8-byte digit, U₃₂ on i686 (ADR 0127's "Why not");
  - every program measured runs at 0.59–0.88× its time before D5.
- **Compile time**: 1.00–1.05× in every run.
- **The full check is about 19 minutes**, the compiler step 14–16 of them.
- **Run time of this round's constructs**: a record erased costs nothing by construction. A map of
  products, a struct by value and a struct cell are not timed against C: owed with the first program
  that runs one in a loop.

### 3.3 Soundness: the round's findings

1. **`-checked` did not trap** (checked-2026-11-18), the book's finding 93, a wrong answer on three
   targets.
   - The decision trapped an unproven operation only where the target declared a checked form, and
     the build then called the operation a trap.
   - JavaScript declared none, so 3^128 printed 1.18·10⁶¹. Go and the JVM declared none for `<<`, so
     `3 << 69` printed 0 on Go.
   - Now every operation is proven, trapped or closed on the word (Lemma C), or the program is
     refused. Each fault is planted.
2. **A claimed constant nobody read, and an unread call's contract** (deadclaims-2026-11-18): false
   programs that compiled. Both are now decided where they are written.
3. **A case arm's `r.x` was refused as an import** (mapvalue §2): the pattern's binder was renamed
   without its projections. It was a refusal, not a wrong answer, but it stood in any case arm.

### 3.4 Process: five things

1. **I patched before tracing which path runs, three times.**
   - A `case` rule in `resolve` that case expansion never let run.
   - The eraser's map rule inside the branch for a name operator.
   - The cell's claim as an ascription that is erased before lowering.

   Each time, one debug print settled it. assessment-2026-11-17 §3.4's first item, "make a
   suspicious result explain itself", was the same lesson: trace first.
2. **Heredocs mangled escapes three more times** (`\n` in Python inside a bash heredoc). That is the
   last assessment's second item again, broken by habit. The rule stands: any script holding a
   backslash goes through the Write or Edit tool.
3. **A full check stalled for 40 minutes**: two tests each waited 37 minutes on a child process at
   the same moment, and passed alone in seconds. The build tests' runners have no timeout on the
   program they run, so a stall costs the whole check. A timeout per run would bound it.
4. **Two differential cases were bent around a capacity proof** (mapvalue §5). The capacity facts
   refuse two inserts in one iteration, scalars too, on HEAD. Writing the cases in the shape that
   proves was right for the step, and the refusal is now named twice, by the rule's own standard.
5. **CLAUDE.md grew from 7,761 to 8,899 words**, most of it plan item 6's paragraph, which now
   reads as a narrative. CLAUDE.md's own rule is that it states where the project is.

## 4. What is left

- **TinyRetroPad's second wall, callbacks**: eleven decisions with hamza, then the piece shared by
  both researches: a function of ours printed with the host's C convention, its address a value.
- **TinyRetroPad's first wall, its remainder**:
  - a buffer field with `= len(m)`, which needs a spelling for a record field holding a linear
    buffer, hamza's;
  - a nested record by pointer;
  - a C string read back from a struct;
  - a struct returned by value.
- **TinyRetroPad itself**: its body, written in J.
- **Found and owed this round**:
  - the capacity facts' two-inserts shape;
  - `len(m)` unbounded on Go;
  - a header's declared `Map(K, A × B)`;
  - a hull per component of a map's value;
  - an unread binder's typing claim (design).
- **Set aside for hamza, unchanged**: windows32 in `cmd/check`, ADR 0123's placement of D6, `urem`
  by a constant.
- **Owed from earlier rounds**: unchanged in CLAUDE.md's list.

## 5. The next plan (proposed; hamza's to approve)

1. **TinyRetroPad's body in J, module by module, the window procedure last.** The file's reading and
   writing, the status line's rendering, find and replace over the text, the menu's commands as a
   variant. Each module is a program that runs on windows and windows32 and demands what it needs.
   This answers §3.1: the round's walls were built for this program, and it is time it was written.
2. **The callback decisions** (callbacks-research §7, the tier-1 research §7), when hamza takes
   them. Then the shared piece, then the window procedure, which ends item 1.
3. **The capacity facts**: two inserts in one iteration, and an insert in an arm whose guard bounds
   its index. Two cases named it, and so did a scalar program on HEAD.
4. **A timeout on every program a build test runs** (§3.4 item 3), so a stalled child costs one test
   and not the check.
5. **CLAUDE.md as state again** (§3.4 item 5): item 6's paragraph rewritten to what is built and
   what is owed, each with its link.

### Deliberately not next

- **Callbacks tier 1**, until a Go-library program asks for `strings.IndexFunc`, `sort.Slice` or
  `filepath.WalkDir`.
- **A header's declared `Map(K, A × B)`, a struct returned by value, the survey's struct by value,
  a hull per map component**: each waits on a program.
- **A buffer field with `= len(m)`**, until hamza decides how a record field holding a linear buffer
  is spelled, which TinyRetroPad's dialogs will ask.
- **windows32 in every `cmd/check` run**, ARM, Linux and macOS.
- **The postponed big-integer list** (subtractive Karatsuba first): its trigger is the standing
  measure's next round, not this one.
- **Anything in the 11-16 and 11-17 lists' "deliberately not next"**, which stands.

## 6. The one sentence

The program's first wall is built whole and its second is a theorem away from being built, and the
measure says plainly that the program itself is what to write next.
