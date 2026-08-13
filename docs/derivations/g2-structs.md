# Derivation: gauntlet program 2 — centroid and bounding box over structs

> **⚠ Corrected by measurement, 2026-08-13.** The AoS penalty is real on **JS only** (2.86×).
> On Java, `Point[]` costs just 1.05× over parallel `double[]` — HotSpot's bump allocator lays
> the objects out contiguously. That shrinks §3's representation problem from two targets to
> one. Also: **Go performs SROA itself**, so §4's scalarization is needed for JS and Java, not
> Go. See [baseline C4, R3](../../gauntlet/results/baseline-2026-08-13.md).

Exploration only. No commitments, no ADR.

The question: does an array of structs stay flat, and does a loop that *looks* like it builds
one struct per iteration compile to scalar arithmetic with zero allocations?

**Result: it survives on Go, and forces the largest open decision found so far — `(slice T)`
for a struct `T` cannot have one representation across targets.** Also produces a defect class
neither earlier derivation hit, and a meta-observation that ties all the defects together.

---

## 1. The references, which diverge structurally

This is the first program where the three references are not the same program in three
syntaxes.

**Go** — array of structs, contiguous, zero allocations:

```go
type Point struct{ X, Y float64 }

func Centroid(ps []Point) Point {
	n := len(ps)
	accX, accY := 0.0, 0.0
	for i := 0; i < n; i++ { accX += ps[i].X; accY += ps[i].Y }
	return Point{accX / float64(n), accY / float64(n)}
}
```

**JavaScript** — an array of `{x, y}` objects is an array of *pointers to separately allocated
heap objects*. Fast hand-written JS does not do that; it uses parallel `Float64Array`s:

```js
function centroid(px, py) {
  const n = px.length;
  let accX = 0.0, accY = 0.0;
  for (let i = 0; i < n; i++) { accX += px[i]; accY += py[i]; }
  return { x: accX / n, y: accY / n };     // one allocation, at the boundary
}
```

**Java** — `Point[]` is an array of *references*; the JVM has no value types before Valhalla.
Fast hand-written Java also uses parallel `double[]`.

So two of three targets cannot represent an array of structs flatly at all.

## 2. The source

```lisp
(struct point ((x f64) (y f64)))
(struct bbox  ((lo point) (hi point)))

(fn centroid ((ps (slice point))) -> point
  (require (> (len ps) 0))
  (let n (len ps))
  (let s (fold-range (point 0.0 0.0) n
           (fn (acc i) (point (+ (.x acc) (.x (at ps i)))
                              (+ (.y acc) (.y (at ps i)))))))
  (point (/ (.x s) (int->f64 n)) (/ (.y s) (int->f64 n))))
```

The fold body constructs a `point` on **every iteration**. That is the boxing test: the source
says *n* allocations, the reference does zero.

## 3. The representation problem

`(slice point)` has no single answer — but the problem is **smaller than this derivation
assumed**. Measured, n=65536, array-of-structs against parallel arrays:

| Target | AoS | SoA | Penalty |
|---|---|---|---|
| Go | `[]Point`, contiguous | — | none; AoS is correct |
| JS | array of object pointers | `Float64Array` per field | **2.86×** |
| Java | `Point[]`, array of references | `double[]` per field | **1.05×** |

**Java does not fail the way this derivation predicted.** HotSpot's bump allocator lays freshly
built objects out contiguously, so iterating `Point[]` is near-sequential despite the
indirection. Only JS actually pays.

That halves the scope of the problem: one target of three needs a different representation, not
two.

A further caveat on the JS answer — `Float64Array` is not obviously right either. At n=65536 a
plain `Array` of doubles measured 1.6× *faster* (33,281 ns vs 53,522 ns), while at n=1024 the
two were equal. Unexplained; V8's `PACKED_DOUBLE_ELEMENTS` may get better locality than an
`ArrayBuffer`. **The representation choice needs measuring per target and per size**, which is
[ADR 0008](../decisions/0008-measurement-over-principle.md).

Three options, none free:

- **(a) Target-chosen representation.** `(slice point)` is a Tier 1 capability; Go picks AoS,
  JS and Java pick struct-of-arrays. Portable and fast everywhere. **Cost:** the emitted
  signature changes arity across targets — Go takes one `[]Point`, JS takes two
  `Float64Array`s. [ADR 0001](../decisions/0001-parasite-model.md)'s binding story assumed
  signatures map across targets. They do not.
- **(b) Always AoS.** Signatures stay stable; JS and Java lose badly and fail the gauntlet.
- **(c) Layout in the type** — explicit `(aos point)` vs `(soa point)`. Honest and predictable,
  in the spirit of [ADR 0003](../decisions/0003-range-typed-integers.md). **Cost:** `(aos point)`
  is simply slow on JS, so portable code cannot use it, and the programmer now carries a
  decision they may not be able to make portably.

Leaning: **(a) as the portable default, (c) available for interop**, matching the Tier 1 /
Tier 2 split. Not a commitment — this is the largest open question the derivations have
produced.

## 4. Value semantics makes scalarization unconditional

If `(at ps i)` may return a *reference* into the array, then under SoA there is nothing to
point at — the point does not exist contiguously anywhere. So option (a) forces:

> **Structs are values. Always copied, never referenced. No interior pointers.**

That decision pays for itself immediately. Scalar replacement of aggregates normally requires
escape and alias analysis. With no interior pointers, **nothing can alias a struct local**, so
the analysis is discharged by the type system rather than computed. SROA becomes unconditional.

The rules are ordinary lowering rules, `structs` layer → `core`, hence layer-decreasing and
terminating for free:

```lisp
(layer structs
  ;; field-of-constructor: purely local, needs no analysis at all
  (rule (.x (point ?a ?b)) => ?a)
  (rule (.y (point ?a ?b)) => ?b)

  ;; a struct local splits into per-field locals
  (rule (var ?v point (point ?a ?b))
     => (seq (var ?v#x f64 ?a) (var ?v#y f64 ?b)))
  (rule (set ?v (point ?a ?b)) => (par (set ?v#x ?a) (set ?v#y ?b)))
  (rule (.x ?v) => ?v#x))
```

Escape via `return` is handled by **reconstructing at the boundary** — `return acc` becomes
`return (point acc#x acc#y)`. And since non-recursive functions are rules
([g3](g3-generics.md)), inlining removes most boundaries, so the reconstruction usually
disappears too.

## 5. Derivation

**Step 1.** `fold-range` lowers exactly as in [g1](g1-dot-product.md):

```lisp
(block
  (var acc point (point 0.0 0.0))
  (var i (int 0 n) 0)
  (loop (when (= i n) (break))
        (set acc (point (+ (.x acc) (.x (at ps i)))
                        (+ (.y acc) (.y (at ps i)))))
        (set i (+ i 1)))
  acc)
```

**Step 2.** Split the local, and fold field-of-constructor:

```lisp
(block
  (var acc#x f64 0.0)
  (var acc#y f64 0.0)
  (var i (int 0 n) 0)
  (loop (when (= i n) (break))
        (par (set acc#x (+ acc#x (.x (at ps i))))
             (set acc#y (+ acc#y (.y (at ps i)))))
        (set i (+ i 1)))
  (point acc#x acc#y))
```

**Step 3.** `(.x (at ps i))` lowers per representation — `ps[i].X` on Go, `px[i]` on a SoA
target. The trailing `(point acc#x acc#y)` is the boundary reconstruction.

## 6. New defect — struct assignment is *parallel* assignment

Step 2 wrote `par`, not `seq`, and that matters. Splitting

```lisp
(set acc (point (.y acc) (.x acc)))          ; a swap
```

sequentially gives `acc#x = acc#y; acc#y = acc#x` — wrong. The original assignment was
*simultaneous*; sequencing the pieces destroys that.

The fix is textbook: when any field read on the right-hand side names the variable being
assigned, bind temporaries first. Detectable syntactically, no analysis required.

## 7. The meta-observation

Every defect across all four derivations is the same shape:

| Derivation | Property the original term had | Lost by | Classical fix |
|---|---|---|---|
| g4 | **Sharing** — a subterm evaluated once | Naive substitution | Let-binding / graph reduction |
| g1, g3 | **Capture-freedom** — binders are distinct | Rule-introduced binders | Hygiene, fresh names |
| g2 | **Simultaneity** — fields assigned at once | Splitting into a sequence | Temporaries |

> **Naive term rewriting loses properties the original term held implicitly.**

That is a bounded, well-studied failure family — every entry has a textbook solution predating
this project by decades. It is a much better situation than an open-ended risk, and it is a
genuine argument in candidate B's favour: the hazards are known in advance.

## 8. Aggregate return, and float comparison

**Return cost is per-target and matches each reference.** Go returns `Point` by value: zero
allocations. JS and Java must box the result: one allocation, exactly as hand-written JS and
Java do. Parity holds against each target's own ceiling, which is
[ADR 0004](../decisions/0004-first-targets.md) behaving as intended. When the caller is in the
same compilation and `bounds` inlines, the aggregate never materializes at all — *better* than
hand-written JS.

**The `bbox` case tests recursion in SROA**: `bbox` → two `point`s → four `f64`s. The rules
above apply structurally and reach fixpoint at scalars, still layer-decreasing.

**`min`/`max` on floats need a specified NaN rule.** `(min a b)` and `if a < b` disagree when
an operand is NaN, and Go's builtin `min`, `math.Min`, JS's `Math.min`, and Java's `Math.min`
do not all agree either. This is the same class as `sum`'s associativity in
[g1 §8](g1-dot-product.md): **float operations need explicit ordering and NaN semantics in
their Tier 1 specification**, or the same program gives different answers per target.

## 9. Findings

1. **`(slice T)` for struct `T` cannot have one representation across targets** — but the
   penalty is **JS-only** (2.86×). Java's `Point[]` costs 1.05×, so only one target of three
   needs a different representation. Still the largest open decision the derivations produced,
   at two-thirds the size originally stated.
2. **A Tier 1 capability can change a function's arity across targets.** ADR 0001's binding
   story assumed signatures map. They do not.
3. **Value semantics with no interior pointers makes SROA unconditional** — the alias analysis
   is discharged by the type system instead of computed. This is what makes option (a) viable.
4. **Struct assignment is parallel assignment** and needs the standard temporaries discipline.
5. **All defects so far are one family:** naive rewriting loses sharing, capture-freedom, or
   simultaneity. Bounded, and each has a textbook fix.
6. **Float `min`/`max` need specified NaN semantics**, echoing g1's associativity finding.
7. **Aggregate return costs one allocation on JS and Java, zero on Go** — parity per target,
   and inlining can beat hand-written JS.

## 10. Verdict

Boxing did not hide here. The loop that reads as *n* allocations compiles to two float
accumulators, and the mechanism is ordinary layer-decreasing rules rather than a new pass.

**Measurement added a wrinkle worth keeping:** Go performs this scalarization *itself*.
`CentroidStructAcc` — written exactly as the Oroboros source reads before SROA, constructing a
`Point` every iteration — matches the hand-scalarized version at zero allocations. So §4's SROA
rules are needed for JS and Java, and are redundant on Go. That is the Parasite model applying
to our own passes: **do not implement what the host already does.**

Program 2 is still the first derivation where the three targets want structurally different
source programs. That crack is real but narrower than it first looked — JS alone pays the AoS
penalty — and [g5 §1](g5-bindings.md) narrows it further: differing *interior* code is fine,
and only the boundary is a semantic contract.

**Machinery list, now complete across four derivations:**

> auto let-binding · layer stratification · linearity analysis · hygiene · range analysis with
> `require` facts · deforestation measure check · ANF normalization · monomorphization for
> recursive generics · polymorphic type checking · SROA with parallel-assignment temporaries

**Still untested: escaping closures.** All four derivations had every function argument literal
at its call site, so no closure ever formed. A closure that is returned or stored is the one
remaining place the "closures are not a core primitive" constraint is assumed rather than
demonstrated.
