# Assessment: two defects fixed in every program, two claims made true, and a word count over files

2026-10-24. Deliberately **not** an ADR. The previous fifteen are [2026-10-21](assessment-2026-10-21.md),
[2026-10-20](assessment-2026-10-20.md), [2026-10-14](assessment-2026-10-14.md),
[2026-10-03](assessment-2026-10-03.md), [2026-09-28](assessment-2026-09-28.md),
[2026-09-24](assessment-2026-09-24.md), [2026-09-23](assessment-2026-09-23.md),
[2026-09-17](assessment-2026-09-17.md), [2026-09-13](assessment-2026-09-13.md),
[2026-09-11](assessment-2026-09-11.md), [2026-09-09](assessment-2026-09-09.md),
[2026-09-06](assessment-2026-09-06.md), [2026-08-20](assessment-2026-08-20.md),
[2026-08-19](assessment-2026-08-19.md) and [2026-08-13](assessment-2026-08-13.md).

Written when the 10-21 plan finished, all but the items that wait on a trigger. Three days, **8
commits**, **7 ADRs** (0070–0076), 5 result documents.

> **Short answer: the plan's work is done, and the balance moved the wrong way.** The two defects every
> program had are fixed:
> - work stays where it was written (ADR 0071): β counts evaluations, not occurrences;
> - a declared result is a claim, decided at its definition (ADR 0072).
>
> Two of hamza's questions became decisions: a host header is the caller's view of a borrow (ADR 0070),
> and a set's name must denote a set (ADR 0073). One program, `wc`, brought in `unicode` and segment
> reads (ADR 0074, 0075). Writing it raised a question, may a `case` arm continue a loop, which hamza
> answered (ADR 0076).
>
> **The margin is 20.6 : 1** (1,590 lines of compiler against 77 of the language), against last round's
> 6.6 : 1. Five of the six phases were compiler work with no program: the fixes the last assessment
> asked for, which touch every program and add none.
>
> **Measured, for the first time, the cost of a language rule.** `wc` is at parity with hand-written Go
> of its own shape (1.01×), and 1.18× slower than idiomatic Go. 16% of that is the one flat loop the
> language forces, because an `again` cannot follow a nested loop's join point.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | **Work stays where it was written** | **Done** (ADR 0071, work-2026-10-22). β counts evaluations in {0, 1, ω}, a λ multiplying its body by its shot. Let-floating exposes a value, a residual let takes β's rule, and the IR reads a measured length as the length. 26 files changed, among them `freq`, which no longer runs a loop inside the loop that uses it. The gauntlet is unchanged within noise on Go, JS and Java |
| 2 | **A declared result range is an obligation** | **Done** (ADR 0072, resultclaim-2026-10-23). The contract ∀x ∈ P. φ ⟹ ⟦B⟧(x) ∈ R is decided at the definition, called or not, a tuple factor by factor. The plan's sketch, marks at the inlined sites, was rejected: it decides only the instances a program forms. 22 corpus claims are decided, all true |
| 3 | **The Go library, program first** | **Done** (wc-2026-10-24). `examples/os/wc.oro`, 39 operations, all proven: `unicode`, segment reads, a `where`'s bounds decided on the IR's intervals, a length keyed by what it denotes. It is checked against a count over the whole input at every straddle of the read |
| 4 | **The clause chain's copying**, when a program names it | **Waits**: no program paid for it |
| 5 | **The analysis grows only for a refusal a program names** | **Kept.** Two rules grew, each for `wc`, each with its planted fault: the length key, and the `where`'s bounds |
| 6 | **TinyRetroPad**: first a measurement | **Not started**; the download was approved at this assessment. §5 makes it a track |
| 7 | **A set's name denotes a set** (added 10-23) | **Done** (ADR 0073, setname-2026-10-23). A scope check against Γ_T. It found the other side of the defect: an alias-qualified host set was never resolved |
| 8 | **A π is justified by its branch** (added 10-23) | **Waits** for the IR's lowering to grow (§5 item 5) |

**Past the plan.** hamza's two questions on 10-22, why `hex.Encode`'s header named a product its call did
not, and whether the write-borrow's wrapper could go, became ADR 0070: a header in the caller's view,
the wrapper gone, and `shape`'s read loop lost a copy of its buffer on every edge. And `wc`'s question
became ADR 0076.

The round in six phases, counted with gauntlet/balance.py (§3.1):

| phase | commits | ADRs | compiler | Oroboros | hand declarations | survey tooling |
|---|---:|---:|---:|---:|---:|---:|
| **A. A header is the caller's view of a borrow** | 1 | 1 | +303 | 0 | +6 | +18 |
| **B. Work stays where it was written** | 1 | 1 | +384 | +2 | 0 | 0 |
| **C. A declared result is a claim** | 2 | 1 | +261 | 0 | 0 | 0 |
| **D. A set's name denotes a set** | 1 | 1 | +250 | 0 | 0 | 0 |
| **E. `wc`: `unicode`, segments, a `where`'s bounds** | 1 | 2 | +273 | +81 | +68 | +184 |
| **F. A case in a tail is a tail** | 1 | 1 | +119 | −6 | 0 | 0 |

(The README's rewrite in J opened the round and changed no count.)

## 2. What this round established

**A substitution's cost is a count in a semiring.** let x = e in E[x] and E[e] mean the same for a pure
e, and cost e times the number of times E evaluates its hole. The count is in {0, 1, ω}, with a λ
multiplying its body by its shot: Moran and Sands' improvement theory, GHC's OnceInLam. The count that
relevance needs is a different one, uses along a path (𝒰, abstract.md §8.1), over the same signature.

**A claim holds of a definition, not of its uses.** A header is f ∈ Π(x ∈ P). R. Checking it where the
definition is inlined decides the instances a program happens to form. Checking it at the definition
decides the statement. postconditions.md's "redundant" was true of the claim as an assumption and false
of it as a claim.

**A name in a claim is a scope question.** ⟦·⟧ on set expressions is a homomorphism, total exactly when
every atom is bound. An unbound one was an opaque type, the shape refinements.md §3a removed elsewhere.

**A decoder with bounded look-ahead makes chunking invisible.** `utf8.DecodeRune` reads at most 4 bytes,
so decoding only while 4 remain is the whole input's decoding, chunk by chunk. The transducer's state is
a tail of 3 bytes. Lines and bytes are homomorphisms; characters and words are sequential functions.

**A bound is the binder's set, whatever its spelling.** A host `where`'s constant bounds on one parameter
are decided as that parameter's range, on the IR's intervals too (ADR 0075). The first attempt, narrowing
the parameter's type, broke the hand check's planted mistakes: a narrowed parameter is, mechanically, a
transposed function.

**The tail sort is closed under a sum's eliminator.** An eliminator runs one arm once, so a case in a
tail whose arms are tails is a tail: the commuting conversion step ∘ case = case ∘ step. Nothing below
the reader changed.

## 3. Are we on the right track

### 3.1 The balance of effort

| | 2026-10-20 | 2026-10-21 | **2026-10-24** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 70,449 | 71,377 | **72,967** |
| of which tests | 23,206 | 23,532 | **23,871** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 1,869 | 2,009 | **2,086** |
| … in core-term nodes | 10,877 | 11,894 | **12,491** |
| **at the margin** | 78 : 1 | 6.6 : 1 | **20.6 : 1** (1,590 : 77) |
| overall | 37.7 : 1 | 35.5 : 1 | **35.0 : 1** |
| `emit` : `core` | 1.66 : 1 | 1.69 : 1 | **1.67 : 1** |
| (`emit` + `ir`) : `core` | 2.71 : 1 | 2.72 : 1 | **2.69 : 1** |
| analysis layer, extended | 5,808 | 6,039 | **6,065** |
| the IR's analysis | 3,342 | 3,342 | **3,350** |
| hand declarations (`targets/*.oro`) | 4,298 | 4,444 | **4,518** |
| survey tooling | 9,509 | 9,833 | **10,035** |

**Why the margin rose.** The plan put two whole-corpus fixes first, and each is compiler work no program
adds to: β's count (`core/work.go`), the claim at the definition, the scope check. Phases A to D are
1,198 lines of compiler and 2 of the language. Phase E alone, `wc`, is 273 : 81, about 3.4 : 1. **The
programs keep the margin down, and a round whose plan begins with fixes does not have them.**

**Where the compiler grew.** β's evaluation count and let-floating (+384). The caller's view of a borrow:
the reader's derived threaded result and the host primitive (+303). The claim at the definition (+261).
The scope check and the alias's resolution (+250). The length key, the `where`'s bounds and the segment
check (+273, with +184 in the survey's tooling). The tail case (+119).

### 3.2 Precision and performance

**Provability**: 2,103 of 2,148 operations and 353 of 376 loops. **45 operations and 23 loops unproven**:
last round's 45 and 21, and `wc`'s two read loops, which end when the file does. `smooth` lost one
operation, computing `len(a) − 2` once.

**Performance.**
- ADR 0071 changed 26 files, and the gauntlet's 9 regenerated files were timed old against new: Go
  0.99–1.00×, JS 0.99–1.02×, Java 1.00–1.01×, nothing slower beyond noise.
- ADR 0070 removed a closure, and `shape`'s buffer copy on every edge, at no measured cost.
- **`wc` is the round's measurement of the language itself**, on 100 MB:

| | ms | |
|---|---:|---|
| generated `wc` | 607 | |
| hand-written Go, the same flat shape | 601 | generated / it = **1.01** |
| hand-written Go, nested loops | 519 | flat / nested = **1.16** |
| hand-written Go, nested, no saturation | 513–516 | saturation = 1.03 |

The printer is at parity, and the program's shape is not. A nested rune loop would hand its counts back
as a tuple, and an `again` after a loop's join point is not built (tables.md §2.5). So one loop carries
16 variables where Go's nested loop keeps 5 in registers.

**Compile time** is flat: the sweeps read 0.99–1.15× the baseline, under the rule.

### 3.3 Soundness: the round's findings

| found | lived | how it was found |
|---|---|---|
| **An internal definition's declared result was dropped**: `bad` printed 11 | since ADR 0028 | last round; **closed** by ADR 0072 |
| **A tuple's factors were never claims**: a false carry, `0..0`, passed | since `sig` had several results | falsifying one in a copy of `lib/`, after the check passed the true ones. **Closed** |
| **β moved a pure value's work into a loop**: `freq` ran a digit loop inside the loop that used it | since call-by-need | last round; **closed** by ADR 0071 |
| **Lowering kept the first of a let's several values**, silently (`vs[:1]`) | latent since lowering; reachable from ADR 0071's let-floating | the verifier's W5 caught the ill-typed result on `lib/os`. **Closed**: refused |
| **A first version of the residual let's rule moved a table read past host calls** | never shipped | reading `ls`'s emitted diff. On `os.Args` it was harmless, on a buffer it is the swap miscompile. **Closed** before commit, with a witness |
| **A set name nothing declares was an opaque type**: Go with `[]/*Nonesuch?*/`. **An alias-qualified host set** printed `/*strings.Replacer?*/` at an export | since program headers | the plan's item; the alias was found measuring which names must stay legal. **Closed** |
| **A π on the wrong table passes the verifier** | since lowering | a planted fault in ADR 0071's length guard; only an exact-text witness caught it. **Open**, §5 item 5 |
| **A refusal test pinned a false refusal**: "an exit of 3i, up to 300", when the loop exits by i = 50 | since the test | building ADR 0075, computing the bound by hand. **Closed**: 6i is the refusal, 3i a control |
| **A narrowed parameter in the hand check admits a transposed function** | never shipped | the hand check's planted mistakes. Reverted |

**This round's soundness finds came from falsification**: a true claim made false in a copy, a planted
fault in each new rule, a bound computed by hand. None came from the gate, which compares a corpus of
true programs. **Make a check refuse something false before trusting that it passes something true.**

### 3.4 Process: five things

**1. A WRITE REPLACED A FILE OF TESTS.** `emit/claim_ir_test.go` held three tests from the observer round.
Writing ADR 0072's test as a "new" file replaced them, because the file had been read before a summary,
so the tool's read-before-write guard did not stop it. The full check passed without them: **a deleted
test fails nothing.** `git status` showing M, not ??, caught it. **`ls` a path before writing a file
meant to be new, and read `git status` before every commit.**

**2. A BACKUP RESTORED THE WRONG FILE.** `cp a $TEMP/x || cp a scratch/x`: the first copy succeeded, and
the restore read a week-old file from the second path. `git diff --stat` caught it within seconds, but a
running check built three differential cases in that window. **A planted fault backs up to one fresh,
unique path, and the restore is verified by a hash.**

**3. A SCRIPT NOTHING RAN WAS BROKEN FOR A WEEK.** `gauntlet/regen.sh` rewrote `(use go)`, the
s-expression spelling, so every port to JS failed after the J switch, and `gen_report_native.go` stayed
stale. A path nothing runs is a path nothing checks. §5 item 1 makes `cmd/check` run it.

**4. A REFUSAL TEST'S REASON WAS CHECKED AND FALSE.** The rule "a refusal test states its reason and is
checked for it" held: the reason was in the message. But the program was correct, and the test pinned
an imprecision as a property. **A refusal test's premise is computed by hand, not assumed.**

**5. THE FIRST FIX OF A ROUTE RELAXED A RELATION.** Narrowing `Exit`'s parameter type to reach the IR's
intervals weakened the hand check. Its planted mistakes caught it at once, and the route was changed
instead. The recurring lesson, *a relation relaxed for one check weakens every check that reads it*,
worked as a lesson should.

## 4. What is left, read off the outcomes

**45 unproven operations and 23 unproven loops.**

**Decided and not built**:
- `lent` (borrow-research.md, waiting for `Peek`);
- the arrow and callbacks tier 1 (ADR 0053);
- the rest of 0062's logic: lemmas, `assert`, `spec`;
- the inference of 0066;
- step 2 of types as values (0063);
- abstract.md §2.4's loop-arm patterns, ADR 0076's (b);
- `try`'s handlers, whose arms are tails by ADR 0076 item 5.

**Found and named this round**:
- an `again` after a nested loop's join point, measured at 16% (§3.2);
- a π's justification, unchecked (§3.3);
- a result claim on a definition over a function, decided per instance (ADR 0072 item 5);
- a static function reached as `f(x)(fn …)` taken for an eliminator, σ = 1, a precision gap (ADR 0071);
- `wc`'s body emitted twice, once per call site (code size);
- a `where`'s bound refused with the words "declared in".

**Carried**: the clause chain's copying; Houdini's 0 ≤ c (`e2`); freq's `slen`; ℤ/2ⁿ; a join across
representations.

**The documents** (hamza, 2026-10-24):
- j.md's status line says "specification, not built";
- state.md writes the core's s-expressions where the language is J;
- inventory.md lists `def` and `sig` as words of the surface. They are the core's forms, made by D from
  J's `fn` header;
- CLAUDE.md is 1,330 lines and 16,273 words, its "Where it stands" about 500 lines of history the
  assessments already hold.

## 5. The next plan

**The documents tell the truth, two surface questions are decided, then the Windows track begins with
its own question.** hamza's items and the round's, in an order where each step leaves the next one less
to rewrite.

1. **The documents tell the truth** (hamza's), and the gauntlet's files cannot rot again:
   - j.md's status: built, with what is not;
   - state.md in J, the core's terms shown as such where a spec needs them;
   - inventory.md: J's words. `def` and `sig` are the core's forms, made by D from a `fn` header, and are
     listed under the core;
   - `cmd/check` regenerates the gauntlet's 26 files from `regen.sh`'s list and compares them (§3.4.3).
2. **Two surface questions, researched and decided** (hamza's), each an ADR, the corpus moved and
   emission byte-identical:
   - **The unbounded range.** `` `+inf` `` is no integer, and its backquotes read as a host's name. The
     set is a ray, an up-set ↑lo = { x ∈ ℤ : x ≥ lo }. **Proposed: an omitted end, `lo..` and `..hi`
     (Rust's `RangeFrom`, Haskell's `[lo..]`), and `Nat` for `0..`, TLA+'s name.** ℤ is `Int`
     already. Six sites in `examples/big/` use it, and abstract.md and j.md say how a range is read.
   - **Visibility.** Private by default with `pub`, revisiting ADR 0064 items 4–5 with its "Why not" in
     view. That rejection weighed a forgotten `private` (silent) against a forgotten `export` (caught),
     and judged a leaked name harmless. A library's surface is a promise, and the Go library is growing.
     **`export` stays the entry point**: visibility and the entry point are two relations, a morphism
     of theories and a boundary where a claim crosses to the host, and ADR 0064's third "Why not"
     (making everything public made du's helpers entry points, so nothing built) is about merging them.
3. **CLAUDE.md rewritten as state** (hamza asked whether it is time: it is). It comes after 1 and 2, so it
   states the new surface once, and aims at a third of its length, with history reached by link. The
   assessments and results already hold the history.
4. **The Windows track, in steps** (hamza's):
   - **4a. MASM or LLVM IR, researched and decided** (an ADR). hamza's framing: LLVM IR reaches every
     major architecture (ARM, Linux, macOS) from one backend, while MASM needs a new target for each. On
     the other side, it may duplicate work and may reintroduce checks the type system made unnecessary.
     The project's rules add their own weights:
     - ADR 0001 and 0008: emit at the highest layer the target natively provides. LLVM is itself an
       ecosystem a backend parasitizes, as the JVM is, not an abstraction of ours;
     - what our IR already decides: registers by chordal colouring (x86's printer), ranges proven.
       LLVM's `nsw`, `nuw` and poison let a proven range *drop* a check, if the backend emits them;
     - the size standard: TinyRetroPad is 2,794 bytes, packed by Crinkler;
     - the toolchain a user needs;
     - and a measurement: the Windows sieve and one gauntlet program through each path, timed and sized.
   - **4b. TinyRetroPad's Win32 surface, measured** (downloaded 2026-10-24, commit `42f4c6c`, as data). It
     is 2,686 lines of 32-bit `stdcall` MASM, 30 procedures, and 67 Win32 functions over kernel32,
     user32, gdi32, comdlg32 and shell32, with RichEdit loaded at run time. The question: what is declared,
     callable and linkable today.
   - **4c. Its walls, spec first, an ADR each**: a window procedure, which is a callback the host calls
     (tier 2); its structs (`OPENFILENAME`, `CHOOSEFONT`, `PRINTDLG`), which are records; 32-bit against
     x86-64, which 4a decides.
5. **A π is justified by its branch, and the verifier checks it** (the 10-21 plan's item 8), before item 6
   grows lowering.
6. **An `again` after a nested loop's join point** (tables.md §2.5, ADR 0031's "not built"). `wc` puts it
   at 16% against idiomatic Go with the printer at parity. It would let a program nest its loops as Go
   does. Research it as a join point that may jump, build it, and measure `wc` nested.
7. **The Go library, program first**: a `cmp` comparing two files chunk by chunk, two buffers read by
   segment, more of `io`.
8. **The analysis grows only for a refusal a program names**, each rule's planted fault written with it.

**The measure of the round**: the margin back below 6.6 : 1. Items 1–3 change few compiler lines, and
items 4 and 7 are programs. No refusal test left whose premise was not computed.

**Kept as rules**: a refusal test states its reason and is checked for it, and its premise is computed;
an `-accept` that changes a count updates every document that quotes it; every experiment gets a time
limit; a check refuses something false before it is trusted to pass something true.

### Deliberately not next

- **ARM, Linux and macOS targets themselves**: 4a decides the direction, and a target follows a program.
- **Abstract.md §2.4's loop-arm patterns, and `try`'s handlers**: ADR 0076 covers what `wc` needed.
- **A result claim decided per instance**, and the σ gap for `f(x)(fn …)`: no program writes either.
- **The clause chain's copying, `e2`, `lent`**: carried, each until a program pays for it.
- **Carried with their reasons**: rendering the prover's fact log in J; hoisting or CSE on the IR;
  records, `with` and named views, beyond what TinyRetroPad's walls need.

## 6. The one sentence

The round fixed the two defects every program had and made two kinds of claim true. It wrote one
program, which measured for the first time what a rule of the language costs. The next round should make
the documents true, decide two questions of the surface, and start the Windows application with the
question of what it targets.
