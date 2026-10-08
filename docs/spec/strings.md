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

> **CORRECTED 2026-09-04 — this is a target fact wearing a language fact's clothes**, which is the
> same error [string-literals.md](string-literals.md) was rewritten to remove. The three integers
> answer three DIFFERENT questions: Go's `len` asks how many UTF-8 bytes, Java's and JavaScript's
> ask how many UTF-16 code units, and neither asks how many SCALAR VALUES — the only question
> `Scalar*` poses, and the one with a single answer (4, 2, **1**, 2 for the rows above). It is
> computable on every target at O(n) rather than O(1), which maps.md already records as a
> difference in PRICE and not a reason to call a construct Tier 2. **`length` is Tier 1**, and so
> are `=` and `concat` — see [string-operations.md](../string-operations.md).

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

> **The literal itself is specified in [string-literals.md](string-literals.md)** — what it
> denotes, the escape set, and how each target must emit one. It supersedes the third bullet
> below and §7's last item, and it was written after measuring what the four hosts actually do
> with an escape: `\a` is a bell on Go, a **compile error** on Java and the letter `a` on
> JavaScript, and an unprintable scalar above the BMP is correct on Go, a compile error on
> Java and **ten literal characters** on JavaScript.

**Core provides exactly one thing: a literal.**

- A literal denotes a sequence of Unicode scalar values
  ([string-literals.md §1](string-literals.md)). The source is UTF-8 and NFC-normalised
  ([core-0 §1.1](core-0.md)) — but NFC constrains the program TEXT and not the data, which
  that document establishes and this line used to conflate.
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

> **AND IT IS DERIVED, not merely possible.** `Scalar` has decidable equality, so the free monoid
> over it does — and every host's own equality already computes it, because UTF-8 and UTF-16 are
> both bijections on scalar sequences. Nothing has to be built for it to be right
> ([string-operations.md §1](../string-operations.md)).

## 5. A hazard the measurement turned up

> **CLOSED BY CONSTRUCTION, 2026-09-04.** The emitter now writes ASCII only
> ([string-literals.md §6](string-literals.md)), so an emitted `.java` file has no
> byte whose meaning depends on the build's locale. The `-encoding UTF-8` flag is
> still passed and is now belt as well as braces. And the hazard has a twin on
> the way OUT that this section did not know about: `System.out` also encodes in
> the platform charset, so a printed `é` is one byte on a Windows console and two
> on Go and V8 — a property of PRINTING rather than of the literal, confirmed by
> asking the JVM for the string's UTF-16 length instead.


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
- ~~**Escape-set narrowing.**~~ **Specified** in [string-literals.md](string-literals.md), after
  measuring three live divergences: `\a` and `\v` do not exist on Java, `\xHH`
  denotes a byte rather than a scalar, and an unprintable scalar above the BMP is emitted as
  `\U…` — which Java refuses and JavaScript silently reads as ten characters. The corpus
  uses exactly three escapes inside literals and no non-ASCII at all, so narrowing costs
  nothing.

## 8. The host's string ([ADR 0030](../decisions/0030-a-hosts-string-is-the-hosts.md))

§3's definition, a sequence of Unicode scalar values, is now true of **every value a program holds**,
not only of literals. A host's string is the host's type, and it enters ours through one total decode.

| | set | on Go |
|---|---|---|
| `string` | Σ*, Σ the scalar values | a Go `string` that is valid UTF-8 |
| `go.bytestring` | B*, B the bytes | any Go `string` |
| `string ≤ go.bytestring` | by enc, UTF-8 | the identity: no code is emitted |
| `go.text : bytestring → string` | d, maximal-subpart substitution (Unicode §3.9) | a validity scan, then the substitution only if it fails |
| `go.valid : bytestring → bool` | χ_V, V = enc(Σ*) | `utf8.ValidString` |

**d, exactly.** Read the bytes left to right. A byte below 0x80 is its scalar. A byte in C2–DF, E0–EF
or F0–F4 begins a sequence of 2, 3 or 4 bytes, whose continuation bytes must lie in the ranges of
Unicode Table 3-7:
- after E0, the second byte is A0–BF;
- after ED, 80–9F;
- after F0, 90–BF;
- after F4, 80–8F;
- otherwise 80–BF.

A complete sequence is its scalar. **Any other byte, or the longest prefix of a sequence that cannot
be completed, is one U+FFFD**, and reading resumes at the byte that broke it. So `E6 97 61` is FFFD a,
`ED A0 80` is FFFD FFFD FFFD, and `80` is FFFD.

**The laws**, from ADR 0030:
- enc is an injective homomorphism, and V is a free submonoid of B*;
- d ∘ enc = id, and enc ∘ d = id on V;
- d is not a homomorphism;
- Theorem 1 (synchronization) keeps searching, splitting, replacing, trimming and concatenation of
  valid strings inside V.

**The portable `os.text-of` is d on every host.**
- **JS:** `TextDecoder` is d.
- **Go:** implements d, with `utf8.Valid` as the fast path.
- **Java:** implements d. Its fast path is the lenient decoder, whose answer is kept when it holds no
  U+FFFD: the lenient decoder gives one U+FFFD for an encoded surrogate where d gives three, but a
  U+FFFD in its answer marks every ill-formed input.

The portable `os.Args` and `os.Getenv` on Go decode by d.

**The rule is variance.** `string ≤ go.bytestring`, and a function type is contravariant in its
parameters and covariant in its result:
- **a parameter** is the host's string unless a result's claim depends on it. A Go function accepts
  any bytes, and a `go.bytestring` parameter accepts both kinds of string, ours by the free coercion.
  Declaring `string` would claim the host requires valid UTF-8, and refuse programs that hold a host
  string. `strconv.ParseInt` did, until `lines.oro` passed it a field from `Scanner.Text`
  (bufio-2026-10-01);
- **a result** is ours only where the host maps every input into V, read against Go's source.

**The 70 hand-declared Go results that mention a string** (checked 2026-10-06):

| `string` (24) | why the result is in V |
|---|---|
| `big-str`, `hex.EncodeToString`, `hex.Dump`, `strconv.FormatBool`/`FormatInt`/`FormatUint`/`Itoa`/`FormatFloat`/`QuoteToASCII`/`QuoteRuneToASCII` (10) | the output is ASCII |
| `strconv.Quote`/`QuoteToGraphic`/`QuoteRune`/`QuoteRuneToGraphic`, `strconv.NumError.Error`, `hex.InvalidByteError.Error` (6) | every byte that is not a printable scalar is escaped; `NumError.Error` prints its `Func` unquoted, which is why that field is declared ours |
| `concat`, `string-of`, `go.text` (3) | by construction: V·V ⊆ V, a scalar's encoding is in V, and d maps B* into Σ* |
| `strings.ToLower`/`ToUpper`/`ToTitle`/`Title`, `strings.ToValidUTF8` (5) | a map B* → V: the case maps go through `strings.Map`, which re-encodes an invalid byte as U+FFFD (measured on 1,011,155 inputs, gotarget-2026-09-30); `ToValidUTF8` replaces with a replacement declared ours |

| `go.bytestring` (46) | why the result may be outside V |
|---|---|
| `strings.Cut`/`CutLast`/`CutPrefix`/`CutSuffix`, `Split`/`SplitN`/`SplitAfter`/`SplitAfterN`, `Fields`, `TrimSpace`/`Trim`/`TrimLeft`/`TrimRight`/`TrimPrefix`/`TrimSuffix`, `strconv.QuotedPrefix`, `UnquoteChar`'s tail (17) | a factor of the argument, which may be any bytes. A backquoted literal is "not verified … as valid UTF-8" (strconv/quote.go) |
| `strings.Join`, `Repeat`, `Replace`, `ReplaceAll`, `Clone` (5) | built from the arguments' bytes |
| `strconv.Unquote` (1) | `\x` and octal escapes denote bytes |
| `fmt.Sprint`, `Sprintln`, `Sprintf`, `fmt.Stringer.String` (4) | an `any` argument may be host bytes, and `%s` prints a byte slice raw |
| `os.Args`, `Getenv`, `LookupEnv`, `Environ`, `Getwd`, `Hostname`, `Executable`, `TempDir`, `MkdirTemp`, `File.Name` (10) | the operating system's own strings, which are bytes on Unix |
| `string-of-bytes`, `bufio.Scanner.Text`, `strings.Builder.String` (3) | `string(b)` of arbitrary bytes: a conversion, data read, or bytes accumulated |

**What one signature cannot say.** Most of the factor functions are two facts at once: Fields maps B*
into (B*)*, and it also maps V into V*, since splitting a valid string at whitespace leaves valid
pieces (Theorem 1). A `sig` states one type, so a program that passes ours to `strings.Fields` gets
`(array bytestring)` back, and recovers ours only through `go.text`, a validity scan at run time for a
fact that was known statically. The precise type is an intersection, (B* → (B*)*) ∧ (V → V*): datasort
refinements carried through functions by intersection types (Freeman and Pfenning, "Refinement types
for ML", PLDI 1991). Not built; CLAUDE.md names its trigger.

**Parameters.** Of 141 string parameters, 137 are `go.bytestring`. The 4 that are ours are each
narrowed because a result's claim depends on them: `concat`'s two (the language's monoid), `strings.ToValidUTF8`'s
replacement (its result is in V exactly when the replacement is), and `strconv.NumError`'s `Func`
(printed unquoted by `Error`). Until 2026-10-06, 21 more were ours with no such reason: `fmt`'s 12
format parameters, `strconv.AppendQuote`'s three, `CanBackquote`, `Unquote`, `QuotedPrefix`,
`UnquoteChar`, `NumError`'s `Num` and `hex.DecodeString`. Widening them accepted more programs and
changed no emitted code; `QuotedPrefix`'s result and `UnquoteChar`'s tail became the host's, as factors
of a host string.

**Generated declarations** spell every Go `string` as `go.bytestring`, because the generator cannot read
a body (ADR 0023). A string constant is `string` when its value is valid UTF-8.
