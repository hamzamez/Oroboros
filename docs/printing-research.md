# Research: printing, the Windows console, and how to organize Win32

2026-10-05. **Research, not a decision.** hamza asked three things:
- What is the Windows API for reading and printing in the terminal?
- Should the language have a printing library, as part of the standard library, done right, with the
  literature, the mathematics and the algebra? And what would it do in a browser, and on Android?
- How should the Windows API be organized: per DLL, per library, or mirroring the documentation?

Measurements: [win32org-2026-10-05](../gauntlet/results/win32org-2026-10-05.md).

The short version:
1. **On Windows, the terminal is written with `WriteConsoleW` and read with `ReadConsoleW` when the
   handle is a console, and with `WriteFile` and `ReadFile` when it is not.** Measured: the language's
   Windows printing uses `WriteFile` only, and garbles every non-ASCII string on this machine's
   console.
2. **"Printing" is three different things with three different algebras**: *rendering* a value as
   text (a pure function into the free monoid Σ*), *laying out* text (a document algebra), and
   *emitting* text to a sink (an effect). Only the first two are a portable library. Sinks differ
   across targets, and not only in name: a terminal is a **stream**, a monoid action of Σ*. A browser's
   console and Android's logcat are **logs**, sequences of records. A page or a screen is a **document**,
   updated by a Mealy machine. The browser has no stream at all.
3. **The text of a value differs across hosts in its digits, not only its notation.** Measured: 2e23
   is `2e+23`, `1.9999999999999998E23` (JDK 17 and ART) or `2.0E23` (JDK 21), and −0 is `"0"` in
   JavaScript. So a portable renderer must be specified exactly and implemented where the host does
   not already compute it: Windows and Android.
4. **Win32 should be organized by header**, the partition the documentation uses and the one that
   nearly refines the DLLs. Measured: 303 of 327 headers bind one DLL; the DLL partition is far
   coarser (kernel32 spans 45 headers). The DLL is a fact about linking, and each declaration already
   carries it as data.

---

## 1. The Windows console, answered

The console is an object with a screen buffer, an input queue and two code pages. A program reaches
it through handles.

**The handles.** `GetStdHandle(STD_INPUT_HANDLE | STD_OUTPUT_HANDLE | STD_ERROR_HANDLE)` (processenv.h)
gives the three standard handles. Each is a **console** or anything else (a file, a pipe), and
`GetConsoleMode` (consoleapi.h) decides which: it succeeds only on a console.

**Writing.**

| the handle is | write with | because |
|---|---|---|
| a console | **`WriteConsoleW`** (consoleapi.h), UTF-16 | the console's buffer is UTF-16 cells. Measured: correct for every character tried |
| a file or a pipe | **`WriteFile`** (fileapi.h), bytes | a byte stream. UTF-8 is the convention; the reader decides. `WriteConsoleW` **fails** on it ("The handle is invalid", measured) |

The third route, `SetConsoleOutputCP(65001)` and then `WriteFile`, is correct on the screen
(measured), but it changes a property of the **console**, which every attached process shares and
which outlives the program. Restoring it at exit is skipped by a crash. **It is a side effect beyond
the program, and is rejected.**

**Transcoding** UTF-8 to UTF-16 is `MultiByteToWideChar` (stringapiset.h), or a loop written in the
language. A program writes its text in pieces, so an encoding split across two writes must be
carried. Algebraically, encoding is a monoid homomorphism enc : Σ* → B*, with
enc(ab) = enc(a)·enc(b). But **decoding a write is not a homomorphism on arbitrary byte chunks**: a
sink that decodes each write separately breaks the associativity of writes. Go keeps the incomplete
code point (`lastbits` in `internal/poll/fd_windows.go`) and prepends it to the next write.

**Reading.**

| the handle is | read with | the default behaviour |
|---|---|---|
| a console | **`ReadConsoleW`**, UTF-16 | *cooked*: `ENABLE_LINE_INPUT`, `ECHO_INPUT` and `PROCESSED_INPUT` are on, so the console edits the line, echoes it, and returns it at Enter; Ctrl+Z at the start of a line is the end of input by convention |
| a file or a pipe | **`ReadFile`**, bytes | a byte stream |

Raw input clears `ENABLE_LINE_INPUT` with `SetConsoleMode`. Key events come from
`ReadConsoleInputW`. **Colour and cursor control** are virtual-terminal sequences, enabled by
`ENABLE_VIRTUAL_TERMINAL_PROCESSING`. Microsoft's guidance ("Classic console APIs versus virtual
terminal sequences") prefers VT sequences to the older attribute calls.

**Go's and Rust's runtimes do exactly this split**, on a console and off one. The language's Windows
printing (`lib/win/fmt.oro`) does only `WriteFile`. So the answer to "what is the API" is also a bug
report: **`print-str` of any non-ASCII text is garbled on a Windows console** (win32org §2).

The strings themselves: Win32's `W` functions take UTF-16, and a Windows string is a sequence of
16-bit units that need not be valid UTF-16 (WTF-16, with lone surrogates). That is ADR 0030's
situation exactly. The host's string is the host's type, and ours (Σ*) enters it totally (encode) and
leaves it by one total decode that replaces a lone surrogate. The target's `A` functions
(`GetCommandLineA`, `GetEnvironmentVariableA`) read through the ANSI code page, and garble text the
same way.

---

## 2. What printing is

### 2.1 Three operations, three algebras

1. **Rendering**, render : A → Σ*. Pure. The text is the free monoid Σ* (strings.md), and a renderer
   is **compositional** when the text of a compound is built from the texts of its parts, by a fixed
   context. For a product, render(a, b) = "(" · render a · " " · render b · ")". A compositional
   renderer is a homomorphism out of the free algebra of the value's type: rendering is a *fold*.
   Its correctness criterion is the inverse:

     **parse ∘ render = id**,

   so render is a section of a parser. That is the same law the IR's text obeys (ADR 0038,
   read ∘ print = id).
2. **Layout**: arranging rendered text into lines within a width. Its algebra is the document algebra
   of the pretty-printing literature (§2.4).
3. **Emission**, emit : Σ* → Sink → Sink. An effect, and a host capability. Its laws depend on the
   sink (§3).

Every library that conflates them pays somewhere. C's `printf` renders and emits in one call; Java's
`println` renders through `toString`, whose text is the class's to choose; JavaScript's `console.log`
emits objects, not text. **Separating the three is the design.**

### 2.2 Rendering integers

Positional notation in base b is a bijection between ℕ and the digit strings with no leading zero
(Euclidean division gives existence and uniqueness), so render is exact and parse ∘ render = id
holds by construction. The digits come from repeated division by b, d = ⌈log_b(n+1)⌉ steps, or by 100
with a two-digit table, half the divisions. Division by a constant compiles to a multiplication
(Granlund and Montgomery 1994).

**Every host agrees on integers within the word**, and the language already renders them itself on
Windows (`lib/win/fmt.oro`, 30 lines). Above the word, rendering is the bignum's: host on Go, Java
and JavaScript, the limb library on Windows.

### 2.3 Rendering floats: the shortest round trip

A double x stands for an interval of reals that round to it, under round-to-nearest-even. **The
right decimal is the shortest one in that interval, and the closest to x among the shortest**: it
reads back to x (parse ∘ render = id), and nothing shorter does.
- Steele and White (1990) defined this and gave Dragon4, with Gay (1990) the correctly rounded reader
  and writer.
- Fast algorithms followed:
  - Grisu (Loitsch 2010), fast for most inputs, falling back to the exact method for the rest;
  - **Ryū** (Adams 2018), always fast and exact;
  - **Schubfach** (Giulietti 2018–2020), JDK 19's;
  - Dragonbox (Jeon 2020).

**Measured, the hosts do not agree** (win32org §3):
- **Digits:** JDK 17 and **ART** print 2e23 as `1.9999999999999998E23`, which round-trips but is not
  shortest (JDK-4511638, fixed in JDK 19 and not on Android).
- **Notation:** exponent thresholds (JavaScript `10000000`, Java `1.0E7`, Go `1e+07`), the exponent's
  spelling, a trailing `.0`, Java's two significant digits at least (`4.9E-324`).
- **Zero's sign:** JavaScript's `String(-0)` is `"0"`.

ECMA-262 specifies JavaScript's conversion exactly (Number::toString: shortest digits, decimal between
10⁻⁷ and 10²¹). Go's `'g'` with precision −1 is shortest.

So a portable renderer:
1. **specifies the digits**: shortest, then closest, by Steele and White's definition;
2. **specifies one notation**: when to use an exponent, how to spell it, and that −0 renders as `-0`;
3. **is implemented where the host does not compute it**:
   - Go, JavaScript and JDK 19+ give the digits, and the notation is a few lines over them;
   - **Windows and Android do not.** Win32 has no conversion at all, and the C runtime's `%g` is not
     shortest. So the digit algorithm is written once, in the language (Ryū or Schubfach needs a
     64×128-bit multiply, which `lib/num/u128.oro` has), and checked once, as the limb library is
     (ADR 0035).

**Staging must not change the text** (ADR 0009). A float rendered at compile time must be the bytes the
program renders at run time, which is one more reason for one specified algorithm instead of each
host's.

### 2.4 Layout: the document algebra

Pretty printing is the best-developed algebra in this whole area:
- **Oppen** (1980) gave the streaming algorithm, linear time with bounded lookahead.
- **Hughes** (1995) designed a combinator library from algebraic laws.
- **Wadler**'s "A prettier printer" (2003) is the standard algebra. A document is built from:
  - `text s`;
  - `line`, a newline or a space;
  - `nest i d`;
  - concatenation `<>`, associative with unit `nil`;
  - `group d`, which flattens d when it fits.

  Its laws (`text (s·t) = text s <> text t`, `nest` distributes over `<>`, `nest i (nest j d) =
  nest (i+j) d`) make the layout choice a search over the flattenings `group` allows, and Wadler
  derives a greedy linear-time solution from them.
- **Bernardy** (2017) showed greedy layout is not optimal and gave an optimal one with a measured
  cost.
- Swierstra and Chitil (2009) gave a linear, bounded, functional version.

This is a **library** over rendering, for structured output: a JSON value, an IR file, a table.

### 2.5 Format strings, and why staging makes them free

`printf`'s type depends on the *value* of its first argument: Π(f : Format). Args(f) → Σ*. That is a
dependent type, which Augustsson's Cayenne (1998) expressed directly. Danvy (1998) got the same typing
with continuation-passing combinators and no dependent types ("functional unparsing").

Rust's `format_args!` and Zig's `std.fmt` parse the format at compile time: Zig's `comptime` is
exactly staging. **In this language a literal format string is static.** Staging can parse it, check
each argument's type against its directive, and leave a residual of renderings and concatenations,
with no format string at run time. Go's `vet` printf check is a lint over a run-time parse; here the
parse would not exist at run time.

Interpolation, `(concat "x = " (render x))`, is the free monoid written directly. A format string adds
width, precision and alignment, which are layout.

---

## 3. Sinks: what "print" means on each target

A sink is where emitted text goes, and **sinks have different algebras**:

| sink | its algebra | the law that matters |
|---|---|---|
| **stream** (a terminal, a file, a pipe) | a monoid action of B* (or Σ*) on the sink's state | write(a); write(b) = write(a·b). So writes may be buffered and coalesced |
| **log** (records) | the free monoid of *records*, each with a level, a tag and a time | log(a); log(b) ≠ log(a·b). A record is a unit, and coalescing two changes what is logged |
| **document** (a screen, a page) | a tree, replaced or patched by the program's step | it is the *state* of a Mealy machine (concurrency-research.md §2.1), not a sequence |

What each target has:

| | Go | Windows | Android | the browser |
|---|---|---|---|---|
| **stream** | `os.Stdout`, `os.Stderr` | the console or a file, by §1 | `System.out` exists, but in an application process it goes **to logcat**, as records tagged `System.out`. Only under `dalvikvm` is it a stream | **none** |
| **log** | `log`, `slog` (to standard error) | `OutputDebugStringW` (to a debugger), the Event Log | `android.util.Log` (logcat: priority, tag, message) | `console.log`, `.warn`, `.error`: records of **objects**, shown in developer tools |
| **document** | none built in | a window, drawn in its window procedure (`DrawTextW`, `SetWindowTextW`) | a `View` (`TextView.setText`) or Compose | the DOM (`element.textContent`) |

**So the answer to "what does print do in the browser, and on Android" is: there is no print.**
- **The browser has no stream.** Mapping `print` to `console.log` would claim an equivalence that is
  false in two ways: a record is not a stream, and the user never sees developer tools.
- **On Android a stream becomes a log**, by the platform's redirection.

lib/os/README.md's rule applies: *a portable name is a claim about several hosts agreeing*. A
portable `print` over sinks that do not agree would be the `split-words` mistake again: one name,
different meanings.

What the capability graph already does instead:
- **`stream` is a capability Go and Windows provide**, and Android under `dalvikvm` for tests;
- **`log` is a capability all four provide**, with the record algebra;
- **`document` belongs to the application model**, the Mealy machine of the concurrency research.

A program that writes to a stream is portable to the targets with one, and the compiler computes it
(ADR 0001). Reading is the mirror: a stream source on Go and Windows, and on Android and in the
browser input arrives as events, as messages to the step.

---

## 4. Organizing Win32

Win32 has **four partitions** of its names, each the API's own:

| partition | what it is | who uses it |
|---|---|---|
| **header** | what a C program includes: `consoleapi.h`, `fileapi.h` | the compiler; **the documentation**: every page is `…/api/HEADER/nf-HEADER-NAME` |
| **DLL / import library** | what the loader binds: kernel32.dll, through kernel32.lib | the linker. Each declaration here already carries it: `(import "WriteFile")` and `lib` (target-files.md §6a) |
| **API set** | the loader's contract (`api-ms-win-core-file-l1-1-0`), resolved to an implementing DLL on each machine | the loader |
| **metadata namespace** | `Windows.Win32.System.Console`: Microsoft's machine-readable description for language projections (the win32metadata project). Curated by feature area, grouping related headers | Rust's `windows` crate, C#'s CsWin32, Zig's zigwin32 |

**Measured** (win32org §1):
- the header partition nearly refines the DLL partition: 303 of 327 headers bind one DLL, holding
  77.9% of the names;
- the DLL partition is coarse: kernel32 takes 1,236 names from 45 headers, and only 42.1% of names
  live in a DLL that comes from one header.

**So the DLL is the wrong unit for a module.** It groups files, processes, the console, memory and
national-language support under one name, and it is a linking fact that each declaration already
records.

**ADR 0025 says a module path is the host's path.** On Go that is the import path, on Java the
package. On Win32 the host's own path, the one its programmers write and its documentation is keyed
by, is the **header**. Two shapes follow:

- **W1, by header**:
  - `windows/consoleapi`, `windows/fileapi`, `windows/processenv`, `windows/winuser`;
  - the module is the header with `.h` dropped, and the function keeps its name;
  - `WriteConsoleW` lives at `windows/consoleapi`, and its documentation is
    `…/api/consoleapi/nf-consoleapi-writeconsolew`, a one-to-one map. That is what "easy to look up"
    means.
  - The survey's emitter already writes modules this way (`win/HEADER`).
  - The cost: a few headers are huge (`WinUser.h` 715 names, `WinBase.h` 463), and some topics
    span several headers (the console is `consoleapi`, `consoleapi2`, `consoleapi3` and `wincon`).
- **W2, by metadata namespace**: `windows/System/Console`, `windows/Storage/FileSystem`.
  - Curated groupings, the names Rust and C# programmers know, with the documentation linked from
    the metadata.
  - The cost: a second source of truth, a NuGet package outside the SDK, and Microsoft's
    projection's opinion rather than the C API's own path.
  - Not measured: the metadata was not downloaded.

**W1 is ADR 0025 applied as written**, and the documentation mirrors it exactly. W2 is friendlier for
topics that span headers. A module may contain a module (target-files.md §1a), so W1 can also offer
W2's grouping as a module that re-exports several headers, without a second source of truth.

**The `W` functions, not the `A` ones**, by §1's string argument. The target's existing declarations
are per DLL (`windows/kernel32`, `windows/msvcrt`) and use `A` functions (`GetCommandLineA`). The
C runtime (`msvcrt`, `ucrt`) is not Win32, and would be its own host namespace, organized by its own
headers (`stdio.h`).

---

## 5. Candidates for the printing library

| | what | verdict |
|---|---|---|
| **P1** | each host's own printing, no portable library (as now: `fmt.Println`, `console.log`, `System.out`) | **killed by measurement**: the text of a float differs on every host, in its digits on Android; and the browser has no stream |
| **P2** | a portable `print` mapped to each host's nearest sink (`console.log` in the browser, `Log.i` on Android) | **killed by the algebra**: a log is not a stream, and the name would claim an agreement that does not exist (§3) |
| **P3** | **a portable, pure rendering library** (`render`: integers, floats by shortest round trip with one notation, strings, and the language's own data: tuples, variants and tables, rendered from their types), **plus sinks declared truthfully as capabilities** (`stream`, `log`, `document`) per target | **the hypothesis** |
| **P4** | P3 plus a document algebra (Wadler) for layout | a library on P3, when a program prints structure |
| **P5** | format strings parsed at staging (§2.5) | a surface over P3's rendering, free at run time; with interpolation as the plain form |

Rendering the language's own data from its type is what Haskell derives for `Show` and Rust for
`Debug`. Here the residual is monomorphic and every type is known at staging, so the renderer for a
type is a fold generated at compile time, and nothing is dispatched at run time.

---

## 6. Measurements and programs that decide

| | what | decides | status |
|---|---|---|---|
| M1 | Win32's partitions by header and by DLL | W1 against per-DLL | **done**: the header nearly refines the DLL |
| M2 | the console's routes, in a real console | the Windows stream sink | **done**: `WriteConsoleW` on a console, `WriteFile` off one; the code page route rejected |
| M3 | a float's text on five hosts | whether rendering must be specified and implemented | **done**: digits differ on JDK 17 and ART; notation differs everywhere |
| M4 | **Ryū or Schubfach written in the language** over `lib/num/u128`, against Go's `strconv` on every double class (subnormals, powers of ten, the boundaries), and its speed against the hosts' own | P3's float renderer, and whether it is at parity where the host has one | proposed |
| M5 | the Windows stream sink rebuilt: `GetConsoleMode`, then `WriteConsoleW` with a carried partial code point, or `WriteFile`, and reading the same way; tested through `conpty.go` | the bug in §1, and the first sink | proposed |
| M6 | the win32metadata namespaces against the headers: how many namespaces span several headers, and which headers several namespaces | W1 against W2 | proposed: needs the NuGet package, with permission to download it |
| M7 | **a program that prints**: a table of numbers and text, in a terminal on Go and Windows, to a log on all four, and into a document in a browser page | P3's surface; the sinks' interfaces | proposed |

---

## 7. Questions for hamza

To be decided when a program needs them:

1. **P3**: a pure rendering library plus truthful sinks, or something else?
2. **Win32 by header (W1), or by metadata namespace (W2)?** And may the metadata be downloaded to
   measure M6?
3. **The float notation**: which one? Go's, JavaScript's (an ECMA standard), or the language's own?
4. **The Windows sink (M5) now**? It is a bug in what ships, not only a design question.

---

## References

- Adams, U. "Ryū: fast float-to-string conversion". PLDI 2018.
- Augustsson, L. "Cayenne — a language with dependent types". ICFP 1998.
- Bernardy, J.-P. "A pretty but not greedy printer". ICFP 2017.
- Danvy, O. "Functional unparsing". *JFP* 8(6), 1998.
- Gay, D. M. "Correctly rounded binary-decimal and decimal-binary conversions". AT&T Bell
  Laboratories, Numerical Analysis Manuscript 90-10, 1990.
- Giulietti, R. "The Schubfach way to render doubles". 2020; JDK-4511638, fixed in JDK 19.
- Granlund, T., Montgomery, P. L. "Division by invariant integers using multiplication". PLDI 1994.
- Hughes, J. "The design of a pretty-printing library". *Advanced Functional Programming*, LNCS 925,
  1995.
- Jeon, J. "Dragonbox: a new floating-point binary-to-decimal conversion algorithm". 2020.
- Loitsch, F. "Printing floating-point numbers quickly and accurately with integers". PLDI 2010.
- Microsoft. "Console API reference"; "Classic console APIs versus virtual terminal sequences";
  "Console virtual terminal sequences"; the win32metadata project. learn.microsoft.com and GitHub.
- Oppen, D. C. "Prettyprinting". *TOPLAS* 2(4), 1980.
- Steele, G. L., White, J. L. "How to print floating-point numbers accurately". PLDI 1990.
- Swierstra, S. D., Chitil, O. "Linear, bounded, functional pretty-printing". *JFP* 19(1), 2009.
- Wadler, P. "A prettier printer". *The Fun of Programming*, 2003.
- Ecma International. ECMA-262, Number::toString.
