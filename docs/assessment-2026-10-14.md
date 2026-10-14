# Assessment: a language specified twice over, and one program's clamps

2026-10-14. Deliberately **not** an ADR. The previous twelve are
[2026-10-03](assessment-2026-10-03.md), [2026-09-28](assessment-2026-09-28.md),
[2026-09-24](assessment-2026-09-24.md), [2026-09-23](assessment-2026-09-23.md),
[2026-09-17](assessment-2026-09-17.md), [2026-09-13](assessment-2026-09-13.md),
[2026-09-11](assessment-2026-09-11.md), [2026-09-09](assessment-2026-09-09.md),
[2026-09-06](assessment-2026-09-06.md), [2026-08-20](assessment-2026-08-20.md),
[2026-08-19](assessment-2026-08-19.md) and [2026-08-13](assessment-2026-08-13.md).

Written when hamza asked for it, after the derivation of item 2 finished. Eleven days, **96 commits**,
**26 ADRs** (0038–0063), 28 result documents and twelve research documents.

> **Short answer: the round asked its questions, answered most of them with mathematics, built the
> error model and the walls it unblocked, and then spent four days specifying the language twice, as
> an abstract syntax and as a surface, without a line of compiler or program. The balance is the worst
> it has been since the IR was built: 64 : 1 at the margin.**
>
> **Asked and answered.** The 10-03 plan began with a round of questions. Six were researched, and
> four were decided and built: the IR's text as a format (ADR 0038), the targets (0039), the error
> model (0040–0044), and the Go library's three closable walls (G, H, I: 0045–0048).
>
> **Reworked from first principles.** Type variables led to a question underneath them: what a type
> is. The answer was a logic above the computation whose types are sets (ADR 0049), an abstract
> syntax that is the initial algebra of the semantics (0050–0051), its first steps built, the Omega
> test measured beside today's entailment, and types as static values built far enough for
> `errors.AsType` (0063).
>
> **Specified.** The surface conversation ran five installments, from the λ to claims. It narrowed
> four notations to two, D and J, and decided eleven ADRs (0052–0062), none built.
>
> **Derived.** Item 2 was to build claims; the measurement said no program needs a written one, and
> every fact the corpus's clamps lacked was derivable. Five steps derived them; freq has one clamp
> left of a dozen sites.
>
> **The balance.** +10,557 lines of compiler against +166 of the language, 64 : 1, against 33 : 1 the
> round before. Two of the five phases changed no line of either.

---

## 1. What the last assessment asked for, scored

| | asked | happened |
|---|---|---|
| 1 | **A round of questions**, each answered with its set, operation, law and literature, a measurement where one decides, and an ADR where it settles one | **Done, and more.** Answered: the IR's text (ADR 0038: read, yes; written, once the IR proves indices). Researched, each with six decisions for hamza: concurrency, errors, Android, printing. Errors were decided and built (§2). The starting list's own items: callbacks (the arrow decided, ADR 0053, not built); one prover or two (settypes: one logic with a stack of procedures, ADR 0049); which Go package next (a coverage census by cause, gocoverage-2026-10-06). Not answered: a borrow a later call ends, ℤ/2ⁿ as a type, an unsatisfiable contract |
| 2 | **The Go standard library, package by package, program first** | **Partly.** The walls were closed program first: the error model (142 declarations depended on it), variadics over one type (G) and over `any` (H), out-parameters as cells (I), and `errors` with the first declaration over sets. Declared: 284 → **304 of 590** names in the ten packages counted (48.1% → 51.5%). The work the plan named after the walls, `os` (118 names) and `bufio`'s `Reader`, was not begun |
| 3 | **The Windows application** | **Waiting** on hamza |
| 4 | **The analysis grows only for a refusal a program names**, every rule with a soundness witness and a planted fault | **Kept.** Every rule of the derivation steps was demanded by a clamp in a program, and every one has a planted fault its test catches. The relevance hole and the shared-length bug were found by witnesses written for new rules (§3.3) |

**What the plan did not have.** Items 1 and 2 led, through type variables, to settypes, the
notations, the abstract syntax and the surface design. That chain was hamza's choice at each step, and
it is the round's largest part.

The round in five phases, cut at 7351790, 9daa71b, 5fe2f1f and 2a5ffc3 and counted with
gauntlet/balance.py (§3.1):

| phase | dates | commits | ADRs | compiler | of which tests | Oroboros | declarations | documents |
|---|---|---:|---:|---:|---:|---:|---:|---|
| **A. Questions**: the IR's text, the targets, four researches | 10-04 – 10-06 | 10 | 2 | +5 | 0 | 0 | 0 | four research documents |
| **B. The error model and the walls**: errors steps 1–6, G, H, I, `local`, cells | 10-06 – 10-09 | 30 | 9 | +5,666 | +1,690 | +168 | +175 | errors.md, local.md, variadic.md, two researches |
| **C. Types as sets, the abstract syntax**: typevars, settypes, three notation researches, abstract.md, steps 1a–3a (installments 1–2 interleaved) | 10-09 – 10-11 | 23 | 3 | +2,543 | +913 | 0 | 0 | six researches, abstract.md |
| **D. The surface**: installments 2–5 | 10-11 – 10-13 | 22 | 11 | 0 | 0 | 0 | 0 | surface-design.md, 42,427 words; two researches |
| **E. Built**: types as static values, the derivation, `local` tables | 10-13 – 10-14 | 11 | 1 | +2,343 | +759 | −2 | +58 | typevalues.md, refinements.md §3c, §5c–§5e |

## 2. What this round established

**A failure is a value of a marked sum** (ADR 0040, [errors.md](spec/errors.md)). The compiler knows
two roles, `success` and `relevant`, not a type. `try` is the exception monad's bind, a host's encoding
is a niche of its error type, and H, the host values behind a declared result, is a functor with r
natural in each factor, so one retraction covers a sum, a unit with an error and a partial success. A
crash is a value in 0, which absorbs every strict context. A relevant value is used on every path,
checked where a binder meets its value (ADR 0043); `ignore` is !_A : A → 1.

**A variadic is one function of a word** (ADR 0046): f : X × T* → B, Go's own call; Go's `any` is the
empty interface ∃X. X, a type every one-value type is below (0045).

**A cell is a loop variable** (ADR 0047, the state-passing translation, Schmidt 1985), and a host
writes one through a reference to a fresh copy, copy-in copy-out (0048): Ref τ ≅ Buffer τ 1.

**Two languages** (ADR 0049): a computation that reduces and prints, and a logic of sets that reasons
about it and never prints, ρ the one bridge. Types form a Boolean algebra; Theorem P makes a union
within a sort free and a union across sorts ADR 0042's tagged slots. The ceiling of automation is the
decided fragment. **The Omega test measured it**: today's entailment is sound on all 27,693 distinct
questions the corpus asks, and a complete procedure flips no refusal. Refusals lack facts, not proving
power.

**The abstract syntax is the initial algebra of the semantics** (ADR 0050): a notation is a rendering
with two round-trip laws, meanings are homomorphisms, and claims are inert because erasure is a
retraction. Built through step 3a: the reader builds the trees, D gives today's core term, every
source reads identically, and a definition is a named λ (0051).

**The surface's semantics is decided in eleven ADRs** (0052–0062): buffers and maps as state binders
in program order with syntactic control of interference (Reynolds 1978); `let` and `local` one scope,
a monoid action of binder words; a loop variable bound by application; one arm grammar; the handled
bind as case(e; p → b, ā); tags as quoted symbols; separation and replacement told apart by the
binder's place (Zermelo, Fraenkel); a header as the membership statement f ∈ Π(P).R. **The notation is
not**: D or J, one or two.

**A type is a static value** (ADR 0063): one namespace, a declaration over sets instantiated by
application at compile time, the bound as membership by subsumption, an instance identified by its
name.

**Every fact the corpus's clamps lacked was derivable** ([claimsdemand](../gauntlet/results/claimsdemand-2026-10-13.md)):
a disequality beside an order (strict over ℤ); translation invariants (Karr 1976); the first
iteration peeled (Mauborgne and Rival 2005); unit resolution on a failed conjunction; reads as atoms
and record facts, the array property fragment at a plain index over the curried table (Bradley, Manna
and Sipma 2006). No program needs a hand-written claim.

## 3. Are we on the right track

### 3.1 The balance of effort

Counted by **gauntlet/balance.py**, kept this time. It reproduces every figure of the 10-03 table at
its revision, 137e9c6, once "analysis, extended" is read as the layer and `fact`, `content` and
`component`, which is what 10-03 counted.

| | 2026-09-28 | 2026-10-03 | **2026-10-14** |
|---|---:|---:|---:|
| **compiler** (`core`, `emit`, `ir`, tests included) | 53,563 | 56,061 | **66,618** |
| of which tests | 19,000 | 20,034 | **23,396** |
| Oroboros written (`examples/`, `lib/`, lines of code) | 1,575 | 1,530 | **1,696** |
| **at the margin** | 504 : 1 | 33 : 1 | **64 : 1** (10,557 : 166) |
| overall | 34.0 : 1 | 36.6 : 1 | **39.3 : 1** |
| `emit` : `core` | 2.48 : 1 | 2.68 : 1 | **2.10 : 1** |
| (`emit` + `ir`) : `core` | 4.36 : 1 | 4.58 : 1 | **3.41 : 1** |
| analysis layer (`refine`, `linear`, `monotone`) | 3,530 | 3,877 | **4,399** |
| analysis layer, extended | 4,617 | 4,964 | **5,808** |
| the IR's analysis | 3,328 | 3,336 | **3,342** |
| largest program (`freq`) | 164 | 164 | **163** |
| acceptance programs | 409 | 414 | **530** |
| hand declarations (`targets/*.oro`) | 3,743 | 3,534 | **3,767** |
| survey tooling | 7,553 | 8,197 | **9,432** |

**Where the compiler grew.** `core` grew by half (10,049 → 15,112): the abstract syntax's trees, μ, D,
the declarations as trees, usage, the cell translation and the out-cells. `emit` +4,763: the error
model's retraction, variadics, the instances, the derivation. `ir` +731. The `emit` : `core` ratios
fell for the first time because `core` grew faster, which is the abstract syntax arriving where it
belongs.

**Where the language grew.** +166, all in phase B and the derivation: the error model's programs
(`errors/*`, `lines/ages.oro`, `io/count.oro`), and twenty fewer clamp lines in freq. The acceptance
programs grew 116, to 530 lines across twenty programs.

**And documents.** +17,005 lines under `docs/`: twelve research documents, five specs, and
surface-design.md at 42,427 words, longer than the whole of the language's code by a factor the
table does not show. Phases A, C (research half) and D wrote documents and nothing else.

**The largest program still has not grown.** `freq` has been the largest for four assessments, and
it shrank: a dozen clamp sites became one.

### 3.2 Precision and performance

**Provability is unchanged in what is left**: 2,039 of 2,084 operations and 351 of 369 loops, **45
and 18 unproven, the same ones** (§4). The denominators grew with the round's programs, each proven.

**The gauntlet was not re-run.** No printer changed this round; the last full run is
gauntlet-2026-09-29. One benchmark decided a change: deleting the tokenizer's stale clamp halved Go's
bounds checks there and measured 6% slower, interleaved, so the clamp stays.

**Compile time is flat after one regression was caught.** Reads as atoms first made freq compile 2.7
times slower and tree 1.9: the gate's pair confirmed four suspects. Restricting each rule to where it
can matter brought tree to 0.76 against 0.82 s and freq to about 7.0 against 6.7 s. The merged tree's
sweep reads 1.08×.

### 3.3 Soundness: the round's findings

Each has a witness that fails without its fix:

| found | lived since | how it was found |
|---|---|---|
| **Two filled tables' lengths were one variable**: the reader spells each fill's size binder alike, the length leaked it, and an index proven against the longer table read past the shorter, accepted and panicking on Go | the length rule, since fills existed | a gap found while writing record-fact witnesses; the session that fixed it saw the leak (localtab) |
| **A relevant value bound inside an iteration and dropped before `again` was accepted** | relevance, ADR 0043 | usage on the trees disagreed with the reducer (abstract2) |
| **A higher-order redex applying an effectful λ was weakened** | the effect discipline | a cell the host writes (outcells) |
| **A substitution of a value holding an index panicked the refinement layer** | the product pass | the sum-loop case (sumloop) |
| **A hand-off through a let-bound name escaped relevance** | relevance's first build | its own witness (relevance) |
| **An `again` of an enclosing loop was captured by an inner loop** in the cell translation | the translation | the `local` cases (local) |
| **A variadic's list was read as one value**: `strings.Join` gave a wrong answer | the loader | the variadic program (variadic) |
| **A refusal hid a wrong answer** in a sum's representation | sums at boundaries | removing the refusal (sumrep) |

Four of the eight were found by a program or a witness written for a new construct, two by a second
implementation disagreeing with the first, and none by an audit.

### 3.4 Process: five things

**1. DOCUMENTS RAN AHEAD OF PROGRAMS.** Phase D decided eleven ADRs by derivation and by token counts
over programs written for the purpose, and no notation has yet been used for a real program. CLAUDE.md:
*every design claim here that was not measured has been wrong about half the time.* The surface's
claims are measured, but on size and nesting; the property that decides a notation, whether programs
are written correctly in it, is unmeasured (syntax-research proposed the experiment, and it was not
run).

**2. A CENSUS COUNTED FROM THE WRONG ENTRY POINT.** The clamp census called merge-sort's clamp stale
because it compiled the case's export, which the sweep compiles, and not its `main`, which the
differential runner builds. It is CLAUDE.md's lesson, met again, and the census was first incomplete
as well (freq's `cli` by use site). Both were corrected before anything was built on them.

**3. A RULE APPLIED EVERYWHERE COST EVERYWHERE.** Reads as atoms made every guard on a read linear and
every join over one run; the first version was 2.7 times slower. The gate caught it, by a pair. Each
rule now says where it runs, in the spec.

**4. THE PROGRAM-FIRST RULE DECIDED AN ITEM AWAY.** Item 2 was proposed as claims; its own trigger, a
program a claim would let through, did not exist, and the measurement turned it into derivation. The
rule worked as written.

**5. TOOLING, AGAIN.** Heredocs ate quotes and a NUL byte; the balance script was kept this time
(`gauntlet/balance.py`), and it reproduces the last table.

## 4. What is left, read off the outcomes

**45 unproven operations and 18 unproven loops**, the same as at 10-03 (assessment-2026-10-03 §4).

**Built in part, and named:**
- freq's `slen` clamp: the record fact beg ≤ end in the IR's interval domain, which decides the
  report's allocation size.
- kara's `c4`: read over write; kara stays refused for `pack`'s geometric accumulation.
- Inference of a static argument, a program's own polymorphic `sig`, type sets, `errors.As`.
- Step 2 of types as values, the Boolean algebra of sorts, when a program states a set it cannot
  write.
- The JavaScript survey's Node pin (v26.8.1 here), which fails `cmd/check`'s tooling step.
- A read whose table is a loop's value is refused by lowering.

**Decided and not built: ADRs 0052–0062**, each "built with the surface": program-order stores (55
loop variables and 51 back-edge arguments the corpus spends carrying buffers), map buffers, one scope,
loop binding by application, one arm grammar, the handled bind, tags, formers, the header.

**Carried from 10-03**: a borrow a later call ends; callbacks tiers 1 and 2; ℤ/2ⁿ; an unsatisfiable
contract; a join across representations; a host `where` on the IR's intervals.

## 5. The next plan: the options weighed

hamza prefers to continue the surface design, installments 6 to 8, then move to the new syntax; I had
recommended the Go standard library first. The two orders are weighed here.

### 5.1 What each costs and buys

**S, the surface first.** Installments 6 (declarations, modules, variants, host declarations), 7
(static parameters, the arrow) and 8 (lexical); the choice of D or J, and of one notation or two;
then the reader for it and the corpus moved to it.

- *It is cheap to build, and the cost is measurable.* ADR 0050 makes a notation a rendering of
  abstract.md, and the reader already builds the trees (steps 1a–1c). A new surface is a parser into
  the same trees, J → Abs, beside μ : T → Abs, and D is unchanged. The migration is a translator
  T → J carrying every comment, and its correctness is the byte-identical gate over the whole corpus:
  ⟦τ(P)⟧ = ⟦P⟧ checked on 556 runs.
- *It unblocks eleven ADRs.* 0052–0062 are design debt that grows with every program written in T.
  Program-order stores alone remove 55 loop variables and 51 back-edge arguments: the programs of the
  next plan would be shorter and simpler written after them.
- *It fixes the notation before the corpus grows.* Every program written in T now is translated
  later; the translator makes that cheap, but programs are read by people, and the next programs
  should be read in the notation that stays.
- *Its risk is documents without programs* (§3.4.1). Installments 6–8 would add three more sections
  to a 42,000-word document whose only programs are written to be counted.

**G, the Go standard library first.** `os` (118 names), then `bufio`'s `Reader` (50) and `io` (27),
program first, in today's notation.

- *It is what the balance asks for*, and programs are what found most of the round's soundness bugs.
- *Its cost is the notation*: the programs are written in T with threaded buffers, then translated,
  and the ADRs that would simplify them wait longer.

### 5.2 The algebra says the order is sound either way; the measurement says how

Both orders end in the same place, because the notation is a homomorphic image of one abstract syntax
and the gate checks the translation exactly. So the choice is not about correctness. It is about which
measurement comes first, and the one missing is the notation's: **whether programs are written
correctly in D or J**. That is measured by writing real programs in them, which is also what the
balance needs.

**Recommended: S, with programs as its measurement.**
1. **Installments 6–8, each with one real program as its gauntlet**, not programs written to be
   counted: the `os` program the next package needs, written by hand in D and in J (it is first a
   declarations-and-modules program, installment 6's subject).
2. **D or J decided by an experiment, not by token counts**: the same programs written in both, by
   hamza and by a model, counting what the reader catches and what it does not (syntax-research's
   proposal). One notation or two is decided by the same data.
3. **The reader for the chosen notation, and the translator**, behind the byte-identical gate.
4. **ADRs 0052–0062 built one at a time**, each its own step behind the gate, program-order stores
   first (the largest saving, and the precondition of map buffers).
5. **Then the Go standard library resumes in the new notation**: `os`, already written in step 1.

This honors the preference for the surface and puts a program in every step, which is the balance's
remedy. Its cost: the gauntlet of step 1 is real work in a notation with no reader, checked by hand
until step 3.

### 5.3 Items that stay

- **The Windows application — hamza's**, when he hands it over.
- **The analysis grows only for a refusal a program names**, every rule with a soundness witness and a
  planted fault, planted before the witness is trusted.
- **The Node pin**, a task chip, before the next full `cmd/check`.

### Deliberately not next

- **More derivation for its own sake**: `slen`'s record fact in the IR, read over write for kara, each
  waits for a program the new notation makes.
- **Step 2 of types as values, inference, type sets**: no program states the set it would need.
- **Writing the notation's reader before 6–8 and the experiment decide it.**
- **Optimising the compile**: flat after the regression was cut.
- **Carried from 10-03 with their reasons**: host callbacks (tier 1 comes with the arrow, ADR 0053); a
  join across representations; a host `where` on the IR's intervals; hoisting or CSE on the IR; the
  limb instance as a function; geometric accumulation; per-result enforcement above the word; the
  higher-order contract gap; dependent result ranges; records, `with` and named views.

## 6. The one sentence

The round answered its questions with mathematics and built what they decided, then specified the
language twice without writing a program in either specification; the next round should finish the
surface by writing real programs in it, and let those programs choose the notation.
