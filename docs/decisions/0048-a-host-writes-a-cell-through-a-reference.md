# 0048 — A host writes a cell through a reference, and `(out τ)` is a cell for one call

Date: 2026-10-09
Status: Accepted (hamza: "go first", then "go ahead"). Extends ADR 0047; closes cause I of
gocoverage-2026-10-06 for Go.

## Context

Go's out-parameters are all caller-typed through `any`: the Scan family stores each operand through
the pointer it is given (`fmt.Sscan(s, &age, &name)`), as `errors.As`, `json.Unmarshal` and
`binary.Read` do. Go's 49 explicit pointer parameters are atomics and flags, which keep the address.
So Go's first program needing ADR 0047's stages B and C needed them together: cells passed where the
host stores, in a variadic `...any` list, and `(out τ)` for a cell that lives for one call
(mutscope-research.md §7, outparams-research.md §5).

A fallible declaration is a target-library definition around its raw host call (ADR 0040's
retraction), with fixed arity, and Sscan's number of cells varies per call. So a cell's new value
cannot come back as an extra result of the raw call: that would need a result list of variable
length through a definition that knows nothing of it, the type pack the research ruled out.

## Decision

1. **Inside a host call, a cell is a reference**, the address of a fresh copy of its value, and after
   the call the cell holds what the copy holds:
   `S⟦(f a… c …)⟧ = let r = (#ref c) in let y = (f a… r …) in let c = (#deref r) in (y, state)`.
   f : X × Ref T → R that keeps no address is observably X × T → R × T, because a cell the compiler
   owns is unaliased; this is Reynolds' `new` read operationally, and Go's own `&age`.
2. **`#ref`, `#fresh` and `#deref` are the language's**, injected into every target, impure, and out
   of a program's reach. The effect discipline (ADR 0010) keeps each on its side of the call. The
   reference is an ordinary argument, so it passes unchanged through the retraction, the variadic
   list and Go's box, and nothing below the loader learns a cell was passed.
3. **A declaration says which parameters are cells**: `(p (cell T))`, the address of a T the host
   may read and write during the call and does not keep. `(cell go.Value)` is a cell of any type in
   Go's box: the loader records the position and gives the parameter the box type, below which every
   `cell T` is derived (ADR 0045). The target hands the loader one `cells` form per declaration, as it
   hands `variadic` ones, and a program loaded without them is refused.
4. **At a cell parameter a call passes a cell in scope or `(out τ)`**, and anything else is refused:
   a host writing through the address of a value the program cannot read again has written into
   nothing.
5. **`(out τ)` is a fresh cell holding τ's zero, for that call**, and the call's value is the cells'
   values, then the declared result's components: a buffer write-borrow's order (hamza, mutscope
   decision 4). It is sugar with a unique expansion over decision 1, recognized only at a cell
   parameter.
6. **The IR has a `cell T` type and three statements**, `cell-ref`, `cell-fresh`, `cell-get`. Go
   prints a variable declared once at the function's top, assigned at the call, and its address, as
   a programmer writes `var v int` before a loop; a read-back value has its type's whole range in
   the interval analysis. JavaScript, Java and x86 refuse a reference: no host of theirs takes one
   yet (Win32's fixed-type cells are next, on x86).
7. **A cell's translation runs after resolution**, where a call names the declaration whose cell
   parameters it fills, and before the variadic rewrite, which gathers a call's references into its
   list. Its threading keeps a call's head in place, so the rewrite still reads the call by name.

## Why not

- **The cells' new values as extra results of the host call.** The raw call sits inside a
  retraction definition of fixed arity, and Sscan's cells vary per call; it is the type pack the
  research ruled out, and every consumer of a host call's results would learn of it.
- **A cell region in the IR** (`(#cells (fn (r…) body) v…)` yielding the body's value and the
  cells'). Every analysis would learn a new scope form, and the region would have to hold the call's
  continuation for its values to flow on, which an `again` in the continuation would leave. A
  reference is a value, and the values the call needs are the ones it has.
- **A one-element buffer as the reference.** A buffer passed to a host is a write-borrow it must
  return, or a consume; the variadic list has no place to return it, and the analyses read a
  buffer's contents off its store chain, which a host write breaks.
- **Templates that own the pointer** (outparams-research.md §4.1): one per type and arity, under
  names Go does not have, the mistake ADR 0046 closed.
- **The copy declared where it is used.** Measured: Go moved a fresh copy to the heap on each loop
  iteration, 3,999 allocations against hand-written Go's 3,000 over a thousand calls. Declared once,
  the counts are equal and the times within noise.

## Consequences

- The nine Scan functions are declared under Go's names (`fmt` 23 of 29, Go 302 of 590, 51.2%), and
  cause I is closed for Go. `examples/lines/ages.oro` reads people with `(out …)` cells and keeps its
  state in cells; `gauntlet/stdlib/acceptance/fmt-scan.oro` prints what hand-written Go prints.
- `errors.As` needs a declaration whose result names its cell's type, `(option T)`: a type variable
  bound by the cell parameter (mutscope-research.md §7.2). Next.
- Win32's fixed-type cells need a stack slot on x86.
- Writing it found three faults that predate it, each with a witness now: the reducer's purity was
  first-order and weakened an applied λ's effect (a discarded cell scope lost its body); the
  unifier merged two terms before their arguments unified; and a variadic call with a cell argument
  inside a scope was never rewritten.
