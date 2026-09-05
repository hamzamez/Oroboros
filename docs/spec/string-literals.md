# String literals

2026-09-04. The specification [strings.md](strings.md) §4 gestured at and §7 recorded
as missing: *"the reader delegates to `strconv.Unquote`, so the accepted escapes
are Go's rather than the four this document implies."*

Everything below is measured on all four targets before it is specified. The
divergences in §3 are real and three of them are live today.

---

## 1. What a literal denotes

> A literal denotes a **finite sequence of Unicode scalar values**.

A *scalar value* is a code point outside the surrogate range — U+0000…U+D7FF and
U+E000…U+10FFFF. That is the largest set all four targets can represent: Go and
windows store UTF-8, Java and JavaScript store UTF-16 and encode a non-BMP
scalar as a surrogate **pair**.

Two consequences, and both are refusals rather than conventions.

**A lone surrogate is not denotable.** A Java or JavaScript string is a sequence
of UTF-16 *code units* and can hold an unpaired surrogate; a Go string cannot
represent one in valid UTF-8. So the language does not offer a way to write one,
which keeps the definition the same on four hosts instead of three.

**U+0000 is not denotable.** On windows a string is a pointer to NUL-terminated
bytes, because that is what every Win32 and CRT entry point expects
([windows-target.md](windows-target.md)). An embedded NUL would truncate the
string there and not on the other three — an observable divergence inside a
construct that claims to be portable, so it is refused at the reader instead.

## 2. Source text

The source file is UTF-8 and must be NFC-normalised; the reader rejects both
failures with the offending position, and that is already true today.

**NFC constrains the program TEXT, not the data.** `"e\u{301}"` is a two-scalar
string — `e` followed by COMBINING ACUTE ACCENT — and it is legal, because the
source text containing it is itself normal. Nothing normalises a string's value.

That distinction is not pedantry and it was found by measuring: the reader
checks NFC on the raw source and then decodes escapes, so an escape can build a
sequence the source could not have contained literally. Normalising values
instead would silently rewrite data a programmer wrote deliberately, so the
current behaviour is right — it had simply never been said. strings.md §4's *"a
sequence of Unicode scalar values, written in source, which is UTF-8 and
NFC-normalised"* conflates the two and is superseded by this section.

A literal is **one line**. A raw newline before the closing quote is an error;
write `\n`.

## 3. The measurement that decides the escape set

The same literal, built and run on each host. `A` is 41 and `B` is 42.

| written | Go | Java | JavaScript |
|---|---|---|---|
| `\t` | `41 09 42` | ok | `41 09 42` |
| `\a` | `41 07 42` | **compile error** | `41 61 42` — the letter **`a`** |
| `\v` | `41 0b 42` | **compile error** | `41 0b 42` |
| `\xff` | `41 ff 42` — a raw byte | **compile error** | `41 c3 bf 42` — U+00FF |
| `​` | `41 e2 80 8b 42` | ok | `41 e2 80 8b 42` |
| `🙂` (printable, non-BMP) | correct | ok | correct |
| `\U000e0001` (unprintable, non-BMP) | correct | **compile error** | `41 55 30 30 30 65 30 30 30 31 42` |

Read the last row carefully. **U+E0001 is a perfectly ordinary scalar value**, and
the program that contains it produces the right bytes on Go, fails to compile on
Java, and on JavaScript prints the ten characters `U000e0001` — because JS drops
the backslash of an escape it does not know. A silent wrong answer, and it does
not depend on how the programmer wrote the character: `strconv.Quote` renders any
*unprintable* scalar above the BMP as `\U`, so writing the character raw produces
exactly the same emitted text.

Three separate causes, all measured:

- **`\a` and `\v` do not exist in Java.** `\a` also does not exist in JavaScript,
  where an unknown escape means the character itself.
- **`\xHH` and octal denote a BYTE, not a scalar.** `\xff` in Go gives one byte
  0xFF, which is not valid UTF-8 and therefore not a scalar sequence at all —
  the reader accepts something §1 says is not a string. Java has no `\x`, and
  JavaScript reads `\xff` as U+00FF, a different value.
- **`\UHHHHHHHH` exists only in Go.** Java and JavaScript are UTF-16 and spell a
  non-BMP scalar as a surrogate pair.

## 4. The escape set

Seven single-character escapes and one general one. Every scalar value is
reachable, and nothing here means two things on two hosts.

| written | scalar | |
|---|---|---|
| `\\` | U+005C | reverse solidus |
| `\"` | U+0022 | quotation mark |
| `\n` | U+000A | line feed |
| `\r` | U+000D | carriage return |
| `\t` | U+0009 | character tabulation |
| `\b` | U+0008 | backspace |
| `\f` | U+000C | form feed |
| `\u{H…}` | that scalar | one to six hexadecimal digits, upper or lower case |

The seven are exactly the intersection of Go's, Java's and JavaScript's escape
sets — `\a` and `\v` are the two that Java lacks, and they are the two §3
measured breaking.

`\u{…}` rather than `\uHHHH` and `\UHHHHHHHH`:

- **one form for every scalar**, with no four-digit / eight-digit split and no
  rule about which to use;
- **it cannot spell half of a surrogate pair**, so §1's refusal is a check on a
  number rather than a rule about how two escapes combine;
- and the emitter has to convert to each host's spelling anyway — Java and
  JavaScript need a surrogate pair, Go needs `\U` or a raw character — so the
  source form is free to be the clear one rather than any host's.

Non-ASCII may also be written **raw**: `"café"`, `"日本"`, `"🙂"`. That is the
normal way; the escape is for characters that are hard to type or invisible.

### Refused, with the reason

| written | why |
|---|---|
| `\a`, `\v` | §3: Java has neither, and `\a` on JavaScript is the letter `a` |
| `\xHH`, `\NNN` | denote a byte, not a scalar — they can build a string that is not one |
| `\uHHHH`, `\UHHHHHHHH` | superseded by `\u{…}`; `\U` is Go-only and `\uHHHH` can spell a lone surrogate |
| `\u{D800}`…`\u{DFFF}` | not a scalar value (§1) |
| `\u{0}` | a windows string is NUL-terminated (§1) |
| `\'`, `\0`, any other `\X` | not an escape; a backslash must introduce one |

An unknown escape is an **error**, never the character itself. That is the rule
JavaScript does not have, and §3 shows what it costs there.

## 5. Emission: ASCII only, and only escapes the host has

> The emitter writes a literal using **ASCII characters only**, and only escapes
> the target language actually has.

| | how a scalar is written |
|---|---|
| Go | `\\ \" \n \r \t \b \f`; otherwise `\uHHHH` in the BMP, `\UHHHHHHHH` above it |
| Java | the same seven; otherwise `\uHHHH`, and a **surrogate pair** above the BMP |
| JavaScript | the same seven; otherwise `\uHHHH`, and a **surrogate pair** above the BMP |
| windows | raw UTF-8 bytes in a `db` directive, NUL-terminated — no escaping, it is data |

ASCII-only output is not tidiness. `javac` defaults to the **platform charset**,
so a non-ASCII byte in an emitted `.java` file means whatever the build's locale
says it means — the hazard strings.md §5 recorded, where `café` became five
characters because Windows-1252 was assumed. Emitting `é` instead removes
the hazard **by construction** rather than by remembering to pass `-encoding
UTF-8`.

That is also a correction. All three of `emit/golang.go`, `emit/js.go` and
`emit/java.go` carry the comment *"strconv.Quote escapes non-ASCII to \u, which
is valid in all three hosts and sidesteps javac's platform-charset hazard
entirely"* — and `strconv.Quote` does no such thing. It passes printable
non-ASCII through **raw**, which is why `"🙂"` reaches the emitted Java file as
four UTF-8 bytes and works only because the build happens to pass `-encoding
UTF-8`. The comment states a property the code does not have.

## 6. What is checked, and where

| | |
|---|---|
| the reader | UTF-8, NFC, the escape set of §4, the two refusals of §1, one line |
| the emitter | §5's per-target rendering, which is total: every scalar has a spelling on every target |
| the differential suite | one case whose literal contains an ASCII escape, a BMP scalar and a non-BMP scalar, printed and required byte-identical on the hosts that can print |

The suite case matters more than usual here, because **three of the four
divergences in §3 are invisible to a compiler**: two are host compile errors,
which a differential suite catches only if it builds every target, and one is a
silent wrong answer on JavaScript.

## 7. What this does not decide

**Nothing about operations.** [strings.md](strings.md) §2 measured that `length`
is 4, 2 and 2 for `"🙂"` and 1 counting scalars, so there is no portable length
and the core has no string operation at all. This document specifies how to
*write* a string; what can be done with one is a separate question and a harder
one.

**Nothing about representation.** §1 constrains what a literal *denotes*, and
strings.md §3 leaves what a target *stores* deliberately unspecified — UTF-8 on
Go and windows, UTF-16 on Java and JavaScript. The two are independent, which is
exactly ADR 0003's split between meaning and machine representation applied to a
second type.

**Nothing about `char`.** A code-point type would force the representation
question this avoids, and there is no program for it.
