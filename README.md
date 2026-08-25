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
and **all seven gauntlet programs at parity with hand-written code on Go, JavaScript and Java** —
two of them producing byte-identical machine code on Go.

The language's own data structure — a **table**, which is a function with a known finite domain —
works on all four targets, and on x86 the portable sieve is **0.88×** of hand-written assembly.

| | |
|---|---|
| `core/` | reader, terms, β/δ reducer — [the atom](docs/the-atom.md) |
| `emit/` | **Go, JavaScript, Java, x86-64**. Types, refinements and termination live here, *not* in the language |
| `targets/` | target declarations — **data, not Go** |
| `cmd/oro` | reduce a file to normal form against a target |
| `cmd/gen` | emit a file into the gauntlet |
| `cmd/build` | follow imports, reduce `main`, emit, run the host toolchain |
| `examples/` | 58 programs |
| `gauntlet/` | hand-written references and 38 recorded measurements — **the bar** |

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
And **indexing has no word at all**: `(a i)` is an application, because a table *is* a function.

Every word an `.oro` file can contain is audited in [inventory.md](docs/spec/inventory.md) — 62 of
them, 57 specified, taken from the code rather than from memory.

The rule count was **3** until sums landed on 2026-08-22 and it went up honestly rather than by
relabelling: `=` now folds on two integer literals, and an eliminator is pushed through `if` and
`let`. Both exist because a sum should cost nothing at *either* level, and the second was
[measured across 184 residuals](docs/spec/sums.md) to change no existing program before it shipped.

Read off the code rather than from memory: [state.md](docs/spec/state.md).

### The gauntlet

Seven programs that must reach parity with hand-written code. They are the one fixed commitment.

Numbers are *emitted ÷ hand-written*, so **lower is better and ~1.00× is the bar**. All three full
runs are on the **native** targets — [Go](gauntlet/results/native-gauntlet-2026-08-20.md),
[JavaScript](gauntlet/results/native-js-2026-08-20.md),
[Java](gauntlet/results/native-java-2026-08-25.md):

| | Go | JavaScript | Java |
|---|---|---|---|
| dot product | 1.00× | 1.03× | 0.98× |
| search | 0.97× early / 1.00× late | 1.21× early / 1.00× late | 1.01× late |
| centroid (structs) | 1.01× | 0.97× | 1.00× |
| word count | 0.997× | 0.89× | 1.00× |
| generics | 0.98× | 0.98× | 1.00× |
| formatted output | correct — its pass condition is not a number | correct | correct |
| stencil (aliasing) | 0.93× allocating / 0.999× reusing | 0.94× / 0.97× | 0.99× / 1.00× |

All but one are inside the ~15% noise floor, which is the claim: **at parity, not faster**. The
exception is JavaScript's early-exit search — 7.5 ns against 6.2 ns on a single call returning at
index 6, so the 1.3 ns gap is call overhead at the timer's resolution floor. It is recorded as a
loss rather than argued away.

Separately, on the *earlier portable layer*, centroid and generics compiled to **byte-identical
machine code** against hand-written Go
([structs](gauntlet/results/structs-2026-08-14.md), [generics](gauntlet/results/generics-2026-08-14.md)) —
a stronger result than a timing, and the reason those two programs exist.

Three things stated rather than buried:

- **Java's migration refuted the measurement that made Java the interesting case.** The fused
  `merge` was recorded 2.6× slower than unfused and is **1.19× faster** on JDK 17
  ([native-java-2026-08-25](gauntlet/results/native-java-2026-08-25.md)). Both forms are declared
  now and the program picks — which is what "carry both forms" asks for and what the old conclusion
  prevented. ADR 0008's *rule* is what survives; its example is not.
- **x86-64/Windows runs two programs, not the gauntlet** — a sieve at 0.97× of hand-written
  assembly ([windows-2026-08-19](gauntlet/results/windows-2026-08-19.md)), and the *portable* sieve
  on the language's own table, which started at 3.7× and ends at **0.88×**
  ([wintables-2026-08-25](gauntlet/results/wintables-2026-08-25.md)). It is the host where the
  remaining costs become visible, which is what
  [ADR 0016](docs/decisions/0016-targets-need-not-have-expressions.md) says it is for: both of
  those costs — element size not being part of the type, and a threaded buffer costing a register —
  were being absorbed invisibly by the other three hosts.
- **[ADR 0013](docs/decisions/0013-accept-the-allocation-price.md)'s allocation price is the SHAPE,
  now on three hosts.** Allocating costs 1.54× against reusing for *hand-written Java* too, and
  2.71× for hand-written Go. The emitted code matches hand-written code in each shape; what the
  portable language still cannot express is writing into a buffer the **caller** owns, which
  [ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md) scopes to `build`. That is the
  open half, and it is expected to be paid off rather than kept.

## What it looks like

`examples/table/dot.oro`. A table is **a function with a known finite domain**, so `(table n f)` is
a vector with no runtime existence and **indexing is application** — `(a i)`, with no word of its
own:

```lisp
(use go)
(export dot)

(sig dot ((p (array f64)) (q (array f64))) f64
  (where (and (= (len p) (len q))
              (go.< (len p) 65536))))

(def zip (fn (g a b) (table (len a) (fn (i) (g (a i) (b i))))))

(def sum (fn (v)
  (loop ((acc 0.0) (i 0))
    (go.>= i (len v))  acc
    else               (again (go.f+ acc (v i)) (go.+ i 1)))))

(def dot (fn (a b) (sum (zip go.f* a b))))
```

```bash
go run ./cmd/gen examples/table/dot.oro go dot.go
```

```go
func GenDot(a []float64, b []float64) float64 {
	acc := 0.0
	var i int = 0
	var n1 int = len(a)
	b = b[:n1]
	for ; ; i = (i + 1) {
		if (i >= len(a)) {
			break
		}
		acc = (acc + (a[i] * b[i]))
		continue
	}
	return acc
}
```

Every abstraction is gone: no closure, no intermediate table, no allocation. The
[same program](examples/native/dot-go.oro) needed **six** definitions before tables existed —
`vec`, `vlen`, `vindex` and `of-array` were a vector library hand-rolled out of closures, and they
compile to the identical machine code either way.

Two things in that output are the compiler *shaping* rather than translating. `b = b[:n1]` is what
lets Go's own compiler re-prove the bound and drop the second bounds check — because
[our proofs do not transfer](gauntlet/results/bce-2026-08-15.md), so a proof is worth only what the
emitted shape can cash in. And `for ; ; i = (i + 1)` puts the update where a host compiler can see
a counted loop; emitting it inside the body instead
[cost 1.4× on the sieve](gauntlet/results/loopshape-2026-08-25.md).

The `(where …)` is not decoration either. The equality is what makes indexing `q` under `p`'s
length provable at all; the bound is what proves the loop terminates.

### The sieve, which is the program the memory model was decided on

Values are immutable and mutation exists only inside `(build n (fn (b) …))`, whose buffer is
**linear** — `(set b i v)` consumes it and hands it back, so it is threaded like any other loop
variable and frozen on the way out ([ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md)):

```lisp
(def cross (fn (c i n)
  (loop ((c c) (j (go.* i i)))
    (go.< j n)  (again (set c j true) (go.+ j i))
    else        c)))

(def sieve (fn (n)
  (build n (fn (c)
    (loop ((c c) (i 2))
      (go.>= (go.* i i) n)   c
      (c i)                  (again c (go.+ i 1))
      else                   (again (cross c i n) (go.+ i 1)))))))
```

```go
func GenCountPrimes(n int) int {
	c := make([]bool, n)
	c2 := c
	var i int = 2
	for ; ; i = (i + 1) {
		if ((i * i) >= n) {
			break
		}
		if c2[i] {
			continue
		}
		c3 := c2
		var j int = (i * i)
		for ; ; j = (j + i) {
			if (j < n) {
				c3[j] = true
				continue
			}
			break
		}
		c2 = c3
		continue
	}
	…
}
```

```
note: 10 of 10 integer operations bounded; 3 of 3 loop(s) proven terminating
```

**This program is why tables are the shape they are.** `(table n f)` is a *gather* and cannot
express a *scatter*, so the sieve, in-place sorting, histograms, union-find and general dynamic
programming were inexpressible portably **at any speed** — which is what decided ADR 0018, and it
was expressiveness rather than the 2.7×. It runs on all four targets and on x86 it is
[0.88× of hand-written assembly](gauntlet/results/wintables-2026-08-25.md).

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
declares any of it. A sum costs nothing at *either* level — a tag known at compile time reduces
away by β, and one decided at runtime reduces to the `if` that decided it.

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
| **Tables** | the primary data structure, and there is one: **a function with a known finite domain.** `(array e…)` a graph, `(table n f)` a rule with no memory, `(len t)` the domain bound — and **indexing is APPLICATION**, `(a i)`, with no word of its own ([tables.md](docs/spec/tables.md)) |
| **Memory** | immutable values, one scoped **linear** buffer — `(alloc t)` gathers, `(build n f)` scatters, `(set b i v)` consumes and returns. The linearity check is occurrence counting **on the residual, not a type**, and it is an *ordering* property: reads do not consume ([ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md)) |
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

- **Cross-target conformance for the language's own constructs.** `gauntlet/conformance/` exists
  because `split-words` *"passed every check for two months while returning different answers on
  different targets"* — and it covers that one primitive. `table`, `array`, `len`, indexing,
  `alloc`, `build`, `set`, `match`, `case` and `values` have **none**, and two silent wrong-answer
  bugs in one day were caught only by hand-written references
  ([loopshape §3](gauntlet/results/loopshape-2026-08-25.md),
  [wintables §4a](gauntlet/results/wintables-2026-08-25.md)). This is the largest open gap.
- **Recursion** moved from *deferred* to **owed** — a JSON parser, a DOM walk and a
  recursive-descent parser all recurse to a depth the input decides, so
  [ADR 0014](docs/decisions/0014-recursion-is-not-in-the-language.md) needs a superseding ADR
  ([general-purpose.md](docs/general-purpose.md)). The standing counter-claim is that **recursive
  data is a flat table plus indices**, measured 2.02× faster on irregular access — and now that
  tables exist, a JSON parser written that way is the experiment that settles it.
- **Strings**, **growable collections**, **maps** in the portable language
- **The type system reasoning about the target** — expressing a Win32 contract so a program can be
  checked in Oroboros; SAL is the field-tested answer and five of the eight requirements exist
- **The niche encoding** for sums, `try` as bind, and **`match` on a sum** — which would remove one
  of the two eliminators, and is waiting for a program that wants `again` over a sum
  ([sums.md §7](docs/spec/sums.md))
- **Element size in the type**, generally. x86 needed it and got a local answer; nothing says what
  `(array bool)` means on a host that has no `bool`
- **Octagons** instead of the hand-rolled relational layer in `emit/refine.go`
- [ADR 0013](docs/decisions/0013-accept-the-allocation-price.md)'s allocation price — the shape, on
  three hosts now, and expected to be paid off rather than kept

Design questions still open are listed in §8 of
[docs/design-direction.md](docs/design-direction.md). Current standing is
[assessment-2026-08-20](docs/assessment-2026-08-20.md).

## Reading order

1. This file
2. [docs/design-direction.md](docs/design-direction.md) — the reasoning, including why the
   predecessor hit a performance wall
3. [docs/the-atom.md](docs/the-atom.md) — what the core turned out to be
4. [docs/spec/state.md](docs/spec/state.md) — the language as it is today
5. [docs/spec/inventory.md](docs/spec/inventory.md) — every word an `.oro` file can contain, and
   which of them are specified
6. The ADRs in [docs/decisions/](docs/decisions/)
7. [gauntlet/results/](gauntlet/results/) — the authority

## Name

The predecessor was called **Parasite**, after the strategy. **Oroboros** — the serpent eating its
own tail — for the intended endpoint: a language whose compiler is eventually written in itself.
