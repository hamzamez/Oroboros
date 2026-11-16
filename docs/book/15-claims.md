# 15. Claims

Since chapter 6 this book has leaned on one sentence: *the compiler proves it, or refuses the
program.* An addition fits the word. A divisor is not zero. A position is inside its table. A
rounding lands in its set. A string has no zero in it. Each time the text said "proven" and
moved on, and each refusal ended with a line beginning `known:` that was never explained.

This chapter opens the box. The question is: **when the compiler says "proven" or "does not
follow", what did it actually do, and what can you do when it is wrong to refuse?**

The answer is three sentences.

> **Every claim becomes an obligation**: a statement that must be true at one place in the
> program, for every input the headers allow.
>
> **The compiler gathers facts and tries two methods**: intervals, which track the range of
> each value, and linear arithmetic, which combines facts like `i < n` and `n <= len`.
>
> **It never accepts a false claim, and it sometimes refuses a true one.** When it does, you
> tell it more.

Along the way: three ways of knowing a program is right and what each is worth, a method for
solving inequalities that is two hundred years old, the theorem that says where every such
checker must stop, and a way to ask the compiler what it knows.

The examples are in [code/claims/](code/claims/). `go run docs/book/check.go
docs/book/15-claims.md` re-runs every block. Checked 2026-11-16.

---

## 15.1 Three ways of knowing

How do you know a function is right?

**You can test it.** Run it on some inputs and look. Every working programmer does, and it
catches a great deal. Its limit was put in one sentence by Edsger Dijkstra in 1969: *testing
shows the presence of bugs, never their absence.* A function with two 64-bit parameters has
more possible inputs than there are atoms in a mountain. You tried twelve.

**You can give it a type.** Say `n` is an `int` and `s` is a `string`, and the compiler
refuses `n + s` for every input at once, without running anything. That is knowledge of a
different kind: about *all* inputs. Its limit is that the statements are coarse. "This is an
integer" does not say "this is a valid position in that table", and the bugs that hurt are
of the second kind.

**You can prove it.** State exactly what must hold, `0 <= i < len(t)`, and show it follows
from what is known. This is as strong as a type and as fine as you like. Its limit is that
proofs are work, and that some true things are hard to prove.

This language's bet is that the third kind can be made nearly as cheap as the second, for
the statements that matter most in ordinary programs, if the compiler does the proving and
the statements are kept to a kind it is good at. That kind is what this chapter describes.

---

## 15.2 What gets proven

Here is every obligation this book has met, in one table. Each row is something that must
be true at a particular place, and each was a refusal you have seen.

| where | what must hold | chapter |
|---|---|---|
| `a + b`, `a * b`, `a - b` | the result fits the target's word | 6 |
| `a / b`, `a % b` | `b` is not zero | 6 |
| `a << k`, `a >> k` | `k` is not negative | 7 |
| a result declared `(S)` | the body's value is in `S` | 6 |
| `ensure ψ` | ψ holds of the result | 6 |
| a call `f(e)` where `f(x ∈ S)` | `e` is in `S` | 6 |
| a call of `f … where φ` | φ holds of the arguments | 6 |
| `round(S, e)` | the rounded value is in `S` | 8 |
| `round(F64, x + y)` | the sum is within the doubles' range | 8 |
| `string_of(n)` | `n` is a scalar value | 9 |
| a call needing `nulfree(s)` | `s` has no character 0 | 9 |
| `t[i]` | `0 <= i` and `i < len(t)` | 12 |
| `x ∈ S = e` | `e` is in `S` | this chapter |

They look different and are one thing. Each is a *formula*, a statement that is true or
false, about the values at that point. An **obligation** is a formula the compiler must show
to be true there, whatever the inputs. If it can, the program is accepted and the formula is
thrown away: nothing of it reaches the emitted code. If it cannot, the program is refused,
and the message names the formula.

Where a claim is checked depends on which side of it you are on:

- A function's **parameter sets and `where`** are obligations at every place it is *called*,
  and facts *inside* its body.
- Its **result set and `ensure`** are obligations inside its body, and facts for its callers.
- For an **exported** function the caller is outside the program, where nothing can be
  checked. So an export's parameter claims are simply assumed, and they are the program's
  statement to the outside world of what it accepts.

---

## 15.3 The first method: intervals

Chapter 6 introduced this, and it is worth doing once slowly. For every value in the program
the compiler keeps the smallest and largest it could be.

<!-- file docs/book/code/claims/mix.oro -->
```go
export fn mix(a ∈ 0..10, b ∈ 5..7, c ∈ Bool) (-8..33) {
    s = a + b
    d = if c { s * 2 } else { 3 - a }
    d - 1
}
```

<!-- gen go docs/book/code/claims/mix.oro -->
```
note: claims-mix: 4 of 4 integer operations bounded; 0 of 0 loop(s) proven terminating
```

Follow it by hand:

| value | reasoning | interval |
|---|---|---|
| `a` | its header | 0..10 |
| `b` | its header | 5..7 |
| `s = a + b` | smallest plus smallest, largest plus largest | 5..17 |
| `s * 2` | both ends doubled | 10..34 |
| `3 - a` | 3 minus the *largest* `a`, 3 minus the *smallest* | −7..3 |
| `d` | either branch: the smallest of both, the largest of both | −7..34 |
| `d - 1` | | −8..33 |

and −8..33 is what the header claims, so the claim holds. Each operation has a small rule
for how intervals pass through it. Subtraction crosses the ends over. An `if` takes the
*union* of its branches, because the compiler does not know which one runs. A condition
narrows: inside `if n < 10 { … }`, the interval of `n` is cut off at 9.

Intervals are fast, simple and good enough for most of the arithmetic obligations in a
program: "does it fit the word" is a question about how big a value can get, and that is
what an interval records. The count in the note line, *4 of 4 integer operations bounded*,
is intervals at work. (`go run ./cmd/intervals -v FILE TARGET` prints that count for a file,
with every operation it could not bound.)

They have one blind spot, and it is a large one. **An interval describes each value alone.**
It cannot record that two values are related.

<!-- file docs/book/code/claims/no-gap.oro -->
```go
export fn gap(a ∈ 0..100, b ∈ 0..100) (0..100) where a <= b { b - a }
```

<!-- gen go docs/book/code/claims/no-gap.oro -->
```
gen: gap: the body does not establish 0 <= result && result <= 100; its result is [-100, 100]
```

`b - a`, with `a <= b`, is plainly between 0 and 100. But as intervals, `b` is 0..100 and
`a` is 0..100, and 0..100 minus 0..100 is −100..100: the smallest `b` minus the largest `a`.
The fact that makes the claim true, `a <= b`, is a relation *between* the two, and an
interval has nowhere to put it. A function's result claim is decided by intervals, so this
one is refused.

---

## 15.4 The second method: facts

For relations the compiler keeps a second thing: a list of **facts**, formulas known to be
true at a point. Here is the same function, with the claim moved onto a binding:

<!-- file docs/book/code/claims/gap.oro -->
```go
export fn gap(a ∈ 0..100, b ∈ 0..100) (0..100) where a <= b {
    d ∈ 0..100 = b - a
    d
}
```

<!-- gen go docs/book/code/claims/gap.oro -->
```go
func ClaimsGap(v0 int, v1 int) int {
    v31 := (v1 - v0)
    return v31
}
```

`d ∈ 0..100 = b - a` is a binding with a claim: *`d` is `b - a`, and I say it is in 0..100.*
A claim on a binding is decided by the second method, which can use `a <= b`. It is proven,
`d` is then known to be in 0..100, and the result claim follows. (Section 15.6 is about this
construct.)

Take the `where` away to see what the compiler was working with:

<!-- file docs/book/code/claims/no-gap-fact.oro -->
```go
export fn gap(a ∈ 0..100, b ∈ 0..100) (0..100) {
    d ∈ 0..100 = b - a
    d
}
```

<!-- gen go docs/book/code/claims/no-gap-fact.oro -->
```
gen: claims-gap: the claim d ∈ 0..100 is not proven: 0 <= b - a does not follow
  known: assumed -a <= 0, assumed a + -100 <= 0, assumed -b <= 0, assumed b + -100 <= 0
```

That is the `known:` line, and now it can be read. It lists the facts in force, each
rewritten into one standard shape, *something `<= 0`*:

| as printed | as you would write it | from |
|---|---|---|
| `-a <= 0` | `0 <= a` | `a ∈ 0..100` |
| `a + -100 <= 0` | `a <= 100` | `a ∈ 0..100` |
| `-b <= 0` | `0 <= b` | `b ∈ 0..100` |
| `b + -100 <= 0` | `b <= 100` | `b ∈ 0..100` |

"assumed" means *taken from a header*, as opposed to derived. The goal was `0 <= b - a`, and
from these four facts it does not follow: `a` could be 100 and `b` 0. With the `where`, a
fifth fact joins the list, `a + -b <= 0`, which is `a <= b`, which is the goal itself.

When you meet a refusal, read this line first. It tells you exactly what the compiler knew.
The fix is nearly always to add the one fact that is missing.

### Combining facts

The method earns its keep when no single fact is the goal and several together are.

<!-- file docs/book/code/claims/chain.oro -->
```go
export fn chain(i ∈ Int, n ∈ Int, t ∈ Array(Int)) (Int) where 0 <= i && i < n && n <= len(t) {
    t[i]
}
```

<!-- gen go docs/book/code/claims/chain.oro -->
```go
func ClaimsChain(v0 int, v1 int, v2 []int) int {
    _ = v1
    v17 := v2[v0]
    return v17
}
```

`t[i]` needs `i < len(t)`. Nothing says that. What is known is `i < n` and `n <= len(t)`,
and the conclusion follows from the two *together*: `i` is below `n`, which is at most
`len(t)`.

How does a program find that? By adding inequalities, the way you would solve them on
paper. In whole numbers `i < n` is `i + 1 <= n`. Write the two facts with everything on the
left:

```
   i - n + 1  <=  0            (i < n)
   n - len    <=  0            (n <= len)
   ─────────────────  add them: the n cancels
   i - len + 1  <=  0          which is  i < len
```

To prove a goal, the compiler *assumes the opposite* and looks for a contradiction, adding
facts so as to cancel one unknown at a time. If it reaches something absurd, like `1 <= 0`,
the opposite was impossible and the goal holds. If every unknown is eliminated and no
absurdity appears, the goal does not follow. The method is **Fourier–Motzkin elimination**:
Joseph Fourier described it in 1826 and Theodore Motzkin rediscovered it in 1936. It is
schoolroom algebra, done exhaustively.

Weaken one fact by one character and the proof is gone:

<!-- file docs/book/code/claims/no-chain.oro -->
```go
export fn chain(i ∈ Int, n ∈ Int, t ∈ Array(Int)) (Int) where 0 <= i && i <= n && n <= len(t) {
    t[i]
}
```

<!-- gen go docs/book/code/claims/no-chain.oro -->
```
gen: claims-chain: t[i] is an indexing, and i < len(t) does not follow
  known: assumed -i <= 0, assumed i + -n <= 0, assumed -len(t) + n <= 0
```

With `i <= n` the sum of the two facts gives only `i <= len(t)`, which allows `i == len(t)`,
one past the end. The refusal is correct, and it is chapter 12's off-by-one again.

Facts may carry multipliers, as long as each term is a number times an unknown:

<!-- file docs/book/code/claims/combine.oro -->
```go
export fn combine(x ∈ 0..100, y ∈ 0..100, t ∈ Array(Int)) (Int) where 2 * x + 3 * y < len(t) {
    t[x + y]
}

export fn strict(a ∈ 0..100, b ∈ 0..100) (Int) where a < b { 1000 / (b - a) }
```

<!-- gen go docs/book/code/claims/combine.oro -->
```go
func ClaimsCombine(v0 int, v1 int, v2 []int) int {
    v34 := (v0 + v1)
    v35 := v2[v34]
    return v35
}

func ClaimsStrict(v0 int, v1 int) int {
    v32 := (v1 - v0)
    v33 := (1000 / v32)
    return v33
}
```

- `combine`: is `x + y < len(t)`? Known: `2x + 3y < len(t)`, and `x` and `y` are not
  negative. Then `x + y <= 2x + 3y`, since the right side only adds more of each. Proven.
- `strict`: is `b - a` nonzero? Known: `a < b`, so `b - a >= 1`. Proven.

And each fails the moment the facts stop reaching:

<!-- file docs/book/code/claims/no-combine.oro -->
```go
export fn combine(x ∈ 0..100, y ∈ 0..100, t ∈ Array(Int)) (Int) where x + y < len(t) {
    t[2 * x]
}
```

<!-- gen go docs/book/code/claims/no-combine.oro -->
```
gen: claims-combine: t[2 * x] is an indexing, and 2 * x < len(t) does not follow
  known: assumed -len(t) + x + y + +1 <= 0, assumed -x <= 0, assumed x + -100 <= 0, assumed -y <= 0, assumed y + -100 <= 0
```

<!-- file docs/book/code/claims/no-strict.oro -->
```go
export fn strict(a ∈ 0..100, b ∈ 0..100) (Int) where a <= b { 1000 / (b - a) }
```

<!-- gen go docs/book/code/claims/no-strict.oro -->
```
gen: claims-strict: `/` requires b - a != 0, which does not follow
  known: assumed a + -b <= 0, assumed -a <= 0, assumed a + -100 <= 0, assumed -b <= 0, assumed b + -100 <= 0
```

`x + y < len` says nothing about `2 * x` when `y` is 0 and `x` is large. And `a <= b` allows
`a == b`, a division by zero. Both programs are wrong, and the compiler found the input that
shows it without running anything.

---

## 15.5 Where facts come from

You have written facts in headers. Most of them the compiler collects for itself.

<!-- file docs/book/code/claims/sources.oro -->
```go
fn at(t ∈ Array(Int), i ∈ 0..len(t)-1) (Int) { t[i] }

export fn third(t ∈ Array(Int)) (Int) where len(t) >= 3 { at(t, 2) }

export fn either(n ∈ 0..100, t ∈ Array(Int)) (Int) where len(t) >= 51 {
    if n > 50 { t[n - 50] } else { t[n] }
}

export fn equation(x ∈ 0..100) (Int) {
    y = x + 1
    z = y - x
    array { 10, 20 }[z]
}

export fn element(t ∈ Array(0..9)) (0..18) where len(t) > 0 { t[0] * 2 }

export fn counted(n ∈ 0..1000) (0..1000) {
    loop i = 0 {
        i < n -> again(i + 1)
        else -> i
    }
}
```

<!-- gen go docs/book/code/claims/sources.oro -->
```go
func ClaimsEither(v0 int, v1 []int) int {
    v17 := (v0 > 50)
    if v17 {
        v20 := (v0 - 50)
        v21 := v1[v20]
        return v21
    }
    v23 := v1[v0]
    return v23
}

func ClaimsElement(v0 []byte) int {
    v5 := int(v0[0])
    v7 := (v5 * 2)
    return v7
}

func ClaimsEquation(v0 int) int {
    v11 := []byte{10, 20}
    v13 := (v0 + 1)
    v14 := (v13 - v0)
    v15 := int(v11[v14])
    return v15
}

func ClaimsThird(v0 []int) int {
    v5 := v0[2]
    return v5
}
```

Five sources, one function each.

**Headers.** `at` says its `i` is in `0..len(t)-1`. A set's end may mention an earlier
parameter: the range of `i` depends on `t`. Inside `at` that is a fact, and `t[i]` is
proven. At the call `at(t, 2)` it is an obligation, `2 <= len(t) - 1`, and `third`'s own
`where len(t) >= 3` proves it. Ask for position 2 of a table known only to have two values:

<!-- file docs/book/code/claims/no-third.oro -->
```go
fn at(t ∈ Array(Int), i ∈ 0..len(t)-1) (Int) { t[i] }

export fn third(t ∈ Array(Int)) (Int) where len(t) >= 2 { at(t, 2) }
```

<!-- gen go docs/book/code/claims/no-third.oro -->
```
gen: claims-third: at requires 2 <= len(t) - 1 at a call, which does not follow
  known: assumed -len(t) + +2 <= 0, assumed -len(t) <= 0 (fact len-nonneg)
```

**Conditions.** In `either`, the `if` splits the facts. In the first branch `n > 50`, so
`n - 50` is in 1..50. In the second `n <= 50`. With `len(t) >= 51` both lookups are inside.
Every `if`, every `case` arm and every loop guard adds a fact to its own branch and the
opposite fact to what follows.

**Equations.** `y = x + 1` is a fact too, an exact one. So `z = y - x` is known to be
exactly 1, whatever `x` is, and position 1 of a two-element table is inside. Intervals alone
would say `z` is somewhere in −99..101.

**The data's own laws.** The message above shows one: `-len(t) <= 0 (fact len-nonneg)`. A
length is never negative, and the compiler knows it without being told. `element` uses
another: a value read from an `Array(0..9)` is in 0..9, so twice it is in 0..18.

**Loops.** `counted` claims its result is in 0..1000, and the result is `i` on the way out.
What is known about a loop variable? It is different every time round. The compiler looks
for something true *every* time: an **invariant**. It guesses candidates of simple shapes
(`0 <= i`, `i <= n`), checks that each holds on entry and that each trip round preserves
it, and keeps the ones that survive. Here both do: `i` starts at 0, and it is increased only
when `i < n`, so it never passes `n`. With `n <= 1000`, the claim follows. This is what was
happening in chapter 11 every time a sum in a loop was "bounded by the number of trips", and
in chapter 12 when "the loop's own test is the proof".

### Stating an invariant yourself

The compiler's guesses have simple shapes: a variable against a constant, a variable against
another variable. Sometimes what is true every time round has a shape it does not guess. A
counter moves along a board of 201 squares. Each square holds a digit, and the digit says
how far to move: two squares if it is more than 4, one otherwise. Where is the counter after
some number of turns, at most 100?

<!-- file docs/book/code/claims/no-walk.oro -->
```go
export fn walk(board ∈ Array(0..9), turns ∈ 0..100) (Int) where len(board) == 201 {
    loop i = 0, s = 0 {
        i >= turns -> s
        board[s] > 4 -> again(i + 1, s + 2)
        else -> again(i + 1, s + 1)
    }
}
```

<!-- gen go docs/book/code/claims/no-walk.oro -->
```
gen: claims-walk: board[s] is an indexing, and s < len(board) does not follow
  known: assumed -turns <= 0, assumed turns + -100 <= 0, 0 <= i, 0 <= s, assumed i + -turns <= 0, assumed i + -s <= 0, assumed i + -turns + +1 <= 0, assumed i + -turns + +1 <= 0
```

You can see why it is safe. The counter moves at most two squares a turn, so after `i` turns
it is at most at square `2 * i`, and with at most 100 turns that is square 200, the last
one. Now look at what the compiler found on its own, in the `known:` line: `0 <= i`,
`0 <= s`, and `i + -s <= 0`, which is `i <= s`, the counter has moved at least one square a
turn. All true, and none of them is the one that matters. "At most *twice* `i`" has a 2 in
it, and the compiler does not try every multiplier there is.

So say it. A loop may state its invariant, after its variables and before its brace:

<!-- file docs/book/code/claims/walk.oro -->
```go
use io

export fn walk(board ∈ Array(0..9), turns ∈ 0..100) (Int) where len(board) == 201 {
    loop i = 0, s = 0 invariant 0 <= s && s <= 2 * i {
        i >= turns -> s
        board[s] > 4 -> again(i + 1, s + 2)
        else -> again(i + 1, s + 1)
    }
}

export fn main() {
    board = array { k % 10 : k ∈ 0..200 }
    io.print_int(walk(board, 7))
    io.print_int(walk(board, 100))
}
```

<!-- run go docs/book/code/claims/walk.oro -->
```
9
142
```

`invariant 0 <= s && s <= 2 * i` is a claim like every other in this chapter: proven, or
the program is refused. The proof has two parts, and they are the two parts of every proof
by induction you have ever seen.

- **It holds on entry.** `i` and `s` start at 0, and `0 <= 0 && 0 <= 2 * 0`.
- **Every `again` keeps it.** Suppose it holds now, `s <= 2 * i`. One arm goes round with
  `i + 1` and `s + 2`: is `s + 2 <= 2 * (i + 1)`? That is `s + 2 <= 2 * i + 2`, which is what
  was supposed. The other goes round with `s + 1`, which is smaller still.

True at the start, and never broken by a step: so true every time. And then it is a *fact*
inside the loop. With `i < turns` (the first arm was not taken) and `turns <= 100`, `s` is
at most 198, and `board[s]` is inside.

<!-- gen go docs/book/code/claims/walk.oro -->
```go
note: claims-main: 8 of 8 integer operations bounded; 2 of 2 loop(s) proven terminating
note: claims-walk: 4 of 4 integer operations bounded; 1 of 1 loop(s) proven terminating
…
func ClaimsWalk(v0 []byte, v1 int) int {
    var v19 int = 0
    var v20 int = 0
    for ; ; v19 = (v19 + 1) {
        v21 := (v19 >= v1)
        if v21 {
            break
        }
        v26 := int(v0[v20])
        v28 := (v26 > 4)
        if v28 {
            v32 := (v20 + 2)
            v20 = v32
            continue
        }
        v36 := (v20 + 1)
        v20 = v36
        continue
    }
    return v20
}
```

The invariant is nowhere in the Go. It is not a test that runs: it was a statement to the
compiler, checked once, and then it was gone. That is the difference from chapter 11's
stated bounds, `p <= 4 * 10^17` in the factorial, which are real tests in the loop and
which stop it early if they are ever false. A guard says "stop if this fails". An invariant
says "this does not fail", and has to be right.

Each half of the proof can go wrong, and each has its own refusal. Claim too much, that the
counter moves at most one square a turn:

<!-- file docs/book/code/claims/no-kept.oro -->
```go
export fn walk(board ∈ Array(0..9), turns ∈ 0..100) (Int) where len(board) == 201 {
    loop i = 0, s = 0 invariant 0 <= s && s <= i {
        i >= turns -> s
        board[s] > 4 -> again(i + 1, s + 2)
        else -> again(i + 1, s + 1)
    }
}
```

<!-- gen go docs/book/code/claims/no-kept.oro -->
```
gen: claims-walk: a loop's invariant does not hold at every iteration: it is not kept by every `again`, 0 <= s && s <= i
```

It holds on entry, and the arm that adds 2 breaks it. Or claim something false from the
first moment:

<!-- file docs/book/code/claims/no-entry.oro -->
```go
export fn walk(board ∈ Array(0..9), turns ∈ 0..100) (Int) where len(board) == 201 {
    loop i = 0, s = 0 invariant 1 <= s && s <= 2 * i {
        i >= turns -> s
        board[s] > 4 -> again(i + 1, s + 2)
        else -> again(i + 1, s + 1)
    }
}
```

<!-- gen go docs/book/code/claims/no-entry.oro -->
```
gen: claims-walk: a loop's invariant does not hold on entry: 1 <= 0 && 0 <= 2 * 0
```

The message has put the starting values in for you: `s` is 0, and 1 is not at most 0.

Finding the invariant is the hard part of proving anything about a loop, and it is a skill.
The statement must be *weak enough to be true at the start*, *strong enough to give what
you want at the end*, and *of a shape each step preserves*. Robert Floyd in 1967 and Tony
Hoare in 1969 made this the foundation of reasoning about programs, and it is still how
every proof of a loop goes, by hand or by machine. Most loops in this book needed no stated
invariant because theirs was one of the simple shapes. When a loop of yours is refused, ask
what is true every time round, and whether the `known:` line has it.

An invariant here is linear arithmetic over the loop's variables, like every fact in this
chapter. The factorial's real invariant, "`p` is the factorial of `i - 1`", is not that, so
chapter 11's guard stays a guard. And an invariant cannot yet speak of what a buffer
*holds*, only of its length.

---

## 15.6 A claim on a binding

`x ∈ S = e` binds `x` to `e` and claims `e` is in `S`. Section 15.4 used one to get a
relational fact to a result. It has a second use that is worth more: it is how you *ask the
compiler what it knows*.

<!-- file docs/book/code/claims/lemma.oro -->
```go
export fn total(n ∈ 0..100, m ∈ 0..100) (Int) {
    s ∈ 0..200 = n + m
    s
}
```

<!-- gen go docs/book/code/claims/lemma.oro -->
```go
func ClaimsTotal(v0 int, v1 int) int {
    v23 := (v0 + v1)
    return v23
}
```

The claim was checked and has left no trace. It is not an `assert` that runs: there is no
test in the Go. Now claim something false:

<!-- file docs/book/code/claims/no-lemma.oro -->
```go
export fn total(n ∈ 0..100, m ∈ 0..100) (Int) {
    s ∈ 0..150 = n + m
    s
}
```

<!-- gen go docs/book/code/claims/no-lemma.oro -->
```
gen: claims-total: the claim s ∈ 0..150 is not proven: n + m <= 150 does not follow
  known: assumed -n <= 0, assumed n + -100 <= 0, assumed -m <= 0, assumed m + -100 <= 0
```

When a refusal deep in a function puzzles you, this is the tool. Somewhere above it, write
down a thing you believe as a claim on a binding. If it is accepted, your belief is right
and the trouble is further down. If it is refused, you have found where your picture and
the compiler's part ways, and the `known:` line shows the compiler's.

Mathematicians do the same and call the intermediate statement a *lemma*: a small thing
proven on the way to a larger one. Proving the lemma once, and then using it, is often what
turns a proof that will not go through into one that does.

The set's ends may mention other values, as a header's may. This claims that `i` lies in a
window of the table, and nothing says so:

<!-- file docs/book/code/claims/no-between.oro -->
```go
export fn pick(t ∈ Array(Int), lo ∈ Int, hi ∈ Int, i ∈ Int) (Int) where 0 <= lo && lo <= hi && hi <= len(t) {
    k ∈ lo..hi-1 = i
    t[k]
}
```

<!-- gen go docs/book/code/claims/no-between.oro -->
```
gen: claims-pick: the claim k ∈ lo..hi - 1 is not proven: lo <= i && i <= hi - 1 does not follow
  known: assumed -lo <= 0, assumed -hi + lo <= 0, assumed hi + -len(t) <= 0
```

The message turns the set into the two inequalities it stands for. Add `lo <= i && i < hi`
to the `where` and the claim is proven, and with it `t[k]`.

And a claim is checked whether or not anything uses the name:

<!-- file docs/book/code/claims/no-idle.oro -->
```go
export fn f(n ∈ 0..100) (Int) {
    k ∈ 0..5 = n
    0
}
```

<!-- gen go docs/book/code/claims/no-idle.oro -->
```
gen: claims-f: the claim k ∈ 0..5 is not proven: 0 <= n && n <= 5 does not follow
  known: assumed -n <= 0, assumed n + -100 <= 0
```

`k` is never read, and the answer is 0 whatever `n` is. The program is refused anyway,
because it says something false. A claim is not a comment: every one in the source is true,
or there is no program.

---

## 15.7 `ensure`

A header's `ensure` states something about the result by name:

<!-- file docs/book/code/claims/ensure.oro -->
```go
export fn abs(n ∈ -1000..1000) (r ∈ Int) ensure r >= 0 {
    if n < 0 { -n } else { n }
}
```

<!-- gen go docs/book/code/claims/ensure.oro -->
```go
func ClaimsAbs(v0 int) int {
    v10 := (v0 < 0)
    if v10 {
        v13 := (0 - v0)
        return v13
    }
    return v0
}
```

In the first branch `n < 0`, so `-n` is positive. In the second `n >= 0`. Either way the
result is not negative, and the interval of the result shows it.

What `ensure` can say today is narrow: the result compared with *constants*. A promise
that relates the result to the parameters is refused as outside what is decided:

<!-- file docs/book/code/claims/no-ensure.oro -->
```go
export fn clamp(x ∈ Int, lo ∈ Int, hi ∈ Int) (r ∈ Int) where lo <= hi ensure lo <= r && r <= hi {
    case { x < lo -> lo; x > hi -> hi; else -> x }
}
```

<!-- gen go docs/book/code/claims/no-ensure.oro -->
```
gen: clamp: the postcondition lo <= result && result <= hi is outside what the compiler decides (constant bounds on the result, and their conjunctions); an obligation is discharged or the program is refused (refinements.md §3a)
```

`clamp` is correct, and "the result is between `lo` and `hi`" is exactly what one wants to
say about it. The message is honest about why it is refused: not "this is false" but "this
is outside what I decide". It matters less than it looks, because a function inside a
program is unfolded where it is called (chapter 2), and the caller then sees the *body*, not
a summary of it: every fact the body establishes is available. `ensure` is for the edge,
where a promise is made to someone who cannot see inside.

---

## 15.8 The edge of the method

The two methods are strong on one kind of statement: sums of unknowns, each multiplied by a
constant, compared with `<=`, `<`, `==`. That is called **linear** arithmetic. Outside it
the compiler knows less than you do, and it is worth collecting, in one place, every wall
this book has walked into.

| the true thing | why it is not found | seen in |
|---|---|---|
| `n * n >= 0` | a product of two unknowns is not linear | below |
| `row * width + col >= 0` | the same | 12 |
| `p * i` stays small over 20 trips | intervals multiply the bounds: 20²⁰ | 11 |
| `n mod 2` is 0 or 1, as a position | the same, for `mod` | below |
| `a != b` gives `a - b != 0` | "not equal" is two cases, not one inequality | below |
| `n < 0xD800 \|\| n > 0xDFFF` | an "or" in a fact is two cases | 9 |
| `hi - lo` shrinks from both ends | termination looks for one variable moving | 12 |

<!-- file docs/book/code/claims/no-square.oro -->
```go
export fn square(n ∈ -1000..1000) (r ∈ Int) ensure r >= 0 { n * n }
```

<!-- gen go docs/book/code/claims/no-square.oro -->
```
gen: square: the body does not establish result >= 0; its result is [-1000000, 1000000]
```

A square is never negative, and a child knows it. As intervals, −1000..1000 times
−1000..1000 is −1000000..1000000, because the method does not know the two factors are the
same number.

<!-- file docs/book/code/claims/no-mod.oro -->
```go
export fn parity(n ∈ 0..100, t ∈ Array(Int)) (Int) where len(t) == 2 { t[n mod 2] }
```

<!-- gen go docs/book/code/claims/no-mod.oro -->
```
gen: claims-parity: t[n mod 2] is an indexing, and 0 <= n mod 2 does not follow
  known: assumed -n <= 0, assumed n + -100 <= 0
```

Chapter 6 said `mod` is the remainder that is *never negative*. That is its definition, and
here it is not known. The other remainder is, for a dividend that is not negative, so this
compiles:

<!-- file docs/book/code/claims/parity.oro -->
```go
export fn parity(n ∈ 0..100, t ∈ Array(Int)) (Int) where len(t) == 2 { t[n % 2] }
```

<!-- gen go docs/book/code/claims/parity.oro -->
```go
func ClaimsParity(v0 int, v1 []int) int {
    v17 := (v0 & 1)
    v18 := v1[v17]
    return v18
}
```

<!-- file docs/book/code/claims/no-unequal.oro -->
```go
export fn ratio(a ∈ 0..9, b ∈ 0..9) (Int) where a != b { 100 / (a - b) }
```

<!-- gen go docs/book/code/claims/no-unequal.oro -->
```
gen: claims-ratio: `/` requires a - b != 0, which does not follow
  known: assumed ne(a,b), assumed -a <= 0, assumed a + -9 <= 0, assumed -b <= 0, assumed b + -9 <= 0
```

The fact is right there in the list, `ne(a,b)`, and the goal is the same statement moved
across the equals sign. "Not equal" means "less *or* greater", two cases, and the method
works with single inequalities. Write `a < b` (15.4's `strict`) and it is one.

Some of these are gaps that will close: nothing deep stops a compiler learning that a
square is not negative. But the wall as a whole is not an accident, and the next section is
why.

---

## 15.9 Sound, and not complete

Two words describe any checker.

It is **sound** if everything it accepts is true. It is **complete** if everything true is
accepted.

This compiler aims to be sound and makes no attempt to be complete. That is a choice, and
it is forced. In 1929 Mojżesz Presburger proved that statements of linear arithmetic over
the whole numbers, the kind in 15.4, can *always* be decided: there is a procedure that
answers true or false for every one. Add multiplication of unknowns and the picture
changes completely. In 1970 Yuri Matiyasevich finished the proof that no procedure can
decide whether an equation in whole numbers with multiplication has a solution. And in 1953
Henry Rice had shown the general case: *every* interesting question about what a program
computes is undecidable. Chapter 11's halting problem was the first instance.

So any checker for a real language faces the same three-way choice, and can have only two:

| | accepts only true claims | accepts every true claim | always answers |
|---|---|---|---|
| a checker that guesses | no | yes | yes |
| a prover that searches forever | yes | yes | no |
| **this compiler** | **yes** | **no** | **yes** |

It always answers, quickly, and "proven" is always right. The price is the third thing: it
sometimes answers "does not follow" about something true.

That changes how to read a refusal. **"Does not follow" is not "this is wrong".** It is "I
could not show this from what I have". Sometimes the program is wrong, and the compiler has
just saved you (15.4's `no-strict`, every off-by-one in chapter 12). Sometimes the program
is right and the compiler is short of a fact or of a method. You have three responses, and
you have used all three.

**Say more in a header.** Narrow a parameter's set, add a `where`. It costs nothing at run
time and makes the function's assumptions part of its interface. Most refusals end here.

**Test.** Write the condition as a guard: `if 0 <= k && k < len(t) { … } else { … }`. The
test runs, so it costs a comparison, and it makes the program say what happens when the
condition is false. Chapter 11's "a guard you are sure of" and chapter 12's binary search
are this. It is the right answer when the fact comes from outside, and an honest answer
when it is true but beyond the method.

**Ask for a trap.** For arithmetic that might leave the word, the `-checked` flag emits the
host's own overflow check in place of a proof:

<!-- file docs/book/code/claims/no-add.oro -->
```go
export fn add(a ∈ Int, b ∈ Int) (Int) { a + b }
```

<!-- gen go docs/book/code/claims/no-add.oro -->
```
gen: claims-add: 1 of 1 integer operation(s) cannot be proven to stay inside the word of target go, [-9223372036854775808, 9223372036854775807] and [0, 18446744073709551615]
  + [-inf, +inf] in a + b
```

<!-- gen go docs/book/code/claims/no-add.oro -checked -->
```go
func ClaimsAdd(v0 int, v1 int) int {
    v2 := (func(a, b int) int { t := a + b; if (t > a) != (b > 0) { panic("int overflow") }; return t }(v0, v1))
    return v2
}
```

Any two integers may overflow when added, so without the flag the function is refused. With
it, the addition is done and then checked, and the program stops if it wrapped. That is
chapter 14's `expect`, supplied by the build line: the program still never computes a wrong
number, it has only traded a proof for a possible stop. `-checked` covers arithmetic and
nothing else. A position in a table or a divisor is proven, or guarded by a test you write.

The order of those three is the order to try them in.

---

## 15.10 A type is a set

One more thing was quietly true all along. This book has never used the word *type* for
something separate from *set*. In `fn f(n ∈ 0..9, s ∈ String, t ∈ Array(Int))`, the three
things after `∈` are all sets, written in the same place in the same way: a range, a named
set, a set of tables. There is no second language for types.

The coarse sets are checked first, and a mismatch between them is what other languages call
a type error:

<!-- file docs/book/code/claims/no-text.oro -->
```go
export fn plus(n ∈ 0..9) (Int) { n + "one" }
```

<!-- gen go docs/book/code/claims/no-text.oro -->
```
gen: plus: in argument 2 of `+`: a string literal is String, but Int is required here
```

<!-- file docs/book/code/claims/no-number.oro -->
```go
export fn name(n ∈ 0..9) (String) { n }
```

<!-- gen go docs/book/code/claims/no-number.oro -->
```
gen: name: n is 0..9, but String is required here
```

<!-- file docs/book/code/claims/no-table.oro -->
```go
export fn bump(t ∈ Array(Int)) (Int) { t + 1 }
```

<!-- gen go docs/book/code/claims/no-table.oro -->
```
gen: bump: in argument 1 of `+`: t is Array(Int), but Int is required here
```

"`n` is 0..9, but String is required": a number is not a member of the set of strings. That
is the same *kind* of statement as "5 is not a member of 0..3", only coarser, and settled by
a cheaper check that needs no arithmetic. A classical type system is the part of this
chapter that stops at the coarse sets. This language keeps going.

Sets are built with the notation chapter 6 began:

| written | the set |
|---|---|
| `Int`, `Nat`, `Bool`, `String`, `F64` | the basic ones |
| `lo..hi`, `lo..`, `..hi` | a range of integers |
| `D(k, lo..hi)` | decimals with `k` places (chapter 8) |
| `Array(S)` | tables with elements in `S` |
| `S × T` | pairs (chapter 10) |
| `'a ∪ 'b(T)` | a variant (chapter 10) |
| `set { x ∈ S : φ }` | the members of `S` for which φ holds |

The last is the general one, and it is how mathematics writes a set by a property:

<!-- file docs/book/code/claims/types.oro -->
```go
set Small = set { n ∈ Int : 0 <= n && n < 10 }

export fn twice(n ∈ Small) (0..18) { n + n }
```

<!-- gen go docs/book/code/claims/types.oro -->
```go
func ClaimsTwice(v0 int) int {
    v9 := (v0 + v0)
    return v9
}
```

`Small` is "the integers `n` with `0 <= n < 10`", and the compiler read it as the range
0..9: `n + n` was proven in 0..18. A set written by a property is a claim like any other.
Its formula becomes a fact where a value is known to be in the set, and an obligation where
a value must be shown to be.

---

## 15.11 What it costs

Go back through this chapter's emitted code and look for the claims. `chain`'s three-part
`where` is one table read. `gap`'s claim on a binding is one subtraction. `third`'s header,
the call's obligation and `at`'s range are `v0[2]`. `twice`'s set is `v0 + v0`.

**A claim that is proven is erased.** It is used once, while compiling, and then it is
gone. The program that runs is the program you would have written in C with no checks at
all, and the difference is that someone checked.

That is the other half of chapter 6's bargain. A language that checks at run time (Go's
index checks, Java's exceptions) pays a little on every operation, forever, and still finds
the bug only when the bad input arrives, in production. A language that does not check (C)
pays nothing and finds the bug when an attacker does. Proof is the one arrangement that
costs nothing at run time and finds the bug before the program exists. What it costs is
this chapter: a compiler that sometimes needs to be told a thing you consider obvious.

---

## 15.12 What other languages do

| | what can be stated | who checks | when | if it cannot be shown |
|---|---|---|---|---|
| Oroboros | sets, ranges, linear relations | the compiler's own procedures | compiling | refused; add a fact, a test or `-checked` |
| C, Go, Java | coarse types | a type checker | compiling | n/a: finer facts cannot be stated |
| Eiffel, and `assert` anywhere | any condition | the running program | at run time | the program stops |
| Ada SPARK, Dafny, Why3 | any formula | an external theorem prover (SMT) | compiling | a timeout or "unknown"; add annotations |
| Liquid Haskell, F* | types refined by formulas | an SMT solver | compiling | the same |
| Rust | types, ownership, lifetimes | the compiler | compiling | refused for ownership; bounds are checked at run time |
| Coq, Lean, Agda | anything | a small trusted kernel | compiling | you write the proof yourself |

The nearest relatives are in the fourth and fifth rows: languages where a claim is a
formula and a machine proves it. They state far more than this language can (loop
invariants of any shape, properties of data structures), and they pay with an external
solver whose answers depend on its version and its time limit. A proof that went through
yesterday can time out today.

This language chose the other side of that trade. It decides a smaller class of statements,
with procedures that are part of the compiler, always finish and give the same answer every
time. What it cannot decide it says so, in a message that lists what it knew.

The idea that a type can be a set with a formula attached, checked by a decision procedure,
is called *refinement types* and dates from 1991 (Freeman and Pfenning). The procedure for
linear integer arithmetic that the compiler's is checked against is William Pugh's *Omega
test* (1991), a descendant of Fourier's.

---

## What to remember

- **Every claim is an obligation**: a formula that must hold at one place for every input
  the headers allow. Proven, or the program is refused.
- **Parameter sets and `where` are obligations at calls and facts in the body.** Result sets
  and `ensure` are the reverse. An export's parameter claims are assumed.
- **Intervals** track each value's least and greatest. They decide most arithmetic, and a
  function's result claim. They cannot see that two values are related.
- **Facts** are formulas known at a point, shown on the `known:` line in the form `… <= 0`.
  The compiler combines them by adding inequalities to cancel unknowns.
- **Facts come from** headers, conditions, equations (`y = x + 1`), the data's own laws
  (`len(t) >= 0`, an element's set), and loop invariants the compiler finds.
- **`loop … invariant I { … }`** states an invariant the compiler did not guess. It must
  hold on entry and be kept by every `again`. Then it is a fact in the loop, and it never
  runs.
- **`x ∈ S = e` claims a binding.** Use it to get a relational fact to a result, and to ask
  the compiler what it knows.
- **`ensure` compares the result with constants.**
- **The methods are for linear arithmetic.** Products of unknowns, quotients, `mod`, "not
  equal" and "or" are where true things get refused.
- **The compiler is sound and not complete.** "Does not follow" means "I could not show
  it". Then: say more in a header; or test, and say what happens otherwise; or, for
  arithmetic, build with `-checked`.
- **A type is a set.** A type error is the coarsest kind of refused membership.
- **A proven claim is erased.** It costs nothing at run time.

Next: [chapter 16](16-targets.md), Targets. Where `io.print_line` comes from, and how to describe a host's
function so that the compiler can check what you say about it.
