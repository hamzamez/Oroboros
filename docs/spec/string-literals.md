# String literals

2026-09-04. **Specified and built.** The specification [strings.md](strings.md) §4
gestured at and §7 recorded as missing.

**Derived from what a string is, not from what the hosts happen to accept.** The
first draft of this document built its escape set as *"the intersection of Go's,
Java's and JavaScript's"* and justified two refusals with *"Go cannot represent
one"* and *"a windows string is NUL-terminated"*. That is a target dictating a
language. Targets inform an implementation; §6 is where they belong, and one of
those two refusals does not survive the correction.

---

## 1. What a string is

> A string is an element of **Scalar\*** — the free monoid over Unicode scalar
> values.

**The free monoid**, because a string is a finite sequence: concatenation is
associative, the empty string is its identity, and nothing further is assumed.
That is the least structure the object needs, and every operation added later has
to respect it.

**Over scalar values**, and this is the load-bearing choice. Unicode defines a
*scalar value* as a code point outside the surrogate range D800–DFFF. A surrogate
is not a character; it exists only as an artefact of the UTF-16 encoding form, and
the standard excludes it from the scalar values for exactly that reason.

The consequence is what makes it the right set:

> A sequence of scalar values has a **unique representation in every Unicode
> encoding form**. A sequence of arbitrary code points does not — a lone
> surrogate has no encoding in UTF-8 or UTF-32.

So `Scalar*` is the encoding-independent object, and encoding independence is the
property a portable string type needs. **A lone surrogate is not denotable because
it is not a scalar value**, which is a fact about Unicode. That Go could not store
one either is a coincidence, and citing it was the error.

**U+0000 is an ordinary scalar value and a string may contain it.** The first
draft refused it because a windows string is a pointer to NUL-terminated bytes.
That is a property of the C calling convention, not of strings — see §6.4, where
it belongs.

## 2. What a literal has to be

A literal is a **notation** for an element of `Scalar*`. Four requirements:

1. **Total** — every element of `Scalar*` is writable.
2. **Unambiguous** — every well-formed literal denotes exactly one element.
3. **Decidable in one pass** — the reader can find the end of a literal without
   backtracking, and can reject a malformed one where it goes wrong.
4. **Legible** — the common cases read as themselves.

The first three are requirements on any notation. The fourth is a preference and
is marked as one wherever it decides something.

## 3. What that forces

Start from the only rule that needs no justification: **a scalar denotes itself**.
A literal is then a delimited sequence of scalar denotations, and its value is
their concatenation — which is the monoid operation, so the notation mirrors the
object.

Three classes of scalar cannot denote themselves, and each forces exactly one
escape.

| scalar | why it cannot denote itself | escape |
|---|---|---|
| U+0022 `"` | it delimits the literal — (2) | `\"` |
| U+005C `\` | it introduces an escape — (2) | `\\` |
| U+0009, U+000A, U+000D | the source grammar's own whitespace — see below | `\t`, `\n`, `\r` |

**The third row is the source grammar, not a host.** A literal is a lexical token
and the reader is line-oriented: a token that could contain a raw line terminator
makes an unterminated literal indistinguishable from a long one, and the error is
then reported at the end of the file rather than where it happened — which fails
(3). Tab joins them for (4): a literal containing a raw tab is
indistinguishable from indentation at a glance.

And **totality forces one more**, because the three classes above are not the only
scalars a programmer cannot type — most of the 1,112,064 of them are unavailable
on any keyboard, and many are invisible:

| | |
|---|---|
| `\u{H…}` | one to six hexadecimal digits, denoting that scalar value |

**Six escapes, every one of them forced.** `\u{…}` is written that way rather than
as `\uHHHH` plus `\UHHHHHHHH` because scalar values are one set, not two: a
notation that splits them at U+FFFF is describing UTF-16's internals rather than
the object. It also cannot spell half of a surrogate pair, so §1's exclusion is
checked as a numeric range rather than as a rule about how two escapes combine.

### Not escapes

`\a`, `\b`, `\f`, `\v`, `\xHH`, `\NNN` and every other `\X`. **An unknown escape
is an error**, which follows from (2): if `\a` meant `a`, then two different
literals would denote the same string and the notation would carry a redundancy
that only hides mistakes.

`\b` and `\f` were in the first draft. They are not forced — U+0008 and U+000C
can be written literally, and are `\u{8}` and `\u{C}` if you would rather not —
and they were there because Java and JavaScript have them. That is the error this
document was rewritten to remove.

`\xHH` and `\NNN` denote a **byte**. A byte is not a scalar value, so they are not
notation for anything in `Scalar*`; a notation that can build `\xff` is a notation
for a different object.

## 4. Source text

The source is UTF-8 and NFC-normalised, and the reader rejects both failures at
the offending position. That is already true today.

**NFC constrains the program TEXT, not the data.** `"e\u{301}"` is a two-scalar
string — `e` followed by COMBINING ACUTE ACCENT — and it is legal, because the
source containing it is itself normal.

That distinction was found by measuring rather than by reading: the reader checks
NFC on the raw source and *then* decodes escapes, so an escape can build a
sequence the source could not have contained literally. It is the right
behaviour — normalising a value would silently rewrite data a programmer wrote
deliberately — but it had never been said, and strings.md §4's *"a sequence of
Unicode scalar values, written in source, which is UTF-8 and NFC-normalised"*
conflates the two.

## 5. What the language does not decide

**How a string is stored.** [strings.md §3](strings.md) leaves that to the target
and it stays there: UTF-8 on Go and windows, UTF-16 on Java and JavaScript. §1
constrains what a literal *denotes*; the representation is chosen underneath it,
which is [ADR 0003](../decisions/0003-range-typed-integers.md)'s split between
mathematical semantics and machine representation applied to a second type.

**What can be done with a string.** strings.md §2 measured that `length` is 4, 2
and 2 for `"🙂"` and 1 counting scalars, so there is no portable length and the
core has no string operation at all. This document says how to *write* one.

## 6. The implementation obligation

> The compiler must render **every** element of `Scalar*` faithfully on every
> target. Where a host's notation cannot express a scalar directly, the compiler
> finds one that can.

That is the parasite rule, and it is the only place a target may speak.

### 6.1 What each target is given

| | how a scalar is written |
|---|---|
| Go | `\\ \" \n \r \t`; otherwise `\uHHHH` in the BMP, `\UHHHHHHHH` above it |
| Java | the same five; otherwise `\uHHHH`, and a **surrogate pair** above the BMP |
| JavaScript | the same five; otherwise `\uHHHH`, and a **surrogate pair** above the BMP |
| windows | raw UTF-8 bytes in a `db` directive, with a length — it is data, not source |

Java and JavaScript get surrogate pairs because their string type is a sequence of
UTF-16 code units, and the pair is that encoding form's representation of the same
scalar. Nothing is lost: §1 chose `Scalar*` precisely because it has a unique
representation in every encoding form.

**Output is ASCII only.** `javac` defaults to the *platform charset*, so a
non-ASCII byte in an emitted `.java` file means whatever the build's locale says —
the hazard strings.md §5 recorded, where `café` became five characters because
Windows-1252 was assumed. Emitting `é` removes it **by construction** rather
than by remembering a flag.

### 6.2 The hazards, measured

Not a source of requirements — a record of what careless rendering costs. The same
literal, built and run on each host:

| written | Go | Java | JavaScript |
|---|---|---|---|
| `\t` | `41 09 42` | ok | `41 09 42` |
| `\a` | `41 07 42` | **compile error** | `41 61 42` — the letter **`a`** |
| `\v` | `41 0b 42` | **compile error** | `41 0b 42` |
| `\xff` | `41 ff 42` — a raw byte | **compile error** | `41 c3 bf 42` — U+00FF |
| `\u{200B}` | `41 e2 80 8b 42` | ok | `41 e2 80 8b 42` |
| `🙂` | correct | ok | correct |
| U+E0001 | correct | **compile error** | `41 55 30 30 30 65 30 30 30 31 42` |

The last row is the one to keep. **U+E0001 is an ordinary scalar value**, and the
program containing it is correct on Go, fails to compile on Java, and on
JavaScript prints the ten characters `U000e0001` — because JS drops the backslash
of an escape it does not know. It does not depend on how the character was
written: `strconv.Quote` renders any *unprintable* scalar above the BMP as `\U`,
which only Go has, so writing the character raw produces the same emitted text.

That is a failure of §6's obligation, not a reason to remove U+E0001 from the
language. §6.1 discharges it.

It also corrects a comment carried by all three of `emit/golang.go`, `emit/js.go`
and `emit/java.go`: *"strconv.Quote escapes non-ASCII to \u, which is valid in all
three hosts and sidesteps javac's platform-charset hazard entirely."* It does not.
`Quote` passes printable non-ASCII through **raw**, so the emitted Java file
contains UTF-8 bytes and works only because the build happens to pass
`-encoding UTF-8`.

### 6.3 What the build found

**Two of §6.2's four rows are now correct on every host and one is a reader
error.** Re-measured after the change, with the JVM asked for its string's UTF-16
length rather than for printed bytes, since `System.out` encodes in the platform
charset and that is a property of *printing*:

| written | Go | Java | JavaScript |
|---|---|---|---|
| `\u{200B}` | `41 e2 80 8b 42` | 3 units | `41 e2 80 8b 42` |
| `🙂` | `41 f0 9f 99 82 42` | 4 units | `41 f0 9f 99 82 42` |
| `\u{E0001}` | `41 f3 a0 80 81 42` | 4 units | `41 f3 a0 80 81 42` |
| `\u{0}` | `41 00 42` | 3 units | `41 00 42` |
| `\a`, `\v`, `\xff` | refused by the reader, on every target |

The third row was a Java compile error and ten literal characters on JavaScript.
The fourth is the refusal §6.4 removes, and it works everywhere.

**And `msvcrt.puts` does not link.** Found by writing the differential case: the
generated `build.bat` linked `msvcrt.lib` and `legacy_stdio_definitions.lib` and
**not the UCRT**, so every CRT stdio entry point that target declares —
`puts`, `printf`, `putchar` and the rest — has been unreachable since it was
declared, and no program had used one. Adding `ucrt.lib vcruntime.lib` to the
link line fixes it and changes no emitted file. A **build** concern rather than a
language one, which is the half of this project strings.md §5 already named as
not existing yet.

### 6.4 U+0000, which is where the first draft went wrong

A windows string is a pointer to NUL-terminated bytes, because that is what every
Win32 and CRT entry point expects. So `msvcrt.puts` handed a string containing
U+0000 prints a prefix.

**That is a precondition on `puts`, not a hole in `Scalar*`.** `puts` is a
target-native primitive with no portability claim, and the language already has
the machinery to say so — a `where` on the primitive, discharged at every call
site. The x86 emitter already writes a length beside the bytes (`LS…_len`), so
the data survives; it is the C API that cannot read past a NUL.

Refusing U+0000 in the language would have been a host's calling convention
deciding what a string is, on four targets, forever, to save one `where` on one
primitive.

## 7. What is checked, and where

| | |
|---|---|
| the reader | UTF-8, NFC, §3's six escapes, the surrogate exclusion, no raw tab or line terminator |
| the emitter | §6.1, which is total: every scalar has a spelling on every target |
| the differential suite | `string-escapes`, whose literal carries all six escapes, built and run on **all four targets** |
| `emit/strlit_test.go` | **every one of the 1,112,064 scalar values**, both host shapes: the output is ASCII, and the UTF-16 rendering decodes back to the scalar it came from |

**The suite case is ASCII only, and the reason is a real divergence rather than
caution.** `System.out` on the JVM encodes in the platform charset, so a printed
`é` is one byte on a Windows console and two on Go and V8. That is a property of
PRINTING and not of the literal — asking the JVM for the string's UTF-16 length
instead gives 3, so the literal is intact. **Printing a string is not portable**,
which is strings.md §2's finding arriving in the test harness.

So totality is checked where it can be: by rendering every scalar value, which a
table of examples cannot establish. That test fails when either original bug is
put back — a `\U` escape given to the UTF-16 hosts fails at U+10005, and passing
printable non-ASCII through raw fails at U+007F.

Running a string case at all needed the harness to grow a **string driver**: it
prints an integer, and there is no portable operation from a string to one.

## 8. Cost of the narrowing

The corpus uses exactly three escapes inside string literals — `\"` (20), `\\` (5),
`\n` (172) — and **no non-ASCII inside any literal**. Every one of them survives.
Scanned with the reader's own rule rather than by grep, which counts prose.
