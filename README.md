# Oroboros

Oroboros is a programming language whose programs become ordinary code for **Go, the browser,
Android and Windows**: Go source, a JavaScript module, Java source, or LLVM IR built into a Windows
executable. It has no runtime of its own. A program calls each platform's own libraries by their
own names, and what it cannot prove about itself (an index out of range, an integer that would
overflow, a claim about a value) is refused before it runs.

## The book

The language is taught in [a book](docs/book/), one idea at a time: each chapter uses only what came
before it, shows a program refused before it shows the one that works, and leans on examples, edge
cases included. Every example in it is compiled and run by `go run ./cmd/check`, 1,130 blocks
across 20 chapters, and a chapter that stops saying what the compiler gives fails the check.

| | chapter | the one idea |
|---|---|---|
| 0 | [A tour](docs/book/00-tour.md) | the whole language at a glance |
| | **I. How a program is reduced** | |
| 1 | [`fn`](docs/book/01-fn.md) | a function is a value, and applying one is compiling |
| 2 | [Definitions](docs/book/02-def.md) | a name is an abbreviation, unfolded where it is used |
| 3 | [Modules](docs/book/03-modules.md) | where a name lives, and when the platform supplies it |
| 4 | [Choice](docs/book/04-choice.md) | `if` and guards; a choice known at compile time keeps one branch |
| 5 | [Effects](docs/book/05-effects.md) | what the compiler may not drop, copy or move |
| | **II. What a program computes with** | |
| 6 | [Integers](docs/book/06-integers.md) | an integer is an integer: a range is a set, proven or refused, exact past the machine word |
| 7 | [Bits](docs/book/07-bits.md) | the bits belong to the integer; wrapping is `mod 2ⁿ`, written where it is meant |
| 8 | [Numbers with a point](docs/book/08-numbers.md) | exact decimals, `round` the one place precision is lost, and IEEE doubles |
| 9 | [Text](docs/book/09-text.md) | a string is a sequence of characters; the platform's strings are its own |
| 10 | [Tuples and variants](docs/book/10-data.md) | "and" and "or" of data, and `case` |
| 11 | [Records](docs/book/11-records.md) | a record is a function on its labels; `with`; the zero |
| | **III. Repetition and storage** | |
| 12 | [`loop`](docs/book/12-loop.md) | the one way to repeat, and knowing that it stops |
| 13 | [Tables](docs/book/13-tables.md) | a table is a function on `0..len-1`, and an index is proven |
| 14 | [State](docs/book/14-state.md) | `var`, buffers, maps, cells lent with `inout` |
| 15 | [Failures](docs/book/15-failures.md) | a failure is a value: `try`, `expect`, and results that must be used |
| | **IV. Claims and hosts** | |
| 16 | [Claims](docs/book/16-claims.md) | how a header's claims are proven, and where the method stops |
| 17 | [Targets](docs/book/17-targets.md) | declaring a platform's function, a function the platform calls, and portability computed |
| 18 | [Calling C](docs/book/18-c.md) | a C function by its prototype, structs as records, a window procedure |
| 19 | [A whole program](docs/book/19-whole.md) | one real program, read from its first line to its last |

## A first program

```
use io

fn fib(n ∈ 0..1000) (0..) {
    loop a = 0, b = 1, i = 0 { i >= n -> a; else -> again(b, a + b, i + 1) }
}

export fn main() { io.print_line(big_str(fib(100))) }
```

```
354224848179261915075          on Go, JavaScript, the JVM, and 64- and 32-bit Windows
```

`n ∈ 0..1000` is a claim, proven at every call. `(0..)`, a result with no upper end, asks for the
answer exactly, so it is held in `math/big` on Go, a `BigInt` in JavaScript, a `BigInteger` on the
JVM, and on Windows, which has none, in numbers written in this language.

> **Status: a working compiler, not a release.** Every claim on this page was measured or run, and
> each number links to the measurement. The language still changes; nothing is versioned, and there
> is no package manager, no editor support and no promise of stability.

---

## What the language is

**It compiles by evaluating.** A program is reduced at compile time as far as it can be: functions,
abstractions and data structures known in advance disappear, and what reaches the platform is loops,
tables and calls to the platform's own functions. Generic code costs nothing, because each use is
specialised where it is written.

**A type is a set, and a claim is proven.** `0..255`, `Array(V)`, `record { x ∈ Int, y ∈ Int }` and
a variant `'ok(Int) ∪ 'err(E)` are sets of values. A function's header claims which sets its
arguments and result are in, and each claim is proven by the compiler or the program is refused:
by intervals over the integers, then by linear arithmetic over the facts a program states.
Nothing of this reaches the output, except in how each value is held.

**An integer is an integer.** Arithmetic is exact. Each platform declares the machine word it
computes in, an operation not proven to fit is refused on that platform, and which platforms accept
a program is computed and reported. A value claimed past the word is held exactly where it needs to
be, and nowhere else.

**Portability is computed, not promised.** The language's own words mean the same on every
platform. A platform's own functions are declared by hand, checked against the platform's own
source or prototypes, and carry no promise beyond that platform.

**The full language** is [docs/spec/state.md](docs/spec/state.md), read off the code, with its grammar
in [docs/spec/j.md](docs/spec/j.md) and every word the compiler knows in
[docs/spec/inventory.md](docs/spec/inventory.md); a test fails when either drifts from the code.
Every significant decision is an ADR in [docs/decisions/](docs/decisions/), with the alternatives
rejected and why.

---

## What is measured

| | how | the numbers |
|---|---|---|
| **as fast as hand-written code** | seven benchmark programs against hand-written Go, JavaScript, Java, and C on Windows compiled by the same compiler | generated over hand-written: **1.001** on Go, **0.965** on JavaScript, **1.012** on Java, **0.978** on Windows (geometric means, 45 comparisons from 0.85× to 1.18×, each above 1.05× explained) ([gauntlet-2026-11-16](gauntlet/results/gauntlet-2026-11-16.md)) |
| **no integer silently overflows** | an interval analysis against each platform's word; unproven means refused there | **12,460 of 12,516** integer operations in the corpus proven; the rest are meant to be refused, or are proven where the test harness builds them |
| **loops stop** | size-change termination | **1,479 of 1,774** loops proven; the rest read input or the file system, or count through numbers past the word (a halving, a bit cleared) in a way the check does not see |
| **every platform agrees** | 87 test programs built and run on every platform that accepts them, each required to print the same, correct answer | 73 on all four, every one on 32-bit Windows ([gauntlet/differential/](gauntlet/differential/)) |
| **each rule is tested broken** | a fault is planted in every rule that decides what is legal, and its test must fail | listed with each result in [gauntlet/results/](gauntlet/results/) |
| **nothing drifts by accident** | every emitted file, proof count and error message, on five platform targets, compared with a committed baseline | `go run ./cmd/check` |

**How much of each platform a program can reach**, measured by surveys that read each platform's own
list of functions:

| platform | declarable | callable today |
|---|---:|---:|
| Go's standard library | 87.8% | 60.5% |
| the JDK | 81.4% | 60.9% |
| Win32 | 91.2% | 36.2%, and 31.1% callable *and* linkable, by the arguments a program can pass today, a callback included; up to 47.6% as structs are declared ([survey-2026-11-19](gauntlet/results/survey-2026-11-19.md)) |

"Declarable" is not "supported". A Go package is supported when every function a program needs is
declared by hand, with its preconditions, and checked against the package's source: today
`unicode/utf8`, `encoding/hex`, `strconv`, `math/bits` and `encoding/binary`'s varints in full, and
`os`, `io`, `strings`, `fmt`, `bufio`, `errors`, `io/fs`, `path/filepath`, `time` and `bytes` in
part. On Windows, 83 declarations in 17 header modules are written as Microsoft's headers write
them and checked against the SDK's prototypes.

---

## What is not built yet

- **Most of each standard library** is not declared yet; each package comes in with a program that
  needs it.
- **No recursion**, by design: balanced divide and conquer is a loop over levels, and recursive data
  is a table with positions in it.
- **Callbacks of two kinds.** A window procedure, the function Windows calls with each message, and
  a function a platform calls only during a call (a search's predicate, forEach's action) are built,
  the second on every target, writing a variable of its caller's where the platform documents the
  order of its calls. A platform's calls on another thread, and a function a platform hands back,
  are not; there is no concurrency.
- **Strings are thin**: joining, and conversion at a boundary. Text programs so far work in bytes,
  and maps take integer keys only.
- **Windows** lacks a dictionary keyed by strings, the printing of floating-point numbers, handles as
  types of their own, text read back from a window, and the linking of a function whose stack frame
  passes a page.
- **The browser's own API** is not declared yet; the JavaScript output is tested under Node.
- **No packaging and no editor support.** Error messages are written for the compiler's authors.

---

## Current work

*Updated 2026-11-20.* The latest assessment is [docs/assessment-2026-11-20.md](docs/assessment-2026-11-20.md);
each earlier plan, and how it ended, is in the assessments in [docs/](docs/).

**The plan in force** (hamza, 2026-11-20), in order:
1. **The logic, its first three stages** (decided, [ADR 0145](docs/decisions/0145-the-logic-quantified-formulas-over-sets-proven-by-own-procedures.md)):
   claims with quantifiers, `∀ i ∈ 0..k-1 : b[i] != 0`, in a function's conditions and in `assert`,
   proven by the compiler's own procedures; a platform's documented promises believed with their
   sentence kept. The research note and the formulas with `assert` are **done** (2026-11-20: the
   Gregorian leap year's rule proven of its computation, Bloch's midpoint); next, tables'
   properties, with a C string read back from Windows (TinyRetroPad's window text).
2. **C structs' remaining kinds of field** (a buffer with its length, a record by pointer, an array
   inside the struct, a C string that may be NULL) and **the memory model's decided parts**: a
   handle as a type of its own, compared with `==`, a failed call's handle an `Option` (TinyRetroPad's
   file load and save); two parts of one buffer lent at once, where they are proven apart; memory
   Windows keeps past a call, the mode `kept` (TinyRetroPad's find and replace).
3. **A table written by a function a platform calls during a call** (counting sort through
   `forEach`): designed, the call lending the table and handing back its next version.
4. **The logic, its last three stages**: a comparator's law through a platform's sort, one answer
   everywhere; definitions and lemmas by induction, a table's sum; `-checked` running a bounded
   claim as a loop.
5. **[TinyRetroPad](https://github.com/plummersSoftwareLLC/TinyRetroPad)'s body in the language**, a
   Notepad-style editor in about 2.5 KB of hand-written assembly, module by module; then 128-bit
   integers written in the language itself.

**Done since it was set**: the book's findings 100 and 101 (a map read while it is filled; a table of
tables or of tuples written out, read at any index), and Windows' enumerators read against their
documentation ([win32evidence](gauntlet/results/win32evidence-2026-11-20.md)).

**The plan before it** (2026-11-19) ended with the callback decisions and the capacity facts done,
and TinyRetroPad's body carried into the one after this.

**Built since the plan was set**, each with its program:
- **the bit operations on numbers past the word**, with Stein's binary gcd and Kernighan's popcount;
  popcount runs on Windows in 0.64× Go's time ([bigbits-2026-11-19](gauntlet/results/bigbits-2026-11-19.md));
- **a target that names its processor**, `windows-v3`: an integer dot product in 0.60–0.69× the
  baseline's time ([cpu-2026-11-19](gauntlet/results/cpu-2026-11-19.md));
- **Win32 re-measured** by what a program can pass today ([survey-2026-11-19](gauntlet/results/survey-2026-11-19.md));
- **the book brought up to date**: two new chapters, Records and Calling C, and seven defects found
  writing them, five fixed (the book's findings 95–101).

**To come back to, with hamza**:
- a block after a call, `f(a…) { x̄ -> b }`, for a resource taken at its start and given back at its
  end ([scopeforms-research.md](docs/scopeforms-research.md));
- a callback's remaining questions: the word for "each element once, in no order", the laws a
  comparator obeys, and a function a platform hands back;
- how a variadic of places is written;
- a table's elements narrower than a word on Windows;
- a stack frame larger than a page there.

**Researched, and built when a program asks for it**:
- formers, a data structure's literal `F { a, b, c }` as its algebra's
  ([formers-research.md](docs/formers-research.md));
- a callback a platform runs on another thread while the call waits (Go's `T.Run`), with
  concurrency, when concurrency is in the language.

**The standing goal** is Go's standard library, package by package, each with the program that needs
it. **The standing measure** is Windows faster than Go, the gap work owed.

---

## Try it

You need Go 1.26 or newer (checked with 1.27). For the other platforms:
- Node (checked with 26), which runs the JavaScript output in the tests;
- a JDK (checked with 17), and for Android the SDK's `d8` and an emulator or a device;
- for Windows, Visual Studio's C++ tools (for `link`) and LLVM's `clang` (checked with 23.1).

Commands are one per line and a built program is named `.exe`, so they run unchanged in bash, zsh
and PowerShell. (Windows runs nothing without the extension; Linux and macOS do not mind it.)

```bash
go run ./cmd/build -target=go -o hello.exe examples/hello.oro
./hello.exe
```

```
hello from oroboros
42
```

```bash
go run ./cmd/build -target=windows -o stein.exe examples/big/stein.oro   # Stein's gcd over Fibonacci numbers
go run ./cmd/portable examples/io/wc.oro       # which platforms accept a program, and why the others refuse
go run ./cmd/oro -target=go examples/native/dot.oro   # what a program reduces to
go run docs/book/check.go docs/book/11-records.md     # re-run one chapter's examples
go run ./cmd/check                             # every check
```

---

## Where things are

| | |
|---|---|
| [docs/book/](docs/book/) | the book |
| [docs/spec/](docs/spec/) | the specifications: [state.md](docs/spec/state.md) first, then [j.md](docs/spec/j.md) |
| [docs/decisions/](docs/decisions/) | every significant decision, with what was rejected and why |
| [examples/](examples/) | programs: files and text ([io/](examples/io/), [os/](examples/os/)), big integers ([big/](examples/big/)), hashes ([hash/](examples/hash/)), the benchmarks' sources and Windows programs ([native/](examples/native/)) |
| [core/](core/) | the reader, the reducer, and the translation of state |
| [ir/](ir/) | the IR, the analyses that decide what is legal, the representation choices, and the four printers |
| [emit/](emit/) | the type checker, the refinement layer, the target loader, and the big-integer libraries written in the language |
| [targets/](targets/) | what each platform provides, as declarations and never as compiler code |
| [lib/](lib/) | portable modules such as `os` and `io`, and each platform's implementation of them |
| [gauntlet/](gauntlet/) | hand-written references, benchmarks, the differential suite, and [results/](gauntlet/results/) |

## How it is built

- **Derive, then build.** A construct starts from what it *is*, its algebra and its laws, with the
  literature that answered it; then it is specified for every platform, and only then written.
- **Programs first.** A construct or an analysis goes in when a program needs it, and the growth of
  the compiler against the code written in the language is measured every round.
- **Measure, don't assert.** Every performance claim is measured against hand-written code, with the
  form expected to lose measured too. Unmeasured claims here have been wrong about half the time.
- **One structure, read by everything.** After reduction a program is a structured SSA IR, which
  every analysis reads and every printer prints, and whose text is checked to reprint byte for byte.

## The name

The serpent eating its own tail: the intended end is a compiler written in its own language.
