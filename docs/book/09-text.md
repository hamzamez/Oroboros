# 9. Text

Every chapter so far has printed text and none has said what text is. `"hello, world"` went to
`io.print_line` in chapter 0 and has been taken on trust since.

Numbers were easy to pin down: an integer is an integer. Text is harder, and most languages
get it subtly wrong in a way you can measure. Ask five of them how long the string `🙂` is:

| | Go | JavaScript | Java | Python 3 | Swift |
|---|---|---|---|---|---|
| length of `"🙂"` | 4 | 2 | 2 | 1 | 1 |
| length of `"café"` | 5 | 4 | 4 | 4 | 4 |
| length of `"🇫🇷"` | 8 | 4 | 4 | 2 | 1 |

One smiling face, and the answers are 4, 2 and 1. So the question of this chapter is: **what
is a piece of text, and what may a program do with it?**

The answer is two sentences.

> **A string is a finite sequence of characters, where a character is a number that Unicode
> assigned.** How a machine stores the sequence is not part of it, and no program can see it.
>
> **You can write a string, join two with `++`, and make a one-character string from its
> number with `string_of`.** That is the whole list, and it is enough to produce any text.

Along the way: where the numbers came from, why there is a hole in them, the three ways
machines store text and why they disagree about length, a structure called a monoid that you
have been using since chapter 4, and the one-byte decision in 1972 that security teams are
still paying for.

The examples are in [code/text/](code/text/). `go run docs/book/check.go docs/book/09-text.md`
re-runs every block. Checked 2026-11-16.

---

## 9.1 Writing text

<!-- file docs/book/code/text/hello.oro -->
```go
use io

export fn main() {
    io.print_line("hello, world")
    io.print_line("")
    io.print("no line break after this, ")
    io.print("so this continues it")
    io.print_line("")
    io.print_line("café 日本 🙂")
}
```

<!-- run go docs/book/code/text/hello.oro -->
```
hello, world

no line break after this, so this continues it
café 日本 🙂
```

A **string literal** is text between double quotes. `""` is the empty string: no characters at
all, which is a perfectly good string, the way 0 is a perfectly good number. `io.print_line`
prints a string and ends the line; `io.print` prints it and does not.

The last line is not a trick. A literal may hold any character of any writing system, and it
prints the same on every target:

<!-- run js docs/book/code/text/hello.oro -->
```
hello, world

no line break after this, so this continues it
café 日本 🙂
```

<!-- run windows docs/book/code/text/hello.oro -->
```
hello, world

no line break after this, so this continues it
café 日本 🙂
```

The set of all strings is called `String`, and it goes in a header where `Int` or `Bool`
would:

<!-- file docs/book/code/text/greet.oro -->
```go
export fn greet(name ∈ String) (String) { "hello, " ++ name ++ "!" }

export fn answer(c ∈ Bool) (String) { if c { "yes" } else { "no" } }
```

<!-- gen go docs/book/code/text/greet.oro -->
```go
func TextAnswer(v0 bool) string {
    if v0 {
        return "yes"
    }
    return "no"
}

func TextGreet(v0 string) string {
    v2 := ("hello, " + v0)
    v4 := (v2 + "!")
    return v4
}
```

A string is a value like any other. An `if` can choose between two, a function can take one
and return one. `++` joins two strings and is the subject of 9.4. On Go a `String` is Go's own
`string` and `++` is Go's own `+`: nothing was wrapped.

---

## 9.2 A character is a number

A machine stores numbers. To store text, someone has to decide which number means which
character, and everyone who exchanges text has to agree.

The agreement that lasted is **ASCII**, from 1963: 128 numbers, 0 to 127, for the English
letters, the digits, punctuation and some control signals for teleprinters. 65 is `A`, 97 is
`a`, 48 is `0`, 32 is a space, 10 is "new line".

128 numbers do not hold `é`, let alone Greek, Arabic or Chinese. For thirty years every
country and every vendor extended ASCII in its own way, into the numbers 128 to 255, and the
same byte meant `é` on one machine and `Θ` on another. Text that crossed a border arrived as
garbage. The fix, begun in 1991, is **Unicode**: one table for all of humanity's writing, one
number per character, the same on every machine. Its first 128 entries are ASCII, unchanged.

Unicode writes a character's number in hexadecimal after `U+`: `A` is U+0041, `é` is U+00E9,
`日` is U+65E5, `🙂` is U+1F642. The numbers run from 0 to 0x10FFFF, a little over a million.

**`string_of(n)`** is the string made of the one character whose number is `n`:

<!-- file docs/book/code/text/scalars.oro -->
```go
use io

export fn main() {
    io.print_line(string_of(65))
    io.print_line(string_of(0x41))
    io.print_line(string_of(0xE9))
    io.print_line(string_of(0x3B1))
    io.print_line(string_of(0x4E2D))
    io.print_line(string_of(0x1F642))
}
```

<!-- run go docs/book/code/text/scalars.oro -->
```
A
A
é
α
中
🙂
```

65 and 0x41 are the same number (chapter 7), so they are the same letter.

ASCII was laid out by people who had to build the hardware, and three of its patterns are
still useful:

- **The digits are in order from 48**: the character for the digit `d` is number 48 + d.
- **The letters are in order**: `a` to `z` are 97 to 122, `A` to `Z` are 65 to 90.
- **A lower-case letter is its capital plus 32**, and 32 is one bit (chapter 7's bit 5). So
  flipping that bit changes a letter's case.

<!-- file docs/book/code/text/ascii.oro -->
```go
use io

fn digit(d ∈ 0..9) (String) { string_of(48 + d) }

fn letter(i ∈ 0..25) (String) { string_of(97 + i) }

fn capital(i ∈ 0..25) (String) { string_of(65 + i) }

fn other_case(c ∈ 65..122) (String) { string_of(c ⊕ 32) }

export fn main() {
    io.print_line(digit(0) ++ digit(7) ++ digit(9))
    io.print_line(letter(0) ++ letter(1) ++ letter(25))
    io.print_line(capital(0) ++ capital(1) ++ capital(25))
    io.print_line(other_case(97) ++ other_case(65) ++ other_case(122))
}
```

<!-- run go docs/book/code/text/ascii.oro -->
```
079
abz
ABZ
AaZ
```

`other_case(97)` turned `a` into `A`, and `other_case(65)` turned `A` into `a`: one exclusive
or. (It is only a case change for the 52 letters. The six numbers between `Z` and `a` are
punctuation, and the trick does not apply to `é` or `α` at all. Real case conversion is a
table lookup, and belongs to the host.)

---

## 9.3 Writing the characters you cannot type

Most characters can stand for themselves between the quotes. A few cannot: the quote itself
would end the literal, and a line break inside a literal is far more often a forgotten closing
quote than an intention. For those there are **escapes**, each starting with a backslash:

| escape | the character |
|---|---|
| `\"` | a double quote |
| `\\` | a backslash |
| `\t` | a tab (number 9) |
| `\n` | a new line (number 10) |
| `\r` | a carriage return (number 13) |
| `\u{H…}` | the character whose number is the hexadecimal `H…`, one to six digits |

<!-- file docs/book/code/text/escapes.oro -->
```go
use io

export fn main() {
    io.print_line("a \"quoted\" word")
    io.print_line("a back\\slash")
    io.print_line("name\tage")
    io.print_line("line one\nline two")
    io.print_line("caf\u{E9}, \u{65E5}\u{672C}, \u{1F642}")
    io.print_line("\u{41}\u{0041}\u{000041}")
}
```

<!-- run go docs/book/code/text/escapes.oro -->
```
a "quoted" word
a back\slash
name    age
line one
line two
café, 日本, 🙂
AAA
```

`\u{…}` writes any character at all by its Unicode number, so every string can be written
with a plain keyboard. Leading zeros do not matter.

There are six escapes and no others. C has about a dozen, and they do not travel: `\a` rings a
bell in Go, is a compile error in Java, and is the letter `a` in JavaScript. Here an unknown
escape is an error where it is written:

<!-- file docs/book/code/text/no-bell.oro.txt -->
```go
use io

export fn main() {
    io.print_line("ding \a")
}
```

<!-- build go docs/book/code/text/no-bell.oro.txt -->
```
build: docs/book/code/text/no-bell.oro.txt: line 4: \a is not an escape. The escapes are \" \\ \t \n \r and \u{H…} for any scalar value (docs/spec/string-literals.md §3)
```

And a literal ends on the line where it began:

<!-- file docs/book/code/text/no-raw.oro.txt -->
```go
use io

export fn main() {
    io.print_line("line one
line two")
}
```

<!-- build go docs/book/code/text/no-raw.oro.txt -->
```
build: docs/book/code/text/no-raw.oro.txt: line 4: a string literal may not contain a raw tab, newline or carriage return; write \t, \n or \r
```

The error is on line 4, where the mistake is. In a language that lets a literal run across
lines, a missing quote is reported hundreds of lines later, at the next quote it happens to
find.

---

## 9.4 Joining

`a ++ b` is the string `a` followed by the string `b`.

<!-- file docs/book/code/text/join.oro -->
```go
use io

export fn main() {
    io.print_line("rail" ++ "road")
    io.print_line("road" ++ "rail")
    io.print_line(("a" ++ "b") ++ "c")
    io.print_line("a" ++ ("b" ++ "c"))
    io.print_line("" ++ "same" ++ "")
}
```

<!-- run go docs/book/code/text/join.oro -->
```
railroad
roadrail
abc
abc
same
```

Three facts are in those five lines.

- **Order matters**: `"rail" ++ "road"` is not `"road" ++ "rail"`.
- **Grouping does not**: `(a ++ b) ++ c` is `a ++ (b ++ c)`, so nobody writes the parentheses.
- **`""` changes nothing**, on either side.

An operation whose grouping does not matter, with a value that changes nothing, is called a
**monoid**. You have met several without the name:

| the things | the operation | the value that changes nothing | does order matter? |
|---|---|---|---|
| integers | `+` | 0 | no |
| integers | `*` | 1 | no |
| booleans | `&&` | `true` | no |
| booleans | `\|\|` | `false` | no |
| bit patterns | `\|` | 0 | no |
| strings | `++` | `""` | **yes** |

The name is worth having because of what it buys. If grouping does not matter, a long join can
be done in any grouping: left to right, right to left, or in halves on two processors at once,
and the answer is the same. Every fast algorithm that splits work in two leans on exactly this
law, and chapter 12 will use it.

Strings are the *plainest* monoid there is: joining obeys those laws and **no others**. With
numbers, 2 + 3 and 4 + 1 are the same number made two ways. With strings, two different lists
of characters never join to the same string. Mathematicians call this the *free* monoid, and
"free" means free of extra coincidences. That is the precise sense in which a string is "just
a sequence".

`++` is its own symbol, not `+`, because it is a different operation with different laws:

<!-- file docs/book/code/text/no-plus.oro -->
```go
use io

export fn main() {
    io.print_line("rail" + "road")
}
```

<!-- build go docs/book/code/text/no-plus.oro -->
```
build: main: in argument 1 of io.print_line: in argument 1 of `+`: a string literal is String, but Int is required here
```

In JavaScript `"1" + 2` is `"12"` and `"3" - 2` is `1`, and a great many bugs live in that
gap. Here `+` is for numbers, where order does not matter, and `++` is for strings, where it
does. Nothing is converted for you either:

<!-- file docs/book/code/text/no-mix.oro -->
```go
use io

export fn main() {
    io.print_line("n = " ++ 5)
}
```

<!-- build go docs/book/code/text/no-mix.oro -->
```
build: main: in argument 1 of io.print_line: in argument 2 of concat: an integer literal is Int, but String is required here
```

5 is a number, and a number is not text. (`concat` is the compiler's own name for `++`.)
Turning one into the other is a computation, and 9.6 does it.

---

## 9.5 Building text with functions

Since strings are values and `++` is an operation, text is built the way numbers are: with
small functions.

<!-- file docs/book/code/text/build.oro -->
```go
use io

fn bracket(s ∈ String) (String) { "[" ++ s ++ "]" }

fn twice(s ∈ String) (String) { s ++ s }

fn row(a ∈ String, b ∈ String) (String) { a ++ "\t" ++ b ++ "\n" }

export fn main() {
    io.print_line(bracket("x"))
    io.print_line(bracket(""))
    io.print_line(bracket(twice(twice("ab"))))
    io.print(row("name", "age") ++ row("Ada", "36") ++ row("Alan", "41"))
}
```

<!-- run go docs/book/code/text/build.oro -->
```
[x]
[]
[abababab]
name    age
Ada    36
Alan    41
```

`row` ends its text with `\n`, so three rows joined are three lines, and one `io.print` (the
one that adds no line break of its own) writes the table.

A string never changes. `twice(s)` does not double `s`; it makes a new string and `s` is what
it was. There is no operation in the language that alters a string, so two parts of a program
that hold the same string can never surprise each other. (C's strings can be overwritten in
place, and Java made its strings unchangeable in 1995 for exactly this reason.)

---

## 9.6 A number, as text

`io.print_int(42)` printed `42`. Somebody turned the number forty-two into the two characters
`4` and `2`. Here is how, for a small number.

<!-- file docs/book/code/text/digits.oro -->
```go
use io

fn digit(d ∈ 0..9) (String) { string_of(48 + d) }

fn two(n ∈ 0..99) (String) { digit(n / 10) ++ digit(n % 10) }

fn three(n ∈ 0..999) (String) { digit(n / 100) ++ two(n % 100) }

fn clock(h ∈ 0..23, m ∈ 0..59) (String) { two(h) ++ ":" ++ two(m) }

fn date(y ∈ 1000..9999, m ∈ 1..12, d ∈ 1..31) (String) {
    two(y / 100) ++ two(y % 100) ++ "-" ++ two(m) ++ "-" ++ two(d)
}

export fn main() {
    io.print_line(two(7))
    io.print_line(three(42))
    io.print_line(clock(7, 5))
    io.print_line(clock(23, 59))
    io.print_line(date(2026, 11, 16))
}
```

<!-- run go docs/book/code/text/digits.oro -->
```
07
042
07:05
23:59
2026-11-16
```

`two` is the whole idea. Chapter 6's division gives the tens (`n / 10`) and the remainder
gives the ones (`n % 10`); `digit` turns each into its character; `++` puts them side by side.
Positional notation, run backwards.

Every claim in those headers was proven. `two(h)` needs `h` in 0..99, and 0..23 is inside it.
`digit(n / 10)` needs a number in 0..9, and with `n` in 0..99 the quotient is. Try `two(100)`
and the program is refused: a three-digit number has no two-digit numeral.

A number of any size needs repetition: divide by ten until nothing is left. That is chapter
11's `loop`, and it is what `num.decimal` from chapter 8 does. With it, formatting money is one
line:

<!-- file docs/book/code/text/price.oro -->
```go
use io
use num

fn price(cents ∈ 0..10^9) (String) { "$" ++ num.decimal(cents * 0.01, 2) }

export fn main() {
    io.print_line(price(1250))
    io.print_line(price(5))
    io.print_line("total: " ++ price(1250 + 5))
}
```

<!-- run go docs/book/code/text/price.oro -->
```
$12.50
$0.05
total: $12.55
```

---

## 9.7 Not every number is a character

`string_of` has a precondition: its argument must be a character's number.

<!-- file docs/book/code/text/no-any.oro -->
```go
export fn any(n ∈ Int) (String) { string_of(n) }
```

<!-- gen go docs/book/code/text/no-any.oro -->
```
gen: text-any: string_of's parameter c, declared in 0..1114111, requires 0 <= n && n <= 1114111 at a call, which does not follow
```

There is no character number −5 and none at ten million; 1114111 is 0x10FFFF. With the
argument proven in range, the call is the host's own conversion and nothing checks anything at
run time:

<!-- file docs/book/code/text/letter.oro -->
```go
export fn letter(i ∈ 0..25) (String) { string_of(97 + i) }
```

<!-- gen go docs/book/code/text/letter.oro -->
```go
func TextLetter(v0 int) string {
    v10 := (97 + v0)
    v11 := (string(rune(v10)))
    return v11
}
```

<!-- gen js docs/book/code/text/letter.oro -->
```js
export function textLetter(v0) {
    return (String.fromCodePoint((97 + v0)));
}
```

Now the surprise. A range that looks fine is refused:

<!-- file docs/book/code/text/no-hole.oro -->
```go
export fn wide(n ∈ 0..70000) (String) { string_of(n) }
```

<!-- gen go docs/book/code/text/no-hole.oro -->
```
gen: text-wide: `string_of` requires n <= 55295 || n >= 57344 at a call, which does not follow
```

**There is a hole in the characters.** The 2048 numbers from 55296 to 57343 (0xD800 to 0xDFFF)
are not characters and never will be. Unicode's name for a number that *is* allowed is a
**scalar value**: 0 to 0x10FFFF, minus the hole. The compiler's messages use that name.

Step around the hole and the program compiles:

<!-- file docs/book/code/text/hole.oro -->
```go
export fn wide(n ∈ 0..70000) (String) {
    case {
        n < 0xD800 -> string_of(n)
        n > 0xDFFF -> string_of(n)
        else -> "?"
    }
}
```

<!-- gen go docs/book/code/text/hole.oro -->
```go
func TextWide(v0 int) string {
    v10 := (v0 < 55296)
    if v10 {
        v12 := (string(rune(v0)))
        return v12
    }
    v15 := (v0 > 57343)
    if v15 {
        v17 := (string(rune(v0)))
        return v17
    }
    return "?"
}
```

Each arm's guard is the fact that arm needs (chapter 6: a condition is a fact).

The hole cannot be written in a literal either:

<!-- file docs/book/code/text/no-surrogate.oro.txt -->
```go
use io

export fn main() {
    io.print_line("\u{D83D}\u{DE42}")
}
```

<!-- build go docs/book/code/text/no-surrogate.oro.txt -->
```
build: docs/book/code/text/no-surrogate.oro.txt: line 4: \u{D83D} is a SURROGATE, which is not a scalar value — it exists only inside the UTF-16 encoding form, and a sequence containing one has no representation in UTF-8 at all (docs/spec/string-literals.md §1)
```

Why would a table of characters have 2048 numbers reserved for nothing? The message gives it
away: "the UTF-16 encoding form". The hole is a scar left by a storage format, and the next
section is about storage.

---

## 9.8 How machines store it, and why "length" has three answers

A string is a sequence of numbers up to about a million. A machine's memory is a sequence of
bytes, each 0 to 255. There are three common ways to fit one into the other, and they are
called **encodings**.

**UTF-32: four bytes per character.** Simple, and four times the size of ASCII text. Almost
nobody stores text this way.

**UTF-16: two bytes per character, usually.** When Unicode began it promised to need no more
than 65536 characters, so two bytes each would always do. Java (1995), JavaScript (1995) and
Windows all built their strings on that promise. Then Unicode outgrew it. The repair was to
take 2048 numbers out of the table, the hole, and use a *pair* of them to stand for each
character past 65535. Those are the **surrogates**. `🙂`, U+1F642, is the pair D83D DE42.

**UTF-8: one to four bytes per character.** Designed by Ken Thompson and Rob Pike in 1992, on
a placemat in a New Jersey diner. ASCII characters are one byte, themselves, so every ASCII
file was already valid UTF-8. Everything else takes two, three or four bytes, each with its
high bit set:

```
 character number         bytes (x are the number's bits)
 0     to 0x7F            0xxxxxxx
 0x80  to 0x7FF           110xxxxx 10xxxxxx
 0x800 to 0xFFFF          1110xxxx 10xxxxxx 10xxxxxx
 0x10000 to 0x10FFFF      11110xxx 10xxxxxx 10xxxxxx 10xxxxxx
```

A byte beginning `10` is always the middle of a character and never the start, so a reader
dropped anywhere into a stream can find the next character boundary. Go, Rust and nearly the
whole internet use UTF-8.

You can see the three hosts disagree, by compiling one literal:

<!-- file docs/book/code/text/smile.oro -->
```go
export fn smile() (String) { "caf\u{E9} \u{1F642}" }
```

<!-- gen go docs/book/code/text/smile.oro -->
```go
func TextSmile() string {
    return "caf\u00E9 \U0001F642"
}
```

<!-- gen js docs/book/code/text/smile.oro -->
```js
export function textSmile() {
    return "caf\u00E9 \uD83D\uDE42";
}
```

<!-- gen windows docs/book/code/text/smile.oro -->
```llvm
@.str.0 = private unnamed_addr constant { i64, [11 x i8] } { i64 10, [11 x i8] c"caf\C3\A9 \F0\9F\99\82\00" }, align 8
```

One string, six characters, three spellings.

- **Go** writes the smile as one escape, `\U0001F642`, and stores it in UTF-8.
- **JavaScript** has no way to write a character past 65535, so the compiler wrote the
  surrogate pair, `\uD83D\uDE42`. In JavaScript's memory that smile is two units.
- **Windows** gets the bytes themselves: `C3 A9` is `é` in UTF-8, `F0 9F 99 82` is the smile,
  and the `i64 10` in front says ten bytes.

Now the table that opened this chapter explains itself. Ask each host for "the length":

| the string | Go's `len` | JavaScript's `.length` | characters |
|---|---|---|---|
| `abc` | 3 | 3 | 3 |
| `café` | 5 | 4 | 4 |
| `日本` | 6 | 2 | 2 |
| `🙂` | 4 | 2 | 1 |

Go counts **bytes of UTF-8**. JavaScript and Java count **units of UTF-16**. Neither counts
characters, and for the smile all three numbers differ. None of the hosts is wrong. They are
answering three different questions, each about its own storage, and all three call the answer
"length".

<!-- portable docs/book/code/text/smile.oro -->
```
portable to go, js, java, windows
```

The program is portable anyway, because it never asked.

---

## 9.9 So there is no length, no index and no `==`

This language's rule (chapter 6 said it for integers) is that a program means one thing on
every target. "The length of a string" does not, so it is not in the language:

<!-- file docs/book/code/text/no-len.oro -->
```go
export fn count(s ∈ String) (Int) { len(s) }
```

<!-- gen go docs/book/code/text/no-len.oro -->
```
gen: count: len(…) is a table's, and a String is text, the free monoid over Unicode scalars, not a table: its length and its indexing are not in the language, because Go counts bytes and JavaScript and the JVM UTF-16 units (state.md §3, strings.md). Read its bytes with a declaration that says which encoding, or keep the text whole
```

Nor is "the character at position i", which has the same three answers:

<!-- file docs/book/code/text/no-index.oro -->
```go
export fn first(s ∈ String) (Int) { s[0] }
```

<!-- gen go docs/book/code/text/no-index.oro -->
```
gen: first: indexing, s[…], is a table's, and a String is text, the free monoid over Unicode scalars, not a table: its length and its indexing are not in the language, because Go counts bytes and JavaScript and the JVM UTF-16 units (state.md §3, strings.md). Read its bytes with a declaration that says which encoding, or keep the text whole
```

Nor, today, is equality:

<!-- file docs/book/code/text/no-equal.oro -->
```go
export fn same(a ∈ String, b ∈ String) (Bool) { a == b }
```

<!-- gen go docs/book/code/text/no-equal.oro -->
```
gen: same: in argument 1 of `==`: a is String, but Int is required here.
```

Be clear about how much that is. A program in this language can *produce* any text. It cannot
yet *take text apart*: it cannot count, search, compare or split a `String`. The message's
last sentence says where that work goes: "read its bytes". Text that a program must examine
is handled as a table of bytes (chapter 12), where length, position and equality are plain
integer questions with one answer each, and it becomes a `String` at the end, by 9.10's
operations. The repository's JSON reader and its word counter are written this way.

Why not simply count characters, the one count every host could agree on? It could be done,
and may be. But notice that even "character" is not what a person means by it:

<!-- file docs/book/code/text/looks.oro -->
```go
use io

export fn main() {
    io.print_line("\u{E9}")
    io.print_line("e\u{301}")
    io.print_line("\u{1F1EB}\u{1F1F7}")
    io.print_line("\u{1F468}\u{200D}\u{1F469}\u{200D}\u{1F467}")
}
```

<!-- run go docs/book/code/text/looks.oro -->
```
é
é
🇫🇷
👨‍👩‍👧
```

- The first two lines look identical. The first is one character, U+00E9. The second is two:
  a plain `e`, then U+0301, "put an acute accent on the previous character". Two different
  strings, one appearance. Any `==` has to say whether they are equal, and there are good
  arguments both ways.
- The flag is two characters, the "regional indicators" F and R.
- The family is five: three people and two invisible joiners.

What a reader sees as one symbol is called a **grapheme**, and finding graphemes needs tables
that change with every Unicode release. So "how long is this text?" has at least four honest
answers (bytes, UTF-16 units, characters, graphemes) and a fifth if you mean width on screen.
A language that hands you one number and calls it `length` has picked for you, silently. That
was the table at the top of this chapter.

---

## 9.10 From bytes to text

Text arrives from outside as bytes: a file, a network connection, a keyboard. Two operations
turn bytes into a `String`. Both take a list of numbers written `array { … }`, which is
chapter 12's subject; here it is only a way to write some bytes down.

<!-- file docs/book/code/text/bytes.oro -->
```go
use io

export fn main() {
    io.print_line(utf8_text(array { 0x41 }))
    io.print_line(utf8_text(array { 0xC3, 0xA9 }))
    io.print_line(utf8_text(array { 0xE6, 0x97, 0xA5 }))
    io.print_line(utf8_text(array { 0xF0, 0x9F, 0x99, 0x82 }))
    io.print_line(utf8_text(array { 0x63, 0x61, 0x66, 0xC3, 0xA9 }))
    io.print_line(ascii(array { 72, 105, 33 }, 0, 3))
}
```

<!-- run go docs/book/code/text/bytes.oro -->
```
A
é
日
🙂
café
Hi!
```

**`utf8_text(bytes)`** is the string those bytes encode in UTF-8. One byte for `A`, two for
`é`, three for `日`, four for the smile: 9.8's table, run forwards. Work the second one by
hand. C3 is `11000011` and A9 is `10101001`; the table's second row says keep the last five
bits of the first and the last six of the second: `00011` and `101001`, together `11101001`,
which is 0xE9, `é`.

**`ascii(bytes, start, count)`** is for the simple case: `count` bytes from `start`, each
below 128, each its own character.

Not every sequence of bytes is text:

<!-- file docs/book/code/text/valid.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(bit(utf8_valid(array { 0xE6, 0x97, 0xA5 })))
    io.print_int(bit(utf8_valid(array { 0xE6, 0x97 })))
    io.print_int(bit(utf8_valid(array { 0xA5 })))
    io.print_int(bit(utf8_valid(array { 0xC0, 0x80 })))
    io.print_int(bit(utf8_valid(array { 0xED, 0xA0, 0x80 })))
    io.print_int(bit(utf8_valid(array { 0xFF })))
}
```

<!-- run go docs/book/code/text/valid.oro -->
```
1
0
0
0
0
0
```

**`utf8_valid(bytes)`** says whether the bytes are well-formed UTF-8. Five ways to fail:

1. `E6 97 A5` is `日`. Valid.
2. `E6 97`: the first byte promised three and there are two. Cut off.
3. `A5` alone: a middle byte with no beginning.
4. `C0 80`: the two-byte pattern used to write the number 0, which fits in one byte. UTF-8
   allows only the shortest form, so that each character has exactly one encoding. (Without
   that rule a filter looking for the byte `/` could be dodged by writing `/` the long way.
   That was a real attack on web servers in 2000.)
5. `ED A0 80`: the three-byte pattern for 0xD800, the first number of the hole. Not a
   character, so not text.
6. `FF`: no UTF-8 sequence contains this byte at all.

`utf8_text` requires valid bytes, as `string_of` requires a character's number. For the
literals above the compiler checked while compiling. For bytes read at run time, the program
tests `utf8_valid` first, and the test is the proof. So a `String` is always well-formed, on
every target, however it was made. Go's `string` does not promise that, and neither do
Java's or JavaScript's, which can hold half of a surrogate pair.

---

## 9.11 A property that survives joining

One more operation, and it is here for what it teaches about proofs.

In 1972 the C language decided how to mark the end of a string: put a byte 0 after the last
character. No length is stored; you find the end by walking until you meet the zero. The
operating systems that followed, Unix and Windows both, took their strings in that form, and
still do. It has a consequence: **a string that contains character number 0 cannot be handed
to them**, because they will stop reading at it. A file name `report` + (number 0) + `.txt`
passes a check that it "ends in .txt" and then opens the file `report`. That exact trick has
broken security checks in real systems for decades.

Character 0 is a legitimate character here (U+0000), so a `String` may contain it. The
predicate **`nulfree(s)`** says it does not:

<!-- file docs/book/code/text/zero-char.oro -->
```go
use io

fn bit(c ∈ Bool) (0..1) { if c { 1 } else { 0 } }

export fn main() {
    io.print_int(bit(nulfree("report.txt")))
    io.print_int(bit(nulfree("report\u{0}.txt")))
    io.print_int(bit(nulfree("")))
}
```

<!-- run go docs/book/code/text/zero-char.oro -->
```
1
0
1
```

A function that will pass its argument to the operating system says so in its header, with
chapter 6's `where`:

<!-- file docs/book/code/text/cname.oro -->
```go
fn open_name(s ∈ String) (String) where nulfree(s) { s }

export fn fixed() (String) { open_name("report" ++ ".txt") }

export fn checked(s ∈ String) (String) {
    if nulfree(s) { open_name(s ++ ".txt") } else { "" }
}
```

<!-- gen go docs/book/code/text/cname.oro -->
```go
func TextChecked(v0 string) string {
    v1 := ((strings.IndexByte(v0, 0) < 0))
    if v1 {
        v3 := (v0 + ".txt")
        return v3
    }
    return ""
}

func TextFixed() string {
    v2 := ("report" + ".txt")
    return v2
}
```

Both calls were proven, and look at what the second proof needed. `checked` tested `s`. It
then passed `s ++ ".txt"`, a *different* string that was never tested, and the compiler
accepted it. It knew three things: `s` has no zero (the test), `".txt"` has no zero (it can
see the literal), and

> **`nulfree(a ++ b)` is exactly `nulfree(a) && nulfree(b)`.**

A joined string has a zero in it only if one of the pieces did. So the property follows the
structure of the string: check the pieces once, and every string built from them by `++` is
known good without looking at it again. One scan in the emitted Go, not two.

This is the same idea as chapter 7's "the machine may wrap early". There, taking a remainder
commuted with `+` and `*`. Here, "has no zero" commutes with `++`, turning it into `&&`. A
function that turns one monoid's operation into another's is called a **homomorphism**, and it
is the most useful shape a property can have: it can be computed piece by piece. Length would
be another (the length of `a ++ b` is the sum of the lengths, `++` into `+`), if it were
defined.

The proof cannot be faked:

<!-- file docs/book/code/text/no-cname.oro -->
```go
fn open_name(s ∈ String) (String) where nulfree(s) { s }

export fn hidden() (String) { open_name("report\u{0}.txt") }
```

<!-- gen go docs/book/code/text/no-cname.oro -->
```
gen: text-hidden: open_name requires nulfree("report\u{0}.txt") at a call, which does not follow
```

and a string nobody tested is not assumed good:

<!-- file docs/book/code/text/no-unknown.oro -->
```go
fn open_name(s ∈ String) (String) where nulfree(s) { s }

export fn unknown(s ∈ String) (String) { open_name(s) }
```

<!-- gen go docs/book/code/text/no-unknown.oro -->
```
gen: text-unknown: open_name requires nulfree(s) at a call, which does not follow
```

---

## 9.12 The host's own strings

Chapter 3 said a target can supply modules. Go's standard library is full of string
functions, and a program for Go may use them:

<!-- file docs/book/code/text/host.oro -->
```go
use io
use go
use go/strings

export fn main() {
    io.print_line(strings.ToUpper("stra\u{DF}e caf\u{E9}"))
    io.print_int(strings.Count("banana", "an"))
    io.print_int(strings.Index("chicken", "ken"))
    io.print_line(go.text(strings.Repeat("ab", 3)) ++ "!")
}
```

<!-- run go docs/book/code/text/host.oro -->
```
STRAßE CAFÉ
2
4
ababab!
```

Here is counting and searching, and it is honest about what it is: `strings.Index` returns
Go's answer, a position in bytes, and the program that calls it runs on Go only.

<!-- portable docs/book/code/text/host.oro -->
```
portable to go — not js, java, windows
```

The last line of `main` shows the one piece of care this needs. A Go `string` is any sequence
of bytes, valid text or not, so it is a different set from `String`, a bigger one. A `String`
may go wherever Go wants a string (every text is bytes). The other way needs a step:

<!-- file docs/book/code/text/no-host.oro -->
```go
use io
use go/strings

export fn main() {
    io.print_line(strings.Repeat("ab", 3) ++ "!")
}
```

<!-- build go docs/book/code/text/no-host.oro -->
```
build: main: in argument 1 of io.print_line: in argument 1 of concat: `go/strings`.Repeat is go.bytestring, but String is required here
```

`strings.Repeat` hands back whatever bytes it was given, repeated, so its result is typed as
Go's string, `go.bytestring`. **`go.text`** brings it into `String`: it checks the bytes, and
replaces anything that is not well-formed UTF-8 with the character U+FFFD, `�`, which Unicode
reserves for "something unreadable was here". It never fails. `strings.ToUpper` needed no such
step, because Go's source shows it always produces valid text, and its declaration says so.

Chapter 16 is about writing those declarations.

---

## 9.13 What other languages do

| | a string is | `length` counts | `s[i]` gives | can hold ill-formed text? |
|---|---|---|---|---|
| Oroboros | a sequence of Unicode scalar values | not in the language | not in the language | no |
| C | bytes up to the first 0 | bytes (`strlen` walks them) | a byte | yes |
| Go | bytes, by convention UTF-8 | bytes | a byte | yes |
| Java, JavaScript, C# | UTF-16 units | UTF-16 units | a unit, maybe half a character | yes, a lone surrogate |
| Python 3 | code points | code points | a code point | yes, a lone surrogate |
| Rust | bytes, always valid UTF-8 | bytes | a compile error | no |
| Swift | graphemes | graphemes, by walking the string | a grapheme, by a special index type | no |

Rust is the nearest relative. Its strings are guaranteed well-formed, and `s[0]` on a string
does not compile there either, for the reason given here: there is no one right answer, so
the programmer is asked which they mean (`.bytes()`, `.chars()`).

Swift went the other way and chose the answer a person would give, graphemes, and paid for
it: counting is a walk of the whole string, and the answer can change when the operating
system updates its Unicode tables.

Python 3 counts code points and so nearly agrees with this chapter's "character", except that
it lets the hole in: `'\ud83d'` is a Python string, and writing it to a file raises an error
far from where it was made.

---

## What to remember

- **A string is a finite sequence of characters; a character is a Unicode number.** `String`
  is the set of them. A string never changes.
- **The numbers that are characters are the scalar values**: 0 to 0x10FFFF, except the hole
  0xD800 to 0xDFFF.
- **A literal is text in double quotes.** Six escapes: `\"`, `\\`, `\t`, `\n`, `\r`, and
  `\u{…}` for any character by number. A literal ends on its own line.
- **`string_of(n)`** is the one-character string for `n`, which must be proven a scalar value.
  The digit `d` is `string_of(48 + d)`.
- **`a ++ b` joins.** Grouping does not matter, `""` changes nothing, order matters. That is a
  monoid, and strings are the free one.
- **`+` is not `++`**, and a number is not converted to text for you.
- **Hosts store text differently**: Go in UTF-8 (bytes), Java and JavaScript in UTF-16 (units,
  with surrogate pairs), and "length" means a different thing on each.
- **So `len(s)`, `s[i]` and `==` on strings are not in the language.** Text a program must
  examine is handled as bytes and made a `String` at the end.
- **`utf8_text(bytes)` and `ascii(bytes, start, count)`** make a `String` from bytes;
  `utf8_valid(bytes)` says whether bytes are well-formed. A `String` is always well-formed.
- **`nulfree(s)`** says `s` has no character 0, and `nulfree(a ++ b)` is `nulfree(a) &&
  nulfree(b)`: a property proven piece by piece.
- **A host's string is the host's** (`go.bytestring`); `go.text` brings one into `String`.

Next: [chapter 10](10-data.md), Tuples and variants. Two values at once, and one value out of several
kinds.
