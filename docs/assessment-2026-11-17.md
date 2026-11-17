# Assessment: the second triple built to its arithmetic, and φ's first four kinds

2026-11-17. Deliberately **not** an ADR. The previous eighteen are listed in
[2026-11-16](assessment-2026-11-16.md), which this follows.

Written under the rule: an assessment every ten days or 30 commits, here at **one day and 30
commits**. 7 of them are the book's, from its own session. The round wrote **6 ADRs** (0119–0124)
and **15 result documents**, each with its ledger.

> **Short answer: the 11-16 plan's first three items and its last are done, the second triple is
> built to its arithmetic, and φ has four of its kinds.**
> - Done: the book's wrong answers and faults (capture, store, bookfaults, maprealize, capacity),
>   j.md's two promises, the two checks, the claims finding (80/81), and CLAUDE.md as state.
> - **The second triple** (item 4): research, hamza's D1–D7, then four steps.
>   - The family as a fibration (ADR 0122).
>   - `windows32` (D3, the sentinel, W15).
>   - D6, i686's 64-bit division as a runtime in the language (ADR 0123).
>   - D2, K(ℕ\*)'s base per triple (ADR 0124).
>
>   72 of the 74 differential cases pass on i686; the two left are Go-only. Not built: D1 by
>   header, D5 (i32 scalars), and D4's 2-by-1 `divl`.
> - **φ** (item 5): `range`, `samesign`, `nonnull`/`dereferenceable`/`align` and `noalias`, each
>   neutral where measured. One was unsound when first printed (`noalias` on results), and was
>   caught before commit.
> - **One wrong answer, older than the round, found by the second triple** (§3.3): a raw region's
>   zero byte was written at a layout its table did not take, so a path ran into the heap. It
>   reproduced on x86_64. Fixed, with W15.
> - Not started: TinyRetroPad (item 6).
>
> **The margin is 25 : 1 counted the usual way, and 1.9 : 1 with the J the count does not see**
> (§3.1).

---

## 1. What the last plan asked for, scored

| | asked | happened |
|---|---|---|
| 1 | **The book's wrong answers and faults** | **Done** (capture, store, bookfaults, maprealize, capacity; ADR 0119–0121), each with a planted fault |
| 2 | **j.md's two promises** | **Done** (invariant): `invariant I` by Hoare's rule, and `try` with several binds |
| 3 | **Two checks** | **Done** (checks): `endings`, and every planted rewrite through `plantOnce` |
| — | *found*: the book's findings 80/81, a false claim accepted | **Done** (claims), ahead of the plan as the rule says |
| 4 | **The second triple** | **Built to its arithmetic**: research (63/74), D1–D7 decided by hamza, then four steps (ADR 0122–0124; windows32, d6, d2). Now 72/74. **Not built**: D1 by header, D5, D4's `divl` |
| 5 | **φ maximal** | **Four kinds** (phi-range, -samesign, -ptr, -noalias), each neutral where measured; `urem` by a constant at 1.42× C set aside for hamza. **Not built**: `noundef`, `memory(…)`, `willreturn` |
| 6 | **TinyRetroPad** | **Not started** |
| 7 | **CLAUDE.md as state** | **Done**: 7,761 words |

**The round's measure** was the second triple's research brought to decisions and built. That is
met, the parts hamza's decisions named.

## 2. What this round established

- **A triple is a model of what the base leaves open**, and a member is a directory: triple and
  toolchain, data model, address space, ISA, runtime (ADR 0122, 0123). The algebra held at each
  step:
  - provides across glue;
  - the byte bound as Σ_OS's precondition;
  - the sentinel at pointer width.
- **A triple's code generator may leave symbols free, and that is part of its model.** On i686,
  LLVM calls four helpers for 64-bit division. Defining them in the language closes Σ_LLVM there,
  so ι is the same on both triples, and the runtime is 13% faster than libcmt's (ADR 0123).
- **K(ℕ\*)'s base is a theorem about the ISA's word, not a choice.** One library with a parameter
  block serves both triples, and x86_64 is byte-identical. The two-digit lemma had to learn the
  unsigned word's spelling of ℤ's operations, the same integers (ADR 0124).
- **A refinement fact the IR already used was missing from the other route**: a host result's
  declared set (ADR 0123 item 5).

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-11-15 | 2026-11-16 | **2026-11-17** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 89,868 | 93,533 | **95,277** |
| of which tests | 27,677 | 28,174 | **28,615** |
| `core` / `emit` / `ir` | 23,150 / 35,626 / 31,092 | 23,874 / 36,605 / 33,054 | **24,595 / 36,907 / 33,775** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 2,668 | 2,677 | **2,746** |
| J outside that count: `emit/natnum.oro` | 867 | 875 | **901** |
| J outside that count: the book's programs (`docs/book/code`) | 334 | 2,555 | **3,254** |
| J outside that count: the LLVM layer (`targets/llvm/*.oro`) | 249 | 288 | **389** |
| **at the margin**, counted the usual way | 29 : 1 | 407 : 1 | **25 : 1** (1,744 : 69) |
| at the margin, with the J outside the count | 11.7 : 1 | 1.6 : 1 | **1.9 : 1** (1,744 : 895) |
| overall | 33.7 : 1 | 34.9 : 1 | **34.7 : 1** |
| (`emit` + `ir`) : `core` | 2.88 : 1 | 2.92 : 1 | **2.87 : 1** |
| hand declarations (`targets/*.oro`) | 5,011 | 5,095 | **5,369** |

`python gauntlet/balance.py 92f8402 HEAD` reproduces the first eight rows. The J outside the count
is lines of code (no blank or comment line), counted by the same rule.

**Where the compiler grew** (+1,744):
- `core` +721: the capture fix, the invariant and `try`'s binds, the state translation, and
  `where`'s aliases;
- `emit` +302: the second triple's target loading, its runtime, and K(ℕ\*)'s base;
- `ir` +721: φ's attributes, W15, and the lemma on the unsigned word.

### 3.2 Precision and performance

- **Provability**: **8,852 of 8,906** operations and **924 of 962** loops, unchanged since the claims
  fix. No step of the second triple moved them.
- **The second triple**, measured:
  - 64-bit division at 0.87× libcmt's time;
  - K(ℕ\*)'s 20000! at **7.9× Go's `math/big` on 386**, against 3.7× on x86_64. The cause, read off
    the machine code (d2 §4), is the digit step's 64-bit arithmetic: a full 64×64 multiply, 8-byte
    digits and a 64-bit index, where Go's kernel does one `mull` per 4-byte digit. q̂'s division
    through the helper is a few percent.
- **φ**: neutral wherever measured. LLVM had derived the same facts.
- **Compile time**: 1.00–1.05× in every accepting run. The full check is now about 11 minutes, the
  compiler step its longest. Adding windows32 to the differential step would make it 16 minutes;
  that is set aside for hamza.

### 3.3 Soundness: the round's findings

1. **A NUL nobody wrote** (windows32-2026-11-17), a wrong answer on both triples. A table born full
   was asked at w bytes an element and laid out at 1, so its zero byte sat at 8n + 8, and a path
   handed to `CreateFileA` ran into the heap. It was found because i686 made it fail on even path
   lengths. The fix is the derivation b ∈ {1, w}: the allocator writes both zeros, and W15 refuses
   any other layout, with a planted fault.
2. **`noalias` on a result** (phi-noalias): unsound, accepted, then caught by the gauntlet's
   one-line diff before commit. My check of the accepted emission asked that each changed line
   differ by the attribute alone, not where the attribute went.
3. **A compile that does not end is not refused** (d2-2026-11-17). An instance its contract does
   not prove is inlined, and Algorithm D inlined into a program ran for over 20 minutes. It was
   stopped by hand. No rule bounds it.

### 3.4 Process: five things

1. **I guessed at a premise three times before instrumenting it** (d2 §3 item 4). The lesson is
   CLAUDE.md's, "make a suspicious result explain itself". Applied first, it would have saved
   three rounds.
2. **Heredocs and stdin mangled text five times**: backslashes in `"\\01"` and `"\n"`, and a Python
   script read in the console's code page. The rule is CLAUDE.md's, and I broke it by habit. Write
   any script containing a backslash or a non-ASCII character with the Write tool.
3. **The five-target suite was built before its cost was measured**, then reverted. The check's
   time is a cost the ledger names, and a change that triples it is hamza's call. I should have
   asked before building, not after measuring.
4. **I placed D6 myself.** hamza decided the source: (b), the language's algorithm over `divl`. The
   place, the symbols LLVM calls rather than a realization per site, I took as "the algebra
   decides" (ADR 0123's "Why not"). hamza should read that argument; a reversal is a new ADR.
5. **A false alarm was raised and checked before it spread**: an interval that looked unsound was
   the bit reading of a conversion. Reading the IR settled it in one step.

## 4. What is left

- **The second triple**:
  - the 7.9×'s causes (d2 §4): a digit's range on its loads (φ) and its width (D5); D4's 2-by-1
    `divl` for q̂, a few percent;
  - D1 by header (W1);
  - D5, i32 scalars: 2.6× on i686 loops, by the research's measurement;
  - `adc` at 32 bits;
  - the Karatsuba threshold at 32-bit digits;
  - the runtime linked whole (+2,560 bytes a program);
  - windows32 in `cmd/check` (hamza's).
- **φ**: `noundef`, `memory(…)`, `willreturn`/`mustprogress`.
- **TinyRetroPad**: 6c, its surface measured; 6d, its walls.
- **The compile that does not end** (§3.3 item 3): a rule, and its planted fault.
- **Owed from earlier rounds**: unchanged in CLAUDE.md's list.

## 5. The next plan (approved by hamza, 2026-11-17: "Go with the proposed plan")

1. **A compile that does not end is a refusal.** It is a soundness-adjacent gap a program found,
   and it goes ahead of the plan. Either bound the reducer's and the decision's work per program,
   or refuse a program whose needed instance is not decided rather than inline it. Research
   first: which of the two the algebra prefers.
2. **φ's next kind, `!range` on a load**: a table element's proven range stated where it is read.
   This is the first measured cause of windows32's 7.9× (d2 §4), and neutral on x86_64 if it
   behaves as the four kinds did. D4's 2-by-1 `divl` for q̂ follows, a few percent.
3. **D1 by header (W1)** with **TinyRetroPad 6c**: the type names in their headers, then the 67
   functions TinyRetroPad calls, declared by header and checked.
4. **D5, i32 scalars**, its own ADR.
5. **φ's remaining kinds**, one at a time, each a soundness rule with its fault.
6. **TinyRetroPad 6d**: its walls, spec first, an ADR each.

### Deliberately not next

- windows32 in every `cmd/check` run, until hamza weighs its five minutes;
- ARM, Linux and macOS;
- a per-site realization of `/` chosen by facts (ADR 0123's "Why not" keeps it open as a measured
  optimization);
- the Karatsuba threshold at 32 bits, until a program pays for it;
- anything in the 11-16 list's "deliberately not next", which stands.

## 6. The one sentence

The second triple is a model of what the LLVM layer leaves open, and building it found one wrong
answer that had been on x86_64 all along.
