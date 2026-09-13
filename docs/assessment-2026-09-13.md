# Assessment: the round an application was written, and the measurer was measured

2026-09-13. Deliberately **not** an ADR. The previous six are
[2026-09-11](assessment-2026-09-11.md), [2026-09-09](assessment-2026-09-09.md),
[2026-09-06](assessment-2026-09-06.md), [2026-08-20](assessment-2026-08-20.md),
[2026-08-19](assessment-2026-08-19.md) and [2026-08-13](assessment-2026-08-13.md).

Written because the last one's plan is finished: all four items are done, and the fifth thing it
produced — the link line — is done too.

> **Short answer: yes on the balance for the first time, yes on the thesis, and the survey was
> corrected more than the language grew.**
>
> **The balance is the best this repository has recorded.** **574 lines of compiler against 131 of
> Oroboros — 4.4 : 1 at the margin**, against 18.5 : 1 last round and 41 : 1 the round before, and
> all 131 are one application running on two hosts. `emit : core` moved the RIGHT way for the first
> time in six rounds, **4.13 → 4.03**, and the analysis layer grew 42 lines, both from fixes the
> application forced.
>
> **The application paid exactly the way three assessments said one would.** `tally` found **five
> compiler bugs, two of them exponential** — a build killed at 500 seconds now takes 3 — and every
> construct it used was already in the differential suite. What was new was the size.
>
> **And the measurements moved more than the language did.** In three days the Win32 survey's
> headline figures were corrected three times — enums, then pointer parameters, then the link line —
> and one direction was flattering: *callable* was 36.8% and is **31.0%** once it has to link. **585
> generated declarations typed an address as an integer.** Struct by value, which the last assessment
> put fourth on its plan as the largest remaining refusal, was **1.2%**: the plan was built on a
> number its author had not looked inside.
>
> **The tooling outgrew everything again**: +2,033 lines against the compiler's 574 — though 701 of
> them are its first tests, and much of the rest asks the host rather than answering for it.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | The Win32 enum fix with the signed-result fix, as one change | **Done the same day** — [win32enum](../gauntlet/results/win32enum-2026-09-11.md). The enum share is 665, not the unsourced 675; every result read at its C width; `signed-result.oro` the witness that printed the wrong answer first |
| 2 | Tests for the survey tooling | **Done** — [tooling](../gauntlet/results/tooling-2026-09-11.md). Four properties; **three failed on the first run**: reports nondeterministic at eight sites, two published JVM figures produced by nothing, a recipe that had never run |
| 3 | An application on two hosts through generated declarations | **Done** — [tally](../gauntlet/results/tally-2026-09-11.md). 97-line core, 15- and 19-line bindings, byte-identical to `grep \| sort \| uniq -c`; five compiler bugs; one host fact the JVM survey had wrong (a `String` result may be `null`) |
| 4 | Win32 struct by value, research first | **Done, and it dissolved** — [structval](../gauntlet/results/structval-2026-09-12.md). 805 of 945 names were pointer parameters the survey misread; 140 remain, 84 of them one register |
| — | (produced by 4) the link line computed from the declarations | **Done** — [linkline](../gauntlet/results/linkline-2026-09-13.md). 666 → 3,593 callable names that link; 155 refused because libraries bind them to different DLLs |

**Four of four, and a fifth.** Five results in three days, one application, one format addition
with its spec written first.

## 2. What this round established

**A program finds compiler bugs a construct suite cannot, and now it is measured rather than
argued.** Every construct `tally` uses was covered by the differential suite, which passed. The five
bugs needed size and one shape — a string read from a table handed to an impure host call: a λ refused
substitution, a residual whose binder names captured (Go's own compiler is the witness), a checker
whose binder types outlived their bodies, and two exponentials. `jsonfmt` and `freq` made the same
argument with eight bugs; this is the third time, and the first on generated declarations.

**An interface over a host can be a static-level argument that reduction erases.** `tally`'s core
takes its six host operations as a `(values …)` and holds a regex it has no type for. That is an ML
functor applied at compile time, and it needed no new mechanism.

**The survey is a measuring instrument, and it had to be calibrated against the host three times.**
MSVC now answers two of its questions — an aggregate's size, and whether a name it read as an enum is
an integer type, with a control that must be refused. The SDK's own libraries answer a third:
which DLL a name binds to, read from the short import objects rather than inferred from a library's
name or its DLL count.

**A generated declaration is a claim about two things, and the link line is the second.** Its TYPES
say what may be passed; its LIBRARY says which code answers. The first was wrong 585 times; the second
was a Go constant that reached 16.3% of the callable surface.

**A tie is information, not noise.** 155 callable names are bound to different DLLs by different
libraries — the print client and the spooler's own side, two JavaScript engines — and the headers do
not say which is meant. They are refused rather than guessed, and some are refused needlessly; none is
bound wrongly.

## 3. Are we on the right track

### 3.1 The balance of effort: the best recorded, and the reason is one program

Counted exactly as before: compiler is the `.go` files, tests included, in `core/` and `emit/`;
Oroboros is non-blank, non-comment `.oro` lines in `examples/` and `lib/`. Both reproduce the last
assessment's figures at its own revision (`af28756`): 37,037 and 1,382.

| | 2026-09-06 | 2026-09-09 | 2026-09-11 | **2026-09-13** |
|---|---:|---:|---:|---:|
| compiler (with tests) | 32,709 | 35,963 | 37,037 | **37,611** |
| Oroboros written | 1,049 | 1,324 | 1,382 | **1,513** |
| **at the margin** | 41 : 1 | 11.5 : 1 | 18.5 : 1 | **4.4 : 1** |
| overall | 31.2 : 1 | 27.1 : 1 | 26.8 : 1 | **24.9 : 1** |
| `emit` : `core` | 3.62 : 1 | 3.98 : 1 | 4.13 : 1 | **4.03 : 1** |
| analysis layer | | 5,175 | 5,175 | **5,217** |
| largest program | 112 | 155 | 154 | **154** |
| survey tooling | | 1,511 | 4,559 | **6,592** |

**All 131 lines of Oroboros are `tally`**: its core and two bindings. Eleven acceptance programs, 99
lines, sit outside the counted directories, four of them new this round.

**Of the compiler's 574, 269 are tests** — every bug fixed came with one that fails against HEAD. The
rest is `core/hygiene.go` (129), the checker's binder scoping, two interval-analysis fixes, the λ
exemption, and the link line's field, list and script (73). **`core` grew more than `emit` in
proportion**, which is why the ratio fell: the hygiene pass is a property of the residual and
belongs to the core.

**The survey tooling grew by 2,033**, more than the compiler and the language together, for the
second round running. 701 are its tests, which the last assessment asked for. Of the rest, a
substantial share is code that asks the host — a COFF archive reader, two MSVC probes, a sizes table
— which is the kind of survey code that cannot drift from the host. That is a mitigation and not an
answer: it is still the largest body of growth, and its numbers moved more this round than anything
it measures.

### 3.2 The thesis: holds, and the Win32 figure is now honest

| | callable | declarable | usable |
|---|---:|---:|---:|
| Go | 4,932 | 87.8% | **60.4%** |
| JVM | 38,042 | 81.4% | **60.9%** |
| Win32 | 11,575 | **90.5%** | **32.4%** — and **31.0%** callable *and linkable* |
| JavaScript | 2,340 | 100% | **not answerable** |

Win32's column had to be re-read three times. Declarable rose 76.1% → 90.5% as the survey stopped
misreading enums and pointer parameters; *callable* fell 36.8% → 35.4% for the same reason, since a
pointer read as a word had counted as passable; and **a callable name that cannot link was never
callable by a program**, which takes the comparable figure to 31.0%. Go and the JVM have no such
column because the host's own compiler resolves every import.

**The core did not move.** Seven term kinds, three reduction rules. The hygiene pass is a
post-condition on the residual, not a rule.

### 3.3 The gauntlet

Last benchmarked 2026-09-07, all seven at parity. This round changed the reducer, the checker, the
interval pass and both expression emitters, and the emitted corpus was swept: **175 of 178 files
byte-identical, the three being one renamed variable in `limbs.oro`**, which is not a gauntlet
program. The link line changed no emitted file, pinned by a test against the literal build script.
So parity holds by construction, **not by a run**, and that stays the right call only while the
sweeps keep coming back identical.

### 3.4 Process: four things, three of them mine

**1. THE PLAN WAS BUILT ON A NUMBER NOBODY HAD LOOKED INSIDE.** The last assessment put struct by
value fourth because it was *"the largest remaining Win32 refusal, 1,610 names at 13.9%"*. Two
thirds of that was the survey. The enum share had at least been named as unmeasured; the pointer
misread had not, and it was found only when the research printed *which* aggregates the names were
and `SURFOBJ` appeared 78 times. **A survey figure should not become a plan item until ten of its
members have been read one at a time.** The instrument for that now exists (`-sig`), and it exists
because of this.

**2. 48,518 GENERATED DECLARATIONS AND ELEVEN ACCEPTANCE PROGRAMS.** Go 4,773, the JVM 30,940,
Win32 10,479, JavaScript 2,326. **No survey has the host compile what it generates** — the one
exception is Go's subsumption edges, host-filtered since coercion-2026-09-09. Every correction this
round was found by writing something: an application, a witness, a research question. The 585
mistyped declarations are exactly what a host check exists for — though whether MSVC refuses them
is itself unmeasured: an integer where C expects a pointer is a warning in C and an error in C++, so
the check has to be built so that it fails. The last assessment's first criticism said exactly this and it is still the largest open
risk.

**3. A RULE APPLIED SIX TIMES AND WRITTEN DOWN NOWHERE.** *A generator does not make a claim it
cannot justify* removed `pure` from every generated Go line, withheld constructors from struct types
with no spellable field, refused floating-point entry points declared through integer registers,
refused unverified subsumption edges, and this round refused tied libraries and names reachable only
through an API set or bound to no DLL. It governs what 48,518
declarations say and it is a sentence repeated in `CLAUDE.md` and three result documents — not an
ADR, not a spec. This repository's convention is that a significant decision gets an ADR with its
"Why not", and this one has been made six times without one.

**4. ONE CLAIM I WROTE WAS FALSE, AND CHECKING IT BEFORE COMMITTING IS WHAT CAUGHT IT.** The link-line
result's first draft called the split between single-DLL and umbrella libraries *bimodal*; the full
histogram had twenty libraries in between, and the rule built on that claim also misread what an
umbrella binds. It was replaced by reading the binding itself before the commit, and the number moved
3,622 → 3,593. That is the process working rather than failing — recorded because the draft was
written before the histogram was printed, which is the order that produces wrong claims.

## 4. What this round points at

**1. Generated declarations are not checked by the host that will run them.** §3.4 item 2. The
probes exist on three hosts — `go build` on a generated file, MSVC on a generated translation unit,
and javac would take a generated class. A file calling every declaration with values of its declared
types, compiled and not run, checks the types; on Win32, linking it checks the library. **The
witness must fail against the bug that shipped**: revert the declarator fix, and the check must name
the 585.

**2. The generator's rule is a decision without an ADR.** §3.4 item 3. Cheap, owed, and the "Why
not" section is the valuable part — each of the six applications had a larger number available and
declined it.

**3. Windows has declarations and no application.** `tally` runs on Go and the JVM; the link line made
3,593 Win32 names reachable, and nothing written in this language uses one it did not already
reach. The last application found five compiler bugs. A Windows program will also meet the gap the
survey calls *"the call says it all"* — 32.4% usable, where an out-parameter or a returned buffer is
memory the program cannot read — and meeting it as a program rather than as a percentage is how the
last three format questions were answered.

**4. The survey is still the largest body of growth.** It needs no action beyond not growing for
its own sake; items 1 and 3 above are both acceptance-shaped rather than survey-shaped on purpose.

## 5. What is next

**1. CHECK EVERY GENERATED DECLARATION WITH ITS HOST.** Generate, alongside each target file, a
compile-only unit that calls every declaration with values of its declared types — Go and the JVM
through their compilers, Win32 through MSVC with the declared library on the link line. Report what
the host refuses; fix the generator or refuse the declaration. **Acceptance: with structval's
declarator fix reverted, the Win32 check names the 585; with the enum fix reverted, it names the
struct-by-value misreads.** A check that cannot fail against a bug this tooling really shipped proves
nothing.

**2. AN ADR FOR THE GENERATOR RULE**, with its six applications as the record and the larger number
each one declined as the "Why not".

**3. A WINDOWS APPLICATION THROUGH GENERATED DECLARATIONS.** Something awkward on the host with no
expressions, calling at least two libraries the target does not link by default. Its acceptance test
is the program, and what it cannot express is the next item.

### Deliberately not next

**Resolving the 155 ties from the documentation.** The DLL is in prose, and a survey that parses
prose is a survey that grows for its own sake.

**An API-set policy.** 61 names; a target decision nobody has asked for.

**Struct by value.** 140 names, 84 of them a packed word. It returns when a program needs a `POINT`.

**The 267 names no library lists**, the merge sort's termination gap, the SoA layout rule, another
survey, and re-benchmarking the gauntlet while the emitted corpus stays byte-identical.

## 6. The one sentence

The language finally grew faster than the compiler that serves it, because one application was
written and paid for itself with five compiler bugs; but the survey that sets this repository's
plans was corrected three times in three days on one host, a plan item evaporated on inspection, and
48,518 generated declarations are still checked by eleven programs — so the next round should have
each host check what the generator claims about it, write down the rule the generator has been
following, and write the application Windows does not yet have.
