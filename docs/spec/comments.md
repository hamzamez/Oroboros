# Comments

Written 2026-09-20, after hamza asked whether comments were ever specified. They were not: one
grammar line in [core-0.md §1.2](core-0.md), which was also **wrong** about where a comment ends.
Nothing here changes the compiler; every rule below was read off the reader and is pinned by
`core/comment_test.go`.

The decision §7 records is [ADR 0024](../decisions/0024-comments-are-erased.md).

---

## 1. What a comment is

Reading factors:

```
parse : Text ⟶ Form*        =        Text ⟶ Token* ⟶ Form*
```

A comment is **not a token**. It belongs to the *gap* between tokens, together with whitespace, and
the first arrow erases it. So a comment is precisely the part of a program the compiler is defined
not to see, and the erasure is a quotient: writing `s ≡ s'` when `tokens(s) = tokens(s')`, `parse`
factors through `Text/≡`, and the class of a program contains every re-commenting of it.

Two consequences follow immediately, and both are used below:

- **`parse` is not injective**, so no downstream pass — reduction, the checker, a backend — can
  recover a comment or be affected by one. That is what makes a comment free.
- **The information is destroyed at the first arrow**, so anything that wants to keep it must work
  on `Text`, not on `Form*`. §5 is about the tools that do.

## 2. Grammar

```
comment ::= ";" (any but newline)* (newline | end-of-input)
gap     ::= (whitespace | comment)*
```

**The correction.** [core-0.md §1.2](core-0.md) had `comment ::= ";" any* newline`, which makes a
file whose last line is a comment with no trailing newline *unterminated*. The reader accepts it —
`skipSpace` stops at `r.done()` — and so does every editor that writes a file without one. The grammar
was wrong, not the reader; it now reads `(newline | end-of-input)`.

Two rules that follow from the token grammar rather than from this one:

- **`;` is a delimiter** ([core-0.md §1.1](core-0.md)), so it ends a name: `foo;bar` is the name
  `foo` followed by a comment. There is no escape and no way to put `;` in a name.
- **Inside a string literal `;` is an ordinary character.** `"a;b"` is a three-character string.
  A string is scanned by its own rule, which the gap never enters.

## 3. Where a comment may appear

**Anywhere a gap may: between any two tokens.** There is no position in a form that is special,
because the reader calls `skipSpace` before every term it reads. Verified, each a row of
`core/comment_test.go`:

| written | reads as |
|---|---|
| `(+ 1 ; inside a form`⏎`2)` | `(+ 1 2)` |
| `(def ; between the head and the name`⏎`f 1)` | `(def f 1)` |
| `(def f ( ; inside an empty parameter list`⏎`) 1)` | `(def f (fn () 1))` |
| `(cond p 1 ; between clauses`⏎`else 2)` | `(if p 1 2)` |
| `(loop ((i ; inside the variable list`⏎`0)) …)` | the loop |
| `(sig f ((n ; inside a signature`⏎`int)) int)` | the signature |
| `; a file of only comments`⏎ | no forms, and that is not an error |
| `(+ 1 2) ; at end of file, no newline` | `(+ 1 2)` |

And nowhere else: not inside a string (§2), not inside a name (§2), which are the only two places a
`;` means something other than *comment*.

A target file, a `provides` fragment and a program are read by the same reader, so this holds in all
three; `targets/go/os.oro` is mostly comment.

## 4. What a comment means

**Nothing.** It has no position, no attachment, no representation. `core.Read` returns forms; no
comment reaches `Load`, reduction, the checker, the analyses or a backend, and no emitted file
carries one — each backend writes its own header, and the emission baseline would show a difference
if that were not so.

This is not an oversight to be fixed later. It is §7's decision, and §1 is why it is cheap.

## 5. The obligation on a tool that edits source

Because the compiler cannot see a comment, **the only thing that can lose one is a program that
rewrites source** — and one did, twice, on the day the flat `let` landed
([letflat-2026-09-19 §6](../../gauntlet/results/letflat-2026-09-19.md)): a comment sitting between
a `let`'s value and its continuation, and a snippet inside a Go `//` comment that a scanner read as
code.

> **A source rewriter preserves every gap, or refuses the edit and names the site.**

That is the rule, and it is cheap to obey: collect the gap text between the spans being rearranged,
and if any of it is not whitespace, either place it in the output deliberately or leave the site
alone and print where it is. Silently dropping it is the failure, because §4 guarantees nothing
downstream will ever notice.

The corpus makes the stake concrete: **2,032 comment-only lines of 4,047** in `examples/` and `lib/`,
and 217 of `freq.oro`'s 425. Half the source says *why*, and the compiler is defined not to read it.

## 6. No block comment, and no datum comment

Neither is built, and the second is refused for a reason specific to this language.

**`#| … |#`** would need a nesting rule and a second lexer mode, and nothing has asked for it. A
region is commented by prefixing its lines, which every editor does.

**`#;datum`** — Scheme's expression comment, which discards the *next form* — **cannot be added
here without making the lexer semantic.** Our forms are discriminated by arity and parity:

- `def`'s shorthand is chosen by whether the second element is a **list of names**
  ([program-surface.md §8.3](../program-surface.md));
- `let`'s grammar is chosen by whether the element count after the head is **odd or even**
  ([binding.md §2](binding.md));
- `cond`, `loop` and `match` require their clauses to come **in pairs**.

So `(let x 1 #;y #;2 b)` would not be "the same program with a note" — it would be a *different
construct*, chosen by a comment. A comment that can change which grammar a form takes is not a
comment; it is syntax with a misleading name. Scheme can afford `#;` because none of its forms is
parity-discriminated in this way.

(`#` is not an identifier start, so `#;` is a read error today: *"`#` is not a valid identifier or
number"*. The reader uses `#k` for the binder a tuple pattern desugars to, which is exactly why no
source term can contain one.)

## 7. Documentation is a term, not a comment

> **Decided, [ADR 0024](../decisions/0024-comments-are-erased.md): a comment never carries meaning.**
> There are no doc comments and no pragmas. If documentation is ever to be read by a program, it
> arrives as a **form** — data the reader keeps — and not as a comment the reader is defined to
> discard.

The short argument, in full in the ADR: a comment attached to a *term* has no image under reduction,
because δ copies a definition into every use and β deletes the branch it does not take; and a comment
attached to a *form* is coherent but buys nothing until something reads it, while costing the
language a set of attachment rules (*"the comment group immediately above, with no blank line"*) that
every tool would then have to agree on.

Nothing is built, because nothing consumes it. The shape it would take if a consumer appeared is a
top-level form beside `sig` — a declaration about a name, in a language where
[declarations are theories](theories.md) — and not an extra body, which `def` does not have
([def.md §4](def.md)).

## 8. Witnesses

`core/comment_test.go`, five tests, each shown to fail against a planted bug:

| test | pins | fails when |
|---|---|---|
| where a gap may go | the table of §3, including the empty parameter list and the comment-only file | `skipSpace` no longer knows `;` |
| the end of a comment | terminated by a newline **or by end of input** | a comment is required to end at a newline |
| `;` is not magic inside a string or a name | `"a;b"` is three characters; `foo;bar` is a name and a comment | `;` is dropped from the delimiter set |
| no datum or block comment | `#;1` and `#\|x\|#` are refused, **and the refusal names `#`** | `#` becomes an identifier character — and then `(+ 1 #\| a block \|# 2)` *reads*, which is what the second half of that row is for |
| a comment changes nothing | the same program with and without comments reads to an **identical** term | as the first row |
