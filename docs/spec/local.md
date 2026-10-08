# `local`: a scope of local state

Status: built 2026-10-08 (local-2026-10-08, ADR 0047). Supersedes the binder form of `build`
(tables.md §2.4), which it generalizes; the core form `(build n (fn (b) …))` is unchanged.

## 1. What it is

```lisp
(local buf (table n 0)            ; a buffer of n zeros
       sq  (table n (fn (i) (* i i)))   ; a buffer initialized by a rule
       lit (array 9 0 7 8 9)      ; a buffer initialized by a graph
       cp  (table (len xs) xs)    ; a buffer initialized from a table: thaw
       sum 0                      ; a cell
  body)
```

A `local` is **local state encapsulated by a scope**: its binders may be updated destructively inside
it, and nothing outside can observe them, so the scope is a function of its inputs. This is the
locality of Algol-like languages (Reynolds' Idealized Algol, `new x in C`, 1981; proved observationally
pure by O'Hearn and Tennent's parametricity, 1995) and the ST monad's `runST` (Launchbury and Peyton
Jones, 1994). The language's earlier `build` was its restriction to buffers (ADR 0018, ADR 0031).

**The initializer's value decides what a binder is**, by one rule:

| initializer | binder | because |
|---|---|---|
| a table, written `(table n f)` or `(array …)` | a **buffer** with those contents | a buffer's contents are a table, a function on [0, n) |
| anything else | a **cell** holding that value | a cell is a buffer of length one: Ref τ ≅ Buffer τ 1 |

So there is no separate form for a buffer or a thaw:
- **a table of a zero is the zero-filled buffer** `build` made (tables.md §14.3), read as exactly its
  core form, with no fill loop, since a fresh buffer already holds the zero. `(table n 0)` names the
  zero of every numeric and boolean element, as `build`'s buffers always took it, and leaves the
  element to the stores. `(table n false)` and `(table n 0.0)` name the zero of one sort, and that
  claim stays checked: the reader ascribes the buffer, `(the "buffer bool" b)`, the type checker reads
  the ascription, and every pass after it reads the residual without it
  (`emit.KeepSortAscriptions`). `-0.0` is not a fresh buffer's zero, so it fills;
- `(table n f)` with f a function fills element i with `(f i)`;
- `(table n v)` with v a **literal** is the constant table: a constant is the constant function K v, and
  the reading is unambiguous because a table's elements are never functions (closures do not survive
  staging);
- `(table (len xs) xs)` initializes from a table one has, because **a table is a function**
  (tables.md §2): this is Haskell's `thaw`, and it copies. Copying in place when xs is dead after the
  scope begins is a liveness proof on the IR (Perceus' reuse; ADR 0034's ownership and liveness), not a
  surface form. Not built yet;
- `(array e₀ … eₖ₋₁)` initializes from a graph.

The binders are sequential, as `let`'s are: an initializer sees the binders before it.

## 2. Buffers

A buffer binder is the buffer `build` always made, linear (ADR 0018) and threaded: `(set b i v)` hands
the buffer back, a loop carries it, and a host write-borrow returns it. The scope's value is the body's,
and a buffer in it is frozen (ADR 0031). Nothing about buffers changes.

## 3. Cells: local variables in program order

A cell is read by its name and written by `(set c v)`, in **program order**: a read sees the last
write before it on its path.

```lisp
(local total 0
  (loop ((i 0)) (>= i (len xs)) total
    else (seq (set total (+ total (xs i))) (again (+ i 1)))))
```

**What it means: the state-passing translation.** A program with local state is translated into the
pure core by threading each cell's value through it, which is Moggi's state monad compiled away and,
read operationally, SSA construction at the source (Cytron et al. 1991; Kelsey 1995; Appel 1998, *SSA
is functional programming*). Written S⟦t⟧ for the term whose value is the pair (t's value, the cell's
value after t), with x the cell's current value:

```
S⟦x⟧                    = (x, x)
S⟦(set x e)⟧            = let (v, x) = S⟦e⟧ in ((), v)
S⟦c⟧                    = (c, x)                         c mentions no cell
S⟦(if c a b)⟧           = let (c', x) = S⟦c⟧ in if c' S⟦a⟧ S⟦b⟧
S⟦(f a₁ … aₙ)⟧          = let (a₁', x) = S⟦a₁⟧ … (aₙ', x) = S⟦aₙ⟧ in ((f a₁' … aₙ'), x)
S⟦((fn (y…) b) a…)⟧     = let (a', x) = S⟦a⟧ … in S⟦b[y := a']⟧               a let, a seq
S⟦(h (fn (y…) b))⟧      = let (h', x) = S⟦h⟧ in (h' (fn (y…) S⟦b⟧))           an elimination
S⟦(loop (fn (v…) c) z…)⟧ = (loop (fn (v… x) S'⟦c⟧) z'… x)                    the cell a loop variable
S'⟦(again a…)⟧          = (again a'… x)
```

and the scope is `(local x e body)` ⟶ `let x = e in fst S⟦body⟧`. Arguments are threaded **left to
right**, then the call, so that order defines which write a read sees within one expression. An
elimination's λ (a `case` arm, a tuple pattern, a host call's continuation) runs once, now, so the
state flows into it like a `let`.

**What is an elimination is decided on a normal form.** `(op λ)` runs λ once, now, exactly when op's
value is data, a product or a sum, since then the application is η's elimination; the static level is
higher-order, and a static function could run λ twice, or never:

```lisp
(def twice (n) (fn (body) (seq (body n) (body n))))
(local c 0 (seq ((twice 0) (fn (i) (set c (+ c 1)))) c))     ; means 2
```

Translated as an elimination, this counted 1: the translated λ is pure, so the reducer dropped the
first call. The translation runs in the loader, where `(twice 0)` is not yet reduced, so it **marks
the claim**, `((#elim op) λ)`, for every elimination but a `case` arm (whose scrutinee is a
constructor's tuple by construction), and the reducer decides it where it meets it (ADR 0037's
pattern: an obligation in the residual): op's normal form must have no tail that is a λ other than a
tuple. A mark over a variable waits, since β renormalizes a body once its variable has a value; one
left at the end is over a parameter or an impure `let`'s value, which a first-order residual holds
only as data, and `Normalize` strips it. Otherwise the program is refused, as a closure over the
scope's cells (`TestAnEliminationIsDecidedOnTheNormalForm`). The translation is a function of the term, the result is ordinary
`let`s, `if`s and loop variables, and the reducer's η and β take the pairs apart. **Nothing below the
loader learns that cells exist**: the checker, the analyses, the IR and every printer see a loop
variable. A cell costs exactly what a loop variable costs.

**Why program order and not threading, for cells.** Both give the same code: a cell whose address
nobody takes becomes an SSA value in a register either way, which is the point of the translation.
Threading is what buffers need, because the refinement layer's facts induct on a buffer's store chain.
A cell has no chain to induct on, so it reads like Go.

**Typing.** A cell's type is the join of everything written to it, as a buffer's open element is
solved (types.md §3.1): the cell is a loop variable after translation, typed by the same rules. `sum 0`
needs no annotation; `3.0` is an `f64`.

**The value of `(set c v)`** is the unit, `(tuple)`: a write is done for its effect.

**What is refused**, each with the reason:
- a λ that mentions a cell anywhere but an elimination's continuation or a loop, and a λ passed to a
  term whose normal form is a function: a closure that could run any number of times, or later.
  Callbacks over a scope's cells are tier 1 (callbacks.md), not built;
- a binder inside the scope with a cell's name: `(let total 1 …)` would hide the cell, so one must be
  renamed;
- `(set c v)` where c is not a cell in scope, and a cell used after its scope.

## 4. The scope's value

The body's value, frozen: a product of frozen buffers, cells' values and other values (ADR 0031). A
cell's final value leaves the scope only if the body returns it: `(local age 0 name "" … (tuple age
name))`.

**The translation's last step is a projection**, `fst`, of the state tuple off the body. When the
body is a loop, its exits are tuples (v, x₁ … xₖ), and the reducer moves the projection into them:

```
((loop F z…) (fn (x₀ … xₖ) xⱼ))  ⟶  the loop with πⱼ at each exit
```

because a loop's value is its taken exit's. Only a projection moves: its body is a variable, so it
holds no `again` the loop would capture and no code the loop would repeat. Any other eliminator of a
loop stays outside it, ADR 0031's join point. With it, a cell's loop yields its value alone, and a
program written with a cell reduces to the program written with a loop variable
(`TestACellReducesToTheLoopVariableItMeans`, α-equivalence where the surface can spell the second).

## 5. Not here

- **Host calls writing a cell** (Go's `&x`, Win32's out-cells): a write-borrow or consume of a cell,
  host-buffers.md at length one, and `(out τ)` as a fresh consumed cell (mutscope-research.md §7). The
  next steps.
- **`build-map`**: a map buffer keeps its own form until a map value carries its capacity.
- **Thaw in place**, §1.
- **A declared range on a cell**, `(c (int 0 200) 0)`.
