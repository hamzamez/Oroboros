# 0054 — `let` and `local` are one scope, whose binders state their kind

Date: 2026-10-11
Status: Accepted (hamza, 2026-10-11: "I will take 1, 2 and 3"), on surface-design.md §2.13. **Not
built**: it is built with the surface, behind the byte-identical gate. Amends
[ADR 0047](0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md)'s head, after
[ADR 0052](0052-a-buffer-is-a-state-binder-holding-a-table.md) amended its initializer rule, and
[spec/abstract.md](../spec/abstract.md)'s `let` and `local` operators, W2 and W5.

## Context

Abstract Oroboros has two scope operators of one arity: let(b̄; e) over definitional binders and
local(b̄; e) over state binders. surface-design §2.3 found them mergeable (Finding 1) but could not
merge them while the head was what said a binder was a buffer. ADR 0052 made a buffer a state
binder, so the binder's operator now says the kind: `t == table(n, 0)` a value, `b := table(n, 0)` a
buffer, `x := 0` a cell.

Today the two heads also disagree on a repeated name: `(let x 1 x (+ x n) x)` reduces to `(+ 1 n)`,
the second binder shadowing the first, and `(local x 0 y 1 x 5 x)` is refused. abstract.md's W2 says
every group's names are distinct, which is false for `let`.

Measured (`go run gauntlet/surface/scopes.go`): 127 `let` and 66 `local` forms; 7 chains alternate
the two heads, 3 `let` around `local` and 4 `local` around `let`; no group binds a name twice.

## Decision

1. **One scope operator**, scope(b̄; e), whose binders are definitional or state, each stating its
   kind by its operator. A scope is the action of the free monoid of binder words on expressions:
   scope(ε; e) = e, and scope(β̄; scope(γ̄; e)) = scope(β̄γ̄; e). `let` and `local` were its
   restrictions to Def* and State*.
2. **Repeated names, by kind.** A definitional binder may rebind a name; by the action's law that is
   a nested scope, so the later binder shadows (sound by ADR 0036's hygiene). A state binder's name is
   bound once in its region: bound again by a definitional binder it would hide the cell (refused
   today, local.md §3), and bound again by a state binder it could not be told from a write.
3. **The head is written `let`**, where a surface has a head (D's `LET`, B′'s and L's `let`). K and J
   write none.

## Why not

- **Two heads** (surface-design §2.3, option (b)). The head is a function of its binder word: kind is
  the binder's operator, a state binder's region is its lexical scope (§2.13's lemma: the merge joins
  a scope only with its body, so no region moves), encapsulation is per scope, and D is a
  homomorphism of the action. Two operators for one action is a synonym (fit-research's F1), and it
  costs a nesting level per alternation: 7 in the corpus.
- **All names distinct in every group**, abstract.md's W2 as written. It would refuse today's
  shadowing `let`, which the action's law makes sound.
- **Repeated names allowed for state binders too.** In a group, a second `x := e` cannot be told from
  a write of x, and as a second place under one name it hides the first.
- **`local` as the head.** It named "state lives here", which the binders now say; and D and L
  already write `LET` and `let`.

## Consequences

- **The reader**: μ's `muLet` and `muLocal` become one. D is unchanged, a right fold binder by
  binder, so the 7 alternating chains are the byte-identical test.
- **abstract.md**: one scope operator in place of `let` and `local`; W2 by kind as decision 2; W5's
  "a state binder of an enclosing `local`" becomes "of an enclosing scope".
- **Unchanged**: the freeze (ADR 0031, R4), the cell translation, R0–R4, relevance, the proofs, the
  IR and the printers. None of them sees a head.
- **Not merged**: a loop's variables. loop(s̄; I; ā) is a scope iterated by `again`, with arms; it is
  installment 3's operator.
- **Open**: in a statement-list surface (K, J) a state binder's introduction and a write are both
  `x := e`. Whether to mark the introduction (Algol 68, Rust), keep binders in groups (D, L), or
  resolve by context (Python) is decided with the statement list, in installment 3.
