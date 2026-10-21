# Assessment: two programs, C met as a cost, and the observer that made it unnecessary

2026-10-21. Deliberately **not** an ADR. The previous fourteen are [2026-10-20](assessment-2026-10-20.md),
[2026-10-14](assessment-2026-10-14.md), [2026-10-03](assessment-2026-10-03.md),
[2026-09-28](assessment-2026-09-28.md), [2026-09-24](assessment-2026-09-24.md),
[2026-09-23](assessment-2026-09-23.md), [2026-09-17](assessment-2026-09-17.md),
[2026-09-13](assessment-2026-09-13.md), [2026-09-11](assessment-2026-09-11.md),
[2026-09-09](assessment-2026-09-09.md), [2026-09-06](assessment-2026-09-06.md),
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

Written when the 10-20 plan finished. Two days, **6 commits**, **1 ADR** (0069), 6 result documents.

> **Short answer: the round did what the plan asked, program first, and the balance shows it.** Two
> programs were written by hand in J: `ls`, a recursive listing that brought in `time`, and `shape`, a
> delimited table's shape that brought in `bufio`'s Reader. The language grew by 140 lines against the
> compiler's 928, **6.6 : 1 at the margin**, the best since the IR was built (78 : 1 last round).
>
> **C was met, and it was not a wall.** The plan predicted that `bufio`'s Reader would meet a borrow a
> later call ends. It did, but as a cost: `ReadString` copies each record and `ReadSlice` lends it.
> Measuring the cost separated three factors:
> - the host's vectorized search;
> - the buffer's size;
> - and, mattering least, who owns the buffer.
>
> hamza's question, whether the buffer rules already cover it, led to the answer: ADR 0020's observer,
> extended to a host's read-only parameter, with no new mode (ADR 0069). `shape` reads its own buffer
> at parity with hand-written Go, and `lent` waits for `Peek`.
>
> **What writing by hand found.** Four faults in J's reader, none of which the gate could see:
> - `!=` meant nothing;
> - a partial header crashed D;
> - a binder's claim was missing;
> - the core's ascription was writable and trusted, so a false claim compiled.
>
> It also found two faults in the Go survey (a type alias as a type of its own, the manifests in
> lexical order), and seven analysis gaps, five of them now closed.

---

## 1. What the last assessment asked for, scored

The 10-20 plan was "the Go standard library, package by package, program first, in J", in five items.

| | asked | happened |
|---|---|---|
| 1 | **Two loose ends**: the Node pin, and `lib/result.oro` compiled alone | **Done** (nodepin, libalone). The survey re-measured on v26.8.1, 16 callable members more, no conclusion moved. A library file reached by two roots is read once, a λ argument is no index, an undeclared parameter's reading is named, and lowering's messages render in J |
| 2 | **`os`, program first**: an `ls -lR`, bringing in `time` | **Done** (ls). `examples/os/ls.oro`, 13 of 13 operations proven, identical to a hand-written Go listing on four real trees; `time.oro` (UTC, Format), `os.Lstat`, `FileInfo.ModTime`, `FileMode.String`. Declaring `os.Lstat` found the survey's two faults: Go's usable figure 60.4% → 60.5% |
| 3 | **`bufio`'s Reader, program first, meeting C, researched when met** | **Done, and past the plan** (shape, ownbuf, observer). `shape` written with `ReadString`, then C researched (borrow-research.md), then measured, then avoided by ADR 0069. `shape` now reads its own buffer at parity |
| 4 | **Every program written in J by hand, and what J's reader catches counted** | **Done for two programs.** §3.3 counts it: four reader faults and one checker gap found by writing, none by the gate |
| 5 | **The analysis grows only for a refusal a program names**, each rule with a witness and a planted fault | **Kept, with one slip** (§3.4): ten rules built for `shape`, each load-bearing, each with a fault that makes `shape` refused. Three more were built on the way and removed, because none was load-bearing at the end |

The round in five phases, counted with gauntlet/balance.py (§3.1):

| phase | commits | ADRs | compiler | Oroboros | hand declarations | survey tooling |
|---|---:|---:|---:|---:|---:|---:|
| **A. Loose ends**: nodepin, libalone | 2 | 0 | +126 | 0 | 0 | 0 |
| **B. `ls`**: os, io/fs, time; the survey's aliases and order | 1 | 0 | 0 | +60 | +58 | +190 |
| **C. `shape` with `ReadString`**, and C researched | 1 | 0 | +18 | +58 | +37 | +24 |
| **D. The caller's buffer measured**; D's crash fixed | 1 | 0 | +37 | 0 | 0 | 0 |
| **E. The observer, claims as obligations, `shape` in its own buffer** | 1 | 1 | +747 | +22 | +51 | +110 |

## 2. What this round established

**A Reader enumerates a prefix code's factorization.** For a delimiter d, C_d = (B ∖ {d})*·d is a prefix
code, so the input factors uniquely as c₁ … c_k u. `ReadString` returns exactly the factors, which is
why its law reads "err ≠ nil iff the data does not end in delim". The factorization is lossless and
blind to the buffer. `ReadSlice` gives the same factorization refined by the buffer, each piece lent.
A record's field count is a letter count, a monoid homomorphism, so a fold over pieces is a fold over
records: where the reads cut the input is invisible, which the test checks at the buffer's edges.

**C is a cost, and its factors are separable.** On 260 MB:

| program | median |
|---|---:|
| `ReadString` (a copy per record) | 308 ms |
| `ReadSlice`, 4 KB | 243 ms |
| a per-byte fold over the program's own buffer, by hand | 307 ms |
| `ReadSlice`, 64 KB | 145 ms |
| the program's own 64 KB buffer and the host's search, by hand | 131 ms |

Ownership is the smallest factor of the three.

**A lent value is a value held as a view for an epoch** (borrow-research.md, Theorem L). It is
Wadler's `let!`, Reynolds' passive alias and Rust's `fill_buf`. A naive declaration miscompiles a
program that is correct in source order: β moves a pure copy past the next read. Not built; it waits
for `Peek` and `Scanner.Bytes`, which have no caller-owned form.

**An observer at a host parameter is derived, not declared** (ADR 0069). A pure declaration whose
results are scalars reads a live buffer for the length of the call. Its value is the same as on a
copy, because it writes nothing and returns nothing with an extent. The type checker, linearity and
§7c each read the one derived set. No emitted file changed, and the store-order decisions (G5–G7) are
untouched.

**A claim is an obligation.** `b ∈ S = e` is marked as a declared parameter's range is, then proven or
refused, and only a proven claim reaches the IR, which reads it as the value's set. `the` is not J.

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-10-14 | 2026-10-20 | **2026-10-21** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 66,618 | 70,449 | **71,377** |
| of which tests | 23,396 | 23,206 | **23,532** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 1,696 | 1,869 | **2,009** |
| … in core-term nodes | 9,878 | 10,877 | **11,894** |
| **at the margin** | 64 : 1 | 78 : 1 | **6.6 : 1** (928 : 140) |
| overall | 39.3 : 1 | 37.7 : 1 | **35.5 : 1** |
| `emit` : `core` | 2.10 : 1 | 1.66 : 1 | **1.69 : 1** |
| (`emit` + `ir`) : `core` | 3.41 : 1 | 2.71 : 1 | **2.72 : 1** |
| analysis layer, extended | 5,808 | 5,808 | **6,039** |
| the IR's analysis | 3,342 | 3,342 | **3,342** |
| hand declarations (`targets/*.oro`) | 3,767 | 4,298 | **4,444** |
| survey tooling | 9,432 | 9,509 | **9,833** |

**Where the language grew.**
- `ls`: 60 lines.
- `shape`: 58 lines, then 80 in its own-buffer form, net +22.
- The node count agrees, by each revision's own reader: +1,017 (9%).

**Where the compiler grew.** Phase E is four-fifths of it:
- the refinement layer's rules: the extended analysis layer +197 lines (5,842 → 6,039);
- the observer, `emit/observer.go` and three consumers;
- claims as obligations in the reader, D and the reducer;
- their tests.

The target said the margin "should fall below the compiler's growth"; it fell from 78 : 1 to 6.6 : 1,
and the compiler still grew faster. The direction is right; parity at the margin is not reached.

### 3.2 Precision and performance

**Provability**: 2,065 of 2,110 operations and 353 of 374 loops. **45 operations and 21 loops unproven**:
last round's 45 and 19, plus one loop each in `ls` and `shape`, which are driven by the file system and
by the input. `ls` added 13 operations and `shape` 13, all proven.

**Performance.** The gauntlet was not re-run, since no printer changed and emission was byte-identical
in every step but the two new programs. `shape` is at parity with its hand-written form (1.11×, inside
the noise floor) and with `ReadSlice` at the same size (1.00×).

**Compile time** is flat, except `shape` itself: 1.2 s serial. About three-quarters of that is Houdini
over a 12-variable loop, where the pairwise template grows quadratically. It was accepted with that
reason recorded.

### 3.3 Soundness: the round's findings

| found | lived | how it was found |
|---|---|---|
| **The core's ascription was writable in J and trusted**: `the("int 0 5", 10)` compiled; so did `x ∈ 0..5 = 10` once the binder's claim was re-added | since today's reader was deleted (`the`); hours (`b ∈ S = e`) | writing `shape`'s claims, then testing a false one before trusting a true one |
| **A partial header crashed D**: `fn run(s ∈ 0..255) { s + 1 }` | since J's reader | a scratch probe while bisecting another refusal |
| **J's `!=` was a name nothing binds** | since J's reader | `shape`'s first build |
| **A binder's claim, `b ∈ S = e`, was specified and not read** | since today's reader was deleted | needing one |
| **The Go survey ignored type aliases, and read the manifests in lexical order**, so go1.txt overrode every later spelling | since the survey | declaring `os.Lstat` and seeing it refused by the checker |
| **A naive lent declaration miscompiles a correct program** (β moves a pure copy past the next read) | never shipped | an experiment before declaring `ReadSlice` |
| **A definition's declared result range inside the word is not proven**: `fn bad(x ∈ 0..100) (0..5) { x + 10 }` prints 11 | since ADR 0028 | probing whether a claim was trusted; confirmed at HEAD in a worktree. Not a miscompilation: the claim is dropped at inlining and nothing reads it. **Open** (ADR 0062's logic) |
| **β substitutes a pure application used once into a loop body**, where it runs each iteration | since call-by-need | reading `shape`'s emitted Go. **Open** |
| **Houdini loses 0 ≤ c** (ownbuf reproducer `e2`) | unknown | writing the caller-owned form. **Open**: the facts at the failing edge are contradictory, and the stated cause did not explain it (§3.4) |

The round's lesson continues the last one: **every reader fault was found by writing a program by
hand, and none by the gate**, because the gate's corpus is well formed. Writing programs is how the
reader gets tested.

### 3.4 Process: four things

**1. A DIAGNOSIS WAS BUILT ON BEFORE IT WAS VERIFIED.** `e2`'s cause was stated as "Fourier–Motzkin
gives up past 160 facts". Two rules were built on it, nested subsets and ex falso, and neither fixes
`e2`. This was caught only by faulting every rule at the end, and both were removed. **Plant a rule's
fault when the rule is written, against the program it is for**, not after the program compiles.

**2. THE FIRST RECOMMENDATION WAS THE EXPENSIVE ONE.** borrow-research.md first recommended a mode
`lent`. hamza asked whether the buffer rules already cover it, and the answer, ADR 0020's observer, was
cheaper, more general, and already in the project's own documents (host-buffers.md §4, ADR 0020's
"read-borrow is already free"). **Before proposing a construct, ask whether an existing one,
dualized or extended, already is it.**

**3. AN UNBOUNDED EXPERIMENT RAN ON hamza'S MACHINE.** The Omega test as a fallback ran for 7 minutes
and ran out of memory. **Every experiment gets a time limit.**

**4. A SHELL COMMAND UNDID A REAL EDIT.** `git checkout` used to remove one debug line reverted a whole
file's change, which had to be redone. Debug edits are made on a copy, or removed by an edit, never by
checking out the file.

## 4. What is left, read off the outcomes

**45 unproven operations and 21 unproven loops.**

**Decided and not built**: `lent` (borrow-research.md, waiting for `Peek`); the arrow and callbacks tier
1 (ADR 0053); the logic of 0062; the inference of 0066; step 2 of types as values (0063).

**Found and named this round**:
- a definition's declared result range inside the word is not proven (§3.3);
- β substitutes a once-used pure application into a loop body (§3.3);
- the clause chain is copied under every failed conjunct: a six-arm loop emitted 2,423 lines of Go,
  and its proof count was inflated (ownbuf §1);
- Houdini losing 0 ≤ c (`e2`);
- a boolean's definition is not a fact on its arms (bufio-2026-10-01 §3.4, met again by `shape`);
- on the bignum path a claim does not reach the IR's word selection;
- a message shows the compiler's binder names (`#nl4[#nl5 - 1]`).

**Carried**: ℤ/2ⁿ; an unsatisfiable contract; a join across representations; a host `where` on the
IR's intervals; freq's `slen`; kara's read over write.

## 5. The next plan

**Two defects that touch every program, then the library resumes.** Both are measured, both change
emission, and both were found by this round's programs, so they come first, each its own step behind
the byte-identical gate.

1. **Work stays where it was written.** β's one-occurrence rule substitutes a pure application into a
   loop body, so a conversion written once runs per iteration. GHC's occurrence analysis separates the
   case: once used *inside a lambda* is not once used. Research it as the rule says (what it is, where
   the literature settled it), build it, and measure the corpus's emission and the gauntlet's times. It
   is expected to change many files and slow none.
2. **A declared result range is an obligation.** The mechanism is now built for binder claims: the
   definition's result marked at its tails, proven or refused, then read by the IR. This is the first
   piece of ADR 0062's logic. `bad` must be refused, and the corpus may name results whose claims are
   false; each such result is a finding, not a change to accept.
3. **The Go library, program first.** A program over files and text that brings in more of `os` (118
   names declarable) and `io` (27), and `unicode`, a package with no file. A `cmp`-shaped tool
   comparing two files chunk by chunk reads two buffers with the observer; a `wc -m` counts runes. Each
   is written by hand in J, and what J's reader catches is counted.
4. **The clause chain's copying, researched when a program's size or compile names it.** It is a join
   point (bind a failed guard's remainder once), and it would shrink `shape`'s Go by roughly an order
   of magnitude. It waits for a program that pays for it.
5. **The analysis grows only for a refusal a program names**, and every rule gets its planted fault
   when it is written (§3.4).

**The measure of the round**: the margin below 6.6 : 1, and no reader fault left for a program to find
that a refusal test could have.

**Kept as rules**: a refusal test states its reason and is checked for it; an `-accept` that changes a
count updates every document that quotes it; every experiment gets a time limit.

### Deliberately not next

- **`lent`**: until a program calls `Peek` or `Scanner.Bytes`.
- **A string observer, an impure observer, a language segment**: ADR 0069's "Why not", each waiting for
  a program.
- **Houdini's compile cost on wide loops**: one program pays 1.2 s; optimize when a second does.
- **`e2`**: a precision loss with no program refused by it, now that `shape` is written. Diagnose it
  when one is.
- **Carried with their reasons**: more surface design; rendering the prover's fact log in J; the specs
  rewritten wholesale; hoisting or CSE on the IR; records, `with` and named views.
- ~~**The Windows application — hamza's**, when he hands it over.~~ **Amended the same day**: hamza
  handed over [TinyRetroPad](https://github.com/plummersSoftwareLLC/TinyRetroPad), a Notepad-style
  editor in about 2.5 KB of hand-written x86 assembly. It is item 6 of the plan, in CLAUDE.md and
  README.md's "Current work": first a measurement of what it calls from Win32.

## 6. The one sentence

The round wrote two programs by hand, met the wall the plan predicted and found it was a cost, and
answered it with a rule the project already had. The next round should fix the two defects those
programs exposed in every program, then write the next ones.
