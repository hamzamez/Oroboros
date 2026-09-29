# A table's eliminator and store, and the IR's claim edge

2026-09-29. The two items [tabletype-2026-09-29](tabletype-2026-09-29.md) left unbuilt, measured
before they were built. The spec is [types.md §3.1 and §7](../../docs/spec/types.md).

## 1. Measured first: soundness, not messages

The checker is not the only judge: the IR types tables by unification, and W5 checks flow edges. So
the question was whether a missing rule let a wrong program through, or only gave a worse message
before a later refusal. Six wrong programs, on Go and on JavaScript, the host with no type layer of
its own:

| wrong program | before |
|---|---|
| `(len a)` under a declared `string` result | **emitted** `func XF(v0 []float64) int` |
| `(t 0)`, read from a table of `f64`, under `string` | **emitted** `func XF() float64` |
| `(set (set b 0 1.5) 1 "x")` on Go | **emitted**, and then refused by Go's compiler in the generated code |
| the same on JavaScript | **emitted, and wrong**: the string `"x"` stored into a table of `f64` |
| `(go.f+ (a 0) 1.0)`, `a : (array string)` | refused only by the IR verifier: *"W5: %5 is string and flows into…"* |
| `(+ (a 0) 1)` on JavaScript | refused for the wrong reason: *"cannot be proven to stay inside the word"* |

The JavaScript store is types.md §1's founding measurement, the program that printed `hello1`, met
again one level down.

## 2. What they are

Table(σ) is a type former, and a type former is its rules:
- **introduction** (built in tabletype);
- **elimination**: `a : Table(σ), i : int ⊢ (a i) : σ`, and `len : Table(σ) → int`;
- **store**: `set : Table(σ) × int × σ → Table(σ)`.

Without elimination the system does not close under its own reductions: `((table n f) i)` reduces to
`(f i)`, whose type is σ. That is subject reduction (Wright and Felleisen, 1994). A read typed unknown
agreed with every demand.

The second item was the same defect from the other side. `?` was meant as an unknown element to be
solved, but nothing solved it: it agreed with every element, for ever. So the first store never fixed
σ, and a second store of another type agreed too. Treating `?` as a variable solved by the first
demand is unification, and the residual is monomorphic, so it needs no generalisation (types.md §2).

**One subtlety, handled as algebra rather than a special case.** Solving `?` from an integer store
must not fix the integer's **realization**. `(set b 0 104)` making `b` a table of `int` would make
`hex.Encode`'s `(buffer (int 0 255))` refuse a correct program. Which realization holds an integer is
the IR's choice (ADR 0033). So an integer solves the element to `?int`, **the sort ℤ with its
realization open**: it agrees with any integer element and with nothing else, and it is not solved
further.

## 3. What was built

**In the checker** (`emit/tabletype.go`):
- a read `(a i)` has type σ; an open `?` is solved by the read's demand, and `?int` reads as `int`;
- `(len a)` has type `int`;
- a store demands its value at σ and solves an open element on the buffer's root name
  (`BufferRoot`), so a second store is checked against the first;
- a graph and a rule introduce the **join** of their entries: their common type, `?int` if all are
  integers, `?` if they disagree;
- one rule for known elements: a language table agrees with a host type that ρ realizes identically
  (`slice-float64` and `(array f64)` are both `[]float64`). It compares host types, which W5's last
  step already does, so W5 is unchanged.

**In the IR** (`ir/claim.go`, called from `Lower`): each yield meets its declared result by sort. The
relation is the checker's (ℤ one sort, `any`, host identity, subsumption), lifted through tables and
maps element by element, with a host alias read as the table it realizes (`aliasOf`). W5 compared a
yield with the function's result, which lowering sets to the declaration only for a table, so for
every other declaration the edge compared a value with its own type. It is the second line: the
checker refuses these first, and in source terms.

## 4. After

| wrong program | now |
|---|---|
| `(len a)` under `string` | *"(len …) is int, but string is required here"* |
| `(t 0)` under `string` | *"(t …) is f64, but string is required here"* |
| two stores, Go and JavaScript | *"in a store's value: a string literal is string, but f64 is required here"* |
| `(go.f+ (a 0) 1.0)` | *"in argument 1 of go.f+: (a …) is string, but f64 is required here"* |
| `(+ (a 0) 1)` on JavaScript | *"in argument 1 of +: (a …) is string, but int is required here"* |

Still accepted: `(build b 2 (set b 0 104))` under `(array (int 0 255))`. A store of an integer fixes
the sort, not a realization.

## 5. Witnesses

| rule | planted fault | caught by |
|---|---|---|
| a read has the element's type (the shipped bug) | read untyped | `TestATablesReadAndLengthAreTyped` |
| `len` is an `int` | untyped | the same |
| a store solves an open element | solves nothing | `TestAStoreSolvesTheElement`, and three others |
| an integer solves the sort only | fixes `int` | `TestAnIntegerStoreLeavesTheRealizationOpen` |
| `?int` agrees only with integers | agrees with anything | the same, after a case was added (below) |
| a known table meets a host type realizing it | not | `TestATableFormMeetsATableDemand` |
| the IR's claim edge can fail | accepts everything | `TestTheIRChecksADeclaredResultsSort` (bypassing the checker) |
| …and sees through tables | does not | `TestTheIRsClaimEdgeAdmitsWhatTheSortAdmits` |

**One plant survived first.** "`?int` agrees with anything" passed, because the test's only case was
a float stored after an integer, which the store's **demand** refuses before the relation is read. The
relation matters where a table of open integers meets a known non-integer element:
`(build b 2 (set b 0 104))` under `(array f64)`. With that case added, the plant is caught.

## 6. Gates

- **Emission is byte-identical**, refusal texts included, over all 492 source × target runs. No
  program in the corpus changed outcome.
- **Every other step passes**: the compiler, IR (251 of 251), differential, and tooling steps, the last
  with the sixteen acceptance programs. The README's hex program prints `686921`.
- **Compile time** is 1.04×, within noise.

## 7. Not built

- **A map's eliminator.** A read gives `(option V)` (maps.md), and the checker does not type options.
  `insert` demands a known value type but does not solve an open one.
- **Solving through a copy.** `(let t b …)` gives `t` the type `b` had when bound, and a later store
  into `b` does not reach `t`.
- **An application whose operator is not a name**, `((build …) i)`, is not typed as a read. A rule
  applied directly reduces by β, and an impure build is let-bound by ADR 0010, so this shape should be
  rare in a residual. That is not measured.
