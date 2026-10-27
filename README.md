# Oroboros

A language for four platforms: **Go, the browser, Android and Windows**. It compiles to Go,
JavaScript, Java and x86-64 assembly. The output is the code a good programmer on that platform would
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
   module, a Java class, a MASM file), and a call to `os.ReadFile` *is* a call to `os.ReadFile`.
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
and on Android. A range is part of a value's type: `Array(0..255)` is stored as a `[]byte` on Go and as
a `short[]` on the JVM, whose `byte` is signed. Above the machine word, the target picks the
representation.

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
new storage, says so with a result `dst' ∈ buffer(0..255)`
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
| **Integers** | an integer is an integer; each target declares the word it holds (int64 on Go, the JVM and x86, plus `uint64` on Go; ±(2⁵³−1) on JS), an operation not proven inside it is refused on that target, and which targets accept a program is reported; a range `0..255` is a type; `0..`, its upper end omitted, is arbitrary precision |
| **Functions** | fully higher-order at compile time; nothing that needs a closure at run time may survive to the output |
| **Effects** | one purity bit per host call; an impure call runs exactly once, where it was written |
| **Host calls** | under the host's own names: `fmt.Println(a, b)` is Go's variadic call, and `f(xs...)` passes a table as its list. A host's string is the host's type, and a `String` is always valid text |
| **Targets** | directories of declarations, which are data and never compiler code |

## What the compiler guarantees

| | how | measured |
|---|---|---|
| Emitted code is as fast as hand-written | seven benchmark programs held against hand-written Go, JavaScript and Java | geometric mean of generated over hand-written **0.990** on Go, **0.977** on JavaScript, **0.987** on Java, over 35 comparisons from **0.87×** to **1.21×**; the three above 1.10× each explained, the largest a JavaScript module boundary ([gauntlet-2026-09-29](gauntlet/results/gauntlet-2026-09-29.md)). On Android's runtime, in the emulator, 9 of 10 pairs are at parity and the stencil is 1.17–1.19× ([android-2026-10-05](gauntlet/results/android-2026-10-05.md)). The newest program, a table-shape tool reading its own 64 KB buffer, is at 1.11× its hand-written Go ([observer-2026-10-21](gauntlet/results/observer-2026-10-21.md)) |
| An integer never silently wraps or rounds | an interval analysis on the IR, against each target's word; unproven means refused on that target | **2,372 of 2,429** operations proven across the corpus; the rest are refused by design ([examples/int/](examples/int/)) or proven where the test harness builds them |
| Array indices and host preconditions hold | linear-arithmetic proofs, including facts about what a table holds | the JSON tree walker runs with **no bounds clamps** at 1.06× of hand-written unclamped Go ([compfacts](gauntlet/results/compfacts-2026-09-17.md)) |
| A function is only called inside its declared domain | every declared parameter range and `where` is proven at every call, above the machine word as the set its enforcement decides, by sign and bit length ([ADR 0028](docs/decisions/0028-a-definitions-contract-is-checked-at-its-calls.md)) | 346 range obligations in the corpus measured before it was switched on; one program was missing a declaration ([requires](gauntlet/results/requires-2026-09-24.md), [contracts](gauntlet/results/contracts-2026-09-24.md)) |
| A string is always text | every value typed `String` is a sequence of Unicode scalar values: a host's string is its own type (`go.bytestring`), entered only by the standard's decode, which is the same function on every host ([ADR 0030](docs/decisions/0030-a-hosts-string-is-the-hosts.md)) | 57 Go declarations read one at a time; `text_of` agrees on Go, JS and Java over 18 ill-formed sequences ([hoststring](gauntlet/results/hoststring-2026-09-24.md)) |
| Loops terminate | size-change termination on the IR | **363 of 386** loops proven; the rest are driven by input or the file system |
| Every host agrees | 54 programs built and **run** on every target that accepts them (39 on all four), required to print the same, correct answer | [gauntlet/differential/](gauntlet/differential/) |
| Each rule that decides legality is tested against a broken copy of itself | a fault is planted in the rule, and its test must fail | each result in [gauntlet/results/](gauntlet/results/) lists its faults and the test that caught each |
| The compiler's output never drifts by accident | every emitted file, proof count and error message compared with a committed baseline | `go run ./cmd/check` |

## How much of each platform it can reach

Surveys read each host's own API list and count what the declaration format can express:

| host | declarable | usable today |
|---|---:|---:|
| Go standard library | 87.8% | 60.5% |
| JVM (JDK) | 81.4% | 60.9% |
| Win32 | 90.5% | 31.0% callable *and* linkable |
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
  out-cells need a stack slot on x86.
- **Strings are thin:** concatenation and conversion at a boundary. Text programs so far work in bytes.
- **Maps take integer keys only**, and there are no records yet.
- **The browser's own API**, the Web platform, is not declared yet, and the JavaScript output is
  tested under Node, as an engine.
- **Windows** is the least complete target: its `io` has no `print_line`, it has no floats and no host
  bignum, and fixed-limb arithmetic cannot yet be printed there.
- **No packaging, no editor support**, and error messages written for the compiler's authors rather
  than for newcomers.

## Current work

*Updated with the plan; last updated 2026-10-27.* The current assessment,
[docs/assessment-2026-10-24.md](docs/assessment-2026-10-24.md), scores the last plan (finished: two
defects every program had fixed, two kinds of claim made true, and `wc`, a word count over files) and
says why the next is what it is.

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
        `mod 2^64`; built with the next step, first for two hash programs;
     2. ~~**integer widths**~~ **decided 2026-10-27**
        ([ADR 0083](docs/decisions/0083-a-representation-is-an-abstraction-function-chosen-by-place.md)):
        a width is only how a target holds an integer, never part of what it is, so no choice of width can
        change an answer; each is chosen by where the value lives. **Being built now with step 1**,
        first for two programs, FNV and CRC-32 hashes of a file;
     3. pointers;
     4. vectors (several numbers in one register);
     5. float formats other than 64-bit;
     6. operations LLVM allows to give more than one answer;
     7. rounding modes;
   - then the LLVM IR backend itself, until it builds everything the MASM one does, which then retires;
   - then what TinyRetroPad needs from Win32 against what is declared, callable and linkable today;
   - then its walls (a window procedure, Win32's structs, 32-bit code), each specified first.
7. **The IR's verifier checks that each branch justifies the facts it hands its arms.**
8. **A loop that continues after a nested loop's result is taken apart**, which `wc` needs to be written
   as Go is, and which costs it 16% today.
9. **Go's standard library, program first**: next, `cmp`, comparing two files chunk by chunk.

## Try it

You need Go 1.26 or newer (checked with 1.27). For the other targets you need:
- Node (checked with 26), which runs the JavaScript output in the tests until they run in a browser;
- a JDK (checked with 17), and for Android the SDK's `d8` and an emulator or a device;
- Visual Studio's MASM, for Windows.

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
go run ./cmd/oro -target=go examples/native/dot-go.oro   # what a program reduces to
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
