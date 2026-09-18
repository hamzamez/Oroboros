# Oroboros

A small language that compiles to **Go, JavaScript, Java and x86-64 assembly**. The output is the code
a good programmer on that platform would have written, and it calls the platform's own libraries
directly.

```lisp
(def main ()
  (io.print-line (big-str (fib 100))))
```

```
354224848179261915075          ; on Go, on Node, and on the JVM
```

> **Status: a working compiler, not a release.** Every claim on this page was measured or run, and the
> measurements are linked. The language still changes, nothing is versioned, and there is no package manager,
> no editor support and no stability promise.

---

## The idea in four sentences

1. **There is no runtime and no wrapper.** A program becomes ordinary host source (a Go function, a JS
   module, a Java class, a MASM file), and a call to `os.ReadFile` *is* a call to `os.ReadFile`.
2. **Portability is a property the compiler computes**, not a promise the language makes. Use only
   what every target provides and the program runs everywhere; call a Go package and it runs on Go,
   and the compiler says so.
3. **The program is evaluated at compile time as far as it can be.** Functions, abstractions and
   compile-time data structures disappear, and what reaches the backend is loops, tables and host
   calls.
4. **What cannot be proven is refused, not checked at run time.** That covers array bounds,
   preconditions on host calls, and integers leaving their range.

---

## A tour, in five programs

The first four were compiled and run for this page, and each output shown is what they printed. The fifth
comes from the test suite, which runs it on every target.

### 1. A sieve: tables, loops and buffers

```lisp
(use io)
(export main)

(def count-primes (n)
  (let (build n (fn (sieve)                   ; a buffer of n booleans, zero-filled
         (loop ((s sieve) (i 2))
           (>= (* i i) n)   s
           (>= i (len s))   s
           (s i)            (again s (+ i 1))  ; indexing is application
           else             (again (loop ((s s) (j (* i i)))
                                     (>= j n)  s
                                     else      (again (set s j true) (+ j i)))
                                   (+ i 1)))))
    (fn (composite)                           ; frozen on the way out
      (loop ((k 2) (count 0))
        (>= k n)       count
        (composite k)  (again (+ k 1) count)
        else           (again (+ k 1) (+ count 1))))))

(def main () (io.print-int (count-primes 1000)))
```

```
168
```

It prints the same on Go, Node and the JVM. The emitted Go has a `[]bool`, three plain `for` loops, no
bounds-check helpers and no wrapper:

```go
sieve := make([]bool, 1000)
s := sieve
var i int = 2
for ; ; i = (i + 1) {
	if ((i * i) >= 1000) {
		break
	}
	if (i >= len(s)) {
		break
	}
	if s[i] {
		continue
	}
	…
```

The language's whole character is in there:
- **`loop` names its variables and `again` jumps back** with new values. There is no recursion, and no
  other iteration.
- **Mutation happens only on a `build` buffer.** A buffer is *linear*: every `set` consumes it and
  hands it back, so no two parts of a program can write the same memory. On the way out it freezes
  into an ordinary immutable table.
- **`(s i)` is an application.** A table is a function with a known, finite domain, and the compiler
  proved `i` is inside it.

### 2. Fibonacci: exact integers

The same loop, asked for `fib(100)` as an ordinary integer:

```lisp
(def fib (n)
  (loop ((a 0) (b 1) (i 0))
    (>= i n)  a
    else      (again b (+ a b) (+ i 1))))

(def main () (io.print-int (fib 100)))
```

```
build: main: 1 of 2 integer operation(s) cannot be proven to stay inside the portable window, ±(2^53−1)
  + [1, +inf] in (+ a b)
  Outside that window the four targets disagree silently — Go and the JVM
  wrap, JavaScript loses precision — so this is refused rather than noted
```

This refusal is the point. Compiled naively, that line gives `3736710778780434371` on Go and the JVM
(wrapped) and `354224848179262000000` on Node (rounded): three wrong answers from one source. Say the
result is unbounded instead:

```lisp
(sig fib ((n (int 0 1000))) (int 0 +inf))

(def main () (io.print-line (big-str (fib 100))))
```

```
354224848179261915075
```

Each host uses its own big integer: `*big.Int` on Go, `BigInt` on Node, `BigInteger` on the JVM. A
range is part of a value's *type*: `(int 0 255)` is stored as a `[]byte` on Go and as a `short[]` on
the JVM, whose `byte` is signed. Above the machine word, the target picks the representation.

### 3. A line counter: calling a host API that can fail

```lisp
(use os)
(use io)
(export main)

(def cap-in 16777216)

(def main ()
  (let (os.Args) (fn (av)
    (if (< (len av) 2)
        (seq (io.print-line "usage: wc FILE") 0)
        ((os.ReadFile (av 1)) (fn (src err)
          (if (os.err-nil err)
              (if (>= (len src) cap-in)
                  (seq (io.print-line "wc: file is larger than this tool accepts") 0)
                  (loop ((i 0) (lines 0))
                    (>= i (len src)) (io.print-int lines)
                    else (again (+ i 1)
                          (if (< lines cap-in)
                              (if (= (src i) 10) (+ lines 1) lines)
                              lines))))
              (seq (io.print-line "wc: cannot read that file") 0))))))))
```

```bash
go run ./cmd/build -target=go   -o wc.exe     examples/io/wc.oro && ./wc.exe CLAUDE.md
go run ./cmd/build -target=js   -o wc.mjs     examples/io/wc.oro && node wc.mjs CLAUDE.md
go run ./cmd/build -target=java -o wc-classes examples/io/wc.oro && java -cp wc-classes Main CLAUDE.md
```

All three print `455`, the same as `wc -l`.
- **A fallible call gives two results, `(fn (src err) …)`, on every host**, including the ones where
  the platform throws. How each host fails is written once, in that target's declarations. On Go it
  compiles to `src, err := os.ReadFile(av[1])`.
- **`cap-in` is a value, not a function.** A constant is a term the compiler unfolds; the
  `(fn () …)` wrapper is for a *computation*, whose effects unfolding would repeat. And it is not
  decoration: a count over an unbounded file cannot be proven to stay in range, so
  the program states how large a file it accepts. The compiler made it say what happens at the limit.

### 4. `encoding/hex`: a host package, with its preconditions

```lisp
(use go/encoding-hex as hex)
(use os)
(use io)
(export main)

(def main ()
  (let (build 3 (fn (b) (set (set (set b 0 104) 1 105) 2 33))) (fn (src)   ; "hi!"
    (io.print-line
      (os.text-of
        (build (* 2 (len src)) (fn (dst)
          ((hex.Encode dst src) (fn (dst n) dst)))))))))
```

```
686921
```

Go's `hex.Encode` panics if `dst` is too short. Here it cannot be called with one. Change the
destination to `(+ (len src) 1)` bytes and the program does not compile:

```
build: main: go/encoding-hex.Encode requires -len(dst) + 2*len(src) <= 0, which does not follow
```

The declaration is plain data, written by hand from the package's source and checked against the real
package:

```lisp
(sig Encode ((dst (buffer (int 0 255))) (src (array (int 0 255))))
      (tuple (buffer (int 0 255)) (int 0 9007199254740990))
      (where (<= (* 2 (len src)) (len dst)))
      (host expr "func(dst, src []byte) ([]byte, int) { return dst, hex.Encode(dst, src) }(%s, %s)"
        (import "encoding/hex")))
```

The same program built for JavaScript stops with `(use go/encoding-hex) matched no file`. That is
portability being computed: this program is a Go program, and the compiler says so.

### 5. Variants and `match`

From the test suite, where each runs on all four targets:

```lisp
(variant result (ok int) (err int))

(def step (n)
  (if (>= n 10) (err n) (ok (* n 2))))

(def run (n)
  (case (step n)
    (ok v)  v
    (err e) (+ e 1000)))
```

A variant whose constructor is known at compile time disappears. One decided at run time becomes the
`if` that decided it: no tag, no allocation, no dispatch.

```lisp
(def run (n)
  (match (0 n 0)
    _ 0 c                     c
    0 v c (when (>= v 10))   (again 1 (- v 10) (+ c 1))
    _ v c (when (>= v 10))   (again 0 (- v 10) c)
    _ v c                     (again 0 0 c)
    else                      0))
```

`match` is a `loop` over its scrutinees, so `again` means *match again*. A parser's state machine is
written directly.

---

## The language on one page

The full, current description is [docs/spec/state.md](docs/spec/state.md). Every word the compiler
knows is listed in [docs/spec/inventory.md](docs/spec/inventory.md), and a test fails if the two
drift apart.

| | |
|---|---|
| **Terms** | seven kinds: name, integer, float, string, `true`/`false`, `(fn (x…) e)`, application |
| **Top level** | `def`, `sig` (with `where`, `ensures`), `variant`, `module`, `use`, `export` |
| **Sugar** | `(def f (x…) body)` for a λ, `let`, `seq`, `and`/`or`/`not`/`cond`, `tuple`, `match`/`when`, `case`; all gone after reading |
| **Iteration** | `loop` and `again`. No recursion, and termination is checked |
| **Data** | tables `(array V)`, maps `(map int V)`, tuples, variants with type arguments, strings as scalar sequences; `option` for a map read |
| **Mutation** | only on a linear buffer, inside `build` or as a declared `(buffer V)` parameter |
| **Integers** | exact within ±(2⁵³−1) on every host; a range `(int LO HI)` is a type; `(int 0 +inf)` is arbitrary precision |
| **Functions** | fully higher-order at compile time; nothing that needs a closure at run time may survive to the output |
| **Effects** | one purity bit per host call; an impure call runs exactly once, where it was written |
| **Targets** | directories of declarations (`sig`, `type`, `repr`, `fact`, `const`), which are data and never compiler code |

## What the compiler guarantees

| | how | measured |
|---|---|---|
| Emitted code is as fast as hand-written | seven benchmark programs held against hand-written Go, JavaScript and Java | largest gap **1.13×**, best **0.91×** over 19 comparisons ([gauntlet-2026-09-07](gauntlet/results/gauntlet-2026-09-07.md)); two programs compile to byte-identical machine code ([generics](gauntlet/results/generics-2026-08-14.md), [structs](gauntlet/results/structs-2026-08-14.md)) |
| An integer never silently wraps or rounds | interval analysis; unproven means refused | **2,361 of 2,413** operations proven across the corpus; the rest are refused by design ([examples/int/](examples/int/)) |
| Array indices and host preconditions hold | linear-arithmetic proofs, including facts about what a table holds | the JSON tree walker runs with **no bounds clamps** at 1.06× of hand-written unclamped Go ([compfacts](gauntlet/results/compfacts-2026-09-17.md)) |
| Loops terminate | size-change termination | **345 of 382** loops proven |
| Every host agrees | 30 programs built and **run** on all four targets, required to print the same, correct answer | [gauntlet/differential/](gauntlet/differential/) |
| The compiler's output never drifts by accident | every emitted file, proof count and error message compared with a committed baseline | `go run ./cmd/check` |

## How much of each platform it can reach

Surveys read each host's own API list and count what the declaration format can express:

| host | declarable | usable today |
|---|---:|---:|
| Go standard library | 87.8% | 60.4% |
| JVM (JDK) | 81.4% | 60.9% |
| Win32 | 90.5% | 31.0% callable *and* linkable |
| Node | 100% | not a meaningful number: every value has one type |

"Declarable" is not "supported". A package is supported when every function is declared **by hand**,
with its preconditions and what it does to buffers, and checked against the real package
([ADR 0022](docs/decisions/0022-host-declarations-are-written-by-hand.md)). Today that is
`unicode/utf8` and `encoding/hex`, plus `io`'s interfaces. Working through Go's standard library
package by package is the current work.

## What doesn't work yet

- **Most of each standard library** is not declared by hand yet.
- **No recursion.** Balanced divide-and-conquer (merge sort, Karatsuba) is a loop over levels, and
  recursive data is a flat table with indices. It is fast, and it is more to write.
- **No runtime closures, no concurrency, no interfaces you implement.** You can pass a host object where
  a host interface is wanted; you cannot build one.
- **Strings are thin:** concatenation and conversion at a boundary. Text programs so far work in bytes.
- **Maps take integer keys only.**
- **Windows** is the least complete target, and large programs can exceed its register allocator.
- **Rough edges found while writing this page:**
  - an array *literal* passed to a byte-array host parameter passes our type checker and is then
    rejected by the Go compiler;
  - a buffer's length is not carried out of an inner loop, which is why the sieve guards `(len s)` as
    well as `n`.
- **No packaging, no editor support**, and error messages written for the compiler's authors rather
  than for newcomers.

## Try it

You need Go 1.26 or newer (checked with 1.27). For the other targets you need Node (checked with 26)
and a JDK (checked with 17), and Visual Studio's MASM for Windows.

```bash
go run ./cmd/build -target=go -o hello examples/hello.oro && ./hello
```

```
hello from oroboros
42
```

```bash
go run ./cmd/check                                  # every check: tests, emission baseline, all targets
go run ./cmd/oro -target=portable-go examples/dot.oro   # what a program reduces to
cd gauntlet/differential && go run run.go           # every test program, on every target
```

## Where things are

| | |
|---|---|
| [core/](core/) | reader, terms, reducer |
| [emit/](emit/) | the four backends, the type checker and the proofs |
| [targets/](targets/) | what each host provides: declarations, never compiler code |
| [lib/](lib/) | portable modules such as `os` and `io`, and each host's implementation of them |
| [examples/](examples/) | the programs, including [io/](examples/io/) (`wc`, `jsonfmt`, `freq`), [json/](examples/json/) and [tally/](examples/tally/) |
| [gauntlet/](gauntlet/) | hand-written references, benchmarks, the differential suite, and [results/](gauntlet/results/) |
| [docs/decisions/](docs/decisions/) | every significant decision, with what was rejected and why |
| [docs/spec/](docs/spec/) | the specifications; start with [state.md](docs/spec/state.md) |

## How it is built

- **Measure, don't assert.** Every performance or design claim is benchmarked against hand-written code,
  with the expected loser in the benchmark too. Unmeasured claims here have been wrong about half the
  time, and the corrections are kept.
- **Derive, then build.** A feature starts from what it *is*: its algebra, its laws, and the literature.
  Then it is specified, and only then written.

The current assessment of the project, including what is going badly, is
[docs/assessment-2026-09-17.md](docs/assessment-2026-09-17.md).

## The name

The serpent eating its own tail: the intended endpoint is a compiler written in its own language.
