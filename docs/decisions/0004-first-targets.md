# 0004 — Go, JavaScript, and Java/Android first; C deferred

Date: 2026-08-13
Status: Accepted

## Context

The first draft recommended a **C backend first**, on the reasoning that one backend reaches
Windows, Android NDK, iOS, Linux, and embedded, with host C compilers supplying the
optimization.

[0001](0001-parasite-model.md) invalidates that reasoning. The desired first ecosystems are Go
for backend software, the browser for front end, and Android for apps.

## Decision

First targets, in order: **Go**, **JavaScript**, **Java** (→ dex → Android).

C is deferred until iOS, embedded, or native desktop is actually wanted.

Android is reached via **Java source**, using the standard `javac → d8 → apk` toolchain.

## Why not

**C first.** Go's standard library is not reachable through C. Under the Parasite model, a
backend that reaches every *machine* is worth much less than a backend that reaches an
*ecosystem*. The three chosen targets need no C anywhere in their toolchains.

**Kotlin for Android.** Kotlin has the better ecosystem story (Compose, coroutines) but is a
much larger and less stable emission target than Java, with a slower compiler. Java gives full
Android API access at a fraction of the emission complexity. Kotlin can be added later as a
*separate* target — under [0001](0001-parasite-model.md), having both is normal.

**Dex or Dalvik bytecode directly.** Skips R8's optimization and shrinking, and chases a
moving, under-documented format, in exchange for removing one well-maintained toolchain step.

**JVM bytecode directly.** Defensible later for server-side JVM, but it means implementing the
class file format for no immediate gain, and Android needs dex conversion regardless.

## Consequences

- The original argument for C first — *get a hostile second backend early to keep the core
  honest* — survives and is better served here. These three hosts are already mutually
  hostile:

  | Target | Hostility |
  |---|---|
  | Go | GC, no pointer arithmetic, restricted `goto` |
  | JavaScript | No integers, no structs, no int64, GC |
  | Java | No unsigned, no value types, no `goto`, no tail calls |

- **JavaScript is the most hostile host available**, which makes it the best forcing function.
  A core that survives JS survives anything. It should be the *second* backend built, before
  any front-end features are added.
- Goroutines are a significant part of why Go is worth targeting, which puts pressure on the
  decision to defer concurrency.
- No target in the initial set has manual memory management, so the allocator design will not
  be exercised until C arrives. Watch for the core accidentally assuming a GC.
