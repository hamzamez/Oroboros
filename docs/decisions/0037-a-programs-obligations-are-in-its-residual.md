# 0037 — A program's obligations are in its residual, and nothing reads one undecided

Date: 2026-10-02
Status: Accepted. Built in hazard-2026-10-02. Amends ADR 0028's mechanism, not its rule.

## Context

ADR 0028 made a declared parameter range and a `where` obligations at every call. An obligation is
the domain condition of an application: for f : A → B, the term (f a) denotes only if ⟦a⟧ ∈ A. So the
set O(P, T) of a program's obligations on a target is a function of P and T.

The mechanism made it a function of a third thing. Reduction inlines every call, so the reducer marks
each obligation in the term, `(#req …)` and `(#reqw …)`, and a decider proves each mark or refuses
the program. But:

- the reducer marked only when a driver had called `InstallRequires` on the environment;
- a literal argument was decided as it reduced and its mark dropped either way, a failure going to a
  callback that filled a set only that driver held;
- every consumer after reduction (the type checker, the refinement layer, lowering, the printers) is
  transparent to a mark, by design, so one that reached them undecided was read as its value.

`build`, `gen` and `intervals` each performed the ritual. Anything else that took a program to
emitted text had O = ∅: `ir.Entry` called directly, a backend's `FromResidual`, any driver written
later. On the commit before this one, `(digit 12)` against `(sig digit ((x (int 0 9))) …)`,
`(bits.Len32 -1)` and a `where` false at its call each compiled through both.

It was met from the other side in bufio-2026-10-01: moving a host function's interval `where` onto
the range route made thirteen tests accept programs they must refuse, because their pipelines placed
no marks. That change was reverted and this hazard named.

## Decision

**O(P, T) lives in the residual of P on T, and nowhere else.**

1. `Target.Env`, the only way an environment for a target is made, installs every contract: the host
   functions' parameter ranges and the program's ranges and `where`s. Reduction under a target marks
   every obligation.
2. A literal inside its range is the obligation discharged by evaluation, and leaves nothing. A
   literal outside it keeps its mark. A `where` that reduces to false keeps a mark whose condition is
   `(#false clause)`, which nothing proves and which carries the clause for the refusal. There is no
   callback and no side set.
3. Deciding is one procedure, `ir.DecideMarks`: evaluation, the IR's intervals, the refinement layer.
   `ir.Entry` and every backend's `FromResidual` call it.
4. A consumer that is transparent to a mark **requires** the marks decided, and refuses otherwise:
   `emit.Refine` at its entrance, and `ir.ToP`, through which all four printers come.

## Why not

- **Make every driver perform the ritual, and say so.** That is what there was. It is a convention,
  and the next pipeline is written by someone who has not read it. The test helpers were such
  pipelines, and they are where most soundness tests run.
- **Keep the callback, installed by `Target.Env`.** Then a failed literal is in a set the environment
  owns and each consumer must remember to read. The term is what every consumer already has.
- **Have the refinement layer prove a mark it meets**, in its own fragment. It was built, and
  withdrawn. It is a second decision procedure, weaker than the first, whose refusals differ from
  the drivers': a range above the signed word is not statable in the linear fragment, so a program
  the pipeline accepts was refused by a refinement-only caller. One procedure, and a precondition on
  the layers that cannot run it, is the smaller claim.
- **Have lowering refuse a mark.** The interval route decides a range mark by lowering the marked
  residual and reading its values' facts, so lowering must stay transparent. The step after it that
  every printer shares, `ToP`, is where "decided" can be required.
- **Erase the marks where a pass does not want them.** That is the hazard, done on purpose. Two test
  helpers of the refinement layer alone do erase them, visibly and with the reason written at the
  call, because their package cannot import the pipeline that decides.

## Consequences

- A pipeline that forgets to decide is refused by the first consumer it reaches, with a message
  naming `ir.DecideMarks`. It cannot emit.
- `cmd/oro` prints a normal form with its marks: the residual is where the obligations are.
- The drivers still decide for themselves, before `ir.Entry`, because `gen -report-requires` reports
  on the marks. After them `DecideMarks` finds none.
- With every pipeline marking, a host function's interval `where` can take the range route to the
  IR's intervals (bufio-2026-10-01 §6). It is not built: no program refuses for want of it.
- `core.Env.OnRequire` and the literal set are gone; `core.Env.InRange` is the literal's decision.
