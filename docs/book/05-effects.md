# 5. Effects

Four bricks so far: a [function](01-fn.md), a [name](02-def.md), a [place](03-modules.md) for the
name, and a [choice](04-choice.md). Every one of them was explained by saying what the compiler
may do to a program: replace a parameter by its argument, a name by its definition, a choice whose
condition is known by its branch. Each time the claim was that the program *means the same* after
the step as before it.

That claim has a condition, and chapter 1 stated it in one section (§1.11) and moved on: the
replaced thing must do nothing but have a value. This chapter is the fifth brick, and it is not a
new form. **There is no effect form**: no `IO`, no `do`, no `async`, nothing to import. There is
one word in a target's file, and a rule about what reduction may not do to an expression that
reaches it.

The ideas met on the way are among the oldest in the subject: *equals may replace equals*
(Leibniz, and in programming Strachey, 1967), the three *structural rules* of logic (Gentzen,
1934), the *thunk* (Ingerman, 1961), and the question every language answers, usually without
saying so: in what order do things happen?

The examples are in [code/effects/](code/effects/). Most run on the `tutorial` target, where
`f`, `g`, `h` have no effect and `shout`, `whisper`, `tick` have one; the last sections compile
real programs that print. `go run docs/book/check.go docs/book/05-effects.md` re-runs every block.
Checked 2026-11-15.

---

## 5.1 A value, and something else

<!-- file docs/book/code/effects/hello.oro -->
```go
use io

export fn main() {
    io.print_line("one")
    io.print_line("two")
}
```

<!-- run go docs/book/code/effects/hello.oro -->
```
one
two
```

Ask what `io.print_line("one")` *is*. In chapters 1 to 4 the answer to that question was always a
value: `3 < 4` is `true`, `h(1, 2)` is whatever `h` gives for 1 and 2. This call has a value too
(it is the string it was given, §5.12). But nobody wrote it for its value. It was written because,
when it is evaluated, a line appears on a screen.

> **An effect is anything an expression does besides having a value.** Printing is one. So is
> reading a clock, reading a key, writing a file, sending a packet.

A function in mathematics has no effects. `sin` does not do anything; it *is* a set of pairs, and
`sin(0)` is `0` today and tomorrow. A function with an effect is a different kind of thing, and
the simplest way to see the difference is to try to replace it by its value.

## 5.2 Equals for equals

`f` is a function with no effect, and `tick` reads a clock that advances each time it is read.
Each is written once and its value used twice, and then written twice:

<!-- file docs/book/code/effects/same.oro -->
```go
export fn pure_named() { t = f(1); t - t }

export fn pure_written_twice() { f(1) - f(1) }

export fn clock_named() { t = tick(); t - t }

export fn clock_written_twice() { tick() - tick() }
```

<!-- oro tutorial docs/book/code/effects/same.oro -->
```go
pure_named =
fn() { t = f(1); t - t }
pure_written_twice =
fn() { f(1) - f(1) }
clock_named =
fn() { t = tick(); t - t }
clock_written_twice =
fn() { tick() - tick() }
```

The first two are the same program. Whatever `f(1)` is, it is that both times, so both are zero,
and a compiler may turn either into the other. It may share the work (the first form) or, if `f`
is cheap, repeat it (the second).

The last two are different programs. `clock_named` reads the clock once and is zero.
`clock_written_twice` reads it twice and is not. A compiler that turned one into the other would
change the answer.

> **Equals for equals.** If two expressions have the same value, one may replace the other
> anywhere. Leibniz stated it for logic; Strachey (1967) brought it to programming under Quine's
> name for it, **referential transparency**. It holds for an expression with no effect, and fails
> for one with an effect, because such an expression is not only its value.

Every step of reduction in chapters 1 to 4 was a replacement of equals by equals. So the whole
question of this chapter is: **what does reduction do when it meets an expression for which that
law fails?**

## 5.3 One word, in the target's file

First, how does the compiler know? Here are six declarations from
[targets/tutorial.oro](../../targets/tutorial.oro):

<!-- part targets/tutorial.oro -->
```go
fn f(a ∈ num) (num) pure { host expr "f(${a})" }
fn g(a ∈ num) (num) pure { host expr "g(${a})" }
fn h(a ∈ num, b ∈ num) (num) pure { host expr "h(${a}, ${b})" }
```

<!-- part targets/tutorial.oro -->
```go
fn shout(a ∈ num) (num) { host stmt "shout(${a})" }
fn whisper(a ∈ num) (num) { host stmt "whisper(${a})" }
fn tick() (num) { host stmt "tick()" }
```

One word. `f` has `pure` and `shout` does not. A target is the list of operations the host
provides ([chapter 3](03-modules.md)), and whoever writes the list says, for each operation,
whether it is a function in the mathematical sense. Nothing else in the language knows about
effects: not the reader, not the reducer's terms, not the sets in a header. `shout` takes a `num`
and gives a `num`, exactly as `f` does.

Three things follow, and each gets a section later:

- **The word belongs to a host operation, and a program cannot write it** on its own functions.
  Whether a function you define has an effect is computed from what it calls (§5.10).

<!-- file docs/book/code/effects/no-pure.oro.txt -->
```go
fn mine(n) pure { f(n) }

export fn run() { mine(1) }
```

<!-- oro tutorial docs/book/code/effects/no-pure.oro.txt -->
```
oro: docs/book/code/effects/no-pure.oro.txt: line 1: "pure" where "{" was expected
```

- **An operation with no word has an effect.** The default is the careful one (§5.13).
- **The word is believed.** A target that calls a printing operation `pure` is wrong the way a
  target that spells `+` as `-` is wrong, and the compiler can check neither.

## 5.4 Three things reduction does, and may not do here

Chapter 1's rule for an application was: put the argument where the parameter is. Look at what
that does to the argument, depending on how many times the parameter appears and where.

<!-- file docs/book/code/effects/three.oro -->
```go
export fn dropped() { fn(n) { h(1, 2) }(f(9)) }

export fn not_dropped() { fn(n) { h(1, 2) }(shout(9)) }

export fn shared() { fn(n) { h(n, n) }(shout(9)) }

export fn moved() { fn(a, b) { h(b, a) }(f(1), g(2)) }

export fn not_moved() { fn(a, b) { h(b, a) }(shout(1), whisper(2)) }
```

<!-- oro tutorial docs/book/code/effects/three.oro -->
```go
dropped =
fn() { h(1, 2) }
not_dropped =
fn() { n = shout(9); h(1, 2) }
shared =
fn() { n = shout(9); h(n, n) }
moved =
fn() { h(g(2), f(1)) }
not_moved =
fn() { a = shout(1); b = whisper(2); h(b, a) }
```

- `dropped`: the parameter appears **no** times, so the argument `f(9)` is gone. Nobody needed it.
- `not_dropped`: the same function, and `shout(9)` is still there. It was bound to a name nobody
  reads, so that it happens.
- `shared`: the parameter appears **twice**. Writing `shout(9)` in both places would shout twice.
  It is bound once and the name is used twice.
- `moved`: `f(1)` was written first and `g(2)` second, and in the result `g(2)` stands on the
  left. The arguments went where the body wanted them.
- `not_moved`: the same body, and `shout(1)` is still before `whisper(2)`, whatever the body's
  order.

So substitution can do three things to an argument, and for an argument with an effect each one
is a different wrong answer:

| if the parameter appears… | substituting would… | and the program would… |
|---|---|---|
| no times | delete the argument | not do something it says |
| twice | copy the argument | do something twice |
| after another argument's | reorder the two | do things in the wrong order |

These three have had names since 1934, when Gentzen wrote down the rules of logical deduction and
found that three of them were not about any connective. They were about the *hypotheses*
themselves, and he called them **structural**:

> **weakening**: a hypothesis you do not use may be discarded.
> **contraction**: a hypothesis used twice need be stated only once.
> **exchange**: the order of hypotheses does not matter.

Read "hypothesis" as "argument" and those are `dropped`, a copy, and `moved`. Ordinary
mathematics assumes all three without noticing, since they are true of values. An effect is
exactly the thing they are false of. So the rule of this chapter is one sentence:

> **An expression with no effect may be dropped, copied and moved. An expression with an effect is
> evaluated exactly once, where it was written, in the order it was written.**

The next three sections take the three clauses one at a time.

## 5.5 Dropping: only what does nothing

<!-- file docs/book/code/effects/drop.oro -->
```go
export fn all_pure() { a = f(1); b = g(a); c = h(a, b); 7 }

export fn one_effect() { a = f(1); b = shout(a); c = g(b); 7 }

export fn unused_and_first() { fn(a, b) { b }(shout(1), whisper(2)) }

export fn clock_unread() { fn(t) { 7 }(tick()) }
```

<!-- oro tutorial docs/book/code/effects/drop.oro -->
```go
all_pure =
fn() { 7 }
one_effect =
fn() { b = shout(f(1)); 7 }
unused_and_first =
fn() { a = shout(1); b = whisper(2); b }
clock_unread =
fn() { t = tick(); 7 }
```

- `all_pure`: three bindings, each feeding the next, and the result reads none of them. All
  three are gone. Removing work whose result nobody reads is called **dead-code elimination**,
  and for expressions with no effect it is always right.
- `one_effect`: `c = g(b)` is gone. `b = shout(a)` stays, since it has an effect, and it keeps
  alive exactly what it needs: `f(1)`, which moved inside it. Nothing else survives.
- `unused_and_first`: `a` is never read, and `shout(1)` is kept *and kept first*.
- `clock_unread`: reading a clock and ignoring the answer still reads the clock. The compiler
  cannot know that `tick` only looks; the target said it is not a function, and that is all it
  said. (On a real machine a read can matter: reading a key takes it out of the queue.)

Dead-code elimination is one of the first things any optimizing compiler does. Here it is not a
separate pass. It is β on a parameter that appears no times, with one condition.

## 5.6 Copying: purity is not cheapness

<!-- file docs/book/code/effects/copy.oro -->
```go
export fn a_literal() { fn(n) { h(n, n) }(5) }

export fn a_name(v) { fn(n) { h(n, n) }(v) }

export fn costly() { fn(n) { h(n, n) }(f(9)) }

export fn loud() { fn(n) { h(n, n) }(shout(9)) }

export fn costly_once() { fn(n) { h(n, 2) }(f(9)) }

export fn loud_once() { fn(n) { h(n, 2) }(shout(9)) }
```

<!-- oro tutorial docs/book/code/effects/copy.oro -->
```go
a_literal =
fn() { h(5, 5) }
a_name =
fn(v) { h(v, v) }
costly =
fn() { n = f(9); h(n, n) }
loud =
fn() { n = shout(9); h(n, n) }
costly_once =
fn() { h(f(9), 2) }
loud_once =
fn() { n = shout(9); h(n, 2) }
```

`costly` and `loud` came out the same, bound once and read twice, for **two different reasons**:

- `shout(9)` is bound because copying it would be *wrong*.
- `f(9)` is bound because copying it would be *slow* (§1.11): the same answer, computed twice.

The last pair tells them apart. Used once, `f(9)` goes straight to where it is used, since
nothing is gained by naming it. Used once, `shout(9)` is still bound where it was written. And a
literal or a name is copied freely: copying `5` costs nothing and repeats nothing.

| the argument | may be copied | may be moved or dropped |
|---|---|---|
| a literal, a name, a function | yes | yes |
| no effect, but work: `f(9)` | no, it would be slower | yes |
| an effect: `shout(9)` | no, it would be wrong | no |

Two questions, then, and it is worth keeping them apart: *is it safe to move this?* and *is it
cheap to repeat this?* A compiler that confused them would either repeat effects or refuse to
copy a `5`.

## 5.7 Moving: the order is the one you wrote

<!-- file docs/book/code/effects/order.oro -->
```go
export fn mixed() { fn(a, b, c) { h(a, h(b, c)) }(shout(1), f(2), whisper(3)) }

export fn as_written() { fn(a, b) { h(a, b) }(whisper(2), shout(1)) }

export fn in_place() { h(shout(1), whisper(2)) }

export fn operands() { shout(1) + whisper(2) }

export fn inner_first() { h(f(shout(1)), g(whisper(2))) }
```

<!-- oro tutorial docs/book/code/effects/order.oro -->
```go
mixed =
fn() { a = shout(1); c = whisper(3); h(a, h(f(2), c)) }
as_written =
fn() { a = whisper(2); b = shout(1); h(a, b) }
in_place =
fn() { h(shout(1), whisper(2)) }
operands =
fn() { shout(1) + whisper(2) }
inner_first =
fn() { h(f(shout(1)), g(whisper(2))) }
```

- `mixed` is the whole discipline in one line. Three arguments. The two with effects are pinned,
  in order. `f(2)`, written *between* them, has moved past `whisper(3)` into the body, because
  nothing can tell where it was computed. One rule, decided argument by argument.
- `as_written`: the order is the order of the *arguments*, left to right, here `whisper` and then
  `shout`. It is not alphabetical and not the body's.
- `in_place`, `operands`, `inner_first`: nothing was bound at all. These are arguments of a host
  operation, not of a function of yours, so there is no parameter to substitute and nothing to
  move. They stay where they are, and they are evaluated **left to right, an argument before the
  call it is an argument of**, on every target.

That last promise is one many languages do not make. In C, `f() + g()` may call either first, and
`printf("%d %d", next(), next())` has printed both orders on real compilers; the standard leaves
the order *unspecified* so that each machine may choose the faster. That was a reasonable trade in
1972 and has been a source of bugs since. Java and JavaScript fix the order, left to right, and Go fixes it for calls.
This language fixes it too, and since the compiler is free to move everything that has no effect,
fixing it costs nothing.

## 5.8 Where an effect stays: where it was written

"Where it was written" is doing real work. A parameter's uses can be anywhere in a body: inside
one branch of a choice, for instance. Putting the effect where the parameter is *used* would move
it into the branch.

<!-- file docs/book/code/effects/site.oro -->
```go
export fn before_the_choice(c) { fn(n) { if c { n } else { 0 } }(shout(5)) }

export fn pure_goes_in(c) { fn(n) { if c { n } else { 0 } }(f(5)) }

export fn a_block_too(c) { n = shout(5); if c { n } else { 0 } }

export fn written_inside(c) { if c { shout(1) } else { whisper(2) } }

export fn in_the_condition() { if shout(1) < 2 { f(1) } else { g(1) } }

export fn known() { if 1 < 2 { shout(1) } else { whisper(2) } }
```

<!-- oro tutorial docs/book/code/effects/site.oro -->
```go
before_the_choice =
fn(c) { n = shout(5); if c { n } else { 0 } }
pure_goes_in =
fn(c) { if c { f(5) } else { 0 } }
a_block_too =
fn(c) { n = shout(5); if c { n } else { 0 } }
written_inside =
fn(c) { if c { shout(1) } else { whisper(2) } }
in_the_condition =
fn() { if shout(1) < 2 { f(1) } else { g(1) } }
known =
fn() { shout(1) }
```

- `before_the_choice`: `n` is used in one branch only. The shout was written outside the choice,
  so it happens whether `c` is true or not, and it stays outside.
- `pure_goes_in`: the same program with `f(5)`. It moved *into* the branch, and now `f(5)` is
  computed only when `c` is true. Nothing can tell, and work was saved.
- `a_block_too`: a block is a function applied (§1.12), so `n = shout(5)` on its own line is the
  same program as the first and reduces to the same thing.
- `written_inside`: the reverse case. An effect written inside a branch stays inside it. One of
  the two happens, never both: that is what a choice is (§4.2).
- `in_the_condition`: a condition may have an effect. It is evaluated once, before either branch.
- `known`: the condition is known, so one branch is kept (§4.11) and the other is deleted, with
  its `whisper`. That is no violation of anything. The whisper was never going to happen; the
  compiler removed code that could not run, not an effect that would have.

### `&&` and `||` skip effects, as they skip everything

Chapter 4 said `a && b` is `if a { b } else { false }`. So its right side is inside a branch:

<!-- file docs/book/code/effects/short.oro -->
```go
export fn skips(c) { c && shout(1) < 2 }

export fn does_not_skip(c) { p = shout(1) < 2; c && p }
```

<!-- oro tutorial docs/book/code/effects/short.oro -->
```go
skips =
fn(c) { c && shout(1) < 2 }
does_not_skip =
fn(c) { p = shout(1) < 2; c && p }
```

In `skips` the shout happens only when `c` is true. If you want it to happen regardless, say so
by writing it first, as `does_not_skip` does: the binding is where it happens. Here it is on a
real machine, with a function that prints its tag and returns its second argument:

<!-- file docs/book/code/effects/trace.oro -->
```go
use io

fn probe(tag ∈ String, v ∈ Bool) (Bool) { io.print_line(tag); v }

export fn main() {
    if probe("a", false) && probe("b", true) { io.print_line("both") } else { io.print_line("not both") }
    if probe("c", true) || probe("d", true) { io.print_line("either") } else { io.print_line("neither") }
}
```

<!-- run go docs/book/code/effects/trace.oro -->
```
a
not both
c
either
```

`b` and `d` were never printed. `a` was false, so `&&` had its answer; `c` was true, so `||` had
its own.

## 5.9 A function is a value, whatever is in it

<!-- file docs/book/code/effects/value.oro -->
```go
fn ignore_it(a) { h(1, 2) }

fn twice(k, v) { h(k(v), k(v)) }

fn pick(c, a, b) { if c { a } else { b } }

fn pick_later(c, a, b) { if c { a() } else { b() } }

export fn never_applied() { ignore_it(fn(v) { shout(v) }) }

export fn applied_twice() { twice(fn(v) { shout(v) }, 3) }

export fn both_run(c) { pick(c, shout(1), whisper(2)) }

export fn one_runs(c) { pick_later(c, fn() { shout(1) }, fn() { whisper(2) }) }
```

<!-- oro tutorial docs/book/code/effects/value.oro -->
```go
never_applied =
fn() { h(1, 2) }
applied_twice =
fn() { h(shout(3), shout(3)) }
both_run =
fn(c) { a = shout(1); b = whisper(2); if c { a } else { b } }
one_runs =
fn(c) { if c { shout(1) } else { whisper(2) } }
```

- `never_applied`: a function whose body shouts was passed, never applied, and **dropped**.
  Nothing was lost. Writing a function runs nothing.
- `applied_twice`: the same function was **copied**, and there are two shouts. That is right:
  `twice` applies it twice. The function was copied; each *application* happens once.

> **A function is a value.** Its effects happen when it is applied, not when it is written. So a
> function may always be dropped, copied and moved, whatever its body does.

- `both_run`: `pick` looks like `if`, and is not. Its arguments are evaluated before it is
  applied, both of them, and then one *value* is chosen. This is §4.4's lesson again, and with
  effects it is no longer only a matter of wasted work.
- `one_runs`: the cure. Wrap each argument in a function of no parameters, and apply only the one
  chosen. What is passed is now two values, and only one is ever run.

A function of no parameters, written to delay a computation, is called a **thunk**. The word is
Ingerman's (1961), for the way Algol 60's compilers passed an unevaluated argument: as a little
piece of code that had already been "thunk" about. It is the oldest trick for controlling *when*
something happens, and in this language it costs nothing at run time, as the result shows: the
two functions are gone, and what is left is the `if` you would have written by hand.

### The same rule, for a name

Chapter 2 (§2.3) refused this, and can now say why:

<!-- file docs/book/code/def/no-effect.oro -->
```go
noisy = shout(1)

export fn run() { h(noisy, noisy) }
```

<!-- oro tutorial docs/book/code/def/no-effect.oro -->
```
oro: docs/book/code/def/no-effect.oro: the body of noisy is a computation, not a value, so unfolding it would repeat its effects
…
```

A top-level name is an abbreviation, replaced by its definition wherever it appears (δ). That is
copying, and there is no place *before* the copies to do the effect once, since a file's
definitions are not in any order and do not run. So a top-level definition must be a value. Every
`fn` is one. Inside a function, `noisy = shout(1)` is fine: a block is in order, and the shout
happens at that line.

## 5.10 A function has an effect if applying it reaches one

You never mark your own functions. Whether applying one has an effect is computed from its body.

<!-- file docs/book/code/effects/journal.oro -->
```go
pub fn note(n) { shout(n) }

pub fn twice(n) { n + n }
```

<!-- file docs/book/code/effects/inferred.oro -->
```go
use journal

fn loud(n) { shout(n) }

fn quiet(n) { f(n) }

fn deep(n) { quiet(loud(n)) }

fn ignore_it(a) { h(1, 2) }

export fn calls_quiet() { ignore_it(quiet(5)) }

export fn calls_loud() { ignore_it(loud(5)) }

export fn calls_deep() { ignore_it(deep(5)) }

export fn from_a_module() { ignore_it(journal.note(5)) }

export fn pure_from_a_module() { ignore_it(journal.twice(x())) }
```

<!-- oro tutorial docs/book/code/effects/inferred.oro -->
```go
calls_quiet =
fn() { h(1, 2) }
calls_loud =
fn() { a = shout(5); h(1, 2) }
calls_deep =
fn() { a = { n = shout(5); f(n) }; h(1, 2) }
from_a_module =
fn() { a = shout(5); h(1, 2) }
pure_from_a_module =
fn() { h(1, 2) }
```

`ignore_it` ignores its argument, so each line shows whether the argument could be dropped.

- `quiet(5)` reaches only `f`. Dropped.
- `loud(5)` reaches `shout`. Kept.
- `deep(5)` reaches `shout` two calls down. Kept, as a block: the shout, then the `f` of it. The
  `f` is dead and still there: the compiler keeps an argument with an effect whole rather than
  taking it apart.
- A module changes nothing. `journal.note` has an effect, `journal.twice` has none, and the
  program using them never had to be told.

The rule is: applying a host operation has an effect unless the target says `pure`; applying a
function of yours has one if evaluating its body does; and a literal, a name or a function, just
standing there, has none.

This is a real limit, and it should be said plainly. Nothing in a header says whether a function
has an effect. You cannot write "this function does not print" and have it checked. §5.14 says
what was bought with that.

## 5.11 A statement is a binding nobody reads

Chapter 1 (§1.12) showed that a line with an expression on it and no name is a binding whose name
is thrown away. Now it can be said why that is all a statement needs to be.

<!-- file docs/book/code/effects/stmt.oro -->
```go
export fn three() { shout(1); whisper(2); tick() }

export fn pure_statement() { f(1); h(1, 2) }

export fn in_a_branch(c) { if c { shout(1); f(2) } else { g(3) } }

export fn one_armed(c) { if c { shout(1) }; tick() }

export fn forgot_the_call() { shout; 7 }
```

<!-- oro tutorial docs/book/code/effects/stmt.oro -->
```go
three =
fn() { #_ = shout(1); #_1 = whisper(2); tick() }
pure_statement =
fn() { h(1, 2) }
in_a_branch =
fn(c) { if c { #_ = shout(1); f(2) } else { g(3) } }
one_armed =
fn(c) { #_ = if c { shout(1) } else { () }; tick() }
forgot_the_call =
oro: a statement's value is a function, shout, which does nothing until it is applied: call it, `f(…)`, or discard it on purpose with ignore(…) (ADR 0116)
```

- `three`: "do this, then that" is two bindings to names nobody reads (printed `#_`, a name no
  program can write). They stay, in order, **because dropping is denied to an effect** (§5.5).
  Sequencing is not a feature that was added. It is what is left when a binding may not be
  removed.
- `pure_statement`: the same shape with `f(1)`, and the line is gone. A statement with no effect
  does nothing, by definition, and so it is nothing.
- `in_a_branch`: a branch is a block, so it may hold statements.
- `one_armed`: an `if` with no `else` is a statement. Its missing branch is `()`, the value that
  carries no information (§4.9).
- `forgot_the_call` is refused. `shout` without parentheses is the function, not a call of it,
  and a function has no effect until it is applied, so the statement could only be dead code. It
  is the likeliest slip of someone arriving from a language where a bare name can be a call, and
  a line that silently did nothing would be the worst way to learn it. If you do mean to name a
  function and drop it, `ignore(shout)` says so
  ([ADR 0116](../decisions/0116-a-statement-whose-value-is-a-function-is-refused.md)).

Languages descended from Algol have two kinds of thing, expressions (which have values) and
statements (which do something), with separate rules for each. Here there is one kind. A
"statement" is an expression whose value is not kept, and the only expressions worth writing that
way are the ones with effects.

## 5.12 On a real target

Three functions, compiled for Go:

<!-- file docs/book/code/effects/report.oro -->
```go
use io

export fn report(a ∈ 0..100, b ∈ 0..100) (Int) {
    io.print_line("adding")
    sum = a + b
    io.print_int(sum)
    io.print_line("done")
    sum
}

export fn wasted(a ∈ 0..100, b ∈ 0..100) (Int) {
    product = a * b
    shown = io.print_int(a)
    b
}

export fn shown_twice(a ∈ 0..100) (Int) {
    n = io.print_int(a)
    n + n
}
```

<!-- gen go docs/book/code/effects/report.oro -->
```go
…
func EffectsReport(v0 int, v1 int) int {
	fmt.Println("adding")
	v25 := (v0 + v1)
	v26 := (strconv.Itoa(v25))
	fmt.Println(v26)
	fmt.Println("done")
	return v25
}

func EffectsShownTwice(v0 int) int {
	v9 := (strconv.Itoa(v0))
	fmt.Println(v9)
	v11 := (v0 + v0)
	return v11
}

func EffectsWasted(v0 int, v1 int) int {
	v23 := (strconv.Itoa(v0))
	fmt.Println(v23)
	return v1
}
```

- `report`: three prints in the order written, and the sum computed once and used twice.
- `wasted`: `product = a * b` is gone, and so is its multiplication. `shown` is read by nobody,
  and its print is there.
- `shown_twice`: one print, and its value used twice. **The value of a print is what it
  printed**, so `n` is `a`, and the compiler wrote `v0 + v0`. That is a convention of host
  operations that are statements (their value is their first argument), and `io.print_int` keeps
  it: it is defined as printing its integer's decimal text, `strconv.Itoa` on Go, and then being
  that integer. It is occasionally handy: `io.print_int(x)` can be dropped around any integer
  expression to watch it go by.

And a whole program, run on three targets:

<!-- file docs/book/code/effects/run.oro -->
```go
use io

fn second(a, b) { b }

fn announce(n) { io.print_int(n) }

export fn main() {
    io.print_line("start")
    kept = second(announce(1), announce(2))
    io.print_int(kept)
    unused = announce(3)
    io.print_int(announce(4) + announce(5))
    io.print_line("end")
}
```

<!-- run go docs/book/code/effects/run.oro -->
```
start
1
2
2
3
4
5
9
end
```

<!-- run js docs/book/code/effects/run.oro -->
```
start
1
2
2
3
4
5
9
end
```

<!-- run windows docs/book/code/effects/run.oro -->
```
start
1
2
2
3
4
5
9
end
```

Read it against the source. `announce(1)` is printed though `second` ignores it. `unused` is
printed though nobody reads it. `4` comes before `5`, and both before their sum. The three hosts
are a Go compiler, a JavaScript engine and LLVM, and they agree, because the order was settled
before any of them saw the program.

## 5.13 The default is "has an effect", and why

Someone writes a target and forgets the word `pure` on an operation.
[targets/tutorial-sloppy.oro](../../targets/tutorial-sloppy.oro) is the tutorial target with that
one mistake: `pure` is missing from `*`. One program, two targets:

<!-- file docs/book/code/effects/sloppy.oro -->
```go
export fn unused() { fn(n) { 7 }(x() * 4) }

export fn swapped() { fn(a, b) { b + a }(x() * 2, y() * 4) }

export fn literals() { 3 * 4 }
```

<!-- oro tutorial docs/book/code/effects/sloppy.oro -->
```go
unused =
fn() { 7 }
swapped =
fn() { y() * 4 + x() * 2 }
literals =
fn() { 12 }
```

<!-- oro tutorial-sloppy docs/book/code/effects/sloppy.oro -->
```go
unused =
fn() { n = x() * 4; 7 }
swapped =
fn() { a = x() * 2; b = y() * 4; b + a }
literals =
fn() { 3 * 4 }
```

On the careless target a multiplication nobody needs is still performed, two multiplications are
pinned in place, and `3 * 4` is left for run time, since computing it at compile time would be
performing an effect at the wrong moment (§1.3's folding is replacing equals by equals, and so it
has this chapter's condition too). The program is **slower**. It is not wrong, and the damage can
be seen in the output.

Now suppose the default went the other way, and the target's author forgot to mark a logging
operation as having an effect. The compiler would drop it, copy it and reorder it. The program
would be **wrong**, and nothing in the output would look unusual.

> **When a mark can be forgotten, the unmarked case must be the one that fails slow, not wrong.**

That is a design rule far beyond this language. A door that fails locked or unlocked, a railway
signal that shows red when its wire breaks: the question is always what happens when somebody
forgets, and the answer should be chosen, not left to chance.

## 5.14 What other languages do, and what this one gave up

The problem of this chapter is every language's problem. The answers fall in three families.

**Fix the order of everything.** C, Java, Go, Python, JavaScript: a program is a sequence of
steps, and the compiler must prove that a step has no effect before it moves it. That proof is
hard in a language where any call might do anything, so such compilers move less than they could.

**Put the effect in the type.** Haskell's answer (Moggi 1991, Wadler 1992): a function that
prints does not return an `Int`, it returns an `IO Int`, a different type, and the type checker
keeps the two worlds apart. You can read a function's type and know it cannot print. The price is
a second way of writing programs, for the effectful half, and the work of moving between them.
Effect systems (Lucassen and Gifford 1988) are relatives: a function's type lists the effects it
may have, and the list is checked.

**Mark the operations, and constrain the rewriting.** This language. An effect is not in any
type. It is a word on a host operation, and one condition on reduction. What that buys: there is
nothing to write, no wrapper around a value, no second notation, and no cost at run time, since
an effect is simply left where it was. What it gives up is the thing the second family has: here
no header can promise that a function is free of effects.

For a language whose commitment is that the code it emits is the code you would have written by
hand, that is the right side of the trade. For proving that a database layer never sends mail, it
would be the wrong one.

There is a pleasant footnote. Logics that give up some of Gentzen's three rules have been studied
for most of a century, and they are called **substructural**:

| logic | drop | copy | reorder | a thing is used… |
|---|---|---|---|---|
| ordinary | yes | yes | yes | any number of times |
| affine (Rust's ownership) | yes | no | yes | at most once |
| linear (Girard 1987) | no | no | yes | exactly once |
| ordered (Lambek 1958) | no | no | no | exactly once, in place |

The bottom row is this chapter's rule, word for word. Lambek invented it to describe the order of
words in a sentence, twenty years before anyone applied it to programs. The rule here was not
taken from him; it was arrived at from three ways a compiler could get a program wrong, and
turned out to be something with a name. Expressions with no effect live in the top row, those
with an effect in the bottom, and one word in a target's file says which.

## 5.15 What is still to come

Three later chapters meet effects again, and each adds one thing:

- **Repetition** ([chapter 12](README.md)). A function's body may run many times. An effect
  written outside a loop and used inside it must not be moved in, or one print becomes a
  thousand. It is §5.8's rule with a loop where the branch was.
- **State** ([chapter 14](README.md)). Writing to storage is an effect, and so is reading storage
  that may be written. It is an effect the language itself has, where all of this chapter's came
  from the host.
- **Failures** ([chapter 15](README.md)). Giving up is an effect too: it is something an
  expression does besides having a value.

---

## What to remember

- **An effect is anything an expression does besides having a value.** There is no form for it.
- **Equals may replace equals** only where there is no effect. Reduction is replacement of equals
  by equals, so effects are its one condition.
- **One word, `pure`, on each host operation**, in the target's file. No word means an effect. A
  program cannot write it; a function of yours has an effect if applying it reaches one.
- **Without an effect, an expression may be dropped, copied and moved. With one, it is evaluated
  exactly once, where it was written, in the order written.** Gentzen's weakening, contraction
  and exchange, each denied.
- **An argument with an effect is bound before the call**, in argument order, even if never read.
  Arguments of a host operation are evaluated left to right, on every target.
- **Safe to move and cheap to copy are different questions.** `f(9)` used twice is bound for
  speed; `shout(9)` used once is bound for correctness.
- **An effect written before a choice happens regardless; one written in a branch happens only
  there.** `&&` and `||` have branches.
- **A function is a value.** Writing one runs nothing, so it may always be copied or dropped. A
  function of no parameters delays a computation: a thunk.
- **A top-level definition must be a value**, since a name is replaced wherever it appears.
- **A statement is a binding nobody reads.** It stays only if it has an effect. A function named
  without parentheses is not a call, and vanishes.
- **The default is "has an effect"**, because a forgotten mark should make a program slow, not
  wrong.

Next: [chapter 6, Integers](06-integers.md). Part I was about how a program is reduced; part II
is about what it computes with, starting with the numbers that have been standing in the examples
since chapter 0.
