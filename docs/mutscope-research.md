# Mutation scopes and out-cells, weighed

2026-10-08. hamza's proposal, weighed against outparams-research.md's recommendation, with the
mathematics, the literature, the measurements, and where the algebra leads. Decisions for hamza in §9.

The proposal:

```lisp
(build buf1 (buffer n)
       buf2 (buffer m)
       mut1 0                  ; or (int 0) ?
       mut2 3.0
       arr1 (array 9 0 7 8 9)
  body)
```

A scope in which destructive operations are allowed, `(set mut1 9)`, and whose contents become
immutable when the scope ends, as a buffer does today. For `fmt.Sscan` it mirrors Go: declare the
variables in a scope, call, return them from the body.

## 0. The answer, in brief

**The proposal is the better primitive, and the out-cell is its one-call special case.**

- What it is: local state encapsulated by a scope, which is Reynolds' Idealized Algol (`new x in C`,
  1981) and Launchbury and Peyton Jones' state threads (`runST`, 1994). The language already has the
  first-order, lexically scoped instance of it: `build` (ADR 0018), which allows destructive updates
  to a linear buffer and freezes it at the scope's end (ADR 0031). The proposal extends what a scope
  may hold from buffers to **cells**, and a cell is a buffer of length one. So it adds no new kind of
  state, only new shapes of it.
- It is more general than out-cells. It covers what a single call's result cannot: state a callback
  changes across invocations (eleven synchronous Go APIs measured, among them `rand.Shuffle`, whose
  callback rewrites the caller's slice), buffers initialized from a value, in-out parameters, and
  (once records exist) Win32's struct cells.
- It is worse where out-cells are good: one call that fills a value. There it is verbose, and it loses
  the **typed shape** a declaration gives the result: `errors.As` read through a cell hands the
  program a value that may be Go's `nil` whenever As returned false, where the out-cell gives
  `(option T)`.
- **The algebra resolves it by layering, not by choosing.** Build the scope with cells as the
  primitive, and define `(out τ)` by its expansion into a one-call scope. Then the 25 caller-typed Go
  APIs keep the concise call and the typed result, everything else gets the scope, and there is one
  semantics, not two. The layering also **removes outparams-research's type pack** (decision 3 there):
  the expansion knows each cell's type, so the declaration needs none (§7).

---

## 1. What the proposal is

### 1.1 Local state, encapsulated

A computation that allocates a variable, updates it destructively, and lets nothing outside see the
variable is **observationally pure**: from outside it is a function of its inputs. This is the
locality property of Algol-like languages. Meyer and Sieber (1988) showed the classical models fail to
capture it fully, and O'Hearn and Tennent (1995) proved it by relational parametricity. In functional
languages it is the **ST monad** (Launchbury and Peyton Jones, *Lazy Functional State Threads*, 1994):

```
runST : (∀s. ST s a) → a          newSTRef, readSTRef, writeSTRef, newArray, freeze, thaw
```

The rank-2 type `∀s` stops a reference from escaping its thread, which is what makes `runST` pure.

### 1.2 The language already has one: `build`

```lisp
(build b n (loop ((i 0) (b b)) (>= i n) b else (again (+ i 1) (set b i (* i i)))))
```

`build` is ST specialized four ways, each the language's own choice:
- **first-order and lexical**: the thread is a syntactic scope, not a first-class action, so the region
  is the scope (Tofte and Talpin's regions, 1994) and no rank-2 type is needed;
- **linear** (ADR 0018): the buffer is used once along each path, so a store may update in place;
- **frozen at exit** (ADR 0031): the scope's value is a product of frozen tables and buffer-free values,
  and a buffer cannot be an element (ADR 0020) or captured by a closure that escapes (closures-
  direction.md), so nothing mutable leaves;
- **one kind of state**: tables.

The proposal relaxes the last point only. Everything that makes `build` sound applies unchanged.

### 1.3 A cell is a buffer of length one

```
Buffer τ n  ≅  Ref τ ⨯ … ⨯ Ref τ   (n factors)        Ref τ  ≅  Buffer τ 1
```

A buffer is a finite family of cells, and a cell is the family of size one. So a cell needs no new
theory: its store is `set` at the only index, its read is the read at index 0, it is linear because a
buffer is, and it freezes to its one value. The proposal's three binder shapes are three
initializations of one thing:

| binder | is | today |
|---|---|---|
| `buf1 (buffer n)` | a buffer of n, zero-filled | `build`'s binder |
| `mut1 0` | a cell, initialized | a buffer of 1, `(set b 0 v)` |
| `arr1 (array 9 0 7 8 9)` | a buffer initialized from a value: **thaw** | `build` and a copying loop |

### 1.4 A scalar cell alone is renaming

A program-only scalar cell, read and written by nobody but the program, adds nothing a loop variable
lacks. In a language whose state is threaded, a variable updated in place is a variable rebound:
that is SSA, and SSA is functional programming (Kelsey 1995; Appel 1998). The loop

```lisp
(build total 0
  (loop ((i 0)) (>= i n) total else (seq (set total (+ total (xs i))) (again (+ i 1)))))
```

computes exactly what `(loop ((i 0) (total 0)) (>= i n) total else (again (+ i 1) (+ total (xs i))))`
does, and the IR lowers both to the same φ. **A cell earns its keep only when its address is seen by
someone else**: a host call that writes through it, a callback the host invokes that reads or writes
it, or a host that keeps it for a while. That is the line between power and style, and the
measurements below are about the first kind.

---

## 2. Out-cells, restated

outparams-research.md's design: an out-parameter is a result, f : X × Ref T → R ≅ X → T × R, valid
because a compiler-owned cell is unaliased. A declaration marks `(p (out T))`, a type variable makes
it caller-typed, and the call writes `(out τ)`:

```lisp
((fmt.Sscan "42 ana" (out int) (out go.bytestring)) (fn (age name n e) …))
```

In ST terms, this is `runST` around a single action whose result is the cell's final value:

```
f x (out τ)   =   runST (do c ← newSTRef ⊥; r ← f x c; v ← readSTRef c; return (r, v))
```

So the out-cell is the mutation scope restricted to one call. Each design is a point in the same
space. The question is which one is the primitive.

---

## 3. What is measured

From outparams-2026-10-08 and this document's own count of Go's manifest:

| need | count | out-cell (A) | scope with cells (B) |
|---|---|---|---|
| caller-typed out-parameters (Scan, `json.Unmarshal`, `binary.Read`, `sql.Rows.Scan`, `errors.As`) | ~25 in 8 Go packages | yes, concise, typed shape | yes, verbose, untyped shape |
| fixed-type out-cells (Win32 `LPDWORD`, `PHANDLE`) | 3,461 Win32 functions | yes | yes |
| synchronous callbacks that return nothing and change the program's state: `Ring.Do`, `expvar.Do` (2), `flag.Visit`/`VisitAll` (4), `rand.Shuffle` (4) | 11 Go APIs | **no** | yes, with tier-1 callbacks |
| callbacks returning a status that accumulate through the closure: `filepath.WalkDir`, range-over-func iterators (`iter.Seq`) | Go idiom (`io/fs`, `iter`: K) | **no** | yes, with tier-1 callbacks |
| a buffer initialized from a value (thaw) | every "copy, then edit" program | no | yes |
| in-out (Win32 `_Inout_` lengths) | Win32, beside buffers | a second form, `(inout T)` | yes, a cell read after the call |
| struct cells (Win32 `RECT`, `FILETIME`; `json` into a struct) | many | records | records |
| shared or retained cells: `sync/atomic` (35), `flag.IntVar` (14), `http.HandleFunc`, `time.AfterFunc` | Go | no | no: concurrency |

The callback count: 120 lines of Go's API manifest take a function (`syscall`, `runtime`, `unsafe`,
`cmd` excluded; a single-line pattern, so a lower bound), 36 match a callback with no result, and read
one by one, 31 take one. Of those, 11 are synchronous calls a program makes for their effect on its
own state, the row above. The rest are concurrency (`http`, `time.AfterFunc`, `sync.WaitGroup.Go`,
`sync.Once`), `testing`, and crypto modes.

So B covers strictly more. What A has that B lacks is not coverage. It is concision at the 25 call
sites, and the typed result shape.

---

## 4. The typing questions the proposal raises

### 4.1 `mut1 0` or `(int 0)`?

A literal's exact type is the singleton {0}, which is too narrow for a cell that will hold other
values. The cell's type is the **join of everything stored into it**, which is exactly how a
buffer's open element is solved today (types.md §3.1: `?int`, a unification variable solved by the
first store, its range given by the IR's interval analysis). So `mut1 0` needs no annotation, and
`(mut1 (int 0 200) 0)` states a range when the program wants a claim, checked at every store as a
buffer element's declared range is. `mut2 3.0` is an `f64`. A cell a host writes must have its
type known before the call, because the host's writing depends on it (Go's `Sscan` parses by the
pointer's type): from the initializer, or declared.

### 4.2 The initial value, and Go's `nil`

Every cell needs an initial value, and for a host type that is the host's zero: `nil` for Go's
`*fs.PathError`. The language has no `nil`. The error model put Go's `nil` inside a niche (ADR 0040),
where the program never sees it. Under B, `errors.As` is used as Go uses it:

```lisp
(build pe (zero go/io/fs.PathError)            ; Go's var pe *fs.PathError: nil
  ((errors.As e pe) (fn (found)
    (if found (PathError.Path pe) "…"))))      ; pe is nil when found is false
```

and the program may read `pe` when `found` is false, a `nil` it can hand to a method that panics. A's
declaration gives the honest type instead, `(option T)`: the cell is visible only in `some`. That is
the strongest argument for keeping A's shapes, whatever the primitive.

### 4.3 Order: threaded or sequenced

A buffer is threaded today: `(set b i v)` returns the buffer, and a host write-borrow hands it back
(`((hex.Decode b src) (fn (b2 n e) …))`). Threading is what lets ADR 0018 check linearity by counting
occurrences, and what the refinement layer's content facts induct on. A cell can be read the same
way, or **sequenced**: `(set c v)` a statement, `c` read where it stands, with ADR 0010's effect
discipline keeping reads and writes in program order (a buffer read is already impure, effects.md
§7c). Threading a host call's cells hands them back as results, which makes B's call look exactly like
A's. Sequencing is what makes the proposal read like Go. Either is sound. It is decision 3.

### 4.4 The scope's value

As today (ADR 0031): a product of frozen buffers, cells' final values, and buffer-free values, taken
apart by a tuple pattern.

---

## 5. Pros and cons

| | A: out-cells `(out τ)` | B: scopes with cells (the proposal) |
|---|---|---|
| **what it is** | `runST` of one action | `runST`, Idealized Algol's `new`, `build` generalized |
| **call site** | an expression, anywhere: `((fmt.Sscan s (out int)) (fn (v n e) …))` | a scope: declare, call, read, return |
| **fidelity to host docs** | Go's name, Go's argument order, the `&` replaced by `(out τ)` | Go's code line for line: `var age int` ↦ `age 0`, `&age` ↦ `age` |
| **result shape** | typed by the declaration: `option`, sum, partial success | the host's raw results, and cells that may hold `nil` or an unwritten zero |
| **generality** | one call's result | any state whose readers are the program, one call, a callback, a host for a while |
| **callbacks** | no | yes: the cell is in scope for a tier-1 callback (callbacks.md), and `rand.Shuffle`'s swap is a store into a scope buffer |
| **thaw** | no | yes, and in place when the source is dead (§7.3) |
| **type machinery** | type variables in host declarations, and a type **pack** for variadic cells | none for Scan (cells carry their types), a cell's type inferred like a buffer element's |
| **new syntax** | `(out τ)`, the first type in argument position | binder shapes in `build`; `set` on a cell |
| **soundness argument** | copy-out = by-reference for an unaliased cell | ADR 0018's linearity and ADR 0031's freezing, unchanged; a cell is a buffer of one |
| **risk** | small | two spellings of program-only state (cells and loop variables, §1.4); `nil` reaching programs (§4.2) |
| **cost** | roughly G and H together | the binder shapes, cells in the checker and IR (a buffer of one: little), host write-borrow of a cell (host-buffers.md at n = 1), thaw |

---

## 6. On each target

A cell is the same storage whichever design names it, so realization is shared:

| target | a cell | a host writing it |
|---|---|---|
| Go | a local variable | `fmt.Sscan(s, &age)`; Go moves it to the heap when the address is boxed, as hand-written Go does |
| Windows | a stack slot | `lea rdx, [rsp+k]`, replacing `__written`'s global |
| Android | a local; a one-element array only when a Java API wants a holder | Android's out-parameters are arrays and objects, already write-borrows and handles |
| browser | a local | Web APIs return results; no holder needed |

The difference between A and B is entirely in the language. Either emits Go's own code.

---

## 7. Where the algebra leads

### 7.1 One primitive, one derived form

Make B the primitive and define A by its expansion. For a declaration with a cell parameter:

```
(f x … (out τ) …)   ≡   (build c (zero τ)
                          ((f x … c …) (fn (r …) (SHAPE r … c))))
```

where SHAPE is the declaration's result (errors.md §4.2's retraction, with the cell as one of H's
components). The expansion is sugar with a unique expansion, the precedent of `def`'s shorthand and of
`build`'s own binder sugar, so A needs no soundness argument of its own: it is B's.

### 7.2 What the layering buys

- **The zero is never seen where it means nothing.** The expansion initializes the cell with `#any`,
  which ADR 0042 already defines as the zero of its type that no reader observes. The declared shape
  then keeps it out of sight: As's `(option T)` shows the cell only in `some`, a sum only on success.
  Where Go documents the zero as part of the answer (Sscan's operands past `n` keep their values), the
  partial-success shape shows it, and Go's zero is exactly what `#any` realizes. User-written scopes
  (§7.4) always give an initial value, so `nil` never reaches a program.
- **No type pack.** outparams-research needed `Ts …` because one declaration had to describe the
  results of a call with any number of cells. Under the layering the *expansion* builds the result
  tuple, and it knows each cell's type from its `(out τ)`. The declaration takes cells of any type,
  `(cells (array (cell go.Value)))`, a cell's address being one host value (types.md §3.3), and returns
  only the host's own results. Its decision 3 goes away.
- **Type variables only where a shape names a cell.** As's `(option T)` and Unmarshal's sum mention the
  cell's type in the result, so those declarations say which: `(target (cell T))` with `(option T)`.
  This is a variable bound by one parameter and used in the result, the smallest form of
  polymorphism. Scan's declaration needs none.

### 7.3 Thaw, in place when it can be

`arr1 (array 9 0 7 8 9)` initializes a buffer from an immutable table, Haskell's `thaw`, which copies
because the table may be shared. When the table is dead after the scope begins, the copy is
unnecessary and the scope can take the table's storage: this is **functional but in-place**, Perceus's
reuse analysis in Koka (Reinking, Xie, de Moura and Leijen, 2021) and Lean 4's destructive updates on
unique values (Ullrich and de Moura, *Counting Immutable Beans*, 2019). The language already decides
the same question for the bignum rung (ADR 0034: a destination needs ownership and liveness), so thaw
in place is a liveness proof on the IR, not a new language rule.

### 7.4 The resulting surface

```lisp
; out-cell: one call, typed shape (derived)
(case (errors.As e (out go/io/fs.PathError)) (some pe) … none …)
((fmt.Sscan line (out int) (out go.bytestring)) (fn (age name n e) …))

; scope: state that outlives one call (primitive)
(build count 0
       seen  (buffer 256)
  …(flag.Visit (fn (f) (set count (+ count 1))))…     ; a tier-1 callback, when built
  (tuple count seen))

; thaw: copy, or reuse in place when xs is dead
(build ys (thaw xs)
  (seq (set ys 0 99) ys))
```

---

## 8. Examples, side by side

### 8.1 `fmt.Sscan`

```lisp
; A
((fmt.Sscan line (out int) (out go.bytestring)) (fn (age name n e) (use age name n e)))

; B, sequenced (decision 3)
(build age 0 name ""
  ((fmt.Sscan line age name) (fn (n e) (tuple age name n e))))
```

```go
var age int        // both emit this
var name string
n, err := fmt.Sscan(line, &age, &name)
```

### 8.2 `errors.As`

```lisp
; A: the typed shape
(case (errors.As e (out go/io/fs.PathError))
  (some pe) (PathError.Path pe)
  none      "not a path error")

; B: Go's shape, nil included
(build pe (zero go/io/fs.PathError)
  ((errors.As e pe) (fn (found) (if found (PathError.Path pe) "not a path error"))))
```

### 8.3 Win32 `GetConsoleMode`

```lisp
; A
((win.GetConsoleMode h) (fn (ok mode) …))         ; the declaration's (mode (out DWORD))

; B
(build mode 0 ((win.GetConsoleMode h mode) (fn (ok) (tuple ok mode))))
```

### 8.4 `rand.Shuffle`, which only B can express

```lisp
(build xs (thaw words)
  (seq (rand.Shuffle (len xs) (fn (i j)
         (let t (xs i) (seq (set xs i (xs j)) (set xs j t)))))
       xs))
```

```go
xs := slices.Clone(words)        // or words itself, when dead after (§7.3)
rand.Shuffle(len(xs), func(i, j int) { xs[i], xs[j] = xs[j], xs[i] })
```

The swap callback stores into the scope's buffer while the host runs. The state is not a result of
`Shuffle`, it is the buffer's contents after it, so it has no out-cell form. It needs tier-1 callbacks
(callbacks.md), not built, and the scope is what gives the callback something to change.

---

## 9. Decisions for hamza

1. **Cells in `build`**: a binder `c v` (or `c τ v`) is a cell, a buffer of one, linear, frozen to its
   value at exit; its type is inferred like a buffer element's, a declared range checked at stores.
   *Recommended*: it is the proposal, and it generalizes what `build` already is.
2. **Thaw**: a binder `a (thaw xs)` is a buffer initialized from a table, copied, or reused in place
   when xs is dead (a liveness proof on the IR). *Recommended.*
3. **Cells sequenced or threaded** (§4.3): sequenced reads like Go and needs reads and writes ordered
   by ADR 0010's discipline; threaded keeps ADR 0018's occurrence check as it is and hands a host
   call's cells back as results. *Recommended: sequenced for cells, buffers unchanged*, the IR seeing
   both as stores and reads of a region.
4. **A host call may write-borrow a cell**: host-buffers.md at length one; a declaration's parameter
   `(cell T)`, and a variadic list of cells of any type for Scan. *Recommended.*
5. **`(out τ)` as derived sugar** (§7.1): a one-call scope with the declaration's shape; type variables
   only where a shape names a cell; no type pack. *Recommended*: it keeps the 25 call sites concise and
   their results typed.
6. **No `nil` in programs** (§4.2): a user-written cell is always initialized, and a host's zero is
   reached only through the derived form, behind a shape. *Recommended.*

This **supersedes outparams-research's decisions 1–3** if taken: its out-cell becomes the derived
form, and its type pack is not needed.

**Order to build**: cells and thaw in `build` (1, 2, 3), with a program; then a host write-borrow of a
cell (4), with the Scan family and Win32's `GetConsoleMode`; then `(out τ)` (5), with `errors.As`;
callbacks over scope state later, with tier 1.
