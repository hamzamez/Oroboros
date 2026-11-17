# Differential conformance

**One program, four targets, the same answer — and the answer it should be.**

```bash
cd gauntlet/differential
go run run.go              # every case, every target
go run run.go match        # cases whose name contains "match"
go run run.go -keep        # leave the work directories to look at
go run run.go -j 4         # cases at once (default: half the cores)
go run run.go -cases DIR   # another directory of cases (the harness's own test)
```

The cases run in parallel, `cmd/build` built once; each report is printed in the cases' order, and a
verdict depends on nothing but its case (`cmd/check`'s `TestTheParallelDifferentialStillFails` plants a
wrong answer among them; differential-2026-11-13).

## Why this exists

[`../conformance/`](../conformance/) tests our **lowerings** of one primitive, by hand-writing the
same test in three languages. This tests the **compiler**: it takes an `.oro` program, builds it on
every target, *runs* each artifact, and requires the outputs to be byte-identical.

It exists because **emitting is not the same as being right**, and two silent wrong-answer bugs in
one day proved it:

- The JavaScript post-hoist emitted the loop increment **twice**, so the sieve advanced by two per
  iteration and got **1984 of 2000** answers wrong. It compiled and returned a number
  ([loopshape-2026-08-25 §3](../results/loopshape-2026-08-25.md)).
- The x86 element-width pass read a byte table with a **qword** load, taking seven bytes of the
  following elements per access
  ([wintables-2026-08-25 §4a](../results/wintables-2026-08-25.md)).

Both were caught only because someone hand-wrote a reference and compared. The example sweep and
`../conformance/run.sh` both check that a program **emits**, and both of these emitted cleanly.

**Both are cases here, and both fail this suite when their fixes are reverted.** That was the pass
condition for writing it.

## Two checks, not one

**Agreement**, across every target that is not skipped. No target is the oracle; a disagreement
prints every output.

**`; expect:`**, the answer the case should give. Agreement alone is not correctness — four
backends can be wrong the same way, and the one bug a purely differential test cannot see is a bug
in the **reader** or the **reducer**, which all four share. A case without an `; expect:` line
fails.

**Proven, unless the case says otherwise.** Every case is built with `-checked`, which exercises
the rebuild path, and `-checked` turns an operation the compiler cannot bound into a run-time trap.
So `cmd/build` reports each trap it took, and the harness holds the case to what it declares:

```lisp
; checked: windows — why the analysis cannot bound it there
```

A target named there must still need it, or the case fails as stale. Every other target must prove
every integer operation, or the case fails. The reason is required. Without this, a case written to
be proven (u128's arithmetic) passed with its proofs gone ([bounds-2026-09-24](../results/bounds-2026-09-24.md)).

**A declared bound's trap is an answer.** Above the word a range is enforced at run time, and the
property is that every target traps on the same value (ADR 0029). A run that stops with the bound's
own message (`bignum overflow: the declared range is too small`) answers what it printed, then
`trap`, so `; expect: 5 5 trap` states it and a target that answers instead disagrees. Any other
failure is a failure.

**So is an abandonment** (spec/errors.md §8). `expect` crashes with its reason on any constructor but
the success, and every target writes the reason to standard error. A case names it,
`; abandons: REASON`, and a run that stops with it answers what it printed, then `abandon`:
`; expect: 0 1 2 abandon` (cases/expect.oro).

## Writing a case

A case defines `run`, and nothing else. The harness supplies the `main` that prints, because
printing is target-native on every host and is the one thing a portable program cannot do.

```
// expect: 0 1 2 3 4 4 4 4 4
// inputs: 0 1 2 3 5 8 13 21 34          (optional; this is the default)
// skip: windows because "TEXT"          (optional: TEXT is a part of the failure)

fn run(n) {
    loop acc = 0, i = 0 {
        i >= n -> acc
        i > 3 -> again(acc, i + 1)
        else -> again(acc + 1, i + 1)
    }
}
```

`@add @sub @mul @lt @le @gt @ge` are replaced with each target's own spelling — `go.+`, `js.+`,
`java.+`, `x64.add`. **This is not the portable layer sneaking back in.** A test has to say "the
same program on four hosts", and the only thing allowed to differ is the host's own spelling, so
the substitution is exactly the set of names where the four agree on meaning and disagree on
spelling. Anything they genuinely disagree about — integer division, float formatting — is
deliberately absent, because a differential test of a real divergence would only ever fail.

`; inputs:` exists because reduction **inlines** `run` at every call site: nine calls that each
build a table is nine tables in one procedure, which on x86 is more spilled operands than there are
scratch registers. A heavy case uses fewer inputs. That ceiling is real and belongs to the backend;
provoking it from a test harness measures the harness.

`// skip:` is a real answer — a program using a host's own name is not portable and this is not the
place to pretend otherwise — but it has to be **said**, not achieved by a case quietly running
nowhere. **And it is a claim, checked** (the harness, 2026-11-16): `// skip: T because "TEXT"`
says the case fails on T, and TEXT is a part of the failure, an error's text or the output. The
skipped target is still built and run. If it builds and agrees, the skip is **stale** and the case
fails; if it fails some other way, the case fails too, naming what it did. A skip with no reason is
refused. Three skips hid the JVM's unsigned word for a month (llvm64-2026-11-06), and the first run
of the check found one more, `word-wide-native` on JavaScript.

## What it covers

| case | construct |
|---|---|
| `loop-counter` | `loop`/`again`, and the post-clause hoist |
| `match` | `match`, `when` guards, a bare-name scrutinee becoming the loop variable |
| `sum-case` | `sum`, `case`, a **dynamic** tag and the commuting conversion |
| `values-product` | `values` consumed in place, so nothing is built |
| `record-lens` | a record's lens laws, each on components that differ (ADR 0128, data.md §9) |
| `record-path` | an override along a path, against nested `with`s |
| `record-zero` | `zero(T) with { … }` on a nested record set |
| `record-loop` | a record carried by a loop and by a cell, curried at each binder |
| `record-order` | a table of records written in two orders, read through canonical projections |
| `record-buffer`, `tuple-buffer` | a buffer of records, of tuples, written in program order and read back |
| `table-alloc` | `alloc`, `table` as a rule, `len`, indexing |
| `table-bool` | `build`, `set`, a linear buffer, and the x86 element width |
| `array-literal` | `array` as a graph, and a dynamic index that cannot fold |
