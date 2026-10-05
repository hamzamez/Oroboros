# 0039 — The targets are Go, Windows, Android and the browser

Date: 2026-10-05
Status: Accepted (hamza). **Supersedes the target list of [ADR 0004](0004-first-targets.md)**,
which named Go, JavaScript and Java. Android's emission form, which 0004 fixed as Java source, is
reopened by [android-research.md](../android-research.md).

## Context

ADR 0004 (2026-08-13) chose the first targets by ecosystem: Go for backends, the browser for front
ends, Android for apps, all reached through source languages. Since then, four things have happened:
- the `windows` target became a fourth backend, x86-64 through MASM (ADR 0016);
- general-purpose.md named the platforms the language is for: apps on Windows and Android,
  websites, cloud backends;
- the JavaScript target was built, tested and benchmarked under **Node**, because Node is easy to run
  from a test. Its portable `os` cell is Node's `fs`, `process.argv` and `process.env`, and `io.print`
  is `process.stdout.write`;
- the Java target was built, tested and benchmarked on **HotSpot**, a desktop JVM.

So the hosts the code meets in practice had drifted from the platforms the language is for. hamza,
2026-10-05: "the primary target is golang, windows, Android …, the browser (I know it is easier to
test js using node, but our target is the browser and the API that we will support fully is the
browser's, other can add node as a third party target if they want, our system is designed for
this.) and golang of course."

## Decision

**The primary targets are four platforms:**

| target | what it is for | runtime | the API this project supports fully |
|---|---|---|---|
| **Go** | backends, tools | the Go runtime | Go's standard library, package by package |
| **Windows** | desktop applications | x86-64, no runtime | Win32 |
| **Android** | mobile applications | ART | the Android SDK |
| **the browser** | web front ends | a browser's JavaScript engine | the Web platform: the DOM, `fetch`, workers, and the rest of what browsers implement |

1. **The browser, not Node.** The JavaScript backend's host is the browser. Its supported API is
   the Web platform's. **Node is not a primary target.** A third party may add it as a target layer:
   `(target node …)` over the ECMAScript layer, which is what the layered target system
   (target-system.md, Δ_T = L₁ ▷ … ▷ Lₖ) is for. This repository's own Node declarations will move to such
   a layer, as its first example, instead of standing in for the browser.
2. **Android, not the JVM.** The JVM backend exists for Android, and ART is its runtime of record. A
   desktop or server JVM is not a primary target. That the same output runs on HotSpot is a
   convenience for testing, not a claim.
3. **ECMAScript is a layer, not a target.** The engine's language (numbers, `BigInt`, typed arrays,
   `Math`) is the same in every browser and in Node. Declarations of it stay below both. What is a
   browser's and what is Node's is decided by where a name lives, as everywhere else (ADR 0001).

## Why not

- **Keep Node as the JavaScript target, because it is easy to test.** The ease is real, and it is
  the reason the drift happened. A target that is easy to test but is not the one the language is for
  measures the wrong thing. The ECMAScript layer can still run in any engine; what a program does
  with the Web platform can only be checked in a browser.
- **Support Node and the browser equally.** Each is a large API. Supporting both fully doubles the
  surface that ADR 0022 says must be declared by hand and checked. The layered target system already
  lets a third party add Node without this project carrying it. ADR 0001's own premise is that a
  target is an ecosystem, chosen.
- **Keep the JVM as a primary target beside Android.** No program or plan here is for a server JVM,
  and Go is the backend target. A JVM target would be measured on HotSpot, whose optimizer is not
  ART's: android-2026-10-05 found a stencil residue on ART that HotSpot never showed.
- **Leave ADR 0004 standing.** Its target list no longer says what the project is for, and an ADR
  that is wrong in its first line misleads every reader after it.

## Consequences

- **The JavaScript target splits into layers**: ECMAScript, then the browser; Node is a third-party
  layer. The Node-specific declarations are few and known (surveyed 2026-10-05):
  - the portable `os` cell on JavaScript: `ReadFile` and `WriteFile` on `node:fs`, `Args` on
    `process.argv`, `Getenv` on `process.env`, `Buffer`;
  - `io.print` on `process.stdout.write`;
  - the commands that run JavaScript: `cmd/build`, the differential runner and the portable test.
- **Tests of the browser target run in a browser.** A program that touches only ECMAScript can keep
  running in any engine, and V8 is V8. A program that touches the Web platform needs a headless
  browser, and the differential suite's runner must learn to drive one. This machine has Microsoft
  Edge, a Chromium.
- **The portable `os` and `io` were designed for command-line programs, and two of the four targets
  have none.** A browser page has no files, arguments, environment, exit status or standard output.
  An Android application has no `main`; it has an `Activity` and its lifecycle. Go and Windows have
  command lines. So `os`'s portability claim shrinks to the targets where it means something, and the
  entry point of an application is a new question. On Android and in the browser it is an event loop
  delivering messages to a handler, a Mealy machine (concurrency-research.md §2.1).
- **Android is tested on ART.** `dalvikvm64` runs a DEX file in the emulator with no APK
  (android-2026-10-05). The differential suite can gain an `android` column the way it has a
  `windows` one.
- **Benchmarks of record move.** The browser's are run in a browser's engine; Android's on ART,
  in the emulator until a device is wired in. HotSpot's and Node's figures stay as history, not as
  the bar.
- **What 0004 said about hostility still holds.** The browser has no integers, ART no unsigned and no
  value types, Go no pointer arithmetic, and x86 no runtime at all. Four mutually hostile hosts keep
  the core honest.
