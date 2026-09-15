# 0023 — A generator does not make a claim it cannot justify

Date: 2026-09-15
Status: Accepted. Applied at least six times between 2026-09-06 and 2026-09-13, and written down here
for the first time. assessment-2026-09-13 named it as owed.

## Context

This repository measures how much of four hosts' APIs the language can declare, with tools that also
**generate** declarations: `gauntlet/stdlib/survey.go` (Go), `win32.go` (Win32), `jvm.go` (the JVM) and
`js.go` (JavaScript). A generated declaration makes claims: an arity, types, purity, a result range, a
subtyping edge, which library an import lives in. The surveys' percentages are computed from those
claims.

**The recurring failure was a claim the tool had no grounds for, and it happened in both directions.**

| claim | what was wrong | found by |
|---|---|---|
| every generated Go primitive was `pure` | including `os.Chdir`, whose whole purpose is changing global state; a pure declaration lets the reducer substitute a call into two places or drop it (ADR 0010) | gomethods-2026-09-09 |
| a constructor for every struct type | `&bytes.Buffer{}` is the documented idiom and `&os.File{}` is a broken file, and the manifest cannot tell them apart | struct-literals.md, structlit-2026-09-10 |
| an `implements` edge wherever method sets matched | an interface sealed by an unexported method looks satisfied by everything: **182 of 1,651** candidate edges were false | coercion-2026-09-09 |
| a library chosen for every Win32 import | **155** names are bound to *different DLLs* by different import libraries (`AbortPrinter` to `winspool.drv` and to `spoolss.dll`); the first rule for choosing was a heuristic, and it was wrong twice | linkline-2026-09-13 |
| floating-point Win32 entry points declared like integer ones | **36** read their result from `RAX` where Win64 returns it in `XMM0` | win32enum-2026-09-11 |
| `TYPE *name` read as a value | **585** generated declarations typed an address as an integer; the old declaration of `GetDevicePowerState` *compiled* a program passing the literal 0 | structval-2026-09-12 |

**The same shape in the published numbers**, each a claim about the language's reach the tool could not
justify:
- Go's usable share was **31.7%**, and **27.5%** once two same-named types in different packages
  stopped being merged;
- Win32's callable share was **36.8%**, and **35.4%** once a pointer the program cannot build stopped
  counting as a passable integer;
- struct literals were projected at **+486** names, and built at **+229**, because the projection
  seeded a fixed point instead of running it.

CLAUDE.md records the lesson each time in nearly the same words: *"when a measurement of what a
language can do is written by the same hands as the language, the first number describes the
measurer."*

## Decision

**A generated declaration states only what a host artifact or a host oracle justifies.** Everything
else takes the **conservative default**, and the default is chosen so that an omission costs capability
or speed, **never correctness**.

**What justifies a claim:**
- a **host artifact**: Go's `api/go1*.txt` manifest, the Windows SDK's headers and import libraries, the
  JDK's reflection over `jrt:/`, V8's own introspection;
- a **host oracle**: the host's own compiler or linker accepting a generated probe
  (`var _ io.Reader = *new(*os.File)` under `go build`; MSVC's `sizeof` table; `-check-enums` with a
  control that must be refused).

**The conservative defaults:**

| claim | default without justification |
|---|---|
| purity | **impure** (ADR 0010's default, for ADR 0010's reason) |
| a subtyping edge | **no edge** |
| a constructor | **none** |
| an import library | **no `lib`**: the linker refuses the call rather than binding it to the wrong DLL |
| a result range | **the type's full range** |
| a calling convention the backend cannot place | **refused** |
| anything a probe was needed for, when the toolchain cannot run | **not made at all**: *"an unverified edge is a claim"* |

**Three corollaries.**

1. **A name counted declarable and never emitted is a claim.** Every survey closes its arithmetic —
   declarable = emitted + a named residue (Go: 4,331 − 20 voids + 462 constructors = 4,773) — and
   `gauntlet/stdlib/tooling_test.go` checks it.
2. **An oracle that misreports is still reported, never corrected by guessing.** V8 reports
   `Function.length` as 0 for 604 of 2,326 native functions. The JavaScript survey emits the arity the
   host gave and records the misreport; it does not invent a better one.
3. **A heuristic is not an oracle.** A rule of the form *"a library listing many DLLs is an umbrella"*
   or *"a name in capitals starting with P is a pointer"* justifies nothing until a host artifact or
   oracle confirms it, and the confirmation is what the declaration rests on.

## Why not

**Heuristics, tightened when they fail.** Tried repeatedly, and each fix moved the error rather than
removing it:
- the DLL "umbrella" rule reported 3,622 linkable names where the import objects' own bindings say
  3,593, and it hid 25 disagreements it could not see;
- enum detection by regular expression was wrong twice before the decision moved to reading the brace
  that opens the declaration.

A heuristic that is right on the cases someone thought of is exactly what a survey written by the
language's authors produces.

**Claim, then let acceptance programs catch the mistakes.** Thirteen acceptance programs are the only
semantic check on **48,518** generated declarations. A claim no program exercises is never tested, and
`wide-call.oro` showed that a witness can be built so it cannot fail: it called `MulDiv(7, 6, 2)`, which
could never reveal the sign bug it was meant to guard.

**Claim, with a flag saying "unverified".** A flag nobody reads is a claim with a footnote. Purity
already has the right answer, which is the conservative default and not an annotation.

**Refuse to generate at all.** The surveys are the only measurement this project has of whether the
parasite thesis reaches a host's whole API, and ADR 0022's hand declarations need them as a checker.
Generation with justified claims is worth keeping. Generation with unjustified ones is what this ADR
removes.

## Consequences

**Survey numbers move to where the host puts them, in both directions.** The Go survey alone has been
corrected seven times, four downward and three upward (surveys-2026-09-10). Removing an unjustified
claim lowers a number, and removing a misreading can raise one. The published figures carry their
corrections beside the numbers they replaced, so a reader can see what moved and why.

**Some claims move to people.** What no host artifact justifies — what a call does to a buffer, a
precondition, a narrower result range — can only be claimed by someone who read the host, and ADR 0022
is where that happens.

**Probes are part of the generator.** Coercion edges need `go build`; enum widths and aggregate sizes
need MSVC. A machine without the toolchain generates fewer claims, not guessed ones, and the tooling
tests skip that host by name rather than passing.

**It commits the surveys to reporting their residue.** Every refusal has a named reason and a count, so
*"what is left"* is a list someone can act on rather than a gap in a percentage.

**Trigger to revisit:** a class of claim for which a **sound derivation** exists, rather than an oracle —
purity derived by reading a function's body, or a buffer's usage read off it. That would need its own
soundness argument, and host-buffers.md shows the difficulty: a body-reading classifier could not read
79.9% of the Go standard library's slice parameters at all. **Win32's SAL annotations are the nearest
case**: `_Out_writes_(n)` is a host artifact that justifies a buffer claim, so a generator reading SAL
may make that claim under this ADR as written.
