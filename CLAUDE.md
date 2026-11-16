# CLAUDE.md

Guidance for Claude Code working in this repository. **It states where the project is, not how it got
there.** Each result has its document in [gauntlet/results/](gauntlet/results/), each round an
assessment in [docs/](docs/), each decision an ADR in [docs/decisions/](docs/decisions/); follow the
links for history. Update this file where the state changes, and never append a narrative. The long
versions are in git: `73004de` (a 60,000-word changelog) and `b22d553` (17,300 words, before this
rewrite, 2026-10-26).

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
| [arrow-research.md](docs/arrow-research.md): the arrow is a claim on a host's callback parameter | ADR 0053 | not built (callbacks tier 1) |
| [concurrency-research.md](docs/concurrency-research.md): a process is a Mealy machine; a crash the exception effect, a link its propagation, a supervisor its handler; isolation a typing property, determinacy computed | ADR 0081 (the direction, C2 with C3 and C4 as fragments) | not built; §7's six questions and the surface answered when a program meets them |

## Where it stands

**Read first**: the primary documents above, [README.md](README.md), [design-direction.md](docs/design-direction.md)
(the founding argument; its §8 records how its questions were settled, concurrency's by ADR 0081), the
ADRs, [spec/state.md](docs/spec/state.md), and the current assessment. Measurements in
[gauntlet/results/](gauntlet/results/) are the authority: every design claim here not measured has been
wrong about half the time.

**The current assessment is [assessment-2026-11-16.md](docs/assessment-2026-11-16.md)**; read it
before planning. **Its plan (§5)**, in order:
1. **the book's wrong answers and faults**, each with a planted fault: ~~53/76, two payloads under
   one name~~ done ([capture-2026-11-16](gauntlet/results/capture-2026-11-16.md)); ~~70, a store's
   index unchecked~~ done (ADR 0119, [store-2026-11-16](gauntlet/results/store-2026-11-16.md): the
   guards it needed are missing proofs, the builder and K(ℕ\*)'s carrier owed, and an `abandon` arm in
   a borrowed buffer's code a defect); ~~71's `len`, 64, 39, 40, 78~~ done
   ([bookfaults-2026-11-16](gauntlet/results/bookfaults-2026-11-16.md): a map handed on through a
   tuple, a product's eliminator its projections, ⌈q⌉ for all four relations with a decimal, a
   refusal that crashed, the corpus read from git's index); ~~the windows map realized after the
   decision~~ done (ADR 0120, [maprealize-2026-11-16](gauntlet/results/maprealize-2026-11-16.md):
   `emit/winmap.go` deleted, finding 72 closed, the map 1.22× Go's, owed); ~~71's capacity~~ done
   (ADR 0121, [capacity-2026-11-16](gauntlet/results/capacity-2026-11-16.md): hamza's B′, Map_c =
   {m : len(m) ≤ c}, each insert marked len(m) < c by S; presence, a borrowed map's capacity and an
   O(1) windows `len` owed). **Item 1 is done**;
2. ~~**j.md's two promises**~~ done ([invariant-2026-11-16](gauntlet/results/invariant-2026-11-16.md)):
   `invariant I` is Hoare's rule, I's conjuncts Houdini's required candidates, decided at entry and
   kept as a fact once proven (`"a loop's invariant, proven"`, read by the refinement walk, erased
   by lowering); readfile's clamp gone, what readinto owed being an out cell's set and a version's
   len t = len z, not the rule; an allocated rule's length its count; `try b₁, …, bₙ { e }` the
   nested bind. An invariant over a buffer's elements owed. Next item 3;
3. **two checks**: no carriage return in a tracked text file; a planted source rewrite asserts it
   happened;
4. **the second triple**, research first (the data model, K(ℕ\*)'s base per triple, `max_len`,
   `x64.oro` split, i32 scalars), hamza's decisions, then built. With it, **Win32 by header** (W1,
   hamza 2026-11-16): a module is a theory and the header is C's unit of declaration, so a module is
   the header (`windows/fileapi`), and the DLL, where a symbol is realized, is each declaration's
   `lib`; an ADR at that step, the data model's type names (`DWORD`, `SIZE_T`) in their headers;
5. **φ maximal** (hamza 2026-11-16): every fact the decision proves that LLVM's attribute lattice
   expresses is printed (`noundef`, `noalias`, `nonnull`/`dereferenceable`, `range`, `nneg`,
   `disjoint`, `exact`, `samesign`, `memory(…)`, `mustprogress`/`willreturn`), one kind at a time,
   each a soundness rule with its planted fault, measured against C; an intrinsic only where a
   measurement shows LLVM does not recognize the idiom (ADR 0008);
6. **TinyRetroPad**, 6c then 6d;
7. **CLAUDE.md as state**, under 8,000 words.

**A wrong answer, wherever found, goes ahead of the plan** (assessment-2026-11-16 §3.4), and the
book's findings index is read at each item's end.

The 11-15 plan (assessment-2026-11-15 §5) is done but for its last three items, which the plan
above carries: P8 (steps 1–3), the book's findings 1–28 (below), the gauntlet on four targets
([gauntlet-2026-11-16](gauntlet/results/gauntlet-2026-11-16.md)), the harness
([harness-2026-11-16](gauntlet/results/harness-2026-11-16.md)), the file programs on windows
([files-2026-11-16](gauntlet/results/files-2026-11-16.md)), and, at hamza's question, the LLVM layer's
last templates (ADR 0117, 0118).
- ~~**The book's wrong answers**~~ **done** ([bookfix-2026-11-15](gauntlet/results/bookfix-2026-11-15.md),
  ADR 0114 for the fold; emission byte-identical), each with a planted fault:
  - the unit is no integer: a one-armed `if` is `if c {a} else {()}`, typed A ∨ 1, so it is no
    `Int` (19), and a declared `()` is a claim (10, ADR 0072);
  - `len` and indexing of a `String` are refused, as state.md §3 decides, since three hosts answer
    three ways (1); the scalar count, len : Σ\* → (ℕ, +) a homomorphism every host computes, is
    strings.md's question for hamza;
  - relevance through a program's own definition (2, ADR 0043);
  - a `module { }` block's scope ends at its brace (15);
  - a module path resolved case-exactly (17);
  - a literal of Σ_L is not definable (20);
  - an impure primitive is not folded (10-06 #3). The algebra decides it: an effectful application
    is not its value, so the fold consults the purity bit.
- ~~**The surface's promises and its diagnostics**~~ **done** ([surface-2026-11-15](gauntlet/results/surface-2026-11-15.md),
  ADR 0115 for the boundary; a range payload refused in J and owed), where j.md says built and the
  reader refused:
  - prefix `-` (5);
  - a telescope and a constant in a header's range (9, 13);
  - a top-level claim value (14);
  - `_` in a module file's name, σ_J⁻¹ (16);
  - a rule as an export's result (6), one construction with `pe`'s segment at a kept boundary: a
    rule crossing a function boundary is allocated or passed as a view;
  - a big value to `print_int` refused by W5 (3);
  - a pure unread binding kept (12);
  - the diagnostics in J: 4, 7 (the closure's `#1.0`), 11, 21 (`x64.sete`), and core-0's "six term
    kinds".
- ~~**The book's chapters 5 and 6**~~ **done** ([chapters56-2026-11-15](gauntlet/results/chapters56-2026-11-15.md),
  ADR 0116 for 22; emission byte-identical), each with a planted fault:
  - 28: a value above the word whose fact is in S is r(v), `big-word`, the retraction ℤ ⇀ S
    (r ∘ ι = id_S, integers.md §1a); from fixed limbs owed;
  - 27, 26: a value of U at a declared `Int` is refused on JavaScript's pair as on the other three;
  - 23: a claim the(S, e) is an atom in S for the linear fragment (ADR 0049);
  - 24: an unstated result (D's `_`, `Sig.Unstated`) is the body's, `(Any)` one result;
  - 25: an empty range is said to be empty, in J;
  - 22: a statement whose value is a function is refused, `ignore(f)` the discard;
  - 29 deferred. Owed: windows' K(ℕ\*) division of a square by 2⁴⁰ (an unrealizable comparison), a
    JavaScript parameter above the word at `% 2` (W5). An export's K(ℕ\*) result was fixed by the
    harness (harness-2026-11-16).
  Previous:
[11-15](docs/assessment-2026-11-15.md), [10-24](docs/assessment-2026-10-24.md), [10-21](docs/assessment-2026-10-21.md), [10-20](docs/assessment-2026-10-20.md),
[10-14](docs/assessment-2026-10-14.md), [10-03](docs/assessment-2026-10-03.md),
[09-28](docs/assessment-2026-09-28.md), [09-24](docs/assessment-2026-09-24.md),
[09-23](docs/assessment-2026-09-23.md), [09-17](docs/assessment-2026-09-17.md),
[09-13](docs/assessment-2026-09-13.md), [09-11](docs/assessment-2026-09-11.md),
[09-09](docs/assessment-2026-09-09.md), [09-06](docs/assessment-2026-09-06.md),
[08-20](docs/assessment-2026-08-20.md), [08-19](docs/assessment-2026-08-19.md),
[08-13](docs/assessment-2026-08-13.md).

**The standing goal** (hamza) is **the Go standard library, package by package, program first**: a
program in the language that needs a package, its algebra, and exactly the declarations it calls,
checked. When a package hits a wall that needs language work, stop, research it, design it.

### The 10-24 plan (assessment-2026-10-24 §5)

1. ~~The documents tell the truth~~ **done** ([docs-2026-10-25](gauntlet/results/docs-2026-10-25.md)):
   j.md's status read off the reader; state.md in J; inventory.md in two layers; j.md §1.4 and the
   inventory's counts checked against the code; `cmd/check`'s `gauntlet` step.
2. ~~Two surface questions~~ **done** ([surface-2026-10-25](gauntlet/results/surface-2026-10-25.md)):
   an unbounded end is omitted, `lo..` (ADR 0077; hamza's `inf` weighed and rejected there); `pub` marks
   the interface, private the default, `export` the entry points (ADR 0078).
3. ~~Three defects the audit found~~ **done** ([defects-2026-10-26](gauntlet/results/defects-2026-10-26.md)):
   no module defines a name of Σ_L (a nested `fn if` had captured every `if`); the recursion refusal names
   `loop`; a set operator in a term is refused saying what it is.
4. ~~A declared set is a claim, and the facts choose the representation~~ **done** (ADR 0079,
   [claimset-2026-10-26](gauntlet/results/claimset-2026-10-26.md)); then `Nat`.
5. ~~CLAUDE.md rewritten as state~~ **done** (2026-10-26, this file).
6. **The Windows track, in steps** (hamza's):
   - 6a. ~~MASM or LLVM IR~~ **decided: LLVM IR** ([ADR 0080](docs/decisions/0080-the-native-targets-print-llvm-ir.md),
     [backend-2026-10-26](gauntlet/results/backend-2026-10-26.md)). One IR_P printed three ways, MASM, C and
     LLVM IR: MASM is dominated (LLVM `-O2` 0.63 and 0.35 of its time; `minsize` 0.77 and 0.51 of its code at
     no worse speed); C carries less of the IR's proofs (0.55 against 0.35 under one compiler) and its speed
     is one compiler's heuristics. LLVM adds no check: its undefined behaviour is what the IR proves absent,
     so the proofs become `nsw`, `nuw`, `inbounds`, `select`. 32-bit is a triple.
   - 6b. **`ir/llvm`, spec first**: the printer of ADR 0080 at the corpus's windows parity, then MASM
     retired, **both done** (below). **LLVM IR is a host like any other, and
     its target offers every primitive it has** (hamza, 2026-10-27; ADR 0080 item 7): Σ_LLVM declared by
     hand, each operation's undefined behaviour its `where`; **a primitive the language cannot state is a
     wall, brought to hamza** before anything is built for it. LLVM 23.1.4, the official release, is
     installed at `%LOCALAPPDATA%\Programs\LLVM` (2026-10-27). **Σ_LLVM surveyed**
     ([llvmsurvey-2026-10-27](gauntlet/results/llvmsurvey-2026-10-27.md)): 31 of 67 instructions and 83 of
     534 target-independent intrinsics stateable now; **seven walls with hamza** (bit-vectors, widths, float
     formats, relations, the float environment, pointers as values, vectors), each with its algebra and a
     recommendation; the declarations and the printer wait on them. **Each wall is a step, a research
     document first and hamza's decision after** (hamza, 2026-10-27), in this order:
     1. ~~**ℤ/2ⁿ and 𝔽₂ⁿ**~~ **decided** ([ADR 0082](docs/decisions/0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md),
        [bits-research.md](docs/bits-research.md)): `& | ⊕ ~ << >>` are ℤ's own, wrapping is `mod 2ⁿ`, no
        new sort, fusion the one new soundness rule, `⊕` spelled `\oplus`. **Built** with step 2
        ([bits-2026-10-27](gauntlet/results/bits-2026-10-27.md)).
     2. ~~**Widths**~~ **decided** ([ADR 0083](docs/decisions/0083-a-representation-is-an-abstraction-function-chosen-by-place.md),
        [widths-research.md](docs/widths-research.md)): a representation is an abstraction function
        (H, d), one mechanism; ρ chooses by place; native iN above the word on LLVM; the word S₆₄ on
        every triple. **Built with ADR 0082, program first** ([bits-2026-10-27](gauntlet/results/bits-2026-10-27.md)):
        `examples/hash`, FNV-1a and CRC-32 of a file, equal to Go's `hash/fnv` and `hash/crc32` and at
        parity with hand-written code on Go, the JVM and JavaScript (1.00–1.12); the JVM's unsigned word
        and its `byte` read unsigned, JavaScript's typed arrays, and **JavaScript's split pair**
        ([pairs-2026-10-28](gauntlet/results/pairs-2026-10-28.md)): U as two 32-bit digits, every
        operation expanded on the IR (`ir/pairs.go`) and proven by the decision; FNV-1a 64 runs there at
        0.80 of a hand-written pair. Not built on the pair: a comparison of two pairs, a shift by a
        variable count, a division by a non-power of two. `>>` past a count of 63 on windows came with
        `ir/llvm`, the unsigned word there with 6b′ step 3 (llvm64-2026-11-06); LLVM's widths wait.
     3. ~~**Pointers**~~ **decided** ([ADR 0084](docs/decisions/0084-an-address-is-an-integer-and-ownership-is-a-view.md),
        [pointers-research.md](docs/pointers-research.md)): an address is an integer offset in a region,
        ownership a linear **view** in the logic (Theorem V: with views, memory is values). **The portable
        part built, program first** ([views-2026-10-30](gauntlet/results/views-2026-10-30.md)):
        `examples/pe`, a PE header reader and checksum fixer equal to Go's `debug/pe` and Windows' own
        checksum on Go, the JVM and JavaScript; separation and set definitions, segments `b[lo:hi]` as
        rules, `inf`/`sup`, `lib/bytes`' codecs, the linear result (`adopt`: a move, or a copy),
        `out` on a set of tables, `os.ReadAt`. Two soundness holes closed: a rule's index past its end,
        and a buffer a host filled read as its zero fill. Not built: two segments lent together, a
        segment moved out, products of codecs; the LLVM attributes with `ir/llvm`; P4–P5 with 6d.
     4. ~~**Vectors**~~ **decided** ([ADR 0085](docs/decisions/0085-a-vector-is-a-representation-of-a-table-and-a-fold-vectorizes-iff-it-is-a-homomorphism.md),
        [vectors-research.md](docs/vectors-research.md)): a vector is a representation of a table of
        static length, never a sort; the compiler vectorizes rules, folds of an associative operation
        and Omega-independent stores; a float fold keeps its order (measured: reassociated, 2× faster
        and a different number), its reordering a relation for wall 6; explicit SIMD only where
        measured; GPUs a later target layer. Not built: its consumer is `ir/llvm`.
     5–7. ~~**Float formats, relations, the float environment, with decimals and literals**~~ **decided**
        ([ADR 0086](docs/decisions/0086-a-number-is-an-element-of-z-tenth-and-a-rounding-is-its-one-lossy-operation.md),
        [floats-research.md](docs/floats-research.md) Parts I–III; hamza agreed to G1–G7 and S1–S4,
        2026-11-01):
        - **one numeric sort, ℤ[1/10]** (Theorem R): +, −, ·, = and < exact; `Int`, F(p, …) for any p,
          D(k, R) and Q(k, R) are sets in `lib/num`; a literal is exact;
        - **`round(S, e, mode)` is the only lossy operation**, with `÷` (`\div`), `sqrt` and the
          transcendentals only at its root, one per rounding (Theorem T); `/` needs integer operands;
        - every answer is a function of the inputs (Theorem D); the facts (an interval and v₂, v₅)
          choose ρ;
        - a target realizes a rounding by an `ensure r == round(S, t)`, otherwise the host's, Tier 2;
        - `round(F64) { … }` is IEEE's per-operation scope;
        - no ∞, NaN or −0: overflow an obligation, a host double decoded;
        - accuracy a claim.

        Measured: Figueroa's fusion, 0 disagreements in 2·10⁷; the exact accumulator 2.96× a naive sum
        and order-independent. **Step 1, spec first** ([numbers.md](docs/spec/numbers.md) §9):
        - part 2 built ([numerals-2026-11-01](gauntlet/results/numerals-2026-11-01.md)): a numeral is
          exact, `core/num.go`;
        - part 3 built ([decimals-2026-11-01](gauntlet/results/decimals-2026-11-01.md)): `round`, `÷`,
          `D(k, R)` and ℤ ≤ N, each decimal held as its numerator at a static scale and every operation
          an integer one the decision proves (`ir/numbers.go`). `examples/num/ledger.oro` equals
          hand-written Go on three targets. The refinement layer's ℤ no longer reasons about a number
          (`emit/numnames.go`), and the thresholds are closed under the body's scalings;
        - part 4 built ([doubles-2026-11-01](gauntlet/results/doubles-2026-11-01.md)): F64 a set held as
          a double, F64 ∩ [lo, hi] a separation's normal form; a rounding onto F64 the call of the
          declaration whose `ensure` states it (`ir/doubles.go`, `fadd fsub fmul fdiv fint` on four
          targets, LLVM's since llvmf64-2026-11-06), comparisons with constants exact by a Galois connection, the scope, obligations on
          a real interval domain with Theorem A for a rounded accumulator. The gauntlet's float
          programs are the language's, dot, centroid and search one source each, at parity. A host's
          double is refused where the language's numbers read it; its type and decode are not built.
        - grids built ([grids-2026-11-01](gauntlet/results/grids-2026-11-01.md)): one rounding procedure
          for every grid (`core/grid.go`), checked against F(p, …)'s definition and against binary64;
          ADR 0087 closed the open-range hole it found ([resultclaim-2026-11-02](gauntlet/results/resultclaim-2026-11-02.md)).
        - **step 2 and this round's loose ends wait on a trigger** (below).
     **`ir/llvm`, spec first** ([llvm.md](docs/spec/llvm.md); its §9 the steps): steps 1, 2 and 4
     built. **The windows target prints LLVM IR** ([llvm-2026-11-02](gauntlet/results/llvm-2026-11-02.md)):
     every windows differential case agreed with MASM; the sieve and merge sort at 0.65 and 0.36 of
     `ir/x86`, the sieve process 0.64 of hand-written MASM; flags only where the IR proved them, each
     read back by a witness with a planted fault; `>>` past 63 total. **MASM retired**
     ([mastretire-2026-11-03](gauntlet/results/mastretire-2026-11-03.md)): `ir/x86`, `x64.oro`,
     `msvcrt.oro` and `lib/win/fmt.oro` deleted; the programs naming x86 instructions ported by ι⁻¹ to
     the language's operators; the Win32 survey writes `call` and `declare`, an integer parameter its C
     range by `trunc` and a narrow result by `sext`/`zext`, every pinned figure unchanged and the
     acceptance programs proven without `-checked`; a string carries a NUL after its bytes, so it is
     Win32's C string. **The wall, decided** (ADR 0091, [cstring-2026-11-04](gauntlet/results/cstring-2026-11-04.md)):
     a C string is the kernel of ν, the monoid homomorphism Σ* → (𝔹, ∨) true on U+0000; `nulfree` is
     in Σ_L, an `LPCSTR` parameter `where nulfree(a)`, decided by ν on the argument's term and by a
     guard; a UTF-8 manifest makes the `A` functions read our bytes. **The allocator is a term**
     (ADR 0092): a table's storage is an IR operation, `alloc(n, w)`, its `n·w + 8` proven.
   - **6b′. The LLVM target, fully, before the Windows app** (hamza, 2026-11-04; ahead of 6c and 6d). "Fully" is
     measured, not a count of declarations: every differential case runs on windows (no `skip:
     windows`), the gauntlet's seven programs run there at parity, and a second triple runs the suite.
     **The standing measure** (hamza, 2026-11-10): **the windows implementation is faster than the Go
     one**; as long as it is slower, the gap is work owed, done now or not. Today it is slower on every
     K(ℕ\*) program but two (fact 1.31×; fib(10⁵) 1.14× `math/big`; **the 2,000-quotient program 0.79×
     and the product of a numeral by a word 0.94×**, [carry-2026-11-13](gauntlet/results/carry-2026-11-13.md),
     [radix-2026-11-15](gauntlet/results/radix-2026-11-15.md)).
     ~~A quotient by 2³² as `sdiv`/`srem`~~, done there: on ℕ ∩ S₆₄ `/` and `%` are `udiv` and
     `urem` (truncation is the floor; the bits read alike), `llvm.udiv_nat`/`urem_nat` chosen by
     ADR 0088's narrowest domain, the product 0.55× its time. **M4, 64-bit digits, in steps**:
     ~~M4a, the double word~~ done (ADR 0103, [doubleword-2026-11-11](gauntlet/results/doubleword-2026-11-11.md)):
     an intermediate the decision proves in E = (−2¹²⁶, 2¹²⁶), read back into the word, is i128
     (`ir/double.go`, W14), so B = 2⁶³ is the largest base whose multiply-accumulate is proven; the
     Mersenne hash 0.49× idiomatic Go; ~~M4b, K(ℕ\*) over B = 2⁶³~~ done (ADR 0104,
     [natword-2026-11-11](gauntlet/results/natword-2026-11-11.md)): every step reduced by t & (B − 1)
     and t >> 63, D's q̂ the triple's 2-by-1 division (`divq`, `targets/windows/x64.oro`) where the
     two-digit lemma (a transfer for `/`) proves it a digit, render by 10¹⁸; fib 0.46×, fact 0.57×,
     D 0.50× their time. **Left, measured** ([carrysplit-2026-11-12](gauntlet/results/carrysplit-2026-11-12.md)):
     the `shld` splitting each step's t in the product, the sum and D4, a row 1.11 ns per digit
     against **0.37 with B = 2⁶⁴** (C, both forms); so **M4c, B = 2⁶⁴**, in steps: ~~1. E widened to
     192-bit ends and the double word's two readings~~ done (ADR 0105; the corpus byte-identical,
     compile 1.01×); ~~2–3. digits in U₆₄, the library at B = 2⁶⁴~~ done (ADR 0106,
     [nat64-2026-11-12](gauntlet/results/nat64-2026-11-12.md)): the double word chooses first, a
     table's elements by their hull, the product's row the machine's (`mulq`, `add`, `adc`); the
     2,000 quotients 0.78× Go, the product 1.8×, fib 1.9×, fact 3.2×. ~~Storage for a growing
     numeral~~ done (ADR 0107, [class-2026-11-12](gauntlet/results/class-2026-11-12.md)): a heap
     region a block of its power-of-two class, Σ_OS's `class` the capacity, a dead region serving
     every request of its class; the product 0.99× Go, fact 2.7×; reuse across a call not needed.
     ~~The sum's carry kept in the flags~~ done (ADR 0108, [carry-2026-11-13](gauntlet/results/carry-2026-11-13.md)):
     the full adder φ(x ± y ± c) the triple's `adc`/`sbb`, printed where IR_P's types prove it
     (`ir/llvm/carry.go`, llvm.md §3d), A and S two loops in sequence (each alone measured nothing);
     fib 1.9× → 1.23×, its time now about 78 ms of sums, 15 of printing, 15 of process.
     ~~Karatsuba~~ done (ADR 0109, [karatsuba-research.md](docs/karatsuba-research.md),
     [karatsuba-2026-11-14](gauntlet/results/karatsuba-2026-11-14.md)): above 64 digits a product is
     evaluation at 0, 1, ∞ and interpolation by a unimodular W, its levels a tensor power walked as
     three loops (down, leaves, up), each pass one loop over nodes × digits (nested, deciding
     `nat.mul` took 53 s; flat, 1.5 s); a rule's elements are ρ of their hull (`ir/words.go`).
     fib(10⁶) by fast doubling 4.1× → 1.67× Go. ~~Squaring by polarization~~ done (ADR 0110,
     [squaring-2026-11-14](gauntlet/results/squaring-2026-11-14.md)): x·x of one value is the
     quadratic form, a² = Σ aᵢ²B²ⁱ + 2Σ_{i<j} aᵢaⱼBⁱ⁺ʲ, Karatsuba's square the tensor power's diagonal;
     a square 0.54 of a product, fast doubling 1.57× Go. **Per operation, on a quiet machine, ours is
     4.2× Go's product and 3.3× its square at 10,400 words, and the C model of our design is already
     3×** (kernels-research.md §7). The leaf row alone is 1.6× Go's assembly, and LLVM's own `mul
     i4096` gets it to 1.3×. LLVM IR expresses two carry chains; its x86 code generator never selects
     `adcx`/`adox`. **Decided** (hamza 2026-11-15): a target may name its CPU and features (Q2, a
     feature: less portable, faster); a kernel is written **in the language** (Q1), reaching most of
     the gap through LLVM IR, with assembly only if a measurement still asks; Q3 falls away.
     **Postponed** to future work, in order: the design (subtractive, in place, the threshold
     re-measured), the leaf as LLVM's wide product, then assembly if needed; with Toom-3; and after
     them the subquadratic division at a node (Newton and Barrett, or Burnikel and Ziegler), printing's
     remaining 5–6× against Go's `String`. ~~Divide-and-conquer printing~~ done (ADR 0111,
     [radix-research.md](docs/radix-research.md), [radix-2026-11-15](gauntlet/results/radix-2026-11-15.md)):
     render from 32 digits is a tree of divisions by P_j = β^(2^j), β = 10¹⁸, each level one loop of
     Algorithm D **in place** (a node's slot is D's state: each digit stored in its window's top word,
     0 after the step; q̂ in the free word 2k; each node from its first nonzero window, q < B^(L−k+2));
     fact 2.77× → 1.31× Go, fib(10⁶) printed 502 → 106 ms. The leaf division cannot be
     cheapened: `divq` 12.1 ns a step, Möller–Granlund's reciprocal 12.0, both latency-bound
     (carry-2026-11-13, §printing's leaf). Earlier, divide-and-conquer printing
     measured at about 1.8× before Karatsuba, not more, while a D step costs 12 cycles (D4 split by
     its index domain, kept, inside the noise). Its causes, by expected gain
     (fitreuse-2026-11-10, lenonce-2026-11-10):
     ~~each operand's length reloaded per digit~~, done by β (a length is a pure function of an
     immutable value), not by `noalias`, whose scoped form is unsound for inlined code and whose
     parameter form waits on instances as functions; ~~the digit's mask~~, done
     ([digitfacts-2026-11-10](gauntlet/results/digitfacts-2026-11-10.md)): the digit's range is its table's
     element type, every store proven in [0, B) once the header is 2m + s (m < 2³¹, the machine's
     bound stated where it is written); ~~the sign case per digit, a sum's size and bounds tests~~,
     done ([addsub-2026-11-10](gauntlet/results/addsub-2026-11-10.md)): Knuth's A and S each its own
     loop (x + y = y + x, so only S orders its operands), A's size its carry's, the index domain split
     at the shorter size; fib 27% faster, at four inlined loop cases per sum (the division program
     21,102 → 29,579 operations, ten minutes in clang); ~~the same split for `mul_small`, `qsmall`,
     `rem_small`, `qrem` and `macrow`~~, measured on `mul_small` inside the noise (its size now its
     carry's) and not owed: the cost was the signed quotient (llvm-udiv-2026-11-11); ~~the library's operations as functions~~, step 1 done (ADR 0101,
     [progfunc-2026-11-10](gauntlet/results/progfunc-2026-11-10.md)): K(ℕ\*)'s large instances are
     functions of the program, decided once under their parameters' sets (a numeral's table
     `array int 0 4294967295`), the call's obligation its arguments in them, the frame rule
     (arguments borrowed, result a birth; W13 the callee's side); the division program 10 min → 19.5 s
     to compile, 483 → 73 KB, 10% faster to run; `examples/big/` 0.22–0.63× to compile; reuse across a
     call (destination-passing) and parameter `noalias` are steps 2 and 3; M4's 64-bit digits with `uadd.with.overflow` and i128 (native
     iN); a sum's size from its carry; render ~~two passes per chunk~~, one since
     [printpass-2026-11-10](gauntlet/results/printpass-2026-11-10.md) (fact 23% faster, now 9.0×); its
     divide and conquer gains nothing over quadratic D (m²/2 against chunking's 0.53m²), so it waits on
     **Karatsuba and recursive division** (Burnikel and Ziegler 1998), owed for every large product.
     In steps:
     1. ~~**The layering**~~ **done** (ADR 0093, [layers-2026-11-04](gauntlet/results/layers-2026-11-04.md)):
        `targets/llvm/` is T_LLVM[Σ_OS], a theory with a parameter (Σ_LLVM, ι's terms, the
        representations, and Σ_OS's headers, `region`, `write_err`, `exit`, `release`, `zero` and `region_raw` since ADR 0096 and 0097, `class` since ADR 0100, the capacity since ADR 0107); `targets/windows/`
        glues it (`glue llvm`, §2.1's ⊔), names its `triple`, and defines Σ_OS over kernel32; a header
        with no body is an operation symbol a glued layer must define; a run is exit(0) ∘ main.
     2. ~~**The C ABI in the layer**~~ **done** (ADR 0094, [cabi-2026-11-05](gauntlet/results/cabi-2026-11-05.md)):
        a C function is `host expr call "F" [stdcall | cdecl]`, its sets its C types (over C's integer
        types the full range determines the type, so a narrow integer's set is its full range and a
        narrower domain a `where`), its call out ∘ F ∘ in derived by the printer on the triple
        (`trunc` in, `sext`/`zext` out, a C string at s + 8, `dllimport` on Windows, `stdcall`
        x86_stdcallcc on i686); a prototype checked at load, a C string's `nulfree` required; the
        survey writes 10,479 prototypes and no template, its figures unchanged.
     3. **The layer's missing pieces, program first**, each skipped case un-skipped in turn:
        - ~~the unsigned word~~ **done** ([llvm64-2026-11-06](gauntlet/results/llvm64-2026-11-06.md)):
          U₆₄ is `i64` read unsigned (llvm.md §3a), the ring and bit operations S₆₄'s instructions, order,
          quotient, remainder and `>>` U's own; a comparison the facts decide is its truth value and its
          πs are dropped (Theorem E); render : ℤ → Σ* (`print_int`) is defined on ℤ and inlined, each
          call's facts choosing S or U. `codecs`, `fnv64`, `fnv64-fold`, `pairs`, `u64-digits`,
          `u64-square`, `u64-values` run on windows (the last three on the JVM too, a stale skip);
        - ~~F64's roundings~~ **done** ([llvmf64-2026-11-06](gauntlet/results/llvmf64-2026-11-06.md)):
          Σ_LLVM's `fadd fsub fmul fdiv sitofp`, each with its `ensure`, in LLVM's default environment
          (llvm.md §3b); a comparison is N's at its sort, `fcmp` with the ordered predicate where the
          operands are doubles, an operation at a sort no realization takes refused (`sortsHold`), and
          a comparison's realization in IR_P's text (`(gt via llvm.f-gt …)`). `f64-round` runs on
          windows, and the gauntlet's float programs (dot, centroid, smooth, search) emit there;
        - ~~`big_str`~~ **five of seven done** (ADR 0095, [llvm-render-2026-11-07](gauntlet/results/llvm-render-2026-11-07.md)):
          ι : (B₇)* ↪ Σ*, `ascii(b, lo, n)` in Σ_L, text from ASCII bytes, one copy on every target
          and an observer of a live buffer by its meaning; render : ℤ → Σ* per representation, the
          limb library's (`rem_small`/`div_small` by 10⁷, then ι) on limbs and the word's digit loop
          (`realize big_str = lang.render`) in the word; per-loop widening thresholds (below), which
          also prove `codecs` on windows. `big-mul`, `big-negmid`, `big-select`, `big-subdiv`,
          `word-wide` run there. The two walls were researched ([integers-research.md](docs/integers-research.md))
          and **decided** (hamza, 2026-11-09, R1–R4): `big-fib`'s unbounded ℤ needs a representation
          whose image is ℤ, so variable length (Proposition 1), and `big-sign`'s signed range is the
          same construction, Z(M), the group completion of the magnitudes. Found there: **nothing on
          the LLVM layer frees a table** (10⁶ limb additions hold 252 MB, 10⁷ 2.7 GB). In order:
          - **R1, storage by size class, released at death** (ADR 0096):
            - R1a. ~~release at death on the heap~~ **done** ([release-2026-11-09](gauntlet/results/release-2026-11-09.md)):
              ownership the greatest fixed point of birth, one move and borrow, a π the same
              reference; `release(storage(t), len t, w)` after the last use on each path not moving
              it, `storage` the IR's inverse of `store`, Σ_OS's fourth header `release` (VirtualFree
              or HeapFree by `region`'s rule); W11 re-derives each release; len ≤ 2⁴⁷ − 9 a fact of
              x64. M1: 10⁶ and 10⁷ limb additions peak at 3.5 MB (were 278 MB and 2.7 GB), 0.69× and
              0.58× the time; the sieve 1.007×. **And a build's buffer frozen into none of its
              results** (gauntlet-2026-11-16): own(b), the buffer's descendants closed by
              position, none yielded, so `release(p, n, w)` after the build; the JSON tree had
              leaked 18 KiB a call. W11 and W12 re-derive it. Not released: a function's result, the crash's
              newline, the helpers' strings (R1e);
            - R1b. ~~same-size reuse~~ **done** ([reuse-2026-11-09](gauntlet/results/reuse-2026-11-09.md)):
              release ; region(b) ≡ zero, so a dead region is a linear token, handed locally to the
              next allocation of its size and across a back edge as a body parameter (null on entry,
              consumed once on every path, by `reuse` or by `discard`); Σ_OS's `zero` is
              RtlZeroMemory; W12 re-derives every token. 10⁷ limb additions 186 ms (were 900 with
              release, 1,672 leaking); the sieve 0.89×. Static sizes only;
            - R1c. ~~a table of static length in a frame slot~~ **replaced by fill elision** (hamza,
              2026-11-09; ADR 0097, [fill-2026-11-09](gauntlet/results/fill-2026-11-09.md)): after
              R1b no steady state allocates, and zeroing was the 21% left. region = zero ∘
              region_raw, the zero dead when a table is born full: a rule or an array literal by
              construction, a build by the written prefix (`ir/fill.go`); `alloc-raw`, `reuse-raw`.
              10⁷ limb additions 136–143 ms, the no-zeroing bound (0.08× the original leak). Frame
              slots wait on a nested loop that measures its first iteration;
            - R1d. ~~a data stack~~ **replaced by reuse by fit** (ADR 0100,
              [fitreuse-2026-11-10](gauntlet/results/fitreuse-2026-11-10.md)): release(p, b) ;
              region(b′) ≡ zero(p, b′) for b′ ≤ b of one class, Σ_OS's seventh symbol `class`; the
              token carries its table's count k, `reuse_fit` compares at run time; W12 re-derives each
              (`PlantFitCount`, `PlantFitUnchecked`). K(ℕ\*)'s loops are not LIFO, so no stack holds
              them. Measured: fib 6% faster, the rest unchanged; **the allocation was not K(ℕ\*)'s
              cost** (natdiv's 4.4× read as R1d's trigger was wrong): its per-digit computation is,
              named under the standing measure (6b′); a growing numeral's was, at exact sizes, until ADR 0107's
              classes;
            - R1e **waits on a trigger** (hamza, 2026-11-09): strings and containment. 3 programs call
              the string helpers, a fixed few times each. Trigger: input-driven programs on windows
              (`os` there), which bring strings;
          - ~~**R2, Z(V_n)**~~ **done** (ADR 0098, amends ADR 0029 decision 4;
            [signlimbs-2026-11-09](gauntlet/results/signlimbs-2026-11-09.md)): a signed value a table
            of n + 1 entries, the sign in entry n, 0 at 0; every operation the magnitudes' under a case
            on the signs, Z_k enforced on the magnitude. `big-sign` and `big-signed-ops` run on
            windows; `ir.PlantSignFault` its witness's fault. Measured: 10⁷ signed additions 1,219 ms
            against 139 unsigned (8.5×), because **an allocation inside a statement-level `if` takes
            no token** (R1b and W12 walk only the terminator tree) and `with_sign` copies the
            magnitude. Not built: a quotient by a negative word (refused by name);
          - **R3, K(ℕ\*)** (ADR 0099), in steps:
            - R3a. ~~R2's storage finding~~ **done** ([signfuse-2026-11-09](gauntlet/results/signfuse-2026-11-09.md)):
              a conditional commutes with a table's construction, so the sign case is data inside one
              recurrence, x + σ·y with carries in {−1, 0, 1}, and the normal form's zero test an OR-fold
              (`addsub`, `sof`, `smul_small`); the written-prefix theorem holds on each path of a
              terminator tree, counts equal by congruence. 10⁷ signed additions 214 ms (were 1,219),
              1.55× unsigned, every allocation a token; no pass changed. Not built: a token through a
              branch, which nothing now needs;
            - R3b. ~~ℕ\* and K(ℕ\*)~~ **done** ([natnum-2026-11-10](gauntlet/results/natnum-2026-11-10.md)):
              `emit/natnum.oro`, digits in base 2³² over U₆₄ (2⁶⁴ since ADR 0106), the signed size h = ±m last (GMP's), every
              build born full; A and S as one recurrence, M, the word forms, render by 10⁹; an unbounded
              set on a target with no bignum plans `Var`; `big-str`'s argument demanded on every target
              (render is ℤ's); the machine's bound len t ≤ max_len a crash past it, as a case's guard;
              Houdini's frame template both ways (v ≤ z, `PlantNoUpperFrame`). `big-fib` and
              `big-nat-ops` run on windows. fib(10⁵) 257 ms against `math/big`'s 29–35 (≈ 8×). Not
              built: D, an entry point's K(ℕ\*) parameter, render in place;
            - R3c. ~~Algorithm D~~ **done** ([natdiv-2026-11-10](gauntlet/results/natdiv-2026-11-10.md)):
              long division a Mealy machine whose state is the partial remainder, q̂ by Theorem B and
              D3, at most two add-backs (`PlantNoAddBack`); a bounded program V_n cannot finish
              **moves to K(ℕ\*) whole**, its set enforced by sign and bit length (`fit`,
              `PlantNoNatFallback`); `HoldsZ`, the capability read off the declaration. M3: fact 11.3×,
              D 3.1×, fib 8.9× `math/big`; V₉ 4.4× faster than K(ℕ\*) on bounded additions, so V_n stays,
              and R1d's trigger is met. `examples/big/` emit on windows. Not built: M4 (needs native iN);
          - **R4**: nothing enters the language; ρ gains the representations.

          And `u128-fact`, `u128-ops` test a Go-only library (`lib/num/u128` over `go/math/bits`),
          so skip every target but Go: whether they count against "fully" is hamza's, after R1–R4;
        - ~~`os` on windows (`read-loop`, `text-of`)~~ **done** (ADR 0112, [oswin-research.md](docs/oswin-research.md),
          [oswin-2026-11-15](gauntlet/results/oswin-2026-11-15.md)): `lib/os/windows.oro` and `lib/io/windows.oro`,
          the first cells written in J over kernel32. A Win32 failure is a term (the sentinel, then
          GetLastError, each name's sum built once at its tail); an out-parameter is ADR 0048's cell, an
          `alloca` on LLVM; V, the valid UTF-8, is recognized by its monoid, `utf8_valid` and `utf8_text`
          in Σ_L (strings.md §11), text_of = ι_V ∘ r. Both cases run on four targets. `roundtrip.oro`,
          `ledger` and `freq` emit on windows since P8 (ADR 0113, p8-2026-11-15), the cell's definitions
          kept as functions. Then the gauntlet's programs on windows, which meet the same scale;
        - ~~**then a skip states a true reason**~~ **done** ([harness-2026-11-16](gauntlet/results/harness-2026-11-16.md)):
          a skip is a claim, `// skip: T because "TEXT"`, evaluated on every run: the skipped
          target is built and run, a stale skip or a wrong reason fails the case (one found stale,
          `word-wide-native` on JavaScript); `cmd/check`'s `assemble` step puts every windows `.ll`
          through clang (105 files, 3 s); an export's K(ℕ\*) natural is a numeral's table, its
          digits proven by its stores, so examples/big's fib, pair, fact and power emit on windows
          and their files hold `nat.add`'s full adder, the product and D (W5 had reported two fixed
          representations in Theorem D′'s class); the interval route's lowering error is joined to
          the refusal it causes; `book` runs every committed chapter. Each with a planted fault.
     4. **The second triple**: `i686-pc-windows-msvc` runs the suite (TinyRetroPad is 32-bit).
     Left to the walls: LLVM's widths (native iN, i32 scalars on i686) and the rung above as native iN,
     measured against the limbs.
   - 6c. **TinyRetroPad's Win32 surface, measured**: downloaded 2026-10-24 at `42f4c6c` as data, never
     built or run; 2,686 lines of 32-bit `stdcall` MASM, 30 procedures, 67 Win32 functions over kernel32,
     user32, gdi32, comdlg32 and shell32, RichEdit loaded at run time.
   - 6d. **Its walls, spec first, an ADR each**: the window procedure (callbacks tier 2, a function pointer
     LLVM gives its calling convention), its structs (records, laid out by the target's data layout); 32-bit
     is ADR 0080's triple.
7. ~~A π is justified by its branch, and the verifier checks it~~ **done**
   ([pijustified-2026-11-02](gauntlet/results/pijustified-2026-11-02.md)): W6 is Theorem E, each π a
   literal its arm's guard implies, the literals computed as a meet over the guard's routes (spec/ir.md
   §6); faults planted for the wrong table, a flipped relation, a foreign operand, a missing negation, a
   π no branch enters, and a conjunction's false side.
8. ~~Two defects the LLVM printer's programs named~~ **done** (llvm-2026-11-02,
   [quotient-2026-11-03](gauntlet/results/quotient-2026-11-03.md)):
   - ~~the refinement layer refused a goal it held as a fact~~ **fixed**: a range obligation inside the
     word that the interval route could not lower (a table of tuples) was refused as if above the word;
     it is decided by the facts (`emit/requires.go`, `TestARangeMarkTheIntervalsLeftIsDecidedByTheFacts`
     with the bug planted);
   - ~~−2⁶³ / −1 is decided by nothing~~ **fixed** (integers.md §4a): |a / b| ≤ |a| with equality iff
     |b| = 1, so `/` is decided like `+`, its interval exact by the divisor's sign regions; a checked
     form on every target; three programs dividing an unbounded `Int` narrowed to the symmetric word;
     MASM's `irem` answers ℤ's 0 at −2⁶³ % −1, where `idiv` raised #DE.
9. **An `again` after a nested loop's join point** (tables.md §2.5): 16% of `wc` against idiomatic Go
   ([wc-2026-10-24](gauntlet/results/wc-2026-10-24.md)). Research it as a join point that may jump, build
   it, measure `wc` nested.
10. **The Go library, program first**: `cmp`, two files compared chunk by chunk.
11. **The analysis grows only for a refusal a program names**, each rule's planted fault with it.
12. **The JVM's generated JSON tokenizer slows from 8 to 22.5 µs mid-run** (bits-2026-10-27 §2), before
    and after ADR 0083: find the cause (a deoptimization, a profile, or the emitted shape), against
    hand-written code. A gauntlet program, so ahead of new programs once 6b is done.
13. **When to reduce and when to call**, **decided** (ADR 0102, hamza 2026-11-11, Q1–Q4 as recommended;
    [specialization-research.md](docs/specialization-research.md)): the choice of what to
    unfold never changes a meaning (Theorem 1), only proofs, size and time; the static level is always
    unfolded; interval analysis is not distributive, so one contract proves less than the sites
    (Theorem 3, `t * k`), and lifting a function's unproven obligations to its callers as preconditions
    recovers them where expressible (Theorem 4, ADR 0037 at a boundary); folding by pattern after
    reduction is the NP-hard smallest-grammar problem, folding by origin linear and exact. **Factoring
    never changes an answer or a verdict** (P4, built for ADR 0101's instances: a callee its contract
    does not prove, or a call outside it, is inlined; `PlantNoInlineFallback`); no folding by pattern.
    **M-S1, M-S2, M-S4 measured** ([factoring-2026-11-11](gauntlet/results/factoring-2026-11-11.md),
    `gauntlet/factoring/measure.go`): keeping the headed definitions saves 2.5–3% of `examples/`'
    residual, every dynamic one 30–36% (headerless: `json/tree`'s `parse` and `walk` ×4); 87% (Go) and
    70% (windows) of headed definitions are proven under their contract, the rest an obligation over
    the parameters Theorem 4 lifts (4), ρ at the boundary (1), or higher-order by result (2: dynamic
    means first-order in the whole type); no widening non-monotonicity found; the 2,000-quotient
    program inlined is 7.1–7.6 min, 70–74% clang's. **P8, step 1, built** (ADR 0113,
    [p8-2026-11-15](gauntlet/results/p8-2026-11-15.md)): on a target whose printer prints functions
    (windows), the compiler keeps each dynamic definition that pays (|f| · max(π − 1, 1) ≥ 64) and that
    its contract proves. The source names nothing. A verdict is monotone in K: every program accepted
    with K = ∅ is accepted, with the same answer, by P4's net. An unstated result's type is its hull,
    never typing's (`PlantTypingResult`), and a lent buffer is written in place (`sameStorage`, never
    pure). `roundtrip`, `ledger` and `freq` emit on windows; run time 0.985–1.03; Go, JavaScript and
    the JVM byte-identical. **Step 3, built** ([p8step3-2026-11-15](gauntlet/results/p8step3-2026-11-15.md)):
    the contract is a Hoare triple {P ∧ φ} f {ψ}, each half decided by the refinement layer. A kept
    call carries its `where` as a mark, decided with the call's facts, and a refusal names its
    definition, so P4 inlines that one alone. An obligation over the parameters is lifted into φ. ψ is
    Houdini's greatest subset of templates (len(r) = x, r ≤ len(p), …) decided at every value tail. ℤ
    or an unstated integer is the instance's S ∩ W (Proposition D), and a call on literals is its
    value. The net catches 2 programs, not 5; no verdict or unproven count changed; run time
    0.97–1.03. Found there: `hostWrites` never read a `branch` arm (`PlantWritesNoBranch`). Owed:
    `readinto`'s n ≤ len(b) (a relational loop invariant, so `readfile`'s clamp stays), a segment at
    a kept boundary (`pe`), affine templates, instances for U, and per-call fallback.

**Deliberately not next**: ARM, Linux and macOS targets themselves; loop-arm patterns and `try`'s
handlers; a result claim per instance; the clause chain's copying, `e2`, `lent`; optimizing the compile;
a join across representations; hoisting or CSE on the IR (it would reach two premises witnessed only on
hand-built IR); per-result enforcement above the word and
signed limbs (ADR 0029); the higher-order contract gap (ADR 0028); records, `with` and named views beyond
what TinyRetroPad's walls need.

**Waiting on a trigger** (researched, built when a program needs it):
- **Numbers, step 2: exact interiors** (numbers.md §5, floats-research §14.5): a term of N held
  exactly inside a program and rounded once, by expansions and Kulisch's accumulator. It owes the cost
  measurement (an exact sum was 2.96× a naive one). Trigger: a program wanting a correctly rounded sum
  or dot product, or an exact geometric predicate (floats-research §18.3, §18.5).
- **Numbers, the round's loose ends** (doubles-2026-11-01, grids-2026-11-01):
  - the host's double as its own type (`go.float64`, …) and the decode `num.of_double` (numbers.md
    §6.3); today a host's double is refused where the language's numbers read it, and a Tier 2
    declaration still types its result F64. Trigger: a program reading a host's double, or the next
    change to the targets' float declarations or the surveys' `float64` map;
  - the checker's N at its boundaries: a rounding onto F64 is typed as the double, since typing it N
    breaks host values and `Array(F64)`; the fix is N's subsets as ADR 0033 gave ℤ's. Trigger: the
    next defect it causes, or step 2;
  - `F(p, emin, emax)` as a set a program names (F32, BF16, FP8): the procedure takes any p, but J has
    no former and no target realizes those roundings. Trigger: a program needing one;
  - `sqrt` and a fused multiply-add as realizations (`math.Sqrt`, `math.FMA`; JavaScript has no fma),
    the directed modes onto F64, `-checked` for a rounding, a rounding onto F64 of a decimal;
  - F(p, …)'s and D(k, R)'s definitions in `lib/num.oro`, which need replacement sets (ADR 0061), and
    D(k, R)'s separation normal form.
- **Win32's strings, the rest of ADR 0091**: a C string from Win32 into ours (`strlen` and a copy,
  total, into ν's kernel), trigger TinyRetroPad's `GetWindowTextA`; the list of C strings,
  (B*₀)* → bytes, trigger its open dialog's filter; the `W` functions' UTF-16, trigger a `W` function.
  ~~`string_of` and `++` on windows as terms over the bytes~~ **done** (ADR 0117,
  [strings-2026-11-16](gauntlet/results/strings-2026-11-16.md)): the four string operations are
  definitions in the language on the LLVM layer, V a submonoid deciding `++`, η agreeing on every
  scalar, the NUL the allocator's; owed: ι_V as the identity, large string terms called as functions.
  ~~kernel32's segment calls as text~~ **done** (ADR 0118, [cabi2-2026-11-16](gauntlet/results/cabi2-2026-11-16.md)):
  a C call's arguments are a linear term over its parameters, `elem(b, lo)` and `null` among them,
  so `ReadFile` and `WriteFile` are derived and `@__written` is gone; no template in the LLVM or
  windows layers is longer than one instruction (`divq` the one that is no LLVM instruction).
- **One integer rendering on every target**, render : ℤ → Σ* per representation (ADR 0095 built it on
  windows): on Go, the JVM and JavaScript a word value is still widened to the host's bignum to be
  printed, a hidden allocation. Trigger: printing's P3.
- **Storage, the rest of R1** (ADR 0096, 0097, 0100): strings from the helpers and a table inside a
  table, borrowed or copied (R1e); frame slots for a nested loop's first iteration; a token across a
  branch. Trigger: input-driven programs on windows, or a measurement naming one.
- **A 32-bit scalar representation** (ADR 0083 item 2's register rule): FNV-1a 32 masks each step
  with `& 0xFFFFFFFF` where a `uint32` would not (1.08–1.12, inside the noise floor). Trigger: a loop
  whose mask is measured above the noise floor, or `ir/llvm`'s i32. **Met** (files-2026-11-16): `wc`'s
  newline count is 64-bit lanes where C's is 32, 1.43× C's work, the whole gap (a C `long long`
  counter does the same); the second triple needs i32 scalars too.
- **The Boolean operations above the word** (ADR 0082): an operand or result held in arbitrary
  precision is refused. Trigger: a program needing the bits of a big value (`math/big`'s `And`).
- **Integers' four old mechanisms re-declared as (H, d) data** (ADR 0083 item 1; Hoare 1972): the new
  representations are declared so (bits-2026-10-27), the word, the unsigned word, limbs and the bignum
  are still separate code. Trigger: the next change to any of them, or a second string encoding
  (Win32's `W` functions).
- **Validity through functions that preserve it**: refinement sorts (Freeman and Pfenning 1991) for
  `strings` over V ⊂ B*. Trigger: a program paying for `go.text` right after a `strings` call.
- **Concurrency's** six questions and surface (ADR 0081 decided the direction) and six decisions on
  **Android's toolkit** ([android-research.md](docs/android-research.md)); **printing**'s three algebras
  ([printing-research.md](docs/printing-research.md)); a borrow a later call ends (`Scanner.Bytes`,
  [borrow-research.md](docs/borrow-research.md)); §1.4's reservation of J's words.

### What is measured

- **The gauntlet** ([docs/gauntlet.md](docs/gauntlet.md)), the one fixed commitment: seven programs at
  parity with hand-written code. Last full benchmark [gauntlet-2026-11-16](gauntlet/results/gauntlet-2026-11-16.md),
  on four targets: geometric means Go 1.001, JavaScript 0.965, Java 1.012, windows 0.978 (against
  hand-written C by the same clang), each residue above 1.05× explained. Windows runs five of the
  seven (G4 and G3's tally need a dictionary keyed by strings, G5 F64's rendering: walls). The 33
  files its harnesses call are listed in `gauntlet/regen.txt`, written by `bash gauntlet/regen.sh` and
  regenerated and compared by `cmd/check`'s `gauntlet` step; windows' harness is `gauntlet/windows/`.
- **Provability**: **8,852 of 8,906** integer operations proven inside their target's word, the 54 left
  meant to be refused (`examples/int/`, `kara`) or in a case whose `run` takes an unbounded parameter
  (`shift-div`, `limb-subdiv`) or a host map's count (`wordcount`);
  **924 of 962** loops proven to terminate (size-change termination; fourteen of the 38 left are
  windows' `os` cell's read and write loops, whose progress is a count the host returns). A
  definition windows keeps as a function (ADR 0113) is counted once per program, so the totals fell
  with P8 while no program's unproven count rose (p8-2026-11-15, p8step3-2026-11-15); they rose with the
  JSON programs and `generic-win` emitting on more targets, the unproven unchanged (gauntlet-2026-11-16).
  Each program's share is in its
  result document; both counts are pinned by `cmd/check`. K(ℕ\*)'s instances (ADR 0101) are proven
  once per target, under their parameters' sets, and are not counted per program.
- **Host APIs** (surveys, declarable / usable): Go 87.8% / 60.5%, the JVM 81.4% / 60.9%, Win32 90.5%
  declarable and 31.0% callable and linkable; `go run gauntlet/stdlib/coverage.go` gives the declared
  share of the Go packages with a file, with a cause for every missing name
  ([gocoverage-2026-10-06](gauntlet/results/gocoverage-2026-10-06.md)). **Supported** (declared by hand,
  checked against the host, exercised by a program, ADR 0022): `unicode/utf8`, `encoding/hex`, `strconv`,
  `encoding/binary`'s varints, `math/bits`, `io`'s interfaces; **partial and checked**: `os`, `strings`,
  `io`, `fmt`, `errors`, `io/fs`, `path/filepath`, `time`, `bufio` and its `Reader`, `unicode`. The
  portable `os` (`lib/os.oro`) runs on Go, JavaScript and Java with one output.
- **The balance**, named in every assessment: the compiler grows faster than the code written in the
  language. `python gauntlet/balance.py [--nodes] [REV…]` reproduces every assessment's table. At
  `b22d553`: compiler 73,854 lines (24,461 of them tests; `core` 20,257, `emit` 33,442, `ir` 20,155), the
  language 2,086 lines; the 10-24 round's margin 20.6 : 1. The largest program is still `freq`, 194
  lines. **Write programs, and let them demand the analysis.**

## The compiler

`go run ./cmd/check` passes every step: vet, compiler, emission, gauntlet, ir, differential, tooling;
emission first and alone (its gate times compiles), the rest at once, about 4½ minutes, plus 7 s for
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
  (ADR 0041), each arm of a `case` in a tail (ADR 0076); never under an `if`. In the reader the rule is a
  sort. Every walker of a clause chain walks all four forms, or it is unsound. **No recursion**
  (ADR 0014); a balanced recursion is a loop over levels.
- **Relevance** (ADR 0043): a value of a `relevant` variant is used on every path, `ignore(e)` the
  discard. **Failures are values of a marked sum** (ADR 0040): `lib/result`, `try`, `expect`, the host's
  encoding a niche of its error type.
- **Closures** may not survive staging; host callbacks' tiers 1 and 2 are specified
  ([callbacks.md](docs/spec/callbacks.md)) and not built.
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
  point; an `again` after it is not built. Tuples `(a, b)` and the unit `()`; a table of tuples is
  flattened by currying ([products.md](docs/spec/products.md)).
- **Variants**: `set S = 'a ∪ 'c(T)`, closed, finite, non-recursive, with type arguments; at a boundary its
  tag and one slot per payload type (ADR 0042, 0059).
- **Strings** are the free monoid over Unicode scalars, `++` and `string_of`; a host's string is the
  host's (`go.bytestring`), entered by one total decode, `go.text` (ADR 0030,
  [strings.md](docs/spec/strings.md)). **A C string is the kernel of ν** (ADR 0091, strings.md §9):
  `nulfree(s)` in Σ_L, a C-string parameter `where nulfree(s)`, decided by ν's homomorphism on the
  argument's term, and by a guard. **ι : (B₇)* ↪ Σ\*** (ADR 0095, strings.md §10): `ascii(b, lo, n)`,
  a table of scalars below 128 as text, total, one copy on every target, an observer of a live
  buffer by its meaning; what render ends in. **Not built**: records, symbols, `with`.

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
  site, a baseline built from `git stash`. Time a baseline in a worktree. Make a suspicious result explain
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
| The arrow is a claim on a host's callback parameter (not built) | [0053](docs/decisions/0053-the-arrow-is-a-claim-on-a-host-parameter.md) |
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
| The native targets print LLVM IR, stating the IR's proofs; MASM retired at parity; every LLVM primitive offered, walls to hamza (built; MASM retired 2026-11-03) | [0080](docs/decisions/0080-the-native-targets-print-llvm-ir.md) |
| Concurrency is typed processes: a Mealy machine, failure an effect, supervisors its handlers; Kahn networks and fork–join its fragments (the direction; not built) | [0081](docs/decisions/0081-concurrency-is-typed-processes.md) |
| Wrapping is the residue map `mod 2ⁿ` on ℤ, not a sort; `& \| ⊕ ~ << >>` are ℤ's own; fusion computes a term under `mod 2ⁿ` in n-bit arithmetic | [0082](docs/decisions/0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md) |
| A representation is an abstraction function (H, d), chosen by place; native iN above the word on LLVM; the word S₆₄ on every triple (LLVM's part with ir/llvm) | [0083](docs/decisions/0083-a-representation-is-an-abstraction-function-chosen-by-place.md) |
| An address is an integer offset in a region, ownership a linear view in the logic; no load without a view; binders abbreviate view formulas; a definition's linear result; uninit never visible; codecs (not built) | [0084](docs/decisions/0084-an-address-is-an-integer-and-ownership-is-a-view.md) |
| A vector is a representation of a table, never a sort; a fold vectorizes iff its operation is associative; a float fold keeps its order (item 3 amended by 0086; not built) | [0085](docs/decisions/0085-a-vector-is-a-representation-of-a-table-and-a-fold-vectorizes-iff-it-is-a-homomorphism.md) |
| A number is an element of ℤ[1/10]; `round(S, e, mode)` is its one lossy operation, `÷` and the transcendentals at its root; the facts choose ρ; IEEE's arithmetic a static scope; no NaN or ∞; a target realizes a rounding by its `ensure` (not built) | [0086](docs/decisions/0086-a-number-is-an-element-of-z-tenth-and-a-rounding-is-its-one-lossy-operation.md) |
| A result's set is a claim by its ends inside the word, decided on ℤ's facts before any representation; an end past the word, and a bounded range's sign end held above it, ADR 0029's at run time; amends 0029 | [0087](docs/decisions/0087-a-results-set-is-a-claim-by-its-ends-inside-the-word.md) |
| A target states its realization of each language name, chosen per instance by proof; a host operation is declared as it is, its domain its `where` (built for windows; the others by spelling until restated) | [0088](docs/decisions/0088-a-target-states-its-realizations-and-a-host-operation-is-declared-as-it-is.md) |
| A realization is a term over the host's operations, ι : Σ_L → T(Σ_H), checked defined and agreeing, substituted after the decision; templates only for a host's own operations (amends 0088) | [0089](docs/decisions/0089-a-realization-is-a-term-over-the-hosts-operations.md) |
| ∅ is a set; a function into it never returns (0 initial, k ∘ h = h), every declaration not only `abandon`; a host's product taken apart by its projections; legality decided before ι | [0090](docs/decisions/0090-a-function-into-the-empty-set-never-returns.md) |
| A C string is the kernel of ν, the monoid homomorphism true on U+0000; `nulfree` in Σ_L, decided by ν on the argument's term and by a guard | [0091](docs/decisions/0091-a-c-string-is-the-kernel-of-the-nul-homomorphism.md) |
| A table's storage is an operation of the IR, `alloc(n, w)`, and the allocator a term over the host's, total by exhaustion; ι does not change what the program proves | [0092](docs/decisions/0092-a-tables-storage-is-an-operation-and-the-allocator-a-term.md) |
| The LLVM layer is a theory with a parameter, Σ_OS (`region`, `write_err`, `exit`), and an OS target instantiates it by gluing; a header with no body is an operation symbol; the triple is target data | [0093](docs/decisions/0093-the-llvm-layer-is-a-theory-with-a-parameter-and-windows-instantiates-it.md) |
| A C function is declared by its prototype, its sets its C types (a narrow integer's full range); its call derived by the C ABI on the triple | [0094](docs/decisions/0094-a-c-function-is-declared-by-its-prototype-and-the-c-abi-is-derived.md) |
| Text from ASCII is the inclusion ι : (B₇)* ↪ Σ*, `ascii(b, lo, n)` in Σ_L; rendering is ℤ's at every sort, the host's numeral, the limb library's render, or the word's digit loop, each ending in ι | [0095](docs/decisions/0095-text-from-ascii-is-the-inclusion-and-rendering-is-integers-at-every-sort.md) |
| A frozen value's view is counted fractions, not persistent; an owned table is released at its death, `release` an IR operation over Σ_OS; storage chosen by size class (frame, data stack, heap), built in steps; W11 checks every release | [0096](docs/decisions/0096-a-values-storage-is-chosen-by-size-class-and-released-at-its-death.md) |
| A table born full takes a region with no zero fill: region = zero ∘ region_raw, a rule and a literal by construction, a build by the written prefix; replaces R1c's frame slots | [0097](docs/decisions/0097-a-table-born-full-takes-a-region-with-no-zero-fill.md) |
| A dead region serves a request it fits: release ; region(b′) ≡ zero for b′ ≤ b of one class (Σ_OS's `class`), the token carrying its table's count, compared at run time; replaces R1d's data stack (amends 0096 item 6) | [0100](docs/decisions/0100-a-dead-region-serves-a-request-it-fits.md) |
| A library instance is a function of the program, decided once under its parameters' sets, the call's obligation its arguments in them, the frame rule its ownership (W13); amends 0035 | [0101](docs/decisions/0101-a-library-instance-is-a-function-of-the-program.md) |
| Factoring never changes an answer or a verdict: a definition kept as a function only where its contract, completed by lifted preconditions, proves it, inlined otherwise; the static level always unfolded; no folding by pattern; amends 0101 | [0102](docs/decisions/0102-factoring-never-changes-an-answer-or-a-verdict.md) |
| An intermediate above the word is computed in the double word, i128, where the decision proves it in E and its uses read it back into the word; no program value leaves the word (W14); a divisor a constant; extends 0082's fusion, builds 0083 item 3 for intermediates | [0103](docs/decisions/0103-an-intermediate-above-the-word-is-computed-in-the-double-word.md) |
| K(ℕ\*) is in base 2⁶³, the largest the decision proves (B² − 1 ∈ E); the two-digit lemma a transfer for `/`; a quotient by a word the triple's 2-by-1 division; reverses 0099 item 3, amends 0103 item 4 | [0104](docs/decisions/0104-k-n-star-is-in-base-2-63-the-largest-base-the-decision-proves.md) |
| The decision's ends are 192-bit, E = (−2¹⁹⁰, 2¹⁹⁰); the double word has two readings, (−2¹²⁷, 2¹²⁷) signed and [0, 2¹²⁸) unsigned; amends 0103 | [0105](docs/decisions/0105-the-decisions-ends-are-192-bit-and-the-double-word-has-two-readings.md) |
| K(ℕ\*) is in base 2⁶⁴, a digit a natural of U₆₄, every step the double word's read back by its halves; the double word chooses before the unsigned word; a table's elements by their hull; reverses 0104 item 1 | [0106](docs/decisions/0106-k-n-star-is-in-base-2-64-the-machines-digit.md) |
| A region is answered by its capacity class: a heap block of 2^⌈log₂ b⌉ bytes, Σ_OS's `class` the capacity, a dead region serving every request of its class; amends 0100 | [0107](docs/decisions/0107-a-region-is-answered-by-its-capacity-class.md) |
| The full adder is the triple's (`adc`, `sbb`), printed where IR_P's types prove φ(x ± y ± c), c ∈ {0, 1}, every use a half; a fold over a split index domain is two loops | [0108](docs/decisions/0108-the-full-adder-is-the-triples-and-a-fold-over-a-split-domain-is-two-loops.md) |
| A product of numerals above 64 digits is Karatsuba's, evaluation at 0, 1, ∞ and a unimodular interpolation; its levels a tensor power, one loop a pass over nodes × digits; a rule's elements ρ of their hull | [0109](docs/decisions/0109-a-product-is-evaluation-and-interpolation-and-its-levels-are-a-tensor-power.md) |
| A square is the product's quadratic form, by polarization (half the digit products), recognized by its operands' identity; Karatsuba's square is the tensor power's diagonal; each pass reads frozen tables | [0110](docs/decisions/0110-a-square-is-the-products-quadratic-form-by-polarization.md) |
| Printing is a tree of divisions by β^(2^j), β = 10¹⁸, each node Algorithm D in place: the digit stored in its window's top word, q̂ in the free word 2k, a node from its first nonzero window; extends 0095 | [0111](docs/decisions/0111-printing-is-a-tree-of-divisions-each-node-algorithm-d-in-place.md) |
| `os` on windows is J over kernel32: a Win32 failure is a term, an out-parameter ADR 0048's cell (an alloca), V the valid UTF-8 recognized by its monoid, `utf8_valid` and `utf8_text` in Σ_L, text_of = ι_V ∘ r | [0112](docs/decisions/0112-os-on-windows-is-a-library-over-kernel32-and-valid-utf-8-is-recognized-by-its-monoid.md) |
| A program's definitions are kept as functions where the target prints them (windows), the compiler choosing K by pays and proofs; a kept definition is {P ∧ φ} f {ψ}: φ marked at each call and completed by lifting, ψ derived by Houdini, ℤ the instance's S ∩ W; a verdict is monotone in K | [0113](docs/decisions/0113-a-programs-definitions-are-kept-as-functions-where-the-target-prints-them.md) |
| A fold is an equation between pure terms: the language's operators fold only where the primitive is pure | [0114](docs/decisions/0114-a-fold-is-an-equation-between-pure-terms.md) |
| A rule at an export's boundary is its extension, `alloc(rule)`; in the interior a rule fuses or the author writes `alloc` | [0115](docs/decisions/0115-a-rule-at-an-exports-boundary-is-its-extension.md) |
| A statement whose value is a function is refused; `ignore(f)` is the discard | [0116](docs/decisions/0116-a-statement-whose-value-is-a-function-is-refused.md) |
| A string is the table of its UTF-8 on the LLVM layer, its four operations definitions in the language read back by ρ⁻¹ (V a submonoid decides `++`, η agrees on every scalar); every table's region one zero byte past it, the C string's NUL; a realization's host `where` decided; an identity coercion a move | [0117](docs/decisions/0117-a-string-is-the-table-of-its-utf-8-and-its-operations-are-the-languages.md) |
| A C call's arguments are a linear term over its parameters: each a parameter, `elem(b, lo)` (&b[lo], defined where the `where` states lo ≤ len(b)) or `null`, every parameter exactly once; one Win32 function one declaration; `store8` split into single instructions; amends 0094 | [0118](docs/decisions/0118-a-c-calls-arguments-are-a-linear-term-over-its-parameters.md) |
| A store owes its table's domain, 0 <= i < len, as a read does; lang's guarded facts instantiated where an obligation is; a square an atom (F12); where the fragment cannot bound a store the program guards it, a missing proof (the builder, K(ℕ\*)'s carrier owed) | [0119](docs/decisions/0119-a-store-owes-its-tables-domain.md) |
| A map on the LLVM layer is the table of its cells, each operation a term realized after the decision on the IR; the pre-reduction rewrite and `repr map library` retired | [0120](docs/decisions/0120-a-map-on-the-llvm-layer-is-realized-after-the-decision.md) |
| A map of capacity c is in {m ∈ Map(K, V) : len(m) ≤ c}, a claim; S marks each insert len(m) < c, decided from |dom| = 0 at map(c), an insert adding at most one key, and Houdini's |dom m| ≤ i | [0121](docs/decisions/0121-a-maps-capacity-is-a-claim-on-its-len.md) |
| An unbounded integer on a target with no bignum is K(ℕ\*): a numeral of any length in base 2³² over U₆₄ (base 2⁶⁴ since 0106), its signed size last; Knuth's algorithms; render demands an exact integer on every target; a numeral held while its table fits, a crash past it | [0099](docs/decisions/0099-an-unbounded-integer-is-a-numeral-of-any-length-with-its-signed-size-last.md) |
| Fixed limbs realize a signed set by the group completion: Z(V_n), a sign entry over the magnitude, sign 0 at 0, every operation derived from the magnitudes', Z_k enforced on the magnitude; amends 0029 decision 4 | [0098](docs/decisions/0098-fixed-limbs-realize-a-signed-set-by-the-group-completion.md) |

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
bash gauntlet/regen.sh                                          # regenerate the gauntlet's 33 files
cd gauntlet/go && go test -bench='TreeGen|TreeFlat$' -benchtime=20000x -count=5   # generated vs hand-written
```

- `gauntlet/go`, `gauntlet/js`, `gauntlet/java` and `experiments/legibility` are separate modules: `cd`
  into them first. `gauntlet/fmt/*.go` are `//go:build ignore` scripts run with `go run`.
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
| `targets/` | target declarations, data and not Go: `go/`, `js/`, `java/`, `llvm/` (the LLVM layer, T_LLVM[Σ_OS]: `instructions` Σ_LLVM, `lang` ι's terms, `os` Σ_OS's headers, `str`), `windows/` (glues `llvm`: its triple, `os` the view over kernel32, `kernel32`, `strings`); `blas` and the `tutorial*` targets parameterize the normal form for the book |
| `lib/` | library modules: `os`, `io` (portable names over each host), `result`, `num` (`num.oro`: a number's decimal numeral; `num/` the older vectors), `win` (`print`, in the language's operators) |
| `cmd/` | `check`, `build`, `gen`, `oro`, `intervals`, `portable` |
| `examples/` | the programs: `io/` (`wc`, `count`, `jsonfmt`, `roundtrip`, `freq`), `os/` (`du`, `ls`, `wc`), `lines/` (`lines`, `ages`, `shape`), `json/`, `big/`, `hash/` (FNV-1a and CRC-32), `num/` (`ledger`, decimals), `int/` (meant to be refused), `kara/`, `tally/`, `u128/`, `native/` (the gauntlet's sources) |
| `gauntlet/` | hand-written references, `results/`, `check/` (the baseline), `differential/` (cases on four targets, proving their arithmetic unless `; checked:` says why), `conformance/`, `regen.txt`, `balance.py`, `surface/` (the surface's measurements) |
| `gauntlet/stdlib/` | the four host surveys (writing J through `jdecl`), `coverage.go`, `callbacks.go`, twenty acceptance programs, and `tooling_test.go`, which checks every hand declaration against its host |
