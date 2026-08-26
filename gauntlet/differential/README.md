# Differential conformance

**One program, four targets, the same answer — and the answer it should be.**

```bash
cd gauntlet/differential
go run run.go              # every case, every target
go run run.go match        # cases whose name contains "match"
go run run.go -keep        # leave the work directories to look at
```

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

## Writing a case

A case defines `run`, and nothing else. The harness supplies the `main` that prints, because
printing is target-native on every host and is the one thing a portable program cannot do.

```lisp
; expect: 0 1 2 3 4 4 4 4 4
; inputs: 0 1 2 3 5 8 13 21 34     (optional; this is the default)
; skip: windows                     (optional; say why in the comment)

(def run (fn (n)
  (loop ((acc 0) (i 0))
    (@ge i n)   acc
    (@gt i 3)   (again acc (@add i 1))
    else        (again (@add acc 1) (@add i 1)))))
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

`; skip:` is a real answer — a program using a host's own name is not portable and this is not the
place to pretend otherwise — but it has to be **said**, not achieved by a case quietly running
nowhere.

## What it covers

| case | construct |
|---|---|
| `loop-counter` | `loop`/`again`, and the post-clause hoist |
| `match` | `match`, `when` guards, a bare-name scrutinee becoming the loop variable |
| `sum-case` | `sum`, `case`, a **dynamic** tag and the commuting conversion |
| `values-product` | `values` consumed in place, so nothing is built |
| `table-alloc` | `alloc`, `table` as a rule, `len`, indexing |
| `table-bool` | `build`, `set`, a linear buffer, and the x86 element width |
| `array-literal` | `array` as a graph, and a dynamic index that cannot fold |
