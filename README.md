# Oroboros

A small language and build system that **parasitizes** target ecosystems instead of abstracting
over them.

> **Status: 0.0 — work in progress, not a release.**
> The compiler works and is measured, but the core is *deliberately unspecified* and candidates
> are expected to die. Nothing here is frozen, nothing is versioned, and the surface syntax
> changes when a measurement says it should. See [How this is run](#how-this-is-run).

---

## The idea

Most portable languages define a core that is the *intersection* of every host, then treat
anything platform-specific as an escape hatch. That guarantees portability and gives up the
ecosystem — and, historically, gives up performance too.

Oroboros inverts it. If the best way to build a Windows app is Win32, add a Win32 target and use
Win32 fully. If it is .NET, add a .NET target and use .NET fully. Android might be best served by
Kotlin, or the JVM, or the NDK — so have all three as separate targets and pick per program.

Portability is therefore a **property a program may or may not have**, computed and reported by the
compiler, rather than a guarantee the language enforces globally. A program that uses only portable
capabilities is portable. A program that uses Win32 is not, and that is a first-class thing to
write.

**This is meant to be general-purpose**: apps on Windows and Android, websites in the browser,
backends in the cloud. The four targets were application platforms all along — see
[general-purpose.md](docs/general-purpose.md).

## Where this actually stands

A working compiler: a β/δ reducer with call-by-need and an effect discipline, **four backends**,
and **all seven gauntlet programs at parity with hand-written code on two native targets** — two
of them producing byte-identical machine code on Go.

| | |
|---|---|
| `core/` | reader, terms, β/δ reducer — [the atom](docs/the-atom.md) |
| `emit/` | **Go, JavaScript, Java, x86-64**. Types, refinements and termination live here, *not* in the language |
| `targets/` | target declarations — **data, not Go** |
| `cmd/oro` | reduce a file to normal form against a target |
| `cmd/gen` | emit a file into the gauntlet |
| `cmd/build` | follow imports, reduce `main`, emit, run the host toolchain |
| `examples/` | 47 programs |
| `gauntlet/` | hand-written references and 32 recorded measurements — **the bar** |

```bash
go run ./cmd/oro   -target=go examples/dot.oro          # reduce to normal form
go run ./cmd/build -target=portable-go -o hello examples/hello.oro
go test ./core/ ./emit/
```

### The whole language

| | |
|---|---|
| term kinds | **7** |
| top-level forms | **6** — `def`, `sig`, `sum`, and three for modules |
| reduction rules | **4** — β with call-by-need, δ, evaluation on literals, one commuting conversion |
| parameters | **2** — which names are primitive, and which of those are pure |

Everything else is sugar that erases before reduction: `let`, `seq`, `and`/`or`/`not`/`cond`,
`loop`, `values`, `match` and `case`. A `sum` declaration is not even a concept downstream — it
generates ordinary `def`s, so the reducer, the module system and every backend are unchanged by it.

The rule count was **3** until sums landed on 2026-08-22 and it went up honestly rather than by
relabelling: `=` now folds on two integer literals, and an eliminator is pushed through `if` and
`let`. Both exist because a sum should cost nothing at *either* level, and the second was
[measured across 184 residuals](docs/spec/sums.md) to change no existing program before it shipped.

Read off the code rather than from memory: [state.md](docs/spec/state.md).

### The gauntlet

Seven programs that must reach parity with hand-written code. They are the one fixed commitment.

Numbers are *emitted ÷ hand-written*, so **lower is better and ~1.00× is the bar**. The two
current full runs are on the **native** targets — [Go](gauntlet/results/native-gauntlet-2026-08-20.md)
and [JavaScript](gauntlet/results/native-js-2026-08-20.md):

| | Go | JavaScript |
|---|---|---|
| dot product | 1.00× | 1.03× |
| search | 0.97× early / 1.00× late | 1.21× early / 1.00× late |
| centroid (structs) | 1.01× | 0.97× |
| word count | 0.997× | 0.89× |
| generics | 0.98× | 0.98× |
| formatted output | correct — its pass condition is not a number | correct |
| stencil (aliasing) | 0.93× allocating / 0.999× reusing | 0.94× / 0.97× |

All but one are inside the ~15% noise floor, which is the claim: **at parity, not faster**. The
exception is JavaScript's early-exit search — 7.5 ns against 6.2 ns on a single call returning at
index 6, so the 1.3 ns gap is call overhead at the timer's resolution floor. It is recorded as a
loss rather than argued away.
Separately, on the *earlier portable layer*, centroid and generics compiled to **byte-identical
machine code** against hand-written Go
([structs](gauntlet/results/structs-2026-08-14.md), [generics](gauntlet/results/generics-2026-08-14.md)) —
a stronger result than a timing, and the reason those two programs exist.

Three caveats stated rather than buried:

- **Java has moved to the native target** —
  [native-java-2026-08-25](gauntlet/results/native-java-2026-08-25.md). All seven programs, at
  parity, on JDK 17. Two things came out of it: the migration **refuted** the measurement that made
  Java the interesting case (the fused `merge` was recorded 2.6× slower than unfused and is
  **1.19× faster**), and it put a price on ADR 0012 — our `int` maps to Java's `long`, so a loop
  counter was 64-bit and every array access carried a cast, worth **1.04×–1.54×** depending on the
  loop. That is [fixed](gauntlet/results/indextype-2026-08-25.md): a counter the compiler can bound
  by a length is emitted as the host's own `int`, and every program is at parity with idiomatic
  hand-written Java.
- **The language's table works on all four targets** —
  [wintables-2026-08-25](gauntlet/results/wintables-2026-08-25.md) — and measured the price of the
  uniform element on the one host with no types of its own: **3× on a boolean sieve**, eight bytes
  per element against one. Not the compiler; the element size is not part of the type.
- **x86-64/Windows runs one program, not the gauntlet.** A 200,000-element sieve, at **0.97×
  median** against hand-written assembly ([windows-2026-08-19](gauntlet/results/windows-2026-08-19.md)) —
  and the hand-written reference is written the way a person writes it, not in the emitter's shape.
  That is what [ADR 0016](docs/decisions/0016-targets-need-not-have-expressions.md) rests on.
- **The stencil's allocating form is [ADR 0013](docs/decisions/0013-accept-the-allocation-price.md)'s
  open price.** It reaches parity here because the *native* target can express buffer reuse
  (`go.set-float64` is Go's own store); the retired portable layer could not, and paid 1.79×. The
  price is the **shape**, not the compiler — and it is expected to be paid off, not kept.

## What it looks like

`examples/native/dot-go.oro`. A vector is a length paired with an index function — a **library
written in the language**, not a primitive, and it has no runtime existence:

```lisp
(use go)
(export dot)

(sig dot ((p slice-float64) (q slice-float64)) f64
  (where (and (go.== (go.len p) (go.len q))
              (go.< (go.len p) 65536))))

(def vec      (fn (n f) (fn (sel) (sel n f))))
(def vlen     (fn (v)   (v (fn (n f) n))))
(def vindex   (fn (v i) ((v (fn (n f) f)) i)))
(def of-array (fn (a)   (vec (go.len a) (fn (i) (go.at-float64 a i)))))
(def zip      (fn (g a b) (vec (vlen a) (fn (i) (g (vindex a i) (vindex b i))))))

(def sum (fn (v)
  (loop ((acc 0.0) (i 0))
    (go.>= i (vlen v))  acc
    else                (again (go.f+ acc (vindex v i)) (go.+ i 1)))))

(def dot (fn (a b) (sum (zip go.f* (of-array a) (of-array b)))))
```

```bash
go run ./cmd/gen examples/native/dot-go.oro go dot.go
```

```go
func GenDot(a []float64, b []float64) float64 {
	acc := 0.0
	var i int = 0
	var n1 int = (len(a))
	b = b[:n1]
	for {
		if (i >= (len(a))) {
			break
		}
		acc, i = (acc + ((a[i]) * (b[i]))), (i + 1)
		continue
	}
	return acc
}
```

Every abstraction is gone: no closure, no intermediate vector, no allocation. The `b = b[:n1]` is
the emitter *shaping* the output so Go's own compiler re-proves the bound and drops the second
bounds check — because [our proofs do not transfer](gauntlet/results/bce-2026-08-15.md), so a proof
is only worth what the emitted shape can cash in.

The `(where …)` is not decoration. The equality is what makes indexing `q` under `p`'s length
provable at all; the bound is what proves the loop terminates.

### And the same idea, twice more

`match` is `loop`, so a state machine is clauses plus a jump — [spec](docs/spec/match.md):

```lisp
(def runs (fn (n)
  (match (0 n 0)
    _ 0 c                            c
    0 v c (when (= (go.% v 2) 1))    (again 1 (go./ v 2) (go.+ c 1))
    _ v c (when (= (go.% v 2) 1))    (again 1 (go./ v 2) c)
    _ v c                            (again 0 (go./ v 2) c)
    else                             0)))
```

A sum is Σ, so its value is a tag and a payload — and Go's own `(T, error)` idiom is already that
shape ([spec](docs/spec/sums.md)):

```lisp
(sum result (ok int) (err int))
(sig div ((a int) (b int)) result)
(def div (fn (a b) (if (= b 0) (err 0) (ok (go./ a b)))))
```

```go
func GenDiv(a int, b int) (int, int) {
	if (b == 0) {
		return 1, 0
	}
	return 0, (a / b)
}
```

Both are **reader sugar**: zero reduction rules, zero term kinds, no backend change, and no target
declares any of it.

## How it works

Everything runs on one mechanism: a **capability graph**.

- A **capability** is a named, typed unit of functionality — `float64`, `map`, `threads`.
- A **module** declares what it requires.
- A **target** declares what it provides natively, plus **shims** implementing one capability in
  terms of others.

Building covers the required set from what the target provides plus what its shims reach. Anything
uncovered is a build error naming the exact gap.

The rule that makes this fast: **emit at the highest layer the target natively provides.** Lower
only as far as necessary. Go has `map`, so emission stops there. C does not, so the same source
keeps lowering.

**But that rule is a prior, not a proof.** Which host construct is actually fastest is a
measurement. The first baseline run refuted four inferences from it at once: JS's `Map` is 3.25×
*slower* than a null-prototype object; Java's fused `merge` loses 2.6× to unfused
`getOrDefault`+`put` (**re-measured 2026-08-25: it does not reproduce**, and the fused form is now
1.19× faster); Java's `Point[]` costs 1.05× where JS's array-of-objects costs 2.86×; and all
three hosts inline a literal callback. See
[ADR 0008](docs/decisions/0008-measurement-over-principle.md).

### What the core turned out to be

**Lambda calculus in which the normal form is a parameter.** A target supplies a partition of names
into primitive and defined; reduction runs until only primitives remain.

> Everything is a function, evaluated at compile time. What survives is what the target must do at
> runtime, and the compiler tells you exactly what that is.

Lambda calculus *at runtime* allocates, because a closure's environment must outlive the
abstraction. The same substitution *at compile time* costs nothing, because it is gone before the
program runs. That makes this a **two-level language**: the static level is unrestricted
higher-order; the dynamic level is first-order tables and loops. A closure may not survive staging
— [closures-direction.md](docs/closures-direction.md),
[callbacks.md](docs/spec/callbacks.md).

Layers, both directions of the capability graph, and staging all collapse into that:
[the-atom.md](docs/the-atom.md).

### Targets are data

Adding a host function means adding a line to a target file. No Go, no rebuild:

```lisp
(prim sqrt (f64) f64 expr "math.Sqrt(%s)" (import "math"))
```

Expression and statement primitives are pure data — a template, an arity, types, an optional
import. Structural constructs are **not** declarable at all: `if`, `let`, `loop` and `=` are
injected into every target and declaring one is an error, because *a construct promoted to the
language works on every target and the compiler finds the implementation.*

What a declaration carries was not designed — it was read off what four backends turned out to
need. Format: [target-files.md](docs/spec/target-files.md).

### The same capability, opposite idioms

Word count's dictionary, from one source:

| | emitted | because |
|---|---|---|
| Go | `acc[k]++` — **fused** | one `mapassign_faststr` |
| JavaScript | null-prototype object | `Map` is 3.25× slower for string keys |
| Java | **both, and the program picks** | fused `merge` was recorded 2.6× slower in 2026-08; on JDK 17 it is **1.19× faster** |

Go's fused idiom wins and Java's loses, decided by measurements taken before either backend
existed. That is [ADR 0008](docs/decisions/0008-measurement-over-principle.md) when it is real
rather than stated.

## What is in the language

| | |
|---|---|
| **Iteration** | `loop`/`again` — guarded clauses over n variables, no product, early exit at parity. **Recursion is not in the language**; termination is a *computed property* ([ADR 0014](docs/decisions/0014-recursion-is-not-in-the-language.md), [ADR 0015](docs/decisions/0015-loop-and-again.md)) |
| **Booleans** | `bool` is data, `if` is its eliminator, connectives erase in the reader ([ADR 0017](docs/decisions/0017-booleans-are-in-the-language.md)) |
| **Several results** | `(values a b)` is the *negative product* — sugar for `(fn (#k) (#k a b))`, so β is its algebra and the reducer needed nothing ([values.md](docs/spec/values.md)) |
| **Pattern matching** | `match` is `loop`: reader sugar, zero rules, zero term kinds, `again` in a clause body ([match.md](docs/spec/match.md)) |
| **Sums** | closed, finite, non-recursive. A sum is Σ, so its value is a tag and a payload — which is the product, already built on four targets ([sums.md](docs/spec/sums.md)) |
| **Memory** | immutable values, one scoped **linear** buffer; the linearity check is occurrence counting on the residual, not a type ([ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md)) |
| **Effects** | one declared bit per primitive, defaulting to impure. An impure argument is never substituted. No effect types, no monads ([ADR 0010](docs/decisions/0010-effects-as-structural-rules.md)) |

### Types are not in the language, and they still check

There is a type checker, and it runs on the **residual**, before emission — cheap, because
reduction has already made the term monomorphic, first-order and closed. One checker serves all
four targets. `(sig name ((p type)…) result)` is a claim checked in **two directions**: against the
definition's residual, and against any target that provides the name natively — the job no host
compiler can do, because the two implementations live on different targets.

On top of that:

- **Refinements** — `(where …)` in linear integer arithmetic, with a deliberately *incomplete*
  decision procedure. An undischarged obligation is **reported, never assumed**. Found a real
  latent bug in `dot` and `centroid` ([refinements.md](docs/spec/refinements.md)).
- **Termination** — size-change termination plus a trip count proves **96% of loops**; the single
  refusal is a true negative ([sct-2026-08-19](gauntlet/results/sct-2026-08-19.md)).
- **Representation selection** — a declared range decides whether an integer operation keeps the
  host's operator or is rewritten to a `checked` primitive. Opt-in behind `-checked`, deliberately
  ([selection-2026-08-19](gauntlet/results/selection-2026-08-19.md)).

The map of what is decidable, so future decisions can be *located* rather than argued from scratch,
is [decidability-map.md](docs/decidability-map.md).

## Design goals

| | |
|---|---|
| Small | The language should be easy to implement. |
| Parasitic | Take maximum advantage of each target ecosystem. |
| Open | Adding a target should be low effort, for anyone, out of tree. |
| Declarative bindings | Adding a target's APIs should be close to a file listing names. |
| Fast | **Parity with hand-written code in the target language.** |
| Small output | Small binaries and footprints. |
| Abstractable | Express more in fewer tokens over time. |
| Legible to models | Easy for LLMs to write and reason about. |

## How this is run

The core is **not** specified up front. The predecessor project stalled on a fixed language that
work then went into making viable, and committing to a specification now would recreate that.

Instead one thing is fixed: **[the gauntlet](docs/gauntlet.md)**. Candidates are killed by
measurement, not by argument — arguments only select what is worth measuring. When a candidate
dies it gets an ADR naming what killed it, so a dropped direction becomes an accumulating result
rather than lost time.

Three working rules that have earned their place:

1. **Every design claim in this repository that was not measured has been wrong about half the
   time.** The measurements in [gauntlet/results/](gauntlet/results/) are the authority.
2. **Carry both forms** into a benchmark — the one expected to win and the one expected to lose.
   Five beliefs were refuted in the first run only because the losing form was there to measure.
3. **Check the compiler's decisions, not just the clock.** `-gcflags=-m -m` and
   `-d=ssa/check_bce/debug=1` were each decisive where timings were ambiguous.

Benchmarks were taken on a hybrid P/E-core laptop with a **~15% noise floor**. No decision here
rests on a smaller margin than that — and where a measurement failed to measure what it intended,
that is recorded too rather than dropped.

### Decisions so far

| | |
|---|---|
| [0001](docs/decisions/0001-parasite-model.md) | Targets are ecosystems; portability is a program property |
| [0002](docs/decisions/0002-capability-graph.md) | Capability graph, not a fixed layer tower |
| [0003](docs/decisions/0003-range-typed-integers.md) | Range-typed integers, mathematical semantics |
| [0004](docs/decisions/0004-first-targets.md) | Go, JavaScript, Java/Android first; C deferred |
| [0005](docs/decisions/0005-implementation-language.md) | Compiler written in Go |
| [0006](docs/decisions/0006-ir-file-format.md) | Backend interface is a file format, not a Go interface |
| [0007](docs/decisions/0007-exploration-over-specification.md) | Explore candidates against a fixed test |
| [0008](docs/decisions/0008-measurement-over-principle.md) | Parasite decisions are measurements, not principles |
| [0009](docs/decisions/0009-staging-preserves-results.md) | Staging must not change results |
| [0010](docs/decisions/0010-effects-as-structural-rules.md) | Effects are a side condition on β, not a feature |
| [0011](docs/decisions/0011-modules-add-nothing-to-the-reducer.md) | Modules are resolution, not reduction |
| [0012](docs/decisions/0012-portable-integer-range.md) | `int` is exact within ±(2⁵³−1) |
| [0013](docs/decisions/0013-accept-the-allocation-price.md) | Accept the allocation price — **provisional** |
| [0014](docs/decisions/0014-recursion-is-not-in-the-language.md) | Recursion is not in the language |
| [0015](docs/decisions/0015-loop-and-again.md) | `loop`/`again` — guarded clauses over n variables |
| [0016](docs/decisions/0016-targets-need-not-have-expressions.md) | A target need not be an expression language |
| [0017](docs/decisions/0017-booleans-are-in-the-language.md) | Booleans and control flow are in the language |
| [0018](docs/decisions/0018-immutable-values-linear-buffers.md) | Immutable values, one scoped linear buffer |

Each has a "Why not" section recording the alternatives rejected — this project is deliberately put
down at dead ends and picked up later, and the rejected alternatives are what will not be
recoverable from the code.

## Some things measurement decided

**A target need not have expressions.** `targets/windows/` emits x86-64 assembly under MASM and
reaches parity with hand-written assembly, with the structural set still three
([ADR 0016](docs/decisions/0016-targets-need-not-have-expressions.md)). It also showed that *the
optimisations you were parasitizing only become visible on a host that has none* — the first three
hosts were doing common-subexpression elimination for us and nothing noticed until one did not.

**Our proofs do not transfer.** A proof buys nothing unless the emitted code is *shaped* so the
host re-proves it. That win was collected as an emitter pattern needing no types at all: 1.96× on
compute-bound loops and **nothing** on memory-bound ones —
[bce-2026-08-15](gauntlet/results/bce-2026-08-15.md).

**A cost behaves the way a saving does.** An unproven integer operation costs 1.23× to 4.54×
depending on shape, and the isolated microbenchmark was wrong in *both* directions —
[checkcost-2026-08-19](gauntlet/results/checkcost-2026-08-19.md).

**The memory model was decided by expressiveness, not by speed.** A pure gather cannot express a
*scatter*, so the sieve, sorting, histograms, union-find and general DP are inexpressible portably
at any speed. It cost almost nothing, because every mechanism it needs already existed
([ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md)).

**Compile-time materialisation into static data is not a win.** Free on x86 and Go, a pure loss on
Java and JavaScript (3.5× slower to load, 2,600× larger source) —
[staticdata-2026-08-20](gauntlet/results/staticdata-2026-08-20.md).

## What is open

The honest list, with the reasoning written down rather than deferred to memory:

- **Recursion** moved from *deferred* to **owed** — a JSON parser, a DOM walk and a
  recursive-descent parser all recurse to a depth the input decides, so
  [ADR 0014](docs/decisions/0014-recursion-is-not-in-the-language.md) needs a superseding ADR
  ([general-purpose.md](docs/general-purpose.md))
- **Tables** — specified, not built ([tables.md](docs/spec/tables.md))
- **Strings**, **growable collections**, **maps** in the portable language
- **The type system reasoning about the target** — expressing a Win32 contract so a program can be
  checked in Oroboros; SAL is the field-tested answer and five of the eight requirements exist
- **The niche encoding** for sums, `try` as bind, and `match` on a sum
- **Octagons** instead of the hand-rolled relational layer in `emit/refine.go`
- [ADR 0013](docs/decisions/0013-accept-the-allocation-price.md)'s allocation price, which is
  expected to be paid off, not kept

Design questions still open are listed in §8 of
[docs/design-direction.md](docs/design-direction.md). Current standing is
[assessment-2026-08-20](docs/assessment-2026-08-20.md).

## Reading order

1. This file
2. [docs/design-direction.md](docs/design-direction.md) — the reasoning, including why the
   predecessor hit a performance wall
3. [docs/the-atom.md](docs/the-atom.md) — what the core turned out to be
4. [docs/spec/state.md](docs/spec/state.md) — the language as it is today
5. The ADRs in [docs/decisions/](docs/decisions/)
6. [gauntlet/results/](gauntlet/results/) — the authority

## Name

The predecessor was called **Parasite**, after the strategy. **Oroboros** — the serpent eating its
own tail — for the intended endpoint: a language whose compiler is eventually written in itself.
