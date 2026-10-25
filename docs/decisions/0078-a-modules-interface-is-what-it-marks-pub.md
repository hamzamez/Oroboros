# 0078 — A module's interface is what it marks `pub`; `export` is the boundary to the host

Date: 2026-10-25
Status: Accepted (hamza, 2026-10-25: "go ahead with item 2", on the question of 2026-10-24: "should go
back to private by default. and instead of export use pub?"; the 10-24 plan's item 2). Supersedes
[ADR 0064](0064-a-file-is-a-theory-and-a-declaration-is-a-header-and-one-body.md) item 4 and restates
item 5; amends [spec/j.md](../spec/j.md) §2.2 and [modules.md](../spec/modules.md).

## Context

ADR 0064 made visibility public by default, with `private` hiding a name, and `export` an entry point
only. Its "Why not" rejected default-private on one argument: the errors are asymmetric (a forgotten
`export` is caught at the first use elsewhere, a forgotten `private` is silent), but "a leaked name
carries no code and no binary interface", and libraries hid 2 of 23 definitions, so the marker belonged
on those.

**Measured, 2026-10-25:**
- **55 `private` marks in 6 files, 52 of them in three files that hide all or nearly all they define**:
  `emit/bignum.oro` 16 of 16, `emit/winmap.oro` 16 of 16, and `examples/tally/tally.oro` 20 of 21. tally
  is an entry file, which nothing imports, so its 20 marks hide nothing from anyone. The user libraries
  hide 2 of 25 (`os.classify`, `u128.digits18`).
- **Of the 23 public library definitions, 11 are named by another file and 12 by none.** Each of the 12
  is a promise nobody chose to make or to withhold.
- **The core has one relation for two things.** The reader makes one `(export …)` of a library's public
  names and of an entry file's `export`s, and the loader reads it twice: as what an importer may name
  (`checkExported`), and, for the module being compiled, as the entry points. So a library compiled
  alone makes its whole interface its entry points, and the sweep refuses `lib/result` on all four
  targets for it: `err_map` takes a function, which no host can pass.
- **A hole.** A module whose interface is empty is read as "no signature yet; everything is visible", so
  a library that hides every definition hides nothing.

**What the algebra says.** A module is a theory T = (Σ, Ax); an importer sees the reduct T|I to its
interface, a sub-signature I ⊆ Σ (ADR 0021, ADR 0064 item 4). The entry points are another subset, E ⊆ Σ:
the definitions compiled into host functions, where each claim crosses to the host and is assumed (ADR
0028), the dual of a host declaration, whose claim comes in as an axiom. **I and E are two relations**:
resolution reads I (who may name a definition), the build reads E (what is compiled, and whose claim the
host must meet). Their contents differ: `lib/result`'s I holds `err_map`, which cannot be in E until
callbacks are built, and an entry file's E is `{main}` while its I is read by nobody.

**Which side of I is written** is the remaining question, and it is Parnas's (1972): a module's interface
is the decision, its secrets the default. A signature ascription writes I (ML's `sig … end`, Ada's
package specification); so do Rust's `pub`, Go's capital letter and Java's package-private default. TLA+'s
`LOCAL` and Haskell's absent export list write the complement. The error asymmetry ADR 0064 named decides
it once the leak is priced: **a name in I is a promise, whether it carries code or not**. A client that
names it depends on it, and changing it breaks the client (Hyrum's law). The standing goal is the Go
library written in the language: libraries, each an interface.

## Decision

1. **A declaration is in its module's interface only when marked `pub`**: `pub fn`, `pub set`, a
   `pub` value. An unmarked declaration is private to its module, a file or a nested `module`. A `pub`
   variant puts its tags in the interface; an unmarked one keeps them private.
2. **`export` marks an entry point, and only that** (ADR 0064 item 5): when its module is the one
   compiled, the build compiles it into a host function and the host takes on its claim. A library may
   mark entry points, for when it is compiled alone; reached by `use`, they are not the program's.
   **`pub` and `export` are independent**: `pub export fn f` is both.
3. **The core has the two relations**: `(pub n…)` the interface, `(export n…)` the entry points, made
   by D from J's markers. A library module always has an interface, so an empty one hides everything:
   the hole is closed.
4. **`private` is refused**, naming the rule: a declaration is private unless it is `pub`.
5. **`pub` is a contextual word** at a declaration's start (j.md §1.4), as `export` and `private` were.

## Why not

- **Keep ADR 0064's default.** The leak it called harmless is a promise, and the measurement puts 52 of
  the 55 marks it asks for on files that hide nearly everything they define, 20 of them on an entry file,
  where they say nothing.
- **One word, `pub`, for both** (hamza's "instead of export use pub?"). I and E differ in content, so one
  word must make every interface member an entry point, which is the sweep's refusal of `lib/result`
  today, or every entry point a promise to importers. Rust keeps the two apart too: `pub` is visibility,
  and what crosses to a host is `#[no_mangle] extern "C"`. ADR 0064's third "Why not" (making `du`'s
  helpers entry points made two of them unbuildable) is the same fact.
- **An interface list, or an interface file** (ML's `.mli`): each name written twice, ADR 0064's first
  "Why not", which still holds.
- **A capital letter, Go's rule.** Case carries no meaning in J (ADR 0067).
- **`pub` implying an entry point in the entry file**, where nothing imports and the interface is idle.
  The rule would then depend on which file is compiled, and a library compiled alone is both kinds.

## Consequences

Built 2026-10-25 ([surface-2026-10-25](../../gauntlet/results/surface-2026-10-25.md)):
- **The corpus keeps every interface exactly.** Each declaration ADR 0064 made public in a library is
  marked `pub`: 29 in nine files (five in `lib/`, four in the book's code). The 55 `private` marks go,
  52 of them from files that hid all or nearly all they define. The marks fall from 55 to 29.
- **Two libraries keep their entry points by marking them**: `lib/num/u128` (4) and `lib/win/fmt` (3)
  were compiled alone, emitting their functions, so each is `pub export`, and their emission and the
  proof counts are byte-identical. `examples/modules.oro`'s nested `dot`, both visible and an entry
  point, is `pub export` too.
- **Emission**: every program byte-identical. Compiled alone, `lib/os` (three targets) and `lib/result`
  (four) have no entry point and emit an empty file, where the sweep refused them for compiling their
  interface; accepted. 2,103 of 2,148 operations and 353 of 376 loops, unchanged.
- **The hole was load-bearing in tests**: twelve test snippets imported a module that marked nothing,
  and resolved only because an empty interface showed everything. Each now marks what it uses.
- **A scan read declarations by their first token** (`core/jparse.go`'s variant registry, which wants
  `set` at a line's start); it reads past the markers now (`declStart`).
- **The book's prose** (docs/book/, chapter 2 §2.9) is in the notation J replaced and says nothing of
  `pub`; its code is migrated, its text waits for the book's own rewrite.
