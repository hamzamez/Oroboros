# The book's chapter 0: from a file to a program

Date: 2026-11-23. hamza, 2026-11-23: "someone coming new to the book, will not know how the language
compiles. should we add another chapter about that … should't be chapter 0 and we renumber
everything else? … and examples for someone who wants to test outside the repo without polluting it".

## What a new reader lacked

The tour, then chapter 0, ran `gen`, `build` and `portable` from its first page, and no chapter said:

- what each tool is for;
- what the compiler does in what order;
- why `tutorial` emits nothing;
- how to compile a file of one's own without writing into the repository.

The tools resolve `targets/` and `lib/` relative to the current directory. So the obvious attempt,
running a built tool from another directory, fails with *-targets names "targets", which is not a
directory*.

## What was built

1. **[00-compiling.md](../../docs/book/00-compiling.md)**, 36 checked blocks, its sources in
   `docs/book/code/compile/` (12 files):
   - §0.1: what each target needs;
   - §0.2: the five tools on one file, `build` run on four targets;
   - §0.3: the eight stages, each with a program it refuses:
     - reading (`no-read.oro.txt`);
     - desugaring and loading (an unbound name);
     - reduction (arity);
     - the claims (a result past its set);
     - lowering, its IR text in full, and a closure refused;
     - deciding (`intervals`, `mul exact`);
     - printing on four hosts;
     - the host's compiler;
   - §0.4: targets (`go/strings` on Go only by `portable`, `tutorial`'s reduction and its refusal to
     emit, the windows family);
   - §0.5: `gen` or `build` (`main`'s absence refused, an internal definition inlined), and `gen`'s
     output called from Go, JavaScript and Java;
   - §0.6: outside the repository, with `go -C` and with the tools built once and given
     `-targets` and `-path`, and a module beside its user.
2. **The checker** (`docs/book/check.go`) gained three directives:
   - `core`, `oro` without `-j`;
   - `ir`, the text `gen -ir` writes;
   - `intervals`, `intervals -v`.

   Each was trusted after a planted false line failed it: a core term with `* n 3`, an IR line with
   `(int 0 300)`, a proof count of `1/2`.
3. **The renumbering**: the tour is 1 and every chapter after it moved up by one, A whole program
   now 20. A script shifted, in the chapters and the book's README:
   - each chapter's title and headings;
   - every "chapter N", "chapters N and M", "section N.M";
   - every bare `§N.M`;
   - every link.

   It left a `§` whose preceding token names another document alone (`tables.md §3.2`, `data.md
   §4.6`, a research note's `§7`). It was run as a diff first, and its skipped section signs were
   listed: 17, each a spec's. Afterwards every bare `§N.M` and every "chapter N" in the book was
   checked against the headings and files that exist. One dangled, a line of the README's
   2026-10-06 verification pass, which already named a section that no longer exists.
4. **Outside the book**:
   - links to a chapter's file in seven documents point at the new names;
   - the prose numbers changed in the current documents only: CLAUDE.md, the root README, four
     specs;
   - the result documents and ADRs keep the numbers of their day, as the README now says.

## Run by hand, not by the checker

The checker runs commands from the repository's root on the repository's files. What the chapter
shows outside it was run on 2026-11-23, in a directory outside the repository, on Windows 11, and
`git status` stayed empty:

- `go -C <repo> run ./cmd/build` for Go, JavaScript, Windows and Java, each program printing
  `hello from outside`;
- the tools built with `go -C <repo> build -o DIR/ ./cmd/…` and run from a project directory:
  without `-targets`, refused as quoted; with it, a two-module program printed `42`;
- `gen`'s output called from Go (`play/gauntlet`), JavaScript (`import { calcSumTo }`) and Java
  (`Sum.calcSumTo`), each printing `45`.

## What failed first

- **A dry run printed one file of the diff and stopped.** Python's console encoding was cp1252, and
  the first `∈` ended it; `PYTHONIOENCODING=utf-8` printed all 2,348 lines.
- **`gen` writes `package gauntlet`**, the benchmark package's name, with no flag to change it. The
  chapter shows the file used as it is, in a directory of that name, and the flag is named below.

## Paid and bought

| | |
|---|---|
| compile time | 1.00× (the gate's median) |
| emitted text, proofs, refusals | unchanged: byte-identical on 844 runs, 12,504 of 12,558 operations, 1,575 of 1,876 loops |
| the book | 21 chapters, 1,299 checked blocks (1,263 before; chapter 0's 36) |
| checker lines | 3 directives, about 25 lines |
| documents changed | 21 chapters renamed and renumbered, the book's README, CLAUDE.md, the root README, 4 specs, 1 ADR's link, 1 result's link |

## Not built

- `gen -package NAME`, so that a Go host need not name a directory `gauntlet`.
- A checked form of §0.6: a directive running a tool from outside the repository.
