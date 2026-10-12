# 0055 — A loop variable is bound by application, not assigned

Date: 2026-10-12
Status: Accepted (hamza, 2026-10-12: "why are we using := in loops … shouldn't we use = in j, and ==
in D instead. because the programmer can't assign something to j, he can only pass the argument to
again"; then "yes, record it"). Amends [spec/abstract.md](../spec/abstract.md) §2.2's binder kinds and
the loop operator's notation. Not built: notation only, built with the surface; nothing below the reader
changes.

## Context

abstract.md §2.2 listed a loop variable under the state binders, `x := e`, "x starts at e, changes on
a path (a loop variable, a cell)". notation-research §4.4 wrote `LOOP i := 0`, following PlusCal and
Dijkstra's guarded commands, where loop variables are program variables that the body assigns by a
concurrent assignment. surface-design's installments 2 and 3 copied it.

But in this language the body cannot assign a loop variable:
- W5 admits assign(x; e) only for a state binder of an enclosing scope;
- `again(ā)` is the only way a loop variable takes a new value;
- today a write to one is refused, though late and with the wrong cause: `(set i (+ i 2))` in a loop
  reaches `gen`, which says "set stores into i, which is a frozen table, not a buffer".

## Decision

**A loop's variables are definitional binders**, bound at entry by their initializers and at each
`again` by its arguments, by application. They are written as `let`'s binders are:
`loop(x̄ = z̄; I; ā)` in the abstract syntax, `LOOP i == 0 :` in D and K, `loop i = 0 { … }` in J,
`(loop ((i 0)) …)` in the prefix renderings.

**The derivation.**
1. D maps a loop to `(loop (fn (x̄) arms) z̄)`, and `again(ā)` applies that λ. A loop variable is a
   parameter of the loop's λ.
2. The unfolding law (abstract.md §11) states it without D:
   ```
   loop(x̄ = z̄; I; ā)  =  let(x̄ = z̄; ā[again(w̄) ↦ loop(x̄ = w̄; I; ā)])
   ```
   One iteration is a `let`, so the loop's header is a `let`'s binder group. Within an iteration a
   loop variable names one value; a new iteration binds it afresh. That is SSA's φ, a block argument
   (Appel, "SSA is functional programming", 1998).
3. A cell is the other kind: assign(x; e) changes x within one region, in program order. Spelling both
   `:=` was a homonym (fit-research's F2) for two kinds W5 already separates.

## Why not

- **Keep `:=`** (PlusCal, the guarded commands). It marks as writable a binder the grammar refuses to
  write. Their loops assign; ours apply.
- **A third operator for a first value** (`←`, Sisal's `initial`, Lucid's `fby`): a fourth binder
  shape, where the unfolding law shows the loop's binder is `let`'s.
- **Named `again` as a concurrent assignment** (Dijkstra's `x, y := e, f`). That reading goes with
  loop variables as state. With loop variables as parameters, `again` is an application and its
  arguments are positional, which surface-design §3.4's Finding 3 recommends.

## Consequences

- **`:=` means one thing**: a place taking a value. That is a scope's state binder (a cell, or a
  buffer by ADR 0052) and a write (`x := e`, `b[i] := v`).
- **In an arm**, `j = j + 1` is shadowing (ADR 0054 allows it for a definitional name) and `again(j)`
  passes the new value; `j := e` is a write to a name that is not a place, refused by W5 at the reader,
  with that cause, when the surface is built.
- **Precedent**: Clojure's `loop` "is exactly like let, except that it establishes a recursion point";
  Scheme's named let binds with `let`'s syntax.
- **The cell translation runs from `:=` to `=`** (local.md: a cell becomes a loop variable). State is
  implemented by parameters, never the other way, which confirms which kind is primitive.
- **abstract.md**: §2.2's kinds, the loop row, D's loop rules, 𝒪's loop rule, §9.4's rendering, μ's
  table, the unfolding law, the cell law and the worked example write the loop's binders with `=`.
  surface-design's examples and notation-research §4.4 follow.
