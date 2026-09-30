# `os` and `io` — the library layer

`go/fmt` names Go's `fmt` package. `js/Math` names V8's `Math`. `java/System`
names that class. **A prefixed module names a HOST NAMESPACE and claims nothing
beyond that one host** — no portability, no shared meaning, no obligation on any
other target. That is [ADR 0001](../../docs/decisions/0001-parasite-model.md),
and those files live in `targets/`.

`os` and `io` are the other thing: **one interface with an implementation per
host**. [target-system.md](../../docs/spec/target-system.md) writes a
declaration as `Decl ≅ Σ × I` and a target FAMILY as a fibration over a shared
`Σ`; this is that shape between targets that are *not* a family. A program whose
free names lie inside it is **portable — computed by the capability graph** as
ADR 0001 says, rather than promised by a layer.

## Why here and not in `targets/`

They were written in `targets/` first, and that was the wrong layer.

**The host's API is what this project claims it can parasitize. A portable name
over it is a claim about several hosts agreeing.** Writing the second before the
first inverts the order: you cannot know what the shared `Σ` should be until you
know what each host's actually is. `targets/go/os.oro` declaring six names and
calling itself `os` made that mistake look like a design.

`(provides T M decl…)` is exactly `(target T (module M decl…))` written where the
library lives — a target FRAGMENT, which §7.2 already says a target is the glue
of. It is the **lowest** layer, because a library's opinion about a host loses to
that host's own files. So this directory needs no new mechanism, and the target
files go back to naming host namespaces.

## It is not the retired portable layer

That layer had **bodies**: `num/vec.materialize` and `fold-range2` lowered into
shapes the layer chose, and the price was
[measured at 1.79x](../../gauntlet/results/stencil-2026-08-15.md). There is no
body here at all. Each cell is its own host's call at the highest layer that host
provides — `os.ReadFile` on Go, `fs.readFileSync` on Node, `Files.readAllBytes`
on the JVM — and nothing is shared but the name and the type.

## The claim is checked

A claim nothing checks is decoration: `split-words` passed every review for two
months while returning different answers on different targets.

- **Structurally** — `emit/portable_test.go` requires the three cells to declare
  the same names at the same argument and result types, **in both directions**. A
  target may not quietly *add* to a shared interface either, because a program
  written against the richer cell would look portable and not be.
- **Behaviourally** — `examples/io/roundtrip.oro` calls every name of Σ, and
  `cmd/build`'s `TestEveryPortableNameRunsOnEveryHost` builds it on Go,
  JavaScript and Java, runs it and requires one output, byte for byte. A name
  added to `os` or `io` and not called there fails the test. Before it
  ([libos-2026-09-30](../../gauntlet/results/libos-2026-09-30.md)), this was
  the three tools `examples/io/{wc,jsonfmt,freq}.oro`, measured byte-identical
  once, by hand
  ([portableio-2026-09-09](../../gauntlet/results/portableio-2026-09-09.md)).
  None of them wrote a file, and `WriteFile` compiled on Go only.

## Σ, and the law each name states

| name | what it is |
|---|---|
| `ReadFile`, `WriteFile` | get and put on the store F : Path ⇀ B\*, with put-get: after a put of d succeeds, a get of the same path gives d. No mode: the hosts share only "create with 0666 before the umask". |
| `err-nil` | the discriminator of the sum B + E each host encodes differently: a second result, or a throw that a template catches |
| `text-of` | d, the one total decode B\* → Σ\* (ADR 0030), the same function on every host |
| `Args` | the command line, [program, arguments…], the same shape on every host |
| `Getenv` | a read of the environment, absent ↦ `""`. Impure, because `go/os.Setenv` writes what it reads |
| `io.print-line`, `io.print`, `io.print-int` | UTF-8 bytes to standard output, LF on every host |

## What is missing

**windows.** `kernel32.ReadFile` is the handle-based API, so `os.ReadFile` there
is `CreateFileA` + `GetFileSizeEx` + a `build` + `ReadFile` + `CloseHandle` — a
LIBRARY written in the language, `lib/win/fmt.oro`'s shape, not a template. `io`
is the same story and `lib/win/fmt.oro` already has the arithmetic half. Neither
is a language question.
