# Research: how to reach Android — Java source, bytecode, DEX, or Kotlin

2026-10-05. **Research, not a decision.** hamza named Android a primary target
([ADR 0039](decisions/0039-the-targets-are-go-windows-android-and-the-browser.md)) and asked: should
the language still be compiled to Java, or is it better to compile to bytecode? ADR 0004 answered in
three unmeasured paragraphs on 2026-08-13 ("Java source"). This answers with measurements on ART
([android-2026-10-05](../gauntlet/results/android-2026-10-05.md)), and names what would decide the
rest.

The short version:
- **every link of the chain after our printer is a compiler we do not control**, and only one link,
  `javac`, adds nothing an Android build needs;
- **what Java source costs is the Java language's restrictions**, not performance. Measured: 50
  lambdas applied at once, each a synthetic class on Android;
- **class files without stack maps are what Android accepts**, and they are a small format: version
  50, no `StackMapTable`, `d8` takes them and ART runs them (measured);
- **the real fork is the API, not the bytecode.** Modern Android UI, Jetpack Compose, is reachable
  only through Kotlin's compiler. That question is decided by the first Android program, not by
  benchmarks.

---

## 0. The chain, and what each link does

```
IR_P ──printer──▶ Java ──javac──▶ class files ──D8/R8──▶ DEX ──dex2oat / JIT──▶ native code (ART)
                   │                  │                     │
                   └─ Java's rules    └─ the JVM's rules     └─ Dalvik's rules
```

| link | what it does | what it costs us |
|---|---|---|
| `javac` | parses, checks the **Java language's** rules (definite assignment, checked exceptions, effectively final captures, no statement in expression position), desugars (lambdas to `invokedynamic`, records, string concatenation), folds constants. **No optimizer**: the JLS leaves optimization to the VM, and javac emits close to one instruction sequence per construct | a JVM start and ~1 s per build (1.2 s for 25 files); the language's restrictions, which the printer and the templates must work around (§2) |
| D8 | turns class files into DEX: stack code to registers, through its own SSA IR with linear-scan allocation; desugars `invokedynamic` into synthetic classes, and Java 8–17 language and library features for older API levels | 1.4 s for 26 classes |
| R8 | D8 plus whole-program shrinking, inlining and optimization; the release build's dexer | not measured on our output (M5) |
| ART | verifies DEX by type inference; interprets, JIT-compiles with profiles, and AOT-compiles with `dex2oat` | the runtime of record now (ADR 0039) |

**Compilation is a composite of maps, and correctness is that the diagram commutes.** Each link
should preserve meaning: ⟦javac(J)⟧_JVM = ⟦J⟧_Java, ⟦D8(C)⟧_Dalvik = ⟦C⟧_JVM, and so on, so the
composite preserves the IR's meaning (spec/ir.md §9.1). Emitting at a later point removes links from
the composite: fewer compilers whose correctness we assume, but a lower-level format whose rules we
must meet ourselves. **The JVM accepts strictly more programs than are images of Java programs**: a
`throw` anywhere, any jump the verifier accepts, no checked exceptions, no definite-assignment rule.
So the Java source restricts what the printer may say *without* adding anything the chain needs.

**"Emit at the highest layer the target natively provides"** is about *API* layers: use the host's
map, its strings and its APIs, not reimplementations. On ART the native input format is DEX, and
class files are D8's input. Java source is one more layer *above* the native one, so the rule does not
pick it. ADR 0016 already says a target need not be an expression language.

---

## 1. What every JVM language does

Almost every language on the JVM emits **bytecode**, not Java source: Scala, Kotlin, Clojure, Groovy,
JRuby, Jython, Ceylon. Each uses a class-file library (ASM: Bruneton, Lenglet and Coupaye 2002) or its
own writer. Source-to-Java compilers are the exception (Xtend, some DSLs), chosen for readability of
the output. **Kotlin on Android emits class files, and D8 dexes them like javac's.** Emitting bytecode
is the normal path for a language that is not Java, not a heroic one.

---

## 2. What the Java source costs, measured

The emitted corpus (63 Java files) contains **50 lambdas applied at once**:
- 29 `BiFunction`, mostly `big-fit`'s overflow check, which must `throw`, and Java has no throw
  expression;
- 18 `Consumer`, mostly `io.print`'s write, a statement where an expression is wanted;
- 3 `Function`.

On Android **each becomes a synthetic class**: `chain.oro`'s `Main` dexes to 4 classes, and the
gauntlet's 26 class files to 35. The other detours are quieter:
- a `try`/`catch` template assigning `%r0` and `%r1`, because a statement cannot yield a pair;
- `final var` discards;
- records for several results;
- a `(int)` or `(long)` cast at every boundary where Java's typing differs from the IR's.

**Performance is not what it costs.** On ART the generated code is at parity with hand-written Java
on 9 of 10 pairs, and twice as fast on the two JSON programs, because of the element widths. It is
1.17–1.19× on the stencil, an ART-specific residue still to be explained (M6). javac adds no
optimization, so emitting class files directly would not lose any. R8 and ART do the optimizing,
whichever front end produced the class file.

---

## 3. The formats, as algebra

**JVM bytecode** is a stack machine with a typed verifier.
- Since class version 50, verification is *type checking* against a `StackMapTable`, the frames at
  every branch target, which the compiler supplies (JVM Spec §4.10.1).
- Below 50 it is *type inference*, a dataflow fixpoint over a lattice of verification types
  (§4.10.2; Leroy 2003 formalizes both).

The split verifier came from Rose's lightweight bytecode verification (2003) for small devices. The
map is a **certificate** that makes checking linear, the proof-carrying-code idea (Necula 1997) that
ADR 0038 discussed for the IR's text. Computing the certificate needs the least upper bound of two
class types, so the emitter must load the class hierarchy. That is the hard part of writing class
files (ASM's `COMPUTE_FRAMES`).

**DEX** is a register machine. Its verifier infers types and needs no map. Register VMs execute
fewer, larger instructions than stack VMs for the same program (Shi, Casey, Ertl and Gregg 2008:
about 47% fewer executed, 25% larger code), which is why Dalvik chose registers (Bornstein 2008).
The format is public and versioned (the Dalvik Executable format, from version 035 onward, each version
documented), not the
"moving, under-documented format" ADR 0004 feared. Writing it is more work than a class file:
- sorted identifier tables (strings, types, prototypes, fields, methods);
- a map list, and a checksum and signature;
- 4-, 8- and 16-bit register encodings, so allocation affects size;
- try/catch tables.

**The measured shortcut** (android-2026-10-05 §3): **a version-50 class file with no
`StackMapTable` is accepted by D8 and run by ART**, and by HotSpot too, because below 50 the JVM
infers. So for Android a class-file printer needs:
- a constant pool;
- the instructions;
- `max_stack`, an abstract interpretation of our own code's stack depth, which is trivial for code
  we generate structurally;
- `max_locals`;
- **no frames**.

The proof is 103 lines (`experiments/android/cw.go`).

**The IR fits both.** Its SSA values with exact live sets already give x86 an optimal register
colouring (spec/ir.md §9.6). To DEX they would give registers directly. To a class file they give
locals, and the stack is used only within an expression. D8 then re-derives SSA and allocates DEX
registers itself.

---

## 4. The API question, which is the bigger one

The native layer of an Android *application* is its API. That API has two generations:
- **the Android SDK and the View system**, Java APIs, callable from Java source, class files or DEX
  alike;
- **Jetpack**, much of it Kotlin-first. **Jetpack Compose**, the UI toolkit Google recommends for new
  apps, is written in Kotlin and needs the **Compose compiler plugin**, which rewrites every
  `@Composable` function (it adds a `Composer` parameter, groups and change flags). Coroutine-based
  APIs (`suspend`, `Flow`) need a `Continuation` parameter: callable from bytecode, awkward from Java.

So "Java or bytecode?" is the smaller question. The larger one is **whether the language's Android UI
is the View system or Compose**:
- **Views:** any of Java source, class files or DEX reaches the whole SDK.
- **Compose:** only a Kotlin front end reaches it, either by emitting Kotlin source and letting
  `kotlinc` and the plugin run, or by reproducing the plugin's calling convention in our own
  bytecode. That convention is an internal ABI that changes with Kotlin releases, and reproducing it
  would chase it.

**A Compose UI is a Mealy machine too.** A composable is a function from state to a UI description,
re-run when state changes, and its events are messages that update state. That is the Elm
architecture (concurrency-research.md §2.1). The language would emit the state and the step; the
toolkit, Views or Compose, renders.

---

## 5. Candidates

| | form | gain | cost | what would kill it |
|---|---|---|---|---|
| **A1** | **Java source → javac → D8/R8** (as now) | works today, measured on ART; readable output | javac per build; the Java language's detours (50 lambda classes); a JVM start | nothing measured kills it. It is the baseline A2 must beat |
| **A2** | **class files, version 50, no frames → D8/R8** | no javac; `throw`, statements and sequences anywhere; no checked-exception or definite-assignment rules; line numbers pointing at `.oro` source; the same files run on HotSpot for tests | a class-file printer: constant pool, instructions, `max_stack`; a printer the differential suite must cover, since no `javac` checks its output | (1) D8 refusing or slowing frameless input at scale (measured on one method, not on the gauntlet); (2) a performance loss on ART against A1 (M2); (3) a future D8 dropping pre-50 input |
| **A3** | **DEX directly** | the native format; registers from the IR's colouring; no JVM tool for our own code | a larger writer (§3); no R8 on our code (whole-program inlining is ours already); an app still needs D8 for its libraries and aapt2 and apksigner, which are JVM tools, so the toolchain keeps a JVM anyway | the gain over A2 being nil once D8 is needed for the app's libraries regardless |
| **A4** | **Kotlin source → kotlinc** | reaches Compose and `suspend` APIs natively | the slowest compiler of the four; a much larger language to emit into, with its own rules (null safety, smart casts) to satisfy | a first Android program that does not need Compose |
| **A5** | **native code through the NDK** (an ARM64 or x86-64 printer, JNI to the SDK) | no VM for computation | every Android API call crosses JNI; the SDK is Java; a fifth backend | the first API call |
| **A6** | **A2 for the program, plus a thin Kotlin layer for Compose UI** | both: our code as class files, the toolkit through its own compiler | two front ends for one target; the boundary between them is a host interface | a program that needs no Compose (then A2 alone) |

### Where it points

- **For computation and the SDK: A2.** It removes the only link that adds nothing, and its hard
  part is measured away for Android: no frames, version 50. It keeps D8 and R8, which do the work an
  Android build needs.
- **A3** buys little over A2 while D8 is in every app build anyway.
- **For the UI: undecided.** Views need nothing beyond A2; Compose needs A4 or A6. The first Android
  program decides it, as `bufio` was decided by `lines.oro`.
- **A1 stays until A2 is measured against it**, as Go's IR printer was measured against the term
  backend before replacing it (irp2).

---

## 6. Measurements and programs that decide

| | what | decides | status |
|---|---|---|---|
| M1 | the Java backend's output through `d8` on ART, and the gauntlet on ART | whether A1 works on Android, and its parity | **done**: runs; parity on 9 of 10 pairs, stencil 1.17–1.19× |
| M1′ | a frameless version-50 class file through `d8` onto ART | A2's hard part | **done**: accepted and runs |
| M2 | **a class-file printer prototype for the IR** (as irp2 was for Go), on the gauntlet's kernels, against A1 on ART and HotSpot | A2's kill conditions 1 and 2; build time without javac | proposed |
| M3 | the differential suite with an `android` column: `build -target=java`, `d8`, `dalvikvm64` | whether ART computes what the other hosts compute, on every case | proposed: the runner needs the emulator |
| M4 | **the first Android program**, an `Activity` with a UI, in Views and in Compose by hand, against what the language would emit | A4 or A6 against A2; the entry point and the event loop (ADR 0039) | proposed |
| M5 | R8 instead of D8 on the generated code | whether release builds change the parity | proposed |
| M6 | the stencil on ART: `dexdump` and the JIT's compiled code, generated against hand-written | the one ART residue | proposed |

---

## 7. Questions for hamza

1. **A2's prototype (M2):** build a class-file printer beside the Java printer and measure, or keep
   Java source until an Android program asks for more?
2. **The UI:** the View system, Compose, or let the first program decide (M4)?
3. **A device:** is one available? The emulator's ratios point; a phone measures.

---

## References

- Bornstein, D. "Dalvik VM internals". Google I/O, 2008.
- Bruneton, E., Lenglet, R., Coupaye, T. "ASM: a code manipulation tool to implement adaptable
  systems". *Adaptable and Extensible Component Systems*, 2002.
- Leroy, X. "Java bytecode verification: algorithms and formalizations". *Journal of Automated
  Reasoning* 30(3–4), 2003.
- Lindholm, T., Yellin, F., Bracha, G., Buckley, A., Smith, D. *The Java Virtual Machine
  Specification*, Java SE 21 edition, ch. 4 (the class file format, verification §4.10).
- Necula, G. C. "Proof-carrying code". POPL 1997.
- Rose, E. "Lightweight bytecode verification". *Journal of Automated Reasoning* 31(3–4), 2003.
- Shi, Y., Casey, K., Ertl, M. A., Gregg, D. "Virtual machine showdown: stack versus registers". *ACM
  TACO* 4(4), 2008.
- The Android Open Source Project. "Dalvik executable format"; "Verifying app behavior on the Android
  runtime"; "D8 and R8". source.android.com and developer.android.com.
- The Kotlin project. The Compose compiler plugin, part of the Kotlin compiler since Kotlin 2.0.
