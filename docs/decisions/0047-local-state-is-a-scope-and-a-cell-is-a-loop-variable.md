# 0047 — Local state is a scope, its initializer decides buffer or cell, and a cell is a loop variable

Date: 2026-10-08
Status: Accepted (hamza: "go with local, go ahead and build it. always mathematical and algebraic.
do your best"). Generalizes ADR 0018's scoped buffer and supersedes the binder spelling of `build`
(tables.md §2.4); the core form `(build n (fn (b) …))` and ADR 0031's product are unchanged.

## Context

Go's standard library writes through pointers: `fmt.Sscan(s, &age, &name)`, `binary.Read(r, o, &x)`,
`errors.As(err, &target)`, Win32's `GetConsoleMode(h, &mode)`. outparams-research.md counted 9 Go
names that need it (cause I) and found the Win32 surface full of it; mutscope-research.md weighed
hamza's proposal, a scope whose binders are mutable and freeze at its exit, against out-parameters
as results in disguise. The two are one construct seen from two sides: a pointer argument is a cell
the callee writes, and a cell is a buffer of length one. The language had buffers (ADR 0018) and no
cell, so a program that wanted a local variable spelled it as a loop variable or a one-element table.

hamza's decisions (mutscope-research.md §9): cells in the scope (1); thaw need not be surface (2);
cells in program order, since threading them buys no speed (3); a host's write to a cell as a buffer's,
a write-borrow or a consume (4); `(out τ)` as concise sugar (5); no nil (6). And: no new
`(buffer n)` form, the initializer's value decides; and the keyword `local`.

## Decision

1. **`(local x₁ e₁ … xₖ eₖ body)` is a scope of local state**: Reynolds' `new x in C` (Idealized
   Algol, 1981), observationally pure by O'Hearn and Tennent's parametricity (1995), the ST monad's
   `runST` (Launchbury and Peyton Jones 1994). Its binders are sequential, as `let`'s.
2. **The initializer's value decides what a binder is, by one rule**: a table, `(table n f)` or
   `(array …)`, makes a **buffer** with those contents, because a buffer's contents are a table;
   anything else makes a **cell**, because Ref τ ≅ Buffer τ 1. Hence:
   - `(table n 0)` is `build`'s zero-filled buffer, read as exactly its core form, so the 137 migrated
     sites emit byte for byte what they did;
   - `(table n v)`, v a literal, is the constant table K v; with f a function, element i is `(f i)`.
     The reading is unambiguous because a table's element is never a function;
   - **thaw is `(table (len xs) xs)`**, because a table is a function. It copies; reuse in place when
     xs is dead is a liveness proof on the IR (Perceus; ADR 0034's ownership), not surface.
3. **A cell is read by name and written by `(set c v)` in program order**, and means the
   state-passing translation S⟦·⟧ (Moggi's state monad compiled away; SSA construction read at the
   source, Kelsey 1995, Appel 1998). The state is the **flat** tuple of every cell in scope, so an
   inner scope's state extends its parent's and a loop carries every cell its body touches as extra
   variables. Arguments are threaded left to right. An elimination's λ (a `case` arm, a tuple
   pattern, a host call's continuation) runs once, now, so the state flows into it. The translation
   runs in the loader after `case` is expanded, and **nothing after it learns that cells exist**: the
   checker, the analyses, the IR and the printers see loop variables. `(set c v)`'s value is the unit.
4. **An elimination is decided on a normal form, by the reducer.** `(op λ)` runs λ once, now, iff
   op's value is data (η for products and sums); a static function could run λ twice. The loader
   cannot know, since `op` is not reduced yet, so the translation marks each elimination it assumes,
   `((#elim op) λ)`, but a `case` arm, and the reducer discharges the mark on op's normal form (no
   tail a λ but a tuple) or refuses the program as a closure over the cells. A mark over a variable
   waits for β; one left at the end is over data and is stripped. This is ADR 0037's pattern, an
   obligation in the residual, and it was found by planting faults: `((twice 0) (fn (i) (set c (+ c
   1))))` counted 1 where the program means 2.
5. **What the translation cannot order is refused**: a λ mentioning a cell that is not an
   elimination's continuation or a loop body (a closure, which could run any number of times or
   later); a binder in the scope reusing a cell's name; `(set c i v)` on a cell; a cell holding a
   function.
6. **A projection of a loop moves into its exits**: `((loop F z…) (fn (x₁ … xₙ) xⱼ))` ⟶ the loop with
   πⱼ at each exit, since a loop's value is its taken exit's. Only a projection moves: its body is a
   variable, so it holds no `again` the loop would capture and no code the loop would repeat. The
   translation's own projection of the state off a loop is one, so a cell's loop yields its value
   alone and reduces to the loop a programmer would write with a loop variable.
7. **A residual `(let v (fn (x) b))` whose v is duplicable is β**, as a source `let` is, and
   `(let v (fn (x) x))` is v by the monad's right unit law; the translation leaves such lets behind.
8. **Normalization's depth is bounded** (`MaxDepth`, 100,000 nested calls), a `FuelError` like fuel's,
   because fuel counts steps, not depth, and a divergent term overflowed Go's stack once the reducer's
   frame grew.

## Why not

- **A `(buffer n)` form beside `table`.** It is a second way to say a table of n zeros. A buffer's
  contents are a table, so the table is the initializer; hamza asked for exactly this.
- **Threading cells like buffers**, `(set c v)` returning the cell. Both compile to the same SSA
  value; threading exists for buffers because the refinement layer's facts induct on a buffer's store
  chain, and a cell has no chain. Program order reads like Go and costs nothing more (hamza, 3).
- **Cells below the loader** — a cell kind in the IR, a pointer in each printer. Every analysis, the
  checker and four printers would learn a construct the translation removes for free. A cell whose
  address nobody takes is a register in Go's own compiler for the same reason.
- **Nested pairs for nested scopes**, ((v, b), a). Products are associative only up to isomorphism,
  and ADR 0031's join points take apart one tuple: the lowering refused the nested value as "2 values
  where one is wanted". The flat state is the canonical representative.
- **Translating in the reducer**, where the static arguments are known. The reducer normalizes a
  λ's body before applying the λ, and its β substitutes a read past a write: `(let a x (seq (set x 5)
  a))` would read 5. The translation needs the term as written; the decision it cannot make there is
  the one it marks (decision 4).
- **Deciding an elimination by syntax**, an application or a let-bound name taken for data. It is
  unsound on the static level: `twice`.
- **Case-of-loop in general**, `(K (loop F z…))` ⟶ the loop with K at its exits. Built and removed:
  an eliminator holding an `again` of an enclosing loop would be captured by the inner one, and it
  rewrote ADR 0031's tuple-pattern join points in existing programs, moving the pattern's body into
  every exit. Decision 5 is the case of it that moves no code.
- **A keyword `build` for the new scope.** `build` names the buffer's construction; a scope with cells
  builds nothing. `local` names what the binders are (hamza).
- **Nil, or a cell with no initial value.** A cell is initialized by its binder, so there is no read
  before a write and no zero to invent (hamza, 6).

## Consequences

- A program has local variables, typed by the join of their writes, at the cost of a loop variable.
- The 137 binder sites of `build` read `(local b (table n 0) …)`; `build`'s binder form is refused
  with a message naming `local`.
- Host writes to a cell (stage B: the Scan family, Win32's out-cells) are a write-borrow or a consume
  at length one, host-buffers.md unchanged; `(out τ)` is a fresh consumed cell (stage C). Both are
  next, and both are translations onto this one.
- `build-map` keeps its binder form until a map value carries its capacity.
- Closures over cells are callbacks tier 1 (callbacks.md), refused until built.
