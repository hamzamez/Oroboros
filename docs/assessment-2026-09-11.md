# Assessment: the round the hosts were measured, and the language was not written

2026-09-11. Deliberately **not** an ADR. The previous five are
[2026-09-09](assessment-2026-09-09.md), [2026-09-06](assessment-2026-09-06.md),
[2026-08-20](assessment-2026-08-20.md), [2026-08-19](assessment-2026-08-19.md) and
[2026-08-13](assessment-2026-08-13.md).

Written for the same reason as the last one: **its plan has run out.** Items 1 to 3 are done and
item 4 is about to be, so the document saying what to do next has been describing finished work.

> **Short answer: yes on the thesis, no on the balance, and one silent wrong answer found by writing
> this.**
>
> **The thesis is now measured on all four hosts, and the two big ecosystems agree.** Go **60.4%**
> usable, the JVM **60.9%**, from completely different refusals; Win32 **34.8%**; JavaScript
> unanswerable by construction, because a host with one type has no domain for the question.
>
> **The balance went backwards.** Against the last round's 11.5 : 1, this one wrote **1,074 lines of
> compiler and 6 lines of Oroboros programs** — 58 counting the `os`/`io` cells moved out of
> `targets/`. What grew was the survey tooling: **1,511 → 4,559 lines**, more than the compiler and the
> language together, and **none of it has a single test**. `emit : core` moved the wrong way for the
> fifth round, **3.98 → 4.13**; the analysis layer, for the first time, did **not** grow.
>
> **AND A SILENT WRONG ANSWER IN EVERY GENERATED SIGNED WIN32 RESULT.** Checking the enum claim
> before fixing it found that the generator reads every result as `mov %r, rax`. A 32-bit result is
> in `eax`, and on x86-64 writing `eax` ZERO-extends — so a negative `int` comes back as a large
> positive one. `MulDiv(-7, 6, 2)` is **−21** in C and **4,294,967,275** in a program built from our
> generated declaration, run today. Every `HRESULT` is affected, and there failure *is* the negative
> value. Present since win32-2026-09-06; the acceptance program called `MulDiv(7, 6, 2)` and **could
> not have seen it**.
>
> **And one process failure, mine: the "675 enums" figure has no source.** It is in the last
> assessment, products.md and CLAUDE.md, all written the same day, and in no result document and no
> survey output. A number nobody measured was repeated three times as though it had been.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | A spec for the product | **Done the same day**, and it paid before it was finished: three accepted shapes refused, one of them emitting a Go type that does not exist |
| 2 | Close the API residue, host by host, starting with file I/O on JS and Java | **Done, and it became the round.** Portable I/O on three hosts; the layer correction; Go's `os` generated with methods; interfaces researched; the coercion built; struct literals measured and built; the JVM and JavaScript surveyed |
| 3 | AoS against SoA, per host | **Done** — the access pattern chooses, not the host |
| 4 | The Win32 enum classification | **Not done, and its number turned out to be unsourced** — §3.4. It is the next item |

**Three of four**, and the fourth is next. But item 2 absorbed the round, and it was the item with no
natural end — which is what an assessment should notice.

## 2. What this round established

Seven results in three days —
[portableio](../gauntlet/results/portableio-2026-09-09.md),
[gomethods](../gauntlet/results/gomethods-2026-09-09.md),
[coercion](../gauntlet/results/coercion-2026-09-09.md),
[structlit](../gauntlet/results/structlit-2026-09-10.md),
[surveys](../gauntlet/results/surveys-2026-09-10.md),
[aossoa](../gauntlet/results/aossoa-2026-09-11.md),
[product](../gauntlet/results/product-2026-09-09.md) (its §8 answered) —
and two research documents, [interfaces.md](interfaces.md) and [struct-literals.md](struct-literals.md).

**A fallible host call is totalisation, and the compiler never learns exceptions exist.** Go gives a
product plus a convention, JavaScript and Java throw, Win32 uses a sentinel. A target declares the
call and the discriminator; `try`/`catch` is a template. Three programs are portable on three hosts,
byte-identical.

**A module name is a claim, and the order matters.** `go/os` names a host package; `os` names an
interface several hosts share. hamza's correction — support each target's API before writing a
portable name over it — put the portable cells in `lib/` and the host's own API first.

**The host is the oracle, three times.** Go's subsumption edges are candidate-generated and
host-filtered (182 of 1,651 were false); the JDK and V8 surveys read the runtime itself rather than a
description of it; the JVM declares its own subtyping, so its coercion is read rather than derived.

**An interface is an existential type, and passing one costs nothing.** `⟦coerce⟧ = id`: one
declared edge, no emitted characters, 41 names.

**Seeding a fixed point is not running it.** struct-literals.md projected +486 names; built, it is
+229, and the difference is the method, reproducible on corrected data.

**Four hosts are priced, and writing the second big survey corrected the first.** Go's obtainable set
counted only package functions as sources; the JVM survey, where `new` and methods are the idioms,
made the omission obvious. Go 50.0% → **60.4%**.

**The access pattern chooses the layout, not the host.** SoA wins a column scan on every host; flat
wins a random gather on every host, by up to 5.58x. products.md §6 said the choice was the target's;
it is the program's, and the flat stride that shipped is the minimax one.

**One compiler bug**: a single-result JavaScript prim dropped its `(import …)`. Found by generating
the whole Node runtime, which is the one thing the surveys did that nothing else would have.

## 3. Are we on the right track

### 3.1 The balance of effort: it went backwards, and the tooling is why

Counted exactly as before — compiler including tests; Oroboros as non-blank, non-comment lines of
`examples/` and `lib/`, a method that reproduces the last assessment's **1,324** at its own revision.
Baseline `ed202e6`, the assessment and its same-day spec.

| | 2026-09-06 | 2026-09-09 | **2026-09-11** |
|---|---:|---:|---:|
| compiler (with tests) | 32,709 | 35,963 | **37,037** |
| Oroboros written | 1,049 | 1,324 | **1,382** |
| **at the margin** | 41 : 1 | 11.5 : 1 | **18.5 : 1** |
| of which programs | | | **6 lines** |
| overall | 31.2 : 1 | 27.1 : 1 | **26.8 : 1** |
| `emit` : `core` | 3.62 : 1 | 3.98 : 1 | **4.13 : 1** |
| analysis layer | | 5,175 | **5,175** |
| largest program | 112 | 155 | **154** |
| survey tooling (`gauntlet/stdlib`) | | 1,511 | **4,559** |

**58 lines of Oroboros, and 52 of them are the `os` and `io` cells that moved from `targets/` to
`lib/`** — declarations, not programs. The programs grew by six lines, all edits to the three tools
already there. The acceptance programs added 50 more lines outside the counted directories, and they
are twenty-line harnesses around one or two host calls each.

**What grew was the survey tooling, by 3,048 lines** — three times the compiler's growth — and it is
neither compiler nor language. That was the direction hamza set and it was the right one: *"the fact
that we can't express the entire go api, and the entire windows api, tells us we are still short"*
is answered by measuring, and it was. But three assessments asked for more language than compiler,
the last one got it, and this one lost it again because the round's work lived in a third place the
ratio does not count.

**The analysis layer did not grow — for the first time in five assessments.** 5,175 lines, unchanged.
Every change this round was in the format, the emitters' import and multi-result paths, and the
checker's one-line subsumption lookup. That is the risk that led four assessments standing still,
and it should be said as plainly as it was said when it grew.

### 3.2 The thesis: measured on four hosts, and it holds

| | callable | declarable | usable |
|---|---:|---:|---:|
| Go | 4,932 | 87.8% | **60.4%** |
| JVM | 38,042 | 81.4% | **60.9%** |
| Win32 | 11,575 | 76.1% | **34.8%** |
<!-- Win32 is 90.5% / 35.4% as of structval-2026-09-12, and only 16.3% of the
     callable names are in a library the build links. -->
| JavaScript | 2,340 | 100% | **not answerable** |

The two big managed ecosystems land half a point apart from completely different refusals — Go's are
interfaces and structs built by literal, the JVM's are type variables and `Object`. **Either that is a
coincidence or it is the parasite model's ceiling for a first-order, monomorphic residual**, and the
two refusals that remain large on both — a value the host would have to *manufacture* for us, and a
type that needs *quantifying over* — are exactly the two things the language refuses on purpose.

**The core did not move.** Seven term kinds, three reduction rules, unchanged. Everything this round
bought was format, generator, or one lookup in the checker.

### 3.3 The gauntlet

Last benchmarked 2026-09-07, all seven at parity. Every change since was verified byte-identical on
the emitted corpus — 178 of 178, then 19 of 19 on JavaScript — so nothing a gauntlet program emits has
changed and parity holds by construction. **Not re-run**, and that is the right call only for as long
as the byte-identity claims hold.

### 3.4 Process: three failures, all mine

**1. THE SURVEY TOOLING IS THE LARGEST BODY OF NEW CODE AND IT HAS NO TESTS.** 4,559 lines across four
surveys, two dumpers and their generators, and not one `_test.go`. Its numbers are quoted in CLAUDE.md
as findings. And it has been wrong more often than any other part of this repository: **the Go survey
alone has published seven corrections**, Win32 four, JavaScript one — collapsed type names,
predeclared types qualified per package, a readable result called unreadable, a seeded fixed point, a
missing source, a truncated manifest, a non-deterministic generator. Every one was found by the next
question rather than by a check, and two were found only by diffing two runs by hand. The acceptance
programs are the only thing that ties a percentage to a running program, and **nothing runs them**:
they are recipes in a README.

**2. A NUMBER NOBODY MEASURED WAS REPEATED THREE TIMES.** "675 of 1,610 struct-by-value refusals are
enums the tool misreads" is in assessment-2026-09-09, products.md §10 and CLAUDE.md. No survey output
has ever printed it and no result document records it. The mechanism behind it is real — the struct
table's regex takes the name after **any** closing brace, and the SDK has 5,178 `typedef enum`s — but
the count is not a measurement. It is exactly what this repository's first rule is about: *every
design claim that was not measured has been wrong about half the time.* Measuring it is item 1.

**3. A WITNESS THAT CANNOT FAIL PROVES NOTHING, and the Win32 acceptance program was one.** It called
`MulDiv(7, 6, 2)` and printed 21, and the result-extension bug cannot show on a positive value. The
containment harness learned this in August — *it had to be made to fail before it was trusted* — and
the lesson did not travel to the acceptance programs. Found today by checking the enum claim, which
meant reading how a result is emitted, which is the only reason it was found at all.

## 4. The risks, ranked

**1. GENERATED HOST BINDINGS ARE CLAIMS, AND ONE CLASS OF THEM WAS WRONG.** coercion-2026-09-09 wrote
the sentence and did not act on it: *"a template is checked only when some program happens to call
it."* 30,940 JVM, 4,773 Go, 8,809 Win32 and 2,326 JavaScript declarations exist, and the only
semantic check any of them has had is a handful of acceptance programs chosen by hand. The subsumption
edges are the one exception — every one was accepted by `go build` — and that is the pattern to
extend: **a generated declaration should be checked by the host that will run it, not by the next
program that happens to use it.** For the signed result the check is small and exact; for templates
in general it is a real design question, named here rather than answered.

**2. The survey tooling is untested and its numbers are quoted as findings.** §3.4 item 1.
Determinism (run twice, compare), agreement with the host (the acceptance programs as tests, with
inputs that can fail), and pinned counts would have caught four of the twelve corrections before they
were published.

**3. The balance of effort, back after one round away.** Six lines of programs. The corpus is still
where compiler bugs are found — the two tools found eight — and this round's one compiler bug was
found by a generator instead. The fix is the same as last time: **write a program**, and this time
write one that uses a host API through generated declarations on two hosts, which exercises the
tooling, the format and the language at once.

**4. `emit : core`, for the fifth round.** 4.13 : 1. The analysis layer held still; the growth was in
the emitters' multi-result and import paths, which is the right place for it. Watched, not alarming.

**5. Win32 struct by value.** The largest remaining Win32 refusal, 1,610 names at 13.9%, and its
enum share is unmeasured. *(Measured 2026-09-12: 140 names, 1.2%. It was never the largest — the
tool was reading pointer parameters as values, and the real ceiling on that host is a link line that
reaches 666 of 4,093 callable names.)* What the calling convention needs is known; what is missing is
construction, and that is the heterogeneous product's layout products.md §7 deferred — which Win32
and `GUID` now want.

## 5. What is next

**1. THE WIN32 ENUM FIX, WITH THE SIGNED-RESULT FIX, AS ONE CHANGE.** An enum is a C `int`, so making
enums declarable without extending results correctly would add hundreds of new wrong answers to the
ones that exist. Measure the enum share first and replace the unsourced 675 with a number; extend
every result by its C width and signedness; and keep an acceptance program **with a negative result**,
so the witness can fail.

> **DONE THE SAME DAY** — [win32enum-2026-09-11](../gauntlet/results/win32enum-2026-09-11.md). The
> enum share is **665**, not 675; struct by value falls **13.9% → 8.2%**, declarable **76.1% →
> 81.4%**, callable **34.8% → 36.8%**. Every result is read at its C width, `MulDiv(-7, 6, 2)` prints
> **−21** now, and `signed-result.oro` is the witness that printed the wrong answer first. And the fix
> found two more claims of the same kind: **36 floating-point entry points** declared through integer
> registers, and an arity ceiling that had quietly become stale.

**2. Tests for the survey tooling.** Determinism by running twice, the acceptance programs as tests,
and the published counts pinned — so that the next correction is found by a failing test rather than
by the next question.

> **DONE THE SAME DAY** — [tooling-2026-09-11](../gauntlet/results/tooling-2026-09-11.md),
> `gauntlet/stdlib/tooling_test.go`. Four properties, and **three failed on the first run**: the
> Go and Win32 REPORTS differed between identical runs (ties sorted in map order, eight sites); two
> published JVM figures — **5,186 edges and 5,446 overloads** — are produced by nothing, the tool
> printing 4,948 overloads and the relation being **10,095 edges**, the closure rather than the
> direct edges §2 of surveys-2026-09-10 claimed; and `os-methods.oro`'s recipe had never run since
> it was written and does not build. Each property was then made to fail against a bug this tooling
> really shipped. No compiler change.

**3. An application on two hosts, through generated declarations.** The corpus is six lines better
than it was, and the tooling has only ever been exercised by twenty-line harnesses. One real program
reaching a host API by generated declarations on two hosts tests all three layers at once, and it is
what *"write something awkward"* means now.

> **DONE THE SAME DAY** — [tally-2026-09-11](../gauntlet/results/tally-2026-09-11.md),
> `examples/tally`. A regex tally over a file: one 97-line core over six host operations, bound on
> Go (15 lines) and the JVM (19) by generated declarations only, agreeing with a hand-written
> reference on every input tried. It needed no new mechanism — the interface is a static-level
> argument reduction erases — and it found **five compiler bugs**, two of them exponential: a λ
> refused substitution, a residual whose hints capture (the witness is a Go compile error), a
> checker whose binder types outlived their bodies, an interval pass exponential in nested
> conditions, and an emitter walking every λ twice. The build went from not finishing to three
> seconds. And one host fact the JVM survey had wrong: a generated `String` result may be `null`.

**4. Then struct by value on Win32** — research first, starting from the measured enum share.

> **DONE 2026-09-12, AND THE PREMISE DISSOLVED** —
> [structval-2026-09-12](../gauntlet/results/structval-2026-09-12.md). The research was to classify
> 945 names by ABI class, and **805 of them are not struct arguments**: C binds `*` to the
> declarator, and `parseParams` took the last whitespace field as the parameter's name and threw the
> star away, so `SURFOBJ *pso` was a `SURFOBJ` **by value**. Struct by value is **140 names, 1.2%**,
> the third refusal rather than the first; declarable **81.4% → 90.5%**. **And the misread ran the
> other way too, which is the direction that matters**: `BOOL *pfOn` read as a word made an
> unbuildable pointer count as passable, so callable falls **36.8% → 35.4%**, and **585 of the 9,427
> generated declarations typed an address as an integer** — `GetDevicePowerState` was `(int int)`,
> kernel32 exports it, and the old declaration compiled a program handing it the literal 0. That is
> item 1 of §4 arriving with an instance. The enum decision also moved from a regex to matching the
> brace, after being wrong twice, and MSVC now confirms all 403 spellable enum names with a control
> that must be refused. **Of the 140 left, 84 refusals want one packed word and 56 want the
> heterogeneous layout, and the hidden-pointer return convention is worth exactly ONE entry point.**
> What the research found instead: **only 666 of 4,093 callable names are in a library the build
> links** — 16.3%, the hard-coded link line, and the largest move left on that host.

### Deliberately not next

**Another survey, or a larger one.** Four hosts are priced. A TypeScript-fed JavaScript survey, the
JVM's generic-class instantiation and Go's receiver auto-dereference are all real, and all would add
to the untested tooling before it has a single test.

**The SoA layout rule.** Measured and deferred on purpose: it waits for a real program with a product
table past cache.

**More gauntlet programs.** Seven, at parity; the corpus that needs growing is applications.

## 6. The one sentence

The thesis is now measured on four hosts and holds, and the core did not move to make it so — but
this round spent its effort on 3,048 lines of untested tooling whose numbers are quoted as findings,
wrote six lines of programs, repeated a figure nobody measured, and shipped a silent wrong answer in
every signed Win32 result that a witness chosen with positive inputs could never have caught; so the
next round should check what it generates against the host that will run it, give the tooling tests,
and write one program that uses what the surveys made declarable.
