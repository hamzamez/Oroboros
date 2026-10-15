# `local`: a scope of local state

Status: built 2026-10-08 (local-2026-10-08, ADR 0047); a host writing a cell, and `(out τ)`, §5–6,
2026-10-09 (outcells-2026-10-09, ADR 0048). Supersedes the binder form of `build`
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
  element to the stores, or to a read's demand. When nothing fixes it, no element is observed at any
  sort, and the element is the integer (spec/ir.md §2.2). `(table n false)` and `(table n 0.0)` name the zero of one sort, and that
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

In today's notation a buffer binder is the buffer `build` always made, linear (ADR 0018) and threaded:
`(set b i v)` hands the buffer back, a loop carries it, and a host write-borrow returns it. The scope's
value is the body's, and a buffer in it is frozen (ADR 0031).

### 2.1 In program order (ADR 0052)

Status: built 2026-10-15 behind `ORO_VIA_J` (storeorder-2026-10-15): J writes every buffer this way, and
S of the corpus in J is today's residual but for two programs, each explained there. Today's notation
keeps threading until the corpus moves to J. G5–G7 (spec/j.md) are decided (hamza, 2026-10-15).

**A buffer is a state binder whose value is a table**, Ref(Seq(A)) with point update: ADR 0047's "a
cell is a buffer of one" read in reverse.

```
var c := array { false : _ ∈ 0..n-1 }       // a buffer: a state binder, a fresh table
loop j = i * i {
    j < n -> { c[j] := true; again(j + i) }  // an element write; the loop does not carry c
    else  -> c                               // the scope's value: frozen
}
```

**What it means: S, extended** (§3's translation, storeorder-research §2.3). Each new form is a derived
operator whose equation is its D image, so S needs one new clause, the borrow:

| form | equation (D) | |
|---|---|---|
| `b[i] := e` | `b := set(b, i, e)` | McCarthy's store on the current version |
| `b[i][j] := e` | `b := set(b[i], j, e)` | a table of tuples is [0,n) → (Fin k → V); the curried store `(set (b i) j e)` is the point update of its uncurried form at (i, j) (products.md) |
| `x₁, …, xₖ := e₁, …, eₖ` | `let t₁ = e₁ … tₖ = eₖ in x₁ := t₁; …; xₖ := tₖ` | Dijkstra's concurrent assignment (1975): every right side in the state before |
| `var b := t`, t a fresh table | `build` as §1, its body a state scope on b | the region is `build`'s, the binder state |
| `fn f(…, p linear, …) { e }` | λ(…, p, …). a state scope on p, whose value is (p, e) | a borrowed parameter is a place in the body |

**The borrow** (ADR 0052 item 3). A call passing a buffer in scope at a `linear` parameter, of a
definition or of a host declaration with a `(buffer V)` parameter, hands it to the callee for the call's
duration. The callee's value is **the borrowed buffers first, then its result's components, a unit
result dropped**: B₁ × … × Bₘ × R, with B × 1 ≅ B, the monoidal unit law. Then

```
S⟦f(…, b, …)⟧ = let (b', r̄) = f(…, b, …) in (r̄, σ[b ↦ b'])
```

and the call's value is r̄, the unit when the result is. The target hands the loader one
`(borrows RESULTS i…)` form per declaration that borrows, its `(buffer V)` parameters' indices and its
result's other components, as it hands it `cells` forms. The host declarations already state the
convention (`(tuple (buffer V) R…)` for a write-borrow, `(buffer V)` for growth), and the corpus's 23
definitions that thread a buffer all return it alone. A definition's body is a unit when every tail is
`()`, a write, or a borrow whose result is the unit; its value is then its borrowed buffers alone.
Passed by reference, as ADR 0052 decides: after inlining, the callee's place is the caller's.

**Loops carry what they write.** A loop carries the state binders it writes (a write, an element
write, a borrow, a host writing a cell, or a nested scope or loop that does), **first, in binding
order**; a state binder it only reads is free in it. Both are isomorphisms: a variable that every back
edge passes unchanged is invariant, and a loop's state is a product, whose order is its symmetry. The
order is chosen by measurement (gauntlet/surface/stores.go): 49 of the corpus's 53 loops that thread a
buffer by hand put it first, in binding order, so S of their program-order form is their header. This
also holds of cells: §3's translation carried a cell the loop only read, appended.

**The rules** (ADR 0052, R0–R4), checked by S on each place's scope:

| | rule | an occurrence of a buffer b is legal only |
|---|---|---|
| R0 | a buffer's value is fresh | as a binder's fresh table, or moved by a concurrent assignment that also writes b (the moves a permutation of places, so the map from tables to places stays injective) |
| R1 | a buffer is named only by its binder | as an index's head `b[i]`, a write's place, or a call's argument |
| R2 | a call borrows a buffer for its duration | once per call |
| R3 | no closure over a buffer | outside a λ, as §3 has it for cells |
| R4 | the scope's value freezes the buffer | at the scope's tail, alone or in a tuple; a borrowed place never, since it is the caller's |

`again(b)`, `d = b` and `(b, n)` inside the scope are R1's refusals; the jump carries b itself.

**What S produces** is today's threaded residual: the store chain, the loop variable and the φ at a
join that today's programs write by hand. The prediction of storeorder-research §2.6, checked by the
gate, held once S learned what storeorder-2026-10-15 §2 records:
- **a statement `if` that writes one state binder is a φ on it**, x := if c then π_x(S⟦a⟧) else
  π_x(S⟦b⟧), so the rest of the block is not copied into both branches;
- **an argument that writes no state is evaluated where it stands**, when no argument after it writes;
- **a place is threaded under its version name** (`#v` and its own), and β puts a version used once,
  first, where it is used: let x = e in E[x] = E[e] for E evaluating its hole first and
  unconditionally with nothing impure before it, an operator no hole, a loop's initializer one only for
  a value holding no loop (core/reduce.go, firstUse);
- **a selection moves into a loop's exits and a region's body**, as a projection did.

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

## 5. A host writing a cell

Specified 2026-10-09 (outcells-2026-10-09, ADR 0048).

```lisp
(local age 0 name ""
  ((fmt.Sscan "42 ana" age name) (fn (n e) …)))     ; age and name hold what Go stored
```

**What it is.** An out-parameter is a result: a host function f : X × Ref T → R that writes its
cell and keeps no address is, observably, f' : X × T → R × T (outparams-research.md §1.1), valid
because a cell the compiler owns is unaliased. Inside the call the cell is a **reference**, the
address of a fresh copy of its value, and after it the cell holds what the copy holds. That is
Reynolds' `new x in C` read operationally, and it is exactly what hand-written Go does with
`fmt.Sscan(s, &age)`.

**A declaration says which parameters are cells.** `(p (cell T))`: the host receives the address of
a T, may read and write it during the call, and does not keep the address after the call returns.
`(cell go.Value)` is a cell of any type, its address entering Go's box, which is what Go's `...any`
operands of the Scan family are. A variadic list of them, `(a (array (cell go.Value)))`, makes every
argument after the fixed ones a cell. The target hands the loader one `cells` form per such
declaration, `(cells ARITY LIST i…)`, as it hands it `variadic` ones, and a program loaded without
them is refused. A pointer the host *keeps* (`flag.IntVar`, `sync/atomic`) is
not a cell (outparams-research.md §5.7) and has no declaration of this kind.

**A call passes a cell where its declaration says so, and nothing else there.** At a cell parameter
the argument is a cell in scope, or `(out τ)` (§6). A value there is refused: a host that writes
through the address of a value the program cannot see again has written into nothing. At every
other parameter a cell is read, as anywhere else.

**What it means: the translation, extended.** With r a fresh reference and `#ref`, `#deref` the
reference's introduction and elimination,

```
S⟦(f a… c …)⟧              = let r = (#ref c) in let y = (f a… r …) in let c = (#deref r) in (y, state)
S⟦((f a… c …) (fn (ȳ) M))⟧ = let r = (#ref c) in ((f a… r …) (fn (ȳ) let c = (#deref r) in S⟦M⟧))
```

The arguments are evaluated left to right as before, a cell's reference made where the cell stands,
then the call, then each cell read back in order. `#ref` and `#deref` are impure, so the effect
discipline (ADR 0010) keeps them where they are written: never duplicated, never moved across the
call. The reference is an ordinary argument value, so it passes unchanged through what stands between
the program and the host: a fallible declaration's retraction (errors.md §4.2), a variadic call's list
(variadic.md), and Go's box. After the call the cell is again a loop variable, and nothing below the
loader learns it was passed.

**Types.** `(#ref v) : cell T` for v : T, and `(#deref r) : T` for r : cell T. A `cell T` is one
host value, so it enters the box, and `cell go.Value` accepts a `cell T` of any T. The value read
back is a T the program knows nothing more about: the interval analysis gives it its type's whole
range, and no fact holds of it.

**On each target.** Go: a variable per call site, declared once at the function's top as a
programmer declares the variable whose address a loop passes, assigned at the call, `vC = v; r :=
&vC`, and `*r` after it. Declared where it is used, Go moved a fresh copy to the heap on every loop
iteration, one allocation per call more than hand-written Go (outcells-2026-10-09); declared once,
the counts are equal. Windows: a stack slot, not built yet. Android and the
browser: no host declaration takes a cell (Java and JavaScript have no pointers; their out-parameters
are arrays and objects, already buffers and handles), so no program reaches one there, and the
printers refuse a reference.

## 6. `(out τ)`: a cell for one call

```lisp
((fmt.Sscan "42 ana" (out int) (out go.bytestring)) (fn (age name n e) …))
```

**What it is.** The scope of §5 restricted to one call, `runST` around a single action
(mutscope-research.md §2): `(out τ)` at a cell parameter is a fresh cell holding τ's zero, and the
call's value is **the cells' values, then the declared result's components**, the order a buffer's
write-borrow returns (`EncodeRune`'s buffer, then its count), which hamza chose for cells
(mutscope-research.md §9, decision 4):

```
(f a… (out τ₁) … (out τₘ))  ≡  let rᵢ = (#fresh "τᵢ") in ((f a… r₁ … rₘ) (fn (ȳ) (tuple (#deref r₁) … (#deref rₘ) ȳ…)))
```

`(#fresh "τ")` is a reference to a fresh cell holding τ's zero, Go's `new(T)`. The zero is never a
value the program mistakes for an answer where the host's documentation says the cell means nothing:
the declared result's shape says so (`Sscan`'s cells past `n` keep their zero, and `n` is in the
tuple). It is sugar with a unique expansion, so it needs no soundness argument of its own: it is §5's.
`out` is recognized only at a cell parameter; anywhere else `(out x)` is an ordinary application.

## 7. Not here

- **A declaration whose result names its cell's type**: `errors.As`'s `(option T)`, which needs a
  type variable bound by the cell parameter (mutscope-research.md §7.2). Next.
- **Windows' fixed-type cells** (`GetConsoleMode`'s `LPDWORD`): a stack slot on x86.
- **`build-map`**: a map buffer keeps its own form until a map value carries its capacity.
- **Thaw in place**, §1.
- **A declared range on a cell**, `(c (int 0 200) 0)`.
