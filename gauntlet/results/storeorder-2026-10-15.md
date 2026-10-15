# storeorder-2026-10-15 — a buffer in program order (ADR 0052), built behind the gate

Step 4 of the plan after ADR 0068, its first ADR: a buffer is a state binder holding a table, written
in program order, `b[i] := v` (ADR 0052, storeorder-research.md). Built as spec/local.md §2.1
specifies it: J writes every buffer this way, today's notation keeps threading, and the two meet in S,
the state-passing translation, whose image is today's threaded residual.

## 1. What was built

**Σ and D** (`core/abs.go`, `core/desugar.go`, `core/scope.go`). Two operators: `Write`, places and
values, and a `Place` binder of `local`; a λ's `Borrows`. Each new form is a derived operator whose
equation is its D image (spec/local.md §2.1):
- `b[i] := e` ↦ `(set b (set b i e))`, McCarthy's store on the current version;
- `b[i][j] := e` ↦ `(set b (set (b i) j e))`, the curried store into a record of a table of tuples;
- `x̄ := ē` ↦ let t̄ = ē in x₁ := t₁; …, Dijkstra's concurrent assignment;
- `var b := array { … }` ↦ build's region, its binder a state scope `(#place b (fn (b) …))`;
- `fn f(p linear, …)` ↦ λ(…). `(#borrow p (fn (p) …))`, a borrowed parameter a place in the body.

**S** (`core/cell.go`, `core/place.go`). The cell translation, extended by one clause, the **borrow**:
a call lending a place at a linear parameter, of a definition or a host declaration with a
`(buffer V)`, rebinds the place to the version handed back. The callee's value is the borrowed
buffers first, then its result, a unit result dropped (B × 1 ≅ B); the targets hand the loader one
`(borrows RESULTS i…)` form per such host declaration (`emit/target.go`, `borrowForms`), and a
definition's is read off its body, a least fixpoint over unit bodies. Rules **R0–R4** are decided on
each place's scope (`checkPlace`), each refusal naming its rule.

**U, unthreading** (`j/unthread.go`), S's converse on today's corpus: a version of a place (a name its
chain is bound to, a store chain, a borrow, a loop, a φ) becomes writes; a loop forgets its carriers,
each back edge's carrier arguments the writes before the jump; moves between places one concurrent
assignment, refused unless a permutation; a definition returning the buffer it was given borrows it.
Host borrows are read from the target files, as `borrowForms` reads them. The printer prints the new
nodes, and J's reader reads them back.

**The gate covers libraries**: `core.ReadLibrary`, replaced under `ORO_VIA_J` by `j.ViaJLibraries`, so
a program's imported modules are read through J too, not its entry file alone (jread §4's limit).

## 2. What S had to learn to give today's residual

Each found by comparing the two normal forms of a program, `ORO_VIA_J=1 cmd/oro` against `cmd/oro`:

1. **A loop carries what it writes, first, in binding order.** Measured first
   (`gauntlet/surface/stores.go`): 49 of the 53 loops that thread a buffer by hand put it first, in
   binding order; §3's translation appended a cell, and carried one the loop only read. Both are
   isomorphisms: a product's order is its symmetry, and a variable every back edge passes unchanged is
   invariant.
2. **A selection moves into a loop's exits and a region's body** (`core/reduce.go`). The reducer moved
   a projection πⱼ into a loop; an inner state scope drops its own component and keeps the outer state,
   a selection (fn (x̄) (tuple xᵢ …)), for which the same argument holds: its body is variables, holding
   no `again` and no code to repeat. And a region's value is its body's.
3. **A binder takes its value under its own name**: S bound a `let`'s value to a fresh name, and where
   β keeps the binding (freq's `s = d - 1`, used twice) the facts were about the fresh name, and an
   index proof failed.
4. **A condition that touches no state stays where it is**: binding it left a `let` of a table read
   that β, rightly, does not substitute into a body with effects.
5. **A statement `if` writing one state binder is a φ on it**: x := if c then π_x(S⟦a⟧) else
   π_x(S⟦b⟧). The tuple form had the reducer's case-of-case copy the rest of the block, a jump, into
   both branches (jsonfmt: 110 operations against 95, and a loop more).
6. **A version used once, first, is put where it is used** (`core/reduce.go`, `firstUse`): let x = e in
   E[x] = E[e] when E evaluates its hole first and unconditionally, with nothing impure before it,
   Moggi's let-β for a linear use; it neither contracts, weakens nor exchanges, so it holds of an impure
   e (ADR 0010). Applied to every binder it was sound and broke four analyses, which key a
   postcondition and an exit's bound on the name an impure value is bound to; so it is applied to the
   versions S binds, named `#v`. A place is threaded under its version name, `#v` and its own, so
   every binder of it S makes carries the mark. An operator is no hole (a table read keeps a variable
   there, and lowering takes no other application), and a loop's initializer is one only for a value
   holding no loop: a store chain there is what the content facts induct on (tree's `walk` needs it),
   and a loop there was analysed again for every candidate invariant Houdini checks (freq compiled
   1.5× slower).
7. **An argument that writes no state stays in place**: S bound every argument of a call; a φ argument
   of a jump that reads a buffer became a bound table read, a `let` of an `if`, and two jumps (tree).
   Sound only where the caller uses the value once, so a reference, `(#ref c)`, stays bound: the host is
   handed it and the cell read back through the same one. Left in place, it was made twice, and
   `cmd/build`'s ages test caught the wrong answer, 0 for every age.
8. **The `let` conversion keeps its binder's name** unless it would capture a free name of what moves
   under it: it renamed every binder `#c`, so a program's `c` lost the facts keyed on it, and freq's digit
   count was summarised 731 times where it had been 148. Today's freq compiles faster for it too
   (11.5 s to 6.6 s, serial, warm), with emission unchanged.
9. **An element write and a borrow returning its buffer alone put the new version straight into
   their place's slot of the state tuple**, with no binding of their own to reduce.

## 3. Measured

| | |
|---|---:|
| programs printed in program order, every comment carried, no threaded spelling left | **152 of 152** |
| … read back by J's reader to what was printed (R2) | **152 of 152**; planted fault (an index read twice) 119 |
| target files and fragments read back | 43 of 43 |
| `ORO_VIA_J=1 cmd/check -skip tooling`, every module read through J | 560 runs; differential passes |
| … identical to today's path | all but 2: `jsonfmt` (one computation before a store instead of after, where today's program names a version with a `let`) and `kara/core`'s refusal text (a version binder `#vout2` where it read `out3`) |
| compile time through J | 1.05× the baseline; freq 6.4 s against 6.6 s, serial |
| `cmd/check -skip tooling`, today's notation | the two cell programs changed (accepted, below); everything else byte-identical; 0.98× |
| corpus loops threading a buffer whose header S reproduces | 52 of 53 (`encoding-hex`'s `(i b)` is the one) |
| version renames ADR 0052 said R0 refuses | **0**: alloc-copies' four are renames of a dead version, which unthreading spells as the place |
| moves between places ADR 0052's R0 would refuse | **3**, the merge sorts' swap: G5 |

**Accepted into the baseline** (`gauntlet/check/ACCEPTED.md`): `examples/io/count.oro` and
`examples/lines/ages.oro`, the two programs with cells, now with their cells first and one join where
duplicated branches were, 19 fewer operations and the same 45 unproven: S's rules are one for cells and
buffers.

**What ADR 0052 got wrong, measured by building it**: it counted 4 refusals (alloc-copies' renames),
which are none, and missed 3 (the swaps), which need G5; its R0 admits growth as `b := f(b, …)`, which the
one borrow rule reads otherwise (G7); and the record store had no place (G6).

## 4. Planted faults

| rule | fault | caught by |
|---|---|---|
| R0 (fresh value) | a non-fresh whole write accepted | `TestEachRuleOfAPlaceRefusesWhatItForbids/R0` |
| R0 (moves a permutation) | a move without a refill accepted | `…/R0#01` |
| R1 (bound to a name; written as a value; in a tuple) | each accepted | `…/R1`, `…/R1#01`, `…/R1#02` |
| R2 | a buffer handed to one call twice | `…/R2` |
| R3 | a closure over a buffer | `…/R3` |
| R4 | a borrowed buffer as its λ's value | `…/R4` |
| firstUse: purity before the hole | ignored | `TestALetOfAVersionUsedOnceFirstIsItsPlace`, "after an effect" |
| firstUse: a branch | treated as first | "in a branch" |
| firstUse: a loop's body | treated as first | "in a loop's body" |
| the reader's place index | read twice | `TestEveryProgramReadsBackToItsMeaning`, 119 of 152 |

The semantic law, S ∘ U = id, is checked by emission: `TestABufferInProgramOrderMeansTheThreadedBuffer`
on five cases (a loop's store, a borrow, a borrow with a result, a swap, a φ), and the gate on the corpus.

## 5. Decisions for hamza

G5, G6 and G7 (spec/j.md §0.3), built as proposed:
- **G5**, a concurrent assignment `a, b := b, a`, for the three merge sorts' two buffers swapping
  each pass; ADR 0052's R0, as written, refuses them;
- **G6**, `dt[s][1] := v`, the store into a record of a table of tuples (freq, tally);
- **G7**, one rule for every borrow: growth is the statement `strconv.AppendInt(b, k, 10)`, not
  ADR 0052's `b := f(b, …)`, under which B ⊸ B returns the buffer and B ⊸ B ⊗ R does not.

## 6. Not built

- ADR 0058, map buffers in program order: `m linear = map(cap)` stays threaded.
- `emit/linearity.go` reduced to what R0–R4 do not cover (ADR 0052's step 4).
- A loop that only reads a buffer written before it: S's residual keeps a `let` of the store where the
  threaded program's substitutes it into the loop (the test's case, removed and recorded; the gate
  shows no emission it changes).
- A message that quotes a residual names a version `#vout2`: the mark shows through.
