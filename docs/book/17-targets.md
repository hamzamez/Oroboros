# 17. Targets

One line has been in nearly every program since chapter 0, and it has never been explained:

```
use io
```

`io.print_line` is not part of the language. Neither is `io.print_int`, nor `os.ReadFile`,
nor `num.decimal`. Chapter 1 said a program is reduced until only names the target provides
are left, and stopped there. Fifteen chapters later it is time to look at the other side of
that sentence. So the question of this chapter is: **where do the names at the bottom come
from, and how does one program come out as Go, JavaScript, Java and a Windows executable?**

The answer is three sentences.

> **A target is a set of declarations**: for each thing a host can do, a header saying what
> it takes and gives, and the host's own text to emit.
>
> **A declaration is a claim about the host that the compiler cannot check.** It is believed.
> Everything proven in a program rests on those claims being true.
>
> **A program runs on a target when every name it reaches is provided there.** That is
> computed, and reported.

Along the way: the two hard problems of calling someone else's code, how one name gets four
implementations, what happens when a declaration lies, and where the "word" of chapter 6
and the `[]byte` of chapter 13 were actually decided.

The examples are in [code/hosts/](code/hosts/). `go run docs/book/check.go
docs/book/17-targets.md` re-runs every block. Checked 2026-11-16.

---

## 17.1 What `io.print_line` is

<!-- file docs/book/code/hosts/hello.oro -->
```go
use io

export fn main() {
    io.print_line("hello")
    io.print_int(6 * 7)
}
```

<!-- gen go docs/book/code/hosts/hello.oro -->
```go
import "fmt"
import "strconv"

func HostsMain() int {
    fmt.Println("hello")
    v3 := (strconv.Itoa(42))
    fmt.Println(v3)
    return 42
}
```

<!-- gen js docs/book/code/hosts/hello.oro -->
```js
export function hostsMain() {
    console.log("hello");
    console.log((String(42)));
    return 42;
}
```

The same two lines became `fmt.Println` on Go and `console.log` on JavaScript, and Go got an
`import "fmt"` at the top. Nothing in the compiler knows the words `fmt` or `console`. They
come from two small files in the library. Here is Go's:

<!-- part lib/io/go.oro -->
```go
provides go io {
    use io
    fn print_line(s ∈ String) (String) { host stmt "fmt.Println(${s})" import "fmt" }
…
    fn print_int(n ∈ Int) (Int) {
        ignore(io.print_line(decimal(n)))
        n
    }
}
```

and JavaScript's:

<!-- part lib/io/js.oro -->
```go
provides js io {
    use io
    fn print_line(s ∈ String) (String) { host stmt "console.log(${s})" }
…
    fn print_int(n ∈ Int) (Int) {
        ignore(io.print_line(decimal(n)))
        n
    }
}
```

That is all `io` is. Read the first one a piece at a time.

- **`provides go io { … }`**: on the target `go`, the module `io` has these members.
- **`fn print_line(s ∈ String) (String)`**: an ordinary header. It takes a string and gives
  one back.
- **`{ host stmt "fmt.Println(${s})" import "fmt" }`**: where a body would be, there is host
  text. `${s}` is where the argument goes. `import "fmt"` is what the emitted file must
  import for that text to mean anything.

A function whose body is `host …` is a **declaration**. It does not say how to print. It
says: *the host can do this; here is how to ask, and here is what I claim about it.*
Reduction stops at a declared name, because there is nothing to unfold. That is the precise
meaning of chapter 1's "until only what the target provides is left".

`print_int` is not a declaration. It is a definition, written over the others: print the
integer's `decimal` text, chapter 6's rendering of any integer, and be that integer. `decimal`
is the target's in turn: on a value its facts keep in the word it is the host's own formatting,
`strconv.Itoa` and `String`, which is why those appear above, and on a value above the word the
host's big integer's. One name, `n ∈ Int`, prints every integer.

---

## 17.2 The anatomy of a declaration

Go's own library is declared the same way, a file per package, under `targets/go/`. Three
declarations from `go/strings`, which chapter 9 used:

<!-- part targets/go/strings.oro -->
```go
    fn Index(s ∈ bytestring, substr ∈ bytestring) (-1..2^63-1) pure {
        host expr "strings.Index(${s}, ${substr})" import "strings"
    }
…
    fn ToUpper(s ∈ bytestring) (String) pure { host expr "strings.ToUpper(${s})" import "strings" }
…
    fn Repeat(s ∈ bytestring, count ∈ Word) (bytestring) pure
        where count >= 0 {
        host expr "strings.Repeat(${s}, ${count})" import "strings"
    }
```

Every part of a header that this book has taught appears, and each one now means *a fact
about Go that somebody looked up*.

**`host expr` or `host stmt`.** An *expression* has a value and can sit inside a larger
expression: `strings.Index(a, b) + 1`. A *statement* is done for its effect and stands on a
line of its own: `fmt.Println(s)`. The declaration says which the host text is, because the
emitter has to know where it may be put.

**`pure`, or its absence.** Chapter 5: an effect may not be dropped, copied or moved, and a
pure computation may. `strings.Index` only looks, so it is `pure`. `print_line` is not
marked, so it is taken to have an effect. The default is the safe one. A declaration with no
`pure` is never reordered or removed, and the worst a forgotten `pure` costs is a missed
optimization.

**The parameters' sets.** `count ∈ Word`: Go's `int`, the word, which the Go module names once,
`set Word = -2^63..2^63-1`. Not `Int`, which is every integer and which no Go function takes. A
call must prove its count is a word, as a table's store must prove its value is. And `where
count >= 0`: Go's `strings.Repeat` panics on a negative count. Declared this way, that panic is
chapter 16's obligation. Every call must prove `count >= 0`, and a program that could pass −1
does not compile.

**The result's set.** `(-1..2^63-1)` for `Index`: Go's documentation says it returns −1 for
"not found" and otherwise a position. That range is what the compiler knows about the
result. And `ToUpper` gives a `String` where `Repeat` gives a `bytestring`: chapter 9's
distinction, and the comment above those lines in the file records how it was established,
by running Go's `ToUpper` on a million inputs to see whether it could ever return invalid
text.

So a host function with a precondition, a failure mode and a range of results is written
down as a header, in the same notation as your own functions, and from then on the compiler
treats a call of it exactly as it treats a call of yours.

---

## 17.3 Writing one

Declarations are not reserved for the people who wrote the compiler. Go has a clock, and
nothing in the library reaches it. Make a folder named for the module, and in it a file
named for the target:

<!-- file docs/book/code/hosts/clock/go.oro -->
```go
provides go clock {
    fn seconds() (0..2^62) { host expr "time.Now().Unix()" import "time" }
    fn year_of(s ∈ 0..2^62) (1970..9999) pure { host expr "time.Unix(${s}, 0).UTC().Year()" import "time" }
}
```

Two declarations.

- **`seconds`** reads the clock: seconds since the start of 1970, the way nearly every
  computer counts time. It has no `pure`, and must not: two calls give two different
  answers. It is the effect of chapter 5 that was called "reading the world".
- **`year_of`** computes the calendar year of such a count. Same input, same output, no
  effect: `pure`.

A program uses the module by name, like any other:

<!-- file docs/book/code/hosts/year.oro -->
```go
use io
use clock

export fn main() {
    io.print_int(clock.year_of(0))
    io.print_int(clock.year_of(1000000000))
    io.print_int(clock.year_of(1700000000))
}
```

<!-- run go docs/book/code/hosts/year.oro -->
```
1970
2001
2023
```

The billionth second was in 2001. Now watch the two declarations being treated differently,
exactly as their headers ask:

<!-- file docs/book/code/hosts/now.oro -->
```go
use clock

export fn stamp() (Int) { clock.seconds() }

export fn elapsed() (Int) {
    start = clock.seconds()
    finish = clock.seconds()
    finish - start
}

export fn unused() (Int) {
    clock.seconds()
    clock.year_of(0)
    7
}
```

<!-- gen go docs/book/code/hosts/now.oro -->
```go
import "time"

func HostsElapsed() int {
    v0 := int(time.Now().Unix())
    v1 := int(time.Now().Unix())
    v2 := (v1 - v0)
    return v2
}

func HostsStamp() int {
    v0 := int(time.Now().Unix())
    return v0
}

func HostsUnused() int {
    time.Now().Unix()
    return 7
}
```

- `elapsed` reads the clock twice, in order, and subtracts. The two calls are the same
  text, and a compiler that thought them pure would compute one and reuse it, and `elapsed`
  would always be 0. They were kept apart because `seconds` is not `pure`.
- `unused` calls both and uses neither. `clock.seconds()` is still there in the Go: an
  effect may not be dropped. `clock.year_of(0)` is gone: a pure computation nobody reads is
  nothing.

And `v1 - v0` was proven to fit the word, from the claimed `(0..2^62)`. The header you wrote
is where that fact came from.

---

## 17.4 One name, several hosts

The program above says `clock.year_of`. It does not say Go. Give JavaScript a file of its
own:

<!-- file docs/book/code/hosts/clock/js.oro -->
```go
provides js clock {
    fn seconds() (0..2^52) { host expr "Math.floor(Date.now() / 1000)" }
    fn year_of(s ∈ 0..2^52) (1970..9999) pure { host expr "new Date(${s} * 1000).getUTCFullYear()" }
}
```

The same two names, the same meaning, entirely different host text. JavaScript's clock
counts in thousandths of a second and its dates are built from those, so the declarations
divide and multiply by 1000 to present the same interface. (The range is `0..2^52` here,
because that is what a JavaScript number holds exactly.)

<!-- run js docs/book/code/hosts/year.oro -->
```
1970
2001
2023
```

`year.oro` did not change. It ran on a second host because the name it uses is now provided
there. This is how `io` works, and `os`, and every library in this book that ran "on three
targets": a folder with one file per target, each providing the same names in that host's
own terms.

What about a host with no file?

<!-- portable docs/book/code/hosts/year.oro -->
```
  go       accepted
  js       accepted
  java     refused   docs/book/code/hosts/year.oro: in main: clock.year_of is not bound — it is not a parameter, not a definition, and not a primitive on this target
  windows  refused   docs/book/code/hosts/year.oro: in main: clock.year_of is not bound — it is not a parameter, not a definition, and not a primitive on this target
portable to go, js — not java, windows
```

Chapter 6 introduced this tool for word sizes. Here is its other half. A program reaches
some set of names, and each target provides some set of names. **The program is portable to
exactly the targets that provide all it reaches** (and whose word holds its arithmetic).
There is no list of "portable features" to memorize, and no promise that everything runs
everywhere. There is a computation, and it tells you the answer for the program in front of
you.

To make `year.oro` run on Windows, write `clock/windows.oro`. Nothing else would need to
change.

### A library need not be host text

Windows has no `fmt.Println`. What is `io.print_line` there?

<!-- part lib/io/windows.oro -->
```go
    fn print_line(s ∈ String) (String) {
        ignore(print.print_str(s))
        s
    }
```

An ordinary function, written in this language, which calls another module written in this
language, which in the end calls `WriteFile`, a function Windows itself exports. On a host
that offers a high-level operation the declaration is one line of host text. On a host that
offers only a low-level one, the same name is a small program over it. The rule is to use
the highest thing the host really has, and build only the rest: a Go programmer would be
right to object to a hand-written print routine in a language that has `fmt`.

---

## 17.5 A declaration is believed

Here is the uncomfortable part. Nothing checks a declaration against the host. The compiler
cannot read Go's source, or JavaScript's engine, or Windows. It takes your header as true.

So lie to it.

<!-- file docs/book/code/hosts/liar/go.oro -->
```go
provides go liar {
    fn say(s ∈ String) (String) pure { host stmt "fmt.Println(${s})" import "fmt" }
    fn small() (0..9) pure { host expr "1000" }
}
```

Two false claims. `say` prints and is declared `pure`. `small` is declared to be at most 9
and is in fact 1000.

<!-- file docs/book/code/hosts/lies-run.oro -->
```go
use io
use liar

export fn main() {
    liar.say("this line is never printed")
    io.print_int(liar.small() * 10^18)
}
```

<!-- run go docs/book/code/hosts/lies-run.oro -->
```
3875820019684212736
```

Both lies were acted on, faithfully.

The first line of `main` printed nothing. `say` was declared pure, its result was not used,
and a pure computation nobody reads is dropped. The compiler did what chapter 5 says it may.

The second line printed nonsense. `small() * 10^18` was "proven" to fit the word, since 9 ×
10¹⁸ does. The true value, 1000 × 10¹⁸, does not, and Go's multiplication wrapped round:
the number printed is what is left of 10²¹ after the high bits fell off. That is the silent
wrong answer chapter 6 said could not happen in this language. It can happen in exactly one
way, and this is it:

<!-- file docs/book/code/hosts/lies.oro -->
```go
use liar

export fn quiet() (Int) {
    liar.say("this line is never printed")
    7
}

export fn overflow() (Int) {
    liar.small() * 10^18
}
```

<!-- gen go docs/book/code/hosts/lies.oro -->
```go
note: hosts-overflow: 1 of 1 integer operations bounded; 0 of 0 loop(s) proven terminating
…
func HostsOverflow() int {
    v0 := int(1000)
    v2 := (v0 * 1000000000000000000)
    return v2
}

func HostsQuiet() int {
    return 7
}
```

"1 of 1 integer operations bounded." The proof is valid. Its premise was false.

Every proof in this book has the same shape: *if the declarations are true, then …*. The
declarations are the **trusted base**: the part that is assumed and not proven. Every
system that proves anything has one. A C compiler trusts the processor's manual. A Java
program trusts the JVM. What matters is that the trusted part be small, be written down in
one place, and be readable by a person. Here it is a set of headers, in the language's own
notation, in files you can open.

This is why the project's host declarations are written by hand, by someone who read the
host's source or documentation, with the evidence in a comment beside each (17.2's million
inputs). A tool then compares each one with the host's actual signature and reports any
that disagree. A declaration that claims less than the truth is safe and merely weak:
`(Int)` where `(0..9)` would do loses you a proof and nothing else. A declaration that
claims more than the truth is how a wrong answer gets in. When in doubt, claim less.

---

## 17.6 A function the host calls

Some host functions take a function. Go's `strings.Map` takes one from scalars to scalars and
applies it to every scalar of a string:

<!-- file docs/book/code/hosts/rot13.oro -->
```go
use io
use go/strings

fn rot(r ∈ -2^31..2^31-1) (-2^31..2^31-1) {
    case {
        97 <= r && r <= 122 -> 97 + (r - 97 + 13) % 26
        65 <= r && r <= 90 -> 65 + (r - 65 + 13) % 26
        else -> r
    }
}

export fn main() {
    s = "Hello, World"
    t = strings.Map(fn(r) { rot(r) }, s)
    io.print_line(t)
    io.print_line(strings.Map(fn(r) { rot(r) }, t))
}
```

<!-- run go docs/book/code/hosts/rot13.oro -->
```
Uryyb, Jbeyq
Hello, World
```

ROT13 twice is the identity: a rotation by 13 in a cycle of 26, done twice, is a rotation by 26.
And `Map` keeps that, because it maps each scalar and nothing else, so mapping by f and then
by g is mapping by g ∘ f.

The λ is written at the call, as chapter 1 wrote one. Its parameter takes the set Go's
declaration gives it:

```go
set RuneMap = fn(r ∈ int32) (m ∈ int32)
fn Map(mapping ∈ RuneMap, s ∈ bytestring) (String) pure { host expr "strings.Map(${mapping}, ${s})" import "strings" }
```

`RuneMap` is a *claim*, the set of functions of that shape (chapter 16). The host promises
`r ∈ int32`, so the λ's body is decided under that. The λ owes its result in `int32`, because
Go receives it as one; `fn(r) { r + 1 }` is refused, since `r + 1` can reach 2³¹. And the λ
is applied only during the call: Go holds nothing afterwards, so it may read the values around
it, here none, in place.

What Go does *not* promise is the order or the number of its calls. Its documentation says what
`Map` returns, not how it gets there. So a λ that does something, rather than computes
something, is refused:

<!-- file docs/book/code/hosts/no-print.oro -->
```go
use io
use go/strings

export fn comma(s ∈ go.bytestring) (Int) {
    strings.IndexFunc(s, fn(r) { io.print_int(r); r == 44 })
}
```

<!-- gen go docs/book/code/hosts/no-print.oro -->
```
gen: hosts-comma: fn(r) { #_ = { #nl3 = io.print_line(decimal(r)); r }; r == 44 } calls io.print_line, an effect, and its host does not promise the order or the number of its calls (`go/strings`.RunePred is not `ordered`): the effect's history would be the host's, which may differ between releases (Theorem D, ADR 0143). Make the λ pure, or pass it to a host whose declaration is `ordered`
```

The print is shown as chapter 5 shows a statement, a binding nobody reads, `#_`, and as the
compiler holds it: `io.print_int` unfolded into its definition, whose effect is
`io.print_line`'s (a term a message quotes is shown reduced, the half of the book's finding 110
not fixed). And a count is
refused too, since writing a cell is an effect:

<!-- file docs/book/code/hosts/no-count.oro -->
```go
use io
use go/strings

export fn letters(s ∈ go.bytestring) (Int) {
    var n := 0
    t = strings.Map(fn(r) { if n < 1000000 { n := n + 1 }; r }, s)
    n
}
```

<!-- gen go docs/book/code/hosts/no-count.oro -->
```
gen: docs/book/code/hosts/no-count.oro: letters: fn(r) { if n < 1000000 { n := n + 1 } else { () }; r } writes the cell n, and `go/strings`.`Map` does not promise the order or the number of its calls (no `ordered`): the cell's last value would be the host's, which may differ between releases (Theorem D, ADR 0143). Count with a loop of your own, or pass the λ to a host whose declaration is `ordered`
```

This is not caution for its own sake. Go's `strings.FieldsFunc` once called its function twice
for every scalar, and now calls it once, and its documentation never promised either. A count
written in it would give one answer on one Go release and another on the next. A pure λ cannot
tell: whatever order and however many calls, `Map` sees the same answers.

A host that does promise its order says so in its declaration. `slices.EqualFunc`'s
documentation says "the elements are compared in increasing index order, and the comparison
stops at the first index for which eq returns false":

```go
set IntEq = fn(x ∈ Int, y ∈ Int) (b ∈ Bool) ordered
```

Under `ordered` the cell is threaded through the calls in that order, as a loop threads a
cell through its iterations (chapter 14):

<!-- file docs/book/code/hosts/compared.oro -->
```go
use io
use go/slices

export fn main() {
    var k := 0
    same = slices.EqualFunc(array { 3, 1, 4, 1, 5 }, array { 3, 1, 4, 2, 5 }, fn(x, y) {
        if k < 1000000 { k := k + 1 }
        x == y
    })
    io.print_line(if same { "equal" } else { "different" })
    io.print_int(k)
}
```

<!-- run go docs/book/code/hosts/compared.oro -->
```
different
4
```

Four comparisons: the three equal pairs, then (1, 2), where it stopped. The guard on `k` is
there for the reason a loop of unknown length needs one. The host promises no bound on how
many times it calls, so `k + 1` is proven inside the word only under `k < 1000000`.

The same λ is written for any host that takes one. JavaScript's `every` and the JVM's `allMatch`
are both ∀ over a table, each declared with its own claim, and each prints the λ as its host's own
function value, an arrow function and a lambda:

<!-- file docs/book/code/hosts/every-js.oro -->
```go
use io
use js/Array

export fn main() {
    a = array { 3, 1, 4, 1, 5 }
    c = len(a)
    io.print_line(if Array.every(a, fn(x) { x <= c }) { "every element is at most the length" } else { "not every" })
}
```

<!-- run js docs/book/code/hosts/every-js.oro -->
```
every element is at most the length
```

<!-- file docs/book/code/hosts/every-java.oro -->
```go
use io
use java/Arrays

export fn main() {
    a = array { 3, 1, 4, 1, 5 }
    c = len(a)
    io.print_line(if Arrays.allMatch(a, fn(x) { x <= c }) { "every element is at most the length" } else { "not every" })
}
```

<!-- run java docs/book/code/hosts/every-java.oro -->
```
every element is at most the length
```

JavaScript's `every` is declared `ordered`: ECMAScript says it calls the function "once for each
element present in the array, in ascending order". The JVM's `allMatch` is not, since the
streams' documentation promises no order for it. So a function given to `every` may write a
variable of the program's, and one given to `allMatch` may not.

`ordered` is a claim like `pure` (17.5): believed, not proven. The project keeps, beside each
one, the sentence of the host's documentation that promises it, and a tool checks that the
sentence is there: against `go doc` for Go, against the JDK's own source files for the JVM, and
for JavaScript and Windows the sentence and the page it came from are kept, read once by a person.

---

## 17.7 What else a target says

Declarations of functions are most of a target. The rest is a short list of facts about the
host, and several things earlier chapters stated as given were read from it.

<!-- part targets/go/go.oro -->
```go
repr 0..255 host "byte"
repr -128..127 host "int8"
…
repr -2^63..2^63-1 word
…
set Array(A) = host "[]${A}"
set Map(K, V) = host "map[${K}]${V}"
```

- **`repr -2^63..2^63-1 word`** is chapter 6's *word*: the range in which Go's native
  arithmetic is exact. Every "proven to fit the word" was a comparison against this line.
- **`repr 0..255 host "byte"`** is why chapter 13's `Array(0..255)` came out as `[]byte` and
  `Array(0..1000)` as `[]uint16`: a list of the host's storage sizes, narrowest first, and
  the compiler takes the first that holds the element's set.
- **`set Array(A) = host "[]${A}"`** is how a table is spelled on Go at all.

JavaScript's file makes different statements, and the same programs come out differently:

<!-- part targets/js/js.oro -->
```go
repr -9007199254740991..2^53-1 word
…
repr 0..255 host "Uint8Array"
```

A word of ±(2⁵³ − 1): that one line is why chapter 6's `n * n` over `0..2^31` was accepted
on Go and refused on JavaScript, and why chapter 7 went to such lengths over 32-bit
operations there.

The word, the storage sizes, the spelling of a table, the spelling of each operation: none
of it is in the compiler's code. It is data, in files a person can read and change. A new
host is, in principle, a new directory of such files and a printer for its syntax.

---

## 17.8 The four targets

| target | the host | a program becomes |
|---|---|---|
| `go` | the Go toolchain | Go source |
| `js` | a JavaScript engine: the browser | an ES module |
| `java` | the Java virtual machine: Android | Java source |
| `windows` | Windows on x86-64, through LLVM | LLVM IR, then a native executable |

Windows is a family, and each member is a file that glues the shared Win32 layer and states what
it changes: `windows32` the 32-bit processor, and `windows-v3` the 64-bit one with its newer
vector instructions named,

<!-- part targets/windows-v3/windows-v3.oro -->
```go
glue windows
…
cpu "x86-64-v3"
```

one line, and a program built for it runs only on a machine that has them. On the integer dot
product of [`examples/table/idot.oro`](../../examples/table/idot.oro) that buys 0.60–0.69× the
time. Naming more of the machine makes a program faster and less portable, and the choice is
the target's data, as everything in this section is. `windows` names none, and runs on any
64-bit Windows.

The first three are languages with large libraries of their own, and a program for them is
mostly calls into what is already there. The fourth is different in kind. There is no
runtime underneath and no library: the "host functions" are LLVM's instructions and the
operating system's exported calls, and things such as printing an integer or joining two
strings are written in this language on top of those. Chapter 9's string of bytes with its
length in front, chapter 14's block of memory for a buffer: on Windows somebody had to
decide those, and it is all in `targets/llvm/`, `targets/win32/` and each member's directory.
Calling the operating system from a program, as C does, is chapter 18.

That the same source means the same thing on all four is the claim the whole design is
built to keep. It is why an integer is an integer and not a machine word (chapter 6), why a
string has no visible storage (chapter 9), and why the things on which hosts differ are
either hidden behind a library that makes them agree or left as target names that carry no
promise.

That last distinction is worth stating exactly.

| a name is | provided by | does it mean the same everywhere? |
|---|---|---|
| in the language: `+`, `loop`, `case`, `++`, `len`, … | the compiler, on every target | yes: that is what being in the language means |
| in a portable library: `io.print_line`, `os.ReadFile` | one file per target | yes, where provided: the files are written to agree |
| a host's own: `go/strings.Index`, `clock.seconds` on Go | that target only | no claim. It is Go's, with Go's meaning |

A program built only from the first two rows is portable to every target that has its
libraries. A program that reaches into the third row has chosen a host, and `portable`
says which.

---

## 17.9 What other languages do

Calling code written in another language is called a **foreign function interface**, and
every language has to have one, because the operating system is foreign to all of them.

| | how a host function is described | who checks the description | one source, several hosts? |
|---|---|---|---|
| Oroboros | a header with sets, `where` and `pure`, and host text | nobody: it is the trusted base | per-target files providing one name |
| C | a prototype in a header file | the linker, loosely | `#ifdef` |
| Java (JNI), Python (ctypes), Go (cgo) | a signature and glue code | nobody; a mistake is a crash | separate glue per platform |
| Rust | an `extern` block; every call is `unsafe` | nobody; `unsafe` marks the spot | `#[cfg(...)]` |
| TypeScript | a `.d.ts` declaration file | nobody: it is believed | n/a: one host |
| Kotlin Multiplatform | `expect` in common code, `actual` per platform | the compiler checks they match | yes: exactly this |
| Java, C# | not needed: one virtual machine | the VM vendor | by having only one host |

Two of those rows are close relatives.

TypeScript's declaration files are this chapter's 17.5. A `.d.ts` file states the types of
a JavaScript library that was written without any, the compiler believes it, and a wrong
one makes the type checker confidently wrong. The TypeScript community's answer is the
same as this project's: write them carefully, keep them in one known place, and test them
against the real library.

Kotlin Multiplatform's `expect` and `actual` are 17.4: declare a name once, provide it
per platform. The difference here is that the provided thing may be one line of host text,
with no wrapper function left at run time. `io.print_line("hello")` on Go *is*
`fmt.Println("hello")`, and nothing stands between them.

Java took the opposite road, and it is the famous one: define one virtual machine, run it
everywhere, and let every program be portable by having only one host to be portable to.
It works, and the price is that a Java program is a guest on every system it runs on. It
cannot be a Go library, or a browser module, or a Windows executable with no runtime. This
language's bet is the other way round: be a native of each host, and compute, program by
program, which hosts a given program is native to.

---

## What to remember

- **Nothing a program does to the outside world is in the language.** `io`, `os`, `num` are
  libraries.
- **A declaration is a header with `host` text for a body**: `fn f(x ∈ S) (R) { host expr
  "F(${x})" import "pkg" }`. Reduction stops at it.
- **`host expr`** is text with a value; **`host stmt`** is text done for its effect.
- **`pure`** says the host function only computes. Unmarked means it has an effect, and it
  is then never dropped, copied or reordered.
- **Its sets and `where` are claims about the host**: a `where` becomes an obligation on
  every caller, a result's set becomes a fact.
- **`provides TARGET MODULE { … }`** gives a module's members on one target. A library is a
  folder with a file per target.
- **A program is portable to the targets that provide every name it reaches.** `go run
  ./cmd/portable FILE` reports it.
- **A declaration is believed, not checked.** It is the trusted base. A false `pure` loses
  an effect, and a result claimed too narrow lets a wrong answer through.
- **Claim less when in doubt.** Too weak costs a proof; too strong costs correctness.
- **A target also states** its word, its storage sizes and how it spells a table. Those are
  data files, not compiler code.
- **Language names mean the same everywhere. A host's own names carry no such claim.**
- **A target may name its machine**: `windows-v3` is `windows` with `cpu "x86-64-v3"`, faster
  and runnable on fewer machines.

Next: [chapter 18](18-c.md), Calling C. The operating system's own functions, declared as C
declares them.
