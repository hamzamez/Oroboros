# Types, layer 1

Written before the code, per [state.md §6](state.md). The argument is in
[types-direction.md](../types-direction.md); the picture is in [types-sketch.md](../types-sketch.md).

> **Status, 2026-09-28. Built, and narrower than it was.** `emit/check.go` runs on every residual
> before lowering, in `cmd/build` and `cmd/gen` (the pipeline is `ir.Entry`). Since it was first
> written (2026-08-15), three things moved:
> - **An integer's sort is ℤ** ([ADR 0026](../decisions/0026-an-int-is-an-integer.md),
>   [ADR 0033](../decisions/0033-an-integers-representation-is-chosen-on-the-ir.md)). A range, the
>   unsigned word and a range above the word are one sort here. Which realization holds a value is
>   chosen on the IR by its fact, and checked there, by the verifier's rule W5.
> - **Refinements are built** ([refinements.md](refinements.md)): indices, divisors and every
>   declared precondition are obligations the refinement layer discharges or refuses.
> - **The structural forms are the language's**: `loop`/`again`, `if`, `let`, a host call's
>   continuation, a tuple from a loop or a scope, and an ascription. `fold-range`, `loop2` and
>   `make-vec`, which §4 once listed, belong to the retired portable layer; they remain on `blas` and
>   the book's `tutorial*` targets and are checked there.
> - **A table's constructor is typed** (2026-09-29, §3.1, [tabletype-2026-09-29](../../gauntlet/results/tabletype-2026-09-29.md)).
>   Until then every table and map form was *unknown*, which agrees with everything, so
>   `(array 1.0 2.0)` under a result declared `f64` compiled to a Go function returning `[]float64`.

---

## 1. The gap, measured

```lisp
(def main (fn () (io.print-line (f64.add "hello" 1.0))))
```

| | |
|---|---|
| reduction | happy — β and δ are untyped |
| Go | build fails, with the message pointing at *generated* code |
| Java | build fails |
| **JavaScript** | **builds and prints `hello1`** |

So the effective type safety of an Oroboros program depended on which target it was emitted for,
which is the opposite of what this project claims. That is the whole of the case for a checker of
our own, and it is a measurement rather than a principle. Today the program is rejected identically on
every target:

```
main: in argument 1 of io.print-line: in argument 1 of num/f64.add:
      a string literal is string, but f64 is required here
```

## 2. What is checked, and where

> **The residual, before lowering.**

Not the source. The residual is **monomorphic, first-order and closed**
([types-direction §3.3](../types-direction.md)) — reduction has specialised every generic definition,
and a closure may not survive staging — so checking it needs no type schemes and no generalisation.

**Not in a backend.** A target's declarations carry argument types even where its host has no type
layer (`targets/js/` declares them and never uses them), which is what lets one checker serve every
target, including the one that would otherwise catch nothing.

**Three other checks share the work, each with its own question:**

| | asks | where |
|---|---|---|
| this checker | does every host call and language operator get arguments of its declared types, and every form agree with itself? | `emit/check.go` |
| the refinement layer | is every index, divisor and declared precondition inside its domain? | `emit/refine.go`, [refinements.md](refinements.md) |
| linearity | is every buffer used once? | `emit/linear.go`, ADR 0018 |
| the IR | which representation holds each value, and does every value flow into a position of its own? | `ir/typing.go` (unification), `ir/verify.go` (W5), [ir.md §4](ir.md) |

## 3. The judgement

A type is one of the target's declared type names, a range `(int LO HI)`, or **unknown**.

```
type(integer literal)      = int
type(float literal)        = f64
type(boolean literal)      = bool
type(string literal)       = string
type(name)                 = whatever has been demanded of it, or unknown
type((p a…))               = p's declared result type
```

Checking is a walk. At every primitive application, each argument is **demanded** to have the
primitive's declared argument type, and the argument's type must **agree** with the demand:

| the argument's type | what happens |
|---|---|
| equal to the demand | fine |
| **unknown**, and the argument is a name | the name is **bound** to the demanded type — the inference half |
| unknown, otherwise | fine; nothing is learned |
| `any` on either side | fine: `any` demands nothing ([target-files.md §2](target-files.md)) |
| the same host type under two keys | fine: what they realize is what they mean (`Target.SameHostType`) |
| a subtype of the demand | fine, by a declared `include` (`*os.File` where an `io.Reader` is wanted, [interfaces.md](../interfaces.md)) |
| **two integer types** | fine: the sort is ℤ (below) |
| a **different** concrete type | **error** |

A binder's type holds for its body only: a name's entry is restored when the binder's scope ends.

### The integer sort is ℤ

`int`, `(int 0 255)`, `(int 0 18446744073709551615)` and `(int 0 +inf)` are all types of integers,
and they agree with each other (ADR 0033). A declared range is a **claim about values**, and whether
a value satisfies it is not a question of sorts:
- that a value lies in a declared range is an obligation (ADR 0028), discharged on facts or refused;
- which realization holds it, the signed word S, the unsigned word U or exact precision, is ρ of its
  fact, chosen on the IR;
- a value held exactly that reaches a position declared in the word is refused there
  (`ir.SelectBig`), with the message this checker used to give: *"… is WIDER than the word of target
  …"*.

**One exception is a capability, not a representation.** ℤ, an unbounded range, on a target that
declares no bignum cannot be held at all. That disagreement stays here, and says so: *"… needs
arbitrary precision — … and the target declares none."*

### 3.1 The table type: introduction, elimination, store

Two type constructors join the scalars and the declared names: **Table(σ)**, spelled `array σ` or
`buffer σ`, and **Map(κ, σ)**, spelled `map κ σ`. A type former is its rules, and a table's are
three kinds (built 2026-09-29: [tabletype](../../gauntlet/results/tabletype-2026-09-29.md) the first,
[tableelim](../../gauntlet/results/tableelim-2026-09-29.md) the other two).

**The element may be open.** An element is a type, or one of two **open elements**, which are
unification variables, not wildcards:
- `?`, the unknown element: nothing is known yet;
- `?int`, the element **sort ℤ with its realization open**. Which realization holds an integer is
  the IR's choice (ADR 0033), so learning that a table holds integers fixes the sort and nothing
  more. `(set b 0 104)` must not make `b` a table of `int` that `hex.Encode`'s
  `(buffer (int 0 255))` then refuses.

A variable is **solved** by the first thing that fixes it: a store's value, or the demand on an
element that is read. A value of the integer sort solves `?` to `?int`, and any other type solves
it to that type. `?int` is not solved further. Solving is monotone: an element, once known, is only
checked against after that.

**Introduction.**

```
type((array e₁ … eₙ))       = array ⊔eᵢ      the join of the entries: their common type; ?int if
                                              all are integers; ? if they disagree or n = 0
type((table n (fn (i) e)))  = array type(e)   with i : int, joined the same way
type((alloc t))             = buffer ?
type(b) in (build n (fn (b) e))   = buffer ?  the scope's buffer, solved by its stores
type((build n (fn (b) e)))  = type(e), frozen: buffer σ becomes array σ (ADR 0031);
                              a table demand is checked on the frozen value, at the exit
type((map …)), type(m) in (build-map k (fn (m) e))  = map ? ?
type((keys m))              = array ?
```

When a table form is **demanded** at a table type, the demand is its type, and each element it
states (a graph's entries, a rule's body) is demanded at the element type. That is how
`(array 104 105 33)` meets `hex.Encode`'s `(array (int 0 255))`: each literal is an integer, and the
integer sort is ℤ.

**Elimination.** A table is a function on `[0, len)` (tables.md), and its eliminators are
application and its domain bound:

```
a : Table(σ),  i : int   ⊢   (a i) : σ        an open ? is solved by the read's demand;
                                              ?int reads as int, the sort
a : Table(σ)             ⊢   (len a) : int
```

This is what makes the type system close under its own reductions. `((table n f) i)` reduces to
`(f i)`, whose type is the rule's σ, so the application must have type σ too. That is subject
reduction (Wright and Felleisen, 1994), and without the elimination rule a read was *unknown*, which
agrees with every demand.

**Store.** `set : Table(σ) × int × σ → Table(σ)`: the index is demanded at `int` and the value at σ,
and the result is the buffer's type. When σ is open, the store's value solves it on the buffer's
root name (`(set (set b i x) j y)` writes to `b`), so a second store is checked against the first:
`(set (set b 0 1.5) 1 "x")` is refused. `insert` is typed the same way, and its map's elements are
not solved: a map's value type is not yet read anywhere a solution could be used.

**The relation**, for open elements and host aliases:

| | |
|---|---|
| a table whose element is open, against a table type or a host type that realizes one (`slice-float64`, which Go realizes as `[]float64` = ρ(`array f64`)) | the elements agree: `?` with anything, `?int` with any integer element |
| a language table against a host type that realizes it | agrees, when ρ gives both the same host type |
| `map ? ?` against a map type | agrees |
| an open table against anything else concrete | **error**: *"… is a table, but f64 is required here"* |

Open elements do not tell a table from a buffer: which tables may be written is linearity's question
(ADR 0018, ADR 0020), not a type's. Two **known** language tables are compared as before, invariantly
and constructor by constructor, so a target that declares `[]byte` still refuses an `[]int` program.
That is why a scope's value is frozen as it leaves: `hex.Encode` gives back `buffer (int 0 255)`, and
the scope's value is the `array (int 0 255)` that `os.text-of` reads.

**Why this is sound and loses nothing.**
- *Sound.* Every term whose value is a table is introduced by one of the forms above or by a
  declaration, so its type is Table(σ). The relation never equates a table with a scalar, and every
  read and store is typed at σ.
- *Loses nothing.* An open element agrees with every element the old unknown agreed with, until a
  store or a read fixes it, and a program whose stores disagree was never correct. An integer never
  fixes a realization.
- *The IR is untouched.* Open elements never reach the IR, which types tables by unification
  (`ir/typing.go`). W5 reads this relation over the IR's types, which name every element or say
  `any`. The alias rule compares host types, which W5's last step already does. So the relation W5
  reads is unchanged on every input it sees.

**Not built.** A map's eliminator: a read gives `(option V)` (maps.md), and the checker does not type
options. And an element is not solved through a copy: `(let t b …)` gives `t` the type `b` had when
bound.

## 4. The structural forms

Their types are the language's, not a target file's ([target-files.md §4](target-files.md)):

| | |
|---|---|
| **`loop`/`again`** | each variable takes its initial value's type (walked outside the loop's scope); each `again` argument agrees with the variable it feeds; each exit agrees with the loop's type. A clause body may sit under a `let` or a host call's continuation, and the chain is walked through both (ADR 0015, ADR 0027) |
| **`if`** — `(if c t e)` | `c : bool`; the branches agree. Two integer branches **join in ℤ**, and the wider types the conditional |
| **`let`** | the binder takes the value's type, for the body |
| **a host call with several results** | its arguments are demanded as declared; the continuation's parameters take the declared results, and the term has its body's type |
| **a tuple from a loop or a scope** | each projection Pⱼ is walked, and its type is the j-th name's (ADR 0031, [tables.md §2.5](tables.md)) |
| **`(the T e)`** | e is checked against T, and the term has type T. It is how a declared result above the word survives inlining |
| **the language's integer operators** | `+ − · / %` take `(int, int)` and give `int`; the orders and `=` give `bool` |
| **tables and maps** | by §3.1: introduced with their constructor and an element that may be open, eliminated by application (at σ) and `len` (at `int`), stored into at σ, which solves an open element. A map's read is not typed. The IR types every element, by unification (`ir/typing.go`); their domains are the refinement layer's and their linearity is linear.go's |

## 5. What this deliberately does not do

- **Representation.** Which host type holds a value is the IR's, checked by W5 (§3).
- **Refinements.** Built, as their own layer ([refinements.md](refinements.md)).
- **Polymorphism.** The residual has none left; the *source* does, and checking source would need it.
- **A stuck application.** `((fn () "hamza"))`'s operator has no declared type to disagree with;
  lowering to the IR refuses it ([def.md §4](def.md)).

## 6. What must not happen

The checker must **reject no program that is currently correct.** Every example, every gauntlet
program, and every generated file must pass unchanged. If one does not, either the checker is wrong
or a target file is — and given the record, the target file is a live possibility: `fold-range`'s
declared type was false for months, `stmt`'s result type was never implemented, and `loop2`'s was
correct only by luck.

That is the acceptance test, and it is more interesting than the negative one. One bug in the checker
itself is worth keeping: the first version skipped `KFn`, reasoning that a bare abstraction is an
escaping closure the emitter rejects anyway. But **the top-level term of every residual is an
abstraction**, so it checked nothing at all. A checker that accepts everything is indistinguishable
from a correct one until you have a case it must reject.

## 7. `sig` — a claim, checked in two directions

```lisp
(module num/vec)
(export dot)
(sig dot ((a vec-f64) (b vec-f64)) f64)
(def dot (fn (a b) …))
```

A signature is checked against **both** implementations of the name (`emit.CheckSignatures`):

| the target | what is checked |
|---|---|
| **provides it natively** — blas declares `num/vec.dot` as `cblas_ddot` | the target's declared argument and result types against the signature |
| **derives it** — Go has no native `dot` | the *definition's* residual: parameters take their declared types, and the body must produce the declared result |

The first is **the job no host compiler can do.** The library's definition and the target's native
declaration are two implementations of one statement, and they live on *different targets*, so no
single host compiler ever sees both. That is [modules.md T2](modules.md)'s substitution soundness,
machine-checked.

```
num/vec.dot: argument 2 is int in target blas, but vec-f64 in its signature
```

**And the IR checks the claim's sorts, as a second line** (`ir/claim.go`, 2026-09-29). Each yield
meets its declared result by sort: the relation above, with ℤ one sort, lifted through tables and
maps element by element and reading a host alias as the table it realizes. Before, W5 compared a
yield with the function's result, which lowering sets to the declaration only for a table, so for
every other declaration it compared the yield with its own type and could not fail.

**The claim's representation half is the IR's** (`ir.CheckClaim`). With the integer sort ℤ, a body
the checker types `int` agrees with a result declared `int`, even where the value is held exactly. So
the definition is lowered on its own and its values held exactly are chosen: one that reaches a
result declared in the word is refused. A missing operation on the target's rung (windows has no limb
form for a quotient by 2³²) is not a violation of the claim; it is refused where the definition is
used.

### Parameters are named

`((a vec-f64) (b vec-f64))`, not `(vec-f64 vec-f64)`. A refinement attaches to a **name**:
`(where (= (len a) (len b)))`, and a parameter's range is the same claim as a `where` on it (ADR
0028). A bare type is still accepted, which is the shape `prim` uses, so the two grammars have not
diverged.
