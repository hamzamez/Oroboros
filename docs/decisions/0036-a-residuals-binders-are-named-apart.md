# 0036 — A residual's binders are named apart along the scope chain

Date: 2026-10-01
Status: Accepted. Built in bufio-2026-10-01.

## Context

A term is locally nameless: an abstraction is a list of parameter names over a body with indices, and
the names are hints. Reduction never opens a body. Every consumer of a residual does, with `Body()`,
which turns each index into its binder's hint.

`core/hygiene.go` already kept one invariant on what reduction returns: no binder's hint equals a
name its body refers to past it. That keeps a **term** from capturing when it is opened, and it left
a binder alone when its body never mentioned the outer variable of the same name. A test pinned that
as *"harmless shadowing"*.

It is not harmless to a consumer that carries a **context keyed by name**. The refinement layer is
one: its facts are assumptions about names, Γ ⊢ φ, and when its walk enters a binder that reuses a
name in Γ, the assumptions about the outer variable are read as assumptions about the inner one. With
a parameter `n` declared in [0, 5] and a loop variable `n` that runs to 101, the loop's exit value
was given 0 ≤ n ≤ 5, and an index of 101 into a table of 6 was accepted on Go, JavaScript, Java and
x86 (bufio-2026-10-01 §4).

The same leak was carrying true conclusions. `(loop ((c c) …))` is the idiom for filling a buffer
(ADR 0018), and `len c = n`, a fact about the buffer, was reaching the loop variable through the
shared name. The corpus's buffer proofs rested on it.

## Decision

**No binder's hint equals an enclosing binder's**, whether or not its body refers to that variable.
`hygienic` renames such a binder, as it renames one that would capture. This is the variable
convention (Barendregt 1984, 2.1.13) along the scope chain, and it is an invariant of the residual,
so every consumer meets it.

Sibling binders, which no scope holds together, keep their names. A program with no shadowing keeps
every name it had.

What the leak had carried is established instead: a loop variable that threads a buffer has the
buffer's length by induction over the back edges (refinements.md §5b).

## Why not

- **Rename inside the refinement layer only.** Its entrances are several (the check, the contract
  marks' dry walk, the probes), and other consumers carry contexts keyed by name too: the product
  pass kept arities by name and followed a buffer into a loop only when the variable was spelled like
  it. An invariant stated where every consumer meets it was the reason hygiene is on the residual in
  the first place, and the fifth instance of this family is recorded there.
- **Kill the facts about a name when the walk enters a binder of that name.** It is the textbook rule
  for a context, Γ, x:σ with x fresh, done by removal. Every place that extends the context would
  have to do it: the walk, each probe's copy of the facts, each memo keyed by a printed term. One
  place that forgot would be this bug again, silently.
- **α-rename the context instead of the binder.** The context's keys are printed terms, so a rename
  rewrites every fact that mentions the name, at every binder.
- **Make every binder globally unique.** It gives the same guarantee and renames far more: two loops
  one after the other could not both use `i`, and every refusal message would print `i2`, `i3`. The
  scope chain is exactly as far as a context reaches.
- **Leave it, since the conclusions were true in the corpus.** They were true and unproven, and in
  the probe they were false.

## Consequences

- A refusal message may print a binder as `n1` where the source wrote `n`. One message in the corpus
  changed. No emitted file did: the IR numbers its values.
- Nested discards are `_`, `_1`, …: a postcondition attaches to its binder's name, and two `_`s
  would have pooled their facts.
- A consumer may assume that along any path from the root, binder names are distinct. It may not
  assume it across siblings: a quantity named after a binder must not outlive the binder's scope
  (`valueLength` binds a name it cannot measure to *unknown* for this reason).
- A pass that relied on a loop variable sharing its buffer's name is now simply wrong, visibly. Two
  were: the refiner's length of a threaded buffer and the product pass's arity.
