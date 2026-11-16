# 17. A whole program

Sixteen chapters, each about one idea, each with examples of five or ten lines. A reader
could finish them knowing every construct and still not know what a *program* looks like:
which constructs carry the weight, how the pieces meet, how much of the text is about the
problem and how much is about proof.

So this chapter introduces nothing. It reads one complete program from its first line to
its last, and at each line asks two questions: **what is this for, and which chapter taught
it?**

The program is a ledger. It reads a file of purchases, a price and a quantity on each line,
and prints the subtotal, the sales tax and the total:

```
12.50 2
0.99 3
3 10
```

It is 74 lines. It reads a file that may not exist, treats text that may be malformed, does
exact arithmetic on money, and runs unchanged on four targets. It is adapted from
`examples/num/ledger.oro` in this repository, and it is in [code/whole/](code/whole/). `go
run docs/book/check.go docs/book/17-whole.md` re-runs every block. Checked 2026-11-16.

---

## 17.1 Running it

Before reading it, watch it work. The program is a module, `ledger`, with one public
function, and a three-line entry program names the file:

<!-- file docs/book/code/whole/purchases.oro -->
```go
use ledger

export fn main() {
    ledger.run("docs/book/code/whole/purchases.txt")
}
```

<!-- run go docs/book/code/whole/purchases.oro -->
```
subtotal 57.97
tax      4.78
total    62.75
```

Check it by hand. 12.50 × 2 is 25.00. 0.99 × 3 is 2.97. 3 × 10 is 30.00. Together 57.97.
The tax at 8.25% is 4.782525, which is 4.78 to the cent. And 57.97 + 4.78 is 62.75.

<!-- run js docs/book/code/whole/purchases.oro -->
```
subtotal 57.97
tax      4.78
total    62.75
```

<!-- run windows docs/book/code/whole/purchases.oro -->
```
subtotal 57.97
tax      4.78
total    62.75
```

<!-- portable docs/book/code/whole/purchases.oro -->
```
portable to go, js, java, windows
```

The same text, the same three lines, on a Go binary, in a JavaScript engine and in a native
Windows executable with no runtime under it. Now the source.

---

## 17.2 The header

<!-- part docs/book/code/whole/ledger.oro -->
```go
// ledger: the subtotal, tax and total of a file of lines `PRICE QUANTITY`.
//
// A price has at most two places after its point (`12.50`, `3`, `0.99`) and a
// quantity is a whole number. The tax is 8.25% of the subtotal, rounded to the
// cent, a half going away from zero. Every amount is exact.
use io
use os
use num
use result

cap_in = 2^16
```

**The comment** says what the program does, what input it accepts, and the one decision in
it that is a matter of law and not of arithmetic: which way a half-cent of tax rounds.
Chapter 8 said that choice belongs in the program text. It belongs in the first comment too.

**Four `use` lines** (chapter 3). Not one of them is the language. `io` prints, `os` reads
files, `num` writes a number as text, `result` is the variant a fallible function returns.
Chapter 16 showed what each is underneath: a folder with a file per target. This program
never says which target, and that is why it runs on four.

**`cap_in = 2^16`** is a constant (chapter 2): a name for a number, computed while
compiling. It is the largest file the program accepts, 65536 bytes, and 17.6 shows what
depends on it.

---

## 17.3 Two small functions

<!-- part docs/book/code/whole/ledger.oro -->
```go
fn digit?(b) { b >= 48 && b <= 57 }

// The number written by `digits` with `places` of them after the point:
// 1250 with two places is 12.50.
fn price(digits ∈ 0..10^7, places ∈ 0..2) {
    case {
        places == 2 -> digits * 0.01
        places == 1 -> digits * 0.1
        else -> digits * 1.0
    }
}
```

**`digit?`** asks whether a byte is one of the characters `0` to `9`. Chapter 9: those are
numbers 48 to 57 in ASCII, in order. The file is read as bytes (chapter 12), so the question
is a comparison of numbers. A `?` may end a name, and here it marks a function that answers
yes or no.

**`price`** is where text becomes money. The reading loop below collects a price's digits as
one integer, ignoring the point, and counts how many came after the point. `12.50` arrives
as `digits` = 1250 and `places` = 2. `price` moves the point back: 1250 × 0.01.

Three things are packed into those six lines.

- **`0.01` is one hundredth, exactly** (chapter 8). `digits * 0.01` is an exact decimal. No
  double is anywhere in this program.
- **The header's sets are claims** (chapters 6 and 15). `digits ∈ 0..10^7` and `places ∈
  0..2` are obligations on whoever calls `price`. The loop must prove them, and 17.5 shows
  how.
- **The three arms give three different scales**: hundredths, tenths, units. The compiler
  holds a decimal as an integer numerator at a scale known while compiling (chapter 8), so
  it has to bring the arms to one scale, hundredths, where they meet. `digits * 1.0` has a
  point for that reason: it makes the third arm a number of the same kind as the other two.

Neither function exists when the program runs. Both are unfolded where they are called
(chapter 2).

---

## 17.4 The report

<!-- part docs/book/code/whole/ledger.oro -->
```go
fn report(sum) {
    tax = round(D(2, 0..10^10), sum * 0.0825, 'half_away)
    io.print_line("subtotal " ++ num.decimal(sum, 2))
    io.print_line("tax      " ++ num.decimal(tax, 2))
    io.print_line("total    " ++ num.decimal(sum + tax, 2))
}
```

The whole of the program's arithmetic about tax is the first line, and it is chapter 8 in
one expression.

- **`sum * 0.0825`** is the exact tax: for 57.97, the number 4.782525. Six places, more than
  any currency has.
- **`round(D(2, 0..10^10), …, 'half_away)`** is the one place in the program where precision
  is given up, on purpose, and it says three things. *Onto what*: `D(2, …)`, amounts with
  two places. *Within what*: 0 to 10¹⁰. *Which way*: `'half_away`, a half-cent goes up. That
  mode is the tax authority's choice. The default, ties to even, would be the statistician's.
- The rounding **must land in its set**, an obligation. It is proven from a bound on `sum`
  that the loop maintains, and 17.6 removes that bound to show the refusal.

Then three lines of output. `num.decimal(sum, 2)` is the number as text with two places,
`++` joins text (chapter 9), and `io.print_line` is the effect (chapter 5). The three prints
happen in the order written because effects are never reordered.

`sum + tax` is exact, since both are decimals, so the three numbers printed always add up.
In a program that used doubles they would add up most of the time.

---

## 17.5 The loop

This is the heart: thirty-six lines that turn bytes into a sum.

<!-- part docs/book/code/whole/ledger.oro -->
```go
fn ledger(src) {
    var digits := 0
    var places := 0
    var point := false
    var qty := 0
    var field := 0
    var sum := 0.0
    var bad := false
```

**Seven cells** (chapter 13). Together they are everything the program remembers while it
reads:

| cell | what it holds |
|---|---|
| `digits` | the digits of the price seen so far on this line, as one integer |
| `places` | how many of those came after the point |
| `point` | whether the point has been seen |
| `qty` | the quantity seen so far |
| `field` | 0 while reading the price, 1 while reading the quantity |
| `sum` | the subtotal of all finished lines: a decimal |
| `bad` | whether any line was malformed |

Chapter 13 said a cell is a notation for a loop variable, and that cells are the better
spelling when a value changes on some trips and not others. This loop is that case exactly.
On a typical trip one or two of the seven change. Written with `again(…)` carrying all
seven, every arm would list seven values, six of them unchanged.

<!-- part docs/book/code/whole/ledger.oro -->
```go
    loop i = 0 {
        i > len(src) -> if bad { io.print_line("a line is not PRICE QUANTITY") } else { report(sum) }
        else -> {
            b = if i < len(src) { src[i] } else { 10 }
```

**One loop variable, `i`**, the position in the file (chapter 11), and two arms.

The first arm is the exit. Note `i > len(src)`, not `>=`: the loop takes one trip *past* the
end. The line below explains why. At the position one past the last byte, `b` is 10, the
newline character. Many files do not end with a newline, and a last line with none would
never be finished. Pretending there is one more newline closes it, and costs nothing when
the file already had one.

That line is also the proof that `src[i]` is a position (chapter 12). The guard `i <
len(src)` is right there, and `i` started at 0 and only goes up. Nothing else in the program
reads `src`.

At the exit there are two outcomes: a complaint, or the report.

<!-- part docs/book/code/whole/ledger.oro -->
```go
            case {
                b == 10 -> {
                    if field == 1 && sum < 10^10 { sum := sum + price(digits, places) * qty }
                    if field == 0 && digits > 0 { bad := true }
                    digits := 0
                    places := 0
                    point := false
                    qty := 0
                    field := 0
                }
```

Inside, a `case` over what the byte is (chapter 4), tried in order. First, **a newline**:
the line is over.

- If the line got as far as its quantity (`field == 1`), add it in: price times quantity.
  This is the only place `sum` grows. `price(digits, places) * qty` is a decimal times an
  integer, exact.
- If a price was begun and no quantity followed, the line is malformed.
- Then five cells go back to their starting values, ready for the next line.

The `&& sum < 10^10` is a guard of the kind chapter 11 described: a bound stated where the
compiler can lean on it. 17.6 returns to it.

<!-- part docs/book/code/whole/ledger.oro -->
```go
                b == 32 && field == 0 -> { field := 1 }
                b == 46 && field == 0 && !point -> { point := true }
                digit?(b) && field == 0 && digits < 10^6 && places < 2 -> {
                    digits := digits * 10 + (b - 48)
                    if point { places := places + 1 }
                }
                digit?(b) && field == 1 && qty < 100 -> { qty := qty * 10 + (b - 48) }
                b == 13 -> ()
                else -> { bad := true }
            }
            again(i + 1)
        }
    }
}
```

The other bytes, one arm each:

- **A space** (32), while reading the price: the price is over, the quantity begins.
- **A point** (46), in the price, if there has not been one already.
- **A digit in the price.** `digits * 10 + (b - 48)` is positional notation, chapter 9 run
  forwards: shift what you have one place left and add the new digit. `b - 48` is the
  digit's value. If the point has gone by, count a place.
- **A digit in the quantity**: the same step.
- **A carriage return** (13), which Windows puts before each newline: ignored.
- **Anything else**: the line is bad. A letter, a comma, a second point, a third digit after
  the point, a space in the quantity: they all land here, because every arm above is
  guarded by exactly the situation in which its byte is welcome.

Then `again(i + 1)`: on to the next byte.

What you are looking at is a **state machine** (chapter 10): the cells are the state, each
byte is an input, and the arms are the table of what each input does in each state. Every
program that reads text is one, underneath. This one is written out plainly.

### The guards are the proofs

Look again at two of those arms:

```
digit?(b) && field == 0 && digits < 10^6 && places < 2 -> { … }
digit?(b) && field == 1 && qty < 100 -> { … }
```

`digits < 10^6` and `places < 2` and `qty < 100` are not there for the parsing. They are
there because of what they make true. With `digits` below 10⁶ before the step, `digits * 10
+ 9` is below 10⁷ after it, and `price` demanded `digits ∈ 0..10^7`. With `places` below 2
before, it is at most 2 after, and `price` demanded `places ∈ 0..2`. With `qty` below 100
before, it is below 1000 after, which bounds the product.

They also do a second job. A price with too many digits, or a quantity over 999, fails its
guard, falls through to `else`, and the line is reported as bad. So **the limit the proof
needs and the limit the user is told about are the same test.** That is chapter 11's lesson
in its best form. A program that reads numbers from a file has to have *some* largest number
it accepts. Here the compiler made the author pick one, write it down, and say what happens
past it.

---

## 17.6 The world

<!-- part docs/book/code/whole/ledger.oro -->
```go
pub fn run(path) {
    case os.ReadFile(path) {
        result.'err(why) -> io.print_line("cannot read that file")
        result.'ok(src) -> if len(src) >= cap_in { io.print_line("file is too large") } else { ledger(src) }
    }
}
```

The last function, and the only `pub` one (chapter 3): the module's whole interface.

`os.ReadFile(path)` returns a result (chapter 14), and the `case` answers for both tags.
There is no way to reach `src` except through the `'ok` arm, so there is no way to forget
that the file might not be there.

Two files in the same folder show the two failures. One has a comma where a point should
be:

<!-- file docs/book/code/whole/typo.oro -->
```go
use ledger

export fn main() {
    ledger.run("docs/book/code/whole/typo.txt")
}
```

<!-- run go docs/book/code/whole/typo.oro -->
```
a line is not PRICE QUANTITY
```

and one does not exist:

<!-- file docs/book/code/whole/missing.oro -->
```go
use ledger

export fn main() {
    ledger.run("docs/book/code/whole/missing.txt")
}
```

<!-- run go docs/book/code/whole/missing.oro -->
```
cannot read that file
```

<!-- run windows docs/book/code/whole/missing.oro -->
```
cannot read that file
```

Neither is a crash. A program that meets bad input and says so, in a sentence, has done its
job.

---

## 17.7 Taking the guards out

The claim of this book is that those guards are not decoration. The way to check a claim
like that is to remove one and see what happens. Each program below is the ledger with one
change, and each is refused.

**Forget that the file may be missing.** Delete the `'err` arm:

<!-- gen go docs/book/code/whole/no-err.oro -->
```
gen: docs/book/code/whole/no-err.oro: run: case on result is not exhaustive: err is unmatched. Add it a clause, or an `else`
```

Chapter 10: a `case` answers for every tag.

**Read one byte past the end.** Replace `b = if i < len(src) { src[i] } else { 10 }` with
`b = src[i]`:

<!-- gen go docs/book/code/whole/no-end.oro -->
```
gen: whole-main: #nl15[i] is an indexing, and i < len(#nl15) does not follow
```

Chapter 12. The loop runs while `i <= len(src)`, and at `i == len(src)` there is no byte.
(`#nl15` is the compiler's own name for `src`, after the function was unfolded into `main`.
In a small example a message names your variable. In a whole program it sometimes names its
own, which is a wart.)

**Let a price have any number of places.** Remove `&& places < 2`:

<!-- gen go docs/book/code/whole/no-places.oro -->
```
gen: whole-main: price's parameter places, declared in 0..2, requires places <= 2 at a call, which does not follow
```

Chapter 6: a parameter's set is an obligation at the call. Without the guard, `places` can
count as high as the line is long, and `price` has no arm for 3.

**Let the subtotal grow without limit.** Remove `&& sum < 10^10`:

<!-- gen go docs/book/code/whole/no-sum.oro -->
```
gen: whole-main: whole-main: round(D(2, 0..10000000000), …)'s value inside its set: not proven (the numerator is [0, 5406720000000000], its set's ends 0..1000000000000) (ADR 0086, numbers.md §3)
```

Chapter 8: a rounding must land in its set. The message shows the compiler's arithmetic.
Without the guard it bounded the subtotal by multiplying out the worst case, every line of
the largest file at the largest price and quantity, and the tax on *that* is past the 10¹⁰
the tax's set allows. With the guard, the subtotal is below 10¹⁰ plus one line, and its tax
fits.

**Accept a file of any size.** Remove the test against `cap_in`:

<!-- portable docs/book/code/whole/no-size.oro -->
```
portable to go, java, windows — not js
```

This one is not refused everywhere. It is refused *somewhere*. With no limit on the file
there is no limit on how many lines add into `sum`'s numerator, and JavaScript's word is
smaller than the others'. Chapter 6: portability is computed. `cap_in` is the line in this
program that buys JavaScript.

Five changes, five refusals, and each message pointed at a real hole: a file not there, a
read past the end, a case with no arm, a number too large for where it is going, a host too
small for the data. In most languages each of those programs compiles, and each of those
bugs is found by a user.

One honest note about the messages. Remove `digits < 10^6` and the refusal comes from far
away, inside the library function that prints the result, which is the first place the
too-large number gets somewhere it does not fit. The compiler tells you where the proof
broke, and that is not always where the cause is. Chapter 15's advice applies: state what
you believe near where you believe it, and the refusal moves there.

---

## 17.8 What came out

<!-- gen go docs/book/code/whole/purchases.oro -->
```go
func WholeMain() string {
    v3, v4 := os.ReadFile("docs/book/code/whole/purchases.txt")
    v5 := (v4 == nil)
    if v5 {
        v6 := len(v3)
        v8 := (v6 >= 65536)
        if v8 {
            fmt.Println("file is too large")
            return "file is too large"
        }
        var v23 int = 0
        var v24 int = 0
        var v25 bool = false
        var v26 int = 0
        var v27 int = 0
        var v28 int = 0
        var v29 bool = false
        var v30 int = 0
        var v22 string
        for ; ; v30 = (v30 + 1) {
```

The first twenty lines of the Go, and every source construct can be found in them.

- `os.ReadFile(…)` is Go's own, returning its own `(bytes, error)`, and the `case` on the
  result is `v4 == nil`. The `result` variant is nowhere.
- `cap_in` is `65536`.
- The seven cells are seven Go variables. `sum`, the decimal, is `v28`, an `int`: the
  subtotal in hundredths. A few lines further on the tax is `v28 * 825`, and the rounding
  is an integer division by 10000 with a correction for the half.
- The loop is a `for`.

There is one function. `ledger`, `report`, `price`, `digit?` and `run` were all unfolded
into it, along with `num.decimal`. No function is called except Go's own, nothing is
allocated except the strings that are printed, and no check runs that the source did not
write. The note above that function reads *268 of 268 integer operations bounded; 7 of 7
loops proven terminating*.

It is also long: about 1500 lines of Go from 74 of source, because the chain of arms is
repeated where the compiler specialized it. That is the price of unfolding everything, it
is the kind of cost this project measures and has not finished reducing, and it should be
said.

---

## 17.9 What the program is made of

Count the 74 lines by what they are for.

| | lines | chapters |
|---|---|---|
| comments and blank lines | 13 | |
| imports and a constant | 5 | 2, 3 |
| the two helpers | 8 | 4, 6, 8, 9 |
| the report: arithmetic and printing | 6 | 5, 8, 9 |
| the state: seven cells | 7 | 13 |
| the loop and its arms | 29 | 4, 10, 11, 12 |
| the file and its failures | 6 | 3, 14, 16 |

And by a different cut: of the lines that do something, which are about the *problem* and
which are about *proof*?

Almost none are about proof alone. There is no annotation here that exists only to satisfy
the compiler: no invariant, no assertion, no cast. What there is instead are five small
tests (`digits < 10^6`, `places < 2`, `qty < 100`, `sum < 10^10`, `len(src) >= cap_in`), and
each is also a statement of what input the program accepts. The sets on `price`'s header
are two more. That is the whole cost of every guarantee in 17.7.

It is worth saying what those guarantees are, for this one program, as a list:

- It never reads a byte outside the file.
- It never computes a wrong sum by overflow, on any of four targets.
- Every amount it prints is exact, and the three always add up.
- A missing file and a malformed line each produce a message, never a crash.
- It stops.

None of those was tested. All of them were proven, each time the program was compiled, for
every file that could ever be given to it.

---

## 17.10 Where to go from here

This book has taught what is decided and built. The repository holds a good deal more.

**Programs to read.** `examples/` has larger ones in the same style: `io/wc.oro` counts
lines, words and characters; `io/freq.oro` counts words and sorts them; `json/` reads JSON
into a table of nodes; `hash/` computes the checksums of chapter 7 over a file; `big/`
computes factorials and Fibonacci numbers of thousands of digits. Each is a few screens,
and each leans on a chapter of this book harder than the chapter's own examples could.

**The specifications.** `docs/spec/` says, for each construct, what it means on every
target. Where this book and a specification disagree, the specification is right and this
book has a bug.

**The decisions.** `docs/decisions/` is more than a hundred short records, one per design
choice, each with a section titled *Why not*: the alternatives that were weighed and
rejected. If something in this language seemed strange to you (no recursion, no string
length, a guard where you wanted an assertion), the reason is written down there, and so is
the argument against it.

**What is not built.** Records, callbacks, concurrency, text you can take apart, loop
invariants you can state. Each is specified or researched and waiting for a program that
needs it. This book's own [README](README.md) lists, by number, every place where writing a
chapter found the compiler doing something other than what the specification says. It is a
long list. A language whose book is checked against its compiler has one.

The idea under all seventeen chapters is small enough to say in a breath. *Write what you
mean: an integer, a set, a table, a failure. Let the compiler turn it into what the host
already does well. And have it prove, before the program exists, that the two agree.*

---

## What to remember

- **A program is mostly a loop over its input, a little arithmetic, and the handling of what
  can go wrong.**
- **State lives in cells, and the arms of a `case` are a table of what each input does in
  each state.**
- **Bytes in, text out.** Input is examined as a table of bytes. Output is built as text.
- **The limits a program states are its proofs.** One test both bounds the arithmetic and
  rejects the input that would break it.
- **Take a guard out and read the refusal.** It is the fastest way to learn what a line was
  for.
- **Nothing that helped you write the program is left in what runs.**
