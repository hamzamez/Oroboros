# Assessment: the decided list built or blocked, a wrong answer on windows, and TinyRetroPad's body still owed

2026-11-19. Deliberately **not** an ADR. It follows [2026-11-18](assessment-2026-11-18.md).

Written under the rule (an assessment every ten days or 30 commits) at **one day and 33
commits**, all this session's, while the 11-18 plan and hamza's 2026-11-19 list of decisions to
build were still in progress (hamza: "build everything that is decided … walls you hit, choices I
have to make set aside for me to decide when you finish"). The round wrote **9 ADRs** (0130–0138,
each decided by hamza), **19 result documents**, **2 research documents** (joinjump, sendmessage),
callbacks-sketches.md and the words audit, each result with its ledger.

> **Short answer: the window procedure runs, and of the eight items hamza decided on 11-19 five are
> built, one was refuted before it was built, one is blocked on a wall, and one (with TinyRetroPad's
> body) is not started.**
> - **Before the list** (ADR 0130–0133):
>   - a C call written as C writes it, `_Out_` a result and (pointer, count) one segment;
>   - the window procedure, a λ at WNDCLASSA's field over a cell in scope, with its message loop,
>     on both triples;
>   - `cmp`, a Go-library program, one output on four hosts, which found the JVM's ReadAt
>     reporting a missing file as `other`;
>   - the plant audit;
>   - the JVM's mid-run slowdown, found to be the process's age.
> - **Built**:
>   - ADR 0135, `x ∈ T inout`, emission byte-identical;
>   - the measurements' method, the forms interleaved on four hosts;
>   - windows32 as `cmd/check`'s second tier;
>   - ADR 0138, a remainder by a constant of a natural is `srem`, 0.71× on its loop, a power of
>     two excepted by the corpus's measurement;
>   - ADR 0137, an unread binding's typing claim checked.
> - **Refuted**: ADR 0134's B. The core's `again` is the nearest loop's, so B's copy rebinds it.
>   The derivation lands on A as the fourth tail form widened, a reversal for hamza.
> - **Blocked**: u128 in the language's own operations. It is written over ℤ, and the closed ≠ it
>   named is built, but the unsigned word does not widen into the rung above, on any target.
> - **A wrong answer, found by a measurement and fixed ahead of the plan**: windows' `print_int`
>   printed the low 20 digits of a value above the words (§3.3).
> - **Plan item 4**, a time limit on every program a build test runs: done.
>
> **The margin: 21 : 1 counted the usual way** (§3.1), against 573 : 1 last round. One program was written in the language, num/u128 over ℤ, and
> it is refused.

## 1. What the plans asked for, scored

**The 11-18 plan** (approved 2026-11-19):

| # | item | outcome |
|---|---|---|
| 1 | TinyRetroPad's body in J, module by module | **Not started** (§4) |
| 2 | the callback decisions | tier 2's taken and built (ADR 0133); tier 1's wait on hamza |
| 3 | the capacity facts | waiting on a refusing program (probes-2026-11-19) |
| 4 | a timeout on every program a build test runs | **Done** ([timeouts](../gauntlet/results/timeouts-2026-11-19.md)) |
| 5 | CLAUDE.md as state | kept: every item's line rewritten as it finished |
| 6 | the words audit | first pass done; `inout` kept, `cell(T)` refused (ADR 0135) |

**hamza's decided list** (2026-11-19, in order):

| # | item | outcome |
|---|---|---|
| 1 | ADR 0135, `x ∈ T inout` | **Built** ([inout](../gauntlet/results/inout-2026-11-19.md)): 31 sites respelled, emission byte-identical; a program result's `(buffer(V))` kept, its spelling for hamza |
| 2 | ADR 0134, an `again` after a join point, B against C | **B refuted** ([joinjump](../gauntlet/results/joinjump-2026-11-19.md)); A as the fourth tail form widened, for hamza |
| 3 | the measurements' method | **Built** ([pairs](../gauntlet/results/pairs-2026-11-19.md)): four harnesses, each with a self-test; V8's shared feedback vector found |
| 4 | windows32 as the second tier | **Built** ([tier2](../gauntlet/results/tier2-2026-11-19.md)): `cmd/check -tier2`, windows32's baseline accepted |
| 5 | u128 in the language's own operations | **Blocked** ([u128](../gauntlet/results/u128-2026-11-19.md)): the closed ≠ built; U₆₄ ↪ ℤ owed on four targets |
| 6 | ADR 0138, `srem` by a constant | **Built** ([srem](../gauntlet/results/srem-2026-11-19.md)): measured on the corpus first, which decided an exception, a power of two (1.13× on the limbs' factorial with it); 0.71× on the loop |
| 7 | ADR 0137, an unread binding's typing claim | **Built** ([typing](../gauntlet/results/typing-2026-11-19.md)): a typing mark, measured neutral |
| 8 | ADR 0136's struct forms, with TinyRetroPad | **Not started** |

## 2. What this round established

- **A cell and a buffer are places, and a binder writes the set and the mode.** That holds at a
  parameter. At a result there are two relations, a region moved out and a borrow handed back,
  and J has a word only for the first.
- **Theorem J needs a labelled jump.** The core's `again` is de Bruijn's: the nearest loop's. The
  join point's continuation is the fourth tail form's premise ("runs once, immediately, never
  escapes") applied to a producer. So the question is which walkers take the widened form, not
  where to copy k.
- **A ratio is a measurement in one window.** Each host's harness now interleaves the two forms
  round by round. In JavaScript, two timing loops from one source text shared V8's feedback, and
  the second form read 1.7× faster. A harness needs a symmetric witness as well as a self-test.
- **The rung above the word is entered from the signed word only.** Two programs met this: u128's
  digits and `print_int`'s total form. U₆₄ ↪ ℤ is the next wall of the integer tower.
- **A cost choice between equal functions is data**: `point(b)`, a realization's `where` decided
  on the facts, keeps ADR 0088's rule that a target states its realizations.

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-11-17 | 2026-11-18 | **2026-11-19** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 95,277 | 101,010 | **103,772** |
| of which tests | 28,615 | 29,376 | **29,698** |
| `core` / `emit` / `ir` | 24,595 / 36,907 / 33,775 | 27,138 / 38,241 / 35,631 | **28,410 / 38,920 / 36,442** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 2,746 | 2,756 | **2,884** |
| J outside that count: `emit/natnum.oro` | 901 | 901 | **901** |
| J outside that count: the book's programs (`docs/book/code`) | 3,254 | 3,254 | **3,254** |
| J outside that count: the LLVM layer (`targets/llvm/*.oro`) | 389 | 453 | **455** |
| **at the margin**, counted the usual way | 25 : 1 | 573 : 1 | **21.6 : 1** (2,762 : 128) |
| at the margin, with the J outside the count | 1.9 : 1 | 77 : 1 | **21.2 : 1** (2,762 : 130) |
| overall | 34.7 : 1 | 36.7 : 1 | **36.0 : 1** |
| (`emit` + `ir`) : `core` | 2.87 : 1 | 2.72 : 1 | **2.65 : 1** |
| hand declarations (`targets/*.oro`) | 5,369 | 5,898 | **5,952** |

`python gauntlet/balance.py 7a0608e4 HEAD` reproduces the first eight rows. The J outside the
count is lines of code, counted by the same rule.

**Where the compiler grew** (+2,762):
- `core` +1,272: the window procedure's slot and the λ at a field, the value variant for cells,
  the segments' rewrite, ADR 0135's refusal, ADR 0137's mark;
- `ir` +811: the callback's wrapper and address, componentEnsures, the closed constants' and
  typing checks, ADR 0138's `point`;
- `emit` +679: C calls written as C writes them, the closed ≠, the typing marks' passes.

The language grew by 128 lines. Most of it is `examples/u128/inz.oro`, which is refused, plus
`wndproc-win.oro` and `cmp`. **The margin is twenty-six times better than last round's and still
twenty to one**, and the reason is the same: TinyRetroPad's body, the program this compiler work
is for, is unwritten, now because nearly all of it waits on SendMessageA's instances.

### 3.2 Precision and performance

- **Provability**: **8,987 of 9,041** operations and **950 of 989** loops on the four targets,
  unchanged by every step. **3,057 of 3,067** and **417 of 433** on windows32, pinned for the first
  time.
- **Run time**: ADR 0138's loop went from 446 to 307–322 ms, C's time. Every other program of the
  corpus measured within 0.96–1.03 with the rule as built. Taken over every constant, it cost the
  limbs' factorial 1.13×.
- **Compile time**: 1.04–1.11× in every run, within the gate.
- **The full check is about 27 minutes with `-tier2`**: the default run's 19, the windows32 sweep's
  35 s alone, and its differential's 4½ minutes in parallel.

### 3.3 Soundness: the round's findings

1. **`print_int` printed the low 20 digits of a value above the words** (print-2026-11-19), a
   wrong answer on windows and windows32: 25! printed as 10043330985984000000 with exit 0. It is
   the library's fault: the header claimed ℤ and the body's field held 20 digits. It now stops
   with its reason. Two other routes were tried and measured: the claim of the words' union, a set
   above the word, which pulled the rung into the kept function, and a `where`, which the
   refinement layer cannot prove at a call. Total on ℤ waits on U₆₄ ↪ ℤ.
2. **A false typing claim on an unread binding compiled** (typing-2026-11-19), now refused.
3. **Under `-checked`, Go printed a constant above int64 into an `int`** (u128-2026-11-19 §3). No
   wrong answer came of it, since the host refused the text, but it is a gap in Lemma C. Owed.

### 3.4 Process: six things

1. **The respelling script changed a meaning it did not know.** `(buffer(V))` at a result is not
   `(Array(V) linear)`. The emission step refused two programs, which is what it is for: a respelling
   claimed to be one term was checked as one.
2. **A research document's theorem was proved in a calculus the core is not.** Theorem J assumed a
   labelled jump. It was caught by reading the walker before writing the rewrite, one hour in. A
   derivation must name the calculus its terms are in.
3. **A measurement timed a wrong answer.** The 20000! driver's output was 21 bytes. Reading what
   a timed program printed, before timing it, would have found it at once. It was found only
   because the output was read at all.
4. **Heredocs mangled escapes twice more**: a NUL byte in Java source, a shell loop over vet's
   paths. The rule stands, and it was broken by habit again.
5. **The first corpus did not cover the sites that change.** ADR 0138's first measurement timed
   twelve programs, and none ran on fixed limbs. Yet 301 of the largest changed program's 310 sites
   divided by 2²⁴, the limbs' base. Reading the accepted diff, not the summary, found it. A
   measurement of a rule is checked against where the rule acts: the sites, counted, before the
   programs, timed.
6. **Developing in scratch copies while a check runs worked**: three ADRs were prototyped in copies
   of the tree, so no edit raced the compiler step.

## 4. What is left

- **TinyRetroPad's body**, plan item 1, with ADR 0136's struct forms. Its modules are:
  - the title from the path;
  - the time and date;
  - the status line;
  - find and replace over the text;
  - the file's load and save;
  - the menu's commands as a variant.

  Almost all of it goes through SendMessageA, 44 calls of 25 messages, a dependent product over
  the message number ([sendmessage-research.md](sendmessage-research.md)). Its instances are a
  declaration per message, and they wait on one form, a constant among a C call's arguments
  (a wall). The time and date need ADR 0136's optional C string, and the title needs a C string
  from Win32 into ours (ADR 0091's waiting trigger, GetCommandLineA's).
- **U₆₄ ↪ ℤ**: `big-of` on the unsigned word on four targets, and JavaScript's pair comparison.
  It unblocks u128 and `print_int`'s total form.
- **Set aside for hamza** (§5's first item).
- **Owed, found this round**:
  - an exit into ∅ under a tuple pattern, refused by `CheckJoins` though ADR 0090 makes ∅ compose;
  - Lemma C's constant above the word;
  - the inventory not reading `ir/realize.go` (`point`);
  - `ir/java` and `ir/js`'s two runs without a time limit;
  - `gauntlet/java/One.java`, which does not compile.

## 5. The next plan (for hamza)

**Choices set aside, each with its options in its result document:**
1. **ADR 0134's reversal** ([joinjump](../gauntlet/results/joinjump-2026-11-19.md)):
   - A as the fourth tail form widened (recommended);
   - the labelled `again`;
   - D, keep the refusal.
2. **A lent buffer handed back as a program's result** ([inout](../gauntlet/results/inout-2026-11-19.md)):
   - (a) the host's primed name `(dst' ∈ Array(V) linear)`;
   - (b) no result, the borrow ending at the call;
   - (c) keep `(buffer(V))` there.
3. **The variadic of cells**, `fmt.Scan(a ∈ Array(cell(go.Value)))`: until a program calls `Scan`.
4. **SendMessageA's instances** ([sendmessage-research.md](sendmessage-research.md)):
   - (1) a constant among a C call's arguments, `call SendMessageA(w, EM_EXGETSEL, 0, &cr)`, a
     declaration per message (recommended, the only option the memory model admits);
   - (2) a wrapper over the erased declaration;
   - (3) an address cast.
5. **Tier 1 callbacks**: hamza will come back to it.

**Proposed plan**, after those:
1. **TinyRetroPad's body in J**, the modules that need no new design first: time and date, find
   and replace over a byte table, the file's load and save, the menu's commands as a variant. Then
   the title and the status line, after their two designs.
2. **U₆₄ ↪ ℤ**, four realizations, so u128 runs everywhere and `print_int` is total.
3. **The gauntlet re-measured with the pairs**, from a foreground terminal, saying which.
4. **The owed list above**, each small: ∅ under a tuple pattern, Lemma C's constant, the
   inventory's sources.

### Deliberately not next

- **ADR 0134's built form**, until hamza takes the reversal.
- **Callbacks tier 1**, ARM, Linux and macOS, the postponed big-integer list.
- **Anything in the earlier lists' "deliberately not next"**, which stands.

## 6. The one sentence

The decisions the language owed were built where the algebra allowed and refused where it did
not, and the program they were for is the next thing to write.
