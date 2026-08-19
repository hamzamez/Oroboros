# 0016 — A target need not be an expression language

Date: 2026-08-19
Status: Accepted

## Context

Three targets existed — Go, JavaScript, Java — and all three are expression languages. A target
file says

```lisp
(prim + (int int) int expr "%s + %s" pure)
```

and the hole is filled with the operand's *emitted expression*; the host's own parser rebuilds the
tree. Every backend's `emit` returns a string that is an expression, and statements are pushed into
a side buffer.

That is a design, and until now it was also an untested assumption. [ADR 0001](0001-parasite-model.md)
says targets are ecosystems and the compiler emits at the highest layer the target natively
provides. It says nothing about the target having a grammar with nesting in it — but the format,
and all three backends, quietly required one.

**Assembly does not have one.** `add` takes two operands, writes the first, and cannot be nested in
anything. If the format only works on hosts with expressions, then requirement 2 — many targets —
is narrower than it looks, and requirement 3 — a third party can add one — is false for a whole
class of hosts (assemblers, bytecode, stack machines, shader ISAs, hardware description).

The question could not be answered by argument. It was answered by writing the target
([ADR 0007](0007-exploration-over-specification.md), [ADR 0008](0008-measurement-over-principle.md)).

## Decision

**A backend's `emit` returns a PLACE, not an expression, and the target-file format is extended by
three holes rather than restructured.**

`targets/windows/` emits x86-64 assembly under MASM, links against kernel32 and msvcrt, and
produces a `.exe`. `emit/asm.go` returns a `place` — a register, an immediate, or a frame slot —
where the other three backends return a string. That is the entire adaptation to the backend
shape: it still returns one value and pushes statements into a buffer.

The format gains, and nothing is removed:

| addition | why |
|---|---|
| `%r` | there is no expression to *be* the result, so the template is told where to put it |
| `%u` | a unique number, so a template may carry its own labels and therefore its own control flow |
| `%1…%9`, `%b`, `%e` | operands by position, and one register's three width-names |
| `(jump "cc" ["form"])` | the host's condition code for a predicate, so a guard branches instead of materialising a boolean |
| `(data "…")` | storage the target owns, for out-parameters the language cannot spell |

**The structural set does not grow.** `let`, `if`, `loop` — the same three as the other three
native targets, on a host that has none of them.

The result is at parity with hand-written assembly:
[windows-2026-08-19](../../gauntlet/results/windows-2026-08-19.md). 0.97× median against a 15%
noise floor, with the inner loops instruction-for-instruction identical, and a `hello` artifact
the same size as hand-written to the byte.

## Why not

**Why not a separate format for non-expression targets?** Because the measurement says one format
covers both. Two formats would double what a third party has to learn and would have been chosen
before knowing that three holes were enough.

**Why not make `expr` a stack-machine template — `push`/`pop` around every operand?** It is the
obvious way to fit assembly into an expression-shaped format with no additions at all, and it
would work. It also spills every intermediate to memory and would have lost to hand-written code
by a wide margin. The whole point of requirement 5 is that this is not an acceptable answer, and
choosing it would have "proved" the format sufficient by giving up the thing the format is for.

**Why not do register allocation in the target file?** Because it binds variables and emits
control flow, which is exactly the line [target-files.md §8](../spec/target-files.md) draws around
structural kinds. Allocation is in `emit/asm.go` and a target file never mentions a value
register.

**Why not let templates use any register?** A template may clobber rax, rcx, rdx, r8, r9 and
xmm0-xmm3 — the Win64 volatile set minus the two scratch registers this backend reserves to carry
spilled operands in. Widening that would mean the backend could no longer materialise a spilled
operand without asking the template what it touches, which is a dependency the format does not
have and should not gain.

**Why not add a structural kind for `while`, or for a jump?** No program needed one. The clause
chain of ADR 0015 lowers to `cmp`/`jcc`/`jmp` with no new kind, which is the strongest available
evidence that the three are the right three.

**Why not skip `(jump …)` and let the peephole find it?** Recovering `cmp a,b / setl / cmp r,0 /
je` back into `cmp a,b / jge` is a real analysis over flags and register liveness — the same shape
of work as recovering structure from `goto`, which
[CLAUDE.md](../../CLAUDE.md) already refuses. One declared string in the target file is the
information the host compilers had all along, arriving the way everything else about a target
arrives: as data.

**Why not treat this as evidence to freeze the core?** It is not. It is one more falsifier that
did not fire ([assessment-2026-08-13](../assessment-2026-08-13.md)). The core is still
deliberately unspecified.

## What it cost, recorded rather than hidden

The full list is [windows-target.md §6](../spec/windows-target.md). Three that bear on other
decisions:

- **The fifth independent demand for a product type**, and the first from hardware: one `idiv`
  produces quotient and remainder, and we declare two primitives and divide twice.
- **No common-subexpression elimination.** The first three hosts did it for us and we never
  noticed which optimisations we were parasitizing until a host had none.
- **`cmd/build` could not build any program containing a `loop`.** `core.Residual` treated `again`
  as a free name. ADR 0015 was validated entirely through `gen`, which emits a function, never
  through `build`, which makes a binary. A bug on all four targets, found by the fourth.
