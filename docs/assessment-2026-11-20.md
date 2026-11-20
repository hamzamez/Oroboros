# Assessment: callbacks built on every target, the book's two findings fixed, and five items in progress

2026-11-20. Deliberately **not** an ADR. It follows [2026-11-19](assessment-2026-11-19.md).

Written under the rule (an assessment every ten days or 30 commits) at **one day and 30 commits**,
while hamza's list of 2026-11-20 ("Build all 5") was in progress, its first item done. The round
wrote **6 ADRs** (0139–0144), **11 result documents** and **3 research documents** (views-finish,
scopeforms, formers), each result with its ledger.

> **Short answer: tier 1 callbacks are decided and built on every target, a written cell included;
> the book's findings 100 and 101 are fixed; and the margin is the best in five rounds, 5.3 : 1,
> because the round wrote programs (§3.1).**
> - **Built**:
>   - ADR 0139–0142: an `again` after a nested loop's join point; a lent buffer handed back as a
>     primed result; SendMessageA's members as its C function at a constant; a target's CPU as data;
>   - ADR 0143, tier 1 callbacks, after hamza decided T1-2 to T1-5 on callbacks-sketches Part 3. A λ
>     during the call is a region of it, decided under its claim. It is closure-converted into the
>     context word on windows and is the host's own closure on Go, JavaScript and the JVM. A written
>     cell is threaded where the host promises an order: `ordered` or `calls 0..1`, each marker's
>     host sentence kept and checked both ways;
>   - ADR 0144, hamza's "why?": a definition of the program's applied to a λ that writes a cell is
>     expanded before the state translation, the strongest host there is;
>   - findings 100 and 101 (findings-2026-11-20), which were the 11-18 plan's item 3's refusing
>     program;
>   - the book brought up to date (book-2026-11-20): two new chapters and its findings 95–102.
> - **A wrong answer, found writing the derivation and fixed ahead of the plan**: a λ bound to a
>   name that wrote a cell lost the write (finding 102, callbacks-derivation-2026-11-20).
> - **Researched, for hamza**: the memory model's rest (views-finish), the scope form
>   (scopeforms), and formers (formers-research: a data structure's literal as its free algebra's
>   term, the trailing block its first step).

## 1. What the plans asked for, scored

**The 11-18 plan** (approved 2026-11-19):

| # | item | outcome |
|---|---|---|
| 1 | TinyRetroPad's body in J, module by module | **Not started** (§4), for the second round |
| 2 | the callback decisions | **Done**: tier 2 (ADR 0133), tier 1 decided and built (ADR 0143) |
| 3 | the capacity facts | **Done**: the refusing program, a map read then stored to, builds (findings-2026-11-20) |
| 4 | a timeout on every program a test runs | Done last round |
| 5 | CLAUDE.md as state | kept |
| 6 | the words audit | first pass done last round; the rest for hamza |

**hamza's decided list of 2026-11-19**, its remaining items:

| item | outcome |
|---|---|
| ADR 0136's struct forms, with TinyRetroPad | **Not started** (in "Build all 5", item 4) |
| P5, SendMessageA's members | **Built** (ADR 0141) |
| P3, P4, two segments lent at once | **Not started** (in "Build all 5", item 3) |
| u128 in the language's own operations | **Blocked** on U₆₄ ↪ ℤ, as last round |

**hamza's "Build all 5"** (2026-11-20):

| # | item | outcome |
|---|---|---|
| 1 | findings 100 and 101 | **Done** (findings-2026-11-20) |
| 2 | T1-4, a buffer written in a λ | designed (callbacks-written §"Not built"), next |
| 3 | the memory model's decided parts | not started |
| 4 | ADR 0136's struct forms with TinyRetroPad | not started |
| 5 | Win32's enumerator pages as evidence | not started |

## 2. What this round established

- **A host's promise about its calls chooses the algebra a callback's effect must belong to.**
  - No promise: pure (Theorem D).
  - Each element once, no order: a commutative monoid.
  - In order: any monoid.

  formers-research §5 shows this is the Boom hierarchy (lists, bags, sets) and the well-formedness
  theorem of monoid comprehensions. So the callback lattice and a former's laws are one notion, and
  the middle strength's word names the bag level.
- **A definition of the program's is the strongest host.** It has no recursion, no escape, and
  program order. The refusal it met came from the order of two passes, not from the meaning
  (ADR 0144).
- **A version is data, and only the translation knows it.** Under the reducer's binders a read's
  head is an index. A rule that needs "this is a map's version" belongs where the name is: the
  translation (findings-2026-11-20).
- **A table of tables given by its graph is the n-way choice among its rows.** Flattening it
  into rows and offsets needs a theorem relating two reads of one table, which a fact (a schema
  over one trigger) cannot state.
- **Every marker is evidence-checked on every target that has local documentation**: `go doc`
  for Go, the JDK's `src.zip` for the JVM. ECMA-262 and Microsoft Learn are kept as data, read once
  with hamza's permission.

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-11-18 | 2026-11-19 | **2026-11-20** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 101,010 | 103,782 | **106,603** |
| of which tests | 29,376 | 29,708 | **29,996** |
| `core` / `emit` / `ir` | 27,138 / 38,241 / 35,631 | 28,410 / 38,920 / 36,452 | **29,174 / 39,519 / 37,910** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 2,756 | 2,884 | **3,419** |
| J outside that count: the book's programs (`docs/book/code`) | 3,254 | 3,254 | **3,778** |
| J outside that count: the LLVM layer (`targets/llvm/*.oro`) | 453 | 455 | **455** |
| **at the margin**, counted the usual way | 573 : 1 | 21.6 : 1 | **5.3 : 1** (2,821 : 535) |
| at the margin, with the J outside the count | 77 : 1 | 21.2 : 1 | **2.6 : 1** (2,821 : 1,059) |
| overall | 36.7 : 1 | 36.0 : 1 | **31.2 : 1** |
| (`emit` + `ir`) : `core` | 2.72 : 1 | 2.65 : 1 | **2.65 : 1** |
| hand declarations (`targets/*.oro`) | 5,898 | 5,952 | **6,203** |

`python gauntlet/balance.py da1c4134 HEAD` reproduces the compiler's rows and the language's; the
book's and the LLVM layer's are lines of code by the same rule. (The 11-19 column is
recomputed at its commit, `da1c4134`; the assessment quoted 103,772 and 21.6 : 1.)

**The language grew by 535 lines, nineteen programs**:
- five for callbacks: caesar, prefix, allwindows, quantifiers, semilattice;
- four for written cells: dyck, kadane, once, horner;
- four for the findings: collatz, twosum, magic, shoelace;
- the book's new chapters' programs (524 more lines, outside the count).

Each program was a famous algorithm or a host's documented law, chosen because hamza asked for
programs, not because a refusal demanded them. Several found the compiler's defects at once:
- Collatz met three shapes in one program;
- `once` found that a host cell and a λ's cell did not compose;
- `prefix` and `once` found the λ widened with no thresholds.

**Where the compiler grew** (+2,821):
- `core` +764: the callbacks' translation, ADR 0144's expansion, the findings' elimination and
  choice;
- `ir` +1,458: the λ's region in every analysis, closure conversion, the release at the call, the
  printers' written cells;
- `emit` +599: the callback claims and markers, the literal of tuples, the read's continuation.

### 3.2 Precision and performance

- **Provability**: **12,435 of 12,489** operations and **1,477 of 1,772** loops on the four
  targets. Every new operation is proven. Two new loops are not: Collatz's iteration, bounded by a
  guard on its counter that size-change termination does not read. Windows32: **6,869 of 6,879**
  and **777 of 858**.
- **Run time**: the JVM's written cell, a final one-element holder, is 1.6–1.7× a plain loop. A
  static field is the loop's speed and races across threads, so it was rejected. A table of tables
  read at an unknown index is a chain of comparisons. Neither is measured against a program that
  needs it fast.
- **Compile time**: 0.95–1.05× in every run, within the gate; windows32 1.04–1.06×.
- **The survey**: Win32 36.2% callable and 31.1% linkable, a function pointer counted since this
  round (an upper bound) and a record under any of its typedef names.

### 3.3 Soundness: the round's findings

1. **A λ bound to a name wrote a cell and the write was lost** (finding 102), a wrong answer:
   `add = fn(x) { n := n + x }; add(1); add(2)` printed 0. Found deriving tier 1, not by a test.
   Refused now, with its planted fault.
2. **A λ's parameter was typed [0, 0]** during the build, a proof from no premise, because nothing
   analysed its region. It became the domain rule's planted fault before it shipped.
3. **Two meanings changed silently in this round's own work, caught by the full check**:
   - the literal of tuples, flattened where it was handed to a variadic, changed a table's length;
   - a tightened read shape refused a map literal.

   Both were found by existing tests (the empty interface's, a scalar cell's) and the book.

### 3.4 Process: five things

1. **A rule placed where it can never fire.** The first fix for finding 100 went into the reducer,
   where the read's head is an index. It was found by tracing the redex, then removed. A rule's
   premise must be checked against the representation at the place it runs, before the rule.
2. **A tightening checked only against the programs in mind.** The lowering's map read was
   tightened for (((m k) L) K) without asking what else had matched the loose shape: a map literal
   and a `build-map` value. The compiler's tests found it, but only in the full check. A shape
   rule's change should be followed by the package tests at once.
3. **Heredocs mangled escapes three more times** (a Go test's `"\n"`, a debug print hidden by a
   grep, a removal pattern). The memory note now says to put any script with a backslash in a file
   by the Write tool. It was still broken by habit within the hour.
4. **A result's ledger went stale within the session**: callbacks-2026-11-20 described Go's stage
   alone after windows, JavaScript and the JVM were built. Fixed when hamza asked whether
   everything was built.
5. **Programs first worked**: every step this round began with a famous algorithm or a host's
   law, and each such program found a defect before its test was written.

## 4. What is left

- **"Build all 5"**, items 2–5, in order:
  - T1-4, a buffer in the λ, by a read and a store through the reference (designed);
  - the memory model's decided parts (P3's handles, two segments lent at once, P4's `kept`), each
    with its TinyRetroPad module;
  - ADR 0136's struct forms with TinyRetroPad's modules;
  - Win32's enumerator pages as evidence, which may give the slot's restore its witness.
- **TinyRetroPad's body**, plan item 1, for the third round: it is the program this compiler work
  is for, and items 3 and 4 above are its prerequisites.
- **U₆₄ ↪ ℤ**, blocking u128 and `print_int`'s total form.
- **Owed, found this round**:
  - a claim over a table's element set, `fn(x ∈ elem(a))`, so a callback's parameter carries the
    table's bounds and the programs' guards on x go;
  - the reader's `again` after a single-name binding over a nested loop;
  - a table of tables made by a rule, received or built;
  - Collatz's loops: a counter's guard as a measure.
- **Waiting on a trigger** (hamza, after this assessment): formers, built when a program asks;
  a callback on another thread while the call blocks, kept until concurrency is in the language.
- **Set aside for hamza**:
  - the middle strength's word (§2: the bag level);
  - a law a callback obeys;
  - a function a host returns;
  - the variadic of places;
  - a C string from Win32;
  - a table's element by its set on LLVM;
  - `__chkstk`.

## 5. The next plan (for hamza)

**The plan in force is hamza's "Build all 5"**, items 2–5, and it stands. After it:
1. **TinyRetroPad's body in J**, module by module, on the struct forms and memory model just built.
2. **U₆₄ ↪ ℤ**, four realizations, so u128 runs everywhere and `print_int` is total.
3. **The owed list above**, each small.

### Deliberately not next

- **Formers, the trailing block included**, until hamza decides F1–F5.
- **A callback's remaining words** (the middle strength, threads, laws, returned functions), until
  hamza decides them.
- **Anything in the earlier lists' "deliberately not next"**, which stands.

## 6. The one sentence

The round wrote programs, and the programs found the compiler's defects faster than the compiler's
own derivations did, so the next round should start from TinyRetroPad's body.
