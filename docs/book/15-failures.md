# 15. Failures

Every function so far has done what it was asked. Give `half` an even number and it halves
it. But ask for half of 7, or for the contents of a file that is not there, or for the
number written in the text `"12x"`, and there is no answer to give.

This is not a corner of programming. Most of the code in a serious program is about what
happens when something does not work, and most of the bugs that matter are in it, because
it is the code that runs least and is tested least. So the question of this chapter is:
**what does a function say when it cannot do what it was asked, and how do you make sure
someone listens?**

The answer is three sentences.

> **A failure is a value.** A function that can fail returns a variant: one case carrying the
> answer, the others saying what went wrong.
>
> **`try` goes on if a step succeeded and passes any failure straight out. `expect` goes on if
> it succeeded and stops the program if not, with a reason.**
>
> **A result may not be dropped.** It is looked at, passed on, or discarded on purpose with
> `ignore`.

Along the way: the three older answers and what is wrong with each, a rocket lost to an
unheard error, why "it cannot fail" should be written down as a sentence in the program, and
what a file system can say besides yes.

The examples are in [code/fail/](code/fail/). `go run docs/book/check.go
docs/book/15-failures.md` re-runs every block. Checked 2026-11-16.

---

## 15.1 Three answers that do not work

**Return a special number.** C's `getchar` returns −1 at the end of input; `indexOf` returns
−1 for "not found". Chapter 10 met this: the special number is also an ordinary number, and
sooner or later someone uses it as one. A price of −1. A position of −1, used as an index.

**Return nothing special and set a flag somewhere.** C's library does this too: the function
returns something, and a global variable called `errno` says whether to believe it. Nothing
makes anyone look at the flag, and mostly nobody does.

**Throw an exception.** The function does not return at all: control jumps to whatever
enclosing code said it would handle that kind of failure, however far away. This is the
answer of Java, Python, JavaScript and C++, and it fixes the first two problems: a failure
cannot be mistaken for an answer, and cannot be silently ignored, since an unhandled one
stops the program. Its cost is that it is invisible. Looking at a line `total = price(x) +
tax(x)` you cannot tell whether it can jump out of the function, or from where. Every call
is a possible exit, and code that must leave things tidy (close the file, release the lock)
has to be written as though any line might be its last.

On 4 June 1996 the first Ariane 5 rocket turned over and broke up 37 seconds after launch.
Its guidance software had converted a number to a 16-bit integer, the number did not fit,
and the conversion raised an exception that nothing was written to handle. The guidance
computer shut down, and so did its identical backup, for the identical reason. The value
was not even needed after lift-off.

What all three share: **the possibility of failure is not in the function's header.** You
find out by reading the manual, or by an outage.

---

## 15.2 A failure is a value

Chapter 10 already has the right tool. "An answer, or a reason there is none" is a variant
with two cases.

<!-- file docs/book/code/fail/half.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

fn show(r) { case r { result.'ok(v) -> io.print_int(v); result.'err(e) -> io.print_int(-e) } }

export fn main() {
    show(half(10))
    show(half(7))
    show(half(0))
}
```

<!-- run go docs/book/code/fail/half.oro -->
```
5
-7
0
```

`result` is a library module, a few lines long. It declares

```
pub set Result(T, E) = 'ok(T) ∪ 'err(E) success 'ok relevant
```

a variant with two cases and two parameters: **`'ok(T)`** carries an answer of kind `T`, and
**`'err(E)`** carries a description of the failure, of kind `E`. (The two words after the
cases, `success` and `relevant`, are what this chapter is about. They come in 15.3 and 15.5.)
Since it lives in another module, its tags are written `result.'ok` and `result.'err`
(chapter 10).

`half` answers `result.'ok(n / 2)` for an even number and `result.'err(n)` for an odd one,
handing back the number it could not halve. `show` takes a result apart with `case` and
prints the answer, or the offending number with a minus sign. Half of 0 is 0, a perfectly
good answer that nobody will mistake for a failure.

Everything chapter 10 said applies. The only way to the answer is a `case` that has an arm
for `'err`. A caller cannot forget that `half` may fail, because there is no `half(n) + 1`:
a result is not a number.

Compare `Maybe` from chapter 10. `'nothing` says *that* there is no answer. `'err(e)` says
*why*. When there is only one possible reason, use the first. When the caller might want to
know which of several things went wrong, use the second.

---

## 15.3 Passing a failure on: `try`

One step that can fail is easy. Real work is several in a row, each needing the one before:
open the file, *then* read it, *then* parse what was read. If any step fails, the whole
thing fails, for that step's reason.

An eighth of a number is half of half of half. With `case`:

<!-- file docs/book/code/fail/pyramid.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

fn show(r) { case r { result.'ok(v) -> io.print_int(v); result.'err(e) -> io.print_int(-e) } }

fn eighth(n ∈ 0..1000000) {
    case half(n) {
        result.'err(e) -> result.'err(e)
        result.'ok(a) -> case half(a) {
            result.'err(e) -> result.'err(e)
            result.'ok(b) -> half(b)
        }
    }
}

export fn main() {
    show(eighth(80))
    show(eighth(20))
    show(eighth(6))
    show(eighth(7))
}
```

<!-- run go docs/book/code/fail/pyramid.oro -->
```
10
-5
-3
-7
```

80 halves three times to 10. 20 halves to 10, then to 5, and 5 is odd: the failure names 5.
6 fails at 3. 7 fails at once.

It works, and look at its shape. Half the lines say `result.'err(e) -> result.'err(e)`: *if
it failed, fail the same way.* Each step pushes the next one further to the right. Ten steps
would be a staircase off the edge of the screen, and nine of its lines would be that same
line. (Go programmers will recognize `if err != nil { return err }`, which is the same line
in another spelling.)

**`try`** is that line, written once:

<!-- file docs/book/code/fail/try.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

fn show(r) { case r { result.'ok(v) -> io.print_int(v); result.'err(e) -> io.print_int(-e) } }

fn eighth(n ∈ 0..1000000) {
    try result.'ok(a) = half(n) {
        try result.'ok(b) = half(a) {
            half(b)
        }
    }
}

export fn main() {
    show(eighth(80))
    show(eighth(20))
    show(eighth(6))
    show(eighth(7))
}
```

<!-- run go docs/book/code/fail/try.oro -->
```
10
-5
-3
-7
```

Read `try result.'ok(a) = half(n) { … }` as: *compute `half(n)`. If it is an `'ok`, call
what it carries `a` and go on with the block. If it is anything else, that is my answer:
stop here and hand it out unchanged.*

The same four results. A `try` *is* the `case` above, with the pass-it-on arm written for
you: no more, no less. What is left on the page is the path where everything works, which is
the path you want to read, with each possible failure point marked by the word `try`. That
is the difference from an exception: every place this function can leave early is visible,
and it says `try`.

The block of a `try` must itself give a result, since "everything worked" has to be an
`'ok` and a later failure an `'err`. Here the innermost block is `half(b)`, which is one.

### Which case is success?

`try` continues on one case and leaves on all the others. How does it know which is which?
The declaration says: **`success 'ok`**. That is the first of the two extra words.

A variant with no case marked cannot be used with `try`:

<!-- file docs/book/code/fail/no-success.oro -->
```go
set Light = 'red ∪ 'green

fn pick(n ∈ 0..1) { if n == 0 { 'red } else { 'green } }

export fn f(n ∈ 0..1) (Light) { try 'red = pick(n) { 'green } }
```

<!-- gen go docs/book/code/fail/no-success.oro -->
```
gen: docs/book/code/fail/no-success.oro: f: try: variant light declares no (success …) constructor, so it is not a model of the exception monad (spec/errors.md §2)
```

and `try` may name only the case that is marked:

<!-- file docs/book/code/fail/no-err.oro -->
```go
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn f(n ∈ 0..1000000) (result.Result(Int, Int)) {
    try result.'err(e) = half(n) { result.'ok(e) }
}
```

<!-- gen go docs/book/code/fail/no-err.oro -->
```
gen: docs/book/code/fail/no-err.oro: f: try: err is not the success constructor of variant result, which is ok: try continues on success alone
```

("The exception monad" in the first message is the mathematicians' name for exactly this
pattern: computations that either give a value or stop with a reason, chained so that the
first stop wins.)

### What it costs

<!-- file docs/book/code/fail/quarter.oro -->
```go
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn quarter(n ∈ 0..1000000) (result.Result(Int, Int)) {
    try result.'ok(h) = half(n) { half(h) }
}
```

<!-- gen go docs/book/code/fail/quarter.oro -->
```go
func FailQuarter(v0 int) (int, int, int) {
    v10 := (v0 & 1)
    v12 := (v10 == 0)
    if v12 {
        v14 := (v0 >> 1)
        v16 := (v14 & 1)
        v18 := (v16 == 0)
        if v18 {
            v20 := (v14 >> 1)
            return 0, v20, 0
        }
        return 1, 0, v14
    }
    return 1, 0, v0
}
```

Two tests of "is it even?", which the problem requires, and nothing else. No result was
built by the first `half` and inspected by the `try`: chapter 10's law (a `case` applied to
a choice is a choice between `case`s) dissolved it. Each failure is a plain early `return`.
This is what you would write by hand in Go, and what an exception costs a great deal more
than.

The result exists only at the export's edge, as chapter 10's tag and slots: 0 or 1 for
`'ok` or `'err`, a slot for the answer, a slot for the reason.

---

## 15.4 A step that needs an earlier answer

In `eighth` each step used the answer of the one just before. Often a later step needs
answers from *two* earlier ones: half of `a` plus half of `b`. Put one `try` inside the
other, and the inner block can see both names:

<!-- file docs/book/code/fail/both.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

fn show(r) { case r { result.'ok(v) -> io.print_int(v); result.'err(e) -> io.print_int(-e) } }

fn sum_halves(a ∈ 0..1000000, b ∈ 0..1000000) {
    try result.'ok(x) = half(a) {
        try result.'ok(y) = half(b) { result.'ok(x + y) }
    }
}

export fn main() {
    show(sum_halves(40, 4))
    show(sum_halves(40, 5))
    show(sum_halves(41, 4))
}
```

<!-- run go docs/book/code/fail/both.oro -->
```
22
-5
-41
```

20 + 2 is 22. When the second number is odd the failure names it, and when the first is odd
the second is never looked at: the outer `try` has already given its answer.

Read the nesting as a sentence. "Try for `x`. If that worked, try for `y`. If that worked
too, the answer is `x + y`." Each `try` is one place the function may leave, and the block
after it is everything that happens if it does not.

### Several steps in one `try`

Three steps would be three levels of braces, and ten would march off the right edge of the
page. So a `try` takes a list. The steps are separated by commas, run from left to right,
and the first that is not a success is the answer:

<!-- file docs/book/code/fail/several.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

fn show(r) { case r { result.'ok(v) -> io.print_int(v); result.'err(e) -> io.print_int(-e) } }

fn sum_halves(a ∈ 0..1000000, b ∈ 0..1000000) {
    try result.'ok(x) = half(a), result.'ok(y) = half(b) { result.'ok(x + y) }
}

fn three(a ∈ 0..1000000, b ∈ 0..1000000) {
    try result.'ok(x) = half(a), result.'ok(y) = half(b), result.'ok(z) = half(y) {
        result.'ok(x + y + z)
    }
}

export fn main() {
    show(sum_halves(40, 4))
    show(three(40, 4))
    show(three(40, 6))
    show(three(41, 6))
}
```

<!-- run go docs/book/code/fail/several.oro -->
```
22
23
-3
-41
```

`sum_halves` is the function above with one pair of braces fewer. The two spellings mean
exactly the same thing: a `try` with a list *is* the nested `try`s, written flat.

`three` shows what that buys. Its third step is `half(y)`, and `y` is the name the second
step bound: **each step can use the names of the steps before it.** With 40 and 4 the
halves are 20 and 2, and half of 2 is 1, so 23. With 40 and 6 the second step gives 3, and 3
has no half: the third step fails and names the 3. With 41 the first step fails, and neither
of the others is run.

This is the shape most real work has: open the file, then read it, then parse what was
read, then check the total. Each step needs what the one before produced, any of them can
fail, and the reader should be able to see the whole sequence at a glance.

---

## 15.5 A result must be used

Failure-as-a-value has one weakness that exceptions do not: a value can be thrown away. Call
a function that can fail, ignore what it returns, and you have C's `errno` again. This is
not hypothetical. C programs that never check whether `write` or `close` succeeded are the
rule, and files have been silently lost to them for fifty years.

So the second extra word on `Result` is **`relevant`**: a value of this variant *must be
used*.

<!-- file docs/book/code/fail/no-dropped.oro -->
```go
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn f(n ∈ 0..1000000) (Int) {
    half(n)
    0
}
```

<!-- gen go docs/book/code/fail/no-dropped.oro -->
```
gen: the result of half is a value of a relevant type (spec/errors.md §7), and it is discarded: use it, or discard it with ignore(…)
```

Giving it a name and then not reading the name is the same thing:

<!-- file docs/book/code/fail/no-unread.oro -->
```go
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn f(n ∈ 0..1000000) (Int) {
    r = half(n)
    0
}
```

<!-- gen go docs/book/code/fail/no-unread.oro -->
```
gen: the result of half is a value of a relevant type (spec/errors.md §7), and the binder r is not used on every path: use it, or discard it with ignore(…)
```

"On every path." Looking at the result in one branch of an `if` and not the other leaves a
way through the program on which the failure is never heard:

<!-- file docs/book/code/fail/no-one-path.oro -->
```go
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn f(n ∈ 0..1000000, loud ∈ Bool) (Int) {
    r = half(n)
    if loud { case r { result.'ok(v) -> v; result.'err(e) -> -1 } } else { 0 }
}
```

<!-- gen go docs/book/code/fail/no-one-path.oro -->
```
gen: the result of half is a value of a relevant type (spec/errors.md §7), and the binder r is not used on every path: use it, or discard it with ignore(…)
```

Sometimes you really do not care whether it worked: a log message that could not be written,
a temporary file that could not be removed. Then say so. **`ignore(e)`** is the one way to
drop a value that must be used, and it is a word a reader can search for:

<!-- file docs/book/code/fail/ignore.oro -->
```go
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn quiet(n ∈ 0..1000000, loud ∈ Bool) (Int) {
    r = half(n)
    if loud { case r { result.'ok(v) -> v; result.'err(e) -> -1 } } else { ignore(r); 0 }
}

export fn or_zero(n ∈ 0..1000000) (Int) {
    case half(n) { result.'ok(h) -> h; result.'err(e) -> 0 }
}
```

<!-- gen go docs/book/code/fail/ignore.oro -->
```go
func FailOrZero(v0 int) int {
    v10 := (v0 & 1)
    v12 := (v10 == 0)
    if v12 {
        v14 := (v0 >> 1)
        return v14
    }
    return 0
}

func FailQuiet(v0 int, v1 bool) int {
    if v1 {
        v11 := (v0 & 1)
        v13 := (v11 == 0)
        if v13 {
            v15 := (v0 >> 1)
            return v15
        }
        return -1
    }
    return 0
}
```

`ignore` emits nothing. It is a statement to the compiler and to the next person who reads
the code: *I know this can fail, and here it does not matter.* `or_zero` shows the other
kind of "handling": a `case` that turns a failure into an ordinary value, on purpose.

Chapter 5 had three things reduction may not do to an effect: drop it, copy it, move it.
`relevant` is the first of those rules applied to a *value*. The logic of things that may
not be dropped has a name, relevance logic, and that is where the word is from.

---

## 15.6 Giving up on purpose: `expect`

Some failures are part of the job. The user typed a file name that does not exist: tell them
and carry on. Others mean the program itself is wrong. The configuration this program wrote
a second ago cannot be read back. A table that is sorted by construction turns out not to
be. There is nothing sensible to carry on *with*.

For those there is **`expect`**:

<!-- file docs/book/code/fail/must.oro -->
```go
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn must(n ∈ 0..1000000) (Int) {
    expect result.'ok(h) = half(n) else "an even number is expected"
    h
}
```

<!-- gen go docs/book/code/fail/must.oro -->
```go
func FailMust(v0 int) int {
    v10 := (v0 & 1)
    v12 := (v10 == 0)
    if v12 {
        v14 := (v0 >> 1)
        return v14
    }
    panic("an even number is expected")
}
```

<!-- gen js docs/book/code/fail/must.oro -->
```js
export function failMust(v0) {
    if (((v0 & 1) === 0)) {
        return (v0 >> 1);
    }
    throw new Error("an even number is expected");
}
```

Read `expect result.'ok(h) = half(n) else "…"` as: *I expect this to succeed. Call the
answer `h` and go on. If it did not, stop the program and say this.*

After the `expect` line, `h` is a plain integer and the rest of the function has no failure
in it at all: `must` returns `Int`, not a result. The price is that the "no" case ends
everything. Each host does it in its own way, `panic` on Go and a thrown `Error` on
JavaScript, and each prints the reason.

<!-- file docs/book/code/fail/crash.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

export fn main() {
    expect result.'ok(h) = half(10) else "ten is even"
    io.print_int(h)
    expect result.'ok(k) = half(7) else "seven is not even"
    io.print_int(k)
    io.print_line("never printed")
}
```

<!-- run windows docs/book/code/fail/crash.oro -->
```
5
seven is not even
```

The first `expect` held. The second did not: the program printed its reason and ended, and
the last two lines never ran.

The reason is not optional, and that is the point of the construct. `expect` is the one
place in the language where you are allowed to say "trust me". The sentence after `else` is
you writing down *why* you believe it, for the person who will read it on the day it turns
out to be false. `"unreachable"` and `"cannot happen"` are bad reasons. `"the config file was
written by this run"` is a good one.

`try` or `expect`? Ask who is at fault if it fails.

| if it fails, the fault is… | use | because |
|---|---|---|
| the world's: bad input, a missing file, a full disk | `try`, or `case` | the program should say so and carry on, or let its caller decide |
| the program's: something its own logic guarantees turned out false | `expect` | carrying on with a broken assumption does more harm than stopping |

Stopping at the first broken assumption sounds drastic, and it is one of the better ideas in
the field. The telephone switches Erlang was built for in the 1980s ran for years without a
break on the rule "let it crash": a part that finds itself confused stops at once, and
something outside it starts a fresh copy. The alternative, a part that keeps running on
values it knows are wrong, is how small faults become large ones.

---

## 15.7 Failure in a loop

A `try` may be the last thing in a loop's arm, with `again` inside its block. If the step
succeeds the loop goes round. If it fails, the failure is the loop's answer, and the loop is
over:

<!-- file docs/book/code/fail/loop.oro -->
```go
use io
use result

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

fn show(r) { case r { result.'ok(v) -> io.print_int(v); result.'err(e) -> io.print_int(-e) } }

fn small(v) { if 0 <= v && v <= 1000000 { v } else { 1 } }

fn halves(t) {
    loop i = 0, acc = 0 {
        i >= len(t) || acc > 1000000 -> result.'ok(acc)
        else -> {
            try result.'ok(h) = half(small(t[i])) {
                again(i + 1, acc + h)
            }
        }
    }
}

export fn main() {
    show(halves(array { 2, 4, 6 }))
    show(halves(array { 2, 5, 6 }))
    show(halves(array { }))
}
```

<!-- run go docs/book/code/fail/loop.oro -->
```
6
-5
0
```

Half of each of 2, 4 and 6, added up: 6. In the second table the 5 stops everything, and the
6 after it is never looked at. That is "do this to every item, and stop at the first one
that cannot be done", which is what processing a file line by line is.

Chapter 12 said `again` may stand only at the end of an arm. The block of a `try` counts,
because it runs once, at once, and nothing happens after it. So the loop's arms are still
its complete list of ways round, and the `try` adds one way out.

---

## 15.8 The world can say no

So far the failures were invented. Here is a real one. Reading a file can fail in ways no
program can prevent, and the library function says so in what it returns:

<!-- file docs/book/code/fail/file.oro -->
```go
use io
use os
use result

fn size(path) {
    try result.'ok(data) = os.ReadFile(path) { result.'ok(len(data)) }
}

fn report(r) {
    case r {
        result.'ok(n) -> io.print_int(n)
        result.'err(why) -> case why {
            os.'not_found -> io.print_int(-1)
            os.'exists -> io.print_int(-2)
            os.'permission -> io.print_int(-3)
            os.'other(e) -> io.print_int(-4)
        }
    }
}

export fn main() {
    report(size("docs/book/code/fail/hello.txt"))
    report(size("docs/book/code/fail/no-such-file.txt"))
}
```

<!-- run go docs/book/code/fail/file.oro -->
```
12
-1
```

**`os.ReadFile(path)`** gives a result: `'ok` carrying the file's bytes (chapter 13's
`Array(0..255)`), or `'err` carrying the reason. `size` uses `try` to turn the bytes into
their count and let any failure through. `hello.txt` holds the twelve bytes of `hello, file`
and a line end. The other file does not exist.

The reason is itself a variant, declared in `os`:

```
pub set OsError = 'not_found ∪ 'exists ∪ 'permission ∪ 'other(error)
```

and `report`'s inner `case` must answer for all four (chapter 10). That is worth dwelling
on. With exceptions, "what can this call throw?" is answered by documentation that may be
out of date. Here it is answered by a set, and a `case` that forgets a member does not
compile. The three named kinds are the ones a program can usually do something about. The
fourth, `'other`, carries the host's own error for everything else.

<!-- run js docs/book/code/fail/file.oro -->
```
12
-1
```

<!-- run windows docs/book/code/fail/file.oro -->
```
12
-1
```

One source, three hosts. Go reports a missing file as a returned value, JavaScript throws
an exception for it, and the Windows system call returns 0 and leaves a code to be fetched.
The `os` module turns each into the same `os.'not_found`.

Writing a file can fail too, and that result is `relevant`:

<!-- file docs/book/code/fail/no-save.oro -->
```go
use os

export fn save(path ∈ String, data ∈ Array(0..255)) (Int) {
    os.WriteFile(path, data)
    0
}
```

<!-- gen go docs/book/code/fail/no-save.oro -->
```
gen: the result of os.WriteFile is a value of a relevant type (spec/errors.md §7), and it is discarded: use it, or discard it with ignore(…)
```

This is the fifty-year-old C bug, refused. The program believed it had saved the user's
work and never asked. Ask:

<!-- file docs/book/code/fail/save.oro -->
```go
use os
use result

export fn save(path ∈ String, data ∈ Array(0..255)) (Int) {
    case os.WriteFile(path, data) { result.'ok(u) -> 0; result.'err(why) -> 1 }
}
```

<!-- gen go docs/book/code/fail/save.oro -->
```go
func FailSave(v0 string, v1 []byte) int {
    v2 := (os.WriteFile(v0, v1, 0o666))
    v3 := (v2 == nil)
    if v3 {
        return 0
    }
    return 1
}
```

<!-- gen js docs/book/code/fail/save.oro -->
```js
export function failSave(v0, v1) {
    let v2;
    try { fs.writeFileSync(v0, Buffer.from(v1)); v2 = null; } catch (e) { v2 = e; }
    if ((v2 === null)) {
        return 0;
    }
    return 1;
}
```

On Go, `os.WriteFile` returns an `error` that is `nil` when all went well, and the `case` is
the test `v2 == nil`: Go's own convention, used as it stands. Nothing was wrapped. On
JavaScript the host throws, and the one `try`/`catch` in the emitted code is the bridge that
turns the exception back into a value. (`'ok(u)` carries the empty tuple `()`: a write that
worked has nothing to report.)

---

## 15.9 Failures of your own

`Result` is a library declaration, not a built-in. Any variant can say which case is its
success and that it must not be dropped:

<!-- file docs/book/code/fail/own.oro -->
```go
set Parsed = 'value(Int) ∪ 'empty ∪ 'bad(Int) success 'value relevant

fn digit(c ∈ 0..127) {
    case {
        c == 0 -> 'empty
        c >= 48 && c <= 57 -> 'value(c - 48)
        else -> 'bad(c)
    }
}

export fn twice(c ∈ 0..127) (Parsed) {
    try 'value(d) = digit(c) { 'value(2 * d) }
}
```

<!-- gen go docs/book/code/fail/own.oro -->
```go
func FailTwice(v0 int) (int, int) {
    v10 := (v0 == 0)
    if v10 {
        return 1, 0
    }
    v18 := (v0 >= 48)
    v19 := (v18 && (v0 <= 57))
    if v19 {
        v29 := (v0 - 48)
        v30 := (2 * v29)
        return 0, v30
    }
    return 2, v0
}
```

`Parsed` has one success and *two* ways to fail, one of which carries the offending
character. `try 'value(d) = …` continues on `'value` and passes `'empty` and `'bad(c)` out
as they are. A domain usually has its own failures (an order can be `'out_of_stock`,
`'card_declined`, `'address_unknown`), and a variant that names them is better documentation
than any comment.

One rule: **every `try` in a chain passes failures of the same kind**, since they all become
the answer of one function. When two steps fail in different ways, translate one into the
other. `result.err_map(f, r)` applies `f` to the reason inside an `'err` and leaves an `'ok`
alone:

<!-- file docs/book/code/fail/kinds.oro -->
```go
use result

set Why = 'odd(Int) ∪ 'too_big

fn half(n ∈ 0..1000000) { if n % 2 == 0 { result.'ok(n / 2) } else { result.'err(n) } }

fn check(n ∈ 0..1000000) { if n > 1000 { result.'err('too_big) } else { result.'ok(n) } }

export fn code(n ∈ 0..1000000) (Int) {
    r = try result.'ok(m) = check(n) { result.err_map(fn(e) { 'odd(e) }, half(m)) }
    case r {
        result.'ok(v) -> v
        result.'err(why) -> case why { 'odd(k) -> -1; 'too_big -> -2 }
    }
}
```

<!-- gen go docs/book/code/fail/kinds.oro -->
```go
func FailCode(v0 int) int {
    v10 := (v0 > 1000)
    if v10 {
        return -2
    }
    v15 := (v0 & 1)
    v17 := (v15 == 0)
    if v17 {
        v19 := (v0 >> 1)
        return v19
    }
    return -1
}
```

`check` fails with a `Why`. `half` fails with an integer. `err_map(fn(e) { 'odd(e) }, …)`
wraps `half`'s integer as a `Why`, so the two can share a chain. The final `case` answers for
every kind.

Then look at the Go. Two variants, a `try`, a function passed to `err_map`, a nested `case`:
all of it is gone. Two tests and three returns. The whole apparatus of this chapter is a way
of *writing down* which failures exist and where they go, and what runs is the tests the
problem needed.

---

## 15.10 What other languages do

| | a failure is | passing it on | can it be silently dropped? | giving up |
|---|---|---|---|---|
| Oroboros | a value: a variant with a `success` case | `try` | no (`relevant`); `ignore` to mean it | `expect … else "why"` |
| C | a special return value, or `errno` | by hand | yes, and usually is | `abort`, `assert` |
| Go | a second result, `err` | `if err != nil { return err }` | yes: `_ =`, or just not naming it | `panic` |
| Java | an exception; some must be declared | automatic, invisible | no, but `catch (Exception e) {}` | an unchecked exception |
| Python, JavaScript | an exception | automatic, invisible | by an empty `except` / `catch` | let it propagate |
| Rust | a value: `Result<T, E>` | the `?` operator | a warning (`#[must_use]`) | `.expect("why")`, `.unwrap()` |
| Haskell | a value: `Either e a` | `>>=`, `do` | yes | `error` |
| Zig | a value: an error union | `try` | no | `catch unreachable` |
| Erlang | a tagged tuple, or an exit | pattern match | by matching loosely | let it crash; a supervisor restarts |

Rust is the nearest relative again, for the fifth chapter. Its `Result`, its `?` (this
chapter's `try`) and its `.expect("…")` are the same three ideas, and its `#[must_use]` is
`relevant` as a warning where here it is a refusal. Rust also has `.unwrap()`, an `expect`
with no reason given, and experienced Rust programmers will tell you to avoid it.

Go made the choice this chapter makes, that errors are values and exits are visible, and
stopped one step short: nothing in Go makes you look at the `err`, and the pass-it-on line
is written out every time.

Java's *checked exceptions* (1995) were the first serious attempt to put failure in the
header. They are widely disliked, mostly because the header became a list every caller had
to copy or swallow. The value-and-`try` design gets the same honesty with less ceremony: the
failure is part of the result's set, and a caller that does not care about the details
passes it on with one word.

---

## What to remember

- **A failure is a value.** A function that can fail returns a variant. The library's is
  `result.Result(T, E)`: `result.'ok(v)` or `result.'err(why)`.
- **The only way to the answer is a `case` with an arm for the failure.**
- **`try 'ok(x) = e { … }`** continues on success with `x`, and gives any other case as the
  answer, unchanged. It is a `case` with the pass-it-on arms written for you.
- **Each place a function can leave early says `try`.** Nothing leaves invisibly.
- **`try a, b, c { … }`** is several steps in order, each seeing the names bound before
  it. The first that is not a success is the answer.
- **A `relevant` value must be used on every path.** `ignore(e)` discards one on purpose.
- **`expect 'ok(x) = e else "why"`** continues on success and stops the program otherwise,
  printing the reason. Use it when failing means the program is wrong, and write a reason
  worth reading.
- **`try` may hold `again`**: a loop that stops at the first failure.
- **`os.ReadFile` and `os.WriteFile` give results**, and the reasons are a variant: not
  found, exists, permission, other.
- **Any variant can declare `success 'c` and `relevant`.** Name your domain's failures.
  `result.err_map` translates one kind of failure into another.
- **It costs nothing inside a program**: the variants dissolve and the early exits are plain
  returns.

Next: [chapter 16](16-claims.md), Claims. How the compiler proves what a header says.
