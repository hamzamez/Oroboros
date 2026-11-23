# 0. From a file to a program

Before any of the language, the tools. This chapter is about what happens to a file between
writing it and running it: which command does what, what the compiler does in what order, why
the same file gives Go on one target and LLVM IR on another, and how to try all of it in a
directory of your own without touching the repository.

Like the tour that follows, this chapter is a map, not a brick. It uses words the later chapters
explain (a *range*, a *claim*, *reduction*), and each is named here only so you can recognize it
when the compiler prints it. Every block below is checked: `go run docs/book/check.go
docs/book/00-compiling.md` re-runs it. The exceptions are the shell commands and the code written
outside the repository in §0.5 and §0.6, which were run by hand on 2026-11-23. Checked 2026-11-23.

---

## 0.1 What you need

The compiler is written in Go, and is run from the repository's root. Each target then needs its
host's own tools, since a target's output is that host's source and the host compiles it:

| target | the compiler writes | what runs it | needs |
|---|---|---|---|
| `go` | a Go file | `go build` | Go |
| `js` | an ES module (`.mjs`) | `node` | Node.js |
| `java` | a Java class | `javac`, `java` | a JDK |
| `windows` | LLVM IR (`.ll`) for x86-64 | `clang`, `link` | LLVM, and MSVC's linker |
| `windows32` | LLVM IR for i686 | `clang`, `link` | the same |
| `tutorial` | nothing | `oro` alone | nothing more |

You need Go for anything, and the others only for the targets you build. Check what you have:

```bash
go version
node --version
javac -version
clang --version
```

A windows program links against `kernel32` alone, with no C runtime, so nothing else has to be
installed where it runs.

## 0.2 One file, five tools

Here is a program:

<!-- file docs/book/code/compile/hello.oro -->
```go
use io

export fn main() {
    io.print_line("hello")
}
```

**`build`** makes something you can run, and the book's `run` blocks are what that prints:

```bash
go run ./cmd/build -target=go -o hello.exe docs/book/code/compile/hello.oro
./hello.exe
```

<!-- run go docs/book/code/compile/hello.oro -->
```
hello
```

The same command with `-target=js` writes `hello.mjs`, run with `node hello.mjs`; with
`-target=java` it writes a directory of classes, run with `java -cp hello Main`; with
`-target=windows` it writes `hello.exe`:

<!-- run js docs/book/code/compile/hello.oro -->
```
hello
```

<!-- run java docs/book/code/compile/hello.oro -->
```
hello
```

<!-- run windows docs/book/code/compile/hello.oro -->
```
hello
```

The other four tools show what happened on the way. They are easiest to see on a smaller file, one
function and no `main`:

<!-- file docs/book/code/compile/twice.oro -->
```go
export fn twice_plus(n ∈ 0..100) (Int) { n * 2 + 1 }
```

**`gen`** writes the host's source for each `export`, without building anything. Its arguments are
the source, the target and the file to write; `-name` prefixes every function it emits:

```bash
go run ./cmd/gen -name compile docs/book/code/compile/twice.oro go twice.go
```

<!-- gen go docs/book/code/compile/twice.oro -->
```go
note: compile-twice-plus: 2 of 2 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func CompileTwicePlus(v0 int) int {
    v10 := (v0 * 2)
    v12 := (v10 + 1)
    return v12
}
```

**`oro`** reduces each `export` and prints what is left, the program as the compiler holds it
before any target's code is written. `-j` prints it in the language's own notation, and without it
`oro` prints the *core term*, the compiler's internal form, in which everything is a name, a
literal, a `fn` or an application:

```bash
go run ./cmd/oro -j -target=go docs/book/code/compile/twice.oro
```

<!-- oro go docs/book/code/compile/twice.oro -->
```go
twice_plus =
fn(n) { n * 2 + 1 }
```

<!-- core go docs/book/code/compile/twice.oro -->
```
twice-plus =
(fn (n) (+ (* n 2) 1))
```

**`intervals`** says what the compiler proved about each integer operation, and which loops it
proved to end. `-v` lists what it could not prove, here nothing:

```bash
go run ./cmd/intervals -v docs/book/code/compile/twice.oro go
```

<!-- intervals go docs/book/code/compile/twice.oro -->
```
  twice-plus                 2/2   ops  0/0 loops
twice.oro                  2/2   ops (100.0%)  loops 0/0 terminate
```

**`portable`** compiles a file for every target and says which accept it:

```bash
go run ./cmd/portable docs/book/code/compile/twice.oro
```

<!-- portable docs/book/code/compile/twice.oro -->
```
  go       accepted  gen-twice-plus: 2 of 2 integer operations bounded; 0 of 0 loop(s) proven terminating
  js       accepted  gen-twice-plus: 2 of 2 integer operations bounded; 0 of 0 loop(s) proven terminating
  java     accepted  gen-twice-plus: 2 of 2 integer operations bounded; 0 of 0 loop(s) proven terminating
  windows  accepted  gen-twice-plus: 2 of 2 integer operations bounded; 0 of 0 loop(s) proven terminating
portable to go, js, java, windows
```

A sixth, `go run ./cmd/check`, is the repository's own gate: it runs every test, recompiles every
example on every target and compares each with what was recorded. It is for changing the compiler,
not for using it, and takes about a quarter of an hour.

## 0.3 The stages

Between the file and the host's source the compiler does eight things, in order. Each can refuse
the program, and a refusal says which stage it came from by what it talks about. Here are the eight,
each with a program it stops.

**1. Reading.** The text becomes a tree of the language's forms. Only the spelling is checked: a
missing operand, an unbalanced brace. The book names a file the reader refuses `.oro.txt`, since it
is not yet a program:

<!-- file docs/book/code/compile/no-read.oro.txt -->
```go
export fn run() (Int) { 1 + }
```

<!-- oro go docs/book/code/compile/no-read.oro.txt -->
```
oro: docs/book/code/compile/no-read.oro.txt: line 1: "}" begins no expression
```

**2. Desugaring and loading.** Each form becomes the few the core has (a block becomes a `fn`
applied, `if` an application of `if`), the modules named by `use` are read, and every name is
resolved: to a parameter, a definition, a module's member, or something the target provides.
A name that is none of these is refused, even in code nothing calls:

<!-- file docs/book/code/compile/no-name.oro -->
```go
export fn run() (Int) { totl + 1 }
```

<!-- oro go docs/book/code/compile/no-name.oro -->
```
oro: docs/book/code/compile/no-name.oro: in run: totl is not bound — it is not a parameter, not a definition, and not a primitive on this target
```

**3. Reduction.** Every function you wrote is applied where it is called and disappears, and every
computation the compiler can do now, it does. What is left is the **residual**: only what must
happen when the program runs, written with the target's own operations. This is the step the
whole language is built around, and chapters 2 to 6 are about it. A function given the wrong
number of arguments stops here:

<!-- file docs/book/code/compile/no-arity.oro -->
```go
export fn run() { fn(a, b) { a }(1) }
```

<!-- oro go docs/book/code/compile/no-arity.oro -->
```
oro: arity: expects 2 argument(s), given 1, in: fn() { fn(a, b) { a } }
```

**4. Checking the claims.** A header's sets, `n ∈ 0..100` and the result's `(0..150)`, are claims,
and the compiler proves each one or refuses the program. This one says the result is at most 150,
and `n * 2` reaches 200:

<!-- file docs/book/code/compile/no-claim.oro -->
```go
export fn grow(n ∈ 0..100) (0..150) { n * 2 }
```

<!-- gen go docs/book/code/compile/no-claim.oro -->
```
gen: grow: the body does not establish 0 <= result && result <= 150; its result is [0, 200]
```

**5. Lowering to the IR.** The residual becomes the compiler's intermediate representation, the
same for every target: values numbered `%0`, `%1`, …, each with its set, in nested regions.
`gen -ir FILE` writes it out. For `twice_plus`, `%0` is `n`, the first lines check that the caller
passed a value in `0..100`, and the last four are the arithmetic, each value's range already known:

```bash
go run ./cmd/gen -ir twice.ir -name compile docs/book/code/compile/twice.oro go twice.go
```

<!-- ir go docs/book/code/compile/twice.oro -->
```
(ir 1
  (target go)
  (stage P)
  (ops const add mul le if assume yield)
  (func compile-twice-plus (params (%0 (int 0 100))) (results int)
…
      (do (assume %3))
      (val (%9 (int 2 2)) (const 2))
      (val (%10 (int 0 200)) (mul exact %0 %9))
      (val (%11 (int 1 1)) (const 1))
      (val (%12 (int 1 201)) (add exact %10 %11))
      (yield %12))))
```

A function still left over after reduction has no form in the IR, since a host would have to build
it at run time as a closure, and is refused here:

<!-- file docs/book/code/compile/no-closure.oro -->
```go
fn scale(k) { fn(v) { v * k } }

export fn make(k ∈ 0..10) { scale(k) }
```

<!-- gen go docs/book/code/compile/no-closure.oro -->
```
gen: compile-make: lowering: a λ in value position is a closure (callbacks.md): fn(v) { v * k }
```

**6. Deciding.** Each integer operation is proven to stay inside the machine word, or held as an
integer of any size, and each loop is proven to end where it can be. `(int 0 200)` above is that
proof for `n * 2`, and `mul exact` its consequence: the multiplication cannot overflow, so it is one
machine instruction. This is what `gen`'s note counts, *2 of 2 integer operations bounded*, and what
`intervals` lists. The same decision chooses each value's representation, so the Java signature
below has an `int` parameter, since `n` fits one, and a `long` result, since the result is declared
`Int`.

**7. Printing.** One printer per host writes the IR as that host's source:

<!-- gen js docs/book/code/compile/twice.oro -->
```javascript
export function compileTwicePlus(v0) {
    return ((v0 * 2) + 1);
}
```

<!-- gen java docs/book/code/compile/twice.oro -->
```java
    public static long compileTwicePlus(int v0) {
        final int v10 = (v0 * 2);
        final int v12 = (v10 + 1);
        return v12;
    }
```

<!-- gen windows docs/book/code/compile/twice.oro -->
```llvm
define i64 @compile_twice_plus(i64 range(i64 0, 101) %v0) #0 {
```

`range(i64 0, 101)` is the proof from stage 6 handed to LLVM, so its optimizer may use it too.

**8. The host's own compiler.** `build` hands the printed source to `go build`, to `javac`, to
`clang` and `link`, or, for JavaScript, writes the module `node` will run. Nothing of the language's
is left at run time: no runtime library, no garbage collector of its own.

## 0.4 Targets

A **target** is a file of declarations in [targets/](../../targets/): which host it compiles for,
how big its machine word is, and what it provides: printing, files, the host's string functions,
and arithmetic itself. The language's own operations work on every target. A host's library
(`go/strings`, Win32's functions) is one target's, so a program that uses one compiles for that
target only, and `portable` says so:

<!-- file docs/book/code/compile/upper.oro -->
```go
use io
use go/strings

export fn main() {
    io.print_line(strings.ToUpper("hello"))
}
```

<!-- portable docs/book/code/compile/upper.oro -->
```
  go       accepted  
  js       refused   docs/book/code/compile/upper.oro: in main: `go/strings`.ToUpper is not bound — it is not a parameter, not a definition, and not a primitive on this target
…
portable to go — not js, java, windows
```

<!-- run go docs/book/code/compile/upper.oro -->
```
HELLO
```

Being unportable is no error. It is a property of the program that the compiler computes, and you
decide whether you mind.

**`tutorial` is a target that only teaches.** It names a few functions it knows nothing about, `f`,
`g` and `h` of one, one and two arguments, and `x()`, which stands for a value from outside, so
that reduction stops at them and you see the shape of what is left. It has no host, so `oro` is the
only tool that runs on it, and chapters 2 to 6 use it throughout:

<!-- file docs/book/code/compile/shape.oro -->
```go
export fn shape() { fn(n) { h(f(n), g(n)) }(x()) }
```

<!-- oro tutorial docs/book/code/compile/shape.oro -->
```go
shape =
fn() { n = x(); h(f(n), g(n)) }
```

On a real target those names mean nothing, and stage 2 refuses them:

<!-- oro go docs/book/code/compile/shape.oro -->
```
oro: docs/book/code/compile/shape.oro: in shape: h is not bound — it is not a parameter, not a definition, and not a primitive on this target
```

And asked to emit code, the tutorial target has nothing to emit with:

<!-- gen tutorial docs/book/code/compile/shape.oro -->
```
gen: target "tutorial" does not say which backend compiles it, and its name is not one, so it cannot emit code.
```

The windows family has three members: `windows` for x86-64, `windows32` for 32-bit x86 (i686),
and `windows-v3` for x86-64 processors with AVX2. A member fixes the instruction set and the size
of a pointer, and is chosen at the command line like any target.

## 0.5 `gen` or `build`

They answer different questions.

- **`build` makes a program.** The file must have `export fn main()`, which becomes the host's
  entry point, and the result is something to run.
- **`gen` makes a library.** Every `export` becomes one function of the host's, written to a
  source file that you put into a project of your own, in that host's language.

A file of functions with no `main` is a library, and `build` says so:

<!-- file docs/book/code/compile/no-main.oro -->
```go
export fn twice_plus(n ∈ 0..100) (Int) { n * 2 + 1 }
```

<!-- build go docs/book/code/compile/no-main.oro -->
```
build: docs/book/code/compile/no-main.oro has no entry point: a program needs `export fn main() { … }`
```

A function used only inside the program is not exported, and leaves nothing behind. Reduction puts
its body where it is called:

<!-- file docs/book/code/compile/main.oro -->
```go
use io

fn twice_plus(n ∈ 0..100) (Int) { n * 2 + 1 }

export fn main() {
    io.print_int(twice_plus(20))
}
```

<!-- run go docs/book/code/compile/main.oro -->
```
41
```

`build -keep` leaves the host source it compiled in a temporary directory and prints where. That is
how to read what a whole program became, where `gen` shows each export alone.

**Using `gen`'s output from your own code.** Run by hand on 2026-11-23, for the `sum_to` of the
tour's §1.2 generated with `-name calc`:

- **Go.** The file declares `package gauntlet`, the name of the repository's benchmark package, and
  `gen` has no flag to change it. Put it in a directory of that name inside your module:

  ```
  play/
    go.mod          module play
    main.go
    gauntlet/
      sum.go        written by gen
  ```

  ```go
  package main

  import (
      "fmt"

      "play/gauntlet"
  )

  func main() {
      fmt.Println(gauntlet.CalcSumTo(10)) // 45
  }
  ```

- **JavaScript.** The module exports each function by its camel-cased name:

  ```javascript
  import { calcSumTo } from "./sum.mjs";
  console.log(calcSumTo(10)); // 45
  ```

- **Java.** The class takes its name from the file `gen` writes, so `Sum.java` holds `class Sum`:

  ```java
  public class UseSum {
      public static void main(String[] a) {
          System.out.println(Sum.calcSumTo(10)); // 45
      }
  }
  ```

  compiled together with `javac -d classes Sum.java UseSum.java`, run with `java -cp classes UseSum`.

- **Windows.** A function exported for C callers is chapter 19 §19.10's.

## 0.6 Working outside the repository

The examples so far are files in the repository, which is how the book checks them. Your own
experiments belong somewhere else. A file you create inside the repository shows in `git status`,
and an `.oro` file there is taken for part of the corpus once it is committed. Two ways, both run by
hand on 2026-11-23 on Windows, in PowerShell and in Git Bash.

**Run the tools in place, with `go -C`.** `go -C DIR` runs the command as if from `DIR`, so the
compiler finds its `targets/` and `lib/` as it does from the root, while your file and its output
live anywhere. Give both as absolute paths:

```powershell
mkdir $env:TEMP\oro-play
cd $env:TEMP\oro-play
# write hello.oro here, then:
go -C C:\path\to\oroboros run ./cmd/build -target=go -o $PWD\hello.exe $PWD\hello.oro
.\hello.exe
```

```bash
mkdir -p ~/oro-play && cd ~/oro-play
go -C /path/to/oroboros run ./cmd/build -target=js -o "$PWD/hello.mjs" "$PWD/hello.oro"
node hello.mjs
```

`git status` in the repository stays empty: nothing was written there.

**Or build the tools once.** `go build` writes each tool as a program into a directory of your
choosing. Run from anywhere, they need to be told where the targets and the library are:

```powershell
go -C C:\path\to\oroboros build -o $HOME\oro-bin\ ./cmd/oro ./cmd/gen ./cmd/build ./cmd/portable ./cmd/intervals
$repo = "C:\path\to\oroboros"
& $HOME\oro-bin\build.exe -targets $repo\targets -path $repo\lib -target=go -o hello.exe hello.oro
```

Without `-targets` the tool looks for `targets/` in the current directory, and says so:

```
build: -targets names "targets", which is not a directory; a layer named on the command line must exist (an absent one would be skipped silently)
```

Rebuild the tools after you pull a newer compiler; `go -C … run` always uses the current one.

**Your own modules.** A file's own directory is searched first, so `use twice` finds `twice.oro`
beside the file that uses it. `-path` adds more directories, separated by `;` on Windows and `:`
elsewhere:

```
my-project/
  main.oro      use io
                use twice
                export fn main() { io.print_int(twice.double(21)) }
  twice.oro     pub fn double(n ∈ 0..1000) (0..2000) { n * 2 }
```

```
> build.exe -targets $repo\targets -path $repo\lib -target=go -o main.exe main.oro
> .\main.exe
42
```

`pub` puts a name in a module's interface, and chapter 4 is about modules.

---

## What to remember

- **`build` makes a program from `export fn main()`, and `gen` makes a library**, one host function
  per `export`. `oro` shows what reduction left, `intervals` what was proven, and `portable` which
  targets accept a file.
- **A program goes through eight stages**: reading, desugaring and loading, reduction, the claims,
  lowering to the IR, deciding, printing, and the host's own compiler. A refusal is from one of them,
  and says what it is about.
- **Reduction is the language.** Your functions are applied and gone before any code is written;
  what is left is what runs.
- **A target is a file of declarations**, and the host it names compiles what the compiler prints.
  A program using a host's library runs on that host only, and `portable` computes it.
- **`tutorial` only teaches**: reduction stops at its unknown names, and nothing is emitted.
- **Experiment outside the repository**, with `go -C` or with the tools built once and pointed at
  `-targets` and `-path`.

Next: [a tour](01-tour.md), the whole language at walking pace.
