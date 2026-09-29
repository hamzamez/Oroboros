# A table's constructor is typed

2026-09-29. Found while retiring the portable layer ([portable-2026-09-29](portable-2026-09-29.md) §4),
and fixed the same day. The spec is [types.md §3.1](../../docs/spec/types.md).

## 1. The bug

The type checker gives a term the declared result of its primitive. The language's table and map
forms, `array`, `table`, `alloc`, `build`, `set`, `map`, `build-map`, `insert` and `keys`, declare
none. So each of them was typed **unknown**, and unknown agrees with everything. A signature was
therefore never checked against a table-valued body. On the native Go target, under
`(sig bad ((a slice-float64)) f64)`:

| body | before |
|---|---|
| `(array 1.0 2.0)` | **emitted** `func XBad(v0 []float64) []float64` |
| `(alloc (table 3 (fn (i) 1.0)))` | **emitted**, the same |
| `(build b 3 (set b 0 1.5))` | **emitted**, the same |
| `(build b 3 b)` | refused by the IR verifier: *"W9: %2's type array any is not final"* |
| `(table 3 (fn (i) 1.0))` | refused at lowering: *"a table rule that was never allocated"*, which is true whatever the claim |

The first three are wrong answers: an exported claim the program does not keep. A Go caller would be
stopped by Go's compiler. A JavaScript caller that expects a number gets an array.

**Why the IR did not catch it.** The IR types tables completely, by unification, and W5 checks every
yield against the function's declared results. But lowering sets a result to its declaration only
when the declaration is a table, because an integer's representation is chosen later. For every other
declaration the result is the body's own type, and the edge compares the yield with itself.
Overriding every result would change emitted signatures. And the IR checks a signature on a
non-exported definition only where the rung above the word is involved (`ir.CheckClaim`). So the
check belongs to the term checker, which is where types.md puts `sig`.

## 2. What it is

The checker's types gain two constructors, **Table(σ)**, spelled `array σ` or `buffer σ`, and
**Map(κ, σ)**, spelled `map κ σ`, and an **unknown element** `?`.
- Each table form's type is fixed by its constructor even where its element is not: a graph, a rule
  and `keys` are `array ?`, `alloc` and a scope's buffer are `buffer ?`, a map is `map ? ?`, and a
  store gives back its buffer's type.
- **A scope's value is frozen as it leaves** (ADR 0031): `buffer σ` becomes `array σ`, and a table
  demand on the scope is checked on the frozen value, at the exit.
- Demanded at a table type, a form takes the demand as its type, and each element it states is
  demanded at the element type: a graph's entries, a rule's body, and a store's value into a buffer
  whose element is known.
- **One rule joins the relation**, in `compatible`, the one place two types are compared. A type with
  an unknown element agrees with its own constructor, and with nothing else. For a table that
  includes a host type realizing one: Go's `slice-float64` is ρ(`array f64`), found by inverting ρ_T
  over a finite candidate set, as the IR's unifier does.

**Sound.** Every term whose value is a table is introduced by one of these forms or by a
declaration, so its type is Table(σ) for some σ, possibly `?`. The relation never equates a table
with a scalar.

**Loses nothing that was legal.**
- Where the element is unknown, the new type agrees with every table the old unknown agreed with.
- Where the element is known, the comparison is unchanged: invariant, so `[]int` is still not
  `[]byte`.
- `?` never reaches the IR. W5 reads this same relation over the IR's types, which name every element
  or say `any`, so the relation W5 reads is unchanged on every input it sees. That is the ADR 0033
  lesson: a relation relaxed for one check weakened the verifier twice.

## 3. After

| body, under `f64` | now |
|---|---|
| `(array 1.0 2.0)` | *"(array …) is a table, but f64 is required here"* |
| `(table 3 (fn (i) 1.0))` | *"(table …) is a table, …"*, the type reason, found first |
| `(alloc …)`, `(build b 3 b)`, `(build b 3 (set b 0 1.5))` | *"… is a buffer, …"* |
| `(build-map m 4 m)` | *"m is a map, …"* |

Accepted, as before: `(array 1.0 2.0)` and `(build b 3 b)` under `(array f64)`;
`(array 104 105 33)` under `(array (int 0 255))`; `(array 1.0 2.0)` under `slice-float64`; and
`(build-map m 4 m)` under `(map int f64)`.

## 4. Witnesses

| rule | planted fault | caught by |
|---|---|---|
| table forms are typed (the shipped bug) | every form untyped again | `TestATableFormIsNotAScalar`, `TestATableFormsEntriesAreDemanded` |
| the unknown element meets its constructor | agrees with nothing | `TestATableFormMeetsATableDemand` |
| …and only its constructor | a table agrees with any type; a map agrees with any type | `TestATableFormIsNotAScalar` |
| a host alias of a table is a table | aliases not inverted | `TestATableFormMeetsATableDemand` (`slice-float64`) |
| a demanded table's entries are demanded | the element not passed down | `TestATableFormsEntriesAreDemanded` |
| a store's value is demanded at a known element | not demanded | `TestATableFormsEntriesAreDemanded` |
| a scope's value is frozen | not frozen | `TestAScopesBufferIsFrozenAsItLeaves` |

## 5. What failed first

**The first version refused an acceptance program**, `encoding-hex`: *"enc is buffer int 0 255,
but array int 0 255 is required here"*. The corpus sweep, the IR step and the differential suite all
passed, byte-identical. The sweep compiles **exports**, and an acceptance program's `main` is not
one. That is the lesson noprop-2026-09-25 recorded, met again.

The cause was a missing law, not a wrong one. `hex.Encode` gives back `buffer (int 0 255)`, and the
scope took that as its value. But a scope's value is its body's value **with its buffers frozen**
(ADR 0031). So the rule now freezes at the exit. The first fix froze the value but still passed an
`(array σ)` demand into the body, where the live buffer met it before the freeze and failed; the test
written for the law caught that. A table demand is now checked at the exit. The spec's sentence
that a table and a buffer agree was also too broad: it holds for the unknown element only, and says
so now.

## 6. Gates

- **Emission is byte-identical**, refusal texts included, over all 492 source × target runs. No
  program the corpus holds changed outcome; the new refusals are of programs no one had written.
- **Every other step passes**: the compiler, IR (251 of 251), differential, and tooling steps, the last
  with the sixteen acceptance programs. The README's hex program prints `686921`.
- **Compile time** is 0.98–1.02×, within noise.
- Every plant in §4 is caught, re-run against the final code.

## 7. Not built

- **The eliminator.** Application of a table, `(a i)`, and `len` are still typed unknown, so an
  element is checked where it is demanded, not where it is read. A table's element is known to the
  checker only when a declaration states it.
- **Inference of an element from a store.** A scope's buffer stays `buffer ?` after
  `(set b 0 1.5)`; the IR knows it is `f64`.
