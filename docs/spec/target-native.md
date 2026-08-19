# Target-native modules, and the prices

**What this is.** `targets/go/`, `targets/js/` and `targets/java/` declare each host's own names with Go's own semantics and **makes no
portability claim at all**. `go.+` is Go's `+`; `go./` truncates and panics on zero because that is
what Go does. Nothing is renamed so that three hosts can agree, because nothing here claims they do.

The point is to see the language with the portability layer taken away — to find out **which
limitations are the language's and which were the layer's**.

The layers they replaced are preserved as `targets/portable-go.oro`, `portable-js.oro` and
`portable-java.oro`, because the gauntlet's seven programs are written against them and are the
record of what parity was measured on.

---

## 1. What changed

**Targets are directories.** `LoadTarget` accepts a folder and merges one `(target NAME …)` form per
file — union, no precedence, duplicate declarations are an error. `targets/go/` is `go.oro` (types,
structural primitives, toolchain), `builtin.oro` (Go's predeclared identifiers and operators) and
`fmt.oro` (the whole package). This is what [build.md §4](build.md) and
[modules.md](modules.md) both recorded as *"a target is still one file rather than a directory"*.

**Host names, not ours.**

```lisp
(use go)
(use go/fmt)

(go.+ x y)   (go./ a b)   (go.% a b)   (go.len s)   (go.delete m k)   (fmt.Println x)
```

That needed one change to the reader: `%&|^~` joined the identifier characters. A target module that
has to rename `%` to `rem` is teaching the reader's limitations rather than the host's, which is the
opposite of what a target file is for.

**`fold-range` and `fold-range2` are not declared.** `loop` subsumes both
([ADR 0015](../decisions/0015-loop-and-again.md)), and a target offering two ways to write a counted
loop teaches neither. Neither is `make-vec` — see §6. **The structural set on this target is three: `let`, `if`, `loop`.**

## 2. What the language could not express

This is the list the experiment existed to produce.

### 2.1 The type language has no constructors — and it is the big one

`[]T` is not a type applied to an argument; it is an atom. So every instantiation needs a name, and
every polymorphic host function needs one declaration per type:

```lisp
(prim make-int     (int) slice-int     expr "make([]int, %s)")
(prim make-float64 (int) slice-float64 expr "make([]float64, %s)")
(prim at-int     ((v slice-int) (i int)) int         expr "%s[%s]" pure index …)
(prim at-float64 ((v slice-float64) (i int)) float64 expr "%s[%s]" pure index …)
```

Go writes `make([]T, n)` and `s[i]` once. We write them per type, and the *program* has to pick:
`go.at-float64` rather than `go.at`. **The suffix is our type table showing through**, and it is the
single largest source of non-Go-ness in a target-native program.

It bites hardest on `len`, `cap` and `==`, which are polymorphic in Go over every type. They are
declared with `any`, which demands nothing — so they work, at the cost of the checker learning
nothing from them.

### 2.2 No variadics

Every `fmt.Print*` in Go is `(a ...any)`. Our arity is fixed, so `fmt.oro` declares each at the
arities programs use — `Println`, `Println2`, `Println3`, `Printf`, `Printf2`, `Printf3` — and a
`Printf` with four operands cannot be called at all.

This is the first place a *whole host package* was attempted rather than the one function a program
needed, and it is what the attempt found.

### 2.3 No multiple returns

`fmt.Fprintf` returns `(int, error)`; `fmt.Scanln` returns `(int, error)`; `m[k]` has a two-value
form. Every one is declared for its first result only, so **the error is unreachable**. `Scanln`
needs a Go closure in its template to discard it:

```lisp
(prim Scanln (any) int expr "func() int { n, _ := fmt.Scanln(%s); return n }()" (import "fmt"))
```

That is a workaround, and writing it down is the point. It is the **fourth independent demand for a
product type**, after `v, ok := m[k]`, `fold-range2`'s two accumulators, and JavaScript's
"was the key present".

### 2.4 No way to name an interface

`fmt.Fprint*` take an `io.Writer`. There is no type constructor and no way to say "anything with
this method", so the entire F-family of `fmt` is absent. Same for `error` beyond carrying it as an
opaque atom.

### 2.5 Refinements were keyed to the portable layer's names

`aindex`'s bounds obligation was written with `logic.and` and `int.le`. A target declaring Go's own
`&&` and `<=` degraded every refinement to an opaque atom — the fragment is about the *operation*,
not about who named it. `isOp` now maps operator spellings to the fragment's names, so
`(where (go.&& (go.<= 0 i) (go.< i (go.len v))))` is decided.


## 2.6 The language reserves four type names

Found by writing the JavaScript target, which called its boolean `boolean`:

```
sieve-count: in a condition: js.>= is boolean, but bool is required here
```

The structural primitives carry a fixed type vocabulary into every target:

| name | who demands it |
|---|---|
| `int` | what an **integer literal** types as; a loop bound and index |
| `f64` | what a **float literal** types as |
| `bool` | what `if` and a loop guard demand |
| `vec-f64` | what `make-vec` produces — only if a target declares it |

A target may spell them however it likes on the right — JS says `(type int "number")`, Java says
`(type int "long")`, Go says `(type f64 "float64")` — but **the names on the left belong to the
language**, not to the target. That is defensible, since the structural primitives are the
language's and so are their types, but it was undocumented and it is the first thing a new target
gets wrong. It also means a Go programmer writes `f64` in a `sig`, never `float64`: for these four,
the language's name wins over the host's spelling.

Found twice. JavaScript called its boolean `boolean` and every guard failed. The Go target called
its float `float64` and passed only because no program had used a float literal yet.

It has a sharper consequence on JavaScript. **JS has one number type and our language has two**,
because that is what an integer and a float literal type as. Declaring `js.+` on `f64` would make
`(js.+ i 1)` an error the host does not have; declaring both would reintroduce Go's suffix split for
a reason belonging entirely to us. So `targets/js/` declares arithmetic on `any`, which demands
nothing — **one `js.+`, at the price of no numeric checking on that target at all.**

## 2.7 Method syntax cannot be spelled

JavaScript and Java are method-oriented; we are not. `a.map(f)` becomes `(Array.map a f)`, and
`s.length()` becomes `(String.length s)`.

The **emitted** code is exactly right — a template's first hole is the receiver — so nothing is lost
at run time. What is lost is reading order: the receiver moves from before the dot to after the
paren. No target-file work changes that; it is what a Lisp surface costs on a method-oriented host.

**Namespace statics are the exception, and they read perfectly**, because a static call and a
qualified name have the same shape:

```lisp
(use js/Math)      (Math.floor x)        →  Math.floor(x)
(use js/JSON)      (JSON.stringify v)    →  JSON.stringify(v)
(use java/Integer) (Integer.parseInt s)  →  Integer.parseInt(s)
```

## 2.8 Java's generics are the suffix problem squared

Go needed one declaration per slice element type. Java needs one per **(container, K, V)**
combination: `List<Long>` and `List<String>` share not a single declaration, and `Map<K,V>` is
quadratic. `targets/java/util.oro` declares three instantiations; every other one Java can
express — and there are unboundedly many — is unreachable.

Java also pays a tax the others do not: our `int` is Java's `long`, deliberately, because Java's
`int` wraps at 2³¹ which is *inside* the range our literals cover. Every array index therefore
emits `(int)`. Free at run time, unavoidable, and visible in every line.

## 3. The prices, measured

The Go target no longer offers `fold-range`. What that costs, and what the native shape costs, on
the programs measured so far.

| | hand-written | ours | |
|---|---|---|---|
| search, early hit ([ADR 0015](../decisions/0015-loop-and-again.md)) | 2.68 ns | 2.87 ns | 1.07× |
| search, 37k iterations | 37,900 ns | 38,300 ns | 1.01× |
| Newton convergence | 7.44 ns | 7.84 ns | 1.05× |
| `dot`, compute-bound, `loop` vs `fold-range` | 451 ns (fold) | 463 ns (loop) | 1.03× |
| `centroid`, `loop` vs `fold-range2` | 31,414 ns | 31,492 ns | 1.00× |
| **sieve, fully native** | **17,800 ns** | **29,800 ns** | **1.68×** |

Five of the six are at parity. **The sieve is not, and its cause is only partly isolated:**

- It is **not** bounds checks. `-d=ssa/check_bce/debug=1` reports three `IsInBounds` in each.
- It is **not** the loop's shape. Hand-written Go in the same `for {}`-with-guards shape measures
  the same 29,700 ns as ours.
- It is **not** threading the array through loop state. Rewriting the sieve so the array is a `let`
  outside the loop and the write is sequenced with `seq` — which `again`-under-a-`let` exists to
  allow — changed nothing measurable.
- It **is** visible as an allocation: **20,480 B/op and 1 alloc/op against hand-written's zero.**
  `make([]bool, n)` with a variable `n` cannot be stack-allocated; the hand-written function is
  small enough for Go to inline into the benchmark, which makes `n` the constant 20000 and lets the
  slice go on the stack. Ours is larger and is not inlined.

If that reading is right the cost is **the inlining budget**, which
[size-2026-08-13](../../gauntlet/results/size-2026-08-13.md) already found to be a sharp
discontinuity, and the fix is emitting less code rather than a different loop. It is written down
here as an open price rather than a solved one.

### 2.9 The three hosts, side by side

The same sieve, written natively for each — `examples/native/sieve-go.oro`, `sieve-js.oro`,
`sieve-java.oro` — all producing 2262 and checked against a hand-written reference on their own
host.

| | Go | JavaScript | Java |
|---|---|---|---|
| structural set | **3** | **3** | **3** |
| `at` / `set` declarations | one per element type | **one, total** | one per element type |
| arithmetic declarations | one per numeric type | **one, on `any`** | one per numeric type |
| what forces the split | Go really is typed | *our* two number types | Java really is typed |
| generics | none to model | none to model | **one per instantiation, squared** |
| index casts | none | none | `(int)` on every index |
| method syntax | n/a — Go's builtins are functions | receiver moves | receiver moves |
| namespace statics | n/a | read perfectly | read perfectly |

**The clearest result is the second row.** JavaScript needs *one* `js.at` for every element type
there is, where Go needs four and Java needs four. The suffix explosion is not a fact about static
typing; it is a fact about **our type language having no constructors**, and it disappears exactly
where the host has no types for us to have to model.

## 4. What held

Worth stating, because most of it did.

- **Operators as names.** `go.+`, `go.<<`, `go.&^` are ordinary qualified names; nothing in the
  reader, reducer, checker or emitter needed to know they are operators.
- **Mutation.** A `stmt` primitive yields its first argument, so `go.set-bool` threads through a
  loop or sequences with `seq`. No new mechanism.
- **`again` under a `let`.** The rule that looked like a concession is what lets a program mutate
  and continue without threading the container — `(seq (go.set-bool c j (go.true)) (again …))`.
- **Effects.** `go.println` and `go.panic` are impure by default; `fmt.Sprintf` is declared `pure`
  and is correctly duplicated and dropped. The one declared bit still does the whole job.
- **The type checker.** It works on a target whose types are Go's, with no portability layer to
  lean on.
- **Targets as directories.** One file per host package, merged. `fmt.oro` is 30 lines.

## 5. What is deliberately still on the shelf

`targets/portable-go.oro` holds the layer: `num/f64`, `num/int`, `logic`, `io`, `alen`/`aindex`,
`dict-*`, `split-words`, `fold-range`, `fold-range2`. The gauntlet's seven programs and every
`examples/*.oro` outside `examples/native/` still build against it, and every parity number in
`gauntlet/results/` was measured on it.

Nothing about it is deleted, and reaching for it again is `-target=portable-go`. What it cost was
never visible while it was the only thing there: **it made the language look larger than it is.**
Half of what read as core — arithmetic, comparison, arrays, dictionaries, printing — was a target
file all along.

## 6. What is not done

- **The gauntlet is not migrated.** Its seven programs still use the portable layer. Migrating them
  is what would let `portable-go` be deleted rather than shelved.
- **`make-vec` was removed**, which makes the native target's structural set exactly **three**:
  `let`, `if`, `loop`. It had to go: it is hardcoded to the portable layer's type names, producing
  `vec-f64` and demanding `f64` elements, so on a target whose types are Go's it could not be called
  at all — `go.f* is float64, but f64 is required here`. A `loop` over `go.make-float64` and
  `go.set-float64` emits the same `make` plus fill loop. Whether it reaches parity is unmeasured.
