# 0040 — A failure is a value of a marked sum, built from the host's niche

Date: 2026-10-06
Status: Accepted (hamza). Specified in [spec/errors.md](../spec/errors.md); built in the order of its
§12. Settles design-direction §8's item 2, "settled in part" since August.

## Context

The language has handled host failures since it could open a file, with no specification: Go's
`(T, error)`, spelled `(let (tuple v err) …)` and tested with `err-nil`, on every host. JavaScript and
Java reach the same shape through `try`/`catch` templates, and Windows declares no failure at all.
errors-research.md (2026-10-04) measured what that costs:
- **the product leaks.** A program reading a failed call's value compiles on Go, JavaScript and Java
  and does three different things (0, `TypeError`, `NullPointerException`). ADR 0026's promise, the
  same answer wherever accepted, is broken by an encoding;
- **a `pure` partial declaration** (Java's `parseLong`) makes a failure depend on what reduction keeps;
- **nothing distinguishes a forgotten check from a deliberate discard**, and no program reads an error's
  content, because there is none to read portably.

It also measured that **a sum costs nothing**: propagated through three levels, it reduces to one branch
per error path. gocoverage-2026-10-06 found 142 declarations that depend on the model, 73 to write and
69 written. So the model had to be decided before the bulk of `os` and `bufio` was declared.

## Decision

1. **Three kinds of error, three algebras.** A domain condition is an obligation, proven or refused
   (unchanged). Abandonment is crash : E → 0, only where the source says `expect` or the build says
   `-checked`. The environment's failure is a value, a summand of a coproduct.
2. **The compiler knows a theory, not a type.** A variant may mark a constructor `(success c)` and
   itself `(relevant)`. A marked variant is A + R, the exception monad with E = R. `try` (its bind) and
   `expect` name the success constructor as a pattern, `(try (ok x) e body)`, because the language types
   a sum by the constructors a program writes; the pattern is checked to be the marked success, and both
   expand to an ordinary exhaustive `case`. Relevance is checked for any relevant type. Every model obeys
   the monad laws, because they depend only on that shape.
3. **`(result T E)` is a library**, `lib/result`, with `err-map` as its functoriality in E. `option`
   marks `some`, so `try` also works on a map read, and a program's own flat sum is a model too.
4. **The host's encoding is a niche of its error type**, declared once per target:
   `(repr error (niche (host expr "%s == nil")))` on Go, the `null` tests on JavaScript and Java. A
   declaration whose result is a model means the host's T × (1 + E) composed with the retraction
   r(t, none) = success t, r(t, some e) = the other constructor e. The loader realizes it by δ: the raw
   call under an unwritable name, and a definition applying r in the Church encoding. A host whose
   sentinel varies by function (Win32) says it per declaration, `(fails (sentinel v) (error f))`. A
   partial success is a product with an `(option E)`, read through the same niche.
5. **An error must be used.** A relevant value bound and never read is refused; `(ignore e)` writes the
   discard down. A host whose own algebra makes an error redundant (a `bufio.Writer`'s sticky writes)
   is declared so: the write not fallible, `Flush` fallible.
6. **`expect` is the one way to give up**: total, crashing with its reason on any non-success
   constructor, through `abandon`, which every target realizes as its own crash.
7. **One error type per `try` chain**, moved by `err-map`.
8. **The portable `os` returns kinds**, `not-found | exists | permission | (other e)`: the three portable
   sentinels Go's `io/fs` declares, each mapped onto Node's codes and Java's exception classes, with the
   host's error as the residue.

## Why not

- **Keep the product (E1).** Measured: it leaks across hosts, and it cannot carry portable content.
- **The product checked by flow (E2):** reading `v` only under a test of `err`. That encodes the sum in
  facts keyed by name, the hazard ADR 0036 closed once, and every partial-success call needs its own
  invariant.
- **Exceptions (E5).** They need a multi-level exit from every call, which the IR does not have, and
  unwinding, which x86 would need a runtime for. They make error paths implicit, and the linearity
  checker needs every path explicit (Weimer and Necula, 2004).
- **Crash on any failure (E8).** Right for bugs, wrong for a missing file. It survives as `expect`.
- **`result` in `lang`.** The compiler would depend on one type. With roles, it depends on two markers
  of the declaration format, and `try` gains `option` and every flat sum.
- **A `(fails nil)` clause on every declaration.** The encoding is a fact about Go's `error` type, true
  of all 69 fallible Go declarations alike. Stating it once, as a niche, says it where it is true, and
  finishes the niche encoding sums.md §9 left open. The clause survives for Win32, where it varies.
- **The template builds the sum.** A sum's representation is the compiler's per target, and a template
  would hard-code it.
- **The compiler recognises Go's `(T, error)` itself.** That puts Go's convention in compiler code,
  against "primitives are declared in targets, not in Go".
- **`ok!`, a projection discharged as an obligation.** A host call is never evaluated at compile time,
  so its tag is known only where a `case` already reduces it away. It would be a construct the compiler
  nearly always refuses.
- **The union of error sets, inferred (Zig).** It needs structural sums, and the language's are nominal
  and closed. `err-map` until programs need more.

## Consequences

- **A fallible host call's value cannot be read on its failure path**: it is bound only in the success
  arm. The measured leak becomes unrepresentable.
- **Errors carry portable content**, the kinds, checked across hosts by a program.
- **Nothing new reaches the reducer, the IR or the printers.** The markers are declaration syntax, the
  expansions are `case`, and the boundary is δ. The emitted code is the host's own
  `if err != nil` (sumofsums-2026-10-06).
- **The migration touches every fallible declaration and call site**: 69 Go declarations, the portable
  `os` and `io`, and 16 call sites in the corpus. Where a program tested only `err-nil`, the gate expects
  no change in what is emitted.
- **Concurrency inherits `abandon`**: under a supervisor, an `expect` ends one process and the
  supervisor decides.
- **Relevance is general**: a future handle that must be closed can declare it.
- **Named and not checked**: a relevant value made by a pure definition and dropped, because β drops an
  unused pure argument before types are known.
