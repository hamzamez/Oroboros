# Oroboros

A language for four platforms: **Go, the browser, Android and Windows**. It compiles to Go,
JavaScript, Java and LLVM IR. The output is the code a good programmer on that platform would
have written, and it calls the platform's own libraries directly.

```
fn fib(n ∈ 0..1000) (0..) {
    loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) }
}

export fn main() { io.print_line(big_str(fib(100))) }
```

```
354224848179261915075          on Go, on JavaScript, on the JVM, and on Android
```

> **Status: a working compiler, not a release.** Every claim on this page was measured or run, and the
> measurements are linked. The language still changes, nothing is versioned, and there is no package manager,
> no editor support and no stability promise.

---

## The idea in five sentences

1. **There is no runtime and no wrapper.** A program becomes ordinary host source (a Go function, a JS
   module, a Java class, an LLVM module), and a call to `os.ReadFile` *is* a call to `os.ReadFile`.
2. **Portability is a property the compiler computes**, not a promise the language makes. Use only
   what every target provides and the program runs everywhere; call a Go package and it runs on Go,
   and the compiler says so.
3. **The program is evaluated at compile time as far as it can be.** Functions, abstractions and
   compile-time data structures disappear, and what reaches the backend is loops, tables and host
   calls.
4. **What cannot be proven is refused, not checked at run time.** That covers array bounds,
   preconditions on host calls, integers leaving their range, and every claim a program makes about a
   value.
5. **A type is a set, and a claim is proven.** `0..255`, `Array(V)` and a variant are sets of values;
   the logic that reasons about them never reaches the output, and what it proves decides how each
   value is held.

The language is not small, and it is kept as small as the platforms allow: a construct goes in only
when a program needs it, with a specification of what it does on every target, and anything a target
already provides is called, not reimplemented.

---

## A tour, in six programs

The first five were compiled and run for this page, and each output shown is what they printed. The
sixth comes from the test suite, which runs it on every target.

### 1. A sieve: tables, loops and buffers

```
use io

// Mark i², i² + i, i² + 2i, … below n as composite.
fn mark(s linear, i, n) {
    loop j = i * i { j >= n -> (); else -> { s[j] := true; again(j + i) } }
}

// A table of n booleans: k is marked exactly when k ≥ 2 is not prime.
fn sieve(n) {
    var s := array { false : _ ∈ 0..n-1 }        // a buffer of n booleans, all false
    loop i = 2 {
        i * i >= n -> s                          // the buffer freezes on its way out
        s[i] -> again(i + 1)                     // indexing is application
        else -> { mark(s, i, n); again(i + 1) }
    }
}

// How many k in [2, n) are not marked.
fn count_unmarked(t, n) {
    loop k = 2, count = 0 {
        k >= n -> count
        t[k] -> again(k + 1, count)
        else -> again(k + 1, count + 1)
    }
}

export fn main() { io.print_int(count_unmarked(sieve(1000), 1000)) }
```

```
168
```

It prints the same on Go, on JavaScript and on the JVM. The four definitions cost nothing: reduction
inlines every function a program does not export, so the emitted Go is what the same program written
as one function gives. It has a `[]bool`, three plain `for` loops, no bounds-check helpers and no
wrapper, and `mark` is the inner loop. Values are numbered, not named: the code is written for Go's
compiler, and only its speed against hand-written code is held to a standard.

```go
v2 := make([]bool, 1000)
var v1 []bool
v5 := v2
var v6 int = 2
for ; ; v6 = (v6 + 1) {
	v7 := (v6 * v6)
	v9 := (v7 >= 1000)
	if v9 {
		break
	}
	v10 := v5[v6]
	if v10 {
		continue
	}
	v13 := (v6 * v6)
	v15 := v5
	var v16 int = v13
	for {
		v18 := (v16 >= 1000)
		if v18 {
			break
		}
		v15[v16] = true
		v23 := (v16 + v6)
		v16 = v23
		continue
	}
	v5 = v15
	continue
}
…
```

The language's character is in there:
- **`loop` names its variables and `again` jumps back** with new values. There is no recursion, and no
  other iteration.
- **Mutation happens only to local state.** `var s := array { … }` makes a buffer, written in program
  order with `s[j] := true`; a fresh buffer is already zero, so this one costs a `make` and no loop. A
  buffer is *linear*: no two parts of a program can write the same memory, which is checked, and
  `mark(s linear, …)` borrows it. When its scope ends it freezes into an ordinary immutable table.
- **`s[i]` is an application.** A table is a function with a known, finite domain, and the compiler
  proved `i` is inside it.

### 2. Fibonacci: exact integers

The same loop, with nothing said about its result:

```
fn fib(n) {
    loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) }
}

export fn main() { io.print_int(fib(100)) }
```

```
build: main: 1 of 2 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  + [1, +inf] in a + b
  Outside its word a target's arithmetic does not compute the integer the
  program means — Go, the JVM and x86 wrap, JavaScript loses precision — so
  this is refused rather than noted (ADR 0019, ADR 0026).
```

This refusal is the point. Compiled naively, that line gives `3736710778780434371` on Go and the JVM
(wrapped) and `354224848179262000000` on JavaScript (rounded): three wrong answers from one source. An
integer is an integer, and each target says which integers its machine word holds: int64 on Go and the
JVM, plus `uint64` on Go, and ±(2⁵³−1) on JavaScript. The program at the top of this page says the
result is unbounded, `(0..)`, and prints `354224848179261915075` on all three.

Each host uses its own big integer: `*big.Int` on Go, `BigInt` on JavaScript, `BigInteger` on the JVM
and on Android. A range chooses how a table is stored: `Array(0..255)` is a `[]byte` on Go and a
`byte[]` read unsigned (`b & 0xFF`) on the JVM, whose `byte` is signed. Above the machine word, the
target picks the representation.

**So legality is per target, and the compiler reports it.** A product of two numbers up to 3·10⁹ fits
a 64-bit word and not a JavaScript number:

```
fn area(w ∈ 0..3000000000, h ∈ 0..3000000000) (Int) { w * h }
```

```
$ go run ./cmd/portable area.oro
  go       accepted
  js       refused   gen-main: 1 of 1 integer operation(s) cannot be proven to stay inside the word of target js, [-9007199254740991, 9007199254740991]
  java     accepted
  windows  refused   area.oro: in main: io.print_int is not bound — it is not a parameter, not a definition, and not a primitive on this target
portable to go, java — not js, windows
W(go, java) = [-9223372036854775808, 9223372036854775807]   (the meet of their words: derived, not assumed — ADR 0026)
```

Nothing is wrong on JavaScript: it simply isn't a target this program runs on, and the compiler says
so instead of rounding. (Windows refuses for a different reason: its `io` module has no `print_int`.)

### 3. A line counter: calling a host API that can fail

From [examples/io/wc.oro](examples/io/wc.oro), its comments left out:

```
use os
use io
use result

cap_in = 2^24

export fn main() {
    av = os.Args()
    if len(av) < 2 { io.print_line("usage: wc FILE"); 0 } else {
        case os.ReadFile(av[1]) {
            result.'err(k) -> { io.print_line("wc: cannot read that file"); 0 }
            result.'ok(src) ->
                case {
                    len(src) >= cap_in -> {
                        io.print_line("wc: file is larger than this tool accepts")
                        0
                    }
                    else -> loop i = 0, lines = 0 {
                        i >= len(src) -> io.print_int(lines)
                        else ->
                            again(i + 1,
                                if lines < cap_in {
                                    if src[i] == 10 { lines + 1 } else { lines }
                                } else { lines })
                    }
                }
        }
    }
}
```

```bash
go run ./cmd/build -target=go   -o wc.exe     examples/io/wc.oro
go run ./cmd/build -target=js   -o wc.mjs     examples/io/wc.oro
go run ./cmd/build -target=java -o wc-classes examples/io/wc.oro
./wc.exe docs/decisions/0001-parasite-model.md
node wc.mjs docs/decisions/0001-parasite-model.md
java -cp wc-classes Main docs/decisions/0001-parasite-model.md
```

All three print `49`, the same as `wc -l`. (Its JVM build also ran on Android's runtime, ART, in the
emulator: [android-2026-10-05](gauntlet/results/android-2026-10-05.md).)

- **A failure is a value.** `os.ReadFile` returns a `result`, and the bytes exist only in its `'ok`
  arm, so a failed read cannot be counted. Its error is a *kind* a program can act on, `not-found`,
  `exists`, `permission` or `other`, the same kind on every host however that host fails
  ([ADR 0040](docs/decisions/0040-a-failure-is-a-value-of-a-marked-sum.md)). On Go the `case` compiles
  to `src, err := os.ReadFile(av[1])` and `if err == nil`, with no result value built.
- **An error cannot be dropped.** A `result` is *relevant*: every path must use it. Write
  `os.ReadFile("go.mod")` as a statement and the program does not compile:

  ```
  build: the result of os.ReadFile is a value of a relevant type (spec/errors.md §7), and it is discarded: use it, or discard it with ignore(…)
  ```

  `case` takes a result apart, `try` passes its failure on, `expect` gives up with a reason, and
  `ignore` discards it on purpose
  ([ADR 0043](docs/decisions/0043-relevance-is-checked-where-a-binder-meets-its-value.md)).
- **`case { … }` with guards erases to the nested `if`s it means**, so it costs nothing.
- **`cap_in` is a value, not a function**, and it is not decoration: a count over an unbounded file
  cannot be proven to stay in range, so the program states how large a file it accepts.

**On JavaScript, `os` is Node's for now**: `ReadFile` is `fs.readFileSync` and `Args` is
`process.argv`. A browser has no files and no command line, so this program will not be portable to the
browser, and the compiler will say so. The browser is the JavaScript target; Node is a layer a third
party can add ([ADR 0039](docs/decisions/0039-the-targets-are-go-windows-android-and-the-browser.md)).

### 4. Local variables: cells

The same pass over a file, counting lines, words and bytes as POSIX defines `wc`'s
([examples/io/count.oro](examples/io/count.oro)). The state is three local variables:

```
var lines := 0
var words := 0
var in_word := false
loop i = 0 {
    i >= len(src) -> {
        io.print_int(lines)
        io.print_int(words)
        io.print_int(len(src))
    }
    else -> {
        b = src[i]
        if b == 10 && lines < cap_in { lines := lines + 1 }
        case {
            space?(b) -> { in_word := false }
            in_word -> ()
            words < cap_in -> { in_word := true; words := words + 1 }
            else -> ()
        }
        again(i + 1)
    }
}
```

```
49
403
2562
```

A `var` whose initial value is not a table is a **cell**: read by its name, written by `x := v`, in
program order. It is Idealized Algol's `new` and Haskell's `runST`: state nothing outside the scope can
see, so the scope is still a function of its inputs. The compiler translates every cell into loop
variables before anything else sees the program, so a cell costs exactly what a loop variable costs
([ADR 0047](docs/decisions/0047-local-state-is-a-scope-and-a-cell-is-a-loop-variable.md),
[local.md](docs/spec/local.md)).

Go, JavaScript and the JVM print the same. GNU `wc` says 398 words here, not 403: it counts only runs
holding a printable byte, and the five it skips are UTF-8 punctuation between spaces. Both definitions,
computed over the bytes, give the two counts exactly.

### 5. `encoding/hex`: a host package, with its preconditions

```
use go/encoding/hex
use os
use io

// A buffer of exactly 2·len(src) bytes, filled by Go's hex.Encode, frozen on the way out.
fn encode(src) {
    var buf := array { 0 : _ ∈ 0..2 * len(src) - 1 }
    n = hex.Encode(buf, src)
    buf
}

export fn main() {
    src = array { 104, 105, 33 }                     // "hi!"
    io.print_line(os.text_of(encode(src)))
}
```

```
686921
```

Go's `hex.Encode` panics if its destination is too short. Here it cannot be called with one. Make the
buffer `0..len(src)`, one byte more than the source, and the program does not compile:

```
build: main: `go/encoding/hex`.Encode requires -len(buf) + 2*len(src) <= 0, which does not follow
```

The declaration is written by hand from the package's source and checked against the real package. A
module's path is the **host's own path**, and a type's methods are a module inside it:

```
module go/encoding/hex {
    fn Encode(dst ∈ Array(0..255) linear, src ∈ Array(0..255)) (n ∈ 0..9223372036854775806)
        where 2 * len(src) <= len(dst) ensure len(dst') == len(dst) && n == 2 * len(src) {
        host expr "hex.Encode(${dst}, ${src})" import "encoding/hex"
    }

    module InvalidByteError {
        fn Error(self ∈ InvalidByteError) (String) pure {
            host expr "${self}.Error()" import "encoding/hex"
        }
    }
}
```

The header is the caller's view, read the way the call is written. `dst ∈ Array(0..255) linear` lends
the buffer to the host, and since no `dst'` is among the results, the host writes it **in place** (a
write-borrow); `len(dst')`, the buffer after the call, keeps its length. An append, which may hand back
new storage, says so with a result `dst' ∈ Array(0..255) linear`
([ADR 0070](docs/decisions/0070-a-header-is-the-callers-view-of-a-borrow.md)). A host call that only reads a buffer and returns numbers, such as `utf8.Valid` or
`bytes.IndexByte`, reads a live buffer in place, with no copy and no declaration to say so
([ADR 0069](docs/decisions/0069-an-observer-at-a-host-parameter.md)).

The same program built for JavaScript stops with `` `go/encoding/hex`.Encode is not bound ``. That is
portability being computed: this program is a Go program, and the compiler says so.

### 6. Variants and `case`

From the test suite, where it runs on all four targets:

```
set Result = 'ok(Int) ∪ 'err(Int)

fn step(n) { if n >= 10 { 'err(n) } else { 'ok(n * 2) } }

fn run(n) { case step(n) { 'ok(v) -> v; 'err(e) -> e + 1000 } }
```

A variant is a union of tagged sets. One whose tag is known at compile time disappears. One decided at
run time becomes the `if` that decided it: no tag, no allocation, no dispatch.

---

## The language on one page

The full, current description is [docs/spec/state.md](docs/spec/state.md), and the surface's grammar is
[docs/spec/j.md](docs/spec/j.md). Every word the compiler knows is listed in
[docs/spec/inventory.md](docs/spec/inventory.md), and a test fails if the two drift apart.

| | |
|---|---|
| **Terms** | seven kinds underneath: name, integer, float, string, `true`/`false`, `fn`, application |
| **Declarations** | `fn f(x ∈ S, …) (R) where φ ensure ψ { e }`, a header carrying its claims and one body; a constant `c = e`; `set S = …` for a set or a variant; `use`, `export`, nested `module`s. A file is the module its path names |
| **Blocks** | `{ x = e; (a, b) = p; e₁; body }`, sequential bindings and statements, erased before reduction |
| **Choice** | `if c { a } else { b }`, `case { φ -> a; else -> b }`, `case e { 'tag(x) -> … }`, `&&`, `\|\|`, `!`, `!=` |
| **Iteration** | `loop x = z, … { φ -> e; …; else -> e }` with `again(…)`. No recursion, and termination is checked |
| **Data** | tables `Array(V)` with `t[i]`, `array { 1, 2 }` and `array { e : k ∈ 0..n-1 }`; maps `Map(Int, V)`; tuples `(a, b)` and the unit `()`; variants `'a ∪ 'b(T)` with type arguments; strings as sequences of Unicode scalars |
| **Local state** | `var x := e`: a table makes a buffer, written `b[i] := v` in program order and linear; a map former a map buffer; anything else a cell. A parameter `b ∈ Array(V) linear` borrows a buffer. A host may write a cell where its declaration says `inout`, and `f(out S)` passes a cell for one call |
| **Claims** | a binder's set, `x ∈ 0..255`, and `where` are obligations at every call; `b ∈ S = e` claims a value's set; each is proven or the program is refused |
| **Failures** | a value of a sum: `lib/result`'s `result(T, E)`, met by `case`, `try`, `expect` or `ignore`, and *relevant*, so it cannot be dropped silently. A host's own encoding (Go's `(T, error)`, a throw) is translated once, at its declaration |
| **Integers** | an integer is an integer; each target declares the word it holds (int64 on Go, the JVM and Windows, plus `uint64` on Go; ±(2⁵³−1) on JS), an operation not proven inside it is refused on that target, and which targets accept a program is reported; a range `0..255` is a type; `0..`, its upper end omitted, is arbitrary precision |
| **Functions** | fully higher-order at compile time; nothing that needs a closure at run time may survive to the output |
| **Effects** | one purity bit per host call; an impure call runs exactly once, where it was written |
| **Host calls** | under the host's own names: `fmt.Println(a, b)` is Go's variadic call, and `f(xs...)` passes a table as its list. A host's string is the host's type, and a `String` is always valid text |
| **Targets** | directories of declarations, which are data and never compiler code |

## What the compiler guarantees

| | how | measured |
|---|---|---|
| Emitted code is as fast as hand-written | seven benchmark programs held against hand-written Go, JavaScript, Java, and C on Windows by the same compiler | geometric mean of generated over hand-written **1.001** on Go, **0.965** on JavaScript, **1.012** on Java, **0.978** on Windows, over 45 comparisons from **0.85×** to **1.18×**; each above 1.05× explained, the largest a JavaScript module boundary ([gauntlet-2026-11-16](gauntlet/results/gauntlet-2026-11-16.md)). Windows runs five of the seven: word count needs a dictionary keyed by strings, the report printing of floats. On Android's runtime, in the emulator, 9 of 10 pairs are at parity and the stencil is 1.17–1.19× ([android-2026-10-05](gauntlet/results/android-2026-10-05.md)). The newest program, a table-shape tool reading its own 64 KB buffer, is at 1.11× its hand-written Go ([observer-2026-10-21](gauntlet/results/observer-2026-10-21.md)) |
| An integer never silently wraps or rounds | an interval analysis on the IR, against each target's word; unproven means refused on that target | **11,779 of 11,833** operations proven across the corpus, every quotient included; the rest are refused by design ([examples/int/](examples/int/)) or proven where the test harness builds them |
| Array indices and host preconditions hold | linear-arithmetic proofs, including facts about what a table holds | the JSON tree walker runs with **no bounds clamps** at 1.06× of hand-written unclamped Go ([compfacts](gauntlet/results/compfacts-2026-09-17.md)) |
| A function is only called inside its declared domain | every declared parameter range and `where` is proven at every call, above the machine word as the set its enforcement decides, by sign and bit length ([ADR 0028](docs/decisions/0028-a-definitions-contract-is-checked-at-its-calls.md)) | 346 range obligations in the corpus measured before it was switched on; one program was missing a declaration ([requires](gauntlet/results/requires-2026-09-24.md), [contracts](gauntlet/results/contracts-2026-09-24.md)) |
| A string is always text | every value typed `String` is a sequence of Unicode scalar values: a host's string is its own type (`go.bytestring`), entered only by the standard's decode, which is the same function on every host ([ADR 0030](docs/decisions/0030-a-hosts-string-is-the-hosts.md)) | 57 Go declarations read one at a time; `text_of` agrees on Go, JS and Java over 18 ill-formed sequences ([hoststring](gauntlet/results/hoststring-2026-09-24.md)) |
| Loops terminate | size-change termination on the IR | **1,351 of 1,644** loops proven; the rest are driven by input or the file system, or are loops over numbers past the machine word whose progress (a halving, a bit cleared) the check does not see |
| Every host agrees | 54 programs built and **run** on every target that accepts them (39 on all four), required to print the same, correct answer | [gauntlet/differential/](gauntlet/differential/) |
| Each rule that decides legality is tested against a broken copy of itself | a fault is planted in the rule, and its test must fail | each result in [gauntlet/results/](gauntlet/results/) lists its faults and the test that caught each |
| The compiler's output never drifts by accident | every emitted file, proof count and error message compared with a committed baseline | `go run ./cmd/check` |

## How much of each platform it can reach

Surveys read each host's own API list and count what the declaration format can express:

| host | declarable | usable today |
|---|---:|---:|
| Go standard library | 87.8% | 60.5% |
| JVM (JDK) | 81.4% | 60.9% |
| Win32 | 91.1% | 32.0% callable *and* linkable |
| JavaScript (Node's `globalThis`) | 100% | not a meaningful number: every value has one type. The browser's Web APIs are not surveyed yet |

"Declarable" is not "supported". A package is supported when every function is declared **by hand**,
with its preconditions and what it does to buffers, and checked against the real package
([ADR 0022](docs/decisions/0022-host-declarations-are-written-by-hand.md)). Today that is
`unicode/utf8`, `encoding/hex`, `strconv`, `encoding/binary`'s varints and `math/bits`, plus `io`'s
interfaces. `os`, `strings`, `io`, `fmt`, `bufio`, `errors`, `io/fs`, `path/filepath`, `time` and
`bytes` are partly declared, every name checked. Of the 590 exported names in the ten Go packages
the coverage tool tracks, 309 are declared (52.4%), and every missing one has a recorded cause
(`go run gauntlet/stdlib/coverage.go`). Each package comes in with a program that needs it:
[lib/num/u128.oro](lib/num/u128.oro) over `math/bits` and `strconv`,
[examples/os/ls.oro](examples/os/ls.oro) over `os` and `time`,
[examples/lines/shape.oro](examples/lines/shape.oro) over `bytes`.

## What doesn't work yet

- **Most of each standard library** is not declared by hand yet.
- **No recursion.** Balanced divide-and-conquer (merge sort, Karatsuba) is a loop over levels, and
  recursive data is a flat table with indices. It is fast, and it is more to write.
- **No runtime closures, no concurrency, no interfaces you implement.** You can pass a host object where
  a host interface is wanted; you cannot build one. Callbacks are specified and not built, so a window
  procedure, a sort comparator handed to the host, or a function reading a cell cannot be passed
  anywhere yet.
- **No borrowed results.** A host call that lends its own buffer until the next call (`bufio`'s
  `Peek`, `Scanner.Bytes`) is not declared; the research and the design are in
  [borrow-research.md](docs/borrow-research.md).
- **Out-parameters on Go only, and not yet typed by the call.** Go's Scan family stores through
  cells, but `errors.As`, whose result names its cell's type, needs a type variable, and Win32's
  out-cells need a stack slot on Windows.
- **Strings are thin:** concatenation and conversion at a boundary. Text programs so far work in bytes.
- **Maps take integer keys only**, and there are no records yet.
- **The browser's own API**, the Web platform, is not declared yet, and the JavaScript output is
  tested under Node, as an engine.
- **Windows** is the least complete target: its `io` has no `print_line`, no file I/O, and no
  integer without a declared bound, since it has no host bignum: a bounded one is computed and printed
  in fixed limbs, written in the language. Its doubles are the language's roundings onto
  F64 (since 2026-11-06); a host's own double, as elsewhere, is not a type yet.
- **No packaging, no editor support**, and error messages written for the compiler's authors rather
  than for newcomers.

## Current work

*Updated with the plan; last updated 2026-11-19.* The latest assessment,
[docs/assessment-2026-11-19.md](docs/assessment-2026-11-19.md), scores this plan and the decisions
built on 11-19, and proposes the next plan, which hamza has not yet approved. **The plan in force**
(approved by hamza, 2026-11-19), in order:
1. **TinyRetroPad's body in the language**, module by module, the window procedure last. The window
   procedure itself is now possible ([slot-2026-11-19](gauntlet/results/slot-2026-11-19.md)).
2. **The callback decisions**: the window procedure's are taken and built; those for a function a
   Windows or Go call runs only while it lasts are still to discuss.
3. **The capacity facts** of maps: the refusals named last round prove now, so this waits for a
   program that fails.
4. ~~**A time limit on every program a test runs**~~ **done**
   ([timeouts-2026-11-19](gauntlet/results/timeouts-2026-11-19.md)).
5. **The project's instructions to its assistant**, rewritten as the state of things, not a story.
6. **The words that bought nothing**: a first pass is done ([words-audit](docs/words-audit.md)).

**To come back to, with hamza**: a scope form, `f(a…) { x̄ -> b }`, a block after a call for a resource
taken at its start and given back at its end (docs/scopeforms-research.md); callbacks a host calls only
during the call; how a variadic of places is written.

Decided on 11-19 and built since: one spelling for a parameter the host writes back, `x ∈ T inout`
(ADR 0135); benchmarks that time the two forms side by side; 32-bit Windows as the check's second
tier; a remainder by a constant compiled as C compiles it (ADR 0138); a false claim on a value
nothing reads refused (ADR 0137). A nested loop followed by a step of its enclosing loop (ADR 0134)
turned out not to be buildable as decided, and the 128-bit integers in the language's own
operations wait on a wall; both are set aside for hamza. A wrong answer was found and stopped:
32-bit and 64-bit Windows printed only the last 20 digits of a very large integer.

**The 11-17 plan, as it ended** (hamza, 2026-11-17), in order:
1. ~~**a compilation that never finishes is refused**~~ **done, the other way round**
   ([noend-2026-11-17](gauntlet/results/noend-2026-11-17.md)): measured, every compilation does
   finish. The twenty minutes came from a library function the compiler had not proven on 32-bit
   Windows. Such a function is now checked on every target it serves, before any program meets it;
2. ~~**tell LLVM a digit's range where it is read**~~ **measured, not built**
   ([rangeload-2026-11-17](gauntlet/results/rangeload-2026-11-17.md)): the 32-bit compiler LLVM
   provides ignores that fact, and making the multiplication what Go's is bought 4%. Big integers on
   32-bit Windows are slow for another reason, still to be measured;
- **added, at hamza's word**: ~~measure where the time goes~~ **done**
  ([alloc-2026-11-17](gauntlet/results/alloc-2026-11-17.md)): a fresh, zeroed allocation at every
  step of a big-integer loop. ~~Both causes fixed~~ **done**
  ([filltail-2026-11-17](gauntlet/results/filltail-2026-11-17.md),
  [reusecall-2026-11-17](gauntlet/results/reusecall-2026-11-17.md)): a result the compiler proves
  fully written is no longer zeroed first, and a value's memory freed at one step now holds the
  next step's result, so 20000! allocates 22 times instead of 20,000;
3. ~~**the Windows API by header**, with TinyRetroPad's own calls declared that way~~ **done**
   ([tinyretropad-2026-11-17](gauntlet/results/tinyretropad-2026-11-17.md),
   [byheader-2026-11-17](gauntlet/results/byheader-2026-11-17.md)): of the 67 functions
   TinyRetroPad imports, 47 can be called today, and each is declared as its header writes it and
   checked against Microsoft's own prototype; one program calls 17 and prints what C prints, on
   32-bit and 64-bit Windows alike. The other 20 need structures, a callback or a variable
   argument list (item 6). On the way, the survey of the Windows API learned to see 802 functions
   it had missed;
4. ~~**32-bit integers** on 32-bit Windows~~ **done**
   ([d5-2026-11-17](gauntlet/results/d5-2026-11-17.md)): a number the compiler proves fits in 32
   bits is now held in a 32-bit register there, and computed at 32 bits wherever that gives the same
   answer. Every program measured on 32-bit Windows got faster, from 12% to 41%, and none changed its
   output. 64-bit Windows was measured too; it doesn't help there yet, so it stays as it is;
5. ~~**the remaining facts for LLVM**, one kind at a time~~ **measured, not built**
   ([phi5-2026-11-17](gauntlet/results/phi5-2026-11-17.md)): LLVM already works out each of the
   three facts itself, or has nothing that would use them, so stating them changed no measured
   time. Each is written down for the day a program needs it;
6. **TinyRetroPad's walls**: the window procedure and its structures. The structures were
   researched first, 2026-11-17 ([records-research.md](docs/records-research.md)): a record is
   the language's, already specified, and a Windows structure is its layout on one machine,
   computed and checked against the C compiler. hamza decided it (ADR 0128: the whole record, and
   `r with { … }`). The record in the language is built
   ([records-2026-11-17](gauntlet/results/records-2026-11-17.md)), and so are Windows structures,
   passed by address or by value. Since 2026-11-18 a Windows function is called as C writes the
   call, `call WriteFile(h, s, len(s), &written, NULL)`: what the function writes back is one of its
   results, and a pointer with its count is one table, so a program writes
   `(written, ok) = WriteFile(h, b[lo:hi])` ([ccall-2026-11-18](gauntlet/results/ccall-2026-11-18.md)).
   Since 2026-11-19 a program has a window procedure: a function written in the language that Windows
   calls, with a state that survives from one message to the next and is right even when Windows
   calls the procedure again from inside itself (ADR 0133,
   [slot-2026-11-19](gauntlet/results/slot-2026-11-19.md)).

**The 11-16 plan, as it ended** ([assessment-2026-11-16](docs/assessment-2026-11-16.md) set it):
1. **fix the book's wrong answers**, each with a test that fails before the fix:
   - ~~two results taken apart in one expression are confused (it prints 22 for "42")~~ **fixed**
     ([capture-2026-11-16](gauntlet/results/capture-2026-11-16.md));
   - ~~a write past the end of a table compiles, and on Windows writes outside it~~ **fixed**
     ([store-2026-11-16](gauntlet/results/store-2026-11-16.md)): a write must now be proven inside its
     table, as a read already was; three programs whose proof is out of the compiler's reach today
     check their writes instead, at no measurable cost, and the proof that would remove those checks
     is owed;
   - ~~the size of a map is wrong on Windows~~ **fixed**, and Windows' map is now built from the
     language's own definitions after the compiler has checked the program, as every other
     Windows operation is, so programs that read a map inside a function now build there
     ([maprealize-2026-11-16](gauntlet/results/maprealize-2026-11-16.md)); ~~a map's declared
     capacity~~ **enforced** on every target, a program inserting more keys than its map holds now
     refused, where Go grew the map and Windows dropped the key
     ([capacity-2026-11-16](gauntlet/results/capacity-2026-11-16.md));
   - ~~a pair read from a table produces wrong Go~~ **fixed**;
   - ~~with them, a compiler crash, a self-check that rejects a valid program, and a test that
     fails one run in five~~ **fixed** ([bookfaults-2026-11-16](gauntlet/results/bookfaults-2026-11-16.md));
2. ~~**build two things the grammar promises and the compiler refuses**~~ **done**
   ([invariant-2026-11-16](gauntlet/results/invariant-2026-11-16.md)): a loop may state its
   invariant, which the compiler proves and then uses, so an index that only that relation bounds
   is accepted; `try` takes several results at once. The library's run-time clamp is gone, though it
   needed two facts the compiler was missing rather than the invariant;
3. ~~**two checks for this round's mistakes**~~ **done**
   ([checks-2026-11-16](gauntlet/results/checks-2026-11-16.md)): the full check now refuses a
   Windows line ending in a tracked text file (it found five), and a test that plants a fault by
   rewriting a source fails if the rewrite found nothing (it found one test that had checked nothing
   for a month);
4. **32-bit Windows**, starting with what a second processor changes (the sizes of C's types, the
   digit size of big integers), the choices brought to hamza before anything is built; with it, the
   Windows API grouped as its documentation groups it, by header, each function still naming its
   DLL. **The research is done** ([triple-research.md](docs/triple-research.md)): 63 of the 74 test
   programs already run correctly on 32-bit Windows, and the rest fail for three reasons, each
   with a choice for hamza; one measurement says 32-bit integers would make i686 loops 2.6× faster.
   **Decided** (hamza, 2026-11-17, ADR 0122) and being built. **Two steps done**: Windows is now a
   family, a shared Win32 base with one directory per processor; and **`windows32` is a target**
   ([windows32-2026-11-17](gauntlet/results/windows32-2026-11-17.md)), 66 of the 74 programs
   correct on it. On the way, a wrong answer on both processors was found and fixed: a file's path
   could run on past its end into whatever memory followed. **64-bit division on 32-bit Windows
   is the language's own** ([d6-2026-11-17](gauntlet/results/d6-2026-11-17.md)), at the four places
   LLVM calls a helper, 13% faster than Microsoft's; and **big integers use the processor's own
   digit**, 32 bits there and 64 on 64-bit Windows, from one library
   ([d2-2026-11-17](gauntlet/results/d2-2026-11-17.md)); 72 of the 74 programs run. Next, the
   Windows API by header, then 32-bit integers;
5. **tell LLVM everything the compiler proves** (that a value is never undefined, that two buffers
   never overlap, a number's range, that a loop ends), one kind of fact at a time, each checked and
   measured against C. **Four kinds done** (2026-11-17): a function's argument and result ranges,
   comparisons of numbers of one sign, a table's pointer, and a buffer no other argument shares
   (phi-range, phi-samesign, phi-ptr, phi-noalias). None changed the speed of the programs
   measured, because LLVM had derived the same facts there. Found while measuring, and set aside
   for hamza: one division by a constant runs 1.42× C's time and reaches C with a signed
   instruction;
6. **TinyRetroPad's Windows surface, then its walls**;
7. ~~**the guidance file shortened to the project's current state**~~ **done** (2026-11-16):
   CLAUDE.md is 7,761 words, from 14,242, the history left to the assessments, the results and git;
- also found and fixed, from the book: ~~a false statement about a value's range accepted when the
  range's ends were variables, or when nothing read the value~~ **fixed**
  ([claims-2026-11-16](gauntlet/results/claims-2026-11-16.md)).

**The 11-15 plan, as it stands**:
1. keep a ledger of what each step costs and what it buys;
2. ~~let the compiler keep a program's definitions as functions instead of copying them into every
   call (P8), its costs measured~~ **done 2026-11-15**
   ([p8-2026-11-15](gauntlet/results/p8-2026-11-15.md), [p8step3-2026-11-15](gauntlet/results/p8step3-2026-11-15.md)):
   on Windows the compiler now decides, by itself, which of a program's functions to keep as
   functions. Three file programs that were too large to compile there now build. No program that
   built before is refused, they run at the same speed, and the other three targets' output is
   unchanged. A kept function now carries what its body proves about its result and what it needs
   of its arguments, so only two programs still fall back to copying. What it cost is in the
   results;
3. ~~**the wrong answers the book found**~~ **done 2026-11-15** ([bookfix-2026-11-15](gauntlet/results/bookfix-2026-11-15.md)):
   each of these programs is now refused, or computed as the language says, and no program that
   built before changed:
   - `if c { 1 }` returned as an `Int` (0 when `c` is false);
   - a result declared `()` that returns a value;
   - `len(s)` of a string, three different answers on three hosts;
   - a definition's `Result` dropped unused;
   - a declaration after a `module { }` block read into it;
   - `use Geometry` finding `geometry.oro` only on Windows;
   - `true = 1` accepted and ignored;
   - an impure primitive evaluated at compile time;
4. ~~**what the grammar promises and the reader refuses, and the diagnostics**~~ **done 2026-11-15**
   ([surface-2026-11-15](gauntlet/results/surface-2026-11-15.md)): each of these now builds, or is
   refused in the program's own notation, and no program's code changed:
   - prefix `-n`;
   - a range bounded by a parameter or a constant (`i ∈ 0..len(a)-1`, `0..cap`);
   - a top-level value with a claim;
   - a module file with `_` in its name;
   - a table given by a rule returned from an export, the same boundary a segment passed to a kept
     function meets;
   - a big value printed;
   - an unread binding left in the reduced program;
   - five diagnostics quoting internal names or the old notation;
5. ~~**the book's chapters 5 and 6**~~ **done 2026-11-15** ([chapters56-2026-11-15](gauntlet/results/chapters56-2026-11-15.md)):
   - a value computed above the machine word comes back into it where it provably fits, and on
     JavaScript an export declared `Int` no longer returns a `BigInt`;
   - a product of naturals is known to be a natural, and a function with no declared result may
     return a computed pair;
   - an empty range, and JavaScript's quotient and midpoint, are refused for their true reason;
   - a function named on a line of its own is refused, since it does nothing
     ([ADR 0116](docs/decisions/0116-a-statement-whose-value-is-a-function-is-refused.md));
     `1_000_000` and `0b101` wait until a program needs them;
   - no program's code changed;
6. ~~benchmark the seven gauntlet programs on all four targets again~~ **done 2026-11-16**
   ([gauntlet-2026-11-16](gauntlet/results/gauntlet-2026-11-16.md)): every program measured is as
   fast as hand-written code on Go, JavaScript, Java and Windows. Windows is compared against C
   compiled by the same compiler. It found a memory leak there, fixed: a temporary table such as a
   parser's stack was never freed. Two programs cannot run on Windows yet: counting words needs a
   dictionary keyed by strings, and the report needs printing of floating-point numbers;
7. ~~make the test harness catch stale skips and assemble every Windows file, and run the book's
   checker in it~~ **done 2026-11-16** ([harness-2026-11-16](gauntlet/results/harness-2026-11-16.md)):
   a test that skips a target must now say why, and the check runs it anyway to confirm it still
   fails for that reason (one no longer failed at all); every Windows file is assembled; the book's
   examples are re-run on every check. On the way, four programs that return big numbers from a
   function now build on Windows, and a refusal names its real cause instead of a symptom;
8. ~~run the file programs on Windows at hand-written speed~~ **done 2026-11-16**
   ([files-2026-11-16](gauntlet/results/files-2026-11-16.md)): two of them crashed on Windows on any
   file over 8 KB, because a table was given back to a different allocator than the one that made
   it; fixed. As whole programs, all five now run faster than the same programs written by hand in
   Go, and four of five faster than C. What is left is measured and explained: counting in 64-bit
   words where C uses 32-bit ones, and sorting where Go uses a hash table;
   - ~~then the last code written as LLVM text in the Windows layers~~ **done 2026-11-16**
     ([strings-2026-11-16](gauntlet/results/strings-2026-11-16.md),
     [cabi2-2026-11-16](gauntlet/results/cabi2-2026-11-16.md)): the string operations are now
     written in the language, and Win32's file reads and writes are derived from their C
     prototypes. Every remaining host template is a single instruction. Doing it found a wrong
     answer (a character code outside Unicode accepted on Windows) and three gaps in the checks,
     all fixed; one global counter shared by every write became a per-call one;
9. 32-bit Windows (carried: item 4 above);
10. TinyRetroPad's Windows surface, then its walls (carried: item 5 above).

The 10-24 plan, as it stands:

1. ~~**The documents tell the truth.**~~ **Done 2026-10-25**
   ([docs-2026-10-25](gauntlet/results/docs-2026-10-25.md)): the grammar's status says what is built and
   what is not, the language's summary is in today's syntax, and the inventory of words separates what a
   source file says from the core's forms below it. The grammar's keyword list and the inventory's counts
   are now checked against the code, and the files the benchmarks call are regenerated and compared on
   every check, so none of them can go stale unseen.
2. ~~**Two questions of the surface**~~ **Done 2026-10-25**
   ([surface-2026-10-25](gauntlet/results/surface-2026-10-25.md)): a range with no upper end is written
   `0..`, the end left out, in place of `` 0..`+inf` `` ([ADR 0077](docs/decisions/0077-an-unbounded-end-is-omitted.md));
   and a declaration is private unless marked `pub`, while `export` still marks what the build compiles
   ([ADR 0078](docs/decisions/0078-a-modules-interface-is-what-it-marks-pub.md)).
3. ~~**Three defects the audit of item 1 found**~~ **Done 2026-10-26**
   ([defects-2026-10-26](gauntlet/results/defects-2026-10-26.md)): a program may no longer redefine one
   of the language's own names, which inside a module had silently changed what `if` meant; the
   recursion error names the language's loop; and writing a set where a value belongs, such as
   `x ∈ 0..9`, is refused with a clear reason instead of a garbled one.
4. ~~**A declared range stops choosing how a number is stored**~~ **Done 2026-10-26**
   ([claimset-2026-10-26](gauntlet/results/claimset-2026-10-26.md)): the compiler picks the machine word
   or a big integer from what it proves about each value, and a declaration only states the set, so one
   function over `Nat` (the natural numbers, now named) runs in the word where its arguments are small and
   on a big integer where the host hands it one
   ([ADR 0079](docs/decisions/0079-a-declared-set-is-a-claim-and-the-facts-choose-the-representation.md)).
5. ~~**This project's guide for Claude, CLAUDE.md, rewritten**~~ **Done 2026-10-26**: it states where
   the project is in about a third of its former length, and reaches the history by link.
6. **The Windows application**: [TinyRetroPad](https://github.com/plummersSoftwareLLC/TinyRetroPad), a
   Notepad-style editor in about 2.5 KB of hand-written x86 assembly, the standard the Windows target
   will be held to. In steps:
   - ~~first, whether Windows keeps targeting MASM or targets LLVM IR~~ **decided 2026-10-26: LLVM IR**
     ([ADR 0080](docs/decisions/0080-the-native-targets-print-llvm-ir.md),
     [backend-2026-10-26](gauntlet/results/backend-2026-10-26.md)). The same program printed as MASM, C and
     LLVM IR: LLVM IR was faster (up to 2.9× on a merge sort) or smaller (half the code) than our MASM,
     and it reaches 32-bit Windows, ARM, Linux, macOS and WebAssembly from one printer;
   - ~~every LLVM primitive surveyed~~ **done 2026-10-27**
     ([llvmsurvey-2026-10-27](gauntlet/results/llvmsurvey-2026-10-27.md)): of LLVM's 67 instructions and
     534 general intrinsics, 31 and 83 can be offered today; the rest meet seven questions the language
     has not answered, each decided by hamza in turn, with a research document first:
     1. ~~**wrapping integers and bits**~~ **decided 2026-10-27**
        ([ADR 0082](docs/decisions/0082-wrapping-is-the-residue-map-and-the-bits-are-the-integers.md)): the
        bitwise operations become the language's on every target, and a wrapping result is written
        `mod 2^64`; **built 2026-10-27** with step 2;
     2. ~~**integer widths**~~ **decided 2026-10-27**
        ([ADR 0083](docs/decisions/0083-a-representation-is-an-abstraction-function-chosen-by-place.md)):
        a width is only how a target holds an integer, never part of what it is, so no choice of width can
        change an answer; each is chosen by where the value lives. **Built 2026-10-27**
        ([bits-2026-10-27](gauntlet/results/bits-2026-10-27.md)): two programs, the FNV and CRC-32
        hashes of a file, give Go's own answers on Go, Java and JavaScript at the speed of hand-written
        code, and Java and JavaScript now hold a table of bytes as bytes; and since 2026-10-28
        ([pairs-2026-10-28](gauntlet/results/pairs-2026-10-28.md)) JavaScript holds a 64-bit unsigned
        number as two 32-bit halves, so the 64-bit hash runs there too, faster than a hand-written one;
     3. ~~**pointers**~~ **decided 2026-10-30**
        ([ADR 0084](docs/decisions/0084-an-address-is-an-integer-and-ownership-is-a-view.md)): an address
        is an integer, and what a program may do with the memory behind it is decided by a claim the
        compiler checks and then erases, so everything C does with pointers has a form here without
        C's memory bugs; **its portable part built 2026-10-30**
        ([views-2026-10-30](gauntlet/results/views-2026-10-30.md)): a program that reads a Windows
        executable's headers and sets its checksum, matching Go's own reader and Windows' own checksum on
        Go, Java and JavaScript;
     4. ~~**vectors**~~ **decided 2026-10-30** ([ADR 0085](docs/decisions/0085-a-vector-is-a-representation-of-a-table-and-a-fold-vectorizes-iff-it-is-a-homomorphism.md)): several numbers in
        one register, and GPUs, are a table the hardware computes all at once; the compiler does it
        wherever the answer cannot change, which is every integer sum but not a floating-point one;
        built with the LLVM backend;
     5.–7. ~~float formats other than 64-bit, operations LLVM allows to give more than one answer, and
        rounding modes~~, **decided 2026-11-01** ([ADR 0086](docs/decisions/0086-a-number-is-an-element-of-z-tenth-and-a-rounding-is-its-one-lossy-operation.md),
        [floats-research.md](docs/floats-research.md)), with decimals and how `0.1` is read; **decimals
        built 2026-11-01** ([decimals-2026-11-01](gauntlet/results/decimals-2026-11-01.md)): a ledger
        program sums prices and computes tax exactly, to the cent, with the same answer on Go, Java and
        JavaScript as hand-written Go; **binary floating point built 2026-11-01**
        ([doubles-2026-11-01](gauntlet/results/doubles-2026-11-01.md)): the gauntlet's dot product, centroid
        and search are one source each for Go, Java and JavaScript, each operation IEEE's, at the speed of
        hand-written code. Exact sums and products inside a program, rounded once, wait until a program needs
        them (2026-11-02). What the numbers are:
        - integers, decimals and floating-point values become one kind of number, with exact
          arithmetic, so `0.1 + 0.2 == 0.3`;
        - rounding is the one operation that loses information, written where the program wants it,
          to any precision, the small formats machine learning uses (FP8, FP4) included;
        - each answer is the same on every target;
   - ~~then the LLVM IR backend itself, until it builds everything the MASM one does, which then
     retires~~ **done 2026-11-03** ([llvm-2026-11-02](gauntlet/results/llvm-2026-11-02.md),
     [mastretire-2026-11-03](gauntlet/results/mastretire-2026-11-03.md)): the Windows target prints LLVM
     IR, runs every test program with the same answers as the other targets, and runs the benchmark
     sieve in 0.64 of the time of hand-written assembly; the MASM backend is deleted, and the programs
     that named x86 instructions are written in the language's own operators. LLVM's other operations
     are declared as programs need them, TinyRetroPad's first. **Strings to Win32, done 2026-11-04**
     ([cstring-2026-11-04](gauntlet/results/cstring-2026-11-04.md)): a string passed to Windows must
     hold no NUL character, which the compiler now proves or refuses, and Windows reads its bytes as
     UTF-8; and the memory a table lives in is obtained by a program in the language's own terms,
     whose size arithmetic is checked like any other;
   - **then the LLVM target, fully, before the Windows app** (2026-11-04): the LLVM backend is its own
     layer that every LLVM-based target shares, with Windows one of them
     ([layers-2026-11-04](gauntlet/results/layers-2026-11-04.md), done); Windows' functions described as
     C prototypes, so each call is derived rather than written ([cabi-2026-11-05](gauntlet/results/cabi-2026-11-05.md),
     done 2026-11-05); then every test
     program running on Windows (the 64-bit unsigned numbers, which the hashes need, since 2026-11-06,
     [llvm64-2026-11-06](gauntlet/results/llvm64-2026-11-06.md)), and the roundings of IEEE doubles since the
     same day ([llvmf64-2026-11-06](gauntlet/results/llvmf64-2026-11-06.md)), and printing integers too large for
     a machine word since 2026-11-07 ([llvm-render-2026-11-07](gauntlet/results/llvm-render-2026-11-07.md));
     next **memory on Windows** (2026-11-09, ADR 0096): each table is now released when it dies
     ([release-2026-11-09](gauntlet/results/release-2026-11-09.md): a loop that held 2.7 GB holds 3.5 MB),
     and its storage handed to the next table of its size
     ([reuse-2026-11-09](gauntlet/results/reuse-2026-11-09.md): the same loop in 0.11× its old time),
     and not zeroed when every element is written before any is read
     ([fill-2026-11-09](gauntlet/results/fill-2026-11-09.md): 0.08×); the rest waits for programs that read
     input; **signed integers above the word** on Windows since 2026-11-09, as a sign over the
     magnitude, every operation derived from the unsigned ones
     ([ADR 0098](docs/decisions/0098-fixed-limbs-realize-a-signed-set-by-the-group-completion.md),
     [signlimbs-2026-11-09](gauntlet/results/signlimbs-2026-11-09.md)), now 1.55× unsigned
     ([signfuse-2026-11-09](gauntlet/results/signfuse-2026-11-09.md)); **integers of any size** on
     Windows since 2026-11-10, numerals in base 2³² written in the language
     ([ADR 0099](docs/decisions/0099-an-unbounded-integer-is-a-numeral-of-any-length-with-its-signed-size-last.md),
     [natnum-2026-11-10](gauntlet/results/natnum-2026-11-10.md)): fib(100,000) agrees with Go's and
     Python's, at about 8× Go's `math/big`; division by a big integer since the same day
     ([natdiv-2026-11-10](gauntlet/results/natdiv-2026-11-10.md)), Knuth's algorithm, 3.1× `math/big`;
     and a dead table's memory reused by the next one it fits since the same day
     ([ADR 0100](docs/decisions/0100-a-dead-region-serves-a-request-it-fits.md),
     [fitreuse-2026-11-10](gauntlet/results/fitreuse-2026-11-10.md)), which showed the memory was
     not where the time goes: the work per digit is, and **the goal is Windows faster than Go**
     (hamza, 2026-11-10), so those causes are owed; the first paid 2026-11-11
     ([llvm-udiv-2026-11-11](gauntlet/results/llvm-udiv-2026-11-11.md)): a division of
     non-negative numbers now uses the processor's unsigned division, which needs no sign
     correction, and multiplying big integers takes half its time, 1.4× Go's; printing them is
     what is left; next, digits of 63 bits instead of 32, which needs a product of two digits held
     in 128 bits: built the same day
     ([doubleword-2026-11-11](gauntlet/results/doubleword-2026-11-11.md)), and a hash modulo a
     large prime now runs on Windows in half the time of idiomatic Go; and then the digits
     themselves ([natword-2026-11-11](gauntlet/results/natword-2026-11-11.md)): dividing big
     integers is now faster on Windows than in Go, and adding, multiplying and printing them about
     twice as fast as before; since 2026-11-12 each digit is a full 64-bit word
     ([nat64-2026-11-12](gauntlet/results/nat64-2026-11-12.md)), which makes the inner
     multiplication loop the machine's own, and since then memory is handed out in power-of-two
     sizes ([class-2026-11-12](gauntlet/results/class-2026-11-12.md)), which brings multiplying a big
     integer by a word level with Go; since 2026-11-13 each digit of a sum is the processor's own
     add-with-carry, the carry kept in its flag
     ([carry-2026-11-13](gauntlet/results/carry-2026-11-13.md)), which takes a third off adding
     big integers (Fibonacci's 100,000th number 1.23× Go's time, from 1.9×); since 2026-11-14 a
     product of two long numbers is Karatsuba's
     ([karatsuba-2026-11-14](gauntlet/results/karatsuba-2026-11-14.md)), which takes the millionth
     Fibonacci number by doubling from 4.1× Go's time to 1.67×; and since the same day a square takes
     about half a product's work ([squaring-2026-11-14](gauntlet/results/squaring-2026-11-14.md)),
     1.57×; per operation it is still 3–4× Go's, the inner loop of a product
     ([kernels-research.md](docs/kernels-research.md)): that loop can be written in the language,
     and LLVM cannot keep its two carries in two flags, so a target that names its CPU will print it
     as assembly. That is decided and postponed; since 2026-11-15 big numbers are printed by divide
     and conquer, each division done in place
     ([radix-2026-11-15](gauntlet/results/radix-2026-11-15.md)), which takes 20,000! from 2.8× Go's
     time to 1.3× and printing the millionth Fibonacci number from 502 ms to 106; since the same
     day files and the command line work on Windows too
     ([oswin-2026-11-15](gauntlet/results/oswin-2026-11-15.md)), written in the language itself
     over the Windows API, but a program calling all of them at once is still too large for the
     compiler's inliner, which waits on keeping such definitions as functions; then the
     benchmarks that read files, at hand-written speed;
     then the test harness re-running what it skips, so that a skip whose reason is no longer true is
     found, assembling every Windows file it writes, and comparing the Windows text of the
     big-integer library, which no compared file holds today; then 32-bit Windows;
   - then what TinyRetroPad needs from Win32 against what is declared, callable and linkable today;
   - then its walls (a window procedure, Win32's structs, 32-bit code), each specified first.
7. ~~The IR's verifier checks that each branch justifies the facts it hands its arms~~ **done
   2026-11-02** ([pijustified-2026-11-02](gauntlet/results/pijustified-2026-11-02.md)): every fact an
   arm is handed must follow from the condition that enters it, so the LLVM backend can trust each one.
8. ~~Two defects found while building the LLVM backend~~ **done 2026-11-03**: the compiler refused a
   fact it had already established in one program (fixed 2026-11-02); and dividing the most negative
   integer by −1, the one quotient past the largest integer, is now refused unless the program rules it
   out, or trapped under `-checked`, on every target
   ([quotient-2026-11-03](gauntlet/results/quotient-2026-11-03.md)).
9. **A loop that continues after a nested loop's result is taken apart**, which `wc` needs to be written
   as Go is, and which costs it 16% today.
10. **Go's standard library, program first**: next, `cmp`, comparing two files chunk by chunk.
11. **A slowdown in the generated JSON tokenizer on the JVM**, which runs at hand-written speed for its
    first second and then three times slower, found while measuring step 2 above.
12. **When the compiler copies a function into each place it is called, and when it calls it**:
    decided 2026-11-11 ([ADR 0102](docs/decisions/0102-factoring-never-changes-an-answer-or-a-verdict.md),
    [specialization-research.md](docs/specialization-research.md)). The choice never changes what a
    program computes, only what the compiler can prove about it, how large the code is, and how long it
    takes to compile and run; and it may never change whether a program is accepted, which the
    compiler now guarantees for the functions it already keeps. Measured the same day
    ([factoring-2026-11-11](gauntlet/results/factoring-2026-11-11.md)): in today's programs, keeping
    the functions that declare their argument ranges would save about 3% of the code, too little to
    build on yet; most of those functions are proven correct on their own, and every one that is not
    has a reason the research predicted.

## Try it

You need Go 1.26 or newer (checked with 1.27). For the other targets you need:
- Node (checked with 26), which runs the JavaScript output in the tests until they run in a browser;
- a JDK (checked with 17), and for Android the SDK's `d8` and an emulator or a device;
- for Windows, Visual Studio's C++ tools (for `link`) and LLVM's `clang` (checked with 23.1).

The commands on this page are one per line, with no `&&`, and a built program is named `.exe`, so
they run unchanged in bash, zsh, Windows PowerShell and PowerShell 7. (Windows will not run a file
without the extension, and a file named `.exe` runs fine on Linux and macOS.)

```bash
go run ./cmd/build -target=go -o hello.exe examples/hello.oro
./hello.exe
```

```
hello from oroboros
42
```

```bash
go run ./cmd/check                                      # every check: tests, emission baseline, all targets
go run ./cmd/oro -target=go examples/native/dot.oro   # what a program reduces to
go run ./cmd/portable examples/io/wc.oro                # which targets accept a program, and why the others refuse
```

The differential suite runs from its own directory:

```bash
cd gauntlet/differential
go run run.go                                           # every test program, on every target
```

## Where things are

| | |
|---|---|
| [core/](core/) | J's reader, the abstract syntax and its desugaring, terms, the reducer, the state translation |
| [ir/](ir/) | the IR, the analyses that decide what is legal, the representation choices, and the four backends ([spec](docs/spec/ir.md)) |
| [emit/](emit/) | the type checker, the refinement layer (bounds, preconditions, claims), contracts, and the target loader |
| [presburger/](presburger/) | the Omega test, a complete decision procedure for linear integer arithmetic |
| [targets/](targets/) | what each host provides: declarations, never compiler code |
| [lib/](lib/) | portable modules such as `os` and `io`, and each host's implementation of them |
| [examples/](examples/) | the programs, including [io/](examples/io/) (`wc`, `count`, `jsonfmt`, `freq`), [os/](examples/os/) (`du`, `ls`), [lines/](examples/lines/) (`lines`, `shape`), [json/](examples/json/), [tally/](examples/tally/) and [u128/](examples/u128/) |
| [gauntlet/](gauntlet/) | hand-written references, benchmarks, the differential suite, and [results/](gauntlet/results/) |
| [docs/decisions/](docs/decisions/) | every significant decision, with what was rejected and why |
| [docs/spec/](docs/spec/) | the specifications; start with [state.md](docs/spec/state.md) and [j.md](docs/spec/j.md) |

## How it is built

- **One structure, read by everything.** The reducer's output, a closed first-order program, is
  lowered to a structured SSA IR ([ADR 0032](docs/decisions/0032-the-ir-is-structured-ssa.md)). Every
  analysis reads it: legality, termination, preconditions, and each value's representation. Every
  backend prints it. Nothing rediscovers the program's shape from the terms.
- **Measure, don't assert.** Every performance or design claim is benchmarked against hand-written code,
  with the expected loser in the benchmark too. Unmeasured claims here have been wrong about half the
  time, and the corrections are kept.
- **Derive, then build.** A feature starts from what it *is*: its algebra, its laws, and the literature.
  Then it is specified, and only then written.
- **Programs first.** A construct or an analysis goes in when a program needs it, and the balance
  between the compiler's growth and the code written in the language is measured every round.

## The name

The serpent eating its own tail: the intended endpoint is a compiler written in its own language.
