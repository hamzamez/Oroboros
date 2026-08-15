# Building

Written before the code, per [state.md §6](state.md).

> **Status, 2026-08-15. Built, for Go.** `cmd/build` follows imports, reduces the export named
> `main`, emits one `package main`, and runs the host toolchain. `examples/hello.oro` produces a
> binary that prints and exits.
>
> **Requirement 6 is measurable for the first time, and the answer is exact parity:**
> **2,465,280 bytes**, byte-identical to the hand-written Go equivalent — because we emit the same
> Go and ship no runtime.
>
> Two things fell out. `(fn () …)` now parses, which fixes an error message that recommended a
> form the reader rejected. And **no backend could emit a string literal** — the language has had
> one since target templates needed it, specified in [strings.md](strings.md), and `hello.oro` is
> the first program to use one.
>
> **All three targets build and run** `examples/hello.oro`. JS needed a new idea: it has **no
> compile step**, so the emitted module *is* the deliverable and the target declares
> `(artifact "main.mjs")` rather than a build. Java produces a class **directory**, not a file.
> Three hosts, three notions of what a deliverable is — §3.
>
> Not yet built: the doctor (§7).

This project calls itself a *language and build system*, and the build system has never existed.
`cmd/gen` emits **one function into a package that already exists** — the gauntlet's. Nothing
produces an artifact, which is why [requirement 6](../design-direction.md), *small binaries*, has
been unfalsifiable for our own output since the beginning.

---

## 1. What a build is

> **A build takes one entry source, follows its imports, reduces its entry point against a target,
> emits one artifact, and hands it to the host's own toolchain.**

Everything in that sentence already exists except the last two clauses.

## 2. The entry point: `main`, exported, nullary

### What the literature does

| | distinguished by |
|---|---|
| C | the **name** `main`, with a fixed `argc/argv` signature |
| Haskell | the name `main` **and its type** `IO ()`, in a module that must be called `Main` |
| Go | the function name `main` **and** the package name `main` |
| Rust | `fn main` in the **crate root** |
| ML, OCaml, Scheme | **nothing** — the top level runs, in order |

### What we take

> **An export named `main` taking no arguments is the program's entry point.**

That is Haskell's answer with its known wart removed, and Rust's without the crate-root
requirement. Distinguished by **name and arity**, never by module — because a module path is the
*library's* identity, and overloading it with "this is a program" conflates two things. Haskell's
mandatory `Main` module is exactly that mistake and is why the idiom is so often complained about.

### What we reject, and why

**The ML/Scheme answer — no `main`, the top level runs.** Excluded by the architecture rather than
by taste: *our top level is where reduction happens*. A top-level term is reduced at compile time,
not executed at run time. "Run the top level" has no meaning in a language whose top level is the
staging boundary.

**C's `argc`/`argv` in the signature.** Command-line arguments are I/O, and I/O is a library — a
target that has arguments declares a primitive for them and `main` calls it. Baking them into the
entry signature would put one host's convention in the language.

**Haskell's typed entry, `IO ()`.** We have no type system and no `IO`. Arity is the part of a type
we do have, and zero arguments is the whole of the requirement.

### The reader change this forces, and the bug it fixes

`(fn () …)` does not currently parse — an empty list is rejected as a term. So today:

```
$ oro examples/x.oro
the body of main is a computation, not a value, so unfolding it would repeat its effects
  Wrap it in (fn () …) and apply it, or bind it with let at the point of use.
```

**That advice cannot be followed.** The error message names a form the reader refuses. Allowing an
empty *parameter list* — while still rejecting `()` as a term — fixes a live defect and is what
makes a nullary `main` writable at all.

```lisp
(use io)
(def main (fn () (io.print-line "hello")))
(export main)
```

`main`'s body may be impure; the λ makes it a **value**, so
[ADR 0010](../decisions/0010-effects-as-structural-rules.md)'s restriction on definition bodies is
satisfied without an exception. The effects happen when the emitted entry applies it.

## 3. Modules do not appear in the output

Reduction is whole-program by construction and fusion crosses every module boundary
([modules.md §9](modules.md) deferred separate compilation deliberately). So there is no reason to
preserve source structure in the artifact:

> **A build produces one artifact, shaped by the target. Module structure is a *source*
> organisation and is erased by resolution, exactly like every other name.**

Which is the same result as [ADR 0011](../decisions/0011-modules-add-nothing-to-the-reducer.md),
arriving at the other end: **modules add nothing to the reducer and nothing to the output.** They
are purely a naming discipline.

The target decides the shape, and all three already have an implementation:

| | artifact |
|---|---|
| Go | one `package main` with `func main()` calling the emitted entry |
| JS | one ES module, invoked by the host runtime |
| Java | one class with a `public static void main(String[])` |

Nothing here is a new mechanism — `emit.File`, `emit.JSFile` and `emit.JavaFile` already write
whole files.

## 4. The toolchain is target data

Same shape as `import`, `pure`, `index` and `narrow`: a line in the target file, not Go.

```lisp
(build "go build -o %s %s")     ; artifact, source directory
(run   "%s")
```

A target that declares no `build` can still emit source — that is what `cmd/gen` does today, and
it stays the fallback for a target whose toolchain we cannot invoke.

### `artifact`, for a host with no compile step

```lisp
(artifact "main.mjs")           ; the emitted file IS the deliverable
```

JavaScript has no build: `node main.mjs` runs the source. So the emitted module is copied to the
output and `build`, if present, only checks it. **A target may declare either, or both.** This was
not predicted — it fell out of trying the second target, which is the usual way this project learns
what a declaration has to carry.

## 5. Out of scope, with reasons

- **Command-line arguments and input.** A library concern (§2). `print-line` is already a target
  primitive and belongs in an `io` module; anything more arrives the same way. This is the system
  working, not a gap.
- **Multiple artifacts, incremental builds, caching.** Reduction is whole-program; there is no unit
  to cache smaller than the program, and whether that scales is a
  [measurement](modules.md) nobody has taken.
- **Dependency fetching.** A module resolves on a search path. Where the path comes from is a
  packaging question and no program has one yet.
- **Cross-compilation.** The host toolchain decides; we pass through.

## 6. The doctor — later, deliberately

A build assumes `go`, `node` or `javac` is present, on the path, and new enough. When one is not,
the failure today is whatever the shell prints.

**A doctor is worth building and is not being built now.** It reports what a target needs, what is
installed, what version, and what is missing — the property being that it *tells you what is wrong
instead of giving you a headache*, which is the thing Flutter gets right and most toolchains do
not.

It is deferred for one reason: it can only diagnose requirements that exist, and the target files
have carried a toolchain command for exactly one day. What a target must *declare* about its
toolchain — the executable, a minimum version, how to ask for that version — should be read off
what the builds turn out to need, not designed in advance. That is
[ADR 0007](../decisions/0007-exploration-over-specification.md), and it is the same reason the
target file format was read off three backends rather than specified up front.

## 7. What this makes measurable for the first time

Requirement 6. Once a build produces a binary, *our* binary size can be compared against the
hand-written baseline in [size-2026-08-13](../../gauntlet/results/size-2026-08-13.md), which has
measured only host code until now. That is one of the eight original requirements moving from
asserted to checkable.
