# Go's top level

An experiment, not a deliverable. **The measurement is
[gauntlet/results/go-toplevel-2026-08-18.md](../../gauntlet/results/go-toplevel-2026-08-18.md)** —
read that first; this file is what the experiment covered and where the walls are.

## The question

Can Oroboros express **Go's top level** — everything usable with no import — well enough to write
real programs? Not portable programs. Go programs, in Oroboros, testing
[ADR 0001](../../docs/decisions/0001-parasite-model.md)'s claim that portability is a property a
program may or may not have.

Two modules, declared in `targets/go.oro`:

- **`go/builtin`** — Go's predeclared identifiers. Go documents these in a pseudo-package called
  `builtin`, which is exactly the right name for "everything with no import".
- **`go/fmt`** — output, the one import a program cannot do without.

```lisp
(use go/builtin as g)
(use go/fmt as fmt)
```

## Programs

| | what it exercises |
|---|---|
| [sieve.oro](sieve.oro) | `make([]bool)`, indexed read and write, nested loops, integer `%` |
| [histogram.oro](histogram.oro) | `append`, maps, slicing, `let`, `seq` |

`sieve_bench.go.txt` is the harness for the measurement — kept as text so it is not part of the Go
module.

Emit and run:

```bash
go run ./cmd/gen experiments/go-toplevel/sieve.oro go /tmp/sieve.go
```

## What worked, and it is most of it

Every predeclared function tried is one line of target file. Two results worth keeping:

**Mutation needed no new mechanism.** A `stmt` primitive's value is its first argument, so an
indexed write yields its container back and threads through a fold exactly as an accumulator does.
The effect discipline then pins the order for free — no new kind, no new rule.

**`append` is impure, and the discipline gets that right for the right reason.** It may write into
the argument's backing array when capacity allows, so copying, dropping or reordering it is
observable. Declaring it without `pure` is not a hedge; it is the truth.

## The walls

Ordered by how much they cost.

### 1. ~~The loop's iteration space — 1117×~~ **Retracted**

See [loop-encoding-2026-08-18](../../gauntlet/results/loop-encoding-2026-08-18.md). A start and a
step need only a computed **trip count**, which `fold-range` has always accepted, so
[sieve_counted.oro](sieve_counted.oro) runs at **1.2×** of hand-written Go with no new primitive.
The 1117× measured the naive encoding.

What survives: **early exit** has no trip count to compute and is genuinely missing.

### 2. No unbounded iteration

Collatz has no trip count known before entry, so `histogram.oro` writes it as a **fixed budget with
an idempotent tail** — run 200 steps, and once the value reaches 1 keep returning it. Correct,
wasteful, and the shape every convergence loop is forced into.

### 3. The type language has no constructors

`[]int` and `[]bool` are two unrelated atoms, not one constructor applied twice. So:

- every element type needs its own `make`: `make-int`, `make-bool`;
- every element type needs its own indexed read and write;
- `len` works only because `any` demands nothing — at the cost of the checker learning nothing
  from it.

Go's own `[]T` is parametric. Ours is a list of names. This is the first place the monomorphic type
table has been visibly expensive rather than merely small.

### 4. No multi-value return

`v, ok := m[k]` is inexpressible, so a map read yields Go's zero value and there is no way to
distinguish "absent" from "present and zero". Same for `i, err := f()`, which is *the* Go idiom.

This is the same missing product that [loops.md §5](../../docs/spec/loops.md) needs for a
multi-accumulator fold, arriving from a completely different direction. Two independent demands for
one feature is the strongest evidence the feature is real.

### 5. Statements are expressions, and it shows

`if` is an expression, so every conditional inside a loop becomes

```go
var t2 []bool
if cond { t2 = ... } else { t2 = ... }
c = t2
```

where Go would write `continue`. Correct, and three times the size. Bound up with wall 1: with early
exit, most of these conditionals disappear rather than needing a statement `if`.

### 6. Not attempted

Structs and methods (no way to declare a type with fields), interfaces, pointers, `defer`, `go`,
`select`, channels, `range` over a map, variadics, type assertions, closures that escape, `goto`.
Some are refused by design; the rest are simply out of reach and would need their own experiment.

## What the experiment cost, and returned

It found **two bugs in the emitter**, one of them a silent wrong answer that had been latent since
nested folds became possible — see §5 of the measurement.

That is the argument for writing non-portable programs deliberately: the gauntlet's seven programs
are all *portable* ones, and portable programs turn out to exercise a narrow set of shapes. The
first program written to be deliberately unportable broke two things.
