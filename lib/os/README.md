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

## `os` is one definition over `os/host`

Since oskinds-2026-10-06, `os` is a library module, `lib/os.oro`, and the cells in this directory
are **`os/host`**, each host's declarations alone. A portable call is the host's call composed with
the classification of its failure:

```
os.ReadFile = (id + h) ∘ os/host.ReadFile          h : E_host → K
```

`id + h` is `result.err-map h`, lib/result's functoriality in its error. K is the kinds a program can
act on, `os.not-found | os.exists | os.permission | (os.other e)`, the three portable sentinels of
Go's io/fs with the host's error as the residue (spec/errors.md §6). h is written **once**, in
lib/os.oro, as a case analysis over three predicates each cell declares: Go's `os.IsNotExist`,
Node's error codes, Java's exception classes. So the only things that differ by host are the raw call
and the predicates, and the structural check compares exactly those, `os/host`'s interface.

A fallible call's result is a value of lib/result's sum, read off the host through its `error` type's
niche (`nil` on Go, `null` from the try/catch templates on JavaScript and Java). `err-nil` is gone: a
failed call's value is bound only in its success arm.

## Σ, and the law each name states

| name | what it is |
|---|---|
| `ReadFile`, `WriteFile` | get and put on the store F : Path ⇀ B\*, with put-get: after a put of d succeeds, a get of the same path gives d. No mode: the hosts share only "create with 0666 before the umask". `ReadFile` gives `(result (array (int 0 255)) os-error)`, `WriteFile` `(result (tuple) os-error)` |
| `not-found`, `exists`, `permission`, `other` | the kinds of a failure, `os-error`, and the same failure lands in the same kind on every host: `roundtrip.oro` reads a missing path and prints `not-found` on Go, JavaScript and Java |
| `text-of` | d, the one total decode B\* → Σ\* (ADR 0030), the same function on every host |
| `Args` | the command line, [program, arguments…], the same shape on every host |
| `Getenv` | a read of the environment, absent ↦ `""`. Impure, because `go/os.Setenv` writes what it reads |
| `io.print-line`, `io.print`, `io.print-int` | UTF-8 bytes to standard output, LF on every host |

## windows

**Built** (ADR 0112, oswin-2026-11-15): `lib/os/windows.oro` and `lib/io/windows.oro`
are the first cells written in J, each name a short program over kernel32, as
this section once predicted. Three things it took:
- a Win32 failure is a term: CreateFileA's sentinel tested, then GetLastError,
  each name's sum built once at its tail, as a retraction builds it;
- an out-parameter is a cell (ADR 0048), an `alloca` on LLVM;
- `text_of` is ι_V ∘ r: the maximal-subpart repair written in J, then the
  inclusion of the valid UTF-8, `utf8_text`, behind its guard `utf8_valid`, two
  words of Σ_L (strings.md §11).

`read-loop` and `text-of` run there. What waits: `roundtrip.oro` inlines all nine J
definitions into one `main` past the reducer's depth, so its windows run waits on
ADR 0102's P8, a headed definition kept as a function.
