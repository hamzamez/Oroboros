# CLAUDE.md

Guidance for Claude Code working in this repository. **It states where the project is, not how it got
there.** Each result has its document in [gauntlet/results/](gauntlet/results/), each round an
assessment in [docs/](docs/), each decision an ADR in [docs/decisions/](docs/decisions/); follow the
links for history. Update this file where the state changes, and never append a narrative. The long
versions are in git: `73004de` (a 60,000-word changelog), `b22d553` (17,300 words, 2026-10-26) and
`f0b5a62` (14,200 words, before the 2026-11-16 rewrite).

## What this project is

Oroboros is a small language and build system that **parasitizes** target ecosystems rather than
abstracting over them ([ADR 0001](docs/decisions/0001-parasite-model.md)). Portability is a property a
program may or may not have, and the compiler computes it.
- **One mechanism, a capability graph**: modules declare what they require, targets what they provide,
  and building covers the first with the second ([ADR 0002](docs/decisions/0002-capability-graph.md)).
  **Emit at the highest layer the target natively provides**, and lower only as far as necessary.
- **General-purpose** ([general-purpose.md](docs/general-purpose.md)). **The targets are Go, Windows,
  Android and the browser** ([ADR 0039](docs/decisions/0039-the-targets-are-go-windows-android-and-the-browser.md)):
  the JavaScript backend's host is the browser (Node only tests it as an engine), the Java backend's is
  Android, ART its runtime of record.
- **The atom** ([the-atom.md](docs/the-atom.md)): λ-calculus whose normal form is a parameter. A target
  partitions names into primitive and defined, and reduction runs until only primitives remain.
- **Two levels** ([closures-direction.md](docs/closures-direction.md)): a static level, unrestricted and
  higher-order, erased by staging; a dynamic level of first-order tables and loops.
- **Two languages** ([ADR 0049](docs/decisions/0049-types-are-sets-and-the-logic-never-prints.md)): a
  computation that reduces and prints, and a logic whose types are sets and whose claims are formulas,
  which never prints. The one bridge is ρ, a representation chosen from proven sets.

## Working rules (hamza's)

- **Always be mathematical and algebraic, and reach for the literature.** Before building or declaring
  anything, say what it is: the set, the operation, the law, the theorem that makes the design sound,
  where the literature answered it. A host package is an algebra before it is a list of functions.
  Derive first, measure second, and never substitute a list of cases for a derivation.
- **Follow the derivation where it leads, even into a rewrite.** Propose the better design, whatever it
  costs to build; never present the cheaper option as the recommendation when the algebra says
  otherwise. When the algebra decides a question already in front of hamza, take it.
- **Spec first.** Nothing enters the language without a specification of how it behaves on every
  target; every primitive is classified in [primitives.md](docs/spec/primitives.md), and a Tier 1 name
  needs a conformance case ([gauntlet/conformance/](gauntlet/conformance/)). The test is not "is it
  useful": what does it mean independently of a target; what does each target do; if they disagree, is it
  observable (then Tier 2, no portability claim)?
- **An ADR per significant decision**, from the template in [docs/decisions/README.md](docs/decisions/README.md);
  its "Why not" is the point. A reversal is a new ADR. A rejected alternative raised later joins the
  "Why not".
- **A result document per step**, `gauntlet/results/NAME-YYYY-MM-DD.md`: what was measured, what failed
  first, what it cost, what was not built. **An assessment when a plan finishes**, not an ADR: it scores
  the plan, measures the balance the same way every time, names process failures, and sets the next plan
  with a "deliberately not next" list.
- **README.md's "Current work" is the plan for a reader**: updated when a plan is set and when an item
  finishes, dated, and saying no more than the plan.
- **Emission is byte-identical by default.** `cmd/check` compares every source × target's emitted code,
  outcome, proof counts and refusal text against [gauntlet/check/](gauntlet/check/). A difference is kept
  only when correct and no slower, with `-accept "reason"` (which needs the compiler and differential
  steps passing in the same run, and is logged in `ACCEPTED.md`); otherwise fix the code. **An accept
  that changes a count updates every document quoting it.**
- **Compile time is gated** (gauntlet/check/README.md): 1.5× the baseline and 250 ms slower is a change
  for review, confirmed by a second sweep and a serial pair against `gen` built at the baseline's commit.
  To say how long one program compiles, time its binary serially and warm.
- **Every soundness rule gets a planted fault** that fails before its witness is trusted, the bug that
  shipped included; a refusal test states its reason, is checked for it, and computes its premise; a
  check refuses something false before it is trusted to pass something true.
- **Every experiment gets a time limit.**
- **Every step keeps a ledger** (hamza, 2026-11-15: "if we are going to pay a cost, I would like we
  know what we paid, and what for"): a result document's **Paid and bought** names compile time, run
  time, code size, proofs, refusals changed, compiler and language lines, emitted text changed per
  target, and any rule made less general, each measured or named as owed. An unmeasured cost is not
  zero.
- **An assessment at least every ten days or 30 commits**, whichever comes first, even when a plan
  has grown rather than finished (assessment-2026-11-15 §3.4).
- **Keep documents current.** A stale status line is worse than none; fix it when the thing ships.
- **The book moves with the language** (hamza, 2026-11-20): a change to the language (a construct, a
  word, a meaning, a refusal a program can meet) updates the chapter that teaches it in the same step,
  its examples written, run and checked by `docs/book/check.go`, and its "what is not here" made true
  again; where no chapter teaches it, write one or split one and renumber. Brick by brick: a chapter
  uses only what the chapters before it introduced, shows a refusal before the working form, and
  leans on examples and edge cases. Each defect writing it finds joins the findings index in
  `docs/book/README.md`.

## Primary documents

Read these before planning anything that touches types, claims or the surface. They stay here until
everything in them is built (hamza, 2026-10-10).

| document | decided by | built |
|---|---|---|
| [settypes-research.md](docs/settypes-research.md): types are sets, a Boolean algebra of static values; the decidability map (Presburger, regular strings, the array property fragment, BAPA, congruence closure), above it assertions and lemmas; own procedures, no SMT | ADR 0049 | the Omega test, `presburger/` ([presburger-2026-10-11](gauntlet/results/presburger-2026-10-11.md): refusals lack facts, not proof strength); types as static values, step 1 (ADR 0063); next: the algebra of sorts, when a program states a set it cannot write |
| [spec/abstract.md](docs/spec/abstract.md): the language as the initial algebra of a binding signature, its meanings homomorphisms (erasure ε, desugaring D, obligations 𝒪, usage 𝒰) | ADR 0050 | the trees and D (`core/abs.go`, `decl.go`, `desugar.go`); usage on trees; ε and 𝒪 wait for claims that carry sets |
| [fit-research.md](docs/fit-research.md): a syntax fits when its abstract syntax is the semantics' initial algebra; misfits are countable | ADR 0050, 0068 | J is the surface |
| [notation-research.md](docs/notation-research.md), [syntax-research.md](docs/syntax-research.md): the mathematical notation and six notations measured | ADR 0068 | J |
| [storeorder-research.md](docs/storeorder-research.md): program-order stores are the state-passing translation's image | ADR 0052, 0058 | built ([storeorder-2026-10-15](gauntlet/results/storeorder-2026-10-15.md)) |
| [arrow-research.md](docs/arrow-research.md): the arrow is a claim on a host's callback parameter | ADR 0053, 0133, 0143 | built at a host's callback parameter and field, on every target ([callbacks](gauntlet/results/callbacks-2026-11-20.md)); a function a host returns not |
| [concurrency-research.md](docs/concurrency-research.md): a process is a Mealy machine; a crash the exception effect, a link its propagation, a supervisor its handler; isolation a typing property, determinacy computed | ADR 0081 (the direction, C2 with C3 and C4 as fragments) | not built; §7's six questions and the surface answered when a program meets them |

## Where it stands

**Read first**: the primary documents above, [README.md](README.md), [design-direction.md](docs/design-direction.md)
(the founding argument; its §8 records how its questions were settled, concurrency's by ADR 0081), the
ADRs, [spec/state.md](docs/spec/state.md), and the current assessment. Measurements in
[gauntlet/results/](gauntlet/results/) are the authority: every design claim here not measured has been
wrong about half the time.

**The current assessment is [assessment-2026-11-20.md](docs/assessment-2026-11-20.md)**; read it
before planning. **The plan in force** (hamza, 2026-11-20: "yes, reorder", the logic first, since
TinyRetroPad's text rests on it and nothing in "Build all 5" blocks it), in order:
0. ~~**the check, shortened**~~ done ([checktime](gauntlet/results/checktime-2026-11-21.md)): the
   compiler step a schedule, its 926 tests by measured weight onto `-testprocs` processes by Graham's
   LPT, each group its own fault switches, every test checked to report; an `-accept` of a tree whose
   last run passed reuses that run's verdicts (the hash of what it read), running the emission step
   alone. A full run 62 → 13.8–15.4 min, an accept after it 165 s;
0′. **a set is a claim, and ρ alone chooses** (hamza, 2026-11-21: "1. E on Go and the JVM, yes.
   2. A an intermediate outside E yes 3. report is enough, a flag (-words is just a bonus option for
   the compiler)", on [sets-are-claims-research.md](docs/sets-are-claims-research.md), findings 108,
   116, 118), after item 0:
   - ~~E, the double word, on Go and the JVM~~ done (ADR 0149,
     [doubleword](gauntlet/results/doubleword-2026-11-21.md)): two digits by `math/bits` and by the
     JVM's carries, `Math.multiplyHigh` and Knuth's division; a signed quotient by a constant the
     magnitude's (on LLVM its unsigned division, `__divti3` before); finding 118 closed;
   - ~~A~~ done (ADR 0150, [exact](gauntlet/results/exact-2026-11-21.md)): no declared set
     consents; an operation past S, U and E that reaches no sink is ℤ, reported; a value flowing
     into a sink stays a word (the taint, the rung's closure backward); `-words` refuses instead;
   - ~~the book~~ done with E and A: chapter 6 §6.9 on S, U, E, ℤ through the five programs,
     chapters 7, 8 and 18;
   - **A's three open items** (hamza, 2026-11-21: "why don't we close the 3 open: start with the
     javascript one"): ~~JavaScript's pairs~~ done (ADR 0151,
     [pairsel](gauntlet/results/pairsel-2026-11-21.md)): a value of U a pair only where the pair
     computes its uses, a BigInt the rest; the pair compares and reads a word signed; finding 114
     closed, four differential cases on JavaScript; options 2 and 3 recorded for another time
     (hamza: "record 2 and 3 as optimizations for another time"). Then **JavaScript's three gaps**
     (hamza, 2026-11-21: "I agree, go in that order: 2, then 1, then 3"), the programs meant for
     every target JavaScript refused or stopped: ~~2, and, or and xor past 32 bits~~ done
     ([split53](gauntlet/results/split53-2026-11-21.md)): the split x = h·2³² + l, 4.6× BigInt;
     the differential's `// opaque`; ~~1, an entry point's parameter in U as a pair~~ done
     ([pairentry](gauntlet/results/pairentry-2026-11-21.md)): the caller's BigInt split once by
     `pair_bigint`'s inverse; ~~3, the one set checked at its claims~~ done (ADR 0152,
     [oneset](gauntlet/results/oneset-2026-11-21.md)): a fit at each claim above the word and at
     each operation the decision does not bound, none at a finite fact; fixed limbs left where an
     intermediate passes their width. Then (hamza, 2026-11-22: "1A then 2A"): ~~the report
     listing every operation~~ done ([report](gauntlet/results/report-2026-11-22.md)): D_A whole,
     a derived statement named in its source (`Of`); ~~2A, the double word as a loop
     variable's representation~~ done (ADR 0153, [eloop](gauntlet/results/eloop-2026-11-22.md)):
     a loop's variable and result carried as E's residue, a yield into ℤ its digits'
     recombination by Euclid's identity in radix 2³²; ~~the windows IR_P and JVM map-literal
     tasks~~ done ([keptnum](gauntlet/results/keptnum-2026-11-22.md)): a kept definition's
     numeral an integer at its callers, one on fixed limbs not kept; a map literal's entries cast
     into the JVM's carrier; ~~the book's finding 110~~ done (ADR 0154, hamza 2026-11-22: "A now",
     [sortclaim](gauntlet/results/sortclaim-2026-11-22.md)): a parameter's set that is no range a
     claim at the call, a where-mark on its result the type checker decides there; an unread
     one's false claim and a discarded call's contract refused now; ~~the book's finding 112~~ done
     (ADR 0155, hamza 2026-11-22: "yes, fix 112 now", [keptsink](gauntlet/results/keptsink-2026-11-22.md)):
     a kept definition's parameter no sink of the taint, so windows computes `grow` under `-checked`
     as every target does; the finding's pass order changes no program, not built;
1. **the logic, stages 1–3** (ADR 0145, decided L1–L7), each stage a program first and its book
   chapter:
   - ~~a research note~~ done ([logic-research.md](docs/logic-research.md));
   - ~~formulas and `assert`~~ done ([logic2](gauntlet/results/logic2-2026-11-20.md)): `∀ x ∈ S : φ`,
     `∃`, `⇒`, `⇔` and `assert`, decided by polarity (Skolem constants, Cooper's elimination) and the
     Omega test, divisions by constants exact; a definition's `ensure` relating its result to its
     parameters decided (`clamp`); the Gregorian leap year and Bloch's midpoint;
   - ~~the array property fragment~~ done ([logic3](gauntlet/results/logic3-2026-11-20.md)): a ∀
     in Γ instantiated over the index set (Theorem A), reads congruent, DPLL's search over the
     Omega test with Pugh's parallel rows; a formula invariant by Hoare's rule; a sorted table's
     ends, a table's maximum, binary search; L5, a host's `ensure` a formula believed with its
     sentence (`host-ensures-evidence.txt`), GetWindowTextA's text a C string, strnlen proven its
     count, on both windows triples;
2. **ADR 0136's struct forms**: a buffer field `= len(m)`, a nested record by pointer `&f`, an
   inline array `f[N]`, an optional C string `Option(String)` its NULL the niche; and **the memory
   model's decided parts** ([views-finish-research.md](docs/views-finish-research.md)):
   - ~~P3, a handle a host type~~ done ([handle](gauntlet/results/handle-2026-11-20.md), ADR 0146):
     `set HWND = host "HWND__*"`, `==` within a kind, a failed call's handle the declaration's
     `niche` (−1, NULL) of an `Option`, an `_In_opt_` handle an `Option` (held to the SDK's SAL);
     TinyRetroPad's save and load on both triples; a block's components given the strongest
     postcondition of its spine (finding 105); struct fields wait on ADR 0136's optional field;
   - two segments of one buffer lent at once when proven disjoint, wc's tail by RtlMoveMemory;
   - P4, a pointer the host keeps, the mode `kept` (hamza), §14.18's protocol over ADR 0133's slot;
     TinyRetroPad's find and replace;
   with TinyRetroPad's modules;
3. **T1-4, a buffer written in a λ under an ordered host**: designed, a borrow through the call
   returning the buffer's next version (callbacks-written §"Not built"); a research note, then the
   build, the program counting sort;
4. **the logic, stages 4–6**: congruence closure and the combination, with a comparator's law
   through a host's sort, one answer on every target; `spec` and recursive `lemma` (L3, L4), with a
   table's sum proven by induction; `-checked` running a bounded ∀ as a loop (L6);
5. **TinyRetroPad's body in J** (the 11-18 plan's item 1), module by module; then
   `lib/num/u128.oro` in the language's own operations (blocked on U₆₄ ↪ ℤ).

Done of "Build all 5" (2026-11-20): findings 100 and 101
([findings](gauntlet/results/findings-2026-11-20.md)); Win32's enumerator pages
([win32evidence](gauntlet/results/win32evidence-2026-11-20.md): none promises an order, all the
middle strength).

Before it, **the 11-18 plan (assessment-2026-11-18 §5), approved** (hamza, 2026-11-19:
"assessment's plan approved"):
1. TinyRetroPad's body in J, module by module, the window procedure last;
2. the callback decisions: tier 2's taken and built (ADR 0133); tier 1's to discuss with hamza;
3. ~~the capacity facts~~ done: mapvalue's refusals prove on HEAD as reconstructed
   (probes-2026-11-19), and the refusing program, a map read and then stored to, builds
   ([findings-2026-11-20](gauntlet/results/findings-2026-11-20.md));
4. ~~a timeout on every program a build test runs~~ done ([timeouts](gauntlet/results/timeouts-2026-11-19.md));
5. CLAUDE.md as state again;
6. the words audit: first pass done ([words-audit.md](docs/words-audit.md)); `inout` kept and
   `cell(T)` to be refused (ADR 0135); `jump`, `underlying`, `cdecl` kept for now (hamza).

~~**Ahead of the list below, the book's findings 100 and 101**~~ done
([findings-2026-11-20](gauntlet/results/findings-2026-11-20.md)), on every target: a map read and
then stored to (Collatz memoized, Two Sum), the 11-18 plan's item 3's refusing program; a literal
table of tuples (the shoelace formula) and of tables (Lo Shu) read at an unknown index. A table of
tables made by a rule, received or built stays storage's "a table inside a table".

**`Int` is ℤ at an entry point too, built** (ADR 0147, hamza 2026-11-20: "1. yes 2. decimal",
[intboundary](gauntlet/results/intboundary-2026-11-21.md)): an export's `Int` ρ of its set and
`where` (bounds by constants, by a table's length, through another parameter), the host's integer
of any size where unbounded, windows refusing one; a result declared `Int` demands each yield
leaving S; a host's word its module's `Word`; one rendering, `decimal`, `io.print_int` over ℤ; a
table's element and a map's value words by storage. **The midpoint, finding 108, built** (ADR 0148,
hamza 2026-11-21: "yes, I agree", [midpoint](gauntlet/results/midpoint-2026-11-21.md)): a program
asks, and can on every target (`sum ∈ Int = lo + hi`; a helper by a set above the word, its
`(Int)` no demand, finding 116); Bloch's
`lo + (hi - lo) / 2` under `where lo <= hi` proven in the words, a `where` a zone entry and a
repeated atom decided by the Omega test (Moore: the box is exact on independent atoms); the note's
§7 whole, silent exactness, not taken. Not built: a map read with a key of any size (finding 111,
waiting on a trigger).
A definition's claim reported inside its body (finding 110) is decided at the call since ADR 0154.

**Decided and to be built** (hamza, 2026-11-19; its items 1 and 3 are "Build all 5"'s 3 and 4), in this order (the first list's items 1–4, 6, 7 built:
ADR 0135, 0139, the pairs, windows32's tier, 0138, 0137; ADR 0140 and 0141 built since):
1. **the memory model finished** (ADR 0084, [views-finish-research.md](docs/views-finish-research.md)):
   - P5, SendMessageA's members, built (ADR 0141);
   - P3, handles, built (ADR 0146);
   - two segments of one buffer lent at once when proven disjoint, wc's tail by RtlMoveMemory;
   - P4, a pointer the host keeps, the mode `kept` (hamza), §14.18's protocol over ADR 0133's slot;
     TinyRetroPad's find and replace;
2. **`lib/num/u128.oro` in the language's own operations**: blocked on U₆₄ ↪ ℤ, the unsigned word
   widened into the rung above (u128-2026-11-19), four realizations owed;
3. **ADR 0136's struct forms**, with TinyRetroPad's modules (plan item 1).

**To discuss with hamza** (he will come back to them):
- **a scope form and P6's epoch** ([scopeforms-research.md](docs/scopeforms-research.md)): a view
  lent until a named call (GlobalLock … GlobalUnlock) on the declarations, and the trailing block
  `f(a…) { x̄ -> b }` as the user-defined scope form, a derived operator, not a macro; it also spells
  callbacks tier 1 at a call;
- **callbacks tier 1**: every decision made (ADR 0143, callbacks-sketches Part 3), **built on every
  target** ([callbacks](gauntlet/results/callbacks-2026-11-20.md),
  [callbacks-written](gauntlet/results/callbacks-written-2026-11-20.md)): a pure λ decided under its
  claim, W16, closure conversion into the context word on windows (a slot where the API has none);
  a written cell threaded under `ordered` or `calls 0..1` on each target (Go's frame reference,
  JavaScript's variable, the JVM's final holder, 1.6–1.7× a loop and the static slot rejected as a
  race, LLVM's alloca reached through E), every marker's host sentence kept and checked both ways; a
  definition of the program's applied to a λ that writes a cell expanded before the state translation
  (ADR 0144). Not built: a buffer written in the λ (T1-4, refused by R3; its design in the result).
  For hamza: the middle strength's word (each element once, no order: a bag's commutative monoid,
  formers-research §5), a law a callback obeys (a comparator's strict weak order, a lemma), a
  function a host returns (ADR 0053 item 5, a host type with application). A callback on another
  thread while the call blocks waits on concurrency (below, "Waiting on a trigger");
- **the variadic of places** (sendmessage-research §6): (c′) every variadic as its trailing binder,
  or (d) `Array(go.Value inout)`;
- **a table's element by its set on LLVM** (i16, i32 beside i8), a change to llvm.md §4's layout:
  the dot product in C is 0.74× at 16-bit elements ([cpu](gauntlet/results/cpu-2026-11-19.md));
- **`__chkstk`**: a function whose frame passes a page does not link on windows without the C
  runtime; the probe is an assembly routine, a wall (ADR 0123's question,
  [bigbits](gauntlet/results/bigbits-2026-11-19.md)).

**Decided, nothing to build**: D6 stays at the symbols LLVM calls (ADR 0123), a per-site `divl`
where a divisor is proven below 2³² an optimization when a measurement asks (hamza, "agree");
ADR 0133's late message runs on the state the scope left, by derivation (the host keeps the code
until unregistered, pointers-research §2.4).

The 11-17 plan (its §5), approved by hamza 2026-11-17 ("Go with the proposed plan"), ended with
items 1–5 done and item 6 in progress:
1. ~~**a compile that does not end is a refusal**~~ done, its premise reversed
   ([noend](gauntlet/results/noend-2026-11-17.md)): every loop of the compile has a measure, so it
   ends; the 20 minutes were windows32's `div` undecided and inlined (P4, right for the program).
   ADR 0035's "checked once" is now a check: every called K(ℕ\*) instance decided on every target it
   serves (`TestTheLibraryIsDecidedOnEveryTargetItServes`); refusing the program and bounding the
   work join ADR 0102's "Why not";
2. ~~**φ's `!range` on a load**, then D4's `divl`~~ measured, not built
   ([rangeload](gauntlet/results/rangeload-2026-11-17.md)): LLVM 23's i686 backend ignores `!range`
   and `range` for an i64's high half and reads `zext`/`and`; the representation form (`load i32`,
   one `mull`, D5's) buys 4% on 20000!, whose product alone is 8.3× Go/386. The cause is outside the
   multiply, owed to a measurement (allocation per step the candidate); `divl` a few percent, waiting;
— *added at hamza's word, ahead of item 3* ("Measure the allocation first", then "Build both fixes
   first"): ~~the allocation measured~~ ([alloc](gauntlet/results/alloc-2026-11-17.md): 20,000
   zeroed allocations and 50,921 page faults on windows32's 20000!); ~~the written prefix with a
   tail~~ ([filltail](gauntlet/results/filltail-2026-11-17.md), ADR 0097's proof extended);
   ~~a dead region across a call~~ ([reusecall](gauntlet/results/reusecall-2026-11-17.md), ADR
   0125): 22 allocations where there were 20,000;
3. ~~**D1 by header (W1) with TinyRetroPad 6c**~~ done
   ([tinyretropad](gauntlet/results/tinyretropad-2026-11-17.md),
   [byheader](gauntlet/results/byheader-2026-11-17.md), ADR 0126): of the 67 imports 47 are
   reachable, declared by header (61 declarations in 14 headers with kernel32's), each checked
   against its prototype and 17 called by a program on both triples; a data model is basetsd.h's
   four pointer-width names, and a C integer's set its type's whole value set (SIZE_T U₆₄ on LLP64);
   the other 20 are 6d's walls;
4. ~~**D5, i32 scalars**~~ done ([d5](gauntlet/results/d5-2026-11-17.md), ADR 0127): on windows32 a
   value proven in S₃₂ is an i32, ρ₃₂ read off IR_P's range by the printer, an operation at 32 bits
   where exact; every program measured 0.59–0.88×, the loop at the hand form's speed; x86-64 measured
   and not declared (the gauntlet's JSON 1.04–1.06×);
5. ~~**φ's remaining kinds**~~ measured, not built ([phi5](gauntlet/results/phi5-2026-11-17.md)):
   `memory(…)` is the join LLVM's `function-attrs` computes itself (26 of the 27 functions our
   proofs could speak for), `willreturn` from SCT reaches only exports with no caller in their
   module, `noundef` changes 3 of 105 files, neutral; kernel32's effects stated, neutral too. Each
   kind's theorem is kept for its trigger (a module boundary, LTO, a freeze that costs);
6. **TinyRetroPad 6d**: its walls, spec first, an ADR each (the window procedure: callbacks tier 2;
   its structs: records laid out by the data layout). Records decided
   ([records-research.md](docs/records-research.md), ADR 0128, hamza: "the whole record, and yes to
   r with {}"), being built. Step 1, the record in the language, built
   ([records](gauntlet/results/records-2026-11-17.md)): five cases on four targets, nested records
   and an export's record parameter, a buffer of records or tuples written in program order, and a
   record or tuple as a map's value ([mapvalue](gauntlet/results/mapvalue-2026-11-18.md): Go a
   struct, the JVM a record, JavaScript an array, windows n cells of a slot of 2 + n). Step 2a, the host
   struct ([structs](gauntlet/results/structs-2026-11-17.md)): `set RECT = record { … }` and `repr RECT c(…)` in its
   header, the layout Theorem L's fold checked against clang on both triples, a struct a host's
   cell, GetClientRect and GetLocalTime run on both; 2b built: determined fields (`= size`, `= 0`),
   the model's `pack(ILP32 : 1)`, `_In_` structs and C string fields, GlobalMemoryStatusEx,
   PrintDlgA and RegisterClassA run on both; a scalar `var` cell at a host's cell of a range, claimed
   in it ([vcell](gauntlet/results/vcell-2026-11-18.md)); a struct by value, the bare set as its
   prototype writes it and the call's `ref(x)` its const T* (ADR 0129, 0130,
   [sbv](gauntlet/results/sbv-2026-11-18.md), [ref](gauntlet/results/ref-2026-11-18.md), PtInRect
   on both); an `_Out_` a result and a (pointer, count) one segment, every C call written as C
   writes it (ADR 0131, 0132, [ccall](gauntlet/results/ccall-2026-11-18.md)); a buffer field with
   `= len(m)` and a nested record by pointer owed. **The window procedure built** (ADR 0133, hamza's
   decisions on [callbacks-sketches.md](docs/callbacks-sketches.md) Part 1): WNDPROC a named claim
   with its C signature, the λ at WNDCLASSA's field lifted behind a wrapper at that convention
   ([cbfield](gauntlet/results/cbfield-2026-11-19.md)); MSG and the message loop
   ([msgloop](gauntlet/results/msgloop-2026-11-19.md)); its state a `var` cell the λ captures and
   passes to a step's `s ∈ S inout`, the cell its static slot, Theorem T's global
   ([slot](gauntlet/results/slot-2026-11-19.md)), `examples/native/wndproc-win.oro` running a loop on
   both triples. A message after the cell's scope runs on the state it left, by derivation. Tier 1 researched ([callbacks-tier1-research.md](docs/callbacks-tier1-research.md)):
   a callback applied only during the call (105 of Go's 150) a region of the call, read as a loop
   whose trip count is the host's; its markers a usage in 𝕌 and an order, an unordered body pure
   (Theorem D); closure conversion into Win32's context word (291 of 349) or a slot; its 2.1 taken
   (L), the other five to be discussed. `wsprintfA` is no wall (two integer renderings, ADR 0095).

**Done in the 11-16 plan** (assessment-2026-11-17 §1): the book's wrong answers and faults
(ADR 0119–0121), j.md's two promises, two checks, the claims finding, CLAUDE.md as state; the
second triple to its arithmetic (ADR 0122–0124: [windows32](gauntlet/results/windows32-2026-11-17.md),
[d6](gauntlet/results/d6-2026-11-17.md), [d2](gauntlet/results/d2-2026-11-17.md); 72 of 74 on
i686); φ's four kinds (`range`, `samesign`, `nonnull`/`dereferenceable`/`align`, `noalias`),
neutral where measured. **Settled 2026-11-19**: windows32 second tier, D6 at the symbols, `urem` by a
constant as ADR 0138.

**A wrong answer, wherever found, goes ahead of the plan** (assessment-2026-11-16 §3.4), and the
book's findings index (`docs/book/README.md`) is read at each item's end. Found so:
~~80/81, a false claim accepted~~ done ([claims](gauntlet/results/claims-2026-11-16.md)).
~~93, `-checked` not trapping~~ done ([checked](gauntlet/results/checked-2026-11-18.md)): wider than
JavaScript (an unproven `<<` unchecked on Go and the JVM too); under `-checked` every operation is
proven, trapped or closed on the word (Lemma C), or the program is refused.
~~the JVM's `os.ReadAt` reporting a missing file as `other`~~ done, found by `cmp`
([cmpprog](gauntlet/results/cmpprog-2026-11-19.md)): it reads through `FileChannel`, whose `open`
throws `java.nio.file`'s exceptions, as the kinds read.
~~windows' `print_int` printing the low 20 digits of a value above the words~~ fail-stop
([print](gauntlet/results/print-2026-11-19.md)): 25! printed 10043330985984000000; the body's 20-digit
field now stops the program with its reason; total on ℤ waits on U₆₄ ↪ ℤ, as u128 does.
~~a λ bound to a name writing a cell~~ refused
([callbacks-derivation](gauntlet/results/callbacks-derivation-2026-11-20.md)):
`add = fn(x) { n := n + x }; add(1); add(2)` printed n as 0, and `get = fn() { n }` a stale value;
the state translation's let kept a λ argument for β without local.md §3's check.

**Owed from finished steps** (each named in its result document):
- split53: every differential case opaque (`// opaque`, inputs unknown at compile time): the
  windows and JVM failures fixed (keptnum-2026-11-22, the four big cases opaque and `map-built`
  new); quotient-corner, sum-slots, record-map and tuple-map prove only at their literal inputs
  (the last two a built map's value facts); the default stays literal until they are settled;
  the split at 26.7 ns against its eight operations; ir/final.go's bit refusal reached by no
  program on JavaScript;
- sortclaim (ADR 0154): a term a message quotes shown reduced (finding 110's other half); a claim's
  copy that does not reduce or shape alone left to the body (a record read through a table rule); a
  claim reduced to a literal after it was made kept to the checker; `normalizeStep`'s frame within 8
  bytes of Ω's stack at `MaxDepth` (HEAD's too); a range by an alias (`c ∈ error`) no argument mark;
- keptnum (hamza, 2026-11-22: "put them on the list, we will get to them in time"): record-map
  and tuple-map opaque, a map built in a loop carrying its values' facts to a read (`r.x * 3 +
  r.y` proven only at the literal inputs); a kept numeral's facts at its call, its post's
  interval (stein's `gcd` and popcount's `ones` still inlined by P4, the numeral ⊤ at the
  caller); a kept definition's parameter above the word; `TestAnIdentityCoercionMovesItsTable`
  failing once under load (`lang.cat`'s `utf8_valid`, the obligation `emit.PlantCatValid`
  turns off, which `TestATermsHostWhereIsDecided` sets in the same process), not reproduced;
- pairsel (ADR 0151): a residue divided by a non-power of two; the ascriptions on K(ℕ\*), where
  stein's compile did not end with them; a planted fault for SCT's reading of `the` as the identity;
  pairentry: the choice at an entry point by cost (≈ 25 ns a call at the boundary, a one-operation
  export 2–3× a BigInt's); oneset (ADR 0152): each claim at its own set's width, stricter than the one
  set, a candidate;
- handle (ADR 0146): a struct's handle field as `Option(K)` (ADR 0136's optional field); the
  results' niches no program has met (GetStdHandle's two sentinels a two-point niche); Windows'
  pseudo-handles (HWND_TOPMOST); an `Option` and a code crossing a join point together blew the
  reduction's step limit (lib/os's `open`, eliminated at its introduction instead); the book's
  finding 107, a wrong argument refused first by the facts;
- logic3: a C string from Win32 into a `String`, the code page's
  decode; reads over writes under a ∀ (an invariant over a buffer written in the loop); a
  quantifier with several binders; a read and a literal table printed as J writes them (finding 104);
- callbacks (ADR 0133): a slot's facts between two reads; the words audit's full pass ([words-audit.md](docs/words-audit.md): `inout`, `jump`,
  `underlying`, `cdecl` for hamza);
- ccall: a local buffer lent to a host in a scope that ends in further host statements is refused
  as a function term (R1), older than ADR 0131; a segment with no derived variant refused by
  the borrow's arity, not by a message naming the segment;
- invariant: an invariant over a buffer's elements or a cell's value;
- checks: a witness for per-loop widening thresholds (`PlantLoopThresholdFault` reaches no
  program, a constructed one included, [probes-2026-11-19](gauntlet/results/probes-2026-11-19.md)),
  the plant audit done (probes §3: four baselines, `PlantResultUnproven` witnessed by a hand-built
  instance, `PlantRecordLetWhole` witnessed by a record loop variable);
- capacity: presence facts, a borrowed map's capacity, an O(1) windows `len`; maprealize: the
  windows map 1.22× Go's (mapvalue's three refusals prove on HEAD as reconstructed, probes-2026-11-19,
  so they wait on a refusing program);
- store: the builder, K(ℕ\*)'s carrier as a set, the `abandon` arm in a borrowed buffer's code;
- strings (ADR 0117): ι_V as the identity, large string terms called as functions;
- P8 (ADR 0113): a segment at a kept boundary (`pe`), affine templates, instances for U, per-call
  fallback;
- harness: `u128-fact` and `u128-ops` run everywhere once `lib/num/u128.oro` is rewritten in the
  language (hamza, 2026-11-19), which waits on U₆₄ ↪ ℤ (u128-2026-11-19);
- u128: a constant above the word, unclaimed, under `-checked` reached Go's printer as an `int`
  (Lemma C's gap, no wrong answer); print: an exit into ∅ under a tuple pattern refused by
  `CheckJoins`' arity rule, which ADR 0090 makes compose (tried 2026-11-19: with the exit's
  projection itself, `CheckJoins` accepts it and the joined values' facts are lost, `a + b` ⊤
  where a tuple exit in its place proves it; not built);
- bigbits (ADR 0082 above the word, [bigbits](gauntlet/results/bigbits-2026-11-19.md)): a frame
  above a page does not link on windows (no `__chkstk` without the C runtime: ADR 0123's question,
  an assembly routine a wall for hamza); a kept definition returning a numeral typed as its table
  at the caller, so `decimal` of it inlines the program whole; a loop's counter bounded by its trip
  count in the body (`ones(1)`); the seven on the fixed limbs; a word mask on a host bignum
  (`y & 1` as `Bit(0)`); Stein's loops' termination on numerals.

**The standing goal** (hamza) is **the Go standard library, package by package, program first**: a
program in the language that needs a package, its algebra, and exactly the declarations it calls,
checked. When a package hits a wall that needs language work, stop, research it, design it.

**The standing measure** (hamza, 2026-11-10): **the windows implementation is faster than the Go
one**; while it is slower the gap is work owed. It is slower on every K(ℕ\*) program but three (fact
1.31×, fib(10⁵) 1.14× `math/big`; the 2,000-quotient program 0.79×, a numeral by a word 0.94×;
[radix-2026-11-15](gauntlet/results/radix-2026-11-15.md); popcount over 3^4900 … 3^4999 0.64×,
[bigbits](gauntlet/results/bigbits-2026-11-19.md)). Per operation ours is 4.2× Go's product
and 3.3× its square at 10,400 words, the leaf row 1.6× Go's assembly ([kernels-research.md](docs/kernels-research.md)
§7). **Decided** (hamza 2026-11-15): a target may name its CPU and features
(built, ADR 0142: `windows-v3`, the integer dot product 0.60–0.69× the baseline,
[cpu](gauntlet/results/cpu-2026-11-19.md)); a kernel is written in
the language, through LLVM IR, assembly only if a measurement still asks. **Postponed**, in order:
subtractive in-place Karatsuba with the threshold re-measured, the leaf as LLVM's wide product,
Toom-3, a subquadratic division at a node (Newton and Barrett, or Burnikel and Ziegler), printing's
remaining 5–6× against Go's `String`.

**Open from the 10-24 plan** ([assessment-2026-10-24](docs/assessment-2026-10-24.md) §5, the rest done):
- ~~**an `again` after a nested loop's join point**~~ done (ADR 0139,
  [joinlet](gauntlet/results/joinlet-2026-11-19.md)): nested `wc` 1.013× nested Go, where flat was 16%
  ([wc-2026-10-24](gauntlet/results/wc-2026-10-24.md));
- ~~**the Go library, program first**: `cmp`, two files compared chunk by chunk~~ done
  ([cmpprog](gauntlet/results/cmpprog-2026-11-19.md)), one output on four hosts;
- **the analysis grows only for a refusal a program names**, each rule's planted fault with it;
- ~~**the JVM's generated JSON tokenizer slows from 8 to 22.5 µs mid-run**~~ found
  ([jsontok-2026-11-19](gauntlet/results/jsontok-2026-11-19.md)): the process's age, the
  hand-written tokenizer as slow at the same moment; whether the gauntlet's ratios were measured in
  one window is owed;

**Deliberately not next**: ARM, Linux and macOS targets themselves; loop-arm patterns and `try`'s
handlers; a result claim per instance; the clause chain's copying, `e2`, `lent`; optimizing the compile;
a join across representations; hoisting or CSE on the IR (it would reach two premises witnessed only on
hand-built IR); per-result enforcement above the word and signed limbs (ADR 0029); the higher-order
contract gap (ADR 0028); named views (records and `with` decided whole by ADR 0128).

**Waiting on a trigger** (researched, built when a program needs it):
- **numbers' exact interiors** (numbers.md §5): a term of N held exactly and rounded once; trigger a
  correctly rounded sum or dot product, or an exact geometric predicate (floats-research §18);
- **numbers' loose ends** (doubles-2026-11-01, grids-2026-11-01): the host's double as its own type
  and `num.of_double`; N's subsets at the checker's boundaries; `F(p, emin, emax)` as a set a program
  names; `sqrt` and fma as realizations, the directed modes, `-checked` for a rounding; F and D's
  definitions in `lib/num.oro` (ADR 0061's replacement sets); each its trigger in its result;
- **Win32's strings, the rest of ADR 0091**: a C string from Win32 into ours (trigger
  `GetWindowTextA`), a list of C strings (trigger the open dialog's filter), the `W` functions'
  UTF-16 (trigger a `W` function);
- **one integer rendering on every target** (ADR 0095): since ADR 0147 a word is rendered by the
  host's own (`strconv.Itoa`, `Long.toString`, `String(n)`) and a value above it by its bignum, two
  renderings where the ADR has one; trigger printing's P3;
- **a map read with a key of any size** (the book's finding 111, hamza 2026-11-22: "add it under
  waiting on a trigger"): refused as an operand held above the word, a correct program; every key
  enters a map as a word, so dom(m) ⊆ S and m(k) = m(ρ(k)) if k ∈ S, else 'none: a read is no sink
  for its key (an insert stays one). The build: the rung pass branches a read whose key is held
  above the word on its fit, the key's π under the branch the word by ρ, a conversion from the
  big integer to the word on each target (whether ρ narrows at a π is the first measurement); an
  ADR amending 0147/0150, its planted fault the branch dropped; chapter 14's `lookup` then takes
  `k ∈ Int`; trigger a program reading a map at a key computed past the word;
- **a constant past JavaScript's word spelled as a BigInt literal** (keptsink-2026-11-22): 2⁶² is
  held exactly since midpoint-2026-11-21, written as Horner's form over 10¹⁵ in BigInt
  (`BigInt(4611) * BigInt(10**15) + …`) where `4611686018427387904n` is the same value; cosmetic,
  trigger a measurement of a loop reading such a constant;
- **printing an integer on Go** (intboundary-2026-11-21): `strconv.Itoa` then `fmt.Println(s)`
  where it was `fmt.Println(n)`, 52 ns against 36 into `io.Discard`, inside the noise into a file
  (the write's system call); trigger a program printing in a measured loop;
- **`print_int`'s arm above the word on windows** (intboundary-2026-11-21): every program that
  prints carries it, dead where the argument is a word, which LLVM removes (`maximum`'s `.ll`
  1,119 → 3,021 lines, its binary 4,096 bytes before and after); the cure an instance of
  `print_int` by its argument's set (ADR 0101); trigger a compile-time measurement naming it;
- **storage, the rest of R1** (ADR 0096, 0097, 0100): the helpers' strings, a table inside a table,
  frame slots for a nested loop's first iteration, a token across a branch; trigger input-driven
  programs on windows, or a measurement naming one;
- **a 32-bit scalar on x86-64** (ADR 0127 item 5): built for i686 only; `wc`'s 64-bit lanes
  (files-2026-11-16, 1.43× C's work) are a vector's width (ADR 0085), and a scalar by use, never an
  address, is the measured candidate; trigger a program the gauntlet's JSON does not slow; a second
  candidate since, `idot`'s tables of 0..65535 in i64 elements: ours is C's at i64 and both lose
  to Go's scalar loop (SSE2 has no 64-bit lane product), and C at 16-bit elements is 0.74×. A table
  element by its set on LLVM changes llvm.md §4's layout: for hamza
  ([cpu](gauntlet/results/cpu-2026-11-19.md));
- **JavaScript's pair, the rest** (ADR 0151's "Why not", hamza 2026-11-21: "optimizations for
  another time"): short division by a constant below 2²¹ on the digits (4.3× BigInt on
  u64repr-2026-09-22's kernel, 1.6× on pairsel's digit sum), a variable shift, a build's region;
  and E on JavaScript as four 32-bit digits, BigInt today; trigger
  a program the selection sends to ℤ and a measurement that asks;
- **U₃₂ on i686** (ADR 0127's "Why not"): K(ℕ\*)'s digit in an i32 read unsigned; trigger the next
  measurement of windows32's big-integer gap;
- **integers' four old mechanisms as (H, d) data** (ADR 0083 item 1); trigger the next change to any
  of them, or a second string encoding;
- **refinement sorts for `strings` over V ⊂ B\*** (Freeman and Pfenning 1991); trigger a program
  paying for `go.text` right after a `strings` call;
- **formers** ([formers-research.md](docs/formers-research.md), hamza 2026-11-20: "let see the demand
  for it"): a data structure's literal `F { a, b, c }` as its free algebra's term, comprehensions by
  the Boom hierarchy's laws, block formers as handlers run in staging, the trailing block
  `f(a…) { x̄ -> b }` the first step; F1–F5 decided when a program asks;
- **an integer of any size at a Windows export** (the book's §18.10 and finding 113, hamza
  2026-11-21: "leave it this for now until there is demand"): a parameter refused, a result `Int`
  past the word refused, a result `Nat` returned as the program's own numeral; the boundary format,
  a C struct of digits and count or decimal text, decided both ways at once; trigger a library that
  hands such integers to C callers;
- **a callback on another thread while the call blocks** (Go's `T.Run`, the JVM's parallel streams:
  `t.Parallel` lets f outlive the call, `FailNow`'s exit into ∅ skips the cell's store), kept until
  concurrency is in the language (hamza, 2026-11-20);
- **concurrency's** six questions (ADR 0081), **Android's toolkit**'s six decisions
  ([android-research.md](docs/android-research.md)), **printing**'s three algebras
  ([printing-research.md](docs/printing-research.md)), a borrow a later call ends
  ([borrow-research.md](docs/borrow-research.md)), §1.4's reservation of J's words.

### What is measured

- **The gauntlet** ([docs/gauntlet.md](docs/gauntlet.md)), the one fixed commitment: seven programs at
  parity with hand-written code. Last full benchmark [gauntlet-2026-11-16](gauntlet/results/gauntlet-2026-11-16.md),
  on four targets: geometric means Go 1.001, JavaScript 0.965, Java 1.012, windows 0.978 (against
  hand-written C by the same clang), each residue above 1.05× explained. Windows runs five of the
  seven (G4 and G3's tally need a dictionary keyed by strings, G5 F64's rendering: walls). The 33
  files its harnesses call are listed in `gauntlet/regen.txt`, with windows32's runtime
  (`lib/rt/i686.ll`, ADR 0123), written by `bash gauntlet/regen.sh` and regenerated and compared by
  `cmd/check`'s `gauntlet` step; windows' harness is `gauntlet/windows/`.
- **Provability**: **12,595 of 12,649** integer operations proven inside their target's word, the 54 left
  meant to be refused (`examples/int/`, `kara`) or in a case whose `run` takes an unbounded parameter
  (`shift-div`, `limb-subdiv`) or a host map's count (`wordcount`); **1,588 of 1,889** loops proven to
  terminate (size-change termination; of the 301 left, ten are binary search's (`examples/logic/bsearch.oro`), whose measure hi − lo shrinks from both ends, two are Collatz's iteration (`examples/map/collatz.oro`, the book's memo), bounded by a guard on its counter that size-change termination does not read, and 39 were before bigbits-2026-11-19 (sieve-js's two proven since intboundary-2026-11-21), among
  them `wc`'s and `wc-nested`'s read loops and fourteen windows `os` cell loops whose progress is a
  count the host returns; 250 are Stein's and popcount's loops over numerals (stein's two on JavaScript proven since pairsel-2026-11-21), whose measures, a
  halving, a falling maximum, a bit cleared, size-change termination does not see, each counted
  per program and target). A definition windows keeps as a function
  (ADR 0113) is counted once per program, and K(ℕ\*)'s instances (ADR 0101) once per target; a
  program refused counts nothing (the logic midpoint on JavaScript counted since midpoint-2026-11-21, its 2⁶² held exactly). Both
  counts are pinned by `cmd/check`.
- **Host APIs** (surveys, declarable / usable): Go 87.8% / 60.5%, the JVM 81.4% / 60.9%, Win32 91.2%
  declarable and 30.6% callable and linkable by today's model, 35.7% callable, 46.6% if every struct were
  declared (12,377 entry points, survey-2026-11-19, a function pointer counted since callbacks-2026-11-20,
  a record under any of its typedef names since callbacks-written-2026-11-20, a handle a kind of its own
  since handle-2026-11-20 (an _Out_ pointer to one is no word's cell),
  an upper bound; the old model's 32.0% counted UTF-16 strings); `go run gauntlet/stdlib/coverage.go` gives the declared
  share of the Go packages with a file ([gocoverage-2026-10-06](gauntlet/results/gocoverage-2026-10-06.md)).
  **Supported** (declared by hand, checked against the host, exercised by a program, ADR 0022):
  `unicode/utf8`, `encoding/hex`, `strconv`, `encoding/binary`'s varints, `math/bits`, `io`'s
  interfaces; **partial and checked**: `os`, `strings`, `io`, `fmt`, `errors`, `io/fs`,
  `path/filepath`, `time`, `bufio` and its `Reader`, `unicode`; Win32 by header, 81 declarations in 18
  headers (ADR 0126, `TestWin32DeclarationsAreTheirPrototypes`). The portable `os` (`lib/os.oro`) runs
  on Go, JavaScript and Java with one output.
- **The balance**, named in every assessment: the compiler grows faster than the code written in the
  language. `python gauntlet/balance.py [--nodes] [REV…]` reproduces every assessment's table; the
  latest is [assessment-2026-11-16](docs/assessment-2026-11-16.md) §3.1 (overall 34.9 : 1). The
  largest program is still `freq`, 194 lines. **Write programs, and let them demand the analysis.**

## The compiler

`go run ./cmd/check` passes every step: vet, endings, compiler, emission, gauntlet, ir, assemble,
differential, book, tooling;
emission first and alone (its gate times compiles), the rest at once, about 14–15 minutes (the compiler step a schedule of 10 processes, `cmd/check/groups.go`, and the book the two poles, 2026-11-21; an `-accept` of a tree whose last run passed reuses its verdicts, about 3 minutes), plus 7 s for
each suspect the compile-time gate pairs (differential-2026-11-13).

**The pipeline.** J's reader (`core/jlex.go`, `jparse.go`, `jreadtarget.go`) builds abstract Oroboros's
trees, and D (`core/desugar.go`) makes the core term; `Load` resolves modules, expands `case`, `try` and
`expect`, and translates cells and buffers (S, the state translation, `core/cell.go`, `place.go`). The
β/δ reducer, call-by-need with an effect discipline, gives the residual: closed, first-order,
monomorphic. The type checker and the refinement layer run on it; then it is lowered to **the IR**
(ADR 0032, [spec/ir.md](docs/spec/ir.md)), structured SSA with π-parameters, which every analysis and
every backend reads.

**On the IR**: `ir.Decide` proves each operation inside its set (intervals over ℤ̄, thresholds, trip
bounds, the relational difference) and refuses the program or, under `-checked`, traps; size-change
termination (`ir/sct.go`); the representations, the unsigned word (`ir/words.go`, ADR 0033), the rung
above the word as a least fixed point of supply and demand (`ir/big.go`, ADR 0034, 0079), fusion under
`mod 2ⁿ` (`ir/fuse.go`, ADR 0082), a table released at its death on the LLVM layer (`ir/release.go`,
ADR 0096, W11 its check), widening with each loop's own thresholds (Blanchet et al. 2003:
its constants, the ends of what it reads from outside, its guards' images; `ir/interval.go`,
llvm-render-2026-11-07), the unsigned word as a pair of digits on a narrower word (`ir/pairs.go`,
ADR 0083), fixed limbs as
instances of a theory checked once (`ir/limbs.go`, ADR 0035; signed, Z(V_n), ADR 0098; of any length,
K(ℕ\*), ADR 0099), N's decimals as scaled words
(`ir/numbers.go`) and its doubles, each rounding onto F64 the call of a target declaration whose
`ensure` states it and its obligations decided on a real interval domain (`ir/doubles.go`, ADR 0086);
postconditions and a definition's claim
(`ir.CheckClaim`, ADR 0072). Each rule is checked against a concrete interpreter over ℤ
(`ir/interp_test.go`) and has a planted fault.

**The IR's text is the printers' whole input** (ADR 0038): `gen -ir` writes IR_P, and `cmd/check`
reprints every program from that text alone, byte for byte. Four printers, `ir/golang`, `ir/js`,
`ir/java` and `ir/llvm` (LLVM IR, ADR 0080, [llvm.md](docs/spec/llvm.md): a join a slot that mem2reg
makes a φ, and a flag only where the IR proved it), sharing `ir/plan`. The term backends, the term
interval analysis and `ir/x86` (MASM) are deleted (irstep4a, irstep4j, mastretire-2026-11-03).

## The language today

**The surface is J** ([spec/j.md](docs/spec/j.md), ADR 0068; its status says what is built), and every
`.oro` file is J. [spec/state.md](docs/spec/state.md) is the language read off the code, checkable by
grep; [inventory.md](docs/spec/inventory.md) is every word the compiler knows, pinned by a test. A
parenthesized s-expression is a core term, which only the tests and the IR's text write. The other
specs' examples are in the notation J replaced, and each moves to J when it is next touched (hamza's
option (b), jswitch5b-2026-10-16).

### Core

- **Seven core term kinds**: name, integer, numeral (an exact element of ℤ[1/10], `core/num.go`, ADR 0086), string, boolean, `fn`, application. A residual's
  binders are named apart along the scope chain (ADR 0036), so a context keyed by name is sound.
- **A declaration is a header and one body** (ADR 0051, 0062, 0064): `fn f(x ∈ S) (R) where φ ensure ψ
  { e }`, D's `def` and `sig`. A constant is a value, `cap = 65536`. **A comment is `//`, a gap, never a
  token** (ADR 0024, 0067); a tool that rewrites source keeps every gap or refuses the edit.
- **A block binds sequentially**, `{ x = e; (a, b) = p; body }`, and a statement is a binding whose name
  is discarded (ADR 0054). `if c { a } else { b }`, `&&`, `||`, `!`, `!=` and `case { φ -> a; else -> b }`
  are D's (ADR 0017); a guard chain costs nothing ([cond-2026-09-19](gauntlet/results/cond-2026-09-19.md)).
- **Effects are a side condition on β** (ADR 0010, [effects.md](docs/spec/effects.md)): a purity bit per
  primitive, impure by default; an impure argument is let-bound, never substituted. **β counts
  evaluations in {0, 1, ω}** (ADR 0071): a pure argument is substituted when evaluated at most once.
- **Iteration is `loop x = z { φ -> again(…); else -> r }`** (ADR 0015). `again` is a jump in a tail: an
  arm's body, a block's end, a host call's continuation (ADR 0027), a `try` or `expect` success arm
  (ADR 0041), each arm of a `case` in a tail (ADR 0076), the body of a tuple pattern over a loop in a
  tail (ADR 0139); never under an `if`. In the reader the rule is a sort. Every walker of a clause chain
  walks all four forms, a host call's continuation and a pattern over a loop being one (`continuation`),
  or it is unsound. **No recursion**
  (ADR 0014); a balanced recursion is a loop over levels.
- **Relevance** (ADR 0043): a value of a `relevant` variant is used on every path, `ignore(e)` the
  discard. **Failures are values of a marked sum** (ADR 0040): `lib/result`, `try`, `expect`, the host's
  encoding a niche of its error type.
- **Closures** may not survive staging. A λ at a host's callback parameter is second-class (ADR
  0143): an `fn` statement whose one use is the call (W16), decided under its claim, pure unless the
  host is `ordered`, a written cell threaded by a reference; built on every target. A λ at a callback's field
  is the window procedure's (ADR 0133).
- **Modules**: a file is a theory named by its path (ADR 0064); `use p as a`; **`pub` marks the
  interface, private the default, `export` an entry point** (ADR 0078); **no module defines a name of
  Σ_L**, the language's signature (`core.LanguageSignature`, defects-2026-10-26).

### Data

- **Every data form is a function whose domain differs** ([data.md](docs/spec/data.md)). `Array(V)` on
  `[0, len)`, indexing `t[i]` application ([tables.md](docs/spec/tables.md)); a graph `array { 2, 3 }`
  and a rule `array { e : k ∈ 0..n-1 }`; `Map(Int, V)` with a declared capacity, a read an `Option`
  ([maps.md](docs/spec/maps.md)).
- **State is a binder of a block** (ADR 0047, 0052, 0058): `var b := array {…}` a buffer, `var m :=
  map(cap)` a map buffer, anything else a cell, written in program order (`b[i] := v`, `x := v`) with
  R0–R4 decided on each scope; S gives the core's threaded `build`; a cell becomes loop variables. A host
  writes a cell through a reference to a fresh copy (ADR 0048). A buffer parameter is `b ∈ Array(V)
  linear`, borrowed by reference; a host header is the caller's view of a borrow (ADR 0070); a pure host
  call observes a live buffer (ADR 0069), a segment `b[lo:hi]` its function restricted (ADR 0074).
- **A scope's value may be a product** of frozen buffers and values (ADR 0031), taken apart at a join
  point; an `again` after it is a back edge of the enclosing loop (ADR 0139). Tuples `(a, b)` and the unit `()`; a table of tuples is
  flattened by currying ([products.md](docs/spec/products.md)).
- **Variants**: `set S = 'a ∪ 'c(T)`, closed, finite, non-recursive, with type arguments; at a boundary its
  tag and one slot per payload type (ADR 0042, 0059).
- **Strings** are the free monoid over Unicode scalars, `++` and `string_of`; a host's string is the
  host's (`go.bytestring`), entered by one total decode, `go.text` (ADR 0030,
  [strings.md](docs/spec/strings.md)). **A C string is the kernel of ν** (ADR 0091, strings.md §9):
  `nulfree(s)` in Σ_L, a C-string parameter `where nulfree(s)`, decided by ν's homomorphism on the
  argument's term, and by a guard. **ι : (B₇)* ↪ Σ\*** (ADR 0095, strings.md §10): `ascii(b, lo, n)`,
  a table of scalars below 128 as text, total, one copy on every target, an observer of a live
  buffer by its meaning; what render ends in. **Not built**: symbols.
- **Records** (ADR 0128, data.md §4): `record { l ∈ S }` a set, `record { l : e }` a value, `r.l`,
  `r with { l.m : e }` an override along prefix-free paths, `p.l := e` in a cell, `zero(T)`. The
  reducer erases one by the lens laws and by currying at each binder (β, a let, a loop, a header);
  the residual places one stored as data by its shape (`core/record.go`, `recerase.go`).

### Integers

- **An `Int` is an integer** (ADR 0026), and a range `lo..hi` is a set; an end may be omitted, `lo..` and
  `..hi` (ADR 0077), and **`Nat`** is ↑0. Each target declares its word as data, `repr LO..HI word`:
  int64 on Go, the JVM and windows, ±(2⁵³−1) on JavaScript; Go realizes 0..2^64-1 as `uint64`, and windows as `i64` read unsigned.
  Legality is per (program, target), portability computed (`go run ./cmd/portable`).
- **Bounded by default** (ADR 0019): an operation not proven inside the word is refused, cleared by
  narrowing a range, a claim above the word, or `-checked`.
- **A declared set is a claim, and the facts choose the representation** (ADR 0079,
  [integers.md §1a](docs/spec/integers.md)): ρ of a value is its proven fact's, the word, the unsigned
  word, or the rung above; a claim above the word is the consent to that rung, and the representation
  itself only at an entry point's parameter or result. Above the word a target chooses `repr big host`
  or `repr big limbs`, and the program's one set is enforced on both, by sign and bit length (ADR 0029);
  a signed set on limbs is Z(V_n), a sign entry over the magnitude, the group completion (ADR 0098); an
  unbounded set on a target with no bignum is K(ℕ\*), numerals of any length (ADR 0099, `emit/natnum.oro`).
- **Staging never changes an answer** (ADR 0009): integers fold only inside the word, checked, never
  dividing by zero; floats fold through explicit `float64`.

### Types, claims and facts

- **The type checker** runs on the residual ([types.md](docs/spec/types.md)); the integer sort is ℤ, its
  realization the IR's (ADR 0033); `Table(σ)` has its introduction, elimination and store rules.
- **A header's sets are claims**: a parameter's set and `where` are obligations at every call a
  definition is inlined into, assumed at an export's boundary (ADR 0028); **a declared result is a claim
  decided at its definition**, exported or not (ADR 0072), by each end inside the word on ℤ's facts,
  whatever its representation (ADR 0087); every name used as a set must denote one
  (ADR 0073); a host `where`'s constant bounds are its parameter's range (ADR 0075). **A set has no
  representation**: a set operator in a computation or a claim's formula is refused saying what it is.
- **An obligation is discharged or the program is refused** ([refinements.md §3a](docs/spec/refinements.md)),
  by a literal, the IR's intervals, then the refinement layer (Fourier–Motzkin over the linear fragment,
  facts as a solved form). Every pipeline carries its obligations in the residual (ADR 0037).
- **Facts** are guarded boundedness axioms ([facts.md](docs/facts.md)); content facts are derived, never
  declared (Houdini invariants, store chains, strided components, record facts, translation invariants,
  a peeled first iteration). A clamp in a program is a missing proof.

### Declarations and targets

- **Declarations are theories** (ADR 0021, [theories.md](docs/spec/theories.md)): a module a theory, a
  target a model, `use`, `pub`, `export` and `provides` morphisms. **A target is a chain of layers**,
  glue within, override between ([target-system.md](docs/spec/target-system.md)).
- **A host module's path is the host's** (`go/encoding/hex`, ADR 0025), and the file is the path; a
  companion is a child module; a bare type name resolves lexically.
- **∅ is a set, and a function into it never returns** (ADR 0090): 0 is initial, so k ∘ h = h for
  every declaration whose result is ∅, as for `abandon`; a host operation into a product is taken
  apart by its projections (LLVM's aggregate, `extractvalue`).
- **A target states its realization of each language name** (ADR 0088), `realize OP = A, B checked C`:
  an interpretation ι : Σ_L → T(Σ_H), the first a host operation or **a term over host operations**,
  a definition the target makes, substituted after the decision (ADR 0089, `ir/realize.go`); each later
  one a host operation chosen per instance where the facts prove its domain. A host operation is
  declared as it is, its domain its `where`. Built for `windows`; Go, JavaScript and the JVM are found
  by spelling until restated.
- **Host declarations are written by hand**, by someone who read the host's source, and checked against
  it by the generator, which never claims what it cannot justify (ADR 0022, 0023;
  `gauntlet/stdlib/tooling_test.go`). `fn f(x ∈ S) (R) pure { host expr "F(${x})" import "p" }`; a
  variadic is one declaration over a table (ADR 0046); a host's empty interface is a type (ADR 0045).

## Constraints that override normal instincts

- **Anything promoted to the language works on every target, and the compiler finds the
  implementation.** A target may neither decline nor declare a language construct; the capability graph
  is for target-native names, where "this target cannot do it" is a true answer.
- **Never lower further than the target requires**: no hand-rolled hash table into Go, which has `map`.
- **Never assert which host construct is fastest: measure it** (ADR 0008). Treat host compilers as black
  boxes; `go build -gcflags="-m -m"` and `-gcflags="-d=ssa/check_bce/debug=1"` have each been decisive.
- **LLVM IR is a host like any other, and its target offers every primitive it has** (hamza,
  2026-10-27; ADR 0080 item 7): Σ_LLVM declared by hand, each operation's undefined behaviour its
  `where`. **A primitive the language cannot state is a wall, brought to hamza** before anything is
  built for it, and each wall is a step: a research document first, hamza's decision after.
- **Never make the core a superset of one host.** JavaScript has no integers or structs; the JVM no
  unsigned types, `goto` or tail calls; Go restricts `goto`.
- **Never introduce boxing or hidden allocation into the core**: that killed the predecessor
  ([design-direction.md §2](docs/design-direction.md)).
- **Never add unstructured control flow.**
- **Performance claims are measured against hand-written code**, never on less than the ~15% noise floor
  of the hybrid P/E-core laptop; carry the form expected to lose as well as the one expected to win.
- **Primitives are declared in `targets/*.oro`, not in Go.** Only structural constructs live in code.
- **Prefer deriving a fact over clamping around it or declaring it.**
- **The core is deliberately unspecified** ([core-candidates.md](docs/core-candidates.md)); the
  predecessor stalled on a fixed language. Beware the minimality trap: the core is minimal subject to
  lowering natively to every target at zero cost. Candidates are killed by measurement, not argument,
  and the derivations are in [docs/derivations/](docs/derivations/).

## Lessons that recur

Each has bitten more than once; the instances are in the results named.
- **A path nothing runs is a path nothing checks** (a `; skip:`, a backend no target used, an escape
  taken everywhere, a kind nothing has, a file regenerated by hand; bounds-2026-09-24, bufio-2026-10-01,
  docs-2026-10-25).
- **A harness that cannot fail proves nothing**: plant the fault first; refusal-shaped properties need an
  anti-vacuity guard; random search cannot test what over-approximation makes sound.
- **A relation relaxed for one check weakens every check that reads it** (W5 after ADR 0033). Audit
  every consumer.
- **"Not refused" is not "proven"**, and a check that runs only where a caller remembers to ask is no
  property of the program (noprop-2026-09-25, hazard-2026-10-02). State a precondition on the consumer.
- **An empty case read as "everything"**: an interface with no names showed every name (ADR 0078); a
  `where` over an unbound name was assumed at an entry point (defects-2026-10-26).
- **A comment stating what code does is not a review of whether it should**; a rule's comment can state
  a premise that is no longer true (`WiderThanWord`, claimset-2026-10-26). Only a planted fault shows
  whether a rule's conclusions are true.
- **A context keyed by name is captured by the name alone**, and `Body()` and `openFresh` rebuild terms:
  a map keyed by a term pointer answers about a copy (bufio-2026-10-01, localtab-2026-10-14).
- **The emitter must be a function of its input**: run it twice; it is reprinted from the IR's text.
- **A survey's first number describes the measurer**, and a count is a measurement too
  (matchguard-2026-09-24). Read ten members of a figure before it becomes a plan item.
- **Benchmark-method errors have many species**: a closure in the reference, a fixed iteration count,
  shared processes, eliminated work, two things changed at once, a modified input, comparing against
  HEAD after a regression, timing among parallel jobs, a stopped run still running, one megamorphic call
  site, a baseline built from `git stash`, a process's age (about 3 s in, any code on this hybrid
  laptop runs about 3× slower, jsontok-2026-11-19). Time a baseline in a worktree, and compare forms
  measured in the same window. Make a suspicious result explain
  itself before recording it.
- **A refusal can hide a wrong answer**, and **a program finds what a construct suite cannot**.
- **A test can pin a stale text**: `TestScopeCheck` required "fold-range" (defects-2026-10-26). Read
  what a test asserts before trusting it to guard a change.

## Decisions already made

Read the ADR, and its "Why not", before relitigating one.

| Decision | ADR |
|---|---|
| Targets are ecosystems; portability is a program property | [0001](docs/decisions/0001-parasite-model.md) |
| A capability graph, not a fixed layer tower | [0002](docs/decisions/0002-capability-graph.md) |
| Range-typed integers: mathematical semantics, machine representation | [0003](docs/decisions/0003-range-typed-integers.md) |
| ~~Go, JavaScript, Java first~~ (superseded by 0039) | [0004](docs/decisions/0004-first-targets.md) |
| The compiler is written in Go | [0005](docs/decisions/0005-implementation-language.md) |
| The backend interface is a file format | [0006](docs/decisions/0006-ir-file-format.md) |
| Explore candidates against a fixed test; do not specify the core first | [0007](docs/decisions/0007-exploration-over-specification.md) |
| Parasite decisions are per-target measurements | [0008](docs/decisions/0008-measurement-over-principle.md) |
| Staging must not change results | [0009](docs/decisions/0009-staging-preserves-results.md) |
| Effects are a side condition on β | [0010](docs/decisions/0010-effects-as-structural-rules.md) |
| Modules are resolution, not reduction | [0011](docs/decisions/0011-modules-add-nothing-to-the-reducer.md) |
| ~~`int` within ±(2⁵³−1)~~ (window superseded by 0026) | [0012](docs/decisions/0012-portable-integer-range.md) |
| ~~Accept the allocation price~~ (superseded by 0020) | [0013](docs/decisions/0013-accept-the-allocation-price.md) |
| Recursion is not in the language | [0014](docs/decisions/0014-recursion-is-not-in-the-language.md) |
| `loop`/`again`, guarded clauses over n variables | [0015](docs/decisions/0015-loop-and-again.md) |
| A target need not be an expression language | [0016](docs/decisions/0016-targets-need-not-have-expressions.md) |
| Booleans and control flow are in the language | [0017](docs/decisions/0017-booleans-are-in-the-language.md) |
| Immutable values, one scoped linear buffer | [0018](docs/decisions/0018-immutable-values-linear-buffers.md) |
| Precision by declaration, provisionally (items 3–4 amended by 0079) | [0019](docs/decisions/0019-precision-by-declaration.md) |
| A buffer is a nameable type: uniqueness on parameters | [0020](docs/decisions/0020-uniqueness-on-parameters.md) |
| Declarations are theories | [0021](docs/decisions/0021-declarations-are-theories.md) |
| Host declarations are written by hand; the generator checks them | [0022](docs/decisions/0022-host-declarations-are-written-by-hand.md) |
| A generator does not make a claim it cannot justify | [0023](docs/decisions/0023-a-generator-does-not-make-a-claim-it-cannot-justify.md) |
| A comment never carries meaning | [0024](docs/decisions/0024-comments-are-erased.md) |
| A module path is the host's, and the file is the path | [0025](docs/decisions/0025-a-module-path-is-the-hosts.md) |
| An `int` is an integer; each target realizes what it can; portability is reported | [0026](docs/decisions/0026-an-int-is-an-integer.md) |
| A host call's continuation is a tail | [0027](docs/decisions/0027-a-host-calls-continuation-is-a-tail.md) |
| A definition's parameter range and `where` are obligations at its calls | [0028](docs/decisions/0028-a-definitions-contract-is-checked-at-its-calls.md) |
| Above the word, one set decided by sign and bit length, enforced on every representation (decision 4 amended by 0098) | [0029](docs/decisions/0029-above-the-word-one-set-on-every-representation.md) |
| A host's string is the host's; ours is entered by one total decode | [0030](docs/decisions/0030-a-hosts-string-is-the-hosts.md) |
| A `build`'s result is a product of frozen buffers and values | [0031](docs/decisions/0031-a-builds-result-is-a-product.md) |
| The IR is structured SSA with π-parameters; every backend prints from it | [0032](docs/decisions/0032-the-ir-is-structured-ssa.md) |
| An integer's representation is chosen on the IR; the checker's sort is ℤ | [0033](docs/decisions/0033-an-integers-representation-is-chosen-on-the-ir.md) |
| The rung above the word is a least fixed point of supply and demand (supply amended by 0079) | [0034](docs/decisions/0034-the-rung-above-the-word-is-a-least-fixed-point-on-the-ir.md) |
| A compiler library is a theory checked once; a use is an inlined instance | [0035](docs/decisions/0035-a-compiler-library-is-a-theory-checked-once.md) |
| A residual's binders are named apart along the scope chain | [0036](docs/decisions/0036-a-residuals-binders-are-named-apart.md) |
| A program's obligations are in its residual, decided by one procedure | [0037](docs/decisions/0037-a-programs-obligations-are-in-its-residual.md) |
| The IR's text is the printers' whole input; read by others, written by nobody else | [0038](docs/decisions/0038-the-irs-text-is-read-not-written.md) |
| The targets are Go, Windows, Android and the browser | [0039](docs/decisions/0039-the-targets-are-go-windows-android-and-the-browser.md) |
| A failure is a value of a marked sum; `try` is bind, `expect` the one way to give up | [0040](docs/decisions/0040-a-failure-is-a-value-of-a-marked-sum.md) |
| A success arm is a tail; a loop yields a sum | [0041](docs/decisions/0041-a-success-arm-is-a-tail.md) |
| A variant's value is its tag and one slot per payload type | [0042](docs/decisions/0042-a-variants-value-is-its-tag-and-its-slots.md) |
| Relevance is checked at β on every path; `ignore` is !_A | [0043](docs/decisions/0043-relevance-is-checked-where-a-binder-meets-its-value.md) |
| A partial success's error is relevant | [0044](docs/decisions/0044-a-partial-successs-error-is-relevant.md) |
| A host's empty interface is a type | [0045](docs/decisions/0045-a-hosts-empty-interface-is-a-type.md) |
| A variadic is one declaration over a table | [0046](docs/decisions/0046-a-variadic-call-is-its-declaration-applied-to-a-word.md) |
| Local state is a scope; a cell is a loop variable | [0047](docs/decisions/0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md) |
| A host writes a cell through a reference to a fresh copy | [0048](docs/decisions/0048-a-host-writes-a-cell-through-a-reference.md) |
| Types are sets and the logic never prints; ρ from proven sets; own procedures | [0049](docs/decisions/0049-types-are-sets-and-the-logic-never-prints.md) |
| The abstract syntax is the semantics' initial algebra | [0050](docs/decisions/0050-the-abstract-syntax-fits-the-semantics.md) |
| A definition is a named λ carrying its claims | [0051](docs/decisions/0051-a-definition-is-a-named-lambda.md) |
| A buffer is a state binder holding a table, in program order; R0–R4 | [0052](docs/decisions/0052-a-buffer-is-a-state-binder-holding-a-table.md) |
| The arrow is a claim on a host's callback parameter (built by 0133, 0143; a returned function not) | [0053](docs/decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md) |
| `let` and `local` are one scope | [0054](docs/decisions/0054-let-and-local-are-one-scope.md) |
| A loop variable is bound by application | [0055](docs/decisions/0055-a-loop-variable-is-bound-by-application.md) |
| One arm grammar for choice and iteration (the surface: 0068) | [0056](docs/decisions/0056-the-surface-is-d-or-j-and-its-choice-and-iteration.md) |
| The handled bind is a derived operator; its success arm a tail | [0057](docs/decisions/0057-the-handled-bind-and-its-tail.md) |
| A map buffer is a state binder holding a map | [0058](docs/decisions/0058-a-map-buffer-is-a-state-binder-holding-a-map.md) |
| A tag is a quoted symbol; collections have one former | [0059](docs/decisions/0059-a-tag-is-a-quoted-symbol-and-collections-have-one-former.md) |
| Membership `∈`; an imported tag is qualified | [0060](docs/decisions/0060-j-set-operators-and-qualified-tags.md) |
| Separation and replacement by the binder's place (not built) | [0061](docs/decisions/0061-separation-and-replacement-by-the-binders-place.md) |
| A header is a membership statement; `fn`, `spec`, `lemma` (the logic not built) | [0062](docs/decisions/0062-a-header-is-a-membership-statement-and-the-logic-has-three-definitions.md) |
| A type is a static value; instantiation is application | [0063](docs/decisions/0063-a-type-is-a-static-value-and-instantiation-is-application.md) |
| A file is a theory; a declaration a header and one body (item 4 superseded by 0078) | [0064](docs/decisions/0064-a-file-is-a-theory-and-a-declaration-is-a-header-and-one-body.md) |
| J writes sets in TLA+'s symbols | [0065](docs/decisions/0065-j-writes-sets-in-tla-symbols.md) |
| A definition over sets is a λ returning a λ; named splices `${x}` | [0066](docs/decisions/0066-a-definition-over-sets-is-a-lambda-returning-a-lambda.md) |
| The lexical level | [0067](docs/decisions/0067-the-lexical-level.md) |
| **J is the surface** | [0068](docs/decisions/0068-j-is-the-surface.md) |
| An observer at a host parameter | [0069](docs/decisions/0069-an-observer-at-a-host-parameter.md) |
| A host header is the caller's view of a borrow | [0070](docs/decisions/0070-a-header-is-the-callers-view-of-a-borrow.md) |
| Work stays where it was written: β counts evaluations | [0071](docs/decisions/0071-work-stays-where-it-was-written.md) |
| A declared result is a claim decided at its definition | [0072](docs/decisions/0072-a-declared-result-is-a-claim-decided-at-its-definition.md) |
| A set's name denotes a set | [0073](docs/decisions/0073-a-sets-name-denotes-a-set.md) |
| A segment is its function restricted wherever its table is | [0074](docs/decisions/0074-a-segment-is-its-function-restricted-wherever-its-table-is.md) |
| A host `where`'s constant bounds are its parameter's range | [0075](docs/decisions/0075-a-wheres-constant-bounds-are-the-parameters-range.md) |
| A case in a tail is a tail when each arm is | [0076](docs/decisions/0076-a-case-in-a-tail-is-a-tail-when-its-arms-are.md) |
| An unbounded end is omitted, `lo..`; `Nat` is ↑0 | [0077](docs/decisions/0077-an-unbounded-end-is-omitted.md) |
| A module's interface is what it marks `pub`; `export` an entry point | [0078](docs/decisions/0078-a-modules-interface-is-what-it-marks-pub.md) |
| A declared set is a claim, and the facts choose the representation | [0079](docs/decisions/0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md) |
| The native targets print LLVM IR, stating the IR's proofs; every LLVM primitive offered, walls to hamza (MASM retired) | [0080](docs/decisions/0080-the-native-targets-print-llvm-ir.md) |
| Concurrency is typed processes: Mealy machines, failure an effect, supervisors its handlers (not built) | [0081](docs/decisions/0081-concurrency-is-typed-processes.md) |
| Wrapping is the residue map mod 2ⁿ on ℤ, not a sort; the bit operators are ℤ's; fusion | [0082](docs/decisions/0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md) |
| A representation is an abstraction function (H, d), chosen by place; the word S₆₄ on every triple | [0083](docs/decisions/0083-a-representation-is-an-abstraction-function-chosen-by-place.md) |
| An address is an integer offset in a region; ownership a linear view in the logic | [0084](docs/decisions/0084-an-address-is-an-integer-and-ownership-is-a-view.md) |
| A vector is a representation of a table; a fold vectorizes iff associative (not built) | [0085](docs/decisions/0085-a-vector-is-a-representation-of-a-table-and-a-fold-vectorizes-iff-it-is-a-homomorphism.md) |
| A number is in ℤ[1/10]; `round(S, e, mode)` its one lossy operation; no NaN or ∞ | [0086](docs/decisions/0086-a-number-is-an-element-of-z-tenth-and-a-rounding-is-its-one-lossy-operation.md) |
| A result's set is a claim by its ends inside the word, decided on ℤ's facts; amends 0029 | [0087](docs/decisions/0087-a-results-set-is-a-claim-by-its-ends-inside-the-word.md) |
| A target states its realization of each language name, chosen per instance by proof | [0088](docs/decisions/0088-a-target-states-its-realizations-and-a-host-operation-is-declared-as-it-is.md) |
| A realization is a term over the host's operations, ι : Σ_L → T(Σ_H); amends 0088 | [0089](docs/decisions/0089-a-realization-is-a-term-over-the-hosts-operations.md) |
| ∅ is a set; a function into it never returns; a host's product taken apart by projections | [0090](docs/decisions/0090-a-function-into-the-empty-set-never-returns.md) |
| A C string is the kernel of ν, the NUL homomorphism; `nulfree` in Σ_L | [0091](docs/decisions/0091-a-c-string-is-the-kernel-of-the-nul-homomorphism.md) |
| A table's storage is an IR operation, `alloc(n, w)`; the allocator a term over the host's | [0092](docs/decisions/0092-a-tables-storage-is-an-operation-and-the-allocator-a-term.md) |
| The LLVM layer is a theory with a parameter Σ_OS; windows instantiates it by gluing | [0093](docs/decisions/0093-the-llvm-layer-is-a-theory-with-a-parameter-and-windows-instantiates-it.md) |
| A C function is declared by its prototype, its sets its C types; the call derived by the ABI | [0094](docs/decisions/0094-a-c-function-is-declared-by-its-prototype-and-the-c-abi-is-derived.md) |
| Text from ASCII is the inclusion ι, `ascii(b, lo, n)`; rendering is ℤ's at every sort | [0095](docs/decisions/0095-text-from-ascii-is-the-inclusion-and-rendering-is-integers-at-every-sort.md) |
| An owned table is released at its death; storage by size class; W11 checks every release | [0096](docs/decisions/0096-a-values-storage-is-chosen-by-size-class-and-released-at-its-death.md) |
| A table born full takes a region with no zero fill | [0097](docs/decisions/0097-a-table-born-full-takes-a-region-with-no-zero-fill.md) |
| Fixed limbs realize a signed set by the group completion Z(V_n); amends 0029 | [0098](docs/decisions/0098-fixed-limbs-realize-a-signed-set-by-the-group-completion.md) |
| An unbounded integer with no bignum is K(ℕ\*), a numeral of any length, signed size last | [0099](docs/decisions/0099-an-unbounded-integer-is-a-numeral-of-any-length-with-its-signed-size-last.md) |
| A dead region serves a request it fits, the token carrying its count; amends 0096 | [0100](docs/decisions/0100-a-dead-region-serves-a-request-it-fits.md) |
| A library instance is a function of the program, decided once under its parameters' sets | [0101](docs/decisions/0101-a-library-instance-is-a-function-of-the-program.md) |
| Factoring never changes an answer or a verdict; no folding by pattern; amends 0101 | [0102](docs/decisions/0102-factoring-never-changes-an-answer-or-a-verdict.md) |
| An intermediate above the word is computed in the double word i128 where proven in E | [0103](docs/decisions/0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md) |
| K(ℕ\*) in base 2⁶³ (reversed by 0106); the two-digit lemma a transfer for `/` | [0104](docs/decisions/0104-k-n-star-is-in-base-2-63-the-largest-base-the-decision-proves.md) |
| The decision's ends are 192-bit; the double word has two readings; amends 0103 | [0105](docs/decisions/0105-the-decisions-ends-are-192-bit-and-the-double-word-has-two-readings.md) |
| K(ℕ\*) is in base 2⁶⁴, a digit in U₆₄, every step the double word's; reverses 0104 | [0106](docs/decisions/0106-k-n-star-is-in-base-2-64-the-machines-digit.md) |
| A region is answered by its capacity class, 2^⌈log₂ b⌉; amends 0100 | [0107](docs/decisions/0107-a-region-is-answered-by-its-capacity-class.md) |
| The full adder is the triple's `adc`/`sbb` where IR_P's types prove it | [0108](docs/decisions/0108-the-full-adder-is-the-triples-and-a-fold-over-a-split-domain-is-two-loops.md) |
| A product above 64 digits is Karatsuba's, its levels a tensor power | [0109](docs/decisions/0109-a-product-is-evaluation-and-interpolation-and-its-levels-are-a-tensor-power.md) |
| A square is the product's quadratic form, by polarization | [0110](docs/decisions/0110-a-square-is-the-products-quadratic-form-by-polarization.md) |
| Printing is a tree of divisions by β^(2^j), each node Algorithm D in place; extends 0095 | [0111](docs/decisions/0111-printing-is-a-tree-of-divisions-each-node-algorithm-d-in-place.md) |
| `os` on windows is J over kernel32; valid UTF-8 recognized by its monoid | [0112](docs/decisions/0112-os-on-windows-is-a-library-over-kernel32-and-valid-utf-8-is-recognized-by-its-monoid.md) |
| Definitions kept as functions where the target prints them; a kept one a Hoare triple | [0113](docs/decisions/0113-a-programs-definitions-are-kept-as-functions-where-the-target-prints-them.md) |
| A fold is an equation between pure terms: the language's operators fold only where the primitive is pure | [0114](docs/decisions/0114-a-fold-is-an-equation-between-pure-terms.md) |
| A rule at an export's boundary is its extension, `alloc(rule)`; in the interior a rule fuses or the author writes `alloc` | [0115](docs/decisions/0115-a-rule-at-an-exports-boundary-is-its-extension.md) |
| A statement whose value is a function is refused; `ignore(f)` is the discard | [0116](docs/decisions/0116-a-statement-whose-value-is-a-function-is-refused.md) |
| A string is the table of its UTF-8 on the LLVM layer, its operations the language's | [0117](docs/decisions/0117-a-string-is-the-table-of-its-utf-8-and-its-operations-are-the-languages.md) |
| A C call's arguments are a linear term over its parameters; amends 0094 | [0118](docs/decisions/0118-a-c-calls-arguments-are-a-linear-term-over-its-parameters.md) |
| A store owes its table's domain, 0 <= i < len, as a read does | [0119](docs/decisions/0119-a-store-owes-its-tables-domain.md) |
| A map on the LLVM layer is the table of its cells, realized after the decision | [0120](docs/decisions/0120-a-map-on-the-llvm-layer-is-realized-after-the-decision.md) |
| A map of capacity c is in {m : len(m) ≤ c}, each insert marked len(m) < c | [0121](docs/decisions/0121-a-maps-capacity-is-a-claim-on-its-len.md) |
| The windows family is a fibration over Win32 (`win32` glued by each member); a triple is a model: data model, address space in bytes, ISA, toolchain | [0122](docs/decisions/0122-the-windows-family-is-a-fibration-and-a-triple-is-a-model.md) |
| A triple's runtime, what its code generator calls (i686's 64-bit division), is a library in the language linked into every program; a host result's set is a refinement fact | [0123](docs/decisions/0123-a-triples-runtime-is-a-library-in-the-language.md) |
| K(ℕ\*)'s base is the triple's native word, 2⁶⁴ on x86_64 and 2³² on i686, one parameter block; the two-digit lemma reads the unsigned word; amends 0106 | [0124](docs/decisions/0124-k-n-stars-base-is-the-triples-native-word.md) |
| A dead region crosses a call as the callee's token: release(p) ; f(x) = f′(x, p), an instance's variant `f~tok` storing its result by fit; W12 across the call | [0125](docs/decisions/0125-a-dead-region-crosses-a-call-as-the-callees-token.md) |
| Win32 is by header (`windows/H`, each declaration's `lib` its DLL); a C type name is its typedef, a set, and a data model interprets basetsd.h's four pointer-width names; a C integer's set is its type's whole value set, U₆₄ for a 64-bit unsigned type (amends 0094); the unsigned word chosen on ι's image | [0126](docs/decisions/0126-win32-is-by-header-and-a-c-type-name-is-its-whole-value-set.md) |
| A scalar proven in S₃₂ is an i32 where the triple prefers 32 bits (`repr scalar 32`, windows32): ρ₃₂ read off IR_P's range by the printer, an operation at 32 bits iff its operands, result and 32-bit domain are proven, every other statement the word's in a wide scope | [0127](docs/decisions/0127-a-scalar-in-s32-is-held-in-an-i32-where-the-triple-prefers-it.md) |
| A record is built whole: `record { l ∈ S }` a set, `record { l : e }` a value, `r.l`, `r with { p : e }` an override along prefix-free paths, `zero(T)`; wherever a tuple goes, three positions lifted; a host struct is its layout, derived (Theorem L, pack the data model's) and checked against clang; a determined field the declaration's (Theorem F) | [0128](docs/decisions/0128-a-record-is-built-whole-and-a-host-struct-is-its-layout.md) |
| A struct parameter is its prototype: the bare set is the value, passed by the triple's ABI (Win64 `iN` for 1, 2, 4 or 8 bytes, else the copy's address; i686 `byval`), and `in` marked `_In_`'s const T* (reversed by 0130); amends 0128 item 13 | [0129](docs/decisions/0129-a-struct-parameter-is-its-prototype.md) |
| `const T*` is the call's argument term `ref(x)`, &x of a fresh copy, not a mode of the header: a mode is what a program observes; reverses 0129 item 2 | [0130](docs/decisions/0130-const-t-star-is-the-calls-ref-x.md) |
| An `_Out_` parameter is a result of the declaration, its address the call's `&r`; a C call's arguments written as C writes them, `call F(x, &x, &r, &b[lo], NULL)`, read as ADR 0118's terms; `_Inout_` stays a cell, Go's `out τ` where the program chooses τ | [0131](docs/decisions/0131-an-out-parameter-is-a-result.md) |
| A C function's (pointer, count) to a byte region is one segment parameter, `call WriteFile(h, s, len(s), …)`; a program passes `b` or `b[lo:hi]`, the variant derived; amends 0118's linearity | [0132](docs/decisions/0132-a-c-functions-pointer-and-count-are-one-segment.md) |
| The window procedure is a λ at WNDCLASSA's field, its set the named claim WNDPROC (ADR 0053) with its C signature `repr WNDPROC c(h, m, w, l) stdcall`, lifted and printed behind a wrapper at that convention; its state a `var` cell the λ captures and passes to a step's `s ∈ S inout`, the cell its static slot (Theorem T's global): no word for the extent, no classification of dispatching calls | [0133](docs/decisions/0133-the-window-procedure-is-a-lambda-at-the-hosts-field-over-a-cell-in-scope.md) |
| An `again` after a nested loop's join point is accepted in a tail (its representation superseded by 0139) | [0134](docs/decisions/0134-an-again-after-a-join-point-is-theorem-js-rewrite.md) |
| A cell parameter is the mode `x ∈ T inout`, not the set `cell(T)`: a set ranges over values and a cell is a place, as `Array(V) linear` and not `buffer(V)`; `cell(T)` at a J parameter refused naming `inout`; W7 lets `inout` stand on a program definition's parameter; reverses ADR 0133 item 6's spelling | [0135](docs/decisions/0135-a-cell-parameter-is-the-mode-inout-not-the-set-cell.md) |
| A struct's field is a value and a pointer in it the layout's, written as C writes it: a buffer in a struct a table field (`= len(m)`), a nested record by pointer `&f`, an inline array `f[N]`, an optional C string `Option(String)`, NULL its niche (not built) | [0136](docs/decisions/0136-a-struct-field-is-a-value-and-its-pointer-is-the-layouts.md) |
| An unread binder's typing claim is checked, by a typing mark in the residual first, measured | [0137](docs/decisions/0137-an-unread-binders-typing-claim-is-checked.md) |
| On ℕ ∩ S₆₄, `%` and `/` by a constant divisor are `srem`/`sdiv`, a variable divisor keeping `urem`/`udiv`; amends ADR 0088, measured on the corpus | [0138](docs/decisions/0138-a-remainder-by-a-constant-of-a-natural-is-srem.md) |
| A tuple pattern over a loop whose body jumps out is a `let` of several names, which every walker of a clause chain reads (`joinLet`); reverses 0134's B | [0139](docs/decisions/0139-a-tuple-pattern-over-a-loop-is-a-let-of-several-names.md) |
| A lent buffer handed back is a primed result, `(b' ∈ Array(V) linear)`; `buffer(V)` at a result refused | [0140](docs/decisions/0140-a-lent-buffer-handed-back-is-a-primed-result.md) |
| A family's member is its C function called at a constant, `call SendMessageA(w, 1076, 0, &cr)`, typed by the erased declaration; a pointer at an integer the integer carrying it (ADR 0084's P5) | [0141](docs/decisions/0141-a-family-member-is-its-c-function-at-a-constant.md) |
| A target's CPU is data of its triple's model, `cpu "x86-64-v3"`, printed as every function's `"target-cpu"`; a level a member (`windows-v3` glues `windows`); baseline the default | [0142](docs/decisions/0142-a-targets-cpu-is-data-of-its-triples-model.md) |
| A callback applied during the call is a region of the call, read as a loop of the host's count; `calls K` (K a range) and `ordered` after the arrow, the host's documented promises, default `calls 0..` unordered, their evidence in the tooling; an effect in the λ only under `ordered`, a cell or buffer threaded (the state monad's lift of a traversal); on C, closure conversion into the API's context word, `ctx`/`ctx(f)`, a slot where it has none (built on every target) | [0143](docs/decisions/0143-a-callback-during-the-call-is-a-region-and-its-markers-are-the-hosts-promises.md) |
| A definition of the program's applied to a λ that writes a cell is expanded before the state translation: the strongest host, its body the order | [0144](docs/decisions/0144-a-definition-applied-to-a-lambda-is-expanded-before-the-state-translation.md) |
| The logic: `∀ x ∈ S : φ`, `∃`, `⇒`, `⇔`, every quantifier over a set; recursion in `spec` and `lemma` only, checked to terminate; a lemma proven by its steps, no axioms; a host's quantified `ensure` believed with evidence; `-checked` runs a bounded ∀ as a loop; Presburger with quantifiers, the array property fragment, congruence closure, their combination (not built) | [0145](docs/decisions/0145-the-logic-quantified-formulas-over-sets-proven-by-own-procedures.md) |
| A handle is a host type, C's `HWND__*`, its one operation `==` within its kind; a failed call's handle the niche of an `Option` on the declaration (`niche -1`, `niche NULL`); an `_In_opt_` handle an `Option`, none its NULL, held to the SDK's SAL | [0146](docs/decisions/0146-a-handle-is-a-host-type-and-a-failed-calls-handle-is-its-niche.md) |
| `Int` is ℤ at an entry point too, its parameter ρ of its set and `where` (the word, or the host's integer of any size; windows refuses an unbounded one); a host's word its module's `Word`; one rendering of ℤ, `decimal`, `io.print_int` over ℤ | [0147](docs/decisions/0147-int-is-the-integers-at-an-entry-point-and-decimal-renders-them.md) |
| Where an operation's box leaves the word and its term repeats an atom or relates two, the Omega test decides its word (Moore: the box is exact on independent atoms); a `where`'s comparison a zone entry; a range claim the intervals miss decided by the logic; Presburger's constants exact; `Int` at a binder ℤ, the consent | [0148](docs/decisions/0148-where-the-box-forgets-a-relation-presburger-arithmetic-decides.md) |
| The double word E on every 64-bit host (Go by math/bits, the JVM by its carries, `Math.multiplyHigh` and Knuth's division); a signed quotient of E by a constant the magnitude's (LLVM's unsigned division; Go's and the JVM's 2-by-1 with Horner's rule for a remainder); a quotient by a word the 2-by-1 division, where the target has one | [0149](docs/decisions/0149-the-double-word-is-every-hosts.md) |
| A set is a claim and ρ alone chooses: an operation past S, U and E that reaches no sink is held in ℤ and reported; a refusal only at a sink; `-words` refuses it instead | [0150](docs/decisions/0150-a-set-is-a-claim-and-rho-alone-chooses.md) |
| A representation holds a value only where it computes the value's uses: on JavaScript a value of U is a pair where its operations are the pair's, ℤ (BigInt) whole otherwise, the greatest fixed point on the pair the rung's least; the pair compares lexicographically and reads a word signed; amends 0083 and 0150 | [0151](docs/decisions/0151-a-representation-holds-a-value-only-where-it-computes-its-uses.md) |
| The one set is checked where it claims: a fit at each value a declared set above the word is ascribed to and at each operation the decision does not bound, none at an operation with a finite fact; fixed limbs hold a program only within their width; amends 0029 | [0152](docs/decisions/0152-the-one-set-is-checked-where-it-claims.md) |
| The double word holds a loop's variable and result where their inputs are words or E's, marked by an `int` type past both words; a yield into ℤ of a value E can hold is its digits' recombination, Euclid's identity in radix 2³²; amends 0103 and 0149 | [0153](docs/decisions/0153-the-double-word-holds-a-loops-variable.md) |
| A parameter's set that is no range is a claim at the call: a where-mark `(#in "S" a "i")` on the call's result, decided by the type checker as a typing mark and refused as *in argument i of f*; a literal of the set's kind decided by the reducer; `ignore` keeps a pure value's obligations | [0154](docs/decisions/0154-a-calls-sort-claim-is-decided-at-the-call.md) |
| A kept definition's parameter is no sink: keeping is a factoring (ADR 0102), so the taint reads the program inlined and P4 inlines a call whose argument leaves its parameter's set; amends 0150 | [0155](docs/decisions/0155-a-kept-definitions-parameter-is-no-sink.md) |

## Build commands

Module path is `oroboros` (local).

```bash
go run ./cmd/check                 # every check (gauntlet/check/README.md)
go run ./cmd/check -skip tooling   # everything but the host surveys
go run ./cmd/check -accept "why"   # keep this run's emission as the baseline, and log why
go test ./core/ ./emit/ ./ir/...   # the compiler
go run ./cmd/build -target=go -o hello.exe examples/hello.oro   # a real binary
go run ./cmd/oro -target=go examples/native/dot.oro          # reduce to normal form
go run ./cmd/gen -ir dot.ir -name native examples/native/dot.oro go dot.go   # emit, and write the IR
go run ./cmd/intervals -v examples/table/sieve.oro go           # what the decision proves, and what not
go run ./cmd/portable examples/io/wc.oro                        # which targets accept a program, and why
bash gauntlet/regen.sh                                          # regenerate the gauntlet's 33 files and the runtime
cd gauntlet/go && go test -bench='TreeGen|TreeFlat$' -benchtime=20000x -count=5   # generated vs hand-written
```

- `gauntlet/go`, `gauntlet/js`, `gauntlet/java` and `experiments/legibility` are separate modules: `cd`
  into them first. `gauntlet/fmt/*.go` are `//go:build ignore` scripts run with `go run`.
- LLVM 23.1.4, the official release, is installed at `%LOCALAPPDATA%\Programs\LLVM` (2026-10-27);
  MSVC 2022 Community supplies `link`, `vcvars64.bat` and `vcvars32.bat`.
- On Windows, Git Bash's `/tmp` is not visible to native tools: use the session scratchpad. A heredoc
  eats backslashes: write a script containing `\n` with the Write tool, not through bash.
- **Line endings are LF** (`.gitattributes`); the four Windows scripts are CRLF and the emission baseline
  is `-text`, so `gofmt -l` means what it says.

## What exists

| | |
|---|---|
| `core/` | J's reader (`jlex`, `jparse`, `jreadtarget`), σ_J (`jspell`), J in messages (`jshow`), the raw reader of core terms and the IR's text (`raw`, `read`); the trees (`abs`, `decl`) and D (`desugar`); terms, the β/δ reducer and module loading (`reduce`), variants (`sum`), hygiene, the state translation (`cell`, `place`), β's work count (`work`), Σ_L |
| `ir/` | lowering, typing, the verifier (W1–W10), the printer and reader; the definitions kept as functions (`keep`, ADR 0113); the decision (`decide`, `interval`, `trip`, `sct`, `require`), the representations (`words`, `big`, `limbs`, `shift`, `numbers`: N's scaled word, `doubles`: F64's realizations), the claim (`claim`), the pipeline (`ir.Entry`); `ir/plan` and the four printers `ir/golang`, `ir/js`, `ir/java`, `ir/llvm` |
| `presburger/` | the Omega test (Pugh 1991), complete over ℤ, a soundness witness for the refinement layer |
| `emit/` | the type checker, the refinement layer (`refine`, `linear`, `fact`, `content`), contracts (`requires`), the target loader (`target`, `companion`, `alias`, `constend`, `hostdef`), linearity, the limb library as a theory (`bigrep`, `biglimb`, `bignum.oro`), the inventory's tests |
| `targets/` | target declarations, data and not Go: `go/`, `js/`, `java/`, `llvm/` (the LLVM layer, T_LLVM[Σ_OS]: `instructions` Σ_LLVM, `lang` ι's terms, `os` Σ_OS's headers, `str`), `win32/` (the windows family's base, ADR 0122: glues `llvm`; `os` the view over kernel32; Win32 by header, ADR 0126: `minwindef`, `winnt`, `windef`, `basetsd` the type names, `fileapi`, `heapapi`, `winuser`, `wingdi` … the functions), `windows/` (the x86_64 member: glues `win32`; its triple, LLP64's four names in `model.oro`, its address space, `x64.oro` the ISA's operations), `windows32/` (the i686 member: ILP32's four, `x86.oro`'s `divl`, `runtime "rt/i686.ll"`), `windows-v3/` (the x86_64 member at x86-64-v3, `cpu`, ADR 0142); `blas` and the `tutorial*` targets parameterize the normal form for the book |
| `lib/` | library modules: `os`, `io` (portable names over each host), `result`, `num` (`num.oro`: a number's decimal numeral; `num/` the older vectors), `win` (`print`, in the language's operators), `rt` (`i686.oro`: windows32's runtime, ADR 0123) |
| `cmd/` | `check`, `build`, `gen`, `oro`, `intervals`, `portable` |
| `examples/` | the programs: `io/` (`wc`, `count`, `jsonfmt`, `roundtrip`, `freq`), `os/` (`du`, `ls`, `wc`), `lines/` (`lines`, `ages`, `shape`), `json/`, `big/`, `hash/` (FNV-1a and CRC-32), `num/` (`ledger`, decimals), `int/` (meant to be refused), `kara/`, `tally/`, `u128/`, `native/` (the gauntlet's sources) |
| `gauntlet/` | hand-written references, `results/`, `check/` (the baseline), `differential/` (cases on four targets, proving their arithmetic unless `; checked:` says why), `conformance/`, `regen.txt`, `balance.py`, `surface/` (the surface's measurements) |
| `gauntlet/stdlib/` | the four host surveys (writing J through `jdecl`), `coverage.go`, `callbacks.go`, 24 acceptance programs, `tooling_test.go`, which checks every hand declaration against its host, and `win32hand_test.go`, Win32's against its prototypes |
