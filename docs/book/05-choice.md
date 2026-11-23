# 5. Choice

Three bricks so far: a [function](02-fn.md), a [name](03-def.md) for one, and a
[place](04-modules.md) for the name. With those a program can only ever do one thing: the same
steps, whatever it is given. `fn(n) { h(n, n) }` does to 7 what it does to 8.

This chapter is the fourth brick: **doing one thing or another.** It is the smallest piece of data
there is, a value that is one of two, and the one thing you can do with such a value, which is
choose.

It is also where the book starts to teach a little computer science along the way. The ideas here
are old and they are everywhere: Boole's algebra of 1854, the conditional expression McCarthy
invented in 1960, and a trick the compiler does with a known condition that has had a name,
*partial evaluation*, since the 1970s.

The examples are in [code/choice/](code/choice/). Those about the *shape* of a choice run on the
`tutorial` target, as before, with `f`, `g`, `h` and `shout` standing for things we know nothing
about. Those that compare real integers are compiled for Go, and their headers say which integers
they take: `score ∈ 0..100` reads *score is one of 0 to 100*, and `p ∈ Bool` reads *p is true or
false*. Chapter 7 is about that notation; here it is only a label.
`go run docs/book/check.go docs/book/05-choice.md` re-runs every block. Checked 2026-11-15.

---

## 5.1 Two values

<!-- file docs/book/code/choice/values.oro -->
```go
export fn yes() { true }

export fn smaller() { 3 < 4 }

export fn larger() { 4 < 3 }

export fn unknown() { x() < y() }
```

<!-- oro tutorial docs/book/code/choice/values.oro -->
```go
yes =
fn() { true }
smaller =
fn() { true }
larger =
fn() { false }
unknown =
fn() { x() < y() }
```

`true` and `false` are values, as `7` is a value. There are exactly two of them, and the set of
the two is called `Bool`, after George Boole. They are literals, as `7` is.

Where do they come from? Mostly from **comparisons**. `3 < 4` is not a statement that something
is so; it is an expression, with a value, and the value is `true`. The compiler worked two of
them out, since both sides were literals (§2.3), and left the third for run time.

The comparisons on integers are `<`, `<=`, `>`, `>=`, `==` and `!=`.

## 5.2 `if` chooses

<!-- file docs/book/code/choice/if.oro -->
```go
export fn taken() { if true { f(1) } else { g(1) } }

export fn not_taken() { if false { f(1) } else { g(1) } }

export fn computed() { if 3 < 4 { f(1) } else { g(1) } }

export fn waits(c) { if c { f(1) } else { g(1) } }

export fn waits_too() { if x() < y() { f(1) } else { g(1) } }
```

<!-- oro tutorial docs/book/code/choice/if.oro -->
```go
taken =
fn() { f(1) }
not_taken =
fn() { g(1) }
computed =
fn() { f(1) }
waits =
fn(c) { if c { f(1) } else { g(1) } }
waits_too =
fn() { if x() < y() { f(1) } else { g(1) } }
```

`if c { a } else { b }` is `a` when `c` is `true` and `b` when it is `false`. Those are its two
rules, and they are all it is:

```
if true  { a } else { b }   ⟶   a
if false { a } else { b }   ⟶   b
```

Two things to see in the output.

- **When the condition is known, the `if` is gone**, and so is the branch not taken. `taken` is
  `f(1)`; there is no test in it and no `g`. In `computed` the comparison was worked out first,
  then the choice.
- **When it is not known, the `if` stays**, and it is what runs.

**An `if` is an expression.** It has a value, the value of whichever branch is chosen, and it can
stand anywhere a value can:

<!-- file docs/book/code/choice/shape.oro -->
```go
export fn in_parentheses(c) { if (c) { 1 } else { 2 } }

export fn with_a_block(c) {
    if c {
        a = f(1)
        h(a, a)
    } else {
        0
    }
}

export fn as_an_operand(c) { 10 + (if c { 1 } else { 2 }) }

export fn as_an_argument(c) { h(if c { 1 } else { 2 }, 3) }
```

<!-- oro tutorial docs/book/code/choice/shape.oro -->
```go
in_parentheses =
fn(c) { if c { 1 } else { 2 } }
with_a_block =
fn(c) { if c { a = f(1); h(a, a) } else { 0 } }
as_an_operand =
fn(c) { 10 + (if c { 1 } else { 2 }) }
as_an_argument =
fn(c) { h(if c { 1 } else { 2 }, 3) }
```

- The condition needs no parentheses, and may have them.
- A branch is a block (§2.12): it may bind names, and its value is its last expression.
- The braces are not optional, and there is no `c ? a : b`. `if` already is that:

<!-- oro tutorial docs/book/code/choice/no-braces.oro.txt -->
```
oro: docs/book/code/choice/no-braces.oro.txt: line 1: "1" where "{" was expected
```

<!-- oro tutorial docs/book/code/choice/no-ternary.oro.txt -->
```
oro: docs/book/code/choice/no-ternary.oro.txt: line 1: "?" starts no token
```

> **Statements and expressions.** Fortran's `IF` (1957) and C's `if` are *statements*: they do
> something, and have no value, which is why C needed a second conditional, `?:`, for use inside
> an expression. McCarthy's conditional *expression* (1960, for Lisp) has a value, so one form
> does both jobs. This language has no statements of that kind at all (§2.12), so its `if` is
> McCarthy's.

Because the braces are always there, one famous ambiguity cannot be written. In C,
`if (a) if (b) x(); else y();` leaves a reader to wonder which `if` the `else` belongs to, the
*dangling else*. Here each `else` follows the closing brace of its own `if`.

## 5.3 Only a `Bool` chooses

In C, `if (n)` means *if n is not zero*. In Python and JavaScript an empty string, an empty list
and zero are all "falsy". Here a condition is a `Bool`, and nothing else is one:

<!-- file docs/book/code/choice/no-truthy.oro -->
```go
export fn run(n ∈ 0..9) (Int) { if n { 1 } else { 0 } }
```

<!-- gen go docs/book/code/choice/no-truthy.oro -->
```
gen: run: in a condition: n is 0..9, but Bool is required here
```

<!-- gen go docs/book/code/choice/no-truthy-text.oro -->
```
gen: run: in a condition: s is String, but Bool is required here
```

Say what you mean: `if n != 0 { … }`. The rule costs four characters, and it removes a whole
family of bugs in which a value that *happened* to be zero or empty was read as "no".

**The two branches must be the same kind of thing**, since the `if` has one value and whoever
uses it must know what it is:

<!-- file docs/book/code/choice/no-mixed.oro -->
```go
export fn run(c ∈ Bool) { if c { 1 } else { "two" } }
```

<!-- gen go docs/book/code/choice/no-mixed.oro -->
```
gen: run: the branches of a conditional are Int and String
```

## 5.4 `if` is not a function

Chapter 3 built booleans out of functions: `yes(t, e)` gave back `t`, `no(t, e)` gave back `e`
(§3.10). So why is `if` part of the language? Could it not be a function of three arguments?

<!-- file docs/book/code/choice/lazy.oro -->
```go
fn choose(c, a, b) { if c { a } else { b } }

export fn with_if(c) { if c { shout(1) } else { shout(2) } }

export fn with_a_function(c) { choose(c, shout(1), shout(2)) }

export fn pure_arguments(c) { choose(c, f(1), g(2)) }
```

<!-- oro tutorial docs/book/code/choice/lazy.oro -->
```go
with_if =
fn(c) { if c { shout(1) } else { shout(2) } }
with_a_function =
fn(c) { a = shout(1); b = shout(2); if c { a } else { b } }
pure_arguments =
fn(c) { if c { f(1) } else { g(2) } }
```

`with_if` shouts once, `1` or `2`. `with_a_function` shouts **twice**, and then chooses between
the two answers. That is §2.11's rule: `shout(1)` and `shout(2)` were written as arguments, and an
argument with an effect is evaluated exactly once, where it was written.

> **An `if` evaluates only the branch it takes. A function evaluates all of its arguments.**

That is the whole reason `if` is not a function. A choice that performed both alternatives and
then picked one could not guard anything: `if b != 0 { a / b } else { 0 }` would divide by zero
before deciding not to. Every language in which arguments are evaluated before the call (which
is nearly all of them) needs its conditional to be special for this reason.

When the arguments have no effects, as in `pure_arguments`, the difference cannot be observed,
and the compiler moved each into its branch: neither is computed unless it is chosen.

## 5.5 And, or, not

Three operators combine `Bool`s, and each is an `if`:

| written | means | read |
|---|---|---|
| `p && q` | `if p { q } else { false }` | p and q |
| `p \|\| q` | `if p { true } else { q }` | p or q |
| `!p` | `if p { false } else { true }` | not p |

That is not a manner of speaking. Write them out as functions and the result is the same term:

<!-- file docs/book/code/choice/connectives.oro -->
```go
fn both(p, q) { if p { q } else { false } }

fn either(p, q) { if p { true } else { q } }

fn inv(p) { if p { false } else { true } }

export fn written_out(p, q) { both(p, either(q, inv(p))) }

export fn built_in(p, q) { p && (q || !p) }
```

<!-- oro tutorial docs/book/code/choice/connectives.oro -->
```go
written_out =
fn(p, q) { p && (q || !p) }
built_in =
fn(p, q) { p && (q || !p) }
```

(The names `and`, `or` and `not` are the language's own, so the functions are called something
else, §3.4.)

Since each is an `if`, and an `if` on a known condition reduces, the compiler can fill in the
**truth tables** itself:

<!-- file docs/book/code/choice/tables.oro -->
```go
export fn and_tt() { true && true }

export fn and_tf() { true && false }

export fn and_ft() { false && true }

export fn and_ff() { false && false }

export fn or_tt() { true || true }

export fn or_tf() { true || false }

export fn or_ft() { false || true }

export fn or_ff() { false || false }

export fn not_t() { !true }

export fn not_f() { !false }
```

<!-- oro tutorial docs/book/code/choice/tables.oro -->
```go
and_tt =
fn() { true }
and_tf =
fn() { false }
and_ft =
fn() { false }
and_ff =
fn() { false }
or_tt =
fn() { true }
or_tf =
fn() { true }
or_ft =
fn() { true }
or_ff =
fn() { false }
not_t =
fn() { false }
not_f =
fn() { true }
```

*And* is true only when both are; *or* is false only when both are. That table, and the laws that
follow from it, are **Boolean algebra**: Boole's discovery that reasoning with "and", "or" and
"not" is a kind of arithmetic on two values. A century later Shannon (1937) showed that switching
circuits obey the same algebra, which is why the thing computing this page is built of *and* and
*or* gates.

Some of its laws the compiler applies, because they are just `if` reducing:

<!-- file docs/book/code/choice/laws.oro -->
```go
export fn and_true(c) { c && true }

export fn true_and(c) { true && c }

export fn and_false(c) { c && false }

export fn or_false(c) { c || false }

export fn or_true(c) { c || true }

export fn if_true_false(c) { if c { true } else { false } }

export fn if_false_true(c) { if c { false } else { true } }

export fn same_branches(c) { if c { f(1) } else { f(1) } }

export fn double_negation(c) { !!c }

export fn de_morgan_left(p, q) { !(p && q) }

export fn de_morgan_right(p, q) { !p || !q }
```

<!-- oro tutorial docs/book/code/choice/laws.oro -->
```go
and_true =
fn(c) { c }
true_and =
fn(c) { c }
and_false =
fn(c) { false }
or_false =
fn(c) { c }
or_true =
fn(c) { true }
if_true_false =
fn(c) { c }
if_false_true =
fn(c) { !c }
same_branches =
fn(c) { f(1) }
double_negation =
fn(c) { !!c }
de_morgan_left =
fn(p, q) { !(p && q) }
de_morgan_right =
fn(p, q) { !p || !q }
```

- `c && true` is `c`; `c && false` is `false`; `c || false` is `c`; `c || true` is `true`.
- `if c { true } else { false }` is just `c`. Beginners write that `if` in every language; it
  says "if it is true then true, otherwise false".
- An `if` whose branches are the same is that branch: the test decided nothing.
- **`!!c` stayed `!!c`, and De Morgan's two sides stayed different.** `!(p && q)` and `!p || !q`
  are *equal*: they have the same truth table. But reduction is not a search for equal things. It
  takes steps that compute, and stops when there are none left. Nothing here is wrong; the host's
  compiler will tidy `!!c`, and you should write whichever side reads better.

`!` binds tightest, then `&&`, then `||`, as in C; parentheses say otherwise:

<!-- file docs/book/code/choice/precedence.oro -->
```go
export fn and_first(p, q, r) { p || q && r }

export fn grouped(p, q, r) { (p || q) && r }

export fn not_first(p, q) { !p && q }

export fn not_grouped(p, q) { !(p && q) }
```

<!-- oro tutorial docs/book/code/choice/precedence.oro -->
```go
and_first =
fn(p, q, r) { p || q && r }
grouped =
fn(p, q, r) { (p || q) && r }
not_first =
fn(p, q) { !p && q }
not_grouped =
fn(p, q) { !(p && q) }
```

The printer writes parentheses only where they are needed, so `and_first` came back without any:
`p || q && r` *is* `p || (q && r)`.

## 5.6 The right side waits

`p && q` is `if p { q } else { false }`, and an `if` evaluates only the branch it takes. So:

<!-- file docs/book/code/choice/shortcut.oro -->
```go
export fn and_waits(c) { c && shout(1) < 2 }

export fn or_waits(c) { c || shout(1) < 2 }

export fn never_reached() { false && shout(1) < 2 }

export fn never_needed() { true || shout(1) < 2 }

fn both(p, q) { if p { q } else { false } }

export fn a_function_is_eager(c) { both(c, shout(1) < 2) }
```

<!-- oro tutorial docs/book/code/choice/shortcut.oro -->
```go
and_waits =
fn(c) { c && shout(1) < 2 }
or_waits =
fn(c) { c || shout(1) < 2 }
never_reached =
fn() { false }
never_needed =
fn() { true }
a_function_is_eager =
fn(c) { q = shout(1) < 2; c && q }
```

**The right side of `&&` is evaluated only if the left is true; the right side of `||` only if
the left is false.** This is *short-circuit evaluation*, and it follows from the definition; it
is not an extra rule.

- In `never_reached` the `shout` is gone. That does not break §2.11's promise that an effect is
  never dropped. There the effect was an *argument*, which is evaluated. Here it is a *branch*
  that is not taken, which is not.
- `a_function_is_eager` is §5.4 again: pass the same thing to a function, and it is evaluated
  first, whatever `c` is.

Short-circuiting is not only about effects. It is how one test protects the next, and the
compiler understands that:

<!-- file docs/book/code/choice/guard.oro -->
```go
export fn safe(a ∈ 0..100, b ∈ 0..100) (Bool) { b != 0 && a / b > 2 }
```

<!-- gen go docs/book/code/choice/guard.oro -->
```go
func ChoiceSafe(v0 int, v1 int) bool {
    v24 := (v1 == 0)
    v25 := (!v24)
    return (v25 && ((v0 / v1) > 2))
}
```

The division is reached only where `b != 0` is true, so it cannot divide by zero, and the
compiler accepted it for that reason. Turn the two halves round and it does not:

<!-- file docs/book/code/choice/no-guard.oro -->
```go
export fn unsafe(a ∈ 0..100, b ∈ 0..100) (Bool) { a / b > 2 && b != 0 }
```

<!-- gen go docs/book/code/choice/no-guard.oro -->
```
gen: choice-unsafe: `/` requires b != 0, which does not follow
```

`&&` is commutative in Boole's algebra: `p && q` and `q && p` have the same table. In a program
they are not the same, because the left side is a *fact* the right side may rely on. **Inside
`if c { … }`, and to the right of `c &&`, the compiler knows `c`.** That one idea is most of how
chapter 1's proofs worked, and it starts here.

## 5.7 Equality is for integers

<!-- file docs/book/code/choice/no-equal.oro -->
```go
export fn same(p ∈ Bool, q ∈ Bool) (Bool) { p == q }
```

<!-- gen go docs/book/code/choice/no-equal.oro -->
```
gen: same: in argument 1 of `==`: p is Bool, but Int is required here.
```

`==` and `!=` compare integers, and nothing else. That is a deliberate narrowness: the four hosts
do not agree on what it means for two strings, or two floating-point numbers, to be equal, and
the language promises only what all of them can keep. For `Bool`s nothing is lost, because
"equal" and "different" are choices:

<!-- file docs/book/code/choice/equal.oro -->
```go
export fn same(p ∈ Bool, q ∈ Bool) (Bool) { if p { q } else { !q } }

export fn differ(p ∈ Bool, q ∈ Bool) (Bool) { if p { !q } else { q } }

export fn implies(p ∈ Bool, q ∈ Bool) (Bool) { !p || q }
```

<!-- gen go docs/book/code/choice/equal.oro -->
```go
func ChoiceDiffer(v0 bool, v1 bool) bool {
    if v0 {
        return (!v1)
    }
    return v1
}
```

`differ` is *exclusive or*: one or the other, not both. `implies` is the logician's *if p then q*,
false only when `p` holds and `q` does not.

Comparisons do not chain, either. Mathematics writes *a < b < c*; here `a < b` is a `Bool`, and
`<` compares integers:

<!-- file docs/book/code/choice/no-chain.oro -->
```go
export fn between(a ∈ 0..9, b ∈ 0..9, c ∈ 0..9) (Bool) { a < b < c }
```

<!-- gen go docs/book/code/choice/no-chain.oro -->
```
gen: between: in argument 1 of `<`: `<` is Bool, but Int is required here
```

Write `a < b && b < c`. (C accepts the chain, and compares 0 or 1 with `c`.)

## 5.8 More than two ways

A choice among several things is a choice inside a choice:

<!-- file docs/book/code/choice/chain.oro -->
```go
export fn nested(c, d) { if c { 1 } else { if d { 2 } else { 3 } } }

export fn else_if(c, d) { if c { 1 } else if d { 2 } else { 3 } }

export fn guards(c, d) {
    case {
        c    -> 1
        d    -> 2
        else -> 3
    }
}

export fn on_one_line(c, d) { case { c -> 1; d -> 2; else -> 3 } }
```

<!-- oro tutorial docs/book/code/choice/chain.oro -->
```go
nested =
fn(c, d) { case { c -> 1; d -> 2; else -> 3 } }
else_if =
fn(c, d) { case { c -> 1; d -> 2; else -> 3 } }
guards =
fn(c, d) { case { c -> 1; d -> 2; else -> 3 } }
on_one_line =
fn(c, d) { case { c -> 1; d -> 2; else -> 3 } }
```

Four spellings, one term. `else if` is an `if` in the `else` branch, written without the extra
braces. And **`case { … }` is the same chain written as a table**: each line is a *guard*, an
arrow, and a value. Read it top to bottom:

> **The first guard that is true gives the value. `else` is what is left.**

Use `if` for two ways and `case` for more; the table is easier to read than a staircase, and
easier to check, because every way out is a line you can point at.

**Order matters**, since the first true guard wins:

<!-- file docs/book/code/choice/grade.oro -->
```go
export fn grade(score ∈ 0..100) (String) {
    case {
        score >= 90 -> "A"
        score >= 80 -> "B"
        score >= 70 -> "C"
        else        -> "F"
    }
}

export fn wrong_order(score ∈ 0..100) (String) {
    case {
        score >= 70 -> "C"
        score >= 80 -> "B"
        score >= 90 -> "A"
        else        -> "F"
    }
}
```

<!-- gen go docs/book/code/choice/grade.oro -->
```go
func ChoiceGrade(v0 int) string {
    v10 := (v0 >= 90)
    if v10 {
        return "A"
    }
    v15 := (v0 >= 80)
    if v15 {
        return "B"
    }
    v20 := (v0 >= 70)
    if v20 {
        return "C"
    }
    return "F"
}
```

`grade` reads `score >= 80` on its second line, and by then `score >= 90` is known to be false,
so the line means *80 to 89* without saying so. `wrong_order` has the same four lines and gives
everybody from 70 up a C: a 95 is `>= 70`, and that guard comes first. Its second and third arms
can never be reached. It compiles; nothing warns you. When guards overlap, put the narrowest
first.

**The last arm is `else`, always.** A chain with no `else` would have no value when every guard
failed, so it is refused, and so is an arm after the `else`, which could never be reached:

<!-- file docs/book/code/choice/no-else.oro.txt -->
```go
export fn run(c, d) { case { c -> 1; d -> 2 } }
```

<!-- oro tutorial docs/book/code/choice/no-else.oro.txt -->
```
oro: docs/book/code/choice/no-else.oro.txt: line 1: the last arm of case must be `else`, and only the last, so that the choice has a value whatever its guards are (booleans.md §4)
```

<!-- oro tutorial docs/book/code/choice/no-else-first.oro.txt -->
```
oro: docs/book/code/choice/no-else-first.oro.txt: line 1: the last arm of case must be `else`, and only the last, so that the choice has a value whatever its guards are (booleans.md §4)
```

A function that has an answer for every input is called **total**. C's `switch` with no
`default`, and an `if` chain with no final `else`, are how functions quietly stop being total;
here the shape of the form does not let it happen.

This is McCarthy's `cond` (1960) exactly: guards tried in order, the first true one taken.
Dijkstra's *guarded commands* (1975) look the same and differ in one deep way: there, when two
guards are true, either may be taken, and a correct program must not care. Ours always takes the
first.

Guards the compiler can decide disappear, as an `if`'s condition does:

<!-- file docs/book/code/choice/known.oro -->
```go
export fn first_is_true(c) { case { true -> 1; c -> 2; else -> 3 } }

export fn first_is_false(c) { case { false -> 1; c -> 2; else -> 3 } }

export fn only_else() { case { else -> 3 } }

fn sign(n) {
    case {
        n < 0 -> 0 - 1
        0 < n -> 1
        else  -> 0
    }
}

export fn sign_of_five() { sign(5) }

export fn sign_of(n) { sign(n) }
```

<!-- oro tutorial docs/book/code/choice/known.oro -->
```go
first_is_true =
fn(c) { 1 }
first_is_false =
fn(c) { if c { 2 } else { 3 } }
only_else =
fn() { 3 }
sign_of_five =
fn() { 1 }
sign_of =
fn(n) { case { n < 0 -> -1; 0 < n -> 1; else -> 0 } }
```

A `case` can do one more thing, take a value apart by what it *is* and not by a test on it. That
needs a kind of data this book has not introduced, and is chapter 11.

## 5.9 An `if` with no `else`

<!-- file docs/book/code/choice/one-arm.oro -->
```go
export fn alone(c) { if c { shout(1) } }

export fn then_more(c) { if c { shout(1) }; 7 }
```

<!-- oro tutorial docs/book/code/choice/one-arm.oro -->
```go
alone =
fn(c) { if c { shout(1) } else { () } }
then_more =
fn(c) { #_ = if c { shout(1) } else { () }; 7 }
```

An `if` with one branch is an `if` whose other branch is `()`, nothing (§2.4). It is for doing
something or not, where the something is done for its effect and nobody wants a value: here, to
shout or stay quiet. `then_more` is how it is used: on a line of its own, followed by what comes
next.

So a one-armed `if` is not a way to produce a number sometimes. Its value is either the branch's
or `()`, and `()` is not an integer:

<!-- file docs/book/code/choice/no-one-arm.oro -->
```go
export fn sometimes(c ∈ Bool) (Int) { if c { 1 } }
```

<!-- gen go docs/book/code/choice/no-one-arm.oro -->
```
gen: sometimes: the unit () is (), but Int is required here
```

C would return whatever was lying in a register, and Go refuses with *missing return*. The reason
here is the same as Go's, said as a fact about values: the function claims an `Int`, and on one
path it has nothing to give.

## 5.10 A choice moves to where it is needed

Reduction rearranges choices, and each rearrangement is a small theorem about `if`.

<!-- file docs/book/code/choice/moves.oro -->
```go
export fn applied(c) { (if c { f } else { g })(7) }

export fn if_of_if(c, d) { if (if c { d } else { false }) { f(1) } else { g(1) } }

export fn used_once(c) { v = if c { 1 } else { 2 }; f(v) }

export fn used_twice(c) { v = if c { 1 } else { 2 }; h(v, v) }

export fn same_test_twice(c) { if c { if c { f(1) } else { g(1) } } else { g(2) } }
```

<!-- oro tutorial docs/book/code/choice/moves.oro -->
```go
applied =
fn(c) { if c { f(7) } else { g(7) } }
if_of_if =
fn(c, d) { if c && d { f(1) } else { g(1) } }
used_once =
fn(c) { f(if c { 1 } else { 2 }) }
used_twice =
fn(c) { v = if c { 1 } else { 2 }; h(v, v) }
same_test_twice =
fn(c) { if c { if c { f(1) } else { g(1) } } else { g(2) } }
```

- **Applying a choice is choosing between applications.** `(if c { f } else { g })(7)` became
  `if c { f(7) } else { g(7) }`. The application moved *into* the branches, where each one meets
  a function it knows. This is why §2.10 could say no function is ever a run-time value, even
  when which function is called is decided at run time.
- A choice whose condition is a choice is one choice on both conditions.
- A choice bound to a name follows §2.11: used once, it goes where it is used; used twice, it is
  made once.
- `same_test_twice` tests `c` again inside a branch where `c` is already known to be true, and
  the inner test stayed. The reducer computes; it does not carry facts from a condition into its
  branches. The analysis that proves things does (§5.6), and a host's compiler will remove the
  second test, but it is still better not to write it.

## 5.11 A choice made at compile time

Put the pieces together: definitions unfold (chapter 3), arithmetic on literals is worked out
(§2.3), and a choice on a known condition keeps one branch (§5.2).

<!-- file docs/book/code/choice/clamp.oro -->
```go
fn max(a, b) { if a < b { b } else { a } }

fn min(a, b) { if a < b { a } else { b } }

fn clamp(v, lo, hi) { max(lo, min(v, hi)) }

export fn all_known() { clamp(15, 0, 10) }

export fn one_unknown(v) { clamp(v, 0, 10) }
```

<!-- oro tutorial docs/book/code/choice/clamp.oro -->
```go
all_known =
fn() { 10 }
one_unknown =
fn(v) { b = if v < 10 { v } else { 10 }; if 0 < b { b } else { 0 } }
```

`clamp(15, 0, 10)` is `10`. Three function calls, two comparisons and two choices, all decided
before the program exists. With `v` unknown, what is left is exactly the part that depends on
`v`: two tests, with the `0` and the `10` already in place.

> **Partial evaluation.** A program has inputs. Give a compiler some of them early, and it can
> perform every step that depends only on those, leaving a smaller program that waits for the
> rest. The idea is Futamura's (1971); the textbook is Jones, Gomard and Sestoft (1993). Most
> compilers do a little of it and call the pieces *constant folding*, *inlining* and *dead-code
> elimination*. Here it is not an optimization laid over the language. It is how the language
> runs: reduction *is* partial evaluation, and "the branch not taken is removed" is the
> dead-code elimination, with nothing left to eliminate.

It is the reason a general function costs no more than a special one. `clamp` is written once,
for any bounds, and each use is compiled as if it had been written for its own.

A real one, to end on. The rule for leap years is a choice of three tests:

<!-- file docs/book/code/choice/leap.oro -->
```go
export fn leap(y ∈ 1..9999) (Bool) { y % 4 == 0 && (y % 100 != 0 || y % 400 == 0) }
```

<!-- gen go docs/book/code/choice/leap.oro -->
```go
func ChoiceLeap(v0 int) bool {
    v10 := (v0 & 3)
    v12 := (v10 == 0)
    if v12 {
        v14 := (v0 % 100)
        v16 := (v14 == 0)
        v17 := (!v16)
        return (v17 || ((v0 % 400) == 0))
    }
    return false
}
```

Every piece of this chapter is in that output. The `&&` is an `if`, with `false` for its other
branch. The right side waits: a year not divisible by 4 never has its remainder by 100 taken.
The `||` is Go's own, which short-circuits too. And `y % 4` became `y & 3`, because for a year
that cannot be negative they are the same, and the second is cheaper; chapter 8 is about why.

---

## What to remember

- **`true` and `false` are the two values of `Bool`**, and comparisons make them.
- **`if c { a } else { b }` is an expression.** It is `a` when `c` is true and `b` when it is
  false, and it evaluates only the one it takes. There is no `?:`, and the braces are required.
- **Only a `Bool` is a condition.** Zero and the empty string are not "false".
- **`&&`, `||` and `!` are each an `if`**, so the right side of `&&` and `||` waits for the left,
  and to the right of `c &&` the compiler knows `c`.
- **`==` and `!=` compare integers only**, and comparisons do not chain.
- **`case { guard -> value; …; else -> value }` is an `if` chain written as a table.** The first
  true guard wins, so order matters; the last arm is always `else`, so there is always a value.
- **A condition known at compile time is decided at compile time**, and the other branch is
  gone. That is partial evaluation, and with unfolding it is why a general definition costs
  nothing.
- **An `if` is not a function**: a function evaluates all its arguments before it can choose.

Next: [effects](06-effects.md), what it means for an expression to *do* something, and why that
is the one thing reduction may not move. That chapter is still in the notation this surface
replaced, and is the next to be rewritten.
