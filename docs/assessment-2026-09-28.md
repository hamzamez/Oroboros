# Assessment: the round that ate its own tail

2026-09-28. Deliberately **not** an ADR. The previous ten are
[2026-09-24](assessment-2026-09-24.md), [2026-09-23](assessment-2026-09-23.md),
[2026-09-17](assessment-2026-09-17.md), [2026-09-13](assessment-2026-09-13.md),
[2026-09-11](assessment-2026-09-11.md), [2026-09-09](assessment-2026-09-09.md),
[2026-09-06](assessment-2026-09-06.md), [2026-08-20](assessment-2026-08-20.md),
[2026-08-19](assessment-2026-08-19.md) and [2026-08-13](assessment-2026-08-13.md).

Written when the plan that took the round finished: ADR 0032's migration. Four days, **40 commits**,
26 result documents and **six ADRs** (0030–0035).

> **Short answer: the compiler was rebuilt on its own IR and came out no larger, more precise, faster to
> compile, and simpler. The language's code did not grow.**
>
> **The migration.** Every backend prints from a structured SSA IR. Every analysis and every choice of
> representation is made on it. The term interval analysis is deleted, 7,222 lines net of production
> code in its last step alone. Over the round the compiler grew **2,017 lines**, while an 18,776-line
> IR replaced everything it made redundant.
>
> **What it bought, measured:**
> - loops proven to terminate: **357 of 375**, where 38 were unproven at 09-24 and 18 are now;
> - `freq`, the research's own witness, compiles in **0.66×** the time, on **0.46×** the CPU, and its
>   collector share fell from 49% to 31%;
> - the unproven operations fell from 47 to 45, both of them on map reads.
>
> **What it found:** latent wrong answers and false premises the old structure could not show. They
> include an aliasing gap in the bignum destination rule, a range obligation Java printed as n mod 2³²,
> and a limb fallback that could never emit. Each has a witness that fails without its fix.
>
> **The balance.** **Four** lines of Oroboros against 2,017 of compiler: **504 : 1** at the margin, or
> **38.8 : 1** counting hand declarations and acceptance programs. The plan's first item, `bufio`, was
> not started.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | `bufio`, program-first; the tuple-component law met a fourth time becomes the next ADR | **Not started.** The conditional clause fired anyway: the JSON parser's `(tuple nodes nn ok)` met the law a fourth time, and it became ADR 0031 (a `build`'s result is a product) |
| 2 | The Windows application: first a measurement, then spec | **Waiting.** hamza has not handed it over |
| 3 | The analysis grows only for a refusal a program names; every rule gets a soundness witness | **Kept in spirit, overtaken in fact.** The analysis did not grow; it was rebuilt on the IR and the old one deleted. Every IR rule has an execution witness and a planted fault (irstep4b–4j) |

**The round replaced the plan.** On 09-25 hamza framed the next move as "eat our own tail": derive
the compiler's structure from what the language taught, and take every win it offers (research §0).
The research measured what the missing IR cost:
- `freq`'s compile spent 46% of its CPU in the garbage collector;
- 16% was the backend rerunning the interval analysis per `build`;
- 21 backend methods existed three or four times;
- 31 side tables were keyed by rebuilt term pointers.

ADR 0006 had promised the IR on 2026-08-13 and it was never written. So the plan is scored against the
migration it became, and against the balance the 09-24 plan was meant to fix.

The round in four phases, counted with the script of §3.1 (the compiler here is `core`, `emit` and
`ir`, tests included):

| phase | commits | compiler | Oroboros | hand declarations | acceptance |
|---|---:|---:|---:|---:|---:|
| **A. Before the IR**: ADRs 0030, 0031, lexical names, obligations refused | 13 | +2,579 | +3 | +46 | +2 |
| **B. Research and three prototypes** | 5 | +38 | 0 | 0 | 0 |
| **C. IR steps 1–3**: every backend a printer | 10 | +11,171 | 0 | 0 | 0 |
| **D. Step 4**: the analyses onto the IR; the term engine deleted | 12 | **−11,771** | +1 | 0 | 0 |

The prototypes (`experiments/irproto`, 3,158 lines) sit outside the compiler definition. They were
written in B and deleted in D.

## 2. What this round established

**The residual is a closed first-order program, and its algebra is a distributive Freyd category with
Elgot iteration** (research, Theorem A; ADR 0032).
- `loop` is the iterate f† for f : X → Y + X; case-of-case is the coproduct's universal property; and
  pure means central.
- The IR is that category presented as structured SSA with π-parameters. A printer, the concrete
  semantics and every analysis are models of it, so faithfulness and soundness are per operation.
- The three prototypes predicted the costs, and the build met them:
  - lowering is nearly free;
  - a sparse interval analysis runs 12–115× faster;
  - a Go printer is at parity.

**Representation is a type, chosen by ρ of a fact, and every crossing is an obligation** (ADR 0033).
The checker's integer sort is ℤ. The unsigned word is chosen by the ring homomorphism ℤ → ℤ/2⁶⁴:
- `+ − ·` compute in the result's realization;
- `/ %` and the orders compute in a realization holding both operands.

**The rung above the word is a least fixed point** (ADR 0034). B is closed under supply and demand,
with demand gated by the fact. The old pressure rule is demand at an operand position. The Go bignum
destination rule became **ownership**, the greatest set of loop parameters whose objects no one else
holds, and **liveness**. Restating it on SSA found an aliasing the syntactic rule R missed: a continue
handing a loop variable an object from outside the loop.

**A compiler library is a theory checked once** (ADR 0035). The fixed-limb library's lemmas hold
under their preconditions, independently of any call. They are checked per width, and each use is an
instance whose precondition is the program's own obligation.

**The IR's domain proves more than the term analysis did, with less:**
- intervals over ℤ̄ with ends in (−2¹²⁶, 2¹²⁶);
- trip-count theorems, including one on differences over linear forms;
- the relational difference;
- size-change termination with the floor the integers lack;
- a map's cells, as the frozen-buffer theorem over another index set.

It is 3,328 lines, against the ~10,000 of the term analysis layer it replaced.

**Every obligation is discharged or the program is refused** (noprop). "Propagated, not proven" is
gone. An allocation's size became an obligation, which closed Java's `new T[(int) n]`.

## 3. Are we on the right track

### 3.1 The balance of effort

Counted exactly as before. The script reproduces every figure of the 09-24 assessment at its revision
`b97c095`. That checks three definitions the table leaves implicit:
- `emit : core` is with tests;
- hand declarations are raw lines of `targets/*.oro`;
- survey tooling includes `Dump.java` and `jsdump.mjs`.

**One definition changes, and says so.** The compiler was `.go` in `core/` and `emit/`. The IR (`ir/`)
did not exist; it now holds most of the compiler, so it is counted, and the old definition is kept in
its own row for comparison.

| | 2026-09-17 | 2026-09-23 | 2026-09-24 | **2026-09-28** |
|---|---:|---:|---:|---:|
| compiler, `core` + `emit` (the old definition) | 45,503 | 48,445 | 51,546 | **34,787** |
| **compiler, with `ir`** | | | 51,546 | **53,563** |
| of which tests | 15,041 | 16,528 | 17,764 | **19,000** |
| Oroboros written | 1,516 | 1,524 | 1,571 | **1,575** |
| **at the margin** | 2,631 : 1 | 368 : 1 | 66 : 1 | **504 : 1** |
| overall | 30.0 : 1 | 31.8 : 1 | 32.8 : 1 | **34.0 : 1** |
| `emit` : `core` | 4.42 : 1 | 4.23 : 1 | 4.28 : 1 | **2.48 : 1** |
| (`emit` + `ir`) : `core` | | | 4.28 : 1 | **4.36 : 1** |
| analysis layer (`interval`, `refine`, `linear`, `monotone`) | 6,935 | 7,136 | 7,626 | **3,530** |
| analysis layer, extended | 8,727 | 8,942 | 9,432 | **4,617** |
| …plus `bound` and `wordsel` | | 9,524 | 10,014 | **4,617** |
| **the IR's analysis** (`interval`, `trip`, `endpoint`, `sct`, `decide`, `require`) | | | | **3,328** |
| the IR's representation choices (`words`, `big`, `bigreuse`, `limbs`, `shift`, `final`, `restrict`) | | | | **3,031** |
| largest program | 163 | 164 | 164 | **164** |
| survey tooling | 7,150 | 7,387 | 7,533 | **7,553** |
| hand declarations (`targets/*.oro`) | 3,223 | 3,516 | 3,697 | **3,743** |
| acceptance programs | 229 | 347 | 407 | **409** |

**With hand declarations and acceptance programs counted, the margin is 2,017 : 52 = 38.8 : 1.** The
Oroboros figure hides churn: every program was respelled in the binder form of `build` (ADR 0031),
+178 −132 lines. The largest program did not grow, again.

**Read the compiler figure for what it is.**
- 18,776 lines of IR were built.
- Everything they made redundant was deleted:
  - the four term backends (−8,858 in 4a);
  - the term analysis and its satellites (−7,222 in 4j);
  - the prototypes.
- The net is +2,017.
- `emit`'s analysis layer shrank from 10,014 lines to 4,617, which is the refinement layer that stays.
- What decides legality, 3,328 lines of the IR's analysis, is a third of what it replaced.

**But the margin is the worst in three rounds, and the reason is the same as always.** A migration
writes no program. That was the choice, made for measured reasons. The measure is still the one this
project set itself: the language's code has to be what grows. It grew by four lines.

### 3.2 What the migration bought, measured against its own motivation

The research's table (§0) was the case for the IR. Re-measured on the same program, with both binaries
built serially and warm, the baseline in a worktree at `b97c095`:

| `freq` on Go | 09-24 | now | |
|---|---:|---:|---:|
| wall, median of 5 interleaved | 7,991 ms | 5,257 ms | **0.66×** |
| CPU | 15.5 s | 7.2 s | **0.46×** |
| garbage collector's share | 49% | 31% | |
| `BufferRange` (the backend rerunning the analysis) | 9% | gone | |
| the largest single cost | the collector | `ir.Decide`, 31% | |

The rows about structure:
- **21 duplicated methods:** gone with the term backends; each printer is a model of one IR.
- **28 structure recognisers, 13 clause-chain walkers:** the IR's regions are the structure, and a
  shape a pass does not know cannot exist.
- **31 side tables keyed by rebuilt terms:** gone; values are numbers.

The gate's compile time read 0.95× at the end of the round, and fell to 0.56× in 4e, the step the term
analysis left the default path.

**Precision.** Unproven loops went from 38 to 18:
- the JS natives from 16 to 3;
- the bignum loops from 7 to 0, since render's factorial counter is now a word on both rungs.

Unproven operations went from 47 to 45. The IR is not weaker anywhere on the corpus or on the tests
that were ported to it (§3.4).

### 3.3 Soundness: the round's findings

Each item below was found while stating a rule on the IR, and each has a witness that fails without
its fix:

| found | lived since | how it was found |
|---|---|---|
| `(len (build b 4294967297 …))` was **1** on Java: a size past `max-len` was cast | the Java backend | making an allocation's size an obligation (4f) |
| **The bignum destination rule R missed an aliasing**: a continue binding a loop variable to an outside object, written into on the next iteration | bigreuse-2026-09-02 | restating R as ownership (4h). No corpus program had the shape |
| **The limb rung's fallback to the host could never emit**: `CheckSignatures` checked the host's code against the limb signature | the fallback | taking the fallback through the IR (4h) |
| **A printer's inlining assumed a pure value observes no store**, false for a bignum on Go | the IR's plan | the destination rule (4h); witnessed on hand-built IR |
| **u128's `decimal` passed an unconstrained `int` to a `uint64`** by the residue map | u128-2026-09-23 | the unsigned word's crossing obligation (4g) |
| **The verifier accepted a word where a `u64` or `big` was held**, twice | ADR 0033's relaxation | planted faults that passed (4g, 4h) |
| **The IR's domain lacked a map's cell fact** the term analysis had | 4b's parity claim | porting a test (4j) |
| **The IR's `u64` transfers took the residue map as the identity** | step 2 | writing the transfers' soundness table |

### 3.4 Process: five things

**1. THE PLAN WAS SET ASIDE, AND THE BALANCE PAID FOR IT.** The migration was worth doing, and §3.2
says what it bought. It was also the fourth round in a row whose largest item was compiler work, and
`bufio` has now been first on the list twice. A migration has an end, and it has ended. The next plan
starts with a program.

**2. A RELATION RELAXED FOR ONE CHECK WEAKENED ANOTHER, TWICE.** ADR 0033 made the checker's integer
sort ℤ. The verifier's representation rule (W5) read the same relation, so it stopped telling `int`
from `u64` (4g) and then `int` from `big` (4h). Each time a planted fault passed a test that should have
caught it. The repair both times was a check of W5's own. **When a relation is relaxed for one
consumer, every consumer that reads it must be audited.** This goes into CLAUDE.md's lessons.

**3. PARITY ON A CORPUS IS PARITY ON ITS SHAPES.** 4b showed the IR proving what the term analysis
proved, function by function, over every program in the corpus. The map-read shape was pinned only by
a unit test, so the gap surfaced when that test was ported to the IR in 4j. **A comparison over a
corpus covers the corpus. The tests pin other shapes, and they have to be run against the new side
too.** Porting them, not deleting them, is what found it.

**4. A GATE MISSED A POPULATION ONCE MORE.** 4f committed the size obligation after the differential
suite but not after the acceptance programs, the third population of entry points. `encoding-hex` was
refused until 4g. From 4g on, every change ran all three. This is the scope lesson of 09-24, met again.

**5. THE TOOLING RULE WAS NOT FOLLOWED, AGAIN.** Heredocs collapsed backslashes in edit scripts at least
seven more times this round, and a Python edit in text mode wrote CRLF into nine files. Each was caught
before commit, by the build or by `gofmt`, and each cost a repair. A 53-minute hang of `cmd/check` was a
pipe `git archive` wrote and nobody drained; it is fixed. The memory note now covers Python's text mode
too. The rule stands: **edit scripts go through the file tool, and a file is written with
`newline=''`.**

## 4. What is left, read off the outcomes

**45 unproven operations, 18 unproven loops**, by program:

| where | operations | loops | what it is |
|---|---:|---:|---|
| `shift-div`, `limb-subdiv` (differential) | 32 | | the sweep compiles the case alone, and its exported `run` has no signature. **The harness, which builds `main`, proves them** |
| `examples/int/` (collatz, power, fib), `kara/core` | 11 | 1 | meant to be refused |
| the two wordcounts | 2 | | genuine: a count bounded by the input's length |
| `freq` | | 12 | three targets × four loops |
| `sieve-js` | | 3 | V8's word |
| `u128` (library and example) | | 2 | `decimal`'s loop: a component of `Div64`'s tuple, whose law cannot be stated |

**Named and not built, from this round's documents:**
- **Host callbacks, tiers 1 and 2**, specified and not built on any backend (irstep4a).
- **An `again` inside a tuple pattern's body, and a variant out of a scope** (ADR 0031's R1).
- **A limb instance emitted once as a function**, for code size: a measurement of its own (ADR 0035).
- **A bignum operation's transfer is its declared type**, so the second decision knows nothing about a
  value held exactly. Nothing counted reads one.
- **Carried from 09-24:**
  - a declared big result narrower than the program's widest;
  - signed limbs;
  - geometric accumulation;
  - the higher-order contract gap;
  - printing −2⁶³ on windows.

## 5. What is next

**1. THE GAUNTLET, IN FULL, ON THE IR'S PRINTERS.** It is the one fixed commitment, and its last full
run, on 2026-09-07, predates every IR printer. Each step of the migration measured what it changed
against the term backend, never against hand-written code in one run, and a chain of relative
measurements drifts. One run of all seven programs on three targets against the references closes it.
It is short, and it comes first, because everything after it builds on these printers.

**2. `bufio`, PROGRAM-FIRST.** Carried unchanged, and it is the round's real work. Its algebra first:
- a `Reader` is a buffered view of a byte stream;
- `ReadByte`/`ReadRune`/`ReadString` are partial maps with an error sum;
- `Scanner` is a split function iterated to a fixed point.

Then a line tool over standard input with `strconv` (number the lines, sum a column), written in the
language, counted, and not an acceptance program. ADR 0027 was built for exactly this loop shape and
still has one consumer.

**3. THE WINDOWS APPLICATION — hamza's**, when he hands it over. First the measurement of what it
needs from Win32 against what is declared, callable and linkable (31.0% callable and linkable), then
spec, with an ADR per wall.

**4. THE ANALYSIS GROWS ONLY FOR A REFUSAL ONE OF THOSE PROGRAMS NAMES**, and every rule it relies on
gets a witness that it is sound: an execution the interpreter runs, and a planted fault that fails.
That was this round's discipline on the IR, and it found most of §3.3.

### Deliberately not next

- **Optimising the compile.** `ir.Decide` is now the largest cost of `freq`'s compile (31%), and the
  gate is at 0.95×. Nothing is slow enough to be a problem. Measure before touching it.
- **Host callbacks, tiers 1 and 2.** No program has asked.
- **The limb instance as a function.** No program's size has been a problem.
- **Hoisting or CSE on the IR.** Either would make reachable the two premises witnessed only on
  hand-built IR: an object from before the loop, and a pure read moved past a write. Build them only
  with those witnesses running.
- **Carried from 09-24 with their reasons:**
  - the element-bound and geometric-accumulation limits;
  - per-result enforcement above the word, and signed limbs;
  - the higher-order contract gap;
  - a phase-wise trip count;
  - the windows, V8 and JVM unsigned rungs;
  - modular arithmetic as a type (ℤ/2ⁿ, the wall `hash/fnv` will hit);
  - dependent result ranges;
  - `ByteOrder`'s values;
  - records, `with` and named views.

## 6. The one sentence

The compiler now has the IR it promised itself on its first day. Building it removed as much as it
added, proved more, compiled faster, and found wrong answers the old structure could not show. None of
that is a program, and the next round is measured by one.
