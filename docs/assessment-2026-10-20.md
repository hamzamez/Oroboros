# Assessment: the surface built, the corpus moved, and one program written

2026-10-20. Deliberately **not** an ADR. The previous thirteen are
[2026-10-14](assessment-2026-10-14.md), [2026-10-03](assessment-2026-10-03.md),
[2026-09-28](assessment-2026-09-28.md), [2026-09-24](assessment-2026-09-24.md),
[2026-09-23](assessment-2026-09-23.md), [2026-09-17](assessment-2026-09-17.md),
[2026-09-13](assessment-2026-09-13.md), [2026-09-11](assessment-2026-09-11.md),
[2026-09-09](assessment-2026-09-09.md), [2026-09-06](assessment-2026-09-06.md),
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

Written when the 10-14 plan finished: the surface is decided, built and the only notation. Seven days,
**31 commits**, **5 ADRs** (0064–0068), 9 result documents.

> **Short answer: the round did what it set out to do. The surface was decided (installments 6–8),
> J was chosen as the one notation, a reader and a translator were built behind the byte-identical
> gate, two of the eleven waiting ADRs were built (0052, 0058), the corpus, the tests and the
> survey generators moved to J, and today's reader was deleted. Emission never changed but where an
> accepted step made it shorter.**
>
> **What it did not do.** The plan said the notation would be decided by an experiment, and every
> installment would carry a real program; hamza chose J without the experiment (ADR 0068), and the
> round's one new program is `du`, written for installment 6. The Go library, step 5 of the plan, did
> not resume.
>
> **The balance.** +3,831 lines of compiler against +49 of the language, **78 : 1** at the margin,
> the worst since the IR was built; and 7,440 lines of translator written and deleted beside it. The
> switch itself added no meaning: the language's size in core-term nodes moved 6%, all of it ADR
> 0052's representation.
>
> **What the switch found.** A byte-identical gate checks the corpus, and the corpus holds no program
> the reader should refuse: J's reader lacked the NFC and bidirectional-control checks, the binder and tail rules, and a λ's
> parameter grammar, and four refusal tests passed for the wrong reason once the bridge was gone.
> Every one was found by moving the tests, none by the gate.

---

## 1. What the last assessment asked for, scored

The 10-14 plan was "S, the surface first, with programs as its measurement", in five steps.

| | asked | happened |
|---|---|---|
| 1 | **Installments 6–8, each with one real program as its gauntlet** | **Done, programs in part.** Installment 6 (declarations and modules, ADR 0064) was tested on `du`, a real directory walk, program first for `os.ReadDir`, `io/fs` and `path/filepath`. Installment 7 (static parameters, the arrow, ADR 0066) on `top`, written by hand in D and J and not runnable (callbacks tier 1 not built). Installment 8 (the lexical level, ADR 0067) on the corpus and three files. J writes sets in TLA+'s symbols (ADR 0065) |
| 2 | **D or J decided by an experiment**, the same programs written in both, counting what the reader catches | **Not done.** hamza: "no need for an experiment. J is our syntax" (ADR 0068). The question it would have measured, which notation is written more correctly, is open |
| 3 | **The reader and the translator, behind the byte-identical gate** | **Done.** spec/j.md consolidated ADRs 0050–0067 (six conflicts resolved, three gaps filled, all confirmed by hamza). Every program, fragment and target file printed in J with every comment (jprint) and read back to today's meaning (jread), 560 runs byte-identical through J |
| 4 | **ADRs 0052–0062 built one at a time** | **Two built, the syntax of most realized.** Program-order buffers (0052, storeorder) and map buffers (0058, mapbuf), each behind the gate; building 0052 found three gaps, G5–G7, decided by hamza. J's grammar realizes the surface of 0054–0057, 0059–0061, 0064, 0065 and 0067 as forms D desugars into today's core. Not built: the arrow (0053), the logic of 0062 (`spec`, `lemma`, `assert`), the inference of 0066 |
| 5 | **Then the Go library resumes in the new notation** | **Not begun.** 305 of 590 names (51.7%), one more than at 10-14 (`os.ReadDir`) |

**What the plan did not have.** Moving the corpus was planned; deleting today's reader, μ, the
translator and the bridge (hamza: "remove it"), moving the tests' inline snippets and four survey generators
to J, and rendering every message in J were not, and they were half the round's commits.

The round in five phases, cut at 0d64c32, 22623d8, 329e34c and 7214e38, counted with
gauntlet/balance.py (§3.1):

| phase | dates | commits | ADRs | compiler | of which tests | `j/` (translator) | Oroboros | docs |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| **A. The surface decided**: installments 6–8, du, setdef, spec/j.md | 10-14 – 10-15 | 15 | 5 | +186 | +99 | 0 | +49 | +3,323 |
| **B. Printed and read**: the concrete tree, print_J, the translator, J's reader | 10-15 | 4 | 0 | +333 | +100 | +5,529 | 0 | +21 |
| **C. Two ADRs built behind the gate**: 0052, 0058 | 10-15 – 10-16 | 4 | 0 | +1,330 | +85 | +1,513 | 0 | +119 |
| **D. The switch**: the corpus (5a), the tests and generators (5b.1–5b.3), today's reader deleted (5b.4) | 10-16 – 10-20 | 7 | 0 | +1,362 | −614 | −7,042 | +124 | +13 |
| **E. Messages in J** | 10-20 | 1 | 0 | +620 | +140 | 0 | 0 | +20 |

## 2. What this round established

**A file is a theory, and a declaration a header and one body** (ADR 0064): a file is the module named
by its path, a target file a set of fragments of the model its directory names; visibility is public
with `private` hiding; a variant is a union of tagged sets; an inclusion is an axiom of a model.

**A definition over sets is a λ returning a λ** (ADR 0066): `E ⊆ S` binds a set, the arrow is a λ's
header whose domain is a telescope, and a host template is quoted code with named splices, a value of
Code_T = text × requirements, a monoid under concatenation.

**J is the surface, one rendering of abstract.md** (ADR 0068), and the round showed what ADR 0050
promised: a notation is a homomorphic image of the abstract syntax, so moving the corpus was a
translator T → J and a reader J → Abs whose correctness the gate checks exactly. 152 programs and 43
target files read back to today's meaning; 560 runs stayed byte-identical through the switch.

**A buffer is a state binder in program order** (ADR 0052) and so is a map buffer (0058): the
state-passing translation S, with one new clause, the borrow, maps `b[i] := v` onto today's threaded
term, and Reynolds' syntactic control of interference (R1–R4) with a fresh-value rule (R0) replaces the
ordering check. Building it found that R0 refused the merge sorts' swapping buffers (G5, a concurrent
assignment), the record store `dt[s][1] := v` (G6), and one rule for every borrow (G7).

**A message quotes J** (jmsg): a name by σ_J, a set by J's formers, a core term by the inverse of D on
D's image, whose law parse_J(show_J(t)) = t holds on every corpus definition it reaches (492).

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-10-03 | 2026-10-14 | **2026-10-20** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 56,061 | 66,618 | **70,449** |
| of which tests | 20,034 | 23,396 | **23,206** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 1,530 | 1,696 | **1,869** (1,745 before the switch) |
| … in core-term nodes | | 9,878 | **10,877** (10,264 before the switch) |
| **at the margin** | 33 : 1 | 64 : 1 | **78 : 1** (3,831 : 49) |
| overall | 36.6 : 1 | 39.3 : 1 | **37.7 : 1** |
| `emit` : `core` | 2.68 : 1 | 2.10 : 1 | **1.66 : 1** |
| (`emit` + `ir`) : `core` | 4.58 : 1 | 3.41 : 1 | **2.71 : 1** |
| analysis layer, extended | 4,964 | 5,808 | **5,808** |
| the IR's analysis | 3,336 | 3,342 | **3,342** |
| largest program (`freq`) | 164 | 163 | **194** (163 before the switch) |
| acceptance programs | 414 | 530 | **732** (530 before) |
| hand declarations (`targets/*.oro`) | 3,534 | 3,767 | **4,298** |
| survey tooling | 8,197 | 9,432 | **9,509** |

**The language's lines across the switch.** J writes one statement to a line: the corpus's lines of
code rose 7% (1,745 → 1,869) and freq's 19% (163 → 194) with no definition added; the acceptance
programs' 38% (530 → 732), and the hand declarations' raw lines 12% (3,846 → 4,298). So the margin is counted before the switch, in one
notation: +49 lines, all `du`. As a cross-check that does not depend on the rendering,
`balance.py --nodes` counts the core terms' nodes by each revision's own reader
(gauntlet/balance_nodes.go): +386 for `du`, then +613 (6%) across the switch, which is ADR 0052's
representation of a place, not new meaning. Counting J's `//` comments as code, as balance.py did until
this assessment, read freq as 424 lines: a measurement error that reached a status report before the
node count caught it (§3.4).

**Where the compiler grew.** `core` +3,889: J's reader (lexer, parser, target reader, σ_J), the
place translation and its R0–R4, and the message renderer, against μ, the concrete tree and today's
datum reader deleted. `emit` −47 and `ir` −11: the loader and the printers did not change, which is the
abstract syntax doing its job. The translator, package `j`, was written (+7,440) and deleted whole; it
was scaffolding, and counted nowhere above.

**Where the language grew.** `du`, 102 lines, a directory walk. Nothing else was written in the
language; the corpus was translated.

### 3.2 Precision and performance

**Provability is unchanged in what is left**: 2,039 of 2,084 operations and 352 of 371 loops, **45
operations and 19 loops unproven**. `du` added 19 operations and 2 loops (1 unproven), and the
accepted program-order cells (4efca1c) made `count` and `ages` 19 operations shorter, all of them
proven, which is why the operations' totals returned to 10-14's. CLAUDE.md quoted 2,058 of 2,103, the
count between those two commits, for five days (§3.4).

**The gauntlet was not re-run**: no printer changed, and emission was byte-identical through every
step but the one accepted change. **Compile time is flat**: 1.00× and 1.04× the baseline in the two
sweeps after the switch.

### 3.3 Soundness: the round's findings

| found | lived | how it was found |
|---|---|---|
| **J's lexer had no NFC and no bidirectional-control check**: two identifiers that display alike were two names, Trojan Source open again | from J's reader (22623d8); live in the drivers from 5a (2a88c5d) to 08363a4 | a refusal test that passed vacuously once rewritten, found by instrumenting the readers |
| **J's reader did not enforce W2 and W3**: a repeated binder, an `again` outside a tail or of the wrong arity, a loop whose last arm was not `else` | from 22623d8 to 6a5c112 | the tests' refusal snippets moved to J |
| **A λ's parameters were any tokens**: `fn(m ∈ 0..9)` read as five parameters | from 22623d8 to 08363a4 | a test's J snippet failing at an arity no source states |
| **`tuple(1)` was accepted**, a product of one | from 22623d8 to 08363a4 | a vacuous refusal test |
| **A quoted name went through the set convention** (`` `Int` `` read as `int`), **and an x86 hole past the ninth operand read back as `%10`** | J's target reader; only generated files reach either | checking the generators' J against their old output |
| **Four refusal tests passed for the wrong reason** once the bridge was removed: J refuses a leading `(` whatever follows | 08363a4 | instrumenting the J readers over the whole suite, with a planted case |
| **A usage test tested μ's trees** after the switch | 5a to 5b.4 | deleting today's reader |
| **The JVM survey declared a type function named `array`** for every table of tables; **Go's survey wrote four files today's reader could not read** | since each survey | comparing each generator's J with its old output |
| **A refusal quotes the compiler's own names**, `r[fn(#t, #p, #p1) { … }]`: `lib/result.oro`'s `err-map` on windows indexes a variant's value | open | rendering messages in J |

The pattern is the round's lesson: **every finding of the switch was in what the reader refuses,
and the gate, whose corpus breaks none of the reader's rules, could see none of them.** The tests' refusal cases saw
them, but only once they were in the new notation and only where they did not pass vacuously.

### 3.4 Process: five things

**1. THE GATE CHECKS THE CORPUS, AND THE CORPUS IS WELL FORMED.** Byte-identical emission over 560
runs proves the reader gives every corpus program its meaning, and compares 259 refusals' texts; but
those refusals are of names and arithmetic, decided after reading, and no corpus program breaks the
reader's own rules. A reader is two things, its graph and its domain; the gate checks only the
first. The refusal tests are the second, and they need
an anti-vacuity guard of their own: a refusal must be refused for its reason.

**2. A NUMBER IN A STATE DOCUMENT IS A MEASUREMENT THAT AGES.** CLAUDE.md's provability count was right
when written (and jread measured the same, at its commit) and wrong after an accepted change made two
programs shorter, for five days. An `-accept` that changes a count should change the documents that
quote it.

**3. A MEASUREMENT WITH A BUG REACHED HAMZA.** "J inflates lines 2.5×" was reported from balance.py,
which counted J's comments as code. The node count, written to cross-check it, caught it before the
assessment used it. CLAUDE.md: *a survey's first number describes the measurer.*

**4. A PATH NOTHING RUNS.** Ten measurement scripts parsed today's notation with their own scanners and
panicked on the J corpus from 5a until 5b deleted them; nothing noticed for four days. They recount
only at 0c6470f now.

**5. THE PLAN'S MEASUREMENT WAS SKIPPED, AND THE BALANCE SHOWS IT.** The 10-14 plan put a program in
every step as the balance's remedy. Step 2's experiment was declined, installment 7's program cannot
run, and the switch wrote none: 78 : 1.

## 4. What is left, read off the outcomes

**45 unproven operations and 19 unproven loops**: 10-14's 45 and 18, and `du`'s one loop.

**Decided and not built**: the arrow and callbacks tier 1 (ADR 0053); the logic of 0062 (`spec`,
`lemma`, `assert`, a λ's own header, which J's reader now refuses at the `∈`); the inference of 0066;
step 2 of types as values (0063).

**Found and named**:
- `lib/result.oro`'s `err-map` on windows, refused through a Church encoding (§3.3);
- the JavaScript survey's Node pin, v26.7.0 against v26.8.1 here, which fails the tooling step;
- the specs' examples, today's notation until each is next touched (hamza's option (b));
- the prover's fact log after `known:`, in the linear fragment's notation;
- dead code older than the round (`core.BufferElem`, `Has`, `MapKeyOK`, `OnlyFragments`,
  `projectTails`, `occupied`).

**Carried from 10-14**: a borrow a later call ends; ℤ/2ⁿ; an unsatisfiable contract; a join across
representations; a host `where` on the IR's intervals; freq's `slen`; kara's read over write.

## 5. The next plan

The surface plan is finished, so 10-14's alternative stands alone: **G, the Go standard library,
package by package, program first, now in J**, the standing goal. Nothing in the language blocks it:
`os` is 36 of 189 names declared, `bufio` 11 of 77, `io` 39 of 67, and of the 285 names missing from
the ten packages with a file, 203 are work, declarable today (L); and the balance asks for exactly
this.

1. **Two loose ends first, each small.** `err-map`'s refusal on windows: find why a variant's value
   reaches the refinement layer as a table read, a defect upstream of the message (§3.3). And the Node
   pin: re-measure the JavaScript survey and re-pin the documents that quote it, so the tooling step
   passes again.
2. **`os`, program first.** A program that lists a tree with sizes, modes and modification times,
   `ls -lR` shaped: it needs `os.Lstat`, `FileInfo`'s methods, `FileMode`'s rendering and `time.Time`,
   so it brings in `time`, the first package with no file (K). Each declaration checked against the
   host, as `du`'s were.
3. **`bufio`'s `Reader`, program first**: a program that reads delimited records. It will meet C, a
   borrow a later call ends (`Peek`, `ReadSlice`), the first wall; research it when it is met.
4. **The notation measured on real programs, cheaply**: every program of this plan written in J by
   hand, and what J's reader catches counted, which is the half of the declined experiment that costs
   nothing.
5. **The analysis grows only for a refusal a program names**, every rule with a soundness witness and
   a planted fault; **ADR 0053, 0062's logic and 0066's inference each when a program needs it.**

**The measure of the round**: the margin, counted in J lines and checked in nodes; it should fall below
the compiler's growth for the first time since the IR.

**Kept as rules from §3.4**: a refusal test states its reason and is checked for it; an `-accept` that
changes a count updates every document that quotes the count.

### Deliberately not next

- **More surface design**: J is decided; the remaining ADRs are built when a program needs them.
- **Rendering the prover's fact log in J**: it is the decision procedure's notation, and no program
  asks.
- **The specs rewritten in J wholesale**: each when it is next touched.
- **The D-or-J experiment**: declined by hamza; its cheap half is item 4.
- **Optimising the compile**: flat.
- **Carried from 10-14 with their reasons**: hoisting or CSE on the IR; the limb instance as a function;
  geometric accumulation; per-result enforcement above the word; the higher-order contract gap;
  dependent result ranges; records, `with` and named views.
- **The Windows application — hamza's**, when he hands it over.

## 6. The one sentence

The round built the surface the last one specified and moved everything onto it without changing a
byte of output, and found that the gate it trusted cannot see what a reader should refuse; the next round should
write programs in that surface, the Go library's, and let them grow the compiler.
