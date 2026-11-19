# Oroboros

A language for four platforms: **Go, the browser, Android and Windows**. It compiles to Go,
JavaScript, Java and LLVM IR. The output is the code a good programmer on that platform would
have written, and it calls the platform's own libraries directly.

## The book

[docs/book/](docs/book/) teaches the language one idea at a time: a chapter uses only what the
chapters before it introduced, and shows a thing refused before it shows it working. Every code
block in it is compiled by `go run ./cmd/check`, and its output compared with what the chapter
prints.

| | chapter | the one idea |
|---|---|---|
| 0 | [A tour](docs/book/00-tour.md) | the map, where everything may be shown |
| | **I. How a program is reduced** | |
| 1 | [`fn`](docs/book/01-fn.md) | β; reduction is compilation |
| 2 | [Definitions](docs/book/02-def.md) | δ; a name is an abbreviation |
| 3 | [Modules](docs/book/03-modules.md) | where a name lives; the target can supply it |
| 4 | [Choice](docs/book/04-choice.md) | `Bool`, `if`, guards; a known condition keeps one branch |
| 5 | [Effects](docs/book/05-effects.md) | what β may not drop, copy or move; a statement is a binding nobody reads |
| | **II. What a program computes with** | |
| 6 | [Integers](docs/book/06-integers.md) | ℤ, a range is a set, the word, proven or refused, above the word |
| 7 | [Bits](docs/book/07-bits.md) | the bits are the integer's; wrapping is `mod 2ⁿ` |
| 8 | [Numbers with a point](docs/book/08-numbers.md) | exact decimals; `round` the one place precision is lost; `F64` |
| 9 | [Text](docs/book/09-text.md) | strings as sequences, `++`, the host's strings, bytes |
| 10 | [Tuples and variants](docs/book/10-data.md) | "and" and "or" of data; `case` with patterns |
| | **III. Repetition and storage** | |
| 11 | [`loop`](docs/book/11-loop.md) | the one iteration; termination |
| 12 | [Tables](docs/book/12-tables.md) | a table is a function on `0..len-1`; an index is proven; fusion |
| 13 | [State](docs/book/13-state.md) | `var`, buffers, maps; why a table never changes under you |
| 14 | [Failures](docs/book/14-failures.md) | `Result`, `try`, `expect`, `relevant` |
| | **IV. Claims and hosts** | |
| 15 | [Claims](docs/book/15-claims.md) | how a header's sets are proven: intervals, facts, obligations |
| 16 | [Targets](docs/book/16-targets.md) | host modules, writing a declaration, portability computed |
| 17 | [A whole program](docs/book/17-whole.md) | one real program read line by line |

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

The result's set, `0..`, has no upper end, so it is held exactly on every target: `math/big` on Go,
`BigInt` and `BigInteger` on the browser and the JVM, and on Windows, which has no big integer of
its own, numerals written in the language.

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
5. **A type is a set, and a claim is proven.** `0..255`, `Array(V)`, a record and a variant are sets
   of values; the logic that reasons about them never reaches the output, and what it proves decides
   how each value is held.

The language is not small, and it is kept as small as the platforms allow: a construct goes in only
when a program needs it, with a specification of what it does on every target, and anything a target
already provides is called, not reimplemented.

---

## The language on one page

The full, current description is [docs/spec/state.md](docs/spec/state.md), and the surface's grammar is
[docs/spec/j.md](docs/spec/j.md). Every word the compiler knows is listed in
[docs/spec/inventory.md](docs/spec/inventory.md), and a test fails if the two drift apart. The
[book](#the-book), above, teaches it.

| | |
|---|---|
| **Terms** | seven kinds underneath: name, integer, exact decimal numeral, string, `true`/`false`, `fn`, application |
| **Declarations** | `fn f(x ∈ S, …) (R) where φ ensure ψ { e }`, a header carrying its claims and one body; a constant `c = e`; `set S = …` for a set, a record or a variant; `use`, `pub`, `export`, nested `module`s. A file is the module its path names |
| **Blocks** | `{ x = e; (a, b) = p; e₁; body }`, sequential bindings and statements, erased before reduction |
| **Choice** | `if c { a } else { b }`, `case { φ -> a; else -> b }`, `case e { 'tag(x) -> … }`, `&&`, `\|\|`, `!`, `!=` |
| **Iteration** | `loop x = z, … { φ -> e; …; else -> e }` with `again(…)`. No recursion, and termination is checked |
| **Data** | tables `Array(V)` with `t[i]`, `array { 1, 2 }` and `array { e : k ∈ 0..n-1 }`, and segments `b[lo:hi]`; maps `Map(Int, V)` with a declared capacity; tuples `(a, b)` and the unit `()`; records `record { l ∈ S }`, `r.l` and `r with { l : e }`; variants `'a ∪ 'b(T)` with type arguments; strings as sequences of Unicode scalars |
| **Local state** | `var x := e`: a table makes a buffer, written `b[i] := v` in program order and linear; a map former a map buffer; anything else a cell. A parameter `b ∈ Array(V) linear` borrows a buffer, and `(b' ∈ Array(V) linear)` hands it back; `x ∈ T inout` is a cell the callee writes |
| **Claims** | a binder's set, `x ∈ 0..255`, and `where` are obligations at every call; `b ∈ S = e` claims a value's set; a declared result is proven at its definition; each is proven or the program is refused |
| **Failures** | a value of a sum: `lib/result`'s `result(T, E)`, met by `case`, `try`, `expect` or `ignore`, and *relevant*, so it cannot be dropped silently. A host's own encoding (Go's `(T, error)`, a throw) is translated once, at its declaration |
| **Integers** | an integer is an integer. Each target declares the word it holds (int64 on Go, the JVM and Windows, ±(2⁵³−1) on JavaScript); an operation not proven inside it is refused there, and which targets accept a program is reported. A set past the word is held exactly on every target. `& \| ⊕ ~ << >>` and `mod` are ℤ's own, above the word too, and wrapping is written `mod 2^64` |
| **Numbers with a point** | exact decimals, so `0.1 + 0.2 == 0.3`; `round(S, e, mode)` is the one operation that loses precision; `F64` is IEEE's, the same on every target |
| **Functions** | fully higher-order at compile time; nothing that needs a closure at run time survives to the output, except a function Windows calls back, a window procedure, written in the language |
| **Effects** | one purity bit per host call; an impure call runs exactly once, where it was written |
| **Host calls** | under the host's own names: `fmt.Println(a, b)` is Go's variadic call, and `f(xs...)` passes a table as its list. A Windows function is declared by its C prototype, in its header's module, and called as C writes it: `call WriteFile(h, s, len(s), &n, NULL)`. A C struct is a record, laid out as the C compiler lays it out, and checked against it |
| **Targets** | directories of declarations, which are data and never compiler code; a target may name its CPU (`windows-v3`, AVX2) |

## What the compiler guarantees

| | how | measured |
|---|---|---|
| Emitted code is as fast as hand-written | seven benchmark programs held against hand-written Go, JavaScript, Java, and C on Windows by the same compiler | geometric mean of generated over hand-written **1.001** on Go, **0.965** on JavaScript, **1.012** on Java, **0.978** on Windows, over 45 comparisons from **0.85×** to **1.18×**; each above 1.05× explained ([gauntlet-2026-11-16](gauntlet/results/gauntlet-2026-11-16.md)). Windows runs five of the seven: word count needs a dictionary keyed by strings, the report printing of floats. On Android's runtime, 9 of 10 pairs are at parity ([android-2026-10-05](gauntlet/results/android-2026-10-05.md)) |
| An integer never silently wraps or rounds | an interval analysis on the IR, against each target's word; unproven means refused on that target | **11,812 of 11,866** operations proven across the corpus, every quotient included; the rest are refused by design ([examples/int/](examples/int/)) or proven where the test harness builds them |
| Array indices and host preconditions hold | linear-arithmetic proofs, including facts about what a table holds and what a branch's guard says of a difference | the JSON tree walker runs with **no bounds clamps** at 1.06× of hand-written unclamped Go ([compfacts](gauntlet/results/compfacts-2026-09-17.md)) |
| A function is only called inside its declared domain | every declared parameter range and `where` is proven at every call ([ADR 0028](docs/decisions/0028-a-definitions-contract-is-checked-at-its-calls.md)) | 346 range obligations measured before it was switched on; one program was missing a declaration ([contracts](gauntlet/results/contracts-2026-09-24.md)) |
| A string is always text | every `String` is a sequence of Unicode scalar values: a host's string is its own type, entered only by the standard's decode, the same function on every host ([ADR 0030](docs/decisions/0030-a-hosts-string-is-the-hosts.md)); a string handed to Windows is proven free of NUL ([ADR 0091](docs/decisions/0091-a-c-string-is-the-kernel-of-the-nul-homomorphism.md)) | `text_of` agrees on Go, JS and Java over 18 ill-formed sequences ([hoststring](gauntlet/results/hoststring-2026-09-24.md)) |
| Loops terminate | size-change termination on the IR | **1,361 of 1,654** loops proven; the rest are driven by input or the file system, or are loops over numbers past the machine word whose progress (a halving, a bit cleared) the check does not see |
| Every host agrees | **87** programs built and **run** on every target that accepts them (73 on all four, and every one on 32-bit Windows), required to print the same, correct answer | [gauntlet/differential/](gauntlet/differential/) |
| Each rule that decides legality is tested against a broken copy of itself | a fault is planted in the rule, and its test must fail | each result in [gauntlet/results/](gauntlet/results/) lists its faults and the test that caught each |
| The compiler's output never drifts by accident | every emitted file, proof count and error message, on five targets, compared with a committed baseline | `go run ./cmd/check` |

## How much of each platform it can reach

Surveys read each host's own API list and count what the declaration format can express:

| host | declarable | usable today |
|---|---:|---:|
| Go standard library | 87.8% | 60.5% |
| JVM (JDK) | 81.4% | 60.9% |
| Win32 | 91.2% | 30.2% callable *and* linkable, by the arguments a program passes today and the nine structs the target declares; up to 46.3% callable as structs are declared ([survey-2026-11-19](gauntlet/results/survey-2026-11-19.md)) |
| JavaScript (Node's `globalThis`) | 100% | not a meaningful number: every value has one type. The browser's Web APIs are not surveyed yet |

"Declarable" is not "supported". A package is supported when every function is declared **by hand**,
with its preconditions and what it does to buffers, and checked against the real package
([ADR 0022](docs/decisions/0022-host-declarations-are-written-by-hand.md)). On Go that is
`unicode/utf8`, `encoding/hex`, `strconv`, `encoding/binary`'s varints and `math/bits`, plus `io`'s
interfaces; `os`, `strings`, `io`, `fmt`, `bufio`, `errors`, `io/fs`, `path/filepath`, `time` and
`bytes` are partly declared, every name checked (`go run gauntlet/stdlib/coverage.go`). On Windows,
83 declarations in 17 header modules (six of them `SendMessageA`'s messages) are written as their
headers write them, each checked against Microsoft's own prototype
([byheader-2026-11-17](gauntlet/results/byheader-2026-11-17.md)). Each
package comes in with a program that needs it: [examples/io/cmp.oro](examples/io/cmp.oro) over `os`
and `io`, [examples/os/ls.oro](examples/os/ls.oro) over `os` and `time`,
[examples/native/status-win.oro](examples/native/status-win.oro) over a Rich Edit control.

## What doesn't work yet

- **Most of each standard library** is not declared by hand yet.
- **No recursion.** Balanced divide-and-conquer (merge sort, Karatsuba) is a loop over levels, and
  recursive data is a flat table with indices. It is fast, and it is more to write.
- **No closures at run time but one, no concurrency, no interfaces you implement.** A window
  procedure is the one function a host calls back; a function a host calls only during the call (a
  sort comparator, a directory walker) is researched and not built. You can pass a host object where
  a host interface is wanted; you cannot build one.
- **No borrowed results.** A host call that lends its own buffer until the next call (`bufio`'s
  `Peek`, `Scanner.Bytes`) is not declared ([borrow-research.md](docs/borrow-research.md)).
- **Strings are thin**: concatenation, conversion at a boundary, ASCII and UTF-8 from bytes. Text
  programs so far work in bytes.
- **Maps take integer keys only.**
- **The browser's own API**, the Web platform, is not declared yet, and the JavaScript output is
  tested under Node, as an engine.
- **Windows** is the target still being completed. Missing:
  - a dictionary keyed by strings;
  - printing a floating-point number;
  - a handle as its own type;
  - text read back from a window;
  - memory Windows keeps past a call (the find dialog's);
  - linking a function whose stack frame is larger than a page.

  [TinyRetroPad](https://github.com/plummersSoftwareLLC/TinyRetroPad), the Notepad-style editor it
  is held to, is being written.
- **No packaging, no editor support**, and error messages written for the compiler's authors rather
  than for newcomers.

## Current work

*Updated 2026-11-19.* The latest assessment is [docs/assessment-2026-11-19.md](docs/assessment-2026-11-19.md);
each earlier plan, and how it ended, is in the assessments in [docs/](docs/).

**The plan in force** (approved by hamza, 2026-11-19), in order:
1. **TinyRetroPad's body in the language**, module by module, the window procedure last. The window
   procedure itself already runs ([slot-2026-11-19](gauntlet/results/slot-2026-11-19.md)), and so
   does the status line ([family-2026-11-19](gauntlet/results/family-2026-11-19.md)).
2. **The callback decisions**: the window procedure's taken and built; those for a function a host
   calls only during the call still to discuss.
3. **The capacity facts** of maps: waiting for a program they fail.
4. ~~A time limit on every program a test runs~~ **done** ([timeouts-2026-11-19](gauntlet/results/timeouts-2026-11-19.md)).
5. **The project's instructions to its assistant** kept as the state of things.
6. **The words that bought nothing**: a first pass done ([words-audit](docs/words-audit.md)).

**Decided and being built** (hamza, 2026-11-19), in order:
1. **The memory model finished** ([views-finish-research.md](docs/views-finish-research.md)):
   - ~~a family of calls indexed by a constant, `SendMessageA`'s messages~~ **done** (ADR 0141);
   - a handle as a host type, compared with `==`, its failure an `Option`, with TinyRetroPad's file
     load and save;
   - two disjoint segments of one buffer lent at once;
   - a pointer the host keeps, the mode `kept`, with TinyRetroPad's find and replace.
2. **128-bit integers in the language's own operations**, waiting on the unsigned word read as an
   integer, which needs four realizations.
3. **C structs' remaining fields** (ADR 0136), with TinyRetroPad's modules.

**Built since the plan was set**, each with its program:
- a step of a loop after a nested loop's result is taken apart, so `wc` is written as Go writes it
  (ADR 0139);
- a buffer a function borrows and hands back, `(b' ∈ Array(V) linear)` (ADR 0140);
- `SendMessageA`'s messages as declarations of their own (ADR 0141);
- **the bit operations above the word** (ADR 0082), with Stein's binary gcd and Kernighan's
  popcount. Popcount over 3⁴⁹⁰⁰…3⁴⁹⁹⁹ runs on Windows in 0.64× Go's time
  ([bigbits-2026-11-19](gauntlet/results/bigbits-2026-11-19.md)), and Euclid's subtraction is now
  proven on every target;
- **a target that names its CPU** (ADR 0142): an integer dot product on `windows-v3` in 0.60–0.69×
  the baseline's time. At the baseline ours is hand-written C's ([cpu-2026-11-19](gauntlet/results/cpu-2026-11-19.md)).

**To come back to, with hamza**:
- a block after a call for a resource taken at its start and given back at its end, `f(a…) { x̄ -> b }`
  ([scopeforms-research.md](docs/scopeforms-research.md));
- callbacks a host calls only during the call;
- how a variadic of places is written;
- text read back from Windows;
- a table's elements narrower than a word on Windows;
- a stack frame larger than a page there.

**The standing goal** is the Go standard library, package by package, program first. **The standing
measure** is Windows faster than Go, the gap work owed.

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
go run ./cmd/build -target=windows -o stein.exe examples/big/stein.oro   # Stein's gcd over Fibonacci numbers
go run ./cmd/check                                      # every check: tests, emission baseline, all targets
go run ./cmd/oro -target=go examples/native/dot.oro    # what a program reduces to
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
| [emit/](emit/) | the type checker, the refinement layer (bounds, preconditions, claims), contracts, the target loader, and the big-integer libraries written in the language |
| [presburger/](presburger/) | the Omega test, a complete decision procedure for linear integer arithmetic |
| [targets/](targets/) | what each host provides, declarations and never compiler code: `go`, `js`, `java`, `llvm`, and the Windows family, `win32` with its members `windows`, `windows32` and `windows-v3` |
| [lib/](lib/) | portable modules such as `os` and `io`, and each host's implementation of them |
| [examples/](examples/) | the programs, among them [io/](examples/io/) (`wc`, `cmp`, `jsonfmt`, `freq`), [os/](examples/os/) (`du`, `ls`), [big/](examples/big/) (`fib`, `stein`, `popcount`), [hash/](examples/hash/) (FNV-1a, CRC-32), [native/](examples/native/) (the benchmarks' sources, and Windows programs: a window procedure, a status line) |
| [gauntlet/](gauntlet/) | hand-written references, benchmarks, the differential suite, and [results/](gauntlet/results/) |
| [docs/book/](docs/book/) | the book |
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
