# Strings

Written **after** string literals were added to the reader, which was the wrong order. This
document exists to make the addition legitimate or to remove it.

[def.md §5](def.md) had already said strings were "yes, but they are not simple," and the reason
was recorded there: [g5](../derivations/g5-bindings.md) measured that the three targets disagree
on float formatting. They also disagree on something more basic.

---

## 1. What was added, and what was not

| | |
|---|---|
| Added | a **term kind** `KStr` and reader syntax `"…"` |
| Not added | any semantics, any representation, any operation, any specification |
| Used by | **no program.** Every quote in `examples/` is prose in a comment. |
| Used by, actually | `targets/*.oro`, which are **data files, not programs** |

So the language gained a literal it does not use, in order to let the *compiler* read its own
configuration. Those are two different needs and conflating them is the mistake.

Note also that strings were **already in the language** before the literal existed: `split-words`
returns `vec-string`, `sat` returns `string`, `dict-inc` takes one. Word count has been passing
strings around since it was written. What was missing was only the ability to *write one down*.

## 2. The measurement that decides the design

`length` of the same text, on the three targets:

| string | Go | JS | Java | Unicode scalars |
|---|---|---|---|---|
| `abc` | 3 | 3 | 3 | 3 |
| `café` | **5** | 4 | 4 | 4 |
| `日本` | **6** | **2** | **2** | 2 |
| `🙂` | **4** | **2** | **2** | **1** |
| `e` + combining acute | **3** | **2** | **2** | 2 |

Go counts **UTF-8 bytes**. JS and Java count **UTF-16 code units**. Neither counts what a person
would call characters, and *all three* disagree with the scalar count on the emoji.

There is no portable `string-length`. Not "it is hard" — the three answers are different
integers for the same text.

## 3. So what is a string here

> **An immutable, opaque sequence of Unicode scalar values.**
>
> **Its representation is the target's, and is deliberately unspecified — because it is never
> exposed.**

The question "in C an array, in Go a slice, what here?" has an answer that sounds evasive and is
not: **we have no data structures at all.** `string`, `vec-f64`, and `dict` are opaque handles
that only primitives create and inspect. Nothing in the language can look inside one. Strings are
not *built on* anything here; they are **borrowed**, which is the whole Parasite position applied
to a type.

That is what makes the representation question disappear. A Go string is a UTF-8 slice header, a
Java string is a UTF-16 object, a JS string is a UTF-16 primitive — and no program can tell,
because no operation in the core distinguishes them.

## 4. The specification

**Core provides exactly one thing: a literal.**

- A literal denotes a sequence of Unicode scalar values, written in source, which is UTF-8 and
  NFC-normalised ([core-0 §1.1](core-0.md)).
- A literal may be **passed to a primitive**. That is all it can do.
- There is **no** length, no indexing, no slicing, no concatenation, no comparison in the core.

**Everything else is a per-target primitive, Tier 2, with no portability claim** — because §2
shows that `length`, the simplest operation anyone would ask for, already means three different
things.

### Why this is portable

A literal is representation-independent: the *same scalar sequence* is denoted whether the target
stores UTF-8 or UTF-16. Passing it to a primitive is representation-independent for the same
reason. Since nothing else exists, nothing can observe the difference.

This is the same shape as [q5c](q5c-representation-choice.md)'s pull-versus-push result:
**the core stays portable by refusing to expose what diverges.** Twice now, and it is worth
naming as a pattern rather than two coincidences.

### What it costs

`(string-equal a b)` is not available, and a program that needs it must use a per-target
primitive. That is a genuine limitation and it will be felt as soon as anything compares strings.

Equality *is* representation-independent and could be promoted to Tier 1 later. It is left out
now because nothing needs it, per [ADR 0007](../decisions/0007-exploration-over-specification.md)
— add it when a program demands it, not before.

## 5. A hazard the measurement turned up

`javac` defaults to the **platform charset**, not UTF-8. The Java column in §2 initially matched
Go's exactly, which is impossible for a UTF-16 language — because `javac` on Windows read the
UTF-8 source as Windows-1252 and `café` became five characters.

So a Java source file emitted with a non-ASCII literal **means something different depending on
how the build invokes the compiler.** The build system has to pass `-encoding UTF-8`, and that is
not a compiler concern but a *build* concern — the first thing found that belongs to the half of
this project that does not exist yet.

Recorded here rather than in a results file because it is a specification hazard, not a
measurement.

## 6. What should happen to the code

Two honest options.

**Keep the literal, specified as §4.** It costs one term kind, it is what program 5 will need for
`(print-line "hello")`, and the specification above is short and complete.

**Or remove it and give target files their own reader.** `core.Term` is currently doing double
duty as both the language's term type and the s-expression parse tree for configuration, which is
a real smell: a target file is not a program and should not have to be one.

**Recommendation: keep it, and record the double duty as a known smell.** The literal is genuinely
needed shortly, and splitting the reader is work with no user today. But the smell should not go
unnamed — if target files grow anything the language does not have, the split becomes forced.

## 7. Deliberately still absent

- **Equality, length, indexing, slicing, concatenation.** All per-target until a program needs
  otherwise.
- **A `char` or code-point type.** Would immediately force the representation question §3 avoids.
- **Escape-set narrowing.** The reader delegates to `strconv.Unquote`, so the accepted escapes are
  Go's rather than the four this document implies. Recorded in
  [concerns.md §1.5](concerns.md).
