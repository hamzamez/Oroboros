# Oroboros — Language Direction

Status: **working direction**. Revised 2026-08-13 after the Parasite reframing.
Individual decisions are recorded as ADRs in `docs/decisions/`.

---

## 1. What Oroboros is

A small language and build system that **parasitizes** target ecosystems rather than
abstracting over them.

The predecessor project was called Parasite, and the name states the thesis: if the best way
to build a Windows app is Win32, add a Win32 target and use Win32 fully. If it is .NET, add
a .NET target and use .NET fully. The language is a common notation for exploiting
ecosystems, not a portability layer that hides them.

### Requirements

1. Small language, easy to implement.
2. Compiles to many targets across many ecosystems.
3. Adding a target is low effort, doable by a third party.
4. Adding a target's APIs is declarative — close to a file listing function names.
5. No performance compromise versus writing directly in the target language.
6. Small binaries, small footprint.
7. Supports abstraction, so over time more is expressed in fewer tokens.
8. Easy for LLMs to write and reason about.
9. Bottom-up: a minimal core with abstraction above it, lowering at no cost.

### The reframing

Requirement 2 is **not** "write once, run anywhere."

Portability is a **property a program may or may not have**, computed and reported by the
compiler — not a guarantee the language enforces globally. A program that uses only portable
capabilities is portable. A program that uses Win32 is not, and that is a legitimate,
supported, first-class thing to write.

---

## 2. Why Shen hit a performance wall

This is retained because it predicts the next wall.

Shen lowers to **KLambda**, a core of 43 primitives; porting Shen to a new host means
implementing those primitives. The portability story worked. Performance did not.

KLambda is dynamically typed, closure-based, and cons-cell allocating — lambda calculus plus
symbols. So every value is boxed, every abstraction lowers to a heap-allocated environment,
and every host inherits all of it with no way for the host optimizer to undo it.

**The property that made Shen portable is the property that made it slow.** The substrate
sets the performance ceiling for every target at once.

Two conclusions, both load-bearing:

- The substrate must be **static, unboxed, and allocation-free**.
- Lisp syntax was never the problem. Keep it.

---

## 3. The capability graph

This is the central mechanism. It replaces the fixed layer tower from the first draft of this
document, and it answers portability, feature gaps, and "compiling up" with one construct.

### Definitions

- A **capability** is a named, typed unit of functionality: `float64`, `map`, `threads`,
  `matmul`, `fs.read`.
- A **module** declares the capabilities it requires.
- A **target** declares the capabilities it provides natively, plus **shims** that implement
  capability X in terms of capabilities Y and Z.

### Building

Cover the required capability set from (native provides ∪ reachable shims). If some
capability is uncovered, that is a build error naming exactly the gap. The gap is then closed
by one of: adding a binding, writing a shim, or deciding the program is not portable to that
target — all legitimate outcomes.

### Emit at the highest layer the target natively provides

Lower only as far as necessary.

If Go provides `map`, emission stops at `map` and the output uses Go's `map`. If C does not,
the same source keeps lowering into an actual hash table implementation. Same program,
different stopping point per target.

This is simultaneously the performance answer (the target's own idiom is what hand-written
code would use), the binary size answer (nothing shipped that the host already has), and the
ecosystem answer (requirement 2 as reframed).

### Both directions fall out for free

- **Feature missing on a target** — floating point on an integer-only machine — is a shim:
  `float64` implemented over `int32`.
- **Feature natively present on a target** — special hardware with `matmul` in silicon — is
  the *absence* of a shim. The target provides it, so nothing lowers.

"Compiling up" is not a separate mechanism.

This matters, because the obvious way to compile up is idiom recognition over low-level IR:
spotting a memcpy loop, re-vectorizing a scalarized one. That approach is notoriously brittle
and compilers have fought it for decades. The capability graph avoids it entirely by never
lowering the operation in the first place.

### Two tiers, or it does not scale

If every capability needed a specification tight enough that any two implementations were
interchangeable, then binding Go's standard library would mean thousands of specifications.
That is a dead end. So:

| | Tier 1 — Specified | Tier 2 — Raw bindings |
|---|---|---|
| Portability | Guaranteed, conformance-tested | None claimed |
| Cost to add | High — needs a spec and tests | Near zero — names, types, import line |
| Size | Deliberately small | Unbounded |
| Example | `map`, `float64`, `fs.read` | `fmt.Println`, `win32.CreateWindowExW` |

Tier 2 is requirement 4: a file listing function names, their types, and how the target
imports them. Most of any ecosystem lives here and costs nothing. Capabilities get promoted
to Tier 1 only when portability is actually wanted.

### The known risk

Haxe — the closest existing system to this design — suffers real per-target standard library
divergence. Taken far enough, that leaves N dialects under a thin shared syntax.

Mitigation: a library can be **declared portable**, and the compiler then rejects any
non-Tier-1 capability inside it. Discipline is available where it matters (shared libraries)
and absent where it does not (application code exploiting a specific platform). The tradeoff
becomes explicit and checkable rather than global.

### Specification tightness

Two implementations of a Tier 1 capability must be interchangeable, but the specification
must stay loose enough that a native implementation qualifies. Concretely: do not specify map
iteration order, because Go deliberately randomizes it. Each Tier 1 capability carries a
conformance test suite; a target's implementation is valid when it passes.

---

## 4. Type system

**Mathematical semantics, machine representation, range declared in the type.**

Neither of the two obvious options works:

- **Unbounded integers as the default** repeat Shen exactly. They cannot be unboxed in
  general — every operation becomes a branch plus a possible heap allocation, or it depends
  on whole-program range analysis, which is fragile and makes performance *unpredictable*.
  Unpredictable performance is a compromise on performance.
- **Machine types as the default** break the portability story: the JVM has no unsigned
  types, and JS has no integers at all.

So the type carries a range — `(int 0 255)`, `(int -2^31 2^31-1)` — and the compiler picks
the representation that fits. Semantics are exact: no wrapping and no undefined behavior.
Overflow is a compile-time error where provable and a trap otherwise. Wrapping operations
exist under separate names for when they are wanted.

Precedent: Ada/SPARK subrange types, with decades of safety-critical production use, and
Zig's arbitrary-bit-width integers (`u7`, `i33`) with explicit wrapping operators (`+%`).

Consequences:

- **Zero cost.** Representation is a machine word chosen from the declared range.
- **Portable by construction.** The range is the contract. `(int 0 255)` becomes `uint8_t` in
  C and a plain `int` on the JVM; the program means the same thing in both.
- **Predictable.** The programmer declared the range; there is no mystery about what is
  emitted.
- **Validated by the target list.** `(int 0 2^31)` maps exactly onto a JS double, since
  doubles represent integers exactly up to 2^53. An untyped `int64` on JS would be a
  catastrophe.

Ergonomics: `i32`, `u8`, `nat` and friends are sugar for common ranges. Ranges are inferred
for locals from initializers and uses; explicit types are required at function boundaries.

Floats: IEEE-754 binary32 and binary64, because that is what hardware implements everywhere.
Strict IEEE semantics by default, with fast-math as an opt-in capability — float determinism
across hosts is a real hazard (x87 excess precision, FMA contraction, JS number semantics).

Exact decimals for money are a separate library type. Unbounded integers remain available as
a Tier 1 capability in a package — not as the default.

---

## 5. Targets

**C is not required.** The first draft recommended a C backend first, on the reasoning that
one backend reaching every machine is the biggest win. Under the Parasite model that
reasoning fails: Go's standard library is not reachable through C. C earns its place later,
for iOS and embedded, where it is genuinely unavoidable.

The reasoning that *does* survive is the need for mutually hostile hosts early, to keep the
core honest. The chosen targets already supply that:

| Target | Hostility | Ecosystem won |
|---|---|---|
| **Go** | GC, no pointer arithmetic, restricted `goto` | Backend/server software |
| **JavaScript** | No integers, no structs, no int64, GC | Browser front end |
| **Java** (→ dex) | No unsigned, no value types, no `goto`, no tail calls | Android apps |

JS is the most hostile host available and therefore the best forcing function. A core that
survives JS survives anything.

**Android: Java source.** Full ecosystem access, and Java is a smaller and far more stable
emission target than Kotlin. Standard toolchain: `javac → d8 → apk`. Emitting dex directly
would skip R8's optimization and chase a moving, under-documented format. Kotlin can be added
later as a *separate* target if Compose or coroutines demand it — under the Parasite model,
having both Java and Kotlin targets is normal rather than contradictory.

Later, by demand: C (iOS, embedded, desktop native), WASM, Win32, .NET, Swift.

---

## 6. Implementation

**The compiler is written in Go.**

For: single-binary distribution, which matters more than it sounds for a tool people are
meant to download and run; trivial cross-compilation, so the compiler runs everywhere for
free; fast builds; a standard library that covers what a compiler needs; and heavy
representation in LLM training data, which requirement 8 makes relevant.

Against: no sum types. An IR is a sum type, and Go makes you emulate one.

Mitigation: represent the IR as a **flat tagged struct with a kind enum and index-based
children**, not an interface hierarchy. Simpler in Go, better cache behavior, and trivially
serializable — which leads directly to the next decision.

Rejected: Rust (slow builds and high friction on a project that gets dropped and resumed);
OCaml/Haskell (better for compilers, worse for distribution and LLM support); self-hosting in
v1 (bootstrapping early is a reliable way to stall).

### The backend interface is a file format, not a Go interface

If the IR serializes, a third-party backend need not be written in Go at all. Someone can
write a backend in Python or TypeScript that reads IR and emits code.

This single decision does more for requirement 3 than anything else in this document. It also
makes the IR dumpable for inspection, diffing, and LLM tooling.

---

## 7. Rejected options

### Forth-like surface language
The performance reputation belongs to native-compiling Forths; threaded-code Forths are
interpreters and slow. More decisively, stack juggling (`DUP SWAP ROT`) has no named
referents — every reader, human or model, must simulate the stack to know what any word
refers to. That directly contradicts requirement 8. A stack machine is acceptable as a
*bytecode*; it should never be the notation anyone reads or writes.

### Lambda calculus as the core
Shen's wall restated. First-class closures require captured environments, which require heap
allocation. Closures belong above the core, lowered by defunctionalization or explicit
environment structs — never a core primitive.

### TLA+ / state machines as the core
TLA+'s model is nondeterministic action selection over global state, built for model checking,
not for lowering to fast code. Two things are worth taking: **refinement** (each layer
provably preserving the semantics below it) supplies the vocabulary for the lowering
discipline, and **state machines make an excellent DSL** for protocols, drivers, and UI.

### A flat instruction stream (`addi8`, `addi32`, ...)
Typed scalar operations: yes. Flat and `goto`-based: no. Java has no `goto`, Go restricts it,
Swift and Kotlin have none. Recovering structure from unstructured control flow is a hard
algorithm — WASM needed relooper/stackifier precisely for this. Structured control flow
(`if`, `loop`, `break n`, `return`) makes every source-language backend mechanical.

### "As fast as assembly" as a literal global claim
It cannot hold simultaneously with "compiles to JVM bytecode." Replace with **parity with
hand-written code in the target language**, enforced as a CI gate: a benchmark suite
comparing generated output against hand-written Go/JS/Java with a fixed regression threshold.
An acceptance test, not an aspiration.

Under the "emit at the highest layer natively provided" rule, this becomes far more
achievable — the output *is* what hand-written code would contain.

---

## 8. Decisions still open

1. **Memory model.** No GC in core; manual plus arena/region allocators passed explicitly. On
   GC'd hosts (Go, JS, JVM) `free` lowers to a no-op, preserving target parity. Needs
   confirming against the capability model — allocation may itself be a capability.
2. **Error model.** No exceptions; result values and error enums. But Go, JS, and Java all
   have native error idioms, and Tier 2 bindings will surface them. Interop story needed.
3. **Concurrency.** Deferred. Note that Go's goroutines are a major reason to target Go at
   all, so "deferred" cannot mean "forever."
4. **Strings.** Bytes in core, Unicode in a library. But every one of the three initial
   targets has a native string type, and the Parasite rule says use it. Likely a Tier 1
   capability rather than a core type.
5. **Module and package format.** Required before Tier 2 bindings can be written.
6. **Naming translation.** Whether `fmt.Println` is called as-is or mapped to a house
   convention, and whether that mapping is per-binding or per-target.

Items 2 and 4 are where the Parasite model exerts the most pressure on the "small core"
requirement, and are worth deciding early.

---

## 9. Prior art worth reading

- **Haxe** — the closest existing system to Parasite: one language, many source-language
  targets, per-target ecosystem access. Read it for both the design and the divergence
  failure mode.
- **WebAssembly** — structured control flow over a typed core; the scalar subset is ~40
  instructions.
- **MLIR** — dialects with progressive lowering. The strongest validation of layered
  lowering, though far too large to copy. Note that MLIR's dialects form a graph, not a
  tower, for the same reasons found here.
- **Ada/SPARK** — subrange types in production, safety-critical, for decades.
- **Zig** — arbitrary-bit-width integers, explicit wrapping operators, `comptime` as the
  model for zero-cost abstraction.
- **C-- / Cmm (GHC)** — portable assembly as a compiler interchange layer.
- **QBE** — a small SSA backend (~10k LOC) reaching a large fraction of LLVM's performance.
- **Nim, Vala, Chicken Scheme** — transpile-to-source-language as a viable strategy.
- **Shen / KLambda** — the porting model to imitate, at a much lower level.

---

## 10. First milestones

1. Freeze the core on paper: primitives, type set, control-flow forms, and the capability
   declaration format. Write the porting contract before writing the compiler.
2. Specify the **IR file format**. This is the backend interface and the most consequential
   artifact in the project.
3. Reader, printer, and canonical formatter for s-expressions.
4. Front end: functions, locals, structs, range-typed scalars, structured control flow.
5. **Go backend.** One non-trivial program compiling and running.
6. **JS backend, before adding any front-end features.** Every core flaw surfaces here, and
   fixing them later is expensive.
7. Tier 2 binding format plus per-target import recipes. Bind something real on both targets
   — `fmt` on Go, `console`/DOM on JS — to validate requirement 4.
8. Benchmark harness against hand-written Go and JS, wired into CI as a gate.
9. Java/Android target.
10. Only then: macros and compile-time evaluation.

Steps 6 and 8 are the ones most likely to be skipped, and the two that determine whether the
architecture holds.
