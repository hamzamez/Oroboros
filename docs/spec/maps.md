# Maps

**Status: specification. Not built.**

A map is a table whose index set is a finite subset of the key type. Everything here follows from
that sentence and from three decisions taken before it was written:

- **F2** — `(m k)` has type `(option V)`, from [growth.md §2.1](../growth.md);
- **no iteration order**, an explicit non-guarantee rather than silence;
- **a map is a value or a buffer on exactly the same terms as an array**, derived rather than chosen
  in [arrays-revisited.md §6](../arrays-revisited.md).

This document answers [CLAUDE.md](../../CLAUDE.md)'s three questions for a language addition: what it
means independently of any target, what each target does with it, and whether any disagreement is
observable.

---

## 1. The algebra

[tables.md](tables.md) says a table is a dependent product over a finite index set:

```
    T  =  Π_{i ∈ I} V
```

`(fn (A) B)` has `I = A`. `(array V)` has `I = Fin n`. **`(map K V)` has `I = S`, a finite subset of
`K`.** Three points on one scale, and reading is application at every point.

### 1.1 The map is the first construct whose domain condition nothing can discharge

growth.md §2 reads that scale as three answers to one question:

| | domain | the condition | who discharges it |
|---|---|---|---|
| `(fn (A) B)` | all of `A` | trivial | the **type**, statically |
| `(array V)` | `[0, len)` | `0 ≤ i < len a` | the **refinement layer**, in QF-LIA |
| `(map K V)` | the keys present | `k ∈ dom m` | **nobody, at compile time** |

`dom m` is a function of the program's input. For an exported definition that quantifies over all
inputs, and it is not a statement in linear integer arithmetic at all — it is **set membership**. No
relational domain helps; the gap keeps not being a linear one.

**So a map read must be fallible, and that is the first LANGUAGE-INTERNAL argument for sums.**
[sums-research.md](../sums-research.md) justified sums from errors, Win32 contracts and dispatch, all
of them outside the language. Go says the same thing with comma-ok.

---

## 2. The type

```
(map K V)
```

**`K` must be a type on which the language's `=` is defined, and today that is exactly `int`.**

This is not a staging convenience, which is how growth.md §5 framed it. `=` is the language's
equality and it is **integer equality only** ([match.md](match.md)): floats are excluded because NaN
is not an equivalence relation, strings because no two of the four targets agree on comparing them
([strings.md](strings.md)). A map's domain condition is `k ∈ dom m`, which is decided by equality on
`K`, so:

> **`(map K V)` is well-formed exactly where `=` is defined. The language's own equality already
> refuses the key types a map cannot have.**

`(map string V)` becomes well-formed when strings do, and needs no new decision here.

`V` is any type, including a range: `(map int (int 0 65535))` declares the **value range**, through
the mechanism [elemwidth](../../gauntlet/results/elemwidth-2026-08-27.md) built for
`(array (int 0 255))`. That is `wordcount`'s last unproven operation, and it is free.

---

## 3. Constructors

### 3.1 The literal — a graph

```
(map (1 10) (2 20) (5 50))
```

`(array e…)` is a table given by its **graph** with the index set implicit. A map's index set is not
implicit, so the graph carries both columns.

This overloads `map` between type and term exactly as `array` is already overloaded — `(array V)` a
type, `(array e…)` a term — and is disambiguated the same way: **a type has bare type names, a
literal has parenthesised pairs.** There is no collision with `js.map` or any host `map`, because
`tg.Prims` is keyed by the **qualified** name, which is the reason match.md gives for `=` being safe.

**Duplicate keys are an error**, by match.md's precedent for a repeated name in one clause. A graph
is a function; two rows for one key is not one.

### 3.2 There is no rule form, and the reason is the algebra

`(table n f)` exists because a table given by a rule needs **no memory** — the index set is `Fin n`
and `n` is a number. A map's index set is the point of the map, so a "rule" form would still have to
store the key set, and `(map-of keys f)` is then just a `build-map` over `keys`. It buys nothing and
is refused.

### 3.3 The buffer — `build-map`

```
(build-map cap (fn (m) body))     ; a mutable map, scoped. Returns an immutable map.
(insert m k v)                    ; consumes m, returns m
```

Identical in discipline to `(build n (fn (b) …))` and for the reason arrays-revisited.md §6 derives:
**the discipline is about aliasing, and aliasing does not care what the index set is.** So:

- a **frozen map** is an immutable value; `(map K V)` reads are **pure**;
- a **map buffer** is linear and scoped; its reads are **impure**;
- the linearity check is `occurrences` on the residual, not a type;
- the freeze copies nothing.

`wordcount` already has this shape on every target: it threads the map through the loop as a loop
variable and returns it.

### 3.4 `insert` extends the index set by a POINT, and that is a real asymmetry

growth.md §1.1:

```
    append :  Buf V → V → Buf V           dom' = Fin (n+1)      len' = len + 1
    insert :  Map K V → K → V → Map K V   dom' = dom ∪ {k}      |dom'| ∈ {|dom|, |dom|+1}
```

Append keeps an **equation** — `len b = len b₀ + i`, provable by induction on iterations and inside
the existing linear fragment. Insert keeps only an **interval**: whether the key was already present
is a fact about the input.

**`remove` is deferred**, with a named trigger. No program in this repository removes from a map, and
it costs something specific: with removal `|dom m| ∈ [0, k]` instead of `[min(1,k), k]`, so the lower
bound is gone. It should arrive with the program that needs it.

---

## 4. Operations

```
(m k)          →  (option V)      indexing is APPLICATION
(insert m k v) →  the buffer      only inside build-map
(len m)        →  |dom m|
```

That is the whole surface. `(has m k)` is not a primitive because it is
`(case (m k) (some _) true (none) false)`, and a name that adds nothing should not exist.

### 4.1 `(m k) : (option V)` does not break "indexing is application"

It is still application. What changes is that **the result type is decided by the domain kind**, and
that is where the difference between the three points on §1.1's scale *should* show: each pays for its
domain condition in its own coin — the function's is free, the array's is a proof, the map's is a
**value**.

---

## 5. Reduction

### 5.1 β-tab, a second clause of β

[tables.md](tables.md) makes `((array 1 2 3) 1) → 2` a clause of β rather than a fourth rule: a rule
is an intensional presentation of a function and a graph an extensional one. The map clause is the
same, with the sum in the result position:

```
    ((map … (kᵢ vᵢ) …) kᵢ)  →  (some vᵢ)          kᵢ literally present
    ((map … ) k)            →  (none)             k literally absent
```

Both premises require the key and every literal key to be integer literals, where equality is
decidable by inspection. Otherwise the term is stuck and reaches the boundary.

### 5.2 A static map leaves NOTHING

This is the two-level language, and it is why F2 is affordable:

```
(case ((map (1 10) (2 20)) 1) (some v) v (none) 0)
→ (case (some 10) (some v) v (none) 0)         β-tab
→ 10                                            the sum's own reduction (sums.md)
```

No map, no tag, no allocation, no dispatch. Free where it is used to **think**.

It also gives [closures-direction.md](../closures-direction.md) the static-level map library it says
the language should have, for no new machinery: a compile-time keyword table, a memo table, or a
lookup written as data all reduce away.

### 5.3 A dynamic map read IS the host's own fallible read

```
(case (m k) (some v) A (none) B)
```

The map survives to the boundary and the `case` is eliminated against the host's own two-valued read:

```go
if v, ok := m[k]; ok { A } else { B }
```

So **F2's option is not a thing we add, it is a thing the host already has and we were discarding.**
sums.md measured a locally-consumed sum at **1.00× against hand-written Go with 0 allocs**, and this
is that shape.

**What is unmeasured is an option that ESCAPES** — stored into a table, returned across a boundary —
where a real representation is needed and the niche encoding sums.md left open becomes the question.
That is named here and not answered.

---

## 6. Capacity, and why all four targets agree because of it

```
(build-map cap (fn (m) …))
```

**A map buffer has a declared capacity.** Inserting beyond it is a condition the program must handle,
exactly as `tree.oro`'s node table is capped at 512 and the program says what happens when a document
is deeper.

This is the load-bearing decision in the document and it deserves its argument.

**The reason is windows.** Three hosts bring a hash map that grows unboundedly; windows brings none
and CLAUDE.md's rule is explicit — *a target does not get to decline a language construct*. So a map
means writing a hash table for windows, **in Oroboros** (§8.4). A growing hash table rebuilds into a
larger allocation, and **that is not expressible**: `build` fixes its length up front, and a nested
`build` returns a frozen `array`, not a buffer that could replace the outer one. Growth of the
*backing store* is precisely the unsolved question growth.md exists to name.

Three ways out, and the third is chosen:

- **(a) Let the three hosts grow and windows not.** Refused: the disagreement is observable — the same
  program succeeds on three targets and fails on one — which is a Tier 2 construct in the core, the
  thing [ADR 0001](../decisions/0001-parasite-model.md) exists to refuse.
- **(b) Solve growable buffers first.** Honest, and it makes the map wait on the larger question it
  was supposed to be independent of. growth.md's finding is that the map is primary *because* a
  growable array has a workaround; making the map depend on growth inverts that.
- **(c) Give every map a declared capacity.** Chosen. All four targets agree exactly, no growable
  buffer is needed, and the failure is **visible** rather than latent.

The precedent is this repository's own and it is celebrated rather than tolerated: the JSON tokeniser's
`(set stk sp …)` carries `sp < cap` and nothing can discharge it, so **the compiler makes the parser
say what happens when the nesting is deeper than the stack** — and a recursive-descent parser has the
same limit, the C stack, and is never asked. A capacity does not create the limit; it makes it visible.
It is also Low\*'s lesson: *the restriction is the mechanism.*

And a capacity is a **declaration**, which is this project's answer to ADR 0003, ADR 0012 and
ADR 0019 alike.

> **Trigger to revisit:** a program whose distinct-key count genuinely cannot be bounded before the
> loop runs. Reading input to EOF is the named candidate, and it is the same candidate growth.md names
> for the growable array — which is evidence the two should be reopened together.

A **literal** map's capacity is its size, so §3.1 needs nothing.

---

## 7. Iteration has NO ORDER, and that is a declaration

The four hosts disagree, and the disagreement is **observable**:

| | order |
|---|---|
| Go | **randomised on purpose**, per iteration |
| JavaScript | **specified**: integer-like keys ascending, then string keys by insertion |
| Java | `HashMap` unspecified; deterministic in practice, not by contract |
| windows | whatever we write |

By CLAUDE.md's third question this is Tier 2 territory. But an *unordered* iteration is not Tier 2 —
it is a Tier 1 construct with a **weaker guarantee**, and stating the non-guarantee is what
`split-words` failed to do for two months.

### 7.1 Why the surface is a fold over a declared commutative monoid

An unordered iteration is a fold over a **multiset**, and a fold over a multiset is well-defined
exactly when the step functions commute: currying `f : A → V → A` to `V → (A → A)`, order-independence
for all inputs is `f v₁ ∘ f v₂ = f v₂ ∘ f v₁`.

**We cannot check that.** It is program equivalence, which [decidability-map.md](../decidability-map.md)
lists as one of the four things explicitly given up as undecidable.

So the choice is trust, enumerate, or refuse — and trust is `split-words` again, a silent wrong answer
that differs per target. **Enumerate:**

```
(fold-map m op z)      ; op drawn from a fixed set of commutative monoids
```

`+`, `*`, `max`, `min`, `and`, `or` — commutative by construction, not by claim, because the compiler
knows which operator it was given.

### 7.2 A commutative operator is NECESSARY AND NOT SUFFICIENT, and this is the trap

`max` over the **values** of a map is a function of the multiset. `argmax` — *which key holds the
maximum* — is **not**, because ties are broken by whichever key was visited first. So a fold whose
result mentions the **key** is order-dependent even when its operator is commutative.

This is worth stating loudly because it is the shape a `wordcount` reporter naturally has (*the most
frequent word*) and it would produce different answers on Go and JavaScript while passing every test
that used a corpus without ties.

The fix, when it is wanted, is a **total order on keys** as a tie-break — which `int` has and which is
therefore available for `(map int V)` — and it is deliberately not in the first cut.

### 7.3 The first cut refuses `keys` and general iteration

`(len m)` and `(fold-map m op z)`. No `keys`, no `entries`, no general fold.

The cost of that is smaller than it looks: **`wordcount` builds a map and returns it, and never
iterates it** — on all four of the targets it is written for. The first cut is enough for a word
count, a memo table, a sparse array, a graph's adjacency, and the interning table a string map would
itself need.

---

## 8. What each target does

### 8.1 Go

```go
m := make(map[int]int, cap)     // (build-map cap …)
m[k] = v                        // (insert m k v)
v, ok := m[k]                   // (m k) → the option, eliminated in place
len(m)                          // (len m)
```

`m[k]++` is one `mapassign` where `m[k] = m[k] + 1` is a `mapaccess` **and** a `mapassign`, measured
at 1.21× in the first baseline. That is a fusion of `insert` with a read and belongs in the target
file as a declared idiom, not in the language.

### 8.2 JavaScript — a plain object, not `Map`

[maps-2026-08-30](../../gauntlet/results/maps-2026-08-30.md), re-taken:

| | |
|---|---|
| `Map` vs object, string keys | object wins **1.56×** (the old 3.25× did not reproduce) |
| `Map` vs object, **integer keys** | object wins **3.67×** |
| plain `{}` vs `Object.create(null)` | **plain `{}` wins**, by 3% |

So `(map K V)` lowers to a **plain object**, and `(map int V)` — the case being built first — is where
the margin is largest, because V8 stores integer-like keys in the *elements* backing store rather than
a hash.

**One per-target cost, named rather than hidden:** `Object.keys(m).length` is **O(n)** where `len` is
O(1) on Go and Java. The answer is identical, so it is not a Tier 2 disagreement — it is a performance
difference, and the fix is to carry a count beside the object. That is a representation question and
it is left open here.

### 8.3 Java

```java
java.util.HashMap<Long,Long> m = new java.util.HashMap<>(cap);
m.put(k, v);
Long v = m.get(k);   // null means absent — ONE lookup, unlike containsKey+get
m.size();
```

`get` returning a boxed `Long` makes the fallible read one hash rather than two, and the boxing is
already there because `HashMap` cannot hold primitives.

**`merge` is preferred over `getOrDefault`+`put`**, reversing baseline R5. Measured three times now:
R5 said 2.59× slower, native-java-2026-08-25 said 1.19× faster, maps-2026-08-30 said **1.22× faster**.
`targets/java/util.oro` already declares **both** and the program picks; a number that has failed to
reproduce twice should not decide anything on its own.

### 8.4 windows — we write it, in Oroboros

Open addressing with linear probing, over a `build-map` of the declared capacity: one buffer of keys,
one of values, one bit of occupancy — or a key sentinel, since `build` zero-fills and 0 is therefore a
distinguished key value that must be handled explicitly.

**It should be written in Oroboros and not in `emit/`.** Adding a case to `emit/*.go` for a host
function is the single most common way to get this architecture wrong, and a hash table is a library.
It is also the first library the language writes for itself, which is a real test of
[general-purpose.md](../general-purpose.md)'s claim.

The capacity decision of §6 is what makes this writable at all.

---

## 9. What it costs the analyses

| | effect |
|---|---|
| **Linearity** (ADR 0018) | unchanged — `insert` consumes and returns, exactly like `set` |
| **Refinement** | **nothing to discharge** — there is no positional index. The domain condition moves into the type, which is what F2 *is* |
| **Value range** | `(map int (int 0 65535))` through elemwidth's mechanism — `wordcount`'s last unproven operation, free |
| **Intervals** | `(len m)` is an interval `[min(1,k), k]` after k inserts, not an equation — §3.4 |
| **Termination** | a loop over a growing map is a worklist; `tree.oro`'s explicit trip bound is the precedent, and a declared capacity is a bound |

The refinement row is the interesting one: **F2 does not add an obligation, it removes one.** An
array's `0 ≤ i < len` is discharged in QF-LIA; a map's `k ∈ dom m` is discharged by the program,
because the option makes it say what happens when the key is absent.

---

## 10. Refused, with reasons

| | why |
|---|---|
| `(map float V)`, `(map string V)` | `=` is not defined on them — §2. `(map string V)` unblocks when strings do |
| a rule form, `(map-of keys f)` | buys nothing over `build-map` — §3.2 |
| `remove` | no program needs it, and it weakens `|dom m|`'s lower bound — §3.4 |
| unbounded growth | not expressible for windows, and letting three hosts grow is an observable disagreement — §6 |
| `keys`, general iteration | no order exists and commutativity is undecidable — §7 |
| `(has m k)` | is `(case (m k) …)`; a name that adds nothing should not exist |

---

## 11. Open

1. **The escaping option's representation.** §5.3 — free when consumed locally, unmeasured when
   stored or returned. The niche encoding sums.md named is the candidate.
2. **JavaScript's `len`.** O(n) against O(1) — §8.2. Same answer, different cost; carrying a count is
   the obvious fix and is a representation decision.
3. **A total order on keys as a tie-break**, which would make `argmax`-shaped folds well-defined —
   §7.2. Available for `int`; wanted only when a program needs it.
4. **`build-map`'s name.** It is consistent with `build` and it is clumsy. `build` cannot simply be
   overloaded, because its length argument is what fixes the index set.
5. **Our hash table against the host's**, on Go. This is the parasite test and it needs the map to
   exist first: if ours came close, that would be evidence the rule is being applied to something the
   hosts are not actually good at.
