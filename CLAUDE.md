# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## Project state

**One gauntlet program is not at parity, and that is an accepted, provisional decision** —
[ADR 0013](docs/decisions/0013-accept-the-allocation-price.md). g7's stencil runs at **1.79× on Go
and 2.01× on JS** against a hand-written buffer-reusing form
([measurement](gauntlet/results/stencil-2026-08-15.md)). The emitted code is at parity with
hand-written *functional* code; `materialize` is what costs, because it allocates fresh so nothing
can alias.

**The price is the SHAPE, not the compiler, and it is avoidable one layer down** —
[native-gauntlet-2026-08-20](gauntlet/results/native-gauntlet-2026-08-20.md). The stencil moved to
the native Go target carrying both forms: allocating measures **0.93x** hand-written and
buffer-reusing measures **0.999x**. In each shape emitted matches hand-written, and *allocating*
costs 2.71x for hand-written code too. So the portable layer has no way to express reuse and a
native target does — `go.set-float64` is Go's own store, no portability claim, at parity.

**This price is expected to be paid off, not kept.** The ADR names the triggers that should reopen
it. Note the correction recorded there: the original first trigger, *a type system exists*, **fired
and bought nothing** — uniqueness constrains the *context*, not the value, so it is not a
refinement. The nearest machinery is ADR 0010's substructural discipline plus the reducer's
occurrence counting, and that is a hypothesis rather than a finding. Do not treat 1.8× as the bar;
the bar is still hand-written code.

**Working compiler.** A β/δ reducer with call-by-need and an effect discipline, three backends
(Go, JavaScript, Java), and **all seven gauntlet programs** reaching parity with hand-written
code — two of them producing byte-identical machine code on Go.

Start with [README.md](README.md), then [docs/design-direction.md](docs/design-direction.md),
then the ADRs in [docs/decisions/](docs/decisions/). Measurements are in
[gauntlet/results/](gauntlet/results/), and they are the authority: **every design claim in this
repository that was not measured has been wrong about half the time.**

## What this project is

Oroboros is a small language and build system that **parasitizes** target ecosystems rather
than abstracting over them. Portability is a property a program may or may not have, computed
by the compiler — not a global guarantee.

The single mechanism is a **capability graph**: modules declare required capabilities, targets
declare provided capabilities plus shims, and building means covering the former with the
latter. The governing rule is **emit at the highest layer the target natively provides** —
lower only as far as necessary.

## Decisions already made

Do not relitigate these without reading the ADR first. Each one has a "Why not" section
recording alternatives that were considered and rejected.

| Decision | ADR |
|---|---|
| Targets are ecosystems; portability is a program property | [0001](docs/decisions/0001-parasite-model.md) |
| Capability graph, not a fixed layer tower | [0002](docs/decisions/0002-capability-graph.md) |
| Range-typed integers, mathematical semantics, machine representation | [0003](docs/decisions/0003-range-typed-integers.md) |
| Go, JavaScript, Java/Android first; C deferred | [0004](docs/decisions/0004-first-targets.md) |
| Compiler written in Go | [0005](docs/decisions/0005-implementation-language.md) |
| Backend interface is a file format, not a Go interface | [0006](docs/decisions/0006-ir-file-format.md) |
| Explore candidates against a fixed test; do not specify the core first | [0007](docs/decisions/0007-exploration-over-specification.md) |
| Parasite decisions are per-target measurements, not principles | [0008](docs/decisions/0008-measurement-over-principle.md) |
| Staging must not change results | [0009](docs/decisions/0009-staging-preserves-results.md) |
| Effects are a side condition on β, not a feature | [0010](docs/decisions/0010-effects-as-structural-rules.md) |
| Modules are resolution, not reduction | [0011](docs/decisions/0011-modules-add-nothing-to-the-reducer.md) |
| `int` is exact within ±(2⁵³−1) | [0012](docs/decisions/0012-portable-integer-range.md) |
| Accept the allocation price, provisionally | [0013](docs/decisions/0013-accept-the-allocation-price.md) |
| Recursion is not in the language | [0014](docs/decisions/0014-recursion-is-not-in-the-language.md) |
| `loop`/`again` — guarded clauses over n variables | [0015](docs/decisions/0015-loop-and-again.md) |
| A target need not be an expression language | [0016](docs/decisions/0016-targets-need-not-have-expressions.md) |
| Booleans and control flow are in the language | [0017](docs/decisions/0017-booleans-are-in-the-language.md) |
| Immutable values, one scoped linear buffer | [0018](docs/decisions/0018-immutable-values-linear-buffers.md) |
| Precision by declaration — provisionally | [0019](docs/decisions/0019-precision-by-declaration.md) |

Design questions still open are listed in section 8 of
[docs/design-direction.md](docs/design-direction.md) — memory model, error model,
concurrency, strings, module format, naming translation.

## How this project is run

**The core is deliberately unspecified.** Do not propose freezing it, and do not treat
[docs/core-candidates.md](docs/core-candidates.md) as settled — candidates there are
disposable and expected to die. The predecessor project stalled on a fixed language that
effort then went into making viable; recreating that is the primary process risk.

**[The gauntlet](docs/gauntlet.md) is the one fixed commitment.** Five programs, three
targets, parity with hand-written code. Candidates are killed by measurement, not by
argument — arguments only select what is worth measuring. When a candidate dies, write an ADR
naming what killed it.

The baselines exist and have been run: [gauntlet/](gauntlet/), results in
[gauntlet/results/](gauntlet/results/). The leading candidate — a rewriting core, which turns
out to be **lambda calculus staged so it terminates at compile time** — has survived
hand-derivation of all five programs plus escaping closures, in
[docs/derivations/](docs/derivations/). Read those before proposing anything about lowering,
generics, closures, or capability granularity; each records what was believed, what measurement
said, and which of the two won.

**Current standing** is in [docs/assessment-2026-08-20.md](docs/assessment-2026-08-20.md) — the
analysis round, the one place a demonstration became a decision by accident, and the five things
next. The previous two are [2026-08-19](docs/assessment-2026-08-19.md) — four
targets in, what should go into the language next, and the one place the process has drifted (the
gauntlet still runs on the retired portable layer). The previous one is
[docs/assessment-2026-08-13.md](docs/assessment-2026-08-13.md).
Deliberately *not* an ADR — writing "candidate B is the core" as a decision would recreate the
predecessor's failure. Every falsifier it named has since been tested and none fired, so its
**Revised verdict** section calls for building the vertical slice.

**The atom** — the irreducible unit, what lambda calculus is to a Lisp — is in
[docs/the-atom.md](docs/the-atom.md): **lambda calculus in which the normal form is a
parameter.** A target supplies a partition of names into primitive and defined; reduction runs
until only primitives remain. Layers, both directions of ADR 0002, and staging all collapse into
that, and grading turns out to be an *observation on the normal form* rather than a primitive.
Do not describe the core as "a vocabulary" — that was an earlier and worse answer, and the
vocabulary survives only as the parameter to a reduction relation.

**Beware the minimality trap.** The instinct toward a tiny elegant core — lambda calculus,
objects and messages — minimizes *constructs needed to express all computation*, which is not
the property this project needs and is often opposed to it. Lambda calculus is minimal because
everything is a function, which is exactly why it allocates. That is the Shen wall. The core
should be minimal **subject to lowering natively to every target at zero cost**, the way
WebAssembly is. See section 1 of [docs/core-candidates.md](docs/core-candidates.md).

## Constraints that override normal instincts

These are the non-obvious ones. Violating them produces code that looks fine and undermines
the project.

**Anything promoted to the LANGUAGE works on every target, and the compiler is responsible for
finding the implementation.** `fn`, `def`, `loop`, `if` are the language's. A target does not get
to decline one, and it does not get to *declare* one either — we pick the best way to do it on each
host, and if a host has no native form we build one. The capability graph is for **target-native
names** — `go.map`, `js.at`, `x64.andb` — where *this target cannot do it* is a true and useful
answer that a program can be told. It is **not** for language constructs, where that answer means
the construct is a library carrying a portability claim, which is the thing
[ADR 0001](docs/decisions/0001-parasite-model.md) exists to refuse.

**Applied 2026-08-20**: `let` and `loop` joined `if` as names the compiler injects into every
target, and declaring one is now an **error**. The four native targets declare **no structural
primitives at all**. Eleven target files had carried the same two lines, while `core/read.go`
already desugars `let`, `seq` and `loop` into applications of those precise names — so a target
spelling either differently broke every program, and forgetting one made an ADR 0015 language
construct silently unavailable. What stays declarable is the retired portable layer —
`fold-range`, `fold-range2`, `make-vec` — which is library, not language.

The precedent is already written and was not generalised:
[ADR 0017](docs/decisions/0017-booleans-are-in-the-language.md) put `if` in the language and made
*declaring a boolean name an error* — the target may not even offer an opinion. Every language
construct should be held to that.

The instance that produced this rule: `values` — several results from one function — was built as
reader sugar (so, the language) with a `(multi-return …)` target declaration and a refusal on Java
and windows. That is a construct in the core that two of four targets decline, which is incoherent.
Reverted. If several results go into the language, **Java gets a generated record and windows gets a
register or stack convention**, and finding those is the compiler's job, not the target author's.

**Never lower further than the target requires.** Emitting a hand-rolled hash table into Go
when Go has `map` is wrong on performance, binary size, and ecosystem access simultaneously.
This is the single most common way to get the architecture wrong.

**But never assert which host construct is fastest — measure it.** That rule is a prior, not a
derivation. The first baseline run refuted four inferences from it at once: JS's `Map` is 3.25×
*slower* than a null-prototype `Object`; Java's fused `merge` loses 2.6× to unfused
`getOrDefault`+`put` — **and that one did not reproduce**: on JDK 17 the fused form is 1.19×
*faster*, both are declared now, and the rule survives its own example
([native-java-2026-08-25](gauntlet/results/native-java-2026-08-25.md)); Java's `Point[]` costs 1.05× where JS's array-of-objects costs 2.86×; and
all three hosts inline a literal callback we assumed only we would specialize. Every one was a
plausible reading of how the host is documented to work. Treat host compilers as black boxes
with measured behaviour. See [ADR 0008](docs/decisions/0008-measurement-over-principle.md).

**Staging must never change an answer.** Compile-time arithmetic must be bit-identical to
runtime — IEEE-754 binary64 for floats, exactly. Go folds `0.1+0.2` to `0.3` at compile time and
`0.30000000000000004` at runtime because its untyped constants are arbitrary-precision. Writing
the compiler's constant folder the natural way in Go reproduces that bug and makes partial
evaluation unsound. Force every compile-time float operation through explicit `float64`. See
[ADR 0009](docs/decisions/0009-staging-preserves-results.md).

**Never make the core a superset of one host.** The core must be expressible on Go,
JavaScript, and the JVM at once. JavaScript has no integers, no structs, and no int64; the JVM
has no unsigned types, no `goto`, and no tail calls; Go restricts `goto` and forbids pointer
arithmetic. If a proposed core feature only works in one of them, it is not a core feature.

**Never introduce boxing or hidden allocation into the core.** This is what killed the
predecessor project — see section 2 of the design direction. Boxing in the substrate sets a
performance ceiling for every target at once, and no host optimizer can undo it.

**A program could not construct data until 2026-08-15** — every gauntlet program took its arrays
as parameters, and `main` takes none, so programs could only print constants. The fix is one
primitive, `make-vec`, wrapped by `num/vec.materialize`: build with the delayed representation,
which fuses, and **materialize only at a boundary**. Materializing in the interior costs the 13×
the stencil benchmark measured, and [that cost is the point](docs/spec/construction.md).

**Recursion is not in the language** — [ADR 0014](docs/decisions/0014-recursion-is-not-in-the-language.md).
It reduced correctly and no backend emitted it, so `oro` accepted programs `build` refused; it is
now an error, checked per-target before reduction by `Env.CheckProgram`. Emitting it would ship the
first construct that looks Tier 1 and is not — stack depth differs by orders of magnitude across
Go, the JVM and JS, and none of them guarantees tail calls. Iteration is `fold-range`. **TCO is
moot** until a while-shaped loop primitive exists, which is also recursion's own prerequisite.

**A JSON TOKENISER RUNS WITHOUT RECURSION ON ALL FOUR TARGETS** —
[json-2026-08-26](gauntlet/results/json-2026-08-26.md),
[examples/json/tokenize.oro](examples/json/tokenize.oro). Nesting to an input-decided depth, an
explicit stack in a `build` buffer, `again` as the jump — 120 lines, **no new term kind, no new
rule, no new primitive, and no target declares anything**. It settles the **control** half of *"recursive
data is a flat table plus indices"* and deliberately not the **value** half: this produces counts,
not a tree, and building the tree as a flat node table is named rather than assumed. **The best
argument for ADR 0014 found so far is not about speed**: `(set stk sp …)` carries `sp < cap` and
nothing can discharge it, so **the compiler makes the parser say what happens when the nesting is
deeper than the stack** — a recursive-descent parser has the same limit, the C stack, and is never
asked. The explicit stack does not create the limit, it makes it VISIBLE.

**AND THE TREE IS BUILT TOO — the VALUE half** —
[json-tree-2026-08-26](gauntlet/results/json-tree-2026-08-26.md),
[examples/json/tree.oro](examples/json/tree.oro). A flat node table, stride 4, tag/val/kid/sib, with
**node 0 as the `none` sentinel holding the header** — parsed, linked, and then **WALKED**, because
building a tree and never traversing it would prove nothing. Four documents, answers computed by
hand before running and reproduced exactly, on Go/JS/Java in the suite and on x86 by hand. 215
lines, no new term kind, no new rule, no new primitive, no target declares anything. **`build`
zero-fills, so 0 is `none` and `kid`/`sib` never need initialising** — a one-line note in
tables.md §14.3 doing load-bearing work. **Linking is exactly what a recursive parser gets for
free**: it RETURNS its subtree, we have to POINT at it. And **ADR 0015 chose the shape and chose
better**: `again` may not go under an `if`, so five token classes became ONE `again` with
`let`-computed arguments rather than five clauses repeating the link — which was not the first
thing tried.

**AND THE FLAT TABLE IS A *GO* FACT** —
[json-tree-bench-2026-08-26](gauntlet/results/json-tree-bench-2026-08-26.md), and this refutes
something this repository has been repeating. data-structures.md says *"recursive data is a flat
table plus indices, 2.02× faster on irregular access"* and ADR 0014 leans on it. Measured on one
program across three hosts, flat against recursive descent into linked objects: **Go 2.52× for
flat, JavaScript 1.22× for flat, and the JVM 1.24× for RECURSIVE — flat LOSES there.** The JVM
bump-allocates in a TLAB, its young collector pays for survivors and every node here dies, C2
scalar-replaces what does not escape, and our 64-bit `int` makes an emitted node **32 bytes against
a `Node`'s 24** — the flat form is *larger* than the boxed one, which is not true on Go. That is
ADR 0008 landing on a DECISION rather than a primitive: *flat beats pointers* was a measurement on
one host that had become a principle.

**Our code generation is at parity where the shape is held fixed** — **Go 1.00×** against a
hand-written clamped flat table (and **1.88× FASTER** than idiomatic recursive descent, 0 allocs
against 443), **JavaScript 1.06×**, **Java 1.22×**. Java's 1.80× against idiomatic recursion
decomposes into three separable costs and none is "the emitter is bad": the representation loses on
that host (1.24×), our element width costs 1.19×, codegen plus clamps plus indextype's `(int)` casts
cost 1.22×.

**CLAMPING COST 1.35×, so the compiler learned to PROVE instead.** A clamp is not a branch — it is a
data dependency in the address computation, which is why the tokeniser's three never-taken compares
cost nothing and this cost a third. The reason every index was clamped turned out to be a **missing
inference, not a missing fact**: `entails` matched a fact to a goal by requiring **identical
coefficients**, so a known `sp >= 1` could not discharge `2*sp - 1 >= 0`. **One Farkas multiplier** —
scale a fact by a positive integer — fixes it, and **a stride is exactly the shape it missed**, which
is why nothing found it before: `(go.* 4 k)` has a coefficient the guard bounding `k` does not, and
no program here had a strided index until a node table. It took the tree from **110 undischarged
obligations to 40** and **1.12× faster**, and changes nothing on the sieve, the tokeniser, `dot` or
the stencil — the honest scope. **Fewer clamps left FEWER undischarged obligations**, because a clamp
hides the fact instead of establishing it.

**And 0 allocations means the number is the number.** Measured alone the recursive Go version is
stable at 11,370 ns; measured in one process with the others it ran **19,168 to 36,495** — a 1.9×
spread on identical work, because 443 allocations put it at the mercy of the rest of the heap. The
flat versions do not move. On a host where the timing alone does not settle it, that is the
strongest remaining argument for the flat table.

**So the superseding ADR general-purpose.md called owed is NOT owed on the grounds it gave** — that
argument was *"a JSON parser, a DOM walk and a recursive-descent parser all recurse"*, and two of
the three now run without recursion. **What is unsettled is ERGONOMICS**, which is a different
argument and has not been made with a measurement. And **termination got WORSE — 0 of 6 loops**,
against the tokeniser's 12 of 16, because every loop's progress now depends on a link read out of a
table. That is the clearest argument yet for octagons.

**Five more bugs, and the first two are SILENT WRONG ANSWERS.** A **call-by-need binding took a
definition's name** — when β declines to substitute it puts the parameter's NAME back in the body,
which is safe for imports because `resolve` qualifies them and unsafe in the MAIN module where
`qualify("", n)` is bare; one occurrence compiled correctly and two returned the definition.
**`PostVars` hoisted an update out of the `let` that bound it** — ADR 0015 permits `again` under a
`let` and the post clause sits outside every binder the body opened. `any` failed `CheckAgainstSig`
(the **third** site today that had `compatible` and did not call it). And two on x86: a store with
three spilled operands, and the template path **enforcing the wrong constraint** — the rule is one
memory operand per INSTRUCTION, not two spilled operands per template, so `mov rcx, [rsp+56]` was
being refused for no reason.

**AND THE TREE IS BUILT TOO — the VALUE half** —
[json-tree-2026-08-26](gauntlet/results/json-tree-2026-08-26.md),
[examples/json/tree.oro](examples/json/tree.oro). A flat node table, stride 4, tag/val/kid/sib, with
**node 0 as the `none` sentinel holding the header** — parsed, linked, and then **WALKED**, because
building a tree and never traversing it would prove nothing. Four documents, answers computed by
hand before running and reproduced exactly, on Go/JS/Java in the suite and on x86 by hand. 112 lines
of code, no new term kind, no new rule, no new primitive, no target declares anything. **`build`
zero-fills, so 0 is `none` and `kid`/`sib` never need initialising** — a guarantee four targets kept
by four unrelated mechanisms with **nothing specifying it**, which is `split-words`'s shape exactly;
now [tables.md §14.3](docs/spec/tables.md) plus a conformance case. **Linking is exactly what a
recursive parser gets for free**: it RETURNS its subtree, we have to POINT at it — one extra word
per depth level, which is *smaller* than a stack frame. And **ADR 0015 chose the shape and chose
better**: `again` may not go under an `if`, so three scalar kinds became ONE clause with
`let`-computed tag and index rather than three copies of the link expression.

**The bug of the day is the differential suite's whole argument in one line.** The Java and
JavaScript loop emitters both had Go's post clause verbatim — `for (;; a, b = x, y)`. That is
simultaneous tuple assignment on Go, a **syntax error** on Java (a comma list of *statements*), and
on JavaScript the **comma operator** — evaluate `a`, assign `b = x`, discard `y`. So the same
emitted shape was a compile error on one host and a **silent wrong answer** on another, which
returned `3000081` where the other three returned `3030081`. **Latent for months because one hoisted
variable emits `a = x`, correct everywhere**, and no program had two until this walk's `seen` and
`steps`. The sequential fix is safe for a reason that already existed: `PostVars` refuses any update
reading a loop variable other than its own — written to make the *Go* hoist correct.

**Termination is where the honest limit is.** The walk cannot be proven from its structure, because
progress depends on links read out of a table and nothing can know a table is acyclic — so it
carries an explicit trip bound, worth **16 of 20 loops against 12**. The program is **408 of 448**
integer operations bounded against the tokeniser's 80 of 124, because clamped addressing both
discharges the index obligations and bounds the arithmetic.

**So the superseding ADR general-purpose.md called owed is NOT owed on the grounds it gave** — that
argument was *"a JSON parser, a DOM walk and a recursive-descent parser all recurse"*, and two of
the three now run without recursion. **What is unsettled is ERGONOMICS**, which is a different
argument and has not been made with a measurement: 112 lines against maybe 60 with recursion, and
three constructs here — the clamps, the trip bound, the last-child slot — exist because of what the
language refuses. Two things that ARE settled: **two live buffers work** (a `build` inside a
`build`, both threaded through one loop, linearity and termination intact), and **a buffer can
outlive the loop that filled it** — frozen on the way out and read back as an ordinary `array`,
which is ADR 0018's freeze on the first program that needed it.

**CLAUSE ORDER ONCE CHANGED WHAT COULD BE PROVED, AND IT WAS THE FIXPOINT BUG** — json-2026-08-26 §4
recorded the same program with one clause moved going from **80 of 124** and **12 of 16** to **36 and
0**, and called the cause *not isolated* because four hand-written reductions all proved. **Moved back
2026-08-28 it makes NO difference in either position** — 165/165 and 20/20 with the declared `where`,
143/165 and 16/20 without, either way
([frozen-2026-08-28](gauntlet/results/frozen-2026-08-28.md) §1). The cause was almost certainly
`restore` installing its snapshot by reference: the environment leaving an `if` carried `¬c`, and
**which guard's negation leaked into which later clause is decided by clause order**. That is why the
reductions could not find it — they were looking for a precision bug in a soundness bug's shadow. So
*a meaning-preserving edit can lose a termination proof* is **withdrawn**, and with it one of the three
standing arguments for octagons.

**AND IT IS AT PARITY, WHICH IS THE FIRST TIME BRANCHY CODE HAS BEEN MEASURED** —
[jsontok-2026-08-26](gauntlet/results/jsontok-2026-08-26.md). **Go 0.96x, JavaScript 1.02x, Java
1.20x** against hand-written tokenisers, and the tokeniser is now **gauntlet program 7**, added
because the first six are all the same shape — countable loops over arrays, which every host
optimises best and which the emitter had been tuned against for five months. `make([]int, 32) does
not escape`, so **ADR 0018's buffer is stack-allocated**, zero allocs, and it keeps **2** bounds
checks where hand-written Go keeps 5. **Java's 1.20x is the INDEX TYPE and that is isolated, not
assumed**: the same program hand-written with a `long` index measures 9,156 ns against our 9,400,
so the emitter is at **1.03x of the shape it emits** and the whole gap is
[indextype-2026-08-25](gauntlet/results/indextype-2026-08-25.md)'s narrowing failing to fire —
which that result predicted, because our `i` is assigned a scanner's return value rather than
stepping by +1. The fix is a PLATFORM fact, not an inferred range: a Java array holds at most
2^31-1 elements.

**Three things that run counter to the guesses.** The **64-bit `int` is nearly free** — Go's
`[]int` form is 1.05x the `[]byte` one, because the loop is branch-bound and the eight-times-larger
input hides; that is a **third regime** beside checkcost-2026-08-19's arithmetic-bound 4.54x and
memory-bound 1.23x, and the lesson repeats that a cost behaves the way a saving does. The
**refinement layer's three extra compares cost nothing measurable**, because a never-taken branch
is what a predictor is best at. And **a string-based tokeniser is 1.89x slower than an array-based
one on V8**, which lands on the strings work rather than on us.

**A benchmark-method error was caught on the way, the third in this repository.** The first JS run
said **0.68x** — the best number in the project's history — and it was measuring a closure: the
hand-written reference read each byte through `(i) => s.charCodeAt(i)`, an indirect call per byte
worth **1.5x**. Removing it moved the hand-written side from 12,320 ns to 8,314 and left ours
still. Every surprising JavaScript result here has been a method error at least once; the rule that
catches them is to **make a suspicious result explain itself before recording it**.

**Four bugs came with it, and what they share is that the PROGRAM was bigger, not that a construct
was new** — every construct here was already covered by the differential suite, which passed.
`typeOf` of a `build` assumed the body hands the buffer back (this is the first program whose buffer
is **scratch** — a stack, dead at the boundary, returning a count); `any` demanded something in the
conditional and loop-exit checks, which had a `compatible` helper and did not use it, so a running
maximum was a type error on the host that declares everything `any` on purpose; and x86 emitted a
**memory-to-memory `mov`**, because no program before had enough live values to spill the
destination of a `len`. **That is the argument for having both a construct suite and a program.**

**Iteration is `fold-range` and `loop`** — [ADR 0015](docs/decisions/0015-loop-and-again.md).
`(loop ((x z)…) c e … else e)` with `(again a…)` gives n loop variables with **no product**, early
exit at parity with hand-written code, and unbounded iteration. `again` may be a clause body or sit
under a `let`, never under an `if` — *let binds, if branches* — so the clause list is the loop's
whole control flow. `again` is a jump, not a call, so ADR 0014 stands. **Termination is now a
program property**, computed like portability, not a language guarantee.

**Never add unstructured control flow.** Structured only: `if`, `loop`, `break n`, `return`.
Recovering structure from `goto` is a hard algorithm and three of the initial targets cannot
express `goto` at all.

**THE TYPE ALGEBRA IS ARGUED** — [type-algebra.md](docs/type-algebra.md), on hamza's *"this looks a
lot like algebraic types, let's do it right"* and *"make `match` like `loop` and `cond`, with
`again` being match again"*. **Take the semiring — `+`, `×`, `0`, `1` — and the exponential; refuse
fixed points, subtyping and untagged unions.** What it would be, precisely: **a bicartesian closed
category WITHOUT fixed points, plus one indexed family (tables) whose index set is sized at run
time.** Three laws are load-bearing: associativity/commutativity hold **up to isomorphism** so flat
n-ary products and sums are legitimate; **distributivity IS case-of-case**; and `A^(B+C) ≅ A^B × A^C`
says a function from a sum is a list of clauses, so `match` falls out of the algebra.

**And every operation is free at the STATIC level and priced at the DYNAMIC level** — product,
table, sum, function, fourth time — which is the two-level language stated in the type system, and
the reason "the whole machinery" is affordable: it is free where it is used to *think* and priced
where it is used to *run*.

**Three exclusions, argued.** **μ** — refused, and the alternative is *measured*: recursive data is
a flat table plus indices, 2.02× faster on irregular access. **Subtyping** — refused because we
already have the decidable half: `{i | 0 ≤ i < n} ⊆ int` is bounded subtyping decided in QF-LIA, and
generalising it is Pierce's undecidable F<:. **Untagged unions** — `success | fail` is a coproduct
and wanted; `float | int` as a *union* is idempotent, is not a coproduct, needs a runtime type test
three of four hosts lack, and needs subtyping. **Tagged in the semantics, niche-encoded in the
representation** is Rust's answer and costs nothing. Products anonymous, **sums named** — `(inl 3.0)`
does not determine its type, which is why every language without runtime types went nominal.

**`match` IS BUILT** — [match.md](docs/spec/match.md), 2026-08-22. Reader sugar over `loop`, about
120 lines, **zero reduction rules and zero term kinds**, no change to any backend. Three things the
build taught that the argument had not. **`when` guards are not optional**: ADR 0015 forbids `again`
under an `if`, so a condition patterns cannot express has nowhere to go — not the guard, not the
body — and without `(when c)`, `match` is *strictly weaker than the `loop` it desugars to*. **A
bare-name scrutinee must BECOME the loop variable**: with fresh variables, a clause body reading `i`
sees the value the loop *started from* while `again` advances a hidden one, so every iteration after
the first reads a stale value and the program looks right. And a name pattern is a **rename, not a
`let`** — safe without capture analysis because `Fn` has already closed every inner binder, so a
shadowed occurrence is a `KBound`, not a `KName`. It found **a five-month-old JavaScript bug**:
`(loop ((n n)) …)` inside `function f(n)` emitted `let n = n;`, a `SyntaxError` — Go and Java
seeded their fresh-name set from the parameters and JS did not; x86 needed no fix because registers
are positional. `=` is now injected like `if`/`let`/`loop` and resolves to each host's own
equality — **not `==`** (the first reason recorded for that was *false* and is corrected in
match.md §6: `tg.Prims` is keyed by the qualified name, so `js.==` and a bare `==` never collided;
what survives is a weaker legibility argument), and **not the `tag=` it was first built as**, because
a name should say what an operation *is* rather than what it is for, and the honesty a narrow name
buys is better bought by a refusal that can explain itself. Float and string patterns are refused (no portable equality; NaN is not an equivalence
relation), and so is a repeated name in one clause.

**The older framing, kept because it is the argument the build tested**: `match` IS `loop`, and it is SUGAR.** `(match (e₁ … eₙ) pats body … else body)` desugars to
`(loop ((v₁ e₁) …) guard body … else body)` with pattern bindings as `let`s — and **`again` under a
`let` already works** (ADR 0015 permits exactly that, a rule written for another reason), verified
today. **Zero reduction rules, zero term kinds**, joining `let`/`seq`/`and`/`cond` as sugar that
erases; `if` stays primitive because it is what `match` desugars into. It is Erlang's clause-head
shape with a jump instead of a tail call, it is the state machine a parser or event loop is, and it
makes the refinement checker **stronger** — a clause gives the tag *and* narrows later clauses to a
finite remaining set, where a boolean chain gives only a negated predicate. **Flat patterns, because
our data is flat.**

**SUMS ARE BUILT** — [sums.md](docs/spec/sums.md), 2026-08-22, closed/finite/non-recursive, **zero
new term kinds** and **no target declares any of it**. The design is one sentence: **a sum is Σ, so
its value is a tag and a payload, which is a PRODUCT — and the product was already built on all four
targets**. Go's own `(T, error)` idiom IS that shape, so the host with no sum type needed nothing.
A declaration generates **definitions** (`ok = (fn (#p) (fn (#x) (#x 0 #p)))`, `ok#tag = 0`), so
qualification, imports, δ and the occurrence counter all apply without learning sums exist; `case`
expands in **`Load`** rather than the reader, because the reader sees one file and an error type is
declared in another. **Both levels are free**: static `(case (ok n) …)` leaves nothing, and dynamic
`(case (if c (ok n) (err 0)) …)` reduces to `(if c … …)` — no tag, no closure, no allocation, no
dispatch the `if` was not already doing. Measured **1.00× against hand-written Go with 0 allocs**
([sums-2026-08-22](gauntlet/results/sums-2026-08-22.md)).

**Four things the build taught that the argument had not.** **The static sum needed `=` to fold** —
the Church encoding reduced away exactly as predicted and then left `(if (= 0 0) …)` behind, a
tautological test the two-level language says should not exist; the fix is the first entry in
tables.md's constant-folding table, and it is **integers only** because ADR 0009 permits folding
only where compile time and run time provably agree. **Case-of-case needs a `let` companion**, and
only a *nested* test finds it: β itself puts a `let` between a constructor and its eliminator, so
the rule is better stated as **push an eliminator through anything β can leave in operator
position** — exactly `if` and `let`. **A function returning a sum returns from SEVERAL PLACES**, so
each backend gained `multiTail` walking those same two forms to the leaves — which is also V8's
1.31× tail-return finding arriving a second time from a different direction. And **exhaustiveness
REMOVES a branch rather than adding one**: a sum is closed and finite, so the last clause needs no
test — a better argument for checking it than the one that motivated it. Also: the refinement layer
had to be taught the language's `=`, since `b != 0` in an ok-branch comes from negating it.

**Case-of-case was measured before it shipped**, as the build order demanded: across **184
residuals** — every example on all four targets — it changes **nothing**, because it fires only
where a sum is eliminated. Still open and named rather than guessed: the **niche encoding**
(`(option ptr)`, `none-is 0`), **`try` as bind**, and **`match` on a sum** — which is not free,
because `(again (err 3))` must split a sum into two loop-variable updates and doing that for an
opaque sum puts `again` under a lambda.

**The research that produced it, kept**: **SUMS ARE RESEARCHED and not decided** — [sums-research.md](docs/sums-research.md). Three
requirements converge: errors, Win32 contracts (`_Ret_maybenull_`, `_Success_`), and **dispatch,
because a tagged union is what replaces the closure**. Four findings. **A sum is Σ over a finite
index set — the exact dual of the table's Π** — which is why a Π can be given by a *rule* and store
nothing while a Σ must carry *which*: **the tag is information the caller does not have, and it has
to be transmitted.** And **there is no negative sum**: a product has `⊗` and `&` and we took the
free one; a coproduct has only `⊕`. **The Church-encoded sum already works when the tag is STATIC**
— `((inl x) f g)` reduces to `(f x)` — and the dynamic case is stuck at `((if c A B) F G)`, which
**case-of-case** unsticks completely, no closure and no tag: Prawitz's commuting conversion, GHC's
case-of-case, whose blow-up hazard is answered by join points and **`again` is one**. q5b named this
rule and rejected it for collections; for sums it is load-bearing. And **the niche optimisation is
what host APIs already do** — NULL, −1, HRESULT's sign bit are all sums encoded in the payload's own
value space — so declaring `(option ptr)` *names* the representation the API already uses rather
than adding one. Refinements need nothing new: `emit/refine.go`'s `clauses` already case-splits on
`if`, and a `case` on a tag is the same operation. Recursive sums stay **rejected** — a JSON node is
a *non-recursive* sum plus indices into a table. Go is the hardest host here, not x86, because Go
has no sum type.

**THE MAP OF DECIDABILITY IS DRAWN** — [decidability-map.md](docs/decidability-map.md), so future
decisions can be LOCATED rather than argued from scratch. Four questions get conflated and have four
answers: reduction terminating is **undecidable** (quarantined to the static level, bounded by fuel,
where "I gave up" is an honest compile error); the residual type-checking is **trivially decidable**
because staging made it monomorphic, first-order and closed — **we never pay Hindley-Milner's
DEXPTIME or meet System F's undecidable inference, because there is nothing polymorphic left**;
obligations are **QF-LIA, NP-complete**, solved by an incomplete but total procedure; termination is
**size-change, PSPACE**. Two counterintuitive facts on the map: nonlinear arithmetic is
**undecidable over ℤ** (Hilbert's 10th) and **decidable over ℝ** (Tarski), and adding quantifiers to
linear integer arithmetic keeps it decidable and makes it useless (2-EXP lower bound) — **the
frontier is feasibility, not decidability**. The algebra is **two lattices**: abstract
interpretation for *inference* (intervals ⊑ zones ⊑ octagons ⊑ polyhedra, with widening/narrowing)
and a logical theory for *proof*, and our one real mismatch is that **intervals are non-relational
while every interesting obligation is relational** — `emit/refine.go` hand-rolls a weak relational
layer, and **octagons are the principled version at O(n³)**, the highest-value move available. For
nonlinearity: **declared sound axioms, never search** — and per Shen, those axioms should be
DECLARED not hardcoded in Go, which is this project's own rule one level up. Four things given up,
each a published limit: program equivalence, general sortedness, products of two unknowns, and
termination measures that are not size-changes.

**THIS IS A GENERAL-PURPOSE LANGUAGE** — [general-purpose.md](docs/general-purpose.md). hamza,
answering the question lowstar-lessons.md asked: *apps on Windows and Android, websites in the
browser, backends in the cloud.* The four targets were application platforms all along, and the
guess that this is "a systems and numeric language" was a read of the benchmarks rather than the
intent. **The Low\* lesson inverts**: Low\* succeeded by picking a layer where its restrictions are
advantages, so general purpose means **the restrictions must be PAID FOR rather than enjoyed**.
What moves from *deferred* to *owed*: **recursion** — a JSON parser, a DOM walk and a
recursive-descent parser all recurse to a depth the input decides, so ADR 0014 needs a superseding
ADR and the shape of the answer is **ADR 0012's**, name the portable depth and say the rest is the
target's; **sums** — every host API can fail and a refinement cannot discharge a network error;
then **strings**, **growable collections** and **maps**.

**And the type system must reason about the TARGET** — *"I should be able to express a windows api
call so I can check my program in oroboros."* Today `(prim VirtualAlloc (int) ptr expr …)` says a
name, argument types, a result type and a template, and does not say that the size must be
positive, that the result may be NULL, that it points to `size` writable bytes, or that it must
reach `VirtualFree`. **SAL is the field-tested answer** — Microsoft annotated Win32 for `/analyze`,
and most of SAL is buffers and sizes, which our refinements already do decidably; the rest is
nullability, success and acquire/release. **Five of the eight requirements already exist.** Missing:
postconditions naming the result, a fallible result, and a surface for linear handles — and the
third is ADR 0018's buffer generalised, which is the best evidence a mechanism is right. An effect
system is NOT obviously needed and ADR 0010 holds.

**F\*/Low\* has been cited five times as an authority and is now examined** —
[lowstar-lessons.md](docs/lowstar-lessons.md). Nine lessons: five confirm decisions, three
challenge them, one names something we get free. **The restriction IS the mechanism** — Low\* is
the *subset* of F\* that has a C meaning and KaRaMeL **refuses** the rest, which is our emitter's
refusal in a better register. **Erasure must be total AND checkable** — F\* can *tell* you
something is ghost; we find out when the emitter fails. **Do not add SMT**: F\*'s best-known
practical problem is proof instability, and our deliberately incomplete linear-arithmetic
procedure that *reports* rather than assumes is the right side of that trade. **Ours is cheaper
than theirs in one specific way**: HACL\* needs a *proof* that its implementation refines its
spec, and we need none because reduction **produces** the implementation *from* the spec.
**LINEARITY IS FRAMING done by the type system** — Low\*'s HyperStack needs a `modifies` clause per
function and it is HACL\*'s largest proof burden; ADR 0018's buffer has a `modifies` set that is
*syntactically* the buffer, which is ADR 0018's best argument and ADR 0018 does not make it.
Challenged: Low\* keeps recursion and real data types and still hits hand-written C, so **our
minimalism is stricter than speed requires and the reason is FOUR HOSTS, not performance**; and
**we cannot write a fast implementation and prove it equivalent to a clean one**, which HACL\* does
routinely — the seed of the answer is `sig`'s existing *claim checked in two directions*, and
generalising it is the one genuinely new capability on the list. And Low\* targets **the systems
layer** on purpose, which is the crossroad question restated: *are we trying to be general-purpose?*

**This is a TWO-LEVEL language and had not said so** —
[closures-direction.md](docs/closures-direction.md). We already have closures and they already
work: a **Church-encoded list** — `(cons x xs) = (fn (c n) (c x (xs c n)))` — compiles today and
reduces to `1 + (2 + (3 + (k + 0)))`, which is the free monoid with `foldr`, needing no recursion
because `cons` nests closures at *construction*. So "closures are refused" is the wrong sentence;
the true one is **"a closure may not survive staging"**, and the boundary is exactly a *dynamic*
length: a list sized by a runtime value leaves a closure in the residual and is refused. The static
level is unrestricted higher-order (Zig's `comptime`, Terra, MetaML, Nielson & Nielson 1992,
binding-time analysis); the dynamic level is first-order tables and loops.

**Letting closures survive is REFUSED, and the reasons are ranked**: on x86 a closure is a heap
environment and an indirect call we would have to ship, which is a **runtime** against requirement
6; environments are **hidden allocation**, the predecessor's cause of death; and **closures plus
buffers give recursion via Landin's knot**, which kills size-change termination and ADR 0015's
"termination is a computed property". Also `(a i)` stops being unambiguous, every analysis loses
its call graph, and ADR 0018 would need rank-2 types. Against that, the real gain is a
*manufactured* callback and dispatch tables. **With closures the design converges on an ML with
four backends**, and the competition becomes MLton. Three cheap things follow: say the levels in
the diagnostics, **relax ADR 0014 to refuse recursion that SURVIVES rather than recursion that is
written**, and give the static level a real library — lists, maps, trees, all free.

**Refusing closures costs FUNCTION VALUES, not host APIs** — [callbacks.md](docs/spec/callbacks.md).
Three tiers, and only the third is a closure. **Tier 1** — a lambda written at the call site, where
the HOST closes over it: `go func(){}`, `defer`, `sort.Slice`, `addEventListener`, `setTimeout`,
`Runnable`. The lambda never becomes a value in our residual; it sits in a structural position the
backend already reads positionally, and Go's/V8's/the JVM's own closure does the capture, at parity
by construction. **Tier 2** — a function pointer with no free variables, which all four hosts have
and which we **refuse today with the wrong message**: `(def twice (fn (x) (go.* x 2)))` returned as
a value says *"this is an escaping closure"* and it is not one. And **the Win32 API is designed for
a language without closures** — `EnumWindows(fn, LPARAM)`, `CreateThread(…, lpParameter)`,
`qsort_s(…, context)` pass the environment explicitly, so the OS API that looks most hostile is the
one best suited. **Tier 3** — a manufactured closure that escapes — stays refused, and ADR 0018
depends on it. What is genuinely lost: dispatch tables and partially-applied callbacks. One hazard
to carry: **a callback body may not capture a buffer**, or two goroutines hold it and ADR 0018's
linearity is gone.

**Closures are not a core primitive.** They belong above the core, lowered by defunctionalization
or explicit environment structs. First-class closures require captured environments, which
require heap allocation.

**Performance claims must be measured, not asserted.** The standard is parity with
hand-written code in the target language, checked by benchmark against a hand-written
equivalent. "Should be fast" is not a result.

When adding to the gauntlet, **carry both forms** — the one expected to win and the one expected
to lose. Five beliefs were refuted in the first run only because the losing form was there to
measure. And check the compiler's own decisions, not just the clock:
`go build -gcflags="-m -m"` and `-gcflags="-d=ssa/check_bce/debug=1"` were each decisive where
timings were ambiguous.

Benchmarks in [gauntlet/results/](gauntlet/results/) were taken on a hybrid P/E-core laptop with
a ~15% noise floor. Do not rest a decision on a smaller margin than that.

## Adding to the language

**Nothing goes in without a specification saying how it behaves on every target.** String
literals were added without one and [docs/spec/strings.md](docs/spec/strings.md) is the
correction — write the spec first.

The test for a proposed addition is not "is it useful":

1. What does it mean, independently of any target?
2. What does each target do with it, and do they agree?
3. If they disagree, is the disagreement **observable**? If so it is Tier 2 and carries no
   portability claim.

Every primitive is classified in [docs/spec/primitives.md](docs/spec/primitives.md). Two are
Tier 1 only *within bounds* — `aindex` and `sat`, because an out-of-range read panics on Go, throws
on Java, and **silently returns `undefined`** on JS. A Tier 1 name without a conformance suite is
decoration: `split-words` passed every check for two months while returning different answers on
different targets. The suite is [gauntlet/conformance/](gauntlet/conformance/).

`length` fails (3) — `"🙂"` is 4 on Go, 2 on JS and Java, 1 counting characters — which is why it
is not in the core. Strings pass only by having almost no operations.

**The current state of the language is [docs/spec/state.md](docs/spec/state.md)**, read off the
code rather than from memory. Seven term kinds, five top-level forms, three reduction rules, two
parameters.

**Booleans are in the language and the connectives are sugar** —
[ADR 0017](docs/decisions/0017-booleans-are-in-the-language.md),
[booleans.md](docs/spec/booleans.md). `bool` is data, `if` is its eliminator, and
`and`/`or`/`not`/`cond` erase in the reader — Scheme's answer, ML's answer, and McCarthy's reason
from 1960: **a conditional cannot be a function**. Each backend puts the host's `&&` back, so
nothing is lowered further than the target requires, and that was
[measured](gauntlet/results/and-form-2026-08-19.md) rather than argued. A target declares zero
boolean names now, and declaring one is an error. Two consequences to carry: `(if true a b) → a` is
the **only** evaluation reduction performs, which gives conditional compilation with no
preprocessor; and the strict branchless operators survive as host names — `x64.andb` — because Ada
kept `and` beside `and then` for a measurable reason.

**A function may return SEVERAL VALUES, and it is not a tuple** —
[values.md](docs/spec/values.md), [multiresult-2026-08-22](gauntlet/results/multiresult-2026-08-22.md).
`(values a b)` is **reader sugar** for `(fn (#k) (#k a b))` — the negative product, linear logic's
`&` — so its β law IS β and the reducer needed **nothing**: three rules before, three after. Consumed
in the same reduction it vanishes; crossing a boundary it becomes the target's own form, and
**`(sig f (…) (int int))` is what disambiguates a product from an escaping closure**. Measured:
**Go 0.99x with 0 allocs, Java 0.97x**. **NO TARGET DECLARES IT** — the first attempt carried a
`(multi-return …)` declaration and refused on Java and windows, and was reverted for making a core
construct declinable. What that dodge was hiding turned out to be small: **Java gets a `record`
shared by result SHAPE**, and C2 scalar-replaces it **across a class boundary** — 1.01x against no
product at all, which was the unknown this build existed to answer; **windows gets `rax`/`rdx`**,
mirroring Win64's argument convention, ours because both sides of the call are ours. And the build
found two things argument had not. **x86 needs TWO PASSES**, because placing a result into `rax` as
it is computed is clobbered by the next `idiv`. And **JavaScript returns an OBJECT, not an array** —
the first version emitted an array because `const [a, b] = f()` reads well, which is clarity chosen
over requirement 5 and was argued from a measurement of a *different shape*. Measured properly:
object **5,164 ns** against array **8,348** when the caller reads a property, identical when it
destructures — so the object is better or equal in both. And the larger finding is about the
**caller**: `const {f0, f1} = f(x)` costs *nothing* because V8 scalar-replaces the object, while
`const p = f(x); … p.f0 …` keeps the allocation at **5.4x**.

**Effects are a side condition on β, not a feature** — [docs/spec/effects.md](docs/spec/effects.md).
Purity is one declared bit per primitive, defaulting to *impure* so that a target author's omission
costs speed rather than correctness. An impure argument is never substituted; it is let-bound at
the application site, whatever its occurrence count, which denies contraction, weakening and
exchange in that order. There are no effect types, no monads, and no linear types on values, and
adding any of them should be argued against this first. `seq` is sugar for a β-redex with an unused
binder and works *only* because weakening is denied.

**There is now a type checker, and it is not in the language** —
[docs/spec/types.md](docs/spec/types.md). It runs on the **residual**, before emission, which is
cheap because reduction has already made the term monomorphic, first-order and closed. One checker
serves all three targets, including JavaScript, which previously compiled
`(f64.add "hello" 1.0)` into a program that printed `hello1`. `(sig name ((p type)…) result)` on a module export is a **claim checked in two directions**:
against the definition's residual, and against any target that provides the name *natively*. The
second is the job no host compiler can do, because the two implementations live on different
targets and no single compiler sees both. Parameters are **named** because a refinement attaches to a name.

**Refinements are built** — [docs/spec/refinements.md](docs/spec/refinements.md). `aindex` carries
`(where (and (<= 0 i) (< i (alen v))))`, and the obligation is discharged at every call site from
facts collected out of loop bounds. The fragment is linear integer arithmetic with a deliberately
incomplete decision procedure; **an undischarged obligation is reported, never assumed**. This
closed the first of the two holes shaped like a refinement, and **found a real latent bug in `dot`
and `centroid`**, which index two arrays under one loop bound. Still open: the integer range hole
([arithmetic.md §4](docs/spec/arithmetic.md)).

**A `where` means THREE things and nothing said so** — [refinements.md §6b](docs/spec/refinements.md).
On a `prim` it is an obligation discharged at every call site. On an **exported** definition it is a
published contract, *assumed*, because the caller is outside the program. On an **internal** one it
is **dropped** — and what protects the program is that reduction inlines the call, so the body's own
obligations land on the caller's concrete values. That is **stronger** than checking the clause,
because the clause is a conservative summary: a declared `(< n 100)` on a body that needs
`n < len a` would reject a legal `(get a 400)` against a 500-element array. **So enforcing a
definition's `where` would be a regression, not a fix.** The genuine gap is the case inlining cannot
reach — a body that is total and merely *wrong* outside its domain, where nothing fires;
`win/fmt.print-int` prints a blank line for a negative number and is within its rights. That is
SAL's `_Success_` shape and belongs beside the Win32 work.

**What an unproven operation costs is 1.23× to 4.54×, and the shape decides** —
[checkcost-2026-08-19](gauntlet/results/checkcost-2026-08-19.md), the same source compiled twice
differing only in the declared range. Arithmetic-bound: **Go 4.54×, Java 1.52×** — the JVM has
`Math.addExact` as an intrinsic and Go has nothing, which is the §0 spread rule biting at a factor
of three. Memory-bound: **Go 1.23×, windows 1.46×**, because the branch hides behind the cache
miss. **The isolated microbenchmark was wrong in BOTH directions** — the same lesson as
bce-2026-08-15, where a 1.96× win in isolation vanished on memory-bound loops. A cost behaves the
way a saving does, and neither survives being quoted without its condition.

**ELEMENT WIDTH FROM THE RANGE IS BUILT** —
[elemwidth-2026-08-27](gauntlet/results/elemwidth-2026-08-27.md). **A range is a type**:
`(sig tokens ((src (array (int 0 255)))) int)` emits `[]byte` on Go and **`short[]` on Java, because
the JVM's `byte` is SIGNED and 0..255 does not fit it** — and no host fact lives in Go for that. The
target declares `(int-repr LO HI "spelling")` narrowest first and the narrowest one CONTAINING the
range wins; JavaScript declares none and keeps its packed `Array`, which jsontok measured 1.15x
faster than a `Uint8Array` anyway. That is ADR 0003's *"mathematical semantics, machine
representation"* written in the type language five months after it was decided. **A range says what
a value IS; the width belongs to its storage alone** — `ValueType` normalises a range to `int`
everywhere but a table's element slot, or a counter over a byte array would overflow at 255 while
the language says integers do not.

**Go reaches 0.99x of hand-written `[]byte`** — the like-for-like comparison jsontok-2026-08-26 could
not make. **And on Java it works and is NOT what Java's gap was**: `short[]` is now the fastest
hand-written form (7,439 ns against `byte[]`'s 7,744), we moved 1.4%, and we sit at **1.00x of the
shape we emit** with the whole remaining 1.25x being indextype-2026-08-25's `long` index. **Two costs
that looked like one because they were measured together.** The element is a *declaration*; the index
is an *inference*, and it needs the same missing fact the whole precision-integer plan needs — a
postcondition bounding a scanner's result. **Two costs, measured independently, one cause.**

**AND THE WRITE SIDE TOO** — a `build` buffer's range is **INFERRED**, because ADR 0003 says ranges
are declared at boundaries and inferred for LOCALS, and a buffer is a local.
`(set stk sp (if (= (src i) 123) 125 93))` gives `[]byte` on Go, `byte[]` on Java (93..125 fits a
*signed* byte), one byte per element on x86, nothing on JS. **The inference is deliberately
syntactic** — a literal is its own exact range, a conditional joins its branches, a read from an
already-narrowed table carries one, and everything else keeps the host's word. That is a soundness
choice: **a range too narrow truncates on store and is a silent wrong answer**, so only facts exact
by construction are used; the interval domain is a real extension owing its own argument.
**Zero is always an element**, because `build` zero-fills and an unwritten slot reads 0.
**`tree.oro`'s node table correctly stays 64-bit** — it stores node indices no syntactic fact bounds,
and getting that answer right is the feature working.

**Two more bugs, and the differential suite caught the one that mattered.** `BufferElemBytes` took
the **FIRST** store, so the tree's node table — tag 1..5 in one slot, a node index up to 511 in
another — said one byte and truncated every link: windows returned `4030140` where the other three
returned `4040171`. It compiled, ran, and returned a number. And Java's value cast went into **one of
two store paths**, the one a program takes only once its index is narrowed — so it would have failed
on the first real program and passed every test with an index-narrowed loop.

**AND THE ANALYSIS IS NOW TESTED BY A CONTAINMENT HARNESS** —
[containment-2026-08-27](gauntlet/results/containment-2026-08-27.md),
`emit/containment_test.go`. **1,877 randomly generated programs, run concretely**, every integer
operation checked against what the analysis claimed. The property is **γ-soundness**: for every
reachable state and every operation, `⟦e⟧σ ∈ γ(MaxOp)` — **containment, never tightness**, because
a claim too wide costs space and one too narrow is a silent wrong answer.

**It exists because a hand-written soundness test passed for months while the fixpoint was unsound.**
`TestIntervalsNeverOverclaim` only catches what someone thought to write, the differential suite is
**structurally blind** here (every target narrows on the same decision, so they agree and are wrong
together), and two adversarial cases written that week expected a refusal where the analysis was
right — the tests were wrong, not the analysis.

**THE PASS CONDITION, and the harness failed it first.** It must FAIL when the fixpoint bug is put
back — and with `restore` reverted it **passed anyway**. The reason is worth keeping: **every
conditional the first generator made sat in TAIL position**, where the environment after an `if` is
never used again, so the leak was invisible. The bug bites when an `if` is an OPERAND, where what the
analysis believes afterwards is immediately spent on the other operand. With that shape generated it
fails at **seed 15 — claims `-153..765`, program produces 918**. A harness that cannot fail proves
nothing.

**AND BUFFERS ARE COVERED TOO** — the decision that TRUNCATES rather than merely widening. 2,000
generated `build` programs, **1,490 of which get a narrowed element**, every stored value checked
against `ElemType`. The property quantifies over READS and the harness checks WRITES, and that is a
theorem rather than a shortcut: **a slot holds either the zero fill or the most recent `set`**, there
being no third source — `build` is the only allocator, `set` the only store, and ADR 0018's linearity
means nothing else can have written it — so checking the stores and the zero is **sufficient**, not
merely necessary. Both hypotheses are load-bearing and both are checked, because **dropping either is
a real bug shape**.

**Three more bugs reintroduced, three seeds.** The **first-store** rule — `tree.oro`'s node table at
one byte, windows returning `4030140` — fails at **seed 6**, on a program storing 242 and 207 in one
iteration, which is the node table's exact shape found on the sixth program. Dropping the
**`sawOther` guard**, so a buffer narrows on its exact stores and ignores the unanalysable ones,
fails at **seed 10**. Dropping the **zero-fill join** fails at **seed 14** — `int 2 344`, a range
that excludes 0.

**The buffer half needs an ANTI-VACUITY guard the other does not**: refusing to narrow is always
sound, so a harness that only ever saw refusals would pass forever while testing nothing. It fails
unless it watches the compiler COMMIT on at least 200 buffers.

**And one rule random search CANNOT test**: a buffer may not narrow on its own contents. No
counterexample exists to find — a self-reading buffer usually does have a narrow range in fact — so
it is pinned as a POLICY test with a CONTROL, the same program storing a literal, which must narrow.
A test whose passing case and failing case look identical is a test that proves nothing.

**THE INTERVAL FIXPOINT WAS NOT MONOTONE, AND EVERY PROVABILITY NUMBER WAS INFLATED** —
[fixpoint-2026-08-27](gauntlet/results/fixpoint-2026-08-27.md). `restore` installed a snapshot **by
reference**, and what follows a restore is `refine`, which narrows **in place** — so the second
restore undid nothing and the environment leaving an `if` carried `¬c`, a fact true on one path
only. **The property that breaks is monotonicity of the abstract step** `F(c⃗) = z⃗ ⊔ ⨆ ⟦a⃗⟧#(R(c⃗))`,
which is what makes widening converge to a POST-fixpoint and what makes narrowing's
`within(next, cur)` test legitimate. Measured non-monotone: `[0,0] ⊑ [0,2]` and yet `F([0,0])[i] =
[0,2]` while `F([0,2])[i] = [0,0]`, so `i` and `sp` settled at their INITIAL values. **The fix is
that `restore` installs a copy** — one edit, both sites, and the discipline cannot be got wrong
again.

**The unsound intervals were NARROW, so they made operations look bounded that were not.** The
tokeniser's *"100% of integer operations bounded, 20 of 20 loops"* was **86.7% and 16 of 20**; the
tree's 91.3% was 88.9%. And **ADR 0008 lands again**: `elemwidth`'s interval-derived buffer narrowing
is **withdrawn** — `tree.oro`'s node table is `[]int`, not `[]uint16`, because in the `build` lambda
alone `src` is free and the stores genuinely cannot be bounded. The old `int 0 512` was a *sound
range reached unsoundly*; it contained 511, so nothing shipped wrong, and that was luck. **The
syntactic narrowing stands** — literals never needed the fixpoint, so the tokeniser's stack is still
`[]byte`.

**And it CORRECTS the central finding of precision-integers.md.** That document recorded declaring
ranges as *"changing NOTHING"* on the two parsers and called it a qualitatively different failure.
That was the bug. With the fixpoint sound, **one declared range takes the tokeniser from 86.7% to
100% and 16 of 20 loops to 20 of 20** — exactly what intervals-2026-08-19 predicted, so the plan is
in better shape than the research concluded.

**Two more gaps found by re-measuring, and neither is subtle.** **`len` was not recognised**: the
refinement and interval layers knew `alen` and `slen` — the RETIRED portable layer's names — and not
tables.md's structural `len`, which every program written since uses, so `(go.>= i (len src))`
bounded nothing. And **an exactly-known length was thrown away**: reduction INLINES, and call-by-need
*let-binds* an argument used more than once, so a literal document reaches the tokeniser as
`(let (array …) (fn (src) … (len src) …))` — exact at the binding, lost one line later. Carrying it
through took the tokeniser from **33.3% to 86.7%** and the tree from 79.1% to 88.9%.

**AND BOTH GAPS THEN CLOSED against the fixed analysis.** **The derived step through a `let` and an
`if`** — ADR 0015 permits `again` under a `let`, and the tree binds ONE name to a choice of THREE
scanners — takes `tree.oro` from **17 of 25 loops to 21 of 25**, and to **25 of 25** with a range
declared. It lands as a strict **fallback** (a lower bound used where an exact step exists turns
`[1,1]` into `[1,+inf)`) and it gives the arc while **withholding the measure**, keeping the position
out of `tripCount`. **A read carrying its element range** is proved from the zero-fill guarantee and
is **non-circular by construction** — only a DECLARED range or a buffer's SYNTACTIC one, never
`BufferRange`, which would be a fixpoint feeding itself. It **buys nothing measurable on this
corpus**, the second honest negative this week: it takes a squared byte from 1 of 3 operations proven
to 3 of 3, and the parsers' reads feed comparisons rather than counted arithmetic.

**One more bug, caught the same way.** With the analysis finally good enough to narrow a loop that
reads a table, index narrowing fired on `array-literal` and **javac refused the file** — the value
fits, but Java TYPES the expression, and a `long[]` element makes the arithmetic `long` whatever its
range. A narrowed variable needs a cast on assignment; the post-clause path does not, because
`PostVars` already refuses an update that reads another loop variable.

**Where it ends up: `tokenize.oro` 86.7% and 16 of 20 loops, or 100% and 20 of 20 with one range
declared; `tree.oro` 88.9% and 21 of 25, or 91.3% and 25 of 25.** Every loop in both parsers proves
once a range is declared.

**A FROZEN BUFFER CARRIES WHAT WAS PUT IN IT** —
[frozen-2026-08-28](gauntlet/results/frozen-2026-08-28.md). The tree's 50 unproven operations were
NAMED before anything was built, and they were **not relational**: every one traced to `(nodes k)`, a
read out of the frozen node table, returning ⊤. An octagon relates variables and cannot say what a
table holds, so **the highest-value move on the decidability map would have bought none of it**.

**The theorem is the one the harness already tests, now USED rather than checked**: a slot holds either
the zero fill or the most recent `set`, so every value read out of `b` is in `γ(ElemType(b))`. **Why it
is not circular is a STRATIFICATION** — a read inside `λx.e` is stratum 0 and stays ⊤ (nothing binds
the buffer's own name: the only binder is `let`, and a build term is never in scope inside itself); a
read from outside, after the freeze, is stratum 1 and may have the range. Computing `E(b)` analyses
`λx.e`, where every read of `b` is stratum 0, so it never consults itself.

**What the argument had not said is that the sub-analysis loses the LENGTH.** `BufferRange` runs on the
build lambda alone, where everything the enclosing program bound is free. `measure` has a declared
`where`; **`run` has no signature at all**, and `run` is where reduction substituted four literal
documents — so the identical shape succeeded in one and failed four times in the other. The fix is a
**seed**, restricted to **exactly-known lengths only**, and that restriction is soundness rather than
tidiness: a length comes from `exactLen` on a literal or from `assumeWhere` on a signature, so it is
syntactic or a premise, never a fixpoint iterate — and seeding an iterate would seed a claim that is
not yet a post-fixpoint.

**tree.oro 91.3% → 92.7%, `go.-` 70 of 74 → 74 of 74**, and **every emitted file on all four targets is
byte-identical** across 41 programs, with compile time unmoved. A pure provability gain and not a speed
one, which is the honest way to record it.

**AND THE HONEST LIMIT IS NOW A DIFFERENT ONE.** The 42 operations still unproven all chain off `d`, a
depth **read back out of the worklist that stores it** — stratum 0, correctly refused, its element
range being the least solution of `E ⊇ {0,1} ∪ E ∪ (E+1)`, which widens to ⊤. What bounds `d` is
`d ≤ steps`. **This result called that a witness for octagons and that was WRONG** — see
maxlen-2026-08-28: `d` is read out of a *table*, so bounding it needs an inductive invariant over the
buffer's SLOTS, which is a quantified array invariant and strictly stronger than an octagon.

**R3 IS MEASURED, AND OURS BEATS THE HOST ON GO** —
[bigarith-2026-08-28](gauntlet/results/bigarith-2026-08-28.md), on hamza's *"the decision should be
on which is faster, and which fits better with the language."* Measured before anything was designed
around it. The workloads are the two programs `examples/int/` REFUSES — `power`'s accumulator
overflows by multiplication, `fib`'s by addition — and `math/big` is the correctness oracle.

**50! (4 limbs) 3.97×, 200! (20 limbs) 1.81×, fib(1000) 1.11×, all at ZERO allocations; 2000! (1900
limbs) is parity.** The advantage is largest small and amortises away, which is the honest shape:
`math/big`'s inner loops are hand-written assembly and better than ours — what we avoid is its
OVERHEAD, an allocation per operation naively and a receiver plus sign plus length plus a
bounds-checked slice even carefully. **Naive `math/big` is 4–5× worse than careful**, at 100 and 400
allocations, which is what a `bignum` type costs if it lowers without anyone thinking about it.

**AND A DECLARED RANGE IS WHAT MAKES THE BIGNUM CHEAP**, which CORRECTS precision-by-declaration.md
§5. That section recommended not supporting finite-but-huge ranges at first; it is right about the
MIDDLE of the ladder (a 128-bit rung exists on two of four hosts) and wrong about the TOP. A finite
wide range gives a limb count → a `build` of known length → zero allocations. An **unbounded**
declaration gives none of that and lands back on allocate-per-operation. **Bounded-but-huge and
unbounded are two different rungs, worth a factor of four.**

**A bignum needs NO growable storage** — a product has at most `len(a)+len(b)` limbs and a sum
`max+1`, so every result's size is computable from its operands' and "count, then build" covers
bignums completely. **R3 is expressible with `build` today**, which is growth.md's conclusion arriving
from a direction this benchmark was not written to test.

**And the significant length must be a LOOP VARIABLE — leaving it out is a silent 1.21×.** The first
`FibLimbs` added over the whole fixed buffer and measured 1.21× SLOWER than `math/big`; the buffer is
sized for fib(1000) and the early iterations need one limb. `build` gives a buffer its CAPACITY and
the COUNT has to be carried in `again`. Tracking it flipped the result to 1.11× faster — so the
suspicious number was explained before it was recorded, rather than shipped as *"addition is where the
host wins"*, which is false.

**AND JAVASCRIPT DOES NOT INVERT — IT CROSSES OVER.** `BigInt` against a bitwise limb form: **50! is
5.8× OURS, 200! 1.32× ours, fib(1000) parity, and 2000! is 2.62× BigInt.** Go's crossover is around
1900 limbs and V8's around 100, because `BigInt` is C++ with real 64-bit limbs and **JS has no
64×64→128 multiply at all** — but **the sizes precision integers actually reach are the small ones**,
a value just past 2⁵³ being two or three limbs. So the answer is a **THRESHOLD, not a winner**, and a
threshold is what a target declares: `int-repr` already says what a host can hold, and the same file
saying *past N limbs use the host's own* is the parasite model working.

**TWO V8 FACTS WORTH MORE THAN THE HEADLINE.** **The constraint is not 2⁵³, it is 2³¹** — the classic
base-2²⁴ JS-bignum limb (jsbn, node-forge) blew up **75×** from 50! to 200! where the work grows 19×,
with a **2.5× cliff between n=130 and n=132**; `acc[i]*k` leaves int32 at k=128. A **control** settles
it: an identical base-2²⁰ form, chosen to stay under 2³¹, has **no cliff** and is 1.7× faster at 200
*despite carrying more limbs*. Exactness is the folklore limit; the Smi boundary is the performance
one, seven bits lower. And **bitwise carry extraction is worth 3.9× at equal storage** — a shift and a
mask against a compare and a subtract, isolated on fib(1000). Plus a storage rule that is the opposite
of the usual advice in both directions: **match the element kind to the arithmetic** — `Int32Array`
beats a plain `Array` for int limbs (1.20×), `Float64Array` LOSES to one for the same values as floats
(1.47×), because V8 keeps a packed `Array` of small integers as Smis.

**AND JAVA IS WHERE OURS WINS BY THE MOST, WITH NO CROSSOVER FOUND** — **50! 6.2×, 200! 3.4×,
fib(1000) 2.67×, and still 1.84× at 314 limbs** where Go had reached parity and V8 had lost. The
reason is the API, not the arithmetic: **`BigInteger` is IMMUTABLE**, so every operation allocates a
fresh object and a fresh `int[]`, and the JDK's own mutable version is package-private.

**So R3 is OURS with a per-target THRESHOLD, and the threshold is a target declaration.** Go crosses
at ~1900 limbs, V8 at ~100, Java nowhere measured. `int-repr` already says what a host can hold; the
same file saying *past N limbs use the host's own* is the parasite model working.

**And the high multiply is NOT worth declaring unless the host has an UNSIGNED one.** `Math.multiplyHigh`
is a JDK 9 intrinsic and is **signed**; JDK 17 has no `unsignedMultiplyHigh`, so 64-bit limbs need a
three-term correction per multiply plus `Long.compareUnsigned` for the carry — and **32-bit limbs win
at 50! and 200! despite carrying twice the limbs**, losing 2% at 2000!. Go and x86 have unsigned high
multiplies, JDK 18+ has one, JDK 17 and JavaScript do not. Also: **`int[]` buys nothing but memory** —
within noise at every size against `long[]`, which is what our `int` emits as anyway.

**AND WINDOWS ASKS A DIFFERENT QUESTION, because it ships no bignum: what must the target DECLARE to
reach hand-written assembly?** Hand-written x86-64 under MASM, checksums verified independently.
**2000!: `adc` 162,663 ns against an explicit carry's 340,658. fib(1000): `adc` 3,922, a DECLARED
PRIMITIVE 7,271, explicit carry 15,449.**

**The multiply case is FULLY RECOVERABLE and fits better than it had any right to.** `mul` is one
ordinary `(prim …)`, and values.md's multiple return passes two results in **rax/rdx on x86 — exactly
where `mul` puts its low and high halves**. So `(prim mulwide (int int) (int int) …)` needs no new
machinery and emits `mul; add; adc`, which IS the hand-written form.

**The carry chain is HALF recoverable, and that is the finding.** A declared two-result `add-carry`
materialises the carry as a VALUE — nothing survives between statements — but produces it with
`adc r,0` rather than a compare, recovering **2.12× of the 3.94×** and leaving **1.85×**. So **`adc`
is a real hole and a BOUNDED one: 1.85× on addition-heavy bignum code and nothing on multiplication.**
The first version of that file assumed `adc` was simply unreachable and did not measure a middle form
at all.

**Absolute floor for 2000!**: x86 hand-written **162,663 ns**, Java 64-bit limbs 263,712, Go ~271,000,
Java `BigInteger` 485,304, JavaScript bitwise 833,348 — read as a floor rather than a ranking, since
the loop structures differ and the x86 fib does not even track its significant limb count.

**AND BIG×BIG FLIPS THE ANSWER, which makes the threshold PER-OPERATION.** Sized by bits so every form
multiplies the same magnitude: **crossover at 4–8 limbs on Go and Java, and NONE on JavaScript**, where
`BigInt` wins from the smallest size and by **148× at 16,384 bits**. Against big×small, where ours won
to ~1900 limbs on Go and never lost on Java.

**The rule underneath every number in that document: ours wins where the operation is LINEAR, the host
wins where it is QUADRATIC.** Big×small and big+big are linear for both sides, so our advantage is the
per-call overhead we avoid — a constant factor that persists. Big×big is quadratic for us and
sub-quadratic for them, so their advantage compounds and takes over almost immediately.

**AND "KARATSUBA NEEDS RECURSION" WAS WRONG — hamza refused it and was right** —
[karatsuba-2026-08-30](gauntlet/results/karatsuba-2026-08-30.md). Karatsuba's recursion tree is
**balanced and data-independent**, so its shape is a function of `(n, D)` alone: lay it out in advance
and walk it **bottom-up, level by level** — three nested loops, **no recursion, no stack, every buffer
sized before the first loop runs**, verified against `math/big` and worth **2.45× over schoolbook at
1,024 limbs** (9.53× behind `math/big` → 3.91×). The explicit-stack trick answers a *traversal*, where
a node returns nothing; the level-walk sidesteps the value-passing entirely, which is the same reason
an **iterative FFT** exists.

> **A balanced, data-independent recursion is a loop over levels.** That is the general statement and
> it is worth more than the one algorithm: mergesort, FFT, Karatsuba and binary search are all loops.
> What ADR 0014 actually forbids is divide-and-conquer whose **tree shape depends on the data** —
> quicksort's pivot, a search that prunes on what it finds.

**AND IT PORTS: Java 3.59× over schoolbook (5.62× behind `BigInteger` → 1.57×), JavaScript 2.22×
(68.9× → 31.1×).** Java is where it pays most; JavaScript is where it changes least, because 15-bit
limbs against V8's 64-bit ones in C++ is a gap no algorithm closes.

**AND ON WINDOWS, WHERE WE CONTROL EVERYTHING, IT IS THE FASTEST OF THE FOUR** — hand-written MASM,
`mul`/`adc`/`sbb` throughout, **844,682 ns schoolbook → 234,065 Karatsuba, 3.61×**, which is 1.44×
faster than ours on Go and 2.28× faster than ours on Java. **Cross-host verified rather than
self-consistent**: the operand generator is Go's `LimbsOf` reproduced exactly and the product's top
limb is `10113443065733330941` on both, with Go's checked against `math/big`. **And one structural win
the other ports miss** — the descriptor table is a function of `(n, D)` ALONE, so it is computed once
in setup and the timed path never touches it; Go, Java and JavaScript rebuild it every multiply.

**AND ON WINDOWS IT BEATS `BigInt` BUT NOT `math/big` — 1.12× ahead, 1.50× behind, at 65,536 bits.**
Chasing that produced the diagnosis, and **it was never the algorithm**: Go's `math/big` has **no
Toom-Cook**, so at 1024 words it runs the same Karatsuba we do, and the gap was entirely its inner
loop. `addMulVVW` is hand-written **MULX/ADOX/ADCX**; ours was the naive form, serialising on `mul`'s
fixed `rdx:rax` and one carry chain. `MULX` touches no flags so several can be in flight, and `ADCX`/
`ADOX` carry through **CF and OF independently** so two chains run at once — with the catch that `dec`
and `cmp` write OF, so the chains must live inside an **unrolled block** and be folded at its boundary.
**Schoolbook 844,682 → 456,068 (1.85×), Karatsuba 234,065 → 184,058**, checksum unchanged.

**AND THE COMBINE IS DONE: 184,058 → 173,602, so 234,065 → 173,602 overall, 1.35×.** Two changes, both
from noticing the buffers are mostly zero: **z0 and z1 have exact short significant lengths** (`2h` and
`2(l-h)`, zero above), so `zero(sz); add z0; add z1` becomes `copy z0; copy z1; zero the tail` and both
subtractions shorten. **A latent hazard fell out on the way** — `k_school` zeroed `2n` but a slot is
`prodOf[D]` and the lo/hi children have `ln = lenOf[D]-1`, so two limbs were never cleared and the
combine read them; invisible because the benchmark repeats the same operands, so a stale limb held
exactly the right value. **Now 1.41× behind `math/big` (was 1.91×) and 1.18× AHEAD of `BigInt`.**

**AND THE TILED LAYOUT LANDED: ~163,000, so 234,065 → 163,000 overall, 1.44×.** The sizing works out
exactly if **every slot is `2·ln`**: child 0's product goes at the parent's output offset 0 needing
`2h`, child 1's at `2h` needing `2(l−h)`, and `2h + 2(l−h) = 2l` — **they tile the parent exactly**, so
nothing is copied and nothing is zeroed. Only the sum child needs its own storage, and `out[h..] += z2`
reaches `2l − h + 2 ≤ 2l` whenever `h ≥ 2`. The combine collapses to **three passes**:
`z2 -= z0; z2 -= z1; out[h..] += z2`. **The order is the trick** — both subtractions go into z2's OWN
buffer while z0 and z1 are still pristine in the destination; in place they would read `out[0..2h)`
while writing `out[h..)`, which overlap. **Now 1.31× behind `math/big` (from 1.91×) and 1.28× AHEAD of
`BigInt`.** Checksum unchanged at every depth throughout.

**What remains is not structural**: `math/big` recurses below our base case, its inner loop unrolls
further with two interleaved `mulx` streams, and **squaring is not specialised** (half the partial
products in `a·a` are duplicates, worth close to 2× and most of an exponentiation). **None of these is
a language question** — the remaining 1.31× is ordinary assembly tuning.

**What is still on the table, named**: the combine is six passes and the two copies are the largest.
A recursive Karatsuba pays neither, computing z0 and z1 **directly into the destination**. That is
reachable — alias child 0's slot onto the parent's output at 0 and child 1's at 2h — and it is a
LAYOUT change rather than a tweak: `out[h..] -= z0` would read `out[0..2h)` while writing `out[h..)`,
which overlap, so the subtraction must go into z2's buffer first; and slot sizes stop being uniform.

**What is left is the COMBINE, with arithmetic**: the kernel now runs at **0.435 ns per limb-multiply,
about 1.4 cycles**, near the one-`mulx`-per-cycle ceiling. Base-case work falls as `(3/4)^D` while the
combine rises as `3^D` — 17% of the time at D=4, 35% at D=5, **52% at D=6** — and they cross exactly
where depth stops paying. Ours makes **six passes** per node (zero, +z0, +z1, +z2, −z0, −z1) where
`math/big` computes into the destination with one scratch. **So the next move is the combine, not a
better algorithm.**

**Karatsuba is worth 2.2×–3.6× on every host, and it does not change who wins**: 9.53× → 1.93× behind
`math/big`, 5.62× → 1.57× behind `BigInteger`, 68.9× → 31.1× behind `BigInt`, and on windows there is
nothing to lose to. What remains everywhere is the same two things and **neither is about recursion**:
hand-written ADX/MULX inner loops, and Toom-Cook above Karatsuba's range.

**AND THE JAVASCRIPT MEASUREMENT WAS WRONG TWICE — the THIRD method error in this repository's JS
numbers.** A fixed iteration count gave a **4× spread on identical work** (5,900 ns at 20,000
iterations, 23,000 at 2,000), and with the operands loop-invariant and the product unused V8
**eliminated the multiply outright** — a 16,384-bit product *measured* **48 ns/op**. Fixed by a
time-budgeted harness and a sink the result escapes into. The sanity check that settles it: 22.9 µs
for 256 64-bit limbs is the right order against Go's `math/big` at 12.9 µs; **5.9 µs would have made
V8 faster than `math/big`**. bigarith's JS column is corrected — direction unchanged, magnitudes
roughly halved, 148× becoming 68.9×.

**AND IN PLACE IT IS 3.41×, because two of the three children are SUBRANGES OF THE PARENT.**
`(a_lo, b_lo)` and `(a_hi, b_hi)` are already in memory; only the sum child is new data. So a node is
an **offset and a length** — a flat descriptor table over one arena — which is this repository's own
answer to recursive data arriving in a third place. **1.40× over the copying version at 1024 limbs
(337,535 ns against 470,807), 81% of theory's `(3/4)^D` where copying reached 58%, and 9.53× behind
`math/big` becomes 2.78×.** The optimal depth **moved deeper**, which is the clearest evidence the
diagnosis was right: cheaper levels mean more of them pay.

**What is left is not copying** — the sum child is genuinely new, and the upward combine is the
algorithm. The residual 2.78× is close to the ~2× `math/big`'s hand-written ADX/MULX inner loop shows
at sizes where neither side does Karatsuba: **we have most of the algorithm and none of the assembly.**
One bug on the way, in the SIZING: a parent's product must reach `2h + (a child's product)`, and a
flat `+4` is not that. `karatsuba.go` got it right by accident — its uniform padding makes the two
expressions equal — and the ragged version has to compute sizes bottom-up.

**The design conclusion is unchanged and only its REASON changes**: we call the host's multiply
because its inner loop is assembly and its asymptotics go further, not because the language cannot
express the algorithm.

**The comparison was WRONG once and it flattered us.** The first Java run sized all three forms at the
same LIMB COUNT, so the 31-bit form multiplied numbers half the size and reported ours winning at every
size — the exact opposite of the corrected result. Same error shape as the `FibLimbs` one: a number
made good by measuring less work, caught by the same reflex. **And the best limb width is
operation-dependent too** — 32-bit beat 64-bit for big×small on Java and reverses for big×big.

**PRECISION BY DECLARATION IS DECIDED, PROVISIONALLY** —
[ADR 0019](docs/decisions/0019-precision-by-declaration.md). **Bounded by default; an integer
operation the compiler cannot prove stays inside the window is a COMPILE ERROR; and the error is
cleared by saying one of three things — narrow the range, declare a range ABOVE the window (which
promotes that value to arbitrary precision), or ask for the trap.** `int` keeps ADR 0012's meaning and
0012 is not superseded.

**What decided it against B was MEASURED, and did not exist when C was proposed.** A finite range
gives a limb count → a `build` of known length → zero allocations, worth **3.97× on Go, 6.2× on Java,
5.8× on V8**; an unbounded declaration gives none of that. **B's surface is a type name and carries no
size; C's is a range and carries exactly what the fast path needs** — about a factor of four, not a
matter of taste. And turning the checks on now costs a **byte-identical file on 30 of 39 programs**.

**A is rejected because the failure mode is SILENT-SLOW**, against a project that has chosen loud every
time — and note that **SBCL, the most serious attempt at A, added compile-time notes** because it hit
exactly this. Also: a tagged fixnum is not the host's integer, which is the parasite rule; and the
untagged form makes representation a whole-program property that infects tables and ABIs, where C's
blast radius is bounded because **every source of `big` is a declaration somebody wrote**.
**Trap-by-default (Swift, Zig) is declined only because our proof rate makes a COMPILE-time refusal
affordable** — a refusal names the operation, a trap names a stack frame.

**The top rung is NOT one implementation, and that is the part the aesthetics hid**: ours wins where
the operation is linear, the host wins where it is quadratic, so the threshold is per-OPERATION as well
as per-target and something must choose. Whole-program reduction makes that decidable — the compiler
sees which operations occur on a value — but a value whose representation changes needs a conversion
at the boundary, and where that sits is opened rather than closed.

**Owed before any of it works: ~~a scalar range is not usable at all today~~ (DONE, below), a spelling
for the unbounded rung, a **bidirectional** representation solver (factorial is the witness — the
pressure comes from the declared RESULT), and R3 per target. **The trigger to watch is the one nothing
measured can settle**: a real application needing declarations in more places than a programmer will
tolerate. 30-of-39 is numeric kernels and two parsers written by people who knew the analysis.

**A SCALAR RANGE IS A TYPE, AND IT WORKS NOW** —
[scalarrange-2026-08-31](gauntlet/results/scalarrange-2026-08-31.md), ADR 0019's first owed item.
`(sig sq ((n (int 0 1000))) int)` parsed and was then refused at **every use of the parameter** — a
legal program rejected by its own declaration — because `core.ValueType` was called at **exactly one
site**, the table-read path. **A range has THREE effects and the build is keeping them apart**: it is
an `int` **for typing** (normalised at `compatible`, the single point two types are compared); it is a
**premise**, desugaring IN THE READER into the `where` it means, so no analysis learns a new thing
exists; and it is a **representation** declaration, which is still owed — the range is preserved on the
signature and only its consumers normalise. The desugaring is a **definition, not an approximation**:
γ(int LO HI) is exactly the satisfying set of `(and (<= LO n) (<= n HI))`. And it is complete rather
than convenient — **the reader is the only producer of a signature with named parameters**, checked.

**AND THE REFUSAL WAS STANDING IN FRONT OF A SILENT WRONG ANSWER.** With the checker no longer
refusing, the declaration reached `seedFromSig` for the first time in the range language's life and Go
emitted **`func GenSq(n uint16)`** — `n * n` wrapping at 65536, so 1000×1000 returned **16,960**.
Latent because nothing could ever reach that line. *A refusal can hide a wrong answer, and removing the
refusal is what finds it.* The test is the theorem rather than the spelling: **a range and the `where`
it means must emit the same function.**

**AND A RANGE IN THE RESULT POSITION WAS A DECLARATION NOBODY CHECKED.** postconditions.md's algebra is
a swap and this is that swap in the type language: `result : (int LO HI)` is an `ensures`. The same
false claim written as `(ensures …)` was **refused** with the interval that disproves it while
`(int 0 5)` was **accepted in silence** — two spellings of one claim disagreeing. **And the synthesised
conjunction had to be built ERASED**: the connectives do not survive reading (ADR 0017), so `(and a b)`
built *after* that erasure is a shape no consumer knows and came back *"outside the decidable
fragment"*. The failure mode is worth naming — **`CheckEnsures` returns SUCCESS with a note when a
claim is outside its fragment**, so the wrong form passed while checking nothing, and a test asserting
only *not refused* would have passed with it.

**And a differential case was written and DELETED, because it passed with the bug reintroduced.**
Reduction inlines every call, so **a declared parameter only survives at an EXPORT** — the same
structural limit already recorded for index narrowing — and the harness calls with literals, so the
output comes entirely from folded constants. The suite is **not** blind for the reason it is blind to a
bad element narrowing (JavaScript declares no `int-repr` and would have disagreed); it simply cannot be
made to *execute* the code that has the bug. `cmd/gen` reaches it. **Cost: byte-identical output on 41
programs × 4 targets, so no speed claim** — one refusal of a legal program removed, one latent silent
wrong answer fixed, one silently-ignored declaration made real.

**PRECISION BY DECLARATION IS RESEARCHED** — [precision-by-declaration.md](docs/precision-by-declaration.md),
hamza's third option: *bounded by default, but a range declared ABOVE the bound moves that value to
arbitrary precision*. **Possible, and it is ADR 0003's ladder finished rather than a new mechanism** —
`int-repr` already picks the narrowest declared representation containing a range, and precision is one
more rung at the TOP. Below the word is built and measured; above it the language currently says
nothing, and ADR 0012's window is where the ladder was cut off. **Going up the ladder can never lose a
value; going down can** — which is why narrowing a local is refused and widening one is not.

**The expected blocker is NOT the blocker.** `int64` is hardcoded as the width of the range language,
the abstract domain (`sat = 1<<62`) and `IntRepr`, so ⊤ means *"bigger than 2⁶²"* and *"unknown"* at
once. The obvious conclusion — the interval domain must go arbitrary-precision — is **wrong**: an
operation on a value that is ALREADY exact cannot overflow, so the analysis is never asked about it.
What C needs is a **two-point representation lattice `word ⊑ big` with `big` absorbing**, and one
soundness rule: *an operation is an error only if every operand is word-represented and its result
cannot be proven to fit*. `emit/interval.go` is untouched.

**And the propagation must be BIDIRECTIONAL, with factorial as the witness**: `(fact (int 0 30))` has
every input small and an accumulator reaching 30! ≈ 2.65×10³², so the pressure comes from the declared
RESULT and nothing forward makes `acc` big. A forward-only solver would pass the entire current corpus
and fail on the first factorial.

**THE PREREQUISITE, CHECKED RATHER THAN ASSUMED: a scalar range is not usable today at all.**
`(sig f ((n (int 0 1000))) int)` is refused — *"n is int 0 1000, but int is required here"* — because
`core.ValueType` (a range MEANS `int`) is called at **exactly one site**, the table-read path, and
`paramIval` matches `sp.Type == "int"` and so ignores a declared range anyway. **The range language
works on array ELEMENTS and nowhere else**; a scalar's range is stated with a `where`. That is small to
close and worth closing whichever option wins.

**What C does NOT buy, and it is the part to weigh: C is B, with a third answer available where B
already demanded one.** It refuses exactly the same six corpus programs. If the objection to B is that
a real application needs declarations everywhere, **only A answers that** — and A pays with
silent-slow failure and a whole-program boxing story. **The one measurement that can kill B and C is
not about integers**: how many declarations does a real application need? 30-of-39 is a corpus of
numeric kernels and two parsers written by people who knew the analysis.

**WHAT TURNING THE CHECKS ON COSTS NOW: ALMOST NOTHING, AND THE PATH WAS BROKEN** —
[checked-2026-08-28](gauntlet/results/checked-2026-08-28.md). `-checked` emits a **byte-identical file
on 30 of 39 programs**; the nine that change are `tree.oro`'s 42 (the `d` chain), `smooth-java`'s 2,
`divmod`'s undeclared parameters, `wordcount`'s map value range, and **collatz/power/fib, which are in
`examples/int/` TO BE REFUSED**. Cost on the one real program that pays: **1.05× on Go, inside the 15%
noise floor** — consistent with checkcost-2026-08-19's memory-bound regime, not its 4.54× one.

**And `-checked` emitted Go that would not compile.** `evalR` opened a lambda's body with `Body()` and
rewrapped it with **`FnClosed`, which does not close** — so every parameter occurrence came back a FREE
NAME and the binder stopped binding; `tree.oro` emitted `nodes7 := nodes` referring to a different
function's variable. Invisible because the default path throws the rebuilt term away. **Printing hides
it** — an opened body prints the same — so the test has two halves, and only the structural one
(a lambda's stored body holds its parameters as indices, never as names) fails against the bug.

**The affordability blocker for precision integers has therefore CLEARED, and it was never the real
one.** `-checked` is **detection, not precision**: it panics where exactness would promote. Promotion
is a **representation change** — a value that may leave the window is a boxed bignum, which is the one
thing the core may never contain. So the proof rate decides *whether a boxed value can appear at all*,
and the open decision is the policy at an unprovable site: **box** (refused — the predecessor's cause
of death), **trap** (what `-checked` does, honest, not exactness), or **refuse to compile** (Low\*'s
*"the restriction IS the mechanism"*, and `examples/int/` already demonstrates it). That is an ADR, not
a build.

**THE THREE MAP MEASUREMENTS ARE RE-TAKEN, AND ALL THREE MOVED** —
[maps-2026-08-30](gauntlet/results/maps-2026-08-30.md). **JS's `Map` is 1.56× slower than an object,
not 3.25×** — direction survives, magnitude halved — and **a plain `{}` beats `Object.create(null)`**,
which is the opposite of the folklore. **Integer keys are 3.67×**, more than twice the string gap, so
`(map int V)`-first is the case where the host choice matters MOST rather than a way of dodging
strings. **Java's fused `merge` is 1.22× FASTER**, not 2.59× slower — the second independent re-take
to disagree with R5, so a number that has failed to reproduce twice should stop being quoted: the two
forms are within ~20% on JDK 17 with the fused one ahead. `targets/java/util.oro` already declares
**both** and the program picks, so nothing there needs changing. ADR 0008's best example survives by
being applied to itself.

**AND COUNT-THEN-BUILD BEATS APPEND, so a growable array is NOT NEEDED**: **2.95× on Go** against
growing `append` and **1.44×** against one with the capacity already right, and **1.06× on
JavaScript** — the counting pass is a branch-predictable scan with no stores and the fill writes into
an exactly-sized allocation. growth.md's D3 is **withdrawn**. Its own prediction that two passes would
lose on JavaScript was **wrong**, and the first run agreed for the wrong reason: it compared a plain
`Array` against a `Float64Array`, so it measured **array kind rather than growth**. Third time this
session that **a comparison changing two things at once measured neither**.

**A BUFFER READ WAS MOVING ACROSS A STORE, AND A SWAP BECAME A COPY** —
[effects.md §7c](docs/spec/effects.md). ADR 0010 denies **exchange** and ADR 0018 says a
`(buffer V)` read is impure, which is what stops a read being moved past a store. But `pureTerm`
answers *"value"* for every bound variable, so `(b 0)` was judged PURE and substituted into the store
position: `(let (b 0) (fn (vx) (let (b 1) (fn (vy) (set (set b 0 vy) 1 vx)))))` emitted
`b[0] = b[1]; b[1] = b[0]` — **a silent wrong answer on all four targets**, so the differential suite
could not have found it either. Latent because it needs a read, then a store to that slot, then a USE
of the read; the tokeniser and the tree consume each read inside the same expression as the store that
follows. **What found it was writing a SORT** — for a map's `keys` — which is the argument for
programs over construct suites arriving again.

**The fix tests the DESTINATION, not the operand**: nothing in a term says which bound variables are
buffers, since an array read has the identical shape and is genuinely pure. So **a term that reads a
table through a bound variable is not substituted into an impure body** — a read may move into a body
with no effects to be reordered against, and not into one that has. **Testing the operand was tried,
measured and is WRONG**: a rule-table's rule reads its parameter table, so `(table n f)` becomes
impure, stops being substituted and reaches the backend UNFUSED — `dot` and `smooth` on Java stop
compiling. **Cost: 2 of 164 emitted files change** and the one benchmarked program among them measures
**5,842 ns against 5,840**, indistinguishable.

**MAPS ARE SPECIFIED AND BUILT ON ALL FOUR TARGETS** — [maps.md](docs/spec/maps.md). A map is a table whose index set is
a finite subset of the key type, and everything follows from that plus three decisions. **F2**:
`(m k)` is `(option V)` — still application, with the RESULT TYPE decided by the domain kind, which is
where the difference between tables.md's three points should show: the function's domain condition is
free, the array's is a proof, the map's is **a value**. **And F2 REMOVES an obligation rather than
adding one** — the option makes the program say what happens when the key is absent, which is the only
thing that can, since `k ∈ dom m` is set membership and no relational domain helps.

**The key type is FORCED, not preferred.** `(map K V)` is well-formed exactly where the language's `=`
is defined, and `=` is **integer equality only** — floats out because NaN is not an equivalence
relation, strings out because no two targets agree. So `(map int V)` first is what the language already
required; growth.md's framing of it as dodging the string question was weaker than the truth.

**A static map leaves NOTHING**: `((map (1 10) (2 20)) 1) → (some 10)` is **β-tab with a sum in the
result**, and the `case` then folds, so no map, no tag, no allocation. **And a dynamic read IS the
host's own fallible read** — `(case (m k) (some v) A (none) B)` is `if v, ok := m[k]; ok`. So F2's
option is not a thing we add; it is a thing the host already has and we were discarding. Unmeasured is
an option that **escapes**.

**EVERY MAP HAS A DECLARED CAPACITY, and that is the load-bearing decision.** windows ships no map, a
growing hash table rebuilds into a larger allocation, and **that is not expressible** — `build` fixes
its length and a nested `build` returns a frozen array, not a replacement buffer. So: let three hosts
grow and windows not (an **observable** disagreement, which is a Tier 2 construct in the core), block
on growable buffers (inverting growth.md's own finding that the map is primary *because* the array has
a workaround), or **declare it**. The precedent is `tree.oro`'s node cap and it is celebrated rather
than tolerated: **a capacity does not create the limit, it makes it VISIBLE.**

**ITERATION IS `keys`, ASCENDING — DERIVED, and the first answer was wrong.** maps.md §7 first
specified a fold over an ENUMERATED list of commutative monoids. hamza refused the reasoning behind
it: *"it should be mathematical and algebraic, based on reasoning and research… programs when written
will inform us how good our design and implementation are."* **Deriving what a construct IS and
measuring whether it is GOOD are different jobs, and waiting for a program is only ever the second.**

The derivation, in four steps. **The index set decides the free structure**: `Fin n` is ordered so an
array's entries are a LIST (free monoid); a map's `S ⊆ K` is unordered so its values are a MULTISET
(free *commutative* monoid). **The universal property then forces the operator** — a fold out of the
free commutative monoid exists and is unique exactly into a commutative monoid — which is what the
first draft got right. **Commutativity is necessary and NOT sufficient**, and `argmax` shows it: ⊕
fails commutativity *only on ties*. **So supply the missing structure instead of restricting what
consumes it** — a total order `≤_K` makes `argmax` "max by (v, then −k)", a max over a total order,
hence a commutative monoid.

That generalises: an order on K turns the multiset back into a list, so
**`keys : Map K V → Array K` in ascending `≤_K` order** is the eliminator. Both halves are derived —
the result is an `Array`, whose index set `Fin n` is ORDERED, so producing one from an unordered index
set REQUIRES an order, and the only canonical one is K's own. A host's order is not canonical (Go
randomises on purpose, JS specifies its own, Java leaves it unspecified), so **sorting by `≤_K` is what
makes `keys` the same on four hosts precisely because it ignores all four**. Insertion order is ruled
out by the algebra: a map is a SET. And `≤_K` is free — `=` is integer equality and `int` carries `≤`.

**`fold-map` is therefore a COROLLARY, not a primitive**: `fold-map m ⊕ z ≡ foldl ⊕ z (map m (keys m))`.
It survives only as an *optimisation* — skip the sort when ⊕ commutes — and that is where an operator
whitelist belongs, as a licence to optimise rather than the only surface. It also failed on its own
terms: the reason to iterate a word count is *the top N words*, which is `argmax`, which the first
draft refused. **It answered a question nobody asks and refused the one everybody does.**

**Lowerings**: Go `map[int]int` with comma-ok; JavaScript a **plain object** (3.67× on integer keys,
and `{}` beats `Object.create(null)`); Java `HashMap` with **`merge`** and `get` returning null, which
is ONE lookup where `containsKey`+`get` is two; windows **written in Oroboros**, open addressing over
`build-map` — the first library the language writes for itself, and writable only because of the
capacity decision. One honest per-target cost: JS's `len` is **O(n)** against O(1) elsewhere — same
answer, different price, so not Tier 2.

**HOW A VALUE GROWS IS RESEARCHED, AND THE OWED ORDER IS BACKWARDS** — [growth.md](docs/growth.md),
no spec and no decision. general-purpose.md §2.4 owes *"growable collections, and maps"*; **the map is
primary and a growable array may never be needed**. A growable array has a parity-preserving
workaround every array language uses — count, then build, which is how Futhark, ISPC and every GPU
library do `filter` — and `build` has survived two parsers, a sieve, a stencil and a tree without
anyone missing growth **because every one of those is positional**. A map has no such workaround:
`wordcount` is written **four times** in this repository and **cannot run on windows at all**.

**GROWTH IS A CHANGE OF INDEX SET, and there are exactly two of them.** A table is `Π_{i∈I} V`;
`append` extends `I` by a *position* and `insert` by a *point*. **Append keeps an EQUATION**
(`len b = len b₀ + i`, proved by induction on iterations, and it is a linear fact the existing
fragment decides — the same induction `monotone.go` already performs); **insert keeps only an
INTERVAL**, `|dom m| ∈ [min(1,k), k]`, because whether a key was already present is a fact about the
input. No relational domain helps: the missing fact is set membership, not arithmetic — the gap keeps
not being a linear one.

**AND THE MAP IS THE FIRST DOMAIN CONDITION NOTHING CAN DISCHARGE.** tables.md's three points are
really three answers to *who discharges the domain condition*: `(fn (A) B)` the **type**, `(array V)`
the **refinement layer** in QF-LIA, `(map K V)` **nobody, at compile time** — `dom m` is a function of
the input. So a map read must be fallible, and **that is the first LANGUAGE-INTERNAL argument for
sums**, which sums-research.md justified only from errors, Win32 and dispatch. Go's comma-ok is the
host saying the same thing; tables.md noticed the coincidence and did not draw the conclusion.

**Three of the four measurements a design would rest on are STALE and one is known-unstable** — JS's
`Map` 3.25× is from the first baseline and every surprising JS result here has been a method error at
least once; Java's `merge` already failed to reproduce. Re-taking them comes first. **`(map int V)`
is the honest first step**: no string question, trivial hashing, and it answers growth, the fallible
read, the value range and the windows implementation — which should be written **in Oroboros**, the
first library the language writes for itself.

**OCTAGONS ARE REFUTED BY MEASUREMENT, AND A LENGTH BOUND BUILT INSTEAD** —
[maxlen-2026-08-28](gauntlet/results/maxlen-2026-08-28.md). decidability-map.md calls octagons *"the
highest-value move available"* and three results named a demand. Before building an O(n³) domain,
**every unproven operation in the corpus was classified by what fact would settle it, and NOT ONE
needs an octagon**: 30 are a counter or a subtraction under a `len` guard, 4 want a declared parameter
range or a map's value range, 8 are deliberate true negatives, and 42 are `tree.oro`'s `d`.

**The reason generalises**: an octagonal constraint `i − n ≤ c` becomes a BOUND on `i` only when `n` is
itself bounded — and if `n` is bounded, the guard `i < n` already gives it non-relationally in
`refine`. So the relational fact pays off only where the non-relational one already does. The genuine
octagon shape — two variables each unbounded whose DIFFERENCE is bounded — has one candidate here,
`(go.- ni i)`, and both operands are bounded the moment `(len src)` is. ADR 0008 landing on a
*decision* rather than a primitive.

**What measurement selected instead is that A LENGTH IS BOUNDED AT BOTH ENDS, and the upper end needs
no declaration.** `len` returns an `int` and ADR 0012 makes `int` exact within ±(2⁵³−1), so a table
with more elements has a length the language cannot count — every guarantee about `dom(a) = [0, len a)`
has already failed. **It was relied on without being stated**, which is `split-words`'s shape and the
zero-fill guarantee's shape; now [tables.md §2.3.1](docs/spec/tables.md). A target may declare
`(max-len N)` where the host says something **tighter and specified** — Java's `arraylength` returns an
`int`, so 2³¹−1; **Go declares nothing**, because a slice's real limit is the address space, which is
neither specified nor stable. `N` past 2⁵³−1 is an error.

**Thirteen programs improved**: `smooth-go` **0/13 ops and 0/2 loops → 13/13 and 2/2**, `generic-*`
0/2 → 2/2, `search*` 0/1 → 1/1 with termination, `report-go` 1/2 → 2/2, `smooth-java` 0/16 → 14/16.
**Exactly one emitted file changes** across 53 programs × 5 targets — `search-java` gets `int r1`
instead of `long r1` — so Java's index narrowing now fires on a program indextype-2026-08-25's
syntactic rule explicitly could not help. **No benchmarked program changed, so there is no speed claim
here.** The differential suite passes on all four targets with a new case, `len-bounded.oro`, whose
table is sized by a PARAMETER so nothing knows its length.

**RE-BENCHMARKED, and the withdrawal cost is a PER-HOST answer** —
[rebench-2026-08-27](gauntlet/results/rebench-2026-08-27.md). Tokeniser **Go 0.94x, JavaScript 1.00x,
Java 1.20x**; tree **Go 0.92x** of hand-written clamped (and **2.03x FASTER** than recursive),
**JavaScript 1.02x**, **Java 1.23x**. The node table is `[]int` again, 16 KB instead of 4 KB — and
that cost **nothing on Go** and **1.16x on the JVM**. elemwidth §5d recorded 6,053 → 5,524 as a 1.10x
gain from narrowing it; the un-narrowed program measures **5,483**, so that comparison did not
survive being re-taken across sessions on a 15% noise floor. **Withdrawn twice over**: for resting on
an unsound analysis, and for not being a measurable gain on that host anyway. The JVM number is
consistent rather than surprising — json-tree-bench established element width is what that host is
sensitive to.

**Java's tokeniser lost index narrowing, and that is the RULE WORKING**: narrowing needs every
operation in the method to fit a 32-bit index, and 86.7% is the honest figure, so casts went
**5 → 102** and the cost of 97 casts is **3.7%**. Third time this week the casts have proved cheap.

**AND THE STRUCTURAL FIX LANDED, so the narrowing is earned back.** The obvious design — analyse the
whole function once and look the answer up per `build` — **has no key**: `openFresh` REBUILDS a term
to substitute its bound variables, so the build the backend holds is not the pointer the analysis
saw, and its printed form differs because the parameters were renamed. It was built, measured, and
found to record nothing the emitter could find; attribution was wrong too, since a store to the OUTER
buffer happens inside the inner one's extent. **Carrying the ASSUMPTIONS to the subterm works** —
`intervalsAssuming` runs the sub-pass with the enclosing precondition, renaming sig parameters to the
names the backend opened with, which is what `Refine` already does and for the same reason.

**The contract is what earns the representation**: `(sig measure ((src (array (int 0 255)))) int
(where (go.< (len src) 1024)))` — one line, and `tree.oro` reaches **91.3% and 25 of 25 loops** with
its node table in 16-bit slots. **Java 7,901 → 6,554 ns, which is 1.01x of hand-written `long[]`**
where it was 1.23x; **Go is unmoved at 5,539**. So the node table's width is worth **nothing on Go and
about 20% on the JVM** — ADR 0008 twice on one change. The derived range is `int -1021 1024` rather
than `int 0 511`, because a token length is `ni - i` and the abstract state admits a negative one; it
selects `int16` instead of `uint16`, the same two bytes.

**AND THE TOKENISER TOO — both parsers now declare a contract, and it is honest rather than tuning**:
`(where (go.< (len src) 1048576))` on the tokeniser and `(< (len src) 1024)` on the tree, because a
node table fixed at 512 cannot accept more. The element range says what a byte IS; the `where` says
how many there can be, which is what bounds the token count and hence `(go.* nt 1000)`. The tokeniser
goes **86.7% → 100%** and **16 of 20 loops → 20 of 20**, and Java's index narrowing returns.

**Final: tokeniser Go 0.92x, JavaScript 1.00x, Java 1.16x; tree Go 0.84x of hand-written clamped,
JavaScript 1.02x, Java 1.01x.** What the two declarations are worth is **nothing on Go, nothing on
V8, 1.03x and 1.21x on the JVM** — measured four times now on two programs, and the reason is
json-tree-bench's: element width decides whether a flat node beats a boxed one and only the JVM is
near that line. **A `where` costs nothing where it buys nothing** — it is a premise, not a runtime
check, and on Go and JavaScript it changes no emitted code at all.

**LOOP MONOTONICITY IS BUILT, AND IT WAS DERIVED RATHER THAN DECLARED** —
[monotone-2026-08-27](gauntlet/results/monotone-2026-08-27.md), [emit/monotone.go](emit/monotone.go).
Five results in a row terminated at one missing fact — **a scanner returns more than it was given** —
and postconditions.md §3 proved that *declaring* it on an internal definition would be redundant. So
the compiler derives it: a five-rule relation `e ⊒ S` with a soundness lemma, a theorem by induction
on iterations, and a corollary turning a loop into **a lower bound on the value it produces** — which
is what an interval cannot express when the bound mentions a variable. **The tokeniser goes from
64.5% of integer operations bounded and 15 of 20 loops proven to 90.9% and 20 of 20**, same answers
on four targets.

**The corollary needs BOTH halves and a test for one would pass a wrong program**:
`(loop ((j i)) (go.>= j 10) 0 else (again (go.+ j 1)))` increases every step and returns **0**. And
**two places needed it, not one** — `stepOf` and `relate` derive the same thing separately, so wiring
only `stepOf` moved nothing; the diagnostic went from `i` producing **no arc at all** to `i>=i`.

**And `i>=i` was the TRUTH — the fix was in the program.** `scan-run` starts at `i` and returns `i`
when the first byte fails its predicate, so `i' > i` is genuinely false; the analysis declined to
prove a false thing and named the variable. But the clause fires only when `(src i)` already matched,
so scanning can start one byte further on — identical answers, one fewer test, and the init becomes
`i+1`. **The repair was a line the program wanted anyway.**

**AND THE RUNNING EXTREMUM CLOSED IT.** The fifteen remaining unbounded operations all traced to
`mx`, a running maximum: its `else` branch is `mx` itself, so `next ⊇ cur` **always** — widening threw
it to infinity and the narrowing phase, which accepts only a CONTAINED value, could never take it
back. **Theorem (reachable set):** if every `again` argument is built from `vₖ` and `vₖ`-free
expressions using only `if` (whose *condition* may mention it — a condition produces no value), then
`⟦vₖ⟧ ∈ {⟦zₖ⟧} ∪ ⋃U` at every iteration, by induction. So `hull({z} ∪ U)` is **exact in one step and
needs no widening** — the recurrence is not `v' = f(v)` but `v' ∈ {v} ∪ U`, and the reachable set is
closed. **The tokeniser goes to 100% of integer operations bounded, 20 of 20 loops, and
`idx 0..1321`.**

**Which CASCADED into Java and found two more bugs.** With the range fitting an index, narrowing
fired on the tokeniser for the first time — and **an inner loop narrowed while the outer did not**,
because `fitsIndexSource` refused a loop term, so the scanner's `j` became an `int` initialised from
a `long` `i`. javac caught it and nothing else would; the rule is **a loop's value fits when every one
of its exits does**. Then the loop's **result variable** stayed `long` — narrowing is a whole-method
decision, so either every int local is the host's own `int` or none is. Plus two shapes `narrowIdx`
did not know, both rule 4 again: a **conditional index** (what a clamped stack index looks like) and
an integer **literal**. **Java 9,073 → 8,626 ns and 50 → 5 casts.**

**The honest part: casts went 50 → 5 and the time barely moved.** So **Java's residual 1.16× is no
longer the index type or the element type** — both now match the hand-written reference — and what is
left is code generation plus the refinement layer's guards. That is a different question from the one
indextype-2026-08-25 opened, and the first time it has been. **The tree stayed at 91.3%**: its residue
is values read out of the node table, which nothing bounds — the honest limit of a non-relational
domain, and where octagons would be asked next.

**POSTCONDITIONS ARE BUILT, AND THE ALGEBRA IS A SWAP** —
[postconditions.md](docs/spec/postconditions.md). `(ensures Q)` beside `(where P)`, with `result`
naming the value — reserved only inside an `ensures`, and a parameter may not take the name there.
The contract is **∀x. P(x) ⟹ Q(x, f(x))**, used in the two directions `sig` already had. And
refinements.md §6b's trichotomy for P has an exact dual for Q, **each row exchanging the roles**:
on a `prim` P is an obligation and Q an assumption; on an **exported** definition P is assumed and Q
is **checked against the body**; on an **internal** one both vanish, for the same reason — reduction
removes the boundary. So the implementation is **two cases, not six**.

**Two soundness lemmas, and both bit.** **Lemma 1: an assumption needs its precondition** — `C` is an
implication, so with P unproven Q says nothing, and `f = λx.x` with `P ≜ x>0`, `Q ≜ result>0` is
false at `f(-5)`; one false fact makes a conjunctive fragment derive everything. The trap is that
**"not refused" is not "proven"**: `discharge` reports *propagated, not proven* and returns success.
**The first implementation got this wrong** — the refactor giving `discharge` a *proven* result
rewrote every `return nil` in it, including that path — and the lemma test caught it. That test
exercises the **propagated** path on purpose: a *refused* precondition aborts the walk, so the
downstream effect is never reached and nothing is learned. **Lemma 2: Q attaches to the BINDER, not
the call**, because two occurrences of an impure call differ and the fact layer keys by printed
term — and the binder always exists, by **ADR 0010**, which never substitutes an impure argument.

**What it cannot do, and the reason is Lemma 2 again.** A *pure* call is substituted, so it has no
binder, and the linear fragment cannot name a general application — only a literal, a parameter and
`alen`. So `ensures` on a pure prim is carried as an opaque atom and discharges only by syntactic
match. The fix — treat a pure application as an opaque linear variable, sound by referential
transparency — moves terms from *report it* into *refuse it* across every program and wants its own
measurement. **The Win32 case is impure, which is the case that works.**

**And the relational postcondition everything is blocked on should be DERIVED, not declared.**
§3 says a declaration on an internal definition is redundant, and `scan-string` is one. **Theorem
(loop monotonicity):** if every `again` gives position k a value ≥ its current one, then by induction
on iterations `vₖ ≥ zₖ` throughout, and every exit ≥ `vₖ` is ≥ `zₖ`. **Corollary:** `scan-string`
starts `j` at `i+1` and only ever adds, so its result is `> i` — the size-change witness the
tokeniser's outer loop is missing, with **no declaration at all**. Not built; it is the next thing.

**AND INDEX NARROWING IS GENERAL NOW** —
[indexnarrow-2026-08-27](gauntlet/results/indexnarrow-2026-08-27.md). indextype-2026-08-25 narrowed a
counter *bounded by a length and stepping by +1* and named the program it could not help — the
sieve, whose bound is `i*i >= n`. The rule is now **the interval analysis**: if every integer
operation in a method stays inside a 32-bit index, its counters are held in one. **Java's structural
sieve goes from 1.16x to 0.99x — parity** — with `int` locals 0→4 and `(int)` casts 4→1, the
survivor being `new boolean[(int) n]` on the method's own parameter.

**`MaxOp` is a different question from `fits`** — a value inside the portable window at ±(2^53-1)
does not fit a 32-bit index, and the two diverge on exactly the programs this is for. And **MaxOp
does not cover every value a counter can TAKE**: a literal is not an operation and neither is a
table read, so sources are checked directly and anything unrecognised refuses. **Division is bounded
but NOT counted**, so a rule trusting MaxOp must not trust a division — that distinction lived inside
a switch and is now `arithOp`, read by both the transfer function and the narrowing rule.

**It has to be a WHOLE-METHOD question, and that is the same property from the other side.** Run on
the loop alone it narrowed nothing, because **a loop's bound usually comes from the enclosing
`where`** — less context can only widen, which is exactly what makes `BufferRange` safe on a `build`
lambda in isolation and exactly the wrong trade here. So the emitter asks once with the signature in
hand: one unbounded operation anywhere refuses every loop in the method, and that is the safe
coarseness.

**It changed NOTHING on the existing Java gauntlet** — all nine regenerate identically, because the
syntactic rule already covered `+1` counters over lengths. It earns its place on one shape, a
computed bound, and getting there needed two other facts: **`sieve-java.oro` declared no `where` at
all** (half the corpus does not), and **a target-declared accessor hides the cast in its template** —
`(prim at-bool … expr "%s[(int) %s]")` — so the emitter never gets a say and narrowing cannot reach
it. That is an argument for structural indexing tables.md did not make: **a host detail buried in a
target template is one no analysis can improve.**

**Neither parser narrows**, both reporting `idx -inf..+inf`, for the scanner reason — the **third
independent demand** for a postcondition naming a result.

**AND THE INTERVAL ANALYSIS DECIDES THE REST.** `tree.oro`'s node table stores indices bounded by a
loop guard — `nn < 512` — which no literal can show, so the analysis is asked when the exact facts do
not settle it. The node table is **`[]uint16`, 4 KB instead of 16 KB**, the parse stack narrows too,
and **the worklist correctly does NOT** because it stores a depth read back out of itself. **Go
6,053 → 5,524 ns (1.10x)**, now faster than the hand-written *clamped* form; **Java 7,756 → 6,795
(1.14x)**.

**The soundness argument, because this is where an analysis starts deciding BITS.** The pass runs on
the `build` **lambda alone**, and less context can only WIDEN an interval — so a subterm analysis is
conservative against the whole-program one and anything free is unbounded. Exact facts are tried
first, so the argument carries only the residue. Failure is the safe direction and is the default.
**And the differential suite CANNOT catch a bad narrowing** — every target narrows on the same
decision, so they agree and are wrong together; only `; expect:` can. So the checks are direct:
containment against hand-computed extremes on five programs (**containment, not tightness** — over-
approximating costs space, under-approximating corrupts), refusal for a value read out of the buffer
itself, and the tree's agreement test against two hand-written implementations at 443 nodes. **Two
adversarial cases were written expecting a refusal and got a claim, and the claims were RIGHT** —
`0*3` stays 0 forever and `i*j` for `i<10` really is under 9.9e10; the tests were wrong, which is the
correct way round.

**AND IT CORRECTS json-tree-bench's OWN REASONING.** That result explained part of the JVM's
preference for recursion by size — *"our 64-bit `int` makes a node 32 bytes against a `Node`'s 24"*.
A node is **8 bytes** now, a third of a `Node`, and **recursive still wins there** (4,265 ns against a
hand-written `int[]` flat table's 5,330). So the flat form being larger was **not** the driver; TLAB
bump-allocation and a young collector that pays for survivors are. The headline — *flat beats
pointers is a Go fact* — is unchanged, and one of its three explanations is withdrawn.

**Still not built: a differential case** for narrowing, which is itself a finding:
reduction inlines every non-exported call, so **a narrowed parameter only survives at an EXPORT**,
which is precision-integers.md's "where do declarations live after staging" arriving as a limitation.
**One bug on the way**: the target-directory merge dropped `Reprs`, so a single file selected
`[]byte` and the real `targets/go/` selected `[]int` — silently, and every native target is a
directory.

**PRECISION INTEGERS ARE RESEARCHED, and the blockers turn out to be mostly CLEARED** —
[precision-integers.md](docs/precision-integers.md), on hamza's *"they should not interfere with
getting the best performance when the range is within the supported integers; beyond that they cost
what they would cost if we implemented them on the target."* data-model.md §8 blocked this on the
product and on interval analysis, and **both are built** — as are sums, which it needed for a
fallible result. So the question is no longer whether the machinery exists but **how much it
proves**.

**Re-measured, and the number MOVED — in both directions.** intervals-2026-08-19 said 39% with
nothing declared and 81% with one range. On the old corpus that is now **100% everywhere** — one
declared range takes every numeric loop to complete provability. **On the two parsers it is 64.5%
and 91.2%, and declaring ranges changes NOTHING**, which is a qualitatively different failure from
*not enough is declared*. One cause in both: the loop's progress variable is assigned **a scanner's
return value**, so there is no size-change witness, so no trip count, so every accumulator is
unbounded. **`go.*` is 0 of 15 in the tokeniser** — the worst operation to lose, since a checked
multiply is 1.87x where the hardware high-multiply is reachable and **7.40x where it is not**, and
JavaScript has none. Confirmed by construction: adding an explicit trip counter takes it to 78.5%
and multiplication to 8 of 15.

**What unlocks it is a POSTCONDITION NAMING THE RESULT** — `scan-string` returns `> i` and
`<= (len src)`. `where` constrains parameters only today, and general-purpose.md **already lists
result postconditions as owed** for Win32/SAL's `_Out_range_`. The integer work and the Win32 work
want the same feature, which is the best evidence available that it is the right one. The
alternative is octagons (a third independent demand), and a declared postcondition composes better
because it survives inlining as an assumption instead of needing re-derivation.

**Two things to do regardless of the rest.** **Element width from the range** — `(array (int 0 255))`
as `[]byte`/`byte[]`/`Uint8Array`/`db` — subsumes the boolean special case in `ElemBytes` AND
indextype-2026-08-25's hardcoded platform fact, and json-tree-bench measured our 64-bit element
costing **1.19x on the JVM** for exactly that. And **R3 is a capability the target declares, not a
runtime we ship**: Go has `math/big`, Java `BigInteger`, JavaScript `BigInt` in the language — three
of four hosts already have it, so the bar *"costs what it would cost on that target"* is met by
construction there; only windows needs one written.

**Constant folding INVERTS in our favour** — today folding integers would be an ADR 0009 hazard
(arbitrary precision at compile time, fixed width at run time); if integers are exact the hazard
disappears, because folding at arbitrary precision *is* the runtime semantics. **And `int → f64`
stops being free**, since it is exact today only because the window is binary64's exact-integer
range. **Sub-byte storage is REFUSED with a trigger** (expressible natively on zero of four targets,
and it makes adjacent elements alias, which breaks ADR 0018), and so is **reinterpretation** — *types
choose representation* is wanted, *operations determine meaning* would make every bound, range and
termination proof unsound.

**The eleven integer questions are settled** — [integers.md](docs/spec/integers.md), each measured
on all four targets rather than read off four specifications. They **agree** on everything inside
the window: division truncates toward zero, the remainder takes the dividend's sign, the identity
`(a/b)*b + a%b == a` holds, and `int → f64` is exact — because the portable window IS the binary64
exact-integer range, so the constraint that looked like a concession to JavaScript makes the one
conversion everybody needs free. They **disagree** in exactly three places, and each is refused
rather than emulated: outside the window (no claim), **division by zero** (a precondition — and on
JavaScript `1/0` is `Infinity` and it keeps going), and `f64 → int` out of domain (three hosts,
three answers). Not settled deliberately: a bignum, which needs the product first.

**Representation selection is OPT-IN, behind `-checked`, and that is deliberate** — wiring it into
the default path reversed [ADR 0012](docs/decisions/0012-portable-integer-range.md) without an ADR,
breached requirement 5 by up to 4.54×, and made cross-target divergence *worse* (three targets trap,
JavaScript silently loses precision). Turning it on should be the consequence of deciding exact
integers, not the cause — [assessment-2026-08-20 §2](docs/assessment-2026-08-20.md). **A
demonstration wired into the default path is a decision, whether or not anyone made one.**

**The representation is selected for INDEXES too, and Java is where it mattered** —
[indextype-2026-08-25](gauntlet/results/indextype-2026-08-25.md). Our `int` is 64-bit and a Java
array index is not, so an emitted counter was a `long` and every access carried an `(int)` cast —
**1.04× to 1.54×** against hand-written Java, the one place the project missed its own bar with a
number attached. A counter bounded by a **length** and stepping by **+1** is emitted as the host's
own `int`; both conditions are refusals of the same overflow, and the sieve narrows *neither* of its
loops because its bound is `i*i >= n` and its step is `+i`. The justification is a PLATFORM fact —
a Java array holds at most 2³¹−1 elements — not an inferred range, which is why it did not need
`emit/interval.go`. Go, JavaScript and x86 are unchanged: the cost existed on exactly one host.

**A declared range selects the representation** —
[selection-2026-08-19](gauntlet/results/selection-2026-08-19.md). An integer operation the compiler
can bound keeps the host's own operator; one it cannot is rewritten to the `checked` primitive the
target declares. On `examples/native/sieve-go.oro`, adding one `(where …)` to the existing `sig`
takes it from **10 of 10 operations checked and 0 of 3 loops proven terminating** to **0 checked and
3 of 3**. Nothing else in the program changes. Java uses `Math.addExact`, x86 uses `jno`/`ud2`, Go
uses an immediately-called func literal, and **JavaScript declares none** — which is the price list
of [overflow-2026-08-19](gauntlet/results/overflow-2026-08-19.md) showing up as a capability.

**The corpus is `examples/int/`, and three of its six programs are meant to be REFUSED** — Collatz
(the conjecture is open), Fibonacci past 2⁵³, and exponentiation whose accumulator genuinely
overflows. A proof system is worth what it declines to prove. Growing it found four gaps invisible
on the sieves, including that `go./` was not recognised as division at all.

**Termination is computed now, not guarded by fuel** —
[sct-2026-08-19](gauntlet/results/sct-2026-08-19.md). Size-change termination (Lee, Jones &
Ben-Amram 2001) plus a trip count closed both open holes at once: **every integer operation in
every program is provably inside the portable window** once one range is declared, and **96% of
loops are proven to terminate**. The single refusal is a true negative — Newton's method on a
float. Two additions were needed that the paper does not have: integers are not well-founded, so
the **floor comes from the interval analysis** and is demanded of the *witness*; and an ascending
counter is handled by **orientation**, μ = −v, which is a ranking function as a change of variable.
Three bugs found on the way all made the numbers *worse* than the truth, which is the only reason
they were not mistaken for results.

**The type-system question is reopened, and the literature answers it** —
[types-direction.md §6](docs/types-direction.md). Three findings to carry. The **justification
moved**: a range does not need to transfer to the host, it changes what we *emit*, which is
categorically unlike the bounds-check case §1–2 refuted. A range is **already writable** —
`(sig f ((n int)) int (where …))` parses today — and reading it takes the sieve from **45% to 90%**
provable ([intervals-2026-08-19](gauntlet/results/intervals-2026-08-19.md)) with no language
change. And on sequent calculus specifically: GHC built Sequent Core, measured it, and shipped
**join points in direct-style Core** instead — and `again` already *is* a join point. What sequent
calculus does give us is **polarity**: a product eliminated by projection need never be built,
which is exactly why the product measured free. Take the classification, not the core.

**The primary data structure is SPECIFIED and not yet built** —
[tables.md](docs/spec/tables.md). One concept: **a table is a function with a known finite domain**,
and **types are functions; the domain is what varies**: `(fn (A) B)` is total on A, `(array V)` is
`(fn (int) V)` on `[0, len)`, `(map K V)` is `(fn (K) V)` on the keys present. Three points on one
scale. So **a bounds check is a domain condition** and Go's comma-ok is the same condition — and
the dependency rides in the **refinement** layer that already exists, which is simple types plus a
decidable constraint domain, which is **Dependent ML**, which types-direction §6.5 had already
named as our lineage from a measurement. Surface: `(array e…)` a graph, `(table n f)` a rule with
no memory, `(alloc t)` the one construct that allocates, `(len t)`, and **indexing is APPLICATION**.
`vec`/`vector` were rejected — every mainstream `vector` is a growable mutable memory-owning
container and ours is the opposite; `materialize` lost to `alloc`, which says where the money goes
in a word everyone has. **β-tab is a second CLAUSE of β**, not a fourth rule — a rule is an
intensional presentation of a function and a graph an extensional one — so it needs no constant
folder and comes first. **Mullin's Psi Correspondence Theorem is β**: MoA needs an index calculus
because APL arrays are data, ours are functions, so composing index maps is composing functions.
The leading-axis rule is currying, and MoA's DNF/ONF split is our residual/emission split. And
**automatic unrolling is deferred, NOT refuted** — the measurement only covered cheap elements, the
win scales as compile-cost over artifact-cost, and ADR 0009 bites exactly where the win would be
because transcendentals are not bit-reproducible. **AND ADR 0018 WAS RE-EXAMINED AND STANDS — but trigger 2 has FIRED** —
[arrays-revisited.md](docs/arrays-revisited.md), on hamza's *"are our decisions still sound... this is
oroboros after all, and sometimes we eat our own tail."* The ADR was **argued, not measured**, and says
so; this points everything measured since at it.

**Trigger 1 — the one the ADR says to watch — has NOT fired**, and it was tested rather than argued.
It says it can only be found by writing awkward programs, so **Karatsuba's structural core was written
in Oroboros** ([examples/kara/core.oro](examples/kara/core.oro)): one arena, a descriptor table driving
computed offsets, three live buffers, a buffer crossing a binder, and **read AND write of one buffer at
two overlapping computed offsets** — the tiled combine's exact shape. `occurrences` accepted all of it.
What complained was the **refinement layer**, on `(desc 2)` being a value read out of a table — stratum
0, ⊤ — which is `tree.oro`'s `d` gap and which frozen-2026-08-28 already says an octagon would not fix.
**Conflating the memory model with the analysis would have given a false verdict in either direction.**
And **reuse inside a program is free as claimed**: two multiplies threaded through one workspace emit
**one** `make`.

**TRIGGER 2 HAS FIRED, AT 1.07×–1.66×.** A buffer is not a nameable type — there is no `buffer` type
name in the compiler — so an exported multiply must rebuild its workspace every call: **1024 limbs
D=5, 721,064 ns reused against 1,195,545 fresh, 10 allocs and 504 KB per call.** That matters more
than the stencil's larger number, because bigarith's whole result is that ours beats `math/big` **by
allocating nothing**, and naive `math/big` is 4–5× worse than careful for exactly this reason — so a
bignum that allocates per multiply **becomes the thing it beats**. ADR 0018 deferred uniqueness types
saying *"no measured case in this repository needs it"*; **that sentence is now false** and the
counterexample is Karatsuba. Its named answer is an ADR adopting **uniqueness on parameters**, not free
mutation.

**SEVEN PROPERTIES NOW DEPEND ON LINEARITY, four measured AFTER the ADR**: the buffer element
theorem's *sufficiency* (no third source, so checking the stores suffices), the frozen-read
**stratification**, **β's substitution of a table** (free mutation makes every read impure and
substitution meaning-changing — a soundness loss in the reducer, not a slowdown), η-tab, `modifies`
being **syntactically** the buffer (HACL\*'s largest proof burden, for free), race-freedom, and the
parallel/sequential distinction being **in the source**. So the decision is better supported now than
when it was made.

**And the proposal does not survive being made precise.** The load-bearing axis is **aliased-vs-not**,
not mutable-vs-immutable: there are exactly three ways to forbid aliased mutation — immutability
(cannot express scatter), linearity (ours), ownership+borrowing (Rust). *Mutable by default* without
one of them is ADR 0018 §(f); **with** one it is either what we already have or it is Rust. **Free
mutation is strictly dominated by uniqueness types** — it buys one thing (b) also buys and pays seven
(b) does not. The one honest remaining cost is **ergonomics** — threading the buffer through every
`again` and every helper — which is unmeasured, is a different complaint from trigger 1, and probably
wants sugar rather than a memory model.

**And it settles the map's open question by DERIVATION rather than choice**: a map is both, on exactly
the same terms as an array, **because the discipline is about aliasing and aliasing does not care what
the index set is**. Growing map = linear buffer in `build`; frozen map = immutable value. And **Go's
map is a reference type**, so `(set-map m k v)` returning `m` IS Go's own semantics — the host that
looks least like our model needs the least translation, which is the surprise `(T, error)` gave sums.

**The memory model is DECIDED** —
[ADR 0018](docs/decisions/0018-immutable-values-linear-buffers.md), research in
[memory-model.md](docs/memory-model.md). Values are immutable; mutation exists only inside
`(build n (fn (b) …))`, whose buffer is **linear** and frozen on the way out; `(array V)` reads are
pure and `(buffer V)` reads are impure; and **the linearity check is `occurrences` on the residual,
not a type** — uniqueness never enters a signature. What decided it was **expressiveness, not the
2.7x**: `(table n f)` is a *gather* and cannot express a *scatter*, so the sieve, sorting,
histograms, union-find and general DP are inexpressible portably **at any speed** —
`examples/native/sieve-go.oro` is in this repo and could not be written portably. It costs almost
nothing because every mechanism exists already: the heap is acyclic (ADR 0014), a buffer cannot
escape (closures are refused — the only thing Haskell's rank-2 `runST` prevents), it is lexically
local in the residual (whole-program reduction), `occurrences` is in the reducer, and **ADR 0010
already sequences stores** by never substituting an impure argument, denying contraction, weakening
and exchange — the three properties a buffer needs, built for `print-line`. This **fires ADR 0013's
fifth trigger**: η-tab is now sound. Uniqueness types on *parameters* (Futhark, Cogent — the two
languages with our exact constraints, which both chose them) are **deferred with a named trigger**,
because reduction removes every non-exported boundary so buffer reuse already works inside a
program. Reclamation is a **target** decision: three hosts bring collectors, and windows gets a
lexical arena or Perceus, which is available there precisely because we own that allocator.

**The older framing, kept because the reasoning is what changed**: a table is a function from a
finite index set,
and the length is what makes it more than a function. Three constructors — `(array e…)` a graph,
`(vec n f)` a rule with no memory, `(materialize t)` the one construct that allocates — plus
`(len t)`, and **indexing is APPLICATION**: `(a i)`, not `(at a i)`. That is unambiguous by a
checked invariant — in a residual the operator of an application is never a variable, because a
surviving lambda is a refused closure — so the slot is empty and `(a i)` is an indexing before any
type is consulted. `(array T)` exists only in the **signature** language and is erased by staging,
because a dynamic index forces homogeneity and reduction removes every static one, so the checker
only ever sees `Fin n → V` and **no dependent types are needed**. No target declares any of it:
the backends implement it like `if`/`let`/`loop`. It **deletes** surface — Go's 7 array types and
14 `at-`/`make-`/`set-` names, Java's set, and the portable layer's `alen`/`aindex`/`slen`/`sat`,
which never covered int or bool because it *enumerated* element types instead of having a
constructor. Bounds are a **precondition, not a behaviour** (JS returns `undefined` silently).
Reuse is deliberately absent and stays ADR 0013's question.

**The data-structure question is researched and NOT decided** —
[docs/data-structures.md](docs/data-structures.md). Three things to carry. The array is primary
**because a list's algebra needs recursion**, not because a list is slow: `foldr` *is* the list's
universal property, ADR 0014 removed recursion, so a cons chain here would be an array with worse
constants and no compensating law — *a language's iteration construct selects its data structure*.
The product is **not a new kind of thing**: `(vec n f)` and `(fn (sel) (sel x y))` are the same
construct at different index sets, so `materialize` and reifying an escaped product are one
operation, and TLA+, containers, Naperian functors, Dex and SML's `(a,b) = {1=a,2=b}` all say the
same thing — *everything is a function from an index set*. And the recommendation is **multiple
return values, not tuples**: all six measured demands are multiple-return, Scheme and CL made that
exact distinction on purpose, and three of our four targets have a native form. Two results here
were rediscovered independently before being read: the delayed vector is a **container**
(Abbott/Altenkirch/Ghani) and Repa's `Delayed`, and [q5b](docs/spec/q5b-filter.md)'s pull/push
duality is Obsidian's.

**§8 is the literal table, and it is the missing dual** — `(array 1 2 3)` with application as
indexing. `tab`/`idx` are mutually inverse, which IS the Naperian isomorphism written at the term
level, and β-tab is **the extensional counterpart of β**: a function given by its graph rather than
by a rule. It costs no new term kind and no new reduction rule *if constant folding is built*,
since `((array 1 2 3) 1) → 2` is one entry in the same table as `(go.+ 1 2) → 3`. A **dynamic index
forces homogeneity and forces existence, and it is the same condition doing both** — so the
dependent type is erased by staging and the checker only ever sees `Fin n → V`. **The memory half is REFUTED** —
[staticdata-2026-08-20](gauntlet/results/staticdata-2026-08-20.md). Compile-time materialisation
into static data is free of *code* on x86 and Go, a **pure loss** on Java (256 `iastore` in
`<clinit>`) and JavaScript (**3.5x slower to load, 2,600x larger source**), and never a measurable
win — even on Go a 65,536-entry table saves 0.2 ms against 9 ms of process creation and costs
exactly its own size in the binary. So `unroll` should not be built, D-K is demoted to a syntax
question, and **the next build is multiple return values.** And **η-tab is a law we can state and are not
allowed to apply**: `materialize (of-array a) = a` is true of values and unsound with mutation,
which is ADR 0013's fifth trigger.

**Types are not in the language and that is measured, not assumed** — `targets/js.oro` declares
zero types because JS needs none. A type system is *wanted eventually*, and
[docs/types-direction.md](docs/types-direction.md) records the direction and the one measurement
that constrains it: **our proofs do not transfer** — a proof buys nothing unless the emitted code is shaped so the
host re-proves it. That win has been **collected as an emitter pattern**, needing no types at all
([bce-2026-08-15](gauntlet/results/bce-2026-08-15.md)): 1.96× on compute-bound loops, and
**nothing** on memory-bound ones, which is the condition the earlier "1.94×" left off.

## Working conventions

**Every significant decision gets an ADR.** Numbered, in `docs/decisions/`, using the template
in that directory's README. The "Why not" section is the point — this project is deliberately
put down at dead ends and picked up later, and the rejected alternatives are what will not be
recoverable from the code.

**Reversing a decision means a new ADR that supersedes the old one.** Mark the old one
`Superseded by NNNN`. Do not edit decision history.

**Keep documents current rather than accumulating drafts.** A stale design document is worse
than no design document. `docs/design-direction.md` was rewritten, not appended to, when the
Parasite reframing invalidated part of it.

**Tables are on ALL FOUR TARGETS** —
[wintables-2026-08-25](gauntlet/results/wintables-2026-08-25.md). A table on windows is ONE
REGISTER — a pointer whose first eight bytes hold the length — because a fat pointer needs two and
this convention passes one value per register, so a table would stop being a *value*; the header is
free to skip because the displacement is part of x86's addressing mode. The **allocator is the
target's**, found the way `findEq` finds equality, so `targets/windows/` needed no new declaration.
Reclamation is neither of ADR 0018's suggestions — one `VirtualAlloc` per `alloc`, never freed —
and changing that is a target-file edit, not a compiler one.
**It started at 3.7× of hand-written assembly and ends at 0.88×** — faster than hand-written and
faster than the target-native form. Two costs, both invisible on the other three hosts. **Element
size is part of the type**: one byte for a bool against eight, carried BY NAME because a table
crosses binders, and losing it at one binder reads a byte array as qwords — a wrong answer, not a
slow one. Then **a threaded buffer must not cost a register**: `(again (set c j v) …)` hands back
what it was given — ADR 0018's linearity is what makes that reliable — so the variable keeps the
place it already has, aliased rather than taken. Without it the inner loop's index was spilled and
reloaded three times per element. That is ADR 0016's lesson twice over: *the optimisations you were
parasitizing only become visible on a host that has none* — this host has neither a type system to
size our elements nor a register allocator to spill for us. Also built and REVERTED: fusing a
byte-table guard into the compare, which measured indistinguishable because the loop is
memory-bound (bce-2026-08-15 again), with the measurement left in the comment. It also found
three pre-existing holes, all unreachable until indexing became structural here: the refinement
layer knew **none of x86's ordering comparisons**, `imul` was **not multiplication** (the `go./`
bug on a third operator), and the table operations built an addressing mode without materialising a
spilled operand.

**A target need not have expressions** — [ADR 0016](docs/decisions/0016-targets-need-not-have-expressions.md).
`targets/windows/` emits x86-64 assembly under MASM and reaches **parity with hand-written
assembly** ([windows-2026-08-19](gauntlet/results/windows-2026-08-19.md)), with the structural set
still three. The format gained `%r`, `%u`, `(jump …)` and `(data …)` and lost nothing. Two things
to carry forward: a template may not touch r10, r11, xmm4 or xmm5, and **the optimisations you
were parasitizing only become visible on a host that has none** — the first three hosts were doing
common-subexpression elimination for us and nothing noticed until one did not.

**The gauntlet runs on the native Go target now** —
[native-gauntlet-2026-08-20](gauntlet/results/native-gauntlet-2026-08-20.md). All six programs
moved off `num/vec`/`num/int`/`num/f64`/`io`, all at parity. Two things the move deleted and one it
added: **`fold-range2` is gone** — it existed only because two accumulators had no product to pair
them with, and `loop` has n variables and no product at all — and the native Go target **had no
string surface at all** until `wordcount` asked for one (`targets/go/strings.oro`, 25 primitives,
no compiler change). What it added is `(length N)`, a declared primitive attribute saying which
argument decides the result's length; the sieve's `c[i]` is **proven** now rather than propagated.
It is **two** attributes — `(length N)` for a count and `(length-of N)` for a pass-through — because
reading it off the argument's declared type broke on the first target that tried it: `targets/js/`
declares everything `any`. And a **JavaScript array store is a map insert** — `a[10] = x` on a
three-element array extends it — so `js.set` declares neither, which is `go.set-map`'s refusal
arriving from the opposite direction.
**JavaScript is done too** — [native-js-2026-08-20](gauntlet/results/native-js-2026-08-20.md).
All six at parity there as well, and the hostile host earned its reputation: a `loop` in **tail
position** was lowering to a result variable plus `break`, which costs **1.31x on V8** against an
early `return` and **nothing on Go** — five months of Go measurements could not see it. It also
found two benchmark-method errors that had been inflating every JS number here: reaching a function
through a module namespace object (`g.dotTyped`) is **1.66x** slower than a named import, and the
old harness did that on the hand-written side only; and V8 carries optimization state across
benchmarks, so each comparison now gets its own process. **Java has not moved.**

**Two backends before front-end features.** Once the Go backend works, build the JavaScript
backend before adding anything to the language. JS is the most hostile host in the set and
surfaces core flaws while they are still cheap to fix.

## Build commands

Module path is `oroboros` (local). Change it when the repository gets a home.

```bash
go test ./core/ ./emit/          # the compiler
go test ./core/ -run TestBeta    # one test
go vet ./...
```

```bash
go run ./cmd/build -target=portable-go -o hello examples/hello.oro   # a real binary
go run ./cmd/oro -target=portable-go examples/dot.oro   # reduce to normal form
go run ./cmd/oro -target=blas -steps examples/dot.oro
```

```bash
# emit into the gauntlet, then benchmark generated against hand-written
go run ./cmd/gen examples/dot.oro portable-go gauntlet/go/generated_dot.go
go run ./cmd/gen examples/dot.oro js   gauntlet/js/generated_dot.mjs
go run ./cmd/gen examples/dot.oro java gauntlet/java/gen/GenDot.java
cd gauntlet/go && go test -bench='SmallDot|SmallGenDot' -benchtime=3s -count=5
```

The target file format is specified in [docs/spec/target-files.md](docs/spec/target-files.md) —
it is the file a third party writes, so it is the one that most needs to be a specification rather
than a comment.

A **doctor** — reporting which toolchains a target needs, which are installed, and what is
missing — is wanted and deliberately not built yet
([build.md §6](docs/spec/build.md)). It can only diagnose requirements that exist, and what a
target must declare about its toolchain should be read off what builds turn out to need.

**Primitives are declared in `targets/*.oro`, not in Go.** If you find yourself adding a case to
`emit/*.go` for a host function, that is the wrong place — only *structural* primitives (loops,
conditionals, bindings) live in code.

The gauntlet (`gauntlet/go`, `gauntlet/js`, `gauntlet/java`) and `experiments/legibility` are
**separate modules** — `cd` into them before running their tests.

`gauntlet/fmt/*.go` carry `//go:build ignore`; they are standalone scripts run with `go run`.

## What exists

| | |
|---|---|
| `core/` | reader, terms, β/δ reducer. The atom of [core-0](docs/spec/core-0.md). |
| `emit/` | Go, JavaScript, Java and x86-64 backends. Types live here, **not** in the language. |
| `targets/` | Target declarations — **data, not Go**. `go/`, `js/`, `java/` and `windows/` are **directories**, host-native, no portability claim ([target-native.md](docs/spec/target-native.md), [windows-target.md](docs/spec/windows-target.md)); the `portable-*.oro` files are the layer they replaced, kept for the gauntlet. |
| `cmd/oro` | reduce a file to normal form against a target |
| `cmd/gen` | emit a file into the gauntlet's Go package |
| `cmd/build` | follow imports, reduce `main`, emit a program, run the host toolchain |
| `examples/` | twelve programs; `smooth.oro` completes the gauntlet |
| `lib/` | modules a program imports by `(use …)`; resolved on a search path |
| `gauntlet/` | hand-written references and results — the bar |

**Both emitted programs reach parity with hand-written Go.** See
[parity](gauntlet/results/parity-2026-08-14.md).
