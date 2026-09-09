# A tool runs on three hosts, and the gap was never the language

2026-09-09. `targets/{js,java}/os.oro`, `targets/{go,js,java}/io.oro`,
`emit/js.go`, `emit/java.go`, `emit/values.go`. On hamza's *"the I/O on js and
java is the same problem of supporting the api — we have yet to support it on go
and windows, it is going to be the same question: can we or not? … the fact that
we can't express the entire go api, and the entire windows api, tells us we are
still short."*

> **`wc`, `jsonfmt` and `freq` now build and run on Go, JavaScript AND Java, and
> produce BYTE-IDENTICAL output on all three** — seven real files for `wc` and
> `freq`, two JSON documents including non-ASCII, and every error path. `wc`
> agrees with GNU `wc -l`; `freq` agrees with `sort | uniq -c | sort -rn`.
>
> **THE LANGUAGE NEEDED NOTHING, AGAIN.** A fallible host call is TOTALISATION —
> a partial `A ⇀ B` is a total `A → B + E` — and **no host gives the coproduct**.
> Go's `(T, error)` plus `err-nil` is itself a totalisation nobody had named. So
> a target declares the CALL that produces the pair and the DISCRIMINATOR that
> reads it, and **the compiler learns nothing about exceptions**: a JavaScript
> `throw` and a Java checked exception are `try`/`catch` written in a TEMPLATE.
>
> **What was missing was two BACKENDS and one NAME.** `emitMultiPrim` existed on
> one backend of four — the same incoherence `values` was reverted for — and
> **JavaScript had no import mechanism at all**, silently dropping a field Go,
> Java and x86 all honour.
>
> **AND A MODULE NAME IS A CLAIM.** `go/os` names Go's `os` package; `os` names a
> `Σ` several targets share. That distinction is what makes these three programs
> portable, and it is now CHECKED — structurally by a test over the three
> targets' declarations, behaviourally by the three tools.
>
> **Five bugs, two of them cross-host divergences the differential suite could
> not see**, because no differential case reads a file or prints twice.
>
> **Cost: 171 of 171 pre-existing emitted files byte-identical**, seven new.

---

## 1. The algebra first, because it decided the design

A host offers a **partial** function `f : A ⇀ B`. Partiality is the whole content
of *"it can fail"*, and the standard correspondence is

```
    A ⇀ B   ≅   A → B + 1        and with information on the failure
    A ⇀ B   ≅   A → B + E
```

a **coproduct** — which sums.md already has, and which `(values b e)` already
carries across a function boundary. So **the type of a fallible call is
host-independent and the language has had it since values.md**.

What varies is the host's mechanism for saying which summand you got:

| | mechanism | what it really is |
|---|---|---|
| Go | a second return value | `B × E` plus a convention |
| JavaScript | `throw` | a partial map and an escape |
| Java | `throw`, checked | the same, with `E` in the method's type |
| windows | a sentinel plus `GetLastError` | `B + 1`, niche-encoded |

**NO HOST GIVES THE COPRODUCT.** Go's `(T, error)` is a *product* with a
discipline, and `(prim err-nil ((e error)) bool expr "%s == nil")` is the
discriminator that totalises it — so this repository has been totalising since
the day it could open a file and never called it that.

Two consequences, and both are what got built:

**A target declares two things and the compiler learns nothing about
exceptions** — the CALL that produces the pair, and the DISCRIMINATOR that reads
it. `try`/`catch` appears in `targets/js/os.oro` and `targets/java/os.oro` as
data. `emit/` contains the word *exception* only in a Java type name.

**The producer is a multi-result primitive, which is a LANGUAGE construct.** A
host call with several results working on one backend of four is exactly what
values.md was reverted for the first time: *a construct in the core that two of
four targets decline*. So the work is the two missing consumers, not a feature.

## 2. A module name is a claim, and that is what makes a program portable

Every module in `targets/` was named after a HOST NAMESPACE — `go/fmt` is Go's
`fmt` package, `js/Math` is V8's `Math`, `java/System` is that class. A prefixed
name claims nothing beyond that one host, which is right and is ADR 0001.

`targets/go/os.oro` was named `go/os` and declared six names — read a file, write
a file, test the error, decode bytes, argv, env. **Those are not Go's `os`
package; they are the file system.** The prefix was making three programs
un-portable by spelling.

The rule now stated, and it is target-system.md's own algebra one level up:

> `Decl ≅ Σ × I` — interface and implementation. A target FAMILY is a fibration
> over a shared `Σ`. **An unprefixed module is that shape between targets that
> are not a family**: `os` is one interface with an implementation per host, and
> a program whose free names lie inside it is PORTABLE — computed by the
> capability graph exactly as ADR 0001 says, rather than promised by a layer.

**It is not the retired portable layer returning, and the difference is that the
layer had BODIES.** `num/vec.materialize` and `fold-range2` lowered into shapes
the layer chose, and the price was measured at 1.79x. There is no body here at
all: each target's cell is its own host's call at the highest layer that host
provides — `os.ReadFile` on Go, `fs.readFileSync` on Node,
`Files.readAllBytes` on the JVM — and nothing is shared but the name and the
type.

**A claim nothing checks is decoration** (`split-words` passed every review for
two months while returning different answers on different targets), so
`TestTheUnprefixedModulesShareOneInterface` requires the three targets to declare
the same names at the same argument and result types, **in both directions** — a
target may not quietly add to a shared interface either, because a program
written against the richer one would look portable and not be.

### 2.1 `io`, and why it is three narrow names rather than one wide one

`fmt.Println` could not come along. Go's own name is variadic `...any`, and `any`
is where the agreement stops: a float prints `1e+06` on Go and `1000000` on V8.
So `io` declares `print-line`, `print` and `print-int` — **the signature is the
claim**, and integers.md already measured all four hosts agreeing on decimal
rendering inside ADR 0012's window. The checker refuses the rest.

## 3. What the build found, and two of the five are silent divergences

**(a) JavaScript had no import mechanism.** `(import …)` is one field of the
shared target format: Go collects it, Java collects it, x86 turns it into an
extern — and the JS backend dropped it, silently, because no target file for this
host had ever declared one. *A path nothing runs is a path nothing checks*, and
here that was a whole backend.

**(b) `%r` MET `fmt.Sprintf`.** The first JavaScript build emitted

```js
try { %!r(MISSING)0 = fs.readFileSync(%!)(MISSING); … }
```

because `fill` is `Sprintf` and `%r` is not a verb, so the WHOLE template came
back as a mangled string — with no error. Destination holes are filled BEFORE
argument holes now, and the order is also the safe one for a second reason: an
argument's emitted value is arbitrary text and could contain `%r0` (a string
literal can), where a destination is always an identifier the emitter just made.

**(c) `System.out.println` WRITES THE PLATFORM'S LINE SEPARATOR.** CRLF on
Windows, where `fmt.Println` and `console.log` write LF: **the same program
produced two different byte sequences**, which is exactly what makes a name Tier
2 rather than Tier 1. Found by diffing `freq` across three hosts on a file big
enough to trip its size guard, so the only line printed was the message and
nothing else could hide it.

**(d) `System.out` ENCODES IN THE PLATFORM CHARSET**, so `é` and `日` came out as
`?` where Go and V8 wrote UTF-8. string-literals.md hit this from the other side
and drew the line at PRINTING because nothing then could do better; here the
target file can say what the language means, so all three `io` names now encode
UTF-8 and write the BYTES. That also removes a hazard rather than reasoning about
it: `PrintStream` buffers its character writer separately from its byte stream,
so mixing `print(String)` with `write(byte[])` on one stream is a real ordering
question, and using one path for every name means it cannot arise.

**(e) A NARROWED LOOP VARIABLE'S INITIALISER NEEDED A CAST.** Narrowing is
decided per loop, so an INNER loop can narrow while the outer does not — and the
inner's initial value is computed from the outer's variable, so
`int j = i + 1` with `i` a `long` is *"possible lossy conversion"* and javac
refuses the file. monotone-2026-08-27 closed the half where the inner loop's
EXITS are read; **this is the half where its ENTRY is written**, and it appeared
the moment the first program with a nested scanner reached this host —
`freq.oro`'s word scanner. Nothing but javac would have seen it.

Fixing it wanted one more thing, and the sweep is what asked for it: the cast is
emitted only when the initialiser is not already an `int` expression, and
`narrowIdx` accepted only `add`/`sub` **with a literal on the right**. That was
enough while the predicate answered about INDEXES; it answers about an
initialiser now, and the sieve's `(* i i)` is the shape it missed. The rule is
now what *"already typed `int` in Java"* actually means — the operation is
`add`, `sub` or `mul` and **both** operands are narrow.

## 4. What the JVM charges for a byte, priced rather than hidden

`Files.readAllBytes` gives a `byte[]`, and the JVM's byte is **signed**, so
0..255 does not fit it — the exact fact elemwidth-2026-08-27 found when a
declared `(array (int 0 255))` came out `short[]` here and `[]byte` on Go.

Declaring a file's bytes as −128..127 would make `(src i)` answer −1 for 0xFF and
**the same program answer differently on different hosts**, which is ADR 0009's
rule one level down. So this host WIDENS on the way in and NARROWS on the way
out, one pass each, at the boundary and nowhere else — and the cost is written in
the file that knows why:

| | read a file | bytes to text |
|---|---|---|
| Go | free — `[]byte` is `[]byte` | free — `string(b)` |
| JavaScript | free — a `Buffer` is a `Uint8Array` | one pass, `TextDecoder` |
| Java | **one pass**, `byte[]` → `short[]` | **one pass**, `short[]` → `byte[]` |

That is the parasite model pricing something honestly. It is also
string-operations.md's *convert once at the boundary* arriving from a third
direction.

## 5. Measured: three tools, three hosts, byte-identical

Built with `cmd/build` for `go`, `js` and `java`; run on the same inputs; output
compared byte for byte.

| | Go | JavaScript | Java | against |
|---|---|---|---|---|
| `wc README.md` | 888 | 888 | 888 | GNU `wc -l`: 888 |
| `wc` × 6 more files | agree | agree | agree | GNU `wc -l` |
| `freq` × 7 files | agree | agree | agree | `sort \| uniq -c \| sort -rn` |
| `jsonfmt` × 2 documents | agree | agree | agree | a real JSON parser |
| no argument | `usage: …` | `usage: …` | `usage: …` | |
| missing file | `cannot read that file` | same | same | |

The error row is the totalisation working: a Go `err != nil`, a JavaScript
`throw` and a Java checked exception all reach the same line of the same program.

**Provability did not move**: `jsonfmt` is 99 of 99 integer operations bounded
and 13 of 13 loops on every target, `freq` 872 of 872 and 33 of 37, identical to
the Go-only numbers of the day before.

**And `wc.oro` builds at all for the first time.** It had been refused since it
was written — `(+ lines 1)` is unbounded over an unbounded file — and nobody had
noticed, because nothing built it. Stating the input limit takes it to **2 of 2
bounded with no `-checked`**, which is `tree.oro`'s node cap, `jsonfmt`'s
document cap and `freq`'s derived word cap for the fourth time: *a capacity does
not create the limit, it makes it visible.*

One detail worth keeping, because it is about the analysis and not the program:
the bound has to be the OUTER test. `(and a b)` erases to `(if a b false)`
(ADR 0017), so writing the two conditions as one conjunction makes the enclosing
`if`'s condition a nested `if` rather than a comparison — and the refinement
layer reads its facts off comparisons.

## 6. Cost

| | |
|---|---|
| emitted files | **171 of 171 pre-existing byte-identical**; 7 new (the three tools on the hosts that could not build them) |
| differential suite | 30 cases, four targets, green |
| unit tests, `go vet` | green |
| target data added | 274 lines across five files |
| compiler code added | `emit/js.go` +99, `emit/java.go` +127, `emit/target.go` +43, `emit/values.go` +67 |
| new term kinds, reduction rules | **none** |

Five tests, four verified to fail against their bug: the `Σ` test (rename one
declaration on one target), the JavaScript import (drop the sink), the fill order
(its own control, which requires the wrong order to mangle), and the narrowed
initialiser (remove the cast — `int j = (i + 1)` with `i` wide). The fifth pins
Java's widened element and its declared-then-assigned destinations.

The narrowing test carries an **anti-vacuity guard**: refusing to narrow is
always safe, so a test that only ever saw wide locals would pass forever while
checking nothing — it skips loudly unless the emitted method contains one narrow
local and one wide one. Reproducing that shape took four attempts, and the reason
is worth keeping: the outer loop stops narrowing only when its `again` argument
is a **`let`-bound name**, which happens only when the inner scanner's result is
read twice. A miniature with the result used once inlines the loop into the
`again`, `loopExitsFit` fires, and everything narrows together.

## 7. What this leaves, named

**windows.** It declares `kernel32.ReadFile`, which is the handle-based API —
open a handle, read into a buffer you already have. `os.ReadFile` there is
`CreateFileA` + `GetFileSizeEx` + a `build` + `ReadFile` + `CloseHandle`, which
is a LIBRARY written in the language, `lib/win/fmt.oro`'s shape, not a template.
`io` is the same story and `lib/win/fmt.oro` already has the arithmetic half.
**Neither is a language question, and both are the honest remaining gap.**

**Int-to-decimal is not a library, and `wc` is where it shows.** A conditional's
branches must agree, and printing a message gives back a string where counting
gives back an int — so `wc`'s error branches are `seq`-sequenced and the value is
stated. `examples/big/render.oro` shows the conversion is twenty lines of the
language; until it is in `lib/`, `print-int` has to exist beside `print-line`.

**`text-of` on bytes that are not UTF-8 diverges three ways** — Go keeps the
invalid byte, V8 substitutes U+FFFD, Java substitutes U+FFFD — and that is
**outside the claim rather than a bug**: a string is an element of `Scalar*`
(string-literals.md), and a lone 0xE9 is not a scalar sequence. It is recorded
here because it was measured, and because the honest place for it is a
precondition rather than a silent difference.

**`Prim.Import` is one string**, so a primitive needing two host modules must
name a package rather than a class — `java.nio.file.*` here. It is the same shape
as `Prim.Result` being one string, which is what gostdlib-2026-09-06 found was
refusing 19.8% of Go's standard library, and it is worth watching rather than
fixing on one instance.

**And the question the assessment asked is answered for this instance.** *Can
this language express a host's whole API, and where it cannot, is that the
format, the compiler or the language?* Here: **the format could already say
everything**, the COMPILER was missing two of four consumers, and the LANGUAGE
needed nothing at all. That is the third consecutive time the answer has been the
compiler or the format — several results, a declared result range, and now this.
