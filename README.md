# Oroboros

**Write a program once. Compile it to Go, JavaScript, Java or x86-64 assembly, as the code a good
programmer on that platform would have written — and call that platform's own libraries directly.**

> **Work in progress, not a release.** The compiler works and every claim below is measured, but the
> language is still changing, nothing is versioned, and there is no package manager, no editor
> support and no stability promise. Read it to see where it is going; don't build on it yet.

---

## Why you might care

Every cross-platform language makes you choose.

- **Portable languages wrap the platform.** You get their standard library, their runtime, their
  garbage collector and their idea of a string — and you reach the host's own APIs through an escape
  hatch, a binding generator or not at all.
- **Native code is fast and fully capable, and you write it once per platform.**

Oroboros takes a different position: **there is no runtime and no wrapper**. A program compiles to
ordinary source for the host — a Go function, a JavaScript module, a Java class, a MASM file — and a
call to `os.ReadFile` *is* a call to `os.ReadFile`. What you get in exchange for giving up a global
portability guarantee:

- **The emitted code is at parity with hand-written code.** Not "close enough": the benchmark suite
  holds it to within the noise floor of a hand-written equivalent on each host, and it records where
  it misses.
- **Portability is something the compiler tells you, not something the language enforces.** Use
  only what every target provides and your program runs everywhere. Call Win32, and it runs on
  Windows — and that is a normal, first-class program to write.
- **Arithmetic means what it says on every host.** Integers are exact. If the compiler cannot prove an
  operation stays in range, it refuses to compile rather than letting Go wrap, the JVM wrap and
  JavaScript lose precision — three different wrong answers from one line of source. When you need
  big numbers, you declare the range and get arbitrary precision.
- **The compiler proves things for you, and says so when it can't.** Array indices, preconditions on
  host calls, and loop termination are checked at compile time. An obligation it cannot discharge is
  an error that names the operation, not a silent runtime check.

## What it looks like

A line counter. It reads a file named on the command line and prints how many newlines it has.

```lisp
(use os)
(use io)

(export main)

(def cap-in (fn () 16777216))

(def main (fn ()
  (let (os.Args) (fn (av)
    (if (< (len av) 2)
        (seq (io.print-line "usage: wc FILE") 0)
        ((os.ReadFile (av 1)) (fn (src err)
          (if (os.err-nil err)
              (if (>= (len src) (cap-in))
                  (seq (io.print-line "wc: file is larger than this tool accepts") 0)
                  (loop ((i 0) (lines 0))
                    (>= i (len src)) (io.print-int lines)
                    else (again (+ i 1)
                          (if (< lines (cap-in))
                              (if (= (src i) 10) (+ lines 1) lines)
                              lines))))
              (seq (io.print-line "wc: cannot read that file") 0)))))))))
```

The same file, three hosts, one answer:

```bash
go run ./cmd/build -target=go   -o wc.exe      examples/io/wc.oro && ./wc.exe README.md
go run ./cmd/build -target=js   -o wc.mjs      examples/io/wc.oro && node wc.mjs README.md
go run ./cmd/build -target=java -o wc-classes  examples/io/wc.oro && java -cp wc-classes Main README.md
```

On Go, `os.ReadFile` comes out as the host's own two-result call, with no wrapper type and no
intermediate object:

```go
src, err := os.ReadFile(av[1])
```

A few things in that program are worth a second look, because they are the language's character:

- **There is no recursion and no mutation outside a loop.** `loop` names its variables and `again`
  jumps back with new values. That is all the iteration there is, and it is why termination can be
  checked.
- **A fallible host call gives back two values**, `(fn (src err) …)`, on every host — including the
  ones where the platform throws. How each host signals failure is written once in that target's
  declarations, and your program never sees an exception.
- **`cap-in` is not decoration.** A count over an unbounded file cannot be proven to stay in range,
  so the program states how big a file it accepts. The compiler made it say what happens at the
  limit instead of pretending there isn't one.

### When the compiler says no

Fibonacci, with the result declared as an ordinary integer:

```lisp
(sig fib ((n int)) int (where (and (<= 0 n) (< n 1000))))
(def fib (fn (n)
  (loop ((a 0) (b 1) (i 0))
    (>= i n)  a
    else      (again b (+ a b) (+ i 1)))))
```

```
1 of 2 integer operation(s) cannot be proven to stay inside the portable window, ±(2^53−1)
  + [1, +inf] in (+ a b)
  Outside that window the four targets disagree silently — Go and the JVM
  wrap, JavaScript loses precision — so this is refused rather than noted
  (ADR 0012, ADR 0019).
  Clear it by saying one of:
    · NARROW THE RANGE — `(sig f ((n (int 0 1000))) …)`, or a `(where …)`,
    …
```

Say what you mean instead — the result is a non-negative integer of any size:

```lisp
(sig fib ((n (int 0 1000))) (int 0 +inf))
```

Now it compiles on all three, using each host's own big integer: `*big.Int` on Go, `BigInt` in
JavaScript, `BigInteger` on the JVM. `fib(100)` is `354224848179261915075` everywhere, where the
naive version printed three different wrong numbers.

## What works today

**Four targets.** Go, JavaScript (Node), Java (the JVM) and Windows x86-64 assembly under MASM. The
first three are the everyday ones; Windows is the host with no runtime at all, where the compiler has
to supply everything a higher-level host gave it for free.

**Real programs, not just benchmarks.**

| | |
|---|---|
| [wc](examples/io/wc.oro) | line count, byte-identical output on Go, JavaScript and Java |
| [jsonfmt](examples/io/jsonfmt.oro) | a JSON pretty-printer, same three hosts |
| [freq](examples/io/freq.oro) | a word-frequency report — `sort \| uniq -c \| sort -rn` — same three hosts |
| [tally](examples/tally/) | counts regex captures in a log, using each host's own regex engine; one core, bound to Go and to the JVM |
| [a JSON parser](examples/json/) | tokeniser and tree, on all four targets, with no recursion |

**The language.** Functions, `loop`/`again`, booleans and `if`, several return values, pattern
matching, sum types (`(sum result (ok int) (err int))`), tuples, arrays, maps with integer keys,
strings built by concatenation, exact integers with declared ranges and arbitrary precision, and
mutable buffers that are *linear* — used exactly once, so two parts of a program can never modify the
same memory by surprise. Functions are fully higher-order at compile time; nothing that needs a
closure at runtime survives to the output.

**Performance.** Seven benchmark programs — dot product, search, structs, word count, generics,
stencil, a JSON tokeniser — are held against hand-written code on Go, JavaScript and Java. Nineteen
comparisons in the latest run: the largest gap is **1.13×** and the best is **0.91×**, on a laptop
with a ~15% noise floor ([gauntlet-2026-09-07](gauntlet/results/gauntlet-2026-09-07.md)). Two programs
compiled to *byte-identical machine code* against the hand-written Go.

**Host APIs.** Declaring a host function is one line of data — no binding generator, no Go code:

```lisp
(prim sqrt (f64) f64 expr "math.Sqrt(%s)" (import "math"))
```

A declaration can also carry what the call needs and what it guarantees. From Go's `encoding/hex`:

```lisp
(prim Encode ((dst (buffer (int 0 255))) (src (array (int 0 255))))
      ((buffer (int 0 255)) (int 0 9007199254740990))
      expr "func(dst, src []byte) ([]byte, int) { return dst, hex.Encode(dst, src) }(%s, %s)"
      (where (<= (* 2 (len src)) (len dst)))
      (import "encoding/hex"))
```

Pass a buffer one byte too short and the program doesn't compile — the error names the call and the
inequality it could not prove. Two Go standard library packages, `unicode/utf8` and `encoding/hex`,
are declared completely this way and checked against the real packages. Surveys of all four hosts
measure how much of each API the declaration format can express today
([gauntlet/stdlib](gauntlet/stdlib/)).

**Testing.** Thirty programs are built and *run* on every target on every change, and must print the
same, correct answer ([differential](gauntlet/differential/)). It has caught several silent
wrong answers that compiled cleanly on every host.

## What doesn't work yet

Being straight about it, because it decides whether this is useful to you:

- **Most of each standard library is not declared.** The surveys say how much *can* be; two Go
  packages are fully done by hand, and the rest is the current work.
- **No recursion.** Balanced divide-and-conquer (merge sort, Karatsuba) is written as a loop over
  levels, and recursive data as a flat table with indices. It works and it is fast; it is also more
  to write than recursion, and whether that is acceptable for everyday code is not settled.
- **No closures at runtime, no concurrency, no interfaces you can implement.** You can pass a host
  object where the host expects an interface; you cannot build one.
- **Strings are thin.** Concatenation and conversion at the boundary. Text-processing programs so
  far have worked in bytes.
- **Maps take integer keys only.**
- **Windows is the least complete target**: fewer libraries, and larger programs can exceed its
  register allocator.
- **No packaging, no editor support, no error messages written for newcomers**, and the syntax may
  change — the questions currently open about how declarations read are in
  [declaration-surface.md](docs/declaration-surface.md).

## Try it

You need Go 1.26 or newer. Node and a JDK for those targets; Visual Studio's MASM for Windows.

```bash
go run ./cmd/build -target=go -o hello examples/hello.oro && ./hello
```

```
hello from oroboros
42
```

```bash
go run ./cmd/oro -target=go examples/table/dot.oro   # show what a program reduces to
go test ./core/ ./emit/                              # the compiler's tests
cd gauntlet/differential && go run run.go            # every test program, on every target
```

## How it works, briefly

A program is evaluated **at compile time** as far as it can be. Every function call that can be
inlined is inlined, every abstraction that can be removed is removed, and what is left — loops,
tables and calls to the host — is handed to a backend that writes it the way that host's programmers
would. Higher-order code is free because none of it survives.

A **target** is a directory of plain declarations: which host functions exist, what types they take,
what they need and guarantee, and the text to emit. The language's own constructs (`if`, `loop`,
arithmetic, tables) are implemented by the compiler on every target; everything else is data you can
add without touching the compiler, and your own target layers can live next to your program.

The compiler proves what it can — array bounds, preconditions, integer ranges, termination — using a
deliberately small, predictable decision procedure. It never guesses: what it cannot prove, it reports.

## How the project is run

Two rules decide almost everything:

- **Measure, don't assert.** Every performance or design claim is benchmarked against a hand-written
  equivalent, with both the expected winner and the expected loser in the benchmark. Unmeasured claims
  in this repository have been wrong about half the time, and the corrections are kept, not deleted.
- **Derive, then build.** A feature starts from what it *is* — its algebra, its laws, and prior work in
  the literature — before a line of it is written.

Decisions are recorded as ADRs in [docs/decisions/](docs/decisions/), each with the alternatives that
were rejected and why. Measurements are in [gauntlet/results/](gauntlet/results/), and they are the
authority when a document disagrees with them.

## Going deeper

- [docs/design-direction.md](docs/design-direction.md) — the reasoning behind the design, and the
  predecessor project it learned from
- [docs/spec/state.md](docs/spec/state.md) — the language as it currently is
- [docs/the-atom.md](docs/the-atom.md) — what the core turned out to be
- [docs/decisions/](docs/decisions/) — the decisions, and what was rejected
- [gauntlet/results/](gauntlet/results/) — the measurements
- [CLAUDE.md](CLAUDE.md) — the running log of findings, dense and complete

## The name

The serpent eating its own tail: the intended endpoint is a compiler written in its own language.
