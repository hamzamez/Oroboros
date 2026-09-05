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
and **the gauntlet at parity with hand-written code on Go, JavaScript and Java** — two programs
producing byte-identical machine code on Go, and one miss: Java's tokeniser at 1.16×, with the cause
isolated rather than argued away.

The language's own data structure — a **table**, which is a function with a known finite domain —
works on all four targets, and on x86 the portable sieve is **0.88×** of hand-written assembly.

A **JSON parser** — tokeniser and tree — runs on all four targets **without recursion**, which is
the standing claim that recursive data is a flat table plus indices, put to work rather than
argued ([tokeniser](gauntlet/results/json-2026-08-26.md),
[tree](gauntlet/results/json-tree-2026-08-26.md)).

**Maps** are built on all four ([maps.md](docs/spec/maps.md)) — including windows, which ships no
hash table, so the language supplies one *written in Oroboros*. A static map read reduces to a
constant even there, because the probe itself is static.

And **integer arithmetic is finally the language's**. Until 2026-08-31 `=` was the only integer
operator it owned: `(+ 1 2)` was *"not bound"*, and every portability claim in this repository was
really a claim about `go.+`. `+ - * / % < <= > >=` are now found per target the way `=` always was,
so one source computes the same answer on four hosts with no host name in it.

**And the whole integer ladder is built** — narrower than a word, the host's word, fixed limbs, the
host's own bignum. A **range is semantics and the target picks the storage**, exactly as
`(array (int 0 255))` has always been a `[]byte` on Go and a `short[]` on the JVM; getting that
backwards at the top of the ladder was costing 75× on V8
([bigrepr-2026-09-03](gauntlet/results/bigrepr-2026-09-03.md)). windows, which ships no bignum, gets
one **written in Oroboros** — add, subtract, multiply, divide and take the remainder by a machine
word, and compare — so ADR 0019's fourth item is delivered on the host that had nothing to fall back
to ([subdiv-2026-09-03](gauntlet/results/subdiv-2026-09-03.md)).

| | |
|---|---|
| `core/` | reader, terms, β/δ reducer — [the atom](docs/the-atom.md) |
| `emit/` | **Go, JavaScript, Java, x86-64**. Types, refinements and termination live here, *not* in the language |
| `targets/` | target declarations — **data, not Go** |
| `cmd/oro` | reduce a file to normal form against a target |
| `cmd/gen` | emit a file into the gauntlet |
| `cmd/build` | follow imports, reduce `main`, emit, run the host toolchain |
| `examples/` | 68 programs |
| `gauntlet/` | hand-written references and 63 recorded measurements — **the bar** |
| `gauntlet/differential/` | 25 programs built and **run** on all four targets, outputs required identical *and* right |

```bash
go run ./cmd/oro   -target=go examples/table/dot.oro       # reduce to normal form
go run ./cmd/build -target=go -o hello examples/hello.oro  # a real binary
go test ./core/ ./emit/
cd gauntlet/differential && go run run.go                  # 25 programs x 4 targets
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

Every word an `.oro` file can contain is audited in [inventory.md](docs/spec/inventory.md) — 65 of
them, 60 specified, taken from the code rather than from memory. **That audit now predates the map
and arithmetic work** and is owed a re-run: `map`, `build-map`, `insert`, `keys`, the nine promoted
operators and three new target-file forms have arrived since, and an audit that is not re-taken is
just a number.

The rule count was **3** until sums landed on 2026-08-22 and it went up honestly rather than by
relabelling: literals fold, and an eliminator is pushed through `if` and `let`. Both exist because a
sum should cost nothing at *either* level, and the second was
[measured across 184 residuals](docs/spec/sums.md) to change no existing program before it shipped.

Folding started as `=` on two integers and covers all of `+ - * / % < <= > >=` since they became the
language's. Two side conditions, both [ADR 0009](docs/decisions/0009-staging-preserves-results.md):
a result outside the portable window is **not** folded, because compile time is Go's `int64` and run
time on V8 is a binary64 exact only to ±(2⁵³−1) — and leaving the operation alone is what lets the
overflow analysis report it against what the programmer *wrote*. Division by zero is not folded
either; it is a precondition, so the refinement layer names the call site instead of the compiler
panicking. No float folds at all, which is ADR 0009's original case.

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
| **JSON tokeniser** | **0.99×** | **1.02×** | **1.16×** |

Programs 1–6 are all the same shape — countable loops over arrays, which every host optimises best
and which the emitter had been tuned against for five months. **Program 7 is a tokeniser**, added
because parity on *branchy* code had never been measured
([jsontok-2026-08-26](gauntlet/results/jsontok-2026-08-26.md)): a data-dependent switch per byte,
unpredictable branches, and scanners whose trip count the input decides. Go and JavaScript hold.

**Java's 1.16× is the one place the bar is missed, and it has been taken apart rather than argued
away.** It looked like one cost and was three, each isolated by hand-writing the same program in the
shape we emit:

| | |
|---|---|
| the **element type** — our `int` is 64-bit, so a byte array was `long[]` | fixed: a declared range now picks `short[]` |
| the **index type** — a Java array index is 32-bit, so every access carried an `(int)` cast | fixed: the interval analysis narrows the counter, casts went 50 → 5 |
| what is left | **code generation plus the refinement layer's guards** — a different question, and the first time it has been the residue |

The first two are gone and the time barely moved, which is the finding: they were two costs that
looked like one because they had only ever been measured together.

All but two are inside the ~15% noise floor, which is the claim: **at parity, not faster**. The
first exception is JavaScript's early-exit search — 7.5 ns against 6.2 ns on a single call returning at
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

### A JSON parser, with no recursion

The claim that **recursive data is a flat table plus indices** had been repeated for months. This is
it run. `examples/json/tokenize.oro` nests to a depth the *input* decides, using `loop`, `again` and
a stack in a `build` buffer — 57 lines of code, no new term kind, no new reduction rule, no new
primitive, and no target declares anything for it:

```lisp
(def tokens (fn (src)
  (build (cap) (fn (stk)
    (loop ((stk stk) (i 0) (nt 0) (sp 0) (mx 0) (ok 1))

      (go.< i 0)  0

      (go.>= i (len src))
        (go.+ (go.* nt 1000) (go.+ (go.* mx 10) (if (= sp 0) ok 0)))

      (go.>= sp (cap))  (go.* nt 1000)

      (space? (src i))   (again stk (go.+ i 1) nt sp mx ok)

      (opener? (src i))
        (again (set stk sp (if (= (src i) 123) 125 93))
               (go.+ i 1) (go.+ nt 1) (go.+ sp 1)
               (if (go.> (go.+ sp 1) mx) (go.+ sp 1) mx) ok)

      (closer? (src i))
        (again stk (go.+ i 1) (go.+ nt 1)
               (if (go.< sp 1) 0 (go.- sp 1)) mx
               (if (go.< sp 1) 0
                   (if (= (stk (if (go.< sp 1) 0 (go.- sp 1))) (src i)) ok 0)))

      (punct? (src i))   (again stk (go.+ i 1) (go.+ nt 1) sp mx ok)
      (= (src i) 34)     (again stk (scan-string src i) (go.+ nt 1) sp mx ok)
      (numeric? (src i)) (again stk (scan-run src (go.+ i 1) numeric?) (go.+ nt 1) sp mx ok)
      (alpha? (src i))   (again stk (scan-run src (go.+ i 1) alpha?) (go.+ nt 1) sp mx ok)

      else (again stk (go.+ i 1) nt sp mx 0))))))
```

Three clauses there exist because the **compiler demanded them**, and the third is the result.
`(set stk sp …)` carries `sp < cap` as an obligation that nothing in the program can discharge —
`sp` grows with the input — so the program has to answer *"what happens when the nesting is deeper
than the stack"*:

```lisp
(go.>= sp (cap))  (go.* nt 1000)
```

**A recursive-descent parser has exactly the same limit — the C stack — and is never asked.** The
explicit stack does not create the limit; it makes it visible. That is the best argument for
[ADR 0014](docs/decisions/0014-recursion-is-not-in-the-language.md) found so far, and it is not a
performance argument.

The other two are the interval analysis being non-relational, and they cost one compare each. The
[tree](examples/json/tree.oro) is the other half — a flat node table, stride 4, `tag`/`val`/`kid`/`sib`,
with node 0 as the `none` sentinel holding the header, parsed and then **walked**, because building a
tree and never traversing it would prove nothing.

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
| **Integers** | `int` is exact within ±(2⁵³−1) and a **range is a type** — `(int LO HI)` in a parameter, a result, an array element or a map value. `+ - * / % < <= > >=` and `=` are the language's, found per target by spelling; **bitwise and shifts are not**, because V8 truncates them to int32 and `(2³²) & -1` is 0 there and 4294967296 elsewhere — observable *inside* the window. **Bounded by default**: an operation the compiler cannot prove stays in the window is a compile error, cleared by narrowing the range, taking the trap, or declaring a range *above* the window — which promotes that value to arbitrary precision, stored as the target says ([integers.md](docs/spec/integers.md), [ADR 0003](docs/decisions/0003-range-typed-integers.md), [ADR 0012](docs/decisions/0012-portable-integer-range.md), [ADR 0019](docs/decisions/0019-precision-by-declaration.md)) |
| **Maps** | a table whose index set is a finite subset of the key type. `(m k)` is `(option V)` — the map is the first construct whose domain condition *nothing* can discharge, so the program says what happens when the key is absent, and it lowers to the host's own fallible read. `keys` is ascending by key, which is **derived**: the result is an ordered index set, so producing one requires an order, and the only canonical one is K's ([maps.md](docs/spec/maps.md)) |
| **Tables** | the primary data structure, and there is one: **a function with a known finite domain.** `(array e…)` a graph, `(table n f)` a rule with no memory, `(len t)` the domain bound — and **indexing is APPLICATION**, `(a i)`, with no word of its own ([tables.md](docs/spec/tables.md)) |
| **Memory** | immutable values, one scoped **linear** buffer — `(alloc t)` gathers, `(build n f)` scatters, `(set b i v)` consumes and returns. The linearity check is occurrence counting **on the residual, not a type**, and it is an *ordering* property: reads do not consume, and a read may not **move across a store** — which is where the discipline turned out to be leaking, since a buffer read looks exactly like an array read and an array read is genuinely pure ([ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md), [effects.md §7c](docs/spec/effects.md)) |
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
- **Termination** — size-change termination plus a trip count. On the two parsers, with one range
  declared: the tokeniser proves **20 of 20** loops and 100% of its integer operations, the tree
  **25 of 25** ([rebench-2026-08-27](gauntlet/results/rebench-2026-08-27.md)). Read those numbers
  with [fixpoint-2026-08-27](gauntlet/results/fixpoint-2026-08-27.md) beside them: the interval
  fixpoint was **not monotone** — `restore` installed its snapshot by reference — so every
  provability figure recorded before it was **inflated**, and the earlier 96% was measured against a
  smaller corpus and an unsound analysis.
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
| [0019](docs/decisions/0019-precision-by-declaration.md) | Precision by declaration — **provisional** |

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

**A number that has failed to reproduce twice should stop being quoted.** All three measurements a
map design would have rested on were re-taken and **all three moved**: JavaScript's `Map` is 1.56×
slower than an object rather than 3.25×, a plain `{}` beats `Object.create(null)` against the
folklore, and Java's fused `merge` is 1.22× *faster* where the first baseline had it 2.59× slower.
Integer keys, never measured before, are **3.67×** — so `(map int V)`, picked to dodge the string
question, is the case where the host choice matters most
([maps-2026-08-30](gauntlet/results/maps-2026-08-30.md)). That is
[ADR 0008](docs/decisions/0008-measurement-over-principle.md) applied to its own best example.

**"Recursive data is a flat table plus indices" is a GO fact.** The same program on three hosts:
flat beats recursive descent **2.52× on Go**, **1.22× on JavaScript**, and **loses 1.24× on the
JVM** — which bump-allocates in a TLAB, pays only for survivors when every node dies, and
scalar-replaces what does not escape. A claim this repository had been repeating as a principle was
a measurement on one host ([json-tree-bench-2026-08-26](gauntlet/results/json-tree-bench-2026-08-26.md)).

**Clamping an index cost 1.35×, so the compiler learned to prove instead.** A clamp is not a branch
— it is a data dependency in the address computation. The reason every index was clamped turned out
to be a *missing inference*, not a missing fact: the decision procedure matched a fact to a goal by
requiring identical coefficients, so `sp ≥ 1` could not discharge `2·sp − 1 ≥ 0`. **One Farkas
multiplier** fixed it, and a **stride** is exactly the shape it had been missing — no program here
had one until a node table.

**Compile-time materialisation into static data is not a win.** Free on x86 and Go, a pure loss on
Java and JavaScript (3.5× slower to load, 2,600× larger source) —
[staticdata-2026-08-20](gauntlet/results/staticdata-2026-08-20.md).

## What the compiler proves, and what it refuses to

Types are not in the language, but four analyses run on the residual — where reduction has already
made the term monomorphic, first-order and closed — and every one of them **reports rather than
assumes** when it cannot decide.

A **range is a type**. `(sig tokens ((src (array (int 0 255)))) int)` says what a source byte *is*;
each target says how wide it stores one, declared as data:

| | Go | Java | JavaScript |
|---|---|---|---|
| `(array (int 0 255))` | `[]byte` | **`short[]`** | plain `Array` |

Java picks `short[]` because the JVM's `byte` is **signed**, so `0..255` does not fit it — and
nothing in the compiler special-cases that. The target declares what it can hold and the range picks
([elemwidth-2026-08-27](gauntlet/results/elemwidth-2026-08-27.md)). A `build` buffer's range is
*inferred* rather than declared, because [ADR 0003](docs/decisions/0003-range-typed-integers.md) says
ranges are declared at boundaries and inferred for locals.

Two small theorems close the JSON tokeniser completely — **100% of its integer operations proven
inside the portable window, every loop proven to terminate**:

- **Loop monotonicity.** If every `again` gives a position a value no smaller than its current one,
  and every exit is no smaller either, then the loop's value is at least its initial value. So a
  scanner returns *more than it was given* — the fact five earlier results all terminated at, and it
  is **derived rather than declared**, because a postcondition on an internal definition is
  redundant once reduction inlines the call.
- **The running extremum.** `mx = max(mx, e)` hands the variable back in one branch, so the fixpoint
  could never shrink it. But the reachable set is `{z} ∪ U` — closed after one step — so the bound is
  exact and needs no widening at all.

Both are proved by induction in [monotone.go](emit/monotone.go), and each proof step is a test:
`(loop ((j i)) (go.>= j 10) 0 else (again (go.+ j 1)))` increases at every step and returns **0**,
which is why the exit half of the theorem is not optional.

**Postconditions are the dual of preconditions, and the algebra is a swap.** One syntax, three
meanings each, and every row exchanges the two roles
([postconditions.md](docs/spec/postconditions.md)):

| on | `where` | `ensures` |
|---|---|---|
| a `prim` | obligation, discharged at each call | assumption, granted where the obligation was **proven** |
| an **exported** definition | assumption — the caller is outside | obligation, checked against the body |
| an **internal** definition | dropped — inlining is stronger | redundant — inlining is stronger |

Both vanish on the third for the same reason: reduction removes the boundary.

**And there is exactly one exception, which took five arrivals to see.** That table holds for what a
declaration **asserts** — a fact is something to be proven, and inlining gives strictly more
information, so a fact declared at a boundary is redundant once the boundary is gone. It fails for
what a declaration **requests**. A range *above* the portable window does not assert something the
compiler checks; it asks for arbitrary precision, and inlining gives more information about values
and none at all about intent. So that one is moved onto the term — `(the "int 0 …" e)`, a structural
name injected into every target and erased once the representation is chosen
([ascribe-2026-09-03](gauntlet/results/ascribe-2026-09-03.md)). Nothing here had separated assertion
from request, because until arbitrary precision every declaration was an assertion.

**And twenty-five programs are built and run on all four targets**, outputs required byte-identical *and*
required to be the right answer — because four backends can agree and all be wrong, and the one bug
a purely differential test cannot see is a bug in the reader or the reducer, which they share. It
has caught silent wrong answers twice in a week, including `for (;; a, b = x, y)` — simultaneous
assignment on Go, a **syntax error** on Java, and the **comma operator** on JavaScript
([differential-2026-08-26](gauntlet/results/differential-2026-08-26.md)).

## What is open

The honest list, with the reasoning written down rather than deferred to memory:

- **A precondition that states MEANING has no enforcement anywhere.** A `prim`'s `where` is
  discharged at every call site; a definition's is *dropped*, and what protects the program instead
  is that reduction inlines the call and the body's own obligations land on the caller's concrete
  values — which is **stronger** than checking the declared clause, because that clause is only a
  conservative summary. The gap is the case inlining cannot reach: a body that is total and merely
  *wrong* outside its domain, where nothing fires. `win/fmt.print-int` prints a blank line for a
  negative number and is within its rights. Same shape as SAL's `_Success_`, so it belongs beside
  the Win32 work rather than ahead of it
  ([refinements.md §6b](docs/spec/refinements.md),
  [differential-2026-08-26](gauntlet/results/differential-2026-08-26.md)).
- **Recursion.** [general-purpose.md](docs/general-purpose.md) moved it from *deferred* to **owed**,
  arguing that a JSON parser, a DOM walk and a recursive-descent parser all recurse to a depth the
  input decides. **Two of those three now run here without recursion** — the tokeniser is the parser
  and the tree walk is the DOM walk — so the superseding ADR is not owed *on the grounds it gave*.
  What is genuinely unsettled is **ergonomics**, which is a different argument and has not been made
  with a measurement: 112 lines against maybe 60, and three constructs in that program exist because
  of what the language refuses. And the *performance* half of the counter-claim is now known to be
  host-specific (see above), so ADR 0014 rests on portability — stack depth differs by orders of
  magnitude across the four hosts and none guarantees tail calls.
- **Strings — started, and derived rather than borrowed.** `general-purpose.md`'s list is otherwise
  answered: recursion by two parsers that do not need it, sums built, maps built, growable
  collections withdrawn. **A string is an element of `Scalar*`, the free monoid over Unicode scalar
  values**, and that one choice settles the rest: literals and their **six escapes are FORCED** by
  the notation rather than copied from a host
  ([string-literals.md](docs/spec/string-literals.md)), and the free monoid's universal property
  **enumerates** the operation set instead of leaving it to taste — an encoding being a monoid
  homomorphism too ([string-operations.md](docs/string-operations.md)).
  **And the first real text program FOLDS rather than indexing**: decimal rendering of a bignum is
  20 lines in three derived operations — `concat`, `""` and η — with no `(s i)`, no `length` and no
  string `=`, on three hosts and both storage representations
  ([render-2026-09-04](gauntlet/results/render-2026-09-04.md)). That defers the representation
  question rather than answering it, because a text-CONSUMING program would index: a string-based
  tokeniser is **1.89× slower than an array-based one on V8**
  ([jsontok-2026-08-26](gauntlet/results/jsontok-2026-08-26.md)), and indexing by scalar position is
  **quadratic on every host** — 262 ms on Go, 383 ms on V8, 97 ms on Java at 16,000 scalars — because
  text is stored variable-width everywhere, so `alloc` is the resolution when one is needed.
  It also corrects `strings.md` §2: `length` of `"🙂"` being 4 on Go and 2 on JS and Java are answers
  to *different questions* — bytes and UTF-16 units — and the scalar count is **one answer,
  computable on every target at O(n)**, which is price rather than disagreement. What is still owed
  is windows, which has all the arithmetic and no string type, so `concat` and η there are a library
  over `build`. **Maps** are built ([maps.md](docs/spec/maps.md)), and
  **growable collections are withdrawn**: count-then-build measures **2.95× faster than growing
  `append` on Go** and at parity on JavaScript, so the workaround every array language uses is
  better than the thing it works around ([maps-2026-08-30](gauntlet/results/maps-2026-08-30.md)).
- **Integers are finished, and that is now the honest word for it.** Every one of
  [ADR 0019](docs/decisions/0019-precision-by-declaration.md)'s escapes exists, the four rungs of
  the representation ladder are built, and **a range is semantics while the target picks the
  storage** — `(big-repr host)` on Go, JavaScript and Java, `(big-repr limbs)` on windows, each
  carrying the measurement that decided it. Reading the *shape* of a declaration as a storage
  instruction had been costing **5.85× on Go, 74.9× on V8 and 2.82× on Java**, paid by whoever wrote
  the more informative declaration ([bigrepr-2026-09-03](gauntlet/results/bigrepr-2026-09-03.md)).
  The bound is enforced under both representations, so choosing one changes what a program *costs*
  and never what it computes or whether it is legal.
  Three things worth carrying out of that work. **Our own limb form was decomposed rather than
  guessed at**, and the clamp, the element mask and the buffer clear are together inside the noise
  floor while `/` and `%` by a constant power of two cost **2.39×** — because signed division needs a
  rounding correction, so the fix is a *proof* that the dividend is non-negative and not a bitwise
  operator in the language ([shiftdiv-2026-09-03](gauntlet/results/shiftdiv-2026-09-03.md)).
  **64-bit limbs are unreachable**: ADR 0012 makes an `int` exact to ±(2⁵³−1), so bigarith's 2.75×
  was never available to a portable library, and 32-bit measures 1.20×. And **a declaration is a
  DIRECTIVE, so it moves onto the term**: a range above the window declared on an internal helper was
  erased by inlining, and the analysis cannot supply what it says — it reports `[-inf, +inf]` for a
  loop whose trip count is a constant 6, and a factorial's bound is not expressible in an interval
  domain at all ([ascribe-2026-09-03](gauntlet/results/ascribe-2026-09-03.md)).
  What is left is not integer work: **rendering a bignum on windows** waits on a string library for
  that host — the other three render today ([render-2026-09-04](gauntlet/results/render-2026-09-04.md)),
  **big-by-big division** (Knuth D) has no caller, and **32-bit limbs and a per-operation threshold**
  would move no target's declaration on today's numbers. The one open question is
  [ADR 0019](docs/decisions/0019-precision-by-declaration.md)'s own trigger — *how many declarations
  does a real application need?* — and 70-of-70 programs emitting byte-identically with the checks on
  is a corpus of numeric kernels and two parsers written by people who knew the analysis.
- **Java's last 1.16×**, and it is a smaller question than it was. Element width and index type were
  two costs that looked like one because they were measured together; both are now matched to the
  hand-written reference, casts went from 50 to 5, and what remains is code generation plus the
  refinement layer's guards.
- ~~**Octagons.**~~ **Refuted by measurement, twice**, and left here because
  [decidability-map.md](docs/decidability-map.md) called them *the highest-value move available* and
  three results named a demand. Before building an O(n³) domain, every unproven operation in the
  corpus was classified by the fact that would settle it and **not one needed an octagon**
  ([maxlen-2026-08-28](gauntlet/results/maxlen-2026-08-28.md)) — the reason generalises, since
  `i − n ≤ c` bounds `i` only when `n` is bounded, and if `n` is bounded the guard already gives it
  non-relationally. The tree's residue then turned out to be values read *out of a table*, which
  needs a quantified array invariant — **strictly stronger** than an octagon, not weaker
  ([frozen-2026-08-28](gauntlet/results/frozen-2026-08-28.md)). What measurement selected instead
  was a length bound at both ends, and it improved thirteen programs.
- **The type system reasoning about the target** — expressing a Win32 contract so a program can be
  checked in Oroboros; SAL is the field-tested answer and five of the eight requirements exist
- **The niche encoding** for sums, `try` as bind, and **`match` on a sum** — which would remove one
  of the two eliminators, and is waiting for a program that wants `again` over a sum
  ([sums.md §7](docs/spec/sums.md))
- **Element size in the type**, generally. x86 needed it and got a local answer; nothing says what
  `(array bool)` means on a host that has no `bool`
- **The x86 register allocator**, which is now the binding constraint on windows rather than any
  missing capability. Three differential cases cannot be built there — `x64.movb has more spilled
  operands than there are scratch registers` — and it is program SIZE, not a construct: it
  reproduces at one input, and with the big value built by a loop instead of a literal. That host
  has neither a type system to size its elements nor a register allocator to spill for it, which is
  [ADR 0016](docs/decisions/0016-targets-need-not-have-expressions.md)'s lesson arriving again.
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
7. [gauntlet/results/](gauntlet/results/) — 63 measurements, and **the authority**: every design
   claim here that was not measured has been wrong about half the time

## Name

The predecessor was called **Parasite**, after the strategy. **Oroboros** — the serpent eating its
own tail — for the intended endpoint: a language whose compiler is eventually written in itself.
