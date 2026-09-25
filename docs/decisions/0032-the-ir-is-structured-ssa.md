# 0032 — The IR is structured SSA with π-parameters, and representation is a type

Date: 2026-09-25
Status: Accepted — **specified, not built.** **Realizes [ADR 0006](0006-ir-file-format.md)**, which
decided that the backend interface is a file format and never wrote one. The derivation is
[docs/ir-research.md](../ir-research.md), and the specification is [docs/spec/ir.md](../spec/ir.md).
Decided by three prototypes: [irp1](../../gauntlet/results/irp1-2026-09-25.md),
[irp3](../../gauntlet/results/irp3-2026-09-25.md) and [irp2](../../gauntlet/results/irp2-2026-09-25.md).

## Context

Every backend and every analysis reads the λ-term residual directly. The program's structure is
implicit in those terms, and each consumer rediscovers it. Measured on 2026-09-25 (research §0):
- compiling `freq` spends **46% of its CPU in the garbage collector**;
- **57.7% of its allocation** is the interval pass copying its environment at branches;
- the interval analysis is entered from 19 call sites, and `openFresh` reopens binders at 115;
- 31 side tables are keyed by a term pointer, which a rebuild invalidates;
- the four backends are 9,267 lines, with 21 methods written three or four times and 28 functions that
  each recognise part of the control structure;
- the week's soundness bugs were one species: *a shape a pass did not know*.

Research Theorem A says what the residual already is: a closed, first-order program in A-normal form
with join points. Its algebra is a distributive Freyd category with Elgot iteration. What it lacks is
names. hamza asked for a refactor derived from what the language taught, taking every win in
correctness, simplicity, compile speed and emitted speed, with emitted code no longer required to be
readable.

## Decision

**The compiler has one intermediate representation**, specified in [spec/ir.md](../spec/ir.md), between
the staged residual and everything that reads it:

1. **Structured SSA** (candidate C2).
   - A function is a region.
   - A region is parameters, π-parameters, a sequence of operations, and one of four terminators:
     `yield`, `break`, `continue`, `branch`.
   - `if`, `loop`, `build`, `build-map` and `tabulate` are operations that own regions.
   - Exits are single-level, which is a theorem about the residual and not a restriction (spec §1.3).
2. **Every value is named once**, and lowering is α-renaming with nothing rebuilt (spec §8).
3. **A guard's narrowing is a π-parameter of the arm it guards** (e-SSA/SSI), so each value carries one
   fact and an analysis is sparse (spec §6).
4. **The operation set Σ is closed and finite** (spec §1.2).
   - Integer arithmetic and comparison are operations, classified once, at lowering.
   - Target primitives are `call`s.
   - Lowering refuses what it does not know, so there is no opaque operation.
5. **Representation is a type.** Every value carries its type, and ρ_T maps a type to a host type as
   data (spec §4). Table classes take the narrowest uniform representation (spec Theorem D). A printer
   spells types and recomputes no fact.
6. **Two stages, one format.** Analyses read IR_A, which has obligations (`require`) and ascriptions
   (`the`). Printers read IR_P, where every obligation is discharged, every arithmetic mode is
   decided and every type is final (spec §7). The step from one to the other is the only refusal
   after lowering.
7. **The file is s-expression text** with a canonical printing. Its header carries a version, the
   target and the operations used. A printer covers a program iff it prints every operation in its
   header (ADR 0002, applied to the IR's own operations).
8. **The measured emission decisions are rules on the IR**, each justified by a law of spec §1.4 and
   carrying its evidence (spec §9.4). A rule with a premise states the premise as an obligation.

## Why not

**C0, the residual as the IR** (today). Section 0's measurements are its cost, and each one follows
from the structure being implicit. Its advantage, no second representation, is real, but P1 measured
the price of that second representation at 0.1–0.4 ms and at most 0.7 MB per program (irp1).

**C1, a control-flow graph in SSA form with block parameters.** It represents the same programs,
since the residual's graphs are reducible (Peterson–Kasami–Tokura; Ramsey 2022), and the two
candidates differ in which consumer pays. C1 would pay twice:
- **its printers** would restructure what lowering flattened, with dominators, a reverse postorder and
  Ramsey's algorithm, for JavaScript, Java and Go. P2 printed all seven gauntlet programs from C2 with
  no label and no `goto`;
- **its analyses** would compute a weak topological order (Bourdoncle 1993), which is C2's nesting.

What C1 buys is arbitrary edges: jump threading, tail duplication. The language asks for none of it,
and a structured-control language that did would contradict CLAUDE.md's "never add unstructured
control flow". Lowering to C2 was also cheaper than to C1, by an immaterial ~0.3 ms (irp1).

**C3, continuation-passing style or a higher-order graph** (Appel; Kennedy; Thorin). It reintroduces
the λs that staging removed. Its benefits belong to higher-order languages, and the residual is
first-order.

**C4, sea of nodes.** V8 left it after ten years for a CFG, and reported compile time halved and
simpler code ("Land ahoy", 2025). Its floating nodes also need scheduling, which C2's order already
is.

**C5, an e-graph as the primary IR.** It does not carry effects, control or linearity. It remains a
candidate *pass* inside pure regions, which the Freyd structure identifies (research §8).

**Path-sensitive facts in environments instead of π-parameters.** That is today's interval pass, and
its copying is 57.7% of `freq`'s allocation. Unique names alone move the per-point map from terms to
values and do not remove it. P3 measured the sparse form at 12–115× faster, with 17–96× less
allocation (irp3).

**Arithmetic as `call`s of target primitives**, as the residual spells them. That leaves every
consumer classifying calls by spelling, which is the 19-entry-point problem. The integer operators
are the language's, injected into every target, so they are operations.

**Types inferred by each printer.** P2 needed 275 lines to infer them (irp2 §6), and three more
printers would repeat that. Worse, a printer that infers can disagree with the analysis that proved
the range. The analysis decides, and the file records the decision.

**Facts in the file, beside types.** A printer needs a representation, not the proof of it, and a
third-party printer would otherwise have to read a fact language. Facts stay recomputable from IR_A,
which P3 showed is cheap.

**An explicit world token threaded through impure operations** (as GHC's `State#`). It would make
effect order data dependence like buffer order (spec W7), at the cost of an extra operand and result
on every impure operation. Program order within a region is already a total order on the impure
operations, and the Freyd structure already says which operations float. It is not needed now, and
spec §1.4 would admit it later as a change of presentation.

**A binary format now.** Text uses the lexer that already exists, is diffable, and serves the LLM
tooling ADR 0006 named. No measurement asks for binary.

**`build` as a region rather than an operation** (research §12 left this open). A `build` has an
operand (its length), results, and a scope with one parameter. That is the shape of an operation
owning a region, as `loop` is. A bare region has neither operand nor results.

**Both `if` and `branch`, or only one?** A single form would be smaller. They are, however, the two
readings of the coproduct, sharing a continuation or copying it, and the reducer has already chosen
between them (case-of-case, spec §1.3). Merging them would force a printer to rediscover whether a
join exists, which is one of the 28 recognisers this ADR removes.

## Consequences

- **The migration**, in order, each step gated:
  1. lowering, a verifier (spec §3) and the canonical printer;
  2. the Go printer. Byte-identical emission is re-baselined once, and the gate is the gauntlet
     against hand-written code together with the differential suite;
  3. JavaScript, Java and x86;
  4. the analyses, one domain at a time, with proof counts that never fall.
- **Byte-identical emission stops being a gate once per printer** and resumes on the new baseline.
  That is `cmd/check`'s rule applied to a deliberate change, recorded with `-accept`.
- **A backend is a model of Σ** and can be written outside this repository from spec/ir.md and a
  target's declarations. That is ADR 0006's requirement, met for the first time.
- **Soundness is per (domain, operation)**: a domain's planted-fault table has one row per operation
  of Σ.
- **A rule with a premise states the premise as an obligation.** Writing §9.4 found that today's Go
  bounds-check re-slicing has none. The witness in spec §9.4 panics on a legal program, and the fix
  to the shipped backend is its own change.
- **What stays on terms**: staging, `FlattenProducts`, `PromoteBig` and `SelectWords`. Whether they move
  onto the IR is for later, and not decided here.
- **The language plan waits** (`bufio`, and hamza's Windows application) until the Go printer has
  replaced `golang.go`.
