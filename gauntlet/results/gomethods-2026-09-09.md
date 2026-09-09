# A generated Go method runs, and the survey was flattering itself

2026-09-09. `gauntlet/stdlib/survey.go`, `emit/target.go`,
`gauntlet/stdlib/acceptance/os-methods.oro`. On hamza's *"I wanted to support the
api of each target"* — the first item after the portable module went back to the
library layer where it belongs.

> **A PROGRAM REACHES AN ECOSYSTEM'S OBJECTS NOW.** `os.Open` → `*os.File` →
> `(*File).Read` into a byte buffer → `(*File).Close`, every declaration
> GENERATED, built and run, and the answer is **64** — `head -c 64 go.mod`. That
> is the first Go METHOD this language has ever called, and methods are **3,098
> of the 4,932 callable names**.
>
> **THE GENERATOR WROTE 1,007 PRIMITIVES WHERE THE SURVEY COUNTED 4,331.** It had
> never learned about methods, several results, or a void with an argument — all
> three of which the format can say and `judge` already counted. **1,007 →
> 4,311**, and the residue is exactly the **20** voids with no argument, which
> have nothing to be. *The generated count is the one that has to be true*, and
> it now equals the declarable count.
>
> **AND EVERY GENERATED LINE CLAIMED `pure`, INCLUDING `os.Chdir`** — an
> operation whose whole purpose is to change global state, which a pure
> declaration lets the reducer substitute into two places or drop (ADR 0010). A
> generator cannot justify that claim for a thousand functions, so it no longer
> makes it.
>
> **THE SURVEY'S OWN NUMBER WAS INFLATED, IN THE FLATTERING DIRECTION.** The
> obtainable-type fixed point was keyed by the bare type name, so `*File` in `os`
> and `*File` in `archive/zip` were ONE ENTRY. **Usable falls 31.7% → 27.5%**,
> methods **28.5% → 22.3%**, and *cannot build the argument* rises 42.7% → 49.9%.
> Three days old and mine.
>
> **One compiler change**: a buffer nobody writes to takes its element type from
> the host call it is passed to.
>
> **Cost: 178 of 178 emitted files byte-identical.**

---

## 1. The correction to my own number, first

gostdlib-2026-09-06 computed *obtainable host types* as a least fixed point — a
type is obtainable when some declarable function returns it and every argument it
needs is a scalar or already obtainable — and that is what moved methods from 0%
to 13.7%, then to 28.5% when several results landed.

**It was keyed by the type string as the manifest writes it.** The Go api
manifest writes a package-local type bare (`*File` inside `os`) and a foreign one
qualified (`io.Reader`), so `*File` in `os`, `*File` in `archive/zip` and `*File`
in `net/textproto` were **one map entry**. Obtaining an `*os.File` made every
`*zip.File` method look reachable.

| | before | after |
|---|---:|---:|
| callable surface | 4,932 | 4,932 |
| declarable | 87.8% | 87.8% |
| **usable** | **31.7%** | **27.5%** |
| func usable | 37.0% | 36.4% |
| **method usable** | **28.5%** | **22.3%** |
| obtainable host types | 202 | **289** |
| cannot build the argument | 42.7% | **49.9%** |

The obtainable count goes **up** while usability goes **down**, which is the
correction being real rather than a threshold moved: collapsed names became
distinct entries, and each entry now unlocks only its own package's methods.

**A residue is named rather than hidden.** The key is the package's BASE name,
because that is what the manifest itself writes — so two packages with the same
base name and the same type name still collide (`math/rand` and `crypto/rand`).
Fixing that needs the manifest's import paths, which it does not carry at the use
site.

> Fourth time a survey's first number was the tool talking about itself, after
> methods-at-0%, the Win32 typedef residue, and Win32's void-returning functions.
> **The first three deflated and this one inflated**, which is the direction that
> matters: a measurement of one's own language must never round in its own
> favour.

## 2. The generator wrote a third of what the survey counted

`judge` had learned three things the format gained; `emit` had learned none.

| shape | counted declarable | written | why |
|---|---|---|---|
| several results | yes, since 2026-09-06 | **no** | `if len(s.results) == 1` else skip |
| a METHOD | yes | **no** | `if s.recv != "" { continue }` |
| a void with an argument | yes | **no** | `if res == "none" { continue }` |

`os` alone: **130 declarable, 36 written.** *A name counted declarable and never
emitted is a claim* — win32-2026-09-08's sentence, on the other survey, arriving
here unprompted.

**1,007 → 4,311 primitives across 160 packages**, and the arithmetic closes
exactly: 4,311 written + **20 voids with no argument** = 4,331 declarable. A
statement's value IS its first argument, so a void with no argument has nothing
to be; that is the honest refusal and it is twenty names.

### 2.1 Three shapes, and none of them is new

**A method's receiver is argument 0** — the convention Win32's `HANDLE` already
used, arriving on a host that has objects — and the module is `go/PKG/TYPE`, so
`f.Read(b)` is `(File.Read f b)` and reads like the host.

**A constructor returning `(T, error)`** is what puts anything in the obtainable
set at all. `os.Open` was declarable-and-unwritten for three days.

**A void with an argument is `stmt`.** Same sentence as Win32's, same fix.

### 2.2 And it no longer claims `pure`

Every one of the 1,007 lines ended `pure`. `os.Chdir` was one of them.

Purity is one declared bit whose default is **impure**, chosen precisely so that
an author's omission costs speed rather than correctness (effects.md). A
generator has no way to justify the claim, so it does not make it — and a target
author adding `pure` by hand is how it comes back for the names that deserve it.

### 2.3 A Go integer type is a range, and this is where that bites

`spell` collapsed Go's twelve integer types into `int`. gostdlib-2026-09-06
recorded that as a **generator** limit rather than a format one — *"the format
can already say the right thing"* — and here is where it stops being theoretical:
`[]uint8` as `(array int)` is `[]int`, and `(*os.File).Read` takes `[]byte`. **No
generated line touching a byte slice could ever have compiled.**

`uint8` and `byte` are `(int 0 255)` now, `int16` is `(int -32768 32767)`, and so
on — ADR 0003's ladder, which gives `[]byte` on Go and `short[]` on the JVM
without either being written down (elemwidth-2026-08-27).

**`int64` and `uint64` stay `int`, and that is ADR 0012 rather than laziness.** A
declared range past the portable window promotes the value to arbitrary
precision, which would silently stop it being the host's word; leaving it `int`
makes the operation unprovable at the call site instead, which is a compile error
in the honest place.

### 2.4 And the types had to be declared at all

A generated file said `(prim Open ((a0 string)) (ptr-os-File error) …)` and
nothing anywhere said what `ptr-os-File` spells. The generator now collects every
opaque type it mentions and declares it at target level —
`(type ptr-os-File "*os.File")` — **as spelled, pointer and all**, because
`os.Open`'s result and `(*File).Read`'s receiver are the same type and recording
the pointee would name one no prim mentions.

One hole named rather than fixed: a `go/os` prim returning `fs.FileInfo` carries
`(import "os")`, which does not bring `io/fs`. It bites only when such a
declaration is used, and then the Go compiler says so.

## 3. The one compiler change, and the program that found it

```lisp
(build 64 (fn (b) ((File.Read f b) (fn (n e) …))))
```

`build` zero-fills and **nothing stores into this buffer** — the host does — so
the syntactic element inference correctly had nothing to say and the buffer came
out `[]int`. `(*os.File).Read` takes `[]byte`.

> **A buffer nobody writes to takes its element type from the host call it is
> passed to, because the host is the thing that writes it.**

`declaredElem` in `emit/target.go`, consulted **only** where the stores decide
nothing — and that is soundness rather than an order of preference. A declaration
is the most exact source there is, but a buffer the program *also* writes to must
satisfy its own stores, and narrowing it to what the host expects would truncate
them silently. Where the two disagree the host compiler says so, which is the
loud direction. The test carries that half as its control: a buffer storing
100,000 stays wide.

## 4. Acceptance, because a percentage that does not build is a claim

`gauntlet/stdlib/acceptance/os-methods.oro`, every declaration generated:

```lisp
((gos.Open "go.mod") (fn (f e1)
  (if (gos.err-nil e1)
      (build 64 (fn (b)
        ((File.Read f b) (fn (n e2)
          (seq (File.Close f)
            (if (gos.err-nil e2) (io.print-int n) (io.print-int 0)))))))
      (io.print-int 0))))
```

emits

```go
f, e1 := os.Open("go.mod")
b := make([]byte, 64)
n, e2 := f.Read(b)
_ = (f.Close())
```

and prints **64**, which is `head -c 64 go.mod | wc -c`. That is what a person
writes, produced from declarations nobody hand-wrote.

`err-nil` comes from `go/os` in `targets/`, not from the generated file: Go's
`os` package has no such function, and **testing `err != nil` is a fact about the
target rather than about a package** — the same reason `(type error "Exception")`
is a target-level declaration on Java.

## 5. Cost

| | |
|---|---|
| emitted files | **178 of 178 byte-identical** |
| differential suite, unit tests, `go vet` | green |
| generated primitives | **1,007 → 4,311**, and 4,311 + 20 = 4,331 declarable |
| compiler code | one function, `declaredElem` |
| new term kinds, reduction rules, backends | **none** |

Two tests, both verified to fail against their bug: the buffer rule (remove the
call — `int j = …`, `[]int` where `[]byte` was needed) with a control that must
NOT narrow, and the acceptance program, which does not build if any of the four
generator defects returns.

## 6. What this leaves

**`cannot build the argument` is 49.9% and it is now the whole question.** With
methods reachable, the residue is types no declarable function returns — mostly
interfaces (`io.Reader`, `fs.FS`) and structs built by literal. `io` is 28
declarable and 7 usable for exactly that reason: everything takes an interface.

**Interfaces are the next real refusal, and they are a LANGUAGE question**, not a
format one: `io.Reader` is a value with a method set, and callbacks.md tier 3
refuses the manufactured version. A `*os.File` satisfies it and we can already
hold one — so *passing an obtained concrete value where an interface is wanted*
is a much smaller question than implementing one, and it is the next thing to
measure.

**No survey exists for JavaScript or the JVM.** Two of four targets have never
been priced, and this week wrote file I/O on both by hand. The JDK is reflectable,
which makes it the cheapest of the four to write.

**The generated files are not committed**, and that is deliberate: they go in a
project-local layer beside the program, which is what layers-2026-09-07 built and
what the Win32 acceptance already does. `targets/go/os.oro` stays the curated
binding.
