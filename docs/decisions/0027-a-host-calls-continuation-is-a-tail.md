# 0027 — A host call's continuation is a tail position

Date: 2026-09-24
Status: Accepted. **Extends [ADR 0015](0015-loop-and-again.md)**: "`again` may sit under a `let`"
now covers the `let` that binds several names, which 0015 did not have. Nothing 0015 decided is
reversed.

## Context

A host call with several results is eliminated by a continuation:

```lisp
((bits.Mul64 l w) (fn (hi lo) BODY))          ; = (let (tuple hi lo) (bits.Mul64 l w) BODY)
```

`BODY` sits inside a `fn`, so the reader treated it as a closure, and a jump out of a closure has no
meaning. `again` there was refused, and so was a several-results return from an export whose value is
built inside one. [u128-2026-09-23](../../gauntlet/results/u128-2026-09-23.md) met this three times:
- a loop whose next state is `mulw`'s three results;
- the same loop bound first and then projected, which left a closure;
- an export returning a tuple from inside `Add64`'s continuation.

The program got round the first by calling `mulw` once per component. That works only for a pure
call. A loop that reads a byte per iteration cannot be written at all:

```lisp
(loop ((count 0) (k 0))
  (>= k 100) count
  else (let (tuple b err) (gio.ByteReader.ReadByte r)
         (again (+ count 1) (+ k 1))))
```

`ReadByte` changes the reader, so writing it once per component reads once per component. `bufio`,
the next package, is mostly loops of this shape.

## Decision

**1. The positions a jump may occupy are the tail contexts**

```
E ::= []  |  let x = e in E  |  if c E E  |  (p a…) (λx̄. E)
```

The fourth line is new: `p` is a primitive declaring n ≥ 2 results and `λx̄` takes exactly n. It is
the n-ary let, which the reducer already pushes an eliminator into (state.md, the commuting
conversion). A clause body `(again …)` and a several-results return may sit in any E.

**2. In source, the rule stays one sentence:** *let binds; if branches; binding may wrap an `again`,
branching may not.* A binding is `(let x e …)` or `(let (tuple x̄) e …)`. `again` under an `if` is
still refused in source (ADR 0015's readability rule). A residual may carry one there, because
case-of-case puts it there, and every walker already handles `if`.

**3. The reader knows a tuple binding by what was written, not by its shape.**
`(let (tuple a b) e body)` desugars to `(e (fn (a b) body))`. So does a call to a program's own
higher-order function, `(f (fn (a b) body))`, whose lambda may run twice, never, or later. Only the
first is a binding. The reader marks the tuple binding it builds, checks the clause against the mark,
and erases the mark before any form leaves the reader. Nothing below the reader sees it.

**4. Why it is sound.** A host call's continuation is **second-class**:
- it runs exactly once;
- it runs immediately after the call;
- it never escapes.

Every backend emits it as the call followed by the body's statements, in the same block:
- Go: `hi, lo := bits.Mul64(l, w)`;
- JS: a destructuring binding;
- Java: a record, or declared and assigned destinations.

A jump there is a jump from that block, which each host allows. This is the known-continuation case
of CPS compilation:
- Appel, *Compiling with Continuations* (1992), and Kennedy, "Compiling with continuations,
  continued" (ICFP 2007): a continuation that cannot escape compiles to a jump, with no closure.
- It is also SSA with block arguments, as iteration.md §5 already reads `again`: the host call
  returns into a block whose parameters are x̄.

A lambda passed to the program's own function has none of the three properties, and a jump inside
one stays refused.

**5. Every walker of a clause chain treats the n-ary let as it treats the one-name let.** Each binder
takes the call's declared result: its type, and its range as a fact. The body is walked in the same
tail position. The walkers are:
- the type checker's loop body;
- the interval analysis's back-edge collection, which also feeds size-change termination;
- the monotonicity theorem's walks;
- the three emitters' loop bodies and several-results returns.

A walker that enumerates back edges and misses one is unsound, not merely imprecise, so each one gets
a witness that fails when its case is removed.

**6. Each target.** Go, JS and Java emit the call as they already do and continue in the same block.
windows declares no host call with several results, so it has nothing to emit. The first one it
declares takes the same case. A jump anywhere else in the residual is refused by the backend by name.

## Why not

- **Keep the wall, and project each component** (u128's workaround). It duplicates every pure call,
  so the u128 loop emits 6 `Mul64` where 2 suffice. For an impure call it expresses nothing: a read
  loop cannot be written.
- **Recognise the tuple binding by its shape.** Built and reverted in the u128 round:
  `(f (fn (a b) …))` has the same shape, and accepting a jump inside it miscompiled silently. The
  distinction is what the programmer wrote, and only the reader knows that.
- **Float the binding outward by let-associativity**, `(let ((p a) (fn (x̄) T)) (fn (t) B)) ⟶
  ((p a) (fn (x̄) ((fn (t) B) T)))`. It exposes a built tuple to β, but it moves the `again` into
  the continuation, which is this decision one step later. It was built, and reverted for that.
- **Turn a host call's results into a stored product** that a one-name `let` binds and projects.
  That builds a value the backends currently never build: the boxing the core forbids
  (design-direction.md §2).
- **Allow a jump inside any lambda.** A jump out of a first-class function is a first-class
  continuation. It needs the function to run exactly once, now, which nothing checks for a
  program's own higher-order functions.

## Consequences

- A loop can consume a fallible host call per iteration: reading, parsing, scanning. That is what
  `bufio`, `encoding/*` decoders and every `(value, err)` API need.
- A pure multi-result call in a loop is written once, so u128's step emits one `mulw`, not three.
- An export may return a tuple built inside a host call's continuation, so a module like
  `num/u128` compiles and is checked on its own.
- Every new clause-chain walker must handle four forms, not three. ADR 0015's "only a clause body or
  under a `let`" was an assumption some walkers relied on: `monotoneStep` passed over the new
  position and would have certified a loop it had not checked. The witnesses are what keep the
  next walker honest.
