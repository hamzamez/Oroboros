# 0038 — The IR's text is the printers' whole input, and others may read it but not yet write it

Date: 2026-10-04
Status: Accepted. Built in irtext-2026-10-04. Realizes ADR 0006's promise for readers; defers it for
writers until the IR proves every claim a printer trusts.

## Context

ADR 0006 decided that the interface between the compiler and a backend is a file format, so that a
backend can be written in any language by a third party. ADR 0032 wrote the format (spec/ir.md §2.1,
§10), with a canonical printer and reader, and `cmd/check` pinned read ∘ print = id on every emitted
program. Two questions stayed open, and the 10-03 assessment put them first in its round of questions:

1. **Is the text the printers' whole input?** The four printers took the decided function in memory.
   It carries what the text does not: the interval facts, the range marks, provenance. A printer that
   read any of it would make the file a lossy picture of the interface, not the interface.
2. **May anyone else write a file?** A format others write is an input, so the trust boundary moves
   to the reader.

Let IRₜ be the set of decided programs for a target t, Σ* the texts, p : IRₜ → Σ* the canonical
printer and r : Σ* ⇀ IRₜ the reader, with r ∘ p = id (p is a section of r). A printer is
e : IRₜ → Host. The text is the whole interface exactly when e factors through p:

  e = e′ ∘ p for some e′ : Σ* → Host, which holds iff ker p ⊆ ker e,

that is, iff two programs with one text always print to one file. Then e′ = e ∘ r on p's image. This
is a property of e, which only a measurement over e's inputs can establish.

## Decision

**1. The published file is IR_P, and every printer factors through it.**
- `gen -ir` and `build -ir` write the IR_P the printer read, not the decided IR_A before it. IR_P is
  what spec §9.1 says a printer reads.
- `cmd/check`'s `ir` step checks e_T = wrap_T ∘ e_T ∘ r on every emitted program, byte for byte, with
  the target's declarations and nothing else. All 253 emitted programs pass (the 117 with a function
  print one), on all four printers.
- What a printer reads from the target is declarations only (`Prims`, `HostType`, `ValueType`,
  `BoxedType`, `ReprBytes`, `IsLengthName`, `FindAlloc`, `Name`). So a printer's input is the pair:
  the file, and T's declarations, which are the backend's own data (ADR 0006, "Consequences").

**2. Others may read it.** That covers a printer outside this repository, a diff, a review tool and
an LLM. The grammar is spec §2.1, in the language's lexical syntax. The version and covering are
§10.2: `(ir 1 …)`, which the reader refuses to misread, and `(ops …)`, which W10 checks.

**3. Nobody else writes it yet.** No tool takes an IR file as input. A written file becomes
admissible when both of these hold:
- **it enters at stage A, never at P.** A P file is an A file plus the decision's claims: every
  arithmetic operation's mode, every type's final range, and every index in range (§9.1). Every
  printer trusts them, and the verifier (W1–W10) checks none, by design. Measured: a P file claiming
  `add exact` on two unbounded parameters, one claiming `(int 0 10)` for a sum up to 2⁵⁴, and one
  indexing a table at 5 with no bound were each printed, with no refusal.
- **the IR proves every obligation a printer assumes.** An A file re-enters the decision, which
  re-derives the arithmetic claims: the same `add`, written at A, was refused. But **index
  obligations are proven only on terms**, by the refinement layer, before lowering. The same
  unbounded index, written at A, was printed. A writer would bypass the only prover of the property
  JavaScript and x86 do not check at run time (noprop-2026-09-25).

The second condition is the fifth question of the same round, "one prover or two?". This ADR does not
answer that question. It makes writing the format wait on the answer.

## Why not

- **Trust the writer's claims, as LLVM does.** LLVM's `nsw`, `nuw` and `inbounds` are a producer's
  promises. Breaking one makes the result poison, and its use undefined behaviour (Lee et al.,
  "Taming undefined behavior in LLVM", PLDI 2017). That choice makes the format a channel for
  undefined behaviour. Here every claim a printer relies on is a theorem or a refusal (ADR 0019, ADR
  0026, refinements.md §3a), and "not refused" is not "proven" (noprop-2026-09-25).
- **Accept P and check its claims: proof-carrying code.** Necula's PCC (POPL 1997) pays because
  checking a proof is cheaper than finding one. Abstraction-carrying code (Albert, Puebla and
  Hermenegildo, LPAR 2004) applies the same idea to an abstract interpretation's fixpoint, as does
  the JVM's StackMapTable (the split verifier, JSR 202). The certificate there is the fixpoint, and
  checking it is one pass with no widening. That is the right design when the certificate is
  complete and deriving it is expensive. Neither holds here:
  - the types in P are not a complete certificate. The facts that justified them are not in the file
    (π facts, the relational difference, a map's cells; spec §12);
  - deriving them is cheap (irp3: 12–115× faster than the term analysis it replaced).

  A file without facts is checked by deriving them, and then P carries nothing A does not.
- **Admit A writers now, and let the host catch an index.** Go and the JVM check an index at run
  time; JavaScript returns `undefined` and x86 reads whatever is there. A format admitted on two
  hosts' run-time checks would be ADR 0001's portability claim made by assumption.
- **Keep the decided IR_A as the published file.** A printer reading A would have to take the step to
  IR_P itself, and that step is an analysis: `Finalize` reruns the interval domain and rewrites by L13
  (§7, §9.4). A third-party printer would then reimplement the analysis, the part that is hardest to
  check, and §9.1's "a printer never recomputes a fact" would be false for every printer but ours.
- **Make the file self-contained now, carrying the host forms of the primitives it calls.** Measured:
  every emitted program prints the same with only `targets/` and `lib/` as its declarations, so no
  swept program needs anything its own directory declares. A program with its own `provides`
  fragment (`examples/tally`) would. Carrying those fragments in the file is specified as the next
  step and waits for the first reader outside the repository.
- **A binary form.** Nothing measured asks for one. The text is 6.4× the host code it prints to (§3 of
  the result), which matters to nobody yet.

## Consequences

- ADR 0006 holds for readers, checked on every program: the emission baseline is now also the text's
  baseline, through the factoring law. A printer that reads anything the text does not carry fails
  `cmd/check`. Three planted faults show it does: a sink not reset between programs, functions
  printed in another order than the text's, and a printer reading provenance.
- **An emitted file is a function of its IR's text, and the text is a set of functions.** `gen` prints
  a program's functions in name order, the text's order, because x86 numbers its labels and literals
  across the file.
- Writing the format is an obligation on the IR, not on the format. When the IR proves index
  obligations, an A reader is the decision itself: `Read`, `Verify`, `Decide`, then the steps
  `ir.Entry` takes after lowering. When that lands, this ADR's third decision is revisited.
- The reader stays a grammar check (spec §2.1). Whether a program is well formed is the verifier's
  question, and whether it is legal is the decision's.
